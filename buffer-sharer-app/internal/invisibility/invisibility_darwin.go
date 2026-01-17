//go:build darwin

package invisibility

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <dlfcn.h>

// Private CoreGraphics Services API declarations
typedef int CGSConnectionID;
typedef int CGSWindowID;
typedef uint32_t CGSWindowTag;

// Function pointers for private APIs (loaded dynamically)
static CGSConnectionID (*pCGSMainConnectionID)(void) = NULL;
static CGError (*pCGSSetWindowTags)(CGSConnectionID cid, CGSWindowID wid, CGSWindowTag *tags, int32_t numTags) = NULL;
static CGError (*pCGSClearWindowTags)(CGSConnectionID cid, CGSWindowID wid, CGSWindowTag *tags, int32_t numTags) = NULL;

// Various tags that might hide windows from capture
// These are undocumented and may vary between macOS versions
static const CGSWindowTag kCGSTagSticky = (1 << 0);           // 0x1
static const CGSWindowTag kCGSTagNoShadow = (1 << 3);         // 0x8
static const CGSWindowTag kCGSTagTransparent = (1 << 9);      // 0x200
static const CGSWindowTag kCGSTagNoShadow2 = (1 << 11);       // 0x800
static const CGSWindowTag kCGSTagExcludeFromCapture = (1 << 17); // 0x20000 - potential capture exclusion

static bool apisLoaded = false;

// Load private APIs dynamically
void LoadPrivateAPIs() {
    if (apisLoaded) return;

    void *handle = dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", RTLD_NOW);
    if (handle) {
        pCGSMainConnectionID = dlsym(handle, "CGSMainConnectionID");
        pCGSSetWindowTags = dlsym(handle, "CGSSetWindowTags");
        pCGSClearWindowTags = dlsym(handle, "CGSClearWindowTags");
    }
    apisLoaded = true;
}

// SetWindowInvisibleToScreenCapture hides a window from all screen capture including screenshots
void SetWindowInvisibleToScreenCapture(NSWindow *window) {
    if (window == nil) return;

    LoadPrivateAPIs();

    // Method 1: Set sharing type to none (works for screen sharing apps like Zoom, Meet)
    [window setSharingType:NSWindowSharingNone];

    // Method 2: Try CGS private API tags
    if (pCGSMainConnectionID && pCGSSetWindowTags) {
        CGSConnectionID cid = pCGSMainConnectionID();
        CGSWindowID wid = (CGSWindowID)[window windowNumber];

        // Try multiple tags that might work for capture exclusion
        CGSWindowTag tags[] = {
            kCGSTagNoShadow2,           // 0x800
            kCGSTagExcludeFromCapture,  // 0x20000
        };

        for (int i = 0; i < sizeof(tags)/sizeof(tags[0]); i++) {
            pCGSSetWindowTags(cid, wid, &tags[i], 1);
        }
    }
}

// SetWindowVisibleToScreenCapture makes a window visible to screen capture again
void SetWindowVisibleToScreenCapture(NSWindow *window) {
    if (window == nil) return;

    LoadPrivateAPIs();

    // Restore sharing type
    [window setSharingType:NSWindowSharingReadOnly];

    // Clear CGS tags
    if (pCGSMainConnectionID && pCGSClearWindowTags) {
        CGSConnectionID cid = pCGSMainConnectionID();
        CGSWindowID wid = (CGSWindowID)[window windowNumber];

        CGSWindowTag tags[] = {
            kCGSTagNoShadow2,
            kCGSTagExcludeFromCapture,
        };

        for (int i = 0; i < sizeof(tags)/sizeof(tags[0]); i++) {
            pCGSClearWindowTags(cid, wid, &tags[i], 1);
        }
    }
}

// SetAllWindowsInvisible hides all app windows from screen capture
void SetAllWindowsInvisible() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray *windows = [[NSApplication sharedApplication] windows];
        for (NSWindow *window in windows) {
            SetWindowInvisibleToScreenCapture(window);
        }
    });
}

// SetAllWindowsVisible makes all app windows visible to screen capture
void SetAllWindowsVisible() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray *windows = [[NSApplication sharedApplication] windows];
        for (NSWindow *window in windows) {
            SetWindowVisibleToScreenCapture(window);
        }
    });
}

// GetWindowCount returns the number of app windows (for debugging)
int GetWindowCount() {
    return (int)[[[NSApplication sharedApplication] windows] count];
}

*/
import "C"

import (
	"sync"
)

// Manager handles window invisibility for screen sharing on macOS
type Manager struct {
	mu      sync.RWMutex
	enabled bool
}

// NewManager creates a new invisibility manager
func NewManager() *Manager {
	return &Manager{
		enabled: false,
	}
}

// SetEnabled enables or disables invisibility mode
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enabled == enabled {
		return
	}

	m.enabled = enabled

	if enabled {
		C.SetAllWindowsInvisible()
	} else {
		C.SetAllWindowsVisible()
	}
}

// Toggle toggles invisibility mode and returns the new state
func (m *Manager) Toggle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = !m.enabled

	if m.enabled {
		C.SetAllWindowsInvisible()
	} else {
		C.SetAllWindowsVisible()
	}

	return m.enabled
}

// IsEnabled returns whether invisibility mode is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetWindowCount returns the number of application windows (for debugging)
func (m *Manager) GetWindowCount() int {
	return int(C.GetWindowCount())
}

// IsSupported returns true on macOS
func (m *Manager) IsSupported() bool {
	return true
}
