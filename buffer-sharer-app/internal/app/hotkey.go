package app

import (
	"strconv"

	"buffer-sharer-app/internal/hotkey"
	"buffer-sharer-app/internal/permissions"
)

// checkAndNotifyPermissions проверяет разрешения и отправляет событие на фронтенд
func (a *App) checkAndNotifyPermissions() {
	perms := a.permissionsManager.GetAllPermissions()

	for _, p := range perms {
		a.log("info", "Разрешение "+string(p.Type)+": "+string(p.Status))
	}

	missing := make([]permissions.PermissionInfo, 0)
	for _, p := range perms {
		if p.Required && p.Status != permissions.StatusGranted {
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		a.log("warn", "Недостающие разрешения: "+strconv.Itoa(len(missing)))
		a.emitEvent("permissionsRequired", map[string]interface{}{
			"permissions": perms,
			"missing":     missing,
			"platform":    a.permissionsManager.GetPlatform(),
		})
	} else {
		a.log("info", "Все разрешения получены")
	}
}

// initHotkeyManager initializes and starts the global hotkey manager
func (a *App) initHotkeyManager() {
	a.hotkeyManager = hotkey.NewManager(a.logger)

	a.hotkeyManager.RegisterHandler(hotkey.ActionToggleInputMode, func() {
		a.log("debug", "Hotkey triggered: toggle input mode")
		if !a.sendEvent(func() {
			a.ToggleInputMode()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionPasteFromBuffer, func() {
		a.log("debug", "Hotkey triggered: paste from buffer")
		if !a.sendEvent(func() {
			a.TypeBuffer()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionTakeScreenshot, func() {
		a.log("debug", "Hotkey triggered: take screenshot")
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionToggleInvisibility, func() {
		a.log("debug", "Hotkey triggered: toggle invisibility")
		if !a.sendEvent(func() {
			a.ToggleInvisibility()
		}) {
			a.log("warn", "Event channel full or closed, hotkey action skipped")
		}
	})

	a.hotkeyManager.RegisterHandler(hotkey.ActionToggleHints, func() {
		a.log("debug", "Hotkey triggered: toggle hints")
		if a.overlayManager != nil {
			a.overlayManager.EvalJS(`toggleAllHints()`)
			a.overlayManager.SyncHintRects()
		}
	})

	a.registerHotkeysFromSettings()

	a.hotkeyManager.StartAsync()
	a.log("info", "Hotkey manager started")

	if a.keyInterceptor != nil {
		a.log("info", "Starting key interceptor event tap for hotkeys...")
		if a.keyInterceptor.Start() {
			a.log("info", "Key interceptor event tap started (interception disabled, hotkeys active)")
		} else {
			a.log("warn", "Failed to start key interceptor - Accessibility permission may be required")
		}
	}
}

// registerHotkeysFromSettings registers all hotkeys from current settings
func (a *App) registerHotkeysFromSettings() {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()

	if settings.HotkeyToggle != "" {
		if a.keyInterceptor != nil {
			a.keyInterceptor.SetToggleHotkey(settings.HotkeyToggle)
			a.log("info", "Set toggle hotkey on key interceptor: %s", settings.HotkeyToggle)
		}
	}

	if settings.HotkeyPaste != "" {
		if a.keyInterceptor != nil {
			a.keyInterceptor.SetPasteHotkey(settings.HotkeyPaste)
			a.log("info", "Set paste hotkey on key interceptor: %s", settings.HotkeyPaste)
		}
	}

	if settings.HotkeyScreenshot != "" {
		if err := a.hotkeyManager.Register(hotkey.ActionTakeScreenshot, settings.HotkeyScreenshot); err != nil {
			a.log("error", "Failed to register screenshot hotkey '%s': %v", settings.HotkeyScreenshot, err)
		} else {
			a.log("info", "Registered hotkey for screenshot: %s", settings.HotkeyScreenshot)
		}
	}

	if settings.HotkeyInvisibility != "" {
		if err := a.hotkeyManager.Register(hotkey.ActionToggleInvisibility, settings.HotkeyInvisibility); err != nil {
			a.log("error", "Failed to register invisibility hotkey '%s': %v", settings.HotkeyInvisibility, err)
		} else {
			a.log("info", "Registered hotkey for invisibility: %s", settings.HotkeyInvisibility)
		}
	}

	if err := a.hotkeyManager.Register(hotkey.ActionToggleHints, "Ctrl+Shift+H"); err != nil {
		a.log("error", "Failed to register toggle hints hotkey: %v", err)
	} else {
		a.log("info", "Registered hotkey for toggle hints: Ctrl+Shift+H")
	}
}
