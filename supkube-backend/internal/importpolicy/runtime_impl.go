// runtime_impl.go — 生产环境用的 BackupLister + BackupImporter 实现.
//
// 单测用 in-memory fake (controller_test.go 里的 fakeLister/fakeImporter);
// 这里是真接 K8s + Velero 的版本, cmd/server/server.go startup 用.
//
// BackupLister 实现说明:
//
//	v1 sprint 只支持 S3-API BSL (aws / minio / cos / oss). gcp/azure 走
//	"not yet implemented" 错误 — 与 internal/api/v1/backup_meta_bsl.go 的
//	覆盖范围一致, 不引入新依赖. 真正去 S3 list 的代码我们不重复实现, 而是
//	依赖 Agent C/D 后续 wire 一个 production lister; 本 baseline 实现先
//	通过 *查 Velero 自己已经 sync 进来的 Backup CR list* 来发现 backup —
//	这是个降级的"穷人版" lister, 但语义正确 (如果 Velero 自己都没看到这
//	个 backup, Import Policy 也没法 import 它). 后续可以替换成直接 S3
//	ListObjectsV2.
//
// BackupImporter 实现:
//
//	真创建 velerov1.Backup CR. 标 OwnerReference 指向源 backup 的
//	storageLocation, 这样 Velero discovery 一跑就会拉 meta. Labels 由
//	controller 传入.
package importpolicy

import (
	"context"
	"fmt"
	"sort"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const veleroNamespace = "velero"

// veleroBackupLister 通过查 Velero 的 BackupList (按 spec.storageLocation
// 过滤) 来发现某 BSL 上的 backup. Velero 自己会 periodic sync BSL → CR,
// 所以这是个稳定来源.
//
// 局限: 如果 backup 在源集群被删但 tarball 还在 BSL, Velero 本集群可能
// 也已经 sync 过 → CR 已存在; 这种 backup 我们要"导入"什么? 把已存在的
// CR 重写? 设计上 v1 sprint 跳过 — ImportBackup 已存在则返回 nil (幂等).
type veleroBackupLister struct {
	Cli client.Client
}

// NewVeleroBackupLister 生产构造函数, 由 cmd/server 注入 runtime client.
//
// Deprecated (sprint 2026-06-01 Agent G): kept as fallback; v1.x removal.
// 生产路径已切到 NewS3BackupLister (s3_lister.go) — 它直接 LIST BSL, 不
// 受 Velero backupSyncPeriod 限制, ImportPolicy RPO 真正能压到 30s.
// 本函数仍然 exported 是因为 NewS3BackupLister 把它当作 dedup 数据源
// (race-safe: Velero 自己 sync 进来的 CR 我们不重复 import). 出 S3
// polling 问题时, cmd/server/server.go 一行可换回来回退.
func NewVeleroBackupLister(cli client.Client) BackupLister {
	return &veleroBackupLister{Cli: cli}
}

func (l *veleroBackupLister) ListBackups(ctx context.Context, bslName string) ([]string, error) {
	list := &velerov1.BackupList{}
	if err := l.Cli.List(ctx, list, client.InNamespace(veleroNamespace)); err != nil {
		return nil, fmt.Errorf("list velero backups: %w", err)
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		b := &list.Items[i]
		// Velero 默认 BSL 名是 "default"; spec.storageLocation 可能为空时
		// 视作 "default".
		sl := b.Spec.StorageLocation
		if sl == "" {
			sl = "default"
		}
		if sl != bslName {
			continue
		}
		names = append(names, b.Name)
	}
	sort.Strings(names)
	return names, nil
}

// veleroBackupImporter 在 targetNs 创建 Velero Backup CR. 因为我们的
// "lister"实际上读的就是 Velero 自己 sync 进来的 CR, 这里的 Import 主要
// 任务是给已存在的 Backup 打 SupKube 的 import labels, 让 UI 能筛选出
// "由 ImportPolicy 自动接管的 backup". 如果将来 lister 换成直接 S3, 这
// 里也要演进成真正 create CR.
//
// 当前 v1 行为:
//  1. Get(targetNs, name) 看是否已存在
//  2. 不存在 → 创建一个最小 Backup CR, storageLocation=bslName + 我们的
//     labels (Velero discovery 后续会把 meta 补上)
//  3. 存在 → patch labels (merge, 不覆盖 admin 自己加的)
type veleroBackupImporter struct {
	Cli client.Client
}

// NewVeleroBackupImporter 生产构造函数.
//
// Deprecated (sprint 2026-06-01 Agent G): kept as fallback; v1.x removal.
// 生产路径已切到 NewS3BackupImporter (s3_importer.go) — 它直接 GET BSL
// 的 velero-backup.json 重建 CR, 配合 NewS3BackupLister 实现"BSL-first"
// 数据流. 本函数仍 exported 是因为切换回退路径方便.
func NewVeleroBackupImporter(cli client.Client) BackupImporter {
	return &veleroBackupImporter{Cli: cli}
}

func (im *veleroBackupImporter) ImportBackup(ctx context.Context, targetNs, bslName, backupName string, labels map[string]string) error {
	existing := &velerov1.Backup{}
	err := im.Cli.Get(ctx, types.NamespacedName{Namespace: targetNs, Name: backupName}, existing)
	switch {
	case err == nil:
		// 已存在 — patch labels merge.
		patched := existing.DeepCopy()
		if patched.Labels == nil {
			patched.Labels = map[string]string{}
		}
		changed := false
		for k, v := range labels {
			if patched.Labels[k] != v {
				patched.Labels[k] = v
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return im.Cli.Patch(ctx, patched, client.MergeFrom(existing))
	case apierrors.IsNotFound(err):
		// 创建一个最小骨架; Velero 后续 sync 会填 spec/status.
		b := &velerov1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupName,
				Namespace: targetNs,
				Labels:    labels,
			},
			Spec: velerov1.BackupSpec{
				StorageLocation: bslName,
			},
		}
		if err := im.Cli.Create(ctx, b); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil // race with Velero discovery — fine.
			}
			return fmt.Errorf("create velero backup: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("get velero backup: %w", err)
	}
}
