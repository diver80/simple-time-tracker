package ui

import (
	"fmt"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// HUDView is the main compact time tracking widget.
type HUDView struct {
	widget.WidgetBase

	timerSvc *timer.TimerService
	repo     db.Repository

	// Current state cache
	state       timer.TimerState
	activeEntry *db.TimeEntry
	elapsed     time.Duration
	todayTotal  time.Duration

	// Form inputs for Idle state or inline edits
	inputTaskName    string
	inputBookingText string
	selectedProject  *db.Project

	taskInput    *TextInput
	notesInput   *TextInput
	projectInput *ProjectPicker
	errorText    string

	// UI interactive elements
	startBtn      *GlassButton
	stopBtn       *GlassButton
	quickShiftBtn *GlassButton
	pauseBtn      *GlassButton
	resumeBtn     *GlassButton
	finishQSBtn   *GlassButton

	onRequestRedraw func()
	isTypingNotes   bool
	privacyMasked   bool
}

func (h *HUDView) SetPrivacyMasked(masked bool) {
	h.privacyMasked = masked
	if h.onRequestRedraw != nil {
		h.onRequestRedraw()
	}
}

func NewHUDView(timerSvc *timer.TimerService, repo db.Repository, onRequestRedraw func()) *HUDView {
	h := &HUDView{
		timerSvc:        timerSvc,
		repo:            repo,
		onRequestRedraw: onRequestRedraw,
	}
	h.SetVisible(true)
	h.SetEnabled(true)

	h.taskInput = NewTextInput("Was möchtest du tun?", false, func(text string) {
		h.inputTaskName = text
		if h.activeEntry != nil {
			if err := h.timerSvc.UpdateTaskName(text); err != nil {
				h.errorText = err.Error()
			} else {
				h.errorText = ""
			}
		}
	})
	h.notesInput = NewTextInput("Hier Notizen eintragen …", true, func(text string) {
		h.inputBookingText = text
		if h.activeEntry != nil {
			if err := h.timerSvc.UpdateBookingText(text); err != nil {
				h.errorText = err.Error()
			} else {
				h.errorText = ""
			}
		}
	})
	h.projectInput = NewProjectPicker(repo, func(project *db.Project) {
		if h.activeEntry != nil {
			var id *int64
			if project != nil { id = &project.ID }
			if err := h.timerSvc.SetProject(id); err != nil {
				h.errorText = err.Error()
				h.projectInput.SetProjectID(h.activeEntry.ProjectID)
				return
			}
		}
		h.selectedProject = project
		h.errorText = ""
	})
	for _, input := range []widget.Widget{h.taskInput, h.notesInput, h.projectInput} {
		input.(interface{ SetParent(widget.Widget) }).SetParent(h)
	}

	// Initialize Buttons
	h.startBtn = NewGlassButton("▶ Start Timer", func() {
		var projID *int64
		if h.selectedProject != nil {
			projID = &h.selectedProject.ID
		}
		task := h.inputTaskName
		if task == "" {
			task = "Arbeitsaufgabe"
		}
		entry, err := h.timerSvc.Start(projID, task, true)
		if err == nil && entry != nil {
			if h.inputBookingText != "" {
				_ = h.timerSvc.UpdateBookingText(h.inputBookingText)
			}
		}
		h.refreshState()
	})
	h.startBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), DefaultDarkTheme.AccentPrimary, DefaultDarkTheme.AccentHover)

	h.stopBtn = NewGlassButton("■ Stopp", func() {
		_, _ = h.timerSvc.Stop()
		h.inputTaskName = ""
		h.inputBookingText = ""
		h.refreshState()
	})
	h.stopBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), DefaultDarkTheme.StopColor, widget.RGBA8(220, 38, 38, 255))

	h.quickShiftBtn = NewGlassButton("⚡ QuickShift", func() {
		_, _ = h.timerSvc.StartQuickShift("Unterbrechung / Anruf")
		h.refreshState()
	})
	h.quickShiftBtn.SetCustomColors(widget.RGBA8(0, 0, 0, 255), DefaultDarkTheme.QuickShiftColor, widget.RGBA8(217, 119, 6, 255))

	h.finishQSBtn = NewGlassButton("✓ Unterbrechung beenden & Hauptaufgabe fortsetzen", func() {
		_ = h.timerSvc.FinishQuickShift()
		h.refreshState()
	})
	h.finishQSBtn.SetCustomColors(widget.RGBA8(0, 0, 0, 255), DefaultDarkTheme.QuickShiftColor, widget.RGBA8(217, 119, 6, 255))

	h.pauseBtn = NewGlassButton("⏸ Pause", func() {
		_ = h.timerSvc.Pause()
		h.refreshState()
	})

	h.resumeBtn = NewGlassButton("▶ Weiter", func() {
		_ = h.timerSvc.Resume()
		h.refreshState()
	})

	// Hook timer tick; only request redraw, state refreshed from UI thread
	h.timerSvc.OnTick(func(elapsed time.Duration, state timer.TimerState, entry *db.TimeEntry) {
		if h.onRequestRedraw != nil {
			h.onRequestRedraw()
		}
	})

	h.refreshState()
	return h
}

