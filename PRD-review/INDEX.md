# PRD 评审索引（PRD-Review Index）

> 本目录是 SupKube PRD 评审的归档区。每次评审产出一份带日期的 MD，本 INDEX 汇总全部评审、关键结论与放行状态，便于长期追踪。
> **维护约定**：每完成一份新评审，(1) 在 `PRD-Review/` 新建 `PRD-Review-YYYY-MM-DD[-范围].md`；(2) 回到本 INDEX 追加一行评审记录，并更新下方「未决跨 PRD 主题」与「各 PRD 最新状态」。
> **评审视角**：Kubernetes + AI + 灾备（DR）。**核对基线**：PRD.md、架构设计.md（ADR）、SECURITY.md、测试用例.md 等。

---

## 一、评审记录

| 日期 | 报告文件 | 覆盖 PRD | 核心结论（最高优先项） |
|---|---|---|---|
| 2026-05-31 | [PRD-Review-2026-05-31.md](./PRD-Review-2026-05-31.md) | PRD-001 / 002 / 003 / 004 | T1 两层模型→Velero ConfigMap 编译契约需写明；T2 MCP 弃用 SSE 应改 Streamable HTTP；T3 SC 改 Immediate 多 AZ 风险；T4 AI 默认外发集群上下文、合规与文档空白 |
| 2026-05-31 | [PRD-Review-2026-05-31-PRD005-006.md](./PRD-Review-2026-05-31-PRD005-006.md) | PRD-005 / 006 | X1 deep-link 契约两份不一致；X2 ADR-033 编号撞车；X3 SSE 与 ADR-015 冲突需项目级口径；X4 AI 日志外发并入 T4 治理 |
| 2026-05-31（复审） | （本 INDEX「复审记录」小节） | 全部 | 团队完成修订（PRD.md + SECURITY.md §6 + 架构设计.md ADR 台账 + ADR-033/034/035/036 草稿）。复审：T1–T4 + X1–X4 共 8 条，**全部闭环**（最后 T1 的 ADR-003 修订段于同日补入架构设计.md） |
| 2026-05-31 | [PRD-Review-2026-05-31-PRD007.md](./PRD-Review-2026-05-31-PRD007.md) | PRD-007 | P1 Layer 4 Backup Copy 的 Kopia 共享仓库复制语义 + 快照型数据不在 BSL（承重·恐"可见不可恢复"）；P2 Glacier 冷归档破坏 RTO + Object Lock 冲突；P3 Fingerprint 防篡改威胁模型高估；P4 DR Drill 单 ns 挡不住对生产 egress 副作用；P5 Posture Score 与 PRD-003 Score 口径是否同义 |
| 2026-05-31 | [PRD-Review-2026-05-31-PRD008-009.md](./PRD-Review-2026-05-31-PRD008-009.md) | PRD-008 / 009 | 008：D1 audit 存储 etcd 规模风险 + 对账 ADR-019；D2 不可篡改需真实机制；D3 集群损毁后审计存活；D5 孤儿清理 Kopia 共享仓库安全（同 007 P1）。009：E1 Snapshot 语义澄清；E2 无快照集群 always-on；E4 与 007 Posture L1-L5 术语一致 |
| 2026-05-31 | [PRD-Review-2026-05-31-PRD010.md](./PRD-Review-2026-05-31-PRD010.md) | PRD-010 | F1 Posture 分数第三套口径（"层数×20"误导，应消费后端单一 score）；F2 数据流箭头与 ADR-025 双 Schedule 不符；F3 L1-L5 vs PRD-009 Snapshot/Export 术语；F4 Layer 5 作为节点语义不自洽。低风险基本通过 |
| 2026-06-01（复审） | （本 INDEX 第二/三节） | 全部（重点 002/007/009） | 团队大批修订后复审。**PRD-007 P1-P5 全闭环**（P1 抓真 fixture 双重证实 + Phase 0 可恢复性 E2E）；PRD-002 v1.3（T1 编译契约 + CAS storm DoD）；**PRD-009 实施 Phase 1**（采纳 E3 复数措辞）。**仍 open**：PRD-008（D1-D5，本轮未改）、PRD-010（F1-F4，本轮未改）、Score 深层等权误导、术语轴映射表、BSL 删除侧孤儿清理 |
| 2026-06-01 | [PRD-Review-2026-06-01-PRD009v2-011-012.md](./PRD-Review-2026-06-01-PRD009v2-011-012.md) | PRD-009 v2 / 011 / 012 | **跨文档 ADR-037/038 撞号**（PRD-008/010 让号 039/040）；009 v2 G5 DoD/任务未随 ImportPolicy 扩 + G1 RPO 卖点 + G2 backupSyncPeriod 回归；011 H1 规则版本化/校准 + H2 异地判定 + H5 LLM 异步（AI 纪律范本，近可过）；012 I1 逐次确认+白名单 + I2 客户身份（Blocked 待 Case API） |
| 2026-06-02 | [DOR-DECISION-2026-06-02.md](./DOR-DECISION-2026-06-02.md) | **全 14 PRD (PRD-001~015) DoR 投产判定** | 第一次按 ENGINEERING §6 投产就绪门槛 6 条系统化判定; 9 立即开工 (PRD-002/003/004/005/006/007/009 + Rule H 应尽则尽 PRD-008/010/011) + 5 暂缓整改 (PRD-001 等 Mars 重审 / PRD-012 等 Case API / PRD-013/014/015 草稿). 每个就绪 PRD 配第一编码单元 + 可交付测试线. 派生 D-WAIT-003/004/005. |
| 2026-06-03（复审） | （本 INDEX 第二/三节） | 全部 | 核验 06-01/06-02 处理：**报告③④⑤⑥ finding 基本全闭环且为真改**——ADR 撞号→039/040+ADR-041 LEDGER；PRD-008 D1-D5（嵌入式 store/hash-chain+webhook/BSL 归档/孤儿 mode/D4 task 派生）；PRD-009 G1-G5（§8.2+§9.3）；PRD-010 F1-F4（加权 score/flows 分类/术语映射/L5 验证徽章）；PRD-011 H1-H5。**现在等：** ① Mars 重审 008/009/010/011（改正中→已评审）② PRD-012 Blocked 待 Mars 给 Case API 规格 ③ **PRD-001 自 05-31 改正中未推进**（最该催）④ Phase 0 实测锁定若干"拟方案" ⑤ 文档卫生：闭环段 §12/§13 vs 原 §4.x 不一致 → **已全量回填（008/010/011），见第二节三态表** |

