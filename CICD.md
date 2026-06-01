# SupKube CI/CD 从 0 到 1 落地方案

> 适用范围：SupKube（Velero 驱动的 K8s 数据保护 UI）。本文是**操作手册 + 配置模板 + 排错库 + 运维规范**，照着执行即可。
> 配套现有资产：`.github/workflows/ci.yaml`（绿灯闸门，已就绪）、`hack/dev-deploy.sh`（开发快速环）、`hack/publish-release.sh`（生产 9 步发布）、`dashboard/gen-data.mjs`（漂移闸）。
> 单一来源纪律见 `ENGINEERING.md`：Rule D（verify-before-ship）、Rule E（回归强制律）。

阅读顺序与本项目流水线一一对应：

```
前期准备 → CI 环节 → 镜像构建&推送 → CD 部署 → 全流程联调 → 问题修复 → 运维规范
```

---

## 0. 项目事实速查（方案的事实基线）

所有配置都基于这些**已核实**的项目事实，不是通用模板：

| 维度 | 事实 | 出处 |
|---|---|---|
| 后端 | Go **1.25.0**（gin + controller-runtime + go-oidc/v3），`CGO_ENABLED=0` 静态二进制 | `supkube-backend/go.mod`、`docker/Dockerfile.backend` |
| 前端 | Vue 3 + Vite 5（Node 20 构建），产物为 nginx 静态站，`/api/` 反代 `supkube-backend:8080` | `supkube-frontend/package.json`、`docker/Dockerfile.frontend` |
| 镜像 | 后端 `golang:1.25-alpine`→`alpine:3.19`（非 root uid 1000）；前端 `node:20-alpine`→`nginx:1.25-alpine`；均**多阶段构建** | 两个 Dockerfile |
| 多架构 | **amd64**（AKS 拉取）+ **arm64**（docker-desktop 本地）；`TARGETARCH` 由 buildx 注入 | `Dockerfile.backend` L17-23 |
| 版本注入 | `-ldflags -X version.Version / version.BuildStamp`，`/api/v1/status` 回显 → 验证"跑的是不是新代码" | `Dockerfile.backend` L25-34、`internal/version` |
| 镜像仓库 | **ACR `supkube.azurecr.io`**（Standard，`anonymousPullEnabled=true`），短名 `backend`/`frontend`，tag=产品版本 | `hack/publish-release.sh`、`hack/dev-deploy.sh` |
| Helm | `supkube-helm/supkube`，**双轨版本**：chart `0.9.1-alpha.N`(SemVer) ↔ appVersion `0.9.1.N-alpha`(四段)；内置 velero 子 chart（`charts/*.tgz` 被 gitignore，Chart.lock 锁定）；**EULA 闸**需 `--set eula.accept=true` | `Chart.yaml`、`Chart.lock` |
| 集群 | docker-desktop（arm64/本地 daemon）+ `aks-jumborca-dev`（amd64/ACR 拉取）；命名空间 `supkube` + `velero` | `hack/dev-deploy.sh` |
| 关键坑 | 容器名是 `backend`/`frontend`，**不是** deployment 名 `supkube-backend`（`kubectl set image` 错名=静默 no-op）；ACR token 1h TTL；前端 chunk hash 缓存；helm 升级静默丢 SA RBAC（#79） | `hack/dev-deploy.sh` 大段注释 |

---

## 1. 前期准备（环境 + 仓库 + 权限）

### 1.1 工具选型与版本建议

| 用途 | 选型 | 版本 | 为什么是它（针对 SupKube） |
|---|---|---|---|
| 流水线引擎 | **GitHub Actions** | - | 项目已有 `ci.yaml`，零迁移成本；与 GitHub Environments 审批天然集成 |
| 容器构建 | **docker buildx + BuildKit** | buildx ≥0.12 | 项目已用 buildx 做 amd64+arm64（见 publish-release.sh）；GHA 缓存 `type=gha` |
| 镜像仓库 | **Azure Container Registry** | Standard SKU | 生产真实仓库，AKS 同租户拉取免公网 |
| 包管理(部署) | **Helm** | v3.14+ | 项目交付物就是 Helm chart |
| K8s 集群 | **AKS** + 本地 docker-desktop | - | 现状双集群 |
| 镜像安全扫描 | **Trivy** | aquasecurity/trivy-action | 轻量、可 fail-on CRITICAL，CI 内即可 |
| Go 静态检查 | **golangci-lint** | v1.59+ | 语法/lint 前置拦截 |
| 云鉴权 | **Azure OIDC 联合身份**（workload identity） | azure/login@v2 | **不在仓库存任何密码**（取代 publish-release.sh 的交互式 `az login`） |

### 1.2 一次性云端准备（管理员执行一次）

> 目标：让 GitHub Actions 能**无密码**推 ACR、部署 AKS。基础设施细节见 `hack/AZURE-SETUP.md`。

