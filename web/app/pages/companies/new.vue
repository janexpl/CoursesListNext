<script setup lang="ts">
import type { GUSCompanyDetails } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const api = useApi()

const form = reactive({
  name: '',
  street: '',
  city: '',
  zipcode: '',
  nip: '',
  email: '',
  contactPerson: '',
  telephone: '',
  note: ''
})

const submitPending = ref(false)
const errorMessage = ref('')
const lookupPending = ref(false)
const lookupError = ref('')
const lookupSuccess = ref('')

const trimmedName = computed(() => form.name.trim())
const trimmedStreet = computed(() => form.street.trim())
const trimmedCity = computed(() => form.city.trim())
const trimmedZipcode = computed(() => form.zipcode.trim())
const trimmedNip = computed(() => form.nip.trim())
const trimmedTelephone = computed(() => form.telephone.trim())
const normalizedNip = computed(() => form.nip.replaceAll(/\D/g, ''))
const requiredCompanyDataComplete = computed(() => {
  return !!(
    trimmedName.value
    && trimmedStreet.value
    && trimmedCity.value
    && trimmedZipcode.value
    && trimmedNip.value
    && trimmedTelephone.value
  )
})
const hasOptionalCompanyData = computed(() => {
  return Boolean(form.email.trim() || form.contactPerson.trim() || form.note.trim())
})

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

const payload = computed(() => ({
  name: trimmedName.value,
  street: trimmedStreet.value,
  city: trimmedCity.value,
  zipcode: trimmedZipcode.value,
  nip: trimmedNip.value,
  email: optionalValue(form.email),
  contactPerson: optionalValue(form.contactPerson),
  telephone: trimmedTelephone.value,
  note: optionalValue(form.note)
}))

const isDirty = computed(() => Object.values(form).some(value => value.trim() !== ''))
const canSubmit = computed(() => !submitPending.value && isDirty.value)
const canLookupCompanyByNip = computed(() => !lookupPending.value && !submitPending.value && normalizedNip.value.length > 0)

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

watch(() => form.nip, () => {
  lookupError.value = ''
  lookupSuccess.value = ''
})

function resetForm() {
  form.name = ''
  form.street = ''
  form.city = ''
  form.zipcode = ''
  form.nip = ''
  form.email = ''
  form.contactPerson = ''
  form.telephone = ''
  form.note = ''
  errorMessage.value = ''
  lookupError.value = ''
  lookupSuccess.value = ''
}

function buildStreetFromGUS(company: GUSCompanyDetails) {
  const housePart = [company.houseNumber, company.apartment ? `/${company.apartment}` : '']
    .filter(Boolean)
    .join('')

  return [company.street.trim(), housePart.trim()].filter(Boolean).join(' ').trim()
}

function applyCompanyLookup(company: GUSCompanyDetails) {
  form.nip = company.nip
  form.name = company.name
  form.street = buildStreetFromGUS(company)
  form.city = company.city
  form.zipcode = company.postalCode
}

async function onLookupCompanyByNip() {
  errorMessage.value = ''
  lookupError.value = ''
  lookupSuccess.value = ''

  if (!normalizedNip.value) {
    lookupError.value = 'Wpisz NIP, aby pobrać dane z GUS.'
    return
  }

  lookupPending.value = true

  try {
    const response = await api.lookupCompanyByNIP(form.nip)
    applyCompanyLookup(response.data)
    lookupSuccess.value = `Uzupełniono dane firmy ${response.data.name} na podstawie rejestru GUS.`
  } catch (error) {
    lookupError.value = getApiErrorMessage(error, 'Nie udało się pobrać danych firmy z GUS.')
  } finally {
    lookupPending.value = false
  }
}

async function onSubmit() {
  errorMessage.value = ''

  if (
    !trimmedName.value
    || !trimmedStreet.value
    || !trimmedCity.value
    || !trimmedZipcode.value
    || !trimmedNip.value
    || !trimmedTelephone.value
  ) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createCompany(payload.value)
    await navigateTo(`/companies/${response.data.id}`)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się utworzyć firmy.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: 'Nowa firma'
})
</script>

