<!--
  ActionDetailDrawer (rewritten in v0.8.10.2 per UI_GUIDELINES §5).

  Layout (Kasten-style, single source of truth across all entity drawers):
    - H1 small caps title ("ACTION DETAILS" / "RESTORE POINT DETAILS")
      controlled by the :entityTitle prop. Parent picks the wording.
    - H2 large bold subject name (action.id, mono)
    - Chip row: status + type + role (snapshot/export half)
    - Paired-with banner (dual-policy halves only)
    - Section: PHASES — H3 + phase list
    - Section: DETAILS — H3 + field grid (refs, timings, run instant)
    - Section: ARTIFACTS — H3 + stat strip + grouped breakdown (NO group emojis)
    - Section: ERRORS (failed/partial) — H3 + per-source error blocks
    - Sticky footer: [View YAML] [Restore] (Backup only) [Close]

  Width 560px from CSS token --sk-drawer-width. Header hidden because
  we render our own; Element Plus's default title bar bypasses our H1/H2
  rule, so we set :with-header="false" and own the top-right close button.
-->
<template>
  <el-drawer
    v-model="visibleProxy"
    direction="rtl"
    :size="'560px'"
    :close-on-click-modal="true"
    :with-header="false"
    @opened="onOpened"
  >
    <div v-if="!action" class="sk-drawer-empty sk-body">{{ t('activity.detail.empty') }}</div>
    <div v-else class="sk-drawer">
      <!-- ════ Header zone (H1 + H2 + chips) ════ -->
      <button class="sk-drawer-close" type="button" @click="visibleProxy = false" aria-label="Close">×</button>

      <div class="sk-drawer-header">
        <div class="sk-h1">{{ entityTitleResolved }}</div>
        <div class="sk-h2 sk-mono sk-drawer-subject" :title="action.id">{{ action.id }}</div>

        <div class="sk-drawer-chips">
          <span class="sk-chip" :class="`sk-chip-status-${statusChipKey(action.status)}`">
            {{ t(`activity.status.${action.status}`) }}
          </span>
          <span class="sk-chip sk-chip-status-muted">{{ action.type }}</span>
          <!-- v0.8.12 LBS2: role values renamed to local/cloud; accept
               both for backwards compat with mixed-version deployments
               and unmigrated existing Backups. -->
          <span v-if="action.role === 'local' || action.role === 'snapshot'" class="sk-chip sk-chip-type-snapshot">
            {{ t('activity.detail.roleSnapshot') }}
          </span>
          <span v-else-if="action.role === 'cloud' || action.role === 'export'" class="sk-chip sk-chip-type-exported">
            {{ t('activity.detail.roleExport') }}
          </span>
        </div>
      </div>

      <!-- ════ Paired-with banner — dual policy halves only ════ -->
      <div v-if="action.pairedAction" class="sk-paired-banner">
        <span class="sk-paired-label">{{ t('activity.detail.pairedWith') }}:</span>
        <a class="sk-link" @click="goPaired(action.pairedAction)">
          {{ action.pairedAction.name }}
        </a>
        <el-tooltip placement="top" :content="t('activity.detail.pairedTooltip')">
          <span class="sk-help">?</span>
        </el-tooltip>
      </div>

      <!-- ════ Section: PHASES ════ -->
      <section class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.phases') }}</div>
        <ActionPhases :phases="action.phases" />
      </section>

      <!-- ════ Section: DETAILS — refs + timings ════ -->
      <section class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.detailsSection') }}</div>
        <div class="sk-fields">
          <template v-if="action.protectedObject">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.protectedObject') }}</div>
            <div class="sk-field-value sk-body">
              <a class="sk-link" @click="goRef(action.protectedObject)">{{ action.protectedObject.name }}</a>
            </div>
          </template>
          <template v-if="action.policy">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.policy') }}</div>
            <div class="sk-field-value sk-body">
              <a class="sk-link" @click="goRef(action.policy)">{{ action.policy.name }}</a>
            </div>
          </template>
          <template v-if="action.restorePoint">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.restorePoint') }}</div>
            <div class="sk-field-value sk-body">
              <a class="sk-link" @click="goRef(action.restorePoint)">{{ action.restorePoint.name }}</a>
            </div>
          </template>
          <template v-if="action.targetNamespace">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.targetNamespace') }}</div>
            <div class="sk-field-value sk-body">{{ action.targetNamespace }}</div>
          </template>
          <template v-if="action.policyRunInstant">
            <div class="sk-field-label sk-caption">
              {{ t('activity.detail.policyRunAt') }}
              <el-tooltip placement="top" :content="t('activity.detail.policyRunAtTooltip')">
                <span class="sk-help">?</span>
              </el-tooltip>
            </div>
            <div class="sk-field-value sk-body-strong">{{ formatTime(action.policyRunInstant) }}</div>
          </template>
          <template v-if="action.startTime">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.start') }}</div>
            <div class="sk-field-value sk-body">{{ formatTime(action.startTime) }}</div>
          </template>
          <template v-if="action.endTime">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.end') }}</div>
            <div class="sk-field-value sk-body">{{ formatTime(action.endTime) }}</div>
          </template>
          <template v-if="action.durationSeconds">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.duration') }}</div>
            <div class="sk-field-value sk-body">{{ formatDuration(action.durationSeconds) }}</div>
          </template>
          <template v-if="action.errors > 0 || action.warnings > 0">
            <div class="sk-field-label sk-caption">{{ t('activity.detail.health') }}</div>
            <div class="sk-field-value">
              <span v-if="action.errors > 0" class="sk-health-err">{{ action.errors }} {{ t('activity.errors') }}</span>
              <span v-if="action.warnings > 0" class="sk-health-warn" style="margin-left:10px">{{ action.warnings }} {{ t('activity.warnings') }}</span>
            </div>
          </template>
        </div>
      </section>

      <!-- ════ Section: KUBECTL — copy-paste shortcut to inspect the
                  underlying CR directly. Same block style as
                  Application Details. v0.8.10.3 per UI_GUIDELINES. ════ -->
      <section class="sk-section">
        <div class="sk-h3">kubectl</div>
        <div class="kubectl-block">
          <code>{{ kubectlCommand }}</code>
          <el-button size="small" text class="copy-btn" @click="copyKubectl">copy</el-button>
        </div>
      </section>

      <!-- ════ Section: ARTIFACTS — only when meaningful ════ -->
      <section v-if="action.type === 'Backup' || action.artifacts" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.artifacts') }}</div>
        <!-- v0.8.10.1 Artifacts — Kasten-style grouped breakdown.
             Replaces the old single-row "specs: N" table with a real
             component-by-component view. Data comes from /backups/:name/
             artifact-breakdown (loaded lazily when drawer opens for a
             Backup type). Falls back to the v0.8.0 raw count if the
             breakdown endpoint failed or hasn't loaded yet — never
             leave the user staring at a blank Artifacts row. -->
        <!-- Loading shimmer -->
        <div v-if="breakdownLoading" class="sk-loading sk-caption">
          <el-icon class="spin"><Loading /></el-icon> {{ t('activity.detail.loadingDetail') }}
        </div>

        <!-- Loaded: stat strip + grouped breakdown -->
        <div v-else-if="breakdown && breakdown.groups && breakdown.groups.length > 0">
          <!-- Horizontal stat strip: Application · Infrastructure · Velero total -->
          <div class="sk-art-strip">
            <span class="sk-art-stat">
              <span class="sk-art-stat-num">{{ breakdown.applicationItems }}</span>
              <span class="sk-art-stat-label">{{ t('activity.detail.applicationItems') }}</span>
            </span>
            <span v-if="breakdown.infrastructureItems > 0" class="sk-art-stat">
              <span class="sk-art-stat-num sk-art-stat-infra">{{ breakdown.infrastructureItems }}</span>
              <span class="sk-art-stat-label">{{ t('activity.detail.infrastructureItems') }}</span>
            </span>
            <el-tooltip v-if="breakdown.veleroTotalItems > 0" placement="top" :content="t('activity.detail.veleroCountTooltip')">
              <span class="sk-art-stat sk-art-stat-velero-wrap">
                <span class="sk-art-stat-num sk-art-stat-velero">{{ breakdown.veleroTotalItems }}</span>
                <span class="sk-art-stat-label">{{ t('activity.detail.veleroTotalItems') }}</span>
              </span>
            </el-tooltip>
          </div>

          <!-- Group accordion. NO emoji here — per UI_GUIDELINES §3.1
               section/group headers are pure text. Application categories
               first, then infrastructure (order enforced by backend). -->
          <div class="sk-art-groups">
            <div
              v-for="g in breakdown.groups"
              :key="g.key"
              class="sk-art-group"
              :class="`sk-art-cat-${g.category}`"
            >
              <button class="sk-art-group-header" type="button" @click="toggleGroup(g.key)">
                <span class="sk-art-group-label sk-body-strong">{{ g.label }}</span>
                <span class="sk-art-group-count">{{ g.count }}</span>
                <span class="sk-art-group-chevron">{{ expandedGroups[g.key] ? '▾' : '▸' }}</span>
              </button>
              <div v-if="expandedGroups[g.key]" class="sk-art-group-items">
                <div v-for="(item, i) in g.items" :key="`${g.key}-${i}`" class="sk-art-item">
                  <span class="sk-art-item-kind sk-caption">{{ item.kind }}</span>
                  <span class="sk-art-item-name sk-body">{{ item.name }}</span>
                  <span v-if="item.namespace" class="sk-art-item-ns sk-caption">{{ item.namespace }}</span>
                  <!-- v0.8.10.3 </> icon — click opens YamlPreviewModal -->
                  <button
                    class="sk-art-item-yaml"
                    type="button"
                    :title="t('activity.detail.viewYamlItem')"
                    @click="openYamlPreview(item)"
                  ><el-icon><Document /></el-icon></button>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer note: explains gap between Velero count + our sweep.
               No emoji prefix per §3.1; muted card style. -->
          <div v-if="breakdown.note" class="sk-art-note sk-caption">{{ breakdown.note }}</div>
        </div>

        <!-- Fallback: legacy single-row count when breakdown unavailable
             (source ns deleted, endpoint failed, etc.). -->
        <div v-else class="sk-fallback-stat sk-body">
          <span class="sk-body-strong">{{ action.artifacts?.total ?? 0 }}</span>
          <span class="sk-caption" style="margin-left:6px">{{ t('activity.detail.artifactQty').toLowerCase() }}</span>
        </div>
      </section>

      <!-- ════ Section: ERRORS — Backup-side (DataUpload + PVB) ════ -->
      <section v-if="action.type === 'Backup' && action.errors > 0" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.errorsHeader', { n: action.errors }) }}</div>
        <div v-if="backupErrorsLoading" class="sk-loading sk-caption">
          <el-icon class="spin"><Loading /></el-icon> {{ t('activity.detail.loadingDetail') }}
        </div>
        <div v-else-if="backupErrors && backupErrors.errors && backupErrors.errors.length > 0">
          <div v-for="(entry, i) in backupErrors.errors" :key="`be-${i}`" class="sk-msg-group">
            <div class="sk-msg-group-label sk-caption">
              {{ entry.source }}<template v-if="entry.namespace || entry.sourcePvc"> · {{ entry.namespace || '—' }}/{{ entry.sourcePvc || '?' }}</template>
              <span v-if="entry.name" class="sk-msg-entry-name">{{ entry.name }}</span>
            </div>
            <pre class="sk-msg-block sk-msg-err">{{ entry.message }}</pre>
          </div>
        </div>
        <div v-else class="sk-msg-block sk-msg-warn sk-body">
          <span class="sk-body-strong">{{ t('activity.detail.fetchErrorPrefix') }}:</span>
          {{ backupErrors?.fetchError || t('activity.detail.noBackupErrors') }}
        </div>
        <div v-if="backupErrors?.fetchError && backupErrors?.errors?.length > 0" class="sk-msg-block sk-msg-warn sk-body" style="margin-top:8px">
          {{ backupErrors.fetchError }}
        </div>
      </section>

      <!-- ════ Section: WARNINGS — Backup-side (BSL <backup>-results.gz) ════ -->
      <section v-if="action.type === 'Backup' && action.warnings > 0" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.warningsHeader', { n: action.warnings }) }}</div>
        <div v-if="backupErrorsLoading" class="sk-loading sk-caption">
          <el-icon class="spin"><Loading /></el-icon> {{ t('activity.detail.loadingDetail') }}
        </div>
        <div v-else-if="backupErrors && backupErrors.warnings && backupErrors.warnings.length > 0">
          <div v-for="(entry, i) in backupErrors.warnings" :key="`bw-${i}`" class="sk-msg-group">
            <div class="sk-msg-group-label sk-caption">
              {{ entry.source }}<template v-if="entry.namespace"> · {{ entry.namespace }}</template>
            </div>
            <pre class="sk-msg-block sk-msg-warn">{{ entry.message }}</pre>
          </div>
        </div>
      </section>

      <!-- ════ Section: ERRORS / WARNINGS — Restore-side ════ -->
      <section v-if="action.type === 'Restore' && action.errors > 0" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.errorsHeader', { n: action.errors }) }}</div>
        <div v-if="detailedLoading" class="sk-loading sk-caption">
          <el-icon class="spin"><Loading /></el-icon> {{ t('activity.detail.loadingDetail') }}
        </div>
        <div v-else-if="detailedResults?.errors">
          <div v-for="group in messageGroups(detailedResults.errors)" :key="`e-${group.label}`" class="sk-msg-group">
            <div class="sk-msg-group-label sk-caption">{{ group.label }}</div>
            <ul class="sk-msg-block sk-msg-err">
              <li v-for="(m, i) in group.messages" :key="i">{{ m }}</li>
            </ul>
          </div>
        </div>
        <div v-else-if="detailedError">
          <div class="sk-msg-block sk-msg-err sk-body">
            <span class="sk-body-strong">{{ t('activity.detail.fetchErrorPrefix') }}:</span> {{ detailedError }}
          </div>
          <!-- v0.9.1.10 (#103): never strand the user at "N errors". Tell them
               where the messages live + the exact command to read them. -->
          <p class="sk-caption" style="margin:10px 0 6px">{{ t('activity.detail.detailFetchFallbackHint') }}</p>
          <div class="kubectl-block">
            <code>{{ restoreInspectCommand }}</code>
            <el-button size="small" text class="copy-btn" @click="copyText(restoreInspectCommand)">copy</el-button>
          </div>
        </div>
      </section>
      <section v-if="action.type === 'Restore' && action.warnings > 0" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.warningsHeader', { n: action.warnings }) }}</div>
        <div v-if="detailedLoading" class="sk-loading sk-caption">
          <el-icon class="spin"><Loading /></el-icon> {{ t('activity.detail.loadingDetail') }}
        </div>
        <div v-else-if="detailedResults?.warnings">
          <div v-for="group in messageGroups(detailedResults.warnings)" :key="`w-${group.label}`" class="sk-msg-group">
            <div class="sk-msg-group-label sk-caption">{{ group.label }}</div>
            <ul class="sk-msg-block sk-msg-warn">
              <li v-for="(m, i) in group.messages" :key="i">{{ m }}</li>
            </ul>
          </div>
        </div>
        <div v-else-if="detailedError">
          <div class="sk-msg-block sk-msg-warn sk-body">
            <span class="sk-body-strong">{{ t('activity.detail.fetchErrorPrefix') }}:</span> {{ detailedError }}
          </div>
          <p class="sk-caption" style="margin:10px 0 6px">{{ t('activity.detail.detailFetchFallbackHint') }}</p>
          <div class="kubectl-block">
            <code>{{ restoreInspectCommand }}</code>
            <el-button size="small" text class="copy-btn" @click="copyText(restoreInspectCommand)">copy</el-button>
          </div>
        </div>
      </section>

      <!-- ════ YAML inline block (collapsed by default; toggled from footer) ════ -->
      <section v-if="showYaml" class="sk-section">
        <div class="sk-h3">{{ t('activity.detail.viewYaml') }}</div>
        <pre class="sk-yaml-block">{{ rawYaml }}</pre>
      </section>
    </div>

    <!-- ════ Sticky bottom action bar — visible whenever an action is selected ════ -->
    <div v-if="action" class="sk-drawer-footer">
      <el-button size="default" @click="showYaml = !showYaml">
        {{ showYaml ? t('activity.detail.hideYaml') : t('activity.detail.viewYaml') }}
      </el-button>
      <span class="sk-drawer-footer-spacer"></span>
      <el-button size="default" @click="visibleProxy = false">
        {{ t('common.close') }}
      </el-button>
    </div>
  </el-drawer>

  <!-- v0.8.10.3: shared YAML preview modal for all artifact rows.
       Lives outside el-drawer so its overlay stacks above the drawer. -->
  <YamlPreviewModal
    v-model:visible="yamlPreviewOpen"
    :kind="yamlPreviewTarget.kind"
    :name="yamlPreviewTarget.name"
    :namespace="yamlPreviewTarget.namespace"
  />
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Box, Document, Loading, Link } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getAction, getRestoreResults, getBackupErrors, getBackupArtifactBreakdown } from '../api/velero'
import ActionPhases from './ActionPhases.vue'
import YamlPreviewModal from './YamlPreviewModal.vue'