---

## 二、各 PRD 最新评审状态

> **2026-06-01 复审快照**：团队按评审做了大批修订。PRD-002→v1.3、PRD-003/004/005/006/007 转「已评审」、**PRD-009 已实施 Phase 1**（采纳 E3 复数措辞 "Enable Backups via Snapshot Exports"）。**PRD-007 P1-P5 全闭环**（P1 还抓真 fixture 双重证实）。仍为「草稿」的 PRD-008/010 本轮未改，其 finding 仍 open。详见下表"复审"列与第三节。

| PRD | 名称 | 放行结论 | 关键前置条件 / 复审 | 出处 |
|---|---|---|---|---|
| PRD-001 | 跨集群还原前置检查 | 有条件通过 | blocker 不可忽略 · severity 后端固化 · 删 v1 残留 · 标注 SC→Immediate 多 AZ 风险 | 报告①·三 |
| PRD-002 | Transform 一等公民（两层） | ✅ 已评审 v1.3（06-01 复审：闭环） | T1 编译契约（ADR-003 修订段）+ 统计 CAS storm DoD #18 + CAS fallback 已补；新增 redirect-external-endpoints builtin（配合 007 P4） | 报告①·三 |
| PRD-003 | AI Advisor（推荐型） | 方向通过 | 合规默认本地 LLM、最小外发；Resilience Score 规则化（可复现）；SECURITY.md AI 专章 + ADR | 报告①·三 |
| PRD-004 | MCP Server（Skills） | 建议暂缓 | SSE→Streamable HTTP；HitL 基于服务端快照；多副本确认状态共享 | 报告①·三 |
| PRD-005 | Log Viewer v2 | 有条件通过（按 phase） | 统一 deep-link 契约；SSE 配齐 ingress 配置；ADR-033 重编号；审计导出依赖 forwarding；AI 外发纳入 T4 | 报告②·三 |
| PRD-006 | Activity Timeline | 通过（Phase 0 门禁内建） | 与 PRD-005 统一 deep-link；ETA 取舍；ListActions 预聚合成本约束；AI 外发纳入 T4 | 报告②·三 |
| PRD-007 | 完整 3-2-1-1-0 数据韧性 | ✅ 已评审（06-01 复审：P1-P5 全闭环） | P1 §4.3 重写 + **真 fixture 双重证实** + Phase 0 可恢复性 E2E + `ERR_LAYER4_SNAPSHOT_UNSUPPORTED` 拦截；P2 数据/元数据分推荐 + lifecycle 冲突预检；P3 跨集群 HMAC 强制签名；P4 default-deny egress 提为 v1 必做 + 端点重写 Transform；P5 两独立指标共享数据采集层。**残留**：Posture 仍按层等权×20（PRD-010 F1 深层"等权误导安全感"未动） | 报告③·二 |
| PRD-008 | RP 删除生命周期 + Activity 持久化 | **研发中**（2026-06-03 Mars 评审通过 → 进研发） | D1 嵌入式 store on PV (避 etcd, 与 ADR-019 分工) / D2 hash-chain+admission+WORM 3 层防御 / D3 BSL 归档+verify-archive / D4 deletionState 从 task store 派生不 list DBR / D5 Kopia maintenance 而非裸 mc rm。**§13 修订段 + §8 DoD #19-#23 + M-1/M-2 正文回填**。⏸ ADR-039 占号待 Phase 0 实测锁存储选型 (Rule H 隔离, 主体可推进) | 报告④·二 + Auto 5h 闭环 |
| PRD-009 | Policy 对齐 Kasten + Import Policy | **研发中**（2026-06-03 Mars 评审通过 → 进研发） | Phase 1：E3 已闭环 + 已 ship。**v2 (报告⑥)**：**G5 (High)** §8.2 Phase 2 DoD 14 条 + §9 Phase 2 任务 7 阶段 + §9.3 风险评级独立写（不再 0 迁移风险）/ **G1 (Med)** §4.5.3 RPO 卖点诚实化撤回"10x RPO"过承诺 / **G2 (Med)** 头表 backupSyncPeriod 维持默认 60s 防退化 / **G3 (Low)** §4.5.4 warn 模式 hover tooltip 标"可能未完成半成品" + 列表行加来源 backup phase / **G4 (Low)** §4 Action Type pill 加 inline alert 保存后不可改 + Save confirm dialog + 编辑灰化只读 + **M-3/M-4/M-5 正文回填 (CRD 字段名 + 错误码 FINGERPRINT_INVALID + 失败阈值 N=5)**。fingerprint 三档是 007 P3 的好落地。ADR-038 草稿(内容已写) | 报告④·三 + 报告⑥·三 |
| PRD-010 | DR Topology v2（可视化重构） | **研发中**（2026-06-03 Mars 评审通过 → 进研发） | F1 消费 PRD-007 §4.7 单一 score (不另算层数×20) / F2 flows[].type 5 类着色+线型 / F3 L1-L5↔Snapshot/Export hover tooltip 映射 / F4 Layer 5 改顶部验证徽章 4 状态。**ADR-038 让号→ADR-040 已完成**。**§13 修订段 + §8 DoD #13-#16 + M-6/M-7 正文回填 (§4.1/§5 L5 验证徽章 + §4.2/DoD#5 箭头 sync→import)**。⏸ ADR-040 SVG 视觉规范占号待 PRD-010 实施落地 (Rule H 隔离, 主体可推进先落 SVG 规范) | 报告⑤·二 + Auto 5h 闭环 |
| PRD-011 | AI Backup Advisor MVP | **研发中**（2026-06-03 D-WAIT-002 已拍 → 进研发） | H1 规则集版本化 scoreRulesVersion + evaluator 每条规则带依据注释 + **Mars 自定 4 维 25/35/20/20 标准对标矩阵 + 封顶≤30 + 校准≥90→30 + 90/75/60 分级** (D-WAIT-002 闭环) / H2 BSL region+provider 三元组判定异地 5 级 / H5 拆 /ai/score 同步 5s + /ai/explain SSE 异步流式。**§12 修订段 + §8 DoD #14-#17 + §4.6 API 正文回填**。⏸ ADR-043 评分细则 v1.0.0 占号待写正文 + ADR-046 AI 容灾决策两层体系待写 (Rule H 隔离, evaluator.go skeleton 可推进) | 报告⑥·四 + Auto 5h 闭环 |
| PRD-012 | Call Home / Auto-Support | **改正中**（I1 闭环, I2 仍 Blocked 等 Case API） | I1 默认逐次确认 (UI 抽屉显示 SanitizeReport 全文) + Call Home payload 并入 SECURITY.md §6.C 出境白名单 / I2 customer-id 经 HMAC 派生方案设计就绪, 等 Mars 给 Case API spec。**§10 修订段 + §8 DoD #13-#15** | 报告⑥·五 + Auto 5h 闭环 |
| PRD-013 | SupKube Four-Eyes Authorization (MFA + 二次审批) | **草稿**（2026-06-02 立项 Mars 决策 frame shift 派生） | 4 大类 15 受保护操作 (Veeam VBR 13 对标 + SupKube K8s 多集群特有 3 操作) + ApprovalPolicy/ApprovalRequest CRD + Dex MFA + 状态机 7 态 + 6 条审计 event + 17 条 DoD + 9 阶段 ~13d 估时。Q1-Q5 待 Mars 拍 (紧急 bypass / 优先级 / grace period / Recovery code 用尽 / AI Advisor identity)。⏸ ADR-045 占号待写 | Auto 5h 立项 (无评审报告) |
| PRD-014 | 前端 UI 暴露模型（运维 Day-0 4 模式访问方式） | **草稿**（2026-06-03 立项 charter 级） | LoadBalancer / NodePort / ClusterIP+port-forward / Ingress 4 模式 + Dex publicURL 范本 + 装后 NOTES.txt 按模式打印访问方式。Mars 立项, 详细 §4 待补 | 待 PRD-Review 启动评审 |

