<template>
  <div class="applications-page">
    <div class="page-header">
      <h3>Applications</h3>
    </div>

    <el-card>
      <el-table :data="applications" style="width: 100%" v-loading="loading">
        <el-table-column prop="namespace" label="Namespace" sortable />
        <el-table-column prop="workloads" label="Workloads" width="120" />
        <el-table-column label="Protection Status" width="180">
          <template #default="{ row }">
            <el-tag :type="row.protected ? 'success' : 'warning'">
              {{ row.protected ? 'Protected' : 'Unprotected' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Last Backup" width="200">
          <template #default="{ row }">
            {{ row.lastBackupName || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Last Backup Time">
          <template #default="{ row }">
            {{ formatTime(row.lastBackupTime) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="150">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="backupNamespace(row)">
              Backup
            </el-button>
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
  if (!ts) return '-'
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

const backupNamespace = (row) => {
  router.push({ path: '/backups', query: { namespace: row.namespace } })
}

onMounted(() => {
  fetchApplications()
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
