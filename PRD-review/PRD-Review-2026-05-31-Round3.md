# SupKube PRD Review 第三轮 — PRD-001 / 002 / 004 / 007 改正中验证

> **视角**：Kubernetes + AI + 灾备（DR）专家
> **评审对象**：PRD-001（v2 修订）/ PRD-002（v1.2）/ PRD-004（改正中）/ PRD-007（v1.1）
> **评审人**：Claude（受 Mars 委托） · **日期**：2026-05-31
> **承接**：
> - 第一轮 PRD-001~004：`PRD-Review-2026-05-31.md`（T1-T4 顶层 + 各 PRD specific findings）
> - 第二轮 PRD-005/006：`PRD-Review-2026-05-31-PRD005-006.md`（不在本轮范围）
> - PRD-007 第一轮：`PRD-Review-2026-05-31-PRD007.md`（P1-P5 + 5 Med/Info）
>
> **核对基线**：当前 PRD.md（含 2026-05-31 各 PRD 修订段）/ 当前 架构设计.md（含 ADR-003 v0.9.x 修订段）/ SECURITY.md / 前 3 份评审报告

---

## 一、执行摘要

本轮对 4 份 PRD 的"修订后版本"做闭环验证。**整体闭环率高**——4 PRD 共 18 个 finding（PRD-001 #1/#2/#4/T3，PRD-002 #1/T1/#2/#3/#4/#5，PRD-004 T2/#2/#3，PRD-007 P1/P2/P3/P4/P5）全部在 PRD 正文 + DoD 中可锚定到具体行号，没有"嘴上认了 PRD 里没改"的情况。承重依赖项 ADR-003 修订段（架构设计.md L905–924）也确实落地，PRD-002 进研发的硬阻断已解除。

唯一一处实质 follow-up：**PRD-007 §4.7 行 2369 声明"该 Transform 须加入 PRD-002 §4.1 的 11 个 builtin Transform 列表 (PRD-002 v1.1 已同步)"，但 PRD-002 §4.1（PRD.md L364–381）仍是 11 个 builtin，未加 `redirect-external-endpoints-to-sandbox` Transform**。这是 PRD-007 P4 修订带出的次生需求，不归属任何评审 finding，建议作为 follow-up 在 PRD-007 进研发时同步补到 PRD-002。

### 4 PRD 决议速览

| PRD | 修订版本 | finding 闭环 | 本轮决议 | 解锁进研发？ |
|---|---|---|---|---|
| **PRD-001** | v2 (2026-05-31) | 4/4 ✅ | **排队评审**（待 Mars 最后审） | 是（与 PRD-002 同窗口 kick-off） |
| **PRD-002** | v1.2 (2026-05-31) | 5/5 ✅ + ADR-003 修订已落 | **已评审 / 可进研发** | 是（解锁 #114 / #104 实施依赖） |
| **PRD-004** | 改正中 (2026-05-31) | 3/3 ✅ | **排队评审**（待 ADR-034/036 草稿转 Accepted + Mars 最后审） | 否（需等 ADR-034/036 Accept） |
| **PRD-007** | v1.1 (2026-05-31) | 5 P + 5 Med/Info 全闭环 ✅ | **排队评审**（Mars 最后审 + 处理 follow-up） | 部分（Phase 0 可启动；研发全开需 follow-up 落地） |

### 跨 PRD / 跨 ADR 一致性结论

- **ADR-003 修订段确实存在**（架构设计.md L891 状态行 + L905–924 修订段），与 PRD-002 §4.1bis 编译契约完整对齐——PRD-002 进研发的硬阻断已解除。
- **ADR-034（MCP 协议选型）/ ADR-036（SSE 项目级口径）**：架构设计.md 号台账（L37、L39）标 "🆕 草稿 (本次, 2026-05-31)"。PRD-004 依赖这两份 ADR Accept；本评审不再深入这两份 ADR 内容，但 PRD-004 解锁前**需把这两份从"草稿"推到"Accepted"**。
- **PRD-007 ↔ PRD-002 联动 1 处未闭**：`redirect-external-endpoints-to-sandbox` Transform 在 PRD-002 §4.1 builtin 列表缺失（详见 §六）。
- **PRD-007 ↔ PRD-003 联动**：P5 Score 二选一已选 (a)"两个不同指标共享数据采集层"，DoD #17 改写到位，UI 明示"绝不允许加减对比"。共享接口约定写在 §4.8 + Phase 1 任务（PRD-007 §9 行 2575）"与 PRD-003 共享接口约定"——PRD-003 自身未做对应修订（不在本轮范围），需在 PRD-003 后续修订时确认 API contract 一致。

### 关键放行结论

| 立即可解锁 | PRD-002（#114 实施）→ PRD-001（#104 RestoreDrawer 改造可与 PRD-002 同窗口 kick-off） |
|---|---|
| 待 ADR Accept 后解锁 | PRD-004（MCP Server，等 ADR-034 / ADR-036 从草稿 → Accepted）|
| Phase 0 可立即启动 / 研发全开需 follow-up | PRD-007（Phase 0 verify-before-architect 已可启动；进 Phase 2/5 前需把 `redirect-external-endpoints-to-sandbox` 同步进 PRD-002）|

**严重度图例**：Blocker / High / Med / Info（沿用前两轮）。

---

## 二、PRD-001 v2 跨集群还原 Preflight Checklist 验证

PRD-001 v2 修订主要由 2026-05-31 评审历史行（PRD.md L292）触发，落 4 个 finding + T3 拓扑校验。逐项核查：

