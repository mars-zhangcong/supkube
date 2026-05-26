# SupKube Product Tiers

> Last updated: 2026-05-26
> Audience: sales, partners, license-server backend, frontend feature-gating, customers comparing tiers.
> Companion docs: [ROADMAP.md](ROADMAP.md), [架构设计.md](架构设计.md), [USER_MANUAL.md](USER_MANUAL.md).

---

## TL;DR — 一句话说三层

```
Foundation  · 免费 / 入门 · 备份恢复闭环 + 多集群 + Helm 一键装
Advanced    · 付费 / 主流 · + 安全 / 可观测 / 合规 / 排障工具链
Premium     · 旗舰 / 灾备规划 · + BCM / BIA / RA / 自动化恢复编排 + AI Assistant
```

**定价锚**：Kasten K10。我们的功能上做"超集"，定价做"下沉" —— 把 Kasten 漏斗里的每一段客户都接住。

---

## 1. 设计哲学

### 1.1 三个不变量

| 不变量 | 含义 |
|---|---|
| **Open architecture** | 所有数据走 Velero CRD + K8s native API，不引入私有数据格式 / 私有协议。客户可以随时撤换 SupKube，备份数据继续在 Velero 工具链可读。 |
| **No vendor lock-in** | 镜像在公开 ACR (anonymous pull)，chart 在公开 charts.supkube.com，所有 CRD 是 `supkube.io` group 但语义和 Velero 1:1 映射，可一键导出迁移。 |
| **Self-contained UI** | 不要求客户已部署 Prometheus + Grafana 才能看观测。可观测性做进 UI（v0.8.12.5 DR Topology / v0.8.14 Log Viewer），同时也 expose Prometheus metrics（v1.0 GA）让有栈的客户复用。 |

### 1.2 三层之间的边界规则

| 规则 | 说明 |
|---|---|
| **Foundation 永远是产品完整的** | 备份/恢复/跨集群恢复链路 100% 闭合。Foundation 客户不该有"功能缺胳膊少腿"的感觉。 |
| **Advanced 是"专业感"升级** | 提供合规审计、生产排障、企业认证、品牌定制——这些是企业 IT 标书里 checklist 项。 |
| **Premium 是"咨询服务化"升级** | BCM/BIA/RA/灾备编排——这些是过去要请咨询公司花 50 万搭的能力，我们做进产品。 |
| **跨级降级容忍** | Premium license 到期 → 自动降到 Advanced；Advanced license 到期 → 自动降到 Foundation。**数据永不丢**，只是 Premium-only 的 UI 入口锁掉。 |

---

## 2. 三层功能矩阵

> 状态符号：✅ 已 ship · 🔲 计划中（带版本号）· 🚫 不属于该层

### 2.1 Foundation（免费 / 入门）

**口号**: "Velero 该有的备份/恢复体验，开箱即用，永远免费"

| 模块 | 状态 | 当前版本 |
|---|---|---|
| 核心备份 / 恢复 / 跨集群恢复 | ✅ | v0.9.0 |
| 双 Schedule (Local BSL + Cloud BSL) + 3-2-1-1-0 | ✅ | v0.8.10 + v0.8.12 |
| Object Lock + 不可变保留期 | ✅ | v0.8.12 LBS3 |
| Multi-Cluster Manager + Cluster CRD + 健康检查 | ✅ | v0.9.0 MC1-2 |
| Cross-Cluster Restore Wizard + BSL auto-sync | ✅ | v0.9.0 MC3-4 |
| DR Topology Dashboard | ✅ | v0.8.12.5 |
| Helm bundling (Velero subchart + Local MinIO) | ✅ | v0.8.13 |
| 制品分发 (charts.supkube.com + ACR anonymous pull) | ✅ | v0.9.0.3 |
| Multi-arch (amd64 + arm64) | ✅ | v0.9.0.4 |
| Preflight script + Install Reference (USER_MANUAL §23) | ✅ | v0.9.1.0 |
| Backup integrity check (checksum + Kopia repo validate) | 🔲 v0.8.15 | |
| Swagger REST API doc | 🔲 v0.8.16 | |
| KubeVirt VM 备份恢复 | 🔲 v0.9.7 | 按需 |
| MCP Server (AI 集成基础设施) | 🔲 v0.9.8 | |
| Configuration Backup (备份 SupKube 自身配置) | 🔲 v0.9.10 | |