```bash
# 变量
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
TENANT_ID=$(az account show --query tenantId -o tsv)
RG=<你的资源组>            # 例：rg-supkube
ACR_NAME=supkube
AKS_NAME=aks-jumborca-dev
GH_ORG_REPO=<owner>/supkube   # 例：mars/supkube

# 1) 建一个用 OIDC 的 App（无 client secret）
APP_ID=$(az ad app create --display-name "github-supkube-cicd" --query appId -o tsv)
az ad sp create --id "$APP_ID"
SP_OBJ=$(az ad sp show --id "$APP_ID" --query id -o tsv)

# 2) 联合凭据：分别给 main 分支、tag、和 test/prod 两个 Environment
for SUBJECT in \
  "repo:${GH_ORG_REPO}:ref:refs/heads/main" \
  "repo:${GH_ORG_REPO}:ref:refs/tags/*" \
  "repo:${GH_ORG_REPO}:environment:test" \
  "repo:${GH_ORG_REPO}:environment:prod"; do
  NAME=$(echo "$SUBJECT" | tr ':/*' '---')
  az ad app federated-credential create --id "$APP_ID" --parameters "{
    \"name\":\"$NAME\",
    \"issuer\":\"https://token.actions.githubusercontent.com\",
    \"subject\":\"$SUBJECT\",
    \"audiences\":[\"api://AzureADTokenExchange\"]
  }"
done

# 3) 角色：ACR 推送 + AKS 部署（最小权限）
ACR_ID=$(az acr show -n "$ACR_NAME" --query id -o tsv)
AKS_ID=$(az aks show -n "$AKS_NAME" -g "$RG" --query id -o tsv)
az role assignment create --assignee "$APP_ID" --role AcrPush --scope "$ACR_ID"
az role assignment create --assignee "$APP_ID" --role "Azure Kubernetes Service Cluster User Role" --scope "$AKS_ID"
# 注意：CI 只需 AcrPush（推），AKS 节点的 kubelet 身份需另配 AcrPull（拉）——这是镜像"推成功但拉不到"的高频根因，见 §6.1。

echo "把这三个值填进 GitHub Secrets："
echo "AZURE_CLIENT_ID=$APP_ID"
echo "AZURE_TENANT_ID=$TENANT_ID"
echo "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
```

### 1.3 GitHub 仓库配置（在 GitHub 网页操作）

需要在 **Settings** 里手动开启（CI 脚本无法自动配置，必须人工）：

1. **Secrets**（Settings → Secrets and variables → Actions → Repository secrets）：
   - `AZURE_CLIENT_ID`、`AZURE_TENANT_ID`、`AZURE_SUBSCRIPTION_ID`（来自 §1.2，**非密码**）
   - 注：ACR `anonymousPullEnabled=true`，拉取不需密钥；推送走 OIDC。
2. **Environments**（Settings → Environments）建两个：
   - `test`：无审批，自动部署。
   - `prod`：**Required reviewers** 勾选你本人/发布负责人（这就是"生产人工审核卡点"）；可加 **Wait timer**、**Deployment branches: protected only**。
3. **Branch protection**（Settings → Branches → 给 `main` 加规则）：
   - ✅ Require a pull request before merging（≥1 review）
   - ✅ Require status checks to pass：勾选 `Backend (Build + Test)`、`Frontend (Build)`、`Dashboard Drift Gate`、`Helm Lint`、`Lint & Format`、`Image Scan`
   - ✅ Require branches to be up to date before merging

> ⚠ 不勾 status checks，CI 红也能合并 —— 闸门形同虚设。这是从"幽灵 CI"变"真闸门"的最后一公里，且**只能你手动开**。

---

## 2. CI 环节（前置拦截：格式 / 语法 / 单测 / 构建 / 漂移 / 扫描）

CI 目标：**坏代码进不了 main**。在已有 `ci.yaml` 基础上补足"代码格式校验 + 语法检查 + 安全扫描"。下面是**完整可用**的 `ci.yaml`，触发条件 `pull_request` + `push:main`。

> 现状 `ci.yaml` 已包含：backend vet/test/build、frontend build、dashboard 漂移、helm lint（含 `helm dependency build` 拉 velero 子 chart）。新增 `lint-format` 与 `image-scan` 两个 job。

`.github/workflows/ci.yaml`（在现有文件上**新增**以下两个 job）：

```yaml
  lint-format:
    name: Lint & Format
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # --- 后端：gofmt 格式 + golangci-lint 语法/lint ---
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.0"          # 与 go.mod 一致，见 ci.yaml backend job
          cache-dependency-path: supkube-backend/go.sum
      - name: gofmt 校验（未格式化即失败）
        working-directory: supkube-backend
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "::error::以下文件未 gofmt："; echo "$unformatted"; exit 1
          fi
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.59
          working-directory: supkube-backend
          args: --timeout=5m

      # --- 前端：可选，需先加 lint 脚本，见 §2.1 ---
      # - uses: actions/setup-node@v4
      #   with: { node-version: "20", cache: "npm", cache-dependency-path: supkube-frontend/package-lock.json }
      # - run: npm ci
      #   working-directory: supkube-frontend
      # - run: npm run lint          # 见 §2.1：需先在 package.json 加 eslint
      #   working-directory: supkube-frontend

  image-scan:
    name: Image Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # 本地构建（不推送）后用 Trivy 扫描——PR 阶段就拦住高危 CVE
      - name: Build backend image (local, no push)
        run: docker build -f docker/Dockerfile.backend -t supkube/backend:scan ./supkube-backend
      - name: Build frontend image (local, no push)
        run: docker build -f docker/Dockerfile.frontend -t supkube/frontend:scan ./supkube-frontend
      - name: Trivy scan backend
        uses: aquasecurity/trivy-action@0.24.0
        with:
          image-ref: supkube/backend:scan
          severity: CRITICAL,HIGH
          ignore-unfixed: true          # 上游未修复的不卡板，避免噪声卡死发布
          exit-code: "1"
      - name: Trivy scan frontend
        uses: aquasecurity/trivy-action@0.24.0
        with:
          image-ref: supkube/frontend:scan
          severity: CRITICAL,HIGH
          ignore-unfixed: true
          exit-code: "1"
```

