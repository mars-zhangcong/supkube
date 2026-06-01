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

      <!-- v0.9.1.5: Imported chip — surfaces "this RP came from another
           cluster via BSL sync" right at the top of the drawer. Without
           this customers couldn't visually distinguish local vs imported
           RPs, which was a real demo gap (Mars 2026-05-26). -->
      <div v-if="isImported" class="imported-banner">
        <el-tag type="warning" effect="plain" size="small">
          <el-icon><Operation /></el-icon> {{ t('restoreDrawer.importedBadge') }}
        </el-tag>
        <span class="imported-source">{{ t('restoreDrawer.importedFrom', { cluster: importedSourceCluster }) }}</span>
      </div>

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
          <!--
            PRD-001 v2 (#104, finding #1 — 2026-05-31) Checklist form:
              - blocker  → red ✗  + no "ignore" checkbox (must resolve)
              - warning  → yellow ⚠ + "ignore this warning" checkbox
                           (second-step confirm before flipping on)
              - applied  → green ✓  (Apply Fix landed)
              - ignored  → muted   (user explicitly waived this warning)
            Each row gets a "→ Go to Transform" CTA that hops to the
            TransformSets editor pre-loaded with this conflict's payload
            (PRD-002 Transforms.vue placeholder for now).
          -->
          <div
            v-for="(c, idx) in conflicts"
            :key="idx"
            class="conflict-card"
            :class="[
              `severity-${effectiveSeverity(c)}`,
              appliedFixes[idx] ? 'is-resolved' : '',
              ignoredWarnings[idx] ? 'is-ignored' : ''
            ]"
          >
            <div class="conflict-header">
              <el-icon class="conflict-icon">
                <CircleCheckFilled v-if="appliedFixes[idx]" />
                <CircleCloseFilled v-else-if="effectiveSeverity(c) === 'blocker'" />
                <WarningFilled v-else />
              </el-icon>
              <div class="conflict-body">
                <div class="conflict-title">
                  <el-tag
                    size="small"
                    :type="effectiveSeverity(c) === 'blocker' ? 'danger' : 'warning'"
                    effect="plain"
                    class="severity-tag"
                  >
                    {{ effectiveSeverity(c) === 'blocker'
                        ? t('restoreDrawer.severityBlocker')
                        : t('restoreDrawer.severityWarning') }}
                  </el-tag>
                  <span class="conflict-kind">{{ t(`restoreDrawer.conflictKind.${c.kind}`) }}</span>
                  <span class="conflict-target">
                    — {{ c.artifact.kind }} <code>{{ c.artifact.namespace }}/{{ c.artifact.name }}</code>
                  </span>
                </div>
                <div class="conflict-detail">{{ c.message }}</div>
                <!-- PRD-001 v2 (#104): TransformSets that the backend
                     matched against this conflict kind+payload. Empty
                     slice = no matching TS; the Go-to-Transform button
                     then opens the editor in "author new" mode. -->
                <div v-if="c.matchingTransformSets && c.matchingTransformSets.length > 0" class="matching-ts">
                  <span class="matching-ts-label">{{ t('restoreDrawer.matchingTransformSets') }}:</span>
                  <el-tag
                    v-for="ts in c.matchingTransformSets"
                    :key="ts"
                    size="small"
                    type="info"
                    effect="plain"
                    class="matching-ts-tag"
                  >{{ ts }}</el-tag>
                </div>
              </div>
            </div>

            <!-- Row-level actions: ignore (warnings only), apply fix,
                 go to transform. -->
            <div class="conflict-actions">
              <!-- Ignore warning checkbox — warning only, never blocker.
                   :disabled when blocker so even DOM-tampering can't
                   silently bypass (defense in depth; final gate is on
                   the computed `canSubmit`). -->
              <el-checkbox
                v-if="effectiveSeverity(c) === 'warning' && !appliedFixes[idx]"
                :model-value="!!ignoredWarnings[idx]"
                @change="toggleIgnoreWarning(idx, c)"
              >
                {{ t('restoreDrawer.ignoreThisWarning') }}
              </el-checkbox>
              <el-checkbox
                v-else-if="effectiveSeverity(c) === 'blocker'"
                :model-value="false"
                disabled
              >
                <span class="ignore-disabled-hint">{{ t('restoreDrawer.ignoreBlockerDisabled') }}</span>
              </el-checkbox>

              <span class="conflict-actions-spacer"></span>

              <!-- "→ Go to Transform" — required for blockers, optional
                   for warnings (always show — the editor is the deep
                   path). Stub: PRD-002 §4.4 builds the Transforms page;
                   today it lands on a placeholder. -->
              <el-button
                size="small"
                type="primary"
                plain
                @click.stop="goToTransform(c)"
              >
                {{ t('restoreDrawer.goToTransform') }}
                <el-icon class="el-icon--right"><ArrowRight /></el-icon>
              </el-button>
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

          <!-- PRD-001 v2 (#104): no global "ignore all blockers"
               checkbox. Replaced by a status footer that surfaces the
               blocker count + ignored-warning count so the user sees
               exactly what's gating Restore. -->
          <div class="pf-status-footer">
            <span v-if="hasBlockers" class="pf-status-blocker">
              <el-icon><CircleCloseFilled /></el-icon>
              {{ t('restoreDrawer.mustResolveBlocker', { n: unresolvedBlockers.length }) }}
            </span>
            <span v-else-if="unacknowledgedWarnings.length > 0" class="pf-status-warning">
              <el-icon><WarningFilled /></el-icon>
              {{ t('restoreDrawer.mustAcknowledgeWarning', { n: unacknowledgedWarnings.length }) }}
            </span>
            <span v-else-if="ignoredWarningCount > 0" class="pf-status-ok">
              <el-icon><CircleCheckFilled /></el-icon>
              {{ t('restoreDrawer.allConflictsHandled', { n: ignoredWarningCount }) }}
            </span>
            <span v-else class="pf-status-ok">
              <el-icon><CircleCheckFilled /></el-icon>
              {{ t('restoreDrawer.allConflictsResolved') }}
            </span>
          </div>
        </div>
      </section>

      <!-- ===== Spec (N) ===== -->
      <section class="section">
        <div class="spec-header">
          <h4 class="section-title spec-title">
            <!-- v0.9.x #91: Imported + 0 artifacts → show "Whole tarball"
                 instead of "(0)" which previously made customers think the
                 wizard was broken. Banner below explains the semantics. -->
            {{ t('restoreDrawer.specTitle') }}
            <span v-if="isImported && artifactsList.length === 0">({{ t('restoreDrawer.specWholeTarball') || 'Whole tarball' }})</span>
            <span v-else>({{ artifactsList.length }})</span>
            <el-icon class="collapse-icon" :class="{ open: specOpen }" @click="specOpen = !specOpen">
              <ArrowUp v-if="specOpen" />
              <ArrowDown v-else />
            </el-icon>
          </h4>
        </div>
        <div v-if="specOpen">
          <!-- v0.9.x #91: hide the per-artifact selector controls when we
               can't enumerate artifacts (Imported RPs). The banner that
               follows handles the explanation; showing "Restoring 0 of 0"
               + Select All / Deselect All would just confuse the user. -->
          <div v-if="!(isImported && artifactsList.length === 0)" class="spec-summary">
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
          <!-- v0.9.1.5: imported-RP empty state isn't an "error" — the
               backend's artifact lister currently can't enumerate from
               BSL tarball (see task #91); customer still gets full
               whole-tarball restore from Velero. Show this clearly. -->
          <div v-else-if="artifactsList.length === 0 && isImported" class="imported-asis-banner">
            <el-icon class="imported-asis-icon"><InfoFilled /></el-icon>
            <div>
              <strong>{{ t('restoreDrawer.importedAsIsTitle') }}</strong>
              <p>{{ t('restoreDrawer.importedAsIsBody') }}</p>
            </div>
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
        <!-- v0.9.1.5: tooltip on disabled state. Without this customers
             see a grey button and can't tell which field is blocking
             them. The tooltip surfaces the specific blocker computed
             above. -->
        <el-tooltip
          :content="cantSubmitReason"
          placement="top"
          :disabled="!cantSubmitReason"
        >
          <span>
            <el-button
              type="primary"
              :loading="submitting"
              :disabled="!canSubmit"
              @click="handleSubmit"
            >
              <el-icon><RefreshLeft /></el-icon> {{ t('common.restore') }}
            </el-button>
          </span>
        </el-tooltip>
        <el-button @click="visibleProxy = false">{{ t('common.cancel') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  ArrowLeft, ArrowRight, ArrowUp, ArrowDown,
  CircleCheckFilled, CircleCloseFilled, CirclePlus,
  Document, Folder, InfoFilled, Loading, Operation,
  RefreshLeft, RefreshRight, WarningFilled
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import {
  getNamespaces, createNamespace,
  getBackupArtifacts, createRestore, preflightRestore,
  getTransformSets, applyConflictFixes
} from '../api/velero'
import { useCluster } from '../composables/useCluster'

const { t } = useI18n()
const router = useRouter()
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
// PRD-001 v2 (#104, finding #1 — 2026-05-31): the old global
// `ignoreBlockers` checkbox is gone. Blockers can NEVER be ignored; only
// individual `warning`-severity conflicts can be checked off by the user,
// each requiring a second-step confirm. Maps conflict-index → true once
// the user has confirmed the ignore for that specific warning.
const ignoredWarnings = ref({})
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
// PRD-001 v2 (#104): treat severity as authoritative-from-backend and
// fall back to `blocker` for any conflict that lacks the field (old
// backends or future kinds the server forgot to tag). This is the SAME
// conservative default the backend applies — duplicated client-side so
// upgrades are safe in either direction. The frontend MUST NOT compute
// severity from `kind` or other fields (finding #2: no client-side
// severity calculation, no client-side override).
const effectiveSeverity = (c) => (c?.severity === 'warning' ? 'warning' : 'blocker')

// v0.8.3 fix preserved: a conflict that has an Apply Fix applied is no
// longer a blocker for the Restore (the fix patch will run at restore
// time). PRD-001 v2 (#104, finding #1): blockers are NEVER ignorable via
// a checkbox — the only outlet is Apply Fix or Go-to-Transform.
const unresolvedBlockers = computed(() =>
  conflicts.value.filter((c, idx) => effectiveSeverity(c) === 'blocker' && !appliedFixes.value[idx])
)
const hasBlockers = computed(() => unresolvedBlockers.value.length > 0)

// Warnings the user hasn't yet acknowledged (each requires its own
// per-row checkbox + confirm).
const unacknowledgedWarnings = computed(() =>
  conflicts.value.filter((c, idx) => effectiveSeverity(c) === 'warning' && !appliedFixes.value[idx] && !ignoredWarnings.value[idx])
)
const ignoredWarningCount = computed(() =>
  conflicts.value.filter((c, idx) => effectiveSeverity(c) === 'warning' && ignoredWarnings.value[idx]).length
)

const preflightVisible = computed(() => !!targetNs.value && !!props.backup)

const pfSummaryText = computed(() => {
  if (!preflight.value) return ''
  const n = conflicts.value.length
  if (n === 0) return t('restoreDrawer.preflightZeroConflicts')
  const blockers = conflicts.value.filter(c => effectiveSeverity(c) === 'blocker').length
  const warnings = n - blockers
  if (blockers && warnings) return t('restoreDrawer.preflightMixed', { blockers, warnings })
  if (blockers) return t('restoreDrawer.preflightBlockerCount', { n: blockers })
  return t('restoreDrawer.preflightWarningCount', { n: warnings })
})
const pfSeverityClass = computed(() => {
  if (!preflight.value || conflicts.value.length === 0) return 'pf-summary-ok'
  return hasBlockers.value ? 'pf-summary-blocker' : 'pf-summary-warning'
})

// v0.9.1.5: detect imported RP — now AUTHORITATIVE from the backend.
//
// The backend (SupKubeBackupMeta.Imported) decides this by checking whether
// the Backup's velero.io/schedule-name refers to a Schedule that exists in
// THIS cluster. If not, the owning cluster has it and we don't → this RP was
// synced in from a shared BSL. We trust that flag here.
//
// History (C-001): an earlier version of this computed checked a
// velero.io/source-cluster label that Velero NEVER writes — so the chip
// never lit. Velero only stamps velero.io/source-cluster-k8s-gitversion
// (the K8s version, not a cluster name). The reliable signal lives
// server-side (schedule existence), so detection moved there.
//
// For imported RPs the only viable restore mode is "whole tarball as-is":
// our ListBackupArtifacts endpoint lists LIVE objects in the source ns to
// enumerate contents, which returns empty when the source ns doesn't exist
// locally. Velero restores everything in the backup regardless of our
// client-side artifact selection.
const isImported = computed(() => props.backup?.supkube?.imported === true)

// Velero doesn't record the source cluster NAME, only its K8s version. We
// surface that as a hint ("from a v1.32 cluster") rather than claim a name
// we don't have.
const importedSourceCluster = computed(() => {
  const anns = props.backup?.metadata?.annotations || {}
  const ver  = anns['velero.io/source-cluster-k8s-gitversion']
  return ver ? `Kubernetes ${ver}` : t('restoreDrawer.importedUnknownSource')
})

// Reason the Restore button is disabled — shown as tooltip on hover so
// customers can self-diagnose instead of staring at a grey button.
const cantSubmitReason = computed(() => {
  if (!props.backup)                         return t('restoreDrawer.disabled.noBackup')
  if (!targetNs.value)                       return t('restoreDrawer.disabled.noTargetNs')
  if (!restoreName.value)                    return t('restoreDrawer.disabled.noRestoreName')
  if (isOverwriteInPlace.value && !confirmOverwrite.value)
                                             return t('restoreDrawer.disabled.confirmOverwrite')
  // Imported RPs: client-side artifact selection unavailable, but
  // whole-tarball restore is fine.
  if (includedCount.value === 0 && !isImported.value && artifactsList.value.length > 0)
                                             return t('restoreDrawer.disabled.noArtifactsSelected')
  // PRD-001 v2 (#104): blockers are non-ignorable. Surface the count so
  // the user knows exactly how many rows they still need to resolve
  // (Apply Fix or Go to Transform — no override checkbox).
  if (hasBlockers.value)
                                             return t('restoreDrawer.disabled.mustResolveBlocker', { n: unresolvedBlockers.value.length })
  // Warnings exist that the user hasn't explicitly ignored yet — they
  // must tick the per-row "ignore this warning" checkbox.
  if (unacknowledgedWarnings.value.length > 0)
                                             return t('restoreDrawer.disabled.mustAcknowledgeWarning', { n: unacknowledgedWarnings.value.length })
  if (preflightLoading.value)                return t('restoreDrawer.disabled.preflightRunning')
  if (submitting.value)                      return t('restoreDrawer.disabled.submitting')
  return ''
})

const canSubmit = computed(() => {
  if (!props.backup || !targetNs.value || !restoreName.value) return false
  if (isOverwriteInPlace.value && !confirmOverwrite.value) return false
  // v0.9.1.5: imported RPs may have 0 artifacts listed (backend can't
  // enumerate without reading BSL tarball — see task #91). In that case
  // we still allow submit — Velero restores the whole backup contents
  // server-side. If artifactsList IS populated, require at least one
  // selected as before.
  if (artifactsList.value.length > 0 && includedCount.value === 0) return false
  // PRD-001 v2 (#104, finding #1 — 2026-05-31): Pre-flight blockers gate
  // the Restore button with NO override path. The override checkbox was
  // removed; the only ways forward are Apply Fix (server-side patch) or
  // Go-to-Transform (deep solve in TransformSets editor). Every warning
  // must be individually acknowledged via its per-row checkbox.
  if (hasBlockers.value) return false
  if (unacknowledgedWarnings.value.length > 0) return false
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
  ignoredWarnings.value = {}
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

// PRD-001 v2 (#104): "→ Go to Transform" CTA. Each conflict row gets a
// button that hops to the Transforms editor pre-loaded with the conflict
// kind + payload, so the user can author or pick a TransformSet that
// resolves it. PRD-002 (§4.4) builds out the Transforms.vue page; until
// then the route is a placeholder — what matters is the wire format
// (router.push with `?fromConflict=&payload=`) is correct.
//
// The payload is base64-encoded JSON so it round-trips through the URL
// without escaping headaches (conflict.details may contain quotes, etc).
function goToTransform(conflict) {
  let payloadB64 = ''
  try {
    payloadB64 = btoa(unescape(encodeURIComponent(JSON.stringify(conflict))))
  } catch (e) {
    // Should never happen, but if encoding fails we still want to land
    // on the page — the user can re-trigger Pre-flight from there.
    console.warn('Failed to base64-encode conflict payload:', e)
  }
  router.push({
    path: '/transforms',
    query: {
      fromConflict: conflict.kind,
      payload: payloadB64,
      restoreName: restoreName.value
    }
  })
}

// PRD-001 v2 (#104, finding #1): warnings are ignorable but only behind a
// second-step confirm. The checkbox is two-state (off → ON requires
// confirm; ON → off is silent rollback). Without the confirm gate, a
// stray click could silently degrade the restore.
async function toggleIgnoreWarning(idx, conflict) {
  const currentlyIgnored = !!ignoredWarnings.value[idx]
  if (currentlyIgnored) {
    // Un-ignoring is a safe direction — no confirm needed.
    const next = { ...ignoredWarnings.value }
    delete next[idx]
    ignoredWarnings.value = next
    return
  }
  try {
    await ElMessageBox.confirm(
      t('restoreDrawer.ignoreWarningConfirm', { kind: t(`restoreDrawer.conflictKind.${conflict.kind}`) }),
      t('restoreDrawer.ignoreWarningTitle'),
      {
        type: 'warning',
        confirmButtonText: t('restoreDrawer.ignoreWarningOk'),
        cancelButtonText: t('common.cancel')
      }
    )
    ignoredWarnings.value = { ...ignoredWarnings.value, [idx]: true }
  } catch {
    // User cancelled — keep checkbox unchecked. No state change.
  }
}

// PRD-002 v1.3 (2026-05-31): "Apply Suggested Fix" now creates an atomic
// TRANSFORM (not a TransformSet). The response shape changed:
//   { transformName, created, ruleCount }   (was: { transformSetName, ... })
// Velero binds Restores to TransformSets — not Transforms — so the user
// must wire the new Transform into a TransformSet themselves. We help by
// closing the drawer and navigating to /transforms?name=<new> with the
// auto-generated Transform highlighted; from there they can review the
// rules and "Add to TransformSet" before re-opening this drawer to actually
// trigger the Restore.
//
// We still send the full batch of currently-applied fixes so a single
// Transform consolidates every accepted fix (Velero only honors one
// resourceModifierRef per Restore — same constraint as v0.8.x).
async function applyFix(conflict, idx) {
  applyingFixIndex.value = idx
  try {
    const tentativeFixes = {
      ...appliedFixes.value,
      [idx]: '__pending__'
    }
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
    // Field renamed: transformSetName → transformName. Falling back keeps
    // us bug-compatible if an older backend ever responds, but the new
    // backend is what's deployed.
    const name = res.data.transformName || res.data.transformSetName
    const updated = {}
    for (const i of Object.keys(tentativeFixes)) updated[i] = name
    appliedFixes.value = updated
    // We deliberately do NOT auto-select a Transform Set anymore — the new
    // Transform isn't bound to any TransformSet yet. Clear any stale pick.
    selectedTransformSet.value = ''
    await loadTransformSets()

    ElNotification({
      title: t('restoreDrawer.fixCreatedToast', { name }),
      type: 'success',
      duration: 6000,
      position: 'bottom-right'
    })
    // Close this drawer and jump to the Transforms page filtered to the
    // new auto-generated Transform. The Transforms page reads ?name= and
    // opens the viewer drawer for that Transform — ready for the user to
    // review the rules and add it to a Transform Set.
    visibleProxy.value = false
    router.push({ path: '/transforms', query: { name } })
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
    // PRD-001 v2 (#104): conflict set may have shifted on re-check, so
    // wipe per-row warning acknowledgements — user must re-confirm each
    // warning against the fresh result. (Indices aren't stable across
    // re-checks; the safe move is to reset, not migrate.)
    ignoredWarnings.value = {}
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

    // v0.9.1.10 (#105) — "发起 Restore 后零反馈" fix.
    //
    // The old flow flashed a 3-second ElMessage toast then closed the drawer,
    // dumping the user back on the Restore Points list. The Restore CR they
    // just created lives in the Activity stream — which they weren't looking
    // at — so from their seat "什么都没发生". (Mars hit this in the 2026-05-28
    // demo and couldn't tell whether a restore had been created at all.)
    //
    // Now:
    //   - Local restore  → persistent notification + auto-route to the live,
    //     restore-filtered Activity stream so progress is undeniable.
    //   - Cross-cluster  → the Restore CR runs on the REMOTE cluster's Velero
    //     and does NOT surface in this cluster's Activity, so navigating here
    //     would reproduce the "nothing happened" bug. Instead we show a
    //     persistent notice telling the user to switch clusters.
    const submittedName = restoreName.value
    visibleProxy.value = false
    emit('restored', submittedName)

    if (isCrossCluster.value) {
      ElNotification({
        title: t('restoreDrawer.submittedXTitle'),
        message: t('restoreDrawer.submittedXBody', { name: submittedName, cluster: targetCluster.value }),
        type: 'success',
        duration: 0,            // persistent — no local progress view to land on
        position: 'bottom-right'
      })
    } else {
      ElNotification({
        title: t('restoreDrawer.submittedTitle'),
        message: t('restoreDrawer.submittedBody', { name: submittedName, ns: targetNs.value }),
        type: 'success',
        duration: 4500,
        position: 'bottom-right'
      })
      router.push({ path: '/observability', query: { tab: 'activity', type: 'Restore' } })
    }
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
/* v0.9.1.5: imported-RP top banner + as-is restore banner inside Spec section. */
.imported-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 0 16px;
  padding: 8px 12px;
  background: var(--sk-warning-bg, #fff7e6);
  border-left: 3px solid var(--sk-warning, #faad14);
  border-radius: 4px;
  font-size: 12px;
}
.imported-banner :deep(.el-icon) {
  font-size: 14px;
}
.imported-source {
  color: var(--sk-text-caption, #6b7280);
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 11px;
}
.imported-asis-banner {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  background: var(--sk-info-bg, #f0f7ff);
  border-left: 3px solid var(--sk-info, #1677ff);
  border-radius: 4px;
  font-size: 13px;
}
.imported-asis-icon {
  font-size: 18px;
  color: var(--sk-info, #1677ff);
  margin-top: 2px;
  flex-shrink: 0;
}
.imported-asis-banner strong {
  color: var(--sk-text, #1f2937);
  font-weight: 600;
}
.imported-asis-banner p {
  margin: 4px 0 0;
  color: var(--sk-text-caption, #6b7280);
  font-size: 12px;
  line-height: 1.5;
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

/* PRD-001 v2 (#104) — Checklist row actions, severity tags, status footer */
.severity-tag {
  margin-right: 6px;
  vertical-align: middle;
}
.matching-ts {
  margin-top: 6px;
  font-size: 11.5px;
  color: var(--sk-text-caption);
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
.matching-ts-label { margin-right: 2px; }
.matching-ts-tag { font-family: 'SF Mono', Menlo, Consolas, monospace; font-size: 11px; }
.conflict-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed #ebeef5;
}
.conflict-actions-spacer { flex: 1; }
.ignore-disabled-hint {
  color: var(--sk-text-caption);
  font-size: 12px;
}
.conflict-card.is-resolved {
  opacity: 0.7;
  border-color: #c2e7b0 !important;
  background: #f4faf0 !important;
}
.conflict-card.is-resolved .conflict-icon { color: var(--sk-status-success); }
.conflict-card.is-ignored {
  opacity: 0.75;
  background: #fbfbfb !important;
}
.pf-status-footer {
  margin-top: 6px;
  padding: 10px 12px;
  border-radius: 6px;
  background: #f5f7fa;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.pf-status-blocker { color: var(--sk-status-error); font-weight: 500; }
.pf-status-warning { color: var(--sk-status-warning); font-weight: 500; }
.pf-status-ok { color: var(--sk-status-success); font-weight: 500; }
:global(html.dark) .conflict-card.is-resolved { background: #1a2c14 !important; border-color: #2a5a1f !important; }
:global(html.dark) .conflict-card.is-ignored { background: #1f2026 !important; }
:global(html.dark) .pf-status-footer { background: #1f2026; }
:global(html.dark) .conflict-actions { border-top-color: #2c2f36; }
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
