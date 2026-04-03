<script setup lang="ts">
import type {
  CourseDetails,
  JournalAttendee,
  JournalDetails,
  JournalSession
} from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const journalId = Number.parseInt(`${route.params.id}`, 10)
const api = useApi()

if (!Number.isFinite(journalId) || journalId <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Dziennik nie istnieje'
  })
}

useSeoMeta({
  title: 'Wydruk dziennika'
})

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

const { data, pending, error } = await useAsyncData(`journal-print-${journalId}`, async () => {
  const journalResponse = await api.journal(journalId)

  const [attendeesResponse, sessionsResponse, attendanceResponse] = await Promise.all([
    api.journalAttendees(journalId),
    api.journalSessions(journalId),
    api.journalAttendance(journalId)
  ])

  let course: CourseDetails | null = null
  try {
    const courseResponse = await api.course(journalResponse.data.courseId)
    course = courseResponse.data
  } catch {
    course = null
  }

  return {
    journal: journalResponse.data,
    course,
    attendees: attendeesResponse.data,
    sessions: sessionsResponse.data,
    attendance: attendanceResponse.data
  }
})

const journal = computed<JournalDetails | null>(() => data.value?.journal ?? null)
const course = computed<CourseDetails | null>(() => data.value?.course ?? null)
const attendees = computed<JournalAttendee[]>(() => data.value?.attendees ?? [])
const sessions = computed<JournalSession[]>(() => data.value?.sessions ?? [])
const attendanceEntries = computed(() => data.value?.attendance ?? [])

const courseProgramEntries = computed(() => {
  if (!course.value?.courseProgram) {
    return [] as CourseProgramEntry[]
  }

  try {
    const parsed = JSON.parse(course.value.courseProgram)
    return Array.isArray(parsed) ? (parsed as CourseProgramEntry[]) : []
  } catch {
    return [] as CourseProgramEntry[]
  }
})

const programSplitBySortOrder = computed(() => {
  const map: Record<number, { theory: string, practice: string }> = {}

  for (const session of sessions.value) {
    const entry = courseProgramEntries.value[session.sortOrder - 1]

    if (!entry) {
      map[session.sortOrder] = {
        theory: formatProgramHours(session.hours),
        practice: '0'
      }
      continue
    }

    map[session.sortOrder] = {
      theory: formatProgramHours(entry?.TheoryTime),
      practice: formatProgramHours(entry?.PracticeTime)
    }
  }

  return map
})

const attendanceLookup = computed(() => {
  const map = new Map<string, boolean>()

  for (const entry of attendanceEntries.value) {
    map.set(`${entry.journalSessionId}:${entry.journalAttendeeId}`, entry.present)
  }

  return map
})

const shortenedPrintTopics = computed(() => {
  const map: Record<number, string> = {}

  for (const session of sessions.value) {
    const normalized = session.topic.trim()

    if (normalized.length <= 22) {
      map[session.id] = normalized
      continue
    }

    const shortened = normalized.slice(0, 22).trimEnd()
    const lastSpace = shortened.lastIndexOf(' ')
    map[session.id] = `${lastSpace > 10 ? shortened.slice(0, lastSpace) : shortened}...`
  }

  return map
})

function formatHours(value: number | string) {
  return `${String(value)
    .replace(/\.0+$/, '')
    .replace(/(\.\d*[1-9])0+$/, '$1')} h`
}

function formatProgramHours(value: string | undefined) {
  const normalized = `${value || ''}`.trim().replace(',', '.')
  const parsed = Number.parseFloat(normalized)

  if (!Number.isFinite(parsed)) {
    return '0'
  }

  return parsed
    .toString()
    .replace(/\.0+$/, '')
    .replace(/(\.\d*[1-9])0+$/, '$1')
}

