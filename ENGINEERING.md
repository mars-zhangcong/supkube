# SupKube 工程手册（Engineering Handbook）

> **用途**：本文件是 SupKube 的**工程过程单一来源**。此前这些规则只活在 AI 助手的 memory 和 dashboard 的决策数组里——一旦换人/换工具就蒸发。本文件把它们**落成仓库文档**，让真人能 onboard、让规则可被引用与审计。
> **读者**：研发、SRE、PM、新加入的工程师、以及驱动本项目的 AI Agent。
> **关联**：[术语表.md](术语表.md) · [PRD.md](PRD.md) · [架构设计.md](架构设计.md) · [CHANGELOG.md](CHANGELOG.md) · [测试用例.md](测试用例.md)

---

## 1. 核心工程规则（铁律）

> 这些规则是从踩坑里长出来的，**不是建议，是纪律**。例外都写明了。

### Rule A — PRD 先于代码

任何**影响 UX 的 Feature** 必须先过 PRD（走状态机：草稿→排队评审→评审中→{改正中｜驳回｜已评审}→研发中→待验收→Shipped→归档），**再写代码**。

**例外**（可直接做，不必先 PRD）：
- 单文件 bug 修复
- UX 微调（文案、间距、颜色）
- 纯文档变更
- dev workflow 脚本

### Rule B — 多任务并行开多 Agent

多个**相互独立**的任务，并行开多个 Agent 同时推进，不要串行等待。

### Rule C — 共享契约必须有单一来源

当多个并行 Agent / 多个模块要写**同一个共享契约**（如 deep-link schema、API DTO、CollectionContract、错误码体系）时，**必须指定唯一权威来源**，其他地方引用它而非各写各的。

> **教训来源**：X1 finding —— deep-link schema 在 PRD-005/006 各写一份导致不一致；X2 —— ADR 撞号。**单一来源是防漂移的根本手段**，贯穿全项目（ADR-LEDGER 防撞号、PRD.md 索引、术语表、本手册）。

### Rule D — Verify-before-ship（构建≠交付）

**`build pass` ≠ `deploy 完成` ≠ `ship`。** 一个变更只有满足以下全部才算 ship：

1. 部署到目标集群后 `curl /status` 看到**新的 buildStamp**（证明跑的是新代码，不是缓存镜像）。
2. 走**完整的 user journey**（不是单个 endpoint），从用户视角验证功能真的可用。
3. 涉及双集群的，两个集群都验。

> **教训来源**：5 次 false-complete pattern（D-17）。子规则：① 验完整 journey 不验单 endpoint；② **软件不能问用户"自己是否存在"**（要主动检测，不要把判断推给用户）。

### Rule E — 回归强制律

每个修过的 bug **必须**配一个回归测试用例（测试用例.md TC-REG-*）。详见 [测试用例.md](测试用例.md)。

### Rule F — 非自治 / 推荐型

所有 AI / Call Home 能力**只给建议，不自动执行**。"应用建议"只预填 Wizard，由客户确认后自己保存。软件**绝不**自动修改客户集群资源（ADR-033 §6）。

### Rule G — 取号必先来 LEDGER（2026-06-01 新立）

任何编号文档（PRD / ADR / TC / D / C 等）**新建前必须先去 [`LEDGER.md`](./LEDGER.md) 取号**——不能直接在 PRD.md / 架构设计.md / 测试用例.md 末尾看一眼最大号自己 +1，那样并行 Agent 必撞。

**取号 SOP（最简版，完整版见 LEDGER.md §八）**：

1. 打开 `LEDGER.md` §一速查表，找你的 series（PRD / ADR / TC-* / D / C），读 "下个空号"
2. 改 LEDGER：(a) 详表加一行带号 + 主题 + 你（占号人）+ 时间 + 状态="占号"；(b) 速查表"已占最高号"和"下个空号"都 +1
3. **然后**去 PRD.md / 架构设计.md / 测试用例.md / dashboard/data.js 写正文
4. 正文写完，回 LEDGER 把状态从"占号"改成"草稿"

**并行 Agent 场景**（Rule C v2 的延伸）：**main agent 启动并行 agent 前必须集中预分配号**，把每个号写进对应 agent 的 prompt（"你的号 = PRD-014，不要自己取号"）。让 N 个 background agent 各自跑去 LEDGER 取号 = race condition，禁止。

**让号 / Renumbering**（2026-06-01 ADR-037/038 复发教训）：发现你占的号已被别处占用，**让号到下个空号**，不能抢；让号是 forward-only 的。已让号示例：PRD-008 让号 ADR-037→039；PRD-010 让号 ADR-038→040。

**漂移检查**：每次更新 LEDGER 后跑 `node dashboard/gen-data.mjs` 必须 ✅ 无漂移。

**反例**（这些都是历史踩过的坑）：
- ❌ Agent A11 在 PRD.md 末尾看最大号 = PRD-008，自己 +1 写 PRD-009；同时 Agent B15 也看到 PRD-008 自己 +1 写 PRD-009 → 撞号
- ❌ 不更新台账，靠下次 grep 全文找最大号 → 多文件分散后必漂
- ❌ 跨 session 续工作，不查 LEDGER 直接照记忆里的号写 → 撞旧号

---

## 2. 单一来源清单（Single Source of Truth）

> 项目的防漂移骨架。每类信息只有一个权威出处，其他地方引用而非复制。

