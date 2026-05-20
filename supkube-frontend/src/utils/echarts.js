// Centralized ECharts module imports — keep the bundle slim by only pulling in
// the chart types we actually use. Add to this file when a new chart kind is
// introduced. vue-echarts re-exports the `<v-chart>` component; we pass it the
// ready-to-render option object.

import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart, BarChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DatasetComponent
} from 'echarts/components'

use([
  CanvasRenderer,
  LineChart,
  PieChart,
  BarChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DatasetComponent
])

// Shared color palette — matches SupKube's Kasten-like brand: blue primary,
// green success, orange warning, red danger. Used directly by chart configs.
export const COLORS = {
  primary: '#409eff',
  success: '#67c23a',
  warning: '#e6a23c',
  danger: '#f56c6c',
  info: '#909399',
  muted: '#c0c4cc',
  axisLine: '#ebeef5',
  textPrimary: '#303133',
  textSecondary: '#909399'
}
