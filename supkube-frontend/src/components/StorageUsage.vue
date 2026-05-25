<template>
  <div class="chart-card">
    <div class="chart-header">
      <span class="chart-title">Restore Points per Storage Profile</span>
      <span class="chart-subtitle">{{ totalCount }} total</span>
    </div>
    <v-chart v-if="totalCount > 0" class="chart-body" :option="option" autoresize />
    <div v-else class="chart-empty">No backups yet.</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import VChart from 'vue-echarts'
import '../utils/echarts'
import { useTheme } from '../composables/useTheme'

const { chartColors, isDark } = useTheme()

const props = defineProps({
  backups: { type: Array, default: () => [] }
})

const PROFILE_PALETTE = computed(() => {
  const c = chartColors.value
  return [c.primary, c.success, c.warning, c.danger,
          '#73c0de', '#fac858', '#ee6666', '#9a60b4', '#3ba272', '#5470c6']
})

const groups = computed(() => {
  const counts = new Map()
  for (const b of props.backups) {
    const profile = b?.spec?.storageLocation || 'default'
    counts.set(profile, (counts.get(profile) || 0) + 1)
  }
  return Array.from(counts.entries())
    .map(([name, value]) => ({ name, value }))
    .sort((a, b) => b.value - a.value)
})

const totalCount = computed(() => groups.value.reduce((acc, g) => acc + g.value, 0))

const option = computed(() => {
  const c = chartColors.value
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: {
      bottom: 0,
      left: 'center',
      icon: 'circle',
      textStyle: { color: c.textSecondary, fontSize: 12 }
    },
    color: PROFILE_PALETTE.value,
    series: [{
      type: 'pie',
      radius: ['44%', '64%'],
      center: ['50%', '44%'],
      avoidLabelOverlap: true,
      label: {
        show: true,
        formatter: '{b}\n{c}',
        fontSize: 12,
        color: c.textPrimary,
        lineHeight: 16
      },
      labelLine: { length: 10, length2: 8 },
      itemStyle: {
        borderColor: isDark.value ? '#25282b' : '#ffffff',
        borderWidth: 2
      },
      data: groups.value
    }]
  }
})
</script>

<style scoped>
.chart-card {
  background: #ffffff;
  border-radius: 8px;
  padding: 16px 20px 8px;
  border: 1px solid var(--sk-border);
}
.chart-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.chart-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--sk-text-secondary);
}
.chart-subtitle {
  font-size: 12px;
  color: var(--sk-text-caption);
}
.chart-body { height: 260px; width: 100%; }
.chart-empty {
  height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--sk-text-placeholder);
  font-size: 13px;
}
</style>
