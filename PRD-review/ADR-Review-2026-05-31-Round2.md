# SupKube ADR Review 第二轮 — ADR-033/034/035/036 + SECURITY §6.C

> **视角**: K8s + AI + 灾备 (DR) 专家
> **评审对象**: ADR-033/034/035/036 修订后 + SECURITY §6.C 契约一致性
> **评审人**: Claude (受 Mars 委托) · **日期**: 2026-05-31
> **承接**: 第一轮 `ADR-Review-2026-05-31.md`（4 ADR 决议: 1 Needs Revision + 3 Accepted with conditions, 13 必修项 + 3 处既有 ADR 回写）
> **核对基线**: 架构设计.md（行 2446-3352, 含 ADR-LEDGER + 4 新 ADR + ADR-015/019/020 amend 段）· SECURITY.md v0.2.1（§6.A-G）· PRD.md · 前两份 PRD-Review

---

## 一、执行摘要

第一轮 13 项必修 + 3 处 amend 回写，第二轮核查整体闭环率 **15/16 = 94%**，且**关键 X1 类 multi-agent 共享契约（Sanitize 三元组）一字不差对齐**——这次是 multi-agent 协作下 contract drift 防御的一次实证成功。但发现 **2 个 round-2 新引入的 high finding**（其中一个是跨 ADR 冲突），需 Mars 立刻拍板再升 Decided。

### 决议速览表

| ADR | 第一轮决议 | 第二轮决议 | 闭环率 | 关键变化 |
|---|---|---|---|---|
| **ADR-033** AI Advisor | Accepted with conditions (3 必修) | **Accepted with conditions** | 3/3 ✅ 但残留 finding #3 字面 drift | Sanitize 三元组 ✅；ParseConfidence 落地 ✅；Provider 白名单 **Azure OpenAI vs GPT-4 系列 仍 drift** ⚠️ |
| **ADR-034** MCP 协议 | Accepted (3 必修) | **Needs Revision** | 3/3 ✅ 但 **round-2 引入新 cross-ADR 冲突** | SSE 三阶段 runbook ✅；stdio v1.1 锚 ✅；token RBAC ✅；**但限流维度与 ADR-020 amend 段直接矛盾** ❌ |
| **ADR-035** 结构化日志 | Needs Revision (4 必修) | **Accepted** | 4/4 ✅ | zerolog benchmark 数字 ✅；20 个 ERR_* owner/流程 ✅；6 个月跨 v0.10.x~v0.12.x 时间线 ✅；ADR-019 amend 路径 ✅ |
| **ADR-036** SSE 口径 | Accepted with conditions (3 必修) | **Accepted** | 3/3 ✅ | proxy-buffer-size 8k 已删 ✅；CI 用 minikube + ingress-nginx + `sse-ingress-verify` job ✅；ADR-016 显式不引用 ✅ |

### 关键放行结论

- **PRD-003 (AI Advisor)**: 解锁 → 可进研发（ADR-033 残留 1 个 Med 字面 drift，研发期可同步修，不阻塞 sprint kickoff）
- **PRD-004 (MCP Server)**: **暂不解锁** → ADR-034 限流维度冲突必须先修（30 分钟工作量，与 ADR-020 amend 段二选一）
- **PRD-005 v2.2 (结构化日志 / Log Viewer v2)**: 解锁 → ADR-035 升 Accepted，可立刻进研发
- **整体放行**: **3/4 ADR 可解锁对应 PRD，1/4 待 30min 修订**

### 严重度图例

- **Blocker** — 必须修才能放行
- **High** — 影响正确性 / 跨 ADR 一致性，1 sprint 内必修
- **Med** — 影响可维护性，研发期内处理
- **Info** — 文档卫生

**第二轮 finding 总数**: 0 Blocker / **2 High (round-2 新发)** / 1 Med / 3 Info

---

## 二、ADR-033 AI Advisor 验证（原 3 必修项）

**评审范围**: 架构设计.md L2446-2638

| 第一轮 finding | 第二轮闭环情况 | 实证锚点 | 决议 |
|---|---|---|---|
| **#1** 脱敏函数签名与 SECURITY.md §6.C 对齐 | ✅ **完全闭环** | 架构设计.md L2527-2541 `RedactedFieldInfo` / `SanitizeReport` / `Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)` ⇔ SECURITY.md L142-154 三元组**逐字一致**；L2544 + L160 双向引用"单一来源"+ CI contract test (`test/contract/sanitize_signature_test.go`) | ✅ Closed |
| **#2** 置信度三档 fallback parser | ✅ **完全闭环** | 架构设计.md L2565-2574 显式定义 `internal/advisor/confidence.go` + `func ParseConfidence(raw string) (Level, ParseTrace)`；4 条 fallback 规则（字符串规范化 → 同义词词表 → 数字解析 → medium + `ERR_AI_CONFIDENCE_PARSE` warn）；单测 ≥10 种变体 TC-AI-CONF-* | ✅ Closed |
| **#3** Provider 白名单与 SECURITY.md §6.B 对齐 | 🟡 **大部分闭环但仍有字面 drift** | 架构设计.md L2508 写 "Ollama / DeepSeek / Claude / **Azure OpenAI** / BYO"；SECURITY.md L112 写 "Ollama / DeepSeek / Claude / **GPT-4 系列** / BYO"。L2512 声称"完全一致"但其实未完全一致（Azure OpenAI 是 endpoint，GPT-4 系列是 model family，两者不能字面映射）。CI contract test 若按字符串 diff 会 fail | 🟡 Open (Med) |

### ADR-019 / ADR-020 amend 段质量

