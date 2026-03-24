package companies

type CompanyDTO struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	City          string `json:"city"`
	NIP           string `json:"nip"`
	ContactPerson string `json:"contactPerson,omitempty"`
	Telephone     string `json:"telephone,omitempty"`
}

type CompanyDetailsDTO struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Street        string  `json:"street"`
	City          string  `json:"city"`
	Zipcode       string  `json:"zipcode"`
	Nip           string  `json:"nip"`
	Email         *string `json:"email"`
	Contactperson *string `json:"contactPerson"`
	Telephoneno   string  `json:"telephone"`
	Note          *string `json:"note"`
}

type CompanyDetailsResponse struct {
	Data CompanyDetailsDTO `json:"data"`
}

type ListCompaniesResponse struct {
	Data []CompanyDTO `json:"data"`
}

type UpdateCompanyDTO struct {
	Name          string  `json:"name"`
	Street        string  `json:"street"`
	City          string  `json:"city"`
	Zipcode       string  `json:"zipcode"`
	Nip           string  `json:"nip"`
	Email         *string `json:"email"`
	ContactPerson *string `json:"contactPerson"`
	Telephone     string  `json:"telephone"`
	Note          *string `json:"note"`
}

type CreateCompanyRequest struct {
	Name          string  `json:"name"`
	Street        string  `json:"street"`
	City          string  `json:"city"`
	Zipcode       string  `json:"zipcode"`
	Nip           string  `json:"nip"`
	Email         *string `json:"email"`
	ContactPerson *string `json:"contactPerson"`
	Telephone     string  `json:"telephone"`
	Note          *string `json:"note"`
}
