# ══════════════════════════════════════════════════════════════════════════════
# OmniLedger — Multi-Stage Dockerfile
# Final image: ~15 MB (scratch + static Go binary)
# ══════════════════════════════════════════════════════════════════════════════

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Build metadata
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# OS-level deps for CGO-free compilation
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build

# Cache module download (only re-download when go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source
COPY . .

# Compile a fully static binary — no CGO, no libc dependency
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/vault-api \
    ./cmd/api

# ── Stage 2: Runtime (scratch — zero OS) ──────────────────────────────────────
FROM scratch

# Import TLS certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Import SQL migrations for init
COPY --from=builder /build/sql /sql

# Import compiled binary
COPY --from=builder /build/vault-api /vault-api

# Non-root user (UID 65534 = "nobody")
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/vault-api"]