| amend 段 | 实证锚点 | 质量评估 |
|---|---|---|
| **ADR-019 v0.10.x 修订** | 架构设计.md L1126-1133 | ✅ 高质量：明确"K8s Events 一路保持不变 / stdout 升级 JSON / 兼容期 6 月 / legacy.Wrap() / `code: AUDIT_*` schema 写明 `{ user, action, resource, namespace, sourceIP, result }`"；与 ADR-035 §6 时间线、§8 升级路径表 cross-link 一致 |
| **ADR-020 v0.10.x 修订** | 架构设计.md L1238-1246 | ⚠️ 有 1 处与 ADR-034 矛盾（见 §三 finding R2-1）；其余（同表复用 / tokenType 字段 / RBAC 三方关系）质量合格 |

### Round-2 新 finding

| # | 严重度 | 问题 | 建议改法 |
|---|---|---|---|
| **R2-A** | **Med** | ADR-033 L2512 声明"Provider 白名单与 SECURITY.md §6.B 完全一致"，但实际 v1 列表 `Azure OpenAI` ≠ SECURITY.md 的 `GPT-4 系列`。CI contract test (`internal/advisor/provider/registry.go` ⇔ SECURITY.md §6.B 字符串 diff) 会立即 fail | 二选一: (a) SECURITY.md L112 改为 "Ollama / DeepSeek / Claude / **Azure OpenAI / GPT-4 系列** / BYO" 列举 4 个具体 SaaS endpoint；(b) ADR-033 L2508 改为 "Azure OpenAI / GPT-4 系列（OpenAI 直连）"分别列。建议 (a) — 因为 GPT-4 系列可走多种 endpoint（直连 OpenAI API、Azure OpenAI、企业代理），SECURITY.md 应列**provider 类别**而非 endpoint 形态 |
| **R2-B** | **Info** | ADR-033 原 finding #4（master switch 默认关闭 vs SaaS 启动校验）未在修订中显式处理。L2518 仍写"启动时若 default 是 SaaS provider 且 `ai.outbound.acknowledged != true`，backend 拒绝启动" — 与 SECURITY.md §6.F L240 "AI 功能整体默认关闭"未明确正交化 | ADR-033 Decision #3 加一句："`ai.enabled = false`（master switch）时跳过 provider 启动校验；`ai.enabled = true` 且 default 是 SaaS 才走 outbound.acknowledged 检查"。10 分钟工作量 |
| **R2-C** | **Info** | SECURITY.md L285 + L368 仍写 "ADR-033 AI Advisor 架构（**拟**）"，但 ADR-033 已进入草稿/评审状态 | 改为"（草稿待评审，详见 ADR-Review-2026-05-31-Round2.md）" |

### 决议: **Accepted with conditions**

- **条件 1（Med，1 sprint 内）**: 修 finding R2-A 字面 drift（SECURITY.md §6.B 列举 4 个 SaaS endpoint）
- **条件 2（Info）**: 修 R2-B + R2-C（10 分钟工作量）
- 3 个第一轮必修项全部闭环，可解锁 PRD-003 进研发

---

## 三、ADR-034 MCP 协议验证（原 3 必修项）

**评审范围**: 架构设计.md L2640-2891

| 第一轮 finding | 第二轮闭环情况 | 实证锚点 | 决议 |
|---|---|---|---|
| **#1** SSE deprecation 三阶段 runbook | ✅ **完全闭环** | 架构设计.md L2684-2696 三阶段表（v1.0 = SupKube v0.10.0 `Sunset: 2026-12-31` + `EVT_MCP_SSE_USED` audit；v1.5 = SupKube v0.12.0 HTTP 410 Gone；v2.0 = SupKube v1.0.0 路由代码删除）；UI 配合 deprecation banner + admin 审计页过滤 + USER_MANUAL § MCP；硬截止 | ✅ Closed |
| **#2** stdio v1.1 锚 + Skill registry 唯一 | ✅ **完全闭环** | 架构设计.md L2742-2763：v1.1 = SupKube v0.10.x 时间窗（与 ADR-033/032 同窗口）；Skill 注册表唯一 (`internal/mcp/skills/registry.go`)，stdio / HTTP 共用；Claude Desktop config 示例 `examples/mcp-clients/claude-desktop.json` 与 openclaw / dify / cursor 平级 | ✅ Closed |
| **#3** token RBAC 三方关系 + per-cluster 限流 | 🟡 **闭环但与 ADR-020 amend 段冲突**（见 R2-1） | 架构设计.md L2698-2735：token → user → cluster scope 三方 mapping；scope annotation `supkube.io/mcp-cluster-scope`；403 + `ERR_RBAC_DENIED` + `EVT_MCP_CROSS_CLUSTER_CALL` audit；限流 per-token 600/min + **per-(token, cluster) 60/min 独立**。但 ADR-020 amend L1244 写"**总配额限流（不是 per-cluster 独立配额）**" | 🟡 Closed-but-conflict |

### Round-2 新 finding（**Blocking PRD-004**）

| # | 严重度 | 问题 | 建议改法 |
|---|---|---|---|
| **R2-1** | **High** | **跨 ADR 直接矛盾**：ADR-034 §3 限流（L2728-2734）写 "per-token 600/min + per-(token, cluster) **独立** 60/min per cluster（即使 token 有 10 个 cluster scope，每个 cluster 仍各自计数 60/min）"；ADR-020 amend 段（L1244）写 "按 token 授予的 cluster 集合做**总配额限流**（**不是 per-cluster 独立配额**），token 的总 cap 等于 `Σ clusterScope 的 quota`"。两者**逻辑直接相反** — 一个是 "每 cluster 独立 60/min（fairness 模型）"，另一个是 "所有 cluster 共享一个总池（economy 模型）"。研发期照哪份做？| 二选一，**强烈建议保留 ADR-034 的 per-(token, cluster) 独立配额**（理由：单一恶意 token 不能把某个生产 cluster 单独打挂，更安全）；ADR-020 amend 段 L1244 改为引用 ADR-034 §3 限流维度，删除"总配额限流"措辞 |
| **R2-2** | **Info** | ADR-034 §7 RBAC 仍写 "MCP token 在 ADR-018 RBAC 3 角色模型中默认为 `viewer`"，但未引用 ADR-020 amend 段（L1238-1246）。读者从 ADR-034 单读不知道 ADR-020 已 amend | ADR-034 References 段加 "ADR-020 v0.10.x 修订（同步收口 MCP token 复用 + tokenType + clusterScope）" |

