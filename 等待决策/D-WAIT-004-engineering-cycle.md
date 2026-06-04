# D-WAIT-004 — 工程周期闭环流程标准 — ✅ 已闭环 2026-06-03

> **状态**：closed（2026-06-03 已闭环）+ 含 ORIG audit 段｜**owner**：Mars｜**触发**：2026-06-02
> **迁移说明**：2026-06-04 由 SCM 从单一大文件 `等待决策.md` 拆出。原文件里 `D-WAIT-004`（结论）与 `D-WAIT-004 ORIG`（原始提问）是**同一逻辑决策**的两段，按 Mars 2026-06-04 拍板**合并入本文件**（结论在上、ORIG 作 audit trail 在下，号不变）。决策内容原样保留，未删历史。INDEX 见 [`../等待决策.md`](../等待决策.md)。

**Mars 决策**:
- **Q1 阶段拆解** = 9 阶段: Requirement → PRD(DoR) → 方案ADR → Coding+CI → CD 部署测环境 → 多轮测试 → Test Report → UAT(DoD) → CD 上线
- **Q2 完成报告模板** = 按 9 阶段调整
- **Q3 CICD 闭环** = CI 后, CD 到 Dev 与 Test 双集群才算闭环
- **Q4 完成报告归档** = **Test Report 放测试用例.md** (而非 PRD-Review/), 体现 PRD → Task → 测试用例 三级闭环 ("一个 PRD Feature 由多个测试用例组成")

**落地** (2026-06-03):
- ENGINEERING.md §7 重写: 5 阶段 → 9 阶段, 含完整责任人 + 通过线
- ENGINEERING.md §7.2 Test Report 模板: 位置改测试用例.md + PRD-Task-TC 三级映射
- ENGINEERING.md §7.5 CICD 双集群闭环具体落地 + 派生 task #170 (cd.yaml 改 push to main → dev+test 双 CD)

---

## D-WAIT-004 ORIG (历史问题, Mars 已回, 保留作 audit trail)

**触发时间**：2026-06-02 Mars 3h 委托第 2 件事
**严重度**：🟡 规范性 — 不阻断 demo, 但所有就绪 PRD 进研发后用本流程, 需 Mars 确认

**Mars 原话**: "决策：统计每个 PRD 的工程周期（从 Coding → Testing → 至到出功能完成的报告（按照我们的测试用例.md） CICD，要形成闭环）"

**我落地为 ENGINEERING.md §7 工程周期闭环 5 阶段 + 完成报告模板 + 状态机映射 + 测试可接力线** (2026-06-02 commit 落)。详见 ENGINEERING.md 内容。

**Mars 需确认 (拍/调)**:

### Q1 — 5 阶段拆是否合理
```
Coding → Testing → Verify (Rule D) → 完成报告 → CICD 闭环
```
我把"Verify-before-ship (Rule D)" 独立成 3 阶段, 跟 "完成报告" 分开。如果你认为应该合并 (e.g. 完成报告里附 verify 证据即可), 跟我说。

### Q2 — 完成报告模板是否需 trim 或增段
当前模板 6 段 (PRD 范围 / DoD 对账 / 测试报告 / Verify 证据 / 卡点留尾 / CICD 闭环 todo)。是否够? 缺什么?

### Q3 — CICD 闭环具体落地路径
当前 §7.1 第 5 阶段写 "CHANGELOG + dashboard + PROJECT-STATUS + PRD.md 状态机 4 步"。**是否 CI 自动化**? (e.g. PR merge 时 GitHub Actions 自动 detect 完成报告 → 自动 bump dashboard PRDS 状态 → 自动开新 PROJECT-STATUS PR 让 Mars 拍)。如要自动化, 建新 task。

### Q4 — 完成报告归档位置
当前我建议放 `PRD-Review/PRD-XXX-COMPLETION-REPORT-YYYY-MM-DD.md`。这跟评审报告同目录。是否拆 `PRD-Reports/` 单独目录?

**我现在的做法**: 先按这套规范走, 你回来调。**未确认前, 已就绪 PRD 进研发时按 5 阶段执行, 完成报告按模板写**。
