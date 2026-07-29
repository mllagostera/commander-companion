<script setup lang="ts">
const { user, logout } = useAuth()
const { theme, toggleTheme } = useTheme()

const links = [
  { to: '/', label: 'Inicio' },
  { to: '/decks', label: 'Decks' },
  { to: '/statistics', label: 'Estadísticas' },
  { to: '/playgroups', label: 'Grupos' },
]

const route = useRoute()
function isActive(to: string) {
  return to === '/' ? route.path === '/' : route.path.startsWith(to)
}

const userInitial = computed(() => user.value?.username?.[0]?.toUpperCase() ?? '?')

const isUserMenuOpen = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)

function toggleUserMenu() {
  isUserMenuOpen.value = !isUserMenuOpen.value
}

function closeUserMenu() {
  isUserMenuOpen.value = false
}

async function handleLogout() {
  closeUserMenu()
  await logout()
  await navigateTo('/login')
}

function handleDocumentClick(event: MouseEvent) {
  const target = event.target as Node
  if (userMenuRef.value && !userMenuRef.value.contains(target)) closeUserMenu()
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => document.removeEventListener('click', handleDocumentClick))
</script>

<template>
  <div class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />
    <div class="cc-blob top-[40%] left-1/2 h-[380px] w-[380px] rounded-full" style="background: radial-gradient(circle, rgba(196,181,253,0.14), rgba(196,181,253,0) 70%);" />

    <header class="sticky top-0 z-10 px-4 pt-4 sm:px-6 sm:pt-[18px]">
      <div
        class="mx-auto flex max-w-[1080px] flex-wrap items-center gap-4 rounded-[26px] border px-4 py-3 shadow-[0_8px_30px_rgba(0,0,0,0.2)] backdrop-blur-xl sm:gap-7 sm:rounded-full sm:px-[22px]"
        style="background: var(--header-bg); border-color: var(--card-border);"
      >
        <NuxtLink to="/" class="flex items-center gap-2.5">
          <AppLogo />
          <span class="cc-gradient-text text-[15px] font-semibold tracking-wide">Commander Companion</span>
        </NuxtLink>

        <nav class="flex flex-wrap gap-2.5 text-[13px] sm:gap-5 sm:text-sm">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="transition-colors"
            :style="{ color: isActive(link.to) ? 'var(--accent-link)' : 'var(--text-muted)' }"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>

        <div ref="userMenuRef" class="relative ml-auto">
          <button
            class="flex items-center gap-2 rounded-full py-[5px] pl-[5px] pr-2.5 transition-colors hover:bg-[var(--card-bg-strong)]"
            @click="toggleUserMenu"
          >
            <span
              class="flex h-[30px] w-[30px] flex-shrink-0 items-center justify-center rounded-full text-xs font-bold text-[#0a0714]"
              style="background: linear-gradient(135deg, #8b5cf6, #a855f7);"
            >{{ userInitial }}</span>
            <span class="hidden text-sm sm:inline" style="color: var(--text);">{{ user?.username ?? '…' }}</span>
            <span
              class="inline-block text-[10px] transition-transform"
              :style="{ color: 'var(--text-muted)', transform: isUserMenuOpen ? 'rotate(180deg)' : 'rotate(0deg)' }"
            >▾</span>
          </button>

          <template v-if="isUserMenuOpen">
            <div class="fixed inset-0 z-[29]" @click="closeUserMenu" />
            <div
              class="absolute right-0 top-[calc(100%+10px)] z-30 flex min-w-[210px] flex-col gap-0.5 rounded-[18px] border p-2.5 shadow-[0_16px_36px_rgba(0,0,0,0.3)]"
              style="background: var(--card-bg-strong); border-color: var(--card-border);"
            >
              <div class="flex items-center justify-between px-2.5 py-2">
                <span class="text-[13px]" style="color: var(--text);">Tema oscuro</span>
                <button
                  type="button"
                  title="Cambiar tema"
                  class="relative h-[22px] w-10 flex-shrink-0 rounded-full border p-0"
                  style="background: var(--input-bg); border-color: var(--input-border);"
                  @click="toggleTheme"
                >
                  <span
                    class="absolute top-0.5 h-4 w-4 rounded-full transition-[left]"
                    :style="{ left: theme === 'dark' ? '2px' : '22px', background: 'linear-gradient(135deg, #8b5cf6, #a855f7)' }"
                  />
                </button>
              </div>
              <NuxtLink
                to="/settings"
                class="rounded-[10px] px-2.5 py-[9px] text-left text-[13px] transition-colors hover:bg-[var(--card-bg)]"
                style="color: var(--text);"
                @click="closeUserMenu"
              >
                Configuración
              </NuxtLink>
              <button
                class="rounded-[10px] px-2.5 py-[9px] text-left text-[13px] transition-colors hover:bg-[var(--card-bg)]"
                style="color: var(--lose);"
                @click="handleLogout"
              >
                Salir
              </button>
            </div>
          </template>
        </div>
      </div>
    </header>

    <main class="relative z-[1] mx-auto max-w-[1080px] px-4 pb-[100px] pt-9 sm:px-6">
      <slot />
    </main>
  </div>
</template>
