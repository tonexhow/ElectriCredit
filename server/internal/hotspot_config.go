package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultHotspotSSID           = "ElectriCredit Server"
	defaultHotspotPassword       = "1234567890"
	defaultHotspotInterface      = "wlan0"
	defaultHotspotConnectionName = "ElectriCredit Hotspot"
	defaultHotspotIPv4Address    = "10.42.0.1/24"
)

type hotspotFileConfig struct {
	BootEnabled    bool   `json:"boot_enabled"`
	SSID           string `json:"ssid"`
	Password       string `json:"password"`
	Interface      string `json:"interface"`
	ConnectionName string `json:"connection_name"`
	IPv4Address    string `json:"ipv4_address"`
}

func defaultHotspotFileConfig() hotspotFileConfig {
	return hotspotFileConfig{
		BootEnabled:    true,
		SSID:           defaultHotspotSSID,
		Password:       defaultHotspotPassword,
		Interface:      defaultHotspotInterface,
		ConnectionName: defaultHotspotConnectionName,
		IPv4Address:    defaultHotspotIPv4Address,
	}
}

func hotspotConfigFilePath() string {
	if value := strings.TrimSpace(os.Getenv("ELECTRICREDIT_HOTSPOT_CONFIG")); value != "" {
		return value
	}
	candidates := []string{
		filepath.Join("..", "[BOOT] Scripts", "hotspot.json"),
		filepath.Join("[BOOT] Scripts", "hotspot.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func normalizeHotspotFileConfig(cfg hotspotFileConfig) hotspotFileConfig {
	cfg.SSID = strings.TrimSpace(cfg.SSID)
	cfg.Interface = strings.TrimSpace(cfg.Interface)
	cfg.ConnectionName = strings.TrimSpace(cfg.ConnectionName)
	cfg.IPv4Address = strings.TrimSpace(cfg.IPv4Address)
	if cfg.SSID == "" {
		cfg.SSID = defaultHotspotSSID
	}
	if cfg.Password == "" {
		cfg.Password = defaultHotspotPassword
	}
	if cfg.Interface == "" {
		cfg.Interface = defaultHotspotInterface
	}
	if cfg.ConnectionName == "" {
		cfg.ConnectionName = defaultHotspotConnectionName
	}
	if cfg.IPv4Address == "" {
		cfg.IPv4Address = defaultHotspotIPv4Address
	}
	return cfg
}

func loadHotspotFileConfig() (hotspotFileConfig, error) {
	cfg := defaultHotspotFileConfig()
	path := hotspotConfigFilePath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if saveErr := saveHotspotFileConfig(cfg); saveErr != nil {
			return cfg, saveErr
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultHotspotFileConfig(), err
	}
	return normalizeHotspotFileConfig(cfg), nil
}

func saveHotspotFileConfig(cfg hotspotFileConfig) error {
	cfg = normalizeHotspotFileConfig(cfg)
	path := hotspotConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}
