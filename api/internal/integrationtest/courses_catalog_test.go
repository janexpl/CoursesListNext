package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

// Zlecenie, punkt 8: katalog kursów - updatedSince, paginacja, deliveredByPlatform.

type courseListPage struct {
	Data []struct {
		ID                  int64  `json:"id"`
		Symbol              string `json:"symbol"`
		DeliveredByPlatform *bool  `json:"deliveredByPlatform"`
	} `json:"data"`
	Pagination *struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
	} `json:"pagination"`
}

func (p courseListPage) ids() []int64 {
	ids := make([]int64, 0, len(p.Data))
	for _, c := range p.Data {
		ids = append(ids, c.ID)
	}
	return ids
}

func (e *testEnv) listCourses(t *testing.T, path string, query url.Values) courseListPage {
	t.Helper()
	resp := e.mustCall(t, http.MethodGet, path+"?"+query.Encode(), nil, nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("GET %s?%s: expected 200, got %d: %s", path, query.Encode(), resp.Status, resp.Body)
	}
	var page courseListPage
	resp.decode(t, &page)
	return page
}

func (e *testEnv) createCourseViaAPI(t *testing.T, symbol string, translations []map[string]any) int64 {
	t.Helper()
	body := courseWritePayload(symbol, translations)
	resp := e.mustCall(t, http.MethodPost, "/courses", body, nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("POST /courses: expected 201, got %d: %s", resp.Status, resp.Body)
	}
	return decodeCreated(t, resp).ID
}

func courseWritePayload(symbol string, translations []map[string]any) map[string]any {
	body := map[string]any{
		"mainName":      "Szkolenie",
		"name":          "Kurs " + symbol,
		"symbol":        symbol,
		"expiryTime":    5,
		"courseProgram": `[{"Subject":"Temat","TheoryTime":"1","PracticeTime":"1"}]`,
		"certFrontPage": "<p>front</p>",
	}
	if translations != nil {
		body["certificateTranslations"] = translations
	}
	return body
}

func englishTranslation(name string) map[string]any {
	return map[string]any{
		"languageCode":  "en",
		"courseName":    name,
		"courseProgram": `[{"Subject":"Topic","TheoryTime":"1","PracticeTime":"1"}]`,
		"certFrontPage": "<p>front en</p>",
	}
}

// dbNow bierze czas z bazy - ten sam zegar, który ustawia updated_at.
func (e *testEnv) dbNow(t *testing.T) string {
	t.Helper()
	var now time.Time
	if err := e.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	return now.UTC().Format(time.RFC3339Nano)
}

func TestUpdatedSinceReturnsCourseAfterTranslationOnlyChange(t *testing.T) {
	e := requireEnv(t)
	n := nextSeed()
	changed := e.createCourseViaAPI(t, fmt.Sprintf("UPD%dA", n), []map[string]any{englishTranslation("Course A")})
	untouched := e.createCourseViaAPI(t, fmt.Sprintf("UPD%dB", n), []map[string]any{englishTranslation("Course B")})

	since := e.dbNow(t)
	for _, path := range []string{"/courses", "/courses/details"} {
		page := e.listCourses(t, path, url.Values{"updatedSince": {since}, "limit": {"100"}})
		if slices.Contains(page.ids(), changed) || slices.Contains(page.ids(), untouched) {
			t.Fatalf("%s: no course changed yet, got %v", path, page.ids())
		}
	}

	// PATCH z identyczną treścią kursu i zmienioną wyłącznie nazwą tłumaczenia.
	patch := courseWritePayload(fmt.Sprintf("UPD%dA", n), []map[string]any{englishTranslation("Course A (renamed)")})
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/courses/%d", changed), patch, nil); resp.Status != http.StatusOK {
		t.Fatalf("PATCH course: expected 200, got %d: %s", resp.Status, resp.Body)
	}

	for _, path := range []string{"/courses", "/courses/details"} {
		page := e.listCourses(t, path, url.Values{"updatedSince": {since}, "limit": {"100"}})
		if !slices.Contains(page.ids(), changed) {
			t.Fatalf("%s: course with changed translation missing from updatedSince result %v", path, page.ids())
		}
		if slices.Contains(page.ids(), untouched) {
			t.Fatalf("%s: untouched course must not be returned, got %v", path, page.ids())
		}
	}

	// Zapis bez żadnej zmiany treści nie podnosi znacznika.
	since = e.dbNow(t)
	untouchedPatch := courseWritePayload(fmt.Sprintf("UPD%dB", n), []map[string]any{englishTranslation("Course B")})
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/courses/%d", untouched), untouchedPatch, nil); resp.Status != http.StatusOK {
		t.Fatalf("PATCH course: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	if page := e.listCourses(t, "/courses", url.Values{"updatedSince": {since}, "limit": {"100"}}); slices.Contains(page.ids(), untouched) {
		t.Fatalf("PATCH without changes must not bump updated_at, got %v", page.ids())
	}

	// Usunięcie tłumaczenia to też zmiana kursu.
	since = e.dbNow(t)
	if resp := e.mustCall(t, http.MethodPatch, fmt.Sprintf("/courses/%d", untouched), courseWritePayload(fmt.Sprintf("UPD%dB", n), []map[string]any{}), nil); resp.Status != http.StatusOK {
		t.Fatalf("PATCH course: expected 200, got %d: %s", resp.Status, resp.Body)
	}
	if page := e.listCourses(t, "/courses/details", url.Values{"updatedSince": {since}, "limit": {"100"}}); !slices.Contains(page.ids(), untouched) {
		t.Fatalf("removing a translation must bump updated_at, got %v", page.ids())
	}
}

