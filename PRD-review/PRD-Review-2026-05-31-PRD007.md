# SupKube PRD 评审报告 — PRD-007

> **视角**：Kubernetes + AI + 灾备（DR）专家
> **评审对象**：PRD-007（完整 3-2-1-1-0 数据韧性：5 层可视化 + Layer 4 Backup Copy + Fingerprint + Lifecycle + DR Drill）
> **评审人**：Claude（受 Zack 委托） · **日期**：2026-05-31
> **核对基线**：PRD.md、架构设计.md（ADR-031 / ADR-029 / ADR-025 / ADR-026）、SECURITY.md §4/§5/§6
> **承接**：PRD-Review 第三份；前两份见 `PRD-Review-2026-05-31.md`（PRD-001~004）、`PRD-Review-2026-05-31-PRD005-006.md`（PRD-005/006）

---

## 一、执行摘要

PRD-007 把 ADR-031 的 5 层数据韧性原则产品化，方向正确、与 ADR-031 推荐的 path (c)「Backup-object 复制 = Layer 4」一致，并延续了团队一贯的好纪律：Phase 0 verify-before-architect（甚至预设了 rclone 跨云不达标就回退 S3↔S3 的硬退路）、Fingerprint 默认 hard fail 不开永久信任逃生门、Lifecycle 不自造归档逻辑而委托云端原生、Resilience Score 强制与 PRD-003 共享引擎避免双算。Scope 也克制（明确不做 CDC / block-level / 跨集群强隔离）。

但这份 PRD 触及的是**灾备最容易出"看起来成功、恢复才发现没数据"的暗坑**，需要在研发前把几条 DR 正确性问题钉死。最关键的是 **Layer 4 Backup Copy 的复制语义**：Velero 的卷数据按命名空间存在共享去重的 Kopia 仓库里（`bucket/kopia/<ns>`），且 Velero 原生**不支持单备份跨 BSL 复制**——"逐对象复制某个备份"若不连带整个 ns 的 Kopia 仓库，复制出来的备份**能被发现却恢复不出卷数据**。其余几条（Glacier 冷归档破坏 RTO、Fingerprint 防篡改威胁模型、DR Drill 对生产的副作用、Score 口径与 PRD-003 是否真的同义）都属于"做出来能跑、但在真灾难/审计时翻车"的类型。

### 放行结论速览

| 结论 | 说明 |
|---|---|
| **方向通过 / 有条件（Phase 0 扩测后定）** | 5 层产品化方向、与 ADR-031 一致、verify-before-architect 纪律都对。下列 P1–P5（均 High）需在进入研发前澄清并写入 DoD / Phase 0；P1 不解决可能造成"备份可见但不可恢复"的静默数据丢失，是本 PRD 的承重问题。 |

**严重度图例**：Blocker / High（DR 正确性·合规·数据安全）/ Med / Info。

---

## 二、关键发现

### P1（High，DR 正确性，本 PRD 承重）Layer 4 Backup Copy 的复制粒度与 Kopia 仓库纠缠未澄清

§4.3 把 Layer 4 描述为「直接读 source BSL 对象、写入 target BSL，source data 不重新打 backup」。但 Velero 的对象存储布局有两个硬事实使"逐对象复制某个备份"远不止拷 `backups/<name>/`：

1. **卷数据在共享 Kopia 仓库里，按 ns 不按备份**：Velero 把 PodVolumeBackup / data-mover 的数据写进 `bucket/kopia/<ns>`，**该 ns 所有备份共享、内容寻址去重**。要让一个备份在 target BSL 可恢复，必须连带复制整个 `kopia/<ns>` 仓库（及 `backups/<name>/` 元数据），而不是某个备份的对象子集。按"每条复制规则 / 每个备份"做选择性 object copy 会得到**有元数据、缺数据块**的半成品。
2. **快照型备份的数据根本不在 BSL**：`snapshotMoveData=false` 的原生 CSI 快照备份，BSL 里只有 K8s 资源 tarball + 元数据，**真实卷数据在云厂商的区域快照里**（ADR-031 §1 自己实测并区分过这一点）。对这类备份做 BSL→BSL 对象复制，复制出的备份**恢复时没有卷数据** —— 这正是"看起来成功、恢复才发现空"的典型。
3. **Velero 原生不支持单备份跨 BSL 复制**：官方明确"不能把单个 Velero backup 同时送到多个 BSL"，跨区域推荐用**云原生复制（S3 CRR）**。PRD 把云原生 API（aws s3 cp / azcopy）列为 v1.x 优化是对的，但 v1 的 rclone 路径需要正确建模为**仓库级 / BSL 级 sync**（Kopia 对象内容寻址且不可变 → `rclone sync` 天然增量，只传新块），而不是 UI 暗示的"按备份挑对象"。

