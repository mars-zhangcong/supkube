# SupKube 集成清单

> **谁在读这个文档**：交付工程师 / SRE / 合规审计 / 升级人员。
> **更新规则**：升级任何第三方组件必须同步更新本文件、`integrations/images.yaml`、`USER_MANUAL.md` 中受影响章节，三处保持一致。
> **当前文档版本**：v0.8.7.2 对应。

---

## 0. SupKube 自身组件

| 组件 | 版本 | 镜像 | 说明 |
|---|---|---|---|
| `supkube-backend` | 0.8.7.2-alpha | `supkube/backend:0.8.7.2-alpha` | Go API + RBAC + 审计 + BSL 元数据聚合 |
| `supkube-frontend` | 0.8.7.2-alpha | `supkube/frontend:0.8.7.2-alpha` | Vue3 SPA，由 nginx 服务 |

---

## 1. 已集成 — 必装（v0.8.x 基础栈）

### 1.1 Velero — 备份引擎

- **角色**：所有 Backup / Restore / Schedule 操作的实际执行者。SupKube backend 不直接动 PV，全部通过 Velero CR 间接驱动
- **支持版本**：`v1.18.0`（最低 `v1.15.0`，因为我们用 Data Mover 的 DataUpload / DataDownload CR，那是 v1.15 引入）
- **官方仓库**：https://github.com/vmware-tanzu/velero
- **镜像**：`velero/velero:v1.18.0`（gcr.io 在中国不可达；推荐镜像到客户私有 registry）
- **必装 CRD**：随 Velero install 一起部署，列表见 https://github.com/vmware-tanzu/velero/tree/v1.18.0/config/crd/v1/bases
- **安装方式**：客户**自己**装 Velero（helm 或 `velero install` CLI），SupKube 不内嵌。SupKube 启动时不检查 Velero 是否存在，但 `/api/v1/status` 会返回 Velero 连接状态

#### 1.1.1 必带子组件：node-agent DaemonSet

- **角色**：跑在每个 Node 上的 Kopia 操作员；**所有跨集群 DR**（Data Mover / Filesystem backup）都靠它把 PV 数据上传/下载对象存储
- **触发条件**：BSL backup spec 含 `snapshotMoveData: true` 或 `defaultVolumesToFsBackup: true`
- **资源消耗**：每 Pod ~200 MB RAM 静默，备份/恢复期间峰值 1-2 GB（Kopia 缓存）；CPU 0.1-2 core 视吞吐
- **关键 flag**：
  - `--data-mover-prepare-timeout=30m`（默认 30 min；大 PV 可能需要调到 2h）
  - `--resource-timeout=10m`
- **安装命令**（已有 Velero deploy 的情况下加 node-agent）：
  ```bash
  velero install --use-node-agent --uploader-type=kopia \
                 --features=EnableCSI \
                 --no-default-backup-location \
                 --use-volume-snapshots=false \
                 --plugins=...  # 沿用现有 plugin 列表
  ```
  现实里通常重新跑 `velero install` 即可，或者用 `kubectl apply` 标准 DaemonSet manifest
- **验证**：`kubectl -n velero get pod -l name=node-agent` 每 Node 一个 Running

#### 1.1.2 ObjectStore 插件（每个 BSL provider 一个）

| Provider | 插件镜像 | 版本兼容 Velero v1.18 | 备注 |
|---|---|---|---|
| AWS / S3-兼容（含 MinIO / 腾讯 COS / 阿里 OSS） | `velero/velero-plugin-for-aws:v1.9.0` | ✅ | SupKube `provider: aws` 走这个 |
| Azure Blob Storage | `velero/velero-plugin-for-microsoft-azure:v1.10.0` | ✅ | SupKube `provider: azure` 走这个 |
| Google Cloud Storage | `velero/velero-plugin-for-gcp:v1.9.0` | ✅ | SupKube v0.9 才支持，目前不安装 |

**关键**：插件是 Velero deployment 的 **initContainer**，启动时复制到 `/plugins` 共享卷。客户漏装哪个，对应 provider 的 BSL 启动时报错 `unable to locate ObjectStore plugin named velero.io/<provider>`，SupKube backend 会把这个原文显示到 UI（v0.8.6 起）

加插件用：
```bash
velero plugin add velero/velero-plugin-for-microsoft-azure:v1.10.0
# 自动重启 velero deployment
```

