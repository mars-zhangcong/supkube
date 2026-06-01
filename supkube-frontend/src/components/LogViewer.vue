<!--
  LogViewer.vue (task #79, v0.9.x — Quick Win pass 2026-05-31)

  v1 (initial ship): basic dropdowns + grep + auto 5s + download.
  Quick Win pass adds 7 operator-comfort features based on customer
  feedback "排查备份故障时看着最舒服" 要求:
    1. 顶部错误摘要卡（点 Jump 直达第一条 ERROR）
    2. ↑ Prev Error / ↓ Next Error 按钮（在 errorIndices 间跳转）
    3. 点 ERROR 行展开前后 ±5 行 Info 上下文（原地 expand）
    4. 相对/绝对时间一键切换
    5. Live Tail Lock — 鼠标上滚自动暂停 auto-refresh, 浮窗 Resume
    6. 搜索词亮黄底荧光笔（grep 匹配在 lv-msg 里高亮）
    7. 全屏模式 — 一键最大化 console

  Not yet (留 PRD-005 Log Viewer v2):
    - Virtual scroll（10 万行不卡）
    - 滚动条 minimap（匹配位置可视化）
    - Summary/Detail/Debug 三层日志深度
    - 错误代码 + KB jump
    - SSE 真 live tail（当前 5s 轮询）
    - Forwarding 到 ELK/Loki
    - 根因 AI 摘要（依赖 PRD-003 AI Advisor）
-->
<template>
  <div class="log-viewer" :class="{ 'lv-fullscreen': isFullscreen }">
    <!-- ─── Toolbar ──────────────────────────────────────────────── -->
    <div class="lv-toolbar">
      <div class="lv-filters">
        <el-select v-model="component" size="default" style="width: 200px" @change="onFilterChange">
          <el-option v-for="c in components" :key="c.key" :label="c.display" :value="c.key" />
        </el-select>

        <el-select v-model="severity" size="default" style="width: 130px" @change="onFilterChange">
          <el-option label="Any severity" value="ANY" />
          <el-option label="ERROR" value="ERROR" />
          <el-option label="WARN" value="WARN" />
          <el-option label="INFO" value="INFO" />
          <el-option label="DEBUG" value="DEBUG" />
        </el-select>

        <el-select v-model="sinceSeconds" size="default" style="width: 130px" @change="onFilterChange">
          <el-option label="Last 15 min"   :value="900" />
          <el-option label="Last 1 hour"   :value="3600" />
          <el-option label="Last 6 hours"  :value="21600" />
          <el-option label="Last 24 hours" :value="86400" />
        </el-select>

        <el-select v-model="tailLines" size="default" style="width: 110px" @change="onFilterChange">
          <el-option label="200 / pod"  :value="200" />
          <el-option label="500 / pod"  :value="500" />
          <el-option label="1000 / pod" :value="1000" />
          <el-option label="2000 / pod" :value="2000" />
        </el-select>

        <el-input
          v-model="grep"
          size="default"
          style="width: 220px"
          placeholder="grep (case-insensitive)"
          clearable
          @keyup.enter="onFilterChange"
          @clear="onFilterChange"
        />
      </div>

      <div class="lv-actions">
        <!-- Next/Prev error navigation. Disabled when there are no errors
             so the buttons aren't misleading. Counter shows "current /
             total" for orientation. -->
        <el-button-group>
          <el-button :icon="ArrowUp"   size="default" :disabled="errorIndices.length === 0" @click="prevError" :title="'Prev error (Shift+P)'" />
          <el-button :icon="ArrowDown" size="default" :disabled="errorIndices.length === 0" @click="nextError" :title="'Next error (Shift+N)'" />
        </el-button-group>
        <span v-if="errorIndices.length > 0" class="lv-err-counter">
          {{ currentErrorPos + 1 }} / {{ errorIndices.length }}
        </span>

        <el-button size="default" @click="toggleTimeFormat" :title="timeFormat === 'absolute' ? 'Switch to relative time' : 'Switch to absolute time'">
          {{ timeFormat === 'absolute' ? 'Abs' : 'Rel' }}
        </el-button>

        <el-checkbox v-model="autoRefresh" @change="onAutoRefreshToggle">Auto 5s</el-checkbox>
        <el-button :icon="Refresh" size="default" :loading="loading" @click="fetchLogs">Refresh</el-button>
        <el-button :icon="Download" size="default" @click="onDownloadClick">Download .txt</el-button>
        <el-button :icon="isFullscreen ? Aim : FullScreen" size="default" @click="toggleFullscreen" :title="isFullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'" />
      </div>
    </div>

    <!-- ─── Error summary card (only when there ARE errors) ──────── -->
    <!-- Customer feedback explicitly: "运维人员打开日志时通常处于焦虑状态,
         界面应该直接提取出导致失败的那一条关键日志". Top card does that. -->
    <div v-if="errorIndices.length > 0" class="lv-err-summary" @click="jumpToError(0)">
      <el-icon class="lv-err-summary-icon"><Warning /></el-icon>
      <div class="lv-err-summary-body">
        <div class="lv-err-summary-title">
          ⚠ Detected <strong>{{ errorIndices.length }}</strong> error{{ errorIndices.length > 1 ? 's' : '' }}
          in this window. First:
        </div>
        <div class="lv-err-summary-msg">{{ truncate(lines[errorIndices[0]]?.message, 220) }}</div>
      </div>
      <el-button size="small" type="danger" @click.stop="jumpToError(0)">Jump to first →</el-button>
    </div>

    <!-- ─── Status strip ────────────────────────────────────────── -->
    <div class="lv-status">
      <span v-if="podCount > 0" class="lv-meta">
        <strong>{{ lines.length }}</strong> lines from <strong>{{ podCount }}</strong> pod{{ podCount > 1 ? 's' : '' }}
        <span v-if="truncatedAt > 0" class="lv-truncated">· truncated at {{ truncatedAt }}</span>
      </span>
      <span v-else-if="warningNotice" class="lv-warning">⚠ {{ warningNotice }}</span>
      <span v-else-if="!loading && lines.length === 0" class="lv-meta">No matching lines.</span>
      <span v-if="generatedAt" class="lv-generated">Last fetch: {{ formatTime(generatedAt) }}</span>
    </div>

    <!-- ─── Log body ────────────────────────────────────────────── -->
    <div ref="bodyRef" class="lv-body" @scroll="onBodyScroll" @wheel="onWheel">
      <template v-for="(line, idx) in lines" :key="idx">
        <div
          :ref="el => setLineRef(idx, el)"
          class="lv-line"
          :class="[
            `lv-sev-${(line.severity || 'unknown').toLowerCase()}`,
            { 'lv-line-highlighted': highlightedIdx === idx }
          ]"
          @click="onLineClick(idx, line, $event)"
          :title="line.severity === 'ERROR' ? 'Click to expand ±5 lines of context' : 'Click to copy line'"
        >
          <span class="lv-ts">{{ renderTimestamp(line.timestamp) }}</span>
          <span class="lv-sev">{{ line.severity }}</span>
          <span class="lv-pod">{{ line.pod }}</span>
          <span class="lv-msg" v-html="renderMsg(line.message)"></span>
          <el-icon v-if="line.severity === 'ERROR'" class="lv-expand-icon">
            <ArrowDown v-if="!isExpanded(idx)" />
            <ArrowUp v-else />
          </el-icon>
        </div>
        <!-- Inline context: ±5 lines AROUND this ERROR, rendered as a
             gray nested block so it's visually clear they're context, not
             the error itself. -->
        <div v-if="isExpanded(idx)" class="lv-context">
          <div v-for="ctx in contextWindow(idx)" :key="ctx.absIdx"
               class="lv-line lv-line-ctx"
               :class="[`lv-sev-${(ctx.severity || 'unknown').toLowerCase()}`]">
            <span class="lv-ts">{{ renderTimestamp(ctx.timestamp) }}</span>
            <span class="lv-sev">{{ ctx.severity }}</span>
            <span class="lv-pod">{{ ctx.pod }}</span>
            <span class="lv-msg" v-html="renderMsg(ctx.message)"></span>
          </div>
        </div>
      </template>
      <div v-if="lines.length === 0 && !loading && !warningNotice" class="lv-empty">
        No log lines match the current filters.
      </div>
      <div v-if="loading && lines.length === 0" class="lv-empty">
        <el-icon class="rotating"><Loading /></el-icon> Loading logs...
      </div>
    </div>

    <!-- ─── Floating "Live tail paused" pill (only when locked) ──── -->
    <!-- Operator scrolled up mid-tail → don't yank new lines under their
         cursor. Show a small chip; click resumes scroll-to-bottom. -->
    <div v-if="autoRefresh && liveLocked" class="lv-resume-pill" @click="resumeLiveTail">
      <el-icon><CaretBottom /></el-icon>
      Live tail paused (you scrolled up). Click to resume.
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Refresh, Download, Loading, ArrowUp, ArrowDown,
  Warning, FullScreen, Aim, CaretBottom
} from '@element-plus/icons-vue'
import { getLogComponents, getLogs, downloadLogs } from '../api/velero'

