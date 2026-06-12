<template>
  <div class="restores-page">
    <div class="page-header">
      <div>
        <h1>{{ t('restores.title') }}</h1>
        <p class="page-subtitle">{{ t('restores.subtitle') }}</p>
      </div>
      <div class="toolbar">
        <label class="toggle">
          <input v-model="staleOnly" type="checkbox" />
          <span>{{ t('restores.staleOnly') }}</span>
        </label>
        <button class="refresh-btn" @click="loadRestores" :disabled="loading">
          {{ loading ? t('common.loading') : t('common.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div class="table-card">
      <table class="restores-table">
        <thead>
          <tr>
            <th>{{ t('restores.columns.name') }}</th>
            <th>{{ t('restores.columns.namespace') }}</th>
            <th>{{ t('restores.columns.phase') }}</th>
            <th>{{ t('restores.columns.backupName') }}</th>
            <th>{{ t('restores.columns.scheduleName') }}</th>
            <th>{{ t('restores.columns.warnings') }}</th>
            <th>{{ t('restores.columns.errors') }}</th>
            <th>{{ t('restores.columns.createdAt') }}</th>
            <th>{{ t('restores.columns.age') }}</th>
            <th>{{ t('restores.columns.stale') }}</th>
            <th>{{ t('restores.columns.message') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="11" class="empty">{{ t('common.loading') }}</td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td colspan="11" class="empty">{{ t('restores.empty') }}</td>
          </tr>
          <tr v-for="item in items" :key="`${item.namespace}/${item.name}`">
            <td>{{ item.name }}</td>
            <td>{{ item.namespace }}</td>
            <td>
              <span class="phase-pill" :class="phaseClass(item.phase)">{{ item.phase || '-' }}</span>
            </td>
            <td>{{ item.backupName || '-' }}</td>
            <td>{{ item.scheduleName || '-' }}</td>
            <td>{{ item.warnings }}</td>
            <td>{{ item.errors }}</td>
            <td>{{ formatDate(item.createdAt) }}</td>
            <td>{{ item.age || '-' }}</td>
            <td>
              <span class="stale-pill" :class="item.stale ? 'is-stale' : 'is-fresh'">
                {{ item.stale ? t('common.yes') : t('common.no') }}
              </span>
            </td>
            <td class="message-cell">{{ item.message || '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getRestoreResults } from '../api/velero'

const { t } = useI18n()
const items = ref([])
const loading = ref(false)
const error = ref('')
const staleOnly = ref(false)

function phaseClass(phase) {
  const value = String(phase || '').toLowerCase()
  if (['completed'].includes(value)) return 'is-success'
  if (['failed', 'failedvalidation', 'partiallyfailed'].includes(value)) return 'is-danger'
  if (['inprogress', 'new'].includes(value)) return 'is-warning'
  return 'is-neutral'
}

function formatDate(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

async function loadRestores() {
  loading.value = true
  error.value = ''
  try {
    const data = await getRestoreResults({ staleOnly: staleOnly.value })
    items.value = Array.isArray(data?.items) ? data.items : []
  } catch (err) {
    error.value = err?.message || t('common.loadFailed')
    items.value = []
  } finally {
    loading.value = false
  }
}

watch(staleOnly, () => {
  loadRestores()
})

onMounted(() => {
  loadRestores()
})
</script>

<style scoped>
.restores-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.page-subtitle {
  margin: 4px 0 0;
  color: var(--color-text-secondary, #667085);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.refresh-btn {
  border: 1px solid #d0d5dd;
  background: #fff;
  border-radius: 8px;
  padding: 8px 12px;
  cursor: pointer;
}

.refresh-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.error-banner {
  background: #fef3f2;
  color: #b42318;
  border: 1px solid #fecdca;
  border-radius: 8px;
  padding: 12px;
}

.table-card {
  background: #fff;
  border: 1px solid #eaecf0;
  border-radius: 12px;
  overflow: auto;
}

.restores-table {
  width: 100%;
  border-collapse: collapse;
}

.restores-table th,
.restores-table td {
  padding: 12px 14px;
  border-bottom: 1px solid #eaecf0;
  text-align: left;
  vertical-align: top;
  font-size: 14px;
}

.restores-table th {
  background: #f9fafb;
  color: #475467;
  font-weight: 600;
}

.empty {
  text-align: center;
  color: #667085;
}

.phase-pill,
.stale-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 2px 10px;
  font-size: 12px;
  font-weight: 600;
}

.is-success {
  background: #ecfdf3;
  color: #027a48;
}

.is-danger {
  background: #fef3f2;
  color: #b42318;
}

.is-warning {
  background: #fffaeb;
  color: #b54708;
}

.is-neutral {
  background: #f2f4f7;
  color: #344054;
}

.is-stale {
  background: #fef3f2;
  color: #b42318;
}

.is-fresh {
  background: #ecfdf3;
  color: #027a48;
}

.message-cell {
  max-width: 320px;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
