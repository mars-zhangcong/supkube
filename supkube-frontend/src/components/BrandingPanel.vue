<!--
  BrandingPanel (v0.8.11) — white-label product identity editor.

  Admin-only (rendered only inside the gated tab). Lets the operator
  change three things:

    1. Product name — shown in sidebar header + browser tab title
    2. Logo        — shown in sidebar (24px), should look good at 16px
                     in the tab favicon if no separate favicon is set
    3. Favicon     — overrides the browser tab icon (separate from
                     the sidebar logo so a finely-tuned 16x16 .ico
                     can be uploaded)

  Both image fields accept SVG / PNG / JPEG / WebP / ICO. The picker
  reads the file with FileReader, base64-encodes it into a data: URL,
  and stores it inline in the supkube-settings ConfigMap. 100 KB hard
  cap per asset (backend rejects above).

  Why no separate object-store upload:
    See branding.go header — data URLs in CM keep the system simple,
    avoid an extra storage backend + auth surface, and a small SVG
    embeds in ~10 KB which is well under the per-asset cap.
-->
<template>
  <div class="branding-panel">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      style="margin-bottom: 18px"
    >
      <template #title>{{ t('branding.intro.title') }}</template>
      <template #default>
        <div class="sk-body">{{ t('branding.intro.body') }}</div>
      </template>
    </el-alert>

    <el-form
      :model="form"
      label-position="top"
      class="branding-form"
      @submit.prevent="onSave"
    >
      <!-- ─── Product name ─── -->
      <el-form-item :label="t('branding.productName')">
        <el-input
          v-model="form.productName"
          :placeholder="t('branding.productNamePlaceholder')"
          maxlength="64"
          show-word-limit
        />
        <div class="sk-caption" style="margin-top: 4px">{{ t('branding.productNameHint') }}</div>
      </el-form-item>

      <!-- ─── Sidebar logo ─── -->
      <el-form-item :label="t('branding.logo')">
        <div class="upload-row">
          <div class="logo-preview" :class="{ 'is-empty': !form.logoDataUrl }">
            <img v-if="form.logoDataUrl" :src="form.logoDataUrl" alt="logo preview" />
            <span v-else class="sk-caption">{{ t('branding.preview') }}</span>
          </div>
          <div class="upload-actions">
            <input
              ref="logoFileInput"
              type="file"
              accept="image/svg+xml,image/png,image/jpeg,image/webp,image/x-icon"
              hidden
              @change="(e) => onPickFile(e, 'logo')"
            />
            <el-button @click="logoFileInput?.click()">
              {{ t('branding.chooseFile') }}
            </el-button>
            <el-button
              v-if="form.logoDataUrl"
              type="default"
              @click="form.logoDataUrl = ''"
            >
              {{ t('branding.reset') }}
            </el-button>
          </div>
        </div>
        <div class="sk-caption" style="margin-top: 4px">{{ t('branding.logoHint') }}</div>
      </el-form-item>

      <!-- ─── Favicon ─── -->
      <el-form-item :label="t('branding.favicon')">
        <div class="upload-row">
          <div class="favicon-preview" :class="{ 'is-empty': !form.faviconDataUrl }">
            <img v-if="form.faviconDataUrl" :src="form.faviconDataUrl" alt="favicon preview" />
            <span v-else class="sk-caption">{{ t('branding.preview') }}</span>
          </div>
          <div class="upload-actions">
            <input
              ref="faviconFileInput"
              type="file"
              accept="image/svg+xml,image/png,image/jpeg,image/webp,image/x-icon"
              hidden
              @change="(e) => onPickFile(e, 'favicon')"
            />
            <el-button @click="faviconFileInput?.click()">
              {{ t('branding.chooseFile') }}
            </el-button>
            <el-button
              v-if="form.faviconDataUrl"
              type="default"
              @click="form.faviconDataUrl = ''"
            >
              {{ t('branding.reset') }}
            </el-button>
          </div>
        </div>
        <div class="sk-caption" style="margin-top: 4px">{{ t('branding.faviconHint') }}</div>
      </el-form-item>

      <!-- ─── Color scheme picker — Kasten-style swatch row ─── -->
      <el-form-item :label="t('branding.colorScheme')">
        <div class="color-swatches">
          <button
            v-for="c in colorPalette"
            :key="c.value"
            type="button"
            class="color-swatch"
            :class="{ 'is-active': (form.primaryColor || '') === c.value }"
            :style="{ background: c.value }"
            :title="c.label"
            @click="form.primaryColor = c.value"
          >
            <svg v-if="(form.primaryColor || '') === c.value" class="check" viewBox="0 0 16 16" fill="none">
              <path d="M3 8 L6.5 11.5 L13 5" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
          <!-- "Reset to default" pill — looks visually distinct from
               the colour swatches so the user can clearly find the
               way back to SupKube's indigo without guessing which
               swatch was the original. -->
          <button
            type="button"
            class="color-swatch color-swatch-reset"
            :class="{ 'is-active': !form.primaryColor }"
            :title="t('branding.colorReset')"
            @click="form.primaryColor = ''"
          >
            <span>×</span>
          </button>
        </div>
        <div class="sk-caption" style="margin-top: 4px">{{ t('branding.colorHint') }}</div>
      </el-form-item>

      <!-- ─── Live preview of how the sidebar will look ─── -->
      <el-form-item>
        <div class="live-preview">
          <div class="live-preview-label sk-h3">{{ t('branding.livePreview') }}</div>
          <div class="live-preview-sidebar">
            <img
              :src="form.logoDataUrl || (basePrefix + '/supkube-favicon.svg')"
              :alt="form.productName"
              class="live-preview-logo"
            />
            <span class="live-preview-text">{{ form.productName || 'SupKube' }}</span>
          </div>
        </div>
      </el-form-item>

      <!-- ─── Save / Discard ─── -->
      <div class="form-actions">
        <el-button :disabled="!dirty" @click="onDiscard">
          {{ t('branding.discard') }}
        </el-button>
        <el-button
          type="primary"
          :disabled="!dirty"
          :loading="saving"
          @click="onSave"
        >
          {{ t('branding.save') }}
        </el-button>
      </div>
    </el-form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getBranding, updateBranding } from '../api/velero'
