# Stage 1: Frontend Build (Next.js)
FROM node:24-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Backend Build (Go)
FROM golang:1.24-bookworm AS backend-builder
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc-dev && rm -rf /var/lib/apt/lists/*

ENV GOPRIVATE=github.com/Thoomaastb/CTRLD
ENV GONOSUMDB=github.com/Thoomaastb/CTRLD
ENV GONOPROXY=github.com/Thoomaastb/CTRLD

WORKDIR /app
COPY go.mod go.sum ./
COPY . .
RUN go mod download

ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X github.com/Thoomaastb/CTRLD/pkg/version.Version=${VERSION}" \
    -o /usr/local/bin/ctrld \
    ./cmd/ctrld

# Stage 3: Runtime
FROM node:24-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
RUN useradd --system --no-create-home --shell /sbin/nologin --home-dir /var/lib/ctrld ctrld
RUN mkdir -p /var/lib/ctrld /etc/ctrld /app/web && chown ctrld:ctrld /var/lib/ctrld /etc/ctrld

COPY --from=backend-builder /usr/local/bin/ctrld /usr/local/bin/ctrld
COPY --from=frontend-builder /app/web/.next/standalone /app/web/
COPY --from=frontend-builder /app/web/.next/static     /app/web/.next/static
COPY --from=frontend-builder /app/web/public           /app/web/public
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8443 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -f http://localhost:8443/api/v1/health || exit 1
ENTRYPOINT ["/entrypoint.sh"]