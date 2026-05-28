<template>
  <div class="storage-page">
    <div class="page-header">
      <h3>{{ t('storage.title') }}</h3>
      <el-button type="primary"
        :disabled="!auth.canDo('storage.crud')"
        :title="!auth.canDo('storage.crud') ? t('common.noPermission') : ''"
        @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        {{ t('storage.create') }}
      </el-button>
    </div>

    <!-- v0.8.12 LBS1: SupKube-managed local MinIO BSL.
         Shows as a banner-style card ABOVE the regular BSL table to make
         it visually distinct — it's not a "user-added" location, it's
         provisioned by Helm and managed by SupKube itself. -->
    <el-card v-if="localStore.enabled" class="local-store-card" shadow="never">
      <div class="ls-row">
        <div class="ls-left">
          <div class="ls-title">
            <span class="sk-h2">Local Backup Store</span>
            <el-tag size="small" :type="localStorePhaseTagType">{{ localStore.phase || 'Unknown' }}</el-tag>
            <!-- v0.8.12 LBS3: Object Lock chip with mode + retention tooltip -->
            <el-tooltip
              v-if="localStore.objectLockEnabled"
              placement="top"
            >
              <template #content>
                <div style="max-width: 320px; line-height: 1.6">
                  <div><strong>Mode: {{ localStore.objectLockMode === 'compliance' ? 'Compliance (strictest)' : 'Governance (default)' }}</strong></div>
                  <div v-if="localStore.objectLockMode === 'compliance'">
                    Even cluster-admin / root cannot delete RPs before retention expires. Use for regulatory requirements.
                  </div>
                  <div v-else>
                    Admin with s3:BypassGovernanceRetention can override the lock. Switch to Compliance for regulatory data.
                  </div>
                  <div style="margin-top: 6px">Default bucket retention: <strong>30 days</strong>. Per-RP retention follows Policy TTL.</div>
                  <div style="margin-top: 6px; color: #9ca3af">To change mode: <code>helm upgrade --set localStore.objectLock.mode=compliance</code></div>
                </div>
              </template>
              <el-tag size="small" type="success" effect="plain" style="cursor: help">
                🛡 Object Lock · {{ localStore.objectLockMode || 'governance' }}
              </el-tag>
            </el-tooltip>
            <el-tag v-else size="small" type="warning" effect="plain">
              ⚠ Object Lock disabled
            </el-tag>
          </div>
          <div class="ls-meta">
            <span class="ls-meta-item">Bucket: <code>{{ localStore.bucket || '-' }}</code></span>
            <span class="ls-meta-item">Endpoint: <code>{{ localStore.endpoint || '-' }}</code></span>
            <span v-if="localStore.capacityBytes" class="ls-meta-item">Capacity: {{ formatBytes(localStore.capacityBytes) }}</span>
          </div>
          <div v-if="localStore.message" class="ls-message">{{ localStore.message }}</div>
        </div>
        <div class="ls-right">
          <span class="ls-caption">In-cluster MinIO · Managed by SupKube</span>
        </div>
      </div>
    </el-card>

    <!-- "Not enabled" empty card with hint for v0.8.12. Real "Enable" button
         comes in v0.8.12-LBS3 (triggers helm upgrade through backend). -->
    <el-card v-else-if="localStoreLoaded" class="local-store-card local-store-disabled" shadow="never">
      <div class="ls-row">
        <div class="ls-left">
          <div class="ls-title">
            <span class="sk-h2">Local Backup Store</span>
            <el-tag size="small" type="info" effect="plain">Not Enabled</el-tag>
          </div>
          <div class="ls-meta">
            <span class="ls-hint">
              Enable in-cluster MinIO as a fast local BSL. Backups will write to
              both local (fast recovery) and cloud (DR) — satisfies 3-2-1-1-0.
              Run <code>helm upgrade --set localStore.enabled=true</code> or use
              the Settings → Local Backup Store toggle (coming in LBS3).
            </span>
          </div>
        </div>
      </div>
    </el-card>

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
        <!-- v0.8.12 LBS3: Object Lock column.
             Reads supkube.io/object-lock + mode annotation that our
             local-store.yaml template stamps; for user-added Cloud BSLs
             the annotation is absent and we render a subtle "—" chip
             with a tooltip recommending they enable bucket-level
             immutability on the cloud side. -->
        <el-table-column label="Immutability" width="160">
          <template #default="{ row }">
            <el-tooltip
              v-if="objectLockOf(row).enabled"
              :content="`Object Lock enabled · Mode: ${objectLockOf(row).mode}. Retention enforced at the bucket level — even cluster-admin cannot delete RPs before retention expires.`"
              placement="top"
            >
              <el-tag type="success" size="small" effect="plain">
                🛡 Locked · {{ objectLockOf(row).mode }}
              </el-tag>
            </el-tooltip>
            <el-tooltip
              v-else
              content="No Object Lock. Cloud BSLs need bucket-level immutability (S3 Object Lock / Azure Immutable Blob) for 3-2-1-1-0 compliance."
              placement="top"
            >
              <el-tag type="info" size="small" effect="plain">— No Lock</el-tag>
            </el-tooltip>
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
              <span style="color: var(--sk-status-error); word-break: break-word">{{ selectedRow.status.message }}</span>
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
              <span v-else style="color: var(--sk-text-caption)">none (uses Velero default credentials)</span>
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

    <!-- Edit Storage Location Dialog (v0.8.7.2: identity fields editable).
         Only Name + Provider are truly immutable. Bucket/Container,
         StorageAccount, credentials are all editable — mirrors Kasten K10.
         The dialog shows a Kasten-style banner reminding the user to
         re-enter credentials if they touch identity-bound fields. -->
    <el-dialog v-model="editVisible" title="Edit Storage Location" width="550px">
      <el-form :model="editForm" label-width="160px">
        <div v-if="editIdentityDirty" class="edit-creds-banner">
          🔒 You're changing identity-bound fields. Re-enter credentials below — saving without a new key would leave the BSL pointed at a new target it can't authenticate to.
        </div>

        <el-form-item label="Name">
          <el-input v-model="editForm.name" disabled />
          <span class="form-hint">Name is immutable. To rename, delete and recreate.</span>
        </el-form-item>
        <el-form-item label="Provider">
          <el-select v-model="editForm.provider" style="width: 100%" disabled>
            <el-option label="AWS S3" value="aws" />
            <el-option label="MinIO" value="minio" />
            <el-option label="S3 Compatible (Tencent COS / Alibaba OSS / ...)" value="s3compatible" />
            <el-option label="SupVault" value="supvault" />
            <el-option label="GCP" value="gcp" />
            <el-option label="Azure Blob Storage" value="azure" />
          </el-select>
          <span class="form-hint">Provider can't change (different SDK). Delete and recreate to switch.</span>
        </el-form-item>

        <!-- ─── S3-family edit fields ────────────────────────────────── -->
        <template v-if="editForm.provider !== 'azure'">
          <el-form-item label="Bucket">
            <el-input v-model="editForm.bucket" />
            <span class="form-hint">Editable. Credentials are IAM-scoped, not bucket-scoped.</span>
          </el-form-item>
          <el-form-item label="Prefix">
            <el-input v-model="editForm.prefix" placeholder="(none — Velero owns top level)" />
            <span class="form-hint">Sub-path inside the bucket. Changing this makes existing backups invisible.</span>
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
          <!-- v0.9.1.5: same dynamic-label pattern as Azure section.
               When editIdentityDirty (bucket/region/endpoint changed),
               credentials are required and label reflects it. -->
          <el-form-item :label="editIdentityDirty ? 'Access Key (REQUIRED — bucket changed)' : 'Update Access Key (optional)'">
            <el-input v-model="editForm.accessKey" :placeholder="editIdentityDirty ? 'Required — new credentials' : 'Leave empty to keep existing'" />
          </el-form-item>
          <el-form-item :label="editIdentityDirty ? 'Secret Key (REQUIRED — bucket changed)' : 'Update Secret Key (optional)'">
            <el-input v-model="editForm.secretKey" type="password" :placeholder="editIdentityDirty ? 'Required — new credentials' : 'Leave empty to keep existing'" show-password />
          </el-form-item>
        </template>

        <!-- ─── Azure edit fields ────────────────────────────────────── -->
        <template v-if="editForm.provider === 'azure'">
          <el-form-item label="Container">
            <el-input v-model="editForm.bucket" />
            <span class="form-hint">Editable. The Storage Account Key works for any container under the same account.</span>
          </el-form-item>
          <el-form-item label="Prefix">
            <el-input v-model="editForm.prefix" placeholder="(empty = container root)" />
            <span class="form-hint">Sub-path inside the container. Changing this makes existing backups invisible.</span>
          </el-form-item>
          <el-form-item label="Storage Account">
            <el-input v-model="editForm.storageAccount" />
            <span class="form-hint">Editable. Changing this also requires a New Storage Account Key (keys are account-scoped).</span>
          </el-form-item>
          <!-- v0.9.1.5: label was "New Storage Account Key" which Mars demo
               showed is confusing — customers think it's only for rotating
               and look elsewhere for the initial key input. Switch to
               dynamic label that makes the required-vs-optional intent
               obvious based on whether identity-bound fields changed. -->
          <el-form-item :label="editIdentityDirty ? 'Storage Account Key (REQUIRED — new account)' : 'Update Storage Account Key (optional)'" class="storage-key-item">
            <el-input
              v-model="editForm.storageAccountKey"
              type="password"
              :placeholder="editIdentityDirty ? 'Required — paste the new account\'s key' : 'Leave empty to keep existing key'"
              show-password
              :class="{ 'is-required-changed': editIdentityDirty }"
            />
            <span class="form-hint">
              <template v-if="editIdentityDirty">
                ⚠ You changed Storage Account or Container. The old key won't work for the new account — paste the new key here.
              </template>
              <template v-else>
                Optional. Leave empty to keep the existing key; fill in to rotate credentials.
              </template>
            </span>
          </el-form-item>
          <el-form-item label="Resource Group">
            <el-input v-model="editForm.resourceGroup" placeholder="(optional unless using AAD)" />
          </el-form-item>
          <el-form-item label="Subscription ID">
            <el-input v-model="editForm.subscriptionId" placeholder="(optional)" />
          </el-form-item>
        </template>

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
            <!-- v0.8.12 LBS3.1: MinIO is its own option (typically refers
                 to the in-cluster MinIO deployed by SupKube; user-added
                 external MinIO can use either MinIO or S3 Compatible —
                 both auto-enable path-style addressing). -->
            <el-option label="MinIO" value="minio" />
            <!-- v0.8.12 LBS3.1: Generic "S3 Compatible" for non-MinIO
                 S3-compatible vendors — Tencent COS, Alibaba OSS, Ceph
                 RGW, SeaweedFS, Wasabi, Backblaze B2 S3 API, etc. -->
            <el-option label="S3 Compatible (Tencent COS / Alibaba OSS / Ceph RGW / ...)" value="s3compatible" />
            <el-option label="SupVault" value="supvault" />
            <el-option label="GCP (planned v0.9)" value="gcp" disabled />
            <el-option label="Azure Blob Storage" value="azure" />
          </el-select>
          <span v-if="createForm.provider === 'azure'" class="provider-hint">
            v0.8.7 supports storage-account-key auth. AAD service-principal lands in v0.9.
          </span>
          <span v-if="createForm.provider === 's3compatible'" class="provider-hint">
            Path-style addressing auto-enabled. Tencent COS endpoint: <code>https://cos.&lt;region&gt;.myqcloud.com</code>.
            Alibaba OSS: <code>https://oss-&lt;region&gt;.aliyuncs.com</code>.
          </span>
        </el-form-item>

        <!-- ─── S3-family fields (aws / minio / supvault) ─────────────── -->
        <template v-if="createForm.provider !== 'azure'">
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
        </template>

        <!-- ─── Azure-specific fields ─────────────────────────────────── -->
        <template v-if="createForm.provider === 'azure'">
          <el-form-item label="Container" required>
            <el-input v-model="createForm.bucket" placeholder="my-backup-container" />
            <span class="form-hint">The Azure Blob container name (stored as BSL.spec.objectStorage.bucket).</span>
          </el-form-item>
          <el-form-item label="Prefix">
            <el-input v-model="createForm.prefix" placeholder="velero (recommended if container has other tools' data)" />
            <span class="form-hint">
              Optional subfolder. Use this when the container already holds data from another tool (Kasten K10 writes a <code>k10/</code> directory; Velero refuses to share top-level). Setting <code>velero</code> here scopes all SupKube data under <code>&lt;container&gt;/velero/</code>.
            </span>
          </el-form-item>
          <el-form-item label="Storage Account" required>
            <el-input v-model="createForm.storageAccount" placeholder="mysupkubesa" />
          </el-form-item>
          <el-form-item label="Storage Account Key" required>
            <el-input
              v-model="createForm.storageAccountKey"
              type="password"
              placeholder="from Azure portal → Storage account → Access keys"
              show-password
            />
            <span class="form-hint">
              Never paste this into a Git-tracked values.yaml. SupKube stores it in a K8s Secret labelled supkube.io/bsl-name.
            </span>
          </el-form-item>
          <el-form-item label="Resource Group">
            <el-input v-model="createForm.resourceGroup" placeholder="my-rg (optional)" />
            <span class="form-hint">
              Only required for AAD/MSI auth (planned v0.9). Storage-account-key auth (this form) talks directly to Blob and doesn't need it.
            </span>
          </el-form-item>
          <el-form-item label="Subscription ID">
            <el-input v-model="createForm.subscriptionId" placeholder="00000000-0000-0000-0000-000000000000 (optional)" />
            <span class="form-hint">Only required if you intend to use Azure-native VolumeSnapshots (not for Blob-only DR).</span>
          </el-form-item>
        </template>
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
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'

