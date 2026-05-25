<template>
  <div class="restore-points-page">
    <div class="page-header">
      <h3>{{ t('restorePoints.title') }}</h3>
      <p class="page-desc">{{ t('restorePoints.desc') }}</p>
    </div>

    <!-- Application type pills (Kasten parity; only Namespace enabled for now) -->
    <div class="apptype-section">
      <div class="apptype-label">{{ t('restorePoints.applicationType') }}</div>
      <div class="apptype-pills">
        <button class="apptype-pill is-active" type="button">
          <el-icon><Box /></el-icon> {{ t('restorePoints.namespace') }}
        </button>
        <button class="apptype-pill is-disabled" type="button" disabled title="Coming in v0.7+">
          <el-icon><Monitor /></el-icon> {{ t('restorePoints.virtualMachine') }}
        </button>
      </div>
    </div>

    <!-- v0.7.13: "intent=restore" banner. Shown when the user came from
         Applications → Restore — orients them that this list is a stepping
         stone (pick a Restore Point → kebab → Restore). -->
    <div v-if="restoreIntentActive" class="intent-banner">
      <el-icon class="intent-icon"><MagicStick /></el-icon>
      <div class="intent-body">
        <div class="intent-title">{{ t('restorePoints.intentRestoreTitle', { ns: nsFilter }) }}</div>
        <div class="intent-desc">{{ t('restorePoints.intentRestoreDesc') }}</div>
      </div>
      <button class="intent-dismiss" type="button" @click="dismissIntent">×</button>
    </div>

    <!-- v0.7.13: Active filter chips, Kasten-style. Shown whenever any
         dimensional filter (namespace, etc.) is active. Each chip × removes
         that one filter; "Clear Filters" nukes them all. -->
    <div v-if="activeChips.length > 0" class="chips-row">
      <span class="chips-label">{{ t('common.filters') }}:</span>
      <button class="chip" v-for="chip in activeChips" :key="chip.key" type="button" @click="clearChip(chip.key)">
        <el-icon><Box /></el-icon>
        <span>{{ chip.value }}</span>
        <span class="chip-x">×</span>
      </button>
      <button class="clear-filters-link" type="button" @click="clearAllChips">
        {{ t('common.clearFilters') }}
      </button>
    </div>

    <!-- Filter / search / bulk toolbar -->
    <div class="filter-toolbar">
      <!-- v0.8.10 简化：Type 列三态互斥（Snapshot / Exported / Imported）取代旧的
           Type + Source + Data Path 三列。Source 折进 Type；Data Path 移到 Type
           chip 的 tooltip（CSI / Data Mover / Filesystem 给运维看）。 -->
      <el-select v-model="typeFilter" class="filter-type">
        <el-option :label="t('restorePoints.allTypes')" value="all" />
        <el-option :label="`📸 ${t('restorePoints.typeSnapshot')}`" value="snapshot" />
        <el-option :label="`🚚 ${t('restorePoints.typeExported')}`" value="exported" />
        <el-option :label="`🌐 ${t('restorePoints.typeImported')}`" value="imported" /></el-select>
      <el-input v-model="nameFilter" :placeholder="t('restorePoints.filterPlaceholder')" clearable class="filter-name">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="filter-spacer"></span>
      <span class="filter-summary" v-html="viewingHtml"></span>
      <!-- v0.8.5 step 3: Disable mutating actions if user lacks role. -->
      <el-button
        :disabled="selectedRows.length === 0 || !auth.canDo('backup.delete')"
        :type="selectedRows.length === 0 ? '' : 'danger'"
        :title="!auth.canDo('backup.delete') ? t('common.noPermission') : ''"
        @click="handleDeleteSelected"
      >
        {{ t('common.deleteSelected') }} ({{ selectedRows.length }})
      </el-button>
      <el-button
        type="primary"
        :disabled="!auth.canDo('backup.create')"
        :title="!auth.canDo('backup.create') ? t('common.noPermission') : ''"
        @click="showCreateDialog = true"
      >
        <el-icon><Plus /></el-icon> {{ t('restorePoints.create') }}
      </el-button>
    </div>

    <el-card>
      <el-table
        :data="filteredBackups"
        style="width: 100%"
        v-loading="loading"
        :default-sort="{ prop: 'metadata.creationTimestamp', order: 'descending' }"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="48" />

        <el-table-column label="Namespace" min-width="240" sortable>
          <template #default="{ row }">
            <div class="ns-cell">
              <span class="ns-name">{{ formatNamespace(row) }}</span>
              <span class="rp-name">{{ row.metadata?.name }}</span>
              <!-- v0.8.10.4: Type + Status chips merged INTO the
                   Namespace cell. Two separate columns were a wasteful
                   pattern — both are 1-3 word labels. No emoji per
                   UI_GUIDELINES §3.1 (chip allowed at most 1; we omit). -->
              <div class="ns-cell-chips">
                <el-tooltip :content="typeChipTooltip(row)" placement="top" :show-after="300">
                  <span class="sk-chip" :class="`sk-chip-type-${typePill(row).key}`">
                    {{ typePill(row).label }}
                  </span>
                </el-tooltip>
                <span class="sk-chip" :class="`sk-chip-status-${statusChipKey(row.status?.phase)}`">
                  {{ normalizePhase(row.status?.phase) }}
                </span>
                <!-- v0.8.11.2: App Items chip moved INTO this cell.
                     The Velero "32/32" progress count was removed per
                     user request — it's mostly noise (totalItems can
                     drift across runs; itemsBackedUp == totalItems
                     when Completed which is always). Real signal is
                     applicationItems, shown here as a chip. -->
                <el-tooltip
                  v-if="row.supkube?.applicationItems != null"
                  :content="t('restorePoints.applicationItemsTooltip')"
                  placement="top"
                  :show-after="200"
                >
                  <span class="sk-chip sk-chip-status-muted">
                    {{ row.supkube.applicationItems }} {{ t('restorePoints.applicationItemsChip') }}
                  </span>
                </el-tooltip>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('restorePoints.policy')" min-width="180">
          <template #default="{ row }">
            <!-- v0.8.10 Policy column: schedule label → policy name link;
                 otherwise "Instant Snapshot" badge. No emoji. -->
            <span v-if="policyOf(row)" class="policy-link" @click="goToPolicy(row)">
              {{ policyOf(row) }}
            </span>
            <el-tooltip v-else placement="top" :show-after="200">
              <template #content>
                <div v-if="instantSnapshotMeta(row).user">
                  {{ t('restorePoints.manualSnapshotBy', { user: instantSnapshotMeta(row).user }) }}
                </div>
                <div v-else>{{ t('restorePoints.instantSnapshotGeneric') }}</div>
                <div v-if="instantSnapshotMeta(row).comment" style="margin-top:4px;opacity:.8">
                  {{ instantSnapshotMeta(row).comment }}
                </div>
              </template>
              <span class="manual-snapshot-badge">
                {{ t('restorePoints.instantSnapshot') }}
              </span>
            </el-tooltip>
          </template>
        </el-table-column>

        <el-table-column :label="t('restorePoints.profile')" min-width="140">
          <template #default="{ row }">
            <!-- v0.8.10.4: clickable profile name → jump to /storage-locations.
                 Snapshot RPs still show "—" because the BSL only holds
                 their metadata tarball, not the volume data. -->
            <el-tooltip
              v-if="typePill(row).key === 'snapshot'"
              :content="t('restorePoints.profileSnapshotTooltip')"
              placement="top"
              :show-after="200"
            >
              <span class="muted">—</span>
            </el-tooltip>
            <a
              v-else-if="row.spec?.storageLocation"
              class="policy-link"
              @click="goToStorageProfile(row.spec.storageLocation)"
            >{{ row.spec.storageLocation }}</a>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <!-- v0.8.11.2: dedicated App Items column REMOVED — the chip
             moved into the Namespace cell. Velero progress (32/32)
             also dropped from that cell as it was noise. -->

        <!-- v0.8.10.4: Size column — "actual / reserved" format.
             actual = bytes Velero/Kopia processed (DataUpload / PVB
                      progress; or VSC.restoreSize when present)
             reserved = sum of source PVC requests.storage (computed
                        live on the backend regardless of dataPath)
             For Snapshot RPs whose VSC was auto-deleted by Velero v1.18,
             `actual` is unavailable → shown as "—". `reserved` always
             renders if the source ns still exists. Volumes count is
             gone (was noise). Tooltip explains the caveat. -->
        <el-table-column :label="t('restorePoints.size')" width="160">
          <template #default="{ row }">
            <el-tooltip :content="sizeTooltip(row)" placement="top" :show-after="200">
              <span class="size-cell">
                <span class="size-actual">{{ formatBytesOrDash(row.supkube?.volumeBytes) }}</span>
                <span class="size-sep">/</span>
                <span class="size-reserved">{{ formatBytesOrDash(row.supkube?.reservedBytes) }}</span>
              </span>
            </el-tooltip>
          </template>
        </el-table-column>

        <!-- v0.8.10.5: date / time stacked on two lines so the column
             can shrink to ~120px. Single-line "5/23/2026, 10:14:44 AM"
             previously demanded ~180-200px. -->
        <el-table-column :label="t('restorePoints.createdAt')" width="120" prop="metadata.creationTimestamp" sortable :sort-method="sortByCreated">
          <template #default="{ row }">
            <div class="stacked-time">
              <div class="sk-body">{{ formatDate(row.metadata?.creationTimestamp) }}</div>
              <div class="sk-caption">{{ formatTimeOnly(row.metadata?.creationTimestamp) }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('restorePoints.expiresAt')" width="120">
          <template #default="{ row }">
            <div v-if="row.status?.expiration" class="stacked-time">
              <div class="sk-body">{{ formatDate(row.status.expiration) }}</div>
              <div class="sk-caption">{{ formatTimeOnly(row.status.expiration) }}</div>
            </div>
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
                  <el-dropdown-item command="restore">{{ t('common.restore') }}</el-dropdown-item>
                  <el-dropdown-item command="export" disabled :title="t('common.comingSoon')">{{ t('common.export') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Restore Point Dialog -->
    <el-dialog v-model="showCreateDialog" :title="t('restorePoints.create')" width="500px">
      <el-form :model="createForm" label-width="180px">
        <el-form-item label="Name" required>
          <el-input v-model="createForm.name" placeholder="my-restore-point" />
        </el-form-item>
        <el-form-item label="Included Namespaces">
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="All namespaces (default)"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>
        <el-form-item label="Excluded Namespaces">
          <el-select
            v-model="createForm.excludedNamespaces"
            multiple
            filterable
            allow-create
            placeholder="None"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>
        <el-form-item label="Label Selector">
          <el-input v-model="createForm.labelSelectorStr" placeholder="app=mysql,env=prod" />
          <span class="form-hint">Comma-separated key=value pairs to filter resources</span>
        </el-form-item>
        <el-form-item label="TTL (Retention)">
          <el-input v-model="createForm.ttl" placeholder="720h (30 days)" />
        </el-form-item>
        <el-form-item label="Storage Location (Profile)">
          <el-input v-model="createForm.storageLocation" placeholder="default" />
        </el-form-item>
        <el-form-item label="Include Volumes">
          <el-switch v-model="createForm.snapshotVolumes" />
        </el-form-item>
        <el-form-item v-if="createForm.snapshotVolumes" label="Volume Backup Mode">
          <el-radio-group v-model="createForm.volumeMode">
            <el-radio-button value="filesystem">📁 Filesystem (Restic/Kopia)</el-radio-button>
            <el-radio-button value="csi">📸 CSI Snapshot</el-radio-button>
          </el-radio-group>
          <span class="form-hint">
            <strong>Filesystem</strong>: works with any StorageClass, slower but most compatible.
            <strong>CSI</strong>: faster, requires the PVC's StorageClass to support CSI snapshots
            (e.g. <code>csi-hostpath-sc</code>).
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">Create</el-button>
      </template>
    </el-dialog>

    <!-- v0.7.10: Kasten-style Restore drawer. Opens from the kebab "Restore" -->
    <RestoreDrawer
      v-model:visible="restoreDrawerOpen"
      :backup="restoreTarget"
      @restored="onRestoreSubmitted"
    />

    <!-- v0.8.10.1: ActionDetailDrawer reused for the RP ⋮ → View flow.
         Same component the Activity page uses → consistent UX, single
         place to maintain. @navigate handles the Paired-with banner
         click (swaps to peer Backup without closing). -->
    <ActionDetailDrawer
      v-model:visible="detailDrawerOpen"
      :action="detailDrawerAction"
      entity-title-key="activity.detail.titleRestorePoint"
      @navigate="handleDetailDrawerNavigate"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'

const { t } = useI18n()
// "Viewing N out of M Restore Points" with bold counts. Goes through v-html
// so the inserted HTML survives translation.
const viewingHtml = computed(() =>
  t('restorePoints.viewing', {
    filtered: `<strong>${filteredBackups.value.length}</strong>`,
    total: backups.value.length
  })
)
import { Plus, Search, Box, Monitor, MagicStick } from '@element-plus/icons-vue'
import { getBackups, createBackup, deleteBackup, getNamespaces, getAction } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'
import { normalizePhase, phaseTagType } from '../utils/phase'
import RestoreDrawer from '../components/RestoreDrawer.vue'
import ActionDetailDrawer from '../components/ActionDetailDrawer.vue'
import { useAuth } from '../composables/useAuth'

const auth = useAuth()

const router = useRouter()
const route = useRoute()
const backups = ref([])
const namespaces = ref([])
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)

// Filter / selection state
// v0.8.10: typeFilter now holds the typePill().key value
// ('snapshot' / 'exported' / 'imported' / 'metadata' / 'unknown' / 'all').
// The pre-v0.8.10 sourceFilter has been folded into Type (Imported wins).
const typeFilter = ref('all')
const nameFilter = ref('')
// v0.8.10.1: deep-link from Policies page. /backups?policy=<name>
// pre-applies this chip, scoping the list to RPs from that policy
// (both halves of a dual pair).
const policyFilter = ref('')
// v0.7.13: namespace chip filter (deep-linked from Applications → Restore).
// Distinct from the free-text nameFilter so both can be active at once and
// the chip is removable as a discrete unit.
const nsFilter = ref('')
const restoreIntentActive = ref(false)
const selectedRows = ref([])

// v0.7.10 Restore drawer state. Opened by kebab "Restore"; the row data
// flows in via :backup so the drawer can read includedNamespaces, name, etc.
const restoreDrawerOpen = ref(false)
const restoreTarget = ref(null)

let pollTimer = null

const createForm = ref({
  name: '',
  includedNamespaces: [],
  excludedNamespaces: [],
  labelSelectorStr: '',
  ttl: '720h',
  storageLocation: 'default',
  snapshotVolumes: true,
  // v0.6: 'filesystem' = Restic/Kopia fs backup (defaultVolumesToFsBackup=true)
  //       'csi'        = CSI snapshot (snapshotVolumes=true + plain spec)
  volumeMode: 'filesystem'
})

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}
// v0.8.10.5: split date / time for the two-line stacked Created At /
// Expires At cells. Locale-aware via Intl.
const formatDate = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleDateString()
}
const formatTimeOnly = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// policyOf — the Velero schedule-name label is the source of truth for
// "this RP came from a Policy". v0.8.10 treats anything else as a single
// equivalence class: an Instant Snapshot.
const policyOf = (row) => row?.metadata?.labels?.['velero.io/schedule-name'] || ''

