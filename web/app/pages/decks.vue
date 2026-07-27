<script setup lang="ts">
import type { Deck } from '~/types/api'

const { listDecks, importFromMoxfield } = useDecks()

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
        class="mt-4 rounded-lg border border-emerald-800 bg-emerald-950/40 p-4"
      >
        <p class="text-sm text-emerald-400">Deck importado</p>
        <p class="mt-1 font-medium">{{ importedDeck.name }}</p>
        <p class="text-sm text-slate-400">Comandante: {{ importedDeck.commander }}</p>
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
          class="rounded-lg border border-slate-800 bg-slate-900/40 p-4"
        >
          <div class="flex flex-wrap items-baseline justify-between gap-2">
            <span class="font-medium">{{ deck.name }}</span>
            <span
              v-if="deck.moxfield_id"
              class="rounded bg-slate-800 px-2 py-0.5 text-xs text-slate-400"
            >
              Moxfield · {{ deck.moxfield_id }}
            </span>
          </div>
          <p class="mt-1 text-sm text-slate-400">Comandante: {{ deck.commander }}</p>
        </li>
      </ul>
    </section>
  </div>
</template>
