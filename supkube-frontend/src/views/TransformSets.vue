<!--
  TransformSets (v0.8.2)
  ──────────────────────
  Management UI for the Velero ResourceModifier ConfigMaps that SupKube
  treats as named "Transform Sets". A Transform Set is a bundle of JSONPath
  patches applied during restore — used to fix structural conflicts when
  restoring across namespaces or clusters (see USER_MANUAL.md §4.2).

  Page layout:
    - Top description + Create button
    - Filter row (search by name)
    - Card grid: each Transform Set as a card with name + description +
      rule count + Built-In badge + ⋮ menu (Edit / Delete / Clone)
    - Side drawer for create / edit (visual rule editor — no raw YAML
      required, but YAML preview at bottom for power users)

  Built-in templates ship with the install (strip-nodeport,
  strip-clusterip, strip-loadbalancer-ip, strip-pv-binding). They're
  read-only; users must Clone to customize.
-->
<template>
  <div class="ts-page">
    <div class="page-header">
      <div>
        <h3>{{ t('transformSets.title') }}</h3>
        <p class="page-desc">{{ t('transformSets.desc') }}</p>
      </div>
      <el-button type="primary"
        :disabled="!auth.canDo('transformset.crud')"
        :title="!auth.canDo('transformset.crud') ? t('common.noPermission') : ''"
        @click="openEditor(null)">
        <el-icon><Plus /></el-icon> {{ t('transformSets.create') }}
      </el-button>
    </div>

    <div class="filter-toolbar">
      <el-input
        v-model="filter"
        :placeholder="t('transformSets.filterPlaceholder')"
        clearable
        class="filter-input"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="spacer"></span>
      <span class="result-count">{{ t('transformSets.count', { n: filtered.length, total: items.length }) }}</span>
    </div>

    <div v-if="loading && items.length === 0" class="loading">
      <el-icon class="rot"><Loading /></el-icon> {{ t('common.loading') }}
    </div>
    <div v-else-if="filtered.length === 0" class="empty">
      <el-icon><InfoFilled /></el-icon>
      <div>{{ t('transformSets.empty') }}</div>
    </div>
    <div v-else class="ts-grid">
      <div v-for="ts in filtered" :key="ts.name" class="ts-card">
        <div class="ts-card-head">
          <div class="ts-name">
            <el-icon><Tools /></el-icon>
            <span>{{ ts.name }}</span>
            <span v-if="ts.builtIn" class="builtin-pill">{{ t('transformSets.builtin') }}</span>
          </div>
          <el-dropdown trigger="click" @command="cmd => handleCommand(cmd, ts)">
            <button class="more-btn" type="button"><span class="dots">⋮</span></button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="view">{{ t('common.view') }}</el-dropdown-item>
                <el-dropdown-item command="edit" :disabled="ts.builtIn">{{ t('common.edit') }}</el-dropdown-item>
                <el-dropdown-item command="clone">{{ t('transformSets.clone') }}</el-dropdown-item>
                <el-dropdown-item command="delete" divided :disabled="ts.builtIn">{{ t('common.delete') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="ts-desc">{{ ts.description || t('transformSets.noDesc') }}</div>
        <div class="ts-meta">
          <span class="meta-chip">{{ t('transformSets.ruleCount', { n: (ts.rules || []).length }) }}</span>
          <span class="meta-chip">{{ ts.rules?.reduce((s, r) => s + (r.patches?.length || 0), 0) || 0 }} {{ t('transformSets.patches') }}</span>
        </div>
        <div class="ts-rules-preview">
          <div v-for="(rule, ri) in (ts.rules || []).slice(0, 2)" :key="ri" class="rule-mini">
            <span class="rule-gr">{{ rule.conditions.groupResource }}</span>
            <span class="rule-arrow">→</span>
            <span v-for="(p, pi) in rule.patches" :key="pi" class="rule-op">
              {{ p.operation }} <code>{{ p.path }}</code>
            </span>
          </div>
          <div v-if="(ts.rules || []).length > 2" class="rule-more">+{{ ts.rules.length - 2 }} more rule(s)</div>
        </div>
      </div>
    </div>

    <!-- Editor / Viewer drawer -->
    <el-drawer
      v-model="drawerOpen"
      :title="editorTitle"
      direction="rtl"
      size="640px"
      :close-on-click-modal="false"
    >
      <div v-if="form" class="editor-body">
        <el-form :model="form" label-position="top">
          <el-form-item :label="t('common.name')" required>
            <el-input v-model="form.name" :disabled="!!editingName" placeholder="my-transform" />
          </el-form-item>
          <el-form-item :label="t('transformSets.description')">
            <el-input v-model="form.description" type="textarea" :rows="2"
              :placeholder="t('transformSets.descriptionPlaceholder')" />
          </el-form-item>

          <div class="rules-section">
            <div class="rules-header">
              <span class="rules-title">{{ t('transformSets.rules') }}</span>
              <el-button size="small" @click="addRule">
                <el-icon><Plus /></el-icon> {{ t('transformSets.addRule') }}
              </el-button>
            </div>

            <div v-for="(rule, ri) in form.rules" :key="ri" class="rule-editor">
              <div class="rule-editor-head">
                <span class="rule-num">#{{ ri + 1 }}</span>
                <el-button size="small" type="danger" circle @click="removeRule(ri)" :disabled="form.rules.length <= 1">
                  <el-icon><Close /></el-icon>
                </el-button>
              </div>

              <el-form-item :label="t('transformSets.groupResource')" required>
                <el-input v-model="rule.conditions.groupResource" placeholder="services, deployments.apps, persistentvolumeclaims" />
              </el-form-item>
              <el-form-item :label="t('transformSets.namespaces')">
                <el-select v-model="rule.conditions.namespaces" multiple filterable allow-create
                  :placeholder="t('transformSets.allNamespaces')">
                  <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('transformSets.resourceNameRegex')">
                <el-input v-model="rule.conditions.resourceNameRegex" :placeholder="t('transformSets.resourceNameRegexHint')" />
              </el-form-item>

              <div class="patches-section">
                <div class="patches-head">
                  <span>{{ t('transformSets.patches') }}</span>
                  <el-button size="small" @click="addPatch(rule)">
                    <el-icon><Plus /></el-icon> {{ t('transformSets.addPatch') }}
                  </el-button>
                </div>
                <div v-for="(patch, pi) in rule.patches" :key="pi" class="patch-row">
                  <el-select v-model="patch.operation" class="patch-op">
                    <el-option v-for="op in ['add','remove','replace','test','copy','move']" :key="op" :label="op" :value="op" />
                  </el-select>
                  <el-input v-model="patch.path" :placeholder="t('transformSets.pathPlaceholder')" class="patch-path" />
                  <el-input
                    v-if="['add','replace','test'].includes(patch.operation)"
                    v-model="patch.value"
                    :placeholder="t('transformSets.valuePlaceholder')"
                    class="patch-value"
                  />
                  <el-button size="small" type="danger" circle @click="removePatch(rule, pi)" :disabled="rule.patches.length <= 1">
                    <el-icon><Close /></el-icon>
                  </el-button>
                </div>
              </div>
            </div>
          </div>

          <!-- YAML preview for power users -->
          <details class="yaml-preview">
            <summary>{{ t('transformSets.yamlPreview') }}</summary>
            <pre>{{ yamlPreview }}</pre>
          </details>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="drawerOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" :disabled="readOnly" @click="handleSave">
          {{ readOnly ? t('common.close') : (editingName ? t('common.save') : t('common.create')) }}
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Close, InfoFilled, Loading, Plus, Search, Tools } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getTransformSets, createTransformSet, updateTransformSet, deleteTransformSet,
  getNamespaces
} from '../api/velero'