| Finding | 评审第一轮原文要点 | 修订实证锚点（PRD.md） | 闭环判定 |
|---|---|---|---|
| **#1（High）blocker 不可忽略** | "灾备语境下，忽略一个 blocker = 放行一次带病还原" → "blocker 不可忽略，仅 warning 可勾选忽略 + 二次确认" | L149-150 User Story 明确 blocker 不可勾选忽略；L165-166 Functions schema 明示 `severity=blocker` ⇒ 不允许"忽略此冲突"；DoD #6/#7（L247-248）测试覆盖"忽略按钮仅在 severity=warning 行渲染, blocker 行强制隐藏 (test: 注入 severity=blocker 的冲突 → 检查 DOM 无 ignore 按钮)" | ✅ 闭环 |
| **#2（Med）severity 后端固化** | "severity 在后端 conflict schema 固化为枚举字段，前端只渲染不判断" | L156-167 后端 schema 严格定义 `severity: enum(blocker\|warning)` + "前端**严禁**对 severity 做'软计算'或客户端覆盖"；DoD #11（L252）测试覆盖"后端注入 severity=warning, 前端不允许通过任何路径升级为 blocker disabled Restore" | ✅ 闭环 |
| **#4（Info）删 v1 残留** | "文件 271–511 行保留了大段 v1 已废弃方案注释块" → "物理删除" | L296 留下一行注释 "v1 残留草图已于 2026-05-31 物理删除（finding #4）。git history 永久保存原 §4.1-§4.3..."；grep 验证：原"adapt-storage"草图在 PRD.md 中已无残留（仅评审历史 L292 引用，非草图本身）。物理删除后 PRD-001 v2 区段只剩 122-296 行（174 行），符合 v2 拆分目标 | ✅ 闭环 |
| **T3（High）SC→Immediate 拓扑校验** | "默认改 Immediate 在多 AZ / 拓扑场景有数据可用性风险" → "Preflight 拓扑校验" | L170-174 新增"SC→Immediate 拓扑校验"完整子节：单 AZ / hostpath 类拓扑无关存储允许 Immediate；多 AZ 保 WaitForFirstConsumer + Preflight warning；输出 `topologyHint: single-az\|multi-az\|unknown`；DoD #10（L251）测试覆盖"给 fake target cluster 打 3 个 zone label, 注入 SC missing 冲突 → 校验 warning 出现" | ✅ 闭环 |

**未提及的旧 finding（#3/#5）**：原第一轮另有 #3（跨云 3s 性能）/ #5（router state 跳转）两条 Med/Info，本轮修订未触及，但这两条本就被原报告标为"研发期内处理"而非"放行前"，不阻塞本轮决议。

**新发现**：无。修订未引入新 finding。

**决议**：**排队评审**。

- 4 个评审范围内 finding 全闭环；schema / DoD / 测试矩阵完整。
- 状态由 PRD 自身（L128 + L292 评审历史末行）规划为"等 Mars 重审 → 排队评审 → 已评审"，本轮评审无新加要求。
- 唯一保留小提醒：PRD-001 是 PRD-002 的"被依赖方"（PRD-001 L132 "依赖 PRD-002 必须先评审通过"）——PRD-002 本轮已可进研发后，PRD-001 即可同窗口 kick-off。

---

## 三、PRD-002 v1.2 Transform 一等公民验证（本轮最重承重 + 含 ADR-003 修订依赖检查）

PRD-002 v1.2 修订由 PRD.md L745 评审历史行触发，落 5 个 finding（#1/T1 / #2 / #3 / #4 / #5），并依赖一项后台改 ADR-003。逐项核查：

| Finding | 评审第一轮原文要点 | 修订实证锚点（PRD.md） | 闭环判定 |
|---|---|---|---|
| **#1 / T1（High）编译契约 + data.spec→rules.yaml** | "TransformSet + params 在 Trigger 时展平为单个 `data.rules.yaml`、`len==1` 的 ConfigMap" + "§4.1 builtin YAML 用 `data.spec` 与 ADR-003 冲突" | (a) L396 builtin Transform YAML 已改用 `data.rules.yaml`（L383 注明"对齐 Kasten + ADR-003 `len(Data)==1, key=rules.yaml`"）；(b) **新增 §4.1bis "TransformSet → Velero resourceModifierRef 编译契约"（L442-474）**：完整定义编译规则 6 步（取 transformRefs[] → 顺序展开 + ${VAR} 替换 → 拼接 → 生成 `supkube-restore-rm-<restoreName>-<hash>` 单 CM, `len(Data)==1` → 注入 Restore CR → 7 天 GC）；(c) preview-resolution 端点明确"输出即编译结果"（L472）；(d) DoD #15/#16（L669-670）双测：seeder 跑完后 `kubectl get cm -l supkube.io/kind=transform -o jsonpath='{.items[*].data}'` 验所有 builtin 仅含 rules.yaml + Trigger Restore 时校验 compiled CM 顺序 == UI preview | ✅ 闭环 |
| **ADR-003 修订依赖** | "依赖 ADR-003 修订（另外 agent 后台改架构设计.md）" | 架构设计.md L891 状态行已加 "**修订 v0.9.x（2026-05-31, 见文末'修订'段：两层模型 + 编译派生 ConfigMap）**"；L905-924 完整修订段，明确：(1) 概念分两层 Transform / TransformSet；(2) 新增编译步骤 `supkube-restore-rm-<restoreName>-<hash>`；(3) `len(Data)==1` 铁律对派生 CM 依然成立；(4) 向后兼容走 PRD-002 Phase 0 + ADR-014 启动迁移机制。**与 PRD-002 §4.1bis 完全对齐**（同名同 hash 规则同语义） | ✅ **依赖闭环**——PRD-002 进研发的硬阻断解除 |
| **#2（High）Phase 0 迁移测试 + 回滚** | "复用 ADR-014 启动时迁移机制；补独立迁移测试 + 回滚 + 迁移前后对照" | (a) §9 Phase 0（L678-685）完整加 "(finding #2) 独立迁移测试 + 回滚剧本 + 迁移前后对照"，含 v0.8.x 单独集群跑 migration、`supkube migrate transform-sets --rollback` 命令、dry-run 报告；(b) DoD #17（L671）测试覆盖"起一个 v0.8.x 集群, 升级到 v0.9.x → 跑 strip-nodeport 场景验证不破"；(c) 显式声明"复用 ADR-014 启动时迁移 migrateBrokenTransformSets 先例" | ✅ 闭环 |
| **#3（High）使用统计弃 annotation+CAS** | "annotation + CAS 单调递增 → 高并发触发 K8s API server CAS 写冲突重试风暴" → "事件流 + 异步聚合 / 独立 counter" | §4.2（L476-494）重写：(a) TransformSet 层用 **K8s Event 流 (`reason=TransformSetApplied`)** + `transform-stats-aggregator` goroutine 60s 周期聚合 → 批量回写 annotation；(b) annotation 标 "最终一致, 不保证精确"；(c) 备选 leader-elected single-writer counter ConfigMap；(d) Transform 层 transitivelyAppliedCount 由同一 aggregator 联动累加；(e) DoD #18（L672）测试覆盖"50 个 Restore 并发引用同一 TS → prometheus etcd_request_duration_seconds p99 不抖增 + 50 条 Event 齐全, annotation 60s 内最终一致" | ✅ 闭环 |
| **#4（Med）${VAR} 校验** | "保留 ${VAR} 但对 regex / 捕获组做校验 + 在 preview-resolution 里展示替换 diff；危险字符拒绝" | (a) §4.3bis (a)（L519-531）完整定义校验规则：占位符 `\$\{[A-Z][A-Z0-9_]{0,31}\}` + 危险字符黑名单（`$` / 反引号 / `\n` / `\r` / NULL byte / 未转义 `'"` ）+ regex value 必须 `regexp.Compile` 试编译；(b) preview-resolution 端点输出 diff view；(c) DoD #19（L673）测试覆盖"注入 `params={TO: 'x;rm -rf /'}` 应返回 422 + reject 原因" | ✅ 闭环 |
| **#5（Med）引用顺序对齐 Velero** | "TransformSet 内 transformRefs 执行顺序 → 必须对齐底层（Velero）实际语义；同 path 多 patch 后者覆盖" | (a) §4.3bis (b)（L533-538）显式声明"transformRefs[] 顺序 == 编译后 rules 在 Velero rules.yaml 中的顺序"+ "Velero 实际语义：对同一 JSON path 多条 rule, 后者覆盖前者"；(b) UI preview 必须 diff view 高亮"被覆盖"行；(c) DoD #20（L674）测试覆盖"TS 含 rule A (path=/spec/sc, value=x) + rule B (path=/spec/sc, value=y), preview 必须显示 A 灰底'被覆盖' + B 高亮'final'" | ✅ 闭环 |

