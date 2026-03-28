<script setup lang="ts">
import type {
  CourseCertificateTranslationForm,
  CourseCertificateTranslationProgramRow
} from '~/utils/courseCertificateTranslations'
import {
  buildCourseCertificateTranslationProgramEntries,
  countReadyCourseCertificateTranslations,
  hasInvalidCourseCertificateTranslationProgram,
  isCourseCertificateTranslationReady,
  supportedCourseCertificateTranslationLanguages
} from '~/utils/courseCertificateTranslations'

withDefaults(defineProps<{
  disabled?: boolean
}>(), {
  disabled: false
})

const translations = defineModel<CourseCertificateTranslationForm[]>({ required: true })

type TemplatePlaceholder = {
  label: string
  value: string
}

type FontSizeOption = {
  label: string
  value: string
}

const languagePresets = supportedCourseCertificateTranslationLanguages

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

const activeTranslationId = ref<number | null>(null)
const showTemplateSource = ref(false)
const templateEditorRef = ref<HTMLDivElement | null>(null)

let translationSequence = 0
let programRowSequence = 0
let templateEditorSyncInProgress = false

const activeTranslation = computed(() => {
  return translations.value.find(translation => translation.id === activeTranslationId.value) ?? null
})

const readyTranslationsCount = computed(() => {
  return countReadyCourseCertificateTranslations(translations.value)
})

function syncSequences() {
  translationSequence = translations.value.reduce((maxId, translation) => {
    return Math.max(maxId, translation.id)
  }, translationSequence)

  programRowSequence = translations.value.reduce((maxId, translation) => {
    return Math.max(maxId, ...translation.programRows.map(row => row.id), maxId)
  }, programRowSequence)
}

watch(translations, () => {
  syncSequences()

  if (!translations.value.length) {
    activeTranslationId.value = null
    return
  }

  if (!translations.value.some(translation => translation.id === activeTranslationId.value)) {
    activeTranslationId.value = translations.value[0]?.id ?? null
  }
}, { deep: true, immediate: true })

function createProgramRow(values: Partial<Omit<CourseCertificateTranslationProgramRow, 'id'>> = {}): CourseCertificateTranslationProgramRow {
  programRowSequence += 1

  return {
    id: programRowSequence,
    subject: values.subject ?? '',
    theoryTime: values.theoryTime ?? '',
    practiceTime: values.practiceTime ?? ''
  }
}

function addTranslation() {
  translationSequence += 1

  const translation: CourseCertificateTranslationForm = {
    id: translationSequence,
    languageCode: '',
    courseName: '',
    certFrontPage: '',
    programRows: [createProgramRow()],
    hasInvalidStoredProgram: false
  }

  translations.value = [...translations.value, translation]
  activeTranslationId.value = translation.id
}

function removeTranslation(translationId: number) {
  const currentIndex = translations.value.findIndex(translation => translation.id === translationId)
  if (currentIndex === -1) {
    return
  }

  const nextTranslations = translations.value.filter(translation => translation.id !== translationId)
  const fallback = nextTranslations[currentIndex] ?? nextTranslations[currentIndex - 1] ?? null

  translations.value = nextTranslations
  activeTranslationId.value = fallback?.id ?? null
}

function markProgramAsEdited(translation: CourseCertificateTranslationForm) {
  translation.hasInvalidStoredProgram = false
}

function addProgramRow(translation: CourseCertificateTranslationForm) {
  markProgramAsEdited(translation)
  translation.programRows.push(createProgramRow())
}

function removeProgramRow(translation: CourseCertificateTranslationForm, index: number) {
  markProgramAsEdited(translation)

  if (translation.programRows.length === 1) {
    translation.programRows = [createProgramRow()]
    return
  }

  translation.programRows.splice(index, 1)
}

