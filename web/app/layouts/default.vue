<script setup lang="ts">
import type { Playgroup } from '~/types/api'

const { user, logout } = useAuth()
const { listPlaygroups, createPlaygroup } = usePlaygroups()

const links = [
  { to: '/', label: 'Inicio' },
  { to: '/decks', label: 'Decks' },
  { to: '/statistics', label: 'Estadísticas' },
]

// ------------------------------------------------------------- menú de usuario
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

// -------------------------------------------------------------- menú de grupos
const isGroupsMenuOpen = ref(false)
const groupsMenuRef = ref<HTMLElement | null>(null)
const playgroups = ref<Playgroup[]>([])
const groupsError = ref('')
const isLoadingGroups = ref(false)

const isCreatingGroupForm = ref(false)
const newGroupName = ref('')
const createGroupError = ref('')
const isCreatingGroup = ref(false)

async function toggleGroupsMenu() {
  if (isGroupsMenuOpen.value) {
    closeGroupsMenu()
    return
  }
  isGroupsMenuOpen.value = true
  isLoadingGroups.value = true
  groupsError.value = ''
  try {
    playgroups.value = await listPlaygroups()
  } catch {
    groupsError.value = 'No se pudieron cargar los grupos.'
  } finally {
    isLoadingGroups.value = false
  }
}

function closeGroupsMenu() {
  isGroupsMenuOpen.value = false
  isCreatingGroupForm.value = false
}

function showCreateGroupForm() {
  isCreatingGroupForm.value = true
  newGroupName.value = ''
  createGroupError.value = ''
}

async function handleCreateGroup() {
  createGroupError.value = ''
  isCreatingGroup.value = true
  try {
    const created = await createPlaygroup(newGroupName.value)
    closeGroupsMenu()
    await navigateTo(`/playgroups/${created.id}`)
  } catch (err) {
    createGroupError.value = createPlaygroupError(err)
  } finally {
    isCreatingGroup.value = false
  }
}

/** Cierra los menús al clickear afuera — mismo patrón simple que el resto de la web
 * (no hay @vueuse/core en las dependencias todavía como para usar onClickOutside). */
function handleDocumentClick(event: MouseEvent) {
  const target = event.target as Node
  if (userMenuRef.value && !userMenuRef.value.contains(target)) closeUserMenu()
  if (groupsMenuRef.value && !groupsMenuRef.value.contains(target)) closeGroupsMenu()
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => document.removeEventListener('click', handleDocumentClick))
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <header class="border-b border-slate-800">
      <div class="mx-auto flex max-w-4xl flex-wrap items-center gap-4 px-6 py-4">
        <NuxtLink to="/" class="font-semibold">Commander Companion</NuxtLink>

        <nav class="flex items-center gap-4 text-sm">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="text-slate-400 hover:text-slate-100"
            exact-active-class="text-indigo-400"
          >
            {{ link.label }}
          </NuxtLink>

          <div ref="groupsMenuRef" class="relative">
            <button
              class="flex items-center gap-1 text-slate-400 hover:text-slate-100"
              @click="toggleGroupsMenu"
            >
              Grupos
              <span class="text-xs text-slate-500">▾</span>
            </button>

            <div
              v-if="isGroupsMenuOpen"
              class="absolute left-0 mt-2 w-56 overflow-hidden rounded-lg border border-slate-800 bg-slate-900 shadow-lg"
            >
              <p v-if="isLoadingGroups" class="px-4 py-2 text-sm text-slate-500">Cargando…</p>
              <p v-else-if="groupsError" class="px-4 py-2 text-sm text-red-400">{{ groupsError }}</p>
              <p v-else-if="!playgroups.length" class="px-4 py-2 text-sm text-slate-500">
                Todavía no sos miembro de ningún grupo.
              </p>
              <NuxtLink
                v-for="playgroup in playgroups"
                :key="playgroup.id"
                :to="`/playgroups/${playgroup.id}`"
                class="block truncate px-4 py-2 text-sm text-slate-300 hover:bg-slate-800"
                @click="closeGroupsMenu"
              >
                {{ playgroup.name }}
              </NuxtLink>

              <NuxtLink
                v-if="playgroups.length"
                to="/playgroups"
                class="block border-t border-slate-800 px-4 py-2 text-sm text-slate-400 hover:bg-slate-800"
                @click="closeGroupsMenu"
              >
                Ver todos los grupos
              </NuxtLink>

              <div class="border-t border-slate-800">
                <form v-if="isCreatingGroupForm" class="p-3" @submit.prevent="handleCreateGroup">
                  <input
                    v-model="newGroupName"
                    type="text"
                    required
                    autofocus
                    placeholder="Nombre del grupo"
                    class="w-full rounded-lg border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none"
                  >
                  <p v-if="createGroupError" class="mt-2 text-xs text-red-400">{{ createGroupError }}</p>
                  <button
                    type="submit"
                    :disabled="isCreatingGroup"
                    class="mt-2 w-full rounded-lg bg-indigo-500 py-1.5 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
                  >
                    {{ isCreatingGroup ? 'Creando…' : 'Crear' }}
                  </button>
                </form>
                <button
                  v-else
                  type="button"
                  class="block w-full px-4 py-2 text-left text-sm text-indigo-400 hover:bg-slate-800"
                  @click="showCreateGroupForm"
                >
                  + Crear grupo
                </button>
              </div>
            </div>
          </div>
        </nav>

        <div ref="userMenuRef" class="relative ml-auto text-sm">
          <button
            class="flex items-center gap-2 rounded-lg border border-slate-700 px-3 py-1 text-slate-300 hover:bg-slate-800"
            @click="toggleUserMenu"
          >
            {{ user?.username ?? '…' }}
            <span class="text-xs text-slate-500">▾</span>
          </button>

          <div
            v-if="isUserMenuOpen"
            class="absolute right-0 mt-2 w-40 overflow-hidden rounded-lg border border-slate-800 bg-slate-900 shadow-lg"
          >
            <NuxtLink
              to="/settings"
              class="block px-4 py-2 text-slate-300 hover:bg-slate-800"
              @click="closeUserMenu"
            >
              Ajustes
            </NuxtLink>
            <button
              class="block w-full px-4 py-2 text-left text-slate-300 hover:bg-slate-800"
              @click="handleLogout"
            >
              Salir
            </button>
          </div>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-4xl px-6 py-8">
      <slot />
    </main>
  </div>
</template>
