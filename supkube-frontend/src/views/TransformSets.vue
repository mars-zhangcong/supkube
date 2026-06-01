<!--
  TransformSets (PRD-002 v1.3, 2026-05-31)
  ────────────────────────────────────────
  Two-layer model: TransformSet is now a CONTAINER that references one or
  more atomic Transforms (Velero ResourceModifier rule bundles) by name
  and supplies defaults for ${VAR} placeholders. The visual rule editor
  moved into Transforms.vue.

  Page layout:
    - Top description + Create button
    - Filter row (search by name / description)
    - Card grid: each TransformSet shows its name + description + ref count
      + the names of referenced Transforms + a Built-In badge + ⋮ menu
    - Side drawer for create / edit:
      - Pick Transforms from the catalog (multi-select)
      - Defaults map (key/value rows) for ${VAR} substitution
      - No inline rule editing — "manage Transforms…" link jumps to the
        Transforms page

  Built-ins are read-only (clone-to-edit).
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
            <el-icon><Files /></el-icon>
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
          <span class="meta-chip">{{ t('transformSets.refCount', { n: (ts.transformRefs || []).length }) }}</span>
          <span v-if="ts.defaults && Object.keys(ts.defaults).length > 0" class="meta-chip">{{ Object.keys(ts.defaults).length }} {{ t('transformSets.defaults') }}</span>
        </div>
        <div class="ts-refs-preview">
          <span v-for="ref in (ts.transformRefs || []).slice(0, 6)" :key="ref.name" class="ref-chip">
            <el-icon><Tools /></el-icon> {{ ref.name }}
          </span>
          <span v-if="(ts.transformRefs || []).length > 6" class="rule-more">+{{ ts.transformRefs.length - 6 }} more</span>
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
            <el-input v-model="form.name" :disabled="!!editingName || readOnly" placeholder="my-transform-set" />
          </el-form-item>
          <el-form-item :label="t('transformSets.description')">
            <el-input v-model="form.description" type="textarea" :rows="2"
              :disabled="readOnly"
              :placeholder="t('transformSets.descriptionPlaceholder')" />
          </el-form-item>

          <el-form-item required>
            <template #label>
              <span class="label-row">
                {{ t('transformSets.refsLabel') }}
                <a class="manage-link" href="#" @click.prevent="goManageTransforms">
                  {{ t('transformSets.manageTransforms') }}
                </a>
              </span>
            </template>
            <el-select
              v-model="selectedRefNames"
              multiple
              filterable
              :disabled="readOnly"
              :placeholder="t('transformSets.refsPlaceholder')"
              class="refs-select"
            >
              <el-option
                v-for="tr in availableTransforms"
                :key="tr.name"
                :label="tr.name"
                :value="tr.name"
              >
                <span class="ref-option">
                  <span>{{ tr.name }}</span>
                  <span v-if="tr.builtIn" class="ref-builtin">{{ t('transformSets.builtin') }}</span>
                  <span class="ref-desc">{{ tr.description || '' }}</span>
                </span>
              </el-option>
            </el-select>
            <div v-if="selectedRefNames.length === 0" class="refs-empty-hint">{{ t('transformSets.refsEmpty') }}</div>
          </el-form-item>

          <div class="defaults-section">
            <div class="defaults-head">
              <span class="defaults-title">{{ t('transformSets.defaults') }}</span>
              <el-button size="small" :disabled="readOnly" @click="addDefault">
                <el-icon><Plus /></el-icon> {{ t('transformSets.addDefault') }}
              </el-button>
            </div>
            <div class="defaults-hint">{{ t('transformSets.defaultsHint') }}</div>
            <div v-for="(entry, idx) in defaultsList" :key="idx" class="default-row">
              <el-input v-model="entry.key" :placeholder="t('transformSets.defaultsKey')" :disabled="readOnly" class="default-key" />
              <el-input v-model="entry.value" :placeholder="t('transformSets.defaultsValue')" :disabled="readOnly" class="default-value" />
              <el-button size="small" type="danger" circle :disabled="readOnly" @click="removeDefault(idx)">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
          </div>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="drawerOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" :disabled="readOnly || selectedRefNames.length === 0" @click="handleSave">
          {{ readOnly ? t('common.close') : (editingName ? t('common.save') : t('common.create')) }}
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Close, Files, InfoFilled, Loading, Plus, Search, Tools } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getTransformSets, createTransformSet, updateTransformSet, deleteTransformSet,
  getTransforms
} from '../api/velero'

const { t } = useI18n()
const router = useRouter()
import { useAuth } from '../composables/useAuth'
const auth = useAuth()

const items = ref([])
const availableTransforms = ref([])
const loading = ref(false)
const filter = ref('')
const drawerOpen = ref(false)
const editingName = ref('')
const readOnly = ref(false)
const form = ref(null)
const selectedRefNames = ref([])
// Defaults map is rendered as an array of {key, value} rows so the user
// can add/remove without keying the map directly in templates. We flatten
// back to an object on save.
const defaultsList = ref([])
const saving = ref(false)

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter(ts => (ts.name + ' ' + (ts.description || '')).toLowerCase().includes(q))
})

