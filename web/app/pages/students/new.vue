<script setup lang="ts">
import type { CompanySummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

function formatCompanyLabel(company: Pick<CompanySummary, 'name' | 'city'>) {
  if (company.city) {
    return `${company.name} · ${company.city}`
  }

  return company.name
}

const form = reactive({
  firstName: '',
  lastName: '',
  secondName: '',
  birthDate: '',
  birthPlace: '',
  pesel: '',
  addressStreet: '',
  addressCity: '',
  addressZip: '',
  telephone: '',
  companySearch: ''
})

const createCompanyForm = reactive({
  name: '',
  street: '',
  city: '',
  zipcode: '',
  nip: '',
  telephone: '',
  email: '',
  contactPerson: '',
  note: ''
})

const createCompanyPending = ref(false)
const showCreateCompanyForm = ref(false)
const submitPending = ref(false)
const errorMessage = ref('')
const companyCreateError = ref('')
const companySearch = toRef(form, 'companySearch')

const {
  selectedOption: selectedCompany,
  options: companyOptions,
  pending: companiesPending,
  error: companySearchError,
  normalizedQuery: normalizedCompanySearch,
  showNoResults: showNoCompanyResults,
  initializeSelection: initializeCompanySelection,
  selectOption: selectCompanyOption,
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
  getOptionLabel: formatCompanyLabel,
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy firm.')
})

const prefilledCompanyId = computed(() => {
  const value = Number.parseInt(`${route.query.companyId || ''}`, 10)
  return Number.isFinite(value) && value > 0 ? value : null
})

const prefilledCompanyName = computed(() => {
  const value = typeof route.query.companyName === 'string' ? route.query.companyName.trim() : ''
  return value || null
})

if (prefilledCompanyId.value && prefilledCompanyName.value) {
  initializeCompanySelection({
    id: prefilledCompanyId.value,
    name: prefilledCompanyName.value,
    city: '',
    nip: '',
    contactPerson: '',
    telephone: ''
  })
}

function selectCompany(company: CompanySummary) {
  selectCompanyOption(company)
  companyCreateError.value = ''
  showCreateCompanyForm.value = false
}

const trimmedFirstName = computed(() => form.firstName.trim())
const trimmedLastName = computed(() => form.lastName.trim())
const trimmedBirthDate = computed(() => form.birthDate.trim())
const trimmedBirthPlace = computed(() => form.birthPlace.trim())
const personalDataComplete = computed(() => {
  return !!(
    trimmedFirstName.value
    && trimmedLastName.value
    && trimmedBirthDate.value
    && trimmedBirthPlace.value
  )
})
const hasOptionalStudentData = computed(() => {
  return Boolean(
    form.secondName.trim()
    || form.pesel.trim()
    || form.addressStreet.trim()
    || form.addressCity.trim()
    || form.addressZip.trim()
    || form.telephone.trim()
  )
})
const hasOptionalCreateCompanyData = computed(() => {
  return Boolean(
    createCompanyForm.email.trim()
    || createCompanyForm.contactPerson.trim()
    || createCompanyForm.note.trim()
  )
})

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

const payload = computed(() => ({
  firstName: trimmedFirstName.value,
  lastName: trimmedLastName.value,
  secondName: optionalValue(form.secondName),
  birthDate: trimmedBirthDate.value,
  birthPlace: trimmedBirthPlace.value,
  pesel: optionalValue(form.pesel),
  addressStreet: optionalValue(form.addressStreet),
  addressCity: optionalValue(form.addressCity),
  addressZip: optionalValue(form.addressZip),
  telephone: optionalValue(form.telephone),
  companyId: selectedCompany.value?.id ?? null
}))

const createCompanyPayload = computed(() => ({
  name: createCompanyForm.name.trim(),
  street: createCompanyForm.street.trim(),
  city: createCompanyForm.city.trim(),
  zipcode: createCompanyForm.zipcode.trim(),
  nip: createCompanyForm.nip.trim(),
  telephone: createCompanyForm.telephone.trim(),
  email: optionalValue(createCompanyForm.email),
  contactPerson: optionalValue(createCompanyForm.contactPerson),
  note: optionalValue(createCompanyForm.note)
}))

const isDirty = computed(() => {
  return (
    !!selectedCompany.value
    || Object.values(form).some(value => value.trim() !== '')
    || Object.values(createCompanyForm).some(value => value.trim() !== '')
  )
})

const canSubmit = computed(() => !submitPending.value && isDirty.value)
const fullName = computed(() => {
  return [trimmedLastName.value, trimmedFirstName.value, optionalValue(form.secondName)]
    .filter(Boolean)
    .join(' ')
})

const fullAddress = computed(() => {
  return [
    optionalValue(form.addressStreet),
    [optionalValue(form.addressZip), optionalValue(form.addressCity)].filter(Boolean).join(' ')
  ]
    .filter(Boolean)
    .join(', ')
})

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

function resetForm() {
  form.firstName = ''
  form.lastName = ''
  form.secondName = ''
  form.birthDate = ''
  form.birthPlace = ''
  form.pesel = ''
  form.addressStreet = ''
  form.addressCity = ''
  form.addressZip = ''
  form.telephone = ''
  clearCompanySelection()

  if (prefilledCompanyId.value && prefilledCompanyName.value) {
    initializeCompanySelection({
      id: prefilledCompanyId.value,
      name: prefilledCompanyName.value,
      city: '',
      nip: '',
      contactPerson: '',
      telephone: ''
    })
  }

  errorMessage.value = ''
  companyCreateError.value = ''
  resetCreateCompanyForm()
}

function resetCreateCompanyForm() {
  createCompanyForm.name = ''
  createCompanyForm.street = ''
  createCompanyForm.city = ''
  createCompanyForm.zipcode = ''
  createCompanyForm.nip = ''
  createCompanyForm.telephone = ''
  createCompanyForm.email = ''
  createCompanyForm.contactPerson = ''
  createCompanyForm.note = ''
}

function openCreateCompanyForm() {
  showCreateCompanyForm.value = true
  companyCreateError.value = ''

  if (!createCompanyForm.name.trim() && normalizedCompanySearch.value) {
    createCompanyForm.name = normalizedCompanySearch.value
  }
}

async function onCreateCompany() {
  companyCreateError.value = ''

  if (
    !createCompanyPayload.value.name
    || !createCompanyPayload.value.street
    || !createCompanyPayload.value.city
    || !createCompanyPayload.value.zipcode
    || !createCompanyPayload.value.nip
    || !createCompanyPayload.value.telephone
  ) {
    companyCreateError.value = 'Uzupełnij wszystkie wymagane pola nowej firmy.'
    return
  }

  createCompanyPending.value = true

  try {
    const response = await api.createCompany(createCompanyPayload.value)
    const company = response.data

    selectCompany({
      id: company.id,
      name: company.name,
      city: company.city,
      nip: company.nip,
      contactPerson: company.contactPerson ?? '',
      telephone: company.telephone
    })
    resetCreateCompanyForm()
  } catch (error) {
    companyCreateError.value = getApiErrorMessage(error, 'Nie udało się utworzyć firmy.')
  } finally {
    createCompanyPending.value = false
  }
}

async function onSubmit() {
  errorMessage.value = ''

  if (
    !trimmedFirstName.value
    || !trimmedLastName.value
    || !trimmedBirthDate.value
    || !trimmedBirthPlace.value
  ) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createStudent(payload.value)
    await navigateTo(`/students/${response.data.id}`)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się utworzyć kursanta.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: 'Nowy kursant'
})
</script>

