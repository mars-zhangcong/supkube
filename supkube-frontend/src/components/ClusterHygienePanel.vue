<!--
  ClusterHygienePanel (v0.8.8)
  ─────────────────────────────
  Settings tab panel — orphan resource garbage collection controls.

  The "orphan" problem: Velero v1.18's Data Mover intentionally sets
  deletionPolicy=Retain on intermediate VolumeSnapshotContents so they
  survive the upload phase. Velero never removes them after the parent
  Backup is deleted, so they accumulate as cluster + object-storage
  garbage. SupKube's GC runner sweeps these on a configurable interval.

  UX goals:
    1. Tell the user what we're doing and why (the "explain" block)
    2. Give them control: ON/OFF toggle + interval choice + manual run
    3. Show that something happened: last-run summary with counts
-->
<template>
  <el-card class="hygiene-card">
    <template #header>
      <div class="hygiene-header">
        <span>{{ t('settings.hygiene.title') }}</span>
        <el-tag v-if="settings.enabled" type="success" size="small">{{ t('settings.hygiene.statusOn') }}</el-tag>
        <el-tag v-else type="info" size="small">{{ t('settings.hygiene.statusOff') }}</el-tag>
      </div>
    </template>

    <!-- Why this exists (explain before configure) -->
    <div class="hygiene-explain">
      <p>{{ t('settings.hygiene.explain1') }}</p>
      <p>{{ t('settings.hygiene.explain2') }}</p>
    </div>

    <el-form label-position="left" label-width="220px" v-loading="loading">
      <el-form-item :label="t('settings.hygiene.enabled')">
        <el-switch
          v-model="settings.enabled"
          @change="saveSettings"
          :loading="saving"
        />
        <span class="hygiene-hint">{{
          settings.enabled
            ? t('settings.hygiene.enabledOn')
            : t('settings.hygiene.enabledOff')
        }}</span>
      </el-form-item>

      <el-form-item :label="t('settings.hygiene.interval')" v-if="settings.enabled">
        <el-select
          v-model="settings.intervalHours"
          @change="saveSettings"
          :disabled="saving"
          style="width: 160px"
        >
          <el-option :label="t('settings.hygiene.intervalOpt.1h')"  :value="1" />
          <el-option :label="t('settings.hygiene.intervalOpt.6h')"  :value="6" />
          <el-option :label="t('settings.hygiene.intervalOpt.12h')" :value="12" />
          <el-option :label="t('settings.hygiene.intervalOpt.24h')" :value="24" />
        </el-select>
        <span class="hygiene-hint">{{ t('settings.hygiene.intervalHint') }}</span>
      </el-form-item>

      <el-form-item :label="t('settings.hygiene.manual')">
        <el-button
          type="primary"
          :loading="running"
          @click="runManualCleanup"
        >
          {{ t('settings.hygiene.runNow') }}
        </el-button>
        <span class="hygiene-hint">{{ t('settings.hygiene.runNowHint') }}</span>
      </el-form-item>
    </el-form>

    <!-- Last run summary -->
    <div v-if="lastRun" class="hygiene-lastrun">
      <h4>{{ t('settings.hygiene.lastRun') }}</h4>
      <div class="hygiene-lastrun-grid">
        <div>
          <div class="hygiene-stat-label">{{ t('settings.hygiene.ran') }}</div>
          <div class="hygiene-stat-value">{{ formatRelative(lastRun.finishedAt) }}</div>
        </div>
        <div>
          <div class="hygiene-stat-label">VSC</div>
          <div class="hygiene-stat-value">{{ lastRun.vscDeleted }}</div>
        </div>
        <div>
          <div class="hygiene-stat-label">VS</div>
          <div class="hygiene-stat-value">{{ lastRun.vsDeleted }}</div>
        </div>
        <div>
          <div class="hygiene-stat-label">PodVolumeBackup</div>
          <div class="hygiene-stat-value">{{ lastRun.pvbDeleted }}</div>
        </div>
        <div>
          <div class="hygiene-stat-label">DataUpload</div>
          <div class="hygiene-stat-value">{{ lastRun.dataUploadDeleted }}</div>
        </div>
      </div>
      <p class="hygiene-summary-line">{{ lastRun.summary }}</p>
      <p v-if="lastRun.error" class="hygiene-error">⚠ {{ lastRun.error }}</p>
    </div>
    <div v-else class="hygiene-no-run">{{ t('settings.hygiene.noRunYet') }}</div>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getCleanupSettings, updateCleanupSettings, runOrphanCleanup } from '../api/velero'

