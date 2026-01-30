//go:build !darwin && !windows

package keyboard

import (
	"github.com/go-vgo/robotgo"
)

// KeyInterceptor intercepts keyboard events and replaces them with buffer content
// This is a stub implementation for non-macOS platforms
type KeyInterceptor struct {
	buffer        []rune
	position      int
	enabled       bool
	running       bool
	logger        interface{ Info(string, string, ...interface{}) }
	onBufferEmpty func()
}

// NewKeyInterceptor creates a new key interceptor
func NewKeyInterceptor(logger interface{ Info(string, string, ...interface{}) }) *KeyInterceptor {
	return &KeyInterceptor{
		logger: logger,
	}
}

// MaxBufferSize is the maximum allowed buffer size (10MB) to prevent memory exhaustion
const MaxBufferSize = 10 * 1024 * 1024

// SetBuffer sets the text buffer to type
func (ki *KeyInterceptor) SetBuffer(text string) {
	// Validate buffer size to prevent memory exhaustion from malicious input
	if len(text) > MaxBufferSize {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Buffer size %d exceeds maximum %d, truncating", len(text), MaxBufferSize)
		}
		text = text[:MaxBufferSize]
	}
	ki.buffer = []rune(text)
	ki.position = 0
}

// GetRemainingBuffer returns the remaining buffer content
func (ki *KeyInterceptor) GetRemainingBuffer() string {
	if ki.position >= len(ki.buffer) {
		return ""
	}
	return string(ki.buffer[ki.position:])
}

// GetBufferLength returns total buffer length
func (ki *KeyInterceptor) GetBufferLength() int {
	return len(ki.buffer)
}

// GetPosition returns current position in buffer
func (ki *KeyInterceptor) GetPosition() int {
	return ki.position
}

// ClearBuffer clears the buffer
func (ki *KeyInterceptor) ClearBuffer() {
	ki.buffer = nil
	ki.position = 0
}

// SetOnBufferEmpty sets callback for when buffer is exhausted
func (ki *KeyInterceptor) SetOnBufferEmpty(callback func()) {
	ki.onBufferEmpty = callback
}

// Start starts the key interceptor (stub - not implemented on this platform)
func (ki *KeyInterceptor) Start() bool {
	if ki.logger != nil {
		ki.logger.Info("interceptor", "Key interception not supported on this platform")
	}
	return false
}

// Stop stops the key interceptor
func (ki *KeyInterceptor) Stop() {
	ki.running = false
	ki.enabled = false
}

// SetEnabled enables or disables key interception
// On non-macOS, this types the entire buffer at once when enabled
func (ki *KeyInterceptor) SetEnabled(enabled bool) {
	ki.enabled = enabled

	// On non-macOS, just type the entire buffer when enabled
	if enabled && len(ki.buffer) > 0 && ki.position < len(ki.buffer) {
		text := string(ki.buffer[ki.position:])
		robotgo.Type(text)
		ki.position = len(ki.buffer)

		if ki.onBufferEmpty != nil {
			ki.onBufferEmpty()
		}
	}
}

// IsEnabled returns whether interception is enabled
func (ki *KeyInterceptor) IsEnabled() bool {
	return ki.enabled
}

// IsRunning returns whether the interceptor is running
func (ki *KeyInterceptor) IsRunning() bool {
	return ki.running
}
