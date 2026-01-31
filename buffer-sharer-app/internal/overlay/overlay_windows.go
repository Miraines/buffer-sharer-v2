//go:build windows

package overlay

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// Win32 DLL / proc declarations
// ---------------------------------------------------------------------------

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadId         = kernel32.NewProc("GetCurrentThreadId")
	procDwmExtendFrameIntoClient   = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procCoInitializeEx             = ole32.NewProc("CoInitializeEx")
)

// ---------------------------------------------------------------------------
// Win32 constants
// ---------------------------------------------------------------------------

const (
	_WS_POPUP   = 0x80000000
	_WS_VISIBLE = 0x10000000

	_WS_EX_TOPMOST     = 0x00000008
	_WS_EX_TOOLWINDOW  = 0x00000080
	_WS_EX_NOACTIVATE  = 0x08000000
	_WS_EX_LAYERED     = 0x00080000
	_WS_EX_TRANSPARENT = 0x00000020

	_GWL_EXSTYLE = ^uintptr(19) // -20 in two's complement

	_SW_SHOW = 5
	_SW_HIDE = 0

	_SWP_NOSIZE     = 0x0001
	_SWP_NOMOVE     = 0x0002
	_SWP_NOACTIVATE = 0x0010
	_SWP_SHOWWINDOW = 0x0040

	_HWND_TOPMOST = ^uintptr(0) // (HWND)-1

	_SM_CXSCREEN = 0
	_SM_CYSCREEN = 1

	_LWA_ALPHA = 0x00000002

	_COINIT_APARTMENTTHREADED = 0x2

	// Standard Win32 messages
	_WM_DESTROY = 0x0002
	_WM_SIZE    = 0x0005
	_WM_USER    = 0x0400

	// Custom overlay messages
	_WM_OVERLAY_EVALJS          = _WM_USER + 1
	_WM_OVERLAY_SHOW            = _WM_USER + 2
	_WM_OVERLAY_HIDE            = _WM_USER + 3
	_WM_OVERLAY_DESTROY         = _WM_USER + 4
	_WM_OVERLAY_SET_CLICKTHROUGH = _WM_USER + 5
	_WM_OVERLAY_EVALJS_RESULT   = _WM_USER + 6
)

// ---------------------------------------------------------------------------
// Win32 structs
// ---------------------------------------------------------------------------

type _WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CnClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type _MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      _POINT
	_       uint32
}

type _MARGINS struct {
	CxLeftWidth    int32
	CxRightWidth   int32
	CyTopHeight    int32
	CyBottomHeight int32
}

type _POINT struct {
	X, Y int32
}

// ---------------------------------------------------------------------------
// HintRect — bounding rectangle of an interactive overlay element
// ---------------------------------------------------------------------------

type HintRect struct {
	ID         string
	X, Y, W, H float64
	Collapsed  bool
}

// ---------------------------------------------------------------------------
// Manager — overlay window manager for Windows
// ---------------------------------------------------------------------------

type Manager struct {
	mu      sync.RWMutex
	created bool
	visible bool

	// Window handle — accessed atomically for PostMessage from any goroutine
	hwnd uintptr

	chromium *edge.Chromium

	// JS eval queue: id → JS string (protected by evalMu)
	evalMu      sync.Mutex
	evalQueue   map[uint64]string
	evalCounter uint64

	// JS result channels: string-id → result channel (protected by resultMu)
	resultMu      sync.Mutex
	resultChans   map[string]chan string
	resultCounter uint64

	// DPI scale (cached on overlay thread init, read from other goroutines)
	dpiScale float64

	// Hint interaction
	hintRects     map[string]HintRect
	textRects     map[string]HintRect
	mouseOverHint bool
	hintTimer     *time.Ticker
	hintTimerStop chan struct{}

	// Re-order goroutine
	reorderStop chan struct{}

	// WaitGroup for background goroutines so Destroy can wait for them
	wg sync.WaitGroup

	// Action callback (from JS polling)
	onAction func(action string, actionType string, id string)

	// Overlay thread readiness signal
	ready chan struct{}
}

