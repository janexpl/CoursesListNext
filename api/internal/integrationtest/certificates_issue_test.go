package integrationtest

import (
	"bytes"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
)

// Zlecenie, punkt 3: wystawianie zaświadczeń odporne na ponowienia.

type createCertificateData struct {
	ID             int64 `json:"id"`
	RegistryYear   int64 `json:"registryYear"`
	RegistryNumber int64 `json:"registryNumber"`
}

func certificatePayload(studentID, courseID int64) map[string]any {
	return map[string]any{
		"studentId":       studentID,
		"courseId":        courseID,
		"certificateDate": "2026-03-15",
		"courseDateStart": "2026-03-10",
		"courseDateEnd":   "2026-03-15",
	}
}

func decodeCreated(t *testing.T, resp apiResponse) createCertificateData {
	t.Helper()
	var body struct {
		Data createCertificateData `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func TestIdempotencyKeySameBodyReturnsOriginalResponse(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	payload := certificatePayload(student, course.ID)
	payload["registryYear"] = 2026
	payload["registryNumber"] = 1
	headers := map[string]string{"Idempotency-Key": fmt.Sprintf("it-same-%d", nextSeed())}

	first := e.mustCall(t, http.MethodPost, "/certificates", payload, headers)
	if first.Status != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d: %s", first.Status, first.Body)
	}
	second := e.mustCall(t, http.MethodPost, "/certificates", payload, headers)
	if second.Status != http.StatusOK {
		t.Fatalf("second call: expected 200, got %d: %s", second.Status, second.Body)
	}
	if !bytes.Equal(bytes.TrimSpace(first.Body), bytes.TrimSpace(second.Body)) {
		t.Fatalf("replay body differs:\nfirst:  %s\nsecond: %s", first.Body, second.Body)
	}
	if got := decodeCreated(t, second).ID; got == 0 || got != decodeCreated(t, first).ID {
		t.Fatalf("replay returned id %d, want %d", got, decodeCreated(t, first).ID)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE student_id = $1`, student); n != 1 {
		t.Fatalf("expected exactly 1 certificate, found %d", n)
	}
}

func TestIdempotencyKeyDifferentBodyReturnsConflict(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	otherStudent := e.seedStudent(t)
	headers := map[string]string{"Idempotency-Key": fmt.Sprintf("it-diff-%d", nextSeed())}

	first := e.mustCall(t, http.MethodPost, "/certificates", certificatePayload(student, course.ID), headers)
	if first.Status != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d: %s", first.Status, first.Body)
	}

	second := e.mustCall(t, http.MethodPost, "/certificates", certificatePayload(otherStudent, course.ID), headers)
	if second.Status != http.StatusConflict {
		t.Fatalf("second call: expected 409, got %d: %s", second.Status, second.Body)
	}
	if msg := second.errorMessage(t); msg != "idempotency key reused with different payload" {
		t.Fatalf("unexpected error message %q", msg)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE student_id = $1`, otherStudent); n != 0 {
		t.Fatalf("expected no certificate for the second payload, found %d", n)
	}
	if n := e.countRows(t, `
		SELECT count(*) FROM certificates c JOIN registries r ON r.id = c.registry_id
		WHERE r.course_id = $1`, course.ID); n != 1 {
		t.Fatalf("expected exactly 1 certificate in course, found %d", n)
	}
}

// Równoległe ponowienia z tym samym kluczem - platforma może wysłać drugie
// żądanie, zanim pierwsze się zakończy.
func TestIdempotencyKeyConcurrentRetriesCreateOneCertificate(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)
	payload := certificatePayload(student, course.ID)
	headers := map[string]string{"Idempotency-Key": fmt.Sprintf("it-race-%d", nextSeed())}

	responses := runConcurrently(t, 8, func(int) (apiResponse, error) {
		return e.call(http.MethodPost, "/certificates", payload, headers)
	})

	created, replayed := 0, 0
	ids := map[int64]struct{}{}
	for _, resp := range responses {
		switch resp.Status {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Fatalf("unexpected status %d: %s", resp.Status, resp.Body)
		}
		ids[decodeCreated(t, resp).ID] = struct{}{}
	}
	if created != 1 || replayed != 7 || len(ids) != 1 {
		t.Fatalf("expected 1x201 + 7x200 with one id, got %d created, %d replayed, ids %v", created, replayed, ids)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE student_id = $1`, student); n != 1 {
		t.Fatalf("expected exactly 1 certificate, found %d", n)
	}
}

func TestConcurrentIssuanceWithoutRegistryNumberAssignsConsecutiveNumbers(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	const parallel = 16
	students := make([]int64, parallel)
	for i := range students {
		students[i] = e.seedStudent(t)
	}

	responses := runConcurrently(t, parallel, func(i int) (apiResponse, error) {
		return e.call(http.MethodPost, "/certificates", certificatePayload(students[i], course.ID), nil)
	})

	numbers := make([]int64, 0, parallel)
	for _, resp := range responses {
		if resp.Status != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
		}
		data := decodeCreated(t, resp)
		if data.RegistryYear != 2026 {
			t.Fatalf("expected registry year 2026 taken from courseDateEnd, got %d", data.RegistryYear)
		}
		numbers = append(numbers, data.RegistryNumber)
	}

	// Numery z odpowiedzi muszą się zgadzać z bazą - to baza jest źródłem prawdy.
	rows, err := e.pool.Query(t.Context(), `
		SELECT r.number FROM certificates c JOIN registries r ON r.id = c.registry_id
		WHERE r.course_id = $1 AND r.year = 2026 AND c.deleted_at IS NULL
		ORDER BY r.number`, course.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, n)
	}
	rows.Close()

	slices.Sort(numbers)
	want := make([]int64, parallel)
	for i := range want {
		want[i] = int64(i + 1)
	}
	if !slices.Equal(numbers, want) {
		t.Fatalf("response numbers not consecutive and distinct: %v", numbers)
	}
	if !slices.Equal(stored, want) {
		t.Fatalf("stored numbers not consecutive and distinct: %v", stored)
	}
}

