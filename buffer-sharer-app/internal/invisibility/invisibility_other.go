//go:build !darwin

package invisibility

import "sync"

// Manager handles window invisibility (stub for non-macOS platforms)
type Manager struct {
	mu      sync.RWMutex
	enabled bool
}

// NewManager creates a new invisibility manager
func NewManager() *Manager {
	return &Manager{
		enabled: false,
	}
}

// SetEnabled enables or disables invisibility mode (no-op on this platform)
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// Toggle toggles invisibility mode and returns the new state
func (m *Manager) Toggle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = !m.enabled
	return m.enabled
}

// IsEnabled returns whether invisibility mode is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetWindowCount returns 0 on non-macOS platforms
func (m *Manager) GetWindowCount() int {
	return 0
}

// IsSupported returns false on non-macOS platforms
func (m *Manager) IsSupported() bool {
	return false
}
