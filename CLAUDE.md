# CLAUDE.md — SupKube 上岗说明书

> 这是 Claude Code 每次会话开头必读的入口。只写**代码里看不出、猜不到、每次会重踩**的东西；
> 泛泛的架构说明去看 [README.md](README.md)，别在这里复制。
> 维护原则：踩到一次坑就在这里记一条，让下一次会话不再踩。

## 这是什么

Kubernetes 原生数据保护平台，在 Velero 之上做 Kasten K10 式的备份/还原 UI + DR 编排治理。

```
supkube-backend/    Go 1.25 + Gin，包 Velero CRD（client-go / controller-runtime）
supkube-frontend/   Vue 3 + Element Plus + Vite
supkube-helm/       Helm Chart（含 velero 子 chart）
```

## 怎么跑 / 怎么改（内环）

| 目的 | 命令 |
|---|---|
| **本地全栈快调**（首选，改动 <1s 生效） | `./hack/dev-local.sh` |
| 只调 UI（port-forward 已部署后端 + 本地 Vite） | `./hack/dev-local.sh --mode ui` |
| 后端单跑 | `cd supkube-backend && go run main.go` |
| 前端单跑 | `cd supkube-frontend && npm install && npm run dev` |
| 前端测试 | `cd supkube-frontend && npm run test`（vitest） |
| 后端测试 | `cd supkube-backend && go test ./...` |

改动优先走 `dev-local.sh`，**不要**为了看一个小改动就 build-image→push→deploy 走一整圈（慢且贵）。
详见 [FAST-DEBUG-MODE.md](FAST-DEBUG-MODE.md)。

## 怎么部署（外环）

- **权威命令在 [CD/deploy-commands.md](CD/deploy-commands.md)** — 复制即用，别靠记忆拼命令。
- 工作流（ADR-042）：**push 到 main → aks-dev 自动部署**；tag → aks-test；prod 手动（待建）。
- aks-dev = Mars 日常验证集群，LoadBalancer IP **稳定 = `4.144.200.141`**（= 现运行的 `AUTH_PUBLIC_URL`）。
- 镜像走共享 ACR `supkube.azurecr.io`；取可用 tag：
  `az acr repository show-tags --name supkube --repository backend --orderby time_desc --top 5 -o tsv`

### 部署坑（血泪，务必记住）

- **`az acr build` 不认 `FROM --platform=...` 的 BuildKit 语法** → 构建前 sed 去掉那行，或用 `Dockerfile.acr`。
- **`az acr build` 偶发 `AuthenticationFailed`** → `az acr login --name supkube` 刷新后重试。
- **Dockerfile context 是子目录时**：需要的文件要先 `cp` 进构建目录（ACR 不跨目录取 context）。
- **Dex publicURL 必填**：启用 embedded Dex 时 `auth.dex.publicURL` = 用户浏览器访问的**外部地址**（如 aks-dev 的 `http://4.144.200.141`）。chart 故意 fail-fast，**没有安全默认**；给了 localhost 会在登录后静默崩。issuer 自动 = `<publicURL>/dex`。
- **Velero 命名空间**：chart 已把 Velero 收口进 release ns（`supkube`），后端 `VELERO_NAMESPACE` honor 它。**无需**再单独指定 velero ns。
- Helm 首次或子 chart 变更后先 `helm dependency build supkube-helm/supkube`。

## 文档在哪（重要：别在错的地方找）

生产管理文档已按"工厂文档分治"迁到 **`lighthouse-factory` 仓**，本仓根目录同名文件多是指针/退役桩：

| 你要找 | 去哪 |
|---|---|
| ROADMAP / 优先级矩阵 / Sprint 历史 | `lighthouse-factory/process/supkube/ROADMAP.md` |
| PRD / ADR / 架构权威记录 | `lighthouse-factory/products/supkube/records/`（prd/、adr/） |
| **新弧线 backlog（编排+治理 P01~P12）** | 本仓 `engineer-testing/supkube-roadmap/`（不进 git，本地排产用） |
| 部署命令 | `CD/deploy-commands.md`（本仓，权威） |
| API 契约 | `openapi.yaml`、`API-REFERENCE.md`（本仓） |

## Backlog 主线（2026-07）

- **L1 数据保护内核**（PRD-001~027）：大部分已 ship。最近活口 = **M1-M4 DR 评分 + Copilot**（评分引擎 / Dashboard / Azure OpenAI 对话 / function-calling 建备份），已部署 aks-dev 但 **`ai.enabled=false` gated**，启用只差 Mars 建 `supkube-aoai` secret + helm 开关。
- **L2/L3 新弧线**（`engineer-testing/supkube-roadmap/`）：编排引擎 → 治理平台。关键路径 `P01→P02→P05→P07→P08→P09`。
  - ⚠️ **未拍的前置裁决**：P01 排产前需一条 ADR 正式废止旧 "BPMN.io + Kanister" 线，否则两套编排引擎并存。

## 契约铁律（改编排相关代码前必读）

数据传递语法有**唯一事实来源**，在 `engineer-testing/supkube-roadmap/00-ROADMAP.md §3`（和《流程编排PRD-重写版》）。任何偏离即 bug：
- 字符串插值 `${plan.vars.X}` `${steps.<id>.outputs.<key>}` `${secret.<ref>}`…
- CEL 布尔条件用裸路径（不带 `${}`）。
- `${secret.*}` 日志强制脱敏；保存期静态校验引用可达性；运行期解析失败 fail-fast。

## 工作约定

- 大改动**先出计划、Mars 审计划、再动手**（方向错时快就是慢）。
- 并行不相干的活默认各开一棵 git worktree，物理隔离。
- 汇报守"实事求是"铁律：宁可报阻塞，绝不造假 passed / 绿灯。
