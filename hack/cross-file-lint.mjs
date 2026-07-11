#!/usr/bin/env node
// cross-file-lint.mjs — 跨文件一致性闸（投产安全网）
// 拦截 LHF/任何 codegen 产出的「绿着却断」多文件 PR（见 PR #52：前端引 i18n key 后端没接/locale 悬空）。
//
// 硬闸（违反则 exit 1，挡合并）：
//   [I18N-MISSING] 前端 t('a.b.c') 引用的 key 必须在 en.js 和 zh-CN.js 都存在
//   [I18N-PARITY]  en.js 与 zh-CN.js 的 key 集合必须一致（一边有一边无 = 翻译漏）
// 软警（打印 WARN，不挡）：
//   [PARAM-WIRE]   前端 api 以 {params:{X}} 发出的 query 参数，后端应有 c.Query("X")
// 可选硬闸（--base <ref> 时启用，diff 模式）：
//   [GUTTING]      改动既有文件时出现与改动量不成比例的大删除（整文件重写征兆）
//
// 用法:
//   node hack/cross-file-lint.mjs                 # 全仓扫描（i18n + param）
//   node hack/cross-file-lint.mjs --base origin/main   # 额外做 gutting 检查（CI PR 模式）
//   CI 里：合并前作为必过 check；非 0 退出即拦。

import fs from 'node:fs'
import path from 'node:path'
import { execSync } from 'node:child_process'

const ROOT = process.cwd()
const FE = path.join(ROOT, 'supkube-frontend', 'src')
const BE = path.join(ROOT, 'supkube-backend')
const LOCALES = ['en.js', 'zh-CN.js'].map(f => path.join(FE, 'locales', f))
const baseIdx = process.argv.indexOf('--base')
const BASE = baseIdx >= 0 ? process.argv[baseIdx + 1] : null

const errors = []   // 硬闸
const warns = []    // 软警

// ---- helpers ----
function walk(dir, exts) {
  const out = []
  if (!fs.existsSync(dir)) return out
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) { if (e.name !== 'node_modules') out.push(...walk(p, exts)) }
    else if (exts.some(x => e.name.endsWith(x))) out.push(p)
  }
  return out
}
function rel(p) { return path.relative(ROOT, p) }

// ---- load a locale (export default {...}) into a JS object ----
function loadLocale(file) {
  const src = fs.readFileSync(file, 'utf8')
  const i = src.indexOf('export default')
  if (i < 0) throw new Error(`${rel(file)}: 没有 export default`)
  const body = src.slice(i).replace('export default', 'return ')
  try { return new Function(body)() }
  catch (e) { throw new Error(`${rel(file)}: 无法解析为对象 (${e.message})`) }
}
// 收集对象里所有路径(含中间节点和叶子)
function collectPaths(obj, prefix = '', set = new Set(), leaves = new Set()) {
  for (const k of Object.keys(obj)) {
    const p = prefix ? `${prefix}.${k}` : k
    set.add(p)
    const v = obj[k]
    if (v && typeof v === 'object' && !Array.isArray(v)) collectPaths(v, p, set, leaves)
    else leaves.add(p)
  }
  return { set, leaves }
}
// deep-merge b 进 a（叶子后者胜）——供"单体 + 片段目录"合并
function deepMerge(a, b) {
  for (const k of Object.keys(b)) {
    if (b[k] && typeof b[k] === 'object' && !Array.isArray(b[k]))
      a[k] = deepMerge(a[k] && typeof a[k] === 'object' ? a[k] : {}, b[k])
    else a[k] = b[k]
  }
  return a
}
// 登记处去中心化：locale = 单体 en.js/zh-CN.js  +  片段目录 locales/en|zh-CN/*.js。
// 片段每功能一份、装配见 src/i18n.js 的 import.meta.glob。此处同样合并后再校验，
// 否则迁到片段里的 key 会被误报 I18N-MISSING（正是「绿着却断」）。
function loadLocaleMerged(monolith, fragDir) {
  const merged = deepMerge({}, loadLocale(monolith))
  if (fs.existsSync(fragDir))
    for (const f of fs.readdirSync(fragDir))
      if (f.endsWith('.js')) deepMerge(merged, loadLocale(path.join(fragDir, f)))
  return merged
}

// ===== 1) i18n 检查 =====
let en, zh
try { en = collectPaths(loadLocaleMerged(LOCALES[0], path.join(FE, 'locales', 'en'))) } catch (e) { errors.push(['I18N-LOAD', e.message]) }
try { zh = collectPaths(loadLocaleMerged(LOCALES[1], path.join(FE, 'locales', 'zh-CN'))) } catch (e) { errors.push(['I18N-LOAD', e.message]) }

