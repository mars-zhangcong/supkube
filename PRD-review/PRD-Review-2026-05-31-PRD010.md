# SupKube PRD 评审报告 — PRD-010

> **视角**：Kubernetes + AI + 灾备（DR）专家
> **评审对象**：PRD-010（DR Topology v2 — Cluster/BSL 视觉重构 + Local Snapshot/Backup Copy 节点 + 5 层模型对齐）
> **评审人**：Claude（受 Zack 委托） · **日期**：2026-05-31
> **核对基线**：PRD.md、架构设计.md（ADR-031 / ADR-025 / ADR-026 / 拟 ADR-038）、PRD-003 / PRD-007 / PRD-009
> **承接**：PRD-Review 第五份；前四份见 INDEX

---

## 一、执行摘要

PRD-010 把 DR Topology SVG 从"Cluster + BSL 二元节点 + 紫色撞色 + 缺 Layer 1/4/5"重构为对齐 ADR-031 五层模型的 6 类节点 + Layer 徽章 + 数据流箭头 + Posture 总分。它解决的两个真问题（Cluster 与 Cloud BSL 紫色撞色、看不出 3-2-1-1-0 谁有谁没）确实值得做；后端 aggregator 只加 `localSnapshots/backupCopies/virtualLabs/posture` 三段数组，0 兼容风险；scope 也很克制（不换 SVG 引擎、不做拖拽/多集群/PNG 导出，串行依赖用文档兜底）。a11y / 暗色 / i18n / 响应式都考虑到了。

这是一份低风险的前端重构，**基本可过**。但因为它是**面向客户、可截图当合规材料**的灾备可视化，有三件事必须在实现前定准，否则"护城河"会变成"给客户看了一张不准确的图"：(1) **Posture 分数口径**——本 PRD 用"已启用层数 × 20"，这既与 PRD-003/PRD-007 的 score 第三次打架，又会把"两份本地副本"误报成 40 分安全感；(2) **数据流箭头是否反映真实路径**——把 snapshot/export 画成 L1→L2→L3 的线性链，与 ADR-025 双 Schedule（snapshot 与 export 是并行独立）的实际不符；(3) **L1-L5 术语**与 PRD-009 刚把 Policy 改成的 Snapshot/Export 形成两套词。

### 放行结论

| 结论 | 关键前置 |
|---|---|
| **基本通过（低风险前端重构）** | F1 Posture 分数统一口径、别用"层数×20"当安全百分比；F2 数据流箭头反映真实路径（snapshot/export 并行，非线性链）；F3 L1-L5 与 PRD-009 Snapshot/Export 术语打通；F4 Layer 5 作为"节点"语义重思 |

**严重度图例**：Blocker / High / Med / Info。

---

## 二、评审发现

### F1（High，DR 诚实度 + 跨 PRD 口径）Posture 分数：别用"已启用层数 × 20"，且要与 PRD-003/007 统一

§4.7 把 Posture = 已启用 Layer 数 × 20。问题有三层：

1. **第三套 score 定义**：PRD-003 §3.3 有 Resilience Score（应用级、5 业务/架构维度）、PRD-007 §4.8 有 Posture Score（集群级、5 层覆盖、声称与 PRD-003 共享引擎）、现在 PRD-010 又定义"层数×20"。三处口径打架，这是 PRD-Review 一直在追的"Score 口径"问题再次扩散。**aggregator JSON 本身已返回 `posture.score`**——前端应**直接消费后端 `posture.score`**，绝不在前端用"层数×20"另算，否则前后端两个数字会不一致。
2. **"层数×20"在 DR 语义上误导**：3-2-1-1-0 的价值在**异地 + 不可变**副本，不在"层多"。一个只有 L1+L2（都在本地同站点）的集群算 40 分，但它**没有任何异地保护**——真出机房级灾难数据全丢，却给客户"40/100，过半安全"的错觉；反过来只有 L3 云端（真异地）可能比 L1+L2 更值钱却只算 20。把"层数百分比"当安全度对一个合规/灾备面板是危险的。
3. **建议**：Posture 分数由**后端单一权威**计算（与 PRD-007/PRD-003 收敛为一套，按 3-2-1-1-0 实际贡献加权，而非层数等权），前端只显示；若暂用层数，UI 文案不要呈现为"安全百分比"，而是中性的"5 层已启用 N 层"。

### F2（Med，可视化真实性）数据流箭头把 snapshot/export 画成线性链，与 ADR-025 双 Schedule 实际不符

§4.2 画 `Cluster → Local BSL（实线）` + `Local BSL → Cloud BSL（sync 虚线）` + `Cloud BSL → Backup Copy（copy 虚线）`，暗示数据 L1→L2→L3→L4 一条线流下去。但按 ADR-025/026 的双 Schedule 模型，**snapshot 与 export 是两个并行独立的 Velero Schedule**——export 通常**直接写它的目标 BSL**（可能就是云 BSL），不是"先写本地 BSL 再 sync 到云"。把它画成"Local BSL 同步到 Cloud BSL"会让客户以为云端副本是本地副本的下游派生（本地坏了云端也会受影响的错觉），与实际的"各自独立从集群导出"不符。

**建议**：箭头反映真实数据路径——snapshot（→L1）与 export（→L2/L3，按 Policy 选的 BSL 直写）是**从 Cluster 并行出发**的两条线；Backup Copy（L3→L4 object copy）才是 BSL 间复制。可视化是"护城河"，更要画对，否则截图当合规材料会失真。

### F3（Med，跨 PRD 术语）L1-L5 节点徽章 vs PRD-009 的 Snapshot/Export

