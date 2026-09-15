// Package integrationtest uruchamia API na prawdziwym PostgreSQL.
//
// Testy wymagają zmiennej TEST_DATABASE_URL wskazującej serwer, na którym wolno
// tworzyć bazy (np. postgres://janusz@127.0.0.1:5432/postgres?sslmode=disable).
// Bez niej są pomijane, więc `go test ./...` działa jak dotąd.
//
// Każde uruchomienie zakłada świeżą bazę, ładuje testdata/baseline_schema.sql
// (schemat bazy produkcyjnej po migracji 0018, bez danych) i nakłada na nią
// kolejne migracje z api/migrations - dzięki temu testy sprawdzają migracje
// dokładnie w tej postaci, w jakiej trafią na produkcję.
package integrationtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/server"
	"github.com/janexpl/CoursesListNext/api/internal/webhooks"
)

// Konfiguracja dispatchera webhooków w testach: krótkie opóźnienia zamiast godzin.
const (
	webhookTestRequestTimeout = 300 * time.Millisecond
	webhookTestRetryDelay     = 50 * time.Millisecond
	webhookTestPublicBaseURL  = "https://courseslist.test"
)

// notificationsToken to statyczny token tras /internal/notifications w testach.
const notificationsToken = "integration-notifications-token"

// decodeJSONBody dekoduje ciało odpowiedzi spoza helpera call.
func decodeJSONBody(resp *http.Response, dest any) error {
	return json.NewDecoder(resp.Body).Decode(dest)
}

// baselineMigration to numer ostatniej migracji zawartej w baseline_schema.sql.
const baselineMigration = 18

type testEnv struct {
	pool    *pgxpool.Pool
	server  *httptest.Server
	client  *http.Client
	apiKey  string
	adminDB string
	dbName  string

	stopDispatcher context.CancelFunc
}

var (
	env        *testEnv
	envErr     error
	seedNumber atomic.Int64
)

func TestMain(m *testing.M) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		os.Exit(m.Run())
	}

	env, envErr = setup(adminURL)
	code := m.Run()
	if env != nil {
		env.teardown()
	}
	os.Exit(code)
}

func requireEnv(t *testing.T) *testEnv {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL not set - skipping integration test")
	}
	if envErr != nil {
		t.Fatalf("integration environment setup failed: %v", envErr)
	}
	return env
}

func setup(adminURL string) (*testEnv, error) {
	ctx := context.Background()

	adminConn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return nil, fmt.Errorf("connect admin: %w", err)
	}
	defer adminConn.Close(ctx)

	dbName := fmt.Sprintf("courselist_it_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}
	e := &testEnv{adminDB: adminURL, dbName: dbName}

	connConfig, err := pgx.ParseConfig(adminURL)
	if err != nil {
		e.teardown()
		return nil, err
	}
	connConfig.Database = dbName

	if err := loadSchema(ctx, connConfig); err != nil {
		e.teardown()
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		e.teardown()
		return nil, err
	}
	poolConfig.ConnConfig.Database = dbName
	// Test współbieżności potrzebuje tylu połączeń, ilu równoległych żądań -
	// inaczej pula ustawiłaby transakcje w kolejce i test niczego by nie dowodził.
	poolConfig.MaxConns = 32
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		e.teardown()
		return nil, err
	}
	e.pool = pool

	e.apiKey, err = seedAPIKey(ctx, pool)
	if err != nil {
		e.teardown()
		return nil, err
	}

	cfg := &config.Config{
		SessionTTL:            time.Hour,
		SessionCookieName:     "session_token",
		LoginRateLimit:        600,
		NotificationsAPIToken: notificationsToken,
		PublicBaseURL:         webhookTestPublicBaseURL,
	}
	e.server = httptest.NewServer(server.NewRouter(server.Dependencies{
		Queries: dbsqlc.New(pool),
		Config:  cfg,
		Pool:    pool,
	}))
	dispatcherCtx, stopDispatcher := context.WithCancel(context.Background())
	e.stopDispatcher = stopDispatcher
	retryDelays := make([]time.Duration, 7)
	for i := range retryDelays {
		retryDelays[i] = webhookTestRetryDelay
	}
	go webhooks.NewDispatcher(pool, webhooks.Config{
		PollInterval:   webhookTestRetryDelay,
		RequestTimeout: webhookTestRequestTimeout,
		RetryDelays:    retryDelays,
		BatchSize:      20,
	}).Run(dispatcherCtx)

	e.client = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 64, MaxConnsPerHost: 64},
	}
	return e, nil
}

func (e *testEnv) teardown() {
	ctx := context.Background()
	if e.stopDispatcher != nil {
		e.stopDispatcher()
	}
	if e.server != nil {
		e.server.Close()
	}
	if e.pool != nil {
		e.pool.Close()
	}
	adminConn, err := pgx.Connect(ctx, e.adminDB)
	if err != nil {
		log.Printf("integrationtest: cannot drop %s: %v", e.dbName, err)
		return
	}
	defer adminConn.Close(ctx)
	if _, err := adminConn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{e.dbName}.Sanitize()+" WITH (FORCE)"); err != nil {
		log.Printf("integrationtest: cannot drop %s: %v", e.dbName, err)
	}
}

var migrationFilePattern = regexp.MustCompile(`^(\d{4})_.+\.sql$`)

