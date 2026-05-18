# SupKube v0.5.2 Sprint 复盘

> Sprint period: 2026-05-15 → 2026-05-18 (~3 days)
> Outcome: ✅ Shipped — 大量"超前完成"的 v0.7 / v0.6 工作被吸收进 v0.5.2

## TL;DR

v0.5.2 原本计划只是 v0.5.1 复盘里列的"体验小补"（搜索筛选、Failed Restores 卡片等），实际执行中**大幅扩张**：在做 UX 优化时引出了**Kasten 风格全面对齐**的机会，顺势把 Sidebar、Storage Locations 详情/编辑、Applications 筛选、Restore Points 重构都一并落地。同时**CSI 快照基础设施**在 v0.6 Roadmap 里本来是单独一个 sprint，本次也搭通了端到端。

---

## 完成清单

### Kasten 风格 UX 改造

| 改造 | 文件 |
|---|---|
| **Sidebar 浅色 Kasten 风格**（白底、深字、圆角菜单项、48px 选中态） | `src/App.vue` |
| **SupKube logo + favicon**（盾牌 + S monogram，蓝 #326CE5） | `public/supkube-logo.svg`, `public/supkube-favicon.svg`, `index.html` |
| **Logo 区与顶部 header 底边对齐**（60px height） | `src/App.vue` |
| **顶部 v0.5.x 徽章**（蓝色圆角，每次升级方便验证） | `src/App.vue` |
| **Applications 筛选 toolbar**（Status 下拉 + Name 搜索 + N selected） | `src/views/Applications.vue` |
| **Applications labels 长 tag 截断 + tooltip** | `src/views/Applications.vue` |
| **Applications 多选 checkbox** | `src/views/Applications.vue` |
| **Restore Points 完全重构**（Namespace/Type/Policy/Profile 列、kebab 菜单、批量删除）| `src/views/Backups.vue` |
| **Storage Locations Kasten kebab 菜单**（Verify/View Details/Edit/Delete） | `src/views/StorageLocations.vue` |
| **Storage Locations 详情抽屉 + Edit 对话框** | `src/views/StorageLocations.vue` |
| **Application Details 抽屉标题加粗** + LABELS 区圆角 tag | `src/views/Applications.vue` |

### 功能扩展

| 功能 | 文件 |
|---|---|
| **SupVault provider** (S3 兼容，加入 Provider 下拉) | `src/views/StorageLocations.vue` |
| **RFC1123 命名校验** (前端实时 + 后端兜底，避免大写字符导致 K8s API 拒绝) | `src/views/StorageLocations.vue` + `supkube-backend/internal/api/v1/storage.go` |
| **Storage Location Edit / Delete / Get API** | `supkube-backend/internal/api/v1/storage.go` + `cmd/server/server.go` |
| **删除 BSL 级联清理 supkube-managed Secret** | `supkube-backend/internal/api/v1/storage.go` |
| **Applications 放开系统 ns 过滤**（应用户要求展示完整 ns 列表） | `supkube-backend/internal/api/v1/applications.go` |

### 基础设施（v0.6 提前到 v0.5.2）

| 组件 | 状态 |
|---|---|
| **external-snapshotter v8.0.1**（CRDs + 2/2 controller） | ✅ kube-system 下 |
| **csi-driver-host-path**（8/8 plugin running） | ✅ default ns 下 |
| **StorageClass `csi-hostpath-sc`** | ✅ |
| **VolumeSnapshotClass `csi-hostpath-snapclass`** + `velero.io/csi-volumesnapshot-class=true` label | ✅ |
| **Velero v1.18.0 启用 `--features=EnableCSI`** | ✅ |
| **ClusterRole `velero-csi-snapshot`** | ✅（snapshot.storage.k8s.io 权限） |
| **端到端验证**：Velero backup `csiVolumeSnapshotsCompleted: 1` → 删 PVC → restore → PVC 重建 Bound | ✅ |
| **复现文档 `docs/csi-snapshot-setup.md`** | ✅ |