if (en && zh) {
  // 1a 引用的 key 必须两边都在
  // 词边界(避免 format('x')/at('x') 末尾的 t( 被误抓)；允许裸 t( 与 $t(
  const KEY_RE = /(?<![\w$])\$?t\(\s*['"`]([A-Za-z0-9_][A-Za-z0-9_.]*)['"`]/g
  let dynamic = 0
  const feFiles = walk(FE, ['.vue', '.js', '.ts'])
  const used = new Map() // key -> first file
  for (const f of feFiles) {
    const txt = fs.readFileSync(f, 'utf8')
    // 粗略数一下动态 key（无法静态校验，提示用）
    dynamic += (txt.match(/(?<![\w$])\$?t\(\s*`/g) || []).length
    let m
    while ((m = KEY_RE.exec(txt))) {
      const k = m[1]
      if (k.endsWith('.')) { dynamic++; continue }   // 形如 t('errors.'+code) 的动态前缀，跳过
      if (!k.includes('.')) continue                 // i18n key 均为带命名空间的点路径；无点的多为误抓
      // 是否带 `|| '兜底'`：t('k') || 'x' 会优雅降级，不算硬断（#52 的引用没有兜底）
      const guarded = /^\s*\)\s*\|\|/.test(txt.slice(KEY_RE.lastIndex, KEY_RE.lastIndex + 24))
      if (!used.has(k)) used.set(k, { f, guarded })
    }
  }
  for (const [key, { f, guarded }] of used) {
    const inEn = en.set.has(key), inZh = zh.set.has(key)
    if (!inEn || !inZh) {
      const miss = [!inEn && 'en.js', !inZh && 'zh-CN.js'].filter(Boolean).join(' + ')
      const bucket = guarded ? warns : errors           // 有 `|| 兜底` → 警告；无兜底(如 #52) → 硬断
      bucket.push([guarded ? 'I18N-MISSING-SOFT' : 'I18N-MISSING', `${rel(f)} 引用 t('${key}')${guarded ? '(有||兜底)' : ''}，但 ${miss} 里没有`])
    }
  }
  // 1b locale 对等（按叶子比）
  for (const k of en.leaves) if (!zh.leaves.has(k)) errors.push(['I18N-PARITY', `en.js 有 '${k}'，zh-CN.js 缺`])
  for (const k of zh.leaves) if (!en.leaves.has(k)) errors.push(['I18N-PARITY', `zh-CN.js 有 '${k}'，en.js 缺`])
  console.log(`· i18n: 扫描 ${feFiles.length} 前端文件，静态 key ${used.size} 个，动态 t(\`\`) ${dynamic} 处(跳过校验)`)
}

// ===== 2) 前端 query 参数 ↔ 后端 c.Query 接线（软警）=====
try {
  const beQuery = new Set()
  for (const f of walk(BE, ['.go'])) {
    const txt = fs.readFileSync(f, 'utf8')
    let m; const re = /c\.(?:Query|DefaultQuery)\(\s*"([A-Za-z0-9_]+)"/g
    while ((m = re.exec(txt))) beQuery.add(m[1])
  }
  const sent = new Map()
  for (const f of walk(FE, ['.js', '.ts', '.vue'])) {
    const txt = fs.readFileSync(f, 'utf8')
    // 抓 params: { a, b: ... } 里的顶层键（粗略）
    let m; const re = /params\s*:\s*\{([^{}]*)\}/g
    while ((m = re.exec(txt))) {
      for (const piece of m[1].split(',')) {
        const k = piece.trim().split(':')[0].trim().replace(/['"`]/g, '')
        if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(k) && k !== '' && k !== 'params') {
          if (!sent.has(k)) sent.set(k, f)
        }
      }
    }
  }
  const IGNORE = new Set(['_', 'page', 'pageSize', 'limit', 'offset']) // 常见纯前端/分页参数白名单可调
  for (const [k, f] of sent) {
    if (!beQuery.has(k) && !IGNORE.has(k)) warns.push(['PARAM-WIRE', `前端 ${rel(f)} 发 query '${k}'，但后端无 c.Query("${k}")（可能前端发了后端没接，#52 型）`])
  }
  console.log(`· param: 后端 c.Query 参数 ${beQuery.size} 个；前端发出 ${sent.size} 个`)
} catch (e) { warns.push(['PARAM-WIRE', '参数检查跳过: ' + e.message]) }

// ===== 3) 整文件重写/大删除（仅 --base 时）=====
if (BASE) {
  try {
    const out = execSync(`git diff --numstat ${BASE}...HEAD`, { cwd: ROOT }).toString()
    for (const line of out.trim().split('\n').filter(Boolean)) {
      const [add, del, file] = line.split('\t')
      const a = Number(add), d = Number(del)
      if (Number.isNaN(a)) continue // 二进制
      const exists = fs.existsSync(path.join(ROOT, file))
      if (exists && d >= 25 && d > 5 * (a + 1)) {
        errors.push(['GUTTING', `${file}: -${d}/+${a}，删除远大于新增（疑整文件重写删既有）`])
      }
    }
    console.log(`· gutting: 对比 ${BASE}...HEAD`)
  } catch (e) { warns.push(['GUTTING', 'diff 检查跳过: ' + e.message]) }
}

// ===== 输出 =====
console.log('')
for (const [tag, msg] of warns) console.log(`  WARN  [${tag}] ${msg}`)
for (const [tag, msg] of errors) console.log(`  FAIL  [${tag}] ${msg}`)
console.log('')
if (errors.length) {
  console.error(`✗ 跨文件一致性闸: ${errors.length} 条硬违反，${warns.length} 条警告 — 拦下合并`)
  process.exit(1)
}
console.log(`✓ 跨文件一致性闸通过（${warns.length} 条警告）`)