import { useBranding } from '../composables/useBranding'
// PRD-028: subpath-aware bundled-asset prefix ('' at default basePath).
// Exposed to the template (script-setup top-level binding) for the live-preview fallback.
import { basePrefix } from '../basePath'

const { t } = useI18n()
const { refresh: refreshBranding } = useBranding()

// form  = the editable working copy
// saved = the last server-confirmed state (for the dirty check)
const form  = reactive({ productName: '', logoDataUrl: '', faviconDataUrl: '', primaryColor: '' })
const saved = reactive({ productName: '', logoDataUrl: '', faviconDataUrl: '', primaryColor: '' })

// v0.8.11.1: curated palette. Picked for ENTERPRISE feel + colour-blind
// safe distribution. Linear / Vercel / Stripe-ish tones.
const colorPalette = [
  { value: '#4f46e5', label: 'Indigo (default)' },
  { value: '#2563eb', label: 'Blue' },
  { value: '#0d9488', label: 'Teal' },
  { value: '#10b981', label: 'Emerald' },
  { value: '#d97706', label: 'Amber' },
  { value: '#e11d48', label: 'Rose' },
  { value: '#9333ea', label: 'Purple' },
  { value: '#475569', label: 'Slate' }
]
const saving = ref(false)

const logoFileInput    = ref(null)
const faviconFileInput = ref(null)

const dirty = computed(() =>
  form.productName    !== saved.productName    ||
  form.logoDataUrl    !== saved.logoDataUrl    ||
  form.faviconDataUrl !== saved.faviconDataUrl ||
  form.primaryColor   !== saved.primaryColor
)

// Hard cap from the backend; enforced here too so the user gets an
// immediate error instead of a round-trip 400.
const MAX_ASSET_BYTES = 100 * 1024

// onPickFile reads the chosen file, base64-encodes it, and writes a
// data: URL into the form. Synchronous-feeling UX even though
// FileReader is async — we just await onload.
async function onPickFile(e, kind) {
  const file = e.target?.files?.[0]
  if (!file) return
  if (file.size > MAX_ASSET_BYTES) {
    ElMessage.error(t('branding.errors.tooLarge', { kb: Math.round(file.size / 1024) }))
    e.target.value = ''
    return
  }
  try {
    const dataUrl = await readFileAsDataUrl(file)
    if (kind === 'logo')    form.logoDataUrl    = dataUrl
    if (kind === 'favicon') form.faviconDataUrl = dataUrl
  } catch (err) {
    ElMessage.error(t('branding.errors.readFailed', { msg: err?.message || 'unknown' }))
  } finally {
    // Allow re-selecting the same file (browsers cache the previous pick).
    e.target.value = ''
  }
}

function readFileAsDataUrl(file) {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result || ''))
    r.onerror = () => reject(new Error('FileReader failed'))
    r.readAsDataURL(file)
  })
}