// v0.8.10: unified Instant-Snapshot tooltip metadata. Works for BOTH the
// Application-Snapshot button (carries supkube.io/created-by-user +
// supkube.io/comment) AND the generic POST /backups full form (which has
// neither, so we return blanks — template falls back to a generic tooltip).
const instantSnapshotMeta = (row) => {
  const ann = row?.metadata?.annotations || {}
  return {
    user:    ann['supkube.io/created-by-user'] || '',
    comment: ann['supkube.io/comment'] || ''
  }
}

// v0.8.10 typePill — three mutually-exclusive states for the TYPE column,
// replacing the v0.8.9 rolePill + the separate Source column.
//
//   imported  → 🌐 Imported   (synced from another cluster's BSL)
//                Imported wins over snapshot/exported because the user
//                can't pick the underlying mechanism — they're using
//                whatever the origin cluster produced.
//   snapshot  → 📸 Snapshot   (cluster-local CSI snapshot)
//   exported  → 🚚 Exported   (BSL via Data Mover or Filesystem backup)
//   unknown   → ❔ Unknown    (defensive; should never appear in practice)
//
// Technical detail (csi-snapshot vs data-mover vs filesystem) moved to
// the chip's tooltip — most customers don't read it; the operators who
// care can hover.
const typePill = (row) => {
  if (sourceOf(row) === 'Imported') {
    return { key: 'imported', icon: '🌐', label: t('restorePoints.typeImported') }
  }
  const dp = row?.supkube?.dataPath
  switch (dp) {
    case 'csi-snapshot':
      return { key: 'snapshot', icon: '📸', label: t('restorePoints.typeSnapshot') }
    case 'data-mover':
    case 'filesystem':
      return { key: 'exported', icon: '🚚', label: t('restorePoints.typeExported') }
    case 'metadata-only':
      return { key: 'metadata', icon: '📋', label: t('restorePoints.typeMetadata') }
    default:
      return { key: 'unknown',  icon: '❔', label: t('restorePoints.typeUnknown') }
  }
}

