# SupKube 编号台账 (Number Ledger · Single Source of Truth)

> **用途**：项目**所有文档编号**（PRD / ADR / TC / 决策 D / 客户痛点 C 等）的**唯一权威号源**。
> **规则**：新建任何编号文档前，**必须先来这里取号**（详见 [ENGINEERING.md Rule G](./ENGINEERING.md)）。
> **维护频率**：每次新增编号即写；预期每天有多次 commit。
> **不进本台账**：任务 `#XXX`（由 TaskCreate 工具内置 counter 管理，无需 manual ledger）。

---

## 一、下个空号速查（取号先看这里）

| Series | 已占最高号 | **下个空号** | 用途 | 详表 |
|---|---|---|---|---|
| **PRD** | PRD-015 | **PRD-016** | Product Requirements Document（影响 UX/数据模型的功能） | §二 |
| **ADR** | ADR-046 | **ADR-047** | Architecture Decision Record（架构级决策，含让号占位） | §三 |
| **TC-REG** | TC-REG-011 | **TC-REG-012** | Regression test case（bug fix 强制回归） | §四 |
| **TC-POL** | TC-POL-008 | **TC-POL-009** | Policy 测试用例 | §四 |
| **TC-APP** | TC-APP-003 | **TC-APP-004** | Applications 测试用例 | §四 |
| **TC-IMP** | TC-IMP-004 | **TC-IMP-005** | Import Policy 测试用例 | §四 |
| **TC-XCR** | TC-XCR-005 | **TC-XCR-006** | Cross-Cluster Restore 测试用例 | §四 |
| **TC-LBS** | TC-LBS-003 | **TC-LBS-004** | Local Backup Store 测试用例 | §四 |
| **TC-RP** | TC-RP-006 | **TC-RP-007** | Restore Point 测试用例 | §四 |
| **TC-MC** | TC-MC-004 | **TC-MC-005** | Multi-Cluster 测试用例 | §四 |
| **D**（战略/产品决策） | D-35 | **D-36** | dashboard `DECISIONS` 数组里的当日决策 | §五 |
| **C**（客户痛点） | C-012 | **C-013** | dashboard `CUSTOMER_PAIN` 数组 | §六 |

> **如果你的 series 不在上面**：在 §七"新 series 注册"加一行（e.g. 新增 TC-SEC 安全扫描 series），然后回到本表加一行下个空号。

---

## 二、PRD 已占号 (PRD-001 ~ PRD-014)

