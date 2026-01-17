package hotkey

import (
	"context"
	"runtime"
	"strings"
	"sync"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"

	"buffer-sharer-app/internal/logging"
)

// Platform detection
var isMacOS = runtime.GOOS == "darwin"
var isWindows = runtime.GOOS == "windows"

// Action represents a hotkey action identifier
type Action string

const (
	ActionToggleInputMode Action = "toggle_input_mode"
	ActionTakeScreenshot  Action = "take_screenshot"
	ActionPasteFromBuffer Action = "paste_from_buffer"
)

// Handler is a function called when a hotkey is triggered
type Handler func()

// Manager manages global hotkeys
type Manager struct {
	logger     *logging.Logger
	mu         sync.RWMutex
	hotkeys    map[Action]*registeredHotkey
	handlers   map[Action]Handler
	running    bool
	cancelFunc context.CancelFunc
}

type registeredHotkey struct {
	hk     *hotkey.Hotkey
	keyStr string
}

// NewManager creates a new hotkey manager
func NewManager(logger *logging.Logger) *Manager {
	return &Manager{
		logger:   logger,
		hotkeys:  make(map[Action]*registeredHotkey),
		handlers: make(map[Action]Handler),
	}
}

// RegisterHandler registers a handler for an action
func (m *Manager) RegisterHandler(action Action, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[action] = handler
}

// Register registers a hotkey for an action
func (m *Manager) Register(action Action, keyCombo string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unregister existing hotkey if any
	if existing, ok := m.hotkeys[action]; ok {
		existing.hk.Unregister()
		delete(m.hotkeys, action)
	}

	// Parse the key combination
	mods, key, err := parseKeyCombo(keyCombo)
	if err != nil {
		return err
	}

	// Create new hotkey
	hk := hotkey.New(mods, key)

	m.hotkeys[action] = &registeredHotkey{
		hk:     hk,
		keyStr: keyCombo,
	}

	m.logger.Info("hotkey", "Registered hotkey %s for action %s", keyCombo, action)
	return nil
}

// Start starts listening for hotkeys (must be called from main thread)
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.mu.Unlock()

	// Start hotkey listeners
	mainthread.Init(func() {
		m.startListeners(ctx)
	})
}

// StartAsync starts hotkey listening in a goroutine
func (m *Manager) StartAsync() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.mu.Unlock()

	go m.startListeners(ctx)
}

// startListeners registers and starts all hotkey listeners
func (m *Manager) startListeners(ctx context.Context) {
	m.mu.RLock()
	hotkeys := make(map[Action]*registeredHotkey)
	for k, v := range m.hotkeys {
		hotkeys[k] = v
	}
	m.mu.RUnlock()

	// Register all hotkeys
	for action, rhk := range hotkeys {
		if err := rhk.hk.Register(); err != nil {
			m.logger.Error("hotkey", "Failed to register hotkey for %s: %v", action, err)
			continue
		}

		// Start listener for this hotkey
		go m.listenHotkey(ctx, action, rhk.hk)
	}

	m.logger.Info("hotkey", "Hotkey manager started")

	// Wait for context cancellation
	<-ctx.Done()

	// Unregister all hotkeys
	for _, rhk := range hotkeys {
		rhk.hk.Unregister()
	}

	m.logger.Info("hotkey", "Hotkey manager stopped")
}

// listenHotkey listens for a specific hotkey
func (m *Manager) listenHotkey(ctx context.Context, action Action, hk *hotkey.Hotkey) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-hk.Keydown():
			m.mu.RLock()
			handler := m.handlers[action]
			m.mu.RUnlock()

			if handler != nil {
				m.logger.Debug("hotkey", "Hotkey triggered: %s", action)
				handler()
			}
		}
	}
}

// Stop stops the hotkey manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
}

// IsRunning returns whether the manager is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetRegisteredKey returns the key combo string for an action
func (m *Manager) GetRegisteredKey(action Action) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if rhk, ok := m.hotkeys[action]; ok {
		return rhk.keyStr, true
	}
	return "", false
}

// parseKeyCombo parses a key combination string like "Ctrl+Shift+J"
func parseKeyCombo(combo string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 {
		return nil, 0, nil
	}

	var mods []hotkey.Modifier
	var key hotkey.Key

	for i, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))

		if i == len(parts)-1 {
			// Last part is the key
			key = parseKey(part)
		} else {
			// Other parts are modifiers
			mod := parseModifier(part)
			if mod != 0 {
				mods = append(mods, mod)
			}
		}
	}

	return mods, key, nil
}

// Modifier constants for different platforms
// These values are platform-specific and match golang.design/x/hotkey internal values
const (
	// Common modifiers
	modCtrl  hotkey.Modifier = hotkey.ModCtrl
	modShift hotkey.Modifier = hotkey.ModShift
)

