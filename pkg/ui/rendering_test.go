package ui

import (
	"strings"
	"testing"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"

	"github.com/gogpu/gg"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

type textRecordingCanvas struct {
	widget.Canvas
	texts []string
}

func (c *textRecordingCanvas) DrawText(text string, bounds geometry.Rect, size float32, color widget.Color, bold bool, align widget.TextAlign) {
	c.texts = append(c.texts, text)
	c.Canvas.DrawText(text, bounds, size, color, bold, align)
}

func TestTrackerRendersControlsAndRepaintsAfterStart(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()
	view := NewAppView(repo, svc, nil, func() {})
	view.SetRepaintBoundary(true)
	view.SetBounds(geometry.NewRect(0, 0, 420, 580))
	ctx := widget.NewContext()
	canvas := &textRecordingCanvas{Canvas: render.NewCanvas(gg.NewContext(420, 580), 420, 580)}
	view.Draw(ctx, canvas)
	text := strings.Join(canvas.texts, "\n")
	for _, want := range []string{"Tracker", "00:00:00", "Aufgabe:", "Start Timer", "Live-Buchungstext"} {
		if !strings.Contains(text, want) {
			t.Errorf("initial tracker missing %q", want)
		}
	}
	bounds := view.hudView.startBtn.Bounds()
	if bounds.IsEmpty() || bounds.Max.Y > view.Bounds().Max.Y {
		t.Fatalf("start control is empty or clipped: %v", bounds)
	}
	view.ClearRedraw()
	view.ClearSceneDirty()
	click := &event.MouseEvent{
		MouseType: event.MousePress,
		Position:  geometry.Pt(bounds.Min.X+5, bounds.Min.Y+5),
	}
	if !view.Event(ctx, click) {
		t.Fatal("start click was not handled")
	}
	if state, _, _ := svc.GetCurrentState(); state != timer.StateRunning {
		t.Fatalf("timer did not start: %v", state)
	}
	if !view.IsSceneDirty() {
		t.Fatal("handled click did not invalidate the retained root scene")
	}
	canvas.texts = nil
	view.Draw(ctx, canvas)
	text = strings.Join(canvas.texts, "\n")
	if !strings.Contains(text, "Stopp") || !strings.Contains(text, "QuickShift") {
		t.Fatal("running tracker did not render stop and interruption controls")
	}
}
