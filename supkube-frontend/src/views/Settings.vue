<!--
  Settings (v0.8.5 step 3.5 + 4 refactor)
  ────────────────────────────────────────
  Tabbed shell:
    General      — existing locale / theme / Velero status / Policy switches
    我的权限      — current user's RBAC role + scope (everyone sees this tab)
    审计日志      — recent write operations + filters (admin only)
-->
<template>
  <div class="settings-page">
    <h3>{{ t('settings.title') }}</h3>

    <el-tabs v-model="activeTab" class="settings-tabs">
      <el-tab-pane :label="t('settings.general')" name="general">
        <!-- ───── Existing General settings ───── -->
        <el-card class="appearance-card" style="margin-bottom: 20px">
          <template #header>{{ t('settings.language') }} / {{ t('settings.theme') }}</template>
          <el-form label-width="160px" label-position="left">
            <el-form-item :label="t('settings.language')">
              <el-radio-group v-model="locale" @change="onLocaleChange">
                <el-radio-button
                  v-for="loc in SUPPORTED_LOCALES"
                  :key="loc.code"
                  :value="loc.code"
                >{{ loc.label }}</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('settings.theme')">
              <el-radio-group v-model="theme" @change="onThemeChange">
                <el-radio-button value="light">☀ {{ t('settings.light') }}</el-radio-button>
                <el-radio-button value="dark">☾ {{ t('settings.dark') }}</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-form>
        </el-card>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-card v-loading="loading">
              <template #header>Velero Status</template>
              <el-descriptions :column="1" border>
                <el-descriptions-item label="Status">
                  <el-tag :type="veleroStatus.connected ? 'success' : 'danger'">
                    {{ veleroStatus.connected ? 'Connected' : 'Disconnected' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="Version">{{ veleroStatus.version || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Namespace">{{ veleroStatus.namespace || 'velero' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card v-loading="loading">
              <template #header>Cluster Information</template>
              <el-descriptions :column="1" border>
                <el-descriptions-item label="Kubernetes Version">{{ clusterInfo.k8sVersion || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Nodes">{{ clusterInfo.nodes ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="Namespaces">{{ clusterInfo.namespaces ?? '-' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
            <el-card style="margin-top: 20px">
              <template #header>SupKube</template>
              <el-descriptions :column="1" border>
                <el-descriptions-item label="Version">{{ supkubeInfo.version || '-' }}</el-descriptions-item>
                <el-descriptions-item label="API Endpoint">{{ apiUrl }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>

        <el-card style="margin-top: 20px">
          <template #header>Data Protection Policy</template>
          <el-form label-width="320px" label-position="left">
            <el-form-item label="Block snapshot-only policies">
              <el-switch v-model="blockSnapshotOnly" @change="saveBlockSnapshotOnly" />
              <span class="form-hint">
                When enabled, the Create Policy form refuses to save policies that disable Export.
                Recommended for production clusters where every restore point must be durable.
              </span>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- ───── 我的权限 (My Access) ───── -->
      <el-tab-pane :label="t('settings.myAccess')" name="access">
        <MyAccessPanel />
      </el-tab-pane>

      <!-- ───── 审计日志 (Audit Log) — admin only ───── -->
      <el-tab-pane v-if="auth.isAdmin.value" :label="t('settings.auditLog')" name="audit">
        <AuditLogPanel />
      </el-tab-pane>

      <!-- ───── 集群健康 / Cluster Hygiene — admin only (v0.8.8) ───── -->
      <el-tab-pane v-if="auth.isAdmin.value" :label="t('settings.clusterHygiene')" name="hygiene">
        <ClusterHygienePanel />
      </el-tab-pane>

      <!-- ───── 品牌定制 / Branding — admin only (v0.8.11) ───── -->
      <el-tab-pane v-if="auth.isAdmin.value" :label="t('settings.branding')" name="branding">
        <BrandingPanel />
      </el-tab-pane>

      <!-- ───── 插件管理 / Plugins — admin only (v0.8.13) ───── -->
      <el-tab-pane v-if="auth.isAdmin.value" :label="t('settings.plugins')" name="plugins">
        <PluginsPanel />
      </el-tab-pane>

      <!-- ───── 集群管理 / Clusters — admin only (v0.9.0 MC1) ───── -->
      <el-tab-pane v-if="auth.isAdmin.value" :label="t('settings.clusters')" name="clusters">
        <ClustersPanel />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { SUPPORTED_LOCALES, setLocale } from '../i18n'
import { getStatus, getDashboardSummary } from '../api/velero'
// PRD-028: subpath-aware API base (== '/api/v1' at default basePath).
import { basePrefix } from '../basePath'
import { ElMessage } from 'element-plus'
import { useAuth } from '../composables/useAuth'
import MyAccessPanel from '../components/MyAccessPanel.vue'
import AuditLogPanel from '../components/AuditLogPanel.vue'
import ClusterHygienePanel from '../components/ClusterHygienePanel.vue'
import BrandingPanel from '../components/BrandingPanel.vue'
import PluginsPanel from '../components/PluginsPanel.vue'
import ClustersPanel from '../components/ClustersPanel.vue'

const { t, locale } = useI18n()
const auth = useAuth()
// v0.9.0 Mode Switcher hands off to Settings → Clusters via ?tab=clusters.
// Pre-seed activeTab from the query so the deep-link works.
import { useRoute } from 'vue-router'
const route = useRoute()
const activeTab = ref(route.query.tab || 'general')

const onLocaleChange = (code) => setLocale(code)

const THEME_KEY = 'supkube.theme'
const theme = ref(document.documentElement.classList.contains('dark') ? 'dark' : 'light')
const onThemeChange = (val) => {
  document.documentElement.classList.toggle('dark', val === 'dark')
  try { localStorage.setItem(THEME_KEY, val) } catch (_) {}
}

const loading = ref(false)
const veleroStatus = ref({ connected: false, version: '', namespace: '', plugins: [] })
const clusterInfo = ref({ k8sVersion: '', nodes: 0, namespaces: 0 })
const supkubeInfo = ref({ version: '' })
const apiUrl = import.meta.env.VITE_API_URL || (basePrefix + '/api/v1')

const BLOCK_KEY = 'supkube.policy.blockSnapshotOnly'
const blockSnapshotOnly = ref(localStorage.getItem(BLOCK_KEY) === 'true')
const saveBlockSnapshotOnly = (v) => {
  localStorage.setItem(BLOCK_KEY, String(v))
  ElMessage.success(v ? 'Snapshot-only policies are now blocked' : 'Snapshot-only policies are now allowed')
}

const fetchData = async () => {
  loading.value = true
  try {
    const [statusRes, summaryRes] = await Promise.all([
      getStatus(),
      getDashboardSummary().catch(() => null)
    ])
    const status = statusRes.data
    supkubeInfo.value.version = status.version || ''
    veleroStatus.value.connected = status.status === 'ok'
    veleroStatus.value.version = status.veleroVersion || ''
    veleroStatus.value.namespace = status.veleroNamespace || 'velero'
    if (summaryRes) {
      const summary = summaryRes.data
      clusterInfo.value.nodes = summary.cluster?.nodes ?? 0
      clusterInfo.value.namespaces = summary.cluster?.namespaces ?? 0
      clusterInfo.value.k8sVersion = summary.cluster?.k8sVersion || ''
    }
  } catch (e) {
    ElMessage.error('Failed to load settings data')
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.settings-page h3 { margin: 0 0 16px 0; }
.settings-tabs { background: transparent; }
.form-hint {
  display: block;
  font-size: 12px;
  color: var(--sk-text-caption);
  line-height: 1.5;
  margin-top: 6px;
  max-width: 620px;
}
</style>
