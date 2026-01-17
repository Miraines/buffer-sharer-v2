//go:build darwin

package keyboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework Carbon -framework Foundation

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <Carbon/Carbon.h>
#include <stdlib.h>

// Forward declarations for Go callbacks
extern void goKeyPressCallback();
extern void goToggleHotkeyCallback();
extern void goPasteHotkeyCallback();

// Global state for key interception
static int interceptEnabled = 0;
static int isTyping = 0;  // Flag to prevent recursive calls during typing
static CFMachPortRef eventTap = NULL;
static CFRunLoopSourceRef runLoopSource = NULL;
static CFRunLoopRef tapRunLoop = NULL;

// Toggle hotkey configuration
static CGKeyCode toggleHotkeyKeyCode = 0;
static CGEventFlags toggleHotkeyModifiers = 0;
static int toggleHotkeyConfigured = 0;

// Paste hotkey configuration
static CGKeyCode pasteHotkeyKeyCode = 0;
static CGEventFlags pasteHotkeyModifiers = 0;
static int pasteHotkeyConfigured = 0;

// Set the toggle hotkey (called from Go)
static inline void setToggleHotkey(unsigned short keyCode, unsigned long long modifiers) {
    toggleHotkeyKeyCode = (CGKeyCode)keyCode;
    toggleHotkeyModifiers = (CGEventFlags)modifiers;
    toggleHotkeyConfigured = 1;
}

// Clear the toggle hotkey
static inline void clearToggleHotkey(void) {
    toggleHotkeyKeyCode = 0;
    toggleHotkeyModifiers = 0;
    toggleHotkeyConfigured = 0;
}

// Set the paste hotkey (called from Go)
static inline void setPasteHotkey(unsigned short keyCode, unsigned long long modifiers) {
    pasteHotkeyKeyCode = (CGKeyCode)keyCode;
    pasteHotkeyModifiers = (CGEventFlags)modifiers;
    pasteHotkeyConfigured = 1;
}

// Clear the paste hotkey
static inline void clearPasteHotkey(void) {
    pasteHotkeyKeyCode = 0;
    pasteHotkeyModifiers = 0;
    pasteHotkeyConfigured = 0;
}

// Check if the current event matches the toggle hotkey
static int isToggleHotkeyEvent(CGKeyCode keyCode, CGEventFlags flags) {
    if (!toggleHotkeyConfigured) {
        return 0;
    }
    if (keyCode != toggleHotkeyKeyCode) {
        return 0;
    }
    CGEventFlags relevantFlags = flags & (kCGEventFlagMaskCommand | kCGEventFlagMaskControl |
                                          kCGEventFlagMaskShift | kCGEventFlagMaskAlternate);
    CGEventFlags requiredFlags = toggleHotkeyModifiers & (kCGEventFlagMaskCommand | kCGEventFlagMaskControl |
                                                          kCGEventFlagMaskShift | kCGEventFlagMaskAlternate);
    return (relevantFlags == requiredFlags);
}

// Check if the current event matches the paste hotkey
static int isPasteHotkeyEvent(CGKeyCode keyCode, CGEventFlags flags) {
    if (!pasteHotkeyConfigured) {
        return 0;
    }
    if (keyCode != pasteHotkeyKeyCode) {
        return 0;
    }
    CGEventFlags relevantFlags = flags & (kCGEventFlagMaskCommand | kCGEventFlagMaskControl |
                                          kCGEventFlagMaskShift | kCGEventFlagMaskAlternate);
    CGEventFlags requiredFlags = pasteHotkeyModifiers & (kCGEventFlagMaskCommand | kCGEventFlagMaskControl |
                                                          kCGEventFlagMaskShift | kCGEventFlagMaskAlternate);
    return (relevantFlags == requiredFlags);
}

