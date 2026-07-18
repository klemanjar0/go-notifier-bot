# syntax=docker/dockerfile:1

# ---- Build stage ------------------------------------------------------------
FROM golang:1.25.3-alpine AS build

WORKDIR /src

# Cache dependencies first so code changes don't invalidate the module layer.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Static, stripped binaries. pgx is pure Go, so CGO is not needed.
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/bot ./cmd/bot && \
    CGO_ENABLED=0 GOOS=linux go build \
        -trimpath -ldflags="-s -w" \
        -o /out/migrate ./cmd/migrate

# ---- Runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

# Binaries.
COPY --from=build /out/bot /app/bot
COPY --from=build /out/migrate /app/migrate

# Migration files are read from disk at runtime by cmd/migrate (source/file).
COPY --from=build /src/internal/db/migrations /app/internal/db/migrations

# Runs as the distroless "nonroot" user (uid 65532) by default.
ENV LOG_LEVEL=info

ENTRYPOINT ["/app/bot"]
