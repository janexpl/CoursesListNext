<script setup lang="ts">
import type {
  CourseDetails,
  JournalAttendanceScan,
  JournalSignedScan,
  JournalAttendee,
  JournalSession,
  StudentCertificateSummary,
  StudentSummary
} from '~/composables/useApi'
import type { RouteLocationRaw } from 'vue-router'
import JournalAddAttendeeCard from '~/components/journals/JournalAddAttendeeCard.vue'
import JournalAttendanceScanCard from '~/components/journals/JournalAttendanceScanCard.vue'
import JournalAttendanceSection from '~/components/journals/JournalAttendanceSection.vue'
import JournalAttendeesSection from '~/components/journals/JournalAttendeesSection.vue'
import JournalBasicInfoCard from '~/components/journals/JournalBasicInfoCard.vue'
import JournalHeaderPanel from '~/components/journals/JournalHeaderPanel.vue'
import JournalSessionsSection from '~/components/journals/JournalSessionsSection.vue'
import JournalTechnicalInfoCard from '~/components/journals/JournalTechnicalInfoCard.vue'

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const router = useRouter()
const api = useApi()
const printFrame = ref<HTMLIFrameElement | null>(null)
const activePrintDocument = ref('')

const journalId = Number.parseInt(`${route.params.id}`, 10)

if (!Number.isFinite(journalId) || journalId <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Dziennik nie istnieje'
  })
}

useSeoMeta({
  title: 'Szczegóły dziennika'
})

const showCreatedNotice = ref(route.query.created === '1')
const deleteJournalError = ref('')
const deleteJournalPending = ref(false)
const printJournalError = ref('')
const printJournalPending = ref(false)
const closeJournalError = ref('')
const closeJournalSuccess = ref('')
const closeJournalPending = ref(false)
const attendanceScanActionError = ref('')
const attendanceScanActionSuccess = ref('')
const attendanceScanUploadPending = ref(false)
const attendanceScanDeletePending = ref(false)
const attendanceScanFile = ref<File | null>(null)
const signedScanActionError = ref('')
const signedScanActionSuccess = ref('')
const signedScanUploadPending = ref(false)
const signedScanDeletePending = ref(false)
const signedScanFile = ref<File | null>(null)
const uploadPickerRef = ref<HTMLInputElement | null>(null)
const activeUploadPickerTarget = ref<'attendance' | 'signed' | null>(null)
const addAttendeeCardRef = ref<{ focusSearchInput: () => void } | null>(null)
const studentSearch = ref('')
const addAttendeeError = ref('')
const addAttendeeSuccess = ref('')
const addingStudentId = ref<number | null>(null)
const deleteAttendeeError = ref('')
const deleteAttendeeSuccess = ref('')
const deletingAttendeeId = ref<number | null>(null)
const attendeeCertificateError = ref('')
const attendeeCertificateSuccess = ref('')
const generateAttendeeCertificateError = ref('')
const generateAttendeeCertificateSuccess = ref('')
const generatingAttendeeCertificateId = ref<number | null>(null)
const editingCertificateAttendeeId = ref<number | null>(null)
const loadingAttendeeCertificatesId = ref<number | null>(null)
const savingAttendeeCertificateId = ref<number | null>(null)
const attendeeCertificateDrafts = ref<Record<number, string>>({})
const attendeeCertificateOptions = ref<Record<number, StudentCertificateSummary[]>>({})
const generateSessionsError = ref('')
const generateSessionsSuccess = ref('')
const generatingSessions = ref(false)
const attendanceSaveError = ref('')
const attendanceSaveSuccess = ref('')
const savingAttendanceKey = ref<string | null>(null)
const bulkSavingAttendeeId = ref<number | null>(null)
const attendanceDrafts = ref<Record<string, boolean>>({})
const sessionDrafts = ref<Record<number, { sessionDate: string, trainerName: string }>>({})
const sessionSaveErrors = ref<Record<number, string>>({})
const sessionUpdateSuccess = ref('')
const savingSessionId = ref<number | null>(null)
const printCourse = ref<CourseDetails | null>(null)
const loadingPrintCourse = ref(false)
let printCourseRequestId = 0

const {
  data: journalData,
  pending: journalPending,
  error: journalError,
  refresh: refreshJournal
} = await useAsyncData(`journal-${journalId}`, async () => await api.journal(journalId))

const {
  data: attendeesData,
  pending: attendeesPending,
  error: attendeesError,
  refresh: refreshAttendees
} = await useAsyncData(
  `journal-attendees-${journalId}`,
  async () => await api.journalAttendees(journalId)
)

const {
  data: sessionsData,
  pending: sessionsPending,
  error: sessionsError,
  refresh: refreshSessions
} = await useAsyncData(
  `journal-sessions-${journalId}`,
  async () => await api.journalSessions(journalId)
)

const {
  data: attendanceData,
  pending: attendancePending,
  error: attendanceError,
  refresh: refreshAttendance
} = await useAsyncData(
  `journal-attendance-${journalId}`,
  async () => await api.journalAttendance(journalId)
)

const {
  data: attendanceScanData,
  pending: attendanceScanPending,
  error: attendanceScanError,
  refresh: refreshAttendanceScan
} = await useAsyncData(`journal-attendance-scan-${journalId}`, async () => {
  try {
    return await api.journalAttendanceScanMeta(journalId)
  } catch (error) {
    if (isApiNotFoundError(error)) {
      return null
    }

    throw error
  }
})

const {
  data: signedScanData,
  pending: signedScanPending,
  error: signedScanError,
  refresh: refreshSignedScan
} = await useAsyncData(`journal-signed-scan-${journalId}`, async () => {
  try {
    return await api.journalSignedScanMeta(journalId)
  } catch (error) {
    if (isApiNotFoundError(error)) {
      return null
    }

    throw error
  }
})

