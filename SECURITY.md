# SupKube 安全策略与代码脆弱性检查（Security Policy）

> 本文档说明 SupKube 项目的安全工程实践、脆弱性扫描机制、SBOM 与签名策略，以及漏洞报告渠道。可直接交给客户安全部门审阅。
> **状态图例**：✅ 已实施 · 🟡 计划中（在 #113 task） · 🔬 调研

---

## 1. 漏洞报告渠道（Vulnerability Reporting）

发现 SupKube 安全漏洞，请通过以下方式私下报告（**勿在公开 issue / GitHub 讨论中披露**）：

- **Email**：`security@supkube.io`（PGP key 见附录 A，待补）
- **私有 GitHub Security Advisory**：https://github.com/<org>/supkube/security/advisories/new
- **响应 SLA**：
  - 24 小时内确认收到
  - 7 天内出初步评估（影响范围 + 严重性）
  - 高危/严重：30 天内出修复 + CVE
  - 中低危：随下个 minor release

---

## 2. 代码脆弱性扫描矩阵（Security Scanning Matrix）

| 层 | 工具 | 检测什么 | 频率 | 阻断? | 状态 |
|---|---|---|---|---|---|
| **SAST · Go** | `golangci-lint`（含 `gosec`、`staticcheck`、`errcheck` 等） | Go 源码常见弱点（SQL 注入、命令注入、弱加密、unsafe pointer、错误检查缺失） | 每个 PR | ✅ 阻断 | 🟡 计划中 |
| **SAST · 前端** | `eslint` + `eslint-plugin-security` | JS/Vue 源码弱点（XSS、unsafe eval、prototype pollution 等） | 每个 PR | ✅ 阻断 | 🟡 计划中 |
| **SAST · K8s/Helm** | `helm lint` + `kubesec`/`checkov`/`kube-linter` | Helm template / K8s manifest 安全配置（特权容器、hostPath、RBAC 过度授权） | 每个 PR | ⚠ 警告 | 🟡 计划中 |
| **SCA · Go 依赖** | `govulncheck`（Go 团队官方 CVE 扫描器） | Go module 中已知 CVE | 每个 PR + 每日定时 | ✅ 阻断（高/严重） | 🟡 计划中 |
| **SCA · 前端依赖** | `npm audit` + Dependabot | npm 包已知 CVE | 每个 PR + 每日定时 | ✅ 阻断（高/严重） | 🟡 计划中 |
| **SCA · 容器镜像** | `trivy image` | 镜像 OS 包 + 应用依赖 CVE + 错误配置 | 每次构建镜像 | ✅ 阻断（高/严重） | 🟡 计划中 |
| **Secrets 扫描** | `gitleaks`（CI）+ GitHub 原生 Secret Scanning | 仓库历史 + PR 中泄露的 API key / 密码 | 每个 PR + 每周全量 | ✅ 阻断 | 🟡 计划中 |
| **Dockerfile 静态检查** | `hadolint` | Dockerfile 反模式（root 用户、未固定版本、ADD vs COPY） | 每个 PR | ⚠ 警告 | 🟡 计划中 |
| **License 合规** | `go-licenses` + `license-checker` | 引入禁用 license（GPL、AGPL 等） | 每个 release | ✅ 阻断 | 🟡 计划中 |
| **LLM Provider 风险** | 人工评审 + `trusted-provider` 白名单（§6.G） | LLM provider CVE / prompt-injection / 数据外泄历史；本地 Ollama 镜像走 trivy（同上 SCA · 容器镜像） | 每次新增 provider + 季度复审 | ✅ 阻断（未列入白名单 provider 不得发布） | 🟡 计划中（PRD-003 v1） |

---

## 3. 软件供应链安全（Supply Chain Security）

| 实践 | 工具 | 输出 | 状态 |
|---|---|---|---|
| **SBOM 生成** | `syft` | 每次 release 生成 SPDX/CycloneDX 格式 SBOM，附件发布 | 🟡 计划中 |
| **镜像签名** | `cosign`（Sigstore） | 镜像签名 + 用 Rekor 透明日志记录 | 🟡 计划中 |
| **依赖固定** | `go.sum` checksum + npm `package-lock.json` | 防止 dependency confusion 攻击 | ✅ 已实施 |
| **可重复构建** | multi-arch buildx + 固定 base image SHA | 同一 commit 可重复出位级一致镜像 | 🟡 部分（buildx 已用） |
| **provenance** | SLSA Level 2+（`slsa-github-generator`） | 镜像构建出处证明 | 🔬 v1.0 调研 |

