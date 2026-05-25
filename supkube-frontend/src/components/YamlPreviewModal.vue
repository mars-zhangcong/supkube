<!--
  YamlPreviewModal (v0.8.10.3) — overlay that shows a single resource's
  YAML with syntax highlighting + copy button.

  Triggered by the </> icon next to every artifact row in the
  ActionDetailDrawer + Application Details drawer. Resource YAML is
  fetched lazily on open from /api/v1/resources/yaml.

  Why a custom regex highlighter instead of highlight.js / Prism
  ─────────────────────────────────────────────────────────────
  YAML is simple enough (keys, scalars, comments, list markers) that
  a 40-line regex pass yields perfectly serviceable colouring without
  pulling in a ~50 KB JS dependency. We're not editing YAML here —
  read-only preview only — so we don't need a full tokenizer.
-->
<template>
  <el-dialog
    v-model="visibleProxy"
    :title="title"
    width="720"
    :close-on-click-modal="true"
    align-center
    class="yaml-preview-dialog"
  >
    <div class="yaml-preview-toolbar">
      <span class="sk-caption">{{ subtitle }}</span>
      <span class="yaml-preview-spacer"></span>
      <el-button v-if="yaml" size="small" @click="copyYaml">
        <el-icon><DocumentCopy /></el-icon>
        <span style="margin-left:4px">Copy</span>
      </el-button>
    </div>

    <div v-if="loading" class="yaml-preview-loading sk-caption">
      <el-icon class="spin"><Loading /></el-icon>
      Fetching YAML from the live cluster…
    </div>
    <div v-else-if="error" class="yaml-preview-error sk-body">
      <strong>Couldn't fetch YAML:</strong>
      <div style="margin-top:6px">{{ error }}</div>
    </div>
    <pre v-else class="yaml-preview-block" v-html="highlighted"></pre>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { DocumentCopy, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getResourceYaml } from '../api/velero'

const props = defineProps({
  visible: { type: Boolean, default: false },
  // Resource coordinates — required when visible toggles to true.
  kind:      { type: String, default: '' },
  name:      { type: String, default: '' },
  namespace: { type: String, default: '' }
})
const emit = defineEmits(['update:visible'])
const visibleProxy = computed({
  get: () => props.visible,
  set: (v) => emit('update:visible', v)
})

const yaml = ref('')
const loading = ref(false)
const error = ref('')

const title = computed(() => `${props.kind || 'Resource'} — ${props.name || ''}`)
const subtitle = computed(() => props.namespace ? `namespace: ${props.namespace}` : '')

// Watch visibility — fetch YAML when modal opens. We clear state on
// close so the next open of a different resource doesn't briefly
// flash stale content while the new request is in flight.
watch(() => props.visible, async (v) => {
  if (!v) {
    yaml.value = ''
    error.value = ''
    return
  }
  if (!props.kind || !props.name) return
  loading.value = true
  error.value = ''
  yaml.value = ''
  try {
    const res = await getResourceYaml({
      kind: props.kind,
      name: props.name,
      namespace: props.namespace
    })
    yaml.value = res.data?.yaml || ''
  } catch (e) {
    error.value = e?.response?.data?.error || e.message || 'unknown error'
  } finally {
    loading.value = false
  }
})

// Custom YAML highlighter — tokenises keys, string values, numeric
// values, comments, list markers. Escape-encodes raw text first so
// the input can't break out of the <pre> (security: prevents <script>
// injection if a malicious resource embeds HTML in a label value).
function escapeHTML(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}
const highlighted = computed(() => {
  if (!yaml.value) return ''
  const escaped = escapeHTML(yaml.value)
  return escaped
    // 1. comments — full-line and trailing  (# ... to EOL)
    .replace(/(^|\s)(#.*$)/gm, '$1<span class="yh-comment">$2</span>')
    // 2. document marker
    .replace(/^(---|\.\.\.)\s*$/gm, '<span class="yh-doc">$1</span>')
    // 3. list marker leading "- "
    .replace(/^(\s*)(- )/gm, '$1<span class="yh-list">$2</span>')
    // 4. keys at any depth:   "  foo:"  → bold colour
    .replace(/^(\s*)([\w.\-/]+)(\s*:)/gm, '$1<span class="yh-key">$2</span>$3')
    // 5. quoted string values  "..."
    .replace(/(: )("[^"]*"|'[^']*')/g, '$1<span class="yh-string">$2</span>')
    // 6. numeric values
    .replace(/(: )(-?\d+(\.\d+)?)(\s|$)/g, '$1<span class="yh-num">$2</span>$4')
    // 7. boolean / null literals
    .replace(/(: )(true|false|null|~)\b/g, '$1<span class="yh-bool">$2</span>')
})

function copyYaml() {
  if (!yaml.value) return
  navigator.clipboard.writeText(yaml.value).then(
    () => ElMessage.success('YAML copied to clipboard'),
    () => ElMessage.error('Copy failed')
  )
}
</script>

<style scoped>
.yaml-preview-dialog :deep(.el-dialog__body) {
  padding: 12px 20px 20px;
}
.yaml-preview-toolbar {
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding-bottom: 10px;
  border-bottom: 1px solid var(--sk-border);
  margin-bottom: 10px;
}
.yaml-preview-spacer { flex: 1; }

.yaml-preview-loading,
.yaml-preview-error {
  padding: 24px;
  text-align: center;
}
.yaml-preview-error {
  background: #fef0f0;
  border-left: 3px solid var(--sk-status-error);
  border-radius: 4px;
  text-align: left;
  color: var(--sk-text-secondary);
}

/* The YAML block. Dark Kasten-style; the highlighter classes below
   match a Solarized-ish palette tuned for dark backgrounds.  */
.yaml-preview-block {
  background: #1a1a2e;
  color: #d4d4dc;
  padding: 16px 18px;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 60vh;
  overflow: auto;
}

/* ─── Highlighter token classes ─── */
.yaml-preview-block :deep(.yh-comment) { color: #6a737d;   font-style: italic; }
.yaml-preview-block :deep(.yh-doc)     { color: #d19a66;   font-weight: 700; }
.yaml-preview-block :deep(.yh-list)    { color: #c678dd; }
.yaml-preview-block :deep(.yh-key)     { color: #79b8ff; }
.yaml-preview-block :deep(.yh-string)  { color: #a8d8a8; }
.yaml-preview-block :deep(.yh-num)     { color: #e5c07b; }
.yaml-preview-block :deep(.yh-bool)    { color: #d19a66; }

.spin { animation: yh-spin 0.9s linear infinite; }
@keyframes yh-spin { to { transform: rotate(360deg); } }
</style>