const journal = computed(() => journalData.value?.data ?? null)
const attendees = computed(() => attendeesData.value?.data ?? [])
const sessions = computed(() => sessionsData.value?.data ?? [])
const attendanceEntries = computed(() => attendanceData.value?.data ?? [])
const attendanceScan = computed<JournalAttendanceScan | null>(
  () => attendanceScanData.value?.data ?? null
)
const signedScan = computed<JournalSignedScan | null>(
  () => signedScanData.value?.data ?? null
)
const journalPdfDownloadUrl = computed(() => `/api/v1/journals/${journalId}/pdf`)
const journalAttendanceScanDownloadUrl = computed(
  () => `/api/v1/journals/${journalId}/attendance-scan`
)
const journalSignedScanDownloadUrl = computed(
  () => `/api/v1/journals/${journalId}/signed-scan`
)
const journalsListLink = computed<RouteLocationRaw>(() => ({
  path: '/journals',
  query: {
    ...(typeof route.query.search === 'string' && route.query.search ? { search: route.query.search } : {}),
    ...(typeof route.query.status === 'string' && route.query.status ? { status: route.query.status } : {}),
    ...(typeof route.query.dateFrom === 'string' && route.query.dateFrom ? { dateFrom: route.query.dateFrom } : {}),
    ...(typeof route.query.dateTo === 'string' && route.query.dateTo ? { dateTo: route.query.dateTo } : {})
  }
}))
const editJournalLink = computed<RouteLocationRaw>(() => ({
  path: `/journals/${journalId}/edit`,
  query: {
    ...(typeof route.query.search === 'string' && route.query.search ? { search: route.query.search } : {}),
    ...(typeof route.query.status === 'string' && route.query.status ? { status: route.query.status } : {}),
    ...(typeof route.query.dateFrom === 'string' && route.query.dateFrom ? { dateFrom: route.query.dateFrom } : {}),
    ...(typeof route.query.dateTo === 'string' && route.query.dateTo ? { dateTo: route.query.dateTo } : {})
  }
}))
const attendeeCount = computed(() => attendees.value.length)
const sessionCount = computed(() => sessions.value.length)
const isClosed = computed(() => journal.value?.status === 'closed')
const formattedJournalTotalHours = computed(() => {
  if (!journal.value) {
    return ''
  }

  return formatHours(journal.value.totalHours)
})
const trimmedStudentSearch = computed(() => studentSearch.value.trim())
const {
  options: studentOptions,
  pending: studentsPending,
  error: studentSearchError,
  showNoResults: showNoStudentResults
} = useSearchableSelect<StudentSummary>({
  query: studentSearch,
  fetchOptions: async (search) => {
    if (isClosed.value) {
      return []
    }

    const response = await api.students({
      search,
      companyId: journal.value?.companyId ?? undefined,
      limit: 100
    })

    return response.data
  },
  getOptionLabel: student => `${student.lastName} ${student.firstName}`.trim(),
  getErrorMessage: error => getApiErrorMessage(error, 'Nie udało się pobrać listy kursantów.')
})
const alreadyAddedStudentIds = computed(() => {
  return new Set(attendees.value.map(attendee => attendee.studentId))
})
const availableStudentOptions = computed(() => {
  return studentOptions.value.filter(student => !alreadyAddedStudentIds.value.has(student.id))
})
const showNoAvailableStudentResults = computed(() => {
  return (
    showNoStudentResults.value
    || (trimmedStudentSearch.value.length >= 2
      && !studentsPending.value
      && !studentSearchError.value
      && availableStudentOptions.value.length === 0)
  )
})

watch(
  () => journal.value?.courseId,
  (courseId) => {
    if (!courseId) {
      printCourse.value = null
      return
    }

    void ensurePrintCourseLoaded(courseId)
  },
  { immediate: true }
)

watch(
  () => route.query.created,
  (value) => {
    showCreatedNotice.value = value === '1'
  }
)

watch(
  sessions,
  (value) => {
    const nextDrafts: Record<number, { sessionDate: string, trainerName: string }> = {}

    for (const session of value) {
      nextDrafts[session.id] = {
        sessionDate: session.sessionDate || journal.value?.dateStart || '',
        trainerName: session.trainerName
      }
    }

    sessionDrafts.value = nextDrafts
    sessionSaveErrors.value = {}
  },
  { immediate: true }
)

watch(
  [sessions, attendees, attendanceEntries],
  ([sessionList, attendeeList, entryList]) => {
    const nextDrafts: Record<string, boolean> = {}

    for (const session of sessionList) {
      for (const attendee of attendeeList) {
        nextDrafts[attendanceKey(session.id, attendee.id)] = false
      }
    }

    for (const entry of entryList) {
      nextDrafts[attendanceKey(entry.journalSessionId, entry.journalAttendeeId)] = entry.present
    }

    attendanceDrafts.value = nextDrafts
  },
  { immediate: true }
)

watch(studentSearch, () => {
  addAttendeeError.value = ''
  addAttendeeSuccess.value = ''
  deleteAttendeeError.value = ''
  deleteAttendeeSuccess.value = ''
})

function formatHours(value: number) {
  return `${value
    .toString()
    .replace(/\.0+$/, '')
    .replace(/(\.\d*[1-9])0+$/, '$1')} h`
}

function isApiNotFoundError(error: unknown) {
  if (error && typeof error === 'object' && 'statusCode' in error && error.statusCode === 404) {
    return true
  }

  if (
    error
    && typeof error === 'object'
    && 'response' in error
    && error.response
    && typeof error.response === 'object'
    && 'status' in error.response
    && error.response.status === 404
  ) {
    return true
  }

  if (
    error
    && typeof error === 'object'
    && 'data' in error
    && error.data
    && typeof error.data === 'object'
    && 'error' in error.data
    && error.data.error
    && typeof error.data.error === 'object'
    && 'code' in error.data.error
    && error.data.error.code === 'not_found'
  ) {
    return true
  }

  return false
}

function formatPrintDate(value: string | null) {
  if (!value) {
    return 'Brak'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return parsed.toLocaleDateString('pl-PL')
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll('\'', '&#39;')
}

function shortenAttendanceTopic(topic: string, maxLength = 40) {
  const normalized = topic.trim()

  if (normalized.length <= maxLength) {
    return normalized
  }

  const shortened = normalized.slice(0, maxLength).trimEnd()
  const lastSpace = shortened.lastIndexOf(' ')
  const safeCut
    = lastSpace > Math.floor(maxLength * 0.6) ? shortened.slice(0, lastSpace) : shortened

  return `${safeCut}...`
}

function formatCertificateNumber(certificate: {
  registryNumber: number
  courseSymbol: string
  registryYear: number
}) {
  return `${certificate.registryNumber}/${certificate.courseSymbol}/${certificate.registryYear}`
}

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

const printCourseProgramEntries = computed(() => {
  if (!printCourse.value?.courseProgram) {
    return [] as CourseProgramEntry[]
  }

  try {
    const parsed = JSON.parse(printCourse.value.courseProgram)
    if (!Array.isArray(parsed)) {
      return [] as CourseProgramEntry[]
    }

    return parsed as CourseProgramEntry[]
  } catch {
    return [] as CourseProgramEntry[]
  }
})

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

const printProgramSplitBySortOrder = computed(() => {
  const map: Record<number, { theory: string, practice: string }> = {}

  for (const session of sessions.value) {
    const entry = printCourseProgramEntries.value[session.sortOrder - 1]

    if (!entry) {
      map[session.sortOrder] = {
        theory: formatProgramHours(session.hours),
        practice: '0'
      }
      continue
    }

    map[session.sortOrder] = {
      theory: formatProgramHours(entry.TheoryTime),
      practice: formatProgramHours(entry.PracticeTime)
    }
  }

  return map
})

const attendancePrintDays = computed(() => {
  const days = new Map<string, { date: string, totalHours: number }>()

  for (const session of sessions.value) {
    const dateKey = session.sessionDate || ''
    if (!dateKey) {
      continue
    }

    const hours = Number.parseFloat(`${session.hours}`.replace(',', '.'))
    const current = days.get(dateKey)
    if (current) {
      current.totalHours += Number.isFinite(hours) ? hours : 0
      continue
    }

    days.set(dateKey, {
      date: session.sessionDate,
      totalHours: Number.isFinite(hours) ? hours : 0
    })
  }

  return Array.from(days.entries()).map(([key, value], index) => ({
    key,
    order: index + 1,
    date: value.date,
    totalHours: value.totalHours
  }))
})

function certificateMatchesJournal(certificate: StudentCertificateSummary) {
  return (
    certificate.courseSymbol === journal.value?.courseSymbol
    || certificate.courseName === journal.value?.courseName
  )
}

function hasSessionChanges(session: JournalSession) {
  const draft = sessionDrafts.value[session.id]

  if (!draft) {
    return false
  }

  return (
    draft.sessionDate !== session.sessionDate || draft.trainerName.trim() !== session.trainerName
  )
}

function attendanceKey(sessionId: number, attendeeId: number) {
  return `${sessionId}:${attendeeId}`
}

function attendanceValue(sessionId: number, attendeeId: number) {
  return attendanceDrafts.value[attendanceKey(sessionId, attendeeId)] ?? false
}

function resetAttendanceScanSelection() {
  attendanceScanFile.value = null
}

function resetSignedScanSelection() {
  signedScanFile.value = null
}

function openUploadPicker(target: 'attendance' | 'signed') {
  activeUploadPickerTarget.value = target

  if (uploadPickerRef.value) {
    uploadPickerRef.value.value = ''
    uploadPickerRef.value.click()
  }
}

function onSharedUploadFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0] ?? null

  if (!file || !activeUploadPickerTarget.value) {
    return
  }

  if (activeUploadPickerTarget.value === 'attendance') {
    attendanceScanActionError.value = ''
    attendanceScanActionSuccess.value = ''
    attendanceScanFile.value = file
  } else {
    signedScanActionError.value = ''
    signedScanActionSuccess.value = ''
    signedScanFile.value = file
  }

  activeUploadPickerTarget.value = null
}

