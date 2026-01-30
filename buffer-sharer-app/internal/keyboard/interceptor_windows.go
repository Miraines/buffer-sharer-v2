//go:build windows

package keyboard

import (
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

// InterceptorLogger interface for logging
type InterceptorLogger interface {
	Info(string, string, ...interface{})
	Debug(string, string, ...interface{})
}

// Win32 API
var (
	user32                = syscall.NewLazyDLL("user32.dll")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookEx  = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx    = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHook = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage        = user32.NewProc("GetMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procSendInput         = user32.NewProc("SendInput")
	procGetAsyncKeyState  = user32.NewProc("GetAsyncKeyState")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

// Constants
const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_SYSKEYDOWN  = 0x0104
	WM_QUIT        = 0x0012

	INPUT_KEYBOARD     = 1
	KEYEVENTF_UNICODE  = 0x0004
	KEYEVENTF_KEYUP    = 0x0002

	VK_SHIFT   = 0x10
	VK_CONTROL = 0x11
	VK_MENU    = 0x12 // Alt
	VK_LWIN    = 0x5B
	VK_RWIN    = 0x5C
	VK_CAPITAL = 0x14

	// Modifier key VK codes for filtering
	VK_LSHIFT   = 0xA0
	VK_RSHIFT   = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LMENU    = 0xA4
	VK_RMENU    = 0xA5
)

// KBDLLHOOKSTRUCT is the Windows low-level keyboard hook struct
type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// KEYBDINPUT for SendInput
type KEYBDINPUT struct {
	Type    uint32
	Ki      keyboardInput
	Padding [8]byte
}

type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// MSG structure
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// Hotkey config
type hotkeyConfig struct {
	vkCode    uint32
	ctrl      bool
	shift     bool
	alt       bool
	configured bool
}

// KeyInterceptor intercepts keyboard events and replaces them with buffer content
type KeyInterceptor struct {
	mu            sync.RWMutex
	buffer        []rune
	position      int
	enabled       bool
	running       bool
	typing        bool
	bufferVersion int
	logger        InterceptorLogger
	onBufferEmpty func()
	onToggle      func()
	onPaste       func()

	hookHandle    uintptr
	threadID      uint32
	isTyping      bool // prevents recursion during SendInput

	toggleHotkey hotkeyConfig
	pasteHotkey  hotkeyConfig
}

// Global interceptor instance (needed for hook callback)
var globalInterceptor *KeyInterceptor

// MaxBufferSize is the maximum allowed buffer size (10MB) to prevent memory exhaustion
const MaxBufferSize = 10 * 1024 * 1024

// NewKeyInterceptor creates a new key interceptor
func NewKeyInterceptor(logger InterceptorLogger) *KeyInterceptor {
	if logger != nil {
		logger.Debug("interceptor", "NewKeyInterceptor called")
	}
	ki := &KeyInterceptor{
		logger: logger,
	}
	globalInterceptor = ki
	if logger != nil {
		logger.Debug("interceptor", "KeyInterceptor created and set as globalInterceptor")
	}
	return ki
}

// SetBuffer sets the text buffer to type
func (ki *KeyInterceptor) SetBuffer(text string) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetBuffer called with text length %d bytes", len(text))
	}

	if len(text) > MaxBufferSize {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "WARNING: Buffer size %d exceeds maximum %d, truncating", len(text), MaxBufferSize)
		}
		text = text[:MaxBufferSize]
	}

	ki.mu.Lock()
	defer ki.mu.Unlock()

	if ki.typing {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "WARNING: SetBuffer called while TypeAllBuffer is in progress!")
		}
	}

	ki.buffer = []rune(text)
	ki.position = 0
	ki.bufferVersion++

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Buffer set with %d characters (runes), version=%d", len(ki.buffer), ki.bufferVersion)
		if len(ki.buffer) > 0 {
			preview := string(ki.buffer)
			if len(ki.buffer) > 20 {
				preview = string(ki.buffer[:20]) + "..."
			}
			ki.logger.Debug("interceptor", "Buffer preview: %q", preview)
		}
	}
}

// GetRemainingBuffer returns the remaining buffer content
func (ki *KeyInterceptor) GetRemainingBuffer() string {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	if ki.position >= len(ki.buffer) {
		return ""
	}
	return string(ki.buffer[ki.position:])
}

// GetBufferLength returns total buffer length
func (ki *KeyInterceptor) GetBufferLength() int {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	return len(ki.buffer)
}

