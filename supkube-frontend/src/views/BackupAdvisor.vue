<template>
  <div class="advisor-page">
    <div class="page-header">
      <h3>{{ t('advisor.title') }}</h3>
      <p class="page-desc">{{ t('advisor.desc') }}</p>
    </div>

    <!-- Summary strip — counts by tier -->
    <el-row :gutter="12" class="tier-summary">
      <el-col :span="6" v-for="tier in tierSummary" :key="tier.code">
        <el-card class="tier-card" :class="`tier-${tier.code.toLowerCase()}`">
          <div class="tier-card-body">
            <div class="tier-count">{{ tier.count }}</div>
            <div class="tier-label">{{ tier.label }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Skip-Recommended warning surfaces snapshot-only skip risk -->
    <el-alert
      v-if="skipCount > 0"
      type="info"
      :closable="false"
      class="advisor-skip-notice"
    >
      <template #title>
        {{ t('advisor.skipNoticeTitle', { count: skipCount }) }}
      </template>
      {{ t('advisor.skipNoticeBody') }}
    </el-alert>

    <el-card style="margin-top: 16px">
      <el-table :data="recommendations" v-loading="loading" style="width: 100%">
        <el-table-column :label="t('common.namespace')" prop="namespace" min-width="160" sortable />
        <el-table-column :label="t('advisor.score')" prop="score" width="100" sortable :sort-method="(a, b) => a.score - b.score">
          <template #default="{ row }">
            <span class="score-pill" :class="`score-${row.tier.toLowerCase()}`">
              {{ row.score }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="t('advisor.tier')" prop="tier" width="160">
          <template #default="{ row }">
            <span class="tier-chip" :class="`tier-${row.tier.toLowerCase()}`">
              {{ tierIcon(row.tier) }} {{ t(`advisor.tiers.${row.tier}`) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="t('advisor.recommendedSchedule')" min-width="200">
          <template #default="{ row }">
            <div v-if="row.recommendedSchedule" class="schedule-cell">
              <div class="schedule-human">{{ formatSchedule(row.recommendedSchedule) }}</div>
              <code class="schedule-cron">{{ row.recommendedSchedule }}</code>
            </div>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('advisor.factors')" min-width="320">
          <template #default="{ row }">
            <ul class="factor-list" v-if="row.factors && row.factors.length">
              <li v-for="(f, idx) in row.factors.slice(0, 3)" :key="idx">
                <span :class="f.delta >= 0 ? 'factor-plus' : 'factor-minus'">
                  {{ f.delta >= 0 ? '+' : '' }}{{ f.delta }}
                </span>
                {{ translateFactor(f) }}
              </li>
              <li v-if="row.factors.length > 3" class="muted">
                {{ t('advisor.moreFactors', { count: row.factors.length - 3 }) }}
              </li>
            </ul>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="200" align="right">
          <template #default="{ row }">
            <el-button
              v-if="row.tier !== 'Skip'"
              size="small"
              type="primary"
              @click="apply(row)"
            >
              {{ t('advisor.apply') }}
            </el-button>
            <el-button v-else size="small" disabled>
              {{ t('advisor.skipAction') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getBackupAdvisor } from '../api/velero'
import { ElMessage } from 'element-plus'

const { t } = useI18n()
const router = useRouter()
const recommendations = ref([])
const loading = ref(false)

const tierSummary = computed(() => {
  const order = ['High', 'Medium', 'Low', 'Skip']
  const counts = Object.fromEntries(order.map(o => [o, 0]))
  for (const r of recommendations.value) {
    if (r.tier in counts) counts[r.tier]++
  }
  return order.map(code => ({
    code,
    label: t(`advisor.tiers.${code}`),
    count: counts[code]
  }))
})

const skipCount = computed(() =>
  recommendations.value.filter(r => r.tier === 'Skip').length
)

const tierIcon = (tier) => ({
  High: '🔴',
  Medium: '🟠',
  Low: '🔵',
  Skip: '⚪'
}[tier] || '')

// Translate a backend AdvisorFactor through i18n. The backend sends:
//   { reason: "Has 2 PVC(s) - ...",  reasonKey: "advisor.factors.hasPVC",
//     params: {count: 2}, delta: 40 }
// Frontend renders t(reasonKey, params); falls back to raw English reason
// if the key is missing (older backends / unknown rule).
const translateFactor = (factor) => {
  if (factor.reasonKey) {
    return t(factor.reasonKey, factor.params || {})
  }
  return factor.reason || ''
}

// Convert a Velero/cron schedule into a localized human phrase. We only
// recognize the presets the Advisor itself emits + a few common patterns.
// Anything else falls through to advisor.schedule.custom which keeps the
// raw cron visible so the user isn't lied to.
const SCHEDULE_PRESETS = {
  '0 * * * *': 'hourly',
  '0 */6 * * *': 'every6h',
  '0 */12 * * *': 'every12h',
  '0 0 * * *': 'daily',
  '0 0 * * 0': 'weekly',
  '0 0 1 * *': 'monthly'
}
const formatSchedule = (cron) => {
  if (!cron) return t('advisor.schedule.none')
  const preset = SCHEDULE_PRESETS[cron.trim()]
  if (preset) return t(`advisor.schedule.${preset}`)
  return t('advisor.schedule.custom', { cron })
}

const fetchAdvisor = async () => {
  loading.value = true
  try {
    const res = await getBackupAdvisor()
    // Sort by score desc so the "needs attention most" rows float to top.
    recommendations.value = (res.data?.items || []).sort((a, b) => b.score - a.score)
  } catch (e) {
    ElMessage.error('Failed to load Backup Advisor recommendations')
    console.error(e)
  } finally {
    loading.value = false
  }
}

// Apply Recommendation deep-links to Policies with prefill query params.
// Policies.vue v0.7.6+ will read these to prefill the Create form; for now
// we just navigate and let the user fill in. (See ROADMAP v0.7.5: "Apply
// Recommendation button" — human-in-the-loop, never auto-apply.)
const apply = (row) => {
  router.push({
    path: '/policies',
    query: {
      prefillFromAdvisor: '1',
      namespace: row.namespace,
      schedule: row.recommendedSchedule,
      retention: row.recommendedTTL,
      tier: row.tier
    }
  })
}

onMounted(fetchAdvisor)
</script>

<style scoped>
.page-header { margin-bottom: 16px; }
.page-header h3 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
}
.page-desc {
  margin: 0;
  color: var(--sk-text-caption);
  font-size: 13px;
  max-width: 720px;
}

.tier-summary { margin-top: 4px; }
.tier-card { text-align: center; }
.tier-card-body { padding: 4px 0; }
.tier-count { font-size: 28px; font-weight: 700; }
.tier-label { font-size: 12px; color: var(--sk-text-caption); margin-top: 2px; }
.tier-card.tier-high .tier-count { color: var(--sk-status-error); }
.tier-card.tier-medium .tier-count { color: var(--sk-status-warning); }
.tier-card.tier-low .tier-count { color: #409eff; }
.tier-card.tier-skip .tier-count { color: var(--sk-text-caption); }

.advisor-skip-notice { margin-top: 16px; }

.score-pill {
  display: inline-block;
  min-width: 36px;
  padding: 2px 8px;
  border-radius: 10px;
  text-align: center;
  font-weight: 600;
  font-size: 13px;
}
.score-high { background: #fef0f0; color: var(--sk-status-error); }
.score-medium { background: #fdf6ec; color: #b88230; }
.score-low { background: #ecf5ff; color: #337ecc; }
.score-skip { background: #f4f4f5; color: var(--sk-text-caption); }

.tier-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  font-weight: 500;
}
.tier-chip.tier-high { color: var(--sk-status-error); }
.tier-chip.tier-medium { color: #b88230; }
.tier-chip.tier-low { color: #337ecc; }
.tier-chip.tier-skip { color: var(--sk-text-caption); }

.schedule-cell { display: flex; flex-direction: column; gap: 2px; }
.schedule-human { font-size: 13px; color: var(--sk-text-secondary); font-weight: 500; }
.schedule-cron {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 11px;
  color: var(--sk-text-caption);
  background: transparent;
  padding: 0;
}

.factor-list {
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
  color: var(--sk-text-muted);
  line-height: 1.7;
}
.factor-plus { color: var(--sk-status-success); font-weight: 600; font-family: 'SF Mono', Menlo, monospace; }
.factor-minus { color: var(--sk-status-error); font-weight: 600; font-family: 'SF Mono', Menlo, monospace; }
.muted { color: var(--sk-text-placeholder); font-size: 12px; }
</style>