// ─── Filter state ────────────────────────────────────────────
const components = ref([])
const component = ref('backend')
const severity = ref('ANY')
const sinceSeconds = ref(3600)
const tailLines = ref(500)
const grep = ref('')

// ─── Data state ──────────────────────────────────────────────
const lines = ref([])
const podCount = ref(0)
const truncatedAt = ref(0)
const warningNotice = ref('')
const generatedAt = ref('')
const loading = ref(false)

// ─── Quick-Win state ─────────────────────────────────────────
// Time format: 'absolute' shows HH:MM:SS.fff, 'relative' shows "3m ago"
const timeFormat = ref('absolute')
// Now ref updated each second so 'relative' values refresh smoothly
const now = ref(Date.now())
let nowTimer = null

// Fullscreen: CSS fixed inset:0 z-index:9999
const isFullscreen = ref(false)

// Live tail lock: user scrolled up while autoRefresh on → don't autoscroll
const liveLocked = ref(false)
// Track whether THIS fetch just happened from autoRefresh, vs user-initiated
let pendingAutoScroll = false

// Expanded ERROR rows (set of indices)
const expandedRows = ref(new Set())

// Currently flash-highlighted row (after Jump). Cleared after 2s.
const highlightedIdx = ref(-1)
let highlightClearTimer = null

