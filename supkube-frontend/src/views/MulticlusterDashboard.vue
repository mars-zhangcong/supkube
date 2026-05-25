<!--
  MulticlusterDashboard (v0.9.0.2)
  ────────────────────────────────────────────────────────────────────
  The page Mode Switcher → "Multi-Cluster Manager" lands on.

  Layout (Kasten-inspired):
    ┌─ Header ──────────────────────────────────────────────────┐
    │ Multi-Cluster Manager                  [+ Add Cluster]   │
    │ Aggregated view of every cluster SupKube is managing.    │
    └───────────────────────────────────────────────────────────┘

    ┌─ Total ─┐  ┌─ Apps ─┐  ┌─ Policies ─┐  ┌─ Restore Pts ─┐
    │   3     │  │   12   │  │      8      │  │     101         │
    │ clusters│  │        │  │             │  │                 │
    └─────────┘  └────────┘  └─────────────┘  └─────────────────┘

    ┌─ Clusters ─────────────────────────────────────────────────┐
    │ Cluster              Type    Phase    K8s   Apps Pol RPs  │
    │ this-cluster ★       Primary ●Healthy v1.32  3   4   14   │ →
    │ aks-jumborca-dev     Secondary ●Healthy v1.34 0   0   0   │ →
    │ aks-jumborca-test    Secondary ●Healthy v1.34 0   0   0   │ →
    └────────────────────────────────────────────────────────────┘

  Click a cluster row → setActive(name) + router.push('/dashboard').
  This makes the MCM page a fast "where do I want to go next" launcher
  in addition to a status overview.

  Why not embed DRTopology here too: cross-cluster topology aggregation
  is a deeper backend rework (flows across clusters need a unified BSL
  registry view) — deferred to v0.9.x. For v0.9.0.2 the MCM page is
  "summary + launcher", DRTopology stays per-cluster on /dashboard.
-->

