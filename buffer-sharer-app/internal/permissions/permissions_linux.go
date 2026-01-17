//go:build linux

package permissions

import (
	"os"
	"os/exec"
)

// CheckScreenCapture checks if screen capture is available on Linux
func (m *Manager) CheckScreenCapture() PermissionStatus {
	// Check if X11 display is available
	display := os.Getenv("DISPLAY")
	if display == "" {
		// Check for Wayland
		waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
		if waylandDisplay == "" {
			return StatusDenied
		}
	}
	return StatusGranted
}

// RequestScreenCapture - Linux X11 usually doesn't need permissions
func (m *Manager) RequestScreenCapture() bool {
	return m.CheckScreenCapture() == StatusGranted
}

// CheckAccessibility checks if keyboard simulation is available on Linux
func (m *Manager) CheckAccessibility() PermissionStatus {
	// Check if xdotool or similar is available, or if we can access /dev/uinput
	// For X11, usually no special permissions needed
	display := os.Getenv("DISPLAY")
	if display != "" {
		return StatusGranted
	}

	// Check for uinput access (may need sudo)
	if _, err := os.Stat("/dev/uinput"); err == nil {
		return StatusGranted
	}

	return StatusDenied
}

// RequestAccessibility - Linux usually needs user to add to input group
func (m *Manager) RequestAccessibility() bool {
	return m.CheckAccessibility() == StatusGranted
}

// OpenScreenCaptureSettings opens display settings on Linux
func (m *Manager) OpenScreenCaptureSettings() {
	// Try common desktop settings
	cmds := [][]string{
		{"gnome-control-center", "display"},
		{"systemsettings5", "kcm_kscreen"},
		{"xfce4-display-settings"},
	}

	for _, cmd := range cmds {
		if _, err := exec.LookPath(cmd[0]); err == nil {
			exec.Command(cmd[0], cmd[1:]...).Start()
			return
		}
	}
}

// OpenAccessibilitySettings opens accessibility settings on Linux
func (m *Manager) OpenAccessibilitySettings() {
	cmds := [][]string{
		{"gnome-control-center", "universal-access"},
		{"systemsettings5", "kcm_access"},
	}

	for _, cmd := range cmds {
		if _, err := exec.LookPath(cmd[0]); err == nil {
			exec.Command(cmd[0], cmd[1:]...).Start()
			return
		}
	}
}

// RequestAllPermissions on Linux
func (m *Manager) RequestAllPermissions() map[PermissionType]bool {
	results := make(map[PermissionType]bool)

	results[PermissionScreenCapture] = m.CheckScreenCapture() == StatusGranted
	results[PermissionAccessibility] = m.CheckAccessibility() == StatusGranted

	return results
}

// GetPlatform returns the current platform
func (m *Manager) GetPlatform() string {
	return "linux"
}

// LogDiagnostics logs permission diagnostic info
func (m *Manager) LogDiagnostics() {
	// Linux doesn't have the same permission complexity as macOS
}

// NeedsRestart returns true if app needs restart to apply permissions
func (m *Manager) NeedsRestart() bool {
	// Linux doesn't typically need restart for permissions
	return false
}
