<template>
  <div class="chart-card">
    <div class="chart-header">
      <span class="chart-title">Backup Success Trend</span>
      <el-radio-group v-model="windowDays" size="small" class="chart-range">
        <el-radio-button :value="7">7d</el-radio-button>
        <el-radio-button :value="14">14d</el-radio-button>
        <el-radio-button :value="30">30d</el-radio-button>
      </el-radio-group>
    </div>
    <v-chart v-if="hasData" class="chart-body" :option="option" autoresize />
    <div v-else class="chart-empty">
      No backup activity in the selected window.
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import VChart from 'vue-echarts'
import '../utils/echarts'
import { useTheme } from '../composables/useTheme'

const { chartColors } = useTheme()

const props = defineProps({
  backups: { type: Array, default: () => [] }
})

const windowDays = ref(7)

// Bucket backups into daily counts of {completed, failed}. We anchor the
// window to "today end-of-day" so the rightmost point is always the most
// recent full day's data (and the user sees a clear "today" anchor).
const buckets = computed(() => {
  const days = windowDays.value
  const now = new Date()
  now.setHours(23, 59, 59, 999)
  const oldest = new Date(now)
  oldest.setDate(oldest.getDate() - (days - 1))
  oldest.setHours(0, 0, 0, 0)

  const labels = []
  const completed = []
  const failed = []
  for (let i = 0; i < days; i++) {
    const day = new Date(oldest)
    day.setDate(oldest.getDate() + i)
    labels.push(`${day.getMonth() + 1}/${day.getDate()}`)
    completed.push(0)
    failed.push(0)
  }

  for (const b of props.backups) {
    const ts = b?.status?.completionTimestamp || b?.metadata?.creationTimestamp
    if (!ts) continue
    const t = new Date(ts)
    if (t < oldest || t > now) continue
    const idx = Math.floor((t.getTime() - oldest.getTime()) / 86400000)
    if (idx < 0 || idx >= days) continue
    const phase = b?.status?.phase
    if (phase === 'Completed') completed[idx]++
    else if (phase === 'Failed' || phase === 'PartiallyFailed' || phase === 'FailedValidation') failed[idx]++
  }
  return { labels, completed, failed }
})

const hasData = computed(() => {
  const { completed, failed } = buckets.value
  return completed.some(v => v > 0) || failed.some(v => v > 0)
})

const option = computed(() => {
  const c = chartColors.value
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: {
      data: ['Completed', 'Failed'],
      right: 0,
      top: 0,
      textStyle: { color: c.textSecondary, fontSize: 12 }
    },
    grid: { left: 36, right: 8, top: 32, bottom: 24 },
    xAxis: {
      type: 'category',
      data: buckets.value.labels,
      axisLine: { lineStyle: { color: c.axisLine } },
      axisLabel: { color: c.textSecondary, fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: c.axisLine, type: 'dashed' } },
      axisLabel: { color: c.textSecondary, fontSize: 11 }
    },
    series: [
      {
        name: 'Completed',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { width: 2, color: c.success },
        itemStyle: { color: c.success },
        areaStyle: { color: 'rgba(103, 194, 58, 0.18)' },
        data: buckets.value.completed
      },
      {
        name: 'Failed',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { width: 2, color: c.danger },
        itemStyle: { color: c.danger },
        areaStyle: { color: 'rgba(245, 108, 108, 0.18)' },
        data: buckets.value.failed
      }
    ]
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
.chart-range :deep(.el-radio-button__inner) {
  padding: 4px 10px;
  font-size: 12px;
}
.chart-body {
  height: 220px;
  width: 100%;
}
.chart-empty {
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
