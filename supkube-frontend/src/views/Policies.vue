<template>
  <div class="policies-page">
    <div class="page-header">
      <div class="page-header-text">
        <h3>{{ t('policies.title') }}</h3>
        <p class="page-desc">{{ t('policies.desc') }}</p>
      </div>
      <el-button type="primary"
        :disabled="!auth.canDo('policy.create')"
        :title="!auth.canDo('policy.create') ? t('common.noPermission') : ''"
        @click="openCreateDrawer">
        <el-icon><Plus /></el-icon>
        {{ t('policies.create') }}
      </el-button>
    </div>

    <!-- Kasten-style filter toolbar -->
    <div class="filter-toolbar">
      <el-select v-model="actionFilter" class="filter-action">
        <el-option :label="t('policies.allActions')" value="all" />
        <el-option :label="t('policies.actionSnapshot')" value="Snapshot" />
        <el-option :label="t('policies.actionSnapshotExport')" value="Snapshot+Export" />
      </el-select>
      <el-select v-model="freqFilter" class="filter-freq">
        <el-option :label="t('policies.allFrequencies')" value="all" />
        <el-option :label="t('advisor.schedule.hourly')" value="hourly" />
        <el-option :label="t('advisor.schedule.daily')" value="daily" />
        <el-option :label="t('advisor.schedule.weekly')" value="weekly" />
      </el-select>
      <el-input v-model="nameFilter" :placeholder="t('common.filterByName')" clearable class="filter-name">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="filter-spacer"></span>
      <span class="filter-summary">
        {{ t('policies.viewing', { filtered: filteredSchedules.length, total: schedules.length }) }}
      </span>
    </div>

    <el-card>
      <el-table :data="filteredSchedules" style="width: 100%" v-loading="loading">
        <!-- v0.8.10.6: Name column carries policy name + Action chip +
             Status chip (Validation collapsed to Invalid-only). NO
             namespace chips here — Resources is back as its own column. -->
        <el-table-column prop="metadata.name" :label="t('common.name').toUpperCase()" sortable min-width="240">
          <template #default="{ row }">
            <div class="policy-cell">
              <div class="policy-name">{{ row.metadata?.name }}</div>
              <div class="policy-cell-chips">
                <!-- Agent D: Import 类型蓝色 chip — 复用 tokens.css 里
                     现有的 .sk-chip-type-imported（与 Backups 页一致）。 -->
                <el-tooltip v-if="row._kind === 'import'"
                  :content="t('importPolicy.typeChipTooltip')"
                  placement="top" :show-after="200">
                  <span class="sk-chip sk-chip-type-imported">
                    ⬇ {{ t('importPolicy.typeChip') }}
                  </span>
                </el-tooltip>
                <span v-else class="sk-chip sk-chip-status-muted">{{ actionTextOf(row) }}</span>
                <span v-if="row.spec?.paused" class="sk-chip sk-chip-status-warning">
                  {{ t('policies.paused') }}
                </span>
                <span v-else class="sk-chip sk-chip-status-success">
                  {{ t('policies.active') }}
                </span>
                <span v-if="row._kind !== 'import' && validationOf(row).key !== 'valid'" class="sk-chip sk-chip-status-error">
                  {{ validationOf(row).label }}
                </span>
              </div>
            </div>
          </template>
        </el-table-column>

        <!-- v0.8.10.6: Resources column restored. Per user instruction:
             each namespace renders on its own row (vertical stack), no
             emoji per UI_GUIDELINES §3.1. -->
        <el-table-column :label="t('policies.resources').toUpperCase()" width="160">
          <template #default="{ row }">
            <!-- Agent D: Import 行不针对源集群 ns，显示 — -->
            <span v-if="row._kind === 'import'" class="muted">—</span>
            <div v-else-if="resourceNamespaces(row).length > 0" class="policy-ns-col">
              <el-tooltip
                v-for="ns in resourceNamespaces(row)"
                :key="ns"
                :content="t('policies.namespaceTooltip', { name: ns })"
                placement="top"
                :show-after="200"
              >
                <span class="policy-ns-chip">{{ ns }}</span>
              </el-tooltip>
            </div>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <!-- v0.9.1.8: Storage Location column — shows WHERE this policy
             backs up to. For L2 dual policies the cloud (export-half) BSL
             is the off-site target the user cares about; the local
             (snapshot-half) BSL is shown as the upstream hop. L1 shows its
             single BSL. (Mars demo 2026-05-28 request.) -->
        <el-table-column :label="t('policies.storageLocationCol').toUpperCase()" width="200">
          <template #default="{ row }">
            <div class="policy-bsl-col">
              <!-- Agent D: Import 行展示 source BSL（标 ⬇）。 -->
              <template v-if="row._kind === 'import'">
                <el-tooltip :content="t('importPolicy.sourceBSL') + ': ' + (row._import?.sourceBSL || '—')" placement="top" :show-after="200">
                  <span class="policy-bsl-chip bsl-cloud">⬇ {{ row._import?.sourceBSL || '—' }}</span>
                </el-tooltip>
              </template>
              <template v-else-if="storageLocationOf(row).dual">
                <el-tooltip :content="t('policies.bslLocalTooltip', { name: storageLocationOf(row).local })" placement="top" :show-after="200">
                  <span class="policy-bsl-chip bsl-local">{{ storageLocationOf(row).local }}</span>
                </el-tooltip>
                <span class="bsl-arrow">→</span>
                <el-tooltip :content="t('policies.bslCloudTooltip', { name: storageLocationOf(row).cloud })" placement="top" :show-after="200">
                  <span class="policy-bsl-chip bsl-cloud">{{ storageLocationOf(row).cloud }}</span>
                </el-tooltip>
              </template>
              <template v-else>
                <el-tooltip :content="t('policies.bslCloudTooltip', { name: storageLocationOf(row).local })" placement="top" :show-after="200">
                  <span class="policy-bsl-chip bsl-cloud">{{ storageLocationOf(row).local }}</span>
                </el-tooltip>
              </template>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('policies.frequency').toUpperCase()" width="140">
          <template #default="{ row }">
            <div class="freq-cell">
              <!-- Agent D: Import 行展示 mode + interval/cron。 -->
              <template v-if="row._kind === 'import'">
                <div class="freq-human">
                  {{ row._import?.mode === 'continuous' ? t('importPolicy.modeContinuous') : t('importPolicy.modeScheduled') }}
                </div>
                <code class="freq-cron">{{ row.spec?.schedule || '—' }}</code>
              </template>
              <!-- v0.8.10.5: paused + 0 0 1 1 * cron is SupKube's
                   "On Demand" idiom (no automatic run, fire via Run
                   Once only). Per user feedback show that explicitly. -->
              <template v-else-if="row.spec?.paused">
                <div class="sk-body-strong">{{ t('policies.onDemand') }}</div>
              </template>
              <template v-else>
                <div class="freq-human">{{ frequencyLabelOf(row) }}</div>
                <code class="freq-cron">{{ row.spec?.schedule }}</code>
              </template>
            </div>
          </template>
        </el-table-column>

        <!-- v0.8.10.1: Restore Point count column. Clickable when >0 —
             takes the user to /backups pre-filtered by this policy's
             name. Backups.vue reads ?policy=<name> and matches the
             velero.io/schedule-name label on either half of the dual
             pair. Muted "0" when no RPs (policy created but never ran). -->
        <el-table-column :label="t('policies.restorePoints').toUpperCase()" width="140" sortable
          :sort-method="(a, b) => (a.restorePointCount || 0) - (b.restorePointCount || 0)">
          <template #default="{ row }">
            <a
              v-if="(row.restorePointCount || 0) > 0"
              class="rp-count rp-count-link"
              @click.stop="goPolicyRPs(row)"
            >
              <el-icon><FolderOpened /></el-icon>
              {{ row.restorePointCount }}
            </a>
            <span v-else class="rp-count rp-count-zero">
              <el-icon><FolderOpened /></el-icon>
              0
            </span>
          </template>
        </el-table-column>

        <!-- v0.8.10.5: Last Run Time — date + time on two lines so the
             column can be narrower. (Old single-line "5/22/2026, 5:00:23
             PM" pushed the table to ~1500px wide.) -->
        <el-table-column :label="t('policies.lastRunTime').toUpperCase()" width="120">
          <template #default="{ row }">
            <div v-if="row.status?.lastBackup" class="stacked-time">
              <div class="sk-body">{{ formatDate(row.status.lastBackup) }}</div>
              <div class="sk-caption">{{ formatTimeOnly(row.status.lastBackup) }}</div>
            </div>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>

        <!-- v0.8.10.5: Last Run Status column dropped. The Active /
             Paused chip moved into the Name cell (see top column),
             which is the only meaningful state for a policy at rest. -->


        <el-table-column label="" width="60" align="right">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="cmd => handleCommand(cmd, row)">
              <el-button class="more-btn" text>
                <span class="dots">⋮</span>
              </el-button>
              <template #dropdown>
                <!-- Agent D: Import 行只展示 Run-once / Pause / Delete；
                     Snapshot 行保持原有菜单不动。 -->
                <el-dropdown-menu v-if="row._kind === 'import'">
                  <el-dropdown-item command="importRunOnce">{{ t('importPolicy.actionRunOnce') }}</el-dropdown-item>
                  <el-dropdown-item command="importPause">
                    {{ row.spec?.paused ? t('importPolicy.actionResume') : t('importPolicy.actionPause') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="importDelete" divided>{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
                <el-dropdown-menu v-else>
                  <el-dropdown-item command="view">{{ t('common.view') }}</el-dropdown-item>
                  <el-dropdown-item command="revalidate">{{ t('policies.revalidate') }}</el-dropdown-item>
                  <el-dropdown-item command="edit">{{ t('common.edit') }}</el-dropdown-item>
                  <el-dropdown-item command="editYaml">{{ t('policies.editYaml') }}</el-dropdown-item>
                  <el-dropdown-item command="runOnce" divided>{{ t('policies.runOnce') }}</el-dropdown-item>
                  <el-dropdown-item command="pause">{{ row.spec?.paused ? t('policies.resume') : t('policies.pause') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Policy Details drawer
         v0.8.10.2: aligned to UI_GUIDELINES §5 (Action Details template).
         H1 + H2 + chip row, sections with H3 uppercase, sticky footer. -->
    <el-drawer
      v-model="viewDrawerVisible"
      direction="rtl"
      :size="'560px'"
      :destroy-on-close="true"
      :with-header="false"
      class="sk-policy-view-drawer"
    >
      <div v-if="viewRow" class="sk-drawer">
        <button class="sk-drawer-close" type="button" @click="viewDrawerVisible = false" aria-label="Close">×</button>

        <!-- ════ Header zone ════ -->
        <div class="sk-drawer-header">
          <div class="sk-h1">{{ t('activity.detail.titlePolicy') }}</div>
          <div class="sk-h2 sk-drawer-subject" :title="viewRow.metadata.name">{{ viewRow.metadata.name }}</div>
          <div class="sk-drawer-chips">
            <span class="sk-chip" :class="viewRow.spec?.paused ? 'sk-chip-status-warning' : 'sk-chip-status-success'">
              {{ viewRow.spec?.paused ? t('policies.paused') : t('policies.active') }}
            </span>
            <span class="sk-chip sk-chip-status-muted">{{ actionTextOf(viewRow) }}</span>
            <span v-if="viewRow.restorePointCount > 0" class="sk-chip sk-chip-type-snapshot">
              {{ viewRow.restorePointCount }} {{ t('policies.restorePoints') }}
            </span>
          </div>
        </div>

        <!-- ════ Section: SCHEDULE ════ -->
        <section class="sk-section">
          <div class="sk-h3">{{ t('policies.frequency') }}</div>
          <div class="sk-fields">
            <div class="sk-field-label sk-caption">{{ t('policies.frequency') }}</div>
            <div class="sk-field-value sk-body">{{ frequencyLabelOf(viewRow) }}</div>
            <div class="sk-field-label sk-caption">Cron</div>
            <div class="sk-field-value"><code class="sk-mono">{{ viewRow.spec?.schedule || '—' }}</code></div>
          </div>
        </section>

        <!-- ════ Section: TARGETS ════ -->
        <section class="sk-section">
          <div class="sk-h3">{{ t('policies.resources') }}</div>
          <div class="sk-fields">
            <div class="sk-field-label sk-caption">{{ t('common.namespace') }}</div>
            <div class="sk-field-value sk-body">{{ (viewRow.spec?.template?.includedNamespaces || ['*']).join(', ') }}</div>
          </div>
        </section>

        <!-- ════ Section: RETENTION & STORAGE ════ -->
        <section class="sk-section">
          <div class="sk-h3">{{ t('policies.exportRetention') }}</div>
          <div class="sk-fields">
            <div class="sk-field-label sk-caption">{{ t('policies.exportRetention') }}</div>
            <div class="sk-field-value sk-body">{{ formatTTL(viewRow.spec?.template?.ttl) }}</div>
            <div class="sk-field-label sk-caption">{{ t('policies.storageProfile') }}</div>
            <div class="sk-field-value">
              <code class="sk-mono">{{ viewRow.spec?.template?.storageLocation || 'default' }}</code>
            </div>
          </div>
        </section>

        <!-- ════ Section: LAST RUN ════ -->
        <section class="sk-section" v-if="viewRow.status?.lastBackup">
          <div class="sk-h3">{{ t('policies.lastRunTime') }}</div>
          <div class="sk-body">{{ formatTime(viewRow.status.lastBackup) }}</div>
        </section>
      </div>

      <!-- ════ Sticky bottom action bar ════ -->
      <div v-if="viewRow" class="sk-drawer-footer">
        <el-button
          v-if="viewRow.restorePointCount > 0"
          type="primary"
          @click="goPolicyRPsFromDrawer(viewRow)"
        >
          {{ t('policies.restorePoints') }} ({{ viewRow.restorePointCount }})
        </el-button>
        <span class="sk-drawer-footer-spacer"></span>
        <el-button @click="viewDrawerVisible = false">{{ t('common.close') }}</el-button>
      </div>
    </el-drawer>

    <!-- Edit YAML drawer: full CR as YAML, read-only for now -->
    <el-drawer
      v-model="yamlDrawerVisible"
      :title="t('policies.editYaml')"
      direction="rtl"
      size="720px"
      :destroy-on-close="true"
    >
      <div v-if="yamlRow" class="view-body">
        <pre class="yaml-block">{{ asYaml(yamlRow) }}</pre>
        <p class="form-hint" style="margin-top: 12px">
          {{ t('policies.yamlReadonlyHint') }}
        </p>
      </div>
    </el-drawer>

    <!-- Create/Edit Policy Drawer (v0.7 Actions model + v0.8.5 step 5 edit) -->
    <el-drawer
      v-model="showCreateDialog"
      :title="editMode ? t('policies.editPolicy', { name: editingName }) : t('policies.newPolicy')"
      direction="rtl"
      size="560px"
      :destroy-on-close="false"
      class="new-policy-drawer"
      @close="onDrawerClose"
    >
      <el-form :model="createForm" label-position="top" class="kasten-form">
        <el-form-item required>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('common.name') }}</strong>
              <span class="kasten-label-help">{{ editMode ? t('policies.nameImmutable') : t('policies.nameHelp') }}</span>
            </div>
          </template>
          <el-input v-model="createForm.name" placeholder="daily-backup" :disabled="editMode" />
        </el-form-item>

        <el-form-item>
          <template #label><strong>{{ t('policies.comments') }}</strong></template>
          <el-input
            v-model="createForm.comments"
            type="textarea"
            :rows="2"
            :placeholder="t('policies.commentsPlaceholder')"
          />
        </el-form-item>

        <!-- Agent D / Import Policy 2026-06-01 — Action Type 选择条。
             Snapshot Policy = 源集群备份（保留原有整套表单不动）。
             Import Policy   = 目标集群从共享 BSL 拉新 RP（持续 DR）。
             Edit 模式禁切（一条 Schedule 和一条 ImportPolicy 是不同 CR，
             不能就地互转）；只在 Create 模式露出。 -->
        <el-form-item v-if="!editMode">
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.actionType') }}</strong>
              <span class="kasten-label-help">{{ t('policies.actionTypeHelp') }}</span>
            </div>
          </template>
          <div class="kasten-pill-group">
            <button type="button" class="kasten-pill"
              :class="{ 'is-active': createForm.actionType === 'snapshot' }"
              @click="createForm.actionType = 'snapshot'">
              📸 {{ t('policies.actionTypeSnapshot') }}
            </button>
            <button type="button" class="kasten-pill"
              :class="{ 'is-active': createForm.actionType === 'import' }"
              @click="createForm.actionType = 'import'">
              ⬇ {{ t('policies.actionTypeImport') }}
            </button>
          </div>
        </el-form-item>

        <!-- ════════════════════════════════════════════════════════════
             SNAPSHOT POLICY 分支（原有 Kasten K10 表单整段不动一行）
             ════════════════════════════════════════════════════════════ -->
        <template v-if="createForm.actionType === 'snapshot'">

        <!-- v0.9.1.12 PRD-009 (Mars 2026-06-01) — Kasten K10 model:
             Snapshot is always taken; user only toggles whether to export.
             Replaces the old L1/L2 pill group. Backend stays identical:
             toggle binds to createForm.export.enabled (the boolean ADR-025
             dual-schedule already consumes), so no schema change.
             selectAction() is reused to preserve the snapshot-only guardrail
             dialog when flipping off. -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.snapshotSectionTitle') }}</strong>
              <span class="kasten-label-help">{{ t('policies.snapshotSectionHelp') }}</span>
            </div>
          </template>
          <div class="snapshot-always-on">
            <span class="kasten-pill is-active is-locked">📸 {{ t('policies.snapshotAlwaysOnPill') }}</span>
            <span class="snapshot-always-on-note">· {{ t('policies.alwaysOn') }}</span>
          </div>
        </el-form-item>

        <!-- Enable Backups via Snapshot Exports (Kasten K10 official wording) -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.enableExportLabel') }}</strong>
              <span class="kasten-label-help">{{ t('policies.enableExportHelp') }}</span>
            </div>
          </template>
          <el-switch
            :model-value="createForm.export.enabled"
            size="large"
            inline-prompt
            :active-text="t('policies.enableExportOnText')"
            :inactive-text="t('policies.enableExportOffText')"
            @change="(v) => selectAction(v ? 'snapshot-export' : 'snapshot')"
          />
          <p v-if="!createForm.export.enabled" class="action-disabled-warning" style="margin-top: 10px">
            ⚠ {{ t('policies.snapshotOnlyWarn') }}
          </p>
        </el-form-item>

        <!-- Backup Frequency: 6 preset buttons (Kasten parity) -->
        <el-form-item>
          <template #label><strong>{{ t('policies.backupFrequency') }}</strong></template>
          <div class="kasten-pill-grid">
            <button
              v-for="f in frequencyChoices"
              :key="f.key"
              type="button"
              class="kasten-pill"
              :class="{ 'is-active': createForm.frequency === f.key }"
              @click="selectFrequency(f.key)"
            >{{ f.label }}</button>
          </div>
          <!-- v0.8.7.6: On Demand notice. Shown when user picks the
               new "On Demand" preset to explain that the policy will
               only run when manually triggered via Run Once. -->
          <p v-if="createForm.frequency === 'ondemand'" class="ondemand-notice">
            ℹ️ {{ t('policies.onDemandNotice') }}
          </p>
        </el-form-item>

        <!-- Resources (Included Namespaces) — moved here in v0.8.7.4 to
             mirror Kasten's "Frequency → Selection Type → Applications"
             order. Picking what to protect comes BEFORE retention/storage
             details because it answers the "is this even relevant to me"
             question first. -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.resources') }}</strong>
              <span class="kasten-label-help">{{ t('policies.resourcesHelp') }}</span>
            </div>
          </template>
          <el-select
            v-model="createForm.includedNamespaces"
            multiple
            filterable
            allow-create
            :placeholder="t('policies.namespacesPlaceholder')"
            style="width: 100%"
          >
            <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
          </el-select>
        </el-form-item>

        <!-- Snapshot Retention (always shown) -->
        <el-form-item>
          <template #label><strong>{{ t('policies.snapshotRetention') }}</strong></template>
          <el-select v-model="createForm.snapshot.retention" style="width: 100%">
            <el-option label="6 hours" value="6h" />
            <el-option label="12 hours" value="12h" />
            <el-option :label="`24 hours (${t('common.create').toLowerCase() === 'create' ? 'default' : '默认'})`" value="24h" />
            <el-option label="3 days" value="72h" />
            <el-option label="7 days" value="168h" />
          </el-select>
        </el-form-item>

        <!-- Export-only fields (L2 mode) -->
        <template v-if="createForm.export.enabled">
          <el-form-item>
            <template #label><strong>{{ t('policies.exportRetention') }}</strong></template>
            <el-select v-model="createForm.export.retention" style="width: 100%">
              <el-option label="7 days" value="168h" />
              <el-option label="14 days" value="336h" />
              <el-option :label="`30 days (${t('common.create').toLowerCase() === 'create' ? 'default' : '默认'})`" value="720h" />
              <el-option label="60 days" value="1440h" />
              <el-option label="90 days" value="2160h" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <template #label><strong>{{ t('policies.storageProfile') }}</strong></template>
            <!-- v0.8.7.4: dropdown sourced from /storage-locations.
                 Unavailable BSLs are listed but disabled so user can't
                 pick a known-broken target. Provider chip on the right
                 of each option gives one-glance "is this Azure or S3?". -->
            <el-select
              v-model="createForm.export.storageLocation"
              filterable
              :placeholder="t('policies.storageProfilePlaceholder')"
              style="width: 100%"
            >
              <!-- v0.9.1.10 (#101 finding 1): explicit "Auto" so '' reads as
                   a deliberate choice. Backend resolves the effective cloud
                   BSL (flagged-default / sole / named-default). -->
              <el-option :label="t('policies.storageProfileAuto')" value="" />
              <!-- v0.8.12 LBS3: hide the in-cluster Local BSL from this
                   Cloud-only selector (bslRole=local). The Cloud half
                   should never land in the Local store; pairing rules
                   downstream depend on this separation. -->
              <el-option
                v-for="bsl in cloudOnlyStorageLocations"
                :key="bsl.name"
                :label="bsl.name"
                :value="bsl.name"
                :disabled="bsl.phase !== 'Available'"
              >
                <span style="float: left">
                  {{ bsl.name }}
                  <span v-if="bsl.isDefault" class="bsl-default-badge">default</span>
                  <span v-if="bsl.objectLockEnabled" style="margin-left: 6px; color: #059669; font-size: 12px">🛡</span>
                </span>
                <span style="float: right; color: #909399; font-size: 12px">
                  {{ bsl.provider }}{{ bsl.phase !== 'Available' ? ' · ' + bsl.phase : '' }}
                </span>
              </el-option>
            </el-select>
            <!-- v0.8.12 LBS3: warn when the selected Cloud BSL has NO
                 Object Lock. 3-2-1-1-0 needs "1 Immutable"; without lock
                 the cloud copy is ransom-vulnerable. -->
            <p v-if="selectedCloudBSL && !selectedCloudBSL.objectLockEnabled" class="form-hint form-hint-warn">
              ⚠ {{ t('policies.noObjectLockWarn') }}
            </p>
            <p v-else-if="selectedCloudBSL && selectedCloudBSL.objectLockEnabled" class="form-hint form-hint-ok">
              🛡 {{ t('policies.objectLockOk', { mode: selectedCloudBSL.objectLockMode || 'governance' }) }}
            </p>
            <p v-if="cloudOnlyStorageLocations.length === 0" class="form-hint">
              {{ t('policies.storageProfileEmpty') }}
            </p>
          </el-form-item>
        </template>

        <!-- Data Path — two-column visual reflects the Snapshot / Export
             toggle above (v0.9.1.12 PRD-009 reworded from L1/L2):

               Export OFF → left column active (CSI / Metadata-only)
               Export ON  → right column active (Data Mover / Filesystem)
                            (Data Mover does a CSI snapshot under the hood;
                             Filesystem skips snapshot and reads files directly)

             Implementation: ONE source-of-truth in createForm.snapshot.dataPath.
             A watcher migrates the value when the Export toggle flips so the
             user can't end up with a "Export ON + CSI-only path" combo (which
             would silently fail to export). The columns are visual scaffolding;
             the gating logic lives in setDataPath() + selectAction(). -->
        <!-- (legacy v0.8.7.5 L1/L2 comment retired here; preserved in git blame) -->
        <!-- Left col is keyed on !export.enabled, right col on export.enabled —
             so they auto-track the new Kasten-style switch above. -->
        <el-form-item>
          <template #label>
            <div class="kasten-label-block">
              <strong>{{ t('policies.dataPath') }}</strong>
              <span class="kasten-label-help">{{ t('policies.dataPathHelp') }}</span>
            </div>
          </template>
          <div class="data-path-2col">
            <!-- ─── Left: Snapshot only (Export toggle OFF) ─── -->
            <div class="data-path-col" :class="{ 'is-active-col': !createForm.export.enabled, 'is-disabled-col': createForm.export.enabled }">
              <div class="data-path-col-head">
                <span class="data-path-col-title">📸 {{ t('policies.dataPathSection.snapshotOnly') }}</span>
                <span class="data-path-col-sub">{{ t('policies.dataPathSection.snapshotOnlySub') }}</span>
              </div>
              <div class="data-path-col-body">
                <button
                  type="button"
                  class="kasten-pill data-path-pill"
                  :class="{ 'is-active': createForm.snapshot.dataPath === 'csi-snapshot' }"
                  :disabled="createForm.export.enabled"
                  :title="t('policies.dataPathTip.csi')"
                  @click="setDataPath('csi-snapshot')"
                >📸 {{ t('policies.dataPathChoice.csi') }}</button>
                <button
                  type="button"
                  class="kasten-pill data-path-pill"
                  :class="{ 'is-active': createForm.snapshot.dataPath === 'metadata-only' }"
                  :disabled="createForm.export.enabled"
                  :title="t('policies.dataPathTip.meta')"
                  @click="setDataPath('metadata-only')"
                >📋 {{ t('policies.dataPathChoice.meta') }}</button>
              </div>
            </div>

            <!-- ─── Right: Snapshot Export (Export toggle ON) ─── -->
            <div class="data-path-col" :class="{ 'is-active-col': createForm.export.enabled, 'is-disabled-col': !createForm.export.enabled }">
              <div class="data-path-col-head">
                <span class="data-path-col-title">🚚 {{ t('policies.dataPathSection.snapshotExport') }}</span>
                <span class="data-path-col-sub">{{ t('policies.dataPathSection.snapshotExportSub') }}</span>
              </div>
              <div class="data-path-col-body">
                <button
                  type="button"
                  class="kasten-pill data-path-pill"
                  :class="{ 'is-active': createForm.snapshot.dataPath === 'data-mover' }"
                  :disabled="!createForm.export.enabled"
                  :title="t('policies.dataPathTip.mover')"
                  @click="setDataPath('data-mover')"
                >🚚 {{ t('policies.dataPathChoice.mover') }}</button>
                <button
                  type="button"
                  class="kasten-pill data-path-pill"
                  :class="{ 'is-active': createForm.snapshot.dataPath === 'filesystem' }"
                  :disabled="!createForm.export.enabled"
                  :title="t('policies.dataPathTip.fs')"
                  @click="setDataPath('filesystem')"
                >📁 {{ t('policies.dataPathChoice.fs') }}</button>
              </div>
            </div>
          </div>
          <p v-if="createForm.snapshot.dataPath === 'data-mover' || createForm.snapshot.dataPath === 'filesystem'"
             class="data-path-prereq">
            ℹ️ {{ t('policies.dataPathPrereq') }}
          </p>
        </el-form-item>

        <!-- (Resources block was moved above to right after Backup
             Frequency in v0.8.7.4 to match Kasten ordering. Don't
             re-add it here.) -->

        <!-- Capability detection result (only shown for CSI mode + selected ns).
             Data Mover also runs CSI snapshots under the hood, so the same
             capability check applies — show it for both 'csi-snapshot' and
             'data-mover'. -->
        <el-form-item v-if="(createForm.snapshot.dataPath === 'csi-snapshot' || createForm.snapshot.dataPath === 'data-mover') && createForm.includedNamespaces.length > 0">
          <template #label><strong>{{ t('policies.csiCheck') }}</strong></template>
          <div v-loading="capabilityLoading" class="capability-result">
            <div v-if="capabilityError" class="capability-error">
              ⚠ {{ capabilityError }}
            </div>
            <template v-else-if="capabilityResults.length > 0">
              <div v-for="r in capabilityResults" :key="r.namespace" class="capability-row">
                <div class="capability-ns">
                  <span class="capability-ns-name">{{ r.namespace }}</span>
                  <el-tag v-if="r.incompatibleCount === 0" type="success" size="small">CSI ready</el-tag>
                  <el-tag v-else type="danger" size="small">
                    {{ r.incompatibleCount }} incompatible
                  </el-tag>
                </div>
                <ul v-if="r.incompatibleCount > 0" class="capability-pvc-list">
                  <li v-for="p in r.pvcs.filter(x => !x.csiSnapshot)" :key="p.pvc">
                    <code>{{ p.pvc }}</code> on <code>{{ p.storageClass || '—' }}</code>
                    <span class="capability-reason">— {{ p.reason }}</span>
                  </li>
                </ul>
              </div>
              <div v-if="csiBlocked()" class="capability-blocker">
                ⛔ {{ t('policies.csiBlocker') }}
              </div>
            </template>
          </div>
        </el-form-item>
        </template>

        <!-- ════════════════════════════════════════════════════════════
             IMPORT POLICY 分支 (Agent D 2026-06-01)
             目标集群从共享 BSL 拉新 RP 的持续 DR。后端独立 CR
             (POST /import-policies)，不动 ADR-025 dual-schedule。
             ════════════════════════════════════════════════════════════ -->
        <template v-else>
          <!-- Source Storage Location -->
          <el-form-item required>
            <template #label>
              <div class="kasten-label-block">
                <strong>🗄 {{ t('importPolicy.sourceBSL') }}</strong>
                <span class="kasten-label-help">{{ t('importPolicy.sourceBSLHelp') }}</span>
              </div>
            </template>
            <el-select v-model="createForm.import.sourceBSL"
              filterable
              :placeholder="t('importPolicy.sourceBSLPlaceholder')"
              style="width:100%">
              <el-option v-for="bsl in storageLocations" :key="bsl.name"
                :label="bsl.name" :value="bsl.name"
                :disabled="bsl.phase !== 'Available'">
                <span style="float:left">
                  {{ bsl.name }}
                  <span v-if="bsl.isDefault" class="bsl-default-badge">default</span>
                </span>
                <span style="float:right;color:#909399;font-size:12px">
                  {{ bsl.provider }}{{ bsl.phase !== 'Available' ? ' · ' + bsl.phase : '' }}
                </span>
              </el-option>
            </el-select>
          </el-form-item>

          <!-- Import Mode (Continuous / Scheduled) -->
          <el-form-item>
            <template #label>
              <div class="kasten-label-block">
                <strong>⏱ {{ t('importPolicy.mode') }}</strong>
              </div>
            </template>
            <div class="kasten-pill-group">
              <button type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.mode === 'continuous' }"
                @click="createForm.import.mode = 'continuous'">
                {{ t('importPolicy.modeContinuous') }}
              </button>
              <button type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.mode === 'scheduled' }"
                @click="createForm.import.mode = 'scheduled'">
                {{ t('importPolicy.modeScheduled') }}
              </button>
            </div>
          </el-form-item>

          <!-- Continuous: Poll Interval -->
          <el-form-item v-if="createForm.import.mode === 'continuous'">
            <template #label>
              <div class="kasten-label-block">
                <strong>📡 {{ t('importPolicy.continuousInterval') }}</strong>
                <span class="kasten-label-help">{{ t('importPolicy.continuousIntervalHelp') }}</span>
              </div>
            </template>
            <div class="kasten-pill-grid pill-grid-4">
              <button v-for="iv in importPollPresets" :key="iv.value"
                type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.continuousInterval === iv.value }"
                @click="createForm.import.continuousInterval = iv.value">
                {{ iv.label }}
              </button>
            </div>
            <el-input v-model="createForm.import.continuousInterval"
              :placeholder="t('importPolicy.intervalPlaceholder')"
              style="margin-top:8px" />
          </el-form-item>

          <!-- Scheduled: Cron -->
          <el-form-item v-if="createForm.import.mode === 'scheduled'">
            <template #label>
              <div class="kasten-label-block">
                <strong>📅 {{ t('importPolicy.cron') }}</strong>
                <span class="kasten-label-help">{{ t('importPolicy.cronHelp') }}</span>
              </div>
            </template>
            <div class="kasten-pill-grid pill-grid-3">
              <button v-for="p in importCronPresets" :key="p.value"
                type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.schedule === p.value }"
                @click="createForm.import.schedule = p.value">
                {{ p.label }}
              </button>
            </div>
            <el-input v-model="createForm.import.schedule"
              :placeholder="t('importPolicy.cronPlaceholder')"
              style="margin-top:8px" />
          </el-form-item>

          <!-- Source Cluster Filter -->
          <el-form-item>
            <template #label>
              <div class="kasten-label-block">
                <strong>🆔 {{ t('importPolicy.sourceClusterID') }}</strong>
                <span class="kasten-label-help">{{ t('importPolicy.sourceClusterIDHelp') }}</span>
              </div>
            </template>
            <el-input v-model="createForm.import.sourceClusterID"
              :placeholder="t('importPolicy.sourceClusterIDPlaceholder')" />
          </el-form-item>

          <!-- Fingerprint Verification -->
          <el-form-item>
            <template #label>
              <div class="kasten-label-block">
                <strong>🔐 {{ t('importPolicy.fingerprintMode') }}</strong>
                <span class="kasten-label-help">{{ t('importPolicy.fingerprintHelp') }}</span>
              </div>
            </template>
            <div class="kasten-pill-group">
              <button type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.fingerprintMode === 'enforce' }"
                @click="createForm.import.fingerprintMode = 'enforce'">
                {{ t('importPolicy.fingerprintEnforce') }}
              </button>
              <button type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.fingerprintMode === 'warn' }"
                @click="createForm.import.fingerprintMode = 'warn'">
                {{ t('importPolicy.fingerprintWarn') }}
              </button>
              <button type="button" class="kasten-pill"
                :class="{ 'is-active': createForm.import.fingerprintMode === 'disabled' }"
                @click="createForm.import.fingerprintMode = 'disabled'">
                {{ t('importPolicy.fingerprintDisabled') }}
              </button>
            </div>
          </el-form-item>

          <!-- RPO estimate (Continuous only) -->
          <el-form-item v-if="createForm.import.mode === 'continuous' && importRpoEstimateMin > 0">
            <div class="import-rpo-banner">
              <div class="import-rpo-line">{{ t('importPolicy.rpoEstimate', { min: importRpoEstimateMin }) }}</div>
              <div class="import-rpo-note">{{ t('importPolicy.rpoEstimateNote') }}</div>
            </div>
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <!-- v0.8.12 LBS4: 3-2-1-1-0 score preview.
             Shows how the policy-being-edited contributes to the cluster's
             compliance score. Live-computed from the form state — same
             rules engine the Dashboard DR Topology uses (kept in sync via
             shared definition in the comment below).
             Not blocking: a 2/5 policy still saves, but the chip strip
             makes the gap visible so the user can fix it before clicking
             Create rather than discovering it later on the Dashboard. -->
        <div class="policy-score-strip" v-if="createForm.actionType !== 'import'">
          <div class="pss-head">
            <span class="sk-caption pss-label">3-2-1-1-0</span>
            <span class="pss-count">{{ scorePreview.filter((r) => r.ok).length }}/5</span>
            <span class="sk-caption pss-hint">{{ t('policies.scorePreviewHint') }}</span>
          </div>
          <div class="pss-dots">
            <span
              v-for="(r, i) in scorePreview"
              :key="`pss-${i}`"
              class="pss-item"
              :class="{ 'is-ok': r.ok, 'is-bad': !r.ok }"
              :title="r.note || r.label"
            >
              <span class="pss-dot">{{ r.ok ? '●' : '○' }}</span>
              <span class="pss-rule">{{ r.label }}</span>
            </span>
          </div>
        </div>

        <div class="drawer-footer">
          <el-button @click="showCreateDialog = false">{{ t('common.cancel') }}</el-button>
          <el-button
            type="primary"
            @click="handleSubmit"
            :loading="creating"
            :disabled="csiBlocked()"
          >
            {{ editMode ? t('common.save') : t('common.create') }}
          </el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Plus, Search, FolderOpened } from '@element-plus/icons-vue'
