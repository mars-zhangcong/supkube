# SupKube PRD 评审报告

> **视角**：Kubernetes + AI + 灾备（DR）专家
> **评审对象**：PRD-001 / PRD-002 / PRD-003 / PRD-004
> **评审人**：Claude（受 Zack 委托） · **日期**：2026-05-31
> **核对基线**：PRD.md、架构设计.md（ADR-001~031）、SECURITY.md、测试用例.md、CSI 文档

---

## 一、执行摘要

整体判断：四份 PRD 的产品思考成熟，模型严谨。「谁配置、谁负责」的责任边界原则贯穿 PRD-002/003/004，AI 路线选择「推荐型非自治 + 人确认」是当前阶段最稳的判断，值得肯定。文档结构（Goal→Epic→Story→Function→DoD→开放问题）规范，可追溯性强。

但从 K8s / 灾备 / AI 工程的工程正确性看，仍有若干放行前需处理的问题。本报告第二轮已对照 supkube 仓库内的架构设计.md（含 ADR-001~031）、SECURITY.md、测试用例.md、CSI 文档等做了交叉核对，并据此修正了首轮结论——其中关于 Transform 与 Velero 的关系，ADR-003/006 已给出决策，故该项由「Blocker / 疑似重造轮子」修正为「High / 新两层模型的编译落地需写明」。

当前两条最需优先处理的仍是地基性的：

1. PRD-004 押注的 MCP SSE 传输既被 MCP 官方弃用，也与他们自己 ADR-015「因 nginx ingress 兼容性问题不用 SSE」的既有经验自相矛盾。
2. PRD-003 默认外发集群上下文给第三方 LLM，而 SECURITY.md 与现有 ADR 对此零治理。

### 放行结论速览

| PRD | 结论 | 前置条件 |
|---|---|---|
| **PRD-001** 还原前置检查 | 有条件通过 | 4 处修订后放行：blocker 不可忽略 · severity 后端固化 · 删除 v1 残留注释块 · 标注 SC→Immediate 的多 AZ 拓扑风险 |
| **PRD-002** Transform 两层 | 概念通过 / 补充后研发 | 补 ADR-003 修订：写明新两层模型 + `${VAR}` 参数如何编译为单个 Velero `resourceModifierRef` ConfigMap（data 仅 rules.yaml、len==1）；Phase 0 破坏性迁移复用 ADR-014 机制 + 补迁移测试与回滚；使用统计机制重选 |
| **PRD-003** AI Advisor | 方向通过 | （「重要不紧急」评级合理）放行前提：合规客户默认本地 LLM、不外发集群上下文；Resilience Score 的可复现性方案 |
| **PRD-004** MCP Server | 建议暂缓 | SSE → Streamable HTTP 改型；HitL 二次确认基于服务端持久化快照；多副本下确认状态共享方案 |

**严重度图例**

- **Blocker** — 地基性问题，应在该 PRD 进入研发前解决
- **High** — 影响正确性 / 合规 / 数据安全，放行前需有明确方案
- **Med** — 影响可维护性 / 体验 / 成本，研发期内处理
- **Info** — 建议性优化或文档卫生

---

## 二、跨 PRD 顶层问题（最高优先）

以下四条不属于单一 PRD，而是影响整条依赖链（PRD-001→002，PRD-003→001/002，PRD-004→003）的地基性问题。

### T1（High，首轮为 Blocker，经 ADR 核对后修正）新两层模型如何编译为 Velero 原生 ConfigMap 未写明

首轮我担心 PRD-002 在重造 Velero 的轮子。核对架构设计.md 后修正：**ADR-003** 已明确决策「Transform Set 直接复用 Velero 原生 `Restore.spec.resourceModifierRef`，存为 velero ns 下带 `supkube.io/kind=transform-set` 标签的 ConfigMap」，**ADR-006** 已把三种 Velero patch 类型（JSON Patch / Merge / Strategic Merge）按冲突场景定好。所以执行底座没有问题——SupKube 早已站在 Velero 原生能力上，不是另起炉灶。

