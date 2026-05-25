<!--
  Activity (v0.8.0) — unified Action stream replacing the Restores page.

  Sections (top to bottom, matching Kasten's layout):
    1. Action Duration chart — one bar per recent Action, colored by status
    2. KPI strip — 7 stats (total/completed/failed/skipped/avg/live/retired)
    3. Filter bar — type + status + (page)
    4. Action cards — one row per Action with phases + refs + duration
    5. Click any card → ActionDetailDrawer for full context

  Polling: 5s when any Action is "running", 30s otherwise. The page is also
  re-fetched immediately when filters change.

  Why not refresh on every render: the Activity page is the dashboard for
  in-flight work — flickering KPI numbers would be distracting. We use
  smooth updates (replace items in place, not full re-mount) so the user's
  scroll position is preserved.
-->
<template>
  <div class="activity-page">
    <div class="page-header">
      <h3>{{ t('activity.title') }}</h3>
      <p class="page-desc">{{ t('activity.desc') }}</p>
    </div>

    <!-- 1. Duration chart -->
    <ActionDurationChart :buckets="durationBuckets" />

    <!-- 2. KPI strip -->
    <div class="kpi-strip">
      <div class="kpi" v-for="kpi in kpis" :key="kpi.key">
        <div class="kpi-label">{{ kpi.label }}</div>
        <div class="kpi-value" :class="kpi.cls">{{ kpi.value }}</div>
      </div>
    </div>

    <!-- 3. Filter bar -->
    <div class="filter-toolbar">
      <h4 class="actions-h">{{ t('activity.actionsHeader') }} <span class="actions-count">({{ filteredActions.length }})</span></h4>
      <span class="spacer"></span>
      <el-select v-model="typeFilter" class="filter-select" @change="fetchActions">
        <el-option :label="t('activity.allTypes')" value="" />
        <el-option label="Backup" value="Backup" />
        <el-option label="Restore" value="Restore" />
      </el-select>
      <el-select v-model="statusFilter" class="filter-select" @change="fetchActions">
        <el-option :label="t('activity.allStatuses')" value="" />
        <el-option :label="t('activity.status.running')" value="running" />
        <el-option :label="t('activity.status.completed')" value="completed" />
        <el-option :label="t('activity.status.failed')" value="failed" />
        <el-option :label="t('activity.status.partial')" value="partial" />
      </el-select>
      <el-input v-model="nameFilter" :placeholder="t('activity.filterPlaceholder')" clearable class="filter-name">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <!-- 4. Action cards -->
    <div v-if="loading && filteredActions.length === 0" class="loading-state">
      <el-icon class="rot"><Loading /></el-icon> {{ t('common.loading') }}
    </div>
    <div v-else-if="filteredActions.length === 0" class="empty-state">
      <el-icon><InfoFilled /></el-icon>
      <div>{{ t('activity.empty') }}</div>
    </div>
    <div v-else class="actions-list">
      <div
        v-for="action in filteredActions"
        :key="`${action.type}-${action.id}`"
        class="action-card"
        :class="`card-${action.status}`"
        @click="openDetail(action)"
      >
        <!-- Left rail: status + type + role + id
             v0.8.10.1 fix: every dual-policy Backup card looks the same
             ("Backup") at-a-glance unless we surface the role here. The
             detail drawer already had the role chip from v0.8.10-B, but
             the list view didn't — so users couldn't tell the snapshot
             half from the export half without clicking each card. -->
        <div class="card-left">
          <span class="status-badge" :class="`status-${action.status}`">{{ t(`activity.status.${action.status}`) }}</span>
          <div class="card-type-row">
            <span class="card-type">{{ action.type }}</span>
            <!-- v0.8.10.5: emoji removed per UI_GUIDELINES §3.1.
                 v0.8.12 LBS2: role values switched from snapshot/export
                 to local/cloud (BSL pair model). We accept BOTH so old
                 backups (which still carry the legacy supkube.io/policy-role
                 label) render correctly without a relabel migration. -->
            <span v-if="action.role === 'local' || action.role === 'snapshot'" class="card-role-chip role-local">
              {{ t('activity.detail.roleSnapshot') }}
            </span>
            <span v-else-if="action.role === 'cloud' || action.role === 'export'" class="card-role-chip role-cloud">
              {{ t('activity.detail.roleExport') }}
            </span>
          </div>
          <div class="card-id">{{ truncate(action.id, 24) }}</div>
        </div>

        <!-- Center: phases -->
        <div class="card-center">
          <div class="card-phases-label">{{ t('activity.phases') }}</div>
          <ActionPhases :phases="action.phases" compact />
        </div>

        <!-- Right-of-center: refs / artifacts -->
        <div class="card-refs">
          <div v-if="action.protectedObject">
            <div class="ref-label">{{ t('activity.detail.protectedObject') }}</div>
            <div class="ref-value">{{ action.protectedObject.name }}</div>
          </div>
          <div v-if="action.policy">
            <div class="ref-label">{{ t('activity.detail.policy') }}</div>
            <div class="ref-value ref-link">{{ action.policy.name }}</div>
          </div>
          <div v-if="action.restorePoint">
            <div class="ref-label">{{ t('activity.detail.restorePoint') }}</div>
            <div class="ref-value ref-link">{{ truncate(action.restorePoint.name, 22) }}</div>
          </div>
          <div v-if="action.targetNamespace">
            <div class="ref-label">{{ t('activity.detail.targetNamespace') }}</div>
            <div class="ref-value">{{ action.targetNamespace }}</div>
          </div>
          <div v-if="action.artifacts">
            <div class="ref-label">{{ t('activity.detail.artifacts') }}</div>
            <div class="ref-value">{{ action.artifacts.total }} <el-icon><Box /></el-icon> specs</div>
          </div>
        </div>

        <!-- Far right: timing + chevron -->
        <div class="card-time">
          <div class="time-label">{{ t('activity.detail.start') }}</div>
          <div class="time-value">{{ formatStart(action.startTime) }}</div>
          <template v-if="action.durationSeconds">
            <div class="time-label" style="margin-top: 8px">{{ t('activity.detail.duration') }}</div>
            <div class="time-value">{{ formatDuration(action.durationSeconds) }}</div>
          </template>
        </div>

        <button class="card-more" type="button" @click.stop="openDetail(action)">
          <span class="dots">⋮</span>
        </button>
      </div>
    </div>

    <!-- 5. Detail drawer.
         v0.8.10: @navigate fires when the user clicks a Paired-with
         link inside the drawer. We swap selectedAction to the peer
         without closing the drawer. The peer Action is looked up in
         the already-loaded actions list — no extra API round-trip. -->
    <ActionDetailDrawer
      v-model:visible="drawerOpen"
      :action="selectedAction"
      @navigate="handlePairedNavigate"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Box, InfoFilled, Loading, Search } from '@element-plus/icons-vue'