// ---------------------------------------------------------------------------
// NewManager — creates overlay and immediately shows it
// ---------------------------------------------------------------------------

func NewManager() *Manager {
	m := &Manager{
		hintRects:   make(map[string]HintRect),
		textRects:   make(map[string]HintRect),
		evalQueue:   make(map[uint64]string),
		resultChans: make(map[string]chan string),
		ready:       make(chan struct{}),
	}

	log.Println("[OVERLAY] NewManager: starting overlay thread...")
	go m.overlayThread()

	// Wait for the overlay thread to complete initialization
	select {
	case <-m.ready:
		log.Println("[OVERLAY] NewManager: overlay thread ready")
	case <-time.After(15 * time.Second):
		log.Println("[OVERLAY] NewManager: ERROR — overlay thread init timed out (15s)")
		return m
	}

	// Periodically re-order overlay to stay on top (same as macOS)
	m.reorderStop = make(chan struct{})
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.reorderStop:
				return
			case <-ticker.C:
				m.mu.RLock()
				vis := m.visible
				m.mu.RUnlock()
				if vis {
					m.postToOverlay(_WM_OVERLAY_SHOW, 0, 0)
				}
			}
		}
	}()

	return m
}

// ---------------------------------------------------------------------------
// overlayThread — dedicated OS thread with Win32 message loop
// ---------------------------------------------------------------------------

