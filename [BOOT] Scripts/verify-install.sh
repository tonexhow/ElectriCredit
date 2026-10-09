#!/usr/bin/env bash
set -u

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$SCRIPT_DIR/hotspot.json"
PASS=0
FAIL=0
WARN=0

pass(){ printf '[PASS] %s\n' "$*"; PASS=$((PASS+1)); }
fail(){ printf '[FAIL] %s\n' "$*"; FAIL=$((FAIL+1)); }
warn(){ printf '[WARN] %s\n' "$*"; WARN=$((WARN+1)); }

[[ -f "$PROJECT_DIR/server/electricredit" && -x "$PROJECT_DIR/server/electricredit" ]] && pass "Go server binary exists" || fail "server/electricredit is missing"
[[ -f "$CONFIG_FILE" ]] && pass "hotspot.json exists" || fail "hotspot.json is missing"

if command -v go >/dev/null 2>&1; then
  go version | grep -q 'go1.26.5 ' && pass "Go 1.26.5 installed" || warn "Go exists but is not 1.26.5: $(go version 2>/dev/null)"
else
  fail "Go is not installed"
fi
command -v nmcli >/dev/null 2>&1 && pass "NetworkManager nmcli available" || fail "nmcli missing"
command -v mosquitto_pub >/dev/null 2>&1 && command -v mosquitto_sub >/dev/null 2>&1 && pass "Mosquitto clients available" || fail "Mosquitto clients missing"

MQTT_ENV_FILE=/etc/electricredit/mqtt.env
MQTT_USER=""
MQTT_PASSWORD=""
if [[ -r "$MQTT_ENV_FILE" ]]; then
  while IFS='=' read -r key value; do
    case "$key" in
      ELECTRICREDIT_MQTT_MODULE_USER) MQTT_USER="$value" ;;
      ELECTRICREDIT_MQTT_MODULE_PASSWORD) MQTT_PASSWORD="$value" ;;
    esac
  done < "$MQTT_ENV_FILE"
  [[ -n "$MQTT_USER" && -n "$MQTT_PASSWORD" ]] && pass "Persistent MQTT credentials available" || fail "Persistent MQTT credentials are incomplete"
else
  fail "Persistent MQTT credential file is missing or unreadable"
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl is-enabled --quiet electricredit.service && pass "ElectriCredit autostart enabled" || fail "electricredit.service not enabled"
  systemctl is-enabled --quiet electricredit-hotspot.service && pass "Hotspot autostart enabled" || fail "electricredit-hotspot.service not enabled"
  systemctl is-enabled --quiet mosquitto.service && pass "Mosquitto autostart enabled" || fail "mosquitto.service not enabled"
  systemctl is-active --quiet electricredit.service && pass "ElectriCredit service running" || fail "electricredit.service not running"
  systemctl is-active --quiet mosquitto.service && pass "Mosquitto service running" || fail "mosquitto.service not running"
fi

if [[ -n "$MQTT_USER" && -n "$MQTT_PASSWORD" ]] && command -v mosquitto_pub >/dev/null 2>&1 && command -v mosquitto_sub >/dev/null 2>&1; then
  MQTT_VERIFY_TOPIC="electricredit/verify/$(date +%s)-$$"
  MQTT_VERIFY_OUT="/tmp/electricredit-mqtt-verify.$$"
  rm -f "$MQTT_VERIFY_OUT"
  timeout 6 mosquitto_sub -h 127.0.0.1 -p 1883 -u "$MQTT_USER" -P "$MQTT_PASSWORD" -q 1 -t "$MQTT_VERIFY_TOPIC" -C 1 >"$MQTT_VERIFY_OUT" 2>/dev/null &
  MQTT_SUB_PID=$!
  sleep 0.4
  if mosquitto_pub -h 127.0.0.1 -p 1883 -u "$MQTT_USER" -P "$MQTT_PASSWORD" -q 1 -t "$MQTT_VERIFY_TOPIC" -m "ok" >/dev/null 2>&1 \
    && wait "$MQTT_SUB_PID" >/dev/null 2>&1 \
    && grep -qx 'ok' "$MQTT_VERIFY_OUT"; then
    pass "Authenticated MQTT publish/subscribe round-trip works"
  else
    kill "$MQTT_SUB_PID" >/dev/null 2>&1 || true
    wait "$MQTT_SUB_PID" >/dev/null 2>&1 || true
    fail "Authenticated MQTT publish/subscribe round-trip failed"
  fi
  rm -f "$MQTT_VERIFY_OUT"

  if mosquitto_pub -h 127.0.0.1 -p 1883 -t "$MQTT_VERIFY_TOPIC/anonymous" -m "blocked" >/dev/null 2>&1; then
    fail "Mosquitto accepted an anonymous client"
  else
    pass "Mosquitto rejects anonymous clients"
  fi
