#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <CoreGraphics/CoreGraphics.h>
#import <stdbool.h>

bool javbossProcessHasWindow(pid_t pid) {
	CFArrayRef windows = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (windows == NULL) {
		return false;
	}

	bool found = false;
	CFIndex count = CFArrayGetCount(windows);
	for (CFIndex i = 0; i < count; i++) {
		CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);
		if (window == NULL) {
			continue;
		}

		CFNumberRef ownerPIDValue = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowOwnerPID);
		int ownerPID = 0;
		if (ownerPIDValue == NULL || !CFNumberGetValue(ownerPIDValue, kCFNumberIntType, &ownerPID) || ownerPID != pid) {
			continue;
		}

		CFNumberRef layerValue = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowLayer);
		int layer = 0;
		if (layerValue != NULL && CFNumberGetValue(layerValue, kCFNumberIntType, &layer) && layer != 0) {
			continue;
		}

		found = true;
		break;
	}

	CFRelease(windows);
	return found;
}

bool javbossActivateProcess(pid_t pid) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		bool activated = false;
		if (app != nil) {
			[app unhide];
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			activated = [app activateWithOptions:NSApplicationActivateIgnoringOtherApps | NSApplicationActivateAllWindows];
#pragma clang diagnostic pop
			if (app.active) {
				return true;
			}
		}

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		ProcessSerialNumber psn;
		if (GetProcessForPID(pid, &psn) == noErr) {
			if (SetFrontProcessWithOptions(&psn, kSetFrontProcessFrontWindowOnly) == noErr) {
				return true;
			}
			if (SetFrontProcess(&psn) == noErr) {
				return true;
			}
		}
#pragma clang diagnostic pop

		return activated;
	}
}
