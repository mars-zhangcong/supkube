# D-WAIT-001 — CD #2 deploy-dev 失败：Azure OIDC federated credential for `dev` environment 缺失

> **状态**：closed（Mars 拍板方案 A）｜**owner**：Mars｜**触发**：2026-06-02
> **迁移说明**：2026-06-04 由 SCM 从单一大文件 `等待决策.md` 拆出（一事一文件，防并行写撞车）。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。

### Mars决策：方案 A（推荐）：在 Azure 加 federated credential（3 min）

**触发时间**：2026-06-02 自主工作开始
**严重度**：🔴 阻断 — push to main → aks-jumborca-dev 自动部署链路全断
**实证**：

- ✅ build-push 成功（2m00s + 47s, BUILDPLATFORM 修复见效, 5x 提速）
- ✅ "推送后即时校验"通过 18s（cd.yaml verify 修复见效）
- ❌ deploy-dev 第 3 步 `azure/login@v2` 6s 失败
- ⏭ deploy-test / deploy-prod 因 needs build-push 但条件不匹配 skipped
- run URL: https://github.com/mars-zhangcong/supkube/actions/runs/26766212442

**根因**：

- ADR-042 在 cd.yaml 新增 `deploy-dev` job 用了 `environment: dev`
- Azure AD App 的 federated credentials 表里**没有** subject = `repo:mars-zhangcong/supkube:environment:dev` 的条目
- 已有的是 `:ref:refs/heads/main`（build-push 用）+ `:environment:test`/`:environment:prod`（CICD.md §1.2 历史配的）
- → GitHub 给 dev environment 拿到的 OIDC token 用 `:environment:dev` subject，Azure 拒接

**两个修复路径（你拍）**：

### 方案 A（推荐）：在 Azure 加 federated credential（3 min）

```bash
APP_ID=$(az ad app list --display-name "github-supkube-cicd" --query "[0].appId" -o tsv)
az ad app federated-credential create --id "$APP_ID" --parameters "{
  \"name\":\"repo-mars-zhangcong-supkube-environment-dev\",
  \"issuer\":\"https://token.actions.githubusercontent.com\",
  \"subject\":\"repo:mars-zhangcong/supkube:environment:dev\",
  \"audiences\":[\"api://AzureADTokenExchange\"]
}"
```

然后 GitHub Actions UI 点 "Re-run failed jobs" 触发 deploy-dev 重跑。

**优点**: 干净, ADR-042 设计意图保留（dev/test/prod 三 environment 独立 protection rules + audit history）
**缺点**: 你 az 操作一次, 一次性


### 方案 B：cd.yaml 临时去掉 `environment: dev` 行（fallback）

```yaml
deploy-dev:
  ...
  # environment: dev    ← 删这行, 让 OIDC 用 ref:refs/heads/main subject
  if: github.ref == 'refs/heads/main' || github.event.inputs.environment == 'dev'
```

我**已准备好 patch**（见下方"待 push patch B"），等你选 B 我立即 push 一个 PR。

**优点**: 不动 Azure, 立刻可用
**缺点**: 失去 dev environment 的 GitHub Environments 隔离（deployment history / future reviewer rules / env-scoped secrets 全失）

### 我的推荐

**方案 A**（你 3 min az 命令 + GitHub UI 触发 re-run）。原因:

1. ADR-042 设计明确"三 environment 真物理隔离 + 各自 protection rules"
2. Azure 操作一次性, 之后所有 push 都 work
3. 方案 B 走捷径会埋下"prod 也撞同样问题"风险（如果 prod environment 也没配 federated credential, prod 也会同样失败）

→ 建议你**直接执行方案 A**（同时给 prod 也加一个 `:environment:prod` 的 federated credential, 一次配 dev+prod 两个）

**如果你选 B**，告诉我，我立即 push fallback PR。

---

## 待 push patch B（方案 B 用）

如果你选方案 B, 我有 ready-to-push 的 patch（在我本机 worktree, 未 commit）:

```diff
--- a/.github/workflows/cd.yaml
+++ b/.github/workflows/cd.yaml
@@ deploy-dev:
   name: Deploy → aks-jumborca-dev (auto, on main push)
   needs: build-push
   runs-on: ubuntu-latest
-  environment: dev                          # 无审批，自动 (push to main 或 manual=dev)
+  # environment: dev  ← 暂移除 (2026-06-02): Azure federated credential 没给 dev
+  #                     environment 配置, 导致 azure/login@v2 OIDC token exchange
+  #                     失败 6s。临时降级用 ref:refs/heads/main subject (build-push
+  #                     用的同一 federated credential, 已 work)。
+  #                     后续 Mars 在 Azure 加 :environment:dev federated credential
+  #                     后恢复本行。详见 等待决策.md D-WAIT-001。
   if: github.ref == 'refs/heads/main' || github.event.inputs.environment == 'dev'
```

如选 B → 我会 reset feat branch + 改 + push + 你审 + merge → CD #3 自动跑。