func (m *Manager) overlayThread() {
	runtime.LockOSThread()
	// NOTE: never unlock — this thread is permanently dedicated to the overlay

	log.Println("[OVERLAY] overlayThread: started, OS thread locked")

	// 1. COM initialization (required for WebView2)
	hr, _, err := procCoInitializeEx.Call(0, _COINIT_APARTMENTTHREADED)
	log.Printf("[OVERLAY] CoInitializeEx: hr=0x%x err=%v", hr, err)

	// 2. Get module handle
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	log.Printf("[OVERLAY] hInstance=0x%x", hInstance)

	// 3. Register window class
	className := windows.StringToUTF16Ptr("BufferSharerOverlayWnd")
	wndProcCB := syscall.NewCallback(m.wndProcHandler)

	var wcx _WNDCLASSEXW
	wcx.CbSize = uint32(unsafe.Sizeof(wcx))
	wcx.LpfnWndProc = wndProcCB
	wcx.HInstance = hInstance
	wcx.LpszClassName = className

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))
	log.Printf("[OVERLAY] RegisterClassExW: atom=%d err=%v", atom, err)

	// 4. Get screen dimensions
	screenW, _, _ := procGetSystemMetrics.Call(_SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(_SM_CYSCREEN)
	log.Printf("[OVERLAY] Screen size: %dx%d", screenW, screenH)

	// 5. Extended style — topmost, tool window, no activate, layered, click-through
	exStyle := uintptr(_WS_EX_TOPMOST | _WS_EX_TOOLWINDOW | _WS_EX_NOACTIVATE |
		_WS_EX_LAYERED | _WS_EX_TRANSPARENT)
	log.Printf("[OVERLAY] Creating window with exStyle=0x%x", exStyle)

	// 6. Create overlay window (full-screen popup)
	hwnd, _, err := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(""))),
		_WS_POPUP|_WS_VISIBLE,
		0, 0, screenW, screenH,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		log.Printf("[OVERLAY] ERROR: CreateWindowExW failed! err=%v", err)
		close(m.ready)
		return
	}
	log.Printf("[OVERLAY] Window created: hwnd=0x%x size=%dx%d", hwnd, screenW, screenH)

	// 7. DWM extend frame for per-pixel transparency via DWM composition
	margins := _MARGINS{-1, -1, -1, -1}
	ret, _, err := procDwmExtendFrameIntoClient.Call(hwnd, uintptr(unsafe.Pointer(&margins)))
	log.Printf("[OVERLAY] DwmExtendFrameIntoClientArea: ret=0x%x err=%v", ret, err)

	// 8. Set layered window attributes (full alpha, transparency via DWM)
	ret2, _, err := procSetLayeredWindowAttributes.Call(hwnd, 0, 255, _LWA_ALPHA)
	log.Printf("[OVERLAY] SetLayeredWindowAttributes(alpha=255): ret=%d err=%v", ret2, err)

	// 9. Create WebView2 (Chromium)
	chromium := edge.NewChromium()

	// Use a dedicated data path so it doesn't conflict with the main Wails WebView2
	exePath, _ := exec.LookPath("buffer-sharer-app.exe")
	if exePath == "" {
		exePath = "buffer-sharer-overlay"
	}
	chromium.DataPath = filepath.Join(filepath.Dir(exePath), "overlay_webview2_data")
	log.Printf("[OVERLAY] Chromium DataPath: %s", chromium.DataPath)

	log.Println("[OVERLAY] Calling Chromium.Embed()...")
	if !chromium.Embed(hwnd) {
		log.Println("[OVERLAY] ERROR: Chromium.Embed() returned false! Destroying orphan window.")
		procDestroyWindow.Call(hwnd)
		close(m.ready)
		return
	}
	log.Println("[OVERLAY] Chromium.Embed() completed successfully")

	// 10. Transparent background (A=0 → fully transparent)
	chromium.SetBackgroundColour(0, 0, 0, 0)
	log.Println("[OVERLAY] Background colour set to (0,0,0,0) — transparent")

	// 11. Resize WebView2 to fill the overlay window
	chromium.Resize()
	log.Println("[OVERLAY] Chromium.Resize() done")

	// 12. Set up JS→Go message callback
	chromium.MessageCallback = func(message string, sender *edge.ICoreWebView2, args *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		m.handleWebMessage(message)
	}
	log.Println("[OVERLAY] MessageCallback registered")

	// 13. Inject init script for __evalCallback (EvalJSWithResult pattern)
	chromium.Init(`
		window.__evalCallback = function(id, result) {
			window.chrome.webview.postMessage(JSON.stringify({
				type: "eval_result",
				id: id,
				result: String(result)
			}));
		};
	`)
	log.Println("[OVERLAY] Init script injected (__evalCallback)")

	// 14. Load overlay HTML
	log.Printf("[OVERLAY] Loading overlay HTML, length=%d", len(overlayHTML))
	chromium.NavigateToString(overlayHTML)
	log.Println("[OVERLAY] NavigateToString called")

	// 15. Cache DPI scale for coordinate conversion (GetCursorPos → CSS pixels)
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		dpi = 96 // fallback: 100% scaling
	}
	dpiScale := float64(dpi) / 96.0
	log.Printf("[OVERLAY] DPI=%d scale=%.2f", dpi, dpiScale)

	// 16. Store state
	atomic.StoreUintptr(&m.hwnd, hwnd)
	m.mu.Lock()
	m.chromium = chromium
	m.dpiScale = dpiScale
	m.created = true
	m.visible = true
	m.mu.Unlock()

	// 16. Show window and ensure topmost
	procShowWindow.Call(hwnd, _SW_SHOW)
	procSetWindowPos.Call(hwnd, _HWND_TOPMOST, 0, 0, 0, 0,
		_SWP_NOMOVE|_SWP_NOSIZE|_SWP_NOACTIVATE|_SWP_SHOWWINDOW)
	log.Println("[OVERLAY] ShowWindow + SetWindowPos(TOPMOST) done")

	// 17. Get thread ID (for diagnostics)
	tid, _, _ := procGetCurrentThreadId.Call()
	log.Printf("[OVERLAY] Overlay thread ID: %d", tid)

	// Signal ready
	close(m.ready)
	log.Println("[OVERLAY] === OVERLAY READY ===")

	// 18. Enter Win32 message loop
	var msg _MSG
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if ret == 0 || ret == ^uintptr(0) {
			log.Printf("[OVERLAY] Message loop exit: ret=%d", ret)
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	log.Println("[OVERLAY] overlayThread exiting")
}

