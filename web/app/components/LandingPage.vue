<script setup lang="ts">
/**
 * Public landing page, shown at `/` to visitors without a session (see
 * pages/index.vue). Explains what the app does and funnels to /register.
 *
 * Screenshots live in public/landing/: WebP captures of the running app
 * (1440x900, Spanish locale, dark theme) against a local stack seeded with
 * sample users, decks, games and a tournament. Re-capture them when the
 * screens they show change noticeably.
 */
const { t, locale, locales, setLocale, localeProperties } = useI18n()

const availableLocales = computed(() => locales.value as { code: string, name?: string }[])

// og:image must be absolute: link-preview crawlers (WhatsApp, Discord, X...)
// don't resolve relative URLs. The image is public/og-image.png, 1200x630,
// the size every major platform crops to without losing anything. Bump `v`
// whenever the image changes: platforms cache previews by image URL.
// Two descriptions on purpose: Google shows ~155 characters of `description`,
// while link previews (WhatsApp, X, LinkedIn) cut around 125.
const siteUrl = useSiteUrl()
const ogImage = `${siteUrl}/og-image.png?v=2`
useSeoMeta({
  title: () => t('landing.meta.title'),
  description: () => t('landing.meta.description'),
  ogTitle: () => t('landing.meta.title'),
  ogDescription: () => t('landing.meta.socialDescription'),
  ogType: 'website',
  ogLocale: () => (localeProperties.value.language ?? 'es-ES').replace('-', '_'),
  ogImage,
  ogImageWidth: 1200,
  ogImageHeight: 630,
  ogImageType: 'image/png',
  ogImageAlt: () => t('landing.meta.imageAlt'),
  twitterTitle: () => t('landing.meta.title'),
  twitterDescription: () => t('landing.meta.socialDescription'),
  twitterImage: ogImage,
  twitterImageAlt: () => t('landing.meta.imageAlt'),
})

// Structured data (schema.org JSON-LD) so search engines read the page as a
// free web app rather than inferring it from the copy. Google only shows a
// rich result for SoftwareApplication with ratings, which there are none of
// yet; the rest still feeds the knowledge panel and sitelinks. No CSP change
// needed: security-headers.ts hashes every inline <script> in the response.
const structuredData = computed(() => JSON.stringify({
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'Organization',
      '@id': `${siteUrl}/#organization`,
      'name': 'TapeandoCartones',
      'url': `${siteUrl}/`,
      'logo': `${siteUrl}/icon-512.png`,
    },
    {
      '@type': 'WebSite',
      '@id': `${siteUrl}/#website`,
      'name': 'TapeandoCartones',
      'url': `${siteUrl}/`,
      'inLanguage': ['es', 'en', 'ca'],
      'publisher': { '@id': `${siteUrl}/#organization` },
    },
    {
      '@type': 'WebApplication',
      'name': 'TapeandoCartones',
      'url': `${siteUrl}/`,
      'description': t('landing.meta.description'),
      'image': ogImage,
      'applicationCategory': 'GameApplication',
      'operatingSystem': 'Web, Android',
      'inLanguage': ['es', 'en', 'ca'],
      'offers': { '@type': 'Offer', 'price': '0', 'priceCurrency': 'EUR' },
      'publisher': { '@id': `${siteUrl}/#organization` },
    },
  ],
}))
useHead({
  script: [{ type: 'application/ld+json', innerHTML: structuredData }],
})

/*
 * 24x24 stroke icons, drawn inline so the page needs no icon dependency. Each
 * entry is a list of SVG path `d` values.
 */
