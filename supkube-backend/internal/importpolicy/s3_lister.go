// s3_lister.go — 真正驱动 ImportPolicy RPO 的 BackupLister 实现.
//
// 为什么需要这个文件 (sprint 2026-06-01 Agent G)
// ───────────────────────────────────────────────
// runtime_impl.go::veleroBackupLister 是个"穷人版"实现 — 它只是查 Velero
// 自己已经 sync 进来的 Backup CR. 但 Velero BackupSyncController 默认
// backupSyncPeriod=60s (有时被 admin 配到 90s/2m). 那就意味着 ImportPolicy
// 即便 continuousInterval=30s, 它发现新 backup 的"地板时间"也只能是 Velero
// sync 周期 — 我们 PRD 承诺 "击败 Kasten 5min RPO" 就成了空头支票.
//
// 这里 s3BackupLister 直接 LIST BSL 上的 <prefix>/backups/ 目录,
// 通过寻找 <name>/velero-backup.json marker 来识别 backup. 这跟 Velero
// 自己的 BackupSyncController 走的是同一个数据源, 但我们用 ImportPolicy
// 的轮询节奏, 把 RPO 还给用户.
//
// 与 Velero sync 的协作 (而非互斥)
// ────────────────────────────────
// 我们 NOT 关闭 Velero sync — 它是"安全网". 两个 actor 都可能同时尝试
// create 同名 Backup CR. 解决:
//  1. List 阶段 dedupe — 已经在 Velero ns 里存在的 Backup, lister 不
//     返回, 让 controller 经过 import 流程的开销;
//  2. Import 阶段 IsAlreadyExists 兜底 — race 情况下谁创谁赢, 另一边
//     no-op (s3_importer.go 也实现了这一点).
//
// 不复用 internal/api/v1/backup_meta_bsl.go 的原因
// ────────────────────────────────────────────────
// v1 包里的 S3/Azure 代码是 List+Hydrate 一起做 size 缓存的特化逻辑,
// 而且字段 unexport 的多, 引进来会拉一堆 lru / time helpers, 反而比直
// 接复用 fingerprint.BSLObjectClient 多. fingerprint 包 (Agent C) 已经
// 暴露了 GetObject/PutObject/Prefix, 这次 sprint 我们给它加了
// ListPrefix —— ~40 LOC AWS + 40 LOC Azure, 都是 sister 函数 (跟它已
// 经在做的 GetObject/PutObject 共享一切 auth / client 构造).
package importpolicy

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/supkube/supkube-backend/internal/fingerprint"
)

// veleroBackupManifestFile 是 Velero v1 在每个 backup 目录下写的"主清单"
// 文件名. 它的存在 == 一个完成的 backup 上传. 用这个作为 marker 比用
// .tar.gz 后缀更鲁棒 — 有些 BSL 会写多种文件 (kubeconfig / volumes /
// metadata), 用 .tar.gz 会把同一个 backup 算多次或漏掉 (e.g. CSI-only
// backup 没 tarball 但有 manifest).
const veleroBackupManifestFile = "velero-backup.json"

// backupsSubdir 是 Velero 写入 backup 的二级路径 — 完整 key 长得像
// "<bslPrefix>/backups/<backupName>/velero-backup.json".
const backupsSubdir = "backups"

// s3BackupLister 是 BackupLister 的真 S3/Azure 实现. 通过 BSL 的
// ListPrefix 找出 BSL 上所有 backup 名 (按 manifest 文件存在判定), 再
// dedupe 掉 Velero CR 里已存在的, 返回给 controller "纯新发现"的列表.
//
// 字段都 exported 是因为 cmd/server/server.go 可能想直接初始化时拿到
// 内部组件 (e.g. 测试 BSL 连通性). 但生产路径只走 ListBackups.
type s3BackupLister struct {
	BSL fingerprint.BSLObjectClient // 复用 Agent C 的 client (S3+Azure auth)
	// VeleroBackups 用来 dedupe — 已经在 Velero ns 里的 Backup CR 不需要
	// 我们再 import. 复用现有 veleroBackupLister 作为 wrapper (它本来就
	// 是查 Velero CR), 也可以传 nil 完全不 dedupe (仅测试 / debug 用).
	VeleroBackups BackupLister
}

