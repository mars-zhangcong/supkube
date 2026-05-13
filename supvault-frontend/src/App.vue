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
          <h2>SupVault — Kubernetes Data Protection</h2>
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
