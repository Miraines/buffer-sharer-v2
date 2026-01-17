//go:build darwin

package screenshot

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreGraphics -framework Foundation -framework ImageIO -framework CoreServices -framework ScreenCaptureKit -framework AppKit -framework UniformTypeIdentifiers

#include <CoreGraphics/CoreGraphics.h>
#include <ImageIO/ImageIO.h>
#include <CoreServices/CoreServices.h>
#include <AvailabilityMacros.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

// Forward declaration - ScreenCaptureKit is only available on macOS 12.3+
// SCScreenshotManager is only available on macOS 14.0+

// Check if running on macOS 14.0 or later (for SCScreenshotManager)
static BOOL isMacOS14OrLater(void) {
    if (@available(macOS 14.0, *)) {
        return YES;
    }
    return NO;
}

// Check if running on macOS 12.3 or later (for ScreenCaptureKit)
static BOOL isMacOS12_3OrLater(void) {
    if (@available(macOS 12.3, *)) {
        return YES;
    }
    return NO;
}

// Get JPEG UTType identifier (using modern API when available)
static CFStringRef getJPEGTypeIdentifier(void) {
    if (@available(macOS 11.0, *)) {
        return (__bridge CFStringRef)UTTypeJPEG.identifier;
    }
    // Fallback for older macOS - use the string literal
    return CFSTR("public.jpeg");
}

// Convert CGImage to JPEG data
static CFDataRef cgImageToJPEG(CGImageRef image, int quality) {
    if (image == NULL) {
        return NULL;
    }

    CFMutableDataRef data = CFDataCreateMutable(kCFAllocatorDefault, 0);
    if (data == NULL) {
        return NULL;
    }

    CGImageDestinationRef dest = CGImageDestinationCreateWithData(
        data,
        getJPEGTypeIdentifier(),
        1,
        NULL
    );

    if (dest == NULL) {
        CFRelease(data);
        return NULL;
    }

    float compressionQuality = (float)quality / 100.0f;
    CFStringRef keys[] = { kCGImageDestinationLossyCompressionQuality };
    CFNumberRef values[] = { CFNumberCreate(kCFAllocatorDefault, kCFNumberFloatType, &compressionQuality) };
    CFDictionaryRef options = CFDictionaryCreate(
        kCFAllocatorDefault,
        (const void**)keys,
        (const void**)values,
        1,
        &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks
    );

    CGImageDestinationAddImage(dest, image, options);
    bool success = CGImageDestinationFinalize(dest);

    CFRelease(values[0]);
    CFRelease(options);
    CFRelease(dest);

    if (!success) {
        CFRelease(data);
        return NULL;
    }

    return data;
}

// ============================================================================
// ScreenCaptureKit implementation (macOS 14.0+)
// ============================================================================

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
#import <ScreenCaptureKit/ScreenCaptureKit.h>

static CFDataRef captureScreenWithSCKit(int quality, int *outWidth, int *outHeight) API_AVAILABLE(macos(14.0)) {
    __block CGImageRef capturedImage = NULL;
    __block int displayWidth = 0;
    __block int displayHeight = 0;

    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

    // Get shareable content (displays, windows, apps)
    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent * _Nullable content, NSError * _Nullable error) {
        if (error != nil || content == nil || content.displays.count == 0) {
            NSLog(@"[ScreenCaptureKit] Failed to get shareable content: %@", error);
            dispatch_semaphore_signal(semaphore);
            return;
        }

        // Get main display
        SCDisplay *mainDisplay = nil;
        CGDirectDisplayID mainDisplayID = CGMainDisplayID();

        for (SCDisplay *display in content.displays) {
            if (display.displayID == mainDisplayID) {
                mainDisplay = display;
                break;
            }
        }

        if (mainDisplay == nil) {
            mainDisplay = content.displays[0];
        }

        displayWidth = (int)mainDisplay.width;
        displayHeight = (int)mainDisplay.height;

        // Create content filter for full display (exclude nothing)
        SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:mainDisplay excludingWindows:@[]];

        // Create stream configuration
        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];

        // Use native resolution (account for Retina displays)
        CGFloat scaleFactor = [[NSScreen mainScreen] backingScaleFactor];
        config.width = (size_t)(displayWidth * scaleFactor);
        config.height = (size_t)(displayHeight * scaleFactor);

        // Use BGRA pixel format for compatibility
        config.pixelFormat = kCVPixelFormatType_32BGRA;

        // Show cursor in screenshot
        config.showsCursor = YES;

        // Capture screenshot using SCScreenshotManager
        [SCScreenshotManager captureImageWithFilter:filter
                                      configuration:config
                                  completionHandler:^(CGImageRef _Nullable image, NSError * _Nullable error) {
            if (error != nil) {
                NSLog(@"[ScreenCaptureKit] Screenshot capture failed: %@", error);
            } else if (image != NULL) {
                capturedImage = CGImageRetain(image);
                // Update dimensions from actual captured image
                displayWidth = (int)CGImageGetWidth(image);
                displayHeight = (int)CGImageGetHeight(image);
            }
            dispatch_semaphore_signal(semaphore);
        }];
    }];

    // Wait for completion with 10 second timeout
    dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC);
    long result = dispatch_semaphore_wait(semaphore, timeout);

    if (result != 0) {
        NSLog(@"[ScreenCaptureKit] Screenshot capture timed out");
        return NULL;
    }

    if (capturedImage == NULL) {
        return NULL;
    }

    // Set output dimensions
    *outWidth = displayWidth;
    *outHeight = displayHeight;

    // Convert to JPEG
    CFDataRef jpegData = cgImageToJPEG(capturedImage, quality);
    CGImageRelease(capturedImage);

    return jpegData;
}

