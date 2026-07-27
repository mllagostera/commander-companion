<script setup lang="ts">
import type { Deck } from '~/types/api'

const { listDecks, importFromMoxfield, syncFromMoxfield } = useDecks()

const { data: decks, refresh, error: listError } = await useAsyncData<Deck[]>(
  'decks',
  () => listDecks(),
  { default: () => [] },
)

const moxfieldInput = ref('')
const importError = ref('')
const importedDeck = ref<Deck | null>(null)
const isImporting = ref(false)

async function handleImport() {
  importError.value = ''
  importedDeck.value = null
  isImporting.value = true
  try {
    importedDeck.value = await importFromMoxfield(moxfieldInput.value)
    moxfieldInput.value = ''
    await refresh()
  } catch (err) {
    importError.value = moxfieldImportError(err)
  } finally {
    isImporting.value = false
  }
}

// Estado de sincronización por deck (id → mensaje/loading/error), independiente
// entre filas: sincronizar un deck no debe bloquear ni pisar el estado de otro.
const syncState = reactive<Record<string, { loading: boolean, message: string, isError: boolean }>>({})

async function handleSync(deck: Deck) {
  if (!deck.moxfield_id) return
  syncState[deck.id] = { loading: true, message: '', isError: false }
  try {
    const res = await syncFromMoxfield(deck.moxfield_id)
    const idx = decks.value?.findIndex(d => d.id === deck.id) ?? -1
    if (idx !== -1 && decks.value) decks.value[idx] = res.deck
    syncState[deck.id] = {
      loading: false,
      isError: false,
      message: res.status === 'updated' ? 'Actualizado desde Moxfield' : 'Ya estaba al día',
    }
  } catch (err) {
    syncState[deck.id] = {
      loading: false,
      isError: true,
      message: apiErrorMessage(err, 'No se pudo sincronizar con Moxfield.'),
    }
  }
}
</script>

<template>
  <div class="space-y-8">
    <section>
      <h1 class="text-2xl font-semibold">Mis decks</h1>
      <p class="mt-1 text-sm text-slate-400">
        Importá un deck de Moxfield pegando su URL o su ID público.
      </p>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <h2 class="font-medium">Importar desde Moxfield</h2>

      <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="handleImport">
        <input
          id="moxfield-url"
          v-model="moxfieldInput"
          type="text"
          required
          placeholder="https://moxfield.com/decks/abc123 o abc123"
          class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
        >
        <button
          type="submit"
          :disabled="isImporting"
          class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
        >
          {{ isImporting ? 'Importando…' : 'Importar' }}
        </button>
      </form>

      <p v-if="importError" class="mt-3 text-sm text-red-400">{{ importError }}</p>

      <div
        v-if="importedDeck"
        class="mt-4 flex gap-4 rounded-lg border border-emerald-800 bg-emerald-950/40 p-4"
      >
        <img
          v-if="importedDeck.image_url"
          :src="importedDeck.image_url"
          :alt="importedDeck.commander"
          class="h-16 w-16 shrink-0 rounded-lg object-cover"
        >
        <div>
          <p class="text-sm text-emerald-400">Deck importado</p>
          <p class="mt-1 font-medium">{{ importedDeck.name }}</p>
          <p class="text-sm text-slate-400">Comandante: {{ importedDeck.commander }}</p>
        </div>
      </div>
    </section>

    <section>
      <h2 class="font-medium">Decks guardados</h2>

      <p v-if="listError" class="mt-3 text-sm text-red-400">
        No se pudieron cargar los decks.
      </p>

      <p v-else-if="!decks?.length" class="mt-3 text-sm text-slate-400">
        Todavía no tenés decks. Importá uno desde Moxfield para empezar.
      </p>

      <ul v-else class="mt-3 space-y-2">
        <li
          v-for="deck in decks"
          :key="deck.id"
          class="flex gap-4 rounded-lg border border-slate-800 bg-slate-900/40 p-4"
        >
          <img
            v-if="deck.image_url"
            :src="deck.image_url"
            :alt="deck.commander"
            class="h-16 w-16 shrink-0 rounded-lg object-cover"
          >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <span class="font-medium">{{ deck.name }}</span>
              <div v-if="deck.moxfield_id" class="flex items-center gap-2">
                <span class="rounded bg-slate-800 px-2 py-0.5 text-xs text-slate-400">
                  Moxfield · {{ deck.moxfield_id }}
                </span>
                <button
                  type="button"
                  :disabled="syncState[deck.id]?.loading"
                  class="rounded border border-slate-700 px-2 py-0.5 text-xs text-slate-300 hover:bg-slate-800 disabled:opacity-50"
                  @click="handleSync(deck)"
                >
                  {{ syncState[deck.id]?.loading ? 'Sincronizando…' : 'Actualizar' }}
                </button>
              </div>
            </div>
            <p class="mt-1 text-sm text-slate-400">Comandante: {{ deck.commander }}</p>
            <p
              v-if="syncState[deck.id]?.message"
              class="mt-1 text-xs"
              :class="syncState[deck.id]?.isError ? 'text-red-400' : 'text-emerald-400'"
            >
              {{ syncState[deck.id]?.message }}
            </p>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>