const { t } = useI18n()
import { useAuth } from '../composables/useAuth'
const auth = useAuth()

const items = ref([])
const namespaces = ref([])
const loading = ref(false)
const filter = ref('')
const drawerOpen = ref(false)
const editingName = ref('')
const readOnly = ref(false)
const form = ref(null)
const saving = ref(false)

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter(t => (t.name + ' ' + (t.description || '')).toLowerCase().includes(q))
})

const editorTitle = computed(() => {
  if (readOnly.value) return t('transformSets.viewTitle', { name: editingName.value })
  if (editingName.value) return t('transformSets.editTitle', { name: editingName.value })
  return t('transformSets.createTitle')
})

const yamlPreview = computed(() => {
  if (!form.value) return ''
  // Render a Velero-style YAML preview so power users can copy/paste it
  // outside SupKube if needed. We don't pull js-yaml in just for this;
  // a hand-rolled mini-emitter is fine for the shape we control.
  const lines = ['version: v1', 'resourceModifierRules:']
  for (const rule of form.value.rules) {
    lines.push('  - conditions:')
    lines.push(`      groupResource: ${rule.conditions.groupResource || ''}`)
    if (rule.conditions.namespaces?.length) {
      lines.push('      namespaces:')
      for (const n of rule.conditions.namespaces) lines.push(`        - ${n}`)
    }
    if (rule.conditions.resourceNameRegex) {
      lines.push(`      resourceNameRegex: ${JSON.stringify(rule.conditions.resourceNameRegex)}`)
    }
    lines.push('    patches:')
    for (const p of rule.patches) {
      lines.push(`      - operation: ${p.operation}`)
      lines.push(`        path: ${JSON.stringify(p.path)}`)
      if (['add','replace','test'].includes(p.operation) && p.value !== undefined && p.value !== '') {
        const v = typeof p.value === 'string' ? JSON.stringify(p.value) : JSON.stringify(p.value)
        lines.push(`        value: ${v}`)
      }
    }
  }
  return lines.join('\n')
})