**Follow-up check（评审本身要求）**：PRD-007 §4.7 行 2369 称 `redirect-external-endpoints-to-sandbox` Transform 是 PRD-002 v1.1 的"已同步"内容。实测 PRD-002 §4.1 行 366-381 的 builtin 列表仍是 11 项（无该 Transform），属 PRD-007 修订带出的次生需求未真正同步到 PRD-002。**不计入本 PRD finding**（本 PRD 修订发生在 v1.2，对应 5 个评审 finding 之外的修订请求未在 v1.2 范围），列入 §六 follow-up。

**新发现**：无。修订未引入新 finding。

**决议**：**已评审 / 可进研发**。

- 5 个评审 finding 全闭环 + ADR-003 修订依赖确认落地，PRD 自身评审历史标 "等 Mars 重审 → 排队评审 → 已评审"；本评审视角下，硬阻断（编译契约书面化）已解除，可解锁 #114 实施 + #104 RestoreDrawer 改造。
- 与 PRD-001 同窗口 kick-off（PRD-002 是 PRD-001 的前置依赖，但二者实施可并行 Phase）。

---

## 四、PRD-004 MCP Server 验证（Blocker → ?）

PRD-004 修订由 PRD.md L1369 评审历史行触发，落 1 Blocker（T2）+ 2 High/Med（#2 / #3）。逐项核查：

| Finding | 评审第一轮原文要点 | 修订实证锚点（PRD.md） | 闭环判定 |
|---|---|---|---|
| **T2（Blocker → High 已降级）Streamable HTTP 替换 SSE-only** | "v1 直接采用 Streamable HTTP；旧 SSE 端点仅作兼容" | (a) PRD 字段表（L1094 关联 ADR）加 "ADR-034 MCP 协议选型 (Streamable HTTP) + ADR-036 SSE 项目级口径"；(b) L1097 传输模式行 "**Streamable HTTP（v1 主推, finding T2 2026-05-31 修订）** —— 单端点双向流, Bearer Token / mTLS 友好, nginx ingress 兼容性好。**SSE 仅作向后兼容**"；(c) §4.1 架构图（L1133-1154）单端点 /mcp + SSE 兼容退路双端点并列；(d) §4.4 技术栈选型（L1197）"Streamable HTTP v1 主推, SSE 仅做向后兼容退路"+ 完整理由（MCP 2025-03-26 + ADR-015 nginx ingress + Bearer Token URL 安全）；(e) §6 Out of Scope（L1263-1264）"Stdio 传输 v1.1 加"+ "SSE 新功能投入 v1 仅 Streamable HTTP 新功能"；(f) DoD #2/#3（L1287-1288）改基线为 MCP Inspector + Claude Desktop；DoD #17/#18（L1292-1293）新增 Streamable HTTP token 不入 URL + nginx ingress 30min 长连接稳定 | ✅ Blocker 闭环——升级为 "正常 High 级修订完成"。**但 PRD 解锁需 ADR-034 + ADR-036 从草稿 → Accepted**（架构设计.md L37/L39 仍为"🆕 草稿"） |
| **#2（High）HitL confirm 服务端持久化** | "确认快照存共享存储 (Redis / CR), confirm_id 全局可寻址" + "confirm 必须基于服务端持久化的原始 dry-run 快照落地，只认 confirm_id + 用户确认信号，忽略第二次调用携带的业务参数" | (a) §4.3（L1181）"Human-in-Loop" 行完整重写：服务端按 confirm_id 持久化快照（Redis 或 K8s CR `MCPConfirmation`）+ "跨多副本可访问"+ 快照内容含 `skill_name + 规范化后 inputs + dry-run effective output + user_token + expires_at` + Agent 二次调用 confirm 时**只认 confirm_id + 用户确认信号, 忽略第二次调用携带的 inputs/业务参数**；(b) DoD #6（L1291）测试覆盖 "第一次 dry-run 用 ns=foo, 第二次 confirm 时塞 ns=bar, 必须仍按 ns=foo 落地" + "服务端快照跨多副本 (test: kill server pod, 重启后另一副本能继续 confirm)"；5 分钟超时 expire 写明 | ✅ 闭环 |
| **#3（Med）验收基线改 MCP Inspector + Claude Desktop** | "验收基线改为 MCP Inspector + Claude Desktop（协议 conformance）；OpenClaw 列为 nice-to-have / alpha" | (a) DoD #2（L1287）"通过 MCP 协议 conformance test (**MCP Inspector** 官方工具)"；(b) DoD #3（L1288）"用 Claude Desktop 配置后能列出 5 个 Skills 并成功调用每一个 (**MCP Inspector + Claude Desktop = 验收基线**)"；(c) DoD #4（L1289）"~~OpenClaw alpha 验证~~ 降级为 nice-to-have / alpha: 不再强绑第三方文档作为 v1 DoD; 失败不阻塞 v1 ship"；(d) Phase 5（L1331-1335）"找 1-2 个客户做 alpha 测试" 措辞已与"不阻塞 v1 ship"对齐 | ✅ 闭环 |

