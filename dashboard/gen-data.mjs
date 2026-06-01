#!/usr/bin/env node
/* ====================================================================
   SupKube Dashboard — 半自动数据同步器
   --------------------------------------------------------------------
   单一来源纪律（ENGINEERING.md §2）：dashboard 的 PRDS / ADRS 两段最易
   与源 MD 漂移（历史教训：header 写 v0.9.1.5、KPI 却是 v0.9.1.9；PRDS
   停在 PRD-007、ADRS 停在 ADR-036）。本脚本把这两段的「易漂移字段」
   （状态 state / 关联任务 task / 日期）从权威源重新派生：

     · PRD 状态   ← PRD.md 顶部索引表
     · ADR 台账   ← 架构设计.md ADR-LEDGER

   「人工策展字段」（短标题 title、note 备注）保留 data.js 里已有的值，
   只为新增条目从源补一个初始标题。

   用法：
     node dashboard/gen-data.mjs            # 只报漂移，不改文件（CI 友好）
     node dashboard/gen-data.mjs --write    # 把 PRDS/ADRS 两段写回 data.js

   退出码：0 = 无漂移；1 = 有漂移（适合接进 GitHub Actions 当看门人）。
==================================================================== */

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dir = dirname(fileURLToPath(import.meta.url));
const repo = resolve(__dir, "..");
const P = (f) => resolve(repo, f);

const args = new Set(process.argv.slice(2));
const WRITE = args.has("--write");

