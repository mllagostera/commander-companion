<script setup lang="ts">
const { user, fetchMe, logout } = useAuth()

await useAsyncData('me', async () => {
  const me = await fetchMe()
  if (!me) {
    await navigateTo('/login')
  }
  return me
})

async function handleLogout() {
  await logout()
  await navigateTo('/login')
}
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-slate-950 text-slate-100 p-6">
    <div class="w-full max-w-md rounded-xl border border-slate-800 bg-slate-900/60 p-8 text-center">
      <p class="text-sm text-slate-400">Sesión iniciada como</p>
      <h1 class="mt-1 text-2xl font-semibold">{{ user?.username ?? '…' }}</h1>
      <p class="mt-1 text-sm text-slate-400">{{ user?.email }}</p>

      <button
        class="mt-6 w-full rounded-lg border border-slate-700 py-2 font-medium hover:bg-slate-800"
        @click="handleLogout"
      >
        Cerrar sesión
      </button>
    </div>
  </main>
</template>
