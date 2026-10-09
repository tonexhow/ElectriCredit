#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
SERVER_DIR="$PROJECT_DIR/server"
CONFIG_FILE="$SCRIPT_DIR/hotspot.json"

export ELECTRICREDIT_ADDR="${ELECTRICREDIT_ADDR:-:8080}"
export ELECTRICREDIT_HOTSPOT_CONFIG="${ELECTRICREDIT_HOTSPOT_CONFIG:-$CONFIG_FILE}"

# Load the Raspberry Pi MQTT credentials kept outside the replaceable project
# folder. Existing shell variables still win when explicitly supplied.
MQTT_ENV_FILE=/etc/electricredit/mqtt.env
if [[ -r "$MQTT_ENV_FILE" ]]; then
  while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" == \#* ]] && continue
    case "$key" in
      ELECTRICREDIT_MQTT_*)
        if [[ -z "${!key+x}" ]]; then
          export "$key=$value"
        fi
        ;;
    esac
  done < "$MQTT_ENV_FILE"
fi

if [[ "${ELECTRICREDIT_SKIP_HOTSPOT:-0}" != "1" ]]; then
  if [[ $EUID -eq 0 ]]; then
    "$SCRIPT_DIR/start-hotspot.sh"
  else
    sudo ELECTRICREDIT_HOTSPOT_CONFIG="$ELECTRICREDIT_HOTSPOT_CONFIG" "$SCRIPT_DIR/start-hotspot.sh"
  fi
fi

# The server's MQTT bridge expects the local Mosquitto broker when enabled.
if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files mosquitto.service >/dev/null 2>&1; then
  sudo systemctl start mosquitto.service >/dev/null
fi

cd "$SERVER_DIR"
if [[ -x "$SERVER_DIR/electricredit" ]]; then
  exec "$SERVER_DIR/electricredit"
fi
if [[ -x /usr/local/go/bin/go ]]; then
  exec /usr/local/go/bin/go run .
fi
if command -v go >/dev/null 2>&1; then
  exec go run .
fi

echo "No ElectriCredit server binary was found and Go is not installed." >&2
echo "Run the command in FIRST_TERMINAL_PROMPT.txt first." >&2
exit 1
