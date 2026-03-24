package users

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"golang.org/x/crypto/bcrypt"
)

type fakeServiceQuerier struct {
	createUserFunc           func(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
	getUserByIDFunc          func(ctx context.Context, id int64) (sqlc.User, error)
	updateUserPasswordFunc   func(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error
	deleteSessionsByUserFunc func(ctx context.Context, userID int64) error
}

func (f fakeServiceQuerier) CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error) {
	if f.createUserFunc == nil {
		return sqlc.CreateUserRow{}, errors.New("unexpected CreateUser call")
	}
	return f.createUserFunc(ctx, arg)
}

func (f fakeServiceQuerier) GetUserByID(ctx context.Context, id int64) (sqlc.User, error) {
	if f.getUserByIDFunc == nil {
		return sqlc.User{}, errors.New("unexpected GetUserByID call")
	}
	return f.getUserByIDFunc(ctx, id)
}

func (f fakeServiceQuerier) UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error {
	if f.updateUserPasswordFunc == nil {
		return errors.New("unexpected UpdateUserPassword call")
	}
	return f.updateUserPasswordFunc(ctx, arg)
}

func (f fakeServiceQuerier) DeleteSessionsByUserID(ctx context.Context, userID int64) error {
	if f.deleteSessionsByUserFunc == nil {
		return errors.New("unexpected DeleteSessionsByUserID call")
	}
	return f.deleteSessionsByUserFunc(ctx, userID)
}

func TestServicePatchPasswordReturnsUnauthorizedWithoutUserInContext(t *testing.T) {
	service := NewService(fakeServiceQuerier{})

	err := service.PatchPassword(context.Background(), UpdatePasswordRequest{
		CurrentPassword: "old-secret",
		NewPassword:     "new-secret",
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestServicePatchPasswordReturnsInvalidCurrentPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to create password hash: %v", err)
	}

	service := NewService(fakeServiceQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:       id,
				Password: hash,
			}, nil
		},
	})

	ctx := auth.ContextWithUser(context.Background(), sqlc.User{ID: 10})
	err = service.PatchPassword(ctx, UpdatePasswordRequest{
		CurrentPassword: "wrong-secret",
		NewPassword:     "new-secret",
	})

	if !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Fatalf("expected ErrInvalidCurrentPassword, got %v", err)
	}
}

func TestServicePatchPasswordUpdatesHashAndDeletesSessions(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("old-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to create password hash: %v", err)
	}

	var updatedPassword []byte
	var deletedSessionsFor int64

	service := NewService(fakeServiceQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{
				ID:       id,
				Password: hash,
			}, nil
		},
		updateUserPasswordFunc: func(_ context.Context, arg sqlc.UpdateUserPasswordParams) error {
			if arg.ID != 10 {
				t.Fatalf("expected update for user 10, got %d", arg.ID)
			}
			updatedPassword = arg.Password
			return nil
		},
		deleteSessionsByUserFunc: func(_ context.Context, userID int64) error {
			deletedSessionsFor = userID
			return nil
		},
	})

	ctx := auth.ContextWithUser(context.Background(), sqlc.User{ID: 10})
	err = service.PatchPassword(ctx, UpdatePasswordRequest{
		CurrentPassword: "old-secret",
		NewPassword:     "new-secret",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if deletedSessionsFor != 10 {
		t.Fatalf("expected sessions deleted for user 10, got %d", deletedSessionsFor)
	}

	if len(updatedPassword) == 0 {
		t.Fatal("expected updated password hash")
	}

	if err := bcrypt.CompareHashAndPassword(updatedPassword, []byte("new-secret")); err != nil {
		t.Fatalf("expected password to be updated, got %v", err)
	}
}

func TestServicePatchPasswordReturnsUnauthorizedWhenUserDisappears(t *testing.T) {
	service := NewService(fakeServiceQuerier{
		getUserByIDFunc: func(_ context.Context, id int64) (sqlc.User, error) {
			return sqlc.User{}, pgx.ErrNoRows
		},
	})

	ctx := auth.ContextWithUser(context.Background(), sqlc.User{ID: 10})
	err := service.PatchPassword(ctx, UpdatePasswordRequest{
		CurrentPassword: "old-secret",
		NewPassword:     "new-secret",
	})

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
