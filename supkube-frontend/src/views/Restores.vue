<template>
  <div class="restores-page">
    <div class="page-header">
      <h3>{{ t('restores.title') }}</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><RefreshRight /></el-icon>
        {{ t('restores.create') }}
      </el-button>
    </div>

    <el-card>
      <el-table :data="restores" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column prop="spec.backupName" label="From Backup" />
        <el-table-column label="Included Namespaces">
          <template #default="{ row }">
            {{ (row.spec?.includedNamespaces || ['*']).join(', ') }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="phaseTagType(row.status?.phase)">
              {{ normalizePhase(row.status?.phase) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Items Restored">
          <template #default="{ row }">
            {{ row.status?.progress?.itemsRestored ?? '-' }} / {{ row.status?.progress?.totalItems ?? '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="240">
          <template #default="{ row }">
            <el-button size="small" @click="viewResults(row)">
              Results
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Results Drawer -->
    <el-drawer v-model="showResultsDrawer" :title="`Restore Results: ${resultsRestoreName}`" size="640px">
      <div v-loading="loadingResults">
        <template v-if="results">
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Phase">
              <el-tag :type="phaseTagType(results.phase)">{{ normalizePhase(results.phase) }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item v-if="results.startTimestamp" label="Started">
              {{ formatTime(results.startTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item v-if="results.completionTimestamp" label="Completed">
              {{ formatTime(results.completionTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Progress">
              {{ results.progress?.itemsRestored ?? '-' }} / {{ results.progress?.totalItems ?? '-' }}
            </el-descriptions-item>
            <el-descriptions-item v-if="results.failureReason" label="Failure Reason">
              <pre class="result-block result-error">{{ results.failureReason }}</pre>
            </el-descriptions-item>
          </el-descriptions>

          <h4 v-if="results.validationErrors?.length" class="results-h">Validation Errors ({{ results.validationErrors.length }})</h4>
          <ul v-if="results.validationErrors?.length" class="result-block result-error">
            <li v-for="(e, idx) in results.validationErrors" :key="idx">{{ e }}</li>
          </ul>

          <!-- v0.7.11: Velero's per-resource error/warning messages live in
               object storage. The backend pulls them via DownloadRequest and
               embeds them under results.detailed. We render them grouped by
               section (Velero / Cluster / Namespaces) so the user can see
               *which* resource failed and *why*. -->
          <template v-if="results.errors">
            <h4 class="results-h">Errors ({{ results.errors }})</h4>
            <div v-if="results.detailed?.errors">
              <div v-for="group in messageGroups(results.detailed.errors)" :key="`e-${group.label}`" class="msg-group">
                <div class="msg-group-label">{{ group.label }}</div>
                <ul class="result-block result-error">
                  <li v-for="(m, i) in group.messages" :key="i">{{ m }}</li>
                </ul>
              </div>
            </div>
            <div v-else-if="results.detailedError" class="result-block result-error">
              <strong>Could not fetch detail:</strong> {{ results.detailedError }}
            </div>
            <div v-else class="result-block result-loading">Loading error detail…</div>
          </template>

          <template v-if="results.warnings">
            <h4 class="results-h">Warnings ({{ results.warnings }})</h4>
            <div v-if="results.detailed?.warnings">
              <div v-for="group in messageGroups(results.detailed.warnings)" :key="`w-${group.label}`" class="msg-group">
                <div class="msg-group-label">{{ group.label }}</div>
                <ul class="result-block result-warning">
                  <li v-for="(m, i) in group.messages" :key="i">{{ m }}</li>
                </ul>
              </div>
            </div>
            <div v-else-if="results.detailedError" class="result-block result-warning">
              <strong>Could not fetch detail:</strong> {{ results.detailedError }}
            </div>
            <div v-else class="result-block result-loading">Loading warning detail…</div>
          </template>

          <p v-if="!results.validationErrors?.length && !results.errors && !results.warnings && !results.failureReason" class="result-empty">
            No errors or warnings reported.
          </p>
        </template>
      </div>
    </el-drawer>

    <!-- Create Restore Dialog -->
    <el-dialog v-model="showCreateDialog" title="Create Restore" width="500px">
      <el-form :model="createForm" label-width="180px">
        <el-form-item label="Restore Name" required>
          <el-input v-model="createForm.name" placeholder="my-restore" />
        </el-form-item>
        <el-form-item label="Source Backup" required>
          <el-select
            v-model="createForm.backupName"
            filterable
            placeholder="Select a backup"
            style="width: 100%"
          >
            <el-option
              v-for="b in availableBackups"
              :key="b.metadata.name"
              :label="b.metadata.name"
              :value="b.metadata.name"
            >
              <span>{{ b.metadata.name }}</span>
              <el-tag size="small" :type="phaseTagType(b.status?.phase)" style="margin-left: 8px">
                {{ normalizePhase(b.status?.phase) }}
              </el-tag>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item v-if="backupResources" label="Resource Preview">
          <div v-loading="loadingResources" style="width: 100%; border: 1px solid #eee; border-radius: 4px; padding: 8px; font-size: 13px;">
            <p><strong>Namespaces:</strong> {{ (backupResources.includedNamespaces || ['*']).join(', ') }}</p>
            <p><strong>Items:</strong> {{ backupResources.itemsBackedUp ?? '-' }} / {{ backupResources.totalItems ?? '-' }}</p>
            <p v-if="backupResources.includedResources"><strong>Resources:</strong> {{ backupResources.includedResources.join(', ') }}</p>
            <p v-if="backupResources.labelSelector"><strong>Labels:</strong> {{ JSON.stringify(backupResources.labelSelector) }}</p>
          </div>
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All from backup (default)"
          >
            <el-option
              v-for="ns in namespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Namespace Mapping">
          <div v-for="(mapping, idx) in createForm.namespaceMappings" :key="idx" class="ns-mapping-row">
            <el-input v-model="mapping.from" placeholder="Source NS" style="width: 40%" />
            <span style="margin: 0 8px">→</span>
            <el-input v-model="mapping.to" placeholder="Target NS" style="width: 40%" />
            <el-button type="danger" size="small" @click="removeMapping(idx)" circle>
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
          <el-button size="small" @click="addMapping">+ Add Mapping</el-button>
        </el-form-item>
        <el-form-item label="Restore PVs">
          <el-switch v-model="createForm.restorePVs" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">
          Restore
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import { useRoute } from 'vue-router'
import { RefreshRight, Close } from '@element-plus/icons-vue'
import { getRestores, createRestore, getBackups, getNamespaces, getBackupResources, deleteRestore, getRestoreResults } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'
import { normalizePhase, phaseTagType } from '../utils/phase'

const route = useRoute()
const restores = ref([])
const availableBackups = ref([])
const backupResources = ref(null)
const loadingResources = ref(false)
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const showResultsDrawer = ref(false)
const resultsRestoreName = ref('')
const results = ref(null)
const loadingResults = ref(false)
let pollTimer = null

const createForm = ref({
  name: '',
  backupName: '',
  includedNamespaces: [],
  namespaceMappings: [],
  restorePVs: true
})

const viewResults = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  resultsRestoreName.value = name
  results.value = null
  showResultsDrawer.value = true
  loadingResults.value = true
  try {
    const res = await getRestoreResults(name)
    results.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load restore results: ' + (e.response?.data?.error || e.message))
  } finally {
    loadingResults.value = false
  }
}

const handleDelete = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      `Delete restore "${name}"? This removes the restore CR; restored objects in the cluster are NOT affected.`,
      'Confirm Delete',
      { type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteRestore(name)
    ElMessage.success(`Restore "${name}" deleted`)
    await fetchRestores()
  } catch (e) {
    ElMessage.error('Failed to delete restore: ' + (e.response?.data?.error || e.message))
  }
}

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const addMapping = () => {
  createForm.value.namespaceMappings.push({ from: '', to: '' })
}

const removeMapping = (idx) => {
  createForm.value.namespaceMappings.splice(idx, 1)
}

const fetchRestores = async () => {
  loading.value = true
  try {
    const res = await getRestores()
    restores.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load restores')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchBackups = async () => {
  try {
    const res = await getBackups()
    const items = res.data.items || []
    availableBackups.value = items.filter(b => b.status?.phase === 'Completed')
  } catch (e) {
    console.error('Failed to load backups:', e)
  }
}

const fetchResourcePreview = async (backupName) => {
  if (!backupName) {
    backupResources.value = null
    return
  }
  loadingResources.value = true
  try {
    const res = await getBackupResources(backupName)
    backupResources.value = res.data
  } catch (e) {
    backupResources.value = null
    console.error('Failed to load resource preview:', e)
  } finally {
    loadingResources.value = false
  }
}

const fetchNamespaces = async () => {
  try {
    const res = await getNamespaces()
    const items = res.data.namespaces || res.data.items || res.data || []
    namespaces.value = items.map(ns => ns.metadata?.name || ns).filter(Boolean)
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

const handleCreate = async () => {
  if (!createForm.value.name || !createForm.value.backupName) {
    ElMessage.warning('Please fill in restore name and select a backup')
    return
  }
  creating.value = true
  try {
    const nsMappings = {}
    createForm.value.namespaceMappings.forEach(m => {
      if (m.from && m.to) nsMappings[m.from] = m.to
    })

    const payload = {
      name: createForm.value.name,
      backupName: createForm.value.backupName,
      includedNamespaces: createForm.value.includedNamespaces.length > 0
        ? createForm.value.includedNamespaces : undefined,
      namespaceMapping: Object.keys(nsMappings).length > 0 ? nsMappings : undefined,
      restorePVs: createForm.value.restorePVs
    }
    await createRestore(payload)
    ElMessage.success(`Restore "${createForm.value.name}" created. Monitoring progress...`)
    showCreateDialog.value = false
    createForm.value = {
      name: '',
      backupName: '',
      includedNamespaces: [],
      namespaceMappings: [],
      restorePVs: true
    }
    await fetchRestores()
    startPolling()
  } catch (e) {
    ElMessage.error('Failed to create restore: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

// Watch for backup selection to load resource preview
watch(() => createForm.value.backupName, (newVal) => {
  fetchResourcePreview(newVal)
})

// v0.7.11: flatten the Velero results structure into labeled groups for
// rendering. Input shape (from BSL):
//   { velero: [...], cluster: [...], namespaces: { foo: [...], bar: [...] } }
// Output: [ { label: "Velero engine", messages: [...] },
//           { label: "Cluster-scoped", messages: [...] },
//           { label: "Namespace: foo", messages: [...] }, ... ]
// Empty groups are skipped so we don't render empty headers.
const messageGroups = (section) => {
  if (!section) return []
  const groups = []
  if (section.velero?.length) {
    groups.push({ label: 'Velero engine', messages: section.velero })
  }
  if (section.cluster?.length) {
    groups.push({ label: 'Cluster-scoped', messages: section.cluster })
  }
  if (section.namespaces) {
    for (const [ns, msgs] of Object.entries(section.namespaces)) {
      if (msgs && msgs.length) {
        groups.push({ label: `Namespace: ${ns}`, messages: msgs })
      }
    }
  }
  return groups
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(fetchRestores, 5000)
}

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  fetchRestores()
  fetchBackups()
  fetchNamespaces()

  // Pre-fill backup name if navigated from Backups page
  if (route.query.backup) {
    createForm.value.backupName = route.query.backup
    showCreateDialog.value = true
  }
})

onUnmounted(() => {
  stopPolling()
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
.ns-mapping-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
.result-block {
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 4px;
  padding: 10px 14px;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 8px 0;
}
.result-error {
  background: #fef0f0;
  border-color: #fbc4c4;
  color: #5b2929;
}
.result-warning {
  background: #fdf6ec;
  border-color: #f5dab1;
  color: #67430b;
}
.result-loading {
  background: #f4f5f9;
  border-color: #dcdfe6;
  color: var(--sk-text-caption);
  font-style: italic;
}
.result-empty {
  color: var(--sk-text-caption);
  font-size: 13px;
}
.results-h {
  margin: 18px 0 6px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--sk-text);
}
.msg-group {
  margin-bottom: 10px;
}
.msg-group-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--sk-text-muted);
  margin: 4px 0;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}
.msg-group ul.result-block {
  margin: 4px 0;
  padding-left: 28px;
}
.msg-group ul.result-block li {
  margin: 3px 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
}
</style>
