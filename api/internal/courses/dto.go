package courses

type CourseDTO struct {
	ID         int64  `json:"id"`
	MainName   string `json:"mainName"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	ExpiryTime *int   `json:"expiryTime"`
	// DeliveredByPlatform - czy kurs jest oferowany przez platformę e-learningową.
	DeliveredByPlatform bool `json:"deliveredByPlatform"`
}

type CourseDetailDTO struct {
	ID                      int64                             `json:"id"`
	MainName                string                            `json:"mainName"`
	Name                    string                            `json:"name"`
	Symbol                  string                            `json:"symbol"`
	ExpiryTime              *int                              `json:"expiryTime"`
	CourseProgram           string                            `json:"courseProgram"`
	CertFrontPage           string                            `json:"certFrontPage"`
	CertificateTranslations []CourseCertificateTranslationDTO `json:"certificateTranslations"`
}

type ListCoursesResponse struct {
	Data       []CourseDTO   `json:"data"`
	Pagination PaginationDTO `json:"pagination"`
}

type ListCoursesDetailsResponse struct {
	Data       []CourseDetailDTO `json:"data"`
	Pagination PaginationDTO     `json:"pagination"`
}

type PaginationDTO struct {
	Page       int32 `json:"page"`
	Limit      int32 `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int32 `json:"totalPages"`
}

// PlatformDeliveryDTO to podzasób kursu /courses/{id}/platform-delivery. Flaga nie
// trafia do CourseDetails, którego kształt jest zamrożony dla integracji.
type PlatformDeliveryDTO struct {
	DeliveredByPlatform bool `json:"deliveredByPlatform"`
}

type PlatformDeliveryResponse struct {
	Data PlatformDeliveryDTO `json:"data"`
}

// PlatformDeliveryRequest ma wskaźnik, żeby odróżnić brak pola od false.
type PlatformDeliveryRequest struct {
	DeliveredByPlatform *bool `json:"deliveredByPlatform"`
}

type GetCourseResponse struct {
	Data CourseDetailDTO `json:"data"`
}

type coursePayload struct {
	MainName                string                            `json:"mainName"`
	Name                    string                            `json:"name"`
	Symbol                  string                            `json:"symbol"`
	ExpiryTime              *int                              `json:"expiryTime"`
	CourseProgram           string                            `json:"courseProgram"`
	CertFrontPage           string                            `json:"certFrontPage"`
	CertificateTranslations []CourseCertificateTranslationDTO `json:"certificateTranslations"`
}

type CourseCertificateTranslationDTO struct {
	LanguageCode  string `json:"languageCode"`
	CourseName    string `json:"courseName"`
	CourseProgram string `json:"courseProgram"`
	CertFrontPage string `json:"certFrontPage"`
}

type UpdateCourseRequest struct {
	coursePayload
}

type CreateCourseRequest struct {
	coursePayload
}
