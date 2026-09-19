//go:build darwin && cgo

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include <Cocoa/Cocoa.h>
#include <QuartzCore/QuartzCore.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>

extern void goOnToggleWindow(void);
extern void goOnDayView(void);
extern void goOnExport(void);
extern void goOnQuickShift(void);
extern void goOnQuit(void);

static NSStatusItem *g_statusItem = nil;
static NSWindow *g_appWindow = nil;
static int g_isVisible = 1;
static int g_lastWidth = 420;
static int g_lastHeight = 560;

@interface StatusItemTarget : NSObject
- (void)onStatusItemClicked:(id)sender;
- (void)onMenuToggle:(id)sender;
- (void)onMenuDayView:(id)sender;
- (void)onMenuExport:(id)sender;
- (void)onMenuQuickShift:(id)sender;
- (void)onMenuQuit:(id)sender;
@end

static StatusItemTarget *g_target = nil;

@implementation StatusItemTarget
- (void)onStatusItemClicked:(id)sender {
    NSEvent *event = [NSApp currentEvent];
    if ([event type] == NSEventTypeRightMouseUp) {
        NSMenu *menu = [[NSMenu alloc] init];
        [menu addItemWithTitle:@"Toggle Window" action:@selector(onMenuToggle:) keyEquivalent:@""];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItemWithTitle:@"📅 Day View (Calendar)" action:@selector(onMenuDayView:) keyEquivalent:@"d"];
        [menu addItemWithTitle:@"📊 Monthly Export (CSV)" action:@selector(onMenuExport:) keyEquivalent:@"e"];
        [menu addItemWithTitle:@"⚡ QuickShift" action:@selector(onMenuQuickShift:) keyEquivalent:@"q"];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItemWithTitle:@"Quit Yokto Time" action:@selector(onMenuQuit:) keyEquivalent:@""];
        for (NSMenuItem *item in [menu itemArray]) {
            [item setTarget:self];
        }
        [menu popUpMenuPositioningItem:nil atLocation:[NSEvent mouseLocation] inView:nil];
        return;
    }
    goOnToggleWindow();
}

- (void)onMenuToggle:(id)sender {
    goOnToggleWindow();
}

- (void)onMenuDayView:(id)sender {
    goOnDayView();
}

- (void)onMenuExport:(id)sender {
    goOnExport();
}

- (void)onMenuQuickShift:(id)sender {
    goOnQuickShift();
}

- (void)onMenuQuit:(id)sender {
    goOnQuit();
}
@end

@implementation NSWindow (YoktoKeyWindow)
- (BOOL)canBecomeKeyWindow {
    return YES;
}
- (BOOL)canBecomeMainWindow {
    return YES;
}
- (BOOL)performKeyEquivalent:(NSEvent *)event {
    if ([event keyCode] == 53) { // Escape
        goOnToggleWindow();
        return YES;
    }
    if (([event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask) == NSEventModifierFlagCommand) {
        NSString *chars = [event charactersIgnoringModifiers];
        if ([chars isEqualToString:@"v"]) {
            if ([NSApp sendAction:@selector(paste:) to:nil from:self]) return YES;
        } else if ([chars isEqualToString:@"c"]) {
            if ([NSApp sendAction:@selector(copy:) to:nil from:self]) return YES;
        } else if ([chars isEqualToString:@"x"]) {
            if ([NSApp sendAction:@selector(cut:) to:nil from:self]) return YES;
        } else if ([chars isEqualToString:@"a"]) {
            if ([NSApp sendAction:@selector(selectAll:) to:nil from:self]) return YES;
        } else if ([chars isEqualToString:@"z"]) {
            if ([NSApp sendAction:@selector(undo:) to:nil from:self]) return YES;
        } else if ([chars isEqualToString:@"w"]) {
            goOnToggleWindow();
            return YES;
        }
    }
    return [super performKeyEquivalent:event];
}
@end

static void ApplyWindowStyles(NSWindow *window) {
    if (!window) return;
    [window setStyleMask:NSWindowStyleMaskBorderless];
    [window setAcceptsMouseMovedEvents:YES];
    [window setOpaque:NO];
    [window setBackgroundColor:[NSColor clearColor]];
    [window setHasShadow:YES];

    NSWindowCollectionBehavior behavior =
        NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorStationary |
        NSWindowCollectionBehaviorIgnoresCycle |
        NSWindowCollectionBehaviorFullScreenAuxiliary;
    [window setCollectionBehavior:behavior];
    [window setLevel:NSFloatingWindowLevel];
    [window setHidesOnDeactivate:NO];

    NSView *contentView = [window contentView];
    if (contentView) {
        contentView.wantsLayer = YES;
        contentView.layer.opaque = NO;
        contentView.layer.backgroundColor = [[NSColor clearColor] CGColor];
    }
}

static void DarwinInitStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!g_statusItem) {
            g_statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
            g_target = [[StatusItemTarget alloc] init];
            g_statusItem.button.title = @"⏱️ Yokto";
            g_statusItem.button.target = g_target;
            g_statusItem.button.action = @selector(onStatusItemClicked:);
            [g_statusItem.button sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
        }

        if (!g_appWindow) {
            NSArray *windows = [NSApp windows];
            if ([windows count] > 0) {
                g_appWindow = [windows objectAtIndex:0];
                ApplyWindowStyles(g_appWindow);
            }
        }
    });
}

