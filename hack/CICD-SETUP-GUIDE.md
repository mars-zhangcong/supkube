# SupKube CI/CD 一次性接管指引（照着做即可）

> 这份是**操作清单**，给"只能你手动做"的部分。真实值已为你填好（仓库
> `mars-zhangcong/supkube`、订阅 `Sub-RnD`、AKS `aks-jumborca-dev` @
> `Research_and_Development`）。方案细节见 `CICD.md`。
>
> 全程约 20 分钟。每步末尾有"✅ 期望看到"，对不上就停下别往下走。

---

## 已为你完成（无需再做）

- [x] `ci.yaml` 改造（6 闸，删 ghcr docker job，加 lint-format + image-scan）
- [x] `cd.yaml` + `hack/ci-verify.sh` 新建并校验
- [x] `gofmt -w` 存量格式化、`go vet/build/test` 全绿
- [x] 本机启用 pre-push hook（`core.hooksPath=.githooks`）
- [x] `cd.yaml` 的 AKS 资源组已填真实值

下面是**你要做的 5 步**。

---

## Step 1 · Azure 侧：建 OIDC App + 授权（约 8 分钟）

> 让 GitHub Actions 无密码推 ACR、部署 AKS。整段可直接复制进终端。

```bash
# ── 1.1 变量（已填好你的真实值）──
export SUBSCRIPTION_ID="df83ea02-9ad1-43a1-bc8d-8520029943b4"
export TENANT_ID="aa3c336b-16d4-4c3e-a8ec-47fe88bd62e9"
export GH_ORG_REPO="mars-zhangcong/supkube"
export ACR_NAME="supkube"
export AKS_NAME="aks-jumborca-dev"
export AKS_RG="Research_and_Development"

az account set --subscription "$SUBSCRIPTION_ID"

# ── 1.2 建一个用 OIDC 的 App（无 client secret）──
export APP_ID=$(az ad app create --display-name "github-supkube-cicd" --query appId -o tsv)
az ad sp create --id "$APP_ID"
echo "APP_ID=$APP_ID"          # 记下，Step 2 要用

# ── 1.3 4 条联合凭据：main 分支 / 所有 tag / test 环境 / prod 环境 ──
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

# ── 1.4 角色：ACR 推送 + AKS 部署（最小权限）──
export ACR_ID=$(az acr show -n "$ACR_NAME" --query id -o tsv)
export AKS_ID=$(az aks show -n "$AKS_NAME" -g "$AKS_RG" --query id -o tsv)
az role assignment create --assignee "$APP_ID" --role "AcrPush" --scope "$ACR_ID"
az role assignment create --assignee "$APP_ID" \
  --role "Azure Kubernetes Service Cluster User Role" --scope "$AKS_ID"

# ── 1.5 让 AKS 节点能从 ACR 拉镜像（推成功≠能拉，避免 ImagePullBackOff）──
az aks update -n "$AKS_NAME" -g "$AKS_RG" --attach-acr "$ACR_NAME"

# ── 1.6 打印要填进 GitHub 的三个值 ──
echo ""
echo "===== 复制下面三行到 Step 2 ====="
echo "AZURE_CLIENT_ID=$APP_ID"
echo "AZURE_TENANT_ID=$TENANT_ID"
echo "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
```

✅ **期望看到**：每条 `az role assignment create` 返回 JSON（或 `RoleAssignmentExists`，无害）；最后打印出三个值。
⚠ 若 1.2 报权限不足，说明你的账号不能在租户建 App —— 找 Azure 管理员代跑 Step 1。

---

## Step 2 · GitHub Secrets（约 2 分钟）

进 `https://github.com/mars-zhangcong/supkube/settings/secrets/actions` → **New repository secret**，建 3 个（值来自 Step 1.6）：

| Name | Value |
|---|---|
| `AZURE_CLIENT_ID` | Step 1.6 打印的 APP_ID |
| `AZURE_TENANT_ID` | `aa3c336b-16d4-4c3e-a8ec-47fe88bd62e9` |
| `AZURE_SUBSCRIPTION_ID` | `df83ea02-9ad1-43a1-bc8d-8520029943b4` |

