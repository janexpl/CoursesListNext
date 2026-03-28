<script setup lang="ts">
import CourseCertificateTranslationsEditor from '~/components/courses/CourseCertificateTranslationsEditor.vue'
import type {
  CourseCertificateTranslationForm,
  CourseCertificateTranslationProgramRow
} from '~/utils/courseCertificateTranslations'
import {
  buildCourseCertificateTranslationPayloads,
  countReadyCourseCertificateTranslations,
  getCourseCertificateTranslationsValidationError,
  parseCourseCertificateTranslationProgramRows
} from '~/utils/courseCertificateTranslations'

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

type CourseProgramRow = {
  id: number
  subject: string
  theoryTime: string
  practiceTime: string
}

type EditTab = 'general' | 'program' | 'translations' | 'template'
type TemplatePlaceholder = {
  label: string
  value: string
}
type FontSizeOption = {
  label: string
  value: string
}

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

const courseId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(courseId.value) || courseId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono kursu'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `course-edit:${courseId.value}`,
  async () => await api.course(courseId.value)
)

const course = computed(() => data.value?.data ?? null)

const form = reactive({
  mainName: '',
  name: '',
  symbol: '',
  expiryTime: '',
  certFrontPage: ''
})

const isInitialized = ref(false)
const submitPending = ref(false)
const errorMessage = ref('')
const initialSnapshot = ref('')
const activeTab = ref<EditTab>('general')
const showTemplateSource = ref(false)
const programRows = ref<CourseProgramRow[]>([])
const translationForms = ref<CourseCertificateTranslationForm[]>([])
const hasInvalidStoredProgram = ref(false)
const templateEditorRef = ref<HTMLDivElement | null>(null)
let programRowSequence = 0
let translationProgramRowSequence = 0
let translationSequence = 0
let templateEditorSyncInProgress = false

const templatePlaceholders: TemplatePlaceholder[] = [
  { label: 'Imię', value: '{{ imie }}' },
  { label: 'Drugie imię', value: '{{ drugie_imie }}' },
  { label: 'Nazwisko', value: '{{ nazwisko }}' },
  { label: 'Data urodzenia', value: '{{ data_urodzenia }}' },
  { label: 'Nazwa kursu', value: '{{ nazwa_kursu }}' },
  { label: 'Data rozpoczęcia', value: '{{ data_rozpoczecia }}' },
  { label: 'Data zakończenia', value: '{{ data_zakonczenia }}' },
  { label: 'Data wystawienia', value: '{{ data_wystawienia }}' },
  { label: 'Numer zaświadczenia', value: '{{ numer_zaswiadczenia }}' }
]

const fontSizeOptions: FontSizeOption[] = [
  { label: 'Mała', value: '14px' },
  { label: 'Normalna', value: '16px' },
  { label: 'Duża', value: '20px' },
  { label: 'Bardzo duża', value: '28px' }
]

function createProgramRow(values: Partial<Omit<CourseProgramRow, 'id'>> = {}): CourseProgramRow {
  programRowSequence += 1

  return {
    id: programRowSequence,
    subject: values.subject ?? '',
    theoryTime: values.theoryTime ?? '',
    practiceTime: values.practiceTime ?? ''
  }
}

function createTranslationProgramRow(values: Partial<Omit<CourseCertificateTranslationProgramRow, 'id'>> = {}): CourseCertificateTranslationProgramRow {
  translationProgramRowSequence += 1

  return {
    id: translationProgramRowSequence,
    subject: values.subject ?? '',
    theoryTime: values.theoryTime ?? '',
    practiceTime: values.practiceTime ?? ''
  }
}

function createTranslationForm(
  values: Partial<Omit<CourseCertificateTranslationForm, 'id' | 'programRows' | 'hasInvalidStoredProgram'>> & {
    programRows?: CourseCertificateTranslationProgramRow[]
    hasInvalidStoredProgram?: boolean
  } = {}
): CourseCertificateTranslationForm {
  translationSequence += 1

  return {
    id: translationSequence,
    languageCode: values.languageCode ?? '',
    courseName: values.courseName ?? '',
    certFrontPage: values.certFrontPage ?? '',
    programRows: values.programRows ?? [createTranslationProgramRow()],
    hasInvalidStoredProgram: values.hasInvalidStoredProgram ?? false
  }
}

