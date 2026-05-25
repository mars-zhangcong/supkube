# SupKube 项目状态快照

> 最后更新：**2026-05-26**（v0.9.1.0-alpha 发布后）
> 用途：休息一段时间后回来，5 分钟内回到项目状态。详细 roadmap 看 [ROADMAP.md](ROADMAP.md)。

## 一句话现状

**MVP 已 ship**：备份/恢复/跨集群/3-2-1-1-0/Object Lock/multi-arch/Helm 分发/Preflight + EULA 全到位；客户能从 `charts.supkube.com` 直接 `helm install` 跑通。下一阶段从"功能堆"转向"商业化 + 售前支持"。

## 立刻打开看

| 链接 | 用途 |
|---|---|
| https://charts.supkube.com/ | 公开 Helm repo（Cloudflare Worker → Azure Blob） |
| https://charts.supkube.com/index.yaml | 当前可装版本列表 |
| https://charts.supkube.com/preflight.sh | 客户预检脚本 |
| http://localhost:30888 | 本地 docker-desktop SupKube UI |
| `kubectl --context aks-jumborca-dev port-forward -n supkube svc/supkube-frontend 8080:80` | AKS dev 集群 SupKube UI（无 NodePort 公网，需 port-forward） |
| [ROADMAP.md](ROADMAP.md) | 完整路线图 + 12 项客户需求归化 |
| [USER_MANUAL.md §23](USER_MANUAL.md) | Helm 安装参考（所有 values + Kasten 类比） |
| [hack/AZURE-SETUP.md](hack/AZURE-SETUP.md) | Azure 制品分发基础设施一次性配置手册 |

## 当前发布版本

| 组件 | 版本 | 制品位置 |
|---|---|---|
| Backend image | `0.9.1.0-alpha` | `supkube.azurecr.io/backend:0.9.1.0-alpha` (multi-arch amd64+arm64) |
| Frontend image | `0.9.1.0-alpha` | `supkube.azurecr.io/frontend:0.9.1.0-alpha` (multi-arch) |
| Helm chart | `0.9.1-alpha.0` (SemVer) / appVersion `0.9.1.0-alpha` | `charts.supkube.com/supkube-0.9.1-alpha.0.tgz` |
| Git tag | `v0.9.1.0-alpha` | `28a5ab4` on `origin/v0.7.9-alpha` |

## 客户安装命令（标准开场）

```bash
# Step 1 — preflight
curl -fsSL https://charts.supkube.com/preflight.sh | bash

# Step 2 — install (EULA required)
helm repo add supkube https://charts.supkube.com/
helm repo update
helm install supkube supkube/supkube --version 0.9.1-alpha.0 --devel \
  -n supkube --create-namespace \
  --set eula.accept=true \
  --set eula.email=ops@yourco.com \
  --set eula.company="Your Co" \
  --set velero.enabled=true
```

## 项目结构

```
supkube/
├── supkube-backend/          # Go + Gin + controller-runtime
├── supkube-frontend/         # Vue 3 + Element Plus + Vite
├── supkube-helm/             # Helm Chart (Velero subchart + Local MinIO + Dex)
├── docker/                   # Dockerfile.backend (TARGETARCH) / Dockerfile.frontend
├── hack/
│   ├── AZURE-SETUP.md                  # Azure 资源一次性配置手册
│   ├── publish-release.sh              # build + multi-arch push + chart 发布
│   ├── cloudflare-worker-charts-proxy.js  # Worker 源码 (dashboard 副本)
│   ├── preflight.sh                    # 客户预检脚本 (10 项)
│   └── verify-cluster.sh               # 装后健康验证
├── docs/                     # PRD + Sprint 复盘 + ADR
├── integrations/             # Velero / images.yaml manifest
├── ROADMAP.md                # 路线图
├── PROJECT-STATUS.md         # 本文件
├── USER_MANUAL.md            # 23 章 + 2 附录（§23 Install Reference 最新）
├── 架构设计.md                # 全量架构 + ADR
└── 测试用例.md                # 回归测试
```

## K8S 部署状态（截至 2026-05-26）

```
本地 docker-desktop (kubectl --context docker-desktop):
  Namespace: supkube
    - supkube-backend  : supkube/backend:0.9.0.2-alpha    Running (待升级到 0.9.1.0)
    - supkube-frontend : supkube/frontend:0.9.0.2-alpha   Running (待升级到 0.9.1.0)
    - supkube-dex      : dexidp/dex:v2.39.1               Running
    - supkube-local-store: quay.io/minio/minio:RELEASE.2025-04-22...  Running
    - Service frontend : NodePort 30888 → :80
    - Helm release     : 装在 0.9.0.2-alpha 时代，未走 EULA gate

AKS aks-jumborca-dev (kubectl --context aks-jumborca-dev):
  Namespace: supkube
    - supkube-backend  : supkube.azurecr.io/backend:0.9.1.0-alpha    Running ✓
    - supkube-frontend : supkube.azurecr.io/frontend:0.9.1.0-alpha   Running ✓
    - supkube-dex      : dexidp/dex:v2.39.1                          Running ✓
    - Helm release     : revision 4, chart 0.9.1-alpha.0, eula.accept=true
    - ConfigMap supkube-eula: accepted=true, email=ops@example.com, company="Example Inc"

AKS aks-jumborca-test:
  注册在 docker-desktop 的 Cluster CR 列表里，仅供 multi-cluster 测试用；不部署 SupKube 本体。
```

