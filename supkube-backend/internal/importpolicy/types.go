// Package importpolicy 实现 SupKube Import Policy controller + REST API.
//
// 背景 (PRD-Import / 2026-06-01 sprint):
//
//	多集群灾备场景: 源集群 A 把 backup tarball 写到 BSL (S3/Blob), DR 集群 B
//	需要发现这些 backup 并把 Velero `Backup` CR "导入"到自己的 Velero ns,
//	这样 B 才能基于这些 backup 触发 Restore. 没有 Import Policy 之前需要手动
//	`velero backup-location create + velero backup get + kubectl apply`, 4-7
//	步对 DR 演练很卡. 本模块提供声明式 ImportPolicy CR + 后台 controller 自动
//	同步.
//
// 关键不变量 (shared contract, sprint Agent A/B/C/D 必须一致):
//   - GVR: importpolicies.supkube.io/v1, namespace=supkube
//   - mode: Continuous (轮询 interval) | Scheduled (cron)
//   - fingerprintMode: enforce | warn | disabled (校验来源集群签名)
//   - phase: Active | Paused | Failed (status.phase)
//
// 本文件: ImportPolicy CRD Go 类型 + DeepCopy 手写实现 (我们不跑 codegen).
//
// kubebuilder validation markers 标注在字段上, 便于未来 controller-gen
// 自动生成 CRD YAML; 当前 CRD YAML 由 chart/ 维护, marker 仅作文档.
package importpolicy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ─────────────────────────────────────────────────────────────────────
// GVR / GVK constants
// ─────────────────────────────────────────────────────────────────────

const (
	// GroupName 是 ImportPolicy 的 API group.
	GroupName = "supkube.io"
	// Version 是 ImportPolicy 的 API version (v1, 不带 alpha/beta —
	// 直接 stable; schema 通过 ADR-035 风格错误码兜底破坏性变更).
	Version = "v1"
	// Resource 是 plural (importpolicies).
	Resource = "importpolicies"
	// Kind 是单数 PascalCase.
	Kind = "ImportPolicy"
	// Namespace 是 ImportPolicy CR 默认所在 ns. 与 Cluster CR / SupKube
	// settings 同 ns, 便于 RBAC 一把锁定.
	Namespace = "supkube"
)

// GVR 是 dynamic client 用的 schema.GroupVersionResource.
var GVR = schema.GroupVersionResource{Group: GroupName, Version: Version, Resource: Resource}

// GVK 是 typed scheme 用的 schema.GroupVersionKind.
var GVK = schema.GroupVersionKind{Group: GroupName, Version: Version, Kind: Kind}

// ─────────────────────────────────────────────────────────────────────
// Enum string types (固定字面值, 与 SHARED CONTRACT 严格对齐)
// ─────────────────────────────────────────────────────────────────────

// ImportMode 决定 controller 怎么触发 syncOnce.
//   - Continuous: 按 spec.continuousInterval 轮询
//   - Scheduled: 按 spec.schedule (cron 5-field) 触发
type ImportMode string

const (
	ImportModeContinuous ImportMode = "Continuous"
	ImportModeScheduled  ImportMode = "Scheduled"
)

// FingerprintMode 决定 fingerprint validator 不通过时的处理方式.
//   - enforce: 拒绝导入, 计入 status.rejectedCount + 写 lastError
//   - warn:    仍然导入但 label fingerprint-status=missing|invalid
//   - disabled: 完全跳过 fingerprint 校验 (label 也不写)
type FingerprintMode string

const (
	FingerprintModeEnforce  FingerprintMode = "enforce"
	FingerprintModeWarn     FingerprintMode = "warn"
	FingerprintModeDisabled FingerprintMode = "disabled"
)

// Phase 是 status.phase 的合法取值.
type Phase string

const (
	PhaseActive Phase = "Active"
	PhasePaused Phase = "Paused"
	PhaseFailed Phase = "Failed"
)

// ─────────────────────────────────────────────────────────────────────
// CRD Go types
// ─────────────────────────────────────────────────────────────────────

// ImportPolicy 是 supkube.io/v1 ImportPolicy CR 的 Go 表示.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ip
type ImportPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImportPolicySpec   `json:"spec,omitempty"`
	Status ImportPolicyStatus `json:"status,omitempty"`
}

