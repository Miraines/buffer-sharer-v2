package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadConfig загружает конфигурацию из файла
func (a *App) loadConfig() {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return
	}

	var config struct {
		Settings Settings `json:"settings"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		a.log("warn", "Не удалось загрузить конфиг: "+err.Error())
		return
	}

	a.mu.Lock()
	a.settings = config.Settings
	a.mu.Unlock()

	a.log("info", "Конфигурация загружена")
}

// saveConfig сохраняет конфигурацию в файл
func (a *App) saveConfig() {
	a.mu.RLock()
	config := struct {
		Settings Settings `json:"settings"`
	}{
		Settings: a.settings,
	}
	a.mu.RUnlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		a.log("error", "Не удалось сериализовать конфиг: "+err.Error())
		return
	}

	dir := filepath.Dir(a.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		a.log("error", "Не удалось создать директорию конфига: "+err.Error())
		return
	}

	if err := os.WriteFile(a.configPath, data, 0644); err != nil {
		a.log("error", "Не удалось сохранить конфиг: "+err.Error())
		return
	}
}

// GetSettings returns current settings
func (a *App) GetSettings() Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

// GetHotkeys returns the current hotkey configuration for frontend
func (a *App) GetHotkeys() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return map[string]string{
		"toggle":       a.settings.HotkeyToggle,
		"paste":        a.settings.HotkeyPaste,
		"screenshot":   a.settings.HotkeyScreenshot,
		"invisibility": a.settings.HotkeyInvisibility,
	}
}

// SaveSettings saves settings
func (a *App) SaveSettings(settings Settings) {
	a.mu.Lock()
	oldSettings := a.settings
	a.settings = settings
	a.mu.Unlock()

	if a.screenshotService != nil {
		a.screenshotService.SetInterval(settings.ScreenshotInterval)
		a.screenshotService.SetQuality(settings.ScreenshotQuality)
	}

	if a.screenshotHistory != nil {
		a.screenshotHistory.SetHistoryMaxLen(settings.ScreenshotHistoryLimit)
	}

	if a.hotkeyManager != nil {
		hotkeysChanged := oldSettings.HotkeyToggle != settings.HotkeyToggle ||
			oldSettings.HotkeyPaste != settings.HotkeyPaste ||
			oldSettings.HotkeyScreenshot != settings.HotkeyScreenshot ||
			oldSettings.HotkeyInvisibility != settings.HotkeyInvisibility

		if hotkeysChanged {
			a.log("info", "Hotkeys changed, re-registering...")
			a.registerHotkeysFromSettings()
		}
	}

	a.saveConfig()

	a.log("info", "Настройки сохранены")
}

// GenerateRoomCode generates a random room code
func (a *App) GenerateRoomCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return strings.ToUpper(hex.EncodeToString([]byte{
			byte(time.Now().UnixNano() & 0xFF),
			byte((time.Now().UnixNano() >> 8) & 0xFF),
			byte((time.Now().UnixNano() >> 16) & 0xFF),
		}))
	}
	return strings.ToUpper(hex.EncodeToString(b))
}
