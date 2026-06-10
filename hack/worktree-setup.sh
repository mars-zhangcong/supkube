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

# 已合并 PR 的分支(含 squash 合并):内容已在 main,属"残枝",不再建 worktree。
# 修"领先 main"假阳性 —— squash 合并的分支 rev-list 仍领先,但活早落地了。
MERGED_HEADS=""
if command -v gh >/dev/null 2>&1; then
  MERGED_HEADS=$(gh pr list --state merged --limit 300 --json headRefName -q '.[].headRefName' 2>/dev/null || true)
fi

echo "# ── worktree 预分配方案（一分支一棵树，base=$BASE）──"
n_plan=0
for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin \
            | grep -vE 'origin/(main|HEAD|v[0-9])'); do
  sb=${b#origin/}
  ahead=$(git rev-list --count "origin/main..$b" 2>/dev/null || echo 0)
  [ "$ahead" -gt 0 ] 2>/dev/null || continue          # 只给领先 main 的分支建
  # 残枝过滤:已有 merged PR(含 squash)→ 内容在 main,跳过(别被假"领先"骗)。
  if [ -n "$MERGED_HEADS" ] && printf '%s\n' "$MERGED_HEADS" | grep -qxF "$sb"; then
    echo "# 跳过 $sb —— 已有 merged PR(squash 残枝,内容已在 main,建议删分支)"; continue
  fi
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
