import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const srcRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')
const readSource = (relativePath: string) => readFileSync(resolve(srcRoot, relativePath), 'utf8')

describe('restricted admin frontend contract', () => {
  it('limits navigation and routes to users, recharge records, and usage records', () => {
    const sidebar = readSource('components/layout/AppSidebar.vue')
    const router = readSource('router/index.ts')

    for (const path of ['/admin/users/managed', '/admin/orders/records', '/admin/usage/managed']) {
      expect(sidebar).toContain(path)
      expect(router).toContain(path)
    }
    expect(router).toContain("next('/admin/users/managed')")
    expect(sidebar).toContain("return authStore.isPrimaryAdmin ? '/admin/dashboard' : '/admin/users/managed'")
  })

  it('keeps user management in regular-user mode', () => {
    const users = readSource('views/admin/UsersView.vue')
    const createModal = readSource('components/admin/user/UserCreateModal.vue')
    const editModal = readSource('components/admin/user/UserEditModal.vue')

    expect(users).toContain(':selectable="!props.restricted"')
    expect(users).toContain(':regular-only="props.restricted"')
    expect(users).toContain("role: (props.restricted ? 'user' : filters.role)")
    expect(createModal).toContain("role: props.regularOnly ? 'user' : rest.role")
    expect(editModal).toContain("role: props.regularOnly ? 'user' : form.role")
  })

  it('makes recharge and usage records read-only and hides platform internals', () => {
    const orders = readSource('views/admin/orders/AdminOrdersView.vue')
    const usage = readSource('views/admin/UsageView.vue')
    const usageFilters = readSource('components/admin/usage/UsageFilters.vue')

    expect(orders).toContain("!props.readonly && row.status === 'PENDING'")
    expect(orders).toContain('<AdminRefundDialog v-if="!props.readonly"')
    expect(usage).toContain(':show-account-cost="!props.restricted"')
    expect(usage).toContain(':show-account-billing="!props.restricted"')
    expect(usage).toContain("return props.restricted ? tabs.slice(0, 1) : tabs")
    expect(usageFilters).toContain("v-if=\"!props.restricted\" ref=\"accountSearchRef\"")
    expect(usageFilters).toContain("v-if=\"!props.restricted\" type=\"button\" @click=\"$emit('cleanup')\"")
  })
})
