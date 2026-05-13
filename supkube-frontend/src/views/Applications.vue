<template>
  <div class="applications-page">
    <div class="page-header">
      <h3>Applications</h3>
      <p class="page-desc">View details or perform actions on applications.</p>
    </div>

    <el-card>
      <el-table :data="applications" style="width: 100%" v-loading="loading" row-class-name="app-row">
        <el-table-column prop="namespace" label="Name" sortable min-width="200">
          <template #default="{ row }">
            <span class="app-name">{{ row.namespace }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Status" width="180">
          <template #default="{ row }">
            <span class="status-badge" :class="row.protected ? 'status-compliant' : 'status-unmanaged'">
              <span class="status-icon">{{ row.protected ? '✓' : '!' }}</span>
              {{ row.protected ? 'Compliant' : 'Unmanaged' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Workloads" width="120">
          <template #default="{ row }">
            <span class="workload-count">⚙ {{ row.workloads }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Last Backup" min-width="220">
          <template #default="{ row }">
            <span v-if="row.lastBackupName" class="last-backup">
              {{ row.lastBackupName }}
              <span class="backup-time">{{ formatTime(row.lastBackupTime) }}</span>
            </span>
            <span v-else class="no-restore">No restore point</span>
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
                  <el-dropdown-item command="snapshot">Snapshot</el-dropdown-item>
                  <el-dropdown-item command="restore">Restore</el-dropdown-item>
                  <el-dropdown-item command="backup">Backup</el-dropdown-item>
                  <el-dropdown-item command="details">Details</el-dropdown-item>
                  <el-dropdown-item command="policy">Create a Policy</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getApplications } from '../api/velero'
import { ElMessage } from 'element-plus'

const router = useRouter()
const applications = ref([])
const loading = ref(false)

const formatTime = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
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
      ElMessage.info(`Namespace: ${row.namespace} — ${row.workloads} workload(s)`)
      break
  }
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
.status-compliant {
  color: #67c23a;
}
.status-compliant .status-icon {
  border: 2px solid #67c23a;
  color: #67c23a;
}
.status-unmanaged {
  color: #e6a23c;
}
.status-unmanaged .status-icon {
  border: 2px solid #e6a23c;
  color: #e6a23c;
}
.workload-count {
  color: #606266;
  font-size: 13px;
}
.last-backup {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.backup-time {
  color: #409eff;
  font-size: 12px;
}
.no-restore {
  color: #c0c4cc;
  font-size: 13px;
}
.more-btn {
  padding: 4px 8px;
  font-size: 18px;
  color: #606266;
}
.dots {
  font-size: 20px;
  line-height: 1;
  letter-spacing: 1px;
}
:deep(.app-row:hover .more-btn) {
  color: #409eff;
}
</style>
