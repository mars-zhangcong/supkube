#!/usr/bin/env bash
# test-velero-ns-seam.sh — 断言 Velero 命名空间"接缝三端一致"。
#
# 幽灵配置永远活在接缝上。本次事故的接缝：
#   ┌ 后端读的 ns（configmap veleroNamespace → env VELERO_NAMESPACE）
#   ├ Velero 实际运行的 ns（velero Deployment 落地处）
#   └ BSL 所在 ns（本地 store BSL + 客户云 BSL）
# 三端必须是同一个 ns。事故时它们劈叉（后端读 velero、Velero 在 supkube），
# 没有任何单点测试会红 —— 因为每一端单看都"正常"。本测试比对两端是否一致，
# 这正是单点测试抓不到、而幽灵配置必然现形的地方。
#
# 两种模式：
#   （默认）渲染态 —— `helm template` 后静态断言，CI 里跑，PR 阶段就拦住。
#   --live      现场态 —— 对真集群 kubectl 断言，FED 现场体检用，补上
#                 hack/velero-preflight.sh（查 Velero 自身健康）没覆盖的
#                 ns 一致性维度。
#
# 用法：
#   bash hack/test-velero-ns-seam.sh                  # 渲染态（CI）
#   bash hack/test-velero-ns-seam.sh --live           # 现场态（当前 kubecontext）
#
# 退出码：0 = 三端一致；1 = 接缝劈叉；2 = 工具/前置缺失。

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHART="$ROOT/supkube-helm/supkube"
MODE="template"
[[ "${1:-}" == "--live" ]] && MODE="live"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
die2()  { red "工具错误：$*"; exit 2; }

# ───────────────────────── 渲染态（CI gate） ─────────────────────────
if [[ "$MODE" == "template" ]]; then
  command -v helm >/dev/null || die2 "未装 helm"
  command -v python3 >/dev/null || die2 "未装 python3"
  # charts/velero-*.tgz 是 gitignored，fresh checkout（含 CI / worktree）为空。
  # 必须先 add repo 再 build，否则渲染不出 velero 子 chart，测试会假阴。
  if ! ls "$CHART"/charts/velero-*.tgz >/dev/null 2>&1; then
    helm repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts >/dev/null 2>&1 || true
    helm dependency build "$CHART" >/dev/null 2>&1 \
      || die2 "helm dependency build 失败（无法拉取 velero 子 chart；CI 请在 helm-lint job 内、deps 已 build 后运行本测试）"
  fi

  # 用 supkube 作为 release ns 渲染（与 install-supkube.sh / cd.yaml 一致）。
  RENDER_FILE="$(mktemp)"
  trap 'rm -f "$RENDER_FILE"' EXIT
  helm template supkube "$CHART" --namespace supkube \
    --set eula.accept=true \
    --set auth.dex.publicURL=http://localhost:30888 \
    --set localStore.enabled=true >"$RENDER_FILE" 2>/tmp/seam.err \
    || { red "helm template 失败："; cat /tmp/seam.err; exit 2; }

  # python 读文件路径（不能用 `python3 - <<HEREDOC`：heredoc 会占住 stdin，
  # 渲染内容就进不去了）。
  python3 - "$RENDER_FILE" <<'PY'
import sys, re
docs = open(sys.argv[1]).read().split('\n---\n')
backend_ns = velero_ns = None
bsl_ns = []

def field(d, key):
    m = re.search(r'^\s*%s:\s*(\S+)' % key, d, re.M)
    return m.group(1).strip('"') if m else None

