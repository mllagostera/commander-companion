<script setup lang="ts">
definePageMeta({ layout: false })

const { resetPassword } = useAuth()
const route = useRoute()
const { t } = useI18n()

useSeoMeta({
  title: () => t('resetPassword.meta.title'),
})

// Mirrors the backend's minPasswordLength (internal/users/service.go).
const MIN_PASSWORD_LENGTH = 8

const token = computed(() => {
  const raw = route.query.token
  return typeof raw === 'string' ? raw : ''
})

const password = ref('')
const passwordConfirm = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)
const isDone = ref(false)
// A dead link (unknown, used or expired token) can't be fixed by retyping the
// password, so the form gives way to a "request a new link" message instead.
const isLinkInvalid = ref(!token.value)

async function handleSubmit() {
  if (isSubmitting.value) return

  errorMessage.value = ''
  if (password.value.length < MIN_PASSWORD_LENGTH) {
    errorMessage.value = t('resetPassword.errors.passwordTooShort')
    return
  }
  if (password.value !== passwordConfirm.value) {
    errorMessage.value = t('resetPassword.errors.passwordMismatch')
    return
  }

  isSubmitting.value = true
  try {
    await resetPassword(token.value, password.value)
    isDone.value = true
  } catch (err) {
    if (isInvalidResetTokenError(err)) {
      isLinkInvalid.value = true
    } else {
      errorMessage.value = resetPasswordError(err)
    }
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
      <span class="flex flex-col items-center gap-3.5">
        <AppLogo size="lg" />
        <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">TapeandoCartones</span>
      </span>

      <div
        v-if="isDone || isLinkInvalid"
        class="flex w-full max-w-[340px] flex-col gap-4 rounded-[var(--radius-xl)] border p-[26px] text-center"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
        role="status"
      >
        <template v-if="isDone">
          <h1 class="text-xl font-semibold">{{ $t('resetPassword.success.title') }}</h1>
          <p class="text-sm" style="color: var(--text-muted);">{{ $t('resetPassword.success.body') }}</p>
          <NuxtLink
            to="/login"
            class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('resetPassword.success.goToLogin') }}
          </NuxtLink>
        </template>

        <template v-else>
          <h1 class="text-xl font-semibold">{{ $t('resetPassword.invalidLink.title') }}</h1>
          <p class="text-sm" style="color: var(--lose);">{{ $t('resetPassword.errors.invalidOrExpired') }}</p>
          <NuxtLink
            to="/forgot-password"
            class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('resetPassword.invalidLink.requestNew') }}
          </NuxtLink>
        </template>
      </div>

      <form
        v-else
        class="flex w-full max-w-[340px] flex-col gap-3 rounded-[var(--radius-xl)] border p-[26px]"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
        @submit.prevent="handleSubmit"
      >
        <h1 class="text-xl font-semibold">{{ $t('resetPassword.title') }}</h1>
        <p class="text-sm" style="color: var(--text-muted);">{{ $t('resetPassword.intro') }}</p>

        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('resetPassword.passwordLabel') }}
          <input
            v-model="password"
            type="password"
            autocomplete="new-password"
            required
            :minlength="MIN_PASSWORD_LENGTH"
            :disabled="isSubmitting"
            placeholder="••••••••"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('resetPassword.passwordConfirmLabel') }}
          <input
            v-model="passwordConfirm"
            type="password"
            autocomplete="new-password"
            required
            :minlength="MIN_PASSWORD_LENGTH"
            :disabled="isSubmitting"
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
          {{ isSubmitting ? $t('resetPassword.submitting') : $t('resetPassword.submit') }}
        </button>
      </form>
    </div>
  </main>
</template>
