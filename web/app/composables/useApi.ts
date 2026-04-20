export interface ApiErrorResponse {
  error: {
    code: string
    message: string
  }
}

export interface AuthUser {
  id: number
  email: string
  firstName: string
  lastName: string
  role: number
}

export interface AuthResponse {
  data: AuthUser
}

export interface AdminUser {
  id: number
  email: string
  firstName: string
  lastName: string
  role: number
}

export interface AdminUsersResponse {
  data: AdminUser[]
}

export interface CreateUserPayload {
  email: string
  password: string
  firstName: string
  lastName: string
  role: number
}

export interface UpdateUserPayload {
  email: string
  firstName: string
  lastName: string
  role: number
}

export interface AdminResetUserPasswordPayload {
  newPassword: string
}

export interface UpdateProfilePayload {
  email: string
  firstName: string
  lastName: string
}

export interface UpdatePasswordPayload {
  currentPassword: string
  newPassword: string
}

export interface DeleteUserResponse {
  data: {
    id: number
  }
}

export interface AuditLogEntry {
  id: number
  entityType: string
  entityId: number
  action: string
  actorUserId: number | null
  actorUserEmail: string | null
  actorUserName: string | null
  requestId: string | null
  before: unknown | null
  after: unknown | null
  metadata: unknown | null
  createdAt: string
}

export interface AuditLogResponse {
  data: AuditLogEntry[]
}

export interface LoginPayload {
  email: string
  password: string
}

export interface DashboardResponse {
  data: {
    stats: {
      students: number
      companies: number
      certificates: number
    }
    expiring: {
      in30Days: number
    }
    expiringCertificates: Array<{
      certificateId: number
      expiryDate: string
      studentName: string
      companyName: string
      courseName: string
      courseSymbol: string
      registryYear: number
      registryNumber: number
    }>
  }
}

export interface StudentSummary {
  id: number
  firstName: string
  lastName: string
  pesel: string | null
  birthDate: string
  company: {
    id: number
    name: string
  } | null
}

export interface StudentsResponse {
  data: StudentSummary[]
}

export interface CompanySummary {
  id: number
  name: string
  city: string
  nip: string
  contactPerson: string
  telephone: string
}

export interface CompaniesResponse {
  data: CompanySummary[]
}

export interface CompanyDetails {
  id: number
  name: string
  street: string
  city: string
  zipcode: string
  nip: string
  email: string | null
  contactPerson: string | null
  telephone: string
  note: string | null
}

export interface CompanyDetailsResponse {
  data: CompanyDetails
}

export interface GUSCompanyDetails {
  nip: string
  regon: string
  name: string
  voivodeship: string
  county: string
  commune: string
  city: string
  postalCode: string
  street: string
  houseNumber: string
  apartment: string
  status: string
}

export interface GUSCompanyDetailsResponse {
  data: GUSCompanyDetails
}

export interface UpdateCompanyPayload {
  name: string
  street: string
  city: string
  zipcode: string
  nip: string
  email: string | null
  contactPerson: string | null
  telephone: string
  note: string | null
}

export type CreateCompanyPayload = UpdateCompanyPayload

export interface CompanyStudentSummary {
  id: number
  firstname: string
  lastname: string
  secondname: string | null
  birthdate: string
  birthplace: string
  pesel: string | null
}

export interface CompanyStudentsResponse {
  data: CompanyStudentSummary[]
}

export interface StudentDetails {
  id: number
  firstName: string
  lastName: string
  secondName: string | null
  birthDate: string
  birthPlace: string
  pesel: string | null
  addressStreet: string | null
  addressCity: string | null
  addressZip: string | null
  telephone: string | null
  company: {
    id: number
    name: string
  } | null
}

export interface StudentDetailsResponse {
  data: StudentDetails
}

