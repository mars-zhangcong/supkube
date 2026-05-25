# SupKube UI 规范 v1

> 这是 SupKube 前端**唯一权威**的视觉与交互规范。
>
> 任何新页面、新组件、改版重构 PR 都必须先对照这份规范，**不一致即应拒绝合并**。
>
> 规则变了？先改这份文档，再改代码 —— 规范在前，实现在后。

---

## 0. 哲学

SupKube 是给**集群管理员**用的企业级数据保护工具，不是消费级 SaaS。它的视觉风格要做到：

1. **严肃**：不靠彩色 emoji 抢眼，不靠插画装饰。专业感来自字体层次和留白，不是色彩。
2. **密度可控**：管理员一天看几十次，密度过高会疲劳；密度过低又显得弱。中等密度。
3. **可扫读**：一眼就能找到关键信息，不需要"读完整段"才理解。
4. **可信任**：每个数字、状态、链接的语义都精确；不要"看起来差不多"的东西其实意思不一样。
5. **节制使用颜色**：颜色是稀缺资源。蓝代表"可点击"、绿代表"成功"、红代表"失败"、紫代表"Exported 这一概念"—— 用满了就废了。

参照对象：**Kasten K10**（专业感）、**Linear**（密度）、**Stripe Dashboard**（可信任）。
**不**参照：消费级 SaaS（Notion / Slack）。

---

## 1. 字体层次

### 1.1 Type scale

| Token | 用途 | 大小 | 字重 | 颜色 | letter-spacing | line-height |
|---|---|---|---|---|---|---|
| `--sk-h1` | 抽屉 / 页面标题（"Restore Point Details"） | 13px | 600 大写 | `#909399` | 0.08em | 1.4 |
| `--sk-h2` | 主体名（backup 名、app 名、ns 名） | 22px | 700 | `#1d2129` | -0.01em | 1.3 |
| `--sk-h3` | 分组节标题（"ARTIFACTS"、"PHASES"、"LABELS"） | 11px | 700 大写 | `#909399` | 0.08em | 1.5 |
| `--sk-h4` | 子表格表头（"NAME"、"STATUS"） | 11px | 600 大写 | `#909399` | 0.05em | 1.5 |
| `--sk-body` | 字段值、表格行 | 13px | 400 | `#303133` | 0 | 1.5 |
| `--sk-body-strong` | 强调字段值（数值、状态） | 13px | 600 | `#1d2129` | 0 | 1.5 |
| `--sk-caption` | 时间戳、辅助说明、tooltip 内容 | 11px | 500 | `#909399` | 0 | 1.4 |
| `--sk-mono` | backup 名、cron、kubectl 命令 | 12px | 500 | `#1d2129` SF Mono Menlo monospace | 0 | 1.4 |

### 1.2 Kasten-style 抽屉标题模式

抽屉头部由**两行**组成 —— **H1 在上，H2 在下**：

```
RESTORE POINT DETAILS                        ← H1 灰、小、大写、字间距
test-app-backup-snapshot-export-azure-...    ← H2 黑、大、粗
[COMPLETED]  [Backup]  [📸 Snapshot half]    ← chip 行（详见 §3.3）
```

- H1 写**对象类型**（"Restore Point Details" / "Action Details" / "Application Details" / "Policy Details"）
- H2 写**具体名字**（资源 metadata.name）
- 名字过长不换行 → 中间省略，鼠标 hover 显示完整名（`title=` 属性）

### 1.3 严禁

- ❌ 标题和正文用同一字号 / 同一字重
- ❌ 把 H2 当 H1 用（"backup 名"直接当抽屉头）
- ❌ 在 H3 节标题前面加 emoji
- ❌ 把链接写成与正文相同的颜色，仅靠下划线区分

---

## 2. 颜色 token

颜色**只用 token**，**不在组件里写硬编码色值**。下面这套写进 `src/styles/tokens.css`，所有组件 `var(--sk-color-xxx)`。

### 2.1 中性 / 灰阶

