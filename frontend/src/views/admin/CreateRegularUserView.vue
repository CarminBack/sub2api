<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-2xl">
      <div class="mb-6">
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.users.createUser') }}
        </h1>
      </div>

      <form class="space-y-5" @submit.prevent="submit">
        <div>
          <label class="input-label">{{ t('admin.users.email') }}</label>
          <input
            v-model="form.email"
            type="email"
            required
            autocomplete="off"
            class="input"
            :placeholder="t('admin.users.enterEmail')"
          />
        </div>

        <div>
          <label class="input-label">{{ t('admin.users.password') }}</label>
          <div class="flex gap-2">
            <input
              v-model="form.password"
              type="text"
              required
              minlength="6"
              autocomplete="new-password"
              class="input min-w-0 flex-1"
              :placeholder="t('admin.users.enterPassword')"
            />
            <button
              type="button"
              class="btn btn-secondary px-3"
              :title="t('common.refresh')"
              @click="generateRandomPassword"
            >
              <Icon name="refresh" size="md" />
            </button>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.users.username') }}</label>
          <input
            v-model="form.username"
            type="text"
            class="input"
            :placeholder="t('admin.users.enterUsername')"
          />
        </div>

        <div class="flex justify-end">
          <button type="submit" class="btn btn-primary" :disabled="loading">
            {{ loading ? t('admin.users.creating') : t('common.create') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const form = reactive({ email: '', password: '', username: '' })

async function submit() {
  if (loading.value) return
  loading.value = true
  try {
    await adminAPI.users.createRegular({ ...form })
    appStore.showSuccess(t('admin.users.userCreated'))
    Object.assign(form, { email: '', password: '', username: '' })
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.users.failedToCreate'))
  } finally {
    loading.value = false
  }
}

function generateRandomPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let password = ''
  for (let i = 0; i < 16; i += 1) {
    password += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  form.password = password
}
</script>
