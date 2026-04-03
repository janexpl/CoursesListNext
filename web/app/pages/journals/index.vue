<script setup lang="ts">
import type { JournalSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Dzienniki szkoleń'
})

const route = useRoute()
const router = useRouter()
const api = useApi()

const search = ref('')
const debouncedSearch = ref('')
const status = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const showCreatedNotice = ref(route.query.created === '1')
const showDeletedNotice = ref(route.query.deleted === '1')

let searchDebounceTimer: ReturnType<typeof setTimeout> | undefined

watch(
  () => route.query.created,
  (value) => {
    showCreatedNotice.value = value === '1'
  }
)

watch(
  () => route.query.deleted,
  (value) => {
    showDeletedNotice.value = value === '1'
  }
)

watch(search, (value) => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }

  searchDebounceTimer = setTimeout(() => {
    debouncedSearch.value = value.trim()
  }, 300)
}, { immediate: true })

onBeforeUnmount(() => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }
})

const { data, pending, error, refresh } = await useAsyncData(
  'journals',
  async () => {
    return await api.journals({
      search: debouncedSearch.value || undefined,
      status: status.value || undefined,
      dateFrom: dateFrom.value || undefined,
      dateTo: dateTo.value || undefined,
      limit: 100
    })
  },
  {
    watch: [debouncedSearch, status, dateFrom, dateTo]
  }
)

const journals = computed(() => data.value?.data ?? [])
const hasActiveFilters = computed(() => {
  return !!(
    debouncedSearch.value
    || status.value
    || dateFrom.value
    || dateTo.value
  )
})

function statusLabel(value: string) {
  return value === 'closed' ? 'Zamknięty' : 'Roboczy'
}

function statusBadgeClass(value: string) {
  return value === 'closed'
    ? 'border-slate-300 bg-slate-100 text-slate-700'
    : 'border-sky-200 bg-sky-50 text-sky-700'
}

function formatHours(value: string) {
  return `${value.replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1')} h`
}

function formatCreatedAt(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return parsed.toLocaleDateString('pl-PL')
}

function clearFilters() {
  search.value = ''
  debouncedSearch.value = ''
  status.value = ''
  dateFrom.value = ''
  dateTo.value = ''
}

async function dismissCreatedNotice() {
  showCreatedNotice.value = false

  if (route.query.created !== '1') {
    return
  }

  const nextQuery = {
    ...route.query
  }

  delete nextQuery.created

  await router.replace({
    path: '/journals',
    query: nextQuery
  })
}

async function dismissDeletedNotice() {
  showDeletedNotice.value = false

  if (route.query.deleted !== '1') {
    return
  }

  const nextQuery = {
    ...route.query
  }

  delete nextQuery.deleted

  await router.replace({
    path: '/journals',
    query: nextQuery
  })
}