func loadSchema(ctx context.Context, connConfig *pgx.ConnConfig) error {
	baseline, err := os.ReadFile(filepath.Join("testdata", "baseline_schema.sql"))
	if err != nil {
		return err
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, string(baseline)); err != nil {
		conn.Close(ctx)
		return fmt.Errorf("load baseline schema: %w", err)
	}
	conn.Close(ctx)

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		match := migrationFilePattern.FindStringSubmatch(filepath.Base(file))
		if match == nil {
			continue
		}
		number, _ := strconv.Atoi(match[1])
		if number <= baselineMigration {
			continue
		}
		if err := applyMigration(ctx, connConfig, file); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration odtwarza `psql --single-transaction -v ON_ERROR_STOP=1 -f`:
// cały plik w jednej transakcji, na świeżym połączeniu (baseline zeruje search_path).
func applyMigration(ctx context.Context, connConfig *pgx.ConnConfig, file string) error {
	sql, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("apply %s: %w", filepath.Base(file), err)
	}
	return tx.Commit(ctx)
}

// callWithKey wysyła żądanie innym kluczem API - do sprawdzania wymaganych zakresów.
func (e *testEnv) callWithKey(t *testing.T, key, method, path string, body any) apiResponse {
	t.Helper()
	original := e.apiKey
	e.apiKey = key
	defer func() { e.apiKey = original }()
	return e.mustCall(t, method, path, body, nil)
}

// seedScopedAPIKey tworzy klucz z podanymi zakresami na koncie administratora testów.
func (e *testEnv) seedScopedAPIKey(t *testing.T, scopes ...string) string {
	t.Helper()
	raw, prefix, hash, err := auth.NewAPIKeyToken()
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.pool.Exec(context.Background(), `
		INSERT INTO api_keys (name, prefix, token_hash, user_id, scopes)
		SELECT 'integration-scoped', $1, $2, id, $3 FROM users WHERE email = 'integration@example.com'`,
		prefix, hash, scopes)
	if err != nil {
		t.Fatalf("seed scoped api key: %v", err)
	}
	return raw
}

func seedAPIKey(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var userID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password, firstname, lastname, role)
		VALUES ('integration@example.com', '\x00', 'Integracja', 'Testowa', 1)
		RETURNING id`).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("seed user: %w", err)
	}
	raw, prefix, hash, err := auth.NewAPIKeyToken()
	if err != nil {
		return "", err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (name, prefix, token_hash, user_id, scopes)
		VALUES ('integration', $1, $2, $3, $4)`,
		prefix, hash, userID, auth.AssignableScopes())
	if err != nil {
		return "", fmt.Errorf("seed api key: %w", err)
	}
	return raw, nil
}

type apiResponse struct {
	Status int
	Body   []byte
	Header http.Header
}

func (r apiResponse) decode(t *testing.T, dest any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dest); err != nil {
		t.Fatalf("cannot decode response %q: %v", r.Body, err)
	}
}

func (r apiResponse) errorMessage(t *testing.T) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	r.decode(t, &body)
	return body.Error.Message
}

// call nie używa t.Fatal, żeby dało się go wołać z gorutyn testu współbieżności.
func (e *testEnv) call(method, path string, body any, headers map[string]string) (apiResponse, error) {
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(b)
	default:
		payload, err := json.Marshal(b)
		if err != nil {
			return apiResponse{}, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, e.server.URL+"/api/v1"+path, reader)
	if err != nil {
		return apiResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, err
	}
	recordResponse(method, path, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	return apiResponse{Status: resp.StatusCode, Body: data, Header: resp.Header}, nil
}

var recordMu sync.Mutex

// recordResponse dopisuje odpowiedź do pliku z IT_RECORD_RESPONSES (JSON Lines), żeby
// można było sprawdzić rzeczywiste odpowiedzi względem docs/api/openapi.yaml.
func recordResponse(method, path string, status int, contentType string, body []byte) {
	file := os.Getenv("IT_RECORD_RESPONSES")
	if file == "" || !strings.HasPrefix(contentType, "application/json") {
		return
	}
	line, err := json.Marshal(map[string]any{
		"method": method,
		"path":   "/api/v1" + strings.SplitN(path, "?", 2)[0],
		"status": status,
		"body":   json.RawMessage(body),
	})
	if err != nil {
		return
	}
	recordMu.Lock()
	defer recordMu.Unlock()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("integrationtest: cannot record response: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		log.Printf("integrationtest: cannot record response: %v", err)
	}
}

func (e *testEnv) mustCall(t *testing.T, method, path string, body any, headers map[string]string) apiResponse {
	t.Helper()
	resp, err := e.call(method, path, body, headers)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func nextSeed() int64 {
	return seedNumber.Add(1)
}

type seededCourse struct {
	ID     int64
	Symbol string
}

func (e *testEnv) seedCourse(t *testing.T) seededCourse {
	t.Helper()
	n := nextSeed()
	symbol := fmt.Sprintf("IT%d", n)
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO courses (mainname, name, symbol, expirytime, courseprogram, certfrontpage)
		VALUES ('Szkolenie', $1, $2, '5', '[{"Subject":"Temat","TheoryTime":"1","PracticeTime":"1"}]', '<p>front</p>')
		RETURNING id`, fmt.Sprintf("Kurs integracyjny %d", n), symbol).Scan(&id)
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}
	return seededCourse{ID: id, Symbol: symbol}
}

func (e *testEnv) seedStudent(t *testing.T) int64 {
	t.Helper()
	n := nextSeed()
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO students (firstname, lastname, birthdate, birthplace)
		VALUES ('Jan', $1, DATE '1990-01-10', 'Warszawa')
		RETURNING id`, fmt.Sprintf("Testowy%d", n)).Scan(&id)
	if err != nil {
		t.Fatalf("seed student: %v", err)
	}
	return id
}

func (e *testEnv) countRows(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var count int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