// GetPosition returns current position in buffer
func (ki *KeyInterceptor) GetPosition() int {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	return ki.position
}

// ClearBuffer clears the buffer
func (ki *KeyInterceptor) ClearBuffer() {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.buffer = nil
	ki.position = 0
}

// SetOnBufferEmpty sets callback for when buffer is exhausted
func (ki *KeyInterceptor) SetOnBufferEmpty(callback func()) {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onBufferEmpty = callback
}

// SetOnToggle sets callback for when toggle hotkey is pressed
func (ki *KeyInterceptor) SetOnToggle(callback func()) {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onToggle = callback
}

// SetOnPaste sets callback for when paste hotkey is pressed
func (ki *KeyInterceptor) SetOnPaste(callback func()) {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onPaste = callback
}

// SetToggleHotkey configures the toggle hotkey (e.g., "Ctrl+Shift+T")
func (ki *KeyInterceptor) SetToggleHotkey(hotkeyStr string) {
	if ki.logger != nil {
		ki.logger.Info("interceptor", "SetToggleHotkey called with: %s", hotkeyStr)
	}

	hk := parseWindowsHotkey(hotkeyStr)
	ki.mu.Lock()
	ki.toggleHotkey = hk
	ki.mu.Unlock()

	if ki.logger != nil {
		if hk.configured {
			ki.logger.Info("interceptor", "Toggle hotkey set: vk=0x%X ctrl=%v shift=%v alt=%v", hk.vkCode, hk.ctrl, hk.shift, hk.alt)
		} else {
			ki.logger.Info("interceptor", "Toggle hotkey cleared")
		}
	}
}

// SetPasteHotkey configures the paste hotkey (e.g., "Ctrl+Shift+V")
func (ki *KeyInterceptor) SetPasteHotkey(hotkeyStr string) {
	if ki.logger != nil {
		ki.logger.Info("interceptor", "SetPasteHotkey called with: %s", hotkeyStr)
	}

	hk := parseWindowsHotkey(hotkeyStr)
	ki.mu.Lock()
	ki.pasteHotkey = hk
	ki.mu.Unlock()

	if ki.logger != nil {
		if hk.configured {
			ki.logger.Info("interceptor", "Paste hotkey set: vk=0x%X ctrl=%v shift=%v alt=%v", hk.vkCode, hk.ctrl, hk.shift, hk.alt)
		} else {
			ki.logger.Info("interceptor", "Paste hotkey cleared")
		}
	}
}

// isModifierKey returns true if the vkCode is a modifier key
func isModifierKey(vkCode uint32) bool {
	switch vkCode {
	case VK_SHIFT, VK_CONTROL, VK_MENU,
		VK_LSHIFT, VK_RSHIFT,
		VK_LCONTROL, VK_RCONTROL,
		VK_LMENU, VK_RMENU,
		VK_LWIN, VK_RWIN,
		VK_CAPITAL:
		return true
	}
	return false
}

// isKeyPressed checks if a key is currently pressed
func isKeyPressed(vk int) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return ret&0x8000 != 0
}

// checkHotkey checks if the given event matches a hotkey config
func checkHotkey(hk hotkeyConfig, vkCode uint32) bool {
	if !hk.configured {
		return false
	}
	if vkCode != hk.vkCode {
		return false
	}
	ctrlPressed := isKeyPressed(VK_CONTROL)
	shiftPressed := isKeyPressed(VK_SHIFT)
	altPressed := isKeyPressed(VK_MENU)

	return ctrlPressed == hk.ctrl && shiftPressed == hk.shift && altPressed == hk.alt
}

