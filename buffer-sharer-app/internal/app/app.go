package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/hotkey"
	"buffer-sharer-app/internal/invisibility"
	"buffer-sharer-app/internal/keyboard"
	"buffer-sharer-app/internal/logging"
	"buffer-sharer-app/internal/network"
	"buffer-sharer-app/internal/overlay"
	"buffer-sharer-app/internal/permissions"
	"buffer-sharer-app/internal/screenshot"
)

// Platform-specific hotkey defaults
func getDefaultHotkeys() (toggle, screenshot, paste string) {
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
	HotkeyInvisibility string `json:"hotkeyInvisibility"`
	AutoConnect            bool   `json:"autoConnect"`
	LastRole               string `json:"lastRole"`
	LastRoomCode           string `json:"lastRoomCode"`
	SoundEnabled           bool   `json:"soundEnabled"`
	Theme                  string `json:"theme"`
	ScreenshotSaveDir      string `json:"screenshotSaveDir"`
	ScreenshotHistoryLimit int    `json:"screenshotHistoryLimit"`
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
	TotalConnectTime    int64     `json:"totalConnectTime"`
}

// TextHistoryEntry represents a sent/received text
type TextHistoryEntry struct {
	Text      string    `json:"text"`
	Direction string    `json:"direction"`
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
	cancelFunc context.CancelFunc
	mu         sync.RWMutex
	logger     *logging.Logger

	// Connection state
	client     *network.Client
	connected  bool
	connecting bool
	roomCode   string
	role       string

	// Services
	screenshotService   *screenshot.CaptureService
	keyboardHandler     *keyboard.Handler
	keyInterceptor      *keyboard.KeyInterceptor
	permissionsManager  *permissions.Manager
	hotkeyManager       *hotkey.Manager
	invisibilityManager *invisibility.Manager
	overlayManager      *overlay.Manager

	// Settings
	settings Settings

	// Data
	stats       Statistics
	textHistory []TextHistoryEntry
	configPath  string

	// Screenshot history for controller
	screenshotHistory *screenshot.CaptureService

	// Safe event channel for CGo callbacks
	eventChan       chan func()
	eventChanClosed atomic.Bool

	// Permission polling control
	permissionPollingCancel context.CancelFunc
	permissionPollingMu     sync.Mutex

	// Active hints and text overlays state
	activeHints        map[string]*network.HintPayload
	activeTextOverlays map[string]*network.TextOverlayPayload
	hintsMu            sync.RWMutex
	textOverlayCounter int64
}

// NewApp creates a new App application struct
func NewApp() *App {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".buffer-sharer", "config.json")

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
			HotkeyInvisibility: "Ctrl+Shift+I",
			AutoConnect:        false,
			LastRole:           "controller",
			SoundEnabled:       true,
			Theme:              "dark",
		},
		textHistory:        make([]TextHistoryEntry, 0),
		configPath:         configPath,
		activeHints:        make(map[string]*network.HintPayload),
		activeTextOverlays: make(map[string]*network.TextOverlayPayload),
	}
}

