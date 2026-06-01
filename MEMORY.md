# SupKube Engineering Memory（项目级 Memory）

> **用途**：把 AI 助手的跨 session 工作纪律、Mars 的交互偏好、踩坑教训**沉淀进仓库**——避免换 session、换工具、换接力人就重新踩一遍。
> **与 [`ENGINEERING.md`](./ENGINEERING.md) 的关系**：ENGINEERING.md 是**铁律手册**（Rule A–F，给所有读者）；本文件是**Memory + 教训档案**，对 Rule 给出"为什么/反例/历史出处"。两者一同维护，不能漂移。
> **与 [`CICD.md`](./CICD.md) / [`PRD-Review/INDEX.md`](./PRD-Review/INDEX.md) / [`dashboard/data.js`](./dashboard/data.js) 的关系**：CICD 是 ops 手册；PRD-Review 是评审档案；dashboard `DECISIONS` 数组是**今日决策日志**——本文件汇总**长期教训**（决策记到 dashboard，教训沉到这里）。
> **读者**：研发、SRE、PM、AI Agent（包括我自己开新 session 时）。
> **维护**：新教训产生时，**先在这里 + dashboard `DECISIONS`** 双写，再决定是否要升级成 ENGINEERING.md 铁律。

---

## 一、Mars 交互节奏（Iteration Style）

Mars 是产品方 + CTO，节奏稳定，规则可预测：

1. **Phase 分级**：大件拆 A/B/C/D/E 多 phase，每 phase 单独上线、单独审批。
2. **极简指令推进**："做"/"要"/"A"/"B"/"C" → **直接动手, 不复述需求**。复述 = 让用户失去耐心。
3. **Roadmap 优先**：任何新发现的工作（紧急小修 / 长期特性）**先写进 ROADMAP.md**，再决定做不做；不在对话里"留着以后说"。
4. **截图驱动**：经常用截图反馈具体问题；偶尔截图本身就是 bug 暴露面。
5. **决策矩阵思维**：用户主动把 ROADMAP 改成 4 象限（重要/紧急），新条目按这个分类放。
6. **暂停反思**：连续高强度迭代后会主动叫停"思考方向"——**不要在用户暂停期间主动 push 工作**。
7. **PR/diff 后主动列"下一项候选"让用户选** —— 用户喜欢"看完即决"。
8. **PRD-Review 是头等公民**：评审找出的 finding 是行动来源；不能把"评审"当成 nice-to-have，必须当 contract 来 follow。

