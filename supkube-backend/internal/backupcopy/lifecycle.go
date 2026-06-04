// lifecycle.go — PRD-007 §4.3 Layer 4 lifecycle 冲突预检.
//
// 范围 (Gate 0):
//   - LifecyclePreflight: 检测 Velero Policy retention (source 侧 backup TTL) 与
//     Layer 4 target BSL lifecycle delete 时间窗的冲突
//   - 现已实现: retention 时间窗冲突 (TC-COPY-003); Object-Lock vs lifecycle-delete (PRD §4.5 §校验 1)
//     留 follow-up (跟 §4.5 lifecycle Apply 路径同 owner, 不在本任务范围)
//
// 错误码 (ADR-035 体系, ERR_LIFECYCLE_* family):
//   - ERR_LIFECYCLE_RETENTION_CONFLICT — Velero Policy backup TTL > Layer 4 target delete-after
//     (导致复制过去的备份在 target 上比 source 早删 → 失去 Layer 4 冗余意义)
//
// **拟错误码警告 (等待决策.md D-WAIT-COPY-001)**:
// ERR_LIFECYCLE_RETENTION_CONFLICT 是本 task 新拟; 跟 PRD §4.5 已有的 ERR_LIFECYCLE_LOCK_CONFLICT
// 是不同语义 (一个 retention 时间窗, 一个 WORM 不可删冲突), Mars 决策是否独立成 code 或合并.
package backupcopy