function onUpdateAttendeeCertificateDraft(payload: { attendeeId: number, value: string }) {
  attendeeCertificateDrafts.value = {
    ...attendeeCertificateDrafts.value,
    [payload.attendeeId]: payload.value
  }
}

function onUpdateSessionDraft(payload: {
  sessionId: number
  sessionDate?: string
  trainerName?: string
}) {
  const current = sessionDrafts.value[payload.sessionId]

  if (!current) {
    return
  }

  sessionDrafts.value = {
    ...sessionDrafts.value,
    [payload.sessionId]: {
      sessionDate: payload.sessionDate ?? current.sessionDate,
      trainerName: payload.trainerName ?? current.trainerName
    }
  }
}

async function withPreservedScroll(action: () => Promise<void>) {
  if (!import.meta.client) {
    await action()
    return
  }

  const previousScrollY = window.scrollY
  await action()
  await nextTick()
  window.scrollTo({
    top: previousScrollY,
    behavior: 'auto'
  })
}

async function dismissCreatedNotice() {
  showCreatedNotice.value = false

  if (route.query.created !== '1') {
    return
  }

  const nextQuery = {
    ...route.query
  }

  delete nextQuery.created

  await router.replace({
    path: `/journals/${journalId}`,
    query: nextQuery
  })
}

async function onRefreshAttendees() {
  await withPreservedScroll(async () => {
    await refreshAttendees()
  })
}

async function onRefreshSessions() {
  await withPreservedScroll(async () => {
    await refreshSessions()
  })
}

async function onRefreshAttendance() {
  await withPreservedScroll(async () => {
    await refreshAttendance()
  })
}

async function ensurePrintCourseLoaded(courseId: number) {
  if (printCourse.value?.id === courseId) {
    return true
  }

  const requestId = ++printCourseRequestId
  loadingPrintCourse.value = true

  try {
    const response = await api.course(courseId)
    if (requestId !== printCourseRequestId) {
      return false
    }

    printCourse.value = response.data
    return true
  } catch {
    if (requestId !== printCourseRequestId) {
      return false
    }

    printCourse.value = null
    return false
  } finally {
    if (requestId === printCourseRequestId) {
      loadingPrintCourse.value = false
    }
  }
}

async function onUploadAttendanceScan() {
  if (!attendanceScanFile.value) {
    attendanceScanActionError.value = 'Wybierz plik do załączenia.'
    return
  }

  const hadExistingScan = Boolean(attendanceScan.value)
  attendanceScanActionError.value = ''
  attendanceScanActionSuccess.value = ''
  attendanceScanUploadPending.value = true

  try {
    await api.uploadJournalAttendanceScan(journalId, attendanceScanFile.value)
    await withPreservedScroll(async () => {
      await refreshAttendanceScan()
    })
    attendanceScanActionSuccess.value = hadExistingScan
      ? 'Podmieniono skan podpisanej listy obecności.'
      : 'Załączono skan podpisanej listy obecności.'
    resetAttendanceScanSelection()
  } catch (error) {
    attendanceScanActionError.value = getApiErrorMessage(
      error,
      'Nie udało się załączyć skanu podpisanej listy obecności.'
    )
  } finally {
    attendanceScanUploadPending.value = false
  }
}

async function onDeleteAttendanceScan() {
  if (!attendanceScan.value) {
    return
  }

  if (!window.confirm(`Usunąć skan „${attendanceScan.value.fileName}”?`)) {
    return
  }

  attendanceScanActionError.value = ''
  attendanceScanActionSuccess.value = ''
  attendanceScanDeletePending.value = true

  try {
    await api.deleteJournalAttendanceScan(journalId)
    await withPreservedScroll(async () => {
      await refreshAttendanceScan()
    })
    attendanceScanActionSuccess.value = 'Usunięto skan podpisanej listy obecności.'
    resetAttendanceScanSelection()
  } catch (error) {
    attendanceScanActionError.value = getApiErrorMessage(
      error,
      'Nie udało się usunąć skanu podpisanej listy obecności.'
    )
  } finally {
    attendanceScanDeletePending.value = false
  }
}

async function onUploadSignedScan() {
  if (!signedScanFile.value) {
    signedScanActionError.value = 'Wybierz plik do załączenia.'
    return
  }

  const hadExistingScan = Boolean(signedScan.value)
  signedScanActionError.value = ''
  signedScanActionSuccess.value = ''
  signedScanUploadPending.value = true

  try {
    await api.uploadJournalSignedScan(journalId, signedScanFile.value)
    await withPreservedScroll(async () => {
      await refreshSignedScan()
    })
    signedScanActionSuccess.value = hadExistingScan
      ? 'Podmieniono skan podpisanego dziennika.'
      : 'Załączono skan podpisanego dziennika.'
    resetSignedScanSelection()
  } catch (error) {
    signedScanActionError.value = getApiErrorMessage(
      error,
      'Nie udało się załączyć skanu podpisanego dziennika.'
    )
  } finally {
    signedScanUploadPending.value = false
  }
}

async function onDeleteSignedScan() {
  if (!signedScan.value) {
    return
  }

  if (!window.confirm(`Usunąć skan „${signedScan.value.fileName}”?`)) {
    return
  }

  signedScanActionError.value = ''
  signedScanActionSuccess.value = ''
  signedScanDeletePending.value = true

  try {
    await api.deleteJournalSignedScan(journalId)
    await withPreservedScroll(async () => {
      await refreshSignedScan()
    })
    signedScanActionSuccess.value = 'Usunięto skan podpisanego dziennika.'
    resetSignedScanSelection()
  } catch (error) {
    signedScanActionError.value = getApiErrorMessage(
      error,
      'Nie udało się usunąć skanu podpisanego dziennika.'
    )
  } finally {
    signedScanDeletePending.value = false
  }
}

