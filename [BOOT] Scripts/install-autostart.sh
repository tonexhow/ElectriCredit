#!/usr/bin/env bash
set -Eeuo pipefail

GO_VERSION="1.26.5"
DEFAULT_MQTT_USER="electricredit-module"

if [[ $EUID -ne 0 ]]; then
  echo "Run this installer through FIRST_TERMINAL_PROMPT.txt, or:" >&2
  echo "  sudo bash \"[BOOT] Scripts/install-autostart.sh\"" >&2
  exit 1
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
SERVER_DIR="$PROJECT_DIR/server"
CONFIG_FILE="$SCRIPT_DIR/hotspot.json"
RUN_USER="${SUDO_USER:-}"

if [[ -z "$RUN_USER" || "$RUN_USER" == "root" ]]; then
  # Safe fallback for a directly opened local root terminal.
  if [[ -n "${PKEXEC_UID:-}" ]] && command -v getent >/dev/null 2>&1; then
    RUN_USER="$(getent passwd "$PKEXEC_UID" | cut -d: -f1)"
  fi
fi
if [[ -z "$RUN_USER" || "$RUN_USER" == "root" ]]; then
  echo "Unable to determine the normal Raspberry Pi user. Run the command from FIRST_TERMINAL_PROMPT.txt." >&2
  exit 1
fi

RUN_GROUP="$(id -gn "$RUN_USER")"
RUN_HOME="$(getent passwd "$RUN_USER" | cut -d: -f6)"
[[ -n "$RUN_HOME" ]] || RUN_HOME="/home/$RUN_USER"

log() { printf '\n\033[1;36m[ElectriCredit]\033[0m %s\n' "$*"; }
fail() { printf '\n\033[1;31m[FAILED]\033[0m %s\n' "$*" >&2; exit 1; }

if [[ ! -f /etc/os-release ]]; then
  fail "This installer expects Raspberry Pi OS / Debian Linux."
fi
# shellcheck disable=SC1091
source /etc/os-release
case "${ID:-}" in
  raspbian|debian|ubuntu) ;;
  *) log "Detected ${PRETTY_NAME:-Linux}. Continuing because apt-get is available." ;;
esac
command -v apt-get >/dev/null 2>&1 || fail "apt-get was not found. Use Raspberry Pi OS (Debian based)."

log "Checking project files"
[[ -f "$SERVER_DIR/go.mod" ]] || fail "server/go.mod is missing. Copy the complete ElectriCredit folder first."
[[ -f "$CONFIG_FILE" ]] || fail "[BOOT] Scripts/hotspot.json is missing."
log "Installing Raspberry Pi system requirements"
export DEBIAN_FRONTEND=noninteractive
apt-get update || fail "Package index download failed. First setup needs Internet access."
apt-get install -y --no-install-recommends \
  ca-certificates curl tar gzip coreutils \
  python3 sqlite3 \
  network-manager iproute2 iw rfkill \
  mosquitto mosquitto-clients \
  sudo \
  || fail "Raspberry Pi system requirements could not be installed."

systemctl enable NetworkManager.service >/dev/null 2>&1 || true
systemctl start NetworkManager.service || fail "NetworkManager could not start."
rfkill unblock wifi >/dev/null 2>&1 || true

install_go() {
  local machine go_arch archive checksum url tmp actual current
  machine="$(uname -m)"
  case "$machine" in
    aarch64|arm64)
      go_arch="arm64"
      checksum="fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49"
      ;;
    armv7l|armv6l)
      # Official Go ARMv6 build also runs on ARMv7 Raspberry Pi OS 32-bit.
      go_arch="armv6l"
      checksum="6dae9edab81c13bccf962dec15f1fd2ec26c14a6821b4d2c92dab4130c289d7a"
      ;;
    x86_64|amd64)
      # Useful for development/testing even though deployment target is RPi4B.
      go_arch="amd64"
      checksum="5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"
      ;;
    *) fail "Unsupported CPU architecture for Go $GO_VERSION: $machine" ;;
  esac

  current=""
  if [[ -x /usr/local/go/bin/go ]]; then
    current="$(/usr/local/go/bin/go version 2>/dev/null || true)"
  elif command -v go >/dev/null 2>&1; then
    current="$(go version 2>/dev/null || true)"
  fi
  if [[ "$current" == *"go${GO_VERSION} "* ]]; then
    log "Go $GO_VERSION is already installed"
  else
    archive="go${GO_VERSION}.linux-${go_arch}.tar.gz"
    url="https://go.dev/dl/${archive}"
    tmp="/tmp/${archive}"
    log "Installing official Go $GO_VERSION for linux/$go_arch"
    rm -f "$tmp"
    curl -fL --retry 4 --retry-delay 2 --connect-timeout 20 -o "$tmp" "$url" \
      || fail "Could not download Go. Keep the Raspberry Pi online during first setup."
    actual="$(sha256sum "$tmp" | awk '{print $1}')"
    [[ "$actual" == "$checksum" ]] || fail "Go archive checksum did not match the official Go $GO_VERSION checksum."
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$tmp"
    rm -f "$tmp"
  fi

  ln -sfn /usr/local/go/bin/go /usr/local/bin/go
  ln -sfn /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  /usr/local/go/bin/go version
}
install_go

