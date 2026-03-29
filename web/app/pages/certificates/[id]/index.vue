<script setup lang="ts">
import AuditHistoryPanel from '~/components/audit/AuditHistoryPanel.vue'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()
const auth = useAuth()
const isAdmin = computed(() => auth.user.value?.role === 1)
const previewFrame = ref<HTMLIFrameElement | null>(null)
const showDeleteConfirmation = ref(false)
const deleteReason = ref('')
const deletePending = ref(false)
const deleteError = ref('')

const certificateId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(certificateId.value) || certificateId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono zaświadczenia'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `certificate:${certificateId.value}`,
  async () => {
    return await api.certificate(certificateId.value)
  }
)

const {
  data: auditData,
  pending: auditPending,
  error: auditError,
  refresh: refreshAudit
} = await useAsyncData(
  `certificate-audit:${certificateId.value}`,
  async () => {
    if (!isAdmin.value) {
      return { data: [] }
    }

    return await api.certificateAuditLog(certificateId.value)
  },
  {
    watch: [isAdmin]
  }
)

const certificate = computed(() => data.value?.data ?? null)
const editCertificateLink = computed(() => `/certificates/${certificateId.value}/edit`)
const auditEntries = computed(() => auditData.value?.data ?? [])
const auditErrorMessage = computed(() => {
  return auditError.value ? getApiErrorMessage(auditError.value, 'Nie udało się pobrać historii zmian zaświadczenia.') : ''
})
const selectedPrintLanguageCode = ref<string>('')
const linkedJournalLink = computed(() => {
  if (!certificate.value?.journal) {
    return ''
  }

  return `/journals/${certificate.value.journal.id}`
})

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

type CourseProgramLabels = {
  index: string
  subject: string
  theoryHours: string
  practiceHours: string
  total: string
}

function formatLanguageLabel(value: string) {
  const normalized = value.trim().toLowerCase()
  const labels: Record<string, string> = {
    pl: 'polski',
    en: 'angielski',
    de: 'niemiecki',
    uk: 'ukraiński',
    cs: 'czeski',
    sk: 'słowacki',
    lt: 'litewski'
  }

  if (!normalized) {
    return 'domyślna'
  }

  return labels[normalized]
    ? `${normalized.toUpperCase()} - ${labels[normalized]}`
    : normalized.toUpperCase()
}

function getCourseProgramLabels(languageCode: string): CourseProgramLabels {
  switch (languageCode.trim().toLowerCase()) {
    case 'en':
      return {
        index: 'No.',
        subject: 'Training topic',
        theoryHours: 'Theory hours',
        practiceHours: 'Practical hours',
        total: 'TOTAL'
      }
    case 'de':
      return {
        index: 'Nr.',
        subject: 'Schulungsthema',
        theoryHours: 'Theoriestunden',
        practiceHours: 'Praxisstunden',
        total: 'SUMME'
      }
    case 'uk':
      return {
        index: '№',
        subject: 'Тема навчання',
        theoryHours: 'Теоретичні години',
        practiceHours: 'Практичні години',
        total: 'РАЗОМ'
      }
    case 'cs':
      return {
        index: 'Č.',
        subject: 'Téma školení',
        theoryHours: 'Teoretické hodiny',
        practiceHours: 'Praktické hodiny',
        total: 'CELKEM'
      }
    case 'sk':
      return {
        index: 'Č.',
        subject: 'Téma školenia',
        theoryHours: 'Teoretické hodiny',
        practiceHours: 'Praktické hodiny',
        total: 'SPOLU'
      }
    case 'lt':
      return {
        index: 'Nr.',
        subject: 'Mokymo tema',
        theoryHours: 'Teorijos valandos',
        practiceHours: 'Praktikos valandos',
        total: 'IŠ VISO'
      }
    default:
      return {
        index: 'Lp.',
        subject: 'Temat szkolenia',
        theoryHours: 'Liczba godzin zajęć teoretycznych',
        practiceHours: 'Liczba godzin zajęć praktycznych',
        total: 'RAZEM'
      }
  }
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

const certificateNumber = computed(() => {
  if (!certificate.value) {
    return ''
  }

  return `${certificate.value.registryNumber}/${certificate.value.courseSymbol}/${certificate.value.registryYear}`
})

const studentFullName = computed(() => {
  if (!certificate.value) {
    return ''
  }

  return [
    certificate.value.studentName,
    certificate.value.studentSecondname,
    certificate.value.studentLastname
  ]
    .filter(Boolean)
    .join(' ')
})

watch(
  certificate,
  (value) => {
    if (!value) {
      selectedPrintLanguageCode.value = ''
      return
    }

    if (
      !value.printVariants.some(
        variant => variant.languageCode === selectedPrintLanguageCode.value
      )
    ) {
      selectedPrintLanguageCode.value = value.languageCode
    }
  },
  { immediate: true }
)

const selectedPrintVariant = computed(() => {
  if (!certificate.value) {
    return null
  }

  return (
    certificate.value.printVariants.find(
      variant => variant.languageCode === selectedPrintLanguageCode.value
    )
    ?? certificate.value.printVariants[0]
    ?? null
  )
})

const selectedPrintVariantOptionLabel = computed(() => {
  if (!selectedPrintVariant.value) {
    return ''
  }

  return selectedPrintVariant.value.isOriginal
    ? `${formatLanguageLabel(selectedPrintVariant.value.languageCode)} - wersja wystawiona`
    : `${formatLanguageLabel(selectedPrintVariant.value.languageCode)} - wersja do wydruku`
})

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll('\'', '&#39;')
}

