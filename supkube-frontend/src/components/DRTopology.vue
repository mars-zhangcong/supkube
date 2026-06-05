<!--
  DRTopology (PRD-010 / ADR-040 — v2)
  ───────────────────────────────────────────────────────────────────────
  DR-topology card for the Dashboard. Self-rendered SVG (no d3 / no
  ECharts) — gives us pixel-precise control over rich cards (chips,
  capacity bars, hover tooltips, Layer badges, click navigation).

  ADR-040 visual contract:
    D1  6 node families (cluster / snapshot / bsl-local / bsl-cloud /
        copy / disabled) — mutually exclusive color systems
    D2  5 flow-arrow styles (snapshot / export / import / copy / restore)
        locked enum, NEVER add a 6th type without ADR-040 v2
    D3  Layer 1-5 badges in node top-right corner
    D4  Layer 5 = TOP verification badge (4 states: ok/warn/error/muted)
        NOT a node — see ADR-040 D4 rationale
    D5  ALL colors via var(--svg-*) from styles/svg-topology.css —
        ZERO #RRGGBB literals in this file (CI verifies via TC-TOPO-005)

  Header (always visible):
    > DR Topology  [score chips inline]  N clusters · N policies · N RPs
        click target = chevron toggle

  Body (when expanded):
    - L5 verification badge across the top (D4)
    - Cluster cards (left)
    - BSL / Layer-1 Snapshot / Layer-4 Copy cards (right, by layer)
    - Flow lines: typed per flows[].type (D2)
    - Orphan BSLs (no flow + no RPs) fold into a chip strip below

  Persistence: collapsed/expanded → localStorage 'supkube.drTopology.expanded'
-->

