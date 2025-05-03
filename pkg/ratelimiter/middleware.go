package ratelimiter

import (
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

func RateLimitMiddleware(rl *RateLimiter, logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := extractClientIP(r)

			allowed := rl.Allow(clientID)
			if !allowed {
				logger.Warnw("Rate limit exceeded",
					"client_ip", clientID,
					"method", r.Method,
					"url", r.URL.String(),
					"user_agent", r.UserAgent(),
					"time", time.Now().Format(time.RFC3339),
				)

				w.Header().Set("Retry-After", "10") // опционально
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return strings.TrimSpace(ip)
}
