package integrationtest

import (
	"context"
	"net/http"
	"testing"
)

// Dzień szkolenia to maks. 10 godzin lekcyjnych; temat przekraczający granicę dnia
// dzieli się na części zamiast przesuwać się w całości na następny dzień.
func TestJournalSplitsTopicAcrossDays(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	if _, err := e.pool.Exec(context.Background(), `UPDATE courses SET courseprogram = $2 WHERE id = $1`, course.ID, `[
		{"Subject": "Wstęp", "TheoryTime": "4", "PracticeTime": "0"},
		{"Subject": "Ćwiczenia", "TheoryTime": "4", "PracticeTime": "3"},
		{"Subject": "Bez godzin", "TheoryTime": "", "PracticeTime": ""},
		{"Subject": "Pierwsza pomoc", "TheoryTime": "2", "PracticeTime": "3"},
		{"Subject": "Egzamin", "TheoryTime": "4", "PracticeTime": "0"}
	]`); err != nil {
		t.Fatal(err)
	}

	resp := e.mustCall(t, http.MethodPost, "/journals", map[string]any{
		"courseId":       course.ID,
		"title":          "Dziennik dwudniowy",
		"organizerName":  "Jan Kowalski",
		"location":       "Warszawa",
		"formOfTraining": "stacjonarna",
		"legalBasis":     "§ 1",
		"dateStart":      "2026-03-10",
		"dateEnd":        "2026-03-11",
	}, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("POST /journals: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	journalID := decodeCreated(t, resp).ID

	rows, err := e.pool.Query(context.Background(), `
		SELECT to_char(session_date, 'YYYY-MM-DD'), topic, hours::text, sort_order
		FROM training_journal_sessions WHERE journal_id = $1 ORDER BY sort_order, id`, journalID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type session struct {
		date, topic, hours string
		sortOrder          int32
	}
	var got []session
	for rows.Next() {
		var s session
		if err := rows.Scan(&s.date, &s.topic, &s.hours, &s.sortOrder); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []session{
		{"2026-03-10", "Wstęp", "4.00", 1},
		{"2026-03-10", "Ćwiczenia (cz. 1/2)", "6.00", 2},
		{"2026-03-11", "Ćwiczenia (cz. 2/2)", "1.00", 2},
		{"2026-03-11", "Pierwsza pomoc", "5.00", 4},
		{"2026-03-11", "Egzamin", "4.00", 5},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d sessions, got %+v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("session %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
