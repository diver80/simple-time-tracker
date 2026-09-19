#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#import "manager_darwin.h"

extern void goOnToggleWindow(void);
extern void goOnDayView(void);
extern void goOnExport(void);
extern void goOnQuickShift(void);
extern void goOnQuit(void);

static NSWindow *g_appWindow = nil;
static int g_isVisible = 1;
static int g_lastWidth = 420;
static int g_lastHeight = 580;

static NSWindow* GetGogpuWindow(void) {
    if (g_appWindow) return g_appWindow;
    NSArray *windows = [NSApp windows];
    if ([windows count] > 0) {
        g_appWindow = [windows objectAtIndex:0];
    }
    return g_appWindow;
}

void DarwinInitStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win setLevel:NSNormalWindowLevel];
            [win center];
        }
    });
}

void DarwinUpdateTitle(const char *title) {
    if (!title) return;
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win setTitle:nsTitle];
        }
    });
}

void DarwinPositionPopover(int width, int height) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = GetGogpuWindow();
        if (!win) return;

        g_lastWidth = width;
        g_lastHeight = height;

        NSRect curFrame = [win frame];
        NSRect newFrame = NSMakeRect(curFrame.origin.x, curFrame.origin.y, (CGFloat)width, (CGFloat)height);
        [win setFrame:newFrame display:YES animate:YES];
        [win makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        g_isVisible = 1;
    });
}

void DarwinHidePopover(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = GetGogpuWindow();
        if (win) {
            [win orderOut:nil];
            g_isVisible = 0;
        }
    });
}

void DarwinTogglePopover(int width, int height) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_isVisible) {
            DarwinHidePopover();
        } else {
            DarwinPositionPopover(width, height);
        }
    });
}
