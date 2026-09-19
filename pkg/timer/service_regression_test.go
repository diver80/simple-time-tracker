package timer

import (
	"errors"
	"sync"
	"testing"
	"time"

	"time-tracker/pkg/db"
)

func timerFixture(t *testing.T) (*db.SQLiteRepository, *TimerService, *db.TimeEntry) {
	t.Helper()
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	entry, err := repo.StartTimeEntry(nil, "Main task", true)
	if err != nil {
		t.Fatal(err)
	}
	entry.StartedAt = time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	if err := repo.UpdateEntry(entry); err != nil {
		t.Fatal(err)
	}
	svc := NewTimerService(repo)
	t.Cleanup(svc.Close)
	setClock(svc, entry.StartedAt)
	return repo, svc, entry
}

func setClock(svc *TimerService, now time.Time) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.now = func() time.Time { return now }
}

func TestPauseAccountingSurvivesRestart(t *testing.T) {
	repo, svc, entry := timerFixture(t)
	setClock(svc, entry.StartedAt.Add(10*time.Minute))
	if err := svc.Pause(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(nil, "Must not orphan paused entry", true); err == nil {
		t.Fatal("start while paused must fail")
	}
	setClock(svc, entry.StartedAt.Add(30*time.Minute))
	_, _, elapsed := svc.GetCurrentState()
	if elapsed != 10*time.Minute {
		t.Fatalf("paused elapsed = %v", elapsed)
	}
	svc.Close()
	restored := NewTimerService(repo)
	defer restored.Close()
	setClock(restored, entry.StartedAt.Add(30*time.Minute))
	state, _, elapsed := restored.GetCurrentState()
	if state != StatePaused || elapsed != 10*time.Minute {
		t.Fatalf("restored state=%v elapsed=%v", state, elapsed)
	}
	if err := restored.Resume(); err != nil {
		t.Fatal(err)
	}
	setClock(restored, entry.StartedAt.Add(40*time.Minute))
	stopped, err := restored.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil || stopped.DurationSec != 1200 || !stopped.StartedAt.Equal(entry.StartedAt) {
		t.Fatalf("unexpected stopped entry: %+v", stopped)
	}
	stored, err := repo.GetEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DurationSec != stopped.DurationSec {
		t.Fatal("returned and stored duration differ")
	}
}

func TestStopWhilePausedExcludesCurrentPause(t *testing.T) {
	_, svc, entry := timerFixture(t)
	setClock(svc, entry.StartedAt.Add(5*time.Minute))
	if err := svc.Pause(); err != nil {
		t.Fatal(err)
	}
	setClock(svc, entry.StartedAt.Add(time.Hour))
	stopped, err := svc.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if stopped.DurationSec != 300 {
		t.Fatalf("duration = %d, want 300", stopped.DurationSec)
	}
}

func TestQuickShiftAccountingSurvivesRestart(t *testing.T) {
	for _, stopAll := range []bool{false, true} {
		t.Run(map[bool]string{false: "finish", true: "stop all"}[stopAll], func(t *testing.T) {
			repo, svc, parent := timerFixture(t)
			child, err := svc.StartQuickShift("Interruption")
			if err != nil {
				t.Fatal(err)
			}
			// Set a deterministic timeline before restoring the service.
			svc.Close()
			parent, err = repo.GetEntry(parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			parent.StartedAt = child.StartedAt.Add(-10 * time.Minute)
			parent.PausedNS = int64(2 * time.Minute)
			if err := repo.UpdateEntry(parent); err != nil {
				t.Fatal(err)
			}
			restored := NewTimerService(repo)
			defer restored.Close()
			setClock(restored, child.StartedAt.Add(5*time.Minute))
			state, snapshot, elapsed := restored.GetCurrentState()
			if state != StateQuickShift || snapshot.ID != child.ID || elapsed != 5*time.Minute {
				t.Fatalf("restored state=%v entry=%+v elapsed=%v", state, snapshot, elapsed)
			}
			if stopAll {
				if _, err := restored.Stop(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := restored.FinishQuickShift(); err != nil {
					t.Fatal(err)
				}
				state, snapshot, elapsed = restored.GetCurrentState()
				if state != StateRunning || snapshot.ID != parent.ID || elapsed != 8*time.Minute {
					t.Fatalf("resumed state=%v elapsed=%v", state, elapsed)
				}
				setClock(restored, child.StartedAt.Add(7*time.Minute))
				if _, err := restored.Stop(); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := repo.GetEntry(parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := int64(600)
			if stopAll {
				want = 480
			}
			if stored.DurationSec != want {
				t.Fatalf("parent duration=%d, want %d", stored.DurationSec, want)
			}
			stored, err = repo.GetEntry(child.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.DurationSec != 300 {
				t.Fatalf("child duration=%d", stored.DurationSec)
			}
		})
	}
}

func TestBookingTextPersistsBeforeClose(t *testing.T) {
	repo, svc, entry := timerFixture(t)
	if err := svc.UpdateBookingText("Latest notes"); err != nil {
		t.Fatal(err)
	}
	svc.Close()
	svc.Close()
	stored, err := repo.GetEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BookingText != "Latest notes" {
		t.Fatal("latest notes lost")
	}
	if err := svc.UpdateBookingText("after close"); err == nil {
		t.Fatal("update after close succeeded")
	}
}

var errSave = errors.New("simulated persistence failure")

type failingRepository struct {
	db.Repository
	failUpdate bool
	failStopID int64
	failGetID  int64
}

func (r *failingRepository) UpdateEntry(e *db.TimeEntry) error {
	if r.failUpdate {
		return errSave
	}
	return r.Repository.UpdateEntry(e)
}
func (r *failingRepository) UpdateActiveBookingText(id int64, text string) error {
	if r.failUpdate {
		return errSave
	}
	return r.Repository.UpdateActiveBookingText(id, text)
}
func (r *failingRepository) UpdateActiveTaskName(id int64, name string) error {
	if r.failUpdate {
		return errSave
	}
	return r.Repository.UpdateActiveTaskName(id, name)
}
func (r *failingRepository) StopTimeEntry(id int64, end time.Time) error {
	if id == r.failStopID {
		return errSave
	}
	return r.Repository.StopTimeEntry(id, end)
}

func (r *failingRepository) GetEntry(id int64) (*db.TimeEntry, error) {
	if id == r.failGetID {
		return nil, errSave
	}
	return r.Repository.GetEntry(id)
}

func TestStopRetriesAfterSuccessfulWriteAndFailedRead(t *testing.T) {
	repo, original, entry := timerFixture(t)
	original.Close()
	faults := &failingRepository{Repository: repo, failGetID: entry.ID}
	svc := NewTimerService(faults)
	defer svc.Close()
	end := entry.StartedAt.Add(10 * time.Minute)
	setClock(svc, end)
	if _, err := svc.Stop(); !errors.Is(err, errSave) {
		t.Fatalf("expected read failure, got %v", err)
	}
	faults.failGetID = 0
	setClock(svc, end.Add(time.Hour))
	stopped, err := svc.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if stopped.DurationSec != 600 || !stopped.EndedAt.Equal(end) {
		t.Fatalf("retry changed saved time: %+v", stopped)
	}
}

func TestFailedSavesPreserveTimerState(t *testing.T) {
	repo, original, entry := timerFixture(t)
	original.Close()
	faults := &failingRepository{Repository: repo, failUpdate: true}
	svc := NewTimerService(faults)
	defer svc.Close()
	setClock(svc, entry.StartedAt.Add(time.Minute))
	for name, operation := range map[string]func() error{
		"pause":   svc.Pause,
		"booking": func() error { return svc.UpdateBookingText("unsaved") },
		"task":    func() error { return svc.UpdateTaskName("unsaved") },
	} {
		if err := operation(); !errors.Is(err, errSave) {
			t.Errorf("%s: %v", name, err)
		}
	}
	state, snapshot, _ := svc.GetCurrentState()
	if state != StateRunning || snapshot.PausedAt != nil || snapshot.TaskName != entry.TaskName || snapshot.BookingText != "" {
		t.Fatalf("failed save changed state: %v %+v", state, snapshot)
	}
	faults.failUpdate = false
	if err := svc.Pause(); err != nil {
		t.Fatal(err)
	}
	faults.failUpdate = true
	if err := svc.Resume(); !errors.Is(err, errSave) {
		t.Fatal(err)
	}
	state, snapshot, _ = svc.GetCurrentState()
	if state != StatePaused || snapshot.PausedAt == nil {
		t.Fatal("failed resume changed state")
	}
}

func TestQuickShiftStopFailureCanBeRetried(t *testing.T) {
	repo, original, parent := timerFixture(t)
	original.Close()
	faults := &failingRepository{Repository: repo}
	svc := NewTimerService(faults)
	defer svc.Close()
	child, err := svc.StartQuickShift("Call")
	if err != nil {
		t.Fatal(err)
	}
	end := child.StartedAt.Add(5 * time.Minute)
	setClock(svc, end)
	faults.failStopID = child.ID
	if _, err := svc.Stop(); !errors.Is(err, errSave) {
		t.Fatal("child stop error not propagated")
	}
	state, _, _ := svc.GetCurrentState()
	if state != StateQuickShift {
		t.Fatal("failed stop reset timer")
	}
	faults.failStopID = parent.ID
	if _, err := svc.Stop(); !errors.Is(err, errSave) {
		t.Fatal("parent stop error not propagated")
	}
	setClock(svc, end.Add(time.Hour))
	faults.failStopID = 0
	stopped, err := svc.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if stopped.DurationSec != 300 {
		t.Fatalf("retry inflated child: %+v", stopped)
	}
	stored, err := repo.GetEntry(parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.EndedAt == nil || !stored.EndedAt.Equal(end) {
		t.Fatal("retry moved stop time")
	}
}

func TestSnapshotsDoNotAliasServiceState(t *testing.T) {
	repo, svc, _ := timerFixture(t)
	customer, err := repo.CreateCustomer("Customer")
	if err != nil {
		t.Fatal(err)
	}
	project, err := repo.CreateProject(customer.ID, "Project", 100, 0, 0, "#FFF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AssignProject(project.ID); err != nil {
		t.Fatal(err)
	}
	_, snapshot, _ := svc.GetCurrentState()
	snapshot.TaskName = "Mutated snapshot"
	*snapshot.ProjectID = -1
	_, current, _ := svc.GetCurrentState()
	if current.TaskName != "Main task" || *current.ProjectID != project.ID {
		t.Fatal("snapshot aliases service")
	}

	// Exercise reads and snapshot mutation concurrently with service writes.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, e, _ := svc.GetCurrentState()
			e.BookingText = "snapshot-only"
			*e.ProjectID = -1
		}
	}()
	for i := 0; i < 50; i++ {
		if err := svc.UpdateBookingText("persisted"); err != nil {
			t.Error(err)
		}
	}
	wg.Wait()
	_, current, _ = svc.GetCurrentState()
	if current.BookingText != "persisted" || *current.ProjectID != project.ID {
		t.Fatal("snapshot mutation escaped")
	}
}

func TestStartReturnsIndependentSnapshot(t *testing.T) {
	_, svc, _ := timerFixture(t)
	if _, err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	entry, err := svc.Start(nil, "New task", false)
	if err != nil {
		t.Fatal(err)
	}
	entry.TaskName = "not saved"
	_, current, _ := svc.GetCurrentState()
	if current.TaskName != "New task" {
		t.Fatal("Start exposed internal entry")
	}
}