| 号 | 主题 | 占号人 / 时间 | 状态 | 详情位置 |
|---|---|---|---|---|
| PRD-001 | 跨集群还原前置检查闭环 (Restore Preflight Checklist) | Mars / 2026-05-30 | **研发中（2026-06-03 Mars D-WAIT-003 拍板; 4 finding + T3 拓扑校验已修订完成 §4 + §8 DoD #6/#7/#10）** | PRD.md `#prd-001` |
| PRD-002 | Transform 一等公民（两层模型） | Mars / 2026-05-30 | 已评审 v1.3 | PRD.md `#prd-002` |
| PRD-003 | AI Advisor inside SupKube（推荐型 · 非自治） | Mars / 2026-05-30 | 已评审 | PRD.md `#prd-003` |
| PRD-004 | MCP Server "Supkube Skills"（Streamable HTTP） | Mars / 2026-05-30 | 已评审 | PRD.md `#prd-004` |
| PRD-005 | Log Viewer v2（运维级日志观察平台） | Claude / 2026-05-31 | 已评审 | PRD.md `#prd-005` |
| PRD-006 | Activity Task Detail Timeline | Claude / 2026-05-31 | 已评审 | PRD.md `#prd-006` |
| PRD-007 | 完整 3-2-1-1-0 数据韧性（5 层 + Layer 4 Copy + DR Drill） | Claude / 2026-05-31 | 已评审 v1.1 | PRD.md `#prd-007` |
| PRD-008 | RP 删除生命周期 + Activity 持久化 + Force Delete 治理 | Claude / 2026-05-31 | **研发中（2026-06-03 Mars 评审通过 → 进研发；D1-D5 finding 闭环 + M-1/M-2 正文回填；⏸ ADR-039 存储选型占号待 Phase 0 实测 — Rule H 隔离, 主体可推进）** | PRD.md `#prd-008` |
| PRD-009 | Policy 模型对齐 Kasten（Snapshot + Import Policy 双 Action） | Claude / 2026-05-31 | **研发中（2026-06-03 Mars 评审通过 → 进研发；v2 G1-G5 全闭环 + M-3/M-4/M-5 正文回填 CRD 字段+错误码+N=5 阈值；ADR-038 已写）** | PRD.md `#prd-009` |
| PRD-010 | DR Topology v2（可视化重构 + Local Snapshot/Backup Copy 节点） | Claude / 2026-05-31 | **研发中（2026-06-03 Mars 评审通过 → 进研发；F1-F4 finding 闭环 + M-6/M-7 正文回填 验证徽章+箭头类型；⏸ ADR-040 SVG 视觉规范占号待写 — Rule H 隔离, 主体可推进 + 先落 ADR-040 正文）** | PRD.md `#prd-010` |
| PRD-011 | AI Backup Advisor MVP（规则算分 + LLM 解释 · Canonical DSL · 本地小闭环） | Claude / 2026-06-01 | **研发中（2026-06-03 D-WAIT-002 已拍 Mars 4 维 25/35/20/20 标准对标 → 进研发；H1/H2/H5 闭环 + §4.6 API 正文回填；⏸ ADR-043 评分细则 v1.0.0 + ADR-046 AI 决策两层体系占号待写 — Rule H 隔离, evaluator.go skeleton 可推进）** | PRD.md `#prd-011` |
| PRD-012 | Call Home / Auto-Support（三档连接 · 自动开 Case · opt-in） | Claude / 2026-06-01 | 改正中（I1 finding 闭环 2026-06-02 默认逐次确认 + SECURITY §6.C 并入, §10 修订段 + §8 DoD #13-#15; I2 customer-id 仍 Blocked 等 Case API spec） | PRD.md `#prd-012` |
| PRD-013 | SupKube Four-Eyes Authorization（备份安全二次审批 + MFA · 4 大类 15 受保护操作 · Veeam VBR 13 对标 + Kasten 没有差异化） | Claude / 2026-06-02（Mars D-WAIT-002 frame shift 派生） | 草稿（占号 + 头表 + 立项缘由 + Goal/Stories/Functions/UI/DoD/Tasks/History 待 ship；不卡 PRD-011 Demo 闭环，PRD-011 §6 维度 3 / MFA+二次审批 10 分标 N/A → 等本 PRD ship 激活） | PRD.md `#prd-013` |
| PRD-014 | **前端 UI 暴露模型（运维方 Day-0 可配 · 4 模式：LoadBalancer/NodePort/ClusterIP+port-forward/Ingress · 镜像 Dex publicURL 范本 · 装后 NOTES.txt 按所选模式打印访问方式）** | Mars 决策 / Claude 起草 / 2026-06-02 | **草稿** ✅（chart 机制已具备 service.frontend.type；本 PRD 补 ClusterIP 一等模式 + 修 NOTES.txt 服务名 bug + 模式感知 NOTES + values 4 模式菜单 + USER_MANUAL §5.5；正文 11 段已写） | PRD.md `#prd-014` |
| PRD-015 | **AI 容灾决策顾问（AI DR Decision Advisor · Premium 独占）**：标准基本盘 A + 客户决策面 B 两层体系（ADR-046）+ 决策历史库 + 盲区检测报告 + DRP/CRP 编排 + 风险决策框架工具箱（RICE/RPN/AHP/TOPSIS/FAIR/OCTAVE…）；依赖 CMDB/依赖图引擎/RAG/向量记忆。从 PRD-011 MVP（A 面评分+解释）拆出的 Premium 上层能力 | Mars 决策 / Claude 起草 / 2026-06-03 | **草稿**（charter 级，post-MVP **不阻塞当前研发**；正文见 PRD.md `#prd-015`）| PRD.md `#prd-015` / ADR-046 |

---

## 三、ADR 已占号 (ADR-001 ~ ADR-045)

