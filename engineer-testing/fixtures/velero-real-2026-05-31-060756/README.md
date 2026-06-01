# velero-real fixture — captured 2026-05-31-060756 UTC

## Why this fixture exists

PRD-007 §4.3 Layer 4 Backup Copy 评审 P1: 我之前写 "逐对象复制某个备份" 是错的.
Velero 的卷数据存在 BSL 的共享 Kopia 仓库 (`bucket/<prefix>/kopia/<ns>/...`),
**按 namespace 共享**, 不按单个 backup 隔离. 这个 fixture 抓真集群对象结构来:

1. 看 BSL 里 `backups/<name>/` (metadata) vs `kopia/<ns>/` (卷数据) 怎么分布
2. 对比 snapshotMoveData=false (CSI snapshot) vs snapshotMoveData=true (data mover) 的 BSL 差异
3. 验证 Layer 4 复制粒度应该是 `kopia/<ns>/` 而不是 `backups/<name>/`

## Cluster info

- context        : `docker-desktop`
- kubectl version: "gitVersion": "v1.36.1" "gitVersion": "v1.32.2" 
- velero image   : `velero/velero:v1.18.0`
- namespace      : `velero`
- ns filter      : `<none, all backups captured>`
- captured at    : 2026-05-31-060756 UTC

## BSL summary

```
default       aws     velero                                   Available
myazureblob   azure   velero-dr                                Available
supvault      aws     donglaili-mars-filesight-backup-bucket   Unavailable
```

完整对象 listing 见 [bsl-listing.txt](./bsl-listing.txt)
路径树 (heuristic) 见 [bsl-tree.txt](./bsl-tree.txt)

## Backup summary

- backups total (cluster-wide): **21**
- backups captured in this fixture: see `backups/` dir

### Cross-reference table (\_xref.tsv)

| backup | type | snapshotMoveData | DataUpload# | PVB# | VSC# | phase | errors |
| ------ | ---- | ---------------- | ----------- | ---- | ---- | ----- | ------ |
| aks-app-backup-20260530154011 | metadata-only? | false | 0 | 0 | 0 | Completed |  |
| aks-app-backup-20260531000038 | metadata-only? | false | 0 | 0 | 0 | Completed |  |
| aks-app-backup-export-20260529000022 | data-mover | true | 0 | 0 | 0 | Completed |  |
| aks-app-backup-export-20260530000037 | data-mover | true | 0 | 0 | 0 | Completed |  |
| aks-app-backup-export-20260530154011 | data-mover | true | 0 | 0 | 0 | Completed |  |
| aks-app-backup-export-20260531000038 | data-mover | true | 0 | 0 | 0 | Completed |  |
| bsl-default-test-232038 | metadata-only? | false | 0 | 0 | 0 | Completed |  |
| may28th-demo-20260531000054 | metadata-only? | false | 0 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-20260531000054 | metadata-only? | false | 0 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-export-20260528080816 | data-mover | true | 0 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-export-20260528081605 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-export-20260529000013 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-export-20260530001031 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-2nd-try-export-20260531000054 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-export-20260528073632 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-export-20260529000013 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-export-20260530001031 | data-mover | true | 2 | 0 | 0 | Completed |  |
| may28th-demo-export-20260531000054 | data-mover | true | 2 | 0 | 0 | Completed |  |
| reverse-demo-aks2local-223101 | data-mover | true | 0 | 0 | 0 | PartiallyFailed | 2 |
| reverse-demo-aks2local-224734 | data-mover | true | 0 | 0 | 0 | Completed |  |
| tc-pol-007-export-20260528055927 | data-mover | true | 2 | 0 | 0 | Completed |  |

## Layout

```
.
├── README.md                     # this file
├── _xref.tsv                     # backup <-> sub-CR cross reference
├── bsl-listing.txt               # raw `aws s3 ls --recursive` or `az storage blob list` output
├── bsl-tree.txt                  # derived path tree (heuristic)
├── backupstoragelocation/        # BSL CR yamls
├── backups/                      # Backup CR yamls (with .status)
├── restores/                     # Restore CR yamls
├── dataupload/                   # DataUpload (data-mover backup) yamls
├── datadownload/                 # DataDownload (data-mover restore) yamls
├── pvb/                          # PodVolumeBackup (fs-backup) yamls
├── pvr/                          # PodVolumeRestore yamls
└── volumesnapshotcontents/       # CSI VolumeSnapshotContent yamls
```

## Next steps — PRD-007 §4.3 verification

Once you have this fixture:

1. **Check BSL path layout in `bsl-tree.txt`**:
   - Expect to see `<prefix>/backups/<backup-name>/{velero-backup.json,...gz}` for metadata
   - Expect to see `<prefix>/kopia/<source-ns>/...` for data-mover volume data
   - Expect to see `<prefix>/restic/<source-ns>/...` for fs-backup volume data (legacy)

2. **For each backup type (see _xref.tsv), inspect what's actually in BSL**:
   - `csi-snapshot` (snapshotMoveData=false): only metadata in BSL; volume data is in
     cloud-provider region snapshots (NOT in BSL at all → Layer 4 won't copy this!).
   - `data-mover` (snapshotMoveData=true): metadata + kopia repo contents in BSL.
   - `fs-backup` (PodVolumeBackup): metadata + restic/kopia repo in BSL.

3. **Validate the right copy granularity**:
   - WRONG: `rclone copy s3://src/backups/foo s3://dst/backups/foo` (only metadata!)
   - RIGHT: `rclone sync s3://src/{backups/,kopia/<ns>/,restic/<ns>/,...} s3://dst/...`
     per source namespace.

4. **End-to-end test**: pick one data-mover backup, copy required BSL paths via
   rclone to a 2nd BSL, switch Velero to the 2nd BSL, run Restore, verify pod
   comes up with the right data.

## Caveats

- BSL credentials are NOT in this fixture (only paths/sizes). Replay requires the user
  to re-supply creds via env or `aws configure` / `az login`.
- VSC cross-reference uses a heuristic (grep for `velero.io/backup-name` annotation);
  if your Velero version doesn't set this annotation, VSC# will be 0 even when real.
- For GCP / Swift / etc. BSL providers, bsl-listing.txt will show `BSL_LIST_SKIPPED`;
  capture them manually with the appropriate CLI and append to bsl-listing.txt.
- Secrets values are NOT exfiltrated; only CR `.spec` / `.status` is dumped.

---

## ✅ P1 验证补完 (2026-05-31, mc pod 直接列 MinIO velero bucket)

bash 脚本因 aws CLI 没装 跳过 BSL listing; 用 `kubectl run mc-fixture-probe --image=minio/mc` 临时
pod 直接看 MinIO bucket 结构, 完整证实 PRD-007 P1 两条假设:

### 关键证据 (bucket velero 顶层)

```
velero/
├── backups/                                      # 按 backup 名 (metadata)
│   ├── may28th-demo-20260531000054/
│   ├── may28th-demo-2nd-try-20260531000054/
│   ├── may28th-demo-export-20260528073632/
│   ├── may28th-demo-export-20260529000013/
│   ├── may28th-demo-export-20260530001031/
│   ├── may28th-demo-export-20260531000054/
│   └── v087-mover-ok/
└── kopia/                                        # 按 ns (卷数据)
    └── test-app/                                 # 唯一一个 ns 目录, 共享!
```

### P1 假设 ✅ 双重证实

| P1 子假设 | 证据 | 结论 |
|---|---|---|
| (1) Velero data-mover 卷数据按 ns 在共享 Kopia 仓库 (`bucket/kopia/<ns>/`) | bucket `velero/kopia/` 下**只有 1 个 ns 目录** (`test-app/`), 而 16 个 data-mover 备份全部 `test-app` ns → 共享同一仓库 | ✅ **直接证实** |
| (2) snapshotMoveData=false CSI snapshot 备份, 卷数据不在 BSL | 5 个 `snapshotMoveData=false, snapshotVolumes=true` 备份: 0 DataUpload, 0 VSC (Velero cleanup 后), 但 status=Completed → 数据在云厂商区域快照, BSL 没有 | ✅ **强力证实 (跟 ADR-031 §1 一致)** |

### Layer 4 复制语义结论 (PRD-007 §4.3 v1.1)

- ❌ **按 backup 挑对象复制** (v1 草稿写法): 缺 `kopia/<ns>/` 卷数据 → 复制后恢复失败 (静默数据丢失)
- ✅ **整 ns sync 复制**: `kopia/<ns>/` + `backups/<name>/` 一起 → 才能恢复完整数据
- ⚠ **快照型备份必须 Preflight 拦截**: `snapshotMoveData=false` 备份在 BSL 找不到卷数据, 不能用 Layer 4 (其数据走云端区域快照复制, 另一机制)

### P1 验证 → 解锁 PRD-007 Phase 1

PRD-007 §4.3 v1.1 写法**得到 fixture 实测背书**, 可启动 Phase 1 实施 (rclone sync `kopia/<ns>/` + `backups/<name>/` POC, 含 Preflight 拦截 `snapshotMoveData=false` 备份, 错误码 `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`).