但 PRD-002 引入的「新两层模型」与 Velero 原生模型并不一一对应，这正是必须在 ADR 里补写的地方。Velero 的 `resourceModifierRef` 指向「一个扁平 ConfigMap」，且 ADR-003 用血泪教训钉死了硬约束：data 字段只能有一个 key（`rules.yaml`），多 key 会被 Velero 报 `illegal resource modifiers`（`len(Data)==1`）。而 PRD-002 设计的 Transform（独立 ConfigMap，`kind=transform`）+ TransformSet（含 `transformRefs[]` 引用其它 ConfigMap + `${VAR}` 参数注入）在 Velero 里没有对应物——Velero 既不懂 ConfigMap 之间的引用，也不懂参数注入。因此 TransformSet 必须在还原时被「展平 / 编译」成单个符合 Velero 约束的 `resourceModifierRef` ConfigMap（PRD 的 preview-resolution 端点正是这一步）。

需要在 PRD-002 / ADR-003 修订里讲清三点：

1. 明确「编译」步骤——TransformSet + params 在 Trigger Restore 时展平为单个 `data.rules.yaml`、`len==1` 的 ConfigMap，作为 `resourceModifierRef`。
2. §4.1 给出的 builtin Transform YAML 用了 `data.spec` 这个 key，与 ADR-003「data 仅 rules.yaml」冲突，需统一。
3. TransformSet 的「按引用顺序执行」必须对齐 Velero「按 ConfigMap 内规则顺序、同 path 后者覆盖」的实际语义。

本项已从 Blocker 降为 High（执行底座既定，缺的是编译契约的书面化）。

### T2（Blocker）PRD-004 押注 SSE-only 传输，而 MCP 已弃用 HTTP+SSE

MCP 规范在 2025-03-26 版本即弃用 HTTP+SSE 传输，由 Streamable HTTP 取代；官方明确「所有新实现都应使用 Streamable HTTP」，SSE 仅为向后兼容保留。PRD-004 却把「SSE only（非 Stdio）」作为 Mars 拍板的 v1 方案，等于一上来就建在已弃用传输上。

更值得注意的是：这与 SupKube 自己的既有经验冲突。架构设计.md 的 **ADR-015** 在讨论轮询节奏时已明确写下「不用 WebSocket / SSE：复杂度↑，K8s nginx ingress 兼容性问题」——团队对 SSE 在 K8s ingress 下的运维坑早有书面认知，却在对外 MCP Server 上重新选了 SSE-only。安全层面同样冲突：SSE 双端点（GET /sse + POST /messages）因浏览器难以在握手时传安全 header，业界常被迫把 token 放进 URL query，与 PRD 自己的 Bearer Token 设计和「凭证绝不暴露」原则正面冲突。建议 v1 直接采用 Streamable HTTP（其内部可选用 SSE 做服务端流式），旧 SSE 端点仅作兼容；这同时绕开 ADR-015 已记录的 nginx ingress 长连接难题。

### T3（High）创建同名 / Immediate 别名 StorageClass 在多 AZ / 拓扑场景有数据可用性风险

PRD-001（v1 残留方案）与 PRD-002 的 change-storage-class 都涉及「把 `WaitForFirstConsumer` 改为 `Immediate` 以绕开死锁」。但 `WaitForFirstConsumer` 的存在意义正是延迟到 Pod 调度后再 provision 卷，以保证卷被创建在 Pod 所在的拓扑域 / 可用区。强制 `Immediate` 会让卷在调度前就被 provision，对 zonal 块存储（EBS / Azure Disk / GCE PD）可能落在错误 AZ，导致 Pod 永久无法挂载——这在灾备还原（恰恰是跨集群、拓扑不同）场景里是高发陷阱。架构设计.md 仅在「组件拓扑」意义上用到「拓扑」，未涉及存储可用区，确为真空白。建议：仅在确认目标为单 AZ / hostpath 类拓扑无关存储时使用 Immediate 别名；否则保留 WFC 并在 Preflight 给出拓扑校验与告警，而非默认改写。

### T4（High）默认把集群上下文外发给第三方 LLM，与等保 2.0 / 数据驻留诉求冲突

PRD-003 默认 LLM provider 为 DeepSeek（第三方 SaaS），而其知识库本身就含「等保 2.0 三级 / GDPR 备份保留」等强合规条目。对灾备这类客户，默认把命名空间、镜像、PVC 结构、拓扑等集群上下文外发给境外 / 第三方推理服务，本身是合规反模式——即便 §7 列了脱敏（Secret / env 替换为 `***`），元数据本身仍是敏感的攻击面情报。