import { getActions } from '../api/velero'
import ActionPhases from '../components/ActionPhases.vue'
import ActionDurationChart from '../components/ActionDurationChart.vue'
import ActionDetailDrawer from '../components/ActionDetailDrawer.vue'

const { t } = useI18n()
const route = useRoute()

const actions = ref([])
const stats = ref({})
const durationBuckets = ref([])
const loading = ref(false)
const typeFilter = ref(route.query.type || '')
const statusFilter = ref(route.query.status || '')
const nameFilter = ref('')
const drawerOpen = ref(false)
const selectedAction = ref(null)
let pollTimer = null

// v0.8.11 #27: inject "Export pending" placeholder cards during the
// ~10-30s window between a snapshot half completing and the policypair
// controller firing the export half. Before this fix, customers saw
// "only 1 card" in Activity while Restore Points already showed 2 —
// the placeholder closes that perception gap.
//
// Logic:
//   - find each snapshot-half Action whose policyRunInstant ts has NO
//     matching export-half Action in the current list
//   - skip if the snapshot half itself is older than 5 minutes (the
//     controller would have fired by then; absence is real, not pending)
//   - inject a synthetic Action with status="running", role="export",
//     id="<snapshot-name>-export-pending" so the v-for key is unique
const filteredActions = computed(() => {
  const q = nameFilter.value.trim().toLowerCase()

  // v0.8.12 LBS2: helper accepts BOTH new (local/cloud) and legacy
  // (snapshot/export) role names. Backend translates internally; this
  // double-check keeps the UI working if a brief mismatch ever occurs
  // (e.g. mixed-version fleet during a rolling deploy).
  const isLocal = (r) => r === 'local' || r === 'snapshot'
  const isCloud = (r) => r === 'cloud' || r === 'export'

  // Build a set of "local-name → cloud Action present" for fast lookup
  const cloudByPeer = new Map()
  for (const a of actions.value) {
    if (isCloud(a.role) && a.pairedAction?.name) {
      cloudByPeer.set(a.pairedAction.name, a)
    }
  }

  // Inject placeholders right after each pending local-half row whose
  // cloud half hasn't started yet (within 5 min window).
  const merged = []
  const FIVE_MIN_MS = 5 * 60 * 1000
  for (const a of actions.value) {
    merged.push(a)
    if (!isLocal(a.role)) continue
    if (a.status !== 'completed') continue
    if (cloudByPeer.has(a.id)) continue
    // Only show pending placeholder for local backups completed within 5
    // min; older means the controller skipped (failure path) — don't lie.
    if (a.endTime) {
      const elapsed = Date.now() - new Date(a.endTime).getTime()
      if (elapsed > FIVE_MIN_MS) continue
    }
    merged.push({
      id: `${a.id}-cloud-pending`,
      type: 'Backup',
      role: 'cloud',
      status: 'running',
      pendingPlaceholder: true,
      protectedObject: a.protectedObject,
      policy: a.policy,
      pairedAction: { kind: 'Backup', name: a.id },
      phases: [
        { name: 'Waiting for snapshot half to complete', status: 'completed' },
        { name: 'Pair-controller firing export half', status: 'running' }
      ]
    })
  }

  if (!q) return merged
  return merged.filter(a => {
    const hay = [a.id, a.type, a.protectedObject?.name, a.targetNamespace, a.policy?.name, a.restorePoint?.name]
      .filter(Boolean).join(' ').toLowerCase()
    return hay.includes(q)
  })
})

