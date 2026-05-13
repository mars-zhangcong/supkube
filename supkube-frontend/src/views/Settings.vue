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
const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'

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
</style>