fi

if curl -fsS --max-time 3 http://127.0.0.1:8080/api/public/home >/dev/null 2>&1; then
  pass "HTTP API responds locally on port 8080"
else
  fail "HTTP API did not respond on 127.0.0.1:8080"
fi

if python3 - "$CONFIG_FILE" >/dev/null 2>&1 <<'PY'
import json,sys
c=json.load(open(sys.argv[1]))
assert c.get('ssid')
assert 8 <= len(c.get('password','')) <= 63
assert c.get('interface')
PY
then
  pass "hotspot.json is valid"
else
  fail "hotspot.json is invalid"
fi

if command -v nmcli >/dev/null 2>&1; then
  readarray -t HC < <(python3 - "$CONFIG_FILE" <<'PY'
import json,sys
c=json.load(open(sys.argv[1]))
print('1' if c.get('boot_enabled', True) else '0')
print(c.get('connection_name') or 'ElectriCredit Hotspot')
print(c.get('interface') or 'wlan0')
print((c.get('ipv4_address') or '10.42.0.1/24').split('/',1)[0])
PY
)
  if [[ "${HC[0]:-0}" == "1" ]]; then
    if nmcli -t -f NAME connection show --active 2>/dev/null | grep -Fxq "${HC[1]}"; then
      pass "ElectriCredit hotspot profile is active"
    else
      fail "ElectriCredit hotspot profile is not active"
    fi
    if ip -4 addr show "${HC[2]}" 2>/dev/null | grep -Fq "${HC[3]}/"; then
      pass "Hotspot interface has configured local IP ${HC[3]}"
    else
      fail "Hotspot interface does not have configured local IP ${HC[3]}"
    fi

    HOTSPOT_BAND="$(nmcli -g 802-11-wireless.band connection show "${HC[1]}" 2>/dev/null || true)"
    HOTSPOT_CHANNEL="$(nmcli -g 802-11-wireless.channel connection show "${HC[1]}" 2>/dev/null || true)"
    HOTSPOT_PROTO="$(nmcli -g 802-11-wireless-security.proto connection show "${HC[1]}" 2>/dev/null || true)"
    HOTSPOT_PAIRWISE="$(nmcli -g 802-11-wireless-security.pairwise connection show "${HC[1]}" 2>/dev/null || true)"
    if [[ "$HOTSPOT_BAND" == "bg" && "$HOTSPOT_CHANNEL" == "6" && "$HOTSPOT_PROTO" == *"rsn"* && "$HOTSPOT_PAIRWISE" == *"ccmp"* ]]; then
      pass "Hotspot radio/security is ESP32-safe: 2.4 GHz channel 6, WPA2/RSN + CCMP"
    else
      fail "Hotspot profile is not pinned to ESP32-safe 2.4 GHz channel 6 + WPA2/RSN/CCMP"
    fi
  fi
fi

printf '\nVerification: %d passed, %d warning(s), %d failed.\n' "$PASS" "$WARN" "$FAIL"
[[ "$FAIL" -eq 0 ]]
