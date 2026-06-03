# Changelog

本文件记录 SupKube 所有面向用户的变更。格式参照 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

> **版本号方案（重要，消除歧义）**：SupKube 有**两套并行版本号**，请勿混淆——
> - **产品版本（appVersion）**：四段式 `0.9.1.N-alpha`，对外宣称、镜像 tag、git tag 用它。
> - **Helm Chart 版本**：SemVer 兼容 `0.9.1-alpha.N`（因为 Helm 要求严格 SemVer）。
> - 对应关系示例：appVersion `0.9.1.9-alpha` ↔ chart `0.9.1-alpha.9`。
> - 制品：镜像 `supkube.azurecr.io/{backend,frontend}:<appVersion>`；Chart `charts.supkube.com/supkube-<chartVersion>.tgz`。
>
> 变更类型：`Added` 新增 · `Changed` 变更 · `Fixed` 修复 · `Security` 安全 · `Docs` 文档 · `Deprecated` 弃用 · `Removed` 移除。

---

## [Unreleased]

### Docs
- 建立 **术语表.md**（Glossary 单一来源）：收口 Resilience Score vs Posture、Layer 1-5 vs L1/L2、Snapshot/Export/Copy 三组高发术语撞车。
- 建立 **CHANGELOG.md**（本文件，补 D-21/task P2-6 欠债）、**ENGINEERING.md**（工程手册，落地 Rule A/B/C + verify-before-ship）。
- 建立 **API-REFERENCE.md**（13 组 ~90 端点目录 + 认证三模式 + RBAC 三角色 + ADR-035 错误信封，派生自 `rbac.go`）+ **openapi.yaml**（OpenAPI 3.1，cross-cutting 契约 + 核心资源种子）→ 部分填平 ENGINEERING.md §6「API 无契约」债。
- 建立 **RUNBOOK.md**（9 个故障域：node-agent hang / RP 删除卡住 / CSI VSC 缺失 / **防云账单 checklist** / force-delete / 镜像没滚出去 / RBAC 403 / air-gap / support bundle）。
- 建立 **SLO-RTO-RPO.md**（RPO 档位 + RTO 按 ADR-031 五层 + SupKube 产品 SLO + 规模上限；作为 PRD-011 Resilience Score 评分阈值单源）。
- **Dashboard 数据漂移整改**：`dashboard/data.js`（数据/视图分离）+ `dashboard/gen-data.mjs`（从 PRD.md 索引表 / ADR-LEDGER 派生 PRDS/ADRS + 漂移看门，exit code 可接 CI）+ index.html 升级（需求血缘视图 / 深链回源 MD / Blocked 标记 / 完成趋势 sparkline / Eisenhower 象限修正）。
- **PRD-011**（AI Backup Advisor MVP）+ **PRD-012**（Call Home / Auto-Support）立项（草稿）。
- **ADR-037**（统一数据采集架构：CollectionContract + Collector/Server 分离 + Canonical DSL + 三档连接形态）。
- 测试用例新增 **§19 TC-AI-MVP**（15 例，覆盖 PRD-011 13 条 DoD）。
- 修正 PRD-010 中"PRD-011"占位歧义 → 指向 PRD-013（MCM Dashboard，待立）。
- **正文回填（消除 §4.x 与 §12/§13 闭环段自相矛盾，2026-06-02）**：PRD-008 §4.1.2 存储选型（预倾向 CRD → 嵌入式 store on PV，否决 etcd 反模式，§13 D1）、§4.4.2 孤儿清理（无差别 mc rm → mode 参数区分 metadata-only / 带数据走 kopia maintenance，§13 D5）；PRD-010 §4.6/§4.7 Posture（"已启用层数 × 20" → PRD-007 §4.7 单一加权 score，§13 F1）；PRD-011 §4.6 API（`/ai/analyze` 同步 60s → 拆 `/ai/score` 同步<5s + `/ai/explain` SSE 异步，§12 H5）。均加"以 §12/§13 为准"指针，避免研发只读旧正文拿到被取代的指引。
- **PRD-review/INDEX.md 新增「三态放行跟踪表」**：把 PRD 放行从两态（finding 闭环 / 重审）拆成三态（finding 闭环 → **正文回填** → 可重审），铁律"②正文回填未完成不得进③重审"，根治"闭环段改了正文没跟"的复发。
- **二级残留回填（2026-06-03，AI 辅助深审 M-1~M-7）**：PRD-008 §11 Q1/§9 Phase0/DoD#12（存储选型 + mode 术语）；PRD-009 DoD 字段名对齐 CRD spec（continuous.pollInterval/lastPollAt/sourceClusterFilter.clusterId）+ 错误码统一（ERR_FINGERPRINT_INVALID）+ 失败阈值统一 N=5；PRD-010 §4.1/§5 Layer 5 节点→验证徽章、§4.2/DoD#5 箭头分类 sync→import 对齐 §13 F2。（深审另提出的 ERR_FINGERPRINT_TAMPERED/INTERVAL_TOO_SHORT 经核实不存在，未改 → verify-don't-trust。）

