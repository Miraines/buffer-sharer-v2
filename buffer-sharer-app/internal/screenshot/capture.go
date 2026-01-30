package screenshot

import (
	"context"
	"image"
	"sync"
	"time"

	"buffer-sharer-app/internal/logging"
	"buffer-sharer-app/internal/network"
)

// ScreenshotHistoryEntry represents a screenshot in history
type ScreenshotHistoryEntry struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int       `json:"size"` // bytes
	Data      []byte    `json:"-"`    // not serialized to JSON
}

// CaptureService handles screenshot capture functionality
type CaptureService struct {
	intervalMs int
	quality    int
	logger     *logging.Logger

	mu         sync.RWMutex
	lastImage  *image.RGBA
	lastData   []byte
	running    bool
	cancelFunc context.CancelFunc

	// Screenshot history (for controller)
	history       []ScreenshotHistoryEntry
	historyMaxLen int
	nextID        int

	// Callbacks
	onCapture func(*network.ScreenshotPayload)
}

// Config holds screenshot capture configuration
type Config struct {
	IntervalMs    int
	Quality       int
	HistoryMaxLen int // Maximum screenshots in history (0 = default 50)
}

// NewCaptureService creates a new screenshot capture service
func NewCaptureService(cfg Config, logger *logging.Logger) *CaptureService {
	quality := cfg.Quality
	if quality <= 0 || quality > 100 {
		quality = 80
	}

	intervalMs := cfg.IntervalMs
	if intervalMs <= 0 {
		intervalMs = 4000
	}

	historyMaxLen := cfg.HistoryMaxLen
	if historyMaxLen <= 0 {
		historyMaxLen = 50 // Default: keep last 50 screenshots
	}

	return &CaptureService{
		intervalMs:    intervalMs,
		quality:       quality,
		logger:        logger,
		history:       make([]ScreenshotHistoryEntry, 0),
		historyMaxLen: historyMaxLen,
		nextID:        1,
	}
}

// SetOnCapture sets the callback for when a screenshot is captured
func (s *CaptureService) SetOnCapture(callback func(*network.ScreenshotPayload)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCapture = callback
}

// Start begins automatic screenshot capture
func (s *CaptureService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	// Log which capture API will be used
	if IsScreenCaptureKitAvailable() {
		s.logger.Info("screenshot", "Using ScreenCaptureKit API (macOS 14+)")
	} else {
		s.logger.Info("screenshot", "Using legacy CGWindowListCreateImage API")
	}

	go s.captureLoop(ctx)
	s.logger.Info("screenshot", "Screenshot capture started with interval %dms", s.intervalMs)
}

// Stop stops automatic screenshot capture
func (s *CaptureService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	s.logger.Info("screenshot", "Screenshot capture stopped")
}

// IsRunning returns whether the capture service is running
func (s *CaptureService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// CaptureNow captures a screenshot immediately
func (s *CaptureService) CaptureNow() (*network.ScreenshotPayload, error) {
	return s.capture()
}

// GetLastScreenshot returns the last captured screenshot
func (s *CaptureService) GetLastScreenshot() (*network.ScreenshotPayload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastImage == nil || s.lastData == nil {
		return nil, false
	}

	return &network.ScreenshotPayload{
		Width:   s.lastImage.Bounds().Dx(),
		Height:  s.lastImage.Bounds().Dy(),
		Format:  "jpeg",
		Data:    s.lastData,
		Quality: s.quality,
	}, true
}

// captureLoop runs the periodic capture loop
func (s *CaptureService) captureLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.intervalMs) * time.Millisecond)
	defer ticker.Stop()

	// Capture immediately on start
	s.captureAndNotify()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.captureAndNotify()
		}
	}
}

// captureAndNotify captures a screenshot and notifies the callback
func (s *CaptureService) captureAndNotify() {
	payload, err := s.capture()
	if err != nil {
		s.logger.Error("screenshot", "Failed to capture screenshot: %v", err)
		return
	}

	s.mu.RLock()
	callback := s.onCapture
	s.mu.RUnlock()

	if callback != nil {
		callback(payload)
	}
}

