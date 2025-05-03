package proxy

import (
    "context"
    "encoding/json"
    "net"
    "net/http"
    "net/http/httputil"
    "strings"
    "time"

    "github.com/Manzo48/loadBalancer/pkg/balancer"
    "github.com/Manzo48/loadBalancer/pkg/config"
    "github.com/Manzo48/loadBalancer/pkg/ratelimiter"
    "go.uber.org/zap"
)

type errorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(errorResponse{Code: code, Message: msg})
}

type LoadBalancer struct {
    balancer    *balancer.RoundRobinBalancer
    logger      *zap.SugaredLogger
    server      *http.Server
    rateLimiter *ratelimiter.RateLimiter
}

func NewLoadBalancer(cfg *config.Config, logger *zap.SugaredLogger) *LoadBalancer {
    rr := balancer.NewRoundRobin(cfg.Backends, logger)
    rl := ratelimiter.NewRateLimiter(cfg.RateLimit.Capacity, cfg.RateLimit.RefillRate, logger)

    lb := &LoadBalancer{
        balancer:    rr,
        logger:      logger,
        rateLimiter: rl,
    }
    logger.Infof("Initialized LoadBalancer on :%d with %d backends and rate limit %d/%ds",
        cfg.Port, len(cfg.Backends), cfg.RateLimit.Capacity, cfg.RateLimit.RefillRate)

    // очистка неактивных бакетов
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            lb.logger.Debug("running rate limiter cleanup")
            lb.rateLimiter.Cleanup(5 * time.Minute)
        }
    }()

    return lb
}

func (lb *LoadBalancer) ListenAndServe(addr string) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/", lb.handle)

    handler := ratelimiter.RateLimitMiddleware(lb.rateLimiter, lb.logger)(mux)
    lb.server = &http.Server{Addr: addr, Handler: handler}

    lb.logger.Infof("starting HTTP server on %s", addr)
    return lb.server.ListenAndServe()
}

func (lb *LoadBalancer) Shutdown() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    lb.logger.Info("shutting down HTTP server...")
    if err := lb.server.Shutdown(ctx); err != nil {
        lb.logger.Errorf("graceful shutdown failed: %v", err)
    } else {
        lb.logger.Info("shutdown complete")
    }
}

func (lb *LoadBalancer) handle(w http.ResponseWriter, r *http.Request) {
    clientIP := extractClientIP(r)
    // rate limiter уже сработал в middleware — здесь только проксируем

    backend := lb.balancer.NextBackend()
    if backend == nil {
        lb.logger.Warn("no available backends")
        writeJSONError(w, http.StatusServiceUnavailable, "no available backends")
        return
    }

    proxy := httputil.NewSingleHostReverseProxy(backend.URL)
    originalDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        originalDirector(req)
        req.Host = backend.URL.Host
    }

    proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
        lb.logger.Errorf("proxy error for backend %s: %v", backend.URL, err)
        lb.balancer.MarkBackendDead(backend.URL)
        writeJSONError(rw, http.StatusServiceUnavailable, "backend unavailable")
    }

    lb.logger.Infof("forwarding %s → %s", clientIP, backend.URL)
    proxy.ServeHTTP(w, r)
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
