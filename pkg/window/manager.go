package window

import "time"

type WindowManager interface {
	InitStatusItem(callbacks StatusCallbacks)
	UpdateStatusTitle(title string)
	ShowPopover(width, height int)
	HidePopover()
	TogglePopover(width, height int)
	IsVisible() bool
	SetWindowSize(width, height int)
	SetScreenShareShield(enable bool)
	IsScreenShareShieldEnabled() bool
	TempHide(duration time.Duration)
}

type StatusCallbacks struct {
	OnToggleWindow func()
	OnDayView      func()
	OnExport       func()
	OnQuickShift   func()
	OnQuit         func()
}

// NewWindowManager creates platform-specific window manager.
func NewWindowManager() WindowManager {
	return newPlatformWindowManager()
}
