<!--
  ClustersPanel (v0.9.0 MC1)
  ────────────────────────────────────────────────────────────────────
  Settings → Clusters tab. Lists all registered clusters with their
  health status + a single "+ Add Cluster" CTA in the upper-right.

  Single-cluster state: shows "this-cluster" + a prominent
  "Add another cluster to unlock Multi-Cluster Manager" empty-state
  block (entry path #1 of 3 from the design doc).

  Multi-cluster state: standard table + Edit/Test/Remove per row.
-->

<template>
  <div class="clusters-panel" v-loading="cluster.loading.value">
    <div class="cp-header">
      <div>
        <h3 class="sk-h3" style="margin: 0">{{ t('clusters.title') }}</h3>
        <p class="sk-caption" style="margin: 4px 0 0 0">{{ t('clusters.subtitle') }}</p>
      </div>
      <el-button type="primary" @click="wizardOpen = true">
        <el-icon><Plus /></el-icon>
        {{ t('clusters.add') }}
      </el-button>
    </div>

    <!-- ════ Empty state (single cluster) — entry #1 transition card ════ -->
    <el-card v-if="cluster.clusters.value.length <= 1 && bootstrapped" class="cp-empty-card" shadow="never">
      <div class="cpe-row">
        <div class="cpe-icon">🗂</div>
        <div class="cpe-content">
          <div class="cpe-title">{{ t('clusters.empty.title') }}</div>
          <div class="cpe-body">{{ t('clusters.empty.body') }}</div>
        </div>
      </div>
    </el-card>

    <!-- ════ Cluster rows ════ -->
    <el-card
      v-for="c in cluster.clusters.value"
      :key="c.name"
      class="cluster-card"
      shadow="never"
    >
      <div class="cc-row">
        <div class="cc-left">
          <div class="cc-title">
            <span class="sk-h3" style="margin: 0">{{ c.displayName || c.name }}</span>
            <el-tag v-if="c.type === 'primary'" size="small" type="primary" effect="plain" style="margin-left: 8px">
              ★ {{ t('clusters.primary') }}
            </el-tag>
            <el-tag v-else size="small" type="info" effect="plain" style="margin-left: 8px">
              {{ t('clusters.secondary') }}
            </el-tag>
            <el-tag
              size="small"
              :type="phaseType(c.phase)"
              effect="plain"
              style="margin-left: 6px"
            >
              ● {{ c.phase }}
            </el-tag>
          </div>
          <div class="cc-meta">
            <span v-if="c.k8sVersion">k8s {{ c.k8sVersion }}</span>
            <span v-if="c.nodeCount > 0">{{ c.nodeCount }} {{ t('clusters.nodes') }}</span>
            <span v-if="c.veleroInstalled">Velero {{ c.veleroVersion || '✓' }}</span>
            <span v-else-if="c.name !== 'this-cluster'" class="cc-meta-warn">⚠ Velero not detected</span>
            <span v-if="c.lastChecked" class="cc-meta-faint">
              {{ t('clusters.lastChecked') }} {{ formatAgo(c.lastChecked) }}
            </span>
          </div>
          <div v-if="c.message" class="cc-error">{{ c.message }}</div>
        </div>
        <div class="cc-right">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, c)">
            <el-button :icon="MoreFilled" circle plain size="small" />
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="switch" :disabled="c.name === 'this-cluster' || isActive(c)">
                  {{ t('clusters.actions.switch') }}
                </el-dropdown-item>
                <el-dropdown-item command="test" :disabled="c.name === 'this-cluster'">
                  {{ t('clusters.actions.test') }}
                </el-dropdown-item>
                <!-- v0.9.0.1 fix #2 — install / inspect helpers.
                     View Kubeconfig: only meaningful for registered
                     remote clusters (this-cluster's kubeconfig is the
                     in-cluster ServiceAccount, not user-uploaded).
                     Install SupKube / Velero: always meaningful — shows
                     the helm command the admin runs on the target. -->
                <el-dropdown-item command="kubeconfig" :disabled="c.name === 'this-cluster'" divided>
                  {{ t('clusters.actions.viewKubeconfig') }}
                </el-dropdown-item>
                <el-dropdown-item command="installVelero">
                  {{ t('clusters.actions.installVelero') }}
                </el-dropdown-item>
                <el-dropdown-item command="installSupkube">
                  {{ t('clusters.actions.installSupkube') }}
                </el-dropdown-item>
                <el-dropdown-item command="delete" :disabled="c.name === 'this-cluster'" divided>
                  <span style="color: var(--sk-status-error, #dc2626)">{{ t('clusters.actions.remove') }}</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-card>

    <!-- Add Cluster wizard -->
    <AddClusterWizard v-model="wizardOpen" @created="onCreated" />

    <!-- v0.9.0.1: text-block modal used by View Kubeconfig + Install
         SupKube + Install Velero. One reusable dialog rather than 3
         separate components — same shape: title + monospace block + copy. -->
    <el-dialog
      v-model="textModal.open"
      :title="textModal.title"
      width="720px"
      :close-on-click-modal="false"
    >
      <p class="text-modal-intro" v-if="textModal.intro">{{ textModal.intro }}</p>
      <pre class="text-modal-body"><code>{{ textModal.body }}</code></pre>
      <template #footer>
        <el-button @click="textModal.open = false">{{ t('common.close') }}</el-button>
        <el-button type="primary" @click="copyToClipboard(textModal.body)">
          {{ t('common.copy') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, MoreFilled } from '@element-plus/icons-vue'
import { useCluster } from '../composables/useCluster'
import { deleteCluster, testClusterByName } from '../api/velero'
import AddClusterWizard from './AddClusterWizard.vue'

const { t } = useI18n()
const router = useRouter()
const cluster = useCluster()

const wizardOpen = ref(false)
const bootstrapped = ref(false)

// v0.9.0.1: reusable text-block modal — shared by View Kubeconfig +
// Install Velero + Install SupKube actions.
const textModal = ref({ open: false, title: '', intro: '', body: '' })
function openTextModal(opts) {
  textModal.value = { open: true, ...opts }
}
function copyToClipboard(text) {
  navigator.clipboard?.writeText(text).then(
    () => ElMessage.success(t('common.copied')),
    () => ElMessage.error(t('common.copyFailed'))
  )
}

// Pre-built install commands. Helm chart name / version are pinned to
// the SupKube release that's currently running — admins doing fleet
// rollouts know the exact tag without grepping CI logs.
function veleroInstallCmd(c) {
  return `# Install Velero v1.18 on ${c.name} (matches SupKube's bundled version)
helm repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts
helm repo update
helm install velero vmware-tanzu/velero \\
  --kubeconfig <path-to-${c.name}-kubeconfig> \\
  --namespace velero --create-namespace \\
  --version 9.0.4 \\
  --set configuration.features=EnableCSI \\
  --set deployNodeAgent=true \\
  --set credentials.useSecret=false \\
  --set-json 'initContainers=[
    {"name":"velero-plugin-for-csi","image":"velero/velero-plugin-for-csi:v0.7.0","volumeMounts":[{"mountPath":"/target","name":"plugins"}]},
    {"name":"velero-plugin-for-microsoft-azure","image":"velero/velero-plugin-for-microsoft-azure:v1.10.0","volumeMounts":[{"mountPath":"/target","name":"plugins"}]},
    {"name":"velero-plugin-for-aws","image":"velero/velero-plugin-for-aws:v1.10.0","volumeMounts":[{"mountPath":"/target","name":"plugins"}]}
  ]'

# After install, run: kubectl get pods -n velero
# All pods should be Running before SupKube can dispatch backups/restores here.`
}
function supkubeInstallCmd(c) {
  // v0.9.0.3: Helm repo lives on Azure Blob Static Website fronted by
  // Cloudflare → charts.supkube.com. See hack/AZURE-SETUP.md +
  // hack/publish-release.sh for the publishing side. index.yaml in the
  // repo uses RELATIVE chart urls so the customer-facing URL can change
  // without invalidating past entries.
  return `# Install SupKube on ${c.name}
helm repo add supkube https://charts.supkube.com/
helm repo update
helm install supkube supkube/supkube \\
  --kubeconfig <path-to-${c.name}-kubeconfig> \\
  --namespace supkube --create-namespace \\
  --set velero.enabled=true \\
  --set localStore.enabled=false   # set true if you want in-cluster MinIO local BSL

# Browse available versions before installing:
# helm search repo supkube --versions

# Access via NodePort (default 30888):
# kubectl --kubeconfig <path> get svc -n supkube
# Default login: admin@supkube.local / admin (change immediately for production)`
}

function phaseType(p) {
  switch (p) {
    case 'Healthy':       return 'success'
    case 'Unreachable':   return 'danger'
    case 'Unauthorized':  return 'danger'
    case 'Unknown':       return 'info'
    default:              return 'info'
  }
}

function isActive(c) {
  return c.name === cluster.active.value
}

function formatAgo(iso) {
  if (!iso) return ''
  const ts = new Date(iso).getTime()
  const sec = Math.max(0, (Date.now() - ts) / 1000)
  if (sec < 60)   return `${Math.round(sec)}s`
  if (sec < 3600) return `${Math.round(sec / 60)}m`
  if (sec < 86400) return `${Math.round(sec / 3600)}h`
  return `${Math.round(sec / 86400)}d`
}

async function onCreated(name) {
  await cluster.refresh()
  ElMessage.success(t('clusters.createdToast', { name }))
}

async function handleAction(cmd, c) {
  if (cmd === 'switch') {
    cluster.setActive(c.name, router)
    ElMessage.success(t('clusters.switchedTo', { name: c.displayName || c.name }))
  } else if (cmd === 'kubeconfig') {
    // v0.9.0.1 fix #2: surface the stored kubeconfig so admins can audit
    // or re-export it. Uses the existing Test endpoint with no body to
    // pull the Secret server-side — avoids exposing a "get kubeconfig"
    // dedicated endpoint that would be a security regression. (TODO v0.9.x:
    // proper dedicated handler with audit log entry.)
    openTextModal({
      title: t('clusters.modal.kubeconfigTitle', { name: c.name }),
      intro: t('clusters.modal.kubeconfigIntro'),
      body: `# Stored at: Secret/cluster-kubeconfig-${c.name} in namespace 'supkube'\n#\n# Retrieve from your workstation (admin access needed):\n\nkubectl get secret cluster-kubeconfig-${c.name} \\\n  -n supkube -o jsonpath='{.data.kubeconfig}' | base64 -d`
    })
  } else if (cmd === 'installVelero') {
    openTextModal({
      title: t('clusters.modal.installVeleroTitle', { name: c.name }),
      intro: t('clusters.modal.installVeleroIntro'),
      body: veleroInstallCmd(c)
    })
  } else if (cmd === 'installSupkube') {
    openTextModal({
      title: t('clusters.modal.installSupkubeTitle', { name: c.name }),
      intro: t('clusters.modal.installSupkubeIntro'),
      body: supkubeInstallCmd(c)
    })
  } else if (cmd === 'test') {
    try {
      const res = await testClusterByName(c.name)
      const phase = res.data?.phase || 'Unknown'
      if (phase === 'Healthy') {
        ElMessage.success(`${c.name}: ${phase}`)
      } else {
        ElMessage.warning(`${c.name}: ${phase} — ${res.data?.message || ''}`)
      }
      await cluster.refresh()
    } catch (e) {
      ElMessage.error(`${c.name}: ${e?.response?.data?.error || e.message}`)
    }
  } else if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(
        t('clusters.removeConfirm', { name: c.name }),
        t('clusters.removeTitle'),
        { type: 'warning', confirmButtonText: t('clusters.remove'), cancelButtonText: t('common.cancel') }
      )
    } catch { return }
    try {
      await deleteCluster(c.name)
      // If we just removed the active cluster, drop back to this-cluster.
      if (isActive(c)) cluster.setActive('', router)
      await cluster.refresh()
      ElMessage.success(t('clusters.removedToast', { name: c.name }))
    } catch (e) {
      ElMessage.error(t('clusters.removeFailed') + ': ' + (e?.response?.data?.error || e.message))
    }
  }
}