### 2.2 Advanced（付费 / 主流商业）

**口号**: "企业 IT 部门 checklist 的所有项 —— 一次买全"

| 模块 | 状态 | 当前版本 |
|---|---|---|
| White-label / Branding (产品名 + Logo + Favicon + Color Scheme) | ✅ | v0.8.11 |
| OIDC + Dex + 4 角色 RBAC + 4 IdP 集成 (Keycloak/Okta/Azure AD/GitHub) | ✅ | v0.8.5 |
| Audit Log (全 admin 操作落 K8s Event + 可查页) | ✅ | v0.8.5 |
| Cluster Hygiene / Orphan GC (孤儿 VSC/PVB/DataUpload 自动清理) | ✅ | v0.8.8 |
| Backup Advisor (智能评估每个 application 备份必要性) | ✅ | v0.7.5 |
| Log Viewer + Upload to Support (Datadog 风) | 🔲 v0.8.14 | 当前 sprint |
| `supkube_debug.sh` 一键 debug bundle | 🔲 v0.8.14 | 当前 sprint |
| Settings → Support Contact + System Information 页 | 🔲 v0.8.14 | 当前 sprint |
| License Manager 前端 (Kasten 1:1 复刻) | 🔲 v0.9.2 | |
| 细粒度文件浏览 + 恢复 (Kopia-only) | 🔲 v0.9.3 | |
| 企业安全栈 P1 (EntraID + Azure KeyVault / HashiCorp Vault) | 🔲 v0.9.4 | |
| 企业安全栈 P2 (Kyverno Policy 模板 + Audit channel 抽象) | 🔲 v0.9.5 | |
| Prometheus metrics + 官方 Grafana dashboard 发布 | 🔲 v1.0 | |
| Catalog Service + Fleet Software Install (统一 install/preflight/debug) | 🔲 v0.9.10 | 客户战略需求 |

### 2.3 Premium（旗舰 / 灾备规划）

**口号**: "把咨询公司 50 万的 BCM 流程，做进产品里"

| 模块 | 状态 | 当前版本 |
|---|---|---|
| BPMN.io 应用级恢复演练画布 | 🔲 v0.9.6 | "独创卖点" |
| BIA (Business Impact Analysis) wizard | 🔲 v0.9.6+ | NEW (Mars 2026-05-26) |
| RA (Risk Assessment) 评分卡 + 报告 | 🔲 v0.9.6+ | NEW |
| DR 应急接管流程画布 (BPMN 扩展) | 🔲 v0.9.6+ | NEW |
| Runbook 模式知识库 (100+ 规则编辑器) | 🔲 v0.9.6+ | v0.8.14 LV7 是种子 |
| 自动化 failover orchestration (Kanister 编排深化) | 🔲 v0.9.6+ | NEW |
| SupKube AI Assistant (chatbot 嵌入 UI，对标 Veeam Intelligence) | 🔲 v0.9.8+ | 基于 MCP Server |
| 多集群合规仪表盘 (compliance score 跨集群聚合) | 🔲 v0.9.5+ | |
| Microsoft Sentinel webhook (SIEM 接入) | 🔲 backlog | 触发: 客户启用 Sentinel |
| Veeam One / VBR 互操作 (license sync / event forward) | 🔲 backlog | T1 客户来源时考虑 |

---

## 3. 竞争定位（vs Kasten K10 v9）

### 3.1 Kasten 在做什么（基于 v9 发布资料）

| Kasten v9 特性 | 实现方式 |
|---|---|
| Multi-Cluster Observability via Red Hat ACM | 集成进 OpenShift Container Platform 监控栈 |
| Backup alerts via PagerDuty/Slack/ServiceNow | 通过 ACM alert manager 路由 |
| Pre-built alerts day-one ready | 出厂带 Prometheus alert rules |
| Cleaner audits | License usage + compliance 聚合视图 |
| Kasten Storage Classes 页（CSI snapshot type 探测） | 内置 |
| Veeam Intelligence AI (chatbot in UI) | Veeam Backup & Replication 13.0+ 已有，Kasten 还没接 |

