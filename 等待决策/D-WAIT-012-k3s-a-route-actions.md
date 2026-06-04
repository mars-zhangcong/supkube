# D-WAIT-012 — A 路线（K3S HomeLab）就绪的 2 个外部动作（DR 闭环 Phase 1 · 轨道 1 结论）

> **状态**：open（需 Mars 做 2 个外部动作：A-1 凭据 / A-2 版本）｜**owner**：Mars｜**触发**：2026-06-03
> **取号说明**：本条**迁自** FDE 文件 [`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md`](../engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md) 末尾「A 路线（K3S）就绪的 2 个外部动作」段——FDE 原文**未给它 D-WAIT 号**。因这两个动作同样 waiting-on-Mars，2026-06-04 SCM 经 LEDGER 正式 **home 为 D-WAIT-012**（Mars 2026-06-04 拍板 #3：不让 waiting 项成孤儿）。原 FDE 文件按边界保持原状未改。决策内容原样保留，未改。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **来源**：`FDE DECISIONS-FOR-MARS.md` 末段「A 路线 K3S 2 外部动作」（原无号）→ **D-WAIT-012**

- **A-1 凭据**：K3S HomeLab 经 **Tailscale `100.68.20.72:6443` 现在可达**（隧道/TLS 已验证），仅本地 admin client cert 过期被拒。需 Mars 在 `homelab-mbp-2` 上 `sudo cat /etc/rancher/k3s/k3s.yaml`、把 `server:` 改写为 `https://100.68.20.72:6443`，并入本机 kubeconfig（新 context，勿覆盖既存）。~5 min。
- **A-2 版本**：K3S 上 SupKube = **`0.9.1.9-alpha`（已发布版，无 ImportPolicy/fingerprint）**。A 跑指纹/导入腿前，K3S 端须升级到含命脉能力的 **dev 镜像**（与 dev/test 同源）。