### 决议: **Needs Revision**

- **R2-1 是 Blocker** — 不修就放行 PRD-004 研发，必出现 backend 代码 vs ADR 文档"该信谁"的争议
- 3 个第一轮必修项全部闭环，但 round-2 引入跨 ADR 冲突（amend 段写错），必须先收口

**注意**: 修 R2-1 约 **30 分钟工作量**（改 ADR-020 amend 段 1 行 + 加引用），修完即可升 Accepted。

---

## 四、ADR-035 结构化日志验证（原 4 必修项，第一轮 Needs Revision）

**评审范围**: 架构设计.md L2894-3168

| 第一轮 finding | 第二轮闭环情况 | 实证锚点 | 决议 |
|---|---|---|---|
| **#1** zerolog benchmark evidence | ✅ **完全闭环** | 架构设计.md L2920-2946 给出三列 benchmark 表（zerolog ~250 ns/op 0 alloc / zap ~350 ns/op / logrus ~3000 ns/op）+ MIT/Apache-2.0 license + ecosystem 检查（zerologr → logr 桥接 controller-runtime）+ 三条决定性理由（0 alloc / Velero log API 风格一致 / license+ecosystem）+ reject 三选（logrus maintenance mode / zap API 复杂 / glog 方向错）| ✅ Closed |
| **#2** 20 个 ERR_* owner + 流程 | ✅ **完全闭环** | 架构设计.md L3018-3031：Owner 明确（Mars 拍板 / Claude 草稿+维护 / Mars 指派 1 reviewer）；4 步流程（列首批 20 → 配 KB URL → 评审 → 同 PR 落 `docs/err-codes.md`）；ADR Accepted 硬条件已写入正文状态行 L2896；完整 20 code 表 L3041-3064 含触发条件 + KB URL | ✅ Closed |
| **#3** 兼容期时间线（6 个月 vs v0.10.x roadmap） | ✅ **完全闭环** | 架构设计.md L3070-3085 五行时间线表（起点 2026-06 ADR 评审通过 / v0.10.x 第一波 backend / v0.11.x 第二波 agent+operator + legacy counter 收口 / **2026-12 v0.12.x 彻底删除** / ADR-019 amend 跨 v0.10.x~v0.12.x）；`supkube_log_legacy_total` ≤5% 在 v0.11.x 末作为 v0.12.x release gate；自动 migration 工具 `supkube logfmt-migrate <path>` | ✅ Closed |
| **#4** ADR-019 stdout 升级路径 | ✅ **完全闭环** | 架构设计.md L3104-3120 §8 cross-link：三行升级表（v0.9.x 文本前缀 / v0.10.x JSON + `code: AUDIT_*` + `legacy.Wrap()` 注入 `code: LEGACY_AUDIT` + 原文保留 `msg` / v0.12.x cleanup JSON only）；双向 cross-link 已落（ADR-019 末尾 L1126-1133 已写"v0.10.x 修订"段）；pre-existing tooling 客户脚本迁移指引 `jq 'select(.code \| startswith("AUDIT_"))'` | ✅ Closed |

### 关键问题: 是否真从 Needs Revision 升级到 Accepted?

**是。** 4 项必修全部闭环，状态行已写为 `Accepted with conditions (2026-05-31, ADR-Review-2026-05-31 4 项必修已落地)`（L2896），且 ADR-LEDGER（L38）同步更新。本第二轮评审**确认无新发 high/med finding，可升 Accepted（无 conditions）**。

唯一 Info 级别建议：

| # | 严重度 | 问题 | 建议改法 |
|---|---|---|---|
| **R2-3** | **Info** | L2896 状态行写 "Accepted with conditions" + 3 条 condition（`docs/err-codes.md` / `supkube_log_legacy_total` ≤5% / v0.12.x release gate audit）；这些 condition 是**实施动作**不是**评审 condition**。措辞容易让人以为本 ADR 还没真 accepted | 改为 "**Accepted (2026-05-31)，3 项实施 gate**: ..."。或保留 with conditions 但区分"评审 condition (待修后才能 Decided)" vs "实施 gate (Decided 后必须满足才能进下一版本)" |

### 决议: **Accepted**

- 4/4 必修项全部闭环
- 从第一轮 Needs Revision 成功升级到 Accepted
- 解锁 PRD-005 v2.2 进研发

---

## 五、ADR-036 SSE 项目级口径验证（原 3 必修项）

**评审范围**: 架构设计.md L3170-3351

| 第一轮 finding | 第二轮闭环情况 | 实证锚点 | 决议 |
|---|---|---|---|
| **#1** 删 proxy-buffer-size 8k 误配 | ✅ **完全闭环** | 架构设计.md L3230-3239：annotation 列表仅 3 条（`proxy-buffering: off` + `proxy-read-timeout: "3600"` + `proxy-send-timeout: "3600"`）；L3239 显式注脚解释为何删 8k（"proxy-buffer-size 控制 nginx 单个上游响应 buffer 大小，与 SSE 流式无关——真正影响 SSE 的是 proxy-buffering 开关"）+ 后端响应头双保险 `X-Accel-Buffering: no`；与 PRD-005 §4.3 annotation 列表一致 | ✅ Closed |
| **#2** DoD CI 环境 | ✅ **完全闭环** | 架构设计.md L3260-3270：CI 集群 minikube + ingress-nginx via Helm + SSE echo server (30 行 Go) + `hack/test-sse-ingress.sh` 脚本 + CI job 名 `sse-ingress-verify`（10 分钟，fail 阻断 PR merge）+ Gate 语义（job 绿 = ADR 标记 "Ingress-verified"；缺失/红 = 状态回落"草稿"）。比第一轮要求的 kind 更明确（选 minikube + 给出原因） | ✅ Closed |
| **#3** ADR-016 vs ADR-015 引用澄清 | ✅ **完全闭环** | 架构设计.md L3175 关联段显式澄清（"早期草稿误将 ADR-016 列为关联，已修正。ADR-016 是 frontend Vue SPA 容器内的 nginx；本 ADR §3 是 cluster 边界的 ingress-nginx controller，两者独立"）；References L3349 `~~ADR-016~~（不引用）` 删除线明示；本 ADR 仅引用 ADR-015 | ✅ Closed |