async function onDeleteJournal() {
  if (!journal.value) {
    return
  }

  if (!window.confirm(`Czy na pewno usunąć dziennik „${journal.value.title}”?`)) {
    return
  }

  deleteJournalError.value = ''
  deleteJournalPending.value = true

  try {
    await api.deleteJournal(journalId)
    await navigateTo({
      path: '/journals',
      query: {
        ...(typeof route.query.search === 'string' && route.query.search ? { search: route.query.search } : {}),
        ...(typeof route.query.status === 'string' && route.query.status ? { status: route.query.status } : {}),
        ...(typeof route.query.dateFrom === 'string' && route.query.dateFrom ? { dateFrom: route.query.dateFrom } : {}),
        ...(typeof route.query.dateTo === 'string' && route.query.dateTo ? { dateTo: route.query.dateTo } : {}),
        deleted: '1'
      }
    })
  } catch (error) {
    deleteJournalError.value = getApiErrorMessage(error, 'Nie udało się usunąć dziennika.')
  } finally {
    deleteJournalPending.value = false
  }
}

function openPrintJournal() {
  void printJournal()
}

function openPrintAttendanceList() {
  void printAttendanceList()
}

const journalPrintDocument = computed(() => {
  if (!journal.value) {
    return ''
  }

  const participantRowsHtml = attendees.value
    .map(
      attendee => `
      <tr>
        <td>${attendee.sortOrder}</td>
        <td>${escapeHtml(attendee.fullNameSnapshot)}</td>
        <td>${escapeHtml(formatPrintDate(attendee.birthdateSnapshot))}</td>
        <td>${escapeHtml(attendee.companyNameSnapshot || 'Brak firmy')}</td>
        <td>${escapeHtml(attendee.certificate ? formatCertificateNumber(attendee.certificate) : 'Brak')}</td>
      </tr>
    `
    )
    .join('')

  const programRowsHtml = sessions.value
    .map(
      session => `
      <tr>
        <td>${session.sortOrder}</td>
        <td>${escapeHtml(formatPrintDate(session.sessionDate))}</td>
        <td>${escapeHtml(session.topic)}</td>
        <td>${escapeHtml(printProgramSplitBySortOrder.value[session.sortOrder]?.theory || '0')}</td>
        <td>${escapeHtml(printProgramSplitBySortOrder.value[session.sortOrder]?.practice || '0')}</td>
        <td>${escapeHtml(session.trainerName)}</td>
      </tr>
    `
    )
    .join('')

  const standardAttendanceHeadHtml = sessions.value
    .map(
      session => `
      <th title="${escapeHtml(session.topic)}">
        <div class="attendance-heading">
          <div>${session.sortOrder}</div>
          <div>${escapeHtml(formatPrintDate(session.sessionDate))}</div>
          <div>${escapeHtml(shortenAttendanceTopic(session.topic, 22))}</div>
        </div>
      </th>
    `
    )
    .join('')

  const standardAttendanceRowsHtml = attendees.value
    .map(
      attendee => `
      <tr>
        <td class="attendee-cell">${escapeHtml(attendee.fullNameSnapshot)}</td>
        ${sessions.value
          .map(
            session =>
              `<td class="text-center">${attendanceValue(session.id, attendee.id) ? 'X' : ''}</td>`
          )
          .join('')}
      </tr>
    `
    )
    .join('')

  return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>${escapeHtml(journal.value.title)}</title>
    <style>
      @page {
        size: A4 portrait;
        margin: 14mm;
      }

      @page attendance-landscape {
        size: A4 landscape;
        margin: 12mm;
      }

      * {
        box-sizing: border-box;
      }

      html,
      body {
        margin: 0;
        padding: 0;
        background: white;
        color: #0f172a;
        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }

      .document {
        padding: 12px;
      }

      .print-sheet {
        page-break-after: always;
        break-after: page;
      }

      .print-sheet:last-of-type {
        page-break-after: auto;
        break-after: auto;
      }

      .sheet-header {
        margin-bottom: 18px;
        padding-bottom: 14px;
        border-bottom: 1px solid #cbd5e1;
      }

      .status-badge {
        display: inline-flex;
        margin-bottom: 10px;
        border: 1px solid #cbd5e1;
        border-radius: 999px;
        padding: 4px 10px;
        font-size: 11px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.12em;
        color: #475569;
      }

      .course-symbol {
        margin-left: 8px;
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.18em;
        color: #64748b;
      }

      h1,
      h2 {
        margin: 0;
        color: #0f172a;
      }

      h1 {
        font-size: 28px;
        line-height: 1.2;
      }

      h2 {
        font-size: 20px;
        line-height: 1.25;
      }

      .subtitle {
        margin-top: 6px;
        font-size: 15px;
        color: #475569;
      }

      .section-lead {
        margin: 4px 0 14px;
        font-size: 12px;
        color: #64748b;
      }

      .details-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px 18px;
        font-size: 13px;
      }

      .details-grid dt {
        margin: 0 0 4px;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.14em;
        color: #64748b;
      }

      .details-grid dd {
        margin: 0;
        color: #0f172a;
      }

      .full-width {
        grid-column: 1 / -1;
      }

      .print-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
      }

      .print-table th,
      .print-table td {
        border: 1px solid #cbd5e1;
        padding: 8px 10px;
        vertical-align: top;
      }

      .print-table thead th {
        background: #f8fafc;
        font-size: 8px;
        font-weight: 600;
        text-transform: capitalize;
        letter-spacing: 0.03em;
        color: #475569;
      }

      .trainer-signature {
        padding-top: 96px;
      }

      .trainer-signature__line {
        width: 220px;
        border-top: 1px solid #64748b;
        padding-top: 6px;
        font-size: 11px;
        text-align: center;
        color: #475569;
      }

      .attendance-sheet {
        page: attendance-landscape;
        page-break-before: always;
        break-before: page;
      }

      .attendance-table th:not(:first-child),
      .attendance-table td:not(:first-child) {
        min-width: 64px;
        width: 64px;
        text-align: center;
      }

      .attendance-table th:first-child,
      .attendance-table td:first-child {
        min-width: 165px;
        width: 165px;
      }

      .attendance-heading {
        font-size: 8px;
        line-height: 1.2;
      }

      .attendance-signature-cell {
        height: 44px;
      }

      .attendee-cell {
        font-weight: 500;
      }

      @media print {
        html,
        body {
          background: white !important;
        }

        .document {
          padding: 0;
        }
      }
    </style>
  </head>
  <body>
    <div class="document">
      <article class="print-sheet">
        <div class="sheet-header">
          <div class="status-badge">${journal.value.status === 'closed' ? 'Zamknięty' : 'Roboczy'}</div>
          <span class="course-symbol">${escapeHtml(journal.value.courseSymbol)}</span>
          <h1>${escapeHtml(journal.value.title)}</h1>
        </div>

        <dl class="details-grid">
          <div>
            <dt>Organizator</dt>
            <dd>${escapeHtml(journal.value.organizerName)}</dd>
          </div>
          <div>
            <dt>Miejsce</dt>
            <dd>${escapeHtml(journal.value.location)}</dd>
          </div>
          <div>
            <dt>Forma szkolenia</dt>
            <dd>${escapeHtml(journal.value.formOfTraining)}</dd>
          </div>
          <div>
            <dt>Firma</dt>
            <dd>${escapeHtml(journal.value.companyName || 'Bez przypisanej firmy')}</dd>
          </div>
          <div>
            <dt>Termin</dt>
            <dd>${escapeHtml(formatPrintDate(journal.value.dateStart))} - ${escapeHtml(formatPrintDate(journal.value.dateEnd))}</dd>
          </div>
          <div>
            <dt>Liczba godzin</dt>
            <dd>${escapeHtml(formatHours(journal.value.totalHours))}</dd>
          </div>
          <div class="full-width">
            <dt>Podstawa prawna</dt>
            <dd>${escapeHtml(journal.value.legalBasis)}</dd>
          </div>
          <div class="full-width">
            <dt>Adres organizatora</dt>
            <dd>${escapeHtml(journal.value.organizerAddress || 'Brak adresu organizatora')}</dd>
          </div>
          <div class="full-width">
            <dt>Notatki</dt>
            <dd>${escapeHtml(journal.value.notes || 'Brak notatek')}</dd>
          </div>
        </dl>
      </article>

      <article class="print-sheet">
        <h2>Lista uczestników</h2>
        <p class="section-lead">Lista uczestników przypisanych do tego szkolenia.</p>
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
            ${participantRowsHtml}
          </tbody>
        </table>
      </article>

      <article class="print-sheet">
        <h2>Program szkolenia</h2>
        <p class="section-lead">Tematy, prowadzący i godziny przypisane do dziennika.</p>
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
            ${programRowsHtml}
          </tbody>
        </table>

        <div class="trainer-signature">
          <div class="trainer-signature__line">Podpis wykładowcy</div>
        </div>
      </article>

      <article class="print-sheet attendance-sheet">
        <h2>Lista obecności</h2>
        <p class="section-lead">Obecność uczestników dla poszczególnych pozycji programu.</p>
        <table class="print-table attendance-table">
          <thead>
            <tr>
              <th>Uczestnik</th>
              ${standardAttendanceHeadHtml}
            </tr>
          </thead>
          <tbody>
            ${standardAttendanceRowsHtml}
          </tbody>
        </table>
      </article>
    </div>
  </body>
