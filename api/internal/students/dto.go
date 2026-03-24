package students

type CompanyDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StudentDTO struct {
	ID        int64       `json:"id"`
	FirstName string      `json:"firstName"`
	LastName  string      `json:"lastName"`
	Pesel     *string     `json:"pesel"`
	BirthDate string      `json:"birthDate"`
	Company   *CompanyDTO `json:"company"`
}

type ListStudentsResponse struct {
	Data []StudentDTO `json:"data"`
}

type StudentDetailsDTO struct {
	ID            int64       `json:"id"`
	FirstName     string      `json:"firstName"`
	LastName      string      `json:"lastName"`
	SecondName    *string     `json:"secondName"`
	BirthDate     string      `json:"birthDate"`
	BirthPlace    string      `json:"birthPlace"`
	Pesel         *string     `json:"pesel"`
	AddressStreet *string     `json:"addressStreet"`
	AddressCity   *string     `json:"addressCity"`
	AddressZip    *string     `json:"addressZip"`
	Telephone     *string     `json:"telephone"`
	Company       *CompanyDTO `json:"company"`
}

type StudentDetailsResponse struct {
	Data StudentDetailsDTO `json:"data"`
}

type CertificateByStudentDTO struct {
	ID              int64   `json:"id"`
	Date            string  `json:"date"`
	CourseName      string  `json:"courseName"`
	CourseSymbol    string  `json:"courseSymbol"`
	RegistryYear    int64   `json:"registryYear"`
	RegistryNumber  int64   `json:"registryNumber"`
	CourseDateStart string  `json:"courseDateStart"`
	CourseDateEnd   *string `json:"courseDateEnd"`
	ExpiryDate      *string `json:"expiryDate"`
}

type ListCertificatesByStudentResponse struct {
	Data []CertificateByStudentDTO `json:"data"`
}

type ListStudentsByCompanyIdDTO struct {
	ID         int64   `json:"id"`
	Firstname  string  `json:"firstname"`
	Lastname   string  `json:"lastname"`
	Secondname *string `json:"secondname"`
	Birthdate  string  `json:"birthdate"`
	Birthplace string  `json:"birthplace"`
	Pesel      *string `json:"pesel"`
}

type ListStudentsByCompanyIdResult struct {
	Data []ListStudentsByCompanyIdDTO `json:"data"`
}
type studentPayload struct {
	FirstName     string  `json:"firstName"`
	LastName      string  `json:"lastName"`
	SecondName    *string `json:"secondName"`
	BirthDate     string  `json:"birthDate"`
	BirthPlace    string  `json:"birthPlace"`
	Pesel         *string `json:"pesel"`
	AddressStreet *string `json:"addressStreet"`
	AddressCity   *string `json:"addressCity"`
	AddressZip    *string `json:"addressZip"`
	Telephone     *string `json:"telephone"`
	CompanyID     *int64  `json:"companyId"`
}

type UpdateStudentRequest struct {
	studentPayload
}

type CreateStudentRequest struct {
	studentPayload
}
