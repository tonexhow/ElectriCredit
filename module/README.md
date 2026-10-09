# ElectriCredit ESP32 Modules

ElectriCredit uses **standalone one-file firmware**. There is no custom `ElectriCreditShared` library and no `module/libraries/` folder.

```text
module/
├── README.md
├── hub/
│   ├── hub.ino
│   └── hub_instruction.md
└── registry/
    ├── registry.ino
    └── registry_instruction.md
```

## Common boot and connection behavior

Hub and Registry contain the same embedded connection core. The Work Mode connection path is intentionally based on the proven pre-oral firmware rather than a second FreeRTOS networking task.

1. Load saved pairing, Wi-Fi and module credentials from NVS.
2. If there is no pairing/saved SSID, open Registration Mode.
3. Otherwise use Work Mode and scan asynchronously for the saved SSID.
4. If absent, wait briefly and scan again forever.
5. When visible, call the proven `WiFi.begin(ssid, password)` join path. No forced BSSID/channel is used.
6. If joining fails/times out, disconnect and return to scanning.
7. After Wi-Fi obtains an IP, verify the Raspberry Pi through the HTTP runtime heartbeat first.
8. Only after HTTP server readiness is confirmed is MQTT started.
9. Work Mode never automatically opens Registration Mode because the server/hotspot is temporarily missing.

All Wi-Fi/AP/STA changes are made by the Arduino loop only. This avoids the race created when one task scanned/joined Wi-Fi while another task tried to open the Registration AP.

### Pre-oral compatibility

The pre-oral Raspberry Pi hotspot was normally open and served Flask on port 5000. The current project uses `ElectriCredit Server`, WPA2 password `1234567890`, and Go on port 8080. If a module still has an old open-hotspot credential for the exact `ElectriCredit Server` SSID, firmware 3.3.0 makes one compatibility attempt using the current default WPA2 password and saves it only after a successful join. Runtime HTTP also prefers/falls back to the current Wi-Fi gateway on port 8080.

## Button and buzzer

GPIO33 is identical on Hub and Registry:

- Press: one real tone beep.
- Hold 3 seconds in Work Mode: request safe Registration Mode.
- Hold 3 seconds in Registration Mode: return to Work Mode when paired and Wi-Fi is saved.
- Successful mode change: two beeps.

The button uses the same simple debounced polling pattern as the old working firmware. Networking is non-blocking while Wi-Fi is unavailable, and authorized local HTTP timeouts are kept below the 3-second hold window.

## Brownout recovery

- **Hub:** alternating `ec-hub-run2` session records remain unchanged. Relay power stays OFF after reboot until recovery is reconciled.
- **Registry:** alternating top-up records remain unchanged so interrupted top-ups can be resumed/reconciled safely.

Do not erase NVS when an interrupted session/top-up must recover.

## Arduino dependencies

Install the normal libraries used directly by each standalone sketch:

- ESP32 Arduino core
- ArduinoJson
- ArduinoMqttClient
- MFRC522
- Adafruit GFX Library
- Adafruit SSD1306
- PZEM004Tv30 (Hub only)

No ElectriCredit-specific Arduino library is required.

## 3.3.0 boot/power diagnostics

Firmware 3.3.0 keeps startup transaction hardware quiet and gives the 5 V rail a short settling period before Wi-Fi activity. The OLED and Serial Monitor report the ESP32 reset reason at boot (`POWER ON`, `BROWNOUT`, `WDT`, `PANIC`, etc.). If the boot screen itself repeats, the ESP32 is resetting rather than merely retrying Wi-Fi.

OLED ownership is also serialized: during Wi-Fi/server recovery, only the embedded connection core writes connection status. Work-mode transaction logic waits until the Runtime API is accepted. Connection/status text must remain stable briefly before it is flushed to the SSD1306, which removes rapid screen fighting.

Holding GPIO33 for 3 seconds while powering the module provides a boot-time Registration Mode override before Wi-Fi scanning begins. The normal 3-second Work/Registration mode switch remains available after startup.

Repeated MQTT admin commands are de-duplicated and an already playing buzzer pattern cannot be continuously restarted by a fast loop. Session/top-up brownout journals are unchanged.