### Changed
- **PRD-008 / 009 / 010 / 011 评审通过 → 研发中**（Mars 2026-06-03）。状态同步 PRD.md 索引+正文、dashboard（gen-data 派生）、PRD-review/INDEX.md 三态表、等待决策.md。
- **PRD-011 评分细则定版（D-WAIT-002 resolved）**：Mars 否决简版 5 维（30/20/30/15/5），改用自定 **100 分制 4 维标准对标矩阵**——备份覆盖与合规 25（ISO 27002 §8.13.1.a）/ 3-2-1-1-0 韧性 35（NIST CSF）/ 防勒索与安全 20（NIST SP 1800-26）/ 成功率与可恢复性 20（NIST SP 800-53 CP-9）；含滑动窗口成功率公式 + WORM COMPLIANCE 自动校验断言 + 4 档安全级别（90/75/60）+ Q4 两硬阈值（无备份封顶 30 / 高分校准 30）。§4.2 重写，喂 ADR-043。

### Added
- **ADR-046 立项（Rule G 取号）**：AI 容灾决策**两层正交体系**——标准基本盘 A（评分+盲区检测权威，永不被覆盖，跨客户可比）+ 客户决策面 B（DRP/CRP 执行权威，经 AI 枚举→暴露差异→客户终审签字→决策历史库→唯一执行准则）。"从权 B>A" 仅指执行层非评分层（Mars 2026-06-03 澄清，纠正早前"B 覆盖评分规则"误述）；闭环=标准兜底/AI引导/客户终审/系统落地/全程可追溯。完整能力 = Premium 独占 + 超 PRD-011 MVP → **建议立 PRD-015（AI 容灾决策顾问）**。
- **PRD-015 立项（Rule G 取号）**：AI 容灾决策顾问（Premium 独占，从 PRD-011 MVP 向上拆出的决策治理层）——盲区检测报告 + 决策历史库 + DRP/CRP 编排 + 风险框架工具箱（RICE/RPN/AHP/TOPSIS/FAIR/OCTAVE…）；架构根 = ADR-046。charter 级草稿，**post-MVP 不阻塞当前研发**。
- **ADR-044/045 补登 架构设计.md ADR 台账**：二者早在 LEDGER.md（号源 SSOT）已登记（044=Fast Debug Mode 有正文；045=ApprovalPolicy 占号），但漏镜像进 架构设计.md 元数据表（gen-data 读这张表 → dashboard 此前缺这两号）。RCA = 双台账漂移（详见对话）。
- **PRD-011 §12 H1 / §11 Q1 过时文案回填**：D-WAIT-002 已 resolved（4 维矩阵）后，旧"30/20/30/15/5 + 待 Mars 拍 + 不能进研发"残留 → 标记已解决（与 M-1~M-7 同类）。
- **评分细则 + 两层体系 4 文档传播完成（并行 agent）**：架构设计.md §9 写入 **ADR-043**（评分矩阵正文）+ **ADR-046**（两层体系正文）并登记 ADR 台账；USER_MANUAL.md 新增 **§25 数据韧性评分解读**（客户向 4 维矩阵 + 4 档级别 + 提分建议）；测试用例.md §19 **TC-AI-MVP 升级**到新矩阵 + 新增 TC-AI-016~019（两硬阈值 / 滑动窗口 / WORM 断言 / 缺失字段，11 条 P0）；术语表.md 收口**风险等级 4 档（权威）↔ 旧 5 枚举（内部）映射**，并对齐 PRD-012 Call Home 触发条件。dashboard 经 gen-data 自动收录 ADR-043/046。

