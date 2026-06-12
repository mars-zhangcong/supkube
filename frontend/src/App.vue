<template>
  <div class="page">
    <header class="header">
      <div>
        <h1>还原点列表</h1>
        <p>按最近备份时间排序，显示距今时长，超 RPO 标红</p>
      </div>
      <button class="primary" @click="openCreate">新建还原点</button>
    </header>

    <section class="card filters">
      <div class="grid">
        <label>
          <span>关键字</span>
          <input v-model="filters.q" placeholder="名称/公司/负责人" />
        </label>
        <label>
          <span>公司</span>
          <input v-model="filters.company_name" placeholder="公司名称" />
        </label>
        <label>
          <span>负责人</span>
          <input v-model="filters.owner" placeholder="负责人" />
        </label>
        <label>
          <span>生命周期阶段</span>
          <select v-model="filters.lifecycle_stage">
            <option value="">全部</option>
            <option v-for="item in options.lifecycle_stages" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option v-for="item in options.statuses" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
        <label>
          <span>RPO 状态</span>
          <select v-model="filters.rpo_breached">
            <option value="">全部</option>
            <option value="true">已超 RPO</option>
            <option value="false">未超 RPO</option>
          </select>
        </label>
        <label>
          <span>排序字段</span>
          <select v-model="sortBy">
            <option value="latest_backup_time">最近备份时间</option>
            <option value="age_minutes">距今时长</option>
            <option value="name">名称</option>
            <option value="company_name">公司</option>
            <option value="owner">负责人</option>
            <option value="lifecycle_stage">生命周期阶段</option>
            <option value="status">状态</option>
            <option value="rpo_minutes">RPO(分钟)</option>
            <option value="created_at">创建时间</option>
            <option value="updated_at">更新时间</option>
            <option value="id">ID</option>
          </select>
        </label>
        <label>
          <span>排序方向</span>
          <select v-model="sortOrder">
            <option value="desc">降序</option>
            <option value="asc">升序</option>
          </select>
        </label>
      </div>
      <div class="actions">
        <button class="primary" @click="applyFilters">查询</button>
        <button @click="resetFilters">重置</button>
      </div>
    </section>

    <section class="card">
      <div class="toolbar">
        <div>总数：{{ total }}</div>
        <div class="pager-inline">
          <label>
            <span>每页</span>
            <select v-model.number="pageSize" @change="changePage(1)">
              <option :value="10">10</option>
              <option :value="20">20</option>
              <option :value="50">50</option>
            </select>
          </label>
        </div>
      </div>

      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="loading" class="loading">加载中...</div>

      <table v-if="!loading" class="table">
        <thead>
          <tr>
            <th @click="quickSort('id')">ID</th>
            <th @click="quickSort('name')">名称</th>
            <th @click="quickSort('company_name')">公司</th>
            <th @click="quickSort('owner')">负责人</th>
            <th @click="quickSort('lifecycle_stage')">生命周期阶段</th>
            <th @click="quickSort('status')">状态</th>
            <th @click="quickSort('latest_backup_time')">最近备份时间</th>
            <th @click="quickSort('age_minutes')">距今时长</th>
            <th @click="quickSort('rpo_minutes')">RPO(分钟)</th>
            <th>RPO状态</th>
            <th @click="quickSort('created_at')">创建时间</th>
            <th @click="quickSort('updated_at')">更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.id" :class="{ breached: item.rpo_breached }">
            <td>{{ item.id }}</td>
            <td>{{ item.name }}</td>
            <td>{{ item.company_name }}</td>
            <td>{{ item.owner }}</td>
            <td><span class="badge">{{ item.lifecycle_stage }}</span></td>
            <td><span class="badge status" :class="item.status">{{ item.status }}</span></td>
            <td>{{ formatDate(item.latest_backup_time) }}</td>
            <td>{{ item.age_display }}</td>
            <td>{{ item.rpo_minutes }}</td>
            <td>
              <span class="badge" :class="item.rpo_breached ? 'danger' : 'success'">
                {{ item.rpo_breached ? '已超 RPO' : '正常' }}
              </span>
            </td>
            <td>{{ formatDate(item.created_at) }}</td>
            <td>{{ formatDate(item.updated_at) }}</td>
            <td class="ops">
              <button @click="openEdit(item)">编辑</button>
              <button class="danger-btn" @click="removeItem(item)">删除</button>
            </td>
          </tr>
          <tr v-if="items.length === 0">
            <td colspan="13" class="empty">暂无数据</td>
          </tr>
        </tbody>
      </table>

      <div class="pagination">
        <button :disabled="page <= 1" @click="changePage(page - 1)">上一页</button>
        <span>第 {{ page }} / {{ totalPages }} 页</span>
        <button :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</button>
      </div>
    </section>

    <div v-if="showModal" class="modal-mask" @click.self="closeModal">
      <div class="modal">
        <h2>{{ editingId ? '编辑还原点' : '新建还原点' }}</h2>
        <div class="grid form-grid">
          <label>
            <span>名称</span>
            <input v-model="form.name" />
          </label>
          <label>
            <span>公司</span>
            <input v-model="form.company_name" />
          </label>
          <label>
            <span>负责人</span>
            <input v-model="form.owner" />
          </label>
          <label>
            <span>生命周期阶段</span>
            <select v-model="form.lifecycle_stage">
              <option v-for="item in options.lifecycle_stages" :key="item" :value="item">{{ item }}</option>
            </select>
          </label>
          <label>
            <span>状态</span>
            <select v-model="form.status">
              <option v-for="item in options.statuses" :key="item" :value="item">{{ item }}</option>
            </select>
          </label>
          <label>
            <span>最近备份时间</span>
            <input v-model="form.latest_backup_time_local" type="datetime-local" />
          </label>
          <label>
            <span>RPO(分钟)</span>
            <input v-model.number="form.rpo_minutes" type="number" min="1" />
          </label>
        </div>
        <div v-if="formError" class="error">{{ formError }}</div>
        <div class="actions end">
          <button @click="closeModal">取消</button>
          <button class="primary" @click="submitForm">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'