log "Validating hotspot configuration"
python3 - "$CONFIG_FILE" <<'PY'
import json, sys
p = sys.argv[1]
with open(p, encoding="utf-8") as f:
    c = json.load(f)
ssid = str(c.get("ssid") or "ElectriCredit Server").strip()
pw = str(c.get("password") or "1234567890")
iface = str(c.get("interface") or "wlan0").strip()
if not ssid or len(ssid.encode()) > 32:
    raise SystemExit("hotspot.json: SSID must be 1-32 bytes")
if not 8 <= len(pw) <= 63:
    raise SystemExit("hotspot.json: password must be 8-63 characters")
if not iface:
    raise SystemExit("hotspot.json: interface is required")
print(f"Hotspot profile OK: {ssid} on {iface}")
PY

log "Preparing ElectriCredit MQTT credentials and Mosquitto"
ENV_FILE="$SERVER_DIR/.env"
MQTT_STATE_DIR=/etc/electricredit
MQTT_ENV_FILE="$MQTT_STATE_DIR/mqtt.env"
mkdir -p "$MQTT_STATE_DIR"

# MQTT credentials are persisted outside the replaceable project folder. This
# keeps already-paired Hub/Registry modules working even when a new
# ElectriCredit folder is copied over the old one. On the first migration we
# reuse a password already generated in server/.env when one exists.
MQTT_PASSWORD="$(python3 - "$MQTT_ENV_FILE" "$ENV_FILE" <<'PYMQTT'
from pathlib import Path
import secrets, sys

def read_value(path, key):
    try:
        for raw in Path(path).read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, value = line.split("=", 1)
            if k.strip() == key and value.strip():
                return value.strip()
    except FileNotFoundError:
        pass
    return ""

key = "ELECTRICREDIT_MQTT_MODULE_PASSWORD"
value = read_value(sys.argv[1], key) or read_value(sys.argv[2], key)
print(value or secrets.token_urlsafe(24))
PYMQTT
)"
[[ -n "$MQTT_PASSWORD" ]] || fail "Unable to create an MQTT module password."

cat > "$MQTT_ENV_FILE" <<MQTTENV
# Managed by ElectriCredit [BOOT] Scripts/install-autostart.sh
# Persistent Raspberry Pi MQTT runtime settings. Keep this file private.
ELECTRICREDIT_MQTT_ENABLED=true
ELECTRICREDIT_MQTT_HOST=127.0.0.1
ELECTRICREDIT_MQTT_PUBLIC_HOST=
ELECTRICREDIT_MQTT_PORT=1883
ELECTRICREDIT_MQTT_TLS=false
ELECTRICREDIT_MQTT_CA=
ELECTRICREDIT_MQTT_MODULE_USER=$DEFAULT_MQTT_USER
ELECTRICREDIT_MQTT_MODULE_PASSWORD=$MQTT_PASSWORD
MQTTENV
chown root:"$RUN_GROUP" "$MQTT_ENV_FILE"
chmod 0640 "$MQTT_ENV_FILE"

# Keep the project .env synchronized for normal development/readability, while
# systemd uses /etc/electricredit/mqtt.env as the durable production source.
touch "$ENV_FILE"
chown "$RUN_USER:$RUN_GROUP" "$ENV_FILE"
chmod 0600 "$ENV_FILE"
python3 - "$ENV_FILE" "$DEFAULT_MQTT_USER" "$MQTT_PASSWORD" <<'PYENV'
from pathlib import Path
import sys
path = Path(sys.argv[1])
user, password = sys.argv[2], sys.argv[3]
updates = {
    "ELECTRICREDIT_MQTT_ENABLED": "true",
    "ELECTRICREDIT_MQTT_HOST": "127.0.0.1",
    "ELECTRICREDIT_MQTT_PUBLIC_HOST": "",
    "ELECTRICREDIT_MQTT_PORT": "1883",
    "ELECTRICREDIT_MQTT_TLS": "false",
    "ELECTRICREDIT_MQTT_CA": "",
    "ELECTRICREDIT_MQTT_MODULE_USER": user,
    "ELECTRICREDIT_MQTT_MODULE_PASSWORD": password,
}
lines = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
out, seen = [], set()
for line in lines:
    stripped = line.strip()
    key = stripped.split("=", 1)[0] if "=" in stripped and not stripped.startswith("#") else ""
    if key in updates:
        if key not in seen:
            out.append(f"{key}={updates[key]}")
            seen.add(key)
        # Drop duplicate MQTT key definitions so .env stays deterministic.
    else:
        out.append(line)
