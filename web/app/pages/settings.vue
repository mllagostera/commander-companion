<script setup lang="ts">
import type { MoxfieldImportJob } from '~/types/api'

const { user, fetchSession } = useAuth()
const {
  updateMoxfieldUsername,
  changePassword,
  startMoxfieldImport,
  getMoxfieldImportStatus,
} = useSettings()

// ------------------------------------------------------------- contraseña
const currentPassword = ref('')
const newPassword = ref('')
const newPasswordConfirm = ref('')
const passwordError = ref('')
const passwordSuccess = ref(false)
const isChangingPassword = ref(false)

async function handleChangePassword() {
  passwordError.value = ''
  passwordSuccess.value = false

  if (newPassword.value !== newPasswordConfirm.value) {
    passwordError.value = 'Las contraseñas nuevas no coinciden.'
    return
  }

  isChangingPassword.value = true
  try {
    await changePassword(currentPassword.value, newPassword.value)
    passwordSuccess.value = true
    currentPassword.value = ''
    newPassword.value = ''
    newPasswordConfirm.value = ''
  } catch (err) {
    passwordError.value = changePasswordError(err)
  } finally {
    isChangingPassword.value = false
  }
}

// --------------------------------------------------------------- moxfield
const moxfieldUsername = ref(user.value?.moxfield_username ?? '')
const moxfieldError = ref('')
const moxfieldSuccess = ref(false)
const isSavingMoxfield = ref(false)

async function handleSaveMoxfieldUsername() {
  moxfieldError.value = ''
  moxfieldSuccess.value = false
  isSavingMoxfield.value = true
  try {
    await updateMoxfieldUsername(moxfieldUsername.value)
    await fetchSession()
    moxfieldSuccess.value = true
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
  <div class="space-y-8 max-w-lg">
    <section>
      <h1 class="text-2xl font-semibold">Ajustes</h1>
      <p class="mt-1 text-sm text-slate-400">
        {{ user?.username }} · {{ user?.email }}
      </p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <h2 class="font-medium">Cambiar contraseña</h2>

      <form class="mt-4 space-y-3" @submit.prevent="handleChangePassword">
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="current-password">
            Contraseña actual
          </label>
          <input
            id="current-password"
            v-model="currentPassword"
            type="password"
            autocomplete="current-password"
            required
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="new-password">
            Contraseña nueva
          </label>
          <input
            id="new-password"
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            required
            minlength="8"
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="new-password-confirm">
            Confirmar contraseña nueva
          </label>
          <input
            id="new-password-confirm"
            v-model="newPasswordConfirm"
            type="password"
            autocomplete="new-password"
            required
            minlength="8"
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>

        <p v-if="passwordError" class="text-sm text-red-400">{{ passwordError }}</p>
        <p v-if="passwordSuccess" class="text-sm text-emerald-400">Contraseña actualizada.</p>

        <button
          type="submit"
          :disabled="isChangingPassword"
          class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
        >
          {{ isChangingPassword ? 'Guardando…' : 'Cambiar contraseña' }}
        </button>
      </form>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <h2 class="font-medium">Cuenta de Moxfield</h2>
      <p class="mt-1 text-sm text-slate-400">
        Vinculá tu usuario de Moxfield para poder importar todos tus decks públicos en segundo plano.
      </p>

      <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="handleSaveMoxfieldUsername">
        <input
          id="moxfield-username"
          v-model="moxfieldUsername"
          type="text"
          placeholder="Tu usuario de Moxfield"
          class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
        >
        <button
          type="submit"
          :disabled="isSavingMoxfield"
          class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
        >
          {{ isSavingMoxfield ? 'Guardando…' : 'Guardar' }}
        </button>
      </form>

      <p v-if="moxfieldError" class="mt-3 text-sm text-red-400">{{ moxfieldError }}</p>
      <p v-if="moxfieldSuccess" class="mt-3 text-sm text-emerald-400">Usuario de Moxfield guardado.</p>

      <div class="mt-6 border-t border-slate-800 pt-4">
        <button
          type="button"
          :disabled="isStartingImport || !user?.moxfield_username || importJob?.status === 'in_progress'"
          class="rounded-lg border border-slate-700 px-4 py-2 text-sm hover:bg-slate-800 disabled:opacity-50"
          @click="handleStartImport"
        >
          {{ isStartingImport ? 'Iniciando…' : 'Importar mis decks en segundo plano' }}
        </button>

        <p v-if="importError" class="mt-3 text-sm text-red-400">{{ importError }}</p>

        <div v-if="importJob" class="mt-3 text-sm text-slate-400">
          <p v-if="importJob.status === 'in_progress'">
            Importando… {{ importJob.imported_count + importJob.failed_count }}
            <template v-if="importJob.total_decks !== null"> / {{ importJob.total_decks }}</template>
            decks procesados.
          </p>
          <p v-else-if="importJob.status === 'completed'" class="text-emerald-400">
            Listo: {{ importJob.imported_count }} decks importados, {{ importJob.failed_count }} fallidos.
          </p>
          <p v-else-if="importJob.status === 'failed'" class="text-red-400">
            La importación falló. {{ importJob.error_message }}
          </p>
        </div>
      </div>
    </section>
  </div>
</template>
