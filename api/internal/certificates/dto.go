package certificates

type CertificateDTO struct {
	ID                int64   `json:"id"`
	Date              string  `json:"date"`
	StudentName       string  `json:"studentName"`
	CompanyName       string  `json:"companyName"`
	CourseName        string  `json:"courseName"`
	CourseSymbol      string  `json:"courseSymbol"`
	RegistryYear      int     `json:"registryYear"`
	RegistryNumber    int     `json:"registryNumber"`
	CourseDateStart   string  `json:"courseDateStart"`
	CourseDateEnd     *string `json:"courseDateEnd"`
	ExpiryDate        *string `json:"expiryDate"`
	LanguageCode      string  `json:"languageCode"`
	RevokedAt         *string `json:"revokedAt"`
	DuplicateIssuedAt *string `json:"duplicateIssuedAt"`
}

type CertificateDetailsDTO struct {
	ID                int64                        `json:"id"`
	Date              string                       `json:"date"`
	StudentID         int64                        `json:"studentId"`
	CourseID          int64                        `json:"courseId"`
	StudentFirstname  string                       `json:"studentFirstname"`
	StudentSecondname *string                      `json:"studentSecondname"`
	StudentLastname   string                       `json:"studentLastname"`
	StudentBirthdate  string                       `json:"studentBirthdate"`
	StudentBirthplace string                       `json:"studentBirthplace"`
	StudentPesel      *string                      `json:"studentPesel"`
	CompanyName       *string                      `json:"companyName"`
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
	VerificationCode  string                       `json:"verificationCode"`
	// VerificationURL i VerificationQr są puste, gdy instancja nie ma skonfigurowanego
	// adresu publicznej weryfikacji - wtedy kod QR nie jest też drukowany.
	VerificationURL string `json:"verificationUrl,omitempty"`
	VerificationQr  string `json:"verificationQr,omitempty"`
	// PrintDecor jest wypełniane WYŁĄCZNIE dla zaświadczeń platformowych - przeglądarka
	// nie powtarza kryterium, tylko dostaje gotową odpowiedź. Obrazy idą adresami,
	// a nie jak kod QR w treści: cztery obrazki w każdej odpowiedzi to pół megabajta
	// przy każdym GET /certificates/{id}.
	PrintDecor        *CertificatePrintDecorDTO `json:"printDecor,omitempty"`
	RevokedAt         *string                   `json:"revokedAt"`
	RevokeReason      *string                   `json:"revokeReason"`
	DuplicateIssuedAt *string                   `json:"duplicateIssuedAt"`
	DuplicateReason   *string                   `json:"duplicateReason"`
}

// CertificatePrintDecorDTO opisuje nadruki wydruku: gdzie po nie sięgnąć i jak szerokie
// mają być na papierze. Pola pieczątek i podpisu są puste, dopóki administrator
// nie wgra pliku; tło jest zawsze, bo jest wkompilowane w API.
type CertificatePrintDecorDTO struct {
	StampRound        *CertificatePrintImageDTO `json:"stampRound"`
	StampCompany      *CertificatePrintImageDTO `json:"stampCompany"`
	StampPersonal     *CertificatePrintImageDTO `json:"stampPersonal"`
	Signature         *CertificatePrintImageDTO `json:"signature"`
	GuillocheFrontURL string                    `json:"guillocheFrontUrl"`
	GuillocheBackURL  string                    `json:"guillocheBackUrl"`
}

type CertificatePrintImageDTO struct {
	URL     string `json:"url"`
	WidthMm int    `json:"widthMm"`
}

// PublicCertificateDTO to odpowiedź publicznej weryfikacji - trafia do każdego,
// kto zeskanuje kod QR z dokumentu. Niesie wyłącznie to, co jest na papierze:
// bez PESEL-u, daty i miejsca urodzenia, nazwy firmy, powodów unieważnienia
// i identyfikatorów, którymi dałoby się sięgnąć do chronionego API.
type PublicCertificateDTO struct {
	VerificationCode  string  `json:"verificationCode"`
	CertificateNumber string  `json:"certificateNumber"`
	StudentName       string  `json:"studentName"`
	CourseName        string  `json:"courseName"`
	CourseDateStart   string  `json:"courseDateStart"`
	CourseDateEnd     *string `json:"courseDateEnd"`
	IssuedAt          string  `json:"issuedAt"`
	ValidUntil        *string `json:"validUntil"`
	// Status: "valid" albo "revoked". Wygaśnięcie terminu ważności jest osobnym
	// polem, bo dokument po terminie nadal jest autentyczny.
	Status            string  `json:"status"`
	Expired           bool    `json:"expired"`
	DuplicateIssued   bool    `json:"duplicateIssued"`
	DuplicateIssuedAt *string `json:"duplicateIssuedAt"`
	RevokedAt         *string `json:"revokedAt"`
}

type PublicCertificateResponse struct {
	Data PublicCertificateDTO `json:"data"`
}

// LifecycleRequest to ciało POST /certificates/{id}/revoke i /duplicate.
type LifecycleRequest struct {
	Reason *string `json:"reason"`
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