export interface UpdateStudentPayload {
  firstName: string
  lastName: string
  secondName: string | null
  birthDate: string
  birthPlace: string
  pesel: string | null
  addressStreet: string | null
  addressCity: string | null
  addressZip: string | null
  telephone: string | null
  companyId: number | null
}

export type CreateStudentPayload = UpdateStudentPayload

export interface StudentCertificateSummary {
  id: number
  date: string
  courseName: string
  courseSymbol: string
  registryYear: number
  registryNumber: number
  courseDateStart: string
  courseDateEnd: string | null
  expiryDate: string | null
}

export interface StudentCertificatesResponse {
  data: StudentCertificateSummary[]
}

export interface CourseSummary {
  id: number
  mainName: string
  name: string
  symbol: string
  expiryTime: string | null
}

export interface CoursesResponse {
  data: CourseSummary[]
}

export interface CourseCertificateTranslation {
  languageCode: string
  courseName: string
  courseProgram: string
  certFrontPage: string
}

export interface CourseDetails {
  id: number
  mainName: string
  name: string
  symbol: string
  expiryTime: string | null
  courseProgram: string
  certFrontPage: string
  certificateTranslations: CourseCertificateTranslation[]
}

export interface CourseResponse {
  data: CourseDetails
}

export interface PaginatedCertificatesResponse {
  data: CertificateSummary[]
  pagination: {
    page: number
    limit: number
    total: number
    totalPages: number
  }
}

export type PaginatedCourseCertificatesResponse = PaginatedCertificatesResponse
export type PaginatedCompanyCertificatesResponse = PaginatedCertificatesResponse

export interface UpdateCoursePayload {
  mainName: string
  name: string
  symbol: string
  expiryTime: string
  courseProgram: string
  certFrontPage: string
  certificateTranslations: CourseCertificateTranslation[]
}

export type CreateCoursePayload = UpdateCoursePayload

export interface RegistryNumberResponse {
  data: {
    courseId: number
    year: number
    nextNumber: number
  }
}

export interface CreateCertificatePayload {
  studentId: number
  courseId: number
  certificateDate: string
  courseDateStart: string
  courseDateEnd: string | null
  registryYear: number
  registryNumber: number
}

export interface CreateCertificateResponse {
  data: {
    id: number
  }
}

export interface JournalSummary {
  id: number
  title: string
  courseSymbol: string
  organizerName: string
  location: string
  formOfTraining: string
  dateStart: string
  dateEnd: string
  totalHours: string
  status: string
  course: {
    id: number
    name: string
  }
  company: {
    id: number
    name: string
  } | null
  attendeesCount: number
  sessionsCount: number
  createdAt: string
}

export interface JournalsResponse {
  data: JournalSummary[]
}

export interface JournalDetails {
  id: number
  courseId: number
  courseName: string
  companyId: number | null
  companyName: string | null
  title: string
  courseSymbol: string
  organizerName: string
  organizerAddress: string | null
  location: string
  formOfTraining: string
  legalBasis: string
  dateStart: string
  dateEnd: string
  totalHours: number
  notes: string | null
  status: string
  createdByUserId: number
  createdAt: string
  updatedAt: string | null
  closedAt: string | null
  attendeesCount: number
  sessionsCount: number
}

export interface JournalResponse {
  data: JournalDetails
}

export type CloseJournalResponse = JournalResponse
export interface DeleteJournalResponse {
  data: {
    id: number
  }
}

export interface JournalAttendeeCertificate {
  id: number
  date: string
  registryYear: number
  registryNumber: number
  courseSymbol: string
}

export interface JournalAttendee {
  id: number
  journalId: number
  studentId: number
  fullNameSnapshot: string
  birthdateSnapshot: string
  companyNameSnapshot: string | null
  certificate: JournalAttendeeCertificate | null
  sortOrder: number
  createdAt: string
}

export interface JournalAttendeesResponse {
  data: JournalAttendee[]
}

export interface JournalAttendeeResponse {
  data: JournalAttendee
}

