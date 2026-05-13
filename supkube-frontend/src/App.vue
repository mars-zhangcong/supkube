<template>
  <div id="app">
    <el-container>
      <el-aside width="200px">
        <el-menu :router="true" :default-active="$route.path">
          <el-menu-item index="/dashboard">
            <el-icon><Monitor /></el-icon>Dashboard
          </el-menu-item>
          <el-menu-item index="/applications">
            <el-icon><Grid /></el-icon>Applications
          </el-menu-item>
          <el-menu-item index="/backups">
            <el-icon><FolderOpened /></el-icon>Backups
          </el-menu-item>
          <el-menu-item index="/restores">
            <el-icon><RefreshRight /></el-icon>Restores
          </el-menu-item>
          <el-menu-item index="/policies">
            <el-icon><Clock /></el-icon>Policies
          </el-menu-item>
          <el-menu-item index="/storage">
            <el-icon><Coin /></el-icon>Storage
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>Settings
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-container>
        <el-header>
          <h2>SupKube — Kubernetes Data Protection</h2>
        </el-header>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup>
import { Monitor, Grid, FolderOpened, RefreshRight, Clock, Coin, Setting } from '@element-plus/icons-vue'
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
.el-aside {
  background-color: #1d1f2b;
  min-height: 100vh;
}
.el-aside .el-menu {
  border-right: none;
  background-color: #1d1f2b;
}
.el-aside .el-menu-item {
  color: #bbb;
  font-size: var(--font-size-base);
  font-weight: 500;
}
.el-aside .el-menu-item:hover,
.el-aside .el-menu-item.is-active {
  background-color: #2d2f3b;
  color: #fff;
}
.el-header {
  display: flex;
  align-items: center;
  border-bottom: 1px solid #ebeef5;
  padding: 0 20px;
}
.el-header h2 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  margin: 0;
}
.el-main {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: calc(100vh - 60px);
}
</style>
