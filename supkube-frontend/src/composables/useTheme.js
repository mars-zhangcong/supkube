// Reactive theme detection — observes `<html class="dark">` set by main.js
// and the header toggle, lets components (especially ECharts ones whose
// option object is computed JS, not CSS) re-render on theme change.
//
// Usage:
//   const { isDark, chartColors } = useTheme()
//   const option = computed(() => makeOption(chartColors.value))

import { ref, computed, onMounted, onUnmounted } from 'vue'

// Module-level singleton so every component shares one observer + one ref.
const isDark = ref(typeof document !== 'undefined' && document.documentElement.classList.contains('dark'))

let observer = null
let refCount = 0

function ensureObserver() {
  if (observer || typeof document === 'undefined') return
  observer = new MutationObserver(() => {
    isDark.value = document.documentElement.classList.contains('dark')
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}

function releaseObserver() {
  if (refCount === 0 && observer) {
    observer.disconnect()
    observer = null
  }
}

// Chart color palette — different fg/grid for light vs dark mode. Series
// brand colors stay the same so the meaning stays consistent.
const LIGHT = {
  primary: '#409eff',
  success: '#67c23a',
  warning: '#e6a23c',
  danger: '#f56c6c',
  info: '#909399',
  muted: '#c0c4cc',
  axisLine: '#ebeef5',
  textPrimary: '#303133',
  textSecondary: '#909399',
  cardBg: '#ffffff',
  cardBorder: '#ebeef5'
}
const DARK = {
  primary: '#79bbff',
  success: '#85ce61',
  warning: '#eebe77',
  danger: '#f78989',
  info: '#a3a6ad',
  muted: '#606266',
  axisLine: '#3c4044',
  textPrimary: '#e5eaf3',
  textSecondary: '#a3a6ad',
  cardBg: '#25282b',
  cardBorder: '#3c4044'
}

export function useTheme() {
  refCount++
  onMounted(ensureObserver)
  onUnmounted(() => {
    refCount--
    releaseObserver()
  })
  const chartColors = computed(() => (isDark.value ? DARK : LIGHT))
  return { isDark, chartColors }
}
