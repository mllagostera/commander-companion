<script setup lang="ts">
import type { Deck, Tournament, TournamentLookup } from '~/types/api'

const { t } = useI18n()
const { listTournamentsPage, createTournament, joinTournament, lookupByCode } = useTournaments()
const { listAllDecks } = useDecks()
const { showToast } = useToast()

const { data: tournaments, error: listError } = await useAsyncData<Tournament[]>(
  'tournaments',
  async () => (await listTournamentsPage()).items,
  { default: () => [] },
)

// --------------------------------------------------------------- create
const isCreateModalOpen = ref(false)
const newName = ref('')
const newTargetPlayers = ref<number | null>(null)
const createError = ref('')
const isCreating = ref(false)
const createModalRef = ref<HTMLElement | null>(null)

function openCreateModal() {
  newName.value = ''
  newTargetPlayers.value = null
  createError.value = ''
  isCreateModalOpen.value = true
}

function closeCreateModal() {
  isCreateModalOpen.value = false
}

useModalA11y(isCreateModalOpen, createModalRef, closeCreateModal)

async function handleCreate() {
  createError.value = ''
  isCreating.value = true
  try {
    const created = await createTournament(newName.value, newTargetPlayers.value ?? undefined)
    closeCreateModal()
    showToast(t('toast.tournamentCreated'))
    await navigateTo(`/tournaments/${created.id}`)
  } catch (err) {
    createError.value = createTournamentError(err)
  } finally {
    isCreating.value = false
  }
}

// ---------------------------------------------------------- join by code
const isJoinModalOpen = ref(false)
const joinCode = ref('')
const joinStep = ref<'code' | 'deck'>('code')
const joinPreview = ref<TournamentLookup | null>(null)
const joinLookupError = ref('')
const isLookingUp = ref(false)
const joinDecks = ref<Deck[]>([])
const selectedDeckId = ref('')
const joinError = ref('')
const isJoining = ref(false)
const joinModalRef = ref<HTMLElement | null>(null)

function openJoinModal() {
  joinCode.value = ''
  joinStep.value = 'code'
  joinPreview.value = null
  joinLookupError.value = ''
  joinError.value = ''
  selectedDeckId.value = ''
  isJoinModalOpen.value = true
}

function closeJoinModal() {
  isJoinModalOpen.value = false
}

useModalA11y(isJoinModalOpen, joinModalRef, closeJoinModal)

async function handleLookup() {
  joinLookupError.value = ''
  isLookingUp.value = true
  try {
    const preview = await lookupByCode(joinCode.value)
    if (preview.participant) {
      // Already registered: skip straight to the tournament instead of asking for a deck again.
      closeJoinModal()
      await navigateTo(`/tournaments/${preview.tournament.id}`)
      return
    }
    joinPreview.value = preview
    joinDecks.value = await listAllDecks()
    joinStep.value = 'deck'
  } catch (err) {
    joinLookupError.value = lookupTournamentError(err)
  } finally {
    isLookingUp.value = false
  }
}

async function handleJoin() {
  if (!joinPreview.value || !selectedDeckId.value) return
  joinError.value = ''
  isJoining.value = true
  try {
    await joinTournament(joinCode.value, selectedDeckId.value)
    const id = joinPreview.value.tournament.id
    closeJoinModal()
    showToast(t('toast.tournamentJoined'))
    await navigateTo(`/tournaments/${id}`)
  } catch (err) {
    joinError.value = joinTournamentError(err)
  } finally {
    isJoining.value = false
  }
}