async function onSave() {
  if (!dirty.value) return
  saving.value = true
  try {
    const res = await updateBranding({
      productName:    form.productName,
      logoDataUrl:    form.logoDataUrl,
      faviconDataUrl: form.faviconDataUrl,
      primaryColor:   form.primaryColor
    })
    const d = res?.data || {}
    saved.productName    = d.productName    || 'SupKube'
    saved.logoDataUrl    = d.logoDataUrl    || ''
    saved.faviconDataUrl = d.faviconDataUrl || ''
    saved.primaryColor   = d.primaryColor   || ''
    form.productName     = saved.productName
    form.logoDataUrl     = saved.logoDataUrl
    form.faviconDataUrl  = saved.faviconDataUrl
    form.primaryColor    = saved.primaryColor
    // Tell the global useBranding store to re-fetch — that pushes the
    // new product name + logo + favicon to every page that's open.
    await refreshBranding()
    ElMessage.success(t('branding.saveSuccess'))
  } catch (err) {
    const msg = err?.response?.data?.error || err?.message || 'unknown'
    ElMessage.error(t('branding.errors.saveFailed', { msg }))
  } finally {
    saving.value = false
  }
}

function onDiscard() {
  form.productName    = saved.productName
  form.logoDataUrl    = saved.logoDataUrl
  form.faviconDataUrl = saved.faviconDataUrl
  form.primaryColor   = saved.primaryColor
}

onMounted(async () => {
  try {
    const res = await getBranding()
    const d = res?.data || {}
    saved.productName    = d.productName    || 'SupKube'
    saved.logoDataUrl    = d.logoDataUrl    || ''
    saved.faviconDataUrl = d.faviconDataUrl || ''
    saved.primaryColor   = d.primaryColor   || ''
    form.productName    = saved.productName
    form.logoDataUrl    = saved.logoDataUrl
    form.faviconDataUrl = saved.faviconDataUrl
    form.primaryColor   = saved.primaryColor
  } catch (err) {
    ElMessage.error(t('branding.errors.loadFailed', { msg: err?.message || 'unknown' }))
  }
})
</script>

<style scoped>
.branding-panel { padding: 8px 4px 32px; }
.branding-form { max-width: 640px; }

.upload-row {
  display: flex;
  align-items: center;
  gap: var(--sk-space-lg);
}
.logo-preview,
.favicon-preview {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--sk-border);
  border-radius: 6px;
  background: var(--sk-bg-soft);
  overflow: hidden;
}
.logo-preview     { width: 64px;  height: 64px; }
.favicon-preview  { width: 32px;  height: 32px; }
.logo-preview img,
.favicon-preview img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.logo-preview.is-empty,
.favicon-preview.is-empty {
  font-size: 10px;
  color: var(--sk-text-caption);
  text-align: center;
  padding: 4px;
}
.upload-actions {
  display: flex;
  gap: var(--sk-space-sm);
}

/* v0.8.11.1: colour scheme swatches. Each is a square with the colour
   filled; the active one shows a white check. Kasten / Material -style
   row layout. */
.color-swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 2px;
}
.color-swatch {
  width: 36px;
  height: 36px;
  border-radius: 4px;
  border: 2px solid transparent;
  cursor: pointer;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: transform 120ms ease, border-color 120ms ease, box-shadow 120ms ease;
  position: relative;
}
.color-swatch:hover {
  transform: scale(1.08);
  box-shadow: 0 2px 6px rgba(0,0,0,0.18);
}
.color-swatch.is-active {
  border-color: var(--sk-text);
  box-shadow: 0 0 0 2px var(--sk-bg-page), 0 0 0 4px var(--sk-text);
}
.color-swatch .check { width: 18px; height: 18px; }
.color-swatch-reset {
  background: var(--sk-bg-soft);
  border: 1px dashed var(--sk-border);
  color: var(--sk-text-muted);
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}
.color-swatch-reset:hover { background: var(--sk-bg-hover); }

.live-preview {
  margin-top: var(--sk-space-md);
  padding: var(--sk-space-md);
  background: var(--sk-bg-soft);
  border: 1px solid var(--sk-border);
  border-radius: 6px;
}
.live-preview-label {
  margin-bottom: var(--sk-space-sm);
}
.live-preview-sidebar {
  display: flex;
  align-items: center;
  gap: var(--sk-space-sm);
  padding: 10px 12px;
  background: var(--sk-bg-page);
  border: 1px solid var(--sk-border);
  border-radius: 4px;
  width: 220px;
}
.live-preview-logo {
  width: 28px;
  height: 28px;
  object-fit: contain;
}
.live-preview-text {
  font-size: 16px;
  font-weight: 700;
  color: var(--sk-text);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--sk-space-sm);
  margin-top: var(--sk-space-lg);
  padding-top: var(--sk-space-lg);
  border-top: 1px solid var(--sk-border);
}
</style>
