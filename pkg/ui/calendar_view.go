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

// CalendarView renders a visual daily timeline showing tasks as calendar blocks.
type CalendarView struct {
	widget.WidgetBase

	repo                db.Repository
	currentDay          time.Time
	entries             []db.TimeEntry
	selectedEntry       *db.TimeEntry
	onRequestRedraw     func()
	onEntriesChanged    func()  // Callback after successful mutations

	prevBtn  *GlassButton
	todayBtn *GlassButton
	nextBtn  *GlassButton
	addBtn   *GlassButton
	listModeBtn *GlassButton

	editor              *EntryEditor     // Active editor if any
	isListMode          bool             // True for list view, false for timeline
	listScrollOffset    int              // Scroll position in list mode

	// Record clickable bounds for hit testing
	timelineBlockBounds []struct {
		entryID int64
		bounds  geometry.Rect
	}
	listItemBounds []struct {
		entryID int64
		bounds  geometry.Rect
	}
}

const (
	dayStartHour = 0  // 00:00
	dayEndHour   = 24 // 24:00
)

func NewCalendarView(repo db.Repository, onRequestRedraw func()) *CalendarView {
	cv := &CalendarView{
		repo:            repo,
		currentDay:      time.Now(),
		onRequestRedraw: onRequestRedraw,
		isListMode:      false,
	}
	cv.SetVisible(true)
	cv.SetEnabled(true)

	cv.prevBtn = NewGlassButton("< Gestern", func() {
		cv.currentDay = cv.currentDay.AddDate(0, 0, -1)
		cv.Refresh()
	}).SetCompact(true)

	cv.todayBtn = NewGlassButton("Heute", func() {
		cv.currentDay = time.Now()
		cv.Refresh()
	}).SetCompact(true)

	cv.nextBtn = NewGlassButton("Morgen >", func() {
		cv.currentDay = cv.currentDay.AddDate(0, 0, 1)
		cv.Refresh()
	}).SetCompact(true)

	cv.addBtn = NewGlassButton("+ Buchung", func() {
		cv.openEditor(nil)
	}).SetCompact(true)

	cv.listModeBtn = NewGlassButton("Liste", func() {
		cv.isListMode = !cv.isListMode
		cv.listScrollOffset = 0
		if cv.isListMode {
			cv.listModeBtn.SetText("Tag")
		} else {
			cv.listModeBtn.SetText("Liste")
		}
		if cv.onRequestRedraw != nil {
			cv.onRequestRedraw()
		}
	}).SetCompact(true)

	// Set parent for dirty redraw propagation
	cv.prevBtn.SetParent(cv)
	cv.todayBtn.SetParent(cv)
	cv.nextBtn.SetParent(cv)
	cv.addBtn.SetParent(cv)
	cv.listModeBtn.SetParent(cv)

	cv.Refresh()
	return cv
}

// SetOnEntriesChanged sets the callback for entry mutations.
func (cv *CalendarView) SetOnEntriesChanged(f func()) {
	cv.onEntriesChanged = f
}

// openEditor opens the editor for a given entry (nil for new entry).
// Centralizes editor setup and ensures proper callback handling.
func (cv *CalendarView) openEditor(entry *db.TimeEntry) {
	cv.editor = NewEntryEditor(cv.repo, entry, cv.currentDay, func() {
		// After successful save/delete, update currentDay and refresh
		if cv.editor != nil && cv.editor.currentDay != cv.currentDay {
			cv.currentDay = cv.editor.currentDay
		}
		cv.Refresh()
		if cv.onEntriesChanged != nil {
			cv.onEntriesChanged()
		}
	}, func() {
		// On cancel or close
		cv.editor = nil
		cv.Refresh()
		if cv.onRequestRedraw != nil {
			cv.onRequestRedraw()
		}
	}, cv.onRequestRedraw)
	if cv.onRequestRedraw != nil {
		cv.onRequestRedraw()
	}
}

