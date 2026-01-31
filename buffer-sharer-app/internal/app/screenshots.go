package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ScreenshotHistoryEntryJS represents a screenshot history entry for frontend
type ScreenshotHistoryEntryJS struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Size      int    `json:"size"`
}

// GetScreenshotHistory returns the screenshot history metadata
func (a *App) GetScreenshotHistory() []ScreenshotHistoryEntryJS {
	if a.screenshotHistory == nil {
		return []ScreenshotHistoryEntryJS{}
	}

	history := a.screenshotHistory.GetHistory()
	result := make([]ScreenshotHistoryEntryJS, len(history))
	for i, entry := range history {
		result[i] = ScreenshotHistoryEntryJS{
			ID:        entry.ID,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Width:     entry.Width,
			Height:    entry.Height,
			Size:      entry.Size,
		}
	}
	return result
}

// GetScreenshotByID returns a specific screenshot from history as base64
func (a *App) GetScreenshotByID(id int) map[string]interface{} {
	if a.screenshotHistory == nil {
		return nil
	}

	payload, ok := a.screenshotHistory.GetHistoryScreenshot(id)
	if !ok {
		return nil
	}

	b64 := base64.StdEncoding.EncodeToString(payload.Data)
	return map[string]interface{}{
		"id":     id,
		"data":   "data:image/jpeg;base64," + b64,
		"width":  payload.Width,
		"height": payload.Height,
	}
}

// ClearScreenshotHistory clears the screenshot history
func (a *App) ClearScreenshotHistory() {
	if a.screenshotHistory != nil {
		a.screenshotHistory.ClearHistory()
	}
	a.log("info", "История скриншотов очищена")
}

// SaveScreenshotToFile сохраняет скриншот в файл
func (a *App) SaveScreenshotToFile(base64Data string, filename string) (string, error) {
	a.log("info", "Сохранение скриншота: %s", filename)

	saveDir := a.settings.ScreenshotSaveDir
	if saveDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
		}
		saveDir = filepath.Join(homeDir, "Downloads")
	}

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать директорию: %w", err)
	}

	if filename == "" {
		filename = fmt.Sprintf("screenshot-%s.jpg", time.Now().Format("2006-01-02_15-04-05"))
	}

	fullPath := filepath.Join(saveDir, filename)

	data := base64Data
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования base64: %w", err)
	}

	if err := os.WriteFile(fullPath, decoded, 0644); err != nil {
		return "", fmt.Errorf("ошибка записи файла: %w", err)
	}

	a.log("info", "Скриншот сохранен: %s", fullPath)
	return fullPath, nil
}

// SelectScreenshotDirectory открывает диалог выбора директории
func (a *App) SelectScreenshotDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Выберите папку для сохранения скриншотов",
		DefaultDirectory:     a.settings.ScreenshotSaveDir,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}

	if dir != "" {
		a.settings.ScreenshotSaveDir = dir
		a.log("info", "Выбрана директория для скриншотов: %s", dir)
	}

	return dir, nil
}

// GetScreenshotSaveDir возвращает текущую директорию для сохранения
func (a *App) GetScreenshotSaveDir() string {
	if a.settings.ScreenshotSaveDir != "" {
		return a.settings.ScreenshotSaveDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "Downloads")
}

// SetScreenshotSaveDir устанавливает директорию для сохранения
func (a *App) SetScreenshotSaveDir(dir string) error {
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("не удалось создать директорию: %w", err)
		}
	}
	a.settings.ScreenshotSaveDir = dir
	a.log("info", "Директория для скриншотов установлена: %s", dir)
	return nil
}
