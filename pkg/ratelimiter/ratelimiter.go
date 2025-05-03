package ratelimiter

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type TokenBucket struct {
    Capacity   int
    Tokens     int
    RefillRate int
    mu         sync.Mutex
    lastRefill time.Time
    lastSeen   time.Time // новое поле
}

func NewTokenBucket(capacity, refillRate int) *TokenBucket {
    now := time.Now()
    return &TokenBucket{
        Capacity:   capacity,
        Tokens:     capacity,
        RefillRate: refillRate,
        lastRefill: now,
        lastSeen:   now,
    }
}

// Пополняет токены, если прошло время
func (tb *TokenBucket) refill() {
    now := time.Now()
    elapsed := now.Sub(tb.lastRefill).Seconds()
    tokensToAdd := int(elapsed * float64(tb.RefillRate))
    if tokensToAdd > 0 {
        tb.Tokens = min(tb.Capacity, tb.Tokens+tokensToAdd)
        tb.lastRefill = now
    }
}

func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    tb.refill()
    tb.lastSeen = time.Now()

    if tb.Tokens > 0 {
        tb.Tokens--
        return true
    }
    return false
}

// RateLimiter хранит bucket'ы по IP или API-ключу
type RateLimiter struct {
    buckets map[string]*TokenBucket
    mu      sync.RWMutex

    defaultCapacity   int
    defaultRefillRate int
}

func NewRateLimiter(capacity, refillRate int, logger *zap.SugaredLogger) *RateLimiter {
    return &RateLimiter{
        buckets:          make(map[string]*TokenBucket),
        defaultCapacity:  capacity,
        defaultRefillRate: refillRate,
    }
}

func (rl *RateLimiter) getBucket(clientID string) *TokenBucket {
    rl.mu.RLock()
    bucket, exists := rl.buckets[clientID]
    rl.mu.RUnlock()

    if !exists {
        rl.mu.Lock()
        defer rl.mu.Unlock()
        bucket = NewTokenBucket(rl.defaultCapacity, rl.defaultRefillRate)
        rl.buckets[clientID] = bucket
    }
    return bucket
}

func (rl *RateLimiter) Allow(clientID string) bool {
    bucket := rl.getBucket(clientID)
    return bucket.Allow()
}

func (rl *RateLimiter) Cleanup(expiration time.Duration) {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    for clientID, bucket := range rl.buckets {
        bucket.mu.Lock()
        lastSeen := bucket.lastSeen
        bucket.mu.Unlock()

        if now.Sub(lastSeen) > expiration {
            delete(rl.buckets, clientID)
        }
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
