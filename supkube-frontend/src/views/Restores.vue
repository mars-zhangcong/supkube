<template>
  <div class="restores-page">
    <div class="page-header">
      <h3>Restores</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><RefreshRight /></el-icon>
        Create Restore
      </el-button>
    </div>

    <el-card>
      <el-table :data="restores" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column prop="spec.backupName" label="From Backup" />
        <el-table-column label="Included Namespaces">
          <template #default="{ row }">
            {{ (row.spec?.includedNamespaces || ['*']).join(', ') }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status?.phase)">
              {{ row.status?.phase || 'Unknown' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Items Restored">
          <template #default="{ row }">
            {{ row.status?.progress?.itemsRestored ?? '-' }} / {{ row.status?.progress?.totalItems ?? '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Restore Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Restore" width="500px">
      <el-form :model="createForm" label-width="180px">
        <el-form-item label="Restore Name" required>
          <el-input v-model="createForm.name" placeholder="my-restore" />
        </el-form-item>
        <el-form-item label="Source Backup" required>
          <el-select
            v-model="createForm.backupName"
            filterable
            placeholder="Select a backup"
            style="width: 100%"
          >
            <el-option
              v-for="b in availableBackups"
              :key="b.metadata.name"
              :label="b.metadata.name"
              :value="b.metadata.name"
            >
              <span>{{ b.metadata.name }}</span>
              <el-tag size="small" :type="statusType(b.status?.phase)" style="margin-left: 8px">
                {{ b.status?.phase }}
              </el-tag>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All from backup (default)"
          >
            <el-option
              v-for="ns in namespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Namespace Mapping">
          <div v-for="(mapping, idx) in createForm.namespaceMappings" :key="idx" class="ns-mapping-row">
            <el-input v-model="mapping.from" placeholder="Source NS" style="width: 40%" />
            <span style="margin: 0 8px">→</span>
            <el-input v-model="mapping.to" placeholder="Target NS" style="width: 40%" />
            <el-button type="danger" size="small" @click="removeMapping(idx)" circle>
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
          <el-button size="small" @click="addMapping">+ Add Mapping</el-button>
        </el-form-item>
        <el-form-item label="Restore PVs">
          <el-switch v-model="createForm.restorePVs" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">
          Restore
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshRight, Close } from '@element-plus/icons-vue'
import { getRestores, createRestore, getBackups, getNamespaces, getBackupResources } from '../api/velero'
import { ElMessage } from 'element-plus'

const route = useRoute()
const restores = ref([])
const availableBackups = ref([])
const backupResources = ref(null)
const loadingResources = ref(false)
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
let pollTimer = null

const createForm = ref({
  name: '',
  backupName: '',
  includedNamespaces: [],
  namespaceMappings: [],
  restorePVs: true
})

const statusType = (phase) => {
  const map = {
    Completed: 'success',
    InProgress: 'warning',
    Failed: 'danger',
    PartiallyFailed: 'warning'
  }
  return map[phase] || 'info'
}

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const addMapping = () => {
  createForm.value.namespaceMappings.push({ from: '', to: '' })
}

const removeMapping = (idx) => {
  createForm.value.namespaceMappings.splice(idx, 1)
}

const fetchRestores = async () => {
  loading.value = true
  try {
    const res = await getRestores()
    restores.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load restores')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchBackups = async () => {
  try {
    const res = await getBackups()
    const items = res.data.items || []
    availableBackups.value = items.filter(b => b.status?.phase === 'Completed')
  } catch (e) {
    console.error('Failed to load backups:', e)
  }
}

const fetchResourcePreview = async (backupName) => {
  if (!backupName) {
    backupResources.value = null
    return
  }
  loadingResources.value = true
  try {
    const res = await getBackupResources(backupName)
    backupResources.value = res.data
  } catch (e) {
    backupResources.value = null
    console.error('Failed to load resource preview:', e)
  } finally {
    loadingResources.value = false
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

const handleCreate = async () => {
  if (!createForm.value.name || !createForm.value.backupName) {
    ElMessage.warning('Please fill in restore name and select a backup')
    return
  }
  creating.value = true
  try {
    const nsMappings = {}
    createForm.value.namespaceMappings.forEach(m => {
      if (m.from && m.to) nsMappings[m.from] = m.to
    })

    const payload = {
      name: createForm.value.name,
      backupName: createForm.value.backupName,
      includedNamespaces: createForm.value.includedNamespaces.length > 0
        ? createForm.value.includedNamespaces : undefined,
      namespaceMapping: Object.keys(nsMappings).length > 0 ? nsMappings : undefined,
      restorePVs: createForm.value.restorePVs
    }
    await createRestore(payload)
    ElMessage.success(`Restore "${createForm.value.name}" created. Monitoring progress...`)
    showCreateDialog.value = false
    createForm.value = {
      name: '',
      backupName: '',
      includedNamespaces: [],
      namespaceMappings: [],
      restorePVs: true
    }
    await fetchRestores()
    startPolling()
  } catch (e) {
    ElMessage.error('Failed to create restore: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(fetchRestores, 5000)
}

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  fetchRestores()
  fetchBackups()
  fetchNamespaces()

  // Pre-fill backup name if navigated from Backups page
  if (route.query.backup) {
    createForm.value.backupName = route.query.backup
    showCreateDialog.value = true
  }
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
.ns-mapping-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
</style>
