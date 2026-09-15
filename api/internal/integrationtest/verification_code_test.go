package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/auth"
)

// Zlecenie, punkt 4: kod weryfikacyjny zaświadczenia.

// 12 znaków z alfabetu bez 0, O, 1, I, l (wielkie litery i cyfry 2-9).
var verificationCodePattern = regexp.MustCompile(`^[2-9A-HJ-NP-Z]{12}$`)

type certificateDetails struct {
	ID               int64  `json:"id"`
	RegistryNumber   int64  `json:"registryNumber"`
	VerificationCode string `json:"verificationCode"`
}

func (e *testEnv) getCertificate(t *testing.T, id int64) certificateDetails {
	t.Helper()
	resp := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", id), nil, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("GET certificate %d: expected 200, got %d: %s", id, resp.Status, resp.Body)
	}
	var body struct {
		Data certificateDetails `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func assertVerificationCode(t *testing.T, code string) {
	t.Helper()
	if !verificationCodePattern.MatchString(code) {
		t.Fatalf("verification code %q does not match %s", code, verificationCodePattern)
	}
}

func TestVerificationCodeReturnedOnCreateDetailsAndReplay(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	payload := certificatePayload(e.seedStudent(t), course.ID)
	headers := map[string]string{"Idempotency-Key": fmt.Sprintf("it-code-%d", nextSeed())}

	first := e.mustCall(t, http.MethodPost, "/certificates", payload, headers)
	if first.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", first.Status, first.Body)
	}
	var created struct {
		Data struct {
			ID               int64  `json:"id"`
			VerificationCode string `json:"verificationCode"`
		} `json:"data"`
	}
	first.decode(t, &created)
	assertVerificationCode(t, created.Data.VerificationCode)

	if details := e.getCertificate(t, created.Data.ID); details.VerificationCode != created.Data.VerificationCode {
		t.Fatalf("details code %q differs from create response %q", details.VerificationCode, created.Data.VerificationCode)
	}

	replay := e.mustCall(t, http.MethodPost, "/certificates", payload, headers)
	if replay.Status != http.StatusOK || strings.TrimSpace(string(replay.Body)) != strings.TrimSpace(string(first.Body)) {
		t.Fatalf("replay must return the original body with the code, got %d: %s", replay.Status, replay.Body)
	}
}

// Aplikacja webowa woła te same trasy z ciasteczkiem sesji zamiast klucza API.
func TestVerificationCodeAssignedForCertificateCreatedByWebSession(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	payload := certificatePayload(e.seedStudent(t), course.ID)
	payload["registryYear"] = 2026
	payload["registryNumber"] = 1

	cookie := e.seedSessionCookie(t)
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v1/certificates", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("web session create: expected 201, got %d: %s", resp.StatusCode, data)
	}
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	assertVerificationCode(t, e.getCertificate(t, created.Data.ID).VerificationCode)
}

func TestVerificationCodeAssignedForCertificateGeneratedFromJournal(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)

	journal := e.mustCall(t, http.MethodPost, "/journals", map[string]any{
		"courseId":       course.ID,
		"title":          "Dziennik integracyjny",
		"organizerName":  "Jan Kowalski",
		"location":       "Warszawa",
		"formOfTraining": "stacjonarna",
		"legalBasis":     "Podstawa",
		"dateStart":      "2026-03-10",
		"dateEnd":        "2026-03-12",
	}, nil)
	if journal.Status != http.StatusCreated {
		t.Fatalf("POST /journals: expected 201, got %d: %s", journal.Status, journal.Body)
	}
	journalID := decodeCreated(t, journal).ID

	attendee := e.mustCall(t, http.MethodPost, fmt.Sprintf("/journals/%d/attendees", journalID), map[string]any{"studentId": student}, nil)
	if attendee.Status != http.StatusCreated {
		t.Fatalf("POST attendee: expected 201, got %d: %s", attendee.Status, attendee.Body)
	}
	attendeeID := decodeCreated(t, attendee).ID

	generated := e.mustCall(t, http.MethodPost, fmt.Sprintf("/journals/%d/attendees/%d/certificate/generate", journalID, attendeeID), nil, nil)
	if generated.Status != http.StatusCreated {
		t.Fatalf("generate certificate: expected 201, got %d: %s", generated.Status, generated.Body)
	}
	assertVerificationCode(t, e.getCertificate(t, decodeCreated(t, generated).ID).VerificationCode)
}

// Dokument wstawiony z pominięciem API (stara aplikacja, ręczny INSERT) też dostaje kod.
func TestVerificationCodeAssignedByDatabaseDefaultAndImmutable(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	ctx := context.Background()

	var certificateID int64
	var code string
	err := e.pool.QueryRow(ctx, `
		WITH reg AS (
			INSERT INTO registries (course_id, year, number) VALUES ($1, 2026, 1) RETURNING id
		)
		INSERT INTO certificates (date, student_id, coursedatestart, registry_id,
			student_firstname_snapshot, student_lastname_snapshot, student_birthdate_snapshot,
			student_birthplace_snapshot, course_name_snapshot, course_symbol_snapshot,
			course_program_snapshot, cert_front_page_snapshot)
		SELECT DATE '2026-03-15', $2, DATE '2026-03-10', reg.id, 'Jan', 'Nowak', DATE '1990-01-10',
			'Warszawa', 'Kurs', 'K', '[]', '<p></p>'
		FROM reg
		RETURNING id, verification_code`, course.ID, student).Scan(&certificateID, &code)
	if err != nil {
		t.Fatalf("direct insert: %v", err)
	}
	assertVerificationCode(t, code)

	if _, err := e.pool.Exec(ctx, `UPDATE certificates SET verification_code = 'ABCDEFGHJKLM' WHERE id = $1`, certificateID); err == nil {
		t.Fatal("expected verification_code update to be rejected")
	}
	var after string
	if err := e.pool.QueryRow(ctx, `SELECT verification_code FROM certificates WHERE id = $1`, certificateID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != code {
		t.Fatalf("verification code changed from %q to %q", code, after)
	}

	// Zmiana innych pól (np. PATCH zaświadczenia) nie narusza kodu.
	if _, err := e.pool.Exec(ctx, `UPDATE certificates SET date = DATE '2026-03-16' WHERE id = $1`, certificateID); err != nil {
		t.Fatalf("update of other columns must still work: %v", err)
	}
}

func TestVerificationCodesAreUnique(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	const parallel = 12
	students := make([]int64, parallel)
	for i := range students {
		students[i] = e.seedStudent(t)
	}
	responses := runConcurrently(t, parallel, func(i int) (apiResponse, error) {
		return e.call(http.MethodPost, "/certificates", certificatePayload(students[i], course.ID), nil)
	})
	seen := map[string]bool{}
	for _, resp := range responses {
		if resp.Status != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
		}
		var created struct {
			Data struct {
				VerificationCode string `json:"verificationCode"`
			} `json:"data"`
		}
		resp.decode(t, &created)
		assertVerificationCode(t, created.Data.VerificationCode)
		if seen[created.Data.VerificationCode] {
			t.Fatalf("duplicated verification code %q", created.Data.VerificationCode)
		}
		seen[created.Data.VerificationCode] = true
	}

	if n := e.countRows(t, `SELECT count(*) FROM (SELECT verification_code FROM certificates GROUP BY 1 HAVING count(*) > 1) d`); n != 0 {
		t.Fatalf("found %d duplicated verification codes", n)
	}
	if n := e.countRows(t, `
		SELECT count(*) FROM pg_indexes
		WHERE tablename = 'certificates' AND indexdef LIKE 'CREATE UNIQUE INDEX%(verification_code)%'`); n != 1 {
		t.Fatalf("expected a unique index on certificates.verification_code, found %d", n)
	}
}

func TestGetCertificateByVerificationCode(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	created := e.mustCall(t, http.MethodPost, "/certificates", certificatePayload(e.seedStudent(t), course.ID), nil)
	if created.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Status, created.Body)
	}
	id := decodeCreated(t, created).ID
	details := e.getCertificate(t, id)

	byID := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", id), nil, nil)
	for _, code := range []string{details.VerificationCode, strings.ToLower(details.VerificationCode)} {
		resp := e.mustCall(t, http.MethodGet, "/certificates/by-verification-code/"+code, nil, nil)
		if resp.Status != http.StatusOK {
			t.Fatalf("code %q: expected 200, got %d: %s", code, resp.Status, resp.Body)
		}
		if strings.TrimSpace(string(resp.Body)) != strings.TrimSpace(string(byID.Body)) {
			t.Fatalf("by-verification-code body differs from GET /certificates/{id}:\n%s\n%s", resp.Body, byID.Body)
		}
	}

	if resp := e.mustCall(t, http.MethodGet, "/certificates/by-verification-code/ZZZZZZZZZZZZ", nil, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown code: expected 404, got %d: %s", resp.Status, resp.Body)
	}
	if resp := e.mustCall(t, http.MethodGet, "/certificates/by-verification-code/abc", nil, nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("malformed code: expected 400, got %d: %s", resp.Status, resp.Body)
	}
	readOnlyOther := e.seedScopedAPIKey(t, "students:read")
	if resp := e.callWithKey(t, readOnlyOther, http.MethodGet, "/certificates/by-verification-code/"+details.VerificationCode, nil); resp.Status != http.StatusForbidden {
		t.Fatalf("without certificates:read: expected 403, got %d: %s", resp.Status, resp.Body)
	}
}

// seedSessionCookie tworzy sesję przeglądarki dla użytkownika testów.
func (e *testEnv) seedSessionCookie(t *testing.T) *http.Cookie {
	t.Helper()
	raw := fmt.Sprintf("it-session-%d-%d", nextSeed(), time.Now().UnixNano())
	_, err := e.pool.Exec(context.Background(), `
		INSERT INTO api_sessions (token, user_id, expires_at)
		SELECT $1, id, now() + interval '1 hour' FROM users WHERE email = 'integration@example.com'`,
		auth.HashToken(raw))
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return &http.Cookie{Name: "session_token", Value: raw}
}
