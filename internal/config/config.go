package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"mirroid/internal/adb"
	"mirroid/internal/model"
)

const appDirName = "Mirroid"

// ThemeMode selects the app theme variant.
type ThemeMode string

const (
	ThemeModeSystem ThemeMode = "system"
	ThemeModeDark   ThemeMode = "dark"
	ThemeModeLight  ThemeMode = "light"
)

type Config struct {
	dir     string
	AppConf AppConfig
}

// AppConfig holds top-level app settings.
type AppConfig struct {
	ScrcpyPath       string    `json:"scrcpy_path"`
	ADBPath          string    `json:"adb_path"`
	AutoCheckUpdates bool      `json:"auto_check_updates"`
	LastUpdateCheck  int64     `json:"last_update_check"`
	ThemeMode        ThemeMode `json:"theme_mode"`
}

// New creates a Config, ensuring the config directory exists.
func New() (*Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config: could not find user config dir: %w", err)
	}

	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("config: could not create config dir: %w", err)
	}

	c := &Config{
		dir: dir,
		AppConf: AppConfig{
			ScrcpyPath:       "scrcpy",
			ADBPath:          "adb",
			AutoCheckUpdates: true,
		},
	}

	// load existing config if present
	if data, err := os.ReadFile(c.configPath()); err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("could not read config file, using defaults", "error", err)
		}
	} else {
		if err := json.Unmarshal(data, &c.AppConf); err != nil {
			slog.Warn("config file is malformed, using defaults", "error", err)
		}
	}

	return c, nil
}

func (c *Config) configPath() string {
	return filepath.Join(c.dir, "config.json")
}

func (c *Config) presetsPath() string {
	return filepath.Join(c.dir, "presets.json")
}

func (c *Config) devicesPath() string {
	return filepath.Join(c.dir, "devices.json")
}

func (c *Config) devicePresetsPath() string {
	return filepath.Join(c.dir, "device_presets.json")
}

func (c *Config) SaveAppConfig() error {
	data, err := json.MarshalIndent(c.AppConf, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configPath(), data, 0o644)
}

// LoadPresets reads all saved presets from disk.
func (c *Config) LoadPresets() (map[string]model.ScrcpyOptions, error) {
	presets := make(map[string]model.ScrcpyOptions)
	data, err := os.ReadFile(c.presetsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return presets, nil
		}
		return nil, fmt.Errorf("config: could not read presets: %w", err)
	}

	if err := json.Unmarshal(data, &presets); err != nil {
		return nil, fmt.Errorf("config: presets file is corrupt: %w", err)
	}
	return presets, nil
}

// SavePresets writes all presets to disk.
func (c *Config) SavePresets(presets map[string]model.ScrcpyOptions) error {
	data, err := json.MarshalIndent(presets, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.presetsPath(), data, 0o644)
}

// LoadKnownDevices reads the persisted known devices list from disk.
func (c *Config) LoadKnownDevices() ([]adb.Device, error) {
	var devices []adb.Device
	data, err := os.ReadFile(c.devicesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return devices, nil
		}
		return nil, fmt.Errorf("config: could not read devices: %w", err)
	}
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil, fmt.Errorf("config: devices file is corrupt: %w", err)
	}
	return devices, nil
}

// LoadDevicePresets reads the device-to-preset mapping from disk.
func (c *Config) LoadDevicePresets() (map[string]string, error) {
	result := make(map[string]string)
	data, err := os.ReadFile(c.devicePresetsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("config: could not read device presets: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("config: device presets file is corrupt: %w", err)
	}
	return result, nil
}

// SaveDevicePresets writes the device-to-preset mapping to disk.
func (c *Config) SaveDevicePresets(dp map[string]string) error {
	data, err := json.MarshalIndent(dp, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.devicePresetsPath(), data, 0o644)
}

// SaveKnownDevices writes the known devices list to disk.
func (c *Config) SaveKnownDevices(devices []adb.Device) error {
	data, err := json.MarshalIndent(devices, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.devicesPath(), data, 0o644)
}
