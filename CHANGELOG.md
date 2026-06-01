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
