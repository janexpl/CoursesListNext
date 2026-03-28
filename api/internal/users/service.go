package users

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
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
	ErrUnauthorized                   = errors.New("unauthorized")
	ErrInvalidCurrentPassword         = errors.New("invalid current password")
	ErrCannotDeleteCurrentUser        = errors.New("cannot delete current user")
	ErrCannotDeleteLastAdmin          = errors.New("cannot delete last admin")
	ErrCannotRemoveOwnLastAdminAccess = errors.New("cannot remove your own last admin access")
	ErrCannotUpdateLastAdminRole      = errors.New("cannot update last admin role")
)

type serviceQuerier interface {
	CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
	GetUserByID(ctx context.Context, id int64) (sqlc.User, error)
	CountAdminUsers(ctx context.Context, role int32) (int64, error)
	UpdateUser(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error)
	DeleteUser(ctx context.Context, id int64) (int64, error)
	UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error
	DeleteSessionsByUserID(ctx context.Context, userID int64) error
}

type txScope struct {
	queries  *sqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	queries  serviceQuerier
	recorder *auditlog.Recorder
	beginTx  func(context.Context) (txScope, error)
}

func NewService(queries serviceQuerier) *Service {
	return &Service{
		queries: queries,
	}
}

func NewServiceWithAudit(pool *pgxpool.Pool, queries *sqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		queries:  queries,
		recorder: recorder,
		beginTx: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}

			return txScope{
				queries:  queries.WithTx(tx),
				commit:   tx.Commit,
				rollback: tx.Rollback,
			}, nil
		},
	}
}

func (s *Service) PatchPassword(ctx context.Context, req UpdatePasswordRequest) error {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorized
	}

	if s.recorder != nil && s.beginTx != nil {
		return s.patchPasswordWithAudit(ctx, user.ID, req, map[string]any{"mode": "self_service"}, ErrUnauthorized)
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
	if s.recorder != nil && s.beginTx != nil {
		return s.patchPasswordWithAudit(ctx, userID, UpdatePasswordRequest{NewPassword: newPassword}, map[string]any{"mode": "admin_reset"}, pgx.ErrNoRows)
	}

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
	if s.recorder != nil && s.beginTx != nil {
		return s.createWithAudit(ctx, req)
	}

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

func (s *Service) UpdateProfile(ctx context.Context, req UpdateProfileRequest) (UserDTO, error) {
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return UserDTO{}, ErrUnauthorized
	}

	if s.recorder != nil && s.beginTx != nil {
		return s.updateUserWithAudit(ctx, currentUser.ID, req.Email, req.Firstname, req.Lastname, currentUser.Role, "profile_update", nil)
	}

	row, err := s.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        currentUser.ID,
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      currentUser.Role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	return mapUpdateUserRow(row), nil
}

func (s *Service) Update(ctx context.Context, userID int64, req UpdateUserRequest) (UserDTO, error) {
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return UserDTO{}, ErrUnauthorized
	}

	if s.recorder != nil && s.beginTx != nil {
		return s.updateManagedUserWithAudit(ctx, currentUser, userID, req)
	}

	targetUser, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}

	if err := validateLastAdminChange(ctx, s.queries, currentUser, targetUser, req.Role); err != nil {
		return UserDTO{}, err
	}

	row, err := s.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        userID,
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	return mapUpdateUserRow(row), nil
}

func (s *Service) Delete(ctx context.Context, userID int64) (int64, error) {
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return 0, ErrUnauthorized
	}
	if currentUser.ID == userID {
		return 0, ErrCannotDeleteCurrentUser
	}

	if s.recorder != nil && s.beginTx != nil {
		return s.deleteWithAudit(ctx, currentUser, userID)
	}

	targetUser, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	if targetUser.Role == auth.RoleAdmin {
		adminCount, err := s.queries.CountAdminUsers(ctx, auth.RoleAdmin)
		if err != nil {
			return 0, err
		}
		if adminCount <= 1 {
			return 0, ErrCannotDeleteLastAdmin
		}
	}

	deleted, err := s.queries.DeleteUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if deleted == 0 {
		return 0, pgx.ErrNoRows
	}

	return deleted, nil
}

func (s *Service) createWithAudit(ctx context.Context, req CreateUserRequest) (UserDTO, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserDTO{}, err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return UserDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	createdUser, err := tx.queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email:     req.Email,
		Password:  hashedPassword,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	createdSnapshot := mapCreateUserAuditSnapshot(createdUser)
	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "user",
		EntityID:   createdUser.ID,
		Action:     "create",
		Before:     nil,
		After:      createdSnapshot,
		Metadata:   nil,
	}); err != nil {
		return UserDTO{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return UserDTO{}, err
	}
	committed = true

	return createdSnapshot, nil
}

