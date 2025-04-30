# syntax=docker/dockerfile:1

# Этап сборки
FROM golang:1.21 AS builder

WORKDIR /app

# Копируем go.mod и go.sum и устанавливаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -o loadbalancer ./cmd/loadbalancer

# Финальный образ
FROM alpine:latest

# Установка сертификатов (если балансировщик будет делать HTTPS-запросы к бэкендам)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Копируем бинарник из builder-образа
COPY --from=builder /app/loadbalancer .

# Копируем конфигурационный файл
COPY configs/config.yaml ./configs/config.yaml

# Открываем порт (например, 8080)
EXPOSE 8080

# Стартуем балансировщик
ENTRYPOINT ["./loadbalancer", "-config=configs/config.yaml"]
