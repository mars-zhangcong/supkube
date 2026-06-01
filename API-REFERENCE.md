# SupKube API 参考（API Reference）

> **用途**：SupKube 后端 REST API 的**单一权威目录**——前端、MCP Server、AI Advisor、第三方集成都引用本文件，不再各猜各的（消除 ENGINEERING.md §6「API 无契约」债）。
> **读者**：前端研发、集成方、写 MCP Skill 的人、SRE。
> **权威源**：端点清单 + 角色门槛**派生自** `supkube-backend/internal/auth/rbac.go` 的 `permissionTable`（外加 `internal/importpolicy/rest_routes.go` 的 `RegisterPermissions`）。机器可读契约见 [`openapi.yaml`](openapi.yaml)。
> **关联**：[架构设计.md](架构设计.md)（ADR-005 双 URL OIDC · ADR-035 错误码体系）· [术语表.md](术语表.md) · [ENGINEERING.md](ENGINEERING.md)

---

## 1. 基础约定

| 项 | 值 |
|---|---|
| **Base path** | `/api/v1` |
| **集群内地址** | `http://supkube-backend.<ns>.svc:8080/api/v1` |
| **外部地址** | `https://<ingress-host>/api/v1` |
| **请求/响应体** | `application/json`（UTF-8） |
| **公开探针**（免认证） | `GET /api/v1/status` |
| **版本** | path 里 `v1`；破坏性变更走 `v2`，不在 v1 内改契约 |

> **单一来源纪律（ENGINEERING.md Rule C）**：本目录的「端点 × 角色」矩阵以 `rbac.go` 为准。**新增端点必须同时登记 `permissionTable`**——后端对不在表里的端点**fail-closed 返回 403**（见 §4），所以漏登记会立刻暴露，不会静默放行。

---

## 2. 认证（Authentication）

所有 `/api/v1/*`（除 §3 公开端点）要求 `Authorization` 头。后端按前缀分派三种模式（见 `internal/auth/auth.go`）：

| 模式 | 头格式 | 用途 |
|---|---|---|
| **静态 API Token** | `Authorization: Bearer <opaque-token>` | 脚本 / MCP Server / CI；不走 OIDC |
| **OIDC JWT** | `Authorization: Bearer <jwt>` | 人类用户经 Dex 登录后拿到的 token（ADR-001/005 双 URL 设计） |
| **Basic** | `Authorization: Basic <b64(user:pw)>` | htpasswd 兜底（小集群 / air-gap 无 IdP 时） |

登录流（OIDC）：

```
GET  /api/v1/auth/providers     → 列出可用 IdP（公开）
POST /api/v1/auth/callback      → 用 code 换 token（公开）
GET  /api/v1/auth/me            → 当前身份 + 角色 + namespace 作用域（viewer+）
POST /api/v1/auth/logout        → 注销
```

> **Demo / RBAC 关闭模式**：`RBAC_ENABLED=false` 时角色解析为 `admin`，`IsAtLeast` 恒真——中间件退化为 no-op（方便本地起跑），**生产务必开启**。

---

## 3. 授权模型（RBAC）

三个角色，单调升级 `viewer < editor < admin`（`internal/auth/auth.go`）：

| 角色 | 能做什么 |
|---|---|
| **viewer** | 读一切（**除** audit-log 与 RBAC bindings）；可跑只读 dry-run（如 `preview-resolution`） |
| **editor** | viewer + **Backup / Restore / Schedule / ImportPolicy 写**；受 **namespace 作用域**约束（见下） |
| **admin** | 一切：BSL/VSL CRUD、Transform/TransformSet 管理、Namespace 增删、Cluster 注册、Backup Copy 搬字节、清理孤儿、看 audit |

**Namespace 作用域**：editor 的写操作还要过 `RequireNamespaceAccess()`——操作触及的每个 ns 必须在 `user.NamespaceScope` 内（admin/viewer 不受限）。该检查在各 handler 内做（因为 ns 可能在 URL、body 或两者），不在中间件。

---

## 4. 错误响应（Error Envelope）

两种形态并存（新端点逐步迁向结构化，ADR-035）：

