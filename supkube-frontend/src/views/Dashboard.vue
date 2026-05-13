<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.namespaces }}</div>
            <div class="stat-label">Namespaces</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.backups }}</div>
            <div class="stat-label">Total Backups</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat">
            <div class="stat-value success">{{ stats.successful }}</div>
            <div class="stat-label">Successful</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat">
            <div class="stat-value danger">{{ stats.failed }}</div>
            <div class="stat-label">Failed</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>Recent Backups</span>
          <el-button type="primary" size="small" @click="$router.push('/backups')">
            View All
          </el-button>
        </div>
      </template>
      <el-table :data="recentBackups" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" />
        <el-table-column prop="metadata.namespace" label="Namespace" />
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status?.phase)">
              {{ row.status?.phase || 'Unknown' }}
            </el-tag>
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
      </el-table>
    </el-card>

    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>Recent Restores</span>
          <el-button type="primary" size="small" @click="$router.push('/restores')">
            View All
          </el-button>
        </div>
      </template>
      <el-table :data="recentRestores" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" />
        <el-table-column prop="spec.backupName" label="From Backup" />
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status?.phase)">
              {{ row.status?.phase || 'Unknown' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getBackups, getRestores, getNamespaces } from '../api/velero'

const stats = ref({ namespaces: 0, backups: 0, successful: 0, failed: 0 })
const recentBackups = ref([])
const recentRestores = ref([])
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

onMounted(async () => {
  loading.value = true
  try {
    const [backupsRes, restoresRes, nsRes] = await Promise.all([
      getBackups(),
      getRestores(),
      getNamespaces()
    ])

    const items = backupsRes.data.items || []
    recentBackups.value = items.slice(0, 5)
    stats.value.backups = items.length
    stats.value.successful = items.filter(b => b.status?.phase === 'Completed').length
    stats.value.failed = items.filter(b => b.status?.phase === 'Failed').length

    const restoreItems = restoresRes.data.items || []
    recentRestores.value = restoreItems.slice(0, 5)

    const namespaces = nsRes.data.items || nsRes.data || []
    stats.value.namespaces = Array.isArray(namespaces) ? namespaces.length : 0
  } catch (e) {
    console.error('Failed to load dashboard data:', e)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.dashboard { padding: 0; }
.stat { text-align: center; padding: 10px; }
.stat-value { font-size: 2em; font-weight: bold; }
.stat-label { color: #666; margin-top: 5px; }
.success { color: #67c23a; }
.danger { color: #f56c6c; }
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