> **本段是号源 SSOT**。架构设计.md 顶部 "ADR 号台账" 段保留作 ADR **详细元数据**（含 decision 摘要 / Alternatives 等），但**取号先看这里**。
> ADR-001~031 已固化在 架构设计.md §9 正文，本表只列号 + 主题；ADR-032+ 列完整状态。

| 号 | 主题 | 占号人 / 时间 | 状态 | 详情位置 |
|---|---|---|---|---|
| ADR-001 ~ 031 | （已固化历史决策, 见 架构设计.md §9 + 台账） | 历史 | ✅ Decided | 架构设计.md §9 |
| ADR-032 | Operator 评估（v1 SupKube + Velero 共存） | Mars / 2026-05-30 | ✅ Decided | 架构设计.md §9 |
| ADR-033 | AI Advisor 架构（Engine + Provider 抽象 + 脱敏管线 + 出境治理） | Claude / 2026-05-31 | 草稿 | 架构设计.md §9 |
| ADR-034 | MCP 协议选型 = Streamable HTTP（不用 SSE） | Claude / 2026-05-31 | 草稿 | 架构设计.md §9 |
| ADR-035 | 结构化日志规范 + 错误代码体系（ERR_* + Summary/Detail/Debug） | Claude / 2026-05-31 | ✅ Accepted with conditions | 架构设计.md §9 |
| ADR-036 | SSE / 长连接 / 流式传输项目级口径（Live Tail 保 SSE + nginx ingress 配置） | Claude / 2026-05-31 | 草稿 | 架构设计.md §9 |
| ADR-037 | 统一数据采集架构（CollectionContract + Collector/Server 分离 + Canonical DSL + 三档连接） | Agent A (PRD-011/012 立项) / 2026-06-01 | 草稿 | 架构设计.md §9 |
| ADR-038 | ImportPolicy CRD + Controller 设计（替代 Velero `backupSyncPeriod` 60s 兜底） | Agent A (PRD-009 v2) / 2026-06-01 | 草稿 | 架构设计.md §9 |
| ADR-039 | **Activity 持久化与 audit event 存储选型** ← 让号自 ADR-037 | PRD-008 让号 / 2026-06-01 | 草稿（占号待 PRD-008 Phase 0 落地） | 架构设计.md §9（待写） |
| ADR-040 | **DR Topology SVG 视觉规范** ← 让号自 ADR-038 | PRD-010 让号 / 2026-06-01 | **草稿 ✅ 正文已写**（2026-06-02 解锁 PRD-010 DoR-5; 架构设计.md §9 正文 6 段: Context/Decision (D1-D5) / Consequences/Alternatives/Verification (V1-V6) / References; D1 6 色系互斥 + D2 5 类 flows.type enum + D3 Layer 1-5 徽章 + D4 Layer 5 改顶部验证徽章 4 状态 + D5 design token 集中 svg-topology.css） | 架构设计.md §9 ADR-040 |
| ADR-041 | **项目编号统一台账 (LEDGER.md) + Rule G 取号 SOP**（跨 PRD/ADR/TC/D/C 全 series 防撞号；并发由 main agent 预分配；让号 forward-only；漂移检查 gen-data.mjs） | Claude (Rule G 演练) / 2026-06-01 | **草稿** ✅（架构设计.md §9 正文已写, 含 Context/Decision/Consequences/Alternatives/Verification/References 7 段） | 架构设计.md §9 ADR-041 |
| ADR-042 | **开发主体环境上云 (Azure AKS) + CI/CD 三集群推送策略** (关闭本机 docker-desktop; push to main → aks-dev 自动; tag → aks-test; manual gate → aks-prod; dev-deploy.sh 退役; 保留 amd64+arm64 多架构) | Mars 决策 / Claude 起草 / 2026-06-01 | **草稿** ✅（架构设计.md §9 正文 8 段已写: Context/Decision/Consequences/Alternatives/§5 prod 集群待建/§6 dev-deploy 退役时间线/Verification/References；cd.yaml 三阶段已扩；dev-deploy.sh 头加 DEPRECATED notice） | 架构设计.md §9 ADR-042 / cd.yaml / hack/dev-deploy.sh |
| ADR-043 | **AI Backup Advisor 评分细则 v1.0.0**（Mars 100 分制 4 维 + 行业对标 ISO 27002 §8.13 / NIST CSF / NIST SP 1800-26 / NIST SP 800-53 Rev.5 CP-9 + Mars frame shift "采客户→采平台" 5 个采集 SOP: Tier label / Air-Gapped Vault (WORM+Glacier+Archive Tier) / Vault 间接检测 / MFA 等 PRD-013 / DR Drill 等 PRD-007 §4.6） | Mars D-WAIT-002 / Claude 起草 / 2026-06-02 | **草稿**（占号 + 待写入 架构设计.md §9 正文 7 段: Context/Decision/Consequences/Alternatives/Verification/References + 评分公式细则表 + 维度采集分档表） | 架构设计.md §9 ADR-043 |
| ADR-044 | **快速调试模式 (Fast Debug Mode)：本地秒级内循环 `hack/dev-local.sh` + feature 分支推送节奏**（与 ADR-042 云端部署通道互补；触发词"进入快速调试模式"→ Vite HMR + go run / port-forward 二模式，绕过 docker build/ACR/AKS；调试期每 2h(改动大 1h) push 到 feature 分支，仅触发 CI 校验不触发部署） | Mars 决策 / Claude 起草 / 2026-06-02 | **草稿** ✅（架构设计.md §9 正文 7 段已写: Context/Decision/Consequences/Alternatives/Coverage/Verification/References；hack/dev-local.sh + FAST-DEBUG-MODE.md + README 指引已落） | 架构设计.md §9 ADR-044 / hack/dev-local.sh / FAST-DEBUG-MODE.md |
| ADR-045 | **ApprovalPolicy/ApprovalRequest CRD 设计 + Dex MFA 集成模式** ← 让号自 ADR-044（PRD-013 原非正式预占 ADR-044，与快速调试模式 ADR-044 撞号，按 Rule G §C 让号至下个空号） | PRD-013 让号 / 2026-06-02 | **占号**（待 PRD-013 ship 时写 架构设计.md §9 正文） | 架构设计.md §9（待写）/ PRD.md `#prd-013` |
| ADR-046 | **AI 容灾决策两层体系（标准基本盘 A + 客户决策面 B）**：A=评分+盲区检测权威（永不被覆盖，跨客户可比）；B=DRP/CRP 执行权威（AI 枚举待决策→暴露差异→客户终审签字→决策历史库→唯一执行准则）。"从权 B>A" **仅指执行层、非评分层**（两层正交，故无跨客户不可横比问题）。闭环=标准兜底/AI引导/客户终审/系统落地/全程可追溯；非自治 Rule F 不变。完整能力（决策库+盲区检测+DRP/CRP编排+风险框架工具箱）= Premium 独占，超 PRD-011 MVP → 建议立 PRD-015 | Mars 决策 / Claude 起草 / 2026-06-03 | **草稿**（机制已写 PRD-011 §4.4；架构设计.md §9 正文待写）| PRD.md `#prd-011` §4.4 / 架构设计.md §9（待写）|

