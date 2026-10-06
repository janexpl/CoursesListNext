package integrationtest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// Przedłużanie zaświadczeń: nowy dokument dla tego samego kursanta i kursu, a stary
// dostaje znacznik "przedłużone" i wypada z zestawień wygasających.
//
// Znacznik NIE jest unieważnieniem - dokument zostaje ważny do swojej daty, dalej się
// drukuje i dalej weryfikuje publicznie jako ważny. Te testy pilnują obu stron tej
// granicy: że wypada z przypomnień i że nie traci nic poza nimi.

type renewedCertificateState struct {
	ID                     int64   `json:"id"`
	StudentID              int64   `json:"studentId"`
	CourseID               int64   `json:"courseId"`
	RegistryYear           int64   `json:"registryYear"`
	RegistryNumber         int64   `json:"registryNumber"`
	VerificationCode       string  `json:"verificationCode"`
	RevokedAt              *string `json:"revokedAt"`
	RenewedAt              *string `json:"renewedAt"`
	RenewedByCertificateID *int64  `json:"renewedByCertificateId"`
	RenewalOfCertificateID *int64  `json:"renewalOfCertificateId"`
}

func decodeRenewedState(t *testing.T, resp apiResponse) renewedCertificateState {
	t.Helper()
	var body struct {
		Data renewedCertificateState `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

// seedCompanyWithNotifications tworzy firmę, której kursanci trafiają do przypomnień
// o wygasaniu (endpoint /internal/notifications/expiring-certificates wymaga adresu).
func (e *testEnv) seedCompanyWithNotifications(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO companies (name, street, city, zipcode, nip, telephoneno, email, expiry_notifications_enabled)
		VALUES ($1, 'Ulica', 'Miasto', '00-001', $2, '', 'kadry@example.com', true)
		RETURNING id`, fmt.Sprintf("Firma przedłużenia %d", nextSeed()), validNIP(t)).Scan(&id); err != nil {
		t.Fatalf("seed company: %v", err)
	}
	return id
}