| Token | Hex | 用途 |
|---|---|---|
| `--sk-text` | `#1d2129` | 主要正文文字（H2、body-strong） |
| `--sk-text-secondary` | `#303133` | 次要正文（body 默认） |
| `--sk-text-muted` | `#606266` | 提示文字 |
| `--sk-text-caption` | `#909399` | H3、caption、disabled |
| `--sk-text-placeholder` | `#c0c4cc` | 输入框 placeholder、零值 |
| `--sk-border` | `#ebeef5` | 表格、卡片、分组边线 |
| `--sk-border-light` | `#f0f3f8` | dashed 内分割线 |
| `--sk-bg-soft` | `#f9fafc` | 分组 header 浅底、卡片底 |
| `--sk-bg-page` | `#ffffff` | 页面主背景 |
| `--sk-bg-hover` | `#f5f7fa` | hover 浅底 |

### 2.2 主色 / 可交互

| Token | Hex | 用途 |
|---|---|---|
| `--sk-primary` | `#4f46e5` | **唯一**可点击链接色（indigo） |
| `--sk-primary-hover` | `#3730a3` | 可点击 hover 文字色 |
| `--sk-primary-bg-hover` | `#eef2ff` | 可点击 hover 浅底 |
| `--sk-primary-active` | `#e0e7ff` | 可点击 active 按下底色 |

> ⚠️ **绝对规则**：UI 上**只有**可点击文字 / 按钮用 indigo。如果一段文字是 indigo 但点不动，是 bug。

### 2.3 状态色（status badge / tag）

| Token | Hex | 状态 |
|---|---|---|
| `--sk-status-running` | `#409eff` | running / InProgress |
| `--sk-status-success` | `#67c23a` | completed / Compliant |
| `--sk-status-warning` | `#e6a23c` | partial / Unmanaged |
| `--sk-status-error` | `#c45656` | failed |
| `--sk-status-muted` | `#909399` | skipped / paused / unknown |

### 2.4 类型色（语义化 chip）

| Token | Hex | 含义 |
|---|---|---|
| `--sk-type-snapshot` | `#409eff` 蓝 | Snapshot RP（cluster-local） |
| `--sk-type-exported` | `#722ed1` 紫 | Exported RP（BSL） |
| `--sk-type-imported` | `#13c2c2` 青 | Imported RP（跨集群） |
| `--sk-type-metadata` | `#909399` 灰 | Metadata-only |

### 2.5 严禁

- ❌ 把语义色用在装饰（"觉得紫色好看"就把某个 Kind 涂紫）
- ❌ 把多个语义挤进同一个色（蓝同时表示"可点击"+"运行中"+"snapshot"）
- ❌ 在普通字段值上抹颜色
- ❌ 用斜体表示"次要"或"特殊"（斜体在 K8s 上下文没有语义；要弱化用 caption token）

---

## 3. Icon / Emoji 使用规则

### 3.1 emoji

**emoji 只允许出现在以下两个位置**：

1. **chip / badge 的左缘**，且最多 1 个：`📸 Snapshot` `🚚 Exported` `🌐 Imported`
2. **toast / 一次性提示**，作为状态符号：`✅ Backup created` `⚠️ Profile not verified`

**严禁**用 emoji 的位置：

- ❌ H1 / H2 / H3 标题前面
- ❌ 分组 section 头（如本次发现的 "Workloads 📦 / Configuration ⚙️"）
- ❌ 表格列标题
- ❌ 表格行内多个 emoji 排队
- ❌ 按钮文字

### 3.2 SVG 图标

需要图标的场景用 **Element Plus 内置 SVG**：

- 默认 14px、`--sk-text-muted` 灰色
- hover 时随父元素变色（继承）
- 永远不带额外色彩（不要红 ⭕、绿 ✅、蓝 ⓘ）

### 3.3 chip / badge 模板

```html
<span class="sk-chip sk-chip-type-snapshot">📸 Snapshot</span>
<span class="sk-chip sk-chip-status-completed">Completed</span>
<span class="sk-chip sk-chip-role-snapshot">📸 Snapshot half</span>
```

CSS：

```css
.sk-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  letter-spacing: 0.3px;
  line-height: 1.6;
}
```

---

## 4. 间距 / 布局

### 4.1 间距 token

| Token | px | 用途 |
|---|---|---|
| `--sk-space-xs` | 4 | chip 内部 gap、tag 间隙 |
| `--sk-space-sm` | 8 | 表格行内、字段标签和值之间 |
| `--sk-space-md` | 12 | 卡片内 padding、表格 cell padding |
| `--sk-space-lg` | 16 | section 内部 |
| `--sk-space-xl` | 24 | **section 之间**（默认垂直节奏） |
| `--sk-space-2xl` | 32 | 大模块之间（如抽屉的 header 与 body） |