const { t } = useI18n()
import { useAuth } from '../composables/useAuth'
const auth = useAuth()
import {
  getStorageLocations,
  getStorageLocation,
  createStorageLocation,
  updateStorageLocation,
  deleteStorageLocation,
  verifyStorageLocation,
  getBackups,
  getLocalStoreStatus
} from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const locations = ref([])
const backupsForSync = ref([])
const loading = ref(false)

// v0.8.12 LBS1: SupKube-managed in-cluster MinIO BSL status. Lives in its
// own card above the BSL table — distinct from user-added cloud BSLs.
// localStoreLoaded gates the "Not Enabled" empty state so we don't flash
// it during the initial GET.
const localStore = ref({
  enabled: false,
  phase: '',
  bucket: '',
  endpoint: '',
  objectLockEnabled: false,
  objectLockMode: '',
  capacityBytes: 0,
  message: ''
})
const localStoreLoaded = ref(false)
const localStorePhaseTagType = computed(() => {
  switch (localStore.value.phase) {
    case 'Ready': return 'success'
    case 'Pending': return 'warning'
    case 'Degraded': return 'warning'
    case 'Failed': return 'danger'
    default: return 'info'
  }
})

async function fetchLocalStore() {
  try {
    const res = await getLocalStoreStatus()
    localStore.value = { ...localStore.value, ...res.data }
  } catch (e) {
    // Non-fatal — older backend doesn't have this endpoint. Treat as
    // disabled rather than showing an error toast on every page load.
    console.warn('local-store status unavailable:', e?.response?.status)
  } finally {
    localStoreLoaded.value = true
  }
}

