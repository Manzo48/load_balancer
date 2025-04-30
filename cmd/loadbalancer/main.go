package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"

	"github.com/Manzo48/loadbalancer/pkg/config"
    "github.com/Manzo48/load_balancer/pkg/log"
    "github.com/your_username/loadbalancer/pkg/proxy"
)

func main() {
    // Флаги: путь к конфигу
    configFile := flag.String("config", "configs/config.yaml", "path to config file")
    flag.Parse()

    // Инициализация логгера
    logger := log.New()

    // Чтение конфигурации
    cfg, err := config.Load(*configFile)
    if err != nil {
        logger.Fatalf("failed to load config: %v", err)
    }

    // Инициализация прокси
    lb := proxy.NewLoadBalancer(cfg, logger)

    // Запуск HTTP-сервера
    go func() {
        addr := fmt.Sprintf(":%d", cfg.Port)
        logger.Infof("starting load balancer on %s", addr)
        if err := lb.ListenAndServe(addr); err != nil {
            logger.Fatalf("server error: %v", err)
        }
    }()

    // Graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    <-stop
    logger.Info("shutting down...")
    lb.Shutdown()
}