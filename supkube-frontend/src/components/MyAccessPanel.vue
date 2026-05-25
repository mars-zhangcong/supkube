<!--
  MyAccessPanel (v0.8.5 step 3.5)
  ────────────────────────────────
  Shown inside Settings → 我的权限 tab. Every authenticated user sees their
  own role + permission summary. Admins additionally see the full RBAC
  bindings table (read-only — modifications go through Helm values.yaml).

  Why a separate panel component (not inlined in Settings.vue): keeps
  Settings.vue from ballooning, and makes it natural to also surface this
  panel elsewhere (e.g. an "unauthorized" 403 page in v0.9).
-->
<template>
  <div class="my-access">
    <el-card class="info-card" style="margin-bottom: 16px">
      <template #header>{{ t('myAccess.currentUserTitle') }}</template>
      <el-descriptions :column="1" border>
        <el-descriptions-item :label="t('myAccess.email')">
          {{ user.email || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('myAccess.username')">
          {{ user.username || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('myAccess.groups')">
          <template v-if="user.groups && user.groups.length">
            <el-tag
              v-for="g in user.groups"
              :key="g"
              size="small"
              style="margin-right: 6px"
            >{{ g }}</el-tag>
          </template>
          <span v-else class="muted">{{ t('myAccess.noGroups') }}</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="info-card" style="margin-bottom: 16px">
      <template #header>
        <span>{{ t('myAccess.myRoleTitle') }}</span>
        <span class="role-chip" :class="`role-${user.role}`">{{ (user.role || 'unknown').toUpperCase() }}</span>
      </template>

      <div v-if="!auth.authEnabled.value || !rbacEnabled" class="role-disabled-banner">
        <el-icon><InfoFilled /></el-icon>
        <div>{{ t('myAccess.rbacDisabled') }}</div>
      </div>

      <div v-if="user.role === 'editor' && user.namespaceScope && user.namespaceScope.length" class="ns-scope">
        <div class="scope-title">{{ t('myAccess.allowedNamespaces') }}</div>
        <el-tag v-for="ns in user.namespaceScope" :key="ns" type="info" style="margin-right: 6px">{{ ns }}</el-tag>
      </div>
      <div v-else-if="user.role === 'editor'" class="role-disabled-banner">
        <el-icon><WarningFilled /></el-icon>
        <div>{{ t('myAccess.editorNoNamespaces') }}</div>
      </div>

      <div class="caps-title">{{ t('myAccess.capabilitiesTitle') }}</div>
      <ul class="caps-list">
        <li v-for="c in roleCapabilities" :key="c.key">
          <el-icon class="cap-icon" :class="c.allowed ? 'ok' : 'no'">
            <component :is="c.allowed ? 'CircleCheckFilled' : 'CircleCloseFilled'" />
          </el-icon>
          {{ c.label }}
        </li>
      </ul>
    </el-card>

    <!-- Admin-only: all bindings (read-only audit) -->
    <el-card v-if="auth.isAdmin.value" class="info-card" v-loading="loadingBindings">
      <template #header>
        <span>{{ t('myAccess.allBindingsTitle') }}</span>
        <span class="bindings-source">{{ t('myAccess.allBindingsSource') }}</span>
      </template>

      <div v-if="bindings.length === 0" class="empty">
        <el-icon><InfoFilled /></el-icon>
        <div>{{ t('myAccess.noBindings') }}</div>
      </div>

      <el-table v-else :data="bindings" stripe>
        <el-table-column :label="t('myAccess.bindingType')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.group ? 'primary' : ''">
              {{ row.group ? 'GROUP' : 'USER' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('myAccess.bindingSubject')">
          <template #default="{ row }">
            <code>{{ row.group || row.user }}</code>
          </template>
        </el-table-column>
        <el-table-column :label="t('myAccess.bindingRole')" width="120">
          <template #default="{ row }">
            <span class="role-chip" :class="`role-${row.role}`">{{ row.role.toUpperCase() }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('myAccess.bindingNamespaces')">
          <template #default="{ row }">
            <template v-if="row.namespaces && row.namespaces.length">
              <el-tag
                v-for="ns in row.namespaces"
                :key="ns"
                size="small"
                type="info"
                style="margin-right: 4px"
              >{{ ns }}</el-tag>
            </template>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
      </el-table>

      <el-alert
        type="info"
        :closable="false"
        style="margin-top: 12px"
      >
        <template #title>
          {{ t('myAccess.editHint') }}
          <code style="margin-left: 6px">auth.rbac.bindings</code>
          {{ t('myAccess.editHintCont') }}
        </template>
      </el-alert>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheckFilled, CircleCloseFilled, InfoFilled, WarningFilled } from '@element-plus/icons-vue'
import { useAuth } from '../composables/useAuth'
import { getRBACBindings, getAuthMe } from '../api/velero'

const { t } = useI18n()
const auth = useAuth()

// Make sure user info is fresh (auth.bootstrap may have run a while ago).
const user = ref(auth.user.value || {})
const rbacEnabled = ref(true)
const bindings = ref([])
const loadingBindings = ref(false)

// Permission summary — derived from current role. Mirrors the
// canDo() vocabulary so what's shown here matches what's enabled in the UI.
const roleCapabilities = computed(() => {
  const r = user.value.role
  if (r === 'admin') {
    return [
      { key: 'all', label: t('myAccess.cap.adminAll'), allowed: true }
    ]
  }
  if (r === 'editor') {
    return [
      { key: 'backup',  label: t('myAccess.cap.editorBackup'),  allowed: true },
      { key: 'restore', label: t('myAccess.cap.editorRestore'), allowed: true },
      { key: 'policy',  label: t('myAccess.cap.editorPolicy'),  allowed: true },
      { key: 'preflight', label: t('myAccess.cap.editorPreflight'), allowed: true },
      { key: 'storage', label: t('myAccess.cap.adminStorage'),  allowed: false },
      { key: 'ts',      label: t('myAccess.cap.adminTS'),       allowed: false },
      { key: 'ns',      label: t('myAccess.cap.adminNs'),       allowed: false },
      { key: 'audit',   label: t('myAccess.cap.adminAudit'),    allowed: false }
    ]
  }
  if (r === 'viewer') {
    return [
      { key: 'read',    label: t('myAccess.cap.viewerRead'),  allowed: true },
      { key: 'backup',  label: t('myAccess.cap.editorBackup'), allowed: false },
      { key: 'restore', label: t('myAccess.cap.editorRestore'), allowed: false },
      { key: 'policy',  label: t('myAccess.cap.editorPolicy'), allowed: false },
      { key: 'storage', label: t('myAccess.cap.adminStorage'), allowed: false },
      { key: 'audit',   label: t('myAccess.cap.adminAudit'),   allowed: false }
    ]
  }
  return [{ key: 'unknown', label: t('myAccess.cap.unknownRole'), allowed: false }]
})

async function refreshMe() {
  try {
    const res = await getAuthMe()
    user.value = res.data.user || {}
    rbacEnabled.value = !!res.data.rbacEnabled
  } catch {}
}

async function loadBindings() {
  if (!auth.isAdmin.value) return
  loadingBindings.value = true
  try {
    const res = await getRBACBindings()
    bindings.value = res.data.bindings || []
    rbacEnabled.value = !!res.data.enabled
  } catch (e) {
    console.error('Failed to load bindings:', e)
  } finally {
    loadingBindings.value = false
  }
}

onMounted(async () => {
  await refreshMe()
  await loadBindings()
})
</script>

<style scoped>
.my-access { padding: 4px 0; }

.role-chip {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: 10px;
  letter-spacing: 0.5px;
}
.role-admin  { background: #fef0f0; color: var(--sk-status-error); }
.role-editor { background: #fdf6ec; color: #b88230; }
.role-viewer { background: #f0f9eb; color: var(--sk-status-success); }
.role-unknown { background: #f4f4f5; color: var(--sk-text-caption); }

.role-disabled-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #fdf6ec;
  border: 1px solid #f5dab1;
  border-radius: 6px;
  padding: 10px 14px;
  color: #67430b;
  font-size: 13px;
  margin: 12px 0;
}
.role-disabled-banner .el-icon { color: var(--sk-status-warning); font-size: 18px; }

.ns-scope {
  margin: 12px 0;
}
.scope-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--sk-text-muted);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.caps-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--sk-text);
  margin: 16px 0 8px 0;
}
.caps-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.caps-list li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 13px;
  color: var(--sk-text-secondary);
}
.cap-icon.ok { color: var(--sk-status-success); }
.cap-icon.no { color: var(--sk-text-placeholder); }

.bindings-source {
  font-size: 11px;
  color: var(--sk-text-caption);
  font-weight: 400;
  margin-left: 10px;
}

.empty {
  text-align: center;
  padding: 24px;
  color: var(--sk-text-caption);
  font-size: 13px;
}
.empty .el-icon { font-size: 22px; display: block; margin: 0 auto 6px; }

.muted { color: var(--sk-text-placeholder); font-size: 13px; }
code {
  background: rgba(0,0,0,0.05);
  padding: 1px 6px;
  border-radius: 3px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12.5px;
}

html.dark .role-admin  { background: #2a1a1d; color: #f5b1b1; }
html.dark .role-editor { background: #2b1f0c; color: #f0c473; }
html.dark .role-viewer { background: #1a2c14; color: #c2e7b0; }
html.dark .role-disabled-banner { background: #2b1f0c; border-color: #6e5217; color: #f0c473; }
html.dark .caps-title { color: #e5eaf3; }
html.dark .caps-list li { color: #e5eaf3; }
html.dark code { background: rgba(255,255,255,0.1); }
</style>