function moveProgramRow(translation: CourseCertificateTranslationForm, index: number, direction: -1 | 1) {
  const targetIndex = index + direction
  if (targetIndex < 0 || targetIndex >= translation.programRows.length) {
    return
  }

  markProgramAsEdited(translation)
  const [row] = translation.programRows.splice(index, 1)
  if (!row) {
    return
  }
  translation.programRows.splice(targetIndex, 0, row)
}

function translationProgramEntries(translation: CourseCertificateTranslationForm) {
  return buildCourseCertificateTranslationProgramEntries(translation.programRows)
}

function translationHasInvalidProgram(translation: CourseCertificateTranslationForm) {
  return hasInvalidCourseCertificateTranslationProgram(translation.programRows)
}

function translationTotals(translation: CourseCertificateTranslationForm) {
  return translationProgramEntries(translation).reduce((acc, entry) => {
    acc.theory += Number.parseFloat(entry.TheoryTime ?? '0') || 0
    acc.practice += Number.parseFloat(entry.PracticeTime ?? '0') || 0
    return acc
  }, {
    theory: 0,
    practice: 0
  })
}

function getLanguageLabel(languageCode: string) {
  const normalized = languageCode.trim().toLowerCase()
  const match = languagePresets.find(language => language.code === normalized)
  if (!normalized) {
    return 'Nowa wersja'
  }

  return match ? `${normalized.toUpperCase()} - ${match.label}` : normalized.toUpperCase()
}

function syncTemplateEditorFromTranslation() {
  const editor = templateEditorRef.value
  const translation = activeTranslation.value
  if (!editor || !translation) {
    return
  }

  if (editor.innerHTML === translation.certFrontPage) {
    return
  }

  templateEditorSyncInProgress = true
  editor.innerHTML = translation.certFrontPage
  templateEditorSyncInProgress = false
}

function onTemplateEditorInput() {
  const translation = activeTranslation.value
  if (templateEditorSyncInProgress || !templateEditorRef.value || !translation) {
    return
  }

  translation.certFrontPage = templateEditorRef.value.innerHTML
}

function focusTemplateEditor() {
  templateEditorRef.value?.focus()
}

