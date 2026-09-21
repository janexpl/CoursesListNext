<script setup lang="ts">
// Podmenu administracji. Wcześniej każdy dział administracyjny miał własną pozycję
// w nawigacji głównej - przy trzecim (nadruki) pasek zrobił się nieczytelny, więc
// w nagłówku została jedna zakładka, a przełączanie działów jest tutaj.
const route = useRoute()

const sections = [
  { label: 'Użytkownicy', to: '/admin/users' },
  { label: 'Klucze API', to: '/admin/api-keys' },
  { label: 'Nadruki zaświadczeń', to: '/admin/certificate-print-assets' }
]

function isActive(path: string) {
  return route.path === path || route.path.startsWith(`${path}/`)
}
</script>

<template>
  <nav class="flex flex-wrap gap-2" aria-label="Sekcje administracji">
    <NuxtLink
      v-for="section in sections"
      :key="section.to"
      :to="section.to"
      class="rounded-lg px-3 py-2 text-sm transition"
      :class="
        isActive(section.to)
          ? 'bg-sky-100 text-sky-900'
          : 'text-slate-500 hover:bg-white hover:text-slate-900'
      "
    >
      {{ section.label }}
    </NuxtLink>
  </nav>
</template>
