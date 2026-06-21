package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBodyCapsJSONBodies(t *testing.T) {
	var readErr error
	handler := limitRequestBody(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader(strings.Repeat("a", 100)))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if readErr == nil {
		t.Fatal("expected reading an over-limit body to fail")
	}
}

func TestLimitRequestBodyAllowsBodyWithinLimit(t *testing.T) {
	var read []byte
	var readErr error
	handler := limitRequestBody(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/certificates", strings.NewReader(strings.Repeat("a", 100)))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if readErr != nil {
		t.Fatalf("unexpected error reading within-limit body: %v", readErr)
	}
	if len(read) != 100 {
		t.Fatalf("expected to read 100 bytes, got %d", len(read))
	}
}

func TestLimitRequestBodyExemptsUploadRoutes(t *testing.T) {
	for _, path := range []string{"/api/v1/journals/5/attendance-scan", "/api/v1/journals/5/signed-scan"} {
		t.Run(path, func(t *testing.T) {
			var read []byte
			var readErr error
			handler := limitRequestBody(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				read, readErr = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("a", 100)))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			if readErr != nil {
				t.Fatalf("upload route must not be capped, got error: %v", readErr)
			}
			if len(read) != 100 {
				t.Fatalf("expected full 100 bytes on upload route, got %d", len(read))
			}
		})
	}
}

func TestIsUploadRequestOnlyMatchesUploadPosts(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/v1/journals/5/attendance-scan", true},
		{http.MethodPost, "/api/v1/journals/5/signed-scan", true},
		{http.MethodGet, "/api/v1/journals/5/attendance-scan", false},
		{http.MethodDelete, "/api/v1/journals/5/signed-scan", false},
		{http.MethodGet, "/api/v1/journals/5/attendance-scan/meta", false},
		{http.MethodPost, "/api/v1/certificates", false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := isUploadRequest(req); got != tc.want {
			t.Fatalf("isUploadRequest(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