<template>
  <div class="mcm-page" v-loading="loading">
    <!-- ════ Header ════ -->
    <div class="mcm-header">
      <div>
        <h2 class="sk-h1" style="margin: 0">{{ t('mcm.title') }}</h2>
        <p class="sk-caption" style="margin: 4px 0 0 0">{{ t('mcm.subtitle') }}</p>
      </div>
      <el-button type="primary" @click="goAddCluster">
        <el-icon><Plus /></el-icon>
        {{ t('clusters.add') }}
      </el-button>
    </div>

    <!-- ════ Totals cards ════ -->
    <el-row :gutter="16" class="mcm-totals">
      <el-col :span="6">
        <el-card class="mcm-total-card">
          <div class="mtc-val">{{ totals.clusterCount }}</div>
          <div class="mtc-lbl">{{ t('mcm.totals.clusters') }}</div>
          <div class="mtc-sub" v-if="totals.unhealthyCount > 0">
            <span class="mtc-bad">{{ totals.unhealthyCount }}</span> {{ t('mcm.totals.unhealthy') }}
          </div>
          <div class="mtc-sub" v-else>
            <span class="mtc-ok">●</span> {{ t('mcm.totals.allHealthy') }}
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="mcm-total-card">
          <div class="mtc-val">{{ totals.appCount }}</div>
          <div class="mtc-lbl">{{ t('mcm.totals.apps') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="mcm-total-card">
          <div class="mtc-val">{{ totals.policyCount }}</div>
          <div class="mtc-lbl">{{ t('mcm.totals.policies') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="mcm-total-card">
          <div class="mtc-val">{{ totals.rpCount }}</div>
          <div class="mtc-lbl">{{ t('mcm.totals.restorePoints') }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- ════ Clusters table ════ -->
    <el-card class="mcm-clusters-card">
      <template #header>
        <div class="mcc-header">
          <span class="sk-h3" style="margin: 0">{{ t('mcm.clustersTitle') }}</span>
          <span class="sk-caption">{{ t('mcm.clickHint') }}</span>
        </div>
      </template>

      <el-table
        :data="clusters"
        style="width: 100%"
        @row-click="onRowClick"
        :row-class-name="rowClass"
      >
        <el-table-column :label="t('mcm.col.name')" min-width="220">
          <template #default="{ row }">
            <div class="mcc-name">
              <el-icon class="mcc-icon"><DocumentCopy /></el-icon>
              <span class="mcc-name-text">{{ row.displayName || row.name }}</span>
              <el-tag v-if="row.type === 'primary'" size="small" type="primary" effect="plain">
                ★ {{ t('clusters.primary') }}
              </el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.phase')" width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="phaseType(row.phase)">● {{ row.phase }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.k8s')" width="110">
          <template #default="{ row }">
            <span class="mcc-mono">{{ row.k8sVersion || '—' }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.nodes')" width="80" align="right">
          <template #default="{ row }">{{ row.nodeCount || '—' }}</template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.apps')" width="80" align="right">
          <template #default="{ row }">{{ row.appCount }}</template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.policies')" width="100" align="right">
          <template #default="{ row }">{{ row.policyCount }}</template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.rps')" width="100" align="right">
          <template #default="{ row }">{{ row.rpCount }}</template>
        </el-table-column>

        <el-table-column :label="t('mcm.col.lastBackup')" min-width="160">
          <template #default="{ row }">
            <span class="mcc-faint">{{ formatAgo(row.lastBackupAt) || '—' }}</span>
          </template>
        </el-table-column>

        <el-table-column width="50" align="right">
          <template #default>
            <el-icon class="mcc-chevron"><ArrowRight /></el-icon>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add Cluster wizard (re-used from ClustersPanel) -->
    <AddClusterWizard v-model="wizardOpen" @created="onCreated" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, DocumentCopy, ArrowRight } from '@element-plus/icons-vue'
import { getMultiClusterSummary } from '../api/velero'
import { useCluster } from '../composables/useCluster'
import AddClusterWizard from '../components/AddClusterWizard.vue'

const { t } = useI18n()
const router = useRouter()
const cluster = useCluster()

const loading = ref(true)
const clusters = ref([])
const totals = ref({ clusterCount: 0, appCount: 0, policyCount: 0, rpCount: 0, healthyCount: 0, unhealthyCount: 0 })
const wizardOpen = ref(false)

let pollTimer = null

async function fetchSummary() {
  loading.value = true
  try {
    const res = await getMultiClusterSummary()
    clusters.value = res.data.clusters || []
    totals.value = res.data.totals || {}
  } catch (e) {
    ElMessage.error(t('mcm.fetchFailed') + ': ' + (e?.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

function phaseType(p) {
  switch (p) {
    case 'Healthy':       return 'success'
    case 'Unreachable':
    case 'Unauthorized':  return 'danger'
    case 'Unknown':       return 'info'
    default:              return 'info'
  }
}

function formatAgo(iso) {
  if (!iso) return ''
  const sec = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60)   return Math.round(sec) + 's'
  if (sec < 3600) return Math.round(sec / 60) + 'm'
  if (sec < 86400) return Math.round(sec / 3600) + 'h'
  return Math.round(sec / 86400) + 'd'
}

function rowClass({ row }) {
  return row.isCurrent ? 'mcc-row-current' : ''
}

// v0.9.0.2: row click = switch cluster context + navigate to that
// cluster's Dashboard. Makes the MCM page a launcher in addition to
// a status overview.
function onRowClick(row) {
  cluster.setActive(row.name, router)
  router.push({ path: '/dashboard', query: { cluster: row.name } })
}

function goAddCluster() {
  router.push({ path: '/settings', query: { tab: 'clusters' } })
}

async function onCreated() {
  await fetchSummary()
}

onMounted(() => {
  fetchSummary()
  // Refresh every 30s — slower than DRTopology's 60s because this page
  // is the cluster-status home (admins watching here want fresher data).
  pollTimer = setInterval(fetchSummary, 30000)
})
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<style scoped>
.mcm-page { padding: 8px 0; }

.mcm-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.mcm-totals { margin-bottom: 20px; }
.mcm-total-card {
  text-align: left;
  padding: 4px 0;
}
.mtc-val {
  font-size: 32px;
  font-weight: 700;
  color: var(--sk-primary, #4f46e5);
  line-height: 1.2;
}
.mtc-lbl {
  font-size: 13px;
  color: var(--sk-text-caption, #6b7280);
  margin-top: 2px;
}
.mtc-sub {
  font-size: 12px;
  color: var(--sk-text-caption, #9ca3af);
  margin-top: 8px;
}
.mtc-ok  { color: #10b981; }
.mtc-bad { color: #dc2626; font-weight: 600; }

.mcm-clusters-card { padding-bottom: 0; }
.mcc-header {
  display: flex;
  align-items: baseline;
  gap: 14px;
}
.mcc-name {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mcc-icon {
  color: var(--sk-primary, #4f46e5);
  font-size: 16px;
}
.mcc-name-text {
  font-weight: 500;
  color: var(--sk-text, #1f2937);
}
.mcc-mono {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
  color: var(--sk-text-caption, #6b7280);
}
.mcc-faint { color: var(--sk-text-caption, #9ca3af); font-size: 12px; }
.mcc-chevron {
  color: var(--sk-text-placeholder, #d1d5db);
  font-size: 14px;
}
:deep(.el-table__row) { cursor: pointer; }
:deep(.el-table__row:hover) td { background: var(--sk-primary-bg-hover, #f5f3ff) !important; }
:deep(.el-table__row.mcc-row-current) td { background: #fafbfc; }
:deep(.el-table__row.mcc-row-current:hover) td { background: var(--sk-primary-bg-hover, #f5f3ff) !important; }
</style>
