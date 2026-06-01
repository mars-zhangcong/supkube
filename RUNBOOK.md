# SupKube 运维手册（RUNBOOK）

> **用途**：故障时**照着做**的操作手册——SRE / 客户支持遇到「备份卡住 / 还原不动 / 云账单飙升 / 删不掉」时，按症状查到根因 + 处置步骤，不靠口口相传。
> **读者**：SRE、客户支持、值班工程师。
> **配套脚本**（`hack/`）：`supkube_debug.sh`（诊断包）· `verify-cluster.sh`（依赖体检）· `preflight.sh`（装前预检）· `diagnose-rp-deletion.sh`（删除卡住）· `mirror-images.sh`（air-gap 镜像）。
> **关联**：[SECURITY.md](SECURITY.md) · [架构设计.md](架构设计.md) · [API-REFERENCE.md](API-REFERENCE.md)（`/status` 探针）· [SLO-RTO-RPO.md](SLO-RTO-RPO.md)（目标值）

---

## 0. 黄金 5 分钟（任何故障先做这 3 步）

软件**不能问用户「自己是否存在」**（ENGINEERING.md Rule D 子规则）——所以排障从主动采集开始：

```bash
# 1) 跑的是哪个 build？（确认不是缓存镜像、不是没滚出去）
curl -s https://<host>/api/v1/status        # → {status, version, buildStamp}

# 2) 依赖体检：Velero / node-agent / snapshot-controller / CSI 在不在、版本对不对
bash hack/verify-cluster.sh

# 3) 一键诊断包（客户侧也能跑，类比 Kasten k10_debug.sh）
bash hack/supkube_debug.sh                  # → 生成 tarball，附到工单
```

> 没有 `buildStamp` 变化 → helm upgrade 没真正滚出去（见 §6）。`verify-cluster.sh` 红 → 依赖缺失，先补依赖再谈功能。

---

## 1. 备份卡住 / 还原一直 Restoring（node-agent data path hang）

**症状**：Restore 长时间停在 `Restoring`；Backup 停在 `InProgress`；demo 现场最致命（客户痛点 C-011）。

**根因优先级**：
1. **node-agent data path hang** —— 历史根因（task #102）。`velero v1.16.0-dirty` build 的 data mover 在卷搬运阶段挂死。**已通过固化 velero v1.18.0（chart 12.0.1）根治**。
2. node-agent Pod 没起 / OOM / 节点磁盘满。
3. VolumeSnapshotClass 缺失或 binding mode 不对（CSI 路径，见 §3）。

**处置**：
```bash
# a. 确认 velero 版本（必须 ≥ v1.18.0，不能是 *-dirty）
kubectl -n velero get deploy velero -o jsonpath='{.spec.template.spec.containers[0].image}'; echo
kubectl -n velero get ds node-agent -o jsonpath='{.spec.template.spec.containers[0].image}'; echo

# b. node-agent 是否每个节点都 Ready
kubectl -n velero get pods -l name=node-agent -o wide

# c. 看 data upload/download 进度（卡在哪一步）
kubectl -n velero get datauploads,datadownloads
kubectl -n velero logs ds/node-agent --tail=200 | grep -iE "error|stuck|timeout|hang"

# d. 节点磁盘（data path 落盘在节点）
kubectl get nodes -o json | jq -r '.items[].status.allocatable["ephemeral-storage"]'
```
- 若 velero 是旧 `*-dirty` build → **升级到固化的 v1.18.0 官方 release**（CHANGELOG `0.9.1.9-alpha`）。
- 若 node-agent 缺/挂 → 重建 DaemonSet，确认 hostPath 挂载与节点磁盘空间。
- **绝不**手动删 datupload CR 来「解卡」——会留孤儿快照（→ §4 云账单）。先查清再动（Mars 现场原则：先查清原因再说）。

---

## 2. 还原点（RP）删不掉 / 删除卡住

**症状**：选多个 RP 删除，「后台一直在清、前台慢慢消」，两个集群都有删不掉的残留（2026-05-31 现场，task #101）。

**机制**（异步级联，ADR-012）：
```
DELETE /api/v1/backups/<name>  → 建 DeleteBackupRequest(DBR) → 立即 202
velero backup-deletion-controller 异步处理 DBR → 清 BSL 对象 / VSC / PVB / Backup CR
```
卡住通常卡在「清 BSL 对象」或「VSC 删除」阶段。

**处置**——**先用专用诊断脚本**（跨集群）：
```bash
bash hack/diagnose-rp-deletion.sh

# 手动核对：
kubectl -n velero get deletebackuprequests        # 有没有积压的 DBR
kubectl -n velero get backups                      # 残留 Backup CR
kubectl -n velero logs deploy/velero | grep -i "deletion"
```
- DBR 积压 → 看 velero controller 日志找具体失败对象（BSL 凭据失效？对象被 Object Lock 锁定？）。
- 若 BSL 启用了 **Object Lock / WORM**，到期前**对象删不掉是预期行为**（不是 bug）——告知客户保留期。
- 卡死的 Backup CR（finalizer 不走）→ §5 force-delete。

---

## 3. 备份不产生数据 / VolumeSnapshotClass 缺失（CSI）

**症状**：备份「成功」但卷没进去；或报 `No VolumeSnapshotClass`（客户痛点 C-002/C-017）。

**处置**：
```bash
# CSI 自动适配状态（SupKube #84 的能力）
curl -s https://<host>/api/v1/storage/csi-autoconfig | jq

kubectl get volumesnapshotclass
kubectl get sc                                   # 默认 StorageClass 存在？
kubectl get crd | grep snapshot.storage.k8s.io   # snapshot CRD 装了没
```
- 缺 VSC/SC → SupKube **CSI 一键适配**（task #104）会建别名 SC + VSC + 对齐 binding mode，并显示用到哪个 TransformSet。
- 缺 snapshot-controller / CRD → 这是依赖缺失，`verify-cluster.sh` 会报；按 `preflight.sh` 提示补装（SupKube 不自动装，ADR-023）。

