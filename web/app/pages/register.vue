<script setup lang="ts">
definePageMeta({ layout: false })

const { register } = useAuth()

const username = ref('')
const email = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)
// Registro exitoso pero sin sesión: el backend deja el email sin confirmar (ver
// server/api/auth/register.post.ts), así que en vez de navegar al dashboard mostramos
// esta pantalla en el propio formulario.
const registeredEmail = ref('')

async function handleSubmit() {
  errorMessage.value = ''

  if (password.value !== passwordConfirm.value) {
    errorMessage.value = 'Las contraseñas no coinciden.'
    return
  }

  isSubmitting.value = true
  try {
    await register(username.value, email.value, password.value)
    registeredEmail.value = email.value
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'No se pudo crear la cuenta.')
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <main class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

    <div class="relative z-[1] flex min-h-screen flex-col items-center justify-center gap-8 p-6">
      <template v-if="registeredEmail">
        <span class="flex flex-col items-center gap-3.5">
          <AppLogo size="lg" />
          <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">Revisá tu email</span>
        </span>

        <div
          class="flex w-full max-w-[340px] flex-col gap-4 rounded-[28px] border p-[26px] text-center"
          style="background: var(--card-bg-strong); border-color: var(--card-border);"
        >
          <p class="text-sm" style="color: var(--text-muted);">
            Te mandamos un link de confirmación a <strong style="color: var(--text);">{{ registeredEmail }}</strong>.
            Tenés que confirmarlo antes de poder iniciar sesión.
          </p>

          <NuxtLink
            to="/login"
            class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            Ir a iniciar sesión
          </NuxtLink>
        </div>
      </template>

      <template v-else>
        <span class="flex flex-col items-center gap-3.5">
          <AppLogo size="lg" />
          <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">Commander Companion</span>
          <span class="text-[13px]" style="color: var(--text-muted);">Creá tu cuenta y empezá a trackear tus partidas.</span>
        </span>

        <form
          class="flex w-full max-w-[340px] flex-col gap-3 rounded-[28px] border p-[26px]"
          style="background: var(--card-bg-strong); border-color: var(--card-border);"
          @submit.prevent="handleSubmit"
        >
          <label class="text-xs" style="color: var(--text-dim);">
            Usuario
            <input
              v-model="username"
              type="text"
              autocomplete="username"
              required
              placeholder="tu_usuario"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </label>
          <label class="text-xs" style="color: var(--text-dim);">
            Email
            <input
              v-model="email"
              type="email"
              autocomplete="email"
              required
              placeholder="tu@email.com"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </label>
          <label class="text-xs" style="color: var(--text-dim);">
            Contraseña
            <input
              v-model="password"
              type="password"
              autocomplete="new-password"
              required
              minlength="8"
              placeholder="••••••••"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </label>
          <label class="text-xs" style="color: var(--text-dim);">
            Repetir contraseña
            <input
              v-model="passwordConfirm"
              type="password"
              autocomplete="new-password"
              required
              placeholder="••••••••"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </label>

          <p v-if="errorMessage" class="text-sm" style="color: var(--lose);">{{ errorMessage }}</p>

          <button
            type="submit"
            :disabled="isSubmitting"
            class="mt-2 rounded-full px-5 py-3 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.02] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ isSubmitting ? 'Creando cuenta…' : 'Crear cuenta' }}
          </button>

          <span class="text-center text-xs" style="color: var(--text-dim);">
            ¿Ya tenés cuenta?
            <NuxtLink to="/login" style="color: var(--accent-link);">Iniciá sesión</NuxtLink>
          </span>
        </form>
      </template>
    </div>
  </main>
</template>
