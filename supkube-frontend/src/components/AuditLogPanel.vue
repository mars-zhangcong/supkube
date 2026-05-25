<!--
  AuditLogPanel (v0.8.5 step 4)
  ──────────────────────────────
  Admin-only audit list. Sourced from K8s Events labeled supkube.io/audit=true.
  Filters by user / result / resource / since. Newest first.

  Retention disclaimer: K8s Events default TTL is 1 hour. The audit table
  shows ONLY recent records. For long-term retention, customers should ship
  K8s Events to their SIEM (the same stdout backend logs are already
  capturing the data — see USER_MANUAL §13).
-->
<template>
  <div class="audit-panel">
    <el-alert
      type="info"
      :closable="false"
      style="margin-bottom: 16px"
    >
      <template #title>
        <strong>{{ t('audit.retentionTitle') }}</strong> — {{ t('audit.retentionDesc') }}
      </template>
    </el-alert>

    <div class="filter-bar">
      <el-input
        v-model="filterUser"
        :placeholder="t('audit.filterUser')"
        clearable
        size="default"
        style="width: 200px"
        @change="fetchLogs"
      />
      <el-select
        v-model="filterResult"
        :placeholder="t('audit.filterResult')"
        clearable
        size="default"
        style="width: 140px"
        @change="fetchLogs"
      >
        <el-option label="success" value="success" />
        <el-option label="denied" value="denied" />
        <el-option label="error" value="error" />
      </el-select>
      <el-select
        v-model="filterResource"
        :placeholder="t('audit.filterResource')"
        clearable
        size="default"
        style="width: 160px"
        @change="fetchLogs"
      >
        <el-option v-for="r in resourceTypes" :key="r" :label="r" :value="r" />
      </el-select>
      <el-input-number
        v-model="filterLimit"
        :min="20"
        :max="1000"
        :step="50"
        size="default"
        :placeholder="t('audit.filterLimit')"
        style="width: 140px"
        @change="fetchLogs"
      />
      <el-button type="primary" @click="fetchLogs" :loading="loading">
        <el-icon><Refresh /></el-icon> {{ t('common.refresh') }}
      </el-button>
      <span class="result-count">{{ t('audit.count', { n: items.length }) }}</span>
    </div>

    <div v-if="loading && items.length === 0" class="empty">
      <el-icon class="spin"><Loading /></el-icon>
      {{ t('common.loading') }}
    </div>
    <div v-else-if="items.length === 0" class="empty">
      <el-icon><InfoFilled /></el-icon>
      <div>{{ t('audit.empty') }}</div>
    </div>

    <el-table v-else :data="items" stripe size="small" class="audit-table">
      <el-table-column :label="t('audit.timestamp')" width="170" sortable :sort-method="(a, b) => new Date(a.timestamp) - new Date(b.timestamp)">
        <template #default="{ row }">{{ formatTime(row.timestamp) }}</template>
      </el-table-column>
      <el-table-column :label="t('audit.user')" min-width="170">
        <template #default="{ row }">
          <code class="audit-user">{{ row.user }}</code>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.result')" width="100">
        <template #default="{ row }">
          <el-tag :type="resultTagType(row.result)" size="small">{{ row.result }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.action')" width="100" prop="action" />
      <el-table-column :label="t('audit.resource')" width="140" prop="resource" />
      <el-table-column :label="t('audit.resourceName')" min-width="180">
        <template #default="{ row }">
          <code v-if="row.resourceName" class="audit-resource-name">{{ row.resourceName }}</code>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.namespace')" min-width="140">
        <template #default="{ row }">
          <span v-if="row.namespace">{{ row.namespace }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.method')" width="80" prop="method" />
      <el-table-column :label="t('audit.statusCode')" width="80" prop="statusCode" align="right" />
      <el-table-column :label="t('audit.sourceIP')" width="140">
        <template #default="{ row }">
          <code v-if="row.sourceIP" class="audit-ip">{{ row.sourceIP }}</code>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { InfoFilled, Loading, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getAuditLogs } from '../api/velero'

const { t } = useI18n()

const items = ref([])
const loading = ref(false)
const filterUser = ref('')
const filterResult = ref('')
const filterResource = ref('')
const filterLimit = ref(200)

// Resource types the backend audits — keep in sync with parseResourceFromPath().
const resourceTypes = [
  'Backup', 'Restore', 'Schedule', 'TransformSet',
  'StorageProfile', 'SnapshotProfile', 'Namespace', 'Auth'
]

const formatTime = (iso) => {
  if (!iso) return '-'
  return new Date(iso).toLocaleString()
}

const resultTagType = (r) => ({
  success: 'success',
  denied: 'warning',
  error: 'danger'
})[r] || ''

async function fetchLogs() {
  loading.value = true
  try {
    const params = { limit: filterLimit.value }
    if (filterUser.value) params.user = filterUser.value
    if (filterResult.value) params.result = filterResult.value
    if (filterResource.value) params.resource = filterResource.value
    const res = await getAuditLogs(params)
    items.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load audit log: ' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

onMounted(fetchLogs)
</script>

<style scoped>
.audit-panel { padding: 4px 0; }

.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.result-count {
  margin-left: auto;
  color: var(--sk-text-caption);
  font-size: 12px;
}

.empty {
  text-align: center;
  padding: 40px 20px;
  color: var(--sk-text-caption);
  font-size: 13px;
  background: #fff;
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
}
.empty .el-icon { font-size: 28px; display: block; margin: 0 auto 8px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }

code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11.5px;
  background: rgba(0,0,0,0.04);
  padding: 1px 4px;
  border-radius: 3px;
}
.audit-user { color: var(--sk-primary); }
.audit-ip { color: var(--sk-text-caption); }
.audit-resource-name { color: var(--sk-text-secondary); }
.muted { color: var(--sk-text-placeholder); }

html.dark .empty { background: #1f2026; border-color: #3a3d44; }
html.dark code { background: rgba(255,255,255,0.08); }
html.dark .audit-resource-name { color: #e5eaf3; }
</style>
