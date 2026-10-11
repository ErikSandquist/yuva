<script setup lang="ts">
const route = useRoute()
const { data: page } = await useAsyncData(route.path, () => queryCollection("content").path(route.path).first())
if (!page.value) {
  throw createError({ statusCode: 404, statusMessage: "Page not found", fatal: true })
}
useSeoMeta({ title: page.value.title, description: page.value.description })
</script>

<template>
  <article v-if="page">
    <h1>{{ page.title }}</h1>
    <ContentRenderer :value="page" />
    <YuvaPageFeedback :page-title="page.title" />
    <YuvaDiscussions :page-title="page.title" />
  </article>
</template>