// ---------------------------------------------------------------------------
// wndProcHandler — Win32 window procedure (runs on overlay thread)
// ---------------------------------------------------------------------------

func (m *Manager) wndProcHandler(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case _WM_SIZE:
		w := int(lparam & 0xFFFF)
		h := int(lparam >> 16)
		log.Printf("[OVERLAY] WM_SIZE: %dx%d", w, h)
		m.mu.RLock()
		c := m.chromium
		m.mu.RUnlock()
		if c != nil {
			c.Resize()
		}
		return 0

	case _WM_OVERLAY_EVALJS:
		id := uint64(wparam)
		m.evalMu.Lock()
		js, ok := m.evalQueue[id]
		delete(m.evalQueue, id)
		m.evalMu.Unlock()
		if ok {
			m.mu.RLock()
			c := m.chromium
			m.mu.RUnlock()
			if c != nil {
				preview := js
				if len(preview) > 120 {
					preview = preview[:120]
				}
				log.Printf("[OVERLAY-JS] EvalJS #%d: %s", id, preview)
				c.Eval(js)
			} else {
				log.Printf("[OVERLAY-JS] EvalJS #%d: ERROR chromium is nil", id)
			}
		}
		return 0

	case _WM_OVERLAY_EVALJS_RESULT:
		id := uint64(wparam)
		m.evalMu.Lock()
		js, ok := m.evalQueue[id]
		delete(m.evalQueue, id)
		m.evalMu.Unlock()
		if ok {
			m.mu.RLock()
			c := m.chromium
			m.mu.RUnlock()
			if c != nil {
				log.Printf("[OVERLAY-JS] EvalJSWithResult #%d posting to webview", id)
				c.Eval(js)
			} else {
				log.Printf("[OVERLAY-JS] EvalJSWithResult #%d: ERROR chromium is nil", id)
			}
		}
		return 0

	case _WM_OVERLAY_SHOW:
		log.Println("[OVERLAY] WM_OVERLAY_SHOW")
		procShowWindow.Call(hwnd, _SW_SHOW)
		procSetWindowPos.Call(hwnd, _HWND_TOPMOST, 0, 0, 0, 0,
			_SWP_NOMOVE|_SWP_NOSIZE|_SWP_NOACTIVATE|_SWP_SHOWWINDOW)
		return 0

	case _WM_OVERLAY_HIDE:
		log.Println("[OVERLAY] WM_OVERLAY_HIDE")
		procShowWindow.Call(hwnd, _SW_HIDE)
		return 0

	case _WM_OVERLAY_DESTROY:
		log.Println("[OVERLAY] WM_OVERLAY_DESTROY received")
		m.mu.RLock()
		c := m.chromium
		m.mu.RUnlock()
		if c != nil {
			c.ShuttingDown()
		}
		procDestroyWindow.Call(hwnd)
		return 0

	case _WM_OVERLAY_SET_CLICKTHROUGH:
		enable := wparam != 0
		log.Printf("[OVERLAY] WM_OVERLAY_SET_CLICKTHROUGH: enable=%v", enable)
		style, _, _ := procGetWindowLongPtrW.Call(hwnd, _GWL_EXSTYLE)
		oldStyle := style
		if enable {
			style |= _WS_EX_TRANSPARENT
		} else {
			style &^= _WS_EX_TRANSPARENT
		}
		if style != oldStyle {
			procSetWindowLongPtrW.Call(hwnd, _GWL_EXSTYLE, style)
			log.Printf("[OVERLAY] ExStyle changed: 0x%x -> 0x%x", oldStyle, style)
		} else {
			log.Printf("[OVERLAY] ExStyle unchanged: 0x%x", style)
		}
		return 0

	case _WM_DESTROY:
		log.Println("[OVERLAY] WM_DESTROY — posting WM_QUIT")
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

// ---------------------------------------------------------------------------
// postToOverlay — sends a custom message to the overlay window
// ---------------------------------------------------------------------------

func (m *Manager) postToOverlay(msg uintptr, wparam, lparam uintptr) {
	hwnd := atomic.LoadUintptr(&m.hwnd)
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, msg, wparam, lparam)
	}
}

