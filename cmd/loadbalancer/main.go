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
    // Парсим флаг командной строки — путь до YAML-конфига
    configPath := flag.String("config", "config.yaml", "path to configuration file (YAML)")
    flag.Parse()

    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("failed to initialize logger: %v", err)
    }
    defer logger.Sync() // Убедимся, что логгер синхронизирует буферы при завершении

    sugar := logger.Sugar() 

   
    cfg, err := config.Load(*configPath)
    if err != nil {
        sugar.Fatalf("failed to load config: %v", err)
    }

  
    lb := proxy.NewLoadBalancer(cfg, sugar)

    // Запускаем HTTP-сервер в отдельной горутине
    go func() {
        addr := fmt.Sprintf(":%d", cfg.Port)
        if err := lb.ListenAndServe(addr); err != nil {
            // Если сервер завершился с ошибкой — фатальный лог и выход
            sugar.Fatalf("server failed: %v", err)
        }
    }()

    // Настраиваем канал для перехвата системных сигналов (SIGINT/SIGTERM)
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

    // Блокируем выполнение, пока не получим сигнал завершения
    <-quit
    sugar.Info("received shutdown signal")

    
    lb.Shutdown()
}
