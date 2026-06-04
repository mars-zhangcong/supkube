// transport.go — PRD-007 §4.3 Layer 4 Backup Copy "真搬运" 引擎.
//
// 范围 (Gate 0):
//   - 仓库级 sync 抽象 (Transport interface), 默认实现 RcloneRepoSync (rclone CLI wrapper)
//   - 单元测试用 fakeTransport (不依赖 rclone 二进制), E2E 实测留 Phase 0 task
//
// 设计要点 (与 PRD-007 §4.3 v1.1 锁档对齐):
//   - **不是** per-backup 挑对象复制 (草稿写法, 已 v1.1 砍掉)
//   - **是** ns 级仓库 sync: `kopia/<ns>/` (卷数据) + `backups/<name>/` (元数据)
//   - 增量性来自 Kopia 内容寻址 + rclone 算法, 不重新备份
//   - 拦截快照型 BSL (BSL.Spec.Config["volumeDataSource"] == "snapshot") → ERR_LAYER4_SNAPSHOT_UNSUPPORTED
//     (PRD-007 §4.3 拦截已在 preflight.go 按 backup 粒度做, 本 Transport 再加一层 BSL 粒度防御)
//
// 错误码 (ADR-035 体系, ERR_LAYER4_* family):
//   - ERR_LAYER4_SNAPSHOT_UNSUPPORTED — 复用 preflight.go 同名常量
//   - ERR_LAYER4_BSL_UNREACHABLE — 源/目标 BSL 网络不可达
//   - ERR_LAYER4_TRANSFER_FAILED — sync 过程失败 (非网络)
//
// 实证基础: engineer-testing/fixtures/velero-real-2026-05-31-060756/ (3 BSL × 21 backup 真 fixture)
package backupcopy

