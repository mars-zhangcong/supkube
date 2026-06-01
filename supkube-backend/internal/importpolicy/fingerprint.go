// fingerprint.go — Validator interface seam.
//
// Agent C (sprint 2026-06-01) 实现具体的 fingerprint 校验 (读 tarball 旁
// 的 fingerprint.json, 验 HMAC, 解析 sourceClusterID/name). 本文件只定义
// interface, 让 controller / 单测可以 mock.
//
// 把 interface 放在 importpolicy 包而不是单独的 fingerprint 包, 是为了
// 避免循环依赖 (FingerprintMode 已经在 types.go 定义). Agent C 会在
// internal/fingerprint/ 实现一个 satisfies-this-interface 的 type, 然后
// cmd/server/server.go 在启动时注入.
package importpolicy

import "context"

// Validator 是 fingerprint 校验的 seam. 给 controller 一个抽象的"我能不
// 能信这个 backup"接口, 不在乎背后是 HMAC / signature / 啥都不校验.
//
// 实现注意:
//   - 必须线程安全 (controller 多 ImportPolicy 并发调用).
//   - mode=disabled 时建议直接返回 Status="valid" + 空 reason, 不要做任何
//     I/O (controller 信任此返回, 不会再二次判断).
//   - 网络错误 / S3 拉 fingerprint.json 超时, 返回 error (上层 controller
//     会当作 sync 整体失败处理 + 写 lastError); 不要把网络错误塞进
//     FingerprintResult.Reason — 那是给 valid/missing/invalid 这三种"业务
//     结论"留的.
type Validator interface {
	ValidateBackup(ctx context.Context, bslName string, backupName string, mode FingerprintMode) (*FingerprintResult, error)
}

// FingerprintResult 是 Validator 的业务结论.
//
// Status 取值:
//   - "valid":   fingerprint.json 存在 + HMAC 校验通过 (或 mode=disabled)
//   - "missing": fingerprint.json 不存在 (老 backup / 非 SupKube 写入)
//   - "invalid": fingerprint.json 存在但 HMAC 不匹配 (篡改 / key 漂移)
//
// SourceClusterID/Name 仅在 Status=="valid" 时可信; missing/invalid 时
// 字段可能为空也可能是 best-effort 解析的值, controller 不应依赖.
type FingerprintResult struct {
	Status            string `json:"status"`
	SourceClusterID   string `json:"sourceClusterID,omitempty"`
	SourceClusterName string `json:"sourceClusterName,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

// IsValid 是个语法糖, 简化 controller 里的判断.
func (r *FingerprintResult) IsValid() bool {
	return r != nil && r.Status == "valid"
}

// ─────────────────────────────────────────────────────────────────────
// BackupLister — 列 BSL 上有哪些 backup tarball
// ─────────────────────────────────────────────────────────────────────

// BackupLister 是另一个 seam: 给 controller 一个抽象的"BSL 上有哪些
// backup 名" 接口. 真实实现走 S3/Azure ListObjects (复用 internal/api/v1/
// backup_meta_bsl.go 的代码), 单测用 in-memory map 就够.
//
// 返回的 name 列表必须按字典序 ASC 排好 — controller 据此和
// status.lastSeenBackupName 比对决定哪些是"新"backup. 由 lister 排序
// (而不是 controller 排) 是因为底层 S3 ListObjectsV2 本来就是有序的,
// 不要做无谓的二次排序.
type BackupLister interface {
	ListBackups(ctx context.Context, bslName string) ([]string, error)
}

// ─────────────────────────────────────────────────────────────────────
// BackupImporter — 把发现的 backup 写成 Velero Backup CR
// ─────────────────────────────────────────────────────────────────────

// BackupImporter 是第三个 seam: "已知这个 backup 通过校验, 把它写到本
// 集群 Velero ns 当一个 Backup CR". 抽出 interface 主要为了测试; 生产
// 实现走 controller-runtime client 创建 velerov1.Backup.
//
// labels 由 controller 传入 — 包含 supkube.io/imported=true,
// supkube.io/source-cluster, supkube.io/fingerprint-status. Importer 不
// 该擅自加/改 label, 那是 controller 的事.
type BackupImporter interface {
	ImportBackup(ctx context.Context, targetNs, bslName, backupName string, labels map[string]string) error
}
