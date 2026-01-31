package app

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"buffer-sharer-app/internal/permissions"
)

// PermissionInfoJS для фронтенда
type PermissionInfoJS struct {
	Type        string `json:"type"`
	Status      string `json:"status"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// GetPermissions возвращает статус всех разрешений
func (a *App) GetPermissions() []PermissionInfoJS {
	if a.permissionsManager == nil {
		return nil
	}

	perms := a.permissionsManager.GetAllPermissions()
	result := make([]PermissionInfoJS, len(perms))
	for i, p := range perms {
		result[i] = PermissionInfoJS{
			Type:        string(p.Type),
			Status:      string(p.Status),
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
		}
	}
	return result
}

// CheckPermissions проверяет все разрешения и возвращает результат
func (a *App) CheckPermissions() map[string]interface{} {
	if a.permissionsManager == nil {
		return map[string]interface{}{
			"allGranted": true,
			"platform":   "unknown",
		}
	}

	perms := a.permissionsManager.GetAllPermissions()
	allGranted := true
	missing := make([]string, 0)

	for _, p := range perms {
		if p.Required && p.Status != permissions.StatusGranted {
			allGranted = false
			missing = append(missing, string(p.Type))
		}
	}

	return map[string]interface{}{
		"allGranted":  allGranted,
		"missing":     missing,
		"platform":    a.permissionsManager.GetPlatform(),
		"permissions": a.GetPermissions(),
	}
}

// RequestPermission запрашивает конкретное разрешение
func (a *App) RequestPermission(permType string) bool {
	if a.permissionsManager == nil {
		return false
	}

	switch permissions.PermissionType(permType) {
	case permissions.PermissionScreenCapture:
		return a.permissionsManager.RequestScreenCapture()
	case permissions.PermissionAccessibility:
		return a.permissionsManager.RequestAccessibility()
	default:
		return false
	}
}

// RequestAllPermissions запрашивает все необходимые разрешения
func (a *App) RequestAllPermissions() map[string]bool {
	if a.permissionsManager == nil {
		return nil
	}

	results := a.permissionsManager.RequestAllPermissions()
	jsResults := make(map[string]bool)
	for k, v := range results {
		jsResults[string(k)] = v
	}

	return jsResults
}

// RecheckPermissions повторно проверяет разрешения и возвращает результат
func (a *App) RecheckPermissions() map[string]interface{} {
	a.log("info", "Перепроверка разрешений...")

	result := a.CheckPermissions()

	if perms, ok := result["permissions"].([]PermissionInfoJS); ok {
		for _, p := range perms {
			a.log("info", "Разрешение %s: %s", p.Type, p.Status)
		}
	}

	if allGranted, ok := result["allGranted"].(bool); ok && allGranted {
		a.log("info", "Все разрешения успешно получены!")
		a.restartKeyInterceptorIfNeeded()
	} else {
		if missing, ok := result["missing"].([]string); ok {
			a.log("warn", "Недостающие разрешения: %v", missing)
		}
	}

	return result
}

// restartKeyInterceptorIfNeeded перезапускает перехватчик клавиш если он не работает
func (a *App) restartKeyInterceptorIfNeeded() {
	if a.keyInterceptor == nil {
		return
	}

	if !a.keyInterceptor.IsRunning() {
		a.log("info", "Попытка перезапустить перехватчик клавиш...")
		if a.keyInterceptor.Start() {
			a.log("info", "Перехватчик клавиш успешно запущен!")
		} else {
			a.log("warn", "Не удалось запустить перехватчик клавиш - возможно требуется перезапуск приложения")
		}
	}
}

// OpenPermissionSettings открывает настройки для конкретного разрешения
func (a *App) OpenPermissionSettings(permType string) {
	if a.permissionsManager == nil {
		return
	}

	switch permissions.PermissionType(permType) {
	case permissions.PermissionScreenCapture:
		a.permissionsManager.OpenScreenCaptureSettings()
		a.log("info", "Открыты настройки записи экрана")
	case permissions.PermissionAccessibility:
		a.permissionsManager.OpenAccessibilitySettings()
		a.log("info", "Открыты настройки универсального доступа")
	}
}

// GetPlatform возвращает текущую платформу
func (a *App) GetPlatform() string {
	if a.permissionsManager == nil {
		return "unknown"
	}
	return a.permissionsManager.GetPlatform()
}

// StartPermissionPolling запускает периодическую проверку разрешений
func (a *App) StartPermissionPolling(intervalMs int) {
	if intervalMs < 500 {
		intervalMs = 500
	}
	if intervalMs > 10000 {
		intervalMs = 10000
	}

	a.permissionPollingMu.Lock()
	if a.permissionPollingCancel != nil {
		a.permissionPollingCancel()
		a.permissionPollingCancel = nil
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.permissionPollingCancel = cancel
	a.permissionPollingMu.Unlock()

	a.log("info", "Запуск мониторинга разрешений (интервал %d мс)...", intervalMs)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		prevStatus := make(map[permissions.PermissionType]permissions.PermissionStatus)

		for {
			select {
			case <-ctx.Done():
				a.log("debug", "Permission polling stopped")
				return
			case <-ticker.C:
				if a.permissionsManager == nil {
					continue
				}

				var perms []permissions.PermissionInfo
				if a.keyInterceptor != nil && a.keyInterceptor.IsRunning() {
					perms = a.permissionsManager.GetAllPermissionsCached()
				} else {
					perms = a.permissionsManager.GetAllPermissions()
				}
				changed := false

				for _, p := range perms {
					if prev, ok := prevStatus[p.Type]; ok {
						if prev != p.Status {
							a.log("info", "Статус разрешения %s изменился: %s -> %s", p.Type, prev, p.Status)
							changed = true
						}
					}
					prevStatus[p.Type] = p.Status
				}

				if changed {
					runtime.EventsEmit(a.ctx, "permissionsChanged", map[string]interface{}{
						"permissions": a.GetPermissions(),
						"allGranted":  a.permissionsManager.HasAllRequired(),
					})

					if a.permissionsManager.HasAllRequired() {
						a.log("info", "Все разрешения получены! Пробуем инициализировать сервисы...")
						a.restartKeyInterceptorIfNeeded()
					}
				}
			}
		}
	}()
}

// GetDetailedPermissionStatus возвращает детальный статус разрешений
func (a *App) GetDetailedPermissionStatus() map[string]interface{} {
	if a.permissionsManager == nil {
		return map[string]interface{}{
			"error": "permissions manager not initialized",
		}
	}

	a.permissionsManager.LogDiagnostics()

	perms := a.permissionsManager.GetAllPermissions()
	permDetails := make([]map[string]interface{}, len(perms))

	for i, p := range perms {
		permDetails[i] = map[string]interface{}{
			"type":        string(p.Type),
			"status":      string(p.Status),
			"name":        p.Name,
			"description": p.Description,
			"required":    p.Required,
			"granted":     p.Status == permissions.StatusGranted,
		}
	}

	keyInterceptorRunning := false
	if a.keyInterceptor != nil {
		keyInterceptorRunning = a.keyInterceptor.IsRunning()
	}

	needsRestart := a.permissionsManager.NeedsRestart()

	if !needsRestart && a.permissionsManager.HasAllRequired() && !keyInterceptorRunning {
		needsRestart = true
	}

	return map[string]interface{}{
		"platform":              a.permissionsManager.GetPlatform(),
		"permissions":           permDetails,
		"allGranted":            a.permissionsManager.HasAllRequired(),
		"keyInterceptorRunning": keyInterceptorRunning,
		"needsRestart":          needsRestart,
		"executablePath":        a.GetAppExecutablePath(),
	}
}