func (h *HUDView) refreshState() {
	h.state, h.activeEntry, h.elapsed = h.timerSvc.GetCurrentState()
	if h.activeEntry != nil {
		h.inputTaskName = h.activeEntry.TaskName
		h.inputBookingText = h.activeEntry.BookingText
	}

	// Calculate today's total
	entries, _ := h.repo.ListEntriesForDay(time.Now())
	var totalSec int64
	for _, e := range entries {
		totalSec += e.DurationSec
	}
	h.todayTotal = time.Duration(totalSec) * time.Second
	if h.onRequestRedraw != nil {
		h.onRequestRedraw()
	}
}

func (h *HUDView) refreshTimerSnapshot() {
	// Called on UI thread from Draw to safely fetch current timer state
	h.state, h.activeEntry, h.elapsed = h.timerSvc.GetCurrentState()
}

func (h *HUDView) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	// HUD dimensions: 400px width, responsive height
	w := c.ConstrainWidth(400)
	hSize := c.ConstrainHeight(520)
	return geometry.Sz(w, hSize)
}

func (h *HUDView) Draw(ctx widget.Context, canvas widget.Canvas) {
	// Refresh timer snapshot on UI thread to avoid race with ticker goroutine
	h.refreshTimerSnapshot()

	b := h.Bounds()
	theme := DefaultDarkTheme

	// Card Background
	canvas.DrawRoundRect(b, theme.CardBg, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	// Header banner: QuickShift alert or standard header
	headerY := b.Min.Y + 16
	if h.state == timer.StateQuickShift {
		qsBanner := geometry.NewRect(b.Min.X+16, headerY, b.Width()-32, 28)
		canvas.DrawRoundRect(qsBanner, theme.QuickShiftBg, 6)
		canvas.StrokeRoundRect(qsBanner, theme.QuickShiftColor, 6, 1.0)
		canvas.DrawText("⚡ QuickShift aktiv: Haupttimer wartet im Hintergrund",
			geometry.NewRect(qsBanner.Min.X+8, qsBanner.Min.Y+6, qsBanner.Width()-16, 16),
			11, theme.QuickShiftColor, true, widget.TextAlignCenter)
		headerY += 36
	}

	// Big Timer Readout
	timeStr := timer.FormatDurationHHMMSS(h.elapsed)
	if h.state == timer.StateIdle {
		timeStr = "00:00:00"
	}
	timerRect := geometry.NewRect(b.Min.X+20, headerY, b.Width()-40, 48)
	timerColor := theme.TextPrimary
	if h.state == timer.StateRunning {
		timerColor = theme.AccentPrimary
	} else if h.state == timer.StateQuickShift {
		timerColor = theme.QuickShiftColor
	} else if h.state == timer.StatePaused {
		timerColor = theme.TextMuted
	}
	canvas.DrawText(timeStr, timerRect, 38, timerColor, true, widget.TextAlignCenter)

	// Task Name Display / Input
	taskY := timerRect.Max.Y + 12
	taskLabel := "Aufgabe:"
	canvas.DrawText(taskLabel, geometry.NewRect(b.Min.X+20, taskY, 80, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)

	taskBox := geometry.NewRect(b.Min.X+20, taskY+18, b.Width()-40, 32)
	h.taskInput.SetBounds(taskBox)
	h.taskInput.SetMasked(h.privacyMasked)
	h.taskInput.Draw(ctx, canvas)

	// Project & Customer Indicator
	projY := taskBox.Max.Y + 12
	canvas.DrawText("Kunde & Projekt:", geometry.NewRect(b.Min.X+20, projY, 120, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)

	projBox := geometry.NewRect(b.Min.X+20, projY+18, b.Width()-40, 28)
	h.projectInput.SetBounds(projBox)
	// Note: ProjectPicker doesn't have SetMasked method yet; privacy masking would need to be implemented there

	// Live Booking Text Notes Box (Key feature requested by user)
	notesY := projBox.Max.Y + 12
	canvas.DrawText("Live-Buchungstext & Notizen:", geometry.NewRect(b.Min.X+20, notesY, 200, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)

	notesBox := geometry.NewRect(b.Min.X+20, notesY+18, b.Width()-40, 90)
	h.notesInput.SetBounds(notesBox)
	h.notesInput.SetMasked(h.privacyMasked)
	h.notesInput.Draw(ctx, canvas)

	// Action Buttons Layout
	btnY := notesBox.Max.Y + 16
	btnW := (b.Width() - 50) / 2

	if h.state == timer.StateIdle {
		h.startBtn.SetBounds(geometry.NewRect(b.Min.X+20, btnY, b.Width()-40, 36))
		h.startBtn.Draw(ctx, canvas)
	} else if h.state == timer.StateQuickShift {
		h.finishQSBtn.SetBounds(geometry.NewRect(b.Min.X+20, btnY, b.Width()-40, 36))
		h.finishQSBtn.Draw(ctx, canvas)

		h.stopBtn.SetBounds(geometry.NewRect(b.Min.X+20, btnY+44, b.Width()-40, 30))
		h.stopBtn.Draw(ctx, canvas)
	} else {
		// Running or Paused
		h.stopBtn.SetBounds(geometry.NewRect(b.Min.X+20, btnY, btnW, 36))
		h.stopBtn.Draw(ctx, canvas)

		h.quickShiftBtn.SetBounds(geometry.NewRect(b.Min.X+30+btnW, btnY, btnW, 36))
		h.quickShiftBtn.Draw(ctx, canvas)
	}

	// Bottom Footer: Today's Summary
	footerY := b.Max.Y - 32
	sepRect := geometry.NewRect(b.Min.X+16, footerY-8, b.Width()-32, 1)
	canvas.DrawRect(sepRect, theme.LineSeparator)

	todayStr := fmt.Sprintf("Heute erfasst: %s", timer.FormatDurationHHMM(h.todayTotal))
	canvas.DrawText(todayStr, geometry.NewRect(b.Min.X+20, footerY, b.Width()-40, 18), 11, theme.TextSecondary, false, widget.TextAlignCenter)
	if h.errorText != "" {
		canvas.DrawText(h.errorText, geometry.NewRect(b.Min.X+20, footerY-42, b.Width()-40, 30), 10, theme.StopColor, false, widget.TextAlignLeft)
	}
	// The selection menu must cover other controls, not be painted underneath.
	h.projectInput.Draw(ctx, canvas)
}

func (h *HUDView) Event(ctx widget.Context, e event.Event) bool {
	// Delegate to buttons
	if h.state == timer.StateIdle {
		if h.startBtn.Event(ctx, e) {
			return true
		}
	} else if h.state == timer.StateQuickShift {
		if h.finishQSBtn.Event(ctx, e) {
			return true
		}
		if h.stopBtn.Event(ctx, e) {
			return true
		}
	} else {
		if h.stopBtn.Event(ctx, e) {
			return true
		}
		if h.quickShiftBtn.Event(ctx, e) {
			return true
		}
	}

	// Keyboard typing for live booking text & task name
	switch ev := e.(type) {
	case *event.KeyEvent:
		if ev.KeyType == event.KeyPress {
			if ev.Key == event.KeyBackspace {
				if len(h.inputBookingText) > 0 {
					h.inputBookingText = h.inputBookingText[:len(h.inputBookingText)-1]
					if h.state == timer.StateRunning || h.state == timer.StateQuickShift {
						_ = h.timerSvc.UpdateBookingText(h.inputBookingText)
					}
					if h.onRequestRedraw != nil {
						h.onRequestRedraw()
					}
					return true
				}
			} else if ev.Rune >= 32 {
				h.inputBookingText += string(ev.Rune)
				if h.state == timer.StateRunning || h.state == timer.StateQuickShift {
					_ = h.timerSvc.UpdateBookingText(h.inputBookingText)
				}
				if h.onRequestRedraw != nil {
					h.onRequestRedraw()
				}
				return true
			}
		}
	}

	return false
}

func (h *HUDView) Children() []widget.Widget {
	return nil
}
