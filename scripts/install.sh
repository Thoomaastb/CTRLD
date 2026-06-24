#!/usr/bin/env bash
# CTRLD One-Line Installer
# Usage: curl -fsSL https://get.ctrld.io | bash
# Or:    curl -fsSL https://get.ctrld.io | bash -s -- --version v1.0.0

set -euo pipefail

# ── Farben ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# ── Konfiguration ─────────────────────────────────────────────────────────────
CTRLD_USER="ctrld"
CTRLD_GROUP="ctrld"
CTRLD_HOME="/var/lib/ctrld"
CTRLD_CONFIG_DIR="/etc/ctrld"
CTRLD_LOG_DIR="/var/log/ctrld"
CTRLD_BIN="/usr/local/bin/ctrld"
CTRLD_SERVICE="/etc/systemd/system/ctrld.service"
GITHUB_REPO="Thoomaastb/CTRLD"
VERSION="${CTRLD_VERSION:-latest}"

# ── Argument-Parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case $1 in
    --version) VERSION="$2"; shift 2 ;;
    --no-service) NO_SERVICE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    *) echo "Unbekanntes Argument: $1"; exit 1 ;;
  esac
done

DRY_RUN=${DRY_RUN:-0}
NO_SERVICE=${NO_SERVICE:-0}

log()    { echo -e "${GREEN}[CTRLD]${NC} $*"; }
warn()   { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }
header() { echo -e "\n${BOLD}${BLUE}$*${NC}"; }
run()    { [[ $DRY_RUN -eq 1 ]] && echo "  [dry-run] $*" || eval "$*"; }

# ── System-Prüfungen ──────────────────────────────────────────────────────────
header "🔍 System-Prüfung"

# Root-Check
[[ $EUID -ne 0 ]] && error "Installer muss als root ausgeführt werden (sudo bash install.sh)"

# OS-Prüfung
if [[ -f /etc/os-release ]]; then
  . /etc/os-release
  log "OS: $PRETTY_NAME"
  case "$ID" in
    ubuntu|debian) PKG_MANAGER="apt-get" ;;
    centos|rhel|fedora|rocky|almalinux) PKG_MANAGER="dnf" ;;
    *) warn "Unbekannte Distribution: $ID — versuche trotzdem weiterzumachen" ;;
  esac
else
  error "Kann OS nicht bestimmen — /etc/os-release nicht gefunden"
fi

# Systemd-Check
command -v systemctl &>/dev/null || error "systemd nicht gefunden — CTRLD benötigt systemd"

# Architektur
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH_SUFFIX="linux-amd64" ;;
  aarch64) ARCH_SUFFIX="linux-arm64" ;;
  *)       error "Nicht unterstützte Architektur: $ARCH" ;;
esac
log "Architektur: $ARCH ($ARCH_SUFFIX)"

# ── Version bestimmen ─────────────────────────────────────────────────────────
header "📦 Version"

if [[ "$VERSION" == "latest" ]]; then
  log "Neueste Version wird ermittelt..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  [[ -z "$VERSION" ]] && error "Konnte neueste Version nicht ermitteln"
fi

log "Installiere CTRLD $VERSION"

DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/ctrld-${ARCH_SUFFIX}"

# ── Dependencies ──────────────────────────────────────────────────────────────
header "📚 Dependencies"

run "curl --version &>/dev/null || ${PKG_MANAGER} install -y curl"

# ── User + Verzeichnisse ──────────────────────────────────────────────────────
header "👤 System-User + Verzeichnisse"

if ! id "$CTRLD_USER" &>/dev/null; then
  log "Erstelle System-User: $CTRLD_USER"
  run "useradd --system --no-create-home --shell /sbin/nologin \
    --home-dir $CTRLD_HOME --comment 'CTRLD Service Account' $CTRLD_USER"
else
  log "System-User $CTRLD_USER existiert bereits"
fi

