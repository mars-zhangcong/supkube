# SupKube 服务目标：RTO / RPO / SLO

> **用途**：把「我们承诺多快恢复、最多丢多少数据、产品本身可用性多少、撑多大规模」**写成明确数字**——这些此前散在脑子里。它是 **PRD-011 AI Backup Advisor 评分校准的输入**（Resilience Score 的「够不够好」要有基准），也是销售/SLA 谈判与容量规划的依据。
> **读者**：PM、SRE、销售/售前、写 Resilience Score 规则引擎的研发。
> **关联**：[术语表.md](术语表.md)（RTO/RPO/Resilience Score/Posture）· [PRD.md](PRD.md)（PRD-007 五层韧性 / PRD-011 评分）· [架构设计.md](架构设计.md)（ADR-031 五层模型）· [RUNBOOK.md](RUNBOOK.md)
>
> ⚠ **状态**：本文件给出**目标基准（target）**，多数尚未端到端基准测试坐实（标 `🎯 目标` vs `✅ 实测`）。**Resilience Score 规则引擎应引用本文件的阈值常量**，单点修改、全局生效（单一来源纪律）。

---

## 0. 三个词，别混（详见术语表）

| 词 | 问的问题 | 谁的指标 |
|---|---|---|
| **RPO**（Recovery Point Objective） | **最多丢多少数据**（时间窗） | 客户数据 |
| **RTO**（Recovery Time Objective） | **多久恢复可用** | 客户数据 |
| **SLO**（Service Level Objective） | **SupKube 软件本身多可用/多快** | SupKube 产品 |

> RPO 由**备份频率**决定（每小时备 → RPO≈1h）；RTO 由**还原耗时**决定（数据量 + 通道 + 层级）。SLO 是 SupKube 控制面/API 自己的承诺，跟客户数据无关。

---

## 1. RPO 目标（最多丢多少数据）

RPO ≈ 备份间隔。SupKube 支持的档位与**建议默认**：

| 保护强度 | 备份间隔 | RPO | 实现 | 适用 |
|---|---|---|---|---|
| 持续保护 | Continuous（ImportPolicy time.Ticker） | **分钟级** | PRD-009 Continuous 模式 | 核心 DB / 高价值有状态 |
| 高频 | 每 1h | ~1h | Scheduled cron | 业务库 |
| 标准 | 每 24h | ~24h | Scheduled cron（默认） | 一般应用 |
| 低频 | 每周 | ~7d | Scheduled cron | 冷数据 / 合规留档 |

**校准点（喂给 PRD-011 评分）**：
- RPO ≤ 1h → Protection 维度高分。
- RPO > 24h **且** 应用被判为高业务价值 → 触发降分/告警（与「高分但无备份封顶 30」同源的校准逻辑，PRD-011 §4.2）。
- **无任何备份策略** → Protection 维度归 0，整体风险封顶 CRITICAL。

---

## 2. RTO 目标（多久恢复可用）

RTO 由「数据量 × 恢复路径层级」决定。按 ADR-031 五层模型，**层级越低恢复越快**：

| 恢复来源（层级） | RTO 目标 🎯 | 决定因素 |
|---|---|---|
| **Layer 1 本地快照** | 分钟级 | 同集群 CSI 快照回滚，不过网络 |
| **Layer 2 备份（Snapshot+Export）** | 10 分钟 ~ 小时级 | 从 BSL 拉数据 + node-agent data path 落盘 |
| **Layer 3 异地** | 小时级 | 跨区/跨云拉取带宽 |
| **Layer 4 Backup Copy（第二云）** | 小时级+ | 第二云通道 + 可能先回拉 |
| **Layer 5 DR Drill（虚拟实验室）** | 演练验证，非实时 RTO | 证明 RTO 真能达成 |

**经验公式（粗算，用于 Advisor 估算与销售沟通）**：
```
RTO ≈ 固定开销(调度+建 PV+绑定, ~3-5min)
     + 数据量 / 有效恢复吞吐
有效恢复吞吐 ：本地快照 ≫ 同区 BSL > 跨区 > 跨云
```

**校准点（喂给 PRD-011 评分）**：
- 只有 Layer 2、无 Layer 1 → 小数据也得分钟级以上，RTO 维度中等。
- 有 Layer 1+2+3（3-2-1 达成）→ RTO 维度高分。
- **DR Drill 从未跑过（Layer 5 缺）** → RTO 是「纸面值」，置信度降为 **low**（confidence 三档，不报百分比）——Advisor 应明说「未经演练验证」。

---

## 3. SupKube 产品 SLO（软件本身）