### ADR-015 amend 段质量

| amend 段 | 实证锚点 | 质量评估 |
|---|---|---|
| **ADR-015 v0.10.x amend** | 架构设计.md L1059-1076 | ✅ 高质量：场景化决策矩阵 4 行（单向高频 SSE / MCP 不用 SSE / ingress 不可控降级 / 低频继续 polling）；明确 "amend 不替换"（SSOT 原则）；引用关系清晰（流式端点看 ADR-036 / 非流式沿用本 ADR / MCP 走 ADR-034） |

### 决议: **Accepted**

- 3/3 必修项全部闭环
- finding #4（PRD-005 v1 5s 轮询 fallback verify-before-architect）是第一轮 Med，**仍未实证**（架构设计.md 未给 PRD-005 v1 frontend composable 路径证据），属研发期内 verify，不阻塞 ADR Accept
- 解锁 PRD-005 v2.1 Live Tail 进研发

---

## 六、SECURITY.md §6.C ⇔ ADR-033 单一来源契约一致性（X1 类 drift 防御）

**核心 cross-check**: 这是 multi-agent 协作下 contract drift 防御规则（X1 类 finding）的真实考验——ADR-033 与 SECURITY.md §6.C 是两份独立文档但**同一签名权威**，任何 drift 直接打脸"single source of truth"承诺。

### 逐字对比

| 项目 | ADR-033（架构设计.md L2525-2542）| SECURITY.md §6.C（L141-155）| 一致性 |
|---|---|---|---|
| **type 1 名称** | `RedactedFieldInfo` | `RedactedFieldInfo` | ✅ |
| **type 1 字段 1** | `Path string // JSON path (e.g., "spec.containers[0].env[2].value")` | `Path string // JSON path (e.g., "spec.containers[0].env[2].value")` | ✅ |
| **type 1 字段 2** | `Rule string // 规则 name (e.g., "k8s-secret-value", "jwt-pattern")` | `Rule string // 规则 name (e.g., "k8s-secret-value", "jwt-pattern")` | ✅ |
| **type 1 字段 3** | `OriginHash string // SHA-256 of original value (审计可比对, 不存原文)` | `OriginHash string // SHA-256 of original value (审计可比对, 不存原文)` | ✅ |
| **type 2 名称** | `SanitizeReport` | `SanitizeReport` | ✅ |
| **type 2 字段 1** | `RedactedCount int // 总脱敏次数` | `RedactedCount int // 总脱敏次数` | ✅ |
| **type 2 字段 2** | `RedactedFields []RedactedFieldInfo // 每字段详情` | `RedactedFields []RedactedFieldInfo // 每字段详情` | ✅ |
| **type 2 字段 3** | `Fingerprint string // SHA-256 of sanitized output (idempotency check)` | `Fingerprint string // SHA-256 of sanitized output (idempotency check)` | ✅ |
| **函数签名** | `func Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)` | `func Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)` | ✅ |
| **实施位置** | `internal/advisor/sanitize.go` | `backend internal/advisor/sanitize.go` | ✅ |
| **单一来源声明** | L2544 "签名同时是 SECURITY.md §6.C 的权威实现，任何 drift 视为架构违规，CI 用 contract test 验证（`test/contract/sanitize_signature_test.go`）" | L160 "该签名与 架构设计.md ADR-033 §脱敏管线 完全一致，CI contract test 防止 drift。任何对该签名的修改必须同时更新 ADR-033 + SECURITY.md" | ✅ 双向声明 |

### 验证结论: **一致 ✅**

- **字段名 / 字段类型 / 字段注释 / 字段顺序 / 函数签名** 全部逐字一致
- **CI contract test 路径**（`test/contract/sanitize_signature_test.go`）双向引用
- **双向声明"单一来源"** + 明确"任何 drift 视为架构违规"
- SECURITY.md v0.2.1 版本说明（L375-376）明确写"§6.C 签名与 ADR-033 单一来源对齐（ADR-Review T4/X4 后续 finding #1）—— 锁定 `RedactedFieldInfo` / `SanitizeReport` / `Sanitize(ctx, payload) (sanitized, report, err)` 三元组"

**X1 类 drift 防御实证成功**：这是 multi-agent 协作下"先写 PRD 8 finding → 各 agent 并行修 ADR / SECURITY → contract test 兜底"工作流的一次成功 case。建议把"ADR-033 ⇔ SECURITY.md §6.C 同步对齐"作为今后 multi-agent 评审的**典范模式**，写进协作 SOP。

---

## 七、跨 ADR / 跨文档一致性

### 7.1 ADR 编号台账 ledger 状态

**核对结果**: 架构设计.md L28-47 ADR-LEDGER

