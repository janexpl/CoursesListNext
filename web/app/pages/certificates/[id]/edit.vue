<script setup lang="ts">
import type { StudentSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

function formatStudentLabel(student: StudentSummary) {
  const base = `${student.lastName} ${student.firstName}`.trim()

  if (student.company) {
    return `${base} · ${student.company.name}`
  }

  return base
}

function formatPolishDate(value: string | null) {
  if (!value) {
    return ''
  }

  const [year, month, day] = value.split('-')
  if (!year || !month || !day) {
    return value
  }

  return `${day}.${month}.${year}`
}

const route = useRoute()
const api = useApi()

const certificateId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(certificateId.value) || certificateId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono zaświadczenia'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `certificate-edit:${certificateId.value}`,
  async () => await api.certificate(certificateId.value)
)

const certificate = computed(() => data.value?.data ?? null)
const certificateDetailsLink = computed(() => `/certificates/${certificateId.value}`)

const form = reactive({
  studentSearch: '',
  certificateDate: '',
  courseDateStart: '',
  courseDateEnd: ''
})

const submitPending = ref(false)
const errorMessage = ref('')
const studentSearch = toRef(form, 'studentSearch')
const {
  selectedOption: selectedStudent,
  options: studentOptions,
  pending: studentsPending,
  error: studentSearchError,
  showNoResults: showNoStudentResults,
  initializeSelection: initializeStudentSelection,
  selectOption: selectStudent,
  clearSelection: clearStudentSelection
} = useSearchableSelect<StudentSummary>({
  query: studentSearch,
  fetchOptions: async (search) => {
    const response = await api.students({
      search,
      limit: 8
    })

    return response.data
  },
  getOptionLabel: formatStudentLabel,
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy kursantów.')
})
const isInitialized = ref(false)

watchEffect(() => {
  if (!certificate.value || isInitialized.value) {
    return
  }

  form.certificateDate = certificate.value.date || ''
  form.courseDateStart = certificate.value.courseDateStart || ''
  form.courseDateEnd = certificate.value.courseDateEnd || ''

  initializeStudentSelection({
    id: certificate.value.studentId,
    firstName: certificate.value.studentName,
    lastName: certificate.value.studentLastname,
    pesel: certificate.value.studentPesel || null,
    birthDate: certificate.value.studentBirthdate,
    company: null
  })
  isInitialized.value = true
})

const certificateNumber = computed(() => {
  if (!certificate.value) {
    return ''
  }

  return `${certificate.value.registryNumber}/${certificate.value.courseSymbol}/${certificate.value.registryYear}`
})

const currentStudentFullName = computed(() => {
  return [
    selectedStudent.value?.lastName,
    selectedStudent.value?.firstName
  ]
    .filter(Boolean)
    .join(' ')
})

const trimmedCertificateDate = computed(() => form.certificateDate.trim())
const trimmedCourseDateStart = computed(() => form.courseDateStart.trim())
const trimmedCourseDateEnd = computed(() => form.courseDateEnd.trim())

async function onSubmit() {
  errorMessage.value = ''

  if (!selectedStudent.value || !trimmedCertificateDate.value || !trimmedCourseDateStart.value) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  submitPending.value = true

  try {
    await api.updateCertificate(certificateId.value, {
      studentId: selectedStudent.value.id,
      certificateDate: trimmedCertificateDate.value,
      courseDateStart: trimmedCourseDateStart.value,
      courseDateEnd: trimmedCourseDateEnd.value || null
    })

    await navigateTo(certificateDetailsLink.value)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać zmian zaświadczenia.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: () => certificateNumber.value ? `Edycja: ${certificateNumber.value}` : 'Edycja zaświadczenia'
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Zaświadczenia
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Edycja zaświadczenia
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Możesz skorygować kursanta i daty. Kurs oraz numer rejestru pozostają na tym etapie tylko do podglądu.
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
          :to="certificateDetailsLink"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Anuluj
        </NuxtLink>

        <button
          form="certificate-edit-form"
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
      Nie udało się pobrać danych zaświadczenia.
    </div>

    <div
      v-else-if="pending || !certificate"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie formularza edycji...
    </div>

    <div
      v-else
      class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]"
    >
      <form
        id="certificate-edit-form"
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
              Kursant
            </h2>
            <p class="text-sm text-slate-500">
              Wyszukaj osobę, dla której wpis ma pozostać przypisany.
            </p>
          </div>

          <div class="mt-5 space-y-4">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Szukaj kursanta</span>
              <input
                v-model="form.studentSearch"
                type="text"
                placeholder="Minimum 2 znaki"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <div
              v-if="studentSearchError"
              class="rounded-lg border border-red-200 bg-red-50 px-4 py-4 text-sm text-red-700"
            >
              {{ studentSearchError }}
            </div>

            <div
              v-if="selectedStudent"
              class="rounded-lg border border-sky-200 bg-sky-50 px-4 py-4"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-slate-900">
                    {{ currentStudentFullName }}
                  </p>
                  <p class="text-sm text-slate-600">
                    {{ selectedStudent.company?.name ?? 'Brak przypisanej firmy' }}
                  </p>
                </div>

                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  @click="clearStudentSelection"
                >
                  Zmień wybór
                </button>
              </div>
            </div>

            <div
              v-else-if="studentsPending"
              class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-500"
            >
              Szukanie kursantów...
            </div>

            <div
              v-else-if="form.studentSearch.trim().length >= 2"
              class="rounded-lg border border-slate-200 bg-white"
            >
              <button
                v-for="studentOption in studentOptions"
                :key="studentOption.id"
                type="button"
                class="flex w-full items-center justify-between gap-4 border-b border-slate-100 px-4 py-3 text-left transition last:border-b-0 hover:bg-sky-50"
                @click="selectStudent(studentOption)"
              >
                <div>
                  <p class="text-sm font-medium text-slate-900">
                    {{ studentOption.lastName }} {{ studentOption.firstName }}
                  </p>
                  <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    {{ studentOption.company?.name ?? 'Brak firmy' }}
                  </p>
                </div>

                <span class="text-xs font-semibold uppercase tracking-[0.16em] text-sky-700">
                  Wybierz
                </span>
              </button>

              <div
                v-if="showNoStudentResults"
                class="px-4 py-4 text-sm text-slate-500"
              >
                Brak kursantów pasujących do podanej frazy.
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h2 class="text-lg font-semibold text-slate-900">
              Daty
            </h2>
            <p class="text-sm text-slate-500">
              Daty wpisu i zakres szkolenia używane przy wyliczaniu ważności.
            </p>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-3">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data wystawienia</span>
              <input
                v-model="form.certificateDate"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data rozpoczęcia kursu</span>
              <input
                v-model="form.courseDateStart"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data zakończenia kursu</span>
              <input
                v-model="form.courseDateEnd"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
          </div>
        </section>
      </form>

      <aside class="space-y-6">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-slate-900">
            Stałe dane wpisu
          </h2>

          <dl class="mt-5 space-y-4">
            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Numer
              </dt>
              <dd class="mt-1 text-sm font-semibold text-slate-900">
                {{ certificateNumber }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kurs
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ certificate.courseSymbol }} · {{ certificate.courseName }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Ważność
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ formatPolishDate(certificate.expiryDate) || 'Brak terminu ważności' }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