func TestUpdatedSinceValidation(t *testing.T) {
	e := requireEnv(t)
	for _, value := range []string{"yesterday", "2026-03-10", "2026-03-10T10:00:00"} {
		for _, path := range []string{"/courses", "/courses/details"} {
			resp := e.mustCall(t, http.MethodGet, path+"?updatedSince="+url.QueryEscape(value), nil, nil)
			if resp.Status != http.StatusBadRequest || resp.errorMessage(t) != "invalid updatedSince value" {
				t.Fatalf("%s updatedSince=%q: expected 400 invalid updatedSince value, got %d: %s", path, value, resp.Status, resp.Body)
			}
		}
	}
	for _, value := range []string{"2026-03-10T10:00:00Z", "2026-03-10T10:00:00.123456+02:00"} {
		if resp := e.mustCall(t, http.MethodGet, "/courses?updatedSince="+url.QueryEscape(value), nil, nil); resp.Status != http.StatusOK {
			t.Fatalf("updatedSince=%q: expected 200, got %d: %s", value, resp.Status, resp.Body)
		}
	}
}

func TestCourseListsArePaginated(t *testing.T) {
	e := requireEnv(t)
	prefix := fmt.Sprintf("PGN%dX", nextSeed())
	var created []int64
	for i := range 5 {
		created = append(created, e.createCourseViaAPI(t, fmt.Sprintf("%s%d", prefix, i), nil))
	}

	for _, path := range []string{"/courses", "/courses/details"} {
		var seen []int64
		for pageNumber := 1; pageNumber <= 3; pageNumber++ {
			page := e.listCourses(t, path, url.Values{"search": {prefix}, "limit": {"2"}, "page": {fmt.Sprint(pageNumber)}})
			if page.Pagination == nil {
				t.Fatalf("%s: missing pagination envelope", path)
			}
			p := *page.Pagination
			if p.Page != pageNumber || p.Limit != 2 || p.Total != 5 || p.TotalPages != 3 {
				t.Fatalf("%s page %d: unexpected pagination %+v", path, pageNumber, p)
			}
			wantLen := 2
			if pageNumber == 3 {
				wantLen = 1
			}
			if len(page.Data) != wantLen {
				t.Fatalf("%s page %d: expected %d items, got %d", path, pageNumber, wantLen, len(page.Data))
			}
			seen = append(seen, page.ids()...)
		}
		slices.Sort(seen)
		want := slices.Clone(created)
		slices.Sort(want)
		if !slices.Equal(seen, want) {
			t.Fatalf("%s: pages must cover every course exactly once, got %v want %v", path, seen, want)
		}

		beyond := e.listCourses(t, path, url.Values{"search": {prefix}, "limit": {"2"}, "page": {"9"}})
		if len(beyond.Data) != 0 || beyond.Pagination == nil || beyond.Pagination.Total != 5 {
			t.Fatalf("%s: page beyond range must be empty with the real total, got %+v", path, beyond)
		}

		// Bez page: pierwsza strona i dotychczasowy domyślny limit 50.
		first := e.listCourses(t, path, url.Values{"search": {prefix}})
		if first.Pagination == nil || first.Pagination.Page != 1 || first.Pagination.Limit != 50 || len(first.Data) != 5 {
			t.Fatalf("%s: default page must be 1 with limit 50, got %+v", path, first.Pagination)
		}

		for _, bad := range []string{"0", "-1", "abc"} {
			resp := e.mustCall(t, http.MethodGet, path+"?page="+bad, nil, nil)
			if resp.Status != http.StatusBadRequest || resp.errorMessage(t) != "invalid page value" {
				t.Fatalf("%s page=%s: expected 400 invalid page value, got %d: %s", path, bad, resp.Status, resp.Body)
			}
		}
	}
}

