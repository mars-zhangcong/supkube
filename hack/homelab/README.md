# Homelab：把一台老 Mac 变成永不掉线的 SupKube 演示集群

> 目标：Intel + 16GB 的 Mac，保留 macOS，用 **Colima/k3s** 跑 K8s，部署 **SupKube**，
> 通过 **Tailscale** 从任意地方访问，做云上↔云下数据复制实验与客户 Demo。
> 设计原则：**永不掉线** —— 不睡眠、断电自愈、服务自启、远程不依赖公网 IP。

## 架构

```
                Tailscale tailnet (100.x，永不依赖公网IP)
                          │  tailscale serve (HTTPS)
   ┌──────────────────────┴─────────────────────────────┐
   │  老 Mac (macOS, 100.89.39.62) —— 永不睡眠/断电自启     │
   │   caffeinate 守护 + pmset autorestart                 │
   │   ┌──────────────────────────────────────────────┐   │
   │   │ Colima VM (Lima)  —— LaunchAgent 看门狗自愈     │   │
   │   │   k3s (单节点 K8s)                              │   │
   │   │     ├─ SupKube backend / frontend (NodePort)   │   │
   │   │     ├─ Velero v1.18                            │   │
   │   │     └─ 本地 MinIO (localStore，云下对象存储)     │   │
   │   └──────────────────────────────────────────────┘   │
   └──────────────────────────────────────────────────────┘
```

## 一次性安装（在那台 Mac 上）

```bash
# 0) 把本仓库 clone 到 Homelab Mac，进入仓库根目录
# 1) 永不掉线基础层：电源策略 + caffeinate 守护 + 体检
bash hack/homelab/setup-macos-always-on.sh
#    然后手动做两件 GUI 设置（脚本会提示）：
#      · 系统设置 → 用户与群组 → 自动登录 → 选你的账户
#      · 关闭 FileVault（或接受断电后需手动开机一次）

# 2) 装 Colima/k3s 自启 + 部署 SupKube + Tailscale 暴露
bash hack/homelab/install-supkube.sh
```

## 永不掉线的四道防线

| 防线 | 机制 | 文件 |
|---|---|---|
| 系统不睡眠 | `pmset -c sleep 0 disablesleep 1` + caffeinate 守护 | `setup-macos-always-on.sh`, `com.supkube.caffeinate.plist` |
| 断电自愈 | `pmset autorestart 1` + 自动登录 + FileVault 关 | `setup-macos-always-on.sh` |
| 集群自启/自愈 | LaunchAgent 每 60s 看门狗，Colima 挂了自动拉起 | `com.supkube.colima.plist`, `supkube-colima-up.sh` |
| 远程永在线 | Tailscale 系统守护进程 + `tailscale serve` HTTPS | （手动 `tailscaled install-system-daemon`） |

## 访问

- 本机：`http://localhost:30888`
- 任意地方（Tailscale）：`tailscale serve status` 输出的 `https://<host>.ts.net`
- 健康探针：`curl -s http://localhost:30888/api/v1/status` → `{status, version, buildStamp}`

## 云上↔云下复制 Demo

1. 生成一个共享指纹密钥（**两个集群用同一个**）：
   ```bash
   echo -n "$(openssl rand -base64 32)"
   ```
2. 在 `values-homelab.yaml` 取消 `fingerprint.sharedSecret` 注释、填入该值，重装/升级。
3. 云上集群（AKS 等）用相同 `--set fingerprint.sharedSecret=...` 安装 SupKube。
4. 在 SupKube UI 用 **ImportPolicy** 让云下集群拉取云上的还原点（或反向），即可演示跨云数据复制。

> 云下侧对象存储 = 内置 MinIO（`localStore`）；云上侧 = 真实 S3/Azure Blob。

## 排障

- 集群没起？看 `tail -f /tmp/supkube-colima-up.log`，或手动 `colima status` / `colima start --kubernetes`。
- 睡眠/断电相关：`pmset -g custom`、`tail /tmp/supkube-caffeinate.log`。
- SupKube 本身故障：见仓库根的 [RUNBOOK.md](../../RUNBOOK.md) §0 黄金 5 分钟。
- 依赖体检：`bash hack/verify-cluster.sh`。