核对仓库后这一点更突出：**SECURITY.md 通篇没有任何 AI / LLM / 数据出境 / 脱敏的条目，现有 ADR-001~031 也没有一条覆盖 AI 数据处理**（PRD 自承 ADR-033 / ADR-034 仍是「拟」）。也就是说，一个「默认把集群元数据发往第三方推理服务」的能力，目前没有任何安全文档或架构决策为其背书。这与他们在 ADR-004「凭据只走 Helm / Secret、安全审计走 Git PR」、ADR-030「还原时安全扫描」体现出的强安全自觉形成反差。建议：对合规客户默认使用本地 Ollama（PRD 现规划 v1.1 才加，应提前到 v1 与合规默认绑定），DeepSeek / Claude 等 SaaS 作为显式 opt-in；脱敏清单与「哪些字段会离开集群」必须可审计且默认最小外发；并在 SECURITY.md 增设「AI 数据处理与出境」专章、以 ADR-033 固化。

---

## 三、逐份 PRD 评审

### PRD-001 — 跨集群还原前置检查闭环

方向正确：把 Restore 抽屉做成轻量 checklist、把创建 / 编辑动作委托给 Transform 页，拆分合理，侧抽屉空间问题确实自然消解。

| # | 严重度 | 问题 / 风险 | 建议改法 |
|---|---|---|---|
| 1 | **High** | 「忽略此冲突」对 blocker 级冲突也允许（Q1：允许 + 二次确认）。灾备语境下，忽略一个 blocker = 放行一次带病还原，可能酿成静默数据丢失或还原后不可用。 | 区分语义：blocker 不可忽略（只能去解决，或由 admin 显式降级为 warning 并审计）；仅 warning 可勾选忽略 + 二次确认。 |
| 2 | **Med** | blocker / warning 的分级判定来源不清——后端给出还是前端判断？若前端自行分级，跨版本会漂移。 | severity 在后端 conflict schema 固化为枚举字段，前端只渲染不判断；matchingTransformSets 同理由后端给。 |
| 3 | **Med** | Re-check < 3s 的性能假设未计入跨集群 API 往返 + 目标集群 SC/VSC 枚举。跨云高延迟下易超时。 | 把 3s 限定为同区；跨云给独立 SLO + loading / 超时态；Preflight 结果可短缓存并显示「数据时点」。 |
| 4 | **Info** | 文件 271–511 行保留了大段 v1 已废弃方案注释块，污染「单一权威来源」。 | 物理删除（git history 已永久保存）；PRD 正文只保留 v2 现行内容。 |
| 5 | **Info** | 跳转用新 tab（Q4）保状态，但浏览器拦截 / 多 tab 场景下 router state 易丢。 | 状态以 restoreName 为 key 持久化到后端草稿，回来时按 key 恢复，弱化对 router state 的依赖。 |

### PRD-002 — Transform 一等公民（两层模型）

对齐 Kasten 两层模型是正确方向，§0 概念表清晰，preview-resolution 端点（让客户在确认前看展平后的 effective patch）是很好的「谁配置谁负责」技术抓手。核心隐患见 T1。

| # | 严重度 | 问题 / 风险 | 建议改法 |
|---|---|---|---|
| 1 | **High** | 见 T1（首轮为 Blocker，经 ADR-003/006 核对后修正）：执行底座已是 Velero 原生 resourceModifierRef，但新两层模型 + `${VAR}` 参数在 Velero 无对应物，「编译为单个 data.rules.yaml、len==1 的 ConfigMap」这一步未写明；§4.1 builtin YAML 用 data.spec 与 ADR-003 冲突。 | 修订 ADR-003：写明 TransformSet+params 在 Trigger 时展平编译为单个合规 ConfigMap；统一 data key 为 rules.yaml；preview-resolution 输出即编译结果，顺序对齐 Velero「同 path 后者覆盖」。 |
| 2 | **High** | Phase 0 把已 Shipped 的 4 个 strip-* builtin 从 TransformSet 重塑为 Transform，是对已交付对象的破坏性 schema 迁移。DoD #12 仅靠「label 改名 + seed 3 个 TS 引用回去」保证兼容，依据偏薄。 | 复用现有 ADR-014「启动时迁移机制」（已有 migrateBrokenTransformSets 先例，v0.8.2→0.8.3 迁过 ConfigMap 结构），但本次是语义重塑、风险更高：补独立迁移测试 + 回滚 + 迁移前后对照，纳入「迁移影响评估」门禁。 |
| 3 | **High** | 使用统计写 annotation + 「CAS 单调递增」。K8s 上 CAS 靠 resourceVersion 乐观锁，高并发 Restore 下会触发写冲突重试风暴；annotation 也非为计数 / 查询设计。 | 统计改用事件流或独立 counter（Q1 已预留「超大集群再考虑」——应把阈值 / 重试上限 / 退避写进 NFR）；或接受最终一致并明确不保证精确。 |
| 4 | **Med** | `${VAR}` 字符串模板把 regex 与替换值（如 `acr\.io/(.*)` → `harbor.local/$1`）拼进 JSON Patch，存在转义 / 注入风险且无类型校验（Q6 倾向 `${VAR}`）。 | 保留 `${VAR}` 但对 regex / 捕获组做校验 + 在 preview-resolution 里展示替换 diff；危险字符拒绝；长期演进到 JSON Schema 参数。 |
| 5 | **Med** | TransformSet 内跨资源类型 + 同 path 多 patch 的执行顺序，PRD 声称「按引用顺序」，但未对齐底层引擎实际语义。 | 在 ADR 中明确并写入 DoD：以底层（Velero）实际应用顺序为准，UI preview 必须反映真实顺序。 |