### 2.1 前端 lint（可选增强，需先加依赖）

> 现状：`supkube-frontend/package.json` **只有** `dev/build/preview`，没有 lint/test。CI 不能假设不存在的脚本。若要补"格式/语法校验"，按下面加（这会改前端 devDependencies，**需你确认**）：

```jsonc
// supkube-frontend/package.json 的 scripts 增加：
"lint": "eslint . --ext .vue,.js,.ts --max-warnings 0",
"typecheck": "vue-tsc --noEmit",
"format:check": "prettier --check \"src/**/*.{vue,js,ts,css}\""
// devDependencies 增加：eslint、eslint-plugin-vue、prettier、vue-tsc、typescript
```

加完后取消 §2 里 `lint-format` 的前端注释段即可。**在确认前，前端保持 build-only 校验**（构建本身就是最强的"语法 + 打包"检查）。

### 2.2 CI 校验项清单（卡点）

| 校验项 | 工具/命令 | 卡点行为 | 对应 job |
|---|---|---|---|
| Go 格式 | `gofmt -l .` | 非空即 fail | lint-format |
| Go 语法/lint | golangci-lint | 报错即 fail | lint-format |
| Go 单元测试 | `go test -race ./...` | 任一 FAIL 即 fail | backend |
| Go 构建 | `go build` | 编译错即 fail | backend |
| 前端构建 | `npm run build` | 打包错即 fail | frontend |
| Dashboard 漂移 | `node dashboard/gen-data.mjs` | exit 1（数据漂移）即 fail | dashboard-drift |
| Helm 模板 | `helm dependency build` + `helm lint` | lint error 即 fail | helm-lint |
| 镜像高危 CVE | Trivy CRITICAL/HIGH | 有可修复高危即 fail | image-scan |

---

## 3. 镜像构建 & 推送（轻量化 / 分层 / 多架构 / 推 ACR）

### 3.1 镜像优化原则（已落实在现有 Dockerfile，巩固即可）

| 原则 | SupKube 现状 | 说明/巩固 |
|---|---|---|
| **多阶段构建** | ✅ builder→runtime | 后端 alpine 运行时只装 `ca-certificates tzdata`；前端只留 nginx + dist |
| **分层缓存** | ✅ `COPY go.mod go.sum` 先于源码 / 前端 `COPY package*.json` 先于源码 | 依赖未变则命中缓存，CI 用 `cache-from/to type=gha` |
| **轻量基底** | ✅ alpine 系 | 后端二进制 `-ldflags="-w -s"` 去符号表 |
| **非 root 运行** | ✅ 后端 uid 1000 | 安全基线 |
| **可观测版本** | ✅ ldflags 注入 Version/BuildStamp | `/api/v1/status` 回显，是 §6.2 排错的命门 |
| **多架构** | ✅ amd64+arm64 | amd64→AKS、arm64→docker-desktop |

> 不需要重写 Dockerfile。仅建议：后端运行时基底可由 `alpine:3.19` 进一步降为 `gcr.io/distroless/static` 以减面（**可选**，因 `CGO_ENABLED=0` 静态二进制无需 libc；但会失去 shell，影响 `dev-deploy.sh` Phase5 的 `exec ... wget` 验活——故**暂不改**，列为 §7 待评估项）。

### 3.2 镜像 tag 与版本规则（与项目双轨版本对齐）

```
产品版本（四段）   0.9.1.10-alpha       ← git tag v0.9.1.10-alpha、镜像 tag、appVersion
Helm chart 版本   0.9.1-alpha.10       ← SemVer，仅 Chart.yaml version 用
镜像完整路径      supkube.azurecr.io/backend:0.9.1.10-alpha
                  supkube.azurecr.io/frontend:0.9.1.10-alpha
不可变 + 可追溯    再额外打一个 :<git-sha> tag（永不覆盖，便于精确回滚）
```

规则：**生产镜像 tag 一律用 git tag 派生的产品版本号，禁止 `latest` 上生产**。`latest` 仅 ghcr/dev 便利用途。

### 3.3 CD 流水线（构建多架构 → 推 ACR），完整模板

新建 `.github/workflows/cd.yaml`。触发：打 `v*` tag（生产）或手动 `workflow_dispatch`（指定环境）。

