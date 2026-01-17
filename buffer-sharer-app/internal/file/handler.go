package file

import (
	"io"
	"mime"
	"os"
	"path/filepath"
	"sync"

	"buffer-sharer-app/internal/logging"
	"buffer-sharer-app/internal/network"
)

// Handler manages file operations
type Handler struct {
	logger     *logging.Logger
	mu         sync.RWMutex
	saveDir    string
	maxSize    int64 // Maximum file size in bytes
}

// Config holds file handler configuration
type Config struct {
	SaveDirectory string
	MaxSizeMB     int
}

// NewHandler creates a new file handler
func NewHandler(cfg Config, logger *logging.Logger) (*Handler, error) {
	saveDir := cfg.SaveDirectory
	if saveDir == "" {
		// Default to user's Downloads folder
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		saveDir = filepath.Join(homeDir, "Downloads", "buffer-sharer")
	}

	// Create save directory if it doesn't exist
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return nil, err
	}

	maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // Default 100MB
	}

	return &Handler{
		logger:  logger,
		saveDir: saveDir,
		maxSize: maxSize,
	}, nil
}

// ReadFile reads a file and returns it as a FilePayload
func (h *Handler) ReadFile(path string) (*network.FilePayload, error) {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Check size limit
	if info.Size() > h.maxSize {
		h.logger.Warn("file", "File too large: %s (%d bytes)", path, info.Size())
		return nil, ErrFileTooLarge
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Determine MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	h.logger.Info("file", "Read file: %s (%d bytes)", path, len(data))

	return &network.FilePayload{
		Filename: filepath.Base(path),
		Size:     info.Size(),
		Data:     data,
		MimeType: mimeType,
	}, nil
}

// SaveFile saves a FilePayload to disk
func (h *Handler) SaveFile(payload *network.FilePayload) (string, error) {
	if payload == nil {
		return "", ErrInvalidPayload
	}

	// Sanitize filename
	filename := sanitizeFilename(payload.Filename)
	if filename == "" {
		filename = "unnamed_file"
	}

	// Generate unique path if file exists
	savePath := h.generateUniquePath(filename)

	// Write file
	if err := os.WriteFile(savePath, payload.Data, 0644); err != nil {
		return "", err
	}

	h.logger.Info("file", "Saved file: %s (%d bytes)", savePath, len(payload.Data))

	return savePath, nil
}

// SaveFileStream saves a file from a reader
func (h *Handler) SaveFileStream(filename string, reader io.Reader, size int64) (string, error) {
	// Check size limit
	if size > h.maxSize {
		return "", ErrFileTooLarge
	}

	// Sanitize filename
	filename = sanitizeFilename(filename)
	if filename == "" {
		filename = "unnamed_file"
	}

	// Generate unique path if file exists
	savePath := h.generateUniquePath(filename)

	// Create file
	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Copy data with size limit
	written, err := io.CopyN(file, reader, h.maxSize)
	if err != nil && err != io.EOF {
		os.Remove(savePath)
		return "", err
	}

	h.logger.Info("file", "Saved file: %s (%d bytes)", savePath, written)

	return savePath, nil
}

// generateUniquePath generates a unique file path in the save directory
func (h *Handler) generateUniquePath(filename string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	basePath := filepath.Join(h.saveDir, filename)

	// If file doesn't exist, use the original path
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return basePath
	}

	// Generate unique name with counter
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]

	for i := 1; ; i++ {
		newName := filepath.Join(h.saveDir, name+"_"+itoa(i)+ext)
		if _, err := os.Stat(newName); os.IsNotExist(err) {
			return newName
		}
	}
}

// GetSaveDirectory returns the current save directory
func (h *Handler) GetSaveDirectory() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.saveDir
}

// SetSaveDirectory updates the save directory
func (h *Handler) SetSaveDirectory(dir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	h.saveDir = dir
	h.logger.Info("file", "Save directory updated: %s", dir)
	return nil
}

// sanitizeFilename removes potentially dangerous characters from a filename
func sanitizeFilename(name string) string {
	// Get just the base name (remove any path components)
	name = filepath.Base(name)

	// Remove null bytes and other dangerous characters
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		// Allow alphanumeric, dot, dash, underscore, space
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == '_' || c == ' ' {
			result = append(result, c)
		}
	}

	return string(result)
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var result []byte
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	return string(result)
}

// Errors
var (
	ErrFileTooLarge   = &FileError{"file too large"}
	ErrInvalidPayload = &FileError{"invalid payload"}
)

// FileError represents a file operation error
type FileError struct {
	message string
}

func (e *FileError) Error() string {
	return e.message
}
