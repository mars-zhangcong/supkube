# SupKube ADR 评审报告 — ADR-033~036

> **视角**: Kubernetes + AI + 灾备（DR）专家
> **评审对象**: ADR-033 AI Advisor 架构 / ADR-034 MCP 协议选型 / ADR-035 结构化日志规范 + 错误代码体系 / ADR-036 SSE 项目级口径
> **评审人**: Claude（受 Mars 委托）· **日期**: 2026-05-31
> **核对基线**: 架构设计.md（ADR-001~031 + 新增 ADR-032~036 + ADR 号台账）· SECURITY.md（含新增 §6 AI 数据处理与出境治理）· PRD.md（含 PRD-001~006 修订后）· MCP 2025-03-26 spec
> **承接**: 本次为 PRD-Review 系列第三份；前两份覆盖 PRD-001~006（PRD-Review-2026-05-31.md / PRD-Review-2026-05-31-PRD005-006.md），8 条 finding（T1~T4 + X1~X4）中 4 条要求新 ADR 背书，本次评审即为该前置门禁

---

## 一、执行摘要

四份新 ADR 整体质量明显高于"草稿"应有的成熟度，可以看出是先在 PRD-Review 8 条 finding 上"知道答案"后才回头写决策记录，而不是 ADR-then-PRD 的纸面顺序。这是好的工程惯性：决策书面化滞后于现实问题，比反过来"先憧憬再写代码"安全得多。

四份各自的成熟度有差：

- **ADR-033（AI Advisor）** 最扎实，6 条 Decision 与 SECURITY.md §6.A~G 几乎逐条对应，Resilience Score / LLM 分离（Decision #6）与置信度三档（Decision #5）正面回应了 PRD-Review T4 + X4。脱敏管线"单一来源"是其承重梁，但有几处 owner 与 fingerprint 计算口径与 SECURITY.md §6.C/§6.E 微 drift，需在 Accepted 前对齐。
- **ADR-034（MCP）** 选型方向无争议，与 MCP 规范 2025-03-26 主流路径对齐；但"SSE 仅向后兼容"的 sunset 时间线（`Sunset: 2027-01-01`）与 PRD-004 §6"v1 仅 Streamable HTTP 新功能"之间缺一份**deprecation runbook**，否则到时无人记得拉闸；stdio v1.1（Decision #4）排进 roadmap 但未给版本号锚点。
- **ADR-035（结构化日志 + 错误代码）** 是四份里**前置依赖最广、未决项最多**的一份：zerolog vs zap 选型理由仅"略快 + 简单"，没有 benchmark / 团队历史 evidence；首批 20 个 ERR_* 列出来了，但没有 owner / 评审节奏；6 个月兼容期与 v0.10.x roadmap 时间线匹配性未交叉验证；ADR-019 stdout 双写如何升级到 zerolog 没有写明（这是落地的卡口）。
- **ADR-036（SSE 项目级口径）** 决策合理（区分 Live Tail / MCP / 黑名单三档），且明确 supersede ADR-015 是"refine"而非"否定"——这一点措辞干净；但 DoD 三项实测（TC-SSE-001/002/003）虽列出，**没有写明 CI 跑在什么环境**（kind cluster？真实云？），存在"DoD 设了但跑不动"的 implementation risk；与 ADR-016（ConfigMap-mounted nginx）的耦合点也未阐明。

与 ADR-001~031 既有决策的关系总体一致：

- ADR-003 / ADR-006 / ADR-014（PRD-002 链）与本次 4 份 ADR 无直接冲突。
- ADR-015（不用 SSE）被 ADR-036 显式"refine"，与 ADR-034（MCP 不用 SSE）形成完整光谱，**没有出现"互相打脸"的局面**，这是本次评审的关键正向结论。
- ADR-019（K8s Events + stdout 双写）与 ADR-035 的关系**未在 ADR-035 正文阐明**，需补一句"stdout 这一路升级为 zerolog 结构化 JSON，K8s Events 那一路保留 events/v1 结构不变"才能避免开发期歧义。
- ADR-020（Bearer Token 表）被 ADR-034 复用合理，但 ADR-034 新增 `tokenType: mcp` scope 字段，应回写到 ADR-020 的"后续要做"区段，否则 ADR-020 单看的人不知道 token 表已扩展。

### 放行结论速览

| ADR | 决议 | 前置条件 |
|---|---|---|
| **ADR-033** AI Advisor 架构 | **Accepted with conditions** | (1) Decision #4 脱敏管线与 SECURITY.md §6.C `Sanitize(ctx, payload) (sanitized, fingerprint, err)` 签名对齐（当前 ADR 写 `(Payload, SanitizeReport, error)` 三返回，不一致）；(2) Decision #5 置信度三档定义需在 PRD-003 §5 UI 接口同步落地（finding #2 闭环）；(3) "Prompt 模板版本化" 后续项排期，否则一年后无人审计 prompt 变更 |
| **ADR-034** MCP 协议选型 | **Accepted** | (1) SSE 端点 `Sunset: 2027-01-01` 需在 USER_MANUAL §MCP + CHANGELOG 同步公告，并在 ADR-034 "后续要做" 加一个 ERR_MCP_TRANSPORT_DEPRECATED 触发条件（已在 ADR-035 §5 #15 占位，但 ADR-034 自己未引用）；(2) stdio v1.1 排进 v0.10 vs v1.0 哪个版本需明示，否则"v1.1"在路线图上是悬挂引用 |
| **ADR-035** 结构化日志 + 错误代码 | **Needs Revision** | (1) zerolog vs zap 补 benchmark 引用 + license（zerolog MIT，OK）+ 团队历史决策证据；(2) 首批 20 个 ERR_* 列每条 **owner + 触发 KB 文档评审的节奏**；(3) ADR-019 升级路径写一段（stdout 双写如何过渡到 zerolog）；(4) 6 个月兼容期与 v0.10.x roadmap 交叉验证（v0.10.x 是否真给得起 1.5-2 周改造窗口）；(5) `supkube_log_legacy_total` 启动告警阈值与告警去向（Prometheus 还是 K8s Event）未定 |
| **ADR-036** SSE 项目级口径 | **Accepted with conditions** | (1) DoD TC-SSE-001/002/003 写明 CI 跑在 kind + nginx-ingress-controller 默认 chart；(2) 与 ADR-016 ConfigMap-mounted nginx 的耦合点写一段（4 个 annotation 是 chart 默认 ingress 的 annotation，还是 ADR-016 SPA nginx 的 location 块，这两个不是同一个 nginx，目前 ADR-036 措辞混着）；(3) "降级路径 5s 轮询" 在 PRD-005 v1 是否真已实现，需 verify before architect（座右铭 §11.2 / feedback_verify_before_architect 记忆）|

**严重度图例**（与前两份 PRD-Review 一致）

- **Blocker** — 地基性问题，应在该 ADR Accepted 前解决
- **High** — 影响正确性 / 合规 / 数据安全 / 跨 ADR 一致性，Accepted 前需有明确方案
- **Med** — 影响可维护性 / 体验 / 工程成本，研发期内处理
- **Info** — 文档卫生、措辞统一、建议性优化

**finding 总数**：0 Blocker / 9 High / 11 Med / 6 Info（详见 §二 各 ADR 表格）

---

## 二、逐份 ADR 评审

### § ADR-033 — AI Advisor 架构（Engine + Provider 抽象 + 脱敏管线 + 出境治理）

**评审范围**：架构设计.md 行 2410–2569

**亮点**（罕见）：