function parseProgramRows(value: string) {
  if (!value.trim()) {
    return {
      rows: [] as CourseProgramRow[],
      invalid: false
    }
  }

  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) {
      return {
        rows: [] as CourseProgramRow[],
        invalid: true
      }
    }

    return {
      rows: parsed.map((entry: CourseProgramEntry) => createProgramRow({
        subject: entry.Subject ?? '',
        theoryTime: entry.TheoryTime ?? '',
        practiceTime: entry.PracticeTime ?? ''
      })),
      invalid: false
    }
  } catch {
    return {
      rows: [] as CourseProgramRow[],
      invalid: true
    }
  }
}

function normalizeHours(value: string) {
  return value.trim().replace(',', '.')
}

function isValidHoursValue(value: string) {
  return /^\d+(\.\d+)?$/.test(value)
}

function addProgramRow() {
  programRows.value.push(createProgramRow())
}

function removeProgramRow(index: number) {
  if (programRows.value.length === 1) {
    programRows.value = [createProgramRow()]
    return
  }

  programRows.value.splice(index, 1)
}

function moveProgramRow(index: number, direction: -1 | 1) {
  const targetIndex = index + direction
  if (targetIndex < 0 || targetIndex >= programRows.value.length) {
    return
  }

  const [row] = programRows.value.splice(index, 1)
  if (!row) {
    return
  }
  programRows.value.splice(targetIndex, 0, row)
}

function syncTemplateEditorFromForm() {
  const editor = templateEditorRef.value
  if (!editor) {
    return
  }

  if (editor.innerHTML === form.certFrontPage) {
    return
  }

  templateEditorSyncInProgress = true
  editor.innerHTML = form.certFrontPage
  templateEditorSyncInProgress = false
}

function onTemplateEditorInput() {
  if (templateEditorSyncInProgress || !templateEditorRef.value) {
    return
  }

  form.certFrontPage = templateEditorRef.value.innerHTML
}

function focusTemplateEditor() {
  templateEditorRef.value?.focus()
}

function runTemplateCommand(command: string, value?: string) {
  focusTemplateEditor()
  document.execCommand(command, false, value)
  onTemplateEditorInput()
}

function insertTemplatePlaceholder(placeholder: string) {
  focusTemplateEditor()
  document.execCommand('insertText', false, placeholder)
  onTemplateEditorInput()
}

function applyTemplateFontSize(fontSize: string) {
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
    focusTemplateEditor()
    return
  }

  const range = selection.getRangeAt(0)
  const selectedContent = range.extractContents()
  const span = document.createElement('span')
  span.style.fontSize = fontSize
  span.appendChild(selectedContent)
  range.insertNode(span)

  selection.removeAllRanges()
  const updatedRange = document.createRange()
  updatedRange.selectNodeContents(span)
  selection.addRange(updatedRange)

  onTemplateEditorInput()
}

watch(() => form.certFrontPage, async () => {
  if (activeTab.value !== 'template') {
    return
  }

  await nextTick()
  syncTemplateEditorFromForm()
})

watch(activeTab, async (value) => {
  if (value !== 'template') {
    return
  }

  await nextTick()
  syncTemplateEditorFromForm()
})

onMounted(async () => {
  await nextTick()
  syncTemplateEditorFromForm()
})

const trimmedMainName = computed(() => form.mainName.trim())
const trimmedName = computed(() => form.name.trim())
const trimmedSymbol = computed(() => form.symbol.trim())
const trimmedExpiryTime = computed(() => form.expiryTime.trim())

const normalizedProgramRows = computed(() => {
  return programRows.value
    .map((row) => {
      return {
        subject: row.subject.trim(),
        theoryTime: normalizeHours(row.theoryTime) || '0',
        practiceTime: normalizeHours(row.practiceTime) || '0'
      }
    })
    .filter(row => row.subject || row.theoryTime !== '0' || row.practiceTime !== '0')
})

