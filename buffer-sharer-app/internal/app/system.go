package app

import (
	"os"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// RestartApp перезапускает приложение
func (a *App) RestartApp() {
	a.log("info", "Инициирован перезапуск приложения...")

	execPath, err := os.Executable()
	if err != nil {
		a.log("error", "Не удалось получить путь к исполняемому файлу: "+err.Error())
		return
	}

	a.log("info", "Путь к приложению: "+execPath)

	appBundlePath := execPath
	if strings.Contains(execPath, ".app/Contents/MacOS/") {
		idx := strings.Index(execPath, ".app/Contents/MacOS/")
		appBundlePath = execPath[:idx+4]
		a.log("info", "App bundle: "+appBundlePath)
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(appBundlePath, ".app") {
		cmd = exec.Command("open", "-n", appBundlePath)
	} else {
		cmd = exec.Command(execPath)
	}

	if err := cmd.Start(); err != nil {
		a.log("error", "Не удалось запустить новый экземпляр: "+err.Error())
		return
	}

	a.log("info", "Новый экземпляр запущен, завершаем текущий...")
	runtime.Quit(a.ctx)
}

// QuitApp завершает приложение
func (a *App) QuitApp() {
	a.log("info", "Завершение приложения...")
	runtime.Quit(a.ctx)
}

// GetAppExecutablePath возвращает путь к исполняемому файлу
func (a *App) GetAppExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "unknown: " + err.Error()
	}
	return execPath
}

// ToggleInvisibility toggles window invisibility mode and returns the new state
func (a *App) ToggleInvisibility() bool {
	if a.invisibilityManager == nil {
		a.log("error", "Invisibility manager not initialized")
		return false
	}

	newState := a.invisibilityManager.Toggle()

	if newState {
		a.log("info", "Режим невидимости ВКЛЮЧЁН - окно скрыто от захвата экрана")
		a.showOverlayToast("Невидимость ON — окно скрыто", "warning")
	} else {
		a.log("info", "Режим невидимости ВЫКЛЮЧЕН - окно видно при захвате экрана")
		a.showOverlayToast("Невидимость OFF", "info")
	}

	runtime.EventsEmit(a.ctx, "invisibilityChanged", newState)

	return newState
}

// SetInvisibility sets window invisibility mode explicitly
func (a *App) SetInvisibility(enabled bool) bool {
	if a.invisibilityManager == nil {
		return false
	}

	a.invisibilityManager.SetEnabled(enabled)

	if enabled {
		a.log("info", "Режим невидимости ВКЛЮЧЁН")
	} else {
		a.log("info", "Режим невидимости ВЫКЛЮЧЕН")
	}

	runtime.EventsEmit(a.ctx, "invisibilityChanged", enabled)
	return enabled
}

// GetInvisibilityStatus returns current invisibility status
func (a *App) GetInvisibilityStatus() map[string]interface{} {
	if a.invisibilityManager == nil {
		return map[string]interface{}{
			"enabled":     false,
			"supported":   false,
			"windowCount": 0,
		}
	}

	return map[string]interface{}{
		"enabled":     a.invisibilityManager.IsEnabled(),
		"supported":   a.invisibilityManager.IsSupported(),
		"windowCount": a.invisibilityManager.GetWindowCount(),
	}
}

// IsInvisibilitySupported returns whether invisibility mode is supported on this platform
func (a *App) IsInvisibilitySupported() bool {
	if a.invisibilityManager == nil {
		return false
	}
	return a.invisibilityManager.IsSupported()
}