### 4.2 抽屉

- **统一宽度 560px**（不再有 520 / 640 / 720 混杂）
- 内容左右内边距 **24px**
- header → body 之间 32px
- section 之间 24px
- 底部操作栏（如有）**固定贴底**，灰底分隔线

---

## 5. 抽屉模板（统一）

所有 details 抽屉（Restore Point / Action / Application / Policy）**遵循同一模板**：

```
┌─────────────────────────────────────────────┐
│ [×]                                         │  ← 关闭按钮右上 16x16
│                                             │
│ RESTORE POINT DETAILS                       │  ← H1 灰大写
│ test-app-backup-snapshot-export-azure-...   │  ← H2 大粗黑（长名 hover 看完整）
│                                             │
│ [COMPLETED]  [Backup]  [📸 Snapshot half]   │  ← chip 行（状态+类型+role）
│                                             │
│ ─────────────────────────────────────────── │
│                                             │
│ Paired with: tc-pol-007-20260523021056 ↗    │  ← 紫色 banner（仅 dual）
│                                             │
│ ─────────────────────────────────────────── │
│                                             │
│ PHASES                                      │  ← H3
│ ✓ Validating                                │
│ ✓ Snapshotting Application Components       │
│ ✓ Uploading to Storage Profile              │
│ ✓ Finalizing                                │
│                                             │
│ ─────────────────────────────────────────── │
│                                             │
│ DETAILS                                     │  ← H3
│ Type              Backup                    │
│ Status            Completed                 │
│ Protected Object  test-app ↗                │
│ Policy            tc-pol-007 ↗              │
│ Policy Run At     5/23/2026, 10:11 AM       │
│ Start             5/23/2026, 10:11:08 AM    │
│ Duration          1 min 4 sec               │
│                                             │
│ ─────────────────────────────────────────── │
│                                             │
│ ARTIFACTS                                   │  ← H3
│                                             │
│ 11 Application Items   ·   50 Velero total  │  ← stat 行细横排
│                                             │
│ Workloads                              5  ▾ │  ← 纯文字，无 emoji
│   Deployment   adminer        test-app      │
│   Pod          adminer-...    test-app      │
│   ...                                       │
│ Configuration                          1  ▸ │
│ Networking                             2  ▸ │
│ Storage                                2  ▸ │
│ RBAC                                   1  ▸ │
│                                             │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐  ← 固定操作栏
│ [View YAML]              [Restore] [Delete] │
└─────────────────────────────────────────────┘
```

### 5.1 H1 标题对应表

| 上下文 | H1 文案 |
|---|---|
| Restore Points 页 ⋮ View | `RESTORE POINT DETAILS` |
| Activity 页点 Action card | `ACTION DETAILS` |
| Applications 页 ⋮ Details | `APPLICATION DETAILS` |
| Policies 页 ⋮ View（未来） | `POLICY DETAILS` |

> ⚠️ 不允许同一抽屉组件在不同入口都显示同一个 H1。组件必须接受 `title` prop 或根据 `entityType` 自动切换。

### 5.2 底部操作栏

| 抽屉 | 主操作（蓝） | 次操作（默认） | 第三 |
|---|---|---|---|
| RP Details | Restore | Delete | View YAML |
| Action Details (Backup) | View YAML | — | — |
| Action Details (Restore) | View YAML | — | — |
| Application Details | Backup Now | Restore | Create Policy |
| Policy Details | Run Once | Edit | Delete |

操作栏**永远存在**，让用户知道"看完这个抽屉之后能干啥"。

---

## 6. 表格 / 列表规则

### 6.1 列原则

- **不超过 7 列**（含选择框和 ⋮ kebab）。多了拆抽屉。
- 列头用 H4 token（11px 大写灰）
- 数字列右对齐；文本列左对齐；状态 chip 列居中

### 6.2 单元格不抹色

- 普通文本字段**永远不抹色**（除非该字段本身是状态/类型/可点击）
- 可点击文本用 `--sk-primary` indigo
- 状态用 status token
- 类型用 type token
- 其它一律 `--sk-text` 黑

### 6.3 hover 行