<template>
  <section class="space-y-8">
    <div class="sticky top-4 z-20 flex flex-col gap-4 rounded-xl border border-white/60 bg-white/90 p-6 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursanci
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Nowy kursant
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Dodaj nową osobę do bazy i opcjonalnie przypisz ją od razu do firmy.
        </p>

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="personalDataComplete
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ personalDataComplete ? 'Dane wymagane gotowe' : 'Uzupełnij dane wymagane' }}
          </span>
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="selectedCompany
              ? 'border-sky-200 bg-sky-50 text-sky-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ selectedCompany ? 'Firma przypisana' : 'Firma opcjonalna' }}
          </span>
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="hasOptionalStudentData
              ? 'border-sky-200 bg-sky-50 text-sky-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ hasOptionalStudentData ? 'Dodano dane dodatkowe' : 'Dane dodatkowe opcjonalne' }}
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
          {{ isDirty ? 'Wypełniasz nowego kursanta' : 'Formularz pusty' }}
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
            to="/students"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            form="student-create-form"
            type="submit"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="!canSubmit"
          >
            {{ submitPending ? 'Zapisywanie...' : 'Utwórz kursanta' }}
          </button>
        </div>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <form
        id="student-create-form"
        class="space-y-6"
        novalidate
        :data-show-validation="errorMessage ? 'true' : null"
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
              Dane osobowe
            </h2>
            <p class="text-sm text-slate-500">
              Najpierw uzupełnij pola wymagane. Dane dodatkowe możesz rozwinąć niżej.
            </p>
          </div>

          <div class="mt-5 rounded-md border border-slate-200 bg-slate-50/80 p-4">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-slate-900">
                  Dane wymagane
                </h3>
                <p class="mt-1 text-xs leading-5 text-slate-500">
                  Wystarczą, aby zapisać kursanta i później wystawiać mu zaświadczenia.
                </p>
              </div>

              <span class="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500">
                4 pola
              </span>
            </div>

            <div class="mt-4 grid gap-4 md:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Imię</span>
                <input
                  v-model="form.firstName"
                  type="text"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Nazwisko</span>
                <input
                  v-model="form.lastName"
                  type="text"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Data urodzenia</span>
                <input
                  v-model="form.birthDate"
                  type="date"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Miejsce urodzenia</span>
                <input
                  v-model="form.birthPlace"
                  type="text"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>
            </div>
          </div>

          <details
            class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
            :open="hasOptionalStudentData"
          >
            <summary class="cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-700 marker:hidden">
              <span class="flex items-center justify-between gap-3">
                <span>Dane dodatkowe</span>
                <span class="text-xs text-slate-400">opcjonalne</span>
              </span>
            </summary>

            <div class="border-t border-slate-200 px-4 py-4">
              <div class="grid gap-4 md:grid-cols-2">
                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Drugie imię</span>
                  <input
                    v-model="form.secondName"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">PESEL</span>
                  <input
                    v-model="form.pesel"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium text-slate-700">Telefon</span>
                  <input
                    v-model="form.telephone"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium text-slate-700">Ulica</span>
                  <input
                    v-model="form.addressStreet"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Kod pocztowy</span>
                  <input
                    v-model="form.addressZip"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Miasto</span>
                  <input
                    v-model="form.addressCity"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>
              </div>
            </div>
          </details>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Firma
            </h2>
            <p class="text-sm text-slate-500">
              Możesz przypisać kursanta do istniejącej firmy już na etapie tworzenia.
            </p>
          </div>

          <div class="relative mt-5 space-y-3">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Wyszukaj firmę</span>
              <input
                v-model="form.companySearch"
                type="text"
                placeholder="Wpisz co najmniej 2 znaki, aby wyszukać firmę"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <div
              v-if="companySearchError"
              class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              {{ companySearchError }}
            </div>

            <div
              v-if="selectedCompany"
              class="flex items-center justify-between rounded-lg border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800"
            >
              <div>
                <p class="font-medium">
                  {{ selectedCompany.name }}
                </p>
                <p
                  v-if="selectedCompany.city"
                  class="text-xs text-sky-700"
                >
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
              v-if="companiesPending"
              class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-500"
            >
              Szukanie firm...
            </div>

            <div
              v-else-if="companyOptions.length"
              class="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm"
            >
              <button
                v-for="company in companyOptions"
                :key="company.id"
                type="button"
                class="flex w-full items-start justify-between gap-4 border-b border-slate-100 px-4 py-3 text-left transition last:border-b-0 hover:bg-sky-50"
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

            <div
              v-else-if="showNoCompanyResults"
              class="flex flex-col gap-3 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-4 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between"
            >
              <p>
                Nie znaleziono firmy dla frazy <span class="font-medium text-slate-900">{{ normalizedCompanySearch }}</span>. Możesz dodać ją od razu bez opuszczania formularza.
              </p>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
                @click="openCreateCompanyForm"
              >
                Dodaj nową firmę
              </button>
            </div>

            <div class="flex justify-end">
              <button
                v-if="!showCreateCompanyForm"
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                @click="openCreateCompanyForm"
              >
                Nowa firma
              </button>
            </div>

            <section
              v-if="showCreateCompanyForm"
              class="rounded-xl border border-slate-200 bg-slate-50/80 p-5"
              :data-show-validation="companyCreateError ? 'true' : null"
            >
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-900">
                    Szybkie dodanie firmy
                  </h3>
                  <p class="mt-1 text-sm text-slate-500">
                    Po zapisaniu nowa firma zostanie od razu przypisana do kursanta.
                  </p>
                </div>

                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  @click="showCreateCompanyForm = false"
                >
                  Zamknij
                </button>
              </div>

              <div
                v-if="companyCreateError"
                class="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
              >
                {{ companyCreateError }}
              </div>

              <div class="mt-4 rounded-md border border-slate-200 bg-white/90 p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <h4 class="text-sm font-semibold text-slate-900">
                      Dane wymagane
                    </h4>
                    <p class="mt-1 text-xs leading-5 text-slate-500">
                      Wystarczą, aby zapisać firmę i od razu przypisać ją do kursanta.
                    </p>
                  </div>

                  <span class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-500">
                    6 pól
                  </span>
                </div>

                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <label class="block space-y-2 md:col-span-2">
                    <span class="text-sm font-medium text-slate-700">Nazwa firmy</span>
                    <input
                      v-model="createCompanyForm.name"
                      type="text"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2 md:col-span-2">
                    <span class="text-sm font-medium text-slate-700">Ulica</span>
                    <input
                      v-model="createCompanyForm.street"
                      type="text"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Kod pocztowy</span>
                    <input
                      v-model="createCompanyForm.zipcode"
                      type="text"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Miasto</span>
                    <input
                      v-model="createCompanyForm.city"
                      type="text"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">NIP</span>
                    <input
                      v-model="createCompanyForm.nip"
                      type="text"
                      inputmode="numeric"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Telefon</span>
                    <input
                      v-model="createCompanyForm.telephone"
                      type="text"
                      inputmode="tel"
                      required
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>
                </div>
              </div>

              <details
                class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
                :open="hasOptionalCreateCompanyData"
              >
                <summary class="cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-700 marker:hidden">
                  <span class="flex items-center justify-between gap-3">
                    <span>Dane dodatkowe firmy</span>
                    <span class="text-xs text-slate-400">opcjonalne</span>
                  </span>
                </summary>

                <div class="border-t border-slate-200 px-4 py-4">
                  <div class="grid gap-4 md:grid-cols-2">
                    <label class="block space-y-2">
                      <span class="text-sm font-medium text-slate-700">E-mail</span>
                      <input
                        v-model="createCompanyForm.email"
                        type="email"
                        class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                      >
                    </label>

                    <label class="block space-y-2">
                      <span class="text-sm font-medium text-slate-700">Osoba kontaktowa</span>
                      <input
                        v-model="createCompanyForm.contactPerson"
                        type="text"
                        class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                      >
                    </label>
                  </div>

                  <label class="mt-4 block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Notatka</span>
                    <textarea
                      v-model="createCompanyForm.note"
                      rows="3"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    />
                  </label>
                </div>
              </details>

              <div class="mt-4 flex flex-wrap justify-end gap-3">
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="createCompanyPending"
                  @click="resetCreateCompanyForm"
                >
                  Wyczyść dane firmy
                </button>

                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-lg bg-slate-950 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="createCompanyPending"
                  @click="onCreateCompany"
                >
                  {{ createCompanyPending ? 'Tworzenie firmy...' : 'Utwórz i wybierz firmę' }}
                </button>
              </div>
            </section>
          </div>
        </section>
      </form>

      <aside class="space-y-6">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-sky-700">
            Podgląd rekordu
          </p>

          <dl class="mt-4 space-y-4 text-sm">
            <div>
              <dt class="text-slate-500">
                Imię i nazwisko
              </dt>
              <dd class="mt-1 font-medium text-slate-900">
                {{ fullName || 'Brak danych osobowych' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Data i miejsce urodzenia
              </dt>
              <dd class="mt-1 text-slate-900">
                {{ [trimmedBirthDate, trimmedBirthPlace].filter(Boolean).join(', ') || 'Brak danych' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Firma
              </dt>
              <dd class="mt-1 text-slate-900">
                {{ selectedCompany?.name || 'Brak przypisanej firmy' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Adres
              </dt>
              <dd class="mt-1 text-slate-900">
                {{ fullAddress || 'Brak adresu' }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
