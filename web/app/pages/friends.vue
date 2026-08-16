<script setup lang="ts">
import type { Friend, UserSearchResult } from '~/types/api'

const { t, d } = useI18n()
const {
  listIncomingRequests,
  listOutgoingRequests,
  listFriends,
  sendFriendRequest,
  acceptFriendRequest,
  rejectFriendRequest,
  cancelFriendRequest,
  removeFriend,
} = useFriends()
const { searchUsers } = useUsers()
const { showToast } = useToast()

const { data: incoming, refresh: refreshIncoming } = await useAsyncData(
  'friends-incoming',
  () => listIncomingRequests(),
  { default: () => [] },
)
const { data: outgoing, refresh: refreshOutgoing } = await useAsyncData(
  'friends-outgoing',
  () => listOutgoingRequests(),
  { default: () => [] },
)
const { data: friends, refresh: refreshFriends, error: friendsError } = await useAsyncData(
  'friends-list',
  () => listFriends(),
  { default: () => [] },
)

function friendsSinceLabel(iso: string): string {
  return d(new Date(iso), 'short')
}

// ------------------------------------------------------- add friend
const query = ref('')
const queryRef = ref<HTMLInputElement | null>(null)
const searchResults = ref<UserSearchResult[]>([])
const searchResultRefs = ref<HTMLButtonElement[]>([])
const isSearching = ref(false)
const searchError = ref('')
const sendError = ref('')
const sendingUserId = ref<string | null>(null)
let searchDebounce: ReturnType<typeof setTimeout> | undefined

// A user already a friend, or with a pending request in either direction,
// doesn't need to show up again in the picker.
const excludedUserIds = computed(() => {
  const ids = new Set<string>()
  for (const f of friends.value ?? []) ids.add(f.id)
  for (const r of outgoing.value ?? []) ids.add(r.addressee_id)
  for (const r of incoming.value ?? []) ids.add(r.requester_id)
  return ids
})

function onQueryInput() {
  searchError.value = ''
  clearTimeout(searchDebounce)
  const q = query.value.trim()
  if (q.length < 2) {
    searchResults.value = []
    return
  }
  searchDebounce = setTimeout(async () => {
    isSearching.value = true
    try {
      const results = await searchUsers(q)
      searchResults.value = results.filter((u) => !excludedUserIds.value.has(u.id))
    } catch (err) {
      searchError.value = searchUsersError(err)
      searchResults.value = []
    } finally {
      isSearching.value = false
    }
  }, 300)
}

function focusResult(index: number) {
  const count = searchResultRefs.value.length
  if (!count) return
  searchResultRefs.value[(index + count) % count]?.focus()
}

function handleQueryKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' && searchResults.value.length) {
    event.preventDefault()
    focusResult(0)
  }
}

function handleResultKeydown(event: KeyboardEvent) {
  const currentIndex = searchResultRefs.value.findIndex((el) => el === document.activeElement)
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    focusResult(currentIndex + 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (currentIndex <= 0) {
      queryRef.value?.focus()
    } else {
      focusResult(currentIndex - 1)
    }
  } else if (event.key === 'Escape') {
    event.preventDefault()
    searchResults.value = []
    queryRef.value?.focus()
  }
}

async function handleSendRequest(user: UserSearchResult) {
  sendError.value = ''
  sendingUserId.value = user.id
  try {
    const res = await sendFriendRequest(user.id)
    query.value = ''
    searchResults.value = []
    queryRef.value?.focus()
    if (res.status === 'accepted') {
      await refreshFriends()
      showToast(t('toast.friendAdded', { username: res.addressee_username }))
    } else {
      await refreshOutgoing()
      showToast(t('toast.friendRequestSent', { username: res.addressee_username }))
    }
  } catch (err) {
    sendError.value = sendFriendRequestError(err)
  } finally {
    sendingUserId.value = null
  }
}

// ------------------------------------------------------- respond to requests
const respondingId = ref<string | null>(null)
const respondError = ref('')

async function handleAccept(requestId: string, username: string) {
  respondError.value = ''
  respondingId.value = requestId
  try {
    await acceptFriendRequest(requestId)
    await Promise.all([refreshIncoming(), refreshFriends()])
    showToast(t('toast.friendAdded', { username }))
  } catch (err) {
    respondError.value = respondFriendRequestError(err)
  } finally {
    respondingId.value = null
  }
}

