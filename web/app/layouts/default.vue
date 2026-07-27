<script setup lang="ts">
const { user, logout } = useAuth()

const links = [
  { to: '/', label: 'Inicio' },
  { to: '/decks', label: 'Decks' },
  { to: '/statistics', label: 'Estadísticas' },
]

async function handleLogout() {
  await logout()
  await navigateTo('/login')
}
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <header class="border-b border-slate-800">
      <div class="mx-auto flex max-w-4xl flex-wrap items-center gap-4 px-6 py-4">
        <NuxtLink to="/" class="font-semibold">Commander Companion</NuxtLink>

        <nav class="flex gap-4 text-sm">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="text-slate-400 hover:text-slate-100"
            exact-active-class="text-indigo-400"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>

        <div class="ml-auto flex items-center gap-3 text-sm">
          <span class="text-slate-400">{{ user?.username ?? '…' }}</span>
          <button
            class="rounded-lg border border-slate-700 px-3 py-1 hover:bg-slate-800"
            @click="handleLogout"
          >
            Salir
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-4xl px-6 py-8">
      <slot />
    </main>
  </div>
</template>
