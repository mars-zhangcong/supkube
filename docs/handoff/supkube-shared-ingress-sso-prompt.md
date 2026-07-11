# 修复请求 · 把 SupKube 收编进统一域名 + 共享 SSO 网关

> 给「SSO / 网关侧」(管 `jumborca-dev` 共享 ingress + kb4ai oauth2-proxy + Keycloak 那边)的提示词。
> 用户报「SupKube 进不去」,真因 = SupKube 是这台共享主机上唯一没挂上来的产品。

---

## 现象 / 根因
- 用户访问 `https://jumborca-dev.southeastasia.cloudapp.azure.com/supkube/` → **404 (nginx)**。
- `kubectl -n supkube get ingress` → **空**。SupKube 没有任何 ingress 挂在该共享主机上。
- 它现在跑在**自己独立的 LoadBalancer** `http://4.144.200.141`(35 天前建,早于"全员收口共享 ingress 子路径"的约定),且**自带一套 Dex OIDC 登录**(admin/admin,issuer 钉死在 `http://4.144.200.141/dex`)。
- 其它产品(hub / kbui / gitlab / portal / fozl / jdc…)都已在该主机子路径 + 共享 SSO 网关(`oauth2-proxy.kb4ai` → Keycloak)后面。**SupKube 是唯一漏网的**。

## 目标
让 SupKube 与全队一致:`https://jumborca-dev…/supkube/` + HTTPS + 走同一个 oauth2-proxy/Keycloak 网关 SSO,后端不再自管登录。

## 这是个两侧协作的修复
| 谁 | 做什么 |
|---|---|
| **你(SSO/网关侧)** | 加 `/supkube` 的 ingress(照搬 hub 模式,挂到现有 oauth2-proxy 网关)+ 签 TLS。下方有可直接 apply 的 YAML。 |
| **SupKube 侧(我)** | 前端用 `base=/supkube` 重构(现用根绝对路径,挂子路径会白屏);后端改信任网关身份头 `X-Auth-Request-Email`、**退役自带 Dex**。 |

> 上线顺序:**先等 SupKube 侧出 `base=/supkube` 的前端 + 后端走身份头**,再 apply ingress——否则会 200 但白屏 / 双重登录。我这边好了会通知你。

## 你要做的(可直接 apply)

照搬 hub 的 ingress,只把 `hub`→`supkube`、路径→`/supkube`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: supkube
  namespace: supkube
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/auth-url: http://oauth2-proxy.kb4ai.svc.cluster.local/oauth2/auth
    nginx.ingress.kubernetes.io/auth-signin: https://jumborca-dev.southeastasia.cloudapp.azure.com/oauth2/start?rd=$escaped_request_uri
    nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-Email,X-Auth-Request-User
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: jumborca-dev.southeastasia.cloudapp.azure.com
    http:
      paths:
      - path: /supkube(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: supkube-frontend
            port:
              number: 80
  tls:
  - hosts:
    - jumborca-dev.southeastasia.cloudapp.azure.com
    secretName: supkube-tls
```

要点(都与 hub 一致):
- `auth-url` / `auth-signin` 指向**现有** oauth2-proxy——网关是主机级的,**无需新建 Keycloak client**,未登录会自动 302 到 SSO。
- `rewrite-target: /$2` + `use-regex` + `path: /supkube(/|$)(.*)` —— 把 `/supkube` 前缀剥掉再转给 `supkube-frontend`(配合前端 `base=/supkube` 一起生效)。
- `cert-manager letsencrypt-prod` + `secretName: supkube-tls` —— 复用主机证书签发,~8s 出 HTTPS。

## 请你确认 / 注意的坑
1. **网关身份头**:确认 oauth2-proxy 会注入 `X-Auth-Request-Email` / `X-Auth-Request-User`(hub/kbui 已在用)——SupKube 后端会读它做身份。
2. **别动 SupKube 的 Dex**:SupKube 自带 Dex 的 issuer/redirect 钉死在 `http://4.144.200.141`,**别试图把这套 Dex 搬到子路径**(历史上"照旧文档部署 Dex 崩")。正解是**网关 SSO 替代 Dex**,SupKube 后端走身份头,我这边退役 Dex。
3. **cookie / 放行域**:oauth2-proxy 本是主机级网关,通常自动覆盖新子路径;若 `/supkube` 被网关拦在白名单外,放进去即可。

## 关键事实(给你查 / 用)
- ns:`supkube` · 前端 svc:`supkube-frontend` :80(ClusterIP `10.0.36.248`) · 现 LB:`4.144.200.141`(收编后可保留兜底或回收)
- 网关:`oauth2-proxy.kb4ai.svc.cluster.local` · 主机:`jumborca-dev.southeastasia.cloudapp.azure.com`(ingress IP `104.43.76.64`)
- 金标准参照:`kubectl -n hub get ingress hub -o yaml`
- 用户当前可用的临时入口(收编完成前):`http://4.144.200.141`(admin/admin)
