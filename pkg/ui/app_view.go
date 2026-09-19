package ui

import (
	"yokto-time/pkg/db"
	"yokto-time/pkg/timer"
	"yokto-time/pkg/window"

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

	return a
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
	tabW := float32(88)

	// Tab Tracker
	a.tabTrackerBtn.SetBounds(geometry.NewRect(b.Min.X+16, tabY, tabW, 26))
	if a.activeTab == TabTracker {
		a.tabTrackerBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabTrackerBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabTrackerBtn.Draw(ctx, canvas)

	// Tab Calendar
	a.tabCalendarBtn.SetBounds(geometry.NewRect(b.Min.X+110, tabY, tabW, 26))
	if a.activeTab == TabCalendar {
		a.tabCalendarBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabCalendarBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabCalendarBtn.Draw(ctx, canvas)

	// Tab Export
	a.tabExportBtn.SetBounds(geometry.NewRect(b.Min.X+204, tabY, tabW, 26))
	if a.activeTab == TabExport {
		a.tabExportBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabExportBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabExportBtn.Draw(ctx, canvas)

	// Tab Projects
	a.tabProjectsBtn.SetBounds(geometry.NewRect(b.Min.X+298, tabY, tabW, 26))
	if a.activeTab == TabProjects {
		a.tabProjectsBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.TabActive, theme.AccentPrimary)
	} else {
		a.tabProjectsBtn.SetCustomColors(theme.TextSecondary, theme.TabInactive, theme.LineSeparator)
	}
	a.tabProjectsBtn.Draw(ctx, canvas)

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
}

func (a *AppView) Event(ctx widget.Context, e event.Event) bool {
	// Check tab buttons first
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
