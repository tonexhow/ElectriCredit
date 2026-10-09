/*
  ElectriCredit Hub - Standalone Firmware 3.3.0
  One file contains Registration Mode, common connection logic and Hub Work Mode.
  Active electricity sessions keep the existing alternating NVS brownout journal.
*/

/*
  ---------------------------------------------------------------------------
  ElectriCredit Embedded Module Core 3.3.0
  ---------------------------------------------------------------------------
  This infrastructure is intentionally embedded in this one .ino file.
  No ElectriCreditShared custom Arduino library is required.
  Hub and Registry carry the same connection/registration core so their
  boot, Wi-Fi recovery, server authentication and mode button behavior match.
*/

#include <Arduino.h>
#include <esp_system.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <WebServer.h>
#include <DNSServer.h>
#include <SPI.h>
#include <Wire.h>
#include <MFRC522.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>
#include <ArduinoMqttClient.h>
#include <ArduinoJson.h>

// -----------------------------------------------------------------------------
// ElectriCredit embedded module core
// Common module infrastructure for Hub and Registry ESP32 firmware.
//
// ModuleCore responsibilities:
// - fixed common wiring (OLED, RC522, button, buzzer)
// - Registration Mode captive portal (AP only; never AP+STA during probing)
// - saved Wi-Fi / API base URL / API paths / hotspot name
// - module pairing and server identity storage
// - Wi-Fi work-mode reconnect
// - MQTT connection, QoS 1 request/response and PING/TERMINATE command intake
// - HTTP runtime fallback, including HTTP and HTTPS URLs
// - common OLED, RFID and non-blocking buzzer helpers
// - fixed-point money formatting
//
// The Hub and Registry sketches keep only their different working-mode logic.
// -----------------------------------------------------------------------------

namespace ElectriCredit {

static constexpr int64_t MONEY_SCALE = 100000LL; // P1.00000 = 100000, P0.00001 = 1
static constexpr const char* EMBEDDED_CORE_VERSION = "3.3.0";
static constexpr const char* DEFAULT_SERVER_URL = "http://10.42.0.1:8080";
static constexpr const char* DEFAULT_CONNECT_PATH = "/api/modules/connect";
static constexpr const char* DEFAULT_RUNTIME_PATH = "/api/modules/runtime";
static constexpr const char* DEFAULT_AP_PASSWORD = "1234567890";

// Frozen common pins from the proven old build.
static constexpr uint8_t OLED_SDA_PIN = 21;
static constexpr uint8_t OLED_SCL_PIN = 22;
static constexpr uint8_t RFID_SS_PIN = 5;
static constexpr uint8_t RFID_SCK_PIN = 18;
static constexpr uint8_t RFID_MISO_PIN = 19;
static constexpr uint8_t RFID_MOSI_PIN = 23;
static constexpr uint8_t RFID_RST_PIN = 27;
static constexpr uint8_t BUTTON_PIN = 33;
static constexpr uint8_t BUZZER_PIN = 26;

enum class Mode : uint8_t {
  Registration = 0,
  Work = 1,
};

struct CommonSettings {
  const char* moduleType = "module";       // "hub" or "registry"
  const char* firmwareVersion = "unknown";
  const char* preferencesNamespace = "ec-module"; // <= 15 chars for Preferences namespace
  const char* defaultHotspotPrefix = "ElectriCredit";
  const char* boardName = "ESP32 38-pin / Goouuu V4";
};

struct ModuleConfig {
  bool paired = false;
  Mode preferredMode = Mode::Registration;

  String ssid;
  String wifiPassword;
  String serverUrl = DEFAULT_SERVER_URL;
  String connectPath = DEFAULT_CONNECT_PATH;
  String runtimePath = DEFAULT_RUNTIME_PATH;
  String hotspotName;
  String hotspotPassword = DEFAULT_AP_PASSWORD;

  int64_t moduleId = 0;
  String moduleToken;
  String moduleName;
  String moduleLocation;
  String moduleStatus;

  bool mqttEnabled = true;
  bool mqttTls = false;
  String mqttHost;
  uint16_t mqttPort = 1883;
  String mqttUser;
  String mqttPassword;
  String eventTopic;
  String commandTopic;

  // Human-readable registration diagnostics (not financial/session state).
  bool lastProbeOk = false;
  String lastProbeMessage;
  String lastWifi;
  String lastIp;
};

struct PendingCommand {
  bool pending = false;
  String type;              // ping | terminate | rfid-scan
  String commandId;
  int64_t targetSessionId = 0;
  String reason;
  String scanId;
  uint32_t timeoutMs = 0;
};

class ModuleCore {
 public:
  ModuleCore();
  ~ModuleCore();

  bool begin(const CommonSettings& settings);
  void loop();

  // Mode / registration portal ------------------------------------------------
  Mode mode() const { return _mode; }
  bool inRegistrationMode() const { return _mode == Mode::Registration; }
  bool inWorkMode() const { return _mode == Mode::Work; }
  bool enterRegistrationMode();
  bool enterWorkMode();
  bool takeRegistrationRequest(); // long-press request; sketch performs safe shutdown first

  // Connectivity --------------------------------------------------------------
  bool workNetworkReady() const;
  bool serverRuntimeReady() const { return _serverRuntimeReady && WiFi.status() == WL_CONNECTED; }
  String workConnectionMessage() const;
  String serverConnectionMessage() const;
  void recordConnectionDiagnostic(const String& message);
  void clearConnectionDiagnostics();
  bool mqttConnected() const;
  bool pairWithServer(const String& pairingCode, String& humanMessage);
  bool testSavedServer(String& humanMessage);
  bool runtimeRequest(JsonDocument& request, String& responsePayload, const char* correlationPrefix);
  bool httpRuntimeRequest(JsonDocument& request, String& responsePayload);
  bool publishDocument(JsonDocument& doc);
  void addCommonEventFields(JsonDocument& doc, const char* type, const String& correlationId = "");
  void sendHeartbeat();
  void sendWarningEvent(const String& warning);
  void setHealth(bool rfidOk, bool pzemApplicable = false, bool pzemOk = true);
  bool takeCommand(PendingCommand& out);

  // UI / hardware -------------------------------------------------------------
  void showScreen(const String& title, const String& line1 = "", const String& line2 = "", const String& line3 = "");
  void showTemporary(const String& title, const String& line1, const String& line2, const String& line3, unsigned long durationMs);
  bool overlayActive() const;
  bool displayReady() const { return _displayReady; }

  void startBuzzerPattern(uint16_t onMs, uint16_t offMs, uint8_t beepCount);
  void stopBuzzer();
  void beepPing();
  void beepTerminate();

  void setRfidActive(bool active);
  bool checkRfidHardware();
  bool readPresentUid(String& uidOut);
  bool rfidHealthy() const { return _rfidHealthy; }
  bool rfidActive() const { return _rfidInitialized; }

  // Helpers ------------------------------------------------------------------
  String normalizeServerUrl(String value) const;
  String normalizeApiPath(String value, const char* fallback) const;
  String normalizePairingCode(String value) const;
  String macCompact() const;
  String uidToString(const MFRC522::Uid& uid) const;
  String money2(int64_t units) const;
  String money5(int64_t units) const;
  String makeCorrelationId(const char* prefix) const;
  bool responseOk(const String& raw, JsonDocument& doc) const;

  // Small shared persistence/text helpers used by module-specific journals.
  uint32_t checksum32(const String& value) const;
  bool fieldAt(const String& input, int index, String& out) const;
  void saveAlternatingRecord(const char* prefsNamespace, uint32_t sequence, const String& record);
  void loadAlternatingRecords(const char* prefsNamespace, String& slotA, String& slotB) const;
  void clearStoredNamespace(const char* prefsNamespace);
  void saveUidLock(const char* prefsNamespace, bool locked, const String& uid);
  bool loadUidLock(const char* prefsNamespace, String& uid) const;

  ModuleConfig& config() { return _config; }
  const ModuleConfig& config() const { return _config; }
  const CommonSettings& settings() const { return _settings; }

 private:
  bool initDisplay(bool forceBusReset = false);
  void ensureDisplayReady();

  CommonSettings _settings;
  ModuleConfig _config;
  Mode _mode = Mode::Registration;

  Adafruit_SSD1306 _display;
  bool _displayReady = false;
  unsigned long _lastDisplayRecoveryMs = 0;
  MFRC522 _rfid;
  WebServer _portal;
  DNSServer _dns;

  WiFiClient _mqttPlainNet;
  WiFiClientSecure _mqttSecureNet;
  MqttClient* _mqtt = nullptr;
  bool _mqttBuiltForTls = false;

  PendingCommand _pendingCommand;
  String _lastCommandFingerprint;
  unsigned long _lastCommandAcceptedMs = 0;

  bool _portalActive = false;
  bool _portalHandlersConfigured = false;
  bool _probePending = false;
  unsigned long _probeAtMs = 0;
  String _probePairingCode;
  bool _workTransitionPending = false;
  unsigned long _workTransitionAtMs = 0;

  bool _rfidInitialized = false;
  bool _rfidHealthy = false;
  bool _healthRfidApplicable = false;
  bool _healthRfidOk = false;
  bool _healthPzemApplicable = false;
  bool _healthPzemOk = true;

  unsigned long _overlayUntilMs = 0;

  bool _buzzerRunning = false;
  bool _buzzerLevel = false;
  uint16_t _buzzerOnMs = 100;
  uint16_t _buzzerOffMs = 100;
  uint16_t _buzzerTransitionsRemaining = 0;
  uint16_t _buzzerFrequencyHz = 1500;
  unsigned long _buzzerNextMs = 0;
  unsigned long _lastBuzzerStartMs = 0;

  bool _buttonStable = HIGH;
  bool _buttonLastRead = HIGH;
  unsigned long _buttonChangedMs = 0;
  unsigned long _buttonPressedMs = 0;
  bool _buttonLongHandled = false;
  bool _registrationRequested = false;

  // OLED text remains in RAM so long lines can scroll instead of being cut.
  String _oledTitle;
  String _oledLine1;
  String _oledLine2;
  String _oledLine3;
  unsigned long _oledLastScrollMs = 0;
  uint16_t _oledScrollStep = 0;
  bool _oledScrollable = false;
  bool _oledDirty = false;
  unsigned long _oledLastRenderMs = 0;
  unsigned long _oledContentChangedMs = 0;

  // Wi-Fi is deliberately owned by the Arduino loop only. The pre-oral build
  // was reliable because AP/STA transitions were never changed from another
  // user task while Registration Mode was taking control of the same radio.

  // Automatic Work Mode reconnect diagnostics. RAM-only by design.
  static constexpr uint8_t WORK_DIAG_LIMIT = 16;
  String _workDiag[WORK_DIAG_LIMIT];
  uint8_t _workDiagCount = 0;
  unsigned long _workReconnectStartedMs = 0;
  uint8_t _workAttemptNumber = 0;

  unsigned long _lastWifiAttemptMs = 0;
  unsigned long _wifiAttemptStartedMs = 0;
  bool _wifiAttemptInProgress = false;
  bool _wifiScanInProgress = false;
  bool _wifiTryingDefaultHotspotPassword = false;
  unsigned long _wifiNextActionMs = 0;
  bool _serverRuntimeReady = false;
  unsigned long _lastMqttAttemptMs = 0;
  unsigned long _lastHeartbeatMs = 0;

  // Registration diagnostics/cache. Nearby SSIDs are intentionally RAM-only:
  // they survive the temporary AP -> STA -> AP test cycle, but disappear on
  // reboot and are cleared after a fully successful registration.
  static constexpr uint8_t REGISTRATION_SCAN_LIMIT = 12;
  String _registrationScanSSID[REGISTRATION_SCAN_LIMIT];
  int32_t _registrationScanRSSI[REGISTRATION_SCAN_LIMIT] = {0};
  bool _registrationScanSecure[REGISTRATION_SCAN_LIMIT] = {false};
  uint8_t _registrationScanAuth[REGISTRATION_SCAN_LIMIT] = {0};
  int32_t _registrationScanChannel[REGISTRATION_SCAN_LIMIT] = {0};
  String _registrationScanBSSID[REGISTRATION_SCAN_LIMIT];
  uint8_t _registrationScanCount = 0;

  // Low-level Wi-Fi diagnostics. The disconnect event is raised from the
  // Arduino-ESP32 network task, so only a primitive reason code is written
  // there; human-readable strings are built later in the normal loop task.
  volatile uint16_t _lastWifiDisconnectReason = 0;
  int _lastWifiStatusCode = WL_IDLE_STATUS;
  bool _lastWifiTargetFound = false;
  int32_t _lastWifiTargetRSSI = 0;
  int32_t _lastWifiTargetChannel = 0;
  uint8_t _lastWifiTargetAuth = 0;
  String _lastWifiTargetBSSID;
  String _lastWifiConnectStrategy;
  String _lastPairingErrorCode;

  String _waitingCorrelationId;
  String _waitingResponsePayload;
  bool _waitingResponseReady = false;

  static ModuleCore* _instance;

  // Storage ------------------------------------------------------------------
  String prefsNamespace() const;
  void saveConfig();
  void loadConfig();

  // Common services -----------------------------------------------------------
  void serviceBuzzer();
  void serviceButton();
  void serviceOledScroll();
  void renderOledScreen();
  String oledScrollWindow(const String& value, size_t width, uint16_t phase) const;
  void serviceRegistrationMode();
  void serviceWorkMode();
  void serviceWiFi();
  void beginSavedWiFiJoin(const String& diagnosticLabel);
  bool connectMqtt();
  void disconnectMqtt();
  void rebuildMqttClientIfNeeded();

  // Portal -------------------------------------------------------------------
  void startPortal();
  void stopPortal();
  String portalHtml() const;
  String escapeHtml(String value) const;
  String defaultHotspotName() const;
  String sanitizeHotspotName(String value) const;
  String sanitizeHotspotPassword(String value) const;
  String serverFromGateway() const;
  void scheduleRegistrationProbe(const String& pairingCode);
  void runRegistrationProbe();
  bool connectWiFiBlocking(const String& ssid, const String& password, unsigned long timeoutMs);
  bool waitForWiFiConnected(unsigned long timeoutMs);
  void scanNearbyForRegistration();
  void clearRegistrationScanCache();
  int findCachedSSID(const String& ssid) const;
  String wifiFailureMessage(const String& ssid, const String& password) const;
  String wifiReasonMessage(uint16_t reason) const;
  String wifiAuthLabel(uint8_t auth) const;
  String wifiDiagnosticSummary() const;
  String workDiagnosticHtml() const;
  String oledText(String value, size_t maxLength = 20) const;
  bool testBaseServer(String& message);

  // HTTP ---------------------------------------------------------------------
  bool httpPostJson(const String& url, const String& payload, String& response, int& statusCode, bool authorizedRuntime);
  bool httpGet(const String& url, String& response, int& statusCode);
  String endpointUrl(const String& path) const;
  void applyRuntimeMetadata(const String& responsePayload);

  // MQTT ---------------------------------------------------------------------
  static void wifiEventThunk(WiFiEvent_t event, WiFiEventInfo_t info);
  static void mqttMessageThunk(int messageSize);
  void onMqttMessage(int messageSize);
};

} // namespace ElectriCredit