> 报告① = PRD-Review-2026-05-31.md；报告② = …-PRD005-006.md；报告③ = …-PRD007.md；报告④ = …-PRD008-009.md；报告⑤ = …-PRD010.md；报告⑥ = …-2026-06-01-PRD009v2-011-012.md。

### 二之补、「改正中」PRD 三态放行跟踪（2026-06-02 新增）

> **为什么要这张表**：finding 在 §12/§13 闭环段写了拟方案 **≠** 原 §4.x 正文已对齐。2026-06-02 发现 PRD-008/010/011 都出现"闭环段改了、正文没回填"的自相矛盾——研发只读正文会拿到**被取代的旧指引**。根因是放行只跟踪了两态（闭环 / 重审），缺中间的"正文回填"态。本表把放行拆三态。
> **放行铁律**：**② 正文回填未 ✅，不得进入 ③ 重审**（否则 Mars 会批到脏底稿）。

| PRD | ① finding 闭环 | ② 正文回填（§4.x 对齐/加指针） | ③ 可重审（Mars 拍板） | 备注 |
|---|---|---|---|---|
| PRD-008 | ✅ D1-D5 | ✅ 已回填：§4.1.2 存储选型 + §4.4.2 mode 参数（06-02）；**+ 二级残留**（06-03）§11 Q1/§9 Phase0/§8 DoD#12 同步 CRD→嵌入式 store + mode 术语（M-1/M-2） | ✅ 评审通过 → 研发中（2026-06-03 Mars 拍板） | — |
| PRD-009 | ✅ G1-G5 | ✅ 已回填（06-03 二级残留）：DoD 字段名对齐 CRD spec（continuous.pollInterval/lastPollAt/sourceClusterFilter）、错误码统一（HMAC_INVALID→FINGERPRINT_INVALID）、失败阈值统一 N=5（M-3/M-4/M-5） | ✅ 评审通过 → 研发中（2026-06-03 Mars 拍板） | 原判"无需"被深审推翻 |
| PRD-010 | ✅ F1-F4 | ✅ 已回填：§4.6/§4.7 Posture（06-02）；**+ 二级残留**（06-03）§4.1/§5 L5 节点→验证徽章、§4.2/DoD#5 箭头 sync→import 对齐 §13 F2（M-6/M-7） | ✅ 评审通过 → 研发中（2026-06-03 Mars 拍板） | §3 漏网，回填时补抓 |
| PRD-011 | ✅ H1/H2/H5 | ✅ 已回填 2026-06-02：§4.6 API（`/ai/analyze`→`/ai/score` 同步 + `/ai/explain` SSE 异步） | ✅ 数值已拍 → 研发中（D-WAIT-002 2026-06-03：Mars 自定 4 维标准对标矩阵 25/35/20/20，对标 ISO/NIST；封顶≤30 + 校准≥90→30；带 90/75/60） | 第三例，加列时补抓 |
| PRD-012 | ◑ I1 闭环（I2 Blocked） | ◑ 基本一致（§4.x 未写明"默认逐次确认"，可选加指针，非硬矛盾） | 🔴 Blocked：待 Case API spec | — |