// NewS3BackupLister 生产构造函数. 给 cmd/server/server.go 用.
//
// 注意 veleroBackups 参数: 强烈建议传一个 NewVeleroBackupLister(cli) —
// 这是 dedup 的关键. 测试可以传 nil 来跳过 dedup.
func NewS3BackupLister(bsl fingerprint.BSLObjectClient, veleroBackups BackupLister) BackupLister {
	return &s3BackupLister{BSL: bsl, VeleroBackups: veleroBackups}
}

// ListBackups 实现 BackupLister.
//
// 算法:
//  1. ListPrefix(<bslPrefix>/backups/) 拿到 BSL 上所有 object key;
//  2. 扫一遍, 凡是 key 形如 ".../<name>/velero-backup.json" 的, 把 name
//     收进 candidate set;
//  3. 如果有 veleroBackups dedupe, 把它返回的 names 减去;
//  4. 排序 ASC, 返回.
//
// Pagination 在 BSLObjectClient.ListPrefix 内部完成, 这里看到的就是完
// 整 list.
func (l *s3BackupLister) ListBackups(ctx context.Context, bslName string) ([]string, error) {
	if l.BSL == nil {
		return nil, fmt.Errorf("s3BackupLister: BSL client is nil (wiring bug)")
	}
	// keyPrefix 是 "backups/", BSLObjectClient.ListPrefix 内部会把
	// BSL 的 configured prefix 拼在前面. 用 path.Join 让 ".../backups" /
	// "backups" / "" 都规范化, 但要保留 keyPrefix 不含 BSL prefix.
	keyPrefix := backupsSubdir + "/"
	keys, err := l.BSL.ListPrefix(ctx, bslName, keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("list BSL %q prefix %q: %w", bslName, keyPrefix, err)
	}

	// BSL prefix 也可能让 keys 长这样: "<bslPrefix>/backups/<name>/velero-backup.json"
	// — fingerprint.BSLObjectClient.ListPrefix 当前实现返回包括 BSL prefix
	// 的完整 key. 我们要找的 marker 文件名是 veleroBackupManifestFile;
	// backup name 是它前一级目录的名字. 这里不依赖前缀长度, 直接看
	// "/backups/<name>/<file>" 的尾巴.
	candidates := map[string]struct{}{}
	for _, k := range keys {
		if !strings.HasSuffix(k, "/"+veleroBackupManifestFile) {
			continue
		}
		// 找到最后一个 "/backups/" 段; 该段后第一个 path 元素就是 backup name.
		// 用 path.Dir 拿到 ".../backups/<name>", 再 path.Base.
		dir := path.Dir(k) // ".../backups/<name>"
		name := path.Base(dir)
		if name == "" || name == "." || name == "/" {
			continue
		}
		// 再防御一下: parent dir 应该叫 "backups" — 否则可能是嵌套 prefix
		// 里别的 JSON 文件碰巧叫这个名 (极少, 但写防御性代码免后患).
		parent := path.Base(path.Dir(dir))
		if parent != backupsSubdir {
			continue
		}
		candidates[name] = struct{}{}
	}

	// Dedup against Velero-known backups (race-safe — see file header).
	if l.VeleroBackups != nil && len(candidates) > 0 {
		known, err := l.VeleroBackups.ListBackups(ctx, bslName)
		if err != nil {
			// dedup 失败不致命 — 我们宁可让 importer 跑一遍 (它内部
			// IsAlreadyExists 兜底). log 上层处理, 这里只是把错误吞掉,
			// 用纯 BSL list 作为返回 (相当于 dedup miss).
			// 不返回 error: controller 拿到名字仍然能 IDempotent import.
		} else {
			for _, k := range known {
				delete(candidates, k)
			}
		}
	}

	out := make([]string, 0, len(candidates))
	for n := range candidates {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}