const hasInvalidCourseProgram = computed(() => {
  return normalizedProgramRows.value.some((row) => {
    return !row.subject || !isValidHoursValue(row.theoryTime) || !isValidHoursValue(row.practiceTime)
  })
})

const courseProgramEntries = computed(() => {
  return normalizedProgramRows.value
    .filter(row => row.subject)
    .map((row) => {
      return {
        Subject: row.subject,
        TheoryTime: row.theoryTime,
        PracticeTime: row.practiceTime
      }
    }) as CourseProgramEntry[]
})

const serializedCourseProgram = computed(() => {
  return JSON.stringify(courseProgramEntries.value)
})

const translationValidationMessage = computed(() => {
  return getCourseCertificateTranslationsValidationError(translationForms.value)
})

const translationsReady = computed(() => {
  return translationForms.value.length === 0 || !translationValidationMessage.value
})

const readyTranslationCount = computed(() => {
  return countReadyCourseCertificateTranslations(translationForms.value)
})

function buildSnapshot() {
  return JSON.stringify({
    mainName: trimmedMainName.value,
    name: trimmedName.value,
    symbol: trimmedSymbol.value,
    expiryTime: trimmedExpiryTime.value,
    certFrontPage: form.certFrontPage,
    courseProgram: serializedCourseProgram.value,
    certificateTranslations: buildCourseCertificateTranslationPayloads(translationForms.value)
  })
}

function applyCourseToForm() {
  if (!course.value) {
    return
  }

  form.mainName = course.value.mainName || ''
  form.name = course.value.name || ''
  form.symbol = course.value.symbol || ''
  form.expiryTime = course.value.expiryTime || ''
  form.certFrontPage = course.value.certFrontPage || ''
  const parsedProgram = parseProgramRows(course.value.courseProgram || '')
  hasInvalidStoredProgram.value = parsedProgram.invalid
  programRows.value = parsedProgram.rows.length ? parsedProgram.rows : [createProgramRow()]
  translationForms.value = (course.value.certificateTranslations || []).map((translation) => {
    const parsedTranslationProgram = parseCourseCertificateTranslationProgramRows(
      translation.courseProgram || '',
      createTranslationProgramRow
    )

    return createTranslationForm({
      languageCode: translation.languageCode,
      courseName: translation.courseName,
      certFrontPage: translation.certFrontPage,
      programRows: parsedTranslationProgram.rows.length ? parsedTranslationProgram.rows : [createTranslationProgramRow()],
      hasInvalidStoredProgram: parsedTranslationProgram.invalid
    })
  })
  initialSnapshot.value = buildSnapshot()
  errorMessage.value = ''
  showTemplateSource.value = false
  isInitialized.value = true
}

const isDirty = computed(() => {
  if (!isInitialized.value) {
    return false
  }

  return buildSnapshot() !== initialSnapshot.value
})

const canSubmit = computed(() => {
  return !submitPending.value && !pending.value && !error.value && !!course.value && isDirty.value
})

useUnsavedChangesWarning(() => isDirty.value && !submitPending.value)

watchEffect(() => {
  if (!course.value || isInitialized.value) {
    return
  }

  applyCourseToForm()
})

const programTotals = computed(() => {
  return courseProgramEntries.value.reduce((acc, entry) => {
    acc.theory += Number.parseFloat(entry.TheoryTime ?? '0') || 0
    acc.practice += Number.parseFloat(entry.PracticeTime ?? '0') || 0
    return acc
  }, {
    theory: 0,
    practice: 0
  })
})

function formatExpiryLabel(value: string) {
  const numericValue = Number.parseInt(value, 10)
  if (!Number.isFinite(numericValue)) {
    return value
  }

  if (numericValue === 1) {
    return '1 rok'
  }

  return `${numericValue} lat`
}

async function onRefresh() {
  if (isDirty.value && !window.confirm('Masz niezapisane zmiany. Odświeżyć dane kursu z serwera?')) {
    return
  }

  await refresh()
  applyCourseToForm()
}

function resetForm() {
  applyCourseToForm()
}

