//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework CoreGraphics

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static NSWindow *overlayWindow = nil;
static WKWebView *overlayWebView = nil;

void CreateOverlayWindow(const char *htmlContent) {
	// Copy the string BEFORE dispatch_async, because the caller frees it via defer
	char *htmlCopy = strdup(htmlContent);
	dispatch_async(dispatch_get_main_queue(), ^{
		// htmlCopy is owned by this block; free it when done
		if (overlayWindow != nil) {
			NSLog(@"[OVERLAY] CreateOverlayWindow: already exists, skipping");
			free(htmlCopy);
			return;
		}

		NSLog(@"[OVERLAY] === CREATING OVERLAY WINDOW ===");

		// Screen info
		NSArray *screens = [NSScreen screens];
		NSLog(@"[OVERLAY] Number of screens: %lu", (unsigned long)screens.count);
		for (NSUInteger i = 0; i < screens.count; i++) {
			NSScreen *s = screens[i];
			NSRect sf = s.frame;
			NSRect vf = s.visibleFrame;
			NSLog(@"[OVERLAY] Screen %lu: frame=%.0fx%.0f@(%.0f,%.0f) visible=%.0fx%.0f@(%.0f,%.0f) scale=%.1f",
				(unsigned long)i, sf.size.width, sf.size.height, sf.origin.x, sf.origin.y,
				vf.size.width, vf.size.height, vf.origin.x, vf.origin.y,
				s.backingScaleFactor);
		}

		NSScreen *screen = [NSScreen mainScreen];
		NSRect frame = screen.frame;
		NSLog(@"[OVERLAY] Using mainScreen: %.0fx%.0f at (%.0f,%.0f) scale=%.1f",
			frame.size.width, frame.size.height, frame.origin.x, frame.origin.y,
			screen.backingScaleFactor);

		overlayWindow = [[NSWindow alloc]
			initWithContentRect:frame
			styleMask:NSWindowStyleMaskBorderless
			backing:NSBackingStoreBuffered
			defer:NO];

		if (overlayWindow == nil) {
			NSLog(@"[OVERLAY] ERROR: Failed to create NSWindow!");
			return;
		}

		NSLog(@"[OVERLAY] NSWindow created, windowNumber=%ld", (long)[overlayWindow windowNumber]);

		// Use a high but not extreme window level
		[overlayWindow setLevel:NSScreenSaverWindowLevel + 100];
		NSLog(@"[OVERLAY] Window level set to %ld (NSScreenSaverWindowLevel+100=%ld)",
			(long)[overlayWindow level], (long)(NSScreenSaverWindowLevel + 100));

		[overlayWindow setBackgroundColor:[NSColor clearColor]];
		[overlayWindow setOpaque:NO];
		[overlayWindow setHasShadow:NO];
		[overlayWindow setIgnoresMouseEvents:YES];
		[overlayWindow setCollectionBehavior:
			NSWindowCollectionBehaviorCanJoinAllSpaces |
			NSWindowCollectionBehaviorStationary |
			NSWindowCollectionBehaviorFullScreenAuxiliary];
		[overlayWindow setReleasedWhenClosed:NO];
		NSLog(@"[OVERLAY] Window properties set: opaque=%d hasShadow=%d ignoresMouse=%d",
			[overlayWindow isOpaque], [overlayWindow hasShadow], [overlayWindow ignoresMouseEvents]);

		// Content view info
		NSView *contentView = overlayWindow.contentView;
		NSRect cvBounds = contentView.bounds;
		NSLog(@"[OVERLAY] ContentView bounds: %.0fx%.0f wantsLayer=%d",
			cvBounds.size.width, cvBounds.size.height, contentView.wantsLayer);

		// Ensure content view is layer-backed for proper compositing
		[contentView setWantsLayer:YES];
		contentView.layer.backgroundColor = [NSColor clearColor].CGColor;
		NSLog(@"[OVERLAY] ContentView: wantsLayer=%d layer=%p", contentView.wantsLayer, contentView.layer);

		// Create WKWebView
		WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
		#pragma clang diagnostic push
		#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		config.preferences.javaScriptEnabled = YES;
		#pragma clang diagnostic pop
		NSLog(@"[OVERLAY] WKWebViewConfiguration created, JS enabled");

		overlayWebView = [[WKWebView alloc]
			initWithFrame:cvBounds
			configuration:config];

		if (overlayWebView == nil) {
			NSLog(@"[OVERLAY] ERROR: Failed to create WKWebView!");
			[config release];
			free(htmlCopy);
			return;
		}
		// config is retained by WKWebView internally; release our ownership
		[config release];

		NSLog(@"[OVERLAY] WKWebView created, frame: %.0fx%.0f", cvBounds.size.width, cvBounds.size.height);

		[overlayWebView setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
		[overlayWebView setWantsLayer:YES];
		NSLog(@"[OVERLAY] WKWebView wantsLayer=%d layer=%p", overlayWebView.wantsLayer, overlayWebView.layer);

		// Transparent background methods
		[overlayWebView setValue:@(NO) forKey:@"drawsBackground"];
		NSLog(@"[OVERLAY] drawsBackground set to NO");

		SEL transpSel = NSSelectorFromString(@"_setDrawsTransparentBackground:");
		if ([overlayWebView respondsToSelector:transpSel]) {
			typedef void (*TranspIMP)(id, SEL, BOOL);
			TranspIMP imp = (TranspIMP)[overlayWebView methodForSelector:transpSel];
			imp(overlayWebView, transpSel, YES);
			NSLog(@"[OVERLAY] _setDrawsTransparentBackground: YES applied");
		} else {
			NSLog(@"[OVERLAY] _setDrawsTransparentBackground NOT available");
		}

		[contentView addSubview:overlayWebView];
		NSLog(@"[OVERLAY] WKWebView added to contentView, subviews count=%lu",
			(unsigned long)contentView.subviews.count);

		// Load HTML content (use htmlCopy which is owned by this block)
		NSString *html = [NSString stringWithUTF8String:htmlCopy];
		NSLog(@"[OVERLAY] Loading HTML, length=%lu", (unsigned long)html.length);
		free(htmlCopy); // no longer needed after NSString is created
		[overlayWebView loadHTMLString:html baseURL:nil];

		[overlayWindow orderFrontRegardless];
		NSLog(@"[OVERLAY] orderFrontRegardless called");

		// Full window state dump
		NSRect wFrame = [overlayWindow frame];
		NSLog(@"[OVERLAY] === WINDOW STATE ===");
		NSLog(@"[OVERLAY] visible=%d level=%ld alpha=%.2f",
			[overlayWindow isVisible], (long)[overlayWindow level], [overlayWindow alphaValue]);
		NSLog(@"[OVERLAY] frame=%.0fx%.0f@(%.0f,%.0f)",
			wFrame.size.width, wFrame.size.height, wFrame.origin.x, wFrame.origin.y);
		NSLog(@"[OVERLAY] opaque=%d hasShadow=%d ignoresMouse=%d",
			[overlayWindow isOpaque], [overlayWindow hasShadow], [overlayWindow ignoresMouseEvents]);
		NSLog(@"[OVERLAY] windowNumber=%ld onActiveSpace=%d",
			(long)[overlayWindow windowNumber], [overlayWindow isOnActiveSpace]);

		// Verify WKWebView state
		NSRect wvFrame = [overlayWebView frame];
		NSLog(@"[OVERLAY] WKWebView frame=%.0fx%.0f@(%.0f,%.0f) hidden=%d alpha=%.2f",
			wvFrame.size.width, wvFrame.size.height, wvFrame.origin.x, wvFrame.origin.y,
			overlayWebView.hidden, overlayWebView.alphaValue);
		NSLog(@"[OVERLAY] WKWebView loading=%d title=%@",
			overlayWebView.loading, overlayWebView.title);

		// Re-order after delays to ensure it stays on top
		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(1.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
			if (overlayWindow != nil) {
				[overlayWindow orderFrontRegardless];
				NSLog(@"[OVERLAY] Re-ordered +1s: visible=%d level=%ld onActiveSpace=%d alpha=%.2f",
					[overlayWindow isVisible], (long)[overlayWindow level],
					[overlayWindow isOnActiveSpace], [overlayWindow alphaValue]);

				// Check WKWebView state after HTML should have loaded
				NSLog(@"[OVERLAY] WKWebView +1s: loading=%d hidden=%d frame=%.0fx%.0f",
					overlayWebView.loading, overlayWebView.hidden,
					overlayWebView.frame.size.width, overlayWebView.frame.size.height);
			}
		});

		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(3.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
			if (overlayWindow != nil && overlayWebView != nil) {
				NSLog(@"[OVERLAY] +3s check: window visible=%d webview loading=%d hidden=%d",
					[overlayWindow isVisible], overlayWebView.loading, overlayWebView.hidden);
				// Try evaluating JS to verify WebView is alive
				[overlayWebView evaluateJavaScript:@"document.body ? document.body.innerHTML.length : -1"
					completionHandler:^(id result, NSError *error) {
						if (error) {
							NSLog(@"[OVERLAY] +3s JS error: %@", error.localizedDescription);
						} else {
							NSLog(@"[OVERLAY] +3s JS body.innerHTML.length=%@", result);
						}
					}];

				// Check all NSApplication windows
				NSArray *allWindows = [[NSApplication sharedApplication] windows];
				NSLog(@"[OVERLAY] +3s Total app windows: %lu", (unsigned long)allWindows.count);
				for (NSUInteger i = 0; i < allWindows.count; i++) {
					NSWindow *w = allWindows[i];
					NSLog(@"[OVERLAY] +3s Window[%lu]: level=%ld visible=%d frame=%.0fx%.0f title=%@ windowNumber=%ld",
						(unsigned long)i, (long)w.level, w.isVisible,
						w.frame.size.width, w.frame.size.height,
						w.title ?: @"(nil)", (long)w.windowNumber);
				}
			}
		});
	});
}

