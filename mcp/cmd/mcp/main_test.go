package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type recordedRequest struct {
	Method, Path, Query, Auth, IdempotencyKey string
	Body                                      map[string]any
}

// fakeAPI odpowiada z mapy "METODA ścieżka" -> (status, treść) i zapisuje żądania.
type fakeAPI struct {
	mu        sync.Mutex
	responses map[string]string
	status    map[string]int
	requests  []recordedRequest
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	key := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/v1")

	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
		Auth: r.Header.Get("Authorization"), IdempotencyKey: r.Header.Get("Idempotency-Key"), Body: body,
	})
	response, ok := f.responses[key]
	status := f.status[key]
	f.mu.Unlock()

	if !ok {
		http.Error(w, `{"error":{"code":"not_found","message":"no route"}}`, http.StatusNotFound)
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(response))
}

func connect(t *testing.T, api *fakeAPI) *mcp.ClientSession {
	t.Helper()
	httpServer := httptest.NewServer(api)
	t.Cleanup(httpServer.Close)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := newServer(newAPIClient(httpServer.URL+"/", "clk_test")).Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return result.Content[0].(*mcp.TextContent).Text, result.IsError
}

func TestReadToolCallsAPIWithKeyAndReturnsItsJSON(t *testing.T) {
	api := &fakeAPI{responses: map[string]string{"GET /students": `{"data":[{"id":7}]}`}}
	session := connect(t, api)

	text, isError := call(t, session, "search_students", map[string]any{"search": "Kowalski", "limit": 5})

	if isError || text != `{"data":[{"id":7}]}` {
		t.Fatalf("got %q (error %v)", text, isError)
	}
	got := api.requests[0]
	if got.Path != "/api/v1/students" || got.Query != "limit=5&search=Kowalski" || got.Auth != "Bearer clk_test" {
		t.Fatalf("unexpected request %+v", got)
	}
}

func TestAPIErrorReachesAgentAsToolError(t *testing.T) {
	api := &fakeAPI{
		responses: map[string]string{"POST /journals": `{"error":{"code":"bad_request","message":"course program does not fit within journal dates"}}`},
		status:    map[string]int{"POST /journals": http.StatusBadRequest},
	}
	session := connect(t, api)

	text, isError := call(t, session, "create_journal", map[string]any{
		"courseId": 1, "title": "D", "organizerName": "O", "location": "L",
		"formOfTraining": "stacjonarna", "dateStart": "2026-03-10", "dateEnd": "2026-03-10",
	})

	if !isError || !strings.Contains(text, "course program does not fit within journal dates") {
		t.Fatalf("expected the API message as a tool error, got %q (error %v)", text, isError)
	}
	if _, sent := api.requests[0].Body["legalBasis"]; sent {
		t.Fatal("an omitted legalBasis must stay omitted so the API takes the course's one")
	}
}

func TestIssueCertificateRetryReusesIdempotencyKey(t *testing.T) {
	api := &fakeAPI{
		responses: map[string]string{"POST /certificates": `{"data":{"id":9}}`},
		status:    map[string]int{"POST /certificates": http.StatusCreated},
	}
	session := connect(t, api)
	args := map[string]any{"studentId": 3, "courseId": 4, "certificateDate": "2026-03-12", "courseDateStart": "2026-03-10"}

	call(t, session, "issue_certificate", args)
	call(t, session, "issue_certificate", args)
	args["studentId"] = 5
	call(t, session, "issue_certificate", args)

	first, retry, other := api.requests[0].IdempotencyKey, api.requests[1].IdempotencyKey, api.requests[2].IdempotencyKey
	if first == "" || first != retry || first == other {
		t.Fatalf("keys: first %q, retry %q, other student %q", first, retry, other)
	}
}

func TestUpdateStudentKeepsFieldsTheAgentDidNotSend(t *testing.T) {
	api := &fakeAPI{responses: map[string]string{
		"GET /students/7": `{"data":{"id":7,"firstName":"Jan","lastName":"Nowak","birthDate":"1990-01-10",
			"birthPlace":"Kraków","pesel":"90011012345","telephone":"600100200","company":{"id":3,"name":"ACME"}}}`,
		"PATCH /students/7": `{"data":{"id":7}}`,
	}}
	session := connect(t, api)

	if text, isError := call(t, session, "update_student", map[string]any{"id": 7, "lastName": "Kowalska"}); isError {
		t.Fatal(text)
	}

	body := api.requests[1].Body
	want := map[string]any{
		"firstName": "Jan", "lastName": "Kowalska", "birthDate": "1990-01-10", "birthPlace": "Kraków",
		"pesel": "90011012345", "telephone": "600100200", "companyId": float64(3),
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("%s: got %v, want %v (body %v)", key, body[key], value, body)
		}
	}
	if _, unknown := body["company"]; unknown {
		t.Error("PATCH must not carry response-only fields; the API rejects unknown fields")
	}
}
