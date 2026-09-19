package main

import "sync"

// desktopUpdates transfers native menu actions and timer invalidations to the
// render thread. Waking the OS window alone does not invalidate cached UI scenes.
type desktopUpdates struct {
	mu      sync.Mutex
	dirty   bool
	actions []func()
	wake    func()
}

func (u *desktopUpdates) requestRedraw() {
	u.mu.Lock()
	u.dirty = true
	u.mu.Unlock()
	u.wake()
}

func (u *desktopUpdates) post(action func()) {
	u.mu.Lock()
	u.actions = append(u.actions, action)
	u.dirty = true
	u.mu.Unlock()
	u.wake()
}

// flush is called only by the render thread, before the frame is painted.
func (u *desktopUpdates) flush(invalidate func()) {
	u.mu.Lock()
	actions, dirty := u.actions, u.dirty
	u.actions = nil
	u.dirty = false
	u.mu.Unlock()

	for _, action := range actions {
		action()
	}
	if dirty {
		invalidate()
	}
}
