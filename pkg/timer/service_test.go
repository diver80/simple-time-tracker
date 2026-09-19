package timer

import (
	"path/filepath"
	"testing"
	"time"

	"time-tracker/pkg/db"
)

func TestTimerServiceWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	svc := NewTimerService(repo)
	defer svc.Close()

	// 1. Initial state should be idle
	state, entry, _ := svc.GetCurrentState()
	if state != StateIdle || entry != nil {
		t.Fatalf("expected initial state Idle, got %v", state)
	}

	// 2. Start timer
	started, err := svc.Start(nil, "Develop Feature X", true)
	if err != nil {
		t.Fatalf("failed to start timer: %v", err)
	}
	if started.TaskName != "Develop Feature X" {
		t.Errorf("expected task name Develop Feature X, got %s", started.TaskName)
	}

	state, entry, _ = svc.GetCurrentState()
	if state != StateRunning || entry.ID != started.ID {
		t.Fatalf("expected state Running with entry %d, got %v", started.ID, state)
	}

	// 3. Update live booking text
	err = svc.UpdateBookingText("Added initial unit tests")
	if err != nil {
		t.Fatalf("failed to update booking text: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // wait for debounce

	dbEntry, err := repo.GetEntry(started.ID)
	if err != nil || dbEntry.BookingText != "Added initial unit tests" {
		t.Fatalf("booking text was not persisted to DB, got: %s (err: %v)", dbEntry.BookingText, err)
	}

	// 4. Trigger QuickShift
	qs, err := svc.StartQuickShift("Urgent Support Call")
	if err != nil {
		t.Fatalf("failed to start quickshift: %v", err)
	}
	if !qs.IsQuickShift {
		t.Errorf("expected quickshift flag true")
	}

	state, entry, _ = svc.GetCurrentState()
	if state != StateQuickShift || entry.ID != qs.ID {
		t.Fatalf("expected state QuickShift with entry %d, got %v", qs.ID, state)
	}

	// 5. Finish QuickShift -> should resume main task
	err = svc.FinishQuickShift()
	if err != nil {
		t.Fatalf("failed to finish quickshift: %v", err)
	}

	state, entry, _ = svc.GetCurrentState()
	if state != StateRunning || entry.ID != started.ID {
		t.Fatalf("expected state Running with original entry %d, got %v", started.ID, state)
	}

	// 6. Stop timer
	stopped, err := svc.Stop()
	if err != nil {
		t.Fatalf("failed to stop timer: %v", err)
	}
	if stopped.ID != started.ID {
		t.Errorf("expected stopped entry %d, got %d", started.ID, stopped.ID)
	}

	state, entry, _ = svc.GetCurrentState()
	if state != StateIdle || entry != nil {
		t.Fatalf("expected state Idle after stop, got %v", state)
	}
}
