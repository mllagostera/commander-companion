<script setup lang="ts">
import type { AdminUserSummary } from '~/types/api'

definePageMeta({ middleware: 'admin' })

const { t, d } = useI18n()
const { listUsers } = useAdmin()

const search = ref('')
const users = ref<AdminUserSummary[]>([])
const cursor = ref<string | null>(null)
const isLoading = ref(false)
const isLoadingMore = ref(false)
const loadError = ref(false)
let searchDebounce: ReturnType<typeof setTimeout> | undefined

async function loadUsers() {
  isLoading.value = true
  loadError.value = false
  try {
    const page = await listUsers(undefined, search.value.trim() || undefined)
    users.value = page.items
    cursor.value = page.next_cursor
  } catch {
    loadError.value = true
  } finally {
    isLoading.value = false
  }
}

async function loadMore() {
  if (!cursor.value) return
  isLoadingMore.value = true
  try {
    const page = await listUsers(cursor.value, search.value.trim() || undefined)
    users.value = [...users.value, ...page.items]
    cursor.value = page.next_cursor
  } finally {
    isLoadingMore.value = false
  }
}

function onSearchInput() {
  clearTimeout(searchDebounce)
  searchDebounce = setTimeout(loadUsers, 300)
}

await loadUsers()

function createdAtLabel(iso: string): string {
  return d(new Date(iso), 'short')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('admin.users.title') }}</h1>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('admin.users.subtitle') }}</p>
      </div>
    </section>

    <input
      v-model="search"
      type="search"
      :placeholder="$t('admin.users.searchPlaceholder')"
      :aria-label="$t('admin.users.searchPlaceholder')"
      class="w-full max-w-sm rounded-full border px-4 py-2.5 text-[13px] outline-none"
      style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      @input="onSearchInput"
    >

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ $t('admin.users.loadError') }}</p>
    <p v-else-if="!isLoading && !users.length" class="text-sm" style="color: var(--text-muted);">
      {{ $t('admin.users.empty') }}
    </p>

    <div v-else class="flex flex-col gap-2">
      <NuxtLink
        v-for="user in users"
        :key="user.id"
        :to="`/admin/users/${user.id}`"
        class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-xl)] border px-5 py-3.5 transition-colors hover:border-[var(--accent-link)]"
        style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
      >
        <div class="flex flex-col">
          <span class="text-sm font-semibold">{{ user.username }}</span>
          <span class="text-[13px]" style="color: var(--text-dim);">{{ user.email }}</span>
        </div>
        <div class="flex items-center gap-2 text-[11px] uppercase tracking-wide">
          <span
            v-if="user.is_admin"
            class="rounded-full px-2.5 py-1 font-semibold"
            style="background: rgba(139,92,246,0.15); color: #a78bfa;"
          >
            {{ $t('admin.users.badges.admin') }}
          </span>
          <span
            v-if="!user.is_active"
            class="rounded-full px-2.5 py-1 font-semibold"
            style="background: var(--lose-bg); color: var(--lose);"
          >
            {{ $t('admin.users.badges.deactivated') }}
          </span>
          <span style="color: var(--text-dim);">{{ createdAtLabel(user.created_at) }}</span>
        </div>
      </NuxtLink>
    </div>

    <button
      v-if="cursor"
      type="button"
      :disabled="isLoadingMore"
      class="self-center rounded-full border px-5 py-2 text-sm disabled:opacity-50"
      style="border-color: var(--input-border); color: var(--text);"
      @click="loadMore"
    >
      {{ isLoadingMore ? t('admin.users.loadingMore') : t('admin.users.loadMore') }}
    </button>
  </div>
</template>
