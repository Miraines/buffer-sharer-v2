//go:build windows

package invisibility

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32win                    = syscall.NewLazyDLL("user32.dll")
	procSetWindowDisplayAffinity = user32win.NewProc("SetWindowDisplayAffinity")
	procEnumWindows              = user32win.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32win.NewProc("GetWindowThreadProcessId")

	kernel32win          = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcessId = kernel32win.NewProc("GetCurrentProcessId")
)

const (
	WDA_NONE               = 0x00000000
	WDA_EXCLUDEFROMCAPTURE = 0x00000011 // Windows 10 2004+
)

// Manager handles window invisibility for screen sharing on Windows
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

// setAffinityForAllWindows sets display affinity for all windows of the current process
func setAffinityForAllWindows(affinity uint32) int {
	pid, _, _ := procGetCurrentProcessId.Call()
	currentPID := uint32(pid)

	count := 0

	// EnumWindows callback
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))

		if windowPID == currentPID {
			ret, _, _ := procSetWindowDisplayAffinity.Call(hwnd, uintptr(affinity))
			if ret != 0 {
				count++
			}
		}
		return 1 // Continue enumeration
	})

	procEnumWindows.Call(cb, 0)
	return count
}

// SetEnabled enables or disables invisibility mode
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enabled == enabled {
		return
	}

	m.enabled = enabled

	if enabled {
		setAffinityForAllWindows(WDA_EXCLUDEFROMCAPTURE)
	} else {
		setAffinityForAllWindows(WDA_NONE)
	}
}

// Toggle toggles invisibility mode and returns the new state
func (m *Manager) Toggle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = !m.enabled

	if m.enabled {
		setAffinityForAllWindows(WDA_EXCLUDEFROMCAPTURE)
	} else {
		setAffinityForAllWindows(WDA_NONE)
	}

	return m.enabled
}

// IsEnabled returns whether invisibility mode is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetWindowCount returns the number of application windows
func (m *Manager) GetWindowCount() int {
	pid, _, _ := procGetCurrentProcessId.Call()
	currentPID := uint32(pid)

	count := 0
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID == currentPID {
			count++
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return count
}

// SetExcludedWindowNumber is a no-op on Windows (overlay exclusion handled differently)
func (m *Manager) SetExcludedWindowNumber(wn int) {}

// IsSupported returns true on Windows 10 2004+
func (m *Manager) IsSupported() bool {
	// Check if SetWindowDisplayAffinity is available
	return procSetWindowDisplayAffinity.Find() == nil
}