// Callback for CGEventTap
static CGEventRef eventTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    // Handle tap disabled event
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        if (eventTap) {
            CGEventTapEnable(eventTap, true);
        }
        return event;
    }

    // Only process key down events
    if (type != kCGEventKeyDown) {
        return event;
    }

    // Check if this is a programmatically generated event (not from real keyboard)
    int64_t sourceStateID = CGEventGetIntegerValueField(event, kCGEventSourceStateID);
    if (sourceStateID != 1) {
        // This is a programmatic event, let it through
        return event;
    }

    // Get the key code and flags
    CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    CGEventFlags flags = CGEventGetFlags(event);

    // ALWAYS check for toggle hotkey first, regardless of intercept state
    // This ensures the hotkey can both enable AND disable the mode
    if (isToggleHotkeyEvent(keyCode, flags)) {
        goToggleHotkeyCallback();
        return event;
    }

    // Check for paste hotkey - types all remaining buffer at once
    if (isPasteHotkeyEvent(keyCode, flags)) {
        goPasteHotkeyCallback();
        return event;
    }

    // If interception is not enabled or we're typing, let the event through
    if (!interceptEnabled || isTyping) {
        return event;
    }

    // Ignore modifier keys themselves (they're not characters to type)
    if (keyCode == kVK_Shift || keyCode == kVK_RightShift ||
        keyCode == kVK_Control || keyCode == kVK_RightControl ||
        keyCode == kVK_Option || keyCode == kVK_RightOption ||
        keyCode == kVK_Command || keyCode == kVK_RightCommand ||
        keyCode == kVK_CapsLock || keyCode == kVK_Function) {
        return event;
    }

    // Check modifier flags - if Ctrl, Cmd, or Option are pressed, this is likely
    // a hotkey combination. Let it through so other hotkey handlers can process it.
    // Check for Ctrl (with or without Shift)
    if (flags & kCGEventFlagMaskControl) {
        return event;
    }

    // Check for Cmd (with or without Shift)
    if (flags & kCGEventFlagMaskCommand) {
        return event;
    }

    // Check for Option/Alt (with or without Shift)
    if (flags & kCGEventFlagMaskAlternate) {
        return event;
    }

    // Regular key press (possibly with Shift for uppercase) - intercept it
    // Call Go callback to handle the key press
    goKeyPressCallback();

    // Return NULL to suppress the original key event
    return NULL;
}

// Start the event tap
static int startEventTap() {
    if (eventTap != NULL) {
        return 1; // Already running
    }

    // Create event tap for key down events
    CGEventMask eventMask = CGEventMaskBit(kCGEventKeyDown);

    eventTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,
        eventMask,
        eventTapCallback,
        NULL
    );

    if (eventTap == NULL) {
        return 0; // Failed to create tap (no accessibility permission?)
    }

    // Create run loop source
    runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, eventTap, 0);
    if (runLoopSource == NULL) {
        CFRelease(eventTap);
        eventTap = NULL;
        return 0;
    }

    return 1;
}

// Run the event tap on current run loop (call from dedicated goroutine)
static void runEventTapLoop() {
    if (runLoopSource == NULL) {
        return;
    }

    tapRunLoop = CFRunLoopGetCurrent();
    CFRunLoopAddSource(tapRunLoop, runLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(eventTap, true);

    // This blocks until stopEventTap is called
    CFRunLoopRun();
}

// Stop the event tap
static void stopEventTap() {
    if (tapRunLoop != NULL) {
        CFRunLoopStop(tapRunLoop);
        tapRunLoop = NULL;
    }

    if (eventTap != NULL) {
        CGEventTapEnable(eventTap, false);
    }

    if (runLoopSource != NULL) {
        CFRelease(runLoopSource);
        runLoopSource = NULL;
    }

    if (eventTap != NULL) {
        CFRelease(eventTap);
        eventTap = NULL;
    }

    interceptEnabled = 0;
}

// Enable/disable interception
static void setInterceptEnabled(int enabled) {
    interceptEnabled = enabled;
}

static int isInterceptEnabled() {
    return interceptEnabled;
}

// Set typing flag (to prevent recursive interception during robotgo.Type)
static void setTypingFlag(int typing) {
    isTyping = typing;
}

// Type a single Unicode character using CGEventKeyboardSetUnicodeString
// This is more reliable for non-ASCII characters than robotgo
static void typeUnicodeChar(UniChar ch) {
    // Create an event source for synthetic events
    // ВАЖНО: Используем kCGEventSourceStatePrivate чтобы sourceStateID был != 1
    // и наш event tap callback пропускал эти события (не перехватывал их)
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStatePrivate);
    if (source == NULL) {
        // Fallback to combined session state (sourceStateID = 0, also will be skipped)
        source = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);
    }

    // Create key down and key up events
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, 0, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, 0, false);

    if (keyDown && keyUp) {
        // Set the Unicode character
        CGEventKeyboardSetUnicodeString(keyDown, 1, &ch);
        CGEventKeyboardSetUnicodeString(keyUp, 1, &ch);

        // Post the events using kCGHIDEventTap for better delivery
        CGEventPost(kCGHIDEventTap, keyDown);
        usleep(1000);  // 1ms delay between key down and key up
        CGEventPost(kCGHIDEventTap, keyUp);

        CFRelease(keyDown);
        CFRelease(keyUp);
    }

    if (source != NULL) {
        CFRelease(source);
    }
}

