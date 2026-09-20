#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#import "manager_darwin.h"

extern void goOnToggleWindow(void);
extern void goOnDayView(void);
extern void goOnExport(void);
extern void goOnQuickShift(void);
extern void goOnQuit(void);

static NSWindow *g_appWindow = nil;
static NSStatusItem *g_statusItem = nil;
static NSMenu *g_statusMenu = nil;
static NSString *g_statusTitle = nil;

@interface TimeTrackerStatusActions : NSObject
- (void)statusClicked:(id)sender;
- (void)toggleWindow:(id)sender;
- (void)dayView:(id)sender;
- (void)exportMonth:(id)sender;
- (void)quickShift:(id)sender;
- (void)quit:(id)sender;
@end

static TimeTrackerStatusActions *g_statusActions = nil;

@implementation TimeTrackerStatusActions
- (void)statusClicked:(id)sender {
    NSEvent *event = [NSApp currentEvent];
    if ([event type] == NSEventTypeRightMouseUp ||
        ([event modifierFlags] & NSEventModifierFlagControl)) {
        // Keep the menu detached normally so a left click toggles the tracker.
        [g_statusItem setMenu:g_statusMenu];
        [[g_statusItem button] performClick:nil];
        [g_statusItem setMenu:nil];
    } else {
        goOnToggleWindow();
    }
}
- (void)toggleWindow:(id)sender { goOnToggleWindow(); }
- (void)dayView:(id)sender { goOnDayView(); }
- (void)exportMonth:(id)sender { goOnExport(); }
- (void)quickShift:(id)sender { goOnQuickShift(); }
- (void)quit:(id)sender {
    goOnQuit();
    [NSApp terminate:nil];
}
@end

static NSWindow *GetGogpuWindow(void) {
    if (g_appWindow) return g_appWindow;
    NSWindow *statusWin = g_statusItem ? [[g_statusItem button] window] : nil;
    for (NSWindow *window in [NSApp windows]) {
        if (window == statusWin) continue;
        if ([window isKindOfClass:NSClassFromString(@"NSStatusBarWindow")]) continue;
        g_appWindow = window;
        return g_appWindow;
    }
    return nil;
}

static void AddStatusAction(NSString *title, SEL action, NSString *key) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
    [item setTarget:g_statusActions];
    [g_statusMenu addItem:item];
    [item release];
}

void DarwinInitStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win setLevel:NSNormalWindowLevel];
            [win setSharingType:NSWindowSharingNone];
        }
        if (g_statusItem) return;

        g_statusActions = [[TimeTrackerStatusActions alloc] init];
        g_statusMenu = [[NSMenu alloc] initWithTitle:@"Simple Time Tracker"];
        AddStatusAction(@"Show / Hide Tracker", @selector(toggleWindow:), @"");
        [g_statusMenu addItem:[NSMenuItem separatorItem]];
        AddStatusAction(@"Day View", @selector(dayView:), @"");
        AddStatusAction(@"Monthly Export", @selector(exportMonth:), @"");
        AddStatusAction(@"QuickShift", @selector(quickShift:), @"");
        [g_statusMenu addItem:[NSMenuItem separatorItem]];
        AddStatusAction(@"Quit Simple Time Tracker", @selector(quit:), @"q");

        g_statusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
        NSStatusBarButton *button = [g_statusItem button];
        [button setTitle:g_statusTitle ?: @"⏱️ Time"];
        [button setFont:[NSFont monospacedDigitSystemFontOfSize:13 weight:NSFontWeightRegular]];
        [button setToolTip:@"Simple Time Tracker — click to show/hide; right-click for actions"];
        [button setTarget:g_statusActions];
        [button setAction:@selector(statusClicked:)];
        [button sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
    });
}

void DarwinUpdateTitle(const char *title) {
    if (!title) return;
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    if (!nsTitle) return;
    void (^block)(void) = ^{
        [g_statusTitle release];
        g_statusTitle = [nsTitle copy];
        NSStatusBarButton *btn = [g_statusItem button];
        if (btn) {
            [btn setTitle:g_statusTitle];
            [btn setNeedsDisplay:YES];
        }
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

// All helpers below run on the Cocoa main queue. Sizes are content dimensions,
// not outer frames, so the title bar cannot clip the bottom of the tracker.
static void PositionPopover(int width, int height) {
    NSWindow *win = GetGogpuWindow();
    if (!win || width <= 0 || height <= 0) return;

    NSRect frame = [win frameRectForContentRect:NSMakeRect(0, 0, width, height)];
    NSWindow *statusWindow = [[g_statusItem button] window];
    NSScreen *screen = [statusWindow screen] ?: [win screen] ?: [NSScreen mainScreen];
    NSRect visible = [screen visibleFrame];
    if (statusWindow) {
        NSRect anchor = [statusWindow convertRectToScreen:[[g_statusItem button] frame]];
        frame.origin.x = NSMaxX(anchor) - NSWidth(frame);
        frame.origin.y = NSMinY(anchor) - NSHeight(frame) - 4;
    } else {
        NSRect current = [win frame];
        frame.origin.x = NSMinX(current);
        frame.origin.y = NSMaxY(current) - NSHeight(frame);
    }
    frame.origin.x = MAX(NSMinX(visible), MIN(frame.origin.x, NSMaxX(visible) - NSWidth(frame)));
    frame.origin.y = MAX(NSMinY(visible), MIN(frame.origin.y, NSMaxY(visible) - NSHeight(frame)));
    [win setFrame:frame display:YES];
    [win makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}

void DarwinPositionPopover(int width, int height) {
    void (^block)(void) = ^{
        PositionPopover(width, height);
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

void DarwinHidePopover(void) {
    void (^block)(void) = ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win orderOut:nil];
        }
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

void DarwinTogglePopover(int width, int height) {
    void (^block)(void) = ^{
        NSWindow *win = GetGogpuWindow();
        if (win && [win isVisible]) {
            [win orderOut:nil];
        } else {
            PositionPopover(width, height);
        }
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

void DarwinSetWindowSharingNone(int enable) {
    void (^block)(void) = ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win setSharingType:(enable ? NSWindowSharingNone : NSWindowSharingReadOnly)];
        }
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}
