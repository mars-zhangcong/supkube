# SupKube Roadmap

> Last updated: 2026-05-18
> Current released version: **v0.5.2**（已部署到本地 docker-desktop K8S）
> Reference product: [Kasten K10 by Veeam](https://docs.kasten.io)

## 当前状态总览

| Sprint | 状态 | 备注 |
|---|---|---|
| v0.5.0 | ✅ 已发布 | Phase 1 PRD 7 个页面骨架完成；端到端备份恢复链路通畅 |
| v0.5.1 | ✅ 已完成 2026-05-15 | P0/P1 共 6 项修复；见 [docs/SPRINT-v0.5.1-RETRO.md](docs/SPRINT-v0.5.1-RETRO.md) |
| **v0.5.2**（Kasten polish + CSI infra）| ✅ **已完成 2026-05-18** | Kasten 风格 sidebar/logo/Restore Points、Storage Locations kebab CRUD、SupVault provider、RFC1123 校验、Applications filter toolbar、CSI 快照基础设施落地。见 [docs/SPRINT-v0.5.2-RETRO.md](docs/SPRINT-v0.5.2-RETRO.md) + [docs/csi-snapshot-setup.md](docs/csi-snapshot-setup.md) |
| v0.6（Phase 2 核心） | 🟡 B 阶段进行中 | VSL 管理页面、Volume Backup Mode 双选、CSI 进度展示 |
| v0.7（UI 对标 Kasten） | 🟡 部分提前完成 | Filter toolbar / Multi-select / Kasten sidebar / kebab 已落地；Dashboard 图表、暗色主题、i18n 仍待做 |
| v0.7.5（Backup Advisor MVP） | 🔲 待启动 | 智能备份分级建议（评分 + 推荐档位 + 一键采纳） |
| v0.8（安全多租户） | 🔲 待启动 | OIDC、RBAC、审计日志 |
| v0.9（高阶能力） | 🔲 待启动 | Kanister Blueprints、Hub-Spoke 多集群、Compliance Score + Advisor 完整版 |
| v1.0（GA） | 🔲 待启动 | Prometheus、Webhook、文档站 |

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

## v0.7 — UI/UX 对标 Kasten (2~3 周)

视觉层级与交互细节贴近 Kasten K10。

### Dashboard 升级

- [ ] 备份成功率 7/30 天趋势折线图（ECharts）
- [ ] 存储容量使用饼图（按 BSL）
- [ ] 受保护应用 vs 未保护应用 占比
- [ ] 顶部状态条：Velero / BSL / Storage 三项健康度

### 通用 UX

- [ ] 暗色主题切换（CSS Variables）
- [ ] i18n 中文（zh-CN），所有 view 抽 i18n key
- [ ] 全局搜索（顶栏，跨 backup/restore/app/policy）
- [ ] 面包屑导航
- [ ] 表格列宽自定义 + 列显隐控制

### 详情页面板化

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

## 仍待评审项

1. **OIDC IdP 测试矩阵**：v0.8 至少覆盖 Dex + Keycloak，是否加 Azure AD / Google Workspace？取决于早期客户画像。
2. **Kanister 版本选型**：v0.9 跟随 Kanister upstream master 还是固定到稳定 tag？影响升级节奏。
3. **Hub-Spoke 通信加密**：v0.9 默认 kubeconfig over HTTPS，是否额外加 mTLS？取决于威胁模型。
4. **CSI 快照恢复跨 SC 兼容性**：v0.6 内集群恢复策略 — 若目标 SC 不同是否自动 fallback 到 FS 恢复？需 v0.6 设计评审。
