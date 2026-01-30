//go:build windows

package screenshot

import (
	"fmt"

	"buffer-sharer-app/internal/network"
)

// captureScreenFallback on Windows - not needed since GDI native capture always works
func captureScreenFallback(quality int) (*network.ScreenshotPayload, error) {
	return nil, fmt.Errorf("no fallback capture available on Windows")
}