**① 结构化（ADR-035，ImportPolicy / 新模块）**
```json
{ "error": "import policy 'foo' already exists", "code": "ERR_IMPORTPOLICY_ALREADY_EXISTS" }
```
- `error`：给人看的话（可直接 toast）。
- `code`：`ERR_<DOMAIN>_<REASON>`，给程序判分支用，**稳定不随文案变**。

**② 传统（早期端点）**
```json
{ "error": "namespace \"kube-system\" is protected and cannot be created via SupKube" }
```
只有 `error`，无 `code`。集成方应**先看 HTTP status + 可选 `code`，再 fallback 到 `error` 文本**。

**RBAC 相关固定形态**：
```json
// 401 未认证
{ "error": "missing or malformed Authorization header" }
// 403 角色不足
{ "error": "insufficient role", "required": "admin", "current": "editor" }
// 403 namespace 越权
{ "error": "namespace not in your scope", "namespace": "prod", "scope": ["dev","staging"] }
// 403 端点未登记 RBAC 表（这是 SupKube bug，应报告）
{ "error": "endpoint not in RBAC table (this is a SupKube bug — please report)" }
```

**错误码族（ERR_ families，权威源 = 各模块常量）**：

| 族 | 模块 | 例 |
|---|---|---|
| `ERR_IMPORTPOLICY_*` | ImportPolicy | `_NOT_FOUND` `_ALREADY_EXISTS` `_CRON_INVALID` `_INTERVAL_TOO_SHORT` `_INVALID_MODE` `_INVALID_FINGERPRINT_MODE` `_BSL_NOTFOUND` `_BAD_REQUEST` `_INTERNAL` |
| `ERR_FINGERPRINT_*` | Fingerprint enforce/warn | （指纹校验失败族） |
| `ERR_LAYER4_*` | Backup Copy | `ERR_LAYER4_SNAPSHOT_UNSUPPORTED`（CSI-snapshot-only 不能 BSL→BSL copy） |

---

## 5. 列表与分页约定

读多个的端点统一返回：
```json
{ "items": [ ... ], "total": 42 }
```
（部分新端点只返回 `{ "items": [...] }`，无 `total`——以 `openapi.yaml` 各端点为准。）

---

## 6. 公开探针：`GET /api/v1/status`

免认证。脚本在碰凭据**之前**用它确认「跑的是哪个 build」（verify-before-ship，Rule D 的 buildStamp 检查就靠它）：

```bash
curl -s https://<host>/api/v1/status
```
```json
{ "status": "ok", "version": "0.9.1.10-alpha", "buildStamp": "260531-2310" }
```
> `buildStamp` 形如 `YYMMDD-HHMM`，由 `-ldflags -X ...version.BuildStamp=` 注入。`helm upgrade` 后**buildStamp 变了**才证明新镜像真的滚出去了（不是缓存）。

---

## 7. 端点目录（Endpoint Catalogue）

> 派生自 `rbac.go` + `importpolicy/rest_routes.go`。`Min Role` = 调用所需**最低**角色。带 †者 editor 调用时额外过 namespace 作用域检查。

### 7.1 状态 / 仪表板
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/status` | — 公开 | build 探针（buildStamp） |
| GET | `/dashboard/summary` | viewer | 首页汇总卡 |
| GET | `/dashboard/topology` | viewer | DR 拓扑聚合（5 层节点） |
| GET | `/multicluster/summary` | viewer | 跨集群聚合（MCM Dashboard） |

### 7.2 认证
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/auth/providers` | — 公开 | 可用 IdP |
| POST | `/auth/callback` | — 公开 | code 换 token |
| POST | `/auth/logout` | viewer | 注销 |
| GET | `/auth/me` | viewer | 我是谁 + 角色 + ns 作用域 |
| GET | `/auth/rbac/bindings` | admin | RBAC 绑定审计（只读） |
| GET | `/audit-logs` | admin | 审计日志查询 |