// hookCallback is the low-level keyboard hook procedure
func hookCallback(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode < 0 {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	if globalInterceptor == nil {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	ki := globalInterceptor

	// Only process key down events
	if wParam != WM_KEYDOWN && wParam != WM_SYSKEYDOWN {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	kbs := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
	vkCode := kbs.VkCode

	// Check if this is an injected event (from our SendInput) - skip to avoid recursion
	const LLKHF_INJECTED = 0x00000010
	if kbs.Flags&LLKHF_INJECTED != 0 {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	ki.mu.RLock()
	toggleHk := ki.toggleHotkey
	pasteHk := ki.pasteHotkey
	enabled := ki.enabled
	isTypingNow := ki.isTyping
	ki.mu.RUnlock()

	// Check toggle hotkey
	if checkHotkey(toggleHk, vkCode) {
		ki.mu.RLock()
		callback := ki.onToggle
		ki.mu.RUnlock()
		if callback != nil {
			go callback()
		}
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Check paste hotkey
	if checkHotkey(pasteHk, vkCode) {
		ki.mu.RLock()
		callback := ki.onPaste
		ki.mu.RUnlock()
		if callback != nil {
			go callback()
		}
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// If not enabled or currently typing, let through
	if !enabled || isTypingNow {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Ignore modifier keys
	if isModifierKey(vkCode) {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Check modifier flags - if Ctrl, Alt, or Win are pressed, let through (hotkey combination)
	if isKeyPressed(VK_CONTROL) || isKeyPressed(VK_MENU) || isKeyPressed(VK_LWIN) || isKeyPressed(VK_RWIN) {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Intercept: handle key press and block original
	ki.handleKeyPress()

	// Return 1 to block the original key event
	return 1
}

// inputSize is the size of the INPUT struct on 64-bit Windows (40 bytes)
// Layout: type (4 bytes) + padding (4 bytes) + union (32 bytes)
// The union starts at offset 8 due to pointer alignment in MOUSEINPUT
const inputSize = 40
const inputUnionOffset = 8 // offset of the union inside INPUT on amd64

// typeUnicodeChar types a single Unicode character using SendInput with KEYEVENTF_UNICODE
func typeUnicodeChar(r rune) {
	// Encode the rune as UTF-16
	encoded := utf16.Encode([]rune{r})

	for _, u16 := range encoded {
		// Key down
		var inputDown [inputSize]byte
		*(*uint32)(unsafe.Pointer(&inputDown[0])) = INPUT_KEYBOARD
		ki := (*keyboardInput)(unsafe.Pointer(&inputDown[inputUnionOffset]))
		ki.WVk = 0
		ki.WScan = u16
		ki.DwFlags = KEYEVENTF_UNICODE
		ki.Time = 0
		ki.DwExtraInfo = 0

		procSendInput.Call(1, uintptr(unsafe.Pointer(&inputDown[0])), uintptr(inputSize))

		// Key up
		var inputUp [inputSize]byte
		*(*uint32)(unsafe.Pointer(&inputUp[0])) = INPUT_KEYBOARD
		kiUp := (*keyboardInput)(unsafe.Pointer(&inputUp[inputUnionOffset]))
		kiUp.WVk = 0
		kiUp.WScan = u16
		kiUp.DwFlags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP
		kiUp.Time = 0
		kiUp.DwExtraInfo = 0

		procSendInput.Call(1, uintptr(unsafe.Pointer(&inputUp[0])), uintptr(inputSize))
	}
}

// typeUnicodeString types a string character by character
func typeUnicodeString(s string) {
	for _, r := range s {
		typeUnicodeChar(r)
		time.Sleep(5 * time.Millisecond)
	}
}

// handleKeyPress is called when a key is pressed while interception is enabled
func (ki *KeyInterceptor) handleKeyPress() {
	ki.mu.Lock()

	if ki.position >= len(ki.buffer) {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Buffer exhausted (pos=%d >= len=%d), disabling interception", ki.position, len(ki.buffer))
		}
		ki.enabled = false
		callback := ki.onBufferEmpty
		ki.mu.Unlock()

		if callback != nil {
			go callback()
		}
		return
	}

	char := ki.buffer[ki.position]
	ki.position++
	remaining := len(ki.buffer) - ki.position
	ki.isTyping = true
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Typing char %q (U+%04X), position=%d, remaining=%d", string(char), char, ki.position-1, remaining)
	}

	typeUnicodeChar(char)

	ki.mu.Lock()
	ki.isTyping = false
	ki.mu.Unlock()

	if ki.logger != nil && remaining%10 == 0 {
		ki.logger.Info("interceptor", "Typed character, %d remaining", remaining)
	}
}


// Start starts the key interceptor
func (ki *KeyInterceptor) Start() bool {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Start() called")
	}

	ki.mu.Lock()
	if ki.running {
		ki.mu.Unlock()
		return true
	}
	ki.enabled = false // Ensure disabled on start
	ki.mu.Unlock()

	started := make(chan bool, 1)

	go func() {
		// Lock this goroutine to an OS thread - Windows hooks are thread-bound
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Create the hook callback
		cb := syscall.NewCallback(func(nCode int, wParam uintptr, lParam uintptr) uintptr {
			return hookCallback(nCode, wParam, lParam)
		})

		// Install the low-level keyboard hook
		hookHandle, _, err := procSetWindowsHookEx.Call(
			WH_KEYBOARD_LL,
			cb,
			0, // hMod = 0 for low-level hooks
			0, // dwThreadId = 0 for all threads
		)

		if hookHandle == 0 {
			if ki.logger != nil {
				ki.logger.Info("interceptor", "SetWindowsHookEx failed: %v", err)
			}
			started <- false
			return
		}

		// Get thread ID for later stopping
		threadID, _, _ := procGetCurrentThreadId.Call()

		ki.mu.Lock()
		ki.hookHandle = hookHandle
		ki.threadID = uint32(threadID)
		ki.running = true
		ki.mu.Unlock()

		if ki.logger != nil {
			ki.logger.Info("interceptor", "Key interceptor started successfully (hook=0x%X, thread=%d)", hookHandle, threadID)
		}

		started <- true

		// Message loop - required for low-level hooks to work
		var msg MSG
		for {
			ret, _, _ := procGetMessage.Call(
				uintptr(unsafe.Pointer(&msg)),
				0, 0, 0,
			)
			if ret == 0 || int32(ret) == -1 {
				break
			}
		}

		// Unhook when message loop ends
		procUnhookWindowsHook.Call(hookHandle)

		ki.mu.Lock()
		ki.running = false
		ki.hookHandle = 0
		ki.threadID = 0
		ki.mu.Unlock()

		if ki.logger != nil {
			ki.logger.Info("interceptor", "Event loop goroutine finished, running=false")
		}
	}()

	return <-started
}

// Stop stops the key interceptor
func (ki *KeyInterceptor) Stop() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Stop() called")
	}

	ki.mu.Lock()
	if !ki.running {
		ki.mu.Unlock()
		return
	}
	threadID := ki.threadID
	hookHandle := ki.hookHandle
	ki.enabled = false
	ki.mu.Unlock()

	// Post WM_QUIT to the hook thread to stop the message loop
	if threadID != 0 {
		procPostThreadMessage.Call(
			uintptr(threadID),
			WM_QUIT,
			0, 0,
		)
	}

	// Wait for the goroutine to finish
	stopped := false
	for i := 0; i < 50; i++ {
		ki.mu.RLock()
		running := ki.running
		ki.mu.RUnlock()
		if !running {
			stopped = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// If message loop didn't exit gracefully, force-unhook directly
	if !stopped && hookHandle != 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Message loop didn't exit gracefully, force-unhooking")
		}
		procUnhookWindowsHook.Call(hookHandle)
	}

	ki.mu.Lock()
	ki.running = false
	ki.enabled = false
	ki.hookHandle = 0
	ki.threadID = 0
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Key interceptor stopped")
	}
}

// SetEnabled enables or disables key interception
func (ki *KeyInterceptor) SetEnabled(enabled bool) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetEnabled(%v) called", enabled)
	}

	ki.mu.Lock()
	running := ki.running
	bufLen := len(ki.buffer)
	bufPos := ki.position
	remainingChars := bufLen - bufPos

	if enabled && remainingChars <= 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Cannot enable - buffer is empty! Keeping interception disabled.")
		}
		ki.enabled = false
		ki.mu.Unlock()
		return
	}

	ki.enabled = enabled
	ki.mu.Unlock()

	if enabled && !running {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Hook not running, attempting to start...")
		}
		if !ki.Start() {
			if ki.logger != nil {
				ki.logger.Info("interceptor", "Cannot enable - hook failed to start")
			}
			ki.mu.Lock()
			ki.enabled = false
			ki.mu.Unlock()
			return
		}
	}

	if ki.logger != nil {
		if enabled {
			ki.logger.Info("interceptor", "Key interception ENABLED (buffer has %d chars at pos %d)", bufLen, bufPos)
		} else {
			ki.logger.Info("interceptor", "Key interception DISABLED")
		}
	}
}

