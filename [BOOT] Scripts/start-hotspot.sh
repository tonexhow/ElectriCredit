#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${ELECTRICREDIT_HOTSPOT_CONFIG:-$SCRIPT_DIR/hotspot.json}"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "ElectriCredit hotspot config not found: $CONFIG_FILE" >&2
  exit 1
fi
command -v nmcli >/dev/null 2>&1 || { echo "NetworkManager/nmcli is required." >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "python3 is required." >&2; exit 1; }

readarray -t HOTSPOT_CFG < <(python3 - "$CONFIG_FILE" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    cfg = json.load(fh)
for value in [
    "1" if cfg.get("boot_enabled", True) else "0",
    str(cfg.get("ssid") or "ElectriCredit Server"),
    str(cfg.get("password") or "1234567890"),
    str(cfg.get("interface") or "wlan0"),
    str(cfg.get("connection_name") or "ElectriCredit Hotspot"),
    str(cfg.get("ipv4_address") or "10.42.0.1/24"),
]:
    print(value)
PY
)

BOOT_ENABLED="${HOTSPOT_CFG[0]:-1}"
SSID="${HOTSPOT_CFG[1]:-ElectriCredit Server}"
PASSWORD="${HOTSPOT_CFG[2]:-1234567890}"
INTERFACE="${HOTSPOT_CFG[3]:-wlan0}"
CONNECTION_NAME="${HOTSPOT_CFG[4]:-ElectriCredit Hotspot}"
IPV4_ADDRESS="${HOTSPOT_CFG[5]:-10.42.0.1/24}"

if [[ "$BOOT_ENABLED" != "1" ]]; then
  echo "ElectriCredit hotspot boot is disabled in $CONFIG_FILE"
  exit 0
fi
if [[ -z "$SSID" || ${#SSID} -gt 32 ]]; then
  echo "Hotspot SSID must contain 1 to 32 characters." >&2
  exit 1
fi
if (( ${#PASSWORD} < 8 || ${#PASSWORD} > 63 )); then
  echo "Hotspot password must contain 8 to 63 characters." >&2
  exit 1
fi

# RPi Wi-Fi can remain soft-blocked after imaging or a previous radio toggle.
rfkill unblock wifi >/dev/null 2>&1 || true
nmcli radio wifi on >/dev/null 2>&1 || true

# Give NetworkManager a short window to expose wlan0 after boot.
for _ in $(seq 1 20); do
  if nmcli -t -f DEVICE,TYPE device status | grep -Eq "^${INTERFACE}:wifi$|^${INTERFACE}:802-11-wireless$"; then
    break
  fi
  sleep 0.5
done
if ! ip link show "$INTERFACE" >/dev/null 2>&1; then
  echo "Wireless interface '$INTERFACE' was not found. Check hotspot.json." >&2
  exit 1
fi

if nmcli -t -f NAME connection show | grep -Fxq "$CONNECTION_NAME"; then
  nmcli connection modify "$CONNECTION_NAME" \
    connection.interface-name "$INTERFACE" \
    connection.autoconnect no \
    802-11-wireless.ssid "$SSID" \
    802-11-wireless.mode ap \
    802-11-wireless.band bg \
    802-11-wireless.channel 6 \
    wifi-sec.key-mgmt wpa-psk \
    wifi-sec.proto rsn \
    wifi-sec.pairwise ccmp \
    wifi-sec.group ccmp \
    wifi-sec.pmf disable \
    wifi-sec.psk "$PASSWORD" \
    ipv4.method shared \
    ipv4.addresses "$IPV4_ADDRESS" \
    ipv6.method disabled
else
  nmcli connection add type wifi ifname "$INTERFACE" con-name "$CONNECTION_NAME" ssid "$SSID"
  nmcli connection modify "$CONNECTION_NAME" \
    connection.autoconnect no \
    802-11-wireless.mode ap \
    802-11-wireless.band bg \
    802-11-wireless.channel 6 \
    wifi-sec.key-mgmt wpa-psk \
    wifi-sec.proto rsn \
    wifi-sec.pairwise ccmp \
    wifi-sec.group ccmp \
    wifi-sec.pmf disable \
    wifi-sec.psk "$PASSWORD" \
    ipv4.method shared \
    ipv4.addresses "$IPV4_ADDRESS" \
    ipv6.method disabled
fi

# A single wlan0 is intentionally transferred into AP mode. NetworkManager will
# disconnect any normal Wi-Fi association on that interface as needed.
nmcli connection up "$CONNECTION_NAME" ifname "$INTERFACE"

echo "ElectriCredit hotspot started: $SSID ($IPV4_ADDRESS) on $INTERFACE"
