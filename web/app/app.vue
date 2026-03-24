<script setup lang="ts">
useHead({
  htmlAttrs: {
    lang: 'pl'
  },
  meta: [
    { name: 'viewport', content: 'width=device-width, initial-scale=1' }
  ],
  link: [
    { rel: 'icon', href: '/favicon.ico' }
  ]
})

useSeoMeta({
  titleTemplate: title => title ? `${title} | CoursesList` : 'CoursesList',
  description: 'Nowy panel do zarządzania kursami, kursantami i certyfikatami.'
})
</script>

<template>
  <UApp>
    <div class="min-h-screen">
      <AppHeader />

      <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
        <NuxtPage />
      </main>
    </div>
  </UApp>
</template>