<template>
  <div class="dr-topology sk-card" :class="{ 'is-collapsed': !expanded }" data-testid="dr-topology-root">
    <!-- ════ Header (always visible, click to toggle) ════ -->
    <div class="dr-header" @click="toggle">
      <div class="dr-header-left">
        <span class="dr-chevron" :class="{ 'is-open': expanded }">▶</span>
        <div class="dr-header-text">
          <h3 class="sk-h2 dr-title">{{ t('topology.title') }}</h3>
          <p v-if="expanded" class="sk-caption dr-subtitle">{{ t('topology.subtitle') }}</p>
        </div>
      </div>

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
      <!-- D4: Layer-5 verification badge (top, global, 4 states) -->
      <div class="dr-l5-badge-row" data-testid="l5-badge-row">
        <div
          class="dr-l5-badge"
          :class="[`svg-l5-badge-${l5State.state}`]"
          :title="l5State.tooltip"
          :data-state="l5State.state"
          data-testid="l5-badge"
          @click="goToDRDrill"
        >
          <span class="dr-l5-badge-icon">{{ l5State.icon }}</span>
          <span class="dr-l5-badge-text" :class="[`svg-l5-badge-text-${l5State.state}`]">
            {{ t(`topology.l5.${l5State.state}`) }}
          </span>
        </div>
      </div>

      <div ref="canvasRef" class="dr-canvas svg-canvas-bg">
        <svg
          v-if="layout"
          :viewBox="`0 0 ${layout.width} ${layout.height}`"
          :width="layout.width"
          :height="layout.height"
          class="dr-svg"
          preserveAspectRatio="xMidYMid meet"
          data-testid="dr-svg"
        >
          <!-- Arrow marker defs (D2): filled triangle = movement,
               hollow triangle = copy-only data flow. -->
          <defs>
            <marker id="svg-marker-filled" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M0,0 L8,4 L0,8 Z" class="svg-marker-filled-shape" />
            </marker>
            <marker id="svg-marker-hollow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M0,0 L8,4 L0,8 Z" fill="none" stroke="currentColor" stroke-width="1" />
            </marker>
          </defs>

          <!-- Flow lines (drawn first so cards sit on top) -->
          <g class="dr-flows">
            <g v-for="(flow, i) in layout.flowPaths" :key="`flow-${i}`">
              <path
                :d="flow.d"
                :class="`dr-flow-line svg-arrow-${flow.type}`"
                :stroke-width="flow.width"
                fill="none"
                :data-flow-type="flow.type"
                @mouseenter="hoverFlow = flow"
                @mouseleave="hoverFlow = null"
              />
              <text
                :x="flow.labelX"
                :y="flow.labelY"
                class="dr-flow-label"
                text-anchor="middle"
              >{{ flow.shortLabel }}</text>
            </g>
          </g>

          <!-- Cluster cards (D1: blue family) -->
          <g class="dr-clusters">
            <g
              v-for="(cluster, ci) in layout.clusters"
              :key="`cluster-${ci}`"
              :transform="`translate(${cluster.x}, ${cluster.y})`"
              class="dr-clickable"
              :data-node-type="cluster.disabled ? 'disabled' : 'cluster'"
              data-testid="node-cluster"
              @click="goToApplications"
            >
              <rect
                class="dr-card-bg"
                :class="cluster.disabled ? 'svg-node-disabled' : 'svg-node-cluster'"
                :width="cluster.width"
                :height="cluster.height"
                rx="8"
              />
              <text :x="14" :y="24" class="dr-card-title">{{ cluster.name }}</text>
              <text v-if="cluster.isCurrent" :x="cluster.width - 14" :y="24" text-anchor="end" class="dr-card-badge">
                ★ {{ t('topology.current') }}
              </text>
              <text :x="14" :y="44" class="dr-card-meta">
                <tspan v-if="cluster.k8sVersion">k8s {{ cluster.k8sVersion }}</tspan>
                <tspan v-if="cluster.k8sVersion && cluster.nodeCount" dx="6">·</tspan>
                <tspan v-if="cluster.nodeCount" dx="6">{{ cluster.nodeCount }} {{ t('topology.nodes') }}</tspan>
              </text>
              <text :x="14" :y="62" class="dr-card-meta">
                {{ cluster.namespaceNames.length }} {{ t('topology.namespacesShort') }} · {{ cluster.policyCount }} {{ t('topology.policiesShort') }}
              </text>

              <g
                v-for="(ns, ni) in cluster.nsRows"
                :key="`ns-${ci}-${ni}`"
                :transform="`translate(0, ${ns.y})`"
              >
                <text :x="22" :y="14" class="dr-ns-name">
                  <tspan class="dr-bullet">·</tspan>
                  {{ ns.name === '*' ? t('topology.allNamespaces') : ns.name }}
                </text>
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

          <!-- BSL cards (D1: bsl-local orange / bsl-cloud purple / copy pink / snapshot teal) -->
          <g class="dr-bsls">
            <g
              v-for="(bsl, bi) in layout.bsls"
              :key="`bsl-${bi}`"
              :transform="`translate(${bsl.x}, ${bsl.y})`"
              class="dr-clickable"
              :data-node-type="bsl.nodeType"
              :data-testid="`node-${bsl.nodeType}`"
              @click="goToStorage"
            >
              <rect
                class="dr-card-bg"
                :class="bslCardClass(bsl)"
                :width="bsl.width"
                :height="bsl.height"
                rx="8"
              />
              <circle v-if="bsl.hasFlow" :cx="6" :cy="10" r="4" class="dr-port" />

              <text :x="18" :y="22" class="dr-card-title">{{ bsl.name }}</text>
              <text :x="bsl.width - 12" :y="22" text-anchor="end" class="dr-card-badge">
                {{ nodeIcon(bsl.nodeType) }} {{ nodeKindLabel(bsl) }}
              </text>
              <text :x="18" :y="42" class="dr-card-meta">
                {{ bsl.provider }} · {{ bsl.rpCount }} {{ t('topology.restorePointsShort') }} · {{ bsl.backedupNs }} {{ t('topology.nsCovered') }}
              </text>
              <text v-if="bsl.bucket" :x="18" :y="58" class="dr-card-meta dr-mono">
                {{ bsl.bucket }}
              </text>
              <text :x="bsl.width - 12" :y="42" text-anchor="end" class="dr-card-meta">
                <tspan v-if="bsl.lastBackupAt">{{ t('topology.lastBackup') }} {{ formatAgo(bsl.lastBackupAt) }}</tspan>
                <tspan v-else>{{ t('topology.neverBackedUp') }}</tspan>
              </text>

              <g v-if="bsl.capacityBytes > 0" :transform="`translate(18, 60)`">
                <rect :width="bsl.width - 36" :height="6" rx="3" class="svg-cap-track" />
                <rect
                  :width="(bsl.width - 36) * (bsl.usedBytes / bsl.capacityBytes || 0)"
                  :height="6"
                  rx="3"
                  class="svg-cap-bar"
                />
                <text :x="0" :y="20" class="dr-card-meta">
                  {{ formatBytes(bsl.usedBytes) }} / {{ formatBytes(bsl.capacityBytes) }}
                </text>
              </g>

              <text v-if="bsl.objectLockEnabled" :x="18" :y="bsl.height - 12" class="dr-card-chip svg-chip-lock">
                🛡 Object Lock ({{ bsl.objectLockMode }})
              </text>
              <text v-else :x="18" :y="bsl.height - 12" class="dr-card-chip svg-chip-warn">
                No Object Lock
              </text>
              <text :x="bsl.width - 12" :y="bsl.height - 12" text-anchor="end"
                    :class="['dr-chip-status', bsl.phase === 'Available' ? 'svg-chip-ok' : 'svg-chip-bad']">
                {{ bsl.phase || 'Unknown' }}
              </text>

              <!-- D3: Layer badge at top-right corner of every active node -->
              <g v-if="bsl.layer" class="dr-layer-badge" :transform="`translate(${bsl.width - 32}, ${-8})`"
                 :data-layer="bsl.layer" :data-testid="`badge-${bsl.layer.toLowerCase()}`">
                <rect width="28" height="16" rx="3" :class="`svg-badge-${bsl.layer.toLowerCase()}`">
                  <title>{{ t(`topology.layer.${bsl.layer.toLowerCase()}.tooltip`) }}</title>
                </rect>
                <text x="14" y="12" text-anchor="middle" class="svg-badge-layer-text">{{ bsl.layer }}</text>
              </g>
            </g>
          </g>
        </svg>

        <div v-if="hoverFlow" class="dr-tooltip" :style="hoverFlow.tipStyle">
          <div class="dr-tip-title">{{ hoverFlow.from }} → {{ hoverFlow.to }}</div>
          <div class="dr-tip-row">
            <span class="dr-tip-type" :class="`svg-arrow-${hoverFlow.type}`">{{ t(`topology.flowType.${hoverFlow.type}`) }}</span>
          </div>
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

