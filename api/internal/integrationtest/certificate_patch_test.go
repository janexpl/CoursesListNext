package integrationtest

import (
	"fmt"
	"net/http"
	"testing"
)

// Wykryte przy punkcie 7: odpowiedź PATCH /certificates/{id} (i wpis "after" w historii)
// musi zawierać dane PO zmianie.
func TestPatchCertificateReturnsUpdatedData(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	cert := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	newStudent := e.seedStudent(t)

	resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/certificates/%d", cert.ID), map[string]any{
		"studentId":       newStudent,
		"certificateDate": "2026-03-16",
		"courseDateStart": "2026-03-11",
		"courseDateEnd":   "2026-03-14",
	}, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("PATCH: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	got := decodeCertificateState(t, resp)
	if got.StudentID != newStudent || got.Date != "2026-03-16" || got.CourseDateStart != "2026-03-11" || deref(got.CourseDateEnd) != "2026-03-14" {
		t.Fatalf("PATCH response must reflect the update, got %+v", got)
	}

	var after string
	if err := e.pool.QueryRow(t.Context(), `
		SELECT after_data->>'date' FROM audit_log
		WHERE entity_type = 'certificate' AND entity_id = $1 AND action = 'update'
		ORDER BY id DESC LIMIT 1`, cert.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != "2026-03-16" {
		t.Fatalf("audit log 'after' must hold the updated date, got %q", after)
	}
}
