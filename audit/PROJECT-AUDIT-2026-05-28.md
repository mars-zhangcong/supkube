# SupKube 项目盘点 — 2026-05-28

> **会议性质**：客户 demo 后第一次系统盘点。从今天起进入**每日站会 + DevOps 节奏**，告别瀑布。
> **参会**：Mars (founder/CTO) + Claude (architect/dev)
> **下次盘点**：2026-05-29（每日，固定时间待定）

---

## 0. 本次盘点的触发事件

- **2026-05-28 客户跨集群恢复 demo**（docker-desktop → AKS）
- demo 过程中暴露 5 个 P0 blocker、3 个 P1 体验缺陷
- 客户提出 2 个战略需求（KubeVirt VM 备份 + 还原时安全扫描）
- Mars 决策：**改用 K8s-native DevOps 流程**，工具栈 = **Plane（项目管理）+ Wiki.js（知识库）**，告别瀑布

---

## 1. 工程状态盘点（按优先级矩阵）

### 🔴 P0 — 阻塞客户使用 / 阻塞发版（5 项）

| ID | 问题 | 现状 | 影响 | 负责 |
|---|---|---|---|---|
| **P0-1** | Velero 没真自带 | 客户装完还要自己装 Velero + CRD + plugin；demo 踩 5 个坑（bitnami/kubectl 拉不动 / CRD URL 404 / v1.16 CSI 重复 plugin 崩） | "半成品"感强，对标 Kasten K10 差距大 | 待排 |
| **P0-2** | Dex issuer URL 硬编码 localhost:30888 | demo 时只能 `--set auth.enabled=false` 绕过 | 生产/AKS 上 OIDC 完全不可用 | 待排 |
| **P0-3** | AKS NodePort 默认不可用 | 公有云上 NodePort 不暴露公网，客户首次访问 404 | values.yaml 加了注释但默认值没改成 LoadBalancer-aware | 待排 |
| **P0-4** | CRD ownership 冲突 | helm install 时 `clusters.supkube.io` 已存在 → install 失败，需手动 annotate | 首次安装失败率高 | 待排 |
| **P0-5** | 跨集群 Imported RP 无法还原 | Wizard 显示 0 artifacts；hotfix 仅前端绕过，后端 `ListBackupArtifacts` 未修 | 跨集群 DR 卖点无法演示 | task #91 进行中 |

### 🟡 P1 — 严重影响体验（5 项）

| ID | 问题 | 现状 | 关联任务 |
|---|---|---|---|
| **P1-1** | 没有"一键预检 + 修复"嵌入 Restore Wizard | `preflight.sh` 只在装前跑；客户反馈缺 SC 适配 | task #89 |
| **P1-2** | 0.9.1.5-alpha demo hotfix 未发布 | RestoreDrawer + StorageLocations + values.yaml + i18n 已改未 commit | 本周 commit + publish |
| **P1-3** | "存储管理" tab 未做 | 快照位置 (VSL) 迁集群管理；BSL 留主菜单 | task #86 |
| **P1-4** | Imported RP 无显式入口 | 借鉴 Kasten Export/Import 指纹模型 | task #88 |
| **P1-5** | Restore 失败信号弱 | csi-hostpath-sc 不存在 → PartiallyFailed，无自动 SC mapping | task #89 |

### 🟠 P2 — 架构 / 战略层债务（6 项）

| ID | 问题 | 性质 | 建议时机 |
|---|---|---|---|
| **P2-1** | 没有 CI | 完全靠 `publish-release.sh` 手动跑；任何提交都没自动测试 | v1.0 GA 前必做 |
| **P2-2** | 没有 E2E 测试 | `测试用例.md` 写了 TC-* 一堆，没自动跑 | 与 CI 一起做 |
| **P2-3** | 没有遥测 | 客户用得怎样、踩什么坑、装到哪步失败都不知道 | v0.9.x backlog |
| **P2-4** | 没有文档站 | USER_MANUAL.md 8000+ 行单文件，搜索/导航差 | **Wiki.js 上线后立刻迁** |
| **P2-5** | alpha 阶段无 license 校验 | EULA gate 写了但 license key 任意字符串过 | v1.0 GA 前 |
| **P2-6** | 多版本快速发布无 changelog | 0.9.1.0 → 0.9.1.5 一周发 6 版，客户看不懂差异 | **本周加 CHANGELOG.md** |

