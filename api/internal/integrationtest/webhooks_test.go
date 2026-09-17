package integrationtest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

// Zlecenie, punkt 7: webhook wychodzący.

type receivedWebhook struct {
	RawBody   []byte
	Signature string
	Event     map[string]any
	At        time.Time
}

// webhookReceiver to odbiorca po stronie "platformy": weryfikuje podpis z surowego
// ciała, zapisuje zdarzenia i odpowiada kodem wybranym przez test.
type webhookReceiver struct {
	t        *testing.T
	secret   string
	server   *httptest.Server
	mu       sync.Mutex
	received []receivedWebhook
	// respond wybiera odpowiedź dla n-tej (od 1) próby doręczenia danego zdarzenia.
	respond  func(event map[string]any, attempt int) (status int, delay time.Duration)
	attempts map[string]int
}

func newWebhookReceiver(t *testing.T, e *testEnv) *webhookReceiver {
	t.Helper()
	r := &webhookReceiver{
		t:        t,
		secret:   fmt.Sprintf("secret-%d", nextSeed()),
		attempts: map[string]int{},
		respond:  func(map[string]any, int) (int, time.Duration) { return http.StatusOK, 0 },
	}
	r.server = httptest.NewServer(http.HandlerFunc(r.handle))
	var endpointID int64
	if err := e.pool.QueryRow(context.Background(), `
		INSERT INTO webhook_endpoints (name, url, secret) VALUES ($1, $2, $3) RETURNING id`,
		"integration", r.server.URL+"/hooks", r.secret).Scan(&endpointID); err != nil {
		t.Fatalf("seed webhook endpoint: %v", err)
	}
	t.Cleanup(func() {
		if _, err := e.pool.Exec(context.Background(), `UPDATE webhook_endpoints SET active = false WHERE id = $1`, endpointID); err != nil {
			t.Errorf("deactivate endpoint: %v", err)
		}
		r.server.Close()
	})
	return r
}

func (r *webhookReceiver) handle(w http.ResponseWriter, req *http.Request) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	mac := hmac.New(sha256.New, []byte(r.secret))
	mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	signature := req.Header.Get("X-Az-Signature")
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if req.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	recordWebhook(raw)

	r.mu.Lock()
	key := fmt.Sprint(event["event"], event["timestamp"], event["certificate_number"], event["external_program_id"])
	r.attempts[key]++
	attempt := r.attempts[key]
	r.received = append(r.received, receivedWebhook{RawBody: raw, Signature: signature, Event: event, At: time.Now()})
	respond := r.respond
	r.mu.Unlock()

	status, delay := respond(event, attempt)
	if delay > 0 {
		time.Sleep(delay)
	}
	w.WriteHeader(status)
}

func (r *webhookReceiver) setRespond(fn func(event map[string]any, attempt int) (int, time.Duration)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.respond = fn
}

func (r *webhookReceiver) events() []receivedWebhook {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.received)
}

