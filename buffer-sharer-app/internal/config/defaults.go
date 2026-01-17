package config

// Default configuration values
const (
	DefaultMode           = "client"
	DefaultMiddlewareHost = "localhost"
	DefaultMiddlewarePort = 8080

	DefaultToggleInputMode  = "Ctrl+Shift+J"
	DefaultTakeScreenshot   = "Ctrl+Shift+S"
	DefaultPasteFromBuffer  = "Ctrl+Shift+V"

	DefaultClipboardEnabled     = true
	DefaultClipboardSyncInterval = 1000

	DefaultScreenshotInterval = 4000
	DefaultScreenshotQuality  = 80

	DefaultLoggingEnabled    = true
	DefaultLoggingMaxEntries = 1000
)

// NewDefaultConfig returns a Config with default values
func NewDefaultConfig() *Config {
	return &Config{
		Mode:           DefaultMode,
		MiddlewareHost: DefaultMiddlewareHost,
		MiddlewarePort: DefaultMiddlewarePort,
		Hotkeys: HotkeyConfig{
			ToggleInputMode:  DefaultToggleInputMode,
			TakeScreenshot:   DefaultTakeScreenshot,
			PasteFromBuffer:  DefaultPasteFromBuffer,
		},
		Clipboard: ClipboardConfig{
			Enabled:        DefaultClipboardEnabled,
			SyncIntervalMs: DefaultClipboardSyncInterval,
		},
		Screenshot: ScreenshotConfig{
			IntervalMs: DefaultScreenshotInterval,
			Quality:    DefaultScreenshotQuality,
		},
		Logging: LoggingConfig{
			Enabled:    DefaultLoggingEnabled,
			MaxEntries: DefaultLoggingMaxEntries,
		},
	}
}
