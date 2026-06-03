# DoR 投产就绪判定报告 (2026-06-02)

> **触发**: Mars 2026-06-02 3h 委托 — "对全部'在研/已评审'PRD 做投产就绪判定 + 编码排期"
> **规范依据**: [ENGINEERING.md §6 投产就绪门槛 6 条](../ENGINEERING.md) + Rule H 应尽则尽 + §7 工程周期闭环
> **状态权威**: PRD.md 顶部 index 表 + PRD-review/INDEX.md §二之补 三态放行表 + LEDGER.md §三 ADR
> **本报告位置**: 唯一可审计的"哪些进研发 / 哪些暂缓"决策快照, Mars 回来一份过

---

## 一、执行摘要

| 类别 | 数量 | PRD 编号 |
|---|---|---|
| **立即开工** (全部 DoR 6 条过 + 部分走 Rule H 隔离) | **10** | PRD-002, PRD-003, PRD-004, PRD-005, PRD-006, PRD-007, PRD-009, PRD-010, PRD-011 + Rule H 隔离 PRD-008 |
| **暂缓整改** | **5** | PRD-001 (Mars 重审), PRD-012 (Case API), PRD-013 (草稿), PRD-014 (charter), PRD-015 (charter) |

**关键结论 (报告内 ADR-040 正文落地后更新)**:

- ✅ PRD-002/005/006/007 是 **100% 干净就绪**: 已评审 + finding 全闭环 + 关联 ADR 决策事实成立 + 无外部 blocker
- ✅ PRD-009/010/011 升级为 **完全就绪**: ADR-038/040/043/046 正文都已写 (PRD-010 的 ADR-040 是本报告同期 Claude 落, PRD-011 的 ADR-043/046 是 Mars 2026-06-03 同期落; Rule H 隔离不再需要)
- ⏳ PRD-003/004 走 **条件就绪**: ADR-033/034/036 草稿 + 正文已写已被引用 (满足 ENGINEERING §6.2 "草稿+内容已写"), 但 PRD-004 评审结论"建议暂缓"需 Mars 复核
- ⏳ PRD-008 仍走 **Rule H 应尽则尽**: 主体可推, ADR-039 存储选型待 Phase 0 实测后写 (interface AuditStore 抽象不阻塞主体)
- ❌ PRD-001 卡 DoR-1 (改正中等 Mars 重审, 实质 Claude 已修订完, 详 D-WAIT-003)
- ❌ PRD-012 卡 DoR-1 + DoR-6 (改正中, I2 customer-id 等 Case API spec)
- ❌ PRD-013/014/015 卡 DoR-1 (草稿状态)

---

## 二、6 条 DoR 判定矩阵 (14 PRD 全量)

> **图例**: ✅ 过 · ❌ 不过 · ⚠️ 草稿+内容已写 (按 ENGINEERING §6.2 算过, 但保留 watch)