export interface DeleteJournalAttendeeResponse {
  data: {
    id: number
  }
}

export interface JournalAttendance {
  id: number
  journalSessionId: number
  journalAttendeeId: number
  present: boolean
  createdAt: string
  updatedAt: string
}

export interface JournalAttendanceResponseList {
  data: JournalAttendance[]
}

export interface JournalAttendanceResponse {
  data: JournalAttendance
}

export interface JournalScan {
  id: number
  fileName: string
  contentType: string
  fileSize: number
  uploadedByUserId: number
  createdAt: string
  updatedAt: string
}

export type JournalAttendanceScan = JournalScan
export type JournalSignedScan = JournalScan

export interface JournalScanResponse {
  data: JournalScan
}

export interface JournalSession {
  id: number
  journalId: number
  sessionDate: string
  startTime: string | null
  endTime: string | null
  hours: string
  topic: string
  trainerName: string
  sortOrder: number
  createdAt: string
}

export interface JournalSessionsResponse {
  data: JournalSession[]
}

export interface JournalSessionResponse {
  data: JournalSession
}

export interface GenerateJournalSessionsResponse {
  data: {
    generatedCount: number
  }
}

export interface GenerateJournalAttendeeCertificateResponse {
  data: {
    id: number
  }
}

export interface UpdateJournalSessionPayload {
  sessionDate: string
  trainerName: string
}

export interface UpdateJournalAttendancePayload {
  journalSessionId: number
  journalAttendeeId: number
  present: boolean
}

export interface UpdateJournalAttendeeCertificatePayload {
  certificateId: number | null
}

export interface CreateJournalPayload {
  courseId: number
  companyId: number | null
  title: string
  organizerName: string
  organizerAddress: string | null
  location: string
  formOfTraining: string
  legalBasis: string
  dateStart: string
  dateEnd: string
  notes: string | null
}

export interface UpdateJournalPayload {
  companyId: number | null
  title: string
  organizerName: string
  organizerAddress: string | null
  location: string
  formOfTraining: string
  legalBasis: string
  dateStart: string
  dateEnd: string
  notes: string | null
}

export interface AddJournalAttendeePayload {
  studentId: number
}

export interface CertificateSummary {
  id: number
  date: string
  studentName: string
  companyName: string
  courseName: string
  courseSymbol: string
  registryYear: number
  registryNumber: number
  courseDateStart: string
  courseDateEnd: string | null
  expiryDate: string | null
}

export interface CertificatesResponse {
  data: CertificateSummary[]
}

export interface CertificateDetails {
  id: number
  date: string
  studentId: number
  courseId: number
  studentName: string
  studentSecondname: string
  studentLastname: string
  studentBirthdate: string
  studentBirthplace: string
  studentPesel: string
  companyName: string
  courseDateStart: string
  courseDateEnd: string | null
  registryYear: number
  registryNumber: number
  courseName: string
  courseSymbol: string
  courseExpiryTime: number | null
  courseProgram: string
  certFrontPage: string
  expiryDate: string | null
  journal: {
    id: number
    title: string
    status: string
  } | null
  languageCode: string
  printVariants: Array<{
    languageCode: string
    courseName: string
    courseProgram: string
    certFrontPage: string
    isOriginal: boolean
  }>
}

export interface CertificateResponse {
  data: CertificateDetails
}

export interface UpdateCertificatePayload {
  studentId: number
  certificateDate: string
  courseDateStart: string
  courseDateEnd: string | null
}

export interface DeleteCertificatePayload {
  deleteReason: string | null
}

export interface DeleteCertificateResponse {
  data: {
    id: number
  }
}