const courseProgramEntries = computed(() => {
  if (!selectedPrintVariant.value?.courseProgram) {
    return [] as CourseProgramEntry[]
  }

  try {
    const parsed = JSON.parse(selectedPrintVariant.value.courseProgram)
    if (!Array.isArray(parsed)) {
      return [] as CourseProgramEntry[]
    }

    return parsed as CourseProgramEntry[]
  } catch {
    return [] as CourseProgramEntry[]
  }
})

const courseProgramTableHtml = computed(() => {
  if (courseProgramEntries.value.length === 0) {
    return ''
  }

  const labels = getCourseProgramLabels(selectedPrintLanguageCode.value)

  let theorySum = 0
  let practiceSum = 0

  const rowsHtml = courseProgramEntries.value
    .map((entry, index) => {
      const theory = entry.TheoryTime ?? '0'
      const practice = entry.PracticeTime ?? '0'

      theorySum += Number.parseFloat(theory) || 0
      practiceSum += Number.parseFloat(practice) || 0

      return `
        <tr>
          <td>${index + 1}</td>
          <td>${escapeHtml(entry.Subject ?? '')}</td>
          <td class="hour-cell">${escapeHtml(theory)}</td>
          <td class="hour-cell">${escapeHtml(practice)}</td>
        </tr>
      `
    })
    .join('')

  return `
    <div class="certificate-sheet secondary-sheet">
      <div class="sheet-caption">Program szkolenia</div>
      <table>
        <colgroup>
          <col class="col-lp">
          <col class="col-subject">
          <col class="col-hours">
          <col class="col-hours">
        </colgroup>
        <thead>
          <tr>
            <th>${labels.index}</th>
            <th>${labels.subject}</th>
            <th>${labels.theoryHours}</th>
            <th>${labels.practiceHours}</th>
          </tr>
        </thead>
        <tbody>
          ${rowsHtml}
          <tr>
            <td colspan="2"><strong>${labels.total}</strong></td>
            <td class="hour-cell"><strong>${theorySum.toFixed(1)}</strong></td>
            <td class="hour-cell"><strong>${practiceSum.toFixed(1)}</strong></td>
          </tr>
        </tbody>
      </table>
    </div>
  `
})

const certificatePreviewHtml = computed(() => {
  if (!certificate.value || !selectedPrintVariant.value?.certFrontPage) {
    return ''
  }

  const values: Record<string, string> = {
    imie: certificate.value.studentName || '',
    drugie_imie: certificate.value.studentSecondname || '',
    nazwisko: certificate.value.studentLastname || '',
    pesel: certificate.value.studentPesel || '',
    data_urodzenia: formatPolishDate(certificate.value.studentBirthdate),
    miejsce_urodzenia: certificate.value.studentBirthplace || '',
    nazwa_kursu: selectedPrintVariant.value.courseName || '',
    data_rozpoczecia: formatPolishDate(certificate.value.courseDateStart),
    data_zakonczenia: formatPolishDate(certificate.value.courseDateEnd),
    data_wystawienia: formatPolishDate(certificate.value.date),
    numer_zaswiadczenia: certificateNumber.value
  }

  return selectedPrintVariant.value.certFrontPage.replace(/{{(.*?)}}/g, (_, rawTag: string) => {
    const normalizedTag = rawTag.replaceAll(/\s+/g, '')
    return values[normalizedTag] ?? ''
  })
})