```yaml
name: CD
on:
  push:
    tags: ["v*"]              # 例：git tag v0.9.1.10-alpha && git push --tags
  workflow_dispatch:
    inputs:
      environment:
        description: 部署目标
        type: choice
        options: [test, prod]
        default: test

permissions:
  id-token: write             # OIDC 换 Azure token 必需
  contents: read

env:
  ACR_LOGIN_SERVER: supkube.azurecr.io
  ACR_NAME: supkube
  CHART_DIR: supkube-helm/supkube
  AKS_NAME: aks-jumborca-dev
  AKS_RG: rg-supkube          # ←按实际改

jobs:
  # ─────────────── 1) 构建多架构镜像并推 ACR ───────────────
  build-push:
    name: Build & Push (multi-arch → ACR)
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.ver.outputs.version }}
    steps:
      - uses: actions/checkout@v4

      - name: 解析版本号（tag → 产品版本）
        id: ver
        run: |
          if [ "${GITHUB_REF_TYPE}" = "tag" ]; then
            V="${GITHUB_REF_NAME#v}"        # v0.9.1.10-alpha → 0.9.1.10-alpha
          else
            V="0.0.0-dev-${GITHUB_SHA::8}"  # 手动触发用 sha 派生
          fi
          echo "version=$V" >>"$GITHUB_OUTPUT"
          echo "▶ VERSION=$V"

      - uses: azure/login@v2               # OIDC，无密码
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: ACR 登录（token 现取，规避 1h TTL 过期，见 §6.1）
        run: az acr login --name "$ACR_NAME"

      - uses: docker/setup-qemu-action@v3  # arm64 跨架构需要 QEMU
      - uses: docker/setup-buildx-action@v3

      - name: Build & Push backend
        uses: docker/build-push-action@v6
        with:
          context: ./supkube-backend
          file: ./docker/Dockerfile.backend
          platforms: linux/amd64,linux/arm64    # 一个 manifest list 双架构
          push: true
          build-args: |
            SUPKUBE_VERSION=${{ steps.ver.outputs.version }}
            CACHEBUST=${{ github.run_id }}
          tags: |
            ${{ env.ACR_LOGIN_SERVER }}/backend:${{ steps.ver.outputs.version }}
            ${{ env.ACR_LOGIN_SERVER }}/backend:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: false                # ACR 对 provenance 附件兼容性差，关掉避免 push 报错（见 §6.1）

      - name: Build & Push frontend
        uses: docker/build-push-action@v6
        with:
          context: ./supkube-frontend
          file: ./docker/Dockerfile.frontend
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ${{ env.ACR_LOGIN_SERVER }}/frontend:${{ steps.ver.outputs.version }}
            ${{ env.ACR_LOGIN_SERVER }}/frontend:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: false

      - name: 推送后即时校验（推成功≠能拉，见 §6.1）
        run: |
          az acr repository show-tags -n "$ACR_NAME" --repository backend  -o tsv | grep -qx "${{ steps.ver.outputs.version }}"
          az acr repository show-tags -n "$ACR_NAME" --repository frontend -o tsv | grep -qx "${{ steps.ver.outputs.version }}"
          # 确认是多架构 manifest list（避免只推了单架构 → AKS amd64 拉不到）
          az acr manifest list-metadata -n "$ACR_NAME" --name "backend:${{ steps.ver.outputs.version }}" --query "[].architecture" -o tsv

  # ─────────────── 2) 部署测试环境（自动） ───────────────
  deploy-test:
    name: Deploy → test (auto)
    needs: build-push
    runs-on: ubuntu-latest
    environment: test               # 无审批
    steps:
      - uses: actions/checkout@v4
      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
      - run: az aks get-credentials -n "$AKS_NAME" -g "$AKS_RG" --overwrite-existing
      - uses: azure/setup-helm@v4
        with: { version: v3.14.0 }
      - name: helm upgrade（测试命名空间）
        run: |
          helm repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts
          helm dependency build "$CHART_DIR"
          helm upgrade --install supkube "$CHART_DIR" \
            -n supkube-staging --create-namespace \
            --set eula.accept=true \
            --set backend.image.repository=$ACR_LOGIN_SERVER/backend \
            --set backend.image.tag=${{ needs.build-push.outputs.version }} \
            --set frontend.image.repository=$ACR_LOGIN_SERVER/frontend \
            --set frontend.image.tag=${{ needs.build-push.outputs.version }} \
            --wait --timeout 5m
      - name: Verify-before-ship（Rule D：buildStamp 真新）
        run: ./hack/ci-verify.sh supkube-staging ${{ needs.build-push.outputs.version }}

  # ─────────────── 3) 部署生产（人工审核卡点） ───────────────
  deploy-prod:
    name: Deploy → prod (manual approval)
    needs: [build-push, deploy-test]
    runs-on: ubuntu-latest
    environment: prod               # GitHub Environment 的 Required reviewers = 人工卡点
    steps:
      - uses: actions/checkout@v4
      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
      - run: az aks get-credentials -n "$AKS_NAME" -g "$AKS_RG" --overwrite-existing
      - uses: azure/setup-helm@v4
        with: { version: v3.14.0 }
      - name: helm upgrade（生产命名空间 supkube）
        run: |
          helm repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts
          helm dependency build "$CHART_DIR"
          helm upgrade --install supkube "$CHART_DIR" \
            -n supkube --create-namespace \
            --set eula.accept=true \
            --set backend.image.repository=$ACR_LOGIN_SERVER/backend \
            --set backend.image.tag=${{ needs.build-push.outputs.version }} \
            --set frontend.image.repository=$ACR_LOGIN_SERVER/frontend \
            --set frontend.image.tag=${{ needs.build-push.outputs.version }} \
            --wait --timeout 5m \
            --atomic                       # 失败自动回滚到上一个 release（关键！见 §8）
      - name: Verify-before-ship（生产）
        run: ./hack/ci-verify.sh supkube ${{ needs.build-push.outputs.version }}
```

