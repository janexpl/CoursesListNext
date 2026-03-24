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

export interface CourseDetails {
  id: number
  mainName: string
  name: string
  symbol: string
  expiryTime: string | null
  courseProgram: string
  certFrontPage: string
}

export interface CourseResponse {
  data: CourseDetails
}

export interface UpdateCoursePayload {
  mainName: string
  name: string
  symbol: string
  expiryTime: string
  courseProgram: string
  certFrontPage: string
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

export interface JournalAttendanceScan {
  id: number
  fileName: string
  contentType: string
  fileSize: number
  uploadedByUserId: number
  createdAt: string
  updatedAt: string
}

export interface JournalAttendanceScanResponse {
  data: JournalAttendanceScan
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

export function getApiErrorMessage(error: unknown, fallback = 'Wystąpił błąd.') {
  if (
    error
    && typeof error === 'object'
    && 'data' in error
    && error.data
    && typeof error.data === 'object'
    && 'error' in error.data
    && error.data.error
    && typeof error.data.error === 'object'
    && 'message' in error.data.error
    && typeof error.data.error.message === 'string'
  ) {
    return error.data.error.message
  }

  return fallback
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
    dashboard: async () => await request<DashboardResponse>('/api/v1/dashboard'),
    companies: async (params: { search?: string, limit?: number } = {}) => await request<CompaniesResponse>('/api/v1/companies', {
      query: {
        search: params.search || undefined,
        limit: params.limit || undefined
      }
    }),
    company: async (id: number) => await request<CompanyDetailsResponse>(`/api/v1/companies/${id}`),
    createCompany: async (payload: CreateCompanyPayload) => await request<CompanyDetailsResponse>('/api/v1/companies', {
      method: 'POST',
      body: payload
    }),
    updateCompany: async (id: number, payload: UpdateCompanyPayload) => await request<CompanyDetailsResponse>(`/api/v1/companies/${id}`, {
      method: 'PATCH',
      body: payload
    }),
    companyStudents: async (id: number) => await request<CompanyStudentsResponse>(`/api/v1/companies/${id}/students`),
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
    nextRegistryNumber: async (params: { courseId: number, year: number }) => await request<RegistryNumberResponse>('/api/v1/registries/next-number', {
      query: {
        courseId: params.courseId,
        year: params.year
      }
    }),
    certificates: async (params: { search?: string, limit?: number } = {}) => await request<CertificatesResponse>('/api/v1/certificates', {
      query: {
        search: params.search || undefined,
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
    journalAttendanceScanMeta: async (id: number) => await request<JournalAttendanceScanResponse>(`/api/v1/journals/${id}/attendance-scan/meta`),
    uploadJournalAttendanceScan: async (id: number, file: File) => {
      const formData = new FormData()
      formData.append('file', file)

      return await request<JournalAttendanceScanResponse>(`/api/v1/journals/${id}/attendance-scan`, {
        method: 'POST',
        body: formData
      })
    },
    deleteJournalAttendanceScan: async (id: number) => await request(`/api/v1/journals/${id}/attendance-scan`, {
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
