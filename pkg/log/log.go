package log

import "go.uber.org/zap"

func New() *zap.SugaredLogger {
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    return logger.Sugar()
}