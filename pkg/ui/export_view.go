package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"yokto-time/pkg/db"
	"yokto-time/pkg/export"
	"yokto-time/pkg/timer"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// ExportView handles monthly CSV generation and budget overview.
type ExportView struct {
	widget.WidgetBase

	repo            db.Repository
	selectedYear    int
	selectedMonth   time.Month
	roundingMin     int
	roundMode       timer.RoundMode
	summary         *export.MonthlySummary
	statusMessage   string
	onRequestRedraw func()

	prevMonthBtn *GlassButton
	nextMonthBtn *GlassButton
	round15Btn   *GlassButton
	roundNoneBtn *GlassButton
	exportBtn    *GlassButton
	copyBtn      *GlassButton
}

func NewExportView(repo db.Repository, onRequestRedraw func()) *ExportView {
	now := time.Now()
	ev := &ExportView{
		repo:            repo,
		selectedYear:    now.Year(),
		selectedMonth:   now.Month(),
		roundingMin:     15,
		roundMode:       timer.RoundCeil,
		onRequestRedraw: onRequestRedraw,
	}
	ev.SetVisible(true)
	ev.SetEnabled(true)

	ev.prevMonthBtn = NewGlassButton("◀", func() {
		t := time.Date(ev.selectedYear, ev.selectedMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		ev.selectedYear = t.Year()
		ev.selectedMonth = t.Month()
		ev.Refresh()
	}).SetCompact(true)

	ev.nextMonthBtn = NewGlassButton("▶", func() {
		t := time.Date(ev.selectedYear, ev.selectedMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		ev.selectedYear = t.Year()
		ev.selectedMonth = t.Month()
		ev.Refresh()
	}).SetCompact(true)

	ev.round15Btn = NewGlassButton("15 Min. Aufrunden", func() {
		ev.roundingMin = 15
		ev.roundMode = timer.RoundCeil
		ev.Refresh()
	}).SetCompact(true)

	ev.roundNoneBtn = NewGlassButton("Exakte Zeiten", func() {
		ev.roundingMin = 0
		ev.roundMode = timer.RoundNone
		ev.Refresh()
	}).SetCompact(true)

	ev.exportBtn = NewGlassButton("📥 Als CSV exportieren", func() {
		ev.doExportCSV()
	})
	ev.exportBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), DefaultDarkTheme.AccentPrimary, DefaultDarkTheme.AccentHover)

	ev.copyBtn = NewGlassButton("📋 In Zwischenablage kopieren", func() {
		ev.doCopyClipboard()
	})

	ev.Refresh()
	return ev
}

func (ev *ExportView) Refresh() {
	opts := export.ExportOptions{
		Year:        ev.selectedYear,
		Month:       ev.selectedMonth,
		RoundingMin: ev.roundingMin,
		RoundMode:   ev.roundMode,
	}
	_, summary, err := export.GenerateMonthlyCSV(ev.repo, opts)
	if err == nil {
		ev.summary = summary
	}
	if ev.onRequestRedraw != nil {
		ev.onRequestRedraw()
	}
}

func (ev *ExportView) doExportCSV() {
	home, _ := os.UserHomeDir()
	desktop := filepath.Join(home, "Desktop")
	targetFile := filepath.Join(desktop, fmt.Sprintf("yokto-export-%04d-%02d.csv", ev.selectedYear, int(ev.selectedMonth)))

	opts := export.ExportOptions{
		Year:        ev.selectedYear,
		Month:       ev.selectedMonth,
		RoundingMin: ev.roundingMin,
		RoundMode:   ev.roundMode,
	}

	_, err := export.ExportMonthlyCSVToFile(ev.repo, opts, targetFile)
	if err != nil {
		ev.statusMessage = fmt.Sprintf("Fehler: %v", err)
	} else {
		ev.statusMessage = fmt.Sprintf("✓ Gespeichert auf dem Schreibtisch:\n%s", filepath.Base(targetFile))
	}
	if ev.onRequestRedraw != nil {
		ev.onRequestRedraw()
	}
}

func (ev *ExportView) doCopyClipboard() {
	opts := export.ExportOptions{
		Year:        ev.selectedYear,
		Month:       ev.selectedMonth,
		RoundingMin: ev.roundingMin,
		RoundMode:   ev.roundMode,
	}
	csvContent, _, err := export.GenerateMonthlyCSV(ev.repo, opts)
	if err != nil {
		ev.statusMessage = fmt.Sprintf("Fehler: %v", err)
	} else {
		// Copy to macOS clipboard using pbcopy
		cmd := exec.Command("pbcopy")
		cmd.Stdin = os.NewFile(0, "stdin")
		pipe, _ := cmd.StdinPipe()
		if err := cmd.Start(); err == nil {
			_, _ = pipe.Write([]byte(csvContent))
			_ = pipe.Close()
			_ = cmd.Wait()
			ev.statusMessage = "✓ CSV erfolgreich in die Zwischenablage kopiert!"
		} else {
			ev.statusMessage = "✓ Generiert (pbcopy nicht verfügbar)"
		}
	}
	if ev.onRequestRedraw != nil {
		ev.onRequestRedraw()
	}
}

func (ev *ExportView) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(420)
	h := c.ConstrainHeight(560)
	return geometry.Sz(w, h)
}