### 3.2 SupKube 的差异化

| 维度 | Kasten v9 | SupKube |
|---|---|---|
| **可观测性栈** | 假设客户已有 Prometheus + Grafana + ACM；导出 metrics 让客户自部署 dashboard | UI 内置 DR Topology + Activity + Log Viewer，**无外部依赖**也可用；同时 expose Prometheus metrics 给有栈的客户复用 |
| **国际化** | 仅英文 | zh-CN ✅, ja-JP 路线图（v0.9.x） |
| **AI** | Veeam Intelligence 在 VBR (VM 备份) 有，Kasten 还没 | v0.9.8 MCP Server + 未来 native chatbot |
| **价格** | ~$5K-15K / cluster / year (enterprise) | Foundation 永久免费；Advanced 友好定价；Premium 按价值定价 |
| **生态** | Veeam 锁定（catalog、support、license server 全是 Veeam 内部） | 开源 / Velero 兼容 / 自托管 license server 可选 |
| **Velero 关系** | Kasten 私有引擎（不基于 Velero） | SupKube = Velero 的"产品化包装"，所有数据 Velero 工具链可读 |
| **可定制** | 闭源，customize 靠 Veeam 改 | 开源，客户可 fork |
| **Onboarding** | "follow our 5-step install doc" | `helm install` + Preflight 一行 curl-bash |

### 3.3 Mars 标记的目标客户分层

| 客户类型 | Kasten 给他们的痛点 | SupKube 接得住的理由 |
|---|---|---|
| **T1 不满 Kasten 的现有客户** | 厂商锁定 / 不能改源码 / 工单响应慢 / 产品路线图不透明 | 开源 / 自托管 / 客户能 PR / 路线图公开 |
| **T2 买不起 Kasten 的中型客户** | ~$5K/cluster/year 是中小 K8s 团队拒绝点 | Foundation 免费已覆盖核心；Advanced 友好定价（远低于 Kasten） |
| **T3 中文/日文市场** | Kasten 界面仅英文，文档英文 | 原生 zh-CN，文档双语，日文路线图 |
| **T4 想 AI 集成的客户** | Veeam Intelligence 刚出，仅 VBR 有，Kasten 没 | v0.9.8 MCP Server 让 Claude / OpenClaw 等 AI 直接调 SupKube |
| **T5 需要灾备规划而非仅备份的客户** | Kasten 是"backup-only"，BCM 流程要自己拼 | Premium 层 BPMN + BIA + RA 独有 |

---

## 4. License Gating 机制

### 4.1 数据流

```
admin/EULA acceptance → cm/supkube-eula (licenseKey 字段)
                           ↓
license server (v0.9.2) validates licenseKey
                           ↓
backend caches: tier = Foundation | Advanced | Premium
                           ↓
GET /api/v1/license/summary → frontend
                           ↓
useFeatureGate.js canUse(feature) → 渲染/锁定 UI 入口
```

### 4.2 前端 helper（v0.9.2 实现）

```javascript
// supkube-frontend/src/composables/useFeatureGate.js
import { computed } from 'vue'
import { useLicense } from './useLicense'

const FEATURE_TIER = {
  // Foundation
  backup: 'foundation',
  restore: 'foundation',
  crossClusterRestore: 'foundation',
  multiCluster: 'foundation',
  drTopology: 'foundation',

  // Advanced
  branding: 'advanced',
  auditLog: 'advanced',
  rbac: 'advanced',
  logViewer: 'advanced',
  backupAdvisor: 'advanced',
  fileBrowse: 'advanced',
  entraIdVault: 'advanced',
  kyverno: 'advanced',
  catalogService: 'advanced',

  // Premium
  bpmnRecoveryDrill: 'premium',
  bia: 'premium',
  ra: 'premium',
  runbookEditor: 'premium',
  aiAssistant: 'premium',
}

const TIER_ORDER = { foundation: 0, advanced: 1, premium: 2 }

export function useFeatureGate() {
  const license = useLicense()
  return {
    canUse: (feature) => {
      const required = FEATURE_TIER[feature]
      if (!required) return true   // 未列入 = 默认开放
      const current = license.tier.value
      return TIER_ORDER[current] >= TIER_ORDER[required]
    },
  }
}
```

