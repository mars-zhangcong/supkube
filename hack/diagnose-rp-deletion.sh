#!/usr/bin/env bash
# diagnose-rp-deletion.sh — 跨集群诊断 Velero 还原点删除卡住问题
#
# Why this exists
# ---------------
# 2026-05-31 Mars 现场反馈: 选多个 RP 删除, "后台一直在清, 前台慢慢消", 两个
# cluster 都有没删掉的 RP。Mars 让我先不操作, 先查清原因再说。
#
# 我们的删除路径是异步的:
#   DELETE /backups/<name> → 创建 DeleteBackupRequest (DBR) → 立刻返回 202
#   Velero backup-deletion-controller 异步处理 DBR → 清 BSL/VSC/PVB/CR
#
# 任一环节卡 (controller 死锁 / 网络挂 / cloud API throttle / finalizer wedged)
# 就会"半成品"——前端列表里能消失但 BSL 数据没清, 或反过来 BSL 清了但 CR 卡在
# Terminating。本脚本就是把这些"半成品"全找出来。
#
# 100% read-only
# --------------
# 这个脚本绝不 patch / delete / 任何写操作。所有 kubectl 调用都是 get / describe /
# logs。你看完结果, 自己决定怎么处理 (force-delete / kubectl patch 等), 我不替你做。
#
# Usage
# -----
#   ./hack/diagnose-rp-deletion.sh                              # 默认两个 dev cluster
#   ./hack/diagnose-rp-deletion.sh docker-desktop aks-jumborca-dev
#   ./hack/diagnose-rp-deletion.sh --output /tmp/rp-diagnose.md  # 写到文件 + 屏幕
#   ./hack/diagnose-rp-deletion.sh -h
#
# Output
# ------
# Markdown 格式, 每 cluster 一节, 含 8 类诊断:
#   ① Velero pod 健康度 (restart / OOMKilled / 最近事件)
#   ② Backup CR 删除中状态 (deletionTimestamp + finalizer 卡 = 真卡)
#   ③ DBR (DeleteBackupRequest) 总览 (InProgress 超 5min = 可疑)
#   ④ DBR 错误 / 警告
#   ⑤ 孤儿 VolumeSnapshotContent (DeletionPolicy=Retain = 账单陷阱)
#   ⑥ 长时间 InProgress 的 DataUpload / DataDownload
#   ⑦ BSL 状态
#   ⑧ Velero log 最近 30min ERROR
# 末尾汇总"嫌疑表"。

set -euo pipefail

# ─── 颜色 / 输出 ────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
DEFAULT_CONTEXTS=("docker-desktop" "aks-jumborca-dev")
CONTEXTS=()
OUTPUT_FILE=""

# ─── parse args ────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      sed -n '2,28p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    --output|-o)
      OUTPUT_FILE="$2"; shift 2 ;;
    *)
      CONTEXTS+=("$1"); shift ;;
  esac
done