### PRD-003 — AI Advisor（推荐型 · 非自治）

产品判断成熟：拒绝全自治、Phase A/B/C 分段、只读建议 + 人确认，与 PRD-002 责任原则一致。「重要不紧急」评级合理。关键问题是数据外发（T4）与评分的工程可信度。

| # | 严重度 | 问题 / 风险 | 建议改法 |
|---|---|---|---|
| 1 | **High** | 见 T4：默认 DeepSeek 外发集群上下文，与合规客户诉求冲突。 | 合规客户默认本地 Ollama（提前到 v1）；SaaS provider 显式 opt-in；外发字段清单可审计、默认最小化。 |
| 2 | **High** | Resilience Score（0-100 + 5 维度）由 LLM 产出却要进 Dashboard 当「高层视角 / 运维成熟度信号」。LLM 打分天然不可复现（同输入不同分），分数刷新跳动会摧毁信任。 | 分数用规则引擎计算（可复现，PRD 已有规则评分作降级），LLM 只生成「解释与改进建议」；或对分数缓存固化 + UI 明示「非确定性、仅供参考」。 |
| 3 | **Med** | UI 展示「置信度 92%」这类 LLM 自报置信度。自报置信度是已知不可靠指标，伪精确百分比会误导用户。 | 改为高 / 中 / 低三档或移除；信任靠「引用了哪些 KB 条目 + 检测到哪些事实」，而非一个数字。 |
| 4 | **Med** | RAG 知识库 v1 仅 25–30 条却上嵌入索引 + chroma-go，属过度工程；30 条规模向量检索收益有限。 | v1 直接全量条目注入 prompt（或简单关键词召回）更可靠；待条目过百再引入向量库。 |
| 5 | **Med** | LLM 评分与规则评分两套口径并存（降级用规则版）。开关 AI 前后分数跳变，客户困惑。 | 统一两套口径，或 UI 永远标注分数来源（AI / 规则），并保证规则分为基线、AI 仅叠加解释。 |
| 6 | **Info** | list 场景批量打分 + ≤0.01USD/次 + 24h 缓存，仍可能在大集群放大成本 / 并发。 | 加并发上限 + 每日配额 + 批量异步队列；成本估算口径在多 provider / BYO 下别承诺过死。 |

### PRD-004 — MCP Server（Supkube Skills）

安全设计是亮点：PoLP（readonly 模式 + 两套 SA）、Human-in-Loop、输入校验防注入、4KB 输出裁剪、限流、审计齐全。但传输选型与有状态确认机制有硬伤。

