package journals

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

func TestProgramSplitDividesTopicSplitAcrossDays(t *testing.T) {
	program := []byte(`[
		{"Subject": "Wstęp", "TheoryTime": "2", "PracticeTime": "0"},
		{"Subject": "Ćwiczenia", "TheoryTime": "3", "PracticeTime": "2,5"}
	]`)
	sessions := []sqlc.TrainingJournalSession{
		{ID: 1, SortOrder: 1, Hours: numeric(t, "2")},
		{ID: 2, SortOrder: 2, Hours: numeric(t, "4")},
		{ID: 3, SortOrder: 2, Hours: numeric(t, "1.5")},
	}

	got := buildJournalProgramSplitBySession(program, sessions)

	want := map[int64]journalProgramSplit{
		1: {Theory: "2", Practice: "0"},
		2: {Theory: "3", Practice: "1"},
		3: {Theory: "0", Practice: "1.5"},
	}
	for id, split := range want {
		if got[id] != split {
			t.Errorf("session %d: got %+v, want %+v", id, got[id], split)
		}
	}
}

func numeric(t *testing.T, value string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(value); err != nil {
		t.Fatal(err)
	}
	return n
}
