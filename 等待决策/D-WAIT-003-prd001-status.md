# D-WAIT-003 — PRD-001 改正中状态澄清 (Mars 追问 #3) — ✅ 已闭环 2026-06-03

> **状态**：closed（2026-06-03 Mars 拍方案 A）｜**owner**：Mars｜**触发**：2026-06-02
> **迁移说明**：2026-06-04 由 SCM 从单一大文件 `等待决策.md` 拆出。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。

**Mars 决策**: **方案 A** — 走重审通道, 进研发, 跟 PRD-008/009/010/011 一道走"Mars 评审通过 → 进研发"。

**落地** (2026-06-03):
- PRD.md 顶部 index PRD-001 状态: 改正中 → **研发中**
- LEDGER §二 PRD-001 状态同步
- dashboard/data.js PRDS PRD-001 状态同步
- PRD-review/INDEX.md §二 PRD-001 行同步
- DoR-DECISION-2026-06-02.md PRD-001 从"暂缓"升级为"立即开工" (DoR 6 条全过)

---

**触发时间**：2026-06-02 Mars 3h 委托追问
**严重度**：🟡 流程性 — 不阻断, 但 PRD 流水线卡 Mars 重审

**事实链** (来自 PRD-001 §11 评审历史 + INDEX §二):

| 日期 | 操作人 | 状态变化 | 内容 |
|---|---|---|---|
| 2026-05-31 | Mars (评审人 Claude 委托) | 排队评审 → **改正中** | 落 4 finding + T3 拓扑校验 |
| 2026-05-31 | Claude | (隐式) | 4 finding 已修订到 §4 Functions + DoD #6/#7/#10, T3 拓扑校验段加 |
| (评审历史末行) | **等 Mars 重审 → 排队评审 → 已评审** | — | (Claude 写的预期, Mars 未执行) |

**当前状态判断**:
- ✅ PRD-001 的 4 finding (#1 blocker 二次确认 / #2 conflict schema / #4 v1 残留删除 / T3 SC→Immediate 拓扑校验) 在 §4 + §8 DoD **均已书面落地**
- ⏳ 等 Mars 重审 → 状态转 排队评审 / 已评审
- ❌ Mars 实际未重审, 文档卡在"改正中"状态字符串

**给 Mars 拍**:

### 方案 A (推荐): 走重审通道, 进研发
跟 PRD-008/009/010/011 一道走"Mars 评审通过 → 进研发"。我已检查 PRD-001 §4 + §8 DoD 跟 finding 闭环一致 (DoR-2 通), 是否你确认无遗漏即可拍"研发中"。

### 方案 B: Mars 想加新内容
e.g. PRD-001 §4 Functions 还要加新 finding / 调整某条 DoD。如果是, 请说明加什么。

### 方案 C: PRD-001 实际不优先, 维持改正中
长期搁置 (不阻断), 转研发延后到 v0.10.x 后。

**我现在能做的**: 等你拍。同时 PRD-001 关联任务是 #104 v0.9.x-CSI-ONE-CLICK-FIX (CSI 一键适配), 这个 task 跟 PRD-001 (Restore Preflight Checklist) 的关联度需要二次确认 — task #104 名字看像是新建项目, PRD-001 是 v2 改造 RestoreDrawer Checklist。**这两个看似关联实际可能错位**, 你拍 PRD-001 时一并裁。