// ImportPolicySpec 是 desired state. 全字段都 JSON-omit-empty, 默认值在
// controller 入口处 normalize (defaultsAndValidate) — 简化 CRD YAML.
type ImportPolicySpec struct {
	// SourceBSL 是 Velero BackupStorageLocation 的 name (必须在 velero ns
	// 已存在). 校验在 handlers POST/PUT 入口做; controller 在每次 sync 重读
	// (BSL 可能在 CR 创建后才被 admin 加上).
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SourceBSL string `json:"sourceBSL"`

	// Mode 决定触发节奏 (Continuous=ticker, Scheduled=cron).
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Continuous;Scheduled
	Mode ImportMode `json:"mode"`

	// ContinuousInterval 是 Continuous 模式下的轮询间隔, Go duration 字符串
	// (e.g. "60s", "5m"). 默认 "60s", 最小 "30s" (低于会被 handler 拒绝
	// 返回 ERR_IMPORTPOLICY_INTERVAL_TOO_SHORT, 防止 API server 被打爆).
	//
	// +kubebuilder:validation:Pattern=`^[0-9]+(ns|us|µs|ms|s|m|h)$`
	ContinuousInterval string `json:"continuousInterval,omitempty"`

	// Schedule 是 Scheduled 模式下的 5-field cron 表达式 (与 Velero
	// Schedule.spec.schedule 同语法, robfig/cron/v3 解析).
	Schedule string `json:"schedule,omitempty"`

	// SourceClusterID 是 fingerprint result 里 sourceClusterID 的过滤器.
	// 空 = 不过滤 (接受任何来源); 非空 = 只导入匹配的 backup. 用于一个
	// BSL 被多集群共享时, 把不同来源拆给不同 ImportPolicy.
	SourceClusterID string `json:"sourceClusterID,omitempty"`

	// FingerprintMode 见 FingerprintMode 文档. 默认 enforce — 这是
	// 安全默认, admin 主动放宽时才走 warn/disabled.
	//
	// +kubebuilder:validation:Enum=enforce;warn;disabled
	FingerprintMode FingerprintMode `json:"fingerprintMode,omitempty"`

	// TargetVeleroNamespace 是写入 Velero Backup CR 的 ns, 默认 "velero".
	// 多 Velero 实例场景 (不常见) 可指向其他 ns.
	TargetVeleroNamespace string `json:"targetVeleroNamespace,omitempty"`

	// Paused 为 true 时 controller 跳过 sync 并把 status.phase=Paused.
	// 不删 CR 也不重启 goroutine — handler 的 pause/resume 通过 patch 这个
	// 字段实现.
	Paused bool `json:"paused,omitempty"`
}

// ImportPolicyStatus 是 controller 写入的 observed state. 全字段
// omit-empty, controller 用 mergePatch 写, 不会清空 admin 写的其他字段.
type ImportPolicyStatus struct {
	// Phase 是 controller 当前状态 (Active/Paused/Failed).
	Phase Phase `json:"phase,omitempty"`

	// LastSyncAt 是上一次 syncOnce 成功完成时间 (RFC3339).
	LastSyncAt string `json:"lastSyncAt,omitempty"`

	// LastSeenBackupName 是 watermark — 下次 syncOnce 只看比这个新的
	// backup. 由 controller 按 backup 名字 ASC 排序更新.
	LastSeenBackupName string `json:"lastSeenBackupName,omitempty"`

	// ImportedCount 是累计成功导入数. 不区分 enforce/warn/disabled 模式
	// (warn 模式下 fingerprint 失败的 backup 也算导入成功).
	ImportedCount int `json:"importedCount,omitempty"`

	// RejectedCount 是累计被拒数. enforce 模式下 fingerprint 失败 +
	// 任何模式下创建 Velero Backup CR 失败.
	RejectedCount int `json:"rejectedCount,omitempty"`

	// LastError 是上一次失败的人话信息 (sync 整体失败时写, 单条 backup
	// 拒绝不写此字段, 避免 log 噪音). 成功 sync 会清空.
	LastError string `json:"lastError,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────
// List wrapper
// ─────────────────────────────────────────────────────────────────────

// ImportPolicyList 是 typed client 的 List 返回. 字段命名按 k8s 习惯.
//
// +kubebuilder:object:root=true
type ImportPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImportPolicy `json:"items"`
}

// ─────────────────────────────────────────────────────────────────────
// DeepCopy methods (手写, 因为我们不跑 deepcopy-gen)
// ─────────────────────────────────────────────────────────────────────
//
// 这些方法实现 runtime.Object 接口, 允许 ImportPolicy 走 typed client +
// controller-runtime scheme. 模式 1:1 复制 deepcopy-gen 生成的代码.

// DeepCopyInto 把 in 深拷到 out (out 必须非 nil).
func (in *ImportPolicy) DeepCopyInto(out *ImportPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec // Spec 全是 value type, 浅拷即深拷
	out.Status = in.Status
}

// DeepCopy 返回一份深拷贝.
func (in *ImportPolicy) DeepCopy() *ImportPolicy {
	if in == nil {
		return nil
	}
	out := new(ImportPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject 实现 runtime.Object.
func (in *ImportPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto for List.
func (in *ImportPolicyList) DeepCopyInto(out *ImportPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ImportPolicy, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy for List.
func (in *ImportPolicyList) DeepCopy() *ImportPolicyList {
	if in == nil {
		return nil
	}
	out := new(ImportPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject for List.
func (in *ImportPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
