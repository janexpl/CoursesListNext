package users

type UserDTO struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Firstname string `json:"firstName"`
	Lastname  string `json:"lastName"`
	Role      int32  `json:"role"`
}

type UserResponse struct {
	Data UserDTO `json:"data"`
}

type ListUsersResponse struct {
	Data []UserDTO `json:"data"`
}

type DeleteUserDTO struct {
	ID int64 `json:"id"`
}
type DeleteUserResponse struct {
	Data DeleteUserDTO `json:"data"`
}

type UpdateUserRequest struct {
	Email     string `json:"email"`
	Firstname string `json:"firstName"`
	Lastname  string `json:"lastName"`
	Role      int32  `json:"role"`
}

type UpdateProfileRequest struct {
	Email     string `json:"email"`
	Firstname string `json:"firstName"`
	Lastname  string `json:"lastName"`
}
