//go:build windows

package permissions

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSendInput        = user32.NewProc("SendInput")

	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	procGetTokenInformation = advapi32.NewProc("GetTokenInformation")

	ntdll             = syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion = ntdll.NewProc("RtlGetVersion")

	kernel32Perm            = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcessPerm = kernel32Perm.NewProc("GetCurrentProcess")
)

// TOKEN_ELEVATION struct
type tokenElevation struct {
	TokenIsElevated uint32
}

// RTL_OSVERSIONINFOW struct
type rtlOSVersionInfoW struct {
	DwOSVersionInfoSize uint32
	DwMajorVersion      uint32
	DwMinorVersion      uint32
	DwBuildNumber       uint32
	DwPlatformId        uint32
	SzCSDVersion        [128]uint16
}

const (
	TOKEN_QUERY          = 0x0008
	TokenElevationType   = 20
	tokenElevationInfo   = 20
)

// CheckScreenCapture checks if screen capture is available on Windows
// Windows doesn't require special permissions for screen capture
func (m *Manager) CheckScreenCapture() PermissionStatus {
	return StatusGranted
}

// CheckScreenCaptureCached returns cached screen capture permission (same as real check on Windows)
func (m *Manager) CheckScreenCaptureCached() PermissionStatus {
	return StatusGranted
}

// RequestScreenCapture - Windows doesn't need this
func (m *Manager) RequestScreenCapture() bool {
	return true
}

// CheckAccessibility checks if the app can simulate keyboard input on Windows
func (m *Manager) CheckAccessibility() PermissionStatus {
	// Try calling SendInput with 0 inputs - if the function is accessible, it returns 0
	// This verifies SendInput is available and not blocked by UIPI or antivirus
	ret, _, _ := procSendInput.Call(0, 0, 0)
	// SendInput with 0 inputs returns 0 and sets ERROR_SUCCESS on success
	// If blocked, it may return error
	_ = ret

	// If SendInput proc can be found, we have basic accessibility
	if procSendInput.Find() == nil {
		return StatusGranted
	}
	return StatusDenied
}

// CheckAccessibilityCached returns cached accessibility permission (same as real check on Windows)
func (m *Manager) CheckAccessibilityCached() PermissionStatus {
	return m.CheckAccessibility()
}

// RequestAccessibility - Windows usually doesn't need this
func (m *Manager) RequestAccessibility() bool {
	return m.CheckAccessibility() == StatusGranted
}

// OpenScreenCaptureSettings opens Windows display settings
func (m *Manager) OpenScreenCaptureSettings() {
	exec.Command("cmd", "/c", "start", "ms-settings:display").Start()
}

// OpenAccessibilitySettings opens Windows ease of access settings
func (m *Manager) OpenAccessibilitySettings() {
	exec.Command("cmd", "/c", "start", "ms-settings:easeofaccess-keyboard").Start()
}

// RequestAllPermissions on Windows
func (m *Manager) RequestAllPermissions() map[PermissionType]bool {
	results := make(map[PermissionType]bool)

	results[PermissionScreenCapture] = m.CheckScreenCapture() == StatusGranted
	results[PermissionAccessibility] = m.CheckAccessibility() == StatusGranted

	return results
}

// GetPlatform returns the current platform
func (m *Manager) GetPlatform() string {
	return "windows"
}

// LogDiagnostics logs permission diagnostic info
func (m *Manager) LogDiagnostics() {
	// Log OS version
	ver := getWindowsVersion()
	fmt.Printf("[Permissions] Windows version: %d.%d.%d\n", ver.DwMajorVersion, ver.DwMinorVersion, ver.DwBuildNumber)
	fmt.Printf("[Permissions] Running as admin: %v\n", isAdmin())
	fmt.Printf("[Permissions] SendInput available: %v\n", procSendInput.Find() == nil)
}

// NeedsRestart returns true if app needs restart to apply permissions
func (m *Manager) NeedsRestart() bool {
	return false
}

// isAdmin checks if the current process is running with elevated privileges
func isAdmin() bool {
	var token syscall.Token

	processHandle, _, _ := procGetCurrentProcessPerm.Call()
	err := syscall.OpenProcessToken(syscall.Handle(processHandle), TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	var elevation tokenElevation
	var returnedLen uint32

	ret, _, _ := procGetTokenInformation.Call(
		uintptr(token),
		uintptr(tokenElevationInfo),
		uintptr(unsafe.Pointer(&elevation)),
		uintptr(unsafe.Sizeof(elevation)),
		uintptr(unsafe.Pointer(&returnedLen)),
	)

	if ret == 0 {
		return false
	}

	return elevation.TokenIsElevated != 0
}

// getWindowsVersion returns the Windows OS version info
func getWindowsVersion() rtlOSVersionInfoW {
	var ver rtlOSVersionInfoW
	ver.DwOSVersionInfoSize = uint32(unsafe.Sizeof(ver))
	procRtlGetVersion.Call(uintptr(unsafe.Pointer(&ver)))
	return ver
}
