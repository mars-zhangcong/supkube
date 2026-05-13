<template>
  <div class="backup-detail">
    <div class="page-header">
      <el-button @click="$router.push('/backups')" text>
        ← Back to Backups
      </el-button>
      <h3>Backup: {{ backup?.metadata?.name }}</h3>
    </div>

    <el-row :gutter="20" v-loading="loading">
      <!-- Overview Card -->
      <el-col :span="12">
        <el-card>
          <template #header>Overview</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Name">
              {{ backup?.metadata?.name }}
            </el-descriptions-item>
            <el-descriptions-item label="Namespace">
              {{ backup?.metadata?.namespace }}
            </el-descriptions-item>
            <el-descriptions-item label="Status">
              <el-tag :type="statusType(backup?.status?.phase)">
                {{ backup?.status?.phase || 'Unknown' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Created">
              {{ formatTime(backup?.metadata?.creationTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Started">
              {{ formatTime(backup?.status?.startTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Completed">
              {{ formatTime(backup?.status?.completionTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Expires">
              {{ formatTime(backup?.status?.expiration) }}
            </el-descriptions-item>
            <el-descriptions-item label="Storage Location">
              {{ backup?.spec?.storageLocation || 'default' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>

      <!-- Progress & Spec Card -->
      <el-col :span="12">
        <el-card>
          <template #header>Backup Spec</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Included Namespaces">
              {{ (backup?.spec?.includedNamespaces || ['*']).join(', ') }}
            </el-descriptions-item>
            <el-descriptions-item label="Excluded Namespaces">
              {{ (backup?.spec?.excludedNamespaces || []).join(', ') || 'None' }}
            </el-descriptions-item>
            <el-descriptions-item label="TTL">
              {{ backup?.spec?.ttl || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Snapshot Volumes">
              {{ backup?.spec?.snapshotVolumes !== false ? 'Yes' : 'No' }}
            </el-descriptions-item>
            <el-descriptions-item label="Items Backed Up">
              {{ backup?.status?.progress?.itemsBackedUp ?? '-' }} / {{ backup?.status?.progress?.totalItems ?? '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Format Version">
              {{ backup?.status?.formatVersion || '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card style="margin-top: 20px">
          <template #header>Actions</template>
          <el-space>
            <el-button type="primary" @click="restoreFromBackup">
              Restore from this Backup
            </el-button>
            <el-button type="danger" @click="handleDelete">
              Delete Backup
            </el-button>
          </el-space>
        </el-card>
      </el-col>
    </el-row>

    <!-- Labels & Annotations -->
    <el-card style="margin-top: 20px" v-if="backup?.metadata?.labels">
      <template #header>Labels</template>
      <el-tag
        v-for="(value, key) in backup?.metadata?.labels"
        :key="key"
        style="margin-right: 8px; margin-bottom: 8px"
      >
        {{ key }}={{ value }}
      </el-tag>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getBackup, deleteBackup } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const backup = ref(null)
const loading = ref(false)

const statusType = (phase) => {
  const map = {
    Completed: 'success',
    InProgress: 'warning',
    Failed: 'danger',
    PartiallyFailed: 'warning',
    Deleting: 'info'
  }
  return map[phase] || 'info'
}

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const fetchBackup = async () => {
  loading.value = true
  try {
    const res = await getBackup(route.params.name)
    backup.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load backup details')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const restoreFromBackup = () => {
  router.push({ path: '/restores', query: { backup: backup.value?.metadata?.name } })
}

const handleDelete = async () => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete backup "${backup.value?.metadata?.name}"?`,
      'Delete Backup',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteBackup(backup.value.metadata.name)
    ElMessage.success('Backup deleted')
    router.push('/backups')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete backup')
    }
  }
}

onMounted(() => {
  fetchBackup()
})
</script>

<style scoped>
.page-header {
  margin-bottom: 16px;
}
.page-header h3 {
  margin: 8px 0 0 0;
}
</style>
