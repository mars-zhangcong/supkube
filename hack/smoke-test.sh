#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  smoke-test.sh — SupKube CD 后置「持续冒烟测试」                            ║
# ║                                                                          ║
# ║  在 CD 把新版本部署到测试集群 (aks-jumborca-test) 之后立即运行：对实时集群  ║
# ║  跑 测试用例.md §1 P0 冒烟套件里「机器可验证」的子集，产出结构化结果，      ║
# ║  供 Dashboard 渲染。每条用例映射到真实 TC 编号。                          ║
# ║                                                                          ║
# ║  覆盖 (v1)：                                                             ║
# ║    SMK-VERIFY  部署验证 (复用 ci-verify.sh: rollout+buildStamp+RBAC) TC-ENV-001 ║
# ║    SMK-API     Backend /api/v1/status 健康                         TC-ENV-001 ║
# ║    SMK-TS      内置 Transform Sets 已种入 (>=4)                     TC-TS-001  ║
# ║    SMK-BACKUP  创建备份 → Velero Backup CR Completed                TC-RP-001  ║
# ║    SMK-RESTORE 恢复到新 ns → Completed + 标记数据一致               TC-RST-001 ║
# ║    SMK-CLEANUP 删除 RP → Backup CR 级联清除                         TC-REG-010 ║
# ║    SMK-CSI     CSI 卷快照严格模式 → skipped (需集群 VolumeSnapshotClass)     ║
# ║                                                                          ║
# ║  ⚠ 防云账单 (测试用例.md §0.4)：无论成败，EXIT trap 都会清掉测试           ║
# ║    namespace / Velero CR / 带本次前缀的 VolumeSnapshotContent。           ║
# ║                                                                          ║
# ║  用法：  ./hack/smoke-test.sh <version>                                   ║
# ║  前置：  kubectl 已 set-context 到测试集群；curl、jq 可用。               ║
# ║  环境变量：                                                              ║
# ║    SUPKUBE_NS   (default supkube)   SupKube 部署所在 ns                   ║
# ║    VELERO_NS    (default velero)    Velero ns                            ║
# ║    BACKEND_SVC  (default supkube-backend)  后端 Service 名               ║
# ║    BACKEND_PORT (default 8080)                                          ║
# ║    SUPKUBE_TOKEN (可选)  Bearer token；不设则假定测试集群 AUTH_ENABLED=false ║
# ║    RESULTS_DIR  (default ./smoke-results)                               ║
# ║    BACKUP_TIMEOUT / RESTORE_TIMEOUT (秒, default 300)                    ║
# ║  退出码： 0 = 无 failed；1 = 有 failed 用例（skipped 不算失败）          ║
# ╚══════════════════════════════════════════════════════════════════════════╝
set -uo pipefail   # 故意不加 -e：要捕获每条 check 的失败并继续跑完

VERSION="${1:-${SUPKUBE_VERSION:-unknown}}"
SUPKUBE_NS="${SUPKUBE_NS:-supkube}"
VELERO_NS="${VELERO_NS:-velero}"
BACKEND_SVC="${BACKEND_SVC:-supkube-backend}"
BACKEND_PORT="${BACKEND_PORT:-8080}"
RESULTS_DIR="${RESULTS_DIR:-./smoke-results}"
BACKUP_TIMEOUT="${BACKUP_TIMEOUT:-300}"
RESTORE_TIMEOUT="${RESTORE_TIMEOUT:-300}"
LOCAL_PORT=18080
CLUSTER="${CLUSTER:-$(kubectl config current-context 2>/dev/null || echo unknown)}"
RUN_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
RUN_ID="$(date -u +%Y%m%d-%H%M%S)"

# 测试资源统一前缀，便于清理时精确匹配（绝不误删非本次资源）
PREFIX="smoke-${RUN_ID}"
TEST_NS="${PREFIX}"
RESTORE_NS="${PREFIX}-restore"
BACKUP_NAME="${PREFIX}-bk"
RESTORE_NAME="${PREFIX}-rst"

mkdir -p "$RESULTS_DIR"
RESULTS_TMP="$(mktemp)"
PF_PID=""
RESULT_FAILED=0

