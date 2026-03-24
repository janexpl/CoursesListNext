package dashboard

type DashboardResponse struct {
	Data DashboardDataDTO `json:"data"`
}

type DashboardStatsDTO struct {
	Students     int64 `json:"students"`
	Companies    int64 `json:"companies"`
	Certificates int64 `json:"certificates"`
}

type DashboardDataDTO struct {
	Stats                DashboardStatsDTO        `json:"stats"`
	Expiring             ExpiringSummaryDTO       `json:"expiring"`
	ExpiringCertificates []ExpiringCertificateDTO `json:"expiringCertificates"`
}

type ExpiringSummaryDTO struct {
	In30Days int `json:"in30Days"`
}

type ExpiringCertificateDTO struct {
	CertificateID  int64   `json:"certificateId"`
	ExpiryDate     string  `json:"expiryDate"`
	StudentName    string  `json:"studentName"`
	CompanyName    string  `json:"companyName"`
	CourseName     string  `json:"courseName"`
	CourseSymbol   string  `json:"courseSymbol"`
	RegistryYear   int64   `json:"registryYear"`
	RegistryNumber float64 `json:"registryNumber"`
}