if out and out[-1] != "":
    out.append("")
if not all(k in seen for k in updates):
    out.append("# ESP32 work-mode messaging (managed by first-install.sh)")
    for key, value in updates.items():
        if key not in seen:
            out.append(f"{key}={value}")
path.write_text("\n".join(out).rstrip() + "\n", encoding="utf-8")
PYENV
chown "$RUN_USER:$RUN_GROUP" "$ENV_FILE"
chmod 0600 "$ENV_FILE"

MOSQ_PASSWD=/etc/mosquitto/passwd-electricredit
MOSQ_CONF=/etc/mosquitto/conf.d/electricredit.conf
mkdir -p /etc/mosquitto/conf.d
rm -f "$MOSQ_PASSWD"
mosquitto_passwd -b -c "$MOSQ_PASSWD" "$DEFAULT_MQTT_USER" "$MQTT_PASSWORD" >/dev/null
chown root:mosquitto "$MOSQ_PASSWD"
chmod 0640 "$MOSQ_PASSWD"

# Keep this fragment listener/authentication-only. Raspberry Pi OS/Debian's
# main mosquitto.conf already owns persistence settings. Re-declaring
# persistence_location here makes Mosquitto refuse to start.
cat > "$MOSQ_CONF" <<MOSQ
# Managed by ElectriCredit [BOOT] Scripts/install-autostart.sh
# Local LAN/hotspot broker for ESP32 Hub and Registry modules.
listener 1883 0.0.0.0
allow_anonymous false
password_file $MOSQ_PASSWD
MOSQ
chmod 0644 "$MOSQ_CONF"

systemctl enable mosquitto.service >/dev/null
if ! systemctl restart mosquitto.service; then
  journalctl -u mosquitto.service -n 80 --no-pager || true
  fail "Mosquitto configuration could not start."
fi

# Authenticated broker round-trip test. No credential is echoed.
MQTT_TEST_TOPIC="electricredit/install/$(date +%s)-$$"
MQTT_TEST_OUT="/tmp/electricredit-mqtt-test.$$"
rm -f "$MQTT_TEST_OUT"
cleanup_mqtt_test() { rm -f "$MQTT_TEST_OUT"; }
trap cleanup_mqtt_test EXIT

timeout 8 mosquitto_sub -h 127.0.0.1 -p 1883 -u "$DEFAULT_MQTT_USER" -P "$MQTT_PASSWORD" -q 1 -t "$MQTT_TEST_TOPIC" -C 1 >"$MQTT_TEST_OUT" &
SUB_PID=$!
sleep 0.5
if ! mosquitto_pub -h 127.0.0.1 -p 1883 -u "$DEFAULT_MQTT_USER" -P "$MQTT_PASSWORD" -q 1 -t "$MQTT_TEST_TOPIC" -m "ok"; then
  kill "$SUB_PID" >/dev/null 2>&1 || true
  wait "$SUB_PID" >/dev/null 2>&1 || true
  fail "Mosquitto is running but authenticated MQTT publish failed."
fi
wait "$SUB_PID" || fail "Mosquitto installed but the authenticated MQTT subscribe test failed."
grep -qx 'ok' "$MQTT_TEST_OUT" || fail "Mosquitto MQTT test returned unexpected data."
rm -f "$MQTT_TEST_OUT"
trap - EXIT

# The local broker must reject unauthenticated clients.
if mosquitto_pub -h 127.0.0.1 -p 1883 -t "electricredit/install/anonymous-check" -m "blocked" >/dev/null 2>&1; then
  fail "Mosquitto accepted an anonymous MQTT client; authentication is not being enforced."
fi