static void DarwinUpdateTitle(const char *title) {
    if (!title) return;
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_statusItem && g_statusItem.button) {
            g_statusItem.button.title = nsTitle;
        }
    });
}

static void DarwinPositionPopover(int width, int height) {
    if (!g_appWindow) {
        NSArray *windows = [NSApp windows];
        if ([windows count] > 0) {
            g_appWindow = [windows objectAtIndex:0];
            ApplyWindowStyles(g_appWindow);
        }
    }
    if (!g_appWindow) return;

    g_lastWidth = width;
    g_lastHeight = height;

    NSRect buttonRect = NSZeroRect;
    if (g_statusItem && g_statusItem.button && g_statusItem.button.window) {
        buttonRect = [g_statusItem.button.window convertRectToScreen:g_statusItem.button.bounds];
    } else {
        NSScreen *screen = [NSScreen mainScreen];
        NSRect frame = [screen frame];
        buttonRect = NSMakeRect(frame.size.width - 200, frame.size.height - 25, 100, 25);
    }

    CGFloat winW = (CGFloat)width;
    CGFloat winH = (CGFloat)height;

    // Center window under menu item
    CGFloat x = buttonRect.origin.x + (buttonRect.size.width / 2.0) - (winW / 2.0);
    CGFloat y = buttonRect.origin.y - winH - 6.0;

    // Keep on screen
    NSScreen *targetScreen = [NSScreen mainScreen];
    NSRect screenFrame = [targetScreen visibleFrame];
    if (x + winW > screenFrame.origin.x + screenFrame.size.width) {
        x = screenFrame.origin.x + screenFrame.size.width - winW - 12.0;
    }
    if (x < screenFrame.origin.x) {
        x = screenFrame.origin.x + 12.0;
    }

    [g_appWindow setFrame:NSMakeRect(x, y, winW, winH) display:YES animate:NO];
    [g_appWindow makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
    g_isVisible = 1;
}

static void DarwinHidePopover(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_appWindow) {
            [g_appWindow orderOut:nil];
            g_isVisible = 0;
        }
    });
}

static void DarwinTogglePopover(int width, int height) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_isVisible) {
            DarwinHidePopover();
        } else {
            DarwinPositionPopover(width, height);
        }
    });
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	darwinCallbacks StatusCallbacks
	darwinMu        sync.RWMutex
)

//export goOnToggleWindow
func goOnToggleWindow() {
	darwinMu.RLock()
	cb := darwinCallbacks.OnToggleWindow
	darwinMu.RUnlock()
	if cb != nil {
		cb()
	}
}

//export goOnDayView
func goOnDayView() {
	darwinMu.RLock()
	cb := darwinCallbacks.OnDayView
	darwinMu.RUnlock()
	if cb != nil {
		cb()
	}
}

//export goOnExport
func goOnExport() {
	darwinMu.RLock()
	cb := darwinCallbacks.OnExport
	darwinMu.RUnlock()
	if cb != nil {
		cb()
	}
}

//export goOnQuickShift
func goOnQuickShift() {
	darwinMu.RLock()
	cb := darwinCallbacks.OnQuickShift
	darwinMu.RUnlock()
	if cb != nil {
		cb()
	}
}

//export goOnQuit
func goOnQuit() {
	darwinMu.RLock()
	cb := darwinCallbacks.OnQuit
	darwinMu.RUnlock()
	if cb != nil {
		cb()
	}
}

type DarwinWindowManager struct {
	visible bool
	width   int
	height  int
	mu      sync.RWMutex
}

func newPlatformWindowManager() WindowManager {
	return &DarwinWindowManager{
		visible: true,
		width:   420,
		height:  560,
	}
}

func (m *DarwinWindowManager) InitStatusItem(callbacks StatusCallbacks) {
	darwinMu.Lock()
	darwinCallbacks = callbacks
	darwinMu.Unlock()
	C.DarwinInitStatusItem()
}

func (m *DarwinWindowManager) UpdateStatusTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.DarwinUpdateTitle(cTitle)
}

func (m *DarwinWindowManager) ShowPopover(width, height int) {
	m.mu.Lock()
	m.width = width
	m.height = height
	m.visible = true
	m.mu.Unlock()
	C.DarwinPositionPopover(C.int(width), C.int(height))
}

func (m *DarwinWindowManager) HidePopover() {
	m.mu.Lock()
	m.visible = false
	m.mu.Unlock()
	C.DarwinHidePopover()
}

func (m *DarwinWindowManager) TogglePopover(width, height int) {
	m.mu.Lock()
	m.width = width
	m.height = height
	m.visible = !m.visible
	m.mu.Unlock()
	C.DarwinTogglePopover(C.int(width), C.int(height))
}

func (m *DarwinWindowManager) IsVisible() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visible
}

func (m *DarwinWindowManager) SetWindowSize(width, height int) {
	m.mu.Lock()
	m.width = width
	m.height = height
	m.mu.Unlock()
	C.DarwinPositionPopover(C.int(width), C.int(height))
}