### 🔵 P3 — 长期方向 / 销售线索（4 项）

| ID | 问题 | 性质 |
|---|---|---|
| **P3-1** | 商业化未启动 | PRODUCT-TIERS.md 写了三层套餐，但无收费机制、无销售渠道 |
| **P3-2** | 无公开网站 / 营销内容 | charts.supkube.com 只是 Helm repo，缺产品官网 |
| **P3-3** | 品牌/视觉资产薄弱 | 默认 logo/favicon 临时；无设计系统 |
| **P3-4** | AI 集成 (MCP Server) 未启动 | 卡在前面 sprint 链 |

---

## 2. 客户痛点跟踪（demo 当天反馈）

| ID | 客户原话 | 对应工程问题 | 状态 |
|---|---|---|---|
| **C-001** | "没有看到 Import 的标签" | P0-5 + Imported chip UI | hotfix 已写代码，未发布 |
| **C-002** | "没有了以前的一键适配检查...举个例子，Storage Class 已经不一样了" | P1-1 + P1-5 | 待排 task #89 |
| **C-003** | "没有创建新的 import 还原点的地方...请你借鉴一下 Kasten" | P1-4 (Export/Import 指纹模型) | task #88 |
| **C-004** | "点击不了 Restore" | Restore 按钮 disabled 无 tooltip | hotfix 已写 tooltip，未发布 |
| **C-005** | "我们的客户也不想脱离这个页面" | 跨集群还原全自动化 + 等待 UX | task #89 自动 SC mapping |
| **C-006** | KubeVirt VM 备份能力 | 新需求，已写进 v0.9.8 (5d) | task #93 |
| **C-007** | 还原时安全扫描 | 新需求，已写进 v0.9.6 (9d, YARA + ClamAV) | task #92 |

---

## 3. 本次盘点产生的决策记录

| 决策 ID | 决策内容 | 影响 |
|---|---|---|
| **D-2026-05-28-01** | **改用 DevOps + K8s-native 流程**，告别瀑布；每日站会盘点 | 工程节奏 |
| **D-2026-05-28-02** | **项目管理 = Plane**（自托管 K8s）；**知识库 = Wiki.js**（自托管 K8s） | 工具栈 |
| **D-2026-05-28-03** | **还原扫描 = YARA + ClamAV 双引擎**（不走 Kasten 单引擎路线）；插入 v0.9.6；详见 ADR-030 | 路线图 |
| **D-2026-05-28-04** | **KubeVirt VM 备份估算从 8d 下调到 5d**（Velero KubeVirt plugin 已处理 90%）；插入 v0.9.8 | 路线图 |
| **D-2026-05-28-05** | **v0.9.6 安全扫描 + v0.9.7 还原演练 = "Verified Restore" 完整闭环**（数据干净 + 流程可用），Premium 套餐定价锚点 | 商业定位 |
| **D-2026-05-28-06** | **建项目仪表板 HTML**（Kanban + List + Gantt 三视图）作为日常复盘工具；远期切 Plane | 工具栈 |

---

## 4. 本周行动项（截至 2026-06-03）

