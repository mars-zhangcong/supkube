# D-WAIT-014 — PRD-007 §4.3 「snapshot 型 BSL」判定 schema（config marker vs backup spec）

> **状态**：open（当前实现可工作，但 marker schema 未跟 SupKube CRD 对齐）｜**owner**：Mars｜**触发**：2026-06-04（PRD-007 §4.3 Layer 4 Transport TC-COPY-002 实施）
> **严重度**：🟡 中 — 不阻断当前 task。
> **重编号映射**：原以 `D-WAIT-COPY-002` 由 PRD-007 transport task 提出，2026-06-04 SCM 按 LEDGER Rule G 正式取号为 **D-WAIT-014**（旧 append 进 INDEX 大文件的写法违反「一事一文件」结构，已迁本文件）。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧标识 → 新号**：`COPY-002` → **D-WAIT-014**

## 实证
- PRD-007 §4.3 把「snapshot 型」判定锚在 **Backup CR** 的 `snapshotMoveData=false`（`preflight.go` 已实现，按 backup 粒度）。
- 但 TC-COPY-002 prompt 要求「BSL `spec.objectStorage` 中含 snapshot 类型字段 → 拒绝」，这是 **BSL 粒度**防御（防 preflight 被绕过）。
- BSL CR schema 由 Velero owner 定义（我们 import 不改，Rule C 共享契约锁），没有原生「snapshot type」字段。
- 现暂用 `BSL.Spec.Config["volumeDataSource"]="snapshot"` 作为约定 marker（透传 Velero free-form config map），跟 `transport.go` 的 `SnapshotBSLConfigKey`/`Value` 锁定。

## 两个选项（你拍）

### 方案 A（推荐）：沿用 `config.volumeDataSource=snapshot` 约定 marker
- 优点：不动 BSL CRD schema（Rule C），Helm 模板 / UI 表单可标这个 key，跨 provider 通用。
- 缺点：隐性约定，客户 `kubectl` 直编 BSL 可能漏标。

### 方案 B：引入 SupKube 自己的 CRD `BSLProfile`（annotation 或 sidecar CR）
- 优点：强类型 + UI 可控。
- 缺点：多 1 个 CRD，跟现有 BSL 列表关联复杂。

## 已实施
方案 A（`transport.go` `SnapshotBSLConfigKey="volumeDataSource"`）。单测 `bslSnapshot()` 助手依此构造。

## Note
主拦截路径仍是 `preflight.go` 按 backup 粒度（`snapshotMoveData=false`），transport BSL 标记只是第二道防御。
