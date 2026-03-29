package gusclient

type GUSCompanyDTO struct {
	NIP         string `json:"nip"`
	REGON       string `json:"regon"`
	Name        string `json:"name"`
	Voivodeship string `json:"voivodeship"`
	County      string `json:"county"`
	Commune     string `json:"commune"`
	City        string `json:"city"`
	PostalCode  string `json:"postalCode"`
	Street      string `json:"street"`
	HouseNumber string `json:"houseNumber"`
	Apartment   string `json:"apartment"`
	Status      string `json:"status"`
}

type GUSCompanyResponse struct {
	Data GUSCompanyDTO `json:"data"`
}
