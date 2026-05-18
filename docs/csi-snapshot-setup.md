# CSI Snapshot Setup — SupKube on Docker Desktop K8S

> Built 2026-05-17~18 · Verified end-to-end on docker-desktop K8S 1.32 · Velero v1.18.0
> Purpose: 让本地集群具备 CSI 卷快照能力，使 Velero 备份能调用 CSI 链路（不只是 Restic/Kopia fs 备份）。

## 为什么需要这一步

Docker Desktop K8S 默认只有 `hostpath` StorageClass，provisioner 是 `docker.io/hostpath`，**不是 CSI**，不支持快照。SupKube v0.5.x 备份只能走 fs 模式。要让 v0.6 的"双模式备份"在本地可演示/可开发，必须先搭这套基础。

**生产环境**：不要用 csi-driver-host-path（单节点 hostpath，重启 node 数据丢）。换成 ebs.csi、pd.csi、Longhorn、OpenEBS 等真实 CSI driver。本文流程对生产仍适用，只是 Step 2 换 driver。

## 前置条件

- Docker Desktop 已开 Kubernetes（验证：`kubectl get nodes` 应返回 `docker-desktop Ready`）
- Velero 已部署到 `velero` namespace（v1.14+ 才内置 CSI 集成；v1.13 及更早需要 `velero-plugin-for-csi` 插件，本文不覆盖）
- 本机能拉 `registry.k8s.io` 镜像（国内首次拉镜像耗时较长，参考下方"耗时预期"）

## 四步搭建

### Step 1 — external-snapshotter (CRDs + Controller)

提供 `VolumeSnapshot` / `VolumeSnapshotContent` / `VolumeSnapshotClass` 三个 CRD，以及一个监听这些资源、调用 CSI driver 的 controller。所有 CSI 快照能力的前提。

```bash
SNAP_VER=v8.0.1   # 跟 K8s 1.32 兼容的稳定版

kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml

kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml

kubectl rollout status deploy/snapshot-controller -n kube-system --timeout=600s
```

**验证**：

```bash
kubectl get crd | grep snapshot.storage.k8s.io   # 应有 3 个 CRD
kubectl get pods -n kube-system | grep snapshot-controller   # 应该 2/2 Running
```

**Snapshot-controller deployment 在 `kube-system` ns，label `app.kubernetes.io/name=snapshot-controller`**（不是 `app=snapshot-controller`，selector 易写错）。

### Step 2 — CSI Driver (csi-driver-host-path 参考实现)

```bash
git clone --depth 1 https://github.com/kubernetes-csi/csi-driver-host-path /tmp/csi-driver-host-path
cd /tmp/csi-driver-host-path
./deploy/kubernetes-latest/deploy.sh   # 注册 CSIDriver + StatefulSet + Service
kubectl apply -f ./examples/csi-storageclass.yaml   # 创建 StorageClass csi-hostpath-sc

# 注意：当前仓库已不再提供 csi-snapshotclass.yaml；deploy.sh 内部已创建 VolumeSnapshotClass
# `csi-hostpath-snapclass`，如未自动创建可手动写一个：
cat <<'EOF' | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-hostpath-snapclass
driver: hostpath.csi.k8s.io
deletionPolicy: Delete
EOF
```

**耗时预期**：csi-hostpathplugin Pod 包含 **8 个容器**（hostpath + 7 个 sidecar：external-health-monitor、node-driver-registrar、livenessprobe、attacher、provisioner、resizer、snapshotter + socat），首次拉镜像可能 15~25 分钟（国内网络）。

**验证**：

```bash
kubectl get sc                                  # 应有 csi-hostpath-sc
kubectl get volumesnapshotclass                 # 应有 csi-hostpath-snapclass
kubectl get csidriver                           # 应有 hostpath.csi.k8s.io
kubectl get pod csi-hostpathplugin-0            # 应该 8/8 Running
```

### Step 3 — Velero CSI 启用

**Velero v1.14+ 已把 CSI 集成移入 core**，不需要插件，只要打开 feature flag + 加 RBAC。

```bash
# 3.1 给 VolumeSnapshotClass 打 velero 识别标签
kubectl label volumesnapshotclass csi-hostpath-snapclass \
  velero.io/csi-volumesnapshot-class=true --overwrite

# 3.2 给 velero ServiceAccount 加 snapshot.storage.k8s.io 权限
cat <<'EOF' | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: velero-csi-snapshot
rules:
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshots", "volumesnapshotcontents", "volumesnapshotclasses"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: velero-csi-snapshot
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: velero-csi-snapshot
subjects:
- kind: ServiceAccount
  name: velero
  namespace: velero
EOF

# 3.3 启用 EnableCSI feature flag
kubectl patch deploy velero -n velero --type='json' -p='[
  {"op":"replace","path":"/spec/template/spec/containers/0/args","value":["server","--features=EnableCSI","--uploader-type=kopia"]}
]'

kubectl rollout status deploy/velero -n velero --timeout=120s
```

