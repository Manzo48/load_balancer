
FROM golang:1.23 AS builder

WORKDIR /app


COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod tidy

# Копируем исходный код
COPY . .

# Собираем бинарник для Linux без зависимостей
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o loadbalancer cmd/loadbalancer/main.go

# Второй этап: минимальный образ для финальной сборки
FROM alpine:latest

# Устанавливаем зависимости для работы с SSL (если понадобится)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Копируем бинарник и конфиги
COPY --from=builder /app/loadbalancer /loadbalancer
COPY --from=builder /app/configs /configs


RUN mkdir /logs


EXPOSE 8080

# Команда запуска
CMD ["/loadbalancer", "-config=/configs/config.yaml"]
