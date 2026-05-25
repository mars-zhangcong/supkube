<!--
  ActionDurationChart (v0.8.0)
  ────────────────────────────
  Top-of-Activity bar chart. Each bar = one Action; X-axis is its start
  time, Y-axis is its duration in seconds. Bars are colored by status:
    running   → blue
    completed → grey
    failed/partial → red

  The chart reacts to dark mode through useTheme (the same composable
  Dashboard charts use, so the palette stays in sync).
-->
<template>
  <div class="duration-chart-wrap">
    <h4 class="chart-title">{{ t('activity.actionDurationsTitle') }}</h4>
    <v-chart class="chart" :option="option" autoresize />
    <div class="chart-legend">
      <span class="legend-item"><span class="dot dot-running"></span> {{ t('activity.legendRunning') }}</span>
      <span class="legend-item"><span class="dot dot-completed"></span> {{ t('activity.legendCompleted') }}</span>
      <span class="legend-item"><span class="dot dot-failed"></span> {{ t('activity.legendFailed') }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import VChart from 'vue-echarts'
import { useTheme } from '../composables/useTheme'

const { t } = useI18n()
const { isDark } = useTheme()

const props = defineProps({
  buckets: { type: Array, default: () => [] }
})

// Per-status colors. The CSS legend dots use the same hex.
const COLOR_RUNNING = '#409eff'
const COLOR_COMPLETED = '#3c4046'
const COLOR_FAILED = '#c45656'
const COLOR_COMPLETED_DARK = '#9ba3ad'

const colorForStatus = (status) => {
  if (status === 'running') return COLOR_RUNNING
  if (status === 'failed' || status === 'partial') return COLOR_FAILED
  return isDark.value ? COLOR_COMPLETED_DARK : COLOR_COMPLETED
}

const fmtTime = (iso) => {
  const d = new Date(iso)
  // "Today, 4:55pm" — match Kasten format. For multi-day we'd show date too;
  // the chart caps at 30 buckets so we're usually within a day anyway.
  const today = new Date()
  const isToday = d.toDateString() === today.toDateString()
  const time = d.toLocaleString(undefined, { hour: 'numeric', minute: '2-digit', hour12: true }).toLowerCase().replace(' ', '')
  return isToday ? `Today, ${time}` : d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true })
}

const option = computed(() => {
  const buckets = props.buckets || []
  return {
    grid: { top: 18, right: 16, bottom: 36, left: 56 },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params) => {
        const p = params[0]
        const b = buckets[p.dataIndex]
        if (!b) return ''
        return `<div style="font-weight:600">${b.type} — ${b.id}</div>
                <div>${fmtTime(b.time)}</div>
                <div>Duration: <strong>${b.durationSeconds}s</strong></div>
                <div>Status: <strong style="color:${colorForStatus(b.status)}">${b.status}</strong></div>`
      }
    },
    xAxis: {
      type: 'category',
      data: buckets.map(b => fmtTime(b.time)),
      axisLabel: {
        color: isDark.value ? '#b1b3b8' : '#606266',
        fontSize: 11,
        interval: Math.max(0, Math.floor(buckets.length / 5)) // show ~5 labels max
      },
      axisLine: { lineStyle: { color: isDark.value ? '#3a3d44' : '#dcdfe6' } }
    },
    yAxis: {
      type: 'value',
      name: 'Duration (sec)',
      nameLocation: 'middle',
      nameGap: 38,
      nameTextStyle: { color: isDark.value ? '#909399' : '#606266', fontSize: 11 },
      axisLabel: {
        color: isDark.value ? '#b1b3b8' : '#606266',
        fontSize: 11,
        formatter: (v) => v < 60 ? `${v} sec` : `${Math.round(v / 60)}m`
      },
      splitLine: { lineStyle: { color: isDark.value ? '#2c2f36' : '#ebeef5', type: 'dashed' } },
      axisLine: { show: false },
      axisTick: { show: false }
    },
    series: [{
      type: 'bar',
      barMaxWidth: 28,
      data: buckets.map(b => ({
        value: b.durationSeconds,
        itemStyle: { color: colorForStatus(b.status) }
      }))
    }]
  }
})
</script>

<style scoped>
.duration-chart-wrap {
  background: #ffffff;
  border: 1px solid var(--sk-border);
  border-radius: 8px;
  padding: 16px 20px 12px;
  margin-bottom: 16px;
}
.chart-title {
  margin: 0 0 8px 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--sk-text);
}
.chart {
  height: 200px;
  width: 100%;
}
.chart-legend {
  display: flex;
  gap: 22px;
  justify-content: center;
  margin-top: 4px;
  font-size: 12px;
  color: var(--sk-text-muted);
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.dot-running { background: #409eff; }
.dot-completed { background: #3c4046; }
.dot-failed { background: #c45656; }

:global(html.dark) .duration-chart-wrap { background: #1f2026; border-color: #2c2f36; }
:global(html.dark) .chart-title { color: #e5eaf3; }
:global(html.dark) .chart-legend { color: #b1b3b8; }
:global(html.dark) .dot-completed { background: #9ba3ad; }
</style>
