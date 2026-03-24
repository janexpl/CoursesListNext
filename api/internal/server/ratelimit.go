package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64
	burst    int
}

type visitor struct {
	tokens    float64
	lastSeen  time.Time
	lastRefil time.Time
}

func newIPLimiter(ratePerSecond float64, burst int) *ipLimiter {
	rl := &ipLimiter{
		visitors: make(map[string]*visitor),
		rate:     ratePerSecond,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *ipLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists {
		rl.visitors[ip] = &visitor{
			tokens:    float64(rl.burst) - 1,
			lastSeen:  now,
			lastRefil: now,
		}
		return true
	}

	v.lastSeen = now
	elapsed := now.Sub(v.lastRefil).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}
	v.lastRefil = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (rl *ipLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func RateLimitByIP(limiter *ipLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow(r.RemoteAddr) {
				response.WriteError(w, http.StatusTooManyRequests, "too_many_requests", "too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
