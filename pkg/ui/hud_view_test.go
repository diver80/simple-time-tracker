package ui

import (
	"testing"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
)

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