// seedExpiringCertificate wystawia zaświadczenie, którego ważność kończy się za
// `inDays` dni. Kurs z seedCourse jest ważny 5 lat, więc data zakończenia idzie
// odpowiednio w przeszłość.
func (e *testEnv) seedExpiringCertificate(t *testing.T, courseID, companyID int64, inDays int) createCertificateData {
	t.Helper()
	var studentID int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO students (firstname, lastname, birthdate, birthplace, company_id)
		VALUES ('Ewa', $1, DATE '1985-02-02', 'Łódź', $2) RETURNING id`,
		fmt.Sprintf("Przedluzana%d", nextSeed()), companyID).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	end := time.Now().AddDate(0, 0, inDays-5*365)
	payload := certificatePayload(studentID, courseID)
	payload["courseDateStart"] = end.AddDate(0, 0, -1).Format(time.DateOnly)
	payload["courseDateEnd"] = end.Format(time.DateOnly)
	payload["certificateDate"] = end.Format(time.DateOnly)
	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("seed expiring certificate: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	return decodeCreated(t, resp)
}

// notificationCandidates zwraca identyfikatory z endpointu przypomnień, z którego
// korzysta usługa notifications.
func (e *testEnv) notificationCandidates(t *testing.T, withinDays int) map[int64]bool {
	t.Helper()
	query := url.Values{
		"dateFrom": {time.Now().Format(time.DateOnly)},
		"dateTo":   {time.Now().AddDate(0, 0, withinDays).Format(time.DateOnly)},
		"limit":    {"500"},
	}
	req, err := http.NewRequest(http.MethodGet, e.server.URL+"/api/v1/internal/notifications/expiring-certificates?"+query.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+notificationsToken)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notification candidates: expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			CertificateID int64 `json:"certificateId"`
		} `json:"data"`
	}
	if err := decodeJSONBody(resp, &body); err != nil {
		t.Fatal(err)
	}
	out := map[int64]bool{}
	for _, item := range body.Data {
		out[item.CertificateID] = true
	}
	return out
}

// dashboardExpiring zwraca licznik i listę z kafelka "Wygasające". Oba pochodzą
// z OSOBNYCH zapytań SQL, które już raz się rozjechały - dlatego test czyta oba.
func (e *testEnv) dashboardExpiring(t *testing.T) (count int, listed map[int64]bool) {
	t.Helper()
	resp := e.mustCall(t, http.MethodGet, "/dashboard", nil, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	var body struct {
		Data struct {
			Expiring struct {
				In30Days int `json:"in30Days"`
			} `json:"expiring"`
			ExpiringCertificates []struct {
				CertificateID int64 `json:"certificateId"`
			} `json:"expiringCertificates"`
		} `json:"data"`
	}
	resp.decode(t, &body)
	listed = map[int64]bool{}
	for _, item := range body.Data.ExpiringCertificates {
		listed[item.CertificateID] = true
	}
	return body.Data.Expiring.In30Days, listed
}

func renewPayload() map[string]any {
	today := time.Now()
	return map[string]any{
		"certificateDate": today.Format(time.DateOnly),
		"courseDateStart": today.AddDate(0, 0, -2).Format(time.DateOnly),
		"courseDateEnd":   today.Format(time.DateOnly),
	}
}

func TestRenewCertificateRemovesPredecessorFromExpiringViews(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	company := e.seedCompanyWithNotifications(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, company, 10)

	if !e.notificationCandidates(t, 30)[predecessor.ID] {
		t.Fatal("wygasające zaświadczenie musi być w kandydatach do przypomnień przed przedłużeniem")
	}
	countBefore, listedBefore := e.dashboardExpiring(t)
	if !listedBefore[predecessor.ID] {
		t.Fatal("wygasające zaświadczenie musi być na liście pulpitu przed przedłużeniem")
	}

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("renew: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	successor := decodeRenewedState(t, resp)
	if successor.ID == predecessor.ID {
		t.Fatal("przedłużenie musi wystawić NOWY dokument")
	}
	// Numer nadaje serwer i jest kolejny w kursie i ROKU następcy, a nie poprzednika
	// (ten kończył kurs pięć lat wcześniej), więc porównujemy całe wpisy rejestru.
	if successor.RegistryYear != int64(time.Now().Year()) {
		t.Fatalf("następca trafia do rejestru bieżącego roku, a ma %d", successor.RegistryYear)
	}
	if successor.RegistryYear == predecessor.RegistryYear && successor.RegistryNumber == predecessor.RegistryNumber {
		t.Fatalf("następca musi zająć własny wpis w rejestrze, a ma ten sam: %d/%d", successor.RegistryNumber, successor.RegistryYear)
	}
	if successor.RenewalOfCertificateID == nil || *successor.RenewalOfCertificateID != predecessor.ID {
		t.Fatalf("następca musi wskazywać poprzednika: %s", resp.Body)
	}
	if successor.RenewedAt != nil {
		t.Fatalf("świeżo wystawiony następca nie jest przedłużony: %s", resp.Body)
	}

	// Kursant i kurs pochodzą ze starego wiersza, nie z ciała żądania.
	old := decodeRenewedState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", predecessor.ID), nil, nil))
	if successor.StudentID != old.StudentID || successor.CourseID != old.CourseID {
		t.Fatalf("następca musi dotyczyć tego samego kursanta i kursu: %+v vs %+v", successor, old)
	}
	if old.RenewedAt == nil || old.RenewedByCertificateID == nil || *old.RenewedByCertificateID != successor.ID {
		t.Fatalf("poprzednik musi wskazywać następcę: %+v", old)
	}
	if old.RevokedAt != nil {
		t.Fatal("przedłużenie nie jest unieważnieniem")
	}

	// Wypada z obu zestawień wygasających i z przypomnień.
	if e.notificationCandidates(t, 30)[predecessor.ID] {
		t.Fatal("przedłużony dokument nie może dalej trafiać do przypomnień")
	}
	countAfter, listedAfter := e.dashboardExpiring(t)
	if listedAfter[predecessor.ID] {
		t.Fatal("przedłużony dokument nie może zostać na liście pulpitu")
	}
	if countAfter != countBefore-1 {
		t.Fatalf("licznik wygasających ma spaść dokładnie o 1: %d -> %d", countBefore, countAfter)
	}

	// A poza przypomnieniami nie traci nic: dalej się drukuje i weryfikuje jako ważny.
	pdf := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d/pdf", predecessor.ID), nil, nil)
	if pdf.Status != http.StatusOK {
		t.Fatalf("przedłużony dokument musi się dalej drukować, got %d: %s", pdf.Status, pdf.Body)
	}
	public := e.callPublic(t, http.MethodGet, "/public/certificates/"+old.VerificationCode)
	if public.Status != http.StatusOK {
		t.Fatalf("publiczna weryfikacja: expected 200, got %d: %s", public.Status, public.Body)
	}
	var publicBody struct {
		Data struct {
			Status  string `json:"status"`
			Renewed bool   `json:"renewed"`
		} `json:"data"`
	}
	public.decode(t, &publicBody)
	if publicBody.Data.Status != "valid" {
		t.Fatalf("przedłużony dokument jest nadal ważny, a status to %q", publicBody.Data.Status)
	}
	if !publicBody.Data.Renewed {
		t.Fatalf("publiczna weryfikacja ma pokazać przedłużenie osobnym polem: %s", public.Body)
	}
}

func TestRenewCertificateRejectsSecondRenewalAndRevokedDocuments(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	company := e.seedCompanyWithNotifications(t)

	predecessor := e.seedExpiringCertificate(t, course.ID, company, 10)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil); resp.Status != http.StatusCreated {
		t.Fatalf("pierwsze przedłużenie: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	again := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
	if again.Status != http.StatusConflict || again.errorMessage(t) != "certificate already renewed" {
		t.Fatalf("drugie przedłużenie: expected 409 certificate already renewed, got %d: %s", again.Status, again.Body)
	}

	revoked := e.seedExpiringCertificate(t, course.ID, company, 10)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", revoked.ID), map[string]any{"reason": "x"}, nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", resp.Status, resp.Body)
	}
	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", revoked.ID), renewPayload(), nil)
	if resp.Status != http.StatusConflict || resp.errorMessage(t) != "certificate is revoked" {
		t.Fatalf("przedłużenie unieważnionego: expected 409 certificate is revoked, got %d: %s", resp.Status, resp.Body)
	}

	missing := e.mustCall(t, http.MethodPost, "/certificates/99999999/renew", renewPayload(), nil)
	if missing.Status != http.StatusNotFound {
		t.Fatalf("przedłużenie nieistniejącego: expected 404, got %d: %s", missing.Status, missing.Body)
	}

	// Kursanta i kursu nie da się podmienić ciałem żądania - pola są nieznane.
	payload := renewPayload()
	payload["studentId"] = e.seedStudent(t)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", e.seedExpiringCertificate(t, course.ID, company, 10).ID), payload, nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("studentId w ciele przedłużenia musi dać 400, got %d: %s", resp.Status, resp.Body)
	}
}

func TestRenewCertificateIsIdempotent(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)
	key := fmt.Sprintf("renew-%d", nextSeed())
	headers := map[string]string{"Idempotency-Key": key}
	payload := renewPayload()

	first := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), payload, headers)
	if first.Status != http.StatusCreated {
		t.Fatalf("pierwsze przedłużenie: expected 201, got %d: %s", first.Status, first.Body)
	}
	second := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), payload, headers)
	if second.Status != http.StatusOK {
		t.Fatalf("ponowienie z tym samym kluczem: expected 200, got %d: %s", second.Status, second.Body)
	}
	if got, want := decodeRenewedState(t, second).ID, decodeRenewedState(t, first).ID; got != want {
		t.Fatalf("ponowienie musi zwrócić ten sam dokument: %d vs %d", got, want)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE renewed_by_certificate_id IS NOT NULL AND id = $1`, predecessor.ID); n != 1 {
		t.Fatalf("ponowienie nie może wystawić drugiego następcy, znaczników: %d", n)
	}

	// Ten sam klucz z innym ciałem to konflikt, jak przy wystawianiu.
	changed := renewPayload()
	changed["certificateDate"] = time.Now().AddDate(0, 0, -1).Format(time.DateOnly)
	conflict := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), changed, headers)
	if conflict.Status != http.StatusConflict {
		t.Fatalf("ten sam klucz z innym ciałem: expected 409, got %d: %s", conflict.Status, conflict.Body)
	}
}