const router = useRouter()
const { t } = useI18n()

const props = defineProps({
  visible: { type: Boolean, default: false },
  action: { type: Object, default: null }, // Action summary from list; we'll fetch full on open
  // v0.8.10.2: parent picks the H1 label per UI_GUIDELINES §5.1. Pass
  // an i18n KEY (not the resolved string) so locale swaps stay live.
  //   - "activity.detail.title"             → "ACTION DETAILS" (default)
  //   - "activity.detail.titleRestorePoint" → "RESTORE POINT DETAILS"
  //   - "activity.detail.titleApplication"  → "APPLICATION DETAILS"
  // Falls back to the legacy "activity.detail.title" if not provided.
  entityTitleKey: { type: String, default: 'activity.detail.title' }
})
// v0.8.10: added 'navigate' for Paired-with deep-linking — parent
// Activity page listens and swaps the drawer's :action prop to the
// peer Backup without closing the drawer.
const emit = defineEmits(['update:visible', 'navigate'])
const visibleProxy = computed({
  get: () => props.visible,
  set: (v) => emit('update:visible', v)
})

const showYaml = ref(false)
const rawYaml = ref('')

// v0.8.3 fix: detailed error/warning messages for failed/partial Restores.
// The Action summary from /actions only carries error/warning *counts*; the
// actual messages live in object storage and require a Velero DownloadRequest
// to fetch. We reuse the v0.7.11 endpoint (GetRestoreResults) which already
// implements that whole dance.
const detailedResults = ref(null)
const detailedLoading = ref(false)
const detailedError = ref('')

