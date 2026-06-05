#!/usr/bin/env bash
# session-isolation-guard.sh — 会话启动时的"撞车墙"(SessionStart hook 用)。
#
# 座右铭(ENGINEERING.md Rule I):治理办法没用,有用的是永远不给错误发生的条件。
#
# 背景:Rule I 的 worktree 隔离原本只覆盖"协调者 spawn 的并行子 agent"。但
# 多个**独立启动的顶层交互会话**都默认 cwd=项目根,没有任何触发点让它们自我
# 隔离 —— 于是 N 个会话挤在同一 checkout 上写,共享 index/工作树,谁 commit
# 落谁 HEAD 不可控(2026-06-04 实测 census §2 逮到 6 会话挤主目录)。
#
# 这道守卫在**每次会话启动**跑:若本会话的 cwd = 项目根、且主目录里已有别的
# 活 claude 会话 → 大声告警 + 给一键隔离命令。它**只读 + 只告警**,绝不杀会话、
# 不动工作树 —— 零风险。把"记得隔离"变成"启动就撞墙"。
#
# 退出码恒为 0(SessionStart hook 不应阻断启动;它只提示)。
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# 找出 cwd 落在项目根的 claude 进程数(macOS: lsof 看 cwd)。
# 用 lsof -Fn 字段输出做精确整串比较 —— 项目路径含空格("Claude Code"),
# 绝不能用 awk $NF / cut -d' '(会被空格切断,漏报)。每个 claude 会话一个
# claude 进程、一条 cwd;数 n<ROOT> 行即会话数(disclaimer 启动器命令名不同,
# -c claude 不匹配它)。
count=$(lsof -a -d cwd -c claude -Fn 2>/dev/null | grep -cxF "n$ROOT" || true)
count=${count:-0}

if [ "${count:-0}" -gt 1 ]; then
  cat >&2 <<EOF

  ⚠️  会话隔离告警(ENGINEERING.md Rule I)
  ────────────────────────────────────────────────────────────
  检测到 ${count} 个 claude 会话同时把 cwd 落在主仓库:
    $ROOT
  多个会话共享同一 checkout(同 index / 同工作树)= 撞车风险:
  并发 git checkout/commit/stage 会互踩,谁 commit 落谁 HEAD 不可控。

  ✅ 处理(把本会话搬进独立 worktree,物理隔离):
     bash hack/session-claim-worktree.sh            # 看方案(dry-run)
     bash hack/session-claim-worktree.sh --run      # 真建并 cd 过去

  或临时只读/只看时:不要在主目录做任何 git 写操作(checkout/commit/stage)。
  全景排查: bash hack/agent-census.sh
  ────────────────────────────────────────────────────────────
EOF
else
  echo "  ✓ 会话隔离 OK:主仓库当前仅本会话占用(无撞车)。" >&2
fi
exit 0
