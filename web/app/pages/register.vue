<script setup lang="ts">
definePageMeta({ layout: false })

const { register, checkUsernameAvailable } = useAuth()
const { t } = useI18n()

const username = ref('')
const email = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)
// Successful registration but no session: the backend leaves the email unconfirmed (see
// server/api/auth/register.post.ts), so instead of navigating to the dashboard we show
// this screen within the form itself.
const registeredEmail = ref('')

type UsernameStatus = 'idle' | 'checking' | 'available' | 'taken'
const usernameStatus = ref<UsernameStatus>('idle')
// Guards against a stale response landing after the user has already changed
// the field again (e.g. checked "foo", edited to "foobar" before "foo" resolved).
let usernameCheckToken = 0

async function handleUsernameChange() {
  const value = username.value.trim()
  if (!value) {
    usernameStatus.value = 'idle'
    return
  }

  const token = ++usernameCheckToken
  usernameStatus.value = 'checking'
  try {
    const available = await checkUsernameAvailable(value)
    if (token !== usernameCheckToken) return
    usernameStatus.value = available ? 'available' : 'taken'
  } catch {
    // Best-effort: the real check still happens server-side on submit.
    if (token === usernameCheckToken) usernameStatus.value = 'idle'
  }
}

async function handleSubmit() {
  errorMessage.value = ''

  if (usernameStatus.value === 'taken') {
    errorMessage.value = t('register.errors.usernameTaken')
    return
  }

  if (password.value !== passwordConfirm.value) {
    errorMessage.value = t('register.errors.passwordMismatch')
    return
  }

  isSubmitting.value = true
  try {
    await register(username.value, email.value, password.value)
    registeredEmail.value = email.value
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, t('register.errors.registerFailed'))
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
          <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">{{ $t('register.checkEmail.title') }}</span>
        </span>

        <div
          class="flex w-full max-w-[340px] flex-col gap-4 rounded-[28px] border p-[26px] text-center"
          style="background: var(--card-bg-strong); border-color: var(--card-border);"
        >
          <p class="text-sm" style="color: var(--text-muted);">
            {{ $t('register.checkEmail.bodyIntro') }} <strong style="color: var(--text);">{{ registeredEmail }}</strong>.
            {{ $t('register.checkEmail.bodyOutro') }}
          </p>

          <NuxtLink
            to="/login"
            class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('register.checkEmail.goToLogin') }}
          </NuxtLink>
        </div>
      </template>

      <template v-else>
        <span class="flex flex-col items-center gap-3.5">
          <AppLogo size="lg" />
          <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">Commander Companion</span>
          <span class="text-[13px]" style="color: var(--text-muted);">{{ $t('register.tagline') }}</span>
        </span>

        <form
          class="flex w-full max-w-[340px] flex-col gap-3 rounded-[28px] border p-[26px]"
          style="background: var(--card-bg-strong); border-color: var(--card-border);"
          @submit.prevent="handleSubmit"
        >
          <label class="text-xs" style="color: var(--text-dim);">
            {{ $t('register.usernameLabel') }}
            <input
              v-model="username"
              type="text"
              autocomplete="username"
              required
              :placeholder="$t('register.usernamePlaceholder')"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              :style="{
                background: 'var(--input-bg)',
                borderColor: usernameStatus === 'taken' ? 'var(--lose)' : 'var(--input-border)',
                color: 'var(--text)',
              }"
              @input="usernameStatus = 'idle'"
              @change="handleUsernameChange"
            >
            <span v-if="usernameStatus === 'checking'" class="mt-1 block text-xs" style="color: var(--text-dim);">
              {{ $t('register.username.checking') }}
            </span>
            <span v-else-if="usernameStatus === 'available'" class="mt-1 block text-xs" style="color: var(--win);">
              {{ $t('register.username.available') }}
            </span>
            <span v-else-if="usernameStatus === 'taken'" class="mt-1 block text-xs" style="color: var(--lose);">
              {{ $t('register.username.taken') }}
            </span>
          </label>
          <label class="text-xs" style="color: var(--text-dim);">
            {{ $t('register.emailLabel') }}
            <input
              v-model="email"
              type="email"
              autocomplete="email"
              required
              :placeholder="$t('register.emailPlaceholder')"
              class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </label>
          <label class="text-xs" style="color: var(--text-dim);">
            {{ $t('register.passwordLabel') }}
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
            {{ $t('register.passwordConfirmLabel') }}
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
            {{ isSubmitting ? $t('register.submitting') : $t('register.submit') }}
          </button>

          <span class="text-center text-xs" style="color: var(--text-dim);">
            {{ $t('register.haveAccount') }}
            <NuxtLink to="/login" style="color: var(--accent-link);">{{ $t('register.loginLink') }}</NuxtLink>
          </span>
        </form>
      </template>
    </div>
  </main>
</template>
