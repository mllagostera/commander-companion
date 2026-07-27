<script setup lang="ts">
definePageMeta({ layout: false })

const { register } = useAuth()

const username = ref('')
const email = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)

async function handleSubmit() {
  errorMessage.value = ''

  if (password.value !== passwordConfirm.value) {
    errorMessage.value = 'Las contraseñas no coinciden.'
    return
  }

  isSubmitting.value = true
  try {
    // Nitro registra y deja la sesión iniciada de una (ver
    // server/api/auth/register.post.ts).
    await register(username.value, email.value, password.value)
    await navigateTo('/')
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'No se pudo crear la cuenta.')
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-slate-950 text-slate-100 p-6">
    <div class="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900/60 p-8">
      <h1 class="text-xl font-semibold text-center">Crear cuenta</h1>
      <p class="mt-1 text-center text-sm text-slate-400">
        Empezá a registrar tus partidas de Commander
      </p>

      <form class="mt-6 space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="username">Usuario</label>
          <input
            id="username"
            v-model="username"
            type="text"
            autocomplete="username"
            required
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>
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
            autocomplete="new-password"
            required
            minlength="8"
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>
        <div>
          <label class="block text-sm text-slate-400 mb-1" for="password-confirm">
            Repetir contraseña
          </label>
          <input
            id="password-confirm"
            v-model="passwordConfirm"
            type="password"
            autocomplete="new-password"
            required
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
        </div>

        <p v-if="errorMessage" class="text-sm text-red-400">{{ errorMessage }}</p>

        <button
          type="submit"
          :disabled="isSubmitting"
          class="w-full rounded-lg bg-indigo-500 py-2 font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
        >
          {{ isSubmitting ? 'Creando cuenta…' : 'Crear cuenta' }}
        </button>
      </form>

      <p class="mt-6 text-center text-sm text-slate-400">
        ¿Ya tenés cuenta?
        <NuxtLink to="/login" class="text-indigo-400 hover:text-indigo-300">
          Iniciá sesión
        </NuxtLink>
      </p>
    </div>
  </main>
</template>
