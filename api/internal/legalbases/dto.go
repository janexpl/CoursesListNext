package legalbases

// LegalBasisDTO - wpis biblioteki podstaw prawnych.
type LegalBasisDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	// CourseCount - ilu kursów dotknie zmiana treści.
	CourseCount int64  `json:"courseCount"`
	UpdatedAt   string `json:"updatedAt"`
}

// LegalBasisCourseDTO - kurs wskazujący podstawę; zakładka administratora pokazuje,
// których kursów dotyczy edycja.
type LegalBasisCourseDTO struct {
	ID     int64  `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

type LegalBasisDetailsDTO struct {
	LegalBasisDTO
	Courses []LegalBasisCourseDTO `json:"courses"`
}

type ListLegalBasesResponse struct {
	Data []LegalBasisDTO `json:"data"`
}

type LegalBasisResponse struct {
	Data LegalBasisDetailsDTO `json:"data"`
}

// LegalBasisRequest to ciało POST /legal-bases i PATCH /admin/legal-bases/{id}.
type LegalBasisRequest struct {
	Name    *string `json:"name"`
	Content *string `json:"content"`
}
