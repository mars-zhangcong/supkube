// Package backupcopy 实现 PRD-007 §4.3 Layer 4 Backup Copy.
//
// Phase 1: Preflight (拦截 snapshotMoveData=false 备份)
// Phase 2 (TODO): Transfer (rclone sync kopia/<ns>/ + backups/<name>/)
// Phase 3 (TODO): Verify + Activity Timeline 联动 (PRD-006 BackupCopy ActionType)
//
// 实证基础: engineer-testing/fixtures/velero-real-2026-05-31-060756/
// ADR-031 §8 (二次实测) + PRD-007 §4.3 (P1 fixture 背书)
package backupcopy

import (
	"context"
	"fmt"

	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PRD-007 §4.3 + DoD #2a 锁定错误码: 拦截快照型备份 (snapshotMoveData=false 且无 fs-backup).
// UI 将此码映射到本地化错误提示 (i18n key `backupCopy.error.snapshotUnsupported`).
const ErrLayer4SnapshotUnsupported = "ERR_LAYER4_SNAPSHOT_UNSUPPORTED"

// PreflightRequest 是 POST /backup-copy/preflight 的请求.
type PreflightRequest struct {
	SourceBSL string   `json:"sourceBSL"`         // 源 BSL 名 (Velero BSL CR)
	TargetBSL string   `json:"targetBSL"`         // 目标 BSL 名
	Backups   []string `json:"backups,omitempty"` // 可选, 限制只 preflight 这些备份名; 空=全部
}

// PreflightResult 是 preflight 返回, UI 据此决定是否允许 trigger Transfer.
type PreflightResult struct {
	Eligible []string         `json:"eligible"` // 可走 Layer 4 的备份 (data-mover / fs-backup / snapshotMoveData=true)
	Rejected []RejectedBackup `json:"rejected"` // 被拦截的备份 + 原因
	Summary  PreflightSummary `json:"summary"`
}

// RejectedBackup 是被 Preflight 拦截的备份.
type RejectedBackup struct {
	Name       string `json:"name"`
	Reason     string `json:"reason"`     // 人话原因
	ErrorCode  string `json:"errorCode"`  // ERR_LAYER4_SNAPSHOT_UNSUPPORTED 等
	Suggestion string `json:"suggestion"` // 修复指引
}

// PreflightSummary 摘要.
type PreflightSummary struct {
	EligibleCount  int   `json:"eligibleCount"`
	RejectedCount  int   `json:"rejectedCount"`
	EstimatedBytes int64 `json:"estimatedBytes,omitempty"` // Phase 2 算
}

// Preflight 扫 source BSL 上的 backup 列表, 区分 eligible vs rejected.
// 拦截规则 (基于 fixture 验证):
//   - snapshotMoveData=false AND defaultVolumesToFsBackup=false → 拦截
//     (CSI 快照型, 卷数据在云厂商, 不在 BSL)
//   - 其余 (data-mover / fs-backup / snapshotMoveData=true) → eligible
func Preflight(ctx context.Context, cl client.Client, req PreflightRequest) (*PreflightResult, error) {
	// 1. 验 BSL 存在
	sourceBSL := &velerov1.BackupStorageLocation{}
	if err := cl.Get(ctx, types.NamespacedName{Name: req.SourceBSL, Namespace: velerons.Namespace()}, sourceBSL); err != nil {
		return nil, fmt.Errorf("source BSL %q not found: %w", req.SourceBSL, err)
	}
	targetBSL := &velerov1.BackupStorageLocation{}
	if err := cl.Get(ctx, types.NamespacedName{Name: req.TargetBSL, Namespace: velerons.Namespace()}, targetBSL); err != nil {
		return nil, fmt.Errorf("target BSL %q not found: %w", req.TargetBSL, err)
	}

	// 2. 列 backup, 过滤 storageLocation=source
	list := &velerov1.BackupList{}
	if err := cl.List(ctx, list, client.InNamespace(velerons.Namespace())); err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}

	result := &PreflightResult{
		Eligible: []string{},
		Rejected: []RejectedBackup{},
	}

	nameFilter := toSet(req.Backups) // 空 set = 不限制

	for _, b := range list.Items {
		// 只看 source BSL 的 backup
		if b.Spec.StorageLocation != req.SourceBSL {
			continue
		}
		if len(nameFilter) > 0 && !nameFilter[b.Name] {
			continue
		}

		// 拦截规则: 快照型 (snapshotMoveData=false 且 无 fs-backup)
		snapshotMoveData := false
		if b.Spec.SnapshotMoveData != nil {
			snapshotMoveData = *b.Spec.SnapshotMoveData
		}
		defaultVolumesToFsBackup := false
		if b.Spec.DefaultVolumesToFsBackup != nil {
			defaultVolumesToFsBackup = *b.Spec.DefaultVolumesToFsBackup
		}

		if !snapshotMoveData && !defaultVolumesToFsBackup {
			// 真 CSI snapshot 型, 卷数据在云厂商区域快照, BSL 没有
			result.Rejected = append(result.Rejected, RejectedBackup{
				Name:       b.Name,
				Reason:     "snapshotMoveData=false 且无 fs-backup, 卷数据在云厂商区域快照不在 BSL, Layer 4 BSL→BSL 复制无法搬运",
				ErrorCode:  ErrLayer4SnapshotUnsupported,
				Suggestion: "在 Policy 配 snapshotMoveData=true 让卷数据进 BSL 的 Kopia 仓库 (data-mover), 或改用云厂商区域快照复制 API (另一机制, 见 ADR-031 §8)",
			})
			continue
		}
		result.Eligible = append(result.Eligible, b.Name)
	}

	result.Summary = PreflightSummary{
		EligibleCount: len(result.Eligible),
		RejectedCount: len(result.Rejected),
	}
	return result, nil
}

func toSet(arr []string) map[string]bool {
	s := make(map[string]bool, len(arr))
	for _, x := range arr {
		s[x] = true
	}
	return s
}
