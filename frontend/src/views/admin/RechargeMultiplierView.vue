<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-2xl">
      <div class="mb-6">
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.rechargeMultiplier.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.rechargeMultiplier.description') }}
        </p>
      </div>

      <div v-if="loading" class="flex min-h-48 items-center justify-center" aria-live="polite">
        <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
      </div>

      <form v-else class="card space-y-5 p-6" @submit.prevent="save">
        <div>
          <label for="recharge-multiplier" class="input-label">
            {{ t('admin.rechargeMultiplier.current') }}
          </label>
          <input
            id="recharge-multiplier"
            v-model.number="multiplier"
            type="number"
            inputmode="decimal"
            step="0.01"
            :min="minimum"
            required
            class="input"
            :aria-invalid="Boolean(validationError)"
            :aria-describedby="validationError ? 'recharge-multiplier-error' : 'recharge-multiplier-hint'"
          />
          <p v-if="validationError" id="recharge-multiplier-error" role="alert" class="mt-2 text-sm text-red-600 dark:text-red-400">
            {{ validationError }}
          </p>
          <p v-else id="recharge-multiplier-hint" class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.rechargeMultiplier.preview', { usd: preview }) }}
          </p>
        </div>

        <div>
          <label for="recharge-multiplier-minimum" class="input-label">
            {{ t('admin.rechargeMultiplier.minimum') }}
          </label>
          <input
            id="recharge-multiplier-minimum"
            :value="minimum.toFixed(2)"
            type="text"
            readonly
            class="input bg-gray-50 text-gray-600 dark:bg-dark-800 dark:text-gray-300"
          />
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.rechargeMultiplier.minimumHint', { minimum: minimum.toFixed(2) }) }}
          </p>
        </div>

        <div class="flex justify-end">
          <button type="submit" class="btn btn-primary min-h-11" :disabled="saving || Boolean(validationError)">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const multiplier = ref(1)
const minimum = ref(1)

const validationError = computed(() => {
  if (!Number.isFinite(multiplier.value) || multiplier.value < minimum.value) {
    return t('admin.rechargeMultiplier.belowMinimum')
  }
  return ''
})
const preview = computed(() => (Number.isFinite(multiplier.value) ? multiplier.value : 0).toFixed(2))

async function load() {
  loading.value = true
  try {
    const response = await adminPaymentAPI.getRechargeMultiplier()
    multiplier.value = response.data.balance_recharge_multiplier
    minimum.value = response.data.balance_recharge_multiplier_min
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.rechargeMultiplier.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value || validationError.value) return
  saving.value = true
  try {
    await adminPaymentAPI.updateRechargeMultiplier(multiplier.value)
    appStore.showSuccess(t('admin.rechargeMultiplier.saved'))
    await load()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.rechargeMultiplier.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