#### 1.1.3 Velero 最佳实践

| 项 | 建议 | 原因 |
|---|---|---|
| Velero pod resources | request 500 m / 1 Gi，limit 2 / 4 Gi | 备份 100+ 个 ns 时主控制器会吃内存 |
| node-agent pod resources | request 100 m / 256 Mi，limit 1 / 2 Gi | 单独 PV >100GB 时调高 limit |
| `--default-backup-ttl` | `720h` (30 day) | UI 默认值同步 |
| `--restore-resource-priorities` | 用 v1.18 默认值（已修了 CR 顺序） | v1.15 之前手动 patch |
| BSL `validationFrequency` | `1m` 调试期，生产 `5m` | 减 S3 API 调用费用 |
| 给 Velero ServiceAccount 加 events:create | 必须 | 没有的话 SupKube 审计日志写不进 K8s Events |

### 1.2 Dex — OIDC 编排层

- **角色**：把多种身份源（GitHub / Keycloak / Okta / Azure AD / 静态密码）统一抽象成 OIDC，SupKube backend 只对接 Dex 一家
- **支持版本**：`v2.39.1`（最低 v2.38；v2.40+ 的 storage 改了 API，我们没适配）
- **官方仓库**：https://github.com/dexidp/dex
- **镜像**：`ghcr.io/dexidp/dex:v2.39.1` 或 `dexidp/dex:v2.39.1` (DockerHub mirror)
- **CRD**：无（Dex 用内置 memory / Postgres / K8s CRD storage 三选一；SupKube 默认 memory，重启丢 refresh token，可接受）
- **安装方式**：**随 SupKube Helm chart 一起装**（templates/dex-*.yaml）。客户可关掉 `auth.dex.enabled=false` 切外部 OIDC

#### 1.2.1 Dex 最佳实践

| 项 | 建议 |
|---|---|
| `staticPasswords` 密码 | 用 `htpasswd -bnBC 10` 生成 bcrypt，**不要**手写 hash（v0.8.5 step 1 因为这个浪费半天）|
| `enablePasswordDB: true` | demo 用；生产关掉只走外部 connector |
| `connectors` secrets | 走 K8s Secret + `envFromSecrets`（ADR-017），别明文进 values.yaml |
| Issuer URL | 浏览器和后端 pod 都要可达（双 URL 见 ADR-005）|
| 副本数 | 默认 1（memory storage 不能扩 pod）；生产要扩需切到 Postgres backend |

### 1.3 CSI snapshot-controller — K8s CSI 快照基础设施

- **角色**：所有 `VolumeSnapshot` / `VolumeSnapshotContent` CR 的协调器。K8s 1.20+ 不再内置，必须独立部署
- **支持版本**：`v8.0.x`（snapshot.storage.k8s.io/v1）
- **官方仓库**：https://github.com/kubernetes-csi/external-snapshotter
- **镜像**：
  - `registry.k8s.io/sig-storage/snapshot-controller:v8.0.1`
  - `registry.k8s.io/sig-storage/snapshot-validation-webhook:v8.0.1`
- **CRD**：`VolumeSnapshot`, `VolumeSnapshotContent`, `VolumeSnapshotClass`
- **安装方式**：客户的 K8s 平台**通常**已经装了（EKS / AKS / GKE 都默认带）；裸机 K8s 要手动装
- **验证**：`kubectl get crd volumesnapshots.snapshot.storage.k8s.io && kubectl -n kube-system get pod -l app=snapshot-controller`

---

## 2. 已集成 — 可选 / 测试栈

### 2.1 MinIO — 测试用 S3-兼容存储

- **何时用**：开发 / Demo / 没有真实 S3 的客户
- **版本**：`RELEASE.2024-04-18T19-09-19Z`
- **镜像**：`quay.io/minio/minio:RELEASE.2024-04-18T19-09-19Z`
- **生产不推荐**：单实例无 HA；客户生产应该走 AWS S3 / Azure Blob / 阿里 OSS

### 2.2 CSI hostpath driver — 测试用 CSI 实现

- **何时用**：Docker Desktop / Kind / 单节点测试
- **版本**：`v1.16.0`
- **镜像**：`registry.k8s.io/sig-storage/hostpathplugin:v1.16.0`
- **生产严禁**：hostpath 不抗 Node 失效

---

## 3. 已规划 — Roadmap 即将集成

