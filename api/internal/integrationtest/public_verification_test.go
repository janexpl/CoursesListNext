package integrationtest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Publiczna weryfikacja zaświadczenia: strona, na którą prowadzi kod QR z wydruku.
// Działa bez klucza API i bez sesji, więc odpowiedź musi nieść tylko to, co i tak
// jest na dokumencie.

type publicCertificate struct {
	VerificationCode  string  `json:"verificationCode"`
	CertificateNumber string  `json:"certificateNumber"`
	StudentName       string  `json:"studentName"`
	CourseName        string  `json:"courseName"`
	CourseDateStart   string  `json:"courseDateStart"`
	CourseDateEnd     *string `json:"courseDateEnd"`
	IssuedAt          string  `json:"issuedAt"`
	ValidUntil        *string `json:"validUntil"`
	Status            string  `json:"status"`
	Expired           bool    `json:"expired"`
	DuplicateIssued   bool    `json:"duplicateIssued"`
	DuplicateIssuedAt *string `json:"duplicateIssuedAt"`
	RevokedAt         *string `json:"revokedAt"`
}

func decodePublicCertificate(t *testing.T, resp apiResponse) publicCertificate {
	t.Helper()
	var body struct {
		Data publicCertificate `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

// seedCertificateForVerification wystawia zaświadczenie z danymi, które nie mogą
// trafić do publicznej odpowiedzi (PESEL, miejsce urodzenia, firma).
func (e *testEnv) seedCertificateForVerification(t *testing.T) (certificateID int64, code string, studentName string, pesel string, birthplace string, companyName string) {
	t.Helper()
	n := nextSeed()
	pesel = fmt.Sprintf("900101%05d", n%100000)
	birthplace = fmt.Sprintf("Tajemnicze%d", n)
	companyName = fmt.Sprintf("Firma Poufna %d", n)

	var companyID int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO companies (name, street, city, zipcode, nip, telephoneno)
		VALUES ($1, 'Prosta 1', 'Warszawa', '00-001', $2, '') RETURNING id`,
		companyName, validNIP(t)).Scan(&companyID); err != nil {
		t.Fatal(err)
	}
	lastName := fmt.Sprintf("Weryfikowany%d", n)
	var studentID int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO students (firstname, secondname, lastname, birthdate, birthplace, pesel, company_id)
		VALUES ('Jan', 'Adam', $1, DATE '1990-01-10', $2, $3, $4) RETURNING id`,
		lastName, birthplace, pesel, companyID).Scan(&studentID); err != nil {
		t.Fatal(err)
	}

	course := e.seedCourse(t)
	created := e.issueCertificate(t, course.ID, studentID, "2026-03-15")
	details := e.getCertificate(t, created.ID)
	return created.ID, details.VerificationCode, "Jan Adam " + lastName, pesel, birthplace, companyName
}

func TestPublicVerificationNeedsNoAuthentication(t *testing.T) {
	e := requireEnv(t)
	certificateID, code, studentName, pesel, birthplace, companyName := e.seedCertificateForVerification(t)

	resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+code)
	if resp.Status != http.StatusOK {
		t.Fatalf("expected 200 without credentials, got %d: %s", resp.Status, resp.Body)
	}
	certificate := decodePublicCertificate(t, resp)

	if certificate.VerificationCode != code {
		t.Fatalf("expected code %s, got %s", code, certificate.VerificationCode)
	}
	if certificate.StudentName != studentName {
		t.Fatalf("student name = %q, expected %q", certificate.StudentName, studentName)
	}
	if certificate.Status != "valid" || certificate.Expired || certificate.DuplicateIssued {
		t.Fatalf("freshly issued certificate must be valid: %+v", certificate)
	}
	if certificate.IssuedAt != "2026-03-15" || certificate.CourseDateStart != "2026-03-10" {
		t.Fatalf("unexpected dates: %+v", certificate)
	}
	stored := e.getCertificate(t, certificateID)
	if certificate.CertificateNumber == "" || !strings.HasPrefix(certificate.CertificateNumber, fmt.Sprint(stored.RegistryNumber)+"/") {
		t.Fatalf("unexpected certificate number %q", certificate.CertificateNumber)
	}

	// Dane, które zna tylko rejestr, nie mogą wyjść na zewnątrz.
	for _, secret := range []string{pesel, birthplace, companyName, "1990-01-10"} {
		if strings.Contains(string(resp.Body), secret) {
			t.Fatalf("public response leaks %q: %s", secret, resp.Body)
		}
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control: no-store, got %q", resp.Header.Get("Cache-Control"))
	}
}

func TestPublicVerificationNormalizesCodeAndRejectsGarbage(t *testing.T) {
	e := requireEnv(t)
	_, code, _, _, _, _ := e.seedCertificateForVerification(t)

	if resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+strings.ToLower(code)); resp.Status != http.StatusOK {
		t.Fatalf("lowercase code: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	for _, bad := range []string{"abc", "ZZZZZZZZZZZ", "K7QM4XPA9TZ0", "K7QM4XPA9TZO"} {
		resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+bad)
		if resp.Status != http.StatusBadRequest || resp.errorMessage(t) != "invalid verification code" {
			t.Fatalf("code %q: expected 400 invalid verification code, got %d: %s", bad, resp.Status, resp.Body)
		}
	}
	if resp := e.callPublic(t, http.MethodGet, "/public/certificates/ZZZZZZZZZZZZ"); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown code: expected 404, got %d: %s", resp.Status, resp.Body)
	}
}

func TestPublicVerificationReportsRevocationAndDuplicate(t *testing.T) {
	e := requireEnv(t)
	revokedID, revokedCode, _, _, _, _ := e.seedCertificateForVerification(t)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revokedID), map[string]any{"reason": "Błędne dane"}, nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", resp.Status, resp.Body)
	}

	revoked := decodePublicCertificate(t, e.callPublic(t, http.MethodGet, "/public/certificates/"+revokedCode))
	if revoked.Status != "revoked" || revoked.RevokedAt == nil {
		t.Fatalf("expected revoked status with a date: %+v", revoked)
	}
	// Powód unieważnienia bywa wewnętrzną notatką - nie publikujemy go.
	if strings.Contains(string(e.callPublic(t, http.MethodGet, "/public/certificates/"+revokedCode).Body), "Błędne dane") {
		t.Fatal("public response must not carry the revocation reason")
	}

	duplicatedID, duplicatedCode, _, _, _, _ := e.seedCertificateForVerification(t)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", duplicatedID), map[string]any{"reason": "Utrata"}, nil); resp.Status != http.StatusOK {
		t.Fatalf("duplicate: %d %s", resp.Status, resp.Body)
	}
	duplicated := decodePublicCertificate(t, e.callPublic(t, http.MethodGet, "/public/certificates/"+duplicatedCode))
	if !duplicated.DuplicateIssued || duplicated.DuplicateIssuedAt == nil {
		t.Fatalf("expected the duplicate to be reported: %+v", duplicated)
	}
	// Wtórnik nie odbiera dokumentowi ważności.
	if duplicated.Status != "valid" {
		t.Fatalf("duplicate must not change the status: %+v", duplicated)
	}
}

func TestPublicVerificationHidesDeletedCertificates(t *testing.T) {
	e := requireEnv(t)
	certificateID, code, _, _, _, _ := e.seedCertificateForVerification(t)

	if resp := e.mustCall(t, http.MethodDelete, fmt.Sprintf("/certificates/%d", certificateID), nil, nil); resp.Status != http.StatusOK {
		t.Fatalf("delete: %d %s", resp.Status, resp.Body)
	}
	if resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+code); resp.Status != http.StatusNotFound {
		t.Fatalf("deleted certificate: expected 404, got %d: %s", resp.Status, resp.Body)
	}
}

func TestPublicVerificationMarksExpiredCertificates(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	// Kurs jest ważny 5 lat, więc szkolenie sprzed sześciu lat jest już po terminie.
	end := time.Now().AddDate(-6, 0, 0)
	payload := certificatePayload(student, course.ID)
	payload["courseDateStart"] = end.AddDate(0, 0, -2).Format("2006-01-02")
	payload["courseDateEnd"] = end.Format("2006-01-02")
	payload["certificateDate"] = end.Format("2006-01-02")
	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("issue: %d %s", resp.Status, resp.Body)
	}
	code := e.getCertificate(t, decodeCreated(t, resp).ID).VerificationCode

	certificate := decodePublicCertificate(t, e.callPublic(t, http.MethodGet, "/public/certificates/"+code))
	if !certificate.Expired || certificate.Status != "valid" {
		t.Fatalf("expected an expired but not revoked certificate: %+v", certificate)
	}
	if certificate.ValidUntil == nil {
		t.Fatalf("expected validUntil to be set: %+v", certificate)
	}
}

// Kod ma 60 bitów losowości, ale pojedynczy dokument nie musi znosić młócenia.
func TestPublicVerificationLimitsRequestsPerCode(t *testing.T) {
	e := requireEnv(t)
	_, code, _, _, _, _ := e.seedCertificateForVerification(t)

	limited := false
	for i := range 40 {
		resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+code)
		if resp.Status == http.StatusTooManyRequests {
			limited = true
			break
		}
		if resp.Status != http.StatusOK {
			t.Fatalf("request %d: unexpected status %d: %s", i, resp.Status, resp.Body)
		}
	}
	if !limited {
		t.Fatal("expected the rate limit to kick in for a single code")
	}

	// Limit jednego kodu nie może blokować weryfikacji innych dokumentów.
	_, otherCode, _, _, _, _ := e.seedCertificateForVerification(t)
	if resp := e.callPublic(t, http.MethodGet, "/public/certificates/"+otherCode); resp.Status != http.StatusOK {
		t.Fatalf("another code: expected 200, got %d: %s", resp.Status, resp.Body)
	}
}

// Podgląd i wydruk w przeglądarce muszą pokazywać ten sam kod QR co PDF z serwera,
// więc szczegóły zaświadczenia niosą gotowy obrazek i adres weryfikacji.
func TestCertificateDetailsCarryVerificationQR(t *testing.T) {
	e := requireEnv(t)
	certificateID, code, _, _, _, _ := e.seedCertificateForVerification(t)

	resp := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", certificateID), nil, nil)
	var body struct {
		Data struct {
			VerificationURL string `json:"verificationUrl"`
			VerificationQr  string `json:"verificationQr"`
		} `json:"data"`
	}
	resp.decode(t, &body)

	wantURL := strings.ReplaceAll(verificationURLTemplate, "{code}", code)
	if body.Data.VerificationURL != wantURL {
		t.Fatalf("verificationUrl = %q, expected %q", body.Data.VerificationURL, wantURL)
	}
	if !strings.HasPrefix(body.Data.VerificationQr, "data:image/png;base64,") {
		t.Fatalf("expected a PNG data URI, got %.40q", body.Data.VerificationQr)
	}
}