void DestroyOverlayWindow() {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (overlayWindow != nil) {
			[overlayWindow close];
			overlayWebView = nil;
			overlayWindow = nil;
		}
	});
}

void ShowOverlayWindow() {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (overlayWindow != nil) {
			[overlayWindow orderFrontRegardless];
		}
	});
}

void HideOverlayWindow() {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (overlayWindow != nil) {
			[overlayWindow orderOut:nil];
		}
	});
}

int IsOverlayVisible() {
	if (overlayWindow != nil) {
		return [overlayWindow isVisible] ? 1 : 0;
	}
	return 0;
}

static int evalJSCounter = 0;

void OverlayEvalJS(const char *js) {
	// Copy string before dispatch_async — caller frees original via defer
	char *jsCopy = strdup(js);
	dispatch_async(dispatch_get_main_queue(), ^{
		evalJSCounter++;
		if (overlayWebView == nil) {
			NSLog(@"[OVERLAY-JS] #%d ERROR: overlayWebView is nil!", evalJSCounter);
			free(jsCopy);
			return;
		}
		NSString *script = [NSString stringWithUTF8String:jsCopy];
		free(jsCopy);

		// Log first 120 chars of script
		NSString *preview = script.length > 120 ? [script substringToIndex:120] : script;
		NSLog(@"[OVERLAY-JS] #%d eval: %@", evalJSCounter, preview);

		[overlayWebView evaluateJavaScript:script completionHandler:^(id result, NSError *error) {
			if (error) {
				NSLog(@"[OVERLAY-JS] #%d ERROR: %@", evalJSCounter, error.localizedDescription);
			}
		}];
	});
}

