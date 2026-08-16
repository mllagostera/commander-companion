<script setup lang="ts">
import QRCode from 'qrcode'
import type { MoxfieldImportJob } from '~/types/api'

const { t, d } = useI18n()
const { user, fetchSession, logout } = useAuth()
const {
  updateMoxfieldUsername,
  updateUsername,
  changePassword,
  startMoxfieldImport,
  getMoxfieldImportStatus,
  getLatestMoxfieldImportStatus,
} = useSettings()
const { showToast } = useToast()

const userInitial = computed(() => user.value?.username?.[0]?.toUpperCase() ?? '?')
const memberSince = computed(() => (user.value ? d(new Date(user.value.created_at), 'short') : ''))

// ------------------------------------------------------------------- QR
// The QR encodes the user's own id directly (no separate rotable code, see
// ADR-0017): scanning it (Android, future work) feeds that id into the same
// POST /friends/requests a username search already uses. SVG generation
// runs fine in pure Node (no canvas needed), so it also renders in the
// initial SSR payload.
const friendQrSvg = ref('')

watch(
  () => user.value?.id,
  async (id) => {
    friendQrSvg.value = id
      ? await QRCode.toString(id, { type: 'svg', margin: 1, color: { dark: '#0a0714', light: '#ffffff' } })
      : ''
  },
  { immediate: true },
)

// -------------------------------------------------------------- username
const username = ref(user.value?.username ?? '')
const usernameError = ref('')
const isSavingUsername = ref(false)

async function handleSaveUsername() {
  usernameError.value = ''
  isSavingUsername.value = true
  try {
    await updateUsername(username.value)
    await fetchSession()
    showToast(t('toast.usernameUpdated'))
  } catch (err) {
    usernameError.value = updateUsernameError(err)
  } finally {
    isSavingUsername.value = false
  }
}

// ------------------------------------------------------------- password
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

// ----------------------------------------------------- background import
const importJob = ref<MoxfieldImportJob | null>(null)
const importError = ref('')
const isStartingImport = ref(false)
let pollHandle: ReturnType<typeof setTimeout> | null = null

function stopPolling() {
  if (pollHandle) clearTimeout(pollHandle)
  pollHandle = null
}

// The job is still going in the background (not yet completed/failed): the
// backend resolves the deck list AND imports them without waiting on this
// page, 'pending' just means the list hasn't come back from Moxfield yet.
function isImportRunning(status: MoxfieldImportJob['status']) {
  return status === 'pending' || status === 'in_progress'
}

function pollImportStatus(jobId: string) {
  stopPolling()
  pollHandle = setTimeout(async () => {
    try {
      importJob.value = await getMoxfieldImportStatus(jobId)
    } catch {
      // Best-effort: a one-off network error shouldn't stop polling.
    }
    if (importJob.value && isImportRunning(importJob.value.status)) {
      pollImportStatus(jobId)
    }
  }, 2000)
}

async function handleStartImport() {
  importError.value = ''
  isStartingImport.value = true
  try {
    importJob.value = await startMoxfieldImport()
    if (isImportRunning(importJob.value.status)) {
      pollImportStatus(importJob.value.id)
    }
  } catch (err) {
    importError.value = startMoxfieldImportError(err)
  } finally {
    isStartingImport.value = false
  }
}

// Recovers a job still running (or just finished) from a previous visit:
// the import itself lives in the backend, but importJob/pollImportStatus are
// only in-memory, so navigating away and back used to look like the import
// had silently stopped even though it kept going server-side.
onMounted(async () => {
  const latest = await getLatestMoxfieldImportStatus().catch(() => null)
  if (!latest) return
  importJob.value = latest
  if (isImportRunning(latest.status)) {
    pollImportStatus(latest.id)
  }
})

onUnmounted(stopPolling)
</script>

<template>
  <div class="flex max-w-[640px] flex-col gap-6">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('settings.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('settings.subtitle') }}</p>
    </section>

    <section class="flex flex-col gap-4 rounded-[var(--radius-xl)] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
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

      <form class="flex flex-col gap-2 border-t pt-4 sm:flex-row sm:items-start" style="border-color: var(--card-border);" @submit.prevent="handleSaveUsername">
        <label class="flex-1 text-xs" style="color: var(--text-dim);">
          {{ $t('settings.username.label') }}
          <input
            v-model="username"
            type="text"
            :placeholder="$t('settings.username.placeholder')"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>
        <button
          type="submit"
          :disabled="isSavingUsername"
          class="mt-1.5 rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50 sm:mt-[26px]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ isSavingUsername ? $t('common.saving') : $t('common.save') }}
        </button>
      </form>
      <p v-if="usernameError" class="text-sm" style="color: var(--lose);">{{ usernameError }}</p>
    </section>

    <section class="flex flex-col items-center gap-3 rounded-[var(--radius-xl)] border p-[22px] text-center" style="border-color: var(--card-border); background: var(--card-bg);">
      <h2 class="text-[15px] font-medium">{{ $t('settings.qr.heading') }}</h2>
      <p class="max-w-[380px] text-[13px]" style="color: var(--text-muted);">{{ $t('settings.qr.description') }}</p>
      <!-- eslint-disable-next-line vue/no-v-html -- friendQrSvg is generated locally by the `qrcode` lib from the user's own id (a UUID), never from user-controllable input -->
      <div v-if="friendQrSvg" class="w-[180px] rounded-[var(--radius-md)] bg-white p-3" v-html="friendQrSvg" />
    </section>

    <section class="flex flex-col gap-3.5 rounded-[var(--radius-xl)] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
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
          :disabled="isStartingImport || !user?.moxfield_username || !!importJob && isImportRunning(importJob.status)"
          class="rounded-full border px-4 py-2 text-[13px] disabled:opacity-50"
          style="border-color: var(--input-border); color: var(--text);"
          @click="handleStartImport"
        >
          {{ isStartingImport ? $t('settings.moxfield.starting') : $t('settings.moxfield.importAction') }}
        </button>

        <p v-if="importError" class="mt-3 text-sm" style="color: var(--lose);">{{ importError }}</p>

        <div v-if="importJob" class="mt-3 text-sm" style="color: var(--text-muted);">
          <p v-if="importJob.status === 'pending'">
            {{ $t('settings.moxfield.listing') }}
          </p>
          <p v-else-if="importJob.status === 'in_progress'">
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

    <section class="flex flex-col gap-3.5 rounded-[var(--radius-xl)] border p-[22px]" style="border-color: var(--card-border); background: var(--card-bg);">
      <h2 class="text-[15px] font-medium">{{ $t('settings.security.heading') }}</h2>

      <p v-if="!user?.has_password" class="text-[13px]" style="color: var(--text-muted);">
        {{ $t('settings.security.noPassword') }}
      </p>

      <form v-else class="flex flex-col gap-3" @submit.prevent="handleChangePassword">
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