| 信息 | 唯一权威来源 |
|---|---|
| 术语定义 | **术语表.md** |
| **所有文档编号（取号源）** | **[LEDGER.md](./LEDGER.md)**（PRD / ADR / TC / D / C 等全 series 统一台账，2026-06-01 立） |
| ADR 详细元数据（决策摘要 / Alternatives） | 架构设计.md 顶部 ADR-LEDGER 段（号源已迁 LEDGER.md，本段保留作详细元数据） |
| 架构决策正文 | 架构设计.md §9 |
| PRD 索引与正文 | **PRD.md** |
| 评审结论与 finding 跟踪 | PRD-review/INDEX.md |
| 测试用例与追溯矩阵 | 测试用例.md |
| 版本变更 | CHANGELOG.md |
| 安全/合规/出境治理 | SECURITY.md（§6 AI 数据处理） |
| 当前发布状态 | PROJECT-STATUS.md |
| 路线图与优先级 | ROADMAP.md |
| 工程过程规则（本文件） | ENGINEERING.md |
| **API 契约** | **openapi.yaml + API-REFERENCE.md**（端点×角色矩阵派生自 `supkube-backend/internal/auth/rbac.go`） |
| **运维排障** | **RUNBOOK.md** |
| **RTO/RPO/SLO 目标值** | **SLO-RTO-RPO.md**（同时是 PRD-011 Resilience Score 评分阈值的单源） |

> ⚠ **已知违规**：`dashboard/index.html` 把 PRD/ADR/任务数据硬编码复制，已与源漂移。整改方向见 §6。

---

## 3. 分支与版本

- **主分支**：`main`。
- **版本号双轨**（详见 CHANGELOG.md 顶部）：产品 `0.9.1.N-alpha`（四段）↔ Helm chart `0.9.1-alpha.N`（SemVer）。
- **git tag** 用产品版本：`v0.9.1.N-alpha`。
- **镜像 tag** = 产品版本，推 `supkube.azurecr.io/{backend,frontend}:<appVersion>`。
- 每个发布版本号同时 bump `supkube-helm/supkube/Chart.yaml` 的 `version`（chart）与 `appVersion`（产品）。

---

## 4. 发布流程（Release Checklist）

1. 把 `CHANGELOG.md` 的 `[Unreleased]` 切到新版本号 + 日期。
2. bump `Chart.yaml`（version + appVersion）。
3. 多架构构建镜像（`docker buildx`，amd64 + arm64）→ 推 ACR。
4. 打包 Chart → 推 `charts.supkube.com`（ACR + Azure Blob + Cloudflare Worker）。
5. **Verify-before-ship（Rule D）**：双集群部署 → `curl /status` 验 buildStamp → 走完整 user journey。
6. `git tag v<appVersion>` + push。
7. 更新 PROJECT-STATUS.md 当前发布版本表。

> 工具：`hack/dev-deploy.sh`（dual-arch + dual-cluster + 4 步真验证 + ACR TTL preflight + fail-fast + dry-run）；基础设施一次性配置见 `hack/AZURE-SETUP.md`。

---

## 5. 文档地图（Doc Map · 角色 → 该看哪份）

> 解决"文档很多但不知道该看哪份"的导航问题。

| 我是… | 我想知道… | 看这份 |
|---|---|---|
| **新工程师** | 怎么跑起来 / 工程规则 | README.md → 本文件 → 术语表.md |
| **架构师** | 架构设计 + 历史决策 | 架构设计.md（ADR-LEDGER + §9） |
| **研发** | 某 Feature 怎么做 | PRD.md（对应 PRD-XXX）+ 关联 ADR |
| **集成方 / 写 MCP Skill** | 有哪些 API / 谁能调 | **API-REFERENCE.md** + **openapi.yaml** |
| **PM** | 拿 PRD 过评审 | PRD.md + PRD-review/INDEX.md |
| **PM / 销售** | 我们承诺多快恢复 / 多大规模 | **SLO-RTO-RPO.md** |
| **测试** | 要跑什么用例 | 测试用例.md（+ 附录 A 追溯矩阵） |
| **SRE** | 装什么组件 / 排障 | **RUNBOOK.md** + 各 PRD §7 组件表 + SECURITY.md |
| **合规官** | 数据会不会出境 | SECURITY.md §6 + 术语表.md §6 |
| **任何人** | 术语到底指什么 | **术语表.md** |
| **任何人** | 现在项目啥状态 | PROJECT-STATUS.md + dashboard |
| **任何人** | 改了啥 | CHANGELOG.md |

---

## 6. 已知工程债（Engineering Debt）

| 债 | 现状 | 整改方向 |
|---|---|---|
| ~~**Dashboard 数据漂移**~~ | ✅ 已整改（2026-06-01）：`dashboard/data.js` 数据与视图分离 + `dashboard/gen-data.mjs` 从 PRD.md 索引表/ADR-LEDGER 派生 PRDS/ADRS 并做漂移看门（exit code 可接 CI） | 剩余：把 gen-data.mjs 接进 CI；DECISIONS/TASKS/LINEAGE 仍人工维护 |
| **CI 缺位** | verify-before-ship 全靠手动 | 最小化 GitHub Actions（build + lint + `node dashboard/gen-data.mjs` 漂移检查），逐步加 E2E（task P2-1/P2-2） |
| **API 无契约** | ◑ 部分整改（2026-06-01）：建 **API-REFERENCE.md**（13 组 ~90 端点目录，派生自 rbac.go）+ **openapi.yaml**（cross-cutting 契约 + 核心资源种子） | 剩余：从 rbac.go + handler struct **自动生成** openapi 全量 per-field schema |
| **USER_MANUAL 过长** | 129KB 单文件 | 拆分 + Wiki.js 文档站（task P2-4） |

---

## 变更记录

| 日期 | 操作人 | 变更 |
|---|---|---|
| 2026-06-01 | Claude | 初版。把 Rule A/B/C/D（PRD 先于代码 / 并行 Agent / 共享契约单源 / verify-before-ship）+ Rule E/F 从 memory 落成仓库文档；补单一来源清单、版本/发布流程、文档地图、工程债清单。 |