**建议**：

- §4.3 明确 Layer 4 的**适用边界**：仅适用于**数据驻留在 BSL 的备份**（data-mover / fs-backup / `snapshotMoveData=true`）；**快照型备份不能走 Layer 4 object copy**（其数据需快照级复制，属另一机制 / 另一 PRD）。Preflight（Phase 4）应检测并拦截"对快照型备份配 Layer 4"。
- 明确复制**粒度 = `kopia/<ns>` 仓库 + `backups/<name>/` 元数据**（或整桶 sync），并说明增量性来自 Kopia 不可变内容寻址 + rclone sync，而非"只拷一个备份"。
- **Phase 0 必须加一条 E2E**：从复制后的 target BSL 真正**还原并校验卷数据一致**（不只比对 tarball sha256），且**专门测一个 CSI 快照型备份**确认被正确排除 / 处理。否则 DoD #2「target 对象 sha256 == source」只证明了元数据/对象搬运成功，**没证明可恢复出数据**。

### P2（High，DR 正确性）Glacier 冷归档让备份"不可即时恢复"，且与 Object Lock 保留期冲突

§4.5 的推荐模板把备份数据转到 Glacier（"90d → Glacier"/"7y Glacier"）。两个隐患：

- **Glacier 对象不能直接读**：Velero / Kopia 无法直接从 Glacier / Archive 层读取，恢复前需先 thaw（数小时）。把备份**数据**转 Glacier = 把 RTO 从分钟级拉到小时级，客户多半不知情。UI 模板需明确标注"此层 RTO 受 thaw 影响（数小时）"，并区分"归档元数据"与"归档可恢复数据"。
- **Lifecycle delete 与 Object Lock 保留期冲突**：若 Object Lock(WORM) 保留 7 年，而 lifecycle 配置"7y 后 delete"，两者必须协调——delete 时间必须 ≥ Object Lock 保留期，否则到期删除被 WORM 拒绝、规则静默失效。§4.5「不覆盖、只追加」也要校验追加的 delete 规则不与既有 Object Lock 抵触。

**建议**：模板里把"冷层"默认设为**仅元数据/旧 RP**而非活跃恢复数据；对"数据转 Glacier"给显式 RTO 警告；应用 lifecycle 前做"delete 时间 ≥ Object Lock 保留期"的预检。

### P3（High，安全）Fingerprint 的"防篡改"威胁模型被高估

§4.4 用 SHA256 + 同 BSL 内的 `.supkube-fingerprint.json`（signature 为 optional）来"防止 BSL 数据被外部工具篡改后误恢复脏数据"。问题：**能篡改 tarball 的攻击者通常也能写同一个 BSL 的 fingerprint JSON**——他重算 sha256、改写 fingerprint 即可绕过。SHA256 存在可写 BSL 里只能检出**意外损坏**，挡不住**有 BSL 写权限的恶意方**。真正的防篡改需要二者之一：(a) Object Lock(WORM) 让对象不可被改写；(b) HMAC/签名用**不存在 BSL 里、target 集群独立持有**的密钥，且**强制**而非 optional。

**建议**：重述威胁模型——"SHA256 检测意外损坏；防恶意篡改依赖 Object Lock 或外部持有密钥的强制签名"。§4.4 的 signature 在跨集群场景应**默认必需**（shared secret 经 Helm/Secret 下发，不入 BSL）；Q2 选 SHA256 没问题，但"防篡改"承诺要绑 WORM/签名才成立。

### P4（High，DR Drill 安全）单 ns 沙箱挡不住对生产的副作用

§4.7 v1 用"单 ns + ResourceQuota"隔离，Q8 称"ResourceQuota + NetworkPolicy 已足够隔离 90% 场景"。但被还原的真实业务 Pod 在启动时常会**连生产外部依赖**：生产数据库 / 消息队列 / 支付网关 / 第三方 API / 服务注册中心——并可能**写入**。ResourceQuota 只管资源，NetworkPolicy 只管集群内流量，**都挡不住到集群外生产端点的 egress**。一次"演练"可能往生产库写脏数据、发真实邮件、触发真实支付。这是 DR Drill 最危险的暗坑。