- Decision #1 单一 Engine + 两个出口 + `forbidden-import` lint 规则——把"PRD-005 / PRD-006 自己调 LLM SDK"这种规则漂移在编译期堵掉，是真正的工程止血而非治理 PPT。
- Decision #4 脱敏管线"单一来源 + 单点风险 + 高强度回归测试套件 TC-AI-SAN-*" 这种**先承认是 single point of failure 再给出缓解**的写法，比"我们已经测过没问题"诚实 N 倍。
- Decision #6 Resilience Score 由规则引擎计算、LLM 仅做解释——直接回应 PRD-Review T4 finding #2（Resilience Score LLM 打分不可复现），且 UI 示例画了"Rules Engine, ✓可复现 vs AI 生成 ⚠"双标签，落到产品语义一级。
- Alternatives Considered 真有 reject 理由（不是"摆样子"）：全自治被 (a)(b)(c) 三段论 reject、LiteLLM 网关被"多依赖 + 脱敏需求网关不管"reject——这两条特别值得点出。

| # | Severity | 问题 | 建议改法 |
|---|---|---|---|
| 1 | **High** | Decision #4 `Sanitize(input Payload) (Payload, SanitizeReport, error)` 函数签名与 SECURITY.md §6.C 行 138 `Sanitize(ctx, payload) (sanitized, fingerprint, err)` 不一致——前者返回 SanitizeReport，后者返回 SHA-256 fingerprint。两份文档是 PRD-Review T4/X4 的"单一来源"约束，签名 drift 会导致 PRD-003 实施时不知道以哪份为准。 | 二选一并 supersede 另一份。建议保留 ADR-033 的 `SanitizeReport`（带前后字段计数，对客户合规审计更有用），SECURITY.md §6.C/§6.E 的 `input_fingerprint` 字段则是 SanitizeReport 的 SHA-256 计算结果（写明"fingerprint = sha256(SanitizeReport.payload)"）。 |
| 2 | **High** | Decision #5 置信度三档（high/medium/low）只定义了 LLM prompt 的语义，**没有定义** UI 怎么从 LLM 返回里解析。LLM 不遵守 prompt 输出 `confidence: 87%` 时怎么办？fallback 到 medium？拒绝返回？这是 PRD-Review T4 finding #3 的实施细节，ADR 漏了。 | 加一段 Decision #5bis："LLM 返回若不含 `confidence: {high\|medium\|low}` 字段或值非枚举，Engine 强制 fallback 为 `medium` 并打 audit warning `EVT_AI_CONFIDENCE_PARSE_FALLBACK`（同时计入 PRD-005 §4.5 KB stub）"。 |
| 3 | **High** | Decision #2 Provider 抽象 v1 实现"Ollama / DeepSeek / Claude / Azure OpenAI"四个 provider，但 SECURITY.md §6.B 行 112 trusted-provider 白名单写的是"Ollama / DeepSeek / Claude / GPT-4 系列 / 客户自有 OpenAI-compatible endpoint"——"Azure OpenAI" vs "GPT-4 系列" + "BYO endpoint" 不对仗。新增 provider 必须走 SECURITY.md §6.G 的"§2 LLM Provider 风险评审"，ADR-033 没写这条门禁。 | ADR-033 Decision #2 增加一行："新增 provider 必须经 SECURITY.md §6.G 评审通过加入 §6.B 白名单后才能在 UI 下拉出现"；并把 v1 provider 列表与 SECURITY.md §6.B 对齐（Azure OpenAI 应归入"OpenAI-compatible endpoint"还是单独白名单条目，需澄清）。 |
| 4 | **Med** | Decision #3 "启动时若 default 是 SaaS provider 且 `ai.outbound.acknowledged != true`，**backend 拒绝启动**"——这条与 SECURITY.md §6.F 行 218"AI 功能整体默认关闭"冲突：如果默认关闭，启动时根本不需要 outbound.acknowledged 检查。两份文档对"默认状态"理解不一致。 | 澄清：default = 哪个 provider 是配置层概念，AI 功能是否打开是运行时概念。两者正交。ADR-033 应改为"若 AI 功能 enabled 且 default 是 SaaS provider 且 outbound.acknowledged != true，backend 拒绝启动"；与 SECURITY.md §6.F master switch 关系写清楚。 |
| 5 | **Med** | Consequences 负面"早期 Ollama（本地 7B-13B）性能 < SaaS 大模型"承认了体验降级，但缓解方案仅"在 USER_MANUAL 明示星级"。这对合规客户来说是"被告知了不好用"，不是"我们在让它变好用"。 | 加一条"后续要做"：v1.x 内置 Ollama model selector + 推荐模型（如 `qwen2.5:14b-instruct` 中文 / `llama3.1:8b` 英文），并对常见 SRE 任务在内部 eval set 测准确率，发布"SupKube 推荐 Ollama 配置矩阵"。否则"Ollama 默认"的承诺等于把性能问题甩给客户。 |
| 6 | **Med** | Decision #2 "支持热切换"（reload 不重启 pod，但当前请求按旧 provider 跑完）。这与 ADR-020 token 表"GitOps 改 values → helm upgrade → pod restart"的运维模式风格不一致——ADR-020 故意不做热加载，ADR-033 故意做热加载，团队心智模型分裂。 | 要么 ADR-033 改为"reload 也走 helm upgrade，pod 重启即可"（与 ADR-020 一致），要么在 Consequences 写明"AI provider 是运行时配置非基础设施配置，故走热加载；与 ADR-020 / Helm-only 的区别在于 ..."。 |
| 7 | **Info** | "后续要做"段提到 "Prompt 模板版本化：`internal/advisor/prompt/v1/`，prompt 改版要走 PR review"——但没有定义 prompt diff 的评审标准（accuracy / safety / cost 三轴中至少要测哪个）。一年后 prompt 改了 50 次，无人能回溯哪次改变了行为。 | 加一行："prompt 版本变更必须在 PR 描述里附 internal eval set 对比（accuracy + safety regression），并对比 token 用量影响"。或承认 v1 不做、v1.x 补，但要在 ADR 显式排期。 |
| 8 | **Info** | References 段引用 `https://www.anthropic.com/research`（LLM 自报置信度不可靠的实证）——这是个站点首页，不是具体论文。一年后链接漂移很常见。 | 引用更具体的论文（如 OpenAI "Calibration" 论文 / Anthropic "Constitutional AI"），或留一句"参考 Anthropic 与 OpenAI 研究博客中关于 LLM self-reported confidence calibration 的多篇文章"。 |

**评审结论：Accepted with conditions**（finding #1–#3 必修；#4–#6 强烈建议研发期前修；#7–#8 文档卫生）

---

### § ADR-034 — MCP 协议选型（Streamable HTTP，不用 SSE）

**评审范围**：架构设计.md 行 2573–2757

**亮点**：

- Context 段画出 MCP 协议时间线（2024-11 → 2025-03-26 → 2025-Q2 → 2026-05），直接坐实"PRD-004 初稿 SSE only 是踩弃用路径"，是 ADR 写法范本——**事实驱动，不靠权威**。
- Decision #3 Bearer Token 走 Authorization header 而非 URL query string，与 ADR-020 token 表设计逻辑自洽；并明示"MCP token 是独立 type，不能复用 UI 浏览器登录 JWT"——防止长期 JWT 泄漏给桌面 client，这是细节里的安全 sense。
- Decision #6 给了完整 `curl` + 响应 JSON 示例，比 PRD-004 §4 的端点描述更可执行。
- Alternatives Considered 把 WebSocket、gRPC、stdio-only 都 reject 了，理由都贴在"MCP 规范不定义"+"K8s ingress 配置复杂"两条具体事实上。

