package courses

import "encoding/json"

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
	// LegalBasis - podstawa prawna wstawiana w szablon znacznikiem {{ podstawa_prawna }};
	// null, gdy kurs jej nie wskazuje.
	LegalBasis *CourseLegalBasisDTO `json:"legalBasis"`
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
	LegalBasisID            OptionalID                        `json:"legalBasisId"`
}

// CourseLegalBasisDTO - podstawa prawna wskazana przez kurs.
type CourseLegalBasisDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// OptionalID odróżnia pole pominięte od jawnego null.
//
// PATCH /courses/{id} działa jak pełne nadpisanie, ale legalBasisId - tak jak
// certificateTranslations - pominięte zostawia podstawę bez zmian, a null ją zdejmuje.
// Inaczej każdy klient API, który nie zna nowego pola, zdejmowałby kursom podstawę
// prawną przy każdej edycji.
type OptionalID struct {
	Set   bool
	Value *int64
}

// UnmarshalJSON jest wołane tylko dla obecnego klucza, więc samo wywołanie oznacza Set.
func (o *OptionalID) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
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
