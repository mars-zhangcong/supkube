# SupKube Roadmap

> Last updated: **2026-05-26**
> Current released version: **v0.9.1.0-alpha** （部署在 docker-desktop + aks-jumborca-dev，可公开 `helm install`）
> Public distribution: **https://charts.supkube.com/** （ACR + Azure Blob + Cloudflare Worker）
> Reference product: [Kasten K10 by Veeam](https://docs.kasten.io)

---

## TL;DR

1. **MVP 已 ship**: 备份 / 恢复 / 多集群跨集群恢复 / 3-2-1-1-0 / Object Lock / 多架构 / Helm 分发 / Preflight + EULA 全部到位。
2. **客户能装上**: `helm repo add supkube https://charts.supkube.com/` → `helm install` 在任何 amd64/arm64 K8s 集群跑通；ACR 匿名拉镜像；charts.supkube.com 走 Cloudflare Worker 反代 Azure Blob。
3. **当前阶段**: 从"功能堆砌"转到"商业化 + 合规 + 售前支持工具链"。下个 sprint = **v0.8.14 Log Viewer + Upload to Support**。

---

## 🎯 当前 MVP 完成度（卖货前盘点）

| 维度 | 状态 | Sprint |
|---|---|---|
| 核心备份/恢复链路 | ✅ 100% | v0.5 → v0.7.9 |
| 双 Schedule (Local + Cloud) + 3-2-1-1-0 | ✅ 100% | v0.8.10 + v0.8.12 |
| Object Lock + 不可变保留 | ✅ 100% | v0.8.12 LBS3 |
| Helm bundling (Velero/Dex/MinIO subchart) | ✅ 100% | v0.8.13 |
| 多集群管理 + 跨集群恢复 | ✅ 100% | v0.9.0 MC1-5 |
| DR Topology Dashboard | ✅ 100% | v0.8.12.5/6 |
| 制品分发 (helm repo + ACR + Worker) | ✅ 100% | v0.9.0.3 |
| 多架构 (amd64 + arm64) | ✅ 100% | v0.9.0.4 |
| Install UX (Preflight + EULA + Reference) | ✅ 100% | v0.9.1.0 |
| **Log Viewer + 排障支持** | 🟡 0% | **v0.8.14 = 下个 sprint** |
| **License 真实校验** | 🟡 placeholder | v0.9.2 |
| **Backup integrity check** | 🔴 0% | v0.8.15 |
| **Swagger REST API doc** | 🔴 0% | v0.8.16 |
| 企业安全栈 (EntraID/Vault/Kyverno) | 🔴 0% | v0.9.4 + v0.9.5 |
| 应用级恢复演练 (BPMN + Kanister) | 🔴 0% | v0.9.6 — 独创卖点 |
| 细粒度文件浏览 + 恢复 | 🔴 0% | v0.9.3 |
| KubeVirt VM 备份 | 🔴 0% | v0.9.7 (按需) |
| MCP Server (AI 集成) | 🔴 0% | v0.9.8 |

---

## 📋 12 项客户需求归化（Mars 2026-05-24 拍板 / 2026-05-26 更新状态）

| ID | 需求 | 当前状态 | 实际 Sprint |
|---|---|---|---|
| a | 多集群管理 + 跨集群恢复 | ✅ Done | v0.9.0 MC1-5 + v0.9.0.1/.2 polish |
| b | Immutability Repository | ✅ Done | v0.8.12 LBS3 (Object Lock governance) |
| j | 3-2-1-1-0 | ✅ Done | v0.8.12 LBS4 评分卡 |
| f | Helm 打包 Velero+Dex+MinIO | ✅ Done | v0.8.13 + v0.9.0.3 分发闭环 |
| d | 日志收集 + 查看器（含 c 在线 Case） | 🔲 Next | **v0.8.14 — 下个 sprint** |
| k1 | 备份数据校验 | 🔲 Backlog | v0.8.15 |
| h | Swagger REST API | 🔲 Backlog | v0.8.16 |
| g | 许可证管理器（前端） | 🔲 Backlog | **v0.9.2** (从 v0.9.1 重编号) |
| c1 | 细粒度文件浏览 + 恢复 | 🔲 Backlog | v0.9.3 |
| l1 | 企业安全栈 P1（EntraID + Vault） | 🔲 Backlog | v0.9.4 |
| l2 | 企业安全栈 P2（Kyverno） | 🔲 Backlog | v0.9.5 |
| k2 | 应用级恢复演练 | 🌟 Backlog | v0.9.6 — 独创卖点 |
| c2 | KubeVirt VM 备份恢复 | ⏸ 按需 | v0.9.7 |
| i | MCP Server | 🔲 Backlog | v0.9.8 |

### 🟡 重要不紧急 Backlog

| ID | 内容 | 触发条件 |
|---|---|---|
| **e** | 完整在线 Case 系统 | v0.8.14 已有"手动开 Case"入口，Case API 就绪后无缝升级 |
| **SEC5** | Microsoft Sentinel webhook (SIEM 接入) | SIEM 接口完成 + 客户启用 Sentinel 时 |
| **CI** | GitHub Actions 自动 publish | publish-release.sh 跑稳 2-3 轮后上 CI |
| **TELEMETRY** | preflight.sh 接收方匿名遥测 | 数据驱动 product-led growth 决策时 |
| **DOCSITE** | VitePress + 中英双语文档站 | v1.0 GA 准备时 |

⏳ 待 Mars 提供：License Server API 规格 (v0.9.2 后端实现时)

---

## v0.8.14 — Log Viewer + Upload to Support （下个 sprint，~3 天）

**决策**：MVP 售前优先项。客户出问题时**自助排障 + 一键推 log 包给我们**是商业化前提。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **LV1** | 1.0d | 后端 `GET /api/v1/logs` 流式接口 (kubectl logs 包装) + component 选择器 + since/tail 参数 + admin-only |
| **LV2** | 1.0d | 前端 LogViewer.vue 页面：组件下拉 + 时间窗 + tail + 实时跟随 + 关键词高亮 + 下载 |
| **LV3** | 0.5d | Action Detail Drawer / Restore Drawer 加 "View Logs" 链接直跳过滤好的日志页 |
| **LV4** | 0.5d | "Upload to Support" 弹窗 — 一键打 log bundle (backend+frontend+dex + 所选 action 元数据)；调用 EULA 里填的 email 默认；后续可接 Case API |

**对标 Kasten**: Settings → Logs + "Generate Diagnostic Report"。

---

## v0.8.15 — 备份数据校验 （~2 天）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **BV1** | 1.0d | 后端：Backup 完成后异步调 `velero backup-location get` + `kopia repository validate`；输出 checksum 一致性报告 |
| **BV2** | 1.0d | 前端：RP 抽屉新 "Integrity Status" tab；周期性 cron 触发 deep check；Health Score 计入 |

---

## v0.8.16 — Swagger REST API 文档 （~2 天）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SW1** | 1.0d | 引入 `gin-swagger`；现有所有 handler 加 `// @Summary` `// @Param` 注释 → 自动生成 OpenAPI 3.0 spec |
| **SW2** | 0.5d | 挂载 `/api/v1/swagger/*` → Swagger UI；admin-only |
| **SW3** | 0.5d | Settings 新 "API Tokens" 页 (v0.8.5 后端已就绪，补 UI) |

---

## v0.9.2 — 许可证管理器前端 （~3 天，**前端独立可启动**）

> ⚠️ **版本号说明**：原 roadmap 上 v0.9.1 = License Manager，但 v0.9.1.0 已被 Install UX 占用。License Manager rebadge 到 **v0.9.2**，避免后续混乱。

**设计来源**：Kasten Licenses 页 1:1 复刻。先用 mock data 跑通完整 UI，Mars 后端 license server 就绪后零 UI 改动接入。

### Kasten Licenses 页结构（要复刻的元素）

| 区块 | SupKube 实现 |
|---|---|
| 顶部 3 卡：Node Count / License Summary / Compliance Status | ✅ 复刻；mock data |
| Multi-Cluster Lease 块 (Valid + Licenses contributed to pool) | ✅ 复刻；和 v0.9.0 MCM 联动 |
| Installed Locally 卡片列表 (icon + UUID + Status + Expires + Type + Node Limit + 删除) | ✅ 复刻 |
| Create New License 蓝色按钮 (key 输入弹窗) | ✅ 复刻 |
| Node Usage History 折线图 (按月) | ✅ ECharts |

### Mock 后端 API spec

```
GET    /api/v1/license/summary       → { nodeCount, licenseSummary, complianceStatus, multiClusterLease }
GET    /api/v1/license               → [{ id, name, uuid, status, expiresAt, type, nodeLimit }]
POST   /api/v1/license               → 提交 license key 文本 / file upload
DELETE /api/v1/license/:id
GET    /api/v1/license/usage-history → [{ month, peakNodeCount }]
```

`cm/supkube-eula` 已有 `licenseKey` 字段——前端直接读用作展示种子。

### 子任务

| 子任务 | 估算 | 内容 |
|---|---|---|
| **LM1** | 1.5d | Settings → Licenses 页（复刻 Kasten 整页布局）+ mock data 后端 endpoint |
| **LM2** | 0.5d | Create New License 弹窗 (key 输入 + file upload) + 删除确认 |
| **LM3** | 0.5d | License Activation 拦截页（首次启动 / License 过期时全屏阻断 UI，强制输入 license） |
| **LM4** | 0.5d | 功能门控 helper：`canUseFeature(feature)` 按 license type 返回 true/false；超 nodeLimit 时降级 |

**交付物**：前端 100% 完成，跑在 mock data 上可演示给客户。Mars 后端就绪后只换 endpoint 实现，不动任何 UI。

---

## v0.9.3 — 细粒度文件浏览 + 恢复 （~5 天，Kopia-only）

**决策**：仅支持 **Kopia 卷**（即 Data Mover / FS Backup 写到 BSL 的备份）；CSI 卷需先 restore 整个 PVC 再 mount，本期不做。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **FB1** | 2.0d | 后端 `POST /api/v1/restore-points/:name/browse` → 启动临时 Kopia mount pod → 返回 mount path + session token |
| **FB2** | 2.0d | 前端：RP 抽屉加 "Browse Files" 按钮 → file tree (Element Plus Tree) + 多选 + 路径搜索 |
| **FB3** | 1.0d | 选中文件后：Restore to PVC (目标 PVC + 路径) / Download as zip / Copy 至本地 PVC |

---

## v0.9.4 — 企业安全栈 Phase 1：EntraID + Vault （~5 天）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SEC1** | 2.0d | Dex Connector 加 Microsoft Graph (EntraID)；UI 提供 EntraID Tenant ID / Client ID 配置；Keycloak 已有，不动 |
| **SEC2** | 2.0d | Vault 集成 (合并原 #52)：Azure KeyVault & HashiCorp Vault 二选一；Settings 配置 endpoint + auth；BSL credentials / license key / DR passphrase 改从 Vault 读 |
| **SEC3** | 1.0d | MFA：EntraID Conditional Access 已支持 MFA，我们文档说明 + UI 显示当前用户 MFA 状态 |

---

## v0.9.5 — 企业安全栈 Phase 2：Kyverno （~3 天，SIEM 移到 backlog）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SEC4** | 2.0d | Kyverno Policy 模板 (默认 disabled，UI 一键启用)："PVC 必须关联 BackupPolicy" / "敏感 ns 必须 immutable" / "删 RP 必须 admin approve" |
| **SEC5** | 1.0d | Audit Event 输出 channel 抽象层：现落 K8s Event，后续可插 Sentinel / SOAR / 其他 SIEM；UI 配置 endpoint 占位 (disabled) |

> **Sentinel webhook 实现** → backlog (客户启用 Sentinel 时再做)

---

## v0.9.6 — 应用级恢复演练 （~7 天，独创卖点）

**决策**：编排 = **BPMN 画布 + Activity + Kanister Blueprint** 三层。Kasten 没这能力。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **RV1** | 2.0d | 后端：新 CRD `RestoreVerification`，spec 包含 BPMN XML + 依存图 + readiness 判定规则 |
| **RV2** | 2.5d | 前端：**BPMN.io 画布**集成，节点类型 = Activity (备份 / 恢复 / 等 readiness / 检查 endpoint / 触发 Kanister)，可拖拽编排 |
| **RV3** | 1.5d | 执行引擎：临时 ns 部署 → 按 DAG 执行 → 每节点超时控制 → 全部完成生成 PDF 报告 → 清理临时 ns |
| **RV4** | 1.0d | UI 报告页：执行历史 + 每次成功率 + 失败节点详情 + 下载报告 |

---

## v0.9.7 — KubeVirt VM 备份恢复 （~8 天，按需）

**触发条件**：客户真用 KubeVirt 时启动。

---

## v0.9.8 — MCP Server （~4 天，前瞻 + AI 集成）

**目标**：Claude / OpenClaw 等 AI 助手直接调 SupKube。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **MCP1** | 1.5d | MCP 协议实现 (gomcp) + stdio/SSE transport |
| **MCP2** | 1.5d | 工具集：`list_backups` / `create_restore` / `get_compliance_status` / `browse_files` / `get_logs` |
| **MCP3** | 1.0d | 文档：Cursor / Claude Desktop / OpenClaw 接入示例 |

---

## v1.0 GA 准备

- [ ] Prometheus metrics 完整化 (前期零散加的合并)
- [ ] Webhook 通知 (备份失败 / RP 损坏 → 自定义 webhook)
- [ ] **文档站** (VitePress + 中英双语) — backlog DOCSITE 升级到 GA 关键路径
- [ ] **GitHub Actions CI 自动 publish** — backlog CI 升级到 GA 关键路径
- [ ] **匿名遥测** — backlog TELEMETRY 升级到 GA 关键路径
- [ ] OCI registry 二级镜像 (ghcr.io / quay.io) — backup if Azure 不可达
- [ ] 性能压测 + 调优报告

---

## 📚 已完成 Sprint 列表（逆序）

| Sprint | 完成日期 | 主要内容 | Git tag |
|---|---|---|---|
| **v0.9.1.0** | 2026-05-26 | **Install UX**：preflight.sh (10 项 cluster 检查) + EULA gate + USER_MANUAL §23 Install Reference + image.registry airgap override | `v0.9.1.0-alpha` |
| **v0.9.0.4** | 2026-05-26 | **Multi-arch**：docker buildx amd64+arm64 manifest list；Dockerfile TARGETARCH；客户零参数适配 ARM64 集群 | `v0.9.0.4-alpha` |
| **v0.9.0.3** | 2026-05-25/26 | **制品分发闭环**：ACR (Standard SKU + anonymous pull) + Azure Blob Static Website + Cloudflare Worker (`charts.supkube.com`) + hack/publish-release.sh + 相对 URL index.yaml；SemVer 翻译 | `v0.9.0.3-alpha` |
| v0.9.0.2 | 2026-05-25 | MCM Dashboard 专属页 (/multicluster) + Mode Switcher 3 bug 修 (折叠 dropdown / 居中 / MCM 跳转) | (无独立 tag) |
| v0.9.0.1 | 2026-05-25 | 4 个 MC UX 修：this-cluster 自显 + 单集群 dropdown + 跨集群读路由 + kebab 安装命令 | (无独立 tag) |
| v0.9.0 MC1-5 | 2026-05-24/25 | Cluster CRD + health controller (60s) + Cross-Cluster Restore + BSL auto-sync + DR Playbook (USER_MANUAL §22) | (无独立 tag) |
| v0.8.13 | 2026-05-24 | Helm bundling: Velero subchart 必装 / Dex/MinIO opt-in / Settings → Plugins tab | (无独立 tag) |
| v0.8.12.5/6 | 2026-05-24 | DR Topology Dashboard 组件 (自研 SVG + 折叠 + 3-2-1-1-0 score 显示) | (无独立 tag) |
| v0.8.12 LBS1-4 | 2026-05-24 | 本地 MinIO BSL + Policy UI "Snapshot/Export → Local/Cloud" + Object Lock UI + 3-2-1-1-0 评分卡 | (无独立 tag) |
| v0.8.11.2 | 2026-05-24 | 登录故障修复 (eager fetchBranding 401 中断 OIDC callback) | (无独立 tag) |
| v0.8.11 | 2026-05-23 | White-label / OEM 品牌定制 (Logo + 产品名 + Favicon + Color Scheme) | (无独立 tag) |
| v0.8.10.x | 2026-05-23 | UI 规范 v1 + tokens.css + Kasten 风格 drawer + Application/Policy 抽屉对齐 + USER_MANUAL §21 kubectl 速查 | (无独立 tag) |
| v0.8.10 | 2026-05-23 | L1/L2 双 Schedule policypair 控制器 (snapshot 完成立刻触发 export) | (无独立 tag) |
| v0.8.8-9 | 2026-05-22 | Orphan GC + Backup error 可见性 + Application 一键 Snapshot | (无独立 tag) |
| v0.8.6-7 | 2026-05-22 | 备份组成元数据 (DataPath chip + Volume/Tarball 字节) + 跨集群跨云灾备 (Data Mover + Azure 落地) | (无独立 tag) |
| v0.8.0-5 | 2026-05-21/22 | 安全多租户：**OIDC + Dex + RBAC** 4 角色 + 4 大 IdP 集成 + 审计日志 + Basic Auth + 静态 API Token | (无独立 tag) |
| v0.7.8-9 | 2026-05-21 | Policies Kasten 风格重构 + 真级联删除 (DeleteBackupRequest CRD) | `v0.7.9-alpha` |
| v0.7.5-7 | 2026-05-20 | Backup Advisor MVP + 人类可读 schedule + Advisor i18n | `v0.7.5-alpha` |
| v0.7.0-4 | 2026-05-19/20 | Kasten Actions Model、Capability 检测、BSL Sync 状态、Dashboard ECharts、暗色模式、i18n | `v0.7.3-alpha` / `v0.7.4-alpha` |
| v0.6.0 | 2026-05-18 | Phase 2 核心：VSL 管理、Volume Backup Mode 双选 (FS/CSI)、CSI 进度展示 | (无独立 tag) |
| v0.5.0-2 | 2026-05-15/18 | Phase 1 PRD 7 页面骨架 + P0/P1 修复 + Kasten 风格 sidebar/logo + SupVault provider + CSI 快照基础设施 | (无独立 tag) |

> v0.7.10 → v0.9.0.2 时段有大量内部 sprint，未单独打 tag；2026-05-26 的 `v0.9.0.3-alpha` 大 checkpoint commit (`50485c2`) 一次性收纳了这段时间的 79 个文件改动。

---

## 关键架构决策（已定，不再纠结）

| 决策 | Sprint | 决策内容 |
|---|---|---|
| 卷数据备份模式 | v0.6 | Restic/Kopia + CSI 快照，**双模式并行** |
| Hooks 引擎 | v0.9 | **直接集成 Kanister**（不自研） |
| 多集群架构 | v0.9.0 | **Hub-Spoke**；Cluster CRD = `clusters.supkube.io`；kubeconfig 存 K8s Secret；60s health controller |
| 认证方案 | v0.8.5 | **OIDC via Dex**（不自建本地用户系统）；4 角色 RBAC；4 IdP 集成 |
| 本地快照战略 | v0.8.12 | **方案 B**：放弃 "L1 Snapshot Only"（Velero v1.18 无条件删 VSC 打破承诺）；改 Local MinIO BSL = "fast local copy"，Cloud BSL = "off-site copy"；Object Lock 实现 3-2-1-1-0 的 "1 immutable" |
| 制品分发架构 | v0.9.0.3 | **Azure 一栈**：ACR (Standard, anonymous pull) + Blob Static Website + Cloudflare Worker 反代；index.yaml 相对 URL → 未来切 CDN/域名零成本 |
| 镜像架构 | v0.9.0.4 | **multi-arch buildx** (linux/amd64 + linux/arm64)；客户零 `--platform` 参数 |
| EULA + License | v0.9.1.0 | helm template fail-fast gate + 接受后写 `cm/supkube-eula` 留存元数据；license key alpha 阶段任意字符串通过 |
| 编排引擎 (v0.9.6) | TBD | **BPMN.io 画布 + Activity + Kanister Blueprint** 三层 |

---

## Kasten K10 对标功能映射

| Kasten 模块 | SupKube 当前 | 备注 |
|---|---|---|
| Multi-Cluster Manager | ✅ v0.9.0 | Mode Switcher + Clusters 页 + MCM Dashboard |
| Distributions (Global Policy/Profile 分发) | 🔲 v0.9.10+ | 非 MVP 关键 |
| Bootstraps (token-based cluster join) | 🔲 v0.9.x | MVP 用 kubeconfig 上传 |
| Licenses 页 | 🔲 v0.9.2 | 1:1 复刻 |
| Backup Policies | ✅ v0.8 | 双 schedule (Local + Cloud) |
| Application Items | ✅ v0.8.10 | Workloads/Configuration/Networking/Storage/RBAC 分组 |
| Activity (统一 Action 流) | ✅ v0.8.0 | /actions endpoint，Backups + Restores 合并 |
| Restore Points | ✅ v0.8.10.4 | 表精简 + Profile 跳转 |
| Compliance Score | ✅ v0.8.12 LBS4 | 3-2-1-1-0 |
| Pre-flight Check (k10_primer.sh) | ✅ v0.9.1.0 | 10 项；charts.supkube.com 托管 |
| EULA + Install Options | ✅ v0.9.1.0 | helm template fail-fast |
| Advanced Install Options doc | ✅ v0.9.1.0 | USER_MANUAL §23 |
| Settings → Logs | 🔲 v0.8.14 | 下个 sprint |
| Generate Diagnostic Report | 🔲 v0.8.14-LV4 | 下个 sprint |
| Bundled Velero | ✅ v0.8.13 | **比 Kasten 强**——Kasten 让客户自装 Velero |
| White-label / Branding | ✅ v0.8.11 | Logo + 名 + Favicon + Color Scheme |
| Kanister Blueprints / Hooks | 🔲 v0.9.6 | 编排引擎一起做 |
| KubeVirt | 🔲 v0.9.7 | 按需 |

---

## 历史 Sprint 详细（v0.5.x → v0.8.11 完整记录）

> ⚠️ 这部分是 archive。新读者直接看上面"📚 已完成 Sprint 列表"即可。详细内容保留在 [git log](https://github.com/mars-zhangcong/supkube/commits/) 和各 ADR 文档中：
> - ADR-025: 双 schedule pair (v0.8.10)
> - ADR-026: Velero v1.18 VSC 删除行为 (v0.8.10)
> - ADR-028: L1 Snapshot Only 战略放弃 (v0.8.11)
> - ADR-029: Local MinIO BSL 战略选定 (v0.8.12)
> - 架构设计.md §1-12: 全量架构决策
> - docs/SPRINT-v0.5.1-RETRO.md: 早期 sprint 复盘

如需重读老 ROADMAP：`git show 4e5c2dc:ROADMAP.md`（v0.7.9-alpha 之前的版本）
