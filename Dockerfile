# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ── Stage 2: Build backend ────────────────────────────────────
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed frontend static files into binary
COPY --from=frontend-builder /app/web/out ./web/out

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /ctrld ./cmd/ctrld

# ── Stage 3: Runtime ─────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=backend-builder /ctrld /ctrld

EXPOSE 8443

ENTRYPOINT ["/ctrld", "serve"]
