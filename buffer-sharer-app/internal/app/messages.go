package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/network"
)

// safeFloat replaces NaN/Infinity with 0 to prevent invalid JS output from %f formatting
func safeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// handleMessage handles incoming network messages
func (a *App) handleMessage(msg *network.Message) {
	switch msg.Type {
	case network.TypeScreenshot:
		var payload network.ScreenshotPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "Ошибка парсинга скриншота")
			return
		}

		a.incrementStat("screenshotsReceived", 0)
		a.incrementStat("bytesReceived", int64(len(payload.Data)))

		var historyID int
		if a.screenshotHistory != nil {
			historyID = a.screenshotHistory.AddToHistory(&payload)
		}

		b64 := base64.StdEncoding.EncodeToString(payload.Data)
		runtime.EventsEmit(a.ctx, "screenshot", map[string]interface{}{
			"id":        historyID,
			"data":      "data:image/jpeg;base64," + b64,
			"width":     payload.Width,
			"height":    payload.Height,
			"timestamp": time.Now().Format(time.RFC3339),
		})

	case network.TypeText:
		a.log("debug", "[TEXT] Received TypeText message, parsing payload...")
		var payload network.TextPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[TEXT] Ошибка парсинга текста: %v", err)
			return
		}
		a.log("debug", "[TEXT] Payload parsed successfully, text length=%d bytes, runes=%d", len(payload.Text), len([]rune(payload.Text)))

		a.incrementStat("textsReceived", 0)
		a.incrementStat("bytesReceived", int64(len(payload.Text)))
		a.addTextToHistory(payload.Text, "received")
		a.log("debug", "[TEXT] Stats and history updated")

		if a.keyboardHandler != nil {
			a.log("debug", "[TEXT] Setting text in keyboardHandler...")
			a.keyboardHandler.SetText(payload.Text)
			a.log("debug", "[TEXT] keyboardHandler.SetText() completed")
		} else {
			a.log("warn", "[TEXT] keyboardHandler is nil!")
		}

		if a.keyInterceptor != nil {
			a.log("debug", "[TEXT] Setting text in keyInterceptor buffer...")
			a.keyInterceptor.SetBuffer(payload.Text)
			a.log("info", "Текст загружен в буфер. Включите режим ввода и нажимайте любые клавиши.")
			a.log("debug", "[TEXT] keyInterceptor.SetBuffer() completed, buffer length=%d", a.keyInterceptor.GetBufferLength())
		} else {
			a.log("warn", "[TEXT] keyInterceptor is nil!")
		}

		a.log("debug", "[TEXT] Emitting textReceived event to frontend...")
		runtime.EventsEmit(a.ctx, "textReceived", payload.Text)
		a.log("debug", "[TEXT] textReceived event emitted")
		a.log("info", "Получен текст: "+truncate(payload.Text, 50))
		runeCount := len([]rune(payload.Text))
		a.showOverlayToast(fmt.Sprintf("Новый текст: %d символов", runeCount), "info")

	case network.TypeClipboard:
		var payload network.ClipboardPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		runtime.EventsEmit(a.ctx, "clipboardReceived", payload.Text)

	case network.TypeNotification:
		var payload network.NotificationPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга notification: %v", err)
			return
		}
		dur := payload.Duration
		if dur == 0 {
			dur = 3000
		}
		a.log("debug", "[OVERLAY] Показываю toast: text=%s type=%s dur=%d", truncate(payload.Text, 40), payload.Type, dur)
		a.overlayManager.EvalJS(fmt.Sprintf(`showToast(%s, %s, %d)`,
			jsString(payload.Text), jsString(payload.Type), dur))

	case network.TypeCursorMove:
		var payload network.CursorPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга cursor_move: %v", err)
			return
		}
		if a.overlayManager == nil {
			a.log("error", "[OVERLAY] overlayManager is nil for cursor_move!")
			return
		}
		a.overlayManager.EvalJS(fmt.Sprintf(`moveCursor(%f, %f)`, safeFloat(payload.X), safeFloat(payload.Y)))

	case network.TypeCursorClick:
		var payload network.CursorPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга cursor_click: %v", err)
			return
		}
		a.log("info", "[OVERLAY] Клик курсора: x=%.3f y=%.3f overlayNil=%v", payload.X, payload.Y, a.overlayManager == nil)
		a.overlayManager.EvalJS(fmt.Sprintf(`clickCursor(%f, %f)`, safeFloat(payload.X), safeFloat(payload.Y)))

	case network.TypeCursorShow:
		a.log("info", "[OVERLAY] Курсор контроллера активирован, overlayNil=%v", a.overlayManager == nil)
		a.overlayManager.EvalJS(`showCursor()`)
		a.showOverlayToast("Курсор контроллера активен", "info")

	case network.TypeCursorHide:
		a.log("info", "[OVERLAY] Курсор контроллера скрыт")
		a.overlayManager.EvalJS(`hideCursor()`)

	case network.TypeHintShow:
		var payload network.HintPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга hint_show: %v", err)
			return
		}
		js := fmt.Sprintf(`showHint(%s, %f, %f, %s, %d)`,
			jsString(payload.ID), safeFloat(payload.X), safeFloat(payload.Y), jsString(payload.Text), payload.Duration)
		a.log("info", "[OVERLAY] Подсказка: id=%s text=%s pos=(%.2f,%.2f) js=%s", payload.ID, truncate(payload.Text, 30), payload.X, payload.Y, js)
		a.overlayManager.EvalJS(js)
		a.overlayManager.SyncHintRects()

	case network.TypeHintHide:
		var payload network.HintPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга hint_hide: %v", err)
			return
		}
		a.log("debug", "[OVERLAY] Скрытие подсказки: id=%s", payload.ID)
		a.overlayManager.EvalJS(fmt.Sprintf(`hideHint(%s)`, jsString(payload.ID)))

	case network.TypeHintClear:
		a.log("info", "[OVERLAY] Очистка всех подсказок")
		a.overlayManager.EvalJS(`clearHints()`)

	case network.TypeTextOverlay:
		var payload network.TextOverlayPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга text_overlay: %v", err)
			return
		}
		a.log("info", "[OVERLAY] Текстовая метка: id=%s text=%s pos=(%.2f,%.2f)", payload.ID, truncate(payload.Text, 30), payload.X, payload.Y)
		a.overlayManager.EvalJS(fmt.Sprintf(`showTextOverlay(%s, %f, %f, %s, %s, %f)`,
			jsString(payload.ID), safeFloat(payload.X), safeFloat(payload.Y), jsString(payload.Text), jsString(payload.Color), safeFloat(payload.Size)))
		a.overlayManager.SyncHintRects()

	case network.TypeTextOverlayClear:
		a.log("info", "[OVERLAY] Очистка всех текстовых меток")
		a.overlayManager.EvalJS(`clearTextOverlays()`)

	case network.TypeHintCollapse:
		var payload network.HintPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		a.log("debug", "[OVERLAY] Сворачивание подсказки: id=%s", payload.ID)
		a.overlayManager.EvalJS(fmt.Sprintf(`collapseHint(%s)`, jsString(payload.ID)))
		a.overlayManager.SyncHintRects()

	case network.TypeHintExpand:
		var payload network.HintPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		a.log("debug", "[OVERLAY] Разворачивание подсказки: id=%s", payload.ID)
		a.overlayManager.EvalJS(fmt.Sprintf(`expandHint(%s)`, jsString(payload.ID)))
		a.overlayManager.SyncHintRects()

	case network.TypeHintDelete:
		var payload network.HintPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		a.log("debug", "[OVERLAY] Удаление подсказки: id=%s", payload.ID)
		a.overlayManager.EvalJS(fmt.Sprintf(`hideHint(%s)`, jsString(payload.ID)))
		a.overlayManager.RemoveHintRect(payload.ID)

	case network.TypeTextOverlayDel:
		var payload network.TextOverlayPayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		a.log("debug", "[OVERLAY] Удаление текстовой метки: id=%s", payload.ID)
		a.overlayManager.EvalJS(fmt.Sprintf(`hideTextOverlay(%s)`, jsString(payload.ID)))
		a.overlayManager.RemoveTextRect(payload.ID)

	case network.TypeDrawStart:
		var payload network.DrawStartPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга draw_start: %v", err)
			return
		}
		js := fmt.Sprintf(`drawStart(%f, %f, %s, %f, %s)`,
			safeFloat(payload.X), safeFloat(payload.Y), jsString(payload.Color), safeFloat(payload.Thickness), jsString(payload.Tool))
		a.log("info", "[OVERLAY] Начало рисования: tool=%s color=%s thickness=%.4f pos=(%.3f,%.3f) js=%s", payload.Tool, payload.Color, payload.Thickness, payload.X, payload.Y, js)
		a.overlayManager.EvalJS(js)
		a.showOverlayToast("Рисование на вашем экране", "info")

	case network.TypeDrawMove:
		var payload network.DrawMovePayload
		if err := msg.ParsePayload(&payload); err != nil {
			return
		}
		a.overlayManager.EvalJS(fmt.Sprintf(`drawMove(%f, %f)`, safeFloat(payload.X), safeFloat(payload.Y)))

	case network.TypeDrawEnd:
		var payload network.DrawEndPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log("error", "[OVERLAY] Ошибка парсинга draw_end: %v", err)
			return
		}
		a.log("info", "[OVERLAY] Конец рисования: pos=(%.3f,%.3f)", payload.X, payload.Y)
		a.overlayManager.EvalJS(fmt.Sprintf(`drawEnd(%f, %f)`, safeFloat(payload.X), safeFloat(payload.Y)))

	case network.TypeDrawClear:
		a.log("info", "[OVERLAY] Очистка всех рисунков")
		a.overlayManager.EvalJS(`drawClear()`)

	case network.TypeDrawUndo:
		a.log("info", "[OVERLAY] Отмена последнего штриха")
		a.overlayManager.EvalJS(`drawUndo()`)
	}
}

// jsString escapes a Go string for safe embedding in JavaScript code
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
