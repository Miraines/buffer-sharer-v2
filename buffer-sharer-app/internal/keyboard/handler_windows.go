//go:build windows

package keyboard

import (
	"time"
	"unicode/utf16"
	"unsafe"
)

// platformType types text using Windows SendInput with KEYEVENTF_UNICODE
func platformType(text string) {
	for _, r := range text {
		sendUnicodeChar(r)
		time.Sleep(5 * time.Millisecond)
	}
}

// platformKeyTap simulates a key tap using Windows SendInput
// args are modifier key names (e.g., "ctrl", "shift", "alt")
func platformKeyTap(key string, args ...interface{}) {
	// Resolve the main key VK code
	vk := stringToVKCode(key)
	if vk == 0 {
		return
	}

	// Collect modifier VK codes
	var modVKs []uint32
	for _, arg := range args {
		if s, ok := arg.(string); ok {
			switch toLower(s) {
			case "ctrl", "control":
				modVKs = append(modVKs, VK_CONTROL)
			case "shift":
				modVKs = append(modVKs, VK_SHIFT)
			case "alt", "menu":
				modVKs = append(modVKs, VK_MENU)
			}
		}
	}

	// Press modifiers down
	for _, mk := range modVKs {
		sendKeyEvent(uint16(mk), 0)
	}

	// Press and release the main key
	sendKeyEvent(uint16(vk), 0)
	sendKeyEvent(uint16(vk), KEYEVENTF_KEYUP)

	// Release modifiers
	for i := len(modVKs) - 1; i >= 0; i-- {
		sendKeyEvent(uint16(modVKs[i]), KEYEVENTF_KEYUP)
	}
}

// sendKeyEvent sends a single key event via SendInput
func sendKeyEvent(vk uint16, flags uint32) {
	var input [inputSize]byte
	*(*uint32)(unsafe.Pointer(&input[0])) = INPUT_KEYBOARD
	ki := (*keyboardInput)(unsafe.Pointer(&input[inputUnionOffset]))
	ki.WVk = vk
	ki.WScan = 0
	ki.DwFlags = flags
	ki.Time = 0
	ki.DwExtraInfo = 0

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input[0])), uintptr(inputSize))
}

// sendUnicodeChar sends a single Unicode character via SendInput
func sendUnicodeChar(r rune) {
	encoded := utf16.Encode([]rune{r})

	for _, u16 := range encoded {
		var inputDown [inputSize]byte
		*(*uint32)(unsafe.Pointer(&inputDown[0])) = INPUT_KEYBOARD
		ki := (*keyboardInput)(unsafe.Pointer(&inputDown[inputUnionOffset]))
		ki.WVk = 0
		ki.WScan = u16
		ki.DwFlags = KEYEVENTF_UNICODE
		ki.Time = 0
		ki.DwExtraInfo = 0

		procSendInput.Call(1, uintptr(unsafe.Pointer(&inputDown[0])), uintptr(inputSize))

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