namespace ElectriCredit {

ModuleCore* ModuleCore::_instance = nullptr;

static constexpr unsigned long WIFI_SCAN_RETRY_MS = 4000;
static constexpr unsigned long WIFI_AUTH_RETRY_MS = 5000;
static constexpr unsigned long WIFI_CONNECT_WINDOW_MS = 20000;
static constexpr unsigned long WIFI_RADIO_SETTLE_MS = 200;
static constexpr unsigned long OLED_SCROLL_MS = 850;
static constexpr unsigned long OLED_MIN_RENDER_MS = 250;
static constexpr unsigned long OLED_CONTENT_SETTLE_MS = 450;
static constexpr unsigned long MQTT_RETRY_MS = 5000;
static constexpr unsigned long HEARTBEAT_MS = 30000;
static constexpr unsigned long SERVER_READY_RETRY_MS = 5000;
static constexpr unsigned long BUTTON_DEBOUNCE_MS = 35;
static constexpr unsigned long BUTTON_LONG_PRESS_MS = 3000;
static constexpr unsigned long REQUEST_TIMEOUT_MS = 250;

ModuleCore::ModuleCore()
    : _display(128, 64, &Wire, -1),
      _rfid(RFID_SS_PIN, RFID_RST_PIN),
      _portal(80) {
  _instance = this;
}

ModuleCore::~ModuleCore() {
  disconnectMqtt();
  if (_instance == this) _instance = nullptr;
}

void ModuleCore::wifiEventThunk(WiFiEvent_t event, WiFiEventInfo_t info) {
  if (event == ARDUINO_EVENT_WIFI_STA_DISCONNECTED && _instance) {
    _instance->_lastWifiDisconnectReason = (uint16_t)info.wifi_sta_disconnected.reason;
  }
}

bool ModuleCore::begin(const CommonSettings& settings) {
  _settings = settings;
  _instance = this;

  pinMode(BUZZER_PIN, OUTPUT);
  digitalWrite(BUZZER_PIN, LOW);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  _buttonLastRead = digitalRead(BUTTON_PIN);
  _buttonStable = _buttonLastRead;
  _buttonChangedMs = millis();
  _buttonPressedMs = (_buttonStable == LOW) ? millis() : 0;
  _buttonLongHandled = false;

  // Keep Registry OLED startup byte-for-byte equivalent to the previously
  // working shared path: begin the existing I2C pins and initialize 0x3C once.
  // The probe/retry/bus-reset recovery below is HUB-only, so this Hub fix does
  // not change Registry startup timing, coin handling, RFID handling, or I2C.
  const bool hubModule = _settings.moduleType && String(_settings.moduleType).equalsIgnoreCase("hub");
  bool displayOk = false;
  if (!hubModule) {
    Wire.begin(OLED_SDA_PIN, OLED_SCL_PIN);
    displayOk = _display.begin(SSD1306_SWITCHCAPVCC, 0x3C);
    _displayReady = displayOk;
    if (displayOk) {
      _display.clearDisplay();
      _display.setTextColor(SSD1306_WHITE);
      _display.display();
    }
  } else {
    displayOk = initDisplay(false);
    for (uint8_t attempt = 1; attempt < 3 && !displayOk; ++attempt) {
      delay(80);
      displayOk = initDisplay(true);
    }
    if (!displayOk) {
      Serial.println("[HUB][OLED] SSD1306 not responding on SDA=21 SCL=22 address=0x3C; Hub-only retry remains enabled.");
    }
  }

  const esp_reset_reason_t bootResetReason = esp_reset_reason();
  const char* bootResetLabel = "UNKNOWN";
  switch (bootResetReason) {
    case ESP_RST_POWERON:   bootResetLabel = "POWER ON"; break;
    case ESP_RST_EXT:       bootResetLabel = "EXTERNAL"; break;
    case ESP_RST_SW:        bootResetLabel = "SOFTWARE"; break;
    case ESP_RST_PANIC:     bootResetLabel = "PANIC"; break;
    case ESP_RST_INT_WDT:   bootResetLabel = "INT WDT"; break;
    case ESP_RST_TASK_WDT:  bootResetLabel = "TASK WDT"; break;
    case ESP_RST_WDT:       bootResetLabel = "WDT"; break;
    case ESP_RST_DEEPSLEEP: bootResetLabel = "DEEPSLEEP"; break;
    case ESP_RST_BROWNOUT:  bootResetLabel = "BROWNOUT"; break;
    case ESP_RST_SDIO:      bootResetLabel = "SDIO"; break;
    default: break;
  }
  Serial.printf("[EC][BOOT] reset_reason=%d (%s)\n", (int)bootResetReason, bootResetLabel);
  if (_displayReady) {
    showTemporary("ElectriCredit Boot",
                  String("Reset: ") + bootResetLabel,
                  hubModule ? "Hub starting" : "Registry starting",
                  "Hold button: setup",
                  1200);
  }
  if (hubModule) Serial.println("[HUB][BOOT] OLED initialized; loading saved module configuration.");

  // Keep the RC522 in hardware power-down while Wi-Fi/server recovery is in
  // progress. This reduces startup load without changing wiring. The reader is
  // initialized lazily only after Work Mode has a usable network path.
  pinMode(RFID_RST_PIN, OUTPUT);
  digitalWrite(RFID_RST_PIN, LOW);
  _rfidInitialized = false;
  _rfidHealthy = false;
  _healthRfidApplicable = false;
  _healthRfidOk = false;

  loadConfig();
  bool configDefaultsChanged = false;
  if (hubModule) {
    // HUB registration hotspot is intentionally fixed for predictable setup.
    if (_config.hotspotName != "Hub") {
      _config.hotspotName = "Hub";
      configDefaultsChanged = true;
    }
    if (_config.hotspotPassword != DEFAULT_AP_PASSWORD) {
      _config.hotspotPassword = DEFAULT_AP_PASSWORD;
      configDefaultsChanged = true;
    }
  } else {
    if (!_config.hotspotName.length()) {
      _config.hotspotName = defaultHotspotName();
      configDefaultsChanged = true;
    }
    if (!_config.hotspotPassword.length()) {
      _config.hotspotPassword = DEFAULT_AP_PASSWORD;
      configDefaultsChanged = true;
    }
  }
  // Avoid an unnecessary NVS write on every normal boot.
  if (configDefaultsChanged) saveConfig();
  if (hubModule) {
    Serial.printf("[HUB][BOOT] paired=%s saved_ssid=%s module_id=%lld hotspot=%s\n",
                  _config.paired ? "yes" : "no", _config.ssid.length() ? "yes" : "no",
                  (long long)_config.moduleId, _config.hotspotName.c_str());
  }

  // Give standalone 5 V supplies a brief settling window before the ESP32
  // starts Wi-Fi RF activity. RFID remains in reset and transaction outputs
  // remain disabled during this period.
  WiFi.mode(WIFI_OFF);
  const unsigned long powerSettleStarted = millis();
  while (millis() - powerSettleStarted < 650UL) {
    serviceBuzzer();
    serviceOledScroll();
    delay(10);
  }

  // Emergency setup override: holding GPIO33 continuously for 3 seconds
  // during boot enters Registration Mode before Wi-Fi scanning begins. This is
  // intentionally independent of the normal connection state machine.
  bool bootForceRegistration = false;
  if (digitalRead(BUTTON_PIN) == LOW) {
    startBuzzerPattern(55, 45, 1);
    showTemporary("Setup Override", "Keep holding button", "3 sec: Registration", "Release to cancel", 3200);
    const unsigned long bootHoldStarted = millis();
    while (digitalRead(BUTTON_PIN) == LOW && millis() - bootHoldStarted < BUTTON_LONG_PRESS_MS) {
      serviceBuzzer();
      delay(10);
    }
    if (digitalRead(BUTTON_PIN) == LOW && millis() - bootHoldStarted >= BUTTON_LONG_PRESS_MS) {
      bootForceRegistration = true;
      stopBuzzer();
      startBuzzerPattern(110, 90, 2);
      showTemporary("Registration Mode", "Boot hold accepted", "Starting setup AP", "Please wait", 1400);
    }
    _buttonLastRead = digitalRead(BUTTON_PIN);
    _buttonStable = _buttonLastRead;
    _buttonChangedMs = millis();
    _buttonPressedMs = 0;
    _buttonLongHandled = true;
  }

  // A fully paired module normally attempts Work Mode after power-on. A
  // 3-second boot hold always wins and opens setup without touching saved
  // credentials or session journals.
  const bool hasSavedWiFi = _config.ssid.length() > 0;
  _mode = bootForceRegistration ? Mode::Registration
                                : ((_config.paired && hasSavedWiFi) ? Mode::Work : Mode::Registration);
  WiFi.onEvent(ModuleCore::wifiEventThunk, ARDUINO_EVENT_WIFI_STA_DISCONNECTED);

  if (_mode == Mode::Registration) {
    WiFi.mode(WIFI_OFF);
    delay(40);
    startPortal();
  } else {
    // Hub and Registry use the same non-blocking Work Mode recovery path.
    // The module continuously scans for the saved SSID, attempts the saved
    // credentials only when that SSID is visible, and returns to scanning after
    // an authentication/connection timeout. It never falls into Registration
    // Mode automatically; the physical 3-second hold remains available because
    // Arduino loop() continues to run while Wi-Fi is unavailable.
    _serverRuntimeReady = false;
    showScreen("System", "Scanning saved Wi-Fi", oledText(_config.ssid), "Hold 3 sec: setup");
    recordConnectionDiagnostic(String(_settings.moduleType) + " boot: continuous saved Wi-Fi scan/reconnect started.");
    beginSavedWiFiJoin("Boot recovery: scan for saved SSID before connecting.");
  }
  return displayOk;
}

void ModuleCore::loop() {
  serviceBuzzer();
  serviceButton();
  serviceOledScroll();

  if (_mode == Mode::Registration) {
    serviceRegistrationMode();
  } else {
    serviceWorkMode();
  }
}

// -----------------------------------------------------------------------------
// Storage
// -----------------------------------------------------------------------------

String ModuleCore::prefsNamespace() const {
  String ns = _settings.preferencesNamespace ? _settings.preferencesNamespace : "ec-module";
  if (ns.length() > 15) ns = ns.substring(0, 15);
  return ns;
}

void ModuleCore::saveConfig() {
  Preferences p;
  p.begin(prefsNamespace().c_str(), false);
  p.putBool("paired", _config.paired);
  p.putUChar("mode", (uint8_t)_config.preferredMode);
  p.putString("ssid", _config.ssid);
  p.putString("wifi", _config.wifiPassword);
  p.putString("server", _config.serverUrl);
  p.putString("conn_path", _config.connectPath);
  p.putString("run_path", _config.runtimePath);
  p.putString("ap_name", _config.hotspotName);
  p.putString("ap_pass", _config.hotspotPassword);
  p.putLong64("mod_id", _config.moduleId);
  p.putString("token", _config.moduleToken);
  p.putString("mod_name", _config.moduleName);
  p.putString("mod_loc", _config.moduleLocation);
  p.putString("mod_status", _config.moduleStatus);
  p.putBool("mq_en", _config.mqttEnabled);
  p.putBool("mq_tls", _config.mqttTls);
  p.putString("mq_host", _config.mqttHost);
  p.putUShort("mq_port", _config.mqttPort);
  p.putString("mq_user", _config.mqttUser);
  p.putString("mq_pass", _config.mqttPassword);
  p.putString("ev_topic", _config.eventTopic);
  p.putString("cmd_topic", _config.commandTopic);
  p.putBool("probe_ok", _config.lastProbeOk);
  p.putString("probe_msg", _config.lastProbeMessage);
  p.putString("last_wifi", _config.lastWifi);
  p.putString("last_ip", _config.lastIp);
  p.end();
}

void ModuleCore::loadConfig() {
  Preferences p;
  p.begin(prefsNamespace().c_str(), true);
  _config.paired = p.getBool("paired", false);
  _config.preferredMode = p.getUChar("mode", 0) == 1 ? Mode::Work : Mode::Registration;
  _config.ssid = p.getString("ssid", "");
  _config.wifiPassword = p.getString("wifi", "");
  _config.serverUrl = p.getString("server", "");
  _config.connectPath = p.getString("conn_path", DEFAULT_CONNECT_PATH);
  _config.runtimePath = p.getString("run_path", DEFAULT_RUNTIME_PATH);
  _config.hotspotName = p.getString("ap_name", "");
  _config.hotspotPassword = p.getString("ap_pass", DEFAULT_AP_PASSWORD);
  _config.moduleId = p.getLong64("mod_id", 0);
  _config.moduleToken = p.getString("token", "");
  _config.moduleName = p.getString("mod_name", "");
  _config.moduleLocation = p.getString("mod_loc", "");
  _config.moduleStatus = p.getString("mod_status", "");
  _config.mqttEnabled = p.getBool("mq_en", true);
  _config.mqttTls = p.getBool("mq_tls", false);
  _config.mqttHost = p.getString("mq_host", "");
  _config.mqttPort = p.getUShort("mq_port", 1883);
  _config.mqttUser = p.getString("mq_user", "");
  _config.mqttPassword = p.getString("mq_pass", "");
  _config.eventTopic = p.getString("ev_topic", "");
  _config.commandTopic = p.getString("cmd_topic", "");
  _config.lastProbeOk = p.getBool("probe_ok", false);
  _config.lastProbeMessage = p.getString("probe_msg", "");
  _config.lastWifi = p.getString("last_wifi", "");
  _config.lastIp = p.getString("last_ip", "");
  p.end();

  if (_config.serverUrl.length()) _config.serverUrl = normalizeServerUrl(_config.serverUrl);
  else if (_config.paired) _config.serverUrl = DEFAULT_SERVER_URL;
  _config.connectPath = normalizeApiPath(_config.connectPath, DEFAULT_CONNECT_PATH);
  _config.runtimePath = normalizeApiPath(_config.runtimePath, DEFAULT_RUNTIME_PATH);
  _config.hotspotName = sanitizeHotspotName(_config.hotspotName);
  _config.hotspotPassword = sanitizeHotspotPassword(_config.hotspotPassword);
  _config.paired = _config.paired && _config.moduleId > 0 && _config.moduleToken.length() > 0;
  if (!_config.paired) _config.preferredMode = Mode::Registration;
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

String ModuleCore::normalizeServerUrl(String value) const {
  value.trim();
  if (!value.length()) value = DEFAULT_SERVER_URL;
  if (!value.startsWith("http://") && !value.startsWith("https://")) value = "http://" + value;
  while (value.endsWith("/")) value.remove(value.length() - 1);
  return value;
}

String ModuleCore::normalizeApiPath(String value, const char* fallback) const {
  value.trim();
  if (!value.length()) value = fallback;
  if (value.startsWith("http://") || value.startsWith("https://")) return value;
  if (!value.startsWith("/")) value = "/" + value;
  return value;
}

String ModuleCore::endpointUrl(const String& path) const {
  if (path.startsWith("http://") || path.startsWith("https://")) return normalizeServerUrl(path);
  return normalizeServerUrl(_config.serverUrl) + normalizeApiPath(path, "/");
}

String ModuleCore::normalizePairingCode(String value) const {
  value.trim();
  value.replace(" ", "");
  value.replace("-", "");
  if (value.length() != 12) return "";
  for (size_t i = 0; i < value.length(); i++) {
    if (value[i] < '0' || value[i] > '9') return "";
  }
  return value.substring(0, 4) + "-" + value.substring(4, 8) + "-" + value.substring(8);
}

String ModuleCore::macCompact() const {
  String value = WiFi.macAddress();
  value.replace(":", "");
  value.toUpperCase();
  return value;
}

String ModuleCore::uidToString(const MFRC522::Uid& uid) const {
  String out;
  for (byte i = 0; i < uid.size; i++) {
    if (uid.uidByte[i] < 0x10) out += "0";
    out += String(uid.uidByte[i], HEX);
    if (i + 1 < uid.size) out += ":";
  }
  out.toUpperCase();
  return out;
}

String ModuleCore::money2(int64_t units) const {
  bool neg = units < 0;
  uint64_t absUnits = neg ? (uint64_t)(-units) : (uint64_t)units;
  uint64_t whole = absUnits / MONEY_SCALE;
  uint64_t centavos = (absUnits % MONEY_SCALE) / 1000ULL;
  String s = neg ? "-P" : "P";
  s += String((unsigned long)whole);
  s += ".";
  if (centavos < 10) s += "0";
  s += String((unsigned long)centavos);
  return s;
}

String ModuleCore::money5(int64_t units) const {
  bool neg = units < 0;
  uint64_t absUnits = neg ? (uint64_t)(-units) : (uint64_t)units;
  uint64_t whole = absUnits / MONEY_SCALE;
  uint64_t fraction = absUnits % MONEY_SCALE;
  char frac[7];
  snprintf(frac, sizeof(frac), "%05lu", (unsigned long)fraction);
  String s = neg ? "-" : "";
  s += String((unsigned long)whole);
  s += ".";
  s += frac;
  return s;
}

String ModuleCore::makeCorrelationId(const char* prefix) const {
  String out = prefix ? prefix : "req";
  out += "-";
  out += macCompact().substring(8);
  out += "-";
  out += String((unsigned long)millis(), HEX);
  out += "-";
  out += String((unsigned long)esp_random(), HEX);
  return out;
}

String ModuleCore::escapeHtml(String value) const {
  value.replace("&", "&amp;");
  value.replace("<", "&lt;");
  value.replace(">", "&gt;");
  value.replace("\"", "&quot;");
  value.replace("'", "&#39;");
  return value;
}

String ModuleCore::sanitizeHotspotName(String value) const {
  value.trim();
  if (!value.length()) return defaultHotspotName();
  String clean;
  clean.reserve(32);
  for (size_t i = 0; i < value.length() && clean.length() < 31; i++) {
    char c = value[i];
    if ((uint8_t)c >= 32 && (uint8_t)c <= 126) clean += c;
  }
  clean.trim();
  return clean.length() ? clean : defaultHotspotName();
}

String ModuleCore::sanitizeHotspotPassword(String value) const {
  value.trim();
  if (!value.length()) return DEFAULT_AP_PASSWORD;
  if (value.length() < 8 || value.length() > 63) return DEFAULT_AP_PASSWORD;
  String clean;
  clean.reserve(value.length());
  for (size_t i = 0; i < value.length(); i++) {
    char c = value[i];
    if ((uint8_t)c >= 32 && (uint8_t)c <= 126) clean += c;
  }
  if (clean.length() < 8 || clean.length() > 63) return DEFAULT_AP_PASSWORD;
  return clean;
}

String ModuleCore::serverFromGateway() const {
  if (WiFi.status() != WL_CONNECTED) return "";
  IPAddress gateway = WiFi.gatewayIP();
  if (gateway == IPAddress(0, 0, 0, 0)) return "";
  return "http://" + gateway.toString() + ":8080";
}

String ModuleCore::oledText(String value, size_t maxLength) const {
  (void)maxLength;
  value.trim();
  return value;
}

void ModuleCore::clearRegistrationScanCache() {
  for (uint8_t i = 0; i < REGISTRATION_SCAN_LIMIT; i++) {
    _registrationScanSSID[i] = "";
    _registrationScanRSSI[i] = 0;
    _registrationScanSecure[i] = false;
    _registrationScanAuth[i] = 0;
    _registrationScanChannel[i] = 0;
    _registrationScanBSSID[i] = "";
  }
  _registrationScanCount = 0;
}

int ModuleCore::findCachedSSID(const String& ssid) const {
  for (uint8_t i = 0; i < _registrationScanCount; i++) {
    if (_registrationScanSSID[i] == ssid) return (int)i;
  }
  return -1;
}

void ModuleCore::scanNearbyForRegistration() {
  clearRegistrationScanCache();
  if (WiFi.getMode() != WIFI_STA) {
    WiFi.mode(WIFI_STA);
    delay(100);
  }
  int found = WiFi.scanNetworks(false, true);
  if (found <= 0) {
    WiFi.scanDelete();
    return;
  }
  for (int i = 0; i < found && _registrationScanCount < REGISTRATION_SCAN_LIMIT; i++) {
    String ssid = WiFi.SSID(i);
    ssid.trim();
    if (!ssid.length() || findCachedSSID(ssid) >= 0) continue;
    uint8_t slot = _registrationScanCount++;
    uint8_t auth = (uint8_t)WiFi.encryptionType(i);
    _registrationScanSSID[slot] = ssid;
    _registrationScanRSSI[slot] = WiFi.RSSI(i);
    _registrationScanSecure[slot] = auth != (uint8_t)WIFI_AUTH_OPEN;
    _registrationScanAuth[slot] = auth;
    _registrationScanChannel[slot] = WiFi.channel(i);
    _registrationScanBSSID[slot] = WiFi.BSSIDstr(i);
  }
  WiFi.scanDelete();
}

String ModuleCore::wifiAuthLabel(uint8_t auth) const {
  // wifi_auth_mode_t values are stable across the Arduino-ESP32 2.x/3.x
  // families. Numeric fallbacks keep this library source-compatible even when
  // a board package does not expose every newer WPA3 enum symbol.
  switch (auth) {
    case 0: return "Open";
    case 1: return "WEP";
    case 2: return "WPA";
    case 3: return "WPA2";
    case 4: return "WPA/WPA2";
    case 5: return "WPA2-Enterprise";
    case 6: return "WPA3";
    case 7: return "WPA2/WPA3";
    case 8: return "WAPI";
    default: return "Auth " + String((int)auth);
  }
}

String ModuleCore::wifiReasonMessage(uint16_t reason) const {
  switch (reason) {
    case 0: return "No low-level disconnect reason was reported.";
    case 2: return "Wi-Fi handshake timed out (reason 2). Retrying automatically.";
    case 15: return "WPA handshake timed out (reason 15). Check the password/security mode.";
    case 200: return "Beacon timeout (reason 200). The AP signal disappeared.";
    case 201: return "No matching AP was accepted (reason 201).";
    case 202: return "Authentication failed (reason 202). Check the Wi-Fi password.";
    case 203: return "Association was rejected by the AP (reason 203).";
    case 204: return "WPA handshake failed (reason 204). Check password/security compatibility.";
    case 205: return "Connection failed at the Wi-Fi driver (reason 205).";
    case 210: return "AP security is incompatible with this ESP32 (reason 210).";
    case 211: return "AP security is below the ESP32 minimum (reason 211).";
    case 212: return "AP signal is below the ESP32 RSSI threshold (reason 212).";
    default: return "ESP32 Wi-Fi disconnect reason " + String((int)reason) + ".";
  }
}

String ModuleCore::wifiDiagnosticSummary() const {
  if (!_lastWifiTargetFound) {
    String out;
    if (_lastWifiDisconnectReason) out = wifiReasonMessage(_lastWifiDisconnectReason);
    else out = "No exact AP target was forced; Work Mode used the saved SSID/password.";
    if (_lastWifiConnectStrategy.length()) out += " · " + _lastWifiConnectStrategy;
    return out;
  }
  String out = wifiAuthLabel(_lastWifiTargetAuth);
  out += " · CH " + String((long)_lastWifiTargetChannel);
  out += " · " + String((long)_lastWifiTargetRSSI) + " dBm";
  if (_lastWifiTargetBSSID.length()) out += " · " + _lastWifiTargetBSSID;
  if (_lastWifiDisconnectReason) out += " · reason " + String((int)_lastWifiDisconnectReason);
  if (_lastWifiConnectStrategy.length()) out += " · " + _lastWifiConnectStrategy;
  return out;
}

String ModuleCore::wifiFailureMessage(const String& ssid, const String& password) const {
  int slot = findCachedSSID(ssid);
  if (_lastWifiDisconnectReason == 2) {
    return wifiReasonMessage(_lastWifiDisconnectReason);
  }
  if (_lastWifiDisconnectReason == 210 || _lastWifiDisconnectReason == 211) {
    return wifiReasonMessage(_lastWifiDisconnectReason);
  }
  if (_lastWifiDisconnectReason == 15 || _lastWifiDisconnectReason == 202 || _lastWifiDisconnectReason == 204) {
    return wifiReasonMessage(_lastWifiDisconnectReason);
  }
  if (_lastWifiStatusCode == WL_NO_SSID_AVAIL || _lastWifiDisconnectReason == 201) return "SSID was not available/accepted during the last join attempt.";
  if (slot < 0) {
    if (_lastWifiDisconnectReason) return wifiReasonMessage(_lastWifiDisconnectReason);
    return "Wi-Fi join timed out before a definitive low-level reason was reported.";
  }
  if (_registrationScanSecure[slot] && !password.length()) return "SSID found, but a password is required.";
  if (_lastWifiStatusCode == WL_CONNECT_FAILED) return "SSID found, but authentication/association failed.";
  if (_lastWifiStatusCode == WL_CONNECTION_LOST || _lastWifiDisconnectReason == 200) return "SSID found, but the Wi-Fi connection was lost.";
  if (_lastWifiDisconnectReason) return wifiReasonMessage(_lastWifiDisconnectReason);
  if (_registrationScanSecure[slot]) return "SSID found, but connection timed out. Check password, security, and signal.";
  return "SSID found, but the ESP32 could not complete the connection.";
}

String ModuleCore::defaultHotspotName() const {
  String prefix = _settings.defaultHotspotPrefix ? _settings.defaultHotspotPrefix : "ElectriCredit";
  String type = _settings.moduleType ? _settings.moduleType : "Module";
  type.toLowerCase();
  if (type == "hub") type = "Hub";
  else if (type == "registry") type = "Registry";
  String tail = macCompact();
  if (tail.length() > 4) tail = tail.substring(tail.length() - 4);
  String name = prefix + "-" + type + "-" + tail;
  if (name.length() > 31) name = name.substring(0, 31);
  return name;
}

// -----------------------------------------------------------------------------
// OLED / buzzer / RFID
// -----------------------------------------------------------------------------

bool ModuleCore::initDisplay(bool forceBusReset) {
  if (_displayReady) return true;

  if (forceBusReset) {
    // Reset only the I2C controller. Pins/address remain identical to the
    // proven pre-oral wiring; this simply recovers from a stuck startup bus.
    Wire.end();
    delay(12);
  }

  Wire.begin(OLED_SDA_PIN, OLED_SCL_PIN);
  Wire.setClock(100000);  // conservative SSD1306 I2C speed used for startup
  delay(18);

  // Probe the exact old working OLED address before asking Adafruit_SSD1306
  // to initialize. No alternate address/pin guessing is performed.
  Wire.beginTransmission(0x3C);
  uint8_t probe = Wire.endTransmission();
  if (probe != 0) {
    _displayReady = false;
    return false;
  }

  _displayReady = _display.begin(SSD1306_SWITCHCAPVCC, 0x3C);
  if (!_displayReady) return false;

  _display.clearDisplay();
  _display.setTextColor(SSD1306_WHITE);
  _display.setTextSize(1);
  _display.setCursor(0, 0);
  _display.println("ElectriCredit");
  _display.drawFastHLine(0, 10, 128, SSD1306_WHITE);
  _display.setCursor(0, 17);
  _display.println("Module starting");
  _display.setCursor(0, 31);
  _display.println(_settings.moduleType ? _settings.moduleType : "module");
  _display.setCursor(0, 45);
  _display.println("Please wait");
  _display.display();
  return true;
}

void ModuleCore::ensureDisplayReady() {
  if (_displayReady) return;
  // Automatic I2C bus reset/retry is intentionally isolated to Hub. Registry
  // keeps the pre-existing display behaviour and therefore its RFID/coin
  // timing cannot be changed by the Hub OLED recovery patch.
  if (!_settings.moduleType || !String(_settings.moduleType).equalsIgnoreCase("hub")) return;
  if (_lastDisplayRecoveryMs && millis() - _lastDisplayRecoveryMs < 2000) return;
  _lastDisplayRecoveryMs = millis();
  if (initDisplay(true)) {
    Serial.println("[HUB][OLED] SSD1306 recovered on SDA=21 SCL=22 address=0x3C.");
  }
}

String ModuleCore::oledScrollWindow(const String& value, size_t width, uint16_t phase) const {
  String text = value;
  text.trim();
  if (text.length() <= width) return text;
  String marquee = text + "   ";
  size_t offset = phase % marquee.length();
  String out;
  out.reserve(width);
  for (size_t i = 0; i < width; i++) out += marquee[(offset + i) % marquee.length()];
  return out;
}

void ModuleCore::renderOledScreen() {
  ensureDisplayReady();
  if (!_displayReady) return;
  _display.clearDisplay();
  _display.setTextColor(SSD1306_WHITE);
  _display.setTextSize(1);
  _display.setCursor(0, 0);
  _display.println(oledScrollWindow(_oledTitle, 20, _oledScrollStep));
  _display.drawFastHLine(0, 10, 128, SSD1306_WHITE);
  _display.setCursor(0, 17);
  _display.println(oledScrollWindow(_oledLine1, 20, _oledScrollStep + 3));
  _display.setCursor(0, 31);
  _display.println(oledScrollWindow(_oledLine2, 20, _oledScrollStep + 7));
  _display.setCursor(0, 45);
  _display.println(oledScrollWindow(_oledLine3, 20, _oledScrollStep + 11));
  _display.display();
}

void ModuleCore::serviceOledScroll() {
  if (!_displayReady) ensureDisplayReady();
  const unsigned long now = millis();

  // Do not continuously retransmit the same 1 KB SSD1306 framebuffer. A
  // screen requested by work logic is rendered only when its content changes.
  if (_oledDirty &&
      now - _oledContentChangedMs >= OLED_CONTENT_SETTLE_MS &&
      (!_oledLastRenderMs || now - _oledLastRenderMs >= OLED_MIN_RENDER_MS)) {
    renderOledScreen();
    _oledDirty = false;
    _oledLastRenderMs = now;
  }

  if (!_oledScrollable) return;
  if (_oledLastScrollMs && now - _oledLastScrollMs < OLED_SCROLL_MS) return;
  _oledLastScrollMs = now;
  _oledScrollStep++;
  renderOledScreen();
  _oledDirty = false;
  _oledLastRenderMs = now;
}

void ModuleCore::showScreen(const String& title, const String& line1, const String& line2, const String& line3) {
  const bool changed = title != _oledTitle || line1 != _oledLine1 || line2 != _oledLine2 || line3 != _oledLine3;
  if (!changed) return;

  _oledTitle = title;
  _oledLine1 = line1;
  _oledLine2 = line2;
  _oledLine3 = line3;
  _oledScrollable = _oledTitle.length() > 20 || _oledLine1.length() > 20 || _oledLine2.length() > 20 || _oledLine3.length() > 20;
  _oledScrollStep = 0;
  _oledLastScrollMs = millis();
  _oledContentChangedMs = millis();
  _oledDirty = true;
  // Connection/status screens are settle-filtered before they touch the
  // SSD1306. This prevents unreadable flicker if two transient states arrive
  // close together.
}

void ModuleCore::showTemporary(const String& title, const String& line1, const String& line2, const String& line3, unsigned long durationMs) {
  showScreen(title, line1, line2, line3);
  _overlayUntilMs = millis() + durationMs;
  // Deliberate user-facing messages bypass the settle delay.
  if (_displayReady) {
    renderOledScreen();
    _oledDirty = false;
    _oledLastRenderMs = millis();
  }
}

bool ModuleCore::overlayActive() const {
  return _overlayUntilMs && (long)(millis() - _overlayUntilMs) < 0;
}

void ModuleCore::startBuzzerPattern(uint16_t onMs, uint16_t offMs, uint8_t beepCount) {
  if (!beepCount) return;
  // A fast loop must not continuously restart an already playing pattern.
  if (_buzzerRunning) return;
  const unsigned long now = millis();
  if (_lastBuzzerStartMs && now - _lastBuzzerStartMs < 180UL) return;
  _lastBuzzerStartMs = now;
  _buzzerOnMs = onMs;
  _buzzerOffMs = offMs;
  _buzzerTransitionsRemaining = (uint16_t)beepCount * 2U;
  _buzzerRunning = true;
  _buzzerLevel = true;
  // The pre-oral hardware used a passive buzzer. A square-wave tone gives a
  // real audible beep; holding GPIO HIGH only produces a click on that buzzer.
  tone(BUZZER_PIN, _buzzerFrequencyHz);
  _buzzerNextMs = millis() + _buzzerOnMs;
}

void ModuleCore::stopBuzzer() {
  _buzzerRunning = false;
  _buzzerLevel = false;
  _buzzerTransitionsRemaining = 0;
  noTone(BUZZER_PIN);
  digitalWrite(BUZZER_PIN, LOW);
}

void ModuleCore::serviceBuzzer() {
  if (!_buzzerRunning || (long)(millis() - _buzzerNextMs) < 0) return;
  if (_buzzerTransitionsRemaining > 0) _buzzerTransitionsRemaining--;
  if (_buzzerTransitionsRemaining == 0) {
    stopBuzzer();
    return;
  }
  _buzzerLevel = !_buzzerLevel;
  if (_buzzerLevel) tone(BUZZER_PIN, _buzzerFrequencyHz);
  else noTone(BUZZER_PIN);
  _buzzerNextMs = millis() + (_buzzerLevel ? _buzzerOnMs : _buzzerOffMs);
}

void ModuleCore::beepPing() { startBuzzerPattern(60, 50, 1); }
void ModuleCore::beepTerminate() { startBuzzerPattern(180, 120, 3); }

void ModuleCore::setRfidActive(bool active) {
  if (!active) {
    if (_rfidInitialized) _rfid.PCD_AntennaOff();
    pinMode(RFID_RST_PIN, OUTPUT);
    digitalWrite(RFID_RST_PIN, LOW);
    _rfidInitialized = false;
    _rfidHealthy = false;
    _healthRfidApplicable = false;
    _healthRfidOk = false;
    return;
  }

  if (_rfidInitialized) return;
  SPI.begin(RFID_SCK_PIN, RFID_MISO_PIN, RFID_MOSI_PIN, RFID_SS_PIN);
  pinMode(RFID_RST_PIN, OUTPUT);
  digitalWrite(RFID_RST_PIN, HIGH);
  delay(6);
  _rfid.PCD_Init();
  delay(24);
  _rfidInitialized = true;
}

bool ModuleCore::checkRfidHardware() {
  // Initialization is lazy. Module sketches decide when Work Mode is ready
  // enough to wake the RC522 by calling this function.
  setRfidActive(true);
  byte version = _rfid.PCD_ReadRegister(MFRC522::VersionReg);
  if (version == 0x00 || version == 0xFF) {
    // A warm ESP32 reset/reflash can leave the RC522 powered while its SPI
    // state is not yet ready. Reinitialize it once before declaring a fault.
    _rfid.PCD_Reset();
    delay(4);
    _rfid.PCD_Init();
    delay(12);
    version = _rfid.PCD_ReadRegister(MFRC522::VersionReg);
  }
  _rfidHealthy = version != 0x00 && version != 0xFF;
  _healthRfidApplicable = true;
  _healthRfidOk = _rfidHealthy;
  return _rfidHealthy;
}

bool ModuleCore::readPresentUid(String& uidOut) {
  uidOut = "";
  if (!_rfidHealthy && !checkRfidHardware()) return false;
  // WakeupA also sees a card that is being held in place. PICC_IsNewCardPresent()
  // is not sufficient for ElectriCredit because an active Hub deliberately
  // requires the same RFID to remain on the reader.
  byte atqa[2];
  byte atqaSize = sizeof(atqa);
  MFRC522::StatusCode status = _rfid.PICC_WakeupA(atqa, &atqaSize);
  if (status != MFRC522::STATUS_OK && status != MFRC522::STATUS_COLLISION) return false;
  if (!_rfid.PICC_ReadCardSerial()) return false;
  uidOut = uidToString(_rfid.uid);
  _rfid.PICC_HaltA();
  _rfid.PCD_StopCrypto1();
  return uidOut.length() > 0;
}

// -----------------------------------------------------------------------------
// Button / mode
// -----------------------------------------------------------------------------

void ModuleCore::serviceButton() {
  const unsigned long now = millis();
  const bool reading = digitalRead(BUTTON_PIN);

  if (reading != _buttonLastRead) {
    _buttonLastRead = reading;
    _buttonChangedMs = now;
  }

  if (now - _buttonChangedMs >= BUTTON_DEBOUNCE_MS && reading != _buttonStable) {
    _buttonStable = reading;
    if (_buttonStable == LOW) {
      _buttonPressedMs = now;
      _buttonLongHandled = false;
      // Immediate physical acknowledgement: one short beep.
      Serial.printf("[EC][BUTTON] GPIO%d pressed; hold 3 seconds to change mode.\n", BUTTON_PIN);
      startBuzzerPattern(55, 45, 1);
    } else {
      _buttonPressedMs = 0;
      _buttonLongHandled = false;
    }
  }

  if (_buttonStable != LOW || !_buttonPressedMs || _buttonLongHandled) return;
  if (now - _buttonPressedMs < BUTTON_LONG_PRESS_MS) return;

  _buttonLongHandled = true;
  Serial.printf("[EC][BUTTON] Long press accepted after %lu ms in %s mode.\n",
                now - _buttonPressedMs, _mode == Mode::Work ? "Work" : "Registration");
  if (_mode == Mode::Work) {
    // The module-specific sketch journals/stops its active work safely before
    // it calls enterRegistrationMode().
    _registrationRequested = true;
    showTemporary("Registration Request", "Button held 3 sec", "Opening setup portal", "Finishing safely", 1200);
    return;
  }

  if (_mode == Mode::Registration) {
    if (_config.paired && _config.ssid.length()) {
      showScreen("Work Mode Request", "Button held 3 sec", "Connecting saved Wi-Fi", "Please wait");
      enterWorkMode();
    } else {
      showTemporary("Setup Required", "No saved Work Mode", "Open setup portal", "http://192.168.4.1", 3000);
      startBuzzerPattern(160, 100, 3);
    }
  }
}

bool ModuleCore::takeRegistrationRequest() {
  if (!_registrationRequested) return false;
  _registrationRequested = false;
  return true;
}

bool ModuleCore::enterRegistrationMode() {
  // Publish Registration Mode first so no Work-mode service starts another
  // scan/join while the AP portal takes ownership of the radio.
  _mode = Mode::Registration;

  // Registration Mode is a current-session setup/recovery state. If this
  // paired module already has saved Wi-Fi, do not persist Registration as the
  // next-boot preference; the next restart must automatically try Work Mode.
  _config.preferredMode = (_config.paired && _config.ssid.length()) ? Mode::Work : Mode::Registration;
  saveConfig();
  disconnectMqtt();
  setRfidActive(false);
  _serverRuntimeReady = false;
  if (_wifiScanInProgress) WiFi.scanDelete();
  _wifiScanInProgress = false;
  _wifiAttemptInProgress = false;
  _wifiAttemptStartedMs = 0;
  _wifiNextActionMs = 0;
  startPortal();
  Serial.println("[EC][MODE] Registration Mode active; setup AP started.");
  startBuzzerPattern(110, 90, 2); // ON/OFF/ON/OFF: mode changed successfully
  return true;
}

bool ModuleCore::enterWorkMode() {
  if (!_config.paired || !_config.ssid.length()) return false;

  stopPortal();
  disconnectMqtt();
  setRfidActive(false);
  _serverRuntimeReady = false;
  _config.preferredMode = Mode::Work;
  saveConfig();
  _lastWifiAttemptMs = 0;
  _wifiAttemptStartedMs = 0;
  _wifiAttemptInProgress = false;
  _wifiScanInProgress = false;
  _wifiNextActionMs = 0;
  _workReconnectStartedMs = 0;
  _workAttemptNumber = 0;
  _lastMqttAttemptMs = 0;
  _lastHeartbeatMs = 0;
  showScreen("System", "Trying to connect", _config.ssid.substring(0, 20), "Starting saved Wi-Fi");

  // Both module types use the same single-owner scan/reconnect state machine.
  // Wi-Fi APIs are never driven from a second application task.
  recordConnectionDiagnostic(String(_settings.moduleType) + " Work Mode: continuous saved Wi-Fi recovery started.");
  beginSavedWiFiJoin("Work Mode recovery: scan for saved SSID before connecting.");
  _mode = Mode::Work; // publish Work Mode only after its connection state is ready
  Serial.println("[EC][MODE] Work Mode active; saved-network recovery started.");
  startBuzzerPattern(110, 90, 2); // ON/OFF/ON/OFF: mode changed successfully
  return true;
}

// -----------------------------------------------------------------------------
// Registration portal
// -----------------------------------------------------------------------------

void ModuleCore::startPortal() {
  if (_portalActive) return;
  disconnectMqtt();
  setRfidActive(false);
  if (_wifiScanInProgress) {
    WiFi.scanDelete();
    _wifiScanInProgress = false;
  }
  _wifiAttemptInProgress = false;
  _wifiAttemptStartedMs = 0;
  _wifiNextActionMs = 0;

  // Match the proven older portal radio transition. Do not erase station
  // credentials and do not hard power-cycle the Wi-Fi driver first.
  WiFi.disconnect(false, false);
  delay(250);
  WiFi.mode(WIFI_AP_STA);
  WiFi.setSleep(false);
  WiFi.setAutoReconnect(false);
  String apName = sanitizeHotspotName(_config.hotspotName);
  _config.hotspotName = apName;
  String apPassword = sanitizeHotspotPassword(_config.hotspotPassword);
  _config.hotspotPassword = apPassword;
  WiFi.softAP(apName.c_str(), apPassword.c_str());
  _dns.start(53, "*", WiFi.softAPIP());

  if (!_portalHandlersConfigured) {
    _portal.on("/", HTTP_GET, [this]() {
      _portal.send(200, "text/html", portalHtml());
    });
  _portal.on("/generate_204", HTTP_GET, [this]() {
    _portal.sendHeader("Location", "/");
    _portal.send(302, "text/plain", "");
  });
  _portal.on("/hotspot-detect.html", HTTP_GET, [this]() {
    _portal.sendHeader("Location", "/");
    _portal.send(302, "text/plain", "");
  });
  _portal.on("/save", HTTP_POST, [this]() {
    String ssid = _portal.arg("ssid");
    String wifi = _portal.arg("wifi");
    String serverInput = _portal.arg("server");
    String connectPath = normalizeApiPath(_portal.arg("connect_path"), DEFAULT_CONNECT_PATH);
    String runtimePath = normalizeApiPath(_portal.arg("runtime_path"), DEFAULT_RUNTIME_PATH);
    String apNameInput = _portal.arg("ap_name");
    String apPasswordInput = _portal.arg("ap_password");
    String pairing = normalizePairingCode(_portal.arg("session"));
    ssid.trim();
    serverInput.trim();
    apNameInput.trim();
    apPasswordInput.trim();

    if (!ssid.length()) {
      _portal.send(400, "text/html", "<h2>Wi-Fi SSID is required.</h2><p>Go back and enter the 2.4 GHz network used by this ElectriCredit module.</p>");
      return;
    }
    if (!_config.paired && !pairing.length()) {
      _portal.send(400, "text/html", "<h2>Session ID is required.</h2><p>Enter the 12-digit one-time Session ID from Management, or scan the setup QR again.</p>");
      return;
    }
    if (apPasswordInput.length() && (apPasswordInput.length() < 8 || apPasswordInput.length() > 63)) {
      _portal.send(400, "text/html", "<h2>Registration hotspot password is invalid.</h2><p>Use 8 to 63 characters, or leave it blank to keep the current password.</p>");
      return;
    }

    bool ssidChanged = ssid != _config.ssid;
    _config.ssid = ssid;
    if (wifi.length() || ssidChanged) _config.wifiPassword = wifi;
    _config.serverUrl = serverInput.length() ? normalizeServerUrl(serverInput) : "";
    _config.connectPath = connectPath;
    _config.runtimePath = runtimePath;
    const bool hubModule = _settings.moduleType && String(_settings.moduleType).equalsIgnoreCase("hub");
    if (hubModule) {
      _config.hotspotName = "Hub";
      _config.hotspotPassword = DEFAULT_AP_PASSWORD;
    } else {
      if (apNameInput.length()) _config.hotspotName = sanitizeHotspotName(apNameInput);
      if (apPasswordInput.length()) _config.hotspotPassword = sanitizeHotspotPassword(apPasswordInput);
    }
    String apName = _config.hotspotName;
    _config.lastProbeOk = false;
    _config.lastProbeMessage = "Saved. Connection test pending.";
    saveConfig();

    String page = "<!doctype html><meta name='viewport' content='width=device-width,initial-scale=1'><style>body{font-family:system-ui;background:#f4f7fb;margin:0;padding:24px;color:#172033}main{max-width:520px;margin:auto;background:white;padding:22px;border-radius:20px;box-shadow:0 12px 40px #0001}h2{margin-top:0}p{line-height:1.55;color:#576173}</style><main><h2>Settings saved</h2><p>The hotspot will close temporarily while this module tests <b>";
    page += escapeHtml(ssid);
    page += "</b> and checks the ElectriCredit server. If first-time pairing succeeds, Work Mode starts automatically; otherwise the hotspot reopens. Reconnect to <b>";
    page += escapeHtml(apName);
    page += "</b> to view the result or start Work Mode.</p></main>";
    _portal.send(200, "text/html", page);
    scheduleRegistrationProbe(pairing);
  });
  _portal.on("/work", HTTP_POST, [this]() {
    if (!_config.paired || !_config.ssid.length()) {
      _portal.send(409, "text/html", "<h2>Module is not ready for Work Mode.</h2><p>Save/test Wi-Fi and complete server pairing first.</p>");
      return;
    }
    _portal.send(200, "text/html", "<!doctype html><meta name='viewport' content='width=device-width,initial-scale=1'><style>body{font-family:system-ui;padding:24px;background:#f4f7fb;color:#172033}main{max-width:520px;margin:auto;background:#fff;padding:22px;border-radius:20px}</style><main><h2>Starting Work Mode</h2><p>The setup hotspot will close. The module will connect to its saved Wi-Fi and begin normal ElectriCredit operation.</p></main>");
      _workTransitionPending = true;
    _workTransitionAtMs = millis() + 700;
  });
    _portal.onNotFound([this]() {
      _portal.sendHeader("Location", "/");
      _portal.send(302, "text/plain", "");
    });
    _portalHandlersConfigured = true;
  }
  _portal.begin();
  _portalActive = true;
  _mode = Mode::Registration;
  _config.preferredMode = (_config.paired && _config.ssid.length()) ? Mode::Work : Mode::Registration;
  saveConfig();

  // Never expose the Registration-hotspot password on the OLED. The user only
  // needs the local setup URL once connected to the module hotspot.
  showScreen("Registration Mode", apName.substring(0, 20), "Setup portal", "http://192.168.4.1");
}

void ModuleCore::stopPortal() {
  if (_portalActive) {
    _dns.stop();
    _portal.stop();
    WiFi.softAPdisconnect(true);
    _portalActive = false;
  }
  // Leave the Wi-Fi driver alive. enterWorkMode()/connectWiFiBlocking() will
  // switch it to STA and perform the same 250 ms settle used by the old code.
  delay(50);
}

String ModuleCore::portalHtml() const {
  String type = _settings.moduleType ? _settings.moduleType : "module";
  String typeUpper = type;
  typeUpper.toUpperCase();
  String paired = _config.paired ? "Yes" : "No";
  String probeClass = _config.lastProbeOk ? "ok" : "warn";
  String probeText = _config.lastProbeMessage.length() ? _config.lastProbeMessage : "No connection test has been completed yet.";
  String moduleName = _config.moduleName.length() ? _config.moduleName : "Not assigned yet";
  String lastWifi = _config.lastWifi.length() ? _config.lastWifi : (_config.ssid.length() ? _config.ssid : "Not tested");
  String lastIp = _config.lastIp.length() ? _config.lastIp : "-";
  String portalMac = WiFi.macAddress();
  if (portalMac == "00:00:00:00:00:00") portalMac = "available after WLAN";
  String advancedServer = _config.serverUrl;

  String html;
  html.reserve(18000);
  html += R"HTML(<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#111722"><title>ElectriCredit Module Setup</title><style>
*{box-sizing:border-box}html{color-scheme:light}body{margin:0;font-family:Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;background:#f4f5f2;color:#171a20}button,input{font:inherit}main{width:min(820px,100%);margin:auto;padding:16px 12px 42px}.hero{position:relative;overflow:hidden;background:linear-gradient(150deg,#111722,#1d2a3a);color:#fff;padding:22px;border-radius:24px;box-shadow:0 20px 54px #0f172a24}.hero:after{content:"";position:absolute;width:180px;height:180px;border:1px solid #ffffff14;border-radius:50%;right:-85px;top:-90px}.eyebrow{font-size:11px;font-weight:850;letter-spacing:.12em;color:#f5d565}.hero h1{font-size:25px;letter-spacing:-.025em;margin:8px 0 6px}.hero p{max-width:60ch;margin:0;color:#c7d0dd;font-size:13px;line-height:1.6}.stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin-top:18px}.stat{min-width:0;background:#ffffff0d;border:1px solid #ffffff15;border-radius:14px;padding:11px}.stat small,.stat strong{display:block}.stat small{color:#9eacbf;font-size:10px}.stat strong{margin-top:4px;font-size:12px;overflow-wrap:anywhere}.status-card,.setup-card{background:#fff;border:1px solid #dedfd8;border-radius:20px;margin-top:12px;box-shadow:0 10px 28px #1720330a}.status-card{padding:15px}.status-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:9px}.status-head strong{font-size:14px}.pill{display:inline-flex;align-items:center;gap:5px;padding:5px 8px;border-radius:999px;background:#fff4cc;color:#715600;font-size:10px;font-weight:800}.pill:before{content:"";width:6px;height:6px;border-radius:50%;background:#d8a600}.pill.ok{background:#ebf8ef;color:#226238}.pill.ok:before{background:#28a45b}.status{padding:11px 12px;border-radius:12px;background:#fff8e5;border:1px solid #f2d578;color:#6e590d;font-size:12px;line-height:1.5}.status.ok{background:#edf9f1;border-color:#b9e5c5;color:#235e34}.meta{margin:9px 0 0;color:#777f8a;font-size:10px;line-height:1.6;overflow-wrap:anywhere}.diag-card{border-color:#e6c85b}.diag-intro{margin:0 0 10px;color:#6f7783;font-size:11px;line-height:1.5}.diag-list{display:grid;gap:7px}.diag-row{display:grid;grid-template-columns:24px 1fr;gap:8px;align-items:start;padding:9px;border:1px solid #ece5c6;border-radius:11px;background:#fffdf5}.diag-row span{display:grid;place-items:center;width:22px;height:22px;border-radius:50%;background:#f3c72e;color:#171a20;font-size:10px;font-weight:900}.diag-row p{margin:1px 0 0;font-size:11px;line-height:1.45;color:#4c5563;overflow-wrap:anywhere}.paired-note{padding:11px 12px;margin:0 0 12px;border:1px solid #bfd8ef;border-radius:12px;background:#eef7ff;color:#294d70;font-size:11px;line-height:1.5}.setup-card{padding:16px}.tabs{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:6px;margin-bottom:14px;padding:5px;border-radius:14px;background:#f0f1ed}.tab{border:0;border-radius:10px;background:transparent;padding:10px 8px;color:#69717b;font-size:11px;font-weight:800;cursor:pointer}.tab.active{background:#fff;color:#171a20;box-shadow:0 2px 8px #0001}.pane{display:none}.pane.active{display:block}.setup-title{display:flex;align-items:flex-start;gap:10px;margin-bottom:14px}.step{width:30px;height:30px;flex:0 0 30px;display:grid;place-items:center;border-radius:50%;background:#f3c72e;color:#16191e;font-weight:900;font-size:12px}.setup-title strong,.setup-title small{display:block}.setup-title strong{font-size:15px}.setup-title small{margin-top:2px;color:#727985;font-size:11px;line-height:1.45}.fields{display:grid;gap:12px}.field{display:grid;gap:5px}.field label{font-size:11px;font-weight:800;color:#404752}.field small{color:#7c8490;font-size:10px;line-height:1.45}input{width:100%;min-width:0;border:1px solid #cfd3d8;border-radius:12px;padding:12px;font-size:15px;background:#fff;color:#171a20;outline:none;transition:border-color .16s,box-shadow .16s}input:focus{border-color:#c39c12;box-shadow:0 0 0 3px #f3c72e22}.connection-note{display:flex;gap:9px;align-items:flex-start;padding:11px 12px;border:1px solid #e3d69f;border-radius:13px;background:#fffaf0;color:#615536}.connection-note b,.connection-note span{display:block}.connection-note b{font-size:11px}.connection-note span{margin-top:2px;color:#7a6b45;font-size:10px;line-height:1.5}.qr-import{display:none;align-items:center;justify-content:space-between;gap:8px;margin:12px 0 12px;padding:10px 11px;border:1px solid #afe0bc;border-radius:12px;background:#effaf2;color:#245f35;font-size:10px;font-weight:750}.qr-import.show{display:flex}.actions{display:grid;gap:8px;margin-top:18px}.btn{appearance:none;border:0;border-radius:13px;padding:13px 14px;font-size:13px;font-weight:850;cursor:pointer}.primary{background:#f3c72e;color:#171a20}.work{background:#182a41;color:#fff}.work-card{padding:14px 16px}.foot{font-size:10px;color:#7c8490;margin:12px 0 0;line-height:1.55}.section-label{margin:16px 2px 6px;color:#777f8a;font-size:10px;font-weight:800;text-transform:uppercase;letter-spacing:.08em}.hotspot-current{padding:11px 12px;margin-bottom:12px;border:1px solid #e1e4df;border-radius:12px;background:#f7f8f5;font-size:11px;line-height:1.55}.hotspot-current b{overflow-wrap:anywhere}.advanced-note{padding:10px 11px;border-radius:12px;background:#f7f8f5;color:#69717b;font-size:10px;line-height:1.55;margin-bottom:12px}.scan-cache{padding:14px 15px}.scan-cache-head{display:flex;justify-content:space-between;gap:10px;align-items:flex-start;margin-bottom:10px}.scan-cache-head strong,.scan-cache-head small{display:block}.scan-cache-head strong{font-size:13px}.scan-cache-head small{margin-top:2px;color:#7c8490;font-size:10px;line-height:1.45}.scan-list{display:grid;gap:7px}.scan-network{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;border:1px solid #dfe2dc;border-radius:12px;background:#f8f9f6;padding:10px 11px;text-align:left;color:inherit;cursor:pointer}.scan-network span,.scan-network small{display:block}.scan-network span{font-size:12px;font-weight:800;overflow-wrap:anywhere}.scan-network small{margin-top:2px;color:#7c8490;font-size:9px}.scan-network b{font-size:10px;white-space:nowrap;color:#5c6470}
@media(max-width:640px){main{padding:10px 9px 30px}.hero{padding:18px;border-radius:20px}.hero h1{font-size:22px}.stats{grid-template-columns:1fr 1fr}.setup-card{padding:14px}.status-head{align-items:flex-start;flex-direction:column}.qr-import{align-items:flex-start;flex-direction:column}.tabs{grid-template-columns:1fr}.tab{text-align:left;padding:9px 11px}}
@media(prefers-color-scheme:dark){html{color-scheme:dark}body{background:#0c1118;color:#edf1f6}.status-card,.setup-card{background:#131b25;border-color:#283342}.tabs{background:#0e151d}.tab{color:#98a5b4}.tab.active{background:#202b37;color:#fff}.field label{color:#d6dde6}.setup-title small,.field small,.meta,.foot{color:#98a5b4}input{background:#0d151e;color:#f4f7fb;border-color:#354354}.status{background:#292417;border-color:#67592d;color:#f0da88}.status.ok{background:#15271b;border-color:#315d3d;color:#9ad5aa}.connection-note{background:#252116;border-color:#554b2d;color:#e8d898}.connection-note span{color:#b9ab7b}.qr-import{background:#14251a;border-color:#315b3d;color:#a7ddb4}.hotspot-current,.advanced-note{background:#0e151d;border-color:#283342;color:#a8b3c0}.scan-network{background:#0e151d;border-color:#283342}.scan-cache-head small,.scan-network small,.scan-network b{color:#98a5b4}.diag-row{background:#181b16;border-color:#403c27}.diag-row p{color:#c2c9d2}.paired-note{background:#132231;border-color:#294c6d;color:#a9cae8}}
</style></head><body><main><section class="hero"><div class="eyebrow">REGISTRATION MODE · )HTML";
  html += escapeHtml(typeUpper);
  html += R"HTML(</div><h1>Connect this ElectriCredit module</h1><p>)HTML";
  html += _config.paired
      ? (_config.ssid.length()
          ? "This module is already paired and has saved Wi-Fi. On every restart it automatically tries the saved network first. Registration Mode is shown only because setup was requested. Try replugging the module and make sure the ElectriCredit server/hotspot is reachable. A Session ID is not required."
          : "This module is already paired but has no saved Wi-Fi SSID. Save a 2.4 GHz ElectriCredit network below so future restarts can reconnect automatically. A Session ID is not required.")
      : "First-time setup needs a 2.4 GHz Wi-Fi network and the one-time Session ID from Management. Name and Location are supplied by the server after pairing.";
  html += R"HTML(</p><div class="stats"><div class="stat"><small>Already registered</small><strong>)HTML";
  html += paired;
  html += R"HTML(</strong></div><div class="stat"><small>Server module</small><strong>)HTML";
  html += escapeHtml(moduleName);
  html += R"HTML(</strong></div><div class="stat"><small>Last Wi-Fi</small><strong>)HTML";
  html += escapeHtml(lastWifi);
  html += R"HTML(</strong></div><div class="stat"><small>Last IP</small><strong>)HTML";
  html += escapeHtml(lastIp);
  html += R"HTML(</strong></div></div></section><section class="status-card"><div class="status-head"><strong>Connection status</strong><span class="pill )HTML";
  html += (_config.lastProbeOk ? "ok" : "");
  html += R"HTML(">)HTML";
  html += (_config.lastProbeOk ? "Ready" : "Setup");
  html += R"HTML(</span></div><div class="status )HTML";
  html += probeClass;
  html += "\">" + escapeHtml(probeText) + "</div><p class=\"meta\">Server: " + escapeHtml(_config.serverUrl.length() ? _config.serverUrl : "Automatic from Wi-Fi gateway") + "<br>WLAN diagnostic: " + escapeHtml(wifiDiagnosticSummary()) + "<br>Module ID: " + String((long long)_config.moduleId) + " · MAC: " + escapeHtml(portalMac) + "<br>Firmware: " + escapeHtml(_settings.firmwareVersion ? _settings.firmwareVersion : "unknown") + " · ModuleCore: " + escapeHtml(EMBEDDED_CORE_VERSION) + "</p></section>";

  html += workDiagnosticHtml();

  if (_registrationScanCount > 0) {
    html += "<section class=\"status-card scan-cache\"><div class=\"scan-cache-head\"><div><strong>Nearby Wi-Fi saved from the last failed test</strong><small>This is temporary registration cache. Tap an SSID to copy it into Connection, then enter its password if needed.</small></div><span class=\"pill\">" + String((int)_registrationScanCount) + " found</span></div><div class=\"scan-list\">";
    for (uint8_t i = 0; i < _registrationScanCount; i++) {
      String detail = wifiAuthLabel(_registrationScanAuth[i]) + " · CH " + String((long)_registrationScanChannel[i]);
      if (_registrationScanBSSID[i].length()) detail += " · " + _registrationScanBSSID[i];
      html += "<button type=\"button\" class=\"scan-network\" data-cached-ssid=\"" + escapeHtml(_registrationScanSSID[i]) + "\"><div><span>" + escapeHtml(_registrationScanSSID[i]) + "</span><small>" + escapeHtml(detail) + "</small></div><b>" + String((long)_registrationScanRSSI[i]) + " dBm</b></button>";
    }
    html += "</div></section>";
  }

  html += R"HTML(<form method="post" action="/save" id="setup-form"><div class="section-label">Module setup</div><section class="setup-card"><nav class="tabs" aria-label="Setup sections"><button class="tab active" type="button" data-tab="connection">Connection</button><button class="tab" type="button" data-tab="hotspot">Registration hotspot</button><button class="tab" type="button" data-tab="advanced">Advanced</button></nav><div class="pane active" data-pane="connection"><div class="setup-title"><span class="step">1</span><div><strong>Connect to ElectriCredit</strong><small>Use a 2.4 GHz network. ESP32 modules cannot join a 5 GHz-only SSID.</small></div></div><div class="connection-note"><span>▣</span><div><b>Management QR</b><span>Recommended: connect this phone to the ESP32 Registration hotspot, scan the single QR from Management, and these connection fields are filled automatically.</span></div></div><div class="qr-import" id="qr-import"><span>✓ SCANNED · Setup details imported from Management QR.</span><button type="button" class="btn" id="clear-import">Clear import</button></div>)HTML";
  if (_config.paired) {
    if (_config.ssid.length()) {
      html += R"HTML(<div class="paired-note"><b>Already paired with saved Wi-Fi.</b> Every restart automatically tries this saved network before opening Registration Mode. If Work Mode cannot connect, verify the ElectriCredit server/hotspot is powered on and reachable. No Session ID is required.</div>)HTML";
    } else {
      html += R"HTML(<div class="paired-note"><b>Already paired, but Wi-Fi is not saved.</b> Save a 2.4 GHz network below. After that, future restarts will automatically reconnect without a Session ID.</div>)HTML";
    }
  }
  html += R"HTML(<div class="fields"><div class="field"><label for="wifi-ssid">Wi-Fi SSID</label><input id="wifi-ssid" name="ssid" autocomplete="off" required value=")HTML";
  html += escapeHtml(_config.ssid);
  html += R"HTML(" placeholder="ElectriCredit Server"><small>For Raspberry Pi deployment, use the ElectriCredit 2.4 GHz hotspot rather than the Pi's 5 GHz Internet uplink.</small></div><div class="field"><label for="wifi-password">Wi-Fi password</label><input id="wifi-password" name="wifi" type="password" autocomplete="new-password" placeholder="Leave blank to keep saved password"><small>The Management QR can fill this automatically when the server knows the password.</small></div><div class="field"><label for="pairing-session">Session ID</label><input id="pairing-session" name="session" autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder="3583-0650-9058"><small>)HTML";
  html += _config.paired
      ? "Optional. Leave blank for normal reconnect testing."
      : "Required for first-time registration. Create it from Management → Hub/Registry → Add.";
  html += R"HTML(</small></div></div></div><div class="pane" data-pane="hotspot"><div class="setup-title"><span class="step">2</span><div><strong>Registration hotspot</strong><small>)HTML";
  const bool hubPortal = _settings.moduleType && String(_settings.moduleType).equalsIgnoreCase("hub");
  html += hubPortal ? "Hub setup uses a fixed hotspot for predictable recovery." : "Optional. Change how this ESP32 setup hotspot appears when you need Registration Mode again.";
  html += R"HTML(</small></div></div><div class="hotspot-current">Current hotspot: <b>)HTML";
  html += escapeHtml(_config.hotspotName);
  html += R"HTML(</b></div>)HTML";
  if (hubPortal) {
    html += R"HTML(<div class="advanced-note">Hub Registration Mode is fixed to SSID <b>Hub</b> with the configured default setup password. These values are not changed by Save & Test.</div><input type="hidden" name="ap_name" value="Hub"><input type="hidden" name="ap_password" value="1234567890">)HTML";
  } else {
    html += R"HTML(<div class="fields"><div class="field"><label for="ap-name">New hotspot name</label><input id="ap-name" name="ap_name" maxlength="31" placeholder="Leave blank to keep the current name"><small>This affects only the ESP32 Registration hotspot, not the server hotspot.</small></div><div class="field"><label for="ap-password">New hotspot password</label><input id="ap-password" name="ap_password" type="password" autocomplete="new-password" minlength="8" maxlength="63" placeholder="Leave blank to keep the current password"><small>Use 8 to 63 characters. If changed, reconnect using the new password after Save & Test.</small></div></div>)HTML";
  }
  html += R"HTML(</div><div class="pane" data-pane="advanced"><div class="setup-title"><span class="step">3</span><div><strong>Advanced connection</strong><small>Normally leave these unchanged. They are available for LAN/development setups that cannot discover the server from the Wi-Fi gateway.</small></div></div><div class="advanced-note">When Server URL is blank, the ESP32 automatically tries the Wi-Fi gateway on port 8080. This is the simplest path for the ElectriCredit server hotspot.</div><div class="fields"><div class="field"><label for="server-url">Server / API base URL</label><input id="server-url" name="server" value=")HTML";
  html += escapeHtml(advancedServer);
  html += R"HTML(" placeholder="Automatic from Wi-Fi gateway"><small>Management QR fills this only when an explicit LAN server address is required.</small></div><div class="field"><label>Module connect API</label><input id="connect-path" name="connect_path" value=")HTML";
  html += escapeHtml(_config.connectPath);
  html += R"HTML("></div><div class="field"><label>Runtime API</label><input id="runtime-path" name="runtime_path" value=")HTML";
  html += escapeHtml(_config.runtimePath);
  html += R"HTML("></div></div></div><div class="actions"><button class="btn primary" type="submit">Save & Test Connection</button></div><p class="foot">Save & Test Connection temporarily closes the setup hotspot and tests Wi-Fi/server access. A Session ID is used only for first-time pairing; paired modules reuse their permanent saved identity.</p></section></form>)HTML";

  if (_config.paired) {
    html += R"HTML(<section class="setup-card work-card"><div class="setup-title"><span class="step">✓</span><div><strong>Ready for Work Mode</strong><small>This module already has server credentials. Start Work Mode when the connection result is correct.</small></div></div><form method="post" action="/work"><div class="actions"><button class="btn work" type="submit">Start Work Mode</button></div></form></section>)HTML";
  }

  html += R"HTML(<p class="foot">Registration Mode is AP-only. The ESP32 switches to station mode only during Save & Test and Work Mode.</p></main><script>