| # | Severity | 问题 | 建议改法 |
|---|---|---|---|
| 1 | **High** | Decision #2 SSE 端点保留 + `Sunset: 2027-01-01`（RFC 8594）。问题：**8 个月后**谁负责"v2.0 评估移除 SSE"这个决策？ADR 写"取决于客户接入 client 版本统计"，但没有数据采集机制——MCP token 表（ADR-020 扩展）需要新增字段记录 client transport（streamable / sse），否则到 2027-01-01 没人有数据。 | Decision #2 加一句："MCP 调用审计日志（走 ADR-035 schema）必须记录 `transport: streamable\|sse` + `protocolVersion: 2025-03-26\|2024-11-05` 字段，便于 2027-Q1 评估移除决策"；并把"2027-Q1 移除 SSE 端点评审"作为 explicit task 排进未来 PRD。 |
| 2 | **High** | Decision #4 stdio v1.1（Claude Desktop 桌面用）是放进 roadmap 的，但 ADR 没说"v1.1"对应哪个 SupKube 版本号（v0.10？v0.11？v1.0？）。架构设计.md §12 已规划版本表（v0.8.5 / v0.8.6 / v0.9.0 / v0.10.0），ADR-034 v1.1 在哪？这是悬挂引用。 | Decision #4 加版本锚："stdio 模式 = SupKube v0.10.x（与 ADR-031 Operator 化 + Layer 4 Backup Copy 同期）"或 v0.11——具体哪个由 Mars 拍板，但必须给数字，否则 v1.x 永远在"未来"。 |
| 3 | **High** | Decision #7 RBAC 与错误处理"MCP token 在 ADR-018 RBAC 3 角色模型中默认为 `viewer`"——但 ADR-018 没区分"浏览器登录用户"和"MCP token"。如果同一个 OIDC 用户 Alice 在浏览器是 admin，她为 Cursor 创建了一个 MCP token，这个 token 默认 viewer 还是继承 Alice 的 admin？ADR-018 + ADR-020 + ADR-034 三方关系未定义清楚。 | 加一句："MCP token 的 role 在 token 创建时**显式**指定（不继承创建者的 role），默认 viewer。token 表新增 `tokenType: mcp` + `role: viewer\|editor\|admin` 两字段"；并在 ADR-020 "后续要做" 同步登记此扩展（finding #5 同问题）。 |
| 4 | **Med** | Decision #5 "与 supkube-backend 同一二进制（启动参数 `--mcp-server`）"——但 ADR-032（Operator 评估）已 Decided 要拆分 supkube-operator 二进制。MCP Server 跟 backend 同进程 vs 跟 operator 同进程 vs 独立第三个二进制，三选一未与 ADR-032 对齐。 | 评审前先与 ADR-032 owner 对齐（应该都是 Mars）：MCP Server 跟谁同进程？建议明确"v1: 与 backend 同进程；v0.10.x Operator 化后重评"。否则 Operator 化时会面临"MCP Server 跟着 backend 还是搬去 operator"的二次决策。 |
| 5 | **Med** | Decision #3 "MCP token 是独立 token type" 但 ADR-020 token 表当前不区分 type。这是 ADR-020 的事实性扩展，ADR-020 自己不知道。 | ADR-034 Consequences "后续要做" 段已写"在 ADR-020 token 表加 `tokenType: mcp` 字段"——很好；但还需要在 ADR-020 正文加一段"修订（2026-05-31，v0.10.x）— MCP token type（ADR-034 触发）"，否则将来读 ADR-020 单文不知道 token 表已分 type。 |
| 6 | **Med** | Decision #8 协议版本协商"不支持版本协商的旧客户端 → 默认走 2024-11-05 + SSE 兼容端点 + 打 audit warning"——audit warning 的 code 是什么？ADR-035 §5 #15 占位的 `ERR_MCP_TRANSPORT_DEPRECATED` 似乎就是为此设计，但 ADR-034 没引用。 | 显式引用："旧客户端连接打 audit `EVT_MCP_LEGACY_CLIENT` + 第一次工具调用返回 `data.code: ERR_MCP_TRANSPORT_DEPRECATED`（ADR-035 §5 #15），让客户端开发者在响应里看到 deprecation 提示"。 |
| 7 | **Info** | Decision #6 流式响应示例展示了 `event: progress` / `event: result`，但没说明 client 如何识别"这个连接是 chunk-by-chunk 流式还是单次 JSON"——是看 `Accept` 协商，还是看响应 `Content-Type`，还是看第一个 chunk？MCP 规范定义了，但 ADR 没复述，新人会困惑。 | 加一句："响应 `Content-Type: text/event-stream` 表示流式，client 按 SSE 解析；`Content-Type: application/json` 表示单次响应。判断由响应头驱动，与请求 Accept 协商无关。" |
| 8 | **Info** | "MCP Inspector 集成测试" 写在"后续要做"，但 PRD-004 DoD #2 已经把它作为 v1 验收标准。两边责任不分配清楚的话，PRD-004 DoD 写了 ADR-034 不背书。 | "后续要做"改为"已在 PRD-004 DoD #2 验收基线落地"，避免责任跨文档失锚。 |

**评审结论：Accepted**（finding #1–#3 必修但都可在 1-2 行 ADR 修订完成；#4–#6 强烈建议研发期前修；#7–#8 文档卫生）

---

### § ADR-035 — 结构化日志规范 + 错误代码体系

**评审范围**：架构设计.md 行 2761–2950

**亮点**：

- Context 段把"PRD-005 v1 启发式正则误判 → 三层切换做不出来 → AI Advisor 根因不稳"三条因果链写得很清楚，证明这份 ADR 是 PRD-005 v2.2 的**前置必修**而非可选优化。
- Decision #4 错误代码命名规范"4 段封顶，超出说明该拆 component"是有节制的工程纪律。
- Decision #5 首批 20 个 ERR_* 列出来了——比"待补"的 ADR 强 N 倍。
- Decision #6 旧 logger 兼容期 ≥ 6 个月 + `legacy.Wrap()` wrapper + 监控指标 `supkube_log_legacy_total` 归零，这是渐进迁移的完整三件套。

但这份 ADR 同时是四份里 **未决项最多的一份**，几乎每条 Decision 都缺一个落地细节。