const editorTitle = computed(() => {
  if (readOnly.value) return t('transformSets.viewTitle', { name: editingName.value })
  if (editingName.value) return t('transformSets.editTitle', { name: editingName.value })
  return t('transformSets.createTitle')
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

async function fetchTransforms() {
  try {
    const res = await getTransforms()
    availableTransforms.value = res.data.items || []
  } catch (e) {
    // Non-fatal — the picker just shows nothing. Users see why via the
    // empty hint, and can hop to the Transforms page to investigate.
    console.error('Failed to load Transforms catalog:', e)
    availableTransforms.value = []
  }
}

function openEditor(ts) {
  editingName.value = ts ? ts.name : ''
  readOnly.value = false
  form.value = ts ? deepClone(ts) : freshForm()
  selectedRefNames.value = (form.value.transformRefs || []).map(r => r.name)
  defaultsList.value = Object.entries(form.value.defaults || {}).map(([k, v]) => ({ key: k, value: v }))
  drawerOpen.value = true
  // Refresh the Transforms catalog when opening so the picker reflects any
  // additions made in another tab.
  fetchTransforms()
}

function openViewer(ts) {
  editingName.value = ts.name
  readOnly.value = true
  form.value = deepClone(ts)
  selectedRefNames.value = (form.value.transformRefs || []).map(r => r.name)
  defaultsList.value = Object.entries(form.value.defaults || {}).map(([k, v]) => ({ key: k, value: v }))
  drawerOpen.value = true
  fetchTransforms()
}

function freshForm() {
  return {
    name: '',
    description: '',
    transformRefs: [],
    defaults: {}
  }
}

function deepClone(o) { return JSON.parse(JSON.stringify(o)) }

function addDefault() {
  defaultsList.value.push({ key: '', value: '' })
}
function removeDefault(idx) {
  defaultsList.value.splice(idx, 1)
}

function goManageTransforms() {
  router.push('/transforms')
}

async function handleSave() {
  if (readOnly.value) { drawerOpen.value = false; return }
  saving.value = true
  try {
    // Defaults: flatten back to {key: value}, skip blank rows.
    const defaults = {}
    for (const { key, value } of defaultsList.value) {
      const k = (key || '').trim()
      if (!k) continue
      defaults[k] = value ?? ''
    }
    const payload = {
      name: form.value.name,
      description: form.value.description || '',
      transformRefs: selectedRefNames.value.map(name => ({ name })),
      defaults: Object.keys(defaults).length > 0 ? defaults : undefined
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
    case 'clone': {
      const cloned = { ...deepClone(ts), name: ts.name + '-copy', builtIn: false }
      openEditor(cloned)
      editingName.value = ''
      readOnly.value = false
      break
    }
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

// Keep form.transformRefs in sync with the selected names so the YAML/cards
// always reflect what the user picked.
watch(selectedRefNames, (names) => {
  if (!form.value) return
  form.value.transformRefs = names.map(name => ({ name }))
})

onMounted(() => {
  fetchItems()
  fetchTransforms()
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
.ts-refs-preview {
  display: flex; flex-wrap: wrap; gap: 6px;
  background: #fafbfc;
  border-radius: 4px;
  padding: 8px 10px;
  min-height: 38px;
}
.ref-chip {
  display: inline-flex; align-items: center; gap: 4px;
  background: #fff; border: 1px solid var(--sk-border);
  border-radius: 4px; padding: 2px 8px;
  font-size: 11.5px; color: var(--sk-text);
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.ref-chip .el-icon { color: var(--sk-primary); font-size: 12px; }
.rule-more { color: var(--sk-text-caption); font-style: italic; font-size: 11.5px; }

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

.editor-body { padding: 0 4px; }
.label-row {
  display: inline-flex; align-items: center; gap: 10px; width: 100%;
}
.manage-link {
  margin-left: auto; font-size: 12px; color: var(--sk-primary);
  text-decoration: none;
}
.manage-link:hover { text-decoration: underline; }
.refs-select { width: 100%; }
.ref-option {
  display: inline-flex; align-items: center; gap: 8px; width: 100%;
}
.ref-builtin {
  font-size: 10px; padding: 1px 5px; border-radius: 3px;
  background: #ecf5ff; color: var(--sk-primary);
}
.ref-desc {
  margin-left: auto; color: var(--sk-text-caption); font-size: 11.5px;
  max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.refs-empty-hint {
  margin-top: 6px; font-size: 12px; color: var(--sk-status-warning);
}

.defaults-section { margin-top: 16px; }
.defaults-head { display: flex; justify-content: space-between; align-items: center; }
.defaults-title { font-size: 14px; font-weight: 600; }
.defaults-hint { font-size: 12px; color: var(--sk-text-muted); margin: 4px 0 10px; line-height: 1.5; }
.default-row {
  display: grid;
  grid-template-columns: 200px 1fr 32px;
  gap: 6px;
  margin-bottom: 6px;
}

:global(html.dark) .ts-card { background: #1f2026; border-color: #2c2f36; }
:global(html.dark) .ts-name { color: #e5eaf3; }
:global(html.dark) .ts-desc { color: #b1b3b8; }
:global(html.dark) .ts-refs-preview { background: #16171b; }
:global(html.dark) .ref-chip { background: #2c2f36; border-color: #3a3d44; color: #cfd3dc; }
:global(html.dark) .meta-chip { background: #2c2f36; color: #b1b3b8; }
:global(html.dark) .loading, :global(html.dark) .empty {
  background: #1f2026; border-color: #3a3d44; color: var(--sk-text-caption);
}
</style>
