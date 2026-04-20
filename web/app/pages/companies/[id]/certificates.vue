<script setup lang="ts">
import type { CertificateSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

const companyId = computed(() => Number.parseInt(`${route.params.id}`, 10))
const pageSize = 10
const currentPage = ref(1)
const dateFrom = ref('')
const dateTo = ref('')

if (!Number.isFinite(companyId.value) || companyId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono firmy'
  })
}

function formatDate(value: string | null) {
  if (!value) {
    return ''
  }

  const [year, month, day] = value.split('-')
  if (!year || !month || !day) {
    return value
  }

  return `${day}.${month}.${year}`
}

function certificateNumber(certificate: Pick<CertificateSummary, 'registryNumber' | 'courseSymbol' | 'registryYear'>) {
  return `${certificate.registryNumber}/${certificate.courseSymbol}/${certificate.registryYear}`
}

function buildPageNumbers(current: number, total: number) {
  if (total <= 1) {
    return [1]
  }

  const windowSize = 5
  const halfWindow = Math.floor(windowSize / 2)
  let start = Math.max(1, current - halfWindow)
  const end = Math.min(total, start + windowSize - 1)

  start = Math.max(1, end - windowSize + 1)

  return Array.from({ length: end - start + 1 }, (_, index) => start + index)
}

const { data, pending, error, refresh } = await useAsyncData(
  `company:${companyId.value}`,
  async () => await api.company(companyId.value)
)

const {
  data: certificatesData,
  pending: certificatesPending,
  error: certificatesError,
  refresh: refreshCertificates
} = await useAsyncData(
  `company-certificates:${companyId.value}`,
  async () => await api.companyCertificates(companyId.value, {
    page: currentPage.value,
    limit: pageSize,
    dateFrom: dateFrom.value || undefined,
    dateTo: dateTo.value || undefined
  }),
  {
    watch: [currentPage, dateFrom, dateTo]
  }
)

const company = computed(() => data.value?.data ?? null)
const certificates = computed(() => certificatesData.value?.data ?? [])
const pagination = computed(() => {
  return certificatesData.value?.pagination ?? {
    page: currentPage.value,
    limit: pageSize,
    total: 0,
    totalPages: 1
  }
})
const pageNumbers = computed(() => buildPageNumbers(currentPage.value, Math.max(1, pagination.value.totalPages)))
const certificatesRangeLabel = computed(() => {
  if (pagination.value.total === 0) {
    return ''
  }

  const start = (pagination.value.page - 1) * pagination.value.limit + 1
  const end = Math.min(pagination.value.page * pagination.value.limit, pagination.value.total)

  return `${start}-${end} z ${pagination.value.total}`
})
const hasActiveFilters = computed(() => !!(dateFrom.value || dateTo.value))

watch([dateFrom, dateTo], () => {
  currentPage.value = 1
})

watch(
  () => pagination.value.totalPages,
  (totalPages) => {
    const normalizedTotalPages = Math.max(1, totalPages)

    if (currentPage.value > normalizedTotalPages) {
      currentPage.value = normalizedTotalPages
    }
  }
)

async function refreshAll() {
  await Promise.all([refresh(), refreshCertificates()])
}

function clearFilters() {
  dateFrom.value = ''
  dateTo.value = ''
}

function goToPage(page: number) {
  if (page < 1 || page > Math.max(1, pagination.value.totalPages) || page === currentPage.value) {
    return
  }

  currentPage.value = page
}