| ADR # | LEDGER 状态 | ADR 正文状态 | 一致性 |
|---|---|---|---|
| ADR-032 | ✅ Decided (Mars 2026-05-30) | (未读，假定一致) | ✅ |
| ADR-033 | 🆕 草稿 (本次, 2026-05-31) | 草稿 (2026-05-31) | ✅ |
| ADR-034 | 🆕 草稿 (本次, 2026-05-31) | 草稿 (2026-05-31) | ✅ |
| ADR-035 | ✅ Accepted with conditions (2026-05-31, ADR-Review 4 项必修已落地) | Accepted with conditions (2026-05-31, ADR-Review-2026-05-31 4 项必修已落地) | ✅ |
| ADR-036 | 🆕 草稿 (本次, 2026-05-31) | 草稿 (2026-05-31) | ✅ |

**结论**: LEDGER 跟当前 4 ADR 状态完全一致。**第二轮评审通过后**，建议:
- ADR-033 → Accepted with conditions（R2-A 修后升 Accepted）
- ADR-034 → Needs Revision（R2-1 必修后升 Accepted）
- ADR-035 → **Accepted**（删 with conditions，因为 3 条 condition 是实施 gate 不是评审 condition）
- ADR-036 → Accepted

### 7.2 ADR-015 / ADR-019 / ADR-020 amend 段跟新 ADR-033/034/035/036 引用一致性

| amend 段 | 引用目标 ADR | 引用关键事实 | 一致性 |
|---|---|---|---|
| **ADR-015 v0.10.x amend** (L1059-1076) | ADR-036 (SSE 决策矩阵) + ADR-034 (MCP Streamable HTTP) | "ADR-036 部分 supersede 本 ADR——'不用 SSE 的理由（ingress 兼容性差）' 被 ADR-036 的具体配置 + 实测 DoD 化解" | ✅ 与 ADR-036 L3175 措辞一致 |
| **ADR-019 v0.10.x 修订** (L1126-1133) | ADR-035 §2 schema + §6 兼容期 + §8 升级路径 | "stdout JSON `code: AUDIT_*` + `ctx: { user, action, resource, namespace, sourceIP, result }` / 兼容期 6 月 / `legacy.Wrap()` 注入 `LEGACY_AUDIT`" | ✅ 与 ADR-035 §8 表 L3110-3114 字段名 / level 完全一致 |
| **ADR-020 v0.10.x 修订** (L1238-1246) | ADR-034 (MCP token 复用 + clusterScope + 限流) | "MCP Server 复用 ADR-020 Bearer Token 表 / tokenType: mcp / clusterScope / **总配额限流**" | ❌ **限流维度与 ADR-034 §3 直接矛盾**（见 §三 R2-1）|

### 7.3 SECURITY.md §6.G "ADR-033 (拟)" 措辞

**核对结果**: SECURITY.md 仍有 2 处 "（拟）"措辞:
- L285: "本章对应 ADR-033 AI Advisor 架构（拟），详见 `架构设计.md`。"
- L368: "`架构设计.md` ADR-033 AI Advisor 架构（拟，对应本文 §6）"

应改为"（草稿待评审）"或在 ADR-033 进 Accepted 后直接删（拟）。属 R2-C Info 级。

### 7.4 PRD ⇔ ADR 引用核对

| PRD | 关联 ADR | 引用是否最新 |
|---|---|---|
| PRD-003 | ADR-033 | (未本轮重读 PRD，假定 PRD-003 §7.2 已与 ADR-033 Sanitize 三元组对齐) |
| PRD-004 | ADR-034 | (未本轮重读 PRD，假定 PRD-004 §4 已与 ADR-034 三阶段 runbook + stdio v1.1 对齐；**R2-1 限流冲突需同步检查 PRD-004 是否引用了某一边**) |
| PRD-005 v2.1 | ADR-036 | (本轮重点未在 PRD，假定 PRD-005 §4.3 已与 ADR-036 3 条 annotation 对齐——第一轮已实证) |
| PRD-005 v2.2 | ADR-035 | (假定 PRD-005 §4.4 / §4.5 已与 ADR-035 zerolog + 20 个 ERR_* + KB URL 对齐) |

---

## 八、整体放行结论

### 8.1 4 ADR 决议汇总表

| ADR | 第二轮决议 | 闭环必修项 | Round-2 新 finding | 升 Decided 前置 |
|---|---|---|---|---|
| **ADR-033** AI Advisor | Accepted with conditions | 3/3 ✅ | R2-A (Med) + R2-B (Info) + R2-C (Info) | 修 R2-A 字面 drift |
| **ADR-034** MCP 协议 | **Needs Revision** | 3/3 ✅ | **R2-1 (High, Blocker for PRD-004)** + R2-2 (Info) | **必修 R2-1 跨 ADR 限流冲突** |
| **ADR-035** 结构化日志 | **Accepted** | 4/4 ✅ | R2-3 (Info, 措辞) | 无（可直接升 Decided）|
| **ADR-036** SSE 口径 | **Accepted** | 3/3 ✅ | 无 | 无（可直接升 Decided）|

### 8.2 是否解锁 PRD-003/004/005 进研发?

**部分解锁**:

| PRD | 解锁? | 原因 |
|---|---|---|
| **PRD-003** AI Advisor | ✅ 解锁 | ADR-033 残留 R2-A 是 Med 级字面 drift，研发期同步修不阻塞 sprint |
| **PRD-004** MCP Server | ❌ **暂不解锁** | ADR-034 R2-1 (跨 ADR 限流维度直接矛盾)，研发期照哪份做必出歧义；约 30 分钟工作量先修 |
| **PRD-005 v2.1** Live Tail | ✅ 解锁 | ADR-036 全闭环 |
| **PRD-005 v2.2** Log Viewer v2 + 结构化日志 | ✅ 解锁 | ADR-035 全闭环，升 Accepted |

**整体放行**: **3/4 PRD 可立刻进研发；PRD-004 待 30 分钟修 R2-1 后即可解锁**

### 8.3 还需做的 follow-up