// Next/Prev error cursor — index INTO errorIndices, not into lines
const currentErrorPos = ref(-1)

// Auto-refresh
const autoRefresh = ref(false)
let pollTimer = null

// DOM refs
const bodyRef = ref(null)
const lineRefs = new Map()
const setLineRef = (idx, el) => {
  if (el) lineRefs.set(idx, el); else lineRefs.delete(idx)
}

// ─── Computed ────────────────────────────────────────────────
const errorIndices = computed(() => {
  const out = []
  for (let i = 0; i < lines.value.length; i++) {
    if (lines.value[i].severity === 'ERROR') out.push(i)
  }
  return out
})

// ─── API calls ───────────────────────────────────────────────
const fetchComponents = async () => {
  try {
    const r = await getLogComponents()
    components.value = r.data?.components || []
  } catch (e) {
    components.value = [
      { key: 'backend',    display: 'SupKube Backend' },
      { key: 'frontend',   display: 'SupKube Frontend' },
      { key: 'velero',     display: 'Velero Server' },
      { key: 'node-agent', display: 'Velero node-agent' },
      { key: 'dex',        display: 'Dex (OIDC)' }
    ]
  }
}

const buildParams = () => ({
  component:    component.value,
  sinceSeconds: sinceSeconds.value,
  tailLines:    tailLines.value,
  grep:         grep.value || undefined,
  severity:     severity.value === 'ANY' ? undefined : severity.value
})

