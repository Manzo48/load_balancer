package integration

import (
    "testing"
    "time"

    "github.com/Manzo48/loadBalancer/pkg/ratelimiter"
    "go.uber.org/zap"
)


func BenchmarkRateLimiter(b *testing.B) {
    logger := zap.NewNop().Sugar()
    rl := ratelimiter.NewRateLimiter(1000, 100, logger)
    clientID := "bench_client"

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            rl.Allow(clientID)
        }
    })
}

func BenchmarkRateLimiterWithMultipleClients(b *testing.B) {
    logger := zap.NewNop().Sugar()
    rl := ratelimiter.NewRateLimiter(100, 10, logger)
    
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            clientID := string(rune(i%10 + 65)) // Клиенты A-J
            rl.Allow(clientID)
            i++
        }
    })
}
func TestRateLimiter_BasicLimit(t *testing.T) {
    logger := zap.NewNop().Sugar()
    rl := ratelimiter.NewRateLimiter(5, 1, logger)

    // First 5 requests should be allowed
    for i := 0; i < 5; i++ {
        if !rl.Allow("client1") {
            t.Errorf("Request %d should be allowed", i+1)
        }
    }

    // Sixth request should be blocked
    if rl.Allow("client1") {
        t.Error("Expected request to be blocked")
    }

    // Wait for refill
    time.Sleep(1200 * time.Millisecond)

    // Next request should be allowed
    if !rl.Allow("client1") {
        t.Error("Request after refill should be allowed")
    }
}

func TestRateLimiter_MultipleClients(t *testing.T) {
    logger := zap.NewNop().Sugar()
    rl := ratelimiter.NewRateLimiter(3, 1, logger)

    for i := 0; i < 3; i++ {
        if !rl.Allow("client1") {
            t.Errorf("Client1 request %d should be allowed", i+1)
        }
    }

  
    if !rl.Allow("client2") {
        t.Error("Client2 first request should be allowed")
    }
}