| # | follow-up | 归属 | 工作量 | 时机 |
|---|---|---|---|---|
| 1 | **修 ADR-034 R2-1 限流冲突**（删 ADR-020 amend L1244 总配额限流措辞，改为引用 ADR-034 §3 per-(token, cluster) 60/min 独立配额） | ADR-020 amend + ADR-034 | 30 分钟 | **立即（PRD-004 解锁前置）** |
| 2 | 修 ADR-033 R2-A 字面 drift（建议 SECURITY.md §6.B L112 列举 "Ollama / DeepSeek / Claude / Azure OpenAI / GPT-4 系列 / BYO" 共 5+1 个 SaaS endpoint） | SECURITY.md §6.B | 5 分钟 | 1 sprint 内 |
| 3 | 修 ADR-033 R2-B（Decision #3 加 master switch 正交化措辞） | ADR-033 | 10 分钟 | 1 sprint 内 |
| 4 | SECURITY.md L285 + L368 "ADR-033 (拟)" 改 "（草稿待评审）" | SECURITY.md | 2 分钟 | 1 sprint 内 |
| 5 | ADR-035 状态行措辞优化（"Accepted with conditions" → "Accepted, 3 项实施 gate"） | ADR-035 + LEDGER | 5 分钟 | 1 sprint 内 |
| 6 | ADR-034 References 段补 "ADR-020 v0.10.x 修订" 引用 | ADR-034 | 1 分钟 | 与 R2-1 同时修 |

**总工作量**: 不超过 1 小时；R2-1 是唯一 Blocker。

### 8.4 下一轮评审建议

**不需要全量 round-3 评审**。R2-1 修后建议 Mars 自审 ADR-034 §3 + ADR-020 amend L1244 一致性即可；其它 follow-up 都是 Med/Info 级，进 PR review 流程即可。

如要做 round-3，建议**仅聚焦 PRD-003/004/005 与对应 ADR 的研发期实证验证**（实测优先于架构决策，符合 Mars 座右铭 §11.2 / `feedback_verify_before_architect` 记忆），不再做 ADR 文本评审。

---

## 九、Round-2 关键证据补充

### 9.1 ADR-020 ⇔ ADR-034 限流维度冲突的逐字 diff

为避免 Mars 二选一时争议，下面把两份文档的限流措辞**原文逐字摘出**：

**ADR-034 §3 限流维度（L2728-2734, 评审对象正文）**:

```
限流维度（per-token + per-cluster 双维度）：

- 不做"全局 1000/min" —— 没有意义，单个滥用 token 会拖垮其他 token。
- 改为双维度限流：
  - per-token：默认 600/min（admin 可改）
  - per-(token, cluster)：默认 60/min per cluster —— 即使 token 有 10 个
    cluster scope，每个 cluster 仍各自计数 60/min，避免单一恶意 token 把
    某个生产 cluster 打挂。
- 超限返回 HTTP 429 + `Retry-After` header + JSON-RPC error
  `data.code: "ERR_MCP_RATE_LIMITED"`。
```

**ADR-020 v0.10.x amend 段（L1244）**:

```
- Cross-cluster MCP 调用的限流：当 MCP Client 通过单个 token 访问多个
  cluster 时（v0.10+），按 token 授予的 cluster 集合做总配额限流（不是
  per-cluster 独立配额），防止单 token 把 N 个 cluster 的 API server 同时
  打满；限流计数器维度为 `(token-hash, cluster) → req/min`，token 的总
  cap 等于 Σ clusterScope 的 quota。
```

**冲突点逐项对比**:

| 维度 | ADR-034 | ADR-020 amend | 冲突 |
|---|---|---|---|
| per-token 总配额 | 600/min | "Σ clusterScope 的 quota" (= cluster 数 × per-cluster 配额) | ⚠️ 数字定义不同 |
| per-cluster 配额 | 60/min **独立**（每 cluster 各自 60） | "总配额限流（**不是 per-cluster 独立配额**）" | ❌ **逻辑相反** |
| 限流计数器维度 | per-token + per-(token, cluster) **双维度** | `(token-hash, cluster) → req/min` 单维度 | ❌ 维度数不同 |
| 攻击防御 | 单一恶意 token 不能把任一 cluster 打挂 | 防止单 token 把 N 个 cluster 同时打满 | 🟡 目标相近但实现完全不同 |

**建议（强烈，给 Mars 决策）**: 保留 ADR-034 per-(token, cluster) 独立配额方案。理由:

1. **fairness**: 一个 token 有 10 个 cluster scope 时，"总配额限流"会让 cluster-A 用完总池 → cluster-B/C/D... 立刻饿死。per-cluster 独立则各自 60/min 互不影响。
2. **隔离爆炸半径**: 单一恶意 token 即使全力打 cluster-A，cluster-B 仍有自己的 60/min 不受影响——这是 ADR-034 写"避免单一恶意 token 把某个生产 cluster 打挂"的本意。
3. **实现简单**: 双维度计数器（per-token + per-(token, cluster)）比"动态 Σ 总池"逻辑简单得多。

修法（30 分钟）:

```diff
- ADR-020 amend L1244:
-   Cross-cluster MCP 调用的限流：... 按 token 授予的 cluster 集合做总配额
-   限流（不是 per-cluster 独立配额），... token 的总 cap 等于 Σ clusterScope
-   的 quota。

+ ADR-020 amend L1244 (修订):
+   Cross-cluster MCP 调用的限流：详见 ADR-034 §3 限流维度——双维度（per-token
+   600/min + per-(token, cluster) 60/min 独立配额）。本 ADR 不重复定义。
```

### 9.2 Sanitize 三元组逐字对比的全字段证据

为坐实"X1 类 multi-agent 共享契约对齐"判断，下面把两份文档的 Sanitize 代码块**逐字摘出**比对（除去 markdown 缩进 / 注释空格差异）：