---

## [0.9.1.10-alpha] — 2026-05-31

### Fixed
- 6 项 demo P0 修复（BSL Secret Key UX、Force-delete 卡住的 Backup CR、Imported RP 0 artifacts、编辑策略 404 等）。
- PolicyAggregate 契约全量审计（防同源 bug，task #101）。

### Added
- Export-BSL 处理。

---

## [0.9.1.9-alpha] — 2026-05-31

### Added
- **Log Viewer v1.5**：错误摘要卡 / Prev-Next Error / Expand Context ±5 行 / 时间 Abs-Rel 切换 / Live Tail Lock / 搜索荧光 / 全屏（task #79）。
- **Velero 真自带**：CRD + plugin + 镜像入 ACR + CSI 自动（task #84）。
- **DevOps 盘点基建**：每日 AUDIT 文档 + 项目仪表板 HTML + `hack/dev-deploy.sh`（775 行，6 Phase，dual-arch + dual-cluster + 4 步真验证）。

### Fixed
- **双向灾备闭环验证**：本地 ↔ 云端 postgres 6 行零丢失回流。
- node-agent data path hang 根治方向：换官方 velero release（去 v1.16.0-dirty build）。

### Changed
- **velero v1.16.0 → v1.18.0 固化**（chart 9.0.4 → 12.0.1，task #102）。

---

## [0.9.1.4-alpha] — 2026-05-31

### Fixed
- Restore sidebar 菜单恢复。
- 默认 logo 修复。

---

## [0.9.1.2-alpha] — 2026-05-26

### Added
- **UI 重构**：Sidebar 10 → 7 + Observability hub。
- **PRODUCT-TIERS.md**（商业套餐分层）。
- DEMO guide。

---

## [0.9.1.0-alpha] — 2026-05-26

### Added
- **安装 UX**：Preflight Script + EULA gate + 安装参考（USER_MANUAL §23）。

---

## [0.9.0.4-alpha] — 2026-05-26

### Added
- **多架构镜像**（amd64 + arm64），via `docker buildx`。

---

## [0.9.0.3-alpha] — 2026-05-25

### Added
- **Multi-Cluster Manager + 跨集群恢复 Wizard**（Kasten 风格）。
- **制品分发闭环**：ACR + Azure Blob + Cloudflare Worker（`charts.supkube.com`）。

---

## [0.8.14-alpha] — 2026-05-24

### Added
- **AirGapped 安装真支持**（LV8 foundation）。
- **`supkube_debug.sh`**：面向客户的诊断包（diagnostic bundle，LV6）。

---

## 历史版本（0.5 ~ 0.8 摘要）

| 版本 | 摘要 |
|---|---|
| v0.8.x | Kasten 风格 Policies 页 + Run Once；DeleteBackup 真级联（DeleteBackupRequest CRD）；Snapshot + Export 分层抽象（Kasten Actions Model）。 |
| v0.7.x | Backup Advisor MVP + Advisor i18n；暗色模式全覆盖；i18n zh-CN/English；Dashboard 图表；BSL sync status + Imported card；Capability detection。 |
| v0.6.0 | CSI snapshot 路径 + VSL 管理 + 可折叠 sidebar。 |
| v0.5.x | MVP：备份/恢复/Storage CRUD + CSI snapshot 基建 + Kasten 风格打磨。 |

---

> **维护约定**：每次发布前，把 `[Unreleased]` 下累积的条目切到新版本号小节并标日期；发布脚本（`hack/dev-deploy.sh`）应在 bump 版本时提示更新本文件。详见 [ENGINEERING.md](ENGINEERING.md) 发布流程。
