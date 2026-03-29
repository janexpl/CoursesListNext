<script setup lang="ts">
import type { GUSCompanyDetails } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

const companyId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(companyId.value) || companyId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono firmy'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `company-edit:${companyId.value}`,
  async () => await api.company(companyId.value)
)

const company = computed(() => data.value?.data ?? null)
const companyDetailsLink = computed(() => `/companies/${companyId.value}`)

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

const isInitialized = ref(false)
const submitPending = ref(false)
const errorMessage = ref('')
const lookupPending = ref(false)
const lookupError = ref('')
const lookupSuccess = ref('')
const initialSnapshot = ref('')

const trimmedName = computed(() => form.name.trim())
const trimmedStreet = computed(() => form.street.trim())
const trimmedCity = computed(() => form.city.trim())
const trimmedZipcode = computed(() => form.zipcode.trim())
const trimmedNip = computed(() => form.nip.trim())
const trimmedTelephone = computed(() => form.telephone.trim())
const normalizedNip = computed(() => form.nip.replaceAll(/\D/g, ''))

function optionalValue(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

function buildPayload() {
  return {
    name: trimmedName.value,
    street: trimmedStreet.value,
    city: trimmedCity.value,
    zipcode: trimmedZipcode.value,
    nip: trimmedNip.value,
    email: optionalValue(form.email),
    contactPerson: optionalValue(form.contactPerson),
    telephone: trimmedTelephone.value,
    note: optionalValue(form.note)
  }
}

function applyCompanyToForm() {
  if (!company.value) {
    return
  }

  form.name = company.value.name || ''
  form.street = company.value.street || ''
  form.city = company.value.city || ''
  form.zipcode = company.value.zipcode || ''
  form.nip = company.value.nip || ''
  form.email = company.value.email || ''
  form.contactPerson = company.value.contactPerson || ''
  form.telephone = company.value.telephone || ''
  form.note = company.value.note || ''
  initialSnapshot.value = JSON.stringify(buildPayload())
  errorMessage.value = ''
  lookupError.value = ''
  lookupSuccess.value = ''
  isInitialized.value = true
}

watchEffect(() => {
  if (!company.value || isInitialized.value) {
    return
  }

  applyCompanyToForm()
})

const isDirty = computed(() => {
  if (!isInitialized.value) {
    return false
  }

  return JSON.stringify(buildPayload()) !== initialSnapshot.value
})

const canSubmit = computed(() => {
  return !submitPending.value && !pending.value && isDirty.value && !error.value && !!company.value
})
const canLookupCompanyByNip = computed(() => !lookupPending.value && !submitPending.value && !pending.value && normalizedNip.value.length > 0)

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

watch(() => form.nip, () => {
  lookupError.value = ''
  lookupSuccess.value = ''
})

async function onRefresh() {
  if (isDirty.value && !window.confirm('Masz niezapisane zmiany. Odświeżyć dane firmy z serwera?')) {
    return
  }

  await refresh()
  applyCompanyToForm()
}

function resetForm() {
  applyCompanyToForm()
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
    lookupSuccess.value = `Zaktualizowano dane firmy ${response.data.name} na podstawie rejestru GUS.`
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
    await api.updateCompany(companyId.value, buildPayload())

    await navigateTo(companyDetailsLink.value)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać zmian firmy.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: () => company.value ? `Edycja: ${company.value.name}` : 'Edycja firmy'
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
          Edycja firmy
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Zaktualizuj dane kontaktowe, adresowe i organizacyjne wybranego klienta.
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
            :to="companyDetailsLink"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            form="company-edit-form"
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
      Nie udało się pobrać danych firmy.
    </div>

    <div
      v-else-if="pending || !company"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie formularza edycji...
    </div>

    <div
      v-else
      class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]"
    >
      <form
        id="company-edit-form"
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
              Podstawowe dane
            </h2>
            <p class="text-sm text-slate-500">
              Uzupełnij dane identyfikacyjne firmy i główny kontakt.
            </p>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
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
                    Możesz zaktualizować nazwę i adres firmy na podstawie rejestru GUS.
                  </p>

                  <button
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="!canLookupCompanyByNip"
                    @click="onLookupCompanyByNip"
                  >
                    {{ lookupPending ? 'Sprawdzanie...' : 'Aktualizuj z GUS' }}
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
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Adres
            </h2>
            <p class="text-sm text-slate-500">
              Dane adresowe używane w całym systemie i na widokach list.
            </p>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-[minmax(0,1fr)_10rem_12rem]">
            <label class="block space-y-2 md:col-span-3">
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

            <label class="block space-y-2 md:col-span-2">
              <span class="text-sm font-medium text-slate-700">Miasto</span>
              <input
                v-model="form.city"
                type="text"
                placeholder="np. Warszawa"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
          </div>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Notatka
            </h2>
            <p class="text-sm text-slate-500">
              Pole opcjonalne na dodatkowe informacje o firmie.
            </p>
          </div>

          <label class="mt-5 block space-y-2">
            <span class="text-sm font-medium text-slate-700">Treść notatki</span>
            <textarea
              v-model="form.note"
              rows="6"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            />
          </label>
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
                Nazwa
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ trimmedName || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                NIP
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ trimmedNip || 'Brak' }}
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
                {{ [trimmedStreet, `${trimmedZipcode} ${trimmedCity}`.trim()].filter(Boolean).join(', ') || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                E-mail
              </dt>
              <dd class="mt-1 break-all text-sm text-slate-900">
                {{ optionalValue(form.email) || 'Brak' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Osoba kontaktowa
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ optionalValue(form.contactPerson) || 'Brak' }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
