# SupKube 用户手册

> **版本对应**：v0.8.4-alpha（持续更新）
> **目标读者**：负责在 Kubernetes 集群上部署、运维、排障 SupKube 的工程师
> **本手册回答的问题**：
> - SupKube 是什么、和 Velero / Kasten 的关系是什么
> - 每个关键功能背后**为什么这样设计**（不是只讲怎么用）
> - 客户最容易踩的坑，以及我们的产品层面是如何兜底的
> - 遇到看不懂的报错该去哪里查

---

## 目录

1. [简介与定位](#1-简介与定位)
2. [架构](#2-架构)
3. [核心概念与术语](#3-核心概念与术语)
4. [⚠️ 重要陷阱与产品设计应对](#4-️-重要陷阱与产品设计应对)
   - 4.1 [Velero `existingResourcePolicy: none` 默认陷阱](#41-velero-existingresourcepolicy-none-默认陷阱)
   - 4.2 [**跨 ns / 跨 cluster Restore 冲突矩阵**](#42-跨-ns--跨-cluster-restore-冲突矩阵关键章节)
   - 4.3 [BackupSyncController 自动重建删除的 CR](#43-backupsynccontroller-自动重建删除的-cr)
   - 4.4 [CSI Snapshot vs Filesystem 备份的取舍](#44-csi-snapshot-vs-filesystem-备份的取舍)
5. [安装与升级](#5-安装与升级)
6. [操作指南](#6-操作指南)
7. [故障排查](#7-故障排查)
8. [RBAC 权限说明](#8-rbac-权限说明)
9. [术语表](#9-术语表)
10. [Roadmap 摘要](#10-roadmap-摘要)
11. [认证配置：4 大 IdP 快速集成](#11-认证配置4-大-idp-快速集成v085)
12. [RBAC：3 角色权限模型](#12-rbac3-角色权限模型v085-step-3)
13. [审计日志](#13-审计日志v085-step-4)
14. [非 OIDC 登录：API Token 与 Basic Auth](#14-非-oidc-登录api-token-与-basic-authv085-step-6)
15. [备份组成：数据路径与大小](#15-备份组成数据路径与大小v086)
16. [跨集群跨云灾备](#16-跨集群跨云灾备v087)
17. [Data Mover vs Filesystem — 深度对比](#17-data-mover-vs-filesystem--深度对比v0875)
18. [备份链与去重模型](#18-备份链与去重模型v087)
19. [集群健康：孤儿资源的 GC 与设置](#19-集群健康孤儿资源的-gc-与设置v088)
20. [双策略：Snapshot + Export 模型](#20-双策略snapshot--export-模型v089-引入--v0810-改进)
21. [kubectl 速查 + label / annotation 契约](#21-kubectl-速查--label--annotation-契约v08102)
22. [灾备演练 / DR Playbook](#22-灾备演练--dr-playbookv090-multi-cluster-manager)
23. [Helm 安装参考](#23-helm-安装参考v091-install-reference)
24. [Import Policy（跨集群持续 DR）](#24-import-policy跨集群持续-drv09113)
25. [数据韧性评分（Resilience Score）解读](#25-数据韧性评分resilience-score解读)

---

## 1. 简介与定位

### 1.1 SupKube 是什么

**SupKube 是 Kubernetes 上的应用数据保护 UI**，对标 Veeam Kasten K10，构建在 Velero 之上。它把 Velero 的命令行能力（备份/恢复/快照/对象存储/CSI）包装成生产可用的 Web 界面，并补齐 Velero 没有的东西：备份建议、跨集群可视化、Restore 前置冲突检测、Transform Set（v0.8）等。

### 1.2 三方关系

```
┌──────────────────────────────────────────────────────────┐
│  SupKube  ← UI + 编排 + 智能建议（这是我们）              │
│     ↓                                                     │
│  Velero   ← 数据面，实际执行备份/恢复 / 调度              │
│     ↓                                                     │
│  K8s API + CSI Snapshotter + 对象存储 (S3/MinIO/GCS/Azure) │
└──────────────────────────────────────────────────────────┘
```

**为什么不是从零造一个备份引擎**：Velero 在 K8s 数据保护领域是 de facto 标准，CNCF 毕业项目，社区维护周期长。重做一遍是 18 个月起步的工作量，没有商业价值。SupKube 的差异化在**产品层**——把 Velero 难用的部分（CLI 体验、错误提示、冲突处理）解决掉。

### 1.3 谁会用 SupKube

| 用户角色 | 主要诉求 |
|---|---|
| 平台/SRE 团队 | 集中管理多个集群的备份策略、容量、合规 |
| 应用 Owner | 自助完成单个 namespace 的备份/恢复，不用 kubectl |
| 安全合规 | 备份覆盖率报表、保留策略审计 |
| 灾备测试 | 跨集群恢复演练、Restore Point 时间旅行 |

---

## 2. 架构

### 2.1 组件构成

```
┌─ supkube-frontend (Vue 3 + Element Plus, nginx) ──────┐
│   - 提供 Web UI，所有 /api/* 反向代理到 backend       │
│   - 负责 i18n（zh-CN / en）、深色模式、ECharts 图表    │
└────────────────────────────────────────────────────────┘
                       ↓ HTTP
┌─ supkube-backend (Go + Gin + controller-runtime) ─────┐
│   - REST API on :8080                                  │
│   - 操作 Velero CRD（Backup / Restore / Schedule / ...）│
│   - 动态客户端枚举命名空间资源（artifacts、preflight） │
│   - 调用 Velero DownloadRequest 拉详细错误日志         │
└────────────────────────────────────────────────────────┘
                       ↓ K8s API
┌─ velero-system + 数据存储 ────────────────────────────┐
│   - Velero Controller（备份/恢复/调度）                │
│   - CSI Snapshotter（卷快照）                          │
│   - Restic/Kopia（文件系统级备份）                     │
│   - BSL：S3 / MinIO / GCS / Azure Blob                 │
│   - VSL：CSI 快照位置                                  │
└────────────────────────────────────────────────────────┘
```

### 2.2 四个核心架构决策

这四个决策定义了 SupKube 的产品形态，是其他细节的根基。

#### 决策 1：双模式卷备份（FS + CSI）

**决策内容**：UI 同时支持 Filesystem 备份（Restic/Kopia）和 CSI Snapshot 两条路径，用户在创建 Backup 时选择。

**为什么这样做**：
- **CSI Snapshot 快但有前提**：需要 StorageClass 支持 CSI snapshot（VolumeSnapshotClass 已配置），并且备份是 crash-consistent，对运行中的 DB 可能有数据不一致风险
- **Filesystem 慢但兼容性最好**：任意 StorageClass 都能用，对应用透明，但对 100GB+ PVC 性能差
- **生产环境通常需要并存**：核心 DB 用 CSI（VSS 钩子保证一致性），日志/媒体目录用 FS

**对应实现**：`createBackup` payload 里互斥的两个开关 `snapshotVolumes` 和 `defaultVolumesToFsBackup`。

#### 决策 2：Kanister Blueprints 作为应用一致性入口（v0.9）

**决策内容**：v0.9 引入 Kanister 项目，通过 Blueprint CRD 让用户为有状态应用（MySQL/Postgres/MongoDB 等）定义 pre/post-backup hooks。

**为什么不用 Velero 的 BackupHooks**：Velero 自带的 hook 是简单的 exec command，无法表达"先用 mysqldump 导出再快照 PVC"这种复杂流程。Kanister 是 Kasten 母公司开源的项目，与 Velero 互补，是事实标准。

#### 决策 3：Hub-Spoke 多集群（v0.9）

**决策内容**：SupKube 后续会引入一个 Hub Cluster，所有 Spoke Cluster 把 Velero 状态汇报给 Hub。Hub 负责跨集群 Restore、合规报表、统一仪表盘。

**为什么不用 Mesh / 对等模式**：对等模式在 N>5 集群时全连接复杂度爆炸，运维成本高。Hub-Spoke 简单可控，对小型团队也友好。

#### 决策 4：OIDC-only 认证（v0.8）

**决策内容**：不实现自己的用户/密码管理，强制走 OIDC（Keycloak/Okta/Azure AD/Dex）。

**为什么不做自带账号**：自带账号意味着自带密码哈希、自带 token 签发、自带审计——每一块都是安全攻击面，且企业本来就有 IDP，二次维护没价值。

---

## 3. 核心概念与术语

### 3.1 Backup vs Restore Point —— 同一个东西的两个视角

**`Backup` 是 Velero 的实现层术语，`Restore Point` 是用户视角的产品语言**。它们指的是同一个 K8s CR (`backups.velero.io`)。

- Velero CLI / CRD：叫 `Backup`
- SupKube UI：叫 `Restore Point`（参照 Kasten 命名）
- 选择产品语言的原因：用户脑子里想的是"我要恢复到某个时间点"，不是"我要创建一个备份对象"。术语反映使用意图。

### 3.2 Snapshot vs Export（L1 / L2 保护级别）

参照 Kasten 的 Actions Model：

| 级别 | 名称 | 描述 | 数据位置 | 恢复速度 | 抗灾级别 |
|---|---|---|---|---|---|
| **L1** | Snapshot | 仅在本地存储层快照（CSI snapshot 或 PV 级快照）| 同集群同 Storage | 秒级 | 集群级故障无法恢复 |
| **L2** | Snapshot + Export | L1 之后再 Export 到对象存储（BSL）| 远程 S3/MinIO/GCS | 分钟级 | 跨地域/跨云灾备 |
| **L3** | Snapshot + Immutable Export（v0.9）| L2 + 启用 Object Lock | S3 with Object Lock | 同 L2 | 抗勒索软件 |

**实践指南**：开发环境 L1 够用；生产环境**永远 L2 起步**；关键数据用 L3。

### 3.3 Local vs Imported（集群指纹）

Restore Point 列表的 Source 列有两种值：

- **🏠 Local**：在本集群创建的备份
- **🌐 Imported**：另一个集群创建、通过共享 BSL 同步过来的备份

**实现原理**：Velero v1.10+ 给每个 Backup 加注解 `velero.io/source-cluster-k8s-gitversion`（还有 major/minor）。SupKube 启动后取所有备份里出现频率最高的指纹作为"本集群"基线，其他指纹的就是 Imported。

**为什么这个区分重要**：跨集群恢复需要走 Pre-flight 冲突检测（v0.7.12），因为目标集群的 StorageClass / NodeSelector / Image registry 都可能不同。Local 恢复通常安全。

### 3.4 Storage Profile (BSL) vs Snapshot Profile (VSL)

| 名称 | Velero CRD | 作用 |
|---|---|---|
| Storage Profile | `BackupStorageLocation` | 对象存储位置（S3/MinIO/GCS/Azure Blob）—— 存 Backup 的 tarball + metadata |
| Snapshot Profile | `VolumeSnapshotLocation` | CSI 快照位置 —— 存 CSI VolumeSnapshot 引用 |

一个集群可以配多个 BSL/VSL，Backup 时选择一个。

### 3.5 Policy = Schedule + 保留策略

UI 叫 Policy，底层是 Velero 的 `Schedule` CRD。Policy 把以下三件事绑在一起：

1. **频率**：cron 表达式（如 `0 0 * * *` 每天 0 点）
2. **范围**：includedNamespaces / labelSelector
3. **保留**：TTL（如 720h = 30 天）

**重要**：删除 Policy（Schedule）**不会**删除已经创建的 Backup（Restore Point）。Schedule 只是个调度器，不是 Backup 的拥有者。

### 3.6 Activity / Action（v0.8.0 引入）

参考 Kasten 的设计哲学，**所有"做了一件事"都是一个 Action**。Action 是统一抽象，覆盖：

| Action Type | 底层 CRD | 含义 |
|---|---|---|
| **Backup** | `velero.io/Backup` | 一次备份动作；如果由 Schedule 触发，会带 `policy` 引用 |
| **Restore** | `velero.io/Restore` | 一次恢复动作；带 `restorePoint`、`targetNamespace` 引用 |
| **Export** (v0.9) | （Hub-Spoke 引入）| 把 Backup 发送到远端 BSL/cluster 的动作 |

每个 Action 都带：
- **status**: running / completed / failed / partial / skipped
- **phases**: 有序的多步检查清单（带 ✓/⏳/✗）
- **timing**: startTime / endTime / durationSeconds
- **refs**: protectedObject / policy / restorePoint / targetNamespace（按类型出现）
- **artifacts**: 这次动作涉及的资源总数

UI 上有一个 **Activity 页**（顶级导航），是所有 Action 的统一入口：
- 顶部 Action Duration 柱状图（按时间显示每次动作的耗时，按 status 着色）
- 中部 KPI 一行（total / completed / failed / skipped / avg duration / live artifacts / retired artifacts）
- 下方按时间倒序的 Action 卡片列表，每个卡片直接显示**Phases 实时清单**
- 点任意 Action → 抽屉显示完整详情 + Action YAML

**为什么单独立这个抽象**：v0.7.x 时期我们按 CRD 分页（Restore Points / Restores / Policies），用户必须学 Velero 的 CRD 分类才能找到"刚才做了什么"。v0.8.0 引入 Activity 后，用户回答的是任务问题（"我刚刚做了什么？现在怎么样了？"），不再被迫学实现细节。

**`Restores` 独立页已经废弃**，所有恢复历史都通过 Activity 页的 `type=Restore` 筛选查看（`/restores` URL 会自动 redirect）。

---

## 4. ⚠️ 重要陷阱与产品设计应对

这是手册最重要的章节。**这些坑每个 Velero 新用户都会踩**，SupKube 在产品层提供了对应的兜底。

### 4.1 Velero `existingResourcePolicy: none` 默认陷阱

#### 🐛 现象

执行 Restore 之后，UI 显示 Phase = **Completed**，items = `23/23`，**但应用的数据没有变化**。比如：原本 postgres 表里有 `world` 这条记录，Restore 备份点（备份点里只有 `hello`），完成后表里还是 `world`，仿佛 Restore 没生效。

#### 🔍 原因

Velero 的 `Restore.spec.existingResourcePolicy` **默认值是 `none`**。`none` 的含义是：**目标资源已存在时跳过，不动它**。

所以当你在原 namespace 上恢复时：
- Velero 看到 `Pod postgres-0` 已存在 → 跳过
- Velero 看到 `PVC postgres-data-postgres-0` 已存在 → 跳过
- Velero 看到 `Service postgres` 已存在 → 跳过

结果：所有 23 个 items 都被 "跳过但报告成功"，Phase 标记 Completed，**实际上等于什么都没做**。

#### ✅ SupKube 的解决方案

在 Restore Drawer（侧边抽屉）里：

- **就地恢复**（选择目标 ns = 原 ns）：弹出黄色警告 + 强制勾选确认框 "我已了解 - 删除并重建该命名空间"。后端会**先删除整个 namespace、轮询确认彻底消失（最长 60s），然后才创建 Velero Restore CR**。绕开 Velero 的默认跳过逻辑。
- **跨 ns 恢复**：默认 existingResourcePolicy 仍是 `none`（因为目标 ns 通常是空的，没有冲突）。提供 `update` 选项给高级用户。

保护名单：`velero`、`supkube`、`kube-system`、`kube-public`、`kube-node-lease`、`default` 不允许删除。

#### 📝 给客户的关键提示

> **Velero 默认的 `existingResourcePolicy: none` 会让"就地恢复"变成 no-op**。SupKube 把这个陷阱包成了"删除并重建 ns"的明确选项，所以用 UI 操作不会踩。
> **如果你直接用 `velero restore create` 命令行**：记得加 `--existing-resource-policy=update`，或者先 `kubectl delete ns <target>` 再 restore。

---

### 4.2 跨 ns / 跨 cluster Restore 冲突矩阵（关键章节）

#### 🐛 场景

客户在同一集群上备份某个 namespace，想恢复到一个**新的 namespace**（比如要测试这个应用是不是好用，不愿动原 ns）。或者想把生产备份恢复到测试集群。

按朴素直觉操作 → Velero 报错 `PartiallyFailed`，UI 只看到 `Errors (1)` 但点不开。

#### 🔍 真实原因（举例）

```
error restoring services/test-app-restore/adminer:
  Service "adminer" is invalid:
    spec.ports[0].nodePort: Invalid value: 30080: provided port is already allocated
```

NodePort 30080 是**集群级唯一资源**，原 ns 里的 adminer 已经占了。跨 ns 恢复时，Velero 试图创建一个新的 Service 拿同一个端口，K8s 直接拒绝。

#### 📊 完整冲突矩阵

下表列出所有**跨 ns / 跨 cluster Restore 可能触发的冲突类型**，是产品做 Pre-flight Check（v0.7.12）和 Transform Set（v0.8）的依据。

| # | 类别 | 典型字段 | 触发场景 | 推荐 Transform | 严重程度 |
|---|---|---|---|---|---|
| 1 | **NodePort 冲突** | `Service.spec.ports[].nodePort` | 跨 ns 恢复（端口已被原 ns 服务占用）| `remove`（让 K8s 自动分配空闲端口）| 🔴 Blocker |
| 2 | **clusterIP 冲突** | `Service.spec.clusterIP` | clusterIP 写死时的跨 ns 恢复 | `remove`（让 K8s 自动分配）| 🔴 Blocker |
| 3 | **loadBalancerIP 冲突** | `Service.spec.loadBalancerIP` | 静态 LB IP 被占 | `remove` | 🔴 Blocker |
| 4 | **Ingress host 冲突** | `Ingress.spec.rules[].host` | 同 host 在多 ns 共存，Ingress controller 拒绝 | `replace` 加目标 ns 前缀（`<targetNs>-`）| 🔴 Blocker |
| 5 | **PVC PV binding** | `PVC.spec.volumeName` | PVC 显式 bind 到具体 PV，PV 已被原 PVC 占 | `remove`（让 PVC 重新动态 bind）| 🔴 Blocker |
| 6 | **StorageClass 不可用** | `PVC.spec.storageClassName` | 跨 cluster：目标集群没有这个 SC | `replace` 为目标 cluster 的等价 SC | 🔴 Blocker |
| 7 | **Image registry 不一致** | `.spec.template.spec.containers[].image` | 跨 cluster / 跨 region：私有 registry 域名不可达 | `replace` registry 前缀（`docker.io` → `internal.registry.example.com`）| 🟡 Warning |
| 8 | **ServiceAccount 缺失** | `.spec.serviceAccountName` | 目标 ns 没有同名 SA | `replace` 为 `default` | 🟡 Warning |
| 9 | **NodeSelector 不存在** | `.spec.nodeSelector` | 跨 cluster：目标集群没有匹配的 Node 标签 | `remove` | 🟡 Warning |
| 10 | **PriorityClass 不存在** | `.spec.priorityClassName` | 跨 cluster：目标集群没定义这个 PriorityClass | `remove` | 🟡 Warning |

**为什么这张表很重要**：
- 每类冲突都有**确定的标准动作**，不是"随便改改试试看"。
- v0.7.12 的 Pre-flight Check **就是按这张表逐项检测**。
- v0.8 的 Transform Set **就是把这张表里的"推荐 Transform"做成可一键应用的模板**。
- 这是 SupKube 在 Restore 体验上**和原生 Velero 拉开差距的核心**。

#### ✅ SupKube 的解决方案（分阶段）

**v0.7.12 — Pre-flight Check（已交付）**：
- Restore Drawer 在用户选完目标 ns 后**自动运行**（StorageClass / NodePort / Ingress host 等快速检查）
- 检测到冲突列表显示，每条带"推荐 Transform"代码片段（YAML 预览）
- 含**🔴 Blocker** 时，"Restore" 按钮被禁用，除非用户明确勾选 "忽略冲突并继续"
- 用户至少**在点 Restore 之前就知道会失败**

**v0.8.2 — Transform Set + Apply Suggested Fix（已交付）**：
- 顶级导航新增 **变换集 / Transform Sets** 页面，CRUD 管理
- Pre-flight 的每条冲突的 **"一键修复 / Apply Suggested Fix"** 按钮已激活
- 点击 = 自动创建针对该冲突的临时 Transform Set（精确 namespace + 资源名匹配），并自动选中给当前 Restore
- 提交 Restore 时，SupKube 把 Transform Set 名字写入 `Restore.spec.resourceModifierRef`，Velero 在恢复时按 JSONPath patch 应用
- 内置 4 个模板（启动时自动种入 `velero` ns）：
  - `strip-nodeport`：去掉 Service 的 NodePort（让 K8s 重分配）
  - `strip-clusterip`：去掉 clusterIP
  - `strip-loadbalancer-ip`：去掉 loadBalancerIP
  - `strip-pv-binding`：去掉 PVC 的 volumeName（让 PVC 动态 bind 新 PV）
- 高级用户可在管理页面手写或克隆任意模板，做更复杂的 JSONPath patch（add / remove / replace / test / copy / move）

#### Transform Set 数据存储（实现细节）

Transform Set 直接存为 `velero` namespace 下的 ConfigMap：

| 字段 | 用途 |
|---|---|
| `metadata.name` | Transform Set 名字 |
| `metadata.labels."supkube.io/kind"` = `transform-set` | 让 SupKube 识别 |
| `metadata.labels."supkube.io/builtin"` = `true` | 内置模板（禁止修改/删除）|
| `metadata.labels."supkube.io/auto-generated"` = `true` | Pre-flight Apply Fix 临时生成（v0.9 自动 GC）|
| `data.description` | 人类可读说明 |
| `data."rules.yaml"` | Velero 直接消费的 YAML |
| `data."rules.json"` | 同上 JSON，前端展示用 |

Velero 1.13+ 在 `Restore.spec.resourceModifier` 接收 `TypedLocalObjectReference{Kind: ConfigMap, Name: <ts-name>}`，**和 SupKube 的 Transform Set 完全等价** —— 我们没造任何新 CRD，只是给原生 ResourceModifier 套了一层管理 UI。

#### 不通过 SupKube UI 也能用

由于底层就是 ConfigMap，你也可以纯 kubectl 操作：
```bash
# 列出当前所有 Transform Sets
kubectl get cm -n velero -l supkube.io/kind=transform-set

# 手工创建一个
kubectl apply -f my-transform-set.yaml

# Restore CR 引用它
spec:
  resourceModifier:
    kind: ConfigMap
    name: my-transform-set
```

#### 📝 给客户的关键提示

> 跨 ns / 跨 cluster Restore **结构上必然有冲突**，不是偶发 bug。每次跨 ns 操作前，看 Pre-flight Check 区块（v0.7.12+），照着推荐 Transform 改 backup 或者用 Transform Set（v0.8+）。
>
> **如果在 v0.8 之前需要立即解决**，可以手工创建 Velero ResourceModifier ConfigMap：
> ```yaml
> apiVersion: v1
> kind: ConfigMap
> metadata:
>   name: strip-nodeport
>   namespace: velero
> data:
>   rules.yaml: |
>     version: v1
>     resourceModifierRules:
>       - conditions:
>           groupResource: services
>         patches:
>           - operation: remove
>             path: "/spec/ports/0/nodePort"
> ```
> 然后用 `velero restore create --resource-modifier-configmap=strip-nodeport` 触发。

---

### 4.3 BackupSyncController 自动重建删除的 CR

#### 🐛 现象

通过 SupKube 删除了一个 Restore Point，列表里消失，过 1–2 分钟**又回来了**。怀疑是不是删除功能有 bug。

#### 🔍 原因

Velero 有个 `backup-sync-controller`，**默认每 60s 扫一遍 BSL（对象存储）**。如果 BSL 上还有 backup 数据（tarball + metadata），但集群里没有对应的 Backup CR，它就**自动重建 CR**。

也就是说，单纯 `kubectl delete backup <name>` 只删 K8s 资源，不删存储里的实际数据，结果 sync controller 又给恢复回来了。

#### ✅ SupKube 的解决方案

UI 上的 "Delete Restore Point" 走的是 Velero 的 `DeleteBackupRequest` CRD，**不是直接删 Backup CR**。Velero 的 backup-deletion-controller 看到 DeleteBackupRequest 后会：

1. 删除 BSL 里的 backup tarball + metadata
2. 删除关联的 CSI VolumeSnapshot + VolumeSnapshotContent
3. 删除关联的 PodVolumeBackups（Restic/Kopia 模式）
4. **最后**删除 Backup CR 本身

这样 sync controller 来回扫，发现存储和 CR 双双不存在，不会重建。

UI 在删除时会弹出确认框，明确列出"会被级联删除的内容"。

#### 📝 给客户的关键提示

> **不要用 `kubectl delete backup <name>`** 手工删除，会被 BackupSyncController 重建。用 SupKube UI 或者 `velero backup delete <name>`。

---

### 4.4 CSI Snapshot vs Filesystem 备份的取舍

| 维度 | CSI Snapshot | Filesystem (Restic/Kopia) |
|---|---|---|
| **速度** | 秒级（块级别快照）| 分钟到小时级（要扫文件系统）|
| **兼容性** | 需要 StorageClass 支持 CSI snapshot | 任意 StorageClass |
| **一致性** | Crash-consistent（运行中的 DB 可能不一致）| 同上，除非配 Kanister hook |
| **去重压缩** | 取决于存储 | Kopia 内置去重和压缩 |
| **跨集群恢复** | 难（需要相同 CSI driver）| 简单（数据 portable）|
| **占用 BSL 空间** | 小（增量）| 大（按文件大小）|

**SupKube 的建议（默认值）**：

- **核心数据库 PVC**：CSI Snapshot（速度快，PV 不动）+ Kanister hook（v0.9 提供一致性保证）
- **日志/媒体/可重建数据**：Filesystem（便宜，跨集群好用）
- **不知道选什么时**：Filesystem。CSI 出问题（VSC 状态卡住、driver 不兼容）的排障复杂度比 FS 高 5 倍。

---

## 5. 安装与升级

### 5.1 前置依赖

| 组件 | 最低版本 | 说明 |
|---|---|---|
| Kubernetes | 1.24+ | 1.27+ 推荐 |
| Velero | 1.13.0+ | 1.14.0+ 才有完整 CSI 集成 |
| Helm | 3.0+ | |
| 对象存储 | 任意 S3-compatible | MinIO 也可（本地测试）|
| CSI Snapshotter | external-snapshotter v8.0+ | 仅 CSI 模式需要 |

### 5.2 Helm 安装

```bash
# 1. 先装好 Velero（参考 https://velero.io/docs/main/basic-install/）
# 2. 装 SupKube
helm install supkube ./supkube-helm/supkube \
  -n supkube --create-namespace \
  --set backend.image.tag=0.7.11-alpha \
  --set frontend.image.tag=0.7.11-alpha
```

### 5.3 升级 / 版本切换

```bash
# 重置 user-supplied values（不然旧的 --set 会粘住）
helm upgrade supkube ./supkube-helm/supkube -n supkube --reset-values
kubectl rollout status deploy/supkube-backend -n supkube
kubectl rollout status deploy/supkube-frontend -n supkube
```

**已知坑**：早期版本如果用 `--set backend.image.tag=...` 装过，后续 `helm upgrade` 不带 `--reset-values` 会保留旧 tag，看起来"升级了但 UI 还是旧版本"。一律加 `--reset-values`。

### 5.4 卸载

```bash
helm uninstall supkube -n supkube
# 不会删除 velero 资源，BSL 数据也不会动
```

### 5.5 如何对外暴露 UI（4 种模式）

> 前端 UI 怎么对外访问，是**运维方在安装时（Day-0）的选择**，不是产品替你定死的（PRD-014）。Chart 只给一个到处能跑的安全默认值（NodePort），你按自己的集群环境选其一即可。**装完 `helm install` 后终端会按你选的模式打印对应的访问步骤**（见 NOTES）。改模式 = 改参数 + `helm upgrade`，不用碰镜像。

| 模式 | 适用场景 | 怎么访问 |
|---|---|---|
| **NodePort**（默认） | docker-desktop / on-prem，节点在内网可达 | `http://<node-ip>:30888/` |
| **LoadBalancer** | 公有云 AKS/GKE/EKS，自动拿公网 IP | `http://<EXTERNAL-IP>/` |
| **ClusterIP** | 安全敏感 / 气隙 / 平时不暴露、用时才连 | `kubectl port-forward` 后 `http://localhost:8080` |
| **Ingress** | 生产：域名 + TLS | `https://<your-host>/` |

```bash
# 公有云：开 LoadBalancer，等公网 IP
helm upgrade supkube ./supkube-helm/supkube -n supkube --reset-values \
  --set service.frontend.type=LoadBalancer
kubectl -n supkube get svc supkube-frontend -w   # 等 EXTERNAL-IP 不再是 <pending>

# 本地/内网：默认 NodePort
kubectl get nodes -o wide                          # 找一个 node IP
# 浏览器打开 http://<node-ip>:30888/

# 安全敏感 / 平时不暴露：ClusterIP + 用时 port-forward
helm upgrade supkube ./supkube-helm/supkube -n supkube --reset-values \
  --set service.frontend.type=ClusterIP
kubectl -n supkube port-forward svc/supkube-frontend 8080:80
# 浏览器打开 http://localhost:8080

# 生产：Ingress（域名 + TLS，需自备 ingress controller）
helm upgrade supkube ./supkube-helm/supkube -n supkube --reset-values \
  --set ingress.enabled=true --set ingress.hosts[0].host=supkube.example.com
```

**几个要点：**
- **公有云的节点没有公网 IP** → 在 AKS/GKE/EKS 上 NodePort 从外网访问不到，必须用 LoadBalancer 或 Ingress（这是 `172.188.195.36` UI 失联事件的根因，详见 PRD-014）。
- **LoadBalancer 的公网 IP 是动态的** → 删/重建 Service 会换 IP。要固定地址，用云厂商注解绑预建静态 IP（Azure：`service.beta.kubernetes.io/azure-pip-name=<name>`）。
- **如果启用了内置 Dex 登录**，`auth.dex.publicURL` 必须填成浏览器实际访问 SupKube 的外部地址（和你这里选的暴露方式一致），否则登录后会跳回错误地址（见 §11 认证配置 / `dex-check.yaml` 安装时会 fail-fast 提示）。
- 开发期想要"秒级看 UI"，直接用 `./hack/dev-local.sh --mode ui`（本质就是 ClusterIP + port-forward 模式，见 FAST-DEBUG-MODE.md）。

### 5.X 平台支持矩阵（Support Matrix）

> SupKube 的备份/还原能力依赖底层 K8s 的 **CSI 驱动**实现 `VolumeSnapshot`。不同云厂商 / 发行版在快照对象生命周期、`DeletionPolicy`、`VolumeBindingMode` 上有差异 → **每个目标平台必须走查后才能放进"已支持"列表**。
>
> **走查方法**：见 `测试用例.md §18 CSI 平台适配走查 (TC-CSI)`。
> **自动检测**：装机前跑 `hack/preflight.sh`；运行中由产品内 **#109 Preflight**（v0.9.x+）自动识别 CSI 驱动 + 能力 + 比对本矩阵。
>
> **状态图例**：✅ Verified · 🟡 Planned · 🔬 Research · ⚠ Limited · ❌ Not supported

#### Public Cloud

| 平台 | CSI 驱动 | 已测版本 | 状态 | 备注 |
|---|---|---|---|---|
| **Azure AKS** | `disk.csi.azure.com` | k8s 1.34 + Velero 1.18.0 (chart 12.0.1) | **✅ Verified（2026-05-30）** | snapshotMoveData=false 存储快照保留 + 还原已实证；v1.18 binary 必须配 v1.18 CRD（见 #102 frankenstein 教训） |
| AWS EKS | `ebs.csi.aws.com` | — | 🟡 Planned | — |
| GCP GKE | `pd.csi.storage.gke.io` | — | 🟡 Planned | — |
| Alibaba Cloud ACK | `diskplugin.csi.alibabacloud.com` | — | 🟡 Planned | — |
| Tencent Cloud TKE | `com.tencent.cloud.csi.cbs` | — | 🟡 Planned | — |

#### Private Cloud

| 平台 | CSI 驱动 | 已测版本 | 状态 | 备注 |
|---|---|---|---|---|
| VMware Tanzu (vSphere) | `csi.vsphere.vmware.com` | — | 🟡 Planned | — |
| Red Hat OpenShift | varies | — | 🟡 Planned | 与 OADP（OpenShift 的 Velero Operator）共存策略待评估 |
| Rancher (RKE2 / RKE) | varies（按底层 CSI） | — | 🟡 Planned | — |
| KubeSphere | varies | — | 🟡 Planned | — |

#### Desktop / Edge

| 平台 | CSI 驱动 | 已测版本 | 状态 | 备注 |
|---|---|---|---|---|
| **Docker Desktop** | `hostpath.csi.k8s.io` | Velero 1.18.0 | **✅ Verified（2026-05-30，测试驱动）** | csi-hostpath 测试驱动；`.snap` 文件在 backup delete 时不真删（累积孤儿，生产驱动不会） |
| K3s（默认） | `local-path-provisioner` | — | ⚠ Limited | 默认 local-path **不支持** CSI snapshot；如需快照请自装 CSI 驱动（longhorn-csi / openebs-csi 等） |
| KubeEdge | varies (edge) | — | 🔬 Research | 边缘节点的快照行为待调研 |

> **想把你的平台加进矩阵?** 跑一遍 `测试用例.md §18` 的走查模板，把报告发给我们，通过后即录入。

---

## 6. 操作指南

### 6.1 创建 Restore Point（备份）

> **两种入口**：
> - **一键即时快照**（v0.8.9.2+，推荐用于"上线前回滚点"场景）—— Applications 页 ⋮ → **Snapshot**。零配置，硬编码为本集群 CSI 快照、24h TTL。详见 6.1.1
> - **完整自定义备份** —— Restore Points → **Create Restore Point**，可选 TTL / Storage Profile / 备份模式。详见 6.1.2
>
> 何时用哪个？
> - 上线 / 改 config / 高风险操作前 → 一键 Snapshot（按一下就是 RP，与策略无关）
> - 需要跨集群、需要长期保留、需要 Data Mover 上传到对象存储 → 完整自定义（或者干脆配 Policy）

#### 6.1.1 一键即时快照（v0.8.9.2+）

1. 进入 **Applications** 页面，定位目标应用所在 namespace 行
2. 点击行尾 **⋮** → **Snapshot**
3. 弹出对话框：可选填一行备注（推荐填 _"上线 v1.2.3 前"_ 之类，事后审计找得到）
4. 点 **立即快照**
5. Toast 提示 "Snapshot of \<ns\> started"，跳到 Restore Points 即可看到新行
   - Policy 列显示紫色徽章 📸 **即时快照**（不是 `(manual)` 灰字）
   - 鼠标悬停徽章看 tooltip：会标出 "由 \<user\> 在应用页一键创建" + 备注

技术上等价于：本集群 CSI VolumeSnapshot + Velero 元数据 tarball 写入默认 Storage Profile，**不上传 PV 数据**。所以速度快（秒级触发）、但**离开集群就没用**（kube-apiserver 或 CSI driver 没了就找不回数据）。

适合：上线回滚点、配置变更前快照、demo 现场重置。
不适合：异地灾备、跨集群恢复 —— 用 Policy + L2 Snapshot & Export 模式。

#### 6.1.2 完整自定义备份

1. 进入 **Restore Points** 页面
2. 点 **Create Restore Point**
3. 填表：
   - **Name**：DNS-1123 合法字符
   - **Included Namespaces**：留空 = 整个集群
   - **TTL**：保留时长（默认 `720h` = 30 天）
   - **Storage Location (Profile)**：选已配置好的 BSL
   - **Include Volumes** + **Volume Backup Mode**：选 Filesystem 或 CSI
4. 提交。新行出现在列表，phase 从 `InProgress` → `Completed`（通常 30s-5min）

### 6.2 恢复（v0.7.10+ 新流程）

1. **Restore Points** 列表 → 目标行的 **⋮** 菜单 → **Restore**
2. 侧边抽屉打开：
   - **Application Name**：选目标 namespace
     - 默认 = 原 namespace
     - 可以选别的 ns，或者点 "Create A New Namespace" 现场新建
   - 如果选了原 ns，会弹出 **黄色警告** + 强制确认 "我已了解 - 删除并重建该命名空间"
   - 否则显示 **Optional Restore Settings**：`existingResourcePolicy` 选择（Skip / Update）
3. **Pre-flight Check**（v0.7.12+）：自动列出冲突 + 推荐 Transform。有 Blocker 时 Restore 按钮禁用。
4. **Spec (N)**：每个会恢复的组件勾选/反选，可展开查看 YAML
5. 点 **Restore**，进度可在 **Restores** 页面看到

### 6.3 查看详细错误（Restore Failed / PartiallyFailed）

1. **Restores** 页面 → 行的 **View Results**
2. 抽屉右侧显示：
   - Phase / Started / Completed / Progress
   - **Errors (N)** + **Warnings (N)**：v0.7.11+ 自动展开每条具体消息（按 Namespace 分组）
3. 如果显示 "Could not fetch detail"，说明 Velero `DownloadRequest` 没在 20s 内出 URL —— 通常是 BSL 网络问题，看 velero pod 的日志

### 6.4 删除 Restore Point

- 用 UI 的 ⋮ → **Delete**（走 DeleteBackupRequest CRD，会级联删除存储数据）
- 永远不要 `kubectl delete backup <name>`（会被 sync controller 重建，见 4.3）

---

## 7. 故障排查

### 7.1 Restore Completed 但应用数据没变

→ 见 [4.1](#41-velero-existingresourcepolicy-none-默认陷阱)。用 UI 重做并勾选"删除并重建命名空间"。

### 7.2 Restore PartiallyFailed，Errors (N) 看不到详情

→ 升级到 v0.7.11+；如果已经在 v0.7.11+，看 backend pod 日志：
```bash
kubectl -n supkube logs deploy/supkube-backend | grep DownloadRequest
```
常见原因：BSL 不可达（防火墙 / 凭据过期）。

### 7.3 Pre-flight Check 一直 loading

→ v0.7.12+ 才有这功能。如果勾选了 "Deep check (image registry)"，可能在等 DNS 查询，最长 10s。可以取消勾选试试。

### 7.4 删除 Restore Point 后又自动出现

→ 见 [4.3](#43-backupsynccontroller-自动重建删除的-cr)。检查 UI 删除是不是真的走了 DeleteBackupRequest：
```bash
kubectl get deletebackuprequests -n velero -w
```

### 7.5 跨 cluster 恢复全部资源失败

→ 见 [4.2 冲突矩阵](#42-跨-ns--跨-cluster-restore-冲突矩阵关键章节)。挨个对照表里的 10 类冲突排查。

### 7.6 CSI Snapshot 备份卡在 InProgress

→ 检查 VolumeSnapshot / VolumeSnapshotContent 状态：
```bash
kubectl get volumesnapshots -A
kubectl get volumesnapshotcontents
```
常见原因：external-snapshotter 没装、VolumeSnapshotClass 不存在、CSI driver 不支持快照。

### 7.7 升级后 UI 还显示旧版本号

→ 见 [5.3](#53-升级--版本切换)。一律加 `--reset-values`。

### 7.8 Helm 升级后 RBAC 报权限错误

→ 新版本可能加了新的 ClusterRole 规则。`--reset-values` 之后 ClusterRole 会被重新 apply，应该自动修复。如果还不行：
```bash
kubectl get clusterrole supkube -o yaml
helm get manifest supkube -n supkube | grep -A 30 ClusterRole
```

---

## 8. RBAC 权限说明

SupKube 用一个 cluster-wide 的 ClusterRole。**为什么是 cluster-wide 而不是 namespace-scoped**：备份/恢复跨 namespace 操作是核心需求，namespace-scoped 实现起来 RoleBinding 数量爆炸。v0.8 OIDC + 多租户落地后会增加细粒度委托（Operator 只能管自己 ns）。

| 权限 | 用途 |
|---|---|
| `velero.io/*: get/list/watch/create/update/delete` | 操作 Backup / Restore / Schedule / BSL / VSL / DeleteBackupRequest / DownloadRequest |
| `snapshot.storage.k8s.io/*: get/list/watch` | 读 CSI VolumeSnapshot 状态 |
| `storage.k8s.io/{storageclasses,csidrivers}: get/list/watch` | Policy 能力检测 (Filesystem vs CSI) |
| `core/namespaces: create/delete` | Restore Drawer 的 "Create New Namespace" + cleanupBeforeRestore |
| `core/{pods,services,pvc,configmaps,nodes,serviceaccounts}: get/list` | Artifacts 枚举 |
| `core/secrets: full` | BSL 凭据 Secret 的 CRUD |
| `apps/{deployments,statefulsets,daemonsets,replicasets}: get/list` | Applications 列表 + Artifacts |
| `batch/*: get/list`、`networking.k8s.io/*: get/list`、`rbac.authorization.k8s.io/*: get/list` | Artifacts 枚举（v0.7.10+）|

---

## 9. 术语表

| 术语 | 含义 | 备注 |
|---|---|---|
| **BSL** | BackupStorageLocation | Velero CRD，对象存储位置 |
| **VSL** | VolumeSnapshotLocation | Velero CRD，CSI 快照位置 |
| **DBR** | DeleteBackupRequest | Velero CRD，触发级联删除 |
| **DR** | DownloadRequest | Velero CRD，请求详细 log/results 的预签名 URL |
| **CSI** | Container Storage Interface | K8s 存储插件标准 |
| **VSC** | VolumeSnapshotClass | CSI 快照类，每个 StorageClass 对应一个 |
| **L1/L2/L3** | 保护级别 | Snapshot / Snapshot+Export / Snapshot+Immutable Export |
| **Transform Set** | 一组 JSONPath patch 规则 | v0.8 提供；对应 Velero 的 `resourceModifierRef` |
| **Pre-flight Check** | Restore 前的冲突检测 | v0.7.12+ |
| **Restore Point** | 用户视角的 Backup | 与 `Backup` CRD 同义 |
| **Storage Profile** | 用户视角的 BSL | |
| **Snapshot Profile** | 用户视角的 VSL | |
| **Policy** | 用户视角的 Schedule | Schedule + 保留策略的组合 |
| **Local / Imported** | Restore Point 来源 | 基于 cluster 指纹判断 |
| **fingerprint** | cluster 指纹 | `velero.io/source-cluster-k8s-gitversion` 等注解 |

---

## 10. Roadmap 摘要

| 版本 | 主要内容 | 状态 |
|---|---|---|
| v0.5 | MVP：Backup/Restore/Schedule CRUD + BSL/VSL | ✅ |
| v0.6 | CSI Snapshot 双模式、Velero EnableCSI 集成 | ✅ |
| v0.7.0 ~ v0.7.6 | Backup Advisor、Dark mode、i18n、ECharts、Kasten Policies 重做 | ✅ |
| v0.7.7 ~ v0.7.9 | 真级联删除 (DeleteBackupRequest)、Restore Points Kasten 化 | ✅ |
| v0.7.10 | Restore Drawer (Kasten 风格) + Spec 组件清单 + cleanupBeforeRestore | ✅ |
| v0.7.11 | Restore 详细错误展示 (DownloadRequest 流程) | ✅ |
| v0.7.12 | Pre-flight Check（10 类冲突检测 + 推荐 Transform 只读展示）| ✅ |
| v0.7.13 | Applications → Restore 跳转链路修正 + Restore Points 命名空间 chip 筛选 | ✅ |
| v0.8.0 | Activity / Action 统一抽象、Activity 页、Restores 独立页废弃 | ✅ |
| v0.8.2 | Transform Set 编辑器 + Apply Suggested Fix + 4 个内置模板 | ✅ |
| v0.8.3 | ConfigMap 格式修复（单 key）+ 批量 Apply Fix + hasBlockers 修复 + Action 排序修复 | ✅ |
| v0.8.4 | Velero 三种 patch 类型分场景使用（mergePatch / strategicPatch / JSON Patch），消除假性 PartiallyFailed | ✅ |
| **v0.8.5** | **生产门槛包：OIDC 认证 + 基础 RBAC（3 层）+ 审计日志 + Policy 真编辑表单 + Token/Basic Auth fallback** | **🚧 步骤 1/6 完成（Dex + 静态用户登录）** |
| v0.8.6 | **Data Usage：实时 BSL 查询（AWS/GCS/Azure SDK 全接）+ Dashboard Data 卡片 + Data Usage 详情页（4 个时序图：Snapshot Storage / Object Storage / Object Storage by Application / Live Storage）** | 已设计 |
| v0.9 | 商业卖点 + Q3 顺手包：L3 Immutable + Backup 详细错误 + Monaco YAML 编辑器 + Advisor→Policy 一键应用 + 临时 TS 自动 GC + 删除 Restores legacy | 计划 |
| v0.10 | 平台化：Kanister Blueprints + Hub-Spoke 多集群（并行）| 计划 |
| v0.11+ | 待客户反馈定 | — |

---

## 11. 认证配置：4 大 IdP 快速集成（v0.8.5+）

SupKube 通过内嵌 Dex 支持任意 OIDC 提供方。下面是 4 个最常见 IdP 的"在 IdP 端怎么注册应用 + 在 SupKube 端怎么配 Helm values"对照速查。

完整 connector 字段参考 Dex 官方文档：https://dexidp.io/docs/connectors/

### 11.1 通用步骤

无论用哪个 IdP，三步都一样：

1. **在 IdP 端注册应用**（创建一个 OAuth 客户端）
2. **设置 redirect URI** = `<SupKube 公开 URL>/dex/callback`（注意是 `/dex/callback`，不是 `/auth/callback`）
3. **把 IdP 给你的 client_id + client_secret 填入 SupKube 的 `values.yaml`**

```yaml
# values.yaml
auth:
  dex:
    enabled: true
    publicURL: "https://supkube.example.com"     # 客户的公开访问 URL
    issuer:    "https://supkube.example.com/dex"
    connectors:
      - type: oidc
        id:   keycloak
        name: "Login with Keycloak"
        config:
          issuer: ...
          clientID: ...
          clientSecret: ...
          redirectURI: "https://supkube.example.com/dex/callback"
```

### 11.2 Keycloak

**在 Keycloak 端**：
1. 创建 / 切换到目标 Realm
2. Clients → Create client → ID: `supkube`，类型 `OpenID Connect`
3. Client authentication: **ON**
4. Authentication flow: 勾选 Standard flow
5. Valid Redirect URIs: `https://supkube.example.com/dex/callback`
6. 进入 Credentials 标签复制 Client secret
7. （可选）Client scopes → 添加 `groups` mapper（Group Membership 类型，Token Claim Name = `groups`）

**values.yaml**：
```yaml
connectors:
  - type: oidc
    id: keycloak
    name: "Login with Keycloak"
    config:
      issuer: https://keycloak.example.com/realms/master
      clientID: supkube
      clientSecret: <CREDENTIALS 标签的 Client secret>
      redirectURI: https://supkube.example.com/dex/callback
      scopes: [openid, profile, email, groups]
      getUserInfo: true
```

### 11.3 Okta

**在 Okta 端**：
1. Applications → Create App Integration → OIDC - OpenID Connect → Web Application
2. Sign-in redirect URIs: `https://supkube.example.com/dex/callback`
3. Assignments：把目标用户/组 assign 给这个 app
4. 创建后在 General 标签复制 Client ID + Client secret
5. Sign On → OpenID Connect ID Token → Groups claim type: **Filter**，filter: `Matches regex .*`（或者按需）

**values.yaml**：
```yaml
connectors:
  - type: oidc
    id: okta
    name: "Login with Okta"
    config:
      issuer: https://your-tenant.okta.com
      clientID: 0oaXXXXXX
      clientSecret: <Client secret>
      redirectURI: https://supkube.example.com/dex/callback
      scopes: [openid, profile, email, groups]
```

### 11.4 Azure AD / Entra ID

**在 Azure 端**：
1. Azure Portal → Entra ID → App registrations → New registration
2. Name: `SupKube`，Supported account types: 按需选择
3. Redirect URI: Web → `https://supkube.example.com/dex/callback`
4. 注册后记下 **Application (client) ID** + **Directory (tenant) ID**
5. Certificates & secrets → New client secret，记下 Value
6. Token configuration → Add groups claim（让 JWT 带 groups）
7. API permissions → Microsoft Graph → 添加 `email`、`openid`、`profile`、`User.Read`

**values.yaml**：
```yaml
connectors:
  - type: oidc
    id: azuread
    name: "Login with Microsoft"
    config:
      issuer: https://login.microsoftonline.com/<TENANT_ID>/v2.0
      clientID: <Application (client) ID>
      clientSecret: <Client secret Value>
      redirectURI: https://supkube.example.com/dex/callback
      scopes: [openid, profile, email]
      getUserInfo: true
```

### 11.5 GitHub

**在 GitHub 端**：
1. Settings → Developer settings → OAuth Apps → New OAuth App
2. Authorization callback URL: `https://supkube.example.com/dex/callback`
3. Application name: `SupKube`
4. 注册后复制 Client ID + 生成 Client secret

**values.yaml**：
```yaml
connectors:
  - type: github
    id: github
    name: "Login with GitHub"
    config:
      clientID: <Client ID>
      clientSecret: <Client secret>
      redirectURI: https://supkube.example.com/dex/callback
      loadAllGroups: true
      # 可选：限制只允许特定 org 的成员
      # orgs:
      #   - name: my-org
      #     teams: [platform-admins]
```

### 11.6 ⚡ 生产环境必读：client secret 不要明文进 Git

上面 4 节的示例为了直观把 `clientSecret` 直接写在 values.yaml 里。**生产环境永远不要这么做** —— values.yaml 通常在 Git 里。SupKube 内嵌 Dex 支持从 K8s Secret 通过环境变量注入凭据：

#### 三步法

**Step 1：把所有 IdP 的凭据放进一个 K8s Secret**

```bash
kubectl create secret generic supkube-oauth -n supkube \
  --from-literal=GITHUB_CLIENT_ID=Iv1.abc... \
  --from-literal=GITHUB_CLIENT_SECRET=xyz... \
  --from-literal=KEYCLOAK_CLIENT_SECRET=... \
  --from-literal=OKTA_CLIENT_SECRET=... \
  --from-literal=AZURE_CLIENT_SECRET=...
```

或者用 sealed-secrets / external-secrets-operator / vault-injector，让 Secret 来源安全可审计。

**Step 2：让 Dex 把这个 Secret 加载成环境变量**

```yaml
# values.yaml
auth:
  dex:
    envFromSecrets:
      - secretName: supkube-oauth
```

也可以引用多个 Secret（按 IdP 分开管理）：
```yaml
    envFromSecrets:
      - secretName: supkube-github
      - secretName: supkube-keycloak
      - secretName: supkube-azure
```

或者只取 Secret 里的某一个 key：
```yaml
    env:
      - name: GITHUB_CLIENT_SECRET
        valueFrom:
          secretKeyRef:
            name: supkube-github
            key: client-secret
```

**Step 3：connector 配置用 `$VAR` 占位**

Dex 在加载 `config.yaml` 时会自动用环境变量替换 `$VAR_NAME`：

```yaml
auth:
  dex:
    envFromSecrets:
      - secretName: supkube-oauth
    connectors:
      - type: github
        id: github
        name: "Login with GitHub"
        config:
          clientID: "$GITHUB_CLIENT_ID"           # ← 不写明文
          clientSecret: "$GITHUB_CLIENT_SECRET"   # ← 不写明文
          redirectURI: "https://supkube.example.com/dex/callback"
      - type: oidc
        id: keycloak
        name: "Login with Keycloak"
        config:
          issuer: "https://keycloak.example.com/realms/main"
          clientID: "supkube"
          clientSecret: "$KEYCLOAK_CLIENT_SECRET"
          redirectURI: "https://supkube.example.com/dex/callback"
```

#### Secret 轮换

K8s Secret 内容变更**不会自动重启 Dex**（env var 在容器启动时一次性写入）。轮换时需要手动触发：

```bash
kubectl rollout restart deploy/supkube-dex -n supkube
```

v0.9 计划集成 [reloader.stakater.com](https://github.com/stakater/Reloader) 注解，Secret 变更后自动滚动重启 Dex。

#### 验证 secret 注入工作正常

```bash
# 检查 Dex pod 拿到了正确的 env var
kubectl -n supkube exec deploy/supkube-dex -- printenv | grep -E "GITHUB|KEYCLOAK|OKTA|AZURE"

# 检查 Dex 启动 log 注册了 connector
kubectl -n supkube logs deploy/supkube-dex | grep "config connector:"
# 预期：config connector: github 等
```

如果 Dex 启动 log 里 connector 缺失，意味着 `$VAR` 解析失败（env var 不存在 → 留 `$VAR` 字面值 → Dex 拒绝把它当成 OIDC 凭据）。检查 Secret 名拼写、key 名（大小写敏感）。

---

### 11.7 多 IdP 共存

`connectors` 是数组，可以同时配多个：

```yaml
connectors:
  - type: oidc
    id: keycloak
    name: "公司 SSO"
    config: { ... }
  - type: github
    id: github
    name: "开发者 GitHub"
    config: { ... }
```

—— 登录页就会显示 2 个按钮（外加 1 个 "Username & Password" 如果还启用了 staticPasswords）。

### 11.8 部署 + 验证

```bash
# 应用 values
helm upgrade supkube ./supkube-helm/supkube -n supkube --reset-values

# 等待 Dex + backend 重启（ConfigMap 改变会触发 rollout）
kubectl rollout status deploy/supkube-dex -n supkube
kubectl rollout status deploy/supkube-backend -n supkube

# 验证 backend 收到了正确的 provider 元数据
kubectl -n supkube exec deploy/supkube-backend -- \
  printenv AUTH_PROVIDERS_JSON
```

预期输出（举例）：
```json
[{"id":"local","name":"Username & Password","type":"password"},
 {"id":"keycloak","name":"Login with Keycloak","type":"oidc"}]
```

打开浏览器 → SupKube 登录页 → 应该看到对应数量的按钮。

### 11.9 常见坑

| 现象 | 原因 | 修复 |
|---|---|---|
| Dex 报 `Invalid redirect URI` | IdP 端注册的 redirect URI 和 publicURL/dex/callback 不一致 | 完全一致（含协议、端口、路径，结尾不要 `/`）|
| 登录后 groups 为空 | IdP 没把 groups claim 加到 ID Token | 各家配置不同，见上面每节最后一条 |
| Dex 启动报 `connectors[0].config: ...` | YAML 嵌套对齐错了 | 注意 `config:` 下面的字段相对 `- type:` 多缩进 2 空格 |
| 后端 401 但 Dex log 显示登录成功 | TokenURL / JWKS URL 不通（v0.8.5 step 1 的双 URL 问题）| 检查 `auth.dex.publicURL` vs cluster-internal Service URL，确保 Helm 渲染正确 |

---

## 12. RBAC：3 角色权限模型（v0.8.5 step 3）

### 12.1 模型概述

SupKube 用 **3 个固定角色**：

| 角色 | 权限 |
|---|---|
| **admin** | 全集群 CRUD（Storage Profiles、Snapshot Profiles、Transform Sets、namespaces、所有 Backup/Restore/Policy 跨 ns）|
| **editor** | 指定 namespace 内的 Backup / Restore / Policy 写操作 + 全局只读 |
| **viewer** | 全集群只读 |

OIDC groups claim 映射到角色通过 `values.yaml` 配置（**不在 UI 上配**，参见 ADR-004）。

### 12.2 启用 RBAC

默认情况 `auth.rbac.enabled=false`（每个已认证用户都是 admin —— 向后兼容 v0.8.5 step 1/2 部署）。

生产部署应该启用：

```yaml
auth:
  rbac:
    enabled: true
    defaultRole: viewer         # 未匹配 binding 的用户兜底为 viewer
    bindings:
      - group: platform-admins
        role: admin
      - group: app-team-postgres
        role: editor
        namespaces: [postgres-prod, postgres-staging]
      - group: auditors
        role: viewer
      - user: admin@supkube.local   # 给静态用户绑定（不依赖 groups）
        role: admin
```

### 12.3 binding 字段说明

每个 binding 是 `{group | user, role, namespaces?}`：

| 字段 | 用途 |
|---|---|
| `group` | OIDC `groups` claim 里的值。优先于 `user` |
| `user` | 用户的 email 或 username（来自 token 的对应 claim）|
| `role` | `admin` / `editor` / `viewer` |
| `namespaces` | **仅 editor 必填**。admin 跨集群、viewer 全集群只读，都不需要 |

**多 binding 命中时的累积规则**：
- 用户的 groups 命中多个 binding → 取**最高角色**
- 同一 editor 命中多个 binding → namespaces **求并集**

### 12.4 Editor 的命名空间隔离

editor 角色**必须**在 binding 里指定 namespaces。空 namespaces = 没有任何 ns 可操作。

涉及命名空间的写操作（后端 enforce）：

| 操作 | 检查的 ns |
|---|---|
| 创建 Backup | `req.includedNamespaces` 每个都要在 scope 内 |
| 删除 Backup | Backup CR 的 `spec.includedNamespaces` |
| 创建 Restore | `includedNamespaces` + `namespaceMapping` 的源和目标都要 |
| 删除 Restore | 该 Restore 的目标 ns |
| 创建/编辑/删除 Schedule | Schedule 的 `spec.template.includedNamespaces` |
| Pre-flight Check | `targetNamespace` |
| Run Once | 同 Schedule |

**editor 不能做的事**：
- 创建/编辑/删除 BSL（Storage Profile）→ admin 专属
- 创建/编辑/删除 VSL（Snapshot Profile）→ admin 专属
- 创建/编辑/删除 Transform Set CRUD → admin 专属（**但**通过 Pre-flight 一键修复创建临时 TS 是允许的）
- 创建 namespace → admin 专属
- 查看审计日志 → admin 专属（v0.8.5 step 4）

### 12.5 验证 RBAC 工作正常

```bash
# 1. 查看后端注入的配置
kubectl -n supkube exec deploy/supkube-backend -- printenv RBAC_ENABLED
kubectl -n supkube exec deploy/supkube-backend -- printenv RBAC_BINDINGS_JSON | python3 -m json.tool

# 2. 当前登录用户的解析结果
curl -H "Authorization: Bearer <your-token>" http://localhost:30888/api/v1/auth/me | python3 -m json.tool
# 关注响应中的 user.role / user.namespaceScope
```

UI 头部用户徽章右侧会显示红/黄/绿色角色 chip：admin / editor / viewer。

### 12.6 常见坑

| 现象 | 原因 | 修复 |
|---|---|---|
| 启用 RBAC 后所有人都看不到东西 | bindings 没配，defaultRole 是 viewer，但页面写按钮被 disabled 当然是正常的 | 确认 bindings 有命中、用户登录后看 /auth/me 的 role 字段 |
| editor 报 403 "namespace not in your scope" | binding 的 namespaces 列表里没有这个 ns | 把 ns 加进 binding 或换更大权限的 binding |
| 静态用户（Dex local 密码）登录后被识别为 viewer | static 用户的 groups claim 默认空 | 用 `user: admin@supkube.local` 类型 binding 直接绑用户 |
| 改了 bindings 后用户行为没变 | backend 启动时一次性读 env，重启才生效 | `kubectl rollout restart deploy/supkube-backend -n supkube` |

### 12.7 RBAC 关闭和打开的过渡策略

如果你已经有 v0.8.5 step 2 的部署在运行（多用户但没有 RBAC），切换到启用 RBAC 的流程：

1. **第一阶段**：`rbac.enabled=true, defaultRole=admin, bindings=[]` —— 验证 backend 和 frontend 没新 bug，所有用户依然全权
2. **第二阶段**：开始加 bindings，但 `defaultRole=admin` 保持兜底（避免误伤未列出的用户）
3. **第三阶段**：把 `defaultRole` 改为 `viewer`，未绑定的用户降权为只读，但能登录
4. **第四阶段**（可选）：把 `defaultRole` 改为空字符串，未绑定的用户彻底拒绝

每个阶段持续 1-2 周，给 IdP 团队同步 groups 配置的时间。

---

## 13. 审计日志（v0.8.5 step 4）

### 13.1 SupKube 审计的范围

**会被记录的**：
- 所有 POST / PUT / PATCH / DELETE 请求（"写操作"）
- 登录失败 / 拒绝认证
- RBAC 拒绝（403）

**不会被记录的**：
- GET / HEAD 等只读请求（量太大，价值低）
- `/api/v1/status` 健康检查
- 静态资源（前端 JS / CSS / 图标）

每条审计记录的字段：
| 字段 | 例子 |
|---|---|
| user | `admin@supkube.local` |
| action | `Create` / `Update` / `Delete` / `Patch` |
| resource | `Backup` / `Restore` / `Schedule` / `TransformSet` / `Namespace` / `StorageProfile` / `SnapshotProfile` |
| resourceName | `test-app-backup-001`（如适用）|
| namespace | `test-app`（如适用）|
| method | `POST` |
| path | `/api/v1/backups` |
| result | `success` / `denied` / `error` |
| statusCode | 200 / 403 / 500 等 |
| sourceIP | `192.168.65.3` |
| timestamp | ISO8601 |

### 13.2 在哪里看？

**UI**：登录 SupKube → Settings → **审计日志** 标签页（仅 admin 可见）

提供过滤：按用户、按 result、按 resource 类型、限制条数。

### 13.3 存储与保留期

**双写架构**：

```
HTTP 写请求
    │
    ▼
 ┌──────────────┐
 │ AuditMiddleware │
 └──┬───────────┬─┘
    │           │
    ▼           ▼
┌──────────┐  ┌──────────────────────┐
│  stdout  │  │  K8s Event (events/v1)│
│  (JSON)  │  │  in ns=supkube        │
└──────────┘  │  labels: audit=true   │
              └──────────────────────┘
    │              │
    ▼              ▼
 SIEM / Loki    UI (短期, K8s 默认 1h TTL)
 (长期保留)
```

| 用途 | 数据源 | 保留期 |
|---|---|---|
| **UI 查看（近期审计）** | K8s Events | 默认 1 小时（K8s `--event-ttl` flag 可调整）|
| **长期审计 / SIEM 合规** | backend stdout | 由客户的日志管道决定 |

**为什么 K8s Events？** 见 ADR-019。

### 13.4 长期保留 — 接入 SIEM

后端 stdout 输出的每行审计记录形如：

```
[audit] {"user":"admin@supkube.local","action":"Create","resource":"Backup","resourceName":"test-app-backup-001","namespace":"test-app","method":"POST","path":"/api/v1/backups","result":"success","statusCode":201,"sourceIP":"10.0.0.1","timestamp":"2026-05-21T22:10:34Z"}
```

JSON 格式 + 固定 `[audit]` 前缀，让任何日志管道都能轻松提取：

**Loki 查询示例**：
```logql
{namespace="supkube", container="backend"} |~ "\\[audit\\]"
| json
| user="admin@supkube.local"
```

**Splunk SPL 示例**：
```spl
index=kubernetes namespace=supkube container=backend "[audit]"
| spath
| where result="denied"
```

**Fluent Bit / FluentD parser**：
```ini
[PARSER]
    Name supkube_audit
    Format regex
    Regex ^\[audit\] (?<audit>{.*})$
```

### 13.5 让 K8s Events 在 UI 里保留更长

K8s 默认 1 小时 TTL 太短不适合调试。修改 kube-apiserver flag：

```yaml
# kube-apiserver
--event-ttl=720h    # 30 天
```

Docker Desktop / minikube 通常不让改 kube-apiserver flag，建议接入 SIEM 而不是依赖 K8s Events 长期保留。

### 13.6 增加审计字段（开发者扩展）

如果客户需要更细粒度的字段（例如 RequestSize、Latency），改 `internal/auth/audit.go::buildAuditRecord()`：

1. 在 `AuditRecord` struct 加新字段
2. 在 `buildAuditRecord()` 填入对应数据
3. 在 `writeAuditEvent()` 的 `annotations` 加 `supkube.io/audit-<新字段名>`
4. 在 `eventToAuditRecord()` 读出来
5. 在前端 `AuditLogPanel.vue` 的表格加一列

### 13.7 常见坑

| 现象 | 原因 | 修复 |
|---|---|---|
| 审计日志 UI 空白 | RBAC 启用了但用户不是 admin | 该 endpoint admin-only，普通用户用不了 |
| 审计日志只有最近 1 小时的数据 | K8s Events 默认 TTL | 见 §13.5，或接 SIEM |
| stdout 里看不到 `[audit]` 前缀的行 | backend 容器还没重启 | `kubectl rollout restart deploy/supkube-backend -n supkube` |
| 审计写 Event 失败 | RBAC 缺 events 权限 | 检查 ClusterRole 有 `events` 的 create 权限（v0.8.5 step 4 已默认有）|

---

## 14. 非 OIDC 登录：API Token 与 Basic Auth（v0.8.5 step 6）

OIDC + Dex 适合人类用浏览器登录，但对**机器调用**和**已部署反向代理**的场景过重。v0.8.5 step 6 引入两种 **fallback 认证方式**与 OIDC 并存：

| 方式 | 适用场景 | 启用方式 | RBAC 来源 |
|---|---|---|---|
| **API Token** | CI/CD、脚本、Terraform | `auth.staticTokens.enabled=true` + hash 列表 | 每个 token 自带 `role` + `namespaces` |
| **Basic Auth** | 公司反代已认证（mod_auth_kerb / OIDC2HTTP）、气隙环境 | `auth.basic.enabled=true` + mount htpasswd Secret | 同 OIDC 走 `auth.rbac.bindings`（按 username 匹配）|
| **OIDC**（默认） | 浏览器登录 | `auth.dex.enabled=true` / 外部 IdP | `auth.rbac.bindings`（按 group / email 匹配）|

三种方式**并行**：同一个 backend 同时接受三类 Authorization header，按 `Bearer <opaque>`、`Bearer <jwt>`、`Basic <b64>` 自动分发。

### 14.1 API Token

**思路**：管理员在工作站生成一个 256-bit 随机串作为 plaintext，把它的 SHA-256 hash 放进 Helm，plaintext 只放在调用方的密钥库（GitHub Secrets / Vault / `~/.netrc`）。即使 `helm get values` 泄漏也无法被冒用。

#### 生成 Token

```bash
# 在管理员工作站执行一次
PLAIN=$(openssl rand -hex 32)
echo "Plaintext (give this to CI): $PLAIN"
echo -n "$PLAIN" | sha256sum
# → <64-char hex> ← 这个放到 values.yaml
```

#### 配置 Helm

```yaml
# values.yaml
auth:
  staticTokens:
    enabled: true
    tokens:
      - name: github-actions-prod          # 出现在审计日志里
        hash: 0e9b62f3...3e3f1             # 上一步算出来的 sha256
        role: editor
        namespaces: [shop-prod, shop-staging]
      - name: terraform
        hash: 7c44a9...9b02
        role: admin
```

#### 使用

```bash
# CI 脚本：把 PLAIN 注入为 SUPKUBE_TOKEN
curl -H "Authorization: Bearer $SUPKUBE_TOKEN" \
     https://supkube.example.com/api/v1/backups
```

**安全说明**：
- 比较走 `crypto/subtle.ConstantTimeCompare` — 没有时序侧信道
- Token 不进入审计日志原文，只记录 `name` 字段
- 撤销：删除 values.yaml 对应行 → `helm upgrade` → 立刻生效
- 不打算支持运行时签发：那需要状态存储 + 撤销列表 + 审计签名，溢出 v0.8 范围（v0.10 考虑）

### 14.2 Basic Auth（htpasswd）

**思路**：把一个标准 Apache htpasswd 文件放进 K8s Secret，backend mount 到 `/etc/supkube/htpasswd`，启动时解析。**只接受 bcrypt 条目**，MD5 / SHA1 一律拒绝并打 warn 日志。

#### 生成 htpasswd

```bash
# 注意：必须用 -B 走 bcrypt
htpasswd -bnBC 10 alice 'alice-password' > htpasswd
htpasswd -bnBC 10 bob   'bob-password'  >> htpasswd

# 检查只有 bcrypt（行头是 $2y$ / $2a$ / $2b$）
cat htpasswd

# 推到集群
kubectl -n supkube create secret generic supkube-htpasswd \
  --from-file=htpasswd
```

#### 配置 Helm

```yaml
auth:
  basic:
    enabled: true
    secretName: supkube-htpasswd

  # Basic Auth 用户走和 OIDC 用户同一张 bindings 表（按 username 匹配）
  rbac:
    enabled: true
    bindings:
      - user: alice
        role: editor
        namespaces: [shop-prod]
      - user: bob
        role: viewer
```

#### 使用

```bash
curl -u alice:alice-password https://supkube.example.com/api/v1/backups
```

#### 反向代理场景

如果你的 nginx / Apache 已经认证了用户（Kerberos / SAML2Apache），把它转发到 SupKube 时塞 `Authorization: Basic ` header 即可。SupKube 只看 header，不关心怎么来的。

### 14.3 常见问题

| 问题 | 原因 | 解决 |
|---|---|---|
| CI 调用返回 `invalid API token` | hash 算错 / 没去掉 `\n` | `echo -n` 不要 `echo`，确保 64 位 hex |
| Token 调用走 OIDC 路径报 `invalid token` | Token 含点号被当 JWT | 别用包含 `.` 的 token；用 `openssl rand -hex` 干净 |
| htpasswd 启动后无效 | 文件用了 md5/sha1 | backend 日志有 `not bcrypt — skipping`，改用 `htpasswd -B` |
| `helm upgrade` 后 htpasswd 不生效 | 现版本 boot 时读一次 | 临时方案：`kubectl rollout restart deploy/supkube-backend`（v0.8.6 计划加 SIGHUP reload）|
| Basic Auth 用户没 RBAC 权限 | username 与 binding 不匹配 | bindings 里用纯 username（不带 @domain），与 htpasswd 第一列一致 |

---

## 15. 备份组成：数据路径与大小（v0.8.6）

每个还原点都会显示三类元数据，回答"这个备份究竟是怎么做的、多大"：

```
┌──────────────┬──────────────────────┬──────────────────────┐
│ 数据路径      │ 卷数据                │ 资源清单 (tar.gz)     │
│ 📸 CSI 快照   │ 6.0 GiB · 2 vols    │ 18 KiB              │
└──────────────┴──────────────────────┴──────────────────────┘
```

### 15.1 数据路径（Data Path）

Velero 把"如何保护 PV"委托给底层路径，SupKube 自动识别并打 chip：

| Chip | 物理性质 | 跨集群恢复 | 大小语义 |
|---|---|---|---|
| 📸 **CSI 快照** | 存储层 CoW，集群本地 | ❌ 除非启用 Data Mover | PVC 声明容量（**不**是实际使用）|
| 🚚 **Data Mover** | CSI 快照搬到对象存储（Kopia 后端）| ✅ | 去重后字节 |
| 📁 **文件系统** | Restic/Kopia 走 FS 读 | ✅ | 实际处理字节 |
| 📋 **仅元数据** | 只备份了 YAML，无 PV | ✅ | 0 卷 |

判断逻辑（首匹配）：

```
spec.snapshotMoveData      = true → data-mover
spec.defaultVolumesToFsBackup = true → filesystem
spec.snapshotVolumes       = true 且 status.csiVolumeSnapshotsCompleted > 0 → csi-snapshot
否则 → metadata-only
```

⚠️ 我们看 **status** 不只看 spec — 一个 spec 写了 `snapshotVolumes: true` 但实际一个 CSI 快照都没建出来（比如所有 PVC 都在不支持 snapshot 的 StorageClass 上）的备份会被归为"仅元数据"，因为它实质上没保护任何卷。

### 15.2 卷数据（Volume Data）

来源依赖路径：

- **CSI 快照**：从 `VolumeSnapshotContent.status.restoreSize` 求和。**注意**：这是 PVC 声明容量，对一个 100 GiB / 实际只用 2 GiB 的 PV 也会显示 100 GiB。这是 K8s/CSI API 的限制 — 想要"真实占用"必须走 Kopia 路径。
- **文件系统 / Data Mover**：从 `PodVolumeBackup.status.progress.totalBytes` 或 `DataUpload.status.progress.totalBytes` 求和。这是 Kopia 实际处理的字节数，准确。

CSI 模式的备份详情页会有一行**红色 caveat 提示**告诉用户这个数字的局限。

### 15.3 资源清单 tarball 大小

K8s YAML 归档（`backups/<name>/<name>.tar.gz`），存在 BSL bucket 里。后端从 S3 API 读 Content-Length。

- ✅ 支持 `provider: aws`（含 AWS S3 / MinIO / 腾讯 COS / 阿里 OSS 等所有 S3 兼容）
- ❌ `provider: gcp` / `provider: azure` 在 v0.9 添加
- 单次 ListObjectsV2 拿到一整个 BSL 的所有 tarball 大小，60 秒缓存 — 100 个还原点的列表页只发 1 次 S3 请求

错误情形：
- BSL 不可达 → "BSL Unavailable — bucket unreachable from Velero"
- 凭据 Secret 缺失 → "secret X has no key Y"
- 上述错误**显示在 tooltip**里，不打断列表渲染

### 15.4 暂未提供（v0.9 路线图）

| 指标 | 状态 | 原因 |
|---|---|---|
| **压缩比 / 去重比** | 待 v0.9 | 需要 Kopia 集成（`kopia repository status`），见 ADR-021 |
| **每次备份的物理增量** | 待 v0.9 | 同上 — Kopia 仓库级数据 |
| **GCS / Azure provider 大小** | 待 v0.9 | 需要 google-cloud-storage + azblob SDK |
| **PV 真实占用（非声明容量）** | 待 v0.10 | 走 Kanister Blueprint pre-backup hook 抓 `df -h` |

---

## 16. 跨集群跨云灾备（v0.8.7）

把本地集群的 `test-app` 备份到 Azure Blob，再在 AKS 上还原成 `app-replica`。这是 Kasten 那张架构图想做的事，SupKube v0.8.7 把"配出 DR pipeline"做到了 UI 层全可视化。

### 16.1 一图说明数据路径

四种数据路径决定**卷数据怎么走**，跨集群恢复**必须**走能携带数据的两条：

| 路径 | 卷数据在哪 | 跨集群 | node-agent | 何时选 |
|---|---|---|---|---|
| 📸 CSI 快照 | 源集群存储（CoW）| ❌ 不动 | 不需要 | 集群内回滚最快 |
| 🚚 **Data Mover** | 对象存储（Kopia）| ✅ | ✅ 需 | **DR 首选** — 拿 CSI 快照再上云 |
| 📁 文件系统 | 对象存储（Restic/Kopia）| ✅ | ✅ 需 | CSI 不可用时的退路 |
| 📋 仅元数据 | 没有 PV 数据 | ✅ | 不需要 | 把 K8s 配置搬走，数据另算 |

> 📖 **想深入对比 Data Mover 和 Filesystem 到底差在哪、怎么选？** 见 **§17 Data Mover vs Filesystem 深度对比** —— 含工作流图、一致性差异详解、决策树、按工作负载分类的推荐表。

### 16.2 准备工作：node-agent

Data Mover / 文件系统 路径要求 Velero 在每个节点都有 `node-agent` DaemonSet。默认 `velero install` 不带，需要显式开：

```bash
helm upgrade velero vmware-tanzu/velero \
  --namespace velero \
  --set deployNodeAgent=true \
  --set "initContainers[0].name=velero-plugin-for-microsoft-azure" \
  --set "initContainers[0].image=velero/velero-plugin-for-microsoft-azure:v1.10.0" \
  --set "initContainers[0].volumeMounts[0].name=plugins" \
  --set "initContainers[0].volumeMounts[0].mountPath=/target"
```

部署完检查：

```bash
kubectl -n velero get pods -l name=node-agent
# 每个 Node 都该有一个 Running
```

### 16.3 在 SupKube UI 创建 Azure BSL

**Storage Locations 页 → 添加** → Provider 选 `Azure Blob Storage`：

| 字段 | 例子 | 说明 |
|---|---|---|
| Container | `supkube-dr` | 提前在 Azure 门户建好 Blob 容器 |
| Storage Account | `mysupkubesa` | Storage Account 名 |
| Storage Account Key | `<门户复制>` | 门户 → Storage account → Access keys |
| Resource Group | `my-rg` | RG 名 |
| Subscription ID | 可选 | 用 VolumeSnapshot 才需要（Blob-only DR 不需要）|

提交后 SupKube 会：
- 在 velero ns 建一个 Secret `supkube-bsl-<name>-credentials`（含 Velero Azure 插件期望的 key=value）
- 建 BSL CR，credential 引用刚才的 Secret
- 触发 Velero BackupStorageLocationController 校验，几秒后 Phase 应该变 `Available`

### 16.4 创建带 Data Mover 的备份策略

**Policies 页 → 创建策略**：

- Action: **L2 Snapshot + Export**
- 频率: Daily
- **Data Path: 🚚 Data Mover** ← 关键
- Resources: `test-app`
- Storage Profile: 上一步建的 Azure BSL 名

保存后 SupKube 写出的 Velero Schedule 内含：

```yaml
spec:
  template:
    snapshotVolumes: true
    defaultVolumesToFsBackup: false
    snapshotMoveData: true       # ← Data Mover 开关
    storageLocation: <Azure BSL 名>
    includedNamespaces: [test-app]
```

第一次执行点 "Run Once" 立即跑。后台行为：

1. Velero 起 CSI VolumeSnapshot
2. Velero 起 `DataUpload` CR — 节点上的 node-agent 把 snapshot 内容用 Kopia 上传到 Azure Blob 容器
3. 完成后还原点详情页能看到 **🚚 Data Mover** chip + 实际去重后的字节数

### 16.5 把另一边 AKS 接上来

在 `officialwebsite` AKS 上**独立**装 Velero + SupKube：

```bash
# 1. Velero with Azure 插件（同样开 node-agent）
helm install velero vmware-tanzu/velero -n velero --create-namespace \
  --set deployNodeAgent=true \
  --set "initContainers[0].name=velero-plugin-for-microsoft-azure" \
  --set "initContainers[0].image=velero/velero-plugin-for-microsoft-azure:v1.10.0" \
  --set "initContainers[0].volumeMounts[0].name=plugins" \
  --set "initContainers[0].volumeMounts[0].mountPath=/target"

# 2. 创建与本地集群同形的 Azure BSL（指向同一 container）
#    可以用 SupKube UI 装好后从那边的 Storage Locations 页面创建，参数和上面一样

# 3. 装 SupKube 本身
helm install supkube ./supkube-helm/supkube -n supkube --create-namespace
```

**等 1 分钟**：Velero 的 `BackupSyncController` 默认 60s 扫一次 BSL，把对端集群的 Backup CR 同步过来。打开 AKS 的 SupKube UI → 还原点列表 → 出现 `test-app-backup-*` 行，Source 列标 **🌐 Imported**。

### 16.6 还原到 app-replica

点还原点的 ⋮ → 恢复 → Restore 抽屉打开：

1. 命名空间映射：`test-app → app-replica`
2. （可选）跑 **Pre-flight** 检测跨集群冲突（NodePort、StorageClass 名字、Ingress host …）
3. 有冲突就点 **Apply Suggested Fix** —— SupKube 自动建 Transform Set 给你
4. 执行

恢复后 `app-replica` 的 PV 数据**完整还原**（因为走的是 Data Mover，物理数据上了 Azure Blob 又拉回来），K8s 资源按 Transform Set 改好（不带源集群残留）。

### 16.7 局限（已在 Roadmap）

| 暂未支持 | 计划版本 | 替代 |
|---|---|---|
| 一个 UI 管两边集群（Hub-Spoke）| v0.10 | 装两个 SupKube，UI 切换 |
| Azure AAD service-principal 认证 | v0.9 | 用 Storage Account Key（已支持）|
| GCS / S3-Glacier provider | v0.9 | 暂只 AWS / Azure / S3 兼容 |
| 跨集群恢复后自动 follow-up 验证 | v0.10 Kanister | 手动 kubectl get pods 看 |

### 16.8 排障

| 症状 | 原因 | 解决 |
|---|---|---|
| BSL Phase = `Unavailable` | 凭据错 / 容器不存在 / IP 防火墙 | `kubectl -n velero describe bsl <name>` 看 message |
| DataUpload 卡在 `InProgress` 不动 | node-agent 没装 / 节点权限不够 | `kubectl -n velero get pods -l name=node-agent` |
| 备份 Phase=Completed 但还原后 PV 是空的 | 走了 CSI 而不是 Data Mover | 检查 Backup.spec.snapshotMoveData，应该是 true |
| Imported 备份没出现 | BSL 不一致 / 同步还没跑完 | `kubectl -n velero get backups`；强制 sync `kubectl -n velero patch bsl <name> -p '{"spec":{"validationFrequency":"1m"}}' --type=merge` |
| 还原失败：StorageClass 不存在 | 源/目标集群 SC 名不一样 | Pre-flight → Apply Fix 自动建 Transform Set 改 PVC spec |

---

## 17. Data Mover vs Filesystem — 深度对比（v0.8.7.5）

§15 列出了 SupKube 支持的 4 种数据路径。其中 **🚚 Data Mover** 和 **📁 Filesystem** 都属于"L2 快照 + 导出"（数据上对象存储、可跨集群恢复），但它们**怎么读数据**、**对存储的要求**、**对应用的影响**完全不同。这一章把决策依据讲透，避免选错路径在生产里翻车。

### 17.1 一图说清两者的工作流

```
Data Mover                            Filesystem Backup
═══════════════════════════════════   ═══════════════════════════════════

Source PV: postgres-data              Source PV: postgres-data
   │                                     │  (live, app 还在写)
   │  1. CSI CreateSnapshot              │
   ▼                                     │
┌─────────────────┐                      │
│ VolumeSnapshot  │ 存储层 CoW           │
│ (storage-level  │ 原子、瞬间完成        │  Kopia (in node-agent)
│  frozen view)   │                      │  打开 /host_pods/.../volumes/
└─────────────────┘                      │  postgres-data 这个 mount
   │                                     │
   │  2. CreatePVCFromSnapshot           │
   ▼                                     │
┌─────────────────┐                      │
│ Temp backup PVC │                      │
│ (snapshot 内容) │                      │
└─────────────────┘                      │
   │                                     │
   │  3. backup pod mount it             │
   ▼                                     │
┌─────────────────┐                      │
│ /vol path 里    │                      │  files = os.walk(...)
│ 是冻结时刻的     │                      │  for f in files:
│ 数据            │                       │      kopia.upload(f)
└─────────────────┘                      │
   │                                     │
   │  4. Kopia 读 + 上传                 │
   ▼                                     ▼
┌─────────────────────────────────────────────────────────┐
│                  Azure Blob / S3                         │
│  kopia/<repo>/  (内容寻址 + 去重 + 压缩，两边一样)         │
└─────────────────────────────────────────────────────────┘
```

**共同点**：两者都用 Kopia 写到 BSL，仓库格式完全相同；都需要 node-agent DaemonSet；最终都能跨集群恢复；都受益于 Kopia 的去重压缩。

**根本差异**：数据从哪里读。

### 17.2 关键区别 7 维表

| 维度 | 🚚 Data Mover | 📁 Filesystem |
|---|---|---|
| **数据源** | CSI 快照（存储层冻结的视图）| 活动 PV 文件系统（app 还在写）|
| **一致性** | 存储层原子，**crash-consistent** | 文件级、**逐文件读**，无原子保证 |
| **数据库友好度** | ✅ Mid-transaction 也能恢复（类似断电恢复）| ⚠️ 需要 PreHook 让 DB FLUSH / quiesce，否则恢复后可能损坏 |
| **存储要求** | 必须支持 **CSI VolumeSnapshot**（StorageClass 要有对应 VolumeSnapshotClass）| 任何 K8s 能挂的卷 — NFS / hostPath / 老的 in-tree provisioner 都行 |
| **对运行 app 的影响** | 几乎零（CSI 快照是 CoW，存储层操作）| 占 I/O 带宽（读活动文件系统，可能拖慢 app）|
| **临时资源开销** | 中等（建临时 PVC + backup pod，备份结束后回收）| 低（只起一个 backup pod，没临时 PVC）|
| **大量小文件性能** | 块级，速度稳定 | 慢（要 walk 目录树，每个 file 一次 stat + open）|

### 17.3 一致性 — 最重要的概念

#### Data Mover 的 "crash-consistent"

CSI 快照是存储层在某一**瞬间**冻结所有 block 的状态。等同于"在那一瞬间，机器突然断电"。

PostgreSQL / MySQL / MongoDB / Redis（AOF 模式）都有**崩溃恢复机制**（WAL / AOF / journal），所以从 crash-consistent 快照恢复回来：

- 启动时它们走 WAL replay，把 mid-transaction 的状态卷回去
- 结果是一个**已知一致**的早一些时刻的状态
- 不需要应用层配合（不需要 `pg_dump`，不需要锁表，不需要停服务）

**结论**：对数据库类工作负载，**Data Mover 是默认且推荐选项**。

#### Filesystem 的"逐文件不一致"

Kopia 走 OS 文件系统 walk，**类似你在跑 app 时执行**：

```bash
tar -czf backup.tar.gz /data/
```

发生的事情：

```
读 file1.txt   ← 时刻 T1
读 file2.txt   ← 时刻 T2
[app 在这之间改了 file1 也改了 file2 的关联状态]
读 file3.txt   ← 时刻 T3，但和 file2 已经不对齐
```

恢复后：file3 是新版引用了 file2 的旧版数据 → **数据库认为索引指向某行，但行已经不在那个 page** → **数据库直接崩或返回错位数据**。

**解决办法**：在 Velero Backup spec 配置 `hooks`，让 SupKube 在备份前把应用 quiesce：

```yaml
spec:
  hooks:
    resources:
      - name: postgres-flush
        includedNamespaces: [test-app]
        labelSelector:
          matchLabels:
            app: postgres
        pre:
          - exec:
              command:
                - /bin/bash
                - -c
                - psql -U postgres -c 'CHECKPOINT; SELECT pg_start_backup('"'"'velero'"'"', true);'
              onError: Fail
              timeout: 5m
        post:
          - exec:
              command:
                - /bin/bash
                - -c
                - psql -U postgres -c 'SELECT pg_stop_backup();'
              timeout: 5m
```

这是数据库专属脚本，每种 DB 不同。**Kasten 用 Blueprint，Velero 这边目前要手写**。SupKube v0.10 计划集成 Kanister 后会有内置模板。

⚠️ **配置 hook 是用户的责任**：配错了会**沉默地坏数据**，恢复时才发现 → 数据可能已经被新版备份覆盖。

### 17.4 何时选哪个 — 决策树

```
┌─────────────────────────────────────────┐
│ 我的 PV 在什么 StorageClass 上？         │
└─────────────────────────────────────────┘
              │
     支持 CSI VolumeSnapshot？
              │
      ┌───────┴────────┐
     YES               NO
      │                 │
      ▼                 ▼
┌──────────────┐    ┌──────────────┐
│ 应用类型？    │    │ 📁 Filesystem │
└──────────────┘    │ (唯一选择)    │
      │              └──────────────┘
   ┌──┴────────┐
   │           │
有状态(DB/MQ)  无状态(静态文件)
   │           │
   ▼           ▼
🚚 Data       任选 — Filesystem
Mover 强推   稍微便宜点
```

#### 怎么判断 StorageClass 支持 CSI Snapshot

```bash
# 列出集群里所有的 VolumeSnapshotClass + 它们对应的 driver
kubectl get volumesnapshotclass

# 列出 PVC + 对应的 StorageClass
kubectl get pvc -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{.spec.storageClassName}{"\n"}{end}'

# 拿到 StorageClass 的 provisioner 名字
kubectl get sc <storage-class-name> -o jsonpath='{.provisioner}'

# 检查 driver 是不是支持 SNAPSHOT 能力
kubectl get csidriver <driver-name> -o yaml
```

如果 driver 不在 VolumeSnapshotClass 列表里，**Data Mover 不能用**，必须走 Filesystem。

SupKube 在 Policy 创建表单**自动做了这个检测**：你选 Data Mover + 命名空间后，"CSI Compatibility" 区域会列出每个 PV 是否兼容；不兼容的 PVC 会标出来，并阻止保存。

### 17.5 实战推荐表

| 工作负载 | 推荐路径 | 理由 |
|---|---|---|
| PostgreSQL / MySQL / MongoDB on PVC | 🚚 **Data Mover** | crash-consistent 即可恢复 |
| Redis (with RDB / AOF) | 🚚 **Data Mover** | 同上 |
| Kafka log directory | 🚚 **Data Mover** | 大量小文件，FS walk 慢；CSI 快照秒级 |
| Etcd 数据目录 | 🚚 **Data Mover** | 同上，且需要原子 |
| MinIO / S3 数据 (静态对象) | 任选 | 都 OK |
| Wordpress / Nextcloud 用户上传 | 任选 | 静态多，FS 也行 |
| Jenkins workspace / ML training output | 📁 **Filesystem** | 通常在 NFS，CSI snapshot 不支持 |
| GitLab repos / config | 📁 **Filesystem** + PreHook (`git gc` / 停服务) | 同上 |
| 内部测试集群 hostPath PV | 📁 **Filesystem** | hostPath 没快照可能 |
| Air-gap 客户提供的 NetApp NFS | 📁 **Filesystem** | 看 driver 是否支持 CSI snapshot，多数不支持 |
| Ceph RBD with rook-ceph CSI | 🚚 **Data Mover** | Ceph 原生支持，CSI driver 已集成 |
| Portworx / Longhorn | 🚚 **Data Mover** | 这两个 CSI snapshot 支持都成熟 |

### 17.6 性能数据参考

实测 docker-desktop CSI hostpath driver，单节点：

| 场景 | Data Mover | Filesystem |
|---|---|---|
| 5 GiB PostgreSQL data | 10 秒（CSI snapshot 瞬间 + 47 MB Kopia 上传）| 25-40 秒（walk 数据库目录 + 上传）|
| 1 GiB 空 PVC | 8 秒（snapshot + 30 bytes 上传）| 12 秒（空 walk + 上传）|
| 大量小文件（10 万个 1KB 文件） | 15 秒（块级一次性传输）| 5-15 分钟（每文件 stat + read）|

⚠️ **生产环境差异更大**：云盘 CSI 通常比 hostpath driver 快得多；NFS 在大量小文件场景非常慢。具体走你自己集群的环境测一次。

### 17.7 SupKube UI 里的体现

在 Policy 创建 / 编辑表单：

- L2 选中后右列"快照 + 导出"激活，包含 Data Mover 和 Filesystem 两个选项
- 选 Data Mover：底部出 CSI Compatibility 检查表，列每个 ns 的 PVC 是否兼容
- 选 Filesystem：无额外检查（任何卷都可以）
- 两者都弹"需要 node-agent DaemonSet"的提示 — 详见 §16.2

在 Restore Points 列表里 ↓

| chip | 含义 |
|---|---|
| 🚚 Data Mover | 该备份走 Data Mover，可跨集群恢复 |
| 📁 Filesystem | 该备份走 Filesystem，可跨集群恢复 |
| 📸 CSI Snapshot | L1 模式，集群本地，**不能**跨集群恢复 |
| 📋 Metadata Only | 只备 YAML，无 PV 数据 |

详情见 §15.

### 17.8 备份完成后看到了什么

无论走哪条路径，对象存储里**都会出现这两个目录**：

```
<bucket>/<prefix>/
  ├── backups/<name>/
  │     ├── <name>.tar.gz         ← K8s YAML 资源清单 (KB 级)
  │     ├── velero-backup.json    ← backup 元数据
  │     ├── *-logs.gz             ← Velero 备份过程日志
  │     ├── *-datauploads.json.gz   (Data Mover only)
  │     └── *-podvolumebackups.json.gz (Filesystem only)
  │
  └── kopia/<repo>/
        ├── kopia.repository      ← Kopia 仓库元数据
        ├── p*                    ← 数据块（去重压缩后的 PV 内容）
        ├── q*                    ← 索引块
        └── ...
```

**区别**：

- Data Mover 路径在 `backups/.../*-datauploads.json.gz` 里记录每个 DataUpload 的 Kopia snapshot ID + 字节数
- Filesystem 路径在 `backups/.../*-podvolumebackups.json.gz` 里记录每个 PodVolumeBackup 的 Kopia snapshot ID + 字节数
- `kopia/` 目录下的 chunks **两者格式完全相同**，能互相恢复（即从 Data Mover 备份导出的 Kopia repo，能在某种工具链下被 Filesystem 路径读取，反之亦然 —— 但 Velero 本身不允许这种混用）

### 17.9 常见问题

| 问题 | 答案 |
|---|---|
| 我装了 node-agent，但 Data Mover 卡在 "Accepted" 不动 | node-agent 的 `NODE_NAME` 环境变量没设。手动 `kubectl apply` DaemonSet 时容易漏。详见 INTEGRATION.md 的 node-agent manifest |
| Data Mover 备份成功了，Restore 时报 "no source PVC found" | 目标集群没有同名的 StorageClass。建议先 kubectl 创建 / 用 Transform Set 改 PVC.spec.storageClassName |
| Filesystem 备份后恢复 PostgreSQL 起不来 | 没配 PreHook flush WAL；数据 mid-transaction 状态被备份了，恢复后需要手动跑 `pg_resetwal` 或从更早的备份恢复 |
| Data Mover 第二次备份是不是只传 delta？ | 是。Kopia 内容寻址 + 去重，相同 chunk 不重复上传。一个 50 GiB 的 PostgreSQL 改了 100 MB 数据，第二次备份网络上只传 ~100 MB |
| 同一个 BSL 能同时给 Data Mover 和 Filesystem 用吗？ | 能。两者在 Kopia repo 里是不同的 snapshot 路径（`/<pvc-uid>/...`），互不影响 |

---

## 18. 备份链与去重模型（v0.8.7+）

很多备份产品（Veeam / NetBackup / SimpliVity 等）的核心 UI 概念是**备份链**：

```
Full Backup → Incr → Incr → Incr ...
```

恢复某个 Incr 必须按链 replay；删除中间任一个会破坏后续 RP；选择 Full+Incr 策略需要用户深刻理解。

**SupKube / Velero / Kopia 走的是完全不同的模型**：

| 维度 | 传统 incremental chain | SupKube / Kopia |
|---|---|---|
| RP 之间的依赖 | 链式 — 必须 replay | **无依赖 — 每个 RP 独立恢复** |
| 物理存储 | 链式增量 | **内容寻址 + 去重** |
| 删任一 RP | ❌ 影响整条链 | ✅ 随便删，不影响其它 RP |
| 恢复速度 | 离 Full 越远越慢 | 任意 RP 同速（Kopia snapshot ID 直接拉） |
| 用户心智 | "我得知道 full 在哪、链多长" | "RP-1, RP-2, RP-3，挑哪个都一样" |
| 中途坏块 | ❌ 链断废 | ✅ Kopia 内容寻址，单 chunk 影响有限 |

### 18.1 物理上为什么存储等价

Kopia 仓库是**内容寻址**：每个数据块按内容算 hash 后存。第二次备份同样的内容时，Kopia 看到 hash 已存在，不会再传一遍。从用户视角每个 RP 都是"完整可恢复的快照"；从存储视角只有第一次需要全量上传，之后只增上传 delta。

举例：100 GB PostgreSQL 数据库每天备份一次：

```
RP-1  (Sunday)   逻辑 100 GB    Azure Blob 新增上传:  100 GB
RP-2  (Mon)      逻辑 100 GB    新增上传:               5 GB
RP-3  (Tue)      逻辑 100 GB    新增上传:               3 GB
RP-4  (Wed)      逻辑 100 GB    新增上传:               4 GB

Azure Blob 总占用:                                ≈ 112 GB

恢复 RP-4 → Kopia 直接拉 RP-4 关联的所有 chunk（其中绝大部分早就在仓库里）
删 RP-1   → RP-2/3/4 不受任何影响；RP-1 独有的 chunks 才被 Kopia GC 清掉
```

### 18.2 与"全备 / 增量"概念的对应关系

如果合规审计问你"你们走全备还是增量"，标准答案是：

> 我们走的是 **forever-incremental + content-addressable deduplication**。
> 逻辑视图上每个 RP 是完整快照（用户视角的"全备"），
> 物理存储是增量（只上传 delta chunks），
> 但**无链依赖**（任意 RP 单独可恢复，无需 base + chain）。
> 这比传统全备/增量更现代 —— Veeam 2020+ 也演化到了类似的 SOBR + ReFS Block Cloning 模型。

### 18.3 retention 与 GC 的关系

每个 RP 有自己的 TTL（在 Policy 表单里设的 Snapshot Retention / Export Retention）。RP 过期后：

1. Velero `DeleteBackupRequest` 删除 Backup CR + BSL 里的 tarball
2. Velero 标记 Kopia snapshot 为 "deleted"（写 tombstone，不立刻删数据）
3. **Kopia 仓库维护任务**（默认每 24h 一次）GC 不再被任何 snapshot 引用的 chunks

意味着 Azure Blob 上的实际占用回收**异步发生**：删 RP 后看到 kopia/ 目录里还有 chunks 是**正常的**，不是 bug。Kopia 维护一跑就清。

### 18.4 v0.9 计划：去重比可视化

每个 RP 详情页加一列"unique vs shared bytes"，例如：

```
RP-3:  Total: 100 GB  ·  Unique to this RP: 3 GB  ·  Shared: 97 GB
```

让用户直观看到去重价值。这是 Kopia 仓库自带的统计，只是 UI 还没暴露 —— v0.9 会做。

---

## 19. 集群健康：孤儿资源的 GC 与设置（v0.8.8）

### 19.1 为什么需要

Velero v1.18 的 Data Mover 路径在备份过程中会创建中间 `VolumeSnapshotContent` 对象，设成 `deletionPolicy: Retain` 让它们熬过 Kopia 上传期。**Velero 本身没有在父 Backup 被删除时回收这些 VSC 的逻辑**（[upstream issue #7838](https://github.com/vmware-tanzu/velero/issues/7838)，1.16+ 一直 open）。结果：

- **K8s API 累积 orphan**：删 30 个 RP 可能产生 30+ 个孤儿 VSC，污染 `kubectl get vsc`
- **对象存储费用**：BSL 里 kopia/ 目录的 chunk 持续占用，Kopia maintenance 也不知道哪些可以 GC
- **用户视觉污染**：你看到 RP 删了，但底层还残留 — 心理上很不爽

SupKube **必须**在自己这一层补这个清理逻辑。

### 19.2 三种触发路径

| 触发 | 何时 | 谁能用 |
|---|---|---|
| **定时扫描** | 默认每 6 小时（在 Settings 里可调 1h/6h/12h/24h，或彻底关掉）| 后台自动 |
| **删除 RP 后** | 每次删 Backup 后 ~60 秒（debounce — 连续删多个合并成一次扫）| 自动 |
| **手动按钮** | Settings → Cluster Hygiene → Run Now | Admin |

### 19.3 Settings UI

`Settings → 集群健康` 标签页（admin 才可见）：

```
┌────────────────────────────────────────────────────────────┐
│ 孤儿资源清理 (Orphan GC)                       [✓ 已启用] │
├────────────────────────────────────────────────────────────┤
│ [说明段落：解释为什么需要]                                  │
│                                                            │
│ 自动清理：    [✓ 开关]  后台定时扫描已开启 — 自动删除孤儿   │
│ 扫描周期：    [每 6 小时（推荐）▾]                         │
│ 手动触发：    [立即清理]  不管自动开/关都能用              │
│                                                            │
│ ─── 最近一次扫描 ─────────────────────────────────────     │
│   运行于      5m ago                                       │
│   VSC: 3      VS: 2      PodVolumeBackup: 0   DataUpload: 1│
│   summary:    orphan-gc: deleted 3 VSC + 2 VS + ...        │
└────────────────────────────────────────────────────────────┘
```

### 19.4 客户的选择权

我们**默认开** GC（开发者认为这是正确默认），但完全尊重客户的选择：

- **关掉自动扫**：在某些合规场景客户要求"任何资源删除都必须经过审批"，自动扫违反流程。这种场景下用户关掉自动 → 只有 admin 手动点"立即清理"才扫一次
- **改成每小时扫**：高变更率集群（频繁创建/删除备份），缩短间隔减少累积
- **改成每 24 小时扫**：低变更率集群（生产稳定环境），减少 API 调用

### 19.5 手动 GC 在 Activity 页的体现

每次手动 GC 触发会写一条 K8s Event（带 label `supkube.io/activity=true`），所以在 **Activity 页**会出现一条 "OrphanCleanup" 类型的活动，记录：

- 触发方式（manual / periodic / post-delete）
- 删了多少 VSC / VS / PVB / DataUpload
- 总耗时
- 操作者 user (manual 时)

这样客户回看时能确切知道"什么时候删了什么"。

### 19.6 自己确认 GC 在工作

```bash
# 看 GC 设置 ConfigMap
kubectl -n supkube get cm supkube-settings -o yaml

# 看最近的 GC 活动事件
kubectl -n supkube get events --field-selector reason=OrphanCleanup --sort-by=lastTimestamp

# 手动触发一次（admin token）
curl -X POST http://localhost:30888/api/v1/admin/cleanup/orphans \
  -H "Authorization: Bearer demo-admin-token"
```

### 19.7 GC 的安全保证

担心 GC 误删活资源？算法是 fail-safe 的：

1. **必须有 `velero.io/backup-name` label** — 没这个 label 的资源（用户自建 / 其它系统创建）**永远不动**
2. **必须父 Backup 真的不存在** — 扫描开始时建一个"现存 Backup 名集合"，只删父不在集合里的孤儿
3. **临时性 race**: 如果有人正在创建 Backup 同时 GC 在跑，要么 GC 看到 Backup（不删），要么 GC 看不到孤儿（VSC 还没出来）— 两种情况都安全

零误删风险。

---

## 20. 双策略：Snapshot + Export 模型（v0.8.9 引入 / v0.8.10 改进）

> ⚠️ **v0.8.12 起本章节描述的模型整体被取代** —— 参见架构设计 ADR-029。
>
> **变化概览**：
> - "Snapshot Half"（依赖 CSI VolumeSnapshot/VSC）→ 改为 **"Local Half"**（写到集群内 MinIO BSL，是真备份）
> - "Export Half" → 改名为 **"Cloud Half"**（不变，仍是 Cloud BSL）
> - 术语映射：Snapshot Only → **Local Backup**；Snapshot + Export → **Local + Cloud Backup**
>
> **重要历史真相**：v0.8.11.2 及之前，本章描述的 Snapshot Half 在 Velero v1.18 上**实际不可恢复**——VSC 被 Velero 自动删除，Snapshot RP 仅剩元数据。"快速本地恢复"承诺事实上未兑现，只是无人去试 Snapshot RP 所以未爆雷。Export Half 一直完整可恢复。
>
> **如果你在 v0.8.12+**：直接读 §20-new（Local + Cloud Backup 模型）；本章保留只为兼容老 RP 的解释。
> **如果你在 v0.8.11.2 及之前**：恢复时**只从 Export RP 恢复**，不要点 Snapshot RP（它没数据）。

### 20.1 一次策略运行 = 两个 Restore Point

如果你把 Policy 模式设为 **L2 (Snapshot + Export)**，每次该策略触发，**SupKube 会产生两个 Restore Point**：

| RP | Type 列 | 存储位置 | 用途 |
|---|---|---|---|
| **📸 Snapshot RP** | Snapshot（蓝） | 本集群 CSI 快照 | "上线回滚点"：秒级恢复，但不能跨集群 |
| **🚚 Exported RP** | Exported（紫） | 你选的 Storage Profile（Azure / S3 …） | DR / 跨集群恢复：分钟级，但走对象存储 |

它们各自有独立 retention（默认 Snapshot 24h、Export 720h = 30 天），分别独立淘汰。

> **为什么不是一个 RP**？因为它们字面上是两份数据：本地的 CSI 快照 vs BSL 的 Kopia 数据。客户从 Snapshot RP 恢复 = 用 CSI 直接 clone；从 Exported RP 恢复 = 从 Kopia 下载。两条路完全独立。

### 20.2 ⚠️ 重要：v0.8.10 之前两半之间有时间差

> 这一节客户必读。

Velero v1.18 处理 `snapshotMoveData=true` 的 Backup 时：

1. 拍一份 CSI 快照
2. node-agent 把快照内容上传到 BSL（走 Kopia）
3. **上传完成后 Velero 主动删除 CSI 快照**

所以"Exported RP" 内部的数据，是 Velero 单独又拍了一次新快照，**不**复用 "Snapshot RP" 那次拍的快照。

**v0.8.9 现象**：两半各走自己的 cron 节拍，导致 Snapshot RP 和 Exported RP 之间真实时间差可能达**几分钟**。客户从两边恢复出来的数据**不一致**（中间几分钟客户应用还在写入，两份快照在不同时间点截取）。

**v0.8.10 修复方案（Plan B）**：

- Export 半 Schedule 永久暂停（kubectl 看到 `paused=true` 不要恢复它）
- SupKube 自己监听 Snapshot 半 Backup 进入 `Completed` → 立即触发 Export 半 Backup
- 时间差从**几分钟**压到约 **30 秒**（snapshot 完成→ controller 10s 内扫到 → Velero queue 5s 内启动 → fresh CSI snapshot at +15s）

UI 体现：

- Activity 页：两半各占一条 Action card，详情抽屉顶部有"📸 Snapshot half"或"🚚 Export half" chip
- 详情抽屉显示"**Policy Run At**" 字段，两半**共享**同一个时间戳（= Snapshot 半的 creation time）
- 详情抽屉顶部有紫色"**Paired with**"banner，点击直接跳到另一半

### 20.3 仍未解决的 ~30 秒差距 — 路线图

30 秒比几分钟好，但**不是 0**。要做到 0 差距（"同一份快照同时产 Snapshot RP + Exported RP"），需要 Velero 上游加 `preserveSnapshotsAfterUpload` 字段（社区 issue #7338，截至本文撰写时尚未合并到 v1.18）。

我们已在 v0.8.10 **预留接口**：

- 每条 Export 半 Backup 都带 annotation `velero.io/csi-volumesnapshot-content-retain-policy=retain`
- Velero 上游放出修复后，SupKube 升级到"单 Backup 同时产两个 RP"的 Plan C 时，**无需修改老 RP**就能切换

### 20.4 如何确认你拿到的是"对的"那个 RP

**恢复时优先选 Snapshot RP**（如果存在）：

- 速度快（秒级 vs 分钟级）
- 不消耗 BSL 出口流量
- 但**仅在原集群**可用

**只有这些情况才选 Exported RP**：

- Snapshot RP 已经过 TTL（默认 24h）
- 跨集群恢复（如 AKS A → AKS B）
- 本集群 CSI 快照异常 / CSI driver 升级失败

### 20.5 排查：策略触发了但只看到 Snapshot half，没有 Export half

可能原因：

| 现象 | 诊断 | 行动 |
|---|---|---|
| 等了 1 分钟仍没 Export | snapshot 半 Phase 是 `PartiallyFailed` / `Failed` | SupKube 故意不触发 Export 避免上传脏数据。修复 snapshot 失败原因后重跑 |
| 等了 1 分钟仍没 Export，snapshot 是 Completed | controller 没起来 / 被禁用 | `kubectl logs deploy/supkube-backend -n supkube \| grep policypair`；ConfigMap `supkube-settings` 里 `policyPair.enabled` 应为 `true` |
| 老 policy（v0.8.9 以前创建的）始终没 Export 配对 | Schedule 没 supkube.io/policy-role label → controller 不认识 | 删除老 policy 重建（v0.8.11 会加 "Migrate to v0.8.10" 按钮） |

---

## 21. kubectl 速查 + label / annotation 契约（v0.8.10.2+）

> 给 SRE、合规审计员、CI/自动化团队的章节。
>
> **核心承诺**：SupKube **不发明任何 CRD**。Policies、Restore Points、Restores 全部是 **Velero 原生 CR**，SupKube 只通过 K8s **label + annotation** 加自己的语义。这意味着：
>
> - 你可以**完全用 kubectl 审计 / 自动化 / 集成 GitOps**，无需 SupKube API
> - 卸载 SupKube 后**所有数据仍在原地**，Velero CLI 立刻接管
> - 任何兼容 Velero 的工具都能解读 SupKube 创建的 backup

### 21.1 列出所有 Policy

```bash
# v0.8.9+ 的 dual policy 模型：每个 policy 在 K8s 里是 1-2 个 Schedule
kubectl -n velero get schedules \
  -L supkube.io/policy-name,supkube.io/policy-role

# 只看 snapshot 半（每个 policy 一行，便于一对一统计）
kubectl -n velero get schedules -l supkube.io/policy-role=snapshot

# 看某个具体 policy 的两半
kubectl -n velero get schedules -l supkube.io/policy-name=<policy-name>
```

老的 v0.8.8 policy（无 SupKube label）也会被列出，只是 POLICY-NAME / POLICY-ROLE 列空。

### 21.2 列出 Restore Points

```bash
# 全部 RP
kubectl -n velero get backups

# 某个 policy 产生的所有 RP（snapshot 半 + export 半都返回）
kubectl -n velero get backups -l supkube.io/policy-name=<policy-name>

# 只看 Application 一键拍的 Instant Snapshot
kubectl -n velero get backups -l supkube.io/manual-snapshot=true

# 某 namespace 的全部手动快照
kubectl -n velero get backups -l supkube.io/source-namespace=<ns>

# 看一个 RP 的全部 SupKube 元数据
kubectl -n velero get backup <name> -o jsonpath='\
NAME:                   {.metadata.name}{"\n"}\
policy-name:            {.metadata.labels.supkube\.io/policy-name}{"\n"}\
policy-role:            {.metadata.labels.supkube\.io/policy-role}{"\n"}\
manual-snapshot:        {.metadata.labels.supkube\.io/manual-snapshot}{"\n"}\
paired-with:            {.metadata.annotations.supkube\.io/dual-rp-paired}{"\n"}\
policy-run-instant:     {.metadata.annotations.supkube\.io/policy-run-instant}{"\n"}\
triggered-by:           {.metadata.annotations.supkube\.io/triggered-by}{"\n"}\
created-by-user:        {.metadata.annotations.supkube\.io/created-by-user}{"\n"}\
comment:                {.metadata.annotations.supkube\.io/comment}{"\n"}'
```

### 21.3 找配对的另一半（Plan-B dual policy）

```bash
# 给定 snapshot 半的名字 → 找 export 半
EXPORT=$(kubectl -n velero get backup <snap-name> \
  -o jsonpath='{.metadata.annotations.supkube\.io/dual-rp-paired}')
echo "Export half: $EXPORT"
```

### 21.4 合规审计：谁在什么时候拍了什么

```bash
# 列出过去 24h 所有 Instant Snapshot + 操作者
kubectl -n velero get backup -l supkube.io/manual-snapshot=true -o json \
  | jq -r '.items[] | select(.metadata.creationTimestamp >
            (now - 86400 | strftime("%Y-%m-%dT%H:%M:%SZ"))) |
            [.metadata.creationTimestamp,
             .metadata.labels["supkube.io/source-namespace"],
             .metadata.annotations["supkube.io/created-by-user"],
             .metadata.annotations["supkube.io/comment"]] | @tsv'
```

### 21.5 完整 label / annotation 契约表

| 键 | 位置 | 含义 |
|---|---|---|
| **Labels（用于 `-l` 过滤）** | | |
| `supkube.io/policy-name` | Schedule + Backup | 客户设置的 policy 名 |
| `supkube.io/policy-role` | Schedule + Backup | `snapshot` / `export` |
| `supkube.io/manual-snapshot` | Backup | `true` = Application 一键按钮产生 |
| `supkube.io/source-namespace` | Backup | 被保护的 ns（手动快照专用） |
| `velero.io/schedule-name` | Backup（Velero 原生） | 由哪个 Schedule 触发 |
| **Annotations（值用 `-o jsonpath` 读）** | | |
| `supkube.io/dual-rp-paired` | Backup（两半互指） | 配对的 backup 名 |
| `supkube.io/policy-run-instant` | Backup（两半同值） | 共享逻辑时间戳 RFC3339 |
| `supkube.io/triggered-by` | Backup | `app-manual-snapshot` / `dual-pair-controller` / `policy-run-once` |
| `supkube.io/created-by-user` | Backup | 操作者用户名 |
| `supkube.io/comment` | Backup | 用户输入的备注 |
| `velero.io/csi-volumesnapshot-content-retain-policy` | Backup（export 半） | `retain`（Plan C 升级预留） |

### 21.6 哪些 UI 字段在 kubectl 里**没有直接对应**

这些字段是 SupKube 后端在请求时**计算**出来的，不存在于 K8s 对象里。要 CLI 等价，参考"替代方式"列：

| UI 字段 | 怎么计算的 | kubectl 替代方式 |
|---|---|---|
| Policies 表 `Restore Points` 列 | 后端扫 backups → groupBy schedule-name | `kubectl -n velero get backups -l supkube.io/policy-name=<name> --no-headers \| wc -l` |
| RP 表 `Data Path` chip | 从 `spec.snapshotMoveData` + `defaultVolumesToFsBackup` 派生 | `kubectl get backup <name> -o yaml \| grep -E 'snapshotMoveData\|defaultVolumesToFsBackup'` |
| 抽屉 `Application Items` 计数 | live-cluster query + 分类 | `velero backup describe <name> --details` |
| 抽屉 `Velero Total` | `.status.progress.totalItems` | `kubectl get backup <name> -o jsonpath='{.status.progress.totalItems}'` |
| Activity Action 流 | Backup + Restore 合并 + status 派生 | 各自 list 即可（`kubectl get backups,restores`） |

### 21.7 常见自动化 patterns

**a. CI 触发 backup 并等结果**

```bash
# 通过 velero CLI（推荐）—— 等价于 SupKube 的 POST /backups
velero backup create release-v1.2.3 \
  --include-namespaces my-app \
  --snapshot-volumes \
  --wait

# 通过 SupKube API（带 SupKube 语义 + audit）
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"comment":"CI release v1.2.3"}' \
  https://supkube.example.com/api/v1/applications/my-app/snapshot
```

**b. GitOps 用 ArgoCD / Flux 管理 policies**

把 Schedule YAML 放在 git 里，加 SupKube label：

```yaml
apiVersion: velero.io/v1
kind: Schedule
metadata:
  name: my-app-daily
  namespace: velero
  labels:
    supkube.io/policy-name: my-app-daily
    supkube.io/policy-role: snapshot
spec:
  schedule: "0 2 * * *"
  template:
    includedNamespaces: [my-app]
    snapshotVolumes: true
    ttl: 168h
    storageLocation: default
```

SupKube UI 立刻识别这个 GitOps-managed policy 并展示。

**c. 用 kubectl wait 等 backup 完成**

```bash
kubectl -n velero wait backup/my-app-20260523120000 \
  --for=jsonpath='{.status.phase}'=Completed --timeout=10m
```

### 21.8 RBAC 注意

客户用 kubectl 直接操作 backup 时，**绕过了 SupKube 的 RBAC 表 + 审计日志**。如果合规要求所有备份操作都进审计，建议：

- 限制 K8s ServiceAccount 只有 SupKube 的 SA 能 create/delete `backups.velero.io`
- 客户走 SupKube API（任何方式 — UI / curl / Terraform supkube-provider）
- velero CLI / kubectl 仅作为只读审计入口

**永远不要 `kubectl delete backup <name>`** —— 会被 BackupSync controller 重建 + 不级联删除 BSL 上的数据。删除走 SupKube UI 或 `velero backup delete <name>`（这两个都走 DeleteBackupRequest CRD）。

---

## 22. 灾备演练 / DR Playbook（v0.9.0 Multi-Cluster Manager）

> 给运维 / SRE / 灾备演练负责人。
>
> 本节是把"备份"变成"实际恢复出能用的应用"的可重复操作手册。前提：SupKube v0.9.0+ 部署完成，至少一个 **Cloud BSL** 配好（Azure Blob / S3 / S3 Compatible），且至少一个 namespace 已经在跑备份策略。

### 22.1 三个 DR 场景，怎么选

| 场景 | 用哪条路径 | 复杂度 |
|---|---|---|
| **A** 测试演练：现役集群上把 ns 还原到一个新 ns（同集群） | SupKube UI → Restore Points → ⋮ → Restore；Target Namespace 改名 | ★ 最简单 |
| **B** 计划迁移：备份在 Cluster A，恢复到 Cluster B（A 还活着） | §22.2 跨集群恢复 | ★★ 标准 |
| **C** 真实灾难：Cluster A 全挂，从 Cloud BSL 在全新 Cluster B 重建 | §22.3 灾后重建 | ★★★ 演练过几次比真出事强 |

**前 2 个场景客户应每月演练一次**；第 3 个建议每季度演练一次（哪怕只是恢复一个测试 ns，验证 BSL 凭据有效、Velero 版本兼容、网络可达）。

### 22.2 场景 B — 跨集群恢复（A 健康 → 恢复到 B）

**前提**：
- Cluster A 跑着 SupKube + Velero + 至少一份成功的 Cloud BSL 备份
- Cluster B 跑着 Velero v1.18+（同版本最稳；可以用 Helm 命令 `helm install velero ...` 或 `--set velero.enabled=true` 在 SupKube Helm 安装时一并装）
- Cluster B 的 Velero pod 能访问到 Cluster A 的 Cloud BSL bucket（网络可达 + 凭据可用——SupKube 会自动复制凭据 Secret 过去，但 bucket 必须从 B 的网络拨得通）

**步骤**：

```
1. 在 Cluster A 的 SupKube UI（admin 身份）
   Settings → 集群管理 → + 添加集群
   ─ Name:           aks-cluster-b
   ─ Display Name:   AKS Cluster B
   ─ Type:           Secondary
   ─ Kubeconfig:     上传 cluster B 的 kubeconfig 文件
   ─ Context:        留空（用默认 current-context）
   点 Test Connection → 看到 ✓ Healthy + k8s 版本 + Velero 版本
   点 Add Cluster

2. 等 ~60 秒（cluster health controller 自动 poll 一次）
   Settings → 集群管理 看到 cluster B 行显示 ● Healthy

3. 进入 Backups → Restore Points 页（仍在 cluster A 视角）
   找到要恢复的 RP；点 ⋮ → Restore

4. RestoreDrawer 顶部出现"Target Cluster"段
   ─ 默认是 "this-cluster"（cluster A，原地恢复）
   ─ 改为 aks-cluster-b
   ─ 看到 ⚠ Cross-cluster restore 黄色 banner
   ─ Target Namespace 可以改名（建议改成 "<ns>-restored" 避免与现有 ns 撞）
   点 Start Restore

5. 后端处理（~60-90 秒，UI 显示 spinner）：
   ├─ 读 Cluster A 的源 Backup CR + Cloud BSL CR + credential Secret
   ├─ 把 BSL + Secret apply 到 Cluster B 的 velero namespace（已存在则跳过）
   ├─ 等 Cluster B 的 Velero BackupSyncController 把 Backup metadata 同步出来
   ├─ apply Restore CR 到 Cluster B 的 velero namespace
   └─ 返回 201，前端 toast "Restore submitted"

6. 切到 Cluster B 视角看进度
   Sidebar 顶部点 Mode Switcher → 选 aks-cluster-b
   Activity 页：能看到正在跑的 Restore action
   等 status = Completed

7. 验证目标 namespace
   在 cluster B 上 `kubectl get pods -n <target-ns>` 应看到应用 pod 起来了
```

**故障排查**：

| UI 报错 | 含义 | 修法 |
|---|---|---|
| `424 Failed Dependency: source backup lives in the in-cluster Local BSL` | 你选的 RP 在集群内 MinIO（本地 BSL），别的集群够不到 | 选一个 Cloud BSL 上的 RP（Restore Points 表里 chip = "Cloud" 那些） |
| `424 ... backup did not appear on target within 90s` | Cluster B 的 Velero 等了 90s 还没从 BSL 拉到这条备份的 metadata | 通常是 BSL 在 Cluster B 上凭据不对或网络不通。`kubectl get bsl -n velero` on B 看 PHASE，多半是 Unavailable |
| `502 target cluster: kubeconfig parse error` | 上传的 kubeconfig 无效或被吊销 | Settings → 集群管理 → 该集群 ⋮ → 移除 → 重新 Add，上传新 kubeconfig |
| Restore CR 创建成功但 Velero 把它标 PartiallyFailed | 不是 SupKube 的问题，是 Velero 的 restore 本身有问题 | 进 Activity → 点该 Restore → 抽屉里看 Errors + Warnings 详情 |

### 22.3 场景 C — 灾后重建（Cluster A 全挂）

**前提**：
- Cluster A 不可用（任何原因：API server 死了、网络分区、整个 region 挂了、勒索软件加密了 etcd 等）
- Cloud BSL 仍然完好（这是 3-2-1-1-0 的核心——Cloud BSL 应该在不同 region / 不同 cloud / 启用了 Object Lock）
- 你有一个全新的 Cluster B（与 A 不必同 region 不必同 cloud；k8s 版本 ≥ A 的 minor version）

**步骤**：

```
1. 在 Cluster B 上装 SupKube
   helm install supkube supkube/supkube \
     --namespace supkube --create-namespace \
     --set velero.enabled=true \
     --set localStore.enabled=false   # 灾备时本地 BSL 没意义，先不装

2. 登录 SupKube UI（http://<cluster-B-supkube>:30888）
   默认 admin@supkube.local / admin（首次启动）

3. 在 Storage Locations 页添加跟 Cluster A 同一个 Cloud BSL
   + Add Storage Location
   ─ Name:      （和 Cluster A 上同名，例如 myazureblob）
   ─ Provider:  Azure Blob Storage（或 AWS S3 / S3 Compatible）
   ─ Bucket:    （Cluster A 备份过去的同一个 bucket）
   ─ Region:    （同 Cluster A）
   ─ Credentials: 同 Cluster A 凭据
   点 Create → Status 应该是 Available（再点 ⋮ → Verify）

4. 等 60 秒（Velero BackupSyncController 默认 sync period）
   Backups → Restore Points 应该自动出现 Cluster A 备份过去的所有 RP
   Type 列显示 "Imported"（表示从 BSL 同步来的，非本集群产生）

5. 选要恢复的 RP → ⋮ → Restore
   Target Cluster 默认 this-cluster（也就是 Cluster B），不需改
   Target Namespace 选原 namespace（恢复到同名）或新名
   Start Restore

6. 进度看 Activity 页
   Velero on Cluster B 从 BSL 下载 backup tarball + Kopia 数据 → restore
   时长视数据量：metadata-only ~30 秒；含卷数据 = 数据量 / 网络带宽

7. 应用层验证
   kubectl get pods -n <ns> on Cluster B
   测试 endpoint、检查持久数据、跑 smoke test
```

**关键提醒**：
- 步骤 3 配 BSL 时**别忘了 SupKube 也需要这个 BSL 的凭据**——Cluster A 走 SupKube 创建的 BSL 用的是 `supkube-bsl-<name>-credentials` Secret；Cluster B 上需要手动重建。SupKube UI 的"Add Storage Location"会帮你建好这个 Secret。
- 如果 Cluster A 的 BSL 开了 **Object Lock**（推荐），凭据失效不影响读，Object Lock 保护对象在保留期内不被删——这是 3-2-1-1-0 "1 immutable" 的核心保障。
- 整个流程应该 **小于 1 小时**（除去 restore 数据传输本身）。建议每季度演练一次，把它跑顺。

### 22.4 演练频度建议（行业最佳实践）

| 场景 | 频度 | 验证什么 |
|---|---|---|
| §22.2 跨集群恢复（cluster 还在） | **每月** | kubeconfig 没过期；BSL 凭据有效；网络可达 |
| §22.3 灾后重建（全新集群） | **每季度** | 整套灾备 runbook 还行得通；Velero 版本兼容；团队会做 |
| Restore Verification（应用真起来）| **每月**（v0.9.6 后用 BPMN 引擎自动跑）| 不只 Restore 成功，应用 endpoint 真的能服务 |

**Tip：在演练之外保留至少 1 个"已知好"的 RP**——每次成功演练后给那个 RP 加 label `supkube.io/dr-verified=yes`，下次找 known-good baseline 时方便。

### 22.5 Restore SupKube 自身（v0.9.x+）

把 SupKube 的 ConfigMap / Schedule / Cluster CR 也备份起来，用于"SupKube 自己被弄坏"的恢复。当前 v0.9.0 不包含；规划在 v0.9.x DR1 sprint。临时方案：导出关键 CR 到 git 仓库：

```bash
kubectl get configmap supkube-settings -n supkube -o yaml > supkube-settings.yaml
kubectl get schedule -n velero -o yaml > schedules.yaml
kubectl get cluster.supkube.io -n supkube -o yaml > clusters.yaml
# commit to private git repo, weekly cron
```

---

## 23. Helm 安装参考（v0.9.1+ Install Reference）

> SupKube 的 Helm chart 暴露了一套覆盖**部署形态 / 认证 / 网络 / 制品来源 / 子组件开关**的 values。
> 本节是单一来源，对照 Kasten K10 [advanced install options](https://docs.kasten.io/latest/install/advanced/) 的对应位置。

### 23.1 最小可运行命令

```bash
helm repo add supkube https://charts.supkube.com/
helm repo update
helm install supkube supkube/supkube --devel \
  --namespace supkube --create-namespace \
  --set eula.accept=true \
  --set velero.enabled=true
```

`--devel` 是因为目前所有公开版本都是 `0.9.x-alpha.N` pre-release，正式版（无 `-alpha` 后缀）发出来后可去掉。

### 23.2 装之前先跑一次 Preflight

```bash
curl -fsSL https://charts.supkube.com/preflight.sh | bash
```

预检 10 项：K8s 版本 / kubectl 连通 / Helm 版本 / cluster-admin 权限 / StorageClass / CSI snapshot CRDs / snapshot-controller / VolumeSnapshotClass / 已有 Velero / 节点资源。返回 0 = 可以装，1 = 有 FAIL 项必须先修。

### 23.3 EULA 与 License（必填）

| values 路径 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `eula.accept` | bool | `false` | **必须显式设为 `true`**——否则 `helm install` 在 template render 阶段直接 fail，并打印需要加的参数提示。这是和 Kasten 一致的安装关卡。 |
| `eula.email` | string | `""` | 运维 / 续约联系邮箱。会写进 `cm/supkube-eula` 与 Settings → About。 |
| `eula.company` | string | `""` | 公司 / 团队名。同上。 |
| `license.key` | string | `""` | License Key。**当前 alpha 阶段任意字符串都通过**，空 = 社区免费版。v0.9.1+ License Manager 上线后会做签名校验。 |

实际安装命令：
```bash
helm install supkube supkube/supkube --devel ... \
  --set eula.accept=true \
  --set eula.email=ops@yourco.com \
  --set eula.company="Your Company Ltd" \
  --set license.key=YOUR-KEY-OR-EMPTY
```

### 23.4 镜像来源与 airgap

| values 路径 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `image.registry` | string | `""` | 全局镜像 registry 前缀。**空 = 直接从 `supkube.azurecr.io` 公开匿名拉**。设为 `harbor.internal.corp/supkube-mirror` 则所有镜像（backend/frontend/dex/minio/velero）都从这里拉。Airgap 客户必填。 |
| `backend.image.tag` / `frontend.image.tag` | string | 与 `appVersion` 一致 | 单独覆写某个组件的 tag（hotfix 场景）。 |
| `backend.image.pullPolicy` | string | `IfNotPresent` | 同上。 |

镜像架构: SupKube 镜像是 multi-arch manifest list（amd64 + arm64），K8s 节点自动按 CPU arch 挑变种，**客户安装命令零参数即可适配 AWS Graviton / Apple Silicon docker-desktop 等 ARM 集群**。

### 23.5 认证模式

| values 路径 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `auth.enabled` | bool | `true` | 开启 OIDC 登录流程。设 `false` 进入"demo 模式"（每个请求都是 admin，仅适合本地试用）。 |
| `auth.dex.enabled` | bool | `true` | 启用内置 Dex（自带 connector 配置）。和外部 IdP 接 OIDC 时改 `false`。 |
| `auth.oidc.issuerURL` | string | `""` | 外部 OIDC issuer。例：`https://customer-keycloak.example.com/realms/main`。当 `dex.enabled=false` 时必填。 |
| `auth.oidc.clientID` / `clientSecret` | string | `""` | 上面 IdP 给我们这个 client 的凭据。 |
| `auth.rbac.enabled` | bool | `false` | 开启 group/user → role 映射。生产强烈建议开。 |
| `auth.rbac.defaultRole` | string | `"viewer"` | 未匹配任何 binding 的用户的默认角色。 |
| `auth.rbac.bindings` | array | `[]` | RBAC 绑定列表，详见 §12。 |
| `auth.staticTokens.enabled` | bool | `false` | API token（给 CI/CD/Terraform 用）。详见 §14。 |
| `auth.basicAuth.enabled` | bool | `false` | Basic Auth（适合内网 proxy 已认证转发场景）。 |

对应 Kasten 的 `auth.openshift.*` / `auth.oidc.*` 系列——我们用 Dex 中间层统一抽象，外部 IdP 兼容度更广。

### 23.6 网络 / Ingress / TLS

| values 路径 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `service.frontend.type` | string | `NodePort` | 前端 SVC 类型。LoadBalancer / ClusterIP 可选。 |
| `service.frontend.nodePort` | int | `30888` | NodePort 端口。生产推荐改 Ingress。 |
| `ingress.enabled` | bool | `false` | 启用 Ingress（对应 Kasten 的 `ingress.create`）。 |
| `ingress.className` | string | `""` | IngressClass 名（nginx / traefik / istio）。 |
| `ingress.hosts[].host` | string | `supkube.local` | 域名。 |
| `ingress.annotations` | map | `{}` | cert-manager / 自定义 annotation 注入位置。 |

### 23.7 子组件开关

| values 路径 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `velero.enabled` | bool | `true` | 自动安装 Velero v1.18 subchart。**和 Kasten 最大的差别**——Kasten 让你自己装 Velero，我们 bundle。客户已有 Velero 时设 `false`。 |
| `velero.configuration.features` | string | `EnableCSI` | Velero feature flags。`EnableCSI` 是 CSI snapshot 必需。 |
| `velero.initContainers` | array | csi/aws/azure 三件套 | 默认装 CSI 插件 + AWS S3 + Azure Blob 插件。GCP 客户加 `velero-plugin-for-gcp`。 |
| `localStore.enabled` | bool | `false` | 在集群内起 MinIO 作为 Local BSL（实现 3-2-1-1-0 的 "1 immutable copy"）。多节点 + 默认 SC 时推荐开。 |
| `localStore.size` | string | `100Gi` | MinIO PVC 容量。 |
| `localStore.objectLock.enabled` | bool | `true` | S3 Object Lock（WORM 不可变）。 |
| `localStore.objectLock.mode` | string | `governance` | `governance`（admin 可解锁）/ `compliance`（即使 root 也不可删）。 |
| `localStore.bucket` | string | `supkube-local` | MinIO bucket 名。第一次装完别改。 |

### 23.8 资源限制

| values 路径 | 默认 |
|---|---|
| `backend.resources.requests.{cpu,memory}` | `100m / 128Mi` |
| `backend.resources.limits.{cpu,memory}` | `500m / 256Mi` |
| `frontend.resources.requests.{cpu,memory}` | `50m / 64Mi` |
| `frontend.resources.limits.{cpu,memory}` | `200m / 128Mi` |

大集群（>200 namespace）建议 backend limit 拉到 `2000m / 1Gi`。

### 23.9 完整安装样板

**生产推荐配置**：

```bash
helm install supkube supkube/supkube --version 0.9.0-alpha.4 --devel \
  --namespace supkube --create-namespace \
  --set eula.accept=true \
  --set eula.email=ops@example.com \
  --set eula.company="Example Inc" \
  \
  --set velero.enabled=true \
  --set localStore.enabled=true \
  \
  --set auth.enabled=true \
  --set auth.rbac.enabled=true \
  --set auth.rbac.defaultRole=viewer \
  \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.hosts[0].host=supkube.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```

**Airgap 配置**（把所有公网拉换成内网 mirror）：
```bash
helm install supkube supkube/supkube --version 0.9.0-alpha.4 --devel \
  --namespace supkube --create-namespace \
  --set eula.accept=true \
  --set image.registry=harbor.internal.corp/supkube-mirror \
  --set velero.image.repository=harbor.internal.corp/supkube-mirror/velero \
  ...
```

**已有 Velero 的客户**：
```bash
helm install supkube supkube/supkube --version 0.9.0-alpha.4 --devel \
  --namespace supkube --create-namespace \
  --set eula.accept=true \
  --set velero.enabled=false \      # ← 不重装 Velero，复用现有
  --set config.veleroNamespace=velero   # ← 指向现有 Velero 所在 ns
```

### 23.10 安装后验证

```bash
kubectl -n supkube get pods                 # 三个 pod 应都 Running
kubectl -n supkube get cm supkube-eula -o yaml   # 看 EULA 记录
helm -n supkube list                        # 看 chart 版本和 status
curl -sk https://supkube.example.com/api/v1/status   # /status 返回 backend version
```

---

## 24. Import Policy（跨集群持续 DR）（v0.9.1.13+）

> 给做**双活 / 暖备 / 跨 region DR** 的 SRE。本节是 v0.9.1.13 引入的 Import Policy 模型的客户面单一来源。架构权威：**ADR-038**；产品需求：**PRD-009 v2 §4.5** + **PRD-007 §4.4**。
>
> 关键差异：v0.9.1.13 之前，跨集群"看见对方备份"靠 Velero 内置 `backupSyncPeriod=60s` 全量扫描 BSL，**无 source 区分、无 fingerprint 校验、无 per-policy 控制**。v0.9.1.13 起，跨集群拉取由 SupKube 的 `ImportPolicy` CRD 显式管理，按 BSL × namespace × cron 收敛，并强制 HMAC 校验源真实性。

### 24.1 什么是 Import Policy

**模型**：共享 BSL（源、目标双方都能读写的同一个对象存储 bucket / container）+ HMAC fingerprint 校验（源端写入时签，目标端读入时验）+ 持续 / 定时 pull（目标端按 60s 间隔或 cron 拉取 metadata）。

**与 Snapshot Policy 的分工**：

```
┌─────────── Cluster A (源) ───────────┐         ┌────────── Cluster B (目标) ──────────┐
│                                       │         │                                       │
│  Snapshot Policy (Schedule CR)        │         │  Import Policy (新, ADR-038)         │
│      │                                │         │      │                                │
│      ▼                                │         │      ▼                                │
│  Velero Backup → 写 BSL X            │ ──BSL──>│  watch BSL X → fingerprint 校验 →   │
│      └─ 同时写 .supkube-fingerprint    │   X     │      ▼                                │
│         (HMAC-SHA256 over manifests)   │         │  apply Backup CR (label imported)    │
│                                       │         │      │                                │
│                                       │         │  UI Restore Points: chip = Imported  │
└───────────────────────────────────────┘         └───────────────────────────────────────┘
```

简单记：**源集群** 跑 Snapshot Policy 把数据**推**到 BSL；**目标集群** 跑 Import Policy 把那些 backup 的**身份信息**拉到本地，让 UI 能看到、能 Restore。**数据本身**永远只在 BSL 上一份，Restore 时 Velero 直接从 BSL 拉。

**vs 历史方案（v0.9.1.12 及之前）**：

| 维度 | Velero `backupSyncPeriod` | SupKube Import Policy |
|---|---|---|
| 拉取触发 | 全 BSL 全量扫描，每 60s | 按 ImportPolicy 收敛（可选 ns / label / source cluster） |
| source 真实性 | ❌ 无校验 → 谁能写 BSL 谁就能注入 | ✅ HMAC-SHA256 over manifest + sharedSecret |
| source 标识 | ❌ 看不出来自哪个集群 | ✅ `supkube.io/source-cluster=<id>` label |
| 失败可见性 | log only | `status.rejectedCount` + `lastError` + UI Activity 事件 |
| 节奏可调 | 全局一个 period | per-policy continuous(60s) 或 cron |

### 24.2 选 Continuous 还是 Scheduled

```
是否需要"目标集群一直随时可拉起"（暖备 DR）？
   │
   ├── YES → mode=Continuous, continuousInterval=60s（默认）
   │         ├── 想压成本？ → 改 300s（5 min，对标 Kasten）
   │         └── 准实时？  → 改 30s（advanced，警告 BSL API 调用费用 2 倍）
   │
   └── NO  → mode=Scheduled
             ├── 合规节奏（每 5 min 一次像 Kasten）→ schedule="*/5 * * * *"
             ├── 测试 / 偶尔同步              → schedule="0 2 * * *"（每天凌晨）
             └── 月度归档审计                  → schedule="0 3 1 * *"（每月 1 号凌晨）
```

**经验法则**：生产 DR 选 Continuous 60s；非生产 / 测试集群选 Scheduled，按需运行 Run-once。

### 24.3 RPO 公式与 Kasten K10 对比

**worst-case RPO 公式**：

```
worst_case_RPO = source_backup_interval + import_poll_interval
```

直觉：源端最坏要等一个 backup interval 才有新 backup；目标端最坏要等一个 import interval 才看到它。

**对比表**：

| 方案 | source 配置 | target 配置 | worst RPO | 备注 |
|---|---|---|---|---|
| Kasten K10 默认 | snapshot 每 5 min | sync 每 5 min | **10 min** | 行业基准 |
| SupKube 默认 | snapshot 每 5 min | Continuous 60s | **6 min** | 已击败 Kasten |
| SupKube aggressive | snapshot 每 1 min | Continuous 30s | **1.5 min** | 准实时；⚠ BSL list API 调用频率显著上升，云费用注意 |
| SupKube 合规 | snapshot 每 1 h | Scheduled `0 * * * *` | **2 h** | 每天 24 次；适合合规审计为主、RPO 要求宽的场景 |

> ⚠ "准实时"≠ CDP（Continuous Data Protection）。真正的 RPO≈0 需要应用级日志（PG WAL / MySQL binlog），不在 ImportPolicy 范畴。详见 ROADMAP "数据韧性 6 点路径" 第 1 项。

### 24.4 fingerprint 部署

**单集群场景**（同集群内自校验）：默认 `helm install` 已在 `supkube-fingerprint-secret` 里 generate-once 一个 32 字节随机值。客户**无需做任何事**。

**跨集群场景**（A 写 / B 读 同一 BSL）：**必须显式同步密钥**。

```bash
# 1. 在管理工作站生成一次（任一处都行）
openssl rand -base64 32
# → 例如：Hf3xK9pQwL4r8mC6vN2tBs7DyEa1nZuY0iJ5kFhXgRm=

# 2. Cluster A helm install / upgrade 时 --set
helm upgrade --install supkube supkube/supkube \
  --namespace supkube --create-namespace \
  --set fingerprint.sharedSecret="Hf3xK9pQwL4r8mC6vN2tBs7DyEa1nZuY0iJ5kFhXgRm="

# 3. Cluster B 用同样的 --set 值
helm upgrade --install supkube supkube/supkube \
  --namespace supkube --create-namespace \
  --set fingerprint.sharedSecret="Hf3xK9pQwL4r8mC6vN2tBs7DyEa1nZuY0iJ5kFhXgRm="
```

**失败排查**（三种 ERR）：

| 错误码 | 含义 | 排查命令 |
|---|---|---|
| `ERR_FINGERPRINT_MISSING` | BSL 上的 backup 没有 `.supkube-fingerprint.json` 文件（可能是非 SupKube 集群写的，或者源集群没装 v0.9.1.13+） | `mc ls X/backups/<bk>/` 看是否有 `.supkube-fingerprint.json`；源集群 `kubectl -n supkube get cm supkube-version` 查版本 |
| `ERR_FINGERPRINT_HMAC_INVALID` | HMAC 验证失败（最常见原因：sharedSecret 不匹配，或 backup tarball 被篡改） | 双集群分别 `kubectl -n supkube get secret supkube-fingerprint-secret -o jsonpath='{.data.shared-secret}'` 比对 |
| `ERR_FINGERPRINT_VERSION_MISMATCH` | fingerprint schema 版本与 import 端不兼容（升级窗口期可能出现） | 升级双集群到同一 minor 版本；过渡期把 `fingerprintMode` 临时改 `warn` |

### 24.5 UI 操作速查

**创建 Import Policy**：

```
Policies 页 → [Create New Policy] → Action Type 选 ⤵ "Import Policy"
  ├─ Source BSL:           （下拉，从已配置的 BSL 选）
  ├─ Source Cluster ID:    （可选；留空 = 接受任意源，填写 = 只接受指定 cluster-id）
  ├─ Mode:                 ● Continuous   ○ Scheduled
  ├─ Continuous Interval:  60s (默认) / 30s / 300s   ← Mode=Continuous 时显示
  ├─ Schedule:             */5 * * * *                ← Mode=Scheduled 时显示
  ├─ Fingerprint Mode:     ● enforce  ○ warn  ○ disabled
  ├─ Namespace Filter:     （可选，glob 表达式如 demo-*）
  └─ Label Selector:       （可选，K8s label selector 语法）
[Submit]
```

**列表行 kebab 菜单**：
- **Pause**：暂停拉取（CR `spec.paused=true`）；source 端继续 backup，但目标端 status 冻结
- **Resume**：恢复
- **Run Once**：立即触发一次 sync（不影响 cron 时刻表 / continuous 计时器）
- **Edit**：改 mode / interval / fingerprintMode / filter（**source BSL 不可改**，要换 BSL 请删了重建）
- **Delete**：删 ImportPolicy CR；**已 import 的 Backup CR 不会被删**（保留 RP 让用户决定 cleanup）

**监控 RPO 健康**：
- **列表行 status 列**：显示 `lastSyncAt` 相对时间（如 "32s ago"）；超过 2× `continuousInterval` 标红
- **Dashboard RPO 卡片**：v0.9.x 规划中（PRD-010），届时聚合所有 ImportPolicy 的 worst-case RPO 一屏可视

### 24.6 关联文档

- **架构权威**：架构设计.md ADR-038（Import Policy CRD + fingerprint HMAC 模型）
- **产品需求**：PRD-009 v2 §4.5（双活 DR 流程） / PRD-007 §4.4（5 层 3-2-1-1-0 的 Layer 4）
- **测试用例**：测试用例.md §9.5（TC-IMP-001/002/003/004，4 个端到端场景）
- **任务追踪**：#88 + #157-163（Import Policy 拆分系列）

---

## 25. 数据韧性评分（Resilience Score）解读

SupKube 的 **AI Backup Advisor** 会给每一个应用（namespace）打一个 **0–100 的数据韧性评分（Resilience Score）**，并配一个颜色徽章。这一节讲清楚两件事：**这个分是怎么来的**，以及 **你该怎么把它提上去**。

### 25.1 这个分凭什么 —— 对标国际标准，经得起审计追问

很多"安全评分"工具说不清"凭什么扣这 15 分"，到了合规审计就露馅。SupKube 的评分不一样：

- **对标四套国际标准**：每一个维度、每一个子项都映射到 **ISO 27002 / NIST CSF / NIST SP 1800-26（防勒索）/ NIST SP 800-53（应急计划 CP-9）**。审计师问"凭什么这个分"，每一分都能指回标准条款。
- **确定性规则引擎算分，可复现**：分数由一套固定的规则引擎计算，**同样的环境输入永远得到同样的分**。它不是"AI 估的"，没有随机性。
- **AI 只负责讲人话，绝不改分**：AI（LLM）只把规则引擎算出来的结果**翻译成自然语言**——告诉你哪里扣了分、为什么、怎么改。**它无权改动任何一分**。所以你在不同应用、不同集群之间看到的分是可以横向比较的。

> 一句话：**分是规则算的（可审计、可复现），话是 AI 说的（好懂、给建议）。**

### 25.2 四个维度怎么扣分（100 分制）

总分 100，分成四个维度。下表是每个子项的**分档**——你的环境命中哪一档，就拿那一档的分。

#### 维度 1 · 备份覆盖与合规（25 分，对标 ISO 27002 §8.13.1.a）

| 子项 | 满分 | 怎么拿满分 / 怎么扣分 |
|---|---|---|
| 应用分类覆盖度 | 10 | 按业务重要性做 **Tier 分级 + 差异化备份策略，核心应用 100% 纳管** = 10；统一策略未分级（或有少量僵尸应用）= 5；未分级且有漏备 = 0 |
| RPO 达标率 | 10 | 备份频率**满足或优于**业务 RPO = 10；基本满足但备份窗口过大 = 5；不满足 = 0 |
| 元数据与配置备份 | 5 | 连 **Manifest / 拓扑 / 配置**一起备 = 5；只备纯数据 = 0 |

#### 维度 2 · 韧性与冗余 3-2-1-1-0（35 分，对标 NIST CSF）

这是分值最高的维度，对应业界黄金法则 **3-2-1-1-0**（3 份副本、2 种介质、1 份异地、1 份隔离/不可变、0 个恢复错误）。

| 子项 | 满分 | 怎么拿满分 / 怎么扣分 |
|---|---|---|
| 介质与存储多样性【3-2】 | 10 | **至少 2 种介质** = 10；只有单一介质 = 5 |
| 异地与跨云【1·异地】 | 10 | **跨厂商或 >100km** = 10；同厂商跨 region = 6；同 AZ 同源 = 0 |
| 空气隔离与网络离线【1·隔离】 | 15 | **真 Air-Gap，或专网"备完即断网"** = 15；有隔离但仍在同网络域/同 AD = 8；无隔离、生产环境可直接挂载备份 = 0 |

#### 维度 3 · 防勒索与安全（20 分，对标 NIST SP 1800-26）

| 子项 | 满分 | 怎么拿满分 / 怎么扣分 |
|---|---|---|
| 不可变存储 | 10 | **WORM / Object Lock 用 COMPLIANCE（合规）模式** = 10；只用 Governance 模式或仅靠 IAM 权限挡 = 6；备份可被随意覆盖/删除 = 0 |
| 加密与凭证 | 5 | **静态 AES-256 + 传输 TLS 1.3 + 密钥独立轮换** = 5；有加密但密钥和数据同一套人管 = 2 |
| 访问控制 | 5 | **MFA + 最小权限 + 删除走二次审批 + 审计异地不可篡改** = 5；单密码、无 MFA、无审计 = 0 |

#### 维度 4 · 成功率与可恢复性（20 分，对标 NIST SP 800-53 CP-9）

| 子项 | 满分 | 怎么拿满分 / 怎么扣分 |
|---|---|---|
| 备份执行成功率 | 5 | 看**最近 14 天 / 最近 30 次**的备份成功率。⚠ **核心（Tier 1）资产连续失败 > 3 次，该项直接计 0** |
| 自动化恢复演练通过率【0·验证】 | 15 | **有全自动沙箱恢复，且近 3 个月通过率 100%** = 15；定期人工演练且有报告 = 10；只做过局部文件恢复测试 = 5；**只备份从不演练 = 0** |

> 这里的"0"对应 3-2-1-1-0 里的最后那个 **0（零恢复错误）**——光备份不算数，**能恢复出来才算数**。

### 25.3 四档安全级别：你的分意味着什么

| 分数 | 安全级别 | 含义 |
|---|---|---|
| **90–100** | 极高韧性级 | 对标 NIST 勒索防护，遭攻击也能分钟级拉起 |
| **75–89** | 合规风险低 | 满足 ISO 27001 基本审计；多厂商同时瘫痪的极端场景还有短板 |
| **60–74** | 脆弱级 | 无不可变防护，容易被勒索软件连同生产环境一起加密 |
| **< 60** | 高危级 | **伪备份**——随时面临数据永久丢失 |

### 25.4 两条"防虚高"硬规则 —— 为什么配置看着好却低分

有些环境配置项填得很全，但**实际上从没成功备份出一份数据**。为了不让一个"看起来很安全"的高分骗了你，评分引擎有两条**一票否决**的硬规则：

1. **从未成功备份过 → 即使算分很高，也封顶 30 分（高危级）。**
   配置得再漂亮，只要这个应用**一次都没成功备份过**，它就是裸奔。分数封到 30，提醒你赶紧跑通第一次备份。
2. **算分 ≥ 90，但最近一次备份失败 → 强制降到 30 分（高危级）。**
   配置看似满分，但**最近一次备份没成功**，说明保护链断了。这时候高分是假象，引擎直接把它打回 30，逼你先把备份修好。

> 看到一个应用"配置很全却是高危级"，**先去查它最近一次备份是否成功**——多半就是踩中了上面两条规则之一。

### 25.5 怎么提分：从高危级爬到极高韧性级

下面这些动作直接对应上面的扣分项，**做一项加一档**：

| 想提哪个维度 | 你该做的操作 |
|---|---|
| 先脱离"高危级"（最优先） | **跑通并稳定住备份**——确保每个应用都有至少一次成功备份、最近一次没失败（解开 25.4 两条硬规则） |
| 维度 2 · 异地与隔离（性价比最高，占 35 分） | 做一份**异地 / 跨厂商副本**（跨云或 >100km）；再配置 **Air-Gap**（备完即断网），把空气隔离那 15 分拿到手 |
| 维度 3 · 防勒索 | 在对象存储上**开启 Object Lock 的 COMPLIANCE（合规）模式**——这是和 Governance 模式的关键差别，能从 6 分提到 10 分；同时给备份配 AES-256 + TLS 1.3 + 独立密钥轮换，访问加上 MFA 和二次审批 |
| 维度 4 · 可恢复性 | **定期跑恢复演练**——哪怕先从"人工演练 + 出报告"（10 分）做起，再升级到"全自动沙箱恢复"（15 分）。只备不练永远是 0 分 |
| 维度 1 · 覆盖与合规 | 给核心资产**做 Tier 分级**并配差异化备份策略，核心 100% 纳管；调高备份频率满足 RPO；把 **Manifest / 配置**也一起纳入备份 |

> 想看具体某个应用扣在哪、AI 给的逐条改进建议，进 **Application 详情 → "AI 建议" tab**：每条建议都标了证据来源，部分还带"应用建议"按钮，一键跳到对应的策略向导帮你预填。

---

## 附录 A：常用 kubectl 排障命令

```bash
# 查看 SupKube 自身
kubectl get pods -n supkube
kubectl logs -n supkube deploy/supkube-backend --tail=100
kubectl logs -n supkube deploy/supkube-frontend --tail=100

# Velero
velero backup get
velero backup describe <name> --details
velero restore get
velero restore describe <name> --details
velero restore logs <name>

# CSI 快照
kubectl get volumesnapshots -A
kubectl get volumesnapshotcontents
kubectl get volumesnapshotclasses

# BSL 健康
kubectl get backupstoragelocations -n velero
kubectl describe bsl -n velero <name>

# 删除任务
kubectl get deletebackuprequests -n velero
kubectl get downloadrequests -n velero
```

---

## 附录 B：联系与反馈

- GitHub: https://github.com/mars-zhangcong/supkube
- 当前维护者: Mars Zhang
- 协议: TBD（计划 Apache 2.0）

---

*本手册随版本更新。当文档与实际行为不一致时，以代码行为为准并提 issue 修正文档。*