log "Downloading Go modules and testing the server build"
chown -R "$RUN_USER:$RUN_GROUP" "$PROJECT_DIR"
chmod 0600 "$ENV_FILE" "$CONFIG_FILE"
chmod +x "$SCRIPT_DIR"/*.sh

GO_CMD=/usr/local/go/bin/go
run_go() {
  sudo -u "$RUN_USER" env \
    HOME="$RUN_HOME" \
    PATH="/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin" \
    GOTOOLCHAIN=local \
    "$GO_CMD" "$@"
}

cd "$SERVER_DIR"
run_go mod download || fail "Go module download failed. Keep the Raspberry Pi online during first setup."
run_go mod verify || fail "Go module verification failed."
run_go test ./... || fail "ElectriCredit Go tests/build checks failed."
run_go build -trimpath -o electricredit . || fail "ElectriCredit server build failed."
chown "$RUN_USER:$RUN_GROUP" "$SERVER_DIR/electricredit"
chmod 0755 "$SERVER_DIR/electricredit"

log "Initializing/updating the SQLite database"
sudo -u "$RUN_USER" env \
  HOME="$RUN_HOME" \
  PATH="/usr/local/bin:/usr/bin:/bin" \
  ELECTRICREDIT_ADDR=":8080" \
  "$SERVER_DIR/electricredit" -init-db

log "Installing narrow NetworkManager privilege for the web Network page"
NMCLI_PATH="$(command -v nmcli)"
SUDOERS_FILE=/etc/sudoers.d/electricredit-network
printf '%s ALL=(root) NOPASSWD: %s *\n' "$RUN_USER" "$NMCLI_PATH" > "$SUDOERS_FILE"
chmod 0440 "$SUDOERS_FILE"
visudo -cf "$SUDOERS_FILE" >/dev/null

# If this is an older Raspberry Pi OS still running dhcpcd, stop it only now,
# after all internet downloads are complete, so NetworkManager exclusively owns
# wlan0 for reliable AP <-> Wi-Fi switching.
if systemctl list-unit-files dhcpcd.service >/dev/null 2>&1; then
  if systemctl is-active --quiet dhcpcd.service; then
    log "Switching legacy dhcpcd networking to NetworkManager"
    systemctl disable --now dhcpcd.service || true
    systemctl restart NetworkManager.service
  fi
fi

log "Installing ElectriCredit systemd services"
HOTSPOT_UNIT=/etc/systemd/system/electricredit-hotspot.service
SERVER_UNIT=/etc/systemd/system/electricredit.service

cat > "$HOTSPOT_UNIT" <<UNIT
[Unit]
Description=ElectriCredit local hotspot
Wants=NetworkManager.service
After=NetworkManager.service
Before=electricredit.service

[Service]
Type=oneshot
Environment="ELECTRICREDIT_HOTSPOT_CONFIG=$CONFIG_FILE"
ExecStart=/bin/bash "$SCRIPT_DIR/start-hotspot.sh"
RemainAfterExit=yes
TimeoutStartSec=45

[Install]
WantedBy=multi-user.target
UNIT

cat > "$SERVER_UNIT" <<UNIT
[Unit]
Description=ElectriCredit local server
Wants=electricredit-hotspot.service mosquitto.service network.target
After=electricredit-hotspot.service mosquitto.service network.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_GROUP
WorkingDirectory=$SERVER_DIR
Environment=HOME=$RUN_HOME
Environment=PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin
Environment=ELECTRICREDIT_ADDR=:8080
EnvironmentFile=-/etc/electricredit/mqtt.env
Environment="ELECTRICREDIT_HOTSPOT_CONFIG=$CONFIG_FILE"
ExecStart=$SERVER_DIR/electricredit
Restart=on-failure
RestartSec=3
TimeoutStopSec=10

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable electricredit-hotspot.service electricredit.service mosquitto.service >/dev/null

log "Starting ElectriCredit locally"
systemctl restart mosquitto.service
systemctl restart electricredit-hotspot.service
systemctl restart electricredit.service

# Wait for the HTTP listener rather than declaring success immediately.
SERVER_OK=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 2 http://127.0.0.1:8080/api/public/home >/dev/null 2>&1; then
    SERVER_OK=1
    break
  fi
  sleep 1
done
if [[ "$SERVER_OK" != "1" ]]; then
  systemctl --no-pager --full status electricredit.service || true
  journalctl -u electricredit.service -n 80 --no-pager || true
  fail "The ElectriCredit service did not pass its local HTTP health check."
fi

log "Running final installation verification"
sudo -u "$RUN_USER" env HOME="$RUN_HOME" "$SCRIPT_DIR/verify-install.sh"

HOTSPOT_SSID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["ssid"])' "$CONFIG_FILE")"
HOTSPOT_IP="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("ipv4_address","10.42.0.1/24").split("/",1)[0])' "$CONFIG_FILE")"

printf '\n\033[1;32m============================================================\033[0m\n'
printf '\033[1;32m ElectriCredit Raspberry Pi setup completed successfully.\033[0m\n'
printf '\033[1;32m============================================================\033[0m\n'
printf 'Hotspot SSID : %s\n' "$HOTSPOT_SSID"
printf 'Server URL   : http://%s:8080/\n' "$HOTSPOT_IP"
printf 'Config file  : %s\n' "$CONFIG_FILE"
printf 'Service      : electricredit.service\n'
printf 'Hotspot      : electricredit-hotspot.service\n'
printf 'MQTT broker  : mosquitto.service\n'
printf '\nThe hotspot password is intentionally not printed.\n'
printf 'You can view/edit it locally in [BOOT] Scripts/hotspot.json.\n\n'
