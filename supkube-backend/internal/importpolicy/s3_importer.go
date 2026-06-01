// s3_importer.go — 真正从 BSL 拉 velero-backup.json 重建 Velero Backup
// CR 的 importer. 替代 runtime_impl.go::veleroBackupImporter 的"只 label
// 已存在 CR"版本.
//
// 为什么要替换 (sprint 2026-06-01 Agent G)
// ──────────────────────────────────────────
// veleroBackupImporter 假设 Backup CR 已经被 Velero sync 进来了 — 它只
// 给已存在的 CR 打 SupKube label. 配合 s3_lister.go (我们的新 lister
// 现在能从 BSL 直接看到 backup), 这种"什么都不创建"的 importer 就会跳
// 过那些 Velero 还没 sync 过来的 backup, 没法把 RPO 推到 30s.
//
// s3BackupImporter:
//  1. GET <prefix>/backups/<name>/velero-backup.json (BSL);
//  2. 反序列化成 velerov1.Backup;
//  3. metadata 清理 (resourceVersion / UID 来自源集群, 不能 carry);
//     namespace 改成 targetNs; labels merge SupKube 的导入标签;
//  4. Create. AlreadyExists → 视为成功 (与 Velero sync race 的预期场景,
//     见 race condition handling 设计章节).
//
// Idempotency: 同一个 backup 调多次 Import → 第一次 Create 成功, 后续
// 全部 AlreadyExists no-op. 不做 patch (避免和 Velero/admin 手改打架).
// 如果 admin 想"重置 label", 走 kubectl edit 手动来.
package importpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/supkube/supkube-backend/internal/fingerprint"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// s3BackupImporter 是从 BSL 还原 Velero Backup CR 的 importer.
type s3BackupImporter struct {
	BSL fingerprint.BSLObjectClient
	Cli client.Client
}

// NewS3BackupImporter 生产构造函数.
func NewS3BackupImporter(bsl fingerprint.BSLObjectClient, cli client.Client) BackupImporter {
	return &s3BackupImporter{BSL: bsl, Cli: cli}
}

// ImportBackup 实现 BackupImporter.
//
// labels 由 controller 传入 — 我们直接 merge 到 metadata.labels (Velero
// 自己也可能加些 labels, 比如 velero.io/backup-name; 我们 merge 不覆盖).
//
// 我们不返回 fmt.Errorf("backup %s already imported") 之类的 sentinel —
// 上层 controller 只看 error / nil, AlreadyExists 在我们看来就是"已完成"
// (race with Velero sync 或我们上一个 tick), 不需要 escalate.
func (im *s3BackupImporter) ImportBackup(ctx context.Context, targetNs, bslName, backupName string, labels map[string]string) error {
	if im.BSL == nil || im.Cli == nil {
		return fmt.Errorf("s3BackupImporter: BSL or runtime client is nil (wiring bug)")
	}

	// 1. 从 BSL 拉 manifest.
	//
	// path.Join 注意: ListPrefix 已经把 BSL 的 configured prefix 拼好了,
	// 但 GetObject 的 key 也由 BSLObjectClient 内部拼 prefix — 所以这里
	// 我们传的 key 不能带 BSL prefix, 否则会双重拼接. 与 fingerprint
	// 自己的 keys (e.g. backups/<name>/.supkube-fingerprint.json) 同约定.
	key := path.Join(backupsSubdir, backupName, veleroBackupManifestFile)
	raw, err := im.BSL.GetObject(ctx, bslName, key)
	if err != nil {
		return fmt.Errorf("get manifest %s: %w", key, err)
	}

	// 2. 解析. Velero 的 manifest 就是序列化的 velerov1.Backup (Kind=Backup).
	src := &velerov1.Backup{}
	if err := json.Unmarshal(raw, src); err != nil {
		return fmt.Errorf("decode manifest %s: %w", key, err)
	}

	// 3. 清理 metadata + 注入我们的字段.
	// 不直接复用 src.ObjectMeta — 源集群的 UID / resourceVersion / managedFields
	// 拷过来会被 apiserver 拒绝, 也容易因 ownerReference 指向不存在的 CR 而
	// orphan. 重建一个最小 ObjectMeta 最干净.
	dest := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: targetNs,
			Labels:    mergeLabels(src.Labels, labels),
			// 复制 annotations — 它们承载 velero 自己的内部信息 (e.g.
			// backup-resources-version), Velero discovery 后续会读;
			// 不清掉是出于安全侧的 "尽量保留语义".
			Annotations: src.Annotations,
		},
		// Spec 整段直接搬 — 它的 StorageLocation 必须正确 (是源集群里
		// 的 BSL 名). 我们覆盖一下确保指向当前 BSL.
		Spec: src.Spec,
	}
	dest.Spec.StorageLocation = bslName

	// 4. Create. AlreadyExists 是 race, 视为成功.
	if err := im.Cli.Create(ctx, dest); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// 我们 race 输给了 Velero 的 BackupSyncController (或上一个 tick
			// 的我们). 不 overwrite — Velero 的 sync 会把它的 status 填好,
			// labels 想要的话 controller 下一轮会通过 lister 的 dedupe 看
			// 到它已存在并跳过, label 由 admin/Velero 自行管理.
			log.Printf("[importpolicy] s3-importer: %s/%s already exists (race with Velero sync); skip", targetNs, backupName)
			return nil
		}
		return fmt.Errorf("create velero backup %s/%s: %w", targetNs, backupName, err)
	}
	return nil
}

// mergeLabels 把 src 和 add 合并, add 优先 (覆盖同名). 用于把 SupKube
// 的 import 标签叠加到源 backup 自带的 label 集合上. nil-safe.
func mergeLabels(src, add map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range src {
		out[k] = v
	}
	for k, v := range add {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
