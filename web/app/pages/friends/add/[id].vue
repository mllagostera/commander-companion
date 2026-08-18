<script setup lang="ts">
/**
 * Landing page for a scanned profile QR (see settings.vue, which encodes a
 * link here rather than a bare id).
 *
 * On Android with the app installed this URL is claimed by the App Link and
 * never reaches the browser. This page is what happens otherwise: on a
 * desktop browser, on iOS, or on an Android phone without the app — the
 * scan still has to lead somewhere that adds the friend.
 *
 * It deliberately does NOT send the request on load. A URL can be visited by
 * accident, prefetched, or opened from someone else's screenshot, and a
 * side effect that fires on navigation would make all three send a request.
 * The page identifies who was scanned and waits for a click.
 */
const route = useRoute()
const { t } = useI18n()
const { sendFriendRequest, listFriends } = useFriends()
const { user } = useAuth()

const targetId = computed(() => String(route.params.id ?? ''))

// The id in a URL is whatever the scanner read: validate the shape before
// spending a request the backend would reject anyway.
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const isMalformed = computed(() => !UUID_RE.test(targetId.value))
const isSelf = computed(() => !!user.value?.id && user.value.id === targetId.value)

// There's no public "who is this id" endpoint by design (it would let anyone
// enumerate usernames from guessed ids), so the page can't greet the target
// by name before the request. It can at least tell the user they're already
// friends instead of sending a request that would 409.
const { data: friends } = await useAsyncData('friends-for-add', () => listFriends(), { default: () => [] })
const existingFriend = computed(() => (friends.value ?? []).find((f) => f.id === targetId.value) ?? null)

const isSending = ref(false)
const sendError = ref('')
const result = ref<'sent' | 'accepted' | null>(null)

async function handleSend() {
  sendError.value = ''
  isSending.value = true
  try {
    const request = await sendFriendRequest(targetId.value)
    // The backend auto-accepts when the other user had already sent a
    // request the other way, so a 2xx here can mean either thing.
    result.value = request.status === 'accepted' ? 'accepted' : 'sent'
  } catch (err) {
    sendError.value = sendFriendRequestError(err)
  } finally {
    isSending.value = false
  }
}

useHead({ title: () => t('friends.add.title') })
</script>

<template>
  <div class="mx-auto flex max-w-md flex-col gap-6">
    <div>
      <NuxtLink to="/friends" class="-m-2 inline-block p-2 text-[13px]" style="color: var(--accent-link);">
        {{ $t('friends.add.back') }}
      </NuxtLink>
      <h1 class="mt-2 text-2xl font-semibold sm:text-[26px]">{{ $t('friends.add.title') }}</h1>
    </div>

    <div
      class="flex flex-col gap-4 rounded-[var(--radius-xl)] border p-6"
      style="border-color: var(--card-border); background: var(--card-bg);"
    >
      <p v-if="isMalformed" class="text-sm" style="color: var(--lose);">
        {{ $t('friends.add.malformed') }}
      </p>

      <p v-else-if="isSelf" class="text-sm" style="color: var(--text-muted);">
        {{ $t('friends.add.self') }}
      </p>

      <template v-else-if="existingFriend">
        <p class="text-sm" style="color: var(--text);">
          {{ $t('friends.add.alreadyFriends', { username: existingFriend.username }) }}
        </p>
        <NuxtLink
          to="/friends"
          class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ $t('friends.add.goToFriends') }}
        </NuxtLink>
      </template>

      <template v-else-if="result">
        <p class="text-sm" style="color: var(--win);">
          {{ result === 'accepted' ? $t('friends.add.nowFriends') : $t('friends.add.sent') }}
        </p>
        <NuxtLink
          to="/friends"
          class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        >
          {{ $t('friends.add.goToFriends') }}
        </NuxtLink>
      </template>

      <template v-else>
        <p class="text-sm" style="color: var(--text-muted);">{{ $t('friends.add.body') }}</p>
        <button
          type="button"
          :disabled="isSending"
          class="self-start rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.02] disabled:opacity-50"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          @click="handleSend"
        >
          {{ isSending ? $t('friends.add.sending') : $t('friends.add.action') }}
        </button>
        <p v-if="sendError" class="text-sm" style="color: var(--lose);">{{ sendError }}</p>
      </template>
    </div>
  </div>
</template>