// parseModifier parses a modifier string
func parseModifier(mod string) hotkey.Modifier {
	switch strings.ToLower(mod) {
	case "ctrl", "control":
		return modCtrl
	case "shift":
		return modShift
	case "alt", "option":
		// Alt/Option modifier - platform specific
		// On macOS this is ModOption, on Windows/Linux it's typically Mod1 (1 << 3)
		if isMacOS {
			return hotkey.ModOption
		}
		// Windows/Linux: Alt is typically 1 << 3 in X11/Windows
		return hotkey.Modifier(1 << 3)
	case "cmd", "command":
		// Cmd modifier (macOS only, maps to Ctrl on other platforms for compatibility)
		if isMacOS {
			return hotkey.ModCmd
		}
		// On Windows/Linux, treat Cmd as Ctrl for cross-platform compatibility
		return modCtrl
	case "super", "win":
		// Windows/Super key - maps to Cmd on macOS, Win key on Windows
		if isMacOS {
			return hotkey.ModCmd
		}
		// Windows: Win key is typically Mod4 (1 << 6)
		return hotkey.Modifier(1 << 6)
	default:
		return 0
	}
}

// GetPlatformModifier returns the appropriate modifier key name for the current platform
func GetPlatformModifier() string {
	if isMacOS {
		return "Cmd"
	}
	return "Ctrl"
}

// GetPlatformHotkey converts a cross-platform hotkey string to platform-specific
// e.g., "Mod+Shift+J" becomes "Cmd+Shift+J" on macOS or "Ctrl+Shift+J" on Windows
func GetPlatformHotkey(combo string) string {
	mod := GetPlatformModifier()
	return strings.Replace(combo, "Mod", mod, 1)
}

// NormalizeHotkey normalizes a hotkey string to use the correct modifier for the platform
func NormalizeHotkey(combo string) string {
	parts := strings.Split(combo, "+")
	var result []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)

		// Normalize Ctrl/Cmd based on platform
		if lower == "ctrl" || lower == "control" || lower == "cmd" || lower == "command" {
			if isMacOS {
				result = append(result, "Cmd")
			} else {
				result = append(result, "Ctrl")
			}
		} else if lower == "alt" || lower == "option" {
			if isMacOS {
				result = append(result, "Option")
			} else {
				result = append(result, "Alt")
			}
		} else if lower == "shift" {
			result = append(result, "Shift")
		} else {
			// Key - uppercase first letter
			if len(part) > 0 {
				result = append(result, strings.ToUpper(part[:1])+strings.ToLower(part[1:]))
			}
		}
	}

	return strings.Join(result, "+")
}

// parseKey parses a key string
func parseKey(k string) hotkey.Key {
	switch strings.ToLower(k) {
	// Letters
	case "a":
		return hotkey.KeyA
	case "b":
		return hotkey.KeyB
	case "c":
		return hotkey.KeyC
	case "d":
		return hotkey.KeyD
	case "e":
		return hotkey.KeyE
	case "f":
		return hotkey.KeyF
	case "g":
		return hotkey.KeyG
	case "h":
		return hotkey.KeyH
	case "i":
		return hotkey.KeyI
	case "j":
		return hotkey.KeyJ
	case "k":
		return hotkey.KeyK
	case "l":
		return hotkey.KeyL
	case "m":
		return hotkey.KeyM
	case "n":
		return hotkey.KeyN
	case "o":
		return hotkey.KeyO
	case "p":
		return hotkey.KeyP
	case "q":
		return hotkey.KeyQ
	case "r":
		return hotkey.KeyR
	case "s":
		return hotkey.KeyS
	case "t":
		return hotkey.KeyT
	case "u":
		return hotkey.KeyU
	case "v":
		return hotkey.KeyV
	case "w":
		return hotkey.KeyW
	case "x":
		return hotkey.KeyX
	case "y":
		return hotkey.KeyY
	case "z":
		return hotkey.KeyZ

	// Numbers
	case "0":
		return hotkey.Key0
	case "1":
		return hotkey.Key1
	case "2":
		return hotkey.Key2
	case "3":
		return hotkey.Key3
	case "4":
		return hotkey.Key4
	case "5":
		return hotkey.Key5
	case "6":
		return hotkey.Key6
	case "7":
		return hotkey.Key7
	case "8":
		return hotkey.Key8
	case "9":
		return hotkey.Key9

	// Function keys
	case "f1":
		return hotkey.KeyF1
	case "f2":
		return hotkey.KeyF2
	case "f3":
		return hotkey.KeyF3
	case "f4":
		return hotkey.KeyF4
	case "f5":
		return hotkey.KeyF5
	case "f6":
		return hotkey.KeyF6
	case "f7":
		return hotkey.KeyF7
	case "f8":
		return hotkey.KeyF8
	case "f9":
		return hotkey.KeyF9
	case "f10":
		return hotkey.KeyF10
	case "f11":
		return hotkey.KeyF11
	case "f12":
		return hotkey.KeyF12

	// Special keys
	case "space":
		return hotkey.KeySpace
	case "return", "enter":
		return hotkey.KeyReturn
	case "escape", "esc":
		return hotkey.KeyEscape
	case "tab":
		return hotkey.KeyTab
	case "delete", "backspace":
		return hotkey.KeyDelete

	default:
		return 0
	}
}
