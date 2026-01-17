//go:build darwin

package permissions

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework AppKit -framework Foundation

#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#include <unistd.h>

// ============================================================================
// SCREEN CAPTURE PERMISSION CHECK
// ============================================================================
// The ONLY reliable way to check screen capture permission is to actually
// try to capture the screen and verify the result is not a black/empty image.
// CGPreflightScreenCaptureAccess() can return cached/stale values.

// Check if we can see window names of OTHER applications
// This is the DEFINITIVE test - without Screen Recording permission,
// macOS hides window names of other apps (kCGWindowName returns NULL)
// Returns: 1 = permission granted, 0 = denied
int checkScreenCapturePermissionReal() {
    NSLog(@"[Permissions] === Screen Capture Permission Check (REAL TEST) ===");

    // Strategy 1: Check if we can read window NAMES of other applications
    // Without Screen Recording permission, kCGWindowName is NULL for other apps' windows
    // This is the most reliable and fast check

    pid_t ourPID = getpid();
    NSLog(@"[Permissions] Our PID: %d", ourPID);

    CFArrayRef windowList = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID
    );

    if (windowList == NULL) {
        NSLog(@"[Permissions] CGWindowListCopyWindowInfo returned NULL");
        return 0;
    }

    CFIndex windowCount = CFArrayGetCount(windowList);
    NSLog(@"[Permissions] Found %ld windows on screen", (long)windowCount);

    // Count windows from other apps and check if we can see their names
    int otherAppWindows = 0;
    int windowsWithVisibleName = 0;

    for (CFIndex i = 0; i < windowCount; i++) {
        CFDictionaryRef windowInfo = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);

        // Get window owner PID
        CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(windowInfo, kCGWindowOwnerPID);
        if (pidRef == NULL) continue;

        pid_t windowPID = 0;
        CFNumberGetValue(pidRef, kCFNumberIntType, &windowPID);

        // Skip our own windows - we can always see our own window names
        if (windowPID == ourPID) {
            continue;
        }

        // Get window layer - skip system UI elements (menu bar, dock, etc)
        CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(windowInfo, kCGWindowLayer);
        if (layerRef != NULL) {
            int layer = 0;
            CFNumberGetValue(layerRef, kCFNumberIntType, &layer);
            // Skip windows with layer != 0 (normal windows have layer 0)
            if (layer != 0) continue;
        }

        // Get window bounds to filter out tiny/invisible windows
        CFDictionaryRef boundsRef = (CFDictionaryRef)CFDictionaryGetValue(windowInfo, kCGWindowBounds);
        if (boundsRef == NULL) continue;

        CGRect bounds;
        if (!CGRectMakeWithDictionaryRepresentation(boundsRef, &bounds)) continue;

        // Skip tiny windows (likely invisible or system elements)
        if (bounds.size.width < 50 || bounds.size.height < 50) continue;

        // This is a real window from another app
        otherAppWindows++;

        // Get owner name (always available)
        CFStringRef ownerName = (CFStringRef)CFDictionaryGetValue(windowInfo, kCGWindowOwnerName);
        NSString *owner = ownerName ? (__bridge NSString *)ownerName : @"unknown";

        // Try to get window NAME - this is the key test!
        // Without Screen Recording permission, this returns NULL for other apps
        CFStringRef windowName = (CFStringRef)CFDictionaryGetValue(windowInfo, kCGWindowName);

        if (windowName != NULL) {
            NSString *name = (__bridge NSString *)windowName;
            // Window name is visible - permission granted!
            windowsWithVisibleName++;
            NSLog(@"[Permissions] Window name VISIBLE: owner=%@, name='%@', size=%.0fx%.0f",
                  owner, name, bounds.size.width, bounds.size.height);
        } else {
            NSLog(@"[Permissions] Window name HIDDEN: owner=%@, name=NULL, size=%.0fx%.0f",
                  owner, bounds.size.width, bounds.size.height);
        }
    }

    CFRelease(windowList);

    NSLog(@"[Permissions] Summary: %d other app windows, %d with visible names",
          otherAppWindows, windowsWithVisibleName);

    // Decision logic:
    // 1. If we found windows from other apps and can see at least one name -> GRANTED
    // 2. If we found windows but ALL names are NULL -> DENIED
    // 3. If no other app windows found -> use fallback (screen capture test)

    if (otherAppWindows > 0) {
        if (windowsWithVisibleName > 0) {
            NSLog(@"[Permissions] Screen Recording permission GRANTED (can see window names)");
            return 1;
        } else {
            NSLog(@"[Permissions] Screen Recording permission DENIED (all window names hidden)");
            return 0;
        }
    }

    // Fallback: No other app windows found
    // Try to capture the entire screen and check if we can see window content
    NSLog(@"[Permissions] No other app windows found, using screen capture fallback...");

    // Get main display bounds
    CGDirectDisplayID mainDisplay = CGMainDisplayID();
    CGRect displayBounds = CGDisplayBounds(mainDisplay);
    NSLog(@"[Permissions] Main display size: %.0fx%.0f", displayBounds.size.width, displayBounds.size.height);

    // Capture a small region of the screen (top-left corner where menu bar apps usually are)
    CGRect captureRect = CGRectMake(100, 100, 200, 200);

    CGImageRef image = CGWindowListCreateImage(
        captureRect,
        kCGWindowListOptionOnScreenOnly,
        kCGNullWindowID,
        kCGWindowImageDefault
    );

    if (image == NULL) {
        NSLog(@"[Permissions] Screen capture returned NULL - assuming DENIED");
        return 0;
    }

    size_t width = CGImageGetWidth(image);
    size_t height = CGImageGetHeight(image);
    NSLog(@"[Permissions] Captured image size: %zux%zu", width, height);

    if (width == 0 || height == 0) {
        CFRelease(image);
        NSLog(@"[Permissions] Image has zero size - assuming DENIED");
        return 0;
    }

    // If we got here and captured something, we likely have permission
    // (Without permission, the capture would show only wallpaper, but we can't
    // easily distinguish that without knowing what apps should be visible)
    //
    // Since there are no other windows open, we'll assume permission is granted
    // if we can capture the screen at all. The user can verify by opening another app.

    CFRelease(image);
    NSLog(@"[Permissions] Screen capture successful, no other windows to verify against");
    NSLog(@"[Permissions] Assuming GRANTED (open another app window to verify properly)");
    return 1;
}

