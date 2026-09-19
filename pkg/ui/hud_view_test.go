package ui

import (
	"testing"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/uitest"
	"github.com/gogpu/ui/widget"
)

type testCanvas struct {
	uitest.MockCanvas
}

func TestHUDViewInputEventDelegation(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()
	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	hud := NewHUDView(timerSvc, repo, nil)
	hud.SetBounds(geometry.NewRect(0, 0, 400, 520))

	// Layout and draw to establish bounds
	mockCtx := &testWidgetContext{}
	hud.Draw(mockCtx, &testCanvas{})

	// Click on task input
	taskBounds := hud.taskInput.Bounds()
	clickPt := geometry.Pt(taskBounds.Min.X+10, taskBounds.Min.Y+10)
	pressEv := event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, clickPt, clickPt, event.ModNone)

	handled := hud.Event(mockCtx, pressEv)
	if !handled || !hud.taskInput.IsFocused() {
		t.Fatalf("expected clicking task input to focus it, handled=%v, focused=%v", handled, hud.taskInput.IsFocused())
	}
}

func TestTimerTickOnlyRequestsRedraw(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()
	view := NewAppView(repo, svc, nil, func() {})
	if _, err := svc.Start(nil, "Test task", true); err != nil {
		t.Fatal(err)
	}

	// This listener runs after both UI listeners. The channel synchronizes
	// with the ticker without mutating or reading widgets from that goroutine.
	tick := make(chan struct{}, 1)
	svc.OnTick(func(time.Duration, timer.TimerState, *db.TimeEntry) {
		select {
		case tick <- struct{}{}:
		default:
		}
	})
	select {
	case <-tick:
	case <-time.After(5 * time.Second):
		t.Fatal("timer did not tick")
	}
	if view.timerState != timer.StateIdle || view.hudView.state != timer.StateIdle {
		t.Fatal("background tick mutated UI state")
	}
	view.refreshTimerState()
	view.hudView.refreshTimerSnapshot()
	if view.timerState != timer.StateRunning || view.hudView.state != timer.StateRunning {
		t.Fatal("UI-thread refresh did not pick up timer state")
	}
	if view.activeEntry.TaskName != "Test task" || view.hudView.activeEntry.TaskName != "Test task" {
		t.Fatal("UI received incorrect timer snapshot")
	}
}

func TestHUDViewInputStatePreservation(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()
	hud := NewHUDView(svc, repo, func() {})
	if _, err := svc.Start(nil, "Test task", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateBookingText("Saved notes"); err != nil {
		t.Fatal(err)
	}
	hud.refreshState()
	if hud.inputBookingText != "Saved notes" {
		t.Fatal("notes not loaded")
	}
	hud.inputBookingText = "Edit in progress"
	hud.refreshTimerSnapshot()
	if hud.inputBookingText != "Edit in progress" {
		t.Fatal("timer refresh overwrote input")
	}
}

func TestHUDViewRefreshStateFocusedGuard(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()

	hud := NewHUDView(svc, repo, func() {})
	if _, err := svc.Start(nil, "Saved task", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateBookingText("Saved notes"); err != nil {
		t.Fatal(err)
	}

	// Initial refresh loads saved task and notes
	hud.refreshState()
	if hud.taskInput.Text() != "Saved task" || hud.notesInput.Text() != "Saved notes" {
		t.Fatalf("expected initial text to be loaded, got task=%q, notes=%q", hud.taskInput.Text(), hud.notesInput.Text())
	}

	// Focus both inputs and type draft content
	hud.taskInput.SetFocused(true)
	hud.taskInput.SetText("Draft task")
	hud.inputTaskName = "Draft task"

	hud.notesInput.SetFocused(true)
	hud.notesInput.SetText("Draft notes")
	hud.inputBookingText = "Draft notes"

	// Calling refreshState while inputs are focused must NOT overwrite them
	hud.refreshState()

	if hud.taskInput.Text() != "Draft task" || hud.inputTaskName != "Draft task" {
		t.Errorf("focused taskInput was overwritten: got text=%q, inputTaskName=%q", hud.taskInput.Text(), hud.inputTaskName)
	}
	if hud.notesInput.Text() != "Draft notes" || hud.inputBookingText != "Draft notes" {
		t.Errorf("focused notesInput was overwritten: got text=%q, inputBookingText=%q", hud.notesInput.Text(), hud.inputBookingText)
	}

	// Now blur taskInput; refreshState should now update it from active entry
	hud.taskInput.SetFocused(false)
	hud.refreshState()
	if hud.taskInput.Text() != "Saved task" || hud.inputTaskName != "Saved task" {
		t.Errorf("blurred taskInput was not updated: got text=%q, inputTaskName=%q", hud.taskInput.Text(), hud.inputTaskName)
	}
}

func TestHUDViewChildren(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()
	svc := timer.NewTimerService(repo)
	defer svc.Close()

	hud := NewHUDView(svc, repo, nil)
	children := hud.Children()
	if len(children) != 9 {
		t.Fatalf("expected 9 children in HUDView, got %d", len(children))
	}
	expected := []widget.Widget{
		hud.taskInput,
		hud.notesInput,
		hud.projectInput,
		hud.startBtn,
		hud.stopBtn,
		hud.quickShiftBtn,
		hud.finishQSBtn,
		hud.pauseBtn,
		hud.resumeBtn,
	}
	for i, exp := range expected {
		if children[i] != exp {
			t.Errorf("child[%d] = %v, want %v", i, children[i], exp)
		}
	}
}

