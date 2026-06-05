# DR 闭环 Phase 1 — 合并报告（4 轨道 · 供 Mars 10 min 审）

**日期** 2026-06-03 · **FDE** Auto · **环境** Azure Sub-RnD + aks-jumborca-dev/test + K3S HomeLab(Tailscale)
**身份** ea-rnd-mzhang@jumborca.net · **写操作** 本次**零集群写**（仅新建文件 + 只读探测）

## TL;DR
- **两根命脉支柱代码 = 活的**（轨道 2 实证）：dev/test 两个 dev 镜像都注册了 8 条 `/import-policies` 路由 + `[importpolicy] controller started` + `[fingerprint] runner started`。Gate 0 的黄旗**解除**。
- **A（K3S）不是死的**：之前的「不可达」是 kubeconfig 里的**过期 LAN IP**。HomeLab 经 **Tailscale `100.68.20.72:6443` 现在可达**，差一个 Mars 动作（刷新凭据）+ K3S 端升级 dev 镜像。
- **B 现在跑不了**，但**不是因为我缺权限**——`aks-jumborca-test` 的 **Velero 损坏**（Init:0/2 38h，无 azure 凭据 secret，BSL 非 Available）。这是首要阻断。
- 制品全部 ready + dry-run 通过；B 一条命令可触发（`preflight-b.sh` 守门，现在正确地拦住）。

---

## 🚦 Gate 0（冒烟两根支柱）— 复核结论：**支柱代码 GO；端到端冒烟仍 NO-GO（环境）**

**· 做了什么**：只读探测 4 个 context；定位两个 AKS 的 SupKube/Velero/BSL/CRD；`logs --since=48h` 抓控制器启动；`az storage blob list` 验权限；`auth can-i` 验写范围；Tailscale/网络探活 K3S。

**· 客观证据**
| 支柱 | 证据 | 判定 |
|---|---|---|
| ImportPolicy 控制器在跑 | dev `4e27f3c7` & test `3b0753b4` 日志均：8 条 `/api/v1/import-policies` GIN 路由 + `[importpolicy] controller started (manager interval 30s)` | **ALIVE** |
| fingerprint runner 在跑 | `[fingerprint] source cluster: ...(0363fe09...062c)` (dev) / `(4a724055...e829)` (test) + `runner started (poll 30s)`；id == kube-system uid | **ALIVE** |
| 能从 Azure 列举+导入 | 前置具备（dev BSL `azure-blob`@velero=Available，有 Completed export 备份），**但**唯一可导入的是既存 `aks-app-backup-*`（非隔离、同源），未实跑 | **未端到端验**（待隔离 test-* 备份） |
| fingerprint enforce/篡改 | 代码活；但需隔离 test-* 备份 + 我缺 Storage Blob Data Reader 旁证 | **未端到端验** |

**· GO / NO-GO**：支柱代码 **GO**（黄旗解除）；端到端冒烟 **NO-GO** —— 目标侧 Velero 坏 + 缺存储读权限，按铁律 stage 不跑。

**· 偏差（vs Phase 0 假设）**
1. 命脉能力镜像**已部署在 dev/test**（4e27f3c7 / 3b0753b4），无需现造 dev 镜像；但 **K3S 仍是 0.9.1.9-alpha（无该能力）**。
2. ImportPolicy 过滤字段是**扁平 `spec.sourceClusterID`**，非 `sourceClusterFilter.clusterId`（Phase 0 brief 写错）。
3. 指纹字段是 **`tarballSHA256`**，非 `rp_sha256`（brief 写错）→ 验数脚本已改。
4. `${VAR}` 仅 TransformSet 编译期支持，独立 Transform 不能用 → SC 转换拆成 fwd/rev 两个具体 Transform。
5. 镜像改写 Velero 只支持**精确值替换**（无前缀/正则）→ 逐镜像枚举。
6. dev/test **velero 命名空间不一致**（dev=`velero` ns 且 Available；test=`supkube` ns 且坏）→ 增加 runbook 复杂度。

**· 请示**：见各 D-WAIT（DECISIONS-FOR-MARS.md）。

---

## 轨道结论

