<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.nodes }}</div>
            <div class="stat-label">Nodes</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.namespaces }}</div>
            <div class="stat-label">Namespaces</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value success">{{ stats.protectedApps }}</div>
            <div class="stat-label">Protected</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value">{{ stats.backups }}</div>
            <div class="stat-label">Total Backups</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value success">{{ stats.successful }}</div>
            <div class="stat-label">Successful</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card>
          <div class="stat">
            <div class="stat-value danger">{{ stats.failed }}</div>
            <div class="stat-label">Failed</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Imported Restore Points (v0.7.2) — surfaces cross-cluster backups
         that landed in this cluster via Velero BackupSync. Hidden when zero. -->
    <el-card v-if="stats.importedBackups > 0" class="imported-card" style="margin-top: 16px">
      <div class="imported-row">
        <span class="imported-icon">🌐</span>
        <div class="imported-text">
          <strong>{{ stats.importedBackups }}</strong>
          {{ stats.importedBackups === 1 ? 'restore point was' : 'restore points were' }}
          imported from <strong>{{ stats.importedSources }}</strong>
          other {{ stats.importedSources === 1 ? 'cluster' : 'clusters' }} via shared Storage Profiles.
        </div>
        <el-button size="small" @click="$router.push({ path: '/backups', query: { source: 'Imported' } })">
          View
        </el-button>
      </div>
    </el-card>

    <!-- Protection Compliance (v0.7-policy-2) -->
    <el-card v-if="stats.snapshotOnlyPolicies > 0" class="compliance-card" style="margin-top: 16px">
      <div class="compliance-row">
        <span class="compliance-icon">⚠</span>
        <div class="compliance-text">
          <strong>{{ stats.snapshotOnlyPolicies }}</strong>
          {{ stats.snapshotOnlyPolicies === 1 ? 'policy is' : 'policies are' }}
          configured as <strong>snapshot-only</strong> — these produce restore points that are
          <strong>not durable backups</strong>. Data is lost if the underlying storage fails.
        </div>
        <el-button size="small" @click="$router.push({ path: '/policies', query: { filter: 'snapshot-only' } })">
          Review
        </el-button>
      </div>
    </el-card>
    <el-card v-else-if="stats.totalPolicies > 0" class="compliance-card compliance-ok" style="margin-top: 16px">
      <div class="compliance-row">
        <span class="compliance-icon-ok">✓</span>
        <div class="compliance-text">
          All <strong>{{ stats.totalPolicies }}</strong> {{ stats.totalPolicies === 1 ? 'policy' : 'policies' }}
          produce durable backups (Snapshot + Export).
        </div>
      </div>
    </el-card>

    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>Recent Backups</span>
          <el-button type="primary" size="small" @click="$router.push('/backups')">
            View All
          </el-button>
        </div>
      </template>
      <el-table :data="recentBackups" style="width: 100%" v-loading="loading">
        <el-table-column label="Name">
          <template #default="{ row }">
            {{ row.name || row.metadata?.name || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Namespace">
          <template #default="{ row }">
            {{ row.namespace || row.metadata?.namespace || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="phaseTagType(row.phase ?? row.status?.phase)">
              {{ normalizePhase(row.phase ?? row.status?.phase) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.createdAt || row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
        <el-table-column label="Expires">
          <template #default="{ row }">
            {{ formatTime(row.expiration || row.status?.expiration) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>Recent Restores</span>
          <el-button type="primary" size="small" @click="$router.push('/restores')">
            View All
          </el-button>
        </div>
      </template>
      <el-table :data="recentRestores" style="width: 100%" v-loading="loading">
        <el-table-column prop="metadata.name" label="Name" />
        <el-table-column prop="spec.backupName" label="From Backup" />
        <el-table-column label="Status">
          <template #default="{ row }">
            <el-tag :type="phaseTagType(row.status?.phase)">
              {{ normalizePhase(row.status?.phase) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Created">
          <template #default="{ row }">
            {{ formatTime(row.metadata?.creationTimestamp) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { getDashboardSummary, getBackups, getRestores, getNamespaces, getApplications, getSchedules } from '../api/velero'
import { normalizePhase, phaseTagType } from '../utils/phase'

const stats = ref({
  nodes: 0, namespaces: 0, protectedApps: 0,
  backups: 0, successful: 0, failed: 0,
  totalPolicies: 0, snapshotOnlyPolicies: 0,
  // v0.7.2: cross-cluster import surfacing
  importedBackups: 0, importedSources: 0
})

// Mirror of Backups.vue logic — fingerprint from Velero source-cluster
// annotations, "this cluster" = most-common fingerprint among visible
// backups, anything else = Imported.
function backupClusterFingerprint(b) {
  const ann = b?.metadata?.annotations || {}
  return [
    ann['velero.io/source-cluster-k8s-gitversion'] || '',
    ann['velero.io/source-cluster-k8s-major-version'] || '',
    ann['velero.io/source-cluster-k8s-minor-version'] || ''
  ].join('|')
}
function countImported(backups) {
  const counts = new Map()
  for (const b of backups) {
    const fp = backupClusterFingerprint(b)
    if (fp && fp !== '||') counts.set(fp, (counts.get(fp) || 0) + 1)
  }
  let localFp = ''
  let bestCount = 0
  for (const [fp, c] of counts) {
    if (c > bestCount) { localFp = fp; bestCount = c }
  }
  let imported = 0
  const sources = new Set()
  for (const b of backups) {
    const fp = backupClusterFingerprint(b)
    if (!fp || fp === '||' || fp === localFp) continue
    imported++
    sources.add(fp)
  }
  return { imported, sources: sources.size }
}
const recentBackups = ref([])
const recentRestores = ref([])
const loading = ref(false)
let refreshTimer = null

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

const fetchData = async () => {
  loading.value = true
  try {
    // Try the aggregated summary endpoint first
    const summaryRes = await getDashboardSummary()
    const data = summaryRes.data
    stats.value.nodes = data.cluster?.nodes ?? 0
    stats.value.namespaces = data.cluster?.namespaces ?? 0
    stats.value.backups = data.backupSummary?.total ?? 0
    stats.value.successful = data.backupSummary?.completed ?? 0
    stats.value.failed = data.backupSummary?.failed ?? 0
    recentBackups.value = (data.recentBackups || []).slice(0, 5)

    // Still fetch restores, applications, schedules, and (v0.7.2) the full
    // backup list so we can compute imported-source counts. Summary endpoint
    // only returns the last 10 recent backups, which is not enough.
    const [restoresRes, appsRes, schedulesRes, backupsRes] = await Promise.all([
      getRestores(),
      getApplications().catch(() => null),
      getSchedules().catch(() => null),
      getBackups().catch(() => null)
    ])
    recentRestores.value = (restoresRes.data.items || []).slice(0, 5)
    if (backupsRes) {
      const { imported, sources } = countImported(backupsRes.data?.items || [])
      stats.value.importedBackups = imported
      stats.value.importedSources = sources
    }
    if (appsRes) {
      const apps = appsRes.data.items || []
      stats.value.protectedApps = apps.filter(a => a.protected).length
    }
    if (schedulesRes) {
      const schedules = schedulesRes.data.items || []
      stats.value.totalPolicies = schedules.length
      // Count snapshot-only policies (v0.7 annotation; older policies treated as L2)
      stats.value.snapshotOnlyPolicies = schedules.filter(
        s => s?.metadata?.annotations?.['supkube.io/export-enabled'] === 'false'
      ).length
    }
  } catch (e) {
    // Fallback to individual endpoints if summary not available
    try {
      const [backupsRes, restoresRes, nsRes] = await Promise.all([
        getBackups(),
        getRestores(),
        getNamespaces()
      ])

      const items = backupsRes.data.items || []
      recentBackups.value = items.slice(0, 5)
      stats.value.backups = items.length
      stats.value.successful = items.filter(b => b.status?.phase === 'Completed').length
      stats.value.failed = items.filter(b => b.status?.phase === 'Failed').length
      const imp = countImported(items)
      stats.value.importedBackups = imp.imported
      stats.value.importedSources = imp.sources

      const restoreItems = restoresRes.data.items || []
      recentRestores.value = restoreItems.slice(0, 5)

      const namespaces = nsRes.data.namespaces || nsRes.data.items || nsRes.data || []
      stats.value.namespaces = Array.isArray(namespaces) ? namespaces.length : 0
    } catch (fallbackErr) {
      console.error('Failed to load dashboard data:', fallbackErr)
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  refreshTimer = setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.dashboard { padding: 0; }
.stat { text-align: center; padding: 10px; }
.stat-value { font-size: 2em; font-weight: bold; }
.stat-label { color: #666; margin-top: 5px; }
.success { color: #67c23a; }
.danger { color: #f56c6c; }
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* Protection Compliance card */
.compliance-card {
  border-left: 4px solid #e6a23c;
  background: #fdf6ec;
}
.compliance-card.compliance-ok {
  border-left-color: #67c23a;
  background: #f0f9eb;
}
.imported-card {
  border-left: 4px solid #409eff;
  background: #ecf5ff;
}
.imported-row {
  display: flex;
  align-items: center;
  gap: 14px;
}
.imported-icon {
  font-size: 22px;
}
.imported-text {
  flex: 1;
  font-size: 13px;
  color: #2e5e8a;
  line-height: 1.6;
}
.compliance-row {
  display: flex;
  align-items: center;
  gap: 14px;
}
.compliance-icon {
  font-size: 22px;
  color: #e6a23c;
}
.compliance-icon-ok {
  font-size: 22px;
  color: #67c23a;
}
.compliance-text {
  flex: 1;
  font-size: 13px;
  color: #303133;
  line-height: 1.5;
}
.compliance-text strong { color: #1f2329; }
</style>
