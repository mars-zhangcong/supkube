# 快速调试模式 (Fast Debug Mode)

> 触发词:当 Mars 说 **"进入快速调试模式"** 时,按本文档执行。

## 这是什么

把"改代码 → 打镜像 → 推 ACR → CI/CD → 拉进 AKS → Pod 重启"这条**分钟级**的慢循环,
换成本地**秒级**内循环,用于边改边看的快速迭代(尤其是 UI / 表格 / 表单这类高频小改动)。

镜像构建与部署只属于**发布**通道;调试阶段不该碰它。

## 怎么进入

一条命令(见 [hack/dev-local.sh](hack/dev-local.sh)):

```bash
# 纯前端 / 只改 UI(改一堆表格、按钮、样式)—— 最快,不碰 Go,不打镜像
./hack/dev-local.sh --mode ui          # 端口转发集群里已部署的后端,只本地跑前端

# 前后端一起改(比如表格加字段,后端 API 也要返回新字段)
./hack/dev-local.sh                     # 默认 full:本地 go run 后端 + Vite 前端
./hack/dev-local.sh --context docker-desktop
```

浏览器统一开 **http://localhost:3000**;改 `.vue` 文件 **<1 秒** HMR 生效。
`Ctrl-C` 一次,前端 / 后端 / 端口转发全部一起清理。

## 这套循环覆盖什么 / 不覆盖什么

| 改动类型 | 覆盖? | 怎么生效 |
|---|---|---|
| 前端代码(`.vue`、组件、样式、表格列) | ✅ | Vite HMR,存盘即见(<1s) |
| 后端 Go 代码(handler、API 字段、查询) | ✅ | full 模式,重启 `go run`(几秒) |
| 后端字段已返回、前端只是没显示 | ✅ | 用 `--mode ui` 即可,纯前端 |
| Dockerfile / nginx 配置 | ❌ | 走真实部署(dev loop 绕过 nginx) |
| Helm chart / K8S manifests / Service 端口 | ❌ | 走真实部署(dev loop 绕过 Helm) |
| 新增 CRD / CRD schema 变更 | ❌ | `kubectl apply` 到集群 |
| 新增环境变量 / ConfigMap / RBAC | ❌ | 部署层,走真实部署 |

一句话:**跑在前端/后端进程里的代码 → 秒级覆盖;打包/部署/集群配置那一层 → 必须走真实部署。**

> 前端是自动 HMR;后端 `go run` **不会**自动重载,改完 Go 要重启。如需后端也"存盘即重启",
> 可接入 `air` / `reflex` 监听 `.go` 自动重编译(尚未默认接入,需要时再加)。

## Git 推送节奏(配合 CI/CD)

快速调试模式下虽然本地秒级迭代,但仍要**定期推送**,让 CI/CD 增量地跑、不积压:

- **至少每 2 小时** `git push` 一次。
- **代码改动大时,每 1 小时**推一次。
- 推送目标是**当前 feature 分支**(不是 `main`)。feature 分支上 push 只触发 CI 校验
  (`hack/ci-verify.sh` 等),**不会**自动部署;部署只在合并到 `main` 时发生
  (见 [.github/workflows/cd.yaml](.github/workflows/cd.yaml),ADR-042)。
- 这样 CI 始终在小步验证,合并到 main 时心里有底,CICD 流程可以安心跑。

> 说明:这是**工作会话内**的节奏约定 —— 在我(Claude)主导编码的会话里,
> 我会按上面的间隔在自然断点提交并推送。它不是后台定时任务;
> 若要按真实墙钟时间强制推送,需要额外配 git hook 或 scheduled task(需要时再加)。

## 退出

`Ctrl-C` 停掉本地循环。要发布时走正常通道:合并到 `main` → GitHub Actions 自动构建镜像并部署到 dev。