useSeoMeta({
  title: () => (company.value ? `Zaświadczenia: ${company.value.name}` : 'Zaświadczenia firmy')
})
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Firmy</p>
        <h1 class="wrap-break-word text-3xl font-semibold tracking-tight text-slate-900">
          {{ company?.name || 'Zaświadczenia firmy' }}
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Historia wystawionych zaświadczeń powiązanych z tą firmą, także dla kursantów już odpiętych od klienta.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || certificatesPending"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          :to="`/companies/${companyId}`"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Szczegóły firmy
        </NuxtLink>

        <NuxtLink
          :to="{
            path: '/students/new',
            query: {
              companyId,
              companyName: company?.name
            }
          }"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Dodaj kursanta
        </NuxtLink>
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
      Ładowanie szczegółów firmy...
    </div>

    <template v-else>
      <div class="grid gap-4 md:grid-cols-3">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">NIP</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ company.nip }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Miasto</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ company.city }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">Łącznie</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight">
            {{ pagination.total }}
          </p>
        </div>
      </div>

      <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <div class="grid gap-4 border-b border-slate-200 pb-5 xl:grid-cols-[repeat(2,minmax(0,1fr))_auto]">
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

          <div class="flex items-end">
            <button
              type="button"
              class="inline-flex w-full items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-3 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 xl:w-auto"
              :disabled="!hasActiveFilters"
              @click="clearFilters"
            >
              Wyczyść filtry
            </button>
          </div>
        </div>

        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-slate-900">Wystawione zaświadczenia</h2>
            <p class="mt-6 text-sm text-slate-500">
              Strona {{ pagination.page }} z {{ Math.max(1, pagination.totalPages) }}.
            </p>
          </div>

          <p class="text-sm text-slate-500">
            {{ certificatesRangeLabel || 'Brak wpisów do wyświetlenia.' }}
          </p>
        </div>

        <div
          v-if="certificatesError"
          class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          Nie udało się pobrać listy zaświadczeń dla tej firmy.
        </div>

        <div
          v-else-if="certificatesPending"
          class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500"
        >
          Ładowanie historii zaświadczeń...
        </div>

        <div
          v-else-if="!certificates.length"
          class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
        >
          Brak wystawionych zaświadczeń dla tej firmy.
        </div>

        <div v-else class="mt-5 space-y-3">
          <NuxtLink
            v-for="certificate in certificates"
            :key="certificate.id"
            :to="`/certificates/${certificate.id}`"
            class="block rounded-lg border border-slate-200 bg-white px-4 py-4 transition hover:border-sky-200 hover:bg-sky-50/50"
          >
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    class="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold uppercase tracking-[0.16em] text-slate-600"
                  >
                    {{ formatDate(certificate.date) }}
                  </span>

                  <span
                    class="rounded-md bg-sky-100 px-2 py-1 text-xs font-semibold uppercase tracking-[0.16em] text-sky-700"
                  >
                    {{ certificate.courseSymbol }}
                  </span>
                </div>

                <div>
                  <p class="text-base font-semibold text-slate-900">
                    {{ certificate.studentName }}
                  </p>
                  <p class="mt-1 text-sm text-slate-500">
                    {{ certificate.courseName }}
                  </p>
                  <p class="mt-1 text-sm text-slate-500">
                    Numer: {{ certificateNumber(certificate) }}
                  </p>
                </div>
              </div>

              <dl class="grid gap-3 text-sm text-slate-600 sm:grid-cols-2 lg:min-w-[18rem]">
                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Kurs od</dt>
                  <dd class="mt-1 font-medium text-slate-900">
                    {{ formatDate(certificate.courseDateStart) }}
                  </dd>
                </div>

                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Kurs do</dt>
                  <dd class="mt-1 font-medium text-slate-900">
                    {{ formatDate(certificate.courseDateEnd) || 'Brak' }}
                  </dd>
                </div>

                <div class="sm:col-span-2">
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Ważność</dt>
                  <dd class="mt-1 font-medium text-slate-900">
                    {{ formatDate(certificate.expiryDate) || 'Brak terminu ważności' }}
                  </dd>
                </div>
              </dl>
            </div>
          </NuxtLink>

          <div
            v-if="pagination.totalPages > 1"
            class="flex flex-col gap-3 border-t border-slate-200 pt-4 sm:flex-row sm:items-center sm:justify-between"
          >
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="currentPage === 1"
                @click="goToPage(currentPage - 1)"
              >
                Poprzednia
              </button>

              <button
                v-for="pageNumber in pageNumbers"
                :key="pageNumber"
                type="button"
                class="inline-flex h-10 min-w-10 items-center justify-center rounded-lg border px-3 text-sm font-medium transition"
                :class="
                  currentPage === pageNumber
                    ? 'border-sky-600 bg-sky-600 text-white shadow-sm'
                    : 'border-slate-300 bg-white text-slate-700 hover:border-slate-400 hover:text-slate-900'
                "
                @click="goToPage(pageNumber)"
              >
                {{ pageNumber }}
              </button>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="currentPage >= pagination.totalPages"
                @click="goToPage(currentPage + 1)"
              >
                Następna
              </button>
            </div>
          </div>
        </div>
      </section>
    </template>
  </section>
</template>
