# D-WAIT-010 — Kasten K10 与本测试的命名空间隔离（DR 闭环 Phase 1 · B 前置确认，低危）

> **状态**：open（需 Mars 确认 K10 不纳管 `dr-test`）｜**owner**：Mars｜**触发**：2026-06-03
> **重编号映射**：本条**迁自** FDE 文件 [`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md`](../engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md)，原**自占号 `D-WAIT-006`**。因与 canonical 取号体系撞号，2026-06-04 SCM 按 LEDGER Rule G **forward-only 重编为 D-WAIT-010**。原 FDE 文件按边界保持原状未改。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧号 → 新号**：`FDE D-WAIT-006` → **D-WAIT-010**

- **现状**：test 集群装有 Kasten K10（`kasten-io` ns，~17 pod），有 K10 policy `test-app`（Success 36h）。`preflight-b.sh` B-4 已确认**无 K10 policy 引用 `dr-test`** ✅。
- **请示**：确认 K10 不会自动纳管/快照 `dr-test`（我们的隔离 ns），避免 K10 与 SupKube/Velero 对同一 ns 的快照类争用。低危，但 DR 测试求稳。