function companyLabel(journal: JournalSummary) {
  return journal.company?.name || 'Bez przypisanej firmy'
}
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Dzienniki
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Dzienniki szkoleń
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Kontroluj otwarte i zamknięte dzienniki szkoleniowe, filtruj je po statusie i terminach
          oraz przygotowuj grunt pod uzupełnianie uczestników i przebiegu zajęć.
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
          to="/journals/new"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Nowy dziennik
        </NuxtLink>
      </div>
    </div>

    <div
      v-if="showCreatedNotice"
      class="flex flex-col gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-5 py-4 text-sm text-emerald-800 sm:flex-row sm:items-center sm:justify-between"
    >
      <p>
        Dziennik został utworzony. Możesz teraz wrócić do listy albo od razu założyć kolejny.
      </p>

      <button
        type="button"
        class="inline-flex items-center justify-center rounded-lg border border-emerald-300 bg-white px-4 py-2 font-medium text-emerald-700 transition hover:border-emerald-400 hover:text-emerald-900"
        @click="dismissCreatedNotice"
      >
        Zamknij komunikat
      </button>
    </div>

    <div
      v-if="showDeletedNotice"
      class="flex flex-col gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-5 py-4 text-sm text-emerald-800 sm:flex-row sm:items-center sm:justify-between"
    >
      <p>
        Dziennik został usunięty.
      </p>

      <button
        type="button"
        class="inline-flex items-center justify-center rounded-lg border border-emerald-300 bg-white px-4 py-2 font-medium text-emerald-700 transition hover:border-emerald-400 hover:text-emerald-900"
        @click="dismissDeletedNotice"
      >
        Zamknij komunikat
      </button>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
      <div class="grid gap-4 xl:grid-cols-[minmax(0,1.7fr)_repeat(3,minmax(0,1fr))]">
        <label class="block space-y-2">
          <span class="text-sm font-medium text-slate-700">Szukaj</span>
          <input
            v-model="search"
            type="text"
            placeholder="Tytuł, symbol kursu, firma, miejsce, organizator"
            class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          >
        </label>

        <label class="block space-y-2">
          <span class="text-sm font-medium text-slate-700">Status</span>
          <div class="relative">
            <select
              v-model="status"
              class="h-12.5 w-full appearance-none rounded-md border border-slate-300 bg-white px-4 pr-10 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            >
              <option value="">
                Wszystkie
              </option>
              <option value="draft">
                Robocze
              </option>
              <option value="closed">
                Zamknięte
              </option>
            </select>
            <span class="pointer-events-none absolute inset-y-0 right-4 flex items-center text-slate-400">
              ˅
            </span>
          </div>
        </label>

        <label class="block space-y-2">
          <span class="text-sm font-medium text-slate-700">Data od</span>
          <input
            v-model="dateFrom"
            type="date"
            class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          >
        </label>

        <label class="block space-y-2">
          <span class="text-sm font-medium text-slate-700">Data do</span>
          <input
            v-model="dateTo"
            type="date"
            class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          >
        </label>
      </div>

      <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs leading-5 text-slate-500">
          Filtry obejmują całą listę dzienników, nie tylko wpisy widoczne na ekranie.
        </p>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="!hasActiveFilters"
          @click="clearFilters"
        >
          Wyczyść filtry
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać listy dzienników.
    </div>

    <div
      v-else-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie listy dzienników...
    </div>

    <div
      v-else-if="journals.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
    >
      {{ hasActiveFilters ? 'Brak dzienników spełniających podane kryteria.' : 'Nie ma jeszcze żadnych dzienników szkoleniowych.' }}
    </div>

    <div v-else class="grid gap-4">
      <NuxtLink
        v-for="journal in journals"
        :key="journal.id"
        :to="`/journals/${journal.id}`"
        class="grid gap-5 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm transition hover:border-sky-300 hover:bg-white lg:grid-cols-[minmax(0,1fr)_18rem]"
      >
        <div class="space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
              :class="statusBadgeClass(journal.status)"
            >
              {{ statusLabel(journal.status) }}
            </span>

            <span class="text-xs uppercase tracking-[0.16em] text-slate-400">
              {{ journal.courseSymbol }}
            </span>
          </div>

          <div class="space-y-2">
            <h2 class="text-xl font-semibold tracking-tight text-slate-900">
              {{ journal.title }}
            </h2>
            <p class="text-sm text-slate-500">
              {{ companyLabel(journal) }}
            </p>
          </div>

          <dl class="grid gap-4 text-sm sm:grid-cols-2">
            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Organizator
              </dt>
              <dd class="text-slate-700">
                {{ journal.organizerName }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Miejsce
              </dt>
              <dd class="text-slate-700">
                {{ journal.location }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Forma
              </dt>
              <dd class="text-slate-700">
                {{ journal.formOfTraining }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Utworzono
              </dt>
              <dd class="text-slate-700">
                {{ formatCreatedAt(journal.createdAt) }}
              </dd>
            </div>
          </dl>
        </div>

        <div class="grid gap-3 rounded-lg border border-slate-200 bg-slate-50/80 p-4 text-sm lg:content-start">
          <div>
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
              Termin
            </p>
            <p class="mt-1 font-medium text-slate-700">
              {{ journal.dateStart }} - {{ journal.dateEnd }}
            </p>
          </div>

          <div>
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
              Liczba godzin
            </p>
            <p class="mt-1 font-medium text-slate-700">
              {{ formatHours(journal.totalHours) }}
            </p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="rounded-lg border border-slate-200 bg-white px-4 py-3">
              <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Uczestnicy
              </p>
              <p class="mt-1 text-lg font-semibold text-slate-900">
                {{ journal.attendeesCount }}
              </p>
            </div>

            <div class="rounded-lg border border-slate-200 bg-white px-4 py-3">
              <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Zajęcia
              </p>
              <p class="mt-1 text-lg font-semibold text-slate-900">
                {{ journal.sessionsCount }}
              </p>
            </div>
          </div>
        </div>
      </NuxtLink>
    </div>
  </section>
</template>