const fetchLogs = async () => {
  loading.value = true
  try {
    const r = await getLogs(buildParams())
    const data = r.data || {}
    lines.value         = data.lines || []
    podCount.value      = data.podCount || 0
    truncatedAt.value   = data.truncatedAt || 0
    warningNotice.value = data.warningNotice || ''
    generatedAt.value   = data.generatedAt || ''
    // Reset error cursor whenever the line set changes
    currentErrorPos.value = -1
    // Reset expanded rows (their indices would be stale after a re-fetch)
    expandedRows.value = new Set()
    // Auto-scroll to bottom only if user hasn't manually locked it
    if (autoRefresh.value && !liveLocked.value) {
      pendingAutoScroll = true
      nextTick(() => {
        if (bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight
        pendingAutoScroll = false
      })
    }
  } catch (e) {
    ElMessage.error('Fetch logs failed: ' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

// Any filter change resets cursor/expansion + refetches.
const onFilterChange = () => { fetchLogs() }

const onDownloadClick = async () => {
  try {
    const r = await downloadLogs(buildParams())
    const blob = new Blob([r.data], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const cd = r.headers?.['content-disposition'] || ''
    const m = /filename=([^;]+)/i.exec(cd)
    a.download = m ? m[1].trim() : `supkube-${component.value}-logs.txt`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 1500)
  } catch (e) {
    ElMessage.error('Download failed: ' + (e.response?.data?.error || e.message))
  }
}

const onAutoRefreshToggle = (v) => {
  if (v) {
    liveLocked.value = false
    pollTimer = setInterval(fetchLogs, 5000)
  } else if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// ─── Line interactions ───────────────────────────────────────
const onLineClick = (idx, line, evt) => {
  if (line.severity === 'ERROR') {
    // ERROR rows: clicking toggles ±5 context expansion. Use a flag so
    // a Shift-click can still copy when the user wants the raw text.
    if (evt.shiftKey) {
      copyLine(line); return
    }
    if (expandedRows.value.has(idx)) {
      expandedRows.value.delete(idx)
    } else {
      expandedRows.value.add(idx)
    }
    expandedRows.value = new Set(expandedRows.value)  // trigger reactivity
  } else {
    copyLine(line)
  }
}

const isExpanded = (idx) => expandedRows.value.has(idx)

// Build a window of lines around an ERROR idx, EXCLUDING the error
// itself (which is already rendered as the parent row).
const contextWindow = (idx) => {
  const out = []
  const lo = Math.max(0, idx - 5)
  const hi = Math.min(lines.value.length - 1, idx + 5)
  for (let i = lo; i <= hi; i++) {
    if (i === idx) continue
    out.push({ ...lines.value[i], absIdx: i })
  }
  return out
}

const copyLine = async (line) => {
  const text = `${line.timestamp} [${line.severity}] ${line.pod} ${line.message}`
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success({ message: 'Line copied', duration: 1200 })
  } catch {}
}

// ─── Next/Prev error navigation ──────────────────────────────
const nextError = () => {
  if (errorIndices.value.length === 0) return
  const pos = (currentErrorPos.value + 1) % errorIndices.value.length
  jumpToError(pos)
}
const prevError = () => {
  if (errorIndices.value.length === 0) return
  const pos = (currentErrorPos.value <= 0)
    ? errorIndices.value.length - 1
    : currentErrorPos.value - 1
  jumpToError(pos)
}
const jumpToError = (pos) => {
  currentErrorPos.value = pos
  const lineIdx = errorIndices.value[pos]
  const el = lineRefs.get(lineIdx)
  if (el && el.scrollIntoView) {
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
  // Flash-highlight the target row for 2 seconds so the eye finds it.
  highlightedIdx.value = lineIdx
  if (highlightClearTimer) clearTimeout(highlightClearTimer)
  highlightClearTimer = setTimeout(() => { highlightedIdx.value = -1 }, 2000)
}

// ─── Time format toggle ──────────────────────────────────────
const toggleTimeFormat = () => {
  timeFormat.value = timeFormat.value === 'absolute' ? 'relative' : 'absolute'
}

// ─── Fullscreen ──────────────────────────────────────────────
const toggleFullscreen = () => {
  isFullscreen.value = !isFullscreen.value
}
const onEscKey = (e) => {
  if (e.key === 'Escape' && isFullscreen.value) isFullscreen.value = false
}

// ─── Live tail lock ──────────────────────────────────────────
// If the user manually scrolls up while autoRefresh is on, pause
// auto-scroll-to-bottom so new lines don't shove their reading off
// screen. Resume button (or scrolling back to bottom) clears it.
const onBodyScroll = () => {
  if (!autoRefresh.value || !bodyRef.value) return
  if (pendingAutoScroll) return  // ignore the scrollTop we ourselves wrote
  const el = bodyRef.value
  const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  // 80px tolerance — small jitter shouldn't lock.
  if (distFromBottom > 80) {
    liveLocked.value = true
  } else if (distFromBottom < 8) {
    liveLocked.value = false
  }
}
const onWheel = (e) => {
  // Upward scroll while auto-refresh on → lock immediately even if the
  // scroll handler hasn't fired yet (improves perceived responsiveness).
  if (autoRefresh.value && e.deltaY < 0) liveLocked.value = true
}
const resumeLiveTail = () => {
  liveLocked.value = false
  if (bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight
}

// ─── Rendering helpers ───────────────────────────────────────
const renderTimestamp = (ts) => {
  if (!ts) return ''
  if (timeFormat.value === 'absolute') {
    const m = /T(\d{2}:\d{2}:\d{2}\.?\d*)/.exec(ts)
    return m ? m[1].slice(0, 12) : ts
  }
  // relative — touches `now` so it reactively updates with the tick
  const then = new Date(ts).getTime()
  const diff = Math.max(0, now.value - then)
  return relativeAgo(diff)
}
const relativeAgo = (ms) => {
  const s = Math.floor(ms / 1000)
  if (s < 60)    return s + 's ago'
  const m = Math.floor(s / 60)
  if (m < 60)    return m + 'm ago'
  const h = Math.floor(m / 60)
  if (h < 24)    return h + 'h ago'
  const d = Math.floor(h / 24)
  return d + 'd ago'
}
const formatTime = (iso) => {
  if (!iso) return ''
  return new Date(iso).toLocaleTimeString()
}

// HTML-escape the message and wrap grep matches in <mark> for the
// yellow-highlighter effect. Case-insensitive, all occurrences.
const escapeHTML = (s) => {
  if (s == null) return ''
  return String(s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}
const escapeRegex = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const renderMsg = (msg) => {
  const safe = escapeHTML(msg)
  const term = grep.value?.trim()
  if (!term) return safe
  const re = new RegExp(escapeRegex(term), 'gi')
  return safe.replace(re, m => `<mark class="lv-hl">${m}</mark>`)
}
const truncate = (s, n) => {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}

// ─── Lifecycle ───────────────────────────────────────────────
onMounted(async () => {
  await fetchComponents()
  await fetchLogs()
  // Tick `now` once a second for relative-time refresh.
  nowTimer = setInterval(() => { now.value = Date.now() }, 1000)
  window.addEventListener('keydown', onEscKey)
})
onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  if (nowTimer) clearInterval(nowTimer)
  if (highlightClearTimer) clearTimeout(highlightClearTimer)
  window.removeEventListener('keydown', onEscKey)
})
</script>

<style scoped>
.log-viewer {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 240px);
  min-height: 480px;
  position: relative;
}
/* Fullscreen: lift the whole viewer above all chrome. */
.lv-fullscreen {
  position: fixed;
  inset: 0;
  z-index: 9999;
  height: 100vh !important;
  min-height: 100vh !important;
  background: var(--sk-bg-secondary, #fafbfc);
  padding: 8px;
}

.lv-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  background: var(--sk-bg-secondary, #fafbfc);
  border: 1px solid var(--sk-border-light, #e7e9ec);
  border-radius: 6px 6px 0 0;
}
.lv-filters { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.lv-actions { display: flex; gap: 8px; align-items: center; }
.lv-err-counter {
  font-size: 12px; color: #ef4444; font-weight: 600; font-family: ui-monospace, Menlo, monospace;
  padding: 0 2px;
}

/* Error summary card — only renders when errorIndices.length > 0 */
.lv-err-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: rgba(248, 113, 113, 0.08);
  border-left: 4px solid #ef4444;
  border-right: 1px solid var(--sk-border-light, #e7e9ec);
  cursor: pointer;
  transition: background 0.15s ease;
}
.lv-err-summary:hover { background: rgba(248, 113, 113, 0.14); }
.lv-err-summary-icon { font-size: 20px; color: #ef4444; flex-shrink: 0; }
.lv-err-summary-body { flex: 1; min-width: 0; }
.lv-err-summary-title { font-size: 13px; color: #1f2937; margin-bottom: 2px; }
.lv-err-summary-msg {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  color: #7f1d1d;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.lv-status {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 16px;
  font-size: 12px;
  color: var(--sk-text-caption, #6b7280);
  background: var(--sk-bg-secondary, #fafbfc);
  border-left: 1px solid var(--sk-border-light, #e7e9ec);
  border-right: 1px solid var(--sk-border-light, #e7e9ec);
}
.lv-warning { color: #d97706; font-weight: 500; }
.lv-truncated { color: #d97706; }
.lv-generated { font-style: italic; }

.lv-body {
  flex: 1;
  overflow: auto;
  font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  background: #0f172a;
  color: #cbd5e1;
  border: 1px solid var(--sk-border-light, #e7e9ec);
  border-radius: 0 0 6px 6px;
  padding: 8px 12px;
}
.lv-line {
  display: flex;
  gap: 10px;
  padding: 1px 4px;
  cursor: pointer;
  border-radius: 2px;
  position: relative;
}
.lv-line:hover { background: #1e293b; }
/* Flash highlight when navigated to via Next/Prev Error */
.lv-line-highlighted {
  background: rgba(250, 204, 21, 0.18) !important;
  box-shadow: 0 0 0 1px rgba(250, 204, 21, 0.5);
  animation: lv-flash 2s ease;
}
@keyframes lv-flash {
  0%   { background: rgba(250, 204, 21, 0.4); }
  100% { background: rgba(250, 204, 21, 0.18); }
}
.lv-ts  { color: #64748b; flex-shrink: 0; width: 90px; }
.lv-sev { flex-shrink: 0; width: 60px; font-weight: 600; }
.lv-pod { color: #94a3b8; flex-shrink: 0; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.lv-msg { flex: 1; word-break: break-all; }
.lv-expand-icon {
  font-size: 12px; color: #94a3b8; flex-shrink: 0;
  align-self: center;
}

/* grep match yellow highlighter */
.lv-msg :deep(.lv-hl) {
  background: #facc15;
  color: #0f172a;
  padding: 0 1px;
  border-radius: 1px;
  font-weight: 600;
}

/* Context-expanded block — slightly dimmed + left rule so the eye sees
   "these are context lines around the error above". */
.lv-context {
  margin: 2px 0 6px 24px;
  padding-left: 8px;
  border-left: 2px solid #475569;
  background: rgba(30, 41, 59, 0.4);
  border-radius: 2px;
}
.lv-line-ctx { opacity: 0.85; }

.lv-sev-error   .lv-sev { color: #f87171; }
.lv-sev-error   { background: rgba(248, 113, 113, 0.05); }
.lv-sev-warn    .lv-sev { color: #fbbf24; }
.lv-sev-info    .lv-sev { color: #60a5fa; }
.lv-sev-debug   .lv-sev { color: #a78bfa; }
.lv-sev-unknown .lv-sev { color: #94a3b8; }

.lv-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #94a3b8;
  font-style: italic;
}
.rotating { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }

/* Floating "live tail paused" pill, bottom-center over the log body */
.lv-resume-pill {
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: #1e293b;
  color: #fbbf24;
  border: 1px solid #fbbf24;
  border-radius: 999px;
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 10;
}
.lv-resume-pill:hover { background: #334155; }
</style>