for dir in "$CTRLD_HOME" "$CTRLD_CONFIG_DIR" "$CTRLD_LOG_DIR"; do
  run "mkdir -p $dir"
  run "chown $CTRLD_USER:$CTRLD_GROUP $dir"
  run "chmod 750 $dir"
done

# ── Binary herunterladen ──────────────────────────────────────────────────────
header "⬇️  Binary herunterladen"

log "Download: $DOWNLOAD_URL"
run "curl -fsSL -o /tmp/ctrld-new $DOWNLOAD_URL"
run "chmod +x /tmp/ctrld-new"

# Binary verifizieren
if [[ $DRY_RUN -eq 0 ]]; then
  /tmp/ctrld-new --version &>/dev/null || error "Binary-Test fehlgeschlagen"
  log "Binary verifiziert ✓"
fi

run "mv /tmp/ctrld-new $CTRLD_BIN"
log "Binary installiert: $CTRLD_BIN"

# ── Konfiguration ─────────────────────────────────────────────────────────────
header "⚙️  Konfiguration"

if [[ ! -f "$CTRLD_CONFIG_DIR/config.yaml" ]]; then
  JWT_SECRET=$(openssl rand -hex 32 2>/dev/null || cat /dev/urandom | tr -dc 'a-f0-9' | head -c 64)
  log "Generiere Konfiguration..."

  run "cat > $CTRLD_CONFIG_DIR/config.yaml << EOF
# CTRLD Konfiguration — automatisch generiert
# Datum: $(date -u +%Y-%m-%dT%H:%M:%SZ)

server:
  host: \"0.0.0.0\"
  port: 8443
  read_timeout_sec: 10
  write_timeout_sec: 30
  idle_timeout_sec: 120

log:
  level: \"info\"
  format: \"json\"

security:
  jwt_secret: \"${JWT_SECRET}\"
  jwt_access_ttl_min: 15
  jwt_refresh_ttl_day: 7
  argon_memory: 65536
  argon_iterations: 3
  argon_parallelism: 2

database:
  path: \"${CTRLD_HOME}/ctrld.db\"
EOF"

  run "chown $CTRLD_USER:$CTRLD_GROUP $CTRLD_CONFIG_DIR/config.yaml"
  run "chmod 640 $CTRLD_CONFIG_DIR/config.yaml"
  log "Konfiguration erstellt: $CTRLD_CONFIG_DIR/config.yaml"
else
  log "Konfiguration existiert bereits — wird nicht überschrieben"
fi

# ── systemd Service ───────────────────────────────────────────────────────────
if [[ $NO_SERVICE -eq 0 ]]; then
  header "🔧 systemd Service"

  run "cat > $CTRLD_SERVICE << EOF
[Unit]
Description=CTRLD Server Control Panel
Documentation=https://docs.ctrld.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$CTRLD_USER
Group=$CTRLD_GROUP
ExecStart=$CTRLD_BIN -config $CTRLD_CONFIG_DIR/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# Sicherheits-Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$CTRLD_HOME $CTRLD_LOG_DIR
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
SystemCallFilter=@system-service
SystemCallArchitectures=native
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
EOF"

  run "systemctl daemon-reload"
  run "systemctl enable ctrld"
  run "systemctl start ctrld"

  log "Service gestartet: ctrld.service"
fi

# ── Abschluss ─────────────────────────────────────────────────────────────────
header "✅ Installation abgeschlossen"

HOSTNAME=$(hostname -f 2>/dev/null || hostname)
PORT=8443

echo ""
echo -e "  ${BOLD}CTRLD ist installiert und läuft!${NC}"
echo ""
echo -e "  🌐 URL:      ${BLUE}http://${HOSTNAME}:${PORT}${NC}"
echo -e "  ⚙️  Config:   ${CTRLD_CONFIG_DIR}/config.yaml"
echo -e "  📁 Daten:    ${CTRLD_HOME}/"
echo -e "  📋 Logs:     journalctl -u ctrld -f"
echo ""
echo -e "  ${YELLOW}Wichtig:${NC} Öffne die URL und schließe den Setup-Wizard ab."
echo ""