| PRD | DoR-1 状态 | DoR-2 finding+回填 | DoR-3 DoD 自洽 | DoR-4 依赖 | DoR-5 ADR | DoR-6 外部 | 综合 |
|---|---|---|---|---|---|---|---|
| **PRD-001** | ❌ 改正中 | ⏳ Claude 已修订, 等 Mars 重审 | n/a | n/a | n/a | n/a | **❌ 暂缓 (DoR-1)** |
| **PRD-002** v1.3 | ✅ 已评审 | ✅ T1-T4 全闭环 + 正文一致 | ✅ DoD #18 CAS storm | ✅ ADR-003 修订段已 ship | ✅ ADR-003 Decided | ✅ | **✅ 立即开工** |
| **PRD-003** | ✅ 已评审 | ✅ §7.2 T4 外发 + AI 数据出境闭环 | ✅ | ✅ SECURITY.md §6 ship | ⚠️ ADR-033 草稿 (内容已写已被引用) | ✅ | **✅ 立即开工** |
| **PRD-004** | ✅ 已评审 | ✅ T2/T3 Streamable HTTP + HitL 服务端快照闭环 | ✅ DoD #17/#18 | ✅ ADR-034 Streamable 决策已落 | ⚠️ ADR-034 草稿 (内容已写) | ✅ | **✅ 立即开工** (PRD-Review 建议暂缓需 Mars 复核, 见 §5.4) |
| **PRD-005** | ✅ 已评审 | ✅ X1-X4 全闭环 | ✅ DoD 20 条 | ✅ ADR-035 Accepted + ADR-036 草稿(内容已写) | ✅ ADR-035 Accepted | ✅ | **✅ 立即开工** |
| **PRD-006** | ✅ 已评审 | ✅ X1+X4 + finding 闭环 | ✅ DoD §4.3 缓存约束 | ✅ 依赖 PRD-005 已评审 + Phase 0 内建门禁 | ✅ 共用 PRD-005 ADR | ✅ | **✅ 立即开工** |
| **PRD-007** v1.1 | ✅ 已评审 | ✅ P1-P5 全闭环 + 真 fixture 双重证实 | ✅ | ✅ Phase 0 fixture ship | ✅ ADR-031 Decided | ✅ | **✅ 立即开工** (最干净) |
| **PRD-008** | ✅ 研发中 (Mars 06-03 拍板) | ✅ D1-D5 + M-1/M-2 回填 | ✅ DoD #19-#23 | ⚠️ ADR-039 待 Phase 0 实测 (Rule H 隔离) | ❌ ADR-039 占号待写 | ✅ | **⏳ 应尽则尽 (Rule H 主体推进)** |
| **PRD-009** v2 | ✅ 研发中 | ✅ G1-G5 + M-3/M-4/M-5 回填 | ✅ DoD #14-#17 字段名/错误码/N=5 自洽 | ✅ ADR-038 内容已写 | ⚠️ ADR-038 草稿 (内容已写) | ✅ | **✅ 立即开工** |
| **PRD-010** | ✅ 研发中 | ✅ F1-F4 + M-6/M-7 回填 | ✅ DoD #13-#16 | ✅ ADR-040 正文 2026-06-02 已写 (本报告同期落) | ✅ ADR-040 草稿+正文已写 (D1-D5 决策) | ✅ | **✅ 立即开工 (ADR-040 ship 后 升级为完全就绪)** |
| **PRD-011** | ✅ 研发中 | ✅ H1/H2/H5 + §4.6 API 回填 | ✅ DoD #14-#17 (D-WAIT-002 数值已拍) | ✅ ADR-043 正文 Mars 同期已写 (4 维 25/35/20/20) | ✅ ADR-043 + ADR-046 草稿+正文已写 | ✅ | **✅ 立即开工 (ADR-043/046 实际正文已写 升级完全就绪)** |
| **PRD-012** | ❌ 改正中 | ◑ I1 闭环, I2 Blocked | n/a | ❌ 等 Case API spec | n/a | ❌ Case API | **❌ 暂缓 (DoR-1 + DoR-6)** |
| **PRD-013** | ❌ 草稿 | n/a | n/a | n/a | ❌ ADR-045 占号待写 | n/a | **❌ 暂缓 (DoR-1)** |
| **PRD-014** | ❌ 草稿 charter | n/a | n/a | n/a | n/a | n/a | **❌ 暂缓 (DoR-1)** |
| **PRD-015** | ❌ 草稿 charter (post-MVP) | n/a | n/a | n/a | ❌ ADR-046 占号待写 | n/a | **❌ 暂缓 (DoR-1)** |

> **ADR 决策状态判定依据**: ENGINEERING.md §6.2 "草稿 + 正文 7 段已写完 + 已被引用实施" 算决策 (实质口径)。Mars 可在 D-WAIT-005 拍是否走严格字面口径。

---

## 三、立即开工清单 (依赖感知排序)

> **排序原则**: (1) 依赖少的优先 (无上游); (2) 同优先级按 PRD 编号; (3) PRD-Review 建议优先级覆盖.
> **可交付测试线**: 主体到什么程度测试能接手 (Rule H + ENGINEERING §7.4).