// ============================================================================
// ACCESSIBILITY PERMISSION CHECK
// ============================================================================
// The ONLY reliable way to check accessibility permission is to actually
// try to create a CGEventTap. AXIsProcessTrusted() can return cached values.

// Dummy callback for event tap test
static CGEventRef testEventTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    return event;
}

// Try to create an event tap - this is the definitive test
// Returns: 1 = permission granted (can create tap), 0 = denied
int checkAccessibilityPermissionReal() {
    NSLog(@"[Permissions] === Accessibility Permission Check (REAL TEST) ===");

    // The ONLY reliable way: try to create an event tap
    // This will fail immediately if we don't have accessibility permission

    CGEventMask eventMask = CGEventMaskBit(kCGEventKeyDown);

    CFMachPortRef testTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,  // Active tap requires Accessibility permission
        eventMask,
        testEventTapCallback,
        NULL
    );

    if (testTap != NULL) {
        // Successfully created tap - permission IS granted
        CFRelease(testTap);
        NSLog(@"[Permissions] CGEventTapCreate SUCCESS - permission GRANTED");
        return 1;
    }

    NSLog(@"[Permissions] CGEventTapCreate returned NULL - permission DENIED");

    // Log what the cached APIs say for debugging
    BOOL axTrusted = AXIsProcessTrusted();
    NSLog(@"[Permissions] AXIsProcessTrusted() = %@ (may be cached/stale)", axTrusted ? @"YES" : @"NO");

    if (@available(macOS 10.15, *)) {
        BOOL postEvent = CGPreflightPostEventAccess();
        NSLog(@"[Permissions] CGPreflightPostEventAccess() = %@ (may be cached/stale)", postEvent ? @"YES" : @"NO");
    }

    // Even if cached APIs say yes, the real test failed - permission is NOT granted
    if (axTrusted) {
        NSLog(@"[Permissions] WARNING: AXIsProcessTrusted=YES but CGEventTapCreate failed!");
        NSLog(@"[Permissions] This usually means permission was granted AFTER app started.");
        NSLog(@"[Permissions] App RESTART is required to apply the permission!");
    }

    return 0;
}

// ============================================================================
// LEGACY API CHECKS (for reference/comparison only)
// ============================================================================