---

## 四、TC 已占号（按 series 分）

> 测试用例编号在 [测试用例.md](./测试用例.md) 维护正文。本表是号源。

### TC-REG (Regression, bug fix 强制回归)
| 号 | 主题 | 时间 |
|---|---|---|
| TC-REG-001 ~ 010 | 历史回归用例（见 测试用例.md） | — |
| TC-REG-011 | backup-errors-visibility 回归测试 | 2026-05-?? |

### TC-POL (Policy)
| 号 | 主题 | 时间 |
|---|---|---|
| TC-POL-001 ~ 006 | 历史 Policy 用例 | — |
| TC-POL-007 | RBAC patch + symmetric policy-run-instant | 2026-05-?? |
| TC-POL-008 | （TBD - 见 测试用例.md） | — |

### TC-APP (Applications)
| 号 | 主题 | 时间 |
|---|---|---|
| TC-APP-001 ~ 003 | 含一键 Snapshot 流 | — |

### TC-IMP (Import Policy, 2026-06-01 新立)
| 号 | 主题 | 时间 |
|---|---|---|
| TC-IMP-001 | Continuous 60s 真跨集群 sync | 2026-06-01 |
| TC-IMP-002 | Scheduled cron `*/5 * * * *` | 2026-06-01 |
| TC-IMP-003 | fingerprint enforce + 篡改 sha256 | 2026-06-01 |
| TC-IMP-004 | sharedSecret 不匹配 3 模式对照 | 2026-06-01 |

