# DEMO — 跨集群灾备演示（本地 → AKS）

> **场景**：在本地 docker-desktop 集群上备份一个示例应用 → 通过 Azure Blob (Cloud BSL) 跨集群同步 → 在 Azure AKS dev 集群上完整恢复。
>
> **演示时长**：30-45 分钟（含等待）。
>
> **受众**：客户、合作方、内部 dogfooding。客户复制即可重现。
>
> **配套文档**：[AZURE-SETUP.md](AZURE-SETUP.md)（基础设施）、[USER_MANUAL.md §22](../USER_MANUAL.md) DR Playbook、[USER_MANUAL.md §23](../USER_MANUAL.md) Helm 安装参考。

---

## 0. 演示故事板

```
┌──────────────────────┐                         ┌──────────────────────┐
│  docker-desktop      │                         │  AKS aks-jumborca-dev│
│  (本地 Mac)          │                         │  (Azure southeastasia)│
│                      │                         │                      │
│  ┌────────────────┐  │                         │  ┌────────────────┐  │
│  │ test-app ns    │  │                         │  │ test-app ns    │  │
│  │  · Deployment  │──┼──→ Backup ──→ Cloud BSL │  │  · Deployment  │  │
│  │  · PVC w/marker│  │     (Azure Blob)        │  │  · PVC w/marker│  │
│  │  · ConfigMap   │  │           ↓             │  │  · ConfigMap   │  │
│  └────────────────┘  │      Restore Point      │  └────────────────┘  │
│                      │           ↓             │           ↑          │
│  SupKube UI ─────────┼───────────┼─────────────┼─→ Restore Wizard    │
│                      │           ↓             │     target: AKS dev  │
│                      │      cross-cluster ─────┼──→  ✓ 恢复成功       │
└──────────────────────┘                         └──────────────────────┘
```

**演示亮点**：
1. **零 SSH 跨集群** — 全程在 SupKube UI / 一个 kubectl 终端
2. **数据真的过去了** — PVC 里的 marker 文件在目标集群可读
3. **耗时可量化** — 整个 backup → restore 30 秒内完成（小数据量）

---

## 1. 前置条件检查（演示前 10 分钟跑一次）

### 1.1 集群清单

| 集群 | 用途 | SupKube 版本 |
|---|---|---|
| `docker-desktop` (本地) | 源集群，跑 test-app | ≥ 0.9.0.2-alpha |
| `aks-jumborca-dev` (Azure) | 目标集群 | ≥ 0.9.1.0-alpha |

### 1.2 检查命令

```bash
# 1) 两个集群 kubectl 都通
kubectl --context docker-desktop get nodes
kubectl --context aks-jumborca-dev get nodes

# 2) 两个集群都装了 SupKube
kubectl --context docker-desktop -n supkube get pods
kubectl --context aks-jumborca-dev -n supkube get pods
# 期望: backend / frontend / dex 都 Running

# 3) 跑预检（可选但建议）
curl -fsSL https://charts.supkube.com/preflight.sh | bash    # 当前 context
kubectl config use-context aks-jumborca-dev
curl -fsSL https://charts.supkube.com/preflight.sh | bash    # 切完再跑
```

### 1.3 SupKube UI 入口

| 集群 | UI 访问方式 |
|---|---|
| docker-desktop | http://localhost:30888 （NodePort） |
| aks-jumborca-dev | `kubectl --context aks-jumborca-dev -n supkube port-forward svc/supkube-frontend 8081:80` → http://localhost:8081 |

> **本演示主要在 docker-desktop UI 完成**——v0.9.0 的 Mode Switcher 让你在一个 UI 里管理两个集群。

### 1.4 Azure Blob (Cloud BSL) 凭据

演示假设你已有一个 Azure Storage Account 用于 BSL。如果没有：

```bash
# 一次性创建（独立于 charts.supkube.com 的那个）
az group create --name supkube-demo --location southeastasia
az storage account create \
  --name supkubebackupsdemo \
  --resource-group supkube-demo \
  --location southeastasia \
  --sku Standard_LRS \
  --kind StorageV2

# 拿 storage account key
az storage account keys list \
  --account-name supkubebackupsdemo \
  --resource-group supkube-demo \
  --query '[0].value' -o tsv
# 把这串 key 记下来，下一步配 BSL 时要用

# 创建 backup container
az storage container create \
  --account-name supkubebackupsdemo \
  --name velero-backups \
  --auth-mode login
```