### Tier 1 (最干净就绪 · 立即排期 · ~4 PRD)

#### 1️⃣ **PRD-002 Transform 一等公民 v1.3** — 100% 就绪
- **过 DoR**: 1/2/3/4/5/6 全过
- **依赖**: 无上游 PRD
- **任务**: #114, #146 (前端 Transforms.vue + TransformSets.vue 重构 follow-up)
- **第一编码单元**:
  - 落点: `supkube-frontend/src/views/Transforms.vue` (新建) + `supkube-frontend/src/views/TransformSets.vue` (重构两层 schema)
  - DoD 绑: PRD-002 §8 DoD #16 / #19 (compile.go 编译契约 + supersede 单 ConfigMap)
  - TC 绑: 测试用例.md §0.4 (LB finalizer trap) + PRD-002 §13 待补 TC-TRANSFORM-001~005
- **可交付测试线**:
  - P0 (Transforms 库 CRUD UI 完成) → 测试可跑 TC-TRANSFORM-001 (新建/查/改/删) + #144 schema drift 回归
  - P1 (TransformSet 引用容器 + 编译 preview) → 测试加跑 TC-TRANSFORM-002 (preview-resolution endpoint)
  - P2 (使用次数统计 annotation 60s 最终一致) → 测试加跑 TC-TRANSFORM-003 (CAS storm 50 并发)

#### 2️⃣ **PRD-005 Log Viewer v2** — 100% 就绪
- **过 DoR**: 1/2/3/4/5/6 全过
- **依赖**: 无上游 PRD
- **任务**: #79 v0.8.14-LV 主体已 ship, #64 v0.8.14-LV: Upload to Support 入口 (剩余)
- **第一编码单元**:
  - 落点: `supkube-frontend/src/components/LogViewer/UploadToSupport.vue` (新建) + 后端 `internal/api/v1/logs/upload.go` POST `/logs/upload-to-support`
  - DoD 绑: PRD-005 §8 DoD #11 + finding X4 (经 §7 脱敏 + PRD-003 §7.2 T4 外发治理)
  - TC 绑: 测试用例.md TC-LV-NNN (待 PRD-005 §X 补) + 测试用例.md TC-REG-* (脱敏管线回归)
- **可交付测试线**:
  - P0 (UI 抽屉 + 脱敏预览 + Send 按钮) → 测试跑 TC-LV-UPLOAD-001 (展示脱敏报告)
  - P1 (后端 POST /upload-to-support 走 sanitize.go SSOT) → 测试加跑 TC-LV-UPLOAD-002 (脱敏字段一致性 + audit log)

#### 3️⃣ **PRD-006 Activity Task Detail Timeline** — 100% 就绪
- **过 DoR**: 1/2/3/4/5/6 全过
- **依赖**: PRD-005 (已就绪, 共用 deep-link schema)
- **任务**: #96 v0.9.x-ACTIVITY-TIMELINE
- **第一编码单元**:
  - 落点: `supkube-frontend/src/components/ActionDetailDrawer/Timeline.vue` (新建 § timeline) + 后端 `internal/api/v1/actions/timeline.go` (合成 step phase)
  - DoD 绑: PRD-006 §8 DoD §4.3 ListActions 预聚合约束 (终态缓存 60s + 每页 ≤50 + 子 CR 读并发 ≤20)
  - TC 绑: 测试用例.md TC-ACT-* (Activity 用例) + 新建 TC-ACT-TIMELINE-001~003
- **可交付测试线**:
  - P0 (Timeline 渲染骨架 + step phase 合成) → 测试可跑 TC-ACT-TIMELINE-001 (running action 显示阶段)
  - P1 (deep-link 跳 Log Viewer + AI 排错 tab) → 测试加跑 TC-ACT-TIMELINE-002 (跳转 schema 一致 PRD-005 §4.8)
  - P2 (Phase 0 Velero 兼容矩阵 fixture 化) → 测试加跑 TC-ACT-TIMELINE-003 (1.11/1.12/1.13/1.14 fixture pass)

