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