---

## 4. 防云账单飙升（Orphan / Cloud Cost Checklist）⚠

> 备份产品最容易「悄悄烧钱」：孤儿快照、孤儿 BSL 对象、堆积的镜像。**每次大动作后 + 每周**过一遍。

**4.1 孤儿资源清理（SupKube 内建）**
```bash
# 看清理设置 + 上次运行摘要（含孤儿计数）—— admin
curl -s https://<host>/api/v1/settings/cleanup | jq
# 手动触发孤儿清理（推荐型，先看清单再清）
curl -X POST https://<host>/api/v1/admin/cleanup/orphans
```

**4.2 云侧对账（SupKube 删不到的、要去云控制台核）**
| 资源 | 检查 | 风险 |
|---|---|---|
| **CSI 卷快照** | 云盘快照数 vs RP 数对得上吗 | 手删 CR 留下的孤儿快照持续计费 |
| **BSL 对象（S3/Blob）** | 删除后对象真的没了？Object Lock 到期了吗 | WORM 锁定期内删不掉=预期；锁过期后要清 |
| **Backup Copy（Layer 4）** | 第二云的副本是否按 lifecycle 过期 | 跨 BSL 复制**产生出口费 + 双份存储** |
| **ACR 镜像** | dev tag 是否走 TTL | `dev-deploy.sh` 有 ACR TTL preflight；手推的 tag 要手清 |

**4.3 Backup Copy 出口费**：`POST /api/v1/backup-copy` 跨 BSL 搬字节会产生**云出口费**——所以它是 **admin 门槛**（API-REFERENCE §7.11）。批量复制前先 `/backup-copy/preflight` 看清范围。

---

## 5. 删不掉的 Backup CR（finalizer 卡死）

**症状**：Backup CR 一直在、finalizer 不走，正常 DELETE 无效（客户痛点 task #68）。

**处置**——用 SupKube 的 force-delete（治理过副作用，不是裸 `kubectl patch`）：
```bash
curl -X POST https://<host>/api/v1/backups/<name>/force-delete   # editor†（全集群备份升 admin）
```
> force-delete 会处理级联副作用（DBR / 锁定 / Activity 持久化，PRD-008）。**优先用它**，不要直接 `kubectl patch ... finalizers=null`——裸删会留 §4 的孤儿。

---

## 6. helm upgrade 后功能没变（镜像没滚出去）

**症状**：升级了但行为还是旧的。

**处置**（verify-before-ship 的反向排查）：
```bash
curl -s https://<host>/api/v1/status | jq .buildStamp   # 没变 → 没滚出去
kubectl -n <ns> rollout status deploy/supkube-backend
kubectl -n <ns> get pods -o jsonpath='{.items[*].spec.containers[*].image}'; echo
kubectl -n <ns> describe pod <pod> | grep -iE "image|pull"
```
- `imagePullPolicy: IfNotPresent` + 复用 tag → 节点用了缓存旧镜像。**用不可变 tag（产品版本号）**，别复用同名 tag。
- 双集群只升了一个 → 两个都要升、两个都要验（Rule D）。

---

## 7. 认证 / RBAC 排障（403 / 401）

| 现象 | 含义 | 处置 |
|---|---|---|
| `401 missing or malformed Authorization header` | 没带或格式错 token | 检查 `Authorization: Bearer/Basic` |
| `403 insufficient role {required, current}` | 角色不够 | 用够级别的 token（viewer<editor<admin） |
| `403 namespace not in your scope` | editor 越权 ns | 该 ns 不在用户 NamespaceScope；换 admin 或扩作用域 |
| `403 endpoint not in RBAC table (... SupKube bug)` | 端点没登记 `rbac.go` | **这是 bug**，附 `/status` + 端点上报研发 |

> Demo 起不来且全是 403？确认 `RBAC_ENABLED` 设置；本地可临时关（生产勿关）。

---

## 8. Air-Gap（离线集群）

```bash
bash hack/mirror-images.sh        # 把 backend/frontend/velero 镜像镜像到内网 registry
bash hack/airgap-bundle.sh        # 打离线安装包
bash hack/preflight.sh            # 装前预检（CRD/SC/snapshot-controller/权限/K8s 版本）
```
- AI 能力在 air-gap 下**默认本地 Ollama，0 出境**（SECURITY.md §6）；Call Home（PRD-012）在此形态不可用，属预期。

---

## 9. 上报工单要带什么（Support Bundle）

1. `bash hack/supkube_debug.sh` 的 tarball。
2. `curl -s /api/v1/status`（version + buildStamp）。
3. `hack/verify-cluster.sh` 输出。
4. 复现步骤 + 截图 + 涉及的 namespace / RP 名。
5. 双集群场景：**两个集群都带**。

> 未来：Call Home / Auto-Support（PRD-012）会把 1-3 自动打包上送 + 自动开 Case（opt-in）。当前为手动。

---

## 变更记录

| 日期 | 操作人 | 变更 |
|---|---|---|
| 2026-06-01 | Claude | 初版。9 个故障域 runbook：node-agent hang（#102 根因）/ RP 删除卡住（#101 + diagnose 脚本）/ CSI VSC 缺失 / **防云账单 checklist** / force-delete / 镜像没滚出去 / RBAC 403 排障 / air-gap / support bundle。grounding 到 hack/ 现有脚本。 |