| 优先级 | 行动 | 负责 | 截止 |
|---|---|---|---|
| **🔴 P0** | commit + push 当前所有未提交改动（hotfix + ROADMAP + ADR-030 + 本 AUDIT） | claude+mars | 今日 |
| **🔴 P0** | publish 0.9.1.5-alpha（demo hotfix） | claude+mars | 明日 |
| **🔴 P0** | 部署 Plane + Wiki.js 到 AKS（与 SupKube 同集群或独立集群） | mars | 本周内 |
| **🔴 P0** | 把现有 93 个 task + 客户痛点 C-001~C-007 + P0~P3 全部导入 Plane | claude | Plane 上线后 |
| **🟡 P1** | 启动 v0.9.1.3-VELERO-BUNDLED sprint（task #84，P0-1 的根治） | 待排 | 下周 |
| **🟡 P1** | 写 CHANGELOG.md（0.9.0 起所有发布） | claude | 本周内 |
| **🟢 P2** | 评估 GitHub Actions CI（最小化第一版：build + lint） | 待排 | 月底 |

---

## 5. 关于 "记忆 / 数据库" 的评估

Mars 提出"如果你的记忆存储不够用，我们自己建一个数据库"。我的评估：

### 短期（今天 — 1 个月）
- **不需要自建数据库**。当前每个 session 我可以读 ROADMAP.md / task list / AUDIT 文档作为"长期记忆"，无信息丢失
- **HTML 仪表板**（今天产出）= 静态可视化，数据源 = 本仓库 markdown / JSON，**git 即数据库**
- **Plane** 部署后 = 关系型 DB（Postgres 后端）天然作为任务持久层

### 中期（1-3 个月）
- 客户多了之后，需要**遥测数据库**（接收客户安装/使用上报）→ 建议 **TimescaleDB** 或 **ClickHouse**（时序 + 分析友好）
- **不重复造 Plane / Wiki.js 的轮子** —— 那些是 BSL/AGPL 开源软件，自托管即可

### 长期（半年+）
- 当 Plane + Wiki.js + 遥测库三者数据出现**关联查询需求**（"哪些客户卡在哪个 sprint 上"）→ 建 **数据仓库 / metabase**，做内部分析

### 建议
**今天不动数据库决策**，先把 Plane + Wiki.js 立起来，跑两周看实际瓶颈在哪。我的"记忆"层面只要每日 AUDIT 文档持续更新，配合 git 历史，足够支撑 6 个月内的工程节奏。

---

## 6. Demo 第二次现场 checklist（今天 demo 中要看的）

> Mars 反馈："今天我们要把 Demo 重新走一遍，客户还会发现新的问题。他已经和我说了3个。"

### 客户已说出来的 3 个新问题

| 客户 ID | 客户描述 | 工程映射 | Sprint | 估算 |
|---|---|---|---|---|
| **C-008** | Activity 点开没有详细步骤与用时（对标 **Arcami** 火山引擎产品图） | `GET /backups/:name/timeline` + ActionDetailDrawer 时间线区块 | task #96 | 3d |
| **C-009** | Data Usage Report 没有，Dashboard 也没展现（对标 **Kasten K10** `/data` 页 4 块图，含 **Deduplication 20.8x**） | Prometheus subchart + 自研 exporter + 4 块卡 + 各卡 CSV/Refresh + dedup 算法 | task #97 | 6d |
| **C-010** | Multi-Cluster 下拉切换后 Dashboard / 数据视图**不变化** | `currentClusterId` 不联动，全局 provide/inject + `:key` 强制重建 + 后端 header middleware 加严 | task #98 | 1.5d（P0 demo blocker） |

### Demo 现场要主动观察的点（claude 视角，避免被动只回应客户）

| 区域 | 看什么 |
|---|---|
| **Sidebar 切集群** | 是否真的有 dropdown？切完是否 reload？XHR 是否带新 cluster header？|
| **Activity / Action Details** | 当前 phase 字段对客户是否"够用"？字节速率显示是否易读？|
| **Dashboard** | 当前 Data 卡 vs Kasten 4 块，客户会主动问"我们的 dedup 多少倍"吗？|
| **Restore Wizard 跨集群** | hotfix 没发，Imported chip 还没上，再演会不会被同样的坑卡住？建议 demo 前发完 0.9.1.5-alpha |
| **跨集群 PartiallyFailed** | 上次 csi-hostpath-sc 错误如果再现，要不要现场手动 SC mapping 给客户看（哪怕粗糙）？|
| **Helm install 体验** | 客户自己装时如果撞 P0-1~P0-4，要不要把 P0-1 紧急塞进 0.9.1.5？|