const certFrontPageDocument = computed(() => {
  if (!form.certFrontPage.trim()) {
    return ''
  }

  return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
      :root {
        color-scheme: light;
      }

      @page {
        size: A4 portrait;
        margin: 0;
      }

      html {
        background: #f8fafc;
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        padding: 20px;
        background:
          radial-gradient(circle at top left, rgb(14 165 233 / 0.10), transparent 25%),
          linear-gradient(180deg, #e2e8f0 0%, #f8fafc 100%);
        color: #0f172a;
        font-family: "Times New Roman", "Liberation Serif", Georgia, serif;
        line-height: 1.4;
      }

      .certificate-sheet {
        width: min(210mm, 100%);
        min-height: 297mm;
        margin: 0 auto;
        padding: 16mm 14mm;
        border: 1px solid #cbd5e1;
        border-radius: 8px;
        background: white;
        box-shadow:
          0 30px 70px rgb(15 23 42 / 0.10),
          0 10px 24px rgb(15 23 42 / 0.08);
      }

      .certificate-sheet > :first-child {
        margin-top: 0 !important;
      }

      .certificate-sheet > :last-child {
        margin-bottom: 0 !important;
      }

      h1, h2, h3, h4, h5, h6 {
        margin: 0 0 0.45rem;
        line-height: 1.2;
        color: #020617;
      }

      h1 {
        font-size: 32px;
        font-weight: 700;
      }

      h2 {
        font-size: 24px;
        font-weight: 700;
      }

      h3 {
        font-size: 18px;
        font-weight: 700;
      }

      p {
        margin: 0 0 0.4rem;
        font-size: 15px;
        line-height: 1.45;
      }

      ul, ol {
        margin: 0 0 0.45rem;
        padding-left: 1.25rem;
      }

      img {
        max-width: 100%;
        height: auto;
      }
    </style>
  </head>
  <body>
    <div class="certificate-sheet">
      ${form.certFrontPage}
    </div>
  </body>
</html>`
})

async function onSubmit() {
  errorMessage.value = ''

  if (!trimmedMainName.value || !trimmedName.value || !trimmedSymbol.value || !trimmedExpiryTime.value || !form.certFrontPage.trim()) {
    errorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  if (!/^\d+$/.test(trimmedExpiryTime.value)) {
    errorMessage.value = 'Okres ważności musi być dodatnią liczbą całkowitą.'
    return
  }

  if (!courseProgramEntries.value.length) {
    errorMessage.value = 'Dodaj przynajmniej jeden temat programu kursu.'
    return
  }

  if (hasInvalidCourseProgram.value) {
    errorMessage.value = 'Uzupełnij temat oraz popraw godziny teorii i praktyki w programie kursu.'
    return
  }

  if (translationValidationMessage.value) {
    errorMessage.value = translationValidationMessage.value
    activeTab.value = 'translations'
    return
  }

  submitPending.value = true

  try {
    await api.updateCourse(courseId.value, {
      mainName: trimmedMainName.value,
      name: trimmedName.value,
      symbol: trimmedSymbol.value,
      expiryTime: trimmedExpiryTime.value,
      courseProgram: serializedCourseProgram.value,
      certFrontPage: form.certFrontPage,
      certificateTranslations: buildCourseCertificateTranslationPayloads(translationForms.value)
    })

    await navigateTo(`/courses/${courseId.value}`)
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać zmian kursu.')
  } finally {
    submitPending.value = false
  }
}

useSeoMeta({
  title: () => course.value ? `Edycja: ${course.value.name}` : 'Edycja kursu'
})
</script>

<template>
  <section class="space-y-8">
    <div class="sticky top-4 z-20 flex flex-col gap-4 rounded-xl border border-white/60 bg-white/90 p-6 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursy
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Edycja kursu
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Aktualizuj nazwę, symbol, program oraz szablon zaświadczenia dla wybranego kursu.
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

        <span
          class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
          :class="translationsReady
            ? 'border-sky-200 bg-sky-50 text-sky-700'
            : 'border-amber-200 bg-amber-50 text-amber-700'"
        >
          {{ translationForms.length
            ? `Wersje językowe ${readyTranslationCount}/${translationForms.length}`
            : 'Brak wersji obcojęzycznych' }}
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
            :to="`/courses/${courseId}`"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Anuluj
          </NuxtLink>

          <button
            type="submit"
            form="course-edit-form"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-70"
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
      Nie udało się pobrać danych kursu.
    </div>

    <div
      v-else-if="pending || !course"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie formularza edycji...
    </div>

    <div
      v-else
      class="space-y-6"
    >
      <nav class="sticky top-28 z-10 flex flex-wrap gap-2 rounded-xl border border-slate-200 bg-white/90 p-2 shadow-sm backdrop-blur">
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition"
          :class="activeTab === 'general'
            ? 'bg-sky-600 text-white shadow-sm'
            : 'text-slate-700 hover:bg-slate-100'"
          @click="activeTab = 'general'"
        >
          Ogólne
        </button>
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition"
          :class="activeTab === 'program'
            ? 'bg-sky-600 text-white shadow-sm'
            : 'text-slate-700 hover:bg-slate-100'"
          @click="activeTab = 'program'"
        >
          Program
        </button>
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition"
          :class="activeTab === 'translations'
            ? 'bg-sky-600 text-white shadow-sm'
            : 'text-slate-700 hover:bg-slate-100'"
          @click="activeTab = 'translations'"
        >
          Języki
        </button>
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition"
          :class="activeTab === 'template'
            ? 'bg-sky-600 text-white shadow-sm'
            : 'text-slate-700 hover:bg-slate-100'"
          @click="activeTab = 'template'"
        >
          Szablon
        </button>
      </nav>

      <div
        v-if="errorMessage"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
      >
        {{ errorMessage }}
      </div>

      <form
        id="course-edit-form"
        class="space-y-6"
        @submit.prevent="onSubmit"
      >
        <div
          v-if="activeTab === 'general'"
          class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]"
        >
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="space-y-1">
              <h2 class="text-lg font-semibold text-slate-900">
                Podstawowe dane
              </h2>
              <p class="text-sm text-slate-500">
                Nazwa, symbol i okres ważności kursu.
              </p>
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Nazwa</span>
                <input
                  v-model="form.mainName"
                  type="text"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Nazwa szczegółowa</span>
                <input
                  v-model="form.name"
                  type="text"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Symbol</span>
                <input
                  v-model="form.symbol"
                  type="text"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 font-mono text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Ważność w latach</span>
                <input
                  v-model="form.expiryTime"
                  type="text"
                  inputmode="numeric"
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>
            </div>
          </section>

          <aside class="space-y-6">
            <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
              <h2 class="text-lg font-semibold text-slate-900">
                Podsumowanie
              </h2>

              <dl class="mt-5 space-y-4">
                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    Symbol
                  </dt>
                  <dd class="mt-1 break-all font-mono text-sm text-slate-900">
                    {{ trimmedSymbol || 'Brak' }}
                  </dd>
                </div>

                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    Nazwa
                  </dt>
                  <dd class="mt-1 text-sm text-slate-900">
                    {{ trimmedMainName || 'Brak' }}
                  </dd>
                </div>

                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    Nazwa szczegółowa
                  </dt>
                  <dd class="mt-1 text-sm text-slate-900">
                    {{ trimmedName || 'Brak' }}
                  </dd>
                </div>

                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    Ważność
                  </dt>
                  <dd class="mt-1 text-sm text-slate-900">
                    {{ trimmedExpiryTime ? formatExpiryLabel(trimmedExpiryTime) : 'Brak' }}
                  </dd>
                </div>

                <div>
                  <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                    Tematy programu
                  </dt>
                  <dd class="mt-1 text-sm text-slate-900">
                    {{ courseProgramEntries.length }}
                  </dd>
                </div>
              </dl>
            </section>
          </aside>
        </div>

        <div
          v-else-if="activeTab === 'program'"
          class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_28rem]"
        >
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div class="space-y-1">
                <h2 class="text-lg font-semibold text-slate-900">
                  Program kursu
                </h2>
                <p class="text-sm text-slate-500">
                  Ułóż tematy szkolenia i przypisz godziny bez edycji JSON-a.
                </p>
              </div>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                @click="addProgramRow()"
              >
                Dodaj temat
              </button>
            </div>

            <div class="mt-5 space-y-4">
              <article
                v-for="(row, index) in programRows"
                :key="row.id"
                class="rounded-lg border border-slate-200 bg-slate-50/80 p-4"
              >
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
                      Temat {{ index + 1 }}
                    </p>
                    <p class="mt-1 text-sm text-slate-500">
                      Uzupełnij temat oraz liczbę godzin teorii i praktyki.
                    </p>
                  </div>

                  <div class="flex items-center gap-2 self-start sm:self-auto">
                    <button
                      type="button"
                      class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
                      :disabled="index === 0"
                      @click="moveProgramRow(index, -1)"
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
                      :disabled="index === programRows.length - 1"
                      @click="moveProgramRow(index, 1)"
                    >
                      ↓
                    </button>
                    <button
                      type="button"
                      class="inline-flex h-9 items-center justify-center rounded-md border border-red-200 bg-red-50 px-3 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100"
                      @click="removeProgramRow(index)"
                    >
                      Usuń
                    </button>
                  </div>
                </div>

                <div class="mt-4 space-y-4">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium text-slate-700">Temat szkolenia</span>
                    <textarea
                      v-model="row.subject"
                      rows="5"
                      class="w-full resize-y rounded-md border border-slate-300 bg-white px-3 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                      placeholder="Np. Zasady bezpiecznej obsługi urządzenia"
                    />
                  </label>

                  <div class="grid gap-4 sm:grid-cols-2">
                    <label class="block space-y-2">
                      <span class="text-sm font-medium text-slate-700">Godziny teorii</span>
                      <input
                        v-model="row.theoryTime"
                        type="text"
                        inputmode="decimal"
                        class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                        placeholder="0"
                      >
                    </label>

                    <label class="block space-y-2">
                      <span class="text-sm font-medium text-slate-700">Godziny praktyki</span>
                      <input
                        v-model="row.practiceTime"
                        type="text"
                        inputmode="decimal"
                        class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                        placeholder="0"
                      >
                    </label>
                  </div>
                </div>
              </article>
            </div>

            <div
              v-if="hasInvalidStoredProgram"
              class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
            >
              W bazie zapisany był program w nieobsługiwanym formacie. Kreator załadował pustą listę tematów.
            </div>

            <div
              v-if="hasInvalidCourseProgram"
              class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
            >
              Każdy temat musi mieć nazwę, a godziny teorii i praktyki muszą być liczbami.
            </div>
          </section>

          <aside class="space-y-6">
            <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
              <h2 class="text-lg font-semibold text-slate-900">
                Podgląd programu
              </h2>

              <div
                v-if="courseProgramEntries.length"
                class="mt-5 overflow-hidden rounded-lg border border-slate-200"
              >
                <div class="overflow-x-auto">
                  <table class="min-w-full divide-y divide-slate-200 text-sm">
                    <thead class="bg-slate-50">
                      <tr class="text-left text-slate-600">
                        <th class="w-14 px-4 py-3 font-medium">
                          Lp.
                        </th>
                        <th class="px-4 py-3 font-medium">
                          Temat szkolenia
                        </th>
                        <th class="w-28 px-4 py-3 text-center font-medium">
                          Teoria
                        </th>
                        <th class="w-28 px-4 py-3 text-center font-medium">
                          Praktyka
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-200 bg-white">
                      <tr
                        v-for="(entry, index) in courseProgramEntries"
                        :key="`${entry.Subject || 'subject'}-${index}`"
                      >
                        <td class="px-4 py-3 text-slate-500">
                          {{ index + 1 }}
                        </td>
                        <td class="px-4 py-3 text-slate-900">
                          {{ entry.Subject || 'Brak tematu' }}
                        </td>
                        <td class="px-4 py-3 text-center text-slate-700">
                          {{ entry.TheoryTime || '0' }}
                        </td>
                        <td class="px-4 py-3 text-center text-slate-700">
                          {{ entry.PracticeTime || '0' }}
                        </td>
                      </tr>
                      <tr class="bg-slate-50 font-semibold text-slate-900">
                        <td
                          colspan="2"
                          class="px-4 py-3"
                        >
                          Razem
                        </td>
                        <td class="px-4 py-3 text-center">
                          {{ programTotals.theory.toFixed(1) }}
                        </td>
                        <td class="px-4 py-3 text-center">
                          {{ programTotals.practice.toFixed(1) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <div
                v-else
                class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
              >
                Brak poprawnego programu do podglądu.
              </div>
            </section>
          </aside>
        </div>

        <div
          v-else-if="activeTab === 'translations'"
          class="space-y-6"
        >
          <CourseCertificateTranslationsEditor
            v-model="translationForms"
            :disabled="submitPending || pending"
          />
        </div>

        <div
          v-else
          class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_28rem]"
        >
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="space-y-1">
              <h2 class="text-lg font-semibold text-slate-900">
                Szablon zaświadczenia
              </h2>
              <p class="text-sm text-slate-500">
                Edytuj wizualnie front zaświadczenia i wstawiaj placeholdery bez pisania kodu.
              </p>
            </div>

            <div class="mt-5 space-y-4">
              <div class="rounded-lg border border-slate-200 bg-slate-50 p-3">
                <div class="flex flex-wrap gap-2">
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('formatBlock', 'h1')"
                  >
                    H1
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('formatBlock', 'h2')"
                  >
                    H2
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('bold')"
                  >
                    B
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm italic text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('italic')"
                  >
                    I
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('justifyLeft')"
                  >
                    Lewo
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('justifyCenter')"
                  >
                    Środek
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('justifyRight')"
                  >
                    Prawo
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('insertUnorderedList')"
                  >
                    Lista
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="runTemplateCommand('removeFormat')"
                  >
                    Wyczyść styl
                  </button>
                </div>
              </div>

              <div class="rounded-lg border border-slate-200 bg-slate-50 p-3">
                <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
                  Wielkość czcionki
                </p>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button
                    v-for="option in fontSizeOptions"
                    :key="option.value"
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                    @mousedown.prevent
                    @click="applyTemplateFontSize(option.value)"
                  >
                    {{ option.label }}
                  </button>
                </div>
              </div>

              <div class="rounded-lg border border-slate-200 bg-slate-50 p-3">
                <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
                  Placeholdery
                </p>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button
                    v-for="placeholder in templatePlaceholders"
                    :key="placeholder.value"
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-sky-200 bg-sky-50 px-3 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
                    @mousedown.prevent
                    @click="insertTemplatePlaceholder(placeholder.value)"
                  >
                    {{ placeholder.label }}
                  </button>
                </div>
              </div>

              <ClientOnly>
                <div
                  ref="templateEditorRef"
                  contenteditable="true"
                  class="min-h-[28rem] rounded-lg border border-slate-300 bg-white px-5 py-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                  @input="onTemplateEditorInput"
                />
              </ClientOnly>

              <div class="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
                <p class="text-sm text-slate-500">
                  Możesz pracować wizualnie, a w razie potrzeby podejrzeć też surowy HTML.
                </p>
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  @click="showTemplateSource = !showTemplateSource"
                >
                  {{ showTemplateSource ? 'Ukryj HTML' : 'Pokaż HTML' }}
                </button>
              </div>

              <label
                v-if="showTemplateSource"
                class="block space-y-2"
              >
                <span class="text-sm font-medium text-slate-700">HTML szablonu</span>
                <textarea
                  v-model="form.certFrontPage"
                  rows="16"
                  class="w-full rounded-md border border-slate-300 bg-slate-950 px-4 py-3 font-mono text-sm leading-6 text-slate-100 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                />
              </label>
            </div>
          </section>

          <aside class="space-y-6">
            <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
              <h2 class="text-lg font-semibold text-slate-900">
                Podgląd szablonu
              </h2>

              <div
                v-if="certFrontPageDocument"
                class="mt-5 overflow-hidden rounded-lg border border-slate-200 bg-slate-50"
              >
                <iframe
                  title="Podgląd szablonu zaświadczenia"
                  :srcdoc="certFrontPageDocument"
                  class="h-[72rem] w-full bg-white"
                />
              </div>

              <div
                v-else
                class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
              >
                Brak szablonu do podglądu.
              </div>
            </section>
          </aside>
        </div>
      </form>
    </div>
  </section>
</template>
