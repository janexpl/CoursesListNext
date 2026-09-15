package webhooks

import (
	"errors"
	"net/http"
	"testing"
)

func TestSignIsLowercaseHexHMACOfRawBody(t *testing.T) {
	// HMAC-SHA256("key", "The quick brown fox jumps over the lazy dog") - wektor z RFC/Wikipedii.
	got := Sign("key", []byte("The quick brown fox jumps over the lazy dog"))
	want := "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	if got != want {
		t.Fatalf("Sign = %s, want %s", got, want)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		status int
		err    error
		want   bool
	}{
		{0, errors.New("timeout"), true},
		{http.StatusInternalServerError, nil, true},
		{http.StatusServiceUnavailable, nil, true},
		{http.StatusBadRequest, nil, false},
		{http.StatusUnauthorized, nil, false},
		{http.StatusUnprocessableEntity, nil, false},
		{http.StatusNotFound, nil, false},
		{http.StatusFound, nil, false},
	}
	for _, c := range cases {
		if got := isRetryable(c.status, c.err); got != c.want {
			t.Errorf("isRetryable(%d, %v) = %v, want %v", c.status, c.err, got, c.want)
		}
	}
}

func TestDefaultConfigRetriesAtLeastFiveTimesWithinHours(t *testing.T) {
	cfg := DefaultConfig()
	attempts := len(cfg.RetryDelays) + 1
	var total float64
	for _, d := range cfg.RetryDelays {
		total += d.Hours()
	}
	if attempts < 5 || total > 12 {
		t.Fatalf("expected >= 5 attempts within hours, got %d attempts over %.2f h", attempts, total)
	}
}