func TestRenewCertificateRequiresCertificatesWriteScope(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)
	readOnly := e.seedScopedAPIKey(t, "certificates:read")

	resp := e.callWithKey(t, readOnly, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload())
	if resp.Status != http.StatusForbidden {
		t.Fatalf("klucz bez certificates:write: expected 403, got %d: %s", resp.Status, resp.Body)
	}
}

func TestRenewPlatformCertificateEmitsIssuedAndRenewed(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)
	key := fmt.Sprintf("renew-wh-%d", nextSeed())

	predecessor, _ := e.issueWithKey(t, course.ID, key)
	oldNumber := fmt.Sprintf("%d/%s/%d", predecessor.RegistryNumber, course.Symbol, predecessor.RegistryYear)

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID),
		renewPayload(), map[string]string{"Idempotency-Key": key + "-renew"})
	if resp.Status != http.StatusCreated {
		t.Fatalf("renew: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	successor := decodeRenewedState(t, resp)
	newNumber := fmt.Sprintf("%d/%s/%d", successor.RegistryNumber, course.Symbol, successor.RegistryYear)

	// Dwa zdarzenia: issued następcy i renewed poprzednika. Kolejność między podmiotami
	// nie jest gwarantowana, więc ładunek renewed niesie numery obu dokumentów.
	events := receiver.waitFor(t, "certificate.renewed", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.renewed", oldNumber)) == 1 &&
			len(filterEvents(ev, "certificate.issued", newNumber)) == 1
	})
	renewed := filterEvents(events, "certificate.renewed", oldNumber)[0].Event
	if renewed["renewed_by_certificate_number"] != newNumber {
		t.Fatalf("certificate.renewed musi nieść numer następcy: %v", renewed)
	}
	if renewed["renewed_at"] == nil || renewed["renewed_at"] == "" {
		t.Fatalf("certificate.renewed musi nieść datę przedłużenia: %v", renewed)
	}
}