// typeChipTooltip — explains the underlying mechanism for power users.
// Composes Source (Local / Imported) + Data Path (CSI / Data Mover /
// Filesystem) into one tooltip block so we can drop two whole columns
// without losing the information.
const typeChipTooltip = (row) => {
  const src = sourceOf(row)
  const dp = row?.supkube?.dataPath || 'unknown'
  const parts = []
  if (src === 'Imported') {
    parts.push(t('restorePoints.tooltipImported'))
  } else {
    parts.push(t('restorePoints.tooltipLocal'))
  }
  const dpLabel = t(`restorePoints.dataPathLabel.${dp}`) || dp
  parts.push(`${t('restorePoints.dataPath')}: ${dpLabel}`)
  return parts.join(' · ')
}

// Source detection: Velero annotates every Backup with
// velero.io/source-cluster-k8s-gitversion at create time. When a different
// cluster's Velero syncs a Backup from a shared BSL into this cluster, the
// annotation still reflects the ORIGIN cluster — we compare it against the
// version recorded for backups created in this cluster (cached on first load).
const currentClusterFingerprint = ref('')

function backupClusterFingerprint(row) {
  const ann = row?.metadata?.annotations || {}
  // Available since Velero v1.10. Combination of fields is more reliable
  // than any single one when clusters share the same K8s version.
  return [
    ann['velero.io/source-cluster-k8s-gitversion'] || '',
    ann['velero.io/source-cluster-k8s-major-version'] || '',
    ann['velero.io/source-cluster-k8s-minor-version'] || ''
  ].join('|')
}