// sendEvent safely sends a function to the event channel.
func (a *App) sendEvent(fn func()) bool {
	if a.eventChanClosed.Load() {
		return false
	}
	select {
	case a.eventChan <- fn:
		return true
	default:
		return false
	}
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	a.ctx = ctx
	a.cancelFunc = cancel

	a.eventChan = make(chan func(), 100)

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

	logger, err := logging.NewLogger(logging.Config{
		Enabled:    true,
		MaxEntries: 1000,
		Level:      logging.LevelDebug,
		LogToFile:  true,
		Role:       "app",
	})
	if err == nil {
		a.logger = logger
	}

	a.permissionsManager = permissions.NewManager()
	a.invisibilityManager = invisibility.NewManager()
	a.overlayManager = overlay.NewManager()

	a.overlayManager.SetOnAction(func(action, actionType, id string) {
		a.log("debug", "[OVERLAY] Action from JS: action=%s type=%s id=%s", action, actionType, id)
		switch actionType {
		case "hint":
			switch action {
			case "delete":
				a.hintsMu.Lock()
				delete(a.activeHints, id)
				a.hintsMu.Unlock()
				a.overlayManager.RemoveHintRect(id)
			case "collapse", "expand":
				a.overlayManager.SyncHintRects()
			}
		case "text":
			if action == "delete" {
				a.hintsMu.Lock()
				delete(a.activeTextOverlays, id)
				a.hintsMu.Unlock()
				a.overlayManager.RemoveTextRect(id)
			}
		}
	})

	a.overlayManager.StartHintInteraction()

	a.keyboardHandler = keyboard.NewHandler(a.logger)

	a.keyInterceptor = keyboard.NewKeyInterceptor(a.logger)
	a.keyInterceptor.SetOnBufferEmpty(func() {
		a.sendEvent(func() {
			a.log("info", "Буфер исчерпан - режим ввода выключен")
			runtime.EventsEmit(a.ctx, "inputModeChanged", false)
			runtime.EventsEmit(a.ctx, "bufferExhausted", nil)
			a.showOverlayToast("Буфер исчерпан — текст введён полностью", "success")
		})
	})

	a.keyInterceptor.SetOnToggle(func() {
		a.sendEvent(func() {
			a.log("info", "Toggle hotkey detected in event tap")
			a.ToggleInputMode()
		})
	})

	a.keyInterceptor.SetOnPaste(func() {
		a.sendEvent(func() {
			a.log("info", "Paste hotkey detected in event tap")
			a.TypeBuffer()
		})
	})

	historyLimit := a.settings.ScreenshotHistoryLimit
	if historyLimit <= 0 {
		historyLimit = 50
	}
	a.screenshotHistory = screenshot.NewCaptureService(screenshot.Config{
		IntervalMs:    0,
		Quality:       80,
		HistoryMaxLen: historyLimit,
	}, a.logger)

	a.loadConfig()

	a.log("info", "Buffer Sharer запущен")

	// Diagnostic: verify overlay is working after 2 seconds
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			visible, jsWorks, info := a.overlayManager.DiagnosticCheck()
			a.log("info", "[OVERLAY DIAG] visible=%v jsWorks=%v info=%s", visible, jsWorks, info)
			if !visible {
				a.log("warn", "[OVERLAY DIAG] Overlay window is NOT visible!")
			}
			if !jsWorks {
				a.log("warn", "[OVERLAY DIAG] Overlay JS is NOT executing!")
			}

			// Fetch JS log from overlay
			jsLog := a.overlayManager.EvalJSWithResult(`typeof getJSLog === 'function' ? getJSLog() : 'getJSLog not found'`)
			a.log("info", "[OVERLAY DIAG] JS log from overlay: %s", jsLog)

			// Force a test toast
			a.overlayManager.EvalJS(`showToast("Overlay тест: если видишь это — overlay работает!", "success", 5000)`)

			// Check window count
			wc := a.invisibilityManager.GetWindowCount()
			a.log("info", "[OVERLAY DIAG] App window count: %d", wc)
		}
	}()

	// Second diagnostic after 5 seconds (captures more state)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			jsLog := a.overlayManager.EvalJSWithResult(`typeof getJSLog === 'function' ? getJSLog() : 'N/A'`)
			a.log("info", "[OVERLAY DIAG +5s] JS log: %s", jsLog)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			a.initHotkeyManager()
			a.log("info", "Hotkey manager initialized after startup delay")
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1000 * time.Millisecond):
			a.checkAndNotifyPermissions()
		}
	}()
}

// Shutdown is called when the app is closing
func (a *App) Shutdown(ctx context.Context) {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
		a.log("info", "Hotkey manager stopped")
	}

	if a.screenshotService != nil {
		a.screenshotService.Stop()
	}

	if a.keyInterceptor != nil {
		a.keyInterceptor.Stop()
	}

	if a.overlayManager != nil {
		a.overlayManager.Destroy()
	}

	a.eventChanClosed.Store(true)
	if a.eventChan != nil {
		close(a.eventChan)
	}

	done := make(chan struct{})
	go func() {
		a.Disconnect()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
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