| # | Severity | 问题 | 建议改法 |
|---|---|---|---|
| 1 | **High** | Decision #1 zerolog vs zap 选型理由仅"JSON-first、零分配、benchmark 比 zap 略快、API 比 zap 简单"——但**没有引用 benchmark 数据**（zap 官方 README 的 benchmark 显示 zap SugaredLogger 在某些场景反而比 zerolog 快），也没有团队历史决策证据（如果团队某成员用 zap 写过另一个项目，迁移成本应该计入）。"选型"在 ADR 里需要 evidence，不是个人偏好。 | 引用 [zerolog README benchmark 段](https://github.com/rs/zerolog#benchmarks) 与 [zap README performance 段](https://github.com/uber-go/zap#performance) 的具体数据点（如 "5 fields disabled-level: zerolog X ns/op vs zap Y ns/op"），并明确两者性能差在 SupKube 量级（< 1000 log/sec）**实际上无意义**——选 zerolog 真实理由是"API 简单"而非"快"。这样 ADR 诚实度高，未来想换 zap 也不会被"zerolog 更快"的伪共识挡住。补充 zerolog license（MIT）+ 团队当前依赖（grep 仓库 `go.mod` 看是否已引）作为辅助证据。 |
| 2 | **High** | Decision #5 首批 20 个 ERR_* 列出来了，但**每条没有 owner 和 KB 文档评审节奏**。20 个 KB stub 谁写？写完谁审？KB 内容（现象 / 根因 / 解法）的最低质量门槛是什么？目前是"USER_MANUAL 持续补充"——这是典型的"无主则烂"路径。 | Decision #5 表格加 "Owner" 列（按 component 归属：BACKUP/RESTORE 归 backend lead，NODE 归 agent lead，AI 归 advisor lead，等），并在"后续要做"加一行："首批 20 个 ERR_* KB stub 必须在 v0.10.x 发版前完成，每条最少含 现象 / 根因 / 解法 / 引用 PRD"。否则 v0.10.x 发布时 KB 都是 stub 占位页，客户点 chip 跳进去看到"该错误码尚在补充中"会失望。 |
| 3 | **High** | Decision #6 "兼容期 ≥ 6 个月" + "PRD-005 v2.2 实施时把 backend 的 BSL / Backup / Restore handler 迁掉作为第一波" + "工期估 1.5-2 周"——这三个数字之间没有交叉验证。架构设计.md §12.1 v0.10.0 主题是"平台化：Kanister Blueprints + Hub-Spoke 多集群"，PRD-005 v2.2 在哪个 sprint？6 个月是从 ADR Accepted 之日算还是从 v2.2 实施之日算？v0.10.x 是否真给得起 1.5-2 周改造窗口？ | 在 Consequences 加一段"时间线对齐"：v0.10.x 哪个 minor 版本承载 PRD-005 v2.2 + ADR-035 backend 迁移（推测 v0.10.1 或 v0.10.2）；6 个月 = 从 PRD-005 v2.2 ship 之日算（如 v0.10.1 ship 在 2026-08，则 legacy wrapper 至少保留到 2027-02）；CI 监控 `supkube_log_legacy_total` 阈值（如 > 100 行/小时 触发告警）。 |
| 4 | **High** | Decision #7 / 后续要做 "ADR-019 审计日志升级到 zerolog（stdout 那一路），保留 K8s Events 一路不变"——但 ADR-019 自己的"实现亮点"段提到了 `internal/auth/audit.go` 中的 middleware + sanitizeLabel + InvolvedObject 命名规则。这些细节怎么跟 zerolog 字段映射？ADR-019 `[audit]` 前缀的固定字符串是否被 zerolog `component=audit` 字段取代？没写。 | Decision #7 加一小段："ADR-019 `[audit]` 前缀字符串迁移到 zerolog 字段 `component: audit` + `audit: true`（zerolog `Str()` 字段）；客户的 stdout → SIEM 管道兼容期内同时接受两种格式，6 个月后 sunset `[audit]` 前缀解析"。同时回写 ADR-019 一句"修订（2026-05-31，v0.10.x）— stdout 一路升级到 zerolog（ADR-035）"，避免单读 ADR-019 不知道格式变了。 |
| 5 | **Med** | Decision #4 命名规范"COMPONENT ∈ {BACKUP, RESTORE, SCHEDULE, POLICY, BSL, VSL, AUTH, RBAC, MCP, AI, NODE, OPERATOR, K8S}"——固定 13 项白名单。问题：未来加新 COMPONENT 走什么流程？比如 v0.10.x 加了 BACKUP_COPY（Layer 4，ADR-031），算 BACKUP 还是新 COMPONENT？ADR 没写扩展机制。 | 加一段"COMPONENT 扩展流程"："新增 COMPONENT 需在 ADR-LEDGER 登记 + ADR-035 修订段补充原因；命名应优先复用现有 COMPONENT（如 BACKUP_COPY 归 BACKUP），仅当跨 component 边界明显时才新增"。 |
| 6 | **Med** | Decision #5 首批 20 个 code 跨 backend / agent / operator 三套二进制。但同一个 `ERR_BACKUP_BSL_AUTH` 可能在 backend（用户触发 backup 时 BSL 凭据失败）和 node-agent（dataMover 上传 chunk 时 BSL 凭据失败）**都能发生**，语义略不同。命名规范没有处理"同 code 跨 component"的冲突。 | 要么 code 加 component 前缀（`BACKEND_ERR_BACKUP_BSL_AUTH` vs `AGENT_ERR_BACKUP_BSL_AUTH`）——会导致 KB 跳转 URL 翻倍；要么允许同 code 跨 component，用日志的 `component` 字段区分（推荐）。ADR 应在 Decision #4 显式选其一并说明理由。 |
| 7 | **Med** | Decision #2 schema "字段约束" 段未提**字段集合上限**——`ctx` 允许任意 key-value，是否对每条日志总字节数 / 字段数设上限？否则开发者会把整个 K8s object dump 进 ctx，日志爆炸。 | 加一行约束："单条日志 size 上限 4 KB；超出截断 `ctx` 字段并打 `truncated: true` flag"。与 PRD-004 §6 4KB 输出裁剪一致。 |
| 8 | **Med** | Decision #6 "新 module 强制新 logger：CI lint `forbidden-import: "log".Printf in new packages`"——但 `"log"` 是 Go 标准库，禁用整个 import 会导致 main.go 启动消息也不能用。lint 规则需要更精确。 | 改为 "禁用 `log.Printf` / `log.Println` 调用，允许 import（main.go 启动 fatal 仍可用 `log.Fatal`）"；或在 lint 配置里 whitelist `cmd/*/main.go`。 |
| 9 | **Med** | "首批 20 个 ERR_* KB 内容由 USER_MANUAL 持续补充；初版可以是 stub" + "KB URL 是 production hostname（supkube.io），需要站点提前预留路由"——supkube.io 站点是否已存在？谁维护？404 stub 页是占位还是有 fallback 内容？ | "后续要做"加一行："supkube.io/kb/ 路由由 docs repo 维护，stub 模板由 PRD-005 v2.2 团队首批提供，离线客户用 helm values `kb.baseUrl` 指向内网（如 `https://internal.example.com/kb/`）"。 |
| 10 | **Info** | Decision #3 三层 level 映射表把 "Detail" 包含 "info（非 milestone）+ debug"，"Debug" 又包含 "全开（含 trace）"——但 zerolog 默认不开 trace level，需要构建时启用。这是部署细节，ADR 没提。 | 加一行 "zerolog 构建 tag `release` 模式默认不编译 trace 级（避免生产开 Debug 档泄漏），Debug 档展示的 trace 行仅在 dev 构建可见"。 |
| 11 | **Info** | "首批 20 个 code" 表格 #19 `ERR_OPERATOR_FRANKENSTEIN`——这个名字诚实但有人会觉得 informal。ADR-031 / #102 用 "frankenstein" 是有传承的，但 KB 公开页用这个词面向客户是否合适？ | 保留也行（产品自嘲文化），但 KB URL `/kb/ERR_OPERATOR_FRANKENSTEIN` 公开页 title 建议用 "Velero version drift detection" 副标题，避免客户搜索时一脸懵。 |

**评审结论：Needs Revision**（finding #1–#4 必修；#5–#9 强烈建议研发期前修；#10–#11 文档卫生）

> Mars 注意：这份 ADR 没"过"，但**没有 blocker**。修订工作量预计 2-3 小时，可在 PRD-005 v2.2 开发之前 1 个 sprint 内闭环。建议本 ADR 与 ADR-019 修订段一起做。

---

### § ADR-036 — SSE / 长连接 / 流式传输项目级口径

**评审范围**：架构设计.md 行 2954–3121

**亮点**：

- "本 ADR 给出权威决策树"——Decision #1 白名单（4 条 AND）+ Decision #2 黑名单（4 条 OR）的对称写法很干净，未来新 PRD 引用 SSE 时只需对照这两表，不会再发生 PRD-Review X3"每份 PRD 自己重新决定"的局面。
- Decision #5 自动降级到 polling + 前端 UI 角标"实时 (SSE)" vs "5s 轮询"——产品语义层的诚实度高，跟 ADR-033 Resilience Score "Rules Engine ✓可复现 vs AI 生成 ⚠"是一脉相承的设计哲学。
- Decision #6 "WebSocket 是单独 ADR 议题"——明确不背书 WebSocket，未来需要再单独评审。这种"知道自己边界"的 ADR 比包打天下的强。
- 与 ADR-015 的关系是 **refine 不是 supersede**（措辞干净，不打脸自己）。

