# D-WAIT-013 — PRD-007 §4.3 Layer 4 lifecycle 冲突错误码命名（拟 `ERR_LIFECYCLE_RETENTION_CONFLICT`）

> **状态**：open（ADR-035 错误码注册前必须定名）｜**owner**：Mars｜**触发**：2026-06-04（PRD-007 §4.3 Layer 4 Transport 实施 · `feat/prd-007-layer4-transport`）
> **严重度**：🟡 中 — 不阻断当前 task，但 ADR-035 错误码注册前必须定名。
> **重编号映射**：原以 `D-WAIT-COPY-001` 由 PRD-007 transport task 提出，2026-06-04 SCM 按 LEDGER Rule G 正式取号为 **D-WAIT-013**（旧 append 进 INDEX 大文件的写法违反「一事一文件」结构，已迁本文件）。INDEX 见 [`../等待决策.md`](../等待决策.md)。
>
> **旧标识 → 新号**：`COPY-001` → **D-WAIT-013**

## 实证
- PRD-007 §4.5 已注册 `ERR_LIFECYCLE_LOCK_CONFLICT`（lifecycle delete 时间 < Object Lock retention → WORM 拒删）。
- PRD-007 §4.3 Layer 4 联动新增冲突场景：Velero Policy retention > target BSL lifecycle delete-after → 复制过去的备份比 source 早删，Layer 4 冗余被静默削弱。
- 上述两冲突**语义独立**（WORM 不可删 vs 时间窗短缺），现 transport task 提议新名 `ERR_LIFECYCLE_RETENTION_CONFLICT`。

## 两个选项（你拍）

### 方案 A（推荐）：独立成 code `ERR_LIFECYCLE_RETENTION_CONFLICT`
- 优点：跟 `ERR_LIFECYCLE_LOCK_CONFLICT` 语义并列，UI 错误条文案 / KB 跳转可分别写。
- 缺点：错误码表多 1 条（但 ADR-035 命名规范允许）。

### 方案 B：合并成 `ERR_LIFECYCLE_CONFLICT` 单一 code，sub-reason 字段区分
- 优点：错误码表少 1 条。
- 缺点：UI / 文档对外解释复杂度上升（一个 code 两种 reason）。

## 已实施（待你拍后回写）
`lifecycle.go` 用了 `ErrLifecycleRetentionConflict` 常量 + 同名字符串，单测断言此 code。Mars 拍定后改字符串即可。

## Note
ADR-035 错误码注册表 `docs/err-codes.md` 在当前 task 视野外（Accepted 条件 §5 要求「同 PR 内」落）。PRD-007 §4.3 task scope 只动 4 新文件、不改 `docs/`。如需注册请 Mars 在合并时一并加。
