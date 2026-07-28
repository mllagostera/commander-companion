<script setup lang="ts">
definePageMeta({ layout: false })

const { verifyEmail } = useAuth()
const route = useRoute()

const status = ref<'pending' | 'success' | 'error'>('pending')
const errorMessage = ref('')

onMounted(async () => {
  const token = route.query.token
  if (typeof token !== 'string' || !token) {
    status.value = 'error'
    errorMessage.value = 'El link de verificación no es válido.'
    return
  }

  try {
    await verifyEmail(token)
    status.value = 'success'
  } catch (err) {
    status.value = 'error'
    errorMessage.value = apiErrorMessage(err, 'No se pudo verificar el email.')
  }
})
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-slate-950 text-slate-100 p-6">
    <div class="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900/60 p-8 text-center">
      <template v-if="status === 'pending'">
        <h1 class="text-xl font-semibold">Verificando tu email…</h1>
      </template>

      <template v-else-if="status === 'success'">
        <h1 class="text-xl font-semibold">Email confirmado</h1>
        <p class="mt-4 text-sm text-slate-400">Ya podés iniciar sesión.</p>
        <NuxtLink
          to="/login"
          class="mt-6 block w-full rounded-lg bg-indigo-500 py-2 font-medium text-slate-950 hover:bg-indigo-400"
        >
          Ir a iniciar sesión
        </NuxtLink>
      </template>

      <template v-else>
        <h1 class="text-xl font-semibold">No se pudo verificar</h1>
        <p class="mt-4 text-sm text-red-400">{{ errorMessage }}</p>
        <p class="mt-2 text-sm text-slate-400">
          Podés pedir un link nuevo desde
          <NuxtLink to="/login" class="text-indigo-400 hover:text-indigo-300">
            iniciar sesión
          </NuxtLink>.
        </p>
      </template>
    </div>
  </main>
</template>