**新发现 / 残留风险**：

| 序 | 项 | 严重度 | 说明 |
|---|---|---|---|
| 1 | **ADR-034 / ADR-036 仍为草稿** | High（解锁前提） | 架构设计.md L37 / L39 号台账明确两项均标 "🆕 草稿 (本次, 2026-05-31)"。PRD-004 进研发**必须**先把这两份从草稿推到 Accepted（PRD-004 关联 ADR 列表 L1094 明示依赖）。**建议**：进研发前对 ADR-034/036 单独做一轮简短 ADR 评审（参考 ADR-Review-2026-05-31 范式）后正式 Accept。本轮不展开 ADR 内容评审，但记录此前置条件。 |
| 2 | DoD 行号编号 #17/#18 紧跟 #6 之后但跳过 #7-#16 | Info | PRD-004 DoD 表（L1284-1303）行号顺序：1, 2, 3, 4, 5, 6, **17, 18**, 7, 8, …, 16。这是文档编号瑕疵（finding T2 新加测试硬塞进序号 6 之后），不影响验收逻辑但阅读体验差。**建议**研发收口前重排为 1-18 顺序。 |
| 3 | "audit 模块跨开源 / 闭源边界"（原第一轮 #4 Med，本轮未触及） | Med | 第一轮 PRD-004 finding #4：开源仓 Apache-2.0 复用 PRD-003 闭源 audit 模块。本轮未在评审范围（只跑 T2/#2/#3），可视作"研发期内处理"，但提示在 Phase 1 仓库脚手架时显式定义 audit 接口（开源）+ 可插拔实现，避免反复改。 |

**决议**：**排队评审**（不是"已评审"）。

- 3 个评审 finding 全闭环；但 PRD-004 解锁进研发的硬阻断不止 finding，还包含**新引入的 ADR 依赖（ADR-034/036）状态从草稿 → Accepted**。在 ADR 未 Accept 前不应 kick off Phase 1（脚手架立项依赖 SDK 选型 + 传输契约）。
- 待 Mars 最后审 + ADR-034/036 Accept 后 → "已评审 / 可进研发"。

---

## 五、PRD-007 v1.1 数据韧性验证（本轮重头戏）

PRD-007 v1.1 修订由 PRD.md L2616 评审历史行触发，落 5 P（High）+ 5 Med/Info。逐项核查：

### 5.1 P1-P5（High）

