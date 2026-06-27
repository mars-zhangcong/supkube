<template>
  <div class="drscores-page">
    <div class="page-header">
      <h3>{{ t('drScores.title') }}</h3>
      <p class="page-desc">{{ t('drScores.desc') }}</p>
    </div>

    <!-- ════ Cluster health summary strip ════ -->
    <el-row :gutter="16" class="summary-row">
      <el-col :span="5">
        <el-card shadow="never" class="stat-card">
          <div class="stat-value" :class="`stat-${avgScoreKey}`">{{ summary.avgScore ?? '—' }}</div>
          <div class="stat-label">{{ t('drScores.avgScore') }}</div>
          <div class="stat-sub">{{ t('drScores.ofScored', { n: summary.scoredApps || 0, total: summary.totalApps || 0 }) }}</div>
        </el-card>
      </el-col>
      <el-col :span="5">
        <el-card shadow="never" class="stat-card">
          <div class="stat-value" :class="(summary.atRiskApps || 0) > 0 ? 'stat-error' : 'stat-success'">{{ summary.atRiskApps ?? 0 }}</div>
          <div class="stat-label">{{ t('drScores.atRisk') }}</div>
          <div class="stat-sub">{{ t('drScores.atRiskSub') }}</div>
        </el-card>
      </el-col>
      <el-col :span="5">
        <el-card shadow="never" class="stat-card">
          <div class="stat-value" :class="(summary.unbackedWithData || 0) > 0 ? 'stat-error' : 'stat-success'">{{ summary.unbackedWithData ?? 0 }}</div>
          <div class="stat-label">{{ t('drScores.unbacked') }}</div>
          <div class="stat-sub">{{ t('drScores.unbackedSub') }}</div>
        </el-card>
      </el-col>
      <el-col :span="5">
        <el-card shadow="never" class="stat-card">
          <div class="stat-value">{{ summary.drillsPassed ?? 0 }}<span class="stat-of">/ {{ summary.drillsTotal ?? 0 }}</span></div>
          <div class="stat-label">{{ t('drScores.drills') }}</div>
          <div class="stat-sub">{{ t('drScores.drillsSub') }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card stat-card-action">
          <el-button text @click="fetchScores" :loading="loading">
            <el-icon><Refresh /></el-icon>&nbsp;{{ t('common.refresh') || 'Refresh' }}
          </el-button>
          <div class="stat-sub stat-cluster">{{ scores?.cluster || '—' }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- ════ Per-application score table (worst-first) ════ -->
    <el-card>
      <el-table
        :data="apps"
        style="width: 100%"
        v-loading="loading"
        row-key="namespace"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-body">
              <!-- recommendations -->
              <div class="expand-block">
                <div class="expand-h">{{ t('drScores.recommendations') }}</div>
                <div v-if="(row.recommendations || []).length === 0" class="muted">{{ t('drScores.noRecs') }}</div>
                <div v-for="(r, i) in row.recommendations" :key="i" class="rec-row">
                  <span class="sk-chip" :class="`sk-chip-status-${sevChipKey(r.severity)}`">{{ r.severity }}</span>
                  <span class="rec-type">{{ r.type }}</span>
                  <span class="rec-msg">{{ r.message }}</span>
                </div>
              </div>
              <!-- dimension breakdown (only when scored) -->
              <div v-if="row.status === 'scored' && row.dimensions" class="expand-block">
                <div class="expand-h">{{ t('drScores.dimensions') }}</div>
                <div v-for="d in dimensionList(row)" :key="d.key" class="dim-row">
                  <span class="dim-label">{{ d.label }}</span>
                  <div class="dim-bar"><div class="dim-fill" :class="`dim-${d.key2}`" :style="{ width: d.pct + '%' }"></div></div>
                  <span class="dim-num">{{ d.score }}/{{ d.max }}</span>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="namespace" :label="t('drScores.application')" sortable min-width="220">
          <template #default="{ row }">
            <div class="app-cell">
              <div class="app-name">{{ row.namespace }}</div>
              <div class="app-cell-chips">
                <span v-if="!row.protected" class="sk-chip sk-chip-status-error">{{ t('drScores.unprotected') }}</span>
                <span v-else class="sk-chip sk-chip-status-success">{{ t('drScores.protected') }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('drScores.healthScore')" width="180" sortable
          :sort-method="(a, b) => scoreRank(a) - scoreRank(b)">
          <template #default="{ row }">
            <div class="score-cell">
              <span v-if="row.status === 'scored'" class="score-num" :class="`score-${levelChipKey(row)}`">{{ row.totalScore }}</span>
              <span v-else class="score-num score-muted">—</span>
              <span class="sk-chip" :class="`sk-chip-status-${levelChipKey(row)}`">{{ levelLabel(row) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('drScores.data')" width="120">
          <template #default="{ row }">
            <span class="data-cell">
              <span v-if="row.statefulSets > 0" class="data-tag">STS {{ row.statefulSets }}</span>
              <span v-if="row.pvcs > 0" class="data-tag">PVC {{ row.pvcs }}</span>
              <span v-if="!row.statefulSets && !row.pvcs" class="muted">—</span>
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="t('drScores.advice')" width="140">
          <template #default="{ row }">
            <span v-if="topSeverity(row)" class="sk-chip" :class="`sk-chip-status-${sevChipKey(topSeverity(row))}`">
              {{ (row.recommendations || []).length }} · {{ topSeverity(row) }}
            </span>
            <span v-else class="muted">{{ t('drScores.ok') }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.actions') || ''" width="120" align="right">
          <template #default="{ row }">
            <el-button v-if="!row.protected" size="small" type="primary" plain @click="goCreatePolicy(row)">
              {{ t('drScores.backupNow') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <p class="footer-note">
      {{ t('drScores.footnote', { v: scores?.rulesVersion || 'v1.0.0', at: formatTime(scores?.snapshotAt) }) }}
    </p>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { getDRScores } from '../api/velero'
import { ElMessage } from 'element-plus'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const scores = ref(null)
const summary = computed(() => scores.value?.summary || {})
const apps = computed(() => scores.value?.apps || [])

// avg score → coarse color key for the headline card
const avgScoreKey = computed(() => {
  const s = summary.value?.avgScore
  if (s == null) return 'muted'
  if (s >= 75) return 'success'
  if (s >= 60) return 'warning'
  return 'error'
})

// level → global chip taxonomy (tokens.css .sk-chip-status-*)
const levelChipKey = (app) => {
  if (app.status !== 'scored') return 'warning' // not_eligible = needs attention
  return {
    high_resilience: 'success',
    compliant_low_risk: 'success',
    fragile: 'warning',
    critical: 'error'
  }[app.level] || 'muted'
}
const levelLabel = (app) => {
  if (app.status === 'not_eligible_no_policy') return t('drScores.status.noPolicy')
  if (app.status === 'not_eligible_no_runs') return t('drScores.status.neverRan')
  return t(`drScores.level.${app.level}`)
}

// Sort rank: unbacked-with-data first, then not_eligible, then ascending score.
const scoreRank = (app) => {
  if (!app.protected && (app.statefulSets > 0 || app.pvcs > 0)) return -1000 + (app.totalScore || 0)
  if (app.status !== 'scored') return -500
  return app.totalScore || 0
}

const sevChipKey = (sev) => ({ P1: 'error', P2: 'warning', P3: 'muted' }[sev] || 'muted')
const topSeverity = (app) => {
  const recs = app.recommendations || []
  if (recs.some(r => r.severity === 'P1')) return 'P1'
  if (recs.some(r => r.severity === 'P2')) return 'P2'
  if (recs.length) return 'P3'
  return null
}

// dimension bars for the expand row
const dimensionList = (app) => {
  const d = app.dimensions || {}
  const mk = (key, key2, label, dim) => ({
    key, key2, label,
    score: dim?.score ?? 0,
    max: dim?.maxScore ?? 0,
    pct: dim?.maxScore ? Math.round((dim.score / dim.maxScore) * 100) : 0
  })
  return [
    mk('coverage', 'coverage', t('drScores.dim.coverage'), d.backupCoverage),
    mk('resilience', 'resilience', t('drScores.dim.resilience'), d.resilience),
    mk('security', 'security', t('drScores.dim.security'), d.immutabilityAndSecurity),
    mk('reliability', 'reliability', t('drScores.dim.reliability'), d.reliability)
  ]
}

const formatTime = (ts) => (ts ? new Date(ts).toLocaleString() : '—')

// "Backup now" → reuse the Policies create-wizard deep link (same contract
// Applications.vue uses for its Backup/Create-Policy action).
const goCreatePolicy = (row) => {
  router.push({ path: '/policies', query: { namespace: row.namespace, intent: 'create' } })
}

const fetchScores = async () => {
  loading.value = true
  try {
    const res = await getDRScores()
    scores.value = res.data
  } catch (e) {
    ElMessage.error(`Failed to load DR scores: ${e?.response?.data?.error || e.message}`)
    console.error(e)
  } finally {
    loading.value = false
  }
}

let refreshTimer = null
onMounted(() => {
  fetchScores()
  refreshTimer = setInterval(fetchScores, 30000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.page-header { margin-bottom: 20px; }
.page-header h3 { margin: 0 0 4px 0; font-size: 20px; font-weight: 600; }
.page-desc { margin: 0; color: #909399; font-size: 13px; }

.summary-row { margin-bottom: 20px; }
.stat-card { text-align: center; }
.stat-card :deep(.el-card__body) { padding: 16px 12px; }
.stat-value { font-size: 30px; font-weight: 700; line-height: 1.1; color: var(--sk-text, #303133); }
.stat-of { font-size: 16px; font-weight: 500; color: #909399; margin-left: 2px; }
.stat-success { color: #52c41a; }
.stat-warning { color: #faad14; }
.stat-error { color: #ff4d4f; }
.stat-muted { color: #909399; }
.stat-label { margin-top: 6px; font-size: 13px; font-weight: 600; color: #606266; }
.stat-sub { margin-top: 2px; font-size: 11px; color: #a0a4ab; }
.stat-card-action { display: flex; flex-direction: column; align-items: center; justify-content: center; }
.stat-cluster { margin-top: 6px; }

.app-cell { display: flex; flex-direction: column; gap: 4px; padding: 4px 0; }
.app-name { font-weight: 600; font-size: 14px; color: var(--sk-text, #303133); }
.app-cell-chips { display: flex; flex-wrap: wrap; gap: 6px; }

.score-cell { display: flex; align-items: center; gap: 10px; }
.score-num { font-size: 18px; font-weight: 700; min-width: 28px; }
.score-success { color: #52c41a; }
.score-warning { color: #faad14; }
.score-error { color: #ff4d4f; }
.score-muted { color: #c0c4cc; }

.data-cell { display: inline-flex; gap: 6px; }
.data-tag { font-size: 11px; padding: 1px 8px; border-radius: 10px; background: #f5f7fa; color: #606266; border: 1px solid #e4e7ed; }
.muted { color: #c0c4cc; font-size: 13px; }

/* expand row */
.expand-body { display: flex; gap: 32px; padding: 12px 24px 16px 48px; flex-wrap: wrap; }
.expand-block { flex: 1; min-width: 320px; }
.expand-h { font-size: 13px; font-weight: 600; color: #303133; margin-bottom: 10px; }
.rec-row { display: flex; align-items: baseline; gap: 10px; padding: 5px 0; }
.rec-type { font-size: 12px; font-weight: 600; color: #606266; font-family: 'SF Mono', Menlo, monospace; min-width: 130px; }
.rec-msg { font-size: 13px; color: #606266; }
.dim-row { display: flex; align-items: center; gap: 10px; padding: 4px 0; }
.dim-label { width: 90px; font-size: 12px; color: #606266; }
.dim-bar { flex: 1; height: 8px; background: #f0f2f5; border-radius: 4px; overflow: hidden; max-width: 220px; }
.dim-fill { height: 100%; border-radius: 4px; }
.dim-coverage { background: #1890ff; }
.dim-resilience { background: #52c41a; }
.dim-security { background: #722ed1; }
.dim-reliability { background: #faad14; }
.dim-num { font-size: 12px; color: #909399; min-width: 44px; text-align: right; }

.footer-note { margin-top: 14px; font-size: 12px; color: #a0a4ab; }
</style>