log() { echo -e "  $*"; }

# record <id> <name> <passed|failed|skipped> <seconds> <tc> <notes>
record() {
  jq -nc \
    --arg id "$1" --arg name "$2" --arg status "$3" \
    --arg dur "$4" --arg tc "$5" --arg notes "$6" \
    '{id:$id,name:$name,status:$status,durationSec:(($dur|tonumber?)//0),tc:$tc,notes:$notes}' \
    >> "$RESULTS_TMP"
  log "[$3] $1 $2"
}

# ── 防云账单清理（测试用例.md §0.4）── EXIT trap：成败都跑
cleanup() {
  log "── 清理 (防云账单, §0.4) ──"
  [[ -n "$PF_PID" ]] && kill "$PF_PID" 2>/dev/null || true
  kubectl delete ns "$TEST_NS" "$RESTORE_NS" --ignore-not-found --wait=false 2>/dev/null || true
  kubectl -n "$VELERO_NS" delete backup  "$BACKUP_NAME"  --ignore-not-found 2>/dev/null || true
  kubectl -n "$VELERO_NS" delete restore "$RESTORE_NAME" --ignore-not-found 2>/dev/null || true
  # 只删带本次前缀的 VSC，避免误伤
  for vsc in $(kubectl get volumesnapshotcontent -o name 2>/dev/null | grep "$PREFIX" || true); do
    kubectl delete "$vsc" --ignore-not-found 2>/dev/null || true
  done
  log "清理完成。若用过 CSI 真快照，请额外用 az snapshot 复核云磁盘快照 (见 RUNBOOK 防账单 checklist)。"
}
trap cleanup EXIT

# ── 经 port-forward 调 SupKube API ──
api() {  # api <method> <path> [body]
  local method="$1" path="$2" body="${3:-}"
  local auth=()
  [[ -n "${SUPKUBE_TOKEN:-}" ]] && auth=(-H "Authorization: Bearer ${SUPKUBE_TOKEN}")
  if [[ -n "$body" ]]; then
    curl -sS -m 30 -X "$method" "${auth[@]}" -H "Content-Type: application/json" -d "$body" \
      "http://localhost:${LOCAL_PORT}${path}"
  else
    curl -sS -m 30 -X "$method" "${auth[@]}" "http://localhost:${LOCAL_PORT}${path}"
  fi
}

# poll_phase <backup|restore> <name> <timeout-sec> → 打印最终 phase
poll_phase() {
  local kind="$1" name="$2" timeout="$3" waited=0 phase=""
  while (( waited < timeout )); do
    phase=$(kubectl -n "$VELERO_NS" get "$kind" "$name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    case "$phase" in
      Completed|PartiallyFailed|Failed|FailedValidation) echo "$phase"; return 0 ;;
    esac
    sleep 10; waited=$((waited+10))
  done
  echo "${phase:-Timeout}"
}

start_pf() {
  kubectl -n "$SUPKUBE_NS" port-forward "svc/${BACKEND_SVC}" "${LOCAL_PORT}:${BACKEND_PORT}" >/dev/null 2>&1 &
  PF_PID=$!
  for _ in $(seq 1 15); do
    sleep 1
    curl -sf -m 3 "http://localhost:${LOCAL_PORT}/api/v1/status" >/dev/null 2>&1 && return 0
  done
  return 1
}