| Finding | 评审第一轮原文要点 | 修订实证锚点（PRD.md） | 闭环判定 |
|---|---|---|---|
| **P1（承重 High）Layer 4 复制粒度 + 快照型排除 + Phase 0 真恢复 E2E** | "§4.3 钉死适用边界：仅 BSL-resident 数据，快照型排除；粒度 = `kopia/<ns>` 仓库 + `backups/<name>/` 元数据；Phase 0 加'复制后可恢复 + CSI 快照排除'E2E" | (a) **§4.3 完全重写（L2186-2253）**，开头打 v1.1 重写标记；(b) "适用边界（硬约束, 优先于其他描述）"段（L2190-2192）明确"仅 BSL-resident 数据"+ 快照型备份"恢复时没有卷数据"+ 改用"快照级复制（另一机制 / 另一 PRD）"；(c) "复制粒度（必读, 与 Velero/Kopia 布局对齐）"段（L2194-2204）：source 单元 = `bucket/kopia/<ns>/` + `bucket/backups/<name>/`；显式声明"不是早期草稿 UI 暗示的'按备份挑对象'(该 UI 形态已砍掉)"；(d) API 显式删除 "选某几个 backup" 选项（L2223）；(e) Preflight 拦截（L2225-2229）：错误码 `ERR_LAYER4_SNAPSHOT_UNSUPPORTED` + 完整文案；(f) **Phase 0 必做 E2E（L2249-2252）三条**：5GB MySQL ns fs-backup + data-mover 双跑 + 快照型排除验证；(g) DoD #2 改写（L2543）"不再仅比对元数据 sha256——可恢复性是 Layer 4 的唯一意义"；(h) DoD #2a 新增（L2544）快照型拦截测试；(i) §9 Phase 0 任务表（L2574）完整列入 v1.1 必加 E2E | ✅ 闭环（承重） |
| **P2（High）Glacier 数据/元数据分推荐 + Lifecycle Preflight** | "数据默认不转 Glacier，元数据可冷归档；apply 前预检 delete ≥ Object Lock 保留期" | (a) §4.5 v1.1 关键修订段（L2308-2336）：表格明确分两类——K8s 资源 tarball + Velero backup CRD（元数据）可冷归档 vs Kopia 仓库 / 卷数据默认不转 Glacier；(b) UI 推荐模板拆为 4 种（金融/等保三级 - 数据热保留 / 完全冷归档 / 互联网 / 极简），数据冷归档需 ⚠ 大红字 + 二次确认；(c) Preflight 校验 3 条（L2331-2334）：`ERR_LIFECYCLE_LOCK_CONFLICT` + 数据转 Glacier 强制二次确认 + 同 prefix 多 transition 报错；(d) DoD #17d/#17e 新增（L2563-2564）覆盖冲突预检 + RTO 警告 | ✅ 闭环 |
| **P3（High）Fingerprint 威胁模型重述** | "SHA256 检测意外损坏；防恶意篡改依赖 Object Lock 或外部持有密钥的强制签名（跨集群必需）" | (a) §4.4 开头"威胁模型（明确范围, 必读）"段（L2256-2262）：SHA256 = 完整性, HMAC = 防恶意篡改；明示"能改 BSL 上 tarball 的攻击者通常也能改 fingerprint JSON"；(b) "签名要求（v1.1 强化）"段（L2281-2285）：单集群 optional / **跨集群 hard required**；HMAC 密钥经 Helm `--set fingerprint.sharedSecret=<base64-32B>` 下发，存 K8s Secret `supkube-fingerprint-secret`, **不入 BSL, 不写日志**；(c) DoD #17a 新增（L2560）测试覆盖跨集群缺密钥 → `signature_required` + Restore disabled；admin 配密钥后通过；(d) Q2 标 ✅ 评审已答 + 留 BLAKE3 v1.x 评估 | ✅ 闭环 |
| **P4（High）DR Drill default-deny egress + 端点重写** | "default-deny egress NetworkPolicy 提为 v1 必做；文档明确 DR Drill 仅对自包含工作负载安全；端点重写 Transform" | (a) §4.7 顶部"⚠ 安全警告 (v1.1 必读)"段（L2354）显式声明 default-deny 已提为必做；(b) §4.7 模型第 2 步（L2358-2362）DR Drill Wizard 必勾 "外部依赖确认" checkbox + 审计；(c) §4.7 第 4 步（L2363-2366）完整 NetworkPolicy 模型：默认 egress 全拒 + 沙箱内 Pod-Pod + kube-dns + 客户白名单；(d) §4.7 第 5 步（L2368-2369）新增 `redirect-external-endpoints-to-sandbox` Transform；(e) DoD #17b/#17c 新增（L2561-2562）covering default-deny + Wizard confirmation；(f) Q8 标 ✅ 评审已答 | ✅ 闭环（PRD-007 自身侧）。**但带出未闭的 follow-up**：该 Transform 须加入 PRD-002 §4.1 列表，PRD-007 L2369 称"PRD-002 v1.1 已同步"，实测未同步（见 §六） |
| **P5（High）Score 二选一 + 共享数据采集层** | "明确二者关系——定义为两个不同指标，各自命名、各有分解，DoD #17 改为'共享底层数据采集层、但分数定义不同'" | (a) §4.8 v1.1 关键修订段（L2390-2432）选 (a) 方案：明确 PRD-003 Application Resilience Score（单应用 5 维度）vs PRD-007 Cluster Posture（集群 5 层覆盖, L1~L5 各 20 分）；(b) 共享与独立的边界（L2400-2402）：**共享** = `internal/resilience/` 底层数据采集层；**独立** = 分数算法；显式声明"二者不可直接相减或对比"；(c) UI 上明示关系（L2404-2407）：Posture 卡 hover 显示应用韧性分布聚合 / PRD-003 应用页 banner 显示集群 Posture / **绝不允许同图加减对比**；(d) §4.8 数据采集层共享细节（L2427-2432）：`internal/resilience/scan.go` + `store.go` 由本 PRD 实现，PRD-003 import，避免双采集；(e) DoD #17 改写（L2559）"共享底层数据采集层但分数定义不同, UI 明示二者关系, 不允许直接相减或对比"；(f) Q10 标 ✅ 评审已答 | ✅ 闭环 |

### 5.2 Med #1-#3 / Info #4-#5

| # | 评审第一轮要点 | 修订实证锚点 | 闭环判定 |
|---|---|---|---|
| **Med #1** Kopia 维护 vs Object Lock 兼容性 | "Phase 0 实测 Kopia 维护在 Object-Locked target 上的行为；必要时关闭维护或用 immutable-storage 模式" | §4.2 子功能表（L2184）新增"Kopia 维护 vs Object Lock 兼容性"行，完整说明 Phase 0 必测 + 实测结果写 ADR-031 §X 补遗 + 必要时关闭维护或启用 Kopia immutable-storage 模式（v0.15+）；DoD #17f 新增（L2565）覆盖 | ✅ 闭环 |
| **Med #2** AI 钩子脱敏 / 出境 | "复用 SECURITY.md §6 + PRD-003 §7.2 统一管线，与 PRD-005/006 同口径, PRD 里显式引用" | §4.7 DR Drill 失败→AI（L2373）+ §4.8 Posture→AI 建议（L2423 + L2432）均显式声明"AI 调用走 SECURITY.md §6 + PRD-003 §7.2 统一脱敏管线, 不另做" | ✅ 闭环 |
| **Med #3** DoD #2 改为可恢复性验证 | "改/补为'从 target BSL 完整 Restore + 卷数据一致'" | DoD #2 完全改写（L2543）：5GB MySQL with rows + `SELECT COUNT(*)` 与源端一致 + fs-backup + data-mover 各一次 | ✅ 闭环 |
| **Info #4** Lifecycle 同 prefix 冲突 | "预览页按 BSL 类型展示 merge 后完整规则 + 冲突检测（同 prefix 多 transition 冲突时报错而非静默追加）" | §4.5 预览页段（L2328）"按 BSL 类型 (S3 XML / Azure JSON / GCS JSON) 展示 merge 后完整规则; 同 prefix 多 transition 冲突 → **直接报错而非静默追加**, 客户须显式合并或选'覆盖现有'"；Preflight 校验 3 条之一覆盖（L2334）；DoD #17d 覆盖（L2563） | ✅ 闭环 |
| **Info #5** cluster_id 生命周期假设 | "文档注明该 ID 的生命周期假设；TrustStore 记录绑定时间便于排查" | §4.4 cluster_id 生命周期假设段（L2292-2296）：完整文档化"ns 被重建 / 集群迁移时会变"+ TrustStore 附 `bound_at` + `original_cluster_name` + USER_MANUAL §跨集群信任引用 | ✅ 闭环 |