| # | Severity | 问题 | 建议改法 |
|---|---|---|---|
| 1 | **High** | Decision #3 强制 ingress 配置示例使用 `nginx.ingress.kubernetes.io/proxy-buffer-size: "8k"`——但这与 Decision #3 注释"大响应防 nginx 缓存"逻辑矛盾：proxy-buffer-size 是**单个缓冲区大小**，不是是否缓冲；要"不缓冲"应该 `proxy-buffering: off`（已有）+ 不需要也不应该改 buffer-size。8k 是个无意义的值。 | 删除 `proxy-buffer-size: "8k"` 这行（或改为 "可选，若客户已有大 header 需求可调，与 SSE 无关"）；只保留 proxy-buffering: off + proxy-read/send-timeout 三条核心 annotation。 |
| 2 | **High** | Decision #4 DoD 三项实测（TC-SSE-001/002/003）写得好，但**没有写 CI 跑在什么环境**——kind cluster？真实云？哪个 nginx-ingress-controller 版本？没有 CI 跑不动的 DoD 等于没 DoD（参见 PRD-Review-2026-05-31-PRD005-006.md X3 finding 已踩过）。 | Decision #4 加一段："TC-SSE-001/002/003 在 GitHub Actions 用 kind v0.20+ + nginx-ingress-controller v1.10+ (官方 chart 默认 values) 部署 supkube-helm 默认 chart 跑；TC-SSE-003 N=50 并发由 hey/wrk 模拟。结果产物为 CI artifact `sse-test-report.json`，PR 合入前必须 green。" |
| 3 | **High** | Decision #3 4 个 nginx ingress annotation 写在"`supkube-helm/templates/ingress.yaml`"——但 ADR-016 "ConfigMap-mounted nginx" 讲的是 SupKube SPA 前端用的 nginx（pod 内），不是 ingress nginx。这两个 nginx 不是同一个。ADR-036 引用 ADR-016 "本 ADR 涉及的 ingress 配置承载介质" 措辞不准确，会让读者以为 ADR-016 的 ConfigMap 就承载这些 annotation。 | 改为"本 ADR 的 4 个 annotation 写在 supkube-helm chart 的 `templates/ingress.yaml`，由 K8s ingress controller（nginx-ingress / traefik / istio gateway）消费。与 ADR-016 ConfigMap-mounted nginx（SPA 前端 pod 内 nginx）无关。"删除 References 段对 ADR-016 的关联，或改为"对照说明：ADR-016 是 SPA 前端 nginx，本 ADR 是 ingress nginx，两者独立"。 |
| 4 | **Med** | Decision #5 "前端必须实现 SSE 不可用时自动降级"——但 PRD-005 v1 是否真有 5s 轮询实现？记忆 `feedback_verify_before_architect` 和座右铭 §11.2 都强调"实测优先于架构决策"。如果 v1 没实现 5s 轮询，ADR-036 第 5 条是空头支票。 | 评审通过前 verify：查 PRD-005 v1 实现的 `frontend/src/composables/`（或对应位置）是否有 polling fallback；如无，ADR-036 第 5 条应改为"PRD-005 v2.1 实施时一并实现 polling fallback（共用 useStream composable）"。 |
| 5 | **Med** | Decision #5 fallback "EventSource.CLOSED → 改 polling" 的判断不充分——SSE 在某些 ingress 下会**保持连接但永远不到数据**（buffer 持续攒），EventSource 不会进 CLOSED 状态。这种情况需要 watchdog（如 30s 无 message 强制重连/降级）。 | Decision #5 代码示例补一段："`heartbeat` 机制：后端每 15s 发 `: heartbeat` SSE comment 行；前端 30s 未收到任何消息（heartbeat 或 data）即触发降级"。 |
| 6 | **Med** | Decision #1 SupKube v1 内置 SSE 用例表 3 条（Live Tail / Backup 进度 / Cluster Health）——但 PRD-004 §4 MCP 流式响应（Decision #6 of ADR-034 `text/event-stream`）也是 SSE 形态。这与本 ADR Decision #2 黑名单"MCP 协议层要求：不允许在 MCP 端点上加 SSE"如何调和？ | 澄清术语：ADR-034 的"Streamable HTTP 响应可以是 `text/event-stream`"是 MCP 协议**内**的流式响应，复用 SSE 媒体类型但不属于本 ADR-036 SSE 治理范围；本 ADR-036 仅治理"端点 = SSE 端点（如 `GET /api/v1/logs/stream`）"。ADR-036 Decision #2 第二行改为"MCP 端点设计层（GET /mcp/sse 双端点架构）不允许；但 MCP 响应体使用 text/event-stream 流式格式是允许的，由 ADR-034 治理"。 |
| 7 | **Info** | Decision #4 "TC-SSE-001 延迟：客户端逐行到达延迟 < 500ms（用 `curl -N \| ts %.S` 验证）"——`ts` 是 `moreutils` 包工具，CI 镜像不一定预装。 | 改为 "用 `curl -N \| awk '{ print strftime(\"%H:%M:%S.%3N\"), $0 }'`" 或 Python 一行，避免依赖。 |
| 8 | **Info** | Alternatives Considered "长 polling" 段说"客户端实现复杂度比 SSE 高（要管 cursor / 重试 / 去重）"——但 5s 轮询也要管 cursor / 去重，本 ADR Decision #5 fallback 即如此。措辞不严谨。 | 改为 "长 polling 在 ingress 行为上需要同样配 timeout，且客户端要管连接重启 + 时间窗对齐；与 SSE 相比无明显工程优势。" |

**评审结论：Accepted with conditions**（finding #1–#3 必修；#4 verify-before-architect 验证；#5–#6 强烈建议研发期前修；#7–#8 文档卫生）

---

## 三、跨 ADR 一致性

### 3.1 ADR-033（AI Advisor）⇔ SECURITY.md §6（脱敏 / 出境）：单一来源是否两边引用一致

**核对结果：基本一致，3 处需对齐**：

| 议题 | ADR-033 写法 | SECURITY.md §6 写法 | 一致性 |
|---|---|---|---|
| 脱敏函数签名 | `Sanitize(input Payload) (Payload, SanitizeReport, error)` | `Sanitize(ctx, payload) (sanitized, fingerprint, err)` | ❌ drift（finding #1） |
| 脱敏对象类型 | secret-like / PII / 集群指纹 / 客户名 + namespace | Secret values / env 中含 PASSWORD/TOKEN 等 key / JWT / IP / email / IBAN / 卡号 | 🟡 ADR-033 偏抽象，SECURITY.md 偏具体；不矛盾但表达粒度不一 |
| Provider 白名单 | Ollama / DeepSeek / Claude / Azure OpenAI | Ollama / DeepSeek / Claude / GPT-4 / BYO OpenAI-compatible | ❌ drift（finding #3）|
| 合规默认 | Helm `ai.provider.default = "ollama"` | 默认本地 Ollama，0 字节出境 | ✅ 一致 |
| outbound.acknowledged | "若 default 是 SaaS 且 acknowledged != true 拒绝启动" | "从 Ollama 切到 SaaS 必须 UI 二次确认" | 🟡 表述不一（启动时 vs 切换时），但不冲突 |
| 审计字段 | code: `AI_CALL_*` + sanitize report | timestamp / provider / caller / user / cluster / input_fingerprint / output_digest / egress_bytes / cost_estimate_usd / duration_ms / whitelist_version 共 11 字段 | 🟡 ADR-033 提了"sanitize report"，SECURITY.md 列了 11 字段；需相互引用，避免实施时各取一份 |
| 客户 master switch | 未提 | §6.F 详细写了 master switch + 分项开关 | 🟡 ADR 应至少提一句"客户 opt-out 走 SECURITY.md §6.F" |