const icons = {
  heart: ['M19 14c1.5-1.5 3-3.2 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.8 0-3 .5-4.5 2-1.5-1.5-2.7-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4 3 5.5l7 7Z'],
  download: ['M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4', 'M7 10l5 5 5-5', 'M12 15V3'],
  chart: ['M3 3v18h18', 'M7 16l4-5 3 3 5-7'],
  users: ['M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2', 'M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z', 'M22 21v-2a4 4 0 0 0-3-3.87', 'M16 3.13a4 4 0 0 1 0 7.75'],
  trophy: ['M6 9H4.5a2.5 2.5 0 0 1 0-5H6', 'M18 9h1.5a2.5 2.5 0 0 0 0-5H18', 'M4 22h16', 'M10 14.66V17c0 .55-.47.98-.97 1.21C7.85 18.75 7 20.24 7 22', 'M14 14.66V17c0 .55.47.98.97 1.21C16.15 18.75 17 20.24 17 22', 'M18 2H6v7a6 6 0 0 0 12 0V2Z'],
  qr: ['M3 3h7v7H3z', 'M14 3h7v7h-7z', 'M3 14h7v7H3z', 'M14 14h3v3h-3z', 'M20 14v.01', 'M14 20h.01', 'M17 20h4v-3'],
}

const features = computed(() => [
  { icon: icons.heart, key: 'tracker' },
  { icon: icons.download, key: 'decks' },
  { icon: icons.chart, key: 'statistics' },
  { icon: icons.users, key: 'playgroups' },
  { icon: icons.trophy, key: 'tournaments' },
  { icon: icons.qr, key: 'friends' },
].map(f => ({
  icon: f.icon,
  title: t(`landing.features.${f.key}.title`),
  body: t(`landing.features.${f.key}.body`),
})))

const pillars = computed(() => ['speed', 'simplicity', 'data'].map(key => ({
  title: t(`landing.pillars.${key}.title`),
  body: t(`landing.pillars.${key}.body`),
})))

const showcase = computed(() => [
  { src: '/landing/life-tracker.webp', width: 1440, height: 900, key: 'tracker' },
  { src: '/landing/tournament.webp', width: 1080, height: 930, key: 'tournaments' },
].map(s => ({
  ...s,
  eyebrow: t(`landing.showcase.${s.key}.eyebrow`),
  title: t(`landing.showcase.${s.key}.title`),
  body: t(`landing.showcase.${s.key}.body`),
  alt: t(`landing.showcase.${s.key}.alt`),
})))
</script>

