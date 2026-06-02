#!/usr/bin/env bash
# ============================================================================
# setup-macos-always-on.sh — 把一台 Intel Mac 调成 24/7 永不掉线的 Homelab 主机
# ----------------------------------------------------------------------------
# 在【那台 Homelab Mac】上以普通用户运行（脚本内部会按需 sudo）：
#   bash setup-macos-always-on.sh
#
# 做四件事：
#   1. pmset 电源策略 —— 永不睡眠 / 断电后自动开机 / 网络唤醒
#   2. caffeinate LaunchDaemon —— 兜底防止系统空闲睡眠（即使没人登录）
#   3. 检查 FileVault 与自动登录（断电自愈的前提，脚本只检测+提示，不强改）
#   4. 检查 Tailscale 以系统守护进程方式常驻
#
# 幂等：可重复运行。
# ============================================================================
set -euo pipefail

echo "==> 1/4  pmset 电源策略（永不睡眠 + 断电自动开机）"
# -c = 接电源时的策略（Homelab 始终接电）
sudo pmset -c sleep 0          # 系统永不睡眠
sudo pmset -c disksleep 0      # 硬盘永不休眠
sudo pmset -c displaysleep 10  # 屏幕可以睡（省电，不影响服务）
sudo pmset -a womp 1           # Wake-on-Network：网络访问可唤醒
sudo pmset -a autorestart 1    # 断电恢复后自动开机（最关键的一条）
sudo pmset -a powernap 0       # 关掉 PowerNap，避免周期性半睡眠
# disablesleep：合盖/空闲都不睡（笔记本类机型必须；台式机也无害）
sudo pmset -c disablesleep 1 || echo "   (disablesleep 在此机型不支持，忽略)"
echo "   当前策略："
pmset -g custom | sed 's/^/     /'

echo "==> 2/4  安装 caffeinate 兜底守护（防止任何空闲睡眠）"
PLIST_SRC="$(cd "$(dirname "$0")" && pwd)/com.supkube.caffeinate.plist"
sudo cp "$PLIST_SRC" /Library/LaunchDaemons/com.supkube.caffeinate.plist
sudo chown root:wheel /Library/LaunchDaemons/com.supkube.caffeinate.plist
sudo chmod 644 /Library/LaunchDaemons/com.supkube.caffeinate.plist
sudo launchctl bootout system/com.supkube.caffeinate 2>/dev/null || true
sudo launchctl bootstrap system /Library/LaunchDaemons/com.supkube.caffeinate.plist
echo "   caffeinate 守护已加载（KeepAlive=true，崩溃自动重启）"

echo "==> 3/4  检查 FileVault 与自动登录（断电自愈前提）"
if fdesetup status | grep -q "On"; then
  cat <<'EOF'
   ⚠️  FileVault 已开启。断电重启后磁盘需要有人输密码才能解锁，
       Homelab 无法无人值守自愈。建议二选一：
         a) 关闭 FileVault（Homelab 在可信内网，可接受）：
            系统设置 → 隐私与安全性 → FileVault → 关闭
         b) 保留 FileVault，但接受"断电后需手动开机解锁一次"。
EOF
else
  echo "   FileVault 已关闭 ✓（断电后可无人值守开机）"
fi
cat <<'EOF'
   👉 还需手动开启【自动登录】（脚本无法代设）：
      系统设置 → 用户与群组 → （右上）自动以…身份登录 → 选你的账户
      原因：Colima/k3s 跑在你的用户会话里，自动登录才能开机自启。
EOF

echo "==> 4/4  检查 Tailscale 常驻"
if command -v tailscale >/dev/null 2>&1; then
  tailscale status >/dev/null 2>&1 && echo "   Tailscale 在线 ✓" || echo "   Tailscale 已装但未连接，运行：sudo tailscale up --ssh"
  echo "   建议改用系统守护进程模式（脱离登录会话也常驻）："
  echo "     sudo tailscaled install-system-daemon"
else
  echo "   未检测到 tailscale CLI。若用的是 App Store 版，建议改装独立版："
  echo "     brew install tailscale && sudo tailscaled install-system-daemon && sudo tailscale up --ssh"
fi

echo
echo "✅ 永不掉线基础层就绪。下一步：bash install-supkube.sh"
