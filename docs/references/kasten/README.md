# Kasten K10 / Kanister 参考材料库

> 用途：SupKube 对标开发的参考素材。采自客户 Kasten 环境（K10 **v8.5.9**，`kasten-io`，cluster `aks-jumborca-dev`），采集日 **2026-06-10**。
> 来源声明：K10 CRD = 可观测 API 数据模型（竞品对标/互操作研究）；Kanister 系 **Apache-2.0** 开源，可直接复用。

## 目录
```
kasten/
├── README.md                ← 本文件（索引 + 对标地图 + 待补清单）
├── crd-schemas/             ← 21 个 K10/Kanister CRD 的完整数据模型(YAML)  ★最值钱
└── live-instances/          ← 活实例样例(policies×3 / profile×1 / vm-blueprint×1 …)
```

## 一、CRD 数据模型 → SupKube 对标地图

| Kasten CRD（crd-schemas/） | 是什么 | SupKube 对应 / 借鉴点 |
|---|---|---|
| `policies.config.kio.kasten.io` | 保护策略数据模型（freq/action/selector/retention） | 已有；对模型字段（271KB schema 最全） |
| **`policypresets.config.kio`** | **策略预设组**（freq+location 可复用） | ✗ 借鉴 → Presets 特性（拟并 PRD-009） |
| **`transformsets.config.kio`** | 变换集模型 | 已有(PRD-002)；比对字段 |
| **`blueprints.cr.kanister.io`** | **Kanister 蓝图**（phases/func/逻辑备份） | ✗ **头号借鉴** → 应用一致性备份新 PRD |
| `actionsets.cr.kanister.io` | 蓝图执行实例（ActionSet） | ✗ 同上，执行模型 |
| **`blueprintbindings.config.kio`** | **蓝图↔应用 标签自动绑定** | ✗ 借鉴 → 蓝图自动挂载 |
| `profiles.config.kio` / `profiles.cr.kanister.io` | 位置档/凭据（K10 + Kanister 两套） | 已有(BSL)；对凭据模型 |
| **`filerecoverysessions.datamover.kio`** | **文件级恢复 FLR**（浏览/单文件还原） | ✗ **新能力** → 候选新 PRD |
| **`repositoryserverrequests.datamover.kio`** | **Kopia 仓库服务器**（datamover 后端） | ✗ 我们 Velero 也用 Kopia → safecli-kopia 可借 |
| **`storagesecuritycontexts.config.kio`** + bindings | **存储操作的安全上下文**（runAsUser 等隔离） | ✗ 借鉴 → 数据搬运 Pod 安全加固 |
| `auditconfigs.config.kio` | 审计配置模型 | ◐ 对标(PRD-021 审计留存) |
| `k10clusterroles.auth.kio` + bindings | RBAC 角色/绑定模型 | ◐ 对标细粒度 RBAC（8 内置角色） |
| `reports.reporting.kio` | 报表模型 | ✗ 借鉴 → Reports 特性 |
| `actionpodspecs` / `actionpodspecbindings` | 动作 Pod 规格（资源/亲和/注入） | ✗ 借鉴 → 执行 Pod 可配 |
| `bootstraps`/`clusters`/`distributions`.dist.kio | K10 自身分发/多集群引导 | 参考（多集群我们已有） |

## 二、本次采集暴露的"新能力"发现（我们没有的）

1. **文件级恢复（FLR）** — `filerecoverysessions` + `repositoryserverrequests`：不整包还原，挂载备份仓库、浏览目录、捞单个文件。运维高频刚需，SupKube 完全没有。
2. **Kopia 仓库服务器（datamover）** — 独立的 Kopia repo server 做去重数据搬运（UI Data Usage 显示**去重比 274.2x**）。我们 Velero 也走 Kopia，`kanisterio/safecli-kopia` 可直接借。
3. **PolicyPresets** — 策略预设组，免重复配 freq+location。
4. **StorageSecurityContext** — 给数据搬运 Pod 配安全上下文（非 root/指定 uid），合规加固。
5. **VM 备份蓝图**（见 live-instances/blueprints…yaml）— K10 用 Kanister 蓝图做 KubeVirt VM 备份（phases: BackupData/BackupDataToServer/RestoreData），**直接是 PRD-025 的参考实现**。

## 三、Kanister GitHub（Apache-2.0，可直接用）

org: https://github.com/orgs/kanisterio/repositories

| 仓库 | 用途 | 我们怎么用 |
|---|---|---|
| **`kanister`** | 框架本体（Go，CNCF Sandbox） | 评估集成 operator vs 借模型 |
| **`blueprints`** | **社区数据库蓝图库**（PG/MySQL/Mongo/Cassandra/ES/Redis/etcd…） | ★直接拿蓝图，覆盖非 KB 自带库应用 |
| **`datamover`** | 数据搬运抽象 | 对标数据路径 |
| **`safecli`** + **`safecli-kopia`** | CLI 参数构建 + **Kopia 集成助手** | ★我们 Velero 用 Kopia，直接复用 |
| `kanister-charts` | Helm chart | 部署参考 |
| `errkit` | Go 错误库 | 可选 |

## 四、还需手工补的材料（kubectl 抓不到的 UI/UX 与 API）

> 我能抓 CRD/实例；以下交互流程与 API 响应需要你在 Kasten UI 上操作时**保存**给我：

- [ ] **建策略向导**（Create New Policy 全流程截图/录屏）— Snapshot+Export、retention、preset 选择
- [ ] **Blueprint 编辑器 / Create New Blueprint** 表单 — 看它怎么填 phases/func
- [ ] **Create New Binding**（蓝图绑定）表单 — 标签匹配 UI
- [ ] **Restore 向导**（含 FLR "浏览文件"界面）— 这是我们最缺的交互参考
- [ ] **Reports** 页渲染（报表类型/导出）
- [ ] **dashboardbff REST API 响应**（浏览器 F12 → Network → 存 `*.json`）— 这是 API 契约金矿，比 HTML 有用得多
- [ ] **Create New Policy Preset** 表单
- [ ] （可选）各页 HTML — SPA 空壳，价值低；优先存上面的 API JSON

材料放 `docs/references/kasten/manual/`（自建子目录），按 `NN-页面名/` 组织即可。

---
**采集人**：Claude（2026-06-10）｜**下一步建议**：先立"应用一致性备份(Kanister)"PRD（参考 `blueprints.cr.kanister.io` 模型 + github `kanisterio/blueprints`）。
