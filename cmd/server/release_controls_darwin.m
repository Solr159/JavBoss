#import <Cocoa/Cocoa.h>

void javbossPrepareMenuBarApplication(void) {
	@autoreleasepool {
		// The executable has no Info.plist. Use the runtime equivalent of
		// LSUIElement so it can own a status item without appearing in the Dock.
		[[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
	}
}