int GetOverlayWindowNumber() {
	if (overlayWindow != nil) {
		return (int)[overlayWindow windowNumber];
	}
	return 0;
}

void SetOverlayIgnoresMouseEvents(int ignores) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (overlayWindow != nil) {
			[overlayWindow setIgnoresMouseEvents:(ignores ? YES : NO)];
		}
	});
}

void GetMouseLocation(double *outX, double *outY) {
	NSPoint p = [NSEvent mouseLocation];
	NSScreen *screen = [NSScreen mainScreen];
	*outX = p.x;
	*outY = screen.frame.size.height - p.y;
}

void GetScreenSize(double *outW, double *outH) {
	NSScreen *screen = [NSScreen mainScreen];
	*outW = screen.frame.size.width;
	*outH = screen.frame.size.height;
}

// JS eval with result callback — uses a semaphore to synchronize
static char *jsResultBuffer = NULL;
static dispatch_semaphore_t jsResultSema = NULL;

void initJSResultSema() {
	if (jsResultSema == NULL) {
		jsResultSema = dispatch_semaphore_create(0);
	}
}

void OverlayEvalJSWithResult(const char *js) {
	initJSResultSema();
	// Copy string before dispatch_async — caller may free original on semaphore timeout
	char *jsCopy = strdup(js);
	dispatch_async(dispatch_get_main_queue(), ^{
		if (overlayWebView != nil) {
			NSString *script = [NSString stringWithUTF8String:jsCopy];
			free(jsCopy);
			[overlayWebView evaluateJavaScript:script completionHandler:^(id result, NSError *error) {
				if (jsResultBuffer != NULL) {
					free(jsResultBuffer);
					jsResultBuffer = NULL;
				}
				if (result && !error) {
					NSString *str = [NSString stringWithFormat:@"%@", result];
					const char *utf8 = [str UTF8String];
					jsResultBuffer = strdup(utf8);
				} else {
					jsResultBuffer = strdup("");
				}
				dispatch_semaphore_signal(jsResultSema);
			}];
		} else {
			free(jsCopy);
			if (jsResultBuffer != NULL) {
				free(jsResultBuffer);
			}
			jsResultBuffer = strdup("");
			dispatch_semaphore_signal(jsResultSema);
		}
	});
}

