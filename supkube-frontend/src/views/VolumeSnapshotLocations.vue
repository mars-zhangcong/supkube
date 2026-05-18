<template>
  <div class="vsl-page">
    <div class="page-header">
      <h3>Snapshot Locations</h3>
      <p class="page-desc">
        Volume Snapshot Locations tell Velero how to take volume snapshots — CSI for in-cluster CSI drivers, or cloud-native (AWS EBS / GCP PD / Azure Disk) for managed storage.
      </p>
    </div>

    <div class="filter-toolbar">
      <span class="filter-spacer"></span>
      <span class="filter-summary">
        Viewing <strong>{{ locations.length }}</strong> snapshot location{{ locations.length === 1 ? '' : 's' }}
      </span>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon> Create Snapshot Location
      </el-button>
    </div>

    <el-card>
      <el-table :data="locations" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable min-width="200" />
        <el-table-column label="Provider" min-width="180">
          <template #default="{ row }">
            <span class="provider-chip">{{ row.spec?.provider || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Config" min-width="280">
          <template #default="{ row }">
            <span v-if="hasConfig(row)" class="config-pre">{{ formatConfig(row) }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="Created At" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
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
                  <el-dropdown-item command="details">View Details</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>Delete</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
      <p v-if="!loading && locations.length === 0" class="empty-hint">
        No Snapshot Locations yet. Create one to enable CSI / cloud-native volume snapshots.
      </p>
    </el-card>

    <!-- Details Drawer -->
    <el-drawer
      v-model="detailsVisible"
      title="Snapshot Location Details"
      direction="rtl"
      size="520px"
      :destroy-on-close="true"
      class="vsl-details-drawer"
    >
      <div v-if="selectedRow" class="details-body">
        <div class="details-title">
          <span class="vsl-icon">📸</span>
          <span>{{ selectedRow.metadata?.name }}</span>
        </div>
        <el-descriptions :column="1" border size="small" class="details-table">
          <el-descriptions-item label="Provider">{{ selectedRow.spec?.provider || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Created">{{ formatTime(selectedRow.metadata?.creationTimestamp) }}</el-descriptions-item>
          <el-descriptions-item label="Config">
            <pre v-if="hasConfig(selectedRow)" class="config-block">{{ JSON.stringify(selectedRow.spec.config, null, 2) }}</pre>
            <span v-else class="muted">no config (uses Velero defaults)</span>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-drawer>

    <!-- Create Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Snapshot Location" width="540px">
      <el-form :model="createForm" label-width="160px">
        <el-form-item label="Name" required :error="nameError">
          <el-input v-model="createForm.name" placeholder="csi-default" />
          <span class="form-hint">
            Lowercase letters, digits, '-' or '.'; e.g. <code>csi-default</code>, <code>aws-us-east</code>
          </span>
        </el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="createForm.provider" style="width: 100%" @change="onProviderChange">
            <el-option label="CSI (in-cluster CSI driver)" value="csi" />
            <el-option label="AWS EBS" value="aws" />
            <el-option label="GCP Persistent Disk" value="gcp" />
            <el-option label="Azure Disk" value="azure" />
          </el-select>
          <span class="form-hint">
            Choose <strong>CSI</strong> for csi-hostpath, OpenEBS, Longhorn, Rook-Ceph, etc. The actual driver is resolved via VolumeSnapshotClass at backup time.
          </span>
        </el-form-item>
        <el-form-item v-if="createForm.provider === 'aws' || createForm.provider === 'gcp' || createForm.provider === 'azure'" label="Region">
          <el-input v-model="createForm.region" :placeholder="regionPlaceholder" />
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
import { ref, computed, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import {
  getVolumeSnapshotLocations,
  getVolumeSnapshotLocation,
  createVolumeSnapshotLocation,
  deleteVolumeSnapshotLocation
} from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const locations = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const detailsVisible = ref(false)
const selectedRow = ref(null)

const createForm = ref({
  name: '',
  provider: 'csi',
  region: ''
})

const RFC1123_SUBDOMAIN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/
const nameError = computed(() => {
  const v = createForm.value.name
  if (!v) return ''
  if (v.length > 253) return 'Name too long (max 253 characters)'
  if (!RFC1123_SUBDOMAIN.test(v)) {
    return "Use lowercase letters/digits/'-'/'.'; must start and end with a letter or digit"
  }
  return ''
})

const regionPlaceholder = computed(() => {
  switch (createForm.value.provider) {
    case 'aws': return 'us-east-1'
    case 'gcp': return 'us-central1'
    case 'azure': return 'eastus'
    default: return ''
  }
})

const formatTime = (ts) => ts ? new Date(ts).toLocaleString() : '-'

const hasConfig = (row) =>
  row?.spec?.config && Object.keys(row.spec.config).length > 0

const formatConfig = (row) =>
  Object.entries(row.spec.config).map(([k, v]) => `${k}=${v}`).join(' · ')

const onProviderChange = (val) => {
  // Reset region when switching to/from CSI (CSI doesn't use region).
  if (val === 'csi') createForm.value.region = ''
}

const fetchLocations = async () => {
  loading.value = true
  try {
    const res = await getVolumeSnapshotLocations()
    locations.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load snapshot locations')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleCreate = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Name is required')
    return
  }
  if (nameError.value) {
    ElMessage.warning(nameError.value)
    return
  }
  creating.value = true
  try {
    const config = {}
    if (createForm.value.region) config.region = createForm.value.region
    const payload = {
      name: createForm.value.name,
      provider: createForm.value.provider,
      config: Object.keys(config).length > 0 ? config : undefined
    }
    await createVolumeSnapshotLocation(payload)
    ElMessage.success(`Snapshot location "${createForm.value.name}" created`)
    showCreateDialog.value = false
    createForm.value = { name: '', provider: 'csi', region: '' }
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to create: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const handleCommand = async (cmd, row) => {
  if (cmd === 'details') {
    try {
      const res = await getVolumeSnapshotLocation(row.metadata.name)
      selectedRow.value = res.data
    } catch (e) {
      selectedRow.value = row
    }
    detailsVisible.value = true
  } else if (cmd === 'delete') {
    handleDelete(row)
  }
}

const handleDelete = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      `Delete snapshot location "${name}"? Velero will refuse to delete if any Backup still references it.`,
      'Delete Snapshot Location',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
  } catch { return }
  try {
    await deleteVolumeSnapshotLocation(name)
    ElMessage.success(`Snapshot location "${name}" deleted`)
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to delete: ' + (e.response?.data?.error || e.message))
  }
}

onMounted(() => {
  fetchLocations()
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
  max-width: 720px;
}

.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.filter-spacer { flex: 1; }
.filter-summary { color: #606266; font-size: 13px; }
.filter-summary strong { color: #303133; font-weight: 600; }

.provider-chip {
  display: inline-block;
  padding: 2px 10px;
  background: #ecf5ff;
  color: #409eff;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  font-family: 'SF Mono', Menlo, monospace;
}
.config-pre {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
  color: #606266;
}
.muted { color: #c0c4cc; }
.empty-hint {
  text-align: center;
  color: #909399;
  font-size: 13px;
  padding: 16px 0 8px;
}

.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
.form-hint code {
  background: #f5f7fa;
  padding: 1px 4px;
  border-radius: 3px;
  font-family: 'SF Mono', Menlo, monospace;
}

.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }

:deep(.vsl-details-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 20px 24px;
  border-bottom: 1px solid #ebeef5;
}
:deep(.vsl-details-drawer .el-drawer__title) {
  font-size: 20px;
  font-weight: 700;
  color: #1f2329;
  text-align: center;
  width: 100%;
  letter-spacing: -0.01em;
}
:deep(.vsl-details-drawer .el-drawer__body) { padding: 24px; }

.details-body { padding: 0 4px; }
.details-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 22px;
  font-weight: 700;
  color: #1f2329;
  margin-bottom: 24px;
}
.vsl-icon { font-size: 24px; }
.config-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 12px 16px;
  border-radius: 6px;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
  margin: 0;
  overflow-x: auto;
}
</style>