### 3.1 Prometheus — 监控指标采集（v0.9 计划）

- **角色**：抓 Velero、Dex、SupKube backend 的 `/metrics`，存时序数据
- **目标版本**：`v2.51.0`
- **官方仓库**：https://github.com/prometheus/prometheus
- **镜像**：`quay.io/prometheus/prometheus:v2.51.0`
- **集成方式**：**不内嵌** Prometheus 本体，提供 ServiceMonitor / PodMonitor CRD 让客户的 kube-prometheus-stack 抓取我们
- **暴露的指标**（v0.9 backend 计划加）：
  - `supkube_backup_total{phase,namespace}`
  - `supkube_backup_duration_seconds{phase}`
  - `supkube_restore_total{phase,namespace,result}`
  - `supkube_bsl_phase{name,provider}`
  - `supkube_audit_event_total{user,action,result}`
- **Velero 自己也暴露 metrics**（`/metrics` 在 8085 port），加进 ServiceMonitor

### 3.2 Grafana — 可视化（v0.9 计划）

- **角色**：跑 SupKube 提供的 dashboard JSON（RPO/RTO 趋势、备份成功率、Storage 占用）
- **目标版本**：`10.4.2`（LTS）
- **官方仓库**：https://github.com/grafana/grafana
- **镜像**：`grafana/grafana:10.4.2`
- **集成方式**：发布 dashboard JSON 到 `integrations/grafana-dashboards/`，客户用 ConfigMap + sidecar 或手动 import

### 3.3 Kanister — 应用感知备份（v0.10 计划）

- **角色**：跑数据库一致性 hook（mysqldump / pg_basebackup / redis SAVE）
- **目标版本**：`0.108.0`
- **镜像**：`ghcr.io/kanisterio/controller:0.108.0` + per-app blueprint images
- **集成方式**：客户独立装 Kanister，SupKube UI 在策略表单加 "Blueprint" 下拉

### 3.4 cert-manager — TLS 自动续期（生产部署可选）

- **角色**：给 Ingress / Webhook 自动签发 + 续 Let's Encrypt 证书
- **目标版本**：`v1.14.4`
- **集成方式**：SupKube Helm chart 加可选 Ingress 模板，若 `ingress.certManager.enabled=true` 输出 Certificate CR

### 3.5 Reloader — Secret 轮换后自动重启（生产可选）

- **角色**：给 Dex / backend deployment 加 `reloader.stakater.com/auto: "true"` 注解，K8s Secret 变更时自动滚动重启 pod
- **目标版本**：`v1.0.99`
- **集成方式**：客户安装；SupKube Helm chart 模板支持可选注入注解（ADR-017 提到的 v0.9 增强）

---

## 4. 完整镜像清单

机器可读版本在 `integrations/images.yaml`。本节是人读视图：

```
# v0.8.7.2 部署一个最小集群需要的镜像

# SupKube 自身
supkube/backend:0.8.7.2-alpha
supkube/frontend:0.8.7.2-alpha

# Velero 栈
velero/velero:v1.18.0
velero/velero-plugin-for-aws:v1.9.0
velero/velero-plugin-for-microsoft-azure:v1.10.0    # 只装需要的 provider
# velero/velero-plugin-for-gcp:v1.9.0                # v0.9 才用

# Dex
dexidp/dex:v2.39.1

# CSI snapshot 基础设施（如客户 K8s 没自带）
registry.k8s.io/sig-storage/snapshot-controller:v8.0.1
registry.k8s.io/sig-storage/snapshot-validation-webhook:v8.0.1

# 测试用（生产不要）
quay.io/minio/minio:RELEASE.2024-04-18T19-09-19Z
registry.k8s.io/sig-storage/hostpathplugin:v1.16.0
```

---

## 5. 镜像分发：3 种部署模式

### 5.1 联网部署（开发 / 大多数云客户）

