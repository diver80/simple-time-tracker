//go:build !darwin || !cgo

package window

import "time"

type FallbackWindowManager struct {
	visible bool
	width   int
	height  int
}

func newPlatformWindowManager() WindowManager {
	return &FallbackWindowManager{
		visible: true,
		width:   420,
		height:  560,
	}
}

func (m *FallbackWindowManager) InitStatusItem(callbacks StatusCallbacks) {}
func (m *FallbackWindowManager) UpdateStatusTitle(title string)            {}
func (m *FallbackWindowManager) ShowPopover(width, height int)            { m.visible = true }
func (m *FallbackWindowManager) HidePopover()                             { m.visible = false }
func (m *FallbackWindowManager) TogglePopover(width, height int)          { m.visible = !m.visible }
func (m *FallbackWindowManager) IsVisible() bool                          { return m.visible }
func (m *FallbackWindowManager) SetWindowSize(width, height int)          { m.width = width; m.height = height }
func (m *FallbackWindowManager) SetScreenShareShield(enable bool)          {}
func (m *FallbackWindowManager) IsScreenShareShieldEnabled() bool          { return false }
func (m *FallbackWindowManager) TempHide(duration time.Duration)           { m.visible = false }


