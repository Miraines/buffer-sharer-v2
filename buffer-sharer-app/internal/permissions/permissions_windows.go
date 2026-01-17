//go:build windows

package permissions

import (
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1
)

// CheckScreenCapture checks if screen capture is available on Windows
// Windows usually doesn't require special permissions for screen capture
func (m *Manager) CheckScreenCapture() PermissionStatus {
	// Try to get screen metrics to verify display access
	width, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	height, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSCREEN))

	if width > 0 && height > 0 {
		return StatusGranted
	}
	return StatusDenied
}

// RequestScreenCapture - Windows usually doesn't need this
func (m *Manager) RequestScreenCapture() bool {
	// On Windows, screen capture typically works without special permissions
	return m.CheckScreenCapture() == StatusGranted
}

// CheckAccessibility checks if the app can simulate keyboard input on Windows
func (m *Manager) CheckAccessibility() PermissionStatus {
	// Windows doesn't have the same accessibility permission model as macOS
	// However, some antivirus software may block keyboard simulation
	// We can check by verifying SendInput is available

	sendInput := syscall.NewLazyDLL("user32.dll").NewProc("SendInput")
	if sendInput.Find() == nil {
		return StatusGranted
	}
	return StatusDenied
}

// RequestAccessibility - Windows usually doesn't need this
func (m *Manager) RequestAccessibility() bool {
	return m.CheckAccessibility() == StatusGranted
}

// OpenScreenCaptureSettings opens Windows display settings
func (m *Manager) OpenScreenCaptureSettings() {
	exec.Command("rundll32.exe", "shell32.dll,Control_RunDLL", "desk.cpl").Start()
}

// OpenAccessibilitySettings opens Windows ease of access settings
func (m *Manager) OpenAccessibilitySettings() {
	exec.Command("ms-settings:easeofaccess-keyboard").Start()
}

// RequestAllPermissions on Windows
func (m *Manager) RequestAllPermissions() map[PermissionType]bool {
	results := make(map[PermissionType]bool)

	// On Windows, permissions are usually granted by default
	results[PermissionScreenCapture] = m.CheckScreenCapture() == StatusGranted
	results[PermissionAccessibility] = m.CheckAccessibility() == StatusGranted

	// If running as non-admin and keyboard simulation fails,
	// we might need to suggest running as administrator
	if !results[PermissionAccessibility] {
		// Check if we're running as admin
		if !isAdmin() {
			// Could prompt to run as admin
		}
	}

	return results
}

// GetPlatform returns the current platform
func (m *Manager) GetPlatform() string {
	return "windows"
}

// LogDiagnostics logs permission diagnostic info
func (m *Manager) LogDiagnostics() {
	// Windows doesn't have the same permission complexity as macOS
}

// NeedsRestart returns true if app needs restart to apply permissions
func (m *Manager) NeedsRestart() bool {
	// Windows doesn't typically need restart for permissions
	return false
}

// isAdmin checks if the current process is running with admin privileges
func isAdmin() bool {
	_, err := exec.Command("net", "session").Output()
	return err == nil
}

// Helper to check if a pointer is valid (used internally)
func isValidPointer(ptr unsafe.Pointer) bool {
	return ptr != nil
}
