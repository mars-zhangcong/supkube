<!--
  Observability (v0.9.1.2)
  ────────────────────────────────────────────────────────────────────
  Hub page consolidating the 4 "what's happening in my backup estate"
  views into one navigation entry, matching Datadog / Grafana mental
  model. Previously these lived as 3 separate top-level sidebar menus:

    "备份顾问"       /advisor      → Backup Advisor          (this tab)
    "活动查看"       /activity     → Activity                (this tab)
    "审计日志"       Settings tab  → Audit Log               (this tab)
    (new v0.8.14)                 → Log Viewer              (this tab)

  Why the consolidation
  ─────────────────────
  · Reduces sidebar from 10 → 7 entries — easier scan
  · Matches the customer mental model: "show me what happened" not "where
    is the Activity menu"
  · Single landing place for the v0.8.14 Log Viewer instead of yet another
    top-level menu

  The component re-uses existing views as tab panes via lazy import — no
  duplicate code, no behavior drift.

  Routing
  ───────
  Top-level route: /observability (lands on default tab = Activity)
  Sub-tab is held in component state, NOT URL — keeps deep-linking simple
  for the MVP. Future v0.9.x can promote tab to query param if needed.

  Old top-level routes /advisor and /activity are KEPT as redirects to
  /observability with the right tab pre-selected, so bookmarks from
  v0.8.x still work.
-->

<template>
  <div class="observability-page">
    <div class="obs-header">
      <h2 class="sk-h1" style="margin: 0">{{ t('nav.observability') }}</h2>
      <p class="sk-caption" style="margin: 4px 0 0 0">{{ t('observability.subtitle') }}</p>
    </div>

    <el-tabs v-model="activeTab" class="obs-tabs">
      <el-tab-pane name="dr">
        <template #label>
          <span class="obs-tab-label"><el-icon><Odometer /></el-icon> {{ t('observability.tabs.dr') }}</span>
        </template>
        <!-- DR 健康评分 folded in from the retired standalone /dr-scores page.
             Same DRScores.vue view, embedded as a tab pane (no duplicate code). -->
        <DRScoresView v-if="activeTab === 'dr'" />
      </el-tab-pane>

      <el-tab-pane name="activity">
        <template #label>
          <span class="obs-tab-label"><el-icon><DataLine /></el-icon> {{ t('observability.tabs.activity') }}</span>
        </template>
        <ActivityView v-if="activeTab === 'activity'" />
      </el-tab-pane>

      <el-tab-pane name="advisor">
        <template #label>
          <span class="obs-tab-label"><el-icon><MagicStick /></el-icon> {{ t('observability.tabs.advisor') }}</span>
        </template>
        <BackupAdvisorView v-if="activeTab === 'advisor'" />
      </el-tab-pane>

      <el-tab-pane name="audit">
        <template #label>
          <span class="obs-tab-label"><el-icon><Document /></el-icon> {{ t('observability.tabs.audit') }}</span>
        </template>
        <!-- Audit Log lives inside Settings panel today; embed the same
             component here. The Settings → Audit Log tab still works for
             legacy access; this is the new canonical location. -->
        <AuditLogPanel v-if="activeTab === 'audit'" />
      </el-tab-pane>

      <el-tab-pane name="logs">
        <template #label>
          <span class="obs-tab-label">
            <el-icon><Monitor /></el-icon> {{ t('observability.tabs.logs') }}
          </span>
        </template>
        <!-- v0.9.x #79: real Log Viewer (shipped). The placeholder card
             that lived here is gone — see git history if you need it. -->
        <LogViewer v-if="activeTab === 'logs'" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { DataLine, MagicStick, Document, Monitor, Odometer } from '@element-plus/icons-vue'

// Lazy-load the existing top-level views as tab panes. Keeps the bundle
// chunk-split — opening Observability doesn't drag in Advisor JS until
// the tab is actually clicked.
const DRScoresView      = defineAsyncComponent(() => import('./DRScores.vue'))
const ActivityView      = defineAsyncComponent(() => import('./Activity.vue'))
const BackupAdvisorView = defineAsyncComponent(() => import('./BackupAdvisor.vue'))
const AuditLogPanel     = defineAsyncComponent(() => import('../components/AuditLogPanel.vue'))
const LogViewer         = defineAsyncComponent(() => import('../components/LogViewer.vue'))

const { t } = useI18n()
const route = useRoute()

// Default tab = DR 健康评分 (the flagship posture view, folded in from the
// retired standalone /dr-scores nav entry — opening 可观测性 lands on it,
// preserving the old one-click-to-DR-health behavior). Old /activity,
// /advisor, /dr-scores routes redirect here with ?tab= so bookmarks keep working.
const activeTab = ref('dr')

onMounted(() => {
  const tabParam = route.query.tab
  if (typeof tabParam === 'string' && ['dr', 'activity', 'advisor', 'audit', 'logs'].includes(tabParam)) {
    activeTab.value = tabParam
  }
})
</script>

<style scoped>
.observability-page { padding: 8px 0; }

.obs-header {
  margin-bottom: 20px;
}

.obs-tabs {
  /* Slightly larger tab labels — Observability is a hub, not a sub-page */
}
.obs-tabs :deep(.el-tabs__nav-wrap::after) {
  background-color: var(--sk-border-light, #e7e9ec);
}
.obs-tabs :deep(.el-tabs__item) {
  font-size: 14px;
  padding: 0 18px;
  height: 44px;
  line-height: 44px;
}
.obs-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.obs-coming-soon {
  text-align: center;
  padding: 60px 20px;
  color: var(--sk-text-caption, #6b7280);
}
.obs-cs-icon {
  font-size: 48px;
  color: var(--sk-primary, #4f46e5);
  margin-bottom: 16px;
}
.obs-coming-soon h3 {
  font-size: 18px;
  margin: 0 0 8px;
  color: var(--sk-text, #1f2937);
}
.obs-coming-soon p {
  margin: 0 auto 16px;
  max-width: 520px;
}
.obs-coming-soon ul {
  list-style: none;
  padding: 0;
  margin: 16px auto;
  max-width: 520px;
  text-align: left;
}
.obs-coming-soon li {
  padding: 6px 0;
  padding-left: 24px;
  position: relative;
}
.obs-coming-soon li::before {
  content: '✓';
  position: absolute;
  left: 4px;
  color: var(--sk-primary, #4f46e5);
  font-weight: bold;
}
</style>