func (ev *ExportView) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := ev.Bounds()
	theme := DefaultDarkTheme

	// Card Container
	canvas.DrawRoundRect(b, theme.CardBg, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	// Month Selector Bar
	topY := b.Min.Y + 16
	ev.prevMonthBtn.SetBounds(geometry.NewRect(b.Min.X+20, topY, 32, 26))
	ev.prevMonthBtn.Draw(ctx, canvas)

	monthTitle := fmt.Sprintf("%s %d", ev.selectedMonth.String(), ev.selectedYear)
	canvas.DrawText(monthTitle, geometry.NewRect(b.Min.X+60, topY+4, b.Width()-120, 18), 14, theme.TextPrimary, true, widget.TextAlignCenter)

	ev.nextMonthBtn.SetBounds(geometry.NewRect(b.Max.X-52, topY, 32, 26))
	ev.nextMonthBtn.Draw(ctx, canvas)

	// Rounding Rule Toggles
	roundY := topY + 40
	canvas.DrawText("Rundungsregel für Abrechnung:", geometry.NewRect(b.Min.X+20, roundY, b.Width()-40, 14), 11, theme.TextSecondary, false, widget.TextAlignLeft)

	rBtnY := roundY + 18
	ev.round15Btn.SetBounds(geometry.NewRect(b.Min.X+20, rBtnY, 130, 24))
	if ev.roundingMin == 15 {
		ev.round15Btn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.AccentPrimary, theme.AccentHover)
	} else {
		ev.round15Btn.SetCustomColors(theme.TextSecondary, widget.RGBA8(40, 46, 60, 255), theme.InputBorder)
	}
	ev.round15Btn.Draw(ctx, canvas)

	ev.roundNoneBtn.SetBounds(geometry.NewRect(b.Min.X+160, rBtnY, 100, 24))
	if ev.roundingMin == 0 {
		ev.roundNoneBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.AccentPrimary, theme.AccentHover)
	} else {
		ev.roundNoneBtn.SetCustomColors(theme.TextSecondary, widget.RGBA8(40, 46, 60, 255), theme.InputBorder)
	}
	ev.roundNoneBtn.Draw(ctx, canvas)

	// Metrics Summary Card
	statsY := rBtnY + 40
	statsBox := geometry.NewRect(b.Min.X+20, statsY, b.Width()-40, 150)
	canvas.DrawRoundRect(statsBox, theme.InputBg, 8)
	canvas.StrokeRoundRect(statsBox, theme.InputBorder, 8, 1.0)

	totalHours := 0.0
	billableHours := 0.0
	revenue := 0.0
	entryCount := 0
	if ev.summary != nil {
		totalHours = ev.summary.TotalHours
		billableHours = ev.summary.BillableHours
		revenue = ev.summary.TotalRevenue
		entryCount = ev.summary.TotalEntries
	}

	row1Y := statsBox.Min.X + 16
	_ = row1Y
	canvas.DrawText("Monatsübersicht", geometry.NewRect(statsBox.Min.X+16, statsBox.Min.Y+12, statsBox.Width()-32, 16), 12, theme.TextSecondary, true, widget.TextAlignLeft)

	// Metric items
	m1Y := statsBox.Min.Y + 36
	canvas.DrawText("Gesamtstunden:", geometry.NewRect(statsBox.Min.X+16, m1Y, 150, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)
	canvas.DrawText(fmt.Sprintf("%.2f Std. (%d Einträge)", totalHours, entryCount), geometry.NewRect(statsBox.Max.X-180, m1Y, 164, 16), 11, theme.TextPrimary, true, widget.TextAlignRight)

	m2Y := m1Y + 26
	canvas.DrawText("Abrechenbare Zeit:", geometry.NewRect(statsBox.Min.X+16, m2Y, 150, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)
	canvas.DrawText(fmt.Sprintf("%.2f Std.", billableHours), geometry.NewRect(statsBox.Max.X-180, m2Y, 164, 16), 11, theme.AccentPrimary, true, widget.TextAlignRight)

	m3Y := m2Y + 26
	canvas.DrawText("Abrechenbarer Umsatz:", geometry.NewRect(statsBox.Min.X+16, m3Y, 150, 16), 11, theme.TextSecondary, false, widget.TextAlignLeft)
	canvas.DrawText(fmt.Sprintf("%.2f €", revenue), geometry.NewRect(statsBox.Max.X-180, m3Y, 164, 16), 12, theme.TextPrimary, true, widget.TextAlignRight)

	// Action Buttons
	actionY := statsBox.Max.Y + 24
	ev.exportBtn.SetBounds(geometry.NewRect(b.Min.X+20, actionY, b.Width()-40, 36))
	ev.exportBtn.Draw(ctx, canvas)

	ev.copyBtn.SetBounds(geometry.NewRect(b.Min.X+20, actionY+46, b.Width()-40, 32))
	ev.copyBtn.Draw(ctx, canvas)

	// Status Message
	if ev.statusMessage != "" {
		statusY := actionY + 90
		canvas.DrawText(ev.statusMessage, geometry.NewRect(b.Min.X+20, statusY, b.Width()-40, 36), 11, theme.AccentPrimary, false, widget.TextAlignCenter)
	}
}

func (ev *ExportView) Event(ctx widget.Context, e event.Event) bool {
	if ev.prevMonthBtn.Event(ctx, e) {
		return true
	}
	if ev.nextMonthBtn.Event(ctx, e) {
		return true
	}
	if ev.round15Btn.Event(ctx, e) {
		return true
	}
	if ev.roundNoneBtn.Event(ctx, e) {
		return true
	}
	if ev.exportBtn.Event(ctx, e) {
		return true
	}
	if ev.copyBtn.Event(ctx, e) {
		return true
	}
	return false
}

func (ev *ExportView) Children() []widget.Widget {
	return nil
}
