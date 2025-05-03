
package integration

import (
    "net/http"
    "sync"
    "testing"
    "time"

    "github.com/Manzo48/loadBalancer/pkg/config"
    "github.com/Manzo48/loadBalancer/pkg/proxy"
    "go.uber.org/zap"
)

func TestBalancerWithRace(t *testing.T) {
    cfg := &config.Config{
        Backends: []string{"http://localhost:9001", "http://localhost:9002"},
    }

    lb := proxy.NewLoadBalancer(cfg, zap.NewNop().Sugar())
    go lb.ListenAndServe(":8080")
    defer lb.Shutdown()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            client := &http.Client{Timeout: 1 * time.Second}
            resp, err := client.Get("http://localhost:8080")
            if err == nil {
                defer resp.Body.Close()
            }
        }()
    }
    wg.Wait()
}

func BenchmarkBalancer(b *testing.B) {
    cfg := &config.Config{
        Backends: []string{"http://localhost:9001", "http://localhost:9002"},
    }

    lb := proxy.NewLoadBalancer(cfg, zap.NewNop().Sugar())
    go lb.ListenAndServe(":8080")
    defer lb.Shutdown()

    client := &http.Client{Timeout: 1 * time.Second}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resp, _ := client.Get("http://localhost:8080")
        if resp != nil {
            resp.Body.Close()
        }
    }
}