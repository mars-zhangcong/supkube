// useAuth (v0.8.5)
// ────────────────
// Central auth state for SupKube's SPA.
//
// Storage strategy:
//   - idToken / refreshToken / user → localStorage (survive tab close)
//   - state nonce → sessionStorage (per-tab CSRF defense for OIDC flow)
//
// Lifecycle:
//   1. App boots → useAuth.bootstrap() — calls /auth/me to see if our
//      stored token (if any) is still valid. If not, clear it.
//   2. User clicks "Login" on /login → useAuth.startLogin(provider) →
//      backend gives us an authorize URL → window.location replaces.
//   3. Dex redirects back to /auth/callback?code=...&state=... → the
//      callback route component calls handleCallback(code, state) →
//      we POST to backend /auth/callback → backend exchanges code for
//      token → we store everything and route to /.
//   4. User clicks logout → wipe storage + go to /login.
//
// Why we don't use Pinia/Vuex: only one screen really cares about auth
// state (App.vue's header + the Login screen + router guards). A
// lightweight singleton composable keeps it simple.

import { ref, computed } from 'vue'
import { getAuthProviders, authCallback, getAuthMe, authLogout } from '../api/velero'
// PRD-028: subpath-aware login redirect (== '/login' at default basePath).
import { loginPath } from '../basePath'

// Singleton state. Composables import { auth } directly and share this.
const idToken = ref(localStorage.getItem('supkube.idToken') || '')
const refreshToken = ref(localStorage.getItem('supkube.refreshToken') || '')
const user = ref(parseStoredUser())
const authEnabled = ref(true) // assume on until /auth/me tells us otherwise
const providers = ref([])
const bootstrapped = ref(false)
const bootstrapping = ref(false)

const isLoggedIn = computed(() => {
  // When auth is disabled at the backend, treat anyone as logged in.
  if (!authEnabled.value) return true
  return !!idToken.value && !!user.value
})

// v0.8.5 step 3: role + namespace scope. Resolved by backend at JWT
// verification time and exposed via /auth/me. Frontend never decides the
// role — we just mirror what backend says.
const role = computed(() => user.value?.role || 'viewer')
const namespaceScope = computed(() => user.value?.namespaceScope || [])
const isAdmin = computed(() => role.value === 'admin')
const isEditor = computed(() => role.value === 'editor')
const isViewer = computed(() => role.value === 'viewer')

// canDo is the SINGLE helper components use to gate UI affordances.
// Keep its surface small — one method, mirror server-side permission
// table semantics. Components that need finer control can read role +
// namespaceScope directly.
//
// Action vocabulary (kept in sync with server permissionTable):
//
//   read.*            → viewer + above
//   restore.create    → editor + above (ns check applies)
//   restore.delete    → editor + above (ns check applies)
//   backup.create     → editor + above (ns check applies)
//   backup.delete     → editor + above (ns check applies)
//   policy.create     → editor + above
//   policy.edit       → editor + above
//   policy.delete     → editor + above
//   policy.runonce    → editor + above
//   policy.pauseresume → editor + above
//   transformset.apply → editor + above
//
//   namespace.create  → admin
//   transformset.crud → admin
//   storage.crud      → admin
//   snapshot.crud     → admin
//   rbac.manage       → admin (future v0.9 UI)
//   audit.read        → admin
//
// Optional 2nd arg `ns`: when present, also verifies the user's
// namespaceScope includes that ns (or is admin/viewer).
function canDo(action, ns) {
  // RBAC disabled at backend → every action allowed (legacy demo mode).
  if (user.value && user.value.role === 'admin') return true

  const adminOnly = new Set([
    'namespace.create',
    'transformset.crud',
    'storage.crud',
    'snapshot.crud',
    'rbac.manage',
    'audit.read'
  ])
  if (adminOnly.has(action)) return false  // editor + viewer denied

  const editorOrAbove = new Set([
    'backup.create', 'backup.delete',
    'restore.create', 'restore.delete', 'restore.preflight',
    'policy.create', 'policy.edit', 'policy.delete',
    'policy.runonce', 'policy.pauseresume',
    'transformset.apply'
  ])
  if (editorOrAbove.has(action)) {
    if (!isEditor.value) return false
    if (ns !== undefined && !namespaceScope.value.includes(ns)) return false
    return true
  }
  // Everything else (reads) → viewer + above is fine.
  return true
}