int checkScreenCapturePermissionCached() {
    if (@available(macOS 10.15, *)) {
        return CGPreflightScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}

int checkAccessibilityPermissionCached() {
    return AXIsProcessTrusted() ? 1 : 0;
}

// ============================================================================
// PERMISSION REQUESTS
// ============================================================================

// Request screen capture permission (shows system dialog)
// NOTE: CGRequestScreenCaptureAccess() does NOT add the app to the Screen Recording list automatically!
// To trigger the system to add our app to the list, we need to actually try to capture screen content.
int requestScreenCapturePermission() {
    if (@available(macOS 10.15, *)) {
        NSLog(@"[Permissions] Requesting screen capture permission...");

        // First, call the API (this may return cached value)
        BOOL result = CGRequestScreenCaptureAccess();
        NSLog(@"[Permissions] CGRequestScreenCaptureAccess() = %@", result ? @"YES" : @"NO");

        if (!result) {
            // To make the app appear in Screen Recording list, we need to actually
            // attempt a screen capture operation. This triggers macOS to add the app
            // to the list (similar to how Accessibility works with kAXTrustedCheckOptionPrompt).
            NSLog(@"[Permissions] Attempting screen capture to trigger system prompt...");

            // Try to capture a small region of the screen
            // This will fail but it triggers macOS to add us to Screen Recording list
            CGImageRef image = CGWindowListCreateImage(
                CGRectMake(0, 0, 1, 1),  // Tiny 1x1 rect
                kCGWindowListOptionOnScreenOnly,
                kCGNullWindowID,
                kCGWindowImageDefault
            );

            if (image != NULL) {
                CFRelease(image);
                NSLog(@"[Permissions] Screen capture succeeded - permission was already granted");
            } else {
                NSLog(@"[Permissions] Screen capture failed - app should now appear in Screen Recording list");
            }

            NSLog(@"[Permissions] User needs to enable in: System Settings > Privacy & Security > Screen Recording");
            NSLog(@"[Permissions] After enabling, APP RESTART IS REQUIRED!");
        }
        return result ? 1 : 0;
    }
    return 1;
}

// Request accessibility permission (does NOT open System Preferences automatically)
// User should use openAccessibilityPreferences() via UI button to open settings
void requestAccessibilityPermission() {
    NSLog(@"[Permissions] Checking accessibility permission (no auto-prompt)...");

    // Just check the status without showing system prompt
    // kAXTrustedCheckOptionPrompt: @NO means don't show the system dialog
    NSDictionary *options = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @NO};
    BOOL result = AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
    NSLog(@"[Permissions] AXIsProcessTrustedWithOptions (no prompt) = %@", result ? @"YES" : @"NO");

    if (!result) {
        NSLog(@"[Permissions] User needs to enable in: System Settings > Privacy & Security > Accessibility");
        NSLog(@"[Permissions] After enabling, APP RESTART IS REQUIRED!");
    }
}

// ============================================================================
// OPEN SYSTEM PREFERENCES
// ============================================================================

void openScreenRecordingPreferences() {
    NSLog(@"[Permissions] Opening Screen Recording preferences...");
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    });
}

void openAccessibilityPreferences() {
    NSLog(@"[Permissions] Opening Accessibility preferences...");
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    });
}

// ============================================================================
// DIAGNOSTIC INFO
// ============================================================================

// Get comprehensive permission diagnostic info
void logPermissionDiagnostics() {
    NSLog(@"[Permissions] ========== PERMISSION DIAGNOSTICS ==========");
    NSLog(@"[Permissions] Process ID: %d", getpid());

    // Get app bundle identifier
    NSBundle *bundle = [NSBundle mainBundle];
    NSString *bundleId = [bundle bundleIdentifier];
    NSLog(@"[Permissions] Bundle ID: %@", bundleId ?: @"(none - not bundled app)");

    // Check both cached and real permissions
    NSLog(@"[Permissions] --- Cached API Results (may be stale) ---");

    BOOL axTrusted = AXIsProcessTrusted();
    NSLog(@"[Permissions] AXIsProcessTrusted: %@", axTrusted ? @"YES" : @"NO");

    if (@available(macOS 10.15, *)) {
        BOOL screenPreflight = CGPreflightScreenCaptureAccess();
        BOOL postEventPreflight = CGPreflightPostEventAccess();
        NSLog(@"[Permissions] CGPreflightScreenCaptureAccess: %@", screenPreflight ? @"YES" : @"NO");
        NSLog(@"[Permissions] CGPreflightPostEventAccess: %@", postEventPreflight ? @"YES" : @"NO");
    }

    NSLog(@"[Permissions] --- Real Tests (definitive) ---");
    int screenReal = checkScreenCapturePermissionReal();
    int accessReal = checkAccessibilityPermissionReal();
    NSLog(@"[Permissions] Screen Capture (real test): %s", screenReal ? "GRANTED" : "DENIED");
    NSLog(@"[Permissions] Accessibility (real test): %s", accessReal ? "GRANTED" : "DENIED");

    NSLog(@"[Permissions] ============================================");
}
*/
import "C"

