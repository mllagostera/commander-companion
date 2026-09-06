FROM golang:1.25-alpine AS builder

WORKDIR /app

# Dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Source code
COPY . .

# Build.
#
# GIT_COMMIT stamps the source revision into the binary so GET /health can say
# which build is answering (see ADR-0020). It is optional: without it the
# binary falls back to RENDER_GIT_COMMIT at runtime, which is what the current
# Render deployment relies on. Pass it when building the image elsewhere:
#   docker build --build-arg GIT_COMMIT="$(git rev-parse HEAD)" backend/
# The build context is backend/, which excludes the repository's .git, so the
# Go toolchain cannot stamp the revision on its own here.
ARG GIT_COMMIT=""
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X github.com/usuario/commander-companion-backend/internal/config.buildCommit=${GIT_COMMIT}" \
    -o /api ./cmd/api

FROM alpine:3.24
WORKDIR /root/
COPY --from=builder /api .
# The binary applies goose migrations on startup (see
# internal/common/migrate.go and cmd/api/main.go), so it needs the
# migrations/ directory present at runtime.
COPY migrations ./migrations

EXPOSE 8080
CMD ["./api"]