(function(){
 var params=new URLSearchParams(location.search);
 var imported=false;
 var importedKeys=[];
 var fields={ssid:document.getElementById("wifi-ssid"),wifi:document.getElementById("wifi-password"),server:document.getElementById("server-url"),session:document.getElementById("pairing-session"),connect:document.getElementById("connect-path"),runtime:document.getElementById("runtime-path")};
 ["ssid","wifi","server","session","connect","runtime"].forEach(function(key){var value=params.get(key)||"";if(value&&fields[key]){fields[key].value=key==="session"?value.toUpperCase():value;importedKeys.push(key);imported=true}});
 if(params.get("gateway")==="1"&&fields.server){fields.server.value="";if(importedKeys.indexOf("server")<0)importedKeys.push("server");imported=true}
 if(imported){document.getElementById("qr-import").classList.add("show")}
 if(imported&&history.replaceState){history.replaceState({},"",location.pathname)}
 if(fields.session){fields.session.addEventListener("input",function(){this.value=this.value.toUpperCase().replace(/[^A-Z0-9-]/g,"")})}
 document.getElementById("clear-import").addEventListener("click",function(){importedKeys.forEach(function(key){if(fields[key])fields[key].value=""});importedKeys=[];document.getElementById("qr-import").classList.remove("show")});
 document.querySelectorAll("[data-cached-ssid]").forEach(function(button){button.addEventListener("click",function(){if(fields.ssid)fields.ssid.value=button.getAttribute("data-cached-ssid")||"";document.querySelector('[data-tab="connection"]')?.click();fields.ssid?.scrollIntoView({behavior:"smooth",block:"center"})})});
 document.querySelectorAll("[data-tab]").forEach(function(button){button.addEventListener("click",function(){var target=button.getAttribute("data-tab");document.querySelectorAll("[data-tab]").forEach(function(item){item.classList.toggle("active",item===button)});document.querySelectorAll("[data-pane]").forEach(function(pane){pane.classList.toggle("active",pane.getAttribute("data-pane")===target)})})});
})();
</script></body></html>)HTML";
  return html;
}