const char* WaitJSResult() {
	initJSResultSema();
	dispatch_semaphore_wait(jsResultSema, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
	return jsResultBuffer ? jsResultBuffer : "";
}
*/
import "C"

import (
	"encoding/json"
	"sync"
	"time"
	"unsafe"
)

// HintRect represents the bounding rectangle of an interactive element on the overlay
type HintRect struct {
	ID        string
	X, Y, W, H float64
	Collapsed bool
}

// Manager handles the overlay window on macOS
type Manager struct {
	mu            sync.RWMutex
	created       bool
	visible       bool

	hintRects     map[string]HintRect
	textRects     map[string]HintRect
	mouseOverHint bool
	hintTimer     *time.Ticker
	hintTimerStop chan struct{}

	reorderStop   chan struct{}

	// WaitGroup for background goroutines so Destroy can wait
	wg sync.WaitGroup

	// Mutex to serialize EvalJSWithResult calls (shared C semaphore/buffer)
	evalResultMu sync.Mutex

	// Action callback
	onAction func(action string, actionType string, id string)
}

// NewManager creates overlay manager and immediately creates the overlay window
func NewManager() *Manager {
	m := &Manager{
		hintRects: make(map[string]HintRect),
		textRects: make(map[string]HintRect),
	}

	cHTML := C.CString(overlayHTML)
	defer C.free(unsafe.Pointer(cHTML))
	C.CreateOverlayWindow(cHTML)

	m.created = true
	m.visible = true

	// Periodically re-order overlay to stay on top
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
					C.ShowOverlayWindow()
				}
			}
		}
	}()

	return m
}

// Show makes the overlay window visible
func (m *Manager) Show() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.created {
		C.ShowOverlayWindow()
		m.visible = true
	}
}

// Hide hides the overlay window
func (m *Manager) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.created {
		C.HideOverlayWindow()
		m.visible = false
	}
}

// IsVisible returns whether the overlay is currently visible
func (m *Manager) IsVisible() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visible
}

// EvalJS executes JavaScript in the overlay WebView
func (m *Manager) EvalJS(js string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.created {
		cJS := C.CString(js)
		defer C.free(unsafe.Pointer(cJS))
		C.OverlayEvalJS(cJS)
	}
}