// capture performs the actual screenshot capture
func (s *CaptureService) capture() (*network.ScreenshotPayload, error) {
	// Try native capture first (macOS - uses CGWindowListCreateImage for all windows)
	data, width, height, err := captureScreenNative(s.quality)
	if err != nil {
		s.logger.Error("screenshot", "Native capture failed: %v", err)
	}

	if data != nil && len(data) > 0 {
		// Native capture succeeded
		s.mu.Lock()
		s.lastData = data
		s.lastImage = nil // Native capture doesn't provide RGBA image
		s.mu.Unlock()

		s.logger.Debug("screenshot", "Captured screenshot %dx%d (%d bytes) [native]",
			width, height, len(data))

		return &network.ScreenshotPayload{
			Width:   width,
			Height:  height,
			Format:  "jpeg",
			Data:    data,
			Quality: s.quality,
		}, nil
	}

	// Fallback to platform-specific capture (robotgo on macOS/Linux)
	s.logger.Debug("screenshot", "Using fallback capture method")

	payload, err := captureScreenFallback(s.quality)
	if err != nil {
		s.logger.Error("screenshot", "Fallback capture failed: %v", err)
		return nil, err
	}
	if payload == nil {
		s.logger.Warn("screenshot", "Fallback capture returned nil")
		return nil, nil
	}

	s.mu.Lock()
	s.lastData = payload.Data
	s.lastImage = nil
	s.mu.Unlock()

	s.logger.Debug("screenshot", "Captured screenshot %dx%d (%d bytes) [fallback]",
		payload.Width, payload.Height, len(payload.Data))

	return payload, nil
}

// SetInterval updates the capture interval
func (s *CaptureService) SetInterval(intervalMs int) {
	s.mu.Lock()
	s.intervalMs = intervalMs
	wasRunning := s.running
	s.mu.Unlock()

	// Restart if running to apply new interval
	if wasRunning {
		s.Stop()
		s.Start()
	}
}

// SetQuality updates the JPEG quality
func (s *CaptureService) SetQuality(quality int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if quality > 0 && quality <= 100 {
		s.quality = quality
	}
}

// SetHistoryMaxLen updates the maximum number of screenshots in history
func (s *CaptureService) SetHistoryMaxLen(maxLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxLen <= 0 {
		maxLen = 50 // Default
	}
	s.historyMaxLen = maxLen

	// Trim history if it exceeds the new limit
	for len(s.history) > s.historyMaxLen {
		s.history[0] = ScreenshotHistoryEntry{}
		copy(s.history, s.history[1:])
		s.history[len(s.history)-1] = ScreenshotHistoryEntry{}
		s.history = s.history[:len(s.history)-1]
	}

	if s.logger != nil {
		s.logger.Info("screenshot", "History limit set to %d", s.historyMaxLen)
	}
}

// AddToHistory adds a screenshot to history (called by controller when receiving)
func (s *CaptureService) AddToHistory(payload *network.ScreenshotPayload) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := ScreenshotHistoryEntry{
		ID:        s.nextID,
		Timestamp: time.Now(),
		Width:     payload.Width,
		Height:    payload.Height,
		Size:      len(payload.Data),
		Data:      payload.Data,
	}
	s.nextID++

	s.history = append(s.history, entry)

	// Limit history size to prevent memory leak
	if len(s.history) > s.historyMaxLen {
		// Clear the oldest entry completely to help GC
		s.history[0] = ScreenshotHistoryEntry{}
		// Use copy to shift elements and properly shrink the slice
		// This avoids leaving references in the underlying array
		copy(s.history, s.history[1:])
		s.history[len(s.history)-1] = ScreenshotHistoryEntry{} // Zero the last element
		s.history = s.history[:len(s.history)-1]
	}

	return entry.ID
}

// GetHistory returns metadata about all screenshots in history (without data)
func (s *CaptureService) GetHistory() []ScreenshotHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ScreenshotHistoryEntry, len(s.history))
	for i, entry := range s.history {
		result[i] = ScreenshotHistoryEntry{
			ID:        entry.ID,
			Timestamp: entry.Timestamp,
			Width:     entry.Width,
			Height:    entry.Height,
			Size:      entry.Size,
			// Data intentionally not copied
		}
	}
	return result
}

// GetHistoryScreenshot returns a specific screenshot from history by ID
func (s *CaptureService) GetHistoryScreenshot(id int) (*network.ScreenshotPayload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.history {
		if entry.ID == id {
			return &network.ScreenshotPayload{
				Width:   entry.Width,
				Height:  entry.Height,
				Format:  "jpeg",
				Data:    entry.Data,
				Quality: s.quality,
			}, true
		}
	}
	return nil, false
}

// ClearHistory clears screenshot history and frees memory
func (s *CaptureService) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Free memory
	for i := range s.history {
		s.history[i].Data = nil
	}
	s.history = make([]ScreenshotHistoryEntry, 0)
	s.logger.Info("screenshot", "History cleared")
}