func TestDeliveredByPlatformFlagAndFilter(t *testing.T) {
	e := requireEnv(t)
	prefix := fmt.Sprintf("DLV%dX", nextSeed())
	remote := e.createCourseViaAPI(t, prefix+"R", nil)
	practical := e.createCourseViaAPI(t, prefix+"P", nil)
	flagPath := fmt.Sprintf("/courses/%d/platform-delivery", remote)

	get := e.mustCall(t, http.MethodGet, flagPath, nil, nil)
	if get.Status != http.StatusOK || !jsonEquals(t, get.Body, `{"data":{"deliveredByPlatform":false}}`) {
		t.Fatalf("new course must default to false, got %d: %s", get.Status, get.Body)
	}

	since := e.dbNow(t)
	put := e.mustCall(t, http.MethodPut, flagPath, map[string]any{"deliveredByPlatform": true}, nil)
	if put.Status != http.StatusOK || !jsonEquals(t, put.Body, `{"data":{"deliveredByPlatform":true}}`) {
		t.Fatalf("PUT flag: expected 200 with true, got %d: %s", put.Status, put.Body)
	}
	if again := e.mustCall(t, http.MethodGet, flagPath, nil, nil); !jsonEquals(t, again.Body, `{"data":{"deliveredByPlatform":true}}`) {
		t.Fatalf("flag not persisted: %s", again.Body)
	}

	// Zmiana flagi zmienia to, czy kurs ma być w katalogu platformy - musi być widoczna w updatedSince.
	if page := e.listCourses(t, "/courses/details", url.Values{"updatedSince": {since}, "limit": {"100"}}); !slices.Contains(page.ids(), remote) {
		t.Fatalf("flag change must bump updated_at, got %v", page.ids())
	}

	for _, path := range []string{"/courses", "/courses/details"} {
		onlyPlatform := e.listCourses(t, path, url.Values{"search": {prefix}, "deliveredByPlatform": {"true"}})
		if !slices.Equal(onlyPlatform.ids(), []int64{remote}) {
			t.Fatalf("%s deliveredByPlatform=true: expected [%d], got %v", path, remote, onlyPlatform.ids())
		}
		notPlatform := e.listCourses(t, path, url.Values{"search": {prefix}, "deliveredByPlatform": {"false"}})
		if !slices.Equal(notPlatform.ids(), []int64{practical}) {
			t.Fatalf("%s deliveredByPlatform=false: expected [%d], got %v", path, practical, notPlatform.ids())
		}
		all := e.listCourses(t, path, url.Values{"search": {prefix}})
		if len(all.Data) != 2 {
			t.Fatalf("%s without filter: expected both courses, got %v", path, all.ids())
		}
		resp := e.mustCall(t, http.MethodGet, path+"?deliveredByPlatform=yes", nil, nil)
		if resp.Status != http.StatusBadRequest || resp.errorMessage(t) != "invalid deliveredByPlatform value" {
			t.Fatalf("%s: expected 400 invalid deliveredByPlatform value, got %d: %s", path, resp.Status, resp.Body)
		}
	}

	list := e.listCourses(t, "/courses", url.Values{"search": {prefix + "R"}})
	if len(list.Data) != 1 || list.Data[0].DeliveredByPlatform == nil || !*list.Data[0].DeliveredByPlatform {
		t.Fatalf("GET /courses items must expose deliveredByPlatform, got %s", mustJSON(list))
	}

	// Kształt CourseDetails pozostaje bez zmian.
	details := e.mustCall(t, http.MethodGet, fmt.Sprintf("/courses/%d", remote), nil, nil)
	var detailsBody struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	details.decode(t, &detailsBody)
	wantKeys := []string{"certFrontPage", "certificateTranslations", "courseProgram", "expiryTime", "id", "mainName", "name", "symbol"}
	var gotKeys []string
	for key := range detailsBody.Data {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("CourseDetails shape changed: got keys %v, want %v", gotKeys, wantKeys)
	}

	for _, body := range []any{map[string]any{}, map[string]any{"deliveredByPlatform": true, "extra": 1}, map[string]any{"deliveredByPlatform": "true"}} {
		if resp := e.mustCall(t, http.MethodPut, flagPath, body, nil); resp.Status != http.StatusBadRequest {
			t.Fatalf("PUT %v: expected 400, got %d: %s", body, resp.Status, resp.Body)
		}
	}
	if resp := e.mustCall(t, http.MethodPut, "/courses/999999999/platform-delivery", map[string]any{"deliveredByPlatform": true}, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown course: expected 404, got %d: %s", resp.Status, resp.Body)
	}
	readOnly := e.seedScopedAPIKey(t, "courses:read")
	if resp := e.callWithKey(t, readOnly, http.MethodPut, flagPath, map[string]any{"deliveredByPlatform": false}); resp.Status != http.StatusForbidden {
		t.Fatalf("PUT without courses:write: expected 403, got %d: %s", resp.Status, resp.Body)
	}
	if resp := e.callWithKey(t, readOnly, http.MethodGet, flagPath, nil); resp.Status != http.StatusOK {
		t.Fatalf("GET with courses:read: expected 200, got %d: %s", resp.Status, resp.Body)
	}
}

func jsonEquals(t *testing.T, got []byte, want string) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatal(err)
	}
	return mustJSON(a) == mustJSON(b)
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