### 4.3 UI 模式

```vue
<el-button v-if="canUse('logViewer')" @click="openLogs">
  View Logs
</el-button>
<TierLockOverlay v-else feature="logViewer" requiredTier="advanced" />
```

`TierLockOverlay` 渲染："This is an Advanced feature. [Upgrade to unlock →]"——不报错，仅引导。

---

## 5. 客户战略需求（2026-05-26 评估）

### 5.1 需求拆解

客户提出："把 Pre-flight Check / Debug / Collect Logs / Velero install / SupKube install 统一管理，Activity 统一记录。引入 Catalog Service (考虑 ETCD) + Configuration Backup。"

### 5.2 价值评估：**高**

| 维度 | 评分 | 理由 |
|---|---|---|
| 客户体验 | ⭐⭐⭐⭐⭐ | 终结"装好 SupKube 还要自己 kubectl apply CRD / helm install Velero / 手动 collect log"的碎片化 |
| 商业差异化 | ⭐⭐⭐⭐ | 把 SupKube 提到比 Kasten 更高一层 —— 不只是 backup，而是 "K8s 灾备就绪状态"管理平台 |
| 技术杠杆 | ⭐⭐⭐⭐ | 现有 Cluster CR + kubeconfig Secret + controller-runtime client 已是基础设施 |
| 市场切口 | ⭐⭐⭐⭐ | 切入"我有 10 个 cluster 但不想为每个手动 deploy & monitor"的客户群 |

### 5.3 可行性评估：**中-高**

#### Catalog Service 架构（不用 ETCD，用 CRD）

```yaml
# CatalogItem CR — 可安装的软件 bundle 注册
apiVersion: supkube.io/v1
kind: CatalogItem
metadata:
  name: velero-v1.18
  namespace: supkube
spec:
  type: helm-chart
  chart: vmware-tanzu/velero
  version: 9.0.4
  supportedK8s: [">=1.25"]
  prerequisites:
    - csi-snapshot-crd
    - snapshot-controller-running
  defaultValues:
    configuration.features: EnableCSI
    deployNodeAgent: true
  metadata:
    description: "Bundled Velero v1.18 with CSI/AWS/Azure plugins"
    tier: foundation
---
# InstallTask CR — 一次安装任务（每次 install / preflight / debug 各起一个）
apiVersion: supkube.io/v1
kind: InstallTask
metadata:
  name: install-velero-on-aks-dev-1716700000
  namespace: supkube
spec:
  catalogRef:
    name: velero-v1.18
  targetClusterRef:
    name: aks-jumborca-dev
  action: install   # install | upgrade | uninstall | preflight | debug | collect-logs
  overrideValues: {}
status:
  phase: Pending | Running | Succeeded | Failed
  startedAt: ...
  finishedAt: ...
  activities:
    - { time, type, message }    # 每一步都进 Activity 流
  artifactRef:                    # debug/collect-logs 产物指向
    type: tarball
    s3Path: "supkube-debug-aks-dev-...-tar.gz"
```

**好处**:
- 100% K8s native, no ETCD, no external state store
- Activity 自然集成 —— InstallTask 就是 Activity 一种新 actionType
- RBAC 用 K8s ClusterRole 直接限制谁能 create InstallTask
- 跨集群操作走 controller-runtime client（v0.9.0 MC3 已落地的模式）
- 客户审计：`kubectl get installtask -n supkube` 即可看全历史

**MVP 边界**:
- v0.9.10 起步：CatalogItem 仅 3 种 (velero / snapshot-controller / supkube-self)
- Action 仅 4 种：install / preflight / debug / collect-logs
- 上手路径：MCM Dashboard → 集群 row → kebab → "Install Software" / "Run Preflight" / "Collect Logs"