async function fetchItems() {
  loading.value = true
  try {
    const res = await getTransformSets()
    items.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load Transform Sets: ' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

async function fetchNamespaces() {
  try {
    const res = await getNamespaces()
    namespaces.value = (res.data.items || []).map(ns => ns.metadata?.name || ns)
  } catch {}
}

function openEditor(ts) {
  editingName.value = ts ? ts.name : ''
  readOnly.value = false
  form.value = ts ? deepClone(ts) : freshForm()
  drawerOpen.value = true
}

function openViewer(ts) {
  editingName.value = ts.name
  readOnly.value = true
  form.value = deepClone(ts)
  drawerOpen.value = true
}

function freshForm() {
  return {
    name: '',
    description: '',
    rules: [{
      conditions: { groupResource: '', namespaces: [], resourceNameRegex: '' },
      patches: [{ operation: 'remove', path: '', value: '' }]
    }]
  }
}

function deepClone(o) { return JSON.parse(JSON.stringify(o)) }

function addRule() {
  form.value.rules.push({
    conditions: { groupResource: '', namespaces: [], resourceNameRegex: '' },
    patches: [{ operation: 'remove', path: '', value: '' }]
  })
}
function removeRule(idx) { form.value.rules.splice(idx, 1) }
function addPatch(rule) {
  rule.patches.push({ operation: 'remove', path: '', value: '' })
}
function removePatch(rule, idx) { rule.patches.splice(idx, 1) }

async function handleSave() {
  if (readOnly.value) { drawerOpen.value = false; return }
  saving.value = true
  try {
    // Clean up empty fields so backend validation doesn't reject "".
    const payload = deepClone(form.value)
    for (const rule of payload.rules) {
      if (!rule.conditions.namespaces?.length) delete rule.conditions.namespaces
      if (!rule.conditions.resourceNameRegex) delete rule.conditions.resourceNameRegex
      for (const p of rule.patches) {
        if (!['add','replace','test'].includes(p.operation)) delete p.value
        else if (p.value === '' || p.value === null) delete p.value
      }
    }
    if (editingName.value) {
      await updateTransformSet(editingName.value, payload)
      ElMessage.success(t('transformSets.savedToast', { name: editingName.value }))
    } else {
      await createTransformSet(payload)
      ElMessage.success(t('transformSets.createdToast', { name: payload.name }))
    }
    drawerOpen.value = false
    await fetchItems()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  } finally {
    saving.value = false
  }
}

async function handleCommand(cmd, ts) {
  switch (cmd) {
    case 'view': openViewer(ts); break
    case 'edit': openEditor(ts); break
    case 'clone': openEditor({ ...deepClone(ts), name: ts.name + '-copy', builtIn: false }); editingName.value = ''; break
    case 'delete': await confirmDelete(ts); break
  }
}

async function confirmDelete(ts) {
  try {
    await ElMessageBox.confirm(
      t('transformSets.deleteConfirm', { name: ts.name }),
      t('transformSets.deleteTitle'),
      { type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel') }
    )
  } catch { return }
  try {
    await deleteTransformSet(ts.name)
    ElMessage.success(t('transformSets.deletedToast', { name: ts.name }))
    await fetchItems()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  }
}

onMounted(() => {
  fetchItems()
  fetchNamespaces()
})
</script>

<style scoped>
.ts-page { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 18px; }
.page-header h3 { margin: 0 0 4px 0; font-size: 20px; font-weight: 600; }
.page-desc { margin: 0; color: var(--sk-text-caption); font-size: 13px; max-width: 720px; line-height: 1.5; }

.filter-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.filter-input { width: 300px; }
.spacer { flex: 1; }
.result-count { color: var(--sk-text-muted); font-size: 13px; }

.ts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}
.ts-card {
  background: #fff;
  border: 1px solid var(--sk-border);
  border-radius: 8px;
  padding: 14px 16px 12px;
  transition: box-shadow 0.15s;
}
.ts-card:hover { box-shadow: 0 2px 8px rgba(0,0,0,0.05); }
.ts-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.ts-name {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: 14px; font-weight: 600; color: var(--sk-text);
}
.ts-name .el-icon { color: var(--sk-primary); }
.builtin-pill {
  font-size: 10px; padding: 1px 6px; border-radius: 3px;
  background: #ecf5ff; color: var(--sk-primary); font-weight: 500;
}
.ts-desc {
  font-size: 12.5px; color: var(--sk-text-muted); line-height: 1.5;
  min-height: 38px; margin-bottom: 10px;
}
.ts-meta { display: flex; gap: 6px; margin-bottom: 10px; }
.meta-chip {
  font-size: 11px; color: var(--sk-text-caption); background: #f5f7fa;
  padding: 2px 8px; border-radius: 10px;
}
.ts-rules-preview {
  font-size: 11.5px;
  color: var(--sk-text-muted);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  background: #fafbfc;
  border-radius: 4px;
  padding: 8px 10px;
  line-height: 1.6;
}
.rule-gr { font-weight: 600; color: var(--sk-text); }
.rule-arrow { color: var(--sk-text-placeholder); margin: 0 6px; }
.rule-op { color: var(--sk-primary); }
.rule-op code { background: rgba(0,0,0,0.05); padding: 1px 3px; border-radius: 3px; font-size: 10.5px; }
.rule-more { color: var(--sk-text-caption); font-style: italic; margin-top: 4px; }

.more-btn {
  background: none; border: 0;
  color: var(--sk-text-caption); font-size: 18px; cursor: pointer;
  padding: 2px 4px;
}
.more-btn:hover { color: var(--sk-primary); }
.dots { letter-spacing: 1px; }

.loading, .empty {
  text-align: center; padding: 60px 20px; color: var(--sk-text-caption); font-size: 13px;
  background: #fff; border: 1px dashed #dcdfe6; border-radius: 8px;
}
.empty .el-icon { font-size: 28px; display: block; margin: 0 auto 8px; color: var(--sk-text-placeholder); }
.rot { animation: rot 1s linear infinite; }
@keyframes rot { from { transform: rotate(0); } to { transform: rotate(360deg); } }

/* Editor */
.editor-body { padding: 0 4px; }
.rules-section { margin-top: 12px; }
.rules-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.rules-title { font-size: 14px; font-weight: 600; }
.rule-editor {
  border: 1px solid var(--sk-border); border-radius: 6px;
  padding: 12px 14px; margin-bottom: 12px;
  background: #fafbfc;
}
.rule-editor-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.rule-num { font-size: 12px; color: var(--sk-text-caption); font-weight: 600; }
.patches-section { margin-top: 8px; }
.patches-head {
  display: flex; justify-content: space-between; align-items: center;
  font-size: 12px; color: var(--sk-text-muted); font-weight: 500;
  margin-bottom: 6px;
}
.patch-row {
  display: grid;
  grid-template-columns: 110px 1fr 1fr 32px;
  gap: 6px;
  margin-bottom: 6px;
}
.yaml-preview {
  margin-top: 16px; padding-top: 12px;
  border-top: 1px dashed #ebeef5;
}
.yaml-preview summary {
  font-size: 12px; color: var(--sk-primary); cursor: pointer; user-select: none;
}
.yaml-preview pre {
  margin: 10px 0 0 0;
  background: #1d1e26; color: #e5eaf3;
  padding: 12px 14px; border-radius: 6px;
  font-size: 11.5px; line-height: 1.55;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  white-space: pre; overflow-x: auto; max-height: 360px;
}

/* Dark mode */
:global(html.dark) .ts-card { background: #1f2026; border-color: #2c2f36; }
:global(html.dark) .ts-name { color: #e5eaf3; }
:global(html.dark) .ts-desc { color: #b1b3b8; }
:global(html.dark) .ts-rules-preview { background: #16171b; color: #b1b3b8; }
:global(html.dark) .rule-gr { color: #e5eaf3; }
:global(html.dark) .meta-chip { background: #2c2f36; color: #b1b3b8; }
:global(html.dark) .rule-editor { background: #1a1b21; border-color: #2c2f36; }
:global(html.dark) .loading, :global(html.dark) .empty {
  background: #1f2026; border-color: #3a3d44; color: var(--sk-text-caption);
}
</style>