✅ **期望看到**：Secrets 列表里出现这 3 个名字（值不可见是正常的）。
> 注：ACR 已 `anonymousPullEnabled=true`，**不需要**任何镜像仓库密码。

---

## Step 3 · GitHub Environments（约 3 分钟）—— 这就是"生产人工卡点"

进 `https://github.com/mars-zhangcong/supkube/settings/environments`：

1. **New environment** → 名字 `test` → 直接 Save（不设审批，自动部署）。
2. **New environment** → 名字 `prod` → 进去后：
   - 勾 **Required reviewers** → 添加你自己（`mars-zhangcong`）。**这一步=生产上线必须有人点 Approve。**
   - （可选）**Deployment branches and tags** → 选 `Protected branches and tags`。

✅ **期望看到**：Environments 列表有 `test` 和 `prod`，`prod` 标注有 reviewer。

---

## Step 4 · main 分支保护（约 3 分钟）—— 让"不过 CI 不能合并"

> ⚠ 前提：status check 名字要先在 GitHub 出现过一次才能勾选。所以**先开一个测试 PR 触发一次 CI**（见下），再回来配。

**4a. 先触发一次 CI 让闸门名字注册：**
```bash
git checkout -b chore/ci-bootstrap
git commit --allow-empty -m "chore: trigger CI to register status checks"
git push -u origin chore/ci-bootstrap          # pre-push hook 会先本地校验
# 然后到 GitHub 开 PR（base=main）
```

**4b. 等这次 PR 的 CI 跑完**（6 个 job：Backend / Frontend (Build) / Dashboard Drift Gate / Lint & Format / Image Scan / Helm Lint）。

**4c. 进** `https://github.com/mars-zhangcong/supkube/settings/branches` **→ Add branch ruleset（或 Add rule）→ 目标 `main`：**
- ✅ Require a pull request before merging（Required approvals: 1）
- ✅ Require status checks to pass before merging → 搜索并勾选这 6 个：
  `Backend (Build + Test)`、`Frontend (Build)`、`Dashboard Drift Gate`、`Lint & Format`、`Image Scan`、`Helm Lint`
- ✅ Require branches to be up to date before merging
- Save。

✅ **期望看到**：保存后，任何 PR 不过这 6 闸就显示 "Merging is blocked"。

---

## Step 5 · 团队成员各自启用 pre-push hook（每人一次）

> 本机我已设好。其他人 clone 后各跑一次（hook 是 tracked 文件，但 `core.hooksPath` 是本地配置，不会自动生效）：

```bash
git config core.hooksPath .githooks
```

✅ **验证**：随便改个文件 `git push`，应先看到 `── SupKube pre-push 校验 ──` 一连串 ✅。

---

## 全部做完后：第一次发布演练

```bash
# 1) 合并 Step 4 的 bootstrap PR（CI 全绿后）
# 2) 打 tag 触发 CD
git checkout main && git pull
git tag v0.9.1.10-alpha
git push origin v0.9.1.10-alpha
```
然后到 `https://github.com/mars-zhangcong/supkube/actions` 看：
- `CD / Build & Push` 绿（多架构推 ACR）
- `CD / Deploy → test` 绿（自动部署 + ci-verify buildStamp 校验）
- `CD / Deploy → prod` **卡在 "Review deployments"** → 你点 **Approve** → 部署生产 + verify

✅ 至此：每次提交走 hook+CI，每次发布走 tag→test→人工批→prod，全程 verify-before-ship。

---

## 出问题先看这里

| 现象 | 多半是 | 翻 CICD.md |
|---|---|---|
| CD azure/login 报 `AADSTS70021 No matching federated identity` | Step 1.3 的 subject 与触发源不符（tag/branch/env）| §6.1 #3 |
| 推 ACR `denied` / `401` | AcrPush 角色没生效 或 token 过期 | §6.1 #1/#2 |
| Pod `ImagePullBackOff` | Step 1.5 的 attach-acr 没做 | §6.2 #9 |
| Pod `exec format error` | 只推了单架构 | §6.2 #1 |
| 部署成功但跑的是旧代码 | buildStamp 不变 / 容器名错 | §6.2 #2/#3 |
| status check 勾选列表里找不到那 6 个名字 | 还没跑过一次 CI（先做 4a）| Step 4a |
