<script setup lang="ts">
definePageMeta({ layout: false })

const { login, loginWithGoogle, resendVerification } = useAuth()
const { renderButton } = useGoogleIdentity()

const email = ref('')
const password = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)
const googleButtonRef = ref<HTMLElement | null>(null)
// 403 de /auth/login: la cuenta existe y la contraseña es correcta, pero el email
// todavía no se confirmó (ver server/api/auth/login.post.ts).
const needsVerification = ref(false)
const isResending = ref(false)
const resendSent = ref(false)

async function handleSubmit() {
  errorMessage.value = ''
  needsVerification.value = false
  resendSent.value = false
  isSubmitting.value = true
  try {
    await login(email.value, password.value)
    await navigateTo('/')
  } catch (err) {
    if (apiErrorStatus(err) === 403) {
      needsVerification.value = true
    }
    errorMessage.value = apiErrorMessage(err, 'No se pudo iniciar sesión.')
  } finally {
    isSubmitting.value = false
  }
}

async function handleResendVerification() {
  isResending.value = true
  try {
    await resendVerification(email.value)
    resendSent.value = true
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'No se pudo reenviar el email de verificación.')
  } finally {
    isResending.value = false
  }
}

async function handleGoogleCredential(idToken: string) {
  errorMessage.value = ''
  try {
    await loginWithGoogle(idToken)
    await navigateTo('/')
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'No se pudo iniciar sesión con Google.')
  }
}

onMounted(() => {
  if (googleButtonRef.value) {
    renderButton(googleButtonRef.value, handleGoogleCredential)
  }
})
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-slate-950 text-slate-100 p-6">
    <div class="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900/60 p-8">
      <h1 class="text-xl font-semibold text-center">Commander Companion</h1>
      <p class="mt-1 text-center text-sm text-slate-400">Iniciá sesión para continuar</p>

      <form class="mt-6 space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="email">Email</label>
          <input
            id="email"
            v-model="email"
            type="email"
            autocomplete="email"
            required
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="password">Contraseña</label>
          <input
            id="password"
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>

        <p v-if="errorMessage" class="text-sm text-red-400">{{ errorMessage }}</p>

        <div v-if="needsVerification" class="text-sm">
          <p v-if="resendSent" class="text-slate-400">
            Listo, revisá tu bandeja de entrada.
          </p>
          <button
            v-else
            type="button"
            :disabled="isResending"
            class="text-indigo-400 hover:text-indigo-300 disabled:opacity-50"
            @click="handleResendVerification"
          >
            {{ isResending ? 'Reenviando…' : 'Reenviar email de verificación' }}
          </button>
        </div>

        <button
          type="submit"
          :disabled="isSubmitting"
          class="w-full rounded-lg bg-indigo-500 py-2 font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
        >
          {{ isSubmitting ? 'Ingresando…' : 'Iniciar sesión' }}
        </button>
      </form>

      <div class="mt-6 flex items-center gap-3 text-xs text-slate-500">
        <span class="h-px flex-1 bg-slate-800" />
        o
        <span class="h-px flex-1 bg-slate-800" />
      </div>

      <div ref="googleButtonRef" class="mt-4 flex justify-center" />

      <p class="mt-6 text-center text-sm text-slate-400">
        ¿No tenés cuenta?
        <NuxtLink to="/register" class="text-indigo-400 hover:text-indigo-300">
          Registrate
        </NuxtLink>
      </p>
    </div>
  </main>
</template>
