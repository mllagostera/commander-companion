<script setup lang="ts">
definePageMeta({ layout: false })

const { requestPasswordReset } = useAuth()
const route = useRoute()
const { t } = useI18n()

useSeoMeta({
  title: () => t('forgotPassword.meta.title'),
})

// Prefilled from login.vue's link, so whoever already typed their email there doesn't
// have to type it again.
const initialEmail = route.query.email
const email = ref(typeof initialEmail === 'string' ? initialEmail : '')
const errorMessage = ref('')
const isSubmitting = ref(false)
// The backend answers the same whether the email has an account or not (see
// server/api/auth/forgot-password.post.ts), so this only means "request accepted".
const sentTo = ref('')

async function handleSubmit() {
  if (isSubmitting.value) return

  errorMessage.value = ''
  isSubmitting.value = true
  try {
    await requestPasswordReset(email.value)
    sentTo.value = email.value
  } catch (err) {
    errorMessage.value = forgotPasswordError(err)
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
        v-if="sentTo"
        class="flex w-full max-w-[340px] flex-col gap-4 rounded-[var(--radius-xl)] border p-[26px] text-center"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
        role="status"
      >
        <h1 class="text-xl font-semibold">{{ $t('forgotPassword.sent.title') }}</h1>
        <p class="text-sm" style="color: var(--text-muted);">
          {{ $t('forgotPassword.sent.bodyIntro') }}
          <strong style="color: var(--text);">{{ sentTo }}</strong>{{ $t('forgotPassword.sent.bodyOutro') }}
        </p>
        <NuxtLink
          to="/login"
          class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ $t('forgotPassword.backToLogin') }}
        </NuxtLink>
      </div>

      <form
        v-else
        class="flex w-full max-w-[340px] flex-col gap-3 rounded-[var(--radius-xl)] border p-[26px]"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
        @submit.prevent="handleSubmit"
      >
        <h1 class="text-xl font-semibold">{{ $t('forgotPassword.title') }}</h1>
        <p class="text-sm" style="color: var(--text-muted);">{{ $t('forgotPassword.intro') }}</p>

        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('forgotPassword.emailLabel') }}
          <input
            v-model="email"
            type="email"
            autocomplete="email"
            required
            :disabled="isSubmitting"
            :placeholder="$t('login.emailPlaceholder')"
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
          {{ isSubmitting ? $t('forgotPassword.submitting') : $t('forgotPassword.submit') }}
        </button>

        <NuxtLink to="/login" class="text-center text-xs" style="color: var(--accent-link);">
          {{ $t('forgotPassword.backToLogin') }}
        </NuxtLink>
      </form>
    </div>
  </main>
</template>
