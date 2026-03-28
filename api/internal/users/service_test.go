package users

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"golang.org/x/crypto/bcrypt"
)

type fakeTxDB struct {
	exec     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeTxDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if f.exec == nil {
		return pgconn.CommandTag{}, errors.New("unexpected exec call")
	}
	return f.exec(ctx, sql, args...)
}

func (f fakeTxDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if f.query == nil {
		return nil, errors.New("unexpected query call")
	}
	return f.query(ctx, sql, args...)
}

func (f fakeTxDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if f.queryRow == nil {
		return fakeTxRow{err: errors.New("unexpected query row call")}
	}
	return f.queryRow(ctx, sql, args...)
}

type fakeTxRow struct {
	scan func(dest ...interface{}) error
	err  error
}

func (r fakeTxRow) Scan(dest ...interface{}) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return r.err
}

type fakeServiceQuerier struct {
	createUserFunc           func(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
	getUserByIDFunc          func(ctx context.Context, id int64) (sqlc.User, error)
	countAdminUsersFunc      func(ctx context.Context, role int32) (int64, error)
	updateUserFunc           func(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error)
	deleteUserFunc           func(ctx context.Context, id int64) (int64, error)
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

func (f fakeServiceQuerier) CountAdminUsers(ctx context.Context, role int32) (int64, error) {
	if f.countAdminUsersFunc == nil {
		return 0, errors.New("unexpected CountAdminUsers call")
	}
	return f.countAdminUsersFunc(ctx, role)
}

func (f fakeServiceQuerier) UpdateUser(ctx context.Context, arg sqlc.UpdateUserParams) (sqlc.UpdateUserRow, error) {
	if f.updateUserFunc == nil {
		return sqlc.UpdateUserRow{}, errors.New("unexpected UpdateUser call")
	}
	return f.updateUserFunc(ctx, arg)
}

func (f fakeServiceQuerier) DeleteUser(ctx context.Context, id int64) (int64, error) {
	if f.deleteUserFunc == nil {
		return 0, errors.New("unexpected DeleteUser call")
	}
	return f.deleteUserFunc(ctx, id)
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

func TestServiceCreateWithAuditRecordsCreateEntry(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        99,
		Email:     "admin@example.com",
		Firstname: "Admin",
		Lastname:  "User",
	})

	auditRecorded := false
	commitCalled := false
	rollbackCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "INSERT INTO users"):
							if args[0] != "new@example.com" || args[2] != "Jan" || args[3] != "Nowak" || args[4] != int32(2) {
								return fakeTxRow{err: errors.New("unexpected create user args")}
							}
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 17
								*(dest[1].(*string)) = "new@example.com"
								*(dest[2].(*string)) = "Jan"
								*(dest[3].(*string)) = "Nowak"
								*(dest[4].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							if args[0] != "user" || args[1] != int64(17) || args[2] != "create" {
								return fakeTxRow{err: errors.New("unexpected audit args prefix")}
							}

							var after UserDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								return fakeTxRow{err: err}
							}
							if after.Email != "new@example.com" || after.Firstname != "Jan" || after.Lastname != "Nowak" || after.Role != 2 {
								return fakeTxRow{err: errors.New("unexpected audit after payload")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 44
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error {
					rollbackCalled = true
					return nil
				},
			}, nil
		},
	}

	created, err := service.Create(ctx, CreateUserRequest{
		Email:     "new@example.com",
		Password:  "secret-123",
		Firstname: "Jan",
		Lastname:  "Nowak",
		Role:      2,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID != 17 {
		t.Fatalf("expected created user id 17, got %d", created.ID)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
	if rollbackCalled {
		t.Fatal("did not expect rollback after successful commit")
	}
}

func TestServicePatchPasswordWithAuditRecordsPasswordChange(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("old-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to create password hash: %v", err)
	}

	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        10,
		Email:     "user@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
	})

	auditRecorded := false
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, email, password, firstname, lastname, role FROM users"):
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 10
								*(dest[1].(*string)) = "user@example.com"
								*(dest[2].(*[]byte)) = hash
								*(dest[3].(*string)) = "Jan"
								*(dest[4].(*string)) = "Nowak"
								*(dest[5].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							var after UserDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								return fakeTxRow{err: err}
							}
							var metadata map[string]any
							if err := json.Unmarshal(args[9].([]byte), &metadata); err != nil {
								return fakeTxRow{err: err}
							}
							if args[0] != "user" || args[1] != int64(10) || args[2] != "password_change" {
								return fakeTxRow{err: errors.New("unexpected audit args prefix")}
							}
							if after.Email != "user@example.com" || metadata["mode"] != "self_service" {
								return fakeTxRow{err: errors.New("unexpected audit payload")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 45
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
					exec: func(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
						switch {
						case strings.Contains(sql, "UPDATE users"):
							if args[0] != int64(10) {
								return pgconn.CommandTag{}, errors.New("unexpected update user password args")
							}
							return pgconn.CommandTag{}, nil
						case strings.Contains(sql, "DELETE FROM api_sessions"):
							if args[0] != int64(10) {
								return pgconn.CommandTag{}, errors.New("unexpected delete sessions args")
							}
							return pgconn.CommandTag{}, nil
						default:
							return pgconn.CommandTag{}, errors.New("unexpected exec call")
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	err = service.PatchPassword(ctx, UpdatePasswordRequest{
		CurrentPassword: "old-secret",
		NewPassword:     "new-secret-123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
}

func TestServicePatchPasswordByAdminWithAuditRecordsPasswordChange(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        1,
		Email:     "admin@example.com",
		Firstname: "Admin",
		Lastname:  "User",
	})

	auditRecorded := false
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, email, password, firstname, lastname, role FROM users"):
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 20
								*(dest[1].(*string)) = "target@example.com"
								*(dest[2].(*[]byte)) = []byte("ignored")
								*(dest[3].(*string)) = "Anna"
								*(dest[4].(*string)) = "Kowalska"
								*(dest[5].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							if args[0] != "user" || args[1] != int64(20) || args[2] != "password_change" {
								return fakeTxRow{err: errors.New("unexpected audit args prefix")}
							}
							var after UserDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								return fakeTxRow{err: err}
							}
							var metadata map[string]any
							if err := json.Unmarshal(args[9].([]byte), &metadata); err != nil {
								return fakeTxRow{err: err}
							}
							if after.Email != "target@example.com" || after.Firstname != "Anna" || metadata["mode"] != "admin_reset" {
								return fakeTxRow{err: errors.New("unexpected audit payload")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 46
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
					exec: func(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
						switch {
						case strings.Contains(sql, "UPDATE users"), strings.Contains(sql, "DELETE FROM api_sessions"):
							if args[0] != int64(20) {
								return pgconn.CommandTag{}, errors.New("unexpected exec args")
							}
							return pgconn.CommandTag{}, nil
						default:
							return pgconn.CommandTag{}, errors.New("unexpected exec call")
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	err := service.PatchPasswordByAdmin(ctx, 20, "new-secret-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
}

func TestServiceUpdateProfileWithAuditRecordsProfileUpdate(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        10,
		Email:     "old@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
		Role:      2,
	})

	auditRecorded := false
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, email, password, firstname, lastname, role FROM users"):
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 10
								*(dest[1].(*string)) = "old@example.com"
								*(dest[2].(*[]byte)) = []byte("ignored")
								*(dest[3].(*string)) = "Jan"
								*(dest[4].(*string)) = "Nowak"
								*(dest[5].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "UPDATE users"):
							if args[0] != int64(10) || args[1] != "new@example.com" || args[2] != "Janusz" || args[3] != "Nowakowski" || args[4] != int32(2) {
								return fakeTxRow{err: errors.New("unexpected update profile args")}
							}
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 10
								*(dest[1].(*string)) = "new@example.com"
								*(dest[2].(*string)) = "Janusz"
								*(dest[3].(*string)) = "Nowakowski"
								*(dest[4].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							var before UserDTO
							if err := json.Unmarshal(args[7].([]byte), &before); err != nil {
								return fakeTxRow{err: err}
							}
							var after UserDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								return fakeTxRow{err: err}
							}
							if args[2] != "profile_update" || before.Email != "old@example.com" || after.Email != "new@example.com" {
								return fakeTxRow{err: errors.New("unexpected profile audit payload")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 50
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	updated, err := service.UpdateProfile(ctx, UpdateProfileRequest{
		Email:     "new@example.com",
		Firstname: "Janusz",
		Lastname:  "Nowakowski",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Fatalf("unexpected updated profile payload: %+v", updated)
	}
	if !auditRecorded || !commitCalled {
		t.Fatal("expected audit and commit for profile update")
	}
}

func TestServiceUpdateWithAuditRecordsUserUpdate(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{ID: 1, Role: auth.RoleAdmin})

	auditRecorded := false
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, email, password, firstname, lastname, role FROM users"):
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 15
								*(dest[1].(*string)) = "edited@example.com"
								*(dest[2].(*[]byte)) = []byte("ignored")
								*(dest[3].(*string)) = "Jan"
								*(dest[4].(*string)) = "Nowak"
								*(dest[5].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "UPDATE users"):
							if args[0] != int64(15) || args[4] != int32(2) {
								return fakeTxRow{err: errors.New("unexpected update user args")}
							}
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 15
								*(dest[1].(*string)) = "edited@example.com"
								*(dest[2].(*string)) = "Jan"
								*(dest[3].(*string)) = "Nowak"
								*(dest[4].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							if args[2] != "update" {
								return fakeTxRow{err: errors.New("unexpected audit action")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 51
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	updated, err := service.Update(ctx, 15, UpdateUserRequest{
		Email:     "edited@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
		Role:      2,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.ID != 15 {
		t.Fatalf("unexpected updated user payload: %+v", updated)
	}
	if !auditRecorded || !commitCalled {
		t.Fatal("expected audit and commit for user update")
	}
}

func TestServiceDeleteWithAuditRecordsDelete(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{ID: 1, Role: auth.RoleAdmin})

	auditRecorded := false
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeTxDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, email, password, firstname, lastname, role FROM users"):
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 15
								*(dest[1].(*string)) = "delete@example.com"
								*(dest[2].(*[]byte)) = []byte("ignored")
								*(dest[3].(*string)) = "Anna"
								*(dest[4].(*string)) = "Kowalska"
								*(dest[5].(*int32)) = 2
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							if args[2] != "delete" {
								return fakeTxRow{err: errors.New("unexpected audit action")}
							}
							auditRecorded = true
							return fakeTxRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 52
								return nil
							}}
						default:
							return fakeTxRow{err: errors.New("unexpected query row call")}
						}
					},
					exec: func(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
						if !strings.Contains(sql, "DELETE FROM users") || args[0] != int64(15) {
							return pgconn.CommandTag{}, errors.New("unexpected delete exec call")
						}
						return pgconn.NewCommandTag("DELETE 1"), nil
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	deleted, err := service.Delete(ctx, 15)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected deleted rows 1, got %d", deleted)
	}
	if !auditRecorded || !commitCalled {
		t.Fatal("expected audit and commit for user delete")
	}
}