// ---------------------------------------------------------------------------
// handleWebMessage — processes messages from JS (via window.chrome.webview.postMessage)
// ---------------------------------------------------------------------------

func (m *Manager) handleWebMessage(message string) {
	if len(message) > 200 {
		log.Printf("[OVERLAY-MSG] Received (truncated): %.200s...", message)
	} else {
		log.Printf("[OVERLAY-MSG] Received: %s", message)
	}

	var parsed struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(message), &parsed); err != nil {
		log.Printf("[OVERLAY-MSG] JSON parse error: %v (raw=%.100s)", err, message)
		return
	}

	if parsed.Type == "eval_result" {
		m.resultMu.Lock()
		ch, ok := m.resultChans[parsed.ID]
		if ok {
			delete(m.resultChans, parsed.ID)
		}
		m.resultMu.Unlock()

		if ok {
			// Non-blocking send: if caller already timed out, the channel buffer (size 1)
			// absorbs the value and it gets GC'd. No goroutine leak or panic.
			select {
			case ch <- parsed.Result:
				log.Printf("[OVERLAY-MSG] Delivered result for id=%s len=%d", parsed.ID, len(parsed.Result))
			default:
				log.Printf("[OVERLAY-MSG] Result for id=%s dropped (caller timed out)", parsed.ID)
			}
		} else {
			log.Printf("[OVERLAY-MSG] No waiting channel for eval_result id=%s (already timed out)", parsed.ID)
		}
	} else {
		log.Printf("[OVERLAY-MSG] Unknown message type: %s", parsed.Type)
	}
}

// ---------------------------------------------------------------------------
// Show makes the overlay window visible
// ---------------------------------------------------------------------------

func (m *Manager) Show() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.created {
		log.Println("[OVERLAY] Show() called")
		m.visible = true
		m.postToOverlay(_WM_OVERLAY_SHOW, 0, 0)
	}
}

// ---------------------------------------------------------------------------
// Hide hides the overlay window
// ---------------------------------------------------------------------------

func (m *Manager) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.created {
		log.Println("[OVERLAY] Hide() called")
		m.visible = false
		m.postToOverlay(_WM_OVERLAY_HIDE, 0, 0)
	}
}

// ---------------------------------------------------------------------------
// IsVisible returns whether the overlay is currently visible
// ---------------------------------------------------------------------------

func (m *Manager) IsVisible() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visible
}

// ---------------------------------------------------------------------------
// EvalJS executes JavaScript in the overlay WebView (fire-and-forget)
// ---------------------------------------------------------------------------

