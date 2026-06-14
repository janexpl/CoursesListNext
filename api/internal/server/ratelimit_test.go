package server

import (
	"net/http"
	"testing"
)

func TestClientIPStripsPort(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "ipv4 with port", remoteAddr: "203.0.113.5:54321", want: "203.0.113.5"},
		{name: "ipv6 with port", remoteAddr: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "missing port falls back to raw", remoteAddr: "203.0.113.5", want: "203.0.113.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: tc.remoteAddr}
			if got := clientIP(r); got != tc.want {
				t.Fatalf("clientIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}

func TestClientIPGroupsConnectionsFromSameHost(t *testing.T) {
	// Different ephemeral ports from the same source must map to one limiter key,
	// otherwise every connection gets a fresh bucket and the limit never applies.
	first := clientIP(&http.Request{RemoteAddr: "203.0.113.5:40001"})
	second := clientIP(&http.Request{RemoteAddr: "203.0.113.5:55002"})
	if first != second {
		t.Fatalf("expected same key for same host, got %q and %q", first, second)
	}
}

func TestIPLimiterBlocksAfterBurstExhausted(t *testing.T) {
	limiter := newIPLimiter(0.001, 3)

	for i := 0; i < 3; i++ {
		if !limiter.allow("203.0.113.5") {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}
	if limiter.allow("203.0.113.5") {
		t.Fatal("request beyond burst should be rejected")
	}
	if !limiter.allow("198.51.100.7") {
		t.Fatal("a different host must have its own bucket")
	}
}
