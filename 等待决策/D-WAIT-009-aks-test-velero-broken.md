# D-WAIT-009 — `aks-jumborca-test` 的 Velero 损坏（DR 闭环 Phase 1 · B 的目标侧根本不可用）

> **状态**：open（需 Mars/Infra 修 test velero 凭据 secret）｜**owner**：Mars / Infra｜**触发**：2026-06-03
> **重编号映射**：本条**迁自** FDE 文件 [`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md`](../engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md)，原**自占号 `D-WAIT-005`**。因与 canonical 既有 D-WAIT-005 撞号，2026-06-04 SCM 按 LEDGER Rule G **forward-only 重编为 D-WAIT-009**。原 FDE 文件按边界保持原状未改。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧号 → 新号**：`FDE D-WAIT-005` → **D-WAIT-009**

- **触发**：2026-06-03。`velero-74ff6c7596-9cqjq` 卡 `Init:0/2` 已 38h（两个 init 容器 `velero-plugin-for-aws` / `velero-plugin-for-microsoft-azure` 停在 `PodInitializing`）；一个 `node-agent` 卡 `ContainerCreating`。
- **根因（读到的证据）**：test 的 `supkube` ns **没有 azure cloud-credentials secret**（只有 `velero-repo-credentials`）；BSL `azure-blob` 因此 **Phase 为空（非 Available）**，`spec.credential` 为空。
- **影响**：目标集群**无法 import / restore**，与我是否有存储权限无关 → **B 在此修复前完全不能跑**。
- **需要 Mars/Infra**：在 test `supkube` ns 创建 velero 用的 azure 凭据 secret（含 `AZURE_STORAGE_ACCOUNT_ACCESS_KEY`），并排查 velero pod init 卡死（疑似 ACR 拉取或调度）。属**既存共享基础设施**，我未触碰。
- **建议**：先修 test velero（或改用一个 velero 健康的第三 AKS 作目标）。修好后 `preflight-b.sh` 的 B-2/B-3 会自动转绿。