### 7.3 应用 / 备份顾问
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/applications` | viewer | 应用（namespace 维度）列表 |
| GET | `/applications/:namespace/details` | viewer | 应用详情 |
| GET | `/applications/:namespace/storage-capability` | viewer | 存储能力检测（CSI/快照） |
| POST | `/applications/:namespace/snapshot` | editor † | 一键 Snapshot（=建 Backup CR） |
| GET | `/backup-advisor` | viewer | 备份顾问（全局建议） |
| GET | `/backup-advisor/:namespace` | viewer | 备份顾问（单 ns） |

### 7.4 备份（Backups）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/backups` | viewer | 列表 |
| GET | `/backups/:name` | viewer | 详情 |
| GET | `/backups/:name/resources` | viewer | 备份的 K8s 资源清单 |
| GET | `/backups/:name/artifacts` | viewer | 制品（卷/对象） |
| GET | `/backups/:name/artifact-breakdown` | viewer | 制品分类统计 |
| GET | `/backups/:name/errors` | viewer | 错误 + warning 解读 |
| POST | `/backups` | editor † | 创建备份 |
| DELETE | `/backups/:name` | editor † | 删除（级联 DeleteBackupRequest） |
| POST | `/backups/:name/force-delete` | editor † | 强删卡住的 Backup CR（全集群备份升 admin） |

### 7.5 还原（Restores）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/restores` | viewer | 列表 |
| GET | `/restores/:name` | viewer | 详情 |
| GET | `/restores/:name/results` | viewer | 还原结果明细 |
| POST | `/restores/preflight` | editor † | 还原前置检查（只读诊断） |
| POST | `/restores` | editor † | 发起还原 |
| DELETE | `/restores/:name` | editor † | 删除还原记录 |

### 7.6 活动（Actions）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/actions` | viewer | 活动/任务列表 |
| GET | `/actions/:id` | viewer | 任务详情（时间线见 PRD-006） |

### 7.7 计划（Schedules）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/schedules` | viewer | 列表 |
| GET | `/schedules/:name` | viewer | 详情 |
| POST | `/schedules` | editor † | 创建 |
| PATCH | `/schedules/:name` | editor † | 局部更新（暂停/改 cron 等） |
| POST | `/schedules/:name/run-once` | editor † | 立即跑一次 |
| DELETE | `/schedules/:name` | editor † | 删除 |

### 7.8 导入策略（Import Policies · PRD-009 / ADR-038）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/import-policies` | viewer | 列表 |
| GET | `/import-policies/:name` | viewer | 详情 |
| POST | `/import-policies` | editor | 创建（替代 Velero `backupSyncPeriod` 60s 兜底） |
| PUT | `/import-policies/:name` | editor | 全量更新 |
| DELETE | `/import-policies/:name` | editor | 删除 |
| POST | `/import-policies/:name/run-once` | editor | 立即同步一次 |
| POST | `/import-policies/:name/pause` | editor | 暂停 |
| POST | `/import-policies/:name/resume` | editor | 恢复 |

### 7.9 Transform / TransformSet（PRD-002 两层模型）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/transform-sets` | viewer | TransformSet 列表 |
| GET | `/transform-sets/:name` | viewer | 详情 |
| POST | `/transform-sets/:name/preview-resolution` | viewer | dry-run 编译预览（只读，不建 CM） |
| POST | `/transform-sets/apply-conflict-fixes` | editor | Preflight 一键修冲突 |
| POST | `/transform-sets` | admin | 创建 |
| PUT | `/transform-sets/:name` | admin | 更新 |
| DELETE | `/transform-sets/:name` | admin | 删除 |
| GET | `/transforms` | viewer | 原子 Transform 列表 |
| GET | `/transforms/:name` | viewer | 详情 |
| POST | `/transforms` | admin | 创建 |
| PUT | `/transforms/:name` | admin | 更新 |
| DELETE | `/transforms/:name` | admin | 删除 |

