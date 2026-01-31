package app

import "time"

// GetStatistics возвращает текущую статистику сессии
func (a *App) GetStatistics() Statistics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := a.stats
	if a.connected && !stats.ConnectedAt.IsZero() {
		stats.TotalConnectTime = int64(time.Since(stats.ConnectedAt).Seconds())
	}
	return stats
}

// ResetStatistics сбрасывает статистику
func (a *App) ResetStatistics() {
	a.mu.Lock()
	a.stats = Statistics{}
	a.mu.Unlock()
	a.log("info", "Статистика сброшена")
}

// GetTextHistory возвращает историю текстов
func (a *App) GetTextHistory() []TextHistoryEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]TextHistoryEntry, len(a.textHistory))
	copy(result, a.textHistory)
	return result
}

// addTextToHistory добавляет текст в историю (внутренний метод)
func (a *App) addTextToHistory(text, direction string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := TextHistoryEntry{
		Text:      text,
		Direction: direction,
		Timestamp: time.Now(),
	}
	a.textHistory = append(a.textHistory, entry)

	if len(a.textHistory) > 50 {
		a.textHistory = a.textHistory[len(a.textHistory)-50:]
	}
}

// ClearTextHistory очищает историю текстов
func (a *App) ClearTextHistory() {
	a.mu.Lock()
	a.textHistory = make([]TextHistoryEntry, 0)
	a.mu.Unlock()
	a.log("info", "История текстов очищена")
}

// incrementStat увеличивает счётчик статистики (внутренний метод)
func (a *App) incrementStat(stat string, value int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch stat {
	case "screenshotsSent":
		a.stats.ScreenshotsSent++
	case "screenshotsReceived":
		a.stats.ScreenshotsReceived++
	case "textsSent":
		a.stats.TextsSent++
	case "textsReceived":
		a.stats.TextsReceived++
	case "bytesSent":
		a.stats.BytesSent += value
	case "bytesReceived":
		a.stats.BytesReceived += value
	}
}
