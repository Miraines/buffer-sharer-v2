package permissions

// PermissionStatus represents the status of a permission
type PermissionStatus string

const (
	StatusGranted    PermissionStatus = "granted"
	StatusDenied     PermissionStatus = "denied"
	StatusNotAsked   PermissionStatus = "not_asked"
	StatusRestricted PermissionStatus = "restricted"
	StatusUnknown    PermissionStatus = "unknown"
)

// PermissionType represents the type of permission
type PermissionType string

const (
	PermissionScreenCapture PermissionType = "screen_capture"
	PermissionAccessibility PermissionType = "accessibility"
	PermissionMicrophone    PermissionType = "microphone"
)

// PermissionInfo contains information about a permission
type PermissionInfo struct {
	Type        PermissionType   `json:"type"`
	Status      PermissionStatus `json:"status"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Required    bool             `json:"required"`
}

// Manager handles permission checking and requesting
type Manager struct{}

// NewManager creates a new permission manager
func NewManager() *Manager {
	return &Manager{}
}

// GetAllPermissions returns the status of all required permissions
// Uses REAL checks (creates test event tap for accessibility)
// NOTE: Accessibility is checked FIRST because CGWindowListCreateImage() (used in screen capture check)
// can interfere with CGEventTapCreate() on some macOS versions
func (m *Manager) GetAllPermissions() []PermissionInfo {
	// Check accessibility FIRST (before screen capture check which uses CGWindowListCreateImage)
	accessibilityStatus := m.CheckAccessibility()
	screenCaptureStatus := m.CheckScreenCapture()

	return []PermissionInfo{
		{
			Type:        PermissionAccessibility,
			Status:      accessibilityStatus,
			Name:        "Универсальный доступ",
			Description: "Необходимо для ввода с клавиатуры. После выдачи разрешения ПЕРЕЗАПУСТИТЕ приложение!",
			Required:    true,
		},
		{
			Type:        PermissionScreenCapture,
			Status:      screenCaptureStatus,
			Name:        "Запись экрана",
			Description: "Необходимо для скриншотов ВСЕХ окон. После выдачи - ПЕРЕЗАПУСТИТЕ приложение!",
			Required:    true,
		},
	}
}

// GetAllPermissionsCached returns permissions using CACHED checks
// This is useful when event tap is already running and we can't create another one
func (m *Manager) GetAllPermissionsCached() []PermissionInfo {
	return []PermissionInfo{
		{
			Type:        PermissionAccessibility,
			Status:      m.CheckAccessibilityCached(),
			Name:        "Универсальный доступ",
			Description: "Необходимо для ввода с клавиатуры. После выдачи разрешения ПЕРЕЗАПУСТИТЕ приложение!",
			Required:    true,
		},
		{
			Type:        PermissionScreenCapture,
			Status:      m.CheckScreenCaptureCached(),
			Name:        "Запись экрана",
			Description: "Необходимо для скриншотов ВСЕХ окон. После выдачи - ПЕРЕЗАПУСТИТЕ приложение!",
			Required:    true,
		},
	}
}

// HasAllRequired checks if all required permissions are granted
func (m *Manager) HasAllRequired() bool {
	for _, p := range m.GetAllPermissions() {
		if p.Required && p.Status != StatusGranted {
			return false
		}
	}
	return true
}
