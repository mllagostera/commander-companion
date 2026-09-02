<script setup lang="ts">
definePageMeta({ layout: false })

const { login, loginWithGoogle, resendVerification } = useAuth()
const { renderButton } = useGoogleIdentity()
const { theme } = useTheme()
const { t } = useI18n()

/**
 * Where to land after logging in. `auth.global.ts` puts the blocked
 * destination here so a deep link (a scanned profile QR, see
 * pages/friends/add/[id].vue) survives the trip through login.
 *
 * Only same-site absolute paths are honoured: the value comes from the query
 * string, so accepting `//evil.com` or `https://evil.com` would turn this
 * page into an open redirect that a phishing link could point anywhere.
 */
const route = useRoute()
const redirectTarget = computed(() => {
  const raw = route.query.redirect
  const path = Array.isArray(raw) ? raw[0] : raw
  if (typeof path !== 'string') return '/'
  return path.startsWith('/') && !path.startsWith('//') ? path : '/'
})

const email = ref('')
const password = ref('')
const errorMessage = ref('')
const isSubmitting = ref(false)
const googleButtonRef = ref<HTMLElement | null>(null)
// 403 from /auth/login: the account exists and the password is correct, but the email
// hasn't been confirmed yet (see server/api/auth/login.post.ts).
const needsVerification = ref(false)
const isResending = ref(false)
const resendSent = ref(false)

// The backend may be asleep (cold start) and take ~50s to respond to the
// first request. Without this notice the user has no way to know and
// tends to retry the login thinking nothing happened.
const { active: showSlowHint, start: startSlowHint, stop: stopSlowHint } = useSlowRequestHint()

/**
 * Set once the credentials are accepted and never cleared: the destination page
 * still has to mount and fetch its own data after `navigateTo` resolves, and
 * this screen stays visible during that gap. Hiding the overlay as soon as the
 * request finished put the form back in reach and users submitted again
 * mid-navigation. It goes away with the page itself, on unmount.
 */
const isNavigating = ref(false)

// Drives the overlay, which also acts as the click blocker for the whole form
// (including the Google iframe button, which can't be `disabled`).
const isBusy = computed(() => isSubmitting.value || isNavigating.value)

async function handleSubmit() {
  if (isBusy.value) return

  errorMessage.value = ''
  needsVerification.value = false
  resendSent.value = false
  isSubmitting.value = true
  startSlowHint()
  try {
    await login(email.value, password.value)
    isNavigating.value = true
  } catch (err) {
    if (apiErrorStatus(err) === 403) {
      needsVerification.value = true
    }
    errorMessage.value = apiErrorMessage(err, t('login.errors.loginFailed'))
  } finally {
    isSubmitting.value = false
    if (!isNavigating.value) stopSlowHint()
  }

  if (isNavigating.value) await navigateTo(redirectTarget.value)
}

async function handleResendVerification() {
  isResending.value = true
  try {
    await resendVerification(email.value)
    resendSent.value = true
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, t('login.errors.resendFailed'))
  } finally {
    isResending.value = false
  }
}

async function handleGoogleCredential(idToken: string) {
  if (isBusy.value) return

  errorMessage.value = ''
  needsVerification.value = false
  isSubmitting.value = true
  startSlowHint()
  try {
    await loginWithGoogle(idToken)
    isNavigating.value = true
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, t('login.errors.googleFailed'))
  } finally {
    isSubmitting.value = false
    if (!isNavigating.value) stopSlowHint()
  }

  if (isNavigating.value) await navigateTo(redirectTarget.value)
}

onMounted(() => {
  if (googleButtonRef.value) {
    renderButton(googleButtonRef.value, handleGoogleCredential, {
      theme: theme.value === 'light' ? 'outline' : 'filled_black',
    })
  }
})
</script>

<template>
  <main class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

    <!--
      `inert` takes the whole form out of reach while a login is in flight: the
      overlay already swallows clicks, but not keyboard focus, and the Google
      button is a GSI iframe that has no `disabled` of its own.
    -->
    <div
      class="relative z-[1] flex min-h-screen flex-col items-center justify-center gap-8 p-6"
      :inert="isBusy || undefined"
    >
      <span class="flex flex-col items-center gap-3.5">
        <AppLogo size="lg" />
        <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">Commander Companion</span>
        <span class="text-[13px]" style="color: var(--text-muted);">{{ $t('login.tagline') }}</span>
      </span>

      <form
        class="flex w-full max-w-[340px] flex-col gap-3 rounded-[var(--radius-xl)] border p-[26px]"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
        @submit.prevent="handleSubmit"
      >
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('login.emailLabel') }}
          <input
            v-model="email"
            type="email"
            autocomplete="email"
            required
            :disabled="isBusy"
            :placeholder="$t('login.emailPlaceholder')"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>
        <label class="text-xs" style="color: var(--text-dim);">
          {{ $t('login.passwordLabel') }}
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            :disabled="isBusy"
            placeholder="••••••••"
            class="mt-1.5 w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
        </label>

        <p v-if="errorMessage" class="text-sm" style="color: var(--lose);">{{ errorMessage }}</p>

        <div v-if="needsVerification" class="text-sm">
          <p v-if="resendSent" style="color: var(--text-muted);">
            {{ $t('login.verification.checkInbox') }}
          </p>
          <button
            v-else
            type="button"
            :disabled="isResending || isBusy"
            class="disabled:opacity-50"
            style="color: var(--accent-link);"
            @click="handleResendVerification"
          >
            {{ isResending ? $t('login.verification.resending') : $t('login.verification.resend') }}
          </button>
        </div>

        <button
          type="submit"
          :disabled="isBusy"
          class="mt-2 rounded-full px-5 py-3 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.02] disabled:opacity-50"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ isBusy ? $t('login.submitting') : $t('login.submit') }}
        </button>

        <div class="my-1 flex items-center gap-2.5">
          <span class="h-px flex-1" style="background: var(--card-border);" />
          <span class="text-[11px]" style="color: var(--text-dim);">{{ $t('login.divider') }}</span>
          <span class="h-px flex-1" style="background: var(--card-border);" />
        </div>

        <div ref="googleButtonRef" class="flex justify-center" />

        <span class="text-center text-xs" style="color: var(--text-dim);">
          {{ $t('login.noAccount') }}
          <NuxtLink to="/register" style="color: var(--accent-link);">{{ $t('login.registerLink') }}</NuxtLink>
        </span>
      </form>
    </div>

    <Transition name="cc-fade">
      <div
        v-if="isBusy"
        class="fixed inset-0 z-50 flex flex-col items-center justify-center gap-5 p-6 text-center"
        style="background: rgba(5, 3, 8, 0.72); backdrop-filter: blur(6px);"
      >
        <AppLogo size="lg" pulse />
        <div class="flex flex-col items-center gap-2" role="status" aria-live="polite">
          <span class="text-sm font-medium" style="color: var(--text);">{{ $t('login.loading.title') }}</span>
          <Transition name="cc-fade">
            <span v-if="showSlowHint" class="max-w-[280px] text-xs" style="color: var(--text-muted);">
              {{ $t('login.loading.slowHint') }}
            </span>
          </Transition>
        </div>
      </div>
    </Transition>
  </main>
</template>

<style scoped>
.cc-fade-enter-active,
.cc-fade-leave-active {
  transition: opacity 0.25s ease;
}
.cc-fade-enter-from,
.cc-fade-leave-to {
  opacity: 0;
}
</style>