import {
  getSchedules, getSchedule, createSchedule, patchSchedule, deleteSchedule,
  runScheduleOnce, getNamespaces, getNamespaceStorageCapability,
  getStorageLocations,
  // Agent D 2026-06-01 — Import Policy 跨集群 DR
  getImportPolicies, createImportPolicy, pauseImportPolicy,
  resumeImportPolicy, runImportPolicyOnce, deleteImportPolicy
} from '../api/velero'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

// v0.8.10.1: jump from a Policy row's RP count to the Restore Points
// page pre-filtered by this policy. Same UX pattern as the Applications
// page's RP count cell (which uses ?namespace=). Here we use ?policy=
// because a policy can span multiple namespaces and the user's question
// is "what RPs did THIS policy produce", not "what's in this ns".
const goPolicyRPs = (row) => {
  const name = row?._policy?.name || row?.metadata?.name
  if (!name) return
  router.push({ path: '/backups', query: { policy: name } })
}
// v0.8.10.2: same nav as goPolicyRPs but also closes the View drawer
// first so the user doesn't see the drawer linger over the new page.
const goPolicyRPsFromDrawer = (row) => {
  viewDrawerVisible.value = false
  goPolicyRPs(row)
}
import { useAuth } from '../composables/useAuth'
const auth = useAuth()