const sourceOf = (row) => {
  const fp = backupClusterFingerprint(row)
  if (!fp || fp === '||') return 'Local' // no annotations → assume local
  if (!currentClusterFingerprint.value) return 'Local' // baseline not set yet
  return fp === currentClusterFingerprint.value ? 'Local' : 'Imported'
}

const sourceTooltip = (row) => {
  const ann = row?.metadata?.annotations || {}
  const ver = ann['velero.io/source-cluster-k8s-gitversion'] || 'unknown'
  if (sourceOf(row) === 'Local') {
    return `Created in this cluster (K8s ${ver})`
  }
  return `Synced from another cluster (origin K8s ${ver}). Imported via shared Storage Profile.`
}

// ─── v0.8.6: Backup composition (dataPath + size) ───────────────────────
//
// dataPath answers "is this CSI snapshot / filesystem backup / data mover
// / metadata-only?" — pulled from the new `supkube` enrichment the backend
// attaches to each row. We render it as a colored chip with a tooltip
// explaining the storage implications (e.g. CSI = cluster-local, can't
// move across clusters without enabling Data Mover).
const dataPathOf = (row) => row?.supkube?.dataPath || 'unknown'

const dataPathLabel = (row) => {
  const dp = dataPathOf(row)
  return t(`restorePoints.dataPathLabel.${dp}`) || dp
}

const dataPathIcon = (row) => {
  switch (dataPathOf(row)) {
    case 'csi-snapshot':  return '📸'  // hardware snapshot
    case 'data-mover':    return '🚚'  // moved to object storage
    case 'filesystem':    return '📁'  // walked the FS
    case 'metadata-only': return '📋'  // YAML only
    default:              return '❔'
  }
}

const dataPathTooltip = (row) => t(`restorePoints.dataPathHelp.${dataPathOf(row)}`) || ''

