<template>
  <!-- v0.8.5: bare layout on /login + /auth/callback — no sidebar/header
       chrome around the login card. router-view directly. -->
  <router-view v-if="$route.meta?.public" />
  <div v-else id="app">
    <el-container>
      <el-aside :width="collapsed ? '64px' : '220px'" :class="{ 'is-collapsed': collapsed }">
        <div class="sidebar-brand" :class="{ 'is-collapsed': collapsed }">
          <!-- v0.8.11: logo + product name come from the cluster-wide
               branding store (useBranding). Admin can change them in
               Settings → Branding; everyone sees the new values
               immediately (no per-browser localStorage). -->
          <!-- v0.9.1.3: onerror handler removes the img if the URL 404s,
               instead of showing the browser's ugly broken-image icon.
               useBranding defaults to /supkube-logo.svg which is bundled
               in public/; this onerror is the safety net for OEM customers
               who set logoDataUrl to a bad path. -->
          <img :src="branding.logoUrl" :alt="branding.productName" class="sidebar-logo" @error="$event.target.style.visibility='hidden'" />
          <span v-if="!collapsed" class="sidebar-brand-text">{{ branding.productName }}</span>
          <button
            class="sidebar-toggle"
            type="button"
            :title="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
            @click="collapsed = !collapsed"
          >
            <el-icon><ArrowLeft v-if="!collapsed" /><ArrowRight v-else /></el-icon>
          </button>
        </div>

        <!-- v0.9.0.2: Mode Switcher.
             Always rendered (collapsed-state issue from v0.9.0.1) — when
             sidebar is collapsed we shrink to an icon-only trigger; the
             dropdown menu is the same. Single-cluster users still get
             the dropdown (with just "+ Add Cluster" as the only useful
             action) so the entry path is uniform. -->
        <div class="cluster-switcher" :class="{ 'is-collapsed': collapsed }">
          <el-dropdown
            trigger="click"
            placement="bottom-start"
            @command="onClusterPick"
          >
            <button class="cs-trigger" :class="{ 'is-collapsed': collapsed }" type="button" :title="cluster.activeLabel.value">
              <span class="cs-label">
                <el-icon class="cs-icon"><DocumentCopy /></el-icon>
                <span v-if="!collapsed" class="cs-name">{{ cluster.activeLabel.value }}</span>
              </span>
              <el-icon v-if="!collapsed" class="cs-caret"><ArrowDown /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item :command="cluster.MCM_ID" :class="{ 'is-active': cluster.isMCM.value }">
                  <el-icon><Connection /></el-icon>
                  {{ t('clusterSwitcher.mcm') }}
                </el-dropdown-item>
                <el-dropdown-item divided disabled class="cs-section-label">
                  {{ t('clusterSwitcher.clusters') }}
                </el-dropdown-item>
                <el-dropdown-item
                  v-for="c in cluster.clusters.value"
                  :key="c.name"
                  :command="c.name"
                  :class="{ 'is-active': cluster.active.value === c.name || (!cluster.active.value && c.name === 'this-cluster') }"
                  :disabled="c.phase !== 'Healthy' && c.name !== 'this-cluster'"
                >
                  <el-icon><DocumentCopy /></el-icon>
                  <span>{{ c.displayName || c.name }}</span>
                  <span v-if="c.type === 'primary'" class="cs-primary-mark">★</span>
                  <span v-if="c.phase !== 'Healthy' && c.name !== 'this-cluster'" class="cs-phase-bad">{{ c.phase }}</span>
                </el-dropdown-item>
                <el-dropdown-item divided @click="goAddCluster">
                  <el-icon><Plus /></el-icon>
                  {{ t('clusterSwitcher.addCluster') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>

        <!-- v0.9.1.3 menu restoration (Mars 2026-05-26 demo blocker):
             v0.9.1.2 removed 存储位置 + 快照位置 from the sidebar planning
             to move them into Cluster Management as tabs — but that
             replacement page wasn't built yet, leaving customers unable
             to add a BSL (the literal next step in any demo). Direct
             URL /storage worked but expecting customers to type paths is
             unacceptable.
             Decision: restore the original entries until the proper
             Storage Class Management page is shipped (v0.9.1.4+). Keep
             the Observability hub as an additional entry too — it
             provides better Activity/Advisor/Audit UX. Net: 8 entries
             until v0.9.1.4. -->
        <el-menu
          :router="true"
          :default-active="$route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          class="sidebar-menu"
        >
          <el-menu-item index="/dashboard">
            <el-icon><Monitor /></el-icon>
            <template #title>{{ t('nav.dashboard') }}</template>
          </el-menu-item>
          <el-menu-item index="/observability">
            <el-icon><MagicStick /></el-icon>
            <template #title>{{ t('nav.observability') }}</template>
          </el-menu-item>
          <el-menu-item index="/applications">
            <el-icon><Grid /></el-icon>
            <template #title>{{ t('nav.applications') }}</template>
          </el-menu-item>
          <el-menu-item index="/policies">
            <el-icon><Clock /></el-icon>
            <template #title>{{ t('nav.policies') }}</template>
          </el-menu-item>
          <el-menu-item index="/transform-sets">
            <el-icon><Tools /></el-icon>
            <template #title>{{ t('nav.transformSets') }}</template>
          </el-menu-item>
          <el-menu-item index="/backups">
            <el-icon><FolderOpened /></el-icon>
            <template #title>{{ t('nav.restorePoints') }}</template>
          </el-menu-item>
          <el-menu-item index="/storage">
            <el-icon><Coin /></el-icon>
            <template #title>{{ t('nav.storage') }}</template>
          </el-menu-item>
          <el-menu-item index="/snapshot-locations">
            <el-icon><Camera /></el-icon>
            <template #title>{{ t('nav.snapshotLocations') }}</template>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <template #title>{{ t('nav.settings') }}</template>
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-container>
        <el-header>
          <!-- v0.8.11: header title is reactive to branding. The product
               name comes from the cluster-wide store; the tagline tail
               ("— Kubernetes Data Protection") stays i18n-translated. -->
          <h2>{{ branding.productName }} {{ t('app.titleTail') }}</h2>
          <span class="version-badge" :title="`Build: ${BUILD}`">v{{ VERSION }} <span class="build-suffix">· {{ BUILD }}</span></span>
          <span class="header-spacer"></span>
          <el-dropdown trigger="click" @command="onLocaleChange" class="locale-dropdown">
            <button class="locale-toggle" type="button" :title="t('settings.language')">
              {{ currentLocaleShort }}
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="loc in SUPPORTED_LOCALES"
                  :key="loc.code"
                  :command="loc.code"
                  :class="{ 'is-active': loc.code === locale }"
                >
                  {{ loc.label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <button
            class="theme-toggle"
            type="button"
            :title="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
            @click="toggleTheme"
          >
            {{ isDark ? '☀' : '☾' }}
          </button>
          <!-- v0.8.5: User badge + logout. Only renders when auth is on
               (in demo mode the user is anonymous and logout is a no-op). -->
          <el-dropdown v-if="auth.user.value" trigger="click" @command="onUserCmd" class="user-dropdown">
            <button class="user-toggle" type="button" :title="auth.user.value.email || auth.user.value.username">
              <span class="user-avatar">{{ userInitial }}</span>
              <span class="user-name">{{ auth.user.value.username }}</span>
              <!-- v0.8.5 step 3: role badge so user knows their permission level -->
              <span v-if="auth.user.value.role" class="user-role" :class="`role-${auth.user.value.role}`">
                {{ auth.user.value.role }}
              </span>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  <span class="user-info-line">{{ auth.user.value.email || auth.user.value.username }}</span>
                </el-dropdown-item>
                <el-dropdown-item v-if="auth.user.value.groups?.length" disabled>
                  <span class="user-info-groups">{{ auth.user.value.groups.join(', ') }}</span>
                </el-dropdown-item>
                <el-dropdown-item v-if="auth.authEnabled.value" command="logout" divided>
                  <el-icon><SwitchButton /></el-icon> {{ t('auth.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </el-header>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { SUPPORTED_LOCALES, setLocale } from './i18n'
import {
  Monitor, Grid, FolderOpened, RefreshRight, Clock, Coin, Setting,
  Camera, MagicStick, ArrowLeft, ArrowRight, ArrowDown, DataLine,
  Tools, SwitchButton, DocumentCopy, Connection, Plus
} from '@element-plus/icons-vue'
import { useAuth } from './composables/useAuth'
import { useBranding } from './composables/useBranding'
import { useCluster } from './composables/useCluster'
import { VERSION, BUILD } from './version'

const auth = useAuth()
const cluster = useCluster()
const router = useRouter()
const route = useRoute()

// v0.9.0 MC Mode Switcher actions.
// v0.9.0.2 fix #3: picking "Multi-Cluster Manager" now actually navigates
// to the dedicated /multicluster page (previously it just updated the
// active cluster to _mcm without changing what was rendered).
function onClusterPick(value) {
  cluster.setActive(value, router)
  if (value === cluster.MCM_ID) {
    router.push({ path: '/multicluster' })
  } else if (router.currentRoute.value.path === '/multicluster') {
    // Picking a specific cluster while on the MCM page → drop into that
    // cluster's Dashboard. Without this the user "switches cluster" but
    // sees no change because the MCM page is global.
    router.push({ path: '/dashboard', query: { cluster: value } })
  }
}
function goAddCluster() {
  router.push('/settings')
  // Best-effort: hint Settings to open the Clusters tab. Settings.vue
  // reads ?tab= from query on mount to set activeTab.
  router.replace({ path: '/settings', query: { tab: 'clusters' } })
}
function goManageClusters() {
  // Single-cluster static label → click → jumps to Settings → Clusters,
  // which is entry path #3 of the design doc.
  router.push({ path: '/settings', query: { tab: 'clusters' } })
}

// Refresh registry on app mount + on every route change (a click in
// the Mode Switcher triggers route change → state stays in sync).
onMounted(() => {
  cluster.hydrateFromRoute(route)
  cluster.refresh()
})
watch(() => route.query.cluster, (v) => {
  if (v !== cluster.active.value) {
    cluster.hydrateFromRoute(route)
  }
})
// v0.8.11: cluster-wide branding (product name + logo + favicon).
// Reactive — when admin saves Settings → Branding, the whole UI
// flips to the new identity without a page reload.
const { branding } = useBranding()

const userInitial = computed(() => {
  const u = auth.user.value
  if (!u) return '?'
  const src = u.username || u.email || '?'
  return src.charAt(0).toUpperCase()
})

function onUserCmd(cmd) {
  if (cmd === 'logout') auth.logout()
}

const { t, locale } = useI18n()
const currentLocaleShort = computed(() => {
  const found = SUPPORTED_LOCALES.find(l => l.code === locale.value)
  return found ? found.short : 'EN'
})
const onLocaleChange = (code) => setLocale(code)

// Persist collapsed state across reloads (localStorage). Element Plus's
// el-menu :collapse mode auto-hides item text and shows a tooltip with the
// menu item's #title slot on hover — that's the behavior the screenshot
// shows in the FileSight reference, so we use the built-in mode.
const STORAGE_KEY = 'supkube.sidebar.collapsed'
const collapsed = ref(localStorage.getItem(STORAGE_KEY) === 'true')
watch(collapsed, (v) => {
  try { localStorage.setItem(STORAGE_KEY, String(v)) } catch (_) { /* SSR/private mode */ }
})

// v0.7.3 dark mode toggle. Initial value pulled from <html> class set by
// main.js boot (which read localStorage before mount, avoiding flash).
const THEME_KEY = 'supkube.theme'
const isDark = ref(document.documentElement.classList.contains('dark'))
const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  try { localStorage.setItem(THEME_KEY, isDark.value ? 'dark' : 'light') } catch (_) { /* ignore */ }
}
</script>

<style>
/* Global font configuration — Kasten K10 style */
:root {
  --font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  --font-size-base: 14px;
  --font-size-sm: 13px;
  --font-size-xs: 12px;
  --font-size-lg: 16px;
  --font-size-xl: 20px;
  --color-text-primary: #303133;
  --color-text-regular: #606266;
  --color-text-secondary: #909399;
  --color-text-placeholder: #c0c4cc;
  --color-primary: #409eff;
  --color-success: #67c23a;
  --color-warning: #e6a23c;
  --color-danger: #f56c6c;
}

* {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  font-family: var(--font-family);
  font-size: var(--font-size-base);
  color: var(--color-text-primary);
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* Override Element Plus default font */
.el-menu-item,
.el-table,
.el-card,
.el-button,
.el-input,
.el-select,
.el-dialog,
.el-form-item,
.el-tag,
.el-dropdown-menu__item {
  font-family: var(--font-family) !important;
}

h1, h2, h3, h4, h5, h6 {
  font-family: var(--font-family);
  font-weight: 600;
  color: var(--color-text-primary);
}

/* ============================================================
   v0.7.3 Dark mode — applied when <html class="dark"> is set.
   Element Plus 自带 dark css-vars 处理大部分组件；我们这里只覆盖
   SupKube 自定义的硬编码颜色（sidebar/header/main bg、自绘 card 等）。
   ============================================================ */
html.dark body {
  background-color: #1d1e1f;
  color: #cfd3dc;
}
html.dark .el-aside {
  background-color: #1a1c1e !important;
  border-right-color: #2b2e31 !important;
}
html.dark .sidebar-brand {
  border-bottom-color: #2b2e31 !important;
}
html.dark .sidebar-brand-text { color: #e5eaf3 !important; }
html.dark .sidebar-toggle {
  background: #1a1c1e !important;
  border-color: #3c4044 !important;
  color: #a3a6ad !important;
}
html.dark .sidebar-toggle:hover {
  background: #25282b !important;
  color: #e5eaf3 !important;
}
html.dark .el-aside .el-menu-item { color: #a3a6ad !important; }
html.dark .el-aside .el-menu-item .el-icon { color: #909399 !important; }
html.dark .el-aside .el-menu-item:hover { background-color: #25282b !important; color: #e5eaf3 !important; }
html.dark .el-aside .el-menu-item.is-active { background-color: #2f3236 !important; color: #e5eaf3 !important; }
html.dark .el-header {
  background-color: #1a1c1e !important;
  border-bottom-color: #2b2e31 !important;
  color: #cfd3dc;
}
html.dark .el-main { background-color: #232527 !important; }
html.dark .version-badge {
  background: #1d3a5f !important;
  color: #79bbff !important;
}
html.dark .theme-toggle,
html.dark .locale-toggle {
  background: #2b2e31 !important;
  border-color: #3c4044 !important;
  color: #cfd3dc !important;
}
html.dark .theme-toggle:hover,
html.dark .locale-toggle:hover {
  background: #3c4044 !important;
  color: #79bbff !important;
}
/* SupKube custom card surfaces — sync hint, imported card, action blocks etc. */
html.dark .chart-card,
html.dark .ns-cell,
html.dark .row-label-tag,
html.dark .source-chip,
html.dark .type-chip {
  background-color: transparent !important;
}
html.dark .chart-card {
  background-color: #25282b !important;
  border-color: #3c4044 !important;
}

/* v0.7.6 — broader dark-mode coverage for surfaces and text styles that
   v0.7.3 missed. Symptoms before this pass: stat labels invisible, chart
   titles invisible, Compliance / Imported card backgrounds clashing.
   Strategy: only override SupKube-custom styles. Element Plus's own
   dark css-vars handle el-card / el-table / el-tag / el-button. */

/* All headings — the global rule `h1..h6 { color: var(--color-text-primary) }`
   pins them to #303133 which is invisible on dark surfaces. Hit every level
   uniformly with the dark text color. */
html.dark h1,
html.dark h2,
html.dark h3,
html.dark h4,
html.dark h5,
html.dark h6 {
  color: #e5eaf3 !important;
}

/* Stat cards on Dashboard */
html.dark .stat-label { color: #a3a6ad !important; }
html.dark .stat-value { color: #e5eaf3; }
html.dark .stat-value.success { color: #85ce61; }
html.dark .stat-value.danger { color: #f78989; }
html.dark .stat-value.warning { color: #eebe77; }

/* Imported / Compliance cards — re-tint the light-yellow / light-green
   backgrounds to dark-mode equivalents */
html.dark .imported-card {
  background-color: #1d3a5f !important;
  border-left-color: #79bbff !important;
}
html.dark .imported-text { color: #cfd3dc !important; }
html.dark .imported-text strong { color: #e5eaf3 !important; }

html.dark .compliance-card {
  background-color: #4a3b1e !important;
  border-left-color: #eebe77 !important;
}
html.dark .compliance-card.compliance-ok {
  background-color: #1f3f1a !important;
  border-left-color: #85ce61 !important;
}
html.dark .compliance-text { color: #cfd3dc !important; }
html.dark .compliance-text strong { color: #e5eaf3 !important; }

/* Chart card titles and helper text */
html.dark .chart-title { color: #e5eaf3 !important; }
html.dark .chart-subtitle { color: #a3a6ad !important; }
html.dark .chart-empty { color: #606266 !important; }

/* Recent backups / restores / advisor / generic page descriptions */
html.dark .page-desc { color: #a3a6ad !important; }
html.dark .muted { color: #606266 !important; }

/* Filter toolbar text on Restore Points / Applications */
html.dark .filter-summary { color: #cfd3dc !important; }
html.dark .filter-selected { color: #cfd3dc !important; }

/* Tier summary cards on Backup Advisor */
html.dark .tier-card { background-color: #25282b !important; border-color: #3c4044 !important; }
html.dark .tier-card .tier-label { color: #a3a6ad !important; }
html.dark .tier-card .tier-count { color: #e5eaf3; }
html.dark .tier-card.tier-high .tier-count { color: #f78989; }
html.dark .tier-card.tier-medium .tier-count { color: #eebe77; }
html.dark .tier-card.tier-low .tier-count { color: #79bbff; }
html.dark .tier-card.tier-skip .tier-count { color: #a3a6ad; }

/* Restore Points: namespace cell + RP name secondary */
html.dark .ns-name { color: #e5eaf3 !important; }
html.dark .rp-name { color: #909399 !important; }

/* Apptype pills on Restore Points header */
html.dark .apptype-label { color: #cfd3dc !important; }
html.dark .apptype-pill {
  background: #2b2e31 !important;
  border-color: #3c4044 !important;
  color: #cfd3dc !important;
}
html.dark .apptype-pill.is-active {
  background: #79bbff !important;
  border-color: #79bbff !important;
  color: #1d1e1f !important;
}

/* Row-label tags + source/type chips — pickup card bg from theme */
html.dark .row-label-tag {
  background: #2b2e31 !important;
  border-color: #3c4044 !important;
  color: #cfd3dc !important;
}
html.dark .source-local { background: #2b2e31 !important; color: #cfd3dc !important; }
html.dark .source-imported { background: #4a1d1f !important; color: #f78989 !important; }

/* Settings appearance card form labels */
html.dark .form-hint { color: #a3a6ad !important; }

/* Advisor schedule cell text */
html.dark .schedule-human { color: #e5eaf3 !important; }
html.dark .schedule-cron { color: #909399 !important; }
</style>

<style scoped>
/* Kasten K10-inspired sidebar — light, flat, professional */
.el-aside {
  background-color: #ffffff;
  border-right: 1px solid #e7e9ec;
  min-height: 100vh;
  padding: 0;
  transition: width 0.2s ease;
  overflow: hidden;
}

/* Brand block at the top of the sidebar; height matches el-header (60px)
 * and shares its border-bottom color so the two bottoms line up across the
 * sidebar/header boundary. */
.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 60px;
  padding: 0 14px;
  box-sizing: border-box;
  border-bottom: 1px solid #ebeef5;
  margin-bottom: 8px;
}
/* v0.9.1.2 — collapsed sidebar alignment, take 3 (CSS Grid).
   Take 1 (flex column + align-items: center) and Take 2 (margin: 0 auto
   belt-and-braces) both failed Mars's eye-test — icons still leaned left
   of the el-menu-item column. The flex-based centering is sensitive to
   the implicit content width of children + browser-specific text-baseline
   rules, even when align-items: center is set.
   Take 3 ditches flex for CSS Grid with one fixed-width column. Grid's
   `justify-items: center` is a stricter geometric centering — children's
   bounding boxes are forced to the center of the 64px column, no
   baseline / line-height / inline-flex quirks. Cross-browser identical.

   The 64px column matches el-aside.is-collapsed width exactly, so the
   icons land at x=32 — same as where Element Plus renders el-menu-item
   icons in collapsed mode (margin: 2px 8px → 48px wide menu-item with
   justify-content: center → icon-center at 8 + 24 = 32). All icons in
   the column share one vertical line. */
.sidebar-brand.is-collapsed {
  display: grid;
  grid-template-columns: 64px;
  justify-items: center;
  align-items: center;
  gap: 10px;
  width: 64px;
  height: auto;
  padding: 10px 0 8px;
}
.sidebar-brand.is-collapsed > * {
  /* Reset any margin children had from non-collapsed state; grid handles centering. */
  margin: 0;
}
.sidebar-logo {
  width: 28px;
  height: 30px;
  flex-shrink: 0;
}
.sidebar-brand-text {
  font-size: 18px;
  font-weight: 700;
  color: #1f2329;
  letter-spacing: -0.01em;
  flex: 1;
}
/* v0.9.0 MC Mode Switcher — sits between brand row and menu. */
.cluster-switcher {
  margin: 0 12px 12px;
}
/* v0.9.1.2 — collapsed Mode Switcher (take 3, CSS Grid).
   Same alignment rationale as .sidebar-brand.is-collapsed: use a Grid
   wrapper with one 64px column so the trigger button lands at the exact
   center, matching x=32 of the el-menu-item icon column below. */
.cluster-switcher.is-collapsed {
  display: grid;
  grid-template-columns: 64px;
  justify-items: center;
  margin: 0 0 8px;
}
.cs-trigger.is-collapsed {
  justify-content: center;
  padding: 8px 0;
  width: 48px;
}
.cs-trigger.is-collapsed .cs-label {
  flex: 0 0 auto;          /* don't stretch the label slot — icon-only */
  justify-content: center;
}
.cs-trigger.is-collapsed .cs-icon {
  margin: 0;
}
.cs-trigger,
.cs-static {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--sk-border-light, #e5e7eb);
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font-size: 13px;
  color: var(--sk-text, #1f2937);
  transition: all 0.15s ease;
}
.cs-trigger:hover,
.cs-static:hover {
  border-color: var(--sk-primary, #4f46e5);
  background: var(--sk-primary-bg-hover, #f5f3ff);
}
.cs-label {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}
.cs-icon {
  font-size: 14px;
  flex-shrink: 0;
  color: var(--sk-primary, #4f46e5);
}
.cs-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.cs-caret {
  font-size: 11px;
  color: var(--sk-text-caption, #9ca3af);
  flex-shrink: 0;
}
.cs-section-label {
  font-size: 11px !important;
  letter-spacing: 0.5px;
  color: var(--sk-text-caption, #9ca3af) !important;
  text-transform: uppercase;
}
.cs-primary-mark {
  color: var(--sk-primary, #4f46e5);
  margin-left: 4px;
  font-size: 11px;
}
.cs-phase-bad {
  margin-left: auto;
  font-size: 10px;
  color: var(--sk-status-error, #dc2626);
}
:deep(.el-dropdown-menu__item.is-active) {
  background: var(--sk-primary-bg-hover, #f5f3ff);
  color: var(--sk-primary, #4f46e5);
  font-weight: 500;
}
/* Single-cluster static label: hover hint slides in from right */
.cs-static { position: relative; }
.cs-hover-hint {
  position: absolute;
  right: 10px;
  opacity: 0;
  font-size: 11px;
  color: var(--sk-primary, #4f46e5);
  transition: opacity 0.15s ease;
  pointer-events: none;
}
.cs-static:hover .cs-hover-hint { opacity: 1; }
.cs-static:hover .cs-name { opacity: 0.3; }

.sidebar-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  border-radius: 6px;
  color: #606266;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}
.sidebar-toggle:hover {
  background: #f5f7fa;
  border-color: #c0c4cc;
  color: #1f2329;
}
.el-aside :deep(.el-menu) {
  border-right: none;
  background-color: transparent;
}
.el-aside :deep(.el-menu-item) {
  color: #4a5168;
  font-size: var(--font-size-base);
  font-weight: 500;
  height: 42px;
  line-height: 42px;
  margin: 2px 8px;
  padding: 0 14px !important;
  border-radius: 6px;
  transition: background-color 0.15s ease, color 0.15s ease;
}
.el-aside :deep(.el-menu-item .el-icon) {
  color: #6b7280;
  font-size: 18px;
  margin-right: 10px;
}
.el-aside :deep(.el-menu-item:hover) {
  background-color: #f5f7fa;
  color: #1f2329;
}
.el-aside :deep(.el-menu-item:hover .el-icon) {
  color: #1f2329;
}
.el-aside :deep(.el-menu-item.is-active) {
  background-color: #eef0f3;
  color: #1f2329;
  font-weight: 600;
}
.el-aside :deep(.el-menu-item.is-active .el-icon) {
  color: #1f2329;
}
/* Collapsed sidebar: el-menu adds .el-menu--collapse; tighten item padding
 * and remove margin so the 64px-wide rail keeps icons centered. The icon
 * margin-right is also pointless when there's no text. */
.el-aside.is-collapsed :deep(.el-menu) {
  border-right: none;
}
.el-aside.is-collapsed :deep(.el-menu--collapse) {
  width: 64px;
}
.el-aside.is-collapsed :deep(.el-menu-item) {
  margin: 2px 8px;
  padding: 0 !important;
  justify-content: center;
}
.el-aside.is-collapsed :deep(.el-menu-item .el-icon) {
  margin-right: 0;
}
.el-header {
  display: flex;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid #ebeef5;
  padding: 0 20px;
  background-color: #ffffff;
}
.el-header h2 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  margin: 0;
}
.version-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  background: #ecf5ff;
  color: #409eff;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
}
/* Build stamp tucked next to the version. Lower contrast so the version
   reads first; the timestamp is a "is my browser cache fresh?" signal. */
.version-badge .build-suffix {
  opacity: 0.55;
  font-weight: 500;
  font-family: 'SF Mono', Menlo, monospace;
  margin-left: 2px;
}
.header-spacer { flex: 1; }
.theme-toggle,
.locale-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  color: #606266;
  transition: all 0.15s;
}
.theme-toggle { font-size: 16px; }
.theme-toggle:hover,
.locale-toggle:hover {
  background: #f5f7fa;
  color: #409eff;
}
.locale-dropdown { margin-right: 8px; }

/* v0.8.5: user badge + dropdown in the header. Sits to the right of the
 * theme toggle. Avatar circle + truncated username. */
.user-dropdown { margin-left: 12px; }
.user-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 2px 12px 2px 2px;
  height: 32px;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  border-radius: 16px;
  cursor: pointer;
  font-size: 13px;
  color: #303133;
  transition: all 0.15s;
}
.user-toggle:hover {
  border-color: #4f46e5;
  background: #f5f3ff;
}
.user-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: #4f46e5;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}
.user-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.user-role {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 8px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
}
.role-admin  { background: #fef0f0; color: #c45656; }
.role-editor { background: #fdf6ec; color: #b88230; }
.role-viewer { background: #f0f9eb; color: #67c23a; }
html.dark .role-admin  { background: #2a1a1d; color: #f5b1b1; }
html.dark .role-editor { background: #2b1f0c; color: #f0c473; }
html.dark .role-viewer { background: #1a2c14; color: #c2e7b0; }
.user-info-line {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  color: #606266;
}
.user-info-groups {
  font-size: 11px;
  color: #909399;
}
html.dark .user-toggle {
  background: #1f2026;
  border-color: #3a3d44;
  color: #e5eaf3;
}
html.dark .user-toggle:hover {
  background: #1e1b4b;
  border-color: #6366f1;
}
html.dark .user-info-line { color: #b1b3b8; }
:deep(.el-dropdown-menu__item.is-active) {
  color: #409eff;
  font-weight: 600;
}
.el-main {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: calc(100vh - 60px);
}
</style>
