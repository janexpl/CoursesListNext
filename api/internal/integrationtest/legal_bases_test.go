package integrationtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/auth"
)

// Biblioteka podstaw prawnych: kurs wskazuje podstawę, szablon wstawia ją znacznikiem
// {{ podstawa_prawna }}, a zaświadczenie zamraża treść przy wystawieniu.

type legalBasisData struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Content     string `json:"content"`
	CourseCount int64  `json:"courseCount"`
	Courses     []struct {
		ID int64 `json:"id"`
	} `json:"courses"`
}

func decodeLegalBasis(t *testing.T, resp apiResponse) legalBasisData {
	t.Helper()
	var body struct {
		Data legalBasisData `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

func (e *testEnv) createLegalBasis(t *testing.T, content string) legalBasisData {
	t.Helper()
	resp := e.mustCall(t, http.MethodPost, "/legal-bases", map[string]any{
		"name":    fmt.Sprintf("Podstawa testowa %d", nextSeed()),
		"content": content,
	}, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create legal basis: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	return decodeLegalBasis(t, resp)
}

type courseLegalBasisState struct {
	CertFrontPage string `json:"certFrontPage"`
	LegalBasis    *struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	} `json:"legalBasis"`
}

func (e *testEnv) getCourseLegalBasis(t *testing.T, courseID int64) courseLegalBasisState {
	t.Helper()
	resp := e.mustCall(t, http.MethodGet, fmt.Sprintf("/courses/%d", courseID), nil, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("get course: %d %s", resp.Status, resp.Body)
	}
	var body struct {
		Data courseLegalBasisState `json:"data"`
	}
	resp.decode(t, &body)
	return body.Data
}

// patchCourse wysyła pełne ciało kursu (PATCH = nadpisanie) i dokłada pola z extra.
// legalBasisId podaje się w extra; jego brak w mapie to "pole pominięte".
func (e *testEnv) patchCourse(t *testing.T, courseID int64, extra map[string]any) apiResponse {
	t.Helper()
	body := map[string]any{
		"mainName":      "Szkolenie",
		"name":          fmt.Sprintf("Kurs integracyjny %d", courseID),
		"symbol":        fmt.Sprintf("LB%d", courseID),
		"expiryTime":    5,
		"courseProgram": `[{"Subject":"Temat","TheoryTime":"1","PracticeTime":"1"}]`,
		"certFrontPage": `<p>Zaświadczenie wydano na podstawie {{ podstawa_prawna }}</p>`,
	}
	for k, v := range extra {
		body[k] = v
	}
	return e.mustCall(t, http.MethodPatch, fmt.Sprintf("/courses/%d", courseID), body, nil)
}

func (e *testEnv) certificateLegalBasis(t *testing.T, certificateID int64) string {
	t.Helper()
	resp := e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", certificateID), nil, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("get certificate: %d %s", resp.Status, resp.Body)
	}
	var body struct {
		Data struct {
			LegalBasis *string `json:"legalBasis"`
		} `json:"data"`
	}
	resp.decode(t, &body)
	if body.Data.LegalBasis == nil {
		t.Fatalf("odpowiedź musi zawsze nieść pole legalBasis: %s", resp.Body)
	}
	return *body.Data.LegalBasis
}

// callAsUser wysyła żądanie w sesji nowego użytkownika o podanej roli.
func (e *testEnv) callAsUser(t *testing.T, role int, method, path string, body any) apiResponse {
	t.Helper()
	var userID int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO users (email, password, firstname, lastname, role)
		VALUES ($1, '\x00', 'Operator', 'Testowy', $2) RETURNING id`,
		fmt.Sprintf("operator%d@example.com", nextSeed()), role).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	raw := fmt.Sprintf("it-user-session-%d-%d", nextSeed(), time.Now().UnixNano())
	if _, err := e.pool.Exec(t.Context(), `
		INSERT INTO api_sessions (token, user_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		auth.HashToken(raw), userID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, e.server.URL+"/api/v1"+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: raw})
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return apiResponse{Status: resp.StatusCode, Body: data, Header: resp.Header}
}

// Sedno zmiany: wydane zaświadczenie cytuje podstawę z dnia wystawienia, a poprawka
// w bibliotece dotyczy wyłącznie dokumentów wystawionych później.
func TestLegalBasisIsFrozenAtIssue(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	basis := e.createLegalBasis(t, "§ 22 ust. 3 rozporządzenia z 2019 r.")

	if resp := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": basis.ID}); resp.Status != http.StatusOK {
		t.Fatalf("przypisanie podstawy: %d %s", resp.Status, resp.Body)
	}
	if got := e.getCourseLegalBasis(t, course.ID); got.LegalBasis == nil || got.LegalBasis.ID != basis.ID {
		t.Fatalf("kurs musi pokazywać przypisaną podstawę: %+v", got)
	}

	before := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	if got := e.certificateLegalBasis(t, before.ID); got != basis.Content {
		t.Fatalf("zaświadczenie musi nieść treść podstawy, ma %q", got)
	}

	update := e.callWithSession(t, http.MethodPatch, fmt.Sprintf("/admin/legal-bases/%d", basis.ID), map[string]any{
		"name":    basis.Name,
		"content": "§ 23 ust. 3 rozporządzenia z 2023 r.",
	})
	if update.Status != http.StatusOK {
		t.Fatalf("zmiana treści przez administratora: %d %s", update.Status, update.Body)
	}
	if updated := decodeLegalBasis(t, update); updated.CourseCount != 1 || len(updated.Courses) != 1 || updated.Courses[0].ID != course.ID {
		t.Fatalf("odpowiedź musi pokazywać kursy objęte zmianą: %s", update.Body)
	}

	if got := e.certificateLegalBasis(t, before.ID); got != basis.Content {
		t.Fatalf("wydane zaświadczenie nie może zmienić podstawy po edycji biblioteki, ma %q", got)
	}
	after := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-16")
	if got := e.certificateLegalBasis(t, after.ID); got != "§ 23 ust. 3 rozporządzenia z 2023 r." {
		t.Fatalf("nowe zaświadczenie musi mieć nową treść, ma %q", got)
	}
	if n := e.countRows(t, `SELECT count(*) FROM certificates WHERE id = $1 AND legal_basis_snapshot = $2`, before.ID, basis.Content); n != 1 {
		t.Fatal("treść musi być zapisana w migawce, a nie wyliczana przy odczycie")
	}
}

// PATCH /courses jest pełnym nadpisaniem, ale pominięte legalBasisId zostawia podstawę -
// klient API, który nie zna nowego pola, nie może jej po cichu zdejmować.
func TestCoursePatchLegalBasisIsTriState(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	basis := e.createLegalBasis(t, "§ 16 ust. 3 rozporządzenia z 2004 r.")

	if resp := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": basis.ID}); resp.Status != http.StatusOK {
		t.Fatalf("przypisanie: %d %s", resp.Status, resp.Body)
	}
	if resp := e.patchCourse(t, course.ID, nil); resp.Status != http.StatusOK {
		t.Fatalf("edycja bez pola: %d %s", resp.Status, resp.Body)
	}
	if got := e.getCourseLegalBasis(t, course.ID); got.LegalBasis == nil || got.LegalBasis.ID != basis.ID {
		t.Fatalf("pominięte legalBasisId musi zostawić podstawę: %+v", got)
	}

	if resp := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": nil}); resp.Status != http.StatusOK {
		t.Fatalf("zdjęcie podstawy: %d %s", resp.Status, resp.Body)
	}
	if got := e.getCourseLegalBasis(t, course.ID); got.LegalBasis != nil {
		t.Fatalf("null musi zdjąć podstawę: %+v", got)
	}

	missing := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": 99999999})
	if missing.Status != http.StatusBadRequest || missing.errorMessage(t) != "legal basis not found" {
		t.Fatalf("nieistniejąca podstawa: expected 400 legal basis not found, got %d: %s", missing.Status, missing.Body)
	}
}

func TestLegalBasisDeleteAndPermissions(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	used := e.createLegalBasis(t, "§ 1 podstawa w użyciu")
	unused := e.createLegalBasis(t, "§ 2 podstawa nieużywana")
	if resp := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": used.ID}); resp.Status != http.StatusOK {
		t.Fatalf("przypisanie: %d %s", resp.Status, resp.Body)
	}

	inUse := e.callWithSession(t, http.MethodDelete, fmt.Sprintf("/admin/legal-bases/%d", used.ID), nil)
	if inUse.Status != http.StatusConflict || inUse.errorMessage(t) != "legal basis in use" {
		t.Fatalf("usunięcie używanej: expected 409 legal basis in use, got %d: %s", inUse.Status, inUse.Body)
	}
	if resp := e.callWithSession(t, http.MethodDelete, fmt.Sprintf("/admin/legal-bases/%d", unused.ID), nil); resp.Status != http.StatusNoContent {
		t.Fatalf("usunięcie nieużywanej: expected 204, got %d: %s", resp.Status, resp.Body)
	}

	// Treść zmienia tylko administrator, i tylko w przeglądarce.
	edit := map[string]any{"name": used.Name, "content": "§ 3 próba zmiany"}
	if resp := e.callAsUser(t, 0, http.MethodPatch, fmt.Sprintf("/admin/legal-bases/%d", used.ID), edit); resp.Status != http.StatusForbidden {
		t.Fatalf("zwykły użytkownik: expected 403, got %d: %s", resp.Status, resp.Body)
	}
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/admin/legal-bases/%d", used.ID), edit, nil); resp.Status != http.StatusForbidden {
		t.Fatalf("klucz API: expected 403, got %d: %s", resp.Status, resp.Body)
	}
	// Dodać nową może każdy, kto edytuje kursy.
	if resp := e.callAsUser(t, 0, http.MethodPost, "/legal-bases", map[string]any{
		"name": fmt.Sprintf("Dodana przez operatora %d", nextSeed()), "content": "§ 4",
	}); resp.Status != http.StatusCreated {
		t.Fatalf("dodanie przez zwykłego użytkownika: expected 201, got %d: %s", resp.Status, resp.Body)
	}

	duplicate := e.mustCall(t, http.MethodPost, "/legal-bases", map[string]any{
		"name": "  " + strings.ToUpper(used.Name) + " ", "content": "§ 5",
	}, nil)
	if duplicate.Status != http.StatusConflict {
		t.Fatalf("powtórzona nazwa (inna wielkość liter): expected 409, got %d: %s", duplicate.Status, duplicate.Body)
	}
}

// Przedłużenie przechodzi tą samą ścieżką co wystawienie, więc zamraża aktualną treść.
func TestRenewalFreezesCurrentLegalBasis(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	basis := e.createLegalBasis(t, "§ 18 ust. 2 stara treść")
	if resp := e.patchCourse(t, course.ID, map[string]any{"legalBasisId": basis.ID}); resp.Status != http.StatusOK {
		t.Fatalf("przypisanie: %d %s", resp.Status, resp.Body)
	}
	original := e.issueCertificate(t, course.ID, e.seedStudent(t), "2026-03-15")
	if resp := e.callWithSession(t, http.MethodPatch, fmt.Sprintf("/admin/legal-bases/%d", basis.ID),
		map[string]any{"name": basis.Name, "content": "§ 23 ust. 3 nowa treść"}); resp.Status != http.StatusOK {
		t.Fatalf("zmiana treści: %d %s", resp.Status, resp.Body)
	}

	resp := e.mustCall(t, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", original.ID), renewPayload(), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("renew: %d %s", resp.Status, resp.Body)
	}
	successor := decodeRenewedState(t, resp)
	if got := e.certificateLegalBasis(t, successor.ID); got != "§ 23 ust. 3 nowa treść" {
		t.Fatalf("następca musi cytować aktualną podstawę, ma %q", got)
	}
	if got := e.certificateLegalBasis(t, original.ID); got != "§ 18 ust. 2 stara treść" {
		t.Fatalf("poprzednik zachowuje podstawę z dnia wystawienia, ma %q", got)
	}
}

// Migracja 0030 na szablonach w wariantach spotkanych w bazie produkcyjnej.
func TestLegalBasisMigrationFromTemplates(t *testing.T) {
	e := requireEnv(t)
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0030_legal_bases_from_templates.sql"))
	if err != nil {
		t.Fatal(err)
	}
	bhp := "§ 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r. w sprawie szkolenia w dziedzinie bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.)"
	templates := map[string]string{
		// &nbsp; i podwójne odstępy w środku zdania
		"nbsp": `<p>Zaświadczenie wydano na podstawie § 16 ust. 3 rozporządzenia Ministra&nbsp; Gospodarki i Pracy z dnia 27 lipca 2004 r.&nbsp; w sprawie szkolenia w dziedzinie bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.)</p><p>Nr {{ numer_zaswiadczenia }}</p>`,
		// zdanie złamane na akapity o tym samym formatowaniu
		"split": `<p><span class="a">na podstawie § 16 ust. 3 rozporządzenia Ministra</span></p><p><span class="a">Gospodarki i Pracy z dnia 27 lipca 2004 r. w sprawie szkolenia w dziedzinie</span></p><p><span class="a">bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.)</span></p>`,
		// literówka w roku Dz. U.
		"typo": `<p>na podstawie § 22 ust. 3 rozporządzenia Ministra Edukacji Narodowej z dnia 19 marca 2019 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 20179 poz. 652)</p>`,
		// pogrubienie otwarte w środku zdania i zamknięte za nim - znaczniki się nie bilansują
		"unbalanced": `<p>na podstawie § 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r. w sprawie <strong>szkolenia w dziedzinie bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.) oraz programu</strong></p>`,
		"none":       `<p>ZAŚWIADCZENIE o ukończeniu szkolenia {{ nazwa_kursu }}</p>`,
	}
	ids := map[string]int64{}
	for name, html := range templates {
		course := e.seedCourse(t)
		if _, err := e.pool.Exec(t.Context(), `UPDATE courses SET certfrontpage = $1 WHERE id = $2`, html, course.ID); err != nil {
			t.Fatal(err)
		}
		ids[name] = course.ID
	}

	if _, err := e.pool.Exec(t.Context(), string(script)); err != nil {
		t.Fatalf("migracja 0030: %v", err)
	}

	state := func(name string) (template string, basis string) {
		t.Helper()
		if err := e.pool.QueryRow(t.Context(), `
			SELECT c.certfrontpage, coalesce(lb.content, '')
			FROM courses c LEFT JOIN legal_bases lb ON lb.id = c.legal_basis_id
			WHERE c.id = $1`, ids[name]).Scan(&template, &basis); err != nil {
			t.Fatal(err)
		}
		return template, basis
	}

	if tpl, basis := state("nbsp"); basis != bhp || tpl != `<p>Zaświadczenie wydano na podstawie {{ podstawa_prawna }}</p><p>Nr {{ numer_zaswiadczenia }}</p>` {
		t.Fatalf("wariant z &nbsp;: %q / %q", tpl, basis)
	}
	if tpl, basis := state("split"); basis != bhp || tpl != `<p><span class="a">na podstawie {{ podstawa_prawna }}</span></p>` {
		t.Fatalf("zdanie złamane na akapity musi dać poprawny HTML: %q / %q", tpl, basis)
	}
	if tpl, basis := state("typo"); !strings.Contains(basis, "(Dz. U. z 2019 poz. 652)") || !strings.Contains(tpl, "{{ podstawa_prawna }}") {
		t.Fatalf("literówka musi trafić do poprawionej podstawy: %q / %q", tpl, basis)
	}
	if tpl, basis := state("unbalanced"); basis != bhp || tpl != templates["unbalanced"] {
		t.Fatalf("niezbilansowane znaczniki: przypisanie tak, szablon nietknięty; jest %q / %q", tpl, basis)
	}
	if tpl, basis := state("none"); basis != "" || tpl != templates["none"] {
		t.Fatalf("kurs bez podstawy nie może się zmienić: %q / %q", tpl, basis)
	}
	if n := e.countRows(t, `SELECT count(*) FROM audit_log WHERE entity_type = 'course' AND entity_id = $1 AND metadata->>'operation' = 'legal_basis_migration'`, ids["split"]); n != 1 {
		t.Fatalf("zmiana kursu musi być w historii zmian, wpisów: %d", n)
	}

	// Idempotencja: drugie uruchomienie niczego nie zmienia.
	countAudit := func() int {
		return e.countRows(t, `SELECT count(*) FROM audit_log WHERE metadata->>'operation' = 'legal_basis_migration'`)
	}
	auditBefore := countAudit()
	snapshot := map[string]string{}
	for name := range templates {
		tpl, basis := state(name)
		snapshot[name] = tpl + "|" + basis
	}
	if _, err := e.pool.Exec(t.Context(), string(script)); err != nil {
		t.Fatalf("drugie uruchomienie 0030: %v", err)
	}
	if got := countAudit(); got != auditBefore {
		t.Fatalf("drugie uruchomienie dopisało %d wpisów historii", got-auditBefore)
	}
	for name := range templates {
		tpl, basis := state(name)
		if snapshot[name] != tpl+"|"+basis {
			t.Fatalf("drugie uruchomienie zmieniło %s", name)
		}
	}
}