void ModuleCore::scheduleRegistrationProbe(const String& pairingCode) {
  _probePairingCode = pairingCode;
  _probePending = true;
  _probeAtMs = millis() + 700;
}

void ModuleCore::serviceRegistrationMode() {
  if (_portalActive) {
    _dns.processNextRequest();
    _portal.handleClient();
  }

  if (_workTransitionPending && (long)(millis() - _workTransitionAtMs) >= 0) {
    _workTransitionPending = false;
    enterWorkMode();
    return;
  }

  if (_probePending && (long)(millis() - _probeAtMs) >= 0) {
    _probePending = false;
    runRegistrationProbe();
  }
}

void ModuleCore::runRegistrationProbe() {
  String pairing = _probePairingCode;
  _probePairingCode = "";
  stopPortal();

  String message;
  bool ok = false;
  bool freshPairingSucceeded = false;
  showScreen("Connecting WLAN", oledText("SSID: " + _config.ssid), "Connection: trying", "Please wait");
  bool wifiOK = connectWiFiBlocking(_config.ssid, _config.wifiPassword, 18000);

  if (!wifiOK) {
    showScreen("WLAN FAILED", oledText("SSID: " + _config.ssid), "Connection: FAILED", "Starting scan...");
    delay(1100);
    showScreen("SCANNING NEARBY", "Searching all Wi-Fi", "Saving setup cache", "Please wait");
    scanNearbyForRegistration();
    message = wifiFailureMessage(_config.ssid, _config.wifiPassword);
    String reasonLine = _lastWifiDisconnectReason ? ("Reason: " + String((int)_lastWifiDisconnectReason)) : ("Status: " + String(_lastWifiStatusCode));
    showScreen("WLAN FAILED", oledText(reasonLine), oledText(message), "Saved APs: " + String((int)_registrationScanCount));
    delay(2400);
  } else {
    _config.lastWifi = WiFi.SSID();
    _config.lastIp = WiFi.localIP().toString();
    showScreen("WLAN Connected", oledText("SSID: " + _config.lastWifi), oledText("IP: " + _config.lastIp), "Connection: SUCCESS");
    delay(900);

    String gatewayServer = serverFromGateway();
    if (!_config.serverUrl.length()) _config.serverUrl = gatewayServer;

    showScreen("Checking Server", oledText(_config.serverUrl), "API: /public/home", "Please wait");
    String serverMessage;
    bool serverOK = testBaseServer(serverMessage);
    if (!serverOK && gatewayServer.length() && gatewayServer != _config.serverUrl) {
      String oldServer = _config.serverUrl;
      _config.serverUrl = gatewayServer;
      showScreen("Retry Server", oledText(gatewayServer), "Using Wi-Fi gateway", "Please wait");
      if (!testBaseServer(serverMessage)) {
        _config.serverUrl = oldServer;
      } else {
        serverOK = true;
      }
    }

    if (!serverOK) {
      message = serverMessage;
      showScreen("Server FAILED", oledText(message), oledText(_config.serverUrl), "Returning hotspot");
      delay(1700);
    } else if (pairing.length()) {
      showScreen("Server Connected", oledText(_config.serverUrl), "Connection: SUCCESS", "Connecting session");
      delay(700);
      showScreen("Connecting Session", oledText("ID: " + pairing), oledText(String(_settings.moduleType ? _settings.moduleType : "module")), "Please wait");
      ok = pairWithServer(pairing, message);

      // If an old explicit LAN server is still reachable but does not know the
      // freshly-created session, retry the Wi-Fi gateway once. This prevents a
      // stale saved server URL from trapping a newly scanned hotspot QR.
      if (!ok && gatewayServer.length() && gatewayServer != _config.serverUrl &&
          (_lastPairingErrorCode == "SESSION_NOT_FOUND" || _lastPairingErrorCode == "TRANSPORT")) {
        String oldServer = _config.serverUrl;
        _config.serverUrl = gatewayServer;
        String gatewayHealth;
        if (testBaseServer(gatewayHealth)) {
          showScreen("Retry Session", oledText("ID: " + pairing), "Using Wi-Fi gateway", "Please wait");
          String retryMessage;
          if (pairWithServer(pairing, retryMessage)) {
            ok = true;
            message = retryMessage;
          } else {
            message = retryMessage;
          }
        } else {
          _config.serverUrl = oldServer;
        }
      }

      if (ok) {
        freshPairingSucceeded = true;
        clearRegistrationScanCache();
        showScreen("Registration OK", oledText(_config.moduleName.length() ? _config.moduleName : "Module registered"), "Credentials saved", "Starting Work Mode");
        delay(1100);
      } else {
        String shortReason = message;
        if (_lastPairingErrorCode == "SESSION_NOT_FOUND") shortReason = "Session not found";
        else if (_lastPairingErrorCode == "SESSION_EXPIRED") shortReason = "Session expired";
        else if (_lastPairingErrorCode == "SESSION_NOT_WAITING") shortReason = "Session already used";
        else if (_lastPairingErrorCode == "WRONG_MODULE_TYPE") shortReason = "Wrong module type";
        showScreen("Session FAILED", oledText(shortReason), oledText("ID: " + pairing), "Opening hotspot");
        delay(1900);
      }
    } else if (_config.paired) {
      showScreen("Checking Module", oledText(_config.moduleName), "Runtime credentials", "Please wait");
      ok = testSavedServer(message);
      if (ok) {
        clearRegistrationScanCache();
        clearConnectionDiagnostics();
        showScreen("Module SUCCESS", _config.moduleName, "Runtime API valid", "Returning hotspot");
        delay(1300);
      } else {
        showScreen("Module FAILED", oledText(message), "Runtime API failed", "Returning hotspot");
        delay(1700);
      }
    } else {
      message = "Server reachable. Enter a pairing Session ID before Work Mode.";
      showScreen("Server SUCCESS", oledText(_config.serverUrl), "Session ID required", "Returning hotspot");
      delay(1300);
    }
  }

  _config.lastProbeOk = ok;
  _config.lastProbeMessage = message;
  if (!ok && message.length()) recordConnectionDiagnostic("Registration test: " + message);

  // A one-time pairing Session is only needed to establish the module record.
  // Once the server has returned the permanent module ID/token, immediately
  // enter Work Mode and keep the successful STA connection alive. Reopening
  // the setup hotspot here used to make registration feel like it required a
  // second Save & Test and needlessly tore down the just-working Wi-Fi link.
  if (freshPairingSucceeded && _config.paired) {
    _config.preferredMode = Mode::Work;
    saveConfig();
    _mode = Mode::Work;
    _lastWifiAttemptMs = 0;
    _wifiAttemptStartedMs = 0;
    _wifiAttemptInProgress = false;
    _lastMqttAttemptMs = 0;
    _lastHeartbeatMs = 0;
    WiFi.setAutoReconnect(true);
    clearConnectionDiagnostics();
    showTemporary("System", "Registered", _config.moduleName, "Work Mode starting", 1800);
    startBuzzerPattern(110, 90, 2);
    return;
  }

  saveConfig();
  disconnectMqtt();
  WiFi.disconnect(true, false);
  WiFi.mode(WIFI_OFF);
  delay(120);
  startPortal();

  if (ok) {
    showTemporary("Setup Check OK", _config.moduleName.length() ? _config.moduleName.substring(0, 20) : "Server reachable", "Reconnect to hotspot", "Work Mode available", 5000);
    startBuzzerPattern(100, 80, 1);
  } else {
    showTemporary("Setup Check Failed", oledText(message), "Reconnect to hotspot", "Review diagnostics", 7000);
    startBuzzerPattern(160, 110, 3);
  }
}