import (
	"time"
)

// CheckScreenCapture checks if screen capture permission is granted on macOS
// Uses REAL test (actual window name visibility check) instead of cached API
func (m *Manager) CheckScreenCapture() PermissionStatus {
	result := C.checkScreenCapturePermissionReal()
	if result == 1 {
		return StatusGranted
	}
	return StatusDenied
}

// CheckScreenCaptureCached checks screen capture using cached API (less reliable)
func (m *Manager) CheckScreenCaptureCached() PermissionStatus {
	result := C.checkScreenCapturePermissionCached()
	if result == 1 {
		return StatusGranted
	}
	return StatusDenied
}

// RequestScreenCapture requests screen capture permission on macOS
// NOTE: Does NOT open settings automatically - user should use OpenScreenCaptureSettings() via UI button
func (m *Manager) RequestScreenCapture() bool {
	// Request will trigger adding app to Screen Recording list (via CGWindowListCreateImage attempt)
	result := C.requestScreenCapturePermission()
	if result == 1 {
		return true
	}

	// Return false but don't open preferences automatically
	// User can open settings manually via UI button
	return false
}

// CheckAccessibility checks if accessibility permission is granted on macOS
// Uses REAL test (actual CGEventTap creation) instead of cached API
func (m *Manager) CheckAccessibility() PermissionStatus {
	result := C.checkAccessibilityPermissionReal()
	if result == 1 {
		return StatusGranted
	}
	return StatusDenied
}

// CheckAccessibilityCached checks accessibility using cached API (less reliable)
func (m *Manager) CheckAccessibilityCached() PermissionStatus {
	result := C.checkAccessibilityPermissionCached()
	if result == 1 {
		return StatusGranted
	}
	return StatusDenied
}

// RequestAccessibility requests accessibility permission on macOS
func (m *Manager) RequestAccessibility() bool {
	// This will show the system prompt
	C.requestAccessibilityPermission()

	// Give time for user to respond
	time.Sleep(100 * time.Millisecond)

	// Check if granted using real test
	return C.checkAccessibilityPermissionReal() == 1
}

// OpenScreenCaptureSettings opens the Screen Recording preferences pane
func (m *Manager) OpenScreenCaptureSettings() {
	C.openScreenRecordingPreferences()
}

// OpenAccessibilitySettings opens the Accessibility preferences pane
func (m *Manager) OpenAccessibilitySettings() {
	C.openAccessibilityPreferences()
}

// LogDiagnostics logs comprehensive permission diagnostic info
func (m *Manager) LogDiagnostics() {
	C.logPermissionDiagnostics()
}

// RequestAllPermissions requests all required permissions
// NOTE: Does NOT open settings automatically - user should use OpenPermissionSettings() via UI button
func (m *Manager) RequestAllPermissions() map[PermissionType]bool {
	results := make(map[PermissionType]bool)

	// Log diagnostics first
	C.logPermissionDiagnostics()

	// Check current status using REAL tests
	screenCaptureGranted := m.CheckScreenCapture() == StatusGranted
	accessibilityGranted := m.CheckAccessibility() == StatusGranted

	results[PermissionScreenCapture] = screenCaptureGranted
	results[PermissionAccessibility] = accessibilityGranted

	// Request permissions that aren't granted yet (shows system dialogs where applicable)
	// but do NOT open System Preferences automatically - user can do that via UI button
	if !accessibilityGranted {
		// Request accessibility - this shows system prompt via AXIsProcessTrustedWithOptions
		C.requestAccessibilityPermission()
		// Recheck after request
		results[PermissionAccessibility] = m.CheckAccessibility() == StatusGranted
	}

	if !screenCaptureGranted {
		// Request screen capture - this triggers adding app to the list
		C.requestScreenCapturePermission()
		// Recheck after request
		results[PermissionScreenCapture] = m.CheckScreenCapture() == StatusGranted
	}

	return results
}

// GetPlatform returns the current platform
func (m *Manager) GetPlatform() string {
	return "darwin"
}

// NeedsRestart returns true if permissions appear granted but services are not working
// This indicates the app was running when permission was granted and needs restart
func (m *Manager) NeedsRestart() bool {
	// For accessibility: check if API says yes but real test (event tap) fails
	accessCached := C.checkAccessibilityPermissionCached() == 1
	accessReal := C.checkAccessibilityPermissionReal() == 1

	// If accessibility shows granted in cache but fails real test, restart needed
	if accessCached && !accessReal {
		return true
	}

	return false
}
