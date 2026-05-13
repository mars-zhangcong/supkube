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

    <!-- Application Details Drawer -->
    <el-drawer
      v-model="drawerVisible"
      :title="selectedApp ? selectedApp.namespace : ''"
      direction="rtl"
      size="520px"
    >
      <div v-if="selectedApp" class="app-details">
        <!-- Labels -->
        <div class="detail-section">
          <div class="detail-section-title">LABELS</div>
          <div class="labels-container">
            <el-tag
              v-for="(val, key) in selectedAppLabels"
              :key="key"
              size="small"
              class="label-tag"
            >{{ key }}:{{ val }}</el-tag>
            <span v-if="!selectedAppLabels || Object.keys(selectedAppLabels).length === 0" class="no-data">No labels</span>
          </div>
        </div>

        <!-- kubectl -->
        <div class="detail-section">
          <div class="detail-section-title">kubectl</div>
          <div class="kubectl-block">
            <code>$ kubectl get --raw /api/v1/namespaces/{{ selectedApp.namespace }}</code>
            <el-button size="small" text class="copy-btn" @click="copyKubectl">copy</el-button>
          </div>
        </div>

        <!-- Data (PVCs) -->
        <div class="detail-section">
          <div class="detail-section-title collapsible" @click="dataExpanded = !dataExpanded">
            Data <span class="count">({{ selectedApp.pvcs ? selectedApp.pvcs.length : 0 }})</span>
            <span class="chevron">{{ dataExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="dataExpanded" class="detail-table">
            <div class="detail-table-header">
              <span>PVC</span>
              <span>SIZE</span>
            </div>
            <div v-if="selectedApp.pvcs && selectedApp.pvcs.length > 0">
              <div v-for="pvc in selectedApp.pvcs" :key="pvc.name" class="detail-table-row">
                <span>📄 {{ pvc.name }}</span>
                <span>{{ pvc.size || '-' }}</span>
              </div>
            </div>
            <div v-else class="no-data">No PVCs found</div>
          </div>
        </div>

        <!-- Workloads -->
        <div class="detail-section">
          <div class="detail-section-title collapsible" @click="workloadsExpanded = !workloadsExpanded">
            Workloads <span class="count">({{ selectedApp.workloads || 0 }})</span>
            <span class="chevron">{{ workloadsExpanded ? '∧' : '∨' }}</span>
          </div>
          <div v-if="workloadsExpanded" class="detail-table">
            <div class="no-data">Workload details available via kubectl</div>
          </div>
        </div>

        <!-- Actions -->
        <div class="detail-actions">
          <el-button type="primary" @click="handleCommand('backup', selectedApp)">Backup Now</el-button>
          <el-button @click="handleCommand('restore', selectedApp)">Restore</el-button>
          <el-button @click="handleCommand('policy', selectedApp)">Create Policy</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getApplications } from '../api/velero'
import { ElMessage } from 'element-plus'

const router = useRouter()
const applications = ref([])
const loading = ref(false)
const drawerVisible = ref(false)
const selectedApp = ref(null)
const dataExpanded = ref(true)
const workloadsExpanded = ref(false)

const selectedAppLabels = computed(() => {
  if (!selectedApp.value) return {}
  return selectedApp.value.labels || {}
})

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
      selectedApp.value = row
      dataExpanded.value = true
      workloadsExpanded.value = false
      drawerVisible.value = true
      break
  }
}

const copyKubectl = () => {
  if (!selectedApp.value) return
  const cmd = `kubectl get --raw /api/v1/namespaces/${selectedApp.value.namespace}`
  navigator.clipboard.writeText(cmd).then(() => {
    ElMessage.success('Copied to clipboard')
  })
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
.status-compliant { color: #67c23a; }
.status-compliant .status-icon { border: 2px solid #67c23a; color: #67c23a; }
.status-unmanaged { color: #e6a23c; }
.status-unmanaged .status-icon { border: 2px solid #e6a23c; color: #e6a23c; }
.workload-count { color: #606266; font-size: 13px; }
.last-backup { display: flex; flex-direction: column; gap: 2px; }
.backup-time { color: #409eff; font-size: 12px; }
.no-restore { color: #c0c4cc; font-size: 13px; }
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.app-row:hover .more-btn) { color: #409eff; }

/* Drawer styles */
.app-details { padding: 0 4px; }
.detail-section { margin-bottom: 24px; }
.detail-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #909399;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 10px;
}
.detail-section-title.collapsible {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  text-transform: none;
  letter-spacing: 0;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}
.count { color: #909399; font-weight: 400; }
.chevron { margin-left: auto; color: #909399; }
.labels-container { display: flex; flex-wrap: wrap; gap: 6px; }
.label-tag { font-size: 11px; }
.kubectl-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 12px 16px;
  border-radius: 6px;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.copy-btn { color: #a8d8a8 !important; font-size: 12px; }
.detail-table { margin-top: 8px; }
.detail-table-header {
  display: grid;
  grid-template-columns: 1fr 100px;
  padding: 6px 12px;
  font-size: 11px;
  font-weight: 600;
  color: #909399;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid #f0f0f0;
}
.detail-table-row {
  display: grid;
  grid-template-columns: 1fr 100px;
  padding: 10px 12px;
  font-size: 13px;
  border-bottom: 1px solid #f9f9f9;
}
.no-data { color: #c0c4cc; font-size: 13px; padding: 8px 0; }
.detail-actions {
  display: flex;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
  margin-top: 8px;
}
</style>