// Type a UTF-8 string character by character
static void typeUnicodeString(const char* utf8String) {
    if (utf8String == NULL) return;

    // Convert UTF-8 to UTF-16 (UniChar)
    CFStringRef str = CFStringCreateWithCString(kCFAllocatorDefault, utf8String, kCFStringEncodingUTF8);
    if (str == NULL) return;

    CFIndex length = CFStringGetLength(str);
    for (CFIndex i = 0; i < length; i++) {
        UniChar ch = CFStringGetCharacterAtIndex(str, i);
        typeUnicodeChar(ch);
        // Increased delay between characters for reliable input
        usleep(5000);  // 5ms delay between characters
    }

    CFRelease(str);
}
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// InterceptorLogger interface for logging
type InterceptorLogger interface {
	Info(string, string, ...interface{})
	Debug(string, string, ...interface{})
}

// KeyInterceptor intercepts keyboard events and replaces them with buffer content
type KeyInterceptor struct {
	mu            sync.RWMutex
	buffer        []rune
	position      int
	enabled       bool
	running       bool
	typing        bool // Flag to indicate TypeAllBuffer is in progress
	bufferVersion int  // Incremented on each SetBuffer call to detect changes
	logger        InterceptorLogger
	onBufferEmpty func() // Callback when buffer is exhausted
	onToggle      func() // Callback when toggle hotkey is pressed
	onPaste       func() // Callback when paste hotkey is pressed
}

// Global interceptor instance (needed for C callback)
var globalInterceptor *KeyInterceptor

//export goKeyPressCallback
func goKeyPressCallback() {
	if globalInterceptor != nil {
		globalInterceptor.handleKeyPress()
	} else {
		// This should never happen
		fmt.Println("[CRITICAL] goKeyPressCallback: globalInterceptor is nil!")
	}
}

//export goToggleHotkeyCallback
func goToggleHotkeyCallback() {
	if globalInterceptor != nil {
		globalInterceptor.handleToggleHotkey()
	} else {
		fmt.Println("[CRITICAL] goToggleHotkeyCallback: globalInterceptor is nil!")
	}
}

//export goPasteHotkeyCallback
func goPasteHotkeyCallback() {
	if globalInterceptor != nil {
		globalInterceptor.handlePasteHotkey()
	} else {
		fmt.Println("[CRITICAL] goPasteHotkeyCallback: globalInterceptor is nil!")
	}
}

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

// MaxBufferSize is the maximum allowed buffer size (10MB) to prevent memory exhaustion
const MaxBufferSize = 10 * 1024 * 1024

// SetBuffer sets the text buffer to type
func (ki *KeyInterceptor) SetBuffer(text string) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetBuffer called with text length %d bytes", len(text))
	}

	// Validate buffer size to prevent memory exhaustion from malicious input
	if len(text) > MaxBufferSize {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "WARNING: Buffer size %d exceeds maximum %d, truncating", len(text), MaxBufferSize)
		}
		text = text[:MaxBufferSize]
	}

	ki.mu.Lock()
	defer ki.mu.Unlock()

	// Если TypeAllBuffer в процессе - логируем предупреждение
	if ki.typing {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "WARNING: SetBuffer called while TypeAllBuffer is in progress!")
		}
	}

	ki.buffer = []rune(text)
	ki.position = 0
	ki.bufferVersion++ // Увеличиваем версию для отслеживания изменений

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Buffer set with %d characters (runes), version=%d", len(ki.buffer), ki.bufferVersion)
		if len(ki.buffer) > 0 {
			// Log first few chars for debugging (safe preview)
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
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "GetRemainingBuffer: buffer empty (pos=%d, len=%d)", ki.position, len(ki.buffer))
		}
		return ""
	}
	remaining := string(ki.buffer[ki.position:])
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "GetRemainingBuffer: %d chars remaining", len(ki.buffer)-ki.position)
	}
	return remaining
}

