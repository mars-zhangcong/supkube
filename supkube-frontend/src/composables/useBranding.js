// useBranding (v0.8.11) — reactive white-label customization.
//
// One shared store across the SPA so every page (sidebar, header bar,
// browser tab title, favicon) renders the same product identity.
// The cluster-wide source of truth is the supkube-settings ConfigMap;
// we fetch once on app boot and re-fetch whenever an admin saves the
// Settings → Branding panel.
//
// Why not localStorage:
//   Branding is cluster-state, not per-browser. An OEM admin who
//   rebrands the cluster expects every operator to see the new logo
//   immediately, not when they next clear cookies.
//
// Why not Vuex/Pinia:
//   This is a 4-field flat record. A module would be over-engineered.
//   Vue's reactive() is enough.

import { reactive, readonly } from 'vue'
import { getBranding } from '../api/velero'

// Built-in defaults — kept simple. If the bundled assets later change
// path (e.g. moved out of /public), update these in one place.
// v0.9.1.3 fix (Mars 2026-05-26 demo): defaults were "/favicon.svg" but
// the actual files bundled in public/ are "supkube-logo.svg" and
// "supkube-favicon.svg" — the wrong path made fresh installs show a
// broken-image icon in the sidebar (browser 404'd, then rendered the
// browser's broken-image placeholder). Customer-visible first-impression
// bug. Fixed by pointing defaults at the real bundled paths.
const DEFAULT_PRODUCT_NAME    = 'SupKube'
const DEFAULT_LOGO_URL        = '/supkube-logo.svg'      // bundled built-in (see supkube-frontend/public/)
const DEFAULT_FAVICON_URL     = '/supkube-favicon.svg'   // matches index.html link rel="icon"
const PAGE_TITLE_TAIL         = '— Kubernetes Data Protection'

// One module-level state object. Imports from this file all receive
// references to the same reactive instance, so updates in Settings
// propagate to every consumer (App.vue, sidebar, page title) without
// any event bus.
const state = reactive({
  productName:    DEFAULT_PRODUCT_NAME,
  logoUrl:        DEFAULT_LOGO_URL,
  faviconUrl:     DEFAULT_FAVICON_URL,
  // v0.8.11.1: empty = use tokens.css default --sk-primary. Otherwise
  // overrides the --sk-primary CSS variable globally.
  primaryColor:   '',
  loaded:         false,
  loadError:      ''
})

// hexToHoverAndBg derives the two companion CSS variables we use
// alongside --sk-primary:
//   --sk-primary-hover     ≈ 15% darker
//   --sk-primary-bg-hover  ≈ very light tint (used for hover backgrounds)
//   --sk-primary-active    ≈ slightly darker tint (used for active state)
//
// Tuned so any picker hex produces the same interaction-feel as the
// default indigo palette. Pure JS — no colour library.
function deriveCompanions(hex) {
  // accept #abc or #aabbcc
  let h = hex.replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16)
  const g = parseInt(h.slice(2, 4), 16)
  const b = parseInt(h.slice(4, 6), 16)
  const clamp = (v) => Math.max(0, Math.min(255, Math.round(v)))
  const toHex = (v) => clamp(v).toString(16).padStart(2, '0')
  // darken 15% for hover text
  const hover = `#${toHex(r * 0.85)}${toHex(g * 0.85)}${toHex(b * 0.85)}`
  // light tint (mix with white ~92%) for hover background
  const mix = (c, w) => c * (1 - w) + 255 * w
  const bgHover = `#${toHex(mix(r, 0.92))}${toHex(mix(g, 0.92))}${toHex(mix(b, 0.92))}`
  const active  = `#${toHex(mix(r, 0.85))}${toHex(mix(g, 0.85))}${toHex(mix(b, 0.85))}`
  return { hover, bgHover, active }
}

// applyDocumentSideEffects pushes the current state into the DOM —
// <title> and the favicon <link>. Called whenever state changes.
//
// We update the existing <link rel="icon"> element (created in
// index.html). Creating new ones is unreliable across browsers
// because some cache the first one they saw.
function applyDocumentSideEffects() {
  document.title = `${state.productName} ${PAGE_TITLE_TAIL}`
  // Update or create favicon link. We look for the standard "icon"
  // rel first, then fall back to "shortcut icon" which IE/old Safari
  // used to require.
  let link = document.querySelector("link[rel~='icon']")
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = state.faviconUrl || DEFAULT_FAVICON_URL

  // v0.8.11.1: apply primaryColor as --sk-primary override on <html>.
  // Empty string = remove override → tokens.css default reasserts.
  const root = document.documentElement
  if (state.primaryColor) {
    const { hover, bgHover, active } = deriveCompanions(state.primaryColor)
    root.style.setProperty('--sk-primary', state.primaryColor)
    root.style.setProperty('--sk-primary-hover', hover)
    root.style.setProperty('--sk-primary-bg-hover', bgHover)
    root.style.setProperty('--sk-primary-active', active)
  } else {
    root.style.removeProperty('--sk-primary')
    root.style.removeProperty('--sk-primary-hover')
    root.style.removeProperty('--sk-primary-bg-hover')
    root.style.removeProperty('--sk-primary-active')
  }
}

// fetchBranding pulls the cluster-wide branding once. Public so the
// Settings → Branding panel can call it after a successful save to
// refresh the rest of the UI without a full reload.
//
// Failure is non-fatal: defaults stay in effect, and the caller can
// inspect state.loadError. We never throw because branding isn't
// critical for any flow other than rendering.
async function fetchBranding() {
  try {
    const res = await getBranding()
    const d = res?.data || {}
    state.productName  = d.productName || DEFAULT_PRODUCT_NAME
    state.logoUrl      = d.logoDataUrl || DEFAULT_LOGO_URL
    state.faviconUrl   = d.faviconDataUrl || DEFAULT_FAVICON_URL
    state.primaryColor = d.primaryColor || ''
    state.loadError    = ''
  } catch (e) {
    state.loadError = e?.response?.data?.error || e?.message || 'fetch failed'
  } finally {
    state.loaded = true
    applyDocumentSideEffects()
  }
}

// useBranding is the public hook. Returns a readonly reactive object
// (consumers can't mutate it directly; they must call fetchBranding
// to refresh from the server).
export function useBranding() {
  return {
    branding: readonly(state),
    refresh:  fetchBranding
  }
}

// Boot side-effect: kick off the initial fetch as soon as this module
// loads. Safe to import multiple times — module evaluation is
// idempotent in ES modules.
//
// We don't await; the SPA renders with defaults instantly, then
// upgrades to the custom brand as soon as the response lands
// (typically <100ms on first paint).
//
// v0.8.11.2 fix: skip on /login + /auth/callback. These routes don't
// need branding (they show the bundled SupKube logo regardless), and
// firing this 401 during the OIDC code-exchange used to trigger the
// 401-redirect interceptor and kill the callback. The api/velero.js
// interceptor now also skips redirect on /auth/callback, but
// preventing the unnecessary request altogether is cleaner.
const _bootPath = typeof window !== 'undefined' ? window.location.pathname : ''
if (_bootPath !== '/login' && _bootPath !== '/auth/callback') {
  fetchBranding()
}