### Step 4 — 端到端验证

```bash
# 4.1 建一个 CSI PVC
kubectl create namespace test-csi 2>/dev/null || true
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-test-pvc
  namespace: test-csi
spec:
  accessModes: [ReadWriteOnce]
  resources: { requests: { storage: 1Gi } }
  storageClassName: csi-hostpath-sc
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/csi-test-pvc -n test-csi --timeout=60s

# 4.2 验证 CSI snapshot 本身工作
cat <<'EOF' | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: csi-test-snap-1
  namespace: test-csi
spec:
  volumeSnapshotClassName: csi-hostpath-snapclass
  source:
    persistentVolumeClaimName: csi-test-pvc
EOF
kubectl wait --for=jsonpath='{.status.readyToUse}'=true \
  volumesnapshot/csi-test-snap-1 -n test-csi --timeout=60s

# 4.3 Velero 备份带 CSI 快照
velero backup create csi-test-1 --include-namespaces=test-csi --snapshot-volumes
kubectl wait --for=jsonpath='{.status.phase}'=Completed \
  backup/csi-test-1 -n velero --timeout=120s

# 关键确认: csiVolumeSnapshotsCompleted 应 = csiVolumeSnapshotsAttempted ≥ 1
kubectl get backup csi-test-1 -n velero -o json | \
  jq '{phase: .status.phase, csiAttempted: .status.csiVolumeSnapshotsAttempted, csiCompleted: .status.csiVolumeSnapshotsCompleted}'

# 4.4 完整 restore 验证
kubectl delete pvc csi-test-pvc -n test-csi
velero restore create --from-backup csi-test-1 --include-namespaces=test-csi
# PVC 应自动恢复回来
```

## 常见问题

### snapshot-controller 长时间 `ContainerCreating`

国内首次拉 `registry.k8s.io/sig-storage/snapshot-controller` 可能 10-15 分钟。`kubectl describe pod` 查看 Events 确认是拉镜像中。

### csi-hostpathplugin-0 长时间 0/8

8 个 sidecar 容器逐个拉，每个几分钟。**不要重启 pod**，会重新计时。耐心等。最终应该 8/8 Running。

### Velero backup 不创建 VolumeSnapshot

检查清单：
- [ ] BSL Phase = Available
- [ ] PVC 用 `csi-hostpath-sc`（不是 `hostpath`）
- [ ] velero deploy args 含 `--features=EnableCSI`
- [ ] velero ClusterRole 含 `snapshot.storage.k8s.io` 权限
- [ ] VolumeSnapshotClass 含 label `velero.io/csi-volumesnapshot-class=true`
- [ ] 备份命令带 `--snapshot-volumes`

### Velero 备份完成后 VolumeSnapshot 不见了

**这是 v1.18 的正确行为**：Velero 备份完成后立即清理临时 VolumeSnapshot CR，**快照数据已经移到 object storage**（受 `snapshotMoveData` 控制，默认 false 时只用 CSI snapshot 本身，仍然在底层存储；true 时移到 BSL）。`status.csiVolumeSnapshotsCompleted` 是权威进度计数。

## 移植到生产

把 Step 2 换成生产 CSI driver。其它步骤不变：

| 云/环境 | 推荐 driver | 备注 |
|---|---|---|
| AWS EKS | `aws-ebs-csi-driver` | 自带快照支持 |
| GCP GKE | `pd.csi.storage.gke.io` | 节点自动安装 |
| Azure AKS | `disk.csi.azure.com` | 节点自动安装 |
| 自建 / 私有云 | **Longhorn** 或 **OpenEBS** | 多节点、有 snapshot |
| 高级用例 | **Rook-Ceph CSI** | 块/文件/对象通吃 |

Step 1/3/4 完全一样。

## 清理（卸载）

```bash
# 删除测试 ns
kubectl delete ns test-csi

# 卸载 csi-driver-host-path
/tmp/csi-driver-host-path/deploy/kubernetes-latest/destroy.sh

# 卸载 external-snapshotter
SNAP_VER=v8.0.1
kubectl delete -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
kubectl delete -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$SNAP_VER/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
kubectl delete crd volumesnapshots.snapshot.storage.k8s.io volumesnapshotcontents.snapshot.storage.k8s.io volumesnapshotclasses.snapshot.storage.k8s.io

# 回退 Velero
kubectl delete clusterrolebinding velero-csi-snapshot
kubectl delete clusterrole velero-csi-snapshot
kubectl patch deploy velero -n velero --type='json' -p='[
  {"op":"replace","path":"/spec/template/spec/containers/0/args","value":["server","--features=","--uploader-type=kopia"]}
]'
```
