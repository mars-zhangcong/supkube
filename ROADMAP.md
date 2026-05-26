# SupKube Roadmap

> Last updated: **2026-05-26**
> Current released version: **v0.9.1.2-alpha** （部署在 docker-desktop + aks-jumborca-dev，可公开 `helm install`）
> Public distribution: **https://charts.supkube.com/** （ACR + Azure Blob + Cloudflare Worker）
> Reference product: [Kasten K10 by Veeam](https://docs.kasten.io)

---

## TL;DR

1. **MVP 已 ship**: 备份 / 恢复 / 多集群跨集群恢复 / 3-2-1-1-0 / Object Lock / 多架构 / Helm 分发 / Preflight + EULA / UI 重构 全部到位。
2. **客户能装上**: `helm repo add supkube https://charts.supkube.com/` → `helm install` 在任何 amd64/arm64 K8s 集群跑通；ACR 匿名拉镜像；charts.supkube.com 走 Cloudflare Worker 反代 Azure Blob。
3. **当前阶段**: 从"功能堆砌"转到"商业化 + 合规 + 售前支持工具链"。当前 sprint = **v0.8.14 Log Viewer + Download Logs + AirGapped 真支持**。
4. **导航**：用下面的"优先级矩阵"决定下一步做什么；用"已完成 Sprint 列表"看历史；用 [PRODUCT-TIERS.md](PRODUCT-TIERS.md) 解释商业模型。

---

## 🧭 优先级矩阵（重要-紧急 × 产品价值 × 客户要求 × 市场预期）

> 评分制度：**产品价值** (PV) × **客户要求** (CR) × **市场预期** (ME)，各 1-5 分。总分 = PV + CR + ME（满 15）。象限按下面的边界划分；象限内按总分降序。
>
> 决策原则：**P0 必须做完才能上下一阶段商业**；P1 安排到三月内的 sprint；P2 顺手在 P0/P1 间隙做；P3 等触发条件出现。

```
                                  紧急 (Urgent)
                                       ▲
                                       │
        ┌──────────────────────────────┼──────────────────────────────┐
        │                              │                              │
        │  P0 重要紧急 (Do Now)        │  P2 紧急不重要 (Quick Wins)  │
        │  ──────────────────────      │  ─────────────────────────   │
        │  · v0.8.14 Log Viewer        │  · CI/CD GitHub Actions      │
        │  · v0.9.2 License Manager    │  · Backend version 注入收尾  │
        │  · v0.9.1.3 Storage Class    │  · DOCSITE (VitePress 起步)  │
        │    Mgmt 集群管理 tab         │  · v0.9.0.5 #68 Force-Delete │
        │                              │                              │
重要 ◀──┼──────────────────────────────┼──────────────────────────────┼──▶ 不重要
        │                              │                              │
        │  P1 重要不紧急 (Schedule)    │  P3 不紧急不重要 (Wait)      │
        │  ──────────────────────      │  ─────────────────────────   │
        │  · v0.9.6 BPMN 灾备演练 ★    │  · v0.9.7 KubeVirt VM        │
        │  · v0.9.4 EntraID + Vault    │  · Sentinel webhook          │
        │  · v0.9.5 Kyverno            │  · Case API 完整集成         │
        │  · v0.9.3 文件浏览           │  · TELEMETRY 数据收集        │
        │  · v0.9.8 MCP Server ↑       │  · OCI 镜像备份 (ghcr/quay)  │
        │  · v0.9.10 Catalog Service   │                              │
        │  · v0.8.15 Backup integrity  │                              │
        │  · v0.8.16 Swagger API       │                              │
        │  · v0.9.11 Config Backup     │                              │
        │                              │                              │
        └──────────────────────────────┼──────────────────────────────┘
                                       │
                                       ▼
                                 不紧急 (Not Urgent)

  ★ = Premium tier 独创卖点
  ↑ = 本次 review 升优先级（原 v0.9.8 → 提到 P1 因 Veeam Intelligence 落地证明 AI 已成标配）
```

### P0 — 重要紧急（必须在 v1.0 GA 前完成）

| Item | PV | CR | ME | 总 | 工程量 | Sprint | 当前状态 |
|---|---|---|---|---|---|---|---|
| **Log Viewer + Download Logs + AirGap 真支持** | 5 | 5 | 5 | **15** | 7d | v0.8.14 | 🟢 进行中 (#79) |
| **License Manager 前端** | 5 | 5 | 4 | **14** | 3d | v0.9.2 | 🔲 (#63) |
| **Storage Class Mgmt 集群管理 tab** | 4 | 5 | 4 | **13** | 1d | v0.9.1.3 | 🔲 客户已提 |
| **#68 Force-Delete 卡住的 Backup** | 3 | 4 | 3 | **10** | 0.5d | v0.9.0.5 | 🔲 (#68) |

**为何 P0**：以上 4 项**任何一个未完成都直接卡商业化路径**。Log Viewer 是客户出问题排障的唯一手段；License Manager 是收钱前提；Storage Class Mgmt 是客户提的；Force-Delete 是真实故障数据可能踩到的点。

**v0.8.14 scope 扩张说明 (2026-05-26 客户反馈后)**：原 5d Log Viewer 扩到 7d，新增 LV4 改名（Upload to Support → Download Logs）+ LV8 真 AirGap 支持。详见 §v0.8.14 sprint 表。原因：本地化 + AirGapped 客户群（中国/日本中型企业）是同一群人，他们既不能上传 log（无公网出口）也不能拉公网镜像（防火墙）。两件事在同一 sprint 一起做，叙事完整。

### P1 — 重要不紧急（三月内做完，按总分排序）

| Item | PV | CR | ME | 总 | 工程量 | Sprint | 当前状态 |
|---|---|---|---|---|---|---|---|
| **BPMN 应用级恢复演练 ★** | 5 | 3 | 5 | **13** | 7d | v0.9.6 | 🔲 Premium 独创 |
| **EntraID + Vault (合规 P1)** | 4 | 4 | 5 | **13** | 5d | v0.9.4 | 🔲 标书必备 |
| **Kyverno (合规 P2)** | 4 | 4 | 4 | **12** | 3d | v0.9.5 | 🔲 |
| **细粒度文件浏览 + 恢复** | 4 | 4 | 4 | **12** | 5d | v0.9.3 | 🔲 |
| **MCP Server (AI 集成) ↑** | 4 | 3 | 5 | **12** | 4d | v0.9.8 | 🔲 升优先级 |
| **Catalog Service + Fleet Lifecycle** | 5 | 4 | 3 | **12** | 8d | v0.9.10 | 🔲 客户战略需求 |
| **Backup integrity check** | 3 | 3 | 4 | **10** | 2d | v0.8.15 | 🔲 |
| **Swagger REST API** | 3 | 3 | 4 | **10** | 2d | v0.8.16 | 🔲 |
| **Configuration Backup (dogfooding)** | 4 | 2 | 3 | **9** | 5d | v0.9.11 | 🔲 |

**P1 排序逻辑**：
- **BPMN v0.9.6** 仍是最高 P1 — 这是 Premium 独创卖点，长期看决定旗舰 SKU 能不能定价上去。
- **EntraID + Vault v0.9.4** 排在前面是因为**标书必备**——金融/政企客户没这个直接 disqualify。
- **MCP Server ↑** 从原 v0.9.8 P3 升到 P1：Veeam Intelligence 已在 VBR 落地，AI 已成 backup 产品标配。
- **Catalog Service v0.9.10** 来自客户战略需求（统一管理 install + preflight + debug），架构早定避免后期重写。

### P2 — 紧急不重要（顺手做，不阻塞主线）

| Item | PV | CR | ME | 总 | 工程量 | 触发条件 |
|---|---|---|---|---|---|---|
| **CI/CD GitHub Actions 自动 publish** | 2 | 1 | 3 | **6** | 2d | publish-release.sh 跑稳 3 轮 |
| **Backend hardcoded version 收尾** | 1 | 1 | 1 | **3** | 0.5h | 任何 sprint 顺手做 |
| **DOCSITE (VitePress) 起步** | 3 | 1 | 3 | **7** | 3d | 客户 onboarding 量起来时 |

### P3 — 不紧急不重要（触发条件出现再做）

| Item | PV | CR | ME | 总 | 触发条件 |
|---|---|---|---|---|---|
| **v0.9.7 KubeVirt VM 备份** | 4 | 1 | 2 | 7 | 客户真用 KubeVirt 时 |
| **Microsoft Sentinel webhook** | 2 | 1 | 2 | 5 | 客户启用 Sentinel SIEM |
| **完整 Case API 集成** | 2 | 1 | 1 | 4 | Mars 提供 Case API spec |
| **TELEMETRY 匿名遥测** | 3 | 0 | 1 | 4 | 需要数据驱动决策时 |
| **OCI 镜像备份 (ghcr/quay 二级)** | 1 | 1 | 1 | 3 | Azure 出问题时备份方案 |

### 评分校准说明

| 维度 | 1 分 | 3 分 | 5 分 |
|---|---|---|---|
| **产品价值 (PV)** | 边缘特性 / 偶尔用 | Foundation 层正常完整 | Premium 独有或差异化关键 |
| **客户要求 (CR)** | 没人提过 | 1-2 个客户问过 | 多个客户 / 当前 demo 客户明确要求 |
| **市场预期 (ME)** | 客户不期待 | 行业有这能力 | Kasten / Veeam 已落地，不做就掉队 |

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

## v0.8.14 — Log Viewer + Download Logs + AirGapped 真支持 （进行中 sprint，~7 天）

**决策**（2026-05-26 客户反馈后扩张）：MVP 售前优先项。**本地化 + AirGapped 客户群是同一群人**——他们既不能上传 log（无公网出口）也不能拉公网镜像（防火墙）。两件事在同一 sprint 一起做，叙事完整。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **LV1** | 1.5d | 后端 `GET /api/v1/logs` SSE 流式接口；**覆盖 Velero ns** (deploy/velero + ds/node-agent) + kube-system snapshot-controller；server-side facet count；`/api/v1/backups/:name/velero-logs` 走 Velero DownloadRequest 协议 |
| **LV2** | 1.5d | 前端 LogViewer.vue Datadog 风：facet 侧栏 (Component/Severity/Pod 带 counts) + 彩色 severity 徽章 + 时间窗 + Live tail (⏸▶) + 行展开 + 关键词高亮 |
| **LV3** | 0.5d | Action Detail / Restore Drawer 加 "View Logs" 链接，跳过滤好的 backup-specific log 视图 |
| **LV4** | 0.5d | **Download Logs** Modal (改名 from "Upload to Support") — Settings → System Information 页 + 内容 checkbox + 时间窗 + anonymize PII + 下载 tarball / 复制 CLI 命令 |
| **LV5** | 0.5d | Settings → Support Contact tab — admin 配置紧急联系热线 + 邮箱 + 在线支持 URL，存 `cm/supkube-support-contact`；写入 debug bundle 的 README 给客户内部 ticket 流程用 |
| **LV6** | 0.5d | `hack/supkube_debug.sh` (~150 行 bash) 仿 k10_debug.sh；charts.supkube.com 托管；publish-release.sh 集成自动同步 |
| **LV7** | 0.5d | Runbook patterns MVP (5-10 条 yaml-defined 规则 + 前端 fuzzy match) — Premium 知识库的种子 |
| **LV8** 🆕 | 1.5d | **AirGapped install 真支持**：`global.airgapped.repository`（替代 v0.9.1.0 假支持的 image.registry）+ `_helpers.tpl` supkube.image helper + 改所有 deployment templates 用 helper + Velero subchart 镜像 override + USER_MANUAL §24 完整离线安装指南 + 端到端验证 hack/airgap-bundle.sh |

**对标 Kasten**:
- Settings → Logs + "Generate Diagnostic Report" → 我们的 Download Logs Modal
- `k10_debug.sh` curl-bash → 我们的 `supkube_debug.sh`
- `global.airgapped.repository` Helm value → 我们直接复用同名（命名对齐，客户切换零成本）

**关键改名 (统一)**:
- "Upload to Support" → **"Download Logs"** (airgap 客户无法 upload；统一命名)
- `image.registry` (假字段) → `global.airgapped.repository` (真支持；Velero subchart 也透传)

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
| **v0.9.1.2** | 2026-05-26 | **UI 重构**：Sidebar 10→7 (Observability hub 合并 Activity/Advisor/Audit/Log Viewer)；备份顾问→可观测性，数据还原→应用还原；缩进对齐第三修 (CSS Grid)；存储位置 + 快照位置 退出侧边栏 (集群管理 tab 化 v0.9.1.3 做)；+ PRODUCT-TIERS.md (~430 行 商业模型) + hack/DEMO-cross-cluster-restore.md (~550 行 客户演示指南) | `v0.9.1.2-alpha` |
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