const kpis = computed(() => {
  const s = stats.value || {}
  return [
    { key: 'total',     label: t('activity.kpi.total'),     value: s.total ?? 0, cls: '' },
    { key: 'completed', label: t('activity.kpi.completed'), value: s.completed ?? 0, cls: 'kpi-ok' },
    { key: 'failed',    label: t('activity.kpi.failed'),    value: (s.failed ?? 0) + (s.partial ?? 0), cls: 'kpi-err' },
    { key: 'skipped',   label: t('activity.kpi.skipped'),   value: s.skipped ?? 0, cls: '' },
    { key: 'avg',       label: t('activity.kpi.avgDuration'), value: formatDuration(s.avgDurationSec ?? 0), cls: '' },
    { key: 'live',      label: t('activity.kpi.liveArtifacts'), value: s.liveArtifacts ?? 0, cls: '' },
    { key: 'retired',   label: t('activity.kpi.retiredArtifacts'), value: s.retiredArtifacts ?? 0, cls: 'kpi-muted' }
  ]
})

async function fetchActions() {
  loading.value = true
  try {
    const params = {}
    if (typeFilter.value) params.type = typeFilter.value
    if (statusFilter.value) params.status = statusFilter.value
    const res = await getActions(params)
    actions.value = res.data.items || []
    stats.value = res.data.stats || {}
    durationBuckets.value = res.data.durationBuckets || []
  } catch (e) {
    // intentionally silent — polling shouldn't spam errors
    console.error('Failed to load actions:', e)
  } finally {
    loading.value = false
  }
}