const schedules = ref([])

// Kasten-style filter toolbar state
const actionFilter = ref('all')
const freqFilter = ref('all')
const nameFilter = ref('')
const viewDrawerVisible = ref(false)
const yamlDrawerVisible = ref(false)
const viewRow = ref(null)
const yamlRow = ref(null)

// Derive what action the Velero Schedule performs (Snapshot vs Snapshot+Export)
// from annotations set by Create dialog (v0.7 Actions model).
const actionTextOf = (row) => {
  const ann = row?.metadata?.annotations || {}
  if (ann['supkube.io/export-enabled'] === 'false') return t('policies.actionSnapshot')
  return t('policies.actionSnapshotExport')
}

// Map cron expression to friendly bucket. Same presets as Advisor.
const FREQ_PRESETS = {
  '0 * * * *': 'hourly',
  '0 */6 * * *': 'every6h',
  '0 */12 * * *': 'every12h',
  '0 0 * * *': 'daily',
  '0 0 * * 0': 'weekly',
  '0 0 1 * *': 'monthly'
}
const frequencyKeyOf = (row) => FREQ_PRESETS[(row?.spec?.schedule || '').trim()] || 'custom'
const frequencyLabelOf = (row) => {
  const k = frequencyKeyOf(row)
  if (k === 'custom') return t('advisor.schedule.custom', { cron: row?.spec?.schedule || '' })
  return t(`advisor.schedule.${k}`)
}

// Validation is heuristic — Velero has no single "valid" field. We check
// the schedule has a cron and at least one selector in the template.
const validationOf = (row) => {
  const hasCron = !!row?.spec?.schedule
  const t1 = row?.spec?.template
  const hasTemplate = !!(t1?.includedNamespaces?.length || t1?.includedResources?.length || t1?.labelSelector)
  if (hasCron && hasTemplate) return { key: 'valid', icon: '✓', label: t('policies.valid') }
  return { key: 'invalid', icon: '⚠', label: t('policies.invalid') }
}