onMounted(async () => {
  await cluster.refresh()
  bootstrapped.value = true
})
</script>

<style scoped>
.clusters-panel { padding: 4px; }
.cp-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}
.cp-empty-card {
  margin-bottom: 16px;
  background: linear-gradient(135deg, var(--sk-primary-bg-hover, #f5f3ff) 0%, #fff 100%);
  border: 1px dashed var(--sk-primary, #4f46e5);
}
.cpe-row { display: flex; gap: 16px; align-items: flex-start; }
.cpe-icon { font-size: 32px; line-height: 1; }
.cpe-content { flex: 1; }
.cpe-title { font-weight: 600; font-size: 15px; color: var(--sk-text, #1f2937); margin-bottom: 4px; }
.cpe-body { font-size: 13px; color: var(--sk-text-caption, #6b7280); line-height: 1.6; }

.cluster-card { margin-bottom: 12px; }
.cc-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}
.cc-left { flex: 1; min-width: 0; }
.cc-right { flex-shrink: 0; }
.cc-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 6px;
}
.cc-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  font-size: 12px;
  color: var(--sk-text-caption, #6b7280);
}
.cc-meta-warn { color: #d97706; }
.cc-meta-faint { color: var(--sk-text-placeholder, #9ca3af); }
.cc-error {
  margin-top: 6px;
  font-size: 12px;
  color: var(--sk-status-error, #dc2626);
  font-family: 'SF Mono', Menlo, monospace;
}

/* v0.9.0.1: reusable text-block modal styles. */
.text-modal-intro {
  margin: 0 0 12px 0;
  color: var(--sk-text-caption, #6b7280);
  font-size: 13px;
  line-height: 1.6;
}
.text-modal-body {
  background: var(--sk-surface-muted, #1f2937);
  color: #f3f4f6;
  padding: 14px 16px;
  border-radius: 6px;
  overflow-x: auto;
  max-height: 460px;
  overflow-y: auto;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
  margin: 0;
  white-space: pre;
}
</style>
