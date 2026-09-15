package integrationtest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Zlecenie, punkt 6: znajdź-lub-utwórz kursanta i firmę po identyfikatorze platformy.

type conflictError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		ID      int64  `json:"id"`
	} `json:"error"`
}

type studentData struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	BirthPlace string  `json:"birthPlace"`
	ExternalID *string `json:"externalId"`
}

type companyData struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Nip        string  `json:"nip"`
	Telephone  *string `json:"telephone"`
	ExternalID *string `json:"externalId"`
}

func decodeStudent(t *testing.T, resp apiResponse) studentData {
	t.Helper()
	var body struct {
		Data studentData `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func decodeCompany(t *testing.T, resp apiResponse) companyData {
	t.Helper()
	var body struct {
		Data companyData `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func newExternalID(prefix string) string {
	return fmt.Sprintf("%s-%d-7d0c2a4e-1f5b-4c8e-9a61-3b2d8f0e5c47", prefix, nextSeed())
}

func studentWritePayload(lastName string) map[string]any {
	return map[string]any{
		"firstName":  "Anna",
		"lastName":   lastName,
		"birthDate":  "1991-05-20",
		"birthPlace": "Kraków",
	}
}

func studentByExternalIDPath(externalID string) string {
	return "/students/by-external-id/" + url.PathEscape(externalID)
}

func companyByExternalIDPath(externalID string) string {
	return "/companies/by-external-id/" + url.PathEscape(externalID)
}

// validNIP generuje unikalny NIP z poprawną sumą kontrolną.
func validNIP(t *testing.T) string {
	t.Helper()
	weights := []int{6, 5, 7, 2, 3, 4, 5, 6, 7}
	for {
		base := fmt.Sprintf("9%08d", nextSeed()*7919%100000000)
		sum := 0
		for i, w := range weights {
			sum += int(base[i]-'0') * w
		}
		if check := sum % 11; check != 10 {
			return fmt.Sprintf("%s%d", base, check)
		}
	}
}

func companyWritePayload(t *testing.T, name string) map[string]any {
	return map[string]any{
		"name":      name,
		"street":    "Prosta 1",
		"city":      "Warszawa",
		"zipcode":   "00-001",
		"nip":       validNIP(t),
		"telephone": "500600700",
	}
}

func TestPutStudentByExternalIDCreatesThenUpdates(t *testing.T) {
	e := requireEnv(t)
	externalID := newExternalID("student")
	lastName := fmt.Sprintf("Zewnętrzna%d", nextSeed())
	payload := studentWritePayload(lastName)

	first := e.mustCall(t, http.MethodPut, studentByExternalIDPath(externalID), payload, nil)
	if first.Status != http.StatusCreated {
		t.Fatalf("first PUT: expected 201, got %d: %s", first.Status, first.Body)
	}
	created := decodeStudent(t, first)
	if created.ExternalID == nil || *created.ExternalID != externalID {
		t.Fatalf("expected externalId %q in response, got %v", externalID, created.ExternalID)
	}

	payload["birthPlace"] = "Gdańsk"
	second := e.mustCall(t, http.MethodPut, studentByExternalIDPath(externalID), payload, nil)
	if second.Status != http.StatusOK {
		t.Fatalf("second PUT: expected 200, got %d: %s", second.Status, second.Body)
	}
	updated := decodeStudent(t, second)
	if updated.ID != created.ID || updated.BirthPlace != "Gdańsk" {
		t.Fatalf("expected update of id %d, got %+v", created.ID, updated)
	}
	if n := e.countRows(t, `SELECT count(*) FROM students WHERE external_id = $1`, externalID); n != 1 {
		t.Fatalf("expected exactly 1 student with externalId, found %d", n)
	}
	if n := e.countRows(t, `SELECT count(*) FROM students WHERE lastname = $1`, lastName); n != 1 {
		t.Fatalf("expected exactly 1 student with lastname, found %d", n)
	}

	got := e.mustCall(t, http.MethodGet, fmt.Sprintf("/students/%d", created.ID), nil, nil)
	if s := decodeStudent(t, got); s.ExternalID == nil || *s.ExternalID != externalID {
		t.Fatalf("GET /students/{id}: expected externalId %q, got %v", externalID, s.ExternalID)
	}

	// PATCH to pełne nadpisanie danych, ale externalId nie należy do StudentWrite
	// i nie może zostać wyczyszczony przez edycję w aplikacji webowej.
	payload["birthPlace"] = "Poznań"
	patched := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/students/%d", created.ID), payload, nil)
	if patched.Status != http.StatusOK {
		t.Fatalf("PATCH: expected 200, got %d: %s", patched.Status, patched.Body)
	}
	if s := decodeStudent(t, patched); s.ExternalID == nil || *s.ExternalID != externalID {
		t.Fatalf("PATCH cleared externalId: %+v", s)
	}
}

func TestPutStudentByExternalIDNaturalKeyCollisionOnCreate(t *testing.T) {
	e := requireEnv(t)
	payload := studentWritePayload(fmt.Sprintf("Kolizja%d", nextSeed()))

	existing := e.mustCall(t, http.MethodPost, "/students", payload, nil)
	if existing.Status != http.StatusCreated {
		t.Fatalf("POST /students: expected 201, got %d: %s", existing.Status, existing.Body)
	}
	existingID := decodeStudent(t, existing).ID

	// Ten sam klucz naturalny, inna wielkość liter i spacje - to nadal ta sama osoba.
	payload["lastName"] = "  " + strings.ToUpper(payload["lastName"].(string)) + " "
	externalID := newExternalID("student")
	resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(externalID), payload, nil)
	assertNaturalKeyConflict(t, resp, "student with the same natural key already exists", existingID)

	if n := e.countRows(t, `SELECT count(*) FROM students WHERE external_id = $1`, externalID); n != 0 {
		t.Fatalf("expected no student created, found %d", n)
	}
	if n := e.countRows(t, `SELECT count(*) FROM students WHERE id = $1 AND external_id IS NULL`, existingID); n != 1 {
		t.Fatal("existing student must stay untouched")
	}
}

func TestPutStudentByExternalIDNaturalKeyCollisionOnUpdate(t *testing.T) {
	e := requireEnv(t)
	externalID := newExternalID("student")
	mine := studentWritePayload(fmt.Sprintf("Moja%d", nextSeed()))
	if resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(externalID), mine, nil); resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
	}

	other := studentWritePayload(fmt.Sprintf("Cudza%d", nextSeed()))
	otherResp := e.mustCall(t, http.MethodPost, "/students", other, nil)
	if otherResp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", otherResp.Status, otherResp.Body)
	}
	otherID := decodeStudent(t, otherResp).ID

	resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(externalID), other, nil)
	assertNaturalKeyConflict(t, resp, "student with the same natural key already exists", otherID)

	if n := e.countRows(t, `SELECT count(*) FROM students WHERE external_id = $1 AND lastname = $2`, externalID, mine["lastName"]); n != 1 {
		t.Fatal("student with externalId must keep its data after a rejected update")
	}
}