---

## 2. Step 1 — 在 docker-desktop 部署 test-app 示例应用

### 2.1 一键部署

```bash
kubectl config use-context docker-desktop
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: test-app
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-app-data
  namespace: test-app
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-app-config
  namespace: test-app
data:
  app.conf: |
    demo_date: "REPLACE_ME_AT_DEMO_TIME"
    customer: "客户名称"
    marker: "Cross-cluster restore demo via SupKube"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
          volumeMounts:
            - name: data
              mountPath: /usr/share/nginx/html
            - name: config
              mountPath: /etc/test-app
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: test-app-data
        - name: config
          configMap:
            name: test-app-config
EOF
```

### 2.2 写入一个"独一无二"的 marker（演示数据真实性的关键）

```bash
# 等 pod Ready
kubectl -n test-app wait --for=condition=ready pod -l app=test-app --timeout=60s

# 写一个带时间戳的 marker 文件到 PVC
TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
kubectl -n test-app exec deploy/test-app -- \
  sh -c "echo 'SupKube cross-cluster demo · $TIMESTAMP · 客户名称' > /usr/share/nginx/html/marker.txt"

# 验证写入成功
kubectl -n test-app exec deploy/test-app -- cat /usr/share/nginx/html/marker.txt
# 期望输出: SupKube cross-cluster demo · 2026-05-26 14:30:00 · 客户名称
```

> 这个 marker 是演示的"指纹"——客户能看到完全一致的字符串在 Azure AKS 上出现，证明数据真的过去了。

---

## 3. Step 2 — 在 SupKube UI 配置 Cloud BSL

### 3.1 打开 SupKube

浏览器访问 http://localhost:30888 ，admin 登录。

### 3.2 添加 Backup Storage Location (Azure Blob)