---

## 4. 运行时安全（Runtime Security）

| 维度 | 实践 | 状态 |
|---|---|---|
| **K8s 最小权限** | Helm chart 默认装在 scoped ServiceAccount（非 cluster-admin）；多角色 RBAC（admin / editor / viewer / multicluster-admin） | ✅ 已实施（v0.8.5+） |
| **OIDC 认证** | Dex 集成 4 大 IdP（Google / GitHub / Microsoft / Okta），不自建本地用户系统 | ✅ 已实施 |
| **Container security context** | 非 root user、readOnlyRootFilesystem、drop ALL capabilities | 🟡 计划中（在 hardening sprint） |
| **NetworkPolicy** | backend pod 仅暴露 API 端口，BSL 出站显式允许 | 🟡 计划中 |
| **审计日志** | 用户每个操作都打 audit log（可推 SIEM） | ✅ 已实施（v0.8.x） |
| **Object Lock / WORM** | BSL Object Lock 默认开启，阻止勒索软件删除备份 | ✅ 已实施（v0.8.12 LBS3） |
| **数据加密** | 传输：BSL 走 TLS；静态：依赖 BSL provider（Azure Blob SSE / S3 SSE-KMS） | ✅ 已实施 |

---

## 5. 应用安全特性（直接对客户的安全功能）

| Feature | 描述 | 状态 |
|---|---|---|
| **还原时安全扫描** | YARA + ClamAV 双引擎扫描还原数据（防止把 ransomware 还原回去）| 🟡 v0.9.6 规划中（详见 ADR-030） |
| **3-2-1-1-0 备份原则** | 5 层模型（本地快照 + 本地 BSL + 云 BSL + 第 2 云 + 虚拟实验室）| ✅ 已实施 + 🟡 演进中（详见 ADR-031） |
| **0 错误验证** | 备份后 CRC 校验 + Kopia repo check | 🟡 v0.8.15 规划中 |
| **应用级恢复演练 (DR drill)** | 自动还原到沙箱 + 跑校验脚本 | 🟡 v0.9.7 规划中 |
| **跨集群隔离** | kubeconfig 存 K8s Secret，按集群命名空间隔离 | ✅ 已实施 |

---

## 6. AI 数据处理与出境治理（AI Data Handling & Egress Governance）

> 本章覆盖 SupKube 内嵌的所有 AI/LLM 推理场景，是 PRD-Review T4/X4 high-finding 的正式收口。
> 核心原则：**合规默认本地、SaaS 显式 opt-in、强制脱敏管线、出境白名单可见、100% 审计、客户随时一键关闭**。

### 6.A 适用范围（Scope）

本章约束 SupKube 内**所有**把集群数据送给 LLM 推理的场景：

| 场景 | 对应 PRD | 出境内容（默认） |
|---|---|---|
| AI Advisor (Application 健康评分 / 建议) | PRD-003 | namespace / labels / workload kind / image / PVC 元数据 |
| MCP Skills（AI 执行集群操作） | PRD-004 | tool 调用参数 + 必要上下文 |
| Log Viewer AI 根因分析 | PRD-005 §4.9 | 错误行 ±20 行日志（经脱敏） |
| Activity Timeline AI 排错 | PRD-006 §4.7 | stage status + errorMessage（经脱敏） |

> **凡是 SupKube 把集群数据送给 LLM 推理的场景，都受本章约束**。任何新 AI 功能在合入 main 前必须显式声明遵循 §6.B–§6.G。

---

### 6.B LLM Provider 选型与合规默认（Compliance Default）

**默认行为**：合规客户（等保 2.0 三级 / 金融 / 医疗 / 政务）**默认本地 Ollama**，数据**不出集群边界**。
**SaaS Provider** (DeepSeek / Claude / GPT 等)：**显式 opt-in**，客户必须在 Settings 勾选「我接受将集群元数据发送至 `<Provider>` 进行推理」才能启用。

