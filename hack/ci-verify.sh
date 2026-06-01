#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  ci-verify.sh <namespace> <expected-version>                              ║
# ║                                                                          ║
# ║  verify-before-ship 的 CI 版（ENGINEERING.md Rule D 的流水线落地）。       ║
# ║  部署后证明"跑的是新代码、权限没漂"，否则 exit 1 让 CD 变红。              ║
# ║                                                                          ║
# ║  复用 hack/dev-deploy.sh Phase 5 的验证哲学，但去掉本地 docker daemon /    ║
# ║  双集群部分，专为单集群 CI 上下文设计（kubectl 已由调用方 set-context）。   ║
# ║                                                                          ║
# ║  4 步硬验证：                                                            ║
# ║    1. rollout 真收敛（不是 ImagePull/CrashLoop 卡住）                     ║
# ║    2. 容器镜像 tag == 期望版本（探活真容器名，规避 set-image 静默 no-op）  ║
# ║    3. /api/v1/status.buildStamp 是今天（证明不是缓存旧镜像）              ║
# ║    4. ServiceAccount 能跨 ns list pod（#79 静默 RBAC 丢失防护）           ║
# ║                                                                          ║
# ║  退出码: 0=全过; 1=任一验证失败                                          ║
# ╚══════════════════════════════════════════════════════════════════════════╝
set -euo pipefail

NS="${1:?用法: ci-verify.sh <namespace> <version>}"
VER="${2:?缺少版本号 (例: 0.9.1.10-alpha)}"

DEPLOY_BACKEND=supkube-backend
DEPLOY_FRONTEND=supkube-frontend
NS_VELERO="${NS_VELERO:-velero}"
STATUS_PATH="${STATUS_PATH:-/api/v1/status}"

# GitHub Actions 把 ::error:: 渲染成红色注解；非 CI 环境就当普通 echo。
fail() { echo "::error::$*" >&2; exit 1; }
ok()   { echo "  ✅ $*"; }

echo "▶ verify-before-ship: ns=$NS version=$VER"

# ── 1) rollout 真收敛 ──────────────────────────────────────────────
kubectl -n "$NS" rollout status "deploy/$DEPLOY_BACKEND"  --timeout=3m \
  || fail "[$NS] $DEPLOY_BACKEND rollout 未在 3m 内收敛（ImagePullBackOff / CrashLoopBackOff？跑 kubectl -n $NS describe pod -l app=$DEPLOY_BACKEND）"
kubectl -n "$NS" rollout status "deploy/$DEPLOY_FRONTEND" --timeout=3m \
  || fail "[$NS] $DEPLOY_FRONTEND rollout 未在 3m 内收敛"
ok "rollout 已收敛（backend + frontend）"

# ── 2) 容器镜像 tag == 期望版本 ────────────────────────────────────
# 用 containers[0].image 取真实运行镜像；末尾必须以 :$VER 结尾（相等而非包含，
# 防止某 init 容器的 tag 被误判）。
got=$(kubectl -n "$NS" get "deploy/$DEPLOY_BACKEND" \
        -o jsonpath='{.spec.template.spec.containers[0].image}')
[[ "$got" == *":$VER" ]] \
  || fail "[$NS] backend 镜像 tag 不符：期望 *:$VER，实得 '$got'（可能 helm set 的 tag 没生效，或 set-image 写错容器名静默 no-op）"
ok "backend 镜像 tag 匹配：$got"

# ── 3) buildStamp 是今天 ───────────────────────────────────────────
# buildStamp 格式 YYMMDD-HHMM（Dockerfile 里 date -u +%y%m%d-%H%M 注入）。
# 不比对精确分钟（编译耗时使 stamp 可能比部署时刻早 1-2 分），只校验"今天"，
# 足以抓住"部署成功但跑的是缓存旧镜像"的回归。
today=$(date -u +%y%m%d)
stamp=$(kubectl -n "$NS" exec "deploy/$DEPLOY_BACKEND" -- \
          sh -c "wget -qO- http://localhost:8080${STATUS_PATH} 2>/dev/null || curl -sf http://localhost:8080${STATUS_PATH}" \
          2>/dev/null | sed -n 's/.*"buildStamp":"\([^"]*\)".*/\1/p')
[[ -n "$stamp" ]] \
  || fail "[$NS] 读不到 ${STATUS_PATH}.buildStamp（空响应？端口 8080 不通？）"
[[ "$stamp" == "$today"* ]] \
  || fail "[$NS] buildStamp 非今日：得 '$stamp'，期望 '${today}-*'（强烈提示跑的是缓存旧镜像，不是本次构建）"
ok "buildStamp 是今日：$stamp (today=$today)"

# ── 4) #79 防护：SA 能跨 ns list pod ───────────────────────────────
# helm 升级曾静默丢掉 ClusterRoleBinding，UI 返回空 pod 列表且无任何报错。
# kubectl auth can-i 用 --as 模拟 SA，"no" 即权限缺失。
sa=$(kubectl -n "$NS" get "deploy/$DEPLOY_BACKEND" \
       -o jsonpath='{.spec.template.spec.serviceAccountName}')
sa="${sa:-default}"
[[ "$sa" == "default" ]] && echo "  ⚠ deploy/$DEPLOY_BACKEND 未设 serviceAccountName，用 ns default —— 本身就是配置 smell"
for tgt in "$NS" "$NS_VELERO"; do
  ans=$(kubectl auth can-i list pods -n "$tgt" --as="system:serviceaccount:${NS}:${sa}" 2>/dev/null || echo "no")
  [[ "$ans" == "yes" ]] \
    || fail "[$NS] SA $sa 不能 list ns/$tgt 的 pod（auth can-i=$ans）—— #79 静默 RBAC 丢失特征。查: kubectl get clusterrolebinding | grep $sa"
done
ok "SA $sa 可跨 ns list pod（$NS + $NS_VELERO）"

echo "✅ verify-before-ship 全部通过：ns=$NS version=$VER buildStamp=$stamp"
