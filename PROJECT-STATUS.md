# SupKube 项目状态快照

> 最后更新：2026-05-15 (v0.5.1 发布后)
> 用途：休息一段时间后回来，5 分钟内快速回到项目状态

## 一句话现状

v0.5.1 已部署到本地 Docker Desktop K8S 上跑通，所有 Phase 1 PRD 功能可用；Applications/Backups/Restores/Policies 端到端验证通过；MinIO 作为 BSL 工作正常；每天 0 点 schedule 持续生成备份。

## 立刻打开看

| 链接 | 用途 |
|---|---|
| http://localhost:30888 | SupKube UI（v0.5.1 徽章可见即新版） |
| http://localhost:30888/api/v1/status | 健康检查 |
| [ROADMAP.md](ROADMAP.md) | 完整路线图 + 4 项关键架构决策 |
| [docs/PRD-MVP-Phase1.md](docs/PRD-MVP-Phase1.md) | Phase 1 产品需求文档 |
| [docs/SPRINT-v0.5.1-RETRO.md](docs/SPRINT-v0.5.1-RETRO.md) | v0.5.1 复盘（含血的教训） |

## 项目结构

```
supkube/
├── supkube-backend/    # Go + Gin REST API，包装 Velero CRD
├── supkube-frontend/   # Vue 3 + Element Plus + Vite
├── supkube-helm/       # Helm Chart
├── docker/             # Dockerfile.backend / Dockerfile.frontend
├── docs/               # PRD、Sprint 复盘
├── ROADMAP.md          # 路线图
└── PROJECT-STATUS.md   # 本文件
```

## K8S 部署状态（截至 2026-05-15）

```
Namespace: supkube
  - supkube-backend  : supkube/backend:0.5.1  Running
  - supkube-frontend : supkube/frontend:0.5.1 Running
  - Service frontend : NodePort 30888 → :80
  - Service backend  : ClusterIP :8080（仅集群内访问）
  - Helm release     : revision 5+, chart 0.1.1, appVersion 0.5.1

Namespace: velero
  - velero deploy    : 1/1 Ready
  - BSL `default`    : Available (MinIO at minio.minio:9000)
  - Schedule test-app: 0 0 * * * 每天 0 点

Namespace: minio
  - minio (Running)  : 提供 S3 兼容存储
```

## 几个回来时容易忘的注意点

1. **`.env` 文件锁死了 `VITE_API_URL=/api/v1`**（相对路径）—— 千万别改回 `http://localhost:8080`，那是 v0.5.1 之前的根因 bug。
2. **nginx 现在禁止缓存 `index.html` 和 `/api/`**，所以版本升级不需要手动清浏览器缓存。
3. **每次改完前端要重建镜像 + `kubectl delete pod`**，因为 Helm 默认不重启 pod 当 image tag 不变。
4. **`kubectl get backup -n velero` 看真实 Velero 状态**，UI 显示有问题时这是 ground truth。
5. **Restore 失败的真正原因**用 `/api/v1/restores/<name>/results` 看（Velero status 里的 errors/warnings/failureReason）。

## 下次起手的 3 个候选方向

按工作量从小到大排序：

### A. 体验小补 v0.5.2（半天）
- Dashboard Failed Restores 卡片
- 列表页搜索框
- Backup/Restore phase 筛选下拉

### B. Phase 2 第一项 v0.6.0（1-2 周）
- 跨命名空间恢复前端入口（**后端 e2e 已证明可用**，只缺 UI）
- 备份/恢复日志查看（Velero DownloadRequest 流程）
- Application 详情加 PVC/StatefulSets/Hooks

### C. 思考产品方向（不限时）
开放问题，未决定：

1. **定位**：SupKube 想做"Kasten 的开源替代"（功能对齐 90%），还是"Velero 的更好 UI"（轻量易用）？两者工作量差 5-10×。
2. **目标用户**：自托管中小团队（重简单）还是企业 PoC（重 RBAC/审计/合规）？决定 v0.8 是不是 GA 关键路径。
3. **社区路径**：要不要走 CNCF Sandbox + 文档站 + 社区运营？决定 v0.7-v0.9 要不要投入对外宣传。

## 已知架构决策（已定，别再纠结）

| 决策 | 版本 | 决策 |
|---|---|---|
| 卷数据备份模式 | v0.6 | 保留 Restic/Kopia + 新增 CSI 快照，**双模式并行** |
| Hooks 引擎 | v0.9 | **直接集成 Kanister**（不自研） |
| 多集群架构 | v0.9 | **Hub-Spoke**（中心管理多远端 Velero） |
| 认证方案 | v0.8 | **纯 OIDC**（不做本地用户系统） |