// DiagnosticCheck verifies the overlay is actually working
func (m *Manager) DiagnosticCheck() (visible bool, jsWorks bool, windowInfo string) {
	m.mu.RLock()
	created := m.created
	m.mu.RUnlock()

	if !created {
		return false, false, "not created"
	}

	vis := int(C.IsOverlayVisible())
	visible = vis == 1

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

	visStr := "false"
	if visible {
		visStr = "true"
	}
	windowInfo = "visible=" + visStr + " jsResult=" + result
	return
}

// Destroy closes and cleans up the overlay window
func (m *Manager) Destroy() {
	m.mu.Lock()
	if !m.created {
		m.mu.Unlock()
		return
	}
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

	// Wait for background goroutines to finish before destroying the window
	m.wg.Wait()

	C.DestroyOverlayWindow()
}

// IsSupported returns true on macOS
func (m *Manager) IsSupported() bool {
	return true
}

// GetWindowNumber returns the overlay window number (for exclusion from invisibility)
func (m *Manager) GetWindowNumber() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.created {
		return int(C.GetOverlayWindowNumber())
	}
	return 0
}

// SetIgnoresMouseEvents sets whether the overlay window ignores mouse events
func (m *Manager) SetIgnoresMouseEvents(ignores bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.created {
		v := C.int(0)
		if ignores {
			v = 1
		}
		C.SetOverlayIgnoresMouseEvents(v)
	}
}

// GetMouseLocation returns the current mouse position in screen pixels (top-left origin)
func (m *Manager) GetMouseLocation() (x, y float64) {
	var cx, cy C.double
	C.GetMouseLocation(&cx, &cy)
	return float64(cx), float64(cy)
}

// GetScreenSize returns the main screen size in pixels
func (m *Manager) GetScreenSize() (w, h float64) {
	var cw, ch C.double
	C.GetScreenSize(&cw, &ch)
	return float64(cw), float64(ch)
}

// EvalJSWithResult executes JavaScript and returns the result string.
// Serialized with evalResultMu because the C layer uses a single shared
// jsResultBuffer + semaphore — concurrent calls would race on that buffer.
func (m *Manager) EvalJSWithResult(js string) string {
	m.mu.RLock()
	created := m.created
	m.mu.RUnlock()
	if !created {
		return ""
	}
	m.evalResultMu.Lock()
	defer m.evalResultMu.Unlock()
	cJS := C.CString(js)
	defer C.free(unsafe.Pointer(cJS))
	C.OverlayEvalJSWithResult(cJS)
	result := C.WaitJSResult()
	return C.GoString(result)
}

// UpdateHintRect updates the bounding rect for a hint element
func (m *Manager) UpdateHintRect(id string, rect HintRect) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hintRects[id] = rect
}

// RemoveHintRect removes a hint rect
func (m *Manager) RemoveHintRect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hintRects, id)
}

// UpdateTextRect updates the bounding rect for a text overlay element
func (m *Manager) UpdateTextRect(id string, rect HintRect) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.textRects[id] = rect
}

// RemoveTextRect removes a text overlay rect
func (m *Manager) RemoveTextRect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.textRects, id)
}

// SetOnAction sets the callback for overlay actions (from JS→Go polling)
func (m *Manager) SetOnAction(fn func(action string, actionType string, id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAction = fn
}

// StartHintInteraction starts the goroutine that polls mouse position and toggles ignoresMouseEvents
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

	// Separate ticker for JS→Go action polling (200ms)
	actionTicker := time.NewTicker(200 * time.Millisecond)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer actionTicker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.checkMouseOverHints()
			case <-actionTicker.C:
				m.pollPendingActions()
			}
		}
	}()
}

// StopHintInteraction stops the hint interaction goroutine
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
		return
	}

	m.mu.RLock()
	cb := m.onAction
	m.mu.RUnlock()

	for _, a := range actions {
		if cb != nil {
			cb(a.Action, a.Type, a.ID)
		}
	}
}

// SyncHintRects fetches current hint/text bounding rects from JS and updates the internal maps
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