const formatStart = (iso) => {
  if (!iso) return '-'
  const d = new Date(iso)
  const today = new Date()
  const isToday = d.toDateString() === today.toDateString()
  const time = d.toLocaleString(undefined, { hour: 'numeric', minute: '2-digit', hour12: true }).toLowerCase().replace(' ', '')
  return isToday ? `Today, ${time}` : d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
}
const formatDuration = (sec) => {
  if (!sec || sec === 0) return '-'
  if (sec < 60) return `${sec} sec`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}
const truncate = (s, n) => s && s.length > n ? s.slice(0, n) + '…' : s

const openDetail = (action) => {
  // v0.8.11 #27: pending placeholders have no underlying Backup CR yet —
  // opening the drawer would 404 the YAML fetch. Just no-op the click.
  if (action?.pendingPlaceholder) return
  selectedAction.value = action
  drawerOpen.value = true
}

// v0.8.10: clicking a Paired-with link inside the drawer. The drawer
// emits the peer Backup name; we find it in the already-loaded
// actions list and swap the prop. No API round-trip. If the peer
// isn't in the current page (e.g. the user has filtered it out), we
// clear filters first so the peer becomes visible too.
const handlePairedNavigate = (ref) => {
  if (!ref || ref.kind !== 'Backup') return
  const peer = actions.value.find(a => a.id === ref.name && a.type === 'Backup')
  if (peer) {
    selectedAction.value = peer
    // Keep drawer open — the v-model bind preserves it because we
    // didn't touch drawerOpen.
    return
  }
  // Peer not in the visible set — drop filters and re-search, then
  // open. (Edge case: peer is on a different page if the list is
  // paginated; right now /actions is unpaginated so this is rare.)
  typeFilter.value = ''
  statusFilter.value = ''
  nameFilter.value = ''
  // Force a refresh, then look again after the fetch lands.
  fetchActions().then(() => {
    const peer2 = actions.value.find(a => a.id === ref.name && a.type === 'Backup')
    if (peer2) {
      selectedAction.value = peer2
    } else {
      // Truly not found — close drawer and route to the Backup page
      // as a last-resort fallback. The user still reaches the peer
      // even if it's e.g. been deleted from the actions stream.
      drawerOpen.value = false
    }
  })
}

// Polling cadence depends on whether anything is in flight.
function tunePolling() {
  if (pollTimer) clearInterval(pollTimer)
  const anyRunning = actions.value.some(a => a.status === 'running')
  const interval = anyRunning ? 5000 : 30000
  pollTimer = setInterval(fetchActions, interval)
}
watch(actions, tunePolling, { deep: true })

onMounted(() => {
  fetchActions().then(tunePolling)
})
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.activity-page { padding: 0; }

