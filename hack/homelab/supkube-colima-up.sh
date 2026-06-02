#!/usr/bin/env bash
# ============================================================================
# supkube-colima-up.sh — Colima/k3s 看门狗
# ----------------------------------------------------------------------------
# 由 LaunchAgent (com.supkube.colima.plist) 每 60s 调一次：
#   - Colima 在跑           → 直接退出
#   - Colima 没跑 / 挂了     → 重新拉起（开机首次、或异常退出后自愈）
#
# 装到 /usr/local/bin/supkube-colima-up.sh（install-supkube.sh 会拷过去）。
# 资源按 Intel 16GB 设定：VM 拿 4 vCPU / 10GB / 80GB，给 macOS 留足余量。
# ============================================================================
set -u

# LaunchAgent 环境 PATH 极简，必须手动补齐 brew 路径
export PATH="/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"
LOG="/tmp/supkube-colima-up.log"

if colima status >/dev/null 2>&1; then
  exit 0
fi

echo "$(date '+%F %T') Colima 未运行，正在拉起…" >>"$LOG"
colima start \
  --kubernetes \
  --cpu 4 \
  --memory 10 \
  --disk 80 \
  >>"$LOG" 2>&1
echo "$(date '+%F %T') colima start 退出码=$?" >>"$LOG"
