<script setup lang="ts">
definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Kursy'
})

const api = useApi()
const search = ref('')

const normalizedSearch = computed(() => search.value.trim())

function formatExpiryLabel(value: string | null) {
  if (!value) {
    return 'Bez terminu ważności'
  }

  const numericValue = Number.parseInt(value, 10)
  if (!Number.isFinite(numericValue)) {
    return value
  }

  if (numericValue === 1) {
    return '1 rok'
  }

  return `${numericValue} lat`
}

const { data, pending, error, refresh } = await useAsyncData(
  () => `courses:${normalizedSearch.value || 'all'}`,
  async () => await api.courses({
    search: normalizedSearch.value || undefined,
    limit: 50
  }),
  {
    watch: [normalizedSearch]
  }
)

const courses = computed(() => data.value?.data ?? [])
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursy
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Baza kursów
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Wyszukuj kursy po nazwie, symbolu lub grupie głównej i przechodź do programu oraz
          szablonu zaświadczenia.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending"
          @click="refresh()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/courses/new"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Nowy kurs
        </NuxtLink>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
      <label class="block space-y-2">
        <span class="text-sm font-medium text-slate-700">Szukaj kursu</span>
        <input
          v-model="search"
          type="text"
          placeholder="Np. BHP, operator, PK albo symbol kursu"
          class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
        >
      </label>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać listy kursów.
    </div>

    <div
      v-else-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie listy kursów...
    </div>

    <div
      v-else-if="courses.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
    >
      <div class="space-y-4">
        <p>
          Brak wyników dla podanej frazy.
        </p>
        <NuxtLink
          to="/courses/new"
          class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
        >
          Dodaj nowy kurs
        </NuxtLink>
      </div>
    </div>

    <div
      v-else
      class="grid gap-4"
    >
      <article
        v-for="course in courses"
        :key="course.id"
        class="grid gap-4 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm md:grid-cols-[minmax(0,1fr)_15rem]"
      >
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
            <span>ID {{ course.id }}</span>
            <span>•</span>
            <span class="break-all">{{ course.symbol }}</span>
          </div>

          <NuxtLink
            :to="`/courses/${course.id}`"
            class="inline-block break-words text-lg font-semibold text-slate-900 transition hover:text-sky-700"
          >
            {{ course.name }}
          </NuxtLink>

          <p class="break-words text-sm text-slate-600">
            {{ course.mainName || 'Bez grupy głównej' }}
          </p>

          <p class="text-sm text-slate-500">
            {{ formatExpiryLabel(course.expiryTime) }}
          </p>
        </div>

        <div class="flex flex-col items-start gap-3 md:items-end">
          <NuxtLink
            :to="`/courses/${course.id}`"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Szczegóły
          </NuxtLink>

          <NuxtLink
            :to="{
              path: '/certificates/new',
              query: {
                courseId: course.id,
                courseName: course.name,
                courseSymbol: course.symbol,
                courseMainName: course.mainName || undefined,
                courseExpiryTime: course.expiryTime || undefined
              }
            }"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
          >
            Wystaw zaświadczenie
          </NuxtLink>
        </div>
      </article>
    </div>
  </section>
</template>
