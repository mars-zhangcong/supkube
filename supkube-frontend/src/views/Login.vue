<!--
  Login.vue (v0.8.5)
  ──────────────────
  Renders one button per OIDC provider returned by /api/v1/auth/providers.
  In the embedded-Dex demo this is a single "Login" button that takes the
  user to Dex's chooser screen (where they enter admin@supkube.local /
  admin). In production with upstream connectors, the SPA gets multiple
  buttons like "Login with Keycloak", "Login with Okta", etc.

  This route is also where the OIDC redirect lands — when Dex bounces
  back with ?code=...&state=... in the URL, we delegate to useAuth
  handleCallback() and navigate to /.
-->
<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <div class="logo">🛡</div>
        <h1>SupKube</h1>
        <p class="tagline">Kubernetes Data Protection</p>
      </div>

      <div v-if="state.loading" class="loading">
        <el-icon class="spin"><Loading /></el-icon>
        {{ state.loadingMsg || t('login.loading') }}
      </div>

      <div v-else-if="state.error" class="error-box">
        <el-icon><WarningFilled /></el-icon>
        <div>{{ state.error }}</div>
      </div>

      <template v-else>
        <p v-if="reason === 'expired'" class="hint">{{ t('login.expired') }}</p>
        <p v-if="!authEnabled" class="hint">{{ t('login.demoMode') }}</p>

        <div class="providers" v-if="providers.length > 0">
          <button
            v-for="p in providers"
            :key="p.id"
            class="provider-btn"
            type="button"
            @click="onPick(p)"
          >
            <span class="provider-icon">{{ iconFor(p) }}</span>
            <span class="provider-name">{{ p.name }}</span>
            <span class="provider-arrow">→</span>
          </button>
        </div>

        <div v-else-if="authEnabled" class="empty">
          <p>{{ t('login.noProviders') }}</p>
        </div>

        <div v-else>
          <button class="provider-btn" type="button" @click="goHome">
            {{ t('login.continue') }}
          </button>
        </div>
      </template>

      <div class="footer">
        <a href="https://github.com/mars-zhangcong/supkube" target="_blank" rel="noopener">
          Documentation
        </a>
        <span class="dot">·</span>
        <span class="version">{{ version }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Loading, WarningFilled } from '@element-plus/icons-vue'
import { useAuth } from '../composables/useAuth'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuth()
const { providers, authEnabled } = auth

import { FULL_VERSION } from '../version'
const version = FULL_VERSION
const reason = computed(() => route.query.reason || '')

const state = reactive({
  loading: true,
  loadingMsg: '',
  error: ''
})

// Map IdP id/type → display icon. Picked emojis (instead of brand SVGs)
// so we don't ship trademarked logos. Customer-configured connectors
// land here too; unknown IDs fall back to a generic globe.
function iconFor(p) {
  const m = {
    // Built-in / common IDs (matches Dex connector_id conventions)
    local:     '👤',  // password DB
    dex:       '🔐',  // Dex chooser fallback
    keycloak:  '🔑',
    okta:      '⚡',
    azuread:   '💠',  // Azure AD / Entra ID
    microsoft: '💠',
    github:    '🐙',
    gitlab:    '🦊',
    google:    '🅶',
    bitbucket: '🪣',
    ldap:      '🗂',
    saml:      '🛡',
    // Type-based fallback (Dex connector type)
    password:  '👤',
    oidc:      '🌐'
  }
  return m[p.id] || m[p.type] || '🌐'
}

function onPick(p) {
  state.loadingMsg = t('login.redirecting', { name: p.name })
  state.loading = true
  auth.startLogin(p)
}

function goHome() { router.push('/') }

onMounted(async () => {
  // Two modes: direct visit (show providers) vs OIDC callback landing
  // (process ?code= + ?state= and redirect home).
  if (route.query.code) {
    state.loadingMsg = t('login.exchanging')
    try {
      await auth.handleCallback(String(route.query.code), String(route.query.state || ''))
      ElMessage.success(t('login.success'))
      router.replace('/')
      return
    } catch (e) {
      state.error = e?.response?.data?.error || e.message || t('login.callbackFailed')
      state.loading = false
      return
    }
  }
  // Direct visit — show provider list.
  try {
    await auth.loadProviders()
  } catch (e) {
    state.error = e?.response?.data?.error || e.message || t('login.loadFailed')
  } finally {
    state.loading = false
  }
})
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--sk-primary-bg-hover) 0%, #faf5ff 100%);
  padding: 20px;
}
.login-card {
  width: 100%;
  max-width: 420px;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 28px;
  box-shadow: 0 20px 60px rgba(79, 70, 229, 0.15), 0 4px 12px rgba(0,0,0,0.04);
}

.brand { text-align: center; margin-bottom: 28px; }
.logo { font-size: 44px; line-height: 1; margin-bottom: 8px; }
.brand h1 {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  color: var(--sk-text);
  letter-spacing: -0.3px;
}
.tagline {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: var(--sk-text-caption);
}

.loading {
  text-align: center;
  padding: 30px 0;
  color: var(--sk-text-muted);
  font-size: 14px;
}
.spin {
  animation: spin 1s linear infinite;
  font-size: 22px;
  display: block;
  margin: 0 auto 10px;
  color: var(--sk-primary);
}
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }

.error-box {
  background: #fef0f0;
  border: 1px solid #fbc4c4;
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  gap: 10px;
  color: #5b2929;
  font-size: 13px;
  line-height: 1.5;
}
.error-box .el-icon { font-size: 20px; flex-shrink: 0; margin-top: 1px; color: var(--sk-status-error); }

.hint {
  text-align: center;
  color: var(--sk-text-caption);
  font-size: 13px;
  margin: 0 0 16px 0;
}

.providers { display: flex; flex-direction: column; gap: 10px; }
.provider-btn {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  border: 1.5px solid #ebeef5;
  background: #fff;
  border-radius: 10px;
  cursor: pointer;
  font-size: 14px;
  color: var(--sk-text);
  font-weight: 500;
  transition: all 0.15s ease;
}
.provider-btn:hover {
  border-color: var(--sk-primary);
  background: #f5f3ff;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.08);
}
.provider-icon { font-size: 20px; }
.provider-name { flex: 1; text-align: left; }
.provider-arrow { color: var(--sk-text-placeholder); font-size: 18px; transition: color 0.15s; }
.provider-btn:hover .provider-arrow { color: var(--sk-primary); }

.empty {
  text-align: center;
  color: var(--sk-text-caption);
  font-size: 13px;
  padding: 20px 0;
}

.footer {
  margin-top: 28px;
  padding-top: 18px;
  border-top: 1px solid #f0f2f5;
  text-align: center;
  font-size: 12px;
  color: var(--sk-text-caption);
}
.footer a { color: var(--sk-primary); text-decoration: none; }
.footer a:hover { text-decoration: underline; }
.footer .dot { margin: 0 8px; color: var(--sk-text-placeholder); }
.version { font-family: 'SF Mono', Menlo, monospace; }
</style>