> 这是 **SupKube 控制面**的承诺，不是客户数据的。alpha 阶段为**内部目标**，未对外 SLA 化。

| SLO | 目标 🎯 | 说明 |
|---|---|---|
| 控制面 API 可用性 | 99.5%（alpha 内部目标） | backend `/api/v1/*` |
| `/api/v1/status` 探针成功率 | 99.9% | 公开探针，须比业务 API 更稳 |
| API 读延迟 p95 | < 300ms | 列表/详情（不含云侧聚合） |
| API 读延迟 p99 | < 1s | |
| 跨集群聚合（MCM summary）p95 | < 2s | 扇出多集群，目标更宽 |
| 备份**发起**确认延迟 | < 2s | 「点了有反馈」（客户痛点 C-013）——是发起确认，非备份完成 |
| 控制面恢复（RTO of SupKube itself） | < 10min | SupKube 自身被重装/迁移后恢复管理能力 |

**注**：SupKube 控制面挂掉**不影响已存在的备份数据**（数据在 BSL/云，不在控制面）；控制面是管理/编排层。这点要对客户讲清楚（降焦虑）。

---

## 4. 规模上限（Scale Limits）

> alpha 阶段的**已知验证范围 + 设计目标**。超出范围不等于不行，但未验证 → 标注清楚，别承诺。

| 维度 | 已验证 ✅ | 设计目标 🎯 | 备注 |
|---|---|---|---|
| 纳管集群数（MCM） | 双集群（dual-cluster e2e） | 10+ | 聚合 API 扇出，超大规模需评估 |
| 单集群 namespace/应用数 | demo 级 | 数百 | 应用列表分页 |
| 还原点（RP）总数 | 百级 | 数千 | 删除是异步级联（见 RUNBOOK §2） |
| 单备份卷数据量 | demo 级 | 取决于 node-agent 吞吐 | 大卷 RTO 线性增长 |
| 并发备份/还原 | 个位数 | 受 velero/node-agent 并发限 | 不要假设无限并发 |

> 这些数字**多为目标值**——`✅ 实测` 列要靠后续基准测试逐步填实。**Advisor 不得把目标值当实测值报给客户**（finding #2/#3 精神：可复现 + 置信度诚实）。

---

## 5. 这些数字怎么进 Resilience Score（PRD-011 接口）

Resilience Score 规则引擎（Go，确定性可复现）按维度算分，**阈值常量引用本文件**：

| 评分维度 | 本文件喂的输入 | 规则示例 |
|---|---|---|
| Protection | §1 RPO 档位 | RPO≤1h→满分；无备份→0 + 整体封顶 CRITICAL |
| Operation/Recovery | §2 RTO 层级 + DR Drill 跑没跑 | 达成 3-2-1→高分；未演练→置信度 low |
| Architecture | §4 是否在已验证规模内 | 超设计目标→提示「未验证规模」 |

> **单一来源**：阈值（`RPO_GOOD=1h`、`RPO_BAD=24h`、规模上限…）只在本文件定义，规则引擎引用常量，不要在代码里另写一份魔数（ENGINEERING.md Rule C）。改目标 → 改这里 → 评分自动跟随。
> **LLM 不参与算分**：分数由规则引擎出，LLM 只解释「为什么这个分 / 怎么改善」（finding #2）。

---

## 6. 待办（把目标坐实）

| 项 | 现状 | 做法 |
|---|---|---|
| RTO 各层实测 | 🎯 目标值 | 用 `hack/capture-velero-fixture.sh` + 定量数据集基准测试，填 §2 `✅ 实测` |
| RPO Continuous 下限 | 设计 | 压测 ImportPolicy Continuous 最小安全间隔（呼应 `ERR_IMPORTPOLICY_INTERVAL_TOO_SHORT`） |
| 规模上限 | 双集群验证 | 多集群/大 RP 量压测，填 §4 |
| SLO 监控 | 无 | Prometheus 抓 API 延迟/可用性（呼应客户痛点 C-009 Data Usage Report / task #97） |

---

## 变更记录

| 日期 | 操作人 | 变更 |
|---|---|---|
| 2026-06-01 | Claude | 初版。区分 RPO/RTO/SLO 三词；给 RPO 档位（Continuous→分钟级 ~ 周级）、RTO 按 ADR-031 五层（Layer1 分钟 → Layer4 小时+）、SupKube 产品 SLO（99.5%/p95<300ms 等内部目标）、规模上限（双集群已验/10+ 目标）。明确作为 PRD-011 Resilience Score 评分阈值的单一来源，目标值 vs 实测值严格区分（不把目标当实测报）。 |