const resourceNamespaces = (row) => {
  const ns = row?.spec?.template?.includedNamespaces || []
  return ns.length === 0 ? ['*'] : ns
}

// v0.9.1.8: where does this policy back up to? Row is the flattened
// snapshot half + _policy.exportHalf (see fetchSchedules). For an L2 dual
// policy the export half's BSL is the off-site cloud target; the snapshot
// half's BSL is the local hop. L1 has only the one BSL.
const storageLocationOf = (row) => {
  const localBsl = row?.spec?.template?.storageLocation || 'default'
  const exportHalf = row?._policy?.exportHalf
  if (exportHalf) {
    return {
      dual: true,
      local: localBsl,
      cloud: exportHalf.spec?.template?.storageLocation || 'default'
    }
  }
  return { dual: false, local: localBsl, cloud: null }
}

const filteredSchedules = computed(() => {
  const name = nameFilter.value.trim().toLowerCase()
  return schedules.value.filter((row) => {
    // Agent D: Snapshot/Snapshot+Export 这两个 actionFilter 选项只针对
    // Snapshot 策略行；遇到 Import 行直接排除（频率/导出概念不适用）。
    if (actionFilter.value === 'Snapshot' || actionFilter.value === 'Snapshot+Export') {
      if (row._kind === 'import') return false
      const isSnapOnly = row?.metadata?.annotations?.['supkube.io/export-enabled'] === 'false'
      if (actionFilter.value === 'Snapshot' && !isSnapOnly) return false
      if (actionFilter.value === 'Snapshot+Export' && isSnapOnly) return false
    }
    if (freqFilter.value !== 'all') {
      // Import 行没有 cron-bucket 概念，跳过频率过滤直接保留。
      if (row._kind !== 'import') {
        const k = frequencyKeyOf(row)
        if (freqFilter.value === 'hourly' && !['hourly', 'every6h', 'every12h'].includes(k)) return false
        if (freqFilter.value === 'daily' && k !== 'daily') return false
        if (freqFilter.value === 'weekly' && k !== 'weekly') return false
      }
    }
    if (name && !(row.metadata?.name || '').toLowerCase().includes(name)) return false
    return true
  })
})

// Minimal YAML serializer for the View YAML drawer. Strips status churn,
// keeps spec readable. Not for round-trip editing (drawer is read-only in
// v0.7.8; v0.8 swaps in a Monaco editor).
function toYaml(v, indent) {
  const pad = '  '.repeat(indent)
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'string') {
    if (v.includes('\n') || v.includes(':') || v.includes('#')) return JSON.stringify(v)
    return v
  }
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (Array.isArray(v)) {
    if (v.length === 0) return '[]'
    return '\n' + v.map(item => `${pad}- ${toYaml(item, indent + 1).trimStart()}`).join('\n')
  }
  if (typeof v === 'object') {
    const keys = Object.keys(v).filter(k => v[k] !== undefined && v[k] !== null)
    if (keys.length === 0) return '{}'
    return '\n' + keys.map(k => {
      const child = toYaml(v[k], indent + 1)
      if (child.startsWith('\n')) return `${pad}${k}:${child}`
      return `${pad}${k}: ${child}`
    }).join('\n')
  }
  return String(v)
}
const asYaml = (obj) => {
  if (!obj) return ''
  const clean = {
    apiVersion: obj.apiVersion || 'velero.io/v1',
    kind: obj.kind || 'Schedule',
    metadata: {
      name: obj.metadata?.name,
      namespace: obj.metadata?.namespace,
      labels: obj.metadata?.labels,
      annotations: obj.metadata?.annotations
    },
    spec: obj.spec
  }
  return toYaml(clean, 0).trim()
}

const handleCommand = (cmd, row) => {
  switch (cmd) {
    case 'view': openView(row); break
    case 'revalidate': handleRevalidate(row); break
    case 'edit': handleEdit(row); break
    case 'editYaml': openYaml(row); break
    case 'runOnce': handleRunOnce(row); break
    case 'pause': togglePause(row); break
    case 'delete': handleDelete(row); break
    // Agent D: Import Policy 行的菜单项
    case 'importRunOnce': handleImportRunOnce(row); break
    case 'importPause':   handleImportPauseToggle(row); break
    case 'importDelete':  handleImportDelete(row); break
  }
}

// v0.8.10.5 fix: openView used to re-fetch via getSchedule(name) and
// store the PolicyAggregate response directly. But v0.8.9 changed
// /schedules to return PolicyAggregate (policyName / mode / snapshotSchedule
// / exportSchedule), while the View drawer template reads flat fields
// like viewRow.metadata.name + viewRow.spec.template.ttl. The mismatch
// made every field `undefined` → drawer rendered as gray overlay with
// no content.
//
// Fix: the `row` we receive is ALREADY flattened by fetchSchedules
// (see the .map() there). Use it directly. No need to re-fetch since
// the list view already has fresh data, and re-fetching just gives us
// a shape mismatch.
const openView = (row) => {
  viewRow.value = row
  viewDrawerVisible.value = true
}

const openYaml = async (row) => {
  try {
    const res = await getSchedule(row.metadata.name)
    yamlRow.value = res.data
  } catch {
    yamlRow.value = row
  }
  yamlDrawerVisible.value = true
}

const handleRevalidate = (row) => {
  const v = validationOf(row)
  if (v.key === 'valid') {
    ElMessage.success(t('policies.revalidateOk'))
  } else {
    ElMessage.warning(t('policies.revalidateFail'))
  }
}

// v0.8.5 step 5: real Edit form.
//
// The Create drawer doubles as the Edit drawer — same fields, same layout,
// same handlers. The only differences are:
//   - Drawer title / button label
//   - Name + ns scope are disabled (rename = new policy; ns moves are
//     possible but limited to the editor's scope and surfaced as a warning)
//   - Submit goes to PATCH /schedules/:name instead of POST /schedules
//
// We hydrate `createForm` from the existing Velero Schedule + the
// supkube.io/* intent annotations so the editor sees the SAME shape they'd
// see in Create, not raw Velero fields.
const handleEdit = async (row) => {
  if (!auth.canDo('policy.edit')) {
    ElMessage.warning(t('common.noPermission'))
    return
  }
  // Fetch the live spec — the row from the list may be stale (controller
  // status fields aren't in our list payload).
  let live = row
  try {
    const res = await getSchedule(row.metadata.name)
    live = res.data
  } catch (e) {
    ElMessage.warning(t('policies.editFetchFailed'))
  }
  createForm.value = hydrateFormFromSchedule(live)
  editMode.value = true
  // v0.9.1.6 fix: GET /schedules/:name returns a PolicyAggregate
  // ({policyName, snapshotSchedule, exportSchedule}), NOT a raw Schedule —
  // so live.metadata.name is undefined and editingName ended up "", making
  // the save PATCH /api/v1/schedules/ (empty name) → 404. row.metadata.name
  // is the canonical name we fetched with, so prefer it; fall back through
  // the aggregate fields for robustness against either response shape.
  editingName.value = row.metadata?.name
    || live.policyName
    || live.snapshotSchedule?.metadata?.name
    || live.metadata?.name
    || ''
  // v0.8.7.4: refresh BSL list so the Storage Profile dropdown has the
  // current Available/Unavailable state of every BSL, not whatever was
  // cached when the page first loaded.
  fetchStorageLocations()
  showCreateDialog.value = true
}

// hydrateFormFromSchedule: reverse of collapseToVelero().
// v0.8.9 — accepts either the new PolicyAggregate shape (from GET
// /schedules/:name post-v0.8.9) or a legacy raw Schedule. The
// extractAggregate helper normalizes both into a {snap, exp} pair
// so the rest of the function doesn't care which it got.
const hydrateFormFromSchedule = (input) => {
  const f = defaultForm()

  // Normalize: PolicyAggregate vs raw Schedule.
  let snap = null
  let exp = null
  let policyName = ''
  if (input?.policyName !== undefined) {
    // New aggregate shape.
    snap = input.snapshotSchedule || null
    exp = input.exportSchedule || null
    policyName = input.policyName || snap?.metadata?.name || ''
  } else if (input?.spec) {
    // Legacy raw Schedule.
    snap = input
    policyName = input.metadata?.name || ''
  }
  // The "primary" half (snapshot) drives all common form fields.
  const sched = snap
  const tmpl = sched?.spec?.template || {}
  const ann = sched?.metadata?.annotations || {}

  f.name = policyName
  f.comments = ann['supkube.io/comments'] || ''
  f.includedNamespaces = Array.isArray(tmpl.includedNamespaces) ? [...tmpl.includedNamespaces] : []

  const snapSched = ann['supkube.io/snapshot-schedule'] || sched?.spec?.schedule || '0 0 * * *'
  const snapRet = ann['supkube.io/snapshot-retention'] || '24h'
  // v0.8.7 dataPath derivation. Prefer the annotation; fall back to
  // old volume-mode; last resort, infer from the spec triple.
  // v0.8.9 quirk: when in dual mode, the snapshot half ALWAYS has
  // snapshotMoveData=false, but the user's "intent" is still
  // Data Mover (since the export half does upload). So when we see
  // a paired policy, force dataPath='data-mover' regardless of the
  // snapshot half's flags.
  let dataPath = ann['supkube.io/data-path']
  if (!dataPath) {
    const oldMode = ann['supkube.io/volume-mode']
    if (oldMode === 'csi') dataPath = 'csi-snapshot'
    else if (oldMode === 'filesystem') dataPath = 'filesystem'
  }
  if (!dataPath) {
    if (tmpl.snapshotMoveData)          dataPath = 'data-mover'
    else if (tmpl.defaultVolumesToFsBackup) dataPath = 'filesystem'
    else if (tmpl.snapshotVolumes)      dataPath = 'csi-snapshot'
    else                                 dataPath = 'metadata-only'
  }
  // If there's an export half, the policy is effectively Data Mover OR
  // Filesystem — pick whichever the export half's spec uses.
  if (exp) {
    const expTmpl = exp.spec?.template || {}
    if (expTmpl.defaultVolumesToFsBackup) dataPath = 'filesystem'
    else dataPath = 'data-mover'
  }
  f.snapshot.schedule = snapSched
  f.snapshot.schedulePreset = snapSched
  f.snapshot.retention = snapRet
  f.snapshot.dataPath = dataPath

  // v0.8.9 Export side derivation:
  //   - exp present → dual mode L2; pull export retention + BSL from
  //     the export half's spec.
  //   - exp missing + annotation export-enabled=true → legacy L2 (pre-
  //     v0.8.9 single-schedule that intended L2); pull from annotation.
  //   - else → L1.
  let expEnabled = false
  let expSched = sched?.spec?.schedule || '0 0 * * *'
  let expRet = '720h'
  let expBsl = 'default'
  if (exp) {
    expEnabled = true
    expSched = exp.spec?.schedule || expSched
    const expTmpl = exp.spec?.template || {}
    if (expTmpl.ttl) expRet = expTmpl.ttl
    if (expTmpl.storageLocation) expBsl = expTmpl.storageLocation
  } else if (ann['supkube.io/export-enabled'] === 'true') {
    expEnabled = true
    expSched = ann['supkube.io/export-schedule'] || expSched
    expRet = ann['supkube.io/export-retention'] || (tmpl.ttl ? tmpl.ttl : '720h')
    expBsl = tmpl.storageLocation || 'default'
  }
  f.export.enabled = expEnabled
  f.export.schedule = expSched
  f.export.schedulePreset = expSched
  f.export.retention = expRet
  f.export.storageLocation = expBsl

  // v0.8.7.6 Frequency derivation:
  //   - sched.spec.paused == true  → ondemand (overrides any cron match)
  //   - cron matches a preset      → that preset
  //   - no match + not paused      → fall back to daily (the safest
  //                                  default for an unknown cron;
  //                                  v0.9 will add "Advanced cron" input
  //                                  for real custom cron strings)
  f.paused = !!sched?.spec?.paused
  if (f.paused) {
    f.frequency = 'ondemand'
  } else {
    const matchKey = Object.entries(FREQUENCY_TO_CRON).find(([key, cron]) => key !== 'ondemand' && cron === snapSched)?.[0]
    f.frequency = matchKey || 'daily'
  }

  return f
}

const handleRunOnce = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      t('policies.runOnceConfirmBody', { name }),
      t('policies.runOnceConfirmTitle'),
      { confirmButtonText: t('policies.runOnce'), cancelButtonText: t('common.cancel'), type: 'info' }
    )
  } catch { return }
  try {
    const res = await runScheduleOnce(name)
    ElMessage.success(t('policies.runOnceStarted', { backup: res.data?.backupName || '' }))
  } catch (e) {
    ElMessage.error('Run Once failed: ' + (e.response?.data?.error || e.message))
  }
}
const namespaces = ref([])
// v0.8.7.4: Storage Profile dropdown. Pulled from /storage-locations once
// per drawer open. Includes Phase so the dropdown can disable
// Unavailable BSLs (preventing the user from picking a known-broken one).
// Each entry: { name, provider, phase, isDefault }
const storageLocations = ref([])

// v0.8.12 LBS3: derived list excluding the in-cluster Local BSL. The
// Cloud BSL selector should only show user-added cloud destinations
// (Azure Blob / S3 / OSS / Tencent COS). The Local store gets its own
// dedicated treatment via the Local Backup Store card.
const cloudOnlyStorageLocations = computed(() =>
  storageLocations.value.filter((b) => b.bslRole !== 'local')
)

