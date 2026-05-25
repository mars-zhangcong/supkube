<!--
  DRTopology (v0.8.12.6)
  ───────────────────────────────────────────────────────────────────────
  Collapsible DR-topology card for the Dashboard.

  Header (always visible):
    > DR Topology  [score chips inline]  N clusters · N policies · N RPs
    ^                                                                  ^
    chevron toggle                                            click target

  Body (when expanded):
    - Cluster cards (left): k8s version + node count + ns list
    - BSL cards (right): provider, RP count, capacity bar, Object Lock,
      last-backup-at chip
    - Flow lines: only between actually-connected ns ↔ BSL pairs
    - Orphan BSLs (no flow + no RPs OR Unavailable) fold into a
      "Show N inactive BSLs ▾" link

  Persistence: collapsed/expanded state lives in localStorage
  ("supkube.drTopology.expanded") so the user's preference survives
  reloads. Default is COLLAPSED — frees Dashboard real estate; the
  score is still visible in the compact header chip strip.

  Why self-rendered SVG (vs ECharts): we need precise control over
  card layout (rich children: chips, progress bars, hover tooltips,
  click navigation). 350 LOC, no deps beyond what we already use.
-->

<template>
  <div class="dr-topology sk-card" :class="{ 'is-collapsed': !expanded }">
    <!-- ════ Header (always visible, click to toggle) ════ -->
    <div class="dr-header" @click="toggle">
      <div class="dr-header-left">
        <span class="dr-chevron" :class="{ 'is-open': expanded }">▶</span>
        <div class="dr-header-text">
          <h3 class="sk-h2 dr-title">{{ t('topology.title') }}</h3>
          <p v-if="expanded" class="sk-caption dr-subtitle">{{ t('topology.subtitle') }}</p>
        </div>
      </div>

      <!-- Score chips (compact) — always shown; in collapsed state this
           is the single most-important glance signal. -->
      <div class="dr-header-score" @click.stop>
        <span class="dr-score-label-inline">
          <span class="dr-score-text">3-2-1-1-0</span>
          <span class="dr-score-count-inline">{{ scoreTotal }}/5</span>
        </span>
        <div class="dr-score-dots-inline">
          <span
            v-for="(rule, ri) in scoreRules"
            :key="`rule-${ri}`"
            class="dr-score-item-inline"
            :class="{ 'is-ok': rule.ok, 'is-bad': !rule.ok }"
            :title="rule.note || rule.label"
          >
            <span class="dr-dot">{{ rule.ok ? '●' : '○' }}</span>
            <span class="dr-rule-label">{{ rule.label }}</span>
          </span>
        </div>
        <span class="dr-header-counts sk-caption">
          {{ summary.clusterCount }} {{ t('topology.clusters') }} ·
          {{ summary.policyCount }} {{ t('topology.policies') }} ·
          {{ summary.rpCount }} {{ t('topology.restorePoints') }}
        </span>
      </div>
    </div>

    <!-- ════ Body (only when expanded) ════ -->
    <div v-if="expanded" v-loading="loading" class="dr-body">
      <div ref="canvasRef" class="dr-canvas">
        <svg
          v-if="layout"
          :viewBox="`0 0 ${layout.width} ${layout.height}`"
          :width="layout.width"
          :height="layout.height"
          class="dr-svg"
          preserveAspectRatio="xMidYMid meet"
        >
          <!-- Flow lines (drawn first so cards sit on top) -->
          <g class="dr-flows">
            <g v-for="(flow, i) in layout.flowPaths" :key="`flow-${i}`">
              <path
                :d="flow.d"
                :stroke="flow.color"
                :stroke-width="flow.width"
                :stroke-dasharray="flow.dashed ? '4 4' : '0'"
                fill="none"
                class="dr-flow-line"
                @mouseenter="hoverFlow = flow"
                @mouseleave="hoverFlow = null"
              />
              <!-- Inline policy label at curve midpoint (fix A) -->
              <text
                :x="flow.labelX"
                :y="flow.labelY"
                class="dr-flow-label"
                text-anchor="middle"
              >{{ flow.shortLabel }}</text>
            </g>
          </g>

          <!-- Cluster cards -->
          <g class="dr-clusters">
            <g
              v-for="(cluster, ci) in layout.clusters"
              :key="`cluster-${ci}`"
              :transform="`translate(${cluster.x}, ${cluster.y})`"
              class="dr-clickable"
              @click="goToApplications"
            >
              <rect
                class="dr-card-bg"
                :width="cluster.width"
                :height="cluster.height"
                rx="8"
              />
              <text :x="14" :y="24" class="dr-card-title">{{ cluster.name }}</text>
              <text v-if="cluster.isCurrent" :x="cluster.width - 14" :y="24" text-anchor="end" class="dr-card-badge">
                ★ {{ t('topology.current') }}
              </text>
              <!-- v0.8.12.6 fix B: k8s version + node count -->
              <text :x="14" :y="44" class="dr-card-meta">
                <tspan v-if="cluster.k8sVersion">k8s {{ cluster.k8sVersion }}</tspan>
                <tspan v-if="cluster.k8sVersion && cluster.nodeCount" dx="6">·</tspan>
                <tspan v-if="cluster.nodeCount" dx="6">{{ cluster.nodeCount }} {{ t('topology.nodes') }}</tspan>
              </text>
              <text :x="14" :y="62" class="dr-card-meta">
                {{ cluster.namespaceNames.length }} {{ t('topology.namespacesShort') }} · {{ cluster.policyCount }} {{ t('topology.policiesShort') }}
              </text>

              <!-- ns rows -->
              <g
                v-for="(ns, ni) in cluster.nsRows"
                :key="`ns-${ci}-${ni}`"
                :transform="`translate(0, ${ns.y})`"
              >
                <text :x="22" :y="14" class="dr-ns-name">
                  <tspan class="dr-bullet">·</tspan>
                  {{ ns.name === '*' ? t('topology.allNamespaces') : ns.name }}
                </text>
                <!-- v0.8.12.6 fix 2: only draw port when ns has at least
                     one connected flow (covered by hasFlow lookup). -->
                <circle v-if="ns.hasFlow" :cx="cluster.width - 6" :cy="10" r="4" class="dr-port" />
              </g>

              <text
                v-if="cluster.truncatedCount > 0"
                :x="22"
                :y="cluster.height - 10"
                class="dr-card-meta"
              >+ {{ cluster.truncatedCount }} {{ t('topology.more') }}</text>
            </g>
          </g>

          <!-- BSL cards -->
          <g class="dr-bsls">
            <g
              v-for="(bsl, bi) in layout.bsls"
              :key="`bsl-${bi}`"
              :transform="`translate(${bsl.x}, ${bsl.y})`"
              class="dr-clickable"
              @click="goToStorage"
            >
              <rect
                class="dr-card-bg"
                :class="{
                  'dr-card-bg-local': bsl.kind === 'local',
                  'dr-card-bg-unavailable': bsl.phase !== 'Available'
                }"
                :width="bsl.width"
                :height="bsl.height"
                rx="8"
              />
              <!-- v0.8.12.6 fix 2: port only if at least one flow targets this BSL -->
              <circle v-if="bsl.hasFlow" :cx="6" :cy="10" r="4" class="dr-port" />

              <text :x="18" :y="22" class="dr-card-title">{{ bsl.name }}</text>
              <text :x="bsl.width - 12" :y="22" text-anchor="end" class="dr-card-badge">
                {{ bsl.kind === 'local' ? '🏠' : '☁' }} {{ bsl.kind === 'local' ? t('topology.local') : t('topology.cloud') }}
              </text>
              <text :x="18" :y="42" class="dr-card-meta">
                {{ bsl.provider }} · {{ bsl.rpCount }} {{ t('topology.restorePointsShort') }} · {{ bsl.backedupNs }} {{ t('topology.nsCovered') }}
              </text>
              <text v-if="bsl.bucket" :x="18" :y="58" class="dr-card-meta dr-mono">
                {{ bsl.bucket }}
              </text>
              <!-- v0.8.12.6 fix E: last backup chip -->
              <text :x="bsl.width - 12" :y="42" text-anchor="end" class="dr-card-meta">
                <tspan v-if="bsl.lastBackupAt">{{ t('topology.lastBackup') }} {{ formatAgo(bsl.lastBackupAt) }}</tspan>
                <tspan v-else>{{ t('topology.neverBackedUp') }}</tspan>
              </text>

              <!-- v0.8.12.6 fix C: capacity bar (local BSL only — backend
                   returns 0 capacityBytes for cloud and we hide the bar). -->
              <g v-if="bsl.capacityBytes > 0" :transform="`translate(18, 60)`">
                <rect :width="bsl.width - 36" :height="6" rx="3" class="dr-cap-track" />
                <rect
                  :width="(bsl.width - 36) * (bsl.usedBytes / bsl.capacityBytes || 0)"
                  :height="6"
                  rx="3"
                  class="dr-cap-bar"
                />
                <text :x="0" :y="20" class="dr-card-meta">
                  {{ formatBytes(bsl.usedBytes) }} / {{ formatBytes(bsl.capacityBytes) }}
                </text>
              </g>

              <text v-if="bsl.objectLockEnabled" :x="18" :y="bsl.height - 12" class="dr-card-chip dr-chip-lock">
                🛡 Object Lock ({{ bsl.objectLockMode }})
              </text>
              <text v-else :x="18" :y="bsl.height - 12" class="dr-card-chip dr-chip-warn">
                No Object Lock
              </text>
              <text :x="bsl.width - 12" :y="bsl.height - 12" text-anchor="end" :class="`dr-chip-status dr-chip-${bsl.phase === 'Available' ? 'ok' : 'bad'}`">
                {{ bsl.phase || 'Unknown' }}
              </text>
            </g>
          </g>
        </svg>

        <!-- Floating flow tooltip -->
        <div v-if="hoverFlow" class="dr-tooltip" :style="hoverFlow.tipStyle">
          <div class="dr-tip-title">{{ hoverFlow.from }} → {{ hoverFlow.to }}</div>
          <div class="dr-tip-row">
            {{ hoverFlow.policyNames.length }} {{ t('topology.policies') }} ·
            {{ hoverFlow.backupCount }} {{ t('topology.restorePointsShort') }}
          </div>
          <div v-if="hoverFlow.policyNames.length" class="dr-tip-row">
            <code v-for="p in hoverFlow.policyNames.slice(0, 5)" :key="p" class="dr-tip-code">{{ p }}</code>
            <span v-if="hoverFlow.policyNames.length > 5" class="sk-caption"> +{{ hoverFlow.policyNames.length - 5 }}</span>
          </div>
        </div>
      </div>

      <!-- v0.8.12.6 fix 4: orphan BSL fold-out -->
      <div v-if="orphanBSLs.length" class="dr-orphans">
        <button class="dr-orphan-toggle" @click="showOrphans = !showOrphans">
          {{ showOrphans ? '▼' : '▶' }} {{ orphanBSLs.length }} {{ t('topology.inactiveBSLs') }}
        </button>
        <div v-if="showOrphans" class="dr-orphan-list">
          <span v-for="b in orphanBSLs" :key="b.name" class="dr-orphan-chip" @click="goToStorage">
            {{ b.name }}
            <span class="dr-orphan-meta">{{ b.provider }} · {{ b.phase }}</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getTopology } from '../api/velero'