const certificatePreviewDocument = computed(() => {
  if (!certificatePreviewHtml.value) {
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
        font-family: "Liberation Serif","Times New Roman", Georgia, serif;
        line-height: 1.4;
      }

      .certificate-sheet {
        width: min(210mm, 100%);
        min-height: 297mm;
        margin: 0 auto 20px auto;
        padding: 16mm 16mm;
        border: 1px solid #cbd5e1;
        border-radius: 8px;
        background: white;
        box-shadow:
          0 30px 70px rgb(15 23 42 / 0.10),
          0 10px 24px rgb(15 23 42 / 0.08);
        page-break-after: always;
        break-after: page;
        page-break-inside: avoid;
        break-inside: avoid-page;
      }

      .certificate-sheet:last-of-type {
        margin-bottom: 0;
        page-break-after: auto;
        break-after: auto;
      }

      .certificate-sheet > :first-child {
        margin-top: 0 !important;
      }

      .certificate-sheet > :last-child {
        margin-bottom: 0 !important;
      }

      .sheet-caption {
        margin-bottom: 12px;
        font-size: 11px;
        font-family: ui-sans-serif, system-ui, sans-serif;
        letter-spacing: 0.18em;
        text-transform: uppercase;
        color: #475569;
      }

      h1, h2, h3, h4, h5, h6 {
        margin: 0 0 0.45rem;
        line-height: 1.2;
        color: #020617;
      }

      h1 {
        font-size: 54px;
        font-weight: 700;
        letter-spacing: 0.02em;
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
        line-height: 1.25;
      }

      ul, ol {
        margin: 0 0 0.45rem;
        padding-left: 1.25rem;
      }

      img {
        max-width: 100%;
        height: auto;
      }

      table {
        width: 100%;
        border-collapse: collapse;
        table-layout: fixed;
      }

      .col-lp {
        width: 7%;
      }

      .col-subject {
        width: 61%;
      }

      .col-hours {
        width: 16%;
      }

      th {
        background: #f8fafc;
      }

      td, th {
        border: 1px solid #0f172a;
        padding: 6px;
        vertical-align: top;
        font-size: 12px;
        line-height: 1.35;
      }

      .hour-cell {
        text-align: center;
        white-space: nowrap;
      }

      .secondary-sheet {
        padding-top: 15mm;
      }

      @media print {
        html,
        body {
          width: 210mm;
          margin: 0;
          padding: 0;
          background: white !important;
        }

        body {
          color: #000;
          zoom: 0.96;
        }

        .certificate-sheet {
          width: 210mm;
          min-height: 297mm;
          margin: 0;
          padding: 25mm 25mm 12mm 30mm;
          border: none;
          border-radius: 0;
          box-shadow: none;
          page-break-after: always;
          break-after: page;
        }

        .certificate-sheet:last-of-type {
          page-break-after: auto;
          break-after: auto;
        }

        .secondary-sheet {
          padding-top: 30mm;
        }

        h1 {
          font-size: 54px;
        }

        h2 {
          font-size: 24px;
        }

        h3 {
          font-size: 18px;
        }

        .sheet-caption {
          display: none;
        }

        p {
          margin-bottom: 0.32rem;
          font-size: 14px;
        }

        td, th {
          padding: 5px;
          font-size: 11px;
        }
      }
    </style>
  </head>
  <body>
    <div class="certificate-sheet">
      <div class="sheet-caption">Zaświadczenie</div>
      ${certificatePreviewHtml.value}
    </div>
    ${courseProgramTableHtml.value}
  </body>
</html>`
})

const certificatePreviewHeightClass = computed(() => {
  return courseProgramEntries.value.length > 0 ? 'h-[84rem]' : 'h-[58rem]'
})

const pdfDownloadUrl = computed(() => {
  const languageCode = selectedPrintVariant.value?.languageCode
  if (!languageCode || languageCode === certificate.value?.languageCode) {
    return `/api/v1/certificates/${certificateId.value}/pdf`
  }

  return `/api/v1/certificates/${certificateId.value}/pdf?language=${encodeURIComponent(languageCode)}`
})

function printCertificatePreview() {
  const frameWindow = previewFrame.value?.contentWindow
  if (!frameWindow) {
    return
  }

  frameWindow.focus()
  frameWindow.print()
}

async function onDeleteCertificate() {
  deleteError.value = ''
  deletePending.value = true

  try {
    await api.deleteCertificate(certificateId.value, {
      deleteReason: deleteReason.value.trim() || null
    })
    await navigateTo('/certificates')
  } catch (apiError) {
    deleteError.value = getApiErrorMessage(apiError, 'Nie udało się usunąć zaświadczenia.')
  } finally {
    deletePending.value = false
  }
}

async function refreshAll() {
  await Promise.all([
    refresh(),
    isAdmin.value ? refreshAudit() : Promise.resolve()
  ])
}

useSeoMeta({
  title: () => certificateNumber.value || 'Szczegół zaświadczenia'
})
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Zaświadczenia</p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          {{ certificateNumber || 'Szczegół zaświadczenia' }}
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Szczegóły wybranego wpisu z rejestru wraz z danymi kursanta, kursem i zakresem dat.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || (isAdmin && auditPending)"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/certificates"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Lista zaświadczeń
        </NuxtLink>

        <NuxtLink
          to="/certificates/new"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Nowe zaświadczenie
        </NuxtLink>

        <NuxtLink
          :to="editCertificateLink"
          class="inline-flex items-center justify-center rounded-lg bg-slate-950 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800"
        >
          Edytuj zaświadczenie
        </NuxtLink>

        <button
          v-if="isAdmin"
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100"
          @click="showDeleteConfirmation = !showDeleteConfirmation"
        >
          Usuń zaświadczenie
        </button>

        <label
          v-if="certificate?.printVariants?.length"
          class="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
        >
          <span class="text-slate-500">Wersja wydruku</span>
          <select
            v-model="selectedPrintLanguageCode"
            class="min-w-[14rem] border-0 bg-transparent p-0 text-sm font-medium text-slate-900 outline-none"
          >
            <option
              v-for="variant in certificate.printVariants"
              :key="`${variant.languageCode}-${variant.isOriginal}`"
              :value="variant.languageCode"
            >
              {{
                variant.isOriginal
                  ? `${formatLanguageLabel(variant.languageCode)} - wersja wystawiona`
                  : `${formatLanguageLabel(variant.languageCode)} - wersja do wydruku`
              }}
            </option>
          </select>
        </label>

        <a
          :href="pdfDownloadUrl"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Pobierz PDF
        </a>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          @click="printCertificatePreview"
        >
          Drukuj
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać szczegółów zaświadczenia.
    </div>

    <div
      v-else-if="pending || !certificate"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie szczegółów...
    </div>

    <template v-else>
      <div
        v-if="showDeleteConfirmation && isAdmin"
        class="rounded-xl border border-red-200 bg-red-50 px-6 py-5"
      >
        <div class="space-y-4">
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.16em] text-red-700">
              Operacja administracyjna
            </p>
            <h2 class="text-lg font-semibold text-red-900">
              Usuń zaświadczenie z aktywnego obiegu
            </h2>
            <p class="text-sm leading-6 text-red-800">
              Rekord zostanie usunięty logicznie. Numer w rejestrze pozostanie zajęty.
            </p>
          </div>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-red-900">Powód usunięcia</span>
            <textarea
              v-model="deleteReason"
              rows="3"
              placeholder="Np. dokument wystawiony omyłkowo"
              class="w-full rounded-md border border-red-200 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-red-300 focus:ring-4 focus:ring-red-100"
            />
          </label>

          <div
            v-if="deleteError"
            class="rounded-lg border border-red-200 bg-white px-4 py-3 text-sm text-red-700"
          >
            {{ deleteError }}
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-red-700 disabled:cursor-not-allowed disabled:bg-red-300"
              :disabled="deletePending"
              @click="onDeleteCertificate"
            >
              {{ deletePending ? 'Usuwanie...' : 'Potwierdź usunięcie' }}
            </button>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100"
              :disabled="deletePending"
              @click="showDeleteConfirmation = false"
            >
              Anuluj
            </button>
          </div>
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-3">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Data zaświadczenia</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ certificate.date }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Ważność</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ certificate.expiryDate ?? 'Brak terminu' }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">Numer</p>
          <p class="mt-3 font-mono text-2xl font-semibold tracking-tight break-all">
            {{ certificateNumber }}
          </p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Kursant</h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Imię i nazwisko</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ studentFullName }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">PESEL</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.studentPesel || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Data urodzenia</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.studentBirthdate }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Miejsce urodzenia
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.studentBirthplace }}
                </dd>
              </div>

              <div class="md:col-span-2">
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Firma</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.companyName || 'Brak przypisanej firmy' }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Kurs</h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Symbol</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.courseSymbol }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Nazwa</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.courseName }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Rozpoczęcie</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.courseDateStart }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Zakończenie</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.courseDateEnd ?? 'Brak' }}
                </dd>
              </div>

              <div class="md:col-span-2">
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Okres ważności</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{
                    certificate.courseExpiryTime
                      ? `${certificate.courseExpiryTime} lat`
                      : 'Brak ustawionego okresu ważności'
                  }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div class="space-y-1">
                <h2 class="text-lg font-semibold text-slate-900">Podgląd zaświadczenia</h2>
                <p class="text-sm text-slate-500">
                  Aktualnie oglądasz {{ selectedPrintVariantOptionLabel || 'wersję wystawioną' }}.
                </p>
              </div>

              <span
                v-if="selectedPrintVariant"
                class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
                :class="
                  selectedPrintVariant.isOriginal
                    ? 'border-amber-200 bg-amber-50 text-amber-700'
                    : 'border-sky-200 bg-sky-50 text-sky-700'
                "
              >
                {{ selectedPrintVariant.isOriginal ? 'Wersja wystawiona' : 'Wersja do wydruku' }}
              </span>
            </div>

            <div
              v-if="certificatePreviewDocument"
              class="mt-5 overflow-hidden rounded-lg border border-slate-200 bg-slate-50"
            >
              <div
                class="border-b border-slate-200 bg-white px-4 py-3 text-xs uppercase tracking-[0.16em] text-slate-500"
              >
                Podgląd A4
              </div>
              <iframe
                ref="previewFrame"
                :srcdoc="certificatePreviewDocument"
                title="Podgląd zaświadczenia"
                :class="`${certificatePreviewHeightClass} w-full bg-white`"
              />
            </div>

            <div
              v-else
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Brak dostępnego szablonu podglądu zaświadczenia.
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
                  {{ certificate.id }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Rok rejestru</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.registryYear }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Numer porządkowy</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ certificate.registryNumber }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Powiązany dziennik</h2>

            <div v-if="certificate.journal" class="mt-5 space-y-4">
              <div>
                <p class="text-xs uppercase tracking-[0.16em] text-slate-400">Tytuł</p>
                <p class="mt-1 text-sm font-medium text-slate-900">
                  {{ certificate.journal.title }}
                </p>
              </div>

              <div class="flex flex-wrap items-center gap-3">
                <span
                  class="inline-flex items-center rounded-full px-3 py-1 text-xs font-medium capitalize"
                  :class="
                    certificate.journal.status === 'closed'
                      ? 'bg-emerald-100 text-emerald-700'
                      : 'bg-amber-100 text-amber-700'
                  "
                >
                  {{ certificate.journal.status === 'closed' ? 'Zamknięty' : 'Roboczy' }}
                </span>

                <NuxtLink
                  :to="linkedJournalLink"
                  class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                >
                  Otwórz dziennik
                </NuxtLink>
              </div>
            </div>

            <div
              v-else
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-4 text-sm text-slate-500"
            >
              To zaświadczenie nie jest jeszcze powiązane z żadnym dziennikiem szkolenia.
            </div>
          </section>
        </aside>
      </div>

      <AuditHistoryPanel
        v-if="isAdmin"
        :entries="auditEntries"
        :pending="auditPending"
        :error-message="auditErrorMessage"
        title="Historia zmian zaświadczenia"
        description="Zmiany zapisane dla tego zaświadczenia, w tym operacje administracyjne i wydrukowe."
        empty-message="Brak wpisów historii zmian dla tego zaświadczenia."
      />
    </template>
  </section>
</template>