// Currently-selected Cloud BSL object (or null). Drives the inline
// Object Lock badge + the no-lock warning below the selector.
const selectedCloudBSL = computed(() => {
  const name = createForm.value?.export?.storageLocation
  if (!name) return null
  return storageLocations.value.find((b) => b.name === name) || null
})

// v0.8.12 LBS4: 3-2-1-1-0 score preview for the policy being created/edited.
//
// Rules — must stay in sync with backend internal/api/v1/topology.go
// (computeTopologyScore). The backend's score is global ("does ANY ns
// satisfy this?"); here it's per-policy ("if this policy ran, would
// IT satisfy each rule?"). Same five buckets, different scope.
//
//   3 Copies     = L2 dual policy (writes to BOTH Local + Cloud BSL)
//                  → source PVC + local + cloud = 3 distinct copies
//   2 Media      = Local BSL (in-cluster) + Cloud BSL (object storage)
//                  → satisfied iff L2 dual
//   1 Off-site   = Cloud BSL exists AND is Available
//   1 Immutable  = at least one half writes to a BSL with Object Lock
//                  (Local has it on by default; Cloud depends on user setup)
//   0 Errors     = configuration-level: no missing required field that
//                  would make the first run fail (BSL chosen, ns chosen,
//                  schedule valid). Runtime failures are evaluated by
//                  the global Dashboard scoreboard.
//
// Local BSL state is loaded lazily — we read from storageLocations
// (which is fetched on mount). If the in-cluster Local BSL hasn't been
// enabled yet, the Local copies / immutability assumptions degrade.
const scorePreview = computed(() => {
  const f = createForm.value || {}
  const isDual = !!(f.export && f.export.enabled)
  const cloud = selectedCloudBSL.value
  const cloudAvailable = !!(cloud && cloud.phase === 'Available')
  const cloudLocked = !!(cloud && cloud.objectLockEnabled)

  // Local BSL: SupKube-managed in-cluster MinIO. Detected via the
  // supkube.io/bsl-role=local label that LBS1's Helm chart stamps.
  const localBSL = storageLocations.value.find((b) => b.bslRole === 'local')
  const localAvailable = !!(localBSL && localBSL.phase === 'Available')
  const localLocked = !!(localBSL && localBSL.objectLockEnabled)

  // L1 policies write only to Cloud (Local is implicit via SupKube's
  // policypair controller; we still count "Local" as a copy when the
  // Local BSL exists, even for L1, because the controller will route
  // there). v0.8.12 simplification: treat L1 as "Cloud only".
  const hasLocal = isDual && localAvailable
  const hasCloud = cloudAvailable

  // Config-level "0 Errors" check: required fields populated
  const hasNamespaces = !!(f.namespaces && f.namespaces.length > 0)
  const hasName = !!f.name
  const hasSchedule = !!(f.snapshot && (f.snapshot.cron || f.snapshot.frequency))
  const configClean = hasName && hasNamespaces && hasSchedule && (isDual ? cloudAvailable : true)

  return [
    {
      label: t('topology.score.three'),
      ok: hasLocal && hasCloud,
      note: !hasLocal ? t('policies.scoreNotes.localMissing') : (!hasCloud ? t('policies.scoreNotes.cloudMissing') : '')
    },
    {
      label: t('topology.score.two'),
      ok: hasLocal && hasCloud,
      note: !isDual ? t('policies.scoreNotes.notDual') : ''
    },
    {
      label: t('topology.score.one'),
      ok: cloudAvailable,
      note: !cloud ? t('policies.scoreNotes.noCloudPicked') : (!cloudAvailable ? t('policies.scoreNotes.cloudUnavailable') : '')
    },
    {
      label: t('topology.score.immutable'),
      ok: cloudLocked || (hasLocal && localLocked),
      note: t('policies.scoreNotes.enableLock')
    },
    {
      label: t('topology.score.zero'),
      ok: configClean,
      note: !hasNamespaces ? t('policies.scoreNotes.pickNs') : (!hasName ? t('policies.scoreNotes.pickName') : '')
    }
  ]
})
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
// v0.8.5 step 5: edit mode for the same drawer.
//   editMode === false  → Create flow (POST /schedules)
//   editMode === true   → Edit flow (PATCH /schedules/:name)
// editingName persists across the open-drawer lifecycle; we use it for
// the PATCH URL and to show "Editing <name>" in the title.
const editMode = ref(false)
const editingName = ref('')
// (schedules + filter state already declared above near the toolbar logic)

// v0.7 Actions model: Snapshot (always on) + Export (default on, opt-out
// triggers confirmation). Both have independent schedule + retention in the
// UI; v0.7 maps them to a single Velero Schedule with the shorter cron and
// the longer ttl, with intent recorded in annotations for v0.9 to consume.
const defaultForm = () => ({
  // Agent D 2026-06-01: 'snapshot' = 原 Kasten 备份策略（默认）；
  //                     'import'   = 目标集群从共享 BSL 拉新 RP 的持续 DR。
  // 字段挂在表单顶层；submit handler 根据它派发到不同 endpoint。
  actionType: 'snapshot',
  name: '',
  comments: '',
  import: {
    sourceBSL: '',
    mode: 'continuous',              // 'continuous' | 'scheduled'
    continuousInterval: '30s',       // Go duration; ≥30s
    schedule: '*/15 * * * *',        // 5-field cron
    sourceClusterID: '',             // 留空 = 接受全部 source
    fingerprintMode: 'enforce'       // 'enforce' | 'warn' | 'disabled'
  },
  // Top-level frequency choice (Kasten parity). Each preset maps to a
  // snapshot.schedule cron via FREQUENCY_TO_CRON below. Default Daily =
  // safe baseline for most workloads.
  frequency: 'daily',
  includedNamespaces: [],
  // v0.8.7.6: 'paused' is the Velero Schedule field that disables
  // automatic cron runs. We expose it as the "On Demand" frequency:
  // policy is created with a real cron (whatever was last set) but
  // paused=true so the Schedule controller never triggers a Backup.
  // User can still trigger backups manually via the Run Once action.
  paused: false,
  snapshot: {
    enabled: true,
    schedulePreset: '0 0 * * *',
    schedule: '0 0 * * *',
    retention: '24h',
    // v0.8.7 dataPath — 4-way Velero spec dispatch:
    //   csi-snapshot  → snapshotVolumes=true,  fsBackup=false, moveData=false
    //   data-mover    → snapshotVolumes=true,  fsBackup=false, moveData=true
    //   filesystem    → snapshotVolumes=false, fsBackup=true,  moveData=false
    //   metadata-only → snapshotVolumes=false, fsBackup=false, moveData=false
    // Default 'csi-snapshot' matches Velero's own default behavior so
    // a user clicking through without thinking gets the safest fast path.
    dataPath: 'csi-snapshot'
  },
  export: {
    enabled: true,
    schedulePreset: '0 0 * * *',
    schedule: '0 0 * * *',
    retention: '720h',
    // v0.9.1.10 (#101 finding 1): '' = "Auto (default cloud location)". Was
    // hardcoded 'default', which the payload forwarded literally → Velero
    // looked for a BSL named "default" that doesn't exist on AKS (only
    // "azure-blob") → both policy halves' Backups failed. Empty lets the
    // backend resolve the effective cloud BSL.
    storageLocation: ''
  }
})
const createForm = ref(defaultForm())

// Agent D: Import Policy 的轮询/调度预设。值是后端期望的 Go duration / cron。
const importPollPresets = computed(() => [
  { value: '30s', label: t('importPolicy.pollPreset30s') },
  { value: '60s', label: t('importPolicy.pollPreset60s') },
  { value: '2m',  label: t('importPolicy.pollPreset2m') },
  { value: '5m',  label: t('importPolicy.pollPreset5m') }
])
const importCronPresets = computed(() => [
  { value: '*/5 * * * *',  label: t('importPolicy.schedulePreset5m') },
  { value: '*/15 * * * *', label: t('importPolicy.schedulePreset15m') },
  { value: '0 * * * *',    label: t('importPolicy.schedulePreset1h') },
  { value: '0 2 * * *',    label: t('importPolicy.schedulePresetDaily') },
  { value: '0 3 * * 0',    label: t('importPolicy.schedulePresetWeekly') }
])

// RPO worst-case 估计：仅 Continuous 模式显示。
// v1 实现 = pollInterval × 2 (扫描 + 处理窗口)；与源 backup 间隔加和才是端到端 RPO。
const importRpoEstimateMin = computed(() => {
  if (createForm.value?.actionType !== 'import') return 0
  if (createForm.value?.import?.mode !== 'continuous') return 0
  const iv = createForm.value.import.continuousInterval || ''
  const m = /^(\d+)(s|m|h)$/.exec(iv.trim())
  if (!m) return 0
  const n = parseInt(m[1], 10)
  const seconds = m[2] === 's' ? n : (m[2] === 'm' ? n * 60 : n * 3600)
  // worst-case = 2x pollInterval, 转换为分钟向上取整
  return Math.max(1, Math.ceil((seconds * 2) / 60))
})

// 校验：Continuous 间隔必须是 Go duration 且 ≥ 30s。
const importIntervalValid = () => {
  const iv = (createForm.value?.import?.continuousInterval || '').trim()
  const m = /^(\d+)(s|m|h)$/.exec(iv)
  if (!m) return false
  const n = parseInt(m[1], 10)
  const seconds = m[2] === 's' ? n : (m[2] === 'm' ? n * 60 : n * 3600)
  return seconds >= 30
}

// 简单 5-字段 cron 校验：5 个空格分隔的段（不严格验语义，只看 shape）。
const importCronValid = () => {
  const c = (createForm.value?.import?.schedule || '').trim()
  return /^\S+\s+\S+\s+\S+\s+\S+\s+\S+$/.test(c)
}

// Kasten-style frequency preset map. Each option drives BOTH the snapshot
// and export schedule (export stays in lock-step by default — v0.9 will
// expose an "Advanced: separate cadences" toggle).
//
// v0.8.7.6: replaced 'custom' (which silently set cron to '0 * * * *' ≡
// "Every hour" while UI claimed "Custom" — a pure footgun) with
// 'ondemand'. On Demand = Velero spec.paused=true, no auto-runs at all,
// only manually triggered via Run Once. Matches Kasten K10's "On Demand"
// preset. The cron stays at whatever the user last picked (or daily)
// because Velero needs SOME schedule value even when paused; the
// controller honors paused and skips firing.
const FREQUENCY_TO_CRON = {
  hourly:   '0 * * * *',
  every6h:  '0 */6 * * *',
  daily:    '0 0 * * *',
  weekly:   '0 0 * * 0',
  monthly:  '0 0 1 * *',
  ondemand: '0 0 * * *'  // placeholder; paused=true makes this irrelevant
}
const frequencyChoices = computed(() => [
  { key: 'hourly',   label: t('advisor.schedule.hourly') },
  { key: 'every6h',  label: t('advisor.schedule.every6h') },
  { key: 'daily',    label: t('advisor.schedule.daily') },
  { key: 'weekly',   label: t('advisor.schedule.weekly') },
  { key: 'monthly',  label: t('advisor.schedule.monthly') },
  { key: 'ondemand', label: t('policies.frequencyOnDemand') }
])

const selectFrequency = (key) => {
  createForm.value.frequency = key
  const cron = FREQUENCY_TO_CRON[key]
  if (cron) {
    createForm.value.snapshot.schedule = cron
    createForm.value.snapshot.schedulePreset = cron
    createForm.value.export.schedule = cron
    createForm.value.export.schedulePreset = cron
  }
  // v0.8.7.6: On Demand pauses the schedule; any other preset un-pauses
  // it (in case the user is converting from On Demand back to a cadence).
  createForm.value.paused = (key === 'ondemand')
}

// Action button-group handler: snapshot-only vs snapshot+export. Mirrors
// the original Export checkbox toggle behavior (including the guardrail
// confirm dialog when the user disables Export).
const selectAction = (mode) => {
  if (mode === 'snapshot' && createForm.value.export.enabled) {
    // Reuse the existing snapshot-only guardrail dialog.
    onExportToggle(false)
    return
  }
  if (mode === 'snapshot-export' && !createForm.value.export.enabled) {
    createForm.value.export.enabled = true
  }
}

// v0.8.7.5: Data Path setter respects the L1/L2 Action constraint.
// Components call setDataPath(value) instead of directly assigning so
// invalid combos (e.g. picking 'csi-snapshot' while L2 export is on)
// can't be reached even via DevTools manipulation of v-model bindings.
//
// Why a function and not v-model: the 4-pill grid used to write
// directly to createForm.snapshot.dataPath, which allowed users to
// silently end up with "L2 selected + CSI-only path" → no export
// despite the L2 label. Forcing through this helper keeps the UI
// self-consistent.
const setDataPath = (value) => {
  const L1Only = ['csi-snapshot', 'metadata-only']
  const L2Only = ['data-mover', 'filesystem']
  if (createForm.value.export.enabled && L1Only.includes(value)) return  // L2 active, L1 option clicked → ignore
  if (!createForm.value.export.enabled && L2Only.includes(value)) return // L1 active, L2 option clicked → ignore
  createForm.value.snapshot.dataPath = value
}