# 没给 context 用默认
if [[ ${#CONTEXTS[@]} -eq 0 ]]; then
  CONTEXTS=("${DEFAULT_CONTEXTS[@]}")
fi

# 如果有 OUTPUT_FILE, 同时输出到屏幕 + 文件 (tee)
if [[ -n "$OUTPUT_FILE" ]]; then
  exec > >(tee "$OUTPUT_FILE")
fi

# ─── helpers ───────────────────────────────────────────────────────
banner() {
  echo
  echo -e "${BOLD}${BLUE}════════════════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}${BLUE} $1${NC}"
  echo -e "${BOLD}${BLUE}════════════════════════════════════════════════════════════════════════${NC}"
}
section() {
  echo
  echo -e "${BOLD}${1}${NC}"
  echo "$(printf '─%.0s' $(seq 1 ${#1}))"
}
warn()   { echo -e "${YELLOW}⚠ $*${NC}"; }
err()    { echo -e "${RED}❌ $*${NC}"; }
ok()     { echo -e "${GREEN}✓ $*${NC}"; }
info()   { echo -e "${BLUE}ℹ $*${NC}"; }

# 给一个 ISO timestamp 算从那时到现在多久 (秒). 用于判 "卡住超过 X min"
age_seconds() {
  local ts="$1"
  if [[ -z "$ts" || "$ts" == "null" ]]; then echo ""; return; fi
  local then_epoch now_epoch
  then_epoch=$(date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$ts" "+%s" 2>/dev/null \
            || date -u -d "$ts" "+%s" 2>/dev/null \
            || echo 0)
  now_epoch=$(date -u "+%s")
  if [[ "$then_epoch" -eq 0 ]]; then echo ""; return; fi
  echo $(( now_epoch - then_epoch ))
}
fmt_age() {
  local s="$1"
  if [[ -z "$s" ]]; then echo "?"; return; fi
  if [[ $s -lt 60 ]];      then echo "${s}s"
  elif [[ $s -lt 3600 ]];  then echo "$((s/60))m"
  elif [[ $s -lt 86400 ]]; then echo "$((s/3600))h"
  else echo "$((s/86400))d"; fi
}

# ─── 主诊断函数 (per cluster) ───────────────────────────────────────
diagnose_cluster() {
  local ctx="$1"
  local suspect_count=0
  declare -a suspects=()

  banner "Cluster: $ctx"

  # ── 0. Preflight: cluster 可达 + velero ns 存在 ──
  if ! kubectl --context="$ctx" get ns >/dev/null 2>&1; then
    err "cluster '$ctx' 不可达 — 跳过"
    return
  fi
  if ! kubectl --context="$ctx" get ns velero >/dev/null 2>&1; then
    warn "cluster '$ctx' 没装 Velero (没有 velero ns) — 跳过"
    return
  fi
  ok "cluster '$ctx' 可达, velero ns 存在"

  # ── ① Velero pod 健康度 ──
  section "① Velero pod 健康度"
  kubectl --context="$ctx" -n velero get pods -o custom-columns=\
NAME:.metadata.name,STATUS:.status.phase,READY:.status.containerStatuses[0].ready,RESTARTS:.status.containerStatuses[0].restartCount,IMAGE:.spec.containers[0].image \
    2>&1 | head -10

  # 看 backup-deletion-controller 跟主 velero 是不是 OK
  local velero_restarts
  velero_restarts=$(kubectl --context="$ctx" -n velero get pod -l name=velero \
                    -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "?")
  if [[ "$velero_restarts" != "?" && "$velero_restarts" != "0" ]]; then
    warn "Velero pod restart count = $velero_restarts (可疑, 可能 OOMKilled / panic)"
    suspect_count=$((suspect_count+1))
    suspects+=("Velero pod 重启了 $velero_restarts 次, 见 ${ctx}/① 节")
  fi

  # ── ② Backup CR 删除中状态 (deletionTimestamp 非空 = 标记删除, finalizer 没消 = 卡) ──
  section "② Backup CR 删除中状态 (DEL = deletionTimestamp 非空)"
  local stuck_backups
  stuck_backups=$(kubectl --context="$ctx" -n velero get backup -o json 2>/dev/null | jq -r '
    .items[] |
    select(.metadata.deletionTimestamp != null or ((.metadata.finalizers // []) | length > 0)) |
    [
      .metadata.name,
      (.status.phase // "Unknown"),
      (.metadata.deletionTimestamp // "-"),
      ((.metadata.finalizers // []) | join(",") | if . == "" then "-" else . end),
      (.status.startTimestamp // "-")
    ] | @tsv
  ' 2>/dev/null)

  if [[ -z "$stuck_backups" ]]; then
    ok "无 Backup CR 处于删除中状态"
  else
    {
      echo -e "NAME\tPHASE\tDELETION_TS\tFINALIZERS\tCREATED"
      echo "$stuck_backups"
    } | column -t -s$'\t' | head -30

    # 卡超过 10min 标 ⚠
    while IFS=$'\t' read -r name phase del_ts finalizers _; do
      [[ -z "$del_ts" || "$del_ts" == "-" ]] && continue
      local age
      age=$(age_seconds "$del_ts")
      if [[ -n "$age" && "$age" -gt 600 ]]; then
        warn "Backup '$name' 标删 $(fmt_age $age) 前但 finalizer=[$finalizers] 未清 — 真卡"
        suspect_count=$((suspect_count+1))
        suspects+=("Backup $name 标删 $(fmt_age $age) 卡 finalizer=$finalizers, 见 ${ctx}/② 节")
      fi
    done <<< "$stuck_backups"
  fi

  # ── ③ DBR (DeleteBackupRequest) 总览 ──
  section "③ DeleteBackupRequest 总览 (Phase != Processed 或 老于 5min)"
  local dbr_total
  dbr_total=$(kubectl --context="$ctx" -n velero get deletebackuprequest --no-headers 2>/dev/null | wc -l | tr -d ' ')
  info "DBR 总数: $dbr_total"

  if [[ "$dbr_total" != "0" ]]; then
    {
      echo -e "NAME\tBACKUP\tPHASE\tAGE\tERRORS#"
      kubectl --context="$ctx" -n velero get deletebackuprequest -o json | jq -r '
        .items[] | [
          .metadata.name,
          (.spec.backupName // "-"),
          (.status.phase // "New"),
          (.metadata.creationTimestamp // "-"),
          ((.status.errors // []) | length | tostring)
        ] | @tsv
      ' | while IFS=$'\t' read -r name backup phase ts errs; do
        local age age_str
        age=$(age_seconds "$ts")
        age_str=$(fmt_age "$age")
        echo -e "$name\t$backup\t$phase\t${age_str}\t$errs"
      done
    } | column -t -s$'\t' | head -30

    # 卡 DBR 检测
    kubectl --context="$ctx" -n velero get deletebackuprequest -o json | jq -r '
      .items[] |
      select(.status.phase != "Processed") |
      [.metadata.name, .spec.backupName, .status.phase, .metadata.creationTimestamp] | @tsv
    ' | while IFS=$'\t' read -r name backup phase ts; do
      local age
      age=$(age_seconds "$ts")
      if [[ -n "$age" && "$age" -gt 300 ]]; then
        warn "DBR '$name' (backup=$backup) phase=$phase $(fmt_age $age) 未完 — Velero controller 卡 / DBR worker 死锁?"
        suspect_count=$((suspect_count+1))
        suspects+=("DBR $name phase=$phase $(fmt_age $age) 卡, 见 ${ctx}/③ 节")
      fi
    done
  fi

  # ── ④ DBR 错误 / 警告 ──
  section "④ DBR 错误 / 警告 (cascade 中途失败的真凶)"
  local dbr_with_errs
  dbr_with_errs=$(kubectl --context="$ctx" -n velero get deletebackuprequest -o json | jq -r '
    .items[] |
    select((.status.errors // []) | length > 0) |
    "DBR: " + .metadata.name + " (backup=" + (.spec.backupName // "?") + ")\n  errors: " + ((.status.errors // []) | join(" | "))
  ' 2>/dev/null)
  if [[ -z "$dbr_with_errs" ]]; then
    ok "所有 DBR 无 errors"
  else
    echo "$dbr_with_errs"
    suspect_count=$((suspect_count+1))
    suspects+=("DBR 含 errors — cascade 失败, 见 ${ctx}/④ 节")
  fi

  # ── ⑤ 孤儿 VolumeSnapshotContent (DeletionPolicy=Retain = D-11 账单陷阱) ──
  section "⑤ 孤儿 VolumeSnapshotContent (DeletionPolicy=Retain = 账单陷阱)"
  if ! kubectl --context="$ctx" get crd volumesnapshotcontents.snapshot.storage.k8s.io >/dev/null 2>&1; then
    info "VolumeSnapshotContent CRD 未装 (CSI snapshotter 没用)"
  else
    local orphan_vsc
    orphan_vsc=$(kubectl --context="$ctx" get volumesnapshotcontent -o json 2>/dev/null | jq -r '
      .items[] |
      [.metadata.name, .spec.deletionPolicy, (.spec.volumeSnapshotRef.namespace // "-"), (.spec.volumeSnapshotRef.name // "-"), (.status.snapshotHandle // "-")] | @tsv
    ')
    local retain_count
    retain_count=$(echo "$orphan_vsc" | awk -F'\t' '$2=="Retain"' | wc -l | tr -d ' ')
    if [[ "$retain_count" == "0" ]]; then
      ok "无 Retain 策略 VSC (或全部 Delete 策略, 安全)"
    else
      warn "$retain_count 个 VSC 带 DeletionPolicy=Retain — 删 ns 不会连带删云快照 → 账单陷阱"
      {
        echo -e "NAME\tPOLICY\tSRC_NS\tSRC_VS\tSNAPSHOT_HANDLE"
        echo "$orphan_vsc" | awk -F'\t' '$2=="Retain"'
      } | column -t -s$'\t' | head -15
      suspect_count=$((suspect_count+1))
      suspects+=("$retain_count 个 Retain 策略 VSC 是账单陷阱, 见 ${ctx}/⑤ 节")
    fi
  fi

  # ── ⑥ 长时间 InProgress 的 DataUpload / DataDownload ──
  section "⑥ DataUpload / DataDownload InProgress 长时间未完 (data-mover 卡的真凶)"
  local stuck_du
  stuck_du=$(kubectl --context="$ctx" -n velero get dataupload,datadownload -o json 2>/dev/null | jq -r '
    .items[] |
    select(.status.phase == "InProgress") |
    [.kind, .metadata.name, .status.phase, .metadata.creationTimestamp] | @tsv
  ')
  if [[ -z "$stuck_du" ]]; then
    ok "无长时间 InProgress 的 DataUpload/DataDownload"
  else
    echo "$stuck_du" | while IFS=$'\t' read -r kind name phase ts; do
      local age
      age=$(age_seconds "$ts")
      if [[ -n "$age" && "$age" -gt 1800 ]]; then  # 30 min
        warn "$kind '$name' InProgress $(fmt_age $age) — node-agent hang? (跟 ADR-026 同根因)"
        suspect_count=$((suspect_count+1))
        suspects+=("$kind $name InProgress $(fmt_age $age) 卡, 见 ${ctx}/⑥ 节")
      else
        info "$kind '$name' InProgress $(fmt_age $age) — 正常进行中"
      fi
    done
  fi

  # ── ⑦ BSL 状态 ──
  section "⑦ BSL 状态 (Available / Unavailable)"
  kubectl --context="$ctx" -n velero get bsl -o custom-columns=\
NAME:.metadata.name,PROVIDER:.spec.provider,BUCKET:.spec.objectStorage.bucket,STATUS:.status.phase,LAST_VALIDATED:.status.lastValidationTime \
    2>&1 | head -10
  local unavail
  unavail=$(kubectl --context="$ctx" -n velero get bsl -o json | jq -r '.items[] | select(.status.phase != "Available") | .metadata.name')
  if [[ -n "$unavail" ]]; then
    while IFS= read -r bsl; do
      warn "BSL '$bsl' 不可用 — DBR cascade 删 BSL tarball 会失败"
      suspect_count=$((suspect_count+1))
      suspects+=("BSL $bsl Unavailable, 见 ${ctx}/⑦ 节")
    done <<< "$unavail"
  fi

  # ── ⑧ Velero log 最近 30min ERROR ──
  section "⑧ Velero log 最近 30min ERROR / panic (前 20 条)"
  kubectl --context="$ctx" -n velero logs deploy/velero --since=30m 2>/dev/null \
    | grep -iE "level=error|panic|deletebackuprequest.*error" \
    | head -20 \
    | sed 's/^/  /' || echo "  (no ERROR in 30m)"

  # ── ⑨ 汇总 ──
  section "⑨ Cluster '$ctx' 嫌疑汇总"
  if [[ "$suspect_count" == "0" ]]; then
    ok "本 cluster 无可疑项, 删除卡顿可能纯属异步 (等 Velero 慢慢清)"
  else
    err "$suspect_count 个嫌疑项 (按出现顺序):"
    for s in "${suspects[@]}"; do
      echo "  • $s"
    done
  fi
}

# ─── main ─────────────────────────────────────────────────────────
banner "Velero 还原点删除诊断 — $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
echo "Contexts to check: ${CONTEXTS[*]}"
echo "Mode: 100% read-only (绝不 patch/delete)"
[[ -n "$OUTPUT_FILE" ]] && echo "Output also tee'd to: $OUTPUT_FILE"

for ctx in "${CONTEXTS[@]}"; do
  diagnose_cluster "$ctx"
done

banner "诊断完成 — 把上面输出整段返回给我, 我帮你看真因"
echo "如果太长, 仅返回 ⚠ / ❌ 行 + 每 cluster 第 ⑨ 节 (嫌疑汇总) 即可"
