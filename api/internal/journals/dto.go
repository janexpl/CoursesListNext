package journals

type JournalListItemDTO struct {
	ID             int64          `json:"id"`
	Title          string         `json:"title"`
	CourseSymbol   string         `json:"courseSymbol"`
	OrganizerName  string         `json:"organizerName"`
	Location       string         `json:"location"`
	FormOfTraining string         `json:"formOfTraining"`
	DateStart      string         `json:"dateStart"`
	DateEnd        string         `json:"dateEnd"`
	TotalHours     string         `json:"totalHours"`
	Status         string         `json:"status"`
	Course         CourseRefDTO   `json:"course"`
	Company        *CompanyRefDTO `json:"company"`
	AttendeesCount int64          `json:"attendeesCount"`
	SessionsCount  int64          `json:"sessionsCount"`
	CreatedAt      string         `json:"createdAt"`
}

type CourseRefDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CompanyRefDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ListJournalsResponse struct {
	Data []JournalListItemDTO `json:"data"`
}

type JournalDetailsDTO struct {
	ID               int64   `json:"id"`
	CourseID         int64   `json:"courseId"`
	CourseName       string  `json:"courseName"`
	CompanyID        *int64  `json:"companyId"`
	CompanyName      *string `json:"companyName"`
	Title            string  `json:"title"`
	CourseSymbol     string  `json:"courseSymbol"`
	OrganizerName    string  `json:"organizerName"`
	OrganizerAddress *string `json:"organizerAddress"`
	Location         string  `json:"location"`
	FormOfTraining   string  `json:"formOfTraining"`
	LegalBasis       string  `json:"legalBasis"`
	DateStart        string  `json:"dateStart"`
	DateEnd          string  `json:"dateEnd"`
	TotalHours       float64 `json:"totalHours"`
	Notes            *string `json:"notes"`
	Status           string  `json:"status"`
	CreatedByUserID  int64   `json:"createdByUserId"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        *string `json:"updatedAt"`
	ClosedAt         *string `json:"closedAt"`
	AttendeesCount   int64   `json:"attendeesCount"`
	SessionsCount    int64   `json:"sessionsCount"`
}

type JournalDetailResponse struct {
	Data JournalDetailsDTO `json:"data"`
}

type DeleteJournalResponse struct {
	Data struct {
		ID int64 `json:"id"`
	} `json:"data"`
}

type CreateJournalRequest struct {
	CourseID         int64   `json:"courseId"`
	CompanyID        *int64  `json:"companyId"`
	Title            string  `json:"title"`
	OrganizerName    string  `json:"organizerName"`
	OrganizerAddress *string `json:"organizerAddress"`
	Location         string  `json:"location"`
	FormOfTraining   string  `json:"formOfTraining"`
	LegalBasis       string  `json:"legalBasis"`
	DateStart        string  `json:"dateStart"`
	DateEnd          string  `json:"dateEnd"`
	Notes            *string `json:"notes"`
}

type AddJournalAttendeeRequest struct {
	StudentID int64 `json:"studentId"`
}

type UpdateJournalAttendeeCertificateRequest struct {
	CertificateID *int64 `json:"certificateId"`
}

type JournalAttendeeCertificateDTO struct {
	ID             int64  `json:"id"`
	Date           string `json:"date"`
	RegistryYear   int64  `json:"registryYear"`
	RegistryNumber int64  `json:"registryNumber"`
	CourseSymbol   string `json:"courseSymbol"`
}

type JournalAttendeeDTO struct {
	ID                  int64                          `json:"id"`
	JournalID           int64                          `json:"journalId"`
	StudentID           int64                          `json:"studentId"`
	FullNameSnapshot    string                         `json:"fullNameSnapshot"`
	BirthdateSnapshot   string                         `json:"birthdateSnapshot"`
	CompanyNameSnapshot *string                        `json:"companyNameSnapshot"`
	Certificate         *JournalAttendeeCertificateDTO `json:"certificate"`
	SortOrder           int32                          `json:"sortOrder"`
	CreatedAt           string                         `json:"createdAt"`
}

type JournalAttendeeResponse struct {
	Data JournalAttendeeDTO `json:"data"`
}

type AddJournalAttendeeResponse = JournalAttendeeResponse

type ListJournalAttendeeResponse struct {
	Data []JournalAttendeeDTO `json:"data"`
}

type DeleteJournalAttendeeResponse struct {
	Data struct {
		ID int64 `json:"id"`
	} `json:"data"`
}

type GenerateJournalAttendeeCertificateResponse struct {
	Data struct {
		ID int64 `json:"id"`
	} `json:"data"`
}

type JournalAttendanceDTO struct {
	ID                int64  `json:"id"`
	JournalSessionID  int64  `json:"journalSessionId"`
	JournalAttendeeID int64  `json:"journalAttendeeId"`
	Present           bool   `json:"present"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type ListJournalAttendanceResponse struct {
	Data []JournalAttendanceDTO `json:"data"`
}

type JournalAttendanceResponse struct {
	Data JournalAttendanceDTO `json:"data"`
}

type UpdateJournalAttendanceRequest struct {
	JournalSessionID  int64 `json:"journalSessionId"`
	JournalAttendeeID int64 `json:"journalAttendeeId"`
	Present           bool  `json:"present"`
}

type JournalSessionDTO struct {
	ID          int64   `json:"id"`
	JournalID   int64   `json:"journalId"`
	SessionDate string  `json:"sessionDate"`
	StartTime   *string `json:"startTime"`
	EndTime     *string `json:"endTime"`
	Hours       string  `json:"hours"`
	Topic       string  `json:"topic"`
	TrainerName string  `json:"trainerName"`
	SortOrder   int32   `json:"sortOrder"`
	CreatedAt   string  `json:"createdAt"`
}

type ListJournalSessionsResponse struct {
	Data []JournalSessionDTO `json:"data"`
}

type JournalSessionResponse struct {
	Data JournalSessionDTO `json:"data"`
}

type GenerateJournalSessionsResponse struct {
	Data struct {
		GeneratedCount int64 `json:"generatedCount"`
	} `json:"data"`
}

type UpdateJournalSessionRequest struct {
	SessionDate string `json:"sessionDate"`
	TrainerName string `json:"trainerName"`
}

type UpdateJournalHeaderRequest struct {
	CompanyID        *int64  `json:"companyId"`
	Title            string  `json:"title"`
	OrganizerName    string  `json:"organizerName"`
	OrganizerAddress *string `json:"organizerAddress"`
	Location         string  `json:"location"`
	FormOfTraining   string  `json:"formOfTraining"`
	LegalBasis       string  `json:"legalBasis"`
	DateStart        string  `json:"dateStart"`
	DateEnd          string  `json:"dateEnd"`
	Notes            *string `json:"notes"`
}

type JournalAttendanceScanDTO struct {
	ID               int64  `json:"id"`
	FileName         string `json:"fileName"`
	ContentType      string `json:"contentType"`
	FileSize         int64  `json:"fileSize"`
	UploadedByUserID int64  `json:"uploadedByUserId"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type JournalAttendanceScanResponse struct {
	Data JournalAttendanceScanDTO `json:"data"`
}