// IsEnabled returns whether interception is enabled
func (ki *KeyInterceptor) IsEnabled() bool {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	return ki.enabled
}

// IsRunning returns whether the interceptor is running
func (ki *KeyInterceptor) IsRunning() bool {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	return ki.running
}

// TypeAllBuffer types the entire remaining buffer content at once
func (ki *KeyInterceptor) TypeAllBuffer() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "TypeAllBuffer() called")
	}

	ki.mu.Lock()
	if ki.position >= len(ki.buffer) {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "TypeAllBuffer: buffer is empty")
		}
		ki.mu.Unlock()
		return
	}

	ki.typing = true
	remainingText := string(ki.buffer[ki.position:])
	remainingLen := len(ki.buffer) - ki.position
	expectedEndPosition := len(ki.buffer)
	versionBeforeTyping := ki.bufferVersion

	ki.enabled = false
	ki.isTyping = true
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Info("interceptor", "TypeAllBuffer: typing %d characters (version=%d)", remainingLen, versionBeforeTyping)
	}

	typeUnicodeString(remainingText)

	if ki.logger != nil {
		ki.logger.Info("interceptor", "TypeAllBuffer: finished typing")
	}

	ki.mu.Lock()
	ki.typing = false
	ki.isTyping = false

	if ki.bufferVersion == versionBeforeTyping {
		ki.position = expectedEndPosition
	} else {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "TypeAllBuffer: buffer was changed during typing (version %d -> %d), keeping new buffer position=%d",
				versionBeforeTyping, ki.bufferVersion, ki.position)
		}
	}
	callback := ki.onBufferEmpty
	ki.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// parseWindowsHotkey parses a hotkey string like "Ctrl+Shift+T" into Windows VK config
