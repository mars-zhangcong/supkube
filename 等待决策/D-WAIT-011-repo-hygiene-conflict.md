# D-WAIT-011 — 仓库状态异常（冲突标记 + 并发切分支，阻断「每轨开 feat branch + push + PR」要求）

> **状态**：closed（本次 D-WAIT 单源根治结构手术即其闭环，2026-06-04）｜**owner**：SCM / Mars｜**触发**：2026-06-03
> **重编号映射**：本条**迁自** FDE 文件 [`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md`](../engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md)，原**自占号 `D-WAIT-007`**。因与 canonical 取号体系撞号，2026-06-04 SCM 按 LEDGER Rule G **forward-only 重编为 D-WAIT-011**。原 FDE 文件按边界保持原状未改。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧号 → 新号**：`FDE D-WAIT-007` → **D-WAIT-011**

---

## ✅ 闭环说明（SCM 2026-06-04）

本条提的根因（① 单一大文件 append 撞出 git 冲突标记；② 多 agent 同分支并发写打架；③ FDE 被迫 fork 到独立文件）**正是本次「D-WAIT 单源根治」结构手术要根治的对象**：

- **冲突标记**：canonical `等待决策.md`（origin/main 版）已勘验**干净无冲突标记**（FDE 当时看到的标记是 PR#8 那侧的瞬态分支状态，未进 main）。
- **一事一文件**：本次把 `等待决策.md` 拆成 `等待决策/D-WAIT-NNN-*.md` 一事一文件 → 并行 agent 永不碰同一文件，从结构上消除 append 撞车。
- **取号权威**：D-WAIT 已登记进 LEDGER §一 成正式取号系列，由 main 集中预分配，杜绝自占撞号。
- **worktree 隔离**：本次手术本身在独立 `git worktree` 上做（与 live writer 物理隔离），并写进 ENGINEERING 防复发规则 Rule I。

请示项 (a)/(b) 的回答见下方原文 + 本闭环说明。

---

## 原文（FDE 提报，audit trail）

- **冲突标记**：`等待决策.md` 含**已提交**的 git 冲突标记（L503/504/611，来自 commit `f18f3d5`）。HEAD 工作树「干净」，说明冲突标记被误提交进文件。
- **并发切换**：本 session 中分支从 `feat/auto5h-prd-review-6-findings` 变为 `feat/prd-010-dr-topology-svg-rebuild`（我未切换）→ 有其他进程/agent 在并发操作此仓库。
- **影响**：在并发改动 + 冲突文件下做 commit/PR 有打架风险。我**只**把本次新增的隔离文件（`engineer-testing/dr-loop-phase1/**`）提交到独立分支，**不动** `等待决策.md` 及任何既存 tracked 文件，不做 `git add -A`。
- **请示**：(a) 是否要我把 `等待决策.md` 的冲突标记清掉（需你确认保留哪一侧）？(b) 确认当前应在哪个分支落 Phase-1 产物。