/* ---------- 工具 ---------- */
const stripBold = (s) => s.replace(/\*\*/g, "").trim();
const firstDate = (s) => (s.match(/(\d{4}-\d{2}-\d{2})/) || [])[1] || "";
// 把 PRD.md 索引里五花八门的状态文案归一到 dashboard 认得的几个桶
// （视图按 state 字符串上色：草稿/排队评审/评审中/改正中/驳回/已评审/研发中/待验收/Shipped/归档）。
const normState = (raw) => {
  const s = stripBold(raw);
  if (/驳回/.test(s)) return "驳回";
  if (/改正中/.test(s)) return "改正中";
  if (/(研发中|进行中|Phase)/i.test(s)) return "研发中";
  if (/待验收/.test(s)) return "待验收";
  if (/(Shipped|已发布|已上线)/i.test(s)) return "Shipped";
  if (/归档/.test(s)) return "归档";
  if (/已评审/.test(s)) return "已评审"; // 吃掉 "已评审 v1.3" 之类后缀
  if (/评审中/.test(s)) return "评审中";
  if (/排队评审/.test(s)) return "排队评审";
  if (/草稿/.test(s)) return "草稿";
  return s.split(/[（(]/)[0].trim() || s; // 兜底：取「（日期）」前的词
};
const normAdrStatus = (raw) => (/(草稿|draft)/i.test(raw) ? "草稿" : "已评审");
const splitRow = (line) =>
  line
    .replace(/^\s*\|/, "")
    .replace(/\|\s*$/, "")
    .split("|")
    .map((c) => c.trim());

/* ---------- 1. 解析 PRD.md 索引表 ---------- */
function parsePRDs() {
  const md = readFileSync(P("PRD.md"), "utf8");
  const out = [];
  for (const line of md.split("\n")) {
    // | [PRD-001](#prd-001) | 标题 | **状态（日期）** | #104 |
    const m = line.match(/^\|\s*\[PRD-(\d+)\]\(#prd-\d+\)\s*\|/i);
    if (!m) continue;
    const cells = splitRow(line);
    if (cells.length < 4) continue;
    const id = "PRD-" + m[1];
    const title = stripBold(cells[1]);
    const stateCell = cells[2];
    const taskCell = stripBold(cells[3]);
    out.push({
      id,
      titleFromSrc: title,
      state: normState(stateCell),
      updated: firstDate(stateCell),
      task: (taskCell.match(/#[\w-]+/) || [taskCell])[0], // 取首个 #NNN
    });
  }
  return out;
}

/* ---------- 2. 解析 架构设计.md ADR-LEDGER ---------- */
function parseADRs() {
  const md = readFileSync(P("架构设计.md"), "utf8");
  const out = [];
  for (const line of md.split("\n")) {
    // | **ADR-033** | **标题**（…） | 🆕 草稿 (…, 2026-05-31) | PRD-003 |
    const m = line.match(/^\|\s*\*{0,2}ADR-(\d+)\*{0,2}\s*\|/);
    if (!m) continue; // 跳过 "ADR-001 ~ 031" 范围行（无单一编号）
    const cells = splitRow(line);
    if (cells.length < 4) continue;
    const id = "ADR-" + m[1];
    out.push({
      id,
      titleFromSrc: stripBold(cells[1]),
      status: normAdrStatus(cells[2]),
      date: firstDate(cells[2]),
      note: stripBold(cells[3]),
    });
  }
  return out;
}

/* ---------- 3. 读 data.js 里现有的 PRDS / ADRS（保留策展字段）---------- */
function readDataJs() {
  const src = readFileSync(P("dashboard/data.js"), "utf8");
  const grab = (name) => {
    const re = new RegExp(`const ${name} = \\[([\\s\\S]*?)\\n\\];`);
    const m = src.match(re);
    return m ? m[0] : null;
  };
  return { src, prdsBlock: grab("PRDS"), adrsBlock: grab("ADRS") };
}

// 从一个数组源码块里抽出有序的 [{id, title, note, ...}]，用于保留人工策展。
// 保序很重要：ADR-LEDGER 把 ADR-001~031 折叠成一行范围，data.js 里却单列了
// ADR-030/031 —— 这些「源里没有但 data.js 有」的条目必须保留，不能被覆写删掉。
function curatedFields(block) {
  const list = [];
  const map = {};
  if (!block) return { list, map };
  for (const line of block.split("\n")) {
    const idm = line.match(/id:\s*"([^"]+)"/);
    if (!idm) continue;
    const rec = {
      id: idm[1],
      title: (line.match(/title:\s*"((?:[^"\\]|\\.)*)"/) || [])[1],
      note: (line.match(/note:\s*"((?:[^"\\]|\\.)*)"/) || [])[1],
      state: (line.match(/state:\s*"([^"]*)"/) || [])[1],
      status: (line.match(/status:\s*"([^"]*)"/) || [])[1],
      task: (line.match(/task:\s*"([^"]*)"/) || [])[1],
    };
    list.push(rec);
    map[rec.id] = rec;
  }
  return { list, map };
}

/* ---------- 4. 合并 + 渲染 ---------- */
const esc = (s) => String(s ?? "").replace(/\\/g, "\\\\").replace(/"/g, '\\"');

// 合并顺序：先按 data.js 既有顺序（保留折叠范围里的策展条目，如 ADR-030/031），
// 再把「源里有、data.js 没有」的新条目追加到末尾。非破坏性：从不删除策展条目。
function mergedOrder(curatedList, srcMap) {
  const ids = curatedList.map((c) => c.id);
  for (const id of Object.keys(srcMap)) if (!ids.includes(id)) ids.push(id);
  return ids;
}

function renderPRDS(srcMap, curated) {
  const lines = mergedOrder(curated.list, srcMap).map((id) => {
    const c = curated.map[id] || {};
    const r = srcMap[id]; // 源里有 → 用源刷新易漂移字段；没有 → 原样保留
    const title = c.title || (r ? r.titleFromSrc : "");
    const note = c.note || "";
    const task = r ? r.task : c.task || "";
    const state = r ? r.state : c.state || "";
    const updated = r ? r.updated : "";
    return `  { id: "${esc(id)}", title: "${esc(title)}", task: "${esc(
      task
    )}", state: "${esc(state)}", updated: "${esc(updated)}", note: "${esc(
      note
    )}" },`;
  });
  return "const PRDS = [\n" + lines.join("\n") + "\n];";
}

function renderADRS(srcMap, curated) {
  const lines = mergedOrder(curated.list, srcMap).map((id) => {
    const c = curated.map[id] || {};
    const r = srcMap[id];
    const title = c.title || (r ? r.titleFromSrc : "");
    const note = c.note || (r ? r.note : "") || "";
    const date = r ? r.date : "";
    const status = r ? r.status : c.status || "";
    return `  { id: "${esc(id)}", title: "${esc(title)}", date: "${esc(
      date
    )}", status: "${esc(status)}", note: "${esc(note)}" },`;
  });
  return "const ADRS = [\n" + lines.join("\n") + "\n];";
}

/* ---------- 5. 漂移检测 ----------
   drifts = 真漂移（应修，决定退出码）；infos = 良性提示（折叠范围里的策展条目）。*/
function diff(kind, srcRows, curated, drifts, infos) {
  for (const r of srcRows) {
    if (!curated.map[r.id]) {
      drifts.push(`  [新增] ${kind} ${r.id}「${r.titleFromSrc}」源里有、data.js 里没有`);
    }
  }
  const srcIds = new Set(srcRows.map((r) => r.id));
  for (const rec of curated.list) {
    if (!srcIds.has(rec.id)) {
      infos.push(`  [仅 data.js] ${kind} ${rec.id} 源里没有（折叠范围里的策展条目，--write 会保留）`);
    }
  }
}

/* ---------- main ---------- */
const prdSrc = parsePRDs();
const adrSrc = parseADRs();
const { src, prdsBlock, adrsBlock } = readDataJs();
const prdCur = curatedFields(prdsBlock);
const adrCur = curatedFields(adrsBlock);
const prdSrcMap = Object.fromEntries(prdSrc.map((r) => [r.id, r]));
const adrSrcMap = Object.fromEntries(adrSrc.map((r) => [r.id, r]));

const drifts = [];
const infos = [];
diff("PRD", prdSrc, prdCur, drifts, infos);
diff("ADR", adrSrc, adrCur, drifts, infos);

// 也检测「状态/task 字段值」级别的漂移（既有条目但值变了）
function fieldDrift(kind, srcRows, block) {
  if (!block) return;
  for (const r of srcRows) {
    const re = new RegExp(`id:\\s*"${r.id}"[^\\n]*`);
    const line = (block.match(re) || [])[0];
    if (!line) continue;
    const cur = {
      state: (line.match(/state:\s*"([^"]*)"/) || [])[1],
      status: (line.match(/status:\s*"([^"]*)"/) || [])[1],
      task: (line.match(/task:\s*"([^"]*)"/) || [])[1],
    };
    if (kind === "PRD") {
      if (cur.state !== undefined && cur.state !== r.state)
        drifts.push(`  [状态] PRD ${r.id}: data.js="${cur.state}" → 源="${r.state}"`);
      if (cur.task !== undefined && cur.task !== r.task)
        drifts.push(`  [任务] PRD ${r.id}: data.js="${cur.task}" → 源="${r.task}"`);
    } else {
      if (cur.status !== undefined && cur.status !== r.status)
        drifts.push(`  [状态] ADR ${r.id}: data.js="${cur.status}" → 源="${r.status}"`);
    }
  }
}
fieldDrift("PRD", prdSrc, prdsBlock);
fieldDrift("ADR", adrSrc, adrsBlock);

console.log(
  `SupKube gen-data — 源: PRD.md(${prdSrc.length} 条) · ADR-LEDGER(${adrSrc.length} 条)`
);

if (drifts.length === 0) {
  console.log("✅ 无漂移：data.js 的 PRDS/ADRS 与源 MD 一致。");
} else {
  console.log(`⚠ 检出 ${drifts.length} 处真漂移（需 --write 或人工修）：`);
  console.log(drifts.join("\n"));
}
if (infos.length) {
  console.log(`ℹ ${infos.length} 处良性提示（不计入退出码）：`);
  console.log(infos.join("\n"));
}

if (WRITE) {
  if (!prdsBlock || !adrsBlock) {
    console.error("✗ 无法在 data.js 中定位 PRDS/ADRS 块，未写入。");
    process.exit(2);
  }
  let next = src
    .replace(prdsBlock, renderPRDS(prdSrcMap, prdCur))
    .replace(adrsBlock, renderADRS(adrSrcMap, adrCur));
  writeFileSync(P("dashboard/data.js"), next, "utf8");
  console.log("✍ 已把 PRDS/ADRS 两段写回 dashboard/data.js（策展 title/note 已保留）。");
  process.exit(0);
}

process.exit(drifts.length === 0 ? 0 : 1);
