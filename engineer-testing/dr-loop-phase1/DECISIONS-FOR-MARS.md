# 等待 Mars 决策 — DR 闭环 Phase 1（轨道 4）

> 本应写进根目录 `等待决策.md`，但该文件当前**带已提交的 git 冲突标记**
> (`<<<<<<< HEAD` 503 / `=======` 504 / `>>>>>>> f18f3d5` 611)，且本 session 期间
> 工作分支被**并发切换**（`feat/auto5h-prd-review-6-findings` → `feat/prd-010-dr-topology-svg-rebuild`）。
> 为不污染冲突文件、不与并发操作打架，决策项暂记于此独立文件。**见 D-WAIT-007（仓库状态）。**

---

## D-WAIT-004 — 我的 Azure 身份缺 `Storage Blob Data Reader`（B 的验证手段3 阻断）

- **触发**：2026-06-03 Phase 1 轨道 4。`az storage blob list --account-name rnd7sa71 --auth-mode login` 返回 `You do not have the required permissions`。身份 = `ea-rnd-mzhang@jumborca.net`。
- **影响**：验收手段(3)「读 `.supkube-fingerprint.json` 比对 `tarballSHA256`+`sourceClusterID`」无法独立执行。ImportPolicy 控制器自身用集群内 velero 的 account-key，**不**依赖我的身份；故 import 流程可跑，但我无法**旁证**指纹内容。
- **需要的授权（精确）**：在存储账户 `rnd7sa71`（订阅 Sub-RnD `df83ea02-9ad1-43a1-bc8d-8520029943b4`）上，给 `ea-rnd-mzhang@jumborca.net` 赋 **`Storage Blob Data Reader`** 角色（容器 `velero-dr` / `velero-dr-test` 级即可）。命令：
  ```
  az role assignment create --assignee ea-rnd-mzhang@jumborca.net \
    --role "Storage Blob Data Reader" \
    --scope /subscriptions/df83ea02-9ad1-43a1-bc8d-8520029943b4/resourceGroups/<RG>/providers/Microsoft.Storage/storageAccounts/rnd7sa71
  ```
- **建议**：赋只读 reader（非 contributor），最小权限。**或**接受「手段3 用 SupKube API 间接读」的降级（需确认后端暴露 fingerprint 读端点）。

## D-WAIT-005 — `aks-jumborca-test` 的 Velero 损坏（B 的目标侧根本不可用）

- **触发**：2026-06-03。`velero-74ff6c7596-9cqjq` 卡 `Init:0/2` 已 38h（两个 init 容器 `velero-plugin-for-aws` / `velero-plugin-for-microsoft-azure` 停在 `PodInitializing`）；一个 `node-agent` 卡 `ContainerCreating`。
- **根因（读到的证据）**：test 的 `supkube` ns **没有 azure cloud-credentials secret**（只有 `velero-repo-credentials`）；BSL `azure-blob` 因此 **Phase 为空（非 Available）**，`spec.credential` 为空。
- **影响**：目标集群**无法 import / restore**，与我是否有存储权限无关 → **B 在此修复前完全不能跑**。
- **需要 Mars/Infra**：在 test `supkube` ns 创建 velero 用的 azure 凭据 secret（含 `AZURE_STORAGE_ACCOUNT_ACCESS_KEY`），并排查 velero pod init 卡死（疑似 ACR 拉取或调度）。属**既存共享基础设施**，我未触碰。
- **建议**：先修 test velero（或改用一个 velero 健康的第三 AKS 作目标）。修好后 `preflight-b.sh` 的 B-2/B-3 会自动转绿。

## D-WAIT-006 — Kasten K10 与本测试的命名空间隔离（B 前置确认，低危）

- **现状**：test 集群装有 Kasten K10（`kasten-io` ns，~17 pod），有 K10 policy `test-app`（Success 36h）。`preflight-b.sh` B-4 已确认**无 K10 policy 引用 `dr-test`** ✅。
- **请示**：确认 K10 不会自动纳管/快照 `dr-test`（我们的隔离 ns），避免 K10 与 SupKube/Velero 对同一 ns 的快照类争用。低危，但 DR 测试求稳。

## D-WAIT-007 — 仓库状态异常（阻断「每轨开 feat branch + push + PR」要求）

- **冲突标记**：`等待决策.md` 含**已提交**的 git 冲突标记（L503/504/611，来自 commit `f18f3d5`）。HEAD 工作树「干净」，说明冲突标记被误提交进文件。
- **并发切换**：本 session 中分支从 `feat/auto5h-prd-review-6-findings` 变为 `feat/prd-010-dr-topology-svg-rebuild`（我未切换）→ 有其他进程/agent 在并发操作此仓库。
- **影响**：在并发改动 + 冲突文件下做 commit/PR 有打架风险。我**只**把本次新增的隔离文件（`engineer-testing/dr-loop-phase1/**`）提交到独立分支，**不动** `等待决策.md` 及任何既存 tracked 文件，不做 `git add -A`。
- **请示**：(a) 是否要我把 `等待决策.md` 的冲突标记清掉（需你确认保留哪一侧）？(b) 确认当前应在哪个分支落 Phase-1 产物。

## A 路线（K3S）就绪的 2 个外部动作（轨道 1 结论，需 Mars）

- **A-1 凭据**：K3S HomeLab 经 **Tailscale `100.68.20.72:6443` 现在可达**（隧道/TLS 已验证），仅本地 admin client cert 过期被拒。需 Mars 在 `homelab-mbp-2` 上 `sudo cat /etc/rancher/k3s/k3s.yaml`、把 `server:` 改写为 `https://100.68.20.72:6443`，并入本机 kubeconfig（新 context，勿覆盖既存）。~5 min。
- **A-2 版本**：K3S 上 SupKube = **`0.9.1.9-alpha`（已发布版，无 ImportPolicy/fingerprint）**。A 跑指纹/导入腿前，K3S 端须升级到含命脉能力的 **dev 镜像**（与 dev/test 同源）。
