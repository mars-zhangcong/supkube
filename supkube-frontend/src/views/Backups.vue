<template>
  <div class="restore-points-page">
    <div class="page-header">
      <h3>Restore Points</h3>
      <p class="page-desc">View and manage all Restore Points created in this cluster</p>
    </div>

    <!-- Application type pills (Kasten parity; only Namespace enabled for now) -->
    <div class="apptype-section">
      <div class="apptype-label">Application type</div>
      <div class="apptype-pills">
        <button class="apptype-pill is-active" type="button">
          <el-icon><Box /></el-icon> Namespace
        </button>
        <button class="apptype-pill is-disabled" type="button" disabled title="Coming in v0.7+">
          <el-icon><Monitor /></el-icon> Virtual Machine
        </button>
      </div>
    </div>

    <!-- Filter / search / bulk toolbar -->
    <div class="filter-toolbar">
      <el-select v-model="typeFilter" class="filter-type">
        <el-option label="All Types" value="all" />
        <el-option label="Snapshot (manual)" value="Snapshot" />
        <el-option label="Scheduled" value="Scheduled" />
      </el-select>
      <el-input v-model="nameFilter" placeholder="Filter by namespace or name" clearable class="filter-name">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="filter-spacer"></span>
      <span class="filter-summary">
        Viewing <strong>{{ filteredBackups.length }}</strong> out of {{ backups.length }} Restore Points
      </span>
      <el-button
        :disabled="selectedRows.length === 0"
        :type="selectedRows.length === 0 ? '' : 'danger'"
        @click="handleDeleteSelected"
      >
        Delete Selected ({{ selectedRows.length }})
      </el-button>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon> Create Restore Point
      </el-button>
    </div>

    <el-card>
      <el-table
        :data="filteredBackups"
        style="width: 100%"
        v-loading="loading"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="48" />

        <el-table-column label="Namespace" min-width="240" sortable>
          <template #default="{ row }">
            <div class="ns-cell">
              <span class="ns-name">{{ formatNamespace(row) }}</span>
              <span class="rp-name">{{ row.metadata?.name }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="Type" width="150">
          <template #default="{ row }">
            <span class="type-chip" :class="`type-${backupType(row).toLowerCase()}`">
              {{ backupType(row) === 'Snapshot' ? '📸' : '⏰' }} {{ backupType(row) }}
            </span>
          </template>
        </el-table-column>

        <el-table-column label="Policy" min-width="180">
          <template #default="{ row }">
            <span v-if="policyOf(row)" class="policy-link" @click="goToPolicy(row)">
              {{ policyOf(row) }}
            </span>
            <span v-else class="muted">(manual)</span>
          </template>
        </el-table-column>

        <el-table-column label="Profile" min-width="140">
          <template #default="{ row }">
            <span v-if="row.spec?.storageLocation" class="profile-cell">
              🗄 {{ row.spec.storageLocation }}
            </span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <el-table-column label="Status" width="170">
          <template #default="{ row }">
            <el-tag :type="phaseTagType(row.status?.phase)" size="small">
              {{ normalizePhase(row.status?.phase) }}
            </el-tag>
            <span v-if="row.status?.progress?.totalItems" class="items-mini">
              {{ row.status.progress.itemsBackedUp ?? '-' }}/{{ row.status.progress.totalItems }}
            </span>
          </template>
        </el-table-column>

        <el-table-column label="Created At" min-width="180" sortable :sort-method="sortByCreated">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>

        <el-table-column label="Expires At" min-width="160">
          <template #default="{ row }">
            <span v-if="row.status?.expiration">{{ formatTime(row.status.expiration) }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <el-table-column label="" width="60" align="right">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="cmd => handleCommand(cmd, row)">
              <el-button class="more-btn" text>
                <span class="dots">⋮</span>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="view">View</el-dropdown-item>
                  <el-dropdown-item command="restore">Restore</el-dropdown-item>
                  <el-dropdown-item command="validate">Validate</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>Delete</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Restore Point Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Restore Point" width="500px">
      <el-form :model="createForm" label-width="180px">
        <el-form-item label="Name" required>
          <el-input v-model="createForm.name" placeholder="my-restore-point" />
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All namespaces (default)"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>
        <el-form-item label="Excluded Namespaces">
          <el-select
            v-model="createForm.excludedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="None"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>
        <el-form-item label="Label Selector">
          <el-input v-model="createForm.labelSelectorStr" placeholder="app=mysql,env=prod" />
          <span class="form-hint">Comma-separated key=value pairs to filter resources</span>
        </el-form-item>
        <el-form-item label="TTL (Retention)">
          <el-input v-model="createForm.ttl" placeholder="720h (30 days)" />
        </el-form-item>
        <el-form-item label="Storage Location (Profile)">
          <el-input v-model="createForm.storageLocation" placeholder="default" />
        </el-form-item>
        <el-form-item label="Include Volumes">
          <el-switch v-model="createForm.snapshotVolumes" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">Create</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Search, Box, Monitor } from '@element-plus/icons-vue'
import { getBackups, createBackup, deleteBackup, getNamespaces } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'
import { normalizePhase, phaseTagType } from '../utils/phase'

const router = useRouter()
const backups = ref([])
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)

// Filter / selection state
const typeFilter = ref('all')
const nameFilter = ref('')
const selectedRows = ref([])

let pollTimer = null

const createForm = ref({
  name: '',
  includedNamespaces: [],
  excludedNamespaces: [],
  labelSelectorStr: '',
  ttl: '720h',
  storageLocation: 'default',
  snapshotVolumes: true
})

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

// A Velero Backup with the `velero.io/schedule-name` label was created by a
// Schedule (policy). Anything else is a manual / ad-hoc snapshot.
const backupType = (row) => {
  const scheduleName = row?.metadata?.labels?.['velero.io/schedule-name']
  return scheduleName ? 'Scheduled' : 'Snapshot'
}
const policyOf = (row) => row?.metadata?.labels?.['velero.io/schedule-name'] || ''

// Restore Point row primarily represents a namespace's protected state at a
// point in time. Show the most informative namespace label; * = all.
const formatNamespace = (row) => {
  const ns = row?.spec?.includedNamespaces || []
  if (ns.length === 0) return '*'
  if (ns.length === 1) return ns[0]
  return ns.join(', ')
}

const filteredBackups = computed(() => {
  const name = nameFilter.value.trim().toLowerCase()
  return backups.value.filter((row) => {
    if (typeFilter.value !== 'all' && backupType(row) !== typeFilter.value) return false
    if (name) {
      const haystack = [
        row.metadata?.name,
        formatNamespace(row),
        policyOf(row)
      ].filter(Boolean).join(' ').toLowerCase()
      if (!haystack.includes(name)) return false
    }
    return true
  })
})

const sortByCreated = (a, b) => {
  const at = new Date(a.metadata?.creationTimestamp || 0).getTime()
  const bt = new Date(b.metadata?.creationTimestamp || 0).getTime()
  return at - bt
}

const fetchBackups = async () => {
  loading.value = true
  try {
    const res = await getBackups()
    backups.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load restore points')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchNamespaces = async () => {
  try {
    const res = await getNamespaces()
    const items = res.data.namespaces || res.data.items || res.data || []
    namespaces.value = items.map(ns => ns.metadata?.name || ns).filter(Boolean)
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

const parseLabelSelector = (str) => {
  if (!str || !str.trim()) return undefined
  const labels = {}
  str.split(',').forEach(pair => {
    const [key, value] = pair.trim().split('=')
    if (key && value) labels[key.trim()] = value.trim()
  })
  return Object.keys(labels).length > 0 ? labels : undefined
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(fetchBackups, 5000)
}
const stopPolling = () => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

const handleCreate = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Please enter a name')
    return
  }
  creating.value = true
  try {
    const payload = {
      name: createForm.value.name,
      includedNamespaces: createForm.value.includedNamespaces.length > 0 ? createForm.value.includedNamespaces : undefined,
      excludedNamespaces: createForm.value.excludedNamespaces.length > 0 ? createForm.value.excludedNamespaces : undefined,
      labelSelector: parseLabelSelector(createForm.value.labelSelectorStr),
      ttl: createForm.value.ttl || '720h',
      storageLocation: createForm.value.storageLocation || 'default',
      snapshotVolumes: createForm.value.snapshotVolumes
    }
    await createBackup(payload)
    ElMessage.success(`Restore point "${createForm.value.name}" created. Monitoring progress...`)
    showCreateDialog.value = false
    createForm.value = {
      name: '', includedNamespaces: [], excludedNamespaces: [], labelSelectorStr: '',
      ttl: '720h', storageLocation: 'default', snapshotVolumes: true
    }
    await fetchBackups()
    startPolling()
  } catch (e) {
    ElMessage.error('Failed to create restore point: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const handleCommand = (cmd, row) => {
  switch (cmd) {
    case 'view': viewDetail(row); break
    case 'restore': restoreFromBackup(row); break
    case 'validate': handleValidate(row); break
    case 'delete': handleDelete(row); break
  }
}

const handleDelete = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      `Delete restore point "${name}"? Backup data in object storage is also removed via Velero's standard delete flow.`,
      'Delete Restore Point',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
  } catch { return }
  try {
    await deleteBackup(name)
    ElMessage.success(`Restore point "${name}" deleted`)
    await fetchBackups()
  } catch (e) {
    ElMessage.error('Failed to delete: ' + (e.response?.data?.error || e.message))
  }
}

const handleDeleteSelected = async () => {
  const rows = selectedRows.value.slice()
  if (rows.length === 0) return
  try {
    await ElMessageBox.confirm(
      `Delete ${rows.length} restore point${rows.length > 1 ? 's' : ''}?`,
      'Bulk Delete',
      { confirmButtonText: `Delete ${rows.length}`, cancelButtonText: 'Cancel', type: 'warning' }
    )
  } catch { return }
  let okCount = 0, failCount = 0
  for (const row of rows) {
    try {
      await deleteBackup(row.metadata.name)
      okCount++
    } catch (e) {
      failCount++
      console.error(`Failed to delete ${row.metadata.name}:`, e)
    }
  }
  if (failCount === 0) {
    ElMessage.success(`Deleted ${okCount} restore point${okCount > 1 ? 's' : ''}`)
  } else {
    ElMessage.warning(`Deleted ${okCount}, failed ${failCount}`)
  }
  await fetchBackups()
}

// Validate: a "dry-run" integrity check. v0.6 will invoke a real
// Velero BackupDescribe + DownloadRequest cycle; for now we just confirm
// the underlying Backup CR is still Completed and surface that to the user.
const handleValidate = (row) => {
  const phase = row?.status?.phase
  if (phase === 'Completed') {
    ElMessage.success(`Restore point "${row.metadata.name}" looks valid (phase=Completed). Full integrity check coming in v0.6.`)
  } else {
    ElMessage.warning(`Phase is ${phase || 'Unknown'} — may not be restorable. Full validate coming in v0.6.`)
  }
}

const handleSelectionChange = (rows) => {
  selectedRows.value = rows
}

const restoreFromBackup = (row) => {
  router.push({ path: '/restores', query: { backup: row.metadata.name } })
}

const viewDetail = (row) => {
  router.push({ path: `/backups/${row.metadata.name}` })
}

const goToPolicy = (row) => {
  const policy = policyOf(row)
  if (policy) router.push({ path: '/policies', query: { name: policy } })
}

onMounted(() => {
  fetchBackups()
  fetchNamespaces()
})
onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page-header { margin-bottom: 20px; }
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

/* Application type pills */
.apptype-section {
  margin-bottom: 18px;
}
.apptype-label {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}
.apptype-pills {
  display: flex;
  gap: 10px;
}
.apptype-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  cursor: pointer;
  transition: all 0.15s;
}
.apptype-pill:hover:not(.is-disabled):not(.is-active) {
  border-color: #c0c4cc;
  background: #f5f7fa;
}
.apptype-pill.is-active {
  background: #4f46e5;
  border-color: #4f46e5;
  color: #ffffff;
}
.apptype-pill.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Filter toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.filter-type { width: 180px; }
.filter-name { width: 280px; }
.filter-spacer { flex: 1; }
.filter-summary {
  color: #606266;
  font-size: 13px;
}
.filter-summary strong { color: #303133; font-weight: 600; }

/* Form helper */
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

/* Table cells */
.ns-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.ns-name {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}
.rp-name {
  font-size: 11px;
  color: #909399;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.type-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
}
.type-snapshot { color: #409eff; }
.type-scheduled { color: #67c23a; }
.policy-link {
  color: #409eff;
  cursor: pointer;
  font-size: 13px;
}
.policy-link:hover { text-decoration: underline; }
.profile-cell {
  font-size: 13px;
  color: #606266;
}
.items-mini {
  margin-left: 6px;
  font-size: 11px;
  color: #909399;
  font-family: 'SF Mono', Menlo, monospace;
}
.muted { color: #c0c4cc; font-size: 13px; }

/* Kebab action button */
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }
</style>