</html>`
})

const attendanceListPrintDocument = computed(() => {
  if (!journal.value) {
    return ''
  }

  const attendanceHeadHtml = attendancePrintDays.value
    .map(
      day => `
      <th>
        <div class="attendance-heading">
          <div>Dzień ${day.order}</div>
          <div>${escapeHtml(formatPrintDate(day.date))}</div>
          <div>${escapeHtml(formatHours(day.totalHours))}</div>
        </div>
      </th>
    `
    )
    .join('')

  const attendeeRowsHtml = attendees.value
    .map(
      attendee => `
      <tr>
        <td class="attendee-cell">${escapeHtml(attendee.fullNameSnapshot)}</td>
        ${attendancePrintDays.value
          .map(() => '<td class="attendance-signature-cell"></td>')
          .join('')}
      </tr>
    `
    )
    .join('')

  const blankRowsHtml = Array.from(
    { length: 5 },
    (_, index) => `
      <tr>
        <td class="attendee-cell">${index === 0 ? '&nbsp;' : '&nbsp;'}</td>
        ${attendancePrintDays.value
          .map(() => '<td class="attendance-signature-cell"></td>')
          .join('')}
      </tr>
    `
  ).join('')

  return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Lista obecności - ${escapeHtml(journal.value.title)}</title>
    <style>
      @page {
        size: A4 landscape;
        margin: 12mm;
      }

      * {
        box-sizing: border-box;
      }

      html,
      body {
        margin: 0;
        padding: 0;
        background: white;
        color: #0f172a;
        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }

      .document {
        padding: 12px;
      }

      .sheet-header {
        margin-bottom: 18px;
        padding-bottom: 14px;
        border-bottom: 1px solid #cbd5e1;
      }

      h1 {
        margin: 0;
        font-size: 28px;
        line-height: 1.2;
        color: #0f172a;
      }

      .subtitle {
        margin-top: 6px;
        font-size: 15px;
        color: #475569;
      }

      .meta {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
        gap: 12px;
        margin-top: 16px;
        font-size: 12px;
      }

      .meta-label {
        display: block;
        margin-bottom: 4px;
        font-size: 10px;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: #64748b;
      }

      .print-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
      }

      .print-table th,
      .print-table td {
        border: 1px solid #cbd5e1;
        padding: 8px 10px;
        vertical-align: top;
      }

      .print-table thead th {
        background: #f8fafc;
        font-size: 8px;
        font-weight: 600;
        text-transform: capitalize;
        letter-spacing: 0.03em;
        color: #475569;
      }

      .attendance-table th:first-child,
      .attendance-table td:first-child {
        min-width: 220px;
        width: 220px;
      }

      .attendance-table th:not(:first-child),
      .attendance-table td:not(:first-child) {
        min-width: 124px;
        width: 124px;
        text-align: center;
      }

      .attendance-heading {
        font-size: 8px;
        line-height: 1.2;
      }

      .attendance-signature-cell {
        height: 44px;
      }

      .attendee-cell {
        font-weight: 500;
      }

      @media print {
        html,
        body {
          background: white !important;
        }

        .document {
          padding: 0;
        }
      }
    </style>
  </head>
  <body>
    <div class="document">
      <div class="sheet-header">
        <h1>Lista obecności</h1>
        <p class="subtitle">${escapeHtml(journal.value.title)}</p>

        <div class="meta">
          <div>
            <span class="meta-label">Termin</span>
            <span>${escapeHtml(formatPrintDate(journal.value.dateStart))} - ${escapeHtml(formatPrintDate(journal.value.dateEnd))}</span>
          </div>
          <div>
            <span class="meta-label">Miejsce</span>
            <span>${escapeHtml(journal.value.location)}</span>
          </div>
          <div>
            <span class="meta-label">Organizator</span>
            <span>${escapeHtml(journal.value.organizerName)}</span>
          </div>
          <div>
            <span class="meta-label">Forma</span>
            <span>${escapeHtml(journal.value.formOfTraining)}</span>
          </div>
        </div>
      </div>

      <table class="print-table attendance-table">
        <thead>
          <tr>
            <th>Uczestnik</th>
            ${attendanceHeadHtml}
          </tr>
        </thead>
        <tbody>
          ${attendeeRowsHtml}
          ${blankRowsHtml}
        </tbody>
      </table>
    </div>
  </body>
</html>`
})

async function waitForPrintFrameLoad(frame: HTMLIFrameElement) {
  await new Promise<void>((resolve) => {
    const timeout = window.setTimeout(() => {
      frame.removeEventListener('load', onLoad)
      resolve()
    }, 400)

    function onLoad() {
      window.clearTimeout(timeout)
      resolve()
    }

    if (frame.contentDocument?.readyState === 'complete') {
      window.clearTimeout(timeout)
      resolve()
      return
    }

    frame.addEventListener('load', onLoad, { once: true })
  })
}