// v0.8.8.1: parallel state for Backup error details. Kept separate from
// `detailedResults` because the data shape is different (Backup errors
// have source/namespace/PVC/CR-name fields; Restore errors are a flat
// per-ns list).
const backupErrors = ref(null)
const backupErrorsLoading = ref(false)

// v0.8.10.1: grouped artifact breakdown state. Loaded when the drawer
// opens for a Backup, drives the Kasten-style component-by-component
// view (replacing the old "specs: N" single-row table). Per-group
// expand state lives in expandedGroups — first group (Workloads) opens
// by default because that's what 90% of customers want to see first.
const breakdown = ref(null)
const breakdownLoading = ref(false)
const expandedGroups = ref({})
const toggleGroup = (key) => {
  expandedGroups.value[key] = !expandedGroups.value[key]
}

// v0.8.10.3: per-artifact YAML preview state. Single shared modal for
// all rows — clicking </> on any item populates `yamlPreviewTarget`
// and opens the dialog. Modal handles fetch/render itself.
const yamlPreviewOpen = ref(false)
const yamlPreviewTarget = ref({ kind: '', name: '', namespace: '' })
const openYamlPreview = (item) => {
  yamlPreviewTarget.value = {
    kind: item.kind,
    name: item.name,
    namespace: item.namespace || ''
  }
  yamlPreviewOpen.value = true
}

