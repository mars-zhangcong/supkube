#!/usr/bin/env bash
# check-phantom-config.sh — 把"幽灵配置"变成红板，而不是靠研发记性。
#
# 座右铭（ENGINEERING.md Rule I）：
#   治理办法是没用的，有用的是永远不给错误发生的条件。
#
# 幽灵配置 = 一个看起来权威、但没有任何东西真正消费它的配置项。本次 Velero
# ns 事故由两个幽灵配置叠加：
#   1. values.yaml `velero.namespaceOverride` —— 子 chart 12.0.1 根本不读
#   2. 后端 80+ 处硬编码 "velero" ns —— 让注入的 VELERO_NAMESPACE env 形同虚设
#
# 这道墙在 CI / pre-commit 把两类回潮挡在源头：
#   A) 后端重新出现裸 "velero" 命名空间字面量（绕过 internal/velerons 单一真源）
#   B) values.yaml 重新出现 namespaceOverride（死值）
#
# 用法：
#   bash hack/check-phantom-config.sh            # 扫描，违规即 exit 1
#   （CI 在 .github/workflows/ci.yaml 的 backend job 调它）
#
# 退出码：0 = 干净；1 = 发现幽灵配置回潮；2 = 工具自身错误。

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND="$ROOT/supkube-backend"
HELM="$ROOT/supkube-helm"
fail=0

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[33m%s\033[0m\n' "$*"; }

# ─────────────────────────────────────────────────────────────────────────
# 检查 A — 后端裸 "velero" 命名空间字面量
# ─────────────────────────────────────────────────────────────────────────
# 只命中"确属 ns"的形态，与 internal/velerons 收口时用的同一组 pattern。
# 故意 NOT 命中（合法）：protected-ns 列表 case "velero",／map key "velero":／
# label "velero.io/..."／plugin ID／资源名 .Get(ctx,"velero")／历史迁移标记
# LegacyNamespace = "velero"（= 赋值，非 := / 非 Namespace: 字段）。
# 测试文件(_test.go) 允许字面量（env 未设时 velerons.Namespace()=="velero"）。
FORBIDDEN='Namespace:[[:space:]]*"velero"|InNamespace\("velero"\)|Deployments\("velero"\)|Secrets\("velero"\)|:=[[:space:]]*"velero"|DefaultQuery\("namespace",[[:space:]]*"velero"\)'

hits_a=$(grep -rnE "$FORBIDDEN" "$BACKEND" --include='*.go' 2>/dev/null | grep -v '_test.go' || true)
if [[ -n "$hits_a" ]]; then
  red "❌ 幽灵配置 A：后端出现裸 \"velero\" 命名空间字面量（绕过 internal/velerons 单一真源）"
  echo "$hits_a"
  echo
  yellow "   修复：改用 velerons.Namespace()（import \"github.com/supkube/supkube-backend/internal/velerons\"）。"
  yellow "   背景：见 internal/velerons/velerons.go 顶部 + ENGINEERING.md Rule I。"
  echo
  fail=1
else
  green "✅ A：后端无裸 velero-ns 字面量（单一真源 internal/velerons 未被绕过）"
fi

# ─────────────────────────────────────────────────────────────────────────
# 检查 B — values.yaml 死值 namespaceOverride
# ─────────────────────────────────────────────────────────────────────────
# velero 子 chart 12.0.1 用 .Release.Namespace，不读 namespaceOverride。任何
# 取消注释的 `namespaceOverride:` 都是死值，会让人误以为能分 ns。注释里提到它
# （讲解为什么不能用）是允许的，所以只卡"行首非注释的 key"。
hits_b=$(grep -rnE '^[[:space:]]*namespaceOverride[[:space:]]*:' "$HELM" --include='*.yaml' 2>/dev/null || true)
if [[ -n "$hits_b" ]]; then
  red "❌ 幽灵配置 B：values 里出现生效态 namespaceOverride（velero 子 chart 不读它，是死值）"
  echo "$hits_b"
  echo
  yellow "   修复：删掉它。要换 Velero ns 用 config.veleroNamespace（见 _helpers.tpl supkube.veleroNamespace）。"
  echo
  fail=1
else
  green "✅ B：values 无生效态 namespaceOverride 死值"
fi

echo
if [[ $fail -ne 0 ]]; then
  red "幽灵配置守卫：未通过。上面的配置项没有真实消费者 / 是死值 —— 不给错误发生的条件。"
  exit 1
fi
green "幽灵配置守卫：通过。"
exit 0
