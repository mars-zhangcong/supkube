# AZURE-SETUP.md — 一次性 Azure 资源准备

> **目的**：在 Azure 上准备好 SupKube 制品分发所需的两个基础设施：
> - **Azure Container Registry** (`supkube.azurecr.io`) — 存放镜像
> - **Azure Blob Storage Static Website** (`charts.supkube.com`) — 当 Helm repo
>
> 跑完后给我下面 4 个值，我把它们填进 `hack/publish-release.sh`：
> ```
> ACR_LOGIN_SERVER      e.g. supkube.azurecr.io
> STORAGE_ACCOUNT       e.g. supkubecharts
> STATIC_WEB_ENDPOINT   e.g. https://supkubecharts.z13.web.core.windows.net/
> RESOURCE_GROUP        e.g. supkube-shared
> ```

---

## 前置条件

- 已 `az login`，订阅切到 SupKube 用的那个：
  ```bash
  az account set --subscription "<your-subscription-id>"
  ```
- 准备好一台 Linux/macOS 工作站，安装 `az` CLI（已有）+ `helm` + `docker`
- `supkube.com` 域名已注册（你说过——这一步我们已搞定）
- Cloudflare 账号能管 `supkube.com` 的 DNS（或者其他能加 CNAME 的 DNS 服务）

---

## Step 1 · 选 / 建 Resource Group

如果你的 AKS 集群已经在某个 RG 里（比如 `Research_and_Development`），直接复用——
ACR + Storage Account 体量很小，跟 AKS 同 RG 走同一份账单/RBAC 反而方便。

```bash
# Option A：复用现有 RG（推荐）
export RG=Research_and_Development          # ← 你的实际 RG 名
export REGION=southeastasia                 # 用你 AKS 集群同 region，避免跨 region 流量费

# Option B：新建独立 RG（适合长期客户分发，想独立计费时）
# export RG=supkube-shared
# export REGION=southeastasia
# az group create --name "$RG" --location "$REGION"
```

> ⚠️ **Region 必须用短名（全小写无空格）**，不是 portal 上的显示名。
>
> | Portal 显示 | az CLI 短名 |
> |---|---|
> | Southeast Asia | `southeastasia` |
> | East Asia | `eastasia` |
> | West US 2 | `westus2` |
> | East US 2 | `eastus2` |
>
> 用错了会报 `LocationNotAvailableForResourceType`——bash 的空格会把
> `"Southeast Asia"` 切成两半，第一半被当 region，第二半被丢掉。
>
> 全量列表：`az account list-locations --query "[].name" -o tsv`

---

## Step 2 · 创建 Azure Container Registry

```bash
export ACR_NAME=supkube       # 全 Azure 唯一；如果名字被占了，加后缀如 supkube01

# ⚠️ SKU 必须用 Standard 或 Premium——Basic 不支持 anonymous pull（Azure
# 2024 年起把这个能力限到了 Standard+）。Standard ~$20/月，100 GB 存储、
# 10 webhooks，对我们绰绰有余。
# 变量都加引号——如果 $REGION 含空格（说明你用错了显示名），引号能让 az
# 报 "location 无效" 而不是被 shell 拆成两个参数。
az acr create \
  --resource-group "$RG" \
  --name "$ACR_NAME" \
  --sku Standard \
  --location "$REGION"

# **关键**：开启 anonymous pull，客户拉镜像不用 az/docker login
az acr update --name "$ACR_NAME" --anonymous-pull-enabled true

# 验证
az acr show --name "$ACR_NAME" --query '{sku:sku.name,loginServer:loginServer,anonymousPullEnabled:anonymousPullEnabled}'
# 期望输出:
# {
#   "sku": "Standard",
#   "loginServer": "supkube.azurecr.io",
#   "anonymousPullEnabled": true
# }
```

**记下 `loginServer` 的值** → 这是 `ACR_LOGIN_SERVER`。