// migrateDataPathOnActionChange: when user flips L1 ↔ L2, the currently-
// selected dataPath may no longer be valid. We auto-migrate to the most
// "intent-preserving" sibling:
//
//   L1 → L2:
//     csi-snapshot  → data-mover    (both use CSI; L2 just adds Kopia export)
//     metadata-only → metadata-only invalid for L2; pick data-mover as fallback
//
//   L2 → L1:
//     data-mover    → csi-snapshot  (drop Kopia export, keep CSI)
//     filesystem    → metadata-only (no CSI snapshot was ever taken; nearest sibling)
const migrateDataPathOnActionChange = (l2Enabled) => {
  const dp = createForm.value.snapshot.dataPath
  if (l2Enabled) {
    if (dp === 'csi-snapshot') createForm.value.snapshot.dataPath = 'data-mover'
    else if (dp === 'metadata-only') createForm.value.snapshot.dataPath = 'data-mover'
  } else {
    if (dp === 'data-mover') createForm.value.snapshot.dataPath = 'csi-snapshot'
    else if (dp === 'filesystem') createForm.value.snapshot.dataPath = 'metadata-only'
  }
}

// Watch Action toggle (createForm.export.enabled) and migrate.
watch(
  () => createForm.value.export.enabled,
  (newVal) => { migrateDataPathOnActionChange(newVal) }
)

// Capability detection — when user picks CSI mode + selected namespaces, check
// each namespace's PVCs are on CSI-snapshot-capable StorageClasses. We block
// "Create" if any incompatibility found; UI surfaces the offending PVCs.
const capabilityResults = ref([]) // [{ namespace, incompatibleCount, pvcs: [...] }]
const capabilityLoading = ref(false)
const capabilityError = ref('')

const incompatiblePVCs = (capabilityResults.value)
// v0.8.7: 'csi-snapshot' AND 'data-mover' both depend on CSI snapshot
// support — Data Mover takes the CSI snapshot first then ships it.
// So the same capability gate applies to both. 'filesystem' walks the
// FS (no snapshot needed) and 'metadata-only' captures no PVs, so they
// pass the gate trivially.
const csiBlocked = () => {
  const dp = createForm.value.snapshot.dataPath
  if (dp !== 'csi-snapshot' && dp !== 'data-mover') return false
  return capabilityResults.value.some(r => r.incompatibleCount > 0)
}

const refreshCapability = async () => {
  capabilityError.value = ''
  const nsList = createForm.value.includedNamespaces
  const dp = createForm.value.snapshot.dataPath
  // Check capability for any path that depends on CSI snapshots.
  if ((dp !== 'csi-snapshot' && dp !== 'data-mover') || nsList.length === 0) {
    capabilityResults.value = []
    return
  }
  capabilityLoading.value = true
  try {
    const results = []
    for (const ns of nsList) {
      const res = await getNamespaceStorageCapability(ns)
      results.push(res.data)
    }
    capabilityResults.value = results
  } catch (e) {
    capabilityError.value = e.response?.data?.error || e.message
    capabilityResults.value = []
  } finally {
    capabilityLoading.value = false
  }
}

watch(
  () => [createForm.value.includedNamespaces.slice(), createForm.value.snapshot.dataPath],
  () => { refreshCapability() },
  { deep: false }
)

const formatTime = (ts) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}
// v0.8.10.5: date / time split for the stacked column layout.
// Locale-aware via Intl so zh-CN sees "2026/5/23" and en sees "5/23/2026".
const formatDate = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleDateString()
}
const formatTimeOnly = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// Derive Protection Level from a Velero Schedule. Reads our intent
// annotations first (set by v0.7+ policies); falls back to spec inference
// for pre-existing schedules (treat them as L2 if Velero will write to BSL,
// which is always true — Velero always exports). v0.9 adds L3 (immutable).
const protectionLevel = (row) => {
  const ann = row?.metadata?.annotations || {}
  const exportEnabled = ann['supkube.io/export-enabled']
  if (exportEnabled === 'false') {
    return {
      key: 'l1',
      label: 'L1 Snapshot Only',
      icon: '⚠',
      tooltip: 'Snapshot-only — NOT a durable backup. Data is lost if the underlying storage fails. Use only for dev/staging or fast rollback scenarios.'
    }
  }
  // Future: detect immutable BSL — for now everything with export is L2.
  return {
    key: 'l2',
    label: 'L2 Backup',
    icon: '✓',
    tooltip: 'Snapshot + Export to object storage. Durable backup that survives storage failure.'
  }
}

// Velero schedule.spec.template.ttl defaults to zero when unset; the actual
// retention then falls back to Velero's server-side default (30 days). Show
// users the effective retention instead of the misleading literal "0s".
const formatTTL = (ttl) => {
  if (!ttl || ttl === '0s' || ttl === '0' || ttl === '0h' || ttl === '0h0m0s') {
    return 'Default (30d)'
  }
  const match = /^(\d+)h$/.exec(ttl)
  if (match) {
    const hours = parseInt(match[1], 10)
    if (hours > 0 && hours % 24 === 0) return `${hours / 24}d`
  }
  return ttl
}

const onSnapshotPresetChange = (val) => {
  if (val !== 'custom') createForm.value.snapshot.schedule = val
}
const onExportPresetChange = (val) => {
  if (val === 'custom') return
  if (val === 'same') {
    createForm.value.export.schedule = createForm.value.snapshot.schedule
  } else {
    createForm.value.export.schedule = val
  }
}

// Default-Export-checked guardrail (v0.7-policy-2 part 1): when user opts
// out of Export, force a confirmation dialog. ElMessageBox is async, so we
// revert the toggle immediately and re-enable only on confirm.
const onExportToggle = (newVal) => {
  if (newVal === true) return // turning ON: no friction
  // newVal === false: confirm + revert if user backs out
  createForm.value.export.enabled = true // revert optimistically
  ElMessageBox.confirm(
    'Snapshot alone is not a backup. Data is lost if the underlying storage fails. ' +
    'This is acceptable only for development/staging environments or fast rollback scenarios. ' +
    'Continue without Export?',
    'Disable Export?',
    {
      type: 'warning',
      confirmButtonText: 'Yes, snapshot-only',
      cancelButtonText: 'Keep Export enabled',
      confirmButtonClass: 'el-button--warning'
    }
  ).then(() => {
    createForm.value.export.enabled = false
  }).catch(() => {
    // user backed out; export stays enabled
  })
}

// ─── Agent D 2026-06-01: Import Policy submit + 行操作 ──────────────────
// 所有错误码经 mapImportError() 转 i18n toast，未知错误回落到 server.error。
const mapImportError = (e) => {
  const data = e?.response?.data || {}
  const code = data.code || data.errorCode
  if (code && (code + '').startsWith('ERR_')) {
    // bsl/intval 等可携参的错误用 t() 渲染
    return t('errors.' + code, { bsl: data.bsl || '' })
  }
  return data.error || e.message
}

const submitImportPolicy = async () => {
  const f = createForm.value
  if (!f.import.sourceBSL) {
    ElMessage.warning(t('importPolicy.sourceBSLHelp'))
    return
  }
  if (f.import.mode === 'continuous' && !importIntervalValid()) {
    ElMessage.error(t('importPolicy.intervalTooShort'))
    return
  }
  if (f.import.mode === 'scheduled' && !importCronValid()) {
    ElMessage.error(t('importPolicy.cronInvalid'))
    return
  }
  creating.value = true
  try {
    // 共享 contract 的 spec shape — 与 Agent A/B/C 后端对齐。
    const body = {
      name: f.name,
      spec: {
        sourceBSL: f.import.sourceBSL,
        mode: f.import.mode,
        continuousInterval: f.import.mode === 'continuous' ? f.import.continuousInterval : undefined,
        schedule: f.import.mode === 'scheduled' ? f.import.schedule : undefined,
        sourceClusterID: f.import.sourceClusterID || undefined,
        fingerprintMode: f.import.fingerprintMode,
        targetVeleroNamespace: 'velero',
        paused: false
      }
    }
    await createImportPolicy(body)
    ElMessage.success(t('importPolicy.createdToast', { name: f.name }))
    showCreateDialog.value = false
    editMode.value = false
    editingName.value = ''
    createForm.value = defaultForm()
    await fetchSchedules()
  } catch (e) {
    ElMessage.error(mapImportError(e))
  } finally {
    creating.value = false
  }
}

const handleImportRunOnce = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await runImportPolicyOnce(name)
    ElMessage.success(t('importPolicy.runOnceStarted', { name }))
    await fetchSchedules()
  } catch (e) {
    ElMessage.error(mapImportError(e))
  }
}

const handleImportPauseToggle = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  const isPaused = !!row.spec?.paused
  try {
    if (isPaused) {
      await resumeImportPolicy(name)
      ElMessage.success(t('importPolicy.resumed', { name }))
    } else {
      await pauseImportPolicy(name)
      ElMessage.success(t('importPolicy.paused', { name }))
    }
    await fetchSchedules()
  } catch (e) {
    ElMessage.error(mapImportError(e))
  }
}