function parseStoredUser() {
  try {
    const raw = localStorage.getItem('supkube.user')
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

function persistSession({ idToken: id, refreshToken: rt, user: u }) {
  if (id) localStorage.setItem('supkube.idToken', id)
  if (rt) localStorage.setItem('supkube.refreshToken', rt)
  if (u) localStorage.setItem('supkube.user', JSON.stringify(u))
  idToken.value = id || ''
  refreshToken.value = rt || ''
  user.value = u || null
}

function clearSession() {
  localStorage.removeItem('supkube.idToken')
  localStorage.removeItem('supkube.refreshToken')
  localStorage.removeItem('supkube.user')
  sessionStorage.removeItem('supkube.oauthState')
  idToken.value = ''
  refreshToken.value = ''
  user.value = null
}

// bootstrap is called once on app start. It calls /auth/me to:
//   (a) discover whether the backend has auth enabled
//   (b) validate any stored token (server will 401 if expired)
async function bootstrap() {
  if (bootstrapped.value || bootstrapping.value) return
  bootstrapping.value = true
  try {
    const res = await getAuthMe()
    authEnabled.value = !!res.data.authEnabled
    if (!authEnabled.value) {
      // Demo mode — synthesize a fake admin user so the SPA renders normally.
      user.value = res.data.user || { username: 'anonymous', groups: ['admin'] }
      bootstrapped.value = true
      return
    }
    // Auth IS enabled.
    if (res.data.user && res.data.user.username) {
      user.value = res.data.user
      // Refresh persisted copy in case groups changed on the IdP.
      localStorage.setItem('supkube.user', JSON.stringify(res.data.user))
    } else {
      // Backend says auth is on but doesn't know us — token is gone or
      // never existed. Clear the stale localStorage and let the router
      // guard redirect to /login.
      clearSession()
    }
  } catch (e) {
    // Network error → assume auth is on but we can't reach the backend.
    // Don't clear session prematurely; the user might just be offline.
    console.warn('useAuth.bootstrap: /auth/me failed', e)
  } finally {
    bootstrapping.value = false
    bootstrapped.value = true
  }
}

// loadProviders fetches the list of "Login with X" buttons + the OIDC
// authorize URL. Called by Login.vue when it mounts.
async function loadProviders() {
  const res = await getAuthProviders()
  authEnabled.value = !!res.data.enabled
  providers.value = res.data.providers || []
  // Backend hands us the OAuth2 state; the SPA echoes it back on
  // callback to defend against cross-origin code-replay attacks.
  if (res.data.state) {
    sessionStorage.setItem('supkube.oauthState', res.data.state)
  }
  return res.data
}

// startLogin redirects the browser to the IdP. After consent the IdP
// (via Dex) redirects back to /auth/callback?code=...&state=...
function startLogin(provider) {
  if (!provider || !provider.loginUrl) {
    throw new Error('provider has no loginUrl')
  }
  window.location.href = provider.loginUrl
}

// handleCallback is called from the /auth/callback route component
// with the URL params Dex appended. We POST them to the backend, which
// exchanges code → token, then we persist and route to /.
async function handleCallback(code, state) {
  const expectedState = sessionStorage.getItem('supkube.oauthState')
  if (!code) throw new Error('missing authorization code')
  if (expectedState && state !== expectedState) {
    throw new Error('state mismatch — possible CSRF attempt')
  }
  const res = await authCallback({ code, state })
  persistSession({
    idToken: res.data.idToken,
    refreshToken: res.data.refreshToken,
    user: res.data.user
  })
  sessionStorage.removeItem('supkube.oauthState')
  return res.data.user
}

async function logout() {
  try { await authLogout() } catch {}
  clearSession()
  window.location.href = loginPath
}

export function useAuth() {
  return {
    // state
    user,
    idToken,
    isLoggedIn,
    isAdmin,
    isEditor,
    isViewer,
    role,
    namespaceScope,
    authEnabled,
    providers,
    bootstrapped,
    // RBAC helper
    canDo,
    // actions
    bootstrap,
    loadProviders,
    startLogin,
    handleCallback,
    logout
  }
}
