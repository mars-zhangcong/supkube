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
        <el-table-column label="Actions" width="150">
          <template #default="{ row }">
            <el-button size="small" @click="handleVerify(row)" :loading="verifying">
              Verify
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Storage Location Dialog -->
    <el-dialog v-model="showCreateDialog" title="Add Storage Location" width="550px">
      <el-form :model="createForm" label-width="140px">
        <el-form-item label="Name" required>
          <el-input v-model="createForm.name" placeholder="my-s3-storage" />
        </el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="createForm.provider" style="width: 100%">
            <el-option label="AWS S3" value="aws" />
            <el-option label="MinIO (S3 Compatible)" value="aws" />
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
          <span style="margin-left: 8px; color: #999; font-size: 12px">Enable for MinIO</span>
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
import { ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { getStorageLocations, createStorageLocation, verifyStorageLocation } from '../api/velero'
import { ElMessage } from 'element-plus'

const locations = ref([])
const loading = ref(false)
const creating = ref(false)
const verifying = ref(false)
const showCreateDialog = ref(false)

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
  creating.value = true
  try {
    const payload = {
      name: createForm.value.name,
      provider: createForm.value.provider,
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
</style>