- 整行浅灰 hover 底色 `--sk-bg-hover`
- 可点击单元格独立 hover（更深底色 `--sk-primary-bg-hover`）

### 6.4 RP 计数 / 类似数字单元格

```html
<a class="sk-rp-count sk-rp-count-link" v-if="count > 0">{{ count }}</a>
<span class="sk-rp-count sk-rp-count-zero" v-else>0</span>
```

```css
.sk-rp-count {
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  font-weight: 500;
}
.sk-rp-count-link {
  color: var(--sk-primary);
  cursor: pointer;
  padding: 3px 8px;
  border-radius: 4px;
  transition: background 120ms ease, color 120ms ease;
}
.sk-rp-count-link:hover {
  background: var(--sk-primary-bg-hover);
  color: var(--sk-primary-hover);
  text-decoration: underline;
}
.sk-rp-count-zero {
  color: var(--sk-text-placeholder);
  cursor: default;
}
```

---

## 7. 交互状态

### 7.1 可交互元素必须有 4 态

| 态 | 视觉表现 |
|---|---|
| Default | 正常 |
| Hover | 颜色加深 + 底色变浅 + cursor:pointer |
| Active（按下） | 底色更深一档 |
| Disabled | 灰色 + cursor:not-allowed + title 解释原因 |

### 7.2 链接

- 文字色 `--sk-primary`
- hover 加下划线 + 颜色加深
- 已访问 (visited) 不区分（防止管理员"哪个我没点过"产生误判）

### 7.3 按钮

- 主按钮：蓝底白字
- 次按钮：透明底蓝字蓝边
- 危险按钮：红边红字（确认对话框里才会变红底）

---

## 8. 加载 / 空 / 错误状态

每个数据展示组件**必须实现 4 个状态**：

| 状态 | 长什么样 |
|---|---|
| Loading | spinner + "Loading..." 灰字 |
| Empty | 灰色斜体一句话解释 "No restore points yet" |
| Error | 红色感叹号 + 错误消息 + Retry 按钮 |
| Data | 实际内容 |

**严禁**：loading 时显示 0 / empty 时无任何反馈 / error 时静默。

---

## 9. 需要修复的现状（audit）

按规则逐页打分。每个 ❌ 必须有对应 task。

| 页面 / 组件 | H1+H2 | Emoji 节制 | 颜色 token | 底部操作栏 | 综合 |
|---|---|---|---|---|---|
| Action Details Drawer | ❌ 标题=Action Details 无 H2 | ❌ 5 个分组 emoji | ❌ Kind 涂紫 | ❌ 缺 | **重写** |
| Application Details Drawer | ✅ 有 H2 (default) | ⚠️ 📦 在 H2 旁边可以保留？ | ✅ 基本对 | ✅ 有 | **小修** |
| Restore Drawer (v0.7.10) | ❓ 待审 | — | — | ✅ 有 | 待审 |
| Restore Points 表格 | — | ⚠️ 📸/🚚/🌐 chip OK，📋 manual chip OK | ✅ | n/a | OK |
| Activity Action card | ✅ | ✅ | ✅ | n/a | OK |
| Policies 表格 | — | ⚠️ 📁 RP icon 应改 SVG | ✅ | n/a | 小修 |
| Applications 表格 | — | ✅ | ✅ | n/a | OK |

### 优先修复顺序

1. **Action Details Drawer 重构**（本次截图问题最严重）
   - 加 H1+H2
   - 删 5 个分组 emoji
   - Kind 列不再涂紫
   - 加底部操作栏
2. **Application Details Drawer 小调**
   - H1 标准化为 `APPLICATION DETAILS`
   - 决定 📦 emoji 是否保留（最多一个）
3. **Policies 表格 📁 改 SVG**
4. **全局 tokens.css 落地**：所有现存色值替换为 var()

---

## 10. 引用与执行

- 新 PR 模板里加 checkbox：「□ 已对照 UI_GUIDELINES 检查」
- 重大视觉改动需要在 PR 描述里贴**对比截图**
- 这份文档与 `架构设计.md`、`USER_MANUAL.md`、`测试用例.md` 同等地位 —— 改文档先于改代码

---

*v1 — 2026-05-23 起立项执行；如有规则缺漏，提 issue 修订。规则修订需更新版本号与变更日志（待加 §11）。*
