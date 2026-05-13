<template>
  <div class="policies-page">
    <div class="page-header">
      <h3>Backup Policies (Schedules)</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        Create Policy
      </el-button>
    </div>

    <el-card>
      <el-table :data="schedules" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column label="Namespaces">
          <template #default="{ row }">
            {{ (row.spec?.template?.includedNamespaces || ['*']).join(', ') }}
          </template>
        </el-table-column>
        <el-table-column label="Schedule">
          <template #default="{ row }">
            <code>{{ row.spec?.schedule || '-' }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="row.spec?.paused ? 'warning' : 'success'">
              {{ row.spec?.paused ? 'Paused' : 'Active' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Last Backup">
          <template #default="{ row }">
            {{ formatTime(row.status?.lastBackup) }}
          </template>
        </el-table-column>
        <el-table-column label="TTL">
          <template #default="{ row }">
            {{ row.spec?.template?.ttl || '720h' }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="250">
          <template #default="{ row }">
            <el-button
              size="small"
              :type="row.spec?.paused ? 'success' : 'warning'"
              @click="togglePause(row)"
            >
              {{ row.spec?.paused ? 'Resume' : 'Pause' }}
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Policy Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Backup Policy" width="550px">
      <el-form :model="createForm" label-width="160px">
        <el-form-item label="Policy Name" required>
          <el-input v-model="createForm.name" placeholder="daily-backup" />
        </el-form-item>
        <el-form-item label="Schedule" required>
          <el-select v-model="createForm.schedulePreset" @change="onPresetChange" style="width: 50%; margin-right: 8px">
            <el-option label="Every day at midnight" value="0 0 * * *" />
            <el-option label="Every 6 hours" value="0 */6 * * *" />
            <el-option label="Every 12 hours" value="0 */12 * * *" />
            <el-option label="Every week (Sunday)" value="0 0 * * 0" />
            <el-option label="Every month (1st)" value="0 0 1 * *" />
            <el-option label="Custom" value="custom" />
          </el-select>
          <el-input
            v-if="createForm.schedulePreset === 'custom'"
            v-model="createForm.schedule"
            placeholder="0 0 * * *"
            style="width: 45%"
          />
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
        <el-form-item label="TTL (Retention)">
          <el-select v-model="createForm.ttl" style="width: 100%">
            <el-option label="7 days" value="168h" />
            <el-option label="14 days" value="336h" />
            <el-option label="30 days (default)" value="720h" />
            <el-option label="60 days" value="1440h" />
            <el-option label="90 days" value="2160h" />
          </el-select>
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
import { ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { getSchedules, createSchedule, patchSchedule, deleteSchedule, getNamespaces } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const schedules = ref([])
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)

const createForm = ref({
  name: '',
  schedulePreset: '0 0 * * *',
  schedule: '0 0 * * *',
  includedNamespaces: [],
  ttl: '720h',
  storageLocation: 'default',
  snapshotVolumes: true
})

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const onPresetChange = (val) => {
  if (val !== 'custom') {
    createForm.value.schedule = val
  }
}

const fetchSchedules = async () => {
  loading.value = true
  try {
    const res = await getSchedules()
    schedules.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load policies')
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

const handleCreate = async () => {
  if (!createForm.value.name || !createForm.value.schedule) {
    ElMessage.warning('Please fill in policy name and schedule')
    return
  }
  creating.value = true
  try {
    const payload = {
      name: createForm.value.name,
      schedule: createForm.value.schedule,
      includedNamespaces: createForm.value.includedNamespaces.length > 0
        ? createForm.value.includedNamespaces : undefined,
      ttl: createForm.value.ttl || '720h',
      storageLocation: createForm.value.storageLocation || 'default',
      snapshotVolumes: createForm.value.snapshotVolumes
    }
    await createSchedule(payload)
    ElMessage.success(`Policy "${createForm.value.name}" created`)
    showCreateDialog.value = false
    createForm.value = {
      name: '',
      schedulePreset: '0 0 * * *',
      schedule: '0 0 * * *',
      includedNamespaces: [],
      ttl: '720h',
      storageLocation: 'default',
      snapshotVolumes: true
    }
    await fetchSchedules()
  } catch (e) {
    ElMessage.error('Failed to create policy: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const togglePause = async (row) => {
  const newPaused = !row.spec?.paused
  try {
    await patchSchedule(row.metadata.name, { paused: newPaused })
    ElMessage.success(`Policy "${row.metadata.name}" ${newPaused ? 'paused' : 'resumed'}`)
    await fetchSchedules()
  } catch (e) {
    ElMessage.error('Failed to update policy: ' + (e.response?.data?.error || e.message))
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete policy "${row.metadata.name}"?`,
      'Delete Policy',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteSchedule(row.metadata.name)
    ElMessage.success(`Policy "${row.metadata.name}" deleted`)
    await fetchSchedules()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete policy')
    }
  }
}

onMounted(() => {
  fetchSchedules()
  fetchNamespaces()
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