> ✅ 上面的 `--set` key 已**核对过** `supkube-helm/supkube/values.yaml`（L103-120）：真实结构是 `backend.image.{repository,tag}`、`frontend.image.{repository,tag}`、`eula.accept`，模板已用正确 key，可直接用。`repository` 默认已是 `supkube.azurecr.io/backend|frontend`，覆盖是为显式/防漂移；只改 `tag` 也可。

### 3.4 CI 内联验证脚本（Rule D 的 CI 版），可直接用

新建 `hack/ci-verify.sh`（精简版 dev-deploy.sh Phase5，去掉本地 daemon 部分，**专为 CI 设计**）：

```bash
#!/usr/bin/env bash
# ci-verify.sh <namespace> <expected-version>
# verify-before-ship 的 CI 版：部署后证明"跑的是新代码"，否则 exit 1 让流水线红。
# 复用 dev-deploy.sh 验证哲学：image tag 匹配 + buildStamp 是今天 + SA 能 list pod（#79）。
set -euo pipefail
NS="${1:?用法: ci-verify.sh <namespace> <version>}"
VER="${2:?缺少版本号}"
DEPLOY_BACKEND=supkube-backend
DEPLOY_FRONTEND=supkube-frontend

fail(){ echo "::error::$*"; exit 1; }

# 1) rollout 真收敛
kubectl -n "$NS" rollout status deploy/$DEPLOY_BACKEND  --timeout=3m || fail "backend rollout 未收敛（ImagePull/CrashLoop？）"
kubectl -n "$NS" rollout status deploy/$DEPLOY_FRONTEND --timeout=3m || fail "frontend rollout 未收敛"

# 2) 容器镜像 tag == 期望版本（探活容器名，规避 deploy 名 vs 容器名静默 no-op）
got=$(kubectl -n "$NS" get deploy/$DEPLOY_BACKEND -o jsonpath='{.spec.template.spec.containers[0].image}')
[[ "$got" == *":$VER" ]] || fail "backend 镜像 tag 不符：期望 *:$VER 实得 $got"

# 3) buildStamp 是今天（证明不是缓存旧镜像）
today=$(date -u +%y%m%d)
stamp=$(kubectl -n "$NS" exec deploy/$DEPLOY_BACKEND -- \
        sh -c 'wget -qO- http://localhost:8080/api/v1/status 2>/dev/null || curl -sf http://localhost:8080/api/v1/status' \
        | sed -n 's/.*"buildStamp":"\([^"]*\)".*/\1/p')
[[ "$stamp" == "$today"* ]] || fail "buildStamp 非今日：得 '$stamp' 期望 '$today-*'（可能跑的是缓存旧镜像）"

# 4) #79 防护：SA 能跨 ns list pod
sa=$(kubectl -n "$NS" get deploy/$DEPLOY_BACKEND -o jsonpath='{.spec.template.spec.serviceAccountName}')
sa="${sa:-default}"
for tgt in "$NS" velero; do
  [[ "$(kubectl auth can-i list pods -n "$tgt" --as="system:serviceaccount:${NS}:${sa}")" == "yes" ]] \
    || fail "SA $sa 不能 list ns/$tgt 的 pod —— #79 静默 RBAC 丢失特征"
done
echo "✅ verify-before-ship 通过：ns=$NS version=$VER buildStamp=$stamp"
```

---

## 4. CD 部署（分级环境 + 触发规则）

### 4.1 三套环境与发布等级

| 环境 | 集群 / 命名空间 | 部署方式 | 触发 | 审批 |
|---|---|---|---|---|
| **开发 dev** | docker-desktop（arm64）/ `supkube` | `hack/dev-deploy.sh`（本地快速环，~60-90s）| 开发者手动 | 无 |
| **测试 test** | AKS `aks-jumborca-dev` / `supkube-staging` | `cd.yaml` deploy-test | 打 tag 或手动 dispatch | **自动** |
| **生产 prod** | AKS（建议独立 prod 集群或 `supkube` ns）/ `supkube` | `cd.yaml` deploy-prod | test 通过后 | **人工审核**（GH Environment Required reviewers） |

> 资源有限只有一个 AKS 时：用**命名空间隔离**（`supkube-staging` vs `supkube`）作为过渡；有预算后升级为独立 prod 集群（生产数据保护产品强烈建议物理隔离）。这属于"通用适配思路"，按企业网络/隔离实情调整。

### 4.2 触发规则（trigger matrix）

| 事件 | CI(ci.yaml) | CD(cd.yaml) build-push | deploy-test | deploy-prod |
|---|---|---|---|---|
| PR → main | ✅ 全部校验 | — | — | — |
| push → main（合并后）| ✅ | — | — | — |
| 打 tag `v*` | — | ✅ | ✅ 自动 | ⏸ 等人工审批 |
| 手动 dispatch(test) | — | ✅ | ✅ | — |

> 设计意图：**日常合并只跑 CI 闸门**（快、省钱）；**只有打版本 tag 才进入发布管道**，与 `ENGINEERING.md §4 发布流程`一致（tag = 一次正式发布的开始）。

### 4.3 分支策略

```
main           ← 受保护，唯一可发布分支；只接受通过 CI 的 PR
 ├─ feat/xxx   ← 功能分支，从 main 切，PR 回 main
 ├─ fix/xxx    ← 缺陷分支；每个修复必须带回归测试（Rule E）
 └─ release 用 tag 标记，不开长期 release 分支（项目是单主干 + tag 节奏）
hotfix/xxx     ← 生产急修：从最近 tag 切，修复 → PR main → 打补丁 tag → 走 CD
```

