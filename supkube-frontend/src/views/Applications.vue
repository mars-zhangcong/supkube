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
        <el-table-column prop="namespace" :label="t('common.name')" sortable min-width="200">
          <template #default="{ row }">
            <span class="app-name">{{ row.namespace }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="180">
          <template #default="{ row }">
            <span class="status-badge" :class="`status-${complianceClass(row)}`">
              <span class="status-icon">{{ complianceIcon(row) }}</span>
              {{ complianceLabel(row) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="t('applications.workloads')" width="120">
          <template #default="{ row }">
            <span class="workload-count">⚙ {{ row.workloads }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('applications.labels')" min-width="280">
          <template #default="{ row }">
            <div class="row-labels">
              <el-tooltip
                v-for="entry in rowLabelPreview(row)"
                :key="entry[0]"
                :content="`${entry[0]}:${entry[1]}`"
                placement="top"
                :show-after="300"
              >
                <el-tag
                  size="small"
                  effect="plain"
                  round
                  class="row-label-tag"
                >{{ entry[0] }}:{{ entry[1] }}</el-tag>
              </el-tooltip>
              <span v-if="rowLabelHidden(row) > 0" class="row-labels-more">
                +{{ rowLabelHidden(row) }} more
              </span>
              <span v-if="rowLabelCount(row) === 0" class="row-labels-empty">—</span>
            </div>
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

    <!-- Application Details Drawer -->
    <el-drawer
      v-model="drawerVisible"
      title="Application Details"
      direction="rtl"
      size="520px"
      :destroy-on-close="true"
      class="app-details-drawer"
    >
      <div v-if="selectedApp" class="app-details" v-loading="detailLoading">
        <!-- App Name with icon -->
        <div class="detail-app-name">
          <span class="app-icon-box">📦</span>
          <span>{{ selectedApp.namespace }}</span>
        </div>

        <!-- Labels -->
        <div class="detail-section labels-section">
          <div class="detail-section-title">LABELS</div>
          <div class="labels-container">
            <el-tag
              v-for="entry in visibleLabelEntries"
              :key="entry[0]"
              size="default"
              effect="plain"
              round
              class="label-tag"
            >{{ entry[0] }}:{{ entry[1] }}</el-tag>
            <span v-if="labelEntries.length === 0" class="no-data">No labels</span>
          </div>
          <a
            v-if="hiddenLabelCount > 0"
            class="labels-toggle"
            @click="labelsExpanded = !labelsExpanded"
          >
            {{ labelsExpanded ? 'Show fewer labels' : `Show ${hiddenLabelCount} more labels ...` }}
          </a>
        </div>

        <!-- kubectl -->
        <div class="detail-section">
          <div class="detail-section-title">kubectl</div>
          <div class="kubectl-block">
            <code>$ kubectl get all -n {{ selectedApp.namespace }}</code>
            <el-button size="small" text class="copy-btn" @click="copyKubectl">copy</el-button>
          </div>
        </div>

        <!-- Pods -->
        <div class="detail-section" v-if="detailData.pods">
          <div class="detail-section-title collapsible" @click="podsExpanded = !podsExpanded">
            Pods <span class="count">({{ detailData.pods.length }})</span>
            <span class="chevron">{{ podsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="podsExpanded" class="detail-table">
            <div class="detail-table-header detail-table-4col">
              <span>NAME</span>
              <span>STATUS</span>
              <span>READY</span>
              <span>RESTARTS</span>
            </div>
            <div v-if="detailData.pods.length > 0">
              <div v-for="pod in detailData.pods" :key="pod.name" class="detail-table-row detail-table-4col">
                <span class="resource-name">{{ pod.name }}</span>
                <span :class="pod.status === 'Running' ? 'status-ok' : 'status-warn'">{{ pod.status }}</span>
                <span>{{ pod.ready }}</span>
                <span>{{ pod.restarts || 0 }}</span>
              </div>
            </div>
            <div v-else class="no-data">No pods found</div>
          </div>
        </div>

        <!-- Services -->
        <div class="detail-section" v-if="detailData.services">
          <div class="detail-section-title collapsible" @click="servicesExpanded = !servicesExpanded">
            Services <span class="count">({{ detailData.services.length }})</span>
            <span class="chevron">{{ servicesExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="servicesExpanded" class="detail-table">
            <div class="detail-table-header detail-table-3col">
              <span>NAME</span>
              <span>TYPE</span>
              <span>PORTS</span>
            </div>
            <div v-if="detailData.services.length > 0">
              <div v-for="svc in detailData.services" :key="svc.name" class="detail-table-row detail-table-3col">
                <span class="resource-name">{{ svc.name }}</span>
                <span>{{ svc.type }}</span>
                <span>{{ svc.ports }}</span>
              </div>
            </div>
            <div v-else class="no-data">No services found</div>
          </div>
        </div>

        <!-- Deployments -->
        <div class="detail-section" v-if="detailData.deployments">
          <div class="detail-section-title collapsible" @click="deploymentsExpanded = !deploymentsExpanded">
            Deployments <span class="count">({{ detailData.deployments.length }})</span>
            <span class="chevron">{{ deploymentsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="deploymentsExpanded" class="detail-table">
            <div class="detail-table-header detail-table-3col">
              <span>NAME</span>
              <span>READY</span>
              <span>AVAILABLE</span>
            </div>
            <div v-if="detailData.deployments.length > 0">
              <div v-for="dep in detailData.deployments" :key="dep.name" class="detail-table-row detail-table-3col">
                <span class="resource-name">{{ dep.name }}</span>
                <span>{{ dep.ready }}</span>
                <span>{{ dep.available }}</span>
              </div>
            </div>
            <div v-else class="no-data">No deployments found</div>
          </div>
        </div>

        <!-- StatefulSets -->
        <div class="detail-section" v-if="detailData.statefulSets">
          <div class="detail-section-title collapsible" @click="statefulsetsExpanded = !statefulsetsExpanded">
            StatefulSets <span class="count">({{ detailData.statefulSets.length }})</span>
            <span class="chevron">{{ statefulsetsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="statefulsetsExpanded" class="detail-table">
            <div class="detail-table-header detail-table-2col">
              <span>NAME</span>
              <span>READY</span>
            </div>
            <div v-if="detailData.statefulSets.length > 0">
              <div v-for="sts in detailData.statefulSets" :key="sts.name" class="detail-table-row detail-table-2col">
                <span class="resource-name">{{ sts.name }}</span>
                <span>{{ sts.ready }}</span>
              </div>
            </div>
            <div v-else class="no-data">No statefulsets found</div>
          </div>
        </div>

        <!-- PVCs -->
        <div class="detail-section" v-if="detailData.pvcs">
          <div class="detail-section-title collapsible" @click="pvcsExpanded = !pvcsExpanded">
            PVCs <span class="count">({{ detailData.pvcs.length }})</span>
            <span class="chevron">{{ pvcsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="pvcsExpanded" class="detail-table">
            <div class="detail-table-header detail-table-3col">
              <span>NAME</span>
              <span>STATUS</span>
              <span>CAPACITY</span>
            </div>
            <div v-if="detailData.pvcs.length > 0">
              <div v-for="pvc in detailData.pvcs" :key="pvc.name" class="detail-table-row detail-table-3col">
                <span class="resource-name">📄 {{ pvc.name }}</span>
                <span :class="pvc.status === 'Bound' ? 'status-ok' : 'status-warn'">{{ pvc.status }}</span>
                <span>{{ pvc.capacity || pvc.size || '-' }}</span>
              </div>
            </div>
            <div v-else class="no-data">No PVCs found</div>
          </div>
        </div>

        <!-- ConfigMaps -->
        <div class="detail-section" v-if="detailData.configMaps">
          <div class="detail-section-title collapsible" @click="configmapsExpanded = !configmapsExpanded">
            ConfigMaps <span class="count">({{ detailData.configMaps.length }})</span>
            <span class="chevron">{{ configmapsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="configmapsExpanded" class="detail-table">
            <div class="detail-table-header detail-table-2col">
              <span>NAME</span>
              <span>DATA</span>
            </div>
            <div v-if="detailData.configMaps.length > 0">
              <div v-for="cm in detailData.configMaps" :key="cm.name" class="detail-table-row detail-table-2col">
                <span class="resource-name">{{ cm.name }}</span>
                <span>{{ cm.data || '-' }}</span>
              </div>
            </div>
            <div v-else class="no-data">No configmaps found</div>
          </div>
        </div>

        <!-- Actions -->
        <div class="detail-actions">
          <el-button type="primary" @click="handleCommand('backup', selectedApp)">Backup Now</el-button>
          <el-button @click="handleCommand('restore', selectedApp)">Restore</el-button>
          <el-button @click="handleCommand('policy', selectedApp)">Create Policy</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { getApplications, getApplicationDetail } from '../api/velero'
import { ElMessage } from 'element-plus'

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

// Expand/collapse states
const podsExpanded = ref(true)
const servicesExpanded = ref(true)
const deploymentsExpanded = ref(true)
const statefulsetsExpanded = ref(true)
const pvcsExpanded = ref(true)
const configmapsExpanded = ref(false)

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

const handleCommand = (command, row) => {
  switch (command) {
    case 'snapshot':
    case 'backup':
      router.push({ path: '/backups', query: { namespace: row.namespace } })
      break
    case 'restore':
      router.push({ path: '/restores', query: { namespace: row.namespace } })
      break
    case 'policy':
      router.push({ path: '/policies', query: { namespace: row.namespace } })
      break
    case 'details':
      openDetail(row)
      break
  }
}

const openDetail = async (row) => {
  selectedApp.value = row
  detailData.value = {}
  detailLoading.value = true
  drawerVisible.value = true
  // Reset expand states
  podsExpanded.value = true
  servicesExpanded.value = true
  deploymentsExpanded.value = true
  statefulsetsExpanded.value = true
  pvcsExpanded.value = true
  configmapsExpanded.value = false
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
.workload-count { color: #606266; font-size: 13px; }
.row-labels {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.row-label-tag {
  font-size: 11px;
  padding: 2px 10px;
  height: auto;
  line-height: 1.5;
  font-weight: 500;
  border-radius: 12px;
  background: #f5f7fa;
  border-color: #e4e7ed;
  color: #303133;
  max-width: 220px;
}
/* el-tag wraps its text in .el-tag__content; ellipsis must be applied there
 * to actually truncate, otherwise text overflows the tag border. */
.row-label-tag :deep(.el-tag__content),
.row-label-tag :deep(span) {
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
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

/* Drawer header — Kasten-style: centered, bold, black, large */
:deep(.app-details-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 20px 24px;
  border-bottom: 1px solid #ebeef5;
}
:deep(.app-details-drawer .el-drawer__title) {
  font-size: 20px;
  font-weight: 700;
  color: #1f2329;
  text-align: center;
  width: 100%;
  letter-spacing: -0.01em;
}
:deep(.app-details-drawer .el-drawer__close-btn) {
  font-size: 18px;
  color: #606266;
}
:deep(.app-details-drawer .el-drawer__body) {
  padding: 24px;
}

/* Drawer styles */
.app-details { padding: 0 4px; }
.detail-app-name {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 22px;
  font-weight: 700;
  color: #1f2329;
  margin-bottom: 28px;
  letter-spacing: -0.01em;
}
.app-icon-box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  font-size: 22px;
}
.detail-section { margin-bottom: 24px; }
.detail-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #909399;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 10px;
}
.detail-section-title.collapsible {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  text-transform: none;
  letter-spacing: 0;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}
.count { color: #909399; font-weight: 400; }
.chevron { margin-left: auto; color: #909399; }
.labels-section { border-bottom: 1px solid #f0f0f0; padding-bottom: 16px; }
.labels-container { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.label-tag {
  font-size: 12px;
  padding: 4px 12px;
  height: auto;
  line-height: 1.4;
  font-weight: 500;
  border-radius: 14px;
  background: #f5f7fa;
  border-color: #e4e7ed;
  color: #303133;
}
.labels-toggle {
  display: inline-block;
  margin-top: 4px;
  font-size: 13px;
  color: #409eff;
  cursor: pointer;
}
.labels-toggle:hover { text-decoration: underline; }
.kubectl-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 12px 16px;
  border-radius: 6px;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.copy-btn { color: #a8d8a8 !important; font-size: 12px; }
.detail-table { margin-top: 8px; }
.detail-table-header {
  display: grid;
  padding: 6px 12px;
  font-size: 11px;
  font-weight: 600;
  color: #909399;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid #f0f0f0;
}
.detail-table-row {
  display: grid;
  padding: 10px 12px;
  font-size: 13px;
  border-bottom: 1px solid #f9f9f9;
}
.detail-table-2col { grid-template-columns: 1fr 100px; }
.detail-table-3col { grid-template-columns: 1fr 100px 100px; }
.detail-table-4col { grid-template-columns: 1fr 80px 60px 70px; }
.resource-name { font-weight: 500; word-break: break-all; }
.status-ok { color: #67c23a; font-weight: 500; }
.status-warn { color: #e6a23c; font-weight: 500; }
.no-data { color: #c0c4cc; font-size: 13px; padding: 8px 0; }
.detail-actions {
  display: flex;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
  margin-top: 8px;
}
</style>