#### Configuration Backup 架构

SupKube 把自己的 CRD 当一个 namespace=supkube 的应用，走自己的备份链路 —— **dogfooding 是最好的灾备演练**。

```yaml
# 备份内容（namespace: supkube）
cluster.supkube.io           ← 集群注册表
policies.supkube.io          ← 备份策略
schedules.supkube.io         ← 调度
transformsets.supkube.io     ← 资源转换
backupstoragelocations.velero.io       ← BSL 注册（通过 reference）
volumesnapshotlocations.velero.io      ← VSL 注册
configmaps:
  - supkube-branding         ← 品牌定制
  - supkube-eula             ← EULA + license
  - supkube-support-contact  ← 支持联系人 (v0.8.14)
secrets:
  - cluster-*-kubeconfig     ← 集群凭据（明文 base64，恢复时 admin 确认）
  - bsl-*-creds              ← BSL 访问凭据
catalogitems.supkube.io     ← Catalog (v0.9.10)
installtasks.supkube.io     ← Activity 历史 (v0.9.10)
```

**恢复路径**:
1. 灾难后在新集群 `helm install supkube --set eula.accept=true`
2. 配 BSL → 看到上次的 "supkube-self-backup" RP
3. 点 Restore → 恢复 namespace `supkube` 全部对象
4. controller 自动 reconcile，所有注册的集群、策略、品牌、凭据回来
5. **灾备过程不超过 30 分钟，零数据丢失**

### 5.4 战略风险重评估（基于 Kasten v9 + 你提供的资料）

| 风险（重评估前） | 重评估后结论 |
|---|---|
| Scope creep：从备份产品扩展到 fleet management，定位竞品从 Kasten 变成 Rancher / Argo CD | **撤销**。Kasten v9 自己也在做（ACM 集成、catalog、observability）。这不是 creep，是行业方向。Rancher 是"通用 K8s 管理"，我们仍然以"备份与灾备"为锚点，往周边扩展是品类内深化，不是品类跳跃。 |
| 客户 1 个需求 ≠ 市场 100 个客户的需求 | 仍然成立，但 Mars 给的 5 类目标客户分层（T1-T5）已经显示市场分布，不是单一客户的偏好。**风险等级降为：低**。 |
| Premium 层 BCM 编排过于咨询服务化 | **重新框定**：BCM/BIA/RA 是金融、医疗、政企客户**强制合规要求**。把这部分做进产品 = 跳过他们采购咨询的需要。**这是机会，不是风险**。 |
| AI Assistant 路线图过晚 (v0.9.8) | **新风险**：Veeam Intelligence 已在 VBR 落地，Kasten 大概率 v10 跟上。**建议**：v0.9.8 MCP Server 可往前提，与 v0.9.3/v0.9.4 并行；MVP chatbot 用现成 AI 服务（Claude API）即可，不必等 SupKube 全功能。 |

---

## 6. 商业化路径（与 ROADMAP 对照）

### 6.1 当前（v0.9.1.0 已 ship）

| 类型 | 状态 |
|---|---|
| 公开试用版 | ✅ Foundation 已全开 |
| 可商业 demo | ✅ 端到端可演示 |
| License 收钱 | 🔲 v0.9.2 License Manager 前端完成后可启动 mock license；真实收费等后端就绪 |

### 6.2 v0.8.14 之后（本周）

下一个里程碑 = 客户 demo 闭环：
- ✅ 客户能装 (v0.9.1.0 preflight + EULA + Install Reference)
- 🔲 客户出问题能自助排障 (v0.8.14 Log Viewer + Upload to Support)
- 🔲 客户能买 (v0.9.2 License Manager 前端 + mock license 验证)

之后即可上 demo 客户。

### 6.3 三月内（v0.9.3 → v0.9.5）

| Sprint | 客户场景 |
|---|---|
| v0.9.3 文件浏览 | 不需要全 PVC restore，只想找一个误删的文件 |
| v0.9.4 EntraID + Vault | 企业 SSO 集成 + 凭据集中管理（合规标书必勾） |
| v0.9.5 Kyverno | 安全策略 enforcement（金融、医疗强制项） |