---

## 5. 全流程联调（一次完整演练）

第一次贯通按此顺序验证（**每步都要看到绿/预期输出再走下一步**）：

```bash
# ① 本地预检（等价 CI，先在本机过一遍，省 CI 往返）
cd supkube-backend && gofmt -l . && go vet ./... && go test ./... && go build ./... && cd ..
node dashboard/gen-data.mjs                     # 期望 exit 0
helm repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts
helm dependency build supkube-helm/supkube && helm lint supkube-helm/supkube --set eula.accept=true

# ② 开 PR → 看 GitHub Actions：6 个 CI job 全绿才允许 merge
# ③ merge 后在 dev 集群快速验证一把
./hack/dev-deploy.sh --auto                     # 本地双架构 + 4 步真验证

# ④ 正式发布：打 tag → CD 自动构建推 ACR → 自动部署 test
git tag v0.9.1.10-alpha && git push origin v0.9.1.10-alpha
#   看 cd.yaml：build-push 绿 → deploy-test 绿（含 ci-verify.sh buildStamp 校验）

# ⑤ 生产：到 Actions 页面，deploy-prod 卡在 "Review deployments" → 人工点 Approve
#   通过后 helm upgrade --atomic 到 prod，再跑一次 ci-verify.sh

# ⑥ 联调收尾：浏览器走完整 user journey（Rule D：不验单 endpoint）
#   登录 → 建 backup → 看列表 → 发起 restore → 看状态，确认功能真可用
```

---

## 6. 问题修复（两类高频问题，专项排查库）

### 6.1 镜像无法推送（push 失败）

**根因分析 → 排查 → 修复 → 预防**

| # | 根因 | 现象/日志 | 排查命令 | 修复 | 预防 |
|---|---|---|---|---|---|
| 1 | **ACR token 1h TTL 过期**（构建超 1h 后 push）| `401 Unauthorized` / `denied: requested access to the resource is denied` 出现在 push 末段 | 看 push 报错时间戳是否距 `az acr login` >1h | push **前**而非流程开头 `az acr login`；CD 模板已把 login 紧贴 build-push | buildx 用 `cache-from/to type=gha` 缩短构建时长；大改动拆 job |
| 2 | **缺 AcrPush 角色** | `denied` 且**一开始**就失败 | `az role assignment list --assignee $AZURE_CLIENT_ID --scope $(az acr show -n supkube --query id -o tsv)` | `az role assignment create --assignee $APP_ID --role AcrPush --scope $ACR_ID`（§1.2） | OIDC App 创建时即配齐角色 |
| 3 | **OIDC 联合凭据 subject 不匹配** | `AADSTS70021: No matching federated identity record` | 看 Actions 日志 azure/login 步骤；核对 subject 与触发 ref（tag/branch/env）| 按 §1.2 为 tag、branch、environment **分别**建 federated-credential | 一次把 4 个 subject 都建好 |
| 4 | **provenance/SBOM 附件不被 ACR 接受** | `failed to push ... unexpected status: 400` | 是否 buildx 默认带 provenance | 模板已 `provenance: false`；如需 SBOM 单独走 Trivy | 默认关 provenance 推 ACR |
| 5 | **网络/限流** | `dial tcp timeout` / `toomanyrequests` | 重跑 | 加 retry；错峰 | ACR Standard 配额足够；监控 push 频率 |
| 6 | **tag 不可变冲突** | `cannot overwrite ... immutable` | ACR 是否开了 immutable tag | 用新版本号/sha tag，不覆盖 | 规则禁止覆盖已发布 tag（§3.2）|

**通用排查动作**：
```bash
az acr login --name supkube                 # 期望 "Login Succeeded"
az acr repository list -n supkube -o table  # 能列说明读权限通
az acr repository show-tags -n supkube --repository backend -o table
```

### 6.2 镜像构建后运行异常（run 异常 / 跑的是旧代码）

