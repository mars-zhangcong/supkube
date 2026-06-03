/* ====================================================================
   SupKube 项目仪表板 — 数据层（与视图分离）
   --------------------------------------------------------------------
   单一来源纪律（见 ENGINEERING.md §2）：
   本文件是 dashboard 的数据，应尽量从源 MD 派生而非手抄。
   半自动同步：`node dashboard/gen-data.mjs` 会用 PRD.md 索引表 +
   架构设计.md ADR-LEDGER 重新生成 PRDS / ADRS 两段（最易漂移），
   其余（DECISIONS / WEEKLY / CUSTOMER_PAIN / TASKS / LINEAGE）人工维护。
   视图通过 window.SUPKUBE_DATA 读取，双击 index.html 即可打开（file://）。
==================================================================== */
window.SUPKUBE_DATA = (function () {

const META = {
  SNAPSHOT_DATE: "2026-06-01",
  CURRENT_VERSION: "v0.9.1.10-alpha",
  CURRENT_SUB: "alpha · chart 0.9.1-alpha.10 · appVersion 0.9.1.10-alpha",
};

/* ---- 战略/产品决策日志（非 ADR；ADR 见 架构设计.md）---- */
const DECISIONS = [
  { id: "D-12", date: "2026-05-30", text: "✅ ADR-031：5 层 3-2-1-1-0 模型 + 6 点韧性路径；本地快照层(非 Velero)技术无阻碍 → 转向 Kasten/Veeam 级定位。" },
  { id: "D-13", date: "2026-05-30", text: "✅ 建立 PRD.md：所有影响 UX 的 Feature 必须先过 PRD（状态机：草稿→排队评审→评审中→{改正中｜驳回｜已评审}→研发中→待验收→Shipped→归档）。" },
  { id: "D-14", date: "2026-05-30", text: "✅ AI 路线分三段：Phase A=AI Advisor inside SupKube（推荐型/非自治, PRD-003）+ Phase B=MCP Server（PRD-004）+ Phase C=选择性自治（v2.0+）。反对全自治。" },
  { id: "D-15", date: "2026-05-30", text: "✅ OpenClaw 定位：客户侧 LLM Agent 框架（类比 LangChain/Dify），非 SupKube 产品/组件 → MCP Skills 必须开源（Apache-2.0）。" },
  { id: "D-16", date: "2026-05-30", text: "🔴 P0 Demo Sprint 锁定（2 周）：先扫客户阻断 #79/#91/#87/#68/#84。" },
  { id: "D-17", date: "2026-05-31", text: "✅ Verify-before-ship 永久入 memory：build pass ≠ deploy 完成，必须 curl /status 验 buildStamp + 走完整 user journey。→ 已落 ENGINEERING.md Rule D。" },
  { id: "D-18", date: "2026-05-31", text: "✅ Rule A=PRD 先于代码 / Rule B=并行 Agent / Rule C=共享契约单一来源。→ 已落 ENGINEERING.md。" },
  { id: "D-19", date: "2026-05-31", text: "🎯 P0 Demo Sprint 完成 5/5（#87/#68/#91/#84/#79 + Quick Win v1.5）。" },
  { id: "D-20", date: "2026-05-31", text: "📋 PRD-Review 系列建立：8 跨切面 finding（T1-T4 + X1-X4）全部闭环。" },
  { id: "D-21", date: "2026-05-31", text: "🗂 ADR-LEDGER 单一编号来源建立（X2 撞号根治）。新增 ADR-033~036。" },
  { id: "D-22", date: "2026-05-31", text: "🔒 SECURITY.md v0.2 加 §6 AI 数据处理与出境治理（合规默认本地 Ollama + 脱敏单源 + 出境白名单 opt-in）。" },
  { id: "D-26", date: "2026-05-31", text: "✅ PRD-003~007 全部评审通过，共 7 个 PRD 已评审就绪可研发。" },
  { id: "D-27", date: "2026-06-01", text: "✅ AI Backup Advisor 3 轮讨论收敛：finding #2 分数规则引擎算（可复现）+ finding #3 置信度三档不用百分比 + 缺失数据 status 枚举显式建模 + 采用 SupEye 四段 DSL（fact/observation/inferred/evidence）。立 PRD-011/012 + ADR-037。" },
  { id: "D-28", date: "2026-06-01", text: "📚 文档体系补强：建 术语表.md（Glossary 单源，收口 Resilience Score vs Posture / Layer 1-5 vs L1-L2 撞车）+ CHANGELOG.md + ENGINEERING.md（Rule A/B/C/D 落仓库）。Dashboard 重构：data.js 与视图分离 + 数据刷新到当日真相。" },
  { id: "D-29", date: "2026-06-01", text: "✅ PRD-009 v2 Ship：Action Type pill（Snapshot Policy / Import Policy）+ ImportPolicy CRD + fingerprint HMAC（Writer/Validator/TrustStore）+ 直 S3 BackupLister。7-agent 并发交付。双集群 v0.9.1.10-alpha-dev-06010819 ship。任务 #88 关闭。教训 L-10/L-11/L-12 落 MEMORY.md。" },
  { id: "D-30", date: "2026-06-01", text: "🔄 ADR-037/038 让号治理：PRD-008→ADR-039（审计存储）/ PRD-010→ADR-040（DR Topology SVG）。台账加 \"被复用过的旧号不得沿用\" 规则。出处：PRD-Review 第六份跨文档 finding。" },
  { id: "D-31", date: "2026-06-01", text: "📚 项目级 MEMORY.md ship（363 行）：交互节奏 + Rule A-F 反例 + 13 条跨 session 教训速查 + 基础设施速查 + 维护约定。从 AI 个人 memory 迁入仓库，方便接力人 Review。" },
  { id: "D-32", date: "2026-06-01", text: "🗂 Rule G + LEDGER.md 立：项目所有文档编号（PRD/ADR/TC/D/C）统一台账，取号 4 步 SOP（read→reserve→write→改状态），并发由 main agent 集中预分配号（Rule C v2 延伸），让号 forward-only，漂移检查 gen-data.mjs 兜底。落 ENGINEERING.md Rule G + MEMORY.md L-14 + ADR-041。本条由 Rule G 4 步演练首次占的 D 号（与 ADR-041 同批演练）。" },
  { id: "D-33", date: "2026-06-01", text: "☁️ 开发主体环境上云 (ADR-042)：关闭本机 docker-desktop，aks-jumborca-dev=研发 (push to main 自动)、aks-jumborca-test=测试 (tag 自动)、aks-jumborca-prod=未来生产 (manual approval + cluster guard)。dev-deploy.sh 头加 DEPRECATED notice，v0.9.1.15 真删除。保留 amd64+arm64 双架构。GitHub 仓库暂保持 public（追 Demo 闭环优先于商业闭源）。Rule G 第 2 次演练取号 ADR-042 + D-33 成功，4 步 SOP 流畅。" },
  { id: "D-34", date: "2026-06-02", text: "📋 PRD-Review 第六份 finding 全 ship (Auto 5h 自主工作): PRD-009 v2 §8.2 Phase 2 DoD 14 条 + §9 Phase 2 任务拆 7 阶段 + §9.3 风险评级 (G5 闭环) / PRD-008 D1-D5 修订段 §13 + §8 DoD #19-#23 (D1 嵌入式 store on PV / D2 hash-chain+admission+WORM 3 层防御 / D3 BSL 归档 + verify-archive / D4 deletionState 从 task store 派生 / D5 Kopia maintenance 而非裸 mc rm) / PRD-010 F1-F4 §13 + §8 DoD #13-#16 (F1 消费 PRD-007 单一 score / F2 flows[].type 5 类 / F3 L1-L5 ↔ Snapshot/Export 映射 / F4 Layer 5 改验证徽章) / PRD-011 H1/H2/H5 §12 + §8 DoD #14-#17 (规则集版本化 scoreRulesVersion + region/provider 异地判定 + 拆 /ai/score 同步 /ai/explain SSE 异步) / PRD-012 I1 §10 + §8 DoD #13-#15 (默认逐次确认 + Call Home payload 并入 SECURITY §6 白名单, I2 仍 Blocked 等 Case API)。**4 PRD 状态 草稿→改正中**, 可转研发中。**待 Mars 拍**: D-WAIT-001 (Azure dev OIDC federated credential) + D-WAIT-002 (PRD-011 Q1 评分权重 + Q4 硬阈值数值)。" },
];

/* ---- PRD 状态（应与 PRD.md 索引表一致）---- */
const PRDS = [
  { id: "PRD-001", title: "跨集群还原前置检查闭环（Restore Preflight Checklist）", task: "#104", state: "研发中", updated: "2026-06-03", note: "Mars D-WAIT-003 拍板进研发; 4 finding + T3 拓扑校验已修订完成 (§4 + §8 DoD #6/#7/#10)" },
  { id: "PRD-002", title: "Transform 一等公民（两层模型）", task: "#114", state: "已评审", updated: "2026-05-31", note: "v1.3 闭环：T1 编译契约 + CAS storm DoD" },
  { id: "PRD-003", title: "AI Advisor inside SupKube（推荐型 · 非自治）", task: "#115", state: "已评审", updated: "2026-05-31", note: "可进研发" },
  { id: "PRD-004", title: "MCP Server 'Supkube Skills'（Streamable HTTP + 5 Skills + 开源）", task: "#116", state: "已评审", updated: "2026-05-31", note: "可进研发" },
  { id: "PRD-005", title: "Log Viewer v2（运维级日志观察平台）", task: "#118", state: "已评审", updated: "2026-05-31", note: "可进研发" },
  { id: "PRD-006", title: "Activity Task Detail Timeline", task: "#117", state: "已评审", updated: "2026-05-31", note: "可进研发" },
  { id: "PRD-007", title: "完整 3-2-1-1-0 数据韧性（5 层 + Layer 4 Copy + DR Drill）", task: "#126", state: "已评审", updated: "2026-05-31", note: "P1-P5 全闭环（真 fixture 双重证实）" },
  { id: "PRD-008", title: "RP 删除生命周期 + Activity 持久化 + Force Delete 治理", task: "#148", state: "研发中", updated: "2026-06-03", note: "D1-D5 finding 全闭环 (§13 修订段 + §8 DoD #19-#23), 可转研发中" },
  { id: "PRD-009", title: "Policy 模型对齐 Kasten（Snapshot + Import Policy 双 Action）", task: "#149", state: "研发中", updated: "2026-06-03", note: "Phase 1 ship + Phase 2 任务拆；PRD-Review 第六份 G1-G5 全闭环 (G5 §8.2 Phase 2 DoD + §9 任务/§9.3 风险 / G1 卖点诚实化 / G2 backupSyncPeriod 默认 60s / G3 warn 半成品标注 / G4 Action Type pill save 不可改 alert), 等 Mars 重审" },
  { id: "PRD-010", title: "DR Topology v2（可视化重构 + Local Snapshot/Backup Copy 节点）", task: "#150", state: "研发中", updated: "2026-06-03", note: "F1-F4 finding 全闭环 (§13 修订段 + §8 DoD #13-#16), 可转研发中" },
  { id: "PRD-011", title: "AI Backup Advisor MVP（规则算分 + LLM 解释 · Canonical DSL · 本地小闭环）", task: "#164", state: "研发中", updated: "2026-06-03", note: "H1/H2/H5 已修订 (§12 + §8 DoD #14-#17); H1 数值待 Mars 拍 (D-WAIT-002)" },
  { id: "PRD-012", title: "Call Home / Auto-Support（三档连接 · 自动开 Case · opt-in）", task: "#165", state: "改正中", updated: "2026-06-02", note: "I1 已闭环 (默认逐次确认+SECURITY §6 白名单); I2 仍 Blocked 等 Case API" },
  { id: "PRD-013", title: "SupKube Four-Eyes Authorization（备份安全二次审批 + MFA · 4 大类 15 受保护操作 · ApprovalPolicy/ApprovalRequest CRD · Veeam VBR 13 对标 + Kasten 没有 = 真差异化）", task: "TBD（取号待 Mars 批后建 task）", state: "草稿", updated: "2026-06-02", note: "Mars D-WAIT-002 frame shift 派生立项: PRD-011 §6 维度 3 / MFA+二次审批 10 分平台落地 dependency. 不卡 Demo 核心闭环 (Mars 决策延后)." },
  { id: "PRD-014", title: "前端 UI 暴露模型（运维方 Day-0 可配 · 4 模式 LoadBalancer/NodePort/ClusterIP+port-forward/Ingress · 镜像 Dex publicURL 范本 · 装后 NOTES.txt 按模式打印访问方式）", task: "TBD（取号待建 task）", state: "草稿", updated: "2026-06-02", note: "" },
  { id: "PRD-015", title: "AI 容灾决策顾问（AI DR Decision Advisor · Premium 独占）：标准基本盘 A + 客户决策面 B 两层体系（ADR-046）+ 决策历史库 + 盲区检测 + DRP/CRP 编排 + 风险框架工具箱 · 从 PRD-011 MVP 拆出的 Premium 上层", task: "TBD", state: "草稿", updated: "2026-06-03", note: "" },
];

/* ---- ADR 台账（应与 架构设计.md ADR-LEDGER 一致）---- */
const ADRS = [
  { id: "ADR-030", title: "还原扫描双引擎（yara-x + ClamAV）", date: "", status: "已评审", note: "PRD-009 系 / v0.9.6" },
  { id: "ADR-031", title: "5 层 3-2-1-1-0 数据韧性模型（纠 ADR-028 误诊）", date: "", status: "已评审", note: "→ PRD-007" },
  { id: "ADR-032", title: "Operator 评估（v1 SupKube + Velero 共存）", date: "2026-05-30", status: "已评审", note: "PRD-003" },
  { id: "ADR-033", title: "AI Advisor 架构（Engine 共享 + 脱敏 + 出境治理 + 非自治）", date: "2026-05-31", status: "草稿", note: "PRD-003 + PRD-004 共享" },
  { id: "ADR-034", title: "MCP 协议选型 = Streamable HTTP（非 SSE）", date: "2026-05-31", status: "草稿", note: "PRD-004 · 对齐 ADR-015" },
  { id: "ADR-035", title: "结构化日志 + ERR_* 错误代码体系", date: "2026-05-31", status: "已评审", note: "PRD-005 v2.2 · Accepted w/ conditions" },
  { id: "ADR-036", title: "SSE 项目级口径 + nginx ingress 配置", date: "2026-05-31", status: "草稿", note: "PRD-005 Live Tail" },
  { id: "ADR-037", title: "统一数据采集架构（CollectionContract + Collector/Server 分离 + Canonical DSL + 三档连接）", date: "2026-06-01", status: "草稿", note: "🆕 PRD-011 / PRD-012" },
  { id: "ADR-038", title: "ImportPolicy CRD + Controller（替代 backupSyncPeriod 60s 兜底）", date: "2026-06-01", status: "草稿", note: "🆕 PRD-009 v2 + #88/#157-163" },
  { id: "ADR-039", title: "Activity 持久化与 audit event 存储选型（让号自 037）", date: "2026-06-01", status: "草稿", note: "🔄 PRD-008 让号 · PRD-Review 第六份 §二（占号待 PRD-008 Phase 0 落地）" },
  { id: "ADR-040", title: "DR Topology SVG 视觉规范（让号自 038）", date: "2026-06-01", status: "草稿", note: "🔄 PRD-010 让号 · PRD-Review 第六份 §二（占号待 PRD-010 实施落地）" },
  { id: "ADR-041", title: "项目编号统一台账 (LEDGER.md) + Rule G 取号 SOP", date: "2026-06-01", status: "草稿", note: "🆕 Rule G 首次演练取的号 · 跨 PRD/ADR/TC/D/C 全 series 防撞号 · 并发由 main agent 集中预分配" },
  { id: "ADR-042", title: "开发主体环境上云 (Azure AKS) + CI/CD 三集群推送策略", date: "2026-06-01", status: "草稿", note: "🆕 关 docker-desktop · push to main → aks-dev / tag → aks-test / manual → aks-prod · dev-deploy.sh 退役 · arm64 保留 · GitHub 暂 public" },
  { id: "ADR-043", title: "AI Backup Advisor 评分细则 v1.0.0（100 分制 · 4 维 · 标准对标：备份覆盖与合规 25 / 3-2-1-1-0 韧性 35 / 防勒索与安全 20 / 成功率与可恢复性 20；对标 ISO 27002 §8.13 / NIST CSF / NIST SP 1800-26 / NIST SP 800-53 CP-9；确定性纯函数 `Score(ctx)→ScoreResult` 带 `scoreRulesVersion`，实现于 `internal/advisor/evaluator/v1_0_0.go`；3 落地公式 + 安全级别 4 档 + Q4 两条校准硬阈值；分数由 Go 规则引擎算，LLM 绝不参与打分只解释）", date: "", status: "草稿", note: "PRD-011 §4.2 / Mars D-WAIT-002 2026-06-03" },
  { id: "ADR-046", title: "AI 容灾决策两层正交体系（标准基本盘 A + 客户决策面 B）：A=评分+盲区检测权威（永不被覆盖，跨客户可横比）；B=DRP/CRP 执行权威（AI 枚举待决策→暴露差异→客户终审签字→决策历史库→唯一执行准则）。\\\"从权 B>A\\\" 仅指执行层、非评分层（两层正交，故无跨客户不可横比问题）。闭环=标准兜底/AI 引导/客户终审/系统落地/全程可追溯；非自治 Rule F 不变。完整能力（决策库+盲区检测+DRP/CRP 编排+风险框架工具箱）= Premium 独占，超 PRD-011 MVP → 建议立 PRD-015", date: "", status: "草稿", note: "PRD-011 §4.4 / Mars 决策 2026-06-03" },
  { id: "ADR-044", title: "快速调试模式 (Fast Debug Mode)：本地秒级内循环 `hack/dev-local.sh`（Vite HMR + go run/port-forward 二模式，绕过 docker build/ACR/AKS）+ feature 分支推送节奏（每 2h/改动大 1h push，仅触发 CI 不触发部署）；与 ADR-042 云端通道互补", date: "", status: "草稿", note: "全项目 / Rule G 取号 / Mars 2026-06-02（补登台账元数据 2026-06-03：号源 LEDGER.md 已有，此前漏镜像进本表）" },
  { id: "ADR-045", title: "ApprovalPolicy/ApprovalRequest CRD 设计 + Dex MFA 集成模式 ← 让号自 ADR-044（PRD-013 原非正式预占 ADR-044，与快速调试模式撞号，按 Rule G §C forward-only 让号至下个空号）", date: "", status: "已评审", note: "PRD-013 让号 / 2026-06-02（补登台账元数据 2026-06-03）" },
];

/* ---- 本周行动 ---- */
const WEEKLY = [
  { p: "P0", text: "PRD-011 评审：拍板 Q1 评分维度配比（30/20/30/15/5）+ Q4 两条硬阈值（无备份封顶 30 / 高分校准 30）", due: "本周" },
  { p: "P0", text: "PRD-012 解 Blocker：提供公司售后系统 Case API 规格（认证 / 字段 / 回执）", due: "本周" },
  { p: "P1", text: "PRD-009 Phase 2：ImportPolicy CRD + controller（ADR-038）收尾", due: "本周" },
  { p: "P1", text: "PRD-001 研发中 — Mars 2026-06-03 D-WAIT-003 拍板进研发 (T3 拓扑校验 + 4 finding 已落 §4 + §8 DoD)", due: "已闭环" },
  { p: "P1", text: "重启 #104 CSI 一键适配 sprint", due: "下周" },
  { p: "P1", text: "PRD-008 / PRD-010 finding 仍 open，排期改正", due: "本月" },
  { p: "P2", text: "最小化 GitHub Actions CI（build + lint），把 verify-before-ship 自动化", due: "月底" },
];

/* ---- 客户痛点（2026-05-28 demo 现场）---- */
const CUSTOMER_PAIN = [
  { id: "C-001", date: "2026-05-28", quote: "没有看到 Import 的标签", link: "imported chip 后端 schedule 检测", status: "✅ 0.9.1.6 已修" },
  { id: "C-002", date: "2026-05-28", quote: "没有了以前的一键适配检查...Storage Class 已经不一样了", link: "CSI 一键适配", status: "task #104 ⭐" },
  { id: "C-003", date: "2026-05-28", quote: "没有创建新的 import 还原点的地方...借鉴 Kasten", link: "Export/Import 指纹模型", status: "task #88" },
  { id: "C-004", date: "2026-05-28", quote: "点击不了 Restore", link: "Restore button 无 tooltip", status: "✅ 0.9.1.5 已修" },
  { id: "C-005", date: "2026-05-28", quote: "我们的客户也不想脱离这个页面", link: "跨集群自动 SC mapping", status: "task #104" },
  { id: "C-006", date: "2026-05-28", quote: "KubeVirt VM 备份能力", link: "新需求", status: "v0.9.8 / task #93" },
  { id: "C-007", date: "2026-05-28", quote: "还原时安全扫描", link: "新需求", status: "v0.9.6 / task #92 (YARA + ClamAV)" },
  { id: "C-008", date: "2026-05-28", quote: "Activity 点开没有详细步骤与用时（对标 Arcami）", link: "Action Details 时间线 + 耗时", status: "PRD-006 / task #117" },
  { id: "C-009", date: "2026-05-28", quote: "Data Usage Report 没有，Dashboard 也没展现（对标 Kasten）", link: "Prometheus + Dashboard 卡 + Dedup 率", status: "task #97" },
  { id: "C-010", date: "2026-05-28", quote: "Multi-Cluster 下拉切换后 Dashboard 不变化", link: "currentClusterId 不联动", status: "task #98" },
  { id: "C-011", date: "2026-05-28", quote: "还原一直卡在 Restoring，Demo 卡住了", link: "node-agent data path hang", status: "✅ #102 根治（velero v1.18）" },
  { id: "C-012", date: "2026-05-28", quote: "3 Error 什么也看不到", link: "Restore 详细错误 + warning 解读", status: "PRD-005 / task #103" },
  { id: "C-013", date: "2026-05-28", quote: "再一次定义 Restore 什么都没发生，没有新 Task", link: "发起 restore 前端零反馈", status: "task #105" },
  { id: "C-014", date: "2026-05-28", quote: "Azure 云上 This Cluster 对客户一点意义也没有", link: "改用集群名替代 This Cluster", status: "✅ PRD-010 / clusters.go 已改" },
  { id: "C-015", date: "2026-05-28", quote: "Log Viewer 让客户看到不同组件的报错点", link: "Log Viewer 组件分类 + error 高亮", status: "PRD-005 / task #103" },
  { id: "C-016", date: "2026-05-28", quote: "应用点击 Backup 应跳 Policy 而非 Restore Point", link: "应用列表导航语义", status: "task #108" },
  { id: "C-017", date: "2026-05-28", quote: "No VolumeSnapshotClass 应有一键适配，并显示用到哪个 Transform Set", link: "CSI 一键适配透明化", status: "task #104 ⭐" },
  { id: "C-018", date: "2026-05-28", quote: "应该有 Restore Advisor — 还原成功率多少？成本（时间/云费用）", link: "还原成功率 + 成本评分", status: "PRD-003 / task #106" },
];

/* ---- 需求血缘（客户痛点/动机 → 任务 → PRD → ADR → 测试）---- */
const LINEAGE = [
  { motive: "C-002 / C-017", task: "#104", prd: "PRD-001 / PRD-002", adr: "ADR-003", tc: "TC-STG / TC-RST", status: "进行中", note: "CSI 一键适配（杀手级）" },
  { motive: "C-008", task: "#117", prd: "PRD-006", adr: "ADR-036", tc: "TC-ACT-001~008", status: "已评审", note: "Activity 时间线 + 耗时" },
  { motive: "C-012 / C-015", task: "#103", prd: "PRD-005", adr: "ADR-035 / ADR-036", tc: "TC-SEC / TC-UX", status: "已评审", note: "Log Viewer + 错误可见" },
  { motive: "C-018", task: "#106", prd: "PRD-003 / PRD-011", adr: "ADR-033 / ADR-037", tc: "TC-AI-001~014", status: "草稿", note: "Resilience Score（规则算分+解释）" },
  { motive: "AI 业务梳理/备份建议", task: "#164", prd: "PRD-011", adr: "ADR-037", tc: "TC-AI-MVP（15 例）", status: "草稿", note: "本地分析小闭环（finding #2/#3）" },
  { motive: "SaaS/AirGap 易调试 + 自动开 Case", task: "#165", prd: "PRD-012", adr: "ADR-037 / ADR-033", tc: "TC-AI-015", status: "Blocked", note: "Call Home，待 Case API 规格" },
  { motive: "3-2-1-1-0 完整韧性", task: "#126", prd: "PRD-007", adr: "ADR-031", tc: "TC-RP / TC-STG", status: "已评审", note: "5 层 + Layer 4 + DR Drill" },
  { motive: "C-003", task: "#88", prd: "PRD-009", adr: "ADR-038", tc: "TC-IMP-001~004", status: "研发中", note: "Import Policy 指纹模型" },
];

/* ---- 任务（无结构化源，人工维护）---- */
const TASKS = [
  { id: "P0-1", version: "v0.9.1.3", title: "Velero 真自带（CRD + plugin + 镜像入 ACR）", status: "done", priority: "P0", category: "Infra", estimate_days: 3, owner: "claude", start_week: 23, end_week: 24, discovered: "2026-05-28", tags: ["customer-pain","velero-bundle"] },
  { id: "P0-6", version: "v0.9.1.5", title: "Multi-Cluster 下拉切换不联动 Dashboard", status: "done", priority: "P0", category: "Bug", estimate_days: 1.5, owner: "claude", start_week: 24, end_week: 24, discovered: "2026-05-28", tags: ["customer-pain","task-98","C-010"] },

  { id: "P1-4", version: "v0.9.1.13", title: "Kasten 风格 Export/Import 指纹模型（PRD-009 v2 Phase 2 + ImportPolicy CRD + fingerprint HMAC + 直 S3 polling）", status: "done", priority: "P0", category: "Backend", estimate_days: 5, owner: "claude", start_week: 24, end_week: 25, discovered: "2026-05-28", tags: ["task-88","customer-pain","C-003","PRD-009","shipped-2026-06-01"] },

  // ── 2026-06-01 dashboard 刷新: ROADMAP P0 表里 3 条缺位补上 (drift 修正) ──
  { id: "T-63", version: "v0.9.2", title: "License Manager 前端（1:1 复刻 Kasten Licenses 页 + mock backend）", status: "todo", priority: "P0", category: "Frontend", estimate_days: 3, owner: "tbd", start_week: 24, end_week: 25, discovered: "2026-05-24", tags: ["task-63","商业闭环","PV5-CR5-ME4=14"] },
  { id: "T-86", version: "v0.9.1.5", title: "存储管理 tab（快照位置迁入集群管理；BSL 保持主菜单）", status: "todo", priority: "P0", category: "Frontend", estimate_days: 1, owner: "tbd", start_week: 24, end_week: 24, discovered: "2026-05-28", tags: ["task-86","customer-pain","PV4-CR5-ME4=13"] },
  { id: "T-114", version: "v0.9.x", title: "PRD-002 Transform 一等公民 v1.3（改正中 → 研发：T1 编译契约 + CAS storm DoD #18 + Event 流统计）", status: "in_progress", priority: "P0", category: "Backend", estimate_days: 7, owner: "claude", start_week: 25, end_week: 26, discovered: "2026-05-31", tags: ["task-114","PRD-002","PV5-CR4-ME3=12"] },

  { id: "P2-1", version: "v1.0-GA", title: "GitHub Actions CI（build + lint + test）", status: "todo", priority: "P2", category: "Infra", estimate_days: 3, owner: "tbd", start_week: 28, end_week: 28, discovered: "2026-05-28", tags: ["ci","tech-debt"] },
  { id: "P2-2", version: "v1.0-GA", title: "E2E 测试自动化（TC-* 测试用例）", status: "todo", priority: "P2", category: "Infra", estimate_days: 5, owner: "tbd", start_week: 29, end_week: 30, discovered: "2026-05-28", tags: ["e2e","tech-debt"] },
  { id: "P2-4", version: "v0.9.x", title: "Wiki.js 文档站（USER_MANUAL 拆分）", status: "todo", priority: "P2", category: "Docs", estimate_days: 2, owner: "mars", start_week: 26, end_week: 27, discovered: "2026-05-28", tags: ["wiki","tech-debt"] },

  { id: "T-104", version: "v0.9.x", title: "★ CSI 一键适配（自动配 SC+VSC+binding mode + 显示 Transform Set）", status: "todo", priority: "P0", category: "Feature", estimate_days: 5, owner: "claude", start_week: 26, end_week: 27, discovered: "2026-05-28", tags: ["task-104","customer-pain","killer-feature","C-002","C-017","PRD-001","PRD-002"] },
  { id: "T-106", version: "v0.9.x", title: "还原任务中心 + Restore Advisor（成功率/成本评分）", status: "todo", priority: "P1", category: "Feature", estimate_days: 8, owner: "tbd", start_week: 28, end_week: 29, discovered: "2026-05-28", tags: ["task-106","differentiator","C-018","PRD-003"] },

  // ── 2026-05-31 P0 Demo Sprint 收尾 + 文档大块 ──
  { id: "S-79", version: "v0.9.1.9", title: "Log Viewer v1.5（错误摘要卡 + Prev/Next Error + Expand ±5 + Live Tail Lock + 荧光 + 全屏）", status: "done", priority: "P0", category: "Feature", estimate_days: 7, owner: "claude", start_week: 22, end_week: 23, discovered: "2026-05-24", tags: ["P0-demo","shipped"] },
  { id: "S-84", version: "v0.9.1.9", title: "Velero bundled + 镜像入 ACR + CSI 自动", status: "done", priority: "P0", category: "Infra", estimate_days: 3, owner: "claude", start_week: 23, end_week: 24, discovered: "2026-05-28", tags: ["P0-demo","shipped"] },
  { id: "T-87", version: "v0.9.1.10", title: "BSL Secret Key 字段澄清", status: "done", priority: "P0", category: "Frontend", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-05-31", tags: ["P0-demo","customer-pain","task-87"] },
  { id: "T-68", version: "v0.9.1.10", title: "Force-delete 卡住的 Backup CR", status: "done", priority: "P0", category: "Backend", estimate_days: 1, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-05-28", tags: ["P0-demo","customer-pain","task-68"] },
  { id: "T-102", version: "v0.9.1.9", title: "node-agent hang 根治（velero v1.16 → v1.18 固化）", status: "done", priority: "P0", category: "Backend", estimate_days: 2, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-05-28", tags: ["customer-pain","C-011","task-102"] },

  // ── 文档/PRD/ADR 产出 ──
  { id: "T-PRD007", version: "PRD-007", title: "PRD-007 3-2-1-1-0 完整数据韧性 + 评审 P1-P5 闭环", status: "done", priority: "P1", category: "Docs", estimate_days: 1.5, owner: "claude", start_week: 22, end_week: 22, discovered: "2026-05-31", tags: ["PRD","data-resilience"] },
  { id: "T-122", version: "ADR", title: "ADR-033~036（AI Advisor / MCP / 日志 / SSE 口径）", status: "done", priority: "P1", category: "Docs", estimate_days: 1.5, owner: "claude", start_week: 22, end_week: 22, discovered: "2026-05-31", tags: ["ADR","ADR-LEDGER"] },
  { id: "T-121", version: "SECURITY.md", title: "SECURITY.md §6 AI 数据处理与出境治理", status: "done", priority: "P1", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 22, end_week: 22, discovered: "2026-05-31", tags: ["security","compliance"] },

  // ── 2026-06-01 当日产出 ──
  { id: "T-PRD011", version: "PRD-011", title: "PRD-011 AI Backup Advisor MVP（规则算分 + LLM 解释 + 双面知识库）", status: "in_review", priority: "P1", category: "Docs", estimate_days: 1, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["PRD","AI","finding-2-3","blocked-on-review"] },
  { id: "T-PRD012", version: "PRD-012", title: "PRD-012 Call Home / Auto-Support（复用 Collector + DSL + 自动开 Case）", status: "todo", priority: "P1", category: "Docs", estimate_days: 1, owner: "claude", start_week: 23, end_week: 24, discovered: "2026-06-01", tags: ["PRD","Call-Home"], blocked: true, blockReason: "待 Mars 提供售后系统 Case API 规格" },
  { id: "T-ADR037", version: "ADR", title: "ADR-037 统一数据采集架构（CollectionContract + Canonical DSL + 三档连接）", status: "done", priority: "P1", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["ADR","ADR-LEDGER"] },
  { id: "T-TCAI", version: "测试用例", title: "测试用例 §19 TC-AI-MVP（15 例，覆盖 PRD-011 13 条 DoD）", status: "done", priority: "P1", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["test","AI"] },
  { id: "T-GLOSS", version: "术语表", title: "术语表.md（Glossary 单源，收口 Score/Layer/Snapshot 撞车）", status: "done", priority: "P1", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["docs","single-source"] },
  { id: "T-CHLOG", version: "CHANGELOG", title: "CHANGELOG.md（Keep-a-Changelog，澄清双轨版本号）", status: "done", priority: "P2", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["docs"] },
  { id: "T-ENG", version: "ENGINEERING", title: "ENGINEERING.md（Rule A/B/C/D 落仓库 + 文档地图）", status: "done", priority: "P1", category: "Docs", estimate_days: 0.5, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["docs","process"] },
  { id: "T-DASH", version: "tools", title: "Dashboard 重构：data.js 与视图分离 + 数据刷新 + 血缘/深链/Blocked/趋势", status: "in_progress", priority: "P2", category: "Docs", estimate_days: 1, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["devops","dashboard"] },
  { id: "T-PRD009-2", version: "v0.9.1.13", title: "PRD-009 v2 Phase 2：ImportPolicy CRD + controller + fingerprint HMAC + 直 S3 polling（ADR-038, 7-agent 并发交付）", status: "done", priority: "P1", category: "Backend", estimate_days: 4, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["PRD-009","ADR-038","task-157-163","shipped-2026-06-01"] },
  { id: "T-LEDGER", version: "tools", title: "LEDGER.md + Rule G 取号 SOP + ADR-041（跨 series 防撞号 SSOT, gen-data.mjs 漂移检查兜底）", status: "done", priority: "P1", category: "Docs", estimate_days: 0.3, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["ADR-041","Rule-G","SSOT","shipped-2026-06-01"] },
  { id: "T-MEMORY", version: "tools", title: "MEMORY.md 项目级 Memory（交互节奏 + Rule A-G 反例 + 13 条跨 session 教训）", status: "done", priority: "P2", category: "Docs", estimate_days: 0.3, owner: "claude", start_week: 23, end_week: 23, discovered: "2026-06-01", tags: ["docs","memory","onboarding","shipped-2026-06-01"] },

  // ── 计划中的特性 ──
  { id: "S-92", version: "v0.9.6", title: "还原时安全扫描 YARA + ClamAV 双引擎", status: "todo", priority: "P1", category: "Feature", estimate_days: 9, owner: "tbd", start_week: 27, end_week: 28, discovered: "2026-05-28", tags: ["premium","yara","clamav","C-007"] },
  { id: "S-93", version: "v0.9.8", title: "KubeVirt VM 备份恢复", status: "todo", priority: "P2", category: "Feature", estimate_days: 5, owner: "tbd", start_week: 30, end_week: 31, discovered: "2026-05-28", tags: ["kubevirt","C-006"] },
  { id: "S-97", version: "v0.9.7", title: "应用级恢复演练（BPMN + Kanister）", status: "todo", priority: "P2", category: "Feature", estimate_days: 7, owner: "tbd", start_week: 29, end_week: 30, discovered: "2026-05-28", tags: ["premium","DR-drill"] },

  // ── 历史里程碑 ──
  { id: "M-E", version: "v0.9.0-MC", title: "Multi-Cluster Manager + 跨集群恢复 Wizard", status: "done", priority: "P0", category: "Feature", estimate_days: 7, owner: "claude", start_week: 20, end_week: 21, discovered: "2026-05-24", tags: ["milestone","kasten-style"] },
  { id: "M-D", version: "v0.9.0.3", title: "制品分发闭环（ACR + Azure Blob + Cloudflare）", status: "done", priority: "P0", category: "Infra", estimate_days: 2, owner: "claude", start_week: 21, end_week: 21, discovered: "2026-05-25", tags: ["milestone"] },
  { id: "M-C", version: "v0.9.0.4", title: "Multi-arch（amd64 + arm64）", status: "done", priority: "P1", category: "Infra", estimate_days: 1, owner: "claude", start_week: 21, end_week: 21, discovered: "2026-05-26", tags: ["milestone"] },
  { id: "M-B", version: "v0.9.1.0", title: "Install UX（preflight + EULA + USER_MANUAL §23）", status: "done", priority: "P1", category: "Infra", estimate_days: 2, owner: "claude", start_week: 21, end_week: 21, discovered: "2026-05-26", tags: ["milestone"] },
  { id: "M-A", version: "v0.9.1.2", title: "UI 重构（Sidebar 10→7 + Observability hub）", status: "done", priority: "P1", category: "Frontend", estimate_days: 3, owner: "claude", start_week: 21, end_week: 21, discovered: "2026-05-26", tags: ["milestone"] },
];

/* ---- 卡片深链：把 PRD/ADR/痛点指回源文档（file:// 相对路径）---- */
const SOURCE_LINKS = {
  prd: "../PRD.md",
  adr: "../架构设计.md",
  test: "../测试用例.md",
  roadmap: "../ROADMAP.md",
  glossary: "../术语表.md",
  changelog: "../CHANGELOG.md",
  engineering: "../ENGINEERING.md",
  review: "../PRD-review/INDEX.md",
  api: "../API-REFERENCE.md",
  openapi: "../openapi.yaml",
  runbook: "../RUNBOOK.md",
  slo: "../SLO-RTO-RPO.md",
};

return { META, DECISIONS, PRDS, ADRS, WEEKLY, CUSTOMER_PAIN, LINEAGE, TASKS, SOURCE_LINKS };
})();
