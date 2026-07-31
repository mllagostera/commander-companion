FROM golang:1.25-alpine AS builder

WORKDIR /app

# Dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

FROM alpine:3.24
WORKDIR /root/
COPY --from=builder /api .
# The binary applies goose migrations on startup (see
# internal/common/migrate.go and cmd/api/main.go), so it needs the
# migrations/ directory present at runtime.
COPY migrations ./migrations

EXPOSE 8080
CMD ["./api"]
