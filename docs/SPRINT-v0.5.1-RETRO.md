# SupKube v0.5.1 Sprint 复盘

> Sprint period: 2026-05-14 → 2026-05-15
> Outcome: Shipped — but burned 2-3× the time it should have

## TL;DR

v0.5.1 sprint 的目标是修 6 个 P0/P1 bug 并完善 Applications 体验。**6 个 bug 全部修复并上线**，但实际花费的时间是预估的 2-3 倍。原因不是 bug 本身复杂，而是**被一个 7 字符的 `.env` 配置错误掩盖了所有真实 bug 的诊断信号**，导致大量精力浪费在追逐错误线索上。

---

## 完成清单

### P0 修复

| # | 修复 | 备注 |
|---|---|---|
| 1 | Backups/Dashboard `Queued` 显示错误 → 防御性 fallback | 抽取 `utils/phase.js` 统一处理 Velero phase；事后判断这个 bug 其实是浏览器 cache 假象 |
| 2 | Restore Delete/Results 后端 API | `handlers.go` 加 `DeleteRestore`、`GetRestoreResults`；后者返回 status.errors/warnings/validationErrors/failureReason，足够定位失败原因 |
| 3 | Restores 页面 Actions 列 + Results 抽屉 | `test-restore-1` 失败原因首次可见——MinIO `NoSuchKey: test-backup-1.tar.gz`，证明 v0.5.1 设计落地有效 |

### P1 改进

| # | 改进 | 备注 |
|---|---|---|
| 4 | Policies TTL `0s` → `Default (30d)` + 整天数自动折叠（168h → 7d） | 纯前端 `formatTTL` helper |
| 5 | Applications `ComplianceStatus` 5 态（Empty/Compliant/Unmanaged/NonCompliant/InProgress） | 后端加字段+derive 逻辑；前端按字段渲染 5 种 badge 颜色 |
| 6 | 系统 ns 过滤扩展（minio/restored-ns）+ 支持 `supkube.io/exclude=true` label | 黑名单 + 标签白名单两策略 |

### 体验额外补强（sprint 中追加）

- Applications 列表新增 **Labels 列**（前 2 个 tag + `+N more`）—— 既是产品功能，也是端到端验证锚点
- Application Details 抽屉标题 Kasten 风格（居中、20px、font-weight 700）
- Application Details 内 LABELS 区圆角 plain tag + "Show N more labels..." 折叠
- 顶部 `v0.5.1` 蓝色徽章——后续任何版本可以直接当部署确认锚

### 基建/可观测性强化

- nginx 缓存策略分层：
  - `index.html` → `no-store, no-cache, must-revalidate`
  - `/assets/*.{js,css,fonts}` → `public, immutable, max-age=1y`
  - `/api/*` → `no-store, no-cache, must-revalidate`（防止任何浏览器复用旧 API 响应）
- axios 全局 cache-buster：每个 GET 自动加 `?_=<timestamp>` 参数
- Helm chart 版本从 0.1.0 → 0.1.1，appVersion 0.1.0 → 0.5.1

---

## 关键发现（深刻教训）

### 🔥 真正的 root cause：`.env` 一行配置

`supkube-frontend/.env` 一直写着：

```
VITE_API_URL=http://localhost:8080/api/v1
```

后果：
- axios baseURL 永远是 `http://localhost:8080/api/v1`
- K8S 部署中 backend 是 ClusterIP，**8080 端口对浏览器不可达**
- 浏览器请求全部失败 → 浏览器从 disk cache 返回历史上某次成功响应（可能用户曾经做过 `kubectl port-forward 8080:8080`）
- 整个 SupKube 在 K8S 部署模式下，**axios 从来没真正与后端通信过**——所有"工作"全靠浏览器 cache 的旧数据

这是一个 0.5.0 之前就埋下的 bug，没人发现是因为页面看起来"有数据"。

### 走了多少弯路

排查这个 1 行 bug 期间，先后误判过：
1. ❌ 后端 `dashboard.go` 状态映射 bug → 修了，但其实后端早就对了
2. ❌ 前端 phase fallback `Queued` 字符串 → 加了 normalizePhase 工具，但 bundle 里从来没有 `Queued` 字符串
3. ❌ 浏览器 HTTP cache 没 Cache-Control → 给 nginx 加了 no-store
4. ❌ 浏览器 cache 持久顽固 → 让用户清缓存、用隐身窗口、换浏览器
5. ❌ Service Worker 拦截 → 让用户跑 `navigator.serviceWorker.getRegistrations()`（结果是 0）
6. ❌ CORS preflight → 移除了 axios 的 `Cache-Control` request header
7. ✅ 最终：Network tab 显示请求去了 `localhost:8080` 不是 `localhost:30888`，才发现 .env

总计触发了 **5-6 次重新构建镜像 + 重新部署**，每次都以为问题解决了。

### 为什么这么晚才发现