async function onOpened() {
  showYaml.value = false
  rawYaml.value = ''
  detailedResults.value = null
  detailedError.value = ''
  // v0.8.10.1: reset breakdown state so navigating between paired halves
  // doesn't show stale data while the new fetch is in flight.
  breakdown.value = null
  expandedGroups.value = {}
  if (!props.action) return
  // Fetch the raw CR for the YAML view. The summary in props.action is
  // already enough to render phases/refs, so we don't refetch the Action.
  try {
    const res = await getAction(props.action.id, props.action.type)
    rawYaml.value = JSON.stringify(res.data.raw, null, 2)
  } catch (e) {
    ElMessage.error('Failed to load action raw: ' + (e.response?.data?.error || e.message))
  }
  // For Restores with any errors/warnings, fetch the per-namespace detail.
  if (props.action.type === 'Restore' && (props.action.errors > 0 || props.action.warnings > 0)) {
    await fetchRestoreDetail(props.action.id)
  }
  // v0.8.8.1: For Backups with errors, fetch DataUpload + PVB messages.
  // The "TODO: do for Backup later" comment that lived here for 6 months
  // is now done.
  if (props.action.type === 'Backup' && props.action.errors > 0) {
    await fetchBackupErrorDetail(props.action.id)
  }
  // v0.8.10.1: for any Backup (including healthy ones), fetch the
  // grouped artifact breakdown so the Artifacts section can render the
  // Kasten-style component view. We don't gate on .artifacts.total > 0
  // because some Backups have no progress info yet but still have items.
  if (props.action.type === 'Backup') {
    fetchBreakdown(props.action.id)
  }
}

async function fetchBreakdown(name) {
  breakdownLoading.value = true
  try {
    const res = await getBackupArtifactBreakdown(name)
    breakdown.value = res.data
    // Default-expand the FIRST group (typically Workloads) so the user
    // sees content immediately, not a stack of collapsed accordions.
    if (breakdown.value?.groups?.length > 0) {
      expandedGroups.value[breakdown.value.groups[0].key] = true
    }
  } catch (e) {
    // Silent fail → the template falls back to the legacy single-row
    // "specs: N" view. Logging only so dev can spot the regression.
    console.warn('Artifact breakdown fetch failed:', e?.response?.data?.error || e.message)
  } finally {
    breakdownLoading.value = false
  }
}