// Asymetria bramy: zdarzenie certificate.renewed zależy od PRZEDŁUŻANEGO dokumentu,
// a certificate.issued następcy - od tego, czy sam ma klucz idempotencji. Przedłużenie
// dokumentu platformowego z aplikacji webowej (bez klucza) musi dojść do odbiorcy,
// bo to jego numer przestaje wymagać przypomnień; o nowym dokumencie, którego odbiorca
// nie zna, nie zawiadamiamy - dostalibyśmy 422 i WEBHOOK ALERT.
func TestRenewPlatformCertificateFromWebAppStillNotifiesAboutPredecessor(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)

	predecessor, _ := e.issueWithKey(t, course.ID, fmt.Sprintf("renew-mixed-%d", nextSeed()))
	oldNumber := fmt.Sprintf("%d/%s/%d", predecessor.RegistryNumber, course.Symbol, predecessor.RegistryYear)

	// Bez Idempotency-Key, czyli tak jak przedłuża aplikacja webowa.
	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("renew: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	successor := decodeRenewedState(t, resp)

	receiver.waitFor(t, "certificate.renewed", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.renewed", oldNumber)) == 1
	})
	if n := e.countRows(t, `SELECT count(*) FROM webhook_events WHERE subject_key = $1`,
		fmt.Sprintf("certificate:%d", successor.ID)); n != 0 {
		t.Fatalf("o następcy bez klucza idempotencji nie zawiadamiamy, zdarzeń: %d", n)
	}
}