// formatBytes renders a byte count using K8s-ish units (Ki/Mi/Gi/Ti).
// Falls back to "—" for missing/zero so the column looks clean.
const formatBytes = (n) => {
  if (n === undefined || n === null || n === 0) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let v = Number(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  // 1 decimal place above MiB, integer below — matches what kubectl shows.
  const fixed = i >= 2 ? v.toFixed(1) : Math.round(v)
  return `${fixed} ${units[i]}`
}
// v0.8.10.4: same as formatBytes but returns '—' explicitly for null
// AND zero, so the "actual / reserved" Size cell renders "— / 6 GiB"
// instead of "0 B / 6 GiB" when the actual figure is unavailable.
const formatBytesOrDash = formatBytes

const sizeTooltip = (row) => {
  const sk = row?.supkube
  if (!sk) return t('restorePoints.sizeTooltip.unknown')
  const lines = []
  // actual = volumeBytes (data Velero/Kopia moved or restoreSize where available)
  lines.push(t('restorePoints.sizeTooltip.actual', {
    size: sk.volumeBytes ? formatBytes(sk.volumeBytes) : t('restorePoints.sizeTooltip.actualUnavailable')
  }))
  // reserved = sum of source PVC requests.storage
  if (sk.reservedBytes) {
    lines.push(t('restorePoints.sizeTooltip.reserved', { size: formatBytes(sk.reservedBytes) }))
  }
  if (sk.tarballBytes) {
    lines.push(t('restorePoints.sizeTooltip.tarball', { size: formatBytes(sk.tarballBytes) }))
  }
  if (sk.tarballError) {
    lines.push(t('restorePoints.sizeTooltip.tarballError', { reason: sk.tarballError }))
  }
  return lines.join('\n')
}

// v0.8.10.4: map Velero phase → sk-chip-status-* class key so the
// Status chip embedded in the Namespace cell follows the same global
// taxonomy as Activity / Actions.
const statusChipKey = (phase) => {
  const p = (phase || '').toLowerCase()
  if (p === 'completed')                         return 'success'
  if (p === 'inprogress' || p === 'new' || p === '') return 'running'
  if (p === 'failed' || p === 'failedvalidation')return 'error'
  if (p === 'partiallyfailed')                   return 'warning'
  return 'muted'
}

// Restore Point row primarily represents a namespace's protected state at a
// point in time. Show the most informative namespace label; * = all.
const formatNamespace = (row) => {
  const ns = row?.spec?.includedNamespaces || []
  if (ns.length === 0) return '*'
  if (ns.length === 1) return ns[0]
  return ns.join(', ')
}

// v0.8.10.1: match a Backup row against a Policy name. Three label
// shapes have to pass:
//   1. v0.8.9+ dual: labels[supkube.io/policy-name] === <name>  ← most
//      reliable; works regardless of snapshot/export half
//   2. v0.8.8 legacy single-Schedule: labels[velero.io/schedule-name]
//      === <name>
//   3. v0.8.9 dual export half (in case supkube.io/policy-name label
//      didn't get copied by Velero v1.15+ schedule-controller): the
//      schedule-name label === <name>-export
const matchesPolicy = (row, policyName) => {
  if (!policyName) return true
  const labels = row?.metadata?.labels || {}
  if (labels['supkube.io/policy-name'] === policyName) return true
  const sn = labels['velero.io/schedule-name']
  if (sn === policyName || sn === policyName + '-export') return true
  return false
}

const filteredBackups = computed(() => {
  const name = nameFilter.value.trim().toLowerCase()
  const ns = nsFilter.value.trim()
  const policy = policyFilter.value.trim()
  return backups.value.filter((row) => {
    // v0.8.10: typeFilter now matches typePill().key (snapshot / exported /
    // imported / metadata / unknown). Source filter merged into Type.
    if (typeFilter.value !== 'all' && typePill(row).key !== typeFilter.value) {
      return false
    }
    // v0.8.10.1: Policy chip filter — same dimensional pattern as ns chip.
    if (policy && !matchesPolicy(row, policy)) return false
    // v0.7.13 chip filter: exact-namespace match. Backup must include the
    // namespace in its spec.includedNamespaces (or it backed up everything
    // and the ns lives in formatNamespace()'s output).
    if (ns) {
      const included = row.spec?.includedNamespaces || []
      // includedNamespaces empty = whole-cluster backup; we still show it.
      if (included.length > 0 && !included.includes(ns)) return false
    }
    if (name) {
      const haystack = [
        row.metadata?.name,
        formatNamespace(row),
        policyOf(row)
      ].filter(Boolean).join(' ').toLowerCase()
      if (!haystack.includes(name)) return false
    }
    return true
  })
})

// v0.7.13 chip system — keyed list of dimensional filters currently active.
// Each chip is { key, value, label }; clearChip(key) removes that one.
const activeChips = computed(() => {
  const out = []
  if (nsFilter.value) out.push({ key: 'ns', value: nsFilter.value })
  // v0.8.10.1: Policy chip — shown with a 📋 prefix so a glance
  // distinguishes "namespace chip" from "policy chip" even when both
  // happen to have similar names.
  if (policyFilter.value) out.push({ key: 'policy', value: '📋 ' + policyFilter.value })
  return out
})
const clearChip = (key) => {
  if (key === 'ns') {
    nsFilter.value = ''
    restoreIntentActive.value = false
    // Strip the query string so a refresh doesn't re-apply the chip.
    router.replace({ path: '/backups', query: {} })
  } else if (key === 'policy') {
    policyFilter.value = ''
    router.replace({ path: '/backups', query: {} })
  }
}
const clearAllChips = () => {
  nsFilter.value = ''
  policyFilter.value = ''
  typeFilter.value = 'all'
  nameFilter.value = ''
  restoreIntentActive.value = false
  router.replace({ path: '/backups', query: {} })
}
const dismissIntent = () => {
  restoreIntentActive.value = false
}

const sortByCreated = (a, b) => {
  const at = new Date(a.metadata?.creationTimestamp || 0).getTime()
  const bt = new Date(b.metadata?.creationTimestamp || 0).getTime()
  return at - bt
}

// Establish "this cluster's" Velero source fingerprint by taking the most
// common fingerprint from manual (non-imported, non-scheduled-from-elsewhere)
// backups. Fallback: most frequent fingerprint overall. Backups annotated
// with that fingerprint are Local, the rest are Imported.
function setLocalFingerprint(items) {
  const counts = new Map()
  for (const b of items) {
    const fp = backupClusterFingerprint(b)
    if (fp && fp !== '||') counts.set(fp, (counts.get(fp) || 0) + 1)
  }
  let best = ''
  let bestCount = 0
  for (const [fp, count] of counts) {
    if (count > bestCount) { best = fp; bestCount = count }
  }
  currentClusterFingerprint.value = best
}

const fetchBackups = async () => {
  loading.value = true
  try {
    const res = await getBackups()
    const items = res.data.items || []
    // Newest first by default — matches user expectation that the most
    // recent restore point is what you usually want to look at. Stable
    // sort by creationTimestamp desc; falls back to backup name as tie
    // breaker so identical timestamps are still deterministic.
    items.sort((a, b) => {
      const at = new Date(a.metadata?.creationTimestamp || 0).getTime()
      const bt = new Date(b.metadata?.creationTimestamp || 0).getTime()
      if (at !== bt) return bt - at
      return (b.metadata?.name || '').localeCompare(a.metadata?.name || '')
    })
    backups.value = items
    setLocalFingerprint(backups.value)
  } catch (e) {
    ElMessage.error('Failed to load restore points')
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

const parseLabelSelector = (str) => {
  if (!str || !str.trim()) return undefined
  const labels = {}
  str.split(',').forEach(pair => {
    const [key, value] = pair.trim().split('=')
    if (key && value) labels[key.trim()] = value.trim()
  })
  return Object.keys(labels).length > 0 ? labels : undefined
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(fetchBackups, 5000)
}
const stopPolling = () => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

const handleCreate = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Please enter a name')
    return
  }
  creating.value = true
  try {
    // Map UI volume mode to Velero spec:
    //   - filesystem: defaultVolumesToFsBackup=true → Restic/Kopia uploader
    //   - csi:        snapshotVolumes=true, no fs backup flag → CSI snapshotter
    // If the user turned off "Include Volumes" entirely, both are false.
    const includeVols = createForm.value.snapshotVolumes
    const isCSI = includeVols && createForm.value.volumeMode === 'csi'
    const isFS = includeVols && createForm.value.volumeMode === 'filesystem'

    const payload = {
      name: createForm.value.name,
      includedNamespaces: createForm.value.includedNamespaces.length > 0 ? createForm.value.includedNamespaces : undefined,
      excludedNamespaces: createForm.value.excludedNamespaces.length > 0 ? createForm.value.excludedNamespaces : undefined,
      labelSelector: parseLabelSelector(createForm.value.labelSelectorStr),
      ttl: createForm.value.ttl || '720h',
      storageLocation: createForm.value.storageLocation || 'default',
      snapshotVolumes: isCSI,
      defaultVolumesToFsBackup: isFS
    }
    await createBackup(payload)
    ElMessage.success(`Restore point "${createForm.value.name}" created (${isCSI ? 'CSI Snapshot' : isFS ? 'Filesystem' : 'no volumes'}). Monitoring progress...`)
    showCreateDialog.value = false
    createForm.value = {
      name: '', includedNamespaces: [], excludedNamespaces: [], labelSelectorStr: '',
      ttl: '720h', storageLocation: 'default', snapshotVolumes: true, volumeMode: 'filesystem'
    }
    await fetchBackups()
    startPolling()
  } catch (e) {
    ElMessage.error('Failed to create restore point: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const handleCommand = (cmd, row) => {
  switch (cmd) {
    case 'view': viewDetail(row); break
    case 'restore': restoreFromBackup(row); break
    case 'export': /* placeholder — v0.8 */ break
    case 'delete': handleDelete(row); break
  }
}

// Wired by the drawer's @restored. Refresh the table so the user sees the
// new Restore CR appear in /restores (we also nudge them there optionally).
const onRestoreSubmitted = (_name) => {
  fetchBackups()
}

const handleDelete = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox({
      title: t('restorePoints.deleteTitle'),
      // dangerouslyUseHTMLString so we can render the per-asset bullet list.
      message: t('restorePoints.deleteConfirmBody', { name }) +
        `<ul style="margin: 8px 0 0 0; padding-left: 18px; font-size: 13px; line-height: 1.7;">
          <li>${t('restorePoints.deleteBullet1')}</li>
          <li>${t('restorePoints.deleteBullet2')}</li>
          <li>${t('restorePoints.deleteBullet3')}</li>
          <li>${t('restorePoints.deleteBullet4')}</li>
         </ul>
         <p style="margin: 10px 0 0 0; color: #c45656; font-size: 13px;">
           ⚠ ${t('restorePoints.deleteIrreversible')}
         </p>`,
      dangerouslyUseHTMLString: true,
      showCancelButton: true,
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
      confirmButtonClass: 'el-button--danger'
    })
  } catch { return }
  try {
    await deleteBackup(name)
    ElMessage.success(t('restorePoints.deleteStarted', { name }))
    await fetchBackups()
  } catch (e) {
    ElMessage.error('Failed to delete: ' + (e.response?.data?.error || e.message))
  }
}

const handleDeleteSelected = async () => {
  const rows = selectedRows.value.slice()
  if (rows.length === 0) return
  try {
    await ElMessageBox.confirm(
      `Delete ${rows.length} restore point${rows.length > 1 ? 's' : ''}?`,
      'Bulk Delete',
      { confirmButtonText: `Delete ${rows.length}`, cancelButtonText: 'Cancel', type: 'warning' }
    )
  } catch { return }
  let okCount = 0, failCount = 0
  for (const row of rows) {
    try {
      await deleteBackup(row.metadata.name)
      okCount++
    } catch (e) {
      failCount++
      console.error(`Failed to delete ${row.metadata.name}:`, e)
    }
  }
  if (failCount === 0) {
    ElMessage.success(`Deleted ${okCount} restore point${okCount > 1 ? 's' : ''}`)
  } else {
    ElMessage.warning(`Deleted ${okCount}, failed ${failCount}`)
  }
  await fetchBackups()
}

// Validate: a "dry-run" integrity check. v0.6 will invoke a real
// Velero BackupDescribe + DownloadRequest cycle; for now we just confirm
// the underlying Backup CR is still Completed and surface that to the user.
const handleValidate = (row) => {
  const phase = row?.status?.phase
  if (phase === 'Completed') {
    ElMessage.success(`Restore point "${row.metadata.name}" looks valid (phase=Completed). Full integrity check coming in v0.6.`)
  } else {
    ElMessage.warning(`Phase is ${phase || 'Unknown'} — may not be restorable. Full validate coming in v0.6.`)
  }
}

const handleSelectionChange = (rows) => {
  selectedRows.value = rows
}

// v0.7.10: open the Kasten-style side drawer instead of routing away. The
// drawer drives the whole restore flow (target ns, overwrite confirm, spec
// artifact selection, existingResourcePolicy) in one place.
const restoreFromBackup = (row) => {
  restoreTarget.value = row
  restoreDrawerOpen.value = true
}

// v0.8.10.1: ⋮ → View opens the Kasten-style ActionDetailDrawer
// in-place instead of routing to /backups/:name. Same component the
// Activity page uses — Artifacts table now shows the grouped breakdown
// (Workloads / Configuration / Networking / Storage / RBAC / Snapshot CRs).
// Loading is async because the drawer expects an Action-shape summary;
// we fetch it via /actions/:id?type=Backup and then open. On failure we
// fall back to the legacy route so the user always reaches their data.
const detailDrawerOpen = ref(false)
const detailDrawerAction = ref(null)
const viewDetail = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    const res = await getAction(name, 'Backup')
    detailDrawerAction.value = res.data?.action || res.data
    detailDrawerOpen.value = true
  } catch (e) {
    console.warn('Action fetch for drawer failed; falling back to legacy detail page:', e)
    router.push({ path: `/backups/${name}` })
  }
}
// Paired-with navigation from inside the drawer — swap to peer Action
// without closing. Mirrors Activity.vue's handlePairedNavigate.
const handleDetailDrawerNavigate = async (ref) => {
  if (!ref || ref.kind !== 'Backup') return
  try {
    const res = await getAction(ref.name, 'Backup')
    detailDrawerAction.value = res.data?.action || res.data
  } catch (e) {
    ElMessage.error('Paired Backup unavailable: ' + (e?.response?.data?.error || e.message))
  }
}