for d in docs:
    kind = field(d, 'kind')
    name = None
    mm = re.search(r'^metadata:\s*\n(?:.*\n)*?\s*name:\s*(\S+)', d, re.M)
    # robust name pickup
    nm = re.search(r'^\s{2}name:\s*(\S+)', d, re.M)
    name = nm.group(1).strip('"') if nm else None
    ns_m = re.search(r'^\s{2}namespace:\s*(\S+)', d, re.M)
    ns = ns_m.group(1).strip('"') if ns_m else None

    # backend deployment: read VELERO_NAMESPACE source (configMapKeyRef →
    # resolve the configmap value rendered elsewhere). Simpler: the configmap
    # itself carries veleroNamespace.
    if kind == 'ConfigMap' and name and name.endswith('-config'):
        v = re.search(r'veleroNamespace:\s*(\S+)', d)
        if v:
            backend_ns = v.group(1).strip('"')
    if kind == 'Deployment' and name == 'velero':
        velero_ns = ns
    if kind == 'BackupStorageLocation':
        bsl_ns.append((name, ns))

errs = []
if backend_ns is None: errs.append("找不到后端 configmap 的 veleroNamespace")
if velero_ns is None:  errs.append("找不到 velero Deployment")
if backend_ns and velero_ns and backend_ns != velero_ns:
    errs.append(f"后端读 ns={backend_ns} ≠ Velero 运行 ns={velero_ns}（接缝劈叉！）")
for n, ns in bsl_ns:
    if velero_ns and ns != velero_ns:
        errs.append(f"BSL {n} 在 ns={ns} ≠ Velero ns={velero_ns}（孤儿 BSL，无人 reconcile）")

print(f"  后端读取 ns      : {backend_ns}")
print(f"  Velero 运行 ns   : {velero_ns}")
for n, ns in bsl_ns:
    print(f"  BSL {n:<28}: ns={ns}")
if errs:
    print("\n❌ 接缝劈叉：")
    for e in errs: print("   -", e)
    sys.exit(1)
print("\n✅ 三端一致（后端 == Velero == BSL）")
PY
  rc=$?
  [[ $rc -eq 0 ]] && green "渲染态接缝测试：通过。" || red "渲染态接缝测试：未通过。"
  exit $rc
fi

# ───────────────────────── 现场态（FED 体检） ─────────────────────────
command -v kubectl >/dev/null || die2 "未装 kubectl"

BACKEND_POD=$(kubectl get pods -A -l app.kubernetes.io/component=backend \
              -o jsonpath='{.items[0].metadata.name} {.items[0].metadata.namespace}' 2>/dev/null)
[[ -z "$BACKEND_POD" ]] && die2 "找不到 supkube backend pod（label app.kubernetes.io/component=backend）"
read -r BPOD BNS <<<"$BACKEND_POD"

backend_ns=$(kubectl -n "$BNS" exec "$BPOD" -- printenv VELERO_NAMESPACE 2>/dev/null)
[[ -z "$backend_ns" ]] && backend_ns="(未设置→后端回退 velero)"

velero_ns=$(kubectl get deploy velero -A -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
[[ -z "$velero_ns" ]] && velero_ns="(集群无 velero Deployment)"

echo "  后端读取 ns（VELERO_NAMESPACE）: $backend_ns"
echo "  Velero 运行 ns                 : $velero_ns"

fail=0
if [[ "$backend_ns" != "$velero_ns" ]]; then
  red "❌ 接缝劈叉：后端读 ns=$backend_ns ≠ Velero 运行 ns=$velero_ns"
  red "   → 后端探测不到 Velero/BSL，存储桶全 Unknown（本次事故根因）。"
  fail=1
fi
# BSL 一致性
while read -r ns name; do
  [[ -z "$name" ]] && continue
  if [[ "$ns" != "$velero_ns" ]]; then
    red "❌ BSL $name 在 ns=$ns ≠ Velero ns=$velero_ns（孤儿 BSL，无人 reconcile → Unknown）"
    fail=1
  fi
done < <(kubectl get bsl -A -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' 2>/dev/null)

echo
if [[ $fail -ne 0 ]]; then
  red "现场态接缝测试：未通过。"
  echo "   恢复见 RUNBOOK：helm upgrade -n $velero_ns 让后端 VELERO_NAMESPACE 对齐 Velero ns，再重建孤儿 BSL。"
  exit 1
fi
green "现场态接缝测试：通过（三端一致）。"
exit 0