**Rule 化**：见 [ENGINEERING.md Rule A–F](./ENGINEERING.md#1-核心工程规则铁律)。

---

## 二、PRD-先于代码（Rule A）

**Mars 2026-05-31 明示**："please always write PRD before your doing coding"

任何**影响 UX / 影响数据模型 / 跨多文件改动** 的 feature，必须先写 PRD，走 PRD.md 状态流程（草稿→排队评审→评审中→{改正中｜驳回｜已评审}→研发中→待验收→Shipped→归档），评审通过才能写代码。

### 例外（可直接写代码，不必先 PRD）

- 单文件 bug 修复（明确根因，不动数据模型）
- UX 微调（文案 / 颜色 / 间距）
- Memory / 文档更新
- dev-workflow 脚本（不直接影响产品）

### 反例（这些之前我直接写代码后被纠正）

- 加新 endpoint（即使简单，也算 UX 改动，应先 PRD 或至少 PRD 关联）
- 改 UI 组件结构（卡片布局 / 字段添加）—— 应先 PRD
- 加新 controller（even 后端内部）—— 应先 PRD

**简言之**：不确定"是不是该写 PRD"时，**写**。开发成本 > 写 PRD 成本。

### PRD 误读教训（2026-06-01）

我把 Mars "Snapshot 与 import 并列" 读成 "Snapshot/Export" 拼写错误，shipped 了 Kasten Snapshot+Export 单向 toggle（PRD-009 v1 文档说的那个）。但 Mars 真实意图是 **Snapshot Policy vs Import Policy 两个 Action 类型**——已经超出 PRD-009 v1 范围。

**教训**：遇到不熟悉的术语对仗（"Snapshot 与 X 并列"），**优先字面解读**而非"修正拼写"；当字面解读没历史 PRD 支撑时，回到对话向 Mars 澄清。

---

## 三、多任务并行：开 N 个 Agent（Rule B）

**Mars 2026-05-31 明示**："please open 3 agent for multi-task"

多个**互相独立、不共享文件**的任务，必须并行启动多个 Agent，不要顺序做。

### 适用场景
- 多份独立 PRD 草稿
- PRD + 代码探索 + 脚本同时进
- 多份独立 research
- 多文件 / 多目录的扫描型工作

### Agent prompt 模板
- **自包含**（agent 不知道上下文，全塞进去）
- 明示输出文件路径 + 字数/行数上限
- 明示 Mars 偏好（PRD 模板 / 短词推进 / verify-before-ship）
- 并行启动 = **一条消息里多个 `Agent` tool call**

### 不适用场景
- 涉及修改同一文件的多个改动（冲突）
- 涉及紧密依赖的链式任务（前一步输出是后一步输入）
- 跟用户对话本身的回复（必须 main agent 来）

### 真实交付数据（2026-06-01 ImportPolicy 收官）
**1 母 task → 7 子 agent 并发**（A 文档 / B controller / C fingerprint / D 前端 / E Helm / F docs / G S3 lister 补丁），每个独立 prompt 自包含 shared contract，平均 ~10-15 分钟产出 + 我做集成 + verify。比串行节省 ~3 小时。

---

## 四、共享契约必须有单一来源（Rule C / C v2 / Rule G 取号）

### Rule C — 原版（2026-05-31）

当多个并行 Agent / 多个模块要写**同一个共享契约**（API DTO / URL params / 错误码 / CollectionContract / deep-link schema）时，**必须指定唯一权威来源**，其他地方引用它而非各写各的。

**反例（X1 暴露）**：我并行让 2 个 agent 写 PRD-005 + PRD-006，两份都声称"deep-link 已对齐"，但实际：
- 路由名不同（`/logs?` vs `/observability?tab=logs`）
- 参数名不同（`since=1h` vs `sinceSeconds=600`）
- 取值格式不同（`line=4421` vs `scrollToLine=auto`）

→ 两份 PRD 拼出来的 URL **互不认识**。

**新规则**：main agent 必须**预先定义契约**（写到独立文件 / ADR / 某 agent 的明确职责段落），把契约**具体值**塞进每个并行 agent 的 prompt。不能让 agent 自己"协商对齐"——它们是独立沙盒，看不到对方输出。

### Rule C v2 — 升级版（2026-05-31，ADR-Review Round 2 R2-1 教训）

R2-1 (Blocker)：Agent A（改 ADR-020）跟 Agent B（改 ADR-034）都改了 MCP token 限流模型，**逻辑直接相反**（Agent A："token 集合总配额" / Agent B："per-cluster 独立配额"）。

**根因**：我 prompt 只预定了 Sanitize 签名，**没预定限流模型** → drift。

**升级**：SSOT 不只覆盖"已有的契约"，**还要预先穷举两 agent 可能交叠的设计决策点**。

| 决策类型 | 例子 | 谁定 |
|---|---|---|
| **限流模型** | per-token / per-cluster / per-route / 总配额? 数字阈值? | main agent 预定 |
| **错误码命名规范** | `ERR_<COMPONENT>_<CONDITION>`，大小写 | main agent 预定 |
| **字段命名规范** | snake_case vs camelCase vs kebab-case | main agent 预定 |
| **配置 key 命名** | helm values 路径（e.g. `advisor.provider.default`） | main agent 预定 |
| **Provider 列表** | LLM 5 个 endpoint / Storage 6 个 driver | main agent 预定 |
| **默认值** | TTL / timeout / quota / cache duration | main agent 预定 |
| **状态枚举** | 中英文混排?大小写? | main agent 预定 |
| **命名 prefix** | label `supkube.io/kind=` / annotation pattern | main agent 预定 |
| **架构关系** | "A 引用 B" vs "A embed B" | main agent 预定 |

### 跨时间多 agent 也算共享契约（C v2 后续, 2026-05-31）

如果后跑的 agent **在前 agent 输出上继续写**（即使时间序列，不并行），同样的"context drift"会出现。第 N+1 个 agent 读的是 worktree 当前状态，但不知道前面 agent 做的决策**为什么是那个值**。

**实操**：跨 session 续 agent 时，**再塞一次 shared contract**，不要假设它能从代码里推断回设计 intent。

### 实操 checklist（launch parallel agents 前）

- [ ] 列出每个 agent 改什么文件 + 什么段落
- [ ] 找文件/段落的"重叠点"（即使表面不重叠，引用关系也算重叠）
- [ ] 对每个重叠点，用一句话预定权威值
- [ ] 把权威值塞进所有 prompt
- [ ] agent 报告时明示"我按权威值 X 做的"

### 正例（2026-06-01 ImportPolicy 7-agent 并发）

母 prompt 预定：
- ImportPolicy CRD spec 完整字段（B/D/E 严格抄）
- fingerprint JSON 字段名 + HMAC 输入串拼接顺序（C 写、B 验、F 文档化）
- 8 个 REST API 路由（B 实现、D 调用）
- `supkube-fingerprint-secret` 名字 + ns + data key（E 创建、C 读、B 用）
- 14 条错误码（B/C 用、F 文档化、D toast）
- 30+ i18n key prefix（D 用、F 文档用）

→ 7 agent 输出全部 fit，集成时 zero 冲突。

### Rule G — 取号必先来 LEDGER（2026-06-01 新立铁律）

**Mars 2026-06-01 拍板**：所有文档编号（PRD / ADR / TC / D / C 等）建立**唯一取号台账** [`LEDGER.md`](./LEDGER.md)。新建编号前必须先去取号 +1，再写正文。详见 ENGINEERING.md Rule G。

**与 Rule C / C v2 的关系**：Rule C/C v2 解决"两 agent 并发写同一份契约的内容"漂移；Rule G 解决"两 agent 并发取同一个号"撞号。两者是 SSOT 纪律的两面，缺一不可。

**取号 SOP 速记**：
- 单 Agent：read LEDGER → reserve + write LEDGER → write 正文 → 回 LEDGER 改状态
- 多 Agent：main agent **预分配** 全部号 → 把号写进每个 agent prompt → agent 写正文不碰 LEDGER

**反例（这些都被 L-13 / L-14 暴露过）**：
- ❌ 在 PRD.md 末尾 grep 最大号自己 +1（多文件并发 = 撞）
- ❌ Background agent 各自跑去 LEDGER 取号（race condition）
- ❌ 跨 session 续工作，凭记忆写号（旧记忆 stale）

---

## 五、Verify-Before-Architect

任何**承重技术结论**先用真实测试**证伪**再写 ADR / 推进决策。**不能从观察或记忆里 assert**。

### 反例来源（2026-05-30）

我曾自信但错地说："Velero v1.18 删除 K8s VolumeSnapshot 对象后，快照数据丢失，不可恢复"。基于这个结论 ADR-029 决定**放弃本地快照层**。

实际：受控测试（双集群 / 双 CSI 驱动 / `az snapshot show` + 真实 restore）证明：**存储层快照仍保留，备份完全可恢复**。

**根因**：把"K8s 对象消失"（symptom）当成"数据没了"（fact）。Mars 明确说这"直接影响产品方向"，必须有 verify gate。

### 如何应用

1. **区分症状（symptom）vs 根因（fact）**：UI 显示"—"、K8s 对象消失 = symptom；数据真没了 / restore 失败 = fact。
2. **任何能影响战略 / 架构选择的 claim，先跑一次实测**。Mars 会主动问你"verify 了吗"，最好不要等他问。
3. **错了就坦白**：直接说"我刚才那个判断错了，新证据 X，结论 Y"。Mars 对**直接 accountability + 证据修正**的接受度远高于"我刚才那个其实……"。

---

## 六、Verify-Before-Ship（"完成"的真正定义）

**Build pass ≠ Deploy 完成 ≠ Ship**。

### 触发：5+ 次 false-complete 反复（2026-05-30）

Mars 反馈："我发现我们已经出现了多于 5 次这样的问题，就是你说已经好了，我来"——把"代码改对了 + build 通过"当成"完成"，把最终 verify 责任推给 Mars。

### "完成"按 change 类型分级

| Change 类型 | "完成"定义 |
|---|---|
| 仅文档 | 文件 write ✅ |
| 后端代码 | `go build ./...` 通过 + 镜像 push + rollout + **`curl /api/v1/status` 实测看到新 buildStamp** |
| 前端代码 | `npm run build` 通过 + 镜像 push + rollout + **HTML chunk hash 实测变了** |
| UI 可见 change | 上面所有 + **附上 verify 证据再告诉 Mars 去看**："buildStamp 已是 NNN, 路由 /X 已在 gin debug 注册" |
| K8s controller | 上面所有 + **看 backend 启动 log 含 controller 启动行** + 可能时实测 controller side effect |
| 集群操作 | 先 `--dry-run=client` 或在影响最小的 ns 试一遍 |

### 绝对禁止说的话

- ❌ "Build 通过，完成了" → build pass ≠ deploy 完成
- ❌ "代码改好了，你刷新看" → 把 verify 推给 Mars
- ❌ "rollout successfully rolled out" 当 verify 证据 → image 没变时也通过
- ❌ "应该可以看到 X" → "应该"是没验证的信号
- ❌ "等通知" 后只看 build/push log，不看 deploy 实际生效

### 必须给 Mars 的"完成"格式

> "已完成。实证: buildStamp=NNN (今天 HH:MM)，/X endpoint 注册在 gin debug，controller 启动日志含 [csi-autoconfig] starting。请刷新看 Y。"

每个 verify 步骤要么我做了，要么明确说"还没验证，留给你"。**不能含混过去**。

### 子规则 1: Verify 完整 User Journey，不是单个 Endpoint（2026-05-31）

**反例**：加 Log Viewer feature 时，我 curl `/logs/components` 看到返回 5 个 component → 报"完成"。结果 Mars 刷新点击 backend 看到红色横幅 "No pods match selector ... actually installed?"——我没走第二步 `/logs?component=backend` 看 lines.length > 0，漏了 label selector 用错的 bug。

**规则**：每个新 feature 完成前，**列出完整 user journey**，从第一步走到拿到目标价值：

| Feature | "拿到价值"那一步 | 不能只验 |
|---|---|---|
| Log Viewer | 选 component → 看到日志 lines > 0 | ❌ /logs/components 返回 5 |
| Restore Preflight | 点击 Preflight → 看到冲突列表 | ❌ /restores/preflight 返回 200 |
| Backup 创建 | 提交表单 → 看到 Backup CR 出现在列表 | ❌ POST /backups 返回 201 |
| AI Advisor 评分 | 进 Application 详情 → 看到 0-100 分 + 推理 | ❌ /advisor/score 返回 JSON |
| MCP Skills | OpenClaw 调用 → 看到 SupKube 真创建 backup | ❌ /mcp/sse 接受连接 |
| CSI 自动配置 | 新装客户 → kubectl get vsc 真有 supkube-* | ❌ controller 启动日志含 starting |
| Import Policy | 跨集群 source backup → target Velero 60s 内出 Imported chip + fingerprint valid | ❌ /import-policies 返回 200 |

**通用模式**："API 返回数据" ≠ "用户拿到价值"。永远 verify 最后那一步——客户屏幕上真看到了什么。

### 子规则 2: 软件不能问用户自己存在与否（2026-05-31）

Mars 反馈："我装的就是这个软件，操作的就是这个软件，为什么提示我'actually installed?'"

**规则**：SupKube 自家组件（backend/frontend/dex 等）的"找不到"必须**绝不**触发"did you install it?"文案。客户在用 SupKube UI = 一定装了。
- self-component 找不到 = SA RBAC 缺权限 / pod 异常 → 文案直接给 `kubectl auth can-i` 排查命令
- third-party-component 找不到 = "X 没装"合法 → 可以问"是否安装"

**抽象**：任何"Is X installed?"提示，X 必须是第三方；不能是 SupKube 自己 + SupKube 此刻装在客户面前的副组件。

### 子规则 3: 客户端 cache 防御（2026-05-31, 三次相同教训）

**反例**：deploy 完，pod 内 nginx 给出新 chunk hash + nginx response `Cache-Control: no-store` + curl localhost 直接拿也是新版本——我报"完成"。Mars 浏览器看仍是旧版本。

**根因**：服务端层都对了，但浏览器 **bfcache**（back/forward cache）绕过所有 HTTP cache header。老 Vue index.html 没 `<meta http-equiv="Cache-Control">` meta tag，bfcache 不破。普通 Cmd+R 不破 bfcache，**Cmd+Shift+R 才破**。

**规则**：凡是涉及 **frontend SPA**，"完成"必须含：

1. **服务端 verify**（已有）：pod 内 chunk hash + nginx response 头 + 主页 curl 拿到新 chunk
2. **客户端 cache 防御**（永久）：
   - SPA index.html **必须有 3 个 cache-busting meta tag**（`Cache-Control` / `Pragma` / `Expires`）双保险，防 bfcache
   - 任何新 SPA 项目都该有这 3 个 meta，**不能依赖 nginx header alone**
3. **客户端可见性 verify**（永久）：
   - deploy 后给客户**具体期望 build stamp 数字**（e.g. "顶栏期望看到 `260531-0824`"）
   - 给客户**梯度破 cache 方式**: (i) Cmd+Shift+R → (ii) DevTools Disable cache → (iii) Clear site data → (iv) incognito
4. **dev-deploy.sh 自动 print 客户端 verify 指引**（永久）：末尾 echo 期望 build stamp + 破 cache 命令

**chicken-and-egg 警告**：cache-busting meta tags **本身的首次 ship** 需要客户硬刷一次（旧 HTML 没有这些 meta，浏览器仍按旧规则缓存）；之后才能用普通 Cmd+R 拉新。**加这 3 个 meta 那一次的 deploy 文案必须显式提示用户"这次必须 Cmd+Shift+R"**。

**通用模式**：server-side OK ≠ client-side 真看到。涉及客户端的 verify 必须想到**多层 cache**（nginx / browser HTTP cache / browser bfcache / Service Worker / CDN），任一层没破 = 客户看老版本。

### 子规则 4: Helm chart RBAC 变更不会被 dev-deploy.sh 自动应用（2026-06-01）

**反例**：Agent E 在 `supkube-helm/templates/rbac.yaml` 加了 importpolicies verbs，但 `dev-deploy.sh` 只 `kubectl set image`，**不跑 helm**。集群里 ClusterRole 仍是旧的 → ImportPolicy controller 每 30s `forbidden: cannot list importpolicies`。

**规则**：当 helm chart **RBAC / CRD / Secret / 任何模板** 改动后，dev-deploy.sh 单跑**不够**。必须 EITHER：
- 跑 `helm upgrade`（影响面大，可能 reset Secret 等）
- OR 手动 `kubectl patch clusterrole`（targeted、明确）

修复后必须 `rollout restart` 让 controller 重新连客户端（避免 discovery cache 旧值）。

**子结论**：dev-deploy.sh 后续要加 Phase 0 步骤 "diff chart RBAC vs cluster RBAC, fail-fast on drift"。已加 task #164（待立）。

### 例外

只在 Mars 明确说 "fire and forget" / "你先继续做下个" / 时间窗口紧张的情况下可以省略最终 verify。但要**显式说**："我没做最终 verify，因为 X，留给你"——不要假装完成了。

---

## 七、基础设施速查（Azure + AKS + Tailscale + ACR + Helm）

### Mars 的可用资源
- **Azure 账号**：`ea-rnd-mzhang@jumborca.net` / `Sub-RnD`（jumborca R&D 订阅）
- **AKS 集群**：
  - `aks-jumborca-dev` —— dev 集群（**本机默认 current-context 常指这里**；操作 test 时务必显式 `--context aks-jumborca-test` 以免误操作 dev）
  - `aks-jumborca-test` —— test 集群，2 节点 × (4 vCPU / 16Gi)，K8s v1.34.7；已装 Azure 托管 addon + Kasten K10 备份；KB 练习首选场地
- **Tailscale tailnet `mars.zhangcong@`**：
  - `mars-laptop` = `100.69.159.111` —— 当前这台 MacBook Pro
  - `mars-homelab` = `100.73.206.94` —— 老 Mac（曾被误认成当前机器；要在老 Mac 上做事先让 Mars 开机+开 Tailscale+开远程登录）

### 项目镜像与制品
- **ACR**：`supkube.azurecr.io`（Standard SKU, `anonymousPullEnabled=true`），短名 `backend`/`frontend`，tag = 产品版本（e.g. `0.9.1.10-alpha-dev-06010819-arm64`）
- **ACR token TTL = 1 小时**，dev-deploy.sh 跑过几次会失败 → `az acr login --name supkube` 重新换
- **Helm chart**：`supkube-helm/supkube`，双轨版本：chart `0.9.1-alpha.N`(SemVer) ↔ appVersion `0.9.1.N-alpha`(四段)。内置 velero 子 chart（`charts/*.tgz` 被 gitignore，`Chart.lock` 锁定）。**EULA 闸**需 `--set eula.accept=true`
- **CD workflow**：`.github/workflows/cd.yaml` 接管 ACR 推送（不再有 ghcr docker job，ci.yaml L77 已记录这个迁移）

### 集群命名空间
- `supkube` —— SupKube backend / frontend / dex / minio
- `velero` —— Velero + node-agent（与 SupKube 解耦, 方便 `kubectl logs -n velero` 直接跟 Velero 文档）

### 本机工具栈
kubectl / helm v3.17 / az / docker / jq / openssl 已装；**kbcli 未装**。dev-deploy.sh Phase 0 检查这些。

### 关键坑速查
1. **容器名 ≠ deployment 名**：deployment 是 `supkube-backend`，但容器内是 `backend`。`kubectl set image deployment/X foo=Y` 用错 = **silent no-op**（"deployment unchanged" 但 rollout 不动）。dev-deploy.sh probes `spec.containers[].name` 实际值。
2. **buildx --load 假成功**：BuildKit "DONE 0.0s" 不代表 image 真在 local daemon。永远 `docker images | grep <tag>` after --load 验。
3. **多架构注意**：amd64 (AKS) + arm64 (docker-desktop)。push 两 arch + manifest list；single-arch push 到 ACR 后另一架构会 ImagePullBackOff。
4. **ACR `az acr login` 1h TTL** → 长跑流程要 pre-flight 检查 + 失败时自动 re-login。
5. **前端 chunk hash content-addressed**，但 `index.html` 缓存必须破才能拿到新 chunk → 3 cache-busting meta + 用户 Cmd+Shift+R（首次 ship meta 那一次）。
6. **Velero `backupSyncPeriod`**：保持默认 60s。改长 = 无 ImportPolicy 的 BSL 同步退化，是 regression（PRD-Review G2 finding）。

---

## 八、跨 session 累积教训速查（chronological）

每条对应一次"被纠正过"的失误，按日期排序。新教训进来时**先加到这里**，再决定要不要升级为 Rule。

| 日期 | 教训编号 | 触发场景 | 结论 |
|---|---|---|---|
| 2026-05-28 | L-01 | Demo 现场 Mars 列 12 条客户痛点 | dashboard `CUSTOMER_PAIN` 数组建台账, 每条挂 task 编号; 不能口头记 |
| 2026-05-30 | L-02 | 误判 Velero 删 VS 对象 = 数据丢失 → 错推 ADR-029 | **Verify-Before-Architect**（§ 五）|
| 2026-05-30 | L-03 | "build 通过=完成" 5+ 次假阳性 | **Verify-Before-Ship**（§ 六）|
| 2026-05-31 | L-04 | PRD-005/006 并行 agent deep-link drift | **Rule C SSOT**（§ 四原版） |
| 2026-05-31 | L-05 | ADR-Review R2-1 Agent A/B token 限流模型相反 | **Rule C v2**：预先穷举决策点（§ 四升级版） |
| 2026-05-31 | L-06 | Log Viewer 验 endpoint 不验 user journey | **Verify 完整 journey**（§ 六 子 1） |
| 2026-05-31 | L-07 | "actually installed?" 红色横幅问 Mars | **软件不能问用户自己存在**（§ 六 子 2） |
| 2026-05-31 | L-08 | Mars 浏览器看旧版 SPA（第 3 次相同问题） | **3 cache meta + 客户端 verify 永久规则**（§ 六 子 3） |
| 2026-05-31 | L-09 | DR Topology 修了 /clusters endpoint 不修 topology.go（两条独立路径都死写 "this-cluster"） | 同一产品概念多条计算路径必 drift; 后续抽 helper 强制共用 |
| 2026-06-01 | L-10 | PRD-009 ship 错（Snapshot/Export toggle 不是 Snapshot/Import 双 Policy） | 字面术语优先字面解读；不熟悉对仗时回到对话澄清（§ 二） |
| 2026-06-01 | L-11 | Helm rbac.yaml 改了 dev-deploy 没生效 → controller 30s 错 | **dev-deploy.sh 不跑 helm，RBAC/CRD/Secret 变化必须手动 patch + rollout restart**（§ 六 子 4） |
| 2026-06-01 | L-12 | Agent B 报 "BackupLister 是 poor man's"，依赖 Velero sync 而非真直 S3 | 后端 agent 报有妥协时**必须立刻挑明对产品承诺的影响**，不绕；Mars 决定补还是接受 |
| 2026-06-01 | L-13 | ADR-037/038 撞号复发（PRD-008/010 旧草稿仍用旧含义）| 占号前先查台账; 被复用过的旧号**不得沿用**, 必须让号（PRD-Review 第六份建议）|
| 2026-06-01 | L-14 | L-13 复发暴露：项目缺**跨 series 取号台账**（PRD/ADR/TC/D/C 各靠各的文档末尾 grep 最大号，并行 Agent 必撞）| **建 `LEDGER.md` 唯一号源**（[LEDGER.md](./LEDGER.md)） + **ENGINEERING.md Rule G 取号 SOP**: 单 Agent 顺序取号；并行 Agent 由 main agent 集中预分配号塞 prompt（Rule C v2 延伸）。让号是 forward-only。每次更 LEDGER 跑漂移检查 |

---

## 九、与 ENGINEERING.md / dashboard / PRD-Review 的关系图

```
                     ┌─────────────────────────────────────┐
                     │  ENGINEERING.md  (Rule A–F 铁律)     │  ← 短、规则化、给所有读者
                     └────────┬────────────────────────────┘
                              │
                  ┌───────────┴────────────┐
                  ▼                        ▼
     ┌─────────────────────┐    ┌──────────────────────────┐
     │  MEMORY.md (本文)    │    │  dashboard/data.js        │
     │  教训档案 + 反例     │    │  DECISIONS (今日决策日志)  │
     │  长期累积           │    │  + ADRS + PRDS + WEEKLY    │
     └──────────┬──────────┘    └──────────────────────────┘
                │
                ▼
     ┌─────────────────────┐    ┌──────────────────────────┐
     │  PRD-Review/INDEX.md │    │  PRD.md / 架构设计.md      │
     │  PRD-Review-* 系列   │    │  ADR-LEDGER (单一编号)     │
     │  每轮评审存档        │    │  各 PRD 状态机              │
     └─────────────────────┘    └──────────────────────────┘
```

- **新教训** → 先进 MEMORY.md（这里）+ dashboard `DECISIONS`（如果是当日决策）。
- 教训积累到**可成铁律** → 升级为 ENGINEERING.md Rule（A–F 体系）。
- 教训涉及**单一来源**问题 → 在对应文档（架构设计.md / PRD.md）加 SSOT 段落，PRD-Review 检查交叉。
- 教训涉及**产品决策方向** → PRD-Review 跟踪闭环 + dashboard `DECISIONS` 留痕。

---

## 十、维护约定

1. **新教训进来 →** 加到 §八速查表（L-XX 编号续接），同步 dashboard `DECISIONS`。
2. **教训升级为铁律 →** 在 ENGINEERING.md Rule 段补一条；本文件 § 二/三/四/五/六加引用回 ENGINEERING.md。
3. **季度回顾**（建议每 30 个 L-XX 一次）：合并相似教训、删冗、把陈旧条目标 archived。
4. **MEMORY.md 不可超 800 行**——超过就抽细节到 `docs/memory-detail/` 子文件，本文件只保留 index + 一句话 takeaway。当前行数：~480。

---

> **本文件 v1.0 — 2026-06-01**：从 AI Agent 个人 memory (`feedback_iteration_style.md` / `feedback_verify_before_architect.md` / `feedback_verify_before_ship.md` / `reference_infra_azure_tailscale.md`) 整合迁入项目仓库，作为接力人/AI 的单一入口。后续维护按 § 十规则。