## Azure 基础设施（不轻易动）

| 资源 | 位置 / 标识 |
|---|---|
| Subscription | `Sub-RnD` (`df83ea02-9ad1-43a1-bc8d-8520029943b4`) |
| Resource Group | `Research_and_Development` (region: `southeastasia`) |
| ACR | `supkube.azurecr.io` (Standard SKU, anonymousPullEnabled=true) |
| Storage Account | `supkubecharts` (StorageV2 + Static Website 启用) |
| Blob Static Website Endpoint | `https://supkubecharts.z23.web.core.windows.net/` |
| Cloudflare Worker | `charts-azure-proxy` (Free plan, 100k req/day) |
| Cloudflare Custom Domain | `charts.supkube.com` (Universal SSL) |
| AKS clusters | `aks-jumborca-dev` + `aks-jumborca-test` (K8s 1.34.7, 2 nodes each) |
| Domain registrar | 腾讯云 (NS 指向 Cloudflare) |

## 几个回来时容易忘的注意点

1. **EULA gate (v0.9.1.0+)**: 所有 `helm install/upgrade` 必须 `--set eula.accept=true`，否则 template render 阶段直接 fail。这是 by design，不是 bug。

2. **Chart version vs SupKube version**：
   - SupKube 用 4 段 `0.9.1.0-alpha`（image tag + appVersion）
   - Helm chart 用 SemVer `0.9.1-alpha.0`（自动翻译）
   - 客户的 `--version` 参数用 **chart 版**
   - 翻译规则在 `hack/publish-release.sh` 的 `chart_version_from()`

3. **Multi-arch buildx**：publish 用 `docker buildx build --platform linux/amd64,linux/arm64 --push`。Dockerfile.backend 必须有 `ARG TARGETARCH`，否则两个架构都会出成 build host 架构。

4. **EULA ConfigMap**: `kubectl -n supkube get cm supkube-eula -o yaml` 看安装时填的 email / company / license / 时间戳。`helm.sh/resource-policy: keep` → uninstall 不删。

5. **Helm pre-release versions**: 所有 0.9.x-alpha 是 SemVer pre-release，`helm search repo supkube` 默认不显示——必须加 `--devel`。

6. **Cloudflare Worker 反代**：客户访问 `charts.supkube.com` 走 Free-plan Worker (`hack/cloudflare-worker-charts-proxy.js`)，因为 Cloudflare Origin Rules 的 Host/SNI 改写被挪到了 Enterprise 套餐。Worker 5 行代码绕开付费墙。

7. **ACR anonymous pull 是 Standard SKU 起步**——2024 年 Azure 取消了 Basic SKU 的 anonymous pull。不要把 ACR 降级到 Basic 想省钱（~$5 vs $20 / 月，差异不大）。

8. **kubeconfig 不进 git**：`engineer-testing/kc-*.yaml` 已加 .gitignore；任何含 client-key-data 的文件都不要 commit。

9. **publish-release.sh 现在手工跑**：`./hack/publish-release.sh <version>`，需 `az login`。GitHub Actions 自动化在 backlog (`CI` 项)。

## 下次起手的 3 个候选方向

### A. v0.8.14 Log Viewer + Upload to Support（next sprint，~3 天）✅ 已选
商业化路径必修。客户出问题能自助排障 + 一键推 log 包给我们。
- LV1: 后端 /api/v1/logs 流式 (kubectl logs 包装)
- LV2: 前端 LogViewer.vue + 关键词高亮 + 实时跟随
- LV3: Action Detail / Restore drawer 加 "View Logs" 跳转
- LV4: "Upload to Support" 弹窗 → log bundle → EULA email

### B. v0.8.15 备份数据校验（~2 天）
checksum + Kopia repo validate；定期 deep check；Health Score 加权重。

### C. v0.9.2 License Manager 前端（~3 天）
1:1 复刻 Kasten Licenses 页 + mock backend。EULA 已经给 license.key 留了位置，前端读 `cm/supkube-eula` 即可起步。

## 收紧的 Backlog（重要不紧急）

| 项 | 触发条件 |
|---|---|
| **CI**: GitHub Actions 自动 publish | publish-release.sh 稳定 2-3 轮后 |
| **DOCSITE**: VitePress 文档站 | v1.0 GA 准备时 |
| **TELEMETRY**: preflight.sh 匿名遥测 | product-led growth 决策需要数据时 |
| **e**: 完整在线 Case 系统 | Mars 提供 Case API |
| **SEC5**: Microsoft Sentinel webhook | 客户启用 Sentinel |
| **#68**: Force-delete 卡住的 Backup CR | 客户报障时 |

## 已知架构决策（已定，别再纠结）

详见 ROADMAP.md 末尾"关键架构决策"小节。最近 3 条：

| 决策 | Sprint | 决策内容 |
|---|---|---|
| 制品分发架构 | v0.9.0.3 | Azure 一栈 (ACR + Blob + Cloudflare Worker)；index.yaml 相对 URL |
| 镜像架构 | v0.9.0.4 | multi-arch buildx；客户零参数 |
| EULA + License | v0.9.1.0 | helm template fail-fast gate；License placeholder 任意通过 |

历史决策（卷数据备份双模式 / Hooks=Kanister / Hub-Spoke 多集群 / OIDC via Dex / 方案 B Local MinIO BSL）见 ROADMAP.md。
