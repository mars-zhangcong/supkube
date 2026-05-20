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
  messages: { en, 'zh-CN': zhCN },
  missingWarn: false,
  fallbackWarn: false
})

export function setLocale(code) {
  if (!['en', 'zh-CN'].includes(code)) return
  i18n.global.locale.value = code
  try { localStorage.setItem(STORAGE_KEY, code) } catch (_) { /* ignore */ }
  document.documentElement.lang = code
}