### 7.10 存储位置（BSL / VSL）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/storage-locations` | viewer | BSL 列表 |
| GET | `/storage-locations/:name` | viewer | 详情 |
| POST | `/storage-locations` | admin | 创建 BSL |
| PUT | `/storage-locations/:name` | admin | 更新 |
| DELETE | `/storage-locations/:name` | admin | 删除 |
| POST | `/storage-locations/:name/verify` | admin | 连通性校验 |
| GET | `/volume-snapshot-locations` | viewer | VSL 列表 |
| GET | `/volume-snapshot-locations/:name` | viewer | 详情 |
| POST | `/volume-snapshot-locations` | admin | 创建 VSL |
| DELETE | `/volume-snapshot-locations/:name` | admin | 删除 |
| GET | `/local-store/status` | viewer | 本地备份库状态（Layer 1） |
| GET | `/storage/csi-autoconfig` | viewer | CSI 自动适配状态（#84） |

### 7.11 Backup Copy（Layer 4 · PRD-007 §4.3）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| POST | `/backup-copy/preflight` | editor † | 列出可 BSL→BSL 复制 vs CSI-only 被拒（`ERR_LAYER4_SNAPSHOT_UNSUPPORTED`） |
| POST | `/backup-copy` | admin | 执行复制（**跨 BSL 搬字节，可能产生云出口费**→ admin 门槛） |

### 7.12 集群注册（Multi-Cluster · MCM）
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/clusters` | viewer | 集群列表（切换上下文需要） |
| GET | `/clusters/:name` | viewer | 详情 |
| POST | `/clusters` | admin | 注册集群（**上传 kubeconfig=授予远端写权**→ admin） |
| DELETE | `/clusters/:name` | admin | 注销 |
| POST | `/clusters/:name/test` | admin | 连通性测试（已注册） |
| POST | `/clusters/test` | admin | 连通性测试（注册前预检） |

### 7.13 命名空间 / 资源 / 可观测 / 设置
| Method | Path | Min Role | 说明 |
|---|---|---|---|
| GET | `/namespaces` | viewer | ns 列表 |
| POST | `/namespaces` | admin | 创建 ns（Restore 抽屉新建目标用；拒绝保护 ns） |
| GET | `/resources/yaml` | viewer | 取任意资源 YAML |
| GET | `/logs` | viewer | 日志查询（Log Viewer，#79/PRD-005） |
| GET | `/logs/components` | viewer | 可选组件列表（日志分类） |
| GET | `/plugins/status` | viewer | 插件安装状态（Velero plugins） |
| GET | `/settings/branding` | viewer | 白标品牌（每页渲染需要） |
| PUT | `/settings/branding` | admin | 改品牌 |
| GET | `/settings/cleanup` | admin | 清理设置 + 上次运行摘要（含孤儿计数） |
| PUT | `/settings/cleanup` | admin | 改清理设置 |
| POST | `/admin/cleanup/orphans` | admin | 手动清理孤儿资源 |

---

## 8. 给集成方的 5 条建议

1. **先 `GET /status` 探活探版本**，再带凭据打别的（脚本/CI 标准姿势）。
2. **认错误优先用 HTTP status + `code`**，别正则匹配 `error` 文案（文案会改，`code` 不会）。
3. **写操作要处理 403 + namespace scope**——editor token 不是万能的。
4. **列表别假设有 `total`**——按 `openapi.yaml` 各端点定义读。
5. **写 MCP Skill / AI Advisor**：只读端点（`backup-advisor` / `dashboard` / `applications`）足够做「建议」；**绝不**调用写端点自动改客户集群（Rule F 非自治）。

---

## 9. 维护约定

- 新增/改端点 → **同步改 `rbac.go` 的 `permissionTable`**（fail-closed 会逼你改）→ 在本表登记一行 → 在 `openapi.yaml` 补 path/schema。
- 本文件的「端点 × 角色」矩阵应与 `rbac.go` 一致；理想终态是从 `rbac.go` + handler struct 自动生成 `openapi.yaml`（见 ENGINEERING.md §6 债：API 契约）。

---

## 变更记录

| 日期 | 操作人 | 变更 |
|---|---|---|
| 2026-06-01 | Claude | 初版。从 `rbac.go` permissionTable + importpolicy RegisterPermissions 派生完整端点目录（13 组 ~90 端点）；补认证三模式、RBAC 三角色、ADR-035 错误信封 + ERR_ 族、`/status` buildStamp 契约。补 ENGINEERING.md §6「API 无契约」债。 |
