package integrationtest

import (
	"fmt"
	"net/http"
	"net/url"
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
	SupersedesID      *int64  `json:"supersedesId"`
	SupersededByID    *int64  `json:"supersededById"`
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

func TestDuplicateCertificateGetsNewNumberAndSupersedesOriginal(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	original := e.issueCertificate(t, course.ID, student, "2026-03-15")
	e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-16")
	originalDetails := decodeCertificateState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", original.ID), nil, nil))

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", original.ID), map[string]any{"reason": "Utrata oryginału"}, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("duplicate: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	duplicate := decodeCertificateState(t, resp)
	if duplicate.ID == original.ID || duplicate.SupersedesID == nil || *duplicate.SupersedesID != original.ID {
		t.Fatalf("duplicate must be a new document with supersedesId=%d: %s", original.ID, resp.Body)
	}
	if duplicate.RegistryYear != 2026 || duplicate.RegistryNumber != 3 {
		t.Fatalf("duplicate must get the next number 2026/3, got %d/%d", duplicate.RegistryYear, duplicate.RegistryNumber)
	}
	if duplicate.DuplicateReason == nil || *duplicate.DuplicateReason != "Utrata oryginału" {
		t.Fatalf("duplicate must carry the reason: %s", resp.Body)
	}
	if duplicate.StudentID != originalDetails.StudentID || duplicate.CourseID != originalDetails.CourseID ||
		duplicate.StudentFirstname != originalDetails.StudentFirstname || duplicate.StudentLastname != originalDetails.StudentLastname ||
		duplicate.StudentBirthdate != originalDetails.StudentBirthdate || duplicate.CourseName != originalDetails.CourseName ||
		duplicate.CourseDateStart != originalDetails.CourseDateStart || deref(duplicate.CourseDateEnd) != deref(originalDetails.CourseDateEnd) {
		t.Fatalf("duplicate must copy the original data:\noriginal:  %+v\nduplicate: %+v", originalDetails, duplicate)
	}
	if today := time.Now().Format("2006-01-02"); duplicate.Date != today {
		t.Fatalf("duplicate must be issued today (%s), got %s", today, duplicate.Date)
	}
	if duplicate.VerificationCode == "" || duplicate.VerificationCode == originalDetails.VerificationCode {
		t.Fatalf("duplicate needs its own verification code, got %q (original %q)", duplicate.VerificationCode, originalDetails.VerificationCode)
	}

	after := decodeCertificateState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", original.ID), nil, nil))
	if after.SupersededByID == nil || *after.SupersededByID != duplicate.ID || after.RevokedAt != nil {
		t.Fatalf("original must point to its duplicate and stay not revoked: %+v", after)
	}

	again := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", original.ID), map[string]any{"reason": "Znowu"}, nil)
	if again.Status != http.StatusConflict || again.errorMessage(t) != "certificate already superseded" {
		t.Fatalf("second duplicate of the same original: expected 409, got %d: %s", again.Status, again.Body)
	}

	// Po duplikacie z dzisiejszą datą kolejny numer nie może mieć daty wcześniejszej (chronologia rejestru).
	revokedCert := e.issueCertificate(t, course.ID, e.seedStudent(t), time.Now().Format("2006-01-02"))
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revokedCert.ID), map[string]any{"reason": "x"}, nil); r.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", r.Status, r.Body)
	}
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", revokedCert.ID), map[string]any{"reason": "x"}, nil); r.Status != http.StatusConflict || r.errorMessage(t) != "certificate is revoked" {
		t.Fatalf("duplicate of revoked: expected 409 certificate is revoked, got %d: %s", r.Status, r.Body)
	}
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", duplicate.ID), map[string]any{}, nil); r.Status != http.StatusBadRequest {
		t.Fatalf("duplicate without reason: expected 400, got %d: %s", r.Status, r.Body)
	}
	readOnly := e.seedScopedAPIKey(t, "certificates:read")
	if r := e.callWithKey(t, readOnly, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", duplicate.ID), map[string]any{"reason": "x"}); r.Status != http.StatusForbidden {
		t.Fatalf("duplicate without certificates:write: expected 403, got %d: %s", r.Status, r.Body)
	}
}

func TestConcurrentDuplicatesOfOneCertificateCreateOne(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	original := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")

	responses := runConcurrently(t, 6, func(int) (apiResponse, error) {
		return e.call(http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", original.ID), map[string]any{"reason": "Utrata"}, nil)
	})
	created := 0
	for _, resp := range responses {
		switch resp.Status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d: %s", resp.Status, resp.Body)
		}
	}
	if created != 1 {
		t.Fatalf("expected exactly one duplicate, got %d", created)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE supersedes_id = $1`, original.ID); n != 1 {
		t.Fatalf("expected one superseding certificate in database, found %d", n)
	}
}

func TestExpiryNotificationsSkipRevokedAndSupersededCertificates(t *testing.T) {
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
	superseded := issue()
	if r := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revoked), map[string]any{"reason": "x"}, nil); r.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", r.Status, r.Body)
	}
	dup := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", superseded), map[string]any{"reason": "x"}, nil)
	if dup.Status != http.StatusCreated {
		t.Fatalf("duplicate: %d %s", dup.Status, dup.Body)
	}
	duplicateID := decodeCertificateState(t, dup).ID

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
	if !got[active] || !got[duplicateID] {
		t.Fatalf("active certificate %d and duplicate %d must be notified, got %v", active, duplicateID, got)
	}
	if got[revoked] || got[superseded] {
		t.Fatalf("revoked %d and superseded %d must not be notified, got %v", revoked, superseded, got)
	}
}

func deref(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