<template>
  <div class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-full focus:px-5 focus:py-2.5 focus:text-[13px] focus:font-semibold"
      style="background: var(--accent-link); color: var(--page-solid);"
    >
      {{ $t('common.skipToContent') }}
    </a>

    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

    <header class="sticky top-0 z-10 px-4 pt-4 sm:px-6 sm:pt-[18px]">
      <div
        class="mx-auto flex max-w-[1080px] items-center gap-3 rounded-full border px-4 py-2.5 shadow-[0_8px_30px_rgba(0,0,0,0.2)] backdrop-blur-xl sm:gap-5 sm:px-[22px]"
        style="background: var(--header-bg); border-color: var(--card-border);"
      >
        <NuxtLink to="/" class="flex items-center gap-2.5">
          <AppLogo />
          <span class="cc-gradient-text hidden text-[15px] font-semibold tracking-wide sm:inline">TapeandoCartones</span>
        </NuxtLink>

        <div class="ml-auto flex items-center gap-1" role="group" :aria-label="$t('nav.language')">
          <button
            v-for="l in availableLocales"
            :key="l.code"
            type="button"
            :aria-pressed="l.code === locale"
            :title="l.name"
            class="rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase transition-colors"
            :style="l.code === locale
              ? { background: 'linear-gradient(135deg, #8b5cf6, #a855f7)', color: '#0a0714', borderColor: 'transparent' }
              : { color: 'var(--text-muted)', borderColor: 'var(--input-border)' }"
            @click="setLocale(l.code as 'es' | 'en' | 'ca')"
          >
            {{ l.code }}
          </button>
        </div>

        <NuxtLink to="/login" class="hidden text-sm sm:inline" style="color: var(--text-muted);">
          {{ $t('landing.nav.login') }}
        </NuxtLink>
        <NuxtLink
          to="/register"
          class="whitespace-nowrap rounded-full px-4 py-2 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.04]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ $t('landing.nav.register') }}
        </NuxtLink>
      </div>
    </header>

    <main id="main-content" class="relative z-[1] mx-auto flex max-w-[1080px] flex-col gap-24 px-4 pb-20 pt-14 sm:px-6 sm:pt-20">
      <!-- Hero -->
      <section class="flex flex-col items-center gap-10 text-center">
        <div class="flex max-w-[720px] flex-col items-center gap-5">
          <span
            class="rounded-full border px-3.5 py-1 text-[12px] font-medium"
            style="border-color: var(--card-border); background: var(--card-bg-strong); color: var(--accent-link);"
          >
            {{ $t('landing.hero.eyebrow') }}
          </span>
          <h1 class="text-[34px] font-semibold leading-[1.1] tracking-tight sm:text-[52px]">
            {{ $t('landing.hero.titleStart') }}
            <span class="cc-gradient-text">{{ $t('landing.hero.titleHighlight') }}</span>
          </h1>
          <p class="max-w-[580px] text-[15px] leading-relaxed sm:text-[17px]" style="color: var(--text-muted);">
            {{ $t('landing.hero.subtitle') }}
          </p>
          <div class="mt-2 flex flex-wrap justify-center gap-3">
            <NuxtLink
              to="/register"
              class="rounded-full px-6 py-3 text-sm font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            >
              {{ $t('landing.hero.primaryCta') }}
            </NuxtLink>
            <NuxtLink
              to="/login"
              class="rounded-full border px-6 py-3 text-sm font-medium transition-colors hover:bg-[var(--card-bg-strong)]"
              style="border-color: var(--input-border); color: var(--text);"
            >
              {{ $t('landing.hero.secondaryCta') }}
            </NuxtLink>
          </div>
        </div>

        <figure class="relative w-full">
          <div
            aria-hidden="true"
            class="absolute inset-x-[10%] top-[8%] bottom-0 rounded-full blur-[70px]"
            style="background: radial-gradient(ellipse, rgba(139,92,246,0.45), rgba(139,92,246,0) 70%);"
          />
          <div
            class="relative overflow-hidden rounded-[var(--radius-lg)] border p-1.5 shadow-[0_30px_80px_rgba(0,0,0,0.45)] sm:rounded-[var(--radius-xl)] sm:p-2"
            style="border-color: var(--card-border); background: var(--card-bg-strong);"
          >
            <img
              src="/landing/dashboard.webp"
              width="1440"
              height="900"
              fetchpriority="high"
              :alt="$t('landing.hero.screenshotAlt')"
              class="block h-auto w-full rounded-[var(--radius-md)] sm:rounded-[20px]"
            >
          </div>
        </figure>
      </section>

      <!-- Pillars -->
      <section class="grid grid-cols-1 gap-4 sm:grid-cols-3" :aria-label="$t('landing.pillars.heading')">
        <div
          v-for="p in pillars"
          :key="p.title"
          class="rounded-[var(--radius-lg)] border p-6"
          style="border-color: var(--card-border); background: var(--card-bg);"
        >
          <p class="cc-gradient-text text-[22px] font-semibold">{{ p.title }}</p>
          <p class="mt-2 text-sm leading-relaxed" style="color: var(--text-muted);">{{ p.body }}</p>
        </div>
      </section>

      <!-- Features -->
      <section class="flex flex-col gap-10">
        <div class="mx-auto flex max-w-[620px] flex-col gap-3 text-center">
          <h2 class="text-[26px] font-semibold tracking-tight sm:text-[34px]">{{ $t('landing.features.heading') }}</h2>
          <p class="text-[15px]" style="color: var(--text-muted);">{{ $t('landing.features.subheading') }}</p>
        </div>
        <ul class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <li
            v-for="f in features"
            :key="f.title"
            class="flex flex-col gap-3 rounded-[var(--radius-lg)] border p-6 transition-all hover:-translate-y-1 hover:shadow-[0_14px_34px_rgba(129,140,248,0.18)]"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <span
              class="flex h-11 w-11 items-center justify-center rounded-[var(--radius-sm)]"
              style="background: linear-gradient(135deg, rgba(139,92,246,0.22), rgba(168,85,247,0.12)); color: var(--accent-link);"
            >
              <svg
                aria-hidden="true"
                viewBox="0 0 24 24"
                width="22"
                height="22"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path v-for="d in f.icon" :key="d" :d="d" />
              </svg>
            </span>
            <h3 class="text-[16px] font-semibold">{{ f.title }}</h3>
            <p class="text-sm leading-relaxed" style="color: var(--text-muted);">{{ f.body }}</p>
          </li>
        </ul>
      </section>

      <!-- Screenshots -->
      <section class="flex flex-col gap-16 sm:gap-20">
        <div
          v-for="(s, i) in showcase"
          :key="s.src"
          class="grid grid-cols-1 items-center gap-8 lg:gap-12"
          :class="i % 2 === 1 ? 'lg:grid-cols-[1.4fr_1fr]' : 'lg:grid-cols-[1fr_1.4fr]'"
        >
          <div class="flex flex-col gap-3" :class="i % 2 === 1 ? 'lg:order-2' : ''">
            <p class="text-[12px] font-semibold uppercase tracking-wide" style="color: var(--accent-link);">{{ s.eyebrow }}</p>
            <h2 class="text-[24px] font-semibold leading-tight tracking-tight sm:text-[30px]">{{ s.title }}</h2>
            <p class="text-[15px] leading-relaxed" style="color: var(--text-muted);">{{ s.body }}</p>
          </div>
          <div
            class="overflow-hidden rounded-[var(--radius-lg)] border p-1.5 shadow-[0_24px_60px_rgba(0,0,0,0.35)]"
            style="border-color: var(--card-border); background: var(--card-bg-strong);"
          >
            <img
              :src="s.src"
              :width="s.width"
              :height="s.height"
              loading="lazy"
              decoding="async"
              :alt="s.alt"
              class="block h-auto w-full rounded-[var(--radius-md)]"
            >
          </div>
        </div>
      </section>

      <!-- Closing CTA -->
      <section
        class="relative overflow-hidden rounded-[var(--radius-xl)] border px-6 py-14 text-center sm:px-12"
        style="border-color: var(--card-border); background: var(--card-bg-strong);"
      >
        <div
          aria-hidden="true"
          class="absolute left-1/2 top-0 h-[260px] w-[520px] -translate-x-1/2 -translate-y-1/2 rounded-full blur-[60px]"
          style="background: radial-gradient(ellipse, rgba(168,85,247,0.4), rgba(168,85,247,0) 70%);"
        />
        <div class="relative mx-auto flex max-w-[560px] flex-col items-center gap-4">
          <AppLogo size="lg" />
          <h2 class="mt-2 text-[26px] font-semibold tracking-tight sm:text-[32px]">{{ $t('landing.cta.title') }}</h2>
          <p class="text-[15px]" style="color: var(--text-muted);">{{ $t('landing.cta.body') }}</p>
          <NuxtLink
            to="/register"
            class="mt-3 rounded-full px-6 py-3 text-sm font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('landing.cta.button') }}
          </NuxtLink>
        </div>
      </section>
    </main>

    <footer class="relative z-[1] mx-auto flex max-w-[1080px] flex-col items-center justify-between gap-2 border-t px-4 py-8 text-xs sm:flex-row sm:px-6" style="border-color: var(--card-border); color: var(--text-dim);">
      <span>© {{ new Date().getFullYear() }} TapeandoCartones</span>
      <span>{{ $t('landing.footer.disclaimer') }}</span>
    </footer>
  </div>
</template>
