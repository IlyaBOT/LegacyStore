package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	Count     int
	ResetTime time.Time
}

type authRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	limit   int
	window  time.Duration
}

func newAuthRateLimiter(limit int, window time.Duration) *authRateLimiter {
	return &authRateLimiter{
		buckets: make(map[string]rateBucket),
		limit:   limit,
		window:  window,
	}
}

func (l *authRateLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[key]
	if !ok || !now.Before(bucket.ResetTime) {
		l.buckets[key] = rateBucket{Count: 1, ResetTime: now.Add(l.window)}
		return true
	}
	if bucket.Count >= l.limit {
		return false
	}
	bucket.Count++
	l.buckets[key] = bucket

	if len(l.buckets) > 4096 {
		for bucketKey, value := range l.buckets {
			if !now.Before(value.ResetTime) {
				delete(l.buckets, bucketKey)
			}
		}
	}
	return true
}

var loginRateLimiter = newAuthRateLimiter(20, 5*time.Minute)

// SecurityMiddleware adds baseline browser/API security headers, rejects obvious
// cross-origin cookie-authenticated state changes, and applies a small in-memory
// rate limit to credential endpoints. A distributed deployment should replace
// the in-memory limiter with a shared limiter at the reverse proxy or datastore.
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if isSecureRequest(req) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		if isCredentialEndpoint(req) {
			key := rateLimitKey(req)
			if !loginRateLimiter.Allow(key, time.Now()) {
				w.Header().Set("Retry-After", "300")
				writeError(w, http.StatusTooManyRequests, "rate_limited")
				return
			}
		}

		if usesCookieAuthentication(req) && isStateChangingMethod(req.Method) && !sameOriginRequest(req) {
			writeError(w, http.StatusForbidden, "csrf_origin_rejected")
			return
		}

		next.ServeHTTP(w, req)
	})
}

func isCredentialEndpoint(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}
	switch req.URL.Path {
	case "/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/legacy/login",
		"/api/v1/auth/2fa/verify":
		return true
	default:
		return false
	}
}

func rateLimitKey(req *http.Request) string {
	ip := clientIP(req)
	if ip == nil {
		return req.URL.Path + "|unknown"
	}
	return req.URL.Path + "|" + ip.String()
}

func usesCookieAuthentication(req *http.Request) bool {
	if strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
		return false
	}
	_, err := req.Cookie("legacystore_session")
	return err == nil
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func sameOriginRequest(req *http.Request) bool {
	origin := strings.TrimSpace(req.Header.Get("Origin"))
	if origin != "" && origin != "null" {
		return originMatchesHost(origin, req.Host)
	}

	referer := strings.TrimSpace(req.Header.Get("Referer"))
	if referer != "" {
		return originMatchesHost(referer, req.Host)
	}

	// Non-browser clients usually omit Origin and Referer. SameSite cookies and
	// Bearer tokens remain the primary protection for these requests.
	return true
}

func originMatchesHost(rawURL, requestHost string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	return equalHostPort(parsed.Host, requestHost)
}

func equalHostPort(a, b string) bool {
	aHost, aPort := splitHostPortLoose(a)
	bHost, bPort := splitHostPortLoose(b)
	if !strings.EqualFold(aHost, bHost) {
		return false
	}
	if aPort == "" || bPort == "" {
		return true
	}
	return aPort == bPort
}

func splitHostPortLoose(value string) (string, string) {
	if host, port, err := net.SplitHostPort(value); err == nil {
		return host, port
	}
	return strings.Trim(value, "[]"), ""
}
