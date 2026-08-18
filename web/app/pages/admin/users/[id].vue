<script setup lang="ts">
definePageMeta({ middleware: 'admin' })

const route = useRoute()
const { d } = useI18n()
const { getUser, updateUserStatus } = useAdmin()
const { user: currentUser } = useAuth()

const userId = route.params.id as string

const { data: detail, refresh, error: loadError } = await useAsyncData(
  `admin-user-${userId}`,
  () => getUser(userId),
  { default: () => null },
)

const isSelf = computed(() => currentUser.value?.id === userId)

const statusError = ref('')
const isUpdating = ref(false)
const isConfirmOpen = ref(false)
const confirmDialogRef = ref<HTMLElement | null>(null)

function askDeactivate() {
  statusError.value = ''
  isConfirmOpen.value = true
}

function cancelDeactivate() {
  isConfirmOpen.value = false
}

useModalA11y(isConfirmOpen, confirmDialogRef, cancelDeactivate)

async function confirmDeactivate() {
  isUpdating.value = true
  statusError.value = ''
  try {
    await updateUserStatus(userId, false)
    isConfirmOpen.value = false
    await refresh()
  } catch (err) {
    statusError.value = adminError(err)
  } finally {
    isUpdating.value = false
  }
}

async function activate() {
  isUpdating.value = true
  statusError.value = ''
  try {
    await updateUserStatus(userId, true)
    await refresh()
  } catch (err) {
    statusError.value = adminError(err)
  } finally {
    isUpdating.value = false
  }
}

function createdAtLabel(iso: string): string {
  return d(new Date(iso), 'short')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <NuxtLink to="/admin/users" class="w-fit text-[13px]" style="color: var(--accent-link);">
      {{ $t('admin.userDetail.back') }}
    </NuxtLink>

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ $t('admin.userDetail.loadError') }}</p>

    <template v-else-if="detail">
      <section class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold sm:text-[26px]">{{ detail.username }}</h1>
          <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ detail.email }}</p>
        </div>

        <div class="flex items-center gap-2 text-[11px] uppercase tracking-wide">
          <span
            v-if="detail.is_admin"
            class="rounded-full px-2.5 py-1 font-semibold"
            style="background: rgba(139,92,246,0.15); color: #a78bfa;"
          >
            {{ $t('admin.users.badges.admin') }}
          </span>
          <span
            v-if="!detail.is_active"
            class="rounded-full px-2.5 py-1 font-semibold"
            style="background: var(--lose-bg); color: var(--lose);"
          >
            {{ $t('admin.users.badges.deactivated') }}
          </span>
        </div>
      </section>

      <section class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div class="flex flex-col gap-1.5 rounded-[var(--radius-xl)] border p-4" style="border-color: var(--card-border); background: var(--card-bg);">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('admin.userDetail.decks') }}</span>
          <span class="text-xl font-semibold" style="color: var(--text);">{{ detail.deck_count }}</span>
        </div>
        <div class="flex flex-col gap-1.5 rounded-[var(--radius-xl)] border p-4" style="border-color: var(--card-border); background: var(--card-bg);">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('admin.userDetail.gamesPlayed') }}</span>
          <span class="text-xl font-semibold" style="color: var(--text);">{{ detail.games_played_count }}</span>
        </div>
        <div class="flex flex-col gap-1.5 rounded-[var(--radius-xl)] border p-4" style="border-color: var(--card-border); background: var(--card-bg);">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('admin.userDetail.emailVerified') }}</span>
          <span class="text-xl font-semibold" style="color: var(--text);">
            {{ detail.email_verified ? $t('admin.userDetail.yes') : $t('admin.userDetail.no') }}
          </span>
        </div>
        <div class="flex flex-col gap-1.5 rounded-[var(--radius-xl)] border p-4" style="border-color: var(--card-border); background: var(--card-bg);">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('admin.userDetail.createdAt') }}</span>
          <span class="text-xl font-semibold" style="color: var(--text);">{{ createdAtLabel(detail.created_at) }}</span>
        </div>
      </section>

      <p v-if="statusError" class="text-sm" style="color: var(--lose);">{{ statusError }}</p>

      <section class="flex flex-wrap items-center gap-3">
        <button
          v-if="detail.is_active"
          type="button"
          :disabled="isUpdating || isSelf"
          :title="isSelf ? $t('admin.userDetail.cannotDeactivateSelf') : undefined"
          class="rounded-full border px-5 py-2 text-sm font-semibold disabled:opacity-50"
          style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
          @click="askDeactivate"
        >
          {{ $t('admin.userDetail.deactivate') }}
        </button>
        <button
          v-else
          type="button"
          :disabled="isUpdating"
          class="rounded-full px-5 py-2 text-sm font-semibold text-[#0a0714] disabled:opacity-50"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          @click="activate"
        >
          {{ $t('admin.userDetail.activate') }}
        </button>
      </section>
    </template>

    <div
      v-if="isConfirmOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="cancelDeactivate"
    >
      <div
        ref="confirmDialogRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="admin-deactivate-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <h2 id="admin-deactivate-title" class="text-[15px] font-medium">
          {{ $t('admin.userDetail.deactivateConfirmTitle', { username: detail?.username }) }}
        </h2>
        <p class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('admin.userDetail.deactivateConfirmBody') }}</p>

        <div class="mt-5 flex justify-end gap-3">
          <button
            type="button"
            class="rounded-full border px-4 py-2 text-sm"
            style="border-color: var(--input-border); color: var(--text);"
            @click="cancelDeactivate"
          >
            {{ $t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="isUpdating"
            class="rounded-full border px-5 py-2 text-sm font-semibold disabled:opacity-50"
            style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
            @click="confirmDeactivate"
          >
            {{ $t('admin.userDetail.deactivate') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
