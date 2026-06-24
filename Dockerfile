# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Frontend Build (Next.js)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:24-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci --frozen-lockfile

COPY web/ ./

# Next.js Standalone-Output aktivieren (wird in next.config.ts gesetzt)
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Backend Build (Go)
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS backend-builder

# CGO benötigt GCC (für go-sqlite3)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X github.com/Thoomaastb/CTRLD/pkg/version.Version=${VERSION}" \
    -o /usr/local/bin/ctrld \
    ./cmd/ctrld

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: Final Runtime Image (Debian Slim)
# ─────────────────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# System-User (kein Login)
RUN useradd --system --no-create-home --shell /sbin/nologin \
    --home-dir /var/lib/ctrld ctrld

# Verzeichnisse anlegen
RUN mkdir -p /var/lib/ctrld /etc/ctrld /app/web && \
    chown ctrld:ctrld /var/lib/ctrld /etc/ctrld

# Backend Binary
COPY --from=backend-builder /usr/local/bin/ctrld /usr/local/bin/ctrld

# Frontend (Next.js Standalone)
COPY --from=frontend-builder /app/web/.next/standalone /app/web/
COPY --from=frontend-builder /app/web/.next/static     /app/web/.next/static
COPY --from=frontend-builder /app/web/public           /app/web/public

# Entrypoint
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8443 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -f http://localhost:8443/api/v1/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
