package certificates

type CreateCertificateRequest struct {
	StudentID       int64   `json:"studentId"`
	CourseID        int64   `json:"courseId"`
	CertificateDate string  `json:"certificateDate"`
	CourseDateStart string  `json:"courseDateStart"`
	CourseDateEnd   *string `json:"courseDateEnd"`
	RegistryYear    int64   `json:"registryYear"`
	RegistryNumber  int32   `json:"registryNumber"`
}

type CreateCertificateResponse struct {
	Data CreateCertificateResponseData `json:"data"`
}

type CreateCertificateResponseData struct {
	ID int64 `json:"id"`
}
