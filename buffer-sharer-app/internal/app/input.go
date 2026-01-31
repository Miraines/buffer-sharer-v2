package app

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/network"
)

// SendText sends text to the connected client
func (a *App) SendText(text string) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()

	if client == nil || !connected {
		return nil
	}

	payload := network.TextPayload{
		Text:      text,
		Immediate: false,
	}

	if err := client.SendPayload(network.TypeText, &payload); err != nil {
		a.log("error", "Не удалось отправить текст: "+err.Error())
		return err
	}

	a.incrementStat("textsSent", 0)
	a.incrementStat("bytesSent", int64(len(text)))
	a.addTextToHistory(text, "sent")

	a.log("info", "Текст отправлен: "+truncate(text, 50))
	return nil
}

// TypeBuffer types the current keyboard buffer content
func (a *App) TypeBuffer() {
	if a.keyInterceptor != nil {
		remaining := a.keyInterceptor.GetRemainingBuffer()
		if remaining != "" {
			a.log("info", "Печатаю текст из interceptor буфера: "+truncate(remaining, 30))
			a.keyInterceptor.TypeAllBuffer()
			return
		}
	}

	if a.keyboardHandler != nil {
		text := a.keyboardHandler.GetBuffer()
		if text != "" {
			a.log("info", "Печатаю текст из handler буфера: "+truncate(text, 30))
			if err := a.keyboardHandler.TypeBuffer(); err != nil {
				a.log("error", "Ошибка печати: "+err.Error())
			}
			return
		}
	}

	a.log("warn", "Буфер пуст")
}

// GetKeyboardBuffer returns the current keyboard buffer
func (a *App) GetKeyboardBuffer() string {
	if a.keyboardHandler == nil {
		return ""
	}
	return a.keyboardHandler.GetBuffer()
}

// ClearKeyboardBuffer clears the keyboard buffer
func (a *App) ClearKeyboardBuffer() {
	if a.keyboardHandler != nil {
		a.keyboardHandler.ClearBuffer()
		a.log("info", "Буфер очищен")
		a.showOverlayToast("Буфер очищен", "info")
	}
}

// ToggleInputMode toggles keyboard input mode (key interception)
func (a *App) ToggleInputMode() bool {
	a.log("debug", "[TOGGLE] ToggleInputMode() called")

	if a.keyInterceptor == nil {
		a.log("error", "[TOGGLE] keyInterceptor is nil!")
		return false
	}

	wasEnabled := a.keyInterceptor.IsEnabled()
	wasRunning := a.keyInterceptor.IsRunning()
	bufLen := a.keyInterceptor.GetBufferLength()
	bufPos := a.keyInterceptor.GetPosition()

	a.log("debug", "[TOGGLE] Current state: enabled=%v, running=%v, bufLen=%d, bufPos=%d", wasEnabled, wasRunning, bufLen, bufPos)

	desiredEnabled := !wasEnabled
	a.log("debug", "[TOGGLE] Setting enabled to %v...", desiredEnabled)
	a.keyInterceptor.SetEnabled(desiredEnabled)
	a.log("debug", "[TOGGLE] SetEnabled() completed")

	actualEnabled := a.keyInterceptor.IsEnabled()
	a.log("debug", "[TOGGLE] Actual enabled state: %v (desired was %v)", actualEnabled, desiredEnabled)

	if a.keyboardHandler != nil {
		a.log("debug", "[TOGGLE] Syncing with keyboardHandler...")
		a.keyboardHandler.SetInputMode(actualEnabled)
		a.log("debug", "[TOGGLE] keyboardHandler synced")
	}

	a.log("debug", "[TOGGLE] Emitting inputModeChanged event with actual state...")
	runtime.EventsEmit(a.ctx, "inputModeChanged", actualEnabled)
	a.log("debug", "[TOGGLE] Event emitted")

	if actualEnabled {
		remaining := a.keyInterceptor.GetRemainingBuffer()
		runeCount := len([]rune(remaining))
		if runeCount > 0 {
			a.log("info", "Режим ввода ВКЛЮЧЁН - нажимайте любые клавиши для ввода текста (%d символов в буфере)", runeCount)
		} else {
			a.log("info", "Режим ввода ВКЛЮЧЁН - но буфер пуст, дождитесь текста от контроллера")
		}
		a.showOverlayToast("Режим ввода ON", "success")
	} else {
		if desiredEnabled && !actualEnabled {
			a.log("warn", "Режим ввода НЕ ВКЛЮЧЁН - буфер пуст! Дождитесь текста от контроллера.")
		} else {
			a.log("info", "Режим ввода ВЫКЛЮЧЕН")
		}
		a.showOverlayToast("Режим ввода OFF", "info")
	}

	a.log("debug", "[TOGGLE] ToggleInputMode() returning %v", actualEnabled)
	return actualEnabled
}

// GetInputMode returns current input mode state
func (a *App) GetInputMode() bool {
	if a.keyInterceptor == nil {
		return false
	}
	return a.keyInterceptor.IsEnabled()
}

// SetInputMode sets input mode state explicitly
func (a *App) SetInputMode(enabled bool) bool {
	if a.keyInterceptor == nil {
		return false
	}
	a.keyInterceptor.SetEnabled(enabled)

	if a.keyboardHandler != nil {
		a.keyboardHandler.SetInputMode(enabled)
	}

	runtime.EventsEmit(a.ctx, "inputModeChanged", enabled)
	return enabled
}

// GetBufferStatus returns the current buffer status for UI
func (a *App) GetBufferStatus() map[string]interface{} {
	if a.keyInterceptor == nil {
		return map[string]interface{}{
			"length":    0,
			"position":  0,
			"remaining": 0,
			"text":      "",
		}
	}
	remaining := a.keyInterceptor.GetRemainingBuffer()
	return map[string]interface{}{
		"length":    a.keyInterceptor.GetBufferLength(),
		"position":  a.keyInterceptor.GetPosition(),
		"remaining": len(remaining),
		"text":      remaining,
	}
}

// emitEvent is a helper that safely emits events to the frontend
func (a *App) emitEvent(eventName string, data ...interface{}) {
	if len(data) > 0 {
		runtime.EventsEmit(a.ctx, eventName, data[0])
	} else {
		runtime.EventsEmit(a.ctx, eventName)
	}
}

