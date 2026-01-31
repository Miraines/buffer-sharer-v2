package app

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/network"
	"buffer-sharer-app/internal/screenshot"
)

// Connect connects to the middleware server
func (a *App) Connect(host string, port int, role, roomCode string) ConnectionStatus {
	a.mu.Lock()
	if a.connected {
		status := ConnectionStatus{
			Connected: true,
			RoomCode:  a.roomCode,
			Role:      a.role,
		}
		a.mu.Unlock()
		return status
	}
	if a.connecting {
		a.mu.Unlock()
		a.log("warn", "Подключение уже выполняется, игнорируем повторный вызов")
		return ConnectionStatus{
			Connected: false,
			Error:     "connection already in progress",
		}
	}
	a.connecting = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.connecting = false
		a.mu.Unlock()
	}()

	a.log("info", "Подключение к "+host+"...")

	client := network.NewClient(network.ClientConfig{
		Host:     host,
		Port:     port,
		Role:     role,
		RoomCode: roomCode,
	}, a.logger)

	client.SetOnMessage(func(msg *network.Message) {
		a.handleMessage(msg)
	})

	client.SetOnConnect(func() {
		a.log("info", "Соединение установлено")
		runtime.EventsEmit(a.ctx, "connected", nil)
		a.showOverlayToast("Соединение установлено", "success")
	})

	client.SetOnDisconnect(func(err error) {
		if err != nil {
			a.log("warn", "Соединение потеряно: "+err.Error())
			a.showOverlayToast("Соединение потеряно", "error")
		} else {
			a.log("info", "Соединение закрыто")
		}
		runtime.EventsEmit(a.ctx, "disconnected", nil)
	})

	client.SetOnRoomCreated(func(code string) {
		a.log("info", "Комната создана: "+code)
		runtime.EventsEmit(a.ctx, "roomCreated", code)
	})

	client.SetOnRoomJoined(func(code string) {
		a.log("info", "Подключено к комнате: "+code)
		runtime.EventsEmit(a.ctx, "roomJoined", code)
	})

	client.SetOnAuthError(func(errMsg string) {
		a.log("error", "Ошибка аутентификации: "+errMsg)
		runtime.EventsEmit(a.ctx, "authError", errMsg)
	})

	if err := client.Connect(); err != nil {
		a.log("error", "Не удалось подключиться: "+err.Error())
		return ConnectionStatus{
			Connected: false,
			Error:     err.Error(),
		}
	}

	connectedRoomCode := client.GetRoomCode()

	a.mu.Lock()
	a.client = client
	a.connected = true
	a.role = role
	a.roomCode = connectedRoomCode
	a.stats = Statistics{ConnectedAt: time.Now()}
	a.settings.LastRole = role
	a.settings.LastRoomCode = connectedRoomCode
	a.mu.Unlock()

	if a.logger != nil {
		a.logger.SetRole(role)
	}

	client.Start()

	if role == "client" {
		a.startScreenshotService()
	}

	a.saveConfig()

	a.log("info", "Успешно подключено!")
	runtime.EventsEmit(a.ctx, "connected", nil)

	return ConnectionStatus{
		Connected: true,
		RoomCode:  connectedRoomCode,
		Role:      role,
	}
}

// startScreenshotService initializes and starts screenshot capture
func (a *App) startScreenshotService() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.screenshotService != nil {
		a.screenshotService.Stop()
	}

	a.screenshotService = screenshot.NewCaptureService(screenshot.Config{
		IntervalMs: a.settings.ScreenshotInterval,
		Quality:    a.settings.ScreenshotQuality,
	}, a.logger)

	a.screenshotService.SetOnCapture(func(payload *network.ScreenshotPayload) {
		if payload == nil {
			return
		}
		a.mu.RLock()
		client := a.client
		connected := a.connected
		a.mu.RUnlock()

		if client != nil && connected {
			if err := client.SendPayload(network.TypeScreenshot, payload); err != nil {
				a.log("error", "Не удалось отправить скриншот: "+err.Error())
			} else {
				a.incrementStat("screenshotsSent", 0)
				a.incrementStat("bytesSent", int64(len(payload.Data)))
				a.log("debug", "Скриншот отправлен")
			}
		}
	})

	a.screenshotService.Start()
	a.log("info", "Захват экрана запущен")
}

// Disconnect disconnects from the middleware
func (a *App) Disconnect() {
	a.mu.Lock()

	if a.screenshotService != nil {
		a.screenshotService.Stop()
		a.screenshotService = nil
	}

	client := a.client
	a.client = nil
	a.connected = false
	a.roomCode = ""
	a.role = ""
	a.mu.Unlock()

	if client != nil {
		client.Stop()
	}

	a.log("info", "Отключено")
	runtime.EventsEmit(a.ctx, "disconnected", nil)
}

// GetConnectionStatus returns the current connection status
func (a *App) GetConnectionStatus() ConnectionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return ConnectionStatus{
		Connected: a.connected,
		RoomCode:  a.roomCode,
		Role:      a.role,
	}
}
