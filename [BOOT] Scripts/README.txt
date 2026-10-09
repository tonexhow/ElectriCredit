ELECTRICREDIT — RASPBERRY PI 4B FIRST START
============================================

EXPECTED PROJECT LOCATION
-------------------------
Copy the complete ElectriCredit folder from your USB flash drive into the
Raspberry Pi user's Home folder.

Examples:
  /home/tony/ElectriCredit
  /home/pi/ElectriCredit
  /home/<your-user>/ElectriCredit

You do NOT need to edit the scripts for the username. They determine the
project location from the script itself and the service user from sudo.

FIRST TIME ONLY
---------------
1. The Raspberry Pi MUST have Internet access for this first installation.
   Ethernet is preferred because the installer later turns wlan0 into the
   ElectriCredit hotspot. Normal Wi-Fi is also fine for the download stage.

2. Open FIRST_TERMINAL_PROMPT.txt in the ElectriCredit folder.

3. Copy its single command and paste it into Raspberry Pi Terminal.

The installer automatically installs/configures:
  - NetworkManager / nmcli
  - Python 3
  - SQLite CLI
  - Mosquitto MQTT broker
  - Mosquitto command-line clients
  - Wi-Fi support utilities
  - Official Go 1.26.5 (verified SHA-256)
  - Go module dependencies from server/go.mod
  - ElectriCredit Go server build
  - SQLite schema initialization/update
  - MQTT authentication for ESP32 modules
  - systemd automatic startup
  - ElectriCredit hotspot startup
  - local HTTP and MQTT verification

No npm, Node.js, Python pip requirements, or frontend build step are required.
The client is served as static HTML/CSS/JavaScript by the Go server.

DEFAULT LOCAL NETWORK
---------------------
SSID:       ElectriCredit Server
Password:   1234567890
Server:     http://10.42.0.1:8080/
Interface:  wlan0

The readable source of truth is:
  [BOOT] Scripts/hotspot.json

Management -> Network edits this same JSON configuration.
If you forget the hotspot password, open hotspot.json locally on the RPi.

AUTOSTART SERVICES
------------------
  electricredit-hotspot.service
  electricredit.service
  mosquitto.service

Useful commands:
  systemctl status electricredit-hotspot.service
  systemctl status electricredit.service
  systemctl status mosquitto.service

  journalctl -u electricredit-hotspot.service -n 100 --no-pager
  journalctl -u electricredit.service -n 100 --no-pager
  journalctl -u mosquitto.service -n 100 --no-pager

Run the installation checker again at any time:
  cd "$HOME/ElectriCredit"
  bash "[BOOT] Scripts/verify-install.sh"

MANUAL LOCAL START
------------------
After the first installation, if you intentionally stopped the systemd
services and want to run locally from Terminal:

  cd "$HOME/ElectriCredit"
  bash "[BOOT] Scripts/start-local.sh"

IMPORTANT
---------
- Do not delete server/.env after installation. The installer stores the local
  MQTT module credential there without printing the password.
- Do not expose MQTT port 1883 directly to the public Internet. This setup is
  designed for the Raspberry Pi LAN/hotspot. If you later move MQTT online,
  configure TLS and firewall rules first.
- hotspot.json is intentionally readable/editable on the Raspberry Pi desktop,
  but the installer sets it to owner-only permissions because it contains the
  hotspot password.
