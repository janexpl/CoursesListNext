package integrationtest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Zlecenie, punkt 5: unieważnienie i duplikat zaświadczenia.

type certificateState struct {
	ID                int64   `json:"id"`
	Date              string  `json:"date"`
	StudentID         int64   `json:"studentId"`
	CourseID          int64   `json:"courseId"`
	StudentFirstname  string  `json:"studentFirstname"`
	StudentLastname   string  `json:"studentLastname"`
	StudentBirthdate  string  `json:"studentBirthdate"`
	CourseName        string  `json:"courseName"`
	CourseDateStart   string  `json:"courseDateStart"`
	CourseDateEnd     *string `json:"courseDateEnd"`
	RegistryYear      int64   `json:"registryYear"`
	RegistryNumber    int64   `json:"registryNumber"`
	VerificationCode  string  `json:"verificationCode"`
	RevokedAt         *string `json:"revokedAt"`
	RevokeReason      *string `json:"revokeReason"`
	DuplicateIssuedAt *string `json:"duplicateIssuedAt"`
	DuplicateReason   *string `json:"duplicateReason"`
	PrintVariantCount int
}

func decodeCertificateState(t *testing.T, resp apiResponse) certificateState {
	t.Helper()
	var body struct {
		Data certificateState `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func (e *testEnv) issueCertificate(t *testing.T, courseID, studentID int64, certificateDate string) createCertificateData {
	t.Helper()
	payload := certificatePayload(studentID, courseID)
	payload["certificateDate"] = certificateDate
	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("issue certificate: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	return decodeCreated(t, resp)
}

func TestRevokeCertificateKeepsRegistryNumberOccupied(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	original := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")

	revoke := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", original.ID), map[string]any{"reason": "Błędne dane kursanta"}, nil)
	if revoke.Status != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", revoke.Status, revoke.Body)
	}
	revoked := decodeCertificateState(t, revoke)
	if revoked.RevokedAt == nil || revoked.RevokeReason == nil || *revoked.RevokeReason != "Błędne dane kursanta" {
		t.Fatalf("revoke response must carry revokedAt and revokeReason: %s", revoke.Body)
	}

	// Nadal widoczne w GET i na liście, ze stanem.
	get := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", original.ID), nil, nil)
	if get.Status != http.StatusOK || decodeCertificateState(t, get).RevokedAt == nil {
		t.Fatalf("GET revoked certificate: expected 200 with revokedAt, got %d: %s", get.Status, get.Body)
	}
	list := e.mustCall(t, http.MethodGet, "/certificates?limit=100&search="+url.QueryEscape(fmt.Sprintf("%d/%s/2026", original.RegistryNumber, course.Symbol)), nil, nil)
	var listBody struct {
		Data []certificateState `json:"data"`
	}
	list.decode(t, &listBody)
	found := false
	for _, item := range listBody.Data {
		if item.ID == original.ID {
			found = item.RevokedAt != nil
		}
	}
	if !found {
		t.Fatalf("revoked certificate must be listed with revokedAt: %s", list.Body)
	}
	courseList := e.mustCall(t, http.MethodGet, fmt.Sprintf("/courses/%d/certificates", course.ID), nil, nil)
	var courseListBody struct {
		Data []certificateState `json:"data"`
	}
	courseList.decode(t, &courseListBody)
	if len(courseListBody.Data) != 1 || courseListBody.Data[0].RevokedAt == nil {
		t.Fatalf("course certificate list must show the revoked state: %s", courseList.Body)
	}

	again := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", original.ID), map[string]any{"reason": "Drugi raz"}, nil)
	if again.Status != http.StatusConflict || again.errorMessage(t) != "certificate already revoked" {
		t.Fatalf("re-revoke: expected 409 certificate already revoked, got %d: %s", again.Status, again.Body)
	}

	assertNumberOccupied := func(stage string) {
		t.Helper()
		explicit := certificatePayload(e.seedStudent(t), course.ID)
		// Ta sama data co oryginał, żeby jedyną przeszkodą był zajęty numer (chronologia sprawdzana jest wcześniej).
		explicit["certificateDate"] = "2026-03-15"
		explicit["registryYear"] = original.RegistryYear
		explicit["registryNumber"] = original.RegistryNumber
		if resp := e.mustCall(t, http.MethodPost, "/certificates", explicit, nil); resp.Status != http.StatusConflict {
			t.Fatalf("%s: explicit reuse of revoked number must be 409, got %d: %s", stage, resp.Status, resp.Body)
		}
	}
	assertNumberOccupied("after revoke")
	next := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-20")
	if next.RegistryNumber != original.RegistryNumber+1 {
		t.Fatalf("server-assigned number must skip the revoked one: got %d after %d", next.RegistryNumber, original.RegistryNumber)
	}

	// DELETE zostaje bez zmian (usuwa dokument), ale numer unieważnionego nie wraca do puli.
	if resp := e.mustCall(t, http.MethodDelete, fmt.Sprintf("/certificates/%d", original.ID), nil, nil); resp.Status != http.StatusOK {
		t.Fatalf("DELETE revoked certificate: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	assertNumberOccupied("after delete")
}

func TestRevokeValidationAndEffects(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	cert := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	path := fmt.Sprintf("/certificates/%d/revoke", cert.ID)

	for _, body := range []any{map[string]any{}, map[string]any{"reason": "   "}, map[string]any{"reason": "x", "extra": true}, "{"} {
		if resp := e.mustCall(t, http.MethodPost, path, body, nil); resp.Status != http.StatusBadRequest {
			t.Fatalf("revoke body %v: expected 400, got %d: %s", body, resp.Status, resp.Body)
		}
	}
	if resp := e.mustCall(t, http.MethodPost, "/certificates/999999999/revoke", map[string]any{"reason": "x"}, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown certificate: expected 404, got %d: %s", resp.Status, resp.Body)
	}
	readOnly := e.seedScopedAPIKey(t, "certificates:read")
	if resp := e.callWithKey(t, readOnly, http.MethodPost, path, map[string]any{"reason": "x"}); resp.Status != http.StatusForbidden {
		t.Fatalf("revoke without certificates:write: expected 403, got %d: %s", resp.Status, resp.Body)
	}

	if resp := e.mustCall(t, http.MethodPost, path, map[string]any{"reason": "  Wystawione omyłkowo "}, nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", resp.Status, resp.Body)
	} else if reason := decodeCertificateState(t, resp).RevokeReason; reason == nil || *reason != "Wystawione omyłkowo" {
		t.Fatalf("reason must be trimmed, got %v", reason)
	}

	if resp := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d/pdf", cert.ID), nil, nil); resp.Status != http.StatusConflict || resp.errorMessage(t) != "certificate is revoked" {
		t.Fatalf("PDF of revoked certificate: expected 409 certificate is revoked, got %d: %s", resp.Status, resp.Body)
	}
	patch := map[string]any{"studentId": e.seedStudent(t), "certificateDate": "2026-03-16", "courseDateStart": "2026-03-10"}
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/certificates/%d", cert.ID), patch, nil); resp.Status != http.StatusConflict || resp.errorMessage(t) != "certificate is revoked" {
		t.Fatalf("PATCH of revoked certificate: expected 409 certificate is revoked, got %d: %s", resp.Status, resp.Body)
	}

	deleted := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-16")
	if resp := e.mustCall(t, http.MethodDelete, fmt.Sprintf("/certificates/%d", deleted.ID), nil, nil); resp.Status != http.StatusOK {
		t.Fatalf("DELETE: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", deleted.ID), map[string]any{"reason": "x"}, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("revoke of deleted certificate: expected 404, got %d: %s", resp.Status, resp.Body)
	}
}

// Duplikat (wtórnik) to ten sam dokument: ten sam numer rejestru, ta sama treść,
// dołożona adnotacja z datą wystawienia duplikatu.
func TestDuplicateMarksTheSameCertificate(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	original := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	before := decodeCertificateState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", original.ID), nil, nil))
	certificatesInCourse := func() int {
		return e.countRows(t, `
			SELECT count(*) FROM certificates c JOIN registries r ON r.id = c.registry_id
			WHERE r.course_id = $1`, course.ID)
	}
	if n := certificatesInCourse(); n != 1 {
		t.Fatalf("expected 1 certificate before the duplicate, found %d", n)
	}

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", original.ID), map[string]any{"reason": "Utrata oryginału"}, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("duplicate: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	duplicate := decodeCertificateState(t, resp)

	if duplicate.ID != original.ID || duplicate.RegistryNumber != original.RegistryNumber || duplicate.RegistryYear != original.RegistryYear {
		t.Fatalf("duplicate must stay the same document %d (%d/%d), got %+v", original.ID, original.RegistryYear, original.RegistryNumber, duplicate)
	}
	if duplicate.VerificationCode != before.VerificationCode || duplicate.Date != before.Date {
		t.Fatalf("duplicate must keep the verification code and the issue date: %+v", duplicate)
	}
	if duplicate.DuplicateIssuedAt == nil || *duplicate.DuplicateIssuedAt == "" {
		t.Fatalf("duplicate must carry duplicateIssuedAt: %s", resp.Body)
	}
	if duplicate.DuplicateReason == nil || *duplicate.DuplicateReason != "Utrata oryginału" {
		t.Fatalf("duplicate must carry the reason: %s", resp.Body)
	}
	if today := time.Now().Format("2006-01-02"); !strings.HasPrefix(*duplicate.DuplicateIssuedAt, today) {
		t.Fatalf("duplicateIssuedAt must be today (%s), got %s", today, *duplicate.DuplicateIssuedAt)
	}
	if n := certificatesInCourse(); n != 1 {
		t.Fatalf("duplicate must not create a second certificate, found %d", n)
	}

	stored := decodeCertificateState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", original.ID), nil, nil))
	if stored.DuplicateIssuedAt == nil {
		t.Fatalf("GET must report the duplicate: %+v", stored)
	}

	// Duplikat nie zużywa kolejnego numeru w rejestrze.
	next := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-16")
	if next.RegistryNumber != original.RegistryNumber+1 {
		t.Fatalf("expected the next number %d, got %d", original.RegistryNumber+1, next.RegistryNumber)
	}
}

// Kursant może zgubić dokument ponownie - liczy się data ostatniego wystawienia.
func TestDuplicateCanBeIssuedAgain(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	cert := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	path := fmt.Sprintf("/certificates/%d/duplicate", cert.ID)

	first := decodeCertificateState(t, e.mustCall(t, http.MethodPost, path, map[string]any{"reason": "Pierwsza utrata"}, nil))
	time.Sleep(10 * time.Millisecond)
	second := e.mustCall(t, http.MethodPost, path, map[string]any{"reason": "Druga utrata"}, nil)
	if second.Status != http.StatusOK {
		t.Fatalf("second duplicate: expected 200, got %d: %s", second.Status, second.Body)
	}
	again := decodeCertificateState(t, second)
	if again.DuplicateReason == nil || *again.DuplicateReason != "Druga utrata" {
		t.Fatalf("second duplicate must overwrite the reason: %s", second.Body)
	}
	if first.DuplicateIssuedAt == nil || again.DuplicateIssuedAt == nil || *again.DuplicateIssuedAt < *first.DuplicateIssuedAt {
		t.Fatalf("second duplicate must not move the date backwards: %v -> %v", first.DuplicateIssuedAt, again.DuplicateIssuedAt)
	}
	if n := e.countRows(t, `SELECT count(*) FROM audit_log WHERE entity_type = 'certificate' AND entity_id = $1 AND metadata->>'operation' = 'duplicate'`, cert.ID); n != 2 {
		t.Fatalf("expected both duplicates in the audit log, found %d", n)
	}
}

func TestDuplicateValidation(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	cert := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	path := fmt.Sprintf("/certificates/%d/duplicate", cert.ID)

	for _, body := range []any{map[string]any{}, map[string]any{"reason": "  "}, map[string]any{"reason": "x", "extra": 1}} {
		if resp := e.mustCall(t, http.MethodPost, path, body, nil); resp.Status != http.StatusBadRequest {
			t.Fatalf("duplicate body %v: expected 400, got %d: %s", body, resp.Status, resp.Body)
		}
	}
	if resp := e.mustCall(t, http.MethodPost, "/certificates/999999999/duplicate", map[string]any{"reason": "x"}, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown certificate: expected 404, got %d: %s", resp.Status, resp.Body)
	}
	readOnly := e.seedScopedAPIKey(t, "certificates:read")
	if resp := e.callWithKey(t, readOnly, http.MethodPost, path, map[string]any{"reason": "x"}); resp.Status != http.StatusForbidden {
		t.Fatalf("duplicate without certificates:write: expected 403, got %d: %s", resp.Status, resp.Body)
	}

	revoked := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-16")
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revoked.ID), map[string]any{"reason": "x"}, nil); r.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", r.Status, r.Body)
	}
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", revoked.ID), map[string]any{"reason": "x"}, nil); r.Status != http.StatusConflict || r.errorMessage(t) != "certificate is revoked" {
		t.Fatalf("duplicate of revoked: expected 409 certificate is revoked, got %d: %s", r.Status, r.Body)
	}
}

func TestExpiryNotificationsSkipRevokedCertificates(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	var companyID int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO companies (name, street, city, zipcode, nip, telephoneno, email, expiry_notifications_enabled)
		VALUES ('Firma powiadomienia', 'Ulica', 'Miasto', '00-001', $1, '', 'kadry@example.com', true)
		RETURNING id`, validNIP(t)).Scan(&companyID); err != nil {
		t.Fatal(err)
	}

	// Kurs ważny 5 lat, zakończony tak, żeby ważność kończyła się za 10 dni.
	end := time.Now().AddDate(0, 0, 10-5*365)
	issue := func() int64 {
		var studentID int64
		if err := e.pool.QueryRow(t.Context(), `
			INSERT INTO students (firstname, lastname, birthdate, birthplace, company_id)
			VALUES ('Ewa', $1, DATE '1985-02-02', 'Łódź', $2) RETURNING id`,
			fmt.Sprintf("Wygasa%d", nextSeed()), companyID).Scan(&studentID); err != nil {
			t.Fatal(err)
		}
		payload := certificatePayload(studentID, course.ID)
		payload["courseDateStart"] = end.AddDate(0, 0, -1).Format("2006-01-02")
		payload["courseDateEnd"] = end.Format("2006-01-02")
		payload["certificateDate"] = end.Format("2006-01-02")
		resp := e.mustCall(t, http.MethodPost, "/certificates", payload, nil)
		if resp.Status != http.StatusCreated {
			t.Fatalf("issue: %d %s", resp.Status, resp.Body)
		}
		return decodeCreated(t, resp).ID
	}
	active := issue()
	revoked := issue()
	duplicated := issue()
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revoked), map[string]any{"reason": "x"}, nil); r.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", r.Status, r.Body)
	}
	// Wtórnik nie zmienia ważności dokumentu - przypomnienie ma nadal przyjść.
	dup := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", duplicated), map[string]any{"reason": "x"}, nil)
	if dup.Status != http.StatusOK {
		t.Fatalf("duplicate: %d %s", dup.Status, dup.Body)
	}

	query := url.Values{
		"dateFrom": {time.Now().Format("2006-01-02")},
		"dateTo":   {time.Now().AddDate(0, 0, 30).Format("2006-01-02")},
		"limit":    {"500"},
	}
	req, err := http.NewRequest(http.MethodGet, e.server.URL+"/api/v1/internal/notifications/expiring-certificates?"+query.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+notificationsToken)
	httpResp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	var body struct {
		Data []struct {
			CertificateID int64 `json:"certificateId"`
		} `json:"data"`
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("notifications: expected 200, got %d", httpResp.StatusCode)
	}
	if err := decodeJSONBody(httpResp, &body); err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, item := range body.Data {
		got[item.CertificateID] = true
	}
	if !got[active] || !got[duplicated] {
		t.Fatalf("active certificate %d and the one with a duplicate %d must be notified, got %v", active, duplicated, got)
	}
	if got[revoked] {
		t.Fatalf("revoked certificate %d must not be notified, got %v", revoked, got)
	}
}

func deref(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