async function fetchBackupErrorDetail(name) {
  backupErrorsLoading.value = true
  backupErrors.value = null
  try {
    const res = await getBackupErrors(name)
    backupErrors.value = res.data
  } catch (e) {
    // Even fetch-error becomes visible — we DO NOT silently fall back
    // to "no errors". Wrap as a synthetic FetchError so the UI's
    // "no errors found" branch shows the real reason.
    backupErrors.value = {
      errors: [],
      warnings: [],
      fetchError: e?.response?.data?.error || e.message || 'Failed to fetch backup errors'
    }
  } finally {
    backupErrorsLoading.value = false
  }
}

async function fetchRestoreDetail(name) {
  detailedLoading.value = true
  detailedError.value = ''
  try {
    const res = await getRestoreResults(name)
    if (res.data.detailed) {
      detailedResults.value = res.data.detailed
    } else if (res.data.detailedError) {
      detailedError.value = res.data.detailedError
    }
  } catch (e) {
    detailedError.value = e.response?.data?.error || e.message
  } finally {
    detailedLoading.value = false
  }
}

// Flatten Velero's nested Result structure into labeled groups for
// rendering. Input shape:
//   { velero: [...], cluster: [...], namespaces: { foo: [...], bar: [...] } }
// Output: [ { label: "Velero engine", messages: [...] },
//           { label: "Cluster-scoped", messages: [...] },
//           { label: "Namespace: foo", messages: [...] }, ... ]
function messageGroups(section) {
  if (!section) return []
  const groups = []
  if (section.velero?.length) groups.push({ label: 'Velero engine', messages: section.velero })
  if (section.cluster?.length) groups.push({ label: 'Cluster-scoped', messages: section.cluster })
  if (section.namespaces) {
    for (const [ns, msgs] of Object.entries(section.namespaces)) {
      if (msgs && msgs.length) groups.push({ label: `Namespace: ${ns}`, messages: msgs })
    }
  }
  return groups
}

const statusTagType = (s) => ({
  completed: 'success',
  running: 'info',
  failed: 'danger',
  partial: 'warning',
  skipped: '',
  unknown: ''
})[s] || ''

// v0.8.10.2: chip class key derived from Action.status, used by the
// .sk-chip-status-* CSS classes defined in tokens.css. Maps Action's
// states onto the global status taxonomy.
const statusChipKey = (s) => ({
  completed: 'success',
  running: 'running',
  failed: 'error',
  partial: 'warning',
  skipped: 'muted',
  unknown: 'muted'
})[s] || 'muted'

// v0.8.10.2: resolves the H1 title from the entityTitleKey prop. Kept
// as a computed so locale switches re-render automatically.
const entityTitleResolved = computed(() => t(props.entityTitleKey))

// v0.8.10.3: kubectl shortcut command for the underlying CR. Velero
// stores both Backup and Restore CRs in the `velero` ns; the only
// difference is the resource type. Empty string fallback when action
// is missing so the template doesn't render "undefined".
const kubectlCommand = computed(() => {
  if (!props.action) return ''
  const kind = (props.action.type || 'Backup').toLowerCase() // backup / restore
  return `$ kubectl -n velero get ${kind} ${props.action.id} -o yaml`
})
// v0.9.1.10 (#103): the command we surface when the BSL detail fetch fails,
// so the user can always read the per-resource error/warning messages
// manually. `velero restore describe --details` gunzips the same results
// blob SupKube tried (and failed) to fetch.
const restoreInspectCommand = computed(() => {
  if (!props.action || props.action.type !== 'Restore') return ''
  return `velero restore describe ${props.action.id} --details`
})

// Generic clipboard copy with toast. copyKubectl delegates here.
function copyText(text) {
  const cmd = (text || '').replace(/^\$ /, '')
  if (!cmd) return
  navigator.clipboard.writeText(cmd).then(
    () => ElMessage.success('Copied to clipboard'),
    () => ElMessage.error('Copy failed — your browser may have blocked clipboard access')
  )
}
function copyKubectl() {
  copyText(kubectlCommand.value)
}

const formatTime = (iso) => new Date(iso).toLocaleString()
const formatDuration = (sec) => {
  if (sec < 60) return `${sec} sec`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s > 0 ? `${m} min ${s} sec` : `${m} min`
}

const goRef = (ref) => {
  if (!ref) return
  if (ref.kind === 'Backup') {
    // Backup ref → take the user to that backup's row, filter-pinned.
    router.push({ path: '/backups', query: { name: ref.name } })
  } else if (ref.kind === 'Schedule') {
    router.push({ path: '/policies', query: { name: ref.name } })
  } else if (ref.kind === 'Namespace') {
    router.push({ path: '/applications', query: { name: ref.name } })
  }
  visibleProxy.value = false
}

// v0.8.10: Paired-with click. Emits "navigate" so the parent (Activity
// page) can swap the drawer's :action prop to the peer Action without
// closing/reopening. Pages that embed this drawer must listen for
// @navigate — see Activity.vue handlePairedNavigate().
const goPaired = (ref) => {
  if (!ref || !ref.name) return
  emit('navigate', { kind: 'Backup', name: ref.name })
}

watch(() => props.visible, (v) => {
  if (!v) {
    showYaml.value = false
    rawYaml.value = ''
  }
})
</script>

<style scoped>
/* ════════════════════════════════════════════════════════════════════
 * ActionDetailDrawer styles (v0.8.10.2 rewrite — UI_GUIDELINES §5)
 * All colours via var(--sk-*); no hardcoded hex.
 * Legacy classes preserved below the §LEGACY marker for backward
 * compatibility with code paths that still reference them.
 * ════════════════════════════════════════════════════════════════════ */

/* Drawer body padding — Element Plus injects extra padding; reset it */
:deep(.el-drawer__body) {
  padding: 0;
  display: flex;
  flex-direction: column;
}

.sk-drawer-empty {
  padding: 60px var(--sk-drawer-padding-x);
  text-align: center;
  color: var(--sk-text-caption);
}