### 路线图增量

| 决策 | 落地 |
|---|---|
| **v0.7.5 新增 Backup Advisor MVP** | ROADMAP.md 新增章节 + 衍生评审项 |
| **CSI 双模式（FS + CSI）方向锁定** | v0.6 Roadmap 章节细化 |

---

## 关键经验

### 顺手做改进的边界

本次 sprint **故意接受"超前完成"**：当一个改进自然引出相邻区域的改进时，趁热打铁比留 TODO 更省力（context 已经在手）。但有几条边界：
- 不动 backend API 路径，保持向前兼容
- 路由保持 `/backups`（侧边栏菜单文字改"Restore Points"），不破坏链接
- 不引入新依赖（除了 logo SVG 静态资源）

### 部署链路验证模板

K8S NodePort 部署的应用，**不能用 preview_start 之类的 dev server 工具验证**（端口已占）。本 sprint 沉淀了一个验证三段式：
1. `docker build` 后 `docker run --rm <img> grep` bundle 内字面量，确认新代码进了镜像
2. `kubectl delete pod` 强制重建（image tag 不变时 Helm 不会主动滚动）
3. `curl http://localhost:30888/...` + `kubectl logs` 看 nginx access log，确认浏览器请求真的到达

每次有"我改了但 UI 没变"的怀疑都用这套排查，v0.5.1 浪费几小时追 SW/cache 的事不会再发生。

### CSI 集成最大的坑

- Velero **v1.14+ 不再需要 `velero-plugin-for-csi` 插件**（CSI 集成已内置在 core 里）。网上很多教程仍在教旧版命令，浪费时间。本 sprint 已写进 `csi-snapshot-setup.md`。
- `csi-driver-host-path` 的 plugin Pod 含 8 个 sidecar 容器，**首次拉镜像可能 15-25 分钟**（国内网络），看到 0/8 不要慌、不要 delete pod 重新计时。
- VolumeSnapshotClass 必须打 `velero.io/csi-volumesnapshot-class=true` label，Velero 才会用它。文档容易漏。

---

## 遗留事项（推 v0.6+）

### v0.5.1 列了但仍未做的小坑

- [ ] Dashboard 加 Failed Restores 卡片 / Recent Restores Failed 高亮
- [ ] Restores 页面加 phase 筛选下拉
- [ ] Settings 页面 API URL 显示完整 origin

### v0.6 已起手（B 阶段）

- [ ] VolumeSnapshotLocation 管理页面
- [ ] 备份创建表单 Volume Backup Mode 单选（Filesystem / CSI）
- [ ] BackupDetail 显示 csiVolumeSnapshotsAttempted/Completed 进度
- [ ] Restores 页面尊重源 backup 的卷模式

### v0.6 仍未起手

- [ ] 跨 ns 恢复前端入口（后端 e2e 已通过）
- [ ] Resource Transform（StorageClass 映射 / Image registry 替换）
- [ ] Backup/Restore 日志查看（通过 Velero DownloadRequest）
- [ ] Application 详情加 PVC / StatefulSets / Hooks 信息

---

## 部署状态（sprint 结束时）

```
namespace=supkube
  - supkube-backend  : supkube/backend:0.5.2   Running
  - supkube-frontend : supkube/frontend:0.5.2  Running
  - Helm release     : revision N+1, chart 0.1.2, appVersion 0.5.2

namespace=velero
  - velero v1.18.0   : --features=EnableCSI
  - BSL `default`    : Available (MinIO)
  - BSL `supvault`   : Unavailable (user-configured external)

namespace=kube-system
  - snapshot-controller : 2/2 Running

namespace=default
  - csi-hostpathplugin-0 : 8/8 Running
  - csi-hostpath-socat-0 : 1/1 Running

storageclasses:
  - hostpath (default)    : Velero/Restic 路径
  - csi-hostpath-sc       : CSI 路径（v0.6 双模式 backup 用）

volumesnapshotclasses:
  - csi-hostpath-snapclass: 带 velero label
```
