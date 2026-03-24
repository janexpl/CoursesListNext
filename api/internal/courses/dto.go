package courses

type CourseDTO struct {
	ID         int64   `json:"id"`
	MainName   string  `json:"mainName"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	ExpiryTime *string `json:"expiryTime"`
}

type CourseDetailDTO struct {
	ID            int64   `json:"id"`
	MainName      string  `json:"mainName"`
	Name          string  `json:"name"`
	Symbol        string  `json:"symbol"`
	ExpiryTime    *string `json:"expiryTime"`
	CourseProgram string  `json:"courseProgram"`
	CertFrontPage string  `json:"certFrontPage"`
}

type ListCoursesResponse struct {
	Data []CourseDTO `json:"data"`
}

type GetCourseResponse struct {
	Data CourseDetailDTO `json:"data"`
}

type coursePayload struct {
	MainName      string  `json:"mainName"`
	Name          string  `json:"name"`
	Symbol        string  `json:"symbol"`
	ExpiryTime    *string `json:"expiryTime"`
	CourseProgram string  `json:"courseProgram"`
	CertFrontPage string  `json:"certFrontPage"`
}

type UpdateCourseRequest struct {
	coursePayload
}

type CreateCourseRequest struct {
	coursePayload
}
