<script setup lang="ts">
definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Kursanci'
})

const api = useApi()
const search = ref('')

function formatBirthDate(value: string) {
  const [year, month, day] = value.split('-')
  if (!year || !month || !day) {
    return value
  }

  return `${day}.${month}.${year}`
}

const normalizedSearch = computed(() => search.value.trim())

const { data, pending, error, refresh } = await useAsyncData(
  () => `students:${normalizedSearch.value || 'all'}`,
  async () => await api.students({
    search: normalizedSearch.value || undefined,
    limit: 50
  }),
  {
    watch: [normalizedSearch]
  }
)

const students = computed(() => data.value?.data ?? [])
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursanci
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Baza kursantów
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Wyszukuj osoby po nazwisku, imieniu albo numerze PESEL i przechodź do formularza
          wystawiania nowego zaświadczenia.
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
          to="/students/new"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Nowy kursant
        </NuxtLink>

        <NuxtLink
          to="/certificates/new"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Nowe zaświadczenie
        </NuxtLink>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
      <label class="block space-y-2">
        <span class="text-sm font-medium text-slate-700">Szukaj kursanta</span>
        <input
          v-model="search"
          type="text"
          placeholder="Np. Nowak, Jan albo 90011012345"
          class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
        >
      </label>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać listy kursantów.
    </div>

    <div
      v-else-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie listy kursantów...
    </div>

    <div
      v-else-if="students.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
    >
      <div class="space-y-4">
        <p>
          Brak wyników dla podanej frazy.
        </p>
        <NuxtLink
          to="/students/new"
          class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
        >
          Dodaj nowego kursanta
        </NuxtLink>
      </div>
    </div>

    <div
      v-else
      class="grid gap-4"
    >
      <article
        v-for="student in students"
        :key="student.id"
        class="grid gap-4 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm md:grid-cols-[minmax(0,1fr)_15rem]"
      >
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
            <span>ID {{ student.id }}</span>
            <span v-if="student.company">•</span>
            <span v-if="student.company">{{ student.company.name }}</span>
          </div>

          <NuxtLink
            :to="`/students/${student.id}`"
            class="inline-block text-lg font-semibold text-slate-900 transition hover:text-sky-700"
          >
            {{ student.lastName }} {{ student.firstName }}
          </NuxtLink>

          <div class="grid gap-2 text-sm text-slate-600 sm:grid-cols-2">
            <p>
              <span class="font-medium text-slate-700">PESEL:</span>
              {{ student.pesel || 'Brak' }}
            </p>
            <p>
              <span class="font-medium text-slate-700">Data urodzenia:</span>
              {{ formatBirthDate(student.birthDate) }}
            </p>
          </div>
        </div>

        <div class="flex flex-col items-start gap-3 md:items-end">
          <NuxtLink
            :to="`/students/${student.id}`"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Szczegóły
          </NuxtLink>

          <NuxtLink
            :to="{
              path: '/certificates/new',
              query: {
                studentId: student.id,
                firstName: student.firstName,
                lastName: student.lastName,
                companyName: student.company?.name || undefined
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
