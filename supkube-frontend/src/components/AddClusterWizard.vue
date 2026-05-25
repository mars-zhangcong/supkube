<!--
  AddClusterWizard (v0.9.0 MC1)
  ────────────────────────────────────────────────────────────────────
  Three-step modal:
    1. Identify — name + display name + type (primary/secondary)
    2. Connect — upload kubeconfig file, pick context
    3. Verify  — Test Connection, show k8s version / nodes / Velero

  Stays on Step 1/2 until the user clicks Verify. We keep all state
  client-side until "Add Cluster" — no half-created cluster CRs left
  in the cluster if the user closes the modal partway.

  Why a wizard (vs a flat form):
    - The Test step gives the user confidence that the kubeconfig
      actually works before they commit. Without this, a misconfigured
      kubeconfig produces a Cluster CR stuck in `Unreachable` forever
      that the user then has to manually delete.
    - Step 3 also surfaces "Velero installed?" so the user sees
      upfront whether they need to install Velero on the remote
      cluster before this cluster is usable for backup operations.
-->

<template>
  <el-dialog
    v-model="visible"
    :title="t('clusters.wizard.title')"
    width="560px"
    :close-on-click-modal="false"
    @closed="reset"
  >
    <el-steps :active="step" finish-status="success" simple style="margin-bottom: 20px">
      <el-step :title="t('clusters.wizard.step1')" />
      <el-step :title="t('clusters.wizard.step2')" />
      <el-step :title="t('clusters.wizard.step3')" />
    </el-steps>

    <!-- ════ Step 1: Identify ════ -->
    <div v-if="step === 0">
      <el-form label-width="120px" label-position="left">
        <el-form-item :label="t('clusters.wizard.name')" required :error="nameError">
          <el-input v-model="form.name" placeholder="aks-prod-eastus" />
          <span class="form-hint">{{ t('clusters.wizard.nameHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('clusters.wizard.displayName')">
          <el-input v-model="form.displayName" placeholder="Production East US" />
        </el-form-item>
        <el-form-item :label="t('clusters.wizard.type')">
          <el-radio-group v-model="form.type">
            <el-radio-button value="secondary">{{ t('clusters.wizard.typeSecondary') }}</el-radio-button>
            <el-radio-button value="primary">{{ t('clusters.wizard.typePrimary') }}</el-radio-button>
          </el-radio-group>
          <span class="form-hint">{{ t('clusters.wizard.typeHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('clusters.wizard.description')">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
    </div>

    <!-- ════ Step 2: Connect ════ -->
    <div v-if="step === 1">
      <el-form label-width="120px" label-position="left">
        <el-form-item :label="t('clusters.wizard.kubeconfig')" required>
          <input
            ref="fileInput"
            type="file"
            style="display: none"
            @change="onFile"
          />
          <el-button @click="fileInput.click()">
            <el-icon><Upload /></el-icon>
            {{ form.kubeconfig ? t('clusters.wizard.replaceFile') : t('clusters.wizard.chooseFile') }}
          </el-button>
          <span v-if="form.kubeconfigName" class="form-hint" style="margin-left: 12px">
            {{ form.kubeconfigName }} ({{ Math.round(form.kubeconfig.length / 1024) }} KiB)
          </span>
          <span class="form-hint">{{ t('clusters.wizard.kubeconfigHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('clusters.wizard.context')">
          <el-input v-model="form.context" :placeholder="t('clusters.wizard.contextPlaceholder')" />
          <span class="form-hint">{{ t('clusters.wizard.contextHint') }}</span>
        </el-form-item>
      </el-form>
    </div>

    <!-- ════ Step 3: Verify ════ -->
    <div v-if="step === 2">
      <div class="verify-block">
        <el-button
          type="primary"
          plain
          @click="runTest"
          :loading="testing"
          :disabled="!form.kubeconfig"
        >
          {{ t('clusters.wizard.testButton') }}
        </el-button>

        <div v-if="testResult" class="verify-result" :class="{ 'is-ok': testResult.phase === 'Healthy', 'is-bad': testResult.phase !== 'Healthy' }">
          <div class="vr-row">
            <el-icon v-if="testResult.phase === 'Healthy'" class="vr-icon ok"><CircleCheck /></el-icon>
            <el-icon v-else class="vr-icon bad"><WarningFilled /></el-icon>
            <strong>{{ testResult.phase }}</strong>
            <span v-if="testResult.message" class="vr-message">{{ testResult.message }}</span>
          </div>
          <div v-if="testResult.phase === 'Healthy'" class="vr-detail">
            <div>✓ {{ t('clusters.wizard.k8sVersion') }}: <code>{{ testResult.k8sVersion }}</code></div>
            <div>✓ {{ t('clusters.wizard.nodeCount') }}: <code>{{ testResult.nodeCount }}</code></div>
            <div v-if="testResult.veleroInstalled">
              ✓ Velero: <code>{{ testResult.veleroVersion || 'detected' }}</code>
            </div>
            <div v-else class="vr-warn">
              ⚠ {{ t('clusters.wizard.veleroMissing') }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button v-if="step > 0" @click="step--">{{ t('clusters.wizard.back') }}</el-button>
      <el-button
        v-if="step < 2"
        type="primary"
        @click="next"
        :disabled="!canAdvance"
      >
        {{ t('clusters.wizard.next') }}
      </el-button>
      <el-button
        v-else
        type="primary"
        @click="submit"
        :loading="submitting"
        :disabled="!testResult || testResult.phase !== 'Healthy'"
      >
        {{ t('clusters.wizard.add') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Upload, CircleCheck, WarningFilled } from '@element-plus/icons-vue'
import { createCluster, testClusterByKubeconfig } from '../api/velero'

const props = defineProps({
  modelValue: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'created'])

const { t } = useI18n()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const step = ref(0)
const testing = ref(false)
const submitting = ref(false)
const testResult = ref(null)
const fileInput = ref(null)

const form = reactive({
  name: '',
  displayName: '',
  type: 'secondary',
  description: '',
  kubeconfig: '',       // raw text content
  kubeconfigName: '',   // filename (display only)
  context: ''
})

const nameError = computed(() => {
  if (!form.name) return ''
  if (!/^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$/.test(form.name)) {
    return t('clusters.wizard.nameInvalid')
  }
  if (form.name === 'this-cluster' || form.name === '_mcm') {
    return t('clusters.wizard.nameReserved')
  }
  return ''
})

// canAdvance: per-step gate for the Next button. Wizard prevents the
// user from skipping a required field rather than failing on Submit.
const canAdvance = computed(() => {
  if (step.value === 0) return !!form.name && !nameError.value
  if (step.value === 1) return !!form.kubeconfig
  return true
})

function next() {
  if (step.value === 1 && form.kubeconfig) {
    // Auto-run the connection test when entering Step 3 — saves the user
    // a click in the happy path.
    step.value = 2
    runTest()
    return
  }
  step.value++
}

function onFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 256 * 1024) {
    ElMessage.error(t('clusters.wizard.fileTooLarge'))
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    form.kubeconfig = String(reader.result || '')
    form.kubeconfigName = file.name
    // Reset test state — different kubeconfig means previous test is stale.
    testResult.value = null
  }
  reader.readAsText(file)
}

async function runTest() {
  testing.value = true
  testResult.value = null
  try {
    const res = await testClusterByKubeconfig({
      kubeconfig: form.kubeconfig,
      context: form.context || undefined
    })
    testResult.value = res.data
  } catch (e) {
    testResult.value = {
      phase: 'Unreachable',
      message: e?.response?.data?.error || e.message || 'unknown error'
    }
  } finally {
    testing.value = false
  }
}

async function submit() {
  submitting.value = true
  try {
    await createCluster({
      name: form.name,
      displayName: form.displayName || undefined,
      type: form.type,
      description: form.description || undefined,
      kubeconfig: form.kubeconfig,
      context: form.context || undefined
    })
    ElMessage.success(t('clusters.wizard.added', { name: form.name }))
    emit('created', form.name)
    visible.value = false
  } catch (e) {
    ElMessage.error(
      t('clusters.wizard.addFailed') + ': ' + (e?.response?.data?.error || e.message)
    )
  } finally {
    submitting.value = false
  }
}

function reset() {
  step.value = 0
  testResult.value = null
  Object.assign(form, {
    name: '', displayName: '', type: 'secondary', description: '',
    kubeconfig: '', kubeconfigName: '', context: ''
  })
}

// Reset on dialog open (covers "user reopens after a previous cancel")
watch(visible, (v) => { if (v) reset() })
</script>

<style scoped>
.form-hint {
  display: block;
  font-size: 12px;
  color: var(--sk-text-caption, #909399);
  margin-top: 4px;
  line-height: 1.5;
}
.verify-block { padding: 8px 0; }
.verify-result {
  margin-top: 16px;
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid var(--sk-border-light, #e5e7eb);
}
.verify-result.is-ok { background: #f0fdf4; border-color: #86efac; }
.verify-result.is-bad { background: #fef2f2; border-color: #fca5a5; }
.vr-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.vr-icon { font-size: 20px; }
.vr-icon.ok { color: #059669; }
.vr-icon.bad { color: #dc2626; }
.vr-message {
  color: #6b7280;
  font-size: 12px;
  font-family: 'SF Mono', Menlo, monospace;
}
.vr-detail {
  margin-top: 8px;
  padding-left: 28px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--sk-text, #1f2937);
}
.vr-detail code {
  background: #fff;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
}
.vr-warn {
  color: #d97706;
  margin-top: 4px;
}
</style>