// waitFor czeka, aż odbiorca dostanie zdarzenia spełniające warunek.
func (r *webhookReceiver) waitFor(t *testing.T, what string, cond func([]receivedWebhook) bool) []receivedWebhook {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if got := r.events(); cond(got) {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; received: %s", what, describeWebhooks(r.events()))
	return nil
}

func describeWebhooks(events []receivedWebhook) string {
	out := ""
	for _, ev := range events {
		out += string(ev.RawBody) + "\n"
	}
	return out
}

func filterEvents(events []receivedWebhook, eventType, certificateNumber string) []receivedWebhook {
	var out []receivedWebhook
	for _, ev := range events {
		if ev.Event["event"] == eventType && (certificateNumber == "" || ev.Event["certificate_number"] == certificateNumber) {
			out = append(out, ev)
		}
	}
	return out
}

func (e *testEnv) deliveryState(t *testing.T, eventType, subject string) (status string, attempts int) {
	t.Helper()
	err := e.pool.QueryRow(context.Background(), `
		SELECT d.status, d.attempts
		FROM webhook_deliveries d JOIN webhook_events ev ON ev.id = d.event_id
		JOIN webhook_endpoints en ON en.id = d.endpoint_id
		WHERE ev.event_type = $1 AND ev.subject_key = $2 AND en.active
		ORDER BY d.id DESC LIMIT 1`, eventType, subject).Scan(&status, &attempts)
	if err != nil {
		t.Fatalf("delivery state %s %s: %v", eventType, subject, err)
	}
	return status, attempts
}

func (e *testEnv) issueWithKey(t *testing.T, courseID int64, key string) (createCertificateData, string) {
	t.Helper()
	payload := certificatePayload(e.seedStudent(t), courseID)
	payload["certificateDate"] = time.Now().Format("2006-01-02")
	payload["courseDateStart"] = time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	payload["courseDateEnd"] = time.Now().Format("2006-01-02")
	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, map[string]string{"Idempotency-Key": key})
	if resp.Status != http.StatusCreated {
		t.Fatalf("issue with key: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	return decodeCreated(t, resp), payload["certificateDate"].(string)
}

func TestWebhookCertificateIssuedIsSignedOverRawBody(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)
	key := fmt.Sprintf("wh-issued-%d", nextSeed())

	cert, issuedAt := e.issueWithKey(t, course.ID, key)
	number := fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, course.Symbol, cert.RegistryYear)
	events := receiver.waitFor(t, "certificate.issued", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.issued", number)) == 1
	})
	issued := filterEvents(events, "certificate.issued", number)[0]

	// Odbiorca przyjął zdarzenie, więc podpis z surowego ciała się zgadza; sprawdzamy też
	// jawnie, że podpis to hex małymi literami.
	mac := hmac.New(sha256.New, []byte(receiver.secret))
	mac.Write(issued.RawBody)
	if issued.Signature != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("signature mismatch: %s", issued.Signature)
	}

	ev := issued.Event
	if ev["idempotency_key"] != key || ev["issued_at"] != issuedAt || ev["verification_code"] != cert.VerificationCode ||
		ev["certificate_number"] != number || ev["pdf_url"] != fmt.Sprintf("%s/api/v1/certificates/%d/pdf", webhookTestPublicBaseURL, cert.ID) {
		t.Fatalf("unexpected certificate.issued payload: %s", issued.RawBody)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.999999Z", fmt.Sprint(ev["timestamp"])); err != nil {
		t.Fatalf("timestamp must be ISO 8601 UTC with Z, got %v", ev["timestamp"])
	}
	wantValidUntil := time.Now().AddDate(0, 0, 5*365).Format("2006-01-02")
	if ev["valid_until"] != wantValidUntil {
		t.Fatalf("valid_until: expected %s, got %v", wantValidUntil, ev["valid_until"])
	}
	for field := range ev {
		if !slices.Contains([]string{"event", "timestamp", "idempotency_key", "certificate_number", "issued_at", "valid_until", "pdf_url", "verification_code"}, field) {
			t.Fatalf("unexpected field %q in certificate.issued", field)
		}
	}
	if status, attempts := e.deliveryState(t, "certificate.issued", fmt.Sprintf("certificate:%d", cert.ID)); status != "delivered" || attempts != 1 {
		t.Fatalf("expected delivered after 1 attempt, got %s/%d", status, attempts)
	}

	// Ponowienie POST z tym samym kluczem (200) nie tworzy drugiego zdarzenia.
	if n := e.countRows(t, `SELECT count(*) FROM webhook_events WHERE subject_key = $1`, fmt.Sprintf("certificate:%d", cert.ID)); n != 1 {
		t.Fatalf("expected one event for the certificate, found %d", n)
	}
}

