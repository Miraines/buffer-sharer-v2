package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/hotkey"
	"buffer-sharer-app/internal/invisibility"
	"buffer-sharer-app/internal/keyboard"
	"buffer-sharer-app/internal/logging"
	"buffer-sharer-app/internal/network"
	"buffer-sharer-app/internal/permissions"
	"buffer-sharer-app/internal/screenshot"
)

// Platform-specific hotkey defaults
func getDefaultHotkeys() (toggle, screenshot, paste string) {
	// Use Ctrl+Shift on all platforms - rarely reserved by system or apps
	// On macOS, Ctrl is rarely used as primary modifier (Cmd is used instead)
	// so Ctrl+Shift combinations are almost always available
	return "Ctrl+Shift+J", "Ctrl+Shift+S", "Ctrl+Shift+V"
}

// Settings holds application settings
type Settings struct {
	MiddlewareHost     string `json:"middlewareHost"`
	MiddlewarePort     int    `json:"middlewarePort"`
	ScreenshotInterval int    `json:"screenshotInterval"`
	ScreenshotQuality  int    `json:"screenshotQuality"`
	ClipboardSync      bool   `json:"clipboardSync"`
	HotkeyToggle       string `json:"hotkeyToggle"`
	HotkeyScreenshot   string `json:"hotkeyScreenshot"`
	HotkeyPaste        string `json:"hotkeyPaste"`
	HotkeyInvisibility string `json:"hotkeyInvisibility"` // Hotkey for toggling invisibility mode
	// Новые настройки
	AutoConnect            bool   `json:"autoConnect"`
	LastRole               string `json:"lastRole"`
	LastRoomCode           string `json:"lastRoomCode"`
	SoundEnabled           bool   `json:"soundEnabled"`
	Theme                  string `json:"theme"`                  // "dark", "light", "system"
	ScreenshotSaveDir      string `json:"screenshotSaveDir"`      // Директория для сохранения скриншотов
	ScreenshotHistoryLimit int    `json:"screenshotHistoryLimit"` // Максимальное количество скриншотов в истории (0 = по умолчанию 50)
}

// Statistics holds session statistics
type Statistics struct {
	ScreenshotsSent     int       `json:"screenshotsSent"`
	ScreenshotsReceived int       `json:"screenshotsReceived"`
	TextsSent           int       `json:"textsSent"`
	TextsReceived       int       `json:"textsReceived"`
	BytesSent           int64     `json:"bytesSent"`
	BytesReceived       int64     `json:"bytesReceived"`
	ConnectedAt         time.Time `json:"connectedAt"`
	TotalConnectTime    int64     `json:"totalConnectTime"` // в секундах
}

// TextHistoryEntry represents a sent/received text
type TextHistoryEntry struct {
	Text      string    `json:"text"`
	Direction string    `json:"direction"` // "sent" или "received"
	Timestamp time.Time `json:"timestamp"`
}

// ConnectionStatus represents the current connection state
type ConnectionStatus struct {
	Connected bool   `json:"connected"`
	RoomCode  string `json:"roomCode"`
	Role      string `json:"role"`
	Error     string `json:"error,omitempty"`
}

// App struct
type App struct {
	ctx        context.Context
	cancelFunc context.CancelFunc // For stopping goroutines on shutdown
	mu         sync.RWMutex
	logger     *logging.Logger

	// Connection state
	client    *network.Client
	connected bool
	roomCode  string
	role      string

	// Services
	screenshotService   *screenshot.CaptureService
	keyboardHandler     *keyboard.Handler
	keyInterceptor      *keyboard.KeyInterceptor
	permissionsManager  *permissions.Manager
	hotkeyManager       *hotkey.Manager
	invisibilityManager *invisibility.Manager

	// Settings
	settings Settings

	// Новые поля для фич
	stats       Statistics
	textHistory []TextHistoryEntry
	configPath  string

	// Screenshot history for controller
	screenshotHistory *screenshot.CaptureService

	// Safe event channel for CGo callbacks
	eventChan       chan func()
	eventChanClosed atomic.Bool // tracks if eventChan is closed to prevent send-on-closed-channel panic

	// Permission polling control
	permissionPollingCancel context.CancelFunc
	permissionPollingMu     sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	// Определяем путь к конфигу
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".buffer-sharer", "config.json")

	// Get platform-specific hotkeys
	hotkeyToggle, hotkeyScreenshot, hotkeyPaste := getDefaultHotkeys()

	return &App{
		settings: Settings{
			MiddlewareHost:     "localhost",
			MiddlewarePort:     8080,
			ScreenshotInterval: 4000,
			ScreenshotQuality:  80,
			ClipboardSync:      true,
			HotkeyToggle:       hotkeyToggle,
			HotkeyScreenshot:   hotkeyScreenshot,
			HotkeyPaste:        hotkeyPaste,
			HotkeyInvisibility: "Ctrl+Shift+I", // Default invisibility hotkey
			AutoConnect:        false,
			LastRole:           "controller",
			SoundEnabled:       true,
			Theme:              "dark",
		},
		textHistory: make([]TextHistoryEntry, 0),
		configPath:  configPath,
	}
}