func (s *Service) updateUserWithAudit(ctx context.Context, userID int64, email, firstname, lastname string, role int32, action string, metadata map[string]any) (UserDTO, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return UserDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	beforeUser, err := tx.queries.GetUserByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}

	updatedUser, err := tx.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        userID,
		Email:     email,
		Firstname: firstname,
		Lastname:  lastname,
		Role:      role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	beforeSnapshot := mapUserAuditSnapshot(beforeUser)
	afterSnapshot := mapUpdateUserRow(updatedUser)
	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "user",
		EntityID:   userID,
		Action:     action,
		Before:     beforeSnapshot,
		After:      afterSnapshot,
		Metadata:   metadata,
	}); err != nil {
		return UserDTO{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return UserDTO{}, err
	}
	committed = true

	return afterSnapshot, nil
}

func (s *Service) updateManagedUserWithAudit(ctx context.Context, currentUser sqlc.User, userID int64, req UpdateUserRequest) (UserDTO, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return UserDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	targetUser, err := tx.queries.GetUserByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}

	if err := validateLastAdminChange(ctx, tx.queries, currentUser, targetUser, req.Role); err != nil {
		return UserDTO{}, err
	}

	updatedUser, err := tx.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        userID,
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
	})
	if err != nil {
		return UserDTO{}, err
	}

	beforeSnapshot := mapUserAuditSnapshot(targetUser)
	afterSnapshot := mapUpdateUserRow(updatedUser)
	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "user",
		EntityID:   userID,
		Action:     "update",
		Before:     beforeSnapshot,
		After:      afterSnapshot,
		Metadata:   nil,
	}); err != nil {
		return UserDTO{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return UserDTO{}, err
	}
	committed = true

	return afterSnapshot, nil
}

func (s *Service) deleteWithAudit(ctx context.Context, currentUser sqlc.User, userID int64) (int64, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	targetUser, err := tx.queries.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	if err := validateDeleteRules(ctx, tx.queries, currentUser, targetUser); err != nil {
		return 0, err
	}

	deleted, err := tx.queries.DeleteUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if deleted == 0 {
		return 0, pgx.ErrNoRows
	}

	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "user",
		EntityID:   userID,
		Action:     "delete",
		Before:     mapUserAuditSnapshot(targetUser),
		After:      nil,
		Metadata:   nil,
	}); err != nil {
		return 0, err
	}

	if err := tx.commit(ctx); err != nil {
		return 0, err
	}
	committed = true

	return deleted, nil
}

func (s *Service) patchPasswordWithAudit(ctx context.Context, userID int64, req UpdatePasswordRequest, metadata map[string]any, notFoundErr error) error {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	userDTO, err := tx.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFoundErr
		}
		return err
	}

	if req.CurrentPassword != "" {
		if err := bcrypt.CompareHashAndPassword(userDTO.Password, []byte(req.CurrentPassword)); err != nil {
			return ErrInvalidCurrentPassword
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err = tx.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:       userID,
		Password: hashedPassword,
	}); err != nil {
		return err
	}

	if err = tx.queries.DeleteSessionsByUserID(ctx, userID); err != nil {
		return err
	}

	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "user",
		EntityID:   userID,
		Action:     "password_change",
		Before:     nil,
		After:      mapUserAuditSnapshot(userDTO),
		Metadata:   metadata,
	}); err != nil {
		return err
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true

	return nil
}

func mapUserAuditSnapshot(user sqlc.User) UserDTO {
	return UserDTO{
		ID:        user.ID,
		Email:     user.Email,
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
		Role:      user.Role,
	}
}

func mapCreateUserAuditSnapshot(user sqlc.CreateUserRow) UserDTO {
	return UserDTO{
		ID:        user.ID,
		Email:     user.Email,
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
		Role:      user.Role,
	}
}

func validateLastAdminChange(ctx context.Context, queries serviceQuerier, currentUser sqlc.User, targetUser sqlc.User, nextRole int32) error {
	if targetUser.Role != auth.RoleAdmin || nextRole == auth.RoleAdmin {
		return nil
	}

	adminCount, err := queries.CountAdminUsers(ctx, auth.RoleAdmin)
	if err != nil {
		return err
	}
	if adminCount > 1 {
		return nil
	}
	if currentUser.ID == targetUser.ID {
		return ErrCannotRemoveOwnLastAdminAccess
	}
	return ErrCannotUpdateLastAdminRole
}

func validateDeleteRules(ctx context.Context, queries serviceQuerier, currentUser sqlc.User, targetUser sqlc.User) error {
	if currentUser.ID == targetUser.ID {
		return ErrCannotDeleteCurrentUser
	}
	if targetUser.Role != auth.RoleAdmin {
		return nil
	}

	adminCount, err := queries.CountAdminUsers(ctx, auth.RoleAdmin)
	if err != nil {
		return err
	}
	if adminCount <= 1 {
		return ErrCannotDeleteLastAdmin
	}
	return nil
}