const { t } = useI18n()
const router = useRouter()

// ──────────────────────────────────────────────────────────────────
// Collapse state (persisted)
// ──────────────────────────────────────────────────────────────────
const STORAGE_KEY = 'supkube.drTopology.expanded'
const expanded = ref(localStorage.getItem(STORAGE_KEY) === 'true')

function toggle() {
  expanded.value = !expanded.value
  try { localStorage.setItem(STORAGE_KEY, String(expanded.value)) } catch (_) {}
}

// ──────────────────────────────────────────────────────────────────
// Data
// ──────────────────────────────────────────────────────────────────
const loading = ref(true)
const clusters = ref([])
const bsls = ref([])
const flows = ref([])
const score = ref({})
const summary = ref({ clusterCount: 0, policyCount: 0, rpCount: 0 })
const showOrphans = ref(false)

async function fetchTopology() {
  loading.value = true
  try {
    const res = await getTopology()
    clusters.value = res.data.clusters || []
    bsls.value = res.data.bsls || []
    flows.value = res.data.flows || []
    score.value = res.data.score || {}
    summary.value = res.data.summary || {}
  } catch (e) {
    console.warn('topology fetch failed', e?.response?.status)
  } finally {
    loading.value = false
  }
}

let timer = null
onMounted(() => {
  fetchTopology()
  timer = setInterval(fetchTopology, 60000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

// ──────────────────────────────────────────────────────────────────
// Navigation (fix D)
// ──────────────────────────────────────────────────────────────────
function goToApplications() { router.push('/applications') }
function goToStorage()      { router.push('/storage') }

// ──────────────────────────────────────────────────────────────────
// Formatting helpers
// ──────────────────────────────────────────────────────────────────
function formatBytes(n) {
  if (!n || n <= 0) return '0 B'
  const u = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0; let v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 ? 0 : v >= 10 ? 1 : 1)} ${u[i]}`
}

function formatAgo(iso) {
  if (!iso) return ''
  const ts = new Date(iso).getTime()
  const diffSec = Math.max(0, (Date.now() - ts) / 1000)
  if (diffSec < 60)   return `${Math.round(diffSec)}s`
  if (diffSec < 3600) return `${Math.round(diffSec / 60)} ${t('topology.minAgo')}`
  if (diffSec < 86400) return `${Math.round(diffSec / 3600)}h`
  return `${Math.round(diffSec / 86400)}d`
}

// ──────────────────────────────────────────────────────────────────
// Active / orphan partition (fix 4)
// ──────────────────────────────────────────────────────────────────
// A BSL is "active" if any flow targets it OR it has RPs. Otherwise
// it's an orphan — folded out of the main canvas to a chip strip
// below; one click expands the list.
const flowsByBSL = computed(() => {
  const m = {}
  for (const f of flows.value) {
    if (!m[f.toBSL]) m[f.toBSL] = []
    m[f.toBSL].push(f)
  }
  return m
})

const activeBSLs = computed(() => {
  return bsls.value.filter((b) => {
    const hasFlow = (flowsByBSL.value[b.name] || []).length > 0
    return hasFlow || b.rpCount > 0
  })
})

const orphanBSLs = computed(() => {
  return bsls.value.filter((b) => {
    const hasFlow = (flowsByBSL.value[b.name] || []).length > 0
    return !hasFlow && b.rpCount === 0
  })
})

// ──────────────────────────────────────────────────────────────────
// Layout math
// ──────────────────────────────────────────────────────────────────
const CLUSTER_W = 280
const BSL_W = 360
const COL_GAP = 180
const CARD_GAP = 16
const CLUSTER_HEADER = 76       // title + meta + version line
const NS_ROW_H = 18
const NS_MAX_VISIBLE = 8
const BSL_H_BASE = 100
const BSL_H_WITH_CAP = 124      // extra room for capacity bar (fix C)
const PAD = 24

const layout = computed(() => {
  if (!clusters.value.length && !activeBSLs.value.length) return null

  // Determine connected ns per cluster (for port markers + flow lookups)
  const connectedNs = new Set()
  const activeNames = new Set(activeBSLs.value.map((b) => b.name))
  for (const f of flows.value) {
    if (activeNames.has(f.toBSL)) connectedNs.add(`${f.fromCluster}|${f.fromNamespace}`)
  }

  // ── Cluster cards
  const clusterCards = clusters.value.map((c) => {
    const nsVisible = c.namespaceNames.slice(0, NS_MAX_VISIBLE)
    const nsRows = nsVisible.map((name, i) => ({
      name,
      y: CLUSTER_HEADER + i * NS_ROW_H,
      hasFlow: connectedNs.has(`${c.id}|${name}`),
    }))
    const truncatedCount = c.namespaceNames.length - nsVisible.length
    const height = Math.max(
      140,
      CLUSTER_HEADER + nsRows.length * NS_ROW_H + (truncatedCount > 0 ? 20 : 12)
    )
    return { ...c, width: CLUSTER_W, height, nsRows, truncatedCount }
  })
  let clusterY = PAD
  for (const c of clusterCards) {
    c.x = PAD
    c.y = clusterY
    clusterY += c.height + CARD_GAP
  }

  // ── BSL cards (only active; orphans are below in HTML, not SVG)
  const bslCards = activeBSLs.value.map((b) => ({
    ...b,
    width: BSL_W,
    height: b.capacityBytes > 0 ? BSL_H_WITH_CAP : BSL_H_BASE,
    hasFlow: (flowsByBSL.value[b.name] || []).length > 0,
  }))
  let bslY = PAD
  const bslX = PAD + CLUSTER_W + COL_GAP
  for (const b of bslCards) {
    b.x = bslX
    b.y = bslY
    bslY += b.height + CARD_GAP
  }

  const width = bslX + BSL_W + PAD
  const height = Math.max(clusterY, bslY, 220) + PAD

  // ── Flow paths
  const flowPaths = []
  for (const f of flows.value) {
    const cluster = clusterCards.find((c) => c.id === f.fromCluster)
    if (!cluster) continue
    const nsIdx = cluster.namespaceNames.indexOf(f.fromNamespace)
    if (nsIdx < 0 || nsIdx >= NS_MAX_VISIBLE) continue
    const bsl = bslCards.find((b) => b.name === f.toBSL)
    if (!bsl) continue

    const sx = cluster.x + cluster.width
    const sy = cluster.y + CLUSTER_HEADER + nsIdx * NS_ROW_H + NS_ROW_H / 2 - 4
    const ex = bsl.x
    const ey = bsl.y + 10
    const cx = (sx + ex) / 2

    const color = bsl.kind === 'local' ? '#3b82f6' : '#8b5cf6'
    const w = Math.min(6, Math.max(1.5, 1 + f.backupCount / 5))
    const dashed = f.backupCount === 0
    // v0.8.12.6 fix A: inline label at curve midpoint shows the policy
    // name. With >1 policy we say "name1 +N". If the line is very short
    // we suppress the label to avoid clipping the cards.
    const first = (f.policyNames && f.policyNames[0]) || ''
    const extra = f.policyNames && f.policyNames.length > 1 ? ` +${f.policyNames.length - 1}` : ''
    const shortLabel = (ex - sx) > 80 ? `${first}${extra}` : ''

    flowPaths.push({
      d: `M ${sx} ${sy} C ${cx} ${sy}, ${cx} ${ey}, ${ex} ${ey}`,
      color, width: w, dashed,
      labelX: cx,
      labelY: (sy + ey) / 2 - 4,
      shortLabel,
      from: f.fromNamespace,
      to: f.toBSL,
      policyNames: f.policyNames || [],
      backupCount: f.backupCount,
      tipStyle: { left: `${cx - 100}px`, top: `${(sy + ey) / 2 + 20}px` },
    })
  }

  return { width, height, clusters: clusterCards, bsls: bslCards, flowPaths }
})

const hoverFlow = ref(null)

// ──────────────────────────────────────────────────────────────────
// 3-2-1-1-0 score chips
// ──────────────────────────────────────────────────────────────────
const scoreRules = computed(() => [
  { label: t('topology.score.three'),     ok: !!score.value.threeCopies,  note: score.value.threeNote },
  { label: t('topology.score.two'),       ok: !!score.value.twoMedia,     note: score.value.twoNote },
  { label: t('topology.score.one'),       ok: !!score.value.oneOffsite,   note: score.value.oneNote },
  { label: t('topology.score.immutable'), ok: !!score.value.oneImmutable, note: score.value.immutableNote },
  { label: t('topology.score.zero'),      ok: !!score.value.zeroErrors,   note: score.value.zeroNote },
])
const scoreTotal = computed(() => scoreRules.value.filter((r) => r.ok).length)
</script>

<style scoped>
.dr-topology {
  background: #fff;
  border: 1px solid var(--sk-border-light, #e5e7eb);
  border-radius: 12px;
  padding: 12px 16px;
  margin-bottom: 16px;
  transition: padding 0.15s ease;
}
.dr-topology.is-collapsed {
  padding: 10px 16px;
}

/* ── Header ─────────────────────────────────────────────────── */
.dr-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  cursor: pointer;
  user-select: none;
}
.dr-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.dr-chevron {
  display: inline-block;
  font-size: 10px;
  color: var(--sk-text-caption, #6b7280);
  transition: transform 0.15s ease;
  width: 14px;
  text-align: center;
}
.dr-chevron.is-open { transform: rotate(90deg); }
.dr-title { margin: 0; }
.dr-subtitle { margin: 2px 0 0 0; }

/* Right side: compact score + counts inline */
.dr-header-score {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.dr-score-label-inline {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.dr-score-text {
  font-size: 11px;
  letter-spacing: 0.5px;
  color: var(--sk-text-caption, #9ca3af);
  text-transform: uppercase;
}
.dr-score-count-inline {
  font-size: 18px;
  font-weight: 700;
  color: var(--sk-primary, #4f46e5);
}
.dr-score-dots-inline {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.dr-score-item-inline {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  cursor: help;
}
.dr-dot { font-size: 14px; line-height: 1; }
.dr-score-item-inline.is-ok .dr-dot { color: #10b981; }
.dr-score-item-inline.is-bad .dr-dot { color: #d1d5db; }
.dr-score-item-inline.is-bad .dr-rule-label { color: var(--sk-text-caption, #9ca3af); }
.dr-header-counts {
  font-size: 12px;
  padding-left: 12px;
  border-left: 1px solid var(--sk-border-light, #e5e7eb);
}

/* ── Body ───────────────────────────────────────────────────── */
.dr-body { margin-top: 12px; }
.dr-canvas {
  position: relative;
  width: 100%;
  overflow-x: auto;
  background: linear-gradient(135deg, #fafbfc 0%, #f5f6fa 100%);
  border-radius: 8px;
  padding: 8px;
}
.dr-svg { display: block; max-width: 100%; height: auto; }

/* ── Cards ─────────────────────────────────────────────────── */
.dr-clickable { cursor: pointer; }
.dr-clickable:hover .dr-card-bg { filter: brightness(0.97); }
.dr-card-bg { fill: #fff; stroke: var(--sk-border-light, #e5e7eb); stroke-width: 1; }
.dr-card-bg-local { fill: #f5f3ff; stroke: #c4b5fd; }
.dr-card-bg-unavailable { fill: #fef2f2; stroke: #fecaca; }
.dr-card-title { font: 600 14px/1 'Inter', sans-serif; fill: var(--sk-text, #1f2937); }
.dr-card-badge { font: 500 11px/1 'Inter', sans-serif; fill: var(--sk-text-caption, #6b7280); }
.dr-card-meta { font: 400 11px/1 'Inter', sans-serif; fill: var(--sk-text-caption, #6b7280); }
.dr-card-chip { font: 500 10px/1 'Inter', sans-serif; }
.dr-chip-lock { fill: #059669; }
.dr-chip-warn { fill: #d97706; }
.dr-chip-status { font: 500 10px/1 'Inter', sans-serif; }
.dr-chip-ok { fill: #059669; }
.dr-chip-bad { fill: #dc2626; }
.dr-mono { font-family: 'SF Mono', Menlo, monospace; font-size: 10px; }

/* ── Capacity bar (fix C) ───────────────────────────────────── */
.dr-cap-track { fill: #e5e7eb; }
.dr-cap-bar   { fill: #4f46e5; }

/* ── Namespace rows in cluster cards ───────────────────────── */
.dr-ns-name { font: 400 12px/1 'Inter', sans-serif; fill: var(--sk-text, #374151); }
.dr-bullet { font-weight: 700; fill: var(--sk-primary, #4f46e5); }
.dr-port { fill: var(--sk-primary, #4f46e5); opacity: 0.7; }

/* ── Flow lines + labels (fix A) ────────────────────────────── */
.dr-flow-line {
  opacity: 0.55;
  transition: opacity 0.15s, stroke-width 0.15s;
}
.dr-flow-line:hover { opacity: 1; cursor: pointer; }
.dr-flow-label {
  font: 400 10px/1 'Inter', sans-serif;
  fill: var(--sk-text-caption, #6b7280);
  pointer-events: none;
}

/* ── Tooltip ───────────────────────────────────────────────── */
.dr-tooltip {
  position: absolute;
  width: 200px;
  background: rgba(31, 41, 55, 0.97);
  color: #fff;
  padding: 8px 10px;
  border-radius: 6px;
  pointer-events: none;
  z-index: 10;
  font-size: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
}
.dr-tip-title { font-weight: 600; margin-bottom: 4px; }
.dr-tip-row { margin-top: 4px; display: flex; flex-wrap: wrap; gap: 4px; }
.dr-tip-code {
  background: rgba(255, 255, 255, 0.12);
  padding: 1px 5px; border-radius: 3px;
  font-size: 10px; font-family: 'SF Mono', Menlo, monospace;
}

/* ── Orphan BSL fold (fix 4) ───────────────────────────────── */
.dr-orphans {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--sk-border-light, #e5e7eb);
}
.dr-orphan-toggle {
  background: none;
  border: none;
  font-size: 12px;
  color: var(--sk-text-caption, #6b7280);
  cursor: pointer;
  padding: 4px 0;
}
.dr-orphan-toggle:hover { color: var(--sk-primary, #4f46e5); }
.dr-orphan-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.dr-orphan-chip {
  padding: 4px 10px;
  background: #f9fafb;
  border: 1px solid var(--sk-border-light, #e5e7eb);
  border-radius: 6px;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: pointer;
  transition: all 0.15s;
}
.dr-orphan-chip:hover {
  border-color: var(--sk-primary, #4f46e5);
  background: #fff;
}
.dr-orphan-meta {
  font-size: 10px;
  color: var(--sk-text-caption, #9ca3af);
}
</style>