完成这三个后 = **Advanced 层完整**，进入主流商业 SKU 阶段。

### 6.4 六月内（v0.9.6 + v0.9.8 + v0.9.10）

| Sprint | 战略意义 |
|---|---|
| v0.9.6 BPMN 应用级恢复演练 | Premium 层独有，咨询服务替代品 |
| v0.9.8 MCP Server + native AI assistant | 对标 Veeam Intelligence，技术领先期 |
| v0.9.10 Catalog Service + Fleet Lifecycle Management | 从"备份产品"过渡到"K8s 灾备就绪平台" |

完成这三个后 = **Premium 层完整**，进入"咨询服务化"高毛利阶段。

---

## 7. 给销售 / 合作方的 talking points

### 7.1 当客户说"我们已经在用 Kasten 了"

- "Kasten 锁定 Veeam 商业栈。SupKube 是开源的，备份数据走 Velero CRD，你随时能撤。"
- "Kasten v9 的多集群观测要你自己搭 ACM + Prometheus + Grafana。SupKube UI 自带 DR Topology + Activity + Log Viewer。"
- "Kasten 界面只有英文。SupKube 原生 zh-CN。"
- "Kasten 还没上 AI Assistant，Veeam Intelligence 只在 VBR (VM 备份)。SupKube v0.9.8 直接对接 Claude / OpenClaw / 自家 chatbot。"

### 7.2 当客户说"Kasten 太贵了"

- "Foundation 永久免费且功能完整 —— 你可以先零成本跑起来。"
- "Advanced 定价是 Kasten 的零头。Premium 才进入企业 SKU 区间。"
- "灾备演练（BCM/BIA/RA）—— Kasten 没有，你过去要请咨询公司花 50 万搭。SupKube Premium 包含。"

### 7.3 当客户说"我们要 BCM / 合规 / 应急流程"

- "Premium 层就是为这个设计的。BPMN 画布编排灾备演练，BIA wizard 评估业务影响，RA 评分卡出审计报告。"
- "Kasten 的合规视角是被动的 (compliance score)。SupKube Premium 是主动的 (定期 drill + report)。"

---

## 8. 文档同步清单

本文档的决策落到这些地方：

| 决策 | 落点 |
|---|---|
| 三层 License gating 机制 | `supkube-frontend/src/composables/useFeatureGate.js` (v0.9.2 实现) |
| Catalog Service CRD 设计 | `架构设计.md` ADR-030 (v0.9.10 实现) |
| Configuration Backup 设计 | `架构设计.md` ADR-030 + `USER_MANUAL.md §24` (v0.9.10 实现) |
| EULA + License key 字段 | `supkube-helm/supkube/templates/eula-check.yaml` (✅ v0.9.1.0 已落地) |
| License Manager UI | `supkube-frontend/src/components/LicensePanel.vue` (v0.9.2) |
| Feature 列表与 Tier 映射 | 本文档 §2 + `useFeatureGate.js` 的 `FEATURE_TIER` 表 |

---

## 附录 A：引用资料

- [Kasten v9 release notes (Veeam blog)](https://www.veeam.com/blog/veeam-kasten-v9-enterprise-kubernetes-resilience.html)
- [Veeam KB4635 — Kasten observability stack](https://www.veeam.com/kb4635)
- [Grafana Dashboard 25200 — Kasten ACM Multi-Cluster Observability](https://grafana.com/grafana/dashboards/25200-veeam-kasten-red-hat-acm-multi-cluster-observability/)
- [Grafana Dashboard 21065 — K10 single-cluster](https://grafana.com/grafana/dashboards/21065-k10-dashboard/)
- [Veeam AI Chatbot announcement](https://community.veeam.com/blogs-and-podcasts-57/veeam-ai-chatbot-7455)
- [Veeam AI Online Assistant docs](https://helpcenter.veeam.com/docs/vbr/userguide/veeam_ai_online_assistant.html?ver=13)