#endif // __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000

// ============================================================================
// Legacy CGWindowListCreateImage implementation (fallback)
// ============================================================================

static CFDataRef captureScreenLegacy(int quality, int *outWidth, int *outHeight) {
    // Get main display bounds
    CGRect bounds = CGDisplayBounds(CGMainDisplayID());
    *outWidth = (int)bounds.size.width;
    *outHeight = (int)bounds.size.height;

    // Capture all windows on screen
    // kCGWindowListOptionOnScreenOnly - only windows currently on screen
    // kCGNullWindowID - capture all windows (not a specific one)
    CGImageRef image = CGWindowListCreateImage(
        bounds,
        kCGWindowListOptionOnScreenOnly,
        kCGNullWindowID,
        kCGWindowImageBoundsIgnoreFraming | kCGWindowImageShouldBeOpaque | kCGWindowImageNominalResolution
    );

    if (image == NULL) {
        return NULL;
    }

    CFDataRef jpegData = cgImageToJPEG(image, quality);
    CGImageRelease(image);

    return jpegData;
}

// ============================================================================
// Main capture function - selects best available API
// ============================================================================

static CFDataRef captureScreen(int quality, int *outWidth, int *outHeight) {
    *outWidth = 0;
    *outHeight = 0;

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    // Try ScreenCaptureKit first (macOS 14.0+)
    if (@available(macOS 14.0, *)) {
        NSLog(@"[Screenshot] Using ScreenCaptureKit (macOS 14+)");
        CFDataRef result = captureScreenWithSCKit(quality, outWidth, outHeight);
        if (result != NULL) {
            return result;
        }
        NSLog(@"[Screenshot] ScreenCaptureKit failed, falling back to legacy API");
    }
#endif

    // Fallback to legacy CGWindowListCreateImage
    NSLog(@"[Screenshot] Using legacy CGWindowListCreateImage");
    return captureScreenLegacy(quality, outWidth, outHeight);
}

// Get screen dimensions
static void getScreenSize(int *width, int *height) {
    CGRect bounds = CGDisplayBounds(CGMainDisplayID());
    *width = (int)bounds.size.width;
    *height = (int)bounds.size.height;
}

// Check if screen recording permission is granted
static int hasScreenRecordingPermission(void) {
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    // On macOS 14+, use ScreenCaptureKit to check permission
    if (@available(macOS 14.0, *)) {
        __block BOOL hasPermission = NO;
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

        [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent * _Nullable content, NSError * _Nullable error) {
            // If we can get content without error, we have permission
            hasPermission = (error == nil && content != nil);
            dispatch_semaphore_signal(semaphore);
        }];

        dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
        return hasPermission ? 1 : 0;
    }
#endif

    // Legacy check using CGWindowListCreateImage
    CGImageRef testImage = CGWindowListCreateImage(
        CGRectMake(0, 0, 1, 1),
        kCGWindowListOptionOnScreenOnly,
        kCGNullWindowID,
        kCGWindowImageDefault
    );

    if (testImage == NULL) {
        return 0;
    }

    size_t width = CGImageGetWidth(testImage);
    size_t height = CGImageGetHeight(testImage);
    CGImageRelease(testImage);

    return (width > 0 && height > 0) ? 1 : 0;
}

// Check if ScreenCaptureKit is available
static int isScreenCaptureKitAvailable(void) {
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    if (@available(macOS 14.0, *)) {
        return 1;
    }
#endif
    return 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// captureScreenNative captures the screen using native macOS APIs
// On macOS 14+, uses ScreenCaptureKit (SCScreenshotManager)
// On older versions, falls back to CGWindowListCreateImage
// Returns JPEG data, width, height, or error
func captureScreenNative(quality int) ([]byte, int, int, error) {
	var width, height C.int

	data := C.captureScreen(C.int(quality), &width, &height)
	if uintptr(unsafe.Pointer(data)) == 0 {
		return nil, 0, 0, fmt.Errorf("failed to capture screen")
	}
	defer C.CFRelease(C.CFTypeRef(data))

	// Get data length and pointer
	length := C.CFDataGetLength(data)
	ptr := C.CFDataGetBytePtr(data)

	// Copy data to Go slice
	goData := C.GoBytes(unsafe.Pointer(ptr), C.int(length))

	return goData, int(width), int(height), nil
}

// hasScreenRecordingPermissionNative checks if screen recording permission is granted
func hasScreenRecordingPermissionNative() bool {
	return C.hasScreenRecordingPermission() == 1
}

// IsScreenCaptureKitAvailable returns true if ScreenCaptureKit (macOS 14+) is available
func IsScreenCaptureKitAvailable() bool {
	return C.isScreenCaptureKitAvailable() == 1
}