func (m *Manager) EvalJS(js string) {
	m.mu.RLock()
	if !m.created {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	m.evalMu.Lock()
	m.evalCounter++
	id := m.evalCounter
	m.evalQueue[id] = js
	m.evalMu.Unlock()

	m.postToOverlay(_WM_OVERLAY_EVALJS, uintptr(id), 0)
}

// ---------------------------------------------------------------------------
// EvalJSWithResult executes JavaScript and returns the result string (blocks, 2s timeout)
// ---------------------------------------------------------------------------

func (m *Manager) EvalJSWithResult(js string) string {
	m.mu.RLock()
	if !m.created {
		m.mu.RUnlock()
		return ""
	}
	m.mu.RUnlock()

	// Generate unique result ID
	m.resultMu.Lock()
	m.resultCounter++
	resultID := fmt.Sprintf("er_%d", m.resultCounter)
	ch := make(chan string, 1)
	m.resultChans[resultID] = ch
	m.resultMu.Unlock()

	// Wrap JS to send result via __evalCallback → postMessage → handleWebMessage
	wrappedJS := fmt.Sprintf(
		`(function(){ try { var __r = (%s); window.__evalCallback("%s", String(__r)); } catch(__e) { window.__evalCallback("%s", "ERROR:" + __e.message); } })()`,
		js, resultID, resultID,
	)

	// Queue wrapped JS and post to overlay thread
	m.evalMu.Lock()
	m.evalCounter++
	queueID := m.evalCounter
	m.evalQueue[queueID] = wrappedJS
	m.evalMu.Unlock()

	log.Printf("[OVERLAY-JS] EvalJSWithResult: resultID=%s queueID=%d jsLen=%d", resultID, queueID, len(js))
	m.postToOverlay(_WM_OVERLAY_EVALJS_RESULT, uintptr(queueID), 0)

	// Wait for result with timeout
	select {
	case result := <-ch:
		log.Printf("[OVERLAY-JS] EvalJSWithResult %s: got result len=%d", resultID, len(result))
		return result
	case <-time.After(2 * time.Second):
		log.Printf("[OVERLAY-JS] EvalJSWithResult %s: TIMEOUT (2s)", resultID)
		m.resultMu.Lock()
		delete(m.resultChans, resultID)
		m.resultMu.Unlock()
		return ""
	}
}

// ---------------------------------------------------------------------------
// Destroy closes and cleans up the overlay window
// ---------------------------------------------------------------------------

func (m *Manager) Destroy() {
	m.mu.Lock()
	if !m.created {
		m.mu.Unlock()
		return
	}
	log.Println("[OVERLAY] Destroy() called")

	// Stop re-order goroutine
	if m.reorderStop != nil {
		close(m.reorderStop)
		m.reorderStop = nil
	}
	// Stop hint interaction timer
	if m.hintTimer != nil {
		m.hintTimer.Stop()
	}
	if m.hintTimerStop != nil {
		close(m.hintTimerStop)
		m.hintTimerStop = nil
	}

	m.created = false
	m.visible = false
	m.mu.Unlock()

	// Wait for background goroutines (reorder, hint interaction) to finish
	m.wg.Wait()

	// Cleanup eval queues to prevent leaks
	m.evalMu.Lock()
	m.evalQueue = make(map[uint64]string)
	m.evalMu.Unlock()

	m.resultMu.Lock()
	for id, ch := range m.resultChans {
		close(ch)
		delete(m.resultChans, id)
	}
	m.resultMu.Unlock()

	m.postToOverlay(_WM_OVERLAY_DESTROY, 0, 0)
}

// ---------------------------------------------------------------------------
// IsSupported returns true on Windows
// ---------------------------------------------------------------------------

func (m *Manager) IsSupported() bool {
	return true
}

// ---------------------------------------------------------------------------
// GetWindowNumber returns the overlay HWND as int (for exclusion from invisibility)
// ---------------------------------------------------------------------------

func (m *Manager) GetWindowNumber() int {
	hwnd := atomic.LoadUintptr(&m.hwnd)
	return int(hwnd)
}

// ---------------------------------------------------------------------------
// SetIgnoresMouseEvents toggles click-through (WS_EX_TRANSPARENT)
// ---------------------------------------------------------------------------

func (m *Manager) SetIgnoresMouseEvents(ignores bool) {
	m.mu.RLock()
	if !m.created {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	val := uintptr(0)
	if ignores {
		val = 1
	}
	log.Printf("[OVERLAY] SetIgnoresMouseEvents: ignores=%v", ignores)
	m.postToOverlay(_WM_OVERLAY_SET_CLICKTHROUGH, val, 0)
}

// ---------------------------------------------------------------------------
// GetMouseLocation returns current mouse position in screen pixels (top-left origin)
// ---------------------------------------------------------------------------

func (m *Manager) GetMouseLocation() (x, y float64) {
	var pt _POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// GetCursorPos returns physical pixels; divide by DPI scale to get CSS pixels
	// (overlay HTML coordinates are in CSS pixels)
	scale := m.dpiScale
	if scale < 1.0 {
		scale = 1.0
	}
	return float64(pt.X) / scale, float64(pt.Y) / scale
}

// ---------------------------------------------------------------------------
// GetScreenSize returns the primary screen size in pixels
// ---------------------------------------------------------------------------

func (m *Manager) GetScreenSize() (w, h float64) {
	cx, _, _ := procGetSystemMetrics.Call(_SM_CXSCREEN)
	cy, _, _ := procGetSystemMetrics.Call(_SM_CYSCREEN)
	return float64(cx), float64(cy)
}

// ---------------------------------------------------------------------------
// DiagnosticCheck verifies the overlay is actually working
// ---------------------------------------------------------------------------

func (m *Manager) DiagnosticCheck() (visible bool, jsWorks bool, windowInfo string) {
	m.mu.RLock()
	created := m.created
	vis := m.visible
	m.mu.RUnlock()

	if !created {
		return false, false, "not created"
	}

	visible = vis
	hwnd := atomic.LoadUintptr(&m.hwnd)

	// Test if JS executes and check rendering details
	result := m.EvalJSWithResult(`(function(){
		var dpr = window.devicePixelRatio || 1;
		var c = document.getElementById('draw-canvas');
		var cw = c ? c.width : -1;
		var ch = c ? c.height : -1;
		var layers = document.querySelectorAll('[id^=layer]').length;
		return "overlay_ok_" + window.innerWidth + "x" + window.innerHeight + "_dpr" + dpr + "_canvas" + cw + "x" + ch + "_layers" + layers;
	})()`)
	jsWorks = len(result) > 0 && result != ""

	windowInfo = fmt.Sprintf("hwnd=0x%x visible=%v jsResult=%s", hwnd, visible, result)
	log.Printf("[OVERLAY] DiagnosticCheck: %s", windowInfo)
	return
}

// ---------------------------------------------------------------------------
// UpdateHintRect updates the bounding rect for a hint element
// ---------------------------------------------------------------------------

func (m *Manager) UpdateHintRect(id string, rect HintRect) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hintRects[id] = rect
}

// ---------------------------------------------------------------------------
// RemoveHintRect removes a hint rect
// ---------------------------------------------------------------------------

func (m *Manager) RemoveHintRect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hintRects, id)
}

