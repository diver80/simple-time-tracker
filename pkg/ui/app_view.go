package ui

import (
	"fmt"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
	"time-tracker/pkg/window"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type ActiveTab int

const (
	TabTracker ActiveTab = iota
	TabCalendar
	TabExport
	TabProjects
)

// AppView is the root view containing tab navigation and view swapping.
type AppView struct {
	widget.WidgetBase

	repo      db.Repository
	timerSvc  *timer.TimerService
	winMgr    window.WindowManager
	activeTab ActiveTab

	hudView      *HUDView
	calendarView *CalendarView
	exportView   *ExportView
	projectView  *ProjectView

	tabTrackerBtn  *GlassButton
	tabCalendarBtn *GlassButton
	tabExportBtn   *GlassButton
	tabProjectsBtn *GlassButton
	tabShieldBtn   *GlassButton

	// Privacy & Screen Sharing State
	showPrivacyOverlay bool
	privacyMasked      bool

	hide15Btn       *GlassButton
	hide30Btn       *GlassButton
	hide60Btn       *GlassButton
	hideNowBtn      *GlassButton
	maskToggleBtn   *GlassButton
	closeOverlayBtn *GlassButton

	// Live timer state for persistent nav bar visibility
	timerState   timer.TimerState
	timerElapsed time.Duration
	activeEntry  *db.TimeEntry

	onRequestRedraw func()
	onResize        func(w, h int)
}

func NewAppView(repo db.Repository, timerSvc *timer.TimerService, winMgr window.WindowManager, onRequestRedraw func()) *AppView {
	a := &AppView{
		repo:            repo,
		timerSvc:        timerSvc,
		winMgr:          winMgr,
		activeTab:       TabTracker,
		onRequestRedraw: onRequestRedraw,
	}
	a.SetVisible(true)
	a.SetEnabled(true)

	// Create Subviews
	a.hudView = NewHUDView(timerSvc, repo, onRequestRedraw)
	a.calendarView = NewCalendarView(repo, onRequestRedraw)
	a.exportView = NewExportView(repo, onRequestRedraw)
	a.projectView = NewProjectView(repo, onRequestRedraw)

	// Create Tab Buttons
	a.tabTrackerBtn = NewGlassButton("⏱️ Tracker", func() {
		a.SwitchTab(TabTracker)
	}).SetCompact(true)

	a.tabCalendarBtn = NewGlassButton("📅 Tag", func() {
		a.SwitchTab(TabCalendar)
	}).SetCompact(true)

	a.tabExportBtn = NewGlassButton("📊 Export", func() {
		a.SwitchTab(TabExport)
	}).SetCompact(true)

	a.tabProjectsBtn = NewGlassButton("📁 Budgets", func() {
		a.SwitchTab(TabProjects)
	}).SetCompact(true)

	// Screen Sharing Shield Button
	a.tabShieldBtn = NewGlassButton("🛡️", func() {
		a.showPrivacyOverlay = !a.showPrivacyOverlay
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	}).SetCompact(true)

	// Temp Hide Snooze Buttons
	a.hide15Btn = NewGlassButton("🙈 15m Call", func() {
		a.showPrivacyOverlay = false
		if a.winMgr != nil {
			a.winMgr.TempHide(15 * time.Minute)
		}
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	a.hide30Btn = NewGlassButton("🙈 30m Call", func() {
		a.showPrivacyOverlay = false
		if a.winMgr != nil {
			a.winMgr.TempHide(30 * time.Minute)
		}
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	a.hide60Btn = NewGlassButton("🙈 60m Call", func() {
		a.showPrivacyOverlay = false
		if a.winMgr != nil {
			a.winMgr.TempHide(60 * time.Minute)
		}
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	a.hideNowBtn = NewGlassButton("🙈 Bis Klick", func() {
		a.showPrivacyOverlay = false
		if a.winMgr != nil {
			a.winMgr.TempHide(0)
		}
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	a.maskToggleBtn = NewGlassButton("🔒 Vertrauliche Daten maskieren", func() {
		a.privacyMasked = !a.privacyMasked
		a.hudView.SetPrivacyMasked(a.privacyMasked)
		if a.privacyMasked {
			a.maskToggleBtn.SetText("👁️ Maskierung aufheben (Sichtbar)")
		} else {
			a.maskToggleBtn.SetText("🔒 Vertrauliche Daten maskieren")
		}
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	a.closeOverlayBtn = NewGlassButton("✕ Schließen", func() {
		a.showPrivacyOverlay = false
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	}).SetCompact(true)

	// Hook timer tick to keep the top nav bar timer always visible across all tabs
	a.timerSvc.OnTick(func(elapsed time.Duration, state timer.TimerState, entry *db.TimeEntry) {
		a.timerElapsed = elapsed
		a.timerState = state
		a.activeEntry = entry
		a.updateTrackerTabLabel()
		if a.onRequestRedraw != nil {
			a.onRequestRedraw()
		}
	})

	// Initial check in case timer was already restored from DB
	a.timerState, a.activeEntry, a.timerElapsed = a.timerSvc.GetCurrentState()
	a.updateTrackerTabLabel()

	return a
}

func (a *AppView) updateTrackerTabLabel() {
	switch a.timerState {
	case timer.StateRunning:
		a.tabTrackerBtn.SetText(fmt.Sprintf("⏱️ %s", timer.FormatDurationHHMMSS(a.timerElapsed)))
	case timer.StateQuickShift:
		a.tabTrackerBtn.SetText(fmt.Sprintf("⚡ %s", timer.FormatDurationHHMMSS(a.timerElapsed)))
	case timer.StatePaused:
		a.tabTrackerBtn.SetText(fmt.Sprintf("⏸️ %s", timer.FormatDurationHHMMSS(a.timerElapsed)))
	default:
		a.tabTrackerBtn.SetText("⏱️ Tracker")
	}
}

func (a *AppView) SetOnResize(fn func(w, h int)) {
	a.onResize = fn
}

func (a *AppView) SwitchTab(tab ActiveTab) {
	a.activeTab = tab

	targetW := 420
	targetH := 580
	if tab == TabCalendar {
		targetH = 640
		a.calendarView.Refresh()
	} else if tab == TabExport {
		targetH = 580
		a.exportView.Refresh()
	} else if tab == TabProjects {
		targetH = 580
		a.projectView.Refresh()
	} else {
		a.hudView.refreshState()
	}

	if a.winMgr != nil {
		a.winMgr.SetWindowSize(targetW, targetH)
	}
	if a.onResize != nil {
		a.onResize(targetW, targetH)
	}
	if a.onRequestRedraw != nil {
		a.onRequestRedraw()
	}
}

func (a *AppView) SwitchToCalendar() {
	a.SwitchTab(TabCalendar)
}

func (a *AppView) SwitchToExport() {
	a.SwitchTab(TabExport)
}

func (a *AppView) TriggerQuickShift() {
	a.SwitchTab(TabTracker)
	_, _ = a.timerSvc.StartQuickShift("Unterbrechung / Anruf")
	a.hudView.refreshState()
}

func (a *AppView) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(420)
	h := c.ConstrainHeight(580)
	if a.activeTab == TabCalendar {
		h = c.ConstrainHeight(640)
	}
	return geometry.Sz(w, h)
}

func (a *AppView) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := a.Bounds()
	theme := DefaultDarkTheme

	// Overall Container Background
	canvas.DrawRoundRect(b, theme.Background, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	// Top Tab Bar
	tabY := b.Min.Y + 10

	// Tab Tracker: width 94 to fit "⏱️ 00:00:00" cleanly
	a.tabTrackerBtn.SetBounds(geometry.NewRect(b.Min.X+16, tabY, 94, 26))
	if a.activeTab == TabTracker {
		a.tabTrackerBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else if a.timerState == timer.StateRunning {
		// Prominently highlight running background timer with emerald glow
		a.tabTrackerBtn.SetCustomColors(theme.AccentPrimary, widget.RGBA8(24, 40, 32, 255), theme.AccentPrimary)
	} else if a.timerState == timer.StateQuickShift {
		// Prominently highlight active QuickShift interruption
		a.tabTrackerBtn.SetCustomColors(theme.QuickShiftColor, widget.RGBA8(44, 34, 18, 255), theme.QuickShiftColor)
	} else if a.timerState == timer.StatePaused {
		a.tabTrackerBtn.SetCustomColors(theme.TextMuted, theme.TabInactive, theme.LineSeparator)
	} else {
		a.tabTrackerBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabTrackerBtn.Draw(ctx, canvas)

	// Tab Calendar: width 74
	a.tabCalendarBtn.SetBounds(geometry.NewRect(b.Min.X+114, tabY, 74, 26))
	if a.activeTab == TabCalendar {
		a.tabCalendarBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabCalendarBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabCalendarBtn.Draw(ctx, canvas)

	// Tab Export: width 82
	a.tabExportBtn.SetBounds(geometry.NewRect(b.Min.X+192, tabY, 82, 26))
	if a.activeTab == TabExport {
		a.tabExportBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabExportBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabExportBtn.Draw(ctx, canvas)

	// Tab Projects: width 84
	a.tabProjectsBtn.SetBounds(geometry.NewRect(b.Min.X+278, tabY, 84, 26))
	if a.activeTab == TabProjects {
		a.tabProjectsBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabProjectsBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabProjectsBtn.Draw(ctx, canvas)

	// Screen Sharing Shield Button: width 36
	a.tabShieldBtn.SetBounds(geometry.NewRect(b.Min.X+368, tabY, 36, 26))
	if a.showPrivacyOverlay {
		a.tabShieldBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else if a.privacyMasked {
		a.tabShieldBtn.SetCustomColors(theme.QuickShiftColor, widget.RGBA8(44, 34, 18, 255), theme.QuickShiftColor)
	} else {
		a.tabShieldBtn.SetCustomColors(theme.AccentPrimary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabShieldBtn.Draw(ctx, canvas)

	// Content Subview Area
	contentRect := geometry.NewRect(b.Min.X+10, tabY+34, b.Width()-20, b.Height()-46)

	switch a.activeTab {
	case TabTracker:
		a.hudView.SetBounds(contentRect)
		a.hudView.Draw(ctx, canvas)
	case TabCalendar:
		a.calendarView.SetBounds(contentRect)
		a.calendarView.Draw(ctx, canvas)
	case TabExport:
		a.exportView.SetBounds(contentRect)
		a.exportView.Draw(ctx, canvas)
	case TabProjects:
		a.projectView.SetBounds(contentRect)
		a.projectView.Draw(ctx, canvas)
	}

	// Draw Privacy & Screen Sharing Shield Overlay
	if a.showPrivacyOverlay {
		overlayRect := geometry.NewRect(contentRect.Min.X+10, contentRect.Min.Y+10, contentRect.Width()-20, 380)
		canvas.DrawRoundRect(overlayRect, widget.RGBA8(18, 22, 32, 250), 12)
		canvas.StrokeRoundRect(overlayRect, theme.AccentPrimary, 12, 1.5)

		// Header
		canvas.DrawText("🛡️ Screen Sharing & Call Schutz",
			geometry.NewRect(overlayRect.Min.X+16, overlayRect.Min.Y+20, overlayRect.Width()-32, 22),
			15, widget.RGBA8(255, 255, 255, 255), true, widget.TextAlignCenter)

		// Status Badge
		canvas.DrawText("✓ macOS Screen-Capture Shield ist AKTIV",
			geometry.NewRect(overlayRect.Min.X+16, overlayRect.Min.Y+48, overlayRect.Width()-32, 18),
			12, theme.AccentPrimary, true, widget.TextAlignCenter)

		canvas.DrawText("Dieses Fenster wird von Teams & Zoom bei Bildschirmübertragung\nautomatisch ausgeblendet (NSWindowSharingNone).",
			geometry.NewRect(overlayRect.Min.X+16, overlayRect.Min.Y+70, overlayRect.Width()-32, 32),
			10, theme.TextSecondary, false, widget.TextAlignCenter)

		// Temp Hide Section
		sepY := overlayRect.Min.Y + 112
		canvas.DrawRect(geometry.NewRect(overlayRect.Min.X+16, sepY, overlayRect.Width()-32, 1), theme.LineSeparator)

		canvas.DrawText("Fenster temporär ausblenden für laufenden Call:",
			geometry.NewRect(overlayRect.Min.X+16, sepY+10, overlayRect.Width()-32, 18),
			11, theme.TextPrimary, true, widget.TextAlignLeft)

		btnW := (overlayRect.Width() - 42) / 2
		btnY1 := sepY + 34
		a.hide15Btn.SetBounds(geometry.NewRect(overlayRect.Min.X+16, btnY1, btnW, 30))
		a.hide15Btn.Draw(ctx, canvas)

		a.hide30Btn.SetBounds(geometry.NewRect(overlayRect.Min.X+26+btnW, btnY1, btnW, 30))
		a.hide30Btn.Draw(ctx, canvas)

		btnY2 := btnY1 + 38
		a.hide60Btn.SetBounds(geometry.NewRect(overlayRect.Min.X+16, btnY2, btnW, 30))
		a.hide60Btn.Draw(ctx, canvas)

		a.hideNowBtn.SetBounds(geometry.NewRect(overlayRect.Min.X+26+btnW, btnY2, btnW, 30))
		a.hideNowBtn.Draw(ctx, canvas)

		// Data Masking Section
		sepY2 := btnY2 + 48
		canvas.DrawRect(geometry.NewRect(overlayRect.Min.X+16, sepY2, overlayRect.Width()-32, 1), theme.LineSeparator)

		a.maskToggleBtn.SetBounds(geometry.NewRect(overlayRect.Min.X+16, sepY2+12, overlayRect.Width()-32, 32))
		if a.privacyMasked {
			a.maskToggleBtn.SetCustomColors(widget.RGBA8(0, 0, 0, 255), theme.QuickShiftColor, widget.RGBA8(217, 119, 6, 255))
		} else {
			a.maskToggleBtn.SetCustomColors(widget.RGBA8(240, 244, 250, 255), theme.TabInactive, theme.CardBorder)
		}
		a.maskToggleBtn.Draw(ctx, canvas)

		// Close button
		a.closeOverlayBtn.SetBounds(geometry.NewRect(overlayRect.Min.X+16, sepY2+52, overlayRect.Width()-32, 28))
		a.closeOverlayBtn.SetCustomColors(theme.TextSecondary, theme.InputBg, theme.CardBorder)
		a.closeOverlayBtn.Draw(ctx, canvas)
	}
}

func (a *AppView) Event(ctx widget.Context, e event.Event) bool {
	// Screen Sharing Shield Button
	if a.tabShieldBtn.Event(ctx, e) {
		return true
	}

	// Handle overlay interactions if open
	if a.showPrivacyOverlay {
		if a.hide15Btn.Event(ctx, e) {
			return true
		}
		if a.hide30Btn.Event(ctx, e) {
			return true
		}
		if a.hide60Btn.Event(ctx, e) {
			return true
		}
		if a.hideNowBtn.Event(ctx, e) {
			return true
		}
		if a.maskToggleBtn.Event(ctx, e) {
			return true
		}
		if a.closeOverlayBtn.Event(ctx, e) {
			return true
		}
		// Intercept other clicks while modal is open
		switch ev := e.(type) {
		case *event.MouseEvent:
			if ev.MouseType == event.MousePress {
				return true
			}
		}
	}

	// Check tab buttons
	if a.tabTrackerBtn.Event(ctx, e) {
		return true
	}
	if a.tabCalendarBtn.Event(ctx, e) {
		return true
	}
	if a.tabExportBtn.Event(ctx, e) {
		return true
	}
	if a.tabProjectsBtn.Event(ctx, e) {
		return true
	}

	// Route to active subview
	switch a.activeTab {
	case TabTracker:
		return a.hudView.Event(ctx, e)
	case TabCalendar:
		return a.calendarView.Event(ctx, e)
	case TabExport:
		return a.exportView.Event(ctx, e)
	case TabProjects:
		return a.projectView.Event(ctx, e)
	}

	return false
}

func (a *AppView) Children() []widget.Widget {
	return nil
}