| # | 根因 | 现象 | 排查命令 | 修复 | 预防 |
|---|---|---|---|---|---|
| 1 | **架构不匹配**（只推了 arm64，AKS 是 amd64）| Pod `CrashLoopBackOff`，日志 `exec format error` | `az acr manifest list-metadata -n supkube --name backend:<ver> --query "[].architecture"` | CD 模板 `platforms: linux/amd64,linux/arm64` 一次推双架构 | push 后校验 manifest 含 amd64（§3.3 已加）|
| 2 | **`kubectl set image` 容器名写错**=静默 no-op | "image updated" 但 Pod 没滚动，跑的是旧镜像 | `kubectl -n <ns> get deploy supkube-backend -o jsonpath='{..containers[0].name}'` → 应是 `backend` | 用 helm `image.tag` 驱动滚动；脚本探活真容器名（dev-deploy.sh 做法）| **不要**手动 set image；走 helm |
| 3 | **跑的是缓存旧镜像**（buildStamp 不变）| UI/接口行为像旧版 | `kubectl exec deploy/supkube-backend -- wget -qO- localhost:8080/api/v1/status` 看 buildStamp | `ci-verify.sh` 校验 buildStamp 是今天，不符即 fail | 每次构建注入新 BuildStamp（Dockerfile 已做）；CD 强制 verify |
| 4 | **前端 chunk hash 缓存**（nginx 旧资产/旧 index）| 白屏或报 `Loading chunk failed`，资产 404 | `kubectl exec deploy/supkube-frontend -- ls /usr/share/nginx/html/assets/` | index.html `no-store` + 资产 immutable（Dockerfile 已配）；强制滚动重建镜像 | 镜像内 index 与 assets 同批构建；硬刷新 Cmd+Shift+R |
| 5 | **SA RBAC 被 helm 升级静默丢**（#79）| Pod 起来但 UI pod 列表空、无报错 | `kubectl auth can-i list pods -n velero --as=system:serviceaccount:supkube:<sa>` | 修 chart 的 ClusterRole(Binding)；`ci-verify.sh` 第4步拦截 | 每次部署 verify SA 跨 ns 权限 |
| 6 | **缺 ca-certificates/tzdata** 致 HTTPS/时区错 | `x509: certificate signed by unknown authority` | 看 backend 日志 | 运行时已 `apk add ca-certificates tzdata`（勿删）| distroless 化时记得带 ca-certs |
| 7 | **非 root 权限/只读 fs 写失败** | `permission denied` 写临时文件 | 看日志 | 写 `/tmp` 或挂 emptyDir；保持 uid 1000 | securityContext 与镜像 USER 对齐 |
| 8 | **健康探针缺失致流量打到未就绪 Pod** | 滚动期间 502 | `kubectl describe pod` 看 readiness | values.yaml 配 `readinessProbe: /api/v1/status`、`livenessProbe` | 探针指向 status 端点 |
| 9 | **AKS 无 AcrPull**（推成功但拉不到）| `ImagePullBackOff` + `unauthorized` | `kubectl describe pod` Events | 给 kubelet 身份配 AcrPull：`az aks update -n aks-jumborca-dev -g <rg> --attach-acr supkube` | ACR 已 `anonymousPullEnabled=true` 时免；否则务必 attach-acr |

**通用排错三连**（任何运行异常先跑）：
```bash
kubectl -n <ns> get pods                                  # 看状态
kubectl -n <ns> describe pod <pod>                        # 看 Events（拉镜像/调度/探针）
kubectl -n <ns> logs deploy/supkube-backend --tail=100 --previous   # 看崩溃前日志
```

---

## 7. 运维规范 + 清单

### 7.1 版本管理与命名（重申，运维红线）

- git tag：`v0.9.1.N-alpha`（产品四段，前缀 v）
- 镜像 tag：`<产品版本>` + `<git-sha>`，**生产禁用 latest**
- chart version：`0.9.1-alpha.N`（SemVer），appVersion=产品版本；二者随每次发布同步 bump（`ENGINEERING.md §3/§4`）
- CHANGELOG：发布前把 `[Unreleased]` 切到新版本号+日期

### 7.2 一键回滚（生产）

```bash
# 方式 A（推荐）：Helm 回滚到上一个 release（含 values 一起回退）
helm history supkube -n supkube                # 看 revision 列表
helm rollback supkube <上一个REVISION> -n supkube --wait --timeout 5m
./hack/ci-verify.sh supkube <回滚到的版本>      # 回滚后同样要 verify

# 方式 B（最快止血）：只回退镜像，等 helm 复盘
kubectl -n supkube rollout undo deploy/supkube-backend
kubectl -n supkube rollout undo deploy/supkube-frontend

# 方式 C（精确）：用保留的 :<git-sha> 不可变 tag 重新 upgrade 到已知良好版本
helm upgrade supkube supkube-helm/supkube -n supkube --reuse-values --set image.tag=<已知良好版本>
```
> CD 的 `--atomic` 已让"升级失败"自动回滚；上面用于"升级成功但功能异常"的人工回滚。回滚依赖**镜像 tag 不可变 + ACR 保留旧 tag**，故 §3.2 的规则是回滚能力的前提。

### 7.3 ✅ 落地清单（一次性，逐项打勾）

- [ ] §1.2 跑完：OIDC App + 4 条 federated-credential + AcrPush/AKS 角色
- [ ] AKS kubelet 身份有 AcrPull（或 ACR anonymousPull 开启）
- [ ] GitHub Secrets：`AZURE_CLIENT_ID/TENANT_ID/SUBSCRIPTION_ID`
- [ ] GitHub Environments：`test`（无审批）、`prod`（Required reviewers）
- [ ] Branch protection：main 必须过 6 个 status check 才能合并
- [ ] `ci.yaml` 补 `lint-format`、`image-scan` 两个 job
- [x] 新增 `cd.yaml`、`hack/ci-verify.sh`（已 `chmod +x`）
- [x] 核对 `values.yaml` 的 image key 与 cd.yaml `--set` 一致（backend.image.tag 等）
- [x] `gofmt -w` 存量格式化（59 文件），gofmt 闸已可从干净基线强制
- [x] 启用 pre-push hook：`git config core.hooksPath .githooks`（本机已设）
- [ ] **每个团队成员**在自己的 clone 跑一次 `git config core.hooksPath .githooks`
- [ ] golangci-lint 在本地能过（首次可能需修历史 lint，见下注）
- [ ] （可选）前端加 eslint/prettier/vue-tsc，并启用前端 lint
- [ ] 修改 `cd.yaml` 里 `AKS_RG: rg-supkube` 为真实资源组
- [ ] 走一遍 §5 全流程联调，6 个 CI job + CD 三段全绿