import (
	"context"
	"errors"
	"fmt"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PRD-007 §4.3 + ADR-035 错误码 (Layer 4 transport 子集).
const (
	// ErrLayer4BSLUnreachable 源 / 目标 BSL 网络不可达 (TC-COPY-005).
	ErrLayer4BSLUnreachable = "ERR_LAYER4_BSL_UNREACHABLE"
	// ErrLayer4TransferFailed sync 过程非网络错 (rclone exit code 非 0 等).
	ErrLayer4TransferFailed = "ERR_LAYER4_TRANSFER_FAILED"
)

// SnapshotBSLConfigKey 在 BSL.Spec.Config 中标记 "本 BSL 仅含快照指针, 无卷数据".
// 客户在 BSL 配置时显式设此 key (e.g. config: { volumeDataSource: "snapshot" }) 表示
// 该 BSL 不是 Layer 4 合格源——卷数据在云厂商区域快照, BSL→BSL object copy 会丢数据.
//
// **设计澄清 (等待决策.md D-WAIT-COPY-002 待 Mars 拍板)**:
// PRD-007 §4.3 把"快照型"判定锚在 *Backup CR* 的 snapshotMoveData=false (已实现在 preflight.go);
// 本 Transport.Copy 再加一层 *BSL 级* 防御 (config marker), 防止用户绕过 preflight 直接调 Copy.
// 现暂用 config key "volumeDataSource" 值 "snapshot" 作为约定; Mars 可改 schema 后回写.
const SnapshotBSLConfigKey = "volumeDataSource"

// SnapshotBSLConfigValue 标记值.
const SnapshotBSLConfigValue = "snapshot"

// CopyRequest 是 BSL→BSL 仓库级 sync 入参.
type CopyRequest struct {
	Source     *velerov1.BackupStorageLocation // 源 BSL CR
	Target     *velerov1.BackupStorageLocation // 目标 BSL CR
	Namespaces []string                        // 复制范围 (Kopia 仓库按 ns 共享)
	// RateLimitMBps 速率限制 (PRD-007 §4.3 默认 100, UI 可配 10-1000).
	RateLimitMBps int
}

// CopyProgress 是 sync 过程进度 / 结果.
type CopyProgress struct {
	BytesTransferred int64    `json:"bytesTransferred"`
	ObjectsCopied    int      `json:"objectsCopied"`
	NamespacesSynced []string `json:"namespacesSynced"`
	// Phase = "Preflight" | "Discovery" | "Transferring" | "Verifying" | "Completed" (PRD-006 ActionType=BackupCopy 联动).
	Phase string `json:"phase"`
}

// Transport 抽象仓库级 sync. v1.0 默认 rclone (PRD-007 §4.3 锁档), 未来可换云原生引擎
// (S3 CRR / Azure OR / GCS Replication) — 跟 ADR-035 错误码体系解耦.
type Transport interface {
	// Copy 执行 BSL→BSL 仓库级 sync. 返回 progress 或错误.
	// 错误码 (TransportError.Code) 由调用方映射到 ADR-035 ERR_LAYER4_* 体系.
	Copy(ctx context.Context, req CopyRequest) (*CopyProgress, error)
}

// TransportError 是 Transport 层错误, 含 ADR-035 错误码 + 人话原因.
// 调用方 (controller / REST) 转 HTTP/UI 错误模型时直接用 Code 字段.
type TransportError struct {
	Code    string // ADR-035 ERR_LAYER4_*
	Message string
	// Cause 原始错误 (e.g. exec.ExitError / net.OpError); 仅作 debug 字段, 不入 UI.
	Cause error
}

// Error 满足 error 接口.
func (e *TransportError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 暴露底层错误供 errors.Is/As.
func (e *TransportError) Unwrap() error { return e.Cause }

// ValidateBSL 检查 BSL 是否合格做 Layer 4 sync 源/目标.
// 拦截:
//   - BSL.Spec.Config["volumeDataSource"]="snapshot" → ERR_LAYER4_SNAPSHOT_UNSUPPORTED
//   - BSL.Spec.ObjectStorage 为空 → ERR_LAYER4_TRANSFER_FAILED (config 不全)
//
// 调用方在 Copy() 顶部用本函数对源/目标各跑一次, 防 preflight 被绕过.
func ValidateBSL(bsl *velerov1.BackupStorageLocation, role string) *TransportError {
	if bsl == nil {
		return &TransportError{
			Code:    ErrLayer4TransferFailed,
			Message: fmt.Sprintf("%s BSL is nil", role),
		}
	}
	if bsl.Spec.ObjectStorage == nil || bsl.Spec.ObjectStorage.Bucket == "" {
		return &TransportError{
			Code:    ErrLayer4TransferFailed,
			Message: fmt.Sprintf("%s BSL %q has no objectStorage.bucket", role, bsl.Name),
		}
	}
	if v := bsl.Spec.Config[SnapshotBSLConfigKey]; v == SnapshotBSLConfigValue {
		return &TransportError{
			Code: ErrLayer4SnapshotUnsupported,
			Message: fmt.Sprintf(
				"%s BSL %q 标记为 snapshot 型 (config.%s=%s), 卷数据在云厂商区域快照不在 BSL, Layer 4 BSL→BSL 复制无法搬运 — 应配 snapshotMoveData=true 或改用云厂商区域快照复制 API",
				role, bsl.Name, SnapshotBSLConfigKey, SnapshotBSLConfigValue,
			),
		}
	}
	return nil
}

// asTransportError 把任意 error 包装成 TransportError. 已是 TransportError 透传.
func asTransportError(code, msg string, cause error) *TransportError {
	var te *TransportError
	if errors.As(cause, &te) {
		return te
	}
	return &TransportError{Code: code, Message: msg, Cause: cause}
}

// ---- 默认实现: RcloneRepoSync (rclone CLI wrapper) ----

// RcloneRunner 抽象 rclone 二进制调用, 便于测试注入 fake. 真实现在生产代码用 os/exec.
type RcloneRunner interface {
	// Sync 等价于 `rclone sync <src> <dst> --bwlimit=<rate> ...`. 返回字节数 / 对象数.
	Sync(ctx context.Context, src, dst string, rateLimitMBps int) (bytes int64, objects int, err error)
}

// RcloneRepoSync 是 PRD-007 §4.3 v1.0 默认 Transport: rclone 仓库级 sync.
//
// 复制单元 (PRD-007 §4.3 锁档):
//
//	source: <src-bsl>/kopia/<ns>/ + <src-bsl>/backups/<name>/
//	target: <dst-bsl>/kopia/<ns>/ + <dst-bsl>/backups/<name>/
//
// 增量性来自 Kopia 内容寻址 (chunk 已存在则零成本跳过) + rclone hash 比对.
type RcloneRepoSync struct {
	Runner RcloneRunner
}

// NewRcloneRepoSync 用注入的 runner 构造 Transport. runner=nil 时 Copy() 返 transfer_failed.
func NewRcloneRepoSync(runner RcloneRunner) *RcloneRepoSync {
	return &RcloneRepoSync{Runner: runner}
}

// Copy 实现 Transport. 单 ns 多 prefix sync, 任一 prefix 失败即 fail-fast.
//
// 拦截顺序 (TC-COPY-001/002/004/005 覆盖):
//  1. ValidateBSL(source) + ValidateBSL(target) — snapshot 型立即拒
//  2. Runner.Sync() 每个 ns 跑 2 个 prefix (kopia/<ns>/ + backups/<name>/)
//  3. Runner 报网络错 → ERR_LAYER4_BSL_UNREACHABLE; 其他错 → ERR_LAYER4_TRANSFER_FAILED
func (r *RcloneRepoSync) Copy(ctx context.Context, req CopyRequest) (*CopyProgress, error) {
	// (1) BSL 合规性 (TC-COPY-002 snapshot 拦截)
	if te := ValidateBSL(req.Source, "source"); te != nil {
		return nil, te
	}
	if te := ValidateBSL(req.Target, "target"); te != nil {
		return nil, te
	}
	if r.Runner == nil {
		return nil, &TransportError{
			Code:    ErrLayer4TransferFailed,
			Message: "no rclone runner configured",
		}
	}
	if len(req.Namespaces) == 0 {
		return nil, &TransportError{
			Code:    ErrLayer4TransferFailed,
			Message: "no namespaces specified for Layer 4 copy",
		}
	}

	progress := &CopyProgress{
		Phase:            "Transferring",
		NamespacesSynced: []string{},
	}

	// (2) 按 ns 仓库级 sync — Kopia 仓库 + Velero backup 元数据 prefix
	for _, ns := range req.Namespaces {
		kopiaSrc := bslRepoURL(req.Source, "kopia/"+ns+"/")
		kopiaDst := bslRepoURL(req.Target, "kopia/"+ns+"/")
		bytes, objs, err := r.Runner.Sync(ctx, kopiaSrc, kopiaDst, req.RateLimitMBps)
		if err != nil {
			return progress, classifyRunnerError(err)
		}
		progress.BytesTransferred += bytes
		progress.ObjectsCopied += objs

		// 元数据 prefix (按 ns prefix 一并搬, 上层 controller 按 backup-name 过滤)
		metaSrc := bslRepoURL(req.Source, "backups/")
		metaDst := bslRepoURL(req.Target, "backups/")
		bytes2, objs2, err2 := r.Runner.Sync(ctx, metaSrc, metaDst, req.RateLimitMBps)
		if err2 != nil {
			return progress, classifyRunnerError(err2)
		}
		progress.BytesTransferred += bytes2
		progress.ObjectsCopied += objs2

		progress.NamespacesSynced = append(progress.NamespacesSynced, ns)
	}

	progress.Phase = "Completed"
	return progress, nil
}

// bslRepoURL 拼 rclone remote URL: `<provider>:<bucket>/<prefix>`.
// (rclone remote 名按 ADR-004 凭据管理预注册; 本函数只负责 URL 字符串拼接)
func bslRepoURL(bsl *velerov1.BackupStorageLocation, prefix string) string {
	bucket := bsl.Spec.ObjectStorage.Bucket
	bslPrefix := bsl.Spec.ObjectStorage.Prefix
	full := bucket
	if bslPrefix != "" {
		full = bucket + "/" + bslPrefix
	}
	return fmt.Sprintf("%s:%s/%s", bsl.Name, full, prefix)
}

// classifyRunnerError 把 runner 返的 error 映射到 ADR-035 错误码.
// 约定: runner 实现把网络错包装成 *NetworkError 子类 (见 RunnerNetworkError 哨兵).
func classifyRunnerError(err error) *TransportError {
	if errors.Is(err, ErrRunnerNetwork) {
		return &TransportError{
			Code:    ErrLayer4BSLUnreachable,
			Message: "rclone runner failed to reach BSL",
			Cause:   err,
		}
	}
	// 已经是 TransportError 透传 (e.g. runner 自己分级)
	var te *TransportError
	if errors.As(err, &te) {
		return te
	}
	return asTransportError(ErrLayer4TransferFailed, "rclone runner reported failure", err)
}

// ErrRunnerNetwork 哨兵错误: runner 实现把网络错 wrap 此 sentinel (e.g. fmt.Errorf("%w: %v", ErrRunnerNetwork, x))
// 让 classifyRunnerError 映射到 ERR_LAYER4_BSL_UNREACHABLE 而不是 ERR_LAYER4_TRANSFER_FAILED.
var ErrRunnerNetwork = errors.New("rclone runner: network unreachable")
