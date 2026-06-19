# ── Stage 1: Build frontend ───────────────────────────────────────────────────
FROM node:24-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ── Stage 2: Build backend ────────────────────────────────────────────────────
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

# CGO für go-sqlite3 (C-Compiler + musl-dev)
RUN apk add --no-cache gcc musl-dev ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Version via ldflags — ARG für CI-Übergabe
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux \
    go build \
    -ldflags="-s -w -X github.com/Thoomaastb/CTRLD/pkg/version.Version=${VERSION}" \
    -o /ctrld \
    ./cmd/ctrld

# ── Stage 3: Runtime (distroless) ─────────────────────────────────────────────
# distroless/base (nicht static) — nötig wegen CGO/go-sqlite3 (braucht glibc)
FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=backend-builder /ctrld /ctrld

EXPOSE 8443

ENTRYPOINT ["/ctrld"]
