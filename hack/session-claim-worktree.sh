#!/usr/bin/env bash
# session-claim-worktree.sh — 把当前会话搬进一棵独立 worktree(物理隔离)。
#
# 配套 session-isolation-guard.sh:守卫告警后,跑这个一键认领一棵属于本会话的
# worktree(基于最新 main),之后在那棵树里作业,与其他会话零撞车(独立 index/
# HEAD,仅共享 .git 对象库,ENGINEERING.md Rule I)。
#
# ★ 默认 DRY-RUN,只打印将执行的命令;加 --run 才真建。
#   脚本无法改父 claude 进程的 cwd,所以建好后会打印 `cd` 命令,你照着切过去
#   (或在该目录开新会话)。
#
# 用法:
#   bash hack/session-claim-worktree.sh         # dry-run
#   bash hack/session-claim-worktree.sh --run
set -uo pipefail

RUN=0; [ "${1:-}" = "--run" ] && RUN=1
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASE=/private/tmp
# 用本 shell 的 PPID(≈ 会话)做唯一后缀,避免多会话撞同一 worktree 名。
SID="s$(echo "${PPID:-$$}" | tail -c 5)"
WT="$BASE/sk-session-$SID"
BR="session/$SID"

cd "$ROOT"
git fetch origin --quiet 2>/dev/null || true

if [ -e "$WT" ]; then
  echo "  worktree 已存在: $WT"
  echo "  → cd \"$WT\""
  exit 0
fi

CMD="git worktree add -b \"$BR\" \"$WT\" origin/main"
echo "# ── 本会话独立 worktree 方案(Rule I 隔离)──"
echo "#   分支: $BR(基于 origin/main)"
echo "#   路径: $WT"
echo "$CMD"

if [ "$RUN" = 1 ]; then
  eval "$CMD"
  echo
  echo "  ✅ 已建。现在切过去作业(本脚本改不了父会话 cwd,请手动):"
  echo "     cd \"$WT\""
  echo "  完工合并后回收: git worktree remove \"$WT\" && git branch -D \"$BR\""
else
  echo
  echo "  (dry-run。确认后加 --run 真建。)"
fi
exit 0