// openEditorWithTimes opens the editor for creating a new entry with pre-filled times.
func (cv *CalendarView) openEditorWithTimes(start, end time.Time) {
	cv.openEditor(nil)
	if cv.editor != nil {
		cv.editor.SetTimes(start, end)
	}
}

func (cv *CalendarView) Refresh() {
	entries, err := cv.repo.ListEntriesForDay(cv.currentDay)
	if err == nil {
		cv.entries = entries
	}
	cv.selectedEntry = nil
	if cv.onRequestRedraw != nil {
		cv.onRequestRedraw()
	}
}

func (cv *CalendarView) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(420)
	h := c.ConstrainHeight(560)
	return geometry.Sz(w, h)
}

func (cv *CalendarView) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := cv.Bounds()
	theme := &DefaultDarkTheme

	// Background
	canvas.DrawRoundRect(b, theme.CardBg, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	// Top Navigation Bar
	navY := b.Min.Y + 12
	cv.prevBtn.SetBounds(geometry.NewRect(b.Min.X+16, navY, 68, 24))
	cv.prevBtn.Draw(ctx, canvas)

	cv.todayBtn.SetBounds(geometry.NewRect(b.Min.X+90, navY, 50, 24))
	cv.todayBtn.Draw(ctx, canvas)

	cv.nextBtn.SetBounds(geometry.NewRect(b.Max.X-84, navY, 68, 24))
	cv.nextBtn.Draw(ctx, canvas)

	// Date Title (e.g. "Sa, 19.09.2026")
	dateStr := cv.currentDay.Format("02.01.2006")
	canvas.DrawText(dateStr, geometry.NewRect(b.Min.X+144, navY+3, b.Width()-232, 18), 12, theme.TextPrimary, true, widget.TextAlignCenter)

	// If editor is open, show it instead of timeline
	if cv.editor != nil {
		cv.editor.SetBounds(geometry.NewRect(b.Min.X+10, b.Min.Y+50, b.Width()-20, b.Height()-70))
		cv.editor.Draw(ctx, canvas)
		return
	}

	// Check for active running entries
	hasActiveEntry := false
	for _, e := range cv.entries {
		if e.EndedAt == nil {
			hasActiveEntry = true
			break
		}
	}

	// Top action buttons
	actionY := navY + 36
	cv.addBtn.SetBounds(geometry.NewRect(b.Min.X+16, actionY, 80, 24))
	cv.addBtn.Draw(ctx, canvas)

	listBtnW := float32(50)
	if cv.isListMode {
		cv.listModeBtn.SetText("Tag")
	} else {
		cv.listModeBtn.SetText("Liste")
	}
	cv.listModeBtn.SetBounds(geometry.NewRect(b.Max.X-16-listBtnW, actionY, listBtnW, 24))
	cv.listModeBtn.Draw(ctx, canvas)

	if hasActiveEntry {
		warningText := "Bitte laufenden Timer zuerst stoppen"
		canvas.DrawText(warningText, geometry.NewRect(b.Min.X+110, actionY+4, 200, 16), 9, widget.RGBA8(255, 100, 100, 255), true, widget.TextAlignLeft)
	}

	// Timeline or List View
	contentTop := actionY + 36
	contentBottom := b.Max.Y - 50

	if cv.isListMode {
		cv.drawListMode(ctx, canvas, b, contentTop, contentBottom, hasActiveEntry, theme)
	} else {
		cv.drawTimelineMode(ctx, canvas, b, contentTop, contentBottom, hasActiveEntry, theme)
	}

	// Day Footer Summary
	var dayTotalSec int64
	var billableSec int64
	for _, e := range cv.entries {
		dayTotalSec += e.DurationSec
		if e.IsBillable {
			billableSec += e.DurationSec
		}
	}
	sepY := b.Max.Y - 36
	canvas.DrawRect(geometry.NewRect(b.Min.X+16, sepY, b.Width()-32, 1), theme.LineSeparator)

	summaryStr := fmt.Sprintf("Gesamt: %s  |  Abrechenbar: %s",
		timer.FormatDurationHHMM(time.Duration(dayTotalSec)*time.Second),
		timer.FormatDurationHHMM(time.Duration(billableSec)*time.Second),
	)
	canvas.DrawText(summaryStr, geometry.NewRect(b.Min.X+20, sepY+8, b.Width()-40, 18), 11, theme.TextSecondary, false, widget.TextAlignCenter)
}