| # | 严重度 | 问题 / 风险 | 建议改法 |
|---|---|---|---|
| 1 | **Blocker** | 见 T2：SSE-only 押注已弃用传输，且与 Bearer Token 安全设计冲突。 | v1 改 Streamable HTTP（内部可用 SSE 流式）；旧 SSE 端点仅作兼容。 |
| 2 | **High** | HitL 二次确认用 confirm_id + 5 分钟 TTL，但 MCP 多连接、PRD 自己要「多副本 + LB 达 99.9%」。confirm 的服务端状态存哪？跨副本如何共享？不共享则 LB 后确认会丢。 | 确认快照存共享存储（Redis / CR），confirm_id 全局可寻址；明确 TTL 与一次性消费语义。 |
| 3 | **High** | create_backup_policy 先返回 dry-run YAML 给 LLM、再凭 confirm 落地。若 confirm 时信任 Agent 回传的参数，LLM 可能在两次调用间幻觉 / 篡改，HitL 形同虚设。 | confirm 必须基于服务端按 confirm_id 持久化的原始 dry-run 快照落地，只认 confirm_id + 用户确认信号，忽略第二次调用携带的业务参数。 |
| 4 | **Med** | 开源仓库（Apache-2.0）复用 PRD-003 的闭源 audit 模块，边界未划清。无 backend 时 audit 缺失，或被迫开源 audit 逻辑。 | 定义 audit 接口（开源）+ 可插拔实现；缺省提供本地文件 audit，企业版接 backend 模块。 |
| 5 | **Med** | DoD #4 用 OpenClaw 做验收，但 PRD 自认 OpenClaw 是客户侧第三方、文档待客户提供（Q8）。验收强绑不可控第三方有风险。 | 验收基线改为 MCP Inspector + Claude Desktop（协议 conformance）；OpenClaw 列为 nice-to-have / alpha。 |
| 6 | **Med** | 多集群一个 Server（Q3）+ Skill 带 cluster_id，但跨集群 RBAC / 租户隔离如何在 MCP 层映射、token 与 cluster 授权边界未定义；限流 60/min 是全局还是 per-cluster 也未明。 | 明确 token→可访问 cluster 集合的授权模型；限流细化到 per-token-per-cluster；审计隔离落到 cluster 维度。 |

---

## 四、跨 PRD 一致性与依赖链风险

- 强串行依赖链：PRD-001 依赖 002；003 依赖 001/002；004 依赖 003。PRD-002 的「TransformSet→Velero ConfigMap 编译契约」（T1）是上层 Restore / Advisor / MCP 都要踩的地基，应在链条最前面书面定稿（修订 ADR-003）。
- 共享 Advisor Engine 同时服务内部 UI（PRD-003）与外部 MCP（PRD-004）是好架构，但认证 / 脱敏 / 限流 / 审计口径必须统一收敛在 Engine 层，避免两条出口各做一套、规则漂移。建议把「脱敏策略 + 外发白名单 + 审计 schema」作为 Engine 的一等接口。
- ADR 缺口：ADR-031 已存在，但 ADR-033（AI Advisor 架构）/ ADR-034（MCP + 开源策略）均为「拟」。AI 与对外协议属架构级决策，应先有 ADR 再大规模研发；建议把「对应 ADR 立项」设为 PRD-003/004 进入研发中的前置门禁。
- 状态机一致性：索引把 PRD-002 标为「已评审 / 可进研发」，但其 Phase 0 是对 Shipped 对象的破坏性迁移。按 PRD 自身铁律（Shipped 永久保留 / 研发期不可静默改），这类迁移应额外过一道「迁移影响评估」，不宜直接 kick off。
- 术语 / 定位已澄清得很好（OpenClaw = 客户侧第三方 Agent 框架，Skills 必须开源），一致性强。仅提醒对外措辞统一为「compatible with any MCP client」，不要在 example / DoD 里反向把 OpenClaw 当成必选依赖。

---

## 五、建议的行动优先级

| 序 | 行动项 | 归属 | 时机 |
|---|---|---|---|
| 1 | 修订 ADR-003：写明新两层模型 + params 如何编译为单个 Velero resourceModifierRef ConfigMap（T1） | PRD-002 | 研发前 / 先 |
| 2 | PRD-004 传输改 Streamable HTTP；HitL 改服务端快照 + 多副本共享（T2，绕开 ADR-015 已记录的 SSE 坑） | PRD-004 | 研发前 |
| 3 | blocker 不可忽略 + severity 后端固化 + 删 v1 残留 | PRD-001 | 研发前（小改） |
| 4 | 合规默认本地 LLM、最小外发 + Resilience Score 规则化（T4） | PRD-003 | 研发前定方案 |
| 5 | Phase 0 破坏性迁移补测试 + 回滚 + 迁移影响评估门禁 | PRD-002 | Phase 0 前 |
| 6 | 统计机制（annotation→事件 / counter）与限流 / 退避写入 NFR | PRD-002/004 | 研发期内 |
| 7 | ADR-033 / ADR-034 立项作为研发前置门禁 | PRD-003/004 | 评审收尾 |