// sendEvent safely sends a function to the event channel.
// Returns true if the function was queued, false if the channel is closed or full.
func (a *App) sendEvent(fn func()) bool {
	// Check if channel is closed before attempting to send
	if a.eventChanClosed.Load() {
		return false
	}
	select {
	case a.eventChan <- fn:
		return true
	default:
		// Channel full, skip to avoid blocking
		return false
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	// Create cancellable context for goroutines
	ctx, cancel := context.WithCancel(ctx)
	a.ctx = ctx
	a.cancelFunc = cancel

	// Initialize event channel for safe event emission from CGo callbacks
	a.eventChan = make(chan func(), 100)

	// Start event processor goroutine (runs events on main goroutine context)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case fn, ok := <-a.eventChan:
				if !ok {
					return
				}
				fn()
			}
		}
	}()

	// Initialize logger with file logging enabled
	logger, err := logging.NewLogger(logging.Config{
		Enabled:    true,
		MaxEntries: 1000,
		Level:      logging.LevelDebug, // Log everything
		LogToFile:  true,
		Role:       "app", // Will be updated when role is selected
	})
	if err == nil {
		a.logger = logger
	}

	// Initialize permissions manager
	a.permissionsManager = permissions.NewManager()

	// Initialize invisibility manager
	a.invisibilityManager = invisibility.NewManager()

	// NOTE: We do NOT request permissions automatically on startup!
	// The user will see the permissions modal and can click buttons to:
	// 1. Open settings manually
	// 2. Request permissions when ready
	// This gives user control over when system dialogs appear.

	// Initialize keyboard handler
	a.keyboardHandler = keyboard.NewHandler(a.logger)

	// Initialize key interceptor for character-by-character typing
	a.keyInterceptor = keyboard.NewKeyInterceptor(a.logger)
	a.keyInterceptor.SetOnBufferEmpty(func() {
		// Use event channel for safe event emission from CGo callback
		a.sendEvent(func() {
			a.log("info", "Буфер исчерпан - режим ввода выключен")
			runtime.EventsEmit(a.ctx, "inputModeChanged", false)
			runtime.EventsEmit(a.ctx, "bufferExhausted", nil)
		})
	})

	// Set up toggle hotkey callback - this will be triggered directly from the event tap
	// ensuring the hotkey works even when interception mode is enabled
	a.keyInterceptor.SetOnToggle(func() {
		// Use event channel for safe event emission from CGo callback
		a.sendEvent(func() {
			a.log("info", "Toggle hotkey detected in event tap")
			a.ToggleInputMode()
		})
	})

	// Set up paste hotkey callback - types all remaining buffer at once
	a.keyInterceptor.SetOnPaste(func() {
		a.sendEvent(func() {
			a.log("info", "Paste hotkey detected in event tap")
			a.TypeBuffer()
		})
	})

	// NOTE: Key interceptor is NOT started here on purpose!
	// It will be started automatically when SetEnabled(true) is called.
	// This prevents keyboard blocking on startup when buffer is empty.

	// Initialize screenshot history service (for controller role)
	historyLimit := a.settings.ScreenshotHistoryLimit
	if historyLimit <= 0 {
		historyLimit = 50 // Default
	}
	a.screenshotHistory = screenshot.NewCaptureService(screenshot.Config{
		IntervalMs:    0, // Not used for history, only storage
		Quality:       80,
		HistoryMaxLen: historyLimit,
	}, a.logger)

	// Загружаем настройки из файла
	a.loadConfig()

	a.log("info", "Buffer Sharer запущен")

	// Delay hotkey manager and permission check to avoid keyboard blocking on startup
	// This gives the system time to stabilize before we create event taps
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			// Initialize hotkey manager after delay
			a.initHotkeyManager()
			a.log("info", "Hotkey manager initialized after startup delay")
		}
	}()

	// Проверяем разрешения и уведомляем фронтенд (with additional delay)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1000 * time.Millisecond):
			a.checkAndNotifyPermissions()
		}
	}()
}

