package balancer

import (
    "net/http"
    "net/url"
    "sync/atomic"
    "time"

    "go.uber.org/zap"
)

// Backend представляет один backend-сервер (сервер, который обрабатывает реальные запросы).
type Backend struct {
    URL   *url.URL     // Адрес backend-сервера
    Alive atomic.Bool  // Флаг, указывающий, жив ли backend (используется в health-check)
}

// RoundRobinBalancer реализует балансировку нагрузки по принципу Round-Robin
// с проверкой состояния backend'ов (health-check).
type RoundRobinBalancer struct {
    backends []*Backend           // Список всех backend'ов
    index    uint32               // Индекс текущего backend'a (для цикличного выбора)
    logger   *zap.SugaredLogger   // Логгер

    checkInterval time.Duration   // Интервал между проверками состояния серверов
    timeout       time.Duration   // Таймаут для health-check запроса
}

// NewRoundRobin создает новый RoundRobinBalancer и запускает health-check loop.
func NewRoundRobin(urls []string, logger *zap.SugaredLogger) *RoundRobinBalancer {
    bb := &RoundRobinBalancer{
        backends:      make([]*Backend, 0, len(urls)),
        logger:        logger,
        checkInterval: 10 * time.Second, // Проверка каждые 10 секунд
        timeout:       2 * time.Second,  // 2 секунды на health-check
    }

    // Инициализация backend'ов
    for _, raw := range urls {
        u, err := url.Parse(raw)
        if err != nil {
            logger.Warnf("invalid backend URL %s: %v", raw, err)
            continue
        }
        b := &Backend{URL: u}
        b.Alive.Store(true) // По умолчанию считаем, что backend живой
        bb.backends = append(bb.backends, b)
        logger.Infof("added backend: %s", u.String())
    }

    // Запускаем фоновую проверку здоровья серверов
    go bb.healthLoop()

    return bb
}

// healthLoop запускается в отдельной горутине и периодически проверяет
// доступность всех backend'ов по адресу /health.
func (r *RoundRobinBalancer) healthLoop() {
    client := &http.Client{Timeout: r.timeout}
    ticker := time.NewTicker(r.checkInterval)
    defer ticker.Stop()

    for range ticker.C {
        for _, b := range r.backends {
            // Проверка каждого backend'a в отдельной горутине
            go func(b *Backend) {
                // Выполняем GET-запрос на /health
                resp, err := client.Get(b.URL.String() + "/health")

                // alive = true, если нет ошибки и статус код OK
                alive := err == nil && resp.StatusCode == http.StatusOK
                b.Alive.Store(alive)

                if alive {
                    r.logger.Debugf("health OK: %s", b.URL)
                } else {
                    r.logger.Warnf("health FAILED: %s (%v)", b.URL, err)
                }

                // Закрываем тело ответа, если он был
                if resp != nil {
                    resp.Body.Close()
                }
            }(b)
        }
    }
}

// NextBackend возвращает следующий доступный backend в порядке Round-Robin.
// Пропускает мертвые сервера.
func (r *RoundRobinBalancer) NextBackend() *Backend {
    total := len(r.backends)
    for i := 0; i < total; i++ {
        // Инкрементируем индекс атомарно и берём модуль по количеству backend'ов
        idx := atomic.AddUint32(&r.index, 1) % uint32(total)
        b := r.backends[idx]

        // Если backend живой, возвращаем его
        if b.Alive.Load() {
            r.logger.Debugf("selected backend: %s", b.URL)
            return b
        }
    }

    // Если ни один backend не доступен
    r.logger.Warn("no alive backends available")
    return nil
}

// MarkBackendDead помечает указанный backend как "мертвый" (Alive = false).
// Используется в случае ошибки проксирования.
func (r *RoundRobinBalancer) MarkBackendDead(target *url.URL) {
    for _, b := range r.backends {
        if b.URL.String() == target.String() {
            b.Alive.Store(false)
            r.logger.Warnf("marked backend dead: %s", target)
            // Он останется мертвым до следующей успешной проверки healthLoop
            return
        }
    }
}
