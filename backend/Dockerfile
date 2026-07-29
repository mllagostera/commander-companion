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
# El binario aplica las migraciones de goose al arrancar (ver
# internal/common/migrate.go y cmd/api/main.go), así que necesita el
# directorio migrations/ presente en runtime.
COPY migrations ./migrations

EXPOSE 8080
CMD ["./api"]
