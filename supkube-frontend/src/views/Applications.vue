<template>
  <div class="applications-page">
    <div class="page-header">
      <h3>{{ t('applications.title') }}</h3>
      <p class="page-desc">{{ t('applications.desc') }}</p>
    </div>

    <!-- Kasten-style filter toolbar: status quick-filter + free-text name search + selection summary -->
    <div class="filter-toolbar">
      <el-select v-model="statusFilter" class="filter-status" placement="bottom-start">
        <el-option :label="t('applications.statusFilterAll')" value="all" />
        <el-option :label="t('compliance.Compliant')" value="Compliant" />
        <el-option :label="t('compliance.Unmanaged')" value="Unmanaged" />
        <el-option :label="t('compliance.NonCompliant')" value="NonCompliant" />
        <el-option :label="t('compliance.InProgress')" value="InProgress" />
        <el-option :label="t('compliance.Empty')" value="Empty" />
      </el-select>
      <el-input
        v-model="nameFilter"
        :placeholder="t('common.filterByName')"
        clearable
        class="filter-name"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="filter-spacer"></span>
      <span class="filter-selected">{{ selectedRows.length }} selected</span>
    </div>

    <el-card>
      <el-table
        :data="filteredApplications"
        style="width: 100%"
        v-loading="loading"
        row-class-name="app-row"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="48" />
        <!-- v0.8.10.6: Name column carries app name + Status chip.
             Status column dropped (was a whole 180px wide for one chip). -->
        <el-table-column prop="namespace" :label="t('common.name')" sortable min-width="220">
          <template #default="{ row }">
            <div class="app-cell">
              <div class="app-name">{{ row.namespace }}</div>
              <div class="app-cell-chips">
                <span class="sk-chip" :class="`sk-chip-status-${complianceChipKey(row)}`">
                  {{ complianceLabel(row) }}
                </span>
              </div>
            </div>
          </template>
        </el-table-column>
        <!-- v0.8.10.6: Components column — number only, no Setting icon.
             Per user instruction "don't want any icons" in the table. -->
        <el-table-column :label="t('applications.components')" width="120" sortable
          :sort-method="(a, b) => (a.components || 0) - (b.components || 0)">
          <template #default="{ row }">
            <span class="workload-count">{{ row.components ?? 0 }}</span>
          </template>
        </el-table-column>
        <!-- v0.8.10.6: Labels column — vertical stack, ONE label per row.
             Old horizontal "row-labels" used flex-wrap which made narrow
             columns wrap awkwardly. Each label its own line is calmer. -->
        <el-table-column :label="t('applications.labels')" min-width="260">
          <template #default="{ row }">
            <div class="row-labels-col">
              <el-tooltip
                v-for="entry in rowLabelPreview(row)"
                :key="entry[0]"
                :content="`${entry[0]}:${entry[1]}`"
                placement="top"
                :show-after="300"
              >
                <span class="row-label-tag">{{ entry[0] }}:{{ entry[1] }}</span>
              </el-tooltip>
              <span v-if="rowLabelHidden(row) > 0" class="row-labels-more sk-caption">
                +{{ rowLabelHidden(row) }} more
              </span>
              <span v-if="rowLabelCount(row) === 0" class="muted">—</span>
            </div>
          </template>
        </el-table-column>
        <!-- v0.8.5: Restore Point count column. Clickable when >0 — takes
             user straight to the filtered Restore Points list.
             v0.8.10.6: folder icon removed per user instruction. -->
        <el-table-column :label="t('applications.restorePoints')" width="120" sortable
          :sort-method="(a, b) => (a.restorePointCount || 0) - (b.restorePointCount || 0)">
          <template #default="{ row }">
            <a
              v-if="(row.restorePointCount || 0) > 0"
              class="rp-count rp-count-link"
              @click="goRestorePoints(row)"
            >{{ row.restorePointCount }}</a>
            <span v-else class="rp-count rp-count-zero">0</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('applications.lastBackup')" min-width="220">
          <template #default="{ row }">
            <span v-if="row.lastBackupName" class="last-backup">
              {{ row.lastBackupName }}
              <span class="backup-time">{{ formatTime(row.lastBackupTime) }}</span>
            </span>
            <span v-else class="no-restore">{{ t('applications.noRestorePoint') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="" width="60" align="right">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="handleCommand($event, row)">
              <el-button class="more-btn" text>
                <span class="dots">⋮</span>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="snapshot">{{ t('applications.snapshot') }}</el-dropdown-item>
                  <el-dropdown-item command="restore">{{ t('common.restore') }}</el-dropdown-item>
                  <el-dropdown-item command="backup">{{ t('applications.backup') }}</el-dropdown-item>
                  <el-dropdown-item command="details">{{ t('common.details') }}</el-dropdown-item>
                  <el-dropdown-item command="policy">{{ t('applications.createPolicy') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Application Details Drawer
         v0.8.10.2: aligned to UI_GUIDELINES §5 (Action Details template).
         Same H1/H2 structure, same section H3 style, same sticky footer.
         No 📦 emoji on H2 (per §3.1 — emoji only in chip/badge). -->
    <el-drawer
      v-model="drawerVisible"
      direction="rtl"
      :size="'560px'"
      :destroy-on-close="true"
      :with-header="false"
    >
      <div v-if="selectedApp" class="sk-drawer" v-loading="detailLoading">
        <button class="sk-drawer-close" type="button" @click="drawerVisible = false" aria-label="Close">×</button>

        <!-- ════ Header zone (H1 + H2 + chip) ════ -->
        <div class="sk-drawer-header">
          <div class="sk-h1">{{ t('activity.detail.titleApplication') }}</div>
          <div class="sk-h2 sk-drawer-subject" :title="selectedApp.namespace">{{ selectedApp.namespace }}</div>
          <div class="sk-drawer-chips">
            <span class="sk-chip" :class="`sk-chip-status-${complianceChipKey(selectedApp)}`">
              {{ complianceLabel(selectedApp) }}
            </span>
            <span class="sk-chip sk-chip-status-muted">Namespace</span>
          </div>
        </div>

        <!-- ════ Section: LABELS ════ -->
        <section class="sk-section">
          <div class="sk-h3">{{ t('applications.labels') }}</div>
          <div class="labels-container">
            <el-tag
              v-for="entry in visibleLabelEntries"
              :key="entry[0]"
              size="default"
              effect="plain"
              round
              class="label-tag"
            >{{ entry[0] }}:{{ entry[1] }}</el-tag>
            <span v-if="labelEntries.length === 0" class="sk-caption">No labels</span>
          </div>
          <a
            v-if="hiddenLabelCount > 0"
            class="sk-link"
            style="display:inline-block;margin-top:6px;font-size:12px"
            @click="labelsExpanded = !labelsExpanded"
          >
            {{ labelsExpanded ? 'Show fewer labels' : `Show ${hiddenLabelCount} more labels …` }}
          </a>
        </section>

        <!-- ════ Section: KUBECTL — command-line shortcut ════ -->
        <section class="sk-section">
          <div class="sk-h3">kubectl</div>
          <div class="kubectl-block">
            <code>$ kubectl get all -n {{ selectedApp.namespace }}</code>
            <el-button size="small" text class="copy-btn" @click="copyKubectl">copy</el-button>
          </div>
        </section>

        <!-- ════ Section: ARTIFACTS (UNIFIED with Action Details) ════
             v0.8.10.3 K2: same Workloads/Configuration/Networking/
             Storage/RBAC accordion as Action Details. Same row format:
             kind / name / namespace / </> YAML button. Customers see
             identical UX whether they're inspecting a backup or a live
             namespace. Data is computed client-side from detailData
             (pods/services/etc.) into the same `groups` shape that the
             backup artifact-breakdown endpoint emits.
        -->
        <section class="sk-section">
          <div class="sk-h3">{{ t('activity.detail.artifacts') }}</div>

          <!-- Stat strip — only one number for live ns (no Velero count
               or infrastructure split since this isn't a backup). -->
          <div v-if="appBreakdownTotal > 0" class="sk-art-strip">
            <span class="sk-art-stat">
              <span class="sk-art-stat-num">{{ appBreakdownTotal }}</span>
              <span class="sk-art-stat-label">{{ t('activity.detail.applicationItems') }}</span>
            </span>
          </div>

          <div class="sk-art-groups">
            <div
              v-for="g in appBreakdown"
              :key="g.key"
              class="sk-art-group sk-art-cat-application"
            >
              <button class="sk-art-group-header" type="button" @click="toggleAppGroup(g.key)">
                <span class="sk-art-group-label sk-body-strong">{{ g.label }}</span>
                <span class="sk-art-group-count">{{ g.count }}</span>
                <span class="sk-art-group-chevron">{{ appExpandedGroups[g.key] ? '▾' : '▸' }}</span>
              </button>
              <div v-if="appExpandedGroups[g.key]" class="sk-art-group-items">
                <div v-for="(item, i) in g.items" :key="`${g.key}-${i}`" class="sk-art-item">
                  <span class="sk-art-item-kind sk-caption">{{ item.kind }}</span>
                  <span class="sk-art-item-name sk-body">{{ item.name }}</span>
                  <span v-if="item.namespace" class="sk-art-item-ns sk-caption">{{ item.namespace }}</span>
                  <button
                    class="sk-art-item-yaml"
                    type="button"
                    :title="t('activity.detail.viewYamlItem')"
                    @click="openAppYamlPreview(item)"
                  ><el-icon><Document /></el-icon></button>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>

      <!-- ════ Sticky bottom action bar — applies UI_GUIDELINES §5.2 ════ -->
      <div v-if="selectedApp" class="sk-drawer-footer">
        <el-button type="primary" @click="handleCommand('snapshot', selectedApp)">{{ t('applications.snapshot') }}</el-button>
        <el-button @click="handleCommand('restore', selectedApp)">{{ t('common.restore') }}</el-button>
        <el-button @click="handleCommand('policy', selectedApp)">{{ t('applications.createPolicy') }}</el-button>
        <span class="sk-drawer-footer-spacer"></span>
        <el-button @click="drawerVisible = false">{{ t('common.close') }}</el-button>
      </div>
    </el-drawer>

    <!-- v0.8.10.3: shared YAML preview for </> clicks in this drawer. -->
    <YamlPreviewModal
      v-model:visible="yamlPreviewOpen"
      :kind="yamlPreviewTarget.kind"
      :name="yamlPreviewTarget.name"
      :namespace="yamlPreviewTarget.namespace"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import { useRouter } from 'vue-router'
import { Search, FolderOpened, Setting, Document } from '@element-plus/icons-vue'
import YamlPreviewModal from '../components/YamlPreviewModal.vue'
import { getApplications, getApplicationDetail, createManualSnapshot } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const applications = ref([])
const loading = ref(false)
const drawerVisible = ref(false)
const selectedApp = ref(null)
const detailLoading = ref(false)
const detailData = ref({})

// Kasten-style filter state
const statusFilter = ref('all')
const nameFilter = ref('')
const selectedRows = ref([])

const filteredApplications = computed(() => {
  const name = nameFilter.value.trim().toLowerCase()
  return applications.value.filter((row) => {
    if (statusFilter.value !== 'all') {
      const c = row?.complianceStatus || (row?.protected ? 'Compliant' : 'Unmanaged')
      if (c !== statusFilter.value) return false
    }
    if (name && !(row.namespace || '').toLowerCase().includes(name)) return false
    return true
  })
})

const handleSelectionChange = (rows) => {
  selectedRows.value = rows
}

// Expand/collapse states — legacy refs kept for compatibility with
// any caller that still binds to them; the new unified Artifacts
// section uses appExpandedGroups instead.
const podsExpanded = ref(true)
const servicesExpanded = ref(true)
const deploymentsExpanded = ref(true)
const statefulsetsExpanded = ref(true)
const pvcsExpanded = ref(true)
const configmapsExpanded = ref(false)

// ─── v0.8.10.3 K2: unified ARTIFACTS breakdown for the live ns ───
//
// Transform getApplicationDetails() data into the same shape that the
// backup artifact-breakdown endpoint emits. The drawer renders this
// using the global sk-art-* classes (tokens.css) — pixel-identical
// to Action Details.
//
// Categories match UI_GUIDELINES §5 + backup breakdown:
//   - workloads:  Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets
//   - config:     ConfigMaps, Secrets
//   - networking: Services
//   - storage:    PVCs
//   - rbac:       ServiceAccounts (when detail endpoint returns them)
//
// Items dropped from the previous design:
//   - Pod STATUS / READY / RESTARTS — was a nice-to-have but breaks
//     the unification promise. User can click </> on the row to see
//     the live YAML which has all those fields.
//   - Service TYPE / PORTS — same logic.
//   - PVC STATUS / CAPACITY — same logic.
const appBreakdown = computed(() => {
  const d = detailData.value || {}
  const ns = selectedApp.value?.namespace || ''
  const groups = []
  const push = (key, label, kinds) => {
    const items = []
    for (const [k, arr] of kinds) {
      if (Array.isArray(arr)) {
        for (const r of arr) {
          items.push({ kind: k, name: r.name, namespace: ns })
        }
      }
    }
    if (items.length > 0) {
      groups.push({ key, label, count: items.length, items })
    }
  }
  push('workloads', t('activity.detail.artifactsGroup.workloads') || 'Workloads', [
    ['Deployment',  d.deployments],
    ['StatefulSet', d.statefulSets],
    ['DaemonSet',   d.daemonSets],
    ['ReplicaSet',  d.replicaSets],
    ['Pod',         d.pods]
  ])
  push('config', t('activity.detail.artifactsGroup.config') || 'Configuration', [
    ['ConfigMap', d.configMaps],
    ['Secret',    d.secrets]
  ])
  push('networking', t('activity.detail.artifactsGroup.networking') || 'Networking', [
    ['Service', d.services]
  ])
  push('storage', t('activity.detail.artifactsGroup.storage') || 'Storage', [
    ['PersistentVolumeClaim', d.pvcs]
  ])
  push('rbac', t('activity.detail.artifactsGroup.rbac') || 'RBAC', [
    ['ServiceAccount', d.serviceAccounts]
  ])
  return groups
})
const appBreakdownTotal = computed(() =>
  appBreakdown.value.reduce((sum, g) => sum + g.count, 0)
)
const appExpandedGroups = ref({})
const toggleAppGroup = (key) => {
  appExpandedGroups.value[key] = !appExpandedGroups.value[key]
}

// ─── YAML preview state (shared by all rows in this drawer) ───
const yamlPreviewOpen = ref(false)
const yamlPreviewTarget = ref({ kind: '', name: '', namespace: '' })
const openAppYamlPreview = (item) => {
  yamlPreviewTarget.value = {
    kind: item.kind,
    name: item.name,
    namespace: item.namespace || ''
  }
  yamlPreviewOpen.value = true
}

const selectedAppLabels = computed(() => {
  if (!detailData.value || !detailData.value.labels) return {}
  return detailData.value.labels
})

// Sort labels deterministically — short user labels first, kubernetes.io/* last —
// so the most informative tags fit in the collapsed preview row.
const LABEL_PREVIEW_COUNT = 5
const labelsExpanded = ref(false)
const labelEntries = computed(() => {
  const entries = Object.entries(selectedAppLabels.value || {})
  return entries.sort(([a], [b]) => {
    const aSys = a.includes('kubernetes.io/') || a.includes('k8s.io/')
    const bSys = b.includes('kubernetes.io/') || b.includes('k8s.io/')
    if (aSys !== bSys) return aSys ? 1 : -1
    return a.localeCompare(b)
  })
})
const visibleLabelEntries = computed(() =>
  labelsExpanded.value ? labelEntries.value : labelEntries.value.slice(0, LABEL_PREVIEW_COUNT)
)
const hiddenLabelCount = computed(() =>
  Math.max(0, labelEntries.value.length - LABEL_PREVIEW_COUNT)
)

const formatTime = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
}

// In-row label preview: show up to 2 user labels per row so the table stays
// scannable. The Details drawer shows the full set.
const ROW_LABEL_PREVIEW = 2
const sortLabelEntries = (labels) => {
  return Object.entries(labels || {}).sort(([a], [b]) => {
    const aSys = a.includes('kubernetes.io/') || a.includes('k8s.io/')
    const bSys = b.includes('kubernetes.io/') || b.includes('k8s.io/')
    if (aSys !== bSys) return aSys ? 1 : -1
    return a.localeCompare(b)
  })
}
const rowLabelCount = (row) => Object.keys(row?.labels || {}).length
const rowLabelPreview = (row) => sortLabelEntries(row?.labels).slice(0, ROW_LABEL_PREVIEW)
const rowLabelHidden = (row) => Math.max(0, rowLabelCount(row) - ROW_LABEL_PREVIEW)

// Status badge values come from the backend's ComplianceStatus field.
// Older payloads (pre-0.5.1) only carry the `protected` boolean; fall back
// to that so the UI doesn't break during a rolling upgrade.
const complianceOf = (row) => {
  if (row?.complianceStatus) return row.complianceStatus
  return row?.protected ? 'Compliant' : 'Unmanaged'
}
const complianceClass = (row) => {
  const c = complianceOf(row)
  return {
    Compliant: 'compliant',
    Unmanaged: 'unmanaged',
    NonCompliant: 'noncompliant',
    InProgress: 'inprogress',
    Empty: 'empty'
  }[c] || 'unmanaged'
}
const complianceIcon = (row) => {
  const c = complianceOf(row)
  return {
    Compliant: '✓',
    Unmanaged: '!',
    NonCompliant: '✕',
    InProgress: '⟳',
    Empty: '–'
  }[c] || '!'
}
const complianceLabel = (row) => {
  const c = complianceOf(row)
  // i18n key `compliance.<state>` is defined in locales/en.js + zh-CN.js;
  // missing keys fall through to the literal key thanks to fallbackLocale.
  return t(`compliance.${c}`)
}
// v0.8.10.2: chip CSS key for the compliance status in the details
// drawer. Maps each compliance state onto the global status taxonomy
// declared in tokens.css (.sk-chip-status-success / warning / error).
const complianceChipKey = (row) => {
  const c = complianceOf(row)
  return {
    Compliant:    'success',
    Unmanaged:    'warning',
    NonCompliant: 'error',
    InProgress:   'running',
    Empty:        'muted'
  }[c] || 'muted'
}

const fetchApplications = async () => {
  loading.value = true
  try {
    const res = await getApplications()
    applications.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load applications')
    console.error(e)
  } finally {
    loading.value = false
  }
}

// v0.8.5: clicking the Restore Points cell jumps to the Restore Points
// list page pre-filtered by this app's namespace (intent=restore so the
// user immediately sees the orientation banner). Same deep-link contract
// the kebab → Restore command uses.
const goRestorePoints = (row) => {
  router.push({ path: '/backups', query: { namespace: row.namespace, intent: 'restore' } })
}

const handleCommand = (command, row) => {
  switch (command) {
    case 'snapshot':
      // v0.8.9.2: one-click cluster-local CSI snapshot. Distinct from
      // "Backup" which still routes to the parameterized Create flow.
      // This is the "instant rollback point" UX promise from the Apps
      // page header. See manual_snapshot.go for backend rationale.
      runManualSnapshot(row)
      break
    case 'backup':
      // v0.9.1.10 (#108): "Backup" an application = set up a protection
      // Policy for it (Kasten mental model), NOT a one-off ad-hoc Restore
      // Point. Mars (2026-05-28 demo): "应用点击 Backup 应该跳转到 Policy
      // 而不是 Restore Point". We route to the Policies page with
      // intent=create so the Create-Policy wizard opens pre-filled with this
      // app's namespace. (One-off backups still available via the Restore
      // Points page's own Create button; the instant path is "Snapshot".)
      router.push({ path: '/policies', query: { namespace: row.namespace, intent: 'create' } })
      break
    case 'restore':
      // FIX (v0.7.13): "Restore" on an Application means "pick one of this
      // app's existing Restore Points and restore from it" — NOT "go to the
      // Restores history page". We route to /backups (Restore Points list)
      // pre-filtered by the app's namespace, like Kasten does. From there
      // the user picks a row and the kebab → Restore opens the drawer.
      router.push({ path: '/backups', query: { namespace: row.namespace, intent: 'restore' } })
      break
    case 'policy':
      router.push({ path: '/policies', query: { namespace: row.namespace } })
      break
    case 'details':
      openDetail(row)
      break
  }
}

// v0.8.9.2: one-click manual Snapshot. The dialog itself is intentionally
// tiny — a single optional comment field — because the whole point of the
// endpoint is "no config, just do it". Anything more elaborate (TTL,
// storage location, mover toggle) lives on the Restore Points → Create
// flow, which routes to /backups intent=create.
//
// The comment is the one piece of metadata users repeatedly asked for
// in v0.7 ("we have 8 snapshots taken today; which one is the one I
// took right before that risky deploy?"). It rides in the body and the
// backend stamps it onto the Backup annotation supkube.io/comment.
const runManualSnapshot = async (row) => {
  let comment = ''
  try {
    const result = await ElMessageBox.prompt(
      t('applications.snapshotDialogBody'),
      t('applications.snapshotDialogTitle', { ns: row.namespace }),
      {
        confirmButtonText: t('applications.snapshotConfirm'),
        cancelButtonText: t('common.cancel') || 'Cancel',
        inputPlaceholder: t('applications.snapshotCommentPlaceholder'),
        // Allow empty comment — it's optional.
        inputValidator: () => true,
        inputType: 'text',
        // Slightly larger so the body text isn't squeezed.
        customClass: 'manual-snapshot-dialog'
      }
    )
    comment = (result && result.value) ? result.value.trim() : ''
  } catch (_e) {
    // User cancelled — no toast, just bail.
    return
  }
  try {
    const res = await createManualSnapshot(row.namespace, comment ? { comment } : {})
    const backupName = res?.data?.backupName || ''
    ElMessage.success(t('applications.snapshotStarted', { ns: row.namespace, name: backupName }))
    // Refresh after a short delay so the Last Backup column picks up
    // the new RP without the user having to F5.
    setTimeout(fetchApplications, 1500)
  } catch (e) {
    // Backend signals protected-namespace via 400 + a recognizable error
    // string. Surface a localized message instead of the raw text so
    // non-English users still get help.
    const status = e?.response?.status
    const msg = e?.response?.data?.error || e.message || 'unknown error'
    if (status === 400 && msg.toLowerCase().includes('protected')) {
      ElMessage.warning(t('applications.snapshotProtectedNs'))
    } else {
      ElMessage.error(t('applications.snapshotFailed', { msg }))
    }
    console.error('Manual snapshot failed:', e)
  }
}

const openDetail = async (row) => {
  selectedApp.value = row
  detailData.value = {}
  detailLoading.value = true
  drawerVisible.value = true
  // Reset expand states (legacy refs + new unified)
  podsExpanded.value = true
  servicesExpanded.value = true
  deploymentsExpanded.value = true
  statefulsetsExpanded.value = true
  pvcsExpanded.value = true
  configmapsExpanded.value = false
  // v0.8.10.3: open Workloads by default — same UX as Action Details.
  appExpandedGroups.value = { workloads: true }
  try {
    const res = await getApplicationDetail(row.namespace)
    detailData.value = res.data || {}
  } catch (e) {
    console.warn('Failed to load application details:', e)
    // Fallback to basic row data
    detailData.value = { ...row }
  } finally {
    detailLoading.value = false
  }
}

const copyKubectl = () => {
  if (!selectedApp.value) return
  const cmd = `kubectl get all -n ${selectedApp.value.namespace}`
  navigator.clipboard.writeText(cmd).then(() => {
    ElMessage.success('Copied to clipboard')
  })
}

onMounted(() => {
  fetchApplications()
})
</script>

<style scoped>
.page-header {
  margin-bottom: 20px;
}
.page-header h3 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
}
.page-desc {
  margin: 0;
  color: #909399;
  font-size: 13px;
}
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.filter-status {
  width: 200px;
}
.filter-name {
  width: 320px;
}
.filter-spacer {
  flex: 1;
}
.filter-selected {
  color: #606266;
  font-size: 13px;
  font-weight: 500;
}
.app-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--sk-text);
}
/* v0.8.10.6: Name cell carries ns name + status chip stacked. Same
   compression pattern as Policies + Restore Points first column. */
.app-cell {
  display: flex;
  flex-direction: column;
  gap: var(--sk-space-xs);
  padding: 4px 0;
}
.app-cell-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-xs);
  align-items: center;
}
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  font-weight: 500;
}
.status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  font-size: 11px;
  font-weight: bold;
}
.status-compliant { color: #67c23a; }
.status-compliant .status-icon { border: 2px solid #67c23a; color: #67c23a; }
.status-unmanaged { color: #e6a23c; }
.status-unmanaged .status-icon { border: 2px solid #e6a23c; color: #e6a23c; }
.status-noncompliant { color: #f56c6c; }
.status-noncompliant .status-icon { border: 2px solid #f56c6c; color: #f56c6c; }
.status-inprogress { color: #409eff; }
.status-inprogress .status-icon { border: 2px solid #409eff; color: #409eff; }
.status-empty { color: #c0c4cc; }
.status-empty .status-icon { border: 2px solid #c0c4cc; color: #c0c4cc; }
/* v0.8.10.6: Setting icon removed — number alone. */
.workload-count {
  color: var(--sk-text);
  font-size: 13px;
  font-weight: 500;
}

/* v0.8.5: Restore Point count cell */
.rp-count {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  font-weight: 500;
}
.rp-count .el-icon { font-size: 14px; }
.rp-count-link {
  color: var(--sk-primary);
  cursor: pointer;
  padding: 3px 8px;
  margin: -3px -8px;
  border-radius: 4px;
  transition: background 120ms ease, color 120ms ease;
}
.rp-count-link:hover {
  text-decoration: underline;
  background: var(--sk-primary-bg-hover);
  color: var(--sk-primary-hover);
}
.rp-count-link:active {
  background: var(--sk-primary-active);
}
.rp-count-zero {
  color: var(--sk-text-placeholder);
  font-weight: 400;
  cursor: default;
}
/* v0.8.10.6: vertical Labels stack — one label per row. The previous
   horizontal flex-wrap version made narrow columns look chaotic. */
.row-labels-col {
  display: flex;
  flex-direction: column;
  gap: var(--sk-space-xs);
  align-items: flex-start;
  padding: 4px 0;
}
.row-label-tag {
  display: inline-block;
  font-size: 11px;
  padding: 2px 10px;
  line-height: 1.5;
  font-weight: 500;
  border-radius: 12px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  color: var(--sk-text-secondary);
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* Legacy `.row-labels` class kept so any other code paths don't break. */
.row-labels {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.row-labels-more {
  font-size: 12px;
  color: #909399;
}
.row-labels-empty {
  color: #c0c4cc;
  font-size: 13px;
}
.last-backup { display: flex; flex-direction: column; gap: 2px; }
.backup-time { color: #409eff; font-size: 12px; }
.no-restore { color: #c0c4cc; font-size: 13px; }
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.app-row:hover .more-btn) { color: #409eff; }

/* ════════════════════════════════════════════════════════════════════
 * Application Details drawer — v0.8.10.2 (UI_GUIDELINES §5)
 * Mirrors ActionDetailDrawer's sk-* class set so customers see the same
 * shell across pages. Local rules below are app-specific (labels,
 * kubectl block, k8s sub-tables); shared structure (header, sections,
 * sticky footer) follows the global pattern.
 * ════════════════════════════════════════════════════════════════════ */

/* Reset the el-drawer body — we own the layout, including the footer */
:deep(.el-drawer__body) {
  padding: 0;
  display: flex;
  flex-direction: column;
}

/* ─── Drawer shell — same as ActionDetailDrawer.sk-drawer ─── */
.sk-drawer {
  flex: 1 1 auto;
  overflow-y: auto;
  padding: 28px var(--sk-drawer-padding-x) var(--sk-drawer-section-spacing);
  position: relative;
  min-height: 0;
}
.sk-drawer-close {
  position: absolute;
  top: 14px;
  right: 14px;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  font-size: 22px;
  line-height: 1;
  color: var(--sk-text-muted);
  cursor: pointer;
  border-radius: 4px;
  z-index: 2;
  transition: background 120ms ease, color 120ms ease;
}
.sk-drawer-close:hover {
  background: var(--sk-bg-hover);
  color: var(--sk-text);
}

.sk-drawer-header { margin-bottom: var(--sk-drawer-header-spacing); }
.sk-drawer-subject {
  margin-top: var(--sk-space-xs);
  margin-bottom: var(--sk-space-md);
  word-break: break-all;
}
.sk-drawer-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-sm);
  align-items: center;
}

/* ─── Section + collapsible button ─── */
.sk-section {
  margin-bottom: var(--sk-drawer-section-spacing);
  padding-bottom: var(--sk-drawer-section-spacing);
  border-bottom: 1px solid var(--sk-border);
}
.sk-section:last-of-type { border-bottom: 0; padding-bottom: 0; }
.sk-section .sk-h3 { margin-bottom: var(--sk-space-md); }

.sk-collapsible {
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  width: 100%;
  border: none;
  background: transparent;
  padding: 0 0 var(--sk-space-md);
  text-align: left;
  cursor: pointer;
  font: inherit;
}
.sk-section-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  padding: 1px 6px;
  border-radius: 10px;
  background: var(--sk-bg-soft);
  color: var(--sk-text-muted);
  font-size: 11px;
  font-weight: 700;
}
.sk-section-chevron {
  margin-left: auto;
  color: var(--sk-text-caption);
  font-size: 11px;
}

/* ─── Labels block (chip cloud) ─── */
.labels-container {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-sm);
  margin-bottom: var(--sk-space-sm);
}
.label-tag {
  font-size: 12px;
  padding: 3px 10px;
  height: auto;
  line-height: 1.5;
  font-weight: 500;
  border-radius: 12px;
  background: var(--sk-bg-soft);
  border-color: var(--sk-border);
  color: var(--sk-text-secondary);
}

/* ─── kubectl shortcut block ─── */
.kubectl-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 10px 14px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sk-space-sm);
}
.copy-btn { color: #a8d8a8 !important; font-size: 12px; }

/* ─── K8s sub-tables (Pods / Services / etc.) ─── */
.detail-table {
  margin-top: var(--sk-space-sm);
  border: 1px solid var(--sk-border);
  border-radius: 4px;
  overflow: hidden;
}
.detail-table-header {
  display: grid;
  padding: 6px 12px;
  background: var(--sk-bg-soft);
  border-bottom: 1px solid var(--sk-border);
}
.detail-table-row {
  display: grid;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--sk-text-secondary);
  border-top: 1px solid var(--sk-border-light);
}
.detail-table-row:first-of-type { border-top: 0; }
.detail-table-2col { grid-template-columns: 1fr 100px; }
.detail-table-3col { grid-template-columns: 1fr 100px 100px; }
.detail-table-4col { grid-template-columns: 1fr 80px 60px 70px; }
.resource-name {
  font-weight: 500;
  word-break: break-all;
  color: var(--sk-text);
}
.sk-status-ok   { color: var(--sk-status-success); font-weight: 500; }
.sk-status-warn { color: var(--sk-status-warning); font-weight: 500; }

/* ─── Sticky bottom action bar (same pattern as ActionDetailDrawer) ─── */
.sk-drawer-footer {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding: 12px var(--sk-drawer-padding-x);
  background: var(--sk-bg-page);
  border-top: 1px solid var(--sk-border);
}
.sk-drawer-footer-spacer { flex: 1; }
</style>