function runTemplateCommand(command: string, value?: string) {
  focusTemplateEditor()
  document.execCommand(command, false, value)
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

function insertTemplatePlaceholder(placeholder: string) {
  if (!activeTranslation.value) {
    return
  }

  focusTemplateEditor()
  document.execCommand('insertText', false, placeholder)
  onTemplateEditorInput()
}

watch(() => activeTranslation.value?.certFrontPage, async () => {
  await nextTick()
  syncTemplateEditorFromTranslation()
})

watch(activeTranslationId, async () => {
  await nextTick()
  syncTemplateEditorFromTranslation()
})

onMounted(async () => {
  await nextTick()
  syncTemplateEditorFromTranslation()
})

function buildTemplatePreviewDocument(html: string) {
  if (!html.trim()) {
    return ''
  }

  return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
      :root { color-scheme: light; }
      html { background: #f8fafc; }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        padding: 18px;
        background:
          radial-gradient(circle at top left, rgb(14 165 233 / 0.10), transparent 25%),
          linear-gradient(180deg, #e2e8f0 0%, #f8fafc 100%);
        color: #0f172a;
        font-family: "Times New Roman", "Liberation Serif", Georgia, serif;
        line-height: 1.45;
      }
      .certificate-sheet {
        min-height: 100%;
        margin: 0 auto;
        padding: 24px;
        border: 1px solid #cbd5e1;
        border-radius: 8px;
        background: white;
        box-shadow: 0 16px 40px rgb(15 23 42 / 0.08);
      }
      .certificate-sheet > :first-child { margin-top: 0 !important; }
      .certificate-sheet > :last-child { margin-bottom: 0 !important; }
      h1, h2, h3, h4, h5, h6 { margin: 0 0 0.45rem; line-height: 1.2; color: #020617; }
      h1 { font-size: 32px; font-weight: 700; }
      h2 { font-size: 24px; font-weight: 700; }
      h3 { font-size: 18px; font-weight: 700; }
      p { margin: 0 0 0.45rem; font-size: 15px; line-height: 1.45; }
      ul, ol { margin: 0 0 0.45rem; padding-left: 1.25rem; }
      img { max-width: 100%; height: auto; }
    </style>
  </head>
  <body>
    <div class="certificate-sheet">
      ${html}
    </div>
  </body>
</html>`
}
</script>

<template>
  <section class="space-y-6">
    <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div class="space-y-2">
          <div class="flex flex-wrap items-center gap-3">
            <h2 class="text-lg font-semibold text-slate-900">
              Wersje obcojęzyczne
            </h2>
            <span class="inline-flex items-center justify-center rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-600">
              {{ translations.length }} języków
            </span>
            <span
              class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
              :class="translations.length > 0 && readyTranslationsCount === translations.length
                ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
                : 'border-slate-200 bg-white text-slate-500'"
            >
              {{ translations.length > 0 ? `${readyTranslationsCount}/${translations.length} gotowych` : 'Sekcja opcjonalna' }}
            </span>
          </div>

          <p class="max-w-3xl text-sm leading-6 text-slate-500">
            Dodaj kompletne wersje zaświadczenia dla innych języków. Przy zapisie wysyłana jest cała aktualna lista.
          </p>
        </div>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-70"
          :disabled="disabled"
          @click="addTranslation"
        >
          Dodaj język
        </button>
      </div>

      <div
        v-if="!translations.length"
        class="mt-5 rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-500"
      >
        Nie masz jeszcze żadnej wersji obcojęzycznej. Dodaj język, aby przygotować osobny program i szablon zaświadczenia.
      </div>

      <div
        v-else
        class="mt-5 flex flex-wrap gap-2"
      >
        <button
          v-for="translation in translations"
          :key="translation.id"
          type="button"
          class="inline-flex items-center gap-2 rounded-full border px-4 py-2 text-sm font-medium transition"
          :class="activeTranslationId === translation.id
            ? 'border-sky-600 bg-sky-600 text-white shadow-sm'
            : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:bg-slate-50'"
          :disabled="disabled"
          @click="activeTranslationId = translation.id"
        >
          <span
            class="inline-flex h-2.5 w-2.5 rounded-full"
            :class="isCourseCertificateTranslationReady(translation, translations)
              ? 'bg-emerald-400'
              : activeTranslationId === translation.id
                ? 'bg-white/80'
                : 'bg-amber-400'"
          />
          {{ getLanguageLabel(translation.languageCode) }}
        </button>
      </div>
    </section>

    <div
      v-if="activeTranslation"
      class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_26rem]"
    >
      <section class="space-y-6">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div class="space-y-1">
              <div class="flex flex-wrap items-center gap-3">
                <h3 class="text-lg font-semibold text-slate-900">
                  {{ getLanguageLabel(activeTranslation.languageCode) }}
                </h3>
                <span
                  class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
                  :class="isCourseCertificateTranslationReady(activeTranslation, translations)
                    ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
                    : 'border-amber-200 bg-amber-50 text-amber-700'"
                >
                  {{ isCourseCertificateTranslationReady(activeTranslation, translations) ? 'Gotowe do zapisu' : 'Wymaga uzupełnienia' }}
                </span>
              </div>

              <p class="text-sm text-slate-500">
                Ustal kod języka, przetłumacz nazwę kursu i przygotuj komplet programu oraz szablonu.
              </p>
            </div>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="disabled"
              @click="removeTranslation(activeTranslation.id)"
            >
              Usuń język
            </button>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-[14rem_minmax(0,1fr)]">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Kod języka</span>
              <input
                v-model="activeTranslation.languageCode"
                type="text"
                list="course-translation-language-options"
                placeholder="np. en"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwa kursu w tym języku</span>
              <input
                v-model="activeTranslation.courseName"
                type="text"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
          </div>

          <datalist id="course-translation-language-options">
            <option
              v-for="language in languagePresets"
              :key="language.code"
              :value="language.code"
            >
              {{ language.label }}
            </option>
          </datalist>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div class="space-y-1">
              <h3 class="text-lg font-semibold text-slate-900">
                Program w wybranym języku
              </h3>
              <p class="text-sm text-slate-500">
                Tlumacz tematy i zachowaj odpowiednie liczby godzin teorii oraz praktyki.
              </p>
            </div>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="disabled"
              @click="addProgramRow(activeTranslation)"
            >
              Dodaj temat
            </button>
          </div>

          <div class="mt-5 space-y-4">
            <article
              v-for="(row, index) in activeTranslation.programRows"
              :key="row.id"
              class="rounded-lg border border-slate-200 bg-slate-50/80 p-4"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
                    Temat {{ index + 1 }}
                  </p>
                  <p class="mt-1 text-sm text-slate-500">
                    Przetłumacz nazwę tematu i skoryguj godziny tylko wtedy, gdy to potrzebne.
                  </p>
                </div>

                <div class="flex items-center gap-2 self-start sm:self-auto">
                  <button
                    type="button"
                    class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="disabled || index === 0"
                    @click="moveProgramRow(activeTranslation, index, -1)"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="disabled || index === activeTranslation.programRows.length - 1"
                    @click="moveProgramRow(activeTranslation, index, 1)"
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-9 items-center justify-center rounded-md border border-red-200 bg-red-50 px-3 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-70"
                    :disabled="disabled"
                    @click="removeProgramRow(activeTranslation, index)"
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
                    rows="4"
                    class="w-full resize-y rounded-md border border-slate-300 bg-white px-3 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                    placeholder="Np. Safe operation of the equipment"
                    @input="markProgramAsEdited(activeTranslation)"
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
                      @input="markProgramAsEdited(activeTranslation)"
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
                      @input="markProgramAsEdited(activeTranslation)"
                    >
                  </label>
                </div>
              </div>
            </article>
          </div>

          <div
            v-if="activeTranslation.hasInvalidStoredProgram"
            class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
          >
            W tej wersji zapisany był program w nieobsługiwanym formacie. Zmień program i zapisz kurs, aby go ujednolicić.
          </div>

          <div
            v-if="translationHasInvalidProgram(activeTranslation)"
            class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
          >
            Każdy temat musi mieć nazwę, a godziny teorii i praktyki muszą być liczbami.
          </div>
        </section>

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-1">
            <h3 class="text-lg font-semibold text-slate-900">
              Szablon w wybranym języku
            </h3>
            <p class="text-sm text-slate-500">
              Edytuj wizualnie front zaświadczenia i wstawiaj placeholdery bez pisania kodu HTML.
            </p>
          </div>

          <div class="mt-5 space-y-5">
            <div class="space-y-3">
              <p class="text-sm font-medium text-slate-700">
                Formatowanie
              </p>

              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('formatBlock', '<h1>')"
                >
                  H1
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('formatBlock', '<h2>')"
                >
                  H2
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('bold')"
                >
                  B
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm italic text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('italic')"
                >
                  I
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('justifyLeft')"
                >
                  Lewo
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('justifyCenter')"
                >
                  Środek
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('justifyRight')"
                >
                  Prawo
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('insertUnorderedList')"
                >
                  Lista
                </button>
                <button
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="runTemplateCommand('removeFormat')"
                >
                  Wyczyść styl
                </button>
              </div>
            </div>

            <div class="space-y-3">
              <p class="text-sm font-medium text-slate-700">
                Wielkość czcionki
              </p>

              <div class="flex flex-wrap gap-2">
                <button
                  v-for="option in fontSizeOptions"
                  :key="option.value"
                  type="button"
                  class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="applyTemplateFontSize(option.value)"
                >
                  {{ option.label }}
                </button>
              </div>
            </div>

            <div class="space-y-3">
              <p class="text-sm font-medium text-slate-700">
                Placeholdery
              </p>

              <div class="flex flex-wrap gap-2">
                <button
                  v-for="placeholder in templatePlaceholders"
                  :key="placeholder.value"
                  type="button"
                  class="rounded-md border border-sky-200 bg-sky-50 px-3 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-70"
                  :disabled="disabled"
                  @mousedown.prevent
                  @click="insertTemplatePlaceholder(placeholder.value)"
                >
                  {{ placeholder.label }}
                </button>
              </div>
            </div>

            <ClientOnly>
              <div class="rounded-lg border border-slate-200 bg-white">
                <div
                  ref="templateEditorRef"
                  :contenteditable="!disabled"
                  class="min-h-[24rem] w-full rounded-lg px-4 py-4 text-slate-900 outline-none"
                  @input="onTemplateEditorInput"
                />
              </div>
            </ClientOnly>

            <div class="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
              <p class="text-sm text-slate-500">
                Pracuj wizualnie, a w razie potrzeby podejrzyj lub popraw surowy HTML.
              </p>
              <button
                type="button"
                class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
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
                v-model="activeTranslation.certFrontPage"
                rows="14"
                class="w-full rounded-md border border-slate-300 bg-slate-950 px-4 py-3 font-mono text-sm leading-6 text-slate-100 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                placeholder="<section><h1>Certificate</h1><p>...</p></section>"
              />
            </label>
          </div>
        </section>
      </section>

      <aside class="space-y-6">
        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <h3 class="text-lg font-semibold text-slate-900">
            Podgląd programu
          </h3>

          <div
            v-if="translationProgramEntries(activeTranslation).length"
            class="mt-5 overflow-hidden rounded-lg border border-slate-200"
          >
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-slate-200 text-sm">
                <thead class="bg-slate-50">
                  <tr>
                    <th class="px-4 py-3 text-left font-medium text-slate-600">
                      Topic
                    </th>
                    <th class="px-4 py-3 text-right font-medium text-slate-600">
                      Theory
                    </th>
                    <th class="px-4 py-3 text-right font-medium text-slate-600">
                      Practice
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 bg-white">
                  <tr
                    v-for="(entry, index) in translationProgramEntries(activeTranslation)"
                    :key="`${entry.Subject}-${index}`"
                  >
                    <td class="px-4 py-3 align-top text-slate-900">
                      {{ entry.Subject }}
                    </td>
                    <td class="px-4 py-3 text-right text-slate-600">
                      {{ entry.TheoryTime }}
                    </td>
                    <td class="px-4 py-3 text-right text-slate-600">
                      {{ entry.PracticeTime }}
                    </td>
                  </tr>
                </tbody>
                <tfoot class="bg-slate-50">
                  <tr>
                    <td class="px-4 py-3 font-medium text-slate-700">
                      Total
                    </td>
                    <td class="px-4 py-3 text-right font-medium text-slate-700">
                      {{ translationTotals(activeTranslation).theory }}
                    </td>
                    <td class="px-4 py-3 text-right font-medium text-slate-700">
                      {{ translationTotals(activeTranslation).practice }}
                    </td>
                  </tr>
                </tfoot>
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

        <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <h3 class="text-lg font-semibold text-slate-900">
            Podgląd szablonu
          </h3>

          <div
            v-if="activeTranslation.certFrontPage.trim()"
            class="mt-5 overflow-hidden rounded-lg border border-slate-200 bg-slate-50"
          >
            <iframe
              title="Podgląd szablonu wersji obcojęzycznej"
              :srcdoc="buildTemplatePreviewDocument(activeTranslation.certFrontPage)"
              class="h-[44rem] w-full bg-white"
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
  </section>
</template>
