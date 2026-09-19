package ui

import (
	"fmt"
	"time"

	"yokto-time/pkg/db"
	"yokto-time/pkg/timer"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// CalendarView renders a visual daily timeline showing tasks as calendar blocks.
type CalendarView struct {
	widget.WidgetBase

	repo            db.Repository
	currentDay      time.Time
	entries         []db.TimeEntry
	selectedEntry   *db.TimeEntry
	onRequestRedraw func()

	prevBtn  *GlassButton
	todayBtn *GlassButton
	nextBtn  *GlassButton
}

const (
	dayStartHour = 7  // 07:00
	dayEndHour   = 21 // 21:00
)

func NewCalendarView(repo db.Repository, onRequestRedraw func()) *CalendarView {
	cv := &CalendarView{
		repo:            repo,
		currentDay:      time.Now(),
		onRequestRedraw: onRequestRedraw,
	}
	cv.SetVisible(true)
	cv.SetEnabled(true)

	cv.prevBtn = NewGlassButton("◀ Gestern", func() {
		cv.currentDay = cv.currentDay.AddDate(0, 0, -1)
		cv.Refresh()
	}).SetCompact(true)

	cv.todayBtn = NewGlassButton("Heute", func() {
		cv.currentDay = time.Now()
		cv.Refresh()
	}).SetCompact(true)

	cv.nextBtn = NewGlassButton("Morgen ▶", func() {
		cv.currentDay = cv.currentDay.AddDate(0, 0, 1)
		cv.Refresh()
	}).SetCompact(true)

	cv.Refresh()
	return cv
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
	theme := DefaultDarkTheme

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

	// Timeline Canvas area
	timelineTop := navY + 36
	timelineBottom := b.Max.Y - 50
	timelineHeight := timelineBottom - timelineTop
	timelineGutterX := b.Min.X + 54
	timelineWidth := b.Max.X - timelineGutterX - 16

	totalHours := float32(dayEndHour - dayStartHour)
	hourHeight := timelineHeight / totalHours

	// Draw Hour Grid lines & labels
	for h := dayStartHour; h <= dayEndHour; h++ {
		curY := timelineTop + float32(h-dayStartHour)*hourHeight
		timeLbl := fmt.Sprintf("%02d:00", h)
		canvas.DrawText(timeLbl, geometry.NewRect(b.Min.X+10, curY-6, 38, 14), 10, theme.TextMuted, false, widget.TextAlignRight)

		// Subtle divider line
		lineRect := geometry.NewRect(timelineGutterX, curY, timelineWidth, 1)
		canvas.DrawRect(lineRect, widget.RGBA8(40, 46, 60, 160))
	}

	// Draw Task Time Blocks
	for _, entry := range cv.entries {
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

		blockY := timelineTop + (startHour-float32(dayStartHour))*hourHeight
		blockH := durHours * hourHeight
		if blockY+blockH > timelineBottom {
			blockH = timelineBottom - blockY
		}
		if blockH < 20 {
			blockH = 20
		}

		blockRect := geometry.NewRect(timelineGutterX+4, blockY, timelineWidth-8, blockH)

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
			titleText = "⚡ " + entry.TaskName
		} else if entry.ProjectName != "" {
			titleText = fmt.Sprintf("[%s] %s", entry.ProjectName, entry.TaskName)
		}

		timeRange := fmt.Sprintf("%s (%s)", entry.StartedAt.Local().Format("15:04"), timer.FormatDurationHHMM(time.Duration(entry.DurationSec)*time.Second))
		canvas.DrawText(titleText, geometry.NewRect(blockRect.Min.X+8, blockRect.Min.Y+3, blockRect.Width()-14, 14), 10, theme.TextPrimary, true, widget.TextAlignLeft)
		if blockH >= 34 {
			canvas.DrawText(timeRange, geometry.NewRect(blockRect.Min.X+8, blockRect.Min.Y+18, blockRect.Width()-14, 12), 9, theme.TextSecondary, false, widget.TextAlignLeft)
		}
	}

	// Red Now-Line indicator if viewing today
	now := time.Now()
	if cv.currentDay.Format("2006-01-02") == now.Format("2006-01-02") {
		nowHour := float32(now.Hour()) + float32(now.Minute())/60.0
		if nowHour >= float32(dayStartHour) && nowHour <= float32(dayEndHour) {
			nowY := timelineTop + (nowHour-float32(dayStartHour))*hourHeight
			nowLine := geometry.NewRect(timelineGutterX, nowY, timelineWidth, 1.5)
			canvas.DrawRect(nowLine, widget.RGBA8(239, 68, 68, 255))

			// Red circle at gutter
			canvas.DrawCircle(geometry.Pt(timelineGutterX, nowY), 3.0, widget.RGBA8(239, 68, 68, 255))
		}
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

func (cv *CalendarView) Event(ctx widget.Context, e event.Event) bool {
	if cv.prevBtn.Event(ctx, e) {
		return true
	}
	if cv.todayBtn.Event(ctx, e) {
		return true
	}
	if cv.nextBtn.Event(ctx, e) {
		return true
	}
	return false
}

func (cv *CalendarView) Children() []widget.Widget {
	return nil
}
