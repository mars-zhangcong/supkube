<template>
  <div id="app">
    <el-container>
      <el-aside :width="collapsed ? '64px' : '220px'" :class="{ 'is-collapsed': collapsed }">
        <div class="sidebar-brand" :class="{ 'is-collapsed': collapsed }">
          <img src="/supkube-logo.svg" alt="SupKube" class="sidebar-logo" />
          <span v-if="!collapsed" class="sidebar-brand-text">SupKube</span>
          <button
            class="sidebar-toggle"
            type="button"
            :title="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
            @click="collapsed = !collapsed"
          >
            <el-icon><ArrowLeft v-if="!collapsed" /><ArrowRight v-else /></el-icon>
          </button>
        </div>
        <el-menu
          :router="true"
          :default-active="$route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          class="sidebar-menu"
        >
          <el-menu-item index="/dashboard">
            <el-icon><Monitor /></el-icon>
            <template #title>Dashboard</template>
          </el-menu-item>
          <el-menu-item index="/applications">
            <el-icon><Grid /></el-icon>
            <template #title>Applications</template>
          </el-menu-item>
          <el-menu-item index="/backups">
            <el-icon><FolderOpened /></el-icon>
            <template #title>Restore Points</template>
          </el-menu-item>
          <el-menu-item index="/restores">
            <el-icon><RefreshRight /></el-icon>
            <template #title>Restores</template>
          </el-menu-item>
          <el-menu-item index="/policies">
            <el-icon><Clock /></el-icon>
            <template #title>Policies</template>
          </el-menu-item>
          <el-menu-item index="/storage">
            <el-icon><Coin /></el-icon>
            <template #title>Storage</template>
          </el-menu-item>
          <el-menu-item index="/snapshot-locations">
            <el-icon><Camera /></el-icon>
            <template #title>Snapshot Locations</template>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <template #title>Settings</template>
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-container>
        <el-header>
          <h2>SupKube — Kubernetes Data Protection</h2>
          <span class="version-badge">v0.7.1-alpha</span>
        </el-header>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { Monitor, Grid, FolderOpened, RefreshRight, Clock, Coin, Setting, Camera, ArrowLeft, ArrowRight } from '@element-plus/icons-vue'

// Persist collapsed state across reloads (localStorage). Element Plus's
// el-menu :collapse mode auto-hides item text and shows a tooltip with the
// menu item's #title slot on hover — that's the behavior the screenshot
// shows in the FileSight reference, so we use the built-in mode.
const STORAGE_KEY = 'supkube.sidebar.collapsed'
const collapsed = ref(localStorage.getItem(STORAGE_KEY) === 'true')
watch(collapsed, (v) => {
  try { localStorage.setItem(STORAGE_KEY, String(v)) } catch (_) { /* SSR/private mode */ }
})
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
.sidebar-brand.is-collapsed {
  flex-direction: column;
  justify-content: center;
  padding: 8px 0;
  gap: 6px;
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
.el-main {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: calc(100vh - 60px);
}
</style>