PRD-009 刚把 Policy UI 的 L1/L2 词汇移除、改成 Kasten 风格 Snapshot/Export；PRD-010 却在 Topology 上用 L1-L5 徽章作为核心视觉。结果客户在 Policy 面看到 "Snapshot + Export"、在 Dashboard 面看到 "L1-L5"，两套词。§10 提到要与 PRD-009 chip 文案对齐，但**节点徽章本身是 L1-L5**。

**建议**：要么徽章同时标注动作词（L1=Snapshot / L2-L3=Snapshot Export to local·cloud / L4=Backup Copy），要么在 Topology 加一行"层↔动作"映射图例；与 INDEX 跟踪的"数据保护术语轴"一并收口。好在 PRD-010 的 L1/L2 tooltip（L1=CSI volumesnapshot 不离开集群 / L2=in-cluster MinIO）比 PRD-009 §4.1 的二义更精确——可反过来用它去消除 PRD-009 E1 的"Snapshot 到底是 CSI 还是 MinIO"歧义。

### F4（Med，语义）Layer 5「虚拟实验室」作为拓扑节点不太自洽

L1-L4 是**数据副本所在位置**（存储节点），L5「Virtual Lab / DR Drill」是**一次验证活动**，不是数据驻留地。把它当作数据流拓扑里的对等节点，并画 `Cluster ← Virtual Lab（Restore 单向弧）`，语义上混了"数据在哪"与"是否验证过"——而且 DR Drill 实际是"从 BSL 还原到沙箱 ns"，不是"从虚拟实验室还原回生产集群"。

**建议**：L5 更适合表达为拓扑上的**验证状态印章 / 徽章**（如在 L2/L3 节点上打"✓ 7 天前演练通过"），或一个明确标注"验证活动、非数据副本"的特殊节点，而不是与存储节点同形的对等节点 + 误导方向的箭头。

### 其它（Info）

| # | 严重度 | 问题 | 建议 |
|---|---|---|---|
| 1 | Info | 6 色系靠颜色区分节点类型，绿(L1)/红(失败)、橙(L2)/粉(L4) 在色盲（deuteranopia）下可能难分。 | 已有 icon + Layer 徽章作冗余通道（好）；DoD 加一条色盲可分性检查。 |
| 2 | Info | 失败节点 tooltip 跳 Activity Task 依赖 PRD-008 持久化；Backup Copy/DR Drill 节点依赖 PRD-007。 | 串行依赖已用空数组/文档兜底（Q2/Q3，处理得好）；注明 PRD-008 未落地时失败跳转降级为只显示文案。 |
| 3 | Info | 拟 ADR-038（SVG 视觉规范）未进 ADR 号台账。 | 登记台账，与 ADR-037 一并补。 |
| 4 | Info（赞） | 0 兼容风险的 aggregator 扩字段、不换 SVG 引擎、多集群留 PRD-011、a11y/暗色/i18n 齐备。 | 保持。 |

---

## 三、跨 PRD / 跨 ADR 一致性

- **Score 口径（F1）**：PRD-003 / PRD-007 / PRD-010 三处 score 必须收敛为后端单一权威，前端只显示。这是 INDEX「Score 口径」未决项的再次扩散，建议升级为"必须先定一套 score 定义再实现这三处 UI"。
- **数据保护术语轴（F3）**：PRD-009 Snapshot/Export ↔ PRD-007 Posture L1-L5 ↔ PRD-010 节点 L1-L5，三者需一张统一映射表（INDEX 已有该跟踪项）。
- **可视化真实性（F2）**：拓扑数据流应与 ADR-025/026 双 Schedule 实际一致。
- **依赖**：PRD-010 的 L4/L5 节点数据源 = PRD-007 §4.3/§4.7；失败联动 = PRD-008。本 PRD 已正确用"空数组 + 文档兜底"避免硬串行，排期解耦做得好。
- **ADR-038** 登记台账。

---

## 四、建议的行动优先级

| 序 | 行动项 | 时机 |
|---|---|---|
| 1 | Posture 分数：消费后端单一 `posture.score`，与 PRD-003/007 收敛为一套加权定义；UI 不呈现"层数×20"为安全百分比（F1） | 实现前 |
| 2 | 数据流箭头按 ADR-025 真实路径画（snapshot/export 从 Cluster 并行出发，非 L2→L3 线性 sync）（F2） | 实现前 |
| 3 | L1-L5 徽章与 PRD-009 Snapshot/Export 打通（映射图例）；反用 L1/L2 tooltip 消除 PRD-009 E1 歧义（F3） | 实现期内 |
| 4 | Layer 5 改为验证状态徽章/特殊节点，修正 Restore 箭头方向语义（F4） | 实现期内 |
| 5 | DoD 补色盲可分性检查；ADR-038 登记台账（Info 1/3） | 实现期内 |

*总体评价：低风险、解决真问题、scope 与依赖解耦都干净。作为面向客户的灾备可视化，把"分数别误导、箭头画真实、术语对得上"这三条收口后即可放行——这恰恰是"可视化护城河"能不能立住的关键。*

---

<!--
评审元数据（勿删）
- 评审轮次：PRD-Review 第五份（PRD-010）
- 关键结论：F1 Posture 分数口径（第三套 score，"层数×20"误导）High；F2 数据流箭头与 ADR-025 双 Schedule 不符 Med；F3 L1-L5 vs PRD-009 Snapshot/Export 术语 Med；F4 Layer 5 作为节点语义不自洽 Med。低风险基本通过。
- 关联：ADR-031/025/026/038、PRD-003/007（score 口径）、PRD-009（术语轴）、PRD-007/008（L4/L5/失败联动依赖）
-->
