# MERGE-PLAYBOOK — 待合分支并入 main 的顺序 + 冲突解法

> **状态**：📋 剧本（plan-only）。**本文件不执行 merge**。merge 是单独的"合并窗口"操作，由 Mars 拍时机后执行。
> **作者**：SCM / 2026-06-04｜**依据**：`git diff --name-only origin/main...<branch>` 实测 + 文件级 identity 校验。
> **范围**：PR#10 `chore/dwait-ledger-single-source`（D-WAIT 单源根治 docs）+ `feat/prd-007-layer4-transport` + `feat/prd-010-dr-topology-svg-rebuild` + `feat/prd-011-ai-score-endpoint` + PR#8 `feat/dr-loop-phase1-validation`（FDE）。

---

## 0. 三条铁律（贯穿全程）

1. **任何 `等待决策.md` 冲突 → 一律取 INDEX（PR#10）侧**。R&D / FDE 分支对旧大文件的 append（Vitest / ADR-040 色号）**已 home 成 D-WAIT-006/007**，合并时**丢弃**其 `等待决策.md` 改动（`git checkout --ours` 见 §3）。
2. **前置门（进合并窗口前每个 PR 都要过）**：① 该 PR 的 **CI 绿**；② **战场静默**——FDE / SRE 的 live writer 已收尾（开工前物理校验 `mtime` / `.git/index.lock` / `lsof` 进程 cwd，不靠口头"清场确认"，见 ENGINEERING Rule I）。
3. **一次一个 PR，合完一个 re-verify 再合下一个**：每合一个跑 `node dashboard/gen-data.mjs`（必须 ✅ 无漂移）+ 后端 `go build ./...` + 前端 `npm run build`。不批量混合。

---

## 1. 冲突矩阵（哪些分支动了同一份共享文件）

| 共享文件 | PR#10 dwait | prd-007 | prd-010 | prd-011 | PR#8 dr-loop |
|---|:--:|:--:|:--:|:--:|:--:|
| `等待决策.md` | ✅ 重构→INDEX | ✅ append | ✅ append | — | ✅ append |
| `ENGINEERING.md` / `LEDGER.md` / `CHANGELOG.md` | ✅ | — | — | — | — |
| `MERGE-PLAYBOOK.md`（本文件） | ✅ | — | — | — | — |
| 前端 SVG 套件¹ | — | ✅ | ✅ | — | ✅ |
| `supkube-backend/cmd/server/server.go` | — | — | — | ✅ +5 行 | — |
| `internal/api/v1/ai_*.go` | — | — | — | ✅ | — |
| `internal/backupcopy/*.go` | — | ✅ | — | — | — |
| `engineer-testing/dr-loop-phase1/*` | — | — | — | — | ✅ |

¹ 前端 SVG 套件 = `DRTopology.vue` / `DRTopology.test.ts` / `svg-topology.css` / `locales/{en,zh-CN}.js` / `package.json` / `package-lock.json` / `vitest.config.js`。

**两个关键 identity 结论（决定冲突是真是假）**：
- **前端 SVG 套件在 prd-007 / prd-010 / PR#8 三分支间字节相同**（实测 diff 为空）。→ 谁先合，后两支的前端改动就成 **no-op，零冲突**。
- **`server.go` 只有 prd-011 动**（+5 行注册 AI 路由，加在 `importpolicy.RegisterRoutes(api)` 之后）。→ **无跨分支冲突**，只是纯追加 hunk。

---

## 2. 合并顺序（HOLD 解除后照做）

> 设计目标：**docs 单源先落地**（让 `等待决策.md` 在 main 上变成 INDEX），**前端权威分支次之**（让其余前端改动变 no-op），后端独立分支随后，FDE 制品收尾。

| 步 | 分支 / PR | 为什么这个位置 | 主要冲突 + 解法 |
|---|---|---|---|
| **1** | **PR#10 `chore/dwait-ledger-single-source`** | docs SSOT 必须先上 main——之后每支的 `等待决策.md` 冲突都用"取 main(INDEX) 侧"统一解 | 仅 docs（ENGINEERING/LEDGER/CHANGELOG/等待决策→INDEX/MERGE-PLAYBOOK）。与 origin/main 无重叠 → **干净 fast-forward / 普通 merge，无冲突** |
| **2** | **`feat/prd-010-dr-topology-svg-rebuild`** | 前端 SVG 的权威源；先上 main，步 3/5 的前端就 no-op | `等待决策.md` 冲突 → **取 INDEX 侧**（§3）。前端套件 = 新内容，落 main。`package.json` 引入 vitest devDep → 见 §5 D-WAIT-006 决策依赖 |
| **3** | **`feat/prd-007-layer4-transport`** | 后端 backupcopy 独立 + 前端已被步 2 落地 | `等待决策.md` → 取 INDEX 侧。前端套件 vs main **字节相同 → 无冲突**。`backupcopy/*.go` 全新文件 → 干净 |
| **4** | **`feat/prd-011-ai-score-endpoint`** | 后端 AI 独立，谁都不碰它的文件 | `server.go` +5 行纯追加（只此分支动）→ **几乎必然干净**；万一 main 的 server.go 在前几步被动过（本批次不会），重跑 `go build` 复核。`ai_*.go` 全新文件 |
| **5** | **PR#8 `feat/dr-loop-phase1-validation`（FDE）** | 制品多、含 engineer-testing 全量，放最后收尾 | `等待决策.md` → 取 INDEX 侧。前端套件 vs main 字节相同 → 无冲突。`engineer-testing/dr-loop-phase1/*` 全新 → 干净。**合并时给 `DECISIONS-FOR-MARS.md` 补 banner**（§4） |