- 用户的现象（"页面显示旧数据"）跟 cache 问题的现象完美重合，导致一直往 cache 方向钻
- nginx access log 显示**完全没有 `/api/` 请求**——这本来应该是最早的预警信号，但我把它解读成"SW 拦截了"
- 没有第一时间让用户打开 Network tab 看实际请求 URL——这才是最直接的诊断手段

---

## 下次怎么避免

1. **诊断顺序**：客户端问题，**先看 Network tab**——这是真相，比 Console 报错和 nginx log 都直接。它能立刻告诉你：请求去哪了、状态码是什么、响应体是什么。
2. **不信任 "0 个 API 请求到 nginx" = "SW 拦截"**——更可能是请求去了别处（不同端口、不同主机、被代理）。
3. **每次部署到 K8S 后，第一件事不是改业务代码而是验证基础链路**：`curl /api/v1/status` + 浏览器 Network 看请求 origin 是否同源——30 秒能确认链路通畅。
4. **`.env` 文件做版本审计**——任何 hardcoded host:port 在生产部署里都是定时炸弹。CI 加一条 lint：禁止 `.env*` 出现 `http://` + 显式端口号。
5. **在 UI 上放版本徽章**（本次已加）—— 用户能秒判断"我看的是哪一版"，不会再纠结"是不是缓存"。

---

## 验证现状（2026-05-15 收尾时）

| 资源 | 状态 |
|---|---|
| Helm release | `supkube` revision 5+, chart 0.1.1, appVersion 0.5.1 |
| Backend pod | `supkube/backend:0.5.1` Running |
| Frontend pod | `supkube/frontend:0.5.1` Running |
| `/api/v1/status` | `{"status":"ok","version":"0.5.1"}` |
| `/api/v1/applications` 行数 | 2（default + test-app，过滤生效） |
| `/api/v1/applications` 返回 labels | ✅ |
| Velero schedule `test-app` | 仍在按 `0 0 * * *` 触发，每日凌晨备份 |
| MinIO BSL `default` | Available |
| 浏览器 UI v0.5.1 徽章 | 顶部右侧蓝色圆角 `v0.5.1` |

---

## 遗留事项（推 v0.5.2 或 v0.6 起手第一波）

- [ ] **API request 也加 `Cache-Control: no-cache` 但避免 preflight**——简单方案：把 header 放在 response 端（已做），不必再加到 request 端。但要确保 Go 后端的 gin CORS 中间件如果未来加了 OPTIONS 处理，`Access-Control-Allow-Headers` 要包含必要 header。
- [ ] **Settings.vue 显示的 API URL** 现在是 `/api/v1` 相对路径——可以改成显示完整 origin 让用户看清"我连的是哪个后端"。
- [ ] **`fewer-permission-prompts`**：sprint 中频繁 `kubectl exec` / `docker run` / `helm upgrade`，触发了大量交互式确认，可以加入 allowlist 提速。
- [ ] **e2e 测试**覆盖："清空浏览器后访问 /applications 能看到 labels 列"——本次 sprint 暴露的关键回归路径。
- [ ] **PRD 中 Phase 1 验收标准里没有"前端必须能在生产 K8S 部署下正常通信"这条**——加进 PRD。
- [ ] Dashboard "Failed Restores" 卡片（ROADMAP v0.5.x 体验小补里列过，没做）

---

## 关键架构决策（在 ROADMAP 中已定，提醒自己）

| 决策 | 版本 | 决策 |
|---|---|---|
| 卷数据备份模式 | v0.6 | 保留 Restic/Kopia + 新增 CSI 快照，**双模式并行** |
| Hooks 引擎 | v0.9 | **直接集成 Kanister**（不自研） |
| 多集群架构 | v0.9 | **Hub-Spoke**（不 federation） |
| 认证方案 | v0.8 | **纯 OIDC**（不做本地用户） |

---

## 下次回来从哪儿起手

按风险/回报排序：

1. **（30 分钟）**修一下本次发现但没做的小遗留：Dashboard Failed Restores 卡片、Applications/Backups/Restores 列表加搜索框筛选（ROADMAP v0.5.x 已列入）
2. **（半天）**写 e2e 防回归测试：包括"K8S 部署下 axios 走相对路径"这条
3. **（1-2 周）**进 v0.6 Phase 2 第一项：跨命名空间恢复前端入口（后端 e2e 已证明可用，只缺 UI）

如果你想先思考一下方向，几个值得想的开放问题：
- SupKube 想做"Kasten 的开源平替"，还是"Velero 的更好 UI"？前者要求功能对齐 Kasten 90%（含 Kanister Blueprints、多集群、Compliance Score），后者只要在 Velero 之上做轻量易用 UI 即可。两者工作量差 5-10 倍。
- 目标用户：自托管的中小团队（重点是简单），还是要进企业客户的 PoC（重点是合规/审计/RBAC）？这决定 v0.8 RBAC 应不应该是 GA 关键路径。
- 是否要走 CNCF Sandbox / 文档站 / 社区运营？这是产品路径，决定要不要在 v0.7-v0.9 投入 docs 站建设和 Kanister 集成的对外宣传。
