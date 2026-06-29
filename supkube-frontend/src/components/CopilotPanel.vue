<template>
  <el-drawer
    :model-value="state.open"
    @update:model-value="(v) => !v && close()"
    :with-header="false"
    :modal="false"
    size="420px"
    class="cop-drawer"
  >
    <div class="cop">
      <!-- header -->
      <div class="cop-head">
        <div class="cop-title">
          <span class="cop-spark"><el-icon><MagicStick /></el-icon></span>
          SupKube Copilot
          <span class="cop-badge">{{ model }}</span>
        </div>
        <el-icon class="cop-close" @click="close"><Close /></el-icon>
      </div>

      <!-- context chip -->
      <div v-if="state.contextLabel" class="cop-ctx">
        <el-icon><DataAnalysis /></el-icon> {{ state.contextLabel }}
      </div>

      <!-- messages -->
      <div ref="msgsEl" class="cop-msgs">
        <div class="cop-msg ai">
          <div class="cop-who">Copilot</div>
          <div class="cop-bubble cop-ai">{{ t('copilot.welcome') }}</div>
        </div>
        <div v-for="m in state.messages" :key="m.id" :class="['cop-msg', m.role]">
          <div v-if="m.role === 'ai'" class="cop-who">Copilot</div>
          <div :class="['cop-bubble', m.role === 'you' ? 'cop-you' : 'cop-ai']">{{ m.text }}</div>
          <!-- M4 HitL: LLM-proposed action → confirm card -->
          <div v-if="m.action" class="cop-action">
            <div class="cop-action-head"><el-icon><Lightning /></el-icon> {{ m.action.summary }}</div>
            <div class="cop-action-btns">
              <el-button size="small" type="primary" :loading="m.acting" @click="confirmAction(m)">{{ t('copilot.confirm') }}</el-button>
              <el-button size="small" @click="cancelAction(m)">{{ t('copilot.cancel') }}</el-button>
            </div>
          </div>
        </div>
        <div v-if="loading" class="cop-thinking">{{ t('copilot.thinking') }}</div>
      </div>

      <!-- footer: chips + input -->
      <div class="cop-foot">
        <div v-if="!state.messages.length" class="cop-chips">
          <button v-for="(q, i) in chips" :key="i" class="cop-chip" @click="send(q)">{{ q }}</button>
        </div>
        <div class="cop-inputbox">
          <el-input
            v-model="question"
            type="textarea"
            :rows="2"
            resize="none"
            :placeholder="t('copilot.placeholder')"
            @keydown.enter.exact.prevent="send()"
          />
          <div class="cop-controls">
            <el-select v-model="model" size="small" style="width: 124px">
              <el-option label="haiku · fast" value="haiku" />
              <el-option label="sonnet" value="sonnet" />
              <el-option label="opus" value="opus" />
            </el-select>
            <el-button type="primary" circle size="small" :loading="loading" @click="send()">
              <el-icon><Top /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </el-drawer>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MagicStick, Close, Top, DataAnalysis, Lightning } from '@element-plus/icons-vue'
import { useCopilot } from '../composables/useCopilot'
import { askCopilot, getDRScores, createManualSnapshot } from '../api/velero'

const { t } = useI18n()
const { state, close, setContext, addMessage } = useCopilot()

const question = ref('')
const model = ref('haiku')
const loading = ref(false)
const msgsEl = ref(null)

const chips = computed(() => [t('copilot.chip1'), t('copilot.chip2'), t('copilot.chip3')])

// On first open, pull cluster DR scores as grounding context (summary + worst
// apps, trimmed so the payload stays small). Best-effort: if it fails the
// Copilot still answers generally.
watch(
  () => state.open,
  async (open) => {
    if (!open || state.context) return
    try {
      const { data: d } = await getDRScores()
      setContext(`${d.cluster} · ${d.summary?.totalApps || 0} apps`, {
        cluster: d.cluster,
        summary: d.summary,
        worstApps: (d.apps || []).slice(0, 12).map((a) => ({
          namespace: a.namespace,
          score: a.totalScore,
          level: a.level,
          status: a.status,
          protected: a.protected,
          statefulSets: a.statefulSets,
          pvcs: a.pvcs,
          recommendations: (a.recommendations || []).map((r) => r.type)
        }))
      })
    } catch (e) {
      // context optional — leave null, Copilot answers generally
    }
  }
)

async function scrollDown() {
  await nextTick()
  if (msgsEl.value) msgsEl.value.scrollTop = msgsEl.value.scrollHeight
}

