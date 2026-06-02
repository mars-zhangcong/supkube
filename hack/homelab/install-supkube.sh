#!/usr/bin/env bash
# ============================================================================
# install-supkube.sh — 在 Homelab 上装好 Colima/k3s 自启 + 部署 SupKube
# ----------------------------------------------------------------------------
# 前提：已先跑过 setup-macos-always-on.sh。
# 在【那台 Homelab Mac】上、在本仓库目录里运行：
#   bash hack/homelab/install-supkube.sh
# ============================================================================
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
CHART="$HERE/../../supkube-helm/supkube"

echo "==> 1/6  安装依赖（colima / kubectl / helm / docker CLI）"
command -v brew >/dev/null || { echo "请先装 Homebrew: https://brew.sh"; exit 1; }
for f in colima kubernetes-cli helm docker; do
  brew list "$f" >/dev/null 2>&1 || brew install "$f"
done

echo "==> 2/6  安装 Colima 看门狗（开机自启 + 异常自愈）"
sudo cp "$HERE/supkube-colima-up.sh" /usr/local/bin/supkube-colima-up.sh
sudo chmod 755 /usr/local/bin/supkube-colima-up.sh
mkdir -p "$HOME/Library/LaunchAgents"
cp "$HERE/com.supkube.colima.plist" "$HOME/Library/LaunchAgents/com.supkube.colima.plist"
launchctl bootout "gui/$(id -u)/com.supkube.colima" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.supkube.colima.plist"

echo "==> 3/6  首次启动 Colima + k3s（VM 4vCPU/10GB/80GB，可能需数分钟）"
if ! colima status >/dev/null 2>&1; then
  colima start --kubernetes --cpu 4 --memory 10 --disk 80
fi
kubectl config use-context colima
kubectl get nodes

echo "==> 4/6  准备 Helm 依赖（Velero 子 chart 已随仓库内置）"
helm dependency build "$CHART" 2>/dev/null || true

echo "==> 5/6  部署 SupKube（内置 Velero v1.18 + 本地 MinIO 存储）"
helm upgrade --install supkube "$CHART" \
  --namespace supkube --create-namespace \
  -f "$HERE/values-homelab.yaml" \
  --wait --timeout 12m
kubectl -n supkube get pods

echo "==> 6/6  通过 Tailscale 把 UI 暴露到 tailnet（任意地方 HTTPS 访问）"
if command -v tailscale >/dev/null 2>&1; then
  sudo tailscale serve --bg 30888 || \
    echo "   tailscale serve 失败，可手动重试：sudo tailscale serve --bg 30888"
  echo "   访问地址（MagicDNS）："
  tailscale serve status 2>/dev/null || true
else
  echo "   未装 tailscale CLI，先本机访问：http://localhost:30888"
fi

cat <<'EOF'

✅ 部署完成。访问方式：
   · 本机：              http://localhost:30888
   · Tailscale（任意地方）：见上面 `tailscale serve status` 输出的 https://<host>.ts.net
   · 健康探针：          curl -s http://localhost:30888/api/v1/status

跨集群复制 Demo：编辑 values-homelab.yaml 取消 fingerprint.sharedSecret 注释，
在 homelab 与云上集群填同一个值，再用 SupKube UI 配 ImportPolicy。
EOF
