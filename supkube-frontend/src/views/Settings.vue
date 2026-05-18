<template>
  <div class="settings-page">
    <h3>Settings</h3>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card v-loading="loading">
          <template #header>Velero Status</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Status">
              <el-tag :type="veleroStatus.connected ? 'success' : 'danger'">
                {{ veleroStatus.connected ? 'Connected' : 'Disconnected' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Version">
              {{ veleroStatus.version || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Namespace">
              {{ veleroStatus.namespace || 'velero' }}
            </el-descriptions-item>
            <el-descriptions-item label="Plugins">
              {{ (veleroStatus.plugins || []).join(', ') || '-' }}
            </el-descriptions-item>
          </el-descriptions>
          <el-alert
            v-if="!veleroStatus.connected && !loading"
            type="error"
            title="Velero is not reachable"
            description="Check that Velero is installed and running in the cluster. Verify the namespace configuration."
            :closable="false"
            style="margin-top: 16px"
          />
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card v-loading="loading">
          <template #header>Cluster Information</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Kubernetes Version">
              {{ clusterInfo.k8sVersion || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Nodes">
              {{ clusterInfo.nodes ?? '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Namespaces">
              {{ clusterInfo.namespaces ?? '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card style="margin-top: 20px">
          <template #header>SupKube</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Version">
              {{ supkubeInfo.version || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="API Endpoint">
              {{ apiUrl }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

    <!-- Data Protection Policy Settings (v0.7-policy-2) -->
    <el-card style="margin-top: 20px">
      <template #header>Data Protection Policy</template>
      <el-form label-width="320px" label-position="left">
        <el-form-item label="Block snapshot-only policies">
          <el-switch v-model="blockSnapshotOnly" @change="saveBlockSnapshotOnly" />
          <span class="form-hint">
            When enabled, the Create Policy form refuses to save policies that disable Export.
            Recommended for production clusters where every restore point must be durable.
          </span>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getStatus, getDashboardSummary } from '../api/velero'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const veleroStatus = ref({ connected: false, version: '', namespace: '', plugins: [] })
const clusterInfo = ref({ k8sVersion: '', nodes: 0, namespaces: 0 })
const supkubeInfo = ref({ version: '' })
const apiUrl = import.meta.env.VITE_API_URL || '/api/v1'

// v0.7-policy-2: global block toggle for snapshot-only policies. Stored
// in localStorage for now — v0.8 RBAC will move this to a CRD setting.
const BLOCK_KEY = 'supkube.policy.blockSnapshotOnly'
const blockSnapshotOnly = ref(localStorage.getItem(BLOCK_KEY) === 'true')
const saveBlockSnapshotOnly = (v) => {
  localStorage.setItem(BLOCK_KEY, String(v))
  ElMessage.success(v ? 'Snapshot-only policies are now blocked' : 'Snapshot-only policies are now allowed')
}

const fetchData = async () => {
  loading.value = true
  try {
    const [statusRes, summaryRes] = await Promise.all([
      getStatus(),
      getDashboardSummary().catch(() => null)
    ])

    // Parse status response
    const status = statusRes.data
    supkubeInfo.value.version = status.version || ''
    veleroStatus.value.connected = status.status === 'ok'
    veleroStatus.value.version = status.veleroVersion || ''
    veleroStatus.value.namespace = status.veleroNamespace || 'velero'
    veleroStatus.value.plugins = status.plugins || []

    // Parse dashboard summary for cluster info
    if (summaryRes) {
      const summary = summaryRes.data
      clusterInfo.value.nodes = summary.cluster?.nodes ?? 0
      clusterInfo.value.namespaces = summary.cluster?.namespaces ?? 0
      clusterInfo.value.k8sVersion = summary.cluster?.k8sVersion || ''
    }
  } catch (e) {
    ElMessage.error('Failed to load settings data')
    console.error(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.settings-page h3 {
  margin: 0 0 16px 0;
}
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
  margin-top: 6px;
  max-width: 620px;
}
</style>