func TestWebhookRetriesOn5xxAndTimeout(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	receiver.setRespond(func(event map[string]any, attempt int) (int, time.Duration) {
		switch {
		case attempt == 1:
			return http.StatusServiceUnavailable, 0
		case attempt == 2:
			return http.StatusOK, 2 * webhookTestRequestTimeout // brak odpowiedzi w czasie
		case attempt == 3:
			return http.StatusInternalServerError, 0
		default:
			return http.StatusOK, 0
		}
	})
	course := e.seedCourse(t)
	cert, _ := e.issueWithKey(t, course.ID, fmt.Sprintf("wh-retry-%d", nextSeed()))
	number := fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, course.Symbol, cert.RegistryYear)

	receiver.waitFor(t, "4 delivery attempts", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.issued", number)) >= 4
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, attempts := e.deliveryState(t, "certificate.issued", fmt.Sprintf("certificate:%d", cert.ID))
		if status == "delivered" {
			if attempts != 4 {
				t.Fatalf("expected 4 attempts, got %d", attempts)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivery not completed: %s/%d", status, attempts)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Wszystkie próby niosą identyczne bajty (ten sam znacznik i podpis).
	attempts := filterEvents(receiver.events(), "certificate.issued", number)
	for _, a := range attempts[1:] {
		if string(a.RawBody) != string(attempts[0].RawBody) {
			t.Fatalf("retry must resend the same body:\n%s\n%s", attempts[0].RawBody, a.RawBody)
		}
	}
}

func TestWebhookDoesNotRetryOn400401422(t *testing.T) {
	e := requireEnv(t)
	for _, code := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusUnprocessableEntity} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			receiver := newWebhookReceiver(t, e)
			receiver.setRespond(func(map[string]any, int) (int, time.Duration) { return code, 0 })
			course := e.seedCourse(t)
			cert, _ := e.issueWithKey(t, course.ID, fmt.Sprintf("wh-noretry-%d", nextSeed()))
			number := fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, course.Symbol, cert.RegistryYear)

			receiver.waitFor(t, "first attempt", func(ev []receivedWebhook) bool {
				return len(filterEvents(ev, "certificate.issued", number)) == 1
			})
			// Czas na kilka cykli dispatchera i opóźnień ponowień w konfiguracji testowej.
			time.Sleep(20 * webhookTestRetryDelay)
			if n := len(filterEvents(receiver.events(), "certificate.issued", number)); n != 1 {
				t.Fatalf("status %d must not be retried, got %d attempts", code, n)
			}
			if status, attempts := e.deliveryState(t, "certificate.issued", fmt.Sprintf("certificate:%d", cert.ID)); status != "failed" || attempts != 1 {
				t.Fatalf("expected failed after 1 attempt, got %s/%d", status, attempts)
			}
		})
	}
}

func TestWebhookRevokeDuplicateValidityAndOrdering(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)
	key := fmt.Sprintf("wh-flow-%d", nextSeed())

	// Pierwsze doręczenie certificate.issued pada (503) - revoked nie może wyprzedzić issued.
	receiver.setRespond(func(event map[string]any, attempt int) (int, time.Duration) {
		if event["event"] == "certificate.issued" && attempt == 1 {
			return http.StatusServiceUnavailable, 0
		}
		return http.StatusOK, 0
	})
	cert, _ := e.issueWithKey(t, course.ID, key)
	number := fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, course.Symbol, cert.RegistryYear)

	// PATCH zmieniający datę końca kursu zmienia ważność.
	newEnd := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	patch := map[string]any{
		"studentId":       e.seedStudent(t),
		"certificateDate": time.Now().Format("2006-01-02"),
		"courseDateStart": time.Now().AddDate(0, 0, -3).Format("2006-01-02"),
		"courseDateEnd":   newEnd,
	}
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/certificates/%d", cert.ID), patch, nil); resp.Status != http.StatusOK {
		t.Fatalf("PATCH: %d %s", resp.Status, resp.Body)
	}
	dup := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", cert.ID), map[string]any{"reason": "Utrata"}, nil)
	if dup.Status != http.StatusOK {
		t.Fatalf("duplicate: %d %s", dup.Status, dup.Body)
	}
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", cert.ID), map[string]any{"reason": "Błędne dane"}, nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", resp.Status, resp.Body)
	}

	events := receiver.waitFor(t, "issued, validity_changed, duplicate_issued and revoked", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.revoked", number)) == 1 &&
			len(filterEvents(ev, "certificate.validity_changed", number)) == 1 &&
			len(filterEvents(ev, "certificate.duplicate_issued", number)) == 1
	})

	// Kolejność dla oryginału: issued (po ponowieniu) -> validity_changed -> revoked, rosnące znaczniki.
	var sequence []string
	var timestamps []string
	for _, ev := range events {
		if ev.Event["certificate_number"] != number {
			continue
		}
		if ev.Event["event"] == "certificate.issued" && len(sequence) > 0 && sequence[len(sequence)-1] == "certificate.issued" {
			continue
		}
		sequence = append(sequence, fmt.Sprint(ev.Event["event"]))
		timestamps = append(timestamps, fmt.Sprint(ev.Event["timestamp"]))
	}
	want := []string{"certificate.issued", "certificate.validity_changed", "certificate.duplicate_issued", "certificate.revoked"}
	if !slices.Equal(sequence, want) {
		t.Fatalf("delivery order for %s: got %v, want %v\n%s", number, sequence, want, describeWebhooks(events))
	}
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i] <= timestamps[i-1] {
			t.Fatalf("timestamps must strictly increase per certificate: %v", timestamps)
		}
	}

	revoked := filterEvents(events, "certificate.revoked", number)[0].Event
	if revoked["reason"] != "Błędne dane" || len(revoked) != 4 {
		t.Fatalf("unexpected certificate.revoked payload: %v", revoked)
	}
	validity := filterEvents(events, "certificate.validity_changed", number)[0].Event
	wantValidUntil, _ := time.Parse("2006-01-02", newEnd)
	if validity["valid_until"] != wantValidUntil.AddDate(0, 0, 5*365).Format("2006-01-02") || len(validity) != 4 {
		t.Fatalf("unexpected certificate.validity_changed payload: %v", validity)
	}
}