> **关于 anonymous pull 的安全性**: 客户 pull 镜像不需要凭据；但 push 仍需 `az acr login`。
> Pull 是公开下载——和 Docker Hub 公开镜像、quay.io 公开镜像同性质。
> 我们的镜像本来就要开源分发，没有秘密泄露风险。

---

## Step 3 · 创建 Storage Account + Static Website

```bash
export STORAGE_ACCOUNT=supkubecharts   # 全 Azure 唯一；3-24 字符，全小写数字

# StorageV2 + Standard_LRS（本地冗余即可，制品丢了我们重发就行）。
# 静态网站功能要求 kind=StorageV2。
az storage account create \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RG" \
  --location "$REGION" \
  --sku Standard_LRS \
  --kind StorageV2 \
  --access-tier Hot \
  --allow-blob-public-access true

# 开启静态网站。--404-document 和 --index-document 不会被 Helm 用到，
# 但 Azure 要求两者都设——给个无害默认。
az storage blob service-properties update \
  --account-name $STORAGE_ACCOUNT \
  --static-website \
  --404-document 404.html \
  --index-document index.html \
  --auth-mode login

# 拿到静态网站的 HTTPS endpoint
az storage account show \
  --name $STORAGE_ACCOUNT \
  --query 'primaryEndpoints.web' \
  --output tsv
# 输出类似: https://supkubecharts.z13.web.core.windows.net/
```

**记下整段 URL** → 这是 `STATIC_WEB_ENDPOINT`。

> Helm chart 会上传到名为 `$web` 的特殊容器（静态网站功能自动创建）。
> 不要去手动建 `$web` 容器——它已经存在。

---

## Step 4 · 自定义域名 `charts.supkube.com`（Cloudflare Worker 代理）

> **2026-05-26 教训沉淀**: 我们最初打算用普通 Cloudflare CNAME + SSL "Full"，
> 但发现 Azure Blob Static Website 同时校验 TLS SNI **和** HTTP Host header，
> 任何一个不是 `supkubecharts.z23.web.core.windows.net` 都返回 400。
> Cloudflare 的 Origin Rules Host/SNI 改写在 2024 年被移到了 **Enterprise plan**
> ($200/月)，Free plan 用不了。
>
> 解法是上一个 **5 行的 Cloudflare Worker** 做反向代理。Free plan 给 10 万
> req/天，比真实 `helm install` 流量大 100 倍。具体代码见
> `hack/cloudflare-worker-charts-proxy.js`（仓库内副本）。

### 4a. 把 supkube.com 加进 Cloudflare

1. 注册 Cloudflare 账号（如已有跳过）
2. **+ Add a domain** → 输入 `supkube.com` → Free plan
3. Cloudflare 给你 2 个 nameserver，类似 `elisabeth.ns.cloudflare.com` / `miguel.ns.cloudflare.com`
4. 去 supkube.com 的 registrar（腾讯云 / GoDaddy / ...）改 nameserver 指向这两个
5. 等 10-60 分钟生效，Cloudflare dashboard 上 supkube.com 状态 = **Active**
6. **不要**手动加 `charts` 的 CNAME——Worker 绑定时会自动建路由

### 4b. 创建 Worker

Cloudflare → **Compute → Workers & Pages → Create application → Workers**
→ 选 **Hello World** 模板 → 起名 `charts-azure-proxy` → **Create and deploy**。

部署完点 **Edit code**，把整个文件替换为 `hack/cloudflare-worker-charts-proxy.js`
的内容。点右上 **Deploy**。

### 4c. 绑定自定义域名

Worker 详情页 → **Settings → Domains & Routes → + Add → Custom Domain**
→ 填 `charts`（提示".supkube.com"会自动拼）→ **Add Domain**。

Cloudflare 自动签 edge 证书 + 建内部路由，30 秒-1 分钟完成。

### 4d. 验证

