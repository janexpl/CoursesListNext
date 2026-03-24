<script setup lang="ts">
import type { CompanySummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

function formatCompanyLabel(company: Pick<CompanySummary, 'name' | 'city'>) {
  if (company.city) {
    return `${company.name} · ${company.city}`
  }

  return company.name
}

const route = useRoute()
const api = useApi()

const studentId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(studentId.value) || studentId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono kursanta'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `student-edit:${studentId.value}`,
  async () => await api.student(studentId.value)
)

const student = computed(() => data.value?.data ?? null)
const studentDetailsLink = computed(() => `/students/${studentId.value}`)

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

const submitPending = ref(false)
const errorMessage = ref('')
const companySearch = toRef(form, 'companySearch')
const {
  selectedOption: selectedCompany,
  options: companyOptions,
  pending: companiesPending,
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
  getOptionLabel: formatCompanyLabel,
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy firm.')
})
const isInitialized = ref(false)

watchEffect(() => {
  if (!student.value || isInitialized.value) {
    return
  }

  form.firstName = student.value.firstName || ''
  form.lastName = student.value.lastName || ''
  form.secondName = student.value.secondName || ''
  form.birthDate = student.value.birthDate || ''
  form.birthPlace = student.value.birthPlace || ''
  form.pesel = student.value.pesel || ''
  form.addressStreet = student.value.addressStreet || ''
  form.addressCity = student.value.addressCity || ''
  form.addressZip = student.value.addressZip || ''
  form.telephone = student.value.telephone || ''

  if (student.value.company) {
    initializeCompanySelection({
      id: student.value.company.id,
      name: student.value.company.name,
      city: '',
      nip: '',
      contactPerson: '',
      telephone: ''
    })
  } else {
    initializeCompanySelection(null)
  }

  isInitialized.value = true
})

const trimmedFirstName = computed(() => form.firstName.trim())
const trimmedLastName = computed(() => form.lastName.trim())
const trimmedBirthDate = computed(() => form.birthDate.trim())
const trimmedBirthPlace = computed(() => form.birthPlace.trim())
const trimmedTelephone = computed(() => form.telephone.trim())

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

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
    await api.updateStudent(studentId.value, {
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
    })

    await navigateTo(studentDetailsLink.value)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać zmian kursanta.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: () => student.value ? `Edycja: ${student.value.lastName} ${student.value.firstName}` : 'Edycja kursanta'
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursanci
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Edycja kursanta
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Zaktualizuj dane osobowe, kontaktowe i przypisanie do firmy.
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
          :to="studentDetailsLink"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Anuluj
        </NuxtLink>

        <button
          form="student-edit-form"
          type="submit"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="submitPending || pending"
        >
          {{ submitPending ? 'Zapisywanie...' : 'Zapisz zmiany' }}
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać danych kursanta.
    </div>

    <div
      v-else-if="pending || !student"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie formularza edycji...
    </div>

    <div
      v-else
      class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]"
    >
      <form
        id="student-edit-form"
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
              Dane osobowe
            </h2>
            <p class="text-sm text-slate-500">
              Podstawowe informacje identyfikujące kursanta.
            </p>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Imię</span>
              <input
                v-model="form.firstName"
                type="text"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwisko</span>
              <input
                v-model="form.lastName"
                type="text"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

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

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data urodzenia</span>
              <input
                v-model="form.birthDate"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Miejsce urodzenia</span>
              <input
                v-model="form.birthPlace"
                type="text"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
          </div>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Kontakt i adres
            </h2>
            <p class="text-sm text-slate-500">
              Dane kontaktowe i adresowe potrzebne w dokumentach.
            </p>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-[minmax(0,1fr)_12rem]">
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
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Firma
            </h2>
            <p class="text-sm text-slate-500">
              Wyszukaj firmę i przypisz ją do kursanta lub usuń obecne przypisanie.
            </p>
          </div>

          <div class="mt-5 space-y-4">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Szukaj firmy</span>
              <input
                v-model="form.companySearch"
                type="text"
                placeholder="Minimum 2 znaki"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <div
              v-if="companySearchError"
              class="rounded-lg border border-red-200 bg-red-50 px-4 py-4 text-sm text-red-700"
            >
              {{ companySearchError }}
            </div>

            <div
              v-if="selectedCompany"
              class="rounded-lg border border-sky-200 bg-sky-50 px-4 py-4"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-slate-900">
                    {{ selectedCompany.name }}
                  </p>
                  <p class="text-sm text-slate-600">
                    {{ selectedCompany.city || 'Brak miasta w podglądzie' }}
                  </p>
                </div>

                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  @click="clearCompanySelection"
                >
                  Usuń przypisanie
                </button>
              </div>
            </div>

            <div
              v-else-if="companiesPending"
              class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-500"
            >
              Szukanie firm...
            </div>

            <div
              v-else-if="form.companySearch.trim().length >= 2"
              class="rounded-lg border border-slate-200 bg-white"
            >
              <button
                v-for="company in companyOptions"
                :key="company.id"
                type="button"
                class="flex w-full items-center justify-between gap-4 border-b border-slate-100 px-4 py-3 text-left transition last:border-b-0 hover:bg-sky-50"
                @click="selectCompany(company)"
              >
                <div>
                  <p class="text-sm font-medium text-slate-900">
                    {{ company.name }}
                  </p>
                  <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    {{ company.city || 'Brak miasta' }}
                  </p>
                </div>

                <span class="text-xs font-semibold uppercase tracking-[0.16em] text-sky-700">
                  Wybierz
                </span>
              </button>

              <div
                v-if="showNoCompanyResults"
                class="px-4 py-4 text-sm text-slate-500"
              >
                Brak firm pasujących do podanej frazy.
              </div>
            </div>
          </div>
        </section>
      </form>

      <aside class="space-y-6">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-slate-900">
            Podsumowanie
          </h2>

          <dl class="mt-5 space-y-4">
            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kursant
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ fullName || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Data i miejsce urodzenia
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ [trimmedBirthDate, trimmedBirthPlace].filter(Boolean).join(' · ') || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Telefon
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ trimmedTelephone || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Adres
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ fullAddress || 'Brak adresu' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Firma
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ selectedCompany?.name || 'Brak przypisanej firmy' }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