func (cv *CalendarView) drawTimelineMode(ctx widget.Context, canvas widget.Canvas, b geometry.Rect, contentTop, contentBottom float32, hasActiveEntry bool, theme *AppTheme) {
	timelineHeight := contentBottom - contentTop
	timelineGutterX := b.Min.X + 54
	timelineWidth := b.Max.X - timelineGutterX - 16

	totalHours := float32(dayEndHour - dayStartHour)
	hourHeight := timelineHeight / totalHours

	// Clear bounds for hit testing
	cv.timelineBlockBounds = nil

	// Draw Hour Grid lines & labels
	for h := dayStartHour; h <= dayEndHour; h++ {
		curY := contentTop + float32(h-dayStartHour)*hourHeight
		timeLbl := fmt.Sprintf("%02d:00", h)
		canvas.DrawText(timeLbl, geometry.NewRect(b.Min.X+8, curY-6, 40, 12), 9, theme.TextMuted, false, widget.TextAlignRight)

		// Subtle divider line (more pronounced on even hours)
		lineRect := geometry.NewRect(timelineGutterX, curY, timelineWidth, 1)
		if h%2 == 0 {
			canvas.DrawRect(lineRect, widget.RGBA8(50, 58, 76, 180))
		} else {
			canvas.DrawRect(lineRect, widget.RGBA8(35, 42, 54, 120))
		}
	}

	// Draw Task Time Blocks
	for _, entry := range cv.entries {
		// Skip active entries
		if entry.EndedAt == nil {
			continue
		}

		start := entry.StartedAt.Local()
		startHour := float32(start.Hour()) + float32(start.Minute())/60.0
		if startHour < float32(dayStartHour) {
			startHour = float32(dayStartHour)
		}
		if startHour > float32(dayEndHour) {
			continue
		}

		durMin := float32(entry.DurationSec) / 60.0
		if durMin < 15 {
			durMin = 15 // minimum block size for visibility
		}
		durHours := durMin / 60.0

		blockY := contentTop + (startHour-float32(dayStartHour))*hourHeight
		blockH := durHours * hourHeight
		if blockH < 18 {
			blockH = 18
		}
		if blockY+blockH > contentBottom {
			blockH = contentBottom - blockY
		}

		blockRect := geometry.NewRect(timelineGutterX+4, blockY, timelineWidth-8, blockH)

		// Record bounds for hit testing
		cv.timelineBlockBounds = append(cv.timelineBlockBounds, struct {
			entryID int64
			bounds  geometry.Rect
		}{entry.ID, blockRect})

		// Card colors based on project or QuickShift
		blockBg := widget.RGBA8(35, 42, 58, 230)
		blockBorder := widget.RGBA8(60, 72, 98, 255)
		if entry.IsQuickShift {
			blockBg = theme.QuickShiftBg
			blockBorder = theme.QuickShiftColor
		} else if entry.ProjectColor != "" {
			pc := ParseHexColor(entry.ProjectColor)
			blockBg = widget.RGBA(pc.R*0.25, pc.G*0.25, pc.B*0.25, 0.9)
			blockBorder = widget.RGBA(pc.R*0.8, pc.G*0.8, pc.B*0.8, 1.0)
		}

		canvas.DrawRoundRect(blockRect, blockBg, 4)
		canvas.StrokeRoundRect(blockRect, blockBorder, 4, 1.0)

		// Left accent color strip
		stripRect := geometry.NewRect(blockRect.Min.X, blockRect.Min.Y, 3, blockRect.Height())
		canvas.DrawRoundRect(stripRect, blockBorder, 2)

		// Title inside block
		titleText := entry.TaskName
		if entry.IsQuickShift {
			titleText = "[QuickShift] " + entry.TaskName
		} else if entry.ProjectName != "" {
			titleText = fmt.Sprintf("[%s] %s", entry.ProjectName, entry.TaskName)
		}

		timeRange := fmt.Sprintf("%s (%s)", entry.StartedAt.Local().Format("15:04"), timer.FormatDurationHHMM(time.Duration(entry.DurationSec)*time.Second))
		canvas.DrawText(titleText, geometry.NewRect(blockRect.Min.X+8, blockRect.Min.Y+2, blockRect.Width()-14, 13), 9, theme.TextPrimary, true, widget.TextAlignLeft)
		if blockH >= 28 {
			canvas.DrawText(timeRange, geometry.NewRect(blockRect.Min.X+8, blockRect.Min.Y+15, blockRect.Width()-14, 11), 8, theme.TextSecondary, false, widget.TextAlignLeft)
		}
	}

	// Red Now-Line indicator if viewing today
	now := time.Now()
	if cv.currentDay.Format("2006-01-02") == now.Format("2006-01-02") {
		nowHour := float32(now.Hour()) + float32(now.Minute())/60.0
		if nowHour >= float32(dayStartHour) && nowHour <= float32(dayEndHour) {
			nowY := contentTop + (nowHour-float32(dayStartHour))*hourHeight
			nowLine := geometry.NewRect(timelineGutterX, nowY, timelineWidth, 1.5)
			canvas.DrawRect(nowLine, widget.RGBA8(239, 68, 68, 255))

			// Red circle at gutter
			canvas.DrawCircle(geometry.Pt(timelineGutterX, nowY), 3.0, widget.RGBA8(239, 68, 68, 255))
		}
	}
}

