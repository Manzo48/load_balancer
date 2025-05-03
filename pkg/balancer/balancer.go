package balancer

import (
    "net/http"
    "net/url"
    "sync/atomic"
    "time"

    "go.uber.org/zap"
)

type Backend struct {
    URL   *url.URL
    Alive atomic.Bool
}

// RoundRobinBalancer распределяет запросы по живым бэкендам.
type RoundRobinBalancer struct {
    backends []*Backend
    index    uint32
    logger   *zap.SugaredLogger

    // health-check
    checkInterval time.Duration
    timeout       time.Duration
}

// NewRoundRobin создаёт балансировщик с health‑check’ем
func NewRoundRobin(urls []string, logger *zap.SugaredLogger) *RoundRobinBalancer {
    bb := &RoundRobinBalancer{
        backends:      make([]*Backend, 0, len(urls)),
        logger:        logger,
        checkInterval: 10 * time.Second,
        timeout:       2 * time.Second,
    }
    for _, raw := range urls {
        u, err := url.Parse(raw)
        if err != nil {
            logger.Warnf("invalid backend URL %s: %v", raw, err)
            continue
        }
        b := &Backend{URL: u}
        b.Alive.Store(true)
        bb.backends = append(bb.backends, b)
        logger.Infof("added backend: %s", u.String())
    }
    go bb.healthLoop()
    return bb
}

// healthLoop раз в checkInterval проверяет всех бекендов
func (r *RoundRobinBalancer) healthLoop() {
    client := &http.Client{Timeout: r.timeout}
    ticker := time.NewTicker(r.checkInterval)
    defer ticker.Stop()

    for range ticker.C {
        for _, b := range r.backends {
            go func(b *Backend) {
                resp, err := client.Get(b.URL.String() + "/health")
                alive := err == nil && resp.StatusCode == http.StatusOK
                b.Alive.Store(alive)
                if alive {
                    r.logger.Debugf("health OK: %s", b.URL)
                } else {
                    r.logger.Warnf("health FAILED: %s (%v)", b.URL, err)
                }
                if resp != nil {
                    resp.Body.Close()
                }
            }(b)
        }
    }
}

func (r *RoundRobinBalancer) NextBackend() *Backend {
    total := len(r.backends)
    for i := 0; i < total; i++ {
        idx := atomic.AddUint32(&r.index, 1) % uint32(total)
        b := r.backends[idx]
        if b.Alive.Load() {
            r.logger.Debugf("selected backend: %s", b.URL)
            return b
        }
    }
    r.logger.Warn("no alive backends available")
    return nil
}

func (r *RoundRobinBalancer) MarkBackendDead(target *url.URL) {
    for _, b := range r.backends {
        if b.URL.String() == target.String() {
            b.Alive.Store(false)
            r.logger.Warnf("marked backend dead: %s", target)
            // не сразу возвращаем в пул — пусть healthLoop решит
            return
        }
    }
}
