<script setup lang="ts">
import type { CompanySummary, CourseDetails, CourseSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Nowy dziennik szkolenia'
})

const api = useApi()

const form = reactive({
  courseSearch: '',
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
const courseSearch = toRef(form, 'courseSearch')
const companySearch = toRef(form, 'companySearch')

const {
  selectedOption: selectedCourse,
  options: courseOptions,
  pending: coursesPending,
  error: courseSearchError,
  showNoResults: showNoCourseResults,
  selectOption: selectCourseOption,
  clearSelection: clearCourseSearchSelection
} = useSearchableSelect<CourseSummary>({
  query: courseSearch,
  fetchOptions: async (search) => {
    const response = await api.courses({
      search,
      limit: 20
    })

    return response.data
  },
  getOptionLabel: courseLabel,
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy kursów.')
})

const {
  selectedOption: selectedCompany,
  options: companyOptions,
  pending: companySearchPending,
  error: companySearchError,
  showNoResults: showNoCompanyResults,
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

const selectedCourseDetails = ref<CourseDetails | null>(null)
const courseDetailsPending = ref(false)
const courseDetailsLoadError = ref('')
const courseDetailsError = computed(() => courseSearchError.value || courseDetailsLoadError.value)
const lastAutoTitle = ref('')
let courseDetailsRequestId = 0

const selectedCourseId = computed(() => selectedCourse.value?.id ?? null)
const selectedCompanyId = computed(() => selectedCompany.value?.id ?? null)

const trimmedTitle = computed(() => form.title.trim())
const trimmedOrganizerName = computed(() => form.organizerName.trim())
const trimmedLocation = computed(() => form.location.trim())
const trimmedFormOfTraining = computed(() => form.formOfTraining.trim())
const trimmedLegalBasis = computed(() => form.legalBasis.trim())

const requiredJournalDataComplete = computed(() => {
  return !!(
    selectedCourseId.value
    && trimmedTitle.value
    && trimmedOrganizerName.value
    && trimmedLocation.value
    && trimmedFormOfTraining.value
    && trimmedLegalBasis.value
    && form.dateStart
    && form.dateEnd
  )
})

const hasOptionalJournalData = computed(() => {
  return Boolean(
    selectedCompany.value
    || form.organizerAddress.trim()
    || form.notes.trim()
  )
})

const isDirty = computed(() => Object.values(form).some(value => value.trim() !== ''))
const canSubmit = computed(() => !submitPending.value && isDirty.value)

const payload = computed(() => ({
  courseId: selectedCourseId.value ?? 0,
  companyId: selectedCompanyId.value,
  title: trimmedTitle.value,
  organizerName: trimmedOrganizerName.value,
  organizerAddress: optionalValue(form.organizerAddress),
  location: trimmedLocation.value,
  formOfTraining: trimmedFormOfTraining.value,
  legalBasis: trimmedLegalBasis.value,
  dateStart: form.dateStart,
  dateEnd: form.dateEnd,
  notes: optionalValue(form.notes)
}))

watch(selectedCourse, (course) => {
  const nextAutoTitle = course ? (course.mainName || course.name) : ''
  const currentTitle = form.title.trim()

  if (!currentTitle || currentTitle === lastAutoTitle.value) {
    form.title = nextAutoTitle
  }

  lastAutoTitle.value = nextAutoTitle
})

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

function parseProgramHours(value: string | undefined) {
  const normalized = `${value || ''}`.trim().replace(',', '.')
  const parsed = Number.parseFloat(normalized)
  return Number.isFinite(parsed) ? parsed : 0
}

function extractCourseProgramHours(courseProgram: string) {
  try {
    const parsed = JSON.parse(courseProgram)
    if (!Array.isArray(parsed)) {
      return null
    }

    return parsed.reduce((acc, entry) => {
      const row = entry as CourseProgramEntry
      return acc + parseProgramHours(row.TheoryTime) + parseProgramHours(row.PracticeTime)
    }, 0)
  } catch {
    return null
  }
}

async function fetchSelectedCourseDetails(courseId: number) {
  const requestId = ++courseDetailsRequestId
  courseDetailsPending.value = true
  courseDetailsLoadError.value = ''

  try {
    const response = await api.course(courseId)

    if (requestId !== courseDetailsRequestId) {
      return
    }

    selectedCourseDetails.value = response.data
  } catch (error) {
    if (requestId !== courseDetailsRequestId) {
      return
    }

    selectedCourseDetails.value = null
    courseDetailsLoadError.value = getApiErrorMessage(error, 'Nie udało się pobrać programu kursu.')
  } finally {
    if (requestId === courseDetailsRequestId) {
      courseDetailsPending.value = false
    }
  }
}

watch(selectedCourseId, (courseId) => {
  courseDetailsRequestId += 1
  courseDetailsLoadError.value = ''

  if (!courseId) {
    selectedCourseDetails.value = null
    courseDetailsPending.value = false
    return
  }

  void fetchSelectedCourseDetails(courseId)
}, { immediate: true })

const calculatedHours = computed(() => {
  if (!selectedCourseDetails.value?.courseProgram) {
    return null
  }

  return extractCourseProgramHours(selectedCourseDetails.value.courseProgram)
})

const formattedCalculatedHours = computed(() => {
  if (calculatedHours.value === null) {
    return null
  }

  return `${calculatedHours.value.toString().replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1')} h`
})

function selectCourse(course: CourseSummary) {
  selectCourseOption(course)
  errorMessage.value = ''
}

function clearCourseSelection() {
  clearCourseSearchSelection()
  selectedCourseDetails.value = null
  courseDetailsLoadError.value = ''
}

function resetForm() {
  clearCourseSelection()
  form.title = ''
  form.organizerName = ''
  form.organizerAddress = ''
  form.location = ''
  form.formOfTraining = ''
  form.legalBasis = ''
  form.dateStart = ''
  form.dateEnd = ''
  form.notes = ''
  lastAutoTitle.value = ''
  clearCompanySelection()
  errorMessage.value = ''
}

async function onSubmit() {
  errorMessage.value = ''

  if (!requiredJournalDataComplete.value) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  if (form.dateEnd < form.dateStart) {
    errorMessage.value = 'Data zakończenia nie może być wcześniejsza niż data rozpoczęcia.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createJournal(payload.value)
    await navigateTo(`/journals/${response.data.id}?created=1`)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się utworzyć dziennika.')
  } finally {
    submitPending.value = false
  }
}

function courseLabel(course: Pick<CourseSummary, 'mainName' | 'name' | 'symbol'>) {
  return `${course.symbol} · ${course.mainName || course.name}`
}

function companyLabel(company: Pick<CompanySummary, 'name' | 'city'>) {
  if (company.city) {
    return `${company.name} · ${company.city}`
  }

  return company.name
}
</script>

<template>
  <section class="space-y-8">
    <div class="sticky top-4 z-20 flex flex-col gap-4 rounded-xl border border-white/60 bg-white/90 p-6 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Dzienniki
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Nowy dziennik szkolenia
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Załóż nagłówek dziennika, a potem uzupełnisz uczestników i przebieg zajęć.
          Teraz wpisujesz podstawowe informacje o szkoleniu.
        </p>

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="requiredJournalDataComplete
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ requiredJournalDataComplete ? 'Dane wymagane gotowe' : 'Uzupełnij dane wymagane' }}
          </span>
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="hasOptionalJournalData
              ? 'border-sky-200 bg-sky-50 text-sky-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ hasOptionalJournalData ? 'Dodano dane dodatkowe' : 'Dane dodatkowe opcjonalne' }}
          </span>
        </div>
      </div>

      <div class="flex flex-col items-stretch gap-3 sm:items-end">
        <span
          class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
          :class="isDirty
            ? 'border-amber-200 bg-amber-50 text-amber-700'
            : 'border-emerald-200 bg-emerald-50 text-emerald-700'"
        >
          {{ isDirty ? 'Wypełniasz nowy dziennik' : 'Formularz pusty' }}
        </span>

        <div class="flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="!isDirty || submitPending"
            @click="resetForm"
          >
            Wyczyść
          </button>

          <NuxtLink
            to="/journals"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            form="journal-create-form"
            type="submit"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="!canSubmit"
          >
            {{ submitPending ? 'Zapisywanie...' : 'Utwórz dziennik' }}
          </button>
        </div>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <form
        id="journal-create-form"
        class="space-y-6"
        @submit.prevent="onSubmit"
      >
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
              Wskaż kurs, nazwę dziennika i kluczowe informacje organizacyjne.
            </p>
          </div>

          <div class="mt-5 rounded-md border border-slate-200 bg-slate-50/80 p-4">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-slate-900">
                  Dane wymagane
                </h3>
                <p class="mt-1 text-xs leading-5 text-slate-500">
                  Te pola są potrzebne, aby utworzyć dziennik i nadać mu podstawowy kontekst szkolenia.
                </p>
              </div>

              <span class="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500">
                8 pól
              </span>
            </div>

            <div class="mt-4 grid gap-4 md:grid-cols-2">
              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Kurs</span>
                <input
                  v-model="form.courseSearch"
                  type="text"
                  placeholder="Wpisz co najmniej 2 znaki, aby wyszukać kurs"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >

                <div
                  v-if="selectedCourse"
                  class="flex items-center justify-between rounded-md border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800"
                >
                  <div>
                    <p class="font-medium">
                      {{ courseLabel(selectedCourse) }}
                    </p>
                    <p class="text-xs text-sky-700">
                      ID {{ selectedCourse.id }}
                    </p>
                  </div>

                  <button
                    type="button"
                    class="rounded-md border border-sky-200 bg-white px-3 py-1.5 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
                    @click="clearCourseSelection"
                  >
                    Usuń wybór
                  </button>
                </div>

                <div
                  v-if="courseSearchError"
                  class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
                >
                  {{ courseSearchError }}
                </div>

                <div
                  v-else-if="coursesPending"
                  class="rounded-md border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500"
                >
                  Szukanie kursów...
                </div>

                <div
                  v-else-if="showNoCourseResults"
                  class="rounded-md border border-dashed border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-500"
                >
                  Nie znaleziono kursu pasującego do podanej frazy.
                </div>

                <div
                  v-else-if="courseOptions.length"
                  class="overflow-hidden rounded-md border border-slate-200 bg-white"
                >
                  <button
                    v-for="course in courseOptions"
                    :key="course.id"
                    type="button"
                    class="flex w-full items-start justify-between gap-4 border-b border-slate-200 px-4 py-3 text-left transition last:border-b-0 hover:bg-slate-50"
                    @click="selectCourse(course)"
                  >
                    <div>
                      <p class="font-medium text-slate-900">
                        {{ courseLabel(course) }}
                      </p>
                      <p class="text-xs text-slate-500">
                        {{ course.expiryTime ? `Ważność: ${course.expiryTime} lat` : 'Brak okresu ważności' }}
                      </p>
                    </div>

                    <span class="text-xs uppercase tracking-[0.16em] text-slate-400">
                      ID {{ course.id }}
                    </span>
                  </button>
                </div>
              </label>

              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Tytuł dziennika</span>
                <input
                  v-model="form.title"
                  type="text"
                  placeholder="np. Szkolenie okresowe BHP dla pracowników administracyjno-biurowych"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Organizator</span>
                <input
                  v-model="form.organizerName"
                  type="text"
                  placeholder="np. CoursesList"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Miejsce szkolenia</span>
                <input
                  v-model="form.location"
                  type="text"
                  placeholder="np. Warszawa, sala 2"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Forma szkolenia</span>
                <input
                  v-model="form.formOfTraining"
                  type="text"
                  placeholder="np. kurs stacjonarny"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Data rozpoczęcia</span>
                <input
                  v-model="form.dateStart"
                  type="date"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Data zakończenia</span>
                <input
                  v-model="form.dateEnd"
                  type="date"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Podstawa prawna</span>
                <textarea
                  v-model="form.legalBasis"
                  rows="3"
                  placeholder="np. Rozporządzenie Ministra Gospodarki i Pracy z dnia..."
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
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
                      placeholder="Wpisz co najmniej 2 znaki, aby wyszukać firmę"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
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
                      class="rounded-md border border-sky-200 bg-white px-3 py-1.5 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
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
                    placeholder="np. ul. Szkolna 12, 00-001 Warszawa"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium text-slate-700">Notatki</span>
                  <textarea
                    v-model="form.notes"
                    rows="4"
                    placeholder="Dodatkowe informacje do wewnętrznego użytku."
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
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
              Podgląd dziennika
            </h2>
            <p class="text-sm text-slate-500">
              Szybkie podsumowanie najważniejszych danych przed zapisem.
            </p>
          </div>

          <dl class="mt-5 grid gap-4 text-sm">
            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kurs
              </dt>
              <dd class="text-slate-700">
                {{ selectedCourse ? courseLabel(selectedCourse) : 'Nie wybrano kursu' }}
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
                Liczba godzin
              </dt>
              <dd class="text-slate-700">
                <span v-if="courseDetailsPending">Liczenie z programu kursu...</span>
                <span v-else-if="formattedCalculatedHours">{{ formattedCalculatedHours }}</span>
                <span v-else-if="courseDetailsError">{{ courseDetailsError }}</span>
                <span v-else-if="selectedCourseId">Zostanie policzona przy zapisie z programu kursu.</span>
                <span v-else>Wybierz kurs, aby policzyć godziny.</span>
              </dd>
            </div>

            <div class="space-y-1">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Status po utworzeniu
              </dt>
              <dd>
                <span class="inline-flex items-center justify-center rounded-full border border-sky-200 bg-sky-50 px-3 py-1 text-xs font-medium text-sky-700">
                  Roboczy
                </span>
              </dd>
            </div>
          </dl>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Co dalej
            </h2>
            <p class="text-sm text-slate-500">
              Po zapisaniu dziennika będziesz mógł wrócić do listy i przejść dalej do rozbudowy modułu.
            </p>
          </div>

          <ul class="mt-4 grid gap-3 text-sm text-slate-600">
            <li class="rounded-lg border border-slate-200 bg-slate-50/80 px-4 py-3">
              Uzupełnisz uczestników szkolenia.
            </li>
            <li class="rounded-lg border border-slate-200 bg-slate-50/80 px-4 py-3">
              Dodasz bloki zajęciowe i liczbę godzin.
            </li>
            <li class="rounded-lg border border-slate-200 bg-slate-50/80 px-4 py-3">
              Przygotujesz dziennik do wydruku.
            </li>
          </ul>
        </section>
      </aside>
    </div>
  </section>
</template>