func TestIssuanceWithoutRegistryYearUsesCourseDateEndThenStart(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)

	crossYear := certificatePayload(e.seedStudent(t), course.ID)
	crossYear["courseDateStart"] = "2025-12-29"
	crossYear["courseDateEnd"] = "2026-01-02"
	crossYear["certificateDate"] = "2026-01-02"
	resp := e.mustCall(t, http.MethodPost, "/certificates", crossYear, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
	}
	if data := decodeCreated(t, resp); data.RegistryYear != 2026 || data.RegistryNumber != 1 {
		t.Fatalf("expected 2026/1 from courseDateEnd, got %d/%d", data.RegistryYear, data.RegistryNumber)
	}

	noEnd := certificatePayload(e.seedStudent(t), course.ID)
	delete(noEnd, "courseDateEnd")
	noEnd["courseDateStart"] = "2025-12-30"
	noEnd["certificateDate"] = "2026-01-05"
	resp = e.mustCall(t, http.MethodPost, "/certificates", noEnd, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
	}
	if data := decodeCreated(t, resp); data.RegistryYear != 2025 || data.RegistryNumber != 1 {
		t.Fatalf("expected 2025/1 from courseDateStart, got %d/%d", data.RegistryYear, data.RegistryNumber)
	}
}

func TestIssuanceWithoutRegistryNumberKeepsChronologyCheck(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)

	first := certificatePayload(e.seedStudent(t), course.ID)
	first["certificateDate"] = "2026-03-20"
	if resp := e.mustCall(t, http.MethodPost, "/certificates", first, nil); resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
	}

	// Kolejny numer (2) z datą wcześniejszą niż numer 1 łamie chronologię rejestru.
	earlier := certificatePayload(e.seedStudent(t), course.ID)
	earlier["certificateDate"] = "2026-03-16"
	resp := e.mustCall(t, http.MethodPost, "/certificates", earlier, nil)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Status, resp.Body)
	}
}

func TestIssuanceWithExplicitRegistryNumberStillWorks(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	payload := certificatePayload(e.seedStudent(t), course.ID)
	payload["registryYear"] = 2026
	payload["registryNumber"] = 7

	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Status, resp.Body)
	}
	if data := decodeCreated(t, resp); data.RegistryYear != 2026 || data.RegistryNumber != 7 {
		t.Fatalf("expected 2026/7, got %d/%d", data.RegistryYear, data.RegistryNumber)
	}

	taken := certificatePayload(e.seedStudent(t), course.ID)
	taken["registryYear"] = 2026
	taken["registryNumber"] = 7
	if resp := e.mustCall(t, http.MethodPost, "/certificates", taken, nil); resp.Status != http.StatusConflict {
		t.Fatalf("expected 409 for taken number, got %d: %s", resp.Status, resp.Body)
	}

	numberWithoutYear := certificatePayload(e.seedStudent(t), course.ID)
	numberWithoutYear["registryNumber"] = 8
	if resp := e.mustCall(t, http.MethodPost, "/certificates", numberWithoutYear, nil); resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 for registryNumber without registryYear, got %d: %s", resp.Status, resp.Body)
	}
}

func TestIdempotencyKeyHeaderValidation(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	payload := certificatePayload(e.seedStudent(t), course.ID)
	// Jawny numer, żeby jedyną przyczyną 400 mógł być nagłówek.
	payload["registryYear"] = 2026
	payload["registryNumber"] = 1

	tooLong := string(bytes.Repeat([]byte("k"), 256))
	for _, key := range []string{tooLong, "zażółć"} {
		resp := e.mustCall(t, http.MethodPost, "/certificates", payload, map[string]string{"Idempotency-Key": key})
		if resp.Status != http.StatusBadRequest {
			t.Fatalf("key %q: expected 400, got %d: %s", key[:min(len(key), 10)], resp.Status, resp.Body)
		}
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates c JOIN registries r ON r.id = c.registry_id WHERE r.course_id = $1`, course.ID); n != 0 {
		t.Fatalf("expected no certificate for rejected keys, found %d", n)
	}
}

// runConcurrently startuje wszystkie żądania naraz: gorutyny czekają na wspólny
// sygnał, więc żadne nie zdąży się zakończyć przed wysłaniem pozostałych.
func runConcurrently(t *testing.T, n int, fn func(i int) (apiResponse, error)) []apiResponse {
	t.Helper()
	start := make(chan struct{})
	responses := make([]apiResponse, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			responses[i], errs[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	return responses
}