bool ModuleCore::waitForWiFiConnected(unsigned long timeoutMs) {
  const unsigned long started = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - started < timeoutMs) {
    serviceBuzzer();
    serviceButton();
    if (_registrationRequested && _mode == Mode::Work) {
      _lastWifiStatusCode = (int)WiFi.status();
      recordConnectionDiagnostic("Wi-Fi join cancelled by 3-second Registration button request.");
      return false;
    }
    delay(20);
  }
  _lastWifiStatusCode = (int)WiFi.status();
  return _lastWifiStatusCode == WL_CONNECTED;
}

bool ModuleCore::connectWiFiBlocking(const String& ssid, const String& password, unsigned long timeoutMs) {
  if (!ssid.length()) return false;

  _lastWifiDisconnectReason = 0;
  _lastWifiStatusCode = WL_IDLE_STATUS;
  _lastWifiTargetFound = false;
  _lastWifiTargetRSSI = 0;
  _lastWifiTargetChannel = 0;
  _lastWifiTargetAuth = 0;
  _lastWifiTargetBSSID = "";
  _lastWifiConnectStrategy = "legacy saved SSID/password";

  // Exact connection sequence from the pre-oral firmware that joined the
  // ElectriCredit hotspot reliably and quickly.
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  WiFi.setAutoReconnect(true);
  WiFi.disconnect(false, false);
  delay(250);
  _lastWifiDisconnectReason = 0;
  WiFi.begin(ssid.c_str(), password.length() ? password.c_str() : nullptr);

  bool ok = waitForWiFiConnected(timeoutMs);
  Serial.printf("[EC][WiFi] legacy result=%s status=%d reason=%u (%s) ip=%s\n",
                ok ? "connected" : "failed", _lastWifiStatusCode,
                (unsigned)_lastWifiDisconnectReason, wifiReasonMessage(_lastWifiDisconnectReason).c_str(),
                ok ? WiFi.localIP().toString().c_str() : "-");
  return ok;
}

// -----------------------------------------------------------------------------
// Pairing / HTTP
// -----------------------------------------------------------------------------

bool ModuleCore::httpPostJson(const String& url, const String& payload, String& response, int& statusCode, bool authorizedRuntime) {
  response = "";
  statusCode = -1;
  HTTPClient http;
  bool begun = false;

  if (url.startsWith("https://")) {
    WiFiClientSecure secure;
    // Supports user-configurable public HTTPS URLs without bundling a CA in
    // every sketch. Transport is encrypted, but certificate verification is
    // intentionally relaxed until a deployment CA/pinning policy is finalized.
    secure.setInsecure();
    begun = http.begin(secure, url);
    if (!begun) return false;
    http.setConnectTimeout(authorizedRuntime ? 900 : 4000);
    http.setTimeout(authorizedRuntime ? 1800 : 8000);
    http.addHeader("Content-Type", "application/json");
    if (authorizedRuntime) {
      http.addHeader("Authorization", "Bearer " + _config.moduleToken);
      http.addHeader("X-ElectriCredit-Module-Type", _settings.moduleType);
      http.addHeader("X-ElectriCredit-Module-ID", String((long long)_config.moduleId));
      http.addHeader("X-ElectriCredit-Module-MAC", WiFi.macAddress());
    }
    statusCode = http.POST(payload);
    if (statusCode > 0) response = http.getString();
    http.end();
    return statusCode > 0;
  }

  WiFiClient plain;
  begun = http.begin(plain, url);
  if (!begun) return false;
  http.setConnectTimeout(authorizedRuntime ? 900 : 4000);
  http.setTimeout(authorizedRuntime ? 1800 : 8000);
  http.addHeader("Content-Type", "application/json");
  if (authorizedRuntime) {
    http.addHeader("Authorization", "Bearer " + _config.moduleToken);
    http.addHeader("X-ElectriCredit-Module-Type", _settings.moduleType);
    http.addHeader("X-ElectriCredit-Module-ID", String((long long)_config.moduleId));
    http.addHeader("X-ElectriCredit-Module-MAC", WiFi.macAddress());
  }
  statusCode = http.POST(payload);
  if (statusCode > 0) response = http.getString();
  http.end();
  return statusCode > 0;
}

bool ModuleCore::httpGet(const String& url, String& response, int& statusCode) {
  response = "";
  statusCode = -1;
  HTTPClient http;
  bool begun = false;
  if (url.startsWith("https://")) {
    WiFiClientSecure secure;
    secure.setInsecure();
    begun = http.begin(secure, url);
    if (!begun) return false;
    http.setConnectTimeout(4000);
    http.setTimeout(7000);
    statusCode = http.GET();
    if (statusCode > 0) response = http.getString();
    http.end();
    return statusCode > 0;
  }
  WiFiClient plain;
  begun = http.begin(plain, url);
  if (!begun) return false;
  http.setConnectTimeout(4000);
  http.setTimeout(7000);
  statusCode = http.GET();
  if (statusCode > 0) response = http.getString();
  http.end();
  return statusCode > 0;
}

bool ModuleCore::pairWithServer(const String& pairingCode, String& humanMessage) {
  _lastPairingErrorCode = "";
  if (WiFi.status() != WL_CONNECTED) {
    _lastPairingErrorCode = "WIFI_DISCONNECTED";
    humanMessage = "Wi-Fi is not connected.";
    return false;
  }
  String normalized = normalizePairingCode(pairingCode);
  if (!normalized.length()) {
    _lastPairingErrorCode = "INVALID_SESSION";
    humanMessage = "Pairing session ID is required for registration.";
    return false;
  }

  JsonDocument body;
  body["session_id"] = normalized;
  body["mac_address"] = WiFi.macAddress();
  JsonObject info = body["information"].to<JsonObject>();
  info["firmware"] = _settings.firmwareVersion;
  info["board"] = _settings.boardName;
  info["module_type"] = _settings.moduleType;
  info["hotspot_name"] = _config.hotspotName;
  info["connect_api"] = _config.connectPath;
  info["runtime_api"] = _config.runtimePath;

  String payload;
  serializeJson(body, payload);
  String response;
  int code = -1;
  if (!httpPostJson(endpointUrl(_config.connectPath), payload, response, code, false)) {
    _lastPairingErrorCode = "TRANSPORT";
    humanMessage = "Unable to reach the module connect API.";
    return false;
  }
  if (code < 200 || code >= 300) {
    JsonDocument err;
    if (deserializeJson(err, response) == DeserializationError::Ok) {
      _lastPairingErrorCode = String(err["code"] | "");
      humanMessage = String(err["error"] | "");
      if (!humanMessage.length()) humanMessage = String(err["message"] | "Module registration was rejected.");
    } else {
      _lastPairingErrorCode = "HTTP_" + String(code);
      humanMessage = "Module registration was rejected by the server (HTTP " + String(code) + ").";
    }
    if (_lastPairingErrorCode == "SESSION_NOT_FOUND") humanMessage = "Session not found on this ElectriCredit server.";
    else if (_lastPairingErrorCode == "SESSION_EXPIRED") humanMessage = "Session expired. Create a new setup session.";
    else if (_lastPairingErrorCode == "SESSION_NOT_WAITING") humanMessage = "Session is no longer waiting. Create a new setup session.";
    else if (_lastPairingErrorCode == "WRONG_MODULE_TYPE") humanMessage = "Session belongs to a different module type.";
    else if (_lastPairingErrorCode == "INVALID_SESSION") humanMessage = "Session ID is invalid.";
    return false;
  }

  JsonDocument doc;
  if (deserializeJson(doc, response) != DeserializationError::Ok) {
    _lastPairingErrorCode = "UNREADABLE_RESPONSE";
    humanMessage = "Server returned an unreadable pairing response.";
    return false;
  }
  bool connected = doc["connected"] | false;
  String kind = doc["module"]["type"] | "";
  if (!connected || !kind.equalsIgnoreCase(_settings.moduleType)) {
    _lastPairingErrorCode = "WRONG_MODULE_TYPE";
    humanMessage = "Pairing response does not match this module type.";
    return false;
  }

  _config.moduleId = doc["module"]["id"] | 0LL;
  _config.moduleToken = doc["module_token"] | "";
  _config.moduleName = doc["module"]["name"] | "";
  _config.moduleLocation = doc["module"]["location"] | "";
  _config.moduleStatus = doc["module"]["status"] | "";

  _config.mqttEnabled = doc["mqtt"]["enabled"] | true;
  _config.mqttTls = doc["mqtt"]["tls"] | false;
  _config.mqttHost = doc["mqtt"]["host"] | "";
  if (doc["mqtt"]["port"].is<const char*>()) {
    _config.mqttPort = (uint16_t)String((const char*)doc["mqtt"]["port"]).toInt();
  } else {
    _config.mqttPort = doc["mqtt"]["port"] | (_config.mqttTls ? 8883 : 1883);
  }
  if (!_config.mqttPort) _config.mqttPort = _config.mqttTls ? 8883 : 1883;
  _config.mqttUser = doc["mqtt"]["username"] | "";
  _config.mqttPassword = doc["mqtt"]["password"] | "";
  _config.eventTopic = doc["mqtt"]["event_topic"] | "";
  _config.commandTopic = doc["mqtt"]["command_topic"] | "";

  _config.paired = _config.moduleId > 0 && _config.moduleToken.length() > 0;
  if (!_config.paired) {
    _lastPairingErrorCode = "MISSING_CREDENTIALS";
    humanMessage = "Server did not return usable module credentials.";
    return false;
  }
  _lastPairingErrorCode = "";
  saveConfig();
  humanMessage = _config.moduleName.length() ? ("Registered as " + _config.moduleName + ".") : "Module registered successfully.";
  return true;
}

bool ModuleCore::testBaseServer(String& message) {
  if (WiFi.status() != WL_CONNECTED) {
    message = "Wi-Fi is not connected.";
    return false;
  }
  String response;
  int code = -1;
  bool transport = httpGet(normalizeServerUrl(_config.serverUrl) + "/api/public/home", response, code);
  if (!transport) {
    message = "Server URL could not be reached.";
    return false;
  }
  if (code < 200 || code >= 300) {
    message = "Server answered, but the public API check failed (HTTP " + String(code) + ").";
    return false;
  }
  message = "Wi-Fi and server URL are reachable.";
  return true;
}

bool ModuleCore::testSavedServer(String& humanMessage) {
  if (!_config.paired) return testBaseServer(humanMessage);
  JsonDocument req;
  addCommonEventFields(req, "heartbeat", makeCorrelationId("probe"));
  if (_healthRfidApplicable) req["rfid_ok"] = _healthRfidOk;
  if (_healthPzemApplicable) req["pzem_ok"] = _healthPzemOk;

  String payload;
  serializeJson(req, payload);
  String response;
  int code = -1;
  if (!httpPostJson(endpointUrl(_config.runtimePath), payload, response, code, true)) {
    humanMessage = "Runtime API could not be reached.";
    return false;
  }
  JsonDocument doc;
  if (deserializeJson(doc, response) != DeserializationError::Ok) {
    humanMessage = "Runtime API returned unreadable data.";
    return false;
  }
  bool ok = (code >= 200 && code < 300) && (doc["ok"] | false);
  if (!ok) {
    String apiCode = doc["code"] | "";
    String msg = doc["message"] | "Saved module credentials were rejected.";
    if (apiCode == "UNAUTHORIZED") msg += " The saved permanent module credential no longer matches this module record.";
    humanMessage = msg;
    return false;
  }
  humanMessage = "Saved module credentials and Runtime API are valid.";
  return true;
}

bool ModuleCore::httpRuntimeRequest(JsonDocument& request, String& responsePayload) {
  if (!_config.paired || WiFi.status() != WL_CONNECTED) return false;
  String payload;
  serializeJson(request, payload);

  int code = -1;
  String primary = endpointUrl(_config.runtimePath);
  if (httpPostJson(primary, payload, responsePayload, code, true)) {
    return code >= 200 && code < 500; // caller parses server-level ok/code
  }

  // The saved server address can become stale when the same paired module is
  // moved between the Raspberry Pi hotspot and a Windows development hotspot.
  // If the saved endpoint is unreachable, retry the current Wi-Fi gateway once
  // and remember it when an ElectriCredit runtime endpoint answers.
  String gateway = serverFromGateway();
  if (!gateway.length()) return false;
  gateway = normalizeServerUrl(gateway);
  if (gateway == normalizeServerUrl(_config.serverUrl)) return false;

  String gatewayEndpoint = gateway + normalizeApiPath(_config.runtimePath, DEFAULT_RUNTIME_PATH);
  responsePayload = "";
  code = -1;
  if (!httpPostJson(gatewayEndpoint, payload, responsePayload, code, true)) return false;
  if (code >= 200 && code < 500) {
    _config.serverUrl = gateway;
    saveConfig();
    return true;
  }
  return false;
}

bool ModuleCore::responseOk(const String& raw, JsonDocument& doc) const {
  if (deserializeJson(doc, raw) != DeserializationError::Ok) return false;
  return doc["ok"] | false;
}

// -----------------------------------------------------------------------------
// MQTT / runtime
// -----------------------------------------------------------------------------

void ModuleCore::rebuildMqttClientIfNeeded() {
  if (_mqtt && _mqttBuiltForTls == _config.mqttTls) return;
  if (_mqtt) {
    _mqtt->stop();
    delete _mqtt;
    _mqtt = nullptr;
  }
  if (_config.mqttTls) {
    _mqttSecureNet.setInsecure();
    _mqtt = new MqttClient(_mqttSecureNet);
  } else {
    _mqtt = new MqttClient(_mqttPlainNet);
  }
  _mqttBuiltForTls = _config.mqttTls;
  if (_mqtt) {
    _mqtt->setConnectionTimeout(4000);
    _mqtt->setKeepAliveInterval(20000);
    _mqtt->onMessage(mqttMessageThunk);
  }
}

bool ModuleCore::connectMqtt() {
  if (!_config.paired || !_config.mqttEnabled || WiFi.status() != WL_CONNECTED || !_config.mqttHost.length()) return false;
  rebuildMqttClientIfNeeded();
  if (!_mqtt) return false;
  if (_mqtt->connected()) return true;

  String clientId = "ec-" + String(_settings.moduleType) + "-" + macCompact();
  _mqtt->setId(clientId);
  _mqtt->setCleanSession(true);
  if (_config.mqttUser.length()) _mqtt->setUsernamePassword(_config.mqttUser, _config.mqttPassword);
  if (!_mqtt->connect(_config.mqttHost.c_str(), _config.mqttPort)) return false;
  if (_config.commandTopic.length()) _mqtt->subscribe(_config.commandTopic, 1);
  sendHeartbeat();
  return true;
}

void ModuleCore::disconnectMqtt() {
  if (_mqtt) _mqtt->stop();
}

bool ModuleCore::mqttConnected() const {
  return _mqtt && _mqtt->connected();
}

bool ModuleCore::workNetworkReady() const {
  return _config.paired && WiFi.status() == WL_CONNECTED;
}

String ModuleCore::workConnectionMessage() const {
  if (!_config.paired) return "Module is not paired";
  if (!_config.ssid.length()) return "Wi-Fi SSID is not saved";
  if (WiFi.status() == WL_CONNECTED) return "Connected to " + WiFi.SSID();
  if (_wifiScanInProgress) return "Scanning for saved Wi-Fi";
  if (_wifiAttemptInProgress) return "Connecting to saved Wi-Fi";
  if (_wifiNextActionMs && (long)(_wifiNextActionMs - millis()) > 0) return "Wi-Fi unavailable - rescanning";
  if (_lastWifiDisconnectReason == 2) return "Wi-Fi handshake timed out - retrying";
  if (_lastWifiDisconnectReason == 15 || _lastWifiDisconnectReason == 202 || _lastWifiDisconnectReason == 204) return "Password/authentication failed";
  if (_lastWifiDisconnectReason == 201 || WiFi.status() == WL_NO_SSID_AVAIL) return "SSID not found - scanning/retrying";
  if (WiFi.status() == WL_CONNECT_FAILED) return "Wi-Fi authentication failed";
  if (WiFi.status() == WL_CONNECTION_LOST) return "Wi-Fi connection lost - retrying";
  if (_lastWifiDisconnectReason) return wifiReasonMessage(_lastWifiDisconnectReason);
  return "Connecting / retrying Wi-Fi";
}

String ModuleCore::serverConnectionMessage() const {
  String msg = _config.lastProbeMessage;
  msg.trim();
  if (!msg.length()) return "Checking server";
  if (msg.indexOf("MODULE_NOT_FOUND") >= 0) return "Module missing - pair again";
  if (msg.indexOf("UNAUTHORIZED") >= 0) return "Credential rejected - pair again";
  if (msg.indexOf("unreachable") >= 0 || msg.indexOf("could not be reached") >= 0) return "Server unreachable - retrying";
  if (msg.length() > 44) msg = msg.substring(0, 44);
  return msg;
}

void ModuleCore::recordConnectionDiagnostic(const String& message) {
  String clean = message;
  clean.trim();
  if (!clean.length()) return;

  // Avoid flooding the small RAM history with the same repeated state.
  String comparable = clean;
  if (_workDiagCount) {
    String last = _workDiag[_workDiagCount - 1];
    int sep = last.indexOf(" | ");
    if (sep >= 0) last = last.substring(sep + 3);
    if (last == comparable) return;
  }

  String entry = String(millis() / 1000UL) + "s | " + clean;
  if (_workDiagCount < WORK_DIAG_LIMIT) {
    _workDiag[_workDiagCount++] = entry;
  } else {
    for (uint8_t i = 1; i < WORK_DIAG_LIMIT; i++) _workDiag[i - 1] = _workDiag[i];
    _workDiag[WORK_DIAG_LIMIT - 1] = entry;
  }
  Serial.printf("[EC][DIAG] %s\n", clean.c_str());
}

void ModuleCore::clearConnectionDiagnostics() {
  for (uint8_t i = 0; i < WORK_DIAG_LIMIT; i++) _workDiag[i] = "";
  _workDiagCount = 0;
  _workReconnectStartedMs = 0;
  _workAttemptNumber = 0;
}

String ModuleCore::workDiagnosticHtml() const {
  if (!_workDiagCount) return "";
  String html = "<section class=\"status-card diag-card\"><div class=\"status-head\"><strong>Automatic reconnect diagnostics</strong><span class=\"pill\">RAM only</span></div><p class=\"diag-intro\">Work Mode continuously scans for the saved SSID. When found, the ESP32 attempts the saved credentials once; a failed or timed-out attempt returns to scanning. Registration Mode opens only from the physical setup request or an explicit setup action.</p><div class=\"diag-list\">";
  for (uint8_t i = 0; i < _workDiagCount; i++) {
    html += "<div class=\"diag-row\"><span>" + String((int)i + 1) + "</span><p>" + escapeHtml(_workDiag[i]) + "</p></div>";
  }
  html += "</div></section>";
  return html;
}



void ModuleCore::mqttMessageThunk(int messageSize) {
  if (_instance) _instance->onMqttMessage(messageSize);
}

void ModuleCore::onMqttMessage(int messageSize) {
  if (!_mqtt || messageSize <= 0) return;
  String raw;
  raw.reserve((size_t)messageSize + 1);
  while (_mqtt->available()) raw += (char)_mqtt->read();

  JsonDocument doc;
  if (deserializeJson(doc, raw) != DeserializationError::Ok) return;
  String correlation = doc["correlation_id"] | "";
  if (_waitingCorrelationId.length() && correlation == _waitingCorrelationId) {
    _waitingResponsePayload = raw;
    _waitingResponseReady = true;
    return;
  }

  String type = doc["type"] | "";
  type.toLowerCase();
  String command = doc["command"] | "";
  command.toLowerCase();
  if (type == "command" && command.length()) type = command;
  if (type == "terminate-session") type = "terminate";
  if (type == "ping-status" || type == "refresh" || type == "status") type = "ping";

  if ((type == "ping" || type == "terminate" || type == "rfid-scan") && !_pendingCommand.pending) {
    String commandId = doc["command_id"] | correlation;
    String scanId = doc["scan_id"] | "";
    int64_t targetSessionId = doc["session_id"] | 0LL;
    String fingerprint = type + "|" + commandId + "|" + scanId + "|" + String((long long)targetSessionId);
    if (!commandId.length() && !scanId.length()) fingerprint += "|" + String(checksum32(raw), HEX);

    const bool exactRepeat = fingerprint.length() && fingerprint == _lastCommandFingerprint;
    const unsigned long repeatWindow = (commandId.length() || scanId.length()) ? 600000UL : 10000UL;
    if (exactRepeat && millis() - _lastCommandAcceptedMs < repeatWindow) return;

    _lastCommandFingerprint = fingerprint;
    _lastCommandAcceptedMs = millis();
    _pendingCommand.pending = true;
    _pendingCommand.type = type;
    _pendingCommand.commandId = commandId;
    _pendingCommand.targetSessionId = targetSessionId;
    _pendingCommand.reason = doc["reason"] | "";
    _pendingCommand.scanId = scanId;
    _pendingCommand.timeoutMs = doc["timeout_ms"] | 0UL;
  }
}

void ModuleCore::addCommonEventFields(JsonDocument& doc, const char* type, const String& correlationId) {
  doc["type"] = type;
  doc["token"] = _config.moduleToken; // required by current MQTT bridge
  if (correlationId.length()) doc["correlation_id"] = correlationId;
}

bool ModuleCore::publishDocument(JsonDocument& doc) {
  if (!mqttConnected() || !_config.eventTopic.length()) return false;
  String payload;
  serializeJson(doc, payload);
  if (!_mqtt->beginMessage(_config.eventTopic, payload.length(), false, 1)) return false;
  _mqtt->print(payload);
  return _mqtt->endMessage() == 1;
}

