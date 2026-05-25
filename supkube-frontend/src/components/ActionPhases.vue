<!--
  ActionPhases (v0.8.0)
  ─────────────────────
  Renders the Kasten-style ordered phase checklist for an Action.

  Each phase has one of:
    pending   — grey circle (not started)
    running   — animated blue spinner (active now)
    completed — green ✓
    failed    — red ✗
    skipped   — muted "(skipped)"

  Phases with `detailLines` get nested bullet sub-rows. Used inside both
  ActionCard (compact) and ActionDetailDrawer (expanded). Pass `compact`
  to hide pending phases — saves vertical space on cards.
-->
<template>
  <div class="action-phases" :class="{ compact }">
    <div
      v-for="(phase, idx) in visiblePhases"
      :key="idx"
      class="phase-row"
      :class="`phase-${phase.status}`"
    >
      <span class="phase-bullet">
        <el-icon v-if="phase.status === 'completed'"><CircleCheckFilled /></el-icon>
        <el-icon v-else-if="phase.status === 'failed'"><CircleCloseFilled /></el-icon>
        <el-icon v-else-if="phase.status === 'running'" class="phase-spin"><Loading /></el-icon>
        <span v-else class="phase-dot" />
      </span>
      <div class="phase-text">
        <div class="phase-name">{{ phase.name }}</div>
        <ul v-if="phase.detailLines && phase.detailLines.length" class="phase-details">
          <li v-for="(d, i) in phase.detailLines" :key="i">{{ d }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { CircleCheckFilled, CircleCloseFilled, Loading } from '@element-plus/icons-vue'

const props = defineProps({
  phases: { type: Array, default: () => [] },
  // Compact mode: hide phases that haven't started yet. Useful inside
  // the Action card list where vertical density matters.
  compact: { type: Boolean, default: false }
})

const visiblePhases = computed(() => {
  if (!props.compact) return props.phases
  // Keep all non-pending. Plus the FIRST pending if it's the one right
  // after a running phase (lets the user see "what's next"). Actually
  // simpler — in compact mode, just drop pure-pending tail phases that
  // are after the last interesting (running/completed/failed) one.
  const out = []
  for (const p of props.phases) {
    if (p.status !== 'pending') out.push(p)
  }
  return out.length > 0 ? out : props.phases
})
</script>

<style scoped>
.action-phases {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.action-phases.compact { gap: 3px; }

.phase-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
}
.phase-bullet {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-top: 2px;
}
.phase-bullet .el-icon { font-size: 14px; }
.phase-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dcdfe6;
}
.phase-spin {
  animation: phase-rotate 1s linear infinite;
}
@keyframes phase-rotate {
  from { transform: rotate(0); }
  to { transform: rotate(360deg); }
}

.phase-text { flex: 1; min-width: 0; }
.phase-name { line-height: 1.45; }

.phase-completed .phase-bullet .el-icon { color: var(--sk-status-success); }
.phase-completed .phase-name { color: var(--sk-text-secondary); }

.phase-running .phase-bullet .el-icon { color: #409eff; }
.phase-running .phase-name { color: var(--sk-text-secondary); font-weight: 500; }

.phase-failed .phase-bullet .el-icon { color: var(--sk-status-error); }
.phase-failed .phase-name { color: var(--sk-status-error); font-weight: 500; }

.phase-pending .phase-name { color: var(--sk-text-caption); }

.phase-skipped .phase-name {
  color: var(--sk-text-caption);
  font-style: italic;
}

.phase-details {
  margin: 4px 0 0 0;
  padding-left: 18px;
  font-size: 12px;
  color: var(--sk-text-muted);
  line-height: 1.5;
}
.phase-details li { margin: 2px 0; }

/* Dark mode */
:global(html.dark) .phase-completed .phase-name,
:global(html.dark) .phase-running .phase-name { color: #e5eaf3; }
:global(html.dark) .phase-pending .phase-name { color: var(--sk-text-caption); }
:global(html.dark) .phase-details { color: #b1b3b8; }
:global(html.dark) .phase-dot { background: #4a4d54; }
</style>