const apiErrorMessages: Record<string, string> = {
  // auth
  'invalid_credentials:invalid credentials': 'Nieprawidłowy adres e-mail lub hasło.',
  'too_many_requests:too many requests, please try again later': 'Zbyt wiele prób. Spróbuj ponownie za chwilę.',

  // users
  'forbidden:cannot delete current user': 'Nie można usunąć aktualnie zalogowanego użytkownika.',
  'forbidden:cannot delete last admin': 'Nie można usunąć ostatniego administratora.',
  'forbidden:cannot reset current user password via admin endpoint': 'Nie można zresetować hasła aktualnie zalogowanego użytkownika.',
  'forbidden:cannot remove your own last admin access': 'Nie można odebrać sobie ostatniego dostępu administratora.',
  'forbidden:cannot update last admin role': 'Nie można zmienić roli ostatniego administratora.',
  'bad_request:invalid email format': 'Nieprawidłowy format adresu e-mail.',
  'bad_request:invalid current password': 'Nieprawidłowe aktualne hasło.',

  // companies
  'conflict:company with this NIP already exists': 'Firma o podanym NIP już istnieje.',
  'bad_request:no nip value in request': 'Podaj NIP, aby pobrać dane z GUS.',
  'bad_request:nip validation error: nip must contain exactly 10 digits': 'NIP musi zawierać dokładnie 10 cyfr.',
  'bad_request:nip validation error: invalid nip checksum': 'Podany NIP ma nieprawidłową sumę kontrolną.',
  'not_found:company not found': 'Nie znaleziono firmy dla podanego NIP.',
  'internal_error:gus lookup is not configured': 'Pobieranie danych z GUS nie jest skonfigurowane.',

  // courses
  'conflict:failed to create course: symbol exist': 'Kurs o podanym symbolu już istnieje.',

  // certificates
  'bad_request:certificate translation not found': 'Nie znaleziono tłumaczenia certyfikatu.',
  'bad_request:invalid certificate data': 'Nieprawidłowe dane zaświadczenia.',
  'conflict:registry number already taken for the given year': 'Numer rejestru jest już zajęty dla wybranego kursu i roku.',

  // journals
  'conflict:journal is already closed': 'Dziennik jest już zamknięty.',
  'conflict:journal is closed': 'Dziennik jest zamknięty.',
  'conflict:unable to change header because journal is closed': 'Nie można zmienić nagłówka — dziennik jest zamknięty.',
  'conflict:student already added to journal': 'Kursant jest już dodany do dziennika.',
  'conflict:certificate already linked to journal attendee': 'Zaświadczenie jest już powiązane z uczestnikiem dziennika.',
  'conflict:certificate already linked to another journal attendee': 'Zaświadczenie jest już powiązane z innym uczestnikiem dziennika.',
  'conflict:journal sessions already exist': 'Pozycje programu dziennika już istnieją.',
  'conflict:session outside range': 'Pozycja programu jest poza zakresem dat dziennika.',
  'bad_request:course program is empty': 'Program kursu jest pusty.',
  'bad_request:file is too large': 'Plik jest za duży.',
  'bad_request:file is required': 'Plik jest wymagany.',
  'bad_request:unsupported file type': 'Nieobsługiwany typ pliku.',
  'not_found:journal attendee not found': 'Nie znaleziono uczestnika dziennika.',
  'not_found:journal or student not found': 'Nie znaleziono dziennika lub kursanta.'
}

function parseApiError(error: unknown): { code: string, message: string } | null {
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
    && typeof error.data.error.code === 'string'
    && 'message' in error.data.error
    && typeof error.data.error.message === 'string'
  ) {
    return { code: error.data.error.code, message: error.data.error.message }
  }
  return null
}

export function getApiErrorMessage(error: unknown, fallback = 'Wystąpił błąd.') {
  const parsed = parseApiError(error)
  if (!parsed) return fallback

  const key = `${parsed.code}:${parsed.message}`
  return apiErrorMessages[key] ?? fallback
}