// Duplikat to ten sam dokument, więc odbiorca dostaje osobne zdarzenie o wystawieniu
// wtórnika, a nie drugie certificate.issued, które nadpisałoby mu dokument.
func TestWebhookDuplicateIssuedForTheSameCertificate(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)
	key := fmt.Sprintf("wh-duplicate-%d", nextSeed())

	cert, _ := e.issueWithKey(t, course.ID, key)
	number := fmt.Sprintf("%d/%s/%d", cert.RegistryNumber, course.Symbol, cert.RegistryYear)

	// Duplikat wystawiany przez pracownika w aplikacji webowej (ciasteczko sesji, bez klucza API).
	duplicateResp := e.callWithSession(t, http.MethodPost, fmt.Sprintf("/certificates/%d/duplicate", cert.ID),
		map[string]any{"reason": "Kursant zgubił oryginał"})
	if duplicateResp.Status != http.StatusOK {
		t.Fatalf("duplicate from web session: expected 200, got %d: %s", duplicateResp.Status, duplicateResp.Body)
	}

	events := receiver.waitFor(t, "certificate.duplicate_issued", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.duplicate_issued", number)) == 1
	})
	duplicateEvent := filterEvents(events, "certificate.duplicate_issued", number)[0].Event
	if duplicateEvent["duplicate_issued_at"] != time.Now().Format("2006-01-02") {
		t.Fatalf("duplicate_issued_at must be today, got %v", duplicateEvent["duplicate_issued_at"])
	}
	if duplicateEvent["reason"] != "Kursant zgubił oryginał" {
		t.Fatalf("unexpected reason: %v", duplicateEvent["reason"])
	}
	if len(duplicateEvent) != 5 {
		t.Fatalf("unexpected fields in certificate.duplicate_issued: %s", filterEvents(events, "certificate.duplicate_issued", number)[0].RawBody)
	}

	// Numer, kod i ważność się nie zmieniają, więc drugiego certificate.issued być nie może.
	if n := len(filterEvents(events, "certificate.issued", number)); n != 1 {
		t.Fatalf("expected exactly one certificate.issued for %s, got %d", number, n)
	}
	// Zdarzenie o duplikacie jest późniejsze niż wystawienie - odbiorca stosuje je po kolei.
	issuedAt := fmt.Sprint(filterEvents(events, "certificate.issued", number)[0].Event["timestamp"])
	if fmt.Sprint(duplicateEvent["timestamp"]) <= issuedAt {
		t.Fatalf("duplicate timestamp %v must be later than issued %v", duplicateEvent["timestamp"], issuedAt)
	}
}

