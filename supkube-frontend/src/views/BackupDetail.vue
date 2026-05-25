<template>
  <div class="backup-detail">
    <div class="page-header">
      <el-button @click="$router.push('/backups')" text>
        ← Back to Restore Points
      </el-button>
      <h3>
        Restore Point: <span class="rp-name">{{ backup?.metadata?.name }}</span>
        <el-tag :type="rpTypeBadge.type" size="small" effect="plain" class="rp-type-tag">
          {{ rpTypeBadge.label }}
        </el-tag>
      </h3>
      <div v-if="fingerprint" class="fingerprint-row">
        <span class="fingerprint-label">Fingerprint:</span>
        <code class="fingerprint-short">{{ fingerprintShort }}</code>
        <el-button text size="small" class="fingerprint-copy" @click="copyFingerprint">
          {{ copied ? '✓ Copied' : 'copy' }}
        </el-button>
      </div>
    </div>

    <!-- Action Layer Cards: Snapshot (L1, local) + Export (L2, object storage) -->
    <el-row :gutter="20" style="margin-bottom: 20px" v-loading="loading">
      <el-col :span="12">
        <el-card class="action-card" :class="`action-card-${snapshotState.tone}`">
          <template #header>
            <div class="action-card-header">
              <span class="action-icon">📸</span>
              <span class="action-title">Snapshot</span>
              <span class="action-subtitle">Local · L1</span>
              <el-tag :type="snapshotState.tagType" size="small" effect="plain" style="margin-left: auto">
                {{ snapshotState.label }}
              </el-tag>
            </div>
          </template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Mode">{{ volumeModeLabel }}</el-descriptions-item>
            <el-descriptions-item label="CSI Volumes">
              <span v-if="hasCSIProgress">
                <span :class="csiAllOk ? 'csi-ok' : 'csi-partial'">
                  {{ backup?.status?.csiVolumeSnapshotsCompleted ?? 0 }}
                </span>
                / {{ backup?.status?.csiVolumeSnapshotsAttempted ?? 0 }} completed
              </span>
              <span v-else class="muted">—</span>
            </el-descriptions-item>
            <el-descriptions-item label="Started">{{ formatTime(backup?.status?.startTimestamp) }}</el-descriptions-item>
            <el-descriptions-item label="Snapshot Location">
              <code v-if="backup?.spec?.volumeSnapshotLocations?.length">{{ backup.spec.volumeSnapshotLocations.join(', ') }}</code>
              <span v-else class="muted">default (Velero auto-selects)</span>
            </el-descriptions-item>
          </el-descriptions>
          <p class="action-caveat" v-if="snapshotState.tone === 'warn'">
            ⚠ Snapshot is not a durable backup. Data is lost if the underlying storage fails.
          </p>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="action-card" :class="`action-card-${exportState.tone}`">
          <template #header>
            <div class="action-card-header">
              <span class="action-icon">📦</span>
              <span class="action-title">Export</span>
              <span class="action-subtitle">Object Storage · L2</span>
              <el-tag :type="exportState.tagType" size="small" effect="plain" style="margin-left: auto">
                {{ exportState.label }}
              </el-tag>
            </div>
          </template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Profile">
              <code>{{ backup?.spec?.storageLocation || 'default' }}</code>
            </el-descriptions-item>
            <el-descriptions-item label="Items">
              {{ backup?.status?.progress?.itemsBackedUp ?? '-' }} / {{ backup?.status?.progress?.totalItems ?? '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Completed">{{ formatTime(backup?.status?.completionTimestamp) }}</el-descriptions-item>
            <el-descriptions-item label="Expires">{{ formatTime(backup?.status?.expiration) }}</el-descriptions-item>
            <el-descriptions-item label="Object Path">
              <code class="object-path">backups/{{ backup?.metadata?.name }}/</code>
            </el-descriptions-item>
          </el-descriptions>
          <p class="action-caveat caveat-success" v-if="exportState.tone === 'ok'">
            ✓ This restore point is a durable backup (data is in object storage).
          </p>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" v-loading="loading">
      <!-- Overview Card -->
      <el-col :span="12">
        <el-card>
          <template #header>Overview</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Name">
              {{ backup?.metadata?.name }}
            </el-descriptions-item>
            <el-descriptions-item label="Namespace">
              {{ backup?.metadata?.namespace }}
            </el-descriptions-item>
            <el-descriptions-item label="Status">
              <el-tag :type="phaseTagType(backup?.status?.phase)">
                {{ normalizePhase(backup?.status?.phase) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Created">
              {{ formatTime(backup?.metadata?.creationTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Started">
              {{ formatTime(backup?.status?.startTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Completed">
              {{ formatTime(backup?.status?.completionTimestamp) }}
            </el-descriptions-item>
            <el-descriptions-item label="Expires">
              {{ formatTime(backup?.status?.expiration) }}
            </el-descriptions-item>
            <el-descriptions-item label="Storage Location">
              {{ backup?.spec?.storageLocation || 'default' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>

      <!-- Progress & Spec Card -->
      <el-col :span="12">
        <el-card>
          <template #header>Backup Spec</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Included Namespaces">
              {{ (backup?.spec?.includedNamespaces || ['*']).join(', ') }}
            </el-descriptions-item>
            <el-descriptions-item label="Excluded Namespaces">
              {{ (backup?.spec?.excludedNamespaces || []).join(', ') || 'None' }}
            </el-descriptions-item>
            <el-descriptions-item label="TTL">
              {{ backup?.spec?.ttl || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Volume Backup Mode">
              <el-tag :type="volumeModeTag" size="small">
                {{ volumeModeLabel }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Items Backed Up">
              {{ backup?.status?.progress?.itemsBackedUp ?? '-' }} / {{ backup?.status?.progress?.totalItems ?? '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Format Version">
              {{ backup?.status?.formatVersion || '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <!-- CSI snapshot progress now lives in the top Snapshot action card (v0.7).
             Keep this slot for future "item operations" detail if needed. -->
        <el-card v-if="backup?.status?.backupItemOperationsAttempted" style="margin-top: 20px">
          <template #header>Item Operations</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Operations">
              {{ backup.status.backupItemOperationsCompleted ?? 0 }} / {{ backup.status.backupItemOperationsAttempted }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card style="margin-top: 20px">
          <template #header>Actions</template>
          <el-space>
            <el-button type="primary" @click="restoreFromBackup">
              Restore from this Backup
            </el-button>
            <el-button type="danger" @click="handleDelete">
              Delete Backup
            </el-button>
          </el-space>
        </el-card>
      </el-col>
    </el-row>

    <!-- v0.8.6 Backup Composition — answers the "what's in this backup
         and how big is it?" questions the v0.8.5 retrospective raised.
         Pure read of the new `supkube` enrichment from /backups/:name. -->
    <el-card v-if="backup?.supkube" style="margin-top: 20px">
      <template #header>
        <span>{{ t('backupDetail.composition.title') }}</span>
        <span class="composition-hint">{{ t('backupDetail.composition.hint') }}</span>
      </template>
      <el-row :gutter="20">
        <el-col :span="8">
          <div class="composition-block">
            <div class="composition-label">{{ t('restorePoints.dataPath') }}</div>
            <span class="data-path-chip" :class="`data-path-${backup.supkube.dataPath}`">
              {{ dataPathIcon }} {{ t(`restorePoints.dataPathLabel.${backup.supkube.dataPath}`) }}
            </span>
            <p class="composition-explain">
              {{ t(`restorePoints.dataPathHelp.${backup.supkube.dataPath}`) }}
            </p>
          </div>
        </el-col>
        <el-col :span="8">
          <div class="composition-block">
            <div class="composition-label">{{ t('backupDetail.composition.volumeData') }}</div>
            <div class="composition-number">{{ formatBytes(backup.supkube.volumeBytes) }}</div>
            <p class="composition-explain">
              <template v-if="backup.supkube.volumeCount > 0">
                {{ t('backupDetail.composition.volumeBreakdown', { count: backup.supkube.volumeCount }) }}
              </template>
              <template v-else>
                {{ t('backupDetail.composition.volumeEmpty') }}
              </template>
            </p>
            <p v-if="backup.supkube.dataPath === 'csi-snapshot'" class="composition-caveat">
              ⚠ {{ t('backupDetail.composition.csiCaveat') }}
            </p>
          </div>
        </el-col>
        <el-col :span="8">
          <div class="composition-block">
            <div class="composition-label">{{ t('backupDetail.composition.tarball') }}</div>
            <div class="composition-number">
              <span v-if="backup.supkube.tarballBytes">{{ formatBytes(backup.supkube.tarballBytes) }}</span>
              <span v-else class="composition-muted">—</span>
            </div>
            <p class="composition-explain">
              {{ t('backupDetail.composition.tarballHelp') }}
            </p>
            <p v-if="backup.supkube.tarballError" class="composition-caveat">
              ⚠ {{ backup.supkube.tarballError }}
            </p>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- Labels & Annotations -->
    <el-card style="margin-top: 20px" v-if="backup?.metadata?.labels">
      <template #header>Labels</template>
      <el-tag
        v-for="(value, key) in backup?.metadata?.labels"
        :key="key"
        style="margin-right: 8px; margin-bottom: 8px"
      >
        {{ key }}={{ value }}
      </el-tag>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getBackup, deleteBackup } from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'
import { normalizePhase, phaseTagType } from '../utils/phase'

const { t } = useI18n()

// v0.8.6: shared formatters mirrored from Backups.vue. We deliberately
// duplicate (rather than extracting to a util module) so this view's
// rendering doesn't break when the list page changes its helper shapes.
// Six lines of copy-paste vs. a coupling risk that bites later.
const formatBytes = (n) => {
  if (n === undefined || n === null || n === 0) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let v = Number(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  const fixed = i >= 2 ? v.toFixed(1) : Math.round(v)
  return `${fixed} ${units[i]}`
}
const dataPathIcon = computed(() => {
  switch (backup.value?.supkube?.dataPath) {
    case 'csi-snapshot':  return '📸'
    case 'data-mover':    return '🚚'
    case 'filesystem':    return '📁'
    case 'metadata-only': return '📋'
    default:              return '❔'
  }
})

const route = useRoute()
const router = useRouter()
const backup = ref(null)
const loading = ref(false)

// Volume backup mode derivation from spec (read-only display).
// Velero spec: snapshotVolumes true → CSI; defaultVolumesToFsBackup true → FS;
// both nil/false → no volume backup at all.
const volumeModeLabel = computed(() => {
  const spec = backup.value?.spec || {}
  if (spec.snapshotVolumes === true) return '📸 CSI Snapshot'
  if (spec.defaultVolumesToFsBackup === true) return '📁 Filesystem (Restic/Kopia)'
  if (spec.snapshotVolumes === false && !spec.defaultVolumesToFsBackup) return 'No volumes'
  return 'Default'
})
const volumeModeTag = computed(() => {
  const spec = backup.value?.spec || {}
  if (spec.snapshotVolumes === true) return 'primary'
  if (spec.defaultVolumesToFsBackup === true) return 'success'
  if (spec.snapshotVolumes === false) return 'info'
  return ''
})
const hasCSIProgress = computed(() => {
  const s = backup.value?.status
  return s && (s.csiVolumeSnapshotsAttempted || s.csiVolumeSnapshotsCompleted)
})
const csiAllOk = computed(() => {
  const s = backup.value?.status
  if (!s) return true
  return (s.csiVolumeSnapshotsCompleted || 0) >= (s.csiVolumeSnapshotsAttempted || 0)
})

// --- v0.7 Actions Model: Restore Point as Snapshot (L1) + Export (L2) ---
// State for each layer is derived from the same Velero Backup spec/status.
// In v0.7, every Velero Backup IS an "Export" (Velero always writes the
// metadata tarball to BSL). Snapshot layer reflects the volume-data handling
// step. v0.9 will properly decouple these via self-managed snapshot
// scheduler — but the UI mental model starts here.

// Velero hides the source K8s cluster UID in spec.metadata; fall back to the
// resource UID + creationTimestamp + storage location, hashed.
async function sha256Hex(s) {
  const enc = new TextEncoder().encode(s)
  const buf = await crypto.subtle.digest('SHA-256', enc)
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}
const fingerprint = ref('')
const fingerprintShort = computed(() => fingerprint.value.slice(0, 16))
const copied = ref(false)
async function computeFingerprint() {
  const b = backup.value
  if (!b) { fingerprint.value = ''; return }
  const seed = [
    b.metadata?.uid || '',
    b.metadata?.creationTimestamp || '',
    b.spec?.storageLocation || 'default'
  ].join('|')
  fingerprint.value = await sha256Hex(seed)
}
function copyFingerprint() {
  if (!fingerprint.value) return
  navigator.clipboard.writeText(fingerprint.value).then(() => {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  })
}

// Restore Point Type — v0.7 derived from labels/spec. Kasten:
//   Local Snapshot   = snapshot-only (in v0.7 not actually achievable via
//                      Velero, but we surface the intent)
//   Exported Backup  = the normal case: Velero wrote tarball + (optional)
//                      moved CSI snapshot data to BSL
//   Imported Backup  = synced from BSL by another cluster's Velero (we tag
//                      via annotation supkube.io/imported-from when the
//                      import flow lands in v0.7-import)
const rpType = computed(() => {
  const ann = backup.value?.metadata?.annotations || {}
  if (ann['supkube.io/imported-from']) return 'Imported'
  // Velero always exports; "snapshot only" requires self-managed scheduler
  // (v0.9). For now anything with status.completionTimestamp is Exported.
  if (backup.value?.status?.completionTimestamp) return 'Exported'
  return 'Snapshot'
})
const rpTypeBadge = computed(() => {
  switch (rpType.value) {
    case 'Imported': return { label: 'Imported Backup', type: 'info' }
    case 'Exported': return { label: 'Exported Backup', type: 'success' }
    default: return { label: 'Local Snapshot', type: 'warning' }
  }
})

// Snapshot layer status: derived from CSI counters + phase
const snapshotState = computed(() => {
  const phase = backup.value?.status?.phase
  const s = backup.value?.status || {}
  const isFS = backup.value?.spec?.defaultVolumesToFsBackup === true
  if (!phase) return { label: 'Pending', tagType: 'info', tone: 'pending' }
  if (phase === 'InProgress') return { label: 'In Progress', tagType: 'warning', tone: 'pending' }
  if (isFS) return { label: 'N/A (filesystem mode)', tagType: 'info', tone: 'info' }
  const attempted = s.csiVolumeSnapshotsAttempted || 0
  const completed = s.csiVolumeSnapshotsCompleted || 0
  if (attempted === 0) return { label: 'No volumes', tagType: 'info', tone: 'info' }
  if (completed >= attempted) return { label: `Completed (${completed}/${attempted})`, tagType: 'success', tone: 'ok' }
  return { label: `Partial (${completed}/${attempted})`, tagType: 'danger', tone: 'warn' }
})
const exportState = computed(() => {
  const phase = backup.value?.status?.phase
  if (!phase) return { label: 'Pending', tagType: 'info', tone: 'pending' }
  if (phase === 'InProgress') return { label: 'Uploading', tagType: 'warning', tone: 'pending' }
  if (phase === 'Completed') return { label: 'Completed', tagType: 'success', tone: 'ok' }
  if (phase === 'PartiallyFailed') return { label: 'Partial', tagType: 'warning', tone: 'warn' }
  return { label: phase, tagType: 'danger', tone: 'warn' }
})

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const fetchBackup = async () => {
  loading.value = true
  try {
    const res = await getBackup(route.params.name)
    backup.value = res.data
    await computeFingerprint()
  } catch (e) {
    ElMessage.error('Failed to load backup details')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const restoreFromBackup = () => {
  // v0.8.0: route to Activity Restore filter (legacy used /restores).
  router.push({ path: '/activity', query: { type: 'Restore' } })
}

const handleDelete = async () => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete backup "${backup.value?.metadata?.name}"?`,
      'Delete Backup',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteBackup(backup.value.metadata.name)
    ElMessage.success('Backup deleted')
    router.push('/backups')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete backup')
    }
  }
}

onMounted(() => {
  fetchBackup()
})
</script>

<style scoped>
.page-header {
  margin-bottom: 16px;
}
.page-header h3 {
  margin: 8px 0 0 0;
  display: flex;
  align-items: center;
  gap: 10px;
}
.rp-name {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 16px;
  color: var(--sk-text-secondary);
}
.rp-type-tag {
  margin-left: 4px;
}
.fingerprint-row {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}
.fingerprint-label {
  color: var(--sk-text-caption);
}
.fingerprint-short {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  background: #f5f7fa;
  padding: 2px 8px;
  border-radius: 4px;
  color: var(--sk-text-muted);
  font-size: 11px;
}
.fingerprint-copy {
  font-size: 12px !important;
  padding: 2px 8px !important;
}

/* Action layer cards */
.action-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}
.action-icon {
  font-size: 20px;
}
.action-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--sk-text-secondary);
}
.action-subtitle {
  font-size: 11px;
  color: var(--sk-text-caption);
  font-weight: 500;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.action-caveat {
  margin: 12px 0 0 0;
  padding: 8px 12px;
  font-size: 12px;
  background: #fef0f0;
  border-radius: 4px;
  color: #5b2929;
  line-height: 1.5;
}
.caveat-success {
  background: #f0f9eb;
  color: #225a17;
}
.action-card-ok { border-left: 3px solid #67c23a; }
.action-card-warn { border-left: 3px solid #e6a23c; }
.action-card-pending { border-left: 3px solid #909399; }
.action-card-info { border-left: 3px solid #909399; }

.object-path {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11px;
  color: var(--sk-text-muted);
  background: #f5f7fa;
  padding: 1px 6px;
  border-radius: 3px;
}
.muted { color: var(--sk-text-placeholder); }
.csi-ok { color: var(--sk-status-success); font-weight: 600; }
.csi-partial { color: var(--sk-status-warning); font-weight: 600; }
.csi-warn { color: var(--sk-status-error); font-size: 12px; margin-left: 6px; }

/* v0.8.6 Backup Composition panel */
.composition-hint {
  margin-left: 12px;
  font-size: 12px;
  color: var(--sk-text-caption);
  font-weight: 400;
}
.composition-block {
  padding: 8px 0;
}
.composition-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--sk-text-caption);
  margin-bottom: 8px;
}
.composition-number {
  font-size: 26px;
  font-weight: 600;
  color: var(--sk-text);
  font-family: 'SF Mono', Menlo, monospace;
  line-height: 1.2;
  margin-bottom: 8px;
}
.composition-explain {
  font-size: 12px;
  color: var(--sk-text-muted);
  line-height: 1.5;
  margin: 0;
}
.composition-caveat {
  font-size: 12px;
  color: var(--sk-status-error);
  background: #fdf3f4;
  padding: 6px 10px;
  border-radius: 6px;
  margin: 8px 0 0 0;
  line-height: 1.4;
}
.composition-muted { color: var(--sk-text-placeholder); font-weight: 500; }

/* Shared chip style with list view — duplicated rather than extracted
   so this view is self-contained and a chip-rename in the list can't
   accidentally break the detail page. */
.data-path-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 500;
  padding: 3px 10px;
  border-radius: 12px;
  margin-bottom: 8px;
}
.data-path-csi-snapshot  { color: #409eff; background: #ecf5ff; }
.data-path-data-mover    { color: #722ed1; background: #f4ecfd; }
.data-path-filesystem    { color: var(--sk-status-success); background: #f0f9eb; }
.data-path-metadata-only { color: var(--sk-text-caption); background: #f4f4f5; }
.data-path-unknown       { color: var(--sk-text-placeholder); background: #f5f7fa; }
</style>