### 轨道 1 — A 连通方案 ✅（差 2 个 Mars 动作即 A-ready）
- HomeLab = `homelab-mbp-2`（Mac+colima→k3s），**Tailscale `100.68.20.72:6443` 实测可达**（nc 通、TLS SAN 含该 IP、SupKube UI `0.9.1.9-alpha` 在 :30888 应答）。"10.17.28.15 不可达" = 本机在 iPhone 热点、kubeconfig 存的旧 LAN IP。
- **A-1**：Mars 在 homelab 刷新 kubeconfig（server→`https://100.68.20.72:6443`）并入本机（新 context）。~5min。
- **A-2**：K3S 端 SupKube 升级到 dev 镜像（当前 0.9.1.9 无 ImportPolicy/fingerprint）。
- 备选：在 test 装 Longhorn 造真跨 CSI 对端——可行但与 Kasten K10 同集群有争用风险，不在读权限内，不建议除非 homelab 凭据拿不到。
- **跨 CSI（Longhorn↔managed-csi）= GA 缺口**，B 范围内验不了（两端都是 Azure disk），留给 A。

### 轨道 2 — 控制器存活判定 ✅ ALIVE（强证据，见上表）
- 之前「日志 0 命中」是 `--tail=2000` 太短（日志 48h 内 7702 行）；`--since=48h` 后铁证。再次印证 verify-don't-trust。

### 轨道 3 — 制品包 ✅ 全部 dry-run 通过
- 路径 `engineer-testing/dr-loop-phase1/`：`manifests/`(4) + `transforms/`(4) + `importpolicy/`(2，已填真 cluster id/BSL/ns) + `scripts/`(4 验数 + `preflight-b.sh` + `run-b.sh`) + `README-runbook.md`。
- 占位符已用实测值填实：dev id `0363fe09...062c`、test id `4a724055...e829`、`sourceBSL: azure-blob`、ACR `supkube.azurecr.io`、velero ns 按集群区分。
- 剩余唯一硬编码假设：sourceClusterID 用**无横线**形（匹配 fingerprint 日志），若 enforce 首跑因格式拒绝则切横线形（已在注释标注）。

### 轨道 4 — B 已 stage + 阻断已记 ✅
- `preflight-b.sh` 实跑结果：**B-1 存储读权限 ✗ / B-2 velero ✗ / B-3 BSL ✗ / B-4 K10 隔离 ✓** → 正确拦住 B。
- 写权限**不是**阻断：我在 test 集群 `auth can-i '*' '*'` = yes（cluster-admin）。
- 阻断项 = D-WAIT-004（存储 reader）+ D-WAIT-005（test velero 修复，首要）+ D-WAIT-006（K10 隔离确认）。

---

## 验收锚映射（5+5→10，每步怎么客观验）
| 手段 | 工具 | 现状 |
|---|---|---|
| (1) 条数 SELECT count(*) | `verify-count.sh`（kubectl exec 进 pg，psql 本机没装） | ready |
| (2) pg_dump\|sha256 跨集群 | `verify-pgsha.sh` | ready |
| (3) fingerprint tarballSHA256+sourceClusterID 双向匹配 | `verify-fingerprint.sh` | ready，**但需 D-WAIT-004 存储读权限** |
| (4) ns 不卡 Terminating | `verify-ns-not-terminating.sh`（+ lb-to-clusterip Transform 兜底） | ready |

## 下一步（解锁顺序）
1. **修 test velero + 给我 storage reader**（D-WAIT-005 + 004）→ `preflight-b.sh` 转绿 → `./run-b.sh` 走 B Gate 0→2。
2. 并行 **A-1 刷 kubeconfig + A-2 升 K3S dev 镜像** → 真跨云跨 CSI（GA 真闸门）。
3. B 跑通后产出 TC-XCR-* 实测用例草稿 + GA 缺口确认清单（跨 CSI builtin、registry builtin、LB→ClusterIP builtin、test velero 配置债）。

**GA 缺口（已确认，必须编码/补测才能上市）**：① 三个 builtin 缺位（SC 改名 / 镜像改写 / LB→ClusterIP）现靠 custom Transform 兜底 ② 跨 CSI 真实验证缺（B 验不了）③ TC-XCR-* 测试用例缺 ④ 环境配置债（test velero 凭据、velero ns 不一致、等待决策.md 冲突标记）。