.sk-drawer {
  flex: 1 1 auto;
  overflow-y: auto;
  padding: 28px var(--sk-drawer-padding-x) var(--sk-drawer-section-spacing);
  position: relative;
  min-height: 0; /* allow flex child to actually scroll instead of growing */
}

.sk-drawer-close {
  position: absolute;
  top: 14px;
  right: 14px;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  font-size: 22px;
  line-height: 1;
  color: var(--sk-text-muted);
  cursor: pointer;
  border-radius: 4px;
  z-index: 2;
  transition: background 120ms ease, color 120ms ease;
}
.sk-drawer-close:hover {
  background: var(--sk-bg-hover);
  color: var(--sk-text);
}

/* ─── Header zone (H1 + H2 + chips) ─── */
.sk-drawer-header {
  margin-bottom: var(--sk-drawer-header-spacing);
}
.sk-drawer-subject {
  margin-top: var(--sk-space-xs);
  margin-bottom: var(--sk-space-md);
  /* Long backup names need to wrap rather than overflow */
  word-break: break-all;
  /* Slight mono size bump so the H2 still reads as a heading */
  font-size: 18px !important;
}
.sk-drawer-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-sm);
  align-items: center;
}

/* ─── Paired-with banner (dual policy halves only) ─── */
.sk-paired-banner {
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding: 10px 14px;
  background: var(--sk-type-exported-bg);
  border-left: 3px solid var(--sk-type-exported);
  border-radius: 4px;
  margin-bottom: var(--sk-drawer-section-spacing);
  font-size: 13px;
}
.sk-paired-label { color: var(--sk-text-secondary); }

.sk-help {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  margin-left: 4px;
  border-radius: 50%;
  background: var(--sk-bg-hover);
  color: var(--sk-text-muted);
  font-size: 10px;
  font-weight: 700;
  cursor: help;
}

/* ─── Sections (H3 + content) ─── */
.sk-section {
  margin-bottom: var(--sk-drawer-section-spacing);
  padding-bottom: var(--sk-drawer-section-spacing);
  border-bottom: 1px solid var(--sk-border);
}
.sk-section:last-of-type { border-bottom: 0; padding-bottom: 0; }
.sk-section .sk-h3 { margin-bottom: var(--sk-space-md); }

/* ─── DETAILS field grid ─── */
.sk-fields {
  display: grid;
  grid-template-columns: 150px 1fr;
  row-gap: var(--sk-space-sm);
  column-gap: var(--sk-space-md);
  align-items: baseline;
}
.sk-field-label {
  text-align: left;
}
.sk-field-value {
  word-break: break-all;
}

/* ─── Universal link style ─── */
.sk-link {
  color: var(--sk-primary);
  cursor: pointer;
  text-decoration: none;
  transition: color 120ms ease;
}
.sk-link:hover {
  text-decoration: underline;
  color: var(--sk-primary-hover);
}

/* ─── Health row colours ─── */
.sk-health-err  { color: var(--sk-status-error);   font-weight: 600; }
.sk-health-warn { color: var(--sk-status-warning); font-weight: 600; }

/* ─── ARTIFACTS — horizontal stat strip ─── */
.sk-art-strip {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-xl);
  padding: 12px 14px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  border-radius: 6px;
  margin-bottom: var(--sk-space-md);
}
.sk-art-stat { display: inline-flex; align-items: baseline; gap: var(--sk-space-sm); }
.sk-art-stat-num {
  font-size: 20px;
  font-weight: 700;
  color: var(--sk-text);
  letter-spacing: -0.02em;
}
.sk-art-stat-num.sk-art-stat-infra  { color: var(--sk-text-caption); }
.sk-art-stat-num.sk-art-stat-velero { color: var(--sk-type-exported); }
.sk-art-stat-velero-wrap { cursor: help; }
.sk-art-stat-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--sk-text-caption);
}

/* ─── ARTIFACTS — group accordion ─── */
.sk-art-groups {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.sk-art-group {
  border: 1px solid var(--sk-border);
  border-radius: 6px;
  overflow: hidden;
}
.sk-art-cat-infrastructure { opacity: 0.85; }
.sk-art-group-header {
  display: grid;
  grid-template-columns: 1fr auto 14px;
  align-items: center;
  gap: var(--sk-space-md);
  padding: 10px 14px;
  background: var(--sk-bg-soft);
  cursor: pointer;
  border: none;
  width: 100%;
  text-align: left;
  font: inherit;
  user-select: none;
  transition: background 120ms ease;
}
.sk-art-group-header:hover { background: var(--sk-bg-hover); }
.sk-art-group-count {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 8px;
  border-radius: 10px;
  background: var(--sk-border);
  color: var(--sk-text-muted);
  min-width: 24px;
  text-align: center;
}
.sk-art-cat-application .sk-art-group-count {
  background: var(--sk-type-snapshot-bg);
  color: var(--sk-type-snapshot);
}
.sk-art-group-chevron { color: var(--sk-text-caption); font-size: 11px; }
.sk-art-group-items {
  padding: 4px 14px 10px;
  background: var(--sk-bg-page);
}
.sk-art-item {
  display: grid;
  /* v0.8.10.3: 4 cols — kind / name / ns / yaml-button. yaml button is
     a fixed 28px right-aligned action affordance. */
  grid-template-columns: 110px 1fr auto 28px;
  gap: var(--sk-space-md);
  align-items: center;
  padding: 4px 0;
  border-top: 1px dashed var(--sk-border-light);
}
.sk-art-item:first-child { border-top: 0; }
.sk-art-item-kind {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  /* Per UI_GUIDELINES: Kind is NOT a clickable element — must NOT be
     coloured indigo or purple. Plain caption gray. */
  color: var(--sk-text-caption);
}
.sk-art-item-name {
  color: var(--sk-text);
  font-weight: 500;
  word-break: break-all;
}
.sk-art-item-ns {
  color: var(--sk-text-caption);
}
/* v0.8.10.3: </> YAML preview button. Indigo on hover (clickable
   token from UI_GUIDELINES). Defaults to muted gray so it doesn't
   compete with the row content for attention. */
.sk-art-item-yaml {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--sk-text-caption);
  border-radius: 4px;
  transition: background 120ms ease, color 120ms ease;
}
.sk-art-item-yaml:hover {
  background: var(--sk-primary-bg-hover);
  color: var(--sk-primary);
}
.sk-art-item-yaml .el-icon {
  font-size: 14px;
}