<template>
  <section class="space-y-8">
    <div class="sticky top-4 z-20 flex flex-col gap-4 rounded-xl border border-white/60 bg-white/90 p-6 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Firmy
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Nowa firma
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Dodaj nowego klienta do bazy, aby przypisywać do niego kursantów i wystawiać zaświadczenia.
        </p>

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="requiredCompanyDataComplete
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ requiredCompanyDataComplete ? 'Dane wymagane gotowe' : 'Uzupełnij dane wymagane' }}
          </span>
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="hasOptionalCompanyData
              ? 'border-sky-200 bg-sky-50 text-sky-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ hasOptionalCompanyData ? 'Dodano dane dodatkowe' : 'Dane dodatkowe opcjonalne' }}
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
          {{ isDirty ? 'Wypełniasz nową firmę' : 'Formularz pusty' }}
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
            to="/companies"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            form="company-create-form"
            type="submit"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="!canSubmit"
          >
            {{ submitPending ? 'Zapisywanie...' : 'Utwórz firmę' }}
          </button>
        </div>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <form
        id="company-create-form"
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
              Dane firmy
            </h2>
            <p class="text-sm text-slate-500">
              Najpierw uzupełnij dane wymagane. Dane dodatkowe możesz rozwinąć niżej.
            </p>
          </div>

          <div class="mt-5 rounded-md border border-slate-200 bg-slate-50/80 p-4">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-slate-900">
                  Dane wymagane
                </h3>
                <p class="mt-1 text-xs leading-5 text-slate-500">
                  Te pola są potrzebne, żeby zapisać firmę i przypisywać do niej kursantów.
                </p>
              </div>

              <span class="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500">
                6 pól
              </span>
            </div>

            <div class="mt-4 grid gap-4 md:grid-cols-2">
              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Nazwa firmy</span>
                <input
                  v-model="form.name"
                  type="text"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">NIP</span>
                <div class="space-y-3">
                  <input
                    v-model="form.nip"
                    type="text"
                    inputmode="numeric"
                    placeholder="np. 5210000000"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >

                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <p class="text-xs leading-5 text-slate-500">
                      Nazwa i adres mogą zostać uzupełnione automatycznie na podstawie rejestru GUS.
                    </p>

                    <button
                      type="button"
                      class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="!canLookupCompanyByNip"
                      @click="onLookupCompanyByNip"
                    >
                      {{ lookupPending ? 'Sprawdzanie...' : 'Pobierz z GUS' }}
                    </button>
                  </div>
                </div>
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Telefon</span>
                <input
                  v-model="form.telephone"
                  type="text"
                  inputmode="tel"
                  placeholder="np. 600 700 800"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <div
                v-if="lookupError"
                class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 md:col-span-2"
              >
                {{ lookupError }}
              </div>

              <div
                v-else-if="lookupSuccess"
                class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 md:col-span-2"
              >
                {{ lookupSuccess }}
              </div>

              <label class="block space-y-2 md:col-span-2">
                <span class="text-sm font-medium text-slate-700">Ulica</span>
                <input
                  v-model="form.street"
                  type="text"
                  placeholder="np. ul. Kwiatowa 12"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Kod pocztowy</span>
                <input
                  v-model="form.zipcode"
                  type="text"
                  inputmode="numeric"
                  placeholder="np. 00-001"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Miasto</span>
                <input
                  v-model="form.city"
                  type="text"
                  placeholder="np. Warszawa"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>
            </div>
          </div>

          <details
            class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
            :open="hasOptionalCompanyData"
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
                  <span class="text-sm font-medium text-slate-700">E-mail</span>
                  <input
                    v-model="form.email"
                    type="email"
                    placeholder="np. biuro@firma.pl"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Osoba kontaktowa</span>
                  <input
                    v-model="form.contactPerson"
                    type="text"
                    placeholder="np. Anna Kowalska"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>
              </div>

              <label class="mt-4 block space-y-2">
                <span class="text-sm font-medium text-slate-700">Notatka</span>
                <textarea
                  v-model="form.note"
                  rows="5"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                />
              </label>
            </div>
          </details>
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
                Nazwa
              </dt>
              <dd class="mt-1 font-medium text-slate-900">
                {{ trimmedName || 'Brak nazwy' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Kontakt
              </dt>
              <dd class="mt-1 text-slate-900">
                {{ optionalValue(form.contactPerson) || 'Brak osoby kontaktowej' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Telefon
              </dt>
              <dd class="mt-1 text-slate-900">
                {{ trimmedTelephone || 'Brak telefonu' }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-500">
                Adres
              </dt>
              <dd class="mt-1 text-slate-900">
                {{
                  [trimmedStreet, `${trimmedZipcode} ${trimmedCity}`.trim()]
                    .filter(Boolean)
                    .join(', ') || 'Brak adresu'
                }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