### 5.3 PRD-007 新发现 / 残留

| 序 | 项 | 严重度 | 说明 |
|---|---|---|---|
| 1 | **`redirect-external-endpoints-to-sandbox` Transform 未真正同步进 PRD-002 §4.1** | Med（follow-up） | 详见 §六。PRD-007 §4.7 行 2369 声明"该 Transform 须加入 PRD-002 §4.1 的 11 个 builtin Transform 列表 (PRD-002 v1.1 已同步)"，实测 PRD-002 §4.1（PRD.md L364-381）仍是 11 项，未加该 Transform。不阻塞 PRD-007 P4 自身的 NetworkPolicy 必做项，但阻塞 §4.7 第 5 步"自动 Transform 链"实现（依赖该 Transform 在 PRD-002 builtin 中存在）。 |
| 2 | PRD-003 自身未做对应 P5 / Med #2 修订（不在本轮范围） | Info | PRD-007 §4.8 + §4.7 都引用 PRD-003 §7.2 脱敏管线 + 共享 `internal/resilience/`，但 PRD-003 自身在本轮不在评审范围（亦无对应 finding 触发其修订）。**不影响 PRD-007 决议**，但 PRD-003 后续修订时应回头核对 §3.3 Score / §7.2 脱敏管线两项与 PRD-007 v1.1 引用是否一致。 |

**决议**：**排队评审**（不是"已评审"）。

- 5 P + 5 Med/Info 全闭环，承重项 P1 修订到位（§4.3 完全重写、Phase 0 真恢复 E2E、快照型排除 Preflight + 错误码 + DoD 全套）。
- 但有 1 个实质 follow-up（`redirect-external-endpoints-to-sandbox` Transform 未同步进 PRD-002）需在进 Phase 2/5 前落地。
- **Phase 0 verify-before-architect 可立即启动**（不依赖该 follow-up）；**Phase 5 DR Drill 实施需 follow-up 先落**。

---

## 六、跨 PRD 一致性

### 6.1 已闭环

| 联动 | 锚点 | 状态 |
|---|---|---|
| **PRD-002 ↔ ADR-003 修订** | PRD-002 §4.1bis（L442-474）⇋ 架构设计.md ADR-003 修订段（L905-924）。两侧 compile rule 同名（`supkube-restore-rm-<restoreName>-<hash>`）、同 `len(Data)==1`、同顺序语义（同 path 后者覆盖）、同 GC 策略（7 天，按 label 清理） | ✅ 完全一致 |
| **PRD-001 ↔ PRD-002** | PRD-001 L132 "依赖 PRD-002 必须先评审通过"；PRD-002 L713 "PRD-001 v2 必须先评审通过本 PRD"。PRD-002 已可进研发，PRD-001 与之同窗口 kick-off | ✅ 一致 |
| **PRD-007 ↔ ADR-031** | PRD-007 §4.1 + §4.3 多处显式引用 ADR-031 §1 实测结论（snapshotMoveData=false 卷数据在云端区域快照），与 ADR-031 原文一致 | ✅ 一致 |
| **PRD-007 P5 Score 拆分 ↔ PRD-003 §3.3 引用** | PRD-007 §4.8 表格（L2395-2398）明确 PRD-003 Score 定义 = "单应用 / 单 ns / 5 业务/架构维度 / 0-100"，与 PRD-003 §3.3 当前文本一致；共享接口约定 `resilience.GetClusterState() *ClusterState` 写明 PRD-003 import 路径 | ✅ 一致 |
| **PRD-007 Layer 4 / DR Drill / IntegrityCheck → PRD-006 Activity Timeline ActionType** | PRD-007 §4.3 L2247 显式声明 "BackupCopy 进 Activity Timeline"；DoD #18（L2566）覆盖；与 PRD-006 §4.1 ActionType 范围吻合（PRD-006 不在本轮，假设一致） | ✅ 一致（PRD-006 不在本轮，按 PRD-007 自述断定） |

### 6.2 未闭 follow-up（本轮唯一）

| Follow-up | 锚点 | 建议处理 | 时机 |
|---|---|---|---|
| **`redirect-external-endpoints-to-sandbox` Transform 未真正同步进 PRD-002 §4.1 builtin 列表** | PRD-007 §4.7 第 5 步（PRD.md L2369）声明"该 Transform 须加入 PRD-002 §4.1 的 11 个 builtin Transform 列表 (PRD-002 v1.1 已同步)"；实测 PRD-002 §4.1 行 364-381 表格仍是 11 个 builtin（strip-nodeport / strip-clusterip / strip-loadbalancer-ip / strip-pv-binding / change-storage-class / change-docker-registry / nginx-ingress-features / traefik-ingress-features / remove-ingress-annotations / remove-ingress-finalizers / scale-deployment），**没有 `redirect-external-endpoints-to-sandbox`** | (a) **短期**：在 PRD-002 §4.1 表格新增第 12 行 `redirect-external-endpoints-to-sandbox`（category=network-ingress 或新增 sandbox-rewrite category）；(b) Phase 5 实施需要的 Transform spec（subject + patch + ${VAR} 占位）由 PRD-002 起草；(c) PRD-007 §4.7 L2369 措辞从"已同步"改"待 PRD-002 v1.3 同步"避免误导 | 在 PRD-007 进 Phase 2/5 前 |

