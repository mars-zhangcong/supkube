#!/usr/bin/env bash
# worktree-setup.sh — 给每个"领先 main 且需继续作业"的分支预分配独立 worktree。
#
# 一分支一 worktree，物理隔离（ENGINEERING Rule I）。
#
# ★ 默认 DRY-RUN：只打印将执行的命令供你 review，不动任何东西。
#   确认无误后加 --run 才真正执行。
#
# 用法：
#   hack/worktree-setup.sh          # 看会建哪些（dry-run）
#   hack/worktree-setup.sh --run    # 真建
set -euo pipefail

RUN=0; [ "${1:-}" = "--run" ] && RUN=1
BASE=/private/tmp
git fetch origin --quiet 2>/dev/null || true

echo "# ── worktree 预分配方案（一分支一棵树，base=$BASE）──"
n_plan=0
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin \
            | grep -vE 'origin/(main|HEAD|v[0-9])'); do
  sb=${b#origin/}
  ahead=$(git rev-list --count "origin/main..$b" 2>/dev/null || echo 0)
  [ "$ahead" -gt 0 ] 2>/dev/null || continue          # 只给领先 main 的分支建
  slug=$(printf '%s' "$sb" | tr '/' '-')
  wt="$BASE/sk-$slug"

  if git worktree list --porcelain | grep -q "branch refs/heads/$sb\$"; then
    echo "# 跳过 $sb —— 已在某 worktree 签出"; continue
  fi
  if [ -e "$wt" ]; then echo "# 跳过 $sb —— $wt 已存在"; continue; fi

  if git show-ref --verify --quiet "refs/heads/$sb"; then
    cmd="git worktree add \"$wt\" \"$sb\""                       # 本地分支已存在
  else
    cmd="git worktree add --track -b \"$sb\" \"$wt\" \"origin/$sb\""  # 仅远端 → 建跟踪分支
  fi

  n_plan=$((n_plan + 1))
  if [ "$RUN" = 1 ]; then
    echo "+ $cmd"; eval "$cmd"
  else
    echo "$cmd"
  fi
done

echo
[ "$n_plan" = 0 ] && echo "# 没有需要新建 worktree 的分支（都已隔离或都已并入 main）。"
echo "# 之后：让对应 agent 'cd' 进自己的 worktree，绝不回主树作业。"
[ "$RUN" = 0 ] && echo "# (当前为 DRY-RUN；确认无误后加 --run 执行)"