function statusColor(status: Tournament['status']): string {
  if (status === 'finished') return 'var(--win)'
  if (status === 'in_progress') return '#fbbf24'
  return 'var(--text-muted)'
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('tournaments.list.title') }}</h1>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('tournaments.list.subtitle') }}</p>
      </div>
      <div class="flex flex-wrap gap-2.5">
        <button
          type="button"
          class="rounded-full border px-4 py-2.5 text-[13px] font-semibold"
          style="border-color: var(--input-border); color: var(--text);"
          @click="openJoinModal"
        >
          {{ $t('tournaments.list.joinWithCode') }}
        </button>
        <button
          type="button"
          class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          @click="openCreateModal"
        >
          {{ $t('tournaments.list.create') }}
        </button>
      </div>
    </section>

    <p v-if="listError" class="text-sm" style="color: var(--lose);">{{ $t('tournaments.list.loadError') }}</p>
    <p v-else-if="!tournaments?.length" class="text-sm" style="color: var(--text-muted);">
      {{ $t('tournaments.list.empty') }}
    </p>

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <NuxtLink
        v-for="tournament in tournaments"
        :key="tournament.id"
        :to="`/tournaments/${tournament.id}`"
        class="flex flex-col gap-2.5 rounded-[28px] border p-5 transition-all hover:-translate-y-1 hover:shadow-[0_14px_34px_rgba(129,140,248,0.18)]"
        style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
      >
        <div class="flex items-start justify-between gap-2">
          <h3 class="text-base font-semibold">{{ tournament.name }}</h3>
          <span class="text-[11px] font-semibold uppercase tracking-wide" :style="{ color: statusColor(tournament.status) }">
            {{ $t(`tournaments.status.${tournament.status}`) }}
          </span>
        </div>
        <p class="text-[13px]" style="color: var(--text-dim);">
          <template v-if="tournament.status === 'registration'">
            {{ $t('tournaments.list.joinCode', { code: tournament.join_code }) }}
          </template>
          <template v-else>
            {{ $t('tournaments.list.roundProgress', { current: tournament.current_round, total: tournament.round_count }) }}
          </template>
        </p>
      </NuxtLink>
    </div>

    <!-- Create tournament -->
    <div
      v-if="isCreateModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeCreateModal"
    >
      <div
        ref="createModalRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="tournaments-create-title"
        class="w-full max-w-sm rounded-[24px] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <h2 id="tournaments-create-title" class="text-[15px] font-medium">{{ $t('tournaments.list.createModal.title') }}</h2>

        <form class="mt-4 space-y-3" @submit.prevent="handleCreate">
          <input
            v-model="newName"
            type="text"
            required
            autofocus
            :placeholder="$t('tournaments.list.createModal.namePlaceholder')"
            :aria-label="$t('tournaments.list.createModal.namePlaceholder')"
            class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
          <input
            v-model.number="newTargetPlayers"
            type="number"
            min="3"
            :placeholder="$t('tournaments.list.createModal.targetPlayersPlaceholder')"
            :aria-label="$t('tournaments.list.createModal.targetPlayersPlaceholder')"
            class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >

          <p v-if="createError" class="text-sm" style="color: var(--lose);">{{ createError }}</p>

          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="rounded-full border px-4 py-2 text-sm"
              style="border-color: var(--input-border); color: var(--text);"
              @click="closeCreateModal"
            >
              {{ $t('common.cancel') }}
            </button>
            <button
              type="submit"
              :disabled="isCreating"
              class="rounded-full px-5 py-2 text-sm font-semibold text-[#0a0714] disabled:opacity-50"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            >
              {{ isCreating ? $t('tournaments.list.createModal.submitting') : $t('tournaments.list.createModal.submit') }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Join with code -->
    <div
      v-if="isJoinModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeJoinModal"
    >
      <div
        ref="joinModalRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="tournaments-join-title"
        class="w-full max-w-sm rounded-[24px] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <h2 id="tournaments-join-title" class="text-[15px] font-medium">{{ $t('tournaments.list.joinModal.title') }}</h2>

        <form v-if="joinStep === 'code'" class="mt-4 space-y-3" @submit.prevent="handleLookup">
          <input
            v-model="joinCode"
            type="text"
            required
            autofocus
            autocomplete="off"
            :placeholder="$t('tournaments.list.joinModal.codePlaceholder')"
            :aria-label="$t('tournaments.list.joinModal.codePlaceholder')"
            class="w-full rounded-full border px-4 py-2.5 text-center text-[13px] uppercase tracking-widest outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
          <p v-if="joinLookupError" class="text-sm" style="color: var(--lose);">{{ joinLookupError }}</p>
          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="rounded-full border px-4 py-2 text-sm"
              style="border-color: var(--input-border); color: var(--text);"
              @click="closeJoinModal"
            >
              {{ $t('common.cancel') }}
            </button>
            <button
              type="submit"
              :disabled="isLookingUp"
              class="rounded-full px-5 py-2 text-sm font-semibold text-[#0a0714] disabled:opacity-50"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            >
              {{ isLookingUp ? $t('tournaments.list.joinModal.looking') : $t('tournaments.list.joinModal.next') }}
            </button>
          </div>
        </form>

        <form v-else class="mt-4 space-y-3" @submit.prevent="handleJoin">
          <p class="text-sm" style="color: var(--text-muted);">
            {{ $t('tournaments.list.joinModal.foundTournament', { name: joinPreview?.tournament.name }) }}
          </p>
          <select
            v-model="selectedDeckId"
            required
            class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
            <option value="" disabled>{{ $t('tournaments.list.joinModal.pickDeck') }}</option>
            <option v-for="deck in joinDecks" :key="deck.id" :value="deck.id">
              {{ deck.name }} ({{ deck.commander }})
            </option>
          </select>
          <p v-if="!joinDecks.length" class="text-xs" style="color: var(--text-dim);">
            {{ $t('tournaments.list.joinModal.noDecks') }}
          </p>
          <p v-if="joinError" class="text-sm" style="color: var(--lose);">{{ joinError }}</p>
          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="rounded-full border px-4 py-2 text-sm"
              style="border-color: var(--input-border); color: var(--text);"
              @click="joinStep = 'code'"
            >
              {{ $t('common.back') }}
            </button>
            <button
              type="submit"
              :disabled="isJoining || !selectedDeckId"
              class="rounded-full px-5 py-2 text-sm font-semibold text-[#0a0714] disabled:opacity-50"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            >
              {{ isJoining ? $t('tournaments.list.joinModal.joining') : $t('tournaments.list.joinModal.submit') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
