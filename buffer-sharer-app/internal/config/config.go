package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Mode           string           `json:"mode" mapstructure:"mode"`
	MiddlewareHost string           `json:"middleware_host" mapstructure:"middleware_host"`
	MiddlewarePort int              `json:"middleware_port" mapstructure:"middleware_port"`
	RoomCode       string           `json:"room_code" mapstructure:"room_code"` // For client mode
	Hotkeys        HotkeyConfig     `json:"hotkeys" mapstructure:"hotkeys"`
	Clipboard      ClipboardConfig  `json:"clipboard" mapstructure:"clipboard"`
	Screenshot     ScreenshotConfig `json:"screenshot" mapstructure:"screenshot"`
	Logging        LoggingConfig    `json:"logging" mapstructure:"logging"`
}

// HotkeyConfig holds hotkey configuration
type HotkeyConfig struct {
	ToggleInputMode string `json:"toggle_input_mode" mapstructure:"toggle_input_mode"`
	TakeScreenshot  string `json:"take_screenshot" mapstructure:"take_screenshot"`
	PasteFromBuffer string `json:"paste_from_buffer" mapstructure:"paste_from_buffer"`
}

// ClipboardConfig holds clipboard monitoring configuration
type ClipboardConfig struct {
	Enabled        bool `json:"enabled" mapstructure:"enabled"`
	SyncIntervalMs int  `json:"sync_interval_ms" mapstructure:"sync_interval_ms"`
}

// ScreenshotConfig holds screenshot capture configuration
type ScreenshotConfig struct {
	IntervalMs int `json:"interval_ms" mapstructure:"interval_ms"`
	Quality    int `json:"quality" mapstructure:"quality"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Enabled    bool `json:"enabled" mapstructure:"enabled"`
	MaxEntries int  `json:"max_entries" mapstructure:"max_entries"`
}

// Load loads configuration from file and environment
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in common locations
		v.SetConfigName("default")
		v.SetConfigType("json")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")

		// Add user config directory
		if homeDir, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(homeDir, ".buffer-sharer"))
		}
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// Config file not found is not an error, we use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Unmarshal into struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setDefaults sets default values in viper
func setDefaults(v *viper.Viper) {
	v.SetDefault("mode", DefaultMode)
	v.SetDefault("middleware_host", DefaultMiddlewareHost)
	v.SetDefault("middleware_port", DefaultMiddlewarePort)

	v.SetDefault("hotkeys.toggle_input_mode", DefaultToggleInputMode)
	v.SetDefault("hotkeys.take_screenshot", DefaultTakeScreenshot)
	v.SetDefault("hotkeys.paste_from_buffer", DefaultPasteFromBuffer)

	v.SetDefault("clipboard.enabled", DefaultClipboardEnabled)
	v.SetDefault("clipboard.sync_interval_ms", DefaultClipboardSyncInterval)

	v.SetDefault("screenshot.interval_ms", DefaultScreenshotInterval)
	v.SetDefault("screenshot.quality", DefaultScreenshotQuality)

	v.SetDefault("logging.enabled", DefaultLoggingEnabled)
	v.SetDefault("logging.max_entries", DefaultLoggingMaxEntries)
}

// Save saves the configuration to a file
func (c *Config) Save(configPath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// IsController returns true if running in controller mode
func (c *Config) IsController() bool {
	return c.Mode == "controller"
}

// IsClient returns true if running in client mode
func (c *Config) IsClient() bool {
	return c.Mode == "client"
}

// MiddlewareAddress returns the full middleware address
func (c *Config) MiddlewareAddress() string {
	return fmt.Sprintf("%s:%d", c.MiddlewareHost, c.MiddlewarePort)
}
