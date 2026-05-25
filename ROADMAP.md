# SupKube Roadmap

> Last updated: 2026-05-24
> Current released version: **v0.8.11.2-alpha**（已部署到本地 docker-desktop K8S）
> Reference product: [Kasten K10 by Veeam](https://docs.kasten.io)

---

## 🎯 MVP 闭环（v0.8.12 + v0.9.0 共 12 天）

> **客户目标**：本地集群备份 → 云端存储桶 → 异地集群恢复。做完这两个 sprint 就能卖。
> 详细架构见 `架构设计.md` ADR-029。

| Sprint | 范围 | 工程量 | 依赖 |
|---|---|---|---|
| **v0.8.12** | 本地 MinIO BSL + Cloud BSL + 3-2-1-1-0（LBS1-4） | 5 天 | 无 |
| **v0.9.0** | Kasten-style 集群管理器 + 跨集群恢复 Wizard（MC1-4） | 7 天 | v0.8.12 |

**MVP 上线后**：客户可以做"在 A 集群备份 → 本地 MinIO（快速）+ 云端 Azure Blob（异地）→ 在 B 集群恢复"完整闭环。

---

## 📋 12 项客户需求归化（Mars 2026-05-24 拍板）

| ID | 需求 | 优先级 | Sprint | 工程量 | 决策 |
|---|---|---|---|---|---|
| a | 多集群管理 + 跨集群恢复 | 🔥 MVP | v0.9.0 | 7d | **Kasten Multi-Cluster Manager 风格**（mode switcher + Clusters 列表 + kubeconfig 上传），见截图分析 |
| b | Immutability Repository | 🔥 MVP | v0.8.12-LBS3 | — | Object Lock 默认 Governance（仅 MinIO 支持，Garage 无） |
| j | 3-2-1-1-0 | 🔥 MVP | v0.8.12-LBS4 | — | 评分卡 |
| f | Helm 打包 Velero+Dex+MinIO | 🔥 紧随 MVP | v0.8.13 | 2d | **Velero 必装**，Dex/MinIO Advanced Install 可选 |
| d | 日志收集 + 查看器（含 c 在线 Case） | 🔥 紧随 MVP | v0.8.14 | 3d | LV4 "Upload to Support" 内嵌——客户手动开 case 现状先用，等 Case API 接入再无缝升级 |
| k1 | 备份数据校验 | 🔥 紧随 MVP | v0.8.15 | 2d | checksum + Kopia repo check |
| h | Swagger REST API | 🔵 拉单 | v0.8.16 | 2d | gin-swagger 自动生成 |
| g | 许可证管理器（前端） | 🔥 收钱前提 | v0.9.1 | 3d | **1:1 复刻 Kasten Licenses 页**；mock 后端先跑通；Mars 后端跟上后零 UI 改动 |
| c1 | 细粒度文件浏览 + 恢复 | ⭐ 留客 | v0.9.3 | 5d | **Kopia-only** |
| l1 | 企业安全栈 Phase 1（EntraID + Vault） | 🔥 合规 | v0.9.4 | 5d | **EntraID/Keycloak + Azure KeyVault/Vault** |
| l2 | 企业安全栈 Phase 2（Kyverno） | 🔥 合规 | v0.9.5 | 3d | Kyverno only，**Sentinel 移到 backlog** |
| k2 | 应用级恢复演练 | 🌟 独创 | v0.9.6 | 7d | **BPMN 画布 + Activity + Kanister Blueprint** |
| c2 | KubeVirt VM 备份恢复 | ⭐ 差异化 | v0.9.7 | 8d | 客户真用 VM 时启动 |
| i | MCP Server | 🌟 前瞻 | v0.9.8 | 4d | 暴露给 Claude/OpenClaw |

### 🟡 重要不紧急（Backlog）

| ID | 内容 | 触发条件 | 备注 |
|---|---|---|---|
| **e** | 完整在线 Case 系统 | Mars 提供 Case API + 日志 sprint 完成 | v0.8.14 LV4 已经有"手动开 Case"的入口；Case API 就绪后无缝升级 |
| **SEC5** | Microsoft Sentinel webhook（SIEM 接入） | SIEM 接口完成 + 客户启用 Sentinel 时 | 不阻塞主线 |

⏳ 待 Mars 提供：Cluster CRD 结构（你截图集群跑的命令）、License Server API 规格（v0.9.1 后端实现时）

## 当前状态总览

| Sprint | 状态 | 主要内容 |
|---|---|---|
| v0.5.0 → v0.5.2 | ✅ 2026-05-15/18 | Phase 1 PRD 7 页面骨架 + P0/P1 修复 + Kasten 风格 sidebar/logo + SupVault provider + CSI 快照基础设施 |
| v0.6.0-alpha | ✅ 2026-05-18 | Phase 2 核心：VSL 管理、Volume Backup Mode 双选（FS/CSI）、CSI 进度展示 |
| v0.7.0 → v0.7.4 | ✅ 2026-05-19/20 | Kasten Actions Model、Capability 检测、BSL Sync 状态、Dashboard ECharts、暗色模式、i18n |
| v0.7.5 → v0.7.7 | ✅ 2026-05-20 | Backup Advisor MVP + 人类可读 schedule + Advisor i18n |
| v0.7.8 → v0.7.9 | ✅ 2026-05-21 | Policies Kasten 风格重构 + 真级联删除（DeleteBackupRequest CRD） |
| v0.8.0 → v0.8.5 | ✅ 2026-05-21/22 | 安全多租户：**OIDC + Dex + RBAC** 4 角色 + 4 大 IdP 集成（Keycloak/Okta/Azure AD/GitHub）+ 审计日志 + Basic Auth + 静态 API Token |
| v0.8.6 → v0.8.7 | ✅ 2026-05-22 | **备份组成元数据**：DataPath chip（CSI/FS/DataMover/MetadataOnly）+ Volume 字节 + Tarball 字节；**跨集群跨云灾备**：Data Mover + Azure 落地 |
| v0.8.8 | ✅ 2026-05-22 | **Orphan GC**：删 RP 后底层 VSC/VS/PVB/DataUpload 自动清理 + Settings 面板可视化 + K8s Event 审计 |
| v0.8.8.1 → v0.8.9.2 | ✅ 2026-05-22 | Backup error 可见性 endpoint + Action Details drawer error 渲染；双 Schedule 模型（L2 Policy = snapshot + export 两个 RP）；UNKNOWN 状态 fix；**Application 一键 Snapshot 按钮** |
| v0.8.10 → v0.8.10.1 | ✅ 2026-05-23 | **方案 B 配对控制器**：snapshot 完成 → 立即触发 export（数据时间差从 ~7 分钟降到 ~30 秒）+ Plan-C 升级三层预留；Restore Points 表 Type 三态互斥 + 列简化；ActionDetailDrawer 加 Paired-with banner + role chip + Policy Run At |
| v0.8.10.2 | ✅ 2026-05-23 | **UI 规范 v1**：`UI_GUIDELINES.md`（10 章 + audit）+ `tokens.css` 全局设计 token + ActionDetailDrawer 重写为 Kasten 模板（H1+H2+section H3+sticky footer）+ Application/Policy drawer 对齐统一 + USER_MANUAL §21 kubectl 速查 |
| v0.8.10.3 | ✅ 2026-05-23 | **Application Artifacts 与 Action Details 完全统一**（同 Workloads/Configuration/Networking/Storage/RBAC 分组手风琴）+ KUBECTL 命令行块加到 Action Details + 每 artifact 行 `</>` 按钮 → YAML 浮窗带语法高亮（自研轻量 highlighter）+ 后端 `/resources/yaml` 懒取端点 |
| v0.8.10.4 | ✅ 2026-05-23 | **Restore Points 表深度简化**：8 列减到 6 列；Type+Status chip 合并到 Namespace；Size 列改 actual/reserved 格式；Profile 列可点击跳 Storage；后端 `reservedBytes` 字段（从源 PVC `requests.storage` 算）—— 解决 Velero v1.18 删 VSC 导致 Snapshot RP 显示 "—" 的盲区 |
| v0.8.10.5 | ✅ 2026-05-23 | Policy View drawer **灰屏 bug 修复**（PolicyAggregate vs 扁平 Schedule 数据结构错配）+ Policies 表简化（删 Validation/Action/Last Run Status 3 列）+ Frequency "On Demand"（paused 时）+ 日期/时间分两行节省列宽 + Activity 卡去 "half" 后缀 + 全表去 emoji |
| v0.8.10.6 | ✅ 2026-05-23 | Policies **Resources 列回填**（每个 ns 一行垂直）+ Applications 简化（删 Status 列、Status chip 合到 Name、Labels 一行一个、Components 去图标、RP 数字去 folder 图标） |
| v0.8.11 → v0.8.11.1 | ✅ 2026-05-23 | **品牌定制（White-label / OEM）**：Settings → Branding 面板支持自定义产品名 + Logo + Favicon + Color Scheme（8 swatches，全局应用 `--sk-primary` 等 CSS 变量）；后端 `GET/PUT /settings/branding` 写入 `supkube-settings` ConfigMap（admin 限定）；前端 `useBranding` 响应式 store 实时联动 Sidebar/Header/Tab Title |
| **v0.8.11.2** | ✅ **2026-05-24** | **登录故障修复**：v0.8.11 引入的 `useBranding` 模块加载时 eager `fetchBranding()` 在 `/auth/callback` 页面也会触发 GET `/settings/branding`，该请求因尚无 token 返回 401 → axios 全局 401 拦截器跳转 `/login?reason=expired` → 中断进行中的 POST `/api/v1/auth/callback`（nginx log 显示 499）→ 死循环。修复：401 拦截器白名单 `/auth/callback`；`useBranding` 在 `/login` / `/auth/callback` 路由跳过 boot fetch；同时修复 `audit Event` 因 `involvedObject.namespace` 为空被 K8s 1.27+ validator 拒绝的噪声日志（fallback 到 `auditNamespace`） |
| **v0.8.12（计划）** | 🔲 **下一 Sprint** | **L1/L2 重构 → 本地 BSL + 云端 BSL（3-2-1-1-0）**：见下方"v0.8.12 重大方向"段落；预计 5 天；同步推进 #51 Restore SupKube DR + #52 HashiCorp Vault 集成 |
| v0.9（高阶能力） | 🔲 待启动 | Kanister Blueprints、Hub-Spoke 多集群、Compliance Score + Advisor v2 with Prometheus |
| v1.0（GA） | 🔲 待启动 | Prometheus metrics、Webhook 通知、文档站、Helm chart 发布到 OCI registry |

---

## v0.8.12 — 本地备份存储 + 3-2-1-1-0 重构（重大方向，~5 天）

> **背景**：v0.8.10 调查（ADR-026/028）确认 Velero v1.18 在 backup `Completed` 时无条件删 `VolumeSnapshotContent`，即使加了 `velero.io/csi-volumesnapshot-content-retain-policy=retain` 注解也无效。"L1 Snapshot Only = 秒级恢复"这条产品承诺事实上已经被上游打破——VSC 一删，所谓的本地快照只剩 metadata，没有可恢复的数据。
>
> **新方向**（Mars 2026-05-24 拍板）：放弃 Velero "Snapshot Only" 概念，重构为"双 BSL"模型——L1 = 备份到**集群内本地 BSL**（局域网快速恢复），L2 = L1 + **云端 BSL**（异地灾备）。两份都是完整 Velero Backup，与 VSC 是否被删无关。遵循 **3-2-1-1-0** 备份规则：3 copies / 2 media / 1 off-site / 1 immutable / 0 errors。架构对比与详细分析见 `架构设计.md` ADR-029（待写）。

### MVP 决策（Mars 2026-05-24 拍板）

| # | 决策项 | 选定 | 理由 |
|---|---|---|---|
| **1** | 本地 BSL 后端 | **MinIO** | Velero 官方 demo 后端，兼容性最稳；支持完整 S3 Object Lock；运维难度远低于 Ceph |
| **2** | 部署模式 | **B（可选启用）** | 用户在 Settings 里点 "Enable Local Backup Store" 按钮触发 helm upgrade；不强制所有客户多扛 200GB PVC |
| **3** | 存储后端 | **PVC** | 企业级 HA 要求；hostPath 单节点绑死不符合产品定位 |
| **4** | VSC 加速 | **保留** | Velero 内部仍先建 VSC 再 Kopia 上传到本地 BSL；VSC 完成后该删就删，不再依赖其持久存在。RPO ~30s 优于"完全重新读源"~1min+ |
| **5** | 上传策略 | **串行**（先本地，再云端） | 本地写完即返回"备份成功"，云端在后台异步拷贝；用户体验快、省网络带宽 |
| **6** | 客户事实问题 | **客户驱动迭代** | 不预设客户拓扑/数据量等假设；按需调整 helm values，不在 v0.8.12 强行设计成"普适方案" |

### 子任务拆解（共 5d）

| Task | 估算 | 范围 |
|---|---|---|
| **LBS1** | 2.0d | MinIO subchart（single binary 模式）+ 自动注入 Velero BSL CR + Object Lock 默认开 + access key 自动生成；Helm value `localStore.enabled` 控制启用 |
| **LBS2** | 1.5d | Policy UI 改造："Snapshot/Export" → "本地/云端" 两个 BSL 选择器；后端 schedule 字段映射调整；policypair controller annotation 改名（`snapshot|export` → `local|cloud`） |
| **LBS3** | 1.0d | Object Lock UI：BSL 详情页显示不可变保留期、Storage Profile 卡片加 🛡 chip；Governance vs Compliance 单选 |
| **LBS4** | 0.5d | 策略 Wizard 加 **3-2-1-1-0 健康度评分卡**（你的策略满足了几条规则）+ Dashboard "Protection Compliance" 卡片更新 |

> **Provider 决策（2026-05-24 锁定）**：唯一支持 **MinIO**。Garage 评估后砍掉——Mars 现有 Mac M3 Pro + 18GB RAM 跑 MinIO 已稳定 10 天，没有"laptop 太重"的实际问题；Garage 砍掉换来一份代码、一套测试、一种部署形态，工程性价比更高。

### 3-2-1-1-0 在 SupKube 中的落地映射

| 规则 | 实现 |
|---|---|
| **3 副本** | 源 PVC + 本地 MinIO BSL + 云端 BSL（S3/OSS/Azure Blob） |
| **2 介质** | 本地块存储 PVC（SSD/HDD）+ 云对象存储 |
| **1 异地** | 云端 BSL（不同 region 或不同 cloud 更佳） |
| **1 不可变** | 本地 MinIO Object Lock + 云端 BSL Object Lock；策略保留期内不可删 |
| **0 错误** | Velero 备份完成后做 checksum；策略可选每周一次 restore-verify |

### ⚠️ v0.8.12 之前 L2 Snapshot+Export 的真实状况（v0.8.11.2 时刻）

> 这条很重要——避免把"L2 现在没问题"当成"Snapshot half 现在可恢复"。

| Half | Velero Backup 参数 | 数据落在哪 | VSC 删除影响 |
|---|---|---|---|
| Snapshot-half | `snapshotMoveData=false` | CSI VolumeSnapshot / VSC（集群内） | ❌ **VSC 被 Velero v1.18 删，Snapshot RP 仅剩元数据，没有可恢复数据** |
| Export-half | `snapshotMoveData=true`（Data Mover） | BSL（远端对象存储） | ✅ 数据在 BSL，不受 VSC 删除影响 |

**结论**：
- Export 半边 100% 可恢复 ✅
- Snapshot 半边事实上是 UI placeholder ❌
- 我们对客户讲过的"L2 双份保护 / 秒级本地恢复"是空头支票——只是没人去试 Snapshot RP，所以未爆雷
- **v0.8.12 双 BSL 重构本质上是补这张支票**，不只是"优化"

### 与现有 Sprint 的关系

- **#28（Velero v1.18 VSC 默删调查）**：走"双 BSL"路线后**#28 直接降级**——VSC 删不删都无所谓了，L1 不再依赖 VSC
- **#51 Restore SupKube**（Self-Protect Schedule）：与本 sprint **并行**，无依赖冲突——Restore SupKube 落地到任意 BSL 即可（首选本地 MinIO 加快 DR）
- **#52 HashiCorp Vault 集成**：放在 #51 之后，保持原序

### UI 术语改造（v0.8.12-LBS2 落实）

| 旧 | 新 |
|---|---|
| Snapshot only | **Local Backup** |
| Snapshot + Export | **Local + Cloud Backup** |
| Export only | **Cloud Backup** |
| "Snapshot Half" 活动卡 | "Local Copy" 活动卡 |
| "Export Half" 活动卡 | "Cloud Copy" 活动卡 |

---

## v0.8.13 — Helm 打包优化（~2 天，小步快跑）

**目标**：首装时间从 30 min 降到 5 min。

| 决策 | 选定 |
|---|---|
| Velero | **必装**（subchart，客户不一定会装） |
| Dex | **可选**（Advanced Install Option，默认关；客户用外部 IdP 时不需要） |
| MinIO | **可选**（默认关，Settings 里勾选启用） |
| 启用方式 | Helm Advanced Install Option **或** 装好后在 Settings 插件管理里勾选 → 触发 helm upgrade |

子任务：
- **HC1** ~0.5d：Velero 作为 SupKube Helm chart 的 subchart，默认 enabled=true
- **HC2** ~0.5d：Dex 作为 conditional subchart，helm value `auth.dex.enabled` 控制
- **HC3** ~0.5d：MinIO subchart 同上，helm value `localStore.enabled` 控制
- **HC4** ~0.5d：Settings → 插件管理页（列出可选插件 + 当前启用状态 + "Enable" 按钮模拟 helm upgrade）

---

## v0.8.14 — 日志查看器 + 在线 Case 入口（~3.5 天，小步快跑 + 顺手做）

**背景**：#28 调查时只有我能看日志，客户隔离环境下无法 troubleshoot。要给客户一个"在 UI 里看 SupKube/Velero/Node-Agent 日志 + 一键下载日志包 + 提交 Case 时附带日志"的能力。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **LV1** | 1.0d | 后端：`GET /api/v1/logs/:component?since=&tail=&search=` 聚合 supkube-backend / velero / node-agent / dex 四个 pod 日志，统一时间戳排序 |
| **LV2** | 1.0d | 前端：Sidebar → "Logs" 新菜单；Element Plus 终端样式 + 按组件 chip 筛选 + 关键字高亮 + 自动滚动开关 |
| **LV3** | 0.5d | 一键 "Download Support Bundle"：当前过滤结果 + cluster info + supkube version + 最近备份/恢复状态 打包成 `.tar.gz` |
| **LV4** | 0.5d | **"Upload to Support" 按钮**（与下载并列）：弹窗 = Subject / Description / Severity / 自动附带日志包；若 Case API 未就绪 → fallback `mailto:support@supkube.com` 自动填好附件路径提示；Case API 就绪后无缝升级（v0.8.14 → v0.9.x） |
| **LV5** | 0.5d | 后端 RBAC：仅 admin 可看 + 上传日志（涉及敏感信息） |

---

## v0.8.15 — 备份数据校验（~2 天，小步快跑）

**目标**：每个备份都有"健康度"报告，不再靠用户祈祷。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **DV1** | 1.0d | 备份完成后自动 trigger Kopia `repository validate-blobs` + checksum；结果写入 Backup CR annotation |
| **DV2** | 0.5d | 周期性（cron 每天 1 次）跑 `kopia repository verify` 在所有 BSL 上；结果写入 ConfigMap `supkube-validation-report` |
| **DV3** | 0.5d | UI：Restore Points 表加 "Health" 列 chip（🟢 Valid / 🟡 Pending / 🔴 Corrupt）；Dashboard 加 "Backup Health" 卡片 |

---

## v0.8.16 — Swagger REST API（~2 天，小步快跑）

**目标**：客户/集成商能调我们的 API 做自动化。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SW1** | 1.0d | 引入 `gin-swagger`，所有现有 handler 加 `// @Summary` 等注释自动生成 OpenAPI 3.0 spec |
| **SW2** | 0.5d | 挂载 `/api/v1/swagger/*` → Swagger UI；admin 可访问 |
| **SW3** | 0.5d | Settings 加 "API Tokens" 页（v0.8.5 已有 token 后端，补 UI） |

---

## v0.9.0 — 多集群管理 + 跨集群恢复 Wizard（~7 天，MVP 闭环关键）

**背景**：基于 Mars 提供的 Kasten Multi-Cluster Manager 截图设计（详见架构分析）。本 sprint 完成 MVP 闭环：**A 集群备份 → Cloud BSL → B 集群恢复**。

### 设计来源（Kasten Multi-Cluster Manager 截图 + kubectl 现场调研）

**Kasten 核心 CRD（2026-05-24 现场确认）**：
- `clusters.dist.kio.kasten.io` — 集群注册表（v0.9.0 MVP 复刻 → `clusters.supkube.io`）
- `distributions.dist.kio.kasten.io` — Global Resource 分发规则（v0.9.10+ 才做）
- `bootstraps.dist.kio.kasten.io` — token-based cluster join 加密初始化（v0.9.x+ 做 Join Token UI 时复刻）

**Distributions 含义**：admin 在 MCM 定义 Global Policy / Global Profile → Distribution CRD 用 label selector 把这些资源 sync 到多个集群（不是 MVP，能力深度）。

| Kasten 元素 | SupKube v0.9.0 对应 |
|---|---|
| 顶部 Mode Switcher dropdown（Multi-Cluster Manager / 各集群） | ✅ MVP 复刻 |
| 侧栏：Clusters / Global Policies / Global Profiles / User Permissions / Distributions / Licensing / Join Tokens | ✅ Clusters only；其他 v0.9.10+ |
| Multi-Cluster Manager Dashboard（4 counts + Data Usage + Recent Activity） | ✅ MVP 复刻 |
| Clusters 列表：name(链接) + label + Apps(3 chip) + Policies + Actions(3 chip) + kebab | ✅ MVP 复刻 |
| `dist.kio.kasten.io/cluster-type:primary\|secondary` label | ✅ 用 `supkube.io/cluster-type` |
| Join Tokens（token-based join） | ❌ MVP 用 **kubeconfig 上传**；v0.9.10+ 加 token |
| Distributions（Global Resource 分发） | ❌ v0.9.10+ |

**SupKube v0.9.0 CRD 设计**（MVP 只一个）：

```yaml
apiVersion: supkube.io/v1
kind: Cluster
metadata:
  name: aks-jumborca-test
  namespace: supkube
  labels:
    supkube.io/cluster-type: secondary    # primary / secondary
spec:
  kubeconfigSecretRef:
    name: cluster-aks-jumborca-test-kubeconfig
  context: aks-jumborca-test
status:
  phase: Healthy                            # Healthy / Unreachable / Unauthorized
  lastChecked: 2026-05-24T...
  k8sVersion: 1.34.7
  capabilities: [csi, dataMover]
```

**MCM 后端实现位置**：Kasten 把 multi-cluster API 内置在 `dashboardbff-svc`，无独立 deployment。SupKube 同理——在现有 `supkube-backend` 加 multi-cluster API 路由（`/api/v1/clusters/*`），无新 pod。

### 子任务

| 子任务 | 估算 | 内容 |
|---|---|---|
| **MC1** | 2.0d | **Cluster Manager 页**（Kasten 风格）：顶部 Mode Switcher dropdown + Sidebar 新菜单 "Clusters" + Multi-Cluster Dashboard 摘要卡 + Clusters 表（name 链接 / labels / apps chips / policies / actions chips / kebab）；操作 "Add Cluster"（kubeconfig 上传） |
| **MC2** | 1.5d | **Cluster CRD**：新建 `clusters.supkube.io`，存储 kubeconfig（K8s Secret 引用）+ context + capabilities + cluster-type label；后端 controller 每 60s 健康检查（`/healthz`）；UI chip "Healthy/Unreachable" |
| **MC3** | 2.0d | **Cross-Cluster Restore Wizard**：Restore Wizard 第一步加 "Target Cluster" 选择器；选源集群的 Restore Point → 选目标集群 → ns mapping → 提交时 SupKube 远程 apply Restore CR 到目标集群（用 cluster A 的 kubeconfig 通过 controller-runtime client） |
| **MC4** | 1.0d | **BSL 共享自动化**：源集群 Cloud BSL（如 Azure Blob）的 secret + URL 自动同步到目标集群 → 目标集群 Velero `BackupSyncController` 同步 metadata（Velero 内置，我们只配 BSL） |
| **MC5** | 0.5d | DR Playbook（USER_MANUAL §22）：客户能照着做"集群 A 挂了 → 在新集群 B 上跑 SupKube → 添加 Cloud BSL → 选 RP 恢复"完整步骤 |

**MVP 闭环验收标准**：
1. 在本地 docker-desktop AKS-A 上备份 `test-app` namespace 到本地 MinIO + Azure Blob
2. 在 AKS-B 上启动 SupKube，添加 AKS-A 的 Azure Blob 作为 Cluster Manager 远端
3. 在 AKS-B 的 SupKube UI 上看到 AKS-A 的 Restore Points
4. 选其中一个 RP → Restore Wizard → Target: AKS-B → Submit
5. AKS-B 上 `test-app` namespace 完整恢复 ✅

---

## v0.9.1 — 许可证管理器前端（~3 天，**前端独立可启动**）

**设计来源**：Kasten Licenses 页 1:1 复刻（截图见任务上下文）。先用 mock data 跑通完整 UI，Mars 后端 license server 就绪后零 UI 改动接入。

### Kasten Licenses 页结构（要复刻的元素）

| 区块 | SupKube 实现 |
|---|---|
| 顶部 3 卡：Node Count / License Summary / Compliance Status | ✅ 复刻；mock data |
| "Multi-Cluster Lease" 块（Valid + Licenses contributed to pool） | ✅ 复刻；v0.9.0 Cluster Manager 配合 |
| "Installed Locally" 卡片列表（icon + UUID + Status chip + Expires + Type + Node Limit + 删除按钮） | ✅ 复刻 |
| "Create New License" 蓝色按钮（输入 license key 弹窗） | ✅ 复刻 |
| "Node Usage History" 折线图（按月） | ✅ 复刻（ECharts） |

### Mock 后端 API spec（先用 mock 实现，等真后端替换）

```
GET    /api/v1/license/summary      → { nodeCount, licenseSummary, complianceStatus, multiClusterLease }
GET    /api/v1/license              → [{ id, name, uuid, status, expiresAt, type, nodeLimit }]
POST   /api/v1/license              → 提交 license key 文本 / file upload
DELETE /api/v1/license/:id
GET    /api/v1/license/usage-history → [{ month, peakNodeCount }]
```

### 子任务

| 子任务 | 估算 | 内容 |
|---|---|---|
| **LM1** | 1.5d | Settings → Licenses 页面（复刻 Kasten 整页布局）+ mock data 后端 endpoint |
| **LM2** | 0.5d | Create New License 弹窗（license key 输入 + file upload） + 删除确认 |
| **LM3** | 0.5d | License Activation 拦截页（首次启动 / License 过期时全屏阻断 UI，强制输入 license） |
| **LM4** | 0.5d | 功能门控 helper：`canUseFeature(feature)` 根据 license type 返回 true/false；超 nodeLimit 时降级 |

**交付物**：前端 100% 完成，跑在 mock data 上可演示给客户。Mars 真后端就绪后只换 endpoint 实现，不动任何 UI。

---

## v0.9.2 — 已合并到 v0.8.14 + Backlog（在线 Case 系统）

**变更**：Mars 决定"现在手工开 Case 也行，等日志 sprint 顺手做"。

- **v0.8.14-LV4** 已包含 "Upload to Support" 入口（手动开 Case 体验先用，等 Case API 就绪后无缝升级）
- 完整 Case API 集成 → **重要不紧急 Backlog**（见顶部表格）

跳过此版本号占位，下一个是 v0.9.3。

---

## v0.9.3 — 细粒度文件浏览 + 恢复（~5 天，Kopia-only）

**决策**：仅支持 **Kopia 卷**（即 Data Mover / FS Backup 写到 BSL 的备份）；CSI 卷需要先 restore 整个 PVC 再 mount，本期不做。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **FB1** | 2.0d | 后端：`POST /api/v1/restore-points/:name/browse` 启动一个临时 Kopia mount pod → 返回 mount path + session token |
| **FB2** | 2.0d | 前端：Restore Point 抽屉加 "Browse Files" 按钮 → 弹出 file tree（Element Plus Tree 组件） + 多选 + 路径搜索 |
| **FB3** | 1.0d | 选中文件后：Restore to PVC（输入目标 PVC + 路径）/ Download as zip / Copy 至本地 PVC |

---

## v0.9.4 — 企业安全栈 Phase 1：EntraID + Vault（~5 天）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SEC1** | 2.0d | Dex Connector 加 Microsoft Graph (EntraID)；UI 提供 EntraID Tenant ID / Client ID 配置；Keycloak 已有，不动 |
| **SEC2** | 2.0d | Vault 集成（合并原 #52）：Azure KeyVault & HashiCorp Vault 二选一；Settings 配置 endpoint + auth；BSL credentials、license key、DR passphrase 改从 Vault 读 |
| **SEC3** | 1.0d | MFA：EntraID Conditional Access 已支持 MFA，我们文档说明 + UI 显示当前用户的 MFA 状态 |

---

## v0.9.5 — 企业安全栈 Phase 2：Kyverno（~3 天，SIEM 移到 backlog）

| 子任务 | 估算 | 内容 |
|---|---|---|
| **SEC4** | 2.0d | Kyverno Policy 模板（默认 disabled，UI 一键启用）："PVC 必须关联 BackupPolicy"、"敏感 ns 必须 immutable"、"删 RP 必须有 admin approve" |
| **SEC5** | 1.0d | Audit Event 输出 channel 抽象层：现在落 K8s Event，后续可插 Sentinel / SOAR / 其他 SIEM；UI 配置 endpoint 占位（disabled） |

> **Sentinel webhook 实现** → 重要不紧急 backlog（客户启用 Sentinel 时再做）

---

## v0.9.6 — 应用级恢复演练（~7 天，独创卖点）

**决策**：编排 = **BPMN 画布 + Activity + Kanister Blueprint** 三层。Kasten 都没有这能力。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **RV1** | 2.0d | 后端：新 CRD `RestoreVerification`，spec 包含 BPMN XML + 依存关系图 + readiness 判定规则 |
| **RV2** | 2.5d | 前端：**BPMN.io 画布**集成，节点类型 = Activity（备份/恢复/等待 readiness/检查 endpoint/Kanister 触发），用户可拖拽编排 |
| **RV3** | 1.5d | 执行引擎：临时 ns 部署 → 按 DAG 执行 → 每个节点超时控制 → 全部完成后生成 PDF 报告 → 清理临时 ns |
| **RV4** | 1.0d | UI 报告页：执行历史 + 每次成功率 + 失败节点详情 + 下载报告 |

---

## v0.9.7 — KubeVirt VM 备份恢复（~8 天，按需启动）

**触发条件**：当客户真的使用 KubeVirt 时启动；否则推迟。

子任务略，详见后续 PRD。

---

## v0.9.8 — MCP Server（~4 天，前瞻 + AI 集成）

**目标**：让 Claude / OpenClaw 等 AI 助手能直接调 SupKube。

| 子任务 | 估算 | 内容 |
|---|---|---|
| **MCP1** | 1.5d | MCP 协议实现（gomcp）+ stdio/SSE transport |
| **MCP2** | 1.5d | 工具集：`list_backups`、`create_restore`、`get_compliance_status`、`browse_files`、`get_logs` |
| **MCP3** | 1.0d | 文档：Cursor / Claude Desktop / OpenClaw 接入示例 |

---

## v1.0 GA 准备

- Prometheus metrics 完整化（前 v0.8.x sprint 中零散加的合并）
- Webhook 通知（备份失败 / RP 损坏 → 自定义 webhook）
- 文档站（VitePress + 中英双语）
- Helm chart 发布到 OCI registry（ghcr.io / quay.io）
- 性能压测 + 调优报告

---

## 项目现状评估

**Phase 1 PRD 7 个页面骨架已落地**（Dashboard / Applications / Backups / Restores / Policies / StorageLocations / Settings），后端基于 Go + Gin + controller-runtime 包装 Velero CRD，前端 Vue 3 + Element Plus。Helm Chart 可一键部署到本地 Docker Desktop K8S，端到端备份/恢复链路已可走通。

**已做得不错的部分**（保留风格）：
- Application Details 抽屉：分组展示 Pods/Services/Deployments + kubectl 命令一键复制 — 已对标 Kasten Application 详情
- Storage Locations 的 Verify 按钮 + Available 状态色块
- Recent Backups / Recent Restores 列表交互
- Phase 5-p1 加入的资源预览面板（Restore 创建对话框）

---

## v0.5.x — 紧急修复 Sprint ✅ 2026-05-15 完成

> ✅ P0/P1 全部 6 项已完成并部署。Sprint 复盘见 [docs/SPRINT-v0.5.1-RETRO.md](docs/SPRINT-v0.5.1-RETRO.md)
> 关键发现：真正 root cause 是 `.env` 里 `VITE_API_URL=http://localhost:8080/api/v1` 这一行硬编码，导致整个 v0.5.0 在 K8S 部署下 axios 从未真正联通后端，所有 UI 数据都靠浏览器 disk cache 兜底。

### P0 Bug 修复

- [x] **Backups/Dashboard "Queued" 状态显示错误**  
  `test-app-20260514000056` 实际 `status.phase=Completed` 但 UI 显示 Queued，导致 Dashboard "Successful" 统计少 1。怀疑前端对 `status.phase` 为空时的兜底逻辑误判，或后端 list API 返回字段被裁剪。需要：
  - 后端 `ListBackups` 确认返回完整 `status` 子对象
  - 前端 status 映射加 fallback：`phase || 'Unknown'` 而非 `'Queued'`
  - 加 e2e 测试覆盖"completed backup 显示为 Completed"

- [x] **Restores 页面缺 Actions 列 / 后端缺 API**  
  后端 `handlers.go` 只有 List/Create/Get Restore，缺：
  - `DELETE /api/v1/restores/:name`
  - `GET /api/v1/restores/:name/logs`
  - `GET /api/v1/restores/:name/results`（resource warnings / errors）  
  
  前端补 View Details / View Logs / Delete 三个按钮。Failed 状态的 Restore 必须能看错误原因。

### P1 数据准确性

- [x] **Policies 页面 TTL 显示 "0s"**  
  Schedule 未设 TTL 时 Velero 默认为 0，UI 应显示 `Default (30d)` 或 `Unset`，并在创建表单里给默认值 720h。

- [x] **Applications Compliance 判定逻辑修正**  
  当前 `default` ns Workloads=0 却显示 Compliant，逻辑应为：
  - Workloads=0 → `Empty`（灰色）
  - 有 Workloads 且有近期成功备份 → `Compliant`
  - 有 Workloads 无备份 → `Unmanaged`
  - 有 Workloads 且最近备份失败 → `Non-Compliant`

- [x] **Applications 系统 ns 过滤**  
  排除清单除 `kube-system/kube-public/kube-node-lease/velero` 外，再加：
  - SupKube 自身：`supkube`
  - MinIO 存储：`minio`（或更通用：通过 label 识别）
  - 已恢复出的临时 ns：`restored-ns`（或加 `supkube.io/system=true` label 标记）  
  
  改为白名单 + 黑名单两种策略可配，写入 ConfigMap。

### 体验小补 (v0.5.2 — 未做)

- [ ] Dashboard 加 Failed Restores 卡片 / Recent Restores 显示 Failed 高亮
- [ ] 所有列表页加搜索框（名称模糊匹配，纯前端）
- [ ] Backups/Restores 列表加 phase 筛选下拉

### Sprint 中追加的强化（已完成，未列入原计划）

- [x] **基建：nginx 三层缓存策略**（index.html no-store / assets immutable / api no-store）— 解决了浏览器缓存的根本性问题
- [x] **基建：axios 全局 cache-buster 拦截器**（每个 GET 自动加 `?_=<ts>`）
- [x] **基建：修复 `.env` BASE_URL 跨域错配**（VITE_API_URL 从 `http://localhost:8080/api/v1` → `/api/v1`）—— 这是 v0.5.0 一个埋了很久的根因 bug
- [x] **UI：Applications 表格新增 Labels 列**（前 2 个 tag + `+N more`）
- [x] **UI：Application Details 抽屉标题 Kasten 风格**（居中、20px、font-weight 700）
- [x] **UI：LABELS 区圆角 plain tag + "Show N more labels..." 折叠**
- [x] **UI：顶部 `v0.5.1` 蓝色版本徽章**——后续版本可作为部署确认锚点

**交付**：v0.5.1 已发布 ✅，v0.5.2 体验小补待启动

---

## v0.6 — Phase 2 核心功能 (2~3 周)

实现 PRD Phase 2 已列、但前端尚未暴露的能力。

### 跨命名空间恢复

- [ ] **前端入口完整化**  
  e2e 测试历史显示后端 API 已支持跨 ns 恢复（`e2e-cross-ns-*` 共 3 次成功），仅缺前端入口。Restore 创建对话框增加：
  - "Target Namespace" 输入（默认原 ns）
  - Namespace Mapping 编辑器（多对一/多对多）
  - 提交时映射到 Velero `spec.namespaceMapping`

### Restore 资源转换 (Transform)

- [ ] **基础 Resource Modifiers**（对标 Kasten Transform Sets）
  - StorageClass 映射（源 sc → 目标 sc）
  - Image registry 替换（如 `gcr.io` → `harbor.local`）
  - Label / Annotation 注入
  - 提交为 Velero `resourceModifiers` ConfigMap

### 卷数据备份双模式（FS + CSI）

**决策：保留 Restic/Kopia 文件系统备份，并行引入 CSI 卷快照**，用户在创建备份/策略时按需选择，两种模式都要在备份详情里可观测。

- [ ] **VolumeSnapshotLocation 管理页面**  
  当前集群无 VSL。新增 UI：列表 + 创建 VSL（关联 CSI Driver，如 hostpath / ebs.csi / pd.csi）。
- [ ] **备份创建表单增加 "Volume Backup Mode" 单选**  
  - `Filesystem (Restic/Kopia)` — 默认，兼容所有 StorageClass，对应 Velero `defaultVolumesToFsBackup=true` / 注解 `backup.velero.io/backup-volumes`
  - `CSI Snapshot` — 仅当 PVC 关联的 SC 支持 CSI 快照时可选，对应 `snapshotVolumes=true`
  - 表单需根据所选 ns 的 PVC 自动检测可用模式，给出推荐
- [ ] **备份详情显示卷快照状态**  
  resourceList 中展开 VolumeSnapshot / VolumeSnapshotContent / PodVolumeBackup（PVB），区分两种模式的进度展示
- [ ] **Restore 表单尊重源备份的卷模式**  
  CSI 快照恢复时校验目标集群 StorageClass 兼容性（v0.6 内集群恢复，跨集群放到 v0.9）

### 日志与调试

- [ ] Backup / Restore 详情页 "View Logs" tab，从 Velero 下载日志 tar 解压后流式输出
- [ ] 失败原因高亮（解析 `status.errors` / `status.warnings`）

### Application 详情增强

- [ ] PVC 列表 + 大小（Kasten 必备）
- [ ] StatefulSets / DaemonSets / Jobs / CronJobs 区块
- [ ] 抽屉内"立即备份"按钮 → 跳到预填好 ns 的 Create Backup 表单
- [ ] 抽屉内显示"该 ns 的备份历史"小列表（最近 5 条）

---

## v0.7 — Kasten Actions Model 引入 (3~4 周)

> 重大架构演进：把 SupKube 的数据保护模型从"Velero 直翻"升级为"Kasten Actions 抽象"。
> 决策背景：Snapshot（本地快速）与 Export（远端持久）是两个 RPO/RTO 层级，强耦合在一个 Velero Backup 里限制了 Kasten 5min RPO 这类核心场景。
> **采用 Path C 混合策略**：v0.7 先做 UI 抽象层（路径 A），v0.8/v0.9 再做底层解耦（路径 B）—— 详见上方"关键架构决策（已定）"。

### v0.7-policy-1 · Actions 概念引入（核心）

- [ ] **Restore Point 抽屉拆分两层状态**：📸 Snapshot (Local) + 📦 Export (Object Storage)，分别显示 status / location / size / retention / 数据指纹
- [ ] **Restore Point Type 字段**：`Local Snapshot` / `Exported Backup` / `Imported Backup` 三选一（基于 Velero Backup spec 派生）
- [ ] **数据指纹** = `SHA256(metadata.uid + creationTimestamp + spec.storageLocation)`，UI 短形式显示前 8 位（如 `a3f9b727`），完整形式可一键复制
- [ ] **Policy 表单按 Actions 重构**：Snapshot Action（必选）+ Export Action（默认勾选）两块独立配置，分别配 retention
- [ ] **Snapshot/Export retention 分别映射**：v0.7 底层仍是单 Velero Schedule，spec.ttl 取两者较大值（UI 双字段是为 v0.9 真分离做铺垫）

### v0.7-policy-2 · Snapshot-only 风险防护（合规）

- [ ] **Policy 表单默认勾 Export**，取消勾时**触发二次确认对话框**：
      "Are you sure? Snapshot alone is not a backup. Data is lost if the underlying storage fails. Use only for development/staging environments."
- [ ] **Policies 列表新增 Protection Level 列**：
      - `L1 Snapshot Only` ⚠ 黄色 + hover 提示 "Not a durable backup"
      - `L2 Backup` ✅ 绿色（snapshot + export）
      - `L3 Immutable Backup` 🛡 蓝色（export to immutable BSL，v0.9 配合 BSL immutability 开关）
- [ ] **Dashboard 加 Protection Compliance 卡片**：`N policies are snapshot-only ⚠`，点击跳到筛选后的 Policies 列表
- [ ] **能力检测**：Create Policy 时若 target namespace 的 PVC 全在不支持 CSI 快照的 SC 上 + 用户又选了 Snapshot-only，立即报错并列出违规 PVC
- [ ] **Settings 加全局开关 `Block snapshot-only policies`**（默认 off，企业 PoC 客户启用后阻止保存任何 snapshot-only policy）

### v0.7-policy-3 · 双 cron Schedule（为 v0.9 RPO 5min 铺垫）

- [ ] Policy 表单 UI 提供 **Snapshot Schedule** 和 **Export Schedule** 两个独立 cron 字段
- [ ] v0.7 实现：底层仍创建单一 Velero Schedule，cron 取**较短间隔**；UI 上展示两者，user 看到的是"两个频率独立配"
- [ ] 默认值：Snapshot Schedule `0 * * * *`（每小时），Export Schedule `0 0 * * *`（每天）
- [ ] 默认 retention：**Snapshot=24h、Export=30d**（这次拍板）
- [ ] 提示框：当两个 cron 设得太接近（< 30 min 差）时提示"Export is expensive; recommend at least 4x Snapshot interval"

### v0.7-import · Import Restore Points 入口（路径 C 关键）

- [ ] **Sidebar 新增菜单项 `Import`**
- [ ] 列出 Storage Profile（BSL），点进去显示该 BSL 里发现的 Backup manifest
- [ ] **同步状态**：基于 Velero `BackupSyncController`（v1.10+），显示 "Synced N min ago" 时间戳
- [ ] **指纹去重**：同指纹的 backup 已在本地 Backup CR 中，标记为 "Already imported"
- [ ] 多选 + "Import Selected" 触发本地 Backup CR 创建（实际是引用同一个 BSL 路径，Velero 自动 sync 已经做了大部分工作）

### Dashboard 升级（保留原 v0.7 计划）

- [ ] 备份成功率 7/30 天趋势折线图（ECharts）
- [ ] 存储容量使用饼图（按 BSL）
- [ ] **Protection Compliance 卡片**（在 v0.7-policy-2 里）
- [ ] 顶部状态条：Velero / BSL / Storage 三项健康度

### 通用 UX（推 v0.7.x 或拆到 v0.7.5）

- [ ] 暗色主题切换（CSS Variables）
- [ ] i18n 中文（zh-CN），所有 view 抽 i18n key
- [ ] 全局搜索（顶栏，跨 backup/restore/app/policy）
- [ ] 面包屑导航
- [ ] 表格列宽自定义 + 列显隐控制

### 详情页面板化（保留原 v0.7 计划）

- [ ] Backup 详情改为左 metadata、右 tabs（Overview / Resources / Logs / Results）
- [ ] Restore 详情同上
- [ ] Storage Location 详情：加 Bucket 用量、Endpoint、关联备份数

---

## v0.7.5 — Backup Advisor MVP (2~3 周)

> **差异化卖点**：从"备份工具"升级为"备份顾问"，帮客户判断"哪些必须备、哪些可低频、哪些可跳过"。
> 决策背景：K8s 自愈能力强，无脑全备浪费存储/时间/成本；市面其他工具（Velero、Stash、PX-Backup）均无"主动评估"能力。
> **完整版**留在 v0.9（Compliance Score 集成）。

### 范围（MVP 故意保守）

- [ ] **Backup Advisor 页面**：每个 application 一行
  - 评分（0-100）
  - 推荐档位：`High Priority` / `Medium` / `Low` / `Skip Recommended`
  - 推荐理由列表（"has 2 PVCs"、"no CM/Secret changes in 30d"、"marked as core by user"）
- [ ] **评分规则引擎 v1**（K8s API 数据，**不**依赖 Prometheus）
  - 有 PVC：+40
  - 有 StatefulSet / CRD instances：+20
  - 用户打 `supkube.io/tier=core` label：+30
  - 在 default ns 但有 workload：+10
  - 纯 ReplicaSet 无 PVC 无 Service：-30
  - 评分阈值：≥70 High / 40-69 Medium / 10-39 Low / <10 Skip Recommended
- [ ] **"Apply Recommendation" 按钮**：跳到 Policies 创建页，预填好 schedule（频率按档位映射：High=每日 / Medium=每周 / Low=每月）
- [ ] **永远显示"未保护应用警告"**——`Skip Recommended` 的也要单独列出来，避免 silent skip 导致客户误以为系统替它兜底了
- [ ] **客户自定义规则**：YAML 配置文件可调整加分权重和阈值（CRD `BackupAdvisorPolicy`）

### MVP 绝对不做（防止过度承诺）

- ❌ 不自动应用策略（必须人工 review + 一键采纳）
- ❌ 不 silent skip 任何应用
- ❌ 不依赖 Prometheus / 运行特征数据（v0.9 才做）
- ❌ 不基于"业务核心/边缘"自动判断（无法可靠实现，只能客户打 label）

### 与现有体系的关系

- 复用 v0.5.1 已加的 `ComplianceStatus` 字段（Compliant/Unmanaged/NonCompliant/Empty/InProgress）
- Advisor 评分 ≠ Compliance Status：前者回答"该不该备"，后者回答"备得对不对"
- Applications 列表加一列显示评分徽章；详情抽屉显示完整推荐理由

---

## v0.8 — 安全与多租户 (3~4 周)

PRD 非功能需求里"Phase 2 加 RBAC"的落地。**决策：认证统一走 OIDC，不做本地用户系统**（运维成本 & 企业集成度考虑）。

- [ ] **认证**：纯 OIDC，主测 Dex / Keycloak / Azure AD / Auth0
  - 后端用 `coreos/go-oidc` + `oauth2`，PKCE 流程
  - 首次启动如未配置 IdP，Helm Chart 内置 Dex 子 chart 作为兜底（带一个 admin demo 用户，仅供初装演示，文档明示"生产必须替换为真实 IdP"）
  - 不支持 username/password 登录页（避免诱导用户长期使用 demo 凭据）
- [ ] **授权**：Role × Namespace 矩阵
  - `Admin`：全集群读写
  - `NamespaceOwner`：仅自己 owned 的 ns 备份/恢复（OIDC group → ns 映射，通过 ConfigMap 或 CRD `SupKubeRoleBinding`）
  - `Viewer`：全局只读
- [ ] **审计日志**：所有写操作（Create/Delete/Restore/Schedule mutation）落库 + UI 可查，记录 OIDC `sub` + email + 操作内容 + IP
- [ ] **会话管理**：JWT（短期）+ Refresh Token；前端无感刷新
- [ ] Settings 页面加 OIDC 配置 tab + RoleBinding 管理 tab

---

## v0.9 — 高阶能力 (4~6 周)

对标 Kasten 真正拉开差距的能力。

### Application Groups

- [ ] 用户自定义"应用边界"：跨多个 ns + label selector 组成一个逻辑 App
- [ ] Group 级别策略（备份/恢复以 Group 为单位）

### Pre/Post Hooks (基于 Kanister)

**决策：直接集成 [Kanister](https://kanister.io/)，复用 Kanister Blueprint 生态**，不自研 Hook 引擎。原因：Kanister 与 Kasten 同源，Blueprint 库覆盖主流数据库；自研需 ≥3 人月，且会重复造轮子。

- [ ] **Kanister 部署集成**：Helm Chart 加 Kanister 子 chart 作为可选依赖
- [ ] **Blueprint 管理 UI**：列出已安装 Blueprints（CRD `Blueprints.cr.kanister.io`），支持从官方仓库 [kanisterio/blueprints](https://github.com/kanisterio/blueprints) 一键导入
- [ ] **ActionSet 触发**：备份/恢复流程中按 namespace label 自动匹配 Blueprint 触发 ActionSet
- [ ] **内置模板库（预装）**：MySQL / PostgreSQL / MongoDB / Redis / Elasticsearch / Kafka 主流中间件
- [ ] **自定义 Blueprint 编辑器**：YAML 编辑 + 语法校验
- [ ] **Hook 执行日志**：从 ActionSet status 提取，集成到 Backup/Restore 详情页

### 跨集群与迁移 (Hub-Spoke 架构)

**决策：采用 Hub-Spoke 模型**（中心 SupKube 控制多个远端集群的 Velero），不做 Federation。原因：Hub-Spoke 实现简单、调试友好，符合典型企业"管理集群 + 业务集群"拓扑；Federation 控制面复杂度高、生态不成熟。

- [ ] **架构**：
  - Hub：单独部署 SupKube Control Plane（含 UI + 中心 API），不再要求与 Velero 同集群
  - Spoke：每个被纳管集群部署 Velero + 轻量 `supkube-agent`（可选，仅用于事件推送加速；无 agent 时 Hub 直接通过 kubeconfig 访问）
  - 通信：Hub → Spoke 用 kubeconfig（短期）；后续可选 agent 反向连接（穿透 NAT/防火墙）
- [ ] **集群注册 UI**：Settings → Clusters tab，上传 kubeconfig 或粘贴 token + CA
- [ ] **集群健康度面板**：每个 Spoke 显示 Velero 版本、BSL 状态、最近备份成功率
- [ ] **跨集群恢复**：从集群 A 的 backup 元数据恢复到集群 B，前提是两集群共享 BSL（v0.9 不做对象拷贝，需用户保证 BSL 可达）
- [ ] **BSL 跨集群共享**：UI 标识"此 BSL 被 N 个集群引用"
- [ ] **Hub 自身高可用**：v0.9 单副本，v1.0 加 leader election

### 合规性 & Backup Advisor 完整版

> 在 v0.7.5 Backup Advisor MVP（基于 K8s API 静态规则评分）之上，引入运行时观测数据让评分更准。

- [ ] **Compliance Score**（per Application）—— Advisor 评分 × 实际备份状况双维度
- [ ] **SLA 配置**（RPO/RTO 目标）+ 超限告警（Compliance Score < 阈值 → 通知 channel）
- [ ] **Advisor 增强：Prometheus 集成**
  - 资源变更频率：CM/Secret 改动 N 次/周 → 提升推荐档位
  - PV 读写活跃度：长时间无 IO 的 PVC 自动降档（Cold tier）
  - Pod 重启频率：异常高的应用降档（"还在调试，不值得高频备份"）
- [ ] **Advisor 增强：K8s 审计日志分析**
  - 检测哪些 ConfigMap/Secret 是手改的（vs GitOps 同步）→ 手改的必须备份
- [ ] **备份完整性校验**（Restore-test 自动化，定期到隔离 ns 恢复验证）
- [ ] **持续重评估**：应用变动（PVC 新增、Workload 类型变更）自动重算评分并通知

---

## v1.0 — GA (按 0.9 验收情况)

- [ ] Prometheus metrics 导出 + Grafana Dashboard 模板
- [ ] Webhook 通知（Slack / 飞书 / 钉钉 / 邮件）
- [ ] 备份导出 / 导入（air-gap 场景）
- [ ] Helm Chart 发布到 OCI registry（artifacthub.io）
- [ ] 文档站（mkdocs-material）
- [ ] 一键迁移工具：从 Velero CLI 现有部署接管

---

## Kasten K10 对标功能映射

| Kasten 能力 | SupKube 落地版本 | 状态 |
|---|---|---|
| Dashboard 概览 | v0.5 | ✅ 数字卡片已有，v0.7 加图表 |
| Applications 列表 | v0.5 | ✅ 已实现，v0.5.1 修 Compliance 逻辑 |
| Application Details | v0.5 | ✅ 已实现抽屉，v0.6 加 PVC/StatefulSets |
| Manual Backup | v0.5 | ✅ |
| Scheduled Policy | v0.5 | ✅ 已实现，v0.5.1 修 TTL 显示 |
| Same-cluster Restore | v0.5 | ✅ |
| Cross-namespace Restore | v0.6 | 后端已支持，前端入口待补 |
| Storage Locations | v0.5 | ✅ S3 兼容已实现 |
| Resource Transformation | v0.6 | 待开发 |
| CSI Snapshots | v0.6 | 待开发 |
| Pre/Post Hooks (Blueprints) | v0.9 | 待开发 |
| Application Groups | v0.9 | 待开发 |
| **Backup Advisor (智能分级推荐)** | **v0.7.5 MVP / v0.9 完整版** | **🆕 Kasten 未做的差异化能力** |
| Compliance Score | v0.9 | 待开发，与 Advisor 联动 |
| RBAC / Multi-tenancy | v0.8 | 待开发 |
| Multi-cluster | v0.9 | 待开发 |
| Reporting / Audit | v0.8 / v1.0 | 待开发 |

---

## 关键架构决策（已定）

| 决策项 | 版本 | 决策 | 影响 |
|---|---|---|---|
| 卷数据备份模式 | v0.6 | **保留 Restic/Kopia + 新增 CSI 快照，双模式并行** | 创建表单需"模式选择器"；详情页需展示 PVB 和 VolumeSnapshot 两种状态 |
| Hooks 引擎 | v0.9 | **直接集成 Kanister** | Helm 加 Kanister 子 chart；可复用社区 Blueprint；接受与 Veeam 生态的耦合 |
| 多集群架构 | v0.9 | **Hub-Spoke** | Hub 独立部署；Spoke 通过 kubeconfig 接入；v1.0 引入反向 agent |
| 认证方案 | v0.8 | **统一 OIDC（无本地用户）** | 强依赖外部 IdP；Helm 内置 Dex 兜底但仅供初装；不实现登录页/密码管理 |
| Actions 模型引入策略 | v0.7→v0.9 | **路径 C 混合分阶段**：v0.7 UI 抽象层先行（拆 Snapshot/Export 两层状态、Policy 双 cron+双 retention），v0.8 引入 SupkubeAction CRD 做审计，v0.9 自研 snapshot scheduler 真分离实现 5min RPO | 不一开始就重写 Velero；UI 心智模型从 v0.7 就对齐 Kasten |
| Snapshot-only 政策 | v0.7 | **支持但严格定位为 L1（快速回滚），不叫 Backup**：默认勾 Export；取消时二次确认对话框；Policies 列表标 Protection Level；Settings 加全局开关 `Block snapshot-only` 给企业 PoC | Policy UI / Dashboard Compliance 卡片 / 能力检测 |
| 默认 retention（v0.7 起） | v0.7 | **Snapshot=24h，Export=30d** | Policy 创建表单初始值；可调 |
| 数据指纹算法 | v0.7 | **`SHA256(metadata.uid + creationTimestamp + spec.storageLocation)`** 前 8 位显示 | Restore Point 抽屉、Import 去重；不依赖应用语义版本，纯 Velero 派生 |

## 仍待评审项

1. **OIDC IdP 测试矩阵**：v0.8 至少覆盖 Dex + Keycloak，是否加 Azure AD / Google Workspace？取决于早期客户画像。
2. **Kanister 版本选型**：v0.9 跟随 Kanister upstream master 还是固定到稳定 tag？影响升级节奏。
3. **Hub-Spoke 通信加密**：v0.9 默认 kubeconfig over HTTPS，是否额外加 mTLS？取决于威胁模型。
4. **CSI 快照恢复跨 SC 兼容性**：v0.6 内集群恢复策略 — 若目标 SC 不同是否自动 fallback 到 FS 恢复？需 v0.6 设计评审。
