#!/usr/bin/env bash
# agent-census.sh — 物理清点所有活跃 claude-code 会话 + worktree 占用。
#
# 只读：不改任何 git 状态，随时可跑。判据靠物理证据（进程 cwd /
# index.lock / 工作区脏度），不靠口头"清场了吗"。
#
# 用法： hack/agent-census.sh
set -uo pipefail

COMMON_GIT=$(git rev-parse --git-common-dir 2>/dev/null)
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)

echo "════════════════════════════════════════════════════════════"
echo " AGENT CENSUS @ $(date '+%F %T')"
echo " repo: $REPO_ROOT"
echo "════════════════════════════════════════════════════════════"

echo
echo "── 1) 活跃 claude-code 会话 + 各自 cwd ──"
found=0
for pid in $(pgrep -f 'claude-code/' 2>/dev/null); do
  cwd=$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p')
  [ -z "$cwd" ] && continue
  printf "  PID %-7s cwd=%s\n" "$pid" "$cwd"
  found=$((found + 1))
done
[ "$found" = 0 ] && echo "  (没探到 claude-code cli 进程；改 pgrep 模式或确认进程名)"

echo
echo "── 2) 同一目录挤了几个会话？(>1 = 撞车风险) ──"
for pid in $(pgrep -f 'claude-code/' 2>/dev/null); do
  lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p'
done | sort | uniq -c | sort -rn | while read -r n dir; do
  flag=""; [ "${n:-0}" -gt 1 ] 2>/dev/null && flag="   ⚠ $n 个会话共享 → 撞车风险"
  printf "  %2s × %s%s\n" "$n" "$dir" "$flag"
done

echo
echo "── 3) 谁正在写 git？(index.lock = 有人 commit/checkout 中，别碰) ──"
locks=$(find "$COMMON_GIT" "$COMMON_GIT/worktrees" -name index.lock 2>/dev/null)
if [ -n "$locks" ]; then echo "$locks" | sed 's/^/  ⚠ LOCK  /'; else echo "  ✓ 无锁，当前没人在写"; fi

echo
echo "── 4) 所有 worktree 当前分支 ──"
git worktree list

echo
echo "  各 worktree 未提交改动："
# NUL-safe 枚举：--porcelain -z 用 NUL 作行/记录终止符，read -r -d '' 逐条读，
# 路径含空格（本仓库 "Claude Code"）甚至换行都不会断。变量一律加引号。
# ⚠ 绝不用 `for wt in $(… awk '{print $2}')`／`cut -d' '` —— 遇空格即断、漏条目。
while IFS= read -r -d '' field; do
  case "$field" in
    "worktree "*)
      wt="${field#worktree }"
      n=$(git -C "$wt" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
      br=$(git -C "$wt" rev-parse --abbrev-ref HEAD 2>/dev/null)
      mark=""; [ "$n" != 0 ] && mark="   ← 有活没 commit"
      printf "    [%-38s] %s 处%s\n" "$br" "$n" "$mark"
      [ "$n" != 0 ] && git -C "$wt" status --short 2>/dev/null | sed 's/^/        /'
      ;;
  esac
done < <(git worktree list --porcelain -z)

echo
echo "════════════════════════════════════════════════════════════"
echo " 判读： 第 2 段任何一行 >1 → 该目录多会话挤着，必须隔离"
echo "        第 3 段有 LOCK    → 等锁消失再做迁移"
echo "        第 4 段有'有活没 commit' → 先让对应会话自己 commit"
echo "════════════════════════════════════════════════════════════"
