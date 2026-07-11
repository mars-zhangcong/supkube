import { reactive } from 'vue'

// Shared Copilot state (Hub-style chat). The header "助手" button toggles `open`;
// CopilotPanel renders the conversation and talks to POST /api/v1/ai/chat.
// `context` is auto-filled with the cluster DR scores when the panel opens, so the
// Copilot can answer "which app is risky / why / how to fix" grounded in real data.
const state = reactive({
  open: false,
  messages: [], // { id, role: 'you'|'ai', text }
  context: null, // object serialized into the /ai/chat context field
  contextLabel: '' // short chip shown at top of panel
})

let _id = 0

export function useCopilot() {
  return {
    state,
    open() { state.open = true },
    toggle() { state.open = !state.open },
    close() { state.open = false },
    setContext(label, ctx) {
      state.contextLabel = label || ''
      state.context = ctx || null
    },
    addMessage(role, text, action = null) {
      // action: optional pendingAction { tool, namespace, summary } → renders a
      // HitL confirm card under the message.
      state.messages.push({ id: ++_id, role, text, action, acting: false })
    },
    clear() { state.messages = [] }
  }
}