func TestPutStudentByExternalIDConcurrentCallsCreateOneRecord(t *testing.T) {
	e := requireEnv(t)
	externalID := newExternalID("student")
	payload := studentWritePayload(fmt.Sprintf("Równoległa%d", nextSeed()))

	responses := runConcurrently(t, 8, func(int) (apiResponse, error) {
		return e.call(http.MethodPut, studentByExternalIDPath(externalID), payload, nil)
	})
	created, updated := 0, 0
	for _, resp := range responses {
		switch resp.Status {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			updated++
		default:
			t.Fatalf("unexpected status %d: %s", resp.Status, resp.Body)
		}
	}
	if created != 1 || updated != 7 {
		t.Fatalf("expected 1x201 + 7x200, got %d created, %d updated", created, updated)
	}
	if n := e.countRows(t, `SELECT count(*) FROM students WHERE external_id = $1`, externalID); n != 1 {
		t.Fatalf("expected exactly 1 student, found %d", n)
	}
}

func TestPutStudentByExternalIDValidation(t *testing.T) {
	e := requireEnv(t)

	withExternalID := studentWritePayload(fmt.Sprintf("Walidacja%d", nextSeed()))
	withExternalID["externalId"] = "abc"
	if resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(newExternalID("s")), withExternalID, nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("externalId in body: expected 400, got %d: %s", resp.Status, resp.Body)
	}

	tooLong := strings.Repeat("x", 65)
	if resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(tooLong), studentWritePayload("Za długi"), nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("65-char externalId: expected 400, got %d: %s", resp.Status, resp.Body)
	}

	missingBirthPlace := studentWritePayload(fmt.Sprintf("BezMiejsca%d", nextSeed()))
	delete(missingBirthPlace, "birthPlace")
	if resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(newExternalID("s")), missingBirthPlace, nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("missing birthPlace: expected 400, got %d: %s", resp.Status, resp.Body)
	}

	exact64 := strings.Repeat("y", 63) + fmt.Sprint(nextSeed()%10)
	if resp := e.mustCall(t, http.MethodPut, studentByExternalIDPath(exact64), studentWritePayload(fmt.Sprintf("Sześćdziesiąt%d", nextSeed())), nil); resp.Status != http.StatusCreated {
		t.Fatalf("64-char externalId: expected 201, got %d: %s", resp.Status, resp.Body)
	}
}