async function send(preset) {
  const q = (preset || question.value).trim()
  if (!q || loading.value) return
  question.value = ''
  addMessage('you', q)
  loading.value = true
  await scrollDown()
  try {
    const ctx = state.context ? JSON.stringify(state.context) : ''
    const res = await askCopilot({ prompt: q, context: ctx, model: model.value })
    addMessage('ai', res.answer || '(empty)', res.pendingAction || null)
  } catch (e) {
    const code = e?.response?.status
    let msg = t('copilot.errGeneric', { e: e?.message || code || 'unknown' })
    if (code === 403) msg = t('copilot.errDisabled')
    else if (code === 502) msg = t('copilot.errTimeout')
    else if (code === 401) msg = t('copilot.errAuth')
    addMessage('ai', msg)
  } finally {
    loading.value = false
    await scrollDown()
  }
}

// M4 HitL: user confirmed the LLM-proposed action → call the real write API.
async function confirmAction(m) {
  if (!m.action || m.acting) return
  const a = m.action
  m.acting = true
  try {
    if (a.tool === 'create_backup') {
      const res = await createManualSnapshot(a.namespace, { comment: 'via Copilot' })
      m.action = null
      addMessage('ai', t('copilot.actionDone', { ns: a.namespace, name: res?.data?.backupName || '' }))
    }
  } catch (e) {
    addMessage('ai', t('copilot.actionFail', { e: e?.response?.data?.error || e?.message || 'error' }))
  } finally {
    m.acting = false
    await scrollDown()
  }
}
function cancelAction(m) {
  m.action = null
  addMessage('ai', t('copilot.actionCancelled'))
}
</script>

<style scoped>
.cop-drawer :deep(.el-drawer__body) { padding: 0; }
.cop { display: flex; flex-direction: column; height: 100%; }

.cop-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--el-border-color-light);
}
.cop-title { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 15px; color: var(--el-text-color-primary); }
.cop-spark {
  display: inline-flex; align-items: center; justify-content: center;
  width: 24px; height: 24px; border-radius: 6px; color: #fff;
  background: linear-gradient(135deg, #6366f1, #06b6d4);
}
.cop-badge { font-size: 11px; color: var(--el-text-color-secondary); background: var(--el-fill-color-light); border-radius: 10px; padding: 1px 8px; }
.cop-close { cursor: pointer; color: var(--el-text-color-secondary); font-size: 18px; }
.cop-close:hover { color: var(--el-text-color-primary); }

.cop-ctx {
  display: flex; align-items: center; gap: 6px; font-size: 12px;
  color: var(--el-color-primary); background: var(--el-color-primary-light-9);
  border-bottom: 1px solid var(--el-border-color-lighter); padding: 6px 16px;
}

.cop-msgs { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 14px; }
.cop-msg.you { align-items: flex-end; display: flex; flex-direction: column; }
.cop-who { font-size: 11px; font-weight: 600; color: var(--el-color-primary); margin-bottom: 4px; }
.cop-bubble { max-width: 88%; padding: 9px 12px; border-radius: 12px; font-size: 13.5px; line-height: 1.6; white-space: pre-wrap; word-break: break-word; }
.cop-you { background: var(--el-color-primary); color: #fff; border-bottom-right-radius: 3px; }
.cop-ai { background: var(--el-fill-color-light); color: var(--el-text-color-primary); border-bottom-left-radius: 3px; }
.cop-thinking { font-size: 13px; color: var(--el-text-color-secondary); font-style: italic; }

.cop-foot { border-top: 1px solid var(--el-border-color-light); padding: 12px 16px; }
.cop-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
.cop-chip {
  font-size: 12px; padding: 4px 10px; border-radius: 14px; cursor: pointer;
  border: 1px solid var(--el-border-color); background: var(--el-fill-color-blank);
  color: var(--el-text-color-regular);
}
.cop-chip:hover { background: var(--el-fill-color-light); border-color: var(--el-color-primary); color: var(--el-color-primary); }
.cop-inputbox { border: 1px solid var(--el-border-color); border-radius: 10px; padding: 8px; }
.cop-inputbox :deep(.el-textarea__inner) { box-shadow: none; padding: 2px 4px; }
.cop-controls { display: flex; align-items: center; justify-content: space-between; margin-top: 6px; }
.cop-action { margin-top: 8px; padding: 10px 12px; border: 1px solid var(--el-color-warning-light-5); background: var(--el-color-warning-light-9); border-radius: 10px; }
.cop-action-head { font-size: 13px; font-weight: 600; color: var(--el-text-color-primary); display: flex; align-items: center; gap: 6px; margin-bottom: 8px; }
.cop-action-btns { display: flex; gap: 8px; }
</style>