const goToPolicy = (row) => {
  const policy = policyOf(row)
  if (policy) router.push({ path: '/policies', query: { name: policy } })
}
// v0.8.10.4: Profile cell click → Storage Locations page filtered to
// this BSL. Same UX pattern as the Policy column.
const goToStorageProfile = (bslName) => {
  if (bslName) router.push({ path: '/storage', query: { name: bslName } })
}

onMounted(() => {
  // v0.8.10: deep-link from Dashboard's Imported card / BSL details
  // Source-filter chip now lands on /backups?type=imported (preferred)
  // or /backups?source=Imported (legacy, still honored). Both map onto
  // the new typeFilter.
  if (route.query.type === 'imported' || route.query.type === 'snapshot' || route.query.type === 'exported') {
    typeFilter.value = String(route.query.type)
  } else if (route.query.source === 'Imported') {
    typeFilter.value = 'imported'
  }
  // v0.7.13: Deep-link from Applications page. The kebab "Restore" routes
  // here with ?namespace=<app>&intent=restore. We pre-apply the chip and
  // show the orientation banner.
  if (route.query.namespace) {
    nsFilter.value = String(route.query.namespace)
  }
  // v0.8.10.1: Deep-link from Policies page RP-count click. The chip
  // matches any Backup whose policy-name label OR schedule-name label
  // ties back to this policy (handles legacy + dual-pair shapes).
  if (route.query.policy) {
    policyFilter.value = String(route.query.policy)
  }
  if (route.query.intent === 'restore') {
    restoreIntentActive.value = true
  }
  fetchBackups()
  fetchNamespaces()
})
onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page-header { margin-bottom: 20px; }
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