### TC-XCR / TC-LBS / TC-RP / TC-MC
（详见 测试用例.md；新建用例按 series 取下个号，回填本表）

---

## 五、D (战略/产品决策) 已占号 (D-12 ~ D-31)

> 全部 D-XX 完整内容在 [dashboard/data.js](./dashboard/data.js) `DECISIONS` 数组。本表只列号 + 一句话主题。

| 号 | 日期 | 一句话主题 |
|---|---|---|
| D-12 | 2026-05-30 | ADR-031 5 层 3-2-1-1-0 模型立 |
| D-13 | 2026-05-30 | PRD.md 状态机建立 |
| D-14 | 2026-05-30 | AI 三段路线 Phase A/B/C |
| D-15 | 2026-05-30 | OpenClaw 定位 + MCP Skills 开源 |
| D-16 | 2026-05-30 | P0 Demo Sprint 2 周锁定 |
| D-17 | 2026-05-31 | Verify-before-ship 入 memory |
| D-18 | 2026-05-31 | Rule A/B/C 三铁律 |
| D-19 | 2026-05-31 | P0 Demo 5/5 完成 |
| D-20 | 2026-05-31 | PRD-Review 8 finding 闭环 |
| D-21 | 2026-05-31 | ADR-LEDGER 单一编号来源建立 |
| D-22 | 2026-05-31 | SECURITY.md §6 AI 出境治理 |
| D-26 | 2026-05-31 | PRD-003~007 全部已评审 |
| D-27 | 2026-06-01 | AI Backup Advisor 3 轮收敛 |
| D-28 | 2026-06-01 | 术语表 + CHANGELOG + ENGINEERING.md 体系 |
| D-29 | 2026-06-01 | PRD-009 v2 Ship + 7-agent 并发 |
| D-30 | 2026-06-01 | ADR-037/038 让号治理 |
| D-31 | 2026-06-01 | MEMORY.md 项目级落地 |
| D-32 | 2026-06-01 | **草稿** ✅ Rule G + LEDGER.md 立 + ADR-041（4 步 SOP 首次演练成功；正文已写入 dashboard `DECISIONS`） |
| D-33 | 2026-06-01 | **草稿** ✅ 开发主体上云（ADR-042 + cd.yaml 三阶段 + dev-deploy.sh DEPRECATED；Rule G 第二次演练; 正文已写入 dashboard `DECISIONS`） |
| D-34 | 2026-06-02 | **草稿** ✅ PRD-Review 第六份 finding 全 ship (Auto 5h 自主工作): PRD-009 v2 §8.2 G5 / PRD-008 D1-D5 / PRD-010 F1-F4 / PRD-011 H1/H2/H5 / PRD-012 I1 全部 finding 闭环。4 PRD 状态 草稿→改正中。PRD-011 H1 数值待 Mars 拍 (D-WAIT-002), PRD-012 I2 仍 Blocked 等 Case API。CD #2 deploy-dev OIDC 凭据缺失 D-WAIT-001 写进 等待决策.md 等 Mars 拍 A/B。 |
| D-35 | 2026-06-03 | **草稿** ✅ Mars 三连决策：① PRD-008/009/010 评审通过→研发中 + 二级残留 M-1~M-7 正文回填（AI 辅助深审，verify-don't-trust 纠了 1 处夸大）；② PRD-011 评分细则 D-WAIT-002 落地——Mars 自定 **4 维标准对标矩阵**（备份覆盖25/韧性35/防勒索20/可恢复性20，对标 ISO 27002+NIST CSF/1800-26/800-53）替代简版 5 维，§4.2 重写 + Q4 两硬阈值生效，PRD-011→研发中；③ **ADR-046 立**：AI 容灾决策**两层体系**（标准基本盘 A=评分+盲区检测权威永不被覆盖跨客户可比；客户决策面 B=DRP/CRP 执行权威经 AI 引导+客户终审+决策历史库）——"从权 B>A"仅指执行层非评分层（Mars 2026-06-03 澄清，纠正早前"B 覆盖评分规则"误述）。完整能力 = Premium 独占 + 超 PRD-011 MVP → **建议立 PRD-015（AI 容灾决策顾问）**。Rule G 第 3 次演练取号 ADR-046 + D-35。**待 propagate（并行 agent）**: ADR-043/046 §9 正文 + USER_MANUAL + TC-AI-MVP + 术语表风险等级收口。 |