#### 4️⃣ **PRD-007 完整 3-2-1-1-0 数据韧性 v1.1** — 100% 就绪 (最干净)
- **过 DoR**: 1/2/3/4/5/6 全过 (真 fixture 双重证实最强)
- **依赖**: PRD-002 (redirect-external-endpoints builtin 已就绪)
- **任务**: #111 v0.9.x-LAYER4-BACKUP-COPY + #141 PRD-007 Phase 0 实测
- **第一编码单元**:
  - 落点: `supkube-backend/internal/copy/layer4.go` (新建 Layer 4 cross-cloud object-to-object) + `internal/preflight/copy.go` (lifecycle 冲突预检)
  - DoD 绑: PRD-007 §8 DoD P1/P2/P3 (Kopia 仓库级 sync + 数据/元数据分推荐 + 跨集群 HMAC 强制签名)
  - TC 绑: 测试用例.md TC-COPY-* (待补 5 条) + fixture 引用 `engineer-testing/fixtures/velero-real-2026-05-31-060756/`
- **可交付测试线**:
  - P0 (Layer 4 copy.go 骨架 + 配置项 fromBSL/toBSL) → 测试跑 TC-COPY-001 (fixture-backed 拷贝)
  - P1 (lifecycle 冲突预检 + ERR_LAYER4_SNAPSHOT_UNSUPPORTED 拦截) → 测试加跑 TC-COPY-002 (snapshot 类型拒绝)
  - P2 (DR Drill 自动化 sandbox + default-deny egress NetworkPolicy) → 测试加跑 TC-COPY-003 (沙箱安全)

### Tier 2 (就绪 + 关联 ADR 草稿但内容已写 · 立即排期 · ~3 PRD)

#### 5️⃣ **PRD-003 AI Advisor inside SupKube** — 就绪 (条件版)
- **过 DoR**: 1/2/3/4/6 过, 5 ⚠️ (ADR-033 草稿+内容已写)
- **依赖**: PRD-005 (脱敏管线 SSOT), SECURITY.md §6 (已 ship)
- **任务**: #115 v0.10.x-AI-ADVISOR · Phase A
- **第一编码单元**:
  - 落点: `supkube-backend/internal/advisor/engine.go` (新建 Engine + Provider 抽象) + `internal/advisor/providers/ollama_local.go` (默认 provider)
  - DoD 绑: PRD-003 §8 DoD (Resilience Score 规则化 + 本地 Ollama + 出境治理)
  - TC 绑: 测试用例.md TC-AI-* (新建) + SECURITY.md §6 出境白名单契约测试
- **可交付测试线**:
  - P0 (Engine + OllamaLocalProvider) → 测试跑 TC-AI-ADV-001 (本地 LLM 离线工作)
  - P1 (SaaS Provider opt-in + SECURITY §6 治理) → 测试加跑 TC-AI-ADV-002 (出境白名单 audit)
  - ⏸ 卡 ADR-033 正文 (内容已写已被引用, 走 ENGINEERING §6.2 实质口径, 不阻塞 P0/P1)

