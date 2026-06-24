#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="/etc/ctrld"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
DATA_DIR="/var/lib/ctrld"

# ── JWT-Secret ────────────────────────────────────────────────────────────────
# Aus Env-Variable oder auto-generieren
if [[ -z "${CTRLD_SECURITY_JWT_SECRET:-}" ]]; then
  echo "[WARN] CTRLD_SECURITY_JWT_SECRET nicht gesetzt — generiere zufälliges Secret"
  export CTRLD_SECURITY_JWT_SECRET=$(cat /dev/urandom | tr -dc 'a-f0-9' | head -c 64)
  echo "[WARN] Secret wird bei Neustart neu generiert — alle Sessions werden ungültig!"
  echo "[WARN] Setze CTRLD_SECURITY_JWT_SECRET in docker-compose.yml für Persistenz"
fi

# ── Konfiguration generieren ──────────────────────────────────────────────────
if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "[INFO] Erstelle Konfiguration: $CONFIG_FILE"
  cat > "$CONFIG_FILE" << EOF
server:
  host: "0.0.0.0"
  port: ${CTRLD_PORT:-8443}
  read_timeout_sec: 10
  write_timeout_sec: 30
  idle_timeout_sec: 120

log:
  level: "${CTRLD_LOG_LEVEL:-info}"
  format: "json"

security:
  jwt_secret: "${CTRLD_SECURITY_JWT_SECRET}"
  jwt_access_ttl_min: 15
  jwt_refresh_ttl_day: 7
  argon_memory: 65536
  argon_iterations: 3
  argon_parallelism: 2

database:
  path: "${DATA_DIR}/ctrld.db"
EOF
  chown ctrld:ctrld "$CONFIG_FILE"
  chmod 640 "$CONFIG_FILE"
else
  echo "[INFO] Konfiguration bereits vorhanden: $CONFIG_FILE"
fi

# ── Next.js Frontend im Hintergrund ──────────────────────────────────────────
echo "[INFO] Starte Frontend (Next.js) auf Port ${NEXT_PORT:-3000}..."
cd /app/web
HOSTNAME="0.0.0.0" \
PORT="${NEXT_PORT:-3000}" \
NEXT_PUBLIC_API_URL="http://localhost:${CTRLD_PORT:-8443}/api/v1" \
NEXT_PUBLIC_WS_HOST="localhost:${CTRLD_PORT:-8443}" \
  node server.js &
FRONTEND_PID=$!

# ── CTRLD Backend ─────────────────────────────────────────────────────────────
echo "[INFO] Starte CTRLD Backend auf Port ${CTRLD_PORT:-8443}..."
exec /usr/local/bin/ctrld -config "$CONFIG_FILE"