（D-23/24/25 跳过号或归并到 D-26；如发现实际占用请补回此表）

---

## 六、C (客户痛点) 已占号 (C-001 ~ C-012)

> 全部 C-XXX 完整内容在 [dashboard/data.js](./dashboard/data.js) `CUSTOMER_PAIN` 数组。本表只列号 + 一句话。

| 号 | 日期 | 一句话主题 | 关联任务 |
|---|---|---|---|
| C-001 | 2026-05-28 | "没有看到 Import 的标签" | ✅ v0.9.1.6 已修 |
| C-002 | 2026-05-28 | "没有一键适配检查" | task #104 ⭐ |
| C-003 | 2026-05-28 | "没有创建新的 import 还原点的地方" | task #88 ✅ v0.9.1.13 |
| C-004 | 2026-05-28 | "点击不了 Restore" | ✅ v0.9.1.5 已修 |
| C-005 | 2026-05-28 | "客户不想脱离这个页面（跨集群自动 SC mapping）" | task #104 |
| C-006 | 2026-05-28 | KubeVirt VM 备份能力 | v0.9.8 / task #93 |
| C-007 | 2026-05-28 | 还原时安全扫描 | v0.9.6 / task #92 |
| C-008 | 2026-05-28 | Activity 详细步骤与用时 | PRD-006 / task #117 |
| C-009 | 2026-05-28 | Data Usage Report | task #97 |
| C-010 | 2026-05-28 | MC 下拉切换 Dashboard 不变化 | ✅ task #98 |
| C-011 | 2026-05-28 | 还原卡在 Restoring | ✅ task #102 |
| C-012 | 2026-05-28 | 3 Error 什么也看不到 | PRD-005 / task #103 |

---

## 七、新 series 注册

如果你要的编号 series 不在 §一速查表里（e.g. 准备新建 TC-SEC 安全扫描测试 series, 或 RFC-001 architecture RFC 等），请：

1. 在本节列一行: `| TC-SEC | 安全扫描 (YARA/ClamAV) 测试 | (空, 新立) | TC-SEC-001 | 测试用例.md `
2. 回到 §一速查表加一行
3. 然后按 §八 SOP 取号

| Series | 用途 | 已占最高号 | 下个空号 | 详表位置 |
|---|---|---|---|---|
| （示例位）TC-SEC | 还原扫描双引擎 (YARA + ClamAV) 测试 | — (未立) | TC-SEC-001 | 测试用例.md（待立） |
| （示例位）RFC | Architecture RFC (较 ADR 更前期的提案) | — (未立) | RFC-001 | 待新建 RFC.md |

---

## 八、取号 SOP（必读）

### A. 单 Agent / 顺序工作

```
1. 打开 LEDGER.md
2. 在 §一速查表找到你要的 series 行, 读 "下个空号"
3. 编辑 LEDGER.md:
   (a) 在对应 §二/三/四/五/六 已占号详表加一行 (号 + 主题 + 占号人 + 时间 + 状态="占号")
   (b) §一速查表那一行 "已占最高号" 改为刚才的号, "下个空号" +1
4. 保存 LEDGER.md
5. 然后去对应文档 (PRD.md / 架构设计.md / 测试用例.md / dashboard/data.js) 写正文
6. 正文写完, 回 LEDGER.md 把刚才那行的状态从 "占号" 改为 "草稿"
```

### B. 多 Agent 并行（这是规则的真正适用场景）