func parseWindowsHotkey(hotkeyStr string) hotkeyConfig {
	if hotkeyStr == "" {
		return hotkeyConfig{}
	}

	parts := splitHotkeyParts(hotkeyStr)
	if len(parts) == 0 {
		return hotkeyConfig{}
	}

	hk := hotkeyConfig{}

	// Last part is the key
	keyStr := parts[len(parts)-1]
	hk.vkCode = stringToVKCode(keyStr)
	if hk.vkCode == 0 {
		return hotkeyConfig{}
	}

	// Other parts are modifiers
	for i := 0; i < len(parts)-1; i++ {
		mod := toLower(parts[i])
		switch mod {
		case "ctrl", "control":
			hk.ctrl = true
		case "shift":
			hk.shift = true
		case "alt", "option":
			hk.alt = true
		case "cmd", "command":
			// On Windows, map Cmd to Ctrl
			hk.ctrl = true
		}
	}

	hk.configured = true
	return hk
}

// splitHotkeyParts splits a hotkey string by "+"
func splitHotkeyParts(s string) []string {
	var parts []string
	var current string
	for _, r := range s {
		if r == '+' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

// stringToVKCode converts a key string to Windows virtual key code
func stringToVKCode(s string) uint32 {
	lower := toLower(s)

	switch lower {
	case "a":
		return 0x41
	case "b":
		return 0x42
	case "c":
		return 0x43
	case "d":
		return 0x44
	case "e":
		return 0x45
	case "f":
		return 0x46
	case "g":
		return 0x47
	case "h":
		return 0x48
	case "i":
		return 0x49
	case "j":
		return 0x4A
	case "k":
		return 0x4B
	case "l":
		return 0x4C
	case "m":
		return 0x4D
	case "n":
		return 0x4E
	case "o":
		return 0x4F
	case "p":
		return 0x50
	case "q":
		return 0x51
	case "r":
		return 0x52
	case "s":
		return 0x53
	case "t":
		return 0x54
	case "u":
		return 0x55
	case "v":
		return 0x56
	case "w":
		return 0x57
	case "x":
		return 0x58
	case "y":
		return 0x59
	case "z":
		return 0x5A
	case "0":
		return 0x30
	case "1":
		return 0x31
	case "2":
		return 0x32
	case "3":
		return 0x33
	case "4":
		return 0x34
	case "5":
		return 0x35
	case "6":
		return 0x36
	case "7":
		return 0x37
	case "8":
		return 0x38
	case "9":
		return 0x39
	case "space":
		return 0x20
	case "return", "enter":
		return 0x0D
	case "tab":
		return 0x09
	case "escape", "esc":
		return 0x1B
	case "delete", "backspace":
		return 0x08
	case "f1":
		return 0x70
	case "f2":
		return 0x71
	case "f3":
		return 0x72
	case "f4":
		return 0x73
	case "f5":
		return 0x74
	case "f6":
		return 0x75
	case "f7":
		return 0x76
	case "f8":
		return 0x77
	case "f9":
		return 0x78
	case "f10":
		return 0x79
	case "f11":
		return 0x7A
	case "f12":
		return 0x7B
	}
	return 0
}

