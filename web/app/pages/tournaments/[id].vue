<script setup lang="ts">
import type { TournamentDetail, TournamentTable } from '~/types/api'
import type { SeatResultInput } from '~/composables/useTournaments'

const route = useRoute()
const tournamentId = route.params.id as string
const { t } = useI18n()
const { user } = useAuth()
const { getTournament, addGuestParticipant, startTournament, recordTableResult, advanceRound } = useTournaments()
const { showToast } = useToast()

const { data: detail, refresh, error: loadError } = await useAsyncData<TournamentDetail | null>(
  `tournament-${tournamentId}`,
  () => getTournament(tournamentId),
  { default: () => null },
)

// GetTournament 404s for anyone who isn't the organizer or an already-joined
// participant (see internal/tournaments/service.go: isVisible), so a
// successfully loaded page never needs a "join this tournament" prompt.
const isOrganizer = computed(() => !!user.value && detail.value?.tournament.organizer_id === user.value.id)

const currentRound = computed(() => {
  const rounds = detail.value?.rounds
  return rounds && rounds.length ? rounds[rounds.length - 1] : null
})

const allTablesRecorded = computed(() =>
  !!currentRound.value
  && currentRound.value.tables.every((table) => table.seats.every((seat) => seat.finish_position !== null)),
)

// ----------------------------------------------------------- copy code
const codeCopied = ref(false)
async function copyJoinCode() {
  if (!detail.value) return
  await navigator.clipboard.writeText(detail.value.tournament.join_code)
  codeCopied.value = true
  setTimeout(() => { codeCopied.value = false }, 2000)
}

// -------------------------------------------------------------- guests
const isAddGuestOpen = ref(false)
const guestName = ref('')
const guestCommander = ref('')
const addGuestError = ref('')
const isAddingGuest = ref(false)

function toggleAddGuest() {
  isAddGuestOpen.value = !isAddGuestOpen.value
  guestName.value = ''
  guestCommander.value = ''
  addGuestError.value = ''
}

async function handleAddGuest() {
  addGuestError.value = ''
  isAddingGuest.value = true
  try {
    await addGuestParticipant(tournamentId, guestName.value, guestCommander.value)
    isAddGuestOpen.value = false
    await refresh()
    showToast(t('toast.guestAdded', { name: guestName.value }))
  } catch (err) {
    addGuestError.value = addGuestParticipantError(err)
  } finally {
    isAddingGuest.value = false
  }
}

// ---------------------------------------------------------------- start
const startError = ref('')
const isStarting = ref(false)

async function handleStart() {
  startError.value = ''
  isStarting.value = true
  try {
    await startTournament(tournamentId)
    await refresh()
    showToast(t('toast.tournamentStarted'))
  } catch (err) {
    startError.value = startTournamentError(err)
  } finally {
    isStarting.value = false
  }
}

// ---------------------------------------------------------- record result
const resultDrafts = reactive<Record<string, Record<string, number | null>>>({})

watch(currentRound, (round) => {
  if (!round) return
  for (const table of round.tables) {
    if (resultDrafts[table.id]) continue
    const draft: Record<string, number | null> = {}
    for (const seat of table.seats) draft[seat.participant_id] = null
    resultDrafts[table.id] = draft
  }
}, { immediate: true })

function draftFor(table: TournamentTable): Record<string, number | null> {
  return resultDrafts[table.id] ?? {}
}

const recordingTableId = ref<string | null>(null)
const recordErrors = reactive<Record<string, string>>({})

async function handleRecordResult(table: TournamentTable) {
  const draft = draftFor(table)
  const results: SeatResultInput[] = Object.entries(draft).map(([participantId, position]) => ({
    participant_id: participantId,
    finish_position: position ?? 0,
  }))
  recordErrors[table.id] = ''
  recordingTableId.value = table.id
  try {
    await recordTableResult(tournamentId, table.id, results)
    await refresh()
  } catch (err) {
    recordErrors[table.id] = recordTableResultError(err)
  } finally {
    recordingTableId.value = null
  }
}

// --------------------------------------------------------------- advance
const advanceError = ref('')
const isAdvancing = ref(false)