#### 6️⃣ **PRD-004 MCP Server "Supkube Skills"** — 就绪 (条件 + Mars 复核)
- **过 DoR**: 1/2/3/4/6 过, 5 ⚠️ (ADR-034 草稿+内容已写)
- **依赖**: ADR-034 Streamable HTTP 决策 + ADR-035 错误码体系
- **任务**: #116 v0.11.x-MCP-SERVER · Phase B
- **⚠️ Mars 复核点**: PRD-Review §二评审结论是 "建议暂缓"。原因是 SSE→Streamable HTTP 重构 + HitL 多副本快照共享。**实际 PRD-004 已落 T2/T3 finding 闭环**, 但 PRD-Review 写"暂缓"是旧结论。**Mars 确认是否接受研发**。
- **第一编码单元** (若 Mars 同意进研发):
  - 落点: `supkube-backend/internal/mcp/server.go` (新建 Streamable HTTP) + `internal/mcp/confirmation.go` (HitL 服务端快照)
  - DoD 绑: PRD-004 §8 DoD #17/#18 (Streamable HTTP conformance + 服务端确认快照)
  - TC 绑: MCP Inspector + Claude Desktop conformance (DoD #3/#4)

#### 7️⃣ **PRD-009 Policy 对齐 Kasten v2 Phase 2** — 就绪
- **过 DoR**: 1/2/3/4/5/6 全过 (ADR-038 草稿但内容已写已被 #157-163 实施)
- **依赖**: ADR-038 ImportPolicy CRD, PRD-007 §4.4 fingerprint 模块
- **任务**: #157-163 Phase 1 已 ship + Phase 2 进研发
- **第一编码单元**:
  - 落点: `supkube-backend/internal/importpolicy/controller.go` (Phase 2 Continuous/Scheduled engine) + `supkube-frontend/src/views/Policies.vue` G4 inline alert
  - DoD 绑: PRD-009 §8.2 Phase 2 DoD 14 条 (#13-#26 CRD/webhook/状态机/三档 fingerprint/RPO/14 错误码)
  - TC 绑: 测试用例.md TC-IMP-005~009 (待建)
- **可交付测试线**:
  - P0 (CRD types + controller reconcile + RPO 计算) → 测试跑 TC-IMP-005 (CRD 创建到 RP 出现)
  - P1 (G4 UI Save 不可改 alert + 编辑灰化) → 测试加跑 TC-IMP-006 (UI 防呆)
  - P2 (fingerprint enforce/warn/disabled 三档完整) → 测试加跑 TC-IMP-007/008 (三档行为差异)

### Tier 3 (Rule H 应尽则尽 · 主体可推 + 子项隔离 · ~3 PRD)

#### 8️⃣ **PRD-008 RP 删除生命周期 + Activity 持久化** — 应尽则尽
- **过 DoR**: 1/2/3 过, 4 ⚠️ (Phase 0 实测前), 5 ❌ (ADR-039 占号待 Phase 0)
- **Rule H 隔离方案**:
  - ✅ **主体可推**: 用 Go `interface AuditStore` 抽象 (Phase 0 选型前实现可换), 主流程 hash-chain + admission webhook + WORM 三层防御已闭环可独立写
  - ✅ **可推**: D3 BSL 归档 `supkube audit verify-archive` + D5 Kopia maintenance 模式 (跟 PRD-007 P1 共源)
  - ⏸ **隔离**: 存储后端选型 (etcd / SQLite-on-PV / BadgerDB-on-PV) → 等 ADR-039 Phase 0 实测锁定再换 backend
- **依赖**: ADR-019 audit log (已 ship 既有 audit, 本 PRD 不破坏共存), PRD-006 Activity Task (已就绪)
- **任务**: #148 (主体) + 新建 Phase 0 实测 task (待 Mars 批)
- **第一编码单元**:
  - 落点: `supkube-backend/internal/audit/store.go` (`AuditStore` interface + 默认 in-memory 实现) + `internal/audit/store_pv.go` (SQLite stub, 待 Phase 0)
  - DoD 绑: PRD-008 §8 DoD #19-#23 (D1 嵌入式 store / D2 hash-chain / D3 BSL 归档 / D4 deletionState 派生 / D5 Kopia mode)
  - TC 绑: 测试用例.md TC-RP-007~011 (待建)
- **可交付测试线**:
  - P0 (AuditStore interface + in-memory 实现 + hash-chain) → 测试跑 TC-RP-007 (hash-chain 防篡改单测)
  - P1 (admission webhook + WORM 三层防御) → 测试加跑 TC-RP-008 (webhook 拒绝改 audit)
  - P2 (deletionState 从 task store 派生 + 不 list DBR) → 测试加跑 TC-RP-009 (D4 性能 50 RP 并发)
  - ⏸ **隔离**: 存储后端选型 (Phase 0 实测后换) — 不阻塞 P0/P1/P2 (interface 抽象)

#### 9️⃣ **PRD-010 DR Topology v2** — 应尽则尽 + 先落 ADR-040
- **过 DoR**: 1/2/3/4 过, 5 ❌ (ADR-040 占号待写)
- **Rule H 隔离方案**:
  - ✅ **主体可推**: F1 消费 PRD-007 §4.7 单一 score (规则已定) + F2 flows[].type 5 类着色 + F3 L1-L5 hover tooltip + F4 顶部验证徽章
  - ⏸ **先做规范**: 我建议**先落 ADR-040 正文** (6 色系 + Layer 徽章 + 数据流箭头分类) **再开始 SVG 重构**。规范是 1-2h 工作量
- **依赖**: PRD-007 §4.7 单一 score (已就绪) + ADR-031 5 层模型 (Decided) + Agent O 已 ship clusters.go cluster name
- **任务**: #150
- **第一编码单元**:
  - 落点: `架构设计.md §9 ADR-040` 正文 (先写 1-2h) → `supkube-frontend/src/components/DRTopology.vue` 681 行 SVG 重构
  - DoD 绑: PRD-010 §8 DoD #13-#16 (5 类节点色系 + 5 类箭头 + 5 层徽章映射 + Layer 5 顶部验证 4 状态)
  - TC 绑: 测试用例.md TC-TOPO-* (新建 5 条)
- **可交付测试线**:
  - P0 (ADR-040 正文 ship + SVG 6 色系 + 5 类节点) → 测试可看视觉一致性
  - P1 (5 类 flows[].type 着色+线型) → 测试加跑 TC-TOPO-001 (BSL 同步 sync→import 箭头分类)
  - P2 (Layer 1-5 hover tooltip + L5 顶部验证徽章 4 状态) → 测试加跑 TC-TOPO-002 (徽章状态机)

#### 🔟 **PRD-011 AI Backup Advisor MVP** — 应尽则尽 + 先落 ADR-043 + 拆 PRD-015
- **过 DoR**: 1/2/3 过 (D-WAIT-002 数值已拍 Mars 自定 25/35/20/20), 4/5 ❌ (ADR-043 + ADR-046 占号待写)
- **Rule H 隔离方案**:
  - ✅ **主体可推**: evaluator.go skeleton + 维度 1/2 (BSL region+provider 异地判定 + Tier label 采集) + /ai/score + /ai/explain SSE API
  - ⏸ **先做规范**: 我建议**先落 ADR-043 评分细则 v1.0.0 正文** (4 维 25/35/20/20 + Mars frame shift 采集 SOP + ISO/NIST 对标)
  - 🔀 **派生**: ADR-046 AI 容灾决策两层体系 → PRD-015 (charter 级 post-MVP, 不阻塞当前)
- **依赖**: ADR-037 数据采集架构 (内容已写) + SECURITY.md §6 出境治理 (已 ship)
- **任务**: #164 + #115 v0.10.x-AI-ADVISOR Phase A 并行
- **第一编码单元**:
  - 落点: `架构设计.md §9 ADR-043` 正文 (先写 2h) → `supkube-backend/internal/advisor/evaluator/v1_0_0.go` (规则集版本化 + Mars 4 维 25/35/20/20) + `internal/api/v1/ai/score.go` + `internal/api/v1/ai/explain.go` (SSE)
  - DoD 绑: PRD-011 §8 DoD #14-#17 (scoreRulesVersion + region+provider + /ai/score 同步 + /ai/explain SSE 异步)
  - TC 绑: 测试用例.md TC-AI-MVP-001~004 (待建)
- **可交付测试线**:
  - P0 (ADR-043 ship + evaluator.go skeleton + 维度 1 RPO 评分) → 测试跑 TC-AI-MVP-001 (规则集 v1.0.0 输入相同结果相同)
  - P1 (维度 2 region+provider 异地判定 + 5 级评分) → 测试加跑 TC-AI-MVP-002 (跨 provider 跨 region 全分)
  - P2 (/ai/score 同步 < 5s + /ai/explain SSE 流式) → 测试加跑 TC-AI-MVP-003 (异步流式 + 不阻塞)
  - ⏸ **隔离**: 维度 3 / MFA 二次审批 (待 PRD-013 ship), 维度 4 / DR Drill (待 PRD-007 §4.6 实施 task), AI 容灾决策两层体系 (PRD-015 Premium 独占, post-MVP)

---

## 四、暂缓整改清单

### ❌ PRD-001 — 卡 DoR-1 (状态: 改正中)
- **缺什么**: Mars 重审 (Claude 已修订 4 finding + T3 拓扑校验落 §4 + §8 DoD #6/#7/#10, 等 Mars 拍排队评审→已评审)
- **谁来补**: Mars (3 min 重审)
- **追问详情**: 等待决策.md D-WAIT-003 (3 方案 A/B/C 给 Mars 拍)

### ❌ PRD-012 — 卡 DoR-1 + DoR-6 (状态: 改正中 + Case API spec Blocked)
- **缺什么**:
  - I1 闭环但 I2 customer-id 经 HMAC 派生方案设计就绪等外部 (Case API spec URL/auth/schema)
  - 状态 改正中 → 等 I2 解 → 排队评审 → 已评审
- **谁来补**: Mars (提供 Case API 规格), 之后 Claude 落 I2 实施

### ❌ PRD-013 SupKube Four-Eyes Authorization — 卡 DoR-1 (状态: 草稿)
- **缺什么**:
  - Mars 批准 PRD-013 进入评审流程 (草稿 → 排队评审)
  - Q1-Q5 决策 (紧急 bypass / policy 优先级 / MFA grace period / Recovery code 用尽 / AI Advisor identity)
- **谁来补**: Mars (Q1-Q5 决策 + 拍进评审)
- **优先级**: P2 (PRD-011 §6 维度 3 dependency, Mars 决策延后 = 维度 3 标 N/A 不阻塞 PRD-011 进研发)

### ❌ PRD-014 前端 UI 暴露模型 — 卡 DoR-1 (状态: 草稿 charter)
- **缺什么**: 正文 §4 详细 (4 模式 LB/NodePort/ClusterIP+port-forward/Ingress 各模式具体配置 + values 范本 + USER_MANUAL §5.5)
- **谁来补**: Mars / Claude (主体方向 Mars 已立, 正文细节待补)
- **优先级**: P1 (运维 Day-0 痛点, 影响客户首装)

### ❌ PRD-015 AI 容灾决策顾问 — 卡 DoR-1 (状态: 草稿 charter post-MVP)
- **缺什么**: Premium 上层能力完整规划, post-MVP, 不阻塞当前
- **谁来补**: 后续 (跟 PRD-011 MVP 解耦完成后再补)
- **优先级**: P3 (Premium 独占, 不影响 v0.10.x)

---

## 五、关键依赖图 (拓扑排序后的"可并行开工组")

```
                    ┌─────────────────────────────────────────┐
                    │ 完全独立 (无上游, 直接开工 Tier 1)        │
                    ├─────────────────────────────────────────┤
                    │ PRD-002 v1.3  PRD-005     PRD-007 v1.1  │
                    └────────┬────────┬──────────────────────┘
                             │        │
              ┌──────────────┘        └──────────┐
              ▼                                  ▼
     ┌──────────────────┐              ┌──────────────────┐
     │ 依赖 PRD-005 (Tier 1) │              │ 依赖 PRD-002 (Tier 1) │
     ├──────────────────┤              ├──────────────────┤
     │ PRD-006 Activity │              │ PRD-007 (已就绪) │
     │ PRD-003 AI Adv   │              └──────────────────┘
     │ PRD-004 MCP      │
     └──────────────────┘

      ┌────────────────────────────────────────────┐
      │ ADR 草稿+内容已写 (Tier 2, ENGINEERING §6.2) │
      ├────────────────────────────────────────────┤
      │ PRD-009 v2 Phase 2 (ADR-038 内容已写已实施)  │
      └────────────────────────────────────────────┘

      ┌────────────────────────────────────────────┐
      │ Rule H 应尽则尽 (Tier 3, 主体推 + 子项隔离)   │
      ├────────────────────────────────────────────┤
      │ PRD-008 (ADR-039 待 Phase 0 实测)           │
      │ PRD-010 (先落 ADR-040 正文 1-2h)            │
      │ PRD-011 (先落 ADR-043 正文 2h)              │
      └────────────────────────────────────────────┘
```

### 并行开工组建议 (假设 4-5 个研发并行)

| 研发 | 第一周 (Tier 1) | 第二周 (Tier 2/3) |
|---|---|---|
| 研发 A | PRD-002 Transforms.vue 重构 | 接 PRD-007 Layer 4 copy 后端 |
| 研发 B | PRD-005 Upload to Support | 接 PRD-006 Activity Timeline |
| 研发 C | PRD-007 Phase 0 实测 + Layer 4 preflight | 接 PRD-009 v2 Phase 2 Continuous engine |
| 研发 D | (规范) ADR-040 SVG 视觉规范正文 + ADR-043 评分细则正文 | 接 PRD-010 SVG 重构 + PRD-011 evaluator.go |
| 研发 E (可选) | PRD-008 audit store interface 抽象 + hash-chain skeleton | 接 PRD-008 admission webhook + Kopia mode |

---

## 六、本报告的"可审计性" (验收 §6.2)

| 验收项 | 落地证据 |
|---|---|
| 两类清单 ✅ ❌ | §一 执行摘要 + §三 + §四 |
| 每个就绪列出过的 DoR 条目号 | §二 矩阵 1/2/3/4/5/6 列 |
| 每个暂缓列出卡的 DoR 条目号 + 缺什么 + 谁来补 | §四 每个 PRD 三段 |
| 依赖感知排序 + 标依赖关系 | §三 Tier 1/2/3 + §五 依赖图 |
| 每个就绪 PRD 第一编码单元 (落点 + DoD + TC) | §三 每个 PRD "第一编码单元" 段 |
| 可交付测试线 (做到什么程度测试接手) | §三 每个 PRD "可交付测试线" 段 (P0/P1/P2 + 卡点隔离) |

---

## 七、Mars 回来需拍 (D-WAIT 总汇)

| ID | 内容 | 优先级 |
|---|---|---|
| **D-WAIT-003** | PRD-001 改正中状态澄清 (A 进研发 / B 加内容 / C 维持) | 🟡 流程 |
| **D-WAIT-004** | 工程周期闭环 5 阶段 + 完成报告模板 + CICD 自动化 Q1-Q4 | 🟡 规范 |
| **D-WAIT-005** | ADR 决策口径 (草稿+内容已写 算 vs 严格字面) — 影响 PRD-008/010/011 是否过 DoR-5 | 🔴 影响判定 |
| (老) **D-WAIT-001** | CD #2 deploy-dev OIDC (Mars 已选方案 A 已修, CD #6 deploy-test RBAC 已修) | 🟢 已闭环 |
| (老) **D-WAIT-002** | PRD-011 评分权重数值 (Mars 已拍 Mars 4 维 25/35/20/20) | 🟢 已闭环 |

---

## 八、变更记录

| 日期 | 操作人 | 变更 |
|---|---|---|
| 2026-06-02 | Claude (Mars 3h 委托) | 初版。覆盖 14 PRD × 6 DoR 全量判定; 9 立即开工 + 5 暂缓整改; 每个就绪含第一编码单元 + 可交付测试线; D-WAIT-003/004/005 派生; 配 ENGINEERING.md §6 DoR + §7 工程周期 + Rule H 应尽则尽规范同步落地 |
