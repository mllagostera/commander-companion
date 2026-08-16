<script setup lang="ts">
definePageMeta({ layout: false })

const { verifyEmail } = useAuth()
const route = useRoute()
const { t } = useI18n()

const status = ref<'pending' | 'success' | 'error'>('pending')
const errorMessage = ref('')

onMounted(async () => {
  const token = route.query.token
  if (typeof token !== 'string' || !token) {
    status.value = 'error'
    errorMessage.value = t('verifyEmail.invalidLink')
    return
  }

  try {
    await verifyEmail(token)
    status.value = 'success'
  } catch (err) {
    status.value = 'error'
    errorMessage.value = apiErrorMessage(err, t('verifyEmail.verifyFailed'))
  }
})
</script>

<template>
  <main class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

    <div class="relative z-[1] flex min-h-screen flex-col items-center justify-center gap-8 p-6">
      <span class="flex flex-col items-center gap-3.5">
        <AppLogo size="lg" />
        <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">Commander Companion</span>
      </span>

      <div
        class="flex w-full max-w-[340px] flex-col gap-4 rounded-[var(--radius-xl)] border p-[26px] text-center"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
      >
        <template v-if="status === 'pending'">
          <h1 class="text-xl font-semibold">{{ $t('verifyEmail.pending') }}</h1>
        </template>

        <template v-else-if="status === 'success'">
          <h1 class="text-xl font-semibold">{{ $t('verifyEmail.success.title') }}</h1>
          <p class="text-sm" style="color: var(--text-muted);">{{ $t('verifyEmail.success.body') }}</p>
          <NuxtLink
            to="/login"
            class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('verifyEmail.success.goToLogin') }}
          </NuxtLink>
        </template>

        <template v-else>
          <h1 class="text-xl font-semibold">{{ $t('verifyEmail.error.title') }}</h1>
          <p class="text-sm" style="color: var(--lose);">{{ errorMessage }}</p>
          <p class="text-sm" style="color: var(--text-muted);">
            {{ $t('verifyEmail.error.requestNewLinkIntro') }}
            <NuxtLink to="/login" style="color: var(--accent-link);">{{ $t('verifyEmail.error.loginLink') }}</NuxtLink>.
          </p>
        </template>
      </div>
    </div>
  </main>
</template>