// Side-effect import: design tokens for SVG (ADR-040 D5).
// MUST be imported once; component CSS only references var(--svg-*).
import '../styles/svg-topology.css'

const { t } = useI18n()
const router = useRouter()

// ──────────────────────────────────────────────────────────────────
// flows[].type locked enum — ADR-040 D2 (Rule C: backend-owned schema)
// Adding a 6th type requires ADR-040 v2 + backend aggregator change.
// ──────────────────────────────────────────────────────────────────
const FLOW_TYPES = ['snapshot', 'export', 'import', 'copy', 'restore']

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
const posture = ref({ layer5Status: 'muted', lastDrillAt: null, nextDrillAt: null })
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
    posture.value = res.data.posture || { layer5Status: 'muted' }
  } catch (e) {
    // eslint-disable-next-line no-console
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
// Navigation
// ──────────────────────────────────────────────────────────────────
function goToApplications() { router.push('/applications') }
function goToStorage()      { router.push('/storage') }
function goToDRDrill()      { router.push('/dr-drill/history') }

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
// Node-family classification (D1) — derived from existing data shape.
// Once backend ships PRD-010 §4.8 (localSnapshots / backupCopies),
// those will become their own BSL.nodeType values.
// ──────────────────────────────────────────────────────────────────
function classifyBSL(b) {
  // Backend extension (PRD-010 §4.8 forward-compat): b.role hints L1/L4
  if (b.role === 'snapshot') return { nodeType: 'snapshot',  layer: 'L1' }
  if (b.role === 'copy')     return { nodeType: 'copy',      layer: 'L4' }
  // Existing kind = local | cloud (PRD-010 §4.1, with color reassignment)
  if (b.kind === 'local')    return { nodeType: 'bsl-local', layer: 'L2' }
  return { nodeType: 'bsl-cloud', layer: 'L3' }
}

function bslCardClass(b) {
  if (b.phase && b.phase !== 'Available') return 'svg-card-bg-unavailable'
  return `svg-node-${b.nodeType}`
}

function nodeIcon(nodeType) {
  return {
    'snapshot':  '📷',
    'bsl-local': '🏠',
    'bsl-cloud': '☁',
    'copy':      '📋',
  }[nodeType] || ''
}

function nodeKindLabel(b) {
  if (b.nodeType === 'snapshot')  return t('topology.layer.l1.short')
  if (b.nodeType === 'bsl-local') return t('topology.local')
  if (b.nodeType === 'bsl-cloud') return t('topology.cloud')
  if (b.nodeType === 'copy')      return t('topology.layer.l4.short')
  return ''
}

// ──────────────────────────────────────────────────────────────────
// D4 — Layer 5 verification badge state
// 4 states per ADR-040 D4: ok / warn / error / muted
// ──────────────────────────────────────────────────────────────────
const l5State = computed(() => {
  const s = posture.value?.layer5Status || 'muted'
  const norm = ['ok', 'warn', 'error', 'muted'].includes(s) ? s : 'muted'
  const icons = { ok: '✓', warn: '⚠', error: '✗', muted: '—' }
  let tooltip = t(`topology.l5.${norm}`)
  if (posture.value?.lastDrillAt) {
    tooltip += ` · ${t('topology.lastBackup')} ${formatAgo(posture.value.lastDrillAt)}`
  }
  return { state: norm, icon: icons[norm], tooltip }
})

// ──────────────────────────────────────────────────────────────────
// Active / orphan BSL partition
// ──────────────────────────────────────────────────────────────────
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
const CLUSTER_HEADER = 76
const NS_ROW_H = 18
const NS_MAX_VISIBLE = 8
const BSL_H_BASE = 100
const BSL_H_WITH_CAP = 124
const PAD = 24

// Map flows[].type → arrow class. Defensive default for unknown type:
// fall back to 'export' (most common) but log a console warning so we
// notice if backend ships an unannounced 6th type (ADR-040 D2 lock).
function normalizeFlowType(rawType) {
  if (FLOW_TYPES.includes(rawType)) return rawType
  if (rawType) {
    // eslint-disable-next-line no-console
    console.warn(`DRTopology: unknown flows[].type='${rawType}', falling back to 'export'. ADR-040 D2 enum is locked.`)
  }
  return 'export'
}

const layout = computed(() => {
  if (!clusters.value.length && !activeBSLs.value.length) return null

  const connectedNs = new Set()
  const activeNames = new Set(activeBSLs.value.map((b) => b.name))
  for (const f of flows.value) {
    if (activeNames.has(f.toBSL)) connectedNs.add(`${f.fromCluster}|${f.fromNamespace}`)
  }

  // Cluster cards
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
    return { ...c, width: CLUSTER_W, height, nsRows, truncatedCount, disabled: false }
  })
  let clusterY = PAD
  for (const c of clusterCards) {
    c.x = PAD
    c.y = clusterY
    clusterY += c.height + CARD_GAP
  }

  // BSL / Snapshot / Copy cards
  const bslCards = activeBSLs.value.map((b) => {
    const cls = classifyBSL(b)
    return {
      ...b,
      ...cls,
      width: BSL_W,
      height: b.capacityBytes > 0 ? BSL_H_WITH_CAP : BSL_H_BASE,
      hasFlow: (flowsByBSL.value[b.name] || []).length > 0,
    }
  })
  let bslY = PAD
  const bslX = PAD + CLUSTER_W + COL_GAP
  for (const b of bslCards) {
    b.x = bslX
    b.y = bslY
    bslY += b.height + CARD_GAP
  }

  const width = bslX + BSL_W + PAD
  const height = Math.max(clusterY, bslY, 220) + PAD

  // Flow paths — type drives styling via CSS class (ADR-040 D2)
  const flowPaths = []
  for (const f of flows.value) {
    const cluster = clusterCards.find((c) => c.id === f.fromCluster)
    if (!cluster) continue
    const nsIdx = cluster.namespaceNames.indexOf(f.fromNamespace)
    if (nsIdx < 0 || nsIdx >= NS_MAX_VISIBLE) continue
    const bsl = bslCards.find((b) => b.name === f.toBSL)
    if (!bsl) continue

    const type = normalizeFlowType(f.type || 'export')
    const sx = cluster.x + cluster.width
    const sy = cluster.y + CLUSTER_HEADER + nsIdx * NS_ROW_H + NS_ROW_H / 2 - 4
    const ex = bsl.x
    const ey = bsl.y + 10
    const cx = (sx + ex) / 2

    // restore is the only reverse-direction arrow → curve back-and-up
    const d = type === 'restore'
      ? `M ${ex} ${ey} C ${cx} ${ey}, ${cx} ${sy - 30}, ${sx} ${sy}`
      : `M ${sx} ${sy} C ${cx} ${sy}, ${cx} ${ey}, ${ex} ${ey}`

    const w = Math.min(6, Math.max(1.5, 1 + (f.backupCount || 0) / 5))
    const first = (f.policyNames && f.policyNames[0]) || ''
    const extra = f.policyNames && f.policyNames.length > 1 ? ` +${f.policyNames.length - 1}` : ''
    const shortLabel = (ex - sx) > 80 ? `${first}${extra}` : ''

    flowPaths.push({
      d,
      type,
      width: w,
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

// Exported for tests (TC-TOPO-001..005). Not part of public component API.
defineExpose({ FLOW_TYPES, l5State, classifyBSL, normalizeFlowType })
</script>

<style scoped>
/* All colors in this scoped block are var(--sk-*) or var(--svg-*) —
   any #RRGGBB literal here is a TC-TOPO-005 violation. */

.dr-topology {
  background: var(--sk-bg-page);
  border: 1px solid var(--sk-border-light);
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
  color: var(--sk-text-caption);
  transition: transform 0.15s ease;
  width: 14px;
  text-align: center;
}
.dr-chevron.is-open { transform: rotate(90deg); }
.dr-title { margin: 0; }
.dr-subtitle { margin: 2px 0 0 0; }

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
  color: var(--sk-text-caption);
  text-transform: uppercase;
}
.dr-score-count-inline {
  font-size: 18px;
  font-weight: 700;
  color: var(--sk-primary);
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
.dr-score-item-inline.is-ok .dr-dot { color: var(--svg-score-ok-fg); }
.dr-score-item-inline.is-bad .dr-dot { color: var(--svg-score-bad-fg); }
.dr-score-item-inline.is-bad .dr-rule-label { color: var(--sk-text-caption); }
.dr-header-counts {
  font-size: 12px;
  padding-left: 12px;
  border-left: 1px solid var(--sk-border-light);
}

/* ── Body ───────────────────────────────────────────────────── */
.dr-body { margin-top: 12px; }
.dr-canvas {
  position: relative;
  width: 100%;
  overflow-x: auto;
  border-radius: 8px;
  padding: 8px;
}
.dr-svg { display: block; max-width: 100%; height: auto; }

/* ── D4: Layer-5 verification badge (top-of-canvas, click → DR Drill) */
.dr-l5-badge-row {
  display: flex;
  justify-content: center;
  margin-bottom: 8px;
}
.dr-l5-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 12px;
  border: 1px solid;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  user-select: none;
  transition: filter 0.15s;
}
.dr-l5-badge:hover { filter: brightness(0.95); }
.dr-l5-badge-icon { font-size: 14px; line-height: 1; }
.dr-l5-badge-text { line-height: 1.2; }

/* ── Cards ─────────────────────────────────────────────────── */
.dr-clickable { cursor: pointer; }
.dr-clickable:hover .dr-card-bg { filter: brightness(0.97); }
.dr-card-bg { stroke-width: 1; }
.dr-card-title { font: 600 14px/1 'Inter', sans-serif; fill: var(--sk-text); }
.dr-card-badge { font: 500 11px/1 'Inter', sans-serif; fill: var(--sk-text-caption); }
.dr-card-meta  { font: 400 11px/1 'Inter', sans-serif; fill: var(--sk-text-caption); }
.dr-card-chip  { font: 500 10px/1 'Inter', sans-serif; }
.dr-chip-status { font: 500 10px/1 'Inter', sans-serif; }
.dr-mono { font-family: 'SF Mono', Menlo, monospace; font-size: 10px; }

/* ── Namespace rows ───────────────────────────────────────── */
.dr-ns-name { font: 400 12px/1 'Inter', sans-serif; fill: var(--sk-text); }
.dr-bullet { font-weight: 700; fill: var(--sk-primary); }
.dr-port { fill: var(--sk-primary); opacity: 0.7; }

/* ── Flow lines (D2) — color/dash/marker driven by .svg-arrow-* class */
.dr-flow-line {
  opacity: 0.55;
  transition: opacity 0.15s, stroke-width 0.15s;
  fill: none;
}
.dr-flow-line:hover { opacity: 1; cursor: pointer; }
.dr-flow-label {
  font: 400 10px/1 'Inter', sans-serif;
  fill: var(--sk-text-caption);
  pointer-events: none;
}

/* Markers use currentColor so they inherit the .svg-arrow-* stroke. */
.svg-marker-filled-shape { fill: currentColor; }

/* ── Tooltip ───────────────────────────────────────────────── */
.dr-tooltip {
  position: absolute;
  width: 220px;
  background: var(--svg-tooltip-bg);
  color: var(--svg-tooltip-fg);
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
  background: var(--svg-tooltip-code-bg);
  padding: 1px 5px; border-radius: 3px;
  font-size: 10px; font-family: 'SF Mono', Menlo, monospace;
}
.dr-tip-type {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  /* Type chip uses arrow stroke color via .svg-arrow-* class on element */
  background: var(--svg-tooltip-code-bg);
}

/* ── Orphan BSL fold ───────────────────────────────────────── */
.dr-orphans {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--sk-border-light);
}
.dr-orphan-toggle {
  background: none;
  border: none;
  font-size: 12px;
  color: var(--sk-text-caption);
  cursor: pointer;
  padding: 4px 0;
}
.dr-orphan-toggle:hover { color: var(--sk-primary); }
.dr-orphan-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.dr-orphan-chip {
  padding: 4px 10px;
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border-light);
  border-radius: 6px;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: pointer;
  transition: all 0.15s;
}
.dr-orphan-chip:hover {
  border-color: var(--sk-primary);
  background: var(--sk-bg-page);
}
.dr-orphan-meta {
  font-size: 10px;
  color: var(--sk-text-caption);
}
</style>
