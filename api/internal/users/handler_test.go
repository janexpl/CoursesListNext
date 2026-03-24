package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	listUsersFunc       func(ctx context.Context) ([]sqlc.ListUsersRow, error)
	deleteUserFunc      func(ctx context.Context, id int64) (int64, error)
	getUserByIDFunc     func(ctx context.Context, id int64) (sqlc.User, error)
	countAdminUsersFunc func(ctx context.Context, role int32) (int64, error)
	updateUserFunc      func(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error)
}

type fakeCreator struct {
	createFunc               func(ctx context.Context, req CreateUserRequest) (UserDTO, error)
	patchPasswordFunc        func(ctx context.Context, req UpdatePasswordRequest) error
	patchPasswordByAdminFunc func(ctx context.Context, userID int64, newPassword string) error
}

func (f fakeQuerier) ListUsers(ctx context.Context) ([]sqlc.ListUsersRow, error) {
	if f.listUsersFunc == nil {
		return nil, errors.New("unexpected ListUsers call")
	}
	return f.listUsersFunc(ctx)
}

func (f fakeQuerier) DeleteUser(ctx context.Context, id int64) (int64, error) {
	if f.deleteUserFunc == nil {
		return 0, errors.New("unexpected DeleteUser call")
	}
	return f.deleteUserFunc(ctx, id)
}

func (f fakeQuerier) GetUserByID(ctx context.Context, id int64) (sqlc.User, error) {
	if f.getUserByIDFunc == nil {
		return sqlc.User{}, errors.New("unexpected GetUserByID call")
	}
	return f.getUserByIDFunc(ctx, id)
}

func (f fakeQuerier) CountAdminUsers(ctx context.Context, role int32) (int64, error) {
	if f.countAdminUsersFunc == nil {
		return 0, errors.New("unexpected CountAdminUsers call")
	}
	return f.countAdminUsersFunc(ctx, role)
}

func (f fakeQuerier) UpdateUser(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
	if f.updateUserFunc == nil {
		return sqlc.UpdateUserRow{}, errors.New("unexpected UpdateUser call")
	}
	return f.updateUserFunc(ctx, arg)
}

func (f fakeCreator) Create(ctx context.Context, req CreateUserRequest) (UserDTO, error) {
	if f.createFunc == nil {
		return UserDTO{}, errors.New("unexpected Create call")
	}
	return f.createFunc(ctx, req)
}

func (f fakeCreator) PatchPassword(ctx context.Context, req UpdatePasswordRequest) error {
	if f.patchPasswordFunc == nil {
		return errors.New("unexpected PatchPassword call")
	}
	return f.patchPasswordFunc(ctx, req)
}

func (f fakeCreator) PatchPasswordByAdmin(ctx context.Context, userID int64, newPassword string) error {
	if f.patchPasswordByAdminFunc == nil {
		return errors.New("unexpected PatchPasswordByAdmin call")
	}
	return f.patchPasswordByAdminFunc(ctx, userID, newPassword)
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if responseBody.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, responseBody.Error.Code)
	}
}

func TestCreateUserReturnsCreatedResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, req CreateUserRequest) (UserDTO, error) {
			if req.Email != "admin@example.com" || req.Password != "secret123" || req.Firstname != "Jan" || req.Lastname != "Nowak" || req.Role != auth.RoleAdmin {
				t.Fatalf("unexpected create request: %+v", req)
			}
			return UserDTO{
				ID:        15,
				Email:     req.Email,
				Firstname: req.Firstname,
				Lastname:  req.Lastname,
				Role:      req.Role,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{
		"email": " admin@example.com ",
		"password": " secret123 ",
		"firstName": " Jan ",
		"lastName": " Nowak ",
		"role": 1
	}`))
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 || responseBody.Data.Email != "admin@example.com" {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestCreateUserReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, req CreateUserRequest) (UserDTO, error) {
			t.Fatalf("Create should not be called for invalid JSON, got %+v", req)
			return UserDTO{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateUserReturnsBadRequestForMissingRequiredFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, req CreateUserRequest) (UserDTO, error) {
			t.Fatalf("Create should not be called for invalid body, got %+v", req)
			return UserDTO{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{
		"email": "",
		"password": "",
		"firstName": "",
		"lastName": "",
		"role": 0
	}`))
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateUserReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, req CreateUserRequest) (UserDTO, error) {
			t.Fatalf("Create should not be called for unknown field, got %+v", req)
			return UserDTO{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{
		"email": "admin@example.com",
		"password": "secret123",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 1,
		"extra": "oops"
	}`))
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateUserReturnsInternalServerErrorWhenCreateFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		createFunc: func(_ context.Context, req CreateUserRequest) (UserDTO, error) {
			return UserDTO{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{
		"email": "admin@example.com",
		"password": "secret123",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 1
	}`))
	rec := httptest.NewRecorder()

	handler.CreateUser(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestListReturnsUsersResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listUsersFunc: func(_ context.Context) ([]sqlc.ListUsersRow, error) {
			return []sqlc.ListUsersRow{
				{
					ID:        1,
					Email:     "admin@example.com",
					Firstname: "Jan",
					Lastname:  "Nowak",
					Role:      auth.RoleAdmin,
				},
				{
					ID:        2,
					Email:     "user@example.com",
					Firstname: "Anna",
					Lastname:  "Kowalska",
					Role:      2,
				},
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListUsersResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 2 {
		t.Fatalf("expected 2 users, got %d", len(responseBody.Data))
	}
	if responseBody.Data[0].Email != "admin@example.com" || responseBody.Data[1].Email != "user@example.com" {
		t.Fatalf("unexpected response body: %+v", responseBody.Data)
	}
}

func TestListReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listUsersFunc: func(_ context.Context) ([]sqlc.ListUsersRow, error) {
			return nil, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestDeleteReturnsDeletedUserResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			if id != 15 {
				t.Fatalf("expected target id 15, got %d", id)
			}
			return sqlc.User{
				ID:   15,
				Role: 2,
			}, nil
		},
		deleteUserFunc: func(_ context.Context, id int64) (int64, error) {
			if id != 15 {
				t.Fatalf("expected delete id 15, got %d", id)
			}
			return 1, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			t.Fatalf("CountAdminUsers should not be called for non-admin target, got role %d", role)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody DeleteUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 {
		t.Fatalf("expected deleted user id 15, got %d", responseBody.Data.ID)
	}
}

func TestDeleteReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for invalid id, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestDeleteReturnsUnauthorizedWithoutUserInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called without current user, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestDeleteReturnsForbiddenForCurrentUser(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for self-delete, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1", nil)
	req.SetPathValue("id", "1")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestDeleteReturnsNotFoundWhenUserDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{}, pgx.ErrNoRows
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestDeleteReturnsForbiddenWhenDeletingLastAdmin(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: auth.RoleAdmin,
			}, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			if role != auth.RoleAdmin {
				t.Fatalf("expected admin role %d, got %d", auth.RoleAdmin, role)
			}
			return 1, nil
		},
		deleteUserFunc: func(_ context.Context, id int64) (int64, error) {
			t.Fatalf("DeleteUser should not be called for last admin, got %d", id)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestDeleteReturnsInternalServerErrorWhenCountAdminsFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: auth.RoleAdmin,
			}, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			return 0, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestDeleteReturnsInternalServerErrorWhenDeleteFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: 2,
			}, nil
		},
		deleteUserFunc: func(_ context.Context, id int64) (int64, error) {
			return 0, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/15", nil)
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchReturnsUpdatedUserResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			if id != 15 {
				t.Fatalf("expected target id 15, got %d", id)
			}
			return sqlc.User{
				ID:   15,
				Role: 2,
			}, nil
		},
		updateUserFunc: func(_ context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
			if arg.ID != 15 || arg.Email != "edited@example.com" || arg.Firstname != "Jan" || arg.Lastname != "Nowak" || arg.Role != 2 {
				t.Fatalf("unexpected update params: %+v", arg)
			}
			return sqlc.UpdateUserRow{
				ID:        15,
				Email:     arg.Email,
				Firstname: arg.Firstname,
				Lastname:  arg.Lastname,
				Role:      arg.Role,
			}, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			t.Fatalf("CountAdminUsers should not be called for non-admin target, got role %d", role)
			return 0, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": " edited@example.com ",
		"firstName": " Jan ",
		"lastName": " Nowak ",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.ID != 15 || responseBody.Data.Email != "edited@example.com" {
		t.Fatalf("unexpected updated user payload: %+v", responseBody.Data)
	}
}

func TestPatchReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for invalid id, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/abc", strings.NewReader(`{}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for invalid JSON, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForMissingRequiredFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for invalid body, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "",
		"firstName": "",
		"lastName": "",
		"role": 0
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsBadRequestForUnknownField(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called for unknown field, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2,
		"extra": "oops"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchReturnsUnauthorizedWithoutUserInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			t.Fatalf("GetUserByID should not be called without current user, got %d", id)
			return sqlc.User{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestPatchReturnsNotFoundWhenUserDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{}, pgx.ErrNoRows
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchReturnsForbiddenWhenUpdatingLastAdminRole(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: auth.RoleAdmin,
			}, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			if role != auth.RoleAdmin {
				t.Fatalf("expected role %d, got %d", auth.RoleAdmin, role)
			}
			return 1, nil
		},
		updateUserFunc: func(_ context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
			t.Fatalf("UpdateUser should not be called when demoting last admin, got %+v", arg)
			return sqlc.UpdateUserRow{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestPatchReturnsInternalServerErrorWhenCountAdminsFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: auth.RoleAdmin,
			}, nil
		},
		countAdminUsersFunc: func(_ context.Context, role int32) (int64, error) {
			return 0, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchReturnsNotFoundWhenUpdateQueryDoesNotFindUser(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: 2,
			}, nil
		},
		updateUserFunc: func(_ context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
			return sqlc.UpdateUserRow{}, pgx.ErrNoRows
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchReturnsInternalServerErrorWhenUpdateFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:   15,
				Role: 2,
			}, nil
		},
		updateUserFunc: func(_ context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
			return sqlc.UpdateUserRow{}, errors.New("db error")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15", strings.NewReader(`{
		"email": "edited@example.com",
		"firstName": "Jan",
		"lastName": "Nowak",
		"role": 2
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.Patch(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchPasswordReturnsNoContent(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(ctx context.Context, req UpdatePasswordRequest) error {
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				t.Fatal("expected user in context")
			}
			if user.ID != 10 {
				t.Fatalf("expected user id 10, got %d", user.ID)
			}
			if req.CurrentPassword != "old-secret" || req.NewPassword != "new-secret" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword": "old-secret",
		"newPassword": "new-secret"
	}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   10,
		Role: 2,
	}))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestPatchPasswordReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(_ context.Context, req UpdatePasswordRequest) error {
			t.Fatalf("PatchPassword should not be called for invalid JSON, got %+v", req)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordReturnsBadRequestForMissingRequiredFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(_ context.Context, req UpdatePasswordRequest) error {
			t.Fatalf("PatchPassword should not be called for invalid body, got %+v", req)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword": "",
		"newPassword": ""
	}`))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordReturnsUnauthorizedWhenUserIsMissingInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(_ context.Context, req UpdatePasswordRequest) error {
			return ErrUnauthorized
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword": "old-secret",
		"newPassword": "new-secret"
	}`))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestPatchPasswordReturnsBadRequestForInvalidCurrentPassword(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(_ context.Context, req UpdatePasswordRequest) error {
			return ErrInvalidCurrentPassword
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword": "wrong-secret",
		"newPassword": "new-secret"
	}`))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordReturnsInternalServerErrorWhenServiceFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordFunc: func(_ context.Context, req UpdatePasswordRequest) error {
			return errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", strings.NewReader(`{
		"currentPassword": "old-secret",
		"newPassword": "new-secret"
	}`))
	rec := httptest.NewRecorder()

	handler.PatchPassword(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestPatchPasswordByAdminReturnsNoContent(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(ctx context.Context, userID int64, newPassword string) error {
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				t.Fatal("expected user in context")
			}
			if user.ID != 1 || user.Role != auth.RoleAdmin {
				t.Fatalf("unexpected current user: %+v", user)
			}
			if userID != 15 {
				t.Fatalf("expected userID 15, got %d", userID)
			}
			if newPassword != "new-secret" {
				t.Fatalf("expected trimmed password, got %q", newPassword)
			}
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{
		"newPassword": " new-secret "
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestPatchPasswordByAdminReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			t.Fatalf("PatchPasswordByAdmin should not be called, got userID=%d password=%q", userID, newPassword)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/abc/password", strings.NewReader(`{
		"newPassword": "new-secret"
	}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordByAdminReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			t.Fatalf("PatchPasswordByAdmin should not be called, got userID=%d password=%q", userID, newPassword)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordByAdminReturnsBadRequestForTooShortPassword(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			t.Fatalf("PatchPasswordByAdmin should not be called, got userID=%d password=%q", userID, newPassword)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{
		"newPassword": "short"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestPatchPasswordByAdminReturnsUnauthorizedWhenUserIsMissingInContext(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			t.Fatalf("PatchPasswordByAdmin should not be called, got userID=%d password=%q", userID, newPassword)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{
		"newPassword": "new-secret"
	}`))
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestPatchPasswordByAdminReturnsForbiddenForCurrentUser(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			t.Fatalf("PatchPasswordByAdmin should not be called, got userID=%d password=%q", userID, newPassword)
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1/password", strings.NewReader(`{
		"newPassword": "new-secret"
	}`))
	req.SetPathValue("id", "1")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestPatchPasswordByAdminReturnsNotFoundWhenUserDoesNotExist(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			return pgx.ErrNoRows
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{
		"newPassword": "new-secret"
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestPatchPasswordByAdminReturnsInternalServerErrorWhenServiceFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{}, fakeCreator{
		patchPasswordByAdminFunc: func(_ context.Context, userID int64, newPassword string) error {
			return errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/15/password", strings.NewReader(`{
		"newPassword": "new-secret"
	}`))
	req.SetPathValue("id", "15")
	req = req.WithContext(auth.ContextWithUser(req.Context(), sqlc.User{
		ID:   1,
		Role: auth.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	handler.PatchPasswordByAdmin(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