| 客户类型 | 默认 Provider | 出境策略 | 状态 |
|---|---|---|---|
| 合规客户（默认） | Ollama（本地，集群内 pod） | 0 字节出境 | 🟡 计划中（PRD-003 v1） |
| 通用客户 | Ollama（本地） | 0 字节出境 | 🟡 计划中（PRD-003 v1） |
| 体验 / Demo | DeepSeek（SaaS） | 经脱敏后白名单字段出境 | 🟡 计划中（PRD-003 v1） |
| 企业自带 | BYO Key（客户自有 Endpoint） | 客户自负责，SupKube 仍走脱敏管线 | 🟡 计划中（PRD-003 v1） |

**关键约束**：
- **不存在「静默切到 SaaS」**：从 Ollama 切到任何 SaaS provider 必须 UI 二次确认 + 写审计。
- **离线/隔离网客户**：installer 检测无外网时强制锁定为本地 Ollama，禁止选择 SaaS provider。
- **trusted-provider 白名单**：仅以下 6 个 provider 允许配置（详见 §6.G），名称一字不差：
  1. `Ollama` (本地, 默认, 合规)
  2. `DeepSeek` (SaaS opt-in)
  3. `Claude` (SaaS opt-in)
  4. `Azure OpenAI` (SaaS opt-in)
  5. `GPT-4 系列` (SaaS opt-in, 含 GPT-4 / GPT-4-Turbo / GPT-4o)
  6. `BYO` (Bring Your Own, 客户自管)

  未在白名单 provider 不允许出现在 UI 下拉。

  **SSOT 约束**: 本列表与 架构设计.md ADR-033 §Provider 抽象 完全一致, 任何修改必须同时改两处, CI contract test 会 diff 字符串。

---

### 6.C 脱敏管线（Sanitization Pipeline）—— 强制前置

**所有外发 LLM 的数据必须经过此管线**，不允许各 PRD 各做一套。

#### 脱敏规则（强制）

| 数据类型 | 脱敏动作 |
|---|---|
| K8s Secret 值（`Secret.data.*` / `Secret.stringData.*`） | `***` |
| env 中 key 含 `PASSWORD` / `TOKEN` / `SECRET` / `KEY` / `CRED` / `AUTH` / `COOKIE` 的值 | `***` |
| 镜像 tag | **保留**（排查需要） |
| 镜像 registry URL 中的认证段 `user:pass@registry/...` | `***@registry/...` |
| 日志行中 JWT 模式（`eyJ[A-Za-z0-9_-]+\.eyJ...`） | `<JWT-redacted>` |
| 日志行中 IP 地址 | **保留**（排查需要） |
| 日志行中 email | `<email>` |
| 日志行中 IBAN / 信用卡号（Luhn 校验） | `<PII-redacted>` |
| 堆栈 trace（文件路径 + 行号） | **保留**（不算 PII，排查必需） |
| PV 内数据 / 文件块 | **绝不外发**（仅 PV metadata: name / SC / size / accessModes） |

#### 实施位置（单一来源）

- **唯一实现位置**：backend `internal/advisor/sanitize.go`
- **权威签名**（PRD-003 / PRD-005 / PRD-006 / PRD-004 都**必须**调用此唯一函数）：

  ```go
  // internal/advisor/sanitize.go
  type RedactedFieldInfo struct {
      Path       string  // JSON path (e.g., "spec.containers[0].env[2].value")
      Rule       string  // 规则 name (e.g., "k8s-secret-value", "jwt-pattern")
      OriginHash string  // SHA-256 of original value (审计可比对, 不存原文)
  }

  type SanitizeReport struct {
      RedactedCount  int                  // 总脱敏次数
      RedactedFields []RedactedFieldInfo  // 每字段详情
      Fingerprint    string               // SHA-256 of sanitized output (idempotency check)
  }

  func Sanitize(ctx context.Context, payload any) (sanitized any, report SanitizeReport, err error)
  ```

  - `sanitized`：脱敏后的 payload（结构与输入同构，敏感叶子节点替换为 `***` 或规则定义的占位符）
  - `report.RedactedCount` / `report.RedactedFields`：本次脱敏的明细，用于 §6.E 审计与白名单合规校验
  - `report.Fingerprint`：脱敏后 payload 的 SHA-256（幂等性校验 + §6.E `input_fingerprint`，**不存原文**）