```bash
curl -I https://charts.supkube.com/index.yaml
# 期望:
#   HTTP/2 200
#   content-type: application/yaml
#   server: cloudflare
#   x-ms-request-id: <id>           ← 这个 header 出现说明 Worker 真把
#                                      请求转发到了 Azure（而不是命中
#                                      Cloudflare 缓存）
```

### 4e. （可选）SSL/TLS 模式校验

进 **SSL/TLS → Overview**，模式应该是 **Full**。Worker 路径其实绕开了
这个设置，但保持 Full 防止以后你又加普通 CNAME 时踩坑。**绝对不要 Flexible**。

> **以后想换 CDN 或换 backend**: 只改 Worker 代码里的 `url.hostname` 一行，
> `helm repo` 的 `index.yaml` 用相对路径所以**一行不用改**，零客户感知。

---

## Step 5 · 验证 publish 权限（push 路径）

发布脚本会以你的 `az login` 身份 push，无需创建额外 service principal。
跑一次 dry-run 确认权限链路：

```bash
# ACR push 权限
az acr login --name $ACR_NAME
# 期望: "Login Succeeded"

# Blob upload 权限——你的账号需要 "Storage Blob Data Contributor" 角色
az role assignment create \
  --role "Storage Blob Data Contributor" \
  --assignee $(az ad signed-in-user show --query id -o tsv) \
  --scope $(az storage account show --name $STORAGE_ACCOUNT --query id -o tsv)
# 已分配过会报 "RoleAssignmentExists"——无害

# 测试上传一个空文件
echo "hello" > /tmp/test.txt
az storage blob upload \
  --account-name $STORAGE_ACCOUNT \
  --container-name '$web' \
  --name test.txt \
  --file /tmp/test.txt \
  --auth-mode login \
  --overwrite

curl https://charts.supkube.com/test.txt
# 期望: hello

# 清理测试文件
az storage blob delete \
  --account-name $STORAGE_ACCOUNT \
  --container-name '$web' \
  --name test.txt \
  --auth-mode login
```

如果最后那个 `curl` 拿到了 "hello"，**Step 1-5 全部通过，你这边的活儿干完了。**

---

## Step 6 · 把这 4 个值告诉我

回到对话里贴这 4 行：

```
ACR_LOGIN_SERVER=supkube.azurecr.io
STORAGE_ACCOUNT=supkubecharts
STATIC_WEB_ENDPOINT=https://supkubecharts.z13.web.core.windows.net/
RESOURCE_GROUP=supkube-shared
```

（如果有名字被占用、region 不同等改动，按你实际的值填）

我把它们写进 `hack/publish-release.sh` 顶部的 config 区，然后跑一次 publish v0.9.0.2-alpha，
我们在 `aks-jumborca-dev` 上验证客户的 `helm repo add` + `helm install` 真能跑通。

---

## FAQ

**Q: 不用 Cloudflare，直接绑 Azure CDN 行不行？**
A: 行。Azure CDN (Microsoft tier) 同样能给 `*.web.core.windows.net` 套自定义域名 + 自动证书。
   只是 portal 操作步骤更多、调试反馈慢。Cloudflare 是更轻的方案；任何时候都能切。

**Q: ACR 我要不要 geo-replication？**
A: 现在不用。Basic SKU 单 region。客户多到全球分布、首屏 pull 超 30s 时再升 Premium 加 replica。

**Q: 资源会有多少钱？**
A:
- ACR Standard: ~$20/month flat + minor egress（注：anonymous pull 要 Standard 起步，Basic 不支持）
- Storage Account Standard_LRS: < $1/month（chart 几 MB）
- Cloudflare: $0（Free plan 够用）
- 总计 ~$21/month

> 早期 Azure ACR Basic SKU 是支持 anonymous pull 的，2024 年这能力被限到了
> Standard+。如果以后客户量大需要 geo-replication，再升 Premium (~$50/月)。

**Q: 出了问题怎么删干净重来？**
A: `az group delete --name $RG --yes --no-wait`——RG 一删，里面的 ACR + Storage Account 一起没。
   Cloudflare CNAME 单独删掉。
