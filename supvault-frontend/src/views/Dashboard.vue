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
      <template #header>Recent Backups</template>
      <el-table :data="recentBackups" style="width: 100%">
        <el-table-column prop="metadata.name" label="Name" />
        <el-table-column prop="status.phase" label="Status">
          <template #default="{ row }">
            <el-tag :type="row.status?.phase === 'Completed' ? 'success' : 'danger'">
              {{ row.status?.phase || 'Unknown' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="metadata.creationTimestamp" label="Created" />
        <el-table-column prop="status.expiration" label="Expires" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getBackups, getStatus } from '../api/velero'

const stats = ref({ namespaces: 0, backups: 0, successful: 0, failed: 0 })
const recentBackups = ref([])

onMounted(async () => {
  try {
    const backupsRes = await getBackups()
    const items = backupsRes.data.items || []
    recentBackups.value = items.slice(0, 10)
    stats.value.backups = items.length
    stats.value.successful = items.filter(b => b.status?.phase === 'Completed').length
    stats.value.failed = items.filter(b => b.status?.phase === 'Failed').length
  } catch (e) {
    console.error(e)
  }
})
</script>

<style scoped>
.stat { text-align: center; padding: 10px; }
.stat-value { font-size: 2em; font-weight: bold; }
.stat-label { color: #666; margin-top: 5px; }
.success { color: #67c23a; }
.danger { color: #f56c6c; }
</style>
