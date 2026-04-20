<script setup lang="ts">
import AuditHistoryPanel from '~/components/audit/AuditHistoryPanel.vue'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()
const auth = useAuth()
const studentSearch = ref('')
const isAdmin = computed(() => auth.user.value?.role === 1)

const companyId = computed(() => Number.parseInt(`${route.params.id}`, 10))

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

const { data, pending, error, refresh } = await useAsyncData(
  `company:${companyId.value}`,
  async () => await api.company(companyId.value)
)

const {
  data: studentsData,
  pending: studentsPending,
  error: studentsError,
  refresh: refreshStudents
} = await useAsyncData(
  `company-students:${companyId.value}`,
  async () => await api.companyStudents(companyId.value)
)

const {
  data: auditData,
  pending: auditPending,
  error: auditError,
  refresh: refreshAudit
} = await useAsyncData(
  `company-audit:${companyId.value}`,
  async () => {
    if (!isAdmin.value) {
      return { data: [] }
    }

    return await api.companyAuditLog(companyId.value)
  },
  {
    watch: [isAdmin]
  }
)

const company = computed(() => data.value?.data ?? null)
const auditEntries = computed(() => auditData.value?.data ?? [])
const auditErrorMessage = computed(() => {
  return auditError.value ? getApiErrorMessage(auditError.value, 'Nie udało się pobrać historii zmian firmy.') : ''
})
const editCompanyLink = computed(() => `/companies/${companyId.value}/edit`)
const companyCertificatesLink = computed(() => `/companies/${companyId.value}/certificates`)
const students = computed(() => studentsData.value?.data ?? [])
const normalizedStudentSearch = computed(() => studentSearch.value.trim().toLocaleLowerCase())
const filteredStudents = computed(() => {
  if (!normalizedStudentSearch.value) {
    return students.value
  }

  return students.value.filter((student) => {
    const haystack = [
      student.lastname,
      student.firstname,
      student.secondname,
      student.pesel
    ]
      .filter(Boolean)
      .join(' ')
      .toLocaleLowerCase()

    return haystack.includes(normalizedStudentSearch.value)
  })
})

const companyAddress = computed(() => {
  if (!company.value) {
    return ''
  }

  return [company.value.street, `${company.value.zipcode} ${company.value.city}`]
    .filter(Boolean)
    .join(', ')
})

const refreshAll = async () => {
  await Promise.all([
    refresh(),
    refreshStudents(),
    isAdmin.value ? refreshAudit() : Promise.resolve()
  ])
}

const studentFullName = (student: {
  lastname: string
  firstname: string
  secondname: string | null
}) => [student.lastname, student.firstname, student.secondname].filter(Boolean).join(' ')

useSeoMeta({
  title: () => company.value?.name || 'Szczegół firmy'
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Firmy
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          {{ company?.name || 'Szczegół firmy' }}
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Dane firmy oraz lista kursantów przypisanych do tego klienta.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || studentsPending || (isAdmin && auditPending)"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/companies"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Lista firm
        </NuxtLink>

        <NuxtLink
          :to="editCompanyLink"
          class="inline-flex items-center justify-center rounded-lg bg-slate-950 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800"
        >
          Edytuj firmę
        </NuxtLink>

        <NuxtLink
          :to="companyCertificatesLink"
          class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
        >
          Wystawione zaświadczenia
        </NuxtLink>

        <NuxtLink
          :to="{
            path: '/students/new',
            query: {
              companyId: company?.id,
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
      <div class="grid gap-4 md:grid-cols-4">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            NIP
          </p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ company.nip }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            Miasto
          </p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ company.city }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            Kursanci
          </p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ students.length }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">
            Kontakt
          </p>
          <p class="mt-3 text-lg font-semibold tracking-tight">
            {{ company.contactPerson || 'Brak osoby kontaktowej' }}
          </p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">
              Dane firmy
            </h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Nazwa
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.name }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Telefon
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.telephone || 'Brak' }}
                </dd>
              </div>

              <div class="md:col-span-2">
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Adres
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ companyAddress }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  E-mail
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.email || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Osoba kontaktowa
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.contactPerson || 'Brak' }}
                </dd>
              </div>

              <div
                v-if="company.note"
                class="md:col-span-2"
              >
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Notatka
                </dt>
                <dd class="mt-1 text-sm leading-6 text-slate-900">
                  {{ company.note }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-slate-900">
                  Kursanci firmy
                </h2>
                <p class="mt-1 text-sm text-slate-500">
                  Aktualnie przypisane osoby. Historyczne zaświadczenia są dostępne osobno.
                </p>
              </div>

              <NuxtLink
                :to="companyCertificatesLink"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              >
                Historia zaświadczeń
              </NuxtLink>
            </div>

            <div class="mt-5 rounded-lg border border-slate-200 bg-slate-50 p-4">
              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Filtruj po nazwisku</span>
                <input
                  v-model="studentSearch"
                  type="text"
                  placeholder="Np. Nowak"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>
            </div>

            <div
              v-if="studentsError"
              class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              Nie udało się pobrać listy kursantów firmy.
            </div>

            <div
              v-else-if="studentsPending"
              class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Ładowanie listy kursantów...
            </div>

            <div
              v-else-if="!students.length"
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Brak kursantów przypisanych do tej firmy.
            </div>

            <div
              v-else-if="!filteredStudents.length"
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Brak kursantów pasujących do podanego nazwiska.
            </div>

            <div
              v-else
              class="mt-5 space-y-3"
            >
              <article
                v-for="student in filteredStudents"
                :key="student.id"
                class="flex flex-col gap-4 rounded-lg border border-slate-200 bg-white px-4 py-4 md:flex-row md:items-start md:justify-between"
              >
                <div class="space-y-2">
                  <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
                    <span>ID {{ student.id }}</span>
                    <span>•</span>
                    <span>{{ formatDate(student.birthdate) }}</span>
                  </div>

                  <NuxtLink
                    :to="`/students/${student.id}`"
                    class="inline-block text-base font-semibold text-slate-900 transition hover:text-sky-700"
                  >
                    {{ studentFullName(student) }}
                  </NuxtLink>

                  <div class="grid gap-2 text-sm text-slate-600 sm:grid-cols-2">
                    <p>
                      <span class="font-medium text-slate-700">PESEL:</span>
                      {{ student.pesel || 'Brak' }}
                    </p>
                    <p>
                      <span class="font-medium text-slate-700">Miejsce urodzenia:</span>
                      {{ student.birthplace }}
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
                        firstName: student.firstname,
                        lastName: student.lastname,
                        companyName: company.name
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
        </div>

        <aside class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">
              Metadane
            </h2>

            <dl class="mt-5 space-y-4">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  ID
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.id }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Miasto
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ company.city }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Kursanci
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ students.length }}
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
        title="Historia zmian firmy"
        description="Zmiany zapisane dla danych firmy i ich aktualizacji administracyjnych."
        empty-message="Brak wpisów historii zmian dla tej firmy."
      />
    </template>
  </section>
</template>
