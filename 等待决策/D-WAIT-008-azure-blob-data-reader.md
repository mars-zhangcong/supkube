# D-WAIT-008 — 我的 Azure 身份缺 `Storage Blob Data Reader`（DR 闭环 Phase 1 · 轨道 4 · B 的验证手段3 阻断）

> **状态**：open（需 Mars/Infra 赋角色 或 确认降级）｜**owner**：Mars / Infra｜**触发**：2026-06-03
> **重编号映射**：本条**迁自** FDE 文件 [`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md`](../engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md)，原**自占号 `D-WAIT-004`**。因与 canonical 既有 D-WAIT-004 撞号，2026-06-04 SCM 按 LEDGER Rule G **forward-only 重编为 D-WAIT-008**（不抢旧号）。原 FDE 文件按"不碰 engineer-testing/ 制品"边界**保持原状未改**，本目录为 canonical SSOT。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧号 → 新号**：`FDE D-WAIT-004` → **D-WAIT-008**

- **触发**：2026-06-03 Phase 1 轨道 4。`az storage blob list --account-name rnd7sa71 --auth-mode login` 返回 `You do not have the required permissions`。身份 = `ea-rnd-mzhang@jumborca.net`。
- **影响**：验收手段(3)「读 `.supkube-fingerprint.json` 比对 `tarballSHA256`+`sourceClusterID`」无法独立执行。ImportPolicy 控制器自身用集群内 velero 的 account-key，**不**依赖我的身份；故 import 流程可跑，但我无法**旁证**指纹内容。
- **需要的授权（精确）**：在存储账户 `rnd7sa71`（订阅 Sub-RnD `df83ea02-9ad1-43a1-bc8d-8520029943b4`）上，给 `ea-rnd-mzhang@jumborca.net` 赋 **`Storage Blob Data Reader`** 角色（容器 `velero-dr` / `velero-dr-test` 级即可）。命令：
  ```
  az role assignment create --assignee ea-rnd-mzhang@jumborca.net \
    --role "Storage Blob Data Reader" \
    --scope /subscriptions/df83ea02-9ad1-43a1-bc8d-8520029943b4/resourceGroups/<RG>/providers/Microsoft.Storage/storageAccounts/rnd7sa71
  ```
- **建议**：赋只读 reader（非 contributor），最小权限。**或**接受「手段3 用 SupKube API 间接读」的降级（需确认后端暴露 fingerprint 读端点）。
