<script setup lang="ts">
import type { CompanySummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

const journalId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(journalId.value) || journalId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono dziennika'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `journal-edit:${journalId.value}`,
  async () => await api.journal(journalId.value)
)

const journal = computed(() => data.value?.data ?? null)
const journalDetailsLink = computed(() => `/journals/${journalId.value}`)

const form = reactive({
  companySearch: '',
  title: '',
  organizerName: '',
  organizerAddress: '',
  location: '',
  formOfTraining: '',
  legalBasis: '',
  dateStart: '',
  dateEnd: '',
  notes: ''
})

const submitPending = ref(false)
const errorMessage = ref('')
const companySearch = toRef(form, 'companySearch')
const {
  selectedOption: selectedCompany,
  options: companyOptions,
  pending: companySearchPending,
  error: companySearchError,
  showNoResults: showNoCompanyResults,
  initializeSelection: initializeCompanySelection,
  selectOption: selectCompany,
  clearSelection: clearCompanySelection
} = useSearchableSelect<CompanySummary>({
  query: companySearch,
  fetchOptions: async (search) => {
    const response = await api.companies({
      search,
      limit: 8
    })

    return response.data
  },
  getOptionLabel: companyLabel,
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy firm.')
})
const isInitialized = ref(false)
const initialSnapshot = ref('')

const trimmedTitle = computed(() => form.title.trim())
const trimmedOrganizerName = computed(() => form.organizerName.trim())
const trimmedLocation = computed(() => form.location.trim())
const trimmedFormOfTraining = computed(() => form.formOfTraining.trim())
const trimmedLegalBasis = computed(() => form.legalBasis.trim())
const isClosed = computed(() => journal.value?.status === 'closed')

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

function companyLabel(company: Pick<CompanySummary, 'name' | 'city'>) {
  if (company.city) {
    return `${company.name} · ${company.city}`
  }

  return company.name
}

function buildPayload() {
  return {
    companyId: selectedCompany.value?.id ?? null,
    title: trimmedTitle.value,
    organizerName: trimmedOrganizerName.value,
    organizerAddress: optionalValue(form.organizerAddress),
    location: trimmedLocation.value,
    formOfTraining: trimmedFormOfTraining.value,
    legalBasis: trimmedLegalBasis.value,
    dateStart: form.dateStart,
    dateEnd: form.dateEnd,
    notes: optionalValue(form.notes)
  }
}

function applyJournalToForm() {
  if (!journal.value) {
    return
  }

  form.title = journal.value.title || ''
  form.organizerName = journal.value.organizerName || ''
  form.organizerAddress = journal.value.organizerAddress || ''
  form.location = journal.value.location || ''
  form.formOfTraining = journal.value.formOfTraining || ''
  form.legalBasis = journal.value.legalBasis || ''
  form.dateStart = journal.value.dateStart || ''
  form.dateEnd = journal.value.dateEnd || ''
  form.notes = journal.value.notes || ''

  if (journal.value.companyId && journal.value.companyName) {
    initializeCompanySelection({
      id: journal.value.companyId,
      name: journal.value.companyName,
      city: '',
      nip: '',
      contactPerson: '',
      telephone: ''
    })
  } else {
    initializeCompanySelection(null)
  }

  errorMessage.value = ''
  initialSnapshot.value = JSON.stringify(buildPayload())
  isInitialized.value = true
}

watchEffect(() => {
  if (!journal.value || isInitialized.value) {
    return
  }

  applyJournalToForm()
})

const isDirty = computed(() => {
  if (!isInitialized.value) {
    return false
  }

  return JSON.stringify(buildPayload()) !== initialSnapshot.value
})

const canSubmit = computed(() => {
  return !submitPending.value && !pending.value && isDirty.value && !error.value && !!journal.value && !isClosed.value
})

const hasOptionalJournalData = computed(() => {
  return Boolean(
    selectedCompany.value
    || form.organizerAddress.trim()
    || form.notes.trim()
  )
})

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

async function onRefresh() {
  if (isDirty.value && !window.confirm('Masz niezapisane zmiany. Odświeżyć dane dziennika z serwera?')) {
    return
  }

  await refresh()
  isInitialized.value = false
  applyJournalToForm()
}

function resetForm() {
  applyJournalToForm()
}

