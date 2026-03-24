<script setup lang="ts">
import type { CompanySummary, CourseSummary, StudentSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Nowe zaświadczenie'
})

function formatDateInput(date = new Date()) {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')

  return `${year}-${month}-${day}`
}

function formatStudentLabel(student: StudentSummary) {
  const base = `${student.lastName} ${student.firstName}`.trim()

  if (student.company) {
    return `${base} · ${student.company.name}`
  }

  return base
}

function formatCompanyLabel(company: Pick<CompanySummary, 'name' | 'city'>) {
  if (company.city) {
    return `${company.name} · ${company.city}`
  }

  return company.name
}

function formatCourseLabel(course: CourseSummary) {
  const title = course.mainName
    ? `${course.mainName} · ${course.name}`
    : course.name

  if (course.symbol) {
    return `${course.symbol} · ${title}`
  }

  return title
}

const api = useApi()
const route = useRoute()
const today = formatDateInput()

const initialRegistryYear = readQueryValue(route.query.registryYear) || `${new Date().getFullYear()}`
const initialCertificateDate = readQueryValue(route.query.certificateDate) || today
const initialCourseDateStart = readQueryValue(route.query.courseDateStart) || today
const initialCourseDateEnd = readQueryValue(route.query.courseDateEnd) || today

const form = reactive({
  studentSearch: '',
  courseSearch: '',
  studentId: null as number | null,
  courseId: null as number | null,
  registryYear: initialRegistryYear,
  registryNumber: '',
  certificateDate: initialCertificateDate,
  courseDateStart: initialCourseDateStart,
  courseDateEnd: initialCourseDateEnd
})

const createStudentForm = reactive({
  firstName: '',
  lastName: '',
  secondName: '',
  birthDate: today,
  birthPlace: '',
  pesel: '',
  telephone: '',
  addressStreet: '',
  addressCity: '',
  addressZip: '',
  companySearch: ''
})