async function handleAdvance() {
  advanceError.value = ''
  isAdvancing.value = true
  try {
    await advanceRound(tournamentId)
    await refresh()
    showToast(t('toast.roundAdvanced'))
  } catch (err) {
    advanceError.value = advanceRoundError(err)
  } finally {
    isAdvancing.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <NuxtLink to="/tournaments" class="self-start text-[13px]" style="color: var(--accent-link);">
      {{ $t('tournaments.detail.back') }}
    </NuxtLink>

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ $t('tournaments.detail.loadError') }}</p>

    <template v-else-if="detail">
      <section>
        <h1 class="text-2xl font-semibold sm:text-[26px]">{{ detail.tournament.name }}</h1>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">
          {{ $t(`tournaments.status.${detail.tournament.status}`) }}
          <template v-if="detail.tournament.round_count">
            · {{ $t('tournaments.detail.roundProgress', {
              current: detail.tournament.current_round, total: detail.tournament.round_count,
            }) }}
          </template>
        </p>
      </section>

      <!-- Registration phase -->
      <template v-if="detail.tournament.status === 'registration'">
        <section class="rounded-[24px] border p-5" style="border-color: var(--card-border); background: var(--card-bg);">
          <p class="text-[13px]" style="color: var(--text-dim);">{{ $t('tournaments.detail.joinCodeLabel') }}</p>
          <div class="mt-1 flex items-center gap-3">
            <span class="text-2xl font-semibold tracking-[0.3em]">{{ detail.tournament.join_code }}</span>
            <button type="button" class="text-[13px]" style="color: var(--accent-link);" @click="copyJoinCode">
              {{ codeCopied ? $t('tournaments.detail.copied') : $t('tournaments.detail.copy') }}
            </button>
          </div>
          <p class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('tournaments.detail.shareHint') }}</p>
        </section>

        <section>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[15px] font-medium">
              {{ $t('tournaments.detail.participants.heading', detail.participants.length) }}
            </h2>
            <button
              v-if="isOrganizer"
              type="button"
              class="text-[13px]"
              style="color: var(--accent-link);"
              @click="toggleAddGuest"
            >
              {{ isAddGuestOpen ? $t('tournaments.detail.participants.close') : $t('tournaments.detail.participants.addGuest') }}
            </button>
          </div>

          <div
            v-if="isAddGuestOpen"
            class="mb-3.5 flex flex-col gap-2.5 rounded-[22px] border p-4"
            style="border-color: var(--card-border); background: var(--card-bg-strong);"
          >
            <form class="flex flex-wrap gap-2.5" @submit.prevent="handleAddGuest">
              <input
                v-model="guestName"
                type="text"
                required
                :placeholder="$t('tournaments.detail.participants.guestNamePlaceholder')"
                :aria-label="$t('tournaments.detail.participants.guestNamePlaceholder')"
                class="min-w-[160px] flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
                style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
              >
              <input
                v-model="guestCommander"
                type="text"
                required
                :placeholder="$t('tournaments.detail.participants.commanderPlaceholder')"
                :aria-label="$t('tournaments.detail.participants.commanderPlaceholder')"
                class="min-w-[160px] flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
                style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
              >
              <button
                type="submit"
                :disabled="isAddingGuest"
                class="rounded-full px-5 py-2 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
                style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
              >
                {{ isAddingGuest ? $t('tournaments.detail.participants.adding') : $t('tournaments.detail.participants.add') }}
              </button>
            </form>
            <p v-if="addGuestError" class="text-xs" style="color: var(--lose);">{{ addGuestError }}</p>
          </div>

          <ul v-if="detail.participants.length" class="flex flex-col gap-2">
            <li
              v-for="p in detail.participants"
              :key="p.id"
              class="flex items-center justify-between rounded-2xl border px-4 py-2.5 text-sm"
              style="border-color: var(--card-border); background: var(--card-bg);"
            >
              <span>{{ p.username ?? p.guest_name }}</span>
              <span style="color: var(--text-muted);">{{ p.commander_name }}</span>
            </li>
          </ul>
          <p v-else class="text-sm" style="color: var(--text-muted);">{{ $t('tournaments.detail.participants.empty') }}</p>
        </section>

        <section v-if="isOrganizer">
          <button
            type="button"
            :disabled="isStarting"
            class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            @click="handleStart"
          >
            {{ isStarting ? $t('tournaments.detail.starting') : $t('tournaments.detail.start') }}
          </button>
          <p v-if="startError" class="mt-2 text-sm" style="color: var(--lose);">{{ startError }}</p>
        </section>
      </template>

      <!-- In progress / finished -->
      <template v-else>
        <section v-if="currentRound && detail.tournament.status === 'in_progress'">
          <h2 class="mb-3.5 text-[15px] font-medium">
            {{ $t('tournaments.detail.round.heading', { number: currentRound.round_number }) }}
          </h2>
          <div class="flex flex-col gap-3.5">
            <div
              v-for="table in currentRound.tables"
              :key="table.id"
              class="rounded-[22px] border p-4"
              style="border-color: var(--card-border); background: var(--card-bg);"
            >
              <p class="text-[13px] font-medium" style="color: var(--text-dim);">
                {{ $t('tournaments.detail.round.table', { number: table.table_number }) }}
              </p>
              <ul class="mt-2 flex flex-col gap-1.5">
                <li v-for="seat in table.seats" :key="seat.id" class="flex items-center justify-between gap-3 text-sm">
                  <span>
                    {{ seat.username ?? seat.guest_name }}
                    <span style="color: var(--text-muted);">({{ seat.commander_name }})</span>
                  </span>
                  <span v-if="seat.finish_position" class="text-[13px] font-semibold" style="color: var(--accent-link);">
                    {{ $t('tournaments.detail.round.finishedPosition', { n: seat.finish_position }) }}
                  </span>
                  <select
                    v-else-if="isOrganizer"
                    v-model.number="draftFor(table)[seat.participant_id]"
                    class="rounded-full border px-3 py-1 text-[12px] outline-none"
                    style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
                  >
                    <option :value="null">{{ $t('tournaments.detail.round.pickPosition') }}</option>
                    <option v-for="n in table.seats.length" :key="n" :value="n">{{ n }}</option>
                  </select>
                </li>
              </ul>
              <button
                v-if="isOrganizer && table.seats.some((s) => !s.finish_position)"
                type="button"
                :disabled="recordingTableId === table.id"
                class="mt-3 rounded-full px-4 py-1.5 text-[12px] font-semibold text-[#0a0714] disabled:opacity-50"
                style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
                @click="handleRecordResult(table)"
              >
                {{ recordingTableId === table.id
                  ? $t('tournaments.detail.round.recording')
                  : $t('tournaments.detail.round.recordResult') }}
              </button>
              <p v-if="recordErrors[table.id]" class="mt-2 text-xs" style="color: var(--lose);">
                {{ recordErrors[table.id] }}
              </p>
            </div>
          </div>

          <div v-if="isOrganizer && allTablesRecorded" class="mt-4">
            <button
              type="button"
              :disabled="isAdvancing"
              class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
              @click="handleAdvance"
            >
              {{ isAdvancing ? $t('tournaments.detail.round.advancing') : $t('tournaments.detail.round.advance') }}
            </button>
            <p v-if="advanceError" class="mt-2 text-sm" style="color: var(--lose);">{{ advanceError }}</p>
          </div>
        </section>

        <section>
          <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('tournaments.detail.standings.heading') }}</h2>
          <div class="overflow-hidden rounded-3xl border" style="border-color: var(--card-border);">
            <div
              class="grid grid-cols-[32px_1fr_70px] gap-2 px-5 py-3 text-[11px] uppercase tracking-wide"
              style="background: var(--dim-bg); color: var(--text-dim);"
            >
              <span>#</span>
              <span>{{ $t('tournaments.detail.standings.columns.player') }}</span>
              <span>{{ $t('tournaments.detail.standings.columns.points') }}</span>
            </div>
            <div
              v-for="(p, i) in detail.participants"
              :key="p.id"
              class="grid grid-cols-[32px_1fr_70px] items-center gap-2 border-t px-5 py-3"
              style="border-color: var(--card-border);"
            >
              <span class="text-[13px]" style="color: var(--text-dim);">{{ i + 1 }}</span>
              <span class="text-sm">
                {{ p.username ?? p.guest_name }}
                <span
                  v-if="detail.tournament.status === 'finished' && i === 0"
                  class="ml-1 text-[11px] font-semibold uppercase"
                  style="color: var(--win);"
                >{{ $t('tournaments.detail.standings.winner') }}</span>
              </span>
              <span class="text-[13px] font-semibold" style="color: var(--accent-link);">{{ p.points }}</span>
            </div>
          </div>
        </section>
      </template>
    </template>
  </div>
</template>