**导航**（v0.9.1.2 后 UI 重构）：
- **当前位置**: 左侧"系统设置"未直接暴露 BSL；过渡期请用直链 [http://localhost:30888/storage](http://localhost:30888/storage)（路由仍有效）。
- **后续位置**（v0.9.1.3 完成集群管理重构后）: Settings → 集群管理 → docker-desktop → Storage Class Management → BSL 子表。

进入页面后点右上 **+ 添加存储位置**。

**填写**：

| 字段 | 值 |
|---|---|
| Name | `azure-blob-demo` |
| Provider | `Azure` |
| Bucket | `velero-backups` |
| Config: `resourceGroup` | `supkube-demo` |
| Config: `storageAccount` | `supkubebackupsdemo` |
| Credentials → Account Key | 1.4 步取到的 key |
| Default | ☑ |

点 **创建**。等几秒，状态应该变为 `Available`（绿色）。

### 3.3 验证 BSL 健康

```bash
kubectl -n velero get backupstoragelocations
# 期望: azure-blob-demo  Available  default
```

如果状态卡在 `Unavailable`：
- 检查 storage account key 拷贝时有没有截断
- `kubectl -n velero logs deploy/velero | grep -i bsl` 看真实错误

---

## 4. Step 3 — 触发备份

### 4.1 方式 A：一键 Snapshot（最快，演示推荐）

**导航**：左侧 → **应用列表**（Applications）→ 找到 `test-app` 行 → kebab `⋮` → **Snapshot Now**

弹窗确认 → SupKube 立刻提交 Backup CR 到 Velero。

### 4.2 方式 B：通过 Policy（更"真实"，演示备选）

**导航**：左侧 → **策略制定** → **+ 创建策略**

| 字段 | 值 |
|---|---|
| Name | `demo-policy` |
| Namespaces | `test-app` |
| Cloud BSL | `azure-blob-demo` |
| Schedule | `Manual / On Demand` |

创建后回到列表 → kebab → **Run Once Now**。

### 4.3 观察执行

**导航**：左侧 → **可观测性** → 默认 tab 即"活动查看"（v0.9.1.2 改版后，活动 / 顾问 / 审计日志 / 日志查看器 四件套合并到这个 hub 里）

应该立刻看到一条 Backup action 出现：
- Phase: `InProgress` → 几秒后 → `Completed`
- 点行展开看 Application Items（PVC、ConfigMap、Deployment、Service... 都列出）

CLI 验证：
```bash
kubectl -n velero get backups
# 期望: 一个新 backup，status Completed

# 看具体备份了什么
kubectl -n velero get backup <backup-name> -o yaml | grep -A5 "progress:"
```

### 4.4 在 Azure Blob 确认数据真的到了

```bash
az storage blob list \
  --account-name supkubebackupsdemo \
  --container-name velero-backups \
  --auth-mode login \
  --query "[].name" -o tsv | head
# 期望: backups/<backup-name>/...  里有一堆文件
```

---

## 5. Step 4 — 切换到 AKS-dev 集群

### 5.1 通过 Mode Switcher 切换（无需重新登录）

SupKube UI 左上 → **Mode Switcher dropdown**（显示当前集群 `this-cluster`）→ 点 **aks-jumborca-dev**。

整个 UI 切换上下文。**数据全部来自远端 AKS-dev 集群**（通过 X-Supkube-Cluster header 路由——v0.9.0.1 MC 架构）。

### 5.2 验证切换生效

左侧 → **系统概览**——dashboard 数据应该是 AKS-dev 的（namespace 数量、cluster size 等都和本地不同）。

URL 应该包含 `?cluster=aks-jumborca-dev`。

### 5.3 备选：通过 Multi-Cluster Manager 跳

UI 左上 Mode Switcher → **Multi-Cluster Manager** → 进 MCM 仪表盘 → 表格里点 `aks-jumborca-dev` 行。

---

## 6. Step 5 — 在 AKS-dev 看到 Restore Point

### 6.1 同步 BSL 到 AKS-dev

**两种路径**：

**自动路径（推荐）** ← v0.9.0 MC4 已实现
当你在 docker-desktop 上跑 Cross-Cluster Restore 时（Step 6），SupKube backend 会自动把 BSL + secret copy 过去。**这步可跳过，直接看 6.3 验证**。

**手动路径**（如果想提前看 RP）
在 AKS-dev 的 SupKube UI（已切换过来后）→ 直链 `/storage` → **+ 添加存储位置** → 填和 docker-desktop 上完全一样的 BSL 配置。Velero 的 `BackupSyncController` 会自动从 Azure Blob 同步 backup metadata 进来。

### 6.2 等 BSL Sync

```bash
# AKS-dev 上
kubectl --context aks-jumborca-dev -n velero get backups
# 等 30-60 秒，应该出现刚在 docker-desktop 上做的那个 backup
```

> 这是 Velero 的内置能力——**任何配了同一 BSL 的集群都能看到 BSL 里的全部 backup**。SupKube 没改任何东西，只是把 BSL 配置自动 sync 过去。

### 6.3 在 UI 看到 RP

AKS-dev 的 SupKube UI → **应用还原**（v0.9.1.2 改名，前身"数据还原"）→ 应该看到刚才做的那个 RP，**带"Imported"标记**（表示来自其他集群）。

---

## 7. Step 6 — 跨集群恢复 Wizard

### 7.1 启动 Wizard

回到 docker-desktop 的 SupKube UI（如果你切到 AKS-dev 了切回来）→ Mode Switcher → 选 `this-cluster`。

**导航**：**数据还原** → 找到刚做的 RP → kebab → **Restore** → **Cross-Cluster Restore**。

### 7.2 填 Wizard

| 步骤 | 填什么 |
|---|---|
| **Step 1 · Target Cluster** | 选 `aks-jumborca-dev`（v0.9.0 MC3 引入） |
| **Step 2 · Namespace Mapping** | 默认 `test-app` → `test-app`（同名恢复） |
| **Step 3 · Resource Filters** | 默认全选 |
| **Step 4 · Preflight** | SupKube 自动检查目标集群（v0.7.12 引入）：StorageClass 兼容性 / 命名冲突 / RBAC |
| **Step 5 · Review** | 显示 plan 摘要；确认无误点 **Submit** |

### 7.3 等待 BSL + Backup 同步到 AKS-dev（v0.9.0 MC4）

后端会自动：
1. 把 docker-desktop 上的 BSL secret copy 到 aks-jumborca-dev 的 velero ns
2. 创建 BSL CR 在目标集群
3. 等 `BackupSyncController` 把 backup metadata 同步过来（90 秒超时）
4. 提交 Restore CR 到 aks-jumborca-dev

UI 上会看到 progress：`Preparing BSL on target` → `Waiting for backup sync` → `Submitting restore`。

### 7.4 切到 AKS-dev 观察恢复

UI → Mode Switcher → **aks-jumborca-dev** → **活动查看**

应该看到一条新的 Restore action：
- Phase: `InProgress` → 30-60 秒 → `Completed`
- 点开看 Resources Restored（PVC、ConfigMap、Deployment、Service... 全部）

---

## 8. Step 7 — 验证（关键的"指纹"环节）

### 8.1 命令行验证

```bash
kubectl --context aks-jumborca-dev -n test-app get all,pvc,cm
# 期望:
#   pod/test-app-xxxx                       Running
#   deployment.apps/test-app                1/1 available
#   persistentvolumeclaim/test-app-data     Bound
#   configmap/test-app-config               (我们的 demo configmap)

# 等 pod Ready
kubectl --context aks-jumborca-dev -n test-app \
  wait --for=condition=ready pod -l app=test-app --timeout=120s

# 关键时刻——读那个 marker 文件
kubectl --context aks-jumborca-dev -n test-app exec deploy/test-app -- \
  cat /usr/share/nginx/html/marker.txt
```

**期望输出**（与 Step 2.2 写的字符串**完全一致**）：

```
SupKube cross-cluster demo · 2026-05-26 14:30:00 · 客户名称
```

🎉 **这就是演示的高潮——同一字符串、同一时间戳、跨了一个公网、跨了两种 K8s（docker-desktop hostpath ↔ AKS managed-csi）、完整恢复。**

### 8.2 ConfigMap 验证

```bash
kubectl --context aks-jumborca-dev -n test-app get cm test-app-config -o jsonpath='{.data.app\.conf}'
# 期望: 包含 "marker: Cross-cluster restore demo via SupKube"
```

### 8.3 PVC 跨 StorageClass 验证

```bash
# 在 docker-desktop 上 PVC 用的是 hostpath SC:
kubectl --context docker-desktop -n test-app get pvc test-app-data -o jsonpath='{.spec.storageClassName}'
# 期望: hostpath

# 在 AKS-dev 恢复后 PVC 用的是 managed-csi:
kubectl --context aks-jumborca-dev -n test-app get pvc test-app-data -o jsonpath='{.spec.storageClassName}'
# 期望: default (AKS 的默认 SC) 或显式 storageClass
```

> **不同 StorageClass 的卷之间数据迁移** —— 这是 Velero filesystem backup (Kopia) 的核心能力。卷的"形式"变了，"内容"完全保留。这点 Kasten 也做，但 SupKube **不收费**。

---

## 9. 演示后清理（可选）

### 9.1 清掉 test-app（两边都做）

```bash
kubectl --context docker-desktop delete ns test-app
kubectl --context aks-jumborca-dev delete ns test-app
```

### 9.2 删 Restore Point（如果演示后不需要保留）

SupKube UI → 数据还原 → 找到刚才的 RP → kebab → **Delete**。
SupKube 会调 Velero 的 `DeleteBackupRequest` CRD 真正级联删除（v0.7.9 修复后保证云端数据也删）。

### 9.3 删 Azure 演示资源（可选）

```bash
az group delete --name supkube-demo --yes --no-wait
```

⚠️ 如果你想保留 BSL 给下次 demo 用就**不要**跑这个。

---

## 10. 常见问题排查

### 10.1 BSL 状态 `Unavailable`

```bash
kubectl -n velero logs deploy/velero | grep -i "azure-blob-demo"
```
常见原因：
- Storage account key 拷贝时缺末尾的 `==` 字符 → 重填
- Storage account name 拼错 → SupKube UI 重新编辑 BSL
- Network policy 阻断了 velero pod 出公网 → 检查 NetworkPolicy

### 10.2 Backup 卡在 `InProgress`

```bash
kubectl -n velero get backup <name> -o yaml | grep -A20 status
kubectl -n velero logs ds/node-agent | tail -50    # filesystem backup 走 node-agent
```
常见原因：
- node-agent DaemonSet 不在跑 → `kubectl -n velero get ds node-agent`
- PVC 不可读（pod 没起来） → 看 pod events

### 10.3 Cross-Cluster Restore 报 "BSL not synced after 90s"

v0.9.0 MC4 引入的超时。常见原因：
- 目标集群 Velero `BackupSyncController` 不健康 → `kubectl --context aks-jumborca-dev -n velero get backups` 看是否出现
- BSL secret 没同步 → `kubectl --context aks-jumborca-dev -n velero get secrets | grep azure`

解决：在 AKS-dev UI 手动 add BSL，等 60 秒，再重跑 Cross-Cluster Restore。

### 10.4 Restore 完成但 pod CrashLoopBackOff

```bash
kubectl --context aks-jumborca-dev -n test-app describe pod -l app=test-app | tail -30
```
常见原因：
- AKS 节点是 arm64 但 nginx 镜像只有 amd64 → 切镜像（演示用 nginx:1.27-alpine 是 multi-arch 不会有这问题）
- StorageClass 不支持 RWO + 集群多 zone → 改 PVC 用 RWX-capable SC

### 10.5 看不到 Marker 文件 / 文件内容是空

意味着数据没真正传输，只 metadata 备份了。检查：
```bash
kubectl --context docker-desktop -n velero get backup <name> -o yaml | grep -A5 podVolumeBackups
# 应该有 podVolumeBackups 字段（filesystem backup 走 node-agent + Kopia）
```
如果没有 podVolumeBackups → backup 时漏了 `--default-volumes-to-fs-backup` 或者 namespace 没标注 `backup.velero.io/backup-volumes`。

---

## 11. 进阶（演示后客户会问的）

### 11.1 "能否定时备份而不是手动 Snapshot？"

**展示 Policy + Schedule** —— 同样的 test-app，创建 daily 02:00 schedule，到点自动备份。

### 11.2 "如果 docker-desktop 集群整个挂了怎么办？"

**展示 DR Playbook** ([USER_MANUAL §22](../USER_MANUAL.md#22-灾备演练--dr-playbookv090-multi-cluster-manager))。客户在新集群 helm install supkube → 配同 BSL → 直接 restore。SupKube 自己也支持 backup（v0.9.10 Configuration Backup）。

### 11.3 "能不能只恢复某个文件而不是整个 namespace？"

**v0.9.3 路线图** —— 细粒度文件浏览 + 恢复，Advanced 层功能。

### 11.4 "License 怎么算？"

参考 [PRODUCT-TIERS.md](../PRODUCT-TIERS.md)。简短版：Foundation 免费，包含本演示的所有功能；Advanced 加 Log Viewer / 文件浏览 / EntraID / Kyverno；Premium 加 BPMN 灾备演练 / BIA / RA / AI Assistant。

### 11.5 "比 Kasten 强在哪？"

参考 [PRODUCT-TIERS.md §3.2](../PRODUCT-TIERS.md#32-supkube-的差异化)。简短版：开源 / 自托管 / 原生中文 / 不依赖外部 Prometheus + Grafana / 多架构（含 arm64）/ 即将上 AI。

---

## 12. 演示时长 cheat sheet

| 阶段 | 预估时间 | 关键动作 |
|---|---|---|
| Step 1 · 部署 test-app + marker | 2 min | kubectl apply + exec |
| Step 2 · 配 Cloud BSL | 3 min | UI 填表 + 等 Available |
| Step 3 · 触发备份 | 1 min trigger + 30-60s 等待 | UI 一键 Snapshot |
| Step 4 · 切到 AKS-dev | 5 sec | Mode Switcher dropdown |
| Step 5 · 看到 RP | 30-60 sec | BSL sync 等待 |
| Step 6 · Cross-Cluster Restore Wizard | 2 min 填表 + 30-60s 等待 | 5 步 wizard |
| Step 7 · 验证 marker | 1 min | kubectl exec |
| Q&A / 进阶 | 10-20 min | 看客户问什么 |
| **总计** | **30-45 分钟** | |

---

## 13. 安全说明（演示完客户必问）

| 问题 | 回答 |
|---|---|
| 备份数据加密吗？ | Velero + Kopia 是 client-side 加密（passphrase-based）。SupKube 默认禁用 passphrase（演示便利），生产建议 enable。 |
| BSL credentials 怎么存？ | K8s Secret，用 cluster encryption-at-rest。v0.9.4 后可选接 Azure KeyVault / HashiCorp Vault。 |
| 跨集群 kubeconfig 怎么管？ | SupKube Cluster CR `kubeconfigSecretRef` 引用一个 K8s Secret，admin-only RBAC 可写。 |
| 备份数据保留多久？ | 由 Policy `ttl` 字段控制；默认 30 天。Object Lock 模式下到期不可删（governance/compliance）。 |
| 谁能触发恢复？ | RBAC 控制。Editor 角色可在自己 ns scope 内 restore；Admin 全集群可恢复。 |

---

**演示完别忘了** — 让客户 download `preflight.sh` 在他们自己的集群跑一次，让他们看到 SupKube 立刻能给他们的真实环境画像。这是把 demo 转成"我也想装一个"的关键一步。