export function useApi() {
  const request = async <T>(path: string, options: Parameters<typeof $fetch<T>>[1] = {}) => {
    const forwardedHeaders = import.meta.server ? useRequestHeaders(['cookie']) : undefined

    try {
      return await $fetch<T>(path, {
        ...options,
        headers: {
          ...forwardedHeaders,
          ...(options.headers ?? {})
        },
        credentials: 'include'
      })
    } catch (error: unknown) {
      if (
        import.meta.client
          && error
          && typeof error === 'object'
          && 'status' in error
          && error.status === 401
          && !path.includes('/auth/')
      ) {
        const auth = useState<AuthUser | null>('auth:user')
        auth.value = null
        await navigateTo('/login')
      }
      throw error
    }
  }

  return {
    request,
    login: async (payload: LoginPayload) => await request<AuthResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: payload
    }),
    logout: async () => await request('/api/v1/auth/logout', {
      method: 'POST'
    }),
    me: async () => await request<AuthResponse>('/api/v1/auth/me'),
    users: async () => await request<AdminUsersResponse>('/api/v1/admin/users'),
    createUser: async (payload: CreateUserPayload) => await request<AuthResponse>('/api/v1/admin/users', {
      method: 'POST',
      body: payload
    }),
    updateUser: async (id: number, payload: UpdateUserPayload) => await request<AuthResponse>(`/api/v1/admin/users/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    resetUserPassword: async (id: number, payload: AdminResetUserPasswordPayload) => await request(`/api/v1/admin/users/${id}/password`, {
      method: 'PATCH',
      body: payload
    }),
    updateProfile: async (payload: UpdateProfilePayload) => await request<AuthResponse>('/api/v1/account/profile', {
      method: 'PATCH',
      body: payload
    }),
    updatePassword: async (payload: UpdatePasswordPayload) => await request('/api/v1/account/password', {
      method: 'PATCH',
      body: payload
    }),
    deleteUser: async (id: number) => await request<DeleteUserResponse>(`/api/v1/admin/users/${id}`, {
      method: 'DELETE'
    }),
    userAuditLog: async (id: number) => await request<AuditLogResponse>(`/api/v1/admin/users/${id}/audit-log`),
    dashboard: async () => await request<DashboardResponse>('/api/v1/dashboard'),
    companies: async (params: { search?: string, limit?: number } = {}) => await request<CompaniesResponse>('/api/v1/companies', {
      query: {
        search: params.search || undefined,
        limit: params.limit || undefined
      }
    }),
    company: async (id: number) => await request<CompanyDetailsResponse>(`/api/v1/companies/${id}`),
    lookupCompanyByNIP: async (nip: string) => await request<GUSCompanyDetailsResponse>('/api/v1/companies/lookup-by-nip', {
      query: { nip }
    }),
    createCompany: async (payload: CreateCompanyPayload) => await request<CompanyDetailsResponse>('/api/v1/companies', {
      method: 'POST',
      body: payload
    }),
    updateCompany: async (id: number, payload: UpdateCompanyPayload) => await request<CompanyDetailsResponse>(`/api/v1/companies/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    companyAuditLog: async (id: number) => await request<AuditLogResponse>(`/api/v1/companies/${id}/audit-log`),
    companyStudents: async (id: number) => await request<CompanyStudentsResponse>(`/api/v1/companies/${id}/students`),
    companyCertificates: async (id: number, params: { page?: number, limit?: number, dateFrom?: string, dateTo?: string } = {}) => await request<PaginatedCompanyCertificatesResponse>(`/api/v1/companies/${id}/certificates`, {
      query: {
        page: params.page || undefined,
        limit: params.limit || undefined,
        dateFrom: params.dateFrom || undefined,
        dateTo: params.dateTo || undefined
      }
    }),
    students: async (params: { search?: string, companyId?: number, limit?: number } = {}) => await request<StudentsResponse>('/api/v1/students', {
      query: {
        search: params.search || undefined,
        companyId: params.companyId || undefined,
        limit: params.limit || undefined
      }
    }),
    student: async (id: number) => await request<StudentDetailsResponse>(`/api/v1/students/${id}`),
    createStudent: async (payload: CreateStudentPayload) => await request<StudentDetailsResponse>('/api/v1/students', {
      method: 'POST',
      body: payload
    }),
    updateStudent: async (id: number, payload: UpdateStudentPayload) => await request<StudentDetailsResponse>(`/api/v1/students/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    studentAuditLog: async (id: number) => await request<AuditLogResponse>(`/api/v1/students/${id}/audit-log`),
    studentCertificates: async (id: number) => await request<StudentCertificatesResponse>(`/api/v1/students/${id}/certificates`),
    courses: async (params: { search?: string, limit?: number } = {}) => await request<CoursesResponse>('/api/v1/courses', {
      query: {
        search: params.search || undefined,
        limit: params.limit || undefined
      }
    }),
    course: async (id: number) => await request<CourseResponse>(`/api/v1/courses/${id}`),
    createCourse: async (payload: CreateCoursePayload) => await request<CourseResponse>('/api/v1/courses', {
      method: 'POST',
      body: payload
    }),
    updateCourse: async (id: number, payload: UpdateCoursePayload) => await request<CourseResponse>(`/api/v1/courses/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    courseAuditLog: async (id: number) => await request<AuditLogResponse>(`/api/v1/courses/${id}/audit-log`),
    courseCertificates: async (id: number, params: { page?: number, limit?: number, dateFrom?: string, dateTo?: string } = {}) => await request<PaginatedCourseCertificatesResponse>(`/api/v1/courses/${id}/certificates`, {
      query: {
        page: params.page || undefined,
        limit: params.limit || undefined,
        dateFrom: params.dateFrom || undefined,
        dateTo: params.dateTo || undefined
      }
    }),
    nextRegistryNumber: async (params: { courseId: number, year: number }) => await request<RegistryNumberResponse>('/api/v1/registries/next-number', {
      query: {
        courseId: params.courseId,
        year: params.year
      }
    }),
    certificates: async (params: { search?: string, dateFrom?: string, dateTo?: string, limit?: number } = {}) => await request<CertificatesResponse>('/api/v1/certificates', {
      query: {
        search: params.search || undefined,
        dateFrom: params.dateFrom || undefined,
        dateTo: params.dateTo || undefined,
        limit: params.limit || undefined
      }
    }),
    journals: async (
      params: {
        search?: string
        status?: string
        courseId?: number
        companyId?: number
        dateFrom?: string
        dateTo?: string
        limit?: number
      } = {}
    ) => await request<JournalsResponse>('/api/v1/journals', {
      query: {
        search: params.search || undefined,
        status: params.status || undefined,
        courseId: params.courseId || undefined,
        companyId: params.companyId || undefined,
        dateFrom: params.dateFrom || undefined,
        dateTo: params.dateTo || undefined,
        limit: params.limit || undefined
      }
    }),
    journal: async (id: number) => await request<JournalResponse>(`/api/v1/journals/${id}`),
    createJournal: async (payload: CreateJournalPayload) => await request<JournalResponse>('/api/v1/journals', {
      method: 'POST',
      body: payload
    }),
    updateJournal: async (id: number, payload: UpdateJournalPayload) => await request<JournalResponse>(`/api/v1/journals/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    closeJournal: async (id: number) => await request<CloseJournalResponse>(`/api/v1/journals/${id}/close`, {
      method: 'POST'
    }),
    deleteJournal: async (id: number) => await request<DeleteJournalResponse>(`/api/v1/journals/${id}`, {
      method: 'DELETE'
    }),
    journalAttendees: async (id: number) => await request<JournalAttendeesResponse>(`/api/v1/journals/${id}/attendees`),
    updateJournalAttendeeCertificate: async (journalId: number, attendeeId: number, payload: UpdateJournalAttendeeCertificatePayload) => await request<JournalAttendeeResponse>(`/api/v1/journals/${journalId}/attendees/${attendeeId}/certificate`, {
      method: 'PATCH',
      body: payload
    }),
    deleteJournalAttendee: async (journalId: number, attendeeId: number) => await request<DeleteJournalAttendeeResponse>(`/api/v1/journals/${journalId}/attendees/${attendeeId}`, {
      method: 'DELETE'
    }),
    journalAttendanceScanMeta: async (id: number) => await request<JournalScanResponse>(`/api/v1/journals/${id}/attendance-scan/meta`),
    uploadJournalAttendanceScan: async (id: number, file: File) => {
      const formData = new FormData()
      formData.append('file', file)

      return await request<JournalScanResponse>(`/api/v1/journals/${id}/attendance-scan`, {
        method: 'POST',
        body: formData
      })
    },
    deleteJournalAttendanceScan: async (id: number) => await request(`/api/v1/journals/${id}/attendance-scan`, {
      method: 'DELETE'
    }),
    journalSignedScanMeta: async (id: number) => await request<JournalScanResponse>(`/api/v1/journals/${id}/signed-scan/meta`),
    uploadJournalSignedScan: async (id: number, file: File) => {
      const formData = new FormData()
      formData.append('file', file)

      return await request<JournalScanResponse>(`/api/v1/journals/${id}/signed-scan`, {
        method: 'POST',
        body: formData
      })
    },
    deleteJournalSignedScan: async (id: number) => await request(`/api/v1/journals/${id}/signed-scan`, {
      method: 'DELETE'
    }),
    journalAttendance: async (id: number) => await request<JournalAttendanceResponseList>(`/api/v1/journals/${id}/attendance`),
    updateJournalAttendance: async (journalId: number, payload: UpdateJournalAttendancePayload) => await request<JournalAttendanceResponse>(`/api/v1/journals/${journalId}/attendance`, {
      method: 'PATCH',
      body: payload
    }),
    journalSessions: async (id: number) => await request<JournalSessionsResponse>(`/api/v1/journals/${id}/sessions`),
    generateJournalSessionsFromCourse: async (id: number) => await request<GenerateJournalSessionsResponse>(`/api/v1/journals/${id}/sessions/generate-from-course`, {
      method: 'POST'
    }),
    updateJournalSession: async (journalId: number, sessionId: number, payload: UpdateJournalSessionPayload) => await request<JournalSessionResponse>(`/api/v1/journals/${journalId}/sessions/${sessionId}`, {
      method: 'PATCH',
      body: payload
    }),
    addJournalAttendee: async (id: number, payload: AddJournalAttendeePayload) => await request<JournalAttendeeResponse>(`/api/v1/journals/${id}/attendees`, {
      method: 'POST',
      body: payload
    }),
    generateJournalAttendeeCertificate: async (journalId: number, attendeeId: number) => await request<GenerateJournalAttendeeCertificateResponse>(`/api/v1/journals/${journalId}/attendees/${attendeeId}/certificate/generate`, {
      method: 'POST'
    }),
    certificate: async (id: number) => await request<CertificateResponse>(`/api/v1/certificates/${id}`),
    certificateAuditLog: async (id: number) => await request<AuditLogResponse>(`/api/v1/certificates/${id}/audit-log`),
    updateCertificate: async (id: number, payload: UpdateCertificatePayload) => await request<CertificateResponse>(`/api/v1/certificates/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    deleteCertificate: async (id: number, payload: DeleteCertificatePayload) => await request<DeleteCertificateResponse>(`/api/v1/certificates/${id}`, {
      method: 'DELETE',
      body: payload
    }),
    createCertificate: async (payload: CreateCertificatePayload) => await request<CreateCertificateResponse>('/api/v1/certificates', {
      method: 'POST',
      body: payload
    })
  }
}