**建议**：评审前在 ADR-033 Decision #1 加一句"本 ADR 是工程实现，合规约束以 SECURITY.md §6 为准；冲突时 SECURITY.md §6 优先"，并修 finding #1 + #3。

### 3.2 ADR-034（MCP）⇔ ADR-036（SSE 项目级口径）：MCP 不用 SSE 决策跟 SSE 项目级口径不矛盾

**核对结果：一致，但有 1 处术语混乱**（已记 ADR-036 finding #6）

- ADR-034 Decision #1 "Streamable HTTP 响应可以是 `text/event-stream`"——是 MCP 协议层内的流式响应格式。
- ADR-036 Decision #2 黑名单"MCP 端点不允许 SSE"——指 SSE **端点架构**（GET /sse + POST /messages 双端点）。

两份 ADR 共用 SSE 这个词但含义不同。建议 ADR-036 Decision #2 明确"指 SSE 端点架构，不指 text/event-stream 媒体类型"。

ADR-034 References 引用了 ADR-036（"Live Tail 用 SSE 合理，MCP 不用"）；ADR-036 References 引用了 ADR-034（"MCP 明确不用 SSE"）——双向引用对称，是好实践。

### 3.3 ADR-035（结构化日志）⇔ ADR-033（AI 根因 anchor）：err_code 依赖关系

**核对结果：依赖关系清楚但单向**

- ADR-035 §1 Context 痛点 #4 "PRD-003 AI Advisor 根因分析靠 grep 文本不稳；如果日志带 `code` 字段，prompt 可以...准确度上一个台阶"——明确把 ADR-035 定位为 ADR-033 的承重基础。
- ADR-035 后续要做 #3 "ADR-033 Advisor Engine 的审计日志直接用本 ADR 的 schema（`code: AI_CALL_*` + `ctx: { provider, model, sanitize_redacted_count }`）"——具体落地点也写了。
- ADR-033 References 引用了 ADR-035；ADR-035 References 也引用了 ADR-033。双向引用对称。

但 ADR-035 Decision #5 首批 20 个 ERR_* 仅列了 `ERR_AI_PROVIDER_TIMEOUT` + `ERR_AI_OUTBOUND_BLOCKED` 2 条 AI 相关 code，而 ADR-033 提到的 `AI_CALL_*` 系列（用于 audit）没有 code 列表。**建议**：ADR-035 Decision #5 表格补 `EVT_AI_CALL_STARTED` / `EVT_AI_CALL_COMPLETED` / `EVT_AI_SANITIZE_REPORT` 至少 3 条事件 code，作为 ADR-033 audit 的承重。

### 3.4 ADR-036（SSE）⇔ ADR-015（历史不用 SSE）：supersede or refine

**核对结果：refine（不是 supersede），措辞干净**

- ADR-015 行 1057 原文："不用 WebSocket / SSE：复杂度↑，K8s nginx ingress 兼容性问题，5s 足够"——这是 polling cadence ADR 的副论点，主论点是"轮询调速"。
- ADR-036 Context 段直接引用 ADR-015 原文 + 加"实际情况：SSE 不是错的，Live Tail 是 SSE 的标准用例，ADR-015 的顾虑有解法"——明确 refine 而非否定。
- ADR-036 Consequences "ADR-015 不再是'禁止 SSE'的全否定，而是'默认不用 + 有解法时用'的有条件许可"——把关系措辞到产品级。

**建议**：ADR-015 也加一行"修订（2026-05-31，v0.9.x）— Live Tail 场景下的 SSE 由 ADR-036 给出有条件许可"，避免单读 ADR-015 不知道豁免存在。

### 3.5 ADR-019（stdout 双写）⇔ ADR-035（结构化 logger）：双写策略兼容性

**核对结果：方向兼容但细节未阐明**（已记 ADR-035 finding #4）

- ADR-019 行 1080 决策："K8s Events + stdout 双写"——K8s Events 给短期 UI 查询，stdout 给长期 SIEM 采集。
- ADR-035 后续要做 #2："ADR-019 审计日志升级到 zerolog（stdout 那一路），保留 K8s Events 一路不变"——明确只升级 stdout 那一路。

但 ADR-019 行 1101 "实现亮点" 段提到的 `[audit]` 前缀字符串如何迁移到 zerolog 字段（`component: audit`），ADR-035 没有具体写。客户 stdout → SIEM 管道**当前是按 `[audit]` 前缀 grep 的**，升级到 JSON 后客户的 SIEM rule 全部要改。这是兼容期内的硬约束，必须双向引用 + 兼容期内同时输出两种格式。

### 3.6 4 ADR 引用其他文档（PRD / SECURITY / 既有 ADR）的准确性 — 实际 grep 验证

| 引用 | 实际验证 | 准确性 |
|---|---|---|
| ADR-033 引用 SECURITY.md "§ AI 数据处理与出境治理" | SECURITY.md §6（行 77）存在 | ✅ |
| ADR-033 引用 ADR-031（5 层韧性） | ADR-031（行 2205）存在 | ✅ |
| ADR-033 引用 PRD-003 §5（置信度 UI） | PRD-003 §5（行 880+）存在 | ✅ |
| ADR-034 引用 ADR-015（不用 SSE） | ADR-015（行 1042）存在 | ✅ |
| ADR-034 引用 ADR-020（Bearer Token） | ADR-020（行 1158）存在 | ✅ |
| ADR-034 引用 ADR-018（RBAC 3 角色） | ADR-018（行 1107）存在 | ✅，但 ADR-018 未提 MCP token（finding #3）|
| ADR-035 引用 PRD-005 §4.4 / §4.5 | PRD-005 §4.4（行 1487+）/ §4.5（行 1508+）存在 | ✅ |
| ADR-035 引用 ADR-019（审计日志） | ADR-019（行 1065）存在 | ✅，但 ADR-019 不知道 stdout 一路要升级（finding #4）|
| ADR-036 引用 ADR-016（ConfigMap-mounted nginx） | ADR-016（行 1059）存在 | ⚠️ 引用不准（finding #3）|
| ADR-036 引用 PRD-005 §4.3 | PRD-005 §4.3（行 1450）存在 | ✅ |

**结论**：内部引用大部分准确，3 处需要回写到既有 ADR（ADR-018 / ADR-019 / ADR-020 的"后续要做"或"修订段"），避免单读旧 ADR 时不知道有新约束。

---

## 四、跟 PRD-001~006 修订后的对齐验证

### 4.1 PRD-003 §7.2 脱敏治理 ⇔ ADR-033 §脱敏管线 ⇔ SECURITY.md §6.C：三处口径

**核对结果：方向一致，3 处需统一**：

| 议题 | PRD-003 §7.2 | ADR-033 Decision #4 | SECURITY.md §6.C |
|---|---|---|---|
| 函数签名 | （未指明）| `Sanitize(input Payload) (Payload, SanitizeReport, error)` | `Sanitize(ctx, payload) (sanitized, fingerprint, err)` |
| 实施位置 | （未指明）| `internal/advisor/sanitize.go` | backend `internal/advisor/sanitize.go` |
| 字段白名单 | K8s metadata / Velero CR / Secret values / ConfigMap values / PVC 数据 / 日志 / 用户身份 / Audit log | secret-like / PII / 集群指纹 / 客户名+namespace | §6.C 规则表 9 行 + §6.D 默认白名单 |
| 出境 audit 字段 | `redacted_fields` + `outbound_byte_count` + `provider` | code `AI_CALL_*` + sanitize report | 11 字段（timestamp / provider / caller / user / cluster / input_fingerprint / output_digest / egress_bytes / cost_estimate_usd / duration_ms / whitelist_version） |

