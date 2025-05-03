# Стадия сборки
FROM golang:1.21 AS builder
WORKDIR /app

# Модули
COPY go.mod go.sum ./
RUN go mod tidy

# Исходный код
COPY . .

# Сборка статического бинарника
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o loadbalancer cmd/loadbalancer/main.go

# Минимальный финальный образ
FROM scratch
COPY --from=builder /app/loadbalancer /loadbalancer
COPY configs/ /configs/
WORKDIR /
ENTRYPOINT ["/loadbalancer"]