> 单 Agent 顺序写 LEDGER 是安全的（read-then-write 在一个 message 内原子）。**多 Agent 并行**写 LEDGER 会有 race condition——必须由 **main agent 集中预分配号**。

**强制规则**（Rule C v2 升级版的延伸）：

```
1. main agent 启动并行 agent 前, 自己 read LEDGER.md
2. main agent 一次性 reserve 所有号 (e.g. 5 个 PRD → reserve PRD-013/014/015/016/017)
3. main agent 把 §二已占号表 + §一速查表一次性更新到位
4. main agent 在每个并行 agent 的 prompt 里**显式写**: "你的号 = PRD-014, 不要自己取号"
5. 并行 agent 收到指定号后直接写正文, 不碰 LEDGER
6. 全部 agent 完成后, main agent 把 LEDGER 里这批号的状态改为对应文档实际状态 (草稿/已评审等)
```

**反例（不能这么做）**：
- ❌ launch 3 个 agent 各自跑 "去 LEDGER 取个 PRD 号然后写" → race
- ❌ launch 1 个 agent 跑 "去 LEDGER 取号写 5 个 PRD" → 串行 + 单点失败

### C. 让号 / Renumbering（2026-06-01 ADR-037/038 复发教训）

如果发现你占的号已被别处占用（包括跨 PRD/ADR 撞号），按以下流程让号：

```
1. 不要直接抢占——以本台账 §一速查表为准
2. 让号到本表 §一 "下个空号" 字段值
3. 在 §二/三 详表里, 老号行标 "🔄 让号自" + 新号行加新条目
4. 同步更新所有引用该号的文档 (PRD.md / 架构设计.md / dashboard/data.js)
5. dashboard 漂移检查 `node dashboard/gen-data.mjs` 必须 ✅ 无漂移
```

### D. 释放号 / Release

号占了但**没写正文 + 主题变了**，可以释放：

```
1. 在 §二/三/四 详表标该行 "❌ 已释放（原计划: XXX, 改用 YYY 号占）"
2. **不改速查表的"下个空号"**（避免后人撞号；让号是 forward-only）
3. 这意味着 LEDGER 历史会出现"跳号"——这是正常的，号是 immutable 标识
```

---

## 九、与现有台账的关系

| 现有台账 | 角色 |
|---|---|
| **本文件 LEDGER.md** | **号源唯一权威 SSOT**——所有 series 取号必须先来这里 |
| `架构设计.md` §ADR 号台账 | ADR 详细元数据（含决策摘要、Alternatives）——号来本表 |
| `PRD.md` 顶部索引表 | PRD 的状态视图——号来本表 |
| `测试用例.md` §前缀表 | TC series 前缀解释 + 章节链接——号来本表 |
| `dashboard/data.js` `DECISIONS` / `CUSTOMER_PAIN` / `ADRS` / `PRDS` 数组 | dashboard 渲染数据——号来本表，由 `gen-data.mjs` 漂移检查保证一致 |

**漂移检查**：每次更新 LEDGER 后跑 `node dashboard/gen-data.mjs`，确保 LEDGER ↔ dashboard ↔ PRD.md/架构设计.md 三方一致。

---

## 十、维护提示

- 新增 PRD/ADR 时, **占号 → 写文档 → 改状态** 三步序列, 不能跳过中间任何一步
- 跨 session 接力时, **新 Agent / 新 session 第一件事 = 来 LEDGER 查最新空号**
- 本台账行数预期 1-2 月增长 30-50 行, 每季度盘点一次"释放/归档"老条目（移到末尾 §十一 历史归档段, 不删）
- 当前未启用的 series（§七示例位 TC-SEC / RFC）等真正启用时, 同时把示例位的"—"改成实际号

---

## 十一、历史归档（用于版本回滚 / 审计追踪）

（空——首次填写时间 2026-06-01。每季度盘点时把超过 180 天 + status=Shipped 的条目移入此段。）

---

> **本台账 v1.0 — 2026-06-01**：Mars 提议建立"全编号统一台账 + 取号 SOP"，从 ADR-037/038 撞号复发教训长出。规则同步落 [ENGINEERING.md Rule G](./ENGINEERING.md) + [MEMORY.md §八 L-14](./MEMORY.md)。