bool ModuleCore::runtimeRequest(JsonDocument& request, String& responsePayload, const char* correlationPrefix) {
  responsePayload = "";
  String correlation = request["correlation_id"] | "";
  if (!correlation.length()) {
    correlation = makeCorrelationId(correlationPrefix);
    request["correlation_id"] = correlation;
  }
  request["token"] = _config.moduleToken;

  if (mqttConnected()) {
    _waitingCorrelationId = correlation;
    _waitingResponsePayload = "";
    _waitingResponseReady = false;
    if (publishDocument(request)) {
      unsigned long started = millis();
      while (!_waitingResponseReady && millis() - started < REQUEST_TIMEOUT_MS) {
        if (_mqtt) _mqtt->poll();
        serviceBuzzer();
        delay(4);
      }
      if (_waitingResponseReady) {
        responsePayload = _waitingResponsePayload;
        _waitingCorrelationId = "";
        _waitingResponsePayload = "";
        _waitingResponseReady = false;
        applyRuntimeMetadata(responsePayload);
        JsonDocument readyDoc;
        if (responseOk(responsePayload, readyDoc)) _serverRuntimeReady = true;
        return true;
      }
    }
    _waitingCorrelationId = "";
    _waitingResponsePayload = "";
    _waitingResponseReady = false;
  }

  // Event fallback only; this is not polling.
  bool delivered = httpRuntimeRequest(request, responsePayload);
  if (delivered) {
    applyRuntimeMetadata(responsePayload);
    JsonDocument readyDoc;
    if (responseOk(responsePayload, readyDoc)) _serverRuntimeReady = true;
  }
  return delivered;
}

void ModuleCore::applyRuntimeMetadata(const String& responsePayload) {
  if (!responsePayload.length()) return;
  JsonDocument doc;
  if (deserializeJson(doc, responsePayload) != DeserializationError::Ok) return;
  JsonObject module = doc["module"].as<JsonObject>();
  if (module.isNull()) return;
  String status = module["status"] | "";
  String name = module["name"] | "";
  String location = module["location"] | "";
  bool changed = false;
  if (status.length() && status != _config.moduleStatus) { _config.moduleStatus = status; changed = true; }
  if (name.length() && name != _config.moduleName) { _config.moduleName = name; changed = true; }
  if (location != _config.moduleLocation) { _config.moduleLocation = location; changed = true; }
  if (changed) saveConfig();
}

void ModuleCore::setHealth(bool rfidOk, bool pzemApplicable, bool pzemOk) {
  _healthRfidApplicable = true;
  _healthRfidOk = rfidOk;
  _healthPzemApplicable = pzemApplicable;
  _healthPzemOk = pzemOk;
}

void ModuleCore::sendHeartbeat() {
  if (!_config.paired || WiFi.status() != WL_CONNECTED) return;
  JsonDocument doc;
  addCommonEventFields(doc, "heartbeat", makeCorrelationId("hb"));
  if (_healthRfidApplicable) doc["rfid_ok"] = _healthRfidOk;
  if (_healthPzemApplicable) doc["pzem_ok"] = _healthPzemOk;
  JsonObject sensors = doc["sensors"].to<JsonObject>();
  sensors["firmware"] = _settings.firmwareVersion;
  sensors["wifi_rssi"] = WiFi.RSSI();
  sensors["uptime_ms"] = millis();
  sensors["mode"] = "work";
  String response;
  if (runtimeRequest(doc, response, "hb")) {
    JsonDocument check;
    if (deserializeJson(check, response) == DeserializationError::Ok) {
      if (check["ok"] | false) {
        _config.lastProbeOk = true;
        _config.lastProbeMessage = "Server runtime connected.";
        if (_workDiagCount) clearConnectionDiagnostics();
      } else {
        String code = check["code"] | "SERVER_REJECTED";
        String message = check["message"] | "Server rejected the saved module identity.";
        _config.lastProbeOk = false;
        _config.lastProbeMessage = code + ": " + message;
        recordConnectionDiagnostic("Runtime heartbeat rejected by server: " + _config.lastProbeMessage);
      }
    }
  } else {
    _config.lastProbeOk = false;
    _config.lastProbeMessage = "Server unreachable; retrying runtime heartbeat.";
  }
}

void ModuleCore::sendWarningEvent(const String& warning) {
  if (!_config.paired || WiFi.status() != WL_CONNECTED) return;
  JsonDocument doc;
  addCommonEventFields(doc, "warning", makeCorrelationId("warn"));
  doc["warning"] = warning;
  if (mqttConnected() && publishDocument(doc)) return;
  String ignored;
  httpRuntimeRequest(doc, ignored);
}

bool ModuleCore::takeCommand(PendingCommand& out) {
  if (!_pendingCommand.pending) return false;
  out = _pendingCommand;
  _pendingCommand = PendingCommand();
  return true;
}

// -----------------------------------------------------------------------------
// Work-mode networking
// -----------------------------------------------------------------------------

void ModuleCore::beginSavedWiFiJoin(const String& diagnosticLabel) {
  if (!_config.ssid.length()) return;

  disconnectMqtt();
  setRfidActive(false);
  _serverRuntimeReady = false;

  // Match the pre-oral radio setup: one STA owner, sleep disabled, normal ESP32
  // reconnect support enabled. Our state machine still decides when a fresh
  // scan/join attempt starts.
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  WiFi.setAutoReconnect(true);
  WiFi.disconnect(false, false);

  _lastWifiDisconnectReason = 0;
  _lastWifiStatusCode = WL_IDLE_STATUS;
  _lastWifiConnectStrategy = "scan saved SSID, then legacy WiFi.begin";
  _lastWifiTargetFound = false;
  _lastWifiTargetRSSI = 0;
  _lastWifiTargetChannel = 0;
  _lastWifiTargetAuth = 0;
  _lastWifiTargetBSSID = "";
  _wifiAttemptInProgress = false;
  _wifiAttemptStartedMs = 0;
  _wifiScanInProgress = false;
  _wifiTryingDefaultHotspotPassword = false;
  _wifiNextActionMs = millis() + WIFI_RADIO_SETTLE_MS;
  if (!_workReconnectStartedMs) _workReconnectStartedMs = millis();

  if (diagnosticLabel.length()) recordConnectionDiagnostic(diagnosticLabel);
}

void ModuleCore::serviceWiFi() {
  const unsigned long now = millis();

  if (_mode != Mode::Work) return;

  if (WiFi.status() == WL_CONNECTED) {
    if (_wifiScanInProgress) WiFi.scanDelete();
    _wifiScanInProgress = false;
    _wifiAttemptInProgress = false;
    _wifiAttemptStartedMs = 0;
    _wifiNextActionMs = 0;
    _lastWifiStatusCode = WL_CONNECTED;
    if (_wifiTryingDefaultHotspotPassword && !_config.wifiPassword.length()) {
      _config.wifiPassword = DEFAULT_AP_PASSWORD;
      _wifiTryingDefaultHotspotPassword = false;
      saveConfig();
      recordConnectionDiagnostic("Compatibility recovery succeeded; current ElectriCredit Server hotspot password was saved.");
    }
    return;
  }

  _serverRuntimeReady = false;
  if (!_config.ssid.length()) return;
  if (!_workReconnectStartedMs) _workReconnectStartedMs = now;

  // A join attempt gets a full window. We deliberately use the same simple
  // WiFi.begin(ssid,password) call that was reliable in the pre-oral firmware.
  if (_wifiAttemptInProgress) {
    _lastWifiStatusCode = (int)WiFi.status();
    if (now - _wifiAttemptStartedMs < WIFI_CONNECT_WINDOW_MS) return;

    String detail = "Saved Wi-Fi attempt " + String((int)_workAttemptNumber) +
                    " timed out; status=" + String(_lastWifiStatusCode);
    if (_lastWifiDisconnectReason) {
      detail += ", reason=" + String((int)_lastWifiDisconnectReason) +
                " (" + wifiReasonMessage(_lastWifiDisconnectReason) + ")";
    }
    detail += ". Returning to scan.";
    recordConnectionDiagnostic(detail);
    WiFi.disconnect(false, false);
    _wifiAttemptInProgress = false;
    _wifiAttemptStartedMs = 0;
    _wifiTryingDefaultHotspotPassword = false;
    _wifiNextActionMs = now + WIFI_SCAN_RETRY_MS;
    return;
  }

  // The scan itself is asynchronous, but every Wi-Fi API call is still made
  // by this Arduino task. That keeps GPIO33, OLED and AP/STA transitions stable.
  if (_wifiScanInProgress) {
    int found = WiFi.scanComplete();
    if (found == WIFI_SCAN_RUNNING) return;

    _wifiScanInProgress = false;
    _lastWifiTargetFound = false;
    _lastWifiTargetRSSI = 0;
    _lastWifiTargetChannel = 0;
    _lastWifiTargetAuth = 0;
    _lastWifiTargetBSSID = "";

    int best = -1;
    int32_t bestRSSI = -1000;
    if (found > 0) {
      for (int i = 0; i < found; ++i) {
        if (WiFi.SSID(i) != _config.ssid) continue;
        int32_t rssi = WiFi.RSSI(i);
        if (best < 0 || rssi > bestRSSI) { best = i; bestRSSI = rssi; }
      }
    }

    if (best >= 0) {
      _lastWifiTargetFound = true;
      _lastWifiTargetRSSI = WiFi.RSSI(best);
      _lastWifiTargetChannel = WiFi.channel(best);
      _lastWifiTargetAuth = (uint8_t)WiFi.encryptionType(best);
      _lastWifiTargetBSSID = WiFi.BSSIDstr(best);
    }
    WiFi.scanDelete();

    if (!_lastWifiTargetFound) {
      _lastWifiStatusCode = WL_NO_SSID_AVAIL;
      showScreen("System", "Saved Wi-Fi not found", oledText(_config.ssid), "Hold 3 sec: setup");
      recordConnectionDiagnostic("Saved SSID not visible: " + _config.ssid + ". Rescanning.");
      _wifiNextActionMs = now + WIFI_SCAN_RETRY_MS;
      return;
    }

    String joinPassword = _config.wifiPassword;
    _wifiTryingDefaultHotspotPassword = false;
    if (_lastWifiTargetAuth != 0 && !joinPassword.length()) {
      // Pre-oral ElectriCredit Server was normally open; the current RPi build
      // ships WPA2 with 1234567890. Recover that exact legacy->current upgrade
      // automatically, but do not guess passwords for arbitrary SSIDs.
      if (_config.ssid == "ElectriCredit Server") {
        joinPassword = DEFAULT_AP_PASSWORD;
        _wifiTryingDefaultHotspotPassword = true;
        recordConnectionDiagnostic("Saved ElectriCredit Server credentials came from an open-hotspot build; trying the current default WPA2 password once.");
      } else {
        _lastWifiStatusCode = WL_CONNECT_FAILED;
        showScreen("Wi-Fi Password", "Saved network is secured", oledText(_config.ssid), "Hold 3 sec: setup");
        recordConnectionDiagnostic("Saved SSID is secured but this module has no saved Wi-Fi password. Enter Registration Mode and save the network password.");
        _wifiNextActionMs = now + WIFI_AUTH_RETRY_MS;
        return;
      }
    }

    _lastWifiDisconnectReason = 0;
    _workAttemptNumber++;
    _lastWifiAttemptMs = now;
    _wifiAttemptStartedMs = now;
    _wifiAttemptInProgress = true;
    _lastWifiConnectStrategy = "legacy WiFi.begin(ssid,password) after visible-SSID scan";

    showScreen("System", "Connecting saved Wi-Fi", oledText(_config.ssid), "Hold 3 sec: setup");
    WiFi.disconnect(false, false);
    delay(20);
    WiFi.begin(_config.ssid.c_str(), joinPassword.length() ? joinPassword.c_str() : nullptr);
    recordConnectionDiagnostic("Saved SSID found (RSSI " + String(_lastWifiTargetRSSI) +
                               " dBm, channel " + String(_lastWifiTargetChannel) +
                               "); legacy connection attempt " + String((int)_workAttemptNumber) + " started.");
    return;
  }

  if (_wifiNextActionMs && (long)(now - _wifiNextActionMs) < 0) return;

  showScreen("System", "Scanning saved Wi-Fi", oledText(_config.ssid), "Hold 3 sec: setup");
  int scanResult = WiFi.scanNetworks(true, true);
  if (scanResult == WIFI_SCAN_RUNNING || scanResult >= 0) {
    _wifiScanInProgress = true;
    _wifiNextActionMs = 0;
    return;
  }

  WiFi.scanDelete();
  _wifiScanInProgress = false;
  _lastWifiStatusCode = (int)WiFi.status();
  _wifiNextActionMs = now + WIFI_SCAN_RETRY_MS;
}

void ModuleCore::serviceWorkMode() {
  // Single-owner networking: the Arduino loop alone scans/joins/switches modes.
  serviceWiFi();
  if (WiFi.status() != WL_CONNECTED) return;

  _wifiAttemptInProgress = false;
  _wifiAttemptStartedMs = 0;
  _lastWifiDisconnectReason = 0;

  const String currentSsid = WiFi.SSID();
  const String currentIp = WiFi.localIP().toString();
  if (_config.lastWifi != currentSsid || _config.lastIp != currentIp) {
    _config.lastWifi = currentSsid;
    _config.lastIp = currentIp;
    saveConfig();
    recordConnectionDiagnostic("Wi-Fi connected: " + currentSsid + " IP=" + currentIp +
                               " gateway=" + WiFi.gatewayIP().toString());
    showScreen("Wi-Fi Connected", oledText(currentSsid), "IP: " + currentIp, "Checking server...");
  }

  // Reproduce the reliable pre-oral order: establish the HTTP server path
  // before attempting MQTT. MQTT is an optimization/live transport, not a
  // prerequisite for declaring the RPi server reachable.
  const unsigned long heartbeatInterval = _serverRuntimeReady ? HEARTBEAT_MS : SERVER_READY_RETRY_MS;
  if (!_lastHeartbeatMs || millis() - _lastHeartbeatMs >= heartbeatInterval) {
    _lastHeartbeatMs = millis();
    sendHeartbeat();
  }

  if (!_serverRuntimeReady) {
    if (!overlayActive()) {
      String server = _config.serverUrl.length() ? _config.serverUrl : serverFromGateway();
      String probe = _config.lastProbeMessage;
      probe.trim();
      if (!probe.length() || _config.lastProbeOk) probe = "Checking Runtime API";
      showScreen("Server Check", oledText(probe), oledText(server), "Hold 3 sec: setup");
    }
    return;
  }

  if (_config.mqttEnabled) {
    if (!mqttConnected() && (!_lastMqttAttemptMs || millis() - _lastMqttAttemptMs >= MQTT_RETRY_MS)) {
      _lastMqttAttemptMs = millis();
      connectMqtt();
    }
    if (_mqtt && _mqtt->connected()) _mqtt->poll();
  }
}

uint32_t ModuleCore::checksum32(const String& value) const {
  uint32_t hash = 2166136261UL;
  for (size_t i = 0; i < value.length(); i++) {
    hash ^= (uint8_t)value[i];
    hash *= 16777619UL;
  }
  return hash;
}

bool ModuleCore::fieldAt(const String& input, int index, String& out) const {
  int start = 0;
  int current = 0;
  for (int i = 0; i <= (int)input.length(); i++) {
    if (i == (int)input.length() || input[i] == '|') {
      if (current == index) {
        out = input.substring(start, i);
        return true;
      }
      current++;
      start = i + 1;
    }
  }
  return false;
}

void ModuleCore::saveAlternatingRecord(const char* prefsNs, uint32_t sequence, const String& record) {
  Preferences p;
  p.begin(prefsNs, false);
  p.putString((sequence % 2U == 0U) ? "slot_a" : "slot_b", record);
  p.end();
}

void ModuleCore::loadAlternatingRecords(const char* prefsNs, String& slotA, String& slotB) const {
  Preferences p;
  p.begin(prefsNs, true);
  slotA = p.getString("slot_a", "");
  slotB = p.getString("slot_b", "");
  p.end();
}

void ModuleCore::clearStoredNamespace(const char* prefsNs) {
  Preferences p;
  p.begin(prefsNs, false);
  p.clear();
  p.end();
}

void ModuleCore::saveUidLock(const char* prefsNs, bool locked, const String& uid) {
  Preferences p;
  p.begin(prefsNs, false);
  if (!locked || !uid.length()) {
    p.clear();
  } else {
    p.putBool("locked", true);
    p.putString("uid", uid);
  }
  p.end();
}

bool ModuleCore::loadUidLock(const char* prefsNs, String& uid) const {
  Preferences p;
  p.begin(prefsNs, true);
  bool locked = p.getBool("locked", false);
  uid = p.getString("uid", "");
  p.end();
  return locked && uid.length() > 0;
}

} // namespace ElectriCredit


#include <Arduino.h>
#include <PZEM004Tv30.h>
#include <math.h>

using namespace ElectriCredit;

// -----------------------------------------------------------------------------
// Hub identity / frozen unique pins
// -----------------------------------------------------------------------------

static const char* FIRMWARE_VERSION = "EC-HUB-STANDALONE-3.3.0";
static const uint8_t PZEM_RX_PIN = 16;
static const uint8_t PZEM_TX_PIN = 17;
static const uint8_t PZEM_ADDRESS = 0x01;
static const uint8_t RELAY_PIN = 32;
static const bool RELAY_ACTIVE_HIGH = true;

static const CommonSettings COMMON = {
  "hub",
  FIRMWARE_VERSION,
  "ec-hub-v2",
  "ElectriCredit",
  "ESP32 38-pin / Goouuu V4"
};

ModuleCore ec;
PZEM004Tv30 pzem(Serial2, PZEM_RX_PIN, PZEM_TX_PIN, PZEM_ADDRESS);

// -----------------------------------------------------------------------------
// Hub policy
// -----------------------------------------------------------------------------

static const int64_t MWH_PER_KWH = 1000000LL;
static const int64_t MWH_PER_WH = 1000LL;
static const int64_t LOW_POSITIVE_THRESHOLD_UNITS = 1LL * MONEY_SCALE;
static const int64_t NEAR_CUTOFF_GAP_UNITS = 1LL * MONEY_SCALE;

static const unsigned long PZEM_INTERVAL_MS = 1000;
static const unsigned long PZEM_FRESH_MS = 2500;
static const uint8_t PZEM_FAILURE_LIMIT = 3;
static const float MIN_VALID_MAINS_VOLTAGE = 80.0f;
static const float MAX_VALID_MAINS_VOLTAGE = 280.0f;
static const float MIN_VALID_FREQUENCY = 40.0f;
static const float MAX_VALID_FREQUENCY = 70.0f;
static const float MAX_VALID_CURRENT = 80.0f;
static const float MAX_VALID_POWER = 20000.0f;

static const unsigned long RFID_CHECK_INTERVAL_MS = 150;
static const unsigned long RFID_REMOVED_AFTER_MS = 250;
static const unsigned long RFID_HEALTH_INTERVAL_MS = 5000;
static const uint8_t RFID_FAILURE_LIMIT = 3;

static const unsigned long NETWORK_ACTIVE_GRACE_MS = 30000;
static const unsigned long CACHE_CHECKPOINT_MS = 60000;
static const unsigned long CACHE_FORCE_CHECKPOINT_MS = 300000;
static const int64_t CACHE_MIN_ENERGY_ADVANCE_MWH = 100; // 0.1 Wh
static const unsigned long NEAR_CUTOFF_ALARM_INTERVAL_MS = 60000;
static const unsigned long RECOVERY_CARD_GRACE_MS = 2500;
static const unsigned long RECOVERY_RETRY_MS = 3000;
static const unsigned long DISPLAY_INTERVAL_MS = 1000;
static const unsigned long ACTIVE_DISPLAY_PHASE_MS = 5000;

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

enum HubState : uint8_t {
  HUB_READY,
  HUB_ACTIVE,
  HUB_RECOVERING,
  HUB_TERMINATED_WAIT_CARD_CHANGE,
  HUB_HARDWARE_FAULT,
  HUB_ADMIN_CARD_SCAN,
  HUB_REGISTRATION
};

struct PzemSnapshot {
  bool valid = false;
  float voltage = 0;
  float current = 0;
  float power = 0;
  float energyKwh = 0;
  float frequency = 0;
  float powerFactor = 0;
  int64_t meterMWh = 0;
  unsigned long readAtMs = 0;
};

struct HubSession {
  bool valid = false;
  int64_t sessionId = 0;
  int64_t cardId = 0;
  String uid;
  String userName;
  int64_t baseBalanceUnits = 0;
  int64_t debtLimitUnits = 0;
  int64_t rateUnits = 0;
  int64_t energyMWh = 0;
  int64_t lastMeterMWh = 0;
  bool lastMeterValid = false;
  bool finalizePending = false;
  bool terminated = false;
  uint32_t sequence = 0;
};

HubState hubState = HUB_READY;
PzemSnapshot lastPzem;
HubSession session;

bool relayOn = false;
uint8_t pzemFailureCount = 0;
uint8_t rfidFailureCount = 0;
bool pzemFaultLatched = false;
bool rfidFaultLatched = false;

String terminatedUid;
bool terminatedLock = false;
unsigned long terminatedNoCardSinceMs = 0;

String deniedUid;
bool deniedUntilRemoval = false;
unsigned long deniedNoCardSinceMs = 0;

unsigned long lastPzemMs = 0;
unsigned long lastRfidCheckMs = 0;
unsigned long lastRfidSeenMs = 0;
unsigned long lastRfidHealthMs = 0;
unsigned long networkLostSinceMs = 0;
unsigned long recoveryStartedMs = 0;
unsigned long lastRecoveryAttemptMs = 0;
unsigned long lastDisplayMs = 0;
unsigned long activeSessionStartedMs = 0;

unsigned long lastCacheWriteMs = 0;
unsigned long lastCacheForceWriteMs = 0;
int64_t lastCachedEnergyMWh = 0;

bool lowPositiveAlarmedThisSession = false;
unsigned long lastNearCutoffAlarmMs = 0;

String adminScanId;
unsigned long adminScanStartedMs = 0;
unsigned long adminScanTimeoutMs = 90000;
unsigned long adminScanLastReadMs = 0;

// -----------------------------------------------------------------------------
// Relay / money
// -----------------------------------------------------------------------------

void setRelay(bool on) {
  relayOn = on;
  bool level = RELAY_ACTIVE_HIGH ? on : !on;
  digitalWrite(RELAY_PIN, level ? HIGH : LOW);
}

int64_t sessionCostUnits() {
  if (!session.valid || session.energyMWh <= 0 || session.rateUnits <= 0) return 0;
  return (session.energyMWh * session.rateUnits) / MWH_PER_KWH;
}

int64_t estimatedBalanceUnits() {
  return session.baseBalanceUnits - sessionCostUnits();
}