- **单一来源**：该签名与 架构设计.md ADR-033 §脱敏管线 完全一致，由 `internal/advisor/sanitize.go` 实现，CI contract test 防止 drift。任何对该签名的修改必须同时更新 ADR-033 + SECURITY.md，否则视为架构违规。
- **禁止**：在前端、caller、provider adapter 等任何其它位置自行做脱敏（避免多套规则漂移）
- **测试**：sanitize.go 必须有**金标本测试集**（≥100 条覆盖所有规则 + 边界 case），CI 阻断回归
- **审计**：每次 sanitize 输出 `report.Fingerprint`（SHA-256），写入 §6.E 审计日志 `input_fingerprint` 字段

状态：🟡 计划中（PRD-003 v1 落地，作为所有 AI 功能的前置依赖）

---

### 6.D 出境白名单（Egress Whitelist）

所有 LLM 外发**必须显式列出「哪些字段会离开集群」**。客户在 `Settings → AI Advisor → 出境字段清单` 看到完整白名单，并可逐项关闭。

#### 默认白名单（最小化原则）

**PRD-003 Application 评分**（元数据外发）：
- `namespace`
- `labels`（已过滤 `kubernetes.io/` / `app.kubernetes.io/` 之外的自定义业务标签需 opt-in）
- `workload kind`（Deployment / StatefulSet / ...）
- `image`（registry 认证段已脱敏）
- `PVC count` + `SC name`（不含 PV 数据）
- `RTO/RPO declared`

**PRD-005 AI 根因**（日志外发）：
- 错误前后 **±20 行**，经 §6.C 脱敏管线
- 不外发完整日志文件、不外发其它容器日志

**PRD-006 AI 排错**（错误外发）：
- `stage status` + `errorMessage`（经脱敏）
- 关联 CR 的 `name` + `kind`（不含 `spec` / `data` / `status.conditions` 之外的字段）

#### 绝不外发清单（硬禁止）

- PV 内数据 / 文件块
- Secret 原值
- kubeconfig
- token / cookie / session id
- 客户业务数据（数据库内容、对象存储内容等）
- BSL credential
- BackupSchedule / Backup 中的存储 endpoint credential 字段

> 任何新增字段进入白名单需 PR review + 写入审计公告，客户可订阅变更。

状态：🟡 计划中（PRD-003 v1 完整实现 UI 与逐项开关）

---

### 6.E 审计要求（Audit）

每次 LLM 调用 **100% 写审计日志**，无例外。

#### 审计字段（最小集）

| 字段 | 说明 |
|---|---|
| `timestamp` | 调用时刻（RFC3339） |
| `provider` | `ollama` / `deepseek` / `claude` / `gpt-4` / `byo` |
| `caller` | `prd-003-advisor` / `prd-005-log-rca` / `prd-006-timeline-rca` / `prd-004-mcp` |
| `user` | 触发用户（OIDC subject） |
| `cluster` | 触发集群 ID |
| `input_fingerprint` | 脱敏后 payload 的 SHA-256（**不存原文**） |
| `output_digest` | LLM 返回前 200 char 摘要（用于排错，超出截断） |
| `egress_bytes` | 出境字节数（脱敏后） |
| `cost_estimate_usd` | 调用费用估算（按 provider token 计价） |
| `duration_ms` | 端到端耗时 |
| `whitelist_version` | 当时生效的 §6.D 白名单版本号 |

#### 留存与导出

- **保留期**：个人版 **90 天**；企业版 **180 天**（满足等保三级日志留存要求）
- **SIEM 推送**：复用 §4 现有审计推送机制（同一通道、同一签名密钥）
- **客户下载**：客户可在 `Settings → AI Advisor → 审计历史` 一键导出完整 AI 调用历史（JSON / CSV），用于第三方审计
- **不可篡改**：审计日志一旦写入即只读，删除需 cluster-admin + 二次确认 + 写元审计

状态：🟡 计划中（PRD-003 v1 落地，复用 §4 审计推送通道）

---

### 6.F 客户可关闭性（Customer Opt-Out）

