<script setup lang="ts">
import type { MoxfieldImportJob } from '~/types/api'

const { t, d } = useI18n()
const { user, fetchSession, logout } = useAuth()
const {
  updateMoxfieldUsername,
  changePassword,
  startMoxfieldImport,
  getMoxfieldImportStatus,
} = useSettings()
const { showToast } = useToast()

const userInitial = computed(() => user.value?.username?.[0]?.toUpperCase() ?? '?')
const memberSince = computed(() => (user.value ? d(new Date(user.value.created_at), 'short') : ''))

// ------------------------------------------------------------- contraseña
const currentPassword = ref('')
const newPassword = ref('')
const newPasswordConfirm = ref('')
const passwordError = ref('')
const isChangingPassword = ref(false)

async function handleChangePassword() {
  passwordError.value = ''

  if (newPassword.value !== newPasswordConfirm.value) {
    passwordError.value = t('settings.errors.passwordMismatch')
    return
  }

  isChangingPassword.value = true
  try {
    await changePassword(currentPassword.value, newPassword.value)
    currentPassword.value = ''
    newPassword.value = ''
    newPasswordConfirm.value = ''
    showToast(t('toast.passwordUpdated'))
  } catch (err) {
    passwordError.value = changePasswordError(err)
  } finally {
    isChangingPassword.value = false
  }
}

async function handleLogout() {
  await logout()
  await navigateTo('/login')
}

// --------------------------------------------------------------- moxfield
const moxfieldUsername = ref(user.value?.moxfield_username ?? '')
const moxfieldError = ref('')
const isSavingMoxfield = ref(false)

async function handleSaveMoxfieldUsername() {
  moxfieldError.value = ''
  isSavingMoxfield.value = true
  try {
    await updateMoxfieldUsername(moxfieldUsername.value)
    await fetchSession()
    showToast(t('toast.moxfieldLinked'))
  } catch (err) {
    moxfieldError.value = updateMoxfieldUsernameError(err)
  } finally {
    isSavingMoxfield.value = false
  }
}

// ----------------------------------------------------- import en background
const importJob = ref<MoxfieldImportJob | null>(null)
const importError = ref('')
const isStartingImport = ref(false)
let pollHandle: ReturnType<typeof setTimeout> | null = null

function stopPolling() {
  if (pollHandle) clearTimeout(pollHandle)
  pollHandle = null
}

function pollImportStatus(jobId: string) {
  stopPolling()
  pollHandle = setTimeout(async () => {
    try {
      importJob.value = await getMoxfieldImportStatus(jobId)
    } catch {
      // Best-effort: un error de red puntual no debe cortar el polling.
    }
    if (importJob.value?.status === 'in_progress') {
      pollImportStatus(jobId)
    }
  }, 2000)
}

async function handleStartImport() {
  importError.value = ''
  isStartingImport.value = true
  try {
    importJob.value = await startMoxfieldImport()
    if (importJob.value.status === 'in_progress') {
      pollImportStatus(importJob.value.id)
    }
  } catch (err) {
    importError.value = startMoxfieldImportError(err)
  } finally {
    isStartingImport.value = false
  }
}

onUnmounted(stopPolling)
</script>