*总体评价：这是一套高于行业平均水平的 PRD——产品取舍清醒、责任边界自洽、文档纪律强。把上述 Blocker / High 项（尤其 T1 编译契约与 T2 MCP 传输）在进入研发前收口，四份 PRD 即具备扎实的落地条件。*

---

## 六、与现有 ADR / 仓库文档的对照（第二轮补充）

本轮通读了 supkube 仓库的 架构设计.md（ADR-001~031）、SECURITY.md、测试用例.md、CSI 文档等，用以验证首轮结论。

### 已有 ADR 覆盖、可直接复用（修正 / 支撑了首轮判断）

- **ADR-003**：Transform Set = Velero 原生 `resourceModifierRef` ConfigMap，且 data 仅一个 `rules.yaml`（`len(Data)==1`，v0.8.2 曾因多 key 被 Velero 拒）。→ 修正 T1：执行底座既定，缺的是「新两层模型如何编译落地」的书面契约。
- **ADR-006**：三种 Velero patch 类型（Strategic / Merge / JSON Patch）已按冲突场景定好，并记录「JSON Patch remove 不存在字段必失败、默认 Merge 更安全」。→ PRD-002 的 builtin Transform 应沿用该选型矩阵，不要在 PRD 里另立 patch 语义。
- **ADR-014**：已有「启动时迁移机制」（`migrateBrokenTransformSets` + 重新 seed）。→ PRD-002 Phase 0 破坏性迁移应复用此机制，但需补语义重塑级别的测试与回滚。
- **ADR-015**：明确「不用 WebSocket / SSE，因 K8s nginx ingress 兼容性问题」。→ 直接坐实 T2：PRD-004 选 SSE-only 与团队既有经验自相矛盾。
- **ADR-031**：5 层 3-2-1-1-0 数据韧性模型，定位已是「Kasten / Veeam 级平台」。→ Transform / Advisor / MCP 都应服务这一定位；Advisor 的「DR Drill / Verified Restore」建议正好对应 Layer 5 虚拟实验室，可在 PRD-003 里显式挂钩。

### 文档与 ADR 的空白（需补）

- SECURITY.md 零 AI / LLM / 数据出境 / 脱敏内容；ADR-001~031 无一覆盖 AI 数据处理（ADR-033/034 仍「拟」）。→ T4 的合规风险目前没有任何安全文档或架构决策背书，应先补 SECURITY.md 专章 + ADR-033 再大规模研发 PRD-003/004。
- 架构设计.md 仅在「组件拓扑」意义上用到「拓扑」，未涉及存储可用区 / VolumeBindingMode 拓扑保证。→ T3（SC 改 Immediate 的多 AZ 风险）确为真空白，PRD-001/002 需自行补拓扑校验，不能假定架构层已处理。

---

## 附录：技术核实来源

**外部来源**

- MCP 传输：HTTP+SSE 于规范 2025-03-26 弃用，由 Streamable HTTP 取代（modelcontextprotocol.io specification / transports）。
- MCP SSE 弃用原因（双端点、LB 兼容差、token 入 URL 安全隐患）：blog.fka.dev、auth0.com MCP Streamable HTTP 文章。
- Velero Restore Resource Modifiers（v1.12+，JSON Patch / Merge / Strategic Merge，ConfigMap + `--resource-modifier-configmap`，按序应用、同 path 后者覆盖）：velero.io/docs restore-resource-modifiers。
- CSI VolumeBindingMode WaitForFirstConsumer 的拓扑保证为通用 K8s 存储调度知识。

**仓库内部来源（架构设计.md）**

- ADR-003 Transform Set 用 ConfigMap（复用 resourceModifierRef、`len(Data)==1`）；ADR-006 三种 Velero patch 分场景；ADR-014 启动时迁移机制；ADR-015 不用 SSE（nginx ingress 兼容性）；ADR-031 5 层数据韧性模型；SECURITY.md（无 AI 条目）。

---

<!--
评审元数据（便于长期追踪，勿删）
- 评审轮次：第二轮（含 ADR 交叉核对）
- 首轮→二轮变更：T1 由 Blocker 降为 High（ADR-003/006 已决策 Velero 复用）；T2 增 ADR-015 佐证；T4 增 SECURITY.md / ADR 空白佐证；迁移项关联 ADR-014。
- 下次评审请在 PRD-Review/ 目录新建 PRD-Review-YYYY-MM-DD.md
-->
