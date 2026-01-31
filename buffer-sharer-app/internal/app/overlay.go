package app

import (
	"fmt"

	"buffer-sharer-app/internal/network"
)

var hintCounter int64

// showOverlayToast shows a toast notification on the local overlay (no network)
func (a *App) showOverlayToast(text, toastType string) {
	if a.overlayManager == nil {
		a.log("warn", "[OVERLAY-TOAST] overlayManager is nil!")
		return
	}
	if text == "" {
		a.log("debug", "[OVERLAY-TOAST] skipping empty text")
		return
	}
	js := fmt.Sprintf(`showToast(%s, %s, 3000)`, jsString(text), jsString(toastType))
	a.log("debug", "[OVERLAY-TOAST] text=%s type=%s js=%s", text, toastType, js)
	a.overlayManager.EvalJS(js)
}

// SendNotification sends a notification to the connected peer's overlay
func (a *App) SendNotification(text, notifType string) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()

	if client == nil || !connected {
		a.log("debug", "[SEND] SendNotification: не подключен")
		return nil
	}
	if notifType == "" {
		notifType = "info"
	}
	a.log("debug", "[SEND] Отправка notification: text=%s type=%s", truncate(text, 40), notifType)
	return client.SendPayload(network.TypeNotification, &network.NotificationPayload{
		Text: text, Type: notifType, Duration: 3000,
	})
}

// --- Cursor methods ---

// SendCursorMove sends cursor position to the connected peer
func (a *App) SendCursorMove(x, y float64) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	return client.SendPayload(network.TypeCursorMove, &network.CursorPayload{X: x, Y: y})
}

// SendCursorShow shows cursor on the connected peer's overlay
func (a *App) SendCursorShow() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		a.log("debug", "[SEND] SendCursorShow: не подключен")
		return nil
	}
	a.log("info", "[SEND] Показываю курсор на клиенте")
	return client.SendPayload(network.TypeCursorShow, nil)
}

// SendCursorHide hides cursor on the connected peer's overlay
func (a *App) SendCursorHide() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.log("info", "[SEND] Скрываю курсор на клиенте")
	return client.SendPayload(network.TypeCursorHide, nil)
}

// SendCursorClick sends a click event to the connected peer's overlay
func (a *App) SendCursorClick(x, y float64) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.log("debug", "[SEND] Клик курсора: x=%.3f y=%.3f", x, y)
	return client.SendPayload(network.TypeCursorClick, &network.CursorPayload{X: x, Y: y})
}

// --- Hint methods ---

// SendHint sends a hint/tooltip to the connected peer's overlay
func (a *App) SendHint(x, y float64, text string, duration int) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		a.log("debug", "[SEND] SendHint: не подключен")
		return nil
	}
	hintCounter++
	id := fmt.Sprintf("hint_%d", hintCounter)
	payload := &network.HintPayload{
		ID: id, X: x, Y: y, Text: text, Duration: duration,
	}
	a.hintsMu.Lock()
	a.activeHints[id] = payload
	a.hintsMu.Unlock()
	a.log("info", "[SEND] Отправка подсказки: id=%s text=%s pos=(%.2f,%.2f) dur=%d", id, truncate(text, 30), x, y, duration)
	return client.SendPayload(network.TypeHintShow, payload)
}

// ClearHints removes all hints from the connected peer's overlay
func (a *App) ClearHints() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.hintsMu.Lock()
	a.activeHints = make(map[string]*network.HintPayload)
	a.hintsMu.Unlock()
	a.log("info", "[SEND] Очистка всех подсказок")
	return client.SendPayload(network.TypeHintClear, nil)
}

// --- Text overlay methods ---

// SendTextOverlay sends a text overlay to the connected peer's overlay
func (a *App) SendTextOverlay(x, y float64, text, color string) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.textOverlayCounter++
	id := fmt.Sprintf("text_%d", a.textOverlayCounter)
	payload := &network.TextOverlayPayload{
		ID: id, X: x, Y: y, Text: text, Color: color, Size: 24,
	}
	a.hintsMu.Lock()
	a.activeTextOverlays[id] = payload
	a.hintsMu.Unlock()
	a.log("info", "[SEND] Текстовая метка: id=%s text=%s pos=(%.2f,%.2f)", id, truncate(text, 30), x, y)
	return client.SendPayload(network.TypeTextOverlay, payload)
}

// ClearTextOverlays removes all text overlays from the connected peer's overlay
func (a *App) ClearTextOverlays() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.hintsMu.Lock()
	a.activeTextOverlays = make(map[string]*network.TextOverlayPayload)
	a.hintsMu.Unlock()
	a.log("info", "[SEND] Очистка всех текстовых меток")
	return client.SendPayload(network.TypeTextOverlayClear, nil)
}

// --- Draw methods ---

// SendDrawStart begins a new drawing stroke
func (a *App) SendDrawStart(x, y float64, color string, thickness float64, tool string) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		a.log("debug", "[SEND] SendDrawStart: не подключен")
		return nil
	}
	a.log("debug", "[SEND] Начало рисования: tool=%s color=%s thickness=%.4f", tool, color, thickness)
	return client.SendPayload(network.TypeDrawStart, &network.DrawStartPayload{
		X: x, Y: y, Color: color, Thickness: thickness, Tool: tool,
	})
}

// SendDrawMove continues a drawing stroke
func (a *App) SendDrawMove(x, y float64) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	return client.SendPayload(network.TypeDrawMove, &network.DrawMovePayload{X: x, Y: y})
}

// SendDrawEnd finishes a drawing stroke
func (a *App) SendDrawEnd(x, y float64) error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	return client.SendPayload(network.TypeDrawEnd, &network.DrawEndPayload{X: x, Y: y})
}

// SendDrawClear clears all drawings on the connected peer's overlay
func (a *App) SendDrawClear() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.log("info", "[SEND] Очистка всех рисунков на клиенте")
	return client.SendPayload(network.TypeDrawClear, nil)
}

// SendDrawUndo undoes the last drawing stroke
func (a *App) SendDrawUndo() error {
	a.mu.RLock()
	client := a.client
	connected := a.connected
	a.mu.RUnlock()
	if client == nil || !connected {
		return nil
	}
	a.log("debug", "[SEND] Отмена последнего штриха на клиенте")
	return client.SendPayload(network.TypeDrawUndo, nil)
}
