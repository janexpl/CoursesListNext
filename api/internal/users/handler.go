package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

type Querier interface {
	ListUsers(ctx context.Context) ([]sqlc.ListUsersRow, error)
	DeleteUser(ctx context.Context, id int64) (int64, error)
	GetUserByID(ctx context.Context, id int64) (sqlc.User, error)
	CountAdminUsers(ctx context.Context, role int32) (int64, error)
	UpdateUser(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error)
}
type Creator interface {
	Create(ctx context.Context, req CreateUserRequest) (UserDTO, error)
	PatchPassword(ctx context.Context, req UpdatePasswordRequest) error
	PatchPasswordByAdmin(ctx context.Context, userID int64, newPassword string) error
}

type Handler struct {
	querier Querier
	creator Creator
}

func NewHandler(querier Querier, creator Creator) *Handler {

	return &Handler{
		querier: querier,
		creator: creator,
	}
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := CreateUserRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	req.Firstname = strings.TrimSpace(req.Firstname)
	req.Lastname = strings.TrimSpace(req.Lastname)
	if req.Email == "" || req.Password == "" || req.Firstname == "" || req.Lastname == "" || req.Role <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if !validation.CheckEmail(req.Email) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid email format")
		return
	}
	row, err := h.creator.Create(r.Context(), CreateUserRequest{
		Email:     req.Email,
		Password:  req.Password,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create user")
		return
	}
	response.WriteJSON(w, http.StatusCreated, UserResponse{
		Data: row,
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.querier.ListUsers(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to retrive users")
		return
	}
	resp := ListUsersResponse{Data: make([]UserDTO, 0, len(rows))}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapUserRow(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid user id")
		return
	}

	currentUser, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	if currentUser.ID == id {
		response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "cannot delete current user")
		return
	}

	targetUser, err := h.querier.GetUserByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "user")
		return
	}

	if targetUser.Role == auth.RoleAdmin {
		adminCount, err := h.querier.CountAdminUsers(r.Context(), auth.RoleAdmin)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to count admin users")
			return
		}
		if adminCount <= 1 {
			response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "cannot delete last admin")
			return
		}
	}

	row, err := h.querier.DeleteUser(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to delete user")
		return
	}
	if row == 0 {
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "user not found")
		return
	}
	response.WriteJSON(w, http.StatusOK, DeleteUserResponse{
		Data: DeleteUserDTO{
			ID: id,
		},
	})
}

func (h *Handler) PatchPasswordByAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid user id")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := AdminUpdatePasswordRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	newPassword := strings.TrimSpace(req.NewPassword)
	if newPassword == "" || len(newPassword) < 8 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	me, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	if id == me.ID {
		response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "cannot reset current user password via admin endpoint")
		return
	}
	if err = h.creator.PatchPasswordByAdmin(r.Context(), id, newPassword); err != nil {
		response.HandleDBError(w, err, "user")
		return
	}
	response.WriteNoContent(w)

}
func (h *Handler) PatchPassword(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdatePasswordRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	err = h.creator.PatchPassword(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
			return
		}
		if errors.Is(err, ErrInvalidCurrentPassword) {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid current password")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "password update failed")
		return
	}
	response.WriteNoContent(w)
}
func (h *Handler) PatchProfile(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateProfileRequest{}
	err := decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Firstname = strings.TrimSpace(req.Firstname)
	req.Lastname = strings.TrimSpace(req.Lastname)
	if req.Email == "" || req.Firstname == "" || req.Lastname == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if !validation.CheckEmail(req.Email) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid email format")
		return
	}
	currentUser, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	row, err := h.querier.UpdateUser(r.Context(), sqlc.UpdateUserParams{
		ID:        currentUser.ID,
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      currentUser.Role,
	})
	if err != nil {
		response.HandleDBError(w, err, "user")
		return
	}

	response.WriteJSON(w, http.StatusOK, UserResponse{
		Data: mapUpdateUserRow(row),
	})

}
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid user id")
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := UpdateUserRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Firstname = strings.TrimSpace(req.Firstname)
	req.Lastname = strings.TrimSpace(req.Lastname)
	if req.Email == "" || req.Firstname == "" || req.Lastname == "" || req.Role <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if !validation.CheckEmail(req.Email) {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid email format")
		return
	}
	currentUser, ok := auth.UserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	targetUser, err := h.querier.GetUserByID(r.Context(), id)
	if err != nil {
		response.HandleDBError(w, err, "user")
		return
	}

	if targetUser.Role == auth.RoleAdmin && req.Role != auth.RoleAdmin {
		adminCount, err := h.querier.CountAdminUsers(r.Context(), auth.RoleAdmin)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to count admin users")
			return
		}
		if adminCount <= 1 {
			if currentUser.ID == id {
				response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "cannot remove your own last admin access")
				return
			}
			response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "cannot update last admin role")
			return
		}
	}

	row, err := h.querier.UpdateUser(r.Context(), sqlc.UpdateUserParams{
		ID:        id,
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		response.HandleDBError(w, err, "user")
		return
	}
	response.WriteJSON(w, http.StatusOK, UserResponse{
		Data: mapUpdateUserRow(row),
	})
}
func mapUpdateUserRow(row sqlc.UpdateUserRow) UserDTO {
	return mapUserRow(sqlc.ListUsersRow(row))
}
func mapUserRow(row sqlc.ListUsersRow) UserDTO {
	dto := UserDTO{
		ID:        row.ID,
		Email:     row.Email,
		Firstname: row.Firstname,
		Lastname:  row.Lastname,
		Role:      row.Role,
	}
	return dto
}