**默认行为**：AI 功能整体**默认关闭**，直到客户在 Settings 主动开启（opt-in）。

#### 关闭开关

`Settings → AI Advisor → 完全关闭 AI 功能`（master switch）。关闭后：

- 所有 LLM 调用**立即停止**（in-flight 调用拒绝返回，前端显示「AI 已关闭」）
- UI 上 AI 相关入口**全部隐藏**：
  - PRD-003 AI Advisor 整体 tab 不渲染
  - PRD-005 §4.9 日志页 AI 根因按钮隐藏
  - PRD-006 §4.7 Timeline AI 排错按钮隐藏
  - PRD-004 MCP Skills 入口隐藏
- 已有 AI 结果**保留可读**（不删除历史推断），但**不再刷新**
- **关闭状态写入审计**（合规审计员可凭审计日志证明「客户在 T 时刻已主动关闭 AI」）

#### 分项开关（细粒度）

客户也可单独关闭某个调用方（不必全关）：
- `禁用 PRD-003 AI Advisor 评分`
- `禁用 PRD-005 日志 AI 根因`
- `禁用 PRD-006 Timeline AI 排错`
- `禁用 PRD-004 MCP Skills`

#### 重新开启

重新开启必须 cluster-admin 角色 + UI 二次确认「我接受 §6.B–§6.E 全部条款」+ 写审计。

状态：🟡 计划中（PRD-003 v1 落地 master switch + 分项开关）

---

### 6.G 与 §1 漏洞报告渠道 + §3 供应链安全的关系

| 议题 | 走哪条流程 |
|---|---|
| LLM provider 自身漏洞（prompt injection / 数据泄露 CVE / model jailbreak） | §1 漏洞报告渠道 → 评估是否要从 §6.B 白名单移除该 provider |
| 本地 Ollama 镜像 CVE / 镜像供应链问题 | §3 镜像供应链管理（trivy + cosign + SBOM），与其它镜像同等管控 |
| 新增 LLM provider 接入 | 必须先经 §2「LLM Provider 风险」评审 → 加入 §6.B trusted-provider 白名单 → 才能在 UI 下拉出现 |
| 脱敏管线漏洞（绕过 / 误判） | §1 漏洞报告渠道（**视为高危**：可能导致客户数据外泄） |
| 出境白名单字段变更 | PR review + 公告 + 客户审计订阅，不需走 §1 |

**硬约束**：不允许使用未列入 §6.B trusted-provider 白名单的 LLM provider。客户 BYO endpoint 必须显式声明 OpenAI-compatible 协议版本，且 SupKube 仍走 §6.C 脱敏管线。

---

> 本章对应 ADR-033 AI Advisor 架构（拟），详见 `架构设计.md`。

---

## 7. CI/CD 安全管道（Implementation Plan）

> 当前状态：**仓库尚未配置 GitHub Actions 工作流**（验证：`.github/workflows/` 不存在）。
> **task #113** 将一次性落地下面的完整 pipeline。

### 7.1 每次 PR 触发
```yaml
# .github/workflows/security-pr.yml （task #113 实施）
- gosec / golangci-lint        # SAST Go
- govulncheck                  # SCA Go CVE
- eslint + eslint-plugin-security  # SAST 前端
- npm audit --audit-level=high # SCA 前端 CVE
- gitleaks                     # Secrets
- hadolint                     # Dockerfile
- helm lint + kube-linter      # K8s manifest
```

### 7.2 每次 main 推送 / release 触发
```yaml
# .github/workflows/security-release.yml
- 构建 multi-arch 镜像
- trivy image scan
- syft 生成 SBOM (SPDX + CycloneDX)
- cosign 签名镜像
- 上传 SBOM + 签名到 release 资源
```

### 7.3 每日定时
```yaml
# .github/workflows/security-daily.yml
- govulncheck（对最新 main）
- npm audit
- trivy（对最新已发布镜像）
- 输出报告到 GitHub Security tab
```

---

## 8. 给客户安全部门的对外问答（FAQ）

