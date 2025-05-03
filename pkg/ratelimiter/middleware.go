package ratelimiter

import (
	"net"
	"net/http"
	"strings"


	"go.uber.org/zap"
)

func RateLimitMiddleware(rl *RateLimiter, logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := extractClientIP(r)

			if !rl.Allow(clientID) {
				// Логируем превышение лимита
				logger.Warnw("Rate limit exceeded", "client_ip", clientID)

				// Отправляем ошибку с кодом 429
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
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