function formatDate(value: string | null) {
  if (!value) {
    return 'Brak'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return parsed.toLocaleDateString('pl-PL')
}

function formatCertificateNumber(attendee: JournalAttendee) {
  if (!attendee.certificate) {
    return 'Brak'
  }

  return `${attendee.certificate.registryNumber}/${attendee.certificate.courseSymbol}/${attendee.certificate.registryYear}`
}

function attendanceValue(sessionId: number, attendeeId: number) {
  return attendanceLookup.value.get(`${sessionId}:${attendeeId}`) ?? false
}

function printNow() {
  if (import.meta.client) {
    window.print()
  }
}

onMounted(() => {
  if (route.query.autoprint === '1') {
    window.setTimeout(() => {
      printNow()
    }, 150)
  }
})
</script>

<template>
  <section class="space-y-6">
    <div
      class="no-print flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm"
    >
      <div class="space-y-1">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Wydruk</p>
        <h1 class="text-2xl font-semibold tracking-tight text-slate-900">Dziennik szkolenia</h1>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <NuxtLink
          :to="`/journals/${journalId}`"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Wróć do dziennika
        </NuxtLink>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
          @click="printNow"
        >
          Drukuj
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się przygotować widoku wydruku dziennika.
    </div>

    <div
      v-else-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Przygotowywanie wydruku dziennika...
    </div>

    <div v-else-if="journal" class="print-document space-y-6">
      <article class="print-sheet space-y-6">
        <div class="space-y-3 border-b border-slate-200 pb-5">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-flex items-center justify-center rounded-full border border-slate-300 px-3 py-1 text-xs font-medium text-slate-600"
            >
              {{ journal.status === 'closed' ? 'Zamknięty' : 'Roboczy' }}
            </span>
            <span class="text-xs uppercase tracking-[0.18em] text-slate-400">
              {{ journal.courseSymbol }}
            </span>
          </div>

          <div class="space-y-1">
            <h2 class="text-3xl font-semibold tracking-tight text-slate-900">
              {{ journal.title }}
            </h2>
            <p class="text-base text-slate-600">
              {{ journal.courseName }}
            </p>
          </div>
        </div>

        <dl class="grid gap-4 text-sm md:grid-cols-2">
          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Organizator</dt>
            <dd class="text-slate-700">
              {{ journal.organizerName }}
            </dd>
          </div>

          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Miejsce</dt>
            <dd class="text-slate-700">
              {{ journal.location }}
            </dd>
          </div>

          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Forma szkolenia</dt>
            <dd class="text-slate-700">
              {{ journal.formOfTraining }}
            </dd>
          </div>

          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Firma</dt>
            <dd class="text-slate-700">
              {{ journal.companyName || 'Bez przypisanej firmy' }}
            </dd>
          </div>

          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Termin</dt>
            <dd class="text-slate-700">
              {{ formatDate(journal.dateStart) }} - {{ formatDate(journal.dateEnd) }}
            </dd>
          </div>

          <div class="space-y-1">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Liczba godzin</dt>
            <dd class="text-slate-700">
              {{ formatHours(journal.totalHours) }}
            </dd>
          </div>

          <div class="space-y-1 md:col-span-2">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Podstawa prawna</dt>
            <dd class="text-slate-700">
              {{ journal.legalBasis }}
            </dd>
          </div>

          <div class="space-y-1 md:col-span-2">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Adres organizatora</dt>
            <dd class="text-slate-700">
              {{ journal.organizerAddress || 'Brak adresu organizatora' }}
            </dd>
          </div>

          <div class="space-y-1 md:col-span-2">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Notatki</dt>
            <dd class="text-slate-700">
              {{ journal.notes || 'Brak notatek' }}
            </dd>
          </div>
        </dl>
      </article>

      <article class="print-sheet">
        <div class="mb-4 space-y-1">
          <h2 class="text-xl font-semibold text-slate-900">Lista uczestników</h2>
          <p class="text-sm text-slate-500">Lista uczestników przypisanych do tego szkolenia.</p>
        </div>

        <div class="overflow-x-auto">
          <table class="print-table">
            <thead>
              <tr>
                <th>Lp.</th>
                <th>Uczestnik</th>
                <th>Data urodzenia</th>
                <th>Firma</th>
                <th>Zaświadczenie</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="attendee in attendees" :key="attendee.id">
                <td>{{ attendee.sortOrder }}</td>
                <td>{{ attendee.fullNameSnapshot }}</td>
                <td>{{ formatDate(attendee.birthdateSnapshot) }}</td>
                <td>{{ attendee.companyNameSnapshot || 'Brak firmy' }}</td>
                <td>{{ formatCertificateNumber(attendee) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>

      <article class="print-sheet">
        <div class="mb-4 space-y-1">
          <h2 class="text-xl font-semibold text-slate-900">Program szkolenia</h2>
          <p class="text-sm text-slate-500">
            Tematy, prowadzący i godziny przypisane do dziennika.
          </p>
        </div>

        <div class="overflow-x-auto">
          <table class="print-table">
            <thead>
              <tr>
                <th>Lp.</th>
                <th>Data</th>
                <th>Temat</th>
                <th>Godziny teorii</th>
                <th>Godziny praktyki</th>
                <th>Prowadzący</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="session in sessions" :key="session.id">
                <td>{{ session.sortOrder }}</td>
                <td>{{ formatDate(session.sessionDate) }}</td>
                <td>{{ session.topic }}</td>
                <td>{{ programSplitBySortOrder[session.sortOrder]?.theory || '0' }}</td>
                <td>{{ programSplitBySortOrder[session.sortOrder]?.practice || '0' }}</td>
                <td>{{ session.trainerName }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="trainer-signature">
          <div class="trainer-signature__line">Podpis wykładowcy</div>
        </div>
      </article>

      <article class="print-sheet print-sheet-wide">
        <div class="mb-4 space-y-1">
          <h2 class="text-xl font-semibold text-slate-900">Lista obecności</h2>
          <p class="text-sm text-slate-500">
            Obecność uczestników dla poszczególnych pozycji programu.
          </p>
        </div>

        <div class="overflow-x-auto">
          <table class="print-table attendance-table">
            <thead>
              <tr>
                <th>Uczestnik</th>
                <th v-for="session in sessions" :key="session.id" :title="session.topic">
                  <div class="space-y-1">
                    <div>{{ session.sortOrder }}</div>
                    <div>{{ formatDate(session.sessionDate) }}</div>
                    <div>{{ shortenedPrintTopics[session.id] || session.topic }}</div>
                  </div>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="attendee in attendees" :key="attendee.id">
                <td class="attendee-cell">
                  {{ attendee.fullNameSnapshot }}
                </td>
                <td
                  v-for="session in sessions"
                  :key="`${attendee.id}-${session.id}`"
                  class="text-center"
                >
                  {{ attendanceValue(session.id, attendee.id) ? 'X' : '' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>
    </div>
  </section>
</template>

<style>
@page {
  size: A4 portrait;
  margin: 10mm;
}

@page attendance-landscape {
  size: A4 landscape;
  margin: 8mm;
}

.print-sheet {
  border: 1px solid rgb(226 232 240);
  border-radius: 14px;
  background: white;
  padding: 24px;
  box-shadow: 0 10px 28px rgb(15 23 42 / 0.08);
}

.print-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.print-table th,
.print-table td {
  border: 1px solid rgb(203 213 225);
  padding: 8px 10px;
  vertical-align: top;
}

.print-table thead th {
  background: rgb(248 250 252);
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: rgb(71 85 105);
}

.trainer-signature {
  padding-top: 96px;
}

.trainer-signature__line {
  width: 220px;
  border-top: 1px solid rgb(100 116 139);
  padding-top: 6px;
  font-size: 11px;
  text-align: center;
  color: rgb(71 85 105);
}

.attendance-table th:not(:first-child),
.attendance-table td:not(:first-child) {
  min-width: 92px;
}

.attendance-table th:first-child,
.attendance-table td:first-child {
  min-width: 220px;
}

@media print {
  html,
  body {
    background: white !important;
  }

  header,
  .no-print {
    display: none !important;
  }

  main {
    max-width: none !important;
    padding: 0 !important;
  }

  .print-document {
    margin: 0 !important;
    padding: 0 !important;
  }

  .print-sheet {
    border: 0;
    border-radius: 0;
    box-shadow: none;
    padding: 0;
    break-after: page;
    page-break-after: always;
  }

  .print-sheet:last-child {
    break-after: auto;
    page-break-after: auto;
  }

  .print-sheet-wide {
    page: attendance-landscape;
    page-break-before: always;
    break-before: page;
  }

  .print-sheet-wide .print-table {
    font-size: 10px;
  }

  .print-sheet-wide .print-table th,
  .print-sheet-wide .print-table td {
    padding: 5px 6px;
  }

  .print-sheet-wide .attendance-table th:first-child,
  .print-sheet-wide .attendance-table td:first-child {
    min-width: 165px;
    width: 165px;
  }

  .print-sheet-wide .attendance-table th:not(:first-child),
  .print-sheet-wide .attendance-table td:not(:first-child) {
    min-width: 64px;
    width: 64px;
  }

  .print-sheet-wide .attendance-table thead th {
    font-size: 8px;
    line-height: 1.2;
  }
}
</style>
