<script setup lang="ts">
/**
 * Landing page of a playgroup invite link (see the "Invite link" section of
 * pages/playgroups/[id].vue and ADR-0023).
 *
 * Without a session, auth.global.ts sends the visitor through /login and
 * back here with the code intact. Like pages/friends/add/[id].vue, it does
 * NOT join on load: a link can be prefetched or opened by accident, so the
 * page shows which group it is and waits for a click.
 */
import type { PlaygroupInvitePreview } from '~/types/api'

const route = useRoute()
const { t } = useI18n()
const { previewInvite, acceptInvite } = usePlaygroups()
const { showToast } = useToast()

const code = computed(() => String(route.params.code ?? ''))

const { data: preview, error: previewError } = await useAsyncData<PlaygroupInvitePreview | null>(
  `playgroup-invite-${code.value}`,
  () => previewInvite(code.value),
  { default: () => null },
)

const isJoining = ref(false)
const joinError = ref('')

async function handleJoin() {
  joinError.value = ''
  isJoining.value = true
  try {
    const joined = await acceptInvite(code.value)
    showToast(t('toast.groupJoined', { name: joined.name }))
    await navigateTo(`/playgroups/${joined.id}`)
  } catch (err) {
    joinError.value = playgroupInviteError(err)
  } finally {
    isJoining.value = false
  }
}

useHead({ title: () => t('playgroups.join.title') })
</script>

<template>
  <div class="mx-auto flex max-w-md flex-col gap-6">
    <div>
      <NuxtLink to="/playgroups" class="-m-2 inline-block p-2 text-[13px]" style="color: var(--accent-link);">
        {{ $t('playgroups.detail.back') }}
      </NuxtLink>
      <h1 class="mt-2 text-2xl font-semibold sm:text-[26px]">{{ $t('playgroups.join.title') }}</h1>
    </div>

    <div
      class="flex flex-col gap-4 rounded-[var(--radius-xl)] border p-6"
      style="border-color: var(--card-border); background: var(--card-bg);"
    >
      <p v-if="previewError || !preview" class="text-sm" style="color: var(--lose);">
        {{ previewError ? playgroupInviteError(previewError) : $t('errors.playgroups.invite.notFound') }}
      </p>

      <template v-else>
        <div>
          <p class="text-lg font-semibold">{{ preview.name }}</p>
          <p class="text-sm" style="color: var(--text-muted);">
            {{ $t('playgroups.detail.memberCount', preview.member_count) }}
          </p>
        </div>

        <template v-if="preview.is_member">
          <p class="text-sm" style="color: var(--text);">{{ $t('playgroups.join.alreadyMember') }}</p>
          <NuxtLink
            :to="`/playgroups/${preview.playgroup_id}`"
            class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ $t('playgroups.join.goToGroup') }}
          </NuxtLink>
        </template>

        <template v-else>
          <p class="text-sm" style="color: var(--text-muted);">{{ $t('playgroups.join.body') }}</p>
          <button
            type="button"
            :disabled="isJoining"
            class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.02] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            @click="handleJoin"
          >
            {{ isJoining ? $t('playgroups.join.joining') : $t('playgroups.join.action') }}
          </button>
          <p v-if="joinError" class="text-sm" style="color: var(--lose);" role="alert">{{ joinError }}</p>
        </template>
      </template>
    </div>
  </div>
</template>