function formatBytes(n) {
  if (!n || n <= 0) return '-'
  const u = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 ? 0 : v >= 10 ? 1 : 2)} ${u[i]}`
}

// v0.8.12 LBS3: pull the Object Lock state out of the BSL's annotations.
// Local BSL (SupKube-managed): annotations stamped by the Helm chart's
// local-store.yaml template. Cloud BSL: annotations absent unless the
// admin set them by hand. Returns { enabled, mode } for the chip.
function objectLockOf(row) {
  const ann = row?.metadata?.annotations || {}
  return {
    enabled: ann['supkube.io/object-lock'] === 'true',
    mode: ann['supkube.io/object-lock-mode'] || 'governance'
  }
}
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
  prefix: '',
  // S3-family
  region: '',
  s3Url: '',
  s3ForcePathStyle: false,
  accessKey: '',
  secretKey: '',
  // Azure
  storageAccount: '',
  storageAccountKey: '',
  resourceGroup: '',
  subscriptionId: ''
})

// v0.8.7.2: snapshot of the BSL's identity-bound fields at Edit open
// time. Used to detect when the user touched a field that requires
// re-supplying credentials (Kasten-style banner + Save-time guards).
const editOriginal = ref({ bucket: '', storageAccount: '' })

// editIdentityDirty fires when the user has typed a different value in
// bucket OR storageAccount than what came out of the live BSL. Shows
// the orange banner asking them to re-enter credentials. We DON'T look
// at prefix here because changing prefix doesn't break auth — only
// changes which folder Velero reads/writes.
const editIdentityDirty = computed(() => {
  if (editForm.value.bucket !== editOriginal.value.bucket) return true
  if (editForm.value.provider === 'azure' &&
      editForm.value.storageAccount !== editOriginal.value.storageAccount) return true
  return false
})

const createForm = ref({
  name: '',
  provider: 'aws',
  // S3-family fields. `bucket` is reused as the Azure container name
  // when provider=azure so the backend contract stays one shape.
  bucket: '',
  region: '',
  s3Url: '',
  s3ForcePathStyle: false,
  accessKey: '',
  secretKey: '',
  // v0.8.7 Azure-specific. Sent only when provider=azure; ignored by
  // the backend AWS path so we can keep them in the same form state.
  storageAccount: '',
  storageAccountKey: '',
  resourceGroup: '',
  subscriptionId: '',
  // v0.8.7.1 BSL prefix — applies to any provider. Common need is
  // Azure container sharing with Kasten K10 (which writes k10/),
  // but works for S3 buckets that already host other data too.
  prefix: ''
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

// S3-compatible UI choices all map to Velero's `aws` provider plugin;
// the UI distinction exists only so we can (a) auto-enable path-style
// addressing for those that need it, (b) show vendor-specific hints
// in the form, (c) render a more accurate "Provider" column on the
// Storage Locations list.
//
// v0.8.12 LBS3.1: 's3compatible' is a NEW generic option for vendors
// not specifically listed (Tencent COS, Alibaba OSS, Ceph RGW, SeaweedFS,
// Wasabi, B2 S3, ...). MinIO stays as its own option because (a) it's
// the in-cluster default and (b) we want the auto-detect on read to
// label SupKube's managed MinIO precisely.
const S3_COMPATIBLE_UI = new Set(['minio', 'supvault', 's3compatible'])
const toVeleroProvider = (uiProvider) =>
  S3_COMPATIBLE_UI.has(uiProvider) ? 'aws' : uiProvider

const onProviderChange = (val) => {
  // Path-style addressing is required by MinIO / Ceph RGW / SeaweedFS;
  // most COS / OSS endpoints support both but path-style is the lowest-
  // common-denominator default. Enable on every S3-compatible variant.
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
    ElMessage.warning('Please fill in name and bucket / container')
    return
  }
  if (nameError.value) {
    ElMessage.warning(nameError.value)
    return
  }
  // Azure-specific guards — match the backend's required-field validation
  // so the user sees a friendly inline error rather than waiting for the
  // server round-trip. v0.8.7.1 dropped RG from "required" (only needed
  // for AAD mode, which isn't shipped yet) — match the backend change.
  if (createForm.value.provider === 'azure') {
    if (!createForm.value.storageAccount) {
      ElMessage.warning('Storage Account is required for Azure')
      return
    }
    if (!createForm.value.storageAccountKey) {
      ElMessage.warning('Storage Account Key is required (v0.8.7 supports key auth only)')
      return
    }
  }
  creating.value = true
  try {
    // Build the payload, branching by provider. The backend accepts
    // ONE shape with optional fields per provider; we only fill what
    // matters for the chosen provider so the request stays small.
    const isAzure = createForm.value.provider === 'azure'
    const payload = {
      name: createForm.value.name,
      // Note: toVeleroProvider maps minio/supvault → aws (they're S3-API),
      // but leaves azure as 'azure' since the backend dispatches on that.
      provider: toVeleroProvider(createForm.value.provider),
      bucket: createForm.value.bucket,
      // v0.8.7.1 prefix is provider-agnostic.
      prefix: createForm.value.prefix || undefined,
      ...(isAzure
        ? {
            storageAccount: createForm.value.storageAccount,
            storageAccountKey: createForm.value.storageAccountKey,
            // RG now optional — only send if user typed one.
            resourceGroup: createForm.value.resourceGroup || undefined,
            subscriptionId: createForm.value.subscriptionId || undefined
          }
        : {
            region: createForm.value.region || undefined,
            s3Url: createForm.value.s3Url || undefined,
            s3ForcePathStyle: createForm.value.s3ForcePathStyle,
            accessKey: createForm.value.accessKey || undefined,
            secretKey: createForm.value.secretKey || undefined
          })
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
      secretKey: '',
      // Reset Azure fields too — otherwise opening Create after editing
      // an Azure entry would leak the previous storage account into the
      // next form session.
      storageAccount: '',
      storageAccountKey: '',
      resourceGroup: '',
      subscriptionId: '',
      prefix: ''
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
  // option: aws + s3Url means an S3-compatible deployment. We look at:
  //   1. The supkube.io/bsl-role=local label → SupKube-managed MinIO
  //   2. The s3Url host fingerprint → infer vendor where possible
  //   3. Fallback → s3compatible (the generic bucket)
  //
  // v0.8.12 LBS3.1: introduce 's3compatible' as the new fallback (was
  // 'minio'); MinIO is now reserved for actual MinIO deployments so
  // vendor-specific UX (path-style toggle, region hint) stays accurate.
  const spec = row.spec || {}
  const cfg = spec.config || {}
  const labels = row.metadata?.labels || {}
  let uiProvider = spec.provider || 'aws'
  if (uiProvider === 'aws' && cfg.s3Url) {
    if (labels['supkube.io/bsl-role'] === 'local') {
      uiProvider = 'minio'
    } else {
      const url = String(cfg.s3Url).toLowerCase()
      if (url.includes('minio'))                     uiProvider = 'minio'
      else if (url.includes('myqcloud') || url.includes('cos.'))  uiProvider = 's3compatible'  // Tencent COS
      else if (url.includes('aliyuncs') || url.includes('oss-'))  uiProvider = 's3compatible'  // Alibaba OSS
      else                                            uiProvider = 's3compatible'
    }
  }

  const bucket = spec.objectStorage?.bucket || ''
  const storageAccount = cfg.storageAccount || ''

  editForm.value = {
    name: row.metadata?.name || '',
    provider: uiProvider,
    bucket,
    prefix: spec.objectStorage?.prefix || '',
    // S3-family — hydrate from spec.config; never from request state.
    region: cfg.region || '',
    s3Url: cfg.s3Url || '',
    s3ForcePathStyle: cfg.s3ForcePathStyle === 'true',
    accessKey: '',
    secretKey: '',
    // Azure — identity fields editable; credential fields blank.
    storageAccount,
    storageAccountKey: '',           // blank = keep existing key
    resourceGroup: cfg.resourceGroup || '',
    subscriptionId: cfg.subscriptionId || ''
  }
  // Snapshot identity values so we can detect changes for the banner +
  // Save-time validation.
  editOriginal.value = { bucket, storageAccount }
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
  // v0.8.7.2: Bucket + StorageAccount are now editable. We send them
  // explicitly so the backend can compare to the live BSL and decide
  // whether to nag for credentials. The backend rejects identity
  // changes without matching credentials with a clear 400 message.
  const isAzure = editForm.value.provider === 'azure'

  if (!editForm.value.bucket) {
    ElMessage.warning('Bucket / Container is required')
    return
  }

  if (isAzure) {
    // If Storage Account changed but no new key, surface the error
    // inline (mirrors what the backend would say) — saves a round-trip.
    const saChanged = editForm.value.storageAccount !== editOriginal.value.storageAccount
    if (saChanged && !editForm.value.storageAccountKey) {
      ElMessage.warning('Changing Storage Account also requires a New Storage Account Key (keys are account-scoped).')
      return
    }
  } else {
    const hasAccess = !!editForm.value.accessKey
    const hasSecret = !!editForm.value.secretKey
    if (hasAccess !== hasSecret) {
      ElMessage.warning('Provide both Access Key and Secret Key, or leave both empty to keep existing credentials')
      return
    }
  }

  updating.value = true
  try {
    const payload = {
      bucket: editForm.value.bucket,
      prefix: editForm.value.prefix || '',
      ...(isAzure
        ? {
            storageAccount: editForm.value.storageAccount || undefined,
            storageAccountKey: editForm.value.storageAccountKey || undefined,
            resourceGroup: editForm.value.resourceGroup || undefined,
            subscriptionId: editForm.value.subscriptionId || undefined
          }
        : {
            region: editForm.value.region || undefined,
            endpoint: editForm.value.s3Url || undefined,
            s3ForcePathStyle: editForm.value.s3ForcePathStyle,
            accessKey: editForm.value.accessKey || undefined,
            secretKey: editForm.value.secretKey || undefined
          })
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
  fetchLocalStore()
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
  color: var(--sk-text-caption);
  line-height: 1.4;
  margin-top: 4px;
}
/* v0.8.7.2: Kasten-style banner that fires when the user edits an
   identity-bound field (bucket/container or storage account). Reminds
   them they need to re-enter credentials — saves a round-trip to the
   backend's 400 reply and matches Kasten K10's UX. */
.edit-creds-banner {
  background: #fdf6e3;
  border-left: 4px solid #e6a23c;
  color: #7a5b00;
  padding: 12px 16px;
  margin-bottom: 16px;
  border-radius: 4px;
  font-size: 13px;
  line-height: 1.5;
}

/* Provider-switch hint: appears under the provider dropdown when the
   user picks Azure to flag the storage-account-key-auth limitation. */
.provider-hint {
  display: block;
  font-size: 12px;
  color: #c79c2a;
  background: #fdf6e3;
  border-left: 3px solid #e6a23c;
  padding: 6px 10px;
  margin-top: 6px;
  border-radius: 4px;
  line-height: 1.4;
}

/* Kebab action button */
.more-btn { padding: 4px 8px; font-size: 18px; color: var(--sk-text-muted); }
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
  color: var(--sk-text);
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
  color: var(--sk-text);
  margin-bottom: 24px;
  letter-spacing: -0.01em;
}
.storage-icon { font-size: 24px; }
.detail-block { margin-bottom: 24px; }
.detail-block-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--sk-text-caption);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 10px;
}
.sync-relative {
  color: var(--sk-text-caption);
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
.muted { color: var(--sk-text-caption); }

.detail-actions {
  display: flex;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
  margin-top: 8px;
}

/* v0.8.12 LBS1: Local Backup Store banner card. Sits above the regular
   BSL table; distinct background so users see at a glance that it's
   SupKube-managed (not a user-added cloud BSL). */
.local-store-card {
  margin-bottom: 16px;
  border: 1px solid var(--sk-border-light, #e5e7eb);
  background: linear-gradient(135deg, var(--sk-primary-bg-hover, #f5f3ff) 0%, #fff 100%);
}
.local-store-disabled {
  background: var(--sk-surface-muted, #fafbfc);
}
.ls-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.ls-left { flex: 1; min-width: 0; }
.ls-right { flex: 0 0 auto; text-align: right; }
.ls-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.ls-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 13px;
  color: var(--sk-text-muted, #6b7280);
}
.ls-meta-item code {
  background: var(--sk-surface-code, #f3f4f6);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
}
.ls-message {
  margin-top: 8px;
  font-size: 12px;
  color: var(--sk-status-warning, #b45309);
  font-family: 'SF Mono', Menlo, monospace;
}
.ls-hint {
  font-size: 13px;
  color: var(--sk-text-caption, #6b7280);
  line-height: 1.6;
}
.ls-hint code {
  background: var(--sk-surface-code, #f3f4f6);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
}
.ls-caption {
  font-size: 12px;
  color: var(--sk-text-caption, #9ca3af);
}
</style>