async function onSubmit() {
  errorMessage.value = ''

  if (
    !trimmedTitle.value
    || !trimmedOrganizerName.value
    || !trimmedLocation.value
    || !trimmedFormOfTraining.value
    || !trimmedLegalBasis.value
    || !form.dateStart
    || !form.dateEnd
  ) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  if (form.dateEnd < form.dateStart) {
    errorMessage.value = 'Data zakończenia nie może być wcześniejsza niż data rozpoczęcia.'
    return
  }

  submitPending.value = true

  try {
    await api.updateJournal(journalId.value, buildPayload())
    await navigateTo(journalDetailsLink.value)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać zmian dziennika.')
  } finally {
    submitPending.value = false
  }
}

function statusLabel(value: string) {
  return value === 'closed' ? 'Zamknięty' : 'Roboczy'
}

useSeoMeta({
  title: () => journal.value ? `Edycja: ${journal.value.title}` : 'Edycja dziennika'
})
</script>

<template>
  <section class="space-y-8">
    <div class="sticky top-4 z-20 flex flex-col gap-4 rounded-xl border border-white/60 bg-white/90 p-6 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Dzienniki
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Edycja nagłówka dziennika
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Zaktualizuj dane organizacyjne szkolenia bez ingerencji w uczestników, obecności i program.
        </p>
      </div>

      <div class="flex flex-col items-stretch gap-3 sm:items-end">
        <span
          class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
          :class="isDirty
            ? 'border-amber-200 bg-amber-50 text-amber-700'
            : 'border-emerald-200 bg-emerald-50 text-emerald-700'"
        >
          {{ isDirty ? 'Niezapisane zmiany' : 'Brak zmian' }}
        </span>

        <div class="flex flex-wrap items-center gap-3">
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            :loading="pending"
            @click="onRefresh"
          >
            Odśwież
          </UButton>

          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="!isDirty || submitPending || pending"
            @click="resetForm"
          >
            Przywróć
          </button>

          <NuxtLink
            :to="journalDetailsLink"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            form="journal-edit-form"
            type="submit"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="!canSubmit"
          >
            {{ submitPending ? 'Zapisywanie...' : 'Zapisz zmiany' }}
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać danych dziennika.
    </div>

    <div
      v-else-if="pending || !journal"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie formularza edycji...
    </div>

    <div
      v-else
      class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]"
    >
      <form
        id="journal-edit-form"
        class="space-y-6"
        novalidate
        :data-show-validation="errorMessage ? 'true' : null"
        @submit.prevent="onSubmit"
      >
        <div
          v-if="isClosed"
          class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
        >
          Ten dziennik jest zamknięty. Nagłówek nie może już być edytowany.
        </div>

        <div
          v-if="errorMessage"
          class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          {{ errorMessage }}
        </div>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Podstawowe informacje
            </h2>
            <p class="text-sm text-slate-500">
              Zmieniasz tylko nagłówek dziennika. Kurs i liczba godzin pozostają bez zmian.
            </p>
          </div>

          <div class="mt-5 rounded-md border border-slate-200 bg-slate-50/80 p-4">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-slate-900">
                  Dane wymagane
                </h3>
                <p class="mt-1 text-xs leading-5 text-slate-500">
                  Te pola definiują tożsamość i ramy organizacyjne szkolenia.
                </p>
              </div>

              <span class="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500">
                8 pól
              </span>
            </div>

            <div class="mt-4 grid gap-4 md:grid-cols-2">
              <div class="space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Kurs</span>
                <div class="rounded-md border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700">
                  {{ journal.courseSymbol }} · {{ journal.courseName }}
                </div>
              </div>

              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Tytuł dziennika</span>
                <input
                  v-model="form.title"
                  type="text"
                  required
                  :disabled="isClosed"
                  placeholder="np. Szkolenie okresowe BHP dla pracowników administracyjno-biurowych"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Organizator</span>
                <input
                  v-model="form.organizerName"
                  type="text"
                  required
                  :disabled="isClosed"
                  placeholder="np. CoursesList"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Miejsce szkolenia</span>
                <input
                  v-model="form.location"
                  type="text"
                  required
                  :disabled="isClosed"
                  placeholder="np. Warszawa, sala 2"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Forma szkolenia</span>
                <input
                  v-model="form.formOfTraining"
                  type="text"
                  required
                  :disabled="isClosed"
                  placeholder="np. kurs stacjonarny"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Data rozpoczęcia</span>
                <input
                  v-model="form.dateStart"
                  type="date"
                  required
                  :disabled="isClosed"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Data zakończenia</span>
                <input
                  v-model="form.dateEnd"
                  type="date"
                  required
                  :disabled="isClosed"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                >
              </label>

              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Podstawa prawna</span>
                <textarea
                  v-model="form.legalBasis"
                  rows="3"
                  required
                  :disabled="isClosed"
                  placeholder="np. Rozporządzenie Ministra Gospodarki i Pracy z dnia..."
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                />
              </label>
            </div>
          </div>

          <details
            class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
            :open="hasOptionalJournalData"
          >
            <summary class="cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-700 marker:hidden">
              <span class="flex items-center justify-between gap-3">
                <span>Dane dodatkowe</span>
                <span class="text-xs text-slate-400">opcjonalne</span>
              </span>
            </summary>

            <div class="border-t border-slate-200 px-4 py-4">
              <div class="grid gap-4 md:grid-cols-2">
                <div class="space-y-3 md:col-span-2">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Firma</span>
                    <input
                      v-model="form.companySearch"
                      type="text"
                      :disabled="isClosed"
                      placeholder="Wpisz co najmniej 2 znaki, aby wyszukać firmę"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                    >
                  </label>

                  <div
                    v-if="companySearchError"
                    class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
                  >
                    {{ companySearchError }}
                  </div>

                  <div
                    v-if="selectedCompany"
                    class="flex items-center justify-between rounded-md border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800"
                  >
                    <div>
                      <p class="font-medium">
                        {{ selectedCompany.name }}
                      </p>
                      <p v-if="selectedCompany.city" class="text-xs text-sky-700">
                        {{ selectedCompany.city }}
                      </p>
                    </div>

                    <button
                      type="button"
                      class="rounded-md border border-sky-200 bg-white px-3 py-1.5 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="isClosed"
                      @click="clearCompanySelection"
                    >
                      Usuń wybór
                    </button>
                  </div>

                  <div
                    v-else-if="companySearchPending"
                    class="rounded-md border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500"
                  >
                    Szukanie firm...
                  </div>

                  <div
                    v-else-if="showNoCompanyResults"
                    class="rounded-md border border-dashed border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-500"
                  >
                    Nie znaleziono firmy pasującej do podanej frazy.
                  </div>

                  <div
                    v-else-if="companyOptions.length"
                    class="overflow-hidden rounded-md border border-slate-200 bg-white"
                  >
                    <button
                      v-for="company in companyOptions"
                      :key="company.id"
                      type="button"
                      class="flex w-full items-start justify-between gap-4 border-b border-slate-200 px-4 py-3 text-left transition last:border-b-0 hover:bg-slate-50"
                      @click="selectCompany(company)"
                    >
                      <div>
                        <p class="font-medium text-slate-900">
                          {{ company.name }}
                        </p>
                        <p class="text-xs text-slate-500">
                          {{ company.city || 'Brak miasta' }}
                        </p>
                      </div>

                      <span class="text-xs uppercase tracking-[0.16em] text-slate-400">
                        ID {{ company.id }}
                      </span>
                    </button>
                  </div>
                </div>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Adres organizatora</span>
                  <input
                    v-model="form.organizerAddress"
                    type="text"
                    :disabled="isClosed"
                    placeholder="np. ul. Szkolna 12, 00-001 Warszawa"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                  >
                </label>

                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium text-slate-700">Notatki</span>
                  <textarea
                    v-model="form.notes"
                    rows="4"
                    :disabled="isClosed"
                    placeholder="Dodatkowe informacje do wewnętrznego użytku."
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
                  />
                </label>
              </div>
            </div>
          </details>
        </section>
      </form>

      <aside class="space-y-4">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Podsumowanie
            </h2>
            <p class="text-sm text-slate-500">
              Szybki podgląd danych, które zapiszesz w nagłówku dziennika.
            </p>
          </div>

          <dl class="mt-5 grid gap-4 text-sm">
            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kurs
              </dt>
              <dd class="text-slate-700">
                {{ journal.courseSymbol }} · {{ journal.courseName }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Status
              </dt>
              <dd class="text-slate-700">
                {{ statusLabel(journal.status) }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Tytuł
              </dt>
              <dd class="text-slate-700">
                {{ trimmedTitle || 'Brak tytułu' }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Firma
              </dt>
              <dd class="text-slate-700">
                {{ selectedCompany ? companyLabel(selectedCompany) : 'Bez przypisanej firmy' }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Termin
              </dt>
              <dd class="text-slate-700">
                {{ form.dateStart && form.dateEnd ? `${form.dateStart} - ${form.dateEnd}` : 'Brak dat' }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Liczba uczestników
              </dt>
              <dd class="text-slate-700">
                {{ journal.attendeesCount }}
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Liczba zajęć
              </dt>
              <dd class="text-slate-700">
                {{ journal.sessionsCount }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
