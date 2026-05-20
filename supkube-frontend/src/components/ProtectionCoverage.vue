<template>
  <div class="chart-card">
    <div class="chart-header">
      <span class="chart-title">Application Protection Coverage</span>
      <span class="chart-subtitle">{{ totalApps }} app{{ totalApps === 1 ? '' : 's' }}</span>
    </div>
    <v-chart v-if="totalApps > 0" class="chart-body" :option="option" autoresize />
    <div v-else class="chart-empty">No applications detected.</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import VChart from 'vue-echarts'
import '../utils/echarts'
import { COLORS } from '../utils/echarts'

const props = defineProps({
  applications: { type: Array, default: () => [] }
})

// Map ComplianceStatus (set by backend in v0.5.1) to a bar chart category.
// 5 buckets matching the badge palette on the Applications page.
const stateMap = computed(() => {
  const counts = { Compliant: 0, Unmanaged: 0, NonCompliant: 0, InProgress: 0, Empty: 0 }
  for (const a of props.applications) {
    const s = a?.complianceStatus || (a?.protected ? 'Compliant' : 'Unmanaged')
    if (s in counts) counts[s]++
    else counts.Unmanaged++
  }
  return counts
})

const totalApps = computed(() =>
  Object.values(stateMap.value).reduce((acc, v) => acc + v, 0)
)

const option = computed(() => {
  const m = stateMap.value
  const categories = [
    { name: 'Compliant', value: m.Compliant, color: COLORS.success },
    { name: 'Non-Compliant', value: m.NonCompliant, color: COLORS.danger },
    { name: 'In Progress', value: m.InProgress, color: COLORS.primary },
    { name: 'Unmanaged', value: m.Unmanaged, color: COLORS.warning },
    { name: 'Empty', value: m.Empty, color: COLORS.muted }
  ]
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 100, right: 20, top: 10, bottom: 20 },
    xAxis: {
      type: 'value',
      minInterval: 1,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: COLORS.axisLine, type: 'dashed' } },
      axisLabel: { color: COLORS.textSecondary, fontSize: 11 }
    },
    yAxis: {
      type: 'category',
      data: categories.map(c => c.name),
      axisLine: { lineStyle: { color: COLORS.axisLine } },
      axisLabel: { color: COLORS.textPrimary, fontSize: 12 }
    },
    series: [{
      type: 'bar',
      barWidth: 16,
      data: categories.map(c => ({ value: c.value, itemStyle: { color: c.color, borderRadius: [0, 4, 4, 0] } })),
      label: { show: true, position: 'right', color: COLORS.textPrimary, fontSize: 12 }
    }]
  }
})
</script>

<style scoped>
.chart-card {
  background: #ffffff;
  border-radius: 8px;
  padding: 16px 20px 8px;
  border: 1px solid #ebeef5;
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
  color: #303133;
}
.chart-subtitle {
  font-size: 12px;
  color: #909399;
}
.chart-body { height: 220px; width: 100%; }
.chart-empty {
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