### 6.3 待 PRD-004 解锁的 ADR 依赖

| 依赖 | 当前状态 | 阻断范围 |
|---|---|---|
| ADR-034 MCP 协议选型（Streamable HTTP）| 🆕 草稿（架构设计.md L37）| PRD-004 进 Phase 1 脚手架 |
| ADR-036 SSE 项目级口径（含 PRD-004 兼容退路 + PRD-005 Live Tail 共用）| 🆕 草稿（架构设计.md L39）| PRD-004 进 Phase 1 + PRD-005 v2.1 |

**建议**：对 ADR-034 / ADR-036 单独跑一次简短 ADR 评审（参考 ADR-Review-2026-05-31 范式），把"草稿 → Accepted"通路打通后再 kick off PRD-004 研发。

---

## 七、整体放行结论

### 7.1 4 PRD 决议汇总

| PRD | finding 闭环 | ADR 依赖 | 跨 PRD 一致性 | 本轮决议 | 解锁动作 |
|---|---|---|---|---|---|
| **PRD-001 v2** | 4/4 ✅ | 无 | ✅ 与 PRD-002 双向引用一致 | **排队评审** | Mars 最后审 → 已评审 → 与 PRD-002 同窗口 kick-off #104 |
| **PRD-002 v1.2** | 5/5 ✅（含 T1 / #1 编译契约）| ✅ ADR-003 修订段已落（架构设计.md L905-924）| ✅ 与 ADR-003 完全一致 | **已评审 / 可进研发** | 解锁 #114 + 与 PRD-001 同窗口 kick-off |
| **PRD-004** | 3/3 ✅（T2 Blocker 闭环）| ⚠ ADR-034 / ADR-036 仍为草稿 | ✅ 与 PRD-003 Engine 共享约定一致 | **排队评审** | ADR-034/036 Accept + Mars 最后审 → 已评审 → kick off Phase 1 |
| **PRD-007 v1.1** | 5 P + 5 Med/Info 全闭环 ✅ | ✅ ADR-031 引用一致；ADR-033（拟）AI Advisor 评分整合不阻塞 | ⚠ 1 处 follow-up：`redirect-external-endpoints-to-sandbox` Transform 未真正同步进 PRD-002 §4.1 | **排队评审** | Mars 最后审 → 已评审 → Phase 0 立即启动；Phase 2/5 前补 follow-up |

### 7.2 解锁路径

| 解锁层级 | 内容 |
|---|---|
| **立即解锁** | PRD-002（#114）→ 与 PRD-001（#104）同窗口 kick-off |
| **Phase 0 立即启动** | PRD-007（verify-before-architect：rclone 跨云吞吐 / Layer 4 真恢复 E2E / Kopia 维护 vs Object Lock 实测）|
| **待 ADR Accept 后解锁** | PRD-004（等 ADR-034 / ADR-036 从 🆕 草稿 → Accepted）|
| **待 follow-up 后 Phase 全开** | PRD-007 Phase 2 / 5（等 `redirect-external-endpoints-to-sandbox` 同步进 PRD-002）|

### 7.3 仍需的 follow-up（按紧急度）

| # | 项 | 触发时机 | 归属 |
|---|---|---|---|
| 1 | PRD-002 §4.1 加 `redirect-external-endpoints-to-sandbox` Transform（v1.3 微调）+ PRD-007 §4.7 L2369 措辞修正 | PRD-007 进 Phase 2/5 前 | PRD-007 实施 owner |
| 2 | ADR-034 / ADR-036 简短评审 + Accept | PRD-004 kick off 前 | ADR-Review owner |
| 3 | PRD-004 DoD 行号重排为 1-18 顺序（#17/#18 紧跟 #6 后面是文档瑕疵）| 研发收口前 | PRD-004 owner |
| 4 | PRD-003 后续修订时核对 §3.3 Score / §7.2 脱敏管线与 PRD-007 v1.1 引用一致 | PRD-003 下次修订 | PRD-003 owner |
| 5 | PRD-007 Phase 0 跑通后把 Layer 4 真恢复 E2E + Kopia 维护实测结论回写 ADR-031 §X 补遗 | Phase 0 完成后 | PRD-007 实施 owner |

### 7.4 下一轮评审建议

- 本轮 PRD-001 / PRD-004 / PRD-007 均"排队评审"——Mars 最后审一次（不需要全文重看，看本报告 §二/四/五 的"决议"段 + §六 follow-up）即可批"已评审"。
- PRD-002 本轮直接"已评审 / 可进研发"，无需 Mars 再审。
- 下次评审建议：(a) ADR-034 / ADR-036 简短 ADR 评审（PRD-004 解锁前置）；(b) PRD-007 Phase 0 跑完后 PRD-007 实测结果 → ADR-031 补遗评审；(c) PRD-003 若做 v1.x 修订时回看 PRD-007 引用一致性。

---

## 附录：验证方法 + 实证锚点

### A.1 验证方法

本轮采用"finding 逐项 cross-check + 锚点 grep + ADR 依赖核查"三段法：