// ---------------------------------------------------------------------------
// UpdateTextRect updates the bounding rect for a text overlay element
// ---------------------------------------------------------------------------

func (m *Manager) UpdateTextRect(id string, rect HintRect) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.textRects[id] = rect
}

// ---------------------------------------------------------------------------
// RemoveTextRect removes a text overlay rect
// ---------------------------------------------------------------------------

func (m *Manager) RemoveTextRect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.textRects, id)
}

// ---------------------------------------------------------------------------
// SetOnAction sets the callback for overlay actions (from JS→Go polling)
// ---------------------------------------------------------------------------

func (m *Manager) SetOnAction(fn func(action string, actionType string, id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAction = fn
}

// ---------------------------------------------------------------------------
// StartHintInteraction starts the goroutine that polls mouse position
// and toggles click-through based on hint hover
// ---------------------------------------------------------------------------

func (m *Manager) StartHintInteraction() {
	m.mu.Lock()
	if m.hintTimer != nil {
		m.mu.Unlock()
		return
	}
	m.hintTimer = time.NewTicker(50 * time.Millisecond)
	m.hintTimerStop = make(chan struct{})
	ticker := m.hintTimer
	stop := m.hintTimerStop
	m.mu.Unlock()

	log.Println("[OVERLAY] StartHintInteraction: started (50ms mouse poll + 200ms action poll)")

	// Separate ticker for JS→Go action polling (200ms)
	actionTicker := time.NewTicker(200 * time.Millisecond)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer actionTicker.Stop()
		for {
			select {
			case <-stop:
				log.Println("[OVERLAY] StartHintInteraction: stopped")
				return
			case <-ticker.C:
				m.checkMouseOverHints()
			case <-actionTicker.C:
				m.pollPendingActions()
			}
		}
	}()
}

