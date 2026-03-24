package auth

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserDTO struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      int32  `json:"role"`
}

type LoginResponse struct {
	Data UserDTO `json:"data"`
}

type MeResponse struct {
	Data UserDTO `json:"data"`
}