async function printCurrentDocument(documentHtml: string) {
  if (!import.meta.client) {
    return
  }

  printJournalError.value = ''
  printJournalPending.value = true

  try {
    if (!documentHtml) {
      printJournalError.value = 'Nie udało się przygotować widoku wydruku dziennika.'
      return
    }

    activePrintDocument.value = ''
    await nextTick()
    activePrintDocument.value = documentHtml
    await nextTick()

    const frame = printFrame.value
    if (!frame) {
      printJournalError.value = 'Nie udało się przygotować widoku wydruku dziennika.'
      return
    }

    await waitForPrintFrameLoad(frame)

    const frameWindow = frame.contentWindow
    if (!frameWindow) {
      printJournalError.value = 'Nie udało się przygotować widoku wydruku dziennika.'
      return
    }

    frameWindow.focus()
    frameWindow.print()
  } catch {
    printJournalError.value = 'Nie udało się przygotować widoku wydruku dziennika.'
  } finally {
    printJournalPending.value = false
  }
}

async function printJournal() {
  if (!journal.value || !import.meta.client) {
    return
  }

  printJournalError.value = ''
  await ensurePrintCourseLoaded(journal.value.courseId)
  await printCurrentDocument(journalPrintDocument.value)
}

async function printAttendanceList() {
  if (!journal.value || !import.meta.client) {
    return
  }

  printJournalError.value = ''
  await printCurrentDocument(attendanceListPrintDocument.value)
}

async function onCloseJournal() {
  if (!journal.value || isClosed.value) {
    return
  }

  if (
    !window.confirm(
      `Zamknąć dziennik „${journal.value.title}”? Po zamknięciu edycja zostanie zablokowana.`
    )
  ) {
    return
  }

  closeJournalError.value = ''
  closeJournalSuccess.value = ''
  closeJournalPending.value = true

  try {
    await api.closeJournal(journalId)
    await withPreservedScroll(async () => {
      await Promise.all([
        refreshJournal(),
        refreshAttendees(),
        refreshSessions(),
        refreshAttendance()
      ])
    })
    closeJournalSuccess.value
      = 'Dziennik został zamknięty. Możesz go teraz wydrukować jako finalną wersję.'
  } catch (error) {
    closeJournalError.value = getApiErrorMessage(error, 'Nie udało się zamknąć dziennika.')
  } finally {
    closeJournalPending.value = false
  }
}

async function onAddAttendee(student: StudentSummary) {
  addAttendeeError.value = ''
  addAttendeeSuccess.value = ''
  deleteAttendeeError.value = ''
  deleteAttendeeSuccess.value = ''
  attendeeCertificateError.value = ''
  attendeeCertificateSuccess.value = ''
  generateAttendeeCertificateError.value = ''
  generateAttendeeCertificateSuccess.value = ''
  addingStudentId.value = student.id

  try {
    await api.addJournalAttendee(journalId, {
      studentId: student.id
    })

    await withPreservedScroll(async () => {
      await Promise.all([refreshJournal(), refreshAttendees(), refreshAttendance()])
    })
    studentSearch.value = ''
    await nextTick()
    addAttendeeCardRef.value?.focusSearchInput()
    addAttendeeSuccess.value = `Dodano kursanta ${student.firstName} ${student.lastName} do dziennika.`
  } catch (error) {
    addAttendeeError.value = getApiErrorMessage(error, 'Nie udało się dodać kursanta do dziennika.')
  } finally {
    addingStudentId.value = null
  }
}

async function onDeleteAttendee(attendeeId: number, fullName: string) {
  deleteAttendeeError.value = ''
  deleteAttendeeSuccess.value = ''
  addAttendeeError.value = ''
  addAttendeeSuccess.value = ''
  attendeeCertificateError.value = ''
  attendeeCertificateSuccess.value = ''
  generateAttendeeCertificateError.value = ''
  generateAttendeeCertificateSuccess.value = ''
  deletingAttendeeId.value = attendeeId

  try {
    await api.deleteJournalAttendee(journalId, attendeeId)
    await withPreservedScroll(async () => {
      await Promise.all([refreshJournal(), refreshAttendees(), refreshAttendance()])
    })
    deleteAttendeeSuccess.value = `Usunięto kursanta ${fullName} z dziennika.`
  } catch (error) {
    deleteAttendeeError.value = getApiErrorMessage(
      error,
      'Nie udało się usunąć kursanta z dziennika.'
    )
  } finally {
    deletingAttendeeId.value = null
  }
}

async function onStartAttendeeCertificateEdit(attendee: JournalAttendee) {
  attendeeCertificateError.value = ''
  attendeeCertificateSuccess.value = ''
  editingCertificateAttendeeId.value = attendee.id
  attendeeCertificateDrafts.value = {
    ...attendeeCertificateDrafts.value,
    [attendee.id]: attendee.certificate ? String(attendee.certificate.id) : ''
  }

  if (attendeeCertificateOptions.value[attendee.id]) {
    return
  }

  loadingAttendeeCertificatesId.value = attendee.id

  try {
    const response = await api.studentCertificates(attendee.studentId)
    attendeeCertificateOptions.value = {
      ...attendeeCertificateOptions.value,
      [attendee.id]: response.data.filter(certificate => certificateMatchesJournal(certificate))
    }
  } catch (error) {
    attendeeCertificateError.value = getApiErrorMessage(
      error,
      'Nie udało się pobrać listy zaświadczeń kursanta.'
    )
  } finally {
    loadingAttendeeCertificatesId.value = null
  }
}

function onCancelAttendeeCertificateEdit() {
  editingCertificateAttendeeId.value = null
}

async function onGenerateAttendeeCertificate(attendee: JournalAttendee) {
  attendeeCertificateError.value = ''
  attendeeCertificateSuccess.value = ''
  generateAttendeeCertificateError.value = ''
  generateAttendeeCertificateSuccess.value = ''
  generatingAttendeeCertificateId.value = attendee.id

  try {
    const response = await api.generateJournalAttendeeCertificate(journalId, attendee.id)
    await withPreservedScroll(async () => {
      await refreshAttendees()
    })
    generateAttendeeCertificateSuccess.value = `Wystawiono zaświadczenie dla uczestnika ${attendee.fullNameSnapshot}. Numer dokumentu znajdziesz teraz w kolumnie zaświadczenia.`
    editingCertificateAttendeeId.value = null
    attendeeCertificateDrafts.value = {
      ...attendeeCertificateDrafts.value,
      [attendee.id]: String(response.data.id)
    }
  } catch (error) {
    generateAttendeeCertificateError.value = getApiErrorMessage(
      error,
      'Nie udało się wystawić zaświadczenia z dziennika.'
    )
  } finally {
    generatingAttendeeCertificateId.value = null
  }
}

async function onSaveAttendeeCertificate(attendee: JournalAttendee) {
  attendeeCertificateError.value = ''
  attendeeCertificateSuccess.value = ''
  savingAttendeeCertificateId.value = attendee.id

  const rawValue = attendeeCertificateDrafts.value[attendee.id] ?? ''
  const certificateId = rawValue ? Number.parseInt(rawValue, 10) : null
  const hasValidCertificateId = typeof certificateId === 'number' && Number.isFinite(certificateId) && certificateId > 0

  if (rawValue && !hasValidCertificateId) {
    attendeeCertificateError.value = 'Wybierz poprawne zaświadczenie.'
    savingAttendeeCertificateId.value = null
    return
  }

  try {
    await api.updateJournalAttendeeCertificate(journalId, attendee.id, {
      certificateId
    })

    await withPreservedScroll(async () => {
      await refreshAttendees()
    })
    editingCertificateAttendeeId.value = null
    attendeeCertificateSuccess.value = certificateId
      ? `Podpięto zaświadczenie do uczestnika ${attendee.fullNameSnapshot}.`
      : `Odpięto zaświadczenie od uczestnika ${attendee.fullNameSnapshot}.`
  } catch (error) {
    attendeeCertificateError.value = getApiErrorMessage(
      error,
      'Nie udało się zaktualizować powiązania zaświadczenia.'
    )
  } finally {
    savingAttendeeCertificateId.value = null
  }
}

