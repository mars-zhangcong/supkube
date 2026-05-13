<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.nodes }}</div>
            <div class="stat-label">Nodes</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.namespaces }}</div>
            <div class="stat-label">Namespaces</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value success">{{ stats.protectedApps }}</div>
            <div class="stat-label">Protected</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.backups }}</div>
            <div class="stat-label">Total Backups</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value success">{{ stats.successful }}</div>
            <div class="stat-label">Successful</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
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
        <el-table-column label="Name">
          <template #default="{ row }">
            {{ row.name || row.metadata?.name || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Namespace">
          <template #default="{ row }">
            {{ row.namespace || row.metadata?.namespace || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="statusType(row.phase || row.status?.phase)">
              {{ row.phase || row.status?.phase || 'Unknown' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.createdAt || row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
        <el-table-column label="Expires">
          <template #default="{ row }">
            {{ formatTime(row.expiration || row.status?.expiration) }}
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
import { ref, onMounted, onUnmounted } from 'vue'
import { getDashboardSummary, getBackups, getRestores, getNamespaces, getApplications } from '../api/velero'

const stats = ref({ nodes: 0, namespaces: 0, protectedApps: 0, backups: 0, successful: 0, failed: 0 })
const recentBackups = ref([])
const recentRestores = ref([])
const loading = ref(false)
let refreshTimer = null

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

const fetchData = async () => {
  loading.value = true
  try {
    // Try the aggregated summary endpoint first
    const summaryRes = await getDashboardSummary()
    const data = summaryRes.data
    stats.value.nodes = data.cluster?.nodes ?? 0
    stats.value.namespaces = data.cluster?.namespaces ?? 0
    stats.value.backups = data.backupSummary?.total ?? 0
    stats.value.successful = data.backupSummary?.completed ?? 0
    stats.value.failed = data.backupSummary?.failed ?? 0
    recentBackups.value = (data.recentBackups || []).slice(0, 5)

    // Still fetch restores and applications separately (not in summary)
    const [restoresRes, appsRes] = await Promise.all([
      getRestores(),
      getApplications().catch(() => null)
    ])
    recentRestores.value = (restoresRes.data.items || []).slice(0, 5)
    if (appsRes) {
      const apps = appsRes.data.items || []
      stats.value.protectedApps = apps.filter(a => a.protected).length
    }
  } catch (e) {
    // Fallback to individual endpoints if summary not available
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

      const namespaces = nsRes.data.namespaces || nsRes.data.items || nsRes.data || []
      stats.value.namespaces = Array.isArray(namespaces) ? namespaces.length : 0
    } catch (fallbackErr) {
      console.error('Failed to load dashboard data:', fallbackErr)
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  refreshTimer = setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
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
