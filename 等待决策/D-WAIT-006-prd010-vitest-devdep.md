# D-WAIT-006 — PRD-010 引入 Vitest 当 devDependency 是否同意

> **状态**：open（待 Mars 选 A/B）｜**owner**：Mars｜**触发**：2026-06-04
> **取号说明**：本条在旧 `等待决策.md` 里**无号**（PRD-010 实施 agent 临时追加未取号）。2026-06-04 SCM 经 LEDGER 正式编为 **D-WAIT-006**（canonical 无号项的最低空号）。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。

**触发时间**：2026-06-04（PRD-010 实施过程中, Auto 模式）
**严重度**：🟡 中 — 不阻塞 (我已先装上跑通了, 待 Mars 追认或回退)
**触发原因**：

PRD-010 §8 DoD 含"TC-TOPO-001~005 5 个单测"。supkube-frontend 在我接手时**没有**任何测试框架 (devDependencies 仅 `@vitejs/plugin-vue + vite`)。我做了:

- `npm install --save-dev vitest@^1.6.0 @vue/test-utils@^2.4.0 jsdom@^24.0.0` (3 个 devDep)
- 新加 `npm run test` script + `vitest.config.js`
- 写 `src/components/DRTopology.test.ts` (14 测试, 全绿)
- 共增 195 transitive packages (`npm audit` 报 3 moderate + 1 critical, 均 vitest/jsdom 间接依赖, 不进 prod bundle)

**为什么自拍**:

- Task brief 写明"如果未配, 加 Vitest 不算扩功能范围 (是测试基础设施)"
- 我同时把决策点写到本文件让 Mars 二选一
- vitest 不进 prod bundle (devDependencies only), Helm/Docker 镜像不受影响
- 已跑 `npm run build` 通过, 现有功能 0 影响

**两个选项**:

- ✅ **选项 A (推荐, 我已做)**: 保留 vitest + @vue/test-utils + jsdom devDep, TC-TOPO 走测试; 后续 PRD 测试基础设施已就位
- 🔁 **选项 B (回退)**: `npm uninstall vitest @vue/test-utils jsdom` + 删 vitest.config.js + 删 DRTopology.test.ts; 仅靠 grep CI 守 TC-TOPO-005, 其余 4 TC 退化为手动 QA。**理由**: 195 包看上去多 (但都是 vitest 自身 + jsdom 标准依赖, 不可避免)

**Mars 决策项**: 选 A 还是 B?
