# ElectriCredit Registry - Standalone Firmware 3.3.0

Use `module/registry/registry.ino`. Registration Mode, Work Mode, Wi-Fi recovery, server authentication, MQTT/HTTP transport, OLED, RFID, button and buzzer support are embedded in this single file.

## Common connection behavior

Registry uses the same connection core as the other module:

- load saved pairing/Wi-Fi;
- scan for the saved SSID;
- when visible, connect using the pre-oral proven `WiFi.begin(ssid,password)` method;
- retry scan/join indefinitely after failure;
- validate the Raspberry Pi HTTP runtime API first;
- start MQTT only after server readiness is confirmed;
- never auto-open Registration Mode because Wi-Fi is unavailable.

There is no dedicated user FreeRTOS Wi-Fi task and no GPIO interrupt-driven mode state machine. One Arduino-loop owner controls Wi-Fi and mode transitions, avoiding AP/STA races.

## GPIO33 mode button

- Press: one tone beep.
- Hold 3 seconds in Work Mode: safe transition request to Registration Mode.
- Hold 3 seconds in Registration Mode: return to Work Mode when paired.
- Successful transition: two beeps.

Serial Monitor at 115200 prints `[EC][BUTTON]`, `[EC][MODE]`, and `[EC][DIAG]` messages to make the exact state visible.

## Brownout safety

The existing Registry alternating top-up journal is retained. Active work is journaled before changing to Registration Mode, and the cache is preserved when the server cannot be reached.

## Dependencies

Install ArduinoJson, ArduinoMqttClient, MFRC522, Adafruit GFX and Adafruit SSD1306, plus the ESP32 Arduino core. No custom ElectriCredit library is required.
