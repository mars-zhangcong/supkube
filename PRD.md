# SupKube PRD（产品需求文档）

> 本文档是 SupKube **产品需求**的单一权威来源（Single Source of Truth）。
> **流程铁律（2026-05-30 Mars 拍板）**：所有**影响用户体验的 Feature**，必须先在这里出 PRD，经过评审通过后再进入研发；研发完成后回到 PRD 对照验收。
> **沟通口径**：Goal → Epic → Feature → User Story → Function → Task。
> **关联文档**：`架构设计.md`（架构 + ADR）/ `ROADMAP.md`（路线 + 排期）/ `USER_MANUAL.md`（客户文档）/ `测试用例.md`（验证）。

---

## 📜 PRD 编写流程

1. **起草**：作者按下方模板填 PRD，编号顺序递增（PRD-001、PRD-002…）。
2. **评审**：团队对照 Goal / User Story / UI / 验收标准 拍板；评审记录写入 PRD"评审历史"小节。
3. **研发**：依据通过的 PRD 写代码。**研发期内 PRD 不可静默改**——若有变更必须回到评审。
4. **验收**：交付物逐条对照"验收标准"小节；通过后状态置为 **Shipped**。
5. **归档**：Shipped 的 PRD 永久保留作为产品记忆。

> **回填**：已交付但未写过 PRD 的核心 Feature 可在后续 sprint 选择性回填（标注"retroactive"）。

---

## 🔄 PRD 状态机（2026-05-30 Mars 拍板）

每份 PRD 的"状态"字段必须取自下表，**仪表板 `PRD 状态` 区按此色标显示**。

| 状态 | 含义 | 谁能进入 | 可流向 |
|---|---|---|---|
| **草稿** Draft | 作者正在写 | 作者 | 排队评审 |
| **排队评审** Queued | 已提交，等评审 | 作者 → 完成草稿后 | 评审中 |
| **评审中** In Review | 评审者正在审 | 评审者 | 改正中 / 驳回 / 已评审 |
| **改正中** In Revision | 评审给了反馈，作者改 | 作者 | 排队评审（改完重交） |
| **驳回** Rejected | 不通过（方向/范围错误） | 评审者 | 归档（或重起新 PRD） |
| **已评审** Approved | 评审通过，可开研发 | 评审者 | 研发中 |
| **研发中** In Development | 研发按 PRD 写代码 | 研发 | 待验收 |
| **待验收** Pending Verification | 代码完成，等对照 PRD 验收 | 研发 → 验收人 | Shipped |
| **Shipped** | 已交付上线 | 验收人 | 归档（自然老化） |
| **归档** Archived | 已 Shipped 一段时间 / 已驳回的存档 | 系统 | — |

**状态流向图**：

```
草稿 → 排队评审 → 评审中 ┬→ 改正中 → 排队评审（回路）
                         ├→ 驳回 → 归档
                         └→ 已评审 → 研发中 → 待验收 → Shipped → 归档
```

> **重要**：状态变更必须在 PRD 文末「评审历史」小节留记录（日期 + 操作人 + 上一状态 → 新状态 + 原因/反馈）。

---

## 📑 当前 PRD 索引

| 编号 | Feature | 状态 | 关联任务 |
|---|---|---|---|
| [PRD-001](#prd-001) | 跨集群还原前置检查闭环（Restore Preflight Checklist） | **研发中（2026-06-03 Mars D-WAIT-003 拍板进研发；4 finding + T3 拓扑校验已修订完成）** | #104 |
| [PRD-002](#prd-002) | Transform 一等公民（两层模型：Transform 库 + TransformSet 引用容器） | **已评审 v1.3（2026-05-31）** | #114 |
| [PRD-003](#prd-003) | AI Advisor inside SupKube（内嵌灾备顾问 · 推荐型 · 非自治） | **已评审（2026-05-31）** | #115 |
| [PRD-004](#prd-004) | MCP Server "Supkube Skills"（对接客户侧 AI Agent 如 OpenClaw, Streamable HTTP + 5 核心 Skills + 开源） | **已评审（2026-05-31）** | #116 |
| [PRD-005](#prd-005) | Log Viewer v2 — 完整运维级日志观察平台（Virtual Scroll + SSE + 三层日志 + 错误代码 + Minimap + Forwarding + AI 根因） | **已评审（2026-05-31）** | #118 |
| [PRD-006](#prd-006) | Activity Task Detail Timeline（任务详情阶段时间线 + Log Viewer 跳转 + AI 排错位） | **已评审（2026-05-31）** | #117 |
| [PRD-007](#prd-007) | 完整 3-2-1-1-0 数据韧性（5 层可视化 + Layer 4 Backup Copy + Fingerprint + Lifecycle + DR Drill） | **已评审（2026-05-31）** | #126 |
| [PRD-008](#prd-008) | RP 删除生命周期 + Activity 持久化 + Force Delete 副作用治理（Activity 不依赖 Velero CR · 删除中锁定 · 孤儿清理） | **研发中（2026-06-03 Mars 评审通过 → 进研发；D1-D5 闭环 + 正文回填 M-1/M-2）** | #148 |
| [PRD-009](#prd-009) | Policy 模型对齐 Kasten（Snapshot Policy + Import Policy 双 Action · Continuous/Scheduled 子模式 · fingerprint enforce/warn/disabled · 替代 Velero `backupSyncPeriod` 60s 兜底） | **研发中（2026-06-03 Mars 评审通过 → 进研发 · v2 G1-G5 全闭环 + 正文回填 M-3/M-4/M-5）** — Phase 1 已 ship + Phase 2 任务拆 7 阶段; PRD-Review 第六份 5 finding 全部闭环 (G5 Phase 2 DoD/§9 任务/§9.3 风险; G1 卖点诚实表述; G2 backupSyncPeriod 默认 60s; G3 fingerprint warn 半成品标注; G4 Action Type save 不可改 UI alert) | #149 / #156 / #157-163 |
| [PRD-010](#prd-010) | DR Topology v2（Cluster/BSL 视觉重构 + Local Snapshot + Backup Copy 节点显示 · 5 类节点对齐 ADR-031 5 层模型） | **研发中（2026-06-03 Mars 评审通过 → 进研发；F1-F4 闭环 + 正文回填 M-6/M-7）** | #150 |
| [PRD-011](#prd-011) | AI Backup Advisor MVP（智能业务梳理 + 数据安全综合评分 + 备份建议 · 规则算分 + LLM 解释 · Canonical DSL · 本地分析小闭环） | **研发中（2026-06-03 Mars 拍 D-WAIT-002 数值 → 进研发；H1/H2/H5 闭环）** | #164 |
| [PRD-012](#prd-012) | Call Home / Auto-Support（三档连接 · 采集器上送 + 自动开 Case · opt-in · 复用 ADR-033 脱敏） | **改正中（2026-06-02, I2 仍 Blocked）** | #165 |
| [PRD-013](#prd-013) | SupKube Four-Eyes Authorization（备份安全二次审批 + MFA · 4 大类 15 受保护操作 · ApprovalPolicy/ApprovalRequest CRD · Veeam VBR 13 对标 + Kasten 没有 = 真差异化） | **草稿（2026-06-02 立项，Mars D-WAIT-002 frame shift 派生）** | TBD（取号待 Mars 批后建 task） |
| [PRD-014](#prd-014) | 前端 UI 暴露模型（运维方 Day-0 可配 · 4 模式 LoadBalancer/NodePort/ClusterIP+port-forward/Ingress · 镜像 Dex publicURL 范本 · 装后 NOTES.txt 按模式打印访问方式） | **草稿（2026-06-02 立项，Mars 决策）** | TBD（取号待建 task） |
| [PRD-015](#prd-015) | AI 容灾决策顾问（AI DR Decision Advisor · **Premium 独占**）：标准基本盘 A + 客户决策面 B 两层体系（ADR-046）+ 决策历史库 + 盲区检测 + DRP/CRP 编排 + 风险框架工具箱 · 从 PRD-011 MVP 拆出的 Premium 上层 | **草稿（2026-06-03 立项，ADR-046 派生 · post-MVP 不阻塞当前研发）** | TBD |

---

## 📐 PRD 模板（复制使用）

```markdown
## PRD-NNN — <Feature 名称>

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-NNN |
| **任务编号** | #NNN |
| **状态** | 草稿 / 待评审 / 评审通过 / 研发中 / 待验收 / Shipped / 归档 |
| **作者** | 姓名 |
| **目标版本** | v0.9.x |
| **关联 ADR** | ADR-XXX |

### 1. Goal（目标）
（这件事最终给客户带来什么价值；一句话）

### 2. Epic（史诗故事）
（这件事属于哪个 Epic）

### 3. User Stories（用户故事）
- 作为 <角色>，我能 <做某事>，以便 <达到某价值>。

### 4. Functions（业务逻辑 / 功能拆解）
（按子功能分点；每点描述触发、行为、边界）

### 5. UI / UX
（流程、关键界面文字描述或 ASCII 草图；i18n 文案键）

### 6. Out of Scope（明确不做）
（哪些是边界外）

### 7. 非功能性要求
（幂等、审计、权限、错误、i18n、性能）

### 8. 验收标准（Definition of Done）
| # | 验收点 |
|---|---|
| 1 | ... |

### 9. 任务拆分
- Phase 1：...
- Phase 2：...

### 10. 关联文档与任务
- ADR / PRD / Task / 文档

### 11. 开放问题（评审时讨论）
- Q1: ...

### 评审历史
- YYYY-MM-DD（参与人）：...
```

---

<a id="prd-001"></a>
## PRD-001 v2 — 跨集群还原前置检查闭环（Restore Preflight Checklist）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-001（v2 修订后） |
| **任务编号** | #104 |
| **状态** | **改正中（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.9.x |
| **关联 ADR** | ADR-031 |
| **依赖** | **PRD-002**（Transform 一等公民）必须先评审通过 |

> **修订摘要**：原 v1 把"Restore 内联创建对象"和"Transform 管理"混在一起。Mars 评审后 → 拆分。本 PRD 现在只管 Restore 抽屉的**轻量 Checklist 体验**；Transform 库管理 + 11 builtin + 使用统计在 **PRD-002**。**侧抽屉空间不足问题在新设计下自然消解**。

### 1. Goal

跨集群还原时，Restore 抽屉应是**轻量、引导式**的体验：客户一眼看到"我离 Restore 成功还差几步"，每一步给**直达解决路径**，解决后回来**重新检查**。**Restore 抽屉不再做创建/编辑动作 —— 全部委托给 Transform 页（PRD-002 承载）**。

### 2. Epic

"全职灾备运维专家" Epic 的**入口体验**：当客户发起跨集群还原时，"专家"先在门口给他一份**体检清单**，每条不通过都给条**直达解决路径**。

### 3. User Stories

- 作为系统管理员，**打开 RestoreDrawer** 时（跨集群目标），Preflight 区给我看到**清单**：每个冲突 → 是否已有匹配的 Transform Set?✓ / ✗
- 看到某条 ✗，**点「去 Transform 解决」** → 跳到 TransformSets 页（预填该冲突的派生草稿）
- 解决后**回来点「Re-check」** → 重跑 Preflight，看清单是否全 ✓
- **blocker 级冲突不可勾选忽略**（finding #1, 2026-05-31）—— 只能去 Transform 解决, 或由 admin 在另一界面**显式降级为 warning（写审计**）后才出现"忽略"选项
- **仅 warning 级冲突**可勾选「忽略此冲突」(带二次确认 dialog), blocker 级**永不显示**忽略选项, Restore 按钮持续 disabled 直到所有 blocker ✓

### 4. Functions

**后端**：
- `PreflightRestore` 端点的 `conflicts[]` 元素严格 schema（**finding #2, 2026-05-31**）—— 后端是 severity 与 matchingTransformSets 的唯一权威, 前端只渲染不判断:
  ```jsonschema
  {
    "kind": "string (enum: StorageClassMissing|VolumeSnapshotClassMissing|StorageClassBindingMode|NodePortConflict|...)",
    "severity": "string (enum: blocker | warning)",   // ← 固化枚举, 后端给定
    "matchingTransformSets": ["string"],              // ← 后端在 TransformSet 库里匹配后给出, 前端 readonly
    "payload": { /* kind-specific */ },
    "description": "string (i18n key 或纯文案)"
  }
  ```
  - `severity=blocker` ⇒ Restore 按钮 disabled, 不允许"忽略此冲突", 唯一出口是去 Transform 解决（或 admin 在另一界面**显式降级**为 warning, 写审计 trail）
  - `severity=warning` ⇒ 允许「忽略此冲突」(二次确认 dialog)
  - 前端**严禁**对 severity 做"软计算"或客户端覆盖（避免渲染不一致 + 安全旁路）
- 复用现有端点做 Re-check，无需新增（前端按需重调）

**SC→Immediate 拓扑校验（finding T3, 2026-05-31, 新增）**：
- 当某个 SC 冲突的解法是"创建同名 Immediate 别名 SC"（PRD-002 `change-storage-class` 早期 PoC 思路）时, **后端 Preflight 必须先做拓扑校验**：
  - **允许 Immediate 别名**：目标确实是**单 AZ 或 hostpath 类拓扑无关存储**（local-path / hostpath / NFS / 单 zone CSI driver, 通过 SC `volumeBindingMode` + provisioner + target cluster node label 判定）
  - **否则**：保留 `WaitForFirstConsumer`, Preflight 仍报 warning **拓扑不匹配**（"目标集群多 AZ, Immediate 可能把 PVC 绑到无 Pod 的 zone, 引发跨 AZ 流量 / 调度失败"）, 提示客户改用拓扑感知方案（NodeAffinity / topology key 限定）
- 此校验输出 conflict.payload 新增字段 `topologyHint: "single-az" | "multi-az" | "unknown"`, 前端在解法卡片上展示

**前端 `RestoreDrawer.vue`**：
- 把现有 `conflict-card with Apply Fix` 替换为 List + Checkmark 形态
- 每个 ✗ item 显示「→ 去 Transform 解决」按钮（跳转 `/transform-sets?fromConflict={kind}&payload={base64}&restoreName={name}`）
- List 顶部 「Re-check」按钮
- **Restore 按钮 disabled** 直到所有 **severity=blocker** 级 ✗ 变 ✓；warning 级 ✗ 可勾选「忽略此冲突」(二次确认), blocker 级**不渲染**忽略按钮
- 跳转 / 回来的状态保留（已选 artifacts、target ns 不丢）—— 用 router back + drawer state

**依赖 PRD-002**：
- Transform 页能识别 `?fromConflict=...&payload=...` query 自动进入派生模式
- 派生草稿命名约定 `adapt-<conflict-kind>-<short-hash>`

### 5. UI/UX

**新清单形态**（替换现有 conflict-card with Apply Fix）：

```
┌─ Preflight Checklist ───────────────────────────────┐
│  [↻ Re-check]                                        │
│                                                       │
│  ✓ NodePort collision  (Service ns/api)              │
│    matched by: strip-nodeport (builtin)              │
│                                                       │
│  ✗ StorageClass missing: csi-hostpath                │
│    no matching Transform Set                         │
│    [→ 去 Transform 解决]                             │
│                                                       │
│  ✗ VolumeSnapshotClass missing: disk.csi.azure.com   │
│    no matching Transform Set                         │
│    [→ 去 Transform 解决]                             │
│                                                       │
│  ⚠ SC binding mode: WaitForFirstConsumer (data-mover)│
│    matched by: ⚠ no builtin yet (manual fix needed)  │
│    [→ 去 Transform 解决] [✓ 忽略此条]                │
│                                                       │
│  Need to fix: 2 of 4 blockers                       │
└───────────────────────────────────────────────────────┘

[Restore]  ← disabled until all blockers ✓
```

**跳转目标**：浏览器新 tab 打开 `/transform-sets?fromConflict=StorageClassMissing&payload=...` —— 用户回来时 RestoreDrawer 状态仍在。

**i18n 新键**（en + zh-CN）：
- `restoreDrawer.preflight.matched` / `restoreDrawer.preflight.noMatch`
- `restoreDrawer.preflight.goToTransform`
- `restoreDrawer.preflight.recheck`
- `restoreDrawer.preflight.ignoreConfirm`
- `restoreDrawer.preflight.blockersRemain`

### 6. Out of Scope

- **不**在 RestoreDrawer 内创建/编辑 Transform Set / SC / VSC
- **不**做"全部一键解决"批量按钮（单条解决，避免误操作）
- **不**做 Transform 应用的 undo（v2）
- 跨集群目标的 cluster picker 在另一 Feature（已有）

### 7. 非功能性要求

- Re-check 性能 < 3s（现有 Preflight 性能足够）
- 跳转回来后 RestoreDrawer 状态完整保留（artifacts 选择、target ns、policy 选择）
- i18n 全文案；色盲友好（✓ 绿、✗ 红、⚠ 黄不仅靠颜色，还带 icon shape）

### 8. 验收标准

| # | 验收点 |
|---|---|
| 1 | 跨集群还原 Preflight 显示 List + Checkmark（不再 conflict-card + Apply Fix） |
| 2 | ✗ item 显示「→ 去 Transform 解决」按钮，点击跳转 `/transform-sets?fromConflict=...&payload=...` |
| 3 | Transform 页识别 query 进入派生模式（依赖 PRD-002） |
| 4 | 用户保存/取消 Transform 后回到 RestoreDrawer，状态不丢 |
| 5 | Re-check 工作，清单更新 |
| 6 | 全部 **severity=blocker** ✓ 后 Restore 按钮可点；否则 disabled。**blocker 级不可被忽略**（warning 级可）|
| 7 | 忽略冲突有二次确认 dialog；**忽略按钮仅在 severity=warning 行渲染, blocker 行强制隐藏**（test: 注入 severity=blocker 的冲突 → 检查 DOM 无 ignore 按钮）|
| 8 | i18n 完整；色盲友好 |
| 9 | 前端 `npm run build` 通过；现有 RestoreDrawer 既有 unit 测试不破 |
| 10 | **SC→Immediate 拓扑校验**: Preflight 在目标多 AZ 集群下不假装允许 Immediate 别名, 显式给 `topologyHint=multi-az` warning（test: 给 fake target cluster 打 3 个 zone label, 注入 SC missing 冲突 → 校验 warning 出现）|
| 11 | conflict schema: severity 字段固化为枚举 (blocker|warning), matchingTransformSets 由后端给出, 前端不做客户端 severity 计算（test: 后端注入 severity=warning, 前端不允许通过任何路径升级为 blocker disabled Restore）|

### 9. 任务拆分

**Phase 1 — 后端（小）**
- `PreflightRestore` 返回 `matchingTransformSets`

**Phase 2 — 前端 RestoreDrawer 改造（核心）**
- conflict-card → checklist 形态
- Re-check 按钮 + 重调 Preflight
- 跳转 + 回来状态保留
- Restore 按钮 disabled 逻辑 + 忽略冲突二次确认

**Phase 3 — i18n + build 验证**

**Phase 4 — E2E（随 #112 走查 campaign）**

### 10. 关联

- **PRD-002**（前置依赖，必须先评审通过）
- **#104**（原 CSI 一键适配，现拆为 PRD-001 v2 + PRD-002）
- ADR-031
- **#109** Preflight（in-product cluster health；本 PRD 的 Restore-time preflight 与之互补）

### 11. 开放问题

| Q | 问题 | 倾向 |
|---|---|---|
| Q1 | 「忽略此冲突」是否允许？ | 允许但二次确认（防止误点） |
| Q2 | 跳转/回来状态保留用 store 还是 router query? | router query（简单 + URL 可分享调试） |
| Q3 | "全部一键解决"按钮是否要? | **不做**（单条解决 + 用户清晰理解每条） |
| Q4 | Transform 页跳转用新 tab 还是同 tab? | **新 tab**（保 RestoreDrawer 状态、避免侧抽屉打断） |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 / 说明 |
|---|---|---|---|
| 2026-05-30 | Claude | 草稿 → 待评审 | 初稿提交（v1：CSI 一键适配，把"创建对象 + Transform 管理"混在 RestoreDrawer 内联） |
| 2026-05-30 | Mars | 待评审 → **改正中** | **架构级反馈**：(1) Restore 应做轻量"List + Checkmark"，不在侧抽屉创建对象（空间不足 + 不可复用）；(2) Transform 升级为一等公民，**像 Policy 一样管理**（命名、版本、Clone、**使用次数统计**），跨集群每次目标不同 → Transform Set 要可复用 + 统计；(3) builtin 库扩到至少 11 种（含 change-storage-class / change-docker-registry / nginx & traefik ingress features / remove ingress annotations & finalizers / scale-deployment）。**拆 PRD-001 为两个**：本 PRD（轻量 Restore Preflight Checklist）+ 新建 **PRD-002**（Transform 一等公民）。 |
| 2026-05-30 | Claude | 改正中 → **排队评审** | v2 修订完成：Restore 改 List + Checkmark + 「→ 去 Transform 解决」+ Re-check；所有 Transform 操作移到 PRD-002 承载；侧抽屉空间问题自然消解；Q1-Q4 待 Mars 拍板。**~~ v1 §2-§11 全部移除 ~~（含原 4.1-4.3 后端端点 / 5.1-5.4 UI 草图 / 6-11）已被本 v2 内容完整取代。** |
| 2026-05-31 | Mars (评审人 Claude 委托) | 排队评审 → **改正中** | 落 4 个 finding + T3 拓扑校验: (1) finding #1 blocker 不可忽略, 仅 warning 可忽略 + 二次确认（DoD #6/#7 加测）; (2) finding #2 conflict schema severity / matchingTransformSets 后端权威, 前端 readonly（§4 Functions 加 schema）; (3) finding #4 物理删除 v1 残留 HTML 注释块 271-511 行（git history 永久保存）; (4) finding T3 SC→Immediate 拓扑校验, 仅在确认单 AZ / hostpath 类拓扑无关存储时允许 Immediate 别名, 多 AZ 保 WaitForFirstConsumer + Preflight 拓扑告警（§4 + DoD #10 加测）。**等 Mars 重审 → 排队评审 → 已评审**。 |

---

<!-- v1 残留草图已于 2026-05-31 物理删除（finding #4）。git history 永久保存原 §4.1-§4.3 (检测 / 一键适配端点 / 与 Restore 串联) + §5.1-§5.4 (UI 草图 + i18n) + §6-§11。如需查阅: git log -p -- PRD.md 检索 "adapt-storage"。 -->

<a id="prd-002"></a>
## PRD-002 — Transform 一等公民（Transform as First-Class）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-002 |
| **任务编号** | #114 |
| **状态** | **已评审 v1.3（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.9.x |
| **关联 ADR** | ADR-031 |
| **被依赖** | PRD-001 v2（Restore Preflight Checklist 跳转入口） |
| **参考** | [Kasten Transform Sets 8.0.2](https://docs.kasten.io/8.0.2/usage/transformsets/) + [Transform API](https://docs.kasten.io/8.0.2/api/transforms) |

> **v1.1 修订（2026-05-30）**：Mars 评审反馈 + 对齐 Kasten 模型 ——
> 原 v1 把 11 个内置对象都画成"TransformSet"是**模型错误**。
> 正确模型是**两层结构**：**Transform = 原子操作**（改 SC / 改 Registry / 改 Ingress 各算一个），**TransformSet = 多个 Transform 的有序引用容器**（例如一个 TransformSet 可同时包含「strip-nodeport + change-storage-class + change-docker-registry」）。
> 客户在 UI 里**自由组合 + 自定排序**形成 TransformSet，Restore 引用**一个** TransformSet。"谁配置, 谁负责" —— 我们提供模板库, 确认责任在客户。

### 0. Transform vs TransformSet 二层模型（本 PRD 的基础概念）

| 维度 | **Transform**（原子） | **TransformSet**（容器） |
|---|---|---|
| **定位** | 一个"小工具"：选 subject + 一组 JSON Patch 操作（add/remove/replace/copy/move/test，支持 regex） | 多个 Transform 的**有序引用列表** + 命名 + 元数据 |
| **示例** | "把 PVC 的 SC 从 `ssd` 替换为 `gp2`"<br>"剥离 Service.spec.ports.nodePort"<br>"把镜像 registry 从 `acr.com/` 改为 `harbor.local/`" | "**cross-cluster-azure-to-onprem**" = [strip-nodeport, change-sc(managed→csi-hostpath), change-registry(acr→harbor)] 按此顺序执行 |
| **粒度** | 改**一类资源** | 引用 N 个 Transform，覆盖**一次 Restore 的整套适配** |
| **由谁定义** | 大部分来自**SupKube builtin 模板库**（read-only，可 Clone 后改）；客户也可从零写 | 客户**完全自定义**：挑哪些 Transform、什么顺序、起什么名 |
| **由谁确认** | 模板我们写、参数客户填 | **客户全权确认** —— 谁配置谁负责 |
| **Restore 绑定** | 不直接绑 Restore | **Restore 引用一个 TransformSet**（Restore.spec.transformSetRef） |
| **顺序** | 单个 Transform 内的 json 操作按列表顺序执行（Kasten 同语义） | **TransformSet 内 Transform 按引用列表顺序执行**（客户可拖拽重排） |
| **复用语义** | 一个 Transform 可被**多个 TransformSet** 引用 | 一个 TransformSet 可被**多次 Restore** 引用 |

> **关键设计原则（"谁配置, 谁负责"）**：
> - SupKube 提供 **Transform 模板库**（11 个 builtin + 客户 Clone 的派生）作为"砖块"；
> - 客户**自己把砖块砌成 TransformSet**（选 Transform、排序、命名、保存）；
> - Restore 时 UI 明确显示"该 TransformSet 包含哪些 Transform、按什么顺序执行" —— 客户**显式确认**后才能 Trigger Restore；
> - 出现适配错误时, 责任归属清晰（客户配置 → 客户负责；我们提供的 builtin 模板有 bug → 我们负责）。

### 1. Goal

把 Transform 和 TransformSet **同时升级为一等公民产品对象**，对齐 Kasten 行业标准的两层模型：
- **Transform 模板库**（builtin + user-defined）—— 像"配方库"
- **TransformSet 引用容器**（客户自由组合）—— 像"菜单"
两者都可命名 / Clone / **使用次数统计**，让客户积累的跨集群适配经验**沉淀为可复用资产**，并为 PRD-001 v2 提供「→ 去 Transform 解决」的承载点。

### 2. Epic

ADR-031 的 "全职灾备运维专家" Epic —— Transform 是这位"专家"的 **工具箱（cookbook）**，每用一次都该被记录、可复用、可分类、可派生。

### 3. User Stories

#### 3.1 Transform 层（模板库）
- 作为系统管理员，我能浏览**Transform 模板库**，看到 11 个 builtin + 自建 Transform，按 5 类（storage / network-ingress / image-registry / scale / cleanup）筛选。
- 作为系统管理员，从 PRD-001 v2 Preflight 的某个冲突**一键派生**新 Transform（不必从零写 JSON Patch），命名 + 编辑 + 保存到模板库。
- 作为系统管理员，我能 Clone builtin Transform 改成"我们公司版本"。
- 作为系统管理员，我能在 Transform 模板上看到"被多少个 TransformSet 引用 + 累计被多少次 Restore 用到"。

#### 3.2 TransformSet 层（引用容器）
- 作为系统管理员，我能**新建 TransformSet** → 从模板库**勾选多个 Transform** → **拖拽排序** → 命名 → 保存 → 形成"一次 Restore 用的成套适配方案"（e.g., `cross-cluster-azure-to-onprem`）。
- 作为系统管理员，在 Restore 时, 我能**选一个已保存的 TransformSet** 或**新建一个**, UI 清晰展示"这个 TransformSet 将按顺序执行哪些 Transform" → 我**显式点击确认** → Trigger Restore。
- 作为系统管理员，我能**Clone** 已有 TransformSet 改成"目标集群 B 的版本"（reorder + 替换一两个 Transform）。
- 作为系统管理员，我能在 TransformSet 列表看到**应用次数 + 最近一次 Restore 名称**，知道哪些是真正在用的成套方案。
- 作为评审/管理员, 仪表板上**看一眼最常用 TransformSet**, 作为客户运维成熟度信号。

### 4. Functions

#### 4.1 后端 seeder 重构（两层：13 个 builtin Transform + 3 个 builtin TransformSet 示范）

**(a) 13 个 builtin Transform**（原子，模板库的"砖块"）

| Name | Category | 用途 | 现状 |
|---|---|---|---|
| strip-nodeport | cleanup | NodePort collision | ✅ 已有（从 TS 重塑为 Transform） |
| strip-clusterip | cleanup | ClusterIP collision | ✅ 已有（重塑） |
| strip-loadbalancer-ip | cleanup | LoadBalancer IP collision | ✅ 已有（重塑） |
| strip-pv-binding | cleanup | PV binding warning | ✅ 已有（重塑） |
| **change-storage-class** | **storage** | SC 同名别名（PRD-001 v2 核心冲突解法） | 🆕 |
| **change-docker-registry** | **image-registry** | 镜像 registry 重映射（搬迁场景） | 🆕 |
| **nginx-ingress-features** | **network-ingress** | 适配 nginx-ingress 注解差异 | 🆕 |
| **traefik-ingress-features** | **network-ingress** | 适配 traefik 注解差异 | 🆕 |
| **remove-ingress-annotations** | **network-ingress** | 剥离不兼容注解 | 🆕 |
| **remove-ingress-finalizers** | **cleanup** | 剥离 LoadBalancer finalizer | 🆕 |
| **scale-deployment** | **scale** | DR 演练时缩 1 副本 / 启停控制 | 🆕 |
| **redirect-external-endpoints-to-sandbox** | **network-ingress** | DR Drill 时重写外部生产端点 (DB_HOST / API_URL / 支付网关) 为 sandbox 内 mock 服务, 防止演练污染生产; subject: `deployments` + `statefulsets` (任何含 env 的 workload); params: `EXTERNAL_PATTERN` (regex) + `SANDBOX_REPLACEMENT` (默认 `sandbox-mock-svc.dr-drill.svc.cluster.local`) | 🆕 v1.2 新增 (跟 PRD-007 §4.7 联动) |
| **strip-loadbalancer-to-clusterip** | **network-ingress** | **把 Service.spec.type: LoadBalancer 改为 ClusterIP**, 适用还原到**无 cloud LB provider 集群** (docker-desktop / K3s 单机 / KubeEdge / 离线集群). 不修复此项 → K8s service controller 永远等 cloud-provider cleanup LB → finalizer `service.kubernetes.io/load-balancer-cleanup` 永远在 → **ns 永远卡 Terminating 删不掉**。本次 demo 实测踩坑 (Mars 2026-05-31, 反向恢复后 `reverse-restored-2` / `reverse-restored-local` 两 ns Terminating 2 天 15 小时, 手动 patch finalizer 才解套). subject: `services`; params: 可选 `EXCLUDE_NAMES` (regex, 默认空 = 全改); 适用 TransformSet: `bundle-azure-to-onprem` / `bundle-cross-cluster-to-local` | 🆕 v1.4 新增 (反向恢复踩坑根治) |

每个 builtin Transform 的 K8s 表示（提议 schema, **对齐 Kasten + ADR-003 `len(Data)==1, key=rules.yaml`**, finding #1 / T1 2026-05-31）：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: change-storage-class
  namespace: supkube
  labels:
    supkube.io/kind: transform           # 🆕 区分 transform vs transform-set
    supkube.io/builtin: "true"
    supkube.io/transform-category: storage
    supkube.io/builtin-version: v1
data:
  rules.yaml: |                          # 🆕 v1.2 修订: 改 spec → rules.yaml, len(data)==1
    subject:
      resource: persistentvolumeclaims
    name: changeStorageClass
    json:
    - op: replace
      path: /spec/storageClassName
      regex: ^${FROM}$         # 参数占位，TransformSet 引用时传 FROM/TO
      value: ${TO}
```

> **finding #1 / T1（2026-05-31 修订）**：原 v1.1 用 `data.spec` 与 **ADR-003** 的"resourceModifier CM 必须 `len(Data)==1`, 且 key=`rules.yaml`"严格要求冲突, Velero 会拒绝。本节统一改为 `data.rules.yaml`。**依赖 ADR-003 修订**（另外 agent 后台改架构设计.md）。同步迁移见 Phase 0。

**(b) 3 个 builtin TransformSet 示范**（"成套菜单", 客户可 Clone 改）

| Name | 引用的 Transform（按序） | 适用场景 |
|---|---|---|
| **bundle-azure-to-onprem** | strip-nodeport → change-storage-class(`managed`→`csi-hostpath`) → change-docker-registry(`acr.io/*`→`harbor.local/*`) | Azure AKS 还原到本地集群 |
| **bundle-cross-region-cleanup** | strip-loadbalancer-ip → strip-clusterip → strip-pv-binding | 同云跨区域还原（剥离绑定 IP/PV） |
| **bundle-dr-drill-scale-down** | scale-deployment(replicas=1) → strip-nodeport | DR 演练沙箱（最小占用） |

builtin TransformSet K8s 表示（reference 模型, 非 embed）：
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: bundle-azure-to-onprem
  namespace: supkube
  labels:
    supkube.io/kind: transform-set     # 区别于 transform
    supkube.io/builtin: "true"
data:
  spec: |
    transformRefs:                     # 🆕 引用列表，按序执行
    - name: strip-nodeport
    - name: change-storage-class
      params: { FROM: "managed", TO: "csi-hostpath" }
    - name: change-docker-registry
      params: { FROM: "acr\\.io/(.*)", TO: "harbor.local/$1" }
```

> **设计说明**：
> - **3 个 bundled TransformSet 是"教学示范 + 起手式"**, 给客户看"这个东西怎么用" —— 客户 95% 场景会 Clone 后改, 不会原样用
> - **大头是 11 个 Transform 模板** —— 客户拼自己的 TransformSet
> - `params` 参数占位用 `${VAR}` 在 Transform spec 里, TransformSet 引用时通过 `params` 注入 —— 让 Transform 真正可复用

#### 4.1bis TransformSet → Velero resourceModifierRef 编译契约（finding #1 / T1, 2026-05-31 新增）

**问题背景**：本 PRD 的 TransformSet 是 SupKube 抽象（reference + params 容器）, Velero `Restore.spec.resourceModifier` 期望的是**单个 ConfigMap, `len(Data)==1`, key=`rules.yaml`, value 为合并后完整 rules**（见 ADR-003 修订版）。两者中间必须有**编译步骤**。

**编译规则**（Trigger Restore 时执行, 一次性）：
1. 取 TransformSet 的 `transformRefs[]`（已按客户拖拽顺序）
2. **按顺序**展开每条 `transformRef` → 取对应 Transform 的 `rules.yaml`, 用 `params` 做 `${VAR}` 替换
3. **拼接**为单一 rules list, 保留**顺序**（Velero 同 path 后者覆盖前者, 符合本 PRD §4.5 的"同 path 后者覆盖"实际语义）
4. 生成临时 ConfigMap `supkube-restore-rm-<restoreName>-<short-hash>` 写入 supkube namespace, 内容:
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: supkube-restore-rm-<restoreName>-<hash>
     namespace: supkube
     labels:
       supkube.io/managed-by: transform-set-compiler
       supkube.io/source-transform-set: <tsName>
       supkube.io/for-restore: <restoreName>
   data:
     rules.yaml: |    # ← 唯一 key, len(Data)==1
       version: v1
       resourceModifierRules:
         - <展开后 rule 1>
         - <展开后 rule 2>
         - ...
   ```
5. 在 Restore CR 注入 `spec.resourceModifier.kind=ConfigMap` + `name=<compiled cm name>`
6. Restore 完成后, Compiled CM 不立刻删（保留 7 天供审计 / 重跑）, 由 GC 控制器按 label 清理

**preview-resolution 端点行为**: §4.3 的 `POST /transform-sets/preview-resolution` 输出**即此编译结果**（compiled CM 的 `rules.yaml` 内容预览), 让客户 Trigger 前 100% 看到"实际给 Velero 的是什么"。

**依赖**: ADR-003 修订（另外 agent 后台改架构设计.md）将此契约固化为架构级决策。

#### 4.2 使用统计机制（两层）—— **finding #3 (2026-05-31) 重选机制**

> **风险背景**：原 v1.1 用 ConfigMap annotation + CAS 写入做 applied-count。**finding #3** 指出: 高并发 Restore 场景（多个 Restore 同时跑 + 各引用同一 TS）会触发 K8s API server CAS 写冲突重试风暴 (HTTP 409 Conflict + retry-loop), 既污染 audit log 又拖垮 controller。

**(a) TransformSet 层** —— 采用**事件流 + 异步聚合**（v1 必做）:
- 每次 Restore 创建时, 后端 emit 一个 K8s Event (`reason=TransformSetApplied`, `involvedObject=<tsCM>`, `note=<restoreName>`) —— **写 Event 是无 CAS 单调的, 无冲突风暴**
- 一个独立的 `transform-stats-aggregator` goroutine 定期（默认 60s）扫 supkube namespace 内 `reason=TransformSetApplied` 的 Event, 按 TS 聚合后**批量**回写到 TS annotation:
  - `supkube.io/applied-count`：聚合自 Event 流, **最终一致, 不保证精确**（明确标注）
  - `supkube.io/last-applied-at`：最近 Event 时间
  - `supkube.io/last-applied-by`：最近 Event 的 restore 名
- 备选: 若客户集群 Event Backend 不可靠（如关闭 etcd events 持久化）, 启用独立 counter ConfigMap fallback（cluster-scoped, 写时 leader-elected single-writer 避免 CAS 风暴）

**(b) Transform 层** —— 引用计数 + 最近使用:
- `supkube.io/referenced-by-count`：被多少个 TransformSet 引用（CRUD TransformSet 时同步**Reconcile** 而非 inline 写, 减小竞争窗口）
- `supkube.io/transitively-applied-count`：通过 TransformSet 间接 Restore 触发, 由同一 aggregator goroutine 联动累加（最终一致, 不保证精确）

> **幂等**：Event 流天然防双写（即使 Restore 多次 reconcile, aggregator 按 `(restoreUID, tsName)` 去重）

> **Q1 重决（v1.2）**：annotation **仅作 UI 展示** + 标注"最终一致, 大集群下可能滞后 60s"; 精确账户由 Event 历史回放保证（审计场景）。**升级到 v1 必做**, 之前"超大集群（10k+）再考虑"的承诺过乐观, 现提前到 v1。

#### 4.3 API 增强

| Method | Path | 说明 |
|---|---|---|
| GET | `/transforms` | 🆕 列出所有 Transform（builtin + user-defined）, 带 `referencedByCount` + `transitivelyAppliedCount` + `category` |
| POST | `/transforms` | 🆕 创建自定义 Transform |
| POST | `/transforms/derive-from-conflict` | 🆕 从 PRD-001 v2 冲突一键派生 Transform 草稿（不直接保存, 返回 draft body） |
| GET | `/transform-sets` | 列出 TransformSet, 带 `appliedCount` + `lastAppliedAt` + `transformRefs` 详情（展开 transform 名） |
| POST | `/transform-sets` | 创建 TransformSet（body 含 `transformRefs[]` + 顺序 + params） |
| PUT | `/transform-sets/:name` | 更新（支持重排 transformRefs + 改 params） |
| POST | `/transform-sets/preview-resolution` | 🆕 给定 transformRefs[] + params, 返回**展平后的 effective patch list** 供 UI 预览（让客户在确认前看清楚"实际会发生什么"——"谁配置, 谁负责"的技术抓手） |

`derive-from-conflict` 请求体（PRD-001 v2 跳过来）：
```json
{
  "conflictKind": "StorageClassMissing",
  "payload": { "missingSCName": "csi-hostpath", "driver": "disk.csi.azure.com" }
}
```
返回：一个 draft **Transform** JSON（建议 name 如 `adapt-sc-csi-hostpath-<short-hash>`, category=storage, 预填 patch + 参数占位）, 前端编辑器让客户改名/调整/保存 → 然后**单独**在 TransformSet 页把这个 Transform 加入到某个 TS（或新建 TS）。

#### 4.3bis 参数模板安全 + 引用顺序语义（finding #4 + #5, 2026-05-31 新增）

**(a) `${VAR}` 模板校验（finding #4）**：

- **regex/捕获组校验**：Transform `data.rules.yaml` 中的占位符必须匹配 `\$\{[A-Z][A-Z0-9_]{0,31}\}` (大写字母/下划线/数字, 首字大写, 长度 ≤ 32)
- **客户在 TransformSet `params:` 注入值时**, 后端做**危险字符拒绝**:
  - 拒绝列表: `$`, `` ` ``, `\n`, `\r`, NULL byte (`\x00`), 单 `'` / 双 `"` 在 value 内**未转义**
  - regex value 必须先用 `regexp.Compile` 试编译, 编译失败拒绝
- **preview-resolution 端点**: 返回 effective rules.yaml 同时输出一份 **diff view**:
  ```
  - path: /spec/storageClassName
  - regex: ^managed$           ← 原 Transform 占位 ^${FROM}$
  + value: csi-hostpath        ← 替换自 params.TO
  ```
- 客户在 UI 看到 diff → 显式确认 → 才能保存 / Trigger（实现"谁配置, 谁负责"的技术抓手）

**(b) TransformSet 引用顺序语义（finding #5）**：

- TransformSet 内 `transformRefs[]` 顺序 == 编译后 rules 在 Velero `rules.yaml` 中的顺序
- **Velero 实际语义**：对同一 JSON path 多条 rule, **后者覆盖前者**（不是合并, 不是报错）
- UI 显示"按序执行"时, 必须文案中明示"**同 path 后者覆盖**, 顺序敏感"
- preview-resolution 给前端的输出**反映真实执行顺序** —— rule 列表按 transformRefs 展开顺序; 若不同 transformRefs 触及同 path, **diff view 高亮"被后者覆盖"的行**, 让客户在保存前看清楚自己拼出来的 TS 实际行为是不是想要的

#### 4.4 前端 UI 改造（两个页面）

**(a) 新建 `Transforms.vue`**（Transform 模板库, 之前没有）
- 顶栏分类 chip：`全部(11+) · storage · network-ingress · image-registry · scale · cleanup`
- 列：Name · Category · Subject (resource) · OpCount · ReferencedBy · LastUsed · Builtin?
- 行操作：View YAML / Clone / Edit（builtin 仅 Clone） / Delete（仅 user-defined 且无引用）
- "从冲突派生"入口：识别 query `?fromConflict=&payload=` → 进编辑器（预填 patch）→ 用户改名 → 保存

**(b) `TransformSets.vue` 改造**（已有页面增强）
- 顶栏：`全部(3+) · [+ New TransformSet]`
- 列：Name · TransformCount · TransformChain（按序徽章, 例如 `① strip-nodeport → ② change-sc → ③ change-registry`）· AppliedCount · LastApplied · Builtin?
- **新建/编辑 TransformSet 抽屉**：
  ```
  ┌─ New TransformSet ────────────────────────────┐
  │ Name: [____________________________________]   │
  │                                                │
  │ ── 引用的 Transforms（按序执行, 可拖拽重排）── │
  │  ⋮⋮ ① [strip-nodeport ▾]            [×]      │
  │  ⋮⋮ ② [change-storage-class ▾]      [×]      │
  │       params: FROM=[managed___] TO=[hostpath_]│
  │  ⋮⋮ ③ [change-docker-registry ▾]    [×]      │
  │       params: FROM=[acr\.io/(.*)] TO=[harbor/$1]│
  │  [+ Add Transform from Library ▾]              │
  │                                                │
  │ [Preview Effective Patches]  [Cancel] [Save]   │
  └────────────────────────────────────────────────┘
  ```
- **Preview Effective Patches** 弹窗：调 `/transform-sets/preview-resolution` 显示展平后的 JSON Patch list, 让客户**确认**

**(c) RestoreDrawer 集成**（与 PRD-001 v2 衔接）
- Restore 抽屉里"TransformSet"字段：select 已有 + "Create new TransformSet" 内联跳转
- 选中后展示 read-only TransformChain 徽章 + "Click to preview" → 弹 Preview Effective Patches
- **客户必须勾选 ☑ "I confirm the above transforms will run in this order on my Restore"** 才能 Trigger（实现"谁配置, 谁负责"）

### 5. UI / UX

**(a) Transforms 页（新增）—— "砖块库"**

```
┌─ Transforms (模板库) ─────────────────────────────────────┐
│ [+ New Transform]                            [Search...]   │
│                                                            │
│ 分类: [全部 11] [storage 1] [network-ingress 3]            │
│       [image-registry 1] [scale 1] [cleanup 5]             │
│                                                            │
│ Name                  Cat        Subject        Refs  Used │
│ ──────────────────────────────────────────────────────────│
│ ⚙ strip-nodeport      cleanup    services         3   12  │
│ ⚙ change-storage-class storage    pvcs              2    8 │
│ ⚙ change-docker-reg   img-reg    deployments       1    3 │
│ ⚙ adapt-sc-hostpath   storage    pvcs              1    3 │
│   (user-defined, 派生自 #fromConflict=SC)                  │
└──────────────────────────────────────────────────────────────┘
```

**(b) TransformSets 页 —— "成套菜单"**

```
┌─ TransformSets (成套方案) ────────────────────────────────┐
│ [+ New TransformSet]                         [Search...]   │
│                                                            │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ 📦 bundle-azure-to-onprem · [Built-In]               │   │
│ │   ① strip-nodeport → ② change-sc → ③ change-registry │   │
│ │   3 transforms · 应用 7 次 · 最近 1h 前               │   │
│ └──────────────────────────────────────────────────────┘   │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ 📦 my-prod-to-staging · 客户自建                      │   │
│ │   ① strip-clusterip → ② change-sc(custom)            │   │
│ │   2 transforms · 应用 24 次 · 最近 5min 前            │   │
│ └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

**(c) Restore 抽屉里的 TransformSet 选择 + 确认 (PRD-001 v2 + 本 PRD 衔接点)**

```
┌─ Restore: ns=demo to cluster B ──────────────────────────┐
│ ...                                                       │
│ TransformSet: [bundle-azure-to-onprem ▾]   [+ New...]    │
│                                                           │
│ 将按此顺序执行 (点击 Preview 看实际 patch):              │
│   ① strip-nodeport       (services)                       │
│   ② change-storage-class (pvcs: managed → csi-hostpath)   │
│   ③ change-docker-registry (deployments: acr → harbor)    │
│   [🔍 Preview Effective Patches]                          │
│                                                           │
│ ☑ 我确认以上 transforms 将在我的 Restore 上按序执行       │
│                                                           │
│ [Cancel]                              [Trigger Restore ▶] │
└──────────────────────────────────────────────────────────────┘
```
> ⚠ Trigger 按钮在勾选确认前 disabled —— 强制"客户确认"环节

### 6. Out of Scope

- **不**做跨集群 TS 同步（v2 或 Marketplace 时考虑）
- **不**做 TS 版本历史（每次保存覆盖）；版本管理留 v2
- **不**做 TS 的批量 import / export YAML 文件（v0.9.x 末考虑）
- **不**改现有 Clone / Edit / Delete / Built-In 保护行为

### 7. 非功能性要求

| 维度 | 要求 |
|---|---|
| 幂等 | 使用计数 CAS 写入，重复写不重复计数 |
| 内置保护 | builtin 不可 Edit / Delete（已有） |
| schema 兼容 | `supkube.io/builtin-version` label 留升级位 |
| 性能 | TS 列表 < 1s（500 个 TS 内）；统计查询不阻塞主列表渲染 |
| i18n | 5 类 category 名、"使用 N 次" 等都走 i18n |

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | 11 个 builtin **Transform** + 3 个 builtin **TransformSet** 在新集群启动后 seeded 完成 |
| 2 | Transform CR/CM 带 `supkube.io/kind=transform` + `transform-category` + `builtin-version` label |
| 3 | TransformSet CR/CM 带 `supkube.io/kind=transform-set` + `transformRefs[]` 数据结构 |
| 4 | `GET /transforms` 返回 `referencedByCount` + `transitivelyAppliedCount` + `category` |
| 5 | `GET /transform-sets` 返回 `appliedCount` + `lastAppliedAt` + 展开后的 `transformRefs` 详情 |
| 6 | 创建引用某 TS 的 Restore → 该 TS appliedCount +1 + 被引用的所有 Transform transitivelyAppliedCount +1（不重复计数） |
| 7 | `POST /transforms/derive-from-conflict` 返回 draft **Transform** JSON |
| 8 | `POST /transform-sets/preview-resolution` 展平 transformRefs[] + params → 返回 effective patch list |
| 9 | TransformSet 编辑抽屉支持**拖拽重排** transformRefs + 编辑 params |
| 10 | RestoreDrawer 集成: 必须勾选"我确认"才能 Trigger（disabled 状态可见） |
| 11 | 从 PRD-001 v2 跳转的 `?fromConflict=...` 进入 **Transform** 派生编辑器（不直接进 TransformSet） |
| 12 | 现有 4 个旧 builtin（strip-*）从 TS 重塑为 Transform 时**不影响**已使用它们的客户场景（迁移脚本兼容） |
| 13 | i18n 完整（en + zh-CN）, 含 "Transform" "TransformSet" "Effective Patches" "I confirm" |
| 14 | 前端 `npm run build` 通过; 后端 `go build ./...` + 单测无新增回归 |
| 15 | **finding #1 / T1**: Transform CM 严格 `len(Data)==1` + key=`rules.yaml`（test: 启动期 seeder 跑完后 `kubectl get cm -l supkube.io/kind=transform -o jsonpath='{.items[*].data}'` 检查所有 builtin 仅含 rules.yaml）|
| 16 | **finding #1 / T1 编译契约**: Trigger Restore 时生成 compiled CM (`supkube-restore-rm-<name>-<hash>`), 其 `data.rules.yaml` = 展平后的 effective rules 列表, 顺序 = transformRefs 顺序（test: 给 TS 含 2 个 transform 各 1 rule, 触发 Restore, 校验 compiled CM 的 rule 顺序与 UI preview 一致）|
| 17 | **finding #2 Phase 0 迁移**: 旧 4 个 strip-* builtin 从 TS 重塑为 Transform, **独立迁移测试** + **回滚剧本** + **迁移前后对照**（test: 起一个 v0.8.x 集群, 升级到 v0.9.x → 跑 strip-nodeport 场景验证不破; 加 ADR-014 启动时迁移机制 migrateBrokenTransformSets 先例) |
| 18 | **finding #3 统计机制**: 高并发场景（50 个 Restore 并发引用同一 TS）下, K8s API server 不出现 CAS write-conflict storm（test: prometheus `etcd_request_duration_seconds{operation=update}` p99 不抖增 + `kubectl get events --field-selector reason=TransformSetApplied` 50 条齐全, annotation 60s 内最终一致）|
| 19 | **finding #4 ${VAR} 校验**: regex/捕获组正确编译, 危险字符（`$`, 反引号, `\n`, NULL byte）注入被拒绝 + preview-resolution 显示替换 diff（test: 注入 `params={TO: "x;rm -rf /"}` 应返回 422 + reject 原因）|
| 20 | **finding #5 引用顺序**: preview UI 反映真实执行顺序; 同 path 多 rule 在 diff view 标记"被后者覆盖"行（test: 给 TS 含 rule A (path=/spec/sc, value=x) + rule B (path=/spec/sc, value=y), preview 必须显示 A 灰底"被覆盖" + B 高亮"final"）|

### 9. 任务拆分

**Phase 0 — 模型迁移（必做前置 + finding #2 升级 2026-05-31）**
- 旧 4 个 builtin TS（strip-*）拆为 Transform；建 3 个 builtin TransformSet 引用旧 transforms（向后兼容）
- 迁移脚本：把 `supkube.io/kind=transform-set` 的 4 个旧对象重塑 → 加上 `kind=transform` label, 移除原 spec.json 改为 transformRefs（**同时**：`data.spec` → `data.rules.yaml` per finding #1）
- **新增 (finding #2)**: 独立迁移测试 + 回滚剧本 + 迁移前后对照
  - 独立测试: 起 v0.8.x 单独集群, 跑 migration → 验证 4 个旧 strip-* 老客户场景仍能 Restore（test fixture: 真实抓取 v0.8.x customer 集群 TS yaml 入 git）
  - 回滚剧本: `supkube migrate transform-sets --rollback`（重写 label 反向）, 文档化前置条件 + 失败回滚步骤
  - 迁移前后对照报告: dry-run mode 输出"将要修改的对象 / 字段变化", 客户运维显式审批
  - 纳入 ROADMAP **迁移影响评估** 门禁（与 ADR-014 启动时迁移 `migrateBrokenTransformSets` 同一机制, 复用其先例）

**Phase 1 — 后端 seeder + 14 builtin（11 Transform + 3 TransformSet）**
- 7 个新 Transform 的 patch yaml（含 `${VAR}` 参数占位）
- 3 个 bundled TransformSet 示范
- 完整 label 矩阵

**Phase 2 — 两层使用统计**
- TransformSet annotation 写入（Restore Create handler）
- Transform transitivelyAppliedCount 联动累加
- 双计保护
- API 返回 stats

**Phase 3 — derive-from-conflict + preview-resolution 端点**
- `/transforms/derive-from-conflict`
- `/transform-sets/preview-resolution`（展平 transformRefs + params → effective patches）

**Phase 4 — 前端两个页面 + RestoreDrawer 集成**
- `Transforms.vue` 新建
- `TransformSets.vue` 重构（拖拽 + params 编辑）
- RestoreDrawer 加 TransformSet 选择 + 确认 checkbox
- i18n + build

**Phase 5 — 11 个 Transform 的实际 patch yaml 完善**
- 每个由我起草, Mars 评审（Q4）

### 10. 关联文档与任务

- **PRD-001 v2**：必须先评审通过本 PRD，PRD-001 v2 才能开 Phase 1
- **#104**：原 CSI 一键适配，现拆为 PRD-001 v2 + 本 PRD
- **#114**：本 PRD 的实施 task（已建）
- **ADR-031**：5 层数据韧性
- **现有文件**：
  - `supkube-backend/internal/api/v1/transform_sets_seed.go`：seeder
  - `supkube-frontend/src/views/TransformSets.vue`：列表页
  - `supkube-frontend/src/components/RestoreDrawer.vue`：将通过 PRD-001 v2 跳转过来

### 11. 开放问题

| Q | 问题 | 倾向 |
|---|---|---|
| Q1 | 使用统计用 annotation 还是独立 counter? | **annotation**（简单 + 幂等 + 不引入新 CR） |
| Q2 | "从冲突派生"是否要让用户改名保存? | **是**（派生 = draft Transform, 命名 + Save 才入库, 然后单独加入 TransformSet） |
| Q3 | 5 个分类是固定 enum 还是开放 tag? | **v1 固定 5 类**（enum 校验）; v2 允许自定义 tag |
| Q4 | 11 个 Transform 的实际 patch yaml 谁来写? | 我起草 each, Mars 评审（可分批合并） |
| Q5 | TS / Transform 的版本历史要不要做? | v1 不做（覆盖式 update）, v2 加 versioning |
| Q6 🆕 | **参数化机制选 `${VAR}` 模板还是 JSON Schema?** | **倾向 `${VAR}` 简单模板**（实现 1 天）; 真做企业级再上 JSON Schema |
| Q7 🆕 | **是否支持"匿名 inline TransformSet"**（Restore 时不命名直接拼一组 Transform 用完即弃）? | **不支持**（v1）—— 强制命名 + Save 才能用, 倒逼客户沉淀资产, 也符合 Kasten 行为 |
| Q8 🆕 | **"我确认"勾选要不要持久化**（避免老用户每次重勾）? | **每次必勾**（v1）—— Restore 不是高频操作, 勾选成本低; 跟 "谁配置, 谁负责"原则一致 |
| Q9 🆕 | **builtin Transform 重命名**（之前叫 strip-nodeport 是个 TS, 现在是个 Transform）是否破坏老客户 API?  | 兼容方案: 同名保留, 但 `supkube.io/kind` label 从 transform-set 改为 transform; 同时 seed 3 个 builtin TS 把 4 个老的引用回去 → 老客户 Restore 流程不破 |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-30 | Claude | — → 草稿 | 由 PRD-001 评审反馈派生 |
| 2026-05-30 | Claude | 草稿 → **排队评审** | 全细节完成；与 PRD-001 v2 同步交付；待 Mars 评审 Q1-Q5 + 整体方向 |
| 2026-05-30 | Mars | 排队评审 → **改正中** | **概念模型反馈**：v1 把 11 个对象都画成 TransformSet 是错的。Transform Set 实际是**多个 Transform 的引用容器**, 可包含改 Ingress + SC + Docker Registry 多个原子操作, 客户可调整顺序, 客户定义并确认（"谁配置, 谁负责"）, Set 对 Transform 是**引用**关系。让我读 Kasten 文档 https://docs.kasten.io/8.0.2/usage/transformsets/ 后修订。 |
| 2026-05-30 | Claude | 改正中 → **改正中 (v1.1 完成, 待重交)** | 读完 Kasten Transform Sets + Transform API 两份文档, 确认两层模型: Transform=原子（subject + json ops, 支持 regex）, TransformSet=有序引用容器。修订: §0 新增二层模型表 + 设计原则; §3 拆 User Stories 为 3.1/3.2; §4.1 11 builtin Transform + 3 bundled TransformSet + YAML schema; §4.2 两层统计; §4.3 新增 /transforms + preview-resolution 端点; §4.4 新建 Transforms.vue + 重构 TransformSets.vue + RestoreDrawer 确认 checkbox; §5 UI mockup 改为两个页面 + Restore 确认环节; §8 DoD 扩到 14 项; §9 加 Phase 0 迁移; Q6/Q7/Q8/Q9 新增。**待 Mars 重审 → 排队评审 → 已评审**。 |
| 2026-05-30 | Mars | 改正中 → **✅ 已评审** | v1.1 通过。可进研发中（与 PRD-001 v2 同步, 一起 kick off）。 |
| 2026-05-31 | Mars (评审人 Claude 委托) | 已评审 → **改正中 (v1.2 修订)** | 落 5 个评审 finding: (1) **finding #1 / T1** 编译契约 — `data.spec` → `data.rules.yaml` (ADR-003), 新增 §4.1bis "TransformSet → Velero resourceModifierRef 编译契约" (transformRefs[] + params 编译为单一 CM, len(Data)==1); preview-resolution 输出即编译结果, 顺序对齐 Velero "同 path 后者覆盖"; **依赖 ADR-003 修订**（后台 agent 改架构设计.md）; (2) **finding #2** Phase 0 破坏性迁移 — 加独立迁移测试 + 回滚剧本 + 迁移前后对照, 纳入"迁移影响评估"门禁, 复用 ADR-014 启动时迁移机制 (migrateBrokenTransformSets 先例); (3) **finding #3** 统计机制重选 — annotation+CAS 高并发会触发 K8s API server 409 风暴, 改为 Event 流 + 异步聚合 goroutine, annotation 仅作 UI 展示标 "最终一致, 不保证精确"; 升级到 v1 必做（之前"超大集群再考虑"过乐观）; (4) **finding #4** `${VAR}` 模板加 regex/捕获组校验 + 危险字符拒绝 + preview-resolution 输出替换 diff; (5) **finding #5** TransformSet 引用顺序对齐 Velero "同 path 后者覆盖" 实际语义, UI preview 反映真实执行顺序 + diff view 高亮被覆盖行; DoD 由 14 项扩至 20 项 (#15-#20 落 finding 测试)。**等 Mars 重审 → 排队评审 → 已评审**。 |
| 2026-05-31 | Claude | 已评审 → **已评审 v1.3** | 加第 12 transform redirect-external-endpoints-to-sandbox (PRD-007 §4.7 联动, #132 follow-up) |

---

<a id="prd-003"></a>
## PRD-003 — AI Advisor inside SupKube（内嵌灾备顾问 · 推荐型 · 非自治）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-003 |
| **任务编号** | #115 |
| **状态** | **已评审（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.10.x |
| **关联 ADR** | ADR-031（5 层数据韧性）+ ADR-033（拟）AI Advisor 架构 (PRD-003 Engine + PRD-004 Engine 共用) |
| **关联 PRD** | PRD-001 v2（Restore Preflight, 落地点）/ PRD-002 v1.1（Transform 推荐内容） / PRD-004（MCP Server, 共享 Engine） |
| **参考决策** | Mars 与外部专家关于产品方向的对话（2026-05-30 飞机上）, 见架构图 `AI Advisor + OpenClaw + MCP` |

> **产品方向决策摘要（2026-05-30）**：
> 经评估, "全自治 AI 灾备 Agent"路线在我们当前阶段**风险过大**（合规黑盒、责任承担、6 个月带宽占用、与 PRD-002 "谁配置, 谁负责"原则冲突）。
> 战略路径分为三段:
> - **Phase A**（本 PRD）= **AI Advisor inside SupKube**: 内嵌"推荐型"灾备顾问, **只读建议, 绝不自动执行**, 客户点击"应用建议"才弹出现有 Wizard 预填。
> - **Phase B**（PRD-004 待立）= **MCP Server (Supkube Skills)**: 暴露能力到客户的 AI Agent 生态（含 OpenClaw、Claude、Copilot 等）, 用**意图级**接口而非原子级。
> - **Phase C**（v2.0+, 不入当前 Roadmap）= **选择性自治**: 仅 dev/test 环境, 仅白名单操作, 必须有客户基数 + 恢复数据飞轮 + 合规审计模板。
>
> **OpenClaw 定义澄清（Mars 拍板 2026-05-30）**: OpenClaw（"小龙虾"）是**客户侧的 LLM Agent 框架/平台**（类比 LangChain / Dify / Coze / n8n+AI 那一类）, **不是 SupKube 同公司产品, 不是 SupKube 组件, 也不是我们的开源参考实现**——它是**客户家里"养的"AI 工作流引擎**, 由客户自主部署运维。本 PRD 不涉及 OpenClaw 内部实现; SupKube 仅通过 PRD-004 的 **MCP Server "Supkube Skills"** 让 OpenClaw（以及任何其他 MCP-compatible Client）能调用 SupKube 能力。**此定位影响 Skills 开源策略**: 必须开源 (BSD/Apache-2.0), 否则客户接入摩擦过大。

### 1. Goal

把 SupKube 从"规则式自动化的灾备工具"升级为"内嵌灾备专家的智能平台"——通过**只读、可解释、人确认**的 AI Advisor, 在 Restore Preflight / Backup 创建 / Application 风险评估 三大场景给客户**专业建议**, 让客户在 5 秒内拿到一个资深灾备 SRE 的判断, 但**最终决策与责任始终在客户**。

### 2. Epic

ADR-031 的"全职灾备运维专家"Epic ——本 PRD 把这个"专家"做成**内嵌产品形态**, 但严格限定在**顾问**角色, 不越界到执行者。

### 3. User Stories

#### 3.1 Restore Preflight 场景（核心起点, 对接 PRD-001 v2）
- 作为系统管理员, 我在 RestoreDrawer 看到 Preflight Checklist 有一项 "⚠ StorageClass `csi-hostpath` not found", 旁边出现**蓝色 💡 AI 建议**按钮 → 点击 → 弹出 Advisor 卡片: **"建议使用 `change-storage-class` Transform, 把源集群的 `managed` 映射为目标集群的 `csi-hostpath`。理由: 检测到源集群是 Azure Disk CSI, 目标集群是 K3s/hostpath, 这是已知跨云场景。下面是预填好的 Transform 参数, 点击下方按钮派生。"** → 我点 "派生 Transform" → 进 Transform 编辑器（预填）→ 命名 → 保存 → 加入 TransformSet → 回 Restore 重检。
- 作为系统管理员, 当 Checklist 全 ✓ 时, Advisor 给一段总结建议: **"本次 Restore 跨 Azure → K3s, 建议: ① 用 TransformSet `bundle-azure-to-onprem`; ② 还原后跑业务 smoke test; ③ 7 天后做一次 DR Drill 验证 RTO"** —— 帮客户**看清整个动作的全貌**而不只是"按钮能不能点"。

#### 3.2 Backup 创建场景
- 作为系统管理员, 在 Policy Wizard 填到一半（选了 namespace, 没选 Retention/加密/BSL）, 右侧出现 Advisor 卡片: **"检测到 ns=prod-orders 包含 MySQL StatefulSet, 建议: Retention ≥ 30 天 / 启用加密 / 选择 Local+Cloud 双 BSL 满足 3-2-1。点击下方按钮一键填入。"** —— 一键填充, 客户可改。

#### 3.3 Application 风险评估场景
- 作为系统管理员, 在 Applications 页, 每个 ns 行多一列 **Resilience Score**（0-100 + 颜色徽章 + hover 显示理由）。点击进 Application 详情, 新增 **"AI 建议" tab**, 展示完整评分 + 5 维度分解（Business Value / Architecture / Protection / Security / Operation）+ 风险点列表 + 改进建议（每条建议都带"应用建议"按钮跳到对应 Wizard 预填）。
- 作为评审/管理员, 仪表板新增 **Resilience Posture 卡片**: 集群整体平均 Score + 风险等级分布 + Top 5 高风险 ns —— 给"高层视角"。

#### 3.4 透明度与信任（贯穿全场景）
- 作为系统管理员, 每个 AI 建议旁都有 **"为什么"链接** → 弹出"AI 推理过程": 引用的知识库条目 + 检测到的事实 + 推导链 —— **可解释 + 可质疑**, 不是黑盒。
- 作为系统管理员, 在 Settings 里能**关闭 AI Advisor**（隐私敏感客户场景）或**切换 LLM Provider**（BYO API Key: DeepSeek / Claude / GPT / Azure OpenAI / 本地 Ollama）。

### 4. Functions

#### 4.1 架构: 一个引擎, 两个出口（与图对齐）

```
┌─────────────────────────────────────────────────────────────┐
│        SupKube Advisor Engine (Phase A 核心)                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ LLM Client (默认 Ollama 本地; DeepSeek/Claude/Azure opt-in)│
│  │ RAG Knowledge Base (灾备原理 + 应用特征 + 合规 + 反模式)│  │
│  │ Context Builder (从 K8s API + Velero CR 实时拼上下文) │   │
│  │ Prompt Templates (Preflight / Backup / Score 三套)   │   │
│  │ Result Validator (JSON schema + 兜底规则修正)        │   │
│  │ Audit Log (每次推理记录, 供事后追溯)                 │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
            ▲                                ▲
            │ 内部调用 (Phase A)               │ 外部调用 (Phase B, MCP)
   ┌────────┴────────────┐           ┌──────┴──────────────┐
   │ SupKube UI 集成      │           │ MCP Server          │
   │ - RestoreDrawer 💡  │           │ "Supkube Skills"     │
   │ - PolicyWizard 💡   │           │ assess_intent(...)   │
   │ - Applications Score│           │ recommend_policy(...) │
   │ - Dashboard 卡片    │           │ (PRD-004 v0.11.x)    │
   └─────────────────────┘           └─────────────────────┘
```

> **关键设计**: 同一个 Advisor Engine 服务**两面**, 0 重复实现。本 PRD 只交付 Engine + 内部 UI 出口; MCP 出口走 PRD-004。

#### 4.2 后端: Advisor Engine（新增 module `supkube-backend/internal/advisor/`）

| 子模块 | 职责 |
|---|---|
| `llm_client.go` | 抽象 LLM 接口, **默认 Ollama 本地** (合规客户), DeepSeek / Claude / OpenAI / Azure OpenAI 多 provider 显式 opt-in, 可热切换 |
| `rag_store.go` | 知识库存储（v1 用本地嵌入 + 内存索引 chroma-go; v2 可升 PGVector） |
| `context.go` | 拼接 K8s 资源 + Velero CR + 当前 BSL 状态 → 结构化 JSON 输入 |
| `prompts/*.tmpl` | 3 套 Prompt 模板: `preflight.tmpl` / `policy.tmpl` / `score.tmpl` |
| `validator.go` | LLM 输出 JSON schema 校验 + 兜底规则修正（不允许超 100 分等） |
| `audit.go` | 每次推理写一条 CR/CM 审计记录（Advisor inputs + outputs + LLM provider + 耗时 + 费用估算） |

#### 4.3 知识库（RAG）—— v1 范围

**v1 必交付 4 类知识条目**（每类 5-10 条, 总计约 30 条, Markdown 格式, 嵌入向量索引）:

| 类别 | 示例条目 |
|---|---|
| **灾备原理** | 3-2-1-1-0 / RTO-RPO 定义 / 数据一致性原则 / Object Lock 防勒索 |
| **应用特征** | MySQL/PG/Mongo/Redis/Kafka/etcd/Nginx 各 1 条（重要度 + 备份要求 + 一致性要求） |
| **合规** | 等保 2.0 三级 / GDPR 备份保留要求 / 金融行业 RPO 要求 |
| **架构反模式** | 备份与生产同 BSL / 单 PVC 单 AZ / 长期不演练 / 备份未加密 |

> **v2** 再扩到 100+ 条 + 客户自定义条目导入。

#### 4.4 API 设计

| Method | Path | 用途 |
|---|---|---|
| POST | `/advisor/preflight` | 输入: restore intent (源/目标集群 + ns), 输出: 每个 Preflight 冲突项的 AI 建议（含推荐 Transform 模板 + 参数） |
| POST | `/advisor/policy-recommend` | 输入: 部分填写的 Policy 表单 + ns 上下文, 输出: 推荐补全（Retention / 加密 / BSL 双写等） |
| POST | `/advisor/score` | 输入: ns 或 application, 输出: 0-100 评分 (**rule engine 计算, 复现性高, 非 LLM 输出**) + 5 维度分解 + 风险点 + **AI 生成的解释 + 改进建议** (LLM 仅在解释 / 建议层) |
| GET | `/advisor/score/:ns` | 缓存的最近评分（rule engine 输出可缓存; AI 解释独立缓存）|

> **finding #2 (2026-05-31)**：Score 数值**必须**由规则引擎计算（输入: ns 资源 + Backup history + Policy 配置）, 保证复现性 (同输入同输出); LLM **只生成"为什么是这个分 + 怎么改进"**的解释文字。UI 上必须显式标注 **"评分: 规则计算" vs "建议: AI 生成"** 两类来源, 让客户区分可信度。
| GET | `/advisor/audit` | 列出最近 N 次推理审计记录（含 input/output/cost）, 供合规审计 |
| GET | `/advisor/settings` | 当前 LLM provider / 是否启用 / 知识库版本 |
| PUT | `/advisor/settings` | 切换 provider / 启用关闭 / 切换知识库版本 |

**关键设计原则**:
- 所有 API 返回结构包含 `recommendation`（建议本身）+ `reasoning`（推理过程）+ `referenced_kb_entries`（引用的知识库条目 ID）+ `confidence_tier`（**finding #3 2026-05-31: 三档 enum: high / medium / low**, 不再返回 0-1 数字）+ `confidence_factors`（结构化: `kb_match_count`, `detected_fact_count`, `rule_engine_corroboration`）+ `apply_action`（前端可点击的"应用此建议"动作描述, 不是直接执行）。
- **绝无任何接口直接修改 Backup/Restore/Policy 资源** —— 所有 apply_action 都必须由前端弹出现有 Wizard 让客户确认。

> **finding #3 (2026-05-31)**: 数字置信度（如 92%）假装精确, 实际 LLM 无法可靠输出连续概率值, 容易让客户**过度信任** 87% vs 92% 的区别。改为 **high / medium / low 三档** + 显式列出"为什么是这一档"的事实依据（KB 条目数 + 检测事实数 + 规则引擎佐证 yes/no）。客户的信任来自**引用证据**, 不是数字。

#### 4.5 前端集成

| 位置 | 集成方式 |
|---|---|
| `RestoreDrawer.vue` | 每个 Preflight ✗/⚠ 项后加 💡 按钮 → 弹 Advisor 卡片 → 含"派生 Transform"按钮（跳 Transforms.vue + 预填, 对接 PRD-002 derive-from-conflict） |
| `Policies.vue` / 新建 wizard | 表单右侧固定一个 Advisor 卡片, 实时根据当前填写状态推荐补全 |
| `Applications.vue` | 行多一列 `Resilience Score`（颜色徽章）; 详情抽屉新增 "AI 建议" tab |
| `Dashboard.vue` | 新增 "Resilience Posture" 卡片: 集群平均 Score / 风险分布饼图 / Top 5 高风险 ns |
| `Settings.vue` | 新增 "AI Advisor" tab: 开关 / Provider 选择 / API Key 输入 / 知识库版本 / 审计日志查看 |

### 5. UI / UX

**(a) RestoreDrawer 中的 AI 建议卡片**

```
┌─ Restore Preflight: ns=demo to cluster B ────────────┐
│ ✓ Namespace 不存在冲突                                │
│ ✓ Backup 存在且未过期                                 │
│ ✗ StorageClass `csi-hostpath` not found on target    │
│   [→ 去 Transform 解决]  [💡 AI 建议]                 │
│   ┌──────── AI Advisor ─────────────────────────┐    │
│   │ 💡 建议: 使用 change-storage-class Transform │    │
│   │    映射 managed → csi-hostpath               │    │
│   │ 📖 理由: 源集群为 Azure Disk CSI, 目标为      │    │
│   │    K3s hostpath, 已知跨云场景, 数据卷可重建    │    │
│   │ 🔗 引用知识库: KB-021 (跨云 SC 映射模式)      │    │
│   │ 🎯 置信度: 高 (引用 KB-021 + 检测到 3 个相符事实)│   │
│   │ [派生 Transform (预填)]  [为什么?]  [忽略]    │    │
│   └─────────────────────────────────────────────┘    │
│ ✗ NodePort 30080 conflict                            │
│   ...                                                 │
└──────────────────────────────────────────────────────┘
```

**(b) Applications 详情中的 AI 建议 tab**

```
┌─ Application: prod-orders ───────────────────────────┐
│ [Overview] [Items] [Activity] [💡 AI 建议]            │
│                                                       │
│ Resilience Score: 62 / 100  [Medium Risk]            │
│                                                       │
│ 维度分解:                                             │
│   Business Value      ███████░░░  25/30 (高)         │
│   Architecture        ███░░░░░░░  8/20  (单 AZ)      │
│   Protection Strategy ████████░░  22/30 (无加密)     │
│   Security/Compliance █░░░░░░░░░  3/15  (不合规)     │
│   Operation           ████░░░░░░  4/5                │
│                                                       │
│ 风险点:                                               │
│  🔴 MySQL StatefulSet 单 AZ, 无跨区副本               │
│  🟡 Backup 未加密, 不符等保三级                       │
│  🟡 6 个月未做恢复演练                                │
│                                                       │
│ 改进建议:                                             │
│  1. 启用 Object Lock + AES-256 加密 [应用此建议 →]   │
│  2. 配置跨区 BSL 备份 [应用此建议 →]                  │
│  3. 排期月度 DR Drill [应用此建议 →]                  │
└──────────────────────────────────────────────────────┘
```

### 6. Out of Scope

| 项 | 原因 | 去向 |
|---|---|---|
| **MCP Server (Supkube Skills)** | Phase B, 单独 PRD | PRD-004 (v0.11.x) |
| **OpenClaw 产品** | 独立产品线, 团队/品牌/Repo 都不同 | 单独立项, 不归 SupKube |
| **自动执行（AI 一键 Apply）** | Phase C, 与 "谁配置, 谁负责" 冲突 | v2.0+ 评估 |
| **自动恢复演练 (DR Drill 自跑)** | 风险高, 沙箱方案未成熟 | v0.9.7 后单独立项 |
| **多 namespace 批量评分对比** | v2 体验增强 | v0.11.x |
| **客户自定义知识库** | v2 高级功能 | v0.11.x |
| **审计日志 SIEM 推送** | 复用现有审计模块即可, 不在本 PRD 重做 | — |

### 7. 非功能性要求 + **脱敏与外发治理（finding T4, 2026-05-31 新增）**

#### 7.1 非功能性要求

| 维度 | 要求 |
|---|---|
| **隐私** | LLM 调用前必须**脱敏**: Secret 值替换为 `***`, 镜像名保留但环境变量值脱敏, PV 数据**绝不**传 LLM。审计日志记录"什么字段被脱敏" |
| **BYO LLM** | 客户可选: **本地 Ollama (合规客户默认, v1 必做)** / SaaS 模式 (SupKube 提供 LLM Key, 入门版) / BYO Key (企业版, DeepSeek / Claude / Azure OpenAI 显式 opt-in) |
| **延迟** | 单次 Advisor 调用 P95 ≤ 5 秒（流式输出 P95 首字 ≤ 1.5 秒）; 列表场景批量调用走异步 + 缓存 |
| **成本** | 单次调用平均 ≤ 0.01 USD (DeepSeek BYO 场景); Ollama 本地零外发成本; 评分类调用走 24 小时缓存 |
| **降级** | LLM 不可达时, 退回**规则版评分**（现有逻辑）+ UI 显示 "AI Advisor 暂不可用" |
| **关闭** | Settings 可一键关闭, 关闭后所有 💡 按钮隐藏, 不影响主功能 |
| **i18n** | 中英双语 prompt + 输出语言跟随 UI 语言 |
| **审计** | 每次推理 100% 记录（input 摘要 + output JSON + provider + 费用 + 耗时）, 保留 ≥ 90 天 |

#### 7.2 脱敏与外发治理 (finding T4, 2026-05-31 新增, 依赖 SECURITY.md AI 专章 + ADR-033)

**默认策略**: **合规客户默认本地 Ollama**, 任何离开集群的字段必须显式列举 + 客户审批。

**外发字段清单（白名单）**：

| 字段类别 | 是否允许外发 | 处理 |
|---|---|---|
| K8s metadata (kind, name, namespace, label) | ✅ | 直传 |
| K8s spec 字段（容器配置/参数/资源）| ✅ | env value 脱敏为 `***` |
| Velero CR (Backup/Restore status) | ✅ | 直传 |
| Secret values | ❌ | 强制 `***`, 永不外发 |
| ConfigMap values | ⚠️ | 默认 `***`, 客户可在 Settings 显式 opt-in "把 ConfigMap 值发给 LLM"（仅极少数排错场景）|
| PVC 数据 / 卷内容 | ❌ | 永不外发 |
| 日志内容 (包含 stack trace / token / PII) | ⚠️ | 经 PRD-005 §7 脱敏管线（log redaction middleware）+ 本节统一治理, **不另做一套** |
| 用户身份 / 邮箱 / IP | ❌ | 强制 `***` |
| K8s Audit log / OIDC token | ❌ | 永不外发 |

**审计可观察**: 每次推理审计 entry 必含 `redacted_fields: [list of field paths]` + `outbound_byte_count` + `provider`, 客户合规官 1 次查询能 100% 复现"这次 AI 调用到底把什么发出去了"。

**Settings UI 强制可见**: AI Advisor tab 顶部必须显示当前 provider + "外发治理状态: ◉ 本地 Ollama 0 字节外发  /  ○ DeepSeek BYO 平均 X KB / 调用"。

**依赖**: SECURITY.md "AI 专章"（后台 agent 写中）+ ADR-033 (拟) AI Advisor 架构。本节即"外发治理统一管线", PRD-004 (MCP Server) / PRD-005 (Log Viewer AI 根因) / PRD-006 (Activity AI 排错 tab) 全部复用本节, **不另做**。

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | Advisor Engine module 在 `internal/advisor/` 编译通过, 单测覆盖率 ≥ 70% |
| 2 | LLM Client 支持至少 2 个 Provider (DeepSeek + Ollama 本地) 可热切换 |
| 3 | RAG 知识库种入 4 类 × 5+ 条 = 至少 25 条目, 嵌入索引可检索 |
| 4 | 3 个 API endpoint (`preflight` / `policy-recommend` / `score`) 都返回 schema-valid JSON |
| 5 | 脱敏机制: Secret values / env values 在审计日志中可验证为 `***` |
| 6 | RestoreDrawer 集成: 至少 3 种 Preflight 冲突类型（缺 SC / NodePort / 缺 PV）能弹出 AI 建议 |
| 7 | "派生 Transform" 按钮跳转到 Transforms.vue 时, 参数正确预填（对接 PRD-002）|
| 8 | Applications 页评分列 + 详情 AI 建议 tab 渲染正确, Score 与 risk_level 一致 |
| 9 | Dashboard Resilience Posture 卡片显示集群平均 Score + 分布 |
| 10 | Settings AI Advisor tab: 开关 / Provider 切换 / API Key 加密存储（K8s Secret） |
| 11 | 降级: 断开 LLM 后, UI 显示"暂不可用" + 主功能不受影响 |
| 12 | 审计日志: 100 次推理后, `GET /advisor/audit` 能列出 100 条, 含费用估算 |
| 13 | i18n: zh-CN + en 双语 prompt 各 1 套, UI 文案双语完整 |
| 14 | 端到端: 在 docker-desktop + AKS 跨集群 Restore demo 流程中, Advisor 给出至少 1 条有用建议 |
| 15 | 前端 `npm run build` 通过; 后端 `go build ./...` + 单测无回归 |

### 9. 任务拆分

**Phase A1 — Engine 骨架 (1-2 周)**
- module 结构 + LLM Client 抽象 + Provider 实现 (DeepSeek + Ollama)
- 上下文构建器 (从 K8s + Velero CR 拼 input)
- JSON schema 校验 + 兜底规则
- 审计日志机制

**Phase A2 — 知识库 + Prompt (1 周)**
- 起草 25 条 RAG 条目 (Mars 评审)
- 3 套 Prompt 模板 + few-shot examples
- 嵌入索引选型 (chroma-go vs PGVector)

**Phase A3 — Preflight 场景集成 (1 周, 对接 PRD-001 v2)**
- `/advisor/preflight` API 实现
- RestoreDrawer 💡 按钮 + Advisor 卡片
- "派生 Transform" 跳转链路 (对接 PRD-002)

**Phase A4 — Score 场景 (1-1.5 周)**
- `/advisor/score` + 缓存
- Applications 列表评分列
- AI 建议 tab + 5 维度可视化
- Dashboard Resilience Posture 卡片

**Phase A5 — Policy 推荐 (0.5 周)**
- `/advisor/policy-recommend` API
- PolicyWizard 侧栏 Advisor 卡片

**Phase A6 — Settings + 隐私 + 降级 (0.5-1 周)**
- Settings AI Advisor tab
- 脱敏机制端到端验证
- LLM 不可达降级体验

**Phase A7 — 联调 + 测试 + 文档 (1 周)**
- 端到端 demo 走查
- USER_MANUAL §AI Advisor 章节
- 测试用例.md TC-ADV-001 至 005

**总计 6-7 周**（一人全职, 或两人 3-4 周）

### 10. 关联文档与任务

- **PRD-001 v2** (Restore Preflight): 本 PRD Phase A3 直接对接
- **PRD-002 v1.1** (Transform 一等公民): "派生 Transform" 按钮链路
- **PRD-004** (待立, MCP Server): Phase B, 复用本 PRD 的 Advisor Engine
- **ADR-031**: 5 层数据韧性, Advisor 是 ADR-031 "全职专家"的产品化形态
- **ADR-033** (拟): AI Advisor 架构决策记录, 本 PRD 评审通过后正式立项
- **#115**: 本 PRD 实施 task
- **现有可复用**:
  - `supkube-backend/internal/api/v1/` REST API 骨架
  - `supkube-frontend/src/views/Applications.vue` / `Dashboard.vue` / `Settings.vue`
  - `supkube-backend/internal/policy/` 现有规则评分（作为降级 fallback）

### 11. 开放问题

| Q | 问题 | 倾向 |
|---|---|---|
| Q | 问题 | 倾向 | **Mars 拍板（2026-05-30）** |
|---|---|---|---|
| Q1 | OpenClaw 与 SupKube 关系？(a) 同公司不同产品 / (b) 同产品两组件 / (c) 客户侧 AI Agent 框架 | 待 Mars | ✅ **OpenClaw 是客户侧 LLM Agent 框架/平台**（类似 LangChain/Dify/Coze, 由客户部署运维, 与 SupKube 无任何产品归属关系）→ Skills **必须开源** 降低接入摩擦, MCP Server 对外措辞用"Compatible with OpenClaw / Claude / Copilot / any MCP Client" |
| Q2 | 默认 LLM Provider: DeepSeek / Claude / BYO only | DeepSeek 默认 + BYO | ✅ ~~DeepSeek 默认 + BYO Key~~ → **重决 2026-05-31 (finding T4)**: 合规客户**默认本地 Ollama** (v1 必做), DeepSeek / Claude / Azure OpenAI 等 SaaS 作为**显式 opt-in**（客户运维明确同意"我家数据出集群"才能切）|
| Q3 | 私有 Ollama 何时支持？v1 / v1.1 / v2 | v1.1 加 | ✅ ~~v1.1 加~~ → **重决 2026-05-31 (finding T4)**: **v1 必做**（合规客户默认即 Ollama, 不能延后）|
| Q4 | 评分缓存 TTL: 1h / 24h / watch 触发失效 | 24h + 手动重评 | ✅ **24h + 手动"立即重评"按钮** |
| Q5 | "为什么"展示形式: 推理链 / 知识库条目 / 两者 | 两者都展示, 默认折叠推理链 | ✅ **简版（KB 条目）+ 折叠详版（推理链）** |
| Q6 | 审计日志保留期 | 默认 90 天, 企业版 ≥ 180 天 | ✅ **默认 90 天, 企业版 ≥ 180 天** |
| Q7 | 失败建议反馈机制 | 记录结果反馈 → 建议质量指标 → prompt 调优 | ✅ **进"建议质量"指标, prompt 调优用** |
| Q8 | Advisor 自学习 | v1 不做, 留数据 | ✅ **v1 不做, 留数据** |
| Q9 | 与 #106 (RESTORE-ADVISOR) 任务关系 | 合并 | ✅ **合并** —— Advisor 部分收为本 PRD 子集; 但 #106 还含 "Restore Task Center" 是独立 UX 项, **建议拆分: Advisor 进 #115, Task Center 留 #106 但更名 RESTORE-TASK-CENTER 单独立项** |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-30 | Mars | 提出产品方向问题 | 与外部专家深度讨论"是否做自治 AI" + 给出架构图（OpenClaw + AI Advisor + MCP）|
| 2026-05-30 | Claude | 评估专家方案 | 同意 AI Advisor + MCP, 反对全自治路线（合规风险 + 责任承担 + 与 PRD-002 冲突 + 6 个月带宽占用）。建议 Phase A/B/C 三段路径 |
| 2026-05-30 | Mars | 决策 | Q1=C / Q2=B / Q3=B, 让 Claude 起 PRD-003 草稿 |
| 2026-05-30 | Claude | — → **草稿** | PRD-003 v1 完成, 含 §0-§11 + 验收标准 15 项 + 任务拆分 7 阶段 + 开放问题 Q1-Q9。**待 Mars 评审** |
| 2026-05-30 | Mars | 草稿 → **排队评审（重要不紧急）** | **整体评级: 重要, 不紧急**。Q1-Q9 全部拍板（见上 §11 表）。最关键决策: **OpenClaw = 客户侧 AI Agent 框架**（非同公司, 非组件, 非参考实现）→ Skills **必须开源**, MCP Server 对外表述应宽到"any MCP-compatible Client"。Mars 表示需要更长时间评审整体方向, 文档先沉淀, 不立刻进研发中。 |
| 2026-05-31 | Mars (评审人 Claude 委托) | 保持 **排队评审 (重要不紧急)** + 修订 | 落 3 个评审 finding (排队中预先修订, 不退回草稿): (1) **finding T4** 默认 LLM Provider 改为 **合规客户默认本地 Ollama (v1 必做)**, DeepSeek / Claude / Azure 等 SaaS 显式 opt-in; **§7 新增 7.2 "脱敏与外发治理"小节** 列举哪些字段会离开集群 (白名单), 客户合规官可审计; **依赖 SECURITY.md AI 专章 + ADR-033**（后台 agent 写中）; PRD-004 / PRD-005 / PRD-006 AI 调用全部复用本节, 不另做; (2) **finding #2** Resilience Score 改为**规则引擎计算** (复现性高, 同输入同输出), LLM 只生成"解释 + 改进建议", UI 显式标注"评分: 规则计算 vs 建议: AI 生成"两类来源; (3) **finding #3** 置信度从 0-1 数字 (如 92%) 改为 **high / medium / low 三档**, 配 confidence_factors (引用 KB 数 + 检测事实数 + 规则引擎佐证), 信任靠引用证据不靠数字。Q2 / Q3 同步重决（Ollama v1 必做）。**状态保持排队评审 (重要不紧急)**, 等 Mars 整体方向定。 |
| 2026-05-31 | Mars | 排队评审 → **✅ 已评审** | 通过, 可进研发. |

---

<a id="prd-004"></a>
## PRD-004 — MCP Server "Supkube Skills"（开源 · 接入客户侧 AI Agent · SSE）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-004 |
| **任务编号** | #116 |
| **状态** | **已评审（2026-05-31）** |
| **作者** | Claude / Mars + 外部专家咨询输入 |
| **目标版本** | v0.11.x（**依赖 PRD-003 已评审通过**, 因为 `get_backup_advice` Skill 复用 Advisor Engine） |
| **关联 ADR** | ADR-033（拟）AI Advisor 架构 + **ADR-034 (拟) MCP 协议选型** (Streamable HTTP, 本 PRD) + **ADR-036 (拟) SSE 项目级口径** (本 PRD 兼容退路) |
| **关联 PRD** | PRD-003（Advisor Engine 共享）/ PRD-002（Transform 推荐反映） |
| **协议依据** | [Model Context Protocol](https://modelcontextprotocol.io) (Anthropic 标准, **MCP 2025-03-26 推荐 Streamable HTTP**) |
| **传输模式** | **Streamable HTTP（v1 主推, finding T2 2026-05-31 修订）** —— 单端点双向流, Bearer Token / mTLS 友好, nginx ingress 兼容性好。**SSE 仅作向后兼容**（旧 MCP Client 过渡期支持, 不投入新功能）|
| **开源许可** | **Apache-2.0**（Skills + Server 全开源）—— 客户接入 0 摩擦 |

> **PRD-004 与 PRD-003 的边界**：
> - **PRD-003** = SupKube **内部** UI 体验集成 Advisor（仪表板/RestoreDrawer/Applications Score）, 服务 SupKube 用户
> - **PRD-004** = **对外** MCP Server, 让客户家里的 AI Agent (OpenClaw / Claude Desktop / Copilot / 任何 MCP Client) 能调用 SupKube
> - **共享同一个 Advisor Engine**（PRD-003 §4.2 module）—— 0 重复实现

### 1. Goal

把 SupKube 的核心能力以 **MCP 标准协议** + **开源 Skills 仓库** 形式对外发布, 让客户侧的 LLM Agent 平台（OpenClaw / Claude / Copilot / Dify / Coze / 自研 Agent...）能用**自然语言**驱动 SupKube 完成: 查询工作负载 / 拿备份建议 / 创建策略 / 触发备份 / 查结果。

### 2. Epic

ADR-031 "全职灾备运维专家"Epic 的**生态延伸**: SupKube 不仅在自家 UI 里是专家, 更要让客户在自己的 AI 工作流里**召之即来**。这是从"产品"到"生态组件"的关键一跃。

### 3. User Stories

#### 3.1 客户运维（最终用户）视角
- 作为 K8s 运维, 我在公司部署的 OpenClaw（或 Claude Desktop / Dify）里打字: **"帮我看下 prod-orders 命名空间的备份现状, 给个建议"** → AI Agent 自动调 SupKube MCP Server 的 `list_k8s_workloads` + `get_backup_advice` → 5 秒内回我一段专业评估。
- 作为 K8s 运维, 我说: **"上线前给 user-db 做个即时备份"** → AI 识别意图 → 调 `trigger_backup_execution` → **询问我确认** → 我说"确认" → 备份启动 → 完成后推送结果到聊天窗口。
- 作为 K8s 运维, 我说: **"帮我建一条 user-db 的每天凌晨 2 点备份策略, 保留 30 天"** → AI 调 `create_backup_policy` → **返回 dry-run YAML 给我看** → 我确认 → 策略创建。

#### 3.2 SupKube 客户的 Platform Team 视角
- 作为 Platform Engineer, 我能在 GitHub 找到 `supkube/mcp-server` 开源仓库, 5 分钟跑起来 (docker run + 环境变量)。
- 作为 Platform Engineer, 我能选择"只读"和"读写"两种 Skills 配置, 限制 AI Agent 能做什么。
- 作为 Platform Engineer, 我能接入到公司的 SSO / Audit Log, 每次 AI 调用都可追溯到具体的人。

#### 3.3 SupKube 销售 / 营销视角
- 作为 PMM, 我能在产品页写"SupKube is MCP-native: integrate with OpenClaw, Claude, Copilot, or any AI Agent in 5 minutes" —— 让 SupKube **被 AI 生态选中**, 而不是 reinvent agent。

### 4. Functions

#### 4.1 架构

```
┌──────────────────────────────────────────────────────────┐
│  客户环境 (Customer Cluster / Workstation)                 │
│  ┌────────────────┐                                       │
│  │ OpenClaw       │ ──┐                                   │
│  │ Claude Desktop │   │ **MCP 协议 (Streamable HTTP v1 主推, SSE 兼容)**│
│  │ Copilot        │   │  单端点双向流, Bearer Token 友好    │
│  │ 自研 Agent      │ ──┤                                   │
│  └────────────────┘   │                                   │
└───────────────────────│───────────────────────────────────┘
                        ▼
┌──────────────────────────────────────────────────────────┐
│  SupKube MCP Server (开源, Apache-2.0, 独立 Pod / 容器)    │
│  ┌────────────────────────────────────────────────────┐  │
│  │ **/mcp HTTP 单端点 (Streamable HTTP, MCP 2025-03-26)**  │
│  │ /mcp/sse + /mcp/message (兼容退路, 旧 Client)        │  │
│  │ Skills Registry (5 个核心 + 未来扩展)                │  │
│  │ Input Validator (JSON Schema + 防注入)              │  │
│  │ Auth (Bearer Token / mTLS, **HitL confirm_id 服务端持久化**)│
│  │ Human-in-Loop 标志位 (requires_confirmation)        │  │
│  │ Audit Logger (复用 PRD-003 audit 模块)              │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
                        │ HTTP REST
                        ▼
┌──────────────────────────────────────────────────────────┐
│  SupKube Backend (现有 REST API + Advisor Engine 共享)     │
│  /api/v1/clusters · /api/v1/applications · /api/v1/policies│
│  /advisor/preflight · /advisor/score (PRD-003)            │
└──────────────────────────────────────────────────────────┘
```

#### 4.2 v1 必交付 5 个 Skills（Mars 已锁定）

| # | Skill name | Description (给 LLM 看的 prompt) | 类型 | Human-in-Loop |
|---|---|---|---|---|
| 1 | `list_k8s_workloads` | "Query Kubernetes workloads (Deployment / StatefulSet / DaemonSet) in a given cluster and namespace, returning name + kind + status + replica count." | **Read** | ❌ 不需 |
| 2 | `get_backup_advice` | "Get backup and resilience recommendations from SupKube Advisor for a given namespace or workload, including risk score, identified weaknesses, and suggested policy parameters." | **Read** | ❌ 不需 |
| 3 | `create_backup_policy` | "Create or update a SupKube backup policy for specified workloads with cron schedule, retention, BSL, and encryption options. **Returns a dry-run preview first**; actual creation requires `confirm=true` parameter." | **Write** | ✅ **必需**（返回 `requires_confirmation: true` + dry-run YAML, Agent 必须二次调用 confirm=true 才落地）|
| 4 | `trigger_backup_execution` | "Manually trigger an existing backup policy to run immediately (one-shot snapshot). Useful before major changes." | **Write** | ✅ **必需**（同上）|
| 5 | `get_backup_status` | "Get execution history, latest status, success rate, or error logs of a specific backup policy. Returns **structured summary** (status/duration/error_kind), not raw K8s logs, to fit LLM context window." | **Read** | ❌ 不需 |

每个 Skill 的 `inputSchema`（JSON Schema）严格定义参数, 写在 `manifest.json` 中, 开源仓库 README 完整文档化。

#### 4.3 关键安全设计（基于专家建议 + 我们的"谁配置, 谁负责"原则）

| 设计 | 实现 |
|---|---|
| **最小权限（PoLP）** | Server 启动支持 `--mode=readonly` 标志, 禁用所有 Write Skills; 提供两套预设 ServiceAccount（`supkube-mcp-readonly` / `supkube-mcp-full`） |
| **人在回路（Human-in-Loop）** | Write Skills 默认返回 `requires_confirmation: true` + dry-run preview + **`confirm_id` (UUID)**; **finding #2 (2026-05-31)**: 服务端按 `confirm_id` **持久化快照** (落 Redis 或 K8s CR `MCPConfirmation`, 跨多副本可访问), 内容含 `skill_name + 规范化后 inputs + dry-run effective output + user_token + expires_at`。Agent **二次调用 confirm** 时**只认 confirm_id + 用户确认信号**, **忽略**第二次调用携带的 inputs / 业务参数（防 LLM 幻觉 / 篡改: 一次 dry-run 后, 第二次"我改个 namespace 名字"也不允许, 必须重新走一次 dry-run）; **二次 confirm 有效期 5 分钟**, 过期自动 GC 持久化记录 |
| **凭证安全** | **绝不**硬编码; 通过环境变量 `SUPKUBE_API_TOKEN` 注入; 支持 K8s Secret 挂载; 推荐 mTLS 部署模式 |
| **输入校验** | 每个 Skill 的 inputSchema 严格校验; cron 表达式用 robfig/cron 解析; namespace/name 走 K8s DNS-1123 校验; 拒绝任何 shell metacharacter |
| **结果裁剪** | `get_backup_status` 输出**结构化摘要**（status/duration/error_kind/last_3_phase_messages）, 不返回完整 K8s log; **限制单次响应 ≤ 4KB** 防止吃爆 LLM context |
| **审计** | 复用 PRD-003 audit 模块, 每次 Skill 调用记录: caller_agent_name (从 user-agent 解析) + skill_name + inputs (脱敏) + outputs (摘要) + duration + cost; **每个客户的 audit log 隔离** |
| **限流** | 单 token 默认 60 calls/min, Write Skills 默认 5 calls/min, 可配置 |
| **降级** | Backend 不可达时, Server 返回 503 + `retry_after_seconds`; 不要返回模糊错误让 LLM 瞎猜 |

#### 4.4 技术栈选型

| 维度 | 选型 | 原因 |
|---|---|---|
| **语言** | **Go**（与 supkube-backend 同语言, 复用 client + 类型）| 而非 Python——避免引入新栈; MCP Go SDK (`github.com/mark3labs/mcp-go`) 已成熟 |
| **MCP SDK** | `mark3labs/mcp-go` 或 Anthropic 官方 Go SDK（待评估）| 看哪个**Streamable HTTP** (MCP 2025-03-26+) 支持更稳 |
| **HTTP 框架** | `gin` 或 `chi`（沿用 backend 选型）| 一致性 |
| **部署形态** | (a) 独立容器（推荐, Helm subchart）/ (b) 嵌入 supkube-backend 同进程（备选）| (a) 让客户能独立扩缩容, 失败隔离 |
| **传输** | **Streamable HTTP v1 主推**（finding T2 2026-05-31）, SSE 仅做向后兼容退路; v1.1 加 Stdio 支持桌面客户端（Claude Desktop 等）| MCP 2025-03-26 推荐 + 团队既有经验（ADR-015 nginx ingress SSE 兼容性差）+ Bearer Token 与 SSE 双端点不兼容（token 易被迫入 URL）|
| **认证** | v1: Bearer Token (走 HTTP header, Streamable HTTP 天然支持); v1.1: mTLS / OIDC | |

> **注**: 外部专家给的 Python FastAPI 示例代码可作为**架构参考**, 但实际实现用 Go 与现有 backend 对齐。

#### 4.5 开源仓库结构

```
github.com/supkube/mcp-server/   (新仓库, Apache-2.0)
├── README.md             # 5 分钟接入教程 (含 OpenClaw / Claude Desktop / Dify 三种 client 配置示例)
├── LICENSE               # Apache-2.0
├── cmd/server/main.go    # 入口
├── internal/
│   ├── skills/           # 5 个 Skills 实现
│   │   ├── list_workloads.go
│   │   ├── backup_advice.go
│   │   ├── create_policy.go
│   │   ├── trigger_backup.go
│   │   └── backup_status.go
│   ├── sse/              # SSE 传输层
│   ├── auth/             # Token + mTLS
│   ├── validator/        # 输入校验
│   └── supkube_client/   # 调 supkube-backend REST API
├── manifest.json         # Skills 注册表（给 LLM 看的 description + inputSchema）
├── examples/
│   ├── openclaw-config.json
│   ├── claude-desktop-config.json
│   └── dify-tool-import.yaml
├── deploy/
│   ├── docker-compose.yml
│   ├── helm/             # subchart
│   └── k8s-manifest.yaml
└── docs/
    ├── security.md       # PoLP / Human-in-Loop / 凭证管理
    └── extending.md      # 客户怎么加自定义 Skill
```

### 5. UI / UX

PRD-004 **没有 SupKube UI 改动**——它是一个独立的 server 进程。但需要在 SupKube **Settings** 页加一个 "MCP Server" tab:

```
┌─ Settings → MCP Server ──────────────────────────────────┐
│ ☑ Enable MCP Server                                       │
│ Endpoint: https://supkube-mcp.your-domain.com/mcp/sse    │
│ Mode: ◉ Read-only  ○ Read-Write                          │
│                                                           │
│ Bearer Tokens:                                            │
│   [+ Generate New Token]                                  │
│   ┌──────────────────────────────────────────────┐       │
│   │ openclaw-prod      Read-Write  · 12 calls/d  │ [↻][🗑]│
│   │ claude-desktop-mar Read-only   · 3 calls/d   │ [↻][🗑]│
│   └──────────────────────────────────────────────┘       │
│                                                           │
│ Recent Audit (last 10):  [View All →]                    │
│   2026-05-30 14:23  openclaw-prod  get_backup_advice OK  │
│   2026-05-30 14:21  openclaw-prod  list_workloads   OK   │
│                                                           │
│ [Download Client Config: OpenClaw ▾]                     │
└──────────────────────────────────────────────────────────┘
```

### 6. Out of Scope

| 项 | 原因 | 去向 |
|---|---|---|
| **Stdio 传输** | v1 Streamable HTTP 优先（生产环境, finding T2 2026-05-31）| v1.1 加（桌面 Claude Desktop 用例）|
| **SSE 新功能投入** | v1 仅 Streamable HTTP 新功能; SSE 双端点 (`/mcp/sse` + `/mcp/message`) 仅维持向后兼容, 不投入新 Skills | v1.x 视 MCP 生态实际情况, 若旧 Client 全升级即 deprecate SSE |
| **MCP Resources / Prompts**（MCP 协议另外两类）| v1 只做 Tools | v2 加 |
| **OpenClaw 内部插件** | 客户侧产品, 不归我们 | 客户自己做 |
| **自定义 Skill 扩展机制（用户写 Lua/JS 加 Skill）** | v2 高级功能 | v0.12.x+ |
| **MCP Gateway（聚合多个 SupKube cluster 的 Server）** | v2 多租户 | v0.12.x+ |

### 7. 非功能性要求

| 维度 | 要求 |
|---|---|
| **延迟** | Read Skills P95 ≤ 2s; Write Skills（含 dry-run）P95 ≤ 3s |
| **可用性** | 单实例 99.5%; 多实例 + LB 99.9% |
| **可观测** | Prometheus metrics: `mcp_skill_calls_total{skill, status}`, `mcp_skill_duration_seconds`, `mcp_active_sse_connections` |
| **凭证轮换** | Bearer Token 可一键 revoke / re-generate; 24h 内自动通知 audit log |
| **兼容性** | 跟随 MCP 协议主版本号; breaking change 走 supkube-mcp v2 仓库 |
| **多语言** | Tool description 同时提供中英双语（很多 LLM 在中文场景中文 prompt 效果更好）|
| **包大小** | 镜像 ≤ 50MB (alpine + Go static binary) |

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | 5 个 Skills 全部实现 + 单测覆盖 ≥ 80% |
| 2 | **Streamable HTTP** endpoint 通过 MCP 协议 conformance test (**MCP Inspector** 官方工具, finding #3 2026-05-31); SSE 兼容退路同步用 inspector 校验 |
| 3 | 用 **Claude Desktop** 配置后能列出 5 个 Skills 并成功调用每一个（**MCP Inspector + Claude Desktop = 验收基线**, finding #3 2026-05-31）|
| 4 | ~~OpenClaw alpha 验证~~（finding #3 2026-05-31 降级为 **nice-to-have / alpha**）: 不再强绑第三方文档作为 v1 DoD; v1 完成后**择机找 1-2 个客户**做 OpenClaw 接入 alpha, 失败不阻塞 v1 ship |
| 5 | `--mode=readonly` 启动后, Write Skills 在 Skills 列表不可见, 强行调用返回 403 |
| 6 | `create_backup_policy` 不带 confirm 时返回 `requires_confirmation: true` + dry-run YAML + `confirm_id` (UUID); 带 `confirm_id` 后才落地; **finding #2 (2026-05-31)**: confirm 时**忽略第二次调用携带的 inputs**, 只认 confirm_id + 用户确认信号（test: 第一次 dry-run 用 ns=foo, 第二次 confirm 时塞 ns=bar, 必须仍按 ns=foo 落地）; 5 分钟超时 expire; **服务端快照跨多副本 (test: kill server pod, 重启后另一副本能继续 confirm)** |
| 17 | **finding T2 (2026-05-31)** Bearer Token 走 Streamable HTTP HTTP header 不进 URL（test: 抓包 `/mcp` 单端点请求, Authorization header 携带 token, query string 无 token）|
| 18 | **finding T2 (2026-05-31)** nginx ingress 上 Streamable HTTP 长连接稳定 30min 不被 buffer; SSE 兼容退路实测 buffer 兼容性差以验证选型理由（test: helm install 默认 ingress + 跑 30min Streamable HTTP `tools/call` 流式响应, 行间隔 < 500ms）|
| 7 | 输入注入测试: `namespace="default; rm -rf /"` 等恶意输入被 validator 拒绝 |
| 8 | `get_backup_status` 输出长度 ≤ 4KB, 结构化字段齐全 |
| 9 | Bearer Token 认证: 无 Token / 错 Token / 过期 Token 都正确拒绝 |
| 10 | 限流: 单 Token 超出 60 calls/min 返回 429 |
| 11 | Prometheus metrics 端点 `/metrics` 暴露 4 类指标 |
| 12 | Audit log 写入与 PRD-003 audit 模块一致 |
| 13 | Settings → MCP Server tab 集成: 启用开关 + Token 管理 + 模式切换 + Client Config 下载 |
| 14 | 开源仓库初始化: README + LICENSE + manifest.json + 3 套 Client 配置示例 |
| 15 | GitHub Action: PR 触发单测 + 镜像构建 + 推 ghcr.io/supkube/mcp-server |
| 16 | Docs: `security.md`（PoLP/HitL/凭证）+ `extending.md`（怎么加 Skill）|

### 9. 任务拆分

**Phase 1 — 仓库脚手架 + 单 Skill 验证（1 周）**
- 新仓库 `supkube/mcp-server` 创建（Apache-2.0）
- Go module 初始化 + MCP SDK 选型验证（mark3labs vs Anthropic 官方）
- **Streamable HTTP** endpoint 跑通 + Claude Desktop 连接验证 + MCP Inspector 通过 conformance（finding T2/finding #3 2026-05-31）
- 实现 `list_k8s_workloads`（最简单的 Read Skill）作为 PoC

**Phase 2 — 5 Skills 完整实现（1.5-2 周）**
- 2 个 Read Skills（list_workloads + get_advice）
- 3 个 Write Skills（create_policy + trigger_backup + 还有什么 write）
- Human-in-Loop 二次确认机制
- Input Validator（防注入）

**Phase 3 — 安全 + 部署（1 周）**
- Bearer Token Auth
- 限流（per-token 60/min）
- 输出裁剪（4KB cap）
- Helm subchart + Docker image
- Prometheus metrics

**Phase 4 — SupKube UI 集成（0.5 周）**
- Settings → MCP Server tab
- Token 管理 UI
- Client Config 下载

**Phase 5 — 开源 + 文档 + 客户验证（1 周）**
- README + 3 套 Client 配置示例（OpenClaw + Claude Desktop + Dify）
- security.md + extending.md
- 找 1-2 个客户做 alpha 测试（OpenClaw 接入）
- 仓库公开 → 发 PR Anthropic 官方 MCP Server 列表

**总计 4-5 周**（一人全职 / 两人 2-3 周）

### 10. 关联文档与任务

- **PRD-003** (Advisor Engine): `get_backup_advice` Skill 直接调用 `/advisor/score` API
- **PRD-002** (Transform): 未来扩展 Skill `recommend_transformset` 可基于 PRD-002 派生
- **ADR-031** (5 层韧性): MCP Server 是"被生态选中"路径
- **ADR-033** (拟): AI Advisor 架构
- **ADR-034** (拟): MCP 协议选型 (Streamable HTTP, 2026-05-31 finding T2 落地)
- **ADR-036** (拟): SSE / 长连接 / 流式传输项目级口径 (PRD-004 SSE 向后兼容 + PRD-005 Live Tail 共用)
- **#116**: 本 PRD 实施 task

### 11. 开放问题

| Q | 问题 | 倾向 |
|---|---|---|
| Q1 | **MCP SDK 选 mark3labs/mcp-go 还是 Anthropic 官方 Go SDK？** | Phase 1 PoC 阶段对比验证, 倾向官方（长期维护保障）|
| Q2 | **Bearer Token 持久化方式？** | K8s Secret + 客户 namespace 隔离 |
| Q3 | **跨集群场景: 一个 MCP Server 管多集群, 还是一个集群一个 Server？** | v1 **一个 Server 管多集群**（与 #65 MCM 对齐, Skill 输入带 cluster_id 参数）|
| Q4 | **开源仓库放在 `supkube/mcp-server` 还是子目录 `supkube/supkube/cmd/mcp-server`？** | 倾向 **独立仓库**, 让开源贡献者 contribute 更聚焦 |
| Q5 | **客户验证 alpha 找谁？** | 待 Mars 在 demo 客户里挑 1-2 个友好客户 |
| Q6 | **Stdio 支持优先级？** | v1.1（让 Claude Desktop / Cursor 这类桌面工具能直接用）|
| Q7 | **是否需要 SupKube 主仓库 import MCP Server 的 changelog？** | 是, 主 ROADMAP 同步 MCP Server release notes |
| Q8 | **OpenClaw 团队是否有官方 MCP 文档可对照？** | 待 Mars 提供（如客户能给 OpenClaw 文档链接, 我们做 example config 更精准）|
| Q9 | **5 个 Skills 之外, 下一批 5 个会是什么？** | 倾向: `restore_from_backup` / `validate_backup_integrity` / `list_transformsets` / `derive_transform_from_conflict` / `get_resilience_posture` (从 PRD-003 score 衍生) |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-30 | Mars + 外部专家 | 讨论 | Mars 与外部 AI 专家深度讨论 MCP Server 实施方案: SSE 传输 / 5 核心 Skills / Python FastAPI 示例代码 / PoLP / Human-in-Loop / cron 翻译 / 输出结构化裁剪 |
| 2026-05-30 | Claude | — → **草稿** | PRD-004 v1 完成。吸收外部专家方案 + 与 PRD-003 对齐共享 Engine + 改用 Go 实现（与现有 backend 对齐）+ 5 个 Skills + 16 项 DoD + Q1-Q9 开放问题 |
| 2026-05-31 | Mars (评审人 Claude 委托) | 草稿 → **改正中** | 落 1 Blocker + 2 High finding: (1) **finding T2 (Blocker)**: §4.1 / §4.4 / §6 传输模式从 "SSE only" 改为 **"v1 Streamable HTTP (MCP 2025-03-26 后官方推荐); SSE 仅作向后兼容"**, 理由: MCP 规范升级 + ADR-015 团队既有经验 (nginx ingress SSE 兼容性差) + Bearer Token 跟 SSE 双端点不兼容 (token 易被迫入 URL); **依赖 ADR-034 MCP 协议选型 + ADR-036 SSE 项目级口径**（后台 agent 写中）; (2) **finding #2 (High)** HitL confirm 必须基于**服务端持久化快照** (按 confirm_id 落 Redis / K8s CR `MCPConfirmation`, 跨多副本可访问), confirm 时**只认 confirm_id + 用户确认信号**, **忽略第二次调用携带的业务参数**（防 LLM 幻觉 / 篡改）; 修 §4.3 + §4.4 + DoD #6; (3) **finding #3 (Med)** 验收基线 (DoD #2/#3/#4) 改为 **MCP Inspector + Claude Desktop (协议 conformance)**; OpenClaw 列为 nice-to-have / alpha（不强绑第三方文档作为 v1 ship 条件, 失败不阻塞）。DoD 由 16 项扩至 18 项 (#17/#18 落 finding T2 测试)。**等 Mars 重审 → 排队评审 → 已评审**。 |
| 2026-05-31 | Mars | 改正中 → **✅ 已评审** | 通过, 可进研发. |

---

<a id="prd-005"></a>
## PRD-005 — Log Viewer v2（完整运维级日志观察平台 · 分批 ship）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-005 |
| **任务编号** | #118 |
| **状态** | **已评审（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.11.x → v0.13.x（**分 5 phase 渐进 ship**, 非一次性大爆炸）|
| **关联 ADR** | **ADR-035 (拟) 结构化日志规范 + 错误代码体系** (本 PRD 主, X2 重编 2026-05-31, 替代原"ADR-033 撞号"的写法) + **ADR-036 (拟) SSE / 长连接 / 流式传输项目级口径** (X3 引用)|
| **关联 PRD** | PRD-003（AI Advisor, 供 §4.9 AI 根因摘要; X4 共享脱敏管线）/ PRD-006（Activity Timeline, 引用本 PRD §4.8 deep-link schema）|
| **前置** | 任务 #79（Log Viewer v1）已 completed; **2026-05-31 当日 7 个 Quick Win** 已 ship（顶部错误摘要 / Prev-Next Error / Expand Context / 时间格式切换 / Live Tail Lock / 荧光笔 / Fullscreen）|

> **诚实判断**: v2 是个大 feature, 估 3-4 周完整工期, **不应一次评审一次 ship**, 应**分批 ship**（见 §9）。本 PRD 一次性把剩余 spec 全部落档, 但**每个 phase 单独允许 Mars 拍板节奏**, Quick Win 验证好 vs. 全量进 v2 不冲突。
>
> **不过度承诺**: 真 SSE Live Tail 延迟 100-500ms（容器日志路径限制）, **不是秒级**; AI 根因输出标注「AI 建议(置信度 X)」, **不说"自动根因分析"**。

### 1. Goal

把 Log Viewer 从"能看日志"升级为**完整运维级日志观察平台**: 客户在 SupKube 内部排错时, 不必再切到 kubectl / Loki / Grafana, 一个界面就能"看清楚 + 找得到 + 跳得过去 + 推得出去 + 看懂为什么"。

### 2. Epic

ADR-031 "全职灾备运维专家"Epic 的**日志面**: 专家不仅"知道哪里出错", 还能让客户**自己看懂**为什么。Log Viewer v2 = **客户运维零摩擦自救**的核心承载。

### 3. User Stories

#### 3.1 Demo 排错（最高优先级场景）
- 作为 SE/售前, 我在 demo 现场跑备份失败, **2 秒内**能在顶部错误摘要看到错误关键句, 点 Next Error 一秒跳到第一条 ERROR, Expand Context 看上下 ±5 行, **不需要 grep**, 也**不需要切 kubectl**, 客户当场被说服。
- 作为 SE, 我把 URL 复制给客户 (`?component=backend&grep=ERR_CSI_INCOMPAT&since=1h&line=4421`), 客户打开**直接定位到那一行**, 不必复述上下文。

#### 3.2 长期生产排查
- 作为客户运维, **10 万行**日志加载不卡浏览器（Virtual Scroll）, 滚动条 minimap 看到 ERROR/WARN/匹配关键词散落位置, **一眼知道异常密度**。
- 作为客户运维, 备份任务连跑 3 天每天失败, 我**订阅 Live Tail**（真 SSE, 不是 5s 轮询）, 顶部一直挂着 "AI 建议: 卡在 csi-driver 注册阶段, 置信度 0.72" + KB 链接 `ERR_CSI_INCOMPAT`, 我跟着 KB 走一遍 → 修好。
- 作为客户运维, 我切到 Debug 层级看到完整 trace, 平时只看 Summary 不被噪声淹没。

#### 3.3 合规审计 / Forwarding（**finding #2 修订 2026-05-31**）
- 作为客户 Platform Engineer, 我把 SupKube 日志通过 Forwarding 推到公司 Loki/ELK/Splunk/Datadog, **保留期由公司日志平台决定**, SupKube 自己**不做长期存储**。
- 作为合规官, 我能导出指定时间段的 ERROR 日志 (CSV/JSON) 附 Job ID + Policy + 触发用户 metadata。**但**: 此能力**仅在**满足以下条件之一时有效:
  - (a) **已配置 Log Forwarding** 到客户日志平台（导出从客户日志平台查, SupKube 仅提供查询代理 UI）
  - (b) **目标 Pod 日志仍存活**（K8s 节点未 GC 老 container, 默认仅最近一次 + 部分历史, 不可期望）
  - **不允许**客户误以为"SupKube 自身可回溯任意历史日志" —— UI 顶部告诉客户当前数据来源 (kubectl pod 日志窗口 vs Forwarding 客户日志平台代理), 数据缺失时不假装有数据。

### 4. Functions

#### 4.1 已完成（v2.0 之前的基线, 不重做）

| # | 功能 | 来源 | 状态 |
|---|---|---|---|
| 1 | 顶部错误摘要卡（前 3 条 ERROR 摘要 + 数量徽章）| Quick Win 2026-05-31 | ✅ Shipped |
| 2 | Prev / Next Error 导航（`[` / `]` 键 + 按钮）| Quick Win | ✅ Shipped |
| 3 | Expand Context（点 ERROR 行展开上下 ±5 行）| Quick Win | ✅ Shipped |
| 4 | 时间格式切换（绝对 HH:MM:SS.fff / 相对 "3m ago"）| Quick Win | ✅ Shipped |
| 5 | Live Tail Lock（用户向上滚 → 暂停 autoscroll）| Quick Win | ✅ Shipped |
| 6 | 搜索词亮黄底荧光笔（grep 命中高亮）| Quick Win | ✅ Shipped |
| 7 | Fullscreen 模式（`F` 键 / 按钮）| Quick Win | ✅ Shipped |
| — | Dark Mode / Monospace / Sticky Timestamp / 颜色语义化（ERROR 红 / WARN 黄 / INFO 蓝 / DEBUG 紫）| v1 | ✅ Shipped |
| — | 多组件切换 / Severity 筛选 / Since 时间窗 / Tail 行数 / 自动刷新（5s 轮询）| v1 | ✅ Shipped |

> v2 范围内**不重做以上 7 项 Quick Win**, 但 §4.6 Minimap 与 §4.2 Virtual Scroll 实现时必须**保持已有键位与状态兼容**。

#### 4.2 Virtual Scroll（v2.0 · P0 · 必做）

**问题**: 当前 `<div v-for>` 直接渲染所有行, 10 万行 → 浏览器卡死 / 内存爆。

**方案**:
- 引入 `vue-virtual-scroller` 或自研 windowed list（推荐前者, 成熟稳定）
- 行高固定 (CSS line-height 锁定 1.5rem) → 启用 `RecycleScroller`（O(1) 复用 DOM）
- Expand Context 展开的多行 → 用动态高度变体 `DynamicScroller` 局部, 或展开内容渲染到 portal 浮层（避免破坏行高一致性）
- 维持 Sticky Timestamp / 高亮 / Prev-Next 跳转的 scrollIntoView 行为
- **DOM 节点上限 ≤ 200**（视口 ~80 行 + buffer）

**约束**:
- 10 万行加载渲染（首屏可见 P95 ≤ 200ms）
- 浏览器内存增量 ≤ 150MB（vs 当前 1 万行 ~80MB）

#### 4.3 SSE Live Tail（v2.1 · P0 · **X3 保留 SSE + ingress 配置固化 2026-05-31**）

> **X3 (High) 决策（2026-05-31）**: 本节 Live Tail **保留 SSE**（服务端→客户端单向流, 正是 SSE 适用场景, 不像 PRD-004 MCP 双向流问题）。**但**之前过度承诺"完美 SSE 体验"不现实, 需要**ingress 配置固化**做承诺替代:
> - nginx ingress annotation: `nginx.ingress.kubernetes.io/proxy-buffering: "off"`
> - 响应 header: `X-Accel-Buffering: no` (后端注入, 防止任何中间 buffer)
> - read/send timeout 加大: `proxy-read-timeout: "3600"` / `proxy-send-timeout: "3600"`
> - **实测**: 在 helm install 默认 ingress 下, 行产生 → UI 显示 < 500ms（DoD 验收）
> - **引用**: ADR-036 (拟) SSE / 长连接 / 流式传输项目级口径（后台 agent 写中）, 本节为其消费者之一

**替换**: 当前 5s 轮询 → 真 Server-Sent Events 流。

**后端 (`supkube-backend/internal/api/v1/logs.go`)**:
- 新端点 `GET /api/v1/logs/stream?component=X&severity=Y&grep=Z` (Content-Type: `text/event-stream`)
- **响应 header 必含 `X-Accel-Buffering: no`**（X3 加, 防止 nginx ingress buffer 导致延迟堆积）
- 底层调 `kubernetes.io/client-go` 的 `Pods().GetLogs(..., Follow:true).Stream(ctx)`, 行级转发
- 多 pod 同名情况下并行 stream + 行级合并（按 timestamp 排序窗口 1s, **finding #3 改进见 §4.3bis**）
- 连接生命周期: 客户端断开 → ctx cancel → 关闭上游 stream; 服务端最长 30min 强制 reconnect

**前端**:
- `EventSource` API + 自动重连（指数退避 1s / 2s / 4s / max 30s）
- 与 Live Tail Lock 集成: 锁定时仍接收但不 autoscroll, 顶部红点提示「新增 N 行」
- 网络断开 → 顶部 banner「连接断开, 重试中... 已重连 N 次」

**Ingress 配置固化（X3 验收硬性要求）**:
- helm chart values 默认含 `nginx.ingress.kubernetes.io/proxy-buffering: "off"` + `proxy-read-timeout: "3600"` + `proxy-send-timeout: "3600"`
- USER_MANUAL §SSE 配置章节固化"如果用其他 ingress (traefik / istio / gloo) 必须查阅本表对应字段"

**延迟约束**: 行产生 → UI 显示 P95 ≤ 500ms（不是秒级承诺）

#### 4.3bis 多 pod 并行 stream 并发约束（finding #3, 2026-05-31 新增）

**"尽力而为" 语义标注**: 多 pod (例如 Deployment 3 副本) 并行 stream + 行级合并, 由于网络 / 时钟漂移, **不保证严格按 wall-clock 顺序**, 合并窗口默认 1s 内按 timestamp 排序, 窗口外**先到先排**。前端 UI 顶部小字"多 pod 合并: 1s 内按时序, 窗口外按到达顺序"。

**NFR 并发上限**:
- 每 viewer × pod 并发 SSE 上限: **5 个 pod / viewer** (即同一 viewer 监控一个 Deployment 最多并行 5 副本流)
- 服务端单进程 SSE 总连接上限: 500
- 超出 → 后端返回 429 + Retry-After

**合并窗口可配置**: 默认 1s, 客户可在 Settings → Log Viewer 调到 100ms / 500ms / 2s / 5s, 也可关闭合并 (raw pod ID prefix 模式)

#### 4.4 三层日志深度 Summary / Detail / Debug（v2.2 · P1）

**前提**: SupKube 后端（backend / agent / operator）需先用 **结构化 logger**（推荐 `slog` / `zerolog`）输出, **此为 ADR-035 (拟) 结构化日志规范 + 错误代码体系范围 (X2 重编 2026-05-31, 替代原 ADR-033 撞号写法)**, 必须先评审通过 ADR-035 再开发本节。

**模型**:
| 层级 | 内容 | 默认显示 |
|---|---|---|
| **Summary** | 阶段切换 / 关键事件 / 错误一句话 | ✅ 默认 |
| **Detail** | + 子步骤进度 / HTTP/gRPC 调用 / 资源变更 | 点击切换 |
| **Debug** | + 完整 trace / 函数调用栈 / 中间状态 dump | 点击切换 + 警告"输出量大" |

**实现**:
- logger 输出 JSON, 含 `level` (info/debug/trace) + `category` (summary/detail/debug)
- 前端 ToggleGroup: `[Summary] [Detail] [Debug]`, 默认 Summary
- 切到 Debug 时**重新请求** (`?verbosity=debug`), 不在前端过滤（避免拉全量浪费带宽）

**Out of Scope（本节）**: 老组件 logger 改造（独立 task, 关联 ADR-035 (拟) 结构化日志规范 + 错误代码体系）

#### 4.5 错误代码规范 `ERR_*` + KB 跳转（v2.2 · P1）

**模型**:
- 每个已知错误一个稳定代码: `ERR_CSI_INCOMPAT` / `ERR_BSL_AUTH_FAIL` / `ERR_RESTORE_NS_CONFLICT` / `ERR_VELERO_NOT_READY` / ...
- logger 输出: `{"level":"error","err_code":"ERR_CSI_INCOMPAT","message":"...","kb":"https://docs.supkube.io/kb/ERR_CSI_INCOMPAT"}`

**前端**:
- ERROR 行解析 `err_code` → 显示为可点击 chip 「ERR_CSI_INCOMPAT ↗」→ 跳 KB
- 顶部错误摘要卡按 `err_code` 聚合（相同代码合并计数）

**KB 仓库**: `supkube-docs/kb/ERR_*.md`, 每个错误模板含 现象 / 根因 / 解法 / 相关 PRD 链接

#### 4.6 滚动条 Minimap（v2.1 · P1 · **finding #4 改 Canvas 分桶聚合 2026-05-31**）

**位置**: Console 右侧, 宽 8px, 与 Virtual Scroll 视口高度等长

**渲染（finding #4 重写）**:
- **Canvas 实现** (不用 SVG 也不用 DOM v-for): 10 万行日志时 SVG / DOM 1px-per-line 会爆 (10 万个 element)
- **视口高度分桶聚合**: 总行数 N / 视口可见高度 H_px → 每像素聚合该区间最高 severity
  - 例: 100k 行 / 600px = 167 行 / pixel; 每像素 167 行中**最高 severity 决定颜色** (ERROR > WARN > grep-hit > INFO > DEBUG)
- 颜色: ERROR 红 / WARN 黄 / grep 命中黄底 / INFO 透明 / DEBUG 透明
- 当前视口范围 → 半透明灰色滑块覆盖（separate Canvas layer）

**交互**:
- 点击任一像素 → 反算 idx = `pixelY * (N / H_px)` → Virtual Scroll `scrollTo(idx)` + 行 flash 高亮
- Hover 显示 tooltip: 显示该像素聚合的"X 个 ERROR + Y 个 WARN" 摘要; 若该像素只对应一行（小日志场景）显示"Line 4421 · ERR_CSI_INCOMPAT · 14:23:01"

**性能保证**: Canvas re-render 仅在 (a) 视口 resize (b) 新 log 行追加 (debounce 100ms) 触发; 单次 redraw < 16ms (60fps)

#### 4.7 Log Forwarding（v2.3 · P2）

**目标**: 推送 SupKube 日志到客户的日志平台 (ELK / Loki / Splunk / Datadog)

**配置位置**: Settings → Log Forwarding tab

**协议支持** (v2.3 初版):
| 平台 | 协议 | 备注 |
|---|---|---|
| **Loki** | HTTP push `/loki/api/v1/push` | v2.3 优先（开源, 客户最常见）|
| **Elasticsearch** | `_bulk` API | v2.3 |
| **Splunk HEC** | HTTPS event collector | v2.3 |
| **Datadog** | logs intake API | v2.4 |
| **syslog (RFC 5424)** | UDP/TCP | v2.4 |

**实现**:
- 后端新组件 `supkube-log-forwarder`（独立 sidecar 或 supkube-backend goroutine）
- 配置 CRD `LogForwardingPolicy` (cluster-scoped): `target`/`endpoint`/`credentials_secret_ref`/`filter`/`batch_size`/`flush_interval`
- 失败重试 + DLQ（本地 ring buffer 1000 条）
- Settings UI 含测试按钮 "Send Test Event"

**Out of Scope**: SupKube **不做日志长期存储**, 保留期由客户日志平台决定（明确写进 §6）。

#### 4.8 Deep-link Query Params（v2.0 · P0 · **权威单一来源 X1, 2026-05-31 重写**）

> **X1 (High)**: 本节为 SupKube 全局 deep-link to Log Viewer schema 的**权威单一来源**。PRD-006 §4.6 引用本节, **不另写**。前后端实现以本节 schema 为准, 任何分歧以本节为准。

**路由（单一）**:
```
/observability?tab=logs&<params>
```

> **路由统一**: 与现有 Observability tab 一致, 不另开 `/logs?` 路径。Observability 是 SupKube 主 tab 之一, deep-link 必须打开后落在 "logs" sub-tab。

**参数（严格枚举 + 类型约束）**:

| 参数 | 类型 | 必填 | 取值 / 含义 |
|---|---|---|---|
| `component` | string | **必填** | enum: `backend` \| `frontend` \| `velero` \| `node-agent` \| `dex`（其他值 → 走 fallback toast）|
| `sinceSeconds` | int | **必填** | 纯秒数（例 `600` = 10 分钟）。**不允许** `1h` / `30m` 字符串形式（前期 v1 误设, 现统一秒数）|
| `severity` | string | 可选 | enum: `ANY` \| `ERROR` \| `WARN` \| `INFO` \| `DEBUG`（默认 ANY）|
| `grep` | string | 可选 | 关键词, **URL-encoded** |
| `scrollToLine` | string | 可选 | 整数 (line index 字符串) 或字面值 `auto`（=自动滚到第一条 ERROR, 见下"`auto` 语义"）|
| `live` | bool | 可选 | `1` \| `true` = 是否自动开 Live Tail（默认 0）|
| `t` | epoch-ms | 可选 | 时间锚点（跨 PRD-006 用, 跳转到此时间附近的日志, 取最近 `sinceSeconds` 窗口）|

**`auto` 语义（scrollToLine=auto）**:
- 前端拿到 query → 完成 fetch → 后端在 response 内附带 `firstErrorLineIdx`（后端在当前 window 找 severity=ERROR 的第一条 line index）
- 前端按该 idx scrollTo + flash; 若无 ERROR, fallback 滚到最新一行

**路由解析失败 fallback**:
- 任何无效参数（schema 不符 / enum 越界 / 类型错 / 必填缺失）→ **进 `/observability?tab=logs` 默认状态**（component=backend, sinceSeconds=3600, severity=ANY）, 顶部 toast: `"Deep-link 参数解析失败: <reason>"`（reason 含哪个字段不合法, 便于排查）

**示例**:
```
/observability?tab=logs&component=backend&sinceSeconds=600&grep=ERR_CSI_INCOMPAT&scrollToLine=auto&live=1
/observability?tab=logs&component=velero&sinceSeconds=900&severity=ERROR&t=1717100000000
```

**行为**:
- 进入页面 → 解析 URL → 初始化 state → fetch → 若 `scrollToLine` 非空, 等加载完成后 scrollTo + flash 2s
- state 变化 → `router.replace`（不污染 history）

**承载场景**: PRD-006 Activity Timeline 的某个阶段卡片点「查看日志」→ 按本 schema 拼 URL 跳转

#### 4.9 AI 根因摘要（v2.x · P2 · PRD-003 依赖 · **X4 复用脱敏管线 2026-05-31**）

**前提**: PRD-003 Advisor Engine 已 ship; **§7 脱敏管线 + PRD-003 §7.2 T4 外发治理统一管线已在位**

> **X4 (High) 2026-05-31**: AI 根因外发**日志**, 必须**经 §7 脱敏管线 + PRD-003 §7.2 T4 外发治理统一管线, 不另做一套**。日志比 K8s 元数据**更危险**: stack trace 含函数路径 / 连接串 / token / cookie / 用户邮箱 / PII / customer 数据片段都常出现在日志里。默认**最小外发**: 只发"行号 + severity + err_code + 截短 200 字符的 message"（不发完整 message body）, 客户在 Settings 显式 opt-in "完整日志行发送给 LLM (排错优先, 隐私下降)" 才发完整。

**形态**:
- 顶部错误摘要卡下方多一行: `🤖 AI 建议: 看起来卡在 csi-driver 注册阶段, 可能 ERR_CSI_INCOMPAT（置信度 高）· 详情`
- 「详情」展开 → Advisor 完整建议卡片（复用 PRD-003 §4 component）

**触发**:
- ERROR 数 ≥ 1 时自动调 `/advisor/log-rootcause` 端点
- 入参: 最近 100 行日志, **经 §7 + PRD-003 §7.2 脱敏管线**（默认仅 metadata + 200 字符 message 截短）+ component + 错误代码列表
- 出参: `{ summary, confidence_tier, suggested_kb_links[], related_prd_links[], redacted_field_paths: [list] }` （`redacted_field_paths` 由 PRD-003 §7.2 审计返回）

**约束**:
- 置信度 = `low` → **不显示** AI 建议（避免噪声; **finding #3 (PRD-003) 三档制后**: low / medium / high）
- 显示时**必须**标注置信度档 + "AI 建议"前缀, **绝不**省略
- 调用 ≤ 1 次/min（debounce, 避免反复触发）
- **审计**: 每次 `/advisor/log-rootcause` 调用必须在审计日志中记录 `redacted_field_paths` + `outbound_byte_count`, 满足 PRD-003 §7.2 治理要求

#### 4.10 §7 脱敏管线（X4 新增 2026-05-31, log redaction middleware）

logger 中间件扫描 known patterns:
- 凭证: `token=<x>` / `password=<x>` / `Authorization: Bearer <x>` / `apikey=<x>` / AWS keys / GCP service account JSON 片段 / Azure SP secret
- PII: 邮箱 / 手机号 / 身份证 / 信用卡（regex 库）
- K8s Secret value pattern (base64 + 长度 > 32)
- 自定义 dirty word list (客户可配置)

**替换为 `***` (或 `***<前 4 字符>***<后 4 字符>***` 用于排错可追踪)**, 责任在 logger 一侧, **不依赖前端**。日志离开集群（forwarding / AI 调用）**前**已脱敏。本节即 PRD-003 §7.2 "日志内容" 行为依赖的具体管线实现。

### 5. UI / UX

**v2 完整布局**（三段式）:

```
┌─ Log Viewer ─────────────────────────────────────────────────────────┐
│ ┌─ Status Ribbon ───────────────────────────────────────────────┐   │
│ │ 🔴 12 ERROR  · 🟡 34 WARN  ·  Live 🟢  ·  Component: backend   │   │
│ └─────────────────────────────────────────────────────────────────┘   │
│ ┌─ 错误摘要卡 (Quick Win 已做) ────────────────────────────────────┐ │
│ │ ⚠ 12 errors detected · [Prev] [Next] [Jump first]              │ │
│ │ • ERR_CSI_INCOMPAT × 5  KB↗   • ERR_BSL_AUTH_FAIL × 3  KB↗     │ │
│ │ 🤖 AI 建议: 卡在 csi-driver 注册（置信度 0.72） · [详情]         │ │
│ └─────────────────────────────────────────────────────────────────┘ │
│ ┌─ Control Bar ──────────────────────────────────────────────────┐  │
│ │ [Component ▾] [Severity ▾] [Since ▾] [Tail ▾]                 │  │
│ │ [grep_______________] [Verbosity: Summary|Detail|Debug]        │  │
│ │ [Time: Abs|Rel] [Live 🔘] [Fullscreen] [Export ▾] [Share Link] │  │
│ └─────────────────────────────────────────────────────────────────┘  │
│ ┌─ Console (Virtual Scroll) ─────────────────────────────┬─Mini─┐  │
│ │ 14:23:01.221  INFO  starting reconcile                  │ ░░  │   │
│ │ 14:23:01.502  ERROR ERR_CSI_INCOMPAT csi-driver not... │ ▆█▆ │   │
│ │   ▼ (Expand Context)  14:23:01.498 ...                  │     │   │
│ │ 14:23:01.745  WARN  retrying...                         │  ▆  │   │
│ │ ...                                                      │ ░   │   │
│ └──────────────────────────────────────────────────────────┴─────┘   │
└──────────────────────────────────────────────────────────────────────┘
```

**键位约定**（兼容 Quick Win）:
| Key | 行为 |
|---|---|
| `[` / `]` | Prev / Next Error |
| `J` / `K` | 上 / 下移一行（vim-style） |
| `F` | Fullscreen toggle |
| `T` | Time format toggle |
| `L` | Live Tail toggle |
| `/` | Focus grep input |
| `Esc` | Exit Fullscreen / Clear grep focus |

### 6. Out of Scope（明确不做）

| 项 | 原因 | 去向 |
|---|---|---|
| **日志长期存储** | SupKube 不做日志平台, 不与 Loki/ELK 竞品 | Forwarding 推到客户日志平台 |
| **审计日志集成** | 已有独立 Audit Log tab | 维持现状, 不合并到 Log Viewer |
| **客户自定义 Skill / 插件** | v2 不开放扩展点 | v3 评估 |
| **跨组件日志关联（distributed tracing）** | 需 OpenTelemetry 全链路改造, 工期 > v2 | 独立 PRD（v0.14.x+）|
| **日志 PII 自动脱敏** | 需独立合规框架 | 独立 PRD |
| **多语言 i18n（日志内容）** | 日志保持英文标准 | 仅 UI chrome i18n |

### 7. 非功能性要求

| 维度 | 要求 |
|---|---|
| **性能·渲染** | 10 万行加载首屏可见 P95 ≤ 200ms; 滚动 60fps（Virtual Scroll DOM 节点 ≤ 200）|
| **性能·内存** | 浏览器 tab 增量内存 ≤ 150MB @ 10 万行 |
| **性能·后端** | 单次 `/logs` 请求 P95 ≤ 1.5s; SSE 行延迟 P95 ≤ 500ms |
| **SSE 生命周期** | 客户端断开 → 后端 ctx cancel ≤ 1s; 服务端最长 30min 强制 reconnect; 自动重连指数退避 max 30s |
| **安全·RBAC** | 用户只能看自己有权限的 cluster/namespace 的日志; 后端按 RBAC 过滤, 前端不做权限判断 |
| **安全·Secret 脱敏** | logger 中间件扫描 known patterns (`token=` / `password=` / `Authorization:`) → 替换 `***`; **责任在 logger 一侧**, 不依赖前端 |
| **审计** | Export / Forwarding 配置变更写 audit log |
| **可观测** | Prometheus: `logs_request_duration_seconds`, `logs_sse_active_connections`, `logs_forwarder_send_total{target,status}` |
| **浏览器兼容** | Chrome 100+, Safari 15+, Firefox 100+; Edge 同 Chrome |
| **i18n** | UI chrome 中英双语; 日志内容保持原文 |

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | 10 万行测试数据加载, 首屏可见 P95 ≤ 200ms, 滚动 60fps（Chrome DevTools Performance 录制证明）|
| 2 | Virtual Scroll DOM 节点数 ≤ 200（DevTools Elements 计数）|
| 3 | 浏览器 tab 内存 @ 10 万行 ≤ 250MB total |
| 4 | SSE Live Tail: 后端 `kubectl logs -f` 同时启动, UI 与 kubectl 行延迟差 P95 ≤ 500ms |
| 5 | SSE 断网 5s 自动重连, banner 显示重连次数 |
| 6 | 关闭 tab → 后端 SSE goroutine 1s 内退出（pprof 验证, 无 leak）|
| 7 | Verbosity 切换 Summary / Detail / Debug 触发新请求, 切回 Summary 不重复请求（缓存）|
| 8 | ERROR 行的 `err_code` chip 可点击, 跳转 KB URL 正确 |
| 9 | Minimap 渲染 ERROR/WARN/grep 三类标记, 点击 minimap 跳转 + flash 行 |
| 10 | Deep-link `?component=X&grep=Y&since=Z&line=N&verbosity=detail` 全部参数生效, line 跳转 + flash |
| 11 | Log Forwarding: Loki / ES / Splunk 三个 target 各做端到端测试, 日志可在目标平台搜到 |
| 12 | Forwarding 配置含 `Send Test Event` 按钮, 1s 内返回成功/失败 |
| 13 | Forwarding 失败有重试 + DLQ, 不丢日志（kill 目标平台 30s 再恢复, 日志最终全到达）|
| 14 | AI 根因摘要置信度 < 0.5 不显示; 显示时含 "AI 建议" 前缀 + 置信度数字 |
| 15 | AI 摘要点「详情」展开 PRD-003 Advisor 完整卡片 |
| 16 | 键位 `[`/`]`/`J`/`K`/`F`/`T`/`L`/`/`/`Esc` 全部工作, 与 input focus 不冲突 |
| 17 | Secret 脱敏: 灌入含 `token=abc123` / `password=xyz` 的日志, UI 显示 `***` |
| 18 | RBAC: 无权限用户调 `/logs?component=X` 返回 403; 前端显示「无权限」空状态 |
| 19 | Prometheus metrics 暴露 3 类指标, label 正确 |
| 20 | Export CSV / JSON 含完整 metadata（Job ID / Policy / 触发用户 / 时间窗）|

### 9. 任务拆分 + 分批 ship 计划

**v2.0 — Virtual Scroll + Deep-link（1 周, 解决"日志炸浏览器"）**
- vue-virtual-scroller 集成（RecycleScroller）
- Expand Context 适配 Virtual Scroll
- Deep-link 全参数支持
- 兼容 Quick Win 已有键位 / 状态
- 验收: DoD #1, #2, #3, #10

**v2.1 — SSE + Minimap（1 周）**
- 后端 SSE endpoint（`GetLogs(Follow:true)` stream）
- 前端 EventSource + 自动重连 + Live Tail Lock 集成
- Minimap 组件（Canvas 或 SVG 实现）
- 验收: DoD #4-#6, #9

**v2.2 — 三层日志深度 + 错误代码（1.5-2 周, 含 ADR-035 后端改造）**
- ADR-035 (拟) 结构化日志规范 + 错误代码体系评审通过（结构化 logger + err_code 规范, X2 重编 2026-05-31）
- supkube-backend / agent / operator 三大组件 logger 改造（slog/zerolog）
- err_code 全量梳理（首批 20 个）+ KB 文档骨架
- 前端 Verbosity ToggleGroup + err_code chip
- 验收: DoD #7, #8

**v2.3 — Log Forwarding（1.5 周）**
- `LogForwardingPolicy` CRD + controller
- Loki / ES / Splunk HEC 三 target 实现
- Settings UI（Forwarding tab）
- DLQ + 重试
- 验收: DoD #11-#13

**v2.x — AI 根因（0.5 周, 依赖 PRD-003 完成）**
- `/advisor/log-rootcause` endpoint 复用 PRD-003 Engine
- 前端集成顶部摘要卡
- debounce 1 次/min
- 验收: DoD #14, #15

**总计**: ~5.5-6 周（一人全职）/ ~3-3.5 周（两人）; **每个 phase 独立 ship, 中间允许 Mars 拍板暂停 / 重排序**

### 10. 关联文档与任务

- **任务 #79**（Log Viewer v1）: 前置, 已 completed
- **任务 #118**（本 PRD 实施）: 待立
- **PRD-003**（AI Advisor）: §4.9 AI 根因摘要直接依赖, Phase v2.x 必须等 PRD-003 ship
- **PRD-006**（Activity Timeline）: §4.8 Deep-link 的核心入口承载者
- **ADR-035** (拟, 结构化日志规范 + 错误代码体系, X2 重编 2026-05-31): §4.4 + §4.5 前置, 必须先评审通过
- **ADR-036** (拟, SSE / 长连接 / 流式传输项目级口径, X3 新引用 2026-05-31): §4.3 SSE Live Tail + §4.4 nginx ingress 配置参考
- **`supkube-frontend/src/components/LogViewer.vue`**: v1 + Quick Win 落点
- **`supkube-backend/internal/api/v1/logs.go`**: 后端改造点（SSE + verbosity 参数）

### 11. 开放问题（评审时讨论）

| Q | 问题 | 倾向 |
|---|---|---|
| Q1 | Virtual Scroll 选 `vue-virtual-scroller` 还是自研？ | 倾向 vue-virtual-scroller（成熟, 8k stars, 维护活跃）; 自研只有"完全控制"一个理由 |
| Q2 | Expand Context 与 Virtual Scroll 行高一致性冲突, 用 `DynamicScroller` 还是 portal 浮层？ | 倾向 portal 浮层（不破坏 RecycleScroller 性能假设）|
| Q3 | SSE 端服务最长 30min 是否合理？某些客户希望 24h 长连？ | v2.1 默认 30min, 客户可配置上限; 不允许无限 |
| Q4 | 三层日志 (Summary/Detail/Debug) 是否需要**回到 v1 兼容模式**（即 logger 改造前的旧组件按"全部 Detail"显示）？ | 是, 兼容期 ≥ 6 个月; ADR-035 (拟) 写清楚迁移路径 |
| Q5 | err_code 首批 20 个由谁梳理？是否值得专开一个 task？ | 倾向 Claude + Mars 联合梳理 1 天产出, 单独 task `#err-code-catalog` |
| Q6 | Log Forwarding 三 target 哪个优先级最高？（开发顺序）| 倾向 Loki > Splunk HEC > Elasticsearch（Loki 客户最多 + 开源生态对齐）|
| Q7 | Forwarding 是 push（SupKube 主动推）还是 pull（客户日志平台拉）？ | v2.3 仅 push（简单, 客户配置在 SupKube 内）; pull 模式 v3 评估 |
| Q8 | AI 根因摘要的置信度阈值 0.5 是否合理？（低于不显示）| 待 PRD-003 ship 后用真实数据校准, 暂定 0.5 |
| Q9 | Minimap 与 fullscreen 模式如何共存？（fullscreen 宽屏 minimap 是否变更宽）| 倾向保持 8px 宽; fullscreen 时 minimap 高度自适应窗口 |
| Q10 | Quick Win 与 v2 的代码合并策略？（v1 + 7 Quick Win → v2 重构是否值得？还是增量加？）| 倾向**增量加**, 不做大重构; Virtual Scroll 是唯一需要触及 v-for 渲染层的改动 |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Mars + Claude | Quick Win 落地 | 同日 ship 7 个 Quick Win（顶部错误摘要 / Prev-Next / Expand Context / 时间格式 / Live Tail Lock / 荧光笔 / Fullscreen）, 剩余 spec 整理为 PRD-005 |
| 2026-05-31 | Claude | — → **草稿** | PRD-005 v1 完成。覆盖 5 内容维度 + 5 UX 维度 + 9 Function 子模块 + 20 DoD + 5 phase 分批 ship 计划 + Q1-Q10。**待 Mars 评审** |
| 2026-05-31 | Mars (评审人 Claude 委托) | 草稿 → **草稿 (修订)** | 落 X1+X2+X3+X4 + 4 specific finding (pre-评审 cleanup, 状态保持草稿): (1) **X1 (High)** §4.8 重写为 deep-link schema **权威单一来源**, 路由 `/observability?tab=logs`, 参数严格枚举 (component / sinceSeconds / severity / grep / scrollToLine / live / t), `scrollToLine=auto` 语义 + 路由解析失败 fallback toast; PRD-006 §4.6 将引用本节; (2) **X2 (High)** 全文 "ADR-033 (拟) 结构化日志规范" → **"ADR-035 (拟) 结构化日志规范 + 错误代码体系"**（之前 PRD-005 写成 ADR-033 与 PRD-003/004 AI Advisor 撞号, 现重编）; (3) **X3 (High)** §4.3 SSE Live Tail **保留** (单向流, SSE 适用), DoD 加 ingress 配置固化 (proxy-buffering: off + X-Accel-Buffering: no + read/send timeout 加大, 实测逐行 < 500ms), 引用 ADR-036 (拟); 新增 §4.3bis 多 pod 并行 stream 并发约束 (finding #3); (4) **X4 (High)** §4.9 AI 根因外发日志经 §7 脱敏管线 + PRD-003 §7.2 T4 外发治理统一管线, **不另做一套**, 默认仅 metadata + 200 字符截短; 新增 §4.10 §7 脱敏管线 (log redaction middleware) 实现细节; (5) **finding #2 (High)** US 3.3 审计 / 合规导出改写 (仅在已配置 Forwarding 或 pod 日志存活时有效, 不让客户误以为可回溯任意历史); (6) **finding #3 (Med)** §4.3bis 多 pod 并行 "尽力而为" + 每 viewer × pod 上限 + 合并窗口可配置; (7) **finding #4 (Med)** Minimap 改 Canvas + 视口高度分桶聚合 (每像素最高 severity), 不逐行 1px。**状态保持草稿** (本次为 pre-评审 cleanup, 评审在下次). |
| 2026-05-31 | Mars | 草稿 → **✅ 已评审** | 通过, 可进研发. |

---

<a id="prd-006"></a>
## PRD-006 — Activity Task Detail Timeline（任务详情阶段时间线 + Log Viewer 跳转 + AI 排错位）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-006 |
| **任务编号** | #117 |
| **状态** | **已评审（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.9.x |
| **关联 ADR** | ADR-031（5 层数据韧性 · "全职 SRE 专家" Epic 的可观测面）|
| **关联 PRD** | PRD-005（Log Viewer v2 deep-link, 跳转目标）/ PRD-003（AI Advisor, 排错建议入口） |
| **关联任务** | #96（v0.9.x-ACTIVITY-TIMELINE: Action Details 步骤时间线 + 耗时, 对标 Arcami）|
| **参考** | SupDRC v1.2.0 跨平台恢复 UI（阶段表 + hover 卡）/ Trilio / Kasten Action 详情 |

> **关键技术风险（必须诚实标记）**：Velero `Backup`/`Restore` CR 顶层只有粗粒度 `.status.phase`（InProgress/Completed/Failed 等）, **没有原生 step-by-step phase 字段**。本 PRD 需要 SupKube backend **合成阶段层**：从 Velero CR + 子 CR（`DataUpload` / `DataDownload` / `PodVolumeBackup` / `PodVolumeRestore` / `VolumeSnapshot` 等）的 `.status` 推导出 5-8 个生命周期阶段。**合成层是本 PRD 的技术承重柱, Phase 0 必须先实测确认可行（verify-before-architect）**。

### 1. Goal

客户点击 Activity 列表中任一 Backup / Restore 任务卡片, 进入详情时**一眼看到任务跑到哪一步、卡在哪一步、卡的原因**, 无需切窗口翻 `kubectl describe`；错误行可一键跳转 Log Viewer 上下文, 未来可接 AI Advisor 深度排错。

### 2. Epic

ADR-031 "全职灾备运维专家" Epic 的**可观测面**——专家在执行 Backup/Restore 时, 客户应像看 CI/CD pipeline 一样看到**每一步状态、耗时、吞吐、错误**, 而不是面对一个黑盒 `InProgress` 转圈。

### 3. User Stories

| # | 角色 | 场景 |
|---|---|---|
| 3.1 | 运维 | **Backup 进行中**: 看到当前在"DataMover 上传"阶段, 进度 62%, 吞吐 45 MB/s, 已耗 8min。**（finding #3 2026-05-31）**: ~~估算剩余 5min~~ **删除 ETA 承诺**（备份 ETA 极不准, 客户会基于不准的承诺做事 → 被吐槽）, 改为只显示"已耗 8m 12s" + "进度 62%", 让客户自己估; 未来若研发觉得有把握再加 `etaSec` 字段且明确标"粗略估算 ± X%"|
| 3.2 | 运维 | **Restore 失败**: 看到 5 个阶段, 第 4 步"DataDownload"红色失败, 点错误行直接跳到 Log Viewer 该 Pod 的对应时间窗 grep 上下文 |
| 3.3 | 运维 | **跨集群 Restore**: hover 阶段卡显示运行时详情（实际 DataDownload CR 名 / namespace / 源 Snapshot ID / 目标 PVC / 分块数 / 服务节点 IP）, 用于工单排查 |
| 3.4 | 评审 | **集群整体故障**: 列表筛选 failed, 批量点详情看哪一步集体卡, 判断是 BSL 网络还是 CSI 驱动问题 |

### 4. Functions

#### 4.1 Velero 内部阶段映射（合成层 · Phase 0 必须先实测）

**Backup 阶段定义**（5 阶段, 按 Velero 真实生命周期）：

| # | 阶段名 | 触发判定 | 完成判定 | 数据源 |
|---|---|---|---|---|
| 1 | **Validation** | Backup CR created, phase=New/Validating | phase 转 InProgress 或 ValidationFailed | Backup CR `.status` |
| 2 | **Snapshot** | phase=InProgress 且任意 VolumeSnapshot/CSI snapshot 创建 | 所有 snapshot=ReadyToUse 或本次无快照 | `VolumeSnapshot` CRs |
| 3 | **DataMover** | 任意 `DataUpload` CR 存在 | 所有 DataUpload=Completed | `DataUpload` CRs |
| 4 | **Finalize** | phase=Finalizing | phase=Completed/PartiallyFailed/Failed | Backup CR `.status.phase` |
| 5 | **Done** | 终态 | — | Backup CR |

**Restore 阶段定义**（5 阶段）：

| # | 阶段名 | 触发判定 | 完成判定 | 数据源 |
|---|---|---|---|---|
| 1 | **Validation** | Restore CR created, phase=New | phase 转 InProgress | Restore CR |
| 2 | **PreRestore** | InProgress 且 ItemOperation 创建 hooks | hooks 完成 | Restore CR + ItemOperations |
| 3 | **DataMover** | 任意 `DataDownload` CR 存在 | 所有 DataDownload=Completed | `DataDownload` CRs |
| 4 | **PostRestore** | phase=Finalizing 或 PostHook 阶段 | phase=Completed/PartiallyFailed/Failed | Restore CR |
| 5 | **Done** | 终态 | — | Restore CR |

> **可行性风险与缓解**：
> - **风险**: Velero 不同版本（1.11/1.12/1.13/1.14）子 CR 字段命名 + 出现时序有差异
> - **缓解**: Phase 0 在 AKS dev + docker-desktop 实测 3 个真实 Backup + 3 个 Restore（含 CSI / 跨集群 / 失败注入）, **抓真实 CR yaml 入库作为 fixture**, 据此写合成器
> - **降级**: 若某阶段无法从 CR 推导（如旧版无 DataUpload）, 阶段状态显示 `unknown`+ 灰色 placeholder, 不假装绿色

#### 4.2 Backend API

| Method | Path | 用途 |
|---|---|---|
| GET | `/actions/:id/timeline` | 返回 `stages[]`, 每条含 `name / status / progressPct / throughputBps / startTime / endTime / durationSec / errorMessage / errorRef / runtimeDetails{}` |
| GET | `/actions/:id/timeline?detail=full` | 多带子 CR 全量字段（hover 卡用）, 默认 lite 节省带宽 |

返回示例（Backup）：
```json
{
  "actionId": "backup-prod-orders-20260531-0200",
  "type": "Backup",
  "topStatus": "InProgress",
  "stages": [
    { "name": "Validation", "status": "completed", "durationSec": 3 },
    { "name": "Snapshot",   "status": "completed", "durationSec": 18, "runtimeDetails": { "snapshotCount": 4 } },
    { "name": "DataMover",  "status": "running",   "progressPct": 62, "throughputBps": 47185920,
      "runtimeDetails": { "dataUploadCRs": ["du-xxx-1","du-xxx-2"], "chunkSize": "10MB", "concurrency": 4, "nodeAgentIP": "10.0.1.23" } },
    { "name": "Finalize",   "status": "pending" },
    { "name": "Done",       "status": "pending" }
  ]
}
```

> **刷新策略**：running 状态轮询 5s（与 Activity 列表一致）; 终态后只取一次缓存 60s。

#### 4.3 Activity 卡片 enrich（列表层增强, 不改卡片高度结构, 仅补字段）

在 `Activity.vue` 当前卡片右侧 phases 区域追加：

| 字段 | 显示 | 数据源 |
|---|---|---|
| 当前阶段名 | 大字 `DataMover` | `stages[*].status=running` 取第一条 |
| 进度 | mini progress bar 62% | 该阶段 progressPct |
| 吞吐 | `45 MB/s` 副文本 | 该阶段 throughputBps |
| 总耗时 | `8m 12s` | sum(durationSec) |

> 列表卡片**不调用** `/timeline` per-card（性能爆炸）, backend 在 `ListActions` 响应中**预聚合**返回这 4 个字段（轻量摘要, 不含 runtimeDetails）。

> **finding #4 (Med) 2026-05-31 成本约束**: ListActions 预聚合**仅对 running action 执行**（终态 action 缓存结果, 不重新合成）, 终态缓存有效期 60s, 每页 action 数量上限 50, 子 CR 读取并发上限 20（避免一页 50 个 Backup × 4 个子 CR = 200 个并发 list 把 K8s API server 打挂）。超出页大小 → 分页 + 客户端按需加载。终态 action 缓存 key: `(actionID, action.status.phase, latest_subcr_uid_hash)`, phase 切换时缓存自动失效。

#### 4.4 详情抽屉改造（`ActionDetailDrawer.vue`）

**现有结构保留**, 把 PHASES section 从 `ActionPhases` 简单列表升级为**阶段表**：

| 列 | 内容 |
|---|---|
| 阶段名 | Validation / Snapshot / DataMover / Finalize / Done |
| 状态 | chip: pending / running / completed / failed / skipped / unknown |
| 进度 | mini progress bar（仅 running）|
| 吞吐 | `45 MB/s`（仅 DataMover 阶段）|
| 耗时 | `8m 12s` |
| 错误 | 红色文字 + `[查看日志 →]` 按钮（失败/警告时）|

#### 4.5 Hover 卡（运行时详情）

阶段行 hover → 弹悬浮卡, 显示 `runtimeDetails`:
- Snapshot 阶段: snapshot 数 / 总字节 / VolumeSnapshot CR 名
- DataMover 阶段: DataUpload/DataDownload CR 名 + namespace / 源设备 / 总字节 / 并发数 / 分块大小 / Node Agent Pod IP / Volume URN / Resource ID
- Finalize 阶段: 处理对象数 / warning 计数

> 设计上对齐 Mars 给的 SupDRC v1.2.0 hover 卡参考, 但字段名走 Velero 真实语义, 不一比一抄。

#### 4.6 跳 Log Viewer（错误行点击）· **X1 改写 2026-05-31: 引用 PRD-005 §4.8 权威 schema**

> **X1 (High) 2026-05-31**: deep-link query-param 协议 **不在本节定义**, 引用 **PRD-005 §4.8** 权威 schema 为单一来源（路由 `/observability?tab=logs` + 严格参数枚举: component / sinceSeconds / severity / grep / scrollToLine / live / t）。本节仅描述"跳转触发逻辑 + 参数填充策略", schema 本身以 PRD-005 §4.8 为准。

**触发**: 点击错误行 `[查看日志 →]` → 按 PRD-005 §4.8 schema 拼 URL。

**示例**（DataMover 阶段失败 → 跳 node-agent 日志）:
```
/observability?tab=logs&component=node-agent&grep=du-xxx-1&sinceSeconds=600&scrollToLine=auto&t=1717100123000
```

**参数填充策略**（本 PRD 决定填什么值, schema 以 PRD-005 §4.8 为准）:

| 参数 | 本 PRD 填值策略 |
|---|---|
| `component` | 按失败阶段映射: DataMover → `node-agent`; Validation/PostRestore → `velero`; 控制器层错 → `backend` |
| `grep` | 失败子 CR 名（DataUpload/DataDownload name）或 Pod 名或 Resource UID |
| `sinceSeconds` | (阶段结束 timestamp - 阶段开始 timestamp) + 60s 缓冲, 上限 3600s |
| `scrollToLine` | 固定填 `auto`（自动滚到第一条 ERROR, 由 PRD-005 后端实现）|
| `t` | 失败时刻 epoch-ms（让 Log Viewer 在该时间附近开窗）|

> **依赖**: PRD-005 §4.8 schema **已经是权威单一来源**（2026-05-31 修订）, 本节实施时**不许另定 schema**。**若 PRD-005 v2 未就绪**, fallback 方案: 详情抽屉内嵌 **mini log viewer**（只读 200 行 + grep 高亮, 不含全功能交互）, 标记"Lite 模式"。**Phase 5 验收时按 PRD-005 实际状态择一**。

#### 4.7 AI Advisor tab 占位（依赖 PRD-003 评审通过）· **X4 复用脱敏管线 2026-05-31**

详情抽屉新增 tab `[💡 AI 建议]`, v1 仅占位（"该功能需 AI Advisor 启用, 见 Settings"）；PRD-003 落地后填实际内容。**finding #5 (Med) 2026-05-31**: AI tab **仅在终态 failed / partial 显示** (running 时 tab 隐藏, 避免半成品建议; 与 Q8 倾向一致, 落地为强制行为)：
- 输入: 该 Action 的 stages[] + 失败阶段 errorMessage + 关联资源
- 输出: "卡在阶段 4 DataDownload, 因为目标集群 PVC 容量不足。建议: ① 扩容 PVC; ② 重启 DataDownload"
- 复用 PRD-003 `/advisor/preflight` 端点的同类 prompt 模板（新增一个 `troubleshoot.tmpl`, 不本 PRD 交付）
- **X4 (High) 2026-05-31**: 任何 stages[] / errorMessage / 子 CR yaml 外发, **必须经 PRD-005 §7 脱敏管线 + PRD-003 §7.2 T4 外发治理统一管线**, 不另做一套。失败阶段 errorMessage 常含路径 / 用户名 / IP / token, 默认 redact, 客户在 Settings 显式 opt-in "完整 errorMessage 给 LLM" 才发送原文。审计 log 必须记录 `redacted_field_paths`（沿用 PRD-003 §7.2 标准）。

#### 4.7bis Velero 版本兼容矩阵 + CI fixture（finding #2, 2026-05-31 新增）

**问题**: Velero 不同版本（1.11 / 1.12 / 1.13 / 1.14）的 Backup / Restore / DataUpload / DataDownload CR 字段差异**真实存在**, 本 PRD §4.1 合成层是承重柱, 必须在每版本上**实测**否则线上必崩。

**版本兼容矩阵**（v1 必测）:

| Velero | Backup CR | DataUpload | DataDownload | PodVolume* | 阶段合成器需调整? |
|---|---|---|---|---|---|
| 1.11.x | ✅ | ✅ | ✅ | ✅ | baseline |
| 1.12.x | ✅ | ✅ (字段微调) | ✅ | ✅ | snapshot status field 一处改名 |
| 1.13.x | ✅ | ✅ | ✅ (新增 progress 字段) | ✅ | DataDownload progress 优先用新字段 |
| 1.14.x | ✅ | ✅ | ✅ | ✅ | 待 Phase 0 实测 |

**CI 契约测试**:
- Phase 0 抓的真实 CR fixture 按 Velero 版本分目录存: `testdata/velero-cr-fixtures/v1.11/`, `v1.12/`, ...
- 单测对每个支持版本各跑一遍合成器, 校验输出 stages[] schema 一致 (status 取值 / 字段非空等)
- **新版本上线前刷新**: Velero 升级时（如 1.15 发布）, ROADMAP 新增一个 "Velero 兼容性 refresh" 任务, 必须先抓新版 fixture + 跑通 CI 测试, 才能升级支持矩阵

### 5. UI / UX

**(a) Activity 卡片 enrich**（最小侵入式）

```
┌─ Backup · prod-orders-20260531 ─────────────────────────┐
│ [Running]  Backup  [Local]                              │
│ backup-prod-orders-20260531-0200                        │
│                                                          │
│ 当前阶段: DataMover                                      │
│ ████████░░░░ 62% · 45 MB/s · 已耗 8m 12s                │
│ (finding #3 2026-05-31: 删除 ETA 承诺, 已耗 + 进度 + 吞吐让客户自己估)│
└──────────────────────────────────────────────────────────┘
```

**(b) 详情抽屉阶段表**

```
┌─ ACTION DETAILS ───────────────────────────────────────────┐
│ backup-prod-orders-20260531-0200                            │
│ [Running] [Backup] [Local]                                  │
│                                                             │
│ ── PHASES ──────────────────────────────────────────────── │
│ # 阶段        状态     进度      吞吐      耗时    错误     │
│ 1 Validation ✓ 完成   —         —        3s     —         │
│ 2 Snapshot   ✓ 完成   —         —        18s    —         │
│ 3 DataMover  ⏳ 运行  ███ 62%   45MB/s   8m12s  —         │
│ 4 Finalize   ○ 等待   —         —        —      —         │
│ 5 Done       ○ 等待   —         —        —      —         │
│                                                             │
│ ── DETAILS ──────────────────────────────────────────────  │
│ Protected Object: ns/prod-orders                            │
│ Policy: daily-prod-2am                                      │
│ ...                                                         │
│                                                             │
│ ── ERRORS ──── (失败时显示)                                │
│ Stage 3 DataMover:                                          │
│   DataUpload du-xxx-1 failed: snapshot read timeout         │
│   [查看日志 →]                                              │
│                                                             │
│ Tabs: [Overview] [Items] [💡 AI 建议 (Coming)]              │
└─────────────────────────────────────────────────────────────┘
```

**(c) Hover 卡（阶段 3 DataMover）**

```
   ┌─ DataMover runtime ────────────────────┐
   │ DataUpload CRs:                         │
   │   du-xxx-1 (ns/velero)                  │
   │   du-xxx-2 (ns/velero)                  │
   │ Source PVC: prod-orders/mysql-data      │
   │ Total bytes: 42 GiB / 68 GiB            │
   │ Concurrency: 4 · Chunk: 10 MB           │
   │ Node Agent: node-3 (10.0.1.23)          │
   │ Backup Repository: kopia-azureblob-...  │
   └─────────────────────────────────────────┘
```

**i18n 新键**（en + zh-CN, prefix `activity.detail.timeline.*`）：
`stages` / `stageName` / `stageStatus.{pending,running,completed,failed,skipped,unknown}` / `progress` / `throughput` / `errorJumpLog` / `runtimeDetails.*` / `aiAdvisorComingSoon`

### 6. Out of Scope

| 项 | 原因 | 去向 |
|---|---|---|
| 阶段**编辑** / 撤销 / 重试 | 改变 Velero 行为, 风险高 | v1.x 评估 |
| **Backup Copy 阶段**（跨 BSL 复制）| 依赖 #111 BSL Copy Feature | #111 落地后增量 |
| **KubeVirt 虚机特殊阶段**（FreezeFS/Quiesce）| 依赖 #93 KubeVirt 集成 | #93 落地后增量 |
| **历史阶段时间线对比**（同 Policy 多次 Backup 对比）| v2 体验增强 | v0.10.x |
| **阶段事件流（kubectl events 风格）**| 与日志重复, 留 Log Viewer 承载 | PRD-005 |

### 7. 非功能性要求

| 维度 | 要求 |
|---|---|
| 刷新频率 | 详情抽屉打开时 running 状态 5s 轮询; 终态后停 |
| 大量阶段 | v1 阶段数硬上限 8; 未来 #111/#93 扩展时若 >12 需评估 UI |
| 历史保留 | 阶段数据**不独立持久化**, 实时从 Velero CR 合成; CR GC 后阶段消失（与 Velero TTL 一致）|
| 性能 | `/timeline` P95 ≤ 800ms（含 CR list）; `ListActions` 增加预聚合后整体 P95 ≤ 1.2s |
| i18n | 全部走 i18n; 阶段名 keep English（Validation/DataMover）+ tooltip 中文释义 |
| 权限 | 与 `GetAction` 同, viewer 即可读 |

### 8. 验收标准（Definition of Done）

| # | 验收点（可实测） |
|---|---|
| 1 | Phase 0 fixture: AKS dev + docker-desktop 各抓 3 个真实 Backup CR + 3 个 Restore CR（含子 CR yaml）, 存入 `supkube-backend/internal/api/v1/testdata/velero-cr-fixtures/`, 至少覆盖 CSI snapshot / DataMover / Failed 三种 |
| 2 | Backup `/actions/:id/timeline` 返回 stages 必含 **Snapshot + DataMover + Finalize** 三阶段, 每阶段独立 status 字段 |
| 3 | Restore `/actions/:id/timeline` 返回 stages 必含 **Validation + DataMover + PostRestore** 三阶段, 每阶段独立 status 字段 |
| 4 | 阶段 status 取值严格枚举 `pending / running / completed / failed / skipped / unknown`, 单测覆盖每种取值 |
| 5 | 一个真实 running Backup: DataMover 阶段 `progressPct` ∈ [0,100], `throughputBps` > 0, 5s 内值会更新 |
| 6 | 一个真实 failed Restore: 失败阶段 `errorMessage` 非空且与 Velero CR `.status.failureReason` 一致, `errorRef` 含失败子 CR 的 name + namespace |
| 7 | Activity 卡片新字段（当前阶段 / 进度 / 吞吐 / 已耗时）通过 `ListActions` 预聚合返回, **未发起 per-card /timeline 调用**（network tab 验证）|
| 8 | 详情抽屉 PHASES section 渲染为表格（5 列: 阶段名/状态/进度/吞吐/耗时 + 错误时 6 列）, 替换原 `ActionPhases` 简单列表 |
| 9 | hover DataMover 阶段, 悬浮卡显示真实 DataUpload CR 名 + namespace + 并发数 + Node Agent IP 至少 4 项 |
| 10 | 失败阶段 `[查看日志 →]` 点击 → 浏览器 URL 出现 `/observability?tab=logs&component=...&grep=...&sinceSeconds=...`（不必 Log Viewer 端真正高亮, 仅校验 URL 协议）|
| 11 | PRD-005 Log Viewer v2 落地后, 同样点击能在 Log Viewer 滚动到对应行（Phase 5 联调）|
| 12 | AI Advisor tab 在 PRD-003 未启用时显示占位文案 + Disabled 状态, 不报错 |
| 13 | `/timeline` 在 Velero 旧版无 DataUpload CR 时, DataMover 阶段降级为 `unknown` 灰色, 不假装绿色 |
| 14 | i18n: zh-CN + en 双语完整, 含 6 个阶段状态 + hover 卡字段 |
| 15 | 前端 `npm run build` 通过; 后端 `go build ./...` + 阶段合成器单测无回归 |

### 9. 任务拆分

| Phase | 内容 | 工期 |
|---|---|---|
| **Phase 0 — 真实 CR 实测 + fixture 入库**（承重前置）| AKS dev + docker-desktop 抓真实 Backup/Restore + 子 CR yaml; 写合成器决策表 | **0.5 周** |
| **Phase 1 — Backend 合成器 + API** | `internal/timeline/` 新 module; `/actions/:id/timeline` 端点; ListActions 预聚合 4 字段 | 1.5 周 |
| **Phase 2 — 前端 Activity 卡片 enrich** | `Activity.vue` 卡片字段; 不破现有 layout | 0.3 周 |
| **Phase 3 — 详情抽屉阶段表 + hover 卡** | `ActionDetailDrawer.vue` PHASES section 重做; 新增 `ActionStageTable.vue` + `ActionStageHoverCard.vue` | 1 周 |
| **Phase 4 — Log Viewer 跳转协议 + AI tab 占位** | URL 协议 + AI tab disabled 占位; Lite mini-log 兜底（若 PRD-005 v2 未就绪）| 0.5 周 |
| **Phase 5 — i18n + 联调 + E2E** | 双语 + 跨 Velero 版本走查 + 文档 | 0.7 周 |

**总计 4.5 周**（一人全职）

### 10. 关联文档与任务

- **#96**（v0.9.x-ACTIVITY-TIMELINE）: 本 PRD 直接落地任务, 已在 backlog
- **PRD-005**（Log Viewer v2）: §4.6 跳转目标, query-param 协议对齐
- **PRD-003**（AI Advisor）: §4.7 AI 建议 tab 数据源
- **ADR-031**: "全职 SRE 专家" Epic 的可观测面
- **#111**（Backup Copy）: Backup 阶段未来扩展, 本 PRD Out-of-Scope
- **#93**（KubeVirt）: 虚机阶段未来扩展, 本 PRD Out-of-Scope
- **现有代码**:
  - `supkube-frontend/src/views/Activity.vue`（卡片列表）
  - `supkube-frontend/src/components/ActionDetailDrawer.vue`（详情抽屉, PHASES section 待重做）
  - `supkube-frontend/src/components/ActionPhases.vue`（旧简单列表, 本 PRD 升级）
  - `supkube-backend/internal/api/v1/handlers.go`（ListActions / GetAction, 待补 timeline 端点）

### 11. 开放问题

| Q | 问题 | 倾向 |
|---|---|---|
| Q1 | Backup 阶段数 5 个 vs 7 个（拆 Snapshot 为 CSISnapshot+PodVolumeBackup 两步）? | **5 个**（v1 简洁; 区别在 hover 卡）|
| Q2 | 阶段名称用英文（Validation/DataMover）还是中文（验证/数据迁移）? | **英文阶段名 + i18n tooltip 中文释义**（与 Velero 文档对齐, 客户搜文档不歧义）|
| Q3 | 进度百分比来源: DataUpload `.status.progress.totalBytes/bytesDone` 还是估算? | **优先真实字段**; 无字段时 hide 进度条（不假装）|
| Q4 | 吞吐 = 当前瞬时 / 最近 10s 滑动 / 累积平均? | **最近 10s 滑动**（backend 缓存上次时间点计算）|
| Q5 | 跳 Log Viewer 用 router push 还是新 tab? | **新 tab**（保详情抽屉状态, 与 PRD-001 v2 同风格）|
| Q6 | Log Viewer query param `component` 枚举怎么定? | 与 PRD-005 v2 联合定义, 本 PRD 列 5 个: velero / node-agent / kopia / supkube-backend / csi-driver |
| Q7 | mini-log fallback 是否值得做（如 PRD-005 v2 未就绪）? | **做最小版**（200 行 + grep 高亮）, 1-2 天能搞定, 降低 PRD-005 阻塞风险 |
| Q8 | AI 建议 tab 在 Action 终态 vs running 都显示? | **仅终态 = failed/partial 显示**, running 时隐藏（避免半成品建议）|
| Q9 | `/timeline` 是否需要 SSE 流式推送（代替 5s 轮询）? | **v1 走 5s 轮询**（与 Activity 列表一致, 实现简单）; SSE 留 v1.x 评估 |

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Claude | — → **草稿** | 起草。基于 Mars 看 SupDRC v1.2.0 截图 + #96 backlog + Velero 真实生命周期。**关键技术风险已诚实标记**: Velero 无原生 step phase, 合成层有 Phase 0 实测前置。Q1-Q9 待 Mars 拍板。待 Mars 评审决定是否 → 排队评审。 |
| 2026-05-31 | Mars (评审人 Claude 委托) | 草稿 → **草稿 (修订)** | 落 X1 + X4 + 4 specific finding (pre-评审 cleanup, 状态保持草稿): (1) **X1 (High)** §4.6 改写为 "deep-link 引用 PRD-005 §4.8 权威 schema, 不另写"; 跳转用 `/observability?tab=logs&component=X&sinceSeconds=N&grep=Y&scrollToLine=auto` 格式; 本节仅决定"填什么值", schema 以 PRD-005 §4.8 为准; (2) **X4 (High)** §4.7 AI 排错 tab 外发, 加 "经 PRD-005 §7 脱敏管线 + PRD-003 §7.2 T4 外发治理统一管线"; (3) **finding #2 (Med)** 新增 §4.7bis Velero 版本兼容矩阵 (1.11/1.12/1.13/1.14) + fixture 化为 CI 契约测试, 新版本上线前刷新; (4) **finding #3 (Med)** US 3.1 "估算剩余 5min" → **删 ETA**（备份 ETA 极不准, 改为只显示已耗 + 进度 + 吞吐让客户自己估; mockup 同步更新）; (5) **finding #4 (Med)** §4.3 ListActions 预聚合成本约束: 仅对 running action 执行, 终态缓存 60s, 每页 ≤50 action, 子 CR 读并发 ≤20; (6) **finding #5 (Med)** §4.7 AI tab **仅终态 failed/partial 显示** (Q8 倾向落地为强制行为)。**状态保持草稿** (本次为 pre-评审 cleanup, 评审在下次). |
| 2026-05-31 | Mars | 草稿 → **✅ 已评审** | 通过, 可进研发. |

---

<a id="prd-007"></a>
## PRD-007 — 完整 3-2-1-1-0 数据韧性（5 层可视化 + Layer 4 Backup Copy + Fingerprint + Lifecycle + DR Drill）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-007 |
| **任务编号** | #126 |
| **状态** | **已评审（2026-05-31）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.10.x（与 PRD-003 AI Advisor 同窗口；实施依赖 #88 / #109 / #111 已落地）|
| **关联 ADR** | ADR-031（5 层数据韧性原则）/ ADR-029（本地快照战略）/ ADR-025（双 Schedule pair, L2 模型）/ ADR-026（Velero v1.18 限制）/ ADR-033（拟, AI Advisor 评分整合）|
| **关联 PRD** | PRD-001（Restore Preflight, restore 端）/ PRD-002（Transform, 还原适配）/ PRD-003（AI Advisor, 推荐 3-2-1-1-0 配置 + Resilience Score 共享）/ PRD-006（Activity Timeline, Backup Copy 阶段可见）|
| **关联文档** | SECURITY.md §4 Object Lock / §5 3-2-1-1-0 + DR Drill v0.9.7 / USER_MANUAL.md §RBAC / ROADMAP.md "数据韧性权威模型" / 架构设计.md ADR-031 |

> **立项缘由（2026-05-31）**：Mars 主动询问"从快照到长期云端保留 with immutability, 3-2-1-1-0 的功能是否已经写了 PRD？" 答案是**没有**。现状是 ADR-031 写了 5 层设计原则、LBS 系列 (#56/#57/#58/#59) 落了基础（本地 BSL / Policy 选择器 / Object Lock UI / 评分卡），但**缺一份 PRD 把"客户期望的完整 3-2-1-1-0 端到端 user journey"串起来**（装 → 备份 → 分层保留 → 跨云复制 → 不可变 → 长期归档 → DR drill 验证）。本 PRD 就是这份串联。

### 1. Goal

把"数据韧性"从设计原则（ADR-031）落地为**可销售的端到端产品能力**：客户在 SupKube UI **一眼就能看到自己处在 3-2-1-1-0 的哪个 level**、缺哪一层、以及**一键升级路径**；5 层（本地快照 / 本地 BSL / 云 BSL / 第 2 云 Backup Copy / 虚拟实验室 DR Drill）**全部可视化 + 一键 ship**。

### 2. Epic

ADR-031 "Kasten / Veeam 级数据保护平台" Epic 的**产品化落地**——把 5 层韧性原则从 ADR 转化为 user-facing feature；让客户不需要看 ADR 也能在 UI 里走完整套 3-2-1-1-0 配置流程。

### 3. User Stories

- 作为 **Platform Lead**，装完 SupKube 后**第一屏**（Dashboard）就看到 "Resilience Posture" 卡片（与 PRD-003 §3.3 共享评分引擎），显示集群当前 3-2-1-1-0 哪几层 ✅ / ⚠ / ✗。
- 作为 **Platform Lead**，我能**一键启用**缺的层——例如 "你只有 Layer 2 本地 BSL, 缺 Layer 3 云 BSL, 一键启用 →" 点击后打开 BSL 配置 Wizard（不让我去翻菜单找）。
- 作为 **SRE**，我能在 Policy 编辑里勾选 "我要 3-2-1-1-0 完整保护"，SupKube 自动生成所需的 schedule pair + cloud BSL + Object Lock + Layer 4 Backup Copy 配置（**不让我手动配 4 个东西**）。
- 作为 **Compliance Officer**，我能下载一份 "我有 3-2-1-1-0 哪几层 + 每层最近一次成功时间 + 数据量 + Object Lock 状态" 的 PDF 报告（拿去给审计 / 等保 / SOC2）。
- 作为 **Backup Admin**，我能看到**长期归档视图**：30 天热（Standard）→ 90 天温（IA）→ 365 天冷（Glacier）→ 7 年后删除，每段都显示当前对象数 / 容量 / WORM Object Lock 剩余时间。
- 作为 **SRE**，我能**一键 DR Drill**：选一个 RP → 还原到沙箱 ns → 跑业务 smoke test → 0 错误 score；1 小时后 SupKube 自动清理沙箱。
- 作为 **SRE**，跨集群 Export/Import RP 时, 我能看到 source cluster fingerprint + integrity hash, **确认拿到的就是原版**（防止 BSL 数据被外部工具篡改后误恢复脏数据）。
- 作为 **SRE**，备份完成后 SupKube **自动 CRC 校验**, 完整性不通过的 backup 直接标 "✗ 完整性失败"，**不让我误以为成功**。

### 4. Functions

#### 4.1 5 层数据韧性映射表（与 ADR-031 对齐）

| Layer | 名称 | 现状 | 本 PRD 范围 |
|---|---|---|---|
| **L1** | 本地快照（CSI Snapshot） | ✅ 已实施（v0.8.x） | 集成可视化, 与 PRD-006 Activity Timeline 衔接 |
| **L2** | 本地 BSL（MinIO） | ✅ #56 LBS1 已实施 | UI 显示 "本地 BSL 在线 ✅" + 空间使用率 + Object Lock 状态 |
| **L3** | 云 BSL（S3 / Azure Blob / GCS） | ✅ #57 LBS2 已实施（Policy UI 选择器） | 加 "推荐你启用 + 一键配置" wizard |
| **L4** | **第 2 云 Backup Copy**（object-to-object 复制, 非重发） | ❌ #111 待做（**本 PRD 核心新功能**）| API + 控制器 + UI（rclone / aws s3 cp / azcopy / gsutil）|
| **L5** | 虚拟实验室 DR Drill（沙箱 ns） | 🟡 部分（v0.9.7 规划） | 本 PRD 立沙箱模型 + 一键 / 月度自动 drill |

**1-1-0 部分**：
- 第 2 个 "1" = Immutability (Object Lock) → 已由 #58 LBS3 落地，本 PRD §4.2 加冲突预检 + 审计
- "0" = 0 错误验证 → 本 PRD §4.6 自动 CRC 校验 + §4.7 DR Drill smoke test

#### 4.2 Object Lock / Immutability（基于 #58 LBS3 增强）

| 子功能 | 状态 | 本 PRD 增量 |
|---|---|---|
| Object Lock UI（开启 / 保留期 / 模式 Compliance vs Governance）| ✅ #58 已实施 | — |
| 不可变保留期可视化（剩余天数 hover） | ✅ #58 已实施 | — |
| **Object Lock 冲突预检** | ❌ 新增 | BSL 类型不支持 Object Lock 时给 UI 警告（e.g., GCS 无原生 Object Lock, 提示用 retention policy 替代; S3 兼容存储需校验 `x-amz-bucket-object-lock-enabled` header）|
| **Object Lock 审计** | ❌ 新增 | 谁开启 / 谁延长 / 谁尝试删除被拒 → 全部写审计日志（复用 SECURITY.md §4 现有机制）；审计字段 `actor / action / target_rp / before / after / result` |
| **Kopia 维护 vs Object Lock 兼容性**（Med #1）| ❌ 新增 | 对启用 Object Lock 的 BSL (尤其作为 Layer 4 copy target 时), **Kopia 维护 (compaction / GC) 会删/改写对象**, 可能与 WORM immutable 冲突 → 导致仓库膨胀 / 维护失败。**Phase 0 必测**该行为；必要时对 copy-target BSL **关闭 Kopia 维护**或启用 **Kopia immutable-storage 模式** (官方 v0.15+ 支持)。Phase 0 实测结果写入 ADR-031 §X 补遗。|

#### 4.3 Backup Copy（Layer 4, #111 核心新功能）

> **【v1.1 重写, 2026-05-31】** 原 v1 草稿描述的「逐对象复制某个备份」语义在 Velero/Kopia 模型下会产生 "可见但不可恢复" 的静默数据丢失。本节按 PRD-Review-2026-05-31-PRD007 §P1 重写, 钉死适用边界与复制粒度。

**适用边界（硬约束, 优先于其他描述）**:

Layer 4 Backup Copy **仅适用于数据驻留在 BSL 的备份**——即 data-mover 备份 / fs-backup (Restic/Kopia) / `snapshotMoveData=true` 的 CSI 备份。**快照型备份（`snapshotMoveData=false` 的原生 CSI 快照）不能走 Layer 4 object copy**, 原因: BSL 上只有 K8s 资源 tarball + 元数据, **真实卷数据在云厂商的区域快照里**（ADR-031 §1 已实测区分）, 对这类备份做 BSL→BSL 对象复制, 复制出的备份**恢复时没有卷数据**。快照型备份的跨区域复制必须走**快照级复制**（云厂商区域快照复制 API）, 属另一机制 / 另一 PRD。

**复制粒度（必读, 与 Velero/Kopia 布局对齐）**:

Velero 的对象存储布局有两个硬事实, 决定 Layer 4 的复制单元 **不是** "某个备份" 而是 **仓库级 / BSL 级**:

1. **Kopia 仓库按 ns 共享, 不按备份**: Velero 把 PodVolumeBackup / data-mover 的数据写进 `bucket/kopia/<ns>`, **该 ns 所有备份共享、内容寻址去重**。要让一个备份在 target BSL 可恢复, 必须连带复制整个 `kopia/<ns>` 仓库 (及 `backups/<name>/` 元数据), 而不是某个备份的对象子集。
2. **Velero 原生不支持单备份跨 BSL 复制**: 官方明示 "不能把单个 Velero backup 同时送到多个 BSL", 跨区域推荐云原生复制 (S3 CRR / Azure Object Replication / GCS bucket replication)。

因此本 PRD 的复制粒度定义为:
- **source 单元** = `bucket/kopia/<ns>/` 仓库 + `bucket/backups/<name>/` 元数据 (即整桶 / 整 ns 级 rclone sync)
- **不是** 早期草稿 UI 暗示的 "按备份挑对象" (该 UI 形态已砍掉, 见下方 UI 段)
- **增量性**来自 Kopia 内容寻址 (不可变 chunks) + rclone sync 算法 → 仓库每次同步只传新块, 已存在的块零成本跳过

**实现策略（按云原生优先）**:

- **v1.0**: **rclone sync** 作为跨云统一抽象（S3 ↔ Azure ↔ GCS）, 仓库级 sync, 不承诺秒级延迟。Phase 0 实测吞吐, 不达标按 ADR-031 退路回退 v1 仅 S3↔S3。
- **v1.1+**: 同云跨区域 / 同账号跨桶时优先用**云原生复制**: AWS S3 CRR (Cross-Region Replication) / Azure Object Replication / GCS bucket replication。云原生方案在性能/成本/可靠性都优于 rclone 用户态拉取, 但仅适用于同云。
- **跨云 (e.g. AWS → Azure)** 长期仍走 rclone / 等价工具, 无云原生路径。

**API**:
```
POST   /api/v1/policies/:name/backup-copy
       { sourceBSL, targetBSL, scope: "namespace|bucket", namespaces: ["ns-a","ns-b"],
         trigger: "cron|immediate|on-success", schedule: "0 3 * * *", rateLimitMBps: 100,
         engine: "rclone|s3-crr|azure-or|gcs-replication" }
GET    /api/v1/policies/:name/backup-copy
GET    /api/v1/backup-copy/:id           # 单次复制任务状态
DELETE /api/v1/policies/:name/backup-copy/:id
```

> 注: `scope` 默认 `namespace` (复制指定 ns 的整个 Kopia 仓库 + 元数据); 选 `bucket` 整桶 sync 用于多租户极简场景。`engine` 默认 `rclone`, 同云场景可选云原生引擎 (v1.1)。**不再提供 "选某几个 backup 复制" 选项**。

**Preflight 拦截（在 #109 Phase 4 实施）**:

客户配 Layer 4 → 后端扫描 source BSL 上 backup 列表 → 任一备份是快照型 (`snapshotMoveData=false` 且无 fs-backup volume) → **拦截 + 提示**:
- 错误码: `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`
- 错误文案: "该备份不能走 Layer 4 object copy (其卷数据在云端区域快照, 不在 BSL)。应在 Policy 配 `snapshotMoveData=true` 让数据进 BSL, 或改用云端区域快照复制 (另一机制)。"

**实现细节**:
- 后端 worker 用 rclone (v1.0) / 云原生 SDK (v1.1) 执行**仓库级 sync** (不是 per-backup copy)
- 凭据来源: 复用 ADR-004 现有凭据管理 + K8s Secret; 跨云 IAM 走最小权限（read source bucket + write target bucket, **不允许 admin**）
- 断点续传: rclone 内置 (按 chunk hash 续传); 失败 retry 3 次, 指数退避; 速率限制可配（默认 100 MBps）
- Kopia 维护与 immutable target 兼容性: 见 §4.2 Med #1 说明

**UI**:
- Policy 编辑加 "Layer 4 Backup Copy" tab:
  - source BSL（下拉, 限本 Policy 已绑定的 BSL）
  - target BSL（下拉, 列已配置的所有 BSL；不允许选 source 自己）
  - **复制范围**: ● 按命名空间 (默认, 多选 ns) ○ 整桶 sync (多租户场景)
  - **引擎**: ● rclone (默认, 跨云通用) ○ S3 CRR / Azure OR / GCS Replication (仅同云可选, UI 自动判断)
  - 触发方式（radio: cron / 备份成功后 / 手动）
  - 速率限制（slider, 10-1000 MBps, 云原生引擎不适用 → disable）
  - 已配置的复制规则列表 + 最近一次状态 + 数据量
  - ⚠ 提示条: "Layer 4 仅复制驻留在 BSL 的备份数据。快照型备份 (snapshotMoveData=false) 将被 Preflight 拒绝, 应配 snapshotMoveData=true 或用云端快照复制。"
- **复制状态进 PRD-006 Activity Timeline**: 新 ActionType=`BackupCopy`, 阶段 = `Preflight → Discovery → Transferring → Verifying → Completed`；详情显示 source / target / 仓库路径 / 对象数 / 字节数 / 吞吐 / 耗时

**Phase 0 必做 E2E**（与 §9 Phase 0 联动）:
1. **可恢复性 E2E**: 跑一个真有数据的 ns (e.g. 5GB MySQL, 含真实表数据) → 用 fs-backup 备到 BSL A → rclone sync `kopia/<ns>/` + `backups/<name>/` 到 BSL B → 新集群 attach BSL B → 完整 Restore 该 ns → MySQL 真起来 + 数据校验 (`SELECT COUNT(*)` 与源端一致)。**不只比对 sha256, 必须真的 restore + 业务校验**。
2. **data-mover 同上**: 跑一个 data-mover backup, 重复上述 sync → restore → 业务校验流程。
3. **快照型排除验证**: 跑一个 `snapshotMoveData=false` 的 CSI 快照型备份 → Layer 4 配置 → Preflight 应拦截, 返回错误码 `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`。

##### 📌 P1 假设 fixture 实测背书 (2026-05-31)

§4.3 上述 "仓库级 sync + 快照型排除" 设计**不是空想**, 在评审第三轮 (PRD-Review-2026-05-31-PRD007.md) P1 提出后, 用 `hack/capture-velero-fixture.sh` 抓 docker-desktop 真集群 (3 BSL / 21 backup / 18 DataUpload) + 起 mc pod 直接 list MinIO bucket 实测两条假设, **fixture 双重证实**:

| P1 子假设 | fixture 证据 | 结论 |
|---|---|---|
| **(1)** data-mover 卷数据按 ns 在共享 Kopia 仓库 (`bucket/kopia/<ns>/`) | `mc ls minio/velero/kopia/` 显示**只有 1 个 `test-app/` 目录**, 而 16 个 data-mover 备份全部 `test-app` ns → 共享同一仓库 (不按备份隔离) | ✅ 直接证实 |
| **(2)** `snapshotMoveData=false` CSI snapshot 备份卷数据不在 BSL | xref 表 5 个 `snapshotMoveData=false, snapshotVolumes=true` 备份: 0 DataUpload, 0 VSC (Velero cleanup 后), status=Completed → 数据在云厂商区域快照, BSL 没有 (跟 ADR-031 §1 实测一致) | ✅ 强力证实 |

**完整 fixture + 证据**: `engineer-testing/fixtures/velero-real-2026-05-31-060756/README.md`

**Layer 4 复制语义锁档**:
- ❌ 按 backup 挑对象复制 (v1 草稿写法) → 缺 `kopia/<ns>/` 卷数据 → 静默数据丢失
- ✅ 整 ns sync (`kopia/<ns>/` + `backups/<name>/`) → 可恢复完整数据
- ⚠ `snapshotMoveData=false` 备份 BSL 找不到卷数据 → 必须 Preflight 拦截 (`ERR_LAYER4_SNAPSHOT_UNSUPPORTED`)

**Phase 1 实施解锁** (rclone sync POC + Preflight + 错误码, 关联 task #111)。

#### 4.4 Export / Import 配对模型（#88, 跨集群 RP 流转）

**威胁模型（明确范围, 必读）**:

> SHA256 fingerprint **检测意外损坏** (位翻转 / 存储 corruption / rclone 传输错误), **不防恶意篡改**——能改 BSL 上 tarball 的攻击者通常也能改 fingerprint JSON (重算 SHA256 + 覆盖即可绕过)。防恶意篡改需要二者之一:
> (a) **Object Lock (WORM)** 让 BSL 对象不可改写 (推荐, 见 §4.2);
> (b) **外部持有密钥的 HMAC 签名**: 密钥存在 target 集群 K8s Secret (由 admin 经 Helm `--set fingerprint.sharedSecret=<base64>` 下发), **不入 BSL**, 攻击者拿不到密钥就重算不出合法 signature。
>
> 因此本 PRD 把 fingerprint 的两项能力明确分层: SHA256 = 完整性, HMAC signature = 防恶意篡改, 不能混淆。

**场景**：客户在集群 A 备份 → BSL X；集群 B 也连接同一 BSL X → 集群 B 应**自动 Import** 这些 RP, **但需要 fingerprint 校验**确认 source cluster identity + integrity。

**Source 集群行为**：
- 备份完成后写一份 **fingerprint 文件** 到 BSL 同 prefix 下：`<bsl-prefix>/backups/<backup-name>/.supkube-fingerprint.json`
  ```json
  {
    "source_cluster_id": "uuid-from-kube-system-ns",
    "source_cluster_name": "prod-cn-east-1",
    "kube_version": "v1.28.5",
    "supkube_version": "v0.10.0",
    "backup_name": "daily-orders-20260531-0300",
    "rp_sha256": "ab12...",
    "created_at": "2026-05-31T03:00:00Z",
    "signature": "<HMAC-SHA256(shared_secret, canonical_json_of_above_fields)>"
  }
  ```

**签名要求（v1.1 强化）**:
- **单集群场景** (source = target, 同集群备份恢复): signature **optional**, hash 校验足够
- **跨集群场景** (target ≠ source, 一个集群导入另一集群备份): signature **必需** (hard required)——若 source 未签名 / target 缺密钥, target 拒绝 Import 并提示 admin 配置 `fingerprint.sharedSecret`
- **密钥管理**: HMAC shared secret 由 admin 通过 Helm `--set fingerprint.sharedSecret=<base64-32B>` 在每个参与跨集群信任的集群配置, 存为 K8s Secret `supkube-fingerprint-secret`, **不入 BSL**, **不写日志**
- **算法**: HMAC-SHA256 (与 fingerprint 的 SHA256 完整性算法对齐, 复用 crypto/hmac 标准库; BLAKE3-MAC 待后续评估)

**Target 集群行为**：
- BSL 同步周期（默认 60s）读 fingerprint, RP 列表行加 **`Imported` chip** + hover 显示 source cluster name + 创建时间 + signature 状态
- **失败模式**: fingerprint 不匹配 / 缺失 / hash 对不上 source recorded / 跨集群但签名缺失 → UI 标 "⚠ 完整性校验失败"，**默认不允许 Restore**（hard fail, 见 Q7）
- Target 集群 admin 可在 Settings 显式信任某个 source cluster_id（白名单, 写审计）→ 跳过 cluster_id 校验但**仍校验 hash + signature** (信任只豁免身份不豁免完整性)

**cluster_id 生命周期假设**:
- cluster_id 取自 `kube-system` ns 的 UID (K8s 集群级稳定标识)
- **该 UID 在 ns 被重建 / 集群迁移时会变** (e.g. etcd restore 到新集群, 用户手动重建 kube-system ns 等极端场景)——属边界情况
- TrustStore 在每条信任记录附 `bound_at` 时间戳 + `original_cluster_name` 便于排查 "为何之前信任的集群突然 mismatch"
- 文档 (USER_MANUAL §跨集群信任) 明示该假设

**后端 module**：`internal/fingerprint/`
- `Writer`: 备份完成 hook 调用, 写入 BSL (含 HMAC 签名, 若本集群配置了 sharedSecret)
- `Validator`: BSL 同步时校验, 输出 `FingerprintStatus = ok | mismatch | missing | tampered | signature_required | signature_invalid`
- `TrustStore`: 已信任 source cluster_id 列表（K8s ConfigMap, 每条带 `bound_at`）
- `SecretLoader`: 读 K8s Secret `supkube-fingerprint-secret` 拿 sharedSecret, 注入 Writer/Validator

#### 4.5 长期归档 / Lifecycle Policy

**目标**：客户能配置 "热 30 天（Standard）→ 温 90 天（Infrequent Access）→ 冷 365 天（Glacier）→ 删除 7 年后" 这种生命周期，**SupKube 不自己做归档逻辑**, 而是把规则**写入 BSL 的 lifecycle policy**（S3 Lifecycle / Azure Blob Lifecycle Management / GCS Object Lifecycle Management）, 让云端原生处理（成本最优 + 不占用 SupKube 资源）。

**【v1.1 关键修订】数据 vs 元数据分推荐 (P2 风险隔离)**:

> Glacier / Archive 层对象**不能直接读**, Velero / Kopia 恢复前需先 **thaw 数小时**。把活跃恢复数据冷归档 = RTO 从分钟级拉到小时级, 客户多半不知情。本 PRD 因此把推荐模板拆成两套, 默认对数据与元数据采用不同策略:

| 对象类别 | 默认策略 | 理由 |
|---|---|---|
| **K8s 资源 tarball + Velero backup CRD** (元数据) | 30d Standard → 90d IA → 365d Glacier 可接受 | 元数据小, 灾难场景 thaw 几小时可接受 |
| **Kopia 仓库 / 卷数据** (`bucket/kopia/<ns>/`) | **30d Standard → 90d IA → 之后保留 IA, 默认不转 Glacier** | 保证分钟级 RTO, 数据是恢复路径的关键 |

客户**可以**手工开启 "数据也转 Glacier", 但 UI 必须显示 **大红字 ⚠ 警告**:
> ⚠ 启用后, 该 BSL 上卷数据进入 Glacier 后**恢复时需 thaw 数小时**, RTO 从分钟级拉到小时级。**仅适合长期合规归档, 不适合可恢复场景**。如需保留低 RTO, 请用 Object Lock + IA 组合, 或在另一个独立 BSL (Layer 4 copy target) 上启用 Glacier。

**UI**：Policy 编辑加 "归档生命周期" tab：
- 默认推荐模板（dropdown）:
  - **金融 / 等保三级 (数据热保留)**: 元数据 30d Std → 90d IA → 7y Glacier → delete; 卷数据 30d Std → IA 保留 (默认不转 Glacier)
  - **金融 / 等保三级 (完全冷归档, 仅长期合规)**: 全部 30d Std → 90d IA → 7y Glacier → delete (⚠ 不可直接恢复, 需 thaw)
  - **互联网 / 常规**: 30d Standard → 365d IA → delete
  - **极简**: 90d Standard → delete
  - **自定义**: 手动填每段天数 + storage class + 对象类别 (数据 / 元数据 / 全部)
- 预览生成的 lifecycle JSON / XML（按 BSL 类型展示）
- **预览页 (Info #4)**: 按 BSL 类型 (S3 XML / Azure JSON / GCS JSON) 展示 merge 后**完整规则**; 同 prefix 多 transition 冲突 (e.g. 30d→IA 和 60d→IA 同时存在) → **直接报错而非静默追加**, 客户须显式合并或选 "覆盖现有"
- 应用按钮 → 后端通过云 SDK PUT lifecycle 到目标 bucket

**Preflight 校验 (P2 必做, apply 前后端拦截)**:
- 校验 1: 对启用 Object Lock 的 BSL, **lifecycle delete 时间 ≥ 任何 Object Lock 保留期** (含已有 RP 上的 retention)。不满足 → 拒绝 apply, 错误码 `ERR_LIFECYCLE_LOCK_CONFLICT`, 提示 "lifecycle delete=Nd 早于 Object Lock retention=Md, 删除会被 WORM 拒绝, 规则将静默失效"
- 校验 2: 推荐模板中如包含 "数据转 Glacier" → 强制二次确认 dialog (含 RTO 警告原文 + admin 显式勾选 "我已知晓 RTO 影响")
- 校验 3: 同 prefix 多 transition 检测 (见 UI 预览段)

**风险提示**: 若 BSL 已有外部 lifecycle 规则, SupKube **不覆盖**, 只追加 + 提示 "检测到现有 N 条规则, 本次将追加 M 条"; 但**遇冲突先报错**, 不沉默。

#### 4.6 0 错误验证（Verified Restore + Hash check）

**自动 CRC 校验**：
- backup 完成后, 后端 worker 读 BSL 上的 tarball, 算 **SHA256**, 与 source recorded hash（Velero PodVolumeBackup 内已有 / 或 SupKube 在 backup hook 时记录）对比
- 不匹配 → 标 backup 为 `IntegrityFailed`, **UI 上不显示绿色 ✅**, 而是显示 `✗ 完整性失败 (expected sha256: ab12..., got: cd34...)`
- v1 范围: **仅本地校验**（读完整 tarball 算 hash）；v2 加 Kopia repo check（chunk 级）

**每个 backup 详情**（PRD-006 Activity Timeline 内）显示：
- 完整性: ✅ / ✗
- SHA256: `ab12...cd34`（可复制）
- 校验耗时（影响估算）

**校验本身的失败模式**: 网络中断 / BSL 不可达 → 标 `IntegrityCheckPending`, 重试 3 次后人工介入。

#### 4.7 DR Drill（Layer 5 虚拟实验室）

> ⚠ **安全警告 (v1.1 必读)**: DR Drill 仅对**自包含工作负载**安全 (e.g. stateless 计算 + 自带 DB)。**含外部生产依赖的应用** (连生产 DB / 消息队列 / 支付网关 / 第三方 API / 服务注册中心) 必须**先做网络隔离 + 配置覆盖**, 否则一次 "演练" 可能往生产库写脏数据 / 发真邮件 / 触发真支付。本 PRD v1.1 已把 default-deny egress NetworkPolicy 提升为**必做项** (原 v1 草稿是 Q8 待评审, 现已落定)。

**模型**：
1. 客户在 Restore Points 页选一个 RP → 点 **"DR Drill"** 按钮
2. **DR Drill Wizard 弹窗**: 必勾 confirmation checkbox:
   > ☐ 我已确认该应用**不依赖外部生产资源** (生产 DB / 支付 / 邮件 / 第三方 API), 或我已配置端点重写 (Transform `redirect-external-endpoints-to-sandbox`); 否则 drill 可能影响生产。
   - 未勾 → 创建按钮 disable
   - 勾选并继续 → 写审计 (actor / RP / `external-dep-confirmed`)
3. SupKube 创建沙箱 ns: 命名规则 `dr-drill-<rp-short>-<unix-ts>`（避免与生产撞名）；ns 打 label `supkube.io/dr-drill=true` + `supkube.io/auto-cleanup-at=<ts+1h>`
4. **自动下发 default-deny egress NetworkPolicy (v1 必做项, P4)**:
   - 默认 egress 全拒, 仅放行: (a) 沙箱内 Pod→Pod 通信; (b) kube-dns; (c) 客户在 Settings 显式配置的**白名单端点** (e.g. 内部 Maven 镜像, 沙箱内 mock 服务)
   - 任何到集群外的 egress (包括生产 DB / 支付网关 / 公网 API) **默认被拒**
   - NetworkPolicy YAML 模板内置, 客户可在 Settings 编辑白名单
5. SupKube 调 Restore 流程, 把 RP 还原到该 ns。**自动 Transform 链** (复用 PRD-002):
   - 原有: NodePort → ClusterIP; Ingress host → `<ns>.drill.local`
   - **新增 (v1.1, P4)**: `redirect-external-endpoints-to-sandbox` Transform——扫描 Deployment/StatefulSet 的 env 与 ConfigMap, 把已知外部端点 (`DB_HOST` / `API_URL` / `PAYMENT_GATEWAY_URL` / `SMTP_HOST` / `KAFKA_BROKERS` 等约定字段) 重写为沙箱内 mock 服务地址 (e.g. `mock-mysql.<ns>.svc:3306`)。该 Transform 须加入 PRD-002 §4.1 的 11 个 builtin Transform 列表 (PRD-002 v1.1 已同步)。
6. 跑客户配置的 **smoke test script**（k8s Job, image 由客户配, 默认提供 `httpbin / mysql-check / redis-ping` 三个模板）
7. 结果展示在 DR Drill 详情页：
   - ✅ Drill 成功（all smoke pass, RTO = 4m32s, RPO = 2h）
   - ✗ Drill 失败（哪个 smoke fail + 错误日志 + AI 建议入口, 接 PRD-003 §4.7; **AI 调用走 SECURITY.md §6 + PRD-003 §7.2 统一脱敏管线**, Med #2, 不另做）

**自动清理**：
- Cleanup controller 每 5min 扫一次, 删除 `auto-cleanup-at` 已过 ns（GC 全部对象 + PV + NetworkPolicy）
- **资源配额限制**: 沙箱 ns 默认 ResourceQuota（CPU 2 / Memory 4Gi / PVC 5）防止 OOM 影响生产

**月度自动 drill**:
- 客户可在 Policy 标记 RP "critical"
- 每月 1 日 02:00 controller 对所有 critical RP 自动跑一次 drill, 结果进 Compliance 报告
- 失败 → 邮件 / Webhook 通知 + PRD-003 AI Advisor 给改进建议 (AI 走统一脱敏管线, Med #2)

**v1 简化范围**:
- 沙箱仅"单 ns 隔离 + default-deny egress NetworkPolicy + 端点重写 Transform" (Q8 v1.1 已收口)
- smoke test 仅支持 Job 模板, 不支持复杂 Pipeline
- 单独 cluster 强隔离方案 (vcluster / kind in-cluster) v1.x 评估

#### 4.8 Resilience Posture 仪表板（共享数据采集层, 分数定义独立 - P5 v1.1 收口）

> **【v1.1 关键修订, P5】** v1 草稿写 "PRD-003 与 PRD-007 Score 数值 100% 一致 / 共享同一引擎" 是范畴错误——二者度量对象不同 (单应用 vs 集群层覆盖)。本 PRD v1.1 按评审 P5 决策**二选一选 (a)**: **定义为两个不同指标, 共享底层数据采集层, 但分数算法独立**。

**两个指标的明确定义**:

| 指标 | 所属 PRD | 度量对象 | 维度 | 取值 |
|---|---|---|---|---|
| **Application Resilience Score** (应用韧性分) | PRD-003 §3.3 | 单应用 / 单 ns | 5 业务/架构维度 (Business Value / Architecture / Protection / Security / Operation) | 0-100 |
| **Cluster 3-2-1-1-0 Posture** (集群 Posture 分) | PRD-007 §4.8 | 集群级 | 5 层覆盖度 (L1~L5, 各 20 分) | 0-100 |

**共享与独立的边界**:
- **共享**: 底层数据采集层 `internal/resilience/` (K8s 资源扫描 / Velero CR 扫描 / BSL 状态查询 / Schedule 配置读取), 同一份 in-memory cache, 同一份 K8s API watch
- **独立**: 分数算法——PRD-003 是业务/架构权重的加权计算, PRD-007 是 5 层覆盖度的加和; **二者不可直接相减或对比**, UI 上明示二者关系

**UI 上明示关系 (避免客户混淆)**:
- **Posture 卡 hover** (PRD-007): 显示 "此集群 N 个 ns 应用韧性分布: avg 72, p50 78, p10 45 → 跳 PRD-003 应用列表" (聚合统计, 非分数对比)
- **PRD-003 应用详情页顶部 banner**: 显示 "本集群 Posture: 62/100 (3 层缺失) → 跳 Dashboard"
- **绝不允许**: 把 Posture (集群层覆盖) 与 Resilience Score (单应用韧性) 做加减对比, 或在同一图表上画

**Dashboard 新增卡片**: "数据韧性 Posture"（位置: PRD-003 Resilience Score 卡之下 / 之上, **二者展示独立, 共享底层数据**）

```
┌─ 数据韧性 Posture (Cluster 3-2-1-1-0 Coverage) ────────┐
│                                                          │
│  L1 本地快照     ✅  最近: 2h 前        [详情]           │
│  L2 本地 BSL     ✅  容量: 1.2TB/2TB    [详情]           │
│  L3 云 BSL       ⚠   仅 1/3 Policy 启用 [一键启用 →]    │
│  L4 第 2 云 Copy  ✗   未配置             [一键启用 →]    │
│  L5 DR Drill     ⚠   上次: 45 天前      [立即 drill]    │
│                                                          │
│  Cluster Posture: 62/100 (Fair, 5 层覆盖度)             │
│  应用韧性分布 (PRD-003): avg 72, p50 78, p10 45 →       │
│  AI 建议: 启用 L4 可提升 Posture 至 81/100              │
│  ↑ AI 调用走 SECURITY.md §6 + PRD-003 §7.2 统一脱敏     │
└──────────────────────────────────────────────────────────┘
```

**数据采集层共享细节**:
- 实现文件: `internal/resilience/scan.go` (扫描) + `internal/resilience/store.go` (cache) — 由本 PRD 实现, PRD-003 import
- PRD-003 调用: `resilience.GetClusterState() *ClusterState` 拿原始事实, 然后跑自己的 `pkg/aiadvisor/score.go` 算应用韧性分
- PRD-007 调用: `resilience.GetClusterState() *ClusterState` 拿原始事实, 然后跑 `internal/resilience/posture.go` 算集群 Posture 分
- **避免双采集**: 两个分数走同一 K8s API 调用 + 同一 cache TTL (60s)
- **AI 钩子脱敏 (Med #2)**: §4.7 (DR Drill 失败→AI) + §4.8 (Posture→AI 建议) 的所有 AI 调用统一走 SECURITY.md §6 + PRD-003 §7.2 出境脱敏管线, **不另做**, PRD 内只引用不复述

#### 4.9 Backend 新增 module

| Module | 职责 | 关联 task |
|---|---|---|
| `internal/resilience/` | 计算 5 层状态 + Posture Score；提供 HTTP API 给前端 + Go API 给 PRD-003 | 本 PRD |
| `internal/backupcopy/` | 跨云 object-to-object 复制控制器（rclone wrapper, 速率限制, retry, 断点续传） | #111 |
| `internal/fingerprint/` | Export/Import 校验 (Writer / Validator / TrustStore) | #88 |
| `internal/drdrill/` | Layer 5 沙箱编排（ns 创建 / Restore 触发 / smoke runner / cleanup controller） | 本 PRD |
| `internal/lifecycle/` | BSL Lifecycle Policy 渲染 + 应用（封装 S3/Azure/GCS SDK 差异） | 本 PRD |
| `internal/integrity/` | 0 错误验证 worker (SHA256 校验调度 + 重试) | 本 PRD |

### 5. UI / UX（关键 mockup）

#### 5.1 Dashboard "Resilience Posture" 卡（见 §4.8）

#### 5.2 Policy 编辑 "Layer 4 Backup Copy" tab

```
┌─ Policy: daily-prod-orders ─────────────────────────────┐
│ [Basic][Schedule][BSL][Lifecycle][Layer 4 Backup Copy][Object Lock]│
│                                                          │
│  已配置复制规则:                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Source: minio-local (L2) → Target: aws-us-east (L3)│ │
│  │ Trigger: on-success    Rate: 100 MBps             │ │
│  │ 最近: 2026-05-30 03:42 ✅ 1.2 GB / 18s            │ │
│  │ [编辑] [删除] [立即执行]                          │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│  + 新增复制规则:                                          │
│    Source: [aws-us-east ▼] → Target: [azure-cn ▼]      │
│    Trigger: ● on-success  ○ cron  ○ manual              │
│    Rate Limit: [====|====] 100 MBps                     │
│    [取消] [保存]                                         │
└──────────────────────────────────────────────────────────┘
```

#### 5.3 Policy 编辑 "归档生命周期" tab

```
┌─ Lifecycle Policy ──────────────────────────────────────┐
│  推荐模板: [金融/等保三级 ▼]                              │
│                                                          │
│  Stage 1: [30] 天     Storage: Standard                 │
│  Stage 2: [90] 天     Storage: Infrequent Access        │
│  Stage 3: [2555] 天 (7y) Storage: Glacier               │
│  Stage 4: Delete                                         │
│                                                          │
│  目标 BSL: aws-us-east (S3)                              │
│  ⚠ 检测到 bucket 已有 2 条 lifecycle 规则, 本次追加 4 条│
│                                                          │
│  [预览生成的 XML] [应用]                                 │
└──────────────────────────────────────────────────────────┘
```

#### 5.4 Restore Points 行 + DR Drill

```
RP: daily-prod-orders-20260530-0300
  Size: 12.3 GB | Created: 9h ago | Fingerprint: ✅ (cluster: prod-cn-east-1)
  Integrity: ✅ sha256: ab12...cd34
  [Restore] [DR Drill ▼] [Export]
              └─ DR Drill 历史:
                  2026-05-15 ✅ RTO 4m32s
                  2026-04-15 ✗  smoke fail (mysql)
                  [立即 Drill]
```

#### 5.5 Backup 详情 "完整性 SHA256" 行

（嵌入 PRD-006 ActionDetailDrawer 的 PHASES section 之下）:

```
Integrity Check:  ✅ Verified
  Algorithm:       SHA256
  Expected:        ab12cd34ef56...（点击复制）
  Got:             ab12cd34ef56...
  Verified at:     2026-05-31 03:15:42 (took 12s)
```

### 6. Out of Scope

- ❌ **跨云 source data 重备份**（我们做 object copy, 不重新 backup source；这是 PRD-007 与传统多目的地备份的根本差异）
- ❌ **Block-level / CDC 持续复制**（Trilio Continuous Restore 路线, 已在 ADR-031 后决策不做; 复杂度过高, 客户基数不够）
- ❌ **KubeVirt 虚机的特殊归档**（依赖 #93, 另立 PRD）
- ❌ **Kopia repo deep check**（chunk 级 / dedup 完整性, v2 范围）
- ❌ **沙箱 DR Drill 跨集群隔离**（v1 仅单 ns, 单独 cluster 隔离方案 v1.x 评估, 见 Q8）
- ❌ **Object Lock Compliance vs Governance 模式自动选择**（v1 由客户在 #58 UI 显式选, 不做 AI 推荐）

### 7. 非功能性要求

| 维度 | 指标 |
|---|---|
| **Layer 4 复制性能** | 单 GB ≤ 10s（千兆网络无瓶颈时）; 失败 retry 3 次（指数退避 1s/3s/9s）; 速率限制可配（10-1000 MBps）|
| **Fingerprint overhead** | 写入 BSL <1s（< 1KB JSON）; 校验 <500ms（含 BSL HEAD + JSON parse）|
| **DR Drill 沙箱配额** | 默认 ResourceQuota CPU=2 / Memory=4Gi / PVC=5 / 存储 50Gi; 客户可在 Settings 调高 |
| **Resilience Posture 计算** | <500ms（in-memory cache 60s; 与 PRD-003 共享 cache）|
| **Lifecycle policy 应用** | <3s（云 SDK PUT 单次调用）|
| **完整性校验性能** | 1GB tarball ≤ 30s（SHA256 单线程；并发数限制为 BSL 同时 2 个避免 IO 风暴）|
| **审计** | Object Lock 操作 / Backup Copy 触发 / DR Drill 执行 / Fingerprint trust 变更 全部写审计日志（actor / before / after / result）|
| **安全** | Layer 4 凭据走 K8s Secret + ADR-004 凭据管理；跨云 IAM 最小权限 (read source / write target), **禁止 admin**；Fingerprint signature 用 HMAC-SHA256 + 集群级 shared secret |
| **i18n** | en + zh-CN 全文案；所有数字 / 时间均按 locale 渲染 |
| **可观测性** | 所有新模块输出 Prometheus metrics: `backupcopy_bytes_transferred` / `drill_success_total` / `integrity_failed_total` / `resilience_posture_score` |

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | Dashboard "Resilience Posture" 卡渲染 5 层 chip, 状态可由 kubectl 集群状态推出 (test: 用 fake fixture 注入 L3 缺失 → chip 显示 ✗ + "一键启用") |
| 2 | **(v1.1 改, Med #3)** Layer 4 Backup Copy 可恢复性: 1GB 真有数据的 ns (e.g. MySQL with rows) 从 source BSL 仓库级 sync 到 target BSL → **新集群完整 Restore + 卷数据一致校验** (`SELECT COUNT(*)` 与源端一致), 至少跑 1 个 fs-backup + 1 个 data-mover backup 各一次。**不再仅比对元数据 sha256**——可恢复性是 Layer 4 的唯一意义 |
| 2a | **(v1.1 新增, P1)** 快照型备份 Layer 4 拦截: 对 `snapshotMoveData=false` 的 CSI 快照备份配 Layer 4 → Preflight 返回错误码 `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`, UI 显示拦截原因 + 修复指引 (配 `snapshotMoveData=true` 或改用云端快照复制) |
| 3 | Layer 4 复制失败 retry: 中断 target BSL 网络, 第一次失败, 1s 后 retry, 3s 后再 retry; 第 3 次成功 → 状态 ✅ + retry_count=2 |
| 4 | Layer 4 速率限制: 设置 50 MBps, 1GB 文件复制实际耗时 ≥ 20s (容忍 ±10%) |
| 5 | Fingerprint: 集群 A 备份 → BSL X 同 prefix 出现 `.supkube-fingerprint.json`; 集群 B 60s 内同步, RP 行显示 `Imported` chip + hover 含 `prod-cn-east-1` |
| 6 | Fingerprint 篡改: 用 mc / aws cli 手动改 BSL 上 tarball 1 字节 → 集群 B 下次同步标 "⚠ 完整性校验失败", Restore 按钮 disabled |
| 7 | Fingerprint 信任: target 集群 admin 把 source cluster_id 加入 TrustStore → UI 不再显示 signature warning（但 hash 仍校验）+ 操作进审计日志 |
| 8 | DR Drill 创建: 选 RP → 点 "DR Drill" → 沙箱 ns `dr-drill-<rp>-<ts>` 出现, label `auto-cleanup-at` 正确; Restore 流程触发 |
| 9 | DR Drill smoke test: 内置 `httpbin` 模板 Job 跑通, 详情页显示 ✅ + RTO 数值 |
| 10 | DR Drill 自动清理: 沙箱 ns 创建 1 小时后, cleanup controller 真正删除 ns + PV (test: 把 cleanup 间隔调成 1min 加速验证) |
| 11 | DR Drill 资源配额: 沙箱 ns ResourceQuota 生效, 超额 Pod 创建被拒（test: 配 cpu=2, 提交 cpu=4 的 Job → ResourceQuota 报错）|
| 12 | 月度自动 drill: 标记 RP critical, cron 触发后 1 小时内出现 drill 执行记录; 失败时邮件通知 webhook 被调用 |
| 13 | 完整性校验: 备份完成后 worker 自动算 SHA256; 主动篡改 BSL tarball 1 字节 → 校验报 `IntegrityFailed`, UI 显示 ✗ + expected vs got hash |
| 14 | Lifecycle: 选 "金融/等保三级" 模板 → 应用到 S3 bucket → AWS console 显示 4 条规则正确 (30d→IA, 90d→Glacier, 2555d→delete); 已有外部规则保留 |
| 15 | Object Lock 冲突预检: 选 GCS BSL 时, UI 警告 "GCS 无原生 Object Lock, 将使用 retention policy 替代" + 链接到云文档 |
| 16 | Object Lock 审计: 客户尝试删除 WORM 期内的 RP → 操作被拒 + 审计日志 1 条 (actor / target_rp / "delete attempted, denied: object locked until 2026-12-31") |
| 17 | **(v1.1 改, P5)** Posture Dashboard 卡数据来源跟 PRD-003 应用韧性 API 共享底层数据采集层 (`internal/resilience/scan.go` + `store.go`), 但二者展示分数定义不同 (Posture = 集群层覆盖, Resilience Score = 单应用韧性), UI 明示二者关系 (Posture 卡 hover 显示应用韧性分布聚合 / PRD-003 应用页 banner 显示集群 Posture), **不允许直接相减或对比**。test: 调 `/api/v1/resilience/cluster-state` 一次, 两个分数算子并行算, 验证共享数据但分数公式独立 |
| 17a | **(v1.1 新增, P3)** Fingerprint 跨集群签名: target 集群无 `supkube-fingerprint-secret` Secret 时, 导入跨集群备份显示 `signature_required` 状态, Restore 按钮 disabled; admin 经 Helm `--set fingerprint.sharedSecret=...` 配密钥后, 同一备份签名校验通过, 允许 Restore |
| 17b | **(v1.1 新增, P4)** DR Drill default-deny egress: 沙箱 ns 创建后立即出现 default-deny egress NetworkPolicy, 还原后 Pod ping 集群外端点 (e.g. 8.8.8.8 / 生产 DB IP) 应被拒; 白名单端点 + sandbox 内 mock 服务可通 |
| 17c | **(v1.1 新增, P4)** DR Drill Wizard confirmation: 未勾 "外部依赖确认" checkbox → DR Drill 创建按钮 disabled; 勾选并提交后, 审计日志含 `actor / RP / external-dep-confirmed=true` 一条 |
| 17d | **(v1.1 新增, P2)** Lifecycle 冲突预检: 对启用 Object Lock 7y 的 BSL apply lifecycle 含 "3y delete" → 拒绝 apply, 错误码 `ERR_LIFECYCLE_LOCK_CONFLICT`; 同 prefix 多 transition (e.g. 30d→IA + 60d→IA) → apply 报错而非静默追加 |
| 17e | **(v1.1 新增, P2)** Glacier 数据冷归档警告: 选含 "数据转 Glacier" 的模板 → UI 弹窗显示 RTO 警告原文 (含 "数小时 thaw"), 需 admin 显式勾选 "我已知晓 RTO 影响" 才能 apply |
| 17f | **(v1.1 新增, Med #1)** Phase 0 Kopia 维护 vs Object Lock 实测报告归档: Phase 0 在 Object-Locked BSL 上跑 Kopia 维护至少 1 次, 行为 (是否报错 / 仓库膨胀情况 / immutable-storage 模式是否可用) 写入 ADR-031 §X 补遗, copy-target BSL 默认配置据此调整 |
| 18 | Backup Copy 进 Activity Timeline: PRD-006 Activity 列表出现 `BackupCopy` ActionType, 详情显示 source / target / 字节数 / 吞吐 |
| 19 | i18n: 所有新文案 en + zh-CN 双语完整, 无 i18n key fallback 警告 |
| 20 | 前端 `npm run build` 通过；新增 module 单元测试覆盖率 ≥ 70%（`internal/backupcopy` / `internal/fingerprint` / `internal/drdrill` / `internal/resilience` / `internal/lifecycle` / `internal/integrity`）|

### 9. 任务拆分（6 Phase）

| Phase | 内容 | 工期 |
|---|---|---|
| **Phase 0 — verify-before-architect** | 5 层各自最小 fixture 入库 + 实测 ADR-031 假设：rclone 跨云吞吐 / Object Lock 行为差异 / fingerprint overhead / sandbox ns 清理可靠性。**【v1.1 必加 E2E】**: (1) **P1 Layer 4 可恢复性 E2E**——5GB MySQL ns fs-backup 备到 BSL A → rclone sync `kopia/<ns>` + `backups/<name>` 到 BSL B → 新集群 attach BSL B 完整 Restore → MySQL 起来 + SELECT COUNT 一致; 同流程跑一次 data-mover; (2) **P1 快照型排除**: CSI 快照型 backup 配 Layer 4 → Preflight 拦截 `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`; (3) **Med #1 Kopia 维护 vs Object Lock**: Object-Locked BSL 上跑 Kopia 维护至少 1 次, 记录是否报错/仓库膨胀/immutable-storage 模式可用性, 结论写 ADR-031 补遗; (4) **P2 Glacier RTO 验证**: 跑一次 Glacier 转换 + thaw + restore, 实测耗时, 把数值写入 UI 警告文案。**回退**: 若 rclone 跨 GCS↔Azure 性能不达标 (<10MB/s), 本 PRD 范围回退 v1 仅 S3↔S3, 其他云移 v1.x | 1.5 周 |
| **Phase 1 — Resilience Posture 引擎 + Dashboard 卡** | `internal/resilience/` + Dashboard 卡 + 与 PRD-003 共享接口约定 | 1 周 |
| **Phase 2 — Layer 4 Backup Copy (#111)** | `internal/backupcopy/` rclone wrapper + 控制器 + Policy "Layer 4" tab + Activity Timeline 集成 | 2 周 |
| **Phase 3 — Fingerprint + Export/Import (#88)** | `internal/fingerprint/` Writer + Validator + TrustStore + UI Imported chip + Settings 信任管理 | 1.5 周 |
| **Phase 4 — Preflight Backup Readiness (#109)** | 复用 PRD-001 preflight 范式但 backup 侧（BSL 可达 / Object Lock 启用 / 容量 / 凭据有效期 预检）；新增 `PreflightBackup` 端点 + Policy Wizard 入口 | 1 周 |
| **Phase 5 — Layer 5 DR Drill** | `internal/drdrill/` 沙箱 ns 编排 + smoke test runner + cleanup controller + 月度 drill cron + UI | 2 周 |
| **Phase 6 — 长期归档 Lifecycle Policy** | `internal/lifecycle/` 云 SDK 封装 + Policy "归档生命周期" tab + 推荐模板 | 0.5 周 |

**总计约 9 周**（一人全职; 两人并行 Phase 2/3 + Phase 5/6 → 约 5-6 周）。

> Phase 6 可与 Phase 1 并行（无依赖）；Phase 5 依赖 Phase 0 沙箱清理验证；Phase 2 依赖 Phase 0 rclone 实测。

### 10. 关联文档与任务

- **ADR**: ADR-031（5 层韧性 / 本 PRD 的源头）+ ADR-029（本地快照战略）+ ADR-025（双 schedule pair, L2 模型）+ ADR-026（Velero v1.18 限制）+ ADR-033（拟, AI Advisor 评分引擎复用本 PRD `internal/resilience/`）
- **PRD**: PRD-001（Restore Preflight, restore 端口对应; 本 PRD 是 backup 端）/ PRD-002（Transform; DR Drill 还原时复用）/ PRD-003（AI Advisor; Resilience Score 共享引擎）/ PRD-006（Activity Timeline; BackupCopy / DRDrill / IntegrityCheck 新 ActionType）
- **Task（已完成, 基础）**: #56 LBS1（本地 MinIO BSL Helm 集成）/ #57 LBS2（Policy UI Local/Cloud BSL 选择器）/ #58 LBS3（Object Lock UI + 不可变保留期可视化）/ #59 LBS4（3-2-1-1-0 健康度评分卡）
- **Task（本 PRD 范围）**: **#88** v0.9.1.6-EXPORT-IMPORT（Kasten 风格 Export/Import 配对模型, RP 跨集群指纹流转）/ **#109** v0.9.x-PREFLIGHT-BACKUP-READINESS（集群备份就绪 pre-flight check）/ **#111** v0.9.x-LAYER4-BACKUP-COPY（第 2 云 object-to-object 复制, 非重发）/ **#126** v0.10.x-RESILIENCE-FULL-3-2-1-1-0（本 PRD 直接任务编号, 串联以上）
- **文档**: SECURITY.md §4（Object Lock 已实施基线）/ §5（3-2-1-1-0 已实施 + 演进; 0 错误验证 v0.8.15 / DR Drill v0.9.7）/ USER_MANUAL.md §RBAC（Lifecycle 操作权限）/ ROADMAP.md "数据韧性权威模型"（本 PRD 是该模型的 PRD 化落地）

### 11. 开放问题（评审时讨论）

| Q | 问题 | 倾向 / 解决状态 |
|---|---|---|
| Q1 | Layer 4 Backup Copy 速率限制：固定 default 100 MBps vs 自适应（探测网络后动态调）？ | **v1 固定 + UI 可配**（自适应需在线探测复杂；客户做容量规划也偏好固定值）; 自适应 v1.x 评估 |
| Q2 | Fingerprint 算法选 SHA256 vs BLAKE3？ | **✅ 评审已答 (PRD-Review-2026-05-31-PRD007 §P3)**: SHA256 用于完整性 (检测意外损坏), 签名用 **HMAC-SHA256** (与 SHA256 算法对齐); BLAKE3-MAC 性能优但客户审计接受度待评估, 留 v1.x |
| Q3 | DR Drill 沙箱 ns 命名 + 配额政策？ | 命名 `dr-drill-<rp-short>-<unix-ts>`; 默认配额 CPU=2/Memory=4Gi/PVC=5/Storage=50Gi; **客户可在 Settings 调高**, 但 hard cap CPU=8/Memory=16Gi 防失控 |
| Q4 | 月度自动 drill 谁触发：controller cron vs 客户手动配置 cron？ | **两者都支持**：默认 controller 每月 1 日 02:00 (UTC) 跑标记 critical 的 RP; 客户可在 Policy 改 cron 表达式覆盖 |
| Q5 | Lifecycle 默认推荐 30/90/365 是否合理？ | **✅ 评审已答 (PRD-Review-2026-05-31-PRD007 §P2)**: 推荐合理但**必须分数据/元数据**——元数据可冷归档, 卷数据默认不转 Glacier (保留 IA 保证分钟级 RTO); 客户手工开 "数据也转 Glacier" 须二次确认 + ⚠ 大红字 RTO 警告。详见 §4.5 v1.1 |
| Q6 | 跨云 IAM 最小权限范围 | **read source bucket + list + get-object; write target bucket + put-object + put-object-tagging; 绝不 admin / delete**。文档提供 AWS / Azure / GCP 三套 IAM policy 模板, 客户复制粘贴 |
| Q7 | Fingerprint 不匹配是 hard fail 还是 customer override？ | **默认 hard fail**（安全保守）; admin 可在 Settings 显式 override 单次 Restore（写审计 + 二次确认 dialog + 30 天后自动失效）—— 不开"永久信任 mismatch"逃生门 |
| Q8 | Layer 5 sandbox 隔离：单 ns vs 单 cluster？ | **✅ 评审已答 (PRD-Review-2026-05-31-PRD007 §P4)**: v1 单 ns 但 **default-deny egress NetworkPolicy + 端点重写 Transform 提升为 v1 必做项** (不再是 "提一句"); 单 cluster (vcluster) v1.x 评估。详见 §4.7 v1.1 |
| Q9 | 0 错误验证：仅本地 SHA256 vs 远端 BSL re-fetch？ | **v1 本地 SHA256**（备份完读 BSL 校验, 1GB ≤ 30s）; **v2 加 Kopia chunk-level dedup check**（更深, 但需依赖 Kopia 路线明朗）|
| Q10 | Resilience Posture: 计算引擎与 PRD-003 AI Advisor §3.3 完全共用？ | **✅ 评审已答 (PRD-Review-2026-05-31-PRD007 §P5)**: 共用是范畴错误。改为**两个独立指标共享底层数据采集层** (PRD-003 = 应用韧性分单 ns; PRD-007 = 集群 Posture 5 层覆盖); 分数算法独立, UI 不允许直接相减或对比。详见 §4.8 v1.1 |
| Q11 | Backup Copy 触发 `on-success` 是 sync 还是 async？ | **async**（不阻塞主备份完成上报）; 但 UI Activity Timeline 显示 BackupCopy 阶段, 客户看得到链路 |
| Q12 | 客户在不支持 Object Lock 的 BSL 上启用 3-2-1-1-0 完整保护勾选, UI 怎么处理？ | **UI 强制提示 + 不允许保存**, 必须先换 BSL 或显式接受降级（写审计 "客户接受非 immutable 保护"）|

### 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Claude | — → **草稿** | 起草。Mars 主动询问 "从快照到长期云端保留 with immutability, 3-2-1-1-0 的功能是否已经写了 PRD？" 后立项, 整合 ADR-031（5 层原则）+ #88（Export/Import）+ #109（Preflight）+ #111（Layer 4 Backup Copy）+ LBS 系列 #56-59（已完成基础）。**关键决策**: (1) Resilience Score 引擎强制与 PRD-003 共享 (DoD #17); (2) Phase 0 verify-before-architect 实测 rclone 跨云吞吐, 不达标可回退 v1 仅 S3↔S3; (3) Fingerprint 默认 hard fail, 不开 mismatch 永久信任逃生门 (Q7); (4) DR Drill v1 单 ns 隔离 (Q8); (5) Lifecycle 不做云端归档逻辑, 仅写 BSL 原生 lifecycle policy (§4.5)。Q1-Q12 待 Mars 评审拍板。待 Mars 评审决定是否 → 排队评审。|
| 2026-05-31 | Mars (Claude 委托评审人) | 草稿 → **改正中 (v1.1)** | 评审报告 PRD-Review-2026-05-31-PRD007.md 出 P1-P5 共 5 个 High + 5 个 Med/Info。**关键修订**: (1) **P1 Layer 4 重写 §4.3**——复制粒度改为 `kopia/<ns>` 仓库级 + `backups/<name>/` 元数据 (rclone sync, 不是 per-backup 挑对象); 钉死适用边界 "仅 BSL-resident 数据, 快照型备份排除 (`ERR_LAYER4_SNAPSHOT_UNSUPPORTED`)"; v1.1 推云原生复制 (S3 CRR / Azure OR / GCS Replication) 同云优先; Phase 0 必跑 5GB MySQL 真恢复 + 数据校验 E2E。(2) **P2 §4.5 Glacier 分数据/元数据**——卷数据默认不转 Glacier (保留 IA 保分钟级 RTO), 元数据可冷归档; Lifecycle apply 前 Preflight 校验 "delete 时间 ≥ Object Lock 保留期" + 同 prefix 冲突报错。(3) **P3 §4.4 Fingerprint 威胁模型重述**——SHA256 = 完整性 (防意外损坏), HMAC-SHA256 签名 = 防恶意篡改 (密钥经 Helm 下发存 K8s Secret, 不入 BSL); 跨集群签名从 optional 改为**必需**。(4) **P4 §4.7 DR Drill default-deny egress NetworkPolicy 提为 v1 必做项**; 新增 `redirect-external-endpoints-to-sandbox` Transform (复用 PRD-002); Drill Wizard 加必勾 confirmation。(5) **P5 §4.8 Score 改为两个独立指标**——PRD-003 = Application Resilience Score (单应用 5 维度), PRD-007 = Cluster 3-2-1-1-0 Posture (集群 5 层覆盖); 共享底层数据采集层 (`internal/resilience/scan.go`) 但分数算法独立, UI 不允许直接对比; DoD #17 改写。Med #1 Kopia 维护 vs Object Lock Phase 0 实测纳入 DoD; Med #2 AI 钩子复用 SECURITY §6 + PRD-003 §7.2 脱敏管线 (不另做); Med #3 DoD #2 改为 "Restore + 卷数据一致" (不再仅 sha256); Info #4 Lifecycle 同 prefix 冲突报错; Info #5 cluster_id 生命周期文档注明 + TrustStore 记 bound_at。Q2/Q5/Q8/Q10 标 ✅ 评审已答。|
| 2026-05-31 | Mars | 改正中 → **✅ 已评审** | 通过, 可进研发 (P1 fixture 已证实 + 5 P 全闭环). |

---

<a id="prd-008"></a>
## PRD-008 — RP 删除生命周期 + Activity 持久化 + Force Delete 副作用治理

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-008 |
| **任务编号** | #148 (placeholder, 待建) |
| **状态** | **研发中**（2026-06-03 Mars 评审通过, 直接进研发；D1-D5 闭环 + 正文回填见 §13 / M-1·M-2；存储选型嵌入式 store on PV 待 Phase 0 实测坐实）|
| **作者** | Claude / Mars |
| **目标版本** | v0.9.x |
| **关联 ADR** | ADR-031（5 层韧性）/ ADR-035（结构化日志）/ **拟 ADR-039（Activity 持久化与 audit event 存储选型）** ← 让号 2026-06-01：ADR-037 已分配给 PRD-011/012 统一数据采集架构（台账自洽，PRD-Review 第六份要求让号） |
| **关联 PRD** | **PRD-006（Activity Task Detail Timeline）—— 本 PRD 是 PRD-006 的数据层基石** / PRD-005（Log Viewer v2）/ PRD-007（5 层韧性, ForceDelete 同源 + D5 孤儿清理 Kopia 同源 P1）|
| **PRD-Review 状态** | 2026-05-31 评审 → 5 finding 已记 → 2026-06-02 finding 修订收口（§13） |

### 1. Goal（目标）

把 RP 删除从当前"fire-and-forget DELETE API 调用"升级为**完整的生命周期 Task**：删除作为 Activity 一等事件被记录、删除中状态在 UI 上**锁定不可操作**防止误点、Activity 历史**独立持久化**不再依赖 Velero CR 是否存在、Force Delete 的副作用（BSL 不清 + 60s 后被 backup-sync 重建 + 历史 12 个老 DBR 残留陷阱）被**明示警告 + 提供根治工具**。

**一句话**: 所有删除操作可追溯、删除中防误操作、3 个月前的 RP 即使早被 retention 清掉, Activity 仍能查到它执行过、Force Delete 不再悄悄留下垃圾。

#### 1.1 关键根因（必读, 决定整个 PRD 架构选型）

**当前 SupKube 的 Activity 列表是"实时派生 view", 不是持久化 audit log**:

- `internal/api/v1/handlers.go :: ListActions` 当前实现是: list Velero `Backup` CR + list Velero `Restore` CR → 按 timestamp merge sort → 返回前端
- 一旦 Backup CR 被 cascade 删除（用户主动删 RP / retention TTL 到期 / Force Delete 抹掉 finalizer）→ Activity 列表里**那一行立刻消失**
- **结果**: Mars 2026-05-31 EOD 反馈"删完 5 个 RP 后 Activity 全空"。这不是 UI bug, 是数据模型缺陷——Activity 从来没被持久化过, Velero CR 就是它的唯一存储

**正确做法**: Activity 必须是**独立的 audit event 流**, 每个 Task（Backup / Restore / BackupCopy / Import / DeleteBackup / ForceDeleteBackup / DRDrill）的生命周期事件都进 event store。ListActions API 改为**union(Velero CR 状态视图, audit event 历史视图)**——CR 活着时拿实时 phase, CR 死了时拿历史 event 重建那一行卡片。

**这是 PRD-006 Activity Timeline 的数据层基石**: 没有持久化 audit log, PRD-006 的 Timeline / Log Viewer 跳转 / AI 根因 全是空中楼阁——RP 一删就什么都看不到了, AI 根因都没数据可学。

### 2. Epic（史诗故事）

**"客户运维零摩擦"** — 所有 SupKube 触发的操作必须可追溯（合规审计要查）、不丢历史（3 个月后回头查 incident）、防误操作（删除中不能再点删 / 再点 Restore）。

### 3. User Stories（用户故事）

**S1 — 删除即 Activity 一等事件**
作为运维, 我在 Restore Points 列表批量勾选 5 个 RP 点删除, 立刻看到 **Activity 顶部出现 5 张 "DeleteBackup" Task 卡片**, 每张含进度阶段（Submitted → DBR-Created → BSL-Cleanup → VSC-Cleanup → CR-Removed → Completed）、可点击跳详情 + Log Viewer 看 Velero DBR controller 真实日志。

**S2 — 删除中 RP 锁定**
作为运维, 我看到删除中的 RP **行变灰 + kebab 菜单禁用 + checkbox 锁定**, hover 提示 "正在删除, 关联 Activity Task #N （点击跳转）"。我此时不能再点删除（防重复触发）、不能再点 Restore（CR 即将消失, 启动 Restore 会失败）、不能再点 Export。

**S3 — 历史合规审计**
作为审计员, **3 个月前的 Backup 早已被 90 天 retention 清理, BSL 上数据也已 GC, 但 Activity 仍能查到该次 Backup 执行历史**: 谁触发（user / cron / API）、何时 created / completed、结果 phase、BSL 路径（即使路径已失效, 用于复盘）、关联的 cluster_id。这是合规（SOC2 / 等保）必须能交付的证据。

**S4 — Force Delete 副作用知情**
作为运维, 我点 Force Delete 时弹窗用**红色横条 + 必勾 checkbox** 明示: "BSL 数据不会清 → 60s 内 Velero backup-sync 会重建 CR → 你会看到这个 RP 又出现 → 需要先去 BSL 手动 mc rm 才能彻底删"。**改名**: "Force Delete" → "**Force Strip Finalizer（绕过 Velero, 不清 BSL）**" 更准确反映行为。

**S5 — 孤儿清理**
作为 admin, 我能在 Settings → Backup Storage 看到 **"扫描孤儿 backup"** 按钮, 点击后列出"BSL 上有 backup 数据但 K8s 里无对应 Backup CR"的孤儿（多数来自历史 Force Delete 残留）, 我可以一键复制 `mc rm --recursive ...` 命令到剪贴板手动清, 也可以勾选后一键 SupKube 帮我执行（需二次确认 + 审计）。

### 4. Functions（业务逻辑 / 功能拆解）

#### 4.1 Activity 持久化层（架构改造, **P0 优先级最高**）

**4.1.1 Event 模型**

新增 `internal/audit/event.go`:

```go
type ActivityEvent struct {
    ID          string            // ULID, 时间有序可排
    TaskID      string            // 一个 Task 多个 event 共用同一 TaskID
    ClusterID   string            // 多集群隔离
    Namespace   string
    ActionType  ActionType        // Backup / Restore / BackupCopy / Import / DeleteBackup / ForceDeleteBackup / DRDrill / Export / OrphanCleanup
    Phase       string            // Submitted / InProgress / phase-X / Completed / Failed / PartialFailed
    ResourceRef string            // 关联的 Velero CR name (CR 死后用于在 UI 上展示"已删除"链接, 不解析)
    Triggered   TriggeredBy       // { Kind: user/cron/api/system, Identity: <username/cronName/...> }
    Message     string            // 人类可读
    ErrorCode   string            // ERR_DBR_TIMEOUT / ERR_BSL_AUTH / ... 结构化错误码 (跟 ADR-035)
    Metadata    map[string]string // BSL 路径, 卷数量, fingerprint 等任意附加上下文
    Timestamp   metav1.Time
}
```

**4.1.2 存储选型（拟 ADR-039 决定）**

> ⚠ **已被 §13 D1 修订（2026-06-02）——以 §13 为准**：下表的"预倾向 B (CRD)"**已翻转**。§13 D1 确认 CRD / K8s Event 都是 **etcd 反模式**（容量 2-8 GB 顶天、apiserver 压力），**拟方案改为下表的 C「嵌入式 store on PV」**（§13 细化为 BadgerDB / BoltDB / SQLite + PVC）。下表保留作决策历史与备选分析。

三选一, 评审时 Mars 拍板:

| 选项 | 优 | 劣 |
|---|---|---|
| **A. K8s 原生 Event (corev1.Event)** | 零新依赖, kubectl 可查 | **默认 1h TTL, 必须 cluster admin 改 `--event-ttl=720h`** + Event 数据模型贫弱 (无结构化字段) |
| **B. 独立 CRD `ActivityEvent`** | 数据模型自定、kubectl 可查、ETCD 持久 | ETCD 容量压力（万级事件）+ list 性能差; 需 informer cache |
| **C. 独立 store (SQLite on PV / ConfigMap 分片 / 外部 PG)** | 性能好, 查询灵活 | 新依赖 + 备份 / 多集群同步麻烦 |

**结论（§13 D1 修订后）**: **拟方案 = C「嵌入式 store on PV」**（BadgerDB / BoltDB / SQLite + PVC；0 apiserver 压力、append-only >10k/s、跟 ADR-019 互补）。**B (CRD) 与 A (Event) 均否决**——二者都把审计流压进 etcd，是 finding D1 点名的反模式。仍待 **Phase 0 实测**坐实（1 万 event 写 < 500ms、list 100 条 < 100ms、容量增长 < 10MB/万条；见 §8 DoD #19）；集群损毁后存活靠归档到 BSL（§13 D3）。

**4.1.3 写入时机（每个 Task lifecycle 至少 5 个 event）**

```
T0  TaskCreated     (用户 / cron / API 触发, Phase=Submitted)
T1  TaskStarted     (controller 开始执行, Phase=InProgress)
T2  PhaseChanged    (Velero CR phase 推进, 可多条)
T3  TaskCompleted / TaskFailed / TaskPartialFailed  (终态)
T4  TaskCleanedUp   (CR 被删 / TTL 到期 / Force Delete 触发, 此时 Activity 仍可见但 ResourceRef 标 "removed")
```

**4.1.4 ListActions API 改造**

```
GET /api/v1/activity?cluster=<>&ns=<>&since=<>&types=<>&limit=100
```

返回逻辑（伪代码）:

```
events = audit_store.list(filter)
crs    = velero.list_backup() + velero.list_restore() + ...
return union_by_task_id(events, crs)  // CR 活着用 CR 实时数据, CR 死了用最后一个 event 重建
```

前端展示一致, 唯一区别: CR 已删的 Task 卡片右上角加 ⚠ icon "底层资源已清理, 数据来自审计历史"。

**4.1.5 TTL 与清理**

- 默认 **90 天** (跟 Velero retention 持平)
- 企业版可配 **≥ 180 天** (跟 `SECURITY.md §6.E AI 审计日志` 保留要求对齐)
- 后台 cron 每日 02:00 跑清理: `delete from activity_event where timestamp < now() - ttl`
- 清理前发 K8s Event 提醒 admin (避免静默丢)

**4.1.6 新增 ActionType（跟 PRD-006 / PRD-007 联动）**

`DeleteBackup`（普通删除）/ `ForceDeleteBackup`（绕过 Velero）/ `BackupCopy`（PRD-007 Layer 4）/ `ImportRP`（PRD-001 + #88）/ `DRDrill`（PRD-007 Layer 5）/ `OrphanCleanup`（本 PRD §4.4）。

#### 4.2 RP 删除 Task 化

**4.2.1 后端流程改造（`internal/api/v1/handlers.go :: DeleteBackup`）**

当前:
```go
DELETE /backups/:name → velero.CreateDeleteBackupRequest(name) → 200 OK fire-and-forget
```

改造后:
```go
DELETE /backups/:name
  → audit.Emit(TaskCreated, ActionType=DeleteBackup, Phase=Submitted)
  → velero.CreateDeleteBackupRequest(name)  // 返回 DBR name
  → audit.Emit(TaskStarted, Phase=DBR-Created, Metadata={dbr: <name>})
  → 200 OK { taskId: <ULID>, dbrName: <> }
  → 后台 watcher (informer on DBR) 推进:
      DBR.Phase=InProgress → audit.Emit(PhaseChanged, Phase=BSL-Cleanup)
      DBR.Phase=Processed  → audit.Emit(PhaseChanged, Phase=VSC-Cleanup)
      Backup CR 被 GC      → audit.Emit(PhaseChanged, Phase=CR-Removed)
      DBR.Phase=Completed  → audit.Emit(TaskCompleted)
      DBR.Phase=Failed     → audit.Emit(TaskFailed, ErrorCode=<>)
```

**5 阶段定义**: Submitted → DBR-Created → BSL-Cleanup → VSC-Cleanup → CR-Removed → Completed / PartialFailed。

**4.2.2 Force Delete 单独 ActionType**

```
POST /backups/:name/force-delete
  → audit.Emit(ActionType=ForceDeleteBackup, Phase=Submitted, Metadata={warning: "BSL not cleaned"})
  → kubectl patch backup <name> -p '{"metadata":{"finalizers":null}}'
  → audit.Emit(TaskCompleted, Message="Finalizer stripped. BSL data NOT cleaned. Backup-sync may recreate CR in 60s.")
```

**关键**: ForceDeleteBackup 在 Activity 卡片上**高亮红色 + 显示 "⚠ BSL 数据保留 → 60s 内可能被 backup-sync 重建"**, 跟普通 DeleteBackup 视觉区分。

#### 4.3 RP 列表锁定

**4.3.1 后端: GET /backups 返回加字段**

```json
{
  "items": [
    {
      "name": "backup-2026-05-30-abc",
      "phase": "Completed",
      "deletionState": "InProgress",
      "deletionTaskId": "01HXY...",
      "deletionStartedAt": "2026-05-31T10:23:00Z"
    }
  ]
}
```

**检测来源**: list 时附加 query `kubectl get dbr -l velero.io/backup-name=<name>` 看是否有 active DBR (status.phase ∈ {New, InProgress})。性能优化: 一次 list DBR 后内存 join, 不要 N+1。

**4.3.2 前端 Restore Points 列表**

- `deletionState != null` → 整行 `opacity: 0.5` + 灰底
- 行首加 🔒 锁图标
- kebab `⋮` 菜单全部禁用 (灰色 + cursor: not-allowed)
- 行 checkbox 锁定不可勾
- hover 显示 tooltip: "正在删除（关联 Activity Task #<id>）, 点击跳转查看进度"
- 点击 tooltip 链接 → 跳 Activity 详情页对应 Task

**4.3.3 实时性**

- v1: 列表每 **5s 轮询**一次 GET /backups (够用, 删除通常 30s-2min)
- v1.x: 评估 SSE 推送 deletionState 变化（跟 PRD-005 Log Viewer SSE 共用通道）

#### 4.4 Force Delete 副作用警告 + 历史孤儿根治

**4.4.1 Force Delete 弹窗强化**

当前 dialog 太轻量。改造后:

```
┌─────────────────────────────────────────────────┐
│  ⚠ Force Strip Finalizer（绕过 Velero）         │
├─────────────────────────────────────────────────┤
│  ████████████ 红色横条 ████████████              │
│  此操作只删 K8s Backup CR, 不清 BSL 数据。       │
│  60 秒内 Velero backup-sync controller 可能       │
│  重新发现 BSL 数据并**重建同名 CR**, 你会         │
│  看到这个 RP 又出现 = "删了又来" 循环陷阱。       │
│                                                  │
│  根治方法（按顺序执行）:                         │
│    1. 先停 backup-sync (kubectl scale ...)        │
│    2. mc rm --recursive minio/backups/<name>/    │
│    3. 再 force delete CR                         │
│    4. 重启 backup-sync                            │
│                                                  │
│  ☐ 我理解 BSL 数据将保留, 可能引发"删了又来"    │
│     循环, 已知悉根治方法                         │
│                                                  │
│  [取消]                    [确认 Force Strip]    │
│                            (灰色, checkbox 必勾后变红激活) │
└─────────────────────────────────────────────────┘
```

**改名共识**: UI 文案 "Force Delete" → "**Force Strip Finalizer (绕过 Velero, 不清 BSL)**"。代码 API endpoint 可保留 `/force-delete` 兼容, UI 层翻译。

**4.4.2 孤儿清理 endpoint**

> ⚠ **已被 §13 D5 修订（2026-06-02）——以 §13 为准**：**不可对带数据的孤儿裸 `mc rm`**。Velero 卷数据走 Kopia 共享去重仓库 `bucket/kopia/<ns>/`，同 ns 多个 backup 数据块去重共享，裸删会**误删同 ns 其它 backup 的卷数据**。清理 endpoint 必须接 **`mode` 参数**区分：纯元数据孤儿可裸 `mc rm`；带数据孤儿走 `kopia maintenance run`（或 Velero DBR），不裸删。下方 endpoint 已按 §13 D5 更新。

```
POST /api/v1/maintenance/orphan-scan
  → 列出 BSL 上有数据但无 Backup CR 的孤儿:
     1. mc ls minio/backups/ → 列出所有 backup 目录
     2. kubectl get backups -A → 列出所有 CR
     3. diff → 孤儿列表; 每个孤儿判定 kind: "metadata-only" | "with-data"
        (with-data = bucket/kopia/<ns>/ 下有该 backup 引用的 block)
  → 返回 { orphans: [{ name, bslPath, size, lastModified, kind }] }

POST /api/v1/maintenance/orphan-cleanup
  → body: { orphanNames: [...], mode: "metadata-only" | "full-with-kopia-maintenance" }
  → mode=metadata-only:  仅 bucket/backups/<name>/ 元数据目录 → 可裸 mc rm（数据不在这）
  → mode=full-with-kopia-maintenance:
       二次确认 → 触发 `kopia maintenance run --full`（让 Kopia 按引用计数 GC 无引用 block）
       或走 Velero DeleteBackupRequest，**绝不**裸 mc rm bucket/kopia/<ns>/
  → 每个孤儿写 audit.Emit(ActionType=OrphanCleanup, ...)；与 PRD-007 P1 BSL 删除指引共用
```

**UI 入口**: Settings → Backup Storage → "🧹 扫描孤儿 Backup（历史 Force Delete 残留 / BSL 直传数据）" 按钮。

### 5. UI / UX

**5.1 Restore Points 列表（删除中行）**

```
☑ backup-2026-05-30-prod     Completed     2026-05-30 10:00     [⋮]
🔒 backup-2026-05-29-prod    🔄 Deleting    2026-05-29 10:00    [⋮ 禁用]
   └─ tooltip: "正在删除, Activity Task #01HXY..., 点击查看"
☑ backup-2026-05-28-prod     Completed     2026-05-28 10:00     [⋮]
```

**5.2 Activity 卡片（DeleteBackup Task）**

```
┌──────────────────────────────────────────────────┐
│ 🗑 DeleteBackup · backup-2026-05-29-prod          │
│ Task #01HXY... · 由 mars@supkube 触发              │
├──────────────────────────────────────────────────┤
│ ● Submitted ─ ● DBR-Created ─ ◐ BSL-Cleanup ─    │
│ ○ VSC-Cleanup ─ ○ CR-Removed ─ ○ Completed       │
│                                                   │
│ 已耗时 23s · 预计剩余 ~40s                        │
│ [查看 Log]  [取消（仅 Submitted 阶段可用）]       │
└──────────────────────────────────────────────────┘
```

**5.3 ForceDeleteBackup 卡片**

跟 5.2 同, 但顶部红色横条 + "⚠ BSL 数据保留, 可能 60s 内重建"。

**5.4 历史已删 Task 卡片**

```
┌──────────────────────────────────────────────────┐
│ 📦 Backup · backup-2026-02-15-prod ⚠              │
│ Task #01HQR... · 由 cron/daily-backup 触发        │
│ ⚠ 底层 Backup CR 已被清理, 数据来自审计历史      │
├──────────────────────────────────────────────────┤
│ ● 已 Completed (2026-02-15 03:14)                 │
│ 持续 4m23s · BSL 路径: minio/backups/.../ (已 GC) │
└──────────────────────────────────────────────────┘
```

**5.5 i18n 文案键**

- `activity.action.deleteBackup` / `activity.action.forceDeleteBackup` / `activity.action.orphanCleanup`
- `activity.phase.dbrCreated` / `activity.phase.bslCleanup` / `activity.phase.vscCleanup` / `activity.phase.crRemoved`
- `rp.list.deleting.tooltip` / `rp.list.deleting.lockHint`
- `rp.forceDelete.dialog.title` / `rp.forceDelete.dialog.warning` / `rp.forceDelete.dialog.confirmCheckbox`
- `maintenance.orphan.scan.button` / `maintenance.orphan.empty` / `maintenance.orphan.confirmCleanup`

中英文均需提供。

### 6. Out of Scope（明确不做）

- **不重写 Velero 删除流程**: Velero DBR controller 仍是真做事的执行者, 本 PRD 只在外层包装 audit + 状态锁
- **不做 RP 软删除 / undo**: Velero 不支持, 删了真没了; "回收站"诉求要单独立 PRD 重新评估
- **不持久化前端日志查询**: PRD-005 Log Viewer 自己管, 这里只持久化 Activity Task 元数据
- **不做 cross-cluster Activity 聚合 UI**: v1 仅按 cluster_id 隔离查询, 跨集群合并视图留 v1.x
- **不做 Backup / Restore 全量历史 backfill**: 老数据无 audit event, 上线后只有新 event; 老 Backup CR 在时仍走 CR 派生路径

### 7. 非功能性要求

| 维度 | 要求 |
|---|---|
| **性能** | Activity API list 100 条 < 200ms / 10000 条总量按时间 index < 500ms |
| **实时性** | RP 列表 deletionState 5s 内反映 (轮询周期) / Activity Task 阶段变化 5s 内反映 |
| **幂等** | DeleteBackup 重复触发同名 RP → 第二次返回 409 + 指向已有 TaskID, 不创建新 event |
| **审计** | 每个 Task 完整生命周期 event chain 不可篡改 (CRD 用 immutable annotation / store 用 append-only) |
| **权限** | ForceDelete + OrphanCleanup execute 限 admin 角色; OrphanScan 限 ops+ |
| **错误码** | 遵循 ADR-035 结构化错误: `ERR_DBR_TIMEOUT` / `ERR_BSL_AUTH` / `ERR_BSL_NETWORK` / `ERR_VSC_NOT_FOUND` / `ERR_FORCE_DELETE_BACKUP_SYNC_RECREATE` / `ERR_ORPHAN_BSL_DELETE_FAIL` |
| **i18n** | 所有 Task 阶段名 / 锁定 tooltip / Force Delete 弹窗 / 孤儿清理 UI 中英文 |
| **TTL 清理** | cron 每日 02:00 跑; 清理前发 K8s Event; 清理操作本身也写一条 audit event (`ActionType=ActivityTTLPrune, Metadata={count: <n>}`) |
| **观测** | Prometheus metric: `supkube_audit_events_total{action_type}` / `supkube_audit_events_pruned_total` / `supkube_rp_deletion_in_progress` gauge |

### 8. 验收标准（Definition of Done）

| # | 验收点 |
|---|---|
| 1 | 拟 ADR-039 已写, events 存储选型经 Phase 0 实测决定（含 1 万 event 写入 + list 性能数据） |
| 2 | `internal/audit/event.go` 实现 + 单测覆盖 ≥ 80% |
| 3 | DeleteBackup 触发后 5 个阶段 event 真写入 (kubectl 可查 / store 可 grep), 顺序正确 |
| 4 | ListActions API 返回 union(Velero CR, audit events), 老 Backup 删除后该 Task 卡仍在 |
| 5 | 历史合规场景实测: 创建 Backup → 删除 → 等 1 分钟 → Activity API 仍能查到完整生命周期 |
| 6 | RP 列表 deletionState 字段在 InProgress 时正确返回, 5s 内反映 |
| 7 | 前端 RP 列表删除中行: 灰底 + 锁图标 + kebab 禁用 + checkbox 锁 + tooltip 跳 Activity, e2e click 测通过 |
| 8 | Force Delete 弹窗: 红色横条 + 必勾 checkbox 才能 Trigger（不勾按钮灰）+ 文案准确警示 "BSL 不清 / 60s 重建" |
| 9 | Force Delete UI 文案改名 "Force Strip Finalizer (绕过 Velero, 不清 BSL)" |
| 10 | ForceDeleteBackup Activity 卡片红色高亮 + 警告文案 |
| 11 | `/maintenance/orphan-scan` endpoint 实测能列出真孤儿（人为造一个: mc 上传 + 没 K8s CR 的目录） |
| 12 | `/maintenance/orphan-cleanup` **mode=metadata-only** 仅删元数据目录(可裸 mc rm); **mode=full-with-kopia-maintenance** 二次确认 + 走 `kopia maintenance run`/Velero DBR(非裸 mc rm) + 写 audit event（§13 D5）|
| 13 | TTL 90 天默认实测: 修改系统时间 / 调短 TTL 跑 cron, event 被清, 清理操作本身写一条 audit |
| 14 | Activity API 性能: 1 万 events 按时间倒序取 100 条 < 500ms（pprof / benchmark 数据贴 PR） |
| 15 | i18n: 所有新文案键中英文齐全, 切语言 UI 正确 |
| 16 | Prometheus metrics 三个指标在 /metrics 可见且数值正确 |
| 17 | E2E: 删除 5 个 RP → Activity 顶部出现 5 张卡 → 等删完 → CR 都没了 → 刷新 Activity 5 张卡仍在 |
| 18 | 重复触发 DeleteBackup 同名 RP → 第二次返回 409 + 引用已有 TaskID（幂等） |

### 9. 任务拆分

| Phase | 内容 | 估时 |
|---|---|---|
| **Phase 0** | 实测**已定选型**（嵌入式 store on PV，§13 D1）：1 万 event 写 <500ms / list 100 条 <100ms / 容量 <10MB-万条，写 ADR-039 坐实 | 1d |
| **Phase 1** | `internal/audit/event.go` 实现 + ListActions API union 改造 + TTL cron | 3d |
| **Phase 2** | DeleteBackup / ForceDeleteBackup Task 化 + DBR controller informer 联动 + 5 阶段 event 推进 | 2d |
| **Phase 3** | GET /backups 加 deletionState 字段 + 前端 RP 列表锁定 UI + tooltip 跳 Activity | 2d |
| **Phase 4** | Force Delete 弹窗强化 + 改名 + 孤儿扫描/清理 endpoint + Settings UI 入口 | 2d |
| **合计** | | **~10d** |

### 10. 关联文档与任务

- **PRD-006** Activity Task Detail Timeline — **本 PRD 是其数据层基石**（PRD-006 的 Timeline / Log 跳转 / AI 根因 都依赖 audit event 持久化, 否则 RP 一删全黑屏）
- **PRD-005** Log Viewer v2 — Activity Task 详情页跳 Log Viewer 共用 SSE 通道
- **PRD-007** 5 层数据韧性 — ForceDelete 副作用问题源自 Layer 1/2 (本地快照 + BSL 上行) 边界不清, 本 PRD 治症, PRD-007 治本
- **ADR-031** 5 层韧性原则 — Activity 持久化是"可观测"维度的基础设施
- **ADR-035** 结构化日志 — audit event 的 ErrorCode 字段遵循 ADR-035 错误码规范
- **拟 ADR-039** Activity 持久化与 audit event 存储选型 — 本 PRD Phase 0 产出（**2026-06-01 让号**：原占 ADR-037 已被 PRD-011/012 统一数据采集架构取用，本 PRD 改占 039；台账见 架构设计.md §ADR 号台账）
- **task #106** RESTORE-TASK-CENTER + RESTORE-ADVISOR — 可合并入本 PRD（Task Center 本质就是 Activity 持久化的 UI 展现）
- **task #148** （新立, 本 PRD 实施主任务 placeholder）
- **现有代码引用**:
  - `internal/api/v1/handlers.go :: DeleteBackup` / `ForceDeleteBackup` / `ListActions`
  - `internal/api/v1/handlers.go :: GetBackups` (加 deletionState 字段)
  - 新文件: `internal/audit/event.go` / `internal/audit/store_<选型>.go` / `internal/audit/cleanup.go` / `internal/api/v1/maintenance_handlers.go`

### 11. 开放问题（评审时讨论）

| # | 问题 | 备注 |
|---|---|---|
| Q1 | ~~Events 存储选 K8s Event / 独立 CRD / 独立 store?~~ **已由 §13 D1 拍板** | **= 嵌入式 store on PV（BadgerDB/BoltDB/SQLite + PVC），否决 CRD/Event 的 etcd 反模式。** Phase 0 不再三选一，改为实测坐实该选型（DoD #19）|
| Q2 | Force Delete UI 文案改名 "Force Strip Finalizer (绕过 Velero, 不清 BSL)" 是否过长? 是否客户更易理解的中文表达? | 中文备选: "强制摘除 Finalizer（不清云端数据）" |
| Q3 | 孤儿清理用 SupKube 主动定时扫还是只 admin 手动触发? | 倾向手动（自动跑风险大, 误删客户的非 SupKube 数据） |
| Q4 | 锁定 UI 实时性: 5s 轮询 vs SSE 推送? | v1 轮询够用, SSE 等 PRD-005 通道做完复用 |
| Q5 | TTL 默认 90d 是否合理? 是否分 ActionType 配 TTL (e.g. Backup 365d, OrphanCleanup 30d)? | 倾向统一 90d 简化, 高级用户可配 |
| Q6 | ForceDelete 是否要默认 disable, 必须 Settings 显式开启才出现按钮? | 倾向 enable 但红色高危样式, disable 会被客户嫌"不灵活" |
| Q7 | 老 Backup CR 已存在但无 audit event, 上线后是否做 backfill (扫一遍 CR 写 placeholder event)? | 倾向不 backfill（数据不完整反而误导）, Activity 卡片标 "上线前数据无 audit 记录" |
| Q8 | Activity API 是否要支持按 ErrorCode 过滤? (e.g. "查最近 7d 所有 ERR_DBR_TIMEOUT") | 跟 PRD-005 Log Viewer 错误码搜索保持一致体验 |

### 12. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Mars (testing 20260531.md #2) | — | 提出三件: (a) 清理 RP 应作为 Activity Task 记录有日志可查; (b) 删除中 RP 应锁定不可操作; (新增) **Activity 数据被清空 = 严重 bug**——"本身即使是还原点不在了, 也应该是给客户看到执行的历史的" |
| 2026-05-31 | Claude | — → **草稿 (v1)** | 起草 PRD-008。**关键架构判定**: 当前 `ListActions` 从 Velero CR 派生 = 实时 view 不是持久化 audit log → Backup CR 被删 Activity 立刻消失 = Mars 反馈的根因。修复方向: 独立 audit event 流 (拟 ADR-039 选 K8s Event / CRD / 独立 store), ListActions 改 union。**定位**: 本 PRD 是 PRD-006 Activity Timeline 的数据层基石——没有 audit 持久化, PRD-006 全是空中楼阁。删除 Task 化 5 阶段 + RP 列表锁定 + Force Delete 改名/警告 + 孤儿清理 endpoint 一并解决。Q1-Q8 待 Mars 评审拍板, 重点 Q1 存储选型决定整个 Phase 0 路径。|
| 2026-06-01 | Claude (PRD-Review 第六份 跨文档 finding) | — | **ADR-037 让号 → ADR-039**：原 PRD-008 占 ADR-037（审计存储选型），但 2026-06-01 PRD-011/012 立项时 Agent A 把 037 分配给"统一数据采集架构"，台账内部自洽但 PRD-008 仍占旧号 = 撞号复发。按 PRD-Review 第六份 §二建议让号到 ADR-039。本 PRD 全文 6 处 037→039 替换 + 架构设计.md ADR 台账登记。|
| 2026-06-02 | Claude (PRD-Review 第六份 D1-D5 finding 闭环) | 草稿 → **改正中** | 加 §13 PRD-Review 第六份 D1-D5 finding 修订段：**D1** audit 存储选型 (拟方案 a 嵌入式 store on PV, 避 etcd 反模式 + 跟 ADR-019 分工: PRD-008 audit 管 Task 生命周期, ADR-019 audit 管业务操作行为) / **D2** 不可篡改 3 层防御 (hash-chain + admission webhook + WORM hint, "tamper-evident not tamper-proof" 客户预期管理) / **D3** 集群损毁后 audit 存活 (每日/每 1000 事件批量归档到 BSL + HMAC 签名 + `supkube audit verify-archive` 命令在 fresh 集群可拉+验签+重建 timeline) / **D4** deletionState 从 task store 派生不 list DBR (跟 PRD-006 task store SSOT 共用) / **D5** 孤儿清理 mode 参数区分纯元数据 (裸 mc rm OK) vs 带数据 (走 Kopia maintenance run, 跟 PRD-007 P1 共用 BSL 删除指引)。§8 DoD 加 #19-#23 五条新验收点。状态草稿→改正中, 可转研发中。|

### 13. PRD-Review 第六份 D1-D5 finding 修订段（2026-06-02 闭环）

> **修订动因**：PRD-Review-2026-05-31-PRD008-009.md（PRD-Review 第六份 §三）对 PRD-008 给出 5 个 finding（D1-D5），未在原 §4-§12 闭环；本段是**独立闭环段**，每个 finding 给出**修订后的拟方案** + **写进 §8 DoD 的可验证条目**。
>
> **优先**：本段优先于 §1-§12 中冲突点（如果原文跟本段矛盾，以本段为准）。Phase 0 起跑前**先实测本段拟方案**（Verify-Before-Architect, MEMORY §五）。

#### D1（High）audit event 存储选型不可走 etcd 反模式 + 跟 ADR-019 既有审计对账

**finding 原文**：etcd 有容量配额（默认 2-8 GB）+ 与集群状态共享, 不适合高写入 append-only 审计流；大量 CRD 对象损害 apiserver list/watch 与 informer 内存。同时 SupKube 已有 ADR-019 审计日志 (K8s Events + stdout)，本 PRD 必须说清是**取代**还是**并存**。

**拟方案（Phase 0 实测后定）**：

| 选项 | 优点 | 缺点 | 评级 |
|---|---|---|---|
| **(a)** 嵌入式 store on PV（BadgerDB / BoltDB / SQLite）+ K8s PVC | 0 apiserver 压力, append-only 写性能高 (>10k/s), 容量可扩展, 跟 ADR-019 完全互补 | 需要 PVC; 集群损毁后随 PVC 丢失（D3 需归档到 BSL）; pod 重启需 fsync | ⭐ **推荐 (拟方案)** |
| (b) 独立 K8s Event with custom reason | 复用 K8s 原生机制 + apiserver list/watch 已 work; ADR-019 同款 | etcd 反模式（K8s Event 也存 etcd）; etcd 容量 2-8 GB 顶天; 1 万 event = ~50 MB | ❌ 拒（确认 finding 担忧）|
| (c) 自建 CRD 类（每个 audit event 一个 CR）| 强 schema, kubectl 可查 | etcd 反模式 + apiserver 内存爆炸 | ❌ 拒 |
| (d) 外部时序数据库（Loki / TimescaleDB） | 专业方案 | 增运维依赖 + airgap 客户额外采购 + 跟一站式定位冲突 | ⏸ v2 候选 |

**与 ADR-019 关系**：**并存 / 分工**。
- ADR-019 audit = **操作行为审计**（actor / verb / resource / timestamp，"who logged in / who created policy"）
- PRD-008 audit = **Task 生命周期审计**（task_id / state_transition / cause / linked_resource，"DeleteBackup task Pending → Releasing → Done"）
- ADR-039 §6 写明**字段不重叠** + 客户排查时分别查

**DoD 入 §8**：见下方 §8.D1 条目（追加 #19）。

#### D2（High）"不可篡改"必须落实到真实机制

**finding 原文**：原 §4 说"audit event 不可篡改"，但实现层面**没有真实机制**——`internal/audit/event.go` 如果只是普通 Go struct 写 PV，运维 SSH 改 BadgerDB 文件就能改。合规审计不认账。

**拟方案（3 层防御, Phase 1 实施）**：

1. **写入路径强制**：Append API 在写 PV 前**自动追加 hash-chain**（`event.prev_hash = sha256(prev_event_content)`），任何中间 event 改了 → 后续 hash 全错 → `audit verify` 命令检出。
2. **K8s admission webhook**：拦截**所有**对 audit store PVC 的 mount + 对 backend pod 的 exec/cp 请求，凡是不带 `audit-admin-bypass-token` 一律拒。
3. **WORM 文件系统 hint**：PVC 的 StorageClass 加 `worm: true`（Phase 2, 仅 Azure Disk Ultra SSD / AWS EBS io2 支持）。Best-effort，不依赖。

**安全等级**：三层 ranged，不是绝对不可篡改（cloud provider 控制台仍能改 disk）。文档化：

> ⚠ "Tamper-evident" not "tamper-proof"：cloud 控制台权限不收的话，最终管理员仍能改。本系统提供应用层 hash-chain + K8s admission 两道防线，足以应付**审计师视角 + 内部 SRE 视角**的检查。

**DoD 入 §8**：见 §8.D2（#20）。

#### D3（High）集群损毁后 audit 必须存活

**finding 原文**：当前方案 audit 写 PVC，集群 DR 场景下 audit 跟 SupKube 一起死。客户需要 audit 证明"我之前确实备份过, 不是事后伪造"——audit 必须**比业务集群活得久**。

**拟方案（Phase 2 实施, 4 路径）**：

| 路径 | 描述 | 取舍 |
|---|---|---|
| **(a)** Audit 持续归档到 BSL（每日 / 每 1000 事件批量上传 `.audit-archive.jsonl.gz`） | 客户 BSL 一般独立 region; SupKube 死 BSL 不死 | ⭐ **拟主方案** |
| (b) Audit 跨集群 replicate 到管理集群 | 多集群部署可用 | v1.x; 单集群客户白搭 |
| (c) ImportPolicy 同款机制 audit 跨集群 sync | 复用 PRD-009 v2 fingerprint + BSL | ⏸ v2 候选 |
| (d) Customer's SIEM (Splunk/DataDog) syslog forward | 真专业 | airgap 客户不可用 |

**Archive 文件格式**：`.supkube-audit-archive-<from>-<to>.jsonl.gz`，跟 fingerprint json 同源签名（HMAC + clusterUID + sourceClusterName）。

**Archive 验证**：`supkube audit verify-archive --bsl=<bucket>` 在 fresh 集群上拉回 + 验签 + 恢复 audit timeline。

**DoD 入 §8**：见 §8.D3（#21）。

#### D4（Med）deletionState 由 Task 状态驱动, 不要 list DBR

**finding 原文**：原 §4.4 GET /backups 加 deletionState 字段，建议 controller list DBR (`DeleteBackupRequest`) 算 deletionState。但 DBR list 性能差 + DBR 是 Velero internal CR 不该作 SupKube 状态机。

**拟方案**：**deletionState 字段从 Task store 派生**：
- Task store 已经有 `task_id` + `state` + `linked_backup_name`（本 PRD 自身设计的核心数据模型）
- GET /backups handler 改: 拿 backup name → query task store `WHERE task_type IN ('DeleteBackup','ForceDeleteBackup') AND linked_backup_name=<name> AND state IN ('InProgress','Pending')` → 命中 = `deletionState=InProgress`
- 不再 list DBR; DBR 是 Velero internal, 用 task store 是 SupKube 自己的状态机
- 性能: in-memory cache + index by linked_backup_name, 增量 < 5ms

**与 PRD-006 关系**：Task store 就是 PRD-006 Activity Timeline 后端那张 SSOT 表，两 PRD 共用 task 表 = SSOT。

**DoD 入 §8**：见 §8.D4（#22）。

#### D5（High）孤儿清理在 Kopia 共享去重仓库不可裸 mc rm

**finding 原文**（也是 PRD-007 P1 同源）：Velero 卷数据走 Kopia 共享仓库 `bucket/kopia/<ns>/`，**同 ns 多个 Backup 数据块去重共享**。用户 `mc rm bucket/kopia/<ns>/` 会**误删同 ns 其它 backup 卷数据**。

**拟方案**：孤儿清理流程**区分元数据 vs 数据**：

1. **纯元数据孤儿**（`bucket/backups/<name>/` 有目录但 K8s 无 CR）→ 可裸 mc rm（数据不在这）
2. **带数据孤儿**（同 ns kopia 仓库里有数据 block 但 K8s 无任何 backup CR 引用）→ **必须走 Kopia maintenance**:
   - `kopia maintenance run --full` 让 Kopia 自己 GC 没引用的 block（Kopia 用 retention/snapshot tag 跟踪引用计数）
   - 或 Velero DBR 走标准路径自动触发 Kopia GC
3. **UI / endpoint 强制提示**：用户点 "清理孤儿" 必须先选 `--mode=metadata-only` 或 `--mode=full-with-kopia-maintenance`，后者**额外二次确认 + 警示横条**

**文档化**：USER_MANUAL §X "Velero/Kopia 共享仓库正确删除操作指引"——本 PRD 跟 PRD-007 P1 共用一份。

**DoD 入 §8**：见 §8.D5（#23）。

#### §8 DoD 新增条目（D1-D5 finding 落地, 接在原 #18 之后）

| # | 验收点 | finding |
|---|---|---|
| 19 | **D1 闭环**：ADR-039 给出选型决定（拟方案 a 嵌入式 store on PV），含 Phase 0 实测数据（1 万 event 写 < 500ms, list 100 条 < 100ms, 容量增长 < 10MB / 1 万 events）；§6 写明跟 ADR-019 对账规则 |
| 20 | **D2 闭环**：`audit verify <store-path>` 命令存在 + 能检出 hash-chain 篡改; admission webhook 拒任何不带 token 的 audit PVC mount 请求 (e2e test 验); 文档化"tamper-evident not tamper-proof" 客户预期管理 |
| 21 | **D3 闭环**：audit 每日 / 每 1000 事件批量归档到 BSL `.supkube-audit-archive-*.jsonl.gz` (HMAC 签名); `supkube audit verify-archive` 命令在 fresh 集群可拉 + 验签 + 重建 timeline (e2e test 验) |
| 22 | **D4 闭环**：GET /backups deletionState 字段从 task store 派生（不 list DBR），增量延时 < 5ms（benchmark 数据贴 PR）|
| 23 | **D5 闭环**：孤儿清理 endpoint 必须接 `mode` 参数（`metadata-only` 或 `full-with-kopia-maintenance`），后者二次确认 + 真触发 `kopia maintenance run`（或 Velero DBR）而非裸 mc rm; USER_MANUAL §X 共用 PRD-007 P1 的 BSL 删除指引 |

---

<a id="prd-009"></a>
## PRD-009 — Policy 模型对齐 Kasten（Snapshot + Enable Backup via Snapshot Export）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-009 |
| **任务编号** | #149（新立, Snapshot UX 词汇重构）/ #156（Phase 1 落地 2026-06-01）/ **#88（P0 升级, Import Policy + fingerprint 配对模型）**/ **#157-163**（v2 Phase 2: #157 ImportPolicy CRD schema + #158 controller reconciler + #159 fingerprint enforce/warn/disabled 三模式落地 + #160 Continuous poll engine + #161 Scheduled cron engine + #162 Action Type 顶部 pill UI + #163 RPO 实时显示 + USER_MANUAL §Import Policy）|
| **状态** | **研发中（2026-06-03 Mars 评审通过）· v1 Phase 1 已 ship（Snapshot/Export 词汇 2026-06-01）, Phase 2 扩展 Action 类型 + Import Policy 进行中（task #157-163）** — Phase 1 的 Snapshot/Export 词汇主体已 ship 到双集群（docker-desktop + AKS）；Phase 2 新增 **Action Type 二选一（Snapshot Policy vs Import Policy）** + ImportPolicy CRD/controller + Continuous/Scheduled 子模式 + fingerprint 三模式（enforce / warn / disabled）；E1/E2/E4 Phase 1 留尾仍在 v1.x 渐进闭环 |
| **作者** | Claude / Mars |
| **目标版本** | v0.10.x（Phase 1 已 ship 0.9.1.12 alpha）/ **v0.9.1.13（Phase 2 Action 类型 + Import Policy 目标版本, Mars 2026-06-01 today 要求）** |
| **关联 ADR** | ADR-025（dual schedule pair, Snapshot/Export 双 Schedule 实现）/ ADR-026（policypair controller）/ **ADR-038（新增, ImportPolicy CRD + Controller 设计, 见架构设计.md §9）** |
| **关联 PRD** | PRD-007 §4.3（Backup Copy 与 Snapshot Export 的关系：Layer 4 是跨云复制 Snapshot Export 后的产物, 不替代本 PRD 的 UX 重构）/ **PRD-007 §4.4（Export/Import 配对模型 + fingerprint 模块设计 = HMAC + TrustStore, 本 PRD §4.5 是其用户面 Policy 包装）**|
| **关联文档** | USER_MANUAL.md §Policy（待同步替换 vocabulary, Phase 2 加 §Import Policy 子章节）/ ROADMAP.md "行业 vocabulary 对齐" + "跨集群 DR 流转" |
| **反向兼容** | Phase 1 后端数据模型（Velero Schedule + `supkube.io/dual=true/false` annotation）**不变**, 仅 UI 词汇层重构, 0 迁移风险。**Phase 2 兼容路径 (2026-06-01 PRD-Review G2 修正)**: Velero `backupSyncPeriod` **保持默认 60s**（不调长——避免无 ImportPolicy 的 BSL 同步退化到 5min 是行为 regression）。Agent G v0.9.1.13.1 加了**直 S3 BackupLister** 复用 fingerprint BSL client, ImportPolicy controller 直接 LIST BSL 而非依赖 Velero sync, 与 Velero 60s sync **并行 race-safe**（双方都 IsAlreadyExists 跳过, 旧 NewVeleroBackupLister 保留作 fallback）。**Phase 2 风险等级**: 不再是 Phase 1 的 "0 迁移风险"——新增 CRD + RBAC + Secret + Helm + controller 复杂度, 详见 §风险评级（待 A3 补）|

> **立项缘由（2026-05-31）**：Mars 在 testing20260531.md 第 4 条反馈——当前 Policy 新建用 "L1 本地 / L2 本地云端" 这套我们自创的 vocabulary, 不符合行业标准。Mars 要求改成 **Kasten K10 一致**: 默认 Action = "Snapshot", 下面有 **"Enable Backup via Snapshot Export"** 开关, 勾选后再选 Snapshot Export 的存储桶（BSL）。附 3 张截图: testing20260531/1780213569295.png（当前 SupKube L1/L2 UI）/ 1780213542977.png（Kasten 风格参考）/ 1780213613916.png（第三张参考）。

### 1. Goal

把 Policy 编辑 UX 从 **"L1 / L2 互斥按钮"（自创 vocabulary）** 重构为 **"Snapshot 默认始终开 + Enable Backup via Snapshot Export 开关 + 选 BSL"**（Kasten K10 风格）, 让从 Kasten / Veeam / Trilio 迁来的客户 **0 学习成本** 直接上手, 同时消除 SupKube 新客户"L1 vs L2 到底选哪个"的认知负担。

**关键: Kasten 的 Policy 模型（我对 Kasten 的认识）**
- 默认 Action = **Snapshot**（本地, 通常 instant + retention 较短, 走 CSI snapshot / 底层存储快照）
- 可勾 **"Enable Backup via Snapshot Export"** → 把 Snapshot **导出（export）** 到对象存储 BSL（云端或本地 S3-compatible）
- 二者**不是** "L1 vs L2 互斥"，而是 **"Snapshot 必有 + Export 可选叠加"**
- 这与 SupKube 现有 L1（仅 snapshot, 无 export）/ L2（snapshot + export）**本质相同**, 但 UX 名词 + 视觉表达不同

**对齐 Kasten 的产品意义**
- 客户从 Kasten 迁来不用重学概念（vocabulary 兼容）
- 与 Veeam / Trilio / Kasten 行业 vocabulary 对齐（"Snapshot" / "Backup" / "Export" 是数据保护行业通用词）
- "Snapshot Export" 比 "L2 本地云端" **更直白**——前者描述行为（导出快照）, 后者描述抽象层级（让客户猜）

### 2. Epic

**"行业标准 vocabulary 对齐"** Epic——SupKube 不发明轮子, 与 Kasten K10 / Veeam Kasten / Trilio TVK 等成熟数据保护平台的术语保持一致, 让客户认知迁移成本 = 0。本 PRD 是该 Epic 的首个 user-facing 落地。

### 3. User Stories

- 作为 **从 Kasten K10 迁来的客户**, 我打开 SupKube Policy 新建抽屉, 看到的是 "Action: Snapshot（始终）+ ☐ Enable Backup via Snapshot Export + 选 BSL 下拉" 三件套, **与 Kasten K10 Policy Wizard 完全一致**, 我立刻知道怎么填, 不需要看任何文档。
- 作为 **SupKube 老客户**（习惯了 L1 / L2 词汇）, 我打开新 UX 第一时间能 map: "原 L1 = 只 Snapshot 不勾 Export; 原 L2 = Snapshot + 勾 Export"; 我的老 Policy YAML 加载到新 UI 自动反映正确状态, 不需要手动迁移。
- 作为 **SupKube 新客户**（从未用过 L1/L2 概念）, 我不需要理解 "L1 vs L2 到底什么意思", 直接用 "Snapshot / Export" 这套行业标准词汇就能配出我想要的备份策略。
- 作为 **销售/Pre-sales**, 我跟客户演示 Policy 时不再需要花 2 分钟解释 "我们的 L1 大致相当于 Kasten 的 Snapshot, L2 相当于 Kasten 的 Snapshot + Export...", 直接说 "和你熟悉的 Kasten 一样"。
- 作为 **Compliance Officer**, 我在 Policy 列表里看到的标签是 "Snapshot only" / "Snapshot + Export", **直接对应等保 / SOC2 审计常用词**, 不用解释"L1/L2 是 SupKube 自创的什么意思"。

### 4. Functions

#### 4.0 Action 类型（Snapshot Policy vs Import Policy）— **v2 Phase 2 新增**

**为什么要二选一**：Mars 2026-06-01 today 产品决策——Policy 顶部应该有 **Action Type pill group**, 两个互斥选项:

| Action Type | 含义 | 谁该用 | 类比 Kasten K10 |
|---|---|---|---|
| **📸 Snapshot Policy** | 源集群备份 (本 PRD-009 v1 Phase 1 的全部 = Snapshot 始终开 + Enable Backups via Snapshot Exports toggle) | 集群是**数据源**, 业务在这边跑, 需要本地快照 + 可选导出云 BSL | Kasten "Snapshot" Policy |
| **📥 Import Policy** | 目标集群从共享 BSL **持续拉新 RP** (含 fingerprint HMAC 校验), 替代 Velero 默认 60s `backupSyncPeriod` 全量扫这一兜底方案 | 集群是**目标/灾备集群**, 业务不在这边跑, 只为接收源集群导出的 RP (DR / 跨地域 / 跨账号) | Kasten "Import" Policy（K10 multi-cluster 场景同名） |

**决策树（谁该用哪种）**：

```
┌────────────────────────────────────────────────────────────┐
│ Q1: 本集群的业务在不在 production 跑 (有要保护的 workload)? │
└──────────┬─────────────────────────────────┬───────────────┘
           │ YES                             │ NO
           ▼                                 ▼
   ┌───────────────────┐           ┌─────────────────────┐
   │ Q2: 备份要留远端? │           │ Q3: 接收哪个集群    │
   └──┬─────────────┬──┘           │     export 的 RP?  │
      │ NO          │ YES          └─────────┬───────────┘
      ▼             ▼                        ▼
  Snapshot      Snapshot          ┌──────────────────────┐
  Policy        Policy +          │  Import Policy       │
  (snapshot     Enable Backups    │  (BSL + cluster      │
   only)        via Snapshot      │   filter +           │
                Exports + BSL     │   fingerprint mode)  │
                                  └──────────────────────┘
```

**与 Kasten K10 概念对齐**:
- Kasten K10 文档明确把 Policy 分为 "Snapshot/Backup Policy"（源端）vs "Import Policy"（目标端）, 二者**互斥**, 不在同一 Policy CR 内混配
- K10 Import Policy 默认 5 分钟轮询一次源端 export profile（Kasten 的 floor）, SupKube 跟进且**做得更细**（30s 起步, 见 §4.5 卖点对比表）
- K10 Import 也支持 cluster identity 校验（K10 走 license-bound cluster ID）, SupKube 走 HMAC 签名 + TrustStore（PRD-007 §4.4）

**UI mockup（Policy 新建抽屉顶部 Action Type pill group）**：

```
┌─ New Policy ──────────────────────────────────────────────┐
│                                                           │
│  Action Type:                                             │
│  ┌──────────────────────────┬──────────────────────────┐  │
│  │ ● 📸 Snapshot Policy     │ ○ 📥 Import Policy       │  │
│  │   (back up this cluster) │   (import RPs from BSL)  │  │
│  └──────────────────────────┴──────────────────────────┘  │
│                                                           │
│  ─── 选 Snapshot Policy 时 ────────────────────────────   │
│  Name: [my-app-backup]                                    │
│  Namespace: [production ▾]                                │
│  Action: ● Snapshot (always on)                           │
│  ☐ Enable Backups via Snapshot Exports                    │
│    (展开后选 BSL + Schedule + Retention)                  │
│  Snapshot Schedule: [Hourly ▾]                            │
│                                                           │
│  ─── 选 Import Policy 时 ──────────────────────────────   │
│  Name: [from-prod-cn-east-1]                              │
│  Source BSL: [aws-s3-east ▾]   (共享 BSL, 必填)           │
│  Source Cluster Filter: [prod-cn-east-1 ▾] (按 fp 列出)   │
│  Sub-mode:  ● Continuous  ○ Scheduled                     │
│    (Continuous → poll 间隔下拉; Scheduled → cron 输入)    │
│  Poll Interval: [60s ▾]  (30s / 60s / 2m / 5m)            │
│  Fingerprint Mode: ● enforce  ○ warn  ○ disabled          │
│  ──────────────────────────────────────────────────────   │
│  🛡 当前 RPO ≤ <X> min  (实时根据 source + poll 计算)     │
│                                                           │
│              [Cancel]  [Save Policy]                      │
└───────────────────────────────────────────────────────────┘
```

**实施约束**:
- Action Type pill group **必须最顶**, 在 Name 字段之上（先决定走哪条数据流, 再填业务字段）
- pill 选中后**切换分支表单**, 老分支字段清空（防 stale state）
- pill **不可逆**（Save 后该 Policy 的 Action Type 不可改, 只能删了重建）——Snapshot Policy 写到 Velero Schedule, Import Policy 写到新 ImportPolicy CR, 数据模型不同, 改类型 = 改 CRD 类型, 无法 in-place migration
- **(G4 Low 2026-06-02 PRD-Review 闭环)**: Save 前 UI 防呆——Action Type pill 下方加固定 inline alert（El-Alert type=warning, 不可关闭）"⚠ Action Type (Snapshot/Import) 保存后**不可更改**, 选错需删除策略后重建。当前选择: <Snapshot ▾>"; 编辑现有 Policy 时该 pill 灰化为只读, 后跟 "(Action Type 不可改, 这是设计约束, 见 USER_MANUAL §Policy)" 文案; Save 按钮触发 confirm dialog "你正在创建 <Snapshot|Import> 类型的 Policy, 此类型不可更改, 是否继续？"（首次创建强制弹一次, localStorage 记 dismissed 标记）
- 列表页 Policy 行加 Action Type icon 前缀（📸 / 📥）+ filter 下拉支持按 Action Type 过滤

#### 4.1 Policy 新建 / 编辑抽屉重构（前端 P0）

- **移除** 当前 Policies.vue L321-L342 的 L1 / L2 互斥按钮组（`<el-radio-group>` 或类似结构）。
- **新 UX 结构**:
  - **Action: Snapshot**（固定显示, 不可取消勾选）
    - 默认 BSL = `local-store`（cluster-local CSI snapshot 或 in-cluster MinIO；视集群配置自动选）
    - 文案下方加 hint: "本地快照, 通常 instant, 适合短周期 RTO; 不离开本集群"
  - **☐ Enable Backup via Snapshot Export**（checkbox, 默认未勾）
    - **未勾**: 仅本地 Snapshot, 等同原 L1
    - **勾选**: 下方展开 "Snapshot Export Storage: [BSL 下拉]"（仅显示 cloud / external BSL, 过滤掉 local-store）
    - 文案 hint: "把 Snapshot 导出到对象存储以实现远端保留 / 跨集群恢复 / 长期归档（3-2-1 规则的 '2 种介质'）"
  - **Schedule / TTL / Retention** 部分**不变**:
    - Snapshot 的 schedule + retention 独立配置
    - Export 的 schedule + retention 独立配置（仅在 Export 勾选时显示该 section）
- **提交时数据转换**:
  - 不勾 Export → 后端写 `supkube.io/dual=false` annotation, 生成 1 个 Velero Schedule（snapshot only）
  - 勾 Export → 后端写 `supkube.io/dual=true` annotation, 生成 2 个 Velero Schedule（snapshot + export）
  - 复用现有 `policypair.go` controller（ADR-026）, 不写新代码

#### 4.2 i18n（en + zh-CN）

| Key | 原值（v0.9.x） | 新值（v0.10.x） |
|---|---|---|
| `policy.actionSnapshot` | `"L1 Snapshot"` / `"L1 本地快照"` | `"Snapshot"` / `"快照"` |
| `policy.actionSnapshotExport` | `"L2 Snapshot+Export"` / `"L2 本地+云端"` | （废弃, 由下面 toggle 替代）|
| `policy.enableExportToggle` | （新增）| `"Enable Backup via Snapshot Export"` / `"启用通过快照导出的备份"` |
| `policy.exportBSLSelect` | （新增）| `"Snapshot Export Storage"` / `"快照导出存储桶"` |
| `policy.exportToggleHint` | （新增）| `"Export snapshot to object storage for remote retention / cross-cluster restore / long-term archive"` / `"把快照导出到对象存储以实现远端保留 / 跨集群恢复 / 长期归档"` |
| `policy.snapshotHint` | （新增）| `"Local snapshot, typically instant, ideal for short RTO; stays within this cluster"` / `"本地快照, 通常 instant, 适合短 RTO; 不离开本集群"` |

**兼容期处理**: 老 `actionSnapshot = "L1 Snapshot"` 等 i18n key 保留 **1 个版本（v0.10.x）**, v0.11.x 移除（防止旧 toast / 旧通知模板还在用）。

#### 4.3 现有 L1 / L2 列表显示对齐

- Policy 列表（Policies.vue 主列表行）当前的 "L1" / "L2" badge → 改为:
  - 原 L1 → `"Snapshot only"` / `"仅快照"`
  - 原 L2 → `"Snapshot + Export"` / `"快照 + 导出"`
- 颜色 / icon 保持一致（避免老用户视觉错位）, 仅文字变。
- **窄屏（< 768px）兼容**: 若一列宽度不够, 可缩为 `"Snap"` / `"Snap+Exp"`（待 Q3 决定）。

#### 4.4 后端数据模型保留（**不动**！）

- **Velero Schedule**（snapshot / export 两个独立 Schedule）保留, 字段定义不变。
- **`supkube.io/dual=true/false` annotation**（ADR-025）保留, 语义不变。
- **`policypair.go` controller**（ADR-026, 监听 PolicyPair CR 生成 2 个 Schedule）**完全不动**。
- API 层（GET / POST / PATCH `/api/policies`）请求/响应 schema 不变。
- → **0 后端迁移风险**, 老 Policy YAML 直接加载到新 UI 自动展示正确状态（L2 → Snapshot + Export 勾起 + BSL 自动填）。

#### 4.5 Import Policy 完整规范 — **v2 Phase 2 新增**

> 本节是 **用户面 Policy 包装层**, 落地到 ImportPolicy CRD + controller。fingerprint 模块本身（Writer/Validator/TrustStore/SecretLoader 四件套, HMAC-SHA256 算法, K8s Secret 密钥管理）在 **PRD-007 §4.4** 已完整设计, 本节**不重复**, 仅做 Policy 用户面引用 + Action Type 包装。CRD schema 与 controller 行为详见 **ADR-038**（架构设计.md §9）。

##### 4.5.1 两个子模式

| 子模式 | 触发方式 | 默认值 | 可选值 | 适用场景 |
|---|---|---|---|---|
| **Continuous** | 短 poll, 每 N 秒扫一次源 BSL fingerprint 索引, 发现新 RP 立即注册到本集群 | 60s | **30s / 60s / 2m / 5m** | DR 备站, 要求最低 RPO; 客户付得起 BSL List API 调用成本 |
| **Scheduled** | cron 表达式触发, 走 robfig/cron/v3 解析（已是 PRD-004 §4.4 验证过的解析器） | `0 */1 * * *`（每小时） | Kasten preset (`*/5 * * * *` / `*/15` / `*/30` / `0 */1` / `0 0 * * *`) + 自由 custom cron 输入 + UI 实时解析预览 "下次触发: 2026-06-01 14:00 CST" | 跨账号 / 跨地域成本敏感; 合规要求固定窗口拉取 |

**子模式互斥, Save 后可改子模式但不可改 Action Type**。

##### 4.5.2 RPO 公式 + UI 实时显示

```
当前 RPO ≤ source_backup_interval + import_poll_interval
         ─────────────────────────  ──────────────────────
         （源集群 Snapshot Policy   （目标集群 Import Policy
          的 Snapshot Schedule       的 poll 间隔 or 下次
          cron 周期, e.g. 每 5min)   cron 触发等待）
```

- UI 在 Import Policy 抽屉底部**实时计算并显示** `🛡 当前 RPO ≤ X min`, X = source 集群该 BSL 上能看到的最大 Snapshot Schedule 周期（按 fingerprint.source_cluster_name 反查源 Policy）+ 本 Import poll/cron 间隔
- 反查不到源 Policy 周期时（首次跨集群无信任 / 源还没 export）, 显示 `🛡 当前 RPO 待确定（源未导出第一个 RP）`, 不阻塞 Save
- 客户改 poll interval 时数字**实时刷新**（前端 watch + setTimeout 节流 250ms）

##### 4.5.3 卖点对比表（Kasten K10 vs SupKube Import Policy）

| 维度 | Kasten K10 Import Policy | SupKube Import Policy (本 PRD) | 备注 |
|---|---|---|---|
| **Poll 间隔 floor** | 5 min (K10 文档 hard floor) | **30s** (Continuous 最快档) | **诚实表述 (2026-06-01 PRD-Review G1 修正, 撤回原"10x RPO"过度承诺)**：RPO 公式 = `source_backup_interval + import_poll_interval`——RPO 几乎总被**源备份周期主导**。源每小时备份时, import 60s vs 5min 对 ~1h 的 RPO 只差几分钟; 把 BSL LIST API 成本 ×10 不划算。**默认 60s/2min, 30s 仅当源亚分钟级备份（罕见场景, e.g. 关键 ns + 小数据集）时才有 RPO 收益**。SupKube 真实卖点 = fingerprint HMAC 防篡改 + source-cluster 过滤 + RPO 可调 + 暂停/run-once 控制, 不是"快 10 倍"。|
| **Scheduled 模式** | ✅ cron preset | ✅ cron preset + 自由 custom | parity, Kasten 没开放完全自由 cron |
| **fingerprint 校验** | license-bound cluster ID（K10 闭源 license server） | HMAC-SHA256 + 客户持有 shared secret（PRD-007 §4.4） | 开源友好, 不依赖 license server |
| **拒绝/告警/关掉 三模式** | 二档（强制 / 关闭） | **三档 enforce / warn / disabled**（见 §4.5.4） | SupKube 多一档 warn, 满足单集群 dev 场景"我知道没签但先用" |
| **状态机 + 暂停按钮** | 有 (Pause/Resume) | ✅ Active ⇄ Paused ⇄ Failed (见 §4.5.6) | parity |
| **Run Once 一次性导入** | 有 | ✅（复用 ADR-025 RunOnce 模式, 同时间戳后缀） | parity |
| **CRD 类型** | 闭源 ImportPolicy.kio.kasten.io | 开源 `importpolicies.supkube.io/v1`（见 §4.5.5） | parity, 开源 schema |

##### 4.5.4 Fingerprint 三模式（enforce / warn / disabled）

> **本节是 Policy 用户面三档选择, 算法/密钥/校验码细节在 PRD-007 §4.4**。三档对应 PRD-007 §4.4 `FingerprintStatus` 状态机的不同处置策略：

| 模式 | 行为 | 适用场景 | 与 PRD-007 §4.4 关系 |
|---|---|---|---|
| **enforce** （**跨集群默认**, hard required） | `FingerprintStatus != ok` → 拒绝 Import, RP 不出现在本集群列表, audit `ERR_FINGERPRINT_REQUIRED`; 缺签名 / 签名无效 / hash 对不上 → 全部拒 | 生产 DR / 跨账号 / 跨地域, 任何被 BSL 篡改的 RP 都不能进 | PRD-007 §4.4 "跨集群场景 signature 必需" 的 Policy 层落地 |
| **warn** | `FingerprintStatus != ok` → RP 仍 Import 但 UI 标 `⚠ 完整性未校验` chip + audit `ERR_FINGERPRINT_WARN`; 用户点 Restore 时**额外** confirm dialog "该 RP 未通过 fingerprint 校验, 仍要恢复?"。**(G3 Low 2026-06-02 PRD-Review 闭环)**: hover chip 时 tooltip 补一行 "fingerprint 缺失也意味着 RP **可能未完成** —— 源备份完成才会写 fingerprint, enforce 模式天然兼"备份完成"门信号; warn 模式失去该保护, 这个 RP 可能是源正在写、尚未完成的半成品, 用前请人工确认源 backup status"; 列表行的 Import 时间列加 `<source RP 状态>` 显示来源 backup phase（Completed/InProgress/Failed）| 单集群 dev / staging, admin 知道没配 shared secret 但想用 | PRD-007 §4.4 "单集群 signature optional" 的扩展（生产严格化 + UI 提示） |
| **disabled** | 完全跳过 fingerprint 校验, 信任 BSL 上的所有 fingerprint JSON, RP 直接 Import | 客户已有外部 WORM / Object Lock 兜底（PRD-007 §4.2）, 不想叠加 HMAC 层 | PRD-007 §4.4 威胁模型注脚 "WORM 替代 HMAC" 的 Policy 层 opt-out |

**默认值**:
- 跨集群（target ≠ source, fingerprint.source_cluster_id ≠ 本集群 cluster_id）→ **enforce**
- 单集群（source = target）→ **warn**
- 用户**显式**选 disabled 时 UI 弹红框 confirm "你正在关闭 fingerprint 校验, BSL 数据被篡改时本集群不会发现, 确认?" + audit 落 `AUDIT_FINGERPRINT_DISABLED_BY_USER`

##### 4.5.5 ImportPolicy CRD spec — 完整字段表

> **CRD GVK**: `importpolicies.supkube.io/v1`, scope = Namespaced（与 Velero Schedule 同 ns, 复用 RBAC）。完整 OpenAPI v3 schema 在 ADR-038, 本节是字段说明 + 取值约束。

```yaml
apiVersion: supkube.io/v1
kind: ImportPolicy
metadata:
  name: from-prod-cn-east-1
  namespace: velero
spec:
  # —— 数据源 —— (必填)
  sourceBSL: aws-s3-east                   # 引用本 ns 现有 BSL name (BackupStorageLocation)
  sourceClusterFilter:                     # 按 fingerprint.source_cluster_id 过滤
    clusterId: "<uuid-from-kube-system>"   # 必填 (除非 fingerprintMode=disabled 且 acceptAnyCluster=true)
    clusterName: "prod-cn-east-1"          # display only, 不参与匹配
  # —— 子模式 —— (二选一, 必填一个)
  continuous:                              # 与 scheduled 互斥
    pollInterval: 60s                      # 枚举: 30s / 60s / 2m / 5m
  scheduled:                               # 与 continuous 互斥
    cron: "*/15 * * * *"                   # robfig/cron/v3 解析, UI 提供 preset + custom
  # —— Fingerprint —— (必填)
  fingerprintMode: enforce                 # 枚举: enforce | warn | disabled
  acceptAnyCluster: false                  # 仅 fingerprintMode=disabled 时允许 true (不校验 sourceClusterFilter.clusterId)
  # —— RP 过滤 —— (可选)
  backupNameSelector:                      # label-selector 风格, 默认全收
    matchLabels:
      app.kubernetes.io/part-of: orders
  # —— 控制 —— (可选)
  paused: false                            # true 时 controller 停止 poll, 已 Import 的 RP 保留
status:
  phase: Active                            # Active | Paused | Failed | Pending
  lastPollAt: "2026-06-01T13:00:00Z"
  lastSuccessAt: "2026-06-01T13:00:00Z"
  nextPollAt: "2026-06-01T13:01:00Z"       # Continuous 模式; Scheduled 模式取下次 cron 触发
  observedRPO: "5m30s"                     # 实测 RPO = max(now - rp.createdAt) over imported RPs
  importedCount: 142                       # 本 Policy 累计 import 的 RP 数
  lastError:                               # 仅 phase=Failed 时填
    code: "ERR_IMPORTPOLICY_BSL_AUTH"
    message: "AWS access denied when listing s3://aws-s3-east/backups/"
    occurredAt: "2026-06-01T13:00:00Z"
  conditions:                              # 标准 K8s conditions
    - type: Ready
      status: "True"
      reason: PollingActive
      lastTransitionTime: "2026-06-01T12:00:00Z"
```

**字段约束**:
- `sourceBSL` + `sourceClusterFilter.clusterId` 联合**唯一**——不允许两个 ImportPolicy 在同 ns 引用同 BSL + 同 source cluster（避免 race + 重复 List 计费）, controller webhook 拒
- `continuous.pollInterval` ∈ `{30s, 60s, 2m, 5m}` 严格枚举, **不允许任意值**（防客户填 1s 打爆 BSL API）
- `scheduled.cron` 走 robfig/cron/v3 标准 5 字段格式（分 时 日 月 周）, 不支持 6 字段含秒（与 PRD-004 §4.4 cron 解析口径对齐）
- `fingerprintMode = disabled` 必须配套 audit `AUDIT_FINGERPRINT_DISABLED_BY_USER`（见 §4.5.4）
- `paused = true` 时, controller 不删已 Import 的 RP, 仅停止新 poll; 恢复时从 `status.lastPollAt` 接着 poll

**G3 文档化 (2026-06-01 PRD-Review)**: fingerprint enforce 模式同时充当 **"备份完成门"**——因为 fingerprint 文件由源集群在 Velero `Backup.status.phase=Completed` 时才写入 BSL（参 ADR-038 §写入时机）, **enforce 模式自然只 Import 已完成的 RP**。但 **warn / disabled 模式失去这一保护**, 会 Import 到源**正在写入、尚未完成**的半成品 tarball（Velero 增量上传中）, restore 可能失败或得到不一致数据。**UI 必须对 warn/disabled 模式 Import 的、源 fingerprint 缺失的 RP 显示 chip "⚠ 可能未完成 (源未提供完成标记)"** + 在 Restore Wizard 加二次确认。

**G4 文档化 (2026-06-01 PRD-Review)**: ImportPolicy 与 SnapshotPolicy 是**两个不同 CRD**（前者 `importpolicies.supkube.io`, 后者 Velero `Schedule`）, **Action Type Save 后不可改**——选错只能删重建。**UI Save 前必须显示明确告警**: "Action Type 保存后不可更改; 如需切换 Snapshot ↔ Import, 需删除当前 Policy 后新建。" Frontend 表单提交前弹 confirm 对话框, 客户点 "确认 Save" 才提交。

##### 4.5.6 状态机

```
        ┌─────────┐  spec valid +    ┌──────────┐
        │ Pending │ ───── BSL ──────▶│  Active  │
        │ (新建)  │      可达        │ (polling)│◀──┐
        └─────────┘                  └────┬─────┘   │
                                          │         │ paused=false
                            paused=true   │         │ (恢复)
                                          ▼         │
                                     ┌──────────┐   │
                                     │  Paused  │───┘
                                     │ (停 poll)│
                                     └──────────┘
                                          ▲
                                          │ controller 重启
                                          │ (保持 Paused, 不自动恢复)
                                          │
                            poll N 连续失败 (默认 N=5)
                                          │
        ┌─────────┐                       │
        │ Failed  │ ◀─────────────────────┘
        │ (lastErr│
        │  填)    │
        └────┬────┘
             │ admin 修 spec / 修 BSL / 修 secret
             │ → controller 自动重试
             ▼
         Active
```

**状态转换**:
- **Pending → Active**: CR 创建后 controller 首次 poll BSL 成功, 写 `status.phase = Active`
- **Active → Paused**: `spec.paused = true` 用户主动暂停; 或 controller 检测 BSL 删除 / 重命名（自动 Paused 而非 Failed, 避免误报）
- **Paused → Active**: `spec.paused = false`, controller 立即拉一次（不等下个 cron / poll tick）
- **Active → Failed**: 连续 N=5 次 poll 失败（BSL auth 错 / network 错 / fingerprint 全部 enforce 拒）, controller 写 `status.lastError` + 停 poll, 等 admin 介入
- **Failed → Active**: admin 修复后下一次 reconcile（spec 改动 / 30min 强制 retry）自动恢复

##### 4.5.7 错误码列表

> 错误码遵循 ADR-035 `ERR_*` 体系。本 PRD 新增 **2 个 family**:

| 错误码 | severity | 含义 | 触发条件 | 用户操作 |
|---|---|---|---|---|
| **`ERR_FINGERPRINT_REQUIRED`** | error | 跨集群 Import 时 fingerprint JSON 不存在 | source 集群未配 sharedSecret, 写 BSL 时无签名 | 源集群 admin 配 `--set fingerprint.sharedSecret=<base64-32B>` 重启 |
| **`ERR_FINGERPRINT_MISSING`** | error | fingerprint JSON 应存在但 BSL 上找不到 (file not found at `<prefix>/.supkube-fingerprint.json`) | 网络部分失败 / BSL 权限不足 / 文件被人为删 | 检查 source 写权限 / BSL 完整性 |
| **`ERR_FINGERPRINT_INVALID`** | error | signature 验签失败（HMAC mismatch） | target 集群密钥与 source 不匹配 | 同步 sharedSecret（Helm `--set` 双集群一致） |
| **`ERR_FINGERPRINT_HASH_MISMATCH`** | error | tarball SHA256 与 fingerprint 记录的 rp_sha256 不一致 | BSL 数据损坏 / 传输错误 / 篡改 | 重新跑 source backup 或检查 BSL 健康 |
| **`ERR_FINGERPRINT_CLUSTER_UNTRUSTED`** | warn (enforce 时 error) | source cluster_id 不在 TrustStore 白名单 | 首次见到新 source 集群 | Settings → Trust 源集群 cluster_id（走 PRD-007 §4.4 TrustStore） |
| **`ERR_FINGERPRINT_WARN`** | warn | fingerprintMode=warn 时, 校验失败但放行 | 见 enforce 列各项触发条件 | UI chip 提示, Restore 时额外 confirm |
| **`ERR_IMPORTPOLICY_BSL_AUTH`** | error | poll 时 BSL List 调用 401/403 | BSL credentials 失效 / IAM policy 变更 | 修 BSL credentials secret |
| **`ERR_IMPORTPOLICY_BSL_NOT_FOUND`** | error | `spec.sourceBSL` 引用的 BSL 不存在 | BSL 被删 / 名字打错 | 改 spec.sourceBSL 或重建 BSL |
| **`ERR_IMPORTPOLICY_CRON_INVALID`** | error (拒 CR) | `spec.scheduled.cron` 格式错误 | 客户输错 cron 表达式 | webhook 拦在 admission 阶段, 给 UI 实时验证提示 |
| **`ERR_IMPORTPOLICY_POLL_INTERVAL_INVALID`** | error (拒 CR) | `spec.continuous.pollInterval` 非枚举值 | 客户改 YAML 手动写 1s | webhook 拦, UI 下拉锁死枚举 |
| **`ERR_IMPORTPOLICY_DUPLICATE`** | error (拒 CR) | 同 ns 已有引用同 BSL + 同 sourceClusterId 的 Policy | 重复创建 | webhook 拦, 提示已有 Policy name |
| **`ERR_IMPORTPOLICY_RECONCILE_TIMEOUT`** | error | reconcile loop 单次 > 30s（默认 controller-runtime timeout） | BSL List 慢 / 网络抖 | 自动重试; 持续超时 → Failed phase |
| **`AUDIT_FINGERPRINT_DISABLED_BY_USER`** | info (audit) | 用户显式选 `fingerprintMode=disabled` | UI confirm 后 | 审计追溯, 不报错 |
| **`AUDIT_IMPORTPOLICY_PAUSED`** | info (audit) | `spec.paused` 0→1 | 用户点 Pause | 审计追溯 |
| **`AUDIT_IMPORTPOLICY_RUN_ONCE`** | info (audit) | 用户在 UI 触发 Run Once 立即拉一次 | manual trigger | 审计追溯 |

##### 4.5.8 与 PRD-007 §4.4 的关系（明确分层, 不重复设计）

| 层 | 负责文档 | 内容 |
|---|---|---|
| **算法 / 密钥 / 模块设计** | PRD-007 §4.4 | HMAC-SHA256 算法 / shared secret K8s Secret / Writer/Validator/TrustStore/SecretLoader 四件套 / fingerprint JSON schema |
| **用户面 Policy 包装** | **本 PRD §4.5（v2 Phase 2）** | Action Type pill / Continuous & Scheduled 子模式 / Poll interval 枚举 / RPO 公式 + UI / fingerprint enforce/warn/disabled 三档选择 / ImportPolicy CRD spec / 状态机 / 错误码 family |
| **CRD + Controller 工程实现** | **ADR-038（新增, 架构设计.md §9）** | OpenAPI v3 schema / controller-runtime reconciler 结构 / Continuous time.Ticker / Scheduled robfig/cron/v3 / 与 Velero Schedule 协同（owner ref 防写冲突）|

**不在本 PRD 范围**:
- fingerprint 算法升级（v2 BLAKE3-MAC 评估）→ PRD-007 §4.4 v1.x 跟进
- TrustStore UI 管理面（Settings → 信任的源集群列表）→ PRD-007 §4.4 已有, 本 PRD 仅在 Import Policy 错误码触发时 deep-link 跳过去
- Layer 4 跨云 Backup Copy（PRD-007 §4.3）→ 独立路径, Import Policy 只关心同 BSL 的 RP 拉取, 不做跨 BSL 复制

#### 4.6 USER_MANUAL 同步

- USER_MANUAL.md §Policy 章节 vocabulary 全量替换:
  - "L1 备份" / "L1 本地快照" → "Snapshot（本地快照）"
  - "L2 备份" / "L2 本地云端" → "Snapshot + Export（快照导出到对象存储）"
- 加 **迁移说明** 小节: "如果你在 v0.9.x 及之前使用 L1 / L2 词汇, 它们在新版本对应: L1 = Snapshot only, L2 = Snapshot + Export, 数据和行为完全不变, 仅 UI 词汇调整。"
- 截图全部更新（旧截图含 L1/L2 按钮的需重拍）。

### 5. UI / UX

```
┌─ New Policy ────────────────────────────────┐
│ Name:        [my-app-backup]                │
│ Namespace:   [production ▾]                 │
│                                             │
│ Action:      ● Snapshot  (always on)        │
│              hint: Local snapshot, instant, │
│                    short RTO, stays in      │
│                    this cluster             │
│                                             │
│ ☐ Enable Backup via Snapshot Export         │
│   hint: Export snapshot to object storage   │
│         for remote retention / cross-cluster│
│         restore / long-term archive         │
│                                             │
│   (勾选后展开 ↓)                            │
│   Snapshot Export Storage: [aws-s3-east ▾]  │
│   Export Schedule: [Daily ▾] at [02:00]     │
│   Export Retention: [30] days               │
│                                             │
│ Snapshot Schedule:  [Hourly ▾]              │
│ Snapshot Retention: [24] hours              │
│                                             │
│ Labels:    [+ Add label]                    │
│                                             │
│              [Cancel]  [Save Policy]        │
└─────────────────────────────────────────────┘
```

**对照 Kasten K10**（Mars 截图 1780213542977.png 参考）:
- Kasten: "Action: Snapshot" + "☐ Enable Backup via Snapshot Export" + "Export Location: [BSL ▾]"
- SupKube 新 UX: 完全一致

### 6. Out of Scope（明确不做）

- **不动后端数据模型**: Velero Schedule + `supkube.io/dual` annotation 不变。
- **不动 policypair.go controller**: ADR-026 实现完整保留。
- **不动 Policy 行为 / 语义**: 仅 UI 词汇 + 视觉重构, 备份频率 / 保留 / 失败重试等行为不变。
- **不做 Backup Copy（Layer 4）UX 集成**: PRD-007 §4.3 处理跨云 Backup Copy, 不在本 PRD 范围。
- **不做 PRD-008（RP 删除生命周期）相关 UX 调整**: 那是另一条线。
- **不做 Policy 模板 / Wizard / 推荐**: 那是 PRD-003 AI Advisor 范围。
- **不动 Restore 端 UX**: 本 PRD 仅 Policy 编辑面。

### 7. 非功能性要求

- **向后兼容**: 老 Policy YAML（含 `supkube.io/dual=true`）加载到新 UI 必须正确反映为 "Snapshot + Export 勾起 + BSL 正确填"; **不能丢配置**。
- **i18n 完整**: en + zh-CN 双语 100% 覆盖, 不允许 fallback 到 key 名。
- **兼容期 i18n key**: 老 `policy.actionSnapshot = "L1 Snapshot"` 等 key 在 v0.10.x **保留**但不在新 UI 使用; v0.11.x 移除（防止 grep 出还在引用的地方漏改）。
- **a11y**: Snapshot 不可取消的视觉（disabled 但 checked）需 ARIA `aria-disabled="true" aria-checked="true"` + 屏幕阅读器念出 "Snapshot, always enabled"。
- **响应式**: 抽屉在 < 768px 窄屏不破版（Export 展开内容可滚动）。
- **审计**: Policy 创建 / 编辑产生的 Audit log 字段 `policy.export_enabled = true/false` 替代原 `policy.tier = "L1"|"L2"`（兼容期内**两个字段都写**, 方便老查询不挂）。

### 8. 验收标准（DoD, 12 条）

1. Policy 新建抽屉**无 L1 / L2 按钮组**（npm build pass + 视觉验, 截图归档）。
2. 默认 Action = Snapshot **不可取消勾选**（点击 checkbox 无反应 + 视觉 disabled-but-checked 样式 + ARIA 正确）。
3. "Enable Backup via Snapshot Export" 勾选后下方展开 BSL 下拉 + Export Schedule + Export Retention; 取消勾选则**完全收起**（不留空白占位）。
4. BSL 下拉**仅显示 cloud / external BSL**, 过滤掉 `local-store`（Snapshot 已在用）。
5. 老 L2 Policy 在新 UI 加载后**正确显示**为 "Snapshot + Export 勾起 + 原 cloud BSL 自动填" → 截图对比验证。
6. 老 L1 Policy 在新 UI 加载后**正确显示**为 "Snapshot only, Export 未勾"。
7. Policies 列表的 "L1" / "L2" badge **全部替换**为 "Snapshot only" / "Snapshot + Export"（en + zh）。
8. USER_MANUAL §Policy 词汇**全量替换**, 含截图更新 + 迁移说明小节。
9. i18n en + zh-CN **完整覆盖**（grep 验证无 key fallback 出现）。
10. 后端 API 契约 + Velero Schedule 字段 + `supkube.io/dual` annotation **零变更**（regression test pass: 创建一个新 Policy, kubectl get schedule 输出与 v0.9.x 完全一致）。
11. 兼容期 i18n key（`policy.actionSnapshot = "L1 Snapshot"` 等）**保留**在 locale 文件但**不被新 UI 引用**（grep 验证）。
12. Audit log 字段 `policy.export_enabled` + `policy.tier`（兼容期双写）正确出现, 文档同步。

> **2026-06-01 PRD-Review G5 修订**：上 12 条是 **Phase 1（词汇重构）DoD**。Phase 2 引入 ImportPolicy CRD/controller/fingerprint, **完整规范在 §4.5**，**独立 Phase 2 DoD 在下方 §8.2**。

### 8.2 Phase 2 验收标准（DoD, 14 条 — 对应 §4.5 ImportPolicy 规范）

> **2026-06-01 PRD-Review 第六份 G5 finding 闭环补**：原 §8 没覆盖 Phase 2 ImportPolicy CRD/controller/fingerprint 三档/状态机/错误码。这一段补齐, 每条可通过 e2e test 或 kubectl 实测验证。

**CRD + Schema**:

13. **ImportPolicy CRD 已注册**（`importpolicies.supkube.io/v1`），`kubectl get crd importpolicies.supkube.io` 返回 + `kubectl explain importpolicy.spec` 列出全部 §4.5.5 字段。
14. **OpenAPI v3 schema 严格校验**：错配的 spec.continuous.pollInterval（如 `10s`/`abc`）+ spec.scheduled.cron（非 5 字段 cron）+ spec.fingerprintMode（非 enum 值）→ `kubectl apply` 直接拒（不进 controller, validation webhook 层就 reject）+ error code 准确（见 §16）。

**Controller 行为**:

15. **Continuous 模式实际跑**：建一个 `continuous.pollInterval=60s` 的 ImportPolicy → controller 每 60s 触发一次 syncOnce（看 controller log + `status.lastPollAt` 时间戳推进）+ 没人为操作 12h 不漂时间（无 drift）。
16. **Scheduled 模式实际跑**：建 `scheduled.cron="*/5 * * * *"` 的 ImportPolicy → 5 min 边界时刻 ±2s 内 `status.lastPollAt` 更新（看 kubectl get -w）。
17. **状态机所有合法转换都覆盖**：Pending→Active（首次成功 sync）/ Active→Paused（spec.paused=true）/ Paused→Active（spec.paused=false） / Active→Failed（连续 5 次 poll 失败, N=5, 与 §4.5.6 状态机一致）/ Failed→Active（重新成功）。每条转换 controller log 有明确日志行。

**fingerprint 三档行为**:

18. **enforce 模式**：源 cluster 新建 backup → fingerprint 写入 BSL `.supkube-fingerprint.json` → target ImportPolicy controller 拉取 → HMAC 验签通过 → 创建 Velero Backup CR 含 `supkube.io/fingerprint-status=valid` label。**篡改 fingerprint json 任意字段**（mc 命令改 tarballSHA256） → controller log 显示 `ERR_FINGERPRINT_INVALID`（验签失败，与 §4.5.7 错误码表一致）+ Backup CR **不创建** + `status.rejectedCount +1`。
19. **warn 模式**：源 cluster 缺 fingerprint json（删掉文件）→ target controller 仍创建 Backup CR 但 label `supkube.io/fingerprint-status=missing` + UI Restore Points 行 chip 显示 "⚠ 未签名"。
20. **disabled 模式**：完全跳过 fingerprint 校验 → 100% 接受 source backup → Backup CR label `supkube.io/fingerprint-status=disabled` + Settings 有 audit `AUDIT_FINGERPRINT_DISABLED_BY_USER` 记录 + UI 警告 chip "🚨 已关闭校验"。

**RPO + UI**:

21. **UI 实时显示 worst-case RPO**：Policies 行新建 ImportPolicy 时，下方提示 "🛡 Worst-case RPO ≤ {source_backup_interval + import_poll_interval} min"，数字随 user input 即时更新。
22. **跨集群真实 sync**：docker-desktop 或 aks-dev 建 Snapshot Policy 备份 ns demo-app 到共享 BSL → AKS-test 建 ImportPolicy mode=Continuous interval=60s → 60s+ε 内 AKS-test `kubectl get backup -n velero` 看到该 backup 名 + Imported chip。

**错误码 + 可观测**:

23. **14 条错误码完整**：`ERR_FINGERPRINT_MISSING` / `_TAMPERED` / `_HMAC_INVALID` / `_REQUIRED` + `ERR_IMPORTPOLICY_BSL_NOTFOUND` / `_CRON_INVALID` / `_INTERVAL_TOO_SHORT` / 等共 14 条 全部有 e2e test 触发（见 测试用例.md §9.5 TC-IMP-001~004 + 拓展）。
24. **`status.lastError` 字段**：失败原因写入 ImportPolicy status, kubectl get/describe 看得到, UI 列表行红色 indicator + tooltip 显示。
25. **GC 行为**：`paused=true` 时 controller 不删已 Imported 的 Backup CR, 仅停止新 poll（防误删客户已恢复的 RP）。
26. **Drift detection**：observed cluster ID 跟 spec.sourceClusterFilter.clusterId 不符 → `status.lastError = "source cluster ID drift"` + 跳过该次 sync（防止恶意 swap source cluster）。

### 9. 任务拆分

#### 9.1 Phase 1（v1 词汇重构, ~3.5d, **2026-06-01 已 ship**）

| Phase | 内容 | 估时 | 依赖 | 实际 |
|---|---|---|---|---|
| **P1.1** | Policies.vue 新建抽屉重构（移除 L1/L2 按钮组, 实现 Snapshot 固定 + Enable Export toggle + BSL 下拉过滤）+ i18n（en + zh-CN）+ a11y | 2d | 无 | ✅ 2026-06-01 task #156 |
| **P1.2** | Policy 列表 badge 替换 + USER_MANUAL §Policy 词汇替换 + 截图更新 + 迁移说明 | 1d | P1.1 | ✅ task #156 |
| **P1.3** | 老 L1 / L2 Policy 加载兼容测试 + Audit log 双写验证 | 0.5d | P1.1+P1.2 | ✅ task #156 |
| **Phase 1 总计** | | **~3.5d** ✅ | | **已 ship v0.9.1.12-alpha** |

#### 9.2 Phase 2（ImportPolicy CRD + controller + fingerprint, ~5-6d, **2026-06-01 进行中**）

> **2026-06-01 PRD-Review G5 修订**：原 §9 没列 Phase 2 任务, 这一段补齐。Phase 2 工程量比 Phase 1 大 5x, 不再是"0 迁移风险" — 见 §9.3 风险评级。

| Phase | 内容 | 估时 | 依赖 | 实际 |
|---|---|---|---|---|
| **P2.1** | **Backend ImportPolicy CRD types + controller**（`internal/importpolicy/` 8 文件 + controller-runtime reconciler + Continuous time.Ticker + Scheduled robfig/cron/v3 + REST handlers 8 个 + 单测 ≥70%）| 1.5d | Phase 1 | ✅ Agent B 2026-06-01 task #159 |
| **P2.2** | **Backend fingerprint 模块**（`internal/fingerprint/` 6 文件 — Writer/Validator/TrustStore/HMAC/SecretLoader + 41/41 单测）| 1d | P2.1 共 interface | ✅ Agent C 2026-06-01 task #160 |
| **P2.3** | **Backend 直 S3 BackupLister**（替代 Velero sync 依赖, 真正 30s/60s RPO）| 0.5d | P2.2 | ✅ Agent G 2026-06-01 task #157 |
| **P2.4** | **Frontend Policies.vue Action Type pill + Import Policy 表单** + Continuous/Scheduled 模式选 + RPO 实时计算 + fingerprintMode 三档 + i18n 30+ key | 1d | P2.1 REST 契约 | ✅ Agent D 2026-06-01 task #161 |
| **P2.5** | **Helm chart**：CRD `importpolicies.yaml` + `supkube-fingerprint-secret` 模板 + RBAC verbs + values.yaml 添加 `fingerprint.sharedSecret` + 调 Velero backupSyncPeriod 60s | 0.5d | P2.1 schema | ✅ Agent E 2026-06-01 task #162 |
| **P2.6** | **测试用例.md §9.5 TC-IMP-001~004 + USER_MANUAL.md §24 Import Policy 章 + ROADMAP #88 ✅** | 0.5d | P2.1~P2.5 | ✅ Agent F 2026-06-01 task #163 |
| **P2.7** | **Phase 2 真跨集群 verify**（TC-IMP-001 实跑：docker-desktop 建 SnapshotPolicy + AKS 建 ImportPolicy + 60s 跨集群验 fingerprint chip 显示）| 0.5d | P2.1~P2.6 | ⏳ 等 ADR-042 CD#2 deploy-dev 修好后, 真验在 aks-dev+aks-test 两集群 |
| **Phase 2 总计** | | **~5.5d** | | **6/7 已 ship; P2.7 等 deploy-dev 通** |

#### 9.3 Phase 2 风险评级（2026-06-01 PRD-Review G5 修订补）

> **重要修订**：Phase 1 标"0 迁移风险"，**Phase 2 不再 0 风险**。引入 CRD + controller + Secret + Helm 模板 + RBAC + 新错误码体系 = 多个工程层面的复杂度增加。

| 维度 | Phase 1 | Phase 2 | 评级 |
|---|---|---|---|
| **数据模型变更** | 0（仅 i18n + UI 改） | 新 CRD `importpolicies.supkube.io/v1` + 新 Secret + 新 ConfigMap (`supkube-fingerprint-truststore`) | 🟡 Med — 升级期老集群无 ImportPolicy CR，旧 Velero sync 继续兜底，无 brownfield broken |
| **后端 controller 复杂度** | 0（无后端改） | 新 reconciler + 跨集群 BSL 读 + HMAC 签名/验签 + 状态机 5 个 transition | 🟡 Med — controller 跑在 backend pod, 单点故障重启即恢复 |
| **跨集群信任契约** | N/A | `supkube-fingerprint-secret` 必须双集群同值, 失同步 = enforce 模式所有 import 拒收 | 🔴 High — 客户运维不当会断 import 链路 |
| **客户操作风险** | 0（升级即看到新 UI, 无 break） | 客户需 `helm install --set fingerprint.sharedSecret=...` 跨集群同步 | 🟡 Med — USER_MANUAL §24 + helm NOTES 已说明 |
| **回退路径** | i18n revert | helm uninstall importpolicies CRD（已写 keep policy, 不会清 secret）→ 退回 Velero 默认 backupSyncPeriod 60s sync | 🟢 Low — 单向 forward but 退路存在 |
| **新错误码引入** | 0 | 14 条 `ERR_FINGERPRINT_*` + `ERR_IMPORTPOLICY_*`（沿 ADR-035 风格）| 🟢 Low — 跟现有错误码体系一致 |

**整体 Phase 2 风险等级**：🟡 **Med**（不 High——因 fingerprint enforce/warn/disabled 三档让客户能渐进信任 + Velero 默认 sync 退后兜底 + helm chart `keep` policy 防 secret 误删）

**Mitigations 已采取**:
- 文档化（USER_MANUAL §24 + helm NOTES + 测试用例 §9.5 TC-IMP-001~004 含 sharedSecret 三态对照）
- fingerprintMode `warn` 默认（不立即 enforce, 给客户观察期）
- helm Secret `keep` annotation（uninstall 不删客户的 sharedSecret）
- Velero backupSyncPeriod 维持 60s（ImportPolicy 死了不阻断老路径）

### 10. 关联文档与任务

- **ADR-025**（dual schedule pair, Snapshot/Export 双 Schedule 实现）—— 不动, 复用
- **ADR-026**（policypair controller）—— 不动, 复用
- **task #149**（新立, 本 PRD 实施）
- 现有代码: `supkube-frontend/src/views/Policies.vue` L321-L342（L1/L2 按钮组, 待移除）, `internal/controller/policypair.go`（后端 controller, 不动）
- **PRD-007 §4.3** Backup Copy 跟 Snapshot Export 关系: Snapshot Export = 单云 BSL 写入（本 PRD 范围）; Backup Copy = 跨第 2 云 BSL 复制（PRD-007 Layer 4 范围）, 两者**不冲突**, Backup Copy 建立在 Snapshot Export 之上

### 11. 开放问题（Q1-Q5）

| # | 问题 | 倾向答案 |
|---|---|---|
| Q1 | Kasten K10 文档里**原话**是 "Snapshot Export" 还是 "Export to Cloud" / "Backup to Object Storage"？我们 i18n 用哪个表述？ | **倾向 "Snapshot Export"**（与 Kasten Policy Wizard 实际 toggle 文案一致, 见 Mars 截图 1780213542977.png）; 待 Mars 确认或查 Kasten 官方文档原文 |
| Q2 | 老 `policy.actionSnapshot = "L1 Snapshot"` 等 i18n key 保留多久？ | **倾向 1 个版本（v0.10.x 保留, v0.11.x 移除）**——避免拖太久 i18n 文件膨胀, 也给老调用方一个 sprint 改完 |
| Q3 | Policy 列表 badge 长度: "Snapshot + Export" 在窄屏（< 768px）较长, 是否截短为 "Snap+Exp" 或 "S+E"？ | **倾向 < 768px 显示 "Snap+Exp"**, hover tooltip 显示全名; PC 端始终全名 |
| Q4 | Action = Snapshot **真的"固定不可取消"** 吗？还是允许 "Metadata-only" 模式（不抓任何数据快照, 只导出 K8s 资源 YAML, 走 Export 路径）？ | **v1 倾向固定不可取消**（保持与 Kasten 一致, 也避免边界情况）; Metadata-only 是单独 Feature, 留 v1.x 评估 |
| Q5 | 反向兼容期内（v0.10.x）如何避免新老 vocabulary 在 UI 同时出现？例如旧 toast 模板还在弹 "L2 备份创建成功" 但新抽屉显示 "Snapshot + Export"？ | **审查所有 toast / 通知 / Activity 模板**, 把字面量 "L1" / "L2" 全 grep 替换; CI 加 lint 规则禁止新代码引用 `policy.actionSnapshot` 老 key |

### 12. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Mars (testing20260531.md 第 4 条) | — | 提出 Kasten 对齐需求: Policy UX 应改为 "Snapshot 默认 + Enable Backup via Snapshot Export 开关 + 选 BSL", 替代当前自创的 L1 / L2 互斥按钮组; 附 3 张截图（当前 SupKube UI + Kasten 参考 × 2）|
| 2026-05-31 | Claude | — → **草稿** | 起草 PRD-009 v1。**核心决策**: (1) 仅 UI 词汇 + 视觉重构, 后端数据模型 / Velero Schedule / policypair controller / `supkube.io/dual` annotation **零变更**（0 迁移风险）; (2) 兼容期保留老 i18n key 1 个版本 v0.10.x, v0.11.x 移除; (3) Audit log 字段 `policy.export_enabled` + `policy.tier` 双写; (4) 默认 Action = Snapshot 不可取消, "Metadata-only" 模式留 v1.x。Q1-Q5 待 Mars 评审拍板。待 Mars 评审决定是否 → 排队评审。|
| 2026-05-31 | PRD-Review 第四轮（INDEX.md） | 草稿 → **基本通过（低风险）** | 4 finding: E1 Snapshot 语义澄清 CSI vs MinIO (Med) / E2 无快照集群 always-on 退化路径 (Med, 可能 High) / E3 贴 Kasten 官方措辞 "Enable Backups via Snapshot Exports" 复数 (Low) / E4 跨 PRD-007 L1-L5 术语映射 (Med)。结论: 方向通过, 4 finding 不阻断 v1 ship, 可在 v1.x 渐进闭环。|
| 2026-06-01 | Claude (task #156) | 基本通过 → **v1 Phase 1 已实施** | Mars 客户阻断要求"快速落地"。**已 ship 内容**: (1) Policies.vue create drawer 移除 L1/L2 pill 按钮组, 替换为 Kasten K10 模型 — 顶部 locked pill "📸 快照（CSI）· 始终启用" + 下方 el-switch "启用通过快照导出做备份（Enable Backups via Snapshot Exports）"; (2) i18n key 保留 `actionSnapshot`/`actionSnapshotExport`（代码 ref 不破）, 仅 value 改; 新增 `snapshotSectionTitle` / `snapshotAlwaysOnPill` / `enableExportLabel` / `enableExportHelp` 等 8 个 key (en+zh-CN); (3) 贴 Kasten 官方复数措辞 (E3 闭环); (4) snapshotOnlyWarn / dataPathHelp / dataPathSection 全部去 L1/L2 字眼; (5) Data Path 两列保留视觉, 改注释 "Export OFF/ON" 替代 L1/L2; (6) 后端 `createForm.export.enabled` boolean 0 改动 → ADR-025 dual-schedule + policypair controller 0 影响, audit `policy.export_enabled` 字段自然继承。**未 ship Phase 2 留尾**: E1 Snapshot CSI tooltip (有 dataPathTip.csi 现成的简单 tooltip, 但未明确"vs MinIO"); E2 无 VSC 集群 always-on 退化路径 (现有 capability check 触发警告, 但 Snapshot 仍强制 on); E4 与 PRD-007 Posture 卡 L1-L5 术语映射表 (INDEX.md 标 🔴 open, 等"层↔动作"统一映射敲定)。**实证 verify**: npm run build 通过, frontend image 推 ACR + 双集群 rollout 收敛, 新 chunk `index--yAMMqg0.js` 双集群一致, chunk grep 含新 EN/中文 label 各 1 处, 旧 "L1 Local"/"L1 本地" 字符串 grep = 0, index.html cache-busting 3 meta 在位。**Phase 1 状态**: 已等待 Mars 用户验收。|
| 2026-06-01 (later today) | Claude (Agent A, task #157-163 立项) | v1 Phase 1 已实施 → **v2 修正中（扩 scope 进 Phase 2）** | **v1.0 漏读 Mars 真实需求"Snapshot 与 Import 并列"**——昨天 ship PRD-009 v1 时仅做 Snapshot Policy 单一 Action 的 Kasten 词汇对齐, 漏了 Mars 一开始就明确的 "Policy 顶部应该有 Action Type 二选一: Snapshot Policy + Import Policy" 这条核心需求; 同时任务 #88（Export/Import 配对模型 + fingerprint, PRD-007 §4.4 设计完整但一直 pending 没动）应在本 PRD 升 P0 落用户面。**v2 修正**: (1) **新增 §4.0 Action 类型**——Policy 顶部 pill group 二选一 Snapshot Policy vs Import Policy + 决策树 + UI mockup + Kasten K10 概念对齐; (2) **新增 §4.5 Import Policy 完整规范**——Continuous/Scheduled 双子模式 (30s/60s/2m/5m 枚举 + cron preset+custom) + RPO 公式 + UI 实时显示 + 卖点对比表 (Kasten 5min floor vs SupKube 30s floor + BSL API 成本 trade-off) + fingerprint enforce/warn/disabled 三档 + ImportPolicy CRD spec 完整字段 + 状态机 (Pending→Active⇄Paused / Active→Failed→Active) + 错误码 family (`ERR_FINGERPRINT_*` + `ERR_IMPORTPOLICY_*` 共 14 条) + 与 PRD-007 §4.4 关系分层说明; (3) **原 §4.5 USER_MANUAL 同步 → 改 §4.6**（无内容变更, 仅编号让位）; (4) **§头表更新**: 任务 +#88 升 P0 + #157-163; 关联 ADR + ADR-038; 关联 PRD + PRD-007 §4.4; 目标版本 v0.9.1.13; 反向兼容澄清 "60s Velero `backupSyncPeriod` 调 5min 退后兜底, 主路径走 ImportPolicy controller"; (5) **顶部索引表 PRD-009 行更新**: 状态 "v1 Phase 2 进行中 (2026-06-01)", 任务 "#149 / #156 / #157-163"。**待评审**: ADR-038 草稿同步 (架构设计.md §9), INDEX.md 已加 PRD-009 v2 评审待跟进 + 术语轴注记 ImportPolicy 加入。|
| 2026-06-02 | Claude (Auto 5h, task #165) | v2 修正中 → **v2 改正中（G1-G5 全闭环）** | PRD-Review 第六份 2026-06-01-PRD009v2-011-012.md 5 finding 全部闭环: **G5 (High)** Phase 2 DoD 14 条 §8.2 + Phase 2 任务 7 阶段 §9 + 风险评级独立写 §9.3 (不再"0 迁移风险"); **G1 (Med)** 卖点对比表 §4.5.3 撤回"10x RPO"过度承诺, 改诚实表述"RPO 公式由源备份周期主导, 默认 60s/2m, 30s 仅亚分钟级源备份场景"; **G2 (Med)** 头表反向兼容行明确 `backupSyncPeriod` 保持默认 60s（不调长避免无 ImportPolicy 的 BSL 退化是行为 regression）, Agent G v0.9.1.13.1 直 S3 BackupLister 与 Velero sync 并行 race-safe (IsAlreadyExists 跳过); **G3 (Low)** §4.5.4 warn 模式行加注 "fingerprint 缺失也意味着 RP 可能未完成 — 源备份完成才写 fp, warn 失保护, hover tooltip 标 '该 RP 可能是源未完成半成品' + 列表行加来源 backup phase 显示"; **G4 (Low)** §4 Action Type pill 加 inline alert "保存后不可更改, 选错需删除重建" + Save 首次 confirm dialog + 编辑现有 Policy 时 pill 灰化只读。**状态 → 改正中**, 等 Mars 重审 → 排队评审 → 研发中。 |

---

<a id="prd-010"></a>
## PRD-010 — DR Topology v2（Cluster/BSL 视觉重构 + Local Snapshot + Backup Copy 节点显示）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-010 |
| **任务编号** | #150（新立） |
| **状态** | **研发中**（2026-06-03 Mars 评审通过, 直接进研发；F1-F4 闭环 + 正文回填 M-6/M-7）|
| **作者** | Claude / Mars |
| **目标版本** | v0.10.x（前端 SVG 重构 + 后端 aggregator 小幅扩字段, 不动持久化）|
| **关联 ADR** | ADR-031（5 层韧性模型, 节点 source-of-truth）/ 拟 **ADR-040**（DR Topology SVG 视觉规范, 6 色系 + Layer 徽章 + 数据流箭头分类）← 让号 2026-06-01：ADR-038 已分配给 PRD-009 v2 ImportPolicy CRD（PRD-Review 第六份要求让号） |
| **关联 PRD** | PRD-007 §4.3（Backup Copy → SVG Layer 4 新节点 + 实线/虚线箭头）/ PRD-008 §Activity 持久化（节点红框失败状态联动 Activity Task tooltip）/ PRD-009（Policy "Snapshot + Export" vocabulary 对齐, 节点标签同步）|
| **关联文档** | `supkube-frontend/src/components/DRTopology.vue`（681 行, 自研 SVG）/ `supkube-backend/internal/api/v1/dashboard_topology.go`（aggregator, 需新增 layer4/5 节点字段）/ USER_MANUAL.md §Dashboard（截图待重拍）|
| **反向兼容** | 后端 aggregator JSON 新增 `localSnapshots` / `backupCopies` / `virtualLabs` 三段数组（旧前端不读则忽略）; 现有 `bsls` 数组结构不动; **0 兼容风险** |

> **立项缘由（2026-05-31）**：Mars 在 testing20260531.md 第 1 条提出 3 件事:
> - **(a) cluster name 显示**: 当前 Cluster 卡片显示 `"This Cluster"` 不直观, 应改成真实 control-plane node 名（如 `docker-desktop`）→ **今晚 Agent O 已 ship `clusters.go:buildThisClusterDTO` 改造**, DisplayName 走 control-plane node label, 此部分已落地, 本 PRD **不重做**, 仅在 §4.5 备注关联;
> - **(b) Cluster card 跟 BSL card 颜色相近难区分**: verify 当前 SVG, Cluster card `.dr-card-bg-local` 用浅紫 `#f5f3ff` + 紫色 stroke `#c4b5fd`, BSL cloud 用紫 `#8b5cf6` → **紫色撞色**, 客户演示时分不清 "这个紫色块到底是 cluster 还是 cloud BSL";
> - **(c) 缺少 Local Snapshot / Snapshot Export / Backup Copy 路径的视觉表达**: 当前 SVG 只画 Cluster + BSL 二元节点, 没有显示 ADR-031 5 层模型里的 Layer 1（本地快照）/ Layer 4（跨云 Backup Copy）/ Layer 5（虚拟实验室）, 客户看 Topology **看不出 3-2-1-1-0 谁有谁没**。
>
> 本 PRD 解决 (b) + (c), (a) 已 ship 不复做。

### 1. Goal

DR Topology SVG 从 **"Cluster + BSL 二元节点 + 紫色撞色 + 缺 Backup Copy / Local Snapshot 视觉"** 重构为对齐 **ADR-031 5 层模型** 的清晰可视化: **5 类节点各自独立色系（蓝/青绿/橙/紫/粉/灰）+ 复制/导出关系用方向箭头（5 类）+ Layer 1-5 标识徽章**, 客户一眼看清 **"数据从哪流到哪 / 5 层覆盖度如何"**, 让 DR Topology 成为 SupKube vs Kasten/Veeam 在 **可视化护城河** 上的差异化卖点。

### 2. Epic

**"客户运维零摩擦自救"** Epic —— DR Topology 是 SupKube 跟 Kasten/Veeam/Trilio 拉开差距的**可视化护城河**: 别家是表格 + 文字配置, SupKube 把 ADR-031 的 5 层韧性定位**产品化呈现**为一张 SVG, 客户截图直接当合规材料 / Demo 素材 / 内部审计依据。本 PRD 是该 Epic 在 Dashboard 主视图上的核心交付。

### 3. User Stories

- 作为 **Demo 演示员**（销售 / Pre-sales / SE）, 我打开 Dashboard **一眼**看到 5 类节点（Cluster / Local Snapshot / Local BSL / Cloud BSL / Backup Copy）+ 节点间数据流箭头, 客户指着任一节点问 "这个紫色块是 cluster 还是 storage?" 我能 **秒答**（颜色规范明确, 紫色只属于 Cloud BSL, Cluster 是蓝色, 不可能混淆）。
- 作为 **运维**, 我打开 Topology 第一时间能识别: **Local Snapshot（Layer 1）有/没**（青绿色节点存在 = 有, 灰禁占位 = 没）; **Backup Copy（Layer 4）是不是已配**（粉色节点存在 = 配了跨云复制, 灰禁 = 没）; **任一层灰禁** = 该层未启用, 点击灰禁节点上 "+ 启用 Layer N" 按钮直接跳 Policy 编辑预填该层配置。
- 作为 **客户审计 / Compliance Officer**, Topology 截图直接当 **3-2-1-1-0 合规材料**（"3 份数据 / 2 种介质 / 1 份异地 / 1 份不可变 / 0 验证失败"逐项可视化）, 不需要再做 PPT。
- 作为 **新客户首次安装**, Topology 显示我只启用了 L1 + L2（Local Snapshot + Local BSL）, L3 / L4 / L5 全灰禁; Posture score 显示 40/100; 我点 L3 灰禁的 "+ 启用 Cloud BSL" 直接进 Policy 配置, **不需要看文档自己摸索**。
- 作为 **从 Kasten 迁来的客户**, 我熟悉 "5 层数据韧性" 的概念（Kasten Concept Doc 里有, 但 Kasten 没把它做成一张图）; SupKube 用一张 SVG 表达, 我 **立刻认同 SupKube 更直观**。

### 4. Functions

#### 4.1 5 类节点 + 色系（核心重构, P0）

重做 `DRTopology.vue` SVG 节点分类, 从当前 **2 类（Cluster card + BSL card 含 local/cloud 两子类）** 重构为 **6 类（5 启用类 + 1 未启用占位）**, 各自独立色系:

| 节点类型 | 颜色（light bg / stroke） | 颜色（dark bg / stroke） | 图标 | Layer | 当前状态 |
|---|---|---|---|---|---|
| **Cluster card**（本/远） | 蓝 `#dbeafe` / `#3b82f6` | `#1e3a5f` / `#60a5fa` | 集群 icon（K8s logo 或方块矩阵）| host | **改色**（原浅紫 `#f5f3ff` → 蓝, 解决撞色）|
| **Local Snapshot**（Layer 1）| 青绿 `#d1fae5` / `#10b981` | `#064e3b` / `#34d399` | 相机 icon `📷` | **L1** | **新增节点** |
| **Local BSL**（Layer 2）| 橙 `#fed7aa` / `#f97316` | `#7c2d12` / `#fb923c` | 桶 icon `📦`（local 变体）| **L2** | **改色**（原蓝 `#3b82f6` → 橙, 与 Cluster 蓝区分）|
| **Cloud BSL**（Layer 3）| 紫 `#e9d5ff` / `#a855f7` | `#581c87` / `#c084fc` | 云桶 icon `☁` | **L3** | **改色**（原 `#8b5cf6` → `#a855f7`, 与 Cluster 老紫脱钩, 紫色现在**独属** Cloud BSL）|
| **Backup Copy target**（Layer 4）| 粉 `#fce7f3` / `#ec4899` | `#831843` / `#f472b6` | 复制桶 icon `📋` | **L4** | **新增节点**（与 PRD-007 §4.3 联动）|
| **虚拟实验室**（Layer 5）| 灰 `#e5e7eb` / `#6b7280` | `#374151` / `#9ca3af` | flask icon `🧪` | **L5** | ~~新增节点~~ **已改"验证徽章"**（§13 F4：L5 不画独立节点，改 SVG 顶部 DR Drill 验证徽章，4 状态）|
| **未启用层占位** | 浅灰 `#f9fafb` 灰禁 + 虚线边框 | `#1f2937` 灰禁 + 虚线边框 | `+` 占位 | — | **新增样式** |

**紫色撞色根治原理**:
- 原 Cluster card stroke 是浅紫 `#c4b5fd`, BSL cloud 是紫 `#8b5cf6`, 视觉上属同色系。
- 新方案: **Cluster 用蓝色家族**（与 K8s 官方 logo 蓝色调一致, 客户认知一致）, **Cloud BSL 独占紫色家族**, **Local BSL 改橙色**（与 Cluster 蓝分开）, 三者不撞色。
- 加上 Layer 1 青绿 + Layer 4 粉 + Layer 5 灰, 6 类色彩**全互斥**。

#### 4.2 数据流箭头（5 类, P0）

> ⚠ **箭头分类以 §13 F2 为准（2026-06-02 修订）**：权威 5 类 = `flows[].type ∈ {snapshot, export, import, copy, restore}`（snapshot 实线蓝 / export 实线紫 / import 虚线粉 / copy 虚线橙 / restore 实线弧绿）。**下表的"Local BSL → Cloud BSL / Sync 🔄"一类已废弃**（不再单列 sync）；**缺失的 import（Cloud BSL → Cluster）见 §13 F2**。下表保留作物理路径参考，类型口径以 §13 F2 + aggregator `flows[].type` enum 为准。

节点间用 SVG `<path>` + `<marker>` 箭头表达数据流动方向 + 类型:

| 起点 → 终点 | 箭头样式 | 图标（路径中段标）| 业务含义 |
|---|---|---|---|
| Cluster → Local Snapshot | **实线**（粗 2px）+ 实心三角箭头 | Camera 📷 | 本地 CSI snapshot, 数据不离开集群 |
| Cluster → Local BSL | **实线**（粗 2px）+ 实心三角箭头 | Upload ⬆ | 集群备份写入本地 MinIO / NFS BSL |
| Local BSL → Cloud BSL | **虚线**（dash 4 2）+ 空心三角箭头 | Sync 🔄 | mover 同步本地 BSL → 云 BSL（异步）|
| Cloud BSL → Backup Copy | **虚线**（dash 4 2）+ 空心三角箭头 | Copy 📋（rclone）| Layer 4 跨第 2 云 BSL 复制 |
| Cluster ← 虚拟实验室 | **单向短弧**（虚线, 反向）| Restore ♻ | Verified Restore, 数据从 BSL 拉到虚拟实验室验证, 不影响生产 |

**实线 vs 虚线语义**:
- 实线 = **同步关键路径**（写入即生效, 失败 = 备份失败）
- 虚线 = **异步副作用**（失败不阻塞主备份, 但 Posture score 扣分）

#### 4.3 Layer 徽章（P0）

每节点**右上角**渲染一个小徽章（圆角矩形, 6×16px）, 显示 `"L1"` / `"L2"` / `"L3"` / `"L4"` / `"L5"`, 对应 ADR-031 5 层编号。徽章背景色 = 该节点 stroke 色（视觉一致）, 文字色 = 白色。

**Hover 行为**: 鼠标悬停徽章, 显示 tooltip:
- **L1**: "本地快照 / Local Snapshot — CSI volumesnapshot, 数据不离开集群, 适合短 RTO（分钟级）"
- **L2**: "本地对象存储 / Local Object Storage — in-cluster MinIO 或 NFS, 集群级隔离, 跨节点容灾"
- **L3**: "云端对象存储 / Cloud Object Storage — Azure Blob / AWS S3 / 阿里 OSS, 跨地域容灾"
- **L4**: "Backup Copy / 跨第 2 云复制 — 3-2-1-1-0 的 '2 种介质 / 异地' 实现, 防云厂商单点失败"
- **L5**: "Virtual Lab / 验证恢复 — DR Drill 自动定期拉备份做 sandbox restore, 验证 RP 真的能用"

#### 4.4 状态显示（节点底部 chip, P0）

每节点底部用 1-3 行 chip 显示运行时状态:

- **Local Snapshot**: `"3 RP, 1.2 GiB"` / `"Last snap 5 min ago"`
- **Local BSL**: `"12 RP, 8.4 GiB"` / `"Encrypted ✓"`
- **Cloud BSL**: `"30 RP, 240 GiB"` / `"Object Lock ✓"` / `"Last sync 5 min ago"`
- **Backup Copy**: `"30 RP, 240 GiB"` / `"Cross-cloud ✓"` / `"Encrypted ✓"`
- **Cluster**: `"docker-desktop"` / `"k8s v1.30.1"` / `"3 nodes"`
- **Virtual Lab**: `"Last drill 7 days ago, ✓ PASSED"` 或 `"未启用"`

**失败状态**:
- 红色边框（stroke `#dc2626` 3px）+ ⚠ icon 角标 + tooltip `"上次复制失败, 见 Activity Task <id>"`（点击跳转 Activity 详情, 与 PRD-008 联动）

**实时刷新**: 节点状态每 5 秒轮询 `/api/dashboard/topology` 更新（不重绘整个 SVG, 仅更新 chip 文本 + 边框样式）。

#### 4.5 Cluster name 显示（**已 ship**, 仅备注关联）

- 后端 `clusters.go:buildThisClusterDTO` 在 v0.9.x 已改造（**今晚 Agent O ship**）, `DisplayName` 字段从硬编码 `"This Cluster"` 改为读取 control-plane node 的 `name` label（如 `docker-desktop` / `master-0`）。
- 本 PRD 前端 SVG **直接消费** `cluster.displayName` 字段, **不重做后端**。
- **客户覆写路径**: 客户可在 helm install 时填 `cluster.displayName=my-prod-cluster`, 覆盖 node name（适合多个 `master-0` 集群避免重名）→ **本 PRD 不实现 helm value, 留 PRD-013（MCM Dashboard, 待立）**, 仅在 USER_MANUAL 注明 "v0.10.x 显示 control-plane node 名, 自定义命名待后续版本"。

#### 4.6 未启用层占位 + "启用" 按钮（P1）

5 层节点 **始终全部渲染**（即使未启用）, 未启用的层用 **浅灰 + 虚线边框 + 半透明（opacity 0.5）+ 中央 "+ 启用 Layer N" 按钮**:

| Layer | 未启用占位文案 | 点击 "+ 启用" 跳转目标 |
|---|---|---|
| L1 Local Snapshot | "+ 启用 Layer 1 本地快照" | Policy 编辑预填 `action=snapshot, bsl=local-store`（注: Layer 1 是 ADR-028 自管层, 若该 ADR 还没实施, **跳到 USER_MANUAL §Layer1 文档**, 见 Q2）|
| L2 Local BSL | "+ 启用 Layer 2 本地 BSL" | BSL 创建抽屉预填 `kind=local`|
| L3 Cloud BSL | "+ 启用 Layer 3 云端 BSL" | BSL 创建抽屉预填 `kind=cloud`|
| L4 Backup Copy | "+ 启用 Layer 4 跨云复制" | Backup Copy 配置抽屉（PRD-007 §4.3）|
| L5 Virtual Lab | "+ 启用 Layer 5 DR 演练" | DR Drill 配置抽屉（PRD-007 §4.7）或文档（见 Q3）|

这是 PRD-007 **"5 层覆盖度 Posture score"** 的可视化对偶。

> ⚠ **已被 §13 F1 修订（2026-06-02）——以 §13 为准**：**Posture score ≠ 已启用层数 × 20**。该公式是误导（把"层数计数"当成分数）。真分数由 **PRD-007 §4.7 单一加权规则**算（按 3-2-1-1-0 各层**实际贡献**加权），003 / 007 / 010 三处**共用同一个后端 score（PRD-011 evaluator）**，本 PRD 只**消费并可视化**这个分数，不自己算。下文 §4.7 的"×20"示例仅作占位说明，实际以加权 score 为准。

#### 4.7 Posture 总分显示（P1）

SVG 底部固定一条 status bar:
```
Posture: 80/100   (L1✓ L2✓ L3✓ L4✓ L5✗)
```
- 分数 = **PRD-007 §4.7 单一加权 score**（按 3-2-1-1-0 各层实际贡献加权，**非**"已启用层数 × 20"；见 §4.6 上方 §13 F1 修订说明）。上例 `80/100` 仅为占位示意。
- 5 个 layer 状态用 ✓ / ✗ 简洁标记
- 点击分数跳转 **PRD-007 §Posture detail 页**（详细评分依据 + 改进建议）

#### 4.8 后端 aggregator 字段扩展（P0）

`dashboard_topology.go` aggregator 返回 JSON 新增字段:
```json
{
  "cluster": { "displayName": "docker-desktop", ... },
  "bsls": [ ... ],                          // 现有, 不动
  "localSnapshots": [                       // 新增
    { "name": "snap-xxx", "rpCount": 3, "sizeBytes": 1234567, "lastSnapAt": "..." }
  ],
  "backupCopies": [                         // 新增 (PRD-007 §4.3 联动)
    { "name": "copy-xxx", "targetBSL": "aws-s3-east", "rpCount": 30, "lastCopyAt": "...", "encrypted": true, "objectLock": true }
  ],
  "virtualLabs": [                          // 新增 (PRD-007 §4.7 联动)
    { "name": "drill-xxx", "lastDrillAt": "...", "passed": true }
  ],
  "posture": { "score": 80, "layers": { "L1": true, "L2": true, "L3": true, "L4": true, "L5": false } }
}
```
- 旧前端不读新字段则忽略 → **向后兼容**
- 新字段为空数组时, 前端渲染对应 layer 占位（未启用样式）

### 5. UI / UX

```
┌─ DR Topology ────────────────────────────────────────────────┐
│                                                              │
│      ┌─[L1]──────┐       ┌─[L2]───────┐                     │
│      │ 📷 Local  │ ──┐   │ 📦 MinIO   │                     │
│      │ Snapshot  │   │   │ (local)    │                     │
│      │ 3 RP      │   │   │ 12 RP      │                     │
│      └───────────┘   │   └────────────┘                     │
│                      │         │                            │
│  ┌────────────┐     │         ↓ sync                       │
│  │ 🌐 Cluster │ ────┘  ┌─[L3]────────┐                     │
│  │ docker-    │ ────── │ ☁ Azure    │                      │
│  │ desktop    │        │ Blob (cloud)│                     │
│  └────────────┘        │ 30 RP       │                     │
│                        └─────────────┘                      │
│                              │                              │
│                              ↓ copy (rclone)                │
│                      ┌─[L4]──────────┐                      │
│                      │ 📋 AWS S3     │                      │
│                      │ (2nd cloud)   │                      │
│                      │ 30 RP, locked │                      │
│                      └───────────────┘                      │
│                                                              │
│   [L5 Virtual Lab]  ┌──────────────┐                        │
│   (灰禁占位)        │ + 启用 DR    │                       │
│                     │   Drill 演练 │                       │
│                     └──────────────┘                        │
│                                                              │
│ Posture: 80/100  (L1✓ L2✓ L3✓ L4✓ L5✗)                    │
└──────────────────────────────────────────────────────────────┘
```

**视觉规范要点**:
- Cluster card 居左中, 蓝色, 是 "数据源", 所有箭头从它出发
- L1 Local Snapshot 在 Cluster 右上, 青绿色, **最近**（数据不离开 cluster）
- L2 Local BSL 在 Cluster 右上偏右, 橙色, 实线箭头
- L3 Cloud BSL 在 L2 右下, 紫色, **虚线** sync 箭头从 L2 来
- L4 Backup Copy 在 L3 正下方, 粉色, **虚线** copy 箭头从 L3 来
- ~~L5 Virtual Lab 在右下角独立位, 灰色~~ **（§13 F4 修订：L5 不再画独立节点，改 SVG 顶部"验证徽章"；本 mockup 为旧版示意，以 §13 F4 为准）**
- 失败节点红框 + ⚠ 角标, hover 显示 "见 Activity Task #xxx"（跳 PRD-008）

### 6. Out of Scope（明确不做）

- **不重做 SVG 引擎**: 沿用现有 `DRTopology.vue` 自研 SVG（681 行）, 仅改 **节点 + 色系 + 加 Layer 4/5 节点 + 加 Layer 徽章**, 不引入 d3 / vis.js / cytoscape 等图库。
- **不做拖拽编辑**: read-only 视图, 节点位置由算法固定布局, 用户不能拖。
- **不做多集群 topology**: 本 PRD 单 cluster, 多 cluster 切换 / 全展开留 PRD-013 / MCM Dashboard（待立）。
- **不做 SVG 导出 PNG / PDF**: 留 v2.1（客户暂可用浏览器截图）。
- **不做节点详情侧抽屉**: 节点 hover 显示 tooltip 即可, 点击不弹抽屉（避免遮挡 Topology 全貌）; 详情看节点 chip + 跳 Activity / Policy 详情页。
- **不做实时 WebSocket 推送**: 5s 轮询足够（节点状态 chip 不要求 sub-second 刷新）, WebSocket 留 v2.x（性能瓶颈出现再升级）。
- **不动 cluster name 显示后端逻辑**: §4.5 已 ship, 不重做。
- **不做 helm `cluster.displayName` value**: 留 PRD-013（MCM Dashboard, 待立）。

### 7. 非功能性要求

- **性能**: SVG render < 200ms（节点上限 = 1 cluster + 1 L1 snapshot + 10 BSL + 5 backup copy + 3 virtual lab ≈ 20 节点 + 30 箭头 / `requestAnimationFrame` 优化）
- **a11y**: 5 类节点都加 `<title>` + `<desc>` SVG 元素 + ARIA `role="img" aria-label="<节点类型> <名称> <状态>"`; 键盘 Tab 可遍历节点, Enter 触发 hover tooltip
- **i18n**: 节点标签 + Layer 徽章 tooltip + chip 文本 + Posture 文字 **en + zh-CN 双语 100% 覆盖**, 不允许 fallback 到 key
- **暗色模式**: 6 类节点色系全部给 dark token（见 §4.1 表格 "颜色 dark" 列）, 切换 dark mode 时 SVG 整体重绘（不只是反色, 用专门 dark 配色保证对比度）
- **响应式**: < 1024px 屏幕 SVG 缩放 80%, < 768px 移动端**折叠为列表视图**（5 层各一行, 不强求 SVG）
- **审计**: Topology 渲染本身无审计需求（read-only）, "+ 启用 Layer N" 按钮点击不直接执行变更（跳 Policy 抽屉, 那边走 Policy 创建审计）
- **回归**: 现有 BSL 节点视觉变化对老用户可能造成一时困惑, 需在 v0.10.x release note 加 "DR Topology 视觉重构" 说明

### 8. 验收标准（DoD, 12 条）

1. **5 类节点各独立颜色**, 紫色不撞（Cluster 蓝 / Cloud BSL 紫 / Local BSL 橙 / Local Snapshot 青绿 / Backup Copy 粉 / Virtual Lab 灰, 视觉验 + 截图归档）。
2. **Layer 1-5 徽章** 正确映射 ADR-031（每节点右上角 L1/L2/L3/L4/L5 标识, hover tooltip 显示该 layer 业务含义）。
3. **未启用层显示占位**（浅灰 + 虚线 + opacity 0.5）+ **"+ 启用 Layer N" 按钮** 跳 Policy / BSL / Backup Copy / DR Drill 创建抽屉（5 种 layer 各验一遍）。
4. **Cluster name** = control-plane node 名（如 `docker-desktop`）, 与 `clusters.go:buildThisClusterDTO` 一致, 不再显示 `"This Cluster"`。
5. **数据流箭头 5 类** 区分（§13 F2 口径：snapshot 实线 / export 实线 / **import 虚线** / copy 虚线 / restore 单向弧；**不再有 sync 类**）+ 中段图标（Camera / Upload / Import / Copy / Restore）。
6. **状态 chip 实时刷新**: 每 5 秒轮询 `/api/dashboard/topology`, RP 数 / Last sync 时间正确更新, 失败节点红框 + ⚠ 角标显示。
7. **暗色模式色系完整**: 6 类节点 dark token 全部生效, 切换 dark mode 无视觉降级。
8. **i18n en + zh-CN**: 节点标签 + 徽章 tooltip + chip + Posture 双语 100% 覆盖（grep 验证无 key fallback）。
9. **后端 aggregator** `/api/dashboard/topology` JSON 新增 `localSnapshots` / `backupCopies` / `virtualLabs` / `posture` 字段, 旧前端不读不挂（向后兼容验证）。
10. **失败节点联动 PRD-008**: 节点红框 tooltip 点击跳转 Activity Task 详情页（`/activity/<id>`）。
11. **a11y**: 5 类节点 SVG `<title>` + ARIA 完整, Tab 键可遍历, 屏幕阅读器念出节点类型 + 名称 + 状态。
12. **`npm run build` pass** + **`go build ./...` pass** + 现有 BSL 节点回归测试通过（不能因为色系改造导致老节点显示错位）。

### 9. 任务拆分（3 phase, 估时 ~4.5 人日）

| Phase | 内容 | 估时 | 依赖 |
|---|---|---|---|
| **Phase 1** | 5 类节点色系重构（DRTopology.vue 改 `.dr-card-bg-*` CSS class + `bsl.kind` 分支扩展 6 类 + Layer 徽章 SVG 组件 + tooltip）+ 后端 aggregator 扩字段（`localSnapshots` / `posture`）| **2d** | 无 |
| **Phase 2** | Backup Copy 节点（PRD-007 §4.3 联动, 复用 Phase 1 节点组件 + 粉色 token）+ Virtual Lab 占位节点（灰色）+ 后端 `backupCopies` / `virtualLabs` 字段（暂返回空数组, 待 PRD-007 实施填充）| **1.5d** | Phase 1 |
| **Phase 3** | 状态 chip 实时刷新（5s polling）+ 失败节点红框 + ARIA + 暗色模式 6 色系 token + i18n en+zh-CN + npm build + go build | **1d** | Phase 1+2 |
| **总计** | | **~4.5d**（中等改造, 不涉及 SVG 引擎替换 / 后端持久化模型）| |

### 10. 关联文档与任务

- **ADR-031**（5 层韧性模型, 节点 source-of-truth）—— SVG 节点分类的理论依据
- **拟 ADR-040**（DR Topology SVG 视觉规范, 6 色系 + Layer 徽章 + 数据流箭头分类）—— 本 PRD 落地后产出, 沉淀视觉规范防止后续 ad-hoc 改色（**2026-06-01 让号**：原占 ADR-038 已被 PRD-009 v2 ImportPolicy CRD 取用，本 PRD 改占 040；台账见 架构设计.md §ADR 号台账）
- **PRD-007 §4.3** Backup Copy —— Layer 4 节点数据源
- **PRD-007 §4.7** DR Drill —— Layer 5 节点数据源
- **PRD-008** Activity 持久化 —— 节点失败红框 tooltip 跳转 Activity Task 详情
- **PRD-009** Policy "Snapshot + Export" vocabulary —— 节点 chip 文案对齐（如 `"Snapshot Export → cloud BSL"`）
- **task #150**（新立, 本 PRD 实施）
- **现有代码**:
  - `supkube-frontend/src/components/DRTopology.vue` L167（`.dr-card-bg-local` class 条件）, L179（kind 图标三元）, L441（BSL 颜色三元 `#3b82f6` / `#8b5cf6`）, L585（CSS `dr-card-bg-local` 定义 `#f5f3ff` / `#c4b5fd`）—— **本 PRD 全部重构**
  - `supkube-backend/internal/api/v1/dashboard_topology.go`（aggregator, 待 grep 确认精确路径; 本 PRD 扩字段, 不改持久化）
  - `supkube-backend/internal/api/v1/clusters.go:buildThisClusterDTO`（cluster name 改造**今晚已 ship**, 不动）

### 11. 开放问题（Q1-Q5）

| # | 问题 | 倾向答案 |
|---|---|---|
| Q1 | 5 类颜色是按本 PRD §4.1 表格定义还是给 designer 重选？ | **倾向本 PRD 方案直接拍板**——紫色撞色问题已规避（Cluster 蓝 / Cloud BSL 紫 / Local BSL 橙互不撞）, Layer 1 青绿 + Layer 4 粉 + Layer 5 灰也都是 K8s / 数据保护行业可读色系。Designer 介入留给 ADR-040 落地时统一审查全局 design token。|
| Q2 | 未启用层 "+ 启用" 按钮是否真跳 Policy 编辑？若 Layer 1（本地快照, ADR-028 自管层）还没实施, 怎么跳？ | **倾向 L1 占位按钮 v0.10.x 跳 USER_MANUAL §Layer1 文档**（"本地快照需要 CSI snapshot 支持, 见文档配置"）, L2/L3/L4/L5 跳实际配置抽屉。ADR-028 实施后 L1 也跳 Policy 抽屉（v0.11.x）。|
| Q3 | Layer 5 虚拟实验室是 placeholder 还是真接 DR Drill（PRD-007 §4.7）？ | **倾向 v0.10.x placeholder**（节点存在 + 状态 chip 始终显示 "未启用 / 查看文档"）, PRD-007 §4.7 落地后（预计 v0.11.x+）真接 DR Drill。否则本 PRD 等 PRD-007 串行依赖, 排期拉长。|
| Q4 | 多 cluster 模式（MCM 切换器）怎么显示？切换显示还是全展开？ | **本 PRD 单 cluster, 不解决多 cluster**。需为 PRD-013 / MCM Dashboard（待立）留接口: 后端 aggregator JSON 顶层 `cluster` 字段未来扩为 `clusters[]` 数组, 前端能 map 多个 cluster 各自一份 5 层节点。本 PRD 不预实现, 仅在代码注释里加 TODO。|
| Q5 | 状态 chip 信息密度上限？5+ layer × 5+ chip 会拥挤, 是否折叠 hover？ | **倾向每节点最多显示 2 行 chip**（最关键 2 项, 如 Local BSL 只显示 "12 RP" + "Encrypted ✓"）, 其余 hover tooltip 全展开。Chip 设计需 Phase 1 评审时确认 "最关键 2 项" 是哪 2 项。|

### 12. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-05-31 | Mars（testing20260531.md 第 1 条）| — | 提出 3 件: (a) Cluster card 显示 "This Cluster" 不直观, 应改 control-plane node 名; (b) Cluster card 跟 BSL card 紫色撞色难区分; (c) 缺 Local Snapshot / Snapshot Export / Backup Copy 路径的视觉表达。(a) 部分今晚 Agent O 已 ship `clusters.go:buildThisClusterDTO` 改造（DisplayName 走 control-plane node label）, (b)+(c) 需立 PRD 重构。|
| 2026-05-31 | Claude | — → **草稿** | 起草 PRD-010 v1。**核心决策**: (1) 6 类节点色系彻底解决紫色撞色（Cluster 蓝 / Cloud BSL 紫 / Local BSL 橙 / L1 青绿 / L4 粉 / L5 灰）; (2) ADR-031 5 层模型可视化对偶 —— 5 类节点 + Layer 1-5 徽章 + 未启用层占位 + Posture 总分; (3) 数据流箭头 5 类区分（snapshot/export 实线 + sync/copy 虚线 + restore 单向弧）; (4) 沿用现有 SVG 引擎（681 行 DRTopology.vue）不引入图库; (5) 后端 aggregator JSON 扩字段 `localSnapshots` / `backupCopies` / `virtualLabs` / `posture`, 向后兼容 0 风险; (6) cluster name 改造已 ship 本 PRD 不重做; (7) 多 cluster 留 PRD-013（MCM Dashboard, 待立）; (8) Q1-Q5 待 Mars 评审拍板（重点 Q1 色系是否需 designer 介入、Q2 L1 占位按钮跳哪里、Q3 Layer 5 是否真接 DR Drill）。待 Mars 评审决定是否 → 排队评审。|
| 2026-06-02 | Claude (PRD-Review 第六份 F1-F4 finding 闭环) | 草稿 → **改正中** | 加 §13 PRD-Review 第六份 F1-F4 finding 修订段：**F1** Posture 分数消费后端 PRD-007 §4.7 单一权威 score 不再 "层数×20" 误导 (跨 003/007/010 共用一个真按 3-2-1-1-0 加权的 score, 不是层数计数) / **F2** 数据流箭头按 ADR-025 真实 dual schedule 渲染 (snapshot 实线 / export 实线 / 不再混色) / **F3** L1-L5 vs PRD-009 v2 Snapshot/Export 术语映射表 (每个 Layer 节点 hover tooltip 显示对应 Policy Action Type) / **F4** Layer 5 改为 "DR Drill 验证徽章" 而非独立节点 (语义自洽: Layer 5 是周期性 verify 行为, 不是 Layer 1-4 的物理实体节点)。§8 DoD 加 #13-#16 四条新验收点。状态草稿→改正中, 可转研发中。|

### 13. PRD-Review 第六份 F1-F4 finding 修订段（2026-06-02 闭环）

> **修订动因**：PRD-Review-2026-05-31-PRD010.md（PRD-Review 第六份 §五）对 PRD-010 给出 4 个 finding（F1-F4）。本段是**独立闭环段**。

#### F1（Med-High）Posture 分数第三套口径 — "层数×20" 误导

**finding 原文**：PRD-010 §4.7 把 Posture 分数算成"已启用层数 × 20"（5 层全启用=100）。但 PRD-003 应用韧性分（5 维加权）/ PRD-007 §4.7 集群 Posture 层覆盖分（按 3-2-1-1-0 实际贡献加权）已经存在两套定义；PRD-010 又造第三套 = **跨 PRD 分数语义打架**。"层数×20" 误导客户：只装本地快照（Layer 1）就 20 分？只 import 不 export（Layer 4）就 80 分？这违反 3-2-1-1-0 的安全直觉。

**拟方案**：PRD-010 **不另算 score**，**直接消费 PRD-007 §4.7 单一权威 score**（HTTP GET /api/v1/dashboard/posture → 单一分数 source-of-truth）：
- 前端 DRTopology.vue **只渲染**后端给的 `posture.totalScore` + `posture.breakdown[]` 数组
- 不在前端做任何"层数计数 × 20" 计算
- PRD-007 §4.7 锁定单一加权规则（按 3 copies / 2 media / 1 offsite / 1 immutable / 0 errors 各自的实际贡献加权）
- PRD-003 应用韧性分跟 PRD-010 集群 Posture 分**是两个独立指标**（per-app vs per-cluster）但**共享同一后端规则引擎**（PRD-011 evaluator.go SSOT）

**与 PRD-007 P5 同源**：本 finding 跟 PRD-007 P5 "Score 口径厘清" 是同一 issue 的不同切面。**两 PRD 共同闭环**：PRD-007 §4.7 定权重表 + PRD-010 §4.7 引用 PRD-007（不另算）。

**DoD 入 §8**：#13。

#### F2（Med）数据流箭头与 ADR-025 双 Schedule 不符

**finding 原文**：PRD-010 §4.5 把"snapshot/export"画成同色实线，但 ADR-025 实际是 **dual schedule pair**——Snapshot 走 CSI snapshot 类型 Schedule（写本地 CSI driver）/ Export 走 file-system 类型 Schedule（写 BSL 对象存储）。两个 Schedule 物理路径不同，UI 应该**区分着色**。

**拟方案**：

| 数据流路径 | ADR-025 来源 | DRTopology 渲染 |
|---|---|---|
| Snapshot Schedule（CSI snapshot type, dual=false 半 / 单 ns 单存储路径） | Cluster → Local Snapshot 节点 | 实线 #3b82f6 蓝色 + "Snapshot" label |
| Export Schedule（file-system type, dual=true 半 / 同 ns 上传 BSL） | Cluster → Cloud BSL 节点 | 实线 #8b5cf6 紫色 + "Export" label |
| Import Policy（PRD-009 v2 §4.5 跨集群） | Cloud BSL → Cluster | 虚线 #ec4899 粉色 + "Import" label |
| Backup Copy（PRD-007 Layer 4） | Cloud BSL → 第 2 Cloud BSL | 虚线 #f59e0b 橙色 + "Copy" label |
| Restore（按需触发） | BSL → Cluster | 实线弧 #10b981 绿色 + "Restore" label |

后端 aggregator JSON 加 `flows[].type` 字段（enum: snapshot/export/import/copy/restore），前端按类型映射颜色 + 实线/虚线。

**DoD 入 §8**：#14。

#### F3（Med）L1-L5 vs PRD-009 v2 Snapshot/Export 术语映射

**finding 原文**：PRD-009 v2 把 Policy 词汇从 "L1/L2" 改成 "Snapshot Policy / Import Policy"，但 PRD-010 DRTopology 仍用 "Layer 1-5" 徽章 + "L1 占位按钮"。同一产品**两套词汇并行**让客户困惑：Policy 面是 Snapshot/Export，Dashboard 面是 L1-L5。

**拟方案**：**两套词汇并存 + 映射表显式**（不消灭一套, 因为 5 层 3-2-1-1-0 模型跟 Policy Action Type 是两个正交概念, 共存合理）：

| 5 层（3-2-1-1-0 韧性模型, PRD-007 + PRD-010 用）| 对应 Policy Action Type（PRD-009 v2 用） | 节点 hover tooltip |
|---|---|---|
| **Layer 1** 本地快照 (CSI snapshot) | Snapshot Policy（无 Export toggle） | "本层由 Snapshot Policy 提供 — 点击跳转新建 Snapshot Policy" |
| **Layer 2** 同集群另一存储介质 | Snapshot Policy（Enable Backups via Snapshot Exports = local BSL） | "本层 = Snapshot + Export 到 local BSL" |
| **Layer 3** 异地副本 | Snapshot Policy（Enable Backups via Snapshot Exports = cloud BSL）| "本层 = Snapshot + Export 到 cloud BSL（异地）" |
| **Layer 4** 第 2 异地副本（Backup Copy） | Snapshot Policy + Backup Copy（PRD-007 Layer 4） | "本层 = Layer 3 + 跨第 2 云 Backup Copy" |
| **Layer 5** DR Drill（演练验证） | 跟 Policy 无关（验证行为，不是 Policy）| "本层是周期性 DR Drill 验证 — 见 PRD-007 §4.7" |

每个 Layer 节点 hover tooltip 显式映射 + 点击节点跳对应 Policy 创建抽屉（已有 PRD-009 v2 Action Type 双 pill）。

**DoD 入 §8**：#15。

#### F4（Low-Med）Layer 5 作为节点语义不自洽

**finding 原文**：Layer 1-4 都是"物理实体"（本地 snapshot / 本地 BSL / 异地 BSL / 第 2 异地 BSL），Layer 5 "虚拟实验室 DR Drill" 是**周期性验证行为**，不是一个**节点**。把它画成跟 Layer 1-4 同等的节点违反语义自洽。

**拟方案**：Layer 5 **不画节点**，改画 **"验证徽章"（Verification Badge）**——展示在整个 DR Topology SVG 顶部 / 边角：

```
┌─────────────────────────────────────────────────────────┐
│  DR Topology                              🛡 Layer 5    │  ← 验证徽章
│                                          ✓ Last drill   │
│  [Cluster]─────[Snapshot]                  6 hours ago  │
│       │                                                  │
│       └──[Local BSL]──[Cloud BSL]──[Copy → 第 2 Cloud]   │
│                                                          │
│  Posture: 4/5  ◆◆◆◆◇                                    │
└─────────────────────────────────────────────────────────┘
```

徽章状态:
- 🛡 ✓ "Last drill 6h ago" (周期内, 绿)
- 🛡 ⚠ "Last drill 8d ago" (周期 7d, 超期, 黄)
- 🛡 ❌ "Never drilled" (从未跑, 红)
- 🛡 — "Not enabled" (PRD-007 §4.7 DR Drill 未启用, 灰)

**DoD 入 §8**：#16。

#### §8 DoD 新增条目（F1-F4 finding 落地, 接在原 #12 之后）

| # | 验收点 | finding |
|---|---|---|
| 13 | **F1 闭环**：前端 DRTopology.vue **不算 score**, 只渲染 `GET /api/v1/dashboard/posture` 返回的 `totalScore` + `breakdown[]`; grep 前端代码确认无 `layerCount * 20` 这种计算; PRD-007 §4.7 锁定单一加权规则; PRD-003 应用韧性分 + PRD-010 集群 Posture 分共享 PRD-011 evaluator.go 规则引擎 |
| 14 | **F2 闭环**：后端 aggregator JSON `flows[]` 数组加 `type` 字段（enum: `snapshot`/`export`/`import`/`copy`/`restore`），前端按 5 类映射颜色 + 线型；ADR-025 dual schedule pair 实际跑出来 SVG 真显示 2 条不同色实线（screenshot 归档） |
| 15 | **F3 闭环**：每个 Layer 节点 hover tooltip 显示 PRD-009 v2 对应 Action Type + "点击跳转" link → 真跳 Policies 抽屉 + Action Type 预选；术语映射表写入 USER_MANUAL §X 单一来源 |
| 16 | **F4 闭环**：Layer 5 不画节点，改画顶部"验证徽章"(4 状态: 周期内✓/超期⚠/从未❌/未启用—); SVG 渲染数据从 `GET /api/v1/dashboard/posture` 的 `lastDrillAt` + `drillEnabled` 字段读 |

---

## PRD-011 — AI Backup Advisor MVP（智能备份顾问 · 规则评分 + LLM 解释 + 双面知识库 · 小闭环）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-011 |
| **任务编号** | #164 |
| **状态** | **研发中**（2026-06-03 Mars 拍 D-WAIT-002 评分数值, 直接进研发；H1/H2/H5 闭环）|
| **作者** | Claude / Mars |
| **目标版本** | v0.11.x |
| **关联 ADR** | ADR-031（5 层数据韧性, 评分维度来源）/ ADR-033（AI Advisor 架构: 一引擎两出口 + sanitize + 非自治）/ **ADR-037（统一数据采集架构: CollectionContract + Collector/Server 分离 + Canonical DSL + 三档连接形态）** |
| **关联 PRD** | PRD-003（AI Advisor inside SupKube, 本 PRD 是其评分内核的工程化落地）/ PRD-004（MCP Server, 复用同一 Advisor Engine）/ PRD-012（Call Home, 复用本 PRD 的 Collector + DSL） |
| **参考决策** | Mars 与外部专家 2026-05-30/31 关于 RAG 维护、数据流水线、SupEye DSL、Call Home 的 3 轮讨论; SupEye Server《数据获取器》文档（fact/observation/inferred/evidence 四段模型来源） |

> **MVP 范围声明（防止范围蔓延）**：本 PRD 只交付**单应用、同步、规则评分 + LLM 解释**的小闭环。**不含**: 异步任务队列、自动学习/在线训练、跨集群聚合评分、B 面知识库的自动标注流水线（B 面 v1 只给"结构化导入入口"）。这些留迭代。

> **三轮讨论收敛的 3 条铁律（写进验收）**：
> - **finding #2 — 分数必须可复现**: Resilience Score 由 **Go 规则引擎计算**（确定性、可单测、同输入同输出）, **LLM 绝不参与打分**, 只负责把分数与扣分项翻译成自然语言解释。
> - **finding #3 — 置信度三档不用百分比**: 所有推断（inferred）的置信度只用 `high / medium / low`, 不输出"87%"这种伪精确数字。
> - **数据缺失要显式建模**: 采集不到的字段不能"装作是 0", 必须用状态枚举 `confirmed | optional | missing | unable_to_confirm | source_conflict` 标注, 评分时按"无法确认"路径处理并在解释里说明。

### 1. Goal

把 PRD-003 描述的"内嵌灾备顾问"中**最核心、最容易被质疑**的一环——**Resilience Score（应用韧性评分）**——做成一个**研发可独立开发、测试可独立验证、产品可拿去评审**的工程化闭环。目标: 对单个应用（K8s namespace / workload 组），软件**自己采集**上下文 → **规则引擎**算出 0-100 分 + 风险等级 → **LLM 解释**为什么是这个分、扣在哪、怎么改 → 所有结论可追溯到证据。**5 秒内**给客户一个资深灾备 SRE 的判断, 且**判断可复现、可解释、可质疑**。

### 2. Epic

ADR-031"全职灾备运维专家" Epic 的**评分内核**。PRD-003 定义了产品形态（三大场景 + UI 出口 + 非自治原则）, 本 PRD 把其中"Resilience Score 怎么来的"从一句话变成一套**确定性算法 + 数据契约 + 知识库**, 让这个评分**经得起客户问"凭什么"**。

### 3. User Stories

#### 3.1 自助采集（对接 ADR-037 Collector）
- 作为系统管理员, 我在 Applications 页对某个 ns 点 **"分析韧性"**, 软件**无需我填任何东西**, 自己通过只读 K8s API + Velero CR 采集 5 个模块的上下文（工作负载 / 存储 / 备份策略 / 备份历史 / 安全配置）, 30 秒内出结果。
- 作为 AirGap 客户管理员, 即使我的集群**完全离线**, 这个分析也能在**本地**跑完（本地 Ollama + 本地规则引擎 + 本地知识库）, **0 出网**。

#### 3.2 看分数与理由
- 作为系统管理员, 我看到该应用的 **Resilience Score = 62 / MEDIUM RISK**, 下面是 5 维度分解条形图（Business Value / Architecture / Protection / Security / Operation）, 每个扣分项一行: **"-15 未配置 PITR / 数据库类应用建议开启 Point-in-Time Recovery（证据: 检测到 StatefulSet `mysql` + PVC, 但 backup schedule 无 `--snapshot-volumes` 与 WAL 归档）"**。
- 作为系统管理员, 每条扣分/建议旁有 **"为什么"** → 展开看到引用的知识库条目 ID + 采集到的原始事实 + 推导链 + 置信度（high/medium/low）。

#### 3.3 应用建议（非自治, 对接 PRD-003）
- 作为系统管理员, 我点某条建议的 **"应用建议"** → 跳到对应 Policy Wizard 并**预填**参数 → 我确认/修改 → 保存。**软件绝不替我自动改任何东西**。

#### 3.4 本地知识库覆盖（B 面 v1）
- 作为有内部规范的客户管理员, 我在 Settings 里**导入**我司的备份规范（结构化 YAML/JSON）作为"本地覆盖知识", 评分解释会引用它; 但它**绝不混入** SupKube 出厂知识（A 面）, 两个平面物理隔离、来源可标。

### 4. Functions

#### 4.1 数据采集（5 模块, 落为 ADR-037 的 `app_context` CollectionContract）

软件按 **CollectionContract `app_context@v1`** 采集以下 5 模块, 每个采集到的字段都封装为 Canonical DSL 的 `fact` 节点 `{value, status, source, evidence_refs}`:

| 模块 | 采集内容 | 来源（只读） |
|---|---|---|
| **M1 工作负载** | Deployment/StatefulSet/DaemonSet 类型、副本数、镜像、annotations/labels、是否数据库特征（StatefulSet + PVC + 已知 DB 镜像名/端口） | K8s API |
| **M2 存储** | PVC 列表、StorageClass、容量、accessMode、CSI driver、是否支持 snapshot（VolumeSnapshotClass 探测） | K8s API |
| **M3 备份策略** | 是否被任一 Velero Schedule 覆盖、retention、是否加密、BSL 数量与类型（local/cloud）、是否跨地域 | Velero CR |
| **M4 备份历史** | 最近 N 次 Backup 成功/失败、最近成功时间、是否做过 Restore Drill（有无 restore 记录） | Velero CR |
| **M5 安全配置** | 是否加密、BSL 凭据是否独立、是否有 RBAC 限制、镜像是否来自可信仓库（启发式） | K8s API + Velero CR |

> **缺失即标注**: 任一字段采集不到, `status` 置 `missing` / `unable_to_confirm`, 不得用 0 或空值冒充"已确认无"。例如探测不到 VolumeSnapshotClass 时, "是否支持 snapshot"= `{value: null, status: unable_to_confirm, source: "k8s", evidence_refs: []}`, 评分按"无法确认快照能力"扣分路径处理并在解释中说明。

#### 4.2 规则评分引擎（确定性 · Go · 可单测）—— **本 PRD 技术核心**

##### 4.2.0 两套评分器业务定位（2026-06-03 Mars 拍板）

SupKube 后端实际并行运行**两套评分体系**, 评估对象、客户阶段、业务目标完全不同, **代码独立, UI 标签区分**:

| 维度 | **Legacy "备份策略"** | **New "策略调优"** (本 §4.2 / ADR-043) |
|---|---|---|
| **评估问题** | "**该不该备份**" + "**怎么备份**" | "**备份做得多好**" + "**哪里短板**" |
| **评估对象** | 应用本身 (workload / PVC / labels) | 应用 + 绑定的 Policy + **Policy 实际执行结果** |
| **客户阶段** | 初次部署 SupKube, 尚未建任何 Policy | 已建 Policy 且**已实际跑过备份**, 想优化 |
| **保险类比** | **投保前**: 该不该买 / 买啥险种 / 保额建议 | **已投保**: 现有保单短板 / 升级建议 / 加保推荐 |
| **输出类型** | Recommendation (推荐): schedule / TTL / tier | Evaluation (评分): 0-100 + 4 档 + 4 维细节 |
| **代码位置** | `internal/api/v1/applications.go scoreNamespaceForAdvisor()` | `internal/advisor/evaluator/v1_0_0.go Score()` |
| **API 端点** | `GET /applications/advisor` (现役) | `POST /ai/score` (PRD-011 MVP 待接) |
| **UI 标签** | "**Backup Recommendation**" / "备份策略" | "**Resilience Score**" / "策略调优" |
| **依赖标准** | 无 (启发式 +40/+30/-30 等) | ISO 27002 §8.13 / NIST CSF / NIST SP 1800-26 / 800-53 CP-9 |

> **两套不冲突 / 互补**: 一个 ns 旧评分 70 (High, "建议你备份") 跟同 ns 新评分 35 (Critical, "数据韧性差") 是**评不同问题**, 不应统一也不应让客户看到时混淆。UI 必须明确标签区分。
>
> **收敛计划**: v0.10.x Phase A (task #115) 写 K8s→AppContext 适配器, 底层共用一份采集; 两个 UI 标签 + 端点不变。本 PRD MVP **v0.9.x 不收敛**, 让两套先共存收集真实使用数据。

##### 4.2.1 评分前置硬性条件 (Mars 拍板 2026-06-03)

新评分器 `evaluator.Score()` 评分前**必须满足全部前置条件**, 否则**不参评**, 输出特殊状态:

| 前置条件 | 不满足时输出 | 前端展示 |
|---|---|---|
| 应用绑定的 Policy **已完成部署** | `Status = not_eligible_no_policy` | "未配置备份 Policy, 请先在 Policies 页创建" |
| Policy **已实际执行运行 ≥ 1 次** (`LastBackupAt != nil`) | `Status = not_eligible_no_runs` | "Policy 未执行, 请先跑首次备份" |

**不参评的行为**:
- evaluator 返回 `ScoreResult { Status: not_eligible_*, NotEligibleReason: "..." }`, **不输出 TotalScore / Level / Dimensions** (留空或 nil)
- 前端**不显示 30 分 + Critical 红框**, 而显示**提示卡**: "建议客户先定义 + 执行 Policy"
- 这跟初版"无备份封顶 30 硬阈值"**语义不同**: 封顶 30 = 评了但低分; not_eligible = **没评 (前置不满足)**

##### 4.2.2 评分矩阵（100 分制 · 4 维 · 标准对标）

✅ **Mars 2026-06-03 拍板（D-WAIT-002，权威源；喂 ADR-043 评分细则 v1.0.0）**。每个子项**分档给分（确定性，可单测）**，对标 ISO 27002 / NIST CSF / NIST SP 1800-26 / NIST SP 800-53：

| 维度 | 权重 | 子项（分档） | 对标 |
|---|---|---|---|
| **1. Backup Coverage & Compliance**（备份覆盖与合规） | **25** | 应用分类覆盖度 10（Tier 分级差异化策略+100%核心纳管=10 / 统一策略未分级或<5%僵尸=5 / 未分级有漏备=0）· **RPO 达标率 10**（满足或优于=10 / **基本满足但窗口过大 (actual ≤ 1.5×target)** =5 / 不满足=0）· 元数据与配置备份 5（含 Manifest/拓扑/配置=5 / 仅纯数据=0）| ISO 27002 §8.13.1.a |
| **2. Resilience & Redundancy**（3-2-1-1-0 韧性） | **35** | **介质与存储多样性 10【3-2】**（≥2 种介质=10 / 单介质=5 / **0 介质=0**（无备份策略, 或策略未生成 RP））· 异地与跨云 10【1·异地】（跨厂商或>100km=10 / 同厂商跨 region=6 / 同 AZ 同源=0）· 空气隔离与网络离线 15【1·隔离/不可变】（真 Air-Gap 或专网备完即断=15 / 有隔离但同网络域/同 AD=8 / 无隔离生产可直挂=0）| NIST CSF (Protect/Respond) |
| **3. Immutability & Security**（防勒索与安全） | **20** | 不可变存储 10（WORM/Object Lock **COMPLIANCE 模式**=10 / Governance 模式或仅 IAM=6 / 可覆盖删除=0）· 加密与凭证 5（静态 AES-256+传输 TLS1.3+KMS 独立轮换=5 / 有加密但密钥同管=2）· **访问控制 5 (graduated 5 档)**（4 项全满足=5 / 3 项=3 / 2 项=1 / ≤1 项=0；4 项=MFA+RBAC+删除二次审批+审计异地不可篡改）| NIST SP 1800-26 · ISO 27002 §8.13.1.c |
| **4. Reliability & Verification**（成功率与可恢复性） | **20** | 备份执行成功率 5（滑动窗口公式，见下）· 自动化恢复演练通过率 15【0·验证】（全自动沙箱恢复+近3月100%=15 / 定期人工演练有报告=10 / 仅局部文件恢复测试=5 / 只备不练=0）| NIST SP 800-53 CP-9 · ISO 27002 §8.13.1.b |

> **3-2-1-1-0 映射**：3-2=维度2 介质多样性；1=维度2 异地；1=维度2 隔离/不可变；0=维度4 恢复演练验证。

##### 4.2.3 3 处边界明确化 (Mars 2026-06-03 拍板)

PRD 初版 §4.2 矩阵在 3 处子项有模糊边界, Mars 已拍数值, 此处焊死:

1. **维度 1 子项 2 RPO "窗口过大"边界 = 1.5× target** (`if actual ≤ target → 10; if actual×2 ≤ target×3 → 5; else → 0`, 整数比较防浮点)
   - 业界 SLO 设计共识 (Google SRE / AWS Well-Architected): 50% 容差是"接近 SLO 但未违反"普遍阈值
   - 1× 太严 (jitter 都不容忍); 2× 太宽 (鼓励虚高)

2. **维度 2 子项 1 介质多样性 0 介质 → 0 分** (从 PRD 初版二档 10/5 升三档 10/5/0)
   - **业务理由 (Mars 原话)**: "因为它也没有建备份策略, 或是备份策略没有生成 RP 那就是 0"
   - 跟"无备份封顶"硬阈值同精神; 0 BSL ≠ 1 BSL, 不能同分

3. **维度 3 子项 3 访问控制 graduated 5/3/1/0** (Option A, 从 PRD 初版二档 5/0 升 4 档)
   - 4 项满足 → **5** (全套合规)
   - 3 项满足 → **3** (大部分到位)
   - 2 项满足 → **1** (一半, 仍有重大单点)
   - ≤1 项满足 → **0** (单点安全控制 = false sense of security)
   - **业务理由 (Mars 原话)**: "让客户知道, 自己要进行配置" — 渐进激励改进

##### 4.2.4 三个落地公式（半自动采集，确定性）

1. **备份成功率滑动窗口**：取最近 14 天 / 最近 30 次快照计 `success/total × 5`；**惩罚**：Tier 1 核心资产**连续失败 > 3 次 → 该项直接计 0**（局部数据断流风险）。
   - **防 panic** (Mars Gate 1 铁律 ①): `attempts == 0` → `unable_to_confirm` (不除零)
2. **不可变 (WORM) 自动校验断言**：调存储 API（如 S3 `GetObjectLockConfiguration`），断言 `ObjectLockEnabled==Enabled` 且 `Mode==COMPLIANCE` 且 `RetainUntilDate > SnapshotAt + 业务安全周期(默认30d)`，三者全真才给 10 分。
3. **最终安全级别分档**（90/75/60 inclusive 上界）：

| 分数 | 安全级别 |
|---|---|
| 90–100 | **极高韧性级**（对标 NIST 勒索防护，分钟级拉起）|
| 75–89 | **合规风险低**（满足 ISO 27001 基本审计，多厂商瘫痪场景有短板）|
| 60–74 | **脆弱级**（无不可变防护，易被勒索连同生产加密）|
| < 60 | **高危级**（伪备份，随时面临永久丢失）|

##### 4.2.5 校准硬阈值

> **Mars 2026-06-03 修正**: 初版的"**无备份封顶 30**" 改为 §4.2.1 的**not_eligible 路径**(不参评), 不再走"封顶 30 + 评分"。配置看似完善但 Policy 从未执行 → evaluator 输出 `Status: not_eligible_no_runs`, UI 提示客户先跑首次备份。

仅保留**高分校准 30**:
- **高分校准 30**：算分 ≥90 但 `app.last_backup_succeeded == false`（最近一次失败）→ 强制下调到 30 + 落"高危级"（配置看似完善但实际备份没落地）。

##### 4.2.6 可复现保证

引擎是纯函数 `Score(ctx AppContext) ScoreResult`, 无随机、无时间依赖（"新鲜度"/滑动窗口用采集快照里的时间戳而非 `time.Now()`）, 同一份 `AppContext` 输入永远得到同一 `ScoreResult`。**单测覆盖每个子项的每一档 + 两条校准硬阈值 + not_eligible 路径**（见 TC-AI-MVP）。每次评分输出带 `scoreRulesVersion`（§12 H1），`internal/advisor/evaluator/v1_0_0.go` 每条规则注释含权重/分档/依据/对标标准/校准来源（Mars 2026-06-03 D-WAIT-002）。

`ScoreResult` schema:
```go
type ScoreResult struct {
    Status            EvaluationStatus  // "scored" / "not_eligible_no_policy" / "not_eligible_no_runs"
    NotEligibleReason string            // 仅 Status != scored 时填
    ScoreRulesVersion string            // 永远 "v1.0.0"
    TotalScore        int               // 仅 Status == scored 时有意义
    Level             SecurityLevel     // 仅 Status == scored 时有意义
    Dimensions        Dimensions        // 仅 Status == scored 时有意义
    CalibrationApplied []string         // 仅 Status == scored 时可能填 (含 "high_score_calibration_30")
    EvaluatedAt       time.Time         // = ctx.SnapshotAt
}
```

#### 4.3 LLM 解释层（inferred · 默认 Ollama · 只解释不打分）

引擎产出 `ScoreResult`（分数 + 维度分解 + 命中的扣分项 ID + 证据引用）后, LLM 把它**翻译成自然语言**, 输出封装为 Canonical DSL 的 `inferred` 节点 `{conclusion, basis, confidence, evidence_refs}`:
- `conclusion` = 给客户看的建议句子;
- `basis` = 引用了哪些 `fact` 节点 + 哪些知识库条目;
- `confidence` = `high / medium / low`（finding #3）;
- `evidence_refs` = 指回原始采集事实 + KB 条目 ID。

**LLM 绝不修改分数**: validator 校验 LLM 输出里若出现与引擎分数不一致的数字, 一律以引擎为准并丢弃 LLM 的数字。**默认 Provider = 本地 Ollama**（0 出网）; DeepSeek/Claude/Azure 等需在 Settings 显式 opt-in 并经 `sanitize.go`（ADR-033 §6 + SECURITY.md §6）脱敏后才出网, 生成 SanitizeReport 审计。

#### 4.4 知识库（RAG · 双面 · chroma-go）

- **A 面（出厂知识, 我们维护）**: knowledge-as-code, git 仓库内 Markdown, 走 PR 评审、版本化、登记 KB-LEDGER。向量库用 **chroma-go**（Mars 拍板, 企业级要上向量库）+ **结构化标签预过滤**（保证可解释: 先按 tag 缩小召回集再向量检索）。本地嵌入模型 `nomic-embed-text`（0 出网）。v1 交付约 30 条（4 类 × 5-10 条: 灾备原理 / 应用特征 / 合规 / 反模式）。
- **B 面（客户本地覆盖, 客户维护）**: v1 只提供**结构化导入入口**（YAML/JSON schema 定义的规范条目）, **不做**自动解析/标注/增强流水线。B 面知识**物理隔离**于独立 collection, 检索时来源标注清晰, **绝不混入 A 面**。

**A/B = 两层正交决策体系（标准基本盘 + 客户决策面）—— ADR-046**（Mars 2026-06-03）。"从权 B>A"**不是** B 覆盖评分引擎，而是**评分层与执行层分治**：

- **A 面（标准基本盘）= 评分 + 盲区检测的权威，永不被覆盖**。Resilience Score 始终按 A 的 NIST/ISO 矩阵（§4.2）算 → **跨客户可横比**。A 还**主动挑刺**：用标准最佳实践识别客户自定义规则的盲区/遗漏/合规偏差（例：客户把 ERP 列最高却漏了邮件/CyberArk 密码库应同级恢复），生成建议报告。
- **B 面（客户决策面）= 执行（DRP/CRP 恢复编排优先级）的权威**。但**不是被动从权**：AI 用 A **枚举待决策项 → 暴露风险差异 → 引导客户显性化决策**；客户**终审签字**后沉淀进**专属决策历史库**（可追溯/可审计/可复盘）。**DRP/CRP 最终以决策库为唯一执行准则**，不再单纯依赖通用标准或静态模板。
- **闭环公式**：标准兜底 / AI 引导 / 客户终审 / 系统落地 / 全程可追溯。
- **非自治照旧（Rule F）**：AI 只枚举+引导+建议，**绝不**强制覆盖客户决策或自动改集群；客户终审是唯一执行依据。

> **可复现不受影响**：评分用 A（comparable），B 治理的是恢复**执行顺序**而非分数——两层正交，故无"跨客户不可横比"问题（修正本 PRD 早前草稿的误述）。
> **范围边界**：完整能力（决策历史库 + 盲区检测报告 + DRP/CRP 编排 + 风险决策框架工具箱 RICE/RPN/FMEA/AHP/TOPSIS/FAIR/OCTAVE… + CMDB/依赖图/向量记忆）是 **Premium 版独占**，且**超出 PRD-011 MVP 范围 → 建议另立 PRD-015（AI 容灾决策顾问）**。PRD-011 MVP 只交付 **A 面评分 + LLM 解释**；B 面 v1 仅"结构化导入入口"。详见 ADR-046（架构设计.md §9）。

#### 4.5 业务梳理（business mapping · 仅 K8s 可推导依赖）

v1 的"业务梳理"**只**呈现**能从 K8s 客观推导**的依赖关系: Service → Endpoints → Pod、Ingress → Service、PVC → Pod、ConfigMap/Secret 引用、StatefulSet 拓扑。**不臆测**业务语义依赖（如"订单服务依赖库存服务"这类需人工确认的）。无法确认的依赖标 `unable_to_confirm`, 不画实线。

#### 4.6 API（评分同步 + 解释异步 SSE）

> ⚠ **已被 §12 H5 修订（2026-06-02）——以 §12 为准**：原单端点 `POST /ai/analyze`（同步 60s 跑完采集+评分+LLM 解释）**已拆为两个**，避免本地 Ollama 解释耗时（10-40s）阻塞首屏：`/ai/score` 仅跑规则引擎、<5s 同步返回 score+evidence+confidence；`/ai/explain` 流式（SSE）返回 LLM 解释。下表已更新；详见 §12 H5。

| Method & Path | 说明 |
|---|---|
| `POST /api/v1/ai/score` | 输入 `{app_id}` → **仅规则引擎**（不调 LLM）→ <5s 同步返回 score + evidence + confidence + `scoreRulesVersion`（§12 H1 可复现） |
| `GET /api/v1/ai/explain/{taskId}` | **SSE 流式**返回 LLM 解释（不阻塞首屏；失败 fallback 轮询 `{status, text}`，§12 H5） |
| `GET /api/v1/ai/result/{app_id}` | 取最近一次分析结果（带 evidence） |
| `GET /api/v1/ai/scores` | 列出所有已分析应用的分数（Dashboard Posture 卡片用） |
| `GET /api/v1/ai/audit` | 推理审计记录（输入/输出/provider/耗时/SanitizeReport） |
| `GET/PUT /api/v1/ai/settings` | LLM provider 切换、出网开关、B 面知识导入 |

### 5. Non-Goals（明确不做）
- ❌ LLM 参与打分（违反 finding #2）。
- ❌ 异步任务队列 / 批量全集群扫描（留迭代）。
- ❌ 自动学习 / 在线训练 / 反馈回灌模型（Mars 明确不要）。
- ❌ 自动执行任何变更（非自治, ADR-033 §6）。
- ❌ B 面自动标注/数据增强流水线（v1 只导入）。
- ❌ 臆测业务语义依赖。

### 6. 度量（成功标准）
- 单应用分析端到端 ≤ 60s（本地 Ollama, 中等集群）。
- 评分引擎单测覆盖率 ≥ 90%, 所有扣分/加分/校准分支有用例。
- 同输入 100 次分析分数完全一致（可复现性测试）。
- AirGap 模式 0 出网（网络抓包验证）。

### 7. 技术方案 / 组件清单（SRE 视角: 要搭什么）

| 组件 | 类型 | 部署形态 | 备注 |
|---|---|---|---|
| Advisor Engine（含 score 引擎 + validator） | Go module, 编入 supkube-backend | 随后端 Pod | 无新增 Pod |
| Collector（ADR-037） | Go module, 只读 K8s/Velero client | 随后端 Pod | 复用现有 informer |
| Ollama | 容器 / sidecar | **默认本地**, AirGap 必备 | 客户可换 BYO API |
| `nomic-embed-text` 嵌入模型 | Ollama model | 本地 | 0 出网 |
| chroma-go 向量库 | 嵌入式 / 本地服务 | 本地持久卷 | A/B 面各一 collection |
| A 面知识库 | git 仓库 Markdown + 构建期嵌入 | 打包进镜像 | KB-LEDGER 登记 |
| 审计存储 | CR / ConfigMap | 集群内 | 推理记录 |

### 8. DoD（验收标准, 13 条 → 对应 TC-AI-MVP）
1. 5 模块采集全部落为 `fact` 节点, 缺失字段正确标 `missing/unable_to_confirm`。
2. 规则引擎对给定 fixture 输出**精确**分数（逐分支单测）。
3. 同输入多次运行分数**完全一致**（可复现）。
4. 分数→风险等级映射符合固定阈值表。
5. 校准规则生效（高分无备份 → 30/CRITICAL）。
6. LLM 解释输出为合法 `inferred` 节点, confidence ∈ {high,medium,low}。
7. LLM 输出的数字与引擎不一致时被 validator 丢弃/纠正。
8. 默认 Ollama, AirGap 模式网络抓包 0 出网。
9. SaaS provider opt-in 时经 sanitize 且生成 SanitizeReport。
10. A/B 面知识物理隔离, 检索结果来源可标, B 面不污染 A 面。
11. "应用建议"只预填 Wizard, **不写任何集群资源**（apply-no-write）。
12. 每条建议可追溯到 evidence（fact + KB 条目 ID）。
13. 业务梳理只画 K8s 可推导依赖, 不确定依赖标 `unable_to_confirm`。

### 9. 任务拆解（Phase 0-5）
- **Phase 0 — 数据契约**: 定义 `app_context@v1` CollectionContract + Canonical DSL Go 类型（fact/observation/inferred/evidence + status 枚举）。
- **Phase 1 — Collector**: 实现 5 模块只读采集, 缺失标注。
- **Phase 2 — Score Engine**: 纯函数评分引擎 + 全分支单测（**本 PRD 重心**）。
- **Phase 3 — LLM 解释 + validator**: Ollama 默认 + sanitize + SanitizeReport。
- **Phase 4 — RAG 双面**: chroma-go + A 面 30 条 + B 面导入入口。
- **Phase 5 — API + UI 接线**: 5 个 endpoint + Applications Score 列 + "AI 建议" tab + "应用建议"预填。

### 10. 待评审开放问题（Q1-Q5）
- ~~**Q1**: 评分维度满分配比（30/20/30/15/5）是否需要 Mars/灾备专家二次校准?~~ ✅ **已解决（2026-06-03 D-WAIT-002）**：改用 Mars 自定 4 维标准对标矩阵，见 §4.2 + ADR-043。
- **Q2**: B 面导入的结构化 schema 用什么标准（自定义 YAML vs 复用某开源规范格式）?
- **Q3**: chroma-go 嵌入式 vs 独立服务, AirGap 打包体积可接受范围?
- **Q4**: "无备份封顶 30" 与"高分校准 30" 两条硬规则阈值是否合理?
- **Q5**: MVP 同步 60s 超时, 大集群单应用是否够（要不要 Phase 1.5 转异步）?

### 11. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-06-01 | Claude | — → **草稿** | 由 Mars 与外部专家 3 轮讨论（RAG 维护 / 数据流水线 / SupEye DSL / Call Home）收敛而成。**核心决策**: (1) 分数由 Go 规则引擎算, LLM 只解释（finding #2）; (2) 置信度三档不用百分比（finding #3）; (3) 缺失数据用 status 枚举显式建模; (4) 采用 SupEye 四段 DSL（fact/observation/inferred/evidence）; (5) A/B 双面知识库物理隔离, chroma-go + 结构化标签预过滤; (6) 默认 Ollama 0 出网, SaaS opt-in 经 sanitize; (7) 非自治, 应用建议只预填不写。MVP 锁单应用/同步/规则评分小闭环, 自动学习/异步/跨集群留迭代。待 Mars 评审拍板 Q1-Q5。|
| 2026-06-02 | Claude (PRD-Review 第六份 H1/H2/H5 finding 闭环, H1 数值待 Mars 拍) | 草稿 → **改正中** | 加 §12 PRD-Review 第六份 H1/H2/H5 finding 修订段（H3 ADR-037 撞号已修 / H4 chroma-go vs 全量注入留 v1.x 决策不阻断）：**H1** 规则集版本化 (`scoreRulesVersion` 字段输出每次评分 + 历史可复现) + 每条扣分依据写入 evaluator.go 注释 + **Mars 需拍 Q1 权重表 + Q4 硬阈值 (写 等待决策.md D-WAIT-002)** / **H2** 异地判定采 BSL `region` + `provider` 元数据 (不只 local/cloud type 二分), "异地"严格定义=不同 region 且/或不同 provider, 同 region 不同 provider 也算异地半档 / **H5** `/ai/analyze` API 拆 2 endpoints: `/ai/score` 快速同步返回 score+evidence+confidence (<5s, 仅规则引擎跑, 不调 LLM), `/ai/explain?taskId=` 流式返回 LLM 解释 (Server-Sent Events, 不阻塞首屏)。§8 DoD 加 #14-#17 四条新验收点。状态草稿→改正中, 部分 Blocked 待 Mars 拍 H1 数值 + 升级到已评审。|

### 12. PRD-Review 第六份 H1/H2/H5 finding 修订段（2026-06-02 闭环）

> **修订动因**：PRD-Review-2026-06-01-PRD009v2-011-012.md（PRD-Review 第六份 §四）对 PRD-011 给出 5 个 finding（H1-H5）。H3 已闭环（ADR-037 撞号 PRD-011 让号给 PRD-008 → ADR-039，PRD-011 仍用 037），H4 MVP 决策接受全量注入 + 标签预过滤（chroma-go v1.x 评估）。本段闭 H1/H2/H5。

#### H1（Med-High）评分权重需版本化 + 专家校准 + 写明依据

**finding 原文**：评分权重（30/20/30/15/5）与扣分值（-10~-25）是合理但**主观的判断**（Q1 自己在问要不要专家校准）。这套规则正是分数可信度的全部来源，"经得起客户问凭什么"取决于它。

**拟方案（3 件事）**：

1. **规则集版本化**：每次评分输出**带 `scoreRulesVersion: "v1.0.0"`** 字段（语义化版本，破坏性改动 major++ / 加新规则 minor++ / 调阈值 patch++）。
   - 客户拿到 score=85 + scoreRulesVersion=v1.0.0 → 永远可以问"v1.0.0 规则下 85 分怎么算的"
   - 后端 evaluator 包多版本并存：`v1_0_0.go`, `v1_1_0.go`, ... 老分数永远可重算
   - 升级 SupKube 时, dashboard 警示"评分规则从 v1.0.0 升到 v1.1.0, 历史分数下方括号显示新规则下重算结果"

2. **每条扣分依据写入 evaluator.go 注释**：
   ```go
   // CheckBackupCoverage 检查应用是否有 backup policy 覆盖
   // 评分逻辑：
   //   - 没有任何 backup policy → 扣 25 分（封顶 30, 即 CRITICAL 等级）
   //   - 依据：3-2-1-1-0 原则首条 = 3 copies, 0 copies 直接 critical;
   //     Kasten K10 "Protection Status: Unmanaged" 也是类似处理
   //   - 决策出处：PRD-011 §6 度量表 + Mars 与灾备专家 2026-06-XX 校准
   func CheckBackupCoverage(app *AppContext) Finding {
   ```

3. **请 Mars / 灾备专家校准** — ✅ **已完成（2026-06-03 · D-WAIT-002 RESOLVED）**：Mars **否决**了上文的简版 5 维（30/20/30/15/5），亲自重构为 **100 分制 4 维标准对标矩阵**（备份覆盖与合规 25 / 3-2-1-1-0 韧性 35 / 防勒索与安全 20 / 成功率与可恢复性 20，对标 ISO 27002 + NIST CSF/1800-26/800-53；3 落地公式 + 安全级别 4 档 + Q4 两条硬阈值）。**权威定义见 §4.2 + ADR-043**；本节上文的 30/20/30/15/5 仅为被否决的初版，保留作 finding 历史。**PRD-011 已进入研发中**。

**DoD 入 §8**：#14。

#### H2（Med）异地判定需真实 region/provider 元数据

**finding 原文**："3-2-1 满足度" / "跨地域" / "同存储后端单点" 这些扣分**依赖准确判定异地/介质多样性**。M3 只采"BSL 数量与类型（local/cloud）"——两个同区域 cloud BSL 不算异地，"cloud" 也可能同区域。判错会误导分数（DR 面板最忌讳）。

**拟方案**：M3 显式采 BSL `region` + `provider` + `bucket` 三元组（不只 local/cloud type 二分）。"异地"严格定义：

| 场景 | 定义 | 评级 |
|---|---|---|
| 两个 BSL 都 = local on-cluster MinIO | 0 异地（同集群存储）| 🔴 critical for 3-2-1 |
| 两个 BSL = (local MinIO + cloud Azure Blob 同 region eastus) | 1 异地（半档, 不同 provider 但同 region）| 🟡 部分满足 |
| 两个 BSL = (cloud Azure Blob eastus + cloud Azure Blob westus) | 1 异地（半档, 同 provider 但不同 region）| 🟡 部分满足 |
| 两个 BSL = (cloud Azure Blob eastus + cloud AWS S3 us-west-2) | 2 异地（全档, 不同 provider 不同 region）| 🟢 满足 |
| 任何 = on-cluster + cloud 不同 region | 2 异地（全档）| 🟢 满足 |

"同存储后端单点" 严格定义：两个 BSL 同 provider + 同 region + 同 bucket prefix。

**Collector 改造**：`internal/collector/app_context.go` 抓 BSL 时 `region` + `provider` 字段必抓（从 Velero BSL CR `.spec.provider` + `.spec.config.region` 读）；如果客户填的 BSL 没 region 字段 → status=`missing`, 评分时按 worst case 算。

**DoD 入 §8**：#15。

#### H5（Med）LLM 解释别阻塞 60s, 分数先返回

**finding 原文**：`/ai/analyze` 同步 60s 跑完采集+评分+LLM 解释。本地 Ollama 在 CPU 节点生成一段解释常需 10-40s，大 ns 采集+LLM 可能破 60s（Q5 已问转异步）。

**拟方案**：拆 2 endpoints, 分数先返回, LLM 解释异步:

```
POST /api/v1/ai/score
  Request:  { appNamespace: "demo-app" }
  Response: { score: 75, evidence: [...], confidence: "medium", scoreRulesVersion: "v1.0.0",
              explainTaskId: "uuid-1234" }
  Time: < 5s (仅规则引擎, 不调 LLM, evidence 是 collector 抓的 fact/observation)

GET /api/v1/ai/explain/:taskId  (Server-Sent Events stream)
  Stream:
    event: chunk
    data: {"text": "你这个应用"}
    event: chunk
    data: {"text": "的备份覆盖率"}
    ...
    event: done
    data: {"totalTokens": 245, "modelUsed": "ollama:llama3:8b"}
  Time: 10-40s (LLM 流式输出)

UI 行为:
  1. 点 "Analyze" → /ai/score 5s 内返回 → UI 立即显示 score + evidence + confidence
  2. UI 自动 SSE 连 /ai/explain/:taskId → 解释**实时增量**显示在抽屉里 (类似 ChatGPT 打字效果)
  3. 用户随时关掉抽屉, /ai/explain 自动 abort
```

**降级**：如果 SSE 失败 (e.g. proxy 不支持) → fallback 到轮询 `GET /ai/explain/:taskId` 返回 `{ status: "running" | "done", text: "..." }`。

**DoD 入 §8**：#16。

#### H4 + H3 备注（不动）

- **H3 (Info)** ADR-037 撞号在 PRD-Review 第六份建议让号 PRD-008 → ADR-039。**PRD-011 仍占 ADR-037**（统一数据采集架构）。已闭环。
- **H4 (Low)** chroma-go + 30 条知识库 vs 全量注入决策 → MVP 接受 finding 建议 "tag 预过滤 + 全量注入"，条目过百再上向量库。Q3 留 v1.x 评估。

#### §8 DoD 新增条目（H1/H2/H5 落地, 接在原 #13 之后）

| # | 验收点 | finding |
|---|---|---|
| 14 | **H1 闭环**：所有 `/ai/score` 响应含 `scoreRulesVersion` 字段; `internal/advisor/evaluator/v1_0_0.go` 每条规则带注释含权重/依据/校准来源; **Mars 在 等待决策.md D-WAIT-002 拍 Q1 权重表 + Q4 硬阈值后**, PRD-011 进研发 |
| 15 | **H2 闭环**：Collector 抓 BSL 含 region + provider + bucket 字段; "异地"判定按 §H2 5 级表; 实测一个客户场景 (e.g. local MinIO + cloud Azure same region) 评分对得上预期 |
| 16 | **H5 闭环**：`/ai/score` 同步返回 < 5s + `/ai/explain/:taskId` SSE 流式返回; UI 抽屉打开 5s 内出 score + 抽屉内 LLM 解释**实时打字效果**; abort 抽屉自动 abort LLM 请求 |
| 17 | **H3/H4 备注**：ADR-037 让号已闭环（PRD-008 → ADR-039）；chroma-go v1 不上，MVP 全量注入 + 标签预过滤 (Q3 留 v1.x 决策)|

---

## PRD-012 — Call Home / Auto-Support（半连接自动支持 · 复用 Collector + DSL · 自动开 Case）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-012 |
| **任务编号** | （待分配） |
| **状态** | **草稿（Blocked, 待 Mars 提供 Case API 规格）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.12.x（晚于 PRD-011） |
| **关联 ADR** | ADR-033（sanitize 出网治理）/ **ADR-037（三档连接形态: SaaS 全连 / Call Home 半连 / AirGap 零出网 —— 本 PRD 是"半连"档的产品化）** |
| **关联 PRD** | PRD-011（复用其 Collector + Canonical DSL, 0 重复实现）/ PRD-003（非自治原则: Call Home 只开 Case 给人, 不自动改客户集群） |
| **参考决策** | Mars 战略判断: "SaaS-for-Kubernetes 比 AirGap 更易调试; collector 让我们能自动建 Case, 售后工程师直连客户给方案"; 市场先例 NetApp AutoSupport / Dell SupportAssist / Pure Phone Home / Veeam |

> **Blocker 声明**: 本 PRD 的实现**依赖 Mars 提供公司售后系统（Case API）的接口规格**（认证方式、Case 创建字段、回执格式）。在拿到规格前, 本 PRD 停留在草稿, 只锁定**客户侧**架构（采集 + 脱敏 + 单向出网 + opt-in）。

### 1. Goal

为**半连接**客户（允许单向出站、不允许入站）提供 **Call Home**: 当 SupKube 在客户集群检测到**高风险态势或备份失败**时, 经客户**显式授权 + 脱敏**后, 把诊断快照单向发回 SupKube 售后系统, **自动开 Case**, 售后工程师据此主动联系客户并给出方案。把"客户出事才打电话求助"变成"我们先看到、先建 Case、先联系"。

### 2. Epic

ADR-037 三档连接形态中"**Call Home 半连**"档的产品化。复用 PRD-011 已经做好的 **Collector + Canonical DSL**——同一套采集、同一套数据模型, 只是把出口从"本地展示"换成"单向 egress 到售后系统"。

### 3. User Stories
- 作为半连接客户管理员, 我在 Settings → **Call Home tab** 看到默认**关闭**, 我阅读"会发送什么数据"清单后**显式开启**, 并可预览每次发送前的**脱敏后**载荷。
- 作为客户, 当我的关键应用备份连续失败, SupKube 自动（在我已 opt-in 前提下）脱敏后回传诊断快照, **公司售后系统自动建了一个 Case**, 售后工程师当天联系我。
- 作为售后工程师, 我在公司 Case 系统收到一个带**结构化诊断 DSL**的 Case（风险点 + 证据 + 建议）, 不用客户描述就能定位问题。
- 作为合规敏感客户, 我**完全不开** Call Home, 退回纯 AirGap 模式（PRD-011 本地闭环), 软件功能不降级。

### 4. Functions
- **复用 PRD-011 Collector**: 同一 CollectionContract 采集, 0 重复实现。
- **触发条件**: 高风险态势（安全级别 ≤ 脆弱级，即脆弱级/高危级；权威 4 档见 术语表.md §3.1）或备份连续失败 N 次 → 进入"建议 Call Home"状态（**不自动发**, 等客户 opt-in 策略: 一次性确认 or 预授权自动）。
- **脱敏**: 单一 `sanitize.go`（ADR-033 §6）, 生成 SanitizeReport, 客户可预览。
- **单向 egress transport**: 只出不入, HTTPS 出站到售后系统 endpoint; 无任何入站监听/反连。
- **自动建 Case**: 调用公司售后系统 Case API（**待 Mars 规格**）, 附结构化诊断 DSL。
- **回执**: Case 号回传客户 SupKube UI, 客户可见"已为您建 Case #xxx, 工程师将联系您"。
- **非自治**: Call Home **绝不**自动修改客户集群, 只是把信息送到"人"手里, 由售后工程师与客户协作执行（ADR-033 §6 / PRD-003 原则一致）。

### 5. Non-Goals
- ❌ 入站连接 / 远程控制客户集群。
- ❌ 自动执行修复。
- ❌ 未经 opt-in 的任何数据外发。
- ❌ 把客户原始数据（非脱敏）外发。

### 6. 技术方案 / 组件清单（SRE 视角）

| 组件 | 复用/新增 | 说明 |
|---|---|---|
| Collector | **复用 PRD-011** | 同一 CollectionContract |
| sanitize.go | **复用 ADR-033** | 单一脱敏入口 + SanitizeReport |
| Egress client | 新增 | 单向 HTTPS 出站, 重试/断点续传 |
| Case API client | 新增（**Blocked**） | 待 Mars 提供规格 |
| Call Home Settings tab | 新增（前端） | opt-in 开关 + 载荷预览 + Case 回执 |
| 公司侧售后系统 Case 自动创建 | 公司侧（非本仓库） | 待 Mars 对接 |

### 7. DoD（验收, 待规格补全后细化）
1. 默认关闭, 必须显式 opt-in 才发送。
2. 每次发送前生成 SanitizeReport, 客户可预览脱敏后载荷。
3. 仅单向出站, 网络层验证无入站监听。
4. 触发 Call Home 后成功在售后系统创建 Case 并回传 Case 号。
5. 不开 Call Home 时, 软件退回 PRD-011 本地闭环, 0 功能降级。
6. Call Home 全程不修改客户集群任何资源。

### 8. 待评审 / 待补全
- **B-Blocker**: Mars 提供 Case API 规格（认证 / 字段 / 回执）。
- Q1: opt-in 粒度——每次确认 vs 预授权自动? 默认哪种?
- Q2: 触发阈值（Score ≤ HIGH? 备份失败 N=?）。
- Q3: 出站 endpoint 多租户/客户标识方案。

### 9. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-06-01 | Claude | — → **草稿（Blocked）** | 由 Mars 战略判断"SaaS-for-Kubernetes 比 AirGap 易调试 + collector 自动建 Case"收敛。**核心决策**: (1) 定位为 ADR-037 三档连接的"半连"档产品化; (2) 复用 PRD-011 Collector + DSL, 0 重复; (3) 默认关闭, 显式 opt-in + 脱敏预览 + SanitizeReport; (4) 单向出站、无入站、非自治, 只开 Case 给人不自动改集群; (5) 市场先例对标 NetApp/Dell/Pure/Veeam。**Blocked**: 待 Mars 提供公司售后系统 Case API 规格方可进入开发。|
| 2026-06-02 | Claude (PRD-Review 第六份 I1 finding 闭环, I2 仍 Blocked) | 草稿 (Blocked) → **改正中 (仍 Blocked 等 Case API)** | 加 §10 PRD-Review 第六份 I1 finding 修订段：**I1** Story 3.2 + Q1 "预授权自动"模式有数据外泄风险——改默认 **逐次确认**（every outbound payload 需人工 review SanitizeReport 后点 Send）, "预授权自动"降级为显式高级 opt-in（Settings 二级开关 + 警告横条 + audit log）+ Call Home payload 字段并入 **SECURITY.md §6 出境白名单**（跟 PRD-011 SaaS / PRD-003 同一可审计管线, 客户可逐项关）。**I2** 客户身份方案（customer-id 一次性 K8s Secret 注入）见 §11 待补——**仍 Blocked 等 Mars 提供 Case API spec**。|

### 10. PRD-Review 第六份 I1 finding 修订段（2026-06-02 闭环, I2 仍 Blocked）

> **修订动因**：PRD-Review-2026-06-01-PRD009v2-011-012.md（PRD-Review 第六份 §五）对 PRD-012 给出 4 个 finding（I1-I4）。本段修 I1 (默认逐次确认 + §6 白名单); I2 (客户身份) 跟 Case API spec 强依赖, 写进 §11 待补 + 等 Mars。I3/I4 已闭环。

#### I1（Med-High）默认逐次确认 + Call Home payload 并入 SECURITY §6 出境白名单

**finding 原文**：Story 3.2 + Q1 的"预授权自动"模式——数据**无每次人工复核**就离开集群。脱敏一旦漏一项即外泄。

**拟方案（两件事）**：

**1. 默认逐次确认（每次 Send 都人工 review）**：

| 场景 | 触发 | 默认行为 |
|---|---|---|
| Score ≤ HIGH + 备份连续 N 次失败 → Call Home 准备 | Collector 跑完 + sanitize 准备好 payload | **不自动 send**, UI 弹窗 "准备 Send to Support" + 展示 SanitizeReport 全文 (≤ 100 KB 全展开 / >100 KB 折叠+下载) + 客户点 "Send" / "Cancel" |
| **"预授权自动"模式**（Settings 二级开关 + 警告横条 + audit log）| 客户在 Settings 显式 enable + accept disclaimer + 二次确认 | 自动 send（仍走 sanitize 管线）, 但 UI 显示 "上次 send: 5h ago" 实时通知 + 客户可随时 disable |

UI mockup：
```
┌─ Send Diagnostic Report to Support ─────────────────────────┐
│  下面是将外发到 jumborca 售后系统的字段（已脱敏）：             │
│                                                              │
│  Tap to expand 8 sections (245 lines, 12 KB)                │
│  ► identity { customer-id, cluster-id (HMAC) }              │
│  ► app_context { ns, workloads, ... }                       │
│  ► backup_health { last_backup_at, failure_count, ... }     │
│  ► sanitize_report { redacted: 14, fields: [pvcName,...] }  │
│                                                              │
│  □ I've reviewed the payload and want to send it.           │
│                                                              │
│  [Cancel]                                  [Send to Support] │
└──────────────────────────────────────────────────────────────┘
```

**2. Call Home payload 并入 SECURITY.md §6 出境白名单**：

PRD-012 出境字段**跟 PRD-011 SaaS / PRD-003 共用同一管线**——SECURITY.md §6.X 一张白名单:

| section | fields | 谁可关 | 关掉后果 |
|---|---|---|---|
| identity | customer-id (一次性 K8s Secret 注入, I2 范围), cluster-id (HMAC of kube-system UID), product-version | 不可关（Case 无法关联） | 关 = Call Home disable 整体 |
| app_context | namespaces[], workloads[], pvc_count, ... | 可关 | Case 没应用上下文, 售后无法定位 |
| backup_health | last_backup_at, failure_count, snapshot_size_bytes | 可关 | Case 没失败 timeline, 售后猜瓶颈难 |
| sanitize_report | redacted_count, fields_redacted[] | 可关（但建议不关）| 关 = 客户不知道我们脱了什么 |

**实施**：
- `SECURITY.md §6.C` 加 PRD-012 出境字段表
- `internal/sanitize/sanitize.go` 是 SSOT, PRD-003 / PRD-011 / PRD-012 都走它
- 客户在 Settings 看到的 "AI / Call Home 出境字段管理" 是 §6 表的可视化

**DoD 入 §8**：#13。

#### I2（Med, Blocked）客户身份方案

**finding 原文**：出站需让售后系统识别"这是哪个客户/合同"，但项目刻意无 license server（ADR-034）。无客户身份则 Case 无法关联。

**拟方案（待 Case API spec 后确认）**：
- 安装期 helm 命令传 `--set callHome.customerToken=<one-time-token>` (jumborca 售后系统发的)
- helm 模板把 token 写入 `supkube-callhome-customer` K8s Secret（namespace=supkube, 跟 fingerprint secret 同模式）
- Call Home payload identity.customer-id = HMAC(token, "callhome-identity-salt")（不直传原 token）
- 售后系统侧根据 customer-id 关联 Case → 客户合同

**Blocked**：等 Mars 提供 Case API spec:
- API endpoint URL (e.g. `https://support.jumborca.com/api/v1/cases`)
- Auth (Bearer token? mTLS?)
- Request schema（customer-id 字段名 / payload section 名 / 是否需 signature）
- Response schema（case-id / status / 后续追踪 url）

**等 Mars 给规格** → 写入 §11 待补 → 解 Blocker → PRD-012 转研发中。

**DoD 入 §8**：#14 (等 spec)。

#### I3 + I4（备注, 不动）

- **I3 (Info)** SanitizeReport 预览是亮点。已有, 保留 §8.2。
- **I4 (Info, 赞)** Blocked 状态用得对。保持。

#### §8 DoD 新增条目（I1-I2 落地）

| # | 验收点 | finding |
|---|---|---|
| 13 | **I1 闭环 (a)**：UI Send to Support 抽屉**默认逐次确认**, 展示 SanitizeReport 全文 + 客户点 "Send" 才发; "预授权自动" 模式是 Settings 二级开关 + warning banner + audit log |
| 14 | **I1 闭环 (b)**：Call Home 出境字段表写入 **SECURITY.md §6.C**，跟 PRD-003/PRD-011 共用一份白名单; `internal/sanitize/sanitize.go` 是 SSOT, 3 处调用方都走它 |
| 15 | **I2 设计就绪**：等 Mars 给 Case API spec (URL/auth/schema), 拿到后 customer-token 经 HMAC 派生 customer-id 走 identity section; helm `--set callHome.customerToken=` 注入 K8s Secret (跟 fingerprint secret 同模式) |

---

<a id="prd-013"></a>
## PRD-013 — SupKube Four-Eyes Authorization（备份安全二次审批 + MFA 平台集成）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-013 |
| **任务编号** | TBD（Mars 批后建 task） |
| **状态** | **草稿（2026-06-02 立项, Mars D-WAIT-002 frame shift 派生）** |
| **作者** | Claude / Mars |
| **目标版本** | v0.11.x（晚于 PRD-011 MVP, 不卡 Demo 核心闭环） |
| **关联 ADR** | 拟 **ADR-045**（ApprovalPolicy/ApprovalRequest CRD 设计 + Dex MFA 集成模式）← 让号自 ADR-044（与快速调试模式 ADR-044 撞号，2026-06-02 Rule G §C 让号） |
| **关联 PRD** | **PRD-011 §6 维度 3 / MFA+二次审批 10 分**（本 PRD ship 前此项标 N/A 不计入分母）/ **PRD-008 §RP 删除**（Delete RP 进入 Four-Eyes 受保护清单）/ **PRD-009 §Import Policy**（跨集群 Restore-to-PROD 受保护）/ PRD-003 §AI Advisor（AI 推荐执行也走 Four-Eyes）|
| **关联文档** | `SECURITY.md` §X 新章 "Four-Eyes Authorization & Privileged Operations Audit" / `USER_MANUAL.md` §X 新章 "MFA 启用与二次审批工作流" / 测试用例.md TC-SEC-001~005 新建（取号待 LEDGER 新 series TC-SEC）|
| **反向兼容** | 向后兼容: ApprovalPolicy 默认**全 disabled**, 客户必须显式 enable 才生效, 0 默认行为变更; MFA toggle 默认 off, 启用后立即对所有 user 生效（强制 TOTP enrollment grace period 7 天）|

> **立项缘由（2026-06-02）**：Mars 在 D-WAIT-002 PRD-011 评分细则讨论时提出"MFA 我就自己加在我们的平台上 + 二次审批只要启用了二次审批策略就 OK, 比如, 操作员要删除一个备份集, 要在另一个管理员、主管, 这样的人做二次审批"。这等价于 **Veeam VBR 13 的 Four-Eyes Authorization** ([helpcenter.veeam.com](https://helpcenter.veeam.com/docs/vbr/userguide/four_eyes_authorization.html?ver=13))——企业 DR 反勒索的金标准, 防"单管理员账号被攻陷后恶意清空备份" + "员工误操作"。**Kasten K10 当前没有这一层**, SupKube 实现后是真差异化护城河。本 PRD 落 Veeam 4 大类策略 + 加 SupKube K8s 多集群特有的 3 个护身符（跨集群 Restore-to-PROD / DR Drill 关闭 / BSL Object Lock 解锁）= **合计 15 个受保护操作**。

### 1. Goal

把 SupKube 从"单管理员账号即可执行任意操作"升级到 **"4 大类 15 个危险操作强制 Four-Eyes Authorization（双人审批 + MFA）"**, 让 SupKube 满足等保 2.0 三级 + ISO 27002:2022 §8.13 + NIST SP 1800-26 (反勒索) 的"特权操作两人责任"要求, 提供 **Kasten K10 没有的真差异化卖点**。

### 2. Epic

**"备份安全反勒索"** Epic ——把"特权操作单人即可"升级为"特权操作两人责任", 跟 Veeam VBR 13 + 金融行业 DR 实践对齐, 是 SupKube **从 SMB 工具升 enterprise 级**的最关键安全特性。本 PRD 是该 Epic 的首个全面落地。

### 3. User Stories

- 作为 **SupKube 平台管理员（Admin）**, 我在 Settings → Security → Approval Policies 启用 Four-Eyes Authorization, 选择哪些操作类别需要双人审批（4 大类 × 15 操作的复选), 设置每个 ApprovalPolicy 的 approvers (一个 Role / 一组 Users), 让平台**强制对危险操作走两人责任流程**, 我自己删 RP 时也需要另一管理员 approve。
- 作为 **运维操作员（Operator）**, 我点击 "Delete Restore Point" 按钮, UI 立刻提示"该操作需 Four-Eyes Authorization, 已发起审批请求, 等待 approver 批准"; 我看到 Pending Approval 列表知道我的请求被谁审; approver 批准后操作自动执行, 我不用重新点; 拒绝或超时后操作取消, 我看到原因。
- 作为 **审批人（Approver）**, 我打开 Pending Approval tab, 看到运维 Alice 发起的"Delete Restore Point: prod-mysql-2026-06-01"请求, 我看到 (a) 申请人 / 时间 / 申请理由 (operator 填) / (b) 操作详情（哪个 RP, 什么时间创建, 多少 GB, 关联哪个 Application）/ (c) 风险评估（这个 RP 是否是该应用最近的有效备份? 删除后 RPO 退化多少?）/ (d) Approve / Reject 按钮（Reject 必须填理由）/ (e) MFA 二次确认（点 Approve 后必须输入 TOTP 才生效, 防 approver session 被劫持）。
- 作为 **审计 / 合规官员（Auditor）**, 我打开 Activity → Privileged Operations 看到所有 Four-Eyes 请求的完整链路: 申请人 / 操作 / 申请时间 / 批准人 / 批准时间 / 操作执行结果, 这些事件**全部走 PRD-008 audit event** 持久化（与 ADR-019 audit log 共用），不可篡改, 满足等保 2.0 / SOC2 / ISO 27001 审计要求。
- 作为 **被入侵的攻击者**（红队场景）, 我攻陷了 Admin Alice 的账号, 想清空所有备份准备勒索; 但当我点击 Force Delete Backup 时, **请求被 Four-Eyes 拦截**, 我必须再攻陷一个独立账号（且对应 approver role）才能执行; 加上 MFA 要求, 我必须**同时获得**两个账号的 TOTP secret, 极大提高攻击成本（"两人责任 = 攻击成本翻倍"）。

### 4. Functions

#### 4.1 受保护操作清单（4 大类 15 操作）

> **来源**: Veeam VBR 13 Four-Eyes Authorization 4 大类 12 操作 + SupKube K8s 多集群 + DR 视角特有 3 操作（合计 15）。**未启用 ApprovalPolicy** 的操作走原路径, 启用后强制走审批流程。

| # | 类别 | 操作 (SupKube API/UI) | Veeam 对应 | 风险描述 |
|---|---|---|---|---|
| 1 | **类 1 删数据** | `DELETE /restore-points/:name` (UI: Restore Points 行 → Delete) | 从磁盘删除备份 | 操作员误删或被攻陷后恶意清空 RP |
| 2 | **类 1 删数据** | `DELETE /volumesnapshots/:ns/:name` (UI: Application Items → Delete VS) | 删除存储快照 | 直接删 CSI VolumeSnapshot CR 等同 RP 失效 |
| 3 | **类 1 删数据** | `DELETE /backups/:name` (UI: 老 Backup CR 清理 / 孤儿元数据) | 从配置 DB 移除备份 | 删 Velero Backup CR 等同 RP 元数据消失 |
| 4 | **类 1 删数据** | `POST /backups/:name/force-delete` (UI: PRD-008 Force Delete 入口) | (Veeam Purge) | 绕 finalizer + 不删 BSL 对象 = 数据残留陷阱（PRD-008 §11 已警告）|
| 5 | **类 2 基建** | `DELETE /bsls/:name` (UI: Settings → Storage Locations → Delete) | 移除备份存储库 | 解绑 BSL 等同所有 RP 失联（虽 BSL 对象仍在云上）|
| 6 | **类 2 基建** | `DELETE /vsls/:name` (UI: Settings → VolumeSnapshotLocations → Delete) | 移除生产存储 | VSL 失联等同 snapshot 路径断 |
| 7 | **类 2 基建** | `DELETE /clusters/:name` (UI: MC Manager → Cluster → Remove) | 移除云提供商 | MC cluster 移除 → 跨集群 RP 入口失效（PRD-009 v2 Import Policy 信任链断）|
| 8 | **类 3 用户** | `POST/PATCH/DELETE /users` (UI: Settings → Users) | 增/改/删用户 | 攻击者建后门高权账号 |
| 9 | **类 3 用户** | `PATCH /users/:id/role` (UI: Settings → Users → Change Role) | 修改用户角色 | 攻击者提权自己为 Admin |
| 10 | **类 4 安全** | `PATCH /settings/approval-policies` set `enabled=false` | 禁用四眼原则本身（**递归保护**） | **关键**: 关 Four-Eyes 本身也必须走 Four-Eyes（防"先关再删"） |
| 11 | **类 4 安全** | `PATCH /settings/mfa` set `globalMFA=false` | 全局 MFA 变更 | 关 MFA = 安全降级铺路 |
| 12 | **类 4 安全** | `POST /users/:id/reset-mfa` | 重置特定用户 MFA | 攻击者重置 admin MFA 后用自己 device 登录 |
| 13 | **类 4 安全** | `PATCH /settings/session` set `autoLogout` 改长或关 | 全局自动注销 | 关 auto-logout = session 永驻给攻击窗口 |
| 14 | **SupKube 特有** | `POST /restores` to `cluster.tier=prod` (跨集群 Restore 到生产) | (Veeam 单集群无对应) | K8s 多集群独有: 误把 dev RP 还原到 prod = 数据覆盖灾难 |
| 15 | **SupKube 特有** | `PATCH /policies/drill-schedule` set `enabled=false` 或 `PATCH /bsls/:name/object-lock` set `disabled=true` | (Veeam 无对应) | 关 DR Drill 或解锁 BSL Object Lock = PRD-011 评分维度 2/4 直接破防 |

> **客户可选**: 上述 15 操作每条**单独可 enable/disable**, 客户按风险偏好配置（e.g. 中小客户只启用类 1 删数据 4 条）。Settings → Security → Approval Policies 提供 **"Veeam 标准 (12 条全开)"** + **"SupKube 增强 (15 条全开, 推荐)"** + **"自定义"** 三档 preset。

#### 4.2 ApprovalPolicy CRD spec

```yaml
apiVersion: supkube.io/v1
kind: ApprovalPolicy
metadata:
  name: prod-cluster-strict
  namespace: supkube-system           # cluster-scoped 也行, 设计为 namespaced 便于 RBAC
spec:
  # —— 适用范围 —— 
  enabled: true                       # 启用本策略
  scope:
    operations:                       # 受保护操作列表 (引用 §4.1 #1-#15 op IDs)
      - delete-restore-point          # #1
      - delete-volumesnapshot         # #2
      - force-delete-backup           # #4
      - delete-bsl                    # #5
      - cross-cluster-restore-to-prod # #14
    clusters:                         # 仅对这些 cluster 生效（空 = 所有）
      - aks-jumborca-prod
    namespaces:                       # 仅对这些 namespace 生效（空 = 所有）
      - prod-finance
      - prod-orders
  
  # —— 审批要求 ——
  approvers:                          # 谁能审批（满足任一条即可）
    roles:                            # K8s ClusterRole names
      - supkube-admin
      - supkube-security-officer
    users:                            # 显式 user list（OIDC sub claim）
      - boss@company.com
    minApprovers: 1                   # 需多少个 approver 同意（默认 1, 可调到 2 / 3 = "三眼" / "四眼"）
  selfApproveForbidden: true          # 申请人不能 approve 自己（强制）
  
  # —— 请求生命周期 ——
  requestTtl: 24h                     # 请求 TTL, 超时自动 Rejected
  
  # —— MFA 要求 ——
  mfaRequired: true                   # approver 点 Approve 时强制输 TOTP
  
  # —— 通知 ——
  notifications:
    email:
      enabled: true
      to:                             # 显式收件人（除自动通知 approvers）
        - security-team@company.com
    slack:
      enabled: false
      webhook: ""
      channel: "#supkube-approvals"
    inApp:
      enabled: true                   # SupKube UI 顶栏红点 + Pending Approval tab badge
```

#### 4.3 ApprovalRequest CRD + 状态机

```yaml
apiVersion: supkube.io/v1
kind: ApprovalRequest
metadata:
  name: req-2026060214301a3f          # 自动生成 UUID-like
  namespace: supkube-system
spec:
  policyRef:                          # 关联的 ApprovalPolicy
    name: prod-cluster-strict
  
  operation:
    type: delete-restore-point         # §4.1 op ID
    target:                           # 操作目标（用于 approver 看上下文）
      kind: RestorePoint
      apiVersion: supkube.io/v1
      namespace: prod-finance
      name: orders-mysql-2026-06-01
    parameters:                       # 透传原 API 调用的 body（脱敏后）
      reason: "test cleanup"          # operator 填的申请理由
    
  requester:                          # 申请人
    username: alice@company.com
    role: supkube-operator
    userAgent: "Mozilla/5.0 ..."
    sourceIP: "10.0.1.42"
  
status:
  phase: Pending                      # Pending → Approved → Executing → Executed (终态)
                                      #         → Rejected (终态)
                                      #         → Expired (终态, requestTtl 过)
                                      #         → Cancelled (终态, requester 主动撤回)
  approvals:                          # 已收到的批准（数量 ≥ minApprovers → phase 进 Approved）
    - approver: boss@company.com
      approvedAt: "2026-06-02T15:30:00Z"
      mfaVerified: true               # MFA 验证通过证据
      comment: "OK, but next time cleanup before EOD"
  rejection: null                     # 若 Rejected, 这里填理由
  executionResult: null               # Approved → 实际执行后填: success / failed + error msg
  expiresAt: "2026-06-03T14:30:00Z"
```

**状态机**:

```
Pending ──(approvals ≥ minApprovers)──> Approved ──(controller 执行原操作)──> Executing ──> Executed (成功) / Executed (失败 + errorMsg)
   │
   ├──(任一 approver Reject)──> Rejected (终态)
   ├──(requestTtl 过)──> Expired (终态)
   └──(requester 撤回)──> Cancelled (终态)
```

**幂等性**: Approved → Executing 由 controller 单次执行（leader election + idempotent guard, 防 controller 重启重复执行）。

#### 4.4 MFA 平台集成（Dex OIDC + TOTP/WebAuthn）

| 模块 | 设计 |
|---|---|
| **认证流程** | Dex OIDC 已是 SupKube login backend（ADR-010）, 增加 Dex **TOTP connector** + **WebAuthn connector**（passkey 支持）。客户登录 = OIDC 主认证 + (若 MFA 启用) TOTP 6 位码 或 WebAuthn 触摸 |
| **Enrollment** | 客户首次登录后, UI 强制 enrollment 页：扫二维码绑定 Authenticator app（Google Authenticator / 1Password / Authy）/ 注册 WebAuthn passkey; grace period 7 天可暂时跳过, 之后必须绑定才能登录 |
| **存储** | TOTP secret 存 K8s Secret `mfa-secrets`（type=Opaque, encrypted at rest with K8s default + sealed-secrets 推荐）, secret name = `mfa-totp-{userId-sha256}` |
| **二次确认** | approver 点 Approve / 执行任一 §4.1 受保护操作时, 即使 session 内已登录, UI 弹"输入当前 TOTP 6 位码"对话框, 验证通过后才生效（防 session hijacking） |
| **管理员重置** | 客户丢手机时, Admin 在 Settings → Users → Reset MFA 重置, 但此操作本身是 §4.1 #12 受保护操作, 必须走 Four-Eyes（防攻击者用"假装丢手机"绕过 MFA）|
| **Recovery codes** | Enrollment 时一次性给 8 个 recovery code（哈希存 Secret）, 客户打印保存; 登录时 TOTP 失败 N 次可用 recovery code（每个一次性, 用完 invalidate）|

#### 4.5 UI/UX

##### 4.5.1 Settings → Security → Approval Policies (新页)

```
┌─────────────────────────────────────────────────────────────────┐
│  Security & Compliance                                          │
│  ├─ Authentication (MFA)  ├─ Approval Policies ◀──  ├─ Audit   │
│                                                                  │
│  ☑ Enable Four-Eyes Authorization (Global)                      │
│                                                                  │
│  Preset:  ○ Veeam Standard (12 ops)  ● SupKube Enhanced (15)   │
│           ○ Custom                                              │
│                                                                  │
│  Policy: [prod-cluster-strict ▾]  [+ New Policy]               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Scope:                                                       ││
│  │   Clusters: [aks-jumborca-prod] [+]                          ││
│  │   Namespaces: [prod-finance, prod-orders] [+]                ││
│  │                                                              ││
│  │ Operations (15 selectable):                                  ││
│  │   类 1 删数据 (4):  ☑☑☑☑                                  ││
│  │   类 2 基建 (3):    ☑☑☑                                    ││
│  │   类 3 用户 (2):    ☑☑                                      ││
│  │   类 4 安全 (4):    ☑☑☑☑                                  ││
│  │   SupKube 特有 (2): ☑☑                                      ││
│  │                                                              ││
│  │ Approvers:                                                   ││
│  │   Roles: [supkube-admin, supkube-security-officer] [+]      ││
│  │   Users: [boss@company.com] [+]                              ││
│  │   Min approvers: [1 ▾]  ☑ Self-approve forbidden            ││
│  │                                                              ││
│  │ Request TTL: [24h ▾]  ☑ MFA required for approvers         ││
│  │ Notifications: ☑ Email  ☐ Slack  ☑ In-App                  ││
│  └─────────────────────────────────────────────────────────────┘│
│  [Save]  [Test (simulate a request)]                           │
└─────────────────────────────────────────────────────────────────┘
```

##### 4.5.2 Operator 视角 - 发起受保护操作

```
[User clicks "Delete Restore Point" on Restore Points page]

┌─────────────────────────────────────────────────────────────────┐
│  ⚠ This operation requires Four-Eyes Authorization              │
│                                                                  │
│  Operation: Delete Restore Point                                │
│  Target:    prod-finance / orders-mysql-2026-06-01              │
│  Policy:    prod-cluster-strict                                 │
│  Approvers needed: 1 (from supkube-admin / supkube-security-officer) │
│                                                                  │
│  Reason for request: *                                          │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Routine cleanup of old test backup, RP > 30 days, no       ││
│  │ active restore needed                                       ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Notification: Approvers will be notified via Email + In-App.   │
│  Request TTL: 24h (auto-rejected if not approved)               │
│                                                                  │
│             [Cancel]  [Submit Approval Request]                  │
└─────────────────────────────────────────────────────────────────┘

→ Submitted! Request ID: req-2026060214301a3f
→ Status: Pending Approval (you'll be notified when approved/rejected)
→ View in: My Requests
```

##### 4.5.3 Approver 视角 - Pending Approval 列表

```
┌────────────────────────────────────────────────────────────────────┐
│  Pending Approval (3)                                              │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ 🔶 Delete Restore Point: orders-mysql-2026-06-01             │ │
│  │    Requested by: alice@company.com (operator)                │ │
│  │    Submitted:    2026-06-02 14:30 (28 min ago)               │ │
│  │    Expires:      2026-06-03 14:30 (23h 32m left)             │ │
│  │    Reason:       "Routine cleanup of old test backup..."     │ │
│  │                                                                │ │
│  │    🔍 Operation Details                                        │ │
│  │    ├─ Target:        RP orders-mysql-2026-06-01               │ │
│  │    ├─ Created:       2026-06-01 03:00 (35h ago, by Policy "..")│
│  │    ├─ Size:          18 GB                                    │ │
│  │    ├─ Application:   prod-finance/orders-mysql                │ │
│  │    └─ Risk:          ⚠ This is the 2nd most recent RP. After │ │
│  │                       deletion, RPO of orders-mysql may worsen│ │
│  │                       from 24h to 48h.                        │ │
│  │                                                                │ │
│  │    [✓ Approve (MFA required)]  [✗ Reject]                     │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ 🔶 Force Delete Backup: stuck-backup-xyz                     │ │
│  │ ... (collapsed)                                              │ │
│  └──────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘
```

##### 4.5.4 Activity → Privileged Operations (新 tab)

跟 PRD-008 Activity 持久化层共用, 显示所有 §4.1 #1-#15 操作的完整链路 (申请人 / 操作 / 申请时间 / approver / 批准时间 / 执行结果)。审计员可按 approver / requester / operation type / time range filter。

#### 4.6 通知

| 通知渠道 | 触发时机 | 内容 |
|---|---|---|
| **Email** | (a) 请求新建 → 通知 approvers; (b) 请求 approved/rejected → 通知 requester; (c) 请求 24h 过半未审批 → 提醒 approvers | 包含请求详情 + UI deep-link `https://supkube.../approvals/req-xxx` |
| **In-App** | 同上, UI 顶栏铃铛红点 + Pending Approval tab badge 数字 | 实时（SSE 推送）|
| **Slack** | Opt-in, Settings 配 webhook | Slack block kit message + Approve/Reject 按钮（点按钮走 Slack OAuth 鉴权后回 SupKube）|

#### 4.7 审计（与 PRD-008 共用 audit event）

每个 ApprovalRequest 生命周期事件 → audit event 落 PRD-008 §audit store:
- `APPROVAL_REQUEST_CREATED` (requester, operation, target)
- `APPROVAL_GRANTED` (approver, mfa_verified, comment)
- `APPROVAL_REJECTED` (approver, reason)
- `APPROVAL_EXPIRED` (auto)
- `APPROVAL_CANCELLED` (requester)
- `PROTECTED_OPERATION_EXECUTED` (final outcome, success/failed)

事件**不可删除**（PRD-008 D2 hash-chain + admission webhook + WORM 三层防御兼用）。审计员通过 PRD-005 Log Viewer + `auditCategory=approval` filter 查询。

### 5. UI/UX 细节

(留 §4.5 三个 mockup 已覆盖主线, 详细组件库设计 + tokens 见 UI_GUIDELINES.md 更新)

### 6. Out of Scope（明确不做）

- **基于 risk score 的自动 escalation**: 高风险操作自动升 minApprovers=2/3, 复杂度高且策略主观, v1 不做（v2 可考虑）
- **审批工作流编排（多级审批链）**: e.g. operator → team lead → security officer 三级串行, v1 只做 "minApprovers" 平行模式（同时通知所有 approver, 满足 N 个即可）, 多级编排 v2 考虑
- **第三方 IdP 强 MFA 透传**: 客户 Azure AD / Okta 自带 MFA 时, 是否信任 IdP 的 MFA claim 跳过 SupKube TOTP, v1 不做（统一走 SupKube TOTP, 简化威胁模型）
- **离线 approver**: 完全断网时通过其他渠道发起的 approval（电话 + 物理 token）, v1 不做

### 7. 非功能性要求

- **可用性**: ApprovalPolicy controller / ApprovalRequest controller 高可用（leader election 模式, 单实例 down 不丢未决请求）
- **性能**: 请求创建 < 100ms (含 K8s CR write); 列表查询 < 500ms (每页 50, 按 user filter)
- **可观测**: Prometheus metrics `supkube_approval_requests_total{phase}` / `supkube_approval_latency_seconds`（创建→批准时长）
- **安全**: TOTP secret 加密存储 + sealed-secrets 推荐; recovery code 哈希存（bcrypt + salt）; MFA enrollment QR code 在 HTTPS-only 显示 + 5 分钟 TTL

### 8. 验收标准（DoD）

| # | 验收点 |
|---|---|
| 1 | ApprovalPolicy CRD + ApprovalRequest CRD schema 通过 kubectl explain 校验; OpenAPI v3 schema 完整 |
| 2 | controller (`internal/approvalpolicy/`) reconcile 状态机正确: Pending → Approved → Executing → Executed; 任一状态机转换均有单元测试 |
| 3 | §4.1 #1-#15 操作的 API handler 全部拦截: 受保护时返回 202 + ApprovalRequest URL; 未启用时走原路径; e2e 测试每个 op id 各一条 |
| 4 | Dex MFA connector (TOTP) 集成: 客户首次登录强制 enrollment / grace period 7 天 / recovery code 8 个一次性 |
| 5 | UI Settings → Security → Approval Policies 完整: 3 个 preset + 自定义 + scope 选择 (cluster/ns) + approver 选择 (role/user) |
| 6 | Operator 发起受保护操作: 弹 §4.5.2 申请抽屉, 必填 reason, submit 后转 Pending Approval |
| 7 | Approver 看 Pending Approval 列表 §4.5.3: 含申请人 / 操作详情 / 风险评估 / Approve(MFA) + Reject 按钮 |
| 8 | MFA 二次确认: approver 点 Approve 后弹 TOTP 输入框, 验证通过才 ApprovalRequest.approvals 写入 |
| 9 | self-approve 拒绝: requester 在 approver 名单内时, UI 灰化 Approve 按钮 + tooltip "self-approve forbidden by policy"; 后端二重验证（绕 UI 直调 API 也拒）|
| 10 | requestTtl 到期自动 Reject: controller 定期扫 Pending 请求, 超 TTL 改 Expired + audit event; e2e 验证 |
| 11 | recursive 保护: §4.1 #10 "Disable ApprovalPolicy" 操作本身走 Four-Eyes (即使 enabled=false 想生效也要 approval); 单测验证 |
| 12 | 通知: Email + In-App + Slack (opt-in) 三通道工作; deep-link 点击直接进对应 ApprovalRequest 页 |
| 13 | 审计 event 6 条 (`APPROVAL_REQUEST_CREATED` / `APPROVAL_GRANTED` / `APPROVAL_REJECTED` / `APPROVAL_EXPIRED` / `APPROVAL_CANCELLED` / `PROTECTED_OPERATION_EXECUTED`) 走 PRD-008 audit store + hash-chain 防篡改 |
| 14 | Prometheus metrics 3 条暴露: `approval_requests_total` (counter, labels phase/policy) / `approval_latency_seconds` (histogram, 创建→批准) / `mfa_verifications_total` (counter, labels result) |
| 15 | helm chart 加 `approval-policy-controller` deployment + RBAC + CRD; values.yaml `security.fourEyes.enabled` 默认 false (向后兼容); `security.mfa.enabled` 默认 false |
| 16 | USER_MANUAL §X "MFA 启用 + Approval Policies" 新章; SECURITY.md §X "Four-Eyes Authorization & Privileged Operations Audit" 新章; 测试用例 TC-SEC-001~005 |
| 17 | **反勒索演练**: 模拟攻击者拿到 admin Alice 凭据 → 尝试 force-delete 所有 Backup CR → 被 ApprovalRequest 拦截 + audit log 留痕 (e2e 测试) |

### 9. 任务拆分

> 估时 ~13 天 = **5d MFA + 8d 二次审批**。可以拆 2 sprint 推进。

| Phase | 内容 | 估时 |
|---|---|---|
| **P0 ADR-045 + skeleton** (1d) | ApprovalPolicy/ApprovalRequest CRD types + 头表 ADR + 状态机图 | 1d |
| **P1 MFA 后端** (3d) | Dex TOTP connector 集成 + secret 存储 + enrollment flow + recovery code | 3d |
| **P2 MFA UI** (2d) | Enrollment 页 + 登录 TOTP 输入 + Recovery code 展示 + Settings → Authentication | 2d |
| **P3 ApprovalPolicy controller** (2d) | reconciler + ApprovalRequest 状态机 + TTL controller + self-approve 拒绝 | 2d |
| **P4 API handler 拦截** (2d) | 15 个 op handler middleware: 检查 ApprovalPolicy → 返回 202 + URL; 测试 e2e 每条 | 2d |
| **P5 Settings UI** (1.5d) | Approval Policies 页 (preset 3 档 + 自定义) + scope 选择器 | 1.5d |
| **P6 Pending Approval UI** (1d) | List + Detail + Approve(MFA)/Reject + 风险评估卡 | 1d |
| **P7 通知** (1d) | Email + In-App SSE + Slack (opt-in) | 1d |
| **P8 审计 + Activity 集成** (0.5d) | 6 条 audit event 落 PRD-008 store + Activity Privileged Operations tab | 0.5d |
| **P9 测试 + 文档** (1d) | TC-SEC-001~005 + USER_MANUAL + SECURITY.md + 反勒索 e2e | 1d |

### 10. 关联文档与任务

- **ADR-045**（拟，让号自 ADR-044）: ApprovalPolicy/ApprovalRequest CRD 设计 + Dex MFA 集成模式
- **PRD-011 §6 维度 3 / MFA + 二次审批 10 分**: 本 PRD ship 前标 N/A 不计入分母; ship 后此 10 分激活
- **PRD-008 §audit event**: 本 PRD 6 条 event 复用 PRD-008 hash-chain + WORM 三层防御
- **PRD-005 Log Viewer §audit category filter**: 审计员通过 `category=approval` 查询 ApprovalRequest 全生命周期
- **PRD-007 §4.6 DR Drill**: 关闭 Drill (op #15) 进入受保护清单
- **SECURITY.md §X 新章**: "Four-Eyes Authorization & Privileged Operations Audit" + 威胁模型 + 反勒索 narrative
- **测试用例.md TC-SEC-001~005**: (1) 受保护操作拦截 / (2) MFA enrollment + login / (3) self-approve 拒绝 / (4) recursive 保护 / (5) 反勒索演练 e2e
- **关联任务**: 本 PRD 落地后建 task (Mars 批后取号)

### 11. 开放问题（Q1-Q5）

- **Q1**: Approver 池如果都不在线（节假日 / 时区错峰）, requestTtl 自动 Reject 还是允许 Admin **超级权限 override**？(我推: 默认 reject + 提供 "Emergency Bypass" 二级权限给 super-admin, 但本身也走 audit; Mars 拍)
- **Q2**: ApprovalPolicy 数量限制？多个 policy 命中同一操作时优先级（最严格 / 第一个匹配 / 合并 minApprovers）？(我推: 同操作多 policy 命中 → 取 max minApprovers + union approvers + AND mfaRequired; Mars 拍)
- **Q3**: MFA enrollment grace period 7 天是否合理？金融客户可能要求 0 天（立即强制）；中小客户可能要求 30 天。(我推: Settings 可配 0/7/30, 默认 7; Mars 拍)
- **Q4**: Recovery code 用尽后流程？(我推: 必须管理员 reset MFA, reset 本身走 Four-Eyes, Lock-in 防绕过; Mars 拍)
- **Q5**: AI Advisor (PRD-003 / PRD-011) 推荐执行操作时, 自身是 "AI agent identity" 还是 "代理 user identity"？前者意味 AI 不能 self-approve, 必须人 approve, 防 AI 幻觉执行毁灭操作（推荐 ✅）；后者意味 AI 借用执行者身份, 简单但风险高（不推）。Mars 拍。

### 12. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-06-02 | Claude (Auto 5h, Mars D-WAIT-002 frame shift 派生) | — → **草稿** | PRD-013 v1 完成。**核心决策**: (1) 4 大类受保护操作 = Veeam VBR 13 对标 12 条 + SupKube K8s 多集群 + DR 视角特有 3 条 = 合计 15 条; (2) ApprovalPolicy / ApprovalRequest 两层 CRD 设计（前者策略, 后者每请求实例）, 状态机 7 状态; (3) MFA 走 Dex OIDC TOTP connector + WebAuthn passkey, 强制二次确认防 session hijack; (4) recursive 保护 — Disable ApprovalPolicy 操作本身也走 Four-Eyes; (5) self-approve forbidden + minApprovers 可调 1/2/3 = 双眼/三眼/四眼; (6) 6 条 audit event 走 PRD-008 hash-chain + WORM 三层防御兼用; (7) 默认全 disabled, 向后兼容 0 风险。**Q1-Q5 待 Mars 拍**: 紧急 bypass / policy 优先级 / MFA grace period / Recovery code 用尽 / AI Advisor identity。**关键依赖**: 本 PRD ship 后 PRD-011 §6 维度 3 / 10 分激活; 不卡 Demo 核心闭环 (Mars 决策延后, 进流水线)。 |

---

<a id="prd-014"></a>
## PRD-014 — 前端 UI 暴露模型（运维方 Day-0 可配 · 4 模式 · 装后访问引导）

> 立项缘由（Mars 2026-06-02）：ADR-042 上云后前端默认 NodePort，AKS 节点无公网 IP → UI 没有任何入口；早前手动开的 LoadBalancer 公网 IP（`172.188.195.36`）被 helm 默认值覆盖、被 Azure 回收（活动日志实证 2026-06-01 23:50 删除）。**根因不是 bug，是"前端怎么对外暴露"从来没有被当作一个一等的、运维方在安装时选择的决策来对待，而且装完之后产品没有告诉运维方该怎么访问。** Mars 拍板：不要把任何环境特定的暴露方式（LoadBalancer / 静态 IP）烧进我们的交付；产品只给安全默认值 + 让客户自己选 + 装后明确引导。镜像我们已有的 Dex `publicURL` 那套范本（install 时填值、改值 `helm upgrade`、缺关键值 fail-fast、装后 NOTES 指路）。

### 1. Goal（目标）

前端 UI 的对外暴露方式是**运维方在 Day-0（`helm install`）的安装决策**，不是产品替客户定死的东西。产品职责：(a) 提供一个到处都能跑的**安全默认值**；(b) 把暴露方式做成**清晰的、值驱动的菜单**（4 模式）；(c) 安装完成后**按所选模式打印准确的访问方式**，让运维方零猜测就能打开 UI；(d) 改暴露方式 = 改 values + `helm upgrade`，无需碰镜像 / 代码。

### 2. Epic（史诗故事）

作为一名**部署 SupKube 的运维方（客户或我们自己）**，我希望在安装时就能按我的集群环境（公有云 / 本地 / 离线 / 安全敏感）选择 UI 的暴露方式，并在装完后立刻得到"怎么打开 UI"的准确指引，这样我不必读源码、不必猜端口、也不会出现"装好了却打不开"的尴尬。

### 3. User Stories（用户故事）

- **US-1（公有云）**：作为 AKS/GKE/EKS 客户，我 `--set service.frontend.type=LoadBalancer`，装完 NOTES 告诉我"等 EXTERNAL-IP 出现，然后 http://<IP>/"。
- **US-2（本地/离线）**：作为 docker-desktop / on-prem 客户，我用默认 NodePort，装完 NOTES 告诉我 `http://<node-ip>:30888`。
- **US-3（安全敏感 / 平时不开）**：作为合规/气隙环境运维，我 `--set service.frontend.type=ClusterIP`（UI 不对外），装完 NOTES 直接给我一条 `kubectl port-forward` 命令，用时才连。
- **US-4（生产域名 + TLS）**：作为生产运维，我 `--set ingress.enabled=true` 配域名，装完 NOTES 告诉我 `https://<host>`。
- **US-5（改主意）**：我随时能 `helm upgrade` 切换模式，无需重建镜像。

### 4. Functions（业务逻辑 / 功能拆解）

| # | 功能 | 说明 |
|---|---|---|
| F1 | `service.frontend.type` = 4 模式 | `LoadBalancer` / `NodePort`（默认）/ `ClusterIP`（+port-forward）/ 配合 `ingress.enabled`。前三者是标准 K8s service type，chart 模板已支持渲染；本 PRD 把 **ClusterIP「不暴露/按需 port-forward」正式确立为一等文档化模式**。 |
| F2 | 模式感知的 NOTES.txt | `helm install` 后按 `.Values.service.frontend.type` / `.Values.ingress.enabled` **分支打印对应访问方式**（LB→等 EXTERNAL-IP；NodePort→`<node>:30888`；ClusterIP→给出 `kubectl port-forward` 命令；ingress→域名）。 |
| F3 | **修 NOTES.txt 服务名 bug** | 现有 NOTES 第 4 行 `port-forward svc/{{ supkube.fullname }}`（渲染成 `svc/supkube`）连不上 —— 真实服务名是 `<fullname>-frontend`。本 PRD 修正。 |
| F4 | values.yaml 4 模式菜单 | 把 `service.frontend` 注释从"3 路径"扩成清晰的 4 模式菜单，含 ClusterIP/port-forward 用途与各自适用场景。 |
| F5 | USER_MANUAL §5.5 | 新增「如何对外暴露 UI」一节，覆盖 4 模式 + 各自访问命令 + 改模式流程。 |

### 5. UI / UX

无前端页面改动。UX 载体 = **`helm install` 的终端输出（NOTES.txt）** + 文档。验收标准：运维方装完**不需要任何额外知识**就能照 NOTES 打开 UI。

### 6. Out of Scope（明确不做）

- **不绑静态公网 IP**：要固定 IP 的运维方自行用 `service.beta.kubernetes.io/azure-pip-name` 等注解（文档提示即可），产品不替客户管 IP 生命周期。
- **不在我们的 CD 里烧死某种模式**：各环境（dev/test/prod）选哪种暴露 = 各环境的 values 选择。把"按环境自动选 values"做成 CD 标准（per-env values overlay：`values-dev/test/prod.yaml`）是**关联但独立**的架构议题 → 见 §10，建议单独 ADR。
- 不附带安装 ingress controller / cert-manager（运维方自备）。

### 7. 非功能性要求

- **向后兼容**：默认仍 `NodePort`（任何集群可装），现有安装 `helm upgrade` 行为不变。
- NOTES 输出**不得泄露 secret**（只打印访问地址 / 命令）。
- 多云通用：LB 模式不写死任一云厂商注解。

### 8. 验收标准（Definition of Done）

1. `helm template` 在 `service.frontend.type` = LoadBalancer / NodePort / ClusterIP 三种取值、及 `ingress.enabled=true` 下，**各渲染出正确且互不串台的 NOTES 访问指引**。
2. NOTES.txt 服务名 bug 修复：port-forward 指向 `<fullname>-frontend`，实测可连。
3. `values.yaml` 的 `service.frontend` 注释呈现清晰 4 模式菜单。
4. USER_MANUAL 有「§5.5 如何对外暴露 UI」一节。
5. `node dashboard/gen-data.mjs` 漂移检查 ✅。
6. `helm lint` / `helm template` 无错误。

### 9. 任务拆分

- T1：重写 `templates/NOTES.txt`（模式感知 + 修服务名 bug）
- T2：`values.yaml` `service.frontend` 4 模式菜单注释
- T3：USER_MANUAL §5.5
- T4（可选）：LoadBalancer/ingress 模式下若缺 `auth.dex.publicURL` 的 fail-fast 提示对齐（与 dex-check.yaml 协同）

### 10. 关联文档与任务

- **ADR-042**（架构设计.md §9）：上云 + CD 默认 values → 本 PRD 修正其遗漏的"前端对外访问"。
- **ADR-044**（架构设计.md §9）：快速调试模式 `dev-local.sh --mode ui` 本质就是 **ClusterIP + port-forward** 模式的"吃自己狗粮"，与 F1 的 US-3 同源。
- **范本**：`templates/dex-check.yaml` + `auth.dex.publicURL`（install 时填值 / fail-fast / NOTES 指路的标准范式）。
- **关联架构议题（建议单独 ADR，取号见 LEDGER §一）**：CD per-environment values overlay（`values-{env}.yaml` 分层 + `helm -f values-{env}.yaml`），把"按环境改参数"从 cd.yaml 内联 `--set` 升级为声明式自动化。homelab 已有 `hack/homelab/values-homelab.yaml` 雏形。

### 11. 开放问题（评审时讨论）

- **Q1**：默认值是否从 `NodePort` 改为 `ClusterIP`（更安全 / 默认不暴露）？代价是 docker-desktop 本地体验多一步 port-forward。（倾向：保持 NodePort 默认，向后兼容优先；Mars 拍）
- **Q2**：LoadBalancer 模式是否要像 Dex 那样对"缺 publicURL"fail-fast？（倾向：是，登录链路一致性；Mars 拍）
- **Q3**：是否本 PRD 一并落地 per-env values overlay（§10 的独立 ADR 候选），还是拆为独立架构任务？（Mars 拍方向，对应你 #1 的 CD 自动化问题）

### 12. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-06-02 | Mars 决策 / Claude 起草 | — → **草稿** | 由 `172.188.195.36` UI 失联事件根因取证派生。核心立场：暴露方式 = 运维方 Day-0 可配，产品只给安全默认 + 装后引导，不烧死任何环境特定方式。发现并修复 NOTES.txt 服务名 bug。Part1（CD per-env 参数自动化）识别为关联独立 ADR 候选（取号见 LEDGER §一）。Q1-Q3 待 Mars 拍。 |

---

## PRD-015 — AI 容灾决策顾问（AI DR Decision Advisor · Premium 独占）

| 字段 | 值 |
|---|---|
| **PRD 编号** | PRD-015 |
| **任务编号** | TBD（post-MVP，待研发排期） |
| **状态** | **草稿（2026-06-03 立项 · charter 级）** |
| **作者** | Mars 决策 / Claude 起草 |
| **目标版本** | post-MVP（v0.12.x+，Premium 套餐） |
| **关联 ADR** | **ADR-046（两层正交决策体系，本 PRD 的架构根）** / ADR-033（AI Advisor 架构）/ ADR-037（统一数据采集 + Canonical DSL）/ ADR-031（5 层韧性）|
| **关联 PRD** | **PRD-011（MVP，本 PRD 的下层：A 面评分+解释）** / PRD-013（Four-Eyes 二次审批，决策执行的护身符）/ PRD-007（DR Drill 数据源）/ PRD-012（Call Home 同采集层）|

> **范围声明（防蔓延）**：本 PRD 是从 PRD-011 MVP **向上拆出**的 Premium 上层能力。**PRD-011 已含的"A 面评分 + LLM 解释"不在本 PRD 重复**。本 PRD 只管 PRD-011 之上的**决策治理层**。**Premium 独占**，不开放 Foundation / Advanced。**post-MVP，不阻塞当前 PRD-008/009/010/011 研发**。

### 1. 背景与价值（差异化壁垒）

传统容灾产品是"**被动从权**"：客户既定规则（B 面）即最终执行依据，系统仅遵从、不挑刺，不会提示客户的盲区/标准偏差/风险缺口。

**典型盲区场景**：客户内部规范把 **ERP** 定为灾后第一恢复；但按 NIST/ISO 27001/Cyber Recovery 最佳实践，**企业邮件系统**（指令传递与组织沟通的核心枢纽）+ **CyberArk 密码金库**（权限信任根基础设施）应**同级、同优先级恢复**。传统产品不会发现这个缺口。

**本 PRD 的核心价值**：突破被动从权，构建「**标准基本盘 + 客户决策面**」双层智能决策——**标准做兜底 / AI 做引导 / 客户做终审 / 系统做落地 / 全程可追溯**。这是 **Kasten K10 当前没有的层**，是 SupKube Premium 的真壁垒。

### 2. 与 PRD-011 的边界（两层正交，详见 ADR-046）

- **A 面（标准基本盘，PRD-011 已交付评分内核）= 评分 + 盲区检测的权威，永不被覆盖**。Resilience Score 始终按 A 的 NIST/ISO 矩阵算 → **跨客户可横比**。
- **B 面（客户决策面，本 PRD 新建）= DRP/CRP 执行的权威**，经 AI 引导 + 客户终审 + 决策历史库沉淀。
- **"从权 B>A" 仅作用于执行层（恢复顺序），非评分层（分数）**——两层正交。

### 3. 功能（§4）

**4.1 盲区检测报告引擎**：用 A 面标准知识识别客户自定义规则的盲区/遗漏/合规偏差（如漏了邮件/密码库同级恢复），生成**可读建议报告** + 决策支撑，**不强制改**。

**4.2 决策历史库（Decision History Library）**：AI 枚举待决策项 → 暴露风险差异 → 引导客户**显性化决策** → 客户终审签字 → 沉淀为**可追溯/可审计/可复盘**的决策记录（who/what/when/基于哪条标准 vs 哪条客户规则/最终裁定）。复用 PRD-013 Four-Eyes 做高敏感决策的二次审批。

**4.3 DRP/CRP 编排**：灾难恢复计划（DRP）/ 网络弹性恢复计划（CRP）的恢复编排、任务调度、优先级排序，**以决策历史库为唯一执行准则**，不再单纯依赖通用标准或静态模板。

**4.4 风险决策框架工具箱（生成报告时精简调用）**：
- 风险打分定级：**RICE / RPN(FMEA) / LEC / FAIR**
- 国标合规基线：**NIST SP 800-30 / ISO 31000 / OCTAVE / CIA 三元组**
- 多规则冲突决策（客户从权 vs 行业标准）：**AHP 层次分析 / 决策树 / CBA 成本收益 / TOPSIS**
- 灾备 DR/CRP 恢复排序：**NIST CSF / PRA**

### 5. 版本边界与底层依赖（Premium 独占）

完整能力闭环依赖：CMDB 资产与配置基线自动读取 · 业务与恢复依赖关系智能解析引擎 · 安全与容灾行业知识库（A 面）· RAG 检索增强 · 向量库记忆与决策沉淀 · 自定义流程编排与决策固化引擎。整套 = Premium 独占壁垒。

### 6. 非自治（Rule F 铁律）

AI 只**枚举 + 引导 + 建议**，**绝不**强制覆盖客户决策或自动改集群。**客户终审签字是唯一执行依据**。

### 7. Non-Goals（v1 明确不做）

- ❌ 不做 PRD-011 已有的 A 面评分/解释（本 PRD 是其上层）。
- ❌ AI 不自动执行恢复（仍是推荐 + 人工确认）。
- ❌ 不做开放给 Foundation/Advanced 的下放（Premium 独占）。

### 8. 开放问题（待 Mars/研发排期时拍）

- Q1：决策历史库的存储载体（复用 PRD-008 嵌入式 audit store？还是独立）？
- Q2：盲区检测的"标准基线"知识如何版本化 + 与 A 面评分规则 `scoreRulesVersion` 的关系？
- Q3：DRP/CRP 编排是本产品内建，还是对接客户既有编排器（如 Cyber Recovery）？
- Q4：框架工具箱 v1 先落哪几个（建议 RICE + AHP + CIA + NIST CSF 四件套起步）？

### 9. 评审历史

| 日期 | 操作人 | 状态变化 | 反馈 |
|---|---|---|---|
| 2026-06-03 | Mars 决策 / Claude 起草 | — → **草稿（charter）** | 由 Mars "AI Advisor 与客户建议并引导定决策、依据决策执行"的 B>A 澄清派生（详见对话）。从 PRD-011 MVP 向上拆出的 Premium 决策治理层，架构根 = ADR-046 两层正交体系。**post-MVP，不阻塞当前研发**。待研发排期时细化 §4 + 拍 Q1-Q4。 |

---