// GetBufferLength returns total buffer length
func (ki *KeyInterceptor) GetBufferLength() int {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	length := len(ki.buffer)
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "GetBufferLength: %d", length)
	}
	return length
}

// GetPosition returns current position in buffer
func (ki *KeyInterceptor) GetPosition() int {
	ki.mu.RLock()
	defer ki.mu.RUnlock()
	pos := ki.position
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "GetPosition: %d", pos)
	}
	return pos
}

// ClearBuffer clears the buffer
func (ki *KeyInterceptor) ClearBuffer() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "ClearBuffer called")
	}
	ki.mu.Lock()
	defer ki.mu.Unlock()
	prevLen := len(ki.buffer)
	ki.buffer = nil
	ki.position = 0
	if ki.logger != nil {
		ki.logger.Info("interceptor", "Buffer cleared (was %d chars)", prevLen)
	}
}

// SetOnBufferEmpty sets callback for when buffer is exhausted
func (ki *KeyInterceptor) SetOnBufferEmpty(callback func()) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetOnBufferEmpty called, callback=%v", callback != nil)
	}
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onBufferEmpty = callback
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "onBufferEmpty callback set")
	}
}

// SetOnToggle sets callback for when toggle hotkey is pressed
func (ki *KeyInterceptor) SetOnToggle(callback func()) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetOnToggle called, callback=%v", callback != nil)
	}
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onToggle = callback
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "onToggle callback set")
	}
}

// SetToggleHotkey configures the toggle hotkey (e.g., "Cmd+Shift+T")
// This hotkey will be detected directly in the event tap and will work
// even when interception mode is enabled.
func (ki *KeyInterceptor) SetToggleHotkey(hotkeyStr string) {
	if ki.logger != nil {
		ki.logger.Info("interceptor", "SetToggleHotkey called with: %s", hotkeyStr)
	}

	keyCode, modifiers := parseHotkeyString(hotkeyStr)
	if keyCode == 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Could not parse hotkey, clearing toggle hotkey")
		}
		C.clearToggleHotkey()
		return
	}

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Setting toggle hotkey: keyCode=%d, modifiers=0x%X", keyCode, modifiers)
	}
	C.setToggleHotkey(C.ushort(keyCode), C.ulonglong(modifiers))
}

// handleToggleHotkey is called when the toggle hotkey is pressed
func (ki *KeyInterceptor) handleToggleHotkey() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "handleToggleHotkey() called")
	}

	ki.mu.RLock()
	callback := ki.onToggle
	ki.mu.RUnlock()

	if callback != nil {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Calling toggle callback")
		}
		callback()
	} else {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "No toggle callback set")
		}
	}
}

// SetOnPaste sets callback for when paste hotkey is pressed
func (ki *KeyInterceptor) SetOnPaste(callback func()) {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetOnPaste called, callback=%v", callback != nil)
	}
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.onPaste = callback
}

// SetPasteHotkey configures the paste hotkey (e.g., "Cmd+Shift+V")
func (ki *KeyInterceptor) SetPasteHotkey(hotkeyStr string) {
	if ki.logger != nil {
		ki.logger.Info("interceptor", "SetPasteHotkey called with: %s", hotkeyStr)
	}

	keyCode, modifiers := parseHotkeyString(hotkeyStr)
	if keyCode == 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Could not parse hotkey, clearing paste hotkey")
		}
		C.clearPasteHotkey()
		return
	}

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Setting paste hotkey: keyCode=%d, modifiers=0x%X", keyCode, modifiers)
	}
	C.setPasteHotkey(C.ushort(keyCode), C.ulonglong(modifiers))
}

// handlePasteHotkey is called when the paste hotkey is pressed
func (ki *KeyInterceptor) handlePasteHotkey() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "handlePasteHotkey() called")
	}

	ki.mu.RLock()
	callback := ki.onPaste
	ki.mu.RUnlock()

	if callback != nil {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Calling paste callback")
		}
		callback()
	} else {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "No paste callback set")
		}
	}
}

