package apikeys

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	getUser  func(ctx context.Context, id int64) (sqlc.User, error)
	getAPIKe func(ctx context.Context, id int64) (sqlc.GetAPIKeyByIDRow, error)
}

func (f fakeQuerier) GetUserByID(ctx context.Context, id int64) (sqlc.User, error) {
	if f.getUser == nil {
		return sqlc.User{}, errors.New("unexpected user lookup")
	}
	return f.getUser(ctx, id)
}

func (f fakeQuerier) GetAPIKeyByID(ctx context.Context, id int64) (sqlc.GetAPIKeyByIDRow, error) {
	if f.getAPIKe == nil {
		return sqlc.GetAPIKeyByIDRow{}, errors.New("unexpected api key lookup")
	}
	return f.getAPIKe(ctx, id)
}

func serviceAccount() sqlc.User {
	return sqlc.User{ID: 42, Email: "integracja@example.com", Firstname: "Konto", Lastname: "Serwisowe"}
}

// serviceWithExistingAccount buduje serwis, w którym konto serwisowe istnieje,
// ale transakcja nie jest osiągalna - wystarczy do testów walidacji, która
// wykonuje się przed jej rozpoczęciem.
func serviceWithExistingAccount() *Service {
	return &Service{
		queries: fakeQuerier{
			getUser: func(_ context.Context, id int64) (sqlc.User, error) {
				user := serviceAccount()
				user.ID = id
				return user, nil
			},
		},
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	_, _, err := serviceWithExistingAccount().Create(context.Background(), CreateAPIKeyRequest{
		Name:   "   ",
		UserID: 42,
		Scopes: []string{auth.ScopeStudentsRead},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateRejectsMissingUser(t *testing.T) {
	service := &Service{
		queries: fakeQuerier{
			getUser: func(context.Context, int64) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		},
	}

	_, _, err := service.Create(context.Background(), CreateAPIKeyRequest{
		Name:   "integracja",
		UserID: 999,
		Scopes: []string{auth.ScopeStudentsRead},
	})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestCreateRejectsUnknownScope(t *testing.T) {
	_, _, err := serviceWithExistingAccount().Create(context.Background(), CreateAPIKeyRequest{
		Name:   "integracja",
		UserID: 42,
		Scopes: []string{"students:destroy"},
	})
	if !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("expected ErrUnknownScope, got %v", err)
	}
}

func TestCreateRejectsEmptyScopes(t *testing.T) {
	// Klucz bez zakresów wyglądałby na działający, a nie miałby dostępu do
	// niczego - lepiej odrzucić go od razu.
	for _, scopes := range [][]string{nil, {}, {"  "}} {
		_, _, err := serviceWithExistingAccount().Create(context.Background(), CreateAPIKeyRequest{
			Name:   "integracja",
			UserID: 42,
			Scopes: scopes,
		})
		if !errors.Is(err, ErrNoScopes) {
			t.Fatalf("expected ErrNoScopes for %v, got %v", scopes, err)
		}
	}
}

func TestCreateRejectsPastExpiry(t *testing.T) {
	_, _, err := serviceWithExistingAccount().Create(context.Background(), CreateAPIKeyRequest{
		Name:      "integracja",
		UserID:    42,
		Scopes:    []string{auth.ScopeStudentsRead},
		ExpiresAt: time.Now().AddDate(0, 0, -1).Format(response.DateFormat),
	})
	if !errors.Is(err, ErrExpiryInPast) {
		t.Fatalf("expected ErrExpiryInPast, got %v", err)
	}
}

func TestCreateRejectsMalformedExpiry(t *testing.T) {
	_, _, err := serviceWithExistingAccount().Create(context.Background(), CreateAPIKeyRequest{
		Name:      "integracja",
		UserID:    42,
		Scopes:    []string{auth.ScopeStudentsRead},
		ExpiresAt: "31-12-2027",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNormalizeScopesDeduplicatesAndSorts(t *testing.T) {
	scopes, err := normalizeScopes([]string{
		auth.ScopeStudentsWrite,
		" " + auth.ScopeCoursesRead + " ",
		auth.ScopeStudentsWrite,
		"",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes after deduplication, got %v", scopes)
	}
	if scopes[0] != auth.ScopeCoursesRead || scopes[1] != auth.ScopeStudentsWrite {
		t.Fatalf("expected sorted scopes, got %v", scopes)
	}
}

func TestParseExpiryCoversWholeDay(t *testing.T) {
	// Klucz ważny "do 31 grudnia" musi działać przez cały 31 grudnia.
	day := time.Now().AddDate(0, 0, 7)
	expiry, err := parseExpiry(day.Format(response.DateFormat))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expiry.Valid {
		t.Fatal("expected a valid timestamp")
	}
	if expiry.Time.Year() != day.Year() || expiry.Time.YearDay() != day.YearDay() {
		t.Fatalf("expiry must stay on the requested day, got %v", expiry.Time)
	}
	if expiry.Time.Hour() != 23 || expiry.Time.Minute() != 59 {
		t.Fatalf("expected end of day, got %v", expiry.Time)
	}
}

func TestParseExpiryEmptyMeansNoExpiry(t *testing.T) {
	expiry, err := parseExpiry("  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expiry.Valid {
		t.Fatal("blank expiry must produce a NULL timestamp")
	}
}

func TestRevokeRejectsAlreadyRevokedKey(t *testing.T) {
	service := &Service{
		queries: fakeQuerier{
			getAPIKe: func(context.Context, int64) (sqlc.GetAPIKeyByIDRow, error) {
				row := sqlc.GetAPIKeyByIDRow{ID: 7, Name: "stary klucz"}
				row.RevokedAt.Time = time.Now().Add(-time.Hour)
				row.RevokedAt.Valid = true
				return row, nil
			},
		},
	}

	if err := service.Revoke(context.Background(), 7); !errors.Is(err, ErrAlreadyRevoked) {
		t.Fatalf("expected ErrAlreadyRevoked, got %v", err)
	}
}

func TestRevokePropagatesMissingKey(t *testing.T) {
	service := &Service{
		queries: fakeQuerier{
			getAPIKe: func(context.Context, int64) (sqlc.GetAPIKeyByIDRow, error) {
				return sqlc.GetAPIKeyByIDRow{}, pgx.ErrNoRows
			},
		},
	}

	if err := service.Revoke(context.Background(), 7); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows so the handler can answer 404, got %v", err)
	}
}

func TestMapListRowNeverExposesRawToken(t *testing.T) {
	dto := mapListRow(sqlc.ListAPIKeysRow{
		ID:            7,
		Name:          "integracja",
		Prefix:        "clk_abcd1234",
		UserID:        42,
		UserEmail:     "integracja@example.com",
		UserFirstname: "Konto",
		UserLastname:  "Serwisowe",
	})

	if dto.Prefix != "clk_abcd1234" {
		t.Fatalf("expected the display prefix, got %q", dto.Prefix)
	}
	if dto.UserName != "Konto Serwisowe" {
		t.Fatalf("expected a joined user name, got %q", dto.UserName)
	}
	if dto.Scopes == nil {
		t.Fatal("scopes must serialize as [] rather than null")
	}
	if !strings.HasPrefix(dto.Prefix, auth.APIKeyTokenPrefix) {
		t.Fatalf("expected key prefix marker, got %q", dto.Prefix)
	}
}