**建议**：沙箱默认下发 **default-deny egress NetworkPolicy**（只放行 drill 内部 + 明确白名单）；文档明确"DR Drill 仅对自包含工作负载安全，含外部生产依赖的应用需先做网络隔离 / 配置覆盖（如把 DB 端点改指 sandbox mock）"；§4.7 的自动 Transform 除了改 NodePort/Ingress，应支持把已知外部端点 env 重写为沙箱内替身。把 NetworkPolicy 从 Q8 的"提一句"提升为 §4.7 v1 模型的**必做项**。

### P5（High，跨 PRD 一致性）Resilience Score（PRD-003）与 Posture Score（PRD-007）是不是同一指标？

DoD #17 要求"PRD-007 Dashboard 卡 Score 与 PRD-003 §3.3 Score 数值 100% 一致"，§4.8 称二者共享同一引擎。但两者的**度量对象与维度不同**：

- PRD-003 §3.3 的 Resilience Score = **单应用/ns**，0-100，按 5 个业务/架构维度（Business Value / Architecture / Protection / Security / Operation）。
- PRD-007 §4.8 的 Posture Score = **集群级**，0-100，按 3-2-1-1-0 的 **5 层覆盖度**（L1~L5）。

"共享引擎"是好目标，但**两个分数测的不是一回事**，强行要求"数值 100% 一致"可能是范畴错误。

**建议**：明确二者关系——要么定义为**两个不同指标**（应用韧性分 vs 集群 3-2-1-1-0 覆盖分），各自命名、各有分解，DoD #17 改为"共享底层数据采集层、但分数定义不同"；要么定义**单一权威指标**并统一分解维度。无论哪种，`internal/resilience/` 作为唯一数据/计算权威是对的，但"数值一致"的措辞要随定义修正，否则前端会做出两个对不上的数字。

---

## 三、其它发现（Med / Info）

| # | 严重度 | 问题 | 建议 |
|---|---|---|---|
| 1 | **Med** | 复制进 Object Lock(WORM) 的 target BSL 时，Kopia 维护（compaction 会删/改写对象）与不可变存储冲突，可能导致仓库膨胀 / 维护失败（Kopia + immutable storage 是已知难点）。 | Phase 0 实测 Kopia 维护在 Object-Locked target 上的行为；必要时对 copy-target 关闭 Kopia 维护或用 Kopia 的 immutable-storage 模式。 |
| 2 | **Med** | AI 钩子（§4.7 DR Drill 失败→PRD-003、§4.8 Posture→AI 建议）未提脱敏/出境治理。 | 复用 SECURITY.md §6 + PRD-003 §7.2 统一管线，与 PRD-005/006 同口径，PRD 里显式引用。 |
| 3 | **Med** | DoD #2 仅验"target 对象 sha256 == source"，等于只验证了对象搬运，未验证可恢复性。 | 改/补为"从 target BSL 完整 Restore + 卷数据一致"（与 P1 Phase 0 联动）。 |
| 4 | **Info** | Lifecycle「追加不覆盖」遇到已有外部规则做 merge，跨云（S3 XML / Azure / GCS JSON）语义不同，易冲突。 | 预览页按 BSL 类型展示 merge 后完整规则 + 冲突检测（同 prefix 多 transition 冲突时报错而非静默追加）。 |
| 5 | **Info** | Fingerprint 用 kube-system ns UID 作 cluster_id 是合理稳定标识，但 ns 被重建/迁移时会变。 | 文档注明该 ID 的生命周期假设；TrustStore 记录绑定时间便于排查。 |
| 6 | **Info（赞）** | Phase 0 verify-before-architect + rclone 不达标回退 S3↔S3 的硬退路，是范本级风险前置。 | 把 P1 的"复制后可恢复性 + CSI 快照排除"和 P2 的"Glacier thaw"也纳入 Phase 0 验证清单。 |

---

## 四、跨 PRD / 跨 ADR 一致性

- **与 ADR-031 一致**：Layer 4 走 path (c) Backup-object 复制，正是 ADR-031 推荐的轻量路径；ADR-031 §1 已实测区分"K8s 快照对象移除 vs 存储侧快照保留"，P1 的"快照型数据不在 BSL"正是该结论的直接推论，应在 PRD-007 显式引用 ADR-031 把边界讲死。
- **与 PRD-003**：共享 `internal/resilience/` 引擎方向对，但 Score 口径需 P5 澄清。
- **与 PRD-006**：BackupCopy / DRDrill / IntegrityCheck 作为新 ActionType 进 Activity Timeline——正确复用，注意这三种的阶段定义要进 PRD-006 §4.1 合成器的 fixture 范围。
- **与 PRD-002**：DR Drill 还原时复用 Transform（NodePort→ClusterIP、Ingress 改写）——正确，建议直接用 PRD-002 的 TransformSet 编译产物，而非在 drdrill 里另写一套 patch。
- **与 SECURITY.md**：§4 Object Lock / §5 3-2-1-1-0 已有基线；§6 AI 出境治理应覆盖本 PRD 的 AI 钩子（Med #2）。