### Demo 后立即产出

- [ ] 现场记录新问题（追加到 `CUSTOMER_PAIN` C-011+）
- [ ] 现场指认哪些是 hotfix 范畴 vs 长 sprint 范畴
- [ ] 当晚追加 `audit/PROJECT-AUDIT-2026-05-28-PM.md` 或同日合并

---

## 7. 下一次盘点（2026-05-29 10:00）的输入

- [ ] Plane 部署进度
- [ ] Wiki.js 部署进度
- [ ] 0.9.1.5-alpha 是否发布成功
- [ ] P0-1 到 P0-5 是否进入开发
- [ ] HTML 仪表板使用反馈（是否要扩字段、是否要接 Plane API）
- [ ] Mars 决策追加（2026-05-28）：
  - Plane + Wiki.js → **aks-tools 集群**（独立，不污染 aks-dev）
  - 每日站会时间 → **10:00**
  - 仪表板路径 → **本地用，一项目一独立仪表板**（未来 dashboard-supkube / dashboard-arcami-rip 等并列；模板化是方向）

---

## 8. Demo 第二次执行实录（2026-05-28 下午）

> 本节是 demo 现场连环发现 + 所有手动 kubectl 操作的**可复现记录**。每个手动操作都对应一个待产品化的 task。

### 8.1 成果：双向灾备闭环完整验证 ✅

| 方向 | 路径 | 结果 |
|---|---|---|
| **正向** | docker-desktop 备份 → Azure Blob 共享 BSL → AKS 还原 | ✅ postgres + adminer Running，"Skipping initialization" 用恢复数据 |
| **反向** | AKS 改数据(6 行) → 备份 → docker-desktop 回流还原 | ✅ 6 行数据完整回到本地，WAL recovery |

**业务证明**：world 表数据在两端自由流动（4 行 → 云上改成 6 行 → 回流本地 6 行），postgres 每次都识别现有数据库跳过初始化。

### 8.2 连环 Bug 修复（5 个发布版本）

| 版本 | 修复 | 根因 |
|---|---|---|
| 0.9.1.5 | Restore tooltip + 整包还原 banner + SecKey UX | demo 反馈 |
| 0.9.1.6 | Imported chip 检测（后端 schedule 存在性，取代假 label） | C-001：检测了 Velero 不写的 `velero.io/source-cluster` label |
| 0.9.1.7 | 编辑策略 404（PolicyAggregate 取名） | v0.8.9 dual-schedule 迁移遗留 |
| 0.9.1.8 | 改云端 BSL 假成功 + 策略列表"存储位置"列 | patchBody 漏传 exportStorageLocation |
| 0.9.1.9 | Imported RP 创建时间显示同步时间 | 用 metadata.creationTimestamp 而非 status.startTimestamp |

> **同源教训**：0.9.1.6/7/8 三个 policy/imported bug 全部源于 **v0.8.9 dual-schedule (PolicyAggregate) 模型迁移时调用方未全量审计**。已建 #101 做全量审计。

### 8.3 跨集群存储障碍 —— 三层 + 反向额外坑（核心发现）

每个都是**真实客户必踩**，且当前**只能回命令行解决**（产品化目标见 #104）：