/* Application type pills */
.apptype-section {
  margin-bottom: 18px;
}
.apptype-label {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}
.apptype-pills {
  display: flex;
  gap: 10px;
}
.apptype-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  cursor: pointer;
  transition: all 0.15s;
}
.apptype-pill:hover:not(.is-disabled):not(.is-active) {
  border-color: #c0c4cc;
  background: #f5f7fa;
}
.apptype-pill.is-active {
  background: #4f46e5;
  border-color: #4f46e5;
  color: #ffffff;
}
.apptype-pill.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Filter toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.filter-type { width: 180px; }
.filter-name { width: 280px; }
.filter-spacer { flex: 1; }
.filter-summary {
  color: #606266;
  font-size: 13px;
}
.filter-summary strong { color: #303133; font-weight: 600; }

/* Form helper */
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

/* Table cells */
.ns-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
}
.ns-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--sk-text);
}
.rp-name {
  font-size: 11px;
  color: var(--sk-text-caption);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
/* v0.8.10.4: chips row inside the Namespace cell. Type + Status pills
   merged here so two separate columns can be dropped. Wraps if needed
   on narrow viewports. */
.ns-cell-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-xs);
  align-items: center;
  margin-top: 2px;
}
.ns-cell-progress {
  margin-left: 2px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.type-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
}
/* v0.8.10.2: token-backed. See tokens.css for palette. */
.type-snapshot  { color: var(--sk-type-snapshot); }
.type-scheduled { color: var(--sk-status-success); }
.type-exported  { color: var(--sk-type-exported); }
.type-metadata  { color: var(--sk-type-metadata); }
.type-unknown   { color: var(--sk-text-placeholder); }
.type-imported  { color: var(--sk-type-imported); }
.source-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
}
.source-local { color: #606266; background: #f5f7fa; }
.source-imported { color: #c45656; background: #fdf3f4; }

/* v0.8.6: data-path chip — same shape as source/type chips so the row
   stays visually consistent. Color coding hints at the storage trade-off
   that path implies (see USER_MANUAL §15 for the user-facing semantics). */
.data-path-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
  white-space: nowrap;
}
.data-path-csi-snapshot  { color: #409eff; background: #ecf5ff; }  /* cluster-local CoW */
.data-path-data-mover    { color: #722ed1; background: #f4ecfd; }  /* moved to object store */
.data-path-filesystem    { color: #67c23a; background: #f0f9eb; }  /* Restic/Kopia */
.data-path-metadata-only { color: #909399; background: #f4f4f5; }  /* YAML only */
.data-path-unknown       { color: #c0c4cc; background: #f5f7fa; }

/* v0.8.10.4 Size column: "actual / reserved" format.
   actual   = what Velero/Kopia moved (volumeBytes)
   reserved = sum of source PVC requested.storage (reservedBytes)
   Mono font for visual alignment across rows. Volume count dropped
   (was noise per UI_GUIDELINES § implicit). */
.size-cell {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 13px;
}
.size-actual   { color: var(--sk-text); font-weight: 500; }
.size-sep      { color: var(--sk-text-placeholder); }
.size-reserved { color: var(--sk-text-caption); }
.policy-link {
  color: var(--sk-primary);
  cursor: pointer;
  font-size: 13px;
  transition: color 120ms ease;
}
.policy-link:hover {
  color: var(--sk-primary-hover);
  text-decoration: underline;
}
.profile-cell {
  font-size: 13px;
  color: #606266;
}
.items-mini {
  margin-left: 6px;
  font-size: 11px;
  color: #909399;
  font-family: 'SF Mono', Menlo, monospace;
}
.muted { color: var(--sk-text-placeholder); font-size: 13px; }

/* v0.8.10.5: two-line stacked date/time cell. */
.stacked-time {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}
.stacked-time .sk-body    { font-weight: 500; }
.stacked-time .sk-caption { font-size: 11px; }

/* v0.8.9.2: badge for one-click App-Snapshot RPs. Visually distinct
   from "(manual)" muted text — gives the audit/operator angle some
   weight without being as loud as the policy-link blue (which would
   imply "click me to navigate"). Cursor stays default; the tooltip is
   the affordance. */
.manual-snapshot-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  color: #6741d9;
  background: #f3eefe;
  border: 1px solid #e4d8fa;
  cursor: default;
}

/* Kebab action button */
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }

/* v0.7.13 — Intent banner shown when arriving from Applications → Restore */
.intent-banner {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 14px;
  background: linear-gradient(90deg, #eef2ff 0%, #f5f7fa 100%);
  border: 1px solid #c7d2fe;
  border-radius: 8px;
}
.intent-icon { color: #4f46e5; font-size: 22px; margin-top: 2px; flex-shrink: 0; }
.intent-body { flex: 1; }
.intent-title { font-weight: 600; font-size: 14px; color: #312e81; }
.intent-desc { font-size: 12.5px; color: #4338ca; line-height: 1.5; margin-top: 2px; }
.intent-dismiss {
  background: none;
  border: 0;
  color: #6366f1;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  padding: 0 4px;
}
.intent-dismiss:hover { color: #312e81; }

/* v0.7.13 — Active chip filter row (Kasten-parity) */
.chips-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.chips-label {
  font-size: 12.5px;
  color: #606266;
  font-weight: 500;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: #eef2ff;
  border: 1px solid #c7d2fe;
  border-radius: 14px;
  color: #4338ca;
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}
.chip:hover { background: #ddd6fe; }
.chip-x {
  margin-left: 2px;
  font-size: 15px;
  line-height: 1;
  color: #6366f1;
  font-weight: bold;
}
.clear-filters-link {
  background: none;
  border: 0;
  color: #4f46e5;
  font-size: 13px;
  cursor: pointer;
  padding: 4px 0;
}
.clear-filters-link:hover { text-decoration: underline; }

/* Dark mode */
:deep(html.dark) .intent-banner {
  background: linear-gradient(90deg, #1e1b4b 0%, #1f2026 100%);
  border-color: #3730a3;
}
:deep(html.dark) .intent-title { color: #c7d2fe; }
:deep(html.dark) .intent-desc { color: #a5b4fc; }
:deep(html.dark) .chip { background: #1e1b4b; border-color: #3730a3; color: #c7d2fe; }
:deep(html.dark) .chip:hover { background: #312e81; }
</style>