// parseHotkeyString parses a hotkey string like "Cmd+Shift+T" into key code and modifiers
func parseHotkeyString(hotkeyStr string) (keyCode uint16, modifiers uint64) {
	if hotkeyStr == "" {
		return 0, 0
	}

	parts := splitHotkeyParts(hotkeyStr)
	if len(parts) == 0 {
		return 0, 0
	}

	// Last part is the key
	keyStr := parts[len(parts)-1]
	keyCode = stringToKeyCode(keyStr)

	// Other parts are modifiers
	for i := 0; i < len(parts)-1; i++ {
		mod := stringToModifier(parts[i])
		modifiers |= mod
	}

	return keyCode, modifiers
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

// stringToModifier converts a modifier string to CGEventFlags
func stringToModifier(s string) uint64 {
	// Normalize to lowercase
	lower := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			lower += string(r + 32)
		} else {
			lower += string(r)
		}
	}

	// CGEventFlags values from macOS headers
	const (
		kCGEventFlagMaskCommand   = 0x00100000 // Cmd
		kCGEventFlagMaskShift     = 0x00020000 // Shift
		kCGEventFlagMaskControl   = 0x00040000 // Ctrl
		kCGEventFlagMaskAlternate = 0x00080000 // Option/Alt
	)

	switch lower {
	case "cmd", "command":
		return kCGEventFlagMaskCommand
	case "shift":
		return kCGEventFlagMaskShift
	case "ctrl", "control":
		return kCGEventFlagMaskControl
	case "alt", "option":
		return kCGEventFlagMaskAlternate
	}
	return 0
}

// stringToKeyCode converts a key string to macOS virtual key code
func stringToKeyCode(s string) uint16 {
	// Normalize to lowercase
	lower := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			lower += string(r + 32)
		} else {
			lower += string(r)
		}
	}

	// Virtual key codes from Carbon/HIToolbox/Events.h
	switch lower {
	case "a":
		return 0x00
	case "b":
		return 0x0B
	case "c":
		return 0x08
	case "d":
		return 0x02
	case "e":
		return 0x0E
	case "f":
		return 0x03
	case "g":
		return 0x05
	case "h":
		return 0x04
	case "i":
		return 0x22
	case "j":
		return 0x26
	case "k":
		return 0x28
	case "l":
		return 0x25
	case "m":
		return 0x2E
	case "n":
		return 0x2D
	case "o":
		return 0x1F
	case "p":
		return 0x23
	case "q":
		return 0x0C
	case "r":
		return 0x0F
	case "s":
		return 0x01
	case "t":
		return 0x11
	case "u":
		return 0x20
	case "v":
		return 0x09
	case "w":
		return 0x0D
	case "x":
		return 0x07
	case "y":
		return 0x10
	case "z":
		return 0x06
	case "0":
		return 0x1D
	case "1":
		return 0x12
	case "2":
		return 0x13
	case "3":
		return 0x14
	case "4":
		return 0x15
	case "5":
		return 0x17
	case "6":
		return 0x16
	case "7":
		return 0x1A
	case "8":
		return 0x1C
	case "9":
		return 0x19
	case "space":
		return 0x31
	case "return", "enter":
		return 0x24
	case "tab":
		return 0x30
	case "escape", "esc":
		return 0x35
	case "delete", "backspace":
		return 0x33
	case "f1":
		return 0x7A
	case "f2":
		return 0x78
	case "f3":
		return 0x63
	case "f4":
		return 0x76
	case "f5":
		return 0x60
	case "f6":
		return 0x61
	case "f7":
		return 0x62
	case "f8":
		return 0x64
	case "f9":
		return 0x65
	case "f10":
		return 0x6D
	case "f11":
		return 0x67
	case "f12":
		return 0x6F
	}
	return 0
}