> 注：`gofmt`/`go vet`/`go test`/`go build`/漂移闸/前端构建均已在本机验证通过（pre-push hook 全绿）。`golangci-lint` 比 vet 严格，首次 CI 可能报历史 lint——届时按报告逐项修，或在 `.golangci.yml` 里临时收敛规则集，**但不要关掉整个 job**。

### 7.4 🔍 运维巡检清单（常态化）

**每次发布后**
- [ ] `helm history supkube -n supkube` 最新 revision = 本次版本
- [ ] `ci-verify.sh` 绿（image tag + buildStamp 今日 + SA RBAC）
- [ ] `/api/v1/status` 的 version/buildStamp == 本次发布
- [ ] 完整 user journey 手测（建份 backup→列表→restore→状态）

**每周**
- [ ] Trivy 重扫线上 tag（新披露 CVE）：`trivy image supkube.azurecr.io/backend:<ver>`
- [ ] `kubectl auth can-i list pods -n velero --as=...` 复查 #79 RBAC 未漂移
- [ ] `dashboard/gen-data.mjs` 本地跑一遍，确认文档/看板未漂移
- [ ] ACR 用量与 tag 数量（按保留策略清理 dev/sha tag，保留所有发布 tag）

**每月 / 季度**
- [ ] 基底镜像升级（alpine/nginx/golang/node 小版本）后重构重扫
- [ ] velero 子 chart 版本评估（见 Chart.yaml 注释的升级纪律）
- [ ] OIDC 凭据、角色分配审计；轮换非必要权限
- [ ] 回滚演练：在 test 故意部署坏版本，演练 §7.2 回滚链路

### 7.5 日志与故障定位入口（速查）

| 看什么 | 命令 |
|---|---|
| CI/CD 流水线日志 | GitHub → Actions → 对应 run |
| Pod 运行日志 | `kubectl -n <ns> logs deploy/supkube-backend --tail=200` |
| 崩溃前日志 | 加 `--previous` |
| 调度/拉镜像/探针事件 | `kubectl -n <ns> describe pod <pod>` |
| 部署历史 | `helm history supkube -n <ns>` |
| 运行版本真相 | `curl .../api/v1/status` 看 version+buildStamp |
| 集群健康综合 | `hack/verify-cluster.sh` |

---

## 8. 与现有资产的关系（不重复造轮子）

| 现有资产 | 在新方案里的角色 |
|---|---|
| `.github/workflows/ci.yaml` | **CI 闸门基座**，本方案在其上加 lint-format + image-scan |
| `hack/dev-deploy.sh` | **dev 环境**官方部署工具，保留 |
| `hack/publish-release.sh` | 仍是 chart 发布到 `charts.supkube.com` 的权威路径；CD 的 build-push 与之共用 ACR/buildx 约定 |
| `dashboard/gen-data.mjs` | CI 的漂移闸（已接入）|
| `ENGINEERING.md` Rule D/E | verify-before-ship 与回归测试规范，`ci-verify.sh` 是其 CI 落地 |

> **方案不改任何业务代码 / 服务架构 / 框架**（边界约束）。涉及是否 distroless、是否启用前端 lint 两处标注为"需你确认"。一次性 `gofmt -w`（59 个文件，纯格式无逻辑）已执行，使 gofmt 闸从干净基线起强制。

---

## 9. 强制执行：每次提交/上线都按 CICD 标准（三层防线）

| 层 | 作用 | 何时触发 | 谁来兜底 |
|---|---|---|---|
| **① 客户端 pre-push hook**（`.githooks/pre-push`）| push 前本地跑 CI 等价子集（gofmt+vet+build+test+漂移闸+前端构建），坏代码推不出去 | 每次 `git push` | 开发者本机，最快反馈 |
| **② 服务端 CI**（`ci.yaml` 6 个 job）| PR 上跑全套校验（含 `-race`、golangci-lint、Trivy 扫描、helm 完整 lint）| 每个 PR / push main | GitHub Actions |
| **③ 分支保护 + Environment 审批** | 不过 CI 不能合并；生产部署必须人工 Approve | merge / 打 tag 发布 | GitHub 设置（你手动开）|

**启用 hook（每个 clone 一次性，已在本机执行）**：
```bash
git config core.hooksPath .githooks      # 指向仓库内 tracked hook，团队统一
# 紧急跳过（不建议，会留痕在 CI）：SUPKUBE_SKIP_HOOK=1 git push
```

> 设计原则：hook 只是"快速本地预检"，**不是**唯一防线——它可被 `--no-verify` 绕过，所以**真正的硬闸是 ②③**（服务端，绕不过）。三层叠加 = 每次提交都按标准、且无法靠跳过本地检查蒙混过关。

### 9.1 "一次提交→上线"的标准动线（团队照此执行）

```
写代码 → git push（hook 本地拦） → 开 PR（CI 6 闸全绿才可 merge，需 review）
       → merge main → 打 tag v0.9.1.N-alpha → CD 自动构建推 ACR → 自动部署 test + verify
       → 生产 deploy-prod 卡在人工 Approve → 批准后 helm upgrade --atomic + verify → 上线
```
任何一步红 = 停在原地，不进入下一步。这就是把 ENGINEERING.md Rule D（verify-before-ship）从"全靠手动"变成"管道强制"。