> 图例：✅ 完成 · ⏳ 等待 · 🔴 阻塞 · ◑ 部分。

---

## 三、未决的跨 PRD / 跨文档主题（滚动维护）

这些不属于单一 PRD，需项目级收口；每次评审复查是否仍 open。

| 编号 | 主题 | 状态 | 摘要 / 下一步 |
|---|---|---|---|
| ADR 编号 | ADR-033 曾被双重占用（AI Advisor vs 结构化日志） | 🟢 已解决 2026-05-31 | 架构设计.md 建「ADR 号台账」；AI=033、MCP=034、结构化日志=035、SSE 口径=036；PRD 引用已统一 |
| SSE 口径 | SSE 在 PRD-004（MCP）/ PRD-005（Live Tail）出现，与 ADR-015 冲突 | 🟢 已解决 2026-05-31 | ADR-036 决策树：Live Tail 保 SSE + nginx ingress 配置；MCP 改 Streamable HTTP（ADR-034）；PRD-004 正文全改 |
| AI 数据出境 | AI Advisor / 日志根因 / 排错均把集群数据·日志外发 LLM | 🟢 已解决 2026-05-31 | SECURITY.md §6（7 子节）+ ADR-033；默认本地 Ollama（v1）、出境白名单、强制脱敏管线、100% 审计、可一键关闭；PRD-003 §7.2 + PRD-005 §4.9/§7 + PRD-006 §4.7 统一复用 |
| deep-link 契约 | PRD-005 与 PRD-006 deep-link 路由/参数不一致 | 🟢 已解决 2026-05-31 | PRD-005 §4.8 定为权威单一来源（`/observability?tab=logs` + `sinceSeconds`/`scrollToLine`/`auto` + 无效参数兜底）；PRD-006 §4.6 改为引用、不另写 |
| Velero 编译契约 | TransformSet 两层模型→单个 resourceModifierRef ConfigMap | 🟢 已解决 2026-05-31 | PRD-002 §4.1bis 编译契约 + DoD #16/#19；架构设计.md ADR-003 已补「修订 (2026-05-31, v0.9.x)」段（两层模型 + 编译派生 CM `supkube-restore-rm-<name>-<hash>` + len==1 + supersede 原单 ConfigMap 心智） |
| 存储拓扑 | SC 改 Immediate 在多 AZ 的数据可用性风险 | 🟢 已解决 2026-05-31 | PRD-001 §4 新增「SC→Immediate 拓扑校验」+ DoD #10；多 AZ 下不假装允许，给 `topologyHint=multi-az` warning |
| ADR 立项门禁 | ADR-033~036 仍「草稿」，AI/MCP/日志属架构级决策 | 🟡 跟踪 | 4 个 ADR 已出草稿；把「对应 ADR 评审通过（草稿→Accepted）」设为 PRD-003/004/005 进研发的前置门禁 |
| ADR-037/038 撞号复发 | 台账 037=数据采集(011/012)、038=ImportPolicy(009)，但 PRD-008 仍引 037=审计存储、PRD-010 仍引 038=Topology SVG | 🟢 已解决 2026-06-01/02 | PRD-008→ADR-039、PRD-010→ADR-040 已让号并登记台账（PRD 正文 6 处替换）；**更进一步**：新建 ADR-041（全项目统一编号 LEDGER.md + Rule G 取号 SOP，"被复用过的旧号不得沿用"制度化）。撞号根因已制度性消除 |
| Layer 4 复制语义 | TransformSet 无关——Velero 卷数据在共享 Kopia 仓库 + 快照型数据不在 BSL | 🟢 已解决 2026-06-01（PRD-007 P1） | §4.3 重写 + **真 fixture 双重证实**（`engineer-testing/fixtures/velero-real-2026-05-31-060756/`）+ Phase 0 可恢复性 E2E + `ERR_LAYER4_SNAPSHOT_UNSUPPORTED` 拦截；复制粒度=`kopia/<ns>` 仓库级 sync |
| Score 口径 | 三处定义：PRD-003 应用韧性分 / PRD-007 层覆盖分 / PRD-010 "层数×20" | 🟢 已解决 2026-06-02（含深层 F1） | P5 两独立指标；**深层也闭环**：PRD-007 §4.7 锁定单一加权规则（按 3 份/2 介质/1 异地/1 不可变/0 错误实际贡献加权，废弃层数×20），前端 grep 无 `layerCount*20`，PRD-010 只渲染 `GET /dashboard/posture`；PRD-003/007/010 共用 PRD-011 `evaluator.go` 规则引擎 |
| 数据保护术语轴 | Policy=Snapshot/Export（009）/ Posture 卡=L1-L5（007）/ Topology 徽章=L1-L5（010）/ **+Import（009 v2 新增 Action Type 二选一）** | 🔴 open（009 v2 引入 ImportPolicy 后术语轴新增"Import"维度, 待映射表敲定时一起对齐 PRD-007 L1-L5；原 009 Phase 2 留尾 E4 + 010 F3 仍 open） | PRD-009 v2 (2026-06-01 本日) 在 Policy 顶部加 Action Type pill (Snapshot Policy vs Import Policy)，"Import" 加入术语轴；映射表敲定时需同时对齐: (a) Snapshot Policy ↔ L1/L2（已是 v1 范围）; (b) **Import Policy ↔ "L3/L4 跨集群拉取" 在 5 层模型里没明确位置**（fingerprint 校验属安全维度, 不是 5 层覆盖维度）, 需要 PRD-007 §4.4 + PRD-010 F3 一并解决 |
| DR Drill 副作用 | 单 ns 沙箱挡不住对生产 egress（写真实库/发邮件/触发支付） | 🟢 已解决 2026-06-01（PRD-007 P4） | default-deny egress NetworkPolicy 提为 v1 必做 + `redirect-external-endpoints` 端点重写 Transform（已并入 PRD-002 builtin）+ Wizard 外部依赖确认 checkbox + DoD #17b/#17c |
| BSL 数据删除/复制统一指引 | Kopia 共享去重仓库：裸 mc rm/object copy 都不安全（PRD-007 P1 + PRD-008 D5 同源） | 🟢 已解决 2026-06-02 | 复制侧（007 P1）fixture 证实；删除侧（008 D5 §13）已闭环：孤儿清理加 `mode`（metadata-only / full-with-kopia-maintenance），后者走 `kopia maintenance run` 或 Velero DBR 而非裸 mc rm + 二次确认；DoD #23；与 007 P1 共用 USER_MANUAL §X。**收尾完成 2026-06-02**：PRD-008 §4.4.2 正文已回填 mode 参数写法，与 §13 对齐（见"文档卫生"行 🟢） |
| 审计存储与存活 | audit event 存 etcd 反模式 + 集群损毁后审计消失 + 与 ADR-019 既有审计并存 | 🟢 已解决 2026-06-02（PRD-008 D1/D2/D3） | ADR-039 选嵌入式 store on PV（离开 etcd）+ 对账 ADR-019；hash-chain + admission webhook（诚实标注"tamper-evident not tamper-proof"）；audit 归档 BSL（HMAC）+ `verify-archive` 新集群可重建；DoD #19-#21。**收尾**：最终选型待 Phase 0 实测锁定 |
| 文档卫生（闭环段 vs 原文不一致） | 06-02 闭环以追加 §12/§13 段实现，原 §4.x 正文未同步 → 自相矛盾 | 🟢 已解决 2026-06-02 | **全量回填完成**：PRD-008 §4.1.2（CRD→嵌入式 store）+ §4.4.2（mc rm→mode）、PRD-010 §4.6/§4.7（层数×20→§4.7 加权 score）、PRD-011 §4.6（/ai/analyze→/ai/score+/ai/explain SSE），均加"以 §12/§13 为准"指针；PRD-009 无矛盾、PRD-012 基本一致可选加指针。**根因收口**：暴露 finding 跟踪缺"正文回填"态 → 已在第二节新增「三态放行跟踪表」（②正文回填未✅不得进③重审），防复发 |

图例：🔴 未解决 · 🟡 跟踪中 · 🟢 已解决（解决后保留一行并注明出处与日期）。

---

## 四、严重度速览（按报告）

| 报告 | Blocker | High | Med | Info |
|---|---|---|---|---|
| 报告①（PRD-001~004） | 1（PRD-004 SSE）* | T1·T3·T4 + 各 PRD High 项 | 多项 | 文档卫生数项 |
| 报告②（PRD-005~006） | 0 | X1·X2·X3·X4 + 各 PRD High 项 | 多项 | 数项 |

> *注：报告①中 T1 首轮为 Blocker，经 ADR-003/006 核对后降为 High；PRD-004 的 SSE 项（T2）维持 Blocker。详见报告①正文。

---

<!--
索引元数据（勿删）
- 创建：2026-05-31
- 维护规则：新评审 → 新建 dated MD → 本 INDEX 追加「评审记录」一行 + 更新「各 PRD 状态」「未决主题」
- 命名：PRD-Review-YYYY-MM-DD[-范围].md
-->