1. **Finding 逐项 cross-check**：对前 3 份评审报告的每个 finding，定位当前 PRD.md 中的修订段（按"finding #N (2026-05-31)" / "v1.1 修订" / "v1.2 修订" 等显式标记），逐条核对修订内容 vs 原 finding 描述是否一一对应（不只看"提了"，要看"落了 DoD 测试"）。
2. **锚点 grep**：所有"闭环判定"行均附 PRD.md / 架构设计.md 的具体行号锚点，可在 git diff / git blame 中复现。
3. **ADR 依赖核查**：对承重依赖（ADR-003 修订 / ADR-034 / ADR-036）独立 grep 架构设计.md 实证，不只信 PRD 自述"已修"。

### A.2 关键实证锚点汇总

| 文档 | 行号 | 内容 |
|---|---|---|
| PRD.md | L122-296 | PRD-001 v2 全文（174 行，v1 残留已物理删除）|
| PRD.md | L149-150 | PRD-001 finding #1 blocker 不可忽略 User Story |
| PRD.md | L156-167 | PRD-001 finding #2 severity 后端固化 schema |
| PRD.md | L170-174 | PRD-001 T3 SC→Immediate 拓扑校验 |
| PRD.md | L247-252 | PRD-001 DoD #6/#7/#10/#11 finding 测试覆盖 |
| PRD.md | L296 | PRD-001 v1 残留物理删除注释 |
| PRD.md | L382-407 | PRD-002 builtin Transform YAML 改 `data.rules.yaml` |
| PRD.md | L442-474 | PRD-002 §4.1bis 编译契约（finding #1/T1 核心修订）|
| PRD.md | L476-494 | PRD-002 §4.2 统计机制 Event 流 + aggregator goroutine（finding #3）|
| PRD.md | L519-538 | PRD-002 §4.3bis ${VAR} 校验 + 引用顺序（finding #4/#5）|
| PRD.md | L669-674 | PRD-002 DoD #15-#20 finding 测试覆盖 |
| PRD.md | L678-685 | PRD-002 Phase 0 迁移测试 + 回滚（finding #2）|
| PRD.md | L1094-1097 | PRD-004 ADR-034/036 依赖 + Streamable HTTP 传输（T2）|
| PRD.md | L1181 | PRD-004 HitL confirm 服务端持久化（finding #2）|
| PRD.md | L1287-1293 | PRD-004 DoD #2/#3/#4/#17/#18 finding 测试覆盖 |
| PRD.md | L1197 | PRD-004 §4.4 技术栈选型 Streamable HTTP 主推（T2）|
| PRD.md | L2186-2253 | PRD-007 §4.3 Layer 4 完全重写（P1 承重）|
| PRD.md | L2249-2252 | PRD-007 Phase 0 必做 E2E 三条（P1）|
| PRD.md | L2256-2285 | PRD-007 §4.4 Fingerprint 威胁模型重述（P3）|
| PRD.md | L2308-2336 | PRD-007 §4.5 Glacier 分数据/元数据 + Lifecycle Preflight（P2）|
| PRD.md | L2354-2386 | PRD-007 §4.7 DR Drill v1.1 default-deny + Wizard confirmation（P4）|
| PRD.md | L2369 | PRD-007 `redirect-external-endpoints-to-sandbox` Transform 引用（follow-up 触发点）|
| PRD.md | L2390-2432 | PRD-007 §4.8 Score 拆分 + 共享数据采集层（P5）|
| PRD.md | L2543-2566 | PRD-007 DoD #2/#2a/#17/#17a-#17f finding 测试覆盖 |
| 架构设计.md | L37 | ADR 号台账 ADR-034 状态行（🆕 草稿）|
| 架构设计.md | L39 | ADR 号台账 ADR-036 状态行（🆕 草稿）|
| 架构设计.md | L891 | ADR-003 状态行加 v0.9.x 修订标记 |
| 架构设计.md | L905-924 | ADR-003 修订段完整内容（PRD-002 T1 硬依赖）|

### A.3 grep 命令复现

```bash
# 验证 PRD-002 §4.1 builtin Transform 列表（确认仍是 11 个，无 redirect-external-endpoints）
grep -n "scale-deployment\|strip-clusterip\|change-storage-class\|11 个 builtin\|11 builtin Transform" PRD.md | head -20

# 验证 redirect-external-endpoints-to-sandbox 在 PRD.md 中的位置（应仅 PRD-007 §4.7 引用 + 评审历史）
grep -n "redirect-external-endpoints" PRD.md
# 预期输出：2359 (Wizard checkbox), 2369 (§4.7 第 5 步), 2616 (评审历史)
# 不应在 PRD-002 §4.1 builtin 表格（L366-381）中出现

# 验证 ADR-003 修订段是否真存在
grep -n "ADR-003\|v0.9.x 修订\|rules.yaml\|len(Data)==1" 架构设计.md | head -50
# 预期输出：L891 状态行 + L893 状态描述 + L903 v0.8.2 教训 + L905 修订段标题 + L916/917/920 修订内容
```

---

<!--
评审元数据（勿删）
- 评审轮次：PRD-Review 第三轮（4 PRD 改正中验证）
- 关键结论：
  - PRD-001 v2: 4/4 finding 闭环 → 排队评审
  - PRD-002 v1.2: 5/5 finding 闭环 + ADR-003 修订已落 → 已评审 / 可进研发（解锁 #114 + #104）
  - PRD-004: 3/3 finding 闭环, 但 ADR-034/036 仍草稿 → 排队评审（待 ADR Accept）
  - PRD-007 v1.1: 5 P + 5 Med/Info 全闭环 → 排队评审（待 follow-up：redirect-external-endpoints-to-sandbox 同步进 PRD-002）
- 跨 PRD 关键发现：PRD-007 §4.7 L2369 误称"PRD-002 v1.1 已同步"该 Transform，实测 PRD-002 §4.1 仍 11 项，列为 follow-up
- ADR-003 修订确认在架构设计.md L905-924，与 PRD-002 §4.1bis 完全对齐
- 下次评审建议：ADR-034/036 简短评审 → PRD-004 解锁；PRD-007 Phase 0 实测结果 → ADR-031 补遗
-->
