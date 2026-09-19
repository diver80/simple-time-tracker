//go:build darwin && cgo

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include "manager_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"time"
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
	visible      bool
	width        int
	height       int
	shieldActive bool
	mu           sync.RWMutex
}

func newPlatformWindowManager() WindowManager {
	return &DarwinWindowManager{
		visible: true,
		width:   420,
		height:  580,
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

func (m *DarwinWindowManager) SetScreenShareShield(enable bool) {
	m.mu.Lock()
	m.shieldActive = enable
	m.mu.Unlock()
	cEnable := C.int(0)
	if enable {
		cEnable = C.int(1)
	}
	C.DarwinSetWindowSharingNone(cEnable)
}

func (m *DarwinWindowManager) IsScreenShareShieldEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shieldActive
}

func (m *DarwinWindowManager) TempHide(duration time.Duration) {
	m.HidePopover()
	if duration > 0 {
		time.AfterFunc(duration, func() {
			m.ShowPopover(m.width, m.height)
		})
	}
}