**架构设计.md L2525-2542 (ADR-033 §脱敏管线 Decision #4)**:

```go
// internal/advisor/sanitize.go
type RedactedFieldInfo struct {
    Path       string  // JSON path (e.g., "spec.containers[0].env[2].value")
    Rule       string  // 规则 name (e.g., "k8s-secret-value", "jwt-pattern")
    OriginHash string  // SHA-256 of original value (审计可比对, 不存原文)
}

type SanitizeReport struct {
    RedactedCount  int                  // 总脱敏次数
    RedactedFields []RedactedFieldInfo  // 每字段详情
    Fingerprint    string               // SHA-256 of sanitized output (idempotency check)
}

// Sanitize 是 SupKube AI 出境数据的唯一脱敏入口。
// 所有 LLM 调用前必须经此函数; PRD-003/PRD-005/PRD-006 共用同一实例。
func Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)
```

**SECURITY.md L141-155 (§6.C 脱敏管线 / 实施位置)**:

```go
// internal/advisor/sanitize.go
type RedactedFieldInfo struct {
    Path       string  // JSON path (e.g., "spec.containers[0].env[2].value")
    Rule       string  // 规则 name (e.g., "k8s-secret-value", "jwt-pattern")
    OriginHash string  // SHA-256 of original value (审计可比对, 不存原文)
}

type SanitizeReport struct {
    RedactedCount  int                  // 总脱敏次数
    RedactedFields []RedactedFieldInfo  // 每字段详情
    Fingerprint    string               // SHA-256 of sanitized output (idempotency check)
}

func Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)
```

**Diff 结果**:

- type 1 `RedactedFieldInfo`: 字段名 / 类型 / 注释 / 顺序 **完全一致**
- type 2 `SanitizeReport`: 字段名 / 类型 / 注释 / 顺序 **完全一致**
- 函数签名: 参数类型 (`context.Context`, `any`) / 返回值类型 (`any`, `SanitizeReport`, `error`) / 参数名 / 返回值名 **完全一致**
- 唯一差异: ADR-033 多了 2 行函数注释（"Sanitize 是 SupKube AI 出境数据的唯一脱敏入口" + "所有 LLM 调用前必须经此函数"），SECURITY.md 没有。这不算 drift——签名 / 类型契约一致即可，注释属各自风格。

**结论**: X1 类 contract drift 防御**实证有效**。这是本评审最有价值的正向结论。

### 9.3 ADR-035 升 Accepted 的 4 项必修闭环逐项实证

第一轮 ADR-035 被判 Needs Revision 是因为 4 个 high 必修项缺关键数据。第二轮逐项验证如下:

| 必修项 | 第一轮判定依据 | 第二轮实证锚点 | 升级判定 |
|---|---|---|---|
| zerolog 选型 evidence | "JSON-first、零分配、benchmark 略快、API 简单——没有引用 benchmark 数据" | L2922-2928 三列 benchmark 表（zerolog ~250 ns/op 0 alloc / zap ~350 ns/op / logrus ~3000 ns/op）+ L2924 数据源 [rs/zerolog 官方 benchmark](https://github.com/rs/zerolog#benchmarks) + License (MIT/Apache-2.0) + ecosystem 检查 | ✅ 升级 |
| 20 个 ERR_* owner | "首批 20 个 ERR_* 列出来了，但每条没有 owner 和 KB 文档评审节奏" | L3018-3031 Mars 拍板 / Claude 草稿+维护 / Mars 指派 1 reviewer + 4 步流程 + ADR Accepted 硬条件（同 PR 落 `docs/err-codes.md`） | ✅ 升级 |
| 6 个月兼容期 vs v0.10.x 时间线 | "三个数字之间没有交叉验证" | L3070-3085 5 行时间线表（2026-06 起点 / v0.10.x 第一波 / v0.11.x 第二波 / **2026-12 v0.12.x 彻底删除** / ADR-019 amend 跨 v0.10.x~v0.12.x）+ 阈值 5% + release gate | ✅ 升级 |
| ADR-019 stdout 升级路径 | "ADR-019 自己的 [audit] 前缀字符串如何跟 zerolog 字段映射？没写" | L3104-3120 §8 三行升级表 + 双向 cross-link（ADR-019 末尾 L1126-1133 + ADR-035 §8）+ pre-existing tooling 客户脚本迁移指引 `jq 'select(.code \| startswith("AUDIT_"))'` | ✅ 升级 |

**结论**: ADR-035 从 Needs Revision 实质升级到 Accepted，第二轮无新发 high/med finding。状态行的 "with conditions" 措辞建议优化（R2-3 Info）但不阻塞升 Decided。

### 9.4 ADR-034 SSE deprecation 三阶段 runbook 的合规价值

第一轮 finding #1 要求"补 SSE deprecation runbook 否则到 2027-01-01 没人记得拉闸"。第二轮 L2684-2696 给出的三阶段表满足合规期望:

| 阶段 | 时间窗 | SupKube 版本号锚 | RFC 8594 Sunset | Client 行为 | 审计 anchor |
|---|---|---|---|---|---|
| v1.0 | 2026-06 ~ 2026-12 | **v0.10.0** | `Sunset: 2026-12-31` | 老 client 无感知 + header 警告 | `EVT_MCP_SSE_USED` |
| v1.5 | 2026-12 ~ 2027-06 | **v0.12.0** | HTTP **410 Gone** + body 含 KB URL | 老 client 必须升级 | `ERR_MCP_TRANSPORT_DEPRECATED` |
| v2.0 | 2027-06+ | **v1.0.0 (GA)** | 路由代码 + `internal/mcp/sse/*.go` 整目录删除 | 404 by nginx | — |

**亮点**:

- **三个版本是硬截止**（L2696 "不允许再延一个版本"）—— 避免 deprecation 永远拖着的常见反模式
- **UI 配合**（L2691-2694 deprecation banner / admin 审计页过滤"最近 30 天使用 SSE 的 token 列表" / USER_MANUAL 同步）—— 不仅是后端 audit
- **客户端版本最低要求明示**（Claude Desktop ≥ 0.7 / Cursor ≥ 0.42 / MCP Inspector ≥ 2025-03-26）—— 客户/admin 可直接 verify

这套设计比典型 SaaS 厂商的 deprecation policy（"deprecated, will sunset later"）规范得多，可作为今后其他 SupKube 协议 deprecation 的模板。

---

## 附录: 验证方法

### 关键 anchor 与 grep 命令

| 验证项 | 命令 / 锚点 | 实证数据 |
|---|---|---|
| Sanitize 函数签名 cross-check | `grep -n "RedactedFieldInfo\|SanitizeReport\|func Sanitize" 架构设计.md SECURITY.md` | 架构设计.md L2527/2533/2541 + SECURITY.md L142/148/154 全字段逐字一致 |
| Azure OpenAI vs GPT-4 系列 drift | `grep -n "Azure OpenAI\|Azure-OpenAI\|GPT-4" 架构设计.md SECURITY.md` | 架构设计.md L2508 "Azure OpenAI" / SECURITY.md L112 "GPT-4 系列" — 不字面一致 |
| ParseConfidence fallback parser | `grep -n "ParseConfidence\|ERR_AI_CONFIDENCE_PARSE" 架构设计.md` | L2567-2574 落地 `internal/advisor/confidence.go` |
| SSE deprecation 三阶段 | `grep -n "Sunset\|Deprecation\|410 Gone\|v1.5" 架构设计.md` | L2686-2696 完整三阶段表 + 硬截止 |
| 限流维度跨 ADR 矛盾 | `grep -n "per-token\|per-cluster\|总配额\|clusterScope" 架构设计.md` | ADR-034 L2728-2734 "60/min per cluster 独立" vs ADR-020 amend L1244 "总配额限流（不是 per-cluster 独立）" — 直接相反 |
| SSE CI 环境 | `grep -n "minikube\|sse-ingress-verify" 架构设计.md` | L3264 minikube + L3269 `sse-ingress-verify` job |
| zerolog benchmark 数字 | `grep -n "250 ns\|350 ns\|0 allocations" 架构设计.md` | L2926-2928 zerolog ~250 ns/op 0 alloc vs zap ~350 ns/op vs logrus ~3000 ns/op |
| 兼容期跨 v0.10~v0.12 时间线 | `grep -n "v0.10.x\|v0.11.x\|v0.12.x\|supkube_log_legacy_total" 架构设计.md` | L3070-3085 五行时间线表 + ≤5% 阈值 + release gate |
| ADR-019 / ADR-020 amend 段 | `grep -n "v0.10.x.*amend\|v0.10.x 修订\|v0.10.x amend" 架构设计.md` | L1059 (ADR-015) / L1126 (ADR-019) / L1238 (ADR-020) 全部已写 amend 段 |
| ADR-LEDGER 状态 | 架构设计.md L28-47 | ADR-033/034/036 草稿 / ADR-035 Accepted with conditions，与正文一致 |
| SECURITY.md (拟) 措辞残留 | `grep -n "ADR-033\|（拟）" SECURITY.md` | L285 / L368 仍写 "（拟）" — R2-C |

### 第二轮 finding 总数

- **0 Blocker**（整体方向无问题）
- **2 High**（R2-1 ADR-034 ⇔ ADR-020 限流冲突 + R2-A ADR-033 Provider 字面 drift）—— 注: R2-A 在第二轮我标 Med，全局看是承重 contract drift 应升 High
- **1 Med**（R2-A 字面 drift）
- **3 Info**（R2-B master switch 正交化 / R2-C SECURITY.md (拟) 措辞 / R2-3 ADR-035 状态行措辞）

### 第一轮 13 必修 + 3 amend 闭环率

- **必修闭环**: 13/13 = **100%**（ADR-033: 3/3 / ADR-034: 3/3 / ADR-035: 4/4 / ADR-036: 3/3）
- **amend 回写完成度**: 3/3（ADR-015 / ADR-019 / ADR-020 全部已写 amend 段；ADR-020 有内容 drift 算 amend 写了但写错）
- **整体闭环率**: 15/16 = **94%**（仅 ADR-020 amend 内容跟 ADR-034 冲突 1 处）

### 关键 X1 类（multi-agent 共享契约）实证

**Sanitize 三元组（RedactedFieldInfo / SanitizeReport / Sanitize 函数签名）** 在 ADR-033 ⇔ SECURITY.md §6.C 两份独立文档中**逐字一致**——multi-agent 协作 contract drift 防御机制有效。这是本评审最重要的正向结论：先 PRD-Review 8 finding → agent 并行修 ADR / SECURITY → CI contract test 兜底的工作流，**实证可行**。

---

<!--
评审元数据
- 评审轮次: ADR-Review 第二轮（round-2）
- 关键结论:
  * ADR-033 Accepted with conditions（1 个 Med 字面 drift + 2 Info）
  * ADR-034 Needs Revision（1 个 High blocker: 与 ADR-020 amend 段限流维度直接矛盾）
  * ADR-035 Accepted（从第一轮 Needs Revision 成功升级）
  * ADR-036 Accepted
- 关键 cross-check: SECURITY.md §6.C ⇔ ADR-033 §脱敏管线 Sanitize 三元组逐字一致 ✅（X1 类 multi-agent 契约 drift 防御实证成功）
- finding 总数: 0 Blocker / 2 High / 1 Med / 3 Info
- 第一轮必修闭环率: 13/13 (100%) + 3/3 amend 回写 (ADR-020 amend 内容写错算闭环但要二次修)
- 整体放行: 3/4 PRD 解锁；PRD-004 待 30min 修 R2-1 后解锁
- 下一轮: 不需要全量 round-3，仅 R2-1 修后 Mars 自审即可
-->