String compactFixed5(int64_t units) {
  bool neg = units < 0;
  uint64_t value = neg ? (uint64_t)(-units) : (uint64_t)units;
  uint64_t whole = value / MONEY_SCALE;
  uint32_t fraction = (uint32_t)(value % MONEY_SCALE);
  char frac[6];
  snprintf(frac, sizeof(frac), "%05lu", (unsigned long)fraction);
  String digits = frac;
  while (digits.length() > 1 && digits.endsWith("0")) digits.remove(digits.length() - 1);
  String out = neg ? "-" : "";
  out += String((unsigned long)whole);
  out += ".";
  out += digits;
  return out;
}

String compactEnergyKwh(int64_t energyMWh) {
  if (energyMWh < 0) energyMWh = 0;
  uint64_t whole = (uint64_t)energyMWh / MWH_PER_KWH;
  uint32_t fraction = (uint32_t)((uint64_t)energyMWh % MWH_PER_KWH / 10ULL); // 5 decimals
  char frac[6];
  snprintf(frac, sizeof(frac), "%05lu", (unsigned long)fraction);
  String digits = frac;
  while (digits.length() > 1 && digits.endsWith("0")) digits.remove(digits.length() - 1);
  return String((unsigned long)whole) + "." + digits;
}

String formatRunTime(unsigned long elapsedMs) {
  unsigned long total = elapsedMs / 1000UL;
  unsigned long h = total / 3600UL;
  unsigned long m = (total % 3600UL) / 60UL;
  unsigned long sec = total % 60UL;
  char out[16];
  snprintf(out, sizeof(out), "%02lu:%02lu:%02lu", h, m, sec);
  return String(out);
}

bool hubStatusAllowsTransactions() {
  String status = ec.config().moduleStatus;
  status.trim();
  status.toLowerCase();
  return status == "active";
}

String hubIdentityTitle() {
  String name = ec.config().moduleName.length() ? ec.config().moduleName : "Hub";
  return name + " | Hub";
}

// -----------------------------------------------------------------------------
// Brownout journal - two alternating records with FNV-1a checksum
// -----------------------------------------------------------------------------

String sessionRecordCore(const HubSession& s) {
  String r;
  r.reserve(230);
  r += "2|";
  r += String(s.sequence); r += "|";
  r += String((long long)s.sessionId); r += "|";
  r += s.uid; r += "|";
  r += String((long long)s.baseBalanceUnits); r += "|";
  r += String((long long)s.debtLimitUnits); r += "|";
  r += String((long long)s.rateUnits); r += "|";
  r += String((long long)s.energyMWh); r += "|";
  r += String((long long)s.lastMeterMWh); r += "|";
  r += s.lastMeterValid ? "1|" : "0|";
  r += s.finalizePending ? "1|" : "0|";
  r += s.terminated ? "1" : "0";
  return r;
}

String buildSessionRecord(const HubSession& s) {
  String core = sessionRecordCore(s);
  return core + "|" + String(ec.checksum32(core), HEX);
}

bool parseSessionRecord(const String& record, HubSession& out) {
  if (!record.length()) return false;
  int lastSep = record.lastIndexOf('|');
  if (lastSep < 0) return false;
  String core = record.substring(0, lastSep);
  uint32_t expected = strtoul(record.substring(lastSep + 1).c_str(), nullptr, 16);
  if (ec.checksum32(core) != expected) return false;

  String f[12];
  for (int i = 0; i < 12; i++) if (!ec.fieldAt(core, i, f[i])) return false;
  if (f[0] != "2") return false;

  HubSession parsed;
  parsed.valid = true;
  parsed.sequence = strtoul(f[1].c_str(), nullptr, 10);
  parsed.sessionId = strtoll(f[2].c_str(), nullptr, 10);
  parsed.uid = f[3];
  parsed.baseBalanceUnits = strtoll(f[4].c_str(), nullptr, 10);
  parsed.debtLimitUnits = strtoll(f[5].c_str(), nullptr, 10);
  parsed.rateUnits = strtoll(f[6].c_str(), nullptr, 10);
  parsed.energyMWh = strtoll(f[7].c_str(), nullptr, 10);
  parsed.lastMeterMWh = strtoll(f[8].c_str(), nullptr, 10);
  parsed.lastMeterValid = f[9] == "1";
  parsed.finalizePending = f[10] == "1";
  parsed.terminated = f[11] == "1";
  if (parsed.sessionId < 1 || !parsed.uid.length()) return false;
  out = parsed;
  return true;
}

void saveSessionCache(const String& reason) {
  if (!session.valid) return;
  session.sequence++;
  ec.saveAlternatingRecord("ec-hub-run2", session.sequence, buildSessionRecord(session));
  lastCacheWriteMs = millis();
  lastCacheForceWriteMs = millis();
  lastCachedEnergyMWh = session.energyMWh;
  Serial.printf("[HUB][CACHE] %s seq=%lu energy_mWh=%lld\n", reason.c_str(), (unsigned long)session.sequence, (long long)session.energyMWh);
}

bool loadSessionCache() {
  String a, b;
  ec.loadAlternatingRecords("ec-hub-run2", a, b);
  HubSession sa, sb;
  bool va = parseSessionRecord(a, sa);
  bool vb = parseSessionRecord(b, sb);
  if (!va && !vb) return false;
  session = (!vb || (va && sa.sequence >= sb.sequence)) ? sa : sb;
  lastCachedEnergyMWh = session.energyMWh;
  lastCacheWriteMs = millis();
  lastCacheForceWriteMs = millis();
  return true;
}

void clearSessionCache() {
  ec.clearStoredNamespace("ec-hub-run2");
  session = HubSession();
  lastCachedEnergyMWh = 0;
}

void checkpointSessionIfNeeded() {
  if (!session.valid || hubState != HUB_ACTIVE) return;
  bool enoughTime = millis() - lastCacheWriteMs >= CACHE_CHECKPOINT_MS;
  bool enoughEnergy = llabs(session.energyMWh - lastCachedEnergyMWh) >= CACHE_MIN_ENERGY_ADVANCE_MWH;
  bool force = millis() - lastCacheForceWriteMs >= CACHE_FORCE_CHECKPOINT_MS;
  if ((enoughTime && enoughEnergy) || force) saveSessionCache(force ? "forced_checkpoint" : "checkpoint");
}

void saveTerminatedLock() {
  ec.saveUidLock("ec-hub-lock2", terminatedLock, terminatedUid);
}

void clearTerminatedLock() {
  terminatedLock = false;
  terminatedUid = "";
  terminatedNoCardSinceMs = 0;
  ec.saveUidLock("ec-hub-lock2", false, "");
}

void loadTerminatedLock() {
  terminatedLock = ec.loadUidLock("ec-hub-lock2", terminatedUid);
}

void lockUidUntilChanged(const String& uid) {
  terminatedUid = uid;
  terminatedLock = uid.length() > 0;
  terminatedNoCardSinceMs = 0;
  if (terminatedLock) saveTerminatedLock();
}

// -----------------------------------------------------------------------------
// PZEM
// -----------------------------------------------------------------------------

bool validatePzemValues(float voltage, float current, float power, float energy, float frequency, float pf) {
  if (isnan(voltage) || isnan(current) || isnan(power) || isnan(energy) || isnan(frequency) || isnan(pf)) return false;
  if (!isfinite(voltage) || !isfinite(current) || !isfinite(power) || !isfinite(energy) || !isfinite(frequency) || !isfinite(pf)) return false;
  if (voltage < MIN_VALID_MAINS_VOLTAGE || voltage > MAX_VALID_MAINS_VOLTAGE) return false;
  if (frequency < MIN_VALID_FREQUENCY || frequency > MAX_VALID_FREQUENCY) return false;
  if (current < 0 || current > MAX_VALID_CURRENT) return false;
  if (power < 0 || power > MAX_VALID_POWER) return false;
  if (energy < 0 || pf < 0 || pf > 1.20f) return false;
  return true;
}

bool readPzemNow(bool updateSessionEnergy) {
  float v = pzem.voltage();
  float c = pzem.current();
  float w = pzem.power();
  float e = pzem.energy();
  float hz = pzem.frequency();
  float pf = pzem.pf();
  bool valid = validatePzemValues(v, c, w, e, hz, pf);

  if (!valid) {
    if (pzemFailureCount < 255) pzemFailureCount++;
    lastPzem.valid = false;
    ec.setHealth(ec.rfidHealthy(), true, false);
    return false;
  }

  int64_t meterMWh = (int64_t)llround((double)e * (double)MWH_PER_KWH);
  lastPzem.valid = true;
  lastPzem.voltage = v;
  lastPzem.current = c;
  lastPzem.power = w;
  lastPzem.energyKwh = e;
  lastPzem.frequency = hz;
  lastPzem.powerFactor = pf;
  lastPzem.meterMWh = meterMWh;
  lastPzem.readAtMs = millis();
  pzemFailureCount = 0;
  pzemFaultLatched = false;
  ec.setHealth(ec.rfidHealthy(), true, true);

  if (updateSessionEnergy && session.valid) {
    if (session.lastMeterValid && meterMWh >= session.lastMeterMWh) {
      session.energyMWh += meterMWh - session.lastMeterMWh;
    }
    // If the PZEM cumulative counter was externally reset or replaced, never
    // create negative/free energy. Re-baseline from the new verified value.
    session.lastMeterMWh = meterMWh;
    session.lastMeterValid = true;
  }
  return true;
}

bool pzemFreshAndValid() {
  return lastPzem.valid && millis() - lastPzem.readAtMs <= PZEM_FRESH_MS;
}

bool ensureFreshPzem() {
  if (pzemFreshAndValid()) return true;
  if (millis() - lastPzemMs < PZEM_INTERVAL_MS) return false;
  lastPzemMs = millis();
  return readPzemNow(false);
}

void servicePzem() {
  if (millis() - lastPzemMs < PZEM_INTERVAL_MS) return;
  lastPzemMs = millis();
  bool ok = readPzemNow(hubState == HUB_ACTIVE || hubState == HUB_RECOVERING);
  if (!ok && hubState == HUB_ACTIVE && pzemFailureCount >= PZEM_FAILURE_LIMIT) {
    pzemFaultLatched = true;
  }
}

// -----------------------------------------------------------------------------
// Server session messages
// -----------------------------------------------------------------------------

void showAuthorizationFailure(JsonDocument& doc) {
  String code = doc["code"] | "";
  String message = doc["message"] | "";
  String reason = doc["result"]["reason"] | "";
  if (!reason.length()) reason = doc["result"]["card"]["reason"] | "";
  String lower = message + " " + reason;
  lower.toLowerCase();

  if (code == "CARD_NOT_REGISTERED" || lower.indexOf("not registered") >= 0) {
    ec.showTemporary("Card Not Registered", "Contact Administrator", "Create/register", "this user/card", 5000);
    ec.startBuzzerPattern(150, 120, 2);
  } else if (code == "PZEM_UNAVAILABLE") {
    ec.showScreen("Power Sensor Error", "Power disabled", "Contact Developer", "");
    ec.startBuzzerPattern(180, 120, 3);
  } else if (lower.indexOf("limit") >= 0) {
    ec.showTemporary("Credit Limit Reached", "Power disabled", "Please top up", "then re-insert", 5000);
    ec.startBuzzerPattern(140, 100, 5);
  } else if (code == "UNAUTHORIZED") {
    ec.showTemporary("Module Authentication", "Credentials rejected", "Contact Developer", "Use Registration Mode", 5000);
    ec.startBuzzerPattern(180, 120, 3);
  } else if (code == "RFID_UNAVAILABLE") {
    ec.showTemporary("RFID Reader Error", "Power disabled", "Contact Developer", "", 5000);
    ec.startBuzzerPattern(180, 120, 3);
  } else if (lower.indexOf("active") >= 0 || lower.indexOf("disabled") >= 0) {
    ec.showTemporary("Account Not Available", "Contact Administrator", "Check user/card", "status", 5000);
    ec.startBuzzerPattern(150, 120, 2);
  } else {
    ec.showTemporary("Server Request Error", message.length() ? message.substring(0, 20) : "Unable to authorize", "Contact Developer", "", 5000);
    ec.startBuzzerPattern(180, 120, 3);
  }
}

bool parseSessionStartResponse(const String& raw, int64_t& returnedSessionId, int64_t& returnedCardId, int64_t& balanceUnits, int64_t& debtUnits, int64_t& rateUnits, String& userName) {
  JsonDocument doc;
  if (!ec.responseOk(raw, doc)) {
    showAuthorizationFailure(doc);
    return false;
  }
  if (!(doc["power_allowed"] | false)) {
    showAuthorizationFailure(doc);
    return false;
  }
  returnedSessionId = doc["session_id"] | 0LL;
  returnedCardId = doc["result"]["card"]["id"] | 0LL;
  balanceUnits = doc["result"]["card"]["balance_units"] | 0LL;
  debtUnits = doc["result"]["card"]["debit_limit_units"] | 0LL;
  if (doc["result"]["card"]["debit_limit_units"].isNull()) debtUnits = doc["result"]["card"]["debt_limit_units"] | 0LL;
  rateUnits = doc["result"]["rate"]["tenant_rate_units"] | 0LL;
  userName = doc["result"]["tenant"]["name"] | "Tenant";
  return returnedSessionId > 0 && rateUnits >= 0;
}

bool responseIsUserDenial(const String& raw) {
  JsonDocument doc;
  if (deserializeJson(doc, raw) != DeserializationError::Ok) return false;
  String code = doc["code"] | "";
  if (code == "CARD_NOT_REGISTERED") return true;
  if (!doc["result"].isNull()) {
    bool valid = doc["result"]["valid"] | true;
    bool allowed = doc["result"]["allowed"] | true;
    String reason = doc["result"]["reason"] | "";
    reason.toLowerCase();
    return !valid || (!allowed && (reason.indexOf("active") >= 0 || reason.indexOf("limit") >= 0));
  }
  return false;
}

void latchDeniedCard(const String& uid) {
  deniedUid = uid;
  deniedUntilRemoval = uid.length() > 0;
  deniedNoCardSinceMs = 0;
}

void clearDeniedCard() {
  deniedUid = "";
  deniedUntilRemoval = false;
  deniedNoCardSinceMs = 0;
}

bool requestSessionStart(const String& uid, String& responsePayload) {
  if (!ensureFreshPzem()) return false;
  if (!ec.rfidHealthy()) return false;
  JsonDocument req;
  ec.addCommonEventFields(req, "hub-session-start");
  req["uid"] = uid;
  req["pzem_ok"] = true;
  req["rfid_ok"] = true;
  JsonObject sensors = req["sensors"].to<JsonObject>();
  sensors["voltage"] = lastPzem.voltage;
  sensors["frequency"] = lastPzem.frequency;
  sensors["firmware"] = FIRMWARE_VERSION;
  return ec.runtimeRequest(req, responsePayload, "start");
}

bool sendFinalSession(const String& reason, bool useLastValidMeasurement) {
  if (!session.valid || session.sessionId < 1) return true;
  JsonDocument req;
  ec.addCommonEventFields(req, "hub-session-stop");
  req["session_id"] = session.sessionId;
  req["uid"] = session.uid;
  req["energy_wh_total"] = session.energyMWh / MWH_PER_WH;
  req["pzem_ok"] = useLastValidMeasurement && session.lastMeterValid;
  req["rfid_ok"] = ec.rfidHealthy();
  JsonObject sensors = req["sensors"].to<JsonObject>();
  sensors["reason"] = reason;
  sensors["energy_mwh"] = session.energyMWh;
  sensors["estimated_balance_units"] = estimatedBalanceUnits();
  sensors["measurement_source"] = pzemFreshAndValid() ? "live" : "cached_last_valid";

  String response;
  if (!ec.runtimeRequest(req, response, "stop")) return false;
  JsonDocument doc;
  if (ec.responseOk(response, doc)) return true;
  // Current server may already have completed the session before an MQTT ACK
  // was lost. A retry then returns SESSION_NOT_FOUND; treat only this known
  // finalization retry as settled.
  String code = doc["code"] | "";
  return code == "SESSION_NOT_FOUND";
}

// -----------------------------------------------------------------------------
// Session lifecycle
// -----------------------------------------------------------------------------

void beginFreshSession(const String& uid) {
  if (!ec.config().paired) {
    ec.showTemporary("Module Not Registered", "Hold button 3 sec", "Open Registration", "Contact Developer", 5000);
    return;
  }
  if (!ec.workNetworkReady()) {
    ec.showTemporary("Network Unavailable", "Power remains off", "Reconnecting", "", 3500);
    return;
  }
  if (!ensureFreshPzem()) {
    ec.showScreen("Power Sensor Error", "No valid PZEM reading", "Power disabled", "Contact Developer");
    ec.startBuzzerPattern(180, 120, 3);
    return;
  }

  String response;
  ec.showScreen("Checking Card", uid, "Contacting server", "Power remains off");
  if (!requestSessionStart(uid, response)) {
    ec.showTemporary("Server Unavailable", "Power remains off", "Reconnecting", "Try again", 2500);
    return;
  }

  JsonDocument scanResponse;
  if (deserializeJson(scanResponse, response) == DeserializationError::Ok && (scanResponse["admin_scan"] | false)) {
    setRelay(false);
    latchDeniedCard(uid);
    ec.showTemporary("RFID Captured", "Sent to Administrator", "Power remains OFF", "Remove the card", 2200);
    ec.startBuzzerPattern(90, 70, 2);
    return;
  }

  int64_t sid = 0, cardId = 0, balance = 0, debt = 0, rate = 0;
  String userName;
  if (!parseSessionStartResponse(response, sid, cardId, balance, debt, rate, userName)) {
    if (responseIsUserDenial(response)) latchDeniedCard(uid);
    return;
  }
  clearDeniedCard();

  if (balance <= debt) {
    ec.showTemporary("Credit Limit Reached", "Power disabled", "Please top up", "then re-insert", 5000);
    ec.startBuzzerPattern(140, 100, 5);
    lockUidUntilChanged(uid);
    hubState = HUB_TERMINATED_WAIT_CARD_CHANGE;
    return;
  }

  // Final live interlock after the server round trip. A stale pre-request PZEM
  // value can never energize GPIO32.
  lastPzemMs = millis();
  if (!readPzemNow(false)) {
    session = HubSession();
    session.valid = true;
    session.sessionId = sid;
    session.uid = uid;
    session.baseBalanceUnits = balance;
    session.debtLimitUnits = debt;
    session.rateUnits = rate;
    session.lastMeterValid = false;
    session.finalizePending = true;
    saveSessionCache("pzem_failed_before_power");
    sendFinalSession("pzem_failed_before_power", false);
    clearSessionCache();
    ec.showTemporary("Power Sensor Error", "No live PZEM reading", "Power remains off", "Contact Developer", 5000);
    ec.startBuzzerPattern(180, 120, 3);
    return;
  }

  session = HubSession();
  session.valid = true;
  session.sessionId = sid;
  session.cardId = cardId;
  session.uid = uid;
  session.userName = userName;
  session.baseBalanceUnits = balance;
  session.debtLimitUnits = debt;
  session.rateUnits = rate;
  session.lastMeterMWh = lastPzem.meterMWh;
  session.lastMeterValid = true;
  saveSessionCache("before_relay_on"); // durable BEFORE power

  lowPositiveAlarmedThisSession = false;
  lastNearCutoffAlarmMs = 0;
  if (balance < 0) {
    ec.startBuzzerPattern(140, 130, 3);
    if (balance - debt <= NEAR_CUTOFF_GAP_UNITS) lastNearCutoffAlarmMs = millis();
  } else if (balance <= LOW_POSITIVE_THRESHOLD_UNITS) {
    lowPositiveAlarmedThisSession = true;
    ec.startBuzzerPattern(250, 250, 10); // ~5 sec once in this session
  } else {
    ec.startBuzzerPattern(120, 100, 1);
  }

  setRelay(true);
  hubState = HUB_ACTIVE;
  activeSessionStartedMs = millis();
  lastRfidSeenMs = millis();
  networkLostSinceMs = 0;
  String title = userName + " (" + String((long long)cardId) + ")";
  ec.showScreen(title, compactFixed5(balance) + " | " + compactFixed5(balance), "kWh: 0.0", "Starting electricity");
}

bool finalizeCurrentSession(const String& reason, bool lockCardAfter, bool adminTerminated = false) {
  if (!session.valid) return true;
  String endingUid = session.uid;
  setRelay(false); // first, never after network work
  readPzemNow(true);
  session.finalizePending = true;
  session.terminated = adminTerminated;
  saveSessionCache("finalize_pending");

  if (sendFinalSession(reason, true)) {
    clearSessionCache();
    if (lockCardAfter) lockUidUntilChanged(endingUid);
    hubState = lockCardAfter ? HUB_TERMINATED_WAIT_CARD_CHANGE : HUB_READY;
    return true;
  }

  hubState = HUB_RECOVERING;
  recoveryStartedMs = millis();
  ec.showScreen("Final Sync Pending", "Power remains off", "Usage is saved", "Reconnecting server");
  return false;
}

void serviceLowBalanceWarnings() {
  if (!session.valid) return;
  int64_t balance = estimatedBalanceUnits();
  if (balance <= session.debtLimitUnits) return;

  // Before negative: one ~5-second warning once per session.
  if (!lowPositiveAlarmedThisSession && balance >= 0 && balance <= LOW_POSITIVE_THRESHOLD_UNITS) {
    lowPositiveAlarmedThisSession = true;
    ec.startBuzzerPattern(250, 250, 10);
  }

  // One peso above the configured debt limit: ~5 seconds, max once/minute.
  if (balance < 0 && balance - session.debtLimitUnits <= NEAR_CUTOFF_GAP_UNITS) {
    if (!lastNearCutoffAlarmMs || millis() - lastNearCutoffAlarmMs >= NEAR_CUTOFF_ALARM_INTERVAL_MS) {
      lastNearCutoffAlarmMs = millis();
      ec.startBuzzerPattern(250, 250, 10);
    }
  }
}

