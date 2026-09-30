<script setup lang="ts">
/**
 * `/` serves two audiences: the landing page for visitors without a session,
 * and the dashboard for everyone else. Sharing the root URL (rather than a
 * separate /welcome) means the domain itself is what gets shared and indexed.
 *
 * The layout is picked here instead of via definePageMeta because it depends
 * on the session: the landing draws its own header, the dashboard uses the
 * app's navigation. `isAuthenticated` reads the same `cc_session` marker in
 * SSR and on the client, so both render the same branch.
 */
definePageMeta({ layout: false })

const { isAuthenticated } = useAuth()
</script>

<template>
  <NuxtLayout v-if="isAuthenticated" name="default">
    <DashboardHome />
  </NuxtLayout>
  <LandingPage v-else />
</template>