// Start starts the key interceptor
func (ki *KeyInterceptor) Start() bool {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Start() called")
	}

	ki.mu.Lock()
	if ki.running {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "Start(): already running, returning true")
		}
		ki.mu.Unlock()
		return true
	}
	ki.mu.Unlock()

	// IMPORTANT: Ensure interception is DISABLED before starting the event tap
	// This prevents keyboard blocking on startup
	C.setInterceptEnabled(0)

	// Log what we're about to try
	if ki.logger != nil {
		ki.logger.Info("interceptor", "Attempting to start event tap (interception disabled)...")
	}

	// Start the event tap
	startTime := time.Now()
	result := C.startEventTap()
	elapsed := time.Since(startTime)

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "C.startEventTap() returned %d in %v", result, elapsed)
	}

	if result == 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "CGEventTapCreate returned NULL - Accessibility permission NOT granted")
			ki.logger.Info("interceptor", "Go to System Settings > Privacy & Security > Accessibility")
			ki.logger.Info("interceptor", "Remove and re-add Buffer Sharer, then RESTART the app")
		}
		return false
	}

	ki.mu.Lock()
	ki.running = true
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Starting event loop goroutine...")
	}

	// Run the event loop in a goroutine
	go func() {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "Event loop goroutine started, calling C.runEventTapLoop()")
		}
		C.runEventTapLoop()
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "C.runEventTapLoop() returned, event loop ended")
		}
		ki.mu.Lock()
		ki.running = false
		ki.mu.Unlock()
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Event loop goroutine finished, running=false")
		}
	}()

	if ki.logger != nil {
		ki.logger.Info("interceptor", "Key interceptor started successfully")
	}
	return true
}

// Stop stops the key interceptor
func (ki *KeyInterceptor) Stop() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Stop() called")
	}

	ki.mu.Lock()
	if !ki.running {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "Stop(): not running, nothing to do")
		}
		ki.mu.Unlock()
		return
	}
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Calling C.stopEventTap()...")
	}
	C.stopEventTap()
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "C.stopEventTap() returned")
	}

	ki.mu.Lock()
	ki.running = false
	ki.enabled = false
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

	// Get current buffer state while holding lock
	bufLen := len(ki.buffer)
	bufPos := ki.position
	remainingChars := bufLen - bufPos

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "SetEnabled: running=%v, bufferLen=%d, bufferPos=%d, remaining=%d", running, bufLen, bufPos, remainingChars)
	}

	// IMPORTANT: Don't enable interception if buffer is empty
	// This prevents keyboard blocking when there's nothing to type
	if enabled && remainingChars <= 0 {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Cannot enable - buffer is empty! Keeping interception disabled.")
		}
		// Make sure interception stays disabled
		ki.enabled = false
		ki.mu.Unlock()
		C.setInterceptEnabled(0)
		return
	}

	// Set the enabled state while still holding lock
	ki.enabled = enabled
	ki.mu.Unlock()

	// If not running, warn and try to start
	if enabled && !running {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Event tap not running, attempting to start...")
		}
		if !ki.Start() {
			if ki.logger != nil {
				ki.logger.Info("interceptor", "Cannot enable - event tap failed to start (no Accessibility permission)")
			}
			// Revert the enabled state
			ki.mu.Lock()
			ki.enabled = false
			ki.mu.Unlock()
			return
		}
	}

	// Set C-level interception flag
	if enabled {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "Calling C.setInterceptEnabled(1)")
		}
		C.setInterceptEnabled(1)
	} else {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "Calling C.setInterceptEnabled(0)")
		}
		C.setInterceptEnabled(0)
	}

	if ki.logger != nil {
		if enabled {
			ki.logger.Info("interceptor", "Key interception ENABLED (event tap running, buffer has %d chars at pos %d)", bufLen, bufPos)
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
// This is used for the "paste all" hotkey functionality
func (ki *KeyInterceptor) TypeAllBuffer() {
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "TypeAllBuffer() called")
	}

	ki.mu.Lock()
	// Check if there's anything to type
	if ki.position >= len(ki.buffer) {
		if ki.logger != nil {
			ki.logger.Info("interceptor", "TypeAllBuffer: buffer is empty")
		}
		ki.mu.Unlock()
		return
	}

	// Устанавливаем флаг typing для защиты от race condition с SetBuffer
	ki.typing = true

	// ВАЖНО: Делаем КОПИЮ текста для печати, чтобы защититься от race condition
	// Если SetBuffer будет вызван во время печати, мы всё равно напечатаем
	// правильный текст (тот, который был в момент вызова TypeAllBuffer)
	remainingText := string(ki.buffer[ki.position:])
	remainingLen := len(ki.buffer) - ki.position
	expectedEndPosition := len(ki.buffer)

	// Сохраняем версию буфера для проверки после печати
	versionBeforeTyping := ki.bufferVersion

	// Disable interception while typing
	ki.enabled = false
	C.setInterceptEnabled(0)
	ki.mu.Unlock()

	if ki.logger != nil {
		ki.logger.Info("interceptor", "TypeAllBuffer: typing %d characters (version=%d)", remainingLen, versionBeforeTyping)
	}

	// Set typing flag to prevent any recursive interception
	C.setTypingFlag(1)

	// Type the entire string using native macOS API
	cStr := C.CString(remainingText)
	C.typeUnicodeString(cStr)
	C.free(unsafe.Pointer(cStr))

	// Clear typing flag
	C.setTypingFlag(0)

	if ki.logger != nil {
		ki.logger.Info("interceptor", "TypeAllBuffer: finished typing")
	}

	// NOW clear the buffer (mark all as typed) AFTER successful typing
	ki.mu.Lock()

	// Снимаем флаг typing
	ki.typing = false

	// ВАЖНО: Проверяем, не изменился ли буфер во время печати по версии
	// Если версия та же - устанавливаем позицию в конец
	// Если версия изменилась (SetBuffer был вызван) - не трогаем позицию,
	// новый буфер должен обрабатываться с начала
	if ki.bufferVersion == versionBeforeTyping {
		// Буфер не изменился - устанавливаем позицию в конец
		ki.position = expectedEndPosition
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "TypeAllBuffer: buffer unchanged (version=%d), position set to %d", ki.bufferVersion, ki.position)
		}
	} else {
		// Буфер изменился во время печати - оставляем новую позицию (0)
		if ki.logger != nil {
			ki.logger.Info("interceptor", "TypeAllBuffer: buffer was changed during typing (version %d -> %d), keeping new buffer position=%d",
				versionBeforeTyping, ki.bufferVersion, ki.position)
		}
	}
	callback := ki.onBufferEmpty
	ki.mu.Unlock()

	if callback != nil {
		if ki.logger != nil {
			ki.logger.Debug("interceptor", "TypeAllBuffer: calling onBufferEmpty callback")
		}
		callback()
	}
}

