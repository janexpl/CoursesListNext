package certificates

type CertificateDTO struct {
	ID              int64   `json:"id"`
	Date            string  `json:"date"`
	StudentName     string  `json:"studentName"`
	CompanyName     string  `json:"companyName"`
	CourseName      string  `json:"courseName"`
	CourseSymbol    string  `json:"courseSymbol"`
	RegistryYear    int     `json:"registryYear"`
	RegistryNumber  int     `json:"registryNumber"`
	CourseDateStart string  `json:"courseDateStart"`
	CourseDateEnd   *string `json:"courseDateEnd"`
	ExpiryDate      *string `json:"expiryDate"`
	LanguageCode    string  `json:"languageCode"`
}

type CertificateDetailsDTO struct {
	ID                int64                        `json:"id"`
	Date              string                       `json:"date"`
	StudentID         int64                        `json:"studentId"`
	CourseID          int64                        `json:"courseId"`
	StudentName       string                       `json:"studentName"`
	StudentSecondname string                       `json:"studentSecondname"`
	StudentLastname   string                       `json:"studentLastname"`
	StudentBirthdate  string                       `json:"studentBirthdate"`
	StudentBirthplace string                       `json:"studentBirthplace"`
	StudentPesel      string                       `json:"studentPesel"`
	CompanyName       string                       `json:"companyName"`
	CourseDateStart   string                       `json:"courseDateStart"`
	CourseDateEnd     *string                      `json:"courseDateEnd"`
	RegistryYear      int                          `json:"registryYear"`
	RegistryNumber    int                          `json:"registryNumber"`
	CourseName        string                       `json:"courseName"`
	CourseSymbol      string                       `json:"courseSymbol"`
	CourseExpiryTime  *int                         `json:"courseExpiryTime"`
	CourseProgram     string                       `json:"courseProgram"`
	CertFrontPage     string                       `json:"certFrontPage"`
	ExpiryDate        *string                      `json:"expiryDate"`
	Journal           *CertificateJournalRefDTO    `json:"journal"`
	LanguageCode      string                       `json:"languageCode"`
	PrintVariants     []CertificatePrintVariantDTO `json:"printVariants"`
}

type CertificatePrintVariantDTO struct {
	LanguageCode  string `json:"languageCode"`
	CourseName    string `json:"courseName"`
	CourseProgram string `json:"courseProgram"`
	CertFrontPage string `json:"certFrontPage"`
	IsOriginal    bool   `json:"isOriginal"`
}

type ListCertificatesResponse struct {
	Data []CertificateDTO `json:"data"`
}

type CertificateResponse struct {
	Data CertificateDetailsDTO `json:"data"`
}

type UpdateCertificateRequest struct {
	StudentID       int64   `json:"studentId"`
	CertificateDate string  `json:"certificateDate"`
	CourseDateStart string  `json:"courseDateStart"`
	CourseDateEnd   *string `json:"courseDateEnd,omitempty"`
}

type SoftDeleteCertificateRequest struct {
	DeleteReason *string `json:"deleteReason"`
}

type DeleteCertificateDTO struct {
	ID int64 `json:"id"`
}

type DeleteCertificateResponse struct {
	Data DeleteCertificateDTO `json:"data"`
}

type CertificateJournalRefDTO struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type PaginationDTO struct {
	Page       int32 `json:"page"`
	Limit      int32 `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int32 `json:"totalPages"`
}

type ListCertificatesByCourseResponse struct {
	Data       []CertificateDTO `json:"data"`
	Pagination PaginationDTO    `json:"pagination"`
}

type ListCertificatesByCompanyResponse struct {
	Data       []CertificateDTO `json:"data"`
	Pagination PaginationDTO    `json:"pagination"`
}

type ExpiringCertificateNotificationCandidateDTO struct {
	CertificateID   int64                                     `json:"certificateId"`
	CertificateDate string                                    `json:"certificateDate"`
	ExpiryDate      string                                    `json:"expiryDate"`
	RegistryYear    int64                                     `json:"registryYear"`
	RegistryNumber  int64                                     `json:"registryNumber"`
	LanguageCode    string                                    `json:"languageCode"`
	Student         ExpiringCertificateNotificationStudentDTO `json:"student"`
	Company         ExpiringCertificateNotificationCompanyDTO `json:"company"`
	Course          ExpiringCertificateNotificationCourseDTO  `json:"course"`
}

type ExpiringCertificateNotificationStudentDTO struct {
	ID        int32   `json:"id"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	PESEL     *string `json:"pesel"`
}

type ExpiringCertificateNotificationCompanyDTO struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CurrentName    string `json:"currentName"`
	RecipientEmail string `json:"recipientEmail"`
}

type ExpiringCertificateNotificationCourseDTO struct {
	Name      string  `json:"name"`
	Symbol    string  `json:"symbol"`
	DateStart string  `json:"dateStart"`
	DateEnd   *string `json:"dateEnd"`
}

type ListExpiringCertificateNotificationCandidatesResponse struct {
	Data []ExpiringCertificateNotificationCandidateDTO    `json:"data"`
	Meta ExpiringCertificateNotificationCandidatesMetaDTO `json:"meta"`
}

type ExpiringCertificateNotificationCandidatesMetaDTO struct {
	Limit      int32                                               `json:"limit"`
	HasMore    bool                                                `json:"hasMore"`
	NextCursor *ExpiringCertificateNotificationCandidatesCursorDTO `json:"nextCursor"`
}

type ExpiringCertificateNotificationCandidatesCursorDTO struct {
	AfterExpiryDate    string `json:"afterExpiryDate"`
	AfterCertificateID int64  `json:"afterCertificateId"`
}
