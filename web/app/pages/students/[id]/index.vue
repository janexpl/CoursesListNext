<script setup lang="ts">
import AuditHistoryPanel from '~/components/audit/AuditHistoryPanel.vue'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()
const auth = useAuth()
const isAdmin = computed(() => auth.user.value?.role === 1)

const studentId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(studentId.value) || studentId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono kursanta'
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

const { data, pending, error, refresh } = await useAsyncData(
  `student:${studentId.value}`,
  async () => await api.student(studentId.value)
)

const {
  data: certificatesData,
  pending: certificatesPending,
  error: certificatesError,
  refresh: refreshCertificates
} = await useAsyncData(
  `student-certificates:${studentId.value}`,
  async () => await api.studentCertificates(studentId.value)
)

const {
  data: auditData,
  pending: auditPending,
  error: auditError,
  refresh: refreshAudit
} = await useAsyncData(
  `student-audit:${studentId.value}`,
  async () => {
    if (!isAdmin.value) {
      return { data: [] }
    }

    return await api.studentAuditLog(studentId.value)
  },
  {
    watch: [isAdmin]
  }
)

const student = computed(() => data.value?.data ?? null)
const certificates = computed(() => certificatesData.value?.data ?? [])
const auditEntries = computed(() => auditData.value?.data ?? [])
const auditErrorMessage = computed(() => {
  return auditError.value ? getApiErrorMessage(auditError.value, 'Nie udało się pobrać historii zmian kursanta.') : ''
})
const editStudentLink = computed(() => `/students/${studentId.value}/edit`)

const fullName = computed(() => {
  if (!student.value) {
    return ''
  }

  return [student.value.lastName, student.value.firstName, student.value.secondName]
    .filter(Boolean)
    .join(' ')
})

const fullAddress = computed(() => {
  if (!student.value) {
    return ''
  }

  return [
    student.value.addressStreet,
    [student.value.addressZip, student.value.addressCity].filter(Boolean).join(' ')
  ]
    .filter(Boolean)
    .join(', ')
})

const refreshAll = async () => {
  await Promise.all([
    refresh(),
    refreshCertificates(),
    isAdmin.value ? refreshAudit() : Promise.resolve()
  ])
}

const certificateNumber = (certificate: {
  registryNumber: number
  courseSymbol: string
  registryYear: number
}) => `${certificate.registryNumber}/${certificate.courseSymbol}/${certificate.registryYear}`

const certificateLink = computed(() => {
  if (!student.value) {
    return '/certificates/new'
  }

  return {
    path: '/certificates/new',
    query: {
      studentId: student.value.id,
      firstName: student.value.firstName,
      lastName: student.value.lastName,
      companyName: student.value.company?.name || undefined
    }
  }
})

useSeoMeta({
  title: () => fullName.value || 'Szczegół kursanta'
})
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Kursanci</p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          {{ fullName || 'Szczegół kursanta' }}
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Dane osobowe wybranego kursanta i szybkie przejście do wystawienia nowego zaświadczenia.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || certificatesPending || (isAdmin && auditPending)"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/students"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Lista kursantów
        </NuxtLink>

        <NuxtLink
          :to="editStudentLink"
          class="inline-flex items-center justify-center rounded-lg bg-slate-950 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800"
        >
          Edytuj kursanta
        </NuxtLink>

        <NuxtLink
          :to="certificateLink"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Wystaw zaświadczenie
        </NuxtLink>
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
      Ładowanie szczegółów kursanta...
    </div>

    <template v-else>
      <div class="grid gap-4 md:grid-cols-3">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Data urodzenia</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ formatDate(student.birthDate) }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">PESEL</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ student.pesel || 'Brak' }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">Firma</p>
          <p class="mt-3 text-lg font-semibold tracking-tight">
            {{ student.company?.name || 'Brak przypisanej firmy' }}
          </p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Dane osobowe</h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Imię i nazwisko</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ fullName }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Miejsce urodzenia
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ student.birthPlace }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Telefon</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ student.telephone || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Adres</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ fullAddress || 'Brak adresu' }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-slate-900">Historia zaświadczeń</h2>
                <p class="mt-1 text-sm text-slate-500">
                  Wystawione zaświadczenia powiązane z tym kursantem.
                </p>
              </div>

              <NuxtLink
                :to="certificateLink"
                class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
              >
                Wystaw kolejne
              </NuxtLink>
            </div>

            <div
              v-if="certificatesError"
              class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              Nie udało się pobrać historii zaświadczeń.
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
              Brak wystawionych zaświadczeń dla tego kursanta.
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
                      <p class="text-base font-semibold text-sm text-slate-900">
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
            </div>
          </section>
        </div>

        <aside class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Metadane</h2>

            <dl class="mt-5 space-y-4">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">ID</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ student.id }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Firma</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ student.company?.name || 'Brak' }}
                </dd>
              </div>
            </dl>
          </section>
        </aside>
      </div>

      <AuditHistoryPanel
        v-if="isAdmin"
        :entries="auditEntries"
        :pending="auditPending"
        :error-message="auditErrorMessage"
        title="Historia zmian kursanta"
        description="Zmiany zapisane dla danych kursanta i powiązań administracyjnych."
        empty-message="Brak wpisów historii zmian dla tego kursanta."
      />
    </template>
  </section>
</template>