<template>
  <div class="flex max-w-[640px] flex-col gap-6">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('settings.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('settings.subtitle') }}</p>
    </section>

    <section class="flex flex-col gap-4 rounded-[28px] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
      <div class="flex items-center gap-3.5">
        <span
          class="flex h-[50px] w-[50px] flex-shrink-0 items-center justify-center rounded-full text-lg font-bold text-[#0a0714]"
          style="background: linear-gradient(135deg, #8b5cf6, #a855f7);"
        >{{ userInitial }}</span>
        <div>
          <p class="text-base font-semibold">{{ user?.username }}</p>
          <p class="mt-0.5 text-[13px]" style="color: var(--text-muted);">{{ user?.email }}</p>
        </div>
      </div>
      <p class="text-xs" style="color: var(--text-dim);">{{ $t('settings.memberSince', { date: memberSince }) }}</p>
    </section>

    <section class="flex flex-col gap-3.5 rounded-[28px] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
      <h2 class="text-[15px] font-medium">{{ $t('settings.moxfield.heading') }}</h2>
      <p class="text-[13px]" style="color: var(--text-muted);">
        {{ $t('settings.moxfield.description') }}
      </p>

      <form class="flex flex-col gap-3 sm:flex-row" @submit.prevent="handleSaveMoxfieldUsername">
        <input
          v-model="moxfieldUsername"
          type="text"
          :placeholder="$t('settings.moxfield.placeholder')"
          class="flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
          style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
        >
        <button
          type="submit"
          :disabled="isSavingMoxfield"
          class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ isSavingMoxfield ? $t('common.saving') : $t('common.save') }}
        </button>
      </form>
      <p v-if="moxfieldError" class="text-sm" style="color: var(--lose);">{{ moxfieldError }}</p>

      <div class="border-t pt-4" style="border-color: var(--card-border);">
        <button
          type="button"
          :disabled="isStartingImport || !user?.moxfield_username || importJob?.status === 'in_progress'"
          class="rounded-full border px-4 py-2 text-[13px] disabled:opacity-50"
          style="border-color: var(--input-border); color: var(--text);"
          @click="handleStartImport"
        >
          {{ isStartingImport ? $t('settings.moxfield.starting') : $t('settings.moxfield.importAction') }}
        </button>

        <p v-if="importError" class="mt-3 text-sm" style="color: var(--lose);">{{ importError }}</p>

        <div v-if="importJob" class="mt-3 text-sm" style="color: var(--text-muted);">
          <p v-if="importJob.status === 'in_progress'">
            {{ importJob.total_decks !== null
              ? $t('settings.moxfield.importing', { done: importJob.imported_count + importJob.failed_count, total: importJob.total_decks })
              : $t('settings.moxfield.importingNoTotal', { done: importJob.imported_count + importJob.failed_count }) }}
          </p>
          <p v-else-if="importJob.status === 'completed'" style="color: var(--win);">
            {{ $t('settings.moxfield.completed', { imported: importJob.imported_count, failed: importJob.failed_count }) }}
          </p>
          <p v-else-if="importJob.status === 'failed'" style="color: var(--lose);">
            {{ $t('settings.moxfield.failed', { message: importJob.error_message }) }}
          </p>
        </div>
      </div>
    </section>

    <section class="flex flex-col gap-3.5 rounded-[28px] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
      <h2 class="text-[15px] font-medium">{{ $t('settings.security.heading') }}</h2>

      <form class="flex flex-col gap-3" @submit.prevent="handleChangePassword">
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('settings.security.currentPassword') }}
          <input
            v-model="currentPassword"
            type="password"
            autocomplete="current-password"
            required
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('settings.security.newPassword') }}
          <input
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            required
            minlength="8"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('settings.security.confirmNewPassword') }}
          <input
            v-model="newPasswordConfirm"
            type="password"
            autocomplete="new-password"
            required
            minlength="8"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>

        <p v-if="passwordError" class="text-sm" style="color: var(--lose);">{{ passwordError }}</p>

        <button
          type="submit"
          :disabled="isChangingPassword"
          class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ isChangingPassword ? $t('common.saving') : $t('settings.security.submit') }}
        </button>
      </form>

      <div class="border-t pt-4" style="border-color: var(--card-border);">
        <p class="text-[13px]" style="color: var(--text-muted);">
          {{ $t('settings.security.sessionInfo') }}
        </p>
        <button
          type="button"
          class="mt-3 rounded-full border px-5 py-2.5 text-[13px] font-medium transition-colors"
          style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
          @click="handleLogout"
        >
          {{ $t('settings.security.logout') }}
        </button>
      </div>
    </section>
  </div>
</template>