const { t } = useI18n()

// Local form-bound state, hydrated from backend on mount.
const settings = ref({ enabled: true, intervalHours: 6 })
const lastRun = ref(null)
const loading = ref(false)
const saving = ref(false)
const running = ref(false)

// Format ISO-ish timestamp as "5 minutes ago" / "3 hours ago" / fall
// back to local-date string. Keeps the "last run" line readable.
function formatRelative(iso) {
  if (!iso) return '—'
  const t1 = new Date(iso).getTime()
  const delta = Math.floor((Date.now() - t1) / 1000)
  if (delta < 60) return `${delta}s ago`
  if (delta < 3600) return `${Math.floor(delta/60)}m ago`
  if (delta < 86400) return `${Math.floor(delta/3600)}h ago`
  return new Date(iso).toLocaleString()
}

const loadSettings = async () => {
  loading.value = true
  try {
    const res = await getCleanupSettings()
    settings.value.enabled = !!res.data.enabled
    settings.value.intervalHours = res.data.intervalHours || 6
    lastRun.value = res.data.lastRun || null
  } catch (e) {
    ElMessage.error(`${t('settings.hygiene.loadFailed')}: ${e.response?.data?.error || e.message}`)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    const res = await updateCleanupSettings({
      enabled: settings.value.enabled,
      intervalHours: settings.value.intervalHours
    })
    settings.value.enabled = !!res.data.enabled
    settings.value.intervalHours = res.data.intervalHours
    lastRun.value = res.data.lastRun || null
    ElMessage.success(t('settings.hygiene.saved'))
  } catch (e) {
    ElMessage.error(`${t('settings.hygiene.saveFailed')}: ${e.response?.data?.error || e.message}`)
    // Reload to revert UI state on error
    await loadSettings()
  } finally {
    saving.value = false
  }
}

const runManualCleanup = async () => {
  running.value = true
  try {
    const res = await runOrphanCleanup()
    lastRun.value = res.data.result
    ElMessage.success(res.data.summary || t('settings.hygiene.runDone'))
  } catch (e) {
    ElMessage.error(`${t('settings.hygiene.runFailed')}: ${e.response?.data?.error || e.message}`)
  } finally {
    running.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.hygiene-card { background: #fff; }
.hygiene-header {
  display: flex;
  align-items: center;
  gap: 10px;
}
.hygiene-explain {
  background: #f5f7fa;
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 16px;
  font-size: 13px;
  color: var(--sk-text-muted);
  line-height: 1.6;
}
.hygiene-explain p { margin: 0 0 6px 0; }
.hygiene-explain p:last-child { margin-bottom: 0; }
.hygiene-hint {
  margin-left: 12px;
  font-size: 12px;
  color: var(--sk-text-caption);
}
.hygiene-lastrun {
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid #ebeef5;
}
.hygiene-lastrun h4 {
  margin: 0 0 12px 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--sk-text);
}
.hygiene-lastrun-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
}
.hygiene-stat-label {
  font-size: 11px;
  color: var(--sk-text-caption);
  text-transform: uppercase;
  letter-spacing: 0.02em;
  margin-bottom: 4px;
}
.hygiene-stat-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--sk-text);
  font-family: 'SF Mono', Menlo, monospace;
}
.hygiene-summary-line {
  margin: 12px 0 0 0;
  font-size: 12px;
  color: var(--sk-text-muted);
  font-family: 'SF Mono', Menlo, monospace;
}
.hygiene-error {
  margin: 8px 0 0 0;
  color: var(--sk-status-error);
  font-size: 12px;
}
.hygiene-no-run {
  margin-top: 16px;
  font-size: 13px;
  color: var(--sk-text-caption);
  padding: 14px;
  background: #fafafa;
  border-radius: 6px;
  text-align: center;
}
</style>