finalize() {
  local results total passed failed skipped
  results=$(jq -s '.' "$RESULTS_TMP" 2>/dev/null || echo '[]')
  total=$(echo "$results"   | jq 'length')
  passed=$(echo "$results"  | jq '[.[]|select(.status=="passed")]|length')
  failed=$(echo "$results"  | jq '[.[]|select(.status=="failed")]|length')
  skipped=$(echo "$results" | jq '[.[]|select(.status=="skipped")]|length')
  RESULT_FAILED=$failed

  jq -n \
    --arg ts "$RUN_TS" --arg runId "$RUN_ID" --arg version "$VERSION" --arg cluster "$CLUSTER" \
    --argjson results "$results" \
    --argjson total "$total" --argjson passed "$passed" --argjson failed "$failed" --argjson skipped "$skipped" \
    '{timestamp:$ts, runId:$runId, version:$version, cluster:$cluster, suite:"P0-smoke",
      summary:{total:$total,passed:$passed,failed:$failed,skipped:$skipped},
      testRuns:$results}' \
    > "$RESULTS_DIR/test-results.json"

  # 人类可读 Markdown 报告（对齐 engineer-testing/demo-result.md 习惯）
  {
    echo "# SupKube 冒烟测试结果"
    echo ""
    echo "- 版本：\`$VERSION\`  · 集群：\`$CLUSTER\`  · 时间：$RUN_TS  · run：\`$RUN_ID\`"
    echo "- 结果：**$passed passed / $failed failed / $skipped skipped**（共 $total）"
    echo ""
    echo "| 用例 | TC | 结果 | 耗时 | 备注 |"
    echo "|---|---|---|---|---|"
    echo "$results" | jq -r '.[] |
      "| \(.id) \(.name) | \(.tc) | \(if .status=="passed" then "✅ passed" elif .status=="failed" then "❌ failed" else "⏭ skipped" end) | \(.durationSec)s | \(.notes) |"'
  } > "$RESULTS_DIR/test-results.md"

  echo ""
  echo "════════ 结果: passed=$passed failed=$failed skipped=$skipped (total=$total) ════════"
  echo "  → $RESULTS_DIR/test-results.json"
  echo "  → $RESULTS_DIR/test-results.md"
}

echo "════════ SupKube 冒烟测试 ════════"
echo "  集群=$CLUSTER  版本=$VERSION  run=$RUN_ID"

# ===== SMK-VERIFY：部署验证（复用 ci-verify.sh）=====
t0=$SECONDS
if [[ -f hack/ci-verify.sh ]]; then chmod +x hack/ci-verify.sh; fi
if [[ -x hack/ci-verify.sh ]] && ./hack/ci-verify.sh "$SUPKUBE_NS" "$VERSION" >/tmp/verify.log 2>&1; then
  record "SMK-VERIFY" "部署验证(rollout+buildStamp+RBAC)" "passed" "$((SECONDS-t0))" "TC-ENV-001" "ci-verify 通过"
else
  record "SMK-VERIFY" "部署验证(rollout+buildStamp+RBAC)" "failed" "$((SECONDS-t0))" "TC-ENV-001" "$(tail -1 /tmp/verify.log 2>/dev/null | tr -d '|')"
fi

# ===== 启动 port-forward =====
if ! start_pf; then
  record "SMK-API" "Backend API 可达(/api/v1/status)" "failed" "0" "TC-ENV-001" "port-forward/status 不可达，跳过后续 API 用例"
  finalize
  exit 1
fi

# ===== SMK-API：status 健康 =====
t0=$SECONDS
status_json=$(api GET /api/v1/status || echo "")
if echo "$status_json" | jq -e '.status=="ok"' >/dev/null 2>&1; then
  record "SMK-API" "Backend API 健康(/api/v1/status)" "passed" "$((SECONDS-t0))" "TC-ENV-001" "buildStamp=$(echo "$status_json" | jq -r '.buildStamp // ""')"
else
  record "SMK-API" "Backend API 健康(/api/v1/status)" "failed" "$((SECONDS-t0))" "TC-ENV-001" "status!=ok"
fi

# ===== SMK-TS：内置 Transform Sets >=4 (TC-TS-001) =====
t0=$SECONDS
ts_json=$(api GET /api/v1/transform-sets || echo "")
builtin_count=$(echo "$ts_json" | jq '[.items[]? | select(.builtIn==true)] | length' 2>/dev/null || echo 0)
if (( builtin_count >= 4 )); then
  record "SMK-TS" "内置 Transform Sets 已种入(>=4)" "passed" "$((SECONDS-t0))" "TC-TS-001" "builtIn=$builtin_count"
else
  record "SMK-TS" "内置 Transform Sets 已种入(>=4)" "failed" "$((SECONDS-t0))" "TC-TS-001" "仅 $builtin_count 个 builtIn"
fi

