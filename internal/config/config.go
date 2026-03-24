package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"mirroid/internal/model"
)

const appDirName = "Mirroid"

// Config manages application configuration and presets.
type Config struct {
	dir     string
	AppConf AppConfig
}

// AppConfig holds top-level app settings.
type AppConfig struct {
	ScrcpyPath string `json:"scrcpy_path"`
	ADBPath    string `json:"adb_path"`
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
			ScrcpyPath: "scrcpy",
			ADBPath:    "adb",
		},
	}

	// load existing config if present
	if data, err := os.ReadFile(c.configPath()); err == nil {
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

// SaveAppConfig persists the current app config to disk.
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