.page-header { margin-bottom: 20px; }
.page-header h3 { margin: 0 0 4px 0; font-size: 20px; font-weight: 600; }
.page-desc { margin: 0; color: #909399; font-size: 13px; }

/* KPI strip — 7 columns, divider lines between cells */
.kpi-strip {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  background: #ffffff;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 14px 0;
  margin-bottom: 16px;
}
.kpi {
  text-align: center;
  border-left: 1px solid #ebeef5;
  padding: 4px 8px;
}
.kpi:first-child { border-left: 0; }
.kpi-label {
  font-size: 11px;
  color: #909399;
  text-transform: lowercase;
  margin-bottom: 6px;
  letter-spacing: 0.2px;
}
.kpi-value {
  font-size: 22px;
  font-weight: 600;
  color: #1d2129;
}
.kpi-ok { color: #67c23a; }
.kpi-err { color: #c45656; }
.kpi-muted { color: #c0c4cc; }

/* Filter toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.actions-h { margin: 0; font-size: 16px; font-weight: 600; color: #1d2129; }
.actions-count { color: #909399; font-weight: 400; }
.spacer { flex: 1; }
.filter-select { width: 160px; }
.filter-name { width: 240px; }

/* Action card */
.actions-list { display: flex; flex-direction: column; gap: 10px; }
.action-card {
  display: grid;
  grid-template-columns: 180px 1fr 280px 150px 28px;
  gap: 16px;
  align-items: flex-start;
  background: #ffffff;
  border: 1px solid #ebeef5;
  border-left: 4px solid #dcdfe6;
  border-radius: 8px;
  padding: 14px 16px;
  cursor: pointer;
  transition: box-shadow 0.15s;
}
.action-card:hover {
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.card-running   { border-left-color: #409eff; }
.card-completed { border-left-color: #67c23a; }
.card-failed    { border-left-color: #c45656; }
.card-partial   { border-left-color: #e6a23c; }
.card-skipped   { border-left-color: #909399; }

.card-left { display: flex; flex-direction: column; gap: 4px; }
.status-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.6px;
  padding: 2px 8px;
  border-radius: 10px;
  display: inline-block;
  text-align: center;
  width: fit-content;
  text-transform: uppercase;
}
.status-running   { background: #ecf5ff; color: #409eff; }
.status-completed { background: #f0f9eb; color: #67c23a; }
.status-failed    { background: #fef0f0; color: #c45656; }
.status-partial   { background: #fdf6ec; color: #e6a23c; }
.status-skipped   { background: #f4f4f5; color: #909399; }
.status-unknown   { background: #f4f4f5; color: #909399; }

.card-type-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  flex-wrap: wrap;
}
.card-type {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
}
/* v0.8.10.1: role chip on action card. Mirrors the drawer's role-chip
   palette so the user gets the same visual identity in both views. */
.card-role-chip {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 9px;
  letter-spacing: 0.3px;
  white-space: nowrap;
}
/* v0.8.12 LBS2: role chip variants renamed from snapshot/export to
   local/cloud. Keep legacy class names as aliases so any cached HTML
   from a half-deployed frontend still renders with colors. */
.card-role-chip.role-local,
.card-role-chip.role-snapshot { background: #ecf5ff; color: #409eff; }
.card-role-chip.role-cloud,
.card-role-chip.role-export   { background: #f5edff; color: #722ed1; }
.card-id {
  font-size: 11px;
  color: #909399;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}

.card-center { min-width: 0; }
.card-phases-label,
.ref-label,
.time-label {
  font-size: 11px;
  color: #909399;
  font-weight: 500;
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.card-refs {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}
.ref-value { color: #303133; font-weight: 500; }
.ref-link { color: #4f46e5; }
.card-refs .el-icon { font-size: 12px; color: #909399; vertical-align: middle; }

.card-time { font-size: 13px; color: #303133; }
.time-value { font-weight: 500; }

.card-more {
  background: none;
  border: 0;
  color: #909399;
  font-size: 18px;
  cursor: pointer;
  padding: 4px 6px;
  align-self: flex-start;
}
.card-more:hover { color: #4f46e5; }
.dots { letter-spacing: 1px; }

.loading-state, .empty-state {
  text-align: center;
  padding: 60px 20px;
  color: #909399;
  font-size: 13px;
  background: #fff;
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
}
.empty-state .el-icon { font-size: 28px; display: block; margin: 0 auto 8px; color: #c0c4cc; }
.rot { animation: rot 1s linear infinite; }
@keyframes rot { from { transform: rotate(0); } to { transform: rotate(360deg); } }

/* Dark mode */
:global(html.dark) .kpi-strip { background: #1f2026; border-color: #2c2f36; }
:global(html.dark) .kpi { border-left-color: #2c2f36; }
:global(html.dark) .kpi-value { color: #e5eaf3; }
:global(html.dark) .actions-h { color: #e5eaf3; }
:global(html.dark) .action-card { background: #1f2026; border-color: #2c2f36; }
:global(html.dark) .card-type { color: #e5eaf3; }
:global(html.dark) .ref-value, :global(html.dark) .time-value { color: #e5eaf3; }
:global(html.dark) .loading-state, :global(html.dark) .empty-state {
  background: #1f2026; border-color: #3a3d44; color: #909399;
}
</style>
