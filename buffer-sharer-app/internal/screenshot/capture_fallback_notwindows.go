//go:build !windows

package screenshot

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"

	"github.com/go-vgo/robotgo"

	"buffer-sharer-app/internal/network"
)

// captureScreenFallback uses robotgo as a fallback capture method
func captureScreenFallback(quality int) (*network.ScreenshotPayload, error) {
	width, height := robotgo.GetScreenSize()
	if width == 0 || height == 0 {
		return nil, nil
	}

	bitmap := robotgo.CaptureScreen(0, 0, width, height)
	if bitmap == nil {
		return nil, nil
	}
	defer robotgo.FreeBitmap(bitmap)

	img := robotgo.ToImage(bitmap)
	if img == nil {
		return nil, nil
	}

	var rgbaImg *image.RGBA
	switch v := img.(type) {
	case *image.RGBA:
		rgbaImg = v
	default:
		bounds := img.Bounds()
		rgbaImg = image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, rgbaImg, opts); err != nil {
		return nil, err
	}

	return &network.ScreenshotPayload{
		Width:   rgbaImg.Bounds().Dx(),
		Height:  rgbaImg.Bounds().Dy(),
		Format:  "jpeg",
		Data:    buf.Bytes(),
		Quality: quality,
	}, nil
}