**建议**：PRD-003 §7.2 与 ADR-033 + SECURITY.md §6 的合并审查（agent 1 在并行做 PRD-007，可一并触达）；统一函数签名 + audit 字段后，三份文档都引用 SECURITY.md §6.C/§6.E 作为权威。

### 4.2 PRD-004 §4.1 Streamable HTTP 改写 ⇔ ADR-034 决策

**核对结果：完全同步**

PRD-004 行 1097 / 1137 / 1146 / 1197 / 1287 / 1310 / 1369 等多处显式引用 finding T2 / ADR-034 + "Streamable HTTP v1 主推, SSE 仅向后兼容"——口径与 ADR-034 Decision #1 + #2 一致。PRD-004 DoD #17 "Bearer Token 走 header 不进 URL" + DoD #18 "nginx ingress 上 Streamable HTTP 长连接稳定 30min 不被 buffer" 都精准落到 ADR-034 finding #3 + ADR-036 finding #2。

**唯一漏洞**：PRD-004 未引用 ADR-035 §5 #15 `ERR_MCP_TRANSPORT_DEPRECATED` 作为旧 client 连接时的返回 code（已记 ADR-034 finding #6）。

### 4.3 PRD-005 §4.4 三层日志 + §4.5 ERR_* ⇔ ADR-035：编号是否 PRD-005 全部已改（X2 finding）

**核对结果：基本闭环**

PRD-005 行 1383 / 1492 / 1506 / 1740 / 1767 / 1793 等多处已将原"ADR-033 (拟) 结构化日志规范" 改为"ADR-035 (拟) 结构化日志规范 + 错误代码体系"，并明确"X2 重编 2026-05-31, 替代原 ADR-033 撞号的写法"。

X2 finding（ADR-033 编号撞车）已闭环 ✅。架构设计.md §ADR 号台账（行 28-47）已建立单一来源，新增编号规则也写进去了。

### 4.4 PRD-005 §4.3 SSE Live Tail + ingress 配置 ⇔ ADR-036

**核对结果：完全同步**

PRD-005 §4.3（行 1450）显式引用 X3 finding + ADR-036 + 4 个 nginx ingress annotation + DoD 中 SSE 测试。helm chart values 默认含 4 个 annotation + USER_MANUAL §SSE 配置章节都已规划——这是 ADR-036 Decision #3 落地的入口。

**注意**：PRD-005 §4.3 的 ingress annotation 列表（行 1474）含 `proxy-buffering: off` + `proxy-read-timeout: "3600"` + `proxy-send-timeout: "3600"` 共 3 条，**没有 ADR-036 误加的 `proxy-buffer-size: "8k"`**——PRD-005 比 ADR-036 更准。ADR-036 finding #1 修订时应对照 PRD-005 §4.3。

---

## 五、与既有 ADR-001~031 的一致性 / 冲突

### 5.1 ADR-003（Transform Set = Velero resourceModifierRef, len(Data)==1）⇔ PRD-002 v1.2 修订 ⇔ 本次 4 ADR

**核对结果：无冲突，PRD-Review T1 finding 已闭环**

ADR-003 已加"修订（2026-05-31, v0.9.x）— 两层模型 + 编译派生 ConfigMap"段（行 905-924）。本次 4 个新 ADR **无一与 ADR-003 冲突**（4 个新 ADR 都不涉及 Transform / Velero 编译契约）。

PRD-Review 8 finding T1 闭环 ✅。

### 5.2 ADR-014（启动时迁移机制）⇔ PRD-002 Phase 0 修订

**核对结果：无冲突，但 ADR-014 与本次 4 ADR 也无关联**

PRD-002 Phase 0 已引用 ADR-014 复用。本次 4 ADR 与迁移机制无关；但 ADR-035 旧 logger 兼容期 6 个月可能涉及类似"启动时迁移"模式，建议在 ADR-035 Consequences 加一行"legacy wrapper 在启动时注册全局拦截器，类似 ADR-014 迁移机制但更轻量"。

### 5.3 ADR-015（不用 SSE）⇔ ADR-036（SSE 项目级口径）：supersede vs refine

**核对结果：refine，措辞干净（见 §3.4）**

PRD-Review 8 finding X3 闭环 ✅（前提是 ADR-015 也加修订段，见 §3.4 建议）。

### 5.4 ADR-019（stdout 双写）⇔ ADR-035（结构化 logger）

**核对结果：兼容但细节未阐明（见 §3.5）**

需要 ADR-035 finding #4 修订（已记）。ADR-019 也应同步加修订段。

### 5.5 ADR-018（RBAC 3 角色）⇔ ADR-020（Bearer Token）⇔ ADR-034（MCP token）：三方关系

**核对结果：未定义清楚**（已记 ADR-034 finding #3 + #5）

ADR-018 + ADR-020 不知道 ADR-034 要扩展 token type。需双向回写。

### 5.6 ADR-031（5 层韧性）⇔ ADR-033（AI Advisor）

**核对结果：完全对齐，是关系最干净的一对**

ADR-033 Decision #6 Resilience Score 由规则引擎计算明确挂钩 ADR-031（"Resilience Score 是 SRE 专家 Epic 的产品化"）；ADR-031 §5 定位是 "Kasten / Veeam 级数据韧性平台"，Advisor 是该平台的产品化形态。无冲突。

---

## 六、建议的行动优先级

| 序 | 行动项 | 归属 ADR | 时机 |
|---|---|---|---|
| 1 | **ADR-035 必修 4 项**（zerolog benchmark 引用 + 20 个 code owner + 6 个月兼容期与 v0.10.x 时间线对齐 + ADR-019 stdout 升级路径）→ Needs Revision → Accepted 评审 | ADR-035 | 评审 round-2 必修，PRD-005 v2.2 开发前 |
| 2 | **ADR-033 必修 3 项**（脱敏函数签名与 SECURITY.md §6.C 对齐 + 置信度三档解析 fallback + provider 白名单与 SECURITY.md §6.B 对齐）| ADR-033 | Accepted with conditions → 1 sprint 内修订 |
| 3 | **ADR-036 必修 3 项**（删 proxy-buffer-size + DoD CI 环境写明 kind + ADR-016 引用澄清）| ADR-036 | Accepted with conditions → 1 sprint 内修订 |
| 4 | **ADR-034 必修 3 项**（SSE deprecation runbook + stdio v1.1 版本锚 + MCP token RBAC 三方关系）| ADR-034 | Accepted → 1 sprint 内修订 |
| 5 | **回写既有 ADR 3 处修订段**（ADR-015 加"SSE refine"段 + ADR-019 加"stdout 升级"段 + ADR-020 加"MCP token type"段）| 跨 ADR | 与本批 ADR Accepted 同期 |
| 6 | **verify-before-architect**：PRD-005 v1 5s 轮询 fallback 是否真实现（ADR-036 finding #4）| ADR-036 | 评审前 verify |
| 7 | ADR-LEDGER 维护规则补全：ADR-032~036 状态从"草稿"更新为"Accepted/Needs Revision"，并明确 ADR 评审通过的标准操作流程（PR review + Mars 签字？）| 全局 | 本批评审收尾 |

---

## 七、与 PRD-Review 系列的关系

