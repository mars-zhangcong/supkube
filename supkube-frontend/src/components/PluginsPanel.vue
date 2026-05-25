<!--
  PluginsPanel (v0.8.13 HC4)
  ───────────────────────────────────────────────────────────────────────
  Settings → Plugins tab. Lists the optional SupKube subcharts (Velero,
  Dex, MinIO Local Store) with current install state + the helm upgrade
  command to enable/disable each.

  Why "show the command" instead of "click to apply":
    - Backend pod ships without `helm` binary and runs as a scoped SA,
      not cluster-admin. Doing `kubectl exec ... helm upgrade ...` from
      the backend would require giving it cluster-admin, which we won't.
    - Enterprise customers want a CI/CD-traceable change, not a "some
      admin clicked a button at 03:14 AM" delta.
    - The helm command is one-line + copyable; that's good enough UX.
-->

<template>
  <div class="plugins-panel" v-loading="loading">
    <p class="sk-caption" style="margin: 0 0 16px 0">
      {{ t('plugins.intro') }}
    </p>

    <el-card
      v-for="p in plugins"
      :key="p.id"
      class="plugin-card"
      shadow="never"
    >
      <div class="pp-row">
        <div class="pp-left">
          <div class="pp-title">
            <span class="sk-h3" style="margin: 0">{{ p.displayName }}</span>
            <el-tag
              size="small"
              :type="p.installed ? 'success' : 'info'"
              effect="plain"
              style="margin-left: 8px"
            >
              {{ p.installed ? t('plugins.installed') : t('plugins.notInstalled') }}
            </el-tag>
            <el-tag v-if="p.required" size="small" type="warning" effect="plain" style="margin-left: 6px">
              {{ t('plugins.required') }}
            </el-tag>
          </div>
          <p class="pp-desc">{{ p.description }}</p>
        </div>
      </div>

      <!-- Enable / Disable command. We show the OPPOSITE of current state
           as the actionable command. -->
      <div class="pp-cmd-block">
        <div class="pp-cmd-label">
          {{ p.installed ? t('plugins.disableCmdLabel') : t('plugins.enableCmdLabel') }}
        </div>
        <div class="pp-cmd-row">
          <code class="pp-cmd">{{ p.installed ? (p.disableCmd || '—') : (p.enableCmd || '—') }}</code>
          <el-button
            v-if="p.installed ? p.disableCmd : p.enableCmd"
            size="small"
            @click="copyCmd(p.installed ? p.disableCmd : p.enableCmd)"
          >
            {{ t('plugins.copy') }}
          </el-button>
        </div>
        <p class="pp-cmd-note">
          {{ t('plugins.cmdNote') }}
        </p>
      </div>
    </el-card>

    <el-alert
      v-if="!plugins.length && !loading"
      type="info"
      :title="t('plugins.empty')"
      :closable="false"
      show-icon
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getPluginsStatus } from '../api/velero'

const { t } = useI18n()
const loading = ref(true)
const plugins = ref([])

async function fetchPlugins() {
  loading.value = true
  try {
    const res = await getPluginsStatus()
    plugins.value = res.data.items || []
  } catch (e) {
    console.warn('plugins fetch failed', e?.response?.status)
  } finally {
    loading.value = false
  }
}

function copyCmd(cmd) {
  if (!cmd) return
  navigator.clipboard?.writeText(cmd).then(
    () => ElMessage.success(t('plugins.copied')),
    () => ElMessage.error(t('plugins.copyFailed'))
  )
}

onMounted(fetchPlugins)
</script>

<style scoped>
.plugins-panel { padding: 4px; }
.plugin-card { margin-bottom: 16px; }
.pp-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}
.pp-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 6px;
}
.pp-desc {
  margin: 0;
  font-size: 13px;
  color: var(--sk-text-caption, #6b7280);
  line-height: 1.5;
  max-width: 720px;
}
.pp-cmd-block {
  margin-top: 14px;
  padding: 12px 14px;
  background: var(--sk-surface-muted, #f9fafb);
  border-radius: 6px;
  border-left: 3px solid var(--sk-primary, #4f46e5);
}
.pp-cmd-label {
  font-size: 11px;
  color: var(--sk-text-caption, #6b7280);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 6px;
}
.pp-cmd-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.pp-cmd {
  flex: 1;
  min-width: 0;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
  background: #fff;
  padding: 8px 12px;
  border: 1px solid var(--sk-border-light, #e5e7eb);
  border-radius: 4px;
  overflow-x: auto;
  white-space: nowrap;
  color: var(--sk-text, #1f2937);
}
.pp-cmd-note {
  margin: 8px 0 0 0;
  font-size: 11px;
  color: var(--sk-text-caption, #9ca3af);
}
</style>