func (cv *CalendarView) drawListMode(ctx widget.Context, canvas widget.Canvas, b geometry.Rect, contentTop, contentBottom float32, hasActiveEntry bool, theme *AppTheme) {
	y := contentTop
	itemHeight := float32(32)

	// Clear bounds for hit testing
	cv.listItemBounds = nil

	// Display sorted entries with scrolling
	for i, entry := range cv.entries {
		if i < cv.listScrollOffset {
			continue
		}
		if y >= contentBottom {
			break
		}

		isActive := entry.EndedAt == nil
		bgColor := theme.InputBg
		if isActive {
			bgColor = widget.RGBA8(60, 50, 50, 200)
		}

		itemRect := geometry.NewRect(b.Min.X+16, y, b.Width()-32, itemHeight-2)

		// Record bounds for hit testing (only visible items, only complete entries)
		if !isActive {
			cv.listItemBounds = append(cv.listItemBounds, struct {
				entryID int64
				bounds  geometry.Rect
			}{entry.ID, itemRect})
		}

		canvas.DrawRoundRect(itemRect, bgColor, 4)
		canvas.StrokeRoundRect(itemRect, theme.InputBorder, 4, 1.0)

		// Entry text
		titleText := entry.TaskName
		if entry.IsQuickShift {
			titleText = "[QuickShift] " + entry.TaskName
		} else if entry.ProjectName != "" {
			titleText = fmt.Sprintf("[%s] %s", entry.ProjectName, entry.TaskName)
		}

		timeStr := entry.StartedAt.Local().Format("15:04")
		if entry.EndedAt != nil {
			timeStr = fmt.Sprintf("%s-%s", entry.StartedAt.Local().Format("15:04"), entry.EndedAt.Local().Format("15:04"))
		} else {
			timeStr += " (Running...)"
		}

		canvas.DrawText(titleText, geometry.NewRect(itemRect.Min.X+8, itemRect.Min.Y+4, itemRect.Width()-16, 12), 10, theme.TextPrimary, true, widget.TextAlignLeft)
		canvas.DrawText(timeStr, geometry.NewRect(itemRect.Min.X+8, itemRect.Min.Y+18, itemRect.Width()-16, 10), 9, theme.TextMuted, false, widget.TextAlignLeft)

		y += itemHeight
	}
}