func TestPutCompanyByExternalIDCreatesThenUpdates(t *testing.T) {
	e := requireEnv(t)
	externalID := newExternalID("company")
	payload := companyWritePayload(t, fmt.Sprintf("Firma zewnętrzna %d", nextSeed()))

	first := e.mustCall(t, http.MethodPut, companyByExternalIDPath(externalID), payload, nil)
	if first.Status != http.StatusCreated {
		t.Fatalf("first PUT: expected 201, got %d: %s", first.Status, first.Body)
	}
	created := decodeCompany(t, first)
	if created.ExternalID == nil || *created.ExternalID != externalID {
		t.Fatalf("expected externalId %q, got %v", externalID, created.ExternalID)
	}

	payload["name"] = payload["name"].(string) + " (zmieniona)"
	second := e.mustCall(t, http.MethodPut, companyByExternalIDPath(externalID), payload, nil)
	if second.Status != http.StatusOK {
		t.Fatalf("second PUT: expected 200, got %d: %s", second.Status, second.Body)
	}
	if updated := decodeCompany(t, second); updated.ID != created.ID || updated.Name != payload["name"] {
		t.Fatalf("expected update of id %d, got %+v", created.ID, updated)
	}
	if n := e.countRows(t, `SELECT count(*) FROM companies WHERE external_id = $1`, externalID); n != 1 {
		t.Fatalf("expected exactly 1 company, found %d", n)
	}
}

func TestPutCompanyByExternalIDNaturalKeyCollision(t *testing.T) {
	e := requireEnv(t)
	payload := companyWritePayload(t, fmt.Sprintf("Firma istniejąca %d", nextSeed()))
	existing := e.mustCall(t, http.MethodPost, "/companies", payload, nil)
	if existing.Status != http.StatusCreated {
		t.Fatalf("POST /companies: expected 201, got %d: %s", existing.Status, existing.Body)
	}
	existingID := decodeCompany(t, existing).ID

	externalID := newExternalID("company")
	resp := e.mustCall(t, http.MethodPut, companyByExternalIDPath(externalID), payload, nil)
	assertNaturalKeyConflict(t, resp, "company with the same natural key already exists", existingID)
	if n := e.countRows(t, `SELECT count(*) FROM companies WHERE external_id = $1`, externalID); n != 0 {
		t.Fatalf("expected no company created, found %d", n)
	}

	// Kolizja przy aktualizacji: własna firma dostaje NIP istniejącej.
	mine := companyWritePayload(t, fmt.Sprintf("Firma własna %d", nextSeed()))
	if r := e.mustCall(t, http.MethodPut, companyByExternalIDPath(externalID), mine, nil); r.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", r.Status, r.Body)
	}
	mine["nip"] = payload["nip"]
	resp = e.mustCall(t, http.MethodPut, companyByExternalIDPath(externalID), mine, nil)
	assertNaturalKeyConflict(t, resp, "company with the same natural key already exists", existingID)
}

