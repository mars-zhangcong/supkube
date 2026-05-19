<template>
  <div class="storage-page">
    <div class="page-header">
      <h3>Storage Locations</h3>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        Add Storage Location
      </el-button>
    </div>

    <el-card>
      <el-table :data="locations" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" sortable />
        <el-table-column label="Provider">
          <template #default="{ row }">
            {{ row.spec?.provider || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Bucket">
          <template #default="{ row }">
            {{ row.spec?.objectStorage?.bucket || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Region">
          <template #default="{ row }">
            {{ row.spec?.config?.region || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="row.status?.phase === 'Available' ? 'success' : 'danger'">
              {{ row.status?.phase || 'Unknown' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Default">
          <template #default="{ row }">
            <el-tag v-if="row.spec?.default" type="primary" size="small">Default</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Last Validated">
          <template #default="{ row }">
            {{ formatTime(row.status?.lastValidationTime) }}
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
                  <el-dropdown-item command="verify">Verify</el-dropdown-item>
                  <el-dropdown-item command="details">View Details</el-dropdown-item>
                  <el-dropdown-item command="edit">Edit</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>Delete</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- View Details Drawer -->
    <el-drawer
      v-model="detailsVisible"
      title="Storage Location Details"
      direction="rtl"
      size="540px"
      :destroy-on-close="true"
      class="storage-details-drawer"
    >
      <div v-if="selectedRow" class="details-body">
        <div class="details-title">
          <span class="storage-icon">🗄</span>
          <span>{{ selectedRow.metadata?.name }}</span>
          <el-tag v-if="selectedRow.spec?.default" type="primary" size="small" style="margin-left: 8px">Default</el-tag>
        </div>

        <div class="detail-block">
          <div class="detail-block-title">STATUS</div>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Phase">
              <el-tag :type="selectedRow.status?.phase === 'Available' ? 'success' : 'danger'" size="small">
                {{ selectedRow.status?.phase || 'Unknown' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Last Validated">
              {{ formatTime(selectedRow.status?.lastValidationTime) }}
            </el-descriptions-item>
            <el-descriptions-item v-if="selectedRow.status?.message" label="Message">
              <span style="color: #f56c6c; word-break: break-word">{{ selectedRow.status.message }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- v0.7.2 Sync status — Velero auto-syncs every backupSyncPeriod (default 60s) -->
        <div class="detail-block">
          <div class="detail-block-title">SYNC</div>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Last Synced">
              <span v-if="selectedRow.status?.lastSyncedTime">
                {{ formatTime(selectedRow.status.lastSyncedTime) }}
                <span class="sync-relative">· {{ relativeTime(selectedRow.status.lastSyncedTime) }} ago</span>
              </span>
              <span v-else class="muted">Never (waiting for first sync)</span>
            </el-descriptions-item>
            <el-descriptions-item label="Sync Schedule">
              <span class="muted">Auto · every 60s (Velero default)</span>
            </el-descriptions-item>
            <el-descriptions-item label="Backups Found">
              {{ syncedBackupCount }} restore point{{ syncedBackupCount === 1 ? '' : 's' }} from this profile
            </el-descriptions-item>
          </el-descriptions>
          <p class="sync-hint">
            💡 Velero polls this object-storage profile every 60s. Restore Points
            written by another cluster's Velero into the same bucket will appear
            here automatically — look for "Imported" in the Source column on the
            <a class="sync-link" @click="goToRestorePoints">Restore Points</a> page.
          </p>
        </div>

        <div class="detail-block">
          <div class="detail-block-title">SPEC</div>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Provider">{{ selectedRow.spec?.provider || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Bucket">{{ selectedRow.spec?.objectStorage?.bucket || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Region">{{ selectedRow.spec?.config?.region || '-' }}</el-descriptions-item>
            <el-descriptions-item label="S3 Endpoint">{{ selectedRow.spec?.config?.s3Url || '-' }}</el-descriptions-item>
            <el-descriptions-item label="S3 Force Path Style">
              {{ selectedRow.spec?.config?.s3ForcePathStyle === 'true' ? 'Enabled' : 'Disabled' }}
            </el-descriptions-item>
            <el-descriptions-item label="Credentials Secret">
              <code v-if="selectedRow.spec?.credential?.name">{{ selectedRow.spec.credential.name }}</code>
              <span v-else style="color: #909399">none (uses Velero default credentials)</span>
            </el-descriptions-item>
            <el-descriptions-item label="Created">
              {{ formatTime(selectedRow.metadata?.creationTimestamp) }}
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <div class="detail-actions">
          <el-button type="primary" @click="openEdit(selectedRow)">Edit</el-button>
          <el-button @click="handleVerify(selectedRow)" :loading="verifying">Verify Now</el-button>
        </div>
      </div>
    </el-drawer>

    <!-- Edit Storage Location Dialog -->
    <el-dialog v-model="editVisible" title="Edit Storage Location" width="550px">
      <el-form :model="editForm" label-width="160px">
        <el-form-item label="Name">
          <el-input v-model="editForm.name" disabled />
          <span class="form-hint">Name is immutable. To rename, delete and recreate.</span>
        </el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="editForm.provider" style="width: 100%" @change="onEditProviderChange">
            <el-option label="AWS S3" value="aws" />
            <el-option label="MinIO (S3 Compatible)" value="minio" />
            <el-option label="SupVault" value="supvault" />
            <el-option label="GCP" value="gcp" />
            <el-option label="Azure" value="azure" />
          </el-select>
        </el-form-item>
        <el-form-item label="Bucket" required>
          <el-input v-model="editForm.bucket" />
        </el-form-item>
        <el-form-item label="Region">
          <el-input v-model="editForm.region" placeholder="us-east-1" />
        </el-form-item>
        <el-form-item label="S3 Endpoint">
          <el-input v-model="editForm.s3Url" placeholder="http://minio.minio:9000" />
        </el-form-item>
        <el-form-item label="S3 Force Path">
          <el-switch v-model="editForm.s3ForcePathStyle" />
          <span style="margin-left: 8px; color: #999; font-size: 12px">Enable for MinIO / SupVault</span>
        </el-form-item>
        <el-form-item label="New Access Key">
          <el-input v-model="editForm.accessKey" placeholder="Leave empty to keep existing" />
        </el-form-item>
        <el-form-item label="New Secret Key">
          <el-input v-model="editForm.secretKey" type="password" placeholder="Leave empty to keep existing" show-password />
        </el-form-item>
        <p class="form-hint" style="margin: 0 0 0 160px">
          Saving triggers automatic re-validation.
        </p>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleUpdate" :loading="updating">Save</el-button>
      </template>
    </el-dialog>

    <!-- Create Storage Location Dialog -->
    <el-dialog v-model="showCreateDialog" title="Add Storage Location" width="550px">
      <el-form :model="createForm" label-width="140px">
        <el-form-item label="Name" required :error="nameError">
          <el-input v-model="createForm.name" placeholder="my-s3-storage" />
          <span class="form-hint">
            Lowercase letters, digits, '-' or '.'; must start and end with a letter or digit.
          </span>
        </el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="createForm.provider" style="width: 100%" @change="onProviderChange">
            <el-option label="AWS S3" value="aws" />
            <el-option label="MinIO (S3 Compatible)" value="minio" />
            <el-option label="SupVault" value="supvault" />
            <el-option label="GCP" value="gcp" />
            <el-option label="Azure" value="azure" />
          </el-select>
        </el-form-item>
        <el-form-item label="Bucket" required>
          <el-input v-model="createForm.bucket" placeholder="my-backup-bucket" />
        </el-form-item>
        <el-form-item label="Region">
          <el-input v-model="createForm.region" placeholder="us-east-1" />
        </el-form-item>
        <el-form-item label="S3 Endpoint">
          <el-input v-model="createForm.s3Url" placeholder="http://minio.minio:9000 (for MinIO)" />
        </el-form-item>
        <el-form-item label="S3 Force Path">
          <el-switch v-model="createForm.s3ForcePathStyle" />
          <span style="margin-left: 8px; color: #999; font-size: 12px">Enable for MinIO / SupVault</span>
        </el-form-item>
        <el-form-item label="Access Key">
          <el-input v-model="createForm.accessKey" placeholder="Access Key ID" />
        </el-form-item>
        <el-form-item label="Secret Key">
          <el-input v-model="createForm.secretKey" type="password" placeholder="Secret Access Key" show-password />
        </el-form-item>
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
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import {
  getStorageLocations,
  getStorageLocation,
  createStorageLocation,
  updateStorageLocation,
  deleteStorageLocation,
  verifyStorageLocation,
  getBackups
} from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const locations = ref([])
const backupsForSync = ref([])
const loading = ref(false)
const creating = ref(false)
const verifying = ref(false)
const updating = ref(false)
const showCreateDialog = ref(false)
const detailsVisible = ref(false)
const editVisible = ref(false)
const selectedRow = ref(null)

const editForm = ref({
  name: '',
  provider: 'aws',
  bucket: '',
  region: '',
  s3Url: '',
  s3ForcePathStyle: false,
  accessKey: '',
  secretKey: ''
})

const createForm = ref({
  name: '',
  provider: 'aws',
  bucket: '',
  region: '',
  s3Url: '',
  s3ForcePathStyle: false,
  accessKey: '',
  secretKey: ''
})

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

// Human-readable "X minutes ago" / "X hours ago" relative to now, used for
// the BSL sync status. Velero's default sync period is 60s, so the line goes
// stale quickly — keep the formatter tight (no seconds for >2 min).
const relativeTime = (ts) => {
  if (!ts) return ''
  const sec = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`
  return `${Math.floor(sec / 86400)}d`
}

// Count how many backups (= Velero Backup CRs) are stored against the
// currently-viewed BSL. Used in the Sync block to show "N restore points from
// this profile" — gives the user a concrete confidence signal that sync is
// working (and surfaces drift if the count looks wrong).
const syncedBackupCount = computed(() => {
  const name = selectedRow.value?.metadata?.name
  if (!name) return 0
  return backupsForSync.value.filter(b => (b.spec?.storageLocation || 'default') === name).length
})

const goToRestorePoints = () => {
  detailsVisible.value = false
  router.push('/backups')
}

// K8s DNS-1123 subdomain rule — both the BSL metadata.name and the derived
// Secret name must satisfy this; validate up front to give a clear message
// instead of waiting for the K8s API to reject it.
const RFC1123_SUBDOMAIN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/
const nameError = computed(() => {
  const v = createForm.value.name
  if (!v) return ''
  if (v.length > 253) return 'Name too long (max 253 characters)'
  if (!RFC1123_SUBDOMAIN.test(v)) {
    return "Use lowercase letters/digits/'-'/'.'; must start and end with a letter or digit"
  }
  return ''
})

// S3-compatible UI choices (MinIO, SupVault, ...) all map to Velero's `aws`
// provider plugin; the distinction is only for UX selection clarity and
// auto-enabling path-style addressing. Add new S3-compatible vendors here.
const S3_COMPATIBLE_UI = new Set(['minio', 'supvault'])
const toVeleroProvider = (uiProvider) =>
  S3_COMPATIBLE_UI.has(uiProvider) ? 'aws' : uiProvider

const onProviderChange = (val) => {
  // Path-style addressing is required by MinIO and SupVault; enable by default.
  if (S3_COMPATIBLE_UI.has(val)) {
    createForm.value.s3ForcePathStyle = true
  }
}

const fetchLocations = async () => {
  loading.value = true
  try {
    const res = await getStorageLocations()
    locations.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load storage locations')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleCreate = async () => {
  if (!createForm.value.name || !createForm.value.bucket) {
    ElMessage.warning('Please fill in name and bucket')
    return
  }
  if (nameError.value) {
    ElMessage.warning(nameError.value)
    return
  }
  creating.value = true
  try {
    const payload = {
      name: createForm.value.name,
      provider: toVeleroProvider(createForm.value.provider),
      bucket: createForm.value.bucket,
      region: createForm.value.region || undefined,
      s3Url: createForm.value.s3Url || undefined,
      s3ForcePathStyle: createForm.value.s3ForcePathStyle,
      accessKey: createForm.value.accessKey || undefined,
      secretKey: createForm.value.secretKey || undefined
    }
    await createStorageLocation(payload)
    ElMessage.success(`Storage location "${createForm.value.name}" created`)
    showCreateDialog.value = false
    createForm.value = {
      name: '',
      provider: 'aws',
      bucket: '',
      region: '',
      s3Url: '',
      s3ForcePathStyle: false,
      accessKey: '',
      secretKey: ''
    }
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to create storage location: ' + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}

const handleVerify = async (row) => {
  verifying.value = true
  try {
    const res = await verifyStorageLocation(row.metadata?.name || row.name)
    const phase = res.data?.phase || res.data?.status?.phase || 'Unknown'
    if (phase === 'Available') {
      ElMessage.success(`Storage location "${row.metadata?.name || row.name}" is available`)
    } else {
      ElMessage.warning(`Storage location "${row.metadata?.name || row.name}" status: ${phase}`)
    }
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to verify storage location: ' + (e.response?.data?.error || e.message))
  } finally {
    verifying.value = false
  }
}

const handleCommand = (cmd, row) => {
  switch (cmd) {
    case 'verify': handleVerify(row); break
    case 'details': openDetails(row); break
    case 'edit': openEdit(row); break
    case 'delete': handleDelete(row); break
  }
}

const openDetails = async (row) => {
  // Fetch fresh copy so the drawer reflects latest status, not just the row
  // data which can be a few seconds stale from the last list poll.
  try {
    const res = await getStorageLocation(row.metadata?.name)
    selectedRow.value = res.data
  } catch (e) {
    selectedRow.value = row
  }
  // Best-effort: fetch backups so the Sync block can show "N restore points
  // from this profile". Failure is fine — count just becomes 0.
  try {
    const bres = await getBackups()
    backupsForSync.value = bres.data?.items || []
  } catch (_) {
    backupsForSync.value = []
  }
  detailsVisible.value = true
}

const openEdit = (row) => {
  // Pre-fill from current spec. Reverse-map Velero provider back to the UI
  // option: if there's an s3Url and provider is "aws", it's likely a MinIO-
  // class deployment — show as MinIO so the user sees the right context. We
  // can't reliably distinguish MinIO vs SupVault from the CR, so default to
  // MinIO; the user can change it to SupVault explicitly if needed.
  const spec = row.spec || {}
  const cfg = spec.config || {}
  let uiProvider = spec.provider || 'aws'
  if (uiProvider === 'aws' && cfg.s3Url) uiProvider = 'minio'

  editForm.value = {
    name: row.metadata?.name || '',
    provider: uiProvider,
    bucket: spec.objectStorage?.bucket || '',
    region: cfg.region || '',
    s3Url: cfg.s3Url || '',
    s3ForcePathStyle: cfg.s3ForcePathStyle === 'true',
    accessKey: '',
    secretKey: ''
  }
  selectedRow.value = row
  detailsVisible.value = false
  editVisible.value = true
}

const onEditProviderChange = (val) => {
  if (S3_COMPATIBLE_UI.has(val)) {
    editForm.value.s3ForcePathStyle = true
  }
}

const handleUpdate = async () => {
  if (!editForm.value.bucket) {
    ElMessage.warning('Bucket is required')
    return
  }
  // Both keys must be provided together, or neither (preserve existing).
  const hasAccess = !!editForm.value.accessKey
  const hasSecret = !!editForm.value.secretKey
  if (hasAccess !== hasSecret) {
    ElMessage.warning('Provide both Access Key and Secret Key, or leave both empty to keep existing credentials')
    return
  }
  updating.value = true
  try {
    const payload = {
      provider: toVeleroProvider(editForm.value.provider),
      bucket: editForm.value.bucket,
      region: editForm.value.region || undefined,
      endpoint: editForm.value.s3Url || undefined,
      s3ForcePathStyle: editForm.value.s3ForcePathStyle,
      accessKey: editForm.value.accessKey || undefined,
      secretKey: editForm.value.secretKey || undefined
    }
    await updateStorageLocation(editForm.value.name, payload)
    ElMessage.success(`Storage location "${editForm.value.name}" updated`)
    editVisible.value = false
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to update storage location: ' + (e.response?.data?.error || e.message))
  } finally {
    updating.value = false
  }
}

const handleDelete = async (row) => {
  const name = row.metadata?.name
  if (!name) return
  const hasManagedSecret = row.spec?.credential?.name?.startsWith('supkube-bsl-')
  const cascadeNote = hasManagedSecret
    ? `\n\nThe linked credentials secret "${row.spec.credential.name}" will also be deleted.`
    : ''
  try {
    await ElMessageBox.confirm(
      `Delete storage location "${name}"? Backups already stored in the bucket are NOT removed.${cascadeNote}`,
      'Confirm Delete',
      { type: 'warning', confirmButtonText: 'Delete', cancelButtonText: 'Cancel' }
    )
  } catch {
    return
  }
  try {
    await deleteStorageLocation(name)
    ElMessage.success(`Storage location "${name}" deleted`)
    detailsVisible.value = false
    await fetchLocations()
  } catch (e) {
    ElMessage.error('Failed to delete storage location: ' + (e.response?.data?.error || e.message))
  }
}

onMounted(() => {
  fetchLocations()
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
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

/* Kebab action button */
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }

/* Drawer header — Kasten style */
:deep(.storage-details-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 20px 24px;
  border-bottom: 1px solid #ebeef5;
}
:deep(.storage-details-drawer .el-drawer__title) {
  font-size: 20px;
  font-weight: 700;
  color: #1f2329;
  text-align: center;
  width: 100%;
  letter-spacing: -0.01em;
}
:deep(.storage-details-drawer .el-drawer__body) { padding: 24px; }

.details-body { padding: 0 4px; }
.details-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 22px;
  font-weight: 700;
  color: #1f2329;
  margin-bottom: 24px;
  letter-spacing: -0.01em;
}
.storage-icon { font-size: 24px; }
.detail-block { margin-bottom: 24px; }
.detail-block-title {
  font-size: 11px;
  font-weight: 600;
  color: #909399;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 10px;
}
.sync-relative {
  color: #909399;
  font-size: 12px;
  margin-left: 4px;
}
.sync-hint {
  margin: 10px 0 0 0;
  padding: 10px 12px;
  font-size: 12px;
  background: #ecf5ff;
  border-radius: 4px;
  color: #2e5e8a;
  line-height: 1.6;
}
.sync-link {
  color: #409eff;
  text-decoration: underline;
  cursor: pointer;
}
.sync-link:hover { color: #66b1ff; }
.muted { color: #909399; }

.detail-actions {
  display: flex;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
  margin-top: 8px;
}
</style>
