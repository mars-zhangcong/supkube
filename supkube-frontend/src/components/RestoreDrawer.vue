<!--
  RestoreDrawer (v0.7.10) — Kasten-style restore flow.

  Opened from the kebab menu on the Restore Points list. Surfaces every
  decision the user needs in one side drawer so they don't have to bounce
  between pages:

    1. Application Name section
       — Picker of existing namespaces (defaults to the backup's source ns)
       — "Create A New Namespace" inline form (so you don't drop out to kubectl)
       — When the picker matches the source ns, a warning appears: "the
         contents of the selected namespace will be overwritten with the
         restored application." Confirming flips cleanupBeforeRestore=true.

    2. Optional Restore Settings
       — Restore name (auto-generated, editable)
       — existingResourcePolicy radio (only visible when NOT cleaning up)

    3. Spec (N) section
       — Lazy-loaded list of artifacts the restore will produce
       — Per-row checkbox controls includedResources/excludedResources
         (the kind name, in Velero's terminology)
       — Click the </> icon to expand the embedded YAML inline

  Wire-up:
    - Parent passes :backup="row" + v-model:visible
    - On submit we POST /api/v1/restores with the composed spec; parent
      handles the ElMessage and refresh.
-->
<template>
  <el-drawer
    v-model="visibleProxy"
    :title="t('restoreDrawer.title')"
    direction="rtl"
    size="640px"
    :close-on-click-modal="false"
    @opened="onOpened"
    @closed="onClosed"
  >
    <div v-if="!backup" class="empty">No backup selected.</div>
    <div v-else class="restore-drawer">
      <!-- Breadcrumb back -->
      <button class="back-link" type="button" @click="visibleProxy = false">
        <el-icon><ArrowLeft /></el-icon> {{ t('restoreDrawer.backToDetails') }}
      </button>

      <!-- ===== v0.9.0 MC3: Target Cluster — only when ≥2 clusters registered ===== -->
      <section v-if="cluster.clusters.value.length > 1" class="section">
        <h4 class="section-title">{{ t('restoreDrawer.targetClusterTitle') }}</h4>
        <p class="section-desc">{{ t('restoreDrawer.targetClusterDesc') }}</p>

        <el-select v-model="targetCluster" filterable class="ns-select">
          <el-option
            v-for="c in selectableClusters"
            :key="c.name"
            :label="`${c.displayName || c.name}${c.type === 'primary' ? ' ★' : ''}`"
            :value="c.name"
            :disabled="c.phase !== 'Healthy' && c.name !== 'this-cluster'"
          >
            <span style="float: left">
              {{ c.displayName || c.name }}
              <span v-if="c.type === 'primary'" class="cluster-primary-badge">★</span>
            </span>
            <span style="float: right; color: #909399; font-size: 12px">
              {{ c.phase }}
            </span>
          </el-option>
        </el-select>

        <div v-if="isCrossCluster" class="cross-cluster-banner">
          ⚠ {{ t('restoreDrawer.crossClusterBanner') }}
        </div>
      </section>

      <!-- ===== Application Name ===== -->
      <section class="section">
        <h4 class="section-title">{{ t('restoreDrawer.appNameTitle') }}</h4>
        <p class="section-desc">{{ t('restoreDrawer.appNameDesc') }}</p>

        <div class="ns-row">
          <el-select
            v-model="targetNs"
            filterable
            :placeholder="t('restoreDrawer.selectNs')"
            class="ns-select"
            @change="onTargetChange"
          >
            <template #prefix><el-icon class="ns-icon"><Folder /></el-icon></template>
            <el-option
              v-for="ns in namespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            >
              <template #default>
                <span class="ns-option">
                  <el-icon><Folder /></el-icon>
                  {{ ns }}
                  <el-icon v-if="ns === sourceNs" class="ns-source-mark" :title="t('restoreDrawer.originalNs')">
                    <CircleCheckFilled />
                  </el-icon>
                </span>
              </template>
            </el-option>
          </el-select>
          <button class="create-ns-link" type="button" @click="showCreateNs = !showCreateNs">
            <el-icon><CirclePlus /></el-icon> {{ t('restoreDrawer.createNewNs') }}
          </button>
        </div>

        <div v-if="showCreateNs" class="new-ns-row">
          <div class="new-ns-label">{{ t('restoreDrawer.newNsLabel') }}</div>
          <div class="new-ns-controls">
            <el-input
              v-model="newNsName"
              :placeholder="t('restoreDrawer.newNsPlaceholder')"
              class="new-ns-input"
              @keyup.enter="handleCreateNs"
            />
            <el-button type="success" :loading="creatingNs" @click="handleCreateNs">
              {{ t('common.create') }}
            </el-button>
            <el-button @click="showCreateNs = false; newNsName = ''">
              {{ t('common.cancel') }}
            </el-button>
          </div>
        </div>

        <div v-if="isOverwriteInPlace" class="overwrite-warning">
          <el-icon class="warn-icon"><WarningFilled /></el-icon>
          <div class="warn-body">
            <div class="warn-title">{{ t('restoreDrawer.overwriteTitle') }}</div>
            <div class="warn-desc">{{ t('restoreDrawer.overwriteDesc', { ns: targetNs }) }}</div>
            <el-checkbox v-model="confirmOverwrite">
              {{ t('restoreDrawer.overwriteConfirm') }}
            </el-checkbox>
          </div>
        </div>
      </section>

      <!-- ===== Optional Settings ===== -->
      <section class="section">
        <h4 class="section-title">{{ t('restoreDrawer.optionsTitle') }}</h4>

        <div class="opt-row">
          <label class="opt-label">{{ t('restoreDrawer.restoreName') }}</label>
          <el-input v-model="restoreName" size="default" />
        </div>

        <!-- v0.8.2: Transform Set picker. Lets advanced users explicitly
             apply a pre-built Transform Set (strip-nodeport, etc.) to the
             Restore without going through Pre-flight Apply Fix. -->
        <div class="opt-row">
          <label class="opt-label">
            {{ t('restoreDrawer.transformSet') }}
            <a class="opt-help-link" :href="'#'" @click.prevent="goManageTransformSets">
              {{ t('restoreDrawer.transformSetManage') }}
            </a>
          </label>
          <el-select
            v-model="selectedTransformSet"
            clearable
            filterable
            :placeholder="t('restoreDrawer.transformSetNone')"
          >
            <el-option
              v-for="ts in transformSets"
              :key="ts.name"
              :label="ts.name"
              :value="ts.name"
            >
              <span>{{ ts.name }}</span>
              <span v-if="ts.builtIn" class="ts-builtin-badge">{{ t('restoreDrawer.transformSetBuiltinBadge') }}</span>
            </el-option>
          </el-select>
          <div v-if="selectedTransformSetObj" class="opt-help">{{ selectedTransformSetObj.description }}</div>
        </div>

        <div v-if="!isOverwriteInPlace" class="opt-row">
          <label class="opt-label">{{ t('restoreDrawer.existingPolicy') }}</label>
          <el-radio-group v-model="existingPolicy" class="policy-radios">
            <el-radio value="" class="policy-radio">
              <div class="policy-text">
                <div class="policy-name">{{ t('restoreDrawer.policySkip') }}</div>
                <div class="policy-hint">{{ t('restoreDrawer.policySkipHint') }}</div>
              </div>
            </el-radio>
            <el-radio value="update" class="policy-radio">
              <div class="policy-text">
                <div class="policy-name">{{ t('restoreDrawer.policyUpdate') }}</div>
                <div class="policy-hint">{{ t('restoreDrawer.policyUpdateHint') }}</div>
              </div>
            </el-radio>
          </el-radio-group>
        </div>
      </section>

      <!-- ===== Pre-flight Check (v0.7.12) ===== -->
      <!-- Auto-runs when the user changes the target ns. Lists every
           structural conflict between the backup artifacts and the live
           target cluster (NodePort, clusterIP, Ingress host, StorageClass,
           PV binding, image registry). v0.8 will turn each suggestedTransform
           into a one-click "Apply Fix" action. -->
      <section v-if="preflightVisible" class="section">
        <div class="spec-header">
          <h4 class="section-title spec-title">
            {{ t('restoreDrawer.preflightTitle') }}
            <span v-if="!preflightLoading && preflight" class="pf-summary" :class="pfSeverityClass">
              {{ pfSummaryText }}
            </span>
          </h4>
          <button class="link-btn" type="button" :disabled="preflightLoading" @click="runPreflight(true)">
            <el-icon><RefreshRight /></el-icon> {{ t('restoreDrawer.preflightRerun') }}
          </button>
        </div>

        <div class="pf-deep-toggle">
          <el-checkbox v-model="deepCheck" :disabled="preflightLoading" @change="runPreflight(true)">
            {{ t('restoreDrawer.preflightDeepCheck') }}
          </el-checkbox>
          <span class="pf-deep-hint">{{ t('restoreDrawer.preflightDeepHint') }}</span>
        </div>

        <div v-if="preflightLoading" class="artifacts-loading">
          <el-icon class="rotating"><Loading /></el-icon>
          {{ t('restoreDrawer.preflightChecking') }}
        </div>

        <div v-else-if="preflightError" class="pf-error-card">
          <el-icon><WarningFilled /></el-icon>
          {{ t('restoreDrawer.preflightError') }}: {{ preflightError }}
        </div>

        <div v-else-if="conflicts.length === 0" class="pf-clean">
          <el-icon class="pf-clean-icon"><CircleCheckFilled /></el-icon>
          <div>
            <div class="pf-clean-title">{{ t('restoreDrawer.preflightClean') }}</div>
            <div class="pf-clean-desc">{{ t('restoreDrawer.preflightCleanDesc') }}</div>
          </div>
        </div>

        <div v-else class="conflicts-list">
          <div
            v-for="(c, idx) in conflicts"
            :key="idx"
            class="conflict-card"
            :class="`severity-${c.severity}`"
          >
            <div class="conflict-header">
              <el-icon class="conflict-icon">
                <WarningFilled v-if="c.severity === 'blocker'" />
                <InfoFilled v-else />
              </el-icon>
              <div class="conflict-body">
                <div class="conflict-title">
                  <span class="conflict-kind">{{ t(`restoreDrawer.conflictKind.${c.kind}`) }}</span>
                  <span class="conflict-target">
                    — {{ c.artifact.kind }} <code>{{ c.artifact.namespace }}/{{ c.artifact.name }}</code>
                  </span>
                </div>
                <div class="conflict-detail">{{ c.message }}</div>
              </div>
            </div>
            <div v-if="c.suggestedTransform" class="suggested-fix">
              <div class="suggested-fix-label">{{ t('restoreDrawer.suggestedFix') }}</div>
              <pre class="suggested-fix-code">{{ formatTransform(c.suggestedTransform) }}</pre>
              <div class="suggested-fix-action">
                <!-- v0.8.2: Apply Suggested Fix is now functional. Clicking
                     creates an ad-hoc Transform Set ConfigMap targeted at
                     THIS exact artifact, then auto-selects it for the
                     pending Restore via selectedTransformSet. -->
                <el-button
                  v-if="!appliedFixes[idx]"
                  size="small"
                  type="primary"
                  :loading="applyingFixIndex === idx"
                  @click.stop="applyFix(c, idx)"
                >
                  <el-icon><Operation /></el-icon> {{ t('restoreDrawer.applyFix') }}
                </el-button>
                <span v-else class="fix-applied">
                  <el-icon><CircleCheckFilled /></el-icon>
                  {{ t('restoreDrawer.fixApplied', { name: appliedFixes[idx] }) }}
                </span>
              </div>
            </div>
          </div>
          <el-checkbox v-if="hasBlockers" v-model="ignoreBlockers" class="pf-ignore-toggle">
            {{ t('restoreDrawer.ignoreBlockers') }}
          </el-checkbox>
        </div>
      </section>

      <!-- ===== Spec (N) ===== -->
      <section class="section">
        <div class="spec-header">
          <h4 class="section-title spec-title">
            {{ t('restoreDrawer.specTitle') }} ({{ artifactsList.length }})
            <el-icon class="collapse-icon" :class="{ open: specOpen }" @click="specOpen = !specOpen">
              <ArrowUp v-if="specOpen" />
              <ArrowDown v-else />
            </el-icon>
          </h4>
        </div>
        <div v-if="specOpen">
          <div class="spec-summary">
            <span>{{ t('restoreDrawer.specRestoring', { selected: includedCount, total: artifactsList.length }) }}</span>
            <span class="spec-actions">
              <button class="link-btn" type="button" @click="selectAll(true)">
                {{ t('restoreDrawer.selectAllArtifacts') }}
              </button>
              <span class="dot-sep">·</span>
              <button class="link-btn" type="button" @click="selectAll(false)">
                {{ t('restoreDrawer.deselectAllArtifacts') }}
              </button>
            </span>
          </div>

          <div v-if="artifactsLoading" class="artifacts-loading">
            <el-icon class="rotating"><Loading /></el-icon>
            {{ t('common.loading') }}
          </div>
          <div v-else-if="artifactsList.length === 0" class="artifacts-empty">
            {{ t('restoreDrawer.noArtifacts') }}
          </div>
          <div v-else class="artifacts-list">
            <div
              v-for="(a, idx) in artifactsList"
              :key="`${a.resource}/${a.namespace}/${a.name}`"
              class="artifact-card"
              :class="{ disabled: !a._included }"
            >
              <div class="artifact-row">
                <el-checkbox v-model="a._included" />
                <div class="artifact-fields">
                  <div class="field">
                    <div class="field-label">Type</div>
                    <div class="field-value">{{ a.resource }}</div>
                  </div>
                  <div class="field">
                    <div class="field-label">Version</div>
                    <div class="field-value">{{ a.version }}</div>
                  </div>
                  <div v-if="a.group" class="field">
                    <div class="field-label">Group</div>
                    <div class="field-value">{{ a.group }}</div>
                  </div>
                  <div class="field">
                    <div class="field-label">Namespace</div>
                    <div class="field-value">{{ a.namespace }}</div>
                  </div>
                  <div class="field grow">
                    <div class="field-label">Name</div>
                    <div class="field-value field-value-name">{{ a.name }}</div>
                  </div>
                </div>
                <button class="yaml-btn" type="button" :title="t('restoreDrawer.viewYaml')" @click="toggleYaml(idx)">
                  <el-icon><Document /></el-icon>
                </button>
              </div>
              <pre v-if="a._yamlOpen" class="artifact-yaml">{{ a.yaml }}</pre>
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="drawer-footer">
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!canSubmit"
          @click="handleSubmit"
        >
          <el-icon><RefreshLeft /></el-icon> {{ t('common.restore') }}
        </el-button>
        <el-button @click="visibleProxy = false">{{ t('common.cancel') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowLeft, ArrowUp, ArrowDown, CircleCheckFilled, CirclePlus,
  Document, Folder, InfoFilled, Loading, Operation,
  RefreshLeft, RefreshRight, WarningFilled
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import {
  getNamespaces, createNamespace,
  getBackupArtifacts, createRestore, preflightRestore,
  getTransformSets, applyConflictFixes
} from '../api/velero'
import { useCluster } from '../composables/useCluster'

const { t } = useI18n()
const cluster = useCluster()
// v0.9.0 MC3: targetCluster defaults to "this-cluster" → existing local-restore
// behavior. The Target Cluster section only renders when ≥2 clusters are
// registered (progressive disclosure — single-cluster users see no change).
const targetCluster = ref('this-cluster')

// selectableClusters: only Healthy ones can receive a Restore. The local
// "this-cluster" entry is always selectable. Unhealthy remotes show in
// the dropdown but are disabled — the user sees they exist but can't
// route a restore into a broken cluster.
const selectableClusters = computed(() => {
  return cluster.clusters.value.filter(c => c.phase === 'Healthy' || c.name === 'this-cluster')
})

const isCrossCluster = computed(() => {
  return targetCluster.value && targetCluster.value !== 'this-cluster'
})

const props = defineProps({
  visible: { type: Boolean, default: false },
  backup: { type: Object, default: null } // a Velero Backup row
})
const emit = defineEmits(['update:visible', 'restored'])

const visibleProxy = computed({
  get: () => props.visible,
  set: (v) => emit('update:visible', v)
})

// Source namespace is read off the backup's spec.includedNamespaces. For
// multi-ns backups we use the first one as the default target; advanced
// multi-ns restore (ns mapping table) is parked for v0.8.
const sourceNs = computed(() => {
  const ns = props.backup?.spec?.includedNamespaces || []
  return ns[0] || ''
})

const namespaces = ref([])
const targetNs = ref('')
const showCreateNs = ref(false)
const newNsName = ref('')
const creatingNs = ref(false)
const confirmOverwrite = ref(false)
const restoreName = ref('')
const existingPolicy = ref('') // "" = default skip, "update" = overwrite mutable
const specOpen = ref(true)
const artifactsList = ref([])
const artifactsLoading = ref(false)
const submitting = ref(false)

// v0.7.12 Pre-flight state. The drawer auto-runs Pre-flight on target ns
// change so the user sees blockers before clicking Restore. deepCheck adds
// the slow image-registry DNS probe (8s budget) and is off by default.
const preflight = ref(null)            // PreflightResponse
const preflightLoading = ref(false)
const preflightError = ref('')
const deepCheck = ref(false)
const ignoreBlockers = ref(false)
let preflightDebounceTimer = null

// v0.8.2 Transform Set state.
// - transformSets: catalog for the optional picker dropdown
// - selectedTransformSet: user-picked TS to apply to the Restore (or one
//   generated by the "Apply Suggested Fix" flow below)
// - appliedFixes: maps conflict index → boolean so the UI can mark a
//   conflict as "resolved by transform" once the user clicks Apply Fix.
//   Note: we don't actually re-check the cluster post-apply (the patch
//   only takes effect at restore time), so this is a UI affordance only.
const transformSets = ref([])
const selectedTransformSet = ref('')
const appliedFixes = ref({})
const applyingFixIndex = ref(-1)

const isOverwriteInPlace = computed(() => {
  // In-place = the target equals the original source. Cross-ns restores are
  // safe by definition (target namespace will be created fresh if missing).
  return !!targetNs.value && targetNs.value === sourceNs.value
})

const includedCount = computed(() => artifactsList.value.filter(a => a._included).length)

// v0.7.12 Pre-flight computeds
const conflicts = computed(() => preflight.value?.conflicts || [])
// v0.8.3 fix: a conflict that has an Apply Fix applied is no longer a
// blocker for the Restore. Without this, the "我已了解 — 忽略阻断" checkbox
// stays mandatory even after the user fixes everything.
const hasBlockers = computed(() =>
  conflicts.value.some((c, idx) => c.severity === 'blocker' && !appliedFixes.value[idx])
)
const preflightVisible = computed(() => !!targetNs.value && !!props.backup)

const pfSummaryText = computed(() => {
  if (!preflight.value) return ''
  const n = conflicts.value.length
  if (n === 0) return t('restoreDrawer.preflightZeroConflicts')
  const blockers = conflicts.value.filter(c => c.severity === 'blocker').length
  const warnings = n - blockers
  if (blockers && warnings) return t('restoreDrawer.preflightMixed', { blockers, warnings })
  if (blockers) return t('restoreDrawer.preflightBlockerCount', { n: blockers })
  return t('restoreDrawer.preflightWarningCount', { n: warnings })
})
const pfSeverityClass = computed(() => {
  if (!preflight.value || conflicts.value.length === 0) return 'pf-summary-ok'
  return hasBlockers.value ? 'pf-summary-blocker' : 'pf-summary-warning'
})

const canSubmit = computed(() => {
  if (!props.backup || !targetNs.value || !restoreName.value) return false
  if (isOverwriteInPlace.value && !confirmOverwrite.value) return false
  if (includedCount.value === 0) return false
  // v0.7.12: Pre-flight blockers gate the Restore button. The user can
  // override with the "Ignore conflicts and continue" checkbox — the
  // Restore will likely fail but we don't paternalistically lock them out.
  if (hasBlockers.value && !ignoreBlockers.value) return false
  if (preflightLoading.value) return false
  return !submitting.value
})

// Format a TransformRule as a copy-pasteable preview. v0.8.4 supports
// three patch variants — only one is populated per Conflict:
//   - JSON Patch  → "operation / path / value" lines
//   - Merge Patch → "mergePatch: {JSON}"  (pretty-printed)
//   - Strategic   → "strategicPatch: {JSON}"
const formatTransform = (rule) => {
  if (!rule) return ''
  if (rule.mergePatch) {
    return 'mergePatch:\n' + prettyJSON(rule.mergePatch)
  }
  if (rule.strategicPatch) {
    return 'strategicPatch:\n' + prettyJSON(rule.strategicPatch)
  }
  if (rule.operation && rule.path) {
    const parts = [`operation: ${rule.operation}`, `path: ${rule.path}`]
    if (rule.value !== undefined && rule.value !== null) {
      parts.push(`value: ${typeof rule.value === 'string' ? rule.value : JSON.stringify(rule.value)}`)
    }
    return parts.join('\n')
  }
  return '(no patch payload)'
}

// prettyJSON safely pretty-prints a JSON string. Falls back to the raw
// string if it isn't valid JSON.
function prettyJSON(s) {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

async function onOpened() {
  // Default restoreName: `<backup>-restore-<HHMMSS>` so multiple restores from
  // the same backup don't collide.
  const ts = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  restoreName.value = `${props.backup.metadata.name}-restore-${pad(ts.getHours())}${pad(ts.getMinutes())}${pad(ts.getSeconds())}`
  // Default target = source ns; user can change.
  targetNs.value = sourceNs.value
  // v0.9.0 MC3: refresh cluster registry so the Target Cluster selector
  // reflects the latest list + health phases (60s controller may have
  // updated a phase since the user last opened the drawer).
  targetCluster.value = 'this-cluster'
  cluster.refresh()
  confirmOverwrite.value = false
  showCreateNs.value = false
  newNsName.value = ''
  existingPolicy.value = ''
  specOpen.value = true
  preflight.value = null
  preflightError.value = ''
  ignoreBlockers.value = false
  selectedTransformSet.value = ''
  appliedFixes.value = {}

  await Promise.all([loadNamespaces(), loadArtifacts(), loadTransformSets()])
  // Kick off Pre-flight once we have an opening state.
  schedulePreflight()
}

function onClosed() {
  // Clear potentially heavy YAML strings from memory.
  artifactsList.value = []
}

async function loadNamespaces() {
  try {
    const res = await getNamespaces()
    const items = res.data.items || res.data.namespaces || []
    namespaces.value = items.map(ns => ns.metadata?.name || ns).filter(Boolean).sort()
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

// v0.8.2: pull the Transform Sets catalog for the picker.
async function loadTransformSets() {
  try {
    const res = await getTransformSets()
    transformSets.value = res.data.items || []
  } catch (e) {
    console.error('Failed to load transform sets:', e)
    transformSets.value = []
  }
}

const selectedTransformSetObj = computed(() => {
  if (!selectedTransformSet.value) return null
  return transformSets.value.find(t => t.name === selectedTransformSet.value)
})

function goManageTransformSets() {
  // Take the user to the TransformSets management page in a new browser
  // tab so they don't lose their in-progress Restore state.
  window.open('/transform-sets', '_blank')
}

// v0.8.3: Batched "Apply Suggested Fix" handler.
//
// Why batched: Velero's Restore.spec.resourceModifierRef points to exactly
// ONE ConfigMap. If we created a separate Transform Set per conflict
// (v0.8.2 behavior), only the last-clicked fix would actually run at
// restore time — the others would be lost. So every click sends the
// *full current list* of applied fixes; the backend writes one
// consolidated Transform Set ConfigMap and we attach that to the Restore.
//
// The TS name is derived from restoreName so re-opens of the same drawer
// hit the same CM (which we delete on cancel anyway since the Restore
// itself never gets created).
async function applyFix(conflict, idx) {
  applyingFixIndex.value = idx
  try {
    // Optimistically add this conflict to the "applied" set so the batch
    // we send below includes it.
    const tentativeFixes = {
      ...appliedFixes.value,
      [idx]: '__pending__'
    }
    // Build the batch from every currently-applied fix (including new one).
    const fixesBatch = []
    for (const [i, _name] of Object.entries(tentativeFixes)) {
      const c = conflicts.value[Number(i)]
      if (!c || !c.suggestedTransform) continue
      fixesBatch.push({
        conflictKind: c.kind,
        artifact: c.artifact,
        suggestedTransform: c.suggestedTransform
      })
    }
    const res = await applyConflictFixes({
      restoreName: restoreName.value,
      fixes: fixesBatch
    })
    const name = res.data.transformSetName
    // Update local state with the actual TS name. Every fix entry shares
    // the same consolidated TS name — that's the design.
    const updated = {}
    for (const i of Object.keys(tentativeFixes)) updated[i] = name
    appliedFixes.value = updated
    selectedTransformSet.value = name
    await loadTransformSets()
    ElMessage.success(t('restoreDrawer.fixCreatedToast', { name }))
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  } finally {
    applyingFixIndex.value = -1
  }
}

async function loadArtifacts() {
  if (!props.backup?.metadata?.name) return
  artifactsLoading.value = true
  try {
    const res = await getBackupArtifacts(props.backup.metadata.name)
    const items = res.data.items || []
    // Augment each item with UI-local state.
    artifactsList.value = items.map(a => ({ ...a, _included: true, _yamlOpen: false }))
  } catch (e) {
    console.error('Failed to load artifacts:', e)
    ElMessage.error(t('restoreDrawer.artifactsLoadError'))
    artifactsList.value = []
  } finally {
    artifactsLoading.value = false
  }
}

function selectAll(value) {
  for (const a of artifactsList.value) a._included = value
}

function toggleYaml(idx) {
  artifactsList.value[idx]._yamlOpen = !artifactsList.value[idx]._yamlOpen
}

function onTargetChange() {
  // Switching away from source clears the overwrite confirm so it doesn't
  // sneak back if the user toggles between ns.
  confirmOverwrite.value = false
  // Re-run Pre-flight because the target ns changed → different cleanup
  // semantics, different conflict set.
  schedulePreflight()
}

// Debounced Pre-flight runner. Watch hooks call this without worrying
// about race conditions or excessive API hits during rapid UI changes.
function schedulePreflight() {
  if (preflightDebounceTimer) clearTimeout(preflightDebounceTimer)
  preflightDebounceTimer = setTimeout(() => runPreflight(false), 300)
}

async function runPreflight(force) {
  if (!props.backup?.metadata?.name || !targetNs.value) return
  // Without `force` we skip re-runs that produce the same answer.
  if (preflightLoading.value && !force) return
  preflightLoading.value = true
  preflightError.value = ''
  try {
    const cleanup = isOverwriteInPlace.value && confirmOverwrite.value
    const res = await preflightRestore({
      backupName: props.backup.metadata.name,
      targetNamespace: targetNs.value,
      cleanupBeforeRestore: cleanup,
      deepCheck: deepCheck.value
    })
    preflight.value = res.data
    // Reset the override checkbox when results change so the user
    // re-affirms after every re-check.
    if (!hasBlockers.value) ignoreBlockers.value = false
  } catch (e) {
    preflightError.value = e.response?.data?.error || e.message
    preflight.value = null
  } finally {
    preflightLoading.value = false
  }
}

// Re-run when confirmOverwrite flips (cleanup semantics affect conflicts).
watch(confirmOverwrite, () => schedulePreflight())

async function handleCreateNs() {
  const name = (newNsName.value || '').trim()
  if (!name) {
    ElMessage.warning(t('restoreDrawer.newNsRequired'))
    return
  }
  creatingNs.value = true
  try {
    await createNamespace(name)
    ElMessage.success(t('restoreDrawer.newNsCreated', { name }))
    // Optimistic: append to list + select + close inline form.
    if (!namespaces.value.includes(name)) namespaces.value.push(name)
    namespaces.value.sort()
    targetNs.value = name
    showCreateNs.value = false
    newNsName.value = ''
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  } finally {
    creatingNs.value = false
  }
}

async function handleSubmit() {
  if (!canSubmit.value) return
  submitting.value = true
  try {
    // Compose includedResources from artifacts selection. Velero filters by
    // *resource name* (e.g. "deployments"). When all artifacts are selected
    // we omit the filter so Velero restores everything in the namespace.
    const allSelected = artifactsList.value.every(a => a._included)
    const includedResources = allSelected
      ? undefined
      : [...new Set(artifactsList.value.filter(a => a._included).map(a => a.resource))]

    const cleanupBeforeRestore = isOverwriteInPlace.value // confirmed via checkbox above

    const payload = {
      name: restoreName.value,
      backupName: props.backup.metadata.name,
      includedNamespaces: [sourceNs.value],
      // Only emit a mapping when target differs from source. Velero's
      // NamespaceMapping is "source -> target".
      namespaceMapping: targetNs.value !== sourceNs.value
        ? { [sourceNs.value]: targetNs.value }
        : undefined,
      cleanupBeforeRestore,
      includedResources,
      existingResourcePolicy: cleanupBeforeRestore ? '' : existingPolicy.value,
      // v0.8.2: Transform Set ref — Velero applies the JSONPath patches
      // during restore. Either picked by the user or auto-assigned by
      // the Pre-flight Apply Fix flow.
      transformSetName: selectedTransformSet.value || undefined,
      // v0.9.0 MC3: cross-cluster target. Omitted when restoring to the
      // local cluster (preserves existing v0.8 wire format for compat).
      targetCluster: isCrossCluster.value ? targetCluster.value : undefined
    }
    await createRestore(payload)
    ElMessage.success(t('restoreDrawer.submitted', { name: restoreName.value }))
    visibleProxy.value = false
    emit('restored', restoreName.value)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  } finally {
    submitting.value = false
  }
}

watch(() => props.visible, (v) => {
  if (!v) onClosed()
})
</script>

<style scoped>
.restore-drawer {
  padding: 0 4px 0 4px;
  font-size: 13px;
  color: var(--sk-text-secondary);
}
.empty {
  padding: 20px;
  color: var(--sk-text-caption);
  text-align: center;
}
.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: none;
  border: 0;
  color: var(--sk-primary);
  font-size: 13px;
  cursor: pointer;
  padding: 0 0 12px 0;
}
.back-link:hover { text-decoration: underline; }

.section {
  border-top: 1px solid #ebeef5;
  padding: 18px 4px;
}
.section:first-of-type { border-top: 0; padding-top: 4px; }
.section-title {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--sk-text);
  display: flex;
  align-items: center;
  gap: 8px;
}
.section-desc {
  margin: 0 0 12px 0;
  color: var(--sk-text-muted);
  font-size: 13px;
  line-height: 1.55;
}

/* Namespace picker */
/* v0.9.0 MC3: cross-cluster restore banner + cluster chip in selector */
.cross-cluster-banner {
  margin-top: 10px;
  padding: 10px 14px;
  background: #fffbeb;
  border-left: 3px solid #f59e0b;
  border-radius: 0 6px 6px 0;
  color: #92400e;
  font-size: 13px;
  line-height: 1.5;
}
.cluster-primary-badge {
  color: var(--sk-primary, #4f46e5);
  font-size: 11px;
  margin-left: 4px;
}

.ns-row {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.ns-select { width: 100%; }
.ns-icon { color: var(--sk-text-caption); }
.ns-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.ns-source-mark { color: var(--sk-status-success); margin-left: auto; }
.create-ns-link {
  align-self: flex-end;
  background: none;
  border: 0;
  color: var(--sk-primary);
  font-size: 13px;
  cursor: pointer;
  padding: 4px 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.create-ns-link:hover { text-decoration: underline; }

.new-ns-row {
  margin-top: 8px;
  padding: 10px 12px;
  background: #f5f7fa;
  border-radius: 6px;
}
.new-ns-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--sk-text-muted);
  margin-bottom: 6px;
}
.new-ns-controls {
  display: flex;
  gap: 8px;
}
.new-ns-input { flex: 1; }

/* Overwrite warning */
.overwrite-warning {
  margin-top: 12px;
  padding: 12px 14px;
  background: #fdf6ec;
  border: 1px solid #f5dab1;
  border-radius: 6px;
  display: flex;
  gap: 10px;
}
.warn-icon {
  color: var(--sk-status-warning);
  font-size: 20px;
  margin-top: 1px;
  flex-shrink: 0;
}
.warn-body { flex: 1; }
.warn-title {
  font-weight: 600;
  font-size: 13px;
  color: #b88230;
  margin-bottom: 4px;
}
.warn-desc {
  font-size: 13px;
  color: #67430b;
  line-height: 1.5;
  margin-bottom: 8px;
}

/* Optional settings */
.opt-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
}
.opt-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--sk-text-muted);
}
/* Radio group laid out as 2 stacked, left-aligned rows.
 *
 * Alignment fix (v0.8.3): the previous CSS relied on el-radio's default
 * __label padding-left: 8px to create the gap between the circle and the
 * text. But the circle's intrinsic width varies subtly between
 * checked/unchecked states in Element Plus (box-shadow + inner-dot
 * pseudo-element), which translated into a ~4px horizontal offset between
 * the two radio rows.
 *
 * Now we pin the __input to exactly 14×14, push the gap to the input's
 * margin-right (not the label's padding-left), and override both checked
 * and unchecked states identically. Result: both rows have IDENTICAL
 * layout regardless of selection.
 */
.policy-radios {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  width: 100%;
  margin: 0;
  padding: 0;
}
.policy-radios :deep(.el-radio.policy-radio),
.policy-radios :deep(.el-radio.policy-radio.is-checked) {
  display: flex;
  align-items: flex-start;
  width: 100%;
  height: auto;
  margin: 0;
  padding: 0;
  white-space: normal;
}
.policy-radios :deep(.el-radio.policy-radio .el-radio__input),
.policy-radios :deep(.el-radio.policy-radio.is-checked .el-radio__input) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  margin: 3px 8px 0 0;
  padding: 0;
  flex-shrink: 0;
  box-sizing: border-box;
}
.policy-radios :deep(.el-radio.policy-radio .el-radio__inner) {
  width: 14px;
  height: 14px;
  margin: 0;
  flex-shrink: 0;
  box-sizing: border-box;
}
.policy-radios :deep(.el-radio.policy-radio .el-radio__label) {
  padding: 0;
  margin: 0;
  font-weight: normal;
  line-height: 1.4;
  flex: 1;
  min-width: 0;
}
.policy-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.policy-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--sk-text-secondary);
}
.policy-hint {
  font-size: 12px;
  color: var(--sk-text-caption);
  font-weight: normal;
  line-height: 1.45;
}

/* Spec */
.spec-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.spec-title { margin: 0; }
.collapse-icon {
  cursor: pointer;
  color: var(--sk-text-caption);
  font-size: 14px;
  transition: transform 0.15s;
}
.spec-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
  font-size: 13px;
  color: var(--sk-text-muted);
  margin-bottom: 10px;
}
.spec-actions { display: inline-flex; gap: 8px; align-items: center; }
.link-btn {
  background: none;
  border: 0;
  color: var(--sk-primary);
  cursor: pointer;
  font-size: 13px;
  padding: 0;
}
.link-btn:hover { text-decoration: underline; }
.dot-sep { color: var(--sk-text-placeholder); }

