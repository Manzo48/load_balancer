package ratelimiter

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// TokenBucket реализует алгоритм "токен-бакета" для ограничения количества запросов.
// Каждый клиент получает свой собственный токен-бакет.
type TokenBucket struct {
	Capacity   int           // Максимальное количество токенов в бакете
	Tokens     int           // Текущее количество токенов
	RefillRate int           // Скорость пополнения токенов (токенов в секунду)
	mu         sync.Mutex    // Мьютекс для потокобезопасного доступа
	lastRefill time.Time     // Последнее время пополнения токенов
	lastSeen   time.Time     // Последнее время активности клиента
}

// NewTokenBucket создает новый токен-бакет с заданной ёмкостью и скоростью пополнения
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		Capacity:   capacity,
		Tokens:     capacity, // бакет стартует полным
		RefillRate: refillRate,
		lastRefill: now,
		lastSeen:   now,
	}
}

// refill добавляет токены в бакет на основе прошедшего времени
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Считаем, сколько токенов можно добавить
	tokensToAdd := int(elapsed * float64(tb.RefillRate))
	if tokensToAdd > 0 {
		// Не превышаем ёмкость
		tb.Tokens = min(tb.Capacity, tb.Tokens+tokensToAdd)
		tb.lastRefill = now
	}
}

// Allow проверяет, есть ли доступный токен для клиента
// Возвращает true, если токен доступен, иначе false
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()                  // Пополняем токены
	tb.lastSeen = time.Now()    // Обновляем время последней активности

	if tb.Tokens > 0 {
		tb.Tokens-- // Используем токен
		return true
	}
	return false // Нет токенов — лимит превышен
}

// RateLimiter управляет токен-бакетами для всех клиентов
type RateLimiter struct {
	buckets           map[string]*TokenBucket // Мапа токен-бакетов по IP/ClientID
	mu                sync.RWMutex            // RW-мьютекс для безопасного доступа
	clientLimits      map[string]ClientLimit  // Индивидуальные лимиты для клиентов
	defaultCapacity   int                     // Значение по умолчанию: ёмкость бакета
	defaultRefillRate int                     // Значение по умолчанию: скорость пополнения
}

// ClientLimit описывает лимит токен-бакета для конкретного клиента
type ClientLimit struct {
	Capacity   int // Максимум токенов
	RefillRate int // Скорость пополнения токенов (в сек.)
}

// NewRateLimiter создает новый rate limiter с настройками по умолчанию
func NewRateLimiter(capacity, refillRate int, logger *zap.SugaredLogger) *RateLimiter {
	return &RateLimiter{
		buckets:           make(map[string]*TokenBucket),
		clientLimits:      make(map[string]ClientLimit),
		defaultCapacity:   capacity,
		defaultRefillRate: refillRate,
	}
}

// SetClientLimit задаёт индивидуальный лимит для конкретного клиента
func (rl *RateLimiter) SetClientLimit(clientID string, limit ClientLimit) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.clientLimits[clientID] = limit
}

// getBucket возвращает токен-бакет для клиента.
// Если он не существует — создаёт его с индивидуальным или дефолтным лимитом.
func (rl *RateLimiter) getBucket(clientID string) *TokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		defer rl.mu.Unlock()

		// Проверяем, есть ли индивидуальный лимит
		limit, exists := rl.clientLimits[clientID]
		if !exists {
			limit = ClientLimit{
				Capacity:   rl.defaultCapacity,
				RefillRate: rl.defaultRefillRate,
			}
		}

		// Создаём и сохраняем новый бакет
		bucket = NewTokenBucket(limit.Capacity, limit.RefillRate)
		rl.buckets[clientID] = bucket
	}
	return bucket
}

// Allow проверяет, можно ли обслужить клиента с данным ID (IP, токен и т.п.)
func (rl *RateLimiter) Allow(clientID string) bool {
	bucket := rl.getBucket(clientID)
	return bucket.Allow()
}

// Cleanup удаляет неактивные токен-бакеты, которые не использовались дольше заданного времени
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

// база)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