直接拉公网镜像。需要可达：
- `docker.io`（velero/velero, dexidp/dex, supkube/*）
- `ghcr.io`（dexidp/dex 备用 + Kanister）
- `quay.io`（prometheus 镜像 + minio）
- `registry.k8s.io`（snapshot-controller）

中国大陆环境注意：`registry.k8s.io` 可能慢，建议走 `dockerproxy.net` 或客户私有 mirror。

### 5.2 半离线（私有 registry mirror）

客户有 Harbor / Artifactory / Nexus，希望所有镜像走自家 registry：

```bash
# 1. 在能联网的机器上拉所有镜像
hack/mirror-images.sh harbor.example.com/supkube

# 2. helm install 时覆盖
helm install supkube ./supkube-helm/supkube \
  --set backend.image.repository=harbor.example.com/supkube/backend \
  --set frontend.image.repository=harbor.example.com/supkube/frontend \
  --set auth.dex.image.repository=harbor.example.com/supkube/dex
# Velero 的 plugin 镜像走 velero CLI:
velero plugin add harbor.example.com/supkube/velero-plugin-for-microsoft-azure:v1.10.0
```

### 5.3 完全离线（气隙环境）

```bash
# 在能联网的机器上打 tarball
hack/airgap-bundle.sh ./supkube-airgap-v0.8.7.2.tar.gz

# 把 tarball 拷到目标环境，加载到本地 docker，再推到本地 registry
tar -xzf supkube-airgap-v0.8.7.2.tar.gz
for img in images/*.tar; do docker load -i $img; done
hack/mirror-images.sh internal-registry.airgap.local/supkube  # 用本地 registry
```

---

## 6. 版本兼容性矩阵

| SupKube | Velero | Dex | snapshot-ctrl | K8s | 状态 |
|---|---|---|---|---|---|
| 0.8.7.x | 1.18.x | 2.39.x | 8.0.x | 1.27 – 1.32 | ✅ 当前 |
| 0.8.5.x | 1.16 – 1.18 | 2.39.x | 6.x – 8.x | 1.27 – 1.32 | ⚠️ 维护中 |
| 0.7.x | 1.14 – 1.16 | — (无 OIDC) | 6.x – 7.x | 1.25 – 1.30 | ❌ EOL |

**升级路径规则**：
1. K8s 版本不跨大版本跳（1.27 → 1.28 OK，→ 1.30 不 OK）
2. Velero 版本可以同 SupKube 版本一起升；不要单独升 Velero 跨大版本（会破坏 BSL schema）
3. Dex 版本平稳，但 v2.39 → v2.40 storage API 改了，**不要单独升 Dex**

---

## 7. 升级 / 卸载 / 灾难恢复

### 7.1 升级单个组件

详见每个组件章节的"升级"小节。通用流程：

1. 改 `integrations/images.yaml` → 新版本
2. 改 `INTEGRATION.md`（本文档）→ 同步版本号 + best practice 变更
3. 改 `USER_MANUAL.md` 受影响章节
4. 改 Helm chart values
5. 跑 `hack/verify-cluster.sh` 在 staging 验证
6. PR → review → merge

### 7.2 全栈卸载

```bash
helm uninstall supkube -n supkube
# Velero 独立卸载
velero uninstall
# CRD 不自动删（防误删数据）；确认无价值后：
kubectl delete crd $(kubectl get crd -o name | grep -E 'velero|snapshot|dex')
```

### 7.3 BSL 数据保留

Helm uninstall **不删** BSL bucket 内容。在 Azure Blob / S3 那边手动清理才会真丢数据。
适合"集群挂了，bucket 还在，去另一集群装新 SupKube 接同一 BSL → 自动同步出所有 backup CR"。

---

## 附录 A：每次集成新组件的 Checklist

新加一个第三方组件（比如未来加 Loki）必须：

- [ ] 本文件 §3 → §1 / §2 升档
- [ ] `integrations/images.yaml` 加镜像 + digest 锁版本
- [ ] `hack/mirror-images.sh` 自动包含（脚本读 images.yaml，无需改）
- [ ] `USER_MANUAL.md` 加章节说明用户怎么用
- [ ] `架构设计.md` 加 ADR 记录"为什么集成它"和"权衡了哪些替代品"
- [ ] Helm chart 加可选启用开关（如果是 SupKube 自带）
- [ ] 兼容性矩阵 §6 加列
- [ ] Roadmap 更新

## 附录 B：当前已知的"接到一半"问题

- ⚠️ **node-agent 没装在测试集群** — 用户必须自己 `velero install --use-node-agent` 才能跑 Data Mover；v0.9 计划在 Helm chart 加 `bundledVelero.enabled` 选项把整个 Velero 栈包进来
- ⚠️ **Grafana dashboard 还没出** — v0.9 交付
- ⚠️ **GCP / Azure AAD 还没支持** — v0.9 交付
