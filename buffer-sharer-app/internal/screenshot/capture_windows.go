//go:build windows

package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"syscall"
	"unsafe"
)

var (
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	usr32  = syscall.NewLazyDLL("user32.dll")

	procGetDesktopWindow    = usr32.NewProc("GetDesktopWindow")
	procGetDC               = usr32.NewProc("GetDC")
	procReleaseDC           = usr32.NewProc("ReleaseDC")
	procGetSystemMetrics    = usr32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procBitBlt              = gdi32.NewProc("BitBlt")
	procGetDIBits           = gdi32.NewProc("GetDIBits")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
)

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1
	SRCCOPY     = 0x00CC0020
	BI_RGB      = 0
	DIB_RGB_COLORS = 0
)

// BITMAPINFOHEADER for GetDIBits
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// BITMAPINFO for GetDIBits
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32 // RGBQUAD
}

// captureScreenNative captures the screen using Windows GDI
func captureScreenNative(quality int) ([]byte, int, int, error) {
	// Get screen dimensions
	width, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	height, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSCREEN))

	if width == 0 || height == 0 {
		return nil, 0, 0, fmt.Errorf("failed to get screen dimensions")
	}

	w := int(width)
	h := int(height)

	// Get desktop window and its DC
	hwnd, _, _ := procGetDesktopWindow.Call()
	hdc, _, _ := procGetDC.Call(hwnd)
	if hdc == 0 {
		return nil, 0, 0, fmt.Errorf("failed to get desktop DC")
	}
	defer procReleaseDC.Call(hwnd, hdc)

	// Create compatible DC and bitmap
	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	if memDC == 0 {
		return nil, 0, 0, fmt.Errorf("failed to create compatible DC")
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBmp.Call(hdc, width, height)
	if bitmap == 0 {
		return nil, 0, 0, fmt.Errorf("failed to create compatible bitmap")
	}
	defer procDeleteObject.Call(bitmap)

	// Select bitmap into memory DC
	procSelectObject.Call(memDC, bitmap)

	// BitBlt: copy screen to memory DC
	ret, _, _ := procBitBlt.Call(
		memDC, 0, 0, width, height,
		hdc, 0, 0,
		SRCCOPY,
	)
	if ret == 0 {
		return nil, 0, 0, fmt.Errorf("BitBlt failed")
	}

	// Prepare BITMAPINFO for GetDIBits
	bi := BITMAPINFO{}
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(w)
	bi.BmiHeader.BiHeight = -int32(h) // Negative = top-down DIB
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = BI_RGB

	// Allocate pixel buffer (BGRA, 4 bytes per pixel)
	pixelDataSize := w * h * 4
	pixelData := make([]byte, pixelDataSize)

	// Get the pixel data
	ret, _, _ = procGetDIBits.Call(
		memDC, bitmap,
		0, uintptr(h),
		uintptr(unsafe.Pointer(&pixelData[0])),
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
	)
	if ret == 0 {
		return nil, 0, 0, fmt.Errorf("GetDIBits failed")
	}

	// Convert BGRA to RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < pixelDataSize; i += 4 {
		img.Pix[i+0] = pixelData[i+2] // R <- B
		img.Pix[i+1] = pixelData[i+1] // G <- G
		img.Pix[i+2] = pixelData[i+0] // B <- R
		img.Pix[i+3] = 255            // A (opaque)
	}

	// Encode to JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("JPEG encode failed: %w", err)
	}

	return buf.Bytes(), w, h, nil
}

// hasScreenRecordingPermissionNative - Windows doesn't require screen recording permission
func hasScreenRecordingPermissionNative() bool {
	return true
}

// IsScreenCaptureKitAvailable returns false on Windows
func IsScreenCaptureKitAvailable() bool {
	return false
}