---

## 五、建议的行动优先级

| 序 | 行动项 | 时机 |
|---|---|---|
| 1 | §4.3 钉死 Layer 4 边界：仅 BSL-resident 数据可复制，快照型排除；粒度 = `kopia/<ns>` 仓库 + 元数据；Phase 0 加"复制后可恢复 + CSI 快照排除"E2E（P1） | 研发前 / Phase 0 |
| 2 | Lifecycle：数据转 Glacier 标 RTO 警告；delete 时间 ≥ Object Lock 保留期预检（P2） | 研发前定方案 |
| 3 | Fingerprint：跨集群默认强制签名（密钥不入 BSL），重述"防篡改"威胁模型绑 WORM/签名（P3） | 研发前 |
| 4 | DR Drill：default-deny egress NetworkPolicy 提为 v1 必做；外部生产依赖告警 + 端点重写（P4） | 研发前 |
| 5 | 厘清 Resilience Score vs Posture Score 是同一指标还是两个指标，修正 DoD #17 措辞（P5） | 研发前 |
| 6 | AI 钩子纳入 SECURITY §6 / T4 出境治理；DoD #2 改为可恢复性验证（Med #2/#3） | 研发期内 |
| 7 | Object-Locked target 上 Kopia 维护行为 Phase 0 实测（Med #1） | Phase 0 |

*总体评价：方向对、与 ADR-031 自洽、Phase 0 纪律好。但 Layer 4 的复制语义（P1）是必须在 Phase 0 用"复制后真正恢复出卷数据"证伪/证实的承重柱，连同 Glacier RTO、Fingerprint 威胁模型、DR Drill 外部副作用、Score 口径四条 High 一并收口后，PRD-007 即可进入分阶段研发。*

---

## 附录：技术核实要点

- Velero 卷数据按命名空间存于共享 Kopia 仓库 `bucket/kopia/<ns>`（内容寻址 + 去重）；Velero 原生**不支持单备份跨多个 BSL**，跨区域官方推荐云原生复制（如 S3 CRR）。⇒ Layer 4 需按仓库/BSL 级 sync 建模，且仅对 BSL-resident 数据有效。
- `snapshotMoveData=false` 的原生 CSI 快照备份：BSL 仅存 K8s 资源 + 元数据，卷数据在云厂商区域快照中（ADR-031 §1 已实测区分）⇒ 不能用 BSL→BSL 对象复制搬运其数据。
- Glacier / Archive 层对象不可直接读，恢复前需 thaw（数小时）⇒ 把备份数据冷归档会显著拉长 RTO。
- S3 Object Lock(WORM) 保留期内对象不可删除/改写 ⇒ 与 lifecycle delete、与 Kopia 维护（compaction）存在冲突，需协调。

**来源**
- [Backup Storage Locations and Volume Snapshot Locations — Velero Docs](https://velero.io/docs/v1.9/locations/)（单备份不能同时送多 BSL）
- [Backup Repository Configuration — Velero Docs](https://velero.io/docs/main/backup-repository-configuration/)（Kopia 仓库按 ns：`bucket/kopia/<ns>`）
- [How to Use Multi-Region Velero Backup Replication](https://oneuptime.com/blog/post/2026-02-09-velero-multi-region-replication/view)（跨区域推荐云原生 S3 CRR）
- [Exporting a Single Velero Backup to Another S3 Location? · velero Discussion #9348](https://github.com/vmware-tanzu/velero/discussions/9348)（单备份跨 BSL 导出是已知非平凡问题）

---

<!--
评审元数据（勿删）
- 评审轮次：PRD-Review 第三份（PRD-007）
- 关键结论：P1 Layer4 Kopia 复制语义/快照排除(High·承重)；P2 Glacier RTO + Object Lock 冲突(High)；P3 Fingerprint 威胁模型(High)；P4 DR Drill 外部副作用(High)；P5 Score 口径与 PRD-003(High)。方向通过 / Phase 0 扩测后定。
- 关联：ADR-031（5 层）、PRD-003（Score 共享）、PRD-006（新 ActionType）、PRD-002（Drill 复用 Transform）、SECURITY §6（AI 出境）
- 下次评审：在 PRD-Review/ 新建 PRD-Review-YYYY-MM-DD[-范围].md
-->
