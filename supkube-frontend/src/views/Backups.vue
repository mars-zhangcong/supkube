<template>
  <div class="backups-page">
    <div class="page-header">
      <h3>Backups</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        Create Backup
      </el-button>
    </div>

    <el-card>
      <el-table :data="backups" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column label="Included Namespaces">
          <template #default="{ row }">
            {{ (row.spec?.includedNamespaces || ['*']).join(', ') }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="phaseTagType(row.status?.phase)">
              {{ normalizePhase(row.status?.phase) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Items">
          <template #default="{ row }">
            {{ row.status?.progress?.itemsBackedUp ?? '-' }} / {{ row.status?.progress?.totalItems ?? '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
        <el-table-column label="Expires">
          <template #default="{ row }">
            {{ formatTime(row.status?.expiration) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="250">
          <template #default="{ row }">
            <el-button size="small" @click="viewDetail(row)">
              View
            </el-button>
            <el-button size="small" @click="restoreFromBackup(row)">
              Restore
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Backup Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Backup" width="500px">
      <el-form :model="createForm" label-width="160px">
        <el-form-item label="Backup Name" required>
          <el-input v-model="createForm.name" placeholder="my-backup" />
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All namespaces (default)"
          >
            <el-option
              v-for="ns in namespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            />
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
            <el-option
              v-for="ns in namespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="TTL">
          <el-input v-model="createForm.ttl" placeholder="720h (30 days)" />
        </el-form-item>
        <el-form-item label="Label Selector">
          <el-input v-model="createForm.labelSelectorStr" placeholder="app=nginx, env=prod" />
          <span style="font-size: 12px; color: #999">Comma-separated key=value pairs to filter resources</span>
        </el-form-item>
        <el-form-item label="Storage Location">
          <el-input v-model="createForm.storageLocation" placeholder="default" />
        </el-form-item>
        <el-form-item label="Include Volumes">
          <el-switch v-model="createForm.snapshotVolumes" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">
          Create
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import { getBackups, createBackup, deleteBackup, getNamespaces } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'
import { normalizePhase, phaseTagType } from '../utils/phase'

const router = useRouter()
const backups = ref([])
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
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

const fetchBackups = async () => {
  loading.value = true
  try {
    const res = await getBackups()
    backups.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load backups')
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
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

const handleCreate = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Please enter a backup name')
    return
  }
  creating.value = true
  try {
    const payload = {
      name: createForm.value.name,
      includedNamespaces: createForm.value.includedNamespaces.length > 0
        ? createForm.value.includedNamespaces : undefined,
      excludedNamespaces: createForm.value.excludedNamespaces.length > 0
        ? createForm.value.excludedNamespaces : undefined,
      labelSelector: parseLabelSelector(createForm.value.labelSelectorStr),
      ttl: createForm.value.ttl || '720h',
      storageLocation: createForm.value.storageLocation || 'default',
      snapshotVolumes: createForm.value.snapshotVolumes
    }
    await createBackup(payload)
    ElMessage.success(`Backup "${createForm.value.name}" created. Monitoring progress...`)
    showCreateDialog.value = false
    createForm.value = {
      name: '',
      includedNamespaces: [],
      excludedNamespaces: [],
      labelSelectorStr: '',
      ttl: '720h',
      storageLocation: 'default',
      snapshotVolumes: true
    }
    await fetchBackups()
    startPolling()
  } catch (e) {
    ElMessage.error('Failed to create backup: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete backup "${row.metadata.name}"?`,
      'Delete Backup',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteBackup(row.metadata.name)
    ElMessage.success(`Backup "${row.metadata.name}" deleted`)
    await fetchBackups()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete backup')
    }
  }
}

const restoreFromBackup = (row) => {
  router.push({ path: '/restores', query: { backup: row.metadata.name } })
}

const viewDetail = (row) => {
  router.push({ path: `/backups/${row.metadata.name}` })
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
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.page-header h3 {
  margin: 0;
}
</style>
