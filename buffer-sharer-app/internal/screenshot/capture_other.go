//go:build !darwin

package screenshot

// captureScreenNative is a stub for non-macOS platforms
// Returns nil to indicate native capture is not available
func captureScreenNative(quality int) ([]byte, int, int, error) {
	return nil, 0, 0, nil
}

// hasScreenRecordingPermissionNative stub for non-macOS
func hasScreenRecordingPermissionNative() bool {
	return true // Assume true on non-macOS
}

// IsScreenCaptureKitAvailable returns false on non-macOS platforms
func IsScreenCaptureKitAvailable() bool {
	return false
}
