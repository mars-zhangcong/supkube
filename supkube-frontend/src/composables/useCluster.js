// useCluster (v0.9.0 MC1)
// ────────────────────────────────────────────────────────────────────
// Singleton state for the Multi-Cluster Manager:
//   - registry: list of all clusters known to SupKube
//   - active: which cluster the SPA is currently scoped to
//
// "Active" is the URL `?cluster=<name>` source-of-truth. We mirror it
// into localStorage so a tab-reload without query params restores the
// last selection. The literal value `_mcm` (Multi-Cluster Manager mode)
// is reserved — it means "aggregated view across all clusters".
//
// Backend wiring:
//   When `active != _mcm`, axios attaches header `X-Supkube-Cluster: <name>`
//   so the backend routes the request via the remote kubeconfig. v0.9.0
//   MVP single-cluster requests omit the header entirely (`active == ''`
//   when only one cluster exists, or `active == 'this-cluster'`).
//
// Progressive disclosure rule:
//   showModeSwitcher = clusters.length >= 2
//   Single-cluster installs see no Mode Switcher; Settings → Clusters
//   tab is the only entry to add a second one (which then unlocks the
//   Mode Switcher).

import { ref, computed, watch } from 'vue'
import { listClusters } from '../api/velero'

const STORAGE_KEY = 'supkube.activeCluster'
const MCM_ID = '_mcm'

// Singleton state shared across components.
const clusters = ref([])
const loading = ref(false)
const loadError = ref('')
const bootstrapped = ref(false)

// active = the cluster name currently selected. '' (empty) and
// 'this-cluster' are equivalent — both mean "the in-cluster SupKube
// instance". `_mcm` = Multi-Cluster Manager aggregated view.
const active = ref(localStorage.getItem(STORAGE_KEY) || '')

// ─────────────────────────────────────────────────────────────────────
// Derived
// ─────────────────────────────────────────────────────────────────────

// v0.9.0.1 fix #1 — show the dropdown UNCONDITIONALLY once the registry
// has loaded. Single-cluster users still get useful actions in the menu
// (View Kubeconfig, "+ Add Cluster"), and the "static label that does
// nothing" UX bug is gone. Falling back to a static label only while the
// initial fetch is in-flight prevents a flash-of-empty-dropdown.
const showModeSwitcher = computed(() => bootstrapped.value)

const isMCM = computed(() => active.value === MCM_ID)

// activeCluster: the resolved DTO for the current active selection.
// null when in MCM mode or when nothing matches (e.g. after deletion).
const activeCluster = computed(() => {
  if (isMCM.value) return null
  const n = active.value || 'this-cluster'
  return clusters.value.find((c) => c.name === n) || null
})

// label: short string the sidebar header displays.
const activeLabel = computed(() => {
  if (isMCM.value) return 'Multi-Cluster Manager'
  const c = activeCluster.value
  return c?.displayName || c?.name || 'this-cluster'
})

// ─────────────────────────────────────────────────────────────────────
// Mutators
// ─────────────────────────────────────────────────────────────────────

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await listClusters()
    clusters.value = res.data.items || []
  } catch (e) {
    loadError.value = e?.response?.data?.error || e.message || 'fetch failed'
    // Don't blow away the existing list — keep showing what we had
    // so a transient backend hiccup doesn't strip the sidebar.
  } finally {
    loading.value = false
    bootstrapped.value = true
  }
}

// setActive: switch the current cluster context. Persists to
// localStorage AND syncs the URL `?cluster=...` if a router is given.
// Router argument is optional (callers from non-router contexts can
// skip it; the URL just won't update).
function setActive(name, router) {
  active.value = name || ''
  try { localStorage.setItem(STORAGE_KEY, name || '') } catch (_) {}
  if (router) {
    // Replace rather than push so the back button doesn't accumulate
    // cluster-switch entries on every navigation.
    router.replace({ query: { ...router.currentRoute.value.query, cluster: name || undefined } })
  }
}

// hydrateFromRoute: called once at app boot by the router guard. If the
// URL has ?cluster=... that wins; otherwise localStorage; otherwise
// default to '' (current cluster).
function hydrateFromRoute(route) {
  const fromUrl = route?.query?.cluster
  if (fromUrl) {
    active.value = String(fromUrl)
    try { localStorage.setItem(STORAGE_KEY, String(fromUrl)) } catch (_) {}
  }
}

// Returns the value to attach as `X-Supkube-Cluster` header. Empty +
// 'this-cluster' both mean "no remote routing"; axios skips the header.
function activeHeaderValue() {
  const v = active.value
  if (!v || v === 'this-cluster' || v === MCM_ID) return ''
  return v
}

export function useCluster() {
  return {
    // state
    clusters,
    active,
    activeCluster,
    activeLabel,
    isMCM,
    showModeSwitcher,
    loading,
    loadError,
    bootstrapped,
    // constants
    MCM_ID,
    // actions
    refresh,
    setActive,
    hydrateFromRoute,
    activeHeaderValue
  }
}
