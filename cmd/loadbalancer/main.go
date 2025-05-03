package main

import (
    "flag"
    "log"
    "os"
    "os/signal"
    "syscall"
    "fmt"

    "github.com/Manzo48/loadBalancer/pkg/config"
    "github.com/Manzo48/loadBalancer/pkg/proxy"
    "go.uber.org/zap"
)

func main() {
    configPath := flag.String("config", "config.yaml", "path to configuration file (YAML)")
    flag.Parse()

    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("failed to initialize logger: %v", err)
    }
    defer logger.Sync()

    sugar := logger.Sugar()

    cfg, err := config.Load(*configPath)
    if err != nil {
        sugar.Fatalf("failed to load config: %v", err)
    }

    lb := proxy.NewLoadBalancer(cfg, sugar)

    go func() {
        addr := fmt.Sprintf(":%d", cfg.Port)
        if err := lb.ListenAndServe(addr); err != nil {
            sugar.Fatalf("server failed: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    sugar.Info("received shutdown signal")
    lb.Shutdown()
}