func (cv *CalendarView) Event(ctx widget.Context, e event.Event) bool {
	// Forward to editor if open
	if cv.editor != nil {
		if cv.editor.Event(ctx, e) {
			return true
		}
		// Don't process underlying calendar buttons when editor exists
		return false
	}

	if cv.prevBtn.Event(ctx, e) {
		return true
	}
	if cv.todayBtn.Event(ctx, e) {
		return true
	}
	if cv.nextBtn.Event(ctx, e) {
		return true
	}
	if cv.addBtn.Event(ctx, e) {
		return true
	}
	if cv.listModeBtn.Event(ctx, e) {
		return true
	}

	// Handle timeline mode clicks and interactions
	if !cv.isListMode {
		switch ev := e.(type) {
		case *event.MouseEvent:
			if ev.MouseType == event.MousePress {
				// Hit-test in reverse order (most recently drawn first)
				for i := len(cv.timelineBlockBounds) - 1; i >= 0; i-- {
					bounds := cv.timelineBlockBounds[i]
					if bounds.bounds.Contains(ev.Position) {
						// Find the entry with this ID and open editor
						for j := range cv.entries {
							if cv.entries[j].ID == bounds.entryID {
								cv.openEditor(&cv.entries[j])
								return true
							}
						}
					}
				}

				// Check if click was in timeline content area
				b := cv.Bounds()
				navY := b.Min.Y + 12
				actionY := navY + 36
				contentTop := actionY + 36
				contentBottom := b.Max.Y - 50
				timelineGutterX := b.Min.X + 54
				totalHours := float32(dayEndHour - dayStartHour)
				hourHeight := (contentBottom - contentTop) / totalHours

				if ev.Position.X >= timelineGutterX && ev.Position.X <= b.Max.X-16 &&
					ev.Position.Y >= contentTop && ev.Position.Y <= contentBottom {
					hourFraction := (ev.Position.Y - contentTop) / hourHeight
					clickedHour := dayStartHour + int(hourFraction)
					if clickedHour >= dayStartHour && clickedHour < dayEndHour {
						start := time.Date(cv.currentDay.Year(), cv.currentDay.Month(), cv.currentDay.Day(), clickedHour, 0, 0, 0, time.Local)
						end := start.Add(1 * time.Hour)
						cv.openEditorWithTimes(start, end)
						return true
					}
				}
			}
		case *event.WheelEvent:
			// Scroll handling would go here, but list mode has wheel handling below
		}
	}

	// Handle list mode entry clicks and scrolling
	if cv.isListMode {
		switch ev := e.(type) {
		case *event.MouseEvent:
			if ev.MouseType == event.MousePress {
				// Hit-test in reverse order (most recently drawn first)
				for i := len(cv.listItemBounds) - 1; i >= 0; i-- {
					bounds := cv.listItemBounds[i]
					if bounds.bounds.Contains(ev.Position) {
						// Find the entry with this ID and open editor
						for j := range cv.entries {
							if cv.entries[j].ID == bounds.entryID {
								cv.openEditor(&cv.entries[j])
								return true
							}
						}
					}
				}
			}
		case *event.WheelEvent:
			// Adjust scroll offset based on wheel direction
			// Positive delta = scroll up (decrease offset), negative = scroll down (increase offset)
			if ev.Delta.Y > 0 {
				cv.listScrollOffset--
				if cv.listScrollOffset < 0 {
					cv.listScrollOffset = 0
				}
			} else if ev.Delta.Y < 0 {
				cv.listScrollOffset++
				// Ensure we don't scroll past available entries
				maxScroll := len(cv.entries) - 1
				if cv.listScrollOffset > maxScroll {
					cv.listScrollOffset = maxScroll
				}
			}
			if cv.onRequestRedraw != nil {
				cv.onRequestRedraw()
			}
			return true
		}
	}

	return false
}

func (cv *CalendarView) Children() []widget.Widget {
	children := []widget.Widget{
		cv.prevBtn,
		cv.todayBtn,
		cv.nextBtn,
		cv.addBtn,
		cv.listModeBtn,
	}
	if cv.editor != nil {
		children = append(children, cv.editor)
	}
	return children
}
