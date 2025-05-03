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

// Структура для представления ошибки в формате JSON
type errorResponse struct {
    Code    int    `json:"code"`    // HTTP статус-код
    Message string `json:"message"` // Сообщение об ошибке
}

// writeJSONError — утилита для отправки JSON-ответа с ошибкой
func writeJSONError(w http.ResponseWriter, code int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(errorResponse{Code: code, Message: msg})
}

// LoadBalancer — основной тип, реализующий поведение прокси-сервера с балансировкой нагрузки и rate limiting
type LoadBalancer struct {
    balancer    *balancer.RoundRobinBalancer     // Round-robin балансировщик
    logger      *zap.SugaredLogger               // Логгер
    server      *http.Server                     // HTTP сервер
    rateLimiter *ratelimiter.RateLimiter         // Rate limiter на основе Token Bucket
}

// NewLoadBalancer инициализирует новый LoadBalancer с заданной конфигурацией
func NewLoadBalancer(cfg *config.Config, logger *zap.SugaredLogger) *LoadBalancer {
    rr := balancer.NewRoundRobin(cfg.Backends, logger) // Инициализация RoundRobin балансировщика
    rl := ratelimiter.NewRateLimiter(cfg.RateLimit.Capacity, cfg.RateLimit.RefillRate, logger) // Инициализация rate limiter'а

    lb := &LoadBalancer{
        balancer:    rr,
        logger:      logger,
        rateLimiter: rl,
    }

    logger.Infof("Initialized LoadBalancer on :%d with %d backends and rate limit %d/%ds",
        cfg.Port, len(cfg.Backends), cfg.RateLimit.Capacity, cfg.RateLimit.RefillRate)

    // Фоновая очистка неактивных токен-бакетов (например, старых IP-адресов)
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

// ListenAndServe запускает HTTP-сервер на указанном адресе
func (lb *LoadBalancer) ListenAndServe(addr string) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/", lb.handle) // Роутинг всех запросов к lb.handle

    // Оборачивание mux в middleware для лимитирования скорости
    handler := ratelimiter.RateLimitMiddleware(lb.rateLimiter, lb.logger)(mux)

    lb.server = &http.Server{
        Addr:    addr,
        Handler: handler,
    }

    lb.logger.Infof("starting HTTP server on %s", addr)
    return lb.server.ListenAndServe()
}

// Shutdown — корректное завершение работы сервера с таймаутом
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

// handle — основной обработчик HTTP-запросов, выполняющий проксирование
func (lb *LoadBalancer) handle(w http.ResponseWriter, r *http.Request) {
    clientIP := extractClientIP(r) // Извлекаем IP клиента

    backend := lb.balancer.NextBackend() // Получаем следующий бэкенд по round-robin
    if backend == nil {
        lb.logger.Warn("no available backends")
        writeJSONError(w, http.StatusServiceUnavailable, "no available backends")
        return
    }

    // Создаём ReverseProxy на выбранный backend
    proxy := httputil.NewSingleHostReverseProxy(backend.URL)

    // Переопределяем director, чтобы указать правильный Host
    originalDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        originalDirector(req)
        req.Host = backend.URL.Host
    }

    // Обработка ошибок проксирования
    proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
        lb.logger.Errorf("proxy error for backend %s: %v", backend.URL, err)
        lb.balancer.MarkBackendDead(backend.URL) // Отмечаем backend как нерабочий
        writeJSONError(rw, http.StatusServiceUnavailable, "backend unavailable")
    }

    lb.logger.Infof("forwarding %s → %s", clientIP, backend.URL)
    proxy.ServeHTTP(w, r)
}

// extractClientIP извлекает IP клиента из заголовков или RemoteAddr
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