.artifacts-loading,
.artifacts-empty {
  padding: 24px;
  text-align: center;
  color: var(--sk-text-caption);
  font-size: 13px;
}
.rotating {
  animation: spin 1s linear infinite;
  margin-right: 6px;
}
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }

.artifacts-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.artifact-card {
  border: 1px solid #5b6cdb;
  border-radius: 8px;
  padding: 12px 14px;
  background: #ffffff;
  transition: opacity 0.15s;
}
.artifact-card.disabled {
  opacity: 0.45;
  border-color: #dcdfe6;
}
.artifact-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.artifact-fields {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  align-items: flex-end;
}
.field { display: flex; flex-direction: column; gap: 2px; }
.field.grow { flex: 1; min-width: 140px; }
.field-label { font-size: 11px; color: var(--sk-text-caption); font-weight: 500; }
.field-value { font-size: 13px; color: var(--sk-text-secondary); font-weight: 600; }
.field-value-name {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  word-break: break-all;
}
.yaml-btn {
  background: #ffffff;
  border: 1px solid #dcdfe6;
  border-radius: 50%;
  width: 32px;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--sk-text-muted);
  cursor: pointer;
  flex-shrink: 0;
}
.yaml-btn:hover { color: var(--sk-primary); border-color: var(--sk-primary); }
.artifact-yaml {
  margin: 12px 0 0 0;
  background: #1d1e26;
  color: #e5eaf3;
  padding: 12px 14px;
  border-radius: 6px;
  font-size: 11.5px;
  line-height: 1.55;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  white-space: pre;
  overflow-x: auto;
  max-height: 360px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-start;
  gap: 8px;
}