> 步 3/4/5 之间**互不冲突**（backupcopy / ai / engineer-testing 三组文件零重叠），顺序可调；只有 **步 1 必须最先、步 2 必须在 3/5 之前**两条硬约束。

---

## 3. `等待决策.md` 冲突标准解法（步 2/3/5 都用这一招）

PR#10 把 `等待决策.md` 从单一大文件改成了瘦 INDEX，并新建了 `等待决策/D-WAIT-NNN-*.md`。R&D/FDE 分支仍是旧大文件（prd-007 = 502 行原版；prd-010/PR#8 = 555 行含 Vitest/色号 append）。合并时 git 会在 `等待决策.md` 报冲突：

```bash
# 在合并冲突态下，等待决策.md 一律保留 main(INDEX) 侧，丢弃分支侧：
git checkout --ours -- 等待决策.md
git add 等待决策.md
# 分支带进来的 等待决策/ 新目录（如有）本就是 PR#10 建的，按 main 侧；
# 分支对旧大文件 append 的 Vitest/色号 内容已 home 成 D-WAIT-006/007，无需再迁。
```

> ⚠️ `--ours` 在 `git merge` 语境 = 当前 main 侧（正确）。若用 rebase 把分支 rebase 到 main 上，`--ours`/`--theirs` 含义相反，届时取 INDEX 那一侧（= rebase 的 `--theirs`）。**认准"保 INDEX 结构、弃旧大文件"这个语义，别认死 flag**。

---

## 4. PR#8 的 engineer-testing banner（步 5 合并时补）

`engineer-testing/dr-loop-phase1/DECISIONS-FOR-MARS.md` 内部仍用 FDE 自占旧号 `D-WAIT-004/005/006/007`（SCM 按"不碰 engineer-testing/ 制品"边界**没改**）。canonical SSOT 已把它们 forward-only 重编为 **D-WAIT-008~011**（+ A 路线 K3S → D-WAIT-012）。合并 PR#8 时，在该文件**头部加一行映射 banner**（不改正文）：

```markdown
> **号映射（canonical SSOT = 等待决策/）**：本文件内 D-WAIT-004→008 / 005→009 / 006→010 / 007→011；
> 文末「A 路线 K3S」→ D-WAIT-012。详见根目录 等待决策.md INDEX。
```

---

## 5. 合并窗口的决策依赖（需 Mars 拍）

- **D-WAIT-006（Vitest devDep，仍 open）**：prd-007 / prd-010 / PR#8 三支都带 `package.json` 的 vitest + @vue/test-utils + jsdom devDep。**步 2 一合，Vitest 就进 main = 事实上选了 option A（保留）**。→ **Mars 须在合并窗口前拍 D-WAIT-006 的 A/B**：选 A 直接合；选 B（回退）则需先在 prd-010 上 `npm uninstall` + 删 vitest.config.js/test 再合。
- **D-WAIT-001（CD deploy-dev OIDC，closed=方案A 但未执行）**：合到 main 会触发 CD → deploy-dev。若 Azure 那条 federated credential 还没加，CD #N deploy-dev 仍会 6s 失败（不阻断合并，但会红一个 job）。合并窗口前确认 Azure 凭据已加。
- **HOLD 解除信号**：协调者确认 FDE/DR-loop session 真收尾（物理校验 mtime 不再动）。

---

## 6. 每步合并后的 re-verify（前置门 §0.3 的落地）

```bash
node dashboard/gen-data.mjs        # 必须 ✅ 无漂移（PRDS/ADRS 与源 MD 一致）
cd supkube-backend && go build ./... && go test ./...   # 后端步（prd-007/011 后必跑）
cd supkube-frontend && npm ci && npm run build && npm run test   # 前端步（prd-010 后必跑）
```

全绿才进下一步。任一红 → 停在当前步，不继续往下合。

---

## 附：本剧本不覆盖的

- 实际 `git merge` / PR squash 策略（fast-forward vs merge commit vs squash）由 Mars / 协调者定。
- 合并后删分支、版本 tag、CHANGELOG `[Unreleased]` → 版本号的收口，属发布流程（ENGINEERING §3/§4），不在本剧本。