void serviceActiveSession() {
  if (!session.valid) {
    setRelay(false);
    hubState = HUB_READY;
    return;
  }

  if (pzemFaultLatched || pzemFailureCount >= PZEM_FAILURE_LIMIT) {
    ec.showScreen("Power Sensor Error", "Power disabled", "Contact Developer", "Usage is protected");
    ec.startBuzzerPattern(180, 120, 3);
    finalizeCurrentSession("pzem_fault", false, false);
    pzemFaultLatched = true;
    return;
  }
  if (rfidFaultLatched || rfidFailureCount >= RFID_FAILURE_LIMIT) {
    ec.showScreen("RFID Reader Error", "Power disabled", "Contact Developer", "Usage is protected");
    ec.startBuzzerPattern(180, 120, 3);
    finalizeCurrentSession("rfid_fault", false, false);
    rfidFaultLatched = true;
    return;
  }

  // Wi-Fi/server requests remain usable through the HTTP event fallback when
  // MQTT is unavailable (notably during Windows development).
  bool networkGood = ec.workNetworkReady();
  if (!networkGood) {
    if (!networkLostSinceMs) networkLostSinceMs = millis();
    if (millis() - networkLostSinceMs >= NETWORK_ACTIVE_GRACE_MS) {
      ec.showTemporary("Connection Lost", "Power disabled", "Session is saved", "Reconnecting", 5000);
      finalizeCurrentSession("network_timeout", false, false);
      return;
    }
  } else {
    networkLostSinceMs = 0;
  }

  if (millis() - lastRfidCheckMs >= RFID_CHECK_INTERVAL_MS) {
    lastRfidCheckMs = millis();
    String seenUid;
    bool hasCard = ec.readPresentUid(seenUid);
    if (hasCard) {
      lastRfidSeenMs = millis();
      if (seenUid != session.uid) {
        String newUid = seenUid;
        if (finalizeCurrentSession("card_changed", false, false)) beginFreshSession(newUid);
        return;
      }
    } else if (millis() - lastRfidSeenMs >= RFID_REMOVED_AFTER_MS) {
      finalizeCurrentSession("card_removed", false, false);
      return;
    }
  }

  int64_t balance = estimatedBalanceUnits();
  if (balance <= session.debtLimitUnits) {
    ec.showTemporary("Credit Limit Reached", "Power disabled", "Please top up", "then re-insert", 5000);
    ec.startBuzzerPattern(140, 100, 5);
    finalizeCurrentSession("credit_limit", true, false);
    return;
  }

  serviceLowBalanceWarnings();
  checkpointSessionIfNeeded();

  if (!ec.overlayActive() && millis() - lastDisplayMs >= DISPLAY_INTERVAL_MS) {
    lastDisplayMs = millis();
    bool secondPhase = ((millis() / ACTIVE_DISPLAY_PHASE_MS) % 2UL) != 0;
    String title = session.userName.length() ? session.userName : "Tenant";
    title += " (" + String((long long)session.cardId) + ")";
    String line1 = compactFixed5(session.baseBalanceUnits) + " | " + compactFixed5(balance);
    String line2 = secondPhase
        ? ("Used: " + compactFixed5(sessionCostUnits()))
        : ("kWh: " + compactEnergyKwh(session.energyMWh));
    String line3;
    if (secondPhase) {
      line3 = "Run: " + formatRunTime(activeSessionStartedMs ? millis() - activeSessionStartedMs : 0);
    } else if (lastPzem.valid) {
      line3 = "V" + String(lastPzem.voltage, 0) + " | A" + String(lastPzem.current, 3);
    } else {
      line3 = "Power sensor check";
    }
    ec.showScreen(title, line1, line2, line3);
  }
}

void recoverCachedSession() {
  setRelay(false);
  if (!session.valid) {
    hubState = terminatedLock ? HUB_TERMINATED_WAIT_CARD_CHANGE : HUB_READY;
    return;
  }
  if (!ec.workNetworkReady()) {
    ec.showScreen("Session Recovery", "Power remains off", "Waiting for server", "Usage is saved");
    return;
  }
  if (millis() - lastRfidCheckMs < RFID_CHECK_INTERVAL_MS) return;
  lastRfidCheckMs = millis();
  String seenUid;
  bool hasCard = ec.readPresentUid(seenUid);

  if (!hasCard && !session.finalizePending && millis() - recoveryStartedMs < RECOVERY_CARD_GRACE_MS) {
    ec.showScreen("Session Recovery", "Checking RFID", "Power remains off", "Please wait");
    return;
  }

  if (millis() - lastRecoveryAttemptMs < RECOVERY_RETRY_MS) return;
  lastRecoveryAttemptMs = millis();

  if (session.finalizePending || session.terminated || !hasCard || seenUid != session.uid) {
    // Recover the last PZEM cumulative delta before syncing a brownout-ended
    // session. This closes the gap between the last journal checkpoint and
    // the actual power interruption whenever the PZEM counter is readable.
    if (session.lastMeterValid) {
      lastPzemMs = millis();
      if (readPzemNow(true)) saveSessionCache("brownout_meter_capture");
    }
    String oldUid = session.uid;
    bool shouldLock = session.terminated;
    if (sendFinalSession(session.terminated ? "admin_terminate_recovery" : "brownout_recovery", true)) {
      clearSessionCache();
      if (shouldLock) lockUidUntilChanged(oldUid);
      if (pzemFaultLatched || rfidFaultLatched) {
        hubState = HUB_HARDWARE_FAULT;
        return;
      }
      hubState = terminatedLock ? HUB_TERMINATED_WAIT_CARD_CHANGE : HUB_READY;
      if (hasCard && (!terminatedLock || seenUid != terminatedUid)) {
        if (terminatedLock && seenUid != terminatedUid) clearTerminatedLock();
        beginFreshSession(seenUid);
      }
    } else {
      ec.showScreen("Final Sync Pending", "Power remains off", "Usage is saved", "Reconnecting server");
    }
    return;
  }

  // Same RFID remains. Re-authorize/recover with server, then force another
  // real PZEM transaction immediately before restoring electricity.
  if (!ensureFreshPzem()) {
    ec.showScreen("Session Recovery", "Checking power sensor", "Power remains off", "Contact Developer if stuck");
    return;
  }

  String response;
  if (!requestSessionStart(seenUid, response)) return;
  int64_t sid = 0, cardId = 0, balance = 0, debt = 0, rate = 0;
  String userName;
  if (!parseSessionStartResponse(response, sid, cardId, balance, debt, rate, userName)) return;

  if (sid != session.sessionId) {
    // Server believes this card now belongs to another session. Settle the old
    // cached session first, then request a clean session.
    String uid = session.uid;
    session.finalizePending = true;
    saveSessionCache("server_session_changed");
    if (sendFinalSession("server_session_changed", true)) {
      clearSessionCache();
      beginFreshSession(uid);
    }
    return;
  }

  // Account may have been topped up while the ESP32 was restarting. Treat the
  // server's current balance/rate/limit as authoritative, while retaining the
  // locally protected energy amount from this same session.
  session.baseBalanceUnits = balance;
  session.debtLimitUnits = debt;
  session.rateUnits = rate;
  session.cardId = cardId;
  session.userName = userName;
  if (estimatedBalanceUnits() <= debt) {
    ec.startBuzzerPattern(140, 100, 5);
    finalizeCurrentSession("credit_limit_during_recovery", true, false);
    return;
  }

  lastPzemMs = millis();
  if (!readPzemNow(false)) {
    ec.showScreen("Power Sensor Error", "Recovery stopped", "Power disabled", "Contact Developer");
    return;
  }
  session.lastMeterMWh = lastPzem.meterMWh;
  session.lastMeterValid = true;
  saveSessionCache("before_recovery_relay_on");
  setRelay(true);
  hubState = HUB_ACTIVE;
  activeSessionStartedMs = millis();
  lastRfidSeenMs = millis();
  networkLostSinceMs = 0;
  ec.showScreen("Session Restored", userName.substring(0, 20), ec.money2(estimatedBalanceUnits()), "Electricity active");
}

void serviceTerminatedWait() {
  setRelay(false);
  if (!terminatedLock) {
    hubState = HUB_READY;
    return;
  }
  if (millis() - lastRfidCheckMs < RFID_CHECK_INTERVAL_MS) return;
  lastRfidCheckMs = millis();
  String uid;
  bool present = ec.readPresentUid(uid);

  if (!present) {
    if (!terminatedNoCardSinceMs) terminatedNoCardSinceMs = millis();
    if (millis() - terminatedNoCardSinceMs >= RFID_REMOVED_AFTER_MS) {
      clearTerminatedLock();
      hubState = HUB_READY;
      ec.showScreen("Ready", "Re-insert RFID", "for a new session", "");
    }
    return;
  }

  terminatedNoCardSinceMs = 0;
  if (uid != terminatedUid) {
    clearTerminatedLock();
    hubState = HUB_READY;
    beginFreshSession(uid);
    return;
  }
  if (!ec.overlayActive()) ec.showScreen("Session Terminated", "Please remove RFID", "then re-insert", "Power remains off");
}

void serviceReady() {
  setRelay(false);
  if (terminatedLock) {
    hubState = HUB_TERMINATED_WAIT_CARD_CHANGE;
    return;
  }

  // No transaction hardware is allowed to wake until this boot has both Wi-Fi
  // and a successful Runtime API response. This keeps startup quiet and avoids
  // card/PZEM work against a stale or unreachable server.
  if (!ec.workNetworkReady() || !ec.serverRuntimeReady()) {
    // The common connection core exclusively owns the OLED while Wi-Fi/server
    // recovery is in progress. Work Mode only keeps transaction hardware OFF.
    ec.setRfidActive(false);
    return;
  }
  if (!hubStatusAllowsTransactions()) {
    ec.setRfidActive(false);
    String status = ec.config().moduleStatus.length() ? ec.config().moduleStatus : "checking";
    if (!ec.overlayActive()) ec.showScreen(hubIdentityTitle(), ec.config().moduleLocation, "Status: " + status, "Power remains OFF");
    return;
  }

  if (!ec.rfidHealthy() && !ec.checkRfidHardware()) {
    rfidFaultLatched = true;
    hubState = HUB_HARDWARE_FAULT;
    return;
  }

  if (deniedUntilRemoval) {
    if (millis() - lastRfidCheckMs < RFID_CHECK_INTERVAL_MS) return;
    lastRfidCheckMs = millis();
    String seen;
    if (!ec.readPresentUid(seen)) {
      if (!deniedNoCardSinceMs) deniedNoCardSinceMs = millis();
      if (millis() - deniedNoCardSinceMs >= RFID_REMOVED_AFTER_MS) clearDeniedCard();
    } else {
      deniedNoCardSinceMs = 0;
      if (seen != deniedUid) {
        clearDeniedCard();
        beginFreshSession(seen);
      }
    }
    return;
  }

  if (!ec.overlayActive()) {
    String location = ec.config().moduleLocation.length() ? ec.config().moduleLocation : "Location not set";
    ec.showScreen(hubIdentityTitle(), location, "Insert your card", "Status: Active");
  }
  if (millis() - lastRfidCheckMs < RFID_CHECK_INTERVAL_MS) return;
  lastRfidCheckMs = millis();
  String uid;
  if (ec.readPresentUid(uid)) beginFreshSession(uid);
}

void serviceHardwareFault() {
  setRelay(false);
  bool rfidOk = ec.checkRfidHardware();
  bool pzemOk = pzemFreshAndValid();
  if (millis() - lastPzemMs >= PZEM_INTERVAL_MS) {
    lastPzemMs = millis();
    pzemOk = readPzemNow(false);
  }
  rfidFailureCount = rfidOk ? 0 : RFID_FAILURE_LIMIT;
  if (rfidOk) rfidFaultLatched = false;
  if (pzemOk) pzemFaultLatched = false;
  ec.setHealth(rfidOk, true, pzemOk);

  if (rfidOk && pzemOk && !session.valid) {
    hubState = terminatedLock ? HUB_TERMINATED_WAIT_CARD_CHANGE : HUB_READY;
    ec.showTemporary("Hardware Restored", "Sensors responding", "Power remains off", "Ready for RFID", 3500);
    ec.startBuzzerPattern(100, 80, 1);
  } else if (!ec.overlayActive()) {
    if (!pzemOk) ec.showScreen("Power Sensor Error", "Power disabled", "Contact Developer", "");
    else ec.showScreen("RFID Reader Error", "Power disabled", "Contact Developer", "");
  }
}

// -----------------------------------------------------------------------------
// Administrator RFID registration scan
// -----------------------------------------------------------------------------

void startAdminCardScan(const PendingCommand& cmd) {
  if (!cmd.scanId.length() || session.valid || hubState == HUB_ACTIVE || hubState == HUB_RECOVERING || hubState == HUB_TERMINATED_WAIT_CARD_CHANGE) {
    ec.showTemporary("Card Scan Unavailable", "Hub is busy", "Try again shortly", "", 3000);
    return;
  }
  if (!ec.rfidHealthy()) {
    ec.showTemporary("Card Scan Unavailable", "RFID reader error", "Contact Developer", "", 3500);
    return;
  }

  setRelay(false);
  adminScanId = cmd.scanId;
  adminScanStartedMs = millis();
  adminScanTimeoutMs = cmd.timeoutMs >= 5000 && cmd.timeoutMs <= 120000 ? cmd.timeoutMs : 90000;
  adminScanLastReadMs = 0;
  hubState = HUB_ADMIN_CARD_SCAN;
  ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "Place RFID card", "Power: OFF");
}

void finishAdminCardScan(bool success) {
  setRelay(false);
  adminScanId = "";
  adminScanStartedMs = 0;
  adminScanLastReadMs = 0;
  hubState = (rfidFaultLatched || pzemFaultLatched) ? HUB_HARDWARE_FAULT : HUB_READY;
  if (success) {
    ec.startBuzzerPattern(90, 70, 2);
    ec.showTemporary("RFID Captured", "Sent to Admin", "Power remained OFF", "Ready", 3000);
  }
}

void serviceAdminCardScan() {
  setRelay(false);
  if (!adminScanId.length()) {
    finishAdminCardScan(false);
    return;
  }
  if (millis() - adminScanStartedMs >= adminScanTimeoutMs) {
    ec.showTemporary("Card Scan Ended", "No RFID detected", "Try again in Admin", "", 3000);
    finishAdminCardScan(false);
    return;
  }
  if (!ec.rfidHealthy()) {
    ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "RFID reader error", "Power: OFF");
    return;
  }
  if (!ec.workNetworkReady()) {
    ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "Waiting for network", "Power: OFF");
    return;
  }
  if (!ec.overlayActive()) ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "Place RFID card", "Power: OFF");
  if (millis() - adminScanLastReadMs < RFID_CHECK_INTERVAL_MS) return;
  adminScanLastReadMs = millis();

  String uid;
  if (!ec.readPresentUid(uid)) return;

  JsonDocument req;
  ec.addCommonEventFields(req, "rfid-scan-result");
  req["scan_id"] = adminScanId;
  req["uid"] = uid;
  req["rfid_ok"] = true;
  String response;
  if (!ec.runtimeRequest(req, response, "rfid-scan")) {
    ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "RFID detected", "Sending to server...");
    return;
  }
  JsonDocument doc;
  if (!ec.responseOk(response, doc) || !(doc["accepted"] | false)) {
    ec.showScreen("ElectriCredit", "HUB | CARD SETUP", "Scan expired/rejected", "Retry from Admin");
    return;
  }
  finishAdminCardScan(true);
}

// -----------------------------------------------------------------------------
// PING / TERMINATE
// -----------------------------------------------------------------------------

void sendPingSnapshot(const PendingCommand& cmd) {
  JsonDocument doc;
  ec.addCommonEventFields(doc, "heartbeat", cmd.commandId.length() ? cmd.commandId : ec.makeCorrelationId("ping"));
  doc["rfid_ok"] = ec.rfidHealthy();
  doc["pzem_ok"] = lastPzem.valid;
  if (session.valid) doc["session_id"] = session.sessionId;
  JsonObject sensors = doc["sensors"].to<JsonObject>();
  sensors["command_status"] = "pinged";
  sensors["relay_on"] = relayOn;
  sensors["voltage"] = lastPzem.voltage;
  sensors["current"] = lastPzem.current;
  sensors["power"] = lastPzem.power;
  sensors["energy_mwh"] = session.valid ? session.energyMWh : 0;
  sensors["estimated_balance_units"] = session.valid ? estimatedBalanceUnits() : 0;
  sensors["firmware"] = FIRMWARE_VERSION;
  ec.publishDocument(doc);
}

void processCommands() {
  PendingCommand cmd;
  if (!ec.takeCommand(cmd)) return;

  if (cmd.type == "rfid-scan") {
    startAdminCardScan(cmd);
    return;
  }

  if (cmd.type == "ping") {
    ec.beepPing();
    ec.showTemporary("You Are Pinged", session.valid ? "Session continues" : "Hub online", relayOn ? "Electricity stays ON" : "Power remains OFF", "Status sent", 3000);
    sendPingSnapshot(cmd);
    return;
  }

  if (cmd.type == "terminate") {
    if (!session.valid) {
      ec.beepTerminate();
      ec.showTemporary("Terminate Received", "No active session", "Power remains off", "", 3000);
      sendPingSnapshot(cmd);
      return;
    }
    if (cmd.targetSessionId > 0 && cmd.targetSessionId != session.sessionId) {
      // Stale command for an older session: never terminate the new tenant.
      sendPingSnapshot(cmd);
      return;
    }
    ec.beepTerminate();
    ec.showScreen("Session Terminated", "Power disabled", "Please remove RFID", "then re-insert");
    finalizeCurrentSession("admin_terminate", true, true);
  }
}

// -----------------------------------------------------------------------------
// Common-mode transitions / health
// -----------------------------------------------------------------------------

void serviceRfidHealth() {
  // Keep the RC522 in reset while Wi-Fi is still recovering. Relay remains
  // OFF independently, so transaction hardware wakes only after Work Mode has
  // a network path.
  if (!ec.workNetworkReady()) {
    ec.setRfidActive(false);
    return;
  }
  if (millis() - lastRfidHealthMs < RFID_HEALTH_INTERVAL_MS) return;
  lastRfidHealthMs = millis();
  bool ok = ec.checkRfidHardware();
  if (ok) {
    rfidFailureCount = 0;
    rfidFaultLatched = false;
  } else {
    if (rfidFailureCount < 255) rfidFailureCount++;
    if (rfidFailureCount >= RFID_FAILURE_LIMIT) rfidFaultLatched = true;
  }
  bool pzemApplicable = hubState == HUB_ACTIVE || hubState == HUB_RECOVERING ||
                        hubState == HUB_HARDWARE_FAULT || lastPzem.readAtMs > 0;
  ec.setHealth(ok, pzemApplicable, pzemApplicable ? lastPzem.valid : true);
}

void handleRegistrationRequest() {
  if (!ec.takeRegistrationRequest()) return;
  setRelay(false);
  if (session.valid) {
    readPzemNow(true);
    session.finalizePending = true;
    saveSessionCache("enter_registration_mode");
    // Try once now; if unreachable, the cache remains and Work Mode recovery
    // will safely finish it later.
    if (sendFinalSession("registration_mode", true)) clearSessionCache();
  }
  hubState = HUB_REGISTRATION;
  ec.enterRegistrationMode();
}

// -----------------------------------------------------------------------------
// Arduino entry points
// -----------------------------------------------------------------------------

void setup() {
  // Safety before absolutely everything else.
  pinMode(RELAY_PIN, OUTPUT);
  setRelay(false);

  Serial.begin(115200);
  delay(60);
  const bool oledReadyAtBoot = ec.begin(COMMON);
  if (!oledReadyAtBoot) {
    // Shared keeps retrying the same proven GPIO21/GPIO22/0x3C OLED in the
    // background. Do not alter pins or continue with any display-specific
    // workaround here.
    Serial.println("[HUB][OLED] Initial SSD1306 startup failed; recovery retry is active.");
  }
  loadTerminatedLock();
  bool recovered = loadSessionCache();

  // Do not block startup on the PZEM before the shared button/mode loop is
  // available. The first normal servicePzem() pass performs the initial probe,
  // and relay safety still requires a fresh valid reading before power can turn on.
  lastPzem.valid = false;
  lastPzemMs = 0;

  if (ec.inRegistrationMode()) {
    hubState = HUB_REGISTRATION;
  } else if (recovered) {
    hubState = HUB_RECOVERING;
    recoveryStartedMs = millis();
    ec.showScreen("Session Recovery", "Power remains off", "Usage found in cache", "Connecting server");
  } else if (terminatedLock) {
    hubState = HUB_TERMINATED_WAIT_CARD_CHANGE;
  } else {
    hubState = HUB_READY;
  }
}

void loop() {
  ec.loop();
  handleRegistrationRequest();

  if (ec.inRegistrationMode()) {
    setRelay(false);
    hubState = HUB_REGISTRATION;
    delay(1);
    return;
  }

  // If the installer selected Work Mode in the portal, reconstruct the proper
  // safe state without rebooting.
  if (hubState == HUB_REGISTRATION) {
    if (session.valid) {
      hubState = HUB_RECOVERING;
      recoveryStartedMs = millis();
    } else if (terminatedLock) {
      hubState = HUB_TERMINATED_WAIT_CARD_CHANGE;
    } else {
      hubState = HUB_READY;
    }
  }

  // Before a normal/recovery transaction can touch RFID or PZEM, the current
  // boot must have a successful Runtime API heartbeat. An already-active
  // session keeps its existing short network-grace safety logic.
  if (hubState != HUB_ACTIVE && !ec.serverRuntimeReady()) {
    // Connection core owns all startup/reconnect OLED messages.
    setRelay(false);
    ec.setRfidActive(false);
    delay(1);
    return;
  }

  if (hubState != HUB_ACTIVE && !hubStatusAllowsTransactions()) {
    setRelay(false);
    ec.setRfidActive(false);
    String status = ec.config().moduleStatus.length() ? ec.config().moduleStatus : "checking";
    if (!ec.overlayActive()) ec.showScreen(hubIdentityTitle(), ec.config().moduleLocation, "Status: " + status, "Power remains OFF");
    delay(1);
    return;
  }

  // Keep transaction hardware quiet until the Runtime API validates the saved
  // module identity on this boot. Active/recovery states perform only the
  // measurements they explicitly need.
  if (ec.serverRuntimeReady()) {
    serviceRfidHealth();
    processCommands();
  } else if (hubState == HUB_READY || hubState == HUB_REGISTRATION) {
    ec.setRfidActive(false);
  }
  if (hubState == HUB_ACTIVE) servicePzem();

  if (pzemFaultLatched && hubState == HUB_READY) hubState = HUB_HARDWARE_FAULT;
  if (rfidFaultLatched && hubState == HUB_READY) hubState = HUB_HARDWARE_FAULT;

  switch (hubState) {
    case HUB_ADMIN_CARD_SCAN:
      serviceAdminCardScan();
      break;
    case HUB_ACTIVE:
      serviceActiveSession();
      break;
    case HUB_RECOVERING:
      recoverCachedSession();
      break;
    case HUB_TERMINATED_WAIT_CARD_CHANGE:
      serviceTerminatedWait();
      break;
    case HUB_HARDWARE_FAULT:
      serviceHardwareFault();
      break;
    case HUB_READY:
    default:
      serviceReady();
      break;
  }

  delay(1);
}