import (
	"context"
	"fmt"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// ErrLifecycleRetentionConflict PRD-007 §4.3 Layer 4 + §4.5 Lifecycle 联动:
// Velero Policy backup TTL (source 保留期) > Layer 4 target BSL lifecycle delete-after (目标桶 delete 时间窗)
// → 复制过去的备份在 target 上比 source 早删, 跨云冗余被静默削弱.
const ErrLifecycleRetentionConflict = "ERR_LIFECYCLE_RETENTION_CONFLICT"

// TargetLifecycleRule 表达 Layer 4 target BSL 已配置的 lifecycle 规则关键字段.
// 现仅取 DeleteAfter (PRD §4.5 同 prefix 多 transition 留 §4.5 owner 实施).
// 实际值由 controller 在 apply 前通过云 SDK 读 BSL 当前 lifecycle 状态填充.
type TargetLifecycleRule struct {
	// Prefix BSL 内 prefix 范围 (e.g. "kopia/" 或 ""=整桶).
	Prefix string
	// DeleteAfter 该 prefix 下对象保留期, 0 = 不删.
	DeleteAfter time.Duration
}

// LifecyclePreflightRequest 是 Layer 4 lifecycle 冲突预检入参.
type LifecyclePreflightRequest struct {
	// Schedule 是 Velero Schedule CR (从中读 spec.template.ttl 当 retention).
	// 若 nil 则用 PolicyRetention 字段 (允许调用方在 SupKube Policy CRD 层把 retention 抽取出来后再传).
	Schedule *velerov1.Schedule
	// PolicyRetention 是显式 retention (与 Schedule.Spec.Template.TTL 等价).
	// Schedule != nil 时优先用 Schedule.Spec.Template.TTL.
	PolicyRetention time.Duration
	// TargetRules 是 target BSL 已配 lifecycle 规则 (由 controller 从云 SDK 读出).
	TargetRules []TargetLifecycleRule
	// TargetBSLName 仅用于错误消息.
	TargetBSLName string
}

// LifecyclePreflightResult 预检结果.
type LifecyclePreflightResult struct {
	OK       bool                  `json:"ok"`
	Conflict *LifecycleConflictDoc `json:"conflict,omitempty"`
}

// LifecycleConflictDoc 单条冲突详情, 喂给 UI 显示.
type LifecycleConflictDoc struct {
	ErrorCode       string        `json:"errorCode"`
	Reason          string        `json:"reason"`
	Suggestion      string        `json:"suggestion"`
	SourceRetention time.Duration `json:"sourceRetention"`
	TargetDelete    time.Duration `json:"targetDelete"`
	TargetPrefix    string        `json:"targetPrefix"`
}

// LifecyclePreflight 检查 source retention 是否被 target lifecycle delete 时间窗"提前删除".
//
// 触发规则:
//   - 任意 target rule 的 DeleteAfter > 0 且 DeleteAfter < sourceRetention → 冲突
//     (target 上备份比 source 早删 → 失去 Layer 4 冗余意义)
//   - DeleteAfter == 0 (永不删) 不冲突
//   - sourceRetention == 0 (永不删) → 若 target 有任意 DeleteAfter, 视为冲突 (source 永久 vs target 有限)
//
// 返回:
//   - OK=true → 无冲突
//   - OK=false → Conflict 字段含 ERR_LIFECYCLE_RETENTION_CONFLICT 详情
//
// 设计说明: 本函数不读 K8s / 不调云 SDK, 纯计算; controller 负责把 Velero CR / 云 SDK 读到的
// 状态填进 Request, 然后调本函数. 单测因此不需要 fake client (跟 preflight.go 风格略不同; 后者
// 直接读 CR list 因为它的语义就是"扫 BSL 上 backup").
func LifecyclePreflight(ctx context.Context, req LifecyclePreflightRequest) (*LifecyclePreflightResult, error) {
	_ = ctx // 现纯计算无 IO, 但保留参数便于未来加入云 SDK 读取

	sourceRetention := req.PolicyRetention
	if req.Schedule != nil && req.Schedule.Spec.Template.TTL.Duration > 0 {
		sourceRetention = req.Schedule.Spec.Template.TTL.Duration
	}

	// 找最严苛 (最短) 的 target delete 规则 — 它会决定 target 上备份何时被删
	var worst *TargetLifecycleRule
	for i := range req.TargetRules {
		r := &req.TargetRules[i]
		if r.DeleteAfter <= 0 {
			continue
		}
		if worst == nil || r.DeleteAfter < worst.DeleteAfter {
			worst = r
		}
	}

	// 无 target delete 规则 → 无冲突
	if worst == nil {
		return &LifecyclePreflightResult{OK: true}, nil
	}

	// source 永不删 (retention=0) vs target 有限 → 冲突
	if sourceRetention == 0 {
		return &LifecyclePreflightResult{
			OK: false,
			Conflict: &LifecycleConflictDoc{
				ErrorCode: ErrLifecycleRetentionConflict,
				Reason: fmt.Sprintf(
					"source Policy retention=∞ (永不删), 但 target BSL %q lifecycle 在 %s 后删除 prefix=%q 对象 — Layer 4 冗余将在该窗口后失效",
					req.TargetBSLName, worst.DeleteAfter, worst.Prefix,
				),
				Suggestion: fmt.Sprintf(
					"调高 target lifecycle delete (≥ source retention) 或调低 source Policy retention (≤ target delete=%s); 若客户期望就是 target 短保留, 请在 UI 明示并确认",
					worst.DeleteAfter,
				),
				SourceRetention: sourceRetention,
				TargetDelete:    worst.DeleteAfter,
				TargetPrefix:    worst.Prefix,
			},
		}, nil
	}

	// 正常对比: target 比 source 严格更短 → 冲突
	if worst.DeleteAfter < sourceRetention {
		return &LifecyclePreflightResult{
			OK: false,
			Conflict: &LifecycleConflictDoc{
				ErrorCode: ErrLifecycleRetentionConflict,
				Reason: fmt.Sprintf(
					"source Policy retention=%s 大于 target BSL %q lifecycle delete=%s (prefix=%q) — Layer 4 复制过去的备份会比 source 早删 %s",
					sourceRetention, req.TargetBSLName, worst.DeleteAfter, worst.Prefix,
					sourceRetention-worst.DeleteAfter,
				),
				Suggestion: fmt.Sprintf(
					"调高 target lifecycle delete (≥ %s) 或调低 source Policy retention (≤ %s); 若 target 是冷归档 BSL 则在 UI 明示客户接受短保留",
					sourceRetention, worst.DeleteAfter,
				),
				SourceRetention: sourceRetention,
				TargetDelete:    worst.DeleteAfter,
				TargetPrefix:    worst.Prefix,
			},
		}, nil
	}

	return &LifecyclePreflightResult{OK: true}, nil
}