async function handleReject(requestId: string) {
  respondError.value = ''
  respondingId.value = requestId
  try {
    await rejectFriendRequest(requestId)
    await refreshIncoming()
  } catch (err) {
    respondError.value = respondFriendRequestError(err)
  } finally {
    respondingId.value = null
  }
}

async function handleCancel(requestId: string) {
  respondError.value = ''
  respondingId.value = requestId
  try {
    await cancelFriendRequest(requestId)
    await refreshOutgoing()
  } catch (err) {
    respondError.value = respondFriendRequestError(err)
  } finally {
    respondingId.value = null
  }
}

// Unfriending is irreversible (the row is deleted, not soft-closed) and the
// button sits right next to the friend's name, so it asks first.
const friendPendingRemoval = ref<Friend | null>(null)
const removeDialogRef = ref<HTMLElement | null>(null)
const isRemoveDialogOpen = computed(() => friendPendingRemoval.value !== null)

function askRemove(friend: Friend) {
  respondError.value = ''
  friendPendingRemoval.value = friend
}

function cancelRemove() {
  friendPendingRemoval.value = null
}

useModalA11y(isRemoveDialogOpen, removeDialogRef, cancelRemove)

async function confirmRemove() {
  const friend = friendPendingRemoval.value
  if (!friend) return
  respondError.value = ''
  respondingId.value = friend.id
  try {
    await removeFriend(friend.id)
    friendPendingRemoval.value = null
    await refreshFriends()
  } catch (err) {
    respondError.value = respondFriendRequestError(err)
  } finally {
    respondingId.value = null
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('friends.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('friends.subtitle') }}</p>
    </section>

    <section class="flex flex-col gap-2.5 rounded-[var(--radius-lg)] border p-4" style="border-color: var(--card-border); background: var(--card-bg-strong);">
      <h2 class="text-[15px] font-medium">{{ $t('friends.add.heading') }}</h2>
      <div class="relative">
        <input
          ref="queryRef"
          v-model="query"
          type="text"
          autocomplete="off"
          role="combobox"
          aria-autocomplete="list"
          :aria-expanded="searchResults.length > 0"
          aria-controls="friend-search-listbox"
          :placeholder="$t('friends.add.searchPlaceholder')"
          :aria-label="$t('friends.add.searchPlaceholder')"
          class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
          style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          @input="onQueryInput"
          @keydown="handleQueryKeydown"
        >
        <ul
          v-if="searchResults.length"
          id="friend-search-listbox"
          role="listbox"
          class="absolute z-10 mt-1 w-full space-y-1 rounded-2xl border p-1 shadow-lg"
          style="border-color: var(--card-border); background: var(--page-solid);"
        >
          <li v-for="user in searchResults" :key="user.id" role="presentation">
            <button
              ref="searchResultRefs"
              type="button"
              role="option"
              aria-selected="false"
              :disabled="sendingUserId === user.id"
              class="flex w-full items-center justify-between rounded-xl px-3 py-2 text-left text-sm hover:bg-white/5 disabled:opacity-50"
              style="color: var(--text);"
              @click="handleSendRequest(user)"
              @keydown="handleResultKeydown"
            >
              <span>{{ user.username }}</span>
              <span class="text-[11px]" style="color: var(--accent-link);">{{ $t('friends.add.send') }}</span>
            </button>
          </li>
        </ul>
      </div>
      <p v-if="isSearching" class="text-xs" style="color: var(--text-dim);">{{ $t('friends.add.searching') }}</p>
      <p v-if="searchError" class="text-xs" style="color: var(--lose);">{{ searchError }}</p>
      <p v-if="sendError" class="text-xs" style="color: var(--lose);">{{ sendError }}</p>
    </section>

    <!-- Both request lists sit in one two-column row: each card carries a name
         and two small actions, so full-width rows left ~800px of dead space
         between the two. -->
    <section v-if="incoming?.length || outgoing?.length" class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div v-if="incoming?.length">
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('friends.incoming.heading') }}</h2>
        <div class="flex flex-col gap-2.5">
          <div
            v-for="req in incoming"
            :key="req.id"
            class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-md)] border px-4 py-3"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <span class="flex min-w-0 items-center gap-2.5">
              <UserAvatar :username="req.requester_username" />
              <span class="truncate text-sm">{{ req.requester_username }}</span>
            </span>
            <div class="flex gap-2">
              <button
                type="button"
                :disabled="respondingId === req.id"
                class="rounded-full px-4 py-1.5 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
                style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
                @click="handleAccept(req.id, req.requester_username)"
              >
                {{ $t('friends.incoming.accept') }}
              </button>
              <button
                type="button"
                :disabled="respondingId === req.id"
                class="rounded-full border px-4 py-1.5 text-[13px] disabled:opacity-50"
                style="border-color: var(--input-border); color: var(--text-muted);"
                @click="handleReject(req.id)"
              >
                {{ $t('friends.incoming.reject') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="outgoing?.length">
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('friends.outgoing.heading') }}</h2>
        <div class="flex flex-col gap-2.5">
          <div
            v-for="req in outgoing"
            :key="req.id"
            class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-md)] border px-4 py-3"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <span class="flex min-w-0 items-center gap-2.5">
              <UserAvatar :username="req.addressee_username" />
              <span class="truncate text-sm">{{ req.addressee_username }}</span>
            </span>
            <span class="flex items-center gap-3">
              <span class="text-[13px]" style="color: var(--text-dim);">{{ $t('friends.outgoing.pending') }}</span>
              <button
                type="button"
                :disabled="respondingId === req.id"
                class="rounded-full border px-4 py-1.5 text-[13px] disabled:opacity-50"
                style="border-color: var(--input-border); color: var(--text-muted);"
                @click="handleCancel(req.id)"
              >
                {{ $t('friends.outgoing.cancel') }}
              </button>
            </span>
          </div>
        </div>
      </div>
    </section>

    <p v-if="respondError" class="text-sm" style="color: var(--lose);">{{ respondError }}</p>

    <section>
      <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('friends.list.heading') }}</h2>

      <p v-if="friendsError" class="text-sm" style="color: var(--lose);">{{ listFriendsError(friendsError) }}</p>
      <EmptyState
        v-else-if="!friends?.length"
        :title="$t('friends.list.emptyTitle')"
        :body="$t('friends.list.emptyBody')"
      />

      <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div
          v-for="friend in friends"
          :key="friend.id"
          class="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border px-4 py-3"
          style="border-color: var(--card-border); background: var(--card-bg);"
        >
          <span class="flex min-w-0 items-center gap-2.5">
            <UserAvatar :username="friend.username" />
            <span class="min-w-0">
              <span class="block truncate text-sm">{{ friend.username }}</span>
              <span class="block text-[11px]" style="color: var(--text-dim);">
                {{ $t('friends.list.since', { date: friendsSinceLabel(friend.friends_since) }) }}
              </span>
            </span>
          </span>
          <button
            type="button"
            :disabled="respondingId === friend.id"
            class="flex-shrink-0 rounded-full border px-3 py-1 text-[13px] disabled:opacity-50"
            style="border-color: var(--input-border); color: var(--text-muted);"
            @click="askRemove(friend)"
          >
            {{ $t('friends.list.remove') }}
          </button>
        </div>
      </div>
    </section>

    <div
      v-if="friendPendingRemoval"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="cancelRemove"
    >
      <div
        ref="removeDialogRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="friends-remove-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <h2 id="friends-remove-title" class="text-[15px] font-medium">
          {{ $t('friends.list.removeConfirmTitle', { username: friendPendingRemoval.username }) }}
        </h2>
        <p class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('friends.list.removeConfirmBody') }}</p>

        <div class="mt-5 flex justify-end gap-3">
          <button
            type="button"
            class="rounded-full border px-4 py-2 text-sm"
            style="border-color: var(--input-border); color: var(--text);"
            @click="cancelRemove"
          >
            {{ $t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="respondingId === friendPendingRemoval.id"
            class="rounded-full border px-5 py-2 text-sm font-semibold disabled:opacity-50"
            style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
            @click="confirmRemove"
          >
            {{ $t('friends.list.remove') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
