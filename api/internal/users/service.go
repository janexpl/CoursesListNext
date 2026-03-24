package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	Firstname string `json:"firstName"`
	Lastname  string `json:"lastName"`
	Role      int32  `json:"role"`
}

type CreateUserResult struct {
	Data UserDTO `json:"data"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type AdminUpdatePasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

var (
	ErrUnauthorized           = errors.New("unauthorized")
	ErrInvalidCurrentPassword = errors.New("invalid current password")
)

type serviceQuerier interface {
	CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
	GetUserByID(ctx context.Context, id int64) (sqlc.User, error)
	UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error
	DeleteSessionsByUserID(ctx context.Context, userID int64) error
}

type Service struct {
	queries serviceQuerier
}

func NewService(queries serviceQuerier) *Service {
	return &Service{
		queries: queries,
	}
}

func (s *Service) PatchPassword(ctx context.Context, req UpdatePasswordRequest) error {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorized
	}

	userDTO, err := s.queries.GetUserByID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword(userDTO.Password, []byte(req.CurrentPassword)); err != nil {
		return ErrInvalidCurrentPassword
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err = s.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:       user.ID,
		Password: hashedPassword,
	}); err != nil {
		return err
	}
	if err = s.queries.DeleteSessionsByUserID(ctx, user.ID); err != nil {
		return err
	}
	return nil
}

func (s *Service) PatchPasswordByAdmin(ctx context.Context, userID int64, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err = s.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:       userID,
		Password: hashedPassword,
	}); err != nil {
		return err
	}
	if err = s.queries.DeleteSessionsByUserID(ctx, userID); err != nil {
		return err
	}
	return nil
}

func (s *Service) Create(ctx context.Context, req CreateUserRequest) (UserDTO, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserDTO{}, err
	}
	user, err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email:     req.Email,
		Password:  hashedPassword,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	return UserDTO{
		ID:        user.ID,
		Email:     user.Email,
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
		Role:      user.Role,
	}, nil

}
