package gusclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeGUSServer struct {
	t             *testing.T
	expectedToken string
	expectedNIP   string
	mode          string
	loginCalls    int
	searchCalls   int
	logoutCalls   int
	loggedIn      bool
}

func newFakeGUSServer(t *testing.T, expectedToken, expectedNIP, mode string) (*fakeGUSServer, *httptest.Server) {
	t.Helper()

	state := &fakeGUSServer{
		t:             t,
		expectedToken: expectedToken,
		expectedNIP:   expectedNIP,
		mode:          mode,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		contentType := r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")

		switch {
		case strings.Contains(contentType, "Zaloguj"):
			state.loginCalls++
			if state.expectedToken == "" || !strings.Contains(string(body), "<ns:pKluczUzytkownika>"+state.expectedToken+"</ns:pKluczUzytkownika>") {
				http.Error(w, "invalid GUS token", http.StatusUnauthorized)
				return
			}
			state.loggedIn = true
			_, _ = w.Write([]byte(soapEnvelope(`<ZalogujResponse><ZalogujResult>session-123</ZalogujResult></ZalogujResponse>`)))
		case strings.Contains(contentType, "Wyloguj"):
			state.logoutCalls++
			state.loggedIn = false
			_, _ = w.Write([]byte(soapEnvelope(`<WylogujResponse><WylogujResult>true</WylogujResult></WylogujResponse>`)))
		case strings.Contains(contentType, "DaneSzukajPodmioty"):
			state.searchCalls++
			if !state.loggedIn {
				http.Error(w, "session is not active", http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("sid"); got != "session-123" {
				http.Error(w, "missing or invalid sid", http.StatusUnauthorized)
				return
			}
			if !strings.Contains(string(body), "<dat:Nip>"+state.expectedNIP+"</dat:Nip>") {
				http.Error(w, "unexpected NIP payload", http.StatusBadRequest)
				return
			}
			if state.mode == "not_found" {
				_, _ = w.Write([]byte(soapEnvelope(`<DaneSzukajPodmiotyResponse><DaneSzukajPodmiotyResult><![CDATA[<root><dane><ErrorCode>4</ErrorCode><ErrorMessagePl>Nie znaleziono podmiotu dla podanych kryteriów wyszukiwania.</ErrorMessagePl><ErrorMessageEn>No data found for the specified search criteria.</ErrorMessageEn><Nip>` + state.expectedNIP + `</Nip></dane></root>]]></DaneSzukajPodmiotyResult></DaneSzukajPodmiotyResponse>`)))
				return
			}
			_, _ = w.Write([]byte(soapEnvelope(`<DaneSzukajPodmiotyResponse><DaneSzukajPodmiotyResult><![CDATA[<root><dane><Regon>123456789</Regon><Nip>` + state.expectedNIP + `</Nip><StatusNip>Czynny</StatusNip><Nazwa>Acme Sp. z o.o.</Nazwa><Wojewodztwo>mazowieckie</Wojewodztwo><Powiat>warszawski</Powiat><Gmina>Centrum</Gmina><Miejscowosc>Warszawa</Miejscowosc><KodPocztowy>00-001</KodPocztowy><Ulica>Prosta</Ulica><NrNieruchomosci>1</NrNieruchomosci><NrLokalu>2</NrLokalu></dane></root>]]></DaneSzukajPodmiotyResult></DaneSzukajPodmiotyResponse>`)))
		default:
			http.Error(w, "unexpected SOAP action", http.StatusBadRequest)
		}
	}))

	return state, server
}

func soapEnvelope(body string) string {
	return `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>` + body + `</s:Body></s:Envelope>`
}

func newTestHandler(serverURL, token string) *Handler {
	return NewHandler(&config.Config{
		GUSUrl:   serverURL,
		GUSToken: token,
	})
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if responseBody.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, responseBody.Error.Code)
	}
}

func TestFindCompanyReturnsBadRequestWhenNIPIsMissing(t *testing.T) {
	handler := newTestHandler("http://example.com", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/lookup-by-nip", nil)
	rec := httptest.NewRecorder()

	handler.FindCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestFindCompanyReturnsBadRequestForInvalidNIPWithoutCallingGUS(t *testing.T) {
	state, server := newFakeGUSServer(t, "secret", "8381771140", "success")
	defer server.Close()

	handler := newTestHandler(server.URL, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/lookup-by-nip?nip=123", nil)
	rec := httptest.NewRecorder()

	handler.FindCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)

	if state.loginCalls != 0 || state.searchCalls != 0 || state.logoutCalls != 0 {
		t.Fatalf("expected no GUS calls for invalid nip, got login=%d search=%d logout=%d", state.loginCalls, state.searchCalls, state.logoutCalls)
	}
}

func TestFindCompanyLooksUpNormalizedNIPAndReturnsCompany(t *testing.T) {
	state, server := newFakeGUSServer(t, "secret", "8381771140", "success")
	defer server.Close()

	handler := newTestHandler(server.URL, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/lookup-by-nip?nip=838-177-11-40", nil)
	rec := httptest.NewRecorder()

	handler.FindCompany(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody GUSCompanyResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.NIP != "8381771140" {
		t.Fatalf("expected normalized NIP in response, got %q", responseBody.Data.NIP)
	}
	if responseBody.Data.Name != "Acme Sp. z o.o." {
		t.Fatalf("unexpected company name: %q", responseBody.Data.Name)
	}
	if responseBody.Data.City != "Warszawa" || responseBody.Data.PostalCode != "00-001" {
		t.Fatalf("unexpected company location payload: %+v", responseBody.Data)
	}
	if state.loginCalls != 1 || state.searchCalls != 1 {
		t.Fatalf("expected login and lookup exactly once, got login=%d search=%d logout=%d", state.loginCalls, state.searchCalls, state.logoutCalls)
	}
}

func TestFindCompanyReturnsNotFoundWhenGUSReturnsBusinessNotFound(t *testing.T) {
	_, server := newFakeGUSServer(t, "secret", "8381771140", "not_found")
	defer server.Close()

	handler := newTestHandler(server.URL, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/lookup-by-nip?nip=8381771140", nil)
	rec := httptest.NewRecorder()

	handler.FindCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestFindCompanyReturnsInternalErrorWhenGUSTokenIsMissing(t *testing.T) {
	state, server := newFakeGUSServer(t, "secret", "8381771140", "success")
	defer server.Close()

	handler := newTestHandler(server.URL, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/lookup-by-nip?nip=8381771140", nil)
	rec := httptest.NewRecorder()

	handler.FindCompany(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)

	if state.loginCalls != 0 || state.searchCalls != 0 || state.logoutCalls != 0 {
		t.Fatalf("expected no GUS calls when token is missing, got login=%d search=%d logout=%d", state.loginCalls, state.searchCalls, state.logoutCalls)
	}
}