// Równoległe przedłużenia tego samego dokumentu szereguje blokada wiersza (L2):
// jedno wystawia następcę, pozostałe dostają 409. Bez tej blokady powstałoby kilku
// następców albo zakleszczenie na blokadzie rejestru.
func TestConcurrentRenewalsProduceExactlyOneSuccessor(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)

	const attempts = 5
	var wg sync.WaitGroup
	statuses := make([]int, attempts)
	bodies := make([][]byte, attempts)
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := e.call(http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
			if err != nil {
				statuses[i] = -1
				return
			}
			statuses[i] = resp.Status
			bodies[i] = resp.Body
		}(i)
	}
	wg.Wait()

	created := 0
	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("próba %d: oczekiwano 201 albo 409, dostano %d: %s", i, status, bodies[i])
		}
	}
	if created != 1 {
		t.Fatalf("dokładnie jedno przedłużenie może się udać, udało się %d", created)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE renewed_by_certificate_id IS NOT NULL AND id = $1`, predecessor.ID); n != 1 {
		t.Fatalf("poprzednik musi mieć dokładnie jednego następcę, znaczników: %d", n)
	}
}

func TestRenewFromWebAppSkipsWebhookForNonPlatformCertificate(t *testing.T) {
	e := requireEnv(t)
	newWebhookReceiver(t, e)
	course := e.seedCourse(t)
	// Bez Idempotency-Key, czyli dokument nieznany platformie.
	predecessor := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)

	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil); resp.Status != http.StatusCreated {
		t.Fatalf("renew: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	if n := e.countRows(t, `SELECT count(*) FROM webhook_events WHERE event_type = 'certificate.renewed' AND subject_key = $1`,
		fmt.Sprintf("certificate:%d", predecessor.ID)); n != 0 {
		t.Fatalf("dokument bez klucza idempotencji nie generuje certificate.renewed, zdarzeń: %d", n)
	}
}

func TestDeletingSuccessorBringsPredecessorBackToExpiringViews(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	company := e.seedCompanyWithNotifications(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, company, 10)

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("renew: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	successor := decodeRenewedState(t, resp)
	if e.notificationCandidates(t, 30)[predecessor.ID] {
		t.Fatal("przed skasowaniem następcy poprzednik nie powinien być w przypomnieniach")
	}

	if del := e.mustCall(t, http.MethodDelete, fmt.Sprintf("/certificates/%d", successor.ID), nil, nil); del.Status != http.StatusOK {
		t.Fatalf("delete successor: expected 200, got %d: %s", del.Status, del.Body)
	}

	// Dokument "przedłużony" zamiennikiem, którego już nie ma, zniknąłby z zestawień
	// na zawsze - czyli dokładnie problem, który ta funkcja naprawia.
	if !e.notificationCandidates(t, 30)[predecessor.ID] {
		t.Fatal("po skasowaniu następcy poprzednik musi wrócić do przypomnień")
	}
	if _, listed := e.dashboardExpiring(t); !listed[predecessor.ID] {
		t.Fatal("po skasowaniu następcy poprzednik musi wrócić na listę pulpitu")
	}
	old := decodeRenewedState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", predecessor.ID), nil, nil))
	if old.RenewedAt != nil || old.RenewedByCertificateID != nil {
		t.Fatalf("znacznik przedłużenia musi zostać zdjęty: %+v", old)
	}

	// Po zdjęciu znacznika można przedłużyć ponownie.
	retry := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil)
	if retry.Status != http.StatusCreated {
		t.Fatalf("ponowne przedłużenie po skasowaniu następcy: expected 201, got %d: %s", retry.Status, retry.Body)
	}
}

// Łańcuch A→B→C jest dozwolony: po roku przedłuża się już następcę.
func TestRenewChainIsAllowed(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	a := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)

	b := decodeRenewedState(t, e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", a.ID), renewPayload(), nil))
	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", b.ID), renewPayload(), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("przedłużenie następcy: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	c := decodeRenewedState(t, resp)
	if c.RenewalOfCertificateID == nil || *c.RenewalOfCertificateID != b.ID {
		t.Fatalf("C musi wskazywać B jako poprzednika: %s", resp.Body)
	}
	mid := decodeRenewedState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", b.ID), nil, nil))
	if mid.RenewedByCertificateID == nil || *mid.RenewedByCertificateID != c.ID {
		t.Fatalf("B musi wskazywać C jako następcę: %+v", mid)
	}
	if mid.RenewalOfCertificateID == nil || *mid.RenewalOfCertificateID != a.ID {
		t.Fatalf("B musi nadal wskazywać A jako poprzednika: %+v", mid)
	}
}

// Przedłużony dokument wolno dalej edytować i unieważniać - znacznik dotyczy tylko
// przypomnień o wygasaniu.
func TestRenewedCertificateStaysEditableAndRevocable(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	predecessor := e.seedExpiringCertificate(t, course.ID, e.seedCompanyWithNotifications(t), 10)
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", predecessor.ID), renewPayload(), nil); resp.Status != http.StatusCreated {
		t.Fatalf("renew: %d %s", resp.Status, resp.Body)
	}

	current := decodeCertificateState(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", predecessor.ID), nil, nil))
	patch := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/certificates/%d", predecessor.ID), map[string]any{
		"studentId":       current.StudentID,
		"certificateDate": current.Date,
		"courseDateStart": current.CourseDateStart,
		"courseDateEnd":   current.CourseDateEnd,
	}, nil)
	if patch.Status != http.StatusOK {
		t.Fatalf("edycja przedłużonego dokumentu: expected 200, got %d: %s", patch.Status, patch.Body)
	}
	if state := decodeRenewedState(t, patch); state.RenewedAt == nil {
		t.Fatalf("edycja nie może zdjąć znacznika przedłużenia: %s", patch.Body)
	}
	revoke := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", predecessor.ID), map[string]any{"reason": "pomyłka"}, nil)
	if revoke.Status != http.StatusOK {
		t.Fatalf("unieważnienie przedłużonego dokumentu: expected 200, got %d: %s", revoke.Status, revoke.Body)
	}
	if !strings.Contains(string(revoke.Body), `"renewedAt"`) {
		t.Fatalf("odpowiedź musi nadal nieść stan przedłużenia: %s", revoke.Body)
	}
}