> **Q：你们怎么扫描代码漏洞？**
> A：见 §2 矩阵 —— Go 用 `gosec` + `govulncheck`，前端用 `eslint-plugin-security` + `npm audit`，镜像用 `trivy`，secrets 用 `gitleaks`；所有扫描在 CI 每个 PR 触发，高/严重 CVE 阻断合并；外加每日定时全量扫描。CI 工作流见 §6（task #113 实施中）。

> **Q：每次发布的镜像有 SBOM 吗？**
> A：v1.0 起每个 release 附 SPDX + CycloneDX 双格式 SBOM（syft 生成）+ cosign 签名（task #113）。

> **Q：如何报告漏洞？**
> A：见 §1，私下报到 `security@supkube.io` 或 GitHub Security Advisory，24h 确认，高危 30 天内修复。

> **Q：备份数据本身怎么防勒索？**
> A：BSL 默认开 Object Lock（WORM），即使勒索软件拿到 K8s 权限也删不了备份（§4）；v0.9.6 + 还原时 YARA + ClamAV 双引擎扫描，防止把 ransomware 还原回生产（§5）。

> **Q：SupKube 本身需要什么集群权限？**
> A：backend 默认装在 **受限 SA**（非 cluster-admin），多角色 RBAC；客户可审计 helm chart 看到的所有 RBAC 资源。详见 USER_MANUAL.md §RBAC 章节。

> **Q：你们用 AI，我的数据会出境吗？**
> A：**默认本地 Ollama，0 字节出境**——合规客户（等保 / 金融 / 医疗 / 政务）安装即默认。SaaS provider（DeepSeek / Claude / GPT）是**显式 opt-in**，需在 Settings 二次勾选才能启用，且仅出境 §6.D 白名单字段（namespace / labels / workload kind / 脱敏后镜像名 / 错误日志 ±20 行等），**绝不外发** PV 数据 / Secret 原值 / kubeconfig / token。所有出境数据强制经 §6.C 脱敏管线，所有 LLM 调用 100% 写审计（§6.E），客户可一键完全关闭 AI（§6.F）。完整治理见 §6。

---

## 9. 合规与审计

| 标准 | 适用性 | 状态 |
|---|---|---|
| **CIS Kubernetes Benchmark** | 我们的 helm chart 部署遵循 CIS 推荐 | 🟡 自检中 |
| **SOC 2 Type II** | 中型客户常问 | 🔬 v1.0 准备 |
| **ISO 27001** | 大型/政府客户 | 🔬 v1.0+ |
| **GDPR / CCPA** | 看客户行业 | 🔬 文档化 |

---

## 附录 A：PGP Key（待补）

`security@supkube.io` 的 PGP 公钥指纹与 Key block 待补。

## 附录 B：相关 ADR / 文档

- `架构设计.md` ADR-030 还原时安全扫描架构
- `架构设计.md` ADR-031 5 层数据韧性模型
- `架构设计.md` ADR-033 AI Advisor 架构（拟，对应本文 §6）
- `USER_MANUAL.md` § RBAC + § OIDC 集成
- `PRODUCT-TIERS.md` Premium 套餐安全功能定位
- `PRD.md` PRD-003 AI Advisor / PRD-004 MCP Skills / PRD-005 §4.9 / PRD-006 §4.7（本文 §6 适用范围）

---

**文档版本**：
- **v0.2.2（2026-05-31）**：§6.B Provider 6 个 endpoint 跟 ADR-033 字符串完全对齐 (R2-A finding)。
- **v0.2.1（2026-05-31）**：§6.C 签名与 ADR-033 单一来源对齐（ADR-Review T4/X4 后续 finding #1）—— 锁定 `RedactedFieldInfo` / `SanitizeReport` / `Sanitize(ctx, payload) (sanitized, report, err)` 三元组，新增「单一来源」契约声明 + CI contract test 要求。
- **v0.2（2026-05-31）**：新增 §6 AI 数据处理与出境治理（PRD-Review T4/X4 finding 收口）；§2 加 LLM Provider 风险一行；§8 FAQ 加 AI 出境问答；原 §6/§7/§8 顺延为 §7/§8/§9。

**下次更新**：task #113 实施完成后 → 把 🟡 改为 ✅ + 附 GitHub Actions 链接 + 首次 SBOM/签名样例；PRD-003 v1 落地后把 §6 各子节 🟡 改为 ✅。