const handleImportDelete = async (row) => {
  const name = row?.metadata?.name
  if (!name) return
  try {
    await ElMessageBox.confirm(
      `Delete import policy "${name}"?`,
      'Delete Import Policy',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteImportPolicy(name)
    ElMessage.success(`Import Policy "${name}" deleted`)
    await fetchSchedules()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(mapImportError(e))
  }
}

// 把 ImportPolicy 资源转成与 Schedule 行同形状的对象，挂 _kind='import'
// 作为模板分支标记。spec.paused 复用同名字段以便 Active/Paused chip 通用。
const normalizeImportPolicy = (ip) => ({
  metadata: ip.metadata || {},
  spec: {
    paused: !!(ip.spec?.paused),
    // 给 frequency 列展示一些有用信息：mode + interval/schedule
    schedule: ip.spec?.mode === 'scheduled'
      ? (ip.spec?.schedule || '')
      : (ip.spec?.continuousInterval || '')
  },
  status: ip.status || {},
  _kind: 'import',
  _import: ip.spec || {},
  restorePointCount: ip.status?.restorePointsImported || 0
})

const fetchSchedules = async () => {
  loading.value = true
  try {
    // 并行拉 Snapshot Schedules 和 Import Policies — Agent D。
    // ImportPolicies 失败不阻塞 Schedules 渲染（404 = 老后端没 ship）。
    const [schedRes, ipRes] = await Promise.all([
      getSchedules(),
      getImportPolicies().catch((e) => {
        console.warn('getImportPolicies failed (backend may not support yet):', e?.message)
        return { data: { items: [] } }
      })
    ])
    const items = schedRes.data.items || []
    const snapshotRows = items.map(agg => {
      // Defensive: an aggregate with no snapshot half shouldn't exist
      // (backend invariant), but if it does we synthesize an empty
      // shell so the row doesn't crash.
      const snap = agg.snapshotSchedule || { metadata: { name: agg.policyName }, spec: {} }
      return {
        ...snap,
        _kind: 'snapshot',
        _policy: {
          name: agg.policyName,
          mode: agg.mode,
          exportHalf: agg.exportSchedule || null
        },
        restorePointCount: agg.restorePointCount || 0
      }
    })
    const importItems = ipRes.data?.items || []
    const importRows = importItems.map(normalizeImportPolicy)
    schedules.value = [...snapshotRows, ...importRows]
  } catch (e) {
    ElMessage.error('Failed to load policies')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchNamespaces = async () => {
  try {
    const res = await getNamespaces()
    const items = res.data.namespaces || res.data.items || res.data || []
    namespaces.value = items.map(ns => ns.metadata?.name || ns).filter(Boolean)
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

// v0.8.7.4: load BSL list for the Storage Profile dropdown. Each entry
// keeps Phase + provider so the dropdown can hint "(Unavailable)" or
// "Azure" beside the name. Disabled when Phase != Available so a user
// can't pick a broken target and not realize until the first run.
const fetchStorageLocations = async () => {
  try {
    const res = await getStorageLocations()
    const items = res.data.items || res.data || []
    storageLocations.value = items.map(bsl => ({
      name: bsl.metadata?.name || '',
      provider: bsl.spec?.provider || '',
      phase: bsl.status?.phase || 'Unknown',
      isDefault: !!bsl.spec?.default,
      // v0.8.12 LBS3: surface Object Lock state into the wizard so the
      // BSL selector can show 🛡 next to locked options and warn when
      // the user picks a non-locked cloud BSL.
      objectLockEnabled: (bsl.metadata?.annotations || {})['supkube.io/object-lock'] === 'true',
      objectLockMode: (bsl.metadata?.annotations || {})['supkube.io/object-lock-mode'] || '',
      // bsl-role lets us hide the in-cluster Local BSL from the Cloud
      // selector — it's not a meaningful Cloud destination.
      bslRole: (bsl.metadata?.labels || {})['supkube.io/bsl-role'] || ''
    })).filter(b => b.name)
  } catch (e) {
    console.error('Failed to load storage locations:', e)
    storageLocations.value = []
  }
}

// Parse Go duration suffixes (h only — Velero's TTL field is `metav1.Duration`,
// in practice we only ever store h). Returns null if input doesn't match.
const parseHours = (s) => {
  const m = /^(\d+)h$/.exec(s || '')
  return m ? parseInt(m[1], 10) : null
}

// v0.7 collapse: single Velero Schedule per Policy. cron = shorter of the
// two; ttl = longer of the two. Snapshot/Export intent preserved in
// annotations so v0.9's self-managed scheduler can hydrate from existing
// Schedules without losing user intent.
const collapseToVelero = (form) => {
  const snapHours = parseHours(form.snapshot.retention) || 24
  const expHours = form.export.enabled ? (parseHours(form.export.retention) || 720) : 0

  // cron picking: simple heuristic — when Export is on, use export cron
  // (slower) by default for cost. If user wants tighter RPO they pick
  // matching cron explicitly via "Same as Snapshot".
  const cron = form.export.enabled ? form.export.schedule : form.snapshot.schedule

  // v0.8.7: 4-way dataPath dispatch into Velero's 3-flag triple.
  //   csi-snapshot  → snapshotVolumes=true,  fsBackup=false, moveData=false
  //   data-mover    → snapshotVolumes=true,  fsBackup=false, moveData=true
  //   filesystem    → snapshotVolumes=false, fsBackup=true,  moveData=false
  //   metadata-only → snapshotVolumes=false, fsBackup=false, moveData=false
  const dp = form.snapshot.dataPath || 'csi-snapshot'
  const snapshotVolumes          = dp === 'csi-snapshot' || dp === 'data-mover'
  const defaultVolumesToFsBackup = dp === 'filesystem'

  // v0.8.9: when L2 (export.enabled), tell the backend to create a
  // PAIR of Schedules with separate retention and BSLs:
  //   snapshot half → snapshotRetention into `snapshotTtl`, no BSL
  //                   override (uses default BSL or whatever)
  //   export half   → exportRetention into `exportTtl`, user-picked BSL
  // The backend sets snapshotMoveData based on role, NOT on our payload.
  // L1 mode (export disabled) still sends the legacy single-shape ttl
  // / storageLocation / snapshotMoveData fields.
  const payload = {
    name: form.name,
    schedule: cron,
    paused: !!form.paused,
    includedNamespaces: form.includedNamespaces.length > 0 ? form.includedNamespaces : undefined,
    snapshotVolumes,
    defaultVolumesToFsBackup,
    // v0.7 intent annotations (consumed by v0.9 self-managed scheduler).
    annotations: {
      'supkube.io/snapshot-schedule': form.snapshot.schedule,
      'supkube.io/snapshot-retention': form.snapshot.retention,
      'supkube.io/export-enabled': String(form.export.enabled),
      'supkube.io/export-schedule': form.export.schedule,
      'supkube.io/export-retention': form.export.retention,
      'supkube.io/data-path': dp,
      'supkube.io/volume-mode': dp === 'csi-snapshot' || dp === 'data-mover' ? 'csi' : (dp === 'filesystem' ? 'filesystem' : '')
    }
  }
  if (form.export.enabled) {
    // L2 — dual pair
    payload.dual = true
    payload.snapshotTtl = `${snapHours}h`
    payload.exportTtl = `${expHours}h`
    // v0.9.1.10 (#101 finding 1): empty → omit so the backend resolves the
    // effective cloud BSL (never silently send the nonexistent "default").
    payload.exportStorageLocation = form.export.storageLocation || undefined
    // snapshotStorageLocation left blank → backend resolves the default BSL
  } else {
    // L1 — single schedule, legacy field set
    payload.dual = false
    payload.ttl = `${snapHours}h`
    payload.storageLocation = form.export.storageLocation || undefined
    payload.snapshotMoveData = dp === 'data-mover'
  }
  return payload
}

// handleSubmit — dispatches to create-or-edit based on editMode.
// Single entry point keeps the drawer template trivial (one button,
// label changes via editMode), and lets us share the validation block.
const handleSubmit = async () => {
  if (!createForm.value.name) {
    ElMessage.warning('Please enter a policy name')
    return
  }
  // Agent D: Import Policy 走完全独立的提交路径，绕开 Snapshot 那套
  // schedule/dataPath/csi 校验。Edit 模式不支持 Import（actionType 只在
  // create 时露出，edit 时 v-if 隐藏）。
  if (createForm.value.actionType === 'import' && !editMode.value) {
    return submitImportPolicy()
  }
  if (!createForm.value.snapshot.schedule) {
    ElMessage.warning('Please set a Snapshot schedule')
    return
  }
  // v0.7-policy-2 global block: if admin set "Block snapshot-only" in
  // Settings and this policy has Export off, refuse to save. Applies to
  // edits too — re-saving a snapshot-only policy in a cluster that's been
  // tightened up should fail loudly.
  if (!createForm.value.export.enabled &&
      localStorage.getItem('supkube.policy.blockSnapshotOnly') === 'true') {
    ElMessageBox.alert(
      'This cluster is configured to block snapshot-only policies. Enable Export to proceed, or change the setting in Settings → Data Protection Policy.',
      'Snapshot-only policies are blocked',
      { type: 'error', confirmButtonText: 'Got it' }
    )
    return
  }
  creating.value = true
  try {
    const veleroPayload = collapseToVelero(createForm.value)
    if (editMode.value) {
      // PATCH path. We don't send `name`; we send the editable subset
      // matching the backend PatchSchedule contract.
      const patchBody = {
        schedule: veleroPayload.schedule,
        // v0.8.7.6: paused must be sent on every PATCH so On Demand
        // and cadence-based policies can flip in both directions.
        // (PatchSchedule backend already supports `paused`, since v0.8.5.)
        paused: veleroPayload.paused,
        ttl: veleroPayload.ttl,
        includedNamespaces: veleroPayload.includedNamespaces || [],
        storageLocation: veleroPayload.storageLocation,
        snapshotVolumes: veleroPayload.snapshotVolumes,
        defaultVolumesToFsBackup: veleroPayload.defaultVolumesToFsBackup,
        // v0.8.7: thread snapshotMoveData through to the PATCH endpoint
        snapshotMoveData: veleroPayload.snapshotMoveData,
        annotations: veleroPayload.annotations,
        // v0.9.1.8 fix: L2 dual policies emit role-specific BSL/TTL via
        // collapseToVelero (snapshotTtl / exportTtl / exportStorageLocation),
        // NOT the legacy flat storageLocation. The PATCH body was dropping
        // them, so editing "云端存储位置" returned 200 but never changed the
        // export half's BSL (backend saw all-nil → kept old value). Thread
        // them through so the backend's role-specific apply() actually fires.
        snapshotTtl: veleroPayload.snapshotTtl,
        exportTtl: veleroPayload.exportTtl,
        snapshotStorageLocation: veleroPayload.snapshotStorageLocation,
        exportStorageLocation: veleroPayload.exportStorageLocation
      }
      await patchSchedule(editingName.value, patchBody)
      ElMessage.success(`Policy "${editingName.value}" updated`)
    } else {
      await createSchedule(veleroPayload)
      const mode = createForm.value.export.enabled ? 'Snapshot + Export' : 'Snapshot-only ⚠'
      ElMessage.success(`Policy "${createForm.value.name}" created (${mode})`)
    }
    showCreateDialog.value = false
    editMode.value = false
    editingName.value = ''
    createForm.value = defaultForm()
    await fetchSchedules()
  } catch (e) {
    const verb = editMode.value ? 'update' : 'create'
    ElMessage.error(`Failed to ${verb} policy: ` + (e.response?.data?.error || e.message))
  } finally {
    creating.value = false
  }
}
// Backward-compat alias: any external @click handler still works.
const handleCreate = handleSubmit

// openCreateDrawer — entry point for the "+ Create Policy" button. Resets
// any leftover edit state so opening Create after editing doesn't carry
// the previous policy's form values.
const openCreateDrawer = () => {
  editMode.value = false
  editingName.value = ''
  createForm.value = defaultForm()
  // v0.8.7.4: refresh BSL list on each open so newly-created Storage
  // Profiles show up without a page reload.
  fetchStorageLocations()
  showCreateDialog.value = true
}

// onDrawerClose — fires whether the drawer was closed via X, ESC, or
// clicking outside. Cancel and Save handle reset themselves; this is the
// safety net so the next open is always clean.
const onDrawerClose = () => {
  editMode.value = false
  editingName.value = ''
}

const togglePause = async (row) => {
  const newPaused = !row.spec?.paused
  try {
    await patchSchedule(row.metadata.name, { paused: newPaused })
    ElMessage.success(`Policy "${row.metadata.name}" ${newPaused ? 'paused' : 'resumed'}`)
    await fetchSchedules()
  } catch (e) {
    ElMessage.error('Failed to update policy: ' + (e.response?.data?.error || e.message))
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete policy "${row.metadata.name}"?`,
      'Delete Policy',
      { confirmButtonText: 'Delete', cancelButtonText: 'Cancel', type: 'warning' }
    )
    await deleteSchedule(row.metadata.name)
    ElMessage.success(`Policy "${row.metadata.name}" deleted`)
    await fetchSchedules()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete policy')
    }
  }
}

onMounted(() => {
  fetchSchedules()
  fetchNamespaces()
  fetchStorageLocations()
  // v0.9.1.10 (#108): deep-link from Applications → kebab "Backup" /
  // "创建新策略". Open the Create-Policy wizard pre-filled with the app's
  // namespace so "back up this app" lands the user directly on policy
  // creation, not a bare list.
  //
  // v0.9.1.11 (testing 20260531 #3): Mars reported the drawer wasn't
  // auto-opening and the policy name wasn't being pre-filled per app.
  // Root cause for the name field: handler only set includedNamespaces.
  // Fix: also derive a sane default name (`<ns>-policy`) so the user
  // can edit-and-create in one step. The drawer-open behavior itself
  // was correct in the existing handler; the auto-fill name was the
  // missing piece causing the "broken" impression.
  //
  // After consuming the intent, replace the route to drop the query so
  // a hard refresh on /policies doesn't re-open the drawer on top of
  // whatever the user was doing.
  if (route.query.intent === 'create') {
    openCreateDrawer() // resets createForm to defaults first
    if (route.query.namespace) {
      const ns = String(route.query.namespace)
      createForm.value.includedNamespaces = [ns]
      // Default name: <ns>-policy. User can rename before submit; the
      // backend rejects duplicates so collisions surface immediately.
      createForm.value.name = `${ns}-policy`
    }
    // Clear the query so a refresh / back-nav doesn't replay the intent.
    router.replace({ path: '/policies' })
  }
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.page-header { align-items: flex-start; }
.page-header-text { flex: 1; }
.page-header h3 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
}
.page-desc {
  margin: 0;
  color: #909399;
  font-size: 13px;
  max-width: 880px;
  line-height: 1.5;
}

/* Kasten-style toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0;
}
.filter-action, .filter-freq { width: 180px; }
.filter-name { width: 260px; }
.filter-spacer { flex: 1; }
.filter-summary { font-size: 13px; color: #606266; }

/* Table cells */
/* v0.8.10.5: Policy Name cell carries name + chip row + ns row, all
   in one stacked column. Matches the Restore Points compression. */
.policy-cell {
  display: flex;
  flex-direction: column;
  gap: var(--sk-space-xs);
  padding: 4px 0;
}
.policy-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--sk-text);
}
.policy-cell-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-xs);
  align-items: center;
}
.policy-ns-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-xs);
  margin-top: 2px;
}
/* v0.8.10.6: vertical Resources column — each namespace on its own row.
   Per user instruction "each NS on a row, not stacked horizontally". */
.policy-ns-col {
  display: flex;
  flex-direction: column;
  gap: var(--sk-space-xs);
  align-items: flex-start;
  padding: 4px 0;
}
/* Namespace chip — neutral gray; no emoji per UI_GUIDELINES §3.1. */
.policy-ns-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  font-size: 11px;
  font-weight: 500;
  color: var(--sk-text-secondary);
}
/* v0.9.1.8: Storage Location column — local → cloud BSL chips. */
.policy-bsl-col {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.policy-bsl-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 500;
  font-family: 'SF Mono', Menlo, monospace;
  border: 1px solid var(--sk-border);
}
.policy-bsl-chip.bsl-local {
  background: var(--sk-bg-soft);
  color: var(--sk-text-caption);
}
.policy-bsl-chip.bsl-cloud {
  background: var(--sk-info-soft, rgba(106,166,255,.12));
  border-color: var(--sk-info, #6aa6ff);
  color: var(--sk-info, #6aa6ff);
}
.bsl-arrow { color: var(--sk-text-caption); font-size: 11px; }
/* Legacy classes kept for any remaining bindings (validation table
   cell removed, but other surfaces still reference these). */
.validation-cell { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; }
.validation-valid { color: var(--sk-status-success); }
.validation-invalid { color: var(--sk-status-warning); }
.ns-chip {
  background: var(--sk-bg-soft) !important;
  border-color: var(--sk-border) !important;
  color: var(--sk-text-secondary) !important;
  font-size: 11px;
  font-weight: 500;
  margin-right: 4px;
}
.action-text { font-size: 13px; color: var(--sk-text-secondary); }
.freq-cell { display: flex; flex-direction: column; gap: 2px; }
.freq-human { font-size: 13px; color: var(--sk-text-secondary); font-weight: 500; }
.freq-cron {
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 11px;
  color: var(--sk-text-caption);
  background: transparent;
  padding: 0;
}
/* v0.8.10.5: two-line date/time stack for narrow time columns. */
.stacked-time {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}
.stacked-time .sk-body    { font-weight: 500; }
.stacked-time .sk-caption { font-size: 11px; }
.muted { color: var(--sk-text-placeholder); font-size: 13px; }

/* v0.8.10.1: Restore Points count cell — same link semantics as the
   Applications page's RP count cell so a user moving between the two
   surfaces sees identical visual affordances. All colours via tokens —
   one place to retune the entire clickable palette. */
.rp-count {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  font-weight: 500;
}
.rp-count .el-icon { font-size: 14px; }
.rp-count-link {
  color: var(--sk-primary);
  cursor: pointer;
  padding: 3px 8px;
  margin: -3px -8px;
  border-radius: 4px;
  transition: background 120ms ease, color 120ms ease;
}
.rp-count-link:hover {
  text-decoration: underline;
  background: var(--sk-primary-bg-hover);
  color: var(--sk-primary-hover);
}
.rp-count-link:active {
  background: var(--sk-primary-active);
}
.rp-count-zero {
  color: var(--sk-text-placeholder);
  font-weight: 400;
  cursor: default;
}

/* Kebab */
.more-btn { padding: 4px 8px; font-size: 18px; color: #606266; }
.dots { font-size: 20px; line-height: 1; letter-spacing: 1px; }
:deep(.el-table__row:hover) .more-btn { color: #409eff; }

/* Drawers */
.view-body { padding: 0 4px; }
.view-section { margin-bottom: 16px; }

/* ════════════════════════════════════════════════════════════════════
 * v0.8.10.2: Policy Details drawer follows UI_GUIDELINES §5 — same
 * sk-* class set as ActionDetailDrawer + Applications drawer.
 * View drawer ONLY (not the Edit/New drawers, which are forms).
 * ════════════════════════════════════════════════════════════════════ */
/* Scope the flex body override to the View drawer ONLY — applying it
   globally to every el-drawer in this file (Edit / New / YAML) broke
   those because their content doesn't use sk-drawer + sk-drawer-footer
   flex children, so the body collapsed to 0 height. v0.8.10.5 fix. */
:deep(.sk-policy-view-drawer .el-drawer__body) {
  padding: 0;
  display: flex;
  flex-direction: column;
}
.sk-drawer {
  flex: 1 1 auto;
  overflow-y: auto;
  padding: 28px var(--sk-drawer-padding-x) var(--sk-drawer-section-spacing);
  position: relative;
  min-height: 0;
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
}
.sk-drawer-close:hover {
  background: var(--sk-bg-hover);
  color: var(--sk-text);
}
.sk-drawer-header { margin-bottom: var(--sk-drawer-header-spacing); }
.sk-drawer-subject {
  margin-top: var(--sk-space-xs);
  margin-bottom: var(--sk-space-md);
  word-break: break-all;
}
.sk-drawer-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sk-space-sm);
  align-items: center;
}
.sk-section {
  margin-bottom: var(--sk-drawer-section-spacing);
  padding-bottom: var(--sk-drawer-section-spacing);
  border-bottom: 1px solid var(--sk-border);
}
.sk-section:last-of-type { border-bottom: 0; padding-bottom: 0; }
.sk-section .sk-h3 { margin-bottom: var(--sk-space-md); }
.sk-fields {
  display: grid;
  grid-template-columns: 150px 1fr;
  row-gap: var(--sk-space-sm);
  column-gap: var(--sk-space-md);
  align-items: baseline;
}
.sk-field-value {
  word-break: break-all;
}
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

/* Kasten-style New Policy drawer */
:deep(.new-policy-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 18px 24px;
  border-bottom: 1px solid #ebeef5;
}
:deep(.new-policy-drawer .el-drawer__title) {
  font-size: 18px;
  font-weight: 700;
  color: #1f2329;
  text-align: center;
  width: 100%;
}
:deep(.new-policy-drawer .el-drawer__body) {
  padding: 20px 24px;
}
.kasten-form .el-form-item {
  margin-bottom: 22px;
}
.kasten-label-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.4;
}
.kasten-label-block strong {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.kasten-label-help {
  font-size: 12px;
  color: #909399;
  font-weight: 400;
}
.kasten-pill-group {
  display: flex;
  gap: 0;
  width: 100%;
}
.kasten-pill-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.kasten-pill {
  flex: 1;
  padding: 10px 14px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  cursor: pointer;
  transition: all 0.15s;
}
.kasten-pill-group .kasten-pill:first-child {
  border-radius: 6px 0 0 6px;
}
.kasten-pill-group .kasten-pill:last-child {
  border-radius: 0 6px 6px 0;
  border-left-width: 0;
}
.kasten-pill-grid .kasten-pill {
  border-radius: 6px;
}
.kasten-pill:hover:not(.is-active) {
  border-color: #c0c4cc;
  background: #f5f7fa;
}
.kasten-pill.is-active {
  background: #4f46e5;
  border-color: #4f46e5;
  color: #ffffff;
}
/* v0.9.1.12 PRD-009: Snapshot "always-on" pill — same indigo as active
   but with default cursor + slight desaturation so users understand it's
   informational, not a clickable toggle. (Kasten K10 model: Snapshot is
   mandatory; user only chooses whether to also export.) */
.kasten-pill.is-locked {
  cursor: default;
  border-radius: 6px;
  flex: 0 0 auto;
  padding: 8px 14px;
}
.snapshot-always-on {
  display: flex;
  align-items: center;
  gap: 10px;
}
.snapshot-always-on-note {
  font-size: 12px;
  color: var(--sk-text-caption, #909399);
}
/* v0.8.7: data-path pills — slightly smaller text so 4-up fits on
   the 560px-wide drawer without wrapping. Hover gets a subtle title
   so the tooltip is reachable even without screen-reader mode. */
.kasten-pill.data-path-pill {
  font-size: 13px;
  padding: 10px 12px;
  text-align: center;
}
.kasten-pill.data-path-pill:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
/* v0.8.7.5: Data Path 2-column layout makes Snapshot-only vs
   Snapshot+Export visually distinct. The Action toggle (L1/L2)
   above this widget determines which column is interactive. */
.data-path-2col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.data-path-col {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 12px;
  background: #fff;
  transition: opacity 0.15s, border-color 0.15s, box-shadow 0.15s;
}
.data-path-col.is-active-col {
  border-color: #4f46e5;
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.08);
}
.data-path-col.is-disabled-col {
  opacity: 0.55;
  background: #fafafa;
}
.data-path-col-head {
  display: flex;
  flex-direction: column;
  margin-bottom: 10px;
}
.data-path-col-title {
  font-size: 13px;
  font-weight: 600;
  color: #1d2129;
}
.data-path-col-sub {
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
  font-family: 'SF Mono', Menlo, monospace;
}
.data-path-col-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.data-path-col-body .kasten-pill {
  width: 100%;
}
.data-path-prereq {
  margin: 10px 0 0 0;
  padding: 8px 12px;
  background: #ecf5ff;
  border-left: 3px solid #409eff;
  border-radius: 4px;
  font-size: 12px;
  color: #1f4e8c;
  line-height: 1.5;
}
/* v0.8.7.6: On Demand notice — explains the paused-schedule semantics.
   Yellow theme (vs blue for prereqs) because it's an attention/state
   advisory rather than a "you might need this dependency" tip. */
.ondemand-notice {
  margin: 10px 0 0 0;
  padding: 8px 12px;
  background: #fdf6e3;
  border-left: 3px solid #e6a23c;
  border-radius: 4px;
  font-size: 12px;
  color: #7a5b00;
  line-height: 1.5;
}
/* v0.8.7.4: small "default" badge next to the cluster-default BSL in
   the Storage Profile dropdown. Velero's default=true BSL is what
   gets used when a Backup spec omits storageLocation; signaling it
   here helps users pick "the same as kubectl velero". */
.bsl-default-badge {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: 8px;
  background: #ecf5ff;
  color: #409eff;
  font-size: 11px;
  font-weight: 600;
}
/* form-hint reused under the Storage Profile dropdown when the BSL
   list is empty. Matches the StorageLocations.vue style. */
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
  margin-top: 6px;
}
/* v0.8.12 LBS3: Object Lock warning + OK variants of form-hint. */
.form-hint-warn {
  color: #d97706;
  background: #fffbeb;
  border-left: 3px solid #f59e0b;
  padding: 6px 10px;
  border-radius: 0 4px 4px 0;
}
.form-hint-ok {
  color: #059669;
  background: #f0fdf4;
  border-left: 3px solid #10b981;
  padding: 6px 10px;
  border-radius: 0 4px 4px 0;
}

/* v0.8.12 LBS4: 3-2-1-1-0 score preview strip — sits above the
   Cancel/Create buttons so it's the last thing the user reads. Same
   chip vocabulary as the Dashboard DR Topology score so the metric
   stays consistent across pages. */
.policy-score-strip {
  background: linear-gradient(90deg, #f8fafc 0%, #fff 100%);
  border: 1px solid var(--sk-border-light, #e5e7eb);
  border-radius: 8px;
  padding: 10px 14px;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}
.pss-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-shrink: 0;
}
.pss-label {
  font-size: 11px;
  letter-spacing: 0.5px;
  color: var(--sk-text-caption, #9ca3af);
  text-transform: uppercase;
}
.pss-count {
  font-size: 18px;
  font-weight: 700;
  color: var(--sk-primary, #4f46e5);
}
.pss-hint {
  font-size: 11px;
  color: var(--sk-text-caption, #9ca3af);
}
.pss-dots {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  flex: 1;
}
.pss-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  cursor: help;
}
.pss-dot { font-size: 14px; line-height: 1; }
.pss-item.is-ok .pss-dot { color: #10b981; }
.pss-item.is-bad .pss-dot { color: #d1d5db; }
.pss-item.is-bad .pss-rule { color: var(--sk-text-caption, #9ca3af); }
.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 24px;
  border-top: 1px solid #ebeef5;
}

.yaml-block {
  background: #1a1a2e;
  color: #a8d8a8;
  padding: 16px;
  border-radius: 6px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
  overflow-x: auto;
  max-height: calc(100vh - 200px);
}

/* Action blocks in Create Policy dialog (v0.7 Actions model) */
.action-block {
  margin-top: 20px;
  padding: 14px 16px 4px;
  border-radius: 8px;
  border: 1px solid #ebeef5;
  background: #fafbfc;
  transition: opacity 0.15s ease, background 0.15s ease;
}
.action-block.is-disabled {
  background: #fef0f0;
  border-color: #fbc4c4;
}
.snapshot-block {
  border-left: 3px solid #409eff;
}
.export-block {
  border-left: 3px solid #67c23a;
}
.export-block.is-disabled {
  border-left-color: #e6a23c;
}
.action-block-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #ebeef5;
}
.action-block-icon { font-size: 20px; }
.action-block-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.action-block-subtitle {
  font-size: 11px;
  color: #909399;
  font-weight: 500;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.action-disabled-warning {
  margin: 0 0 12px 0;
  padding: 10px 12px;
  font-size: 12px;
  color: #5b2929;
  line-height: 1.6;
}
.form-hint {
  display: block;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

/* Capability detection result (v0.7.1) */
.capability-result { font-size: 13px; }
.capability-error {
  padding: 8px 12px;
  background: #fef0f0;
  color: #5b2929;
  border-radius: 4px;
}
.capability-row {
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
  margin-bottom: 6px;
}
.capability-ns {
  display: flex;
  align-items: center;
  gap: 8px;
}
.capability-ns-name {
  font-weight: 600;
  color: #303133;
}
.capability-pvc-list {
  margin: 6px 0 0 0;
  padding-left: 20px;
  font-size: 12px;
  color: #606266;
  line-height: 1.7;
}
.capability-pvc-list code {
  background: #ecf0f5;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 11px;
}
.capability-reason { color: #f56c6c; font-style: italic; }
.capability-blocker {
  margin-top: 8px;
  padding: 10px 12px;
  background: #fef0f0;
  border: 1px solid #fbc4c4;
  border-radius: 4px;
  color: #5b2929;
  font-size: 13px;
}

/* Protection Level badges (v0.7-policy-2) */
.protection-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
}
.protection-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  font-size: 11px;
  font-weight: bold;
}
.protection-l1 {
  background: #fdf6ec;
  color: #b54708;
}
.protection-l1 .protection-icon {
  border: 1.5px solid #e6a23c;
  color: #e6a23c;
  background: #ffffff;
}
.protection-l2 {
  background: #f0f9eb;
  color: #225a17;
}
.protection-l2 .protection-icon {
  border: 1.5px solid #67c23a;
  color: #67c23a;
  background: #ffffff;
}
.protection-l3 {
  background: #ecf5ff;
  color: #1d3a8a;
}
.protection-l3 .protection-icon {
  border: 1.5px solid #409eff;
  color: #409eff;
  background: #ffffff;
}

/* ─── Agent D 2026-06-01: Import Policy 专属样式 ───────────────────── */
/* Import chip 复用全局 .sk-chip-type-imported（tokens.css 已有）。 */
/* Continuous Poll Interval pill 4列布局 / Cron 预设 3列布局 — 复用
   .kasten-pill-grid 但 grid-template-columns 由变体覆盖。 */
.kasten-pill-grid.pill-grid-3 { grid-template-columns: repeat(3, 1fr); }
.kasten-pill-grid.pill-grid-4 { grid-template-columns: repeat(4, 1fr); }
/* RPO 估计提示条 — 比 prereq/notice 更醒目（紫色调代表 SLO 维度）。 */
.import-rpo-banner {
  background: #f4ecfd;
  border-left: 3px solid #722ed1;
  border-radius: 4px;
  padding: 10px 14px;
}
.import-rpo-line {
  font-size: 13px;
  font-weight: 600;
  color: #3a1d6e;
  line-height: 1.4;
}
.import-rpo-note {
  font-size: 11px;
  color: #6b4ba0;
  margin-top: 3px;
}
</style>
