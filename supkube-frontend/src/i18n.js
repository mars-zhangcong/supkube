// SupKube i18n bootstrap. Single composition-API vue-i18n instance, locale
// resolution order on app boot:
//   1. localStorage 'supkube.locale' (user explicit choice persists)
//   2. navigator.language coarse match (zh-* → zh-CN, anything else → en)
//   3. fallback 'en'
//
// Switching at runtime is done by the language selector in App.vue; that
// updates the locale and persists.

import { createI18n } from 'vue-i18n'
import en from './locales/en.js'
import zhCN from './locales/zh-CN.js'

// 登记处去中心化（i18n）：除 legacy 单体 en.js/zh-CN.js 外，每个功能可把自己的文案
// 放进 ./locales/en/<feature>.js + ./locales/zh-CN/<feature>.js（export default 一个
// 命名空间子树）。import.meta.glob 在构建期把所有片段装配进来 —— 加功能只新增自己的
// 片段文件，永不编辑共享单体，故并行 WI 在 locale 上文本永不冲突。片段 deep-merge 到
// 单体之上（叶子后者胜）。cross-file-lint 已同样合并后校验 parity。
const enFragments = import.meta.glob('./locales/en/*.js', { eager: true, import: 'default' })
const zhFragments = import.meta.glob('./locales/zh-CN/*.js', { eager: true, import: 'default' })
function deepMerge(a, b) {
  for (const k of Object.keys(b)) {
    if (b[k] && typeof b[k] === 'object' && !Array.isArray(b[k]))
      a[k] = deepMerge(a[k] && typeof a[k] === 'object' ? a[k] : {}, b[k])
    else a[k] = b[k]
  }
  return a
}
function assemble(base, frags) {
  const out = deepMerge({}, base)
  for (const mod of Object.values(frags)) deepMerge(out, mod)
  return out
}
const enMessages = assemble(en, enFragments)
const zhMessages = assemble(zhCN, zhFragments)

const STORAGE_KEY = 'supkube.locale'

function detectInitialLocale() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'en' || saved === 'zh-CN') return saved
  } catch (_) { /* private mode etc. */ }
  const nav = (navigator.language || 'en').toLowerCase()
  if (nav.startsWith('zh')) return 'zh-CN'
  return 'en'
}

export const SUPPORTED_LOCALES = [
  { code: 'en', label: 'English', short: 'EN' },
  { code: 'zh-CN', label: '简体中文', short: '中' }
]

export const i18n = createI18n({
  legacy: false, // composition API
  globalInjection: true,
  locale: detectInitialLocale(),
  fallbackLocale: 'en',
  messages: { en: enMessages, 'zh-CN': zhMessages },
  missingWarn: false,
  fallbackWarn: false
})

export function setLocale(code) {
  if (!['en', 'zh-CN'].includes(code)) return
  i18n.global.locale.value = code
  try { localStorage.setItem(STORAGE_KEY, code) } catch (_) { /* ignore */ }
  document.documentElement.lang = code
}