func TestCompanyTelephoneIsOptional(t *testing.T) {
	e := requireEnv(t)

	omitted := companyWritePayload(t, fmt.Sprintf("Bez telefonu %d", nextSeed()))
	delete(omitted, "telephone")
	resp := e.mustCall(t, http.MethodPost, "/companies", omitted, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("POST without telephone: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	company := decodeCompany(t, resp)
	// Brak telefonu to pusty string - tak jak w istniejących danych i dotychczasowych odpowiedziach.
	if company.Telephone == nil || *company.Telephone != "" {
		t.Fatalf("expected empty telephone, got %v", company.Telephone)
	}

	null := companyWritePayload(t, fmt.Sprintf("Telefon null %d", nextSeed()))
	null["telephone"] = nil
	if resp := e.mustCall(t, http.MethodPost, "/companies", null, nil); resp.Status != http.StatusCreated {
		t.Fatalf("POST with telephone null: expected 201, got %d: %s", resp.Status, resp.Body)
	}

	omitted["name"] = omitted["name"].(string) + " (edycja)"
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/companies/%d", company.ID), omitted, nil); resp.Status != http.StatusOK {
		t.Fatalf("PATCH without telephone: expected 200, got %d: %s", resp.Status, resp.Body)
	}

	put := companyWritePayload(t, fmt.Sprintf("PUT bez telefonu %d", nextSeed()))
	delete(put, "telephone")
	if resp := e.mustCall(t, http.MethodPut, companyByExternalIDPath(newExternalID("company")), put, nil); resp.Status != http.StatusCreated {
		t.Fatalf("PUT without telephone: expected 201, got %d: %s", resp.Status, resp.Body)
	}
}

func TestExternalIDIsUniqueInDatabase(t *testing.T) {
	e := requireEnv(t)
	externalID := newExternalID("db")
	_, err := e.pool.Exec(t.Context(), `
		INSERT INTO students (firstname, lastname, birthdate, birthplace, external_id)
		VALUES ('A', $1, DATE '1990-01-01', 'X', $3), ('B', $2, DATE '1990-01-01', 'X', $3)`,
		fmt.Sprintf("Unikalny%d", nextSeed()), fmt.Sprintf("Unikalny%d", nextSeed()), externalID)
	if err == nil {
		t.Fatal("expected unique violation for duplicated students.external_id")
	}
	if n := e.countRows(t, `SELECT count(*) FROM students WHERE external_id IS NULL`); n < 2 {
		t.Fatalf("expected many students without externalId, found %d", n)
	}
}

func TestPutByExternalIDRequiresWriteScope(t *testing.T) {
	e := requireEnv(t)
	readOnly := e.seedScopedAPIKey(t, "students:read", "companies:read")

	resp := e.callWithKey(t, readOnly, http.MethodPut, studentByExternalIDPath(newExternalID("scope")), studentWritePayload("Zakres"))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("students: expected 403, got %d: %s", resp.Status, resp.Body)
	}
	resp = e.callWithKey(t, readOnly, http.MethodPut, companyByExternalIDPath(newExternalID("scope")), companyWritePayload(t, "Zakres"))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("companies: expected 403, got %d: %s", resp.Status, resp.Body)
	}
}

func assertNaturalKeyConflict(t *testing.T, resp apiResponse, message string, wantID int64) {
	t.Helper()
	if resp.Status != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.Status, resp.Body)
	}
	var body conflictError
	resp.decode(t, &body)
	if body.Error.Code != "conflict" || body.Error.Message != message {
		t.Fatalf("unexpected error body: %s", resp.Body)
	}
	if body.Error.ID != wantID {
		t.Fatalf("expected colliding id %d in error body, got %d: %s", wantID, body.Error.ID, resp.Body)
	}
}
