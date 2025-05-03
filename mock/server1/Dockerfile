FROM golang:1.21-alpine

WORKDIR /app
COPY . .
RUN go build -o server main.go

EXPOSE 9001
CMD ["./server"]
