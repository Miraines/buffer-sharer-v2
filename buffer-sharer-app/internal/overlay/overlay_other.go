//go:build !darwin && !windows

package overlay

// Manager is a no-op overlay manager for unsupported platforms
type Manager struct{}

// NewManager creates a no-op overlay manager
func NewManager() *Manager {
	return &Manager{}
}

// Show is a no-op on unsupported platforms
func (m *Manager) Show() {}

// Hide is a no-op on unsupported platforms
func (m *Manager) Hide() {}

// IsVisible always returns false on unsupported platforms
func (m *Manager) IsVisible() bool { return false }

// EvalJS is a no-op on unsupported platforms
func (m *Manager) EvalJS(js string) {}

// Destroy is a no-op on unsupported platforms
func (m *Manager) Destroy() {}

// IsSupported returns false on unsupported platforms
func (m *Manager) IsSupported() bool { return false }

// SetIgnoresMouseEvents is a no-op on unsupported platforms
func (m *Manager) SetIgnoresMouseEvents(ignores bool) {}

// GetMouseLocation returns (0,0) on unsupported platforms
func (m *Manager) GetMouseLocation() (x, y float64) { return 0, 0 }

// GetScreenSize returns (0,0) on unsupported platforms
func (m *Manager) GetScreenSize() (w, h float64) { return 0, 0 }

// EvalJSWithResult is a no-op on unsupported platforms
func (m *Manager) EvalJSWithResult(js string) string { return "" }

// HintRect represents the bounding rectangle of an interactive element
type HintRect struct {
	ID          string
	X, Y, W, H float64
	Collapsed   bool
}

// UpdateHintRect is a no-op on unsupported platforms
func (m *Manager) UpdateHintRect(id string, rect HintRect) {}

// RemoveHintRect is a no-op on unsupported platforms
func (m *Manager) RemoveHintRect(id string) {}

// UpdateTextRect is a no-op on unsupported platforms
func (m *Manager) UpdateTextRect(id string, rect HintRect) {}

// RemoveTextRect is a no-op on unsupported platforms
func (m *Manager) RemoveTextRect(id string) {}

// SetOnAction is a no-op on unsupported platforms
func (m *Manager) SetOnAction(fn func(action string, actionType string, id string)) {}

// StartHintInteraction is a no-op on unsupported platforms
func (m *Manager) StartHintInteraction() {}

// StopHintInteraction is a no-op on unsupported platforms
func (m *Manager) StopHintInteraction() {}

// SyncHintRects is a no-op on unsupported platforms
func (m *Manager) SyncHintRects() {}

// GetWindowNumber returns 0 on unsupported platforms
func (m *Manager) GetWindowNumber() int { return 0 }

// DiagnosticCheck is a no-op on unsupported platforms
func (m *Manager) DiagnosticCheck() (visible bool, jsWorks bool, windowInfo string) {
	return false, false, "unsupported platform"
}