// ---------------------------------------------------------------------------
// StopHintInteraction stops the hint interaction goroutine
// ---------------------------------------------------------------------------

func (m *Manager) StopHintInteraction() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hintTimer != nil {
		m.hintTimer.Stop()
		m.hintTimer = nil
	}
	if m.hintTimerStop != nil {
		close(m.hintTimerStop)
		m.hintTimerStop = nil
	}
}

// ---------------------------------------------------------------------------
// checkMouseOverHints — polls mouse position and toggles click-through
// ---------------------------------------------------------------------------

func (m *Manager) checkMouseOverHints() {
	mx, my := m.GetMouseLocation()

	m.mu.RLock()
	overAny := false
	for _, r := range m.hintRects {
		if mx >= r.X && mx <= r.X+r.W && my >= r.Y && my <= r.Y+r.H {
			overAny = true
			break
		}
	}
	if !overAny {
		for _, r := range m.textRects {
			if mx >= r.X && mx <= r.X+r.W && my >= r.Y && my <= r.Y+r.H {
				overAny = true
				break
			}
		}
	}
	wasOver := m.mouseOverHint
	m.mu.RUnlock()

	if overAny != wasOver {
		m.mu.Lock()
		m.mouseOverHint = overAny
		m.mu.Unlock()
		m.SetIgnoresMouseEvents(!overAny)
	}
}

// ---------------------------------------------------------------------------
// pollPendingActions — polls JS _pendingActions array
// ---------------------------------------------------------------------------

type pendingAction struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	ID     string `json:"id"`
}

func (m *Manager) pollPendingActions() {
	result := m.EvalJSWithResult(`(function(){ var a = window._pendingActions || []; window._pendingActions = []; return JSON.stringify(a); })()`)
	if result == "" || result == "[]" {
		return
	}

	var actions []pendingAction
	if err := json.Unmarshal([]byte(result), &actions); err != nil {
		log.Printf("[OVERLAY] pollPendingActions: JSON error: %v", err)
		return
	}

	m.mu.RLock()
	cb := m.onAction
	m.mu.RUnlock()

	for _, a := range actions {
		log.Printf("[OVERLAY] Action from JS: action=%s type=%s id=%s", a.Action, a.Type, a.ID)
		if cb != nil {
			cb(a.Action, a.Type, a.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// SyncHintRects fetches current hint/text bounding rects from JS
// ---------------------------------------------------------------------------

func (m *Manager) SyncHintRects() {
	result := m.EvalJSWithResult(`(function(){ return typeof getHintRects === 'function' ? getHintRects() : '{}'; })()`)
	if result == "" || result == "{}" {
		return
	}

	var data struct {
		Hints map[string]struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"hints"`
		Texts map[string]struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"texts"`
	}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		log.Printf("[OVERLAY] SyncHintRects: JSON error: %v result=%.100s", err, result)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update hint rects
	for id, r := range data.Hints {
		m.hintRects[id] = HintRect{ID: id, X: r.X, Y: r.Y, W: r.W, H: r.H}
	}
	// Remove rects that no longer exist in JS
	for id := range m.hintRects {
		if _, ok := data.Hints[id]; !ok {
			delete(m.hintRects, id)
		}
	}

	// Update text rects
	for id, r := range data.Texts {
		m.textRects[id] = HintRect{ID: id, X: r.X, Y: r.Y, W: r.W, H: r.H}
	}
	for id := range m.textRects {
		if _, ok := data.Texts[id]; !ok {
			delete(m.textRects, id)
		}
	}
}
