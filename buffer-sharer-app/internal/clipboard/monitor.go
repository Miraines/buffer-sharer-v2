package clipboard

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"sync"
	"time"

	"golang.design/x/clipboard"

	"buffer-sharer-app/internal/logging"
	"buffer-sharer-app/internal/network"
)

// Monitor watches for clipboard changes
type Monitor struct {
	logger       *logging.Logger
	intervalMs   int
	enabled      bool

	mu           sync.RWMutex
	lastText     string
	lastImageSum []byte
	running      bool
	cancelFunc   context.CancelFunc

	// Callbacks
	onTextChange  func(string)
	onImageChange func([]byte)
	onChange      func(*network.ClipboardPayload)
}

// Config holds clipboard monitor configuration
type Config struct {
	Enabled        bool
	SyncIntervalMs int
}

// NewMonitor creates a new clipboard monitor
func NewMonitor(cfg Config, logger *logging.Logger) *Monitor {
	intervalMs := cfg.SyncIntervalMs
	if intervalMs <= 0 {
		intervalMs = 1000
	}

	return &Monitor{
		logger:     logger,
		intervalMs: intervalMs,
		enabled:    cfg.Enabled,
	}
}

// Init initializes the clipboard (must be called from main thread)
func Init() error {
	return clipboard.Init()
}

// SetOnTextChange sets the callback for text clipboard changes
func (m *Monitor) SetOnTextChange(callback func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTextChange = callback
}

// SetOnImageChange sets the callback for image clipboard changes
func (m *Monitor) SetOnImageChange(callback func([]byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onImageChange = callback
}

// SetOnChange sets the unified callback for any clipboard change
func (m *Monitor) SetOnChange(callback func(*network.ClipboardPayload)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = callback
}

// Start begins monitoring the clipboard
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running || !m.enabled {
		m.mu.Unlock()
		return
	}
	m.running = true

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.mu.Unlock()

	go m.monitorLoop(ctx)
	m.logger.Info("clipboard", "Clipboard monitor started with interval %dms", m.intervalMs)
}

// Stop stops monitoring the clipboard
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	m.logger.Info("clipboard", "Clipboard monitor stopped")
}

// IsRunning returns whether the monitor is running
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// monitorLoop is the main monitoring loop
func (m *Monitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.intervalMs) * time.Millisecond)
	defer ticker.Stop()

	// Initialize with current clipboard content
	m.checkClipboard(false)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkClipboard(true)
		}
	}
}

// checkClipboard checks for clipboard changes
func (m *Monitor) checkClipboard(notify bool) {
	// Check text clipboard
	textData := clipboard.Read(clipboard.FmtText)
	if textData != nil {
		text := string(textData)
		m.mu.Lock()
		changed := text != m.lastText
		m.lastText = text
		m.mu.Unlock()

		if changed && notify && text != "" {
			m.logger.Debug("clipboard", "Text clipboard changed: %d chars", len(text))
			m.notifyTextChange(text)
		}
	}

	// Check image clipboard
	imageData := clipboard.Read(clipboard.FmtImage)
	if imageData != nil {
		// Simple hash comparison using first+last bytes and length
		sum := makeImageSum(imageData)
		m.mu.Lock()
		changed := !bytes.Equal(sum, m.lastImageSum)
		m.lastImageSum = sum
		m.mu.Unlock()

		if changed && notify {
			m.logger.Debug("clipboard", "Image clipboard changed: %d bytes", len(imageData))
			m.notifyImageChange(imageData)
		}
	}
}

// makeImageSum creates a simple checksum for image data comparison
func makeImageSum(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	// Simple sum: length + first 32 + last 32 bytes
	sum := make([]byte, 0, 72)

	// Add length as 8 bytes
	l := uint64(len(data))
	for i := 0; i < 8; i++ {
		sum = append(sum, byte(l>>(i*8)))
	}

	// Add first 32 bytes
	n := 32
	if len(data) < n {
		n = len(data)
	}
	sum = append(sum, data[:n]...)

	// Add last 32 bytes
	if len(data) > 32 {
		n = 32
		if len(data) < 64 {
			n = len(data) - 32
		}
		sum = append(sum, data[len(data)-n:]...)
	}

	return sum
}

// notifyTextChange notifies listeners of text clipboard change
func (m *Monitor) notifyTextChange(text string) {
	m.mu.RLock()
	textCallback := m.onTextChange
	changeCallback := m.onChange
	m.mu.RUnlock()

	if textCallback != nil {
		textCallback(text)
	}

	if changeCallback != nil {
		changeCallback(&network.ClipboardPayload{
			Text:   text,
			Format: "text",
		})
	}
}

// notifyImageChange notifies listeners of image clipboard change
func (m *Monitor) notifyImageChange(imageData []byte) {
	m.mu.RLock()
	imageCallback := m.onImageChange
	changeCallback := m.onChange
	m.mu.RUnlock()

	if imageCallback != nil {
		imageCallback(imageData)
	}

	if changeCallback != nil {
		changeCallback(&network.ClipboardPayload{
			ImageData: imageData,
			Format:    "image",
		})
	}
}

// GetText returns the current text clipboard content
func (m *Monitor) GetText() string {
	data := clipboard.Read(clipboard.FmtText)
	if data == nil {
		return ""
	}
	return string(data)
}

// SetText sets the text clipboard content
func (m *Monitor) SetText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
	m.logger.Debug("clipboard", "Set clipboard text: %d chars", len(text))
}

// GetImage returns the current image clipboard content as PNG data
func (m *Monitor) GetImage() []byte {
	return clipboard.Read(clipboard.FmtImage)
}

// SetImage sets the image clipboard content
func (m *Monitor) SetImage(img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtImage, buf.Bytes())
	m.logger.Debug("clipboard", "Set clipboard image")
	return nil
}

// SetImageData sets the image clipboard content from raw PNG data
func (m *Monitor) SetImageData(data []byte) {
	clipboard.Write(clipboard.FmtImage, data)
	m.logger.Debug("clipboard", "Set clipboard image: %d bytes", len(data))
}

// SetEnabled enables or disables the monitor
func (m *Monitor) SetEnabled(enabled bool) {
	m.mu.Lock()
	wasEnabled := m.enabled
	wasRunning := m.running
	m.enabled = enabled
	m.mu.Unlock()

	if enabled && !wasEnabled && !wasRunning {
		m.Start()
	} else if !enabled && wasRunning {
		m.Stop()
	}
}