本次承接前两份（PRD-Review-2026-05-31.md + PRD-Review-2026-05-31-PRD005-006.md），8 finding 闭环验证：

| Finding | 描述 | 闭环路径 | 状态 |
|---|---|---|---|
| **T1** | PRD-002 编译契约（新两层模型如何编译为 Velero ConfigMap） | PRD-002 §4.1bis + ADR-003 修订段 | ✅ 全闭环（与本次 4 ADR 无关）|
| **T2** | MCP SSE 弃用 → Streamable HTTP | PRD-004 §4.1/§4.4/§6 + **ADR-034** | ✅ 全闭环（ADR-034 Accepted with 3 必修项）|
| **T3** | SC→Immediate 多 AZ 拓扑风险 | PRD-001/002 已加拓扑校验段 | ✅ 全闭环（与本次 4 ADR 无关）|
| **T4** | AI 出境（默认外发集群上下文给第三方 LLM） | **ADR-033** + SECURITY.md §6 | 🟡 大部分闭环（ADR-033 Accepted with 3 必修项，函数签名 + 白名单需对齐 SECURITY.md）|
| **X1** | PRD-005 / PRD-006 deep-link 契约不一致 | PRD-005 §4.8 权威 + PRD-006 §4.6 引用 | ✅ 全闭环（与本次 4 ADR 无关）|
| **X2** | ADR-033 编号撞车（AI Advisor vs 结构化日志） | ADR-LEDGER（架构设计.md §ADR 号台账，行 28-47）+ 重编号（AI Advisor = ADR-033, 结构化日志 = ADR-035）| ✅ 全闭环 |
| **X3** | SSE 与 ADR-015 冲突 | **ADR-036**（refine ADR-015）+ **ADR-034**（MCP 不用 SSE）| ✅ 全闭环（ADR-036 Accepted with 3 必修项；ADR-015 需加修订段）|
| **X4** | AI 日志外发治理 | **ADR-033** + SECURITY.md §6 + PRD-003 §7.2 单一脱敏管线 | 🟡 大部分闭环（与 T4 同源；脱敏函数签名 drift 待修）|

**闭环总数：✅ 6 全闭环 / 🟡 2 大部分闭环 / ❌ 0 缺**

T4 + X4 的 🟡 都是同源问题（脱敏函数签名 + provider 白名单与 SECURITY.md §6 不完全对齐），属同一组修订；如修完 ADR-033 finding #1 + #3，即升 ✅。

---

## 八、未决主题（跟踪）

实施期内可能暴出的新 ADR（列预测，不阻塞当前评审）：

1. **ADR-037（候选）**：MCP token 与 RBAC 三方关系（ADR-018 + ADR-020 + ADR-034），如 ADR-034 finding #3 修订只是补一行而无法单 ADR 容纳完整模型，则单开 ADR-037。
2. **ADR-038（候选）**：Prompt 模板版本化（ADR-033 finding #7），prompt 是产品的一部分，治理流程独立成 ADR 更清晰。
3. **ADR-039（候选）**：WebSocket 项目级口径（ADR-036 Decision #6 显式 punt 给未来 ADR），首次有 PRD 需要双向通信时触发。
4. **ADR-040（候选）**：Kanister Blueprints 集成（v0.10.0 主题），与 ADR-031 + ADR-033 都有交叉。

**ADR-LEDGER 维护规则**（现状 + 建议）：

- 当前：架构设计.md §ADR 号台账（行 28-47）作为单一来源；新增 ADR 必须先登记再写正文。规则清晰。
- 建议：增加"ADR 状态机"小节——状态枚举 {草稿 → 评审中 → Accepted / Needs Revision / Rejected / Superseded}，状态转换需 PR + Mars 签字；并约定 Accepted ADR 不再编辑（只能 supersede / refine 修订段）。

---

## 附录：技术核实来源

**外部来源**

- MCP 2025-03-26 spec — [Streamable HTTP 取代 HTTP+SSE 的官方声明](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)
- HTTP Deprecation / Sunset header — [RFC 8594](https://www.rfc-editor.org/rfc/rfc8594)
- zerolog README benchmark — [https://github.com/rs/zerolog#benchmarks](https://github.com/rs/zerolog#benchmarks)
- zap README performance — [https://github.com/uber-go/zap#performance](https://github.com/uber-go/zap#performance)
- OpenTelemetry W3C trace context — [https://www.w3.org/TR/trace-context/](https://www.w3.org/TR/trace-context/)
- nginx-ingress proxy-buffering 默认行为 + 注解参考 — [https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/)
- HTML Living Standard - Server-Sent Events — [https://html.spec.whatwg.org/multipage/server-sent-events.html](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- Velero v1.18 限制（ADR-026）— Velero docs（csi / csi-snapshot-data-movement）

**仓库内部来源**

- 架构设计.md：§ADR 号台账（行 28-47）、ADR-003（行 891 + 修订段 905）、ADR-006（行 957）、ADR-014（行 1033）、ADR-015（行 1042）、ADR-016（行 1059）、ADR-018（行 1107）、ADR-019（行 1065）、ADR-020（行 1158）、ADR-031（行 2205）、ADR-032（pre-ADR 行 2289+）、**ADR-033（行 2410）、ADR-034（行 2573）、ADR-035（行 2761）、ADR-036（行 2954）**
- SECURITY.md：§6 AI 数据处理与出境治理（行 77-263），含 §6.A 适用范围 / §6.B Provider 选型 / §6.C 脱敏管线 / §6.D 出境白名单 / §6.E 审计要求 / §6.F 客户可关闭性 / §6.G 与漏洞报告/供应链关系
- PRD.md：PRD-003 §7.2 脱敏与外发治理（行 958-980）、PRD-004 §4.1/§4.4/§6（行 1085+ 多处 Streamable HTTP 引用）、PRD-005 §4.3 SSE Live Tail（行 1450+）+ §4.4 三层日志（行 1487+）+ §4.5 ERR_*（行 1508+）+ §4.8 deep-link schema 权威（行 1592+）
- 前两份 PRD-Review：PRD-Review-2026-05-31.md（T1-T4，PRD-001~004）+ PRD-Review-2026-05-31-PRD005-006.md（X1-X4，PRD-005~006）

---

<!--
评审元数据
- 评审轮次: ADR-Review 第一份（ADR-033~036，即 PRD-Review 系列第三份）
- 关键结论:
  * ADR-033 Accepted with 3 必修项（脱敏函数签名 / 置信度三档 fallback / provider 白名单 vs SECURITY.md §6.B）
  * ADR-034 Accepted with 3 必修项（SSE deprecation runbook / stdio v1.1 版本锚 / MCP token RBAC 三方关系）
  * ADR-035 Needs Revision with 4 必修项（zerolog 选型 evidence / 20 个 code owner / 6 个月兼容期 vs v0.10.x 时间线 / ADR-019 stdout 升级路径）
  * ADR-036 Accepted with 3 必修项（删 proxy-buffer-size 8k / DoD CI 环境写明 / ADR-016 引用澄清）
- finding 总数: 0 Blocker / 9 High / 11 Med / 6 Info
- 8 finding 闭环: ✅ 6 全闭环 (T1/T2/T3/X1/X2/X3) / 🟡 2 大部分闭环 (T4/X4，同源待修) / ❌ 0 缺
- 关联 PRD-Review: PRD-Review-2026-05-31.md（PRD-001~004，T1-T4） + PRD-Review-2026-05-31-PRD005-006.md（PRD-005~006，X1-X4）
- 下次评审: 4 ADR 修订后 round-2 评审（PRD-Review/ADR-Review-YYYY-MM-DD-round2.md），主要确认 ADR-035 是否升 Accepted
-->
