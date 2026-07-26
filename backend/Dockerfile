FROM golang:1.25-alpine AS builder

WORKDIR /app

# Dependencias
COPY go.mod go.sum* ./
RUN go mod download

# Código fuente
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

FROM alpine:3.24
WORKDIR /root/
COPY --from=builder /api .

EXPOSE 8080
CMD ["./api"]
