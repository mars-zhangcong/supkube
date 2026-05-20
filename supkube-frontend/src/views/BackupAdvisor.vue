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
        <el-table-column :label="t('advisor.recommendedSchedule')" min-width="180">
          <template #default="{ row }">
            <code v-if="row.recommendedSchedule">{{ row.recommendedSchedule }}</code>
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
                {{ f.reason }}
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
  color: #909399;
  font-size: 13px;
  max-width: 720px;
}

.tier-summary { margin-top: 4px; }
.tier-card { text-align: center; }
.tier-card-body { padding: 4px 0; }
.tier-count { font-size: 28px; font-weight: 700; }
.tier-label { font-size: 12px; color: #909399; margin-top: 2px; }
.tier-card.tier-high .tier-count { color: #f56c6c; }
.tier-card.tier-medium .tier-count { color: #e6a23c; }
.tier-card.tier-low .tier-count { color: #409eff; }
.tier-card.tier-skip .tier-count { color: #909399; }

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
.score-high { background: #fef0f0; color: #c45656; }
.score-medium { background: #fdf6ec; color: #b88230; }
.score-low { background: #ecf5ff; color: #337ecc; }
.score-skip { background: #f4f4f5; color: #909399; }

.tier-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  font-weight: 500;
}
.tier-chip.tier-high { color: #c45656; }
.tier-chip.tier-medium { color: #b88230; }
.tier-chip.tier-low { color: #337ecc; }
.tier-chip.tier-skip { color: #909399; }

.factor-list {
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
  color: #606266;
  line-height: 1.7;
}
.factor-plus { color: #67c23a; font-weight: 600; font-family: 'SF Mono', Menlo, monospace; }
.factor-minus { color: #f56c6c; font-weight: 600; font-family: 'SF Mono', Menlo, monospace; }
.muted { color: #c0c4cc; font-size: 12px; }
</style>