| # | 障碍 | 现象 | 手动解法（已执行） |
|---|---|---|---|
| 1 | SC 名不匹配 | `csi-hostpath-sc` 在 AKS 不存在 → PVC Pending | `kubectl create cm change-storage-class-config` 映射 |
| 2 | SC binding mode 死锁 | WaitForFirstConsumer → DataMover 卡 Prepared | 建 `supkube-restore-immediate` (Immediate) SC |
| 3 | **node-agent hang** | ready=true 但 data path watcher 静默卡死 | **重启 node-agent → 立刻恢复**（#102 真凶） |
| 4 | 备份缺 VolumeSnapshotClass | AKS 无 disk.csi.azure.com VSC → errors:2 秒失败 | 建 `azuredisk-vsc`（带 velero label） |
| 5 | ChangeStorageClass 映射不生效 | 反向还原 PVC 引用不存在 SC → DataDownload Failed | 直接建别名 SC（比映射可靠） |
| 6 | ns 卡 Terminating | LoadBalancer service 还原到无 LB 集群 → finalizer 残留 | 换新 ns 名 + 强制 finalize |
| 7 | BSL 名 `default` not found | UI backup 空 storageLocation → fallback default → AKS 无此 BSL | `kubectl patch bsl azure-blob spec.default=true` |

### 8.4 手动 kubectl 操作清单（可复现 / 待产品化）

```bash
# AKS — 跨集群还原侧
kubectl --context=aks-jumborca-dev apply -f - <<EOF   # Immediate SC (DataMover 要求)
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata: {name: supkube-restore-immediate}
provisioner: disk.csi.azure.com
volumeBindingMode: Immediate
reclaimPolicy: Delete
EOF
# 同名别名 csi-hostpath-sc (Immediate) 同理
kubectl --context=aks-jumborca-dev patch cm change-storage-class-config -n velero --type merge \
  -p '{"data":{"csi-hostpath-sc":"supkube-restore-immediate"}}'

# AKS — 备份侧 VolumeSnapshotClass
kubectl --context=aks-jumborca-dev apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata: {name: azuredisk-vsc, labels: {velero.io/csi-volumesnapshot-class: "true"}}
driver: disk.csi.azure.com
deletionPolicy: Delete
EOF

# AKS — BSL default 标记 (UI backup 不指定 location 时用)
kubectl --context=aks-jumborca-dev patch bsl azure-blob -n velero --type merge -p '{"spec":{"default":true}}'

# docker-desktop — 反向还原别名 SC
kubectl --context=docker-desktop apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata: {name: supkube-restore-immediate}
provisioner: hostpath.csi.k8s.io
volumeBindingMode: Immediate
EOF
```

> **这整段 kubectl 就是 #104（CSI 一键适配）要产品化的全部内容** —— 客户点"一键修复"，SupKube 自动做这些 + 显示用了哪个 Transform Set。

### 8.5 本次 demo 产生的 task（#92–#108）

| task | 主题 | 优先级 |
|---|---|---|
| #102 | node-agent hang 检测 + 自愈 | P0 |
| #103 | Restore 错误/warning 可见 + 解读 + Log Viewer 组件分类 | P0 |
| #104 | **CSI 一键适配**（SC+VSC+binding mode 自动配 + Transform Set 透明）⭐ | P0 |
| #105 | 发起 Restore 后前端零反馈 | P0 |
| #106 | Restore 任务中心 + Restore Advisor（成功率/成本评分） | P1 差异化 |
| #107 | backup BSL 自动选 + 云集群默认 L1 | P1 |
| #108 | 应用"最近备份"链接跳 Policy 而非 RP | P2 |

### 8.6 临时演示资源（demo 后需收回）

- AKS `test-app-restore/adminer` → LoadBalancer 公网 IP `20.247.232.202`（**省 LB 费用，会后删**）
- 本地 port-forward：`localhost:18888`（AKS adminer）/ `localhost:19999`（docker-desktop 回流 adminer）

---

> **会议结束时间**：（待补）
> **下次会议时间**：2026-05-29 10:00
> **文档维护**：每次盘点结束后追加新文件 `audit/PROJECT-AUDIT-YYYY-MM-DD.md`；本目录是会议纪要时间线