// handleKeyPress is called when a key is pressed while interception is enabled
func (ki *KeyInterceptor) handleKeyPress() {
	// Log every key press for debugging
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "handleKeyPress() called from CGo callback")
	}

	ki.mu.Lock()

	// Check if we have buffer content
	if ki.position >= len(ki.buffer) {
		// Buffer exhausted - disable interception
		if ki.logger != nil {
			ki.logger.Info("interceptor", "Buffer exhausted (pos=%d >= len=%d), disabling interception", ki.position, len(ki.buffer))
		}
		ki.enabled = false
		C.setInterceptEnabled(0)
		callback := ki.onBufferEmpty
		ki.mu.Unlock()

		if callback != nil {
			if ki.logger != nil {
				ki.logger.Debug("interceptor", "Calling onBufferEmpty callback...")
			}
			callback()
			if ki.logger != nil {
				ki.logger.Debug("interceptor", "onBufferEmpty callback returned")
			}
		} else {
			if ki.logger != nil {
				ki.logger.Debug("interceptor", "No onBufferEmpty callback set")
			}
		}
		return
	}

	// Get next character
	char := ki.buffer[ki.position]
	ki.position++
	remaining := len(ki.buffer) - ki.position
	ki.mu.Unlock()

	// Log each character being typed
	if ki.logger != nil {
		ki.logger.Debug("interceptor", "Typing char %q (U+%04X), position=%d, remaining=%d", string(char), char, ki.position-1, remaining)
	}

	// ВАЖНО: Устанавливаем флаг typing чтобы предотвратить рекурсивный перехват
	// событий генерируемых typeUnicodeChar
	C.setTypingFlag(1)

	// Type the character using native macOS API
	startType := time.Now()
	C.typeUnicodeChar(C.UniChar(char))
	typeElapsed := time.Since(startType)

	// Снимаем флаг typing
	C.setTypingFlag(0)

	if ki.logger != nil {
		ki.logger.Debug("interceptor", "typeUnicodeChar() completed in %v", typeElapsed)
	}

	// Also log at INFO level every 10 chars for visibility
	if ki.logger != nil && remaining%10 == 0 {
		ki.logger.Info("interceptor", "Typed character, %d remaining", remaining)
	}
}
