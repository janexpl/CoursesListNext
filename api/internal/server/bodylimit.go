package server

import (
	"net/http"
	"strings"
)

// maxJSONBodyBytes bounds the size of JSON request bodies so a malicious or
// buggy client cannot force the server to buffer an unbounded payload.
const maxJSONBodyBytes int64 = 1 << 20 // 1 MiB

// limitRequestBody wraps the request body with http.MaxBytesReader for every
// route except the multipart upload endpoints, which enforce their own larger
// limit inside the handler.
func limitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && !isUploadRequest(r) {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isUploadRequest reports whether the request targets a multipart file upload
// endpoint, which manages its own size limit.
func isUploadRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	return strings.HasSuffix(r.URL.Path, "/attendance-scan") ||
		strings.HasSuffix(r.URL.Path, "/signed-scan")
}
