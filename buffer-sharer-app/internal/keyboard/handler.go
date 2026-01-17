package keyboard

import (
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-vgo/robotgo"

	"buffer-sharer-app/internal/logging"
)

// Handler manages keyboard input injection
type Handler struct {
	buffer    *Buffer
	logger    *logging.Logger
	mu        sync.RWMutex
	inputMode bool
	typeDelay int // Delay between keystrokes in ms
}

// NewHandler creates a new keyboard handler
func NewHandler(logger *logging.Logger) *Handler {
	return &Handler{
		buffer:    NewBuffer(),
		logger:    logger,
		typeDelay: 0, // No delay by default for speed
	}
}

// SetText sets text in the buffer for later typing
func (h *Handler) SetText(text string) {
	h.buffer.Set(text)
	h.logger.Debug("keyboard", "Buffer set with %d characters", utf8.RuneCountInString(text))
}

// AppendText appends text to the buffer
func (h *Handler) AppendText(text string) {
	h.buffer.Append(text)
	h.logger.Debug("keyboard", "Appended %d characters to buffer", utf8.RuneCountInString(text))
}

// GetBuffer returns the current buffer content
func (h *Handler) GetBuffer() string {
	return h.buffer.Get()
}

// ClearBuffer clears the buffer
func (h *Handler) ClearBuffer() {
	h.buffer.Clear()
	h.logger.Debug("keyboard", "Buffer cleared")
}

// TypeBuffer types out the current buffer content and clears it
func (h *Handler) TypeBuffer() error {
	text := h.buffer.Clear()
	if text == "" {
		return nil
	}
	return h.TypeText(text)
}

// TypeText immediately types the given text
func (h *Handler) TypeText(text string) error {
	if text == "" {
		return nil
	}

	h.logger.Info("keyboard", "Typing %d characters", utf8.RuneCountInString(text))

	// Use robotgo to type the text
	// TypeStr handles unicode characters
	if h.typeDelay > 0 {
		for _, char := range text {
			robotgo.Type(string(char))
			time.Sleep(time.Duration(h.typeDelay) * time.Millisecond)
		}
	} else {
		robotgo.Type(text)
	}

	h.logger.Debug("keyboard", "Finished typing")
	return nil
}

// SetInputMode sets the input mode state
func (h *Handler) SetInputMode(enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inputMode = enabled
	if enabled {
		h.logger.Info("keyboard", "Input mode enabled")
	} else {
		h.logger.Info("keyboard", "Input mode disabled")
	}
}

// IsInputMode returns true if input mode is enabled
func (h *Handler) IsInputMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.inputMode
}

// ToggleInputMode toggles input mode and returns the new state
func (h *Handler) ToggleInputMode() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inputMode = !h.inputMode
	if h.inputMode {
		h.logger.Info("keyboard", "Input mode toggled ON")
	} else {
		h.logger.Info("keyboard", "Input mode toggled OFF")
	}
	return h.inputMode
}

// SetTypeDelay sets the delay between keystrokes in milliseconds
func (h *Handler) SetTypeDelay(delayMs int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.typeDelay = delayMs
}

// PressKey simulates a single key press
func (h *Handler) PressKey(key string) error {
	robotgo.KeyTap(key)
	return nil
}

// PressKeyCombo simulates a key combination (e.g., ctrl+v)
func (h *Handler) PressKeyCombo(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	// robotgo expects the main key first, then modifiers
	// But we'll use the more intuitive order
	if len(keys) == 1 {
		robotgo.KeyTap(keys[0])
	} else {
		// Last key is the main key, others are modifiers
		mainKey := keys[len(keys)-1]
		modifiers := keys[:len(keys)-1]
		// Convert []string to variadic arguments
		args := make([]interface{}, len(modifiers))
		for i, m := range modifiers {
			args[i] = m
		}
		robotgo.KeyTap(mainKey, args...)
	}
	return nil
}

// GetBuffer returns the underlying buffer
func (h *Handler) Buffer() *Buffer {
	return h.buffer
}
