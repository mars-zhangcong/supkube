<template>
  <div class="policies-page">
    <div class="page-header">
      <h3>Backup Policies (Schedules)</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        Create Policy
      </el-button>
    </div>

    <el-card>
      <el-table :data="schedules" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column label="Namespaces">
          <template #default="{ row }">
            {{ (row.spec?.template?.includedNamespaces || ['*']).join(', ') }}
          </template>
        </el-table-column>
        <el-table-column label="Schedule">
          <template #default="{ row }">
            <code>{{ row.spec?.schedule || '-' }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Status" width="120">
          <template #default="{ row }">
            <el-tag :type="row.spec?.paused ? 'warning' : 'success'" size="small">
              {{ row.spec?.paused ? 'Paused' : 'Active' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Protection Level" width="200">
          <template #default="{ row }">
            <el-tooltip :content="protectionLevel(row).tooltip" placement="top" :show-after="300">
              <span class="protection-badge" :class="`protection-${protectionLevel(row).key}`">
                <span class="protection-icon">{{ protectionLevel(row).icon }}</span>
                {{ protectionLevel(row).label }}
              </span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="Last Backup">
          <template #default="{ row }">
            {{ formatTime(row.status?.lastBackup) }}
          </template>
        </el-table-column>
        <el-table-column label="TTL">
          <template #default="{ row }">
            {{ formatTTL(row.spec?.template?.ttl) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="250">
          <template #default="{ row }">
            <el-button
              size="small"
              :type="row.spec?.paused ? 'success' : 'warning'"
              @click="togglePause(row)"
            >
              {{ row.spec?.paused ? 'Resume' : 'Pause' }}
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Policy Dialog (v0.7 Actions model) -->
    <el-dialog v-model="showCreateDialog" title="Create Backup Policy" width="680px" top="6vh">
      <el-form :model="createForm" label-width="180px">
        <el-form-item label="Policy Name" required>
          <el-input v-model="createForm.name" placeholder="daily-backup" />
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All namespaces (default)"
            style="width: 100%"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>

        <!-- ============ ACTIONS BLOCK 1: SNAPSHOT ============ -->
        <div class="action-block snapshot-block">
          <div class="action-block-header">
            <span class="action-block-icon">📸</span>
            <span class="action-block-title">Snapshot</span>
            <span class="action-block-subtitle">Local · L1</span>
            <el-checkbox v-model="createForm.snapshot.enabled" disabled style="margin-left: auto">
              Always on
            </el-checkbox>
          </div>
          <el-form-item label="Snapshot Schedule" required>
            <el-select v-model="createForm.snapshot.schedulePreset" @change="onSnapshotPresetChange" style="width: 56%; margin-right: 8px">
              <el-option label="Every hour" value="0 * * * *" />
              <el-option label="Every 6 hours" value="0 */6 * * *" />
              <el-option label="Every 12 hours" value="0 */12 * * *" />
              <el-option label="Every day at midnight" value="0 0 * * *" />
              <el-option label="Custom" value="custom" />
            </el-select>
            <el-input
              v-if="createForm.snapshot.schedulePreset === 'custom'"
              v-model="createForm.snapshot.schedule"
              placeholder="0 * * * *"
              style="width: 40%"
            />
          </el-form-item>
          <el-form-item label="Snapshot Retention">
            <el-select v-model="createForm.snapshot.retention" style="width: 100%">
              <el-option label="6 hours" value="6h" />
              <el-option label="12 hours" value="12h" />
              <el-option label="24 hours (default)" value="24h" />
              <el-option label="3 days" value="72h" />
              <el-option label="7 days" value="168h" />
            </el-select>
          </el-form-item>
          <el-form-item label="Volume Mode">
            <el-radio-group v-model="createForm.snapshot.volumeMode">
              <el-radio-button value="filesystem">📁 Filesystem</el-radio-button>
              <el-radio-button value="csi">📸 CSI</el-radio-button>
            </el-radio-group>
            <span class="form-hint">CSI requires the namespace's PVCs to use a snapshot-capable StorageClass.</span>
          </el-form-item>
        </div>

        <!-- ============ ACTIONS BLOCK 2: EXPORT ============ -->
        <div class="action-block export-block" :class="{ 'is-disabled': !createForm.export.enabled }">
          <div class="action-block-header">
            <span class="action-block-icon">📦</span>
            <span class="action-block-title">Export to Object Storage</span>
            <span class="action-block-subtitle">L2 · Durable Backup</span>
            <el-checkbox v-model="createForm.export.enabled" @change="onExportToggle" style="margin-left: auto">
              Enable
            </el-checkbox>
          </div>
          <template v-if="createForm.export.enabled">
            <el-form-item label="Export Schedule">
              <el-select v-model="createForm.export.schedulePreset" @change="onExportPresetChange" style="width: 56%; margin-right: 8px">
                <el-option label="Every day at midnight (default)" value="0 0 * * *" />
                <el-option label="Every 6 hours" value="0 */6 * * *" />
                <el-option label="Every 12 hours" value="0 */12 * * *" />
                <el-option label="Every week (Sunday)" value="0 0 * * 0" />
                <el-option label="Same as Snapshot" value="same" />
                <el-option label="Custom" value="custom" />
              </el-select>
              <el-input
                v-if="createForm.export.schedulePreset === 'custom'"
                v-model="createForm.export.schedule"
                placeholder="0 0 * * *"
                style="width: 40%"
              />
            </el-form-item>
            <el-form-item label="Export Retention">
              <el-select v-model="createForm.export.retention" style="width: 100%">
                <el-option label="7 days" value="168h" />
                <el-option label="14 days" value="336h" />
                <el-option label="30 days (default)" value="720h" />
                <el-option label="60 days" value="1440h" />
                <el-option label="90 days" value="2160h" />
              </el-select>
            </el-form-item>
            <el-form-item label="Storage Profile">
              <el-input v-model="createForm.export.storageLocation" placeholder="default" />
            </el-form-item>
          </template>
          <p v-else class="action-disabled-warning">
            ⚠ Without Export, this policy produces <strong>snapshot-only restore points</strong> — these are
            <strong>not durable backups</strong>. Data will be lost if the underlying storage fails.
            <br />
            Use snapshot-only mode for dev/staging environments or fast rollback scenarios only.
          </p>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">
          Create
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { getSchedules, createSchedule, patchSchedule, deleteSchedule, getNamespaces } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const schedules = ref([])
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)

// v0.7 Actions model: Snapshot (always on) + Export (default on, opt-out
// triggers confirmation). Both have independent schedule + retention in the
// UI; v0.7 maps them to a single Velero Schedule with the shorter cron and
// the longer ttl, with intent recorded in annotations for v0.9 to consume.
const defaultForm = () => ({
  name: '',
  includedNamespaces: [],
  snapshot: {
    enabled: true,
    schedulePreset: '0 * * * *',
    schedule: '0 * * * *',
    retention: '24h',
    volumeMode: 'filesystem' // 'filesystem' | 'csi'
  },
  export: {
    enabled: true,
    schedulePreset: '0 0 * * *',
    schedule: '0 0 * * *',
    retention: '720h',
    storageLocation: 'default'
  }
})
const createForm = ref(defaultForm())

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

// Derive Protection Level from a Velero Schedule. Reads our intent
// annotations first (set by v0.7+ policies); falls back to spec inference
// for pre-existing schedules (treat them as L2 if Velero will write to BSL,
// which is always true — Velero always exports). v0.9 adds L3 (immutable).
const protectionLevel = (row) => {
  const ann = row?.metadata?.annotations || {}
  const exportEnabled = ann['supkube.io/export-enabled']
  if (exportEnabled === 'false') {
    return {
      key: 'l1',
      label: 'L1 Snapshot Only',
      icon: '⚠',
      tooltip: 'Snapshot-only — NOT a durable backup. Data is lost if the underlying storage fails. Use only for dev/staging or fast rollback scenarios.'
    }
  }
  // Future: detect immutable BSL — for now everything with export is L2.
  return {
    key: 'l2',
    label: 'L2 Backup',
    icon: '✓',
    tooltip: 'Snapshot + Export to object storage. Durable backup that survives storage failure.'
  }
}

// Velero schedule.spec.template.ttl defaults to zero when unset; the actual
// retention then falls back to Velero's server-side default (30 days). Show
// users the effective retention instead of the misleading literal "0s".
const formatTTL = (ttl) => {
  if (!ttl || ttl === '0s' || ttl === '0' || ttl === '0h' || ttl === '0h0m0s') {
    return 'Default (30d)'
  }
  const match = /^(\d+)h$/.exec(ttl)
  if (match) {
    const hours = parseInt(match[1], 10)
    if (hours > 0 && hours % 24 === 0) return `${hours / 24}d`
  }
  return ttl
}

const onSnapshotPresetChange = (val) => {
  if (val !== 'custom') createForm.value.snapshot.schedule = val
}
const onExportPresetChange = (val) => {
  if (val === 'custom') return
  if (val === 'same') {
    createForm.value.export.schedule = createForm.value.snapshot.schedule
  } else {
    createForm.value.export.schedule = val
  }
}

// Default-Export-checked guardrail (v0.7-policy-2 part 1): when user opts
// out of Export, force a confirmation dialog. ElMessageBox is async, so we
// revert the toggle immediately and re-enable only on confirm.
const onExportToggle = (newVal) => {
  if (newVal === true) return // turning ON: no friction
  // newVal === false: confirm + revert if user backs out
  createForm.value.export.enabled = true // revert optimistically
  ElMessageBox.confirm(
    'Snapshot alone is not a backup. Data is lost if the underlying storage fails. ' +
    'This is acceptable only for development/staging environments or fast rollback scenarios. ' +
    'Continue without Export?',
    'Disable Export?',
    {
      type: 'warning',
      confirmButtonText: 'Yes, snapshot-only',
      cancelButtonText: 'Keep Export enabled',
      confirmButtonClass: 'el-button--warning'
    }
  ).then(() => {
    createForm.value.export.enabled = false
  }).catch(() => {
    // user backed out; export stays enabled
  })
}

const fetchSchedules = async () => {
  loading.value = true
  try {
    const res = await getSchedules()
    schedules.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load policies')
    console.error(e)
  } finally {
    loading.value = false
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

// Parse Go duration suffixes (h only — Velero's TTL field is `metav1.Duration`,
// in practice we only ever store h). Returns null if input doesn't match.
const parseHours = (s) => {
  const m = /^(\d+)h$/.exec(s || '')
  return m ? parseInt(m[1], 10) : null
}

// v0.7 collapse: single Velero Schedule per Policy. cron = shorter of the
// two; ttl = longer of the two. Snapshot/Export intent preserved in
// annotations so v0.9's self-managed scheduler can hydrate from existing
// Schedules without losing user intent.
const collapseToVelero = (form) => {
  const snapHours = parseHours(form.snapshot.retention) || 24
  const expHours = form.export.enabled ? (parseHours(form.export.retention) || 720) : 0
  const ttlHours = Math.max(snapHours, expHours)

  // cron picking: simple heuristic — when Export is on, use export cron
  // (slower) by default for cost. If user wants tighter RPO they pick
  // matching cron explicitly via "Same as Snapshot".
  const cron = form.export.enabled ? form.export.schedule : form.snapshot.schedule

  return {
    name: form.name,
    schedule: cron,
    includedNamespaces: form.includedNamespaces.length > 0 ? form.includedNamespaces : undefined,
    ttl: `${ttlHours}h`,
    storageLocation: form.export.storageLocation || 'default',
    snapshotVolumes: form.snapshot.volumeMode === 'csi',
    defaultVolumesToFsBackup: form.snapshot.volumeMode === 'filesystem',
    // v0.7 intent annotations (consumed by v0.9 self-managed scheduler)
    annotations: {
      'supkube.io/snapshot-schedule': form.snapshot.schedule,
      'supkube.io/snapshot-retention': form.snapshot.retention,
      'supkube.io/export-enabled': String(form.export.enabled),
      'supkube.io/export-schedule': form.export.schedule,
      'supkube.io/export-retention': form.export.retention,
      'supkube.io/volume-mode': form.snapshot.volumeMode
    }
  }
}

const handleCreate = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Please enter a policy name')
    return
  }
  if (!createForm.value.snapshot.schedule) {
    ElMessage.warning('Please set a Snapshot schedule')
    return
  }
  // v0.7-policy-2 global block: if admin set "Block snapshot-only" in
  // Settings and this policy has Export off, refuse to save.
  if (!createForm.value.export.enabled &&
      localStorage.getItem('supkube.policy.blockSnapshotOnly') === 'true') {
    ElMessageBox.alert(
      'This cluster is configured to block snapshot-only policies. Enable Export to proceed, or change the setting in Settings → Data Protection Policy.',
      'Snapshot-only policies are blocked',
      { type: 'error', confirmButtonText: 'Got it' }
    )
    return
  }
  creating.value = true
  try {
    const payload = collapseToVelero(createForm.value)
    await createSchedule(payload)
    const mode = createForm.value.export.enabled ? 'Snapshot + Export' : 'Snapshot-only ⚠'
    ElMessage.success(`Policy "${createForm.value.name}" created (${mode})`)
    showCreateDialog.value = false
    createForm.value = defaultForm()
    await fetchSchedules()
  } catch (e) {
    ElMessage.error('Failed to create policy: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const togglePause = async (row) => {
  const newPaused = !row.spec?.paused
  try {
    await patchSchedule(row.metadata.name, { paused: newPaused })
    ElMessage.success(`Policy "${row.metadata.name}" ${newPaused ? 'paused' : 'resumed'}`)
    await fetchSchedules()
  } catch (e) {
    ElMessage.error('Failed to update policy: ' + (e.response?.data?.error || e.message))
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete policy "${row.metadata.name}"?`,
      'Delete Policy',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteSchedule(row.metadata.name)
    ElMessage.success(`Policy "${row.metadata.name}" deleted`)
    await fetchSchedules()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete policy')
    }
  }
}

onMounted(() => {
  fetchSchedules()
  fetchNamespaces()
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

/* Action blocks in Create Policy dialog (v0.7 Actions model) */
.action-block {
  margin-top: 20px;
  padding: 14px 16px 4px;
  border-radius: 8px;
  border: 1px solid #ebeef5;
  background: #fafbfc;
  transition: opacity 0.15s ease, background 0.15s ease;
}
.action-block.is-disabled {
  background: #fef0f0;
  border-color: #fbc4c4;
}
.snapshot-block {
  border-left: 3px solid #409eff;
}
.export-block {
  border-left: 3px solid #67c23a;
}
.export-block.is-disabled {
  border-left-color: #e6a23c;
}
.action-block-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #ebeef5;
}
.action-block-icon { font-size: 20px; }
.action-block-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.action-block-subtitle {
  font-size: 11px;
  color: #909399;
  font-weight: 500;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.action-disabled-warning {
  margin: 0 0 12px 0;
  padding: 10px 12px;
  font-size: 12px;
  color: #5b2929;
  line-height: 1.6;
}
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

/* Protection Level badges (v0.7-policy-2) */
.protection-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
}
.protection-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  font-size: 11px;
  font-weight: bold;
}
.protection-l1 {
  background: #fdf6ec;
  color: #b54708;
}
.protection-l1 .protection-icon {
  border: 1.5px solid #e6a23c;
  color: #e6a23c;
  background: #ffffff;
}
.protection-l2 {
  background: #f0f9eb;
  color: #225a17;
}
.protection-l2 .protection-icon {
  border: 1.5px solid #67c23a;
  color: #67c23a;
  background: #ffffff;
}
.protection-l3 {
  background: #ecf5ff;
  color: #1d3a8a;
}
.protection-l3 .protection-icon {
  border: 1.5px solid #409eff;
  color: #409eff;
  background: #ffffff;
}
</style>
