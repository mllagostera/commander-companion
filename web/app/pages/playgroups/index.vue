<script setup lang="ts">
import type { Playgroup } from '~/types/api'

const { t } = useI18n()
const { listPlaygroups, createPlaygroup } = usePlaygroups()
const { playgroupStats } = useStatistics()
const { showToast } = useToast()

interface PlaygroupWithCounts extends Playgroup {
  memberCount: number
  gamesPlayed: number
}

const { data: playgroups, refresh, error: listError } = await useAsyncData<PlaygroupWithCounts[]>(
  'playgroups',
  async () => {
    const list = await listPlaygroups()
    // games_played es best-effort: un grupo que nunca jugó puede devolver 404
    // (ver GetPlaygroupStats en internal/statistics/service.go).
    return Promise.all(
      list.map(async (playgroup) => ({
        ...playgroup,
        memberCount: playgroup.members?.length ?? 0,
        gamesPlayed: await playgroupStats(playgroup.id).then((s) => s.games_played).catch(() => 0),
      })),
    )
  },
  { default: () => [] },
)

const isCreateModalOpen = ref(false)
const newName = ref('')
const createError = ref('')
const isCreating = ref(false)

function openCreateModal() {
  newName.value = ''
  createError.value = ''
  isCreateModalOpen.value = true
}

function closeCreateModal() {
  isCreateModalOpen.value = false
}

async function handleCreate() {
  createError.value = ''
  isCreating.value = true
  try {
    const created = await createPlaygroup(newName.value)
    closeCreateModal()
    await refresh()
    showToast(t('toast.groupCreated'))
    await navigateTo(`/playgroups/${created.id}`)
  } catch (err) {
    createError.value = createPlaygroupError(err)
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('playgroups.list.title') }}</h1>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('playgroups.list.subtitle') }}</p>
      </div>
      <button
        type="button"
        class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
        style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        @click="openCreateModal"
      >
        {{ $t('playgroups.list.create') }}
      </button>
    </section>

    <p v-if="listError" class="text-sm" style="color: var(--lose);">{{ $t('playgroups.list.loadError') }}</p>
    <p v-else-if="!playgroups?.length" class="text-sm" style="color: var(--text-muted);">
      {{ $t('playgroups.list.empty') }}
    </p>

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <NuxtLink
        v-for="playgroup in playgroups"
        :key="playgroup.id"
        :to="`/playgroups/${playgroup.id}`"
        class="flex flex-col gap-3.5 rounded-[28px] border p-5 transition-all hover:-translate-y-1 hover:shadow-[0_14px_34px_rgba(129,140,248,0.18)]"
        style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
      >
        <div>
          <h3 class="text-base font-semibold">{{ playgroup.name }}</h3>
          <p class="mt-1 text-[13px]" style="color: var(--text-dim);">
            {{ $t('playgroups.list.gamesPlayedCount', playgroup.gamesPlayed) }}
            · {{ $t('playgroups.list.memberCount', playgroup.memberCount) }}
          </p>
        </div>
        <div class="flex">
          <span
            v-for="(member, i) in (playgroup.members ?? []).slice(0, 4)"
            :key="member.user_id"
            class="-ml-2 flex h-7 w-7 items-center justify-center rounded-full border-2 text-[11px] font-semibold text-[#0a0714] first:ml-0"
            :style="{ background: avatarColor(i), borderColor: 'var(--page-solid)' }"
          >
            {{ member.username[0]?.toUpperCase() }}
          </span>
        </div>
      </NuxtLink>
    </div>

    <div
      v-if="isCreateModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeCreateModal"
    >
      <div class="w-full max-w-sm rounded-[24px] border p-6" style="border-color: var(--card-border); background: var(--page-solid);">
        <h2 class="text-[15px] font-medium">{{ $t('playgroups.list.modal.title') }}</h2>

        <form class="mt-4 space-y-3" @submit.prevent="handleCreate">
          <input
            v-model="newName"
            type="text"
            required
            autofocus
            :placeholder="$t('playgroups.list.modal.placeholder')"
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
              {{ isCreating ? $t('playgroups.list.modal.submitting') : $t('playgroups.list.modal.submit') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
