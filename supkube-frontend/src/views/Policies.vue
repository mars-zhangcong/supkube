<template>
  <div class="policies-page">
    <div class="page-header">
      <div class="page-header-text">
        <h3>{{ t('policies.title') }}</h3>
        <p class="page-desc">{{ t('policies.desc') }}</p>
      </div>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        {{ t('policies.create') }}
      </el-button>
    </div>

    <!-- Kasten-style filter toolbar -->
    <div class="filter-toolbar">
      <el-select v-model="actionFilter" class="filter-action">
        <el-option :label="t('policies.allActions')" value="all" />
        <el-option :label="t('policies.actionSnapshot')" value="Snapshot" />
        <el-option :label="t('policies.actionSnapshotExport')" value="Snapshot+Export" />
      </el-select>
      <el-select v-model="freqFilter" class="filter-freq">
        <el-option :label="t('policies.allFrequencies')" value="all" />
        <el-option :label="t('advisor.schedule.hourly')" value="hourly" />
        <el-option :label="t('advisor.schedule.daily')" value="daily" />
        <el-option :label="t('advisor.schedule.weekly')" value="weekly" />
      </el-select>
      <el-input v-model="nameFilter" :placeholder="t('common.filterByName')" clearable class="filter-name">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="filter-spacer"></span>
      <span class="filter-summary">
        {{ t('policies.viewing', { filtered: filteredSchedules.length, total: schedules.length }) }}
      </span>
    </div>

    <el-card>
      <el-table :data="filteredSchedules" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" :label="t('common.name').toUpperCase()" sortable min-width="160">
          <template #default="{ row }">
            <span class="policy-name">{{ row.metadata?.name }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.validation').toUpperCase()" width="130">
          <template #default="{ row }">
            <span class="validation-cell" :class="`validation-${validationOf(row).key}`">
              {{ validationOf(row).icon }} {{ validationOf(row).label }}
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.resources').toUpperCase()" min-width="180">
          <template #default="{ row }">
            <el-tooltip
              v-for="ns in resourceNamespaces(row)"
              :key="ns"
              :content="t('policies.namespaceTooltip', { name: ns })"
              placement="top"
              :show-after="200"
            >
              <el-tag
                size="small"
                effect="plain"
                round
                class="ns-chip"
              >🗂 {{ ns }}</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.action').toUpperCase()" width="190">
          <template #default="{ row }">
            <span class="action-text">{{ actionTextOf(row) }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.frequency').toUpperCase()" width="160">
          <template #default="{ row }">
            <div class="freq-cell">
              <div class="freq-human">{{ frequencyLabelOf(row) }}</div>
              <code class="freq-cron">{{ row.spec?.schedule }}</code>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.lastRunTime').toUpperCase()" width="180">
          <template #default="{ row }">
            <span v-if="row.status?.lastBackup">{{ formatTime(row.status.lastBackup) }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.lastRunStatus').toUpperCase()" width="140">
          <template #default="{ row }">
            <el-tag v-if="row.spec?.paused" type="warning" size="small">{{ t('policies.paused') }}</el-tag>
            <el-tag v-else-if="row.status?.lastBackup" type="success" size="small">{{ t('policies.scheduled') }}</el-tag>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <el-table-column label="" width="60" align="right">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="cmd => handleCommand(cmd, row)">
              <el-button class="more-btn" text>
                <span class="dots">⋮</span>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="view">{{ t('common.view') }}</el-dropdown-item>
                  <el-dropdown-item command="revalidate">{{ t('policies.revalidate') }}</el-dropdown-item>
                  <el-dropdown-item command="edit">{{ t('common.edit') }}</el-dropdown-item>
                  <el-dropdown-item command="editYaml">{{ t('policies.editYaml') }}</el-dropdown-item>
                  <el-dropdown-item command="runOnce" divided>{{ t('policies.runOnce') }}</el-dropdown-item>
                  <el-dropdown-item command="pause">{{ row.spec?.paused ? t('policies.resume') : t('policies.pause') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- View drawer: read-only display of policy spec + status + raw CR -->
    <el-drawer
      v-model="viewDrawerVisible"
      :title="t('policies.viewTitle', { name: viewRow?.metadata?.name })"
      direction="rtl"
      size="640px"
      :destroy-on-close="true"
    >
      <div v-if="viewRow" class="view-body">
        <el-descriptions :column="1" border size="small" class="view-section">
          <el-descriptions-item :label="t('common.name')">
            <code>{{ viewRow.metadata.name }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="t('policies.action')">
            {{ actionTextOf(viewRow) }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('policies.frequency')">
            {{ frequencyLabelOf(viewRow) }} (<code>{{ viewRow.spec?.schedule }}</code>)
          </el-descriptions-item>
          <el-descriptions-item :label="t('common.namespace')">
            {{ (viewRow.spec?.template?.includedNamespaces || ['*']).join(', ') }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('policies.exportRetention')">
            {{ formatTTL(viewRow.spec?.template?.ttl) }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('policies.storageProfile')">
            <code>{{ viewRow.spec?.template?.storageLocation || 'default' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="t('common.status')">
            <el-tag :type="viewRow.spec?.paused ? 'warning' : 'success'" size="small">
              {{ viewRow.spec?.paused ? t('policies.paused') : t('policies.active') }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item v-if="viewRow.status?.lastBackup" :label="t('policies.lastRunTime')">
            {{ formatTime(viewRow.status.lastBackup) }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-drawer>

    <!-- Edit YAML drawer: full CR as YAML, read-only for now -->
    <el-drawer
      v-model="yamlDrawerVisible"
      :title="t('policies.editYaml')"
      direction="rtl"
      size="720px"
      :destroy-on-close="true"
    >
      <div v-if="yamlRow" class="view-body">
        <pre class="yaml-block">{{ asYaml(yamlRow) }}</pre>
        <p class="form-hint" style="margin-top: 12px">
          {{ t('policies.yamlReadonlyHint') }}
        </p>
      </div>
    </el-drawer>

    <!-- Create Policy Dialog (v0.7 Actions model) -->
    <el-drawer
      v-model="showCreateDialog"
      :title="t('policies.newPolicy')"
      direction="rtl"
      size="560px"
      :destroy-on-close="false"
      class="new-policy-drawer"
    >
      <el-form :model="createForm" label-position="top" class="kasten-form">
        <el-form-item required>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('common.name') }}</strong>
              <span class="kasten-label-help">{{ t('policies.nameHelp') }}</span>
            </div>
          </template>
          <el-input v-model="createForm.name" placeholder="daily-backup" />
        </el-form-item>

        <el-form-item>
          <template #label><strong>{{ t('policies.comments') }}</strong></template>
          <el-input
            v-model="createForm.comments"
            type="textarea"
            :rows="2"
            :placeholder="t('policies.commentsPlaceholder')"
          />
        </el-form-item>

        <!-- Action button-group: L1 Snapshot vs L2 Snapshot+Export -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.action') }}</strong>
              <span class="kasten-label-help">{{ t('policies.actionHelp') }}</span>
            </div>
          </template>
          <div class="kasten-pill-group">
            <button
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': !createForm.export.enabled }"
              @click="selectAction('snapshot')"
            >L1 {{ t('policies.actionSnapshot').replace(/^L1\s*/, '') }}</button>
            <button
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': createForm.export.enabled }"
              @click="selectAction('snapshot-export')"
            >L2 {{ t('policies.actionSnapshotExport').replace(/^L2\s*/, '') }}</button>
          </div>
          <p v-if="!createForm.export.enabled" class="action-disabled-warning" style="margin-top: 10px">
            ⚠ {{ t('policies.snapshotOnlyWarn') }}
          </p>
        </el-form-item>

        <!-- Backup Frequency: 6 preset buttons (Kasten parity) -->
        <el-form-item>
          <template #label><strong>{{ t('policies.backupFrequency') }}</strong></template>
          <div class="kasten-pill-grid">
            <button
              v-for="f in frequencyChoices"
              :key="f.key"
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': createForm.frequency === f.key }"
              @click="selectFrequency(f.key)"
            >{{ f.label }}</button>
          </div>
          <el-input
            v-if="createForm.frequency === 'custom'"
            v-model="createForm.snapshot.schedule"
            placeholder="0 * * * *  (cron)"
            style="margin-top: 8px"
          />
        </el-form-item>

        <!-- Snapshot Retention (always shown) -->
        <el-form-item>
          <template #label><strong>{{ t('policies.snapshotRetention') }}</strong></template>
          <el-select v-model="createForm.snapshot.retention" style="width: 100%">
            <el-option label="6 hours" value="6h" />
            <el-option label="12 hours" value="12h" />
            <el-option :label="`24 hours (${t('common.create').toLowerCase() === 'create' ? 'default' : '默认'})`" value="24h" />
            <el-option label="3 days" value="72h" />
            <el-option label="7 days" value="168h" />
          </el-select>
        </el-form-item>

        <!-- Export-only fields (L2 mode) -->
        <template v-if="createForm.export.enabled">
          <el-form-item>
            <template #label><strong>{{ t('policies.exportRetention') }}</strong></template>
            <el-select v-model="createForm.export.retention" style="width: 100%">
              <el-option label="7 days" value="168h" />
              <el-option label="14 days" value="336h" />
              <el-option :label="`30 days (${t('common.create').toLowerCase() === 'create' ? 'default' : '默认'})`" value="720h" />
              <el-option label="60 days" value="1440h" />
              <el-option label="90 days" value="2160h" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <template #label><strong>{{ t('policies.storageProfile') }}</strong></template>
            <el-input v-model="createForm.export.storageLocation" placeholder="default" />
          </el-form-item>
        </template>

        <!-- Volume Mode -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.volumeMode') }}</strong>
              <span class="kasten-label-help">{{ t('policies.volumeModeHelp') }}</span>
            </div>
          </template>
          <div class="kasten-pill-group">
            <button
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': createForm.snapshot.volumeMode === 'filesystem' }"
              @click="createForm.snapshot.volumeMode = 'filesystem'"
            >📁 Filesystem</button>
            <button
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': createForm.snapshot.volumeMode === 'csi' }"
              @click="createForm.snapshot.volumeMode = 'csi'"
            >📸 CSI</button>
          </div>
        </el-form-item>

        <!-- Resources (Included Namespaces) -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.resources') }}</strong>
              <span class="kasten-label-help">{{ t('policies.resourcesHelp') }}</span>
            </div>
          </template>
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            :placeholder="t('policies.namespacesPlaceholder')"
            style="width: 100%"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>

        <!-- Capability detection result (only shown for CSI mode + selected ns) -->
        <el-form-item v-if="createForm.snapshot.volumeMode === 'csi' && createForm.includedNamespaces.length > 0">
          <template #label><strong>{{ t('policies.csiCheck') }}</strong></template>
          <div v-loading="capabilityLoading" class="capability-result">
            <div v-if="capabilityError" class="capability-error">
              ⚠ {{ capabilityError }}
            </div>
            <template v-else-if="capabilityResults.length > 0">
              <div v-for="r in capabilityResults" :key="r.namespace" class="capability-row">
                <div class="capability-ns">
                  <span class="capability-ns-name">{{ r.namespace }}</span>
                  <el-tag v-if="r.incompatibleCount === 0" type="success" size="small">CSI ready</el-tag>
                  <el-tag v-else type="danger" size="small">
                    {{ r.incompatibleCount }} incompatible
                  </el-tag>
                </div>
                <ul v-if="r.incompatibleCount > 0" class="capability-pvc-list">
                  <li v-for="p in r.pvcs.filter(x => !x.csiSnapshot)" :key="p.pvc">
                    <code>{{ p.pvc }}</code> on <code>{{ p.storageClass || '—' }}</code>
                    <span class="capability-reason">— {{ p.reason }}</span>
                  </li>
                </ul>
              </div>
              <div v-if="csiBlocked()" class="capability-blocker">
                ⛔ {{ t('policies.csiBlocker') }}
              </div>
            </template>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="drawer-footer">
          <el-button @click="showCreateDialog = false">{{ t('common.cancel') }}</el-button>
          <el-button
            type="primary"
            @click="handleCreate"
            :loading="creating"
            :disabled="csiBlocked()"
          >
            {{ t('common.create') }}
          </el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Search } from '@element-plus/icons-vue'
import {
  getSchedules, getSchedule, createSchedule, patchSchedule, deleteSchedule,
  runScheduleOnce, getNamespaces, getNamespaceStorageCapability
} from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()

const schedules = ref([])

// Kasten-style filter toolbar state
const actionFilter = ref('all')
const freqFilter = ref('all')
const nameFilter = ref('')
const viewDrawerVisible = ref(false)
const yamlDrawerVisible = ref(false)
const viewRow = ref(null)
const yamlRow = ref(null)

// Derive what action the Velero Schedule performs (Snapshot vs Snapshot+Export)
// from annotations set by Create dialog (v0.7 Actions model).
const actionTextOf = (row) => {
  const ann = row?.metadata?.annotations || {}
  if (ann['supkube.io/export-enabled'] === 'false') return t('policies.actionSnapshot')
  return t('policies.actionSnapshotExport')
}

// Map cron expression to friendly bucket. Same presets as Advisor.
const FREQ_PRESETS = {
  '0 * * * *': 'hourly',
  '0 */6 * * *': 'every6h',
  '0 */12 * * *': 'every12h',
  '0 0 * * *': 'daily',
  '0 0 * * 0': 'weekly',
  '0 0 1 * *': 'monthly'
}
const frequencyKeyOf = (row) => FREQ_PRESETS[(row?.spec?.schedule || '').trim()] || 'custom'
const frequencyLabelOf = (row) => {
  const k = frequencyKeyOf(row)
  if (k === 'custom') return t('advisor.schedule.custom', { cron: row?.spec?.schedule || '' })
  return t(`advisor.schedule.${k}`)
}

// Validation is heuristic — Velero has no single "valid" field. We check
// the schedule has a cron and at least one selector in the template.
const validationOf = (row) => {
  const hasCron = !!row?.spec?.schedule
  const t1 = row?.spec?.template
  const hasTemplate = !!(t1?.includedNamespaces?.length || t1?.includedResources?.length || t1?.labelSelector)
  if (hasCron && hasTemplate) return { key: 'valid', icon: '✓', label: t('policies.valid') }
  return { key: 'invalid', icon: '⚠', label: t('policies.invalid') }
}

const resourceNamespaces = (row) => {
  const ns = row?.spec?.template?.includedNamespaces || []
  return ns.length === 0 ? ['*'] : ns
}

const filteredSchedules = computed(() => {
  const name = nameFilter.value.trim().toLowerCase()
  return schedules.value.filter((row) => {
    if (actionFilter.value !== 'all') {
      const isSnapOnly = row?.metadata?.annotations?.['supkube.io/export-enabled'] === 'false'
      if (actionFilter.value === 'Snapshot' && !isSnapOnly) return false
      if (actionFilter.value === 'Snapshot+Export' && isSnapOnly) return false
    }
    if (freqFilter.value !== 'all') {
      const k = frequencyKeyOf(row)
      if (freqFilter.value === 'hourly' && !['hourly', 'every6h', 'every12h'].includes(k)) return false
      if (freqFilter.value === 'daily' && k !== 'daily') return false
      if (freqFilter.value === 'weekly' && k !== 'weekly') return false
    }
    if (name && !(row.metadata?.name || '').toLowerCase().includes(name)) return false
    return true
  })
})

// Minimal YAML serializer for the View YAML drawer. Strips status churn,
// keeps spec readable. Not for round-trip editing (drawer is read-only in
// v0.7.8; v0.8 swaps in a Monaco editor).
function toYaml(v, indent) {
  const pad = '  '.repeat(indent)
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'string') {
    if (v.includes('\n') || v.includes(':') || v.includes('#')) return JSON.stringify(v)
    return v
  }
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (Array.isArray(v)) {
    if (v.length === 0) return '[]'
    return '\n' + v.map(item => `${pad}- ${toYaml(item, indent + 1).trimStart()}`).join('\n')
  }
  if (typeof v === 'object') {
    const keys = Object.keys(v).filter(k => v[k] !== undefined && v[k] !== null)
    if (keys.length === 0) return '{}'
    return '\n' + keys.map(k => {
      const child = toYaml(v[k], indent + 1)
      if (child.startsWith('\n')) return `${pad}${k}:${child}`
      return `${pad}${k}: ${child}`
    }).join('\n')
  }
  return String(v)
}
const asYaml = (obj) => {
  if (!obj) return ''
  const clean = {
    apiVersion: obj.apiVersion || 'velero.io/v1',
    kind: obj.kind || 'Schedule',
    metadata: {
      name: obj.metadata?.name,
      namespace: obj.metadata?.namespace,
      labels: obj.metadata?.labels,
      annotations: obj.metadata?.annotations
    },
    spec: obj.spec
  }
  return toYaml(clean, 0).trim()
}

const handleCommand = (cmd, row) => {
  switch (cmd) {
    case 'view': openView(row); break
    case 'revalidate': handleRevalidate(row); break
    case 'edit': handleEdit(row); break
    case 'editYaml': openYaml(row); break
    case 'runOnce': handleRunOnce(row); break
    case 'pause': togglePause(row); break
    case 'delete': handleDelete(row); break
  }
}

const openView = async (row) => {
  try {
    const res = await getSchedule(row.metadata.name)
    viewRow.value = res.data
  } catch {
    viewRow.value = row
  }
  viewDrawerVisible.value = true
}

const openYaml = async (row) => {
  try {
    const res = await getSchedule(row.metadata.name)
    yamlRow.value = res.data
  } catch {
    yamlRow.value = row
  }
  yamlDrawerVisible.value = true
}

const handleRevalidate = (row) => {
  const v = validationOf(row)
  if (v.key === 'valid') {
    ElMessage.success(t('policies.revalidateOk'))
  } else {
    ElMessage.warning(t('policies.revalidateFail'))
  }
}

const handleEdit = (row) => {
  // v0.7.8 placeholder — full Edit form lands in v0.8 alongside RBAC.
  // Open the YAML drawer for now so the user can at least inspect the spec.
  ElMessage.info(t('policies.editComingSoon'))
  openYaml(row)
}

const handleRunOnce = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      t('policies.runOnceConfirmBody', { name }),
      t('policies.runOnceConfirmTitle'),
      { confirmButtonText: t('policies.runOnce'), cancelButtonText: t('common.cancel'), type: 'info' }
    )
  } catch { return }
  try {
    const res = await runScheduleOnce(name)
    ElMessage.success(t('policies.runOnceStarted', { backup: res.data?.backupName || '' }))
  } catch (e) {
    ElMessage.error('Run Once failed: ' + (e.response?.data?.error || e.message))
  }
}
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
// (schedules + filter state already declared above near the toolbar logic)

// v0.7 Actions model: Snapshot (always on) + Export (default on, opt-out
// triggers confirmation). Both have independent schedule + retention in the
// UI; v0.7 maps them to a single Velero Schedule with the shorter cron and
// the longer ttl, with intent recorded in annotations for v0.9 to consume.
const defaultForm = () => ({
  name: '',
  comments: '',
  // Top-level frequency choice (Kasten parity). Each preset maps to a
  // snapshot.schedule cron via FREQUENCY_TO_CRON below. Default Daily =
  // safe baseline for most workloads.
  frequency: 'daily',
  includedNamespaces: [],
  snapshot: {
    enabled: true,
    schedulePreset: '0 0 * * *',
    schedule: '0 0 * * *',
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

// Kasten-style frequency preset map. Each option drives BOTH the snapshot
// and export schedule (export stays in lock-step by default — v0.9 will
// expose an "Advanced: separate cadences" toggle).
const FREQUENCY_TO_CRON = {
  hourly: '0 * * * *',
  every6h: '0 */6 * * *',
  daily: '0 0 * * *',
  weekly: '0 0 * * 0',
  monthly: '0 0 1 * *',
  custom: '0 * * * *'
}
const frequencyChoices = computed(() => [
  { key: 'hourly', label: t('advisor.schedule.hourly') },
  { key: 'every6h', label: t('advisor.schedule.every6h') },
  { key: 'daily', label: t('advisor.schedule.daily') },
  { key: 'weekly', label: t('advisor.schedule.weekly') },
  { key: 'monthly', label: t('advisor.schedule.monthly') },
  { key: 'custom', label: t('policies.frequencyCustom') }
])

const selectFrequency = (key) => {
  createForm.value.frequency = key
  const cron = FREQUENCY_TO_CRON[key]
  if (cron) {
    createForm.value.snapshot.schedule = cron
    createForm.value.snapshot.schedulePreset = cron
    createForm.value.export.schedule = cron
    createForm.value.export.schedulePreset = cron
  }
}

// Action button-group handler: snapshot-only vs snapshot+export. Mirrors
// the original Export checkbox toggle behavior (including the guardrail
// confirm dialog when the user disables Export).
const selectAction = (mode) => {
  if (mode === 'snapshot' && createForm.value.export.enabled) {
    // Reuse the existing snapshot-only guardrail dialog.
    onExportToggle(false)
    return
  }
  if (mode === 'snapshot-export' && !createForm.value.export.enabled) {
    createForm.value.export.enabled = true
  }
}

// Capability detection — when user picks CSI mode + selected namespaces, check
// each namespace's PVCs are on CSI-snapshot-capable StorageClasses. We block
// "Create" if any incompatibility found; UI surfaces the offending PVCs.
const capabilityResults = ref([]) // [{ namespace, incompatibleCount, pvcs: [...] }]
const capabilityLoading = ref(false)
const capabilityError = ref('')

const incompatiblePVCs = (capabilityResults.value)
const csiBlocked = () => {
  if (createForm.value.snapshot.volumeMode !== 'csi') return false
  return capabilityResults.value.some(r => r.incompatibleCount > 0)
}

const refreshCapability = async () => {
  capabilityError.value = ''
  const nsList = createForm.value.includedNamespaces
  if (createForm.value.snapshot.volumeMode !== 'csi' || nsList.length === 0) {
    capabilityResults.value = []
    return
  }
  capabilityLoading.value = true
  try {
    const results = []
    for (const ns of nsList) {
      const res = await getNamespaceStorageCapability(ns)
      results.push(res.data)
    }
    capabilityResults.value = results
  } catch (e) {
    capabilityError.value = e.response?.data?.error || e.message
    capabilityResults.value = []
  } finally {
    capabilityLoading.value = false
  }
}

watch(
  () => [createForm.value.includedNamespaces.slice(), createForm.value.snapshot.volumeMode],
  () => { refreshCapability() },
  { deep: false }
)

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
.page-header { align-items: flex-start; }
.page-header-text { flex: 1; }
.page-header h3 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
}
.page-desc {
  margin: 0;
  color: #909399;
  font-size: 13px;
  max-width: 880px;
  line-height: 1.5;
}

/* Kasten-style toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0;
}
.filter-action, .filter-freq { width: 180px; }
.filter-name { width: 260px; }
.filter-spacer { flex: 1; }
.filter-summary { font-size: 13px; color: #606266; }

/* Table cells */
.policy-name { font-weight: 600; color: #303133; }
.validation-cell { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; }
.validation-valid { color: #67c23a; }
.validation-invalid { color: #e6a23c; }
.ns-chip {
  background: #ecf5ff !important;
  border-color: #d9ecff !important;
  color: #409eff !important;
  font-size: 11px;
  font-weight: 500;
  margin-right: 4px;
}
.action-text { font-size: 13px; color: #303133; }
.freq-cell { display: flex; flex-direction: column; gap: 2px; }
.freq-human { font-size: 13px; color: #303133; font-weight: 500; }
.freq-cron {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 11px;
  color: #909399;
  background: transparent;
  padding: 0;
}
.muted { color: #c0c4cc; font-size: 13px; }

/* Kebab */
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }

/* Drawers */
.view-body { padding: 0 4px; }
.view-section { margin-bottom: 16px; }
/* Kasten-style New Policy drawer */
:deep(.new-policy-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 18px 24px;
  border-bottom: 1px solid #ebeef5;
}
:deep(.new-policy-drawer .el-drawer__title) {
  font-size: 18px;
  font-weight: 700;
  color: #1f2329;
  text-align: center;
  width: 100%;
}
:deep(.new-policy-drawer .el-drawer__body) {
  padding: 20px 24px;
}
.kasten-form .el-form-item {
  margin-bottom: 22px;
}
.kasten-label-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.4;
}
.kasten-label-block strong {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.kasten-label-help {
  font-size: 12px;
  color: #909399;
  font-weight: 400;
}
.kasten-pill-group {
  display: flex;
  gap: 0;
  width: 100%;
}
.kasten-pill-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.kasten-pill {
  flex: 1;
  padding: 10px 14px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  cursor: pointer;
  transition: all 0.15s;
}
.kasten-pill-group .kasten-pill:first-child {
  border-radius: 6px 0 0 6px;
}
.kasten-pill-group .kasten-pill:last-child {
  border-radius: 0 6px 6px 0;
  border-left-width: 0;
}
.kasten-pill-grid .kasten-pill {
  border-radius: 6px;
}
.kasten-pill:hover:not(.is-active) {
  border-color: #c0c4cc;
  background: #f5f7fa;
}
.kasten-pill.is-active {
  background: #4f46e5;
  border-color: #4f46e5;
  color: #ffffff;
}
.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 24px;
  border-top: 1px solid #ebeef5;
}

.yaml-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 16px;
  border-radius: 6px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
  overflow-x: auto;
  max-height: calc(100vh - 200px);
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

/* Capability detection result (v0.7.1) */
.capability-result { font-size: 13px; }
.capability-error {
  padding: 8px 12px;
  background: #fef0f0;
  color: #5b2929;
  border-radius: 4px;
}
.capability-row {
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
  margin-bottom: 6px;
}
.capability-ns {
  display: flex;
  align-items: center;
  gap: 8px;
}
.capability-ns-name {
  font-weight: 600;
  color: #303133;
}
.capability-pvc-list {
  margin: 6px 0 0 0;
  padding-left: 20px;
  font-size: 12px;
  color: #606266;
  line-height: 1.7;
}
.capability-pvc-list code {
  background: #ecf0f5;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 11px;
}
.capability-reason { color: #f56c6c; font-style: italic; }
.capability-blocker {
  margin-top: 8px;
  padding: 10px 12px;
  background: #fef0f0;
  border: 1px solid #fbc4c4;
  border-radius: 4px;
  color: #5b2929;
  font-size: 13px;
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