# ===== SMK-BACKUP：备份生命周期 (TC-RP-001 ns 级变体) =====
# 部署一个简单可备份的工作负载 + 标记 ConfigMap（marker=run id，用于恢复后校验数据一致）
t0=$SECONDS
kubectl create ns "$TEST_NS" 2>/dev/null || true
kubectl -n "$TEST_NS" create configmap smoke-marker --from-literal=marker="$RUN_ID" 2>/dev/null || true
kubectl -n "$TEST_NS" create deployment smoke-nginx --image=nginx:alpine 2>/dev/null || true
kubectl -n "$TEST_NS" rollout status deploy/smoke-nginx --timeout=120s >/dev/null 2>&1 || true
backup_body=$(jq -nc --arg n "$BACKUP_NAME" --arg ns "$TEST_NS" \
  '{name:$n, includedNamespaces:[$ns], ttl:"24h"}')
api POST /api/v1/backups "$backup_body" >/tmp/bk.json 2>&1
bphase=$(poll_phase backup "$BACKUP_NAME" "$BACKUP_TIMEOUT")
if [[ "$bphase" == "Completed" ]]; then
  record "SMK-BACKUP" "创建备份 → Completed" "passed" "$((SECONDS-t0))" "TC-RP-001" "backup=$BACKUP_NAME"
else
  record "SMK-BACKUP" "创建备份 → Completed" "failed" "$((SECONDS-t0))" "TC-RP-001" "phase=$bphase"
fi

# ===== SMK-RESTORE：恢复到新 ns + 数据一致 (TC-RST-001 变体) =====
t0=$SECONDS
if [[ "$bphase" == "Completed" ]]; then
  restore_body=$(jq -nc --arg n "$RESTORE_NAME" --arg bk "$BACKUP_NAME" --arg src "$TEST_NS" --arg dst "$RESTORE_NS" \
    '{name:$n, backupName:$bk, includedNamespaces:[$src], namespaceMapping:{($src):$dst}}')
  api POST /api/v1/restores "$restore_body" >/tmp/rst.json 2>&1
  rphase=$(poll_phase restore "$RESTORE_NAME" "$RESTORE_TIMEOUT")
  marker=$(kubectl -n "$RESTORE_NS" get configmap smoke-marker -o jsonpath='{.data.marker}' 2>/dev/null || echo "")
  if [[ "$rphase" == "Completed" && "$marker" == "$RUN_ID" ]]; then
    record "SMK-RESTORE" "恢复到新 ns → Completed+数据一致" "passed" "$((SECONDS-t0))" "TC-RST-001" "marker 校验通过"
  else
    record "SMK-RESTORE" "恢复到新 ns → Completed+数据一致" "failed" "$((SECONDS-t0))" "TC-RST-001" "phase=$rphase marker='$marker'(期望 $RUN_ID)"
  fi
else
  record "SMK-RESTORE" "恢复到新 ns → Completed+数据一致" "skipped" "$((SECONDS-t0))" "TC-RST-001" "上游备份未 Completed，跳过"
fi

# ===== SMK-CLEANUP：删除 RP → Backup CR 级联清除 (TC-REG-010) =====
t0=$SECONDS
api DELETE "/api/v1/backups/${BACKUP_NAME}" >/tmp/del.json 2>&1
sleep 15   # 等 DeleteBackupRequest cascade
if kubectl -n "$VELERO_NS" get backup "$BACKUP_NAME" >/dev/null 2>&1; then
  record "SMK-CLEANUP" "删除 RP → Backup CR 清除" "failed" "$((SECONDS-t0))" "TC-REG-010" "Backup CR 仍存在(级联未完成?)"
else
  record "SMK-CLEANUP" "删除 RP → Backup CR 清除" "passed" "$((SECONDS-t0))" "TC-REG-010" "Backup CR 已级联删除"
fi

# ===== SMK-CSI：CSI 卷快照严格模式 → skipped =====
record "SMK-CSI" "CSI 卷快照备份(严格 TC-RP-001)" "skipped" "0" "TC-RP-001" "v1 用 ns 级备份；CSI 真快照需集群 VolumeSnapshotClass，列为后续增强"

finalize
[[ "$RESULT_FAILED" -gt 0 ]] && exit 1 || exit 0