// checkAndNotifyPermissions проверяет разрешения и отправляет событие на фронтенд
func (a *App) checkAndNotifyPermissions() {
	perms := a.permissionsManager.GetAllPermissions()

	// Логируем статус каждого разрешения
	for _, p := range perms {
		a.log("info", "Разрешение "+string(p.Type)+": "+string(p.Status))
	}

	// Проверяем есть ли недостающие разрешения
	missing := make([]permissions.PermissionInfo, 0)
	for _, p := range perms {
		if p.Required && p.Status != permissions.StatusGranted {
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		a.log("warn", "Недостающие разрешения: "+strconv.Itoa(len(missing)))
		// Отправляем событие на фронтенд
		runtime.EventsEmit(a.ctx, "permissionsRequired", map[string]interface{}{
			"permissions": perms,
			"missing":     missing,
			"platform":    a.permissionsManager.GetPlatform(),
		})
	} else {
		a.log("info", "Все разрешения получены")
	}
}

// initHotkeyManager initializes and starts the global hotkey manager
func (a *App) initHotkeyManager() {
	a.hotkeyManager = hotkey.NewManager(a.logger)

	// Register handlers for hotkey actions
	a.hotkeyManager.RegisterHandler(hotkey.ActionToggleInputMode, func() {
		a.log("debug", "Hotkey triggered: toggle input mode")
		// Use event channel for thread-safe execution
		if !a.sendEvent(func() {
			a.ToggleInputMode()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionPasteFromBuffer, func() {
		a.log("debug", "Hotkey triggered: paste from buffer")
		if !a.sendEvent(func() {
			a.TypeBuffer()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionTakeScreenshot, func() {
		a.log("debug", "Hotkey triggered: take screenshot")
		// Screenshot functionality can be added here if needed
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionToggleInvisibility, func() {
		a.log("debug", "Hotkey triggered: toggle invisibility")
		if !a.sendEvent(func() {
			a.ToggleInvisibility()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	// Register hotkeys from settings
	a.registerHotkeysFromSettings()

	// Start listening for hotkeys
	a.hotkeyManager.StartAsync()
	a.log("info", "Hotkey manager started")

	// Start key interceptor event tap (with interception disabled)
	// This allows toggle/paste hotkeys to work immediately, even before user enables input mode
	if a.keyInterceptor != nil {
		a.log("info", "Starting key interceptor event tap for hotkeys...")
		if a.keyInterceptor.Start() {
			a.log("info", "Key interceptor event tap started (interception disabled, hotkeys active)")
		} else {
			a.log("warn", "Failed to start key interceptor - Accessibility permission may be required")
		}
	}
}

// registerHotkeysFromSettings registers all hotkeys from current settings
func (a *App) registerHotkeysFromSettings() {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()

	// Register toggle input mode hotkey
	// NOTE: We ONLY register it on the key interceptor's event tap, NOT on the hotkey manager
	// This prevents double triggering (event tap + hotkey library both firing)
	if settings.HotkeyToggle != "" {
		// Set toggle hotkey on key interceptor for direct detection in event tap
		// This is the ONLY place where toggle hotkey is handled
		if a.keyInterceptor != nil {
			a.keyInterceptor.SetToggleHotkey(settings.HotkeyToggle)
			a.log("info", "Set toggle hotkey on key interceptor: %s", settings.HotkeyToggle)
		}
	}

	// Register paste from buffer hotkey
	// NOTE: We ONLY register it on the key interceptor's event tap, NOT on the hotkey manager
	// This prevents double triggering
	if settings.HotkeyPaste != "" {
		if a.keyInterceptor != nil {
			a.keyInterceptor.SetPasteHotkey(settings.HotkeyPaste)
			a.log("info", "Set paste hotkey on key interceptor: %s", settings.HotkeyPaste)
		}
	}

	// Register screenshot hotkey
	if settings.HotkeyScreenshot != "" {
		if err := a.hotkeyManager.Register(hotkey.ActionTakeScreenshot, settings.HotkeyScreenshot); err != nil {
			a.log("error", "Failed to register screenshot hotkey '%s': %v", settings.HotkeyScreenshot, err)
		} else {
			a.log("info", "Registered hotkey for screenshot: %s", settings.HotkeyScreenshot)
		}
	}

	// Register invisibility hotkey
	if settings.HotkeyInvisibility != "" {
		if err := a.hotkeyManager.Register(hotkey.ActionToggleInvisibility, settings.HotkeyInvisibility); err != nil {
			a.log("error", "Failed to register invisibility hotkey '%s': %v", settings.HotkeyInvisibility, err)
		} else {
			a.log("info", "Registered hotkey for invisibility: %s", settings.HotkeyInvisibility)
		}
	}
}

// loadConfig загружает конфигурацию из файла
func (a *App) loadConfig() {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		// Файл не существует - используем значения по умолчанию
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

	// Создаём директорию если не существует
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

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// Cancel our internal context to stop all goroutines
	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	// Stop hotkey manager
	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
		a.log("info", "Hotkey manager stopped")
	}

	// Stop services
	if a.screenshotService != nil {
		a.screenshotService.Stop()
	}

	// Stop key interceptor
	if a.keyInterceptor != nil {
		a.keyInterceptor.Stop()
	}

	// Close event channel - set closed flag first to prevent sends after close
	a.eventChanClosed.Store(true)
	if a.eventChan != nil {
		close(a.eventChan)
	}

	// Disconnect with timeout to prevent hanging
	done := make(chan struct{})
	go func() {
		a.Disconnect()
		close(done)
	}()

	select {
	case <-done:
		// Disconnected successfully
	case <-ctx.Done():
		// Context cancelled, force close
	}

	if a.logger != nil {
		a.logger.Close()
	}
}

// log sends a log message to the frontend
func (a *App) log(level, format string, args ...interface{}) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	if a.logger != nil {
		switch level {
		case "error":
			a.logger.Error("app", message)
		case "warn":
			a.logger.Warn("app", message)
		case "info":
			a.logger.Info("app", message)
		default:
			a.logger.Debug("app", message)
		}
	}
	runtime.EventsEmit(a.ctx, "log", map[string]string{
		"level":   level,
		"message": message,
	})
}

// Connect connects to the middleware server
func (a *App) Connect(host string, port int, role, roomCode string) ConnectionStatus {
	a.mu.Lock()
	if a.connected {
		status := ConnectionStatus{
			Connected: true,
			RoomCode:  a.roomCode,
			Role:      a.role,
		}
		a.mu.Unlock()
		return status
	}
	a.mu.Unlock()

	a.log("info", "Подключение к "+host+"...")

	// Create network client
	client := network.NewClient(network.ClientConfig{
		Host:     host,
		Port:     port,
		Role:     role,
		RoomCode: roomCode,
	}, a.logger)

	// Set up callbacks
	client.SetOnMessage(func(msg *network.Message) {
		a.handleMessage(msg)
	})

	client.SetOnConnect(func() {
		a.log("info", "Соединение установлено")
		runtime.EventsEmit(a.ctx, "connected", nil)
	})

	client.SetOnDisconnect(func(err error) {
		if err != nil {
			a.log("warn", "Соединение потеряно: "+err.Error())
		} else {
			a.log("info", "Соединение закрыто")
		}
		runtime.EventsEmit(a.ctx, "disconnected", nil)
	})

	client.SetOnRoomCreated(func(code string) {
		a.log("info", "Комната создана: "+code)
		runtime.EventsEmit(a.ctx, "roomCreated", code)
	})

	client.SetOnRoomJoined(func(code string) {
		a.log("info", "Подключено к комнате: "+code)
		runtime.EventsEmit(a.ctx, "roomJoined", code)
	})

	client.SetOnAuthError(func(errMsg string) {
		a.log("error", "Ошибка аутентификации: "+errMsg)
		runtime.EventsEmit(a.ctx, "authError", errMsg)
	})

	// Connect
	if err := client.Connect(); err != nil {
		a.log("error", "Не удалось подключиться: "+err.Error())
		return ConnectionStatus{
			Connected: false,
			Error:     err.Error(),
		}
	}

	// Get room code from client after successful connection
	connectedRoomCode := client.GetRoomCode()

	// Update state
	a.mu.Lock()
	a.client = client
	a.connected = true
	a.role = role
	a.roomCode = connectedRoomCode
	// Сбрасываем статистику и сохраняем время подключения
	a.stats = Statistics{ConnectedAt: time.Now()}
	// Сохраняем последние настройки подключения
	a.settings.LastRole = role
	a.settings.LastRoomCode = connectedRoomCode
	a.mu.Unlock()

	// Update logger role for log file naming
	if a.logger != nil {
		a.logger.SetRole(role)
	}

	// Start client services
	client.Start()

	// If client role, start screenshot service
	if role == "client" {
		a.startScreenshotService()
	}

	// Сохраняем конфиг
	a.saveConfig()

	a.log("info", "Успешно подключено!")
	runtime.EventsEmit(a.ctx, "connected", nil)

	return ConnectionStatus{
		Connected: true,
		RoomCode:  connectedRoomCode,
		Role:      role,
	}
}

// startScreenshotService initializes and starts screenshot capture
func (a *App) startScreenshotService() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.screenshotService != nil {
		a.screenshotService.Stop()
	}

	a.screenshotService = screenshot.NewCaptureService(screenshot.Config{
		IntervalMs: a.settings.ScreenshotInterval,
		Quality:    a.settings.ScreenshotQuality,
	}, a.logger)

	a.screenshotService.SetOnCapture(func(payload *network.ScreenshotPayload) {
		if payload == nil {
			return
		}
		a.mu.RLock()
		client := a.client
		connected := a.connected
		a.mu.RUnlock()

		if client != nil && connected {
			if err := client.SendPayload(network.TypeScreenshot, payload); err != nil {
				a.log("error", "Не удалось отправить скриншот: "+err.Error())
			} else {
				// Обновляем статистику
				a.incrementStat("screenshotsSent", 0)
				a.incrementStat("bytesSent", int64(len(payload.Data)))
				a.log("debug", "Скриншот отправлен")
			}
		}
	})

	a.screenshotService.Start()
	a.log("info", "Захват экрана запущен")
}

// Disconnect disconnects from the middleware
func (a *App) Disconnect() {
	a.mu.Lock()

	// Stop screenshot service
	if a.screenshotService != nil {
		a.screenshotService.Stop()
		a.screenshotService = nil
	}

	client := a.client
	a.client = nil
	a.connected = false
	a.roomCode = ""
	a.role = ""
	a.mu.Unlock()

	if client != nil {
		client.Stop()
	}

	a.log("info", "Отключено")
	runtime.EventsEmit(a.ctx, "disconnected", nil)
}

// GetConnectionStatus returns the current connection status
func (a *App) GetConnectionStatus() ConnectionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return ConnectionStatus{
		Connected: a.connected,
		RoomCode:  a.roomCode,
		Role:      a.role,
	}
}

// SendText sends text to the connected client
func (a *App) SendText(text string) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()

	if client == nil || !connected {
		return nil
	}

	payload := network.TextPayload{
		Text:      text,
		Immediate: false,
	}

	if err := client.SendPayload(network.TypeText, &payload); err != nil {
		a.log("error", "Не удалось отправить текст: "+err.Error())
		return err
	}

	// Обновляем статистику и историю
	a.incrementStat("textsSent", 0)
	a.incrementStat("bytesSent", int64(len(text)))
	a.addTextToHistory(text, "sent")

	a.log("info", "Текст отправлен: "+truncate(text, 50))
	return nil
}

// TypeBuffer types the current keyboard buffer content
// Uses keyInterceptor's TypeAllBuffer which types the entire remaining buffer at once
func (a *App) TypeBuffer() {
	// First try to use keyInterceptor (preferred method with native macOS API)
	if a.keyInterceptor != nil {
		remaining := a.keyInterceptor.GetRemainingBuffer()
		if remaining != "" {
			a.log("info", "Печатаю текст из interceptor буфера: "+truncate(remaining, 30))
			a.keyInterceptor.TypeAllBuffer()
			return
		}
	}

	// Fallback to keyboardHandler if interceptor buffer is empty
	if a.keyboardHandler != nil {
		text := a.keyboardHandler.GetBuffer()
		if text != "" {
			a.log("info", "Печатаю текст из handler буфера: "+truncate(text, 30))
			if err := a.keyboardHandler.TypeBuffer(); err != nil {
				a.log("error", "Ошибка печати: "+err.Error())
			}
			return
		}
	}

	a.log("warn", "Буфер пуст")
}

// GetKeyboardBuffer returns the current keyboard buffer
func (a *App) GetKeyboardBuffer() string {
	if a.keyboardHandler == nil {
		return ""
	}
	return a.keyboardHandler.GetBuffer()
}

// ClearKeyboardBuffer clears the keyboard buffer
func (a *App) ClearKeyboardBuffer() {
	if a.keyboardHandler != nil {
		a.keyboardHandler.ClearBuffer()
		a.log("info", "Буфер очищен")
	}
}

// ToggleInputMode toggles keyboard input mode (key interception)
func (a *App) ToggleInputMode() bool {
	a.log("debug", "[TOGGLE] ToggleInputMode() called")

	if a.keyInterceptor == nil {
		a.log("error", "[TOGGLE] keyInterceptor is nil!")
		return false
	}

	// Get current state
	wasEnabled := a.keyInterceptor.IsEnabled()
	wasRunning := a.keyInterceptor.IsRunning()
	bufLen := a.keyInterceptor.GetBufferLength()
	bufPos := a.keyInterceptor.GetPosition()

	a.log("debug", "[TOGGLE] Current state: enabled=%v, running=%v, bufLen=%d, bufPos=%d", wasEnabled, wasRunning, bufLen, bufPos)

	// Toggle the state
	desiredEnabled := !wasEnabled
	a.log("debug", "[TOGGLE] Setting enabled to %v...", desiredEnabled)
	a.keyInterceptor.SetEnabled(desiredEnabled)
	a.log("debug", "[TOGGLE] SetEnabled() completed")

	// Get the ACTUAL state after SetEnabled (it may have been rejected if buffer is empty)
	actualEnabled := a.keyInterceptor.IsEnabled()
	a.log("debug", "[TOGGLE] Actual enabled state: %v (desired was %v)", actualEnabled, desiredEnabled)

	// Also sync with keyboard handler for compatibility
	if a.keyboardHandler != nil {
		a.log("debug", "[TOGGLE] Syncing with keyboardHandler...")
		a.keyboardHandler.SetInputMode(actualEnabled)
		a.log("debug", "[TOGGLE] keyboardHandler synced")
	}

	// Emit the ACTUAL state, not the desired state
	a.log("debug", "[TOGGLE] Emitting inputModeChanged event with actual state...")
	runtime.EventsEmit(a.ctx, "inputModeChanged", actualEnabled)
	a.log("debug", "[TOGGLE] Event emitted")

	if actualEnabled {
		remaining := a.keyInterceptor.GetRemainingBuffer()
		runeCount := len([]rune(remaining))
		if runeCount > 0 {
			a.log("info", "Режим ввода ВКЛЮЧЁН - нажимайте любые клавиши для ввода текста (%d символов в буфере)", runeCount)
		} else {
			a.log("info", "Режим ввода ВКЛЮЧЁН - но буфер пуст, дождитесь текста от контроллера")
		}
	} else {
		if desiredEnabled && !actualEnabled {
			a.log("warn", "Режим ввода НЕ ВКЛЮЧЁН - буфер пуст! Дождитесь текста от контроллера.")
		} else {
			a.log("info", "Режим ввода ВЫКЛЮЧЕН")
		}
	}

	a.log("debug", "[TOGGLE] ToggleInputMode() returning %v", actualEnabled)
	return actualEnabled
}

// GetInputMode returns current input mode state
func (a *App) GetInputMode() bool {
	if a.keyInterceptor == nil {
		return false
	}
	return a.keyInterceptor.IsEnabled()
}

// SetInputMode sets input mode state explicitly
func (a *App) SetInputMode(enabled bool) bool {
	if a.keyInterceptor == nil {
		return false
	}
	a.keyInterceptor.SetEnabled(enabled)

	// Also sync with keyboard handler for compatibility
	if a.keyboardHandler != nil {
		a.keyboardHandler.SetInputMode(enabled)
	}

	runtime.EventsEmit(a.ctx, "inputModeChanged", enabled)
	return enabled
}

// GetBufferStatus returns the current buffer status for UI
func (a *App) GetBufferStatus() map[string]interface{} {
	if a.keyInterceptor == nil {
		return map[string]interface{}{
			"length":    0,
			"position":  0,
			"remaining": 0,
			"text":      "",
		}
	}
	remaining := a.keyInterceptor.GetRemainingBuffer()
	return map[string]interface{}{
		"length":    a.keyInterceptor.GetBufferLength(),
		"position":  a.keyInterceptor.GetPosition(),
		"remaining": len(remaining),
		"text":      remaining,
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

	// Update screenshot service if running
	if a.screenshotService != nil {
		a.screenshotService.SetInterval(settings.ScreenshotInterval)
		a.screenshotService.SetQuality(settings.ScreenshotQuality)
	}

	// Update screenshot history limit
	if a.screenshotHistory != nil {
		a.screenshotHistory.SetHistoryMaxLen(settings.ScreenshotHistoryLimit)
	}

	// Re-register hotkeys if they changed
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

	// Сохраняем в файл
	a.saveConfig()

	a.log("info", "Настройки сохранены")
}

// GenerateRoomCode generates a random room code
func (a *App) GenerateRoomCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based code if crypto/rand fails
		return strings.ToUpper(hex.EncodeToString([]byte{
			byte(time.Now().UnixNano() & 0xFF),
			byte((time.Now().UnixNano() >> 8) & 0xFF),
			byte((time.Now().UnixNano() >> 16) & 0xFF),
		}))
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

// handleMessage handles incoming network messages
func (a *App) handleMessage(msg *network.Message) {
	switch msg.Type {
	case network.TypeScreenshot:
		var payload network.ScreenshotPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "Ошибка парсинга скриншота")
			return
		}

		// Обновляем статистику
		a.incrementStat("screenshotsReceived", 0)
		a.incrementStat("bytesReceived", int64(len(payload.Data)))

		// Add to screenshot history (controller stores all received screenshots)
		var historyID int
		if a.screenshotHistory != nil {
			historyID = a.screenshotHistory.AddToHistory(&payload)
		}

		// Convert to base64 for frontend
		b64 := base64.StdEncoding.EncodeToString(payload.Data)
		runtime.EventsEmit(a.ctx, "screenshot", map[string]interface{}{
			"id":        historyID,
			"data":      "data:image/jpeg;base64," + b64,
			"width":     payload.Width,
			"height":    payload.Height,
			"timestamp": time.Now().Format(time.RFC3339),
		})

	case network.TypeText:
		a.log("debug", "[TEXT] Received TypeText message, parsing payload...")
		var payload network.TextPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[TEXT] Ошибка парсинга текста: %v", err)
			return
		}
		a.log("debug", "[TEXT] Payload parsed successfully, text length=%d bytes, runes=%d", len(payload.Text), len([]rune(payload.Text)))

		// Обновляем статистику и историю
		a.incrementStat("textsReceived", 0)
		a.incrementStat("bytesReceived", int64(len(payload.Text)))
		a.addTextToHistory(payload.Text, "received")
		a.log("debug", "[TEXT] Stats and history updated")

		// Set text in both handlers
		if a.keyboardHandler != nil {
			a.log("debug", "[TEXT] Setting text in keyboardHandler...")
			a.keyboardHandler.SetText(payload.Text)
			a.log("debug", "[TEXT] keyboardHandler.SetText() completed")
		} else {
			a.log("warn", "[TEXT] keyboardHandler is nil!")
		}

		// Set text in interceptor buffer
		if a.keyInterceptor != nil {
			a.log("debug", "[TEXT] Setting text in keyInterceptor buffer...")
			a.keyInterceptor.SetBuffer(payload.Text)
			a.log("info", "Текст загружен в буфер. Включите режим ввода и нажимайте любые клавиши.")
			a.log("debug", "[TEXT] keyInterceptor.SetBuffer() completed, buffer length=%d", a.keyInterceptor.GetBufferLength())
		} else {
			a.log("warn", "[TEXT] keyInterceptor is nil!")
		}

		// Send just the text string - simpler serialization
		a.log("debug", "[TEXT] Emitting textReceived event to frontend...")
		runtime.EventsEmit(a.ctx, "textReceived", payload.Text)
		a.log("debug", "[TEXT] textReceived event emitted")
		a.log("info", "Получен текст: "+truncate(payload.Text, 50))

	case network.TypeClipboard:
		var payload network.ClipboardPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		runtime.EventsEmit(a.ctx, "clipboardReceived", payload.Text)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// =============== НОВЫЕ МЕТОДЫ ДЛЯ ФИЧ ===============

// GetStatistics возвращает текущую статистику сессии
func (a *App) GetStatistics() Statistics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := a.stats
	if a.connected && !stats.ConnectedAt.IsZero() {
		stats.TotalConnectTime = int64(time.Since(stats.ConnectedAt).Seconds())
	}
	return stats
}

// ResetStatistics сбрасывает статистику
func (a *App) ResetStatistics() {
	a.mu.Lock()
	a.stats = Statistics{}
	a.mu.Unlock()
	a.log("info", "Статистика сброшена")
}

// GetTextHistory возвращает историю текстов
func (a *App) GetTextHistory() []TextHistoryEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]TextHistoryEntry, len(a.textHistory))
	copy(result, a.textHistory)
	return result
}

// addTextToHistory добавляет текст в историю (внутренний метод)
func (a *App) addTextToHistory(text, direction string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := TextHistoryEntry{
		Text:      text,
		Direction: direction,
		Timestamp: time.Now(),
	}
	a.textHistory = append(a.textHistory, entry)

	// Ограничиваем до 50 записей
	if len(a.textHistory) > 50 {
		a.textHistory = a.textHistory[len(a.textHistory)-50:]
	}
}

// ClearTextHistory очищает историю текстов
func (a *App) ClearTextHistory() {
	a.mu.Lock()
	a.textHistory = make([]TextHistoryEntry, 0)
	a.mu.Unlock()
	a.log("info", "История текстов очищена")
}

// incrementStat увеличивает счётчик статистики (внутренний метод)
func (a *App) incrementStat(stat string, value int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch stat {
	case "screenshotsSent":
		a.stats.ScreenshotsSent++
	case "screenshotsReceived":
		a.stats.ScreenshotsReceived++
	case "textsSent":
		a.stats.TextsSent++
	case "textsReceived":
		a.stats.TextsReceived++
	case "bytesSent":
		a.stats.BytesSent += value
	case "bytesReceived":
		a.stats.BytesReceived += value
	}
}

// =============== МЕТОДЫ ДЛЯ РАБОТЫ С РАЗРЕШЕНИЯМИ ===============

// PermissionInfo для фронтенда
type PermissionInfoJS struct {
	Type        string `json:"type"`
	Status      string `json:"status"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// GetPermissions возвращает статус всех разрешений
func (a *App) GetPermissions() []PermissionInfoJS {
	if a.permissionsManager == nil {
		return nil
	}

	perms := a.permissionsManager.GetAllPermissions()
	result := make([]PermissionInfoJS, len(perms))
	for i, p := range perms {
		result[i] = PermissionInfoJS{
			Type:        string(p.Type),
			Status:      string(p.Status),
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
		}
	}
	return result
}

// CheckPermissions проверяет все разрешения и возвращает результат
func (a *App) CheckPermissions() map[string]interface{} {
	if a.permissionsManager == nil {
		return map[string]interface{}{
			"allGranted": true,
			"platform":   "unknown",
		}
	}

	perms := a.permissionsManager.GetAllPermissions()
	allGranted := true
	missing := make([]string, 0)

	for _, p := range perms {
		if p.Required && p.Status != permissions.StatusGranted {
			allGranted = false
			missing = append(missing, string(p.Type))
		}
	}

	return map[string]interface{}{
		"allGranted":  allGranted,
		"missing":     missing,
		"platform":    a.permissionsManager.GetPlatform(),
		"permissions": a.GetPermissions(),
	}
}

// RequestPermission запрашивает конкретное разрешение
func (a *App) RequestPermission(permType string) bool {
	if a.permissionsManager == nil {
		return false
	}

	switch permissions.PermissionType(permType) {
	case permissions.PermissionScreenCapture:
		return a.permissionsManager.RequestScreenCapture()
	case permissions.PermissionAccessibility:
		return a.permissionsManager.RequestAccessibility()
	default:
		return false
	}
}

// RequestAllPermissions запрашивает все необходимые разрешения
func (a *App) RequestAllPermissions() map[string]bool {
	if a.permissionsManager == nil {
		return nil
	}

	results := a.permissionsManager.RequestAllPermissions()
	jsResults := make(map[string]bool)
	for k, v := range results {
		jsResults[string(k)] = v
	}

	// Не вызываем checkAndNotifyPermissions() здесь - фронтенд сам вызывает CheckPermissions()
	// после возврата этой функции. Иначе создаётся бесконечный цикл показа модального окна.

	return jsResults
}

// RecheckPermissions повторно проверяет разрешения и возвращает результат
// Вызывайте этот метод после того, как пользователь дал разрешения
func (a *App) RecheckPermissions() map[string]interface{} {
	a.log("info", "Перепроверка разрешений...")

	result := a.CheckPermissions()

	// Log each permission status
	if perms, ok := result["permissions"].([]PermissionInfoJS); ok {
		for _, p := range perms {
			a.log("info", "Разрешение %s: %s", p.Type, p.Status)
		}
	}

	// Если все разрешения получены, пытаемся перезапустить сервисы
	if allGranted, ok := result["allGranted"].(bool); ok && allGranted {
		a.log("info", "Все разрешения успешно получены!")
		// Try to restart key interceptor if it wasn't running
		a.restartKeyInterceptorIfNeeded()
	} else {
		if missing, ok := result["missing"].([]string); ok {
			a.log("warn", "Недостающие разрешения: %v", missing)
		}
	}

	return result
}

// restartKeyInterceptorIfNeeded перезапускает перехватчик клавиш если он не работает
func (a *App) restartKeyInterceptorIfNeeded() {
	if a.keyInterceptor == nil {
		return
	}

	if !a.keyInterceptor.IsRunning() {
		a.log("info", "Попытка перезапустить перехватчик клавиш...")
		if a.keyInterceptor.Start() {
			a.log("info", "Перехватчик клавиш успешно запущен!")
		} else {
			a.log("warn", "Не удалось запустить перехватчик клавиш - возможно требуется перезапуск приложения")
		}
	}
}

// OpenPermissionSettings открывает настройки для конкретного разрешения
func (a *App) OpenPermissionSettings(permType string) {
	if a.permissionsManager == nil {
		return
	}

	switch permissions.PermissionType(permType) {
	case permissions.PermissionScreenCapture:
		a.permissionsManager.OpenScreenCaptureSettings()
		a.log("info", "Открыты настройки записи экрана")
	case permissions.PermissionAccessibility:
		a.permissionsManager.OpenAccessibilitySettings()
		a.log("info", "Открыты настройки универсального доступа")
	}
}

// GetPlatform возвращает текущую платформу
func (a *App) GetPlatform() string {
	if a.permissionsManager == nil {
		return "unknown"
	}
	return a.permissionsManager.GetPlatform()
}

// =============== МЕТОДЫ ДЛЯ ИСТОРИИ СКРИНШОТОВ ===============

// ScreenshotHistoryEntryJS represents a screenshot history entry for frontend
type ScreenshotHistoryEntryJS struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Size      int    `json:"size"`
}

// GetScreenshotHistory returns the screenshot history metadata
func (a *App) GetScreenshotHistory() []ScreenshotHistoryEntryJS {
	if a.screenshotHistory == nil {
		return []ScreenshotHistoryEntryJS{}
	}

	history := a.screenshotHistory.GetHistory()
	result := make([]ScreenshotHistoryEntryJS, len(history))
	for i, entry := range history {
		result[i] = ScreenshotHistoryEntryJS{
			ID:        entry.ID,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Width:     entry.Width,
			Height:    entry.Height,
			Size:      entry.Size,
		}
	}
	return result
}

// GetScreenshotByID returns a specific screenshot from history as base64
func (a *App) GetScreenshotByID(id int) map[string]interface{} {
	if a.screenshotHistory == nil {
		return nil
	}

	payload, ok := a.screenshotHistory.GetHistoryScreenshot(id)
	if !ok {
		return nil
	}

	b64 := base64.StdEncoding.EncodeToString(payload.Data)
	return map[string]interface{}{
		"id":     id,
		"data":   "data:image/jpeg;base64," + b64,
		"width":  payload.Width,
		"height": payload.Height,
	}
}

// ClearScreenshotHistory clears the screenshot history
func (a *App) ClearScreenshotHistory() {
	if a.screenshotHistory != nil {
		a.screenshotHistory.ClearHistory()
	}
	a.log("info", "История скриншотов очищена")
}

// =============== МЕТОДЫ ДЛЯ ПЕРЕЗАПУСКА ПРИЛОЖЕНИЯ ===============

// RestartApp перезапускает приложение (необходимо после выдачи разрешений на macOS)
func (a *App) RestartApp() {
	a.log("info", "Инициирован перезапуск приложения...")

	// Получаем путь к исполняемому файлу
	execPath, err := os.Executable()
	if err != nil {
		a.log("error", "Не удалось получить путь к исполняемому файлу: "+err.Error())
		return
	}

	a.log("info", "Путь к приложению: "+execPath)

	// На macOS нужно найти .app bundle
	// Путь обычно: /path/to/App.app/Contents/MacOS/app
	appBundlePath := execPath
	if strings.Contains(execPath, ".app/Contents/MacOS/") {
		// Извлекаем путь к .app bundle
		idx := strings.Index(execPath, ".app/Contents/MacOS/")
		appBundlePath = execPath[:idx+4] // включая ".app"
		a.log("info", "App bundle: "+appBundlePath)
	}

	// Запускаем новый экземпляр через open (для macOS)
	var cmd *exec.Cmd
	if strings.HasSuffix(appBundlePath, ".app") {
		// macOS: используем open для запуска .app bundle
		cmd = exec.Command("open", "-n", appBundlePath)
	} else {
		// Fallback: запускаем напрямую
		cmd = exec.Command(execPath)
	}

	// Запускаем новый процесс
	if err := cmd.Start(); err != nil {
		a.log("error", "Не удалось запустить новый экземпляр: "+err.Error())
		return
	}

	a.log("info", "Новый экземпляр запущен, завершаем текущий...")

	// Закрываем текущее приложение
	runtime.Quit(a.ctx)
}

// QuitApp завершает приложение
func (a *App) QuitApp() {
	a.log("info", "Завершение приложения...")
	runtime.Quit(a.ctx)
}

// GetAppExecutablePath возвращает путь к исполняемому файлу (для отладки)
func (a *App) GetAppExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "unknown: " + err.Error()
	}
	return execPath
}

// =============== УЛУЧШЕННЫЕ МЕТОДЫ ДЛЯ РАБОТЫ С РАЗРЕШЕНИЯМИ ===============

// StartPermissionPolling запускает периодическую проверку разрешений
// Это полезно для обнаружения момента, когда пользователь выдал разрешения
func (a *App) StartPermissionPolling(intervalMs int) {
	if intervalMs < 500 {
		intervalMs = 500
	}
	if intervalMs > 10000 {
		intervalMs = 10000
	}

	// Stop any existing polling goroutine
	a.permissionPollingMu.Lock()
	if a.permissionPollingCancel != nil {
		a.permissionPollingCancel()
		a.permissionPollingCancel = nil
	}

	// Create new context for this polling goroutine
	ctx, cancel := context.WithCancel(a.ctx)
	a.permissionPollingCancel = cancel
	a.permissionPollingMu.Unlock()

	a.log("info", "Запуск мониторинга разрешений (интервал %d мс)...", intervalMs)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		prevStatus := make(map[permissions.PermissionType]permissions.PermissionStatus)

		for {
			select {
			case <-ctx.Done():
				a.log("debug", "Permission polling stopped")
				return
			case <-ticker.C:
				if a.permissionsManager == nil {
					continue
				}

				// ВАЖНО: Если keyInterceptor уже работает, значит accessibility permission есть
				// В этом случае используем кешированную проверку, чтобы не создавать
				// конфликтующий event tap (macOS ограничивает количество активных tap'ов)
				var perms []permissions.PermissionInfo
				if a.keyInterceptor != nil && a.keyInterceptor.IsRunning() {
					// Interceptor работает = accessibility granted, используем кешированные значения
					perms = a.permissionsManager.GetAllPermissionsCached()
				} else {
					// Interceptor не работает, делаем реальную проверку
					perms = a.permissionsManager.GetAllPermissions()
				}
				changed := false

				for _, p := range perms {
					if prev, ok := prevStatus[p.Type]; ok {
						if prev != p.Status {
							a.log("info", "Статус разрешения %s изменился: %s -> %s", p.Type, prev, p.Status)
							changed = true
						}
					}
					prevStatus[p.Type] = p.Status
				}

				if changed {
					// Отправляем событие на фронтенд (только ОДНО событие для всех изменений)
					runtime.EventsEmit(a.ctx, "permissionsChanged", map[string]interface{}{
						"permissions": a.GetPermissions(),
						"allGranted":  a.permissionsManager.HasAllRequired(),
					})

					// Если все разрешения получены, пробуем перезапустить сервисы
					if a.permissionsManager.HasAllRequired() {
						a.log("info", "Все разрешения получены! Пробуем инициализировать сервисы...")
						a.restartKeyInterceptorIfNeeded()
					}
				}
			}
		}
	}()
}

// GetDetailedPermissionStatus возвращает детальный статус разрешений с дополнительной информацией
func (a *App) GetDetailedPermissionStatus() map[string]interface{} {
	if a.permissionsManager == nil {
		return map[string]interface{}{
			"error": "permissions manager not initialized",
		}
	}

	// Логируем диагностику разрешений
	a.permissionsManager.LogDiagnostics()

	perms := a.permissionsManager.GetAllPermissions()
	permDetails := make([]map[string]interface{}, len(perms))

	for i, p := range perms {
		permDetails[i] = map[string]interface{}{
			"type":        string(p.Type),
			"status":      string(p.Status),
			"name":        p.Name,
			"description": p.Description,
			"required":    p.Required,
			"granted":     p.Status == permissions.StatusGranted,
		}
	}

	// Проверяем работоспособность сервисов
	keyInterceptorRunning := false
	if a.keyInterceptor != nil {
		keyInterceptorRunning = a.keyInterceptor.IsRunning()
	}

	// Проверяем нужен ли перезапуск (cached API говорит да, но реальный тест - нет)
	needsRestart := a.permissionsManager.NeedsRestart()

	// Также считаем что нужен перезапуск если все разрешения есть но interceptor не работает
	if !needsRestart && a.permissionsManager.HasAllRequired() && !keyInterceptorRunning {
		needsRestart = true
	}

	return map[string]interface{}{
		"platform":              a.permissionsManager.GetPlatform(),
		"permissions":           permDetails,
		"allGranted":            a.permissionsManager.HasAllRequired(),
		"keyInterceptorRunning": keyInterceptorRunning,
		"needsRestart":          needsRestart,
		"executablePath":        a.GetAppExecutablePath(),
	}
}

// =============== МЕТОДЫ ДЛЯ СОХРАНЕНИЯ СКРИНШОТОВ ===============

// SaveScreenshotToFile сохраняет скриншот в файл
// Если директория не указана - использует директорию из настроек или Downloads
func (a *App) SaveScreenshotToFile(base64Data string, filename string) (string, error) {
	a.log("info", "Сохранение скриншота: %s", filename)

	// Определяем директорию для сохранения
	saveDir := a.settings.ScreenshotSaveDir
	if saveDir == "" {
		// По умолчанию - папка Downloads
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
		}
		saveDir = filepath.Join(homeDir, "Downloads")
	}

	// Создаем директорию если не существует
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать директорию: %w", err)
	}

	// Генерируем имя файла если не указано
	if filename == "" {
		filename = fmt.Sprintf("screenshot-%s.jpg", time.Now().Format("2006-01-02_15-04-05"))
	}

	// Полный путь к файлу
	fullPath := filepath.Join(saveDir, filename)

	// Декодируем base64 данные
	// Убираем префикс data:image/jpeg;base64, если есть
	data := base64Data
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования base64: %w", err)
	}

	// Записываем в файл
	if err := os.WriteFile(fullPath, decoded, 0644); err != nil {
		return "", fmt.Errorf("ошибка записи файла: %w", err)
	}

	a.log("info", "Скриншот сохранен: %s", fullPath)
	return fullPath, nil
}

// SelectScreenshotDirectory открывает диалог выбора директории
func (a *App) SelectScreenshotDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Выберите папку для сохранения скриншотов",
		DefaultDirectory:     a.settings.ScreenshotSaveDir,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}

	if dir != "" {
		a.settings.ScreenshotSaveDir = dir
		a.log("info", "Выбрана директория для скриншотов: %s", dir)
	}

	return dir, nil
}

// GetScreenshotSaveDir возвращает текущую директорию для сохранения
func (a *App) GetScreenshotSaveDir() string {
	if a.settings.ScreenshotSaveDir != "" {
		return a.settings.ScreenshotSaveDir
	}
	// По умолчанию Downloads
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "Downloads")
}

// SetScreenshotSaveDir устанавливает директорию для сохранения
func (a *App) SetScreenshotSaveDir(dir string) error {
	// Проверяем что директория существует или можно создать
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("не удалось создать директорию: %w", err)
		}
	}
	a.settings.ScreenshotSaveDir = dir
	a.log("info", "Директория для скриншотов установлена: %s", dir)
	return nil
}

// =============== МЕТОДЫ ДЛЯ РЕЖИМА НЕВИДИМОСТИ ===============

// ToggleInvisibility toggles window invisibility mode and returns the new state
func (a *App) ToggleInvisibility() bool {
	if a.invisibilityManager == nil {
		a.log("error", "Invisibility manager not initialized")
		return false
	}

	newState := a.invisibilityManager.Toggle()

	if newState {
		a.log("info", "Режим невидимости ВКЛЮЧЁН - окно скрыто от захвата экрана")
	} else {
		a.log("info", "Режим невидимости ВЫКЛЮЧЕН - окно видно при захвате экрана")
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "invisibilityChanged", newState)

	return newState
}

// SetInvisibility sets window invisibility mode explicitly
func (a *App) SetInvisibility(enabled bool) bool {
	if a.invisibilityManager == nil {
		return false
	}

	a.invisibilityManager.SetEnabled(enabled)

	if enabled {
		a.log("info", "Режим невидимости ВКЛЮЧЁН")
	} else {
		a.log("info", "Режим невидимости ВЫКЛЮЧЕН")
	}

	runtime.EventsEmit(a.ctx, "invisibilityChanged", enabled)
	return enabled
}

// GetInvisibilityStatus returns current invisibility status
func (a *App) GetInvisibilityStatus() map[string]interface{} {
	if a.invisibilityManager == nil {
		return map[string]interface{}{
			"enabled":     false,
			"supported":   false,
			"windowCount": 0,
		}
	}

	return map[string]interface{}{
		"enabled":     a.invisibilityManager.IsEnabled(),
		"supported":   a.invisibilityManager.IsSupported(),
		"windowCount": a.invisibilityManager.GetWindowCount(),
	}
}

// IsInvisibilitySupported returns whether invisibility mode is supported on this platform
func (a *App) IsInvisibilitySupported() bool {
	if a.invisibilityManager == nil {
		return false
	}
	return a.invisibilityManager.IsSupported()
}