/* ===== v0.7.12 Pre-flight Check section ================================ */
.pf-summary {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: 4px;
}
.pf-summary-ok { background: #f0f9eb; color: var(--sk-status-success); }
.pf-summary-warning { background: #fdf6ec; color: #b88230; }
.pf-summary-blocker { background: #fef0f0; color: var(--sk-status-error); }
.pf-deep-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: -2px 0 12px 0;
}
.pf-deep-hint {
  font-size: 11px;
  color: var(--sk-text-caption);
}
.pf-error-card {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: #fef0f0;
  border: 1px solid #fbc4c4;
  border-radius: 6px;
  color: var(--sk-status-error);
  font-size: 13px;
}
.pf-clean {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  background: #f0f9eb;
  border: 1px solid #c2e7b0;
  border-radius: 6px;
}
.pf-clean-icon { color: var(--sk-status-success); font-size: 22px; }
.pf-clean-title { font-weight: 600; color: #2d6a14; font-size: 13px; }
.pf-clean-desc { font-size: 12px; color: #5a7a4a; margin-top: 2px; }

.conflicts-list { display: flex; flex-direction: column; gap: 10px; }
.conflict-card {
  border: 1px solid;
  border-radius: 8px;
  padding: 12px 14px;
  background: #fff;
}
.conflict-card.severity-blocker { border-color: #fbc4c4; background: #fef7f7; }
.conflict-card.severity-warning { border-color: #f5dab1; background: #fdf6ec; }
.conflict-card.severity-info { border-color: #d3d8e0; background: #f5f7fa; }

.conflict-header { display: flex; align-items: flex-start; gap: 10px; }
.conflict-icon { font-size: 18px; margin-top: 1px; flex-shrink: 0; }
.severity-blocker .conflict-icon { color: var(--sk-status-error); }
.severity-warning .conflict-icon { color: var(--sk-status-warning); }
.severity-info .conflict-icon { color: var(--sk-text-caption); }
.conflict-body { flex: 1; min-width: 0; }
.conflict-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--sk-text);
}
.conflict-kind { font-weight: 600; }
.conflict-target {
  font-weight: normal;
  color: var(--sk-text-muted);
}
.conflict-target code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  background: rgba(0,0,0,0.05);
  padding: 1px 4px;
  border-radius: 3px;
}
.conflict-detail {
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--sk-text-muted);
}

.suggested-fix {
  margin-top: 10px;
  padding: 10px 12px;
  background: #1d1e26;
  border-radius: 6px;
}
.suggested-fix-label {
  font-size: 11px;
  font-weight: 600;
  color: #b1b3b8;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  margin-bottom: 4px;
}
.suggested-fix-code {
  margin: 0;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  color: #e5eaf3;
  line-height: 1.55;
  white-space: pre-wrap;
}
.suggested-fix-action {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.coming-soon-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  background: var(--sk-primary);
  color: #fff;
  letter-spacing: 0.3px;
}
.fix-applied {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--sk-status-success);
  font-size: 12px;
  font-weight: 500;
}
.fix-applied .el-icon { font-size: 16px; }
.opt-help-link {
  margin-left: 8px;
  font-size: 11px;
  font-weight: normal;
  color: var(--sk-primary);
  text-decoration: none;
}
.opt-help-link:hover { text-decoration: underline; }
.opt-help {
  font-size: 11.5px;
  color: var(--sk-text-caption);
  margin-top: 4px;
  line-height: 1.45;
}
.ts-builtin-badge {
  margin-left: 8px;
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: #ecf5ff;
  color: var(--sk-primary);
  font-weight: 500;
}
.pf-ignore-toggle {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #ebeef5;
}
:global(html.dark) .conflict-card.severity-blocker { background: #2a1a1d; border-color: #6e2a30; }
:global(html.dark) .conflict-card.severity-warning { background: #2b1f0c; border-color: #6e5217; }
:global(html.dark) .conflict-card.severity-info { background: #1f2026; border-color: #3a3d44; }
:global(html.dark) .conflict-title { color: #e5eaf3; }
:global(html.dark) .conflict-target { color: #b1b3b8; }
:global(html.dark) .conflict-detail { color: #b1b3b8; }
:global(html.dark) .pf-clean { background: #1a2c14; border-color: #2a5a1f; }
:global(html.dark) .pf-clean-title { color: #c2e7b0; }
:global(html.dark) .pf-clean-desc { color: #95c282; }

/* Dark mode overrides — drawer is rendered into body, .dark scopes via root */
:global(html.dark) .restore-drawer { color: #e5eaf3; }
:global(html.dark) .section { border-top-color: #2c2f36; }
:global(html.dark) .section-title { color: #e5eaf3; }
:global(html.dark) .section-desc,
:global(html.dark) .opt-label,
:global(html.dark) .policy-hint,
:global(html.dark) .field-label { color: var(--sk-text-caption); }
:global(html.dark) .new-ns-row { background: #1f2026; }
:global(html.dark) .spec-summary { background: #1f2026; color: #b1b3b8; }
:global(html.dark) .artifact-card { background: #1f2026; border-color: #4754b0; }
:global(html.dark) .artifact-card.disabled { border-color: #3a3d44; }
:global(html.dark) .field-value, :global(html.dark) .policy-name { color: #e5eaf3; }
:global(html.dark) .yaml-btn { background: #1f2026; border-color: #3a3d44; color: #b1b3b8; }
:global(html.dark) .overwrite-warning { background: #2b1f0c; border-color: #6e5217; }
:global(html.dark) .warn-title { color: #f0c473; }
:global(html.dark) .warn-desc { color: #d6b87d; }
</style>