.sk-art-note {
  margin-top: var(--sk-space-md);
  padding: 10px 14px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  border-radius: 4px;
  line-height: 1.5;
}

.sk-fallback-stat {
  display: inline-flex;
  align-items: baseline;
  padding: 10px 14px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  border-radius: 4px;
}

/* ─── kubectl shortcut block (matches Application Details exactly) ─── */
.kubectl-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 10px 14px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sk-space-sm);
}
.copy-btn { color: #a8d8a8 !important; font-size: 12px; }

/* ─── Loading state ─── */
.sk-loading {
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding: 16px 0;
}
.spin {
  animation: sk-spin 0.9s linear infinite;
}
@keyframes sk-spin { to { transform: rotate(360deg); } }

/* ─── Error / warning blocks ─── */
.sk-msg-group { margin-bottom: var(--sk-space-md); }
.sk-msg-group-label { margin-bottom: 4px; }
.sk-msg-entry-name {
  margin-left: var(--sk-space-sm);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  color: var(--sk-text-muted);
}
.sk-msg-block {
  padding: 10px 14px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
}
.sk-msg-err  { background: #fef0f0; border-left: 3px solid var(--sk-status-error);   color: var(--sk-text-secondary); }
.sk-msg-warn { background: #fdf6ec; border-left: 3px solid var(--sk-status-warning); color: var(--sk-text-secondary); }

/* ─── YAML block ─── */
.sk-yaml-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 14px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11px;
  line-height: 1.5;
  max-height: 400px;
  overflow: auto;
  margin: 0;
}

/* ─── Bottom action bar.
 *
 * NOT `position: sticky`. The drawer body is a flex column:
 *   .el-drawer__body { display:flex; flex-direction:column }
 *     ├─ .sk-drawer       { flex: 1 1 auto; overflow-y: auto }   ← scrolls
 *     └─ .sk-drawer-footer { flex: 0 0 auto }                    ← stays put
 *
 * Sticky inside a non-scrolling parent (the body itself doesn't scroll —
 * `.sk-drawer` does) silently falls back to static positioning, which is
 * why v0.8.10.2-rc1 showed the footer floating inline above RBAC. Plain
 * flex sibling is reliable. */
.sk-drawer-footer {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding: 12px var(--sk-drawer-padding-x);
  background: var(--sk-bg-page);
  border-top: 1px solid var(--sk-border);
}
.sk-drawer-footer-spacer { flex: 1; }

/* ════════════════════════════════════════════════════════════════════
 * §LEGACY — pre-v0.8.10.2 classes retained for code paths still
 * referencing them. To be removed in v0.8.11 once full audit done.
 * ════════════════════════════════════════════════════════════════════ */
.empty { padding: 40px; color: var(--sk-text-caption); text-align: center; }
.detail { padding: 0 4px; }
.detail-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}
.status-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.6px;
  padding: 3px 10px;
  border-radius: 11px;
  text-transform: uppercase;
}
.status-running   { background: #ecf5ff; color: #409eff; }
.status-completed { background: #f0f9eb; color: #67c23a; }
.status-failed    { background: #fef0f0; color: #c45656; }
.status-partial   { background: #fdf6ec; color: #e6a23c; }
.status-skipped   { background: #f4f4f5; color: #909399; }
.status-unknown   { background: #f4f4f5; color: #909399; }

.type-tag {
  font-size: 12px;
  font-weight: 600;
  color: #606266;
  background: #f5f7fa;
  padding: 2px 8px;
  border-radius: 4px;
}
.detail-name {
  font-size: 18px;
  font-weight: 600;
  color: #1d2129;
  word-break: break-all;
  margin-bottom: 18px;
}

/* v0.8.10 role chip — visible on dual-policy halves. Echoes the Restore
   Points page's Type chip palette so a user moving between the two pages
   sees consistent colour semantics. */
.role-chip {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  letter-spacing: 0.3px;
}
.role-snapshot { background: #ecf5ff; color: #409eff; }
.role-export   { background: #f5edff; color: #722ed1; }

/* v0.8.10 Paired-with banner. Above the descriptions block to flag
   "this is one half of a pair" at the top of the drawer. Subtle accent
   colour, not a warning — just informational with a click affordance. */
.paired-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  margin: -8px 0 18px 0;
  background: linear-gradient(90deg, #f3eefe 0%, #f9f6ff 100%);
  border-left: 3px solid #722ed1;
  border-radius: 4px;
  font-size: 13px;
}
.paired-text { flex: 1; color: #303133; }
.paired-text .ref-link { color: #722ed1; font-weight: 600; cursor: pointer; }
.paired-text .ref-link:hover { text-decoration: underline; }
.paired-help {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #d6c2f0;
  color: #4b1f88;
  font-size: 11px;
  font-weight: 700;
  cursor: help;
}

/* v0.8.10 unified Policy Run At timestamp — slight emphasis vs the
   regular startTime row, since this is the "logical" time both halves
   share. */
.policy-run-instant {
  font-weight: 600;
  color: #4b1f88;
}

.fields :deep(.el-descriptions__label) {
  width: 160px;
  color: #606266;
  font-weight: 500;
  vertical-align: top;
}
.fields :deep(.el-descriptions__content) {
  color: #303133;
}

.ref-link {
  color: #4f46e5;
  cursor: pointer;
  text-decoration: none;
}
.ref-link:hover { text-decoration: underline; }

.artifacts-table {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  overflow: hidden;
  width: 100%;
  max-width: 360px;
}
.art-row {
  display: grid;
  grid-template-columns: 1fr 100px;
  padding: 8px 12px;
  font-size: 13px;
  border-top: 1px solid #ebeef5;
}
.art-row:first-child { border-top: 0; }
.art-head {
  background: #f5f7fa;
  font-weight: 600;
  font-size: 11px;
  color: #909399;
  text-transform: uppercase;
}
.art-row .el-icon { vertical-align: middle; margin-right: 4px; color: #606266; }

/* ─── v0.8.10.1 Kasten-style grouped Artifacts ─────────────────── */
.art-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 12px;
  padding: 10px 14px;
  background: #fafbfc;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}
.art-stat {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  line-height: 1.2;
}
.art-stat strong {
  font-size: 18px;
  font-weight: 700;
  color: #1d2129;
}
.art-stat-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: #909399;
  margin-top: 2px;
}
.art-stat-infra strong { color: #909399; }
.art-stat-velero strong { color: #b09abc; }
.art-stat-velero { cursor: help; }

.art-groups {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.art-group {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  overflow: hidden;
}
.art-cat-infrastructure { opacity: 0.85; }
.art-group-header {
  display: grid;
  grid-template-columns: 24px 1fr auto 14px;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #f9fafc;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
}
.art-group-header:hover { background: #f0f3f8; }
.art-group-icon { font-size: 14px; line-height: 1; }
.art-group-label { font-weight: 600; color: #1d2129; }
.art-group-count {
  font-size: 11px;
  font-weight: 700;
  color: #606266;
  background: #ebeef5;
  padding: 1px 8px;
  border-radius: 10px;
  min-width: 24px;
  text-align: center;
}
.art-cat-application .art-group-count { background: #ecf5ff; color: #409eff; }
.art-cat-infrastructure .art-group-count { background: #f4f4f5; color: #909399; }
.art-group-chevron { color: #909399; font-size: 11px; }
.art-group-items {
  padding: 4px 12px 8px 44px;
  background: #fff;
  font-size: 12px;
}
.art-item {
  display: grid;
  grid-template-columns: 110px 1fr auto;
  gap: 8px;
  padding: 3px 0;
  border-top: 1px dashed #f0f3f8;
}
.art-item:first-child { border-top: 0; }
.art-item-kind {
  color: #722ed1;
  font-weight: 500;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11px;
}
.art-item-name {
  color: #1d2129;
  font-weight: 500;
  word-break: break-all;
}
.art-item-ns {
  color: #909399;
  font-size: 11px;
  font-style: italic;
}

.art-note {
  margin-top: 10px;
  padding: 8px 12px;
  background: #fdf6ec;
  border-left: 3px solid #e6a23c;
  border-radius: 3px;
  font-size: 12px;
  color: #606266;
  line-height: 1.5;
}

.health-err { color: #c45656; font-weight: 600; }
.health-warn { color: #e6a23c; font-weight: 600; }

/* v0.8.3 detailed error/warning section */
.detail-section { margin-top: 18px; }
.detail-h {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 600;
  color: #1d2129;
}
.detail-loading {
  font-size: 12.5px;
  color: #909399;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
}
.spin { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }
.msg-group { margin-bottom: 10px; }
.msg-group-label {
  font-size: 11px;
  font-weight: 600;
  color: #606266;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  margin: 4px 0;
}
.msg-block {
  margin: 4px 0;
  padding: 10px 14px 10px 28px;
  border-radius: 6px;
  border: 1px solid;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}
.msg-block li { margin: 3px 0; }
.msg-err   { background: #fef0f0; border-color: #fbc4c4; color: #5b2929; }
.msg-warn  { background: #fdf6ec; border-color: #f5dab1; color: #67430b; }
/* v0.8.8.1: <pre> variant for Backup errors. Long DataUpload errors
   need monospace + visible scrolling, not the bullet-list shape used
   for Restore per-ns messages. */
.msg-pre {
  padding: 10px 14px;
  white-space: pre-wrap;
  overflow-x: auto;
  margin: 4px 0 0 0;
}
.msg-entry-name {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 11px;
  color: #909399;
  font-weight: normal;
  margin-left: 6px;
}

/* Dark mode */
:global(html.dark) .detail-h { color: #e5eaf3; }
:global(html.dark) .msg-group-label { color: #909399; }
:global(html.dark) .msg-err   { background: #2a1a1d; border-color: #6e2a30; color: #f5b1b1; }
:global(html.dark) .msg-warn  { background: #2b1f0c; border-color: #6e5217; color: #f0c473; }

.yaml-toggle { margin-top: 16px; }
.yaml-block {
  margin: 10px 0 0 0;
  background: #1d1e26;
  color: #e5eaf3;
  padding: 12px 14px;
  border-radius: 6px;
  font-size: 11.5px;
  line-height: 1.55;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  white-space: pre;
  overflow-x: auto;
  max-height: 480px;
}

/* Dark mode */
:global(html.dark) .detail-name { color: #e5eaf3; }
:global(html.dark) .fields :deep(.el-descriptions__label) { color: #909399; }
:global(html.dark) .fields :deep(.el-descriptions__content) { color: #e5eaf3; }
:global(html.dark) .type-tag { background: #1f2026; color: #b1b3b8; }
:global(html.dark) .artifacts-table { border-color: #2c2f36; }
:global(html.dark) .art-row { border-top-color: #2c2f36; }
:global(html.dark) .art-head { background: #1f2026; color: #909399; }
</style>