func TestWebhookNotSentForCertificatesWithoutIdempotencyKeyOrFailedWrites(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	course := e.seedCourse(t)

	plain := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	if resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/revoke", plain.ID), map[string]any{"reason": "x"}, nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: %d %s", resp.Status, resp.Body)
	}

	// Nieudany zapis (numer zajęty) nie może zostawić zdarzenia.
	failing := certificatePayload(e.seedStudent(t), course.ID)
	failing["registryYear"] = plain.RegistryYear
	failing["registryNumber"] = plain.RegistryNumber
	resp := e.mustCall(t, http.MethodPost, "/certificates", failing, map[string]string{"Idempotency-Key": fmt.Sprintf("wh-fail-%d", nextSeed())})
	if resp.Status != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.Status, resp.Body)
	}

	// Kontrolne zdarzenie, żeby wiedzieć, że dispatcher działa.
	control, _ := e.issueWithKey(t, course.ID, fmt.Sprintf("wh-control-%d", nextSeed()))
	controlNumber := fmt.Sprintf("%d/%s/%d", control.RegistryNumber, course.Symbol, control.RegistryYear)
	events := receiver.waitFor(t, "control event", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "certificate.issued", controlNumber)) == 1
	})
	if len(events) != 1 {
		t.Fatalf("only the control event may be sent, got:\n%s", describeWebhooks(events))
	}
	if n := e.countRows(t, `SELECT count(*) FROM webhook_events WHERE subject_key = $1`, fmt.Sprintf("certificate:%d", plain.ID)); n != 0 {
		t.Fatalf("certificate without idempotency key must not produce events, found %d", n)
	}
}

func TestWebhookProgramUpdatedOnlyForPlatformCourseContentChanges(t *testing.T) {
	e := requireEnv(t)
	receiver := newWebhookReceiver(t, e)
	n := nextSeed()
	platform := e.createCourseViaAPI(t, fmt.Sprintf("WHP%dA", n), nil)
	other := e.createCourseViaAPI(t, fmt.Sprintf("WHP%dB", n), nil)
	if resp := e.mustCall(t, http.MethodPut, fmt.Sprintf("/courses/%d/platform-delivery", platform), map[string]any{"deliveredByPlatform": true}, nil); resp.Status != http.StatusOK {
		t.Fatalf("flag: %d %s", resp.Status, resp.Body)
	}

	update := func(id int64, symbol string, mutate func(map[string]any)) {
		body := courseWritePayload(symbol, nil)
		mutate(body)
		if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/courses/%d", id), body, nil); resp.Status != http.StatusOK {
			t.Fatalf("PATCH course: %d %s", resp.Status, resp.Body)
		}
	}
	update(other, fmt.Sprintf("WHP%dB", n), func(b map[string]any) { b["name"] = "Zmieniona nazwa" })
	update(platform, fmt.Sprintf("WHP%dA", n), func(b map[string]any) {}) // bez zmian treści
	update(platform, fmt.Sprintf("WHP%dA", n), func(b map[string]any) { b["certFrontPage"] = "<p>inny szablon</p>" })
	update(platform, fmt.Sprintf("WHP%dA", n), func(b map[string]any) { b["expiryTime"] = 3 })

	events := receiver.waitFor(t, "program.updated", func(ev []receivedWebhook) bool {
		return len(filterEvents(ev, "program.updated", "")) >= 1
	})
	time.Sleep(20 * webhookTestRetryDelay)
	events = filterEvents(receiver.events(), "program.updated", "")
	if len(events) != 1 {
		t.Fatalf("expected exactly one program.updated (validity change of the platform course), got:\n%s", describeWebhooks(events))
	}
	ev := events[0].Event
	if ev["external_program_id"] != float64(platform) || len(ev) != 3 {
		t.Fatalf("unexpected program.updated payload: %s", events[0].RawBody)
	}
}

var recordWebhookMu sync.Mutex

// recordWebhook dopisuje ciało do pliku z IT_RECORD_WEBHOOKS (JSON Lines) - do sprawdzenia
// zgodności ze schematami webhooków w docs/api/openapi.yaml.
func recordWebhook(raw []byte) {
	file := os.Getenv("IT_RECORD_WEBHOOKS")
	if file == "" {
		return
	}
	recordWebhookMu.Lock()
	defer recordWebhookMu.Unlock()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(append([]byte{}, raw...), '\n'))
}