const options = reactive({
  lifecycle_stages: [],
  statuses: []
})

const filters = reactive({
  q: '',
  company_name: '',
  owner: '',
  lifecycle_stage: '',
  status: '',
  rpo_breached: ''
})

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const sortBy = ref('latest_backup_time')
const sortOrder = ref('desc')
const loading = ref(false)
const error = ref('')
const showModal = ref(false)
const editingId = ref(null)
const formError = ref('')

const form = reactive({
  name: '',
  company_name: '',
  owner: '',
  lifecycle_stage: 'active',
  status: 'healthy',
  latest_backup_time_local: '',
  rpo_minutes: 60
})

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

onMounted(async () => {
  await loadOptions()
  await fetchList()
})

async function loadOptions() {
  const res = await fetch('/api/options')
  const data = await res.json()
  options.lifecycle_stages = data.lifecycle_stages
  options.statuses = data.statuses
}

function buildQuery() {
  const params = new URLSearchParams()
  Object.entries(filters).forEach(([k, v]) => {
    if (v !== '') params.set(k, v)
  })
  params.set('page', String(page.value))
  params.set('page_size', String(pageSize.value))
  params.set('sort_by', sortBy.value)
  params.set('sort_order', sortOrder.value)
  return params.toString()
}

async function fetchList() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/restore-points?${buildQuery()}`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    items.value = data.items
    total.value = data.total
    page.value = data.page
    pageSize.value = data.page_size
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  fetchList()
}

function resetFilters() {
  filters.q = ''
  filters.company_name = ''
  filters.owner = ''
  filters.lifecycle_stage = ''
  filters.status = ''
  filters.rpo_breached = ''
  sortBy.value = 'latest_backup_time'
  sortOrder.value = 'desc'
  page.value = 1
  fetchList()
}

function changePage(next) {
  page.value = next
  fetchList()
}

function quickSort(field) {
  if (sortBy.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    sortOrder.value = field === 'latest_backup_time' ? 'desc' : 'asc'
  }
  page.value = 1
  fetchList()
}

function openCreate() {
  editingId.value = null
  form.name = ''
  form.company_name = ''
  form.owner = ''
  form.lifecycle_stage = options.lifecycle_stages[1] || 'active'
  form.status = options.statuses[0] || 'healthy'
  form.latest_backup_time_local = toDatetimeLocal(new Date())
  form.rpo_minutes = 60
  formError.value = ''
  showModal.value = true
}

function openEdit(item) {
  editingId.value = item.id
  form.name = item.name
  form.company_name = item.company_name
  form.owner = item.owner
  form.lifecycle_stage = item.lifecycle_stage
  form.status = item.status
  form.latest_backup_time_local = toDatetimeLocal(new Date(item.latest_backup_time))
  form.rpo_minutes = item.rpo_minutes
  formError.value = ''
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

async function submitForm() {
  formError.value = ''
  if (!form.name || !form.company_name || !form.owner || !form.latest_backup_time_local || !form.rpo_minutes) {
    formError.value = '请完整填写所有字段'
    return
  }
  const payload = {
    name: form.name,
    company_name: form.company_name,
    owner: form.owner,
    lifecycle_stage: form.lifecycle_stage,
    status: form.status,
    latest_backup_time: new Date(form.latest_backup_time_local).toISOString(),
    rpo_minutes: Number(form.rpo_minutes)
  }
  const url = editingId.value ? `/api/restore-points/${editingId.value}` : '/api/restore-points'
  const method = editingId.value ? 'PUT' : 'POST'
  try {
    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    closeModal()
    await fetchList()
  } catch (e) {
    formError.value = e.message
  }
}

async function removeItem(item) {
  if (!confirm(`确认删除还原点【${item.name}】吗？`)) return
  const res = await fetch(`/api/restore-points/${item.id}`, { method: 'DELETE' })
  const data = await res.json()
  if (!res.ok) {
    alert(data.error || '删除失败')
    return
  }
  if (items.value.length === 1 && page.value > 1) {
    page.value -= 1
  }
  await fetchList()
}

function formatDate(v) {
  return new Date(v).toLocaleString('zh-CN', { hour12: false })
}

function toDatetimeLocal(date) {
  const pad = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
</script>