const selectedCourse = ref<CourseSummary | null>(null)
const courseOptions = ref<CourseSummary[]>([])
const studentSearch = toRef(form, 'studentSearch')
const newStudentCompanySearch = toRef(createStudentForm, 'companySearch')
const {
  selectedOption: selectedStudent,
  options: studentOptions,
  pending: studentsPending,
  error: studentSearchError,
  normalizedQuery: normalizedStudentSearch,
  showNoResults: showNoStudentResults,
  initializeSelection: initializeStudentSelection,
  selectOption: selectStudentOption,
  clearSelection: clearStudentSearchSelection
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
const {
  selectedOption: selectedNewStudentCompany,
  options: newStudentCompanyOptions,
  pending: newStudentCompaniesPending,
  error: newStudentCompanySearchError,
  selectOption: selectNewStudentCompanyOption,
  clearSelection: clearNewStudentCompanySelection
} = useSearchableSelect<CompanySummary>({
  query: newStudentCompanySearch,
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
const coursesPending = ref(false)
const registryPending = ref(false)
const createStudentPending = ref(false)
const submitPending = ref(false)
const showCreateStudentForm = ref(false)

const errorMessage = ref('')
const courseSearchError = ref('')
const createStudentError = ref('')

let courseSearchTimer: ReturnType<typeof setTimeout> | undefined
let courseRequestId = 0

function readQueryValue(value: string | string[] | undefined) {
  if (Array.isArray(value)) {
    return value[0] ?? ''
  }

  return value ?? ''
}

async function fetchCourses(query: string) {
  const normalized = query.trim()
  if (normalized.length < 2) {
    courseOptions.value = []
    coursesPending.value = false
    courseSearchError.value = ''
    return
  }

  const requestId = ++courseRequestId
  coursesPending.value = true

  try {
    const response = await api.courses({
      search: normalized,
      limit: 20
    })

    if (requestId !== courseRequestId) {
      return
    }

    courseOptions.value = response.data
    courseSearchError.value = ''
  } catch (error) {
    if (requestId !== courseRequestId) {
      return
    }

    courseOptions.value = []
    courseSearchError.value = getApiErrorMessage(error, 'Nie udało się pobrać listy kursów.')
  } finally {
    if (requestId === courseRequestId) {
      coursesPending.value = false
    }
  }
}

async function refreshRegistryNumber() {
  if (!form.courseId) {
    form.registryNumber = ''
    return
  }

  const year = Number.parseInt(form.registryYear, 10)
  if (!Number.isFinite(year) || year <= 0) {
    form.registryNumber = ''
    return
  }

  registryPending.value = true

  try {
    const response = await api.nextRegistryNumber({
      courseId: form.courseId,
      year
    })
    form.registryNumber = `${response.data.nextNumber}`
  } catch (error) {
    form.registryNumber = ''
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się pobrać kolejnego numeru.')
  } finally {
    registryPending.value = false
  }
}

function selectStudent(student: StudentSummary) {
  selectStudentOption(student)
  form.studentId = student.id
  createStudentError.value = ''
  showCreateStudentForm.value = false
}

function selectCourse(course: CourseSummary) {
  selectedCourse.value = course
  form.courseId = course.id
  form.courseSearch = formatCourseLabel(course)
  courseOptions.value = []
  errorMessage.value = ''
}

function clearStudentSelection() {
  clearStudentSearchSelection()
  form.studentId = null
}

function clearCourseSelection() {
  selectedCourse.value = null
  form.courseId = null
  form.courseSearch = ''
  form.registryNumber = ''
  courseOptions.value = []
}

const preselectedStudentId = Number.parseInt(readQueryValue(route.query.studentId), 10)
const preselectedFirstName = readQueryValue(route.query.firstName)
const preselectedLastName = readQueryValue(route.query.lastName)
const preselectedCompanyName = readQueryValue(route.query.companyName)
const preselectedCourseId = Number.parseInt(readQueryValue(route.query.courseId), 10)
const preselectedCourseName = readQueryValue(route.query.courseName)
const preselectedCourseSymbol = readQueryValue(route.query.courseSymbol)
const preselectedCourseMainName = readQueryValue(route.query.courseMainName)
const preselectedCourseExpiryTime = readQueryValue(route.query.courseExpiryTime)

if (Number.isFinite(preselectedStudentId) && preselectedStudentId > 0 && preselectedLastName && preselectedFirstName) {
  initializeStudentSelection({
    id: preselectedStudentId,
    firstName: preselectedFirstName,
    lastName: preselectedLastName,
    pesel: null,
    birthDate: '',
    company: preselectedCompanyName
      ? {
          id: 0,
          name: preselectedCompanyName
        }
      : null
  })

  form.studentId = preselectedStudentId
}

if (Number.isFinite(preselectedCourseId) && preselectedCourseId > 0 && preselectedCourseName) {
  selectedCourse.value = {
    id: preselectedCourseId,
    name: preselectedCourseName,
    symbol: preselectedCourseSymbol,
    mainName: preselectedCourseMainName,
    expiryTime: preselectedCourseExpiryTime || null
  }

  form.courseId = preselectedCourseId
  form.courseSearch = formatCourseLabel(selectedCourse.value)
}

watch(() => form.studentSearch, () => {
  if (!selectedStudent.value) {
    form.studentId = null
  }
})

watch(() => form.courseSearch, (value) => {
  if (selectedCourse.value && value !== formatCourseLabel(selectedCourse.value)) {
    selectedCourse.value = null
    form.courseId = null
    form.registryNumber = ''
  }

  if (courseSearchTimer) {
    clearTimeout(courseSearchTimer)
  }

  if (selectedCourse.value && value === formatCourseLabel(selectedCourse.value)) {
    courseOptions.value = []
    coursesPending.value = false
    return
  }

  courseSearchTimer = setTimeout(() => {
    void fetchCourses(value)
  }, 250)
})

watch([
  () => form.courseId,
  () => form.registryYear
], async () => {
  if (!form.courseId) {
    form.registryNumber = ''
    return
  }

  await refreshRegistryNumber()
})

onBeforeUnmount(() => {
  if (courseSearchTimer) {
    clearTimeout(courseSearchTimer)
  }
})

const certificateNumberPreview = computed(() => {
  if (!form.registryNumber || !selectedCourse.value || !form.registryYear) {
    return 'Uzupełnij kurs i rok rejestru'
  }

  return `${form.registryNumber}/${selectedCourse.value.symbol}/${form.registryYear}`
})

const studentStepComplete = computed(() => !!selectedStudent.value && !!form.studentId)
const courseStepComplete = computed(() => !!selectedCourse.value && !!form.courseId)

const selectedCourseDescription = computed(() => {
  if (!selectedCourse.value) {
    return 'Nie wybrano kursu'
  }

  if (!selectedCourse.value.expiryTime) {
    return 'Brak ustawionego okresu ważności'
  }

  return `Ważność: ${selectedCourse.value.expiryTime} lat`
})

const newCourseLink = computed(() => {
  return {
    path: '/courses/new',
    query: {
      returnTo: 'certificate',
      studentId: selectedStudent.value?.id ?? undefined,
      firstName: selectedStudent.value?.firstName ?? undefined,
      lastName: selectedStudent.value?.lastName ?? undefined,
      companyName: selectedStudent.value?.company?.name ?? undefined,
      certificateDate: form.certificateDate || undefined,
      courseDateStart: form.courseDateStart || undefined,
      courseDateEnd: form.courseDateEnd || undefined,
      registryYear: form.registryYear || undefined
    }
  }
})

const trimmedCreateStudentFirstName = computed(() => createStudentForm.firstName.trim())
const trimmedCreateStudentLastName = computed(() => createStudentForm.lastName.trim())
const trimmedCreateStudentBirthDate = computed(() => createStudentForm.birthDate.trim())
const trimmedCreateStudentBirthPlace = computed(() => createStudentForm.birthPlace.trim())
const createStudentHasOptionalData = computed(() => {
  return Boolean(
    createStudentForm.secondName.trim()
    || createStudentForm.pesel.trim()
    || createStudentForm.telephone.trim()
    || createStudentForm.addressStreet.trim()
    || createStudentForm.addressCity.trim()
    || createStudentForm.addressZip.trim()
    || createStudentForm.companySearch.trim()
  )
})

function resetCreateStudentForm() {
  createStudentForm.firstName = ''
  createStudentForm.lastName = ''
  createStudentForm.secondName = ''
  createStudentForm.birthDate = today
  createStudentForm.birthPlace = ''
  createStudentForm.pesel = ''
  createStudentForm.telephone = ''
  createStudentForm.addressStreet = ''
  createStudentForm.addressCity = ''
  createStudentForm.addressZip = ''
  clearNewStudentCompanySelection()
  createStudentError.value = ''
}

function openCreateStudentForm() {
  showCreateStudentForm.value = true
  createStudentError.value = ''

  const searchValue = normalizedStudentSearch.value
  if (!trimmedCreateStudentLastName.value && searchValue) {
    createStudentForm.lastName = searchValue
  }
}

function selectNewStudentCompany(company: CompanySummary) {
  selectNewStudentCompanyOption(company)
}

async function onCreateStudent() {
  createStudentError.value = ''

  if (
    !trimmedCreateStudentFirstName.value
    || !trimmedCreateStudentLastName.value
    || !trimmedCreateStudentBirthDate.value
    || !trimmedCreateStudentBirthPlace.value
  ) {
    createStudentError.value = 'Uzupełnij wszystkie wymagane pola nowego kursanta.'
    return
  }

  createStudentPending.value = true

  try {
    const response = await api.createStudent({
      firstName: trimmedCreateStudentFirstName.value,
      lastName: trimmedCreateStudentLastName.value,
      secondName: optionalValue(createStudentForm.secondName),
      birthDate: trimmedCreateStudentBirthDate.value,
      birthPlace: trimmedCreateStudentBirthPlace.value,
      pesel: optionalValue(createStudentForm.pesel),
      telephone: optionalValue(createStudentForm.telephone),
      addressStreet: optionalValue(createStudentForm.addressStreet),
      addressCity: optionalValue(createStudentForm.addressCity),
      addressZip: optionalValue(createStudentForm.addressZip),
      companyId: selectedNewStudentCompany.value?.id ?? null
    })

    const student = response.data
    selectStudent({
      id: student.id,
      firstName: student.firstName,
      lastName: student.lastName,
      birthDate: student.birthDate,
      pesel: student.pesel,
      company: student.company
    })
    resetCreateStudentForm()
  } catch (error) {
    createStudentError.value = getApiErrorMessage(error, 'Nie udało się utworzyć kursanta.')
  } finally {
    createStudentPending.value = false
  }
}

async function onSubmit() {
  errorMessage.value = ''

  if (!selectedStudent.value || !form.studentId) {
    errorMessage.value = 'Wybierz kursanta z listy.'
    return
  }

  if (!selectedCourse.value || !form.courseId) {
    errorMessage.value = 'Wybierz kurs z listy.'
    return
  }

  const registryYear = Number.parseInt(form.registryYear, 10)
  const registryNumber = Number.parseInt(form.registryNumber, 10)

  if (!Number.isFinite(registryYear) || registryYear <= 0) {
    errorMessage.value = 'Podaj poprawny rok rejestru.'
    return
  }

  if (!Number.isFinite(registryNumber) || registryNumber <= 0) {
    errorMessage.value = 'Podaj poprawny numer rejestru.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createCertificate({
      studentId: form.studentId,
      courseId: form.courseId,
      certificateDate: form.certificateDate,
      courseDateStart: form.courseDateStart,
      courseDateEnd: form.courseDateEnd || null,
      registryYear,
      registryNumber
    })

    await navigateTo(`/certificates/${response.data.id}`)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się utworzyć zaświadczenia.')
  } finally {
    submitPending.value = false
  }
}
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-lg border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Zaświadczenia
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Nowe zaświadczenie
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Wybierz kursanta i kurs, ustaw daty oraz numer rejestru, a następnie zapisz nowe
          zaświadczenie w obecnej bazie.
        </p>

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="studentStepComplete
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ studentStepComplete ? 'Kursant wybrany' : 'Wybierz kursanta' }}
          </span>
          <span
            class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
            :class="courseStepComplete
              ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
              : 'border-slate-200 bg-white text-slate-500'"
          >
            {{ courseStepComplete ? 'Kurs wybrany' : 'Wybierz kurs' }}
          </span>
          <span class="inline-flex items-center justify-center rounded-full border border-sky-200 bg-sky-50 px-3 py-1 text-xs font-medium text-sky-700">
            Numer: {{ certificateNumberPreview }}
          </span>
        </div>
      </div>

      <NuxtLink
        to="/"
        class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
      >
        Powrót do dashboardu
      </NuxtLink>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <form
        class="space-y-6"
        @submit.prevent="onSubmit"
      >
        <section class="rounded-lg border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="mb-5 flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium uppercase tracking-[0.16em] text-sky-700">
                Krok 1
              </p>
              <h2 class="mt-1 text-xl font-semibold text-slate-900">
                Wybór kursanta
              </h2>
            </div>

            <button
              v-if="selectedStudent"
              type="button"
              class="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-600 transition hover:border-slate-400 hover:text-slate-900"
              @click="clearStudentSelection"
            >
              Zmień
            </button>
          </div>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Szukaj kursanta</span>
            <input
              v-model="form.studentSearch"
              type="text"
              autocomplete="off"
              placeholder="Nazwisko, imię lub PESEL"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            >
          </label>

          <p class="mt-2 text-xs text-slate-500">
            Wpisz co najmniej 2 znaki, aby wyszukać kursanta.
          </p>

          <div
            v-if="studentSearchError"
            class="mt-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ studentSearchError }}
          </div>

          <div
            v-if="studentsPending"
            class="mt-4 rounded-md border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-500"
          >
            Wyszukiwanie kursantów...
          </div>

          <div
            v-else-if="studentOptions.length > 0"
            class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
          >
            <button
              v-for="student in studentOptions"
              :key="student.id"
              type="button"
              class="flex w-full items-start justify-between gap-4 border-b border-slate-200 px-4 py-3 text-left transition last:border-b-0 hover:bg-slate-50"
              @click="selectStudent(student)"
            >
              <div>
                <p class="font-medium text-slate-900">
                  {{ student.lastName }} {{ student.firstName }}
                </p>
                <p class="mt-1 text-sm text-slate-500">
                  {{ student.company?.name ?? 'Brak firmy' }}
                </p>
              </div>

              <p
                v-if="student.pesel"
                class="text-sm text-slate-400"
              >
                {{ student.pesel }}
              </p>
            </button>
          </div>

          <div
            v-else-if="showNoStudentResults"
            class="mt-4 rounded-md border border-dashed border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-500"
          >
            Nie znaleziono kursanta dla podanej frazy. Możesz dodać go od razu bez opuszczania tego formularza.
            <div class="mt-3">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-md border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
                @click="openCreateStudentForm"
              >
                Dodaj nowego kursanta
              </button>
            </div>
          </div>

          <div
            v-if="selectedStudent"
            class="mt-4 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4"
          >
            <p class="text-sm font-medium text-emerald-800">
              Wybrany kursant
            </p>
            <p class="mt-2 text-base font-semibold text-slate-900">
              {{ selectedStudent.lastName }} {{ selectedStudent.firstName }}
            </p>
            <p class="mt-1 text-sm text-slate-600">
              {{ selectedStudent.company?.name ?? 'Brak przypisanej firmy' }}
            </p>
          </div>

          <div class="mt-4 flex justify-end">
            <button
              v-if="!showCreateStudentForm"
              type="button"
              class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              @click="openCreateStudentForm"
            >
              Nowy kursant
            </button>
          </div>

          <section
            v-if="showCreateStudentForm"
            class="mt-4 rounded-lg border border-slate-200 bg-slate-50/80 p-5"
          >
            <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-base font-semibold text-slate-900">
                  Szybkie dodanie kursanta
                </h3>
                <p class="mt-1 text-sm text-slate-500">
                  Po zapisaniu nowy kursant zostanie od razu wybrany do zaświadczenia.
                </p>
              </div>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                @click="showCreateStudentForm = false"
              >
                Zamknij
              </button>
            </div>

            <div
              v-if="createStudentError"
              class="mt-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              {{ createStudentError }}
            </div>

            <div class="mt-4 rounded-md border border-slate-200 bg-white/90 p-4">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h4 class="text-sm font-semibold text-slate-900">
                    Dane wymagane
                  </h4>
                  <p class="mt-1 text-xs leading-5 text-slate-500">
                    Wystarczą do szybkiego utworzenia kursanta i przypisania go do zaświadczenia.
                  </p>
                </div>

                <span class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-500">
                  4 pola
                </span>
              </div>

              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Imię</span>
                  <input
                    v-model="createStudentForm.firstName"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Nazwisko</span>
                  <input
                    v-model="createStudentForm.lastName"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Data urodzenia</span>
                  <input
                    v-model="createStudentForm.birthDate"
                    type="date"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium text-slate-700">Miejsce urodzenia</span>
                  <input
                    v-model="createStudentForm.birthPlace"
                    type="text"
                    class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  >
                </label>
              </div>
            </div>

            <details
              class="mt-4 overflow-hidden rounded-md border border-slate-200 bg-white"
              :open="createStudentHasOptionalData"
            >
              <summary class="cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-700 marker:hidden">
                <span class="flex items-center justify-between gap-3">
                  <span>Dane dodatkowe i firma</span>
                  <span class="text-xs text-slate-400">opcjonalne</span>
                </span>
              </summary>

              <div class="border-t border-slate-200 px-4 py-4">
                <div class="grid gap-4 md:grid-cols-2">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Drugie imię</span>
                    <input
                      v-model="createStudentForm.secondName"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">PESEL</span>
                    <input
                      v-model="createStudentForm.pesel"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2 md:col-span-2">
                    <span class="text-sm font-medium text-slate-700">Telefon</span>
                    <input
                      v-model="createStudentForm.telephone"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2 md:col-span-2">
                    <span class="text-sm font-medium text-slate-700">Ulica</span>
                    <input
                      v-model="createStudentForm.addressStreet"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Kod pocztowy</span>
                    <input
                      v-model="createStudentForm.addressZip"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Miasto</span>
                    <input
                      v-model="createStudentForm.addressCity"
                      type="text"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>
                </div>

                <div class="mt-5 space-y-3">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Przypisz firmę</span>
                    <input
                      v-model="createStudentForm.companySearch"
                      type="text"
                      placeholder="Wpisz co najmniej 2 znaki, aby wyszukać firmę"
                      class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    >
                  </label>

                  <div
                    v-if="newStudentCompanySearchError"
                    class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
                  >
                    {{ newStudentCompanySearchError }}
                  </div>

                  <div
                    v-if="selectedNewStudentCompany"
                    class="flex items-center justify-between rounded-md border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800"
                  >
                    <div>
                      <p class="font-medium">
                        {{ selectedNewStudentCompany.name }}
                      </p>
                      <p
                        v-if="selectedNewStudentCompany.city"
                        class="text-xs text-sky-700"
                      >
                        {{ selectedNewStudentCompany.city }}
                      </p>
                    </div>

                    <button
                      type="button"
                      class="rounded-md border border-sky-200 bg-white px-3 py-1.5 text-xs font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
                      @click="clearNewStudentCompanySelection"
                    >
                      Usuń wybór
                    </button>
                  </div>

                  <div
                    v-if="newStudentCompaniesPending"
                    class="rounded-md border border-slate-200 bg-white px-4 py-3 text-sm text-slate-500"
                  >
                    Szukanie firm...
                  </div>

                  <div
                    v-else-if="newStudentCompanyOptions.length"
                    class="overflow-hidden rounded-md border border-slate-200 bg-white"
                  >
                    <button
                      v-for="company in newStudentCompanyOptions"
                      :key="company.id"
                      type="button"
                      class="flex w-full items-start justify-between gap-4 border-b border-slate-200 px-4 py-3 text-left transition last:border-b-0 hover:bg-slate-50"
                      @click="selectNewStudentCompany(company)"
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
              </div>
            </details>

            <div class="mt-4 flex flex-wrap justify-end gap-3">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="createStudentPending"
                @click="resetCreateStudentForm"
              >
                Wyczyść dane kursanta
              </button>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-md bg-slate-950 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="createStudentPending"
                @click="onCreateStudent"
              >
                {{ createStudentPending ? 'Tworzenie kursanta...' : 'Utwórz i wybierz kursanta' }}
              </button>
            </div>
          </section>
        </section>

        <section class="rounded-lg border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="mb-5 flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium uppercase tracking-[0.16em] text-sky-700">
                Krok 2
              </p>
              <h2 class="mt-1 text-xl font-semibold text-slate-900">
                Wybór kursu
              </h2>
            </div>

            <button
              v-if="selectedCourse"
              type="button"
              class="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-600 transition hover:border-slate-400 hover:text-slate-900"
              @click="clearCourseSelection"
            >
              Zmień
            </button>
          </div>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Szukaj kursu</span>
            <input
              v-model="form.courseSearch"
              type="text"
              autocomplete="off"
              placeholder="Symbol, nazwa lub grupa kursu"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            >
          </label>

          <p class="mt-2 text-xs text-slate-500">
            Wpisz co najmniej 2 znaki, aby wyszukać kurs.
          </p>

          <div
            v-if="courseSearchError"
            class="mt-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ courseSearchError }}
          </div>

          <div
            v-if="coursesPending"
            class="mt-4 rounded-md border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-500"
          >
            Wyszukiwanie kursów...
          </div>

          <div
            v-else-if="courseOptions.length > 0"
            class="mt-4 max-h-80 overflow-y-auto rounded-md border border-slate-200 bg-white"
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
                  {{ course.symbol }}
                </p>
                <p class="mt-1 text-sm text-slate-500">
                  {{ course.mainName ? `${course.mainName} · ${course.name}` : course.name }}
                </p>
              </div>

              <p class="text-sm text-slate-400">
                {{ course.expiryTime ? `${course.expiryTime} lat` : 'bez terminu' }}
              </p>
            </button>
          </div>

          <div
            v-else-if="form.courseSearch.trim().length >= 2 && !selectedCourse"
            class="mt-4 rounded-md border border-dashed border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-500"
          >
            Nie znaleziono kursu dla podanej frazy. Możesz dodać nowy kurs w osobnym ekranie i wrócić tutaj z zachowanym formularzem.
            <div class="mt-3 flex flex-wrap items-center gap-3">
              <NuxtLink
                :to="newCourseLink"
                class="inline-flex items-center justify-center rounded-md border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
              >
                Dodaj nowy kurs
              </NuxtLink>
              <span class="text-xs text-slate-400">
                Zachowamy wybranego kursanta i daty.
              </span>
            </div>
          </div>

          <div
            v-if="selectedCourse"
            class="mt-4 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4"
          >
            <p class="text-sm font-medium text-emerald-800">
              Wybrany kurs
            </p>
            <p class="mt-2 text-base font-semibold text-slate-900">
              {{ selectedCourse.symbol }}
            </p>
            <p class="mt-1 text-sm text-slate-600">
              {{ selectedCourse.mainName ? `${selectedCourse.mainName} · ${selectedCourse.name}` : selectedCourse.name }}
            </p>
          </div>

          <div class="mt-4 flex justify-end">
            <div class="flex flex-col items-end gap-2">
              <NuxtLink
                :to="newCourseLink"
                class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              >
                Nowy kurs
              </NuxtLink>
              <p class="text-xs text-slate-400">
                Otworzy się osobny formularz, a po zapisie wrócisz tutaj.
              </p>
            </div>
          </div>
        </section>

        <section class="rounded-lg border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="mb-5">
            <p class="text-sm font-medium uppercase tracking-[0.16em] text-sky-700">
              Krok 3
            </p>
            <h2 class="mt-1 text-xl font-semibold text-slate-900">
              Dane zaświadczenia
            </h2>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Rok rejestru</span>
              <input
                v-model="form.registryYear"
                type="number"
                min="2000"
                step="1"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Numer rejestru</span>
              <div class="relative">
                <input
                  v-model="form.registryNumber"
                  type="number"
                  min="1"
                  step="1"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 pr-12 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
                <span
                  v-if="registryPending"
                  class="pointer-events-none absolute inset-y-0 right-4 inline-flex items-center text-xs text-slate-400"
                >
                  ...
                </span>
              </div>
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data zaświadczenia</span>
              <input
                v-model="form.certificateDate"
                type="date"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data rozpoczęcia kursu</span>
              <input
                v-model="form.courseDateStart"
                type="date"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2 md:col-span-2">
              <span class="text-sm font-medium text-slate-700">Data zakończenia kursu</span>
              <input
                v-model="form.courseDateEnd"
                type="date"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
          </div>

          <div class="mt-4 flex flex-wrap items-center gap-3">
            <button
              type="button"
              class="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-600 transition hover:border-slate-400 hover:text-slate-900"
              @click="refreshRegistryNumber"
            >
              Pobierz kolejny numer
            </button>

            <p class="text-sm text-slate-500">
              Numer zostanie pobrany automatycznie po wyborze kursu i roku.
            </p>
          </div>
        </section>

        <div
          v-if="errorMessage"
          class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          {{ errorMessage }}
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <button
            type="submit"
            :disabled="submitPending"
            class="inline-flex items-center justify-center rounded-md bg-sky-600 px-5 py-3 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-sky-300"
          >
            {{ submitPending ? 'Zapisywanie...' : 'Zapisz zaświadczenie' }}
          </button>

          <span class="text-sm text-slate-500">
            Po zapisaniu numer rejestru zostanie odświeżony.
          </span>
        </div>
      </form>

      <aside class="space-y-6">
        <section class="rounded-lg border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.18em] text-sky-300">
            Numer zaświadczenia
          </p>
          <p class="mt-4 text-3xl font-semibold tracking-tight">
            {{ certificateNumberPreview }}
          </p>
          <p class="mt-3 text-sm leading-6 text-slate-300">
            Podgląd numeru na podstawie wybranego kursu oraz bieżącego roku rejestru.
          </p>
        </section>

        <section class="rounded-lg border border-slate-200 bg-white/90 p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-slate-900">
            Podsumowanie
          </h2>

          <dl class="mt-5 space-y-4">
            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kursant
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ selectedStudent ? `${selectedStudent.lastName} ${selectedStudent.firstName}` : 'Nie wybrano kursanta' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Firma
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ selectedStudent?.company?.name ?? 'Brak firmy' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Kurs
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ selectedCourse ? `${selectedCourse.symbol} · ${selectedCourse.name}` : 'Nie wybrano kursu' }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Ważność
              </dt>
              <dd class="mt-1 text-sm text-slate-900">
                {{ selectedCourseDescription }}
              </dd>
            </div>

            <div>
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                Daty
              </dt>
              <dd class="mt-1 text-sm leading-6 text-slate-900">
                Od {{ form.courseDateStart || '—' }} do {{ form.courseDateEnd || '—' }}<br>
                Zaświadczenie: {{ form.certificateDate || '—' }}
              </dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>
  </section>
</template>