async function onDetachAttendeeCertificate(attendee: JournalAttendee) {
  attendeeCertificateDrafts.value = {
    ...attendeeCertificateDrafts.value,
    [attendee.id]: ''
  }

  await onSaveAttendeeCertificate(attendee)
}

async function onGenerateSessions() {
  generateSessionsError.value = ''
  generateSessionsSuccess.value = ''
  sessionUpdateSuccess.value = ''
  generatingSessions.value = true

  try {
    const response = await api.generateJournalSessionsFromCourse(journalId)
    await withPreservedScroll(async () => {
      await Promise.all([refreshJournal(), refreshSessions(), refreshAttendance()])
    })
    generateSessionsSuccess.value = `Dodano ${response.data.generatedCount} pozycji programu szkolenia.`
  } catch (error) {
    generateSessionsError.value = getApiErrorMessage(
      error,
      'Nie udało się uzupełnić programu szkolenia.'
    )
  } finally {
    generatingSessions.value = false
  }
}

async function onSaveSession(session: JournalSession) {
  const draft = sessionDrafts.value[session.id]
  if (!draft) {
    return
  }

  const trainerName = draft.trainerName.trim()
  sessionUpdateSuccess.value = ''
  sessionSaveErrors.value = {
    ...sessionSaveErrors.value,
    [session.id]: ''
  }

  if (!draft.sessionDate || !trainerName) {
    sessionSaveErrors.value = {
      ...sessionSaveErrors.value,
      [session.id]: 'Data i prowadzący są wymagane.'
    }
    return
  }

  savingSessionId.value = session.id

  try {
    await api.updateJournalSession(journalId, session.id, {
      sessionDate: draft.sessionDate,
      trainerName
    })

    await withPreservedScroll(async () => {
      await refreshSessions()
    })
    sessionUpdateSuccess.value = `Zapisano zmiany w pozycji „${session.topic}”.`
  } catch (error) {
    sessionSaveErrors.value = {
      ...sessionSaveErrors.value,
      [session.id]: getApiErrorMessage(error, 'Nie udało się zapisać zmian w pozycji programu.')
    }
  } finally {
    savingSessionId.value = null
  }
}

async function onToggleAttendance(sessionId: number, attendeeId: number, present: boolean) {
  const key = attendanceKey(sessionId, attendeeId)
  const previous = attendanceValue(sessionId, attendeeId)

  attendanceSaveError.value = ''
  attendanceSaveSuccess.value = ''
  attendanceDrafts.value = {
    ...attendanceDrafts.value,
    [key]: present
  }
  savingAttendanceKey.value = key

  try {
    await api.updateJournalAttendance(journalId, {
      journalSessionId: sessionId,
      journalAttendeeId: attendeeId,
      present
    })

    await withPreservedScroll(async () => {
      await refreshAttendance()
    })
    attendanceSaveSuccess.value = 'Zapisano obecność.'
  } catch (error) {
    attendanceDrafts.value = {
      ...attendanceDrafts.value,
      [key]: previous
    }
    attendanceSaveError.value = getApiErrorMessage(error, 'Nie udało się zapisać obecności.')
  } finally {
    savingAttendanceKey.value = null
  }
}

async function onSetAttendanceForAttendee(attendeeId: number, present: boolean) {
  if (sessions.value.length === 0) {
    return
  }

  attendanceSaveError.value = ''
  attendanceSaveSuccess.value = ''
  bulkSavingAttendeeId.value = attendeeId

  const previousValues = sessions.value.map(session => ({
    key: attendanceKey(session.id, attendeeId),
    value: attendanceValue(session.id, attendeeId)
  }))

  const changedSessions = sessions.value.filter(
    session => attendanceValue(session.id, attendeeId) !== present
  )

  if (changedSessions.length === 0) {
    bulkSavingAttendeeId.value = null
    return
  }

  const nextDrafts = {
    ...attendanceDrafts.value
  }
  for (const session of changedSessions) {
    nextDrafts[attendanceKey(session.id, attendeeId)] = present
  }
  attendanceDrafts.value = nextDrafts

  try {
    await Promise.all(
      changedSessions.map(async (session) => {
        await api.updateJournalAttendance(journalId, {
          journalSessionId: session.id,
          journalAttendeeId: attendeeId,
          present
        })
      })
    )

    await withPreservedScroll(async () => {
      await refreshAttendance()
    })
    attendanceSaveSuccess.value = present
      ? 'Zaznaczono obecność dla wszystkich pozycji uczestnika.'
      : 'Wyczyszczono obecność dla wszystkich pozycji uczestnika.'
  } catch (error) {
    const restoredDrafts = {
      ...attendanceDrafts.value
    }
    for (const entry of previousValues) {
      restoredDrafts[entry.key] = entry.value
    }
    attendanceDrafts.value = restoredDrafts
    attendanceSaveError.value = getApiErrorMessage(
      error,
      'Nie udało się zapisać obecności dla całego wiersza.'
    )
  } finally {
    bulkSavingAttendeeId.value = null
  }
}
</script>

<template>
  <section class="space-y-8">
    <div
      v-if="showCreatedNotice"
      class="flex flex-col gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-5 py-4 text-sm text-emerald-800 sm:flex-row sm:items-center sm:justify-between"
    >
      <p>
        Dziennik został utworzony. Możesz teraz dodać uczestników i uzupełniać kolejne elementy
        dokumentacji.
      </p>

      <button
        type="button"
        class="inline-flex items-center justify-center rounded-lg border border-emerald-300 bg-white px-4 py-2 font-medium text-emerald-700 transition hover:border-emerald-400 hover:text-emerald-900"
        @click="dismissCreatedNotice"
      >
        Zamknij komunikat
      </button>
    </div>

    <div
      v-if="journalError"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać szczegółów dziennika.
    </div>

    <div
      v-else-if="journalPending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie dziennika...
    </div>

    <template v-else-if="journal">
      <JournalHeaderPanel
        :journal="journal"
        :attendee-count="attendeeCount"
        :session-count="sessionCount"
        :formatted-total-hours="formattedJournalTotalHours"
        :print-journal-pending="printJournalPending"
        :close-journal-pending="closeJournalPending"
        :is-closed="isClosed"
        :delete-journal-pending="deleteJournalPending"
        :journal-pdf-download-url="journalPdfDownloadUrl"
        :journals-list-link="journalsListLink"
        :edit-journal-link="editJournalLink"
        @print-journal="openPrintJournal"
        @print-attendance-list="openPrintAttendanceList"
        @close-journal="onCloseJournal"
        @delete-journal="onDeleteJournal"
      />

      <div class="space-y-6">
        <div
          v-if="deleteJournalError"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          {{ deleteJournalError }}
        </div>

        <div
          v-if="printJournalError"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          {{ printJournalError }}
        </div>

        <div
          v-if="closeJournalError"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          {{ closeJournalError }}
        </div>

        <div
          v-if="closeJournalSuccess"
          class="rounded-xl border border-emerald-200 bg-emerald-50 px-5 py-4 text-sm text-emerald-700"
        >
          {{ closeJournalSuccess }}
        </div>

        <JournalBasicInfoCard :journal="journal" />

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem] xl:items-start">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
            <input
              ref="uploadPickerRef"
              type="file"
              accept=".pdf,image/png,image/jpeg,application/pdf"
              class="hidden"
              @change="onSharedUploadFileChange"
            >

            <div class="space-y-1">
              <h2 class="text-lg font-semibold text-slate-900">
                Załączniki dziennika
              </h2>
              <p class="text-xs leading-5 text-slate-500">
                Podpisana lista obecności i podpisany dziennik.
              </p>
            </div>

            <div class="mt-4 grid gap-3 lg:grid-cols-2">
              <JournalAttendanceScanCard
                embedded
                title="Podpisana lista obecności"
                description="PDF lub zdjęcie listy obecności."
                load-error-message="Nie udało się pobrać informacji o skanie listy obecności."
                empty-state-message="Nie załączono jeszcze skanu podpisanej listy obecności."
                :scan="attendanceScan"
                :pending="attendanceScanPending"
                :has-error="Boolean(attendanceScanError)"
                :action-error="attendanceScanActionError"
                :action-success="attendanceScanActionSuccess"
                :upload-pending="attendanceScanUploadPending"
                :delete-pending="attendanceScanDeletePending"
                :selected-file-name="attendanceScanFile?.name || ''"
                :download-url="journalAttendanceScanDownloadUrl"
                @choose-file="openUploadPicker('attendance')"
                @upload="onUploadAttendanceScan"
                @clear-selection="resetAttendanceScanSelection"
                @delete-scan="onDeleteAttendanceScan"
              />

              <JournalAttendanceScanCard
                embedded
                title="Podpisany dziennik"
                description="PDF lub zdjęcie podpisanego dziennika."
                load-error-message="Nie udało się pobrać informacji o skanie podpisanego dziennika."
                empty-state-message="Nie załączono jeszcze skanu podpisanego dziennika."
                :scan="signedScan"
                :pending="signedScanPending"
                :has-error="Boolean(signedScanError)"
                :action-error="signedScanActionError"
                :action-success="signedScanActionSuccess"
                :upload-pending="signedScanUploadPending"
                :delete-pending="signedScanDeletePending"
                :selected-file-name="signedScanFile?.name || ''"
                :download-url="journalSignedScanDownloadUrl"
                @choose-file="openUploadPicker('signed')"
                @upload="onUploadSignedScan"
                @clear-selection="resetSignedScanSelection"
                @delete-scan="onDeleteSignedScan"
              />
            </div>
          </section>

          <div class="xl:sticky xl:top-4">
            <JournalTechnicalInfoCard :journal="journal" />
          </div>
        </div>

        <JournalAddAttendeeCard
          ref="addAttendeeCardRef"
          :journal="journal"
          :is-closed="isClosed"
          :student-search="studentSearch"
          :students-pending="studentsPending"
          :student-search-error="studentSearchError"
          :add-attendee-error="addAttendeeError"
          :add-attendee-success="addAttendeeSuccess"
          :show-no-student-results="showNoAvailableStudentResults"
          :available-student-options="availableStudentOptions"
          :adding-student-id="addingStudentId"
          @update:student-search="studentSearch = $event"
          @add-attendee="onAddAttendee"
        />

        <JournalAttendeesSection
          :attendees="attendees"
          :attendees-pending="attendeesPending"
          :has-error="Boolean(attendeesError)"
          :delete-attendee-error="deleteAttendeeError"
          :delete-attendee-success="deleteAttendeeSuccess"
          :attendee-certificate-error="attendeeCertificateError"
          :attendee-certificate-success="attendeeCertificateSuccess"
          :generate-attendee-certificate-error="generateAttendeeCertificateError"
          :generate-attendee-certificate-success="generateAttendeeCertificateSuccess"
          :editing-certificate-attendee-id="editingCertificateAttendeeId"
          :loading-attendee-certificates-id="loadingAttendeeCertificatesId"
          :saving-attendee-certificate-id="savingAttendeeCertificateId"
          :generating-attendee-certificate-id="generatingAttendeeCertificateId"
          :attendee-certificate-drafts="attendeeCertificateDrafts"
          :attendee-certificate-options="attendeeCertificateOptions"
          :deleting-attendee-id="deletingAttendeeId"
          :is-closed="isClosed"
          @refresh="onRefreshAttendees"
          @start-certificate-edit="onStartAttendeeCertificateEdit"
          @cancel-certificate-edit="onCancelAttendeeCertificateEdit"
          @update-certificate-draft="onUpdateAttendeeCertificateDraft"
          @save-certificate="onSaveAttendeeCertificate"
          @generate-certificate="onGenerateAttendeeCertificate"
          @detach-certificate="onDetachAttendeeCertificate"
          @delete-attendee="onDeleteAttendee($event.attendeeId, $event.fullName)"
        />

        <JournalSessionsSection
          :sessions="sessions"
          :sessions-pending="sessionsPending"
          :has-error="Boolean(sessionsError)"
          :generate-sessions-error="generateSessionsError"
          :generate-sessions-success="generateSessionsSuccess"
          :session-update-success="sessionUpdateSuccess"
          :generating-sessions="generatingSessions"
          :session-drafts="sessionDrafts"
          :session-save-errors="sessionSaveErrors"
          :saving-session-id="savingSessionId"
          :is-closed="isClosed"
          :journal-date-start="journal.dateStart"
          :has-session-changes="hasSessionChanges"
          @refresh="onRefreshSessions"
          @generate-sessions="onGenerateSessions"
          @update-session-draft="onUpdateSessionDraft"
          @save-session="onSaveSession"
        />

        <JournalAttendanceSection
          :attendees="attendees"
          :sessions="sessions"
          :attendance-pending="attendancePending"
          :has-error="Boolean(attendanceError)"
          :attendance-save-error="attendanceSaveError"
          :attendance-save-success="attendanceSaveSuccess"
          :is-closed="isClosed"
          :journal-date-start="journal.dateStart"
          :saving-attendance-key="savingAttendanceKey"
          :bulk-saving-attendee-id="bulkSavingAttendeeId"
          :attendance-drafts="attendanceDrafts"
          @refresh="onRefreshAttendance"
          @set-attendance-for-attendee="
            onSetAttendanceForAttendee($event.attendeeId, $event.present)
          "
          @toggle-attendance="
            onToggleAttendance($event.sessionId, $event.attendeeId, $event.present)
          "
        />
      </div>

      <iframe
        v-if="activePrintDocument"
        ref="printFrame"
        :srcdoc="activePrintDocument"
        title="Ukryty wydruk dziennika"
        class="pointer-events-none fixed -left-24999.75 top-0 h-px w-px opacity-0"
      />
    </template>
  </section>
</template>
