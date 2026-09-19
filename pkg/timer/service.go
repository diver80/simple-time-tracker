package timer

import (
	"errors"
	"sync"
	"time"

	"time-tracker/pkg/db"
)

type TimerState int

const (
	StateIdle TimerState = iota
	StateRunning
	StateQuickShift
	StatePaused
)

func (s TimerState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateRunning:
		return "Running"
	case StateQuickShift:
		return "QuickShift"
	case StatePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// TickListener receives an independent snapshot, outside the service lock.
// Callbacks run on the ticker goroutine and must not mutate UI widgets directly.
type TickListener func(elapsed time.Duration, state TimerState, entry *db.TimeEntry)

type TimerService struct {
	repo db.Repository
	mu   sync.RWMutex

	state       TimerState
	activeEntry *db.TimeEntry
	parentEntry *db.TimeEntry
	listeners   []TickListener
	stopTicker  chan struct{}
	closed      bool
	restoreErr  error
	now         func() time.Time
}

func NewTimerService(repo db.Repository) *TimerService {
	s := &TimerService{
		repo: repo, state: StateIdle,
		stopTicker: make(chan struct{}), now: time.Now,
	}
	active, err := repo.GetActiveEntry()
	s.restoreErr = err
	if err == nil && active != nil {
		s.activeEntry = active
		s.state = StateRunning
		if active.IsQuickShift && active.ParentID != nil {
			s.state = StateQuickShift
			s.parentEntry, s.restoreErr = repo.GetEntry(*active.ParentID)
		} else if active.PausedAt != nil {
			s.state = StatePaused
		}
	}
	go s.runTicker()
	return s
}

func (s *TimerService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.stopTicker)
	}
}

func (s *TimerService) OnTick(fn TickListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn != nil && !s.closed {
		s.listeners = append(s.listeners, fn)
	}
}

func (s *TimerService) checkOpenLocked() error {
	if s.closed {
		return errors.New("timer service is closed")
	}
	return s.restoreErr
}

func (s *TimerService) Start(projectID *int64, taskName string, isBillable bool) (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return nil, err
	}
	if s.state != StateIdle {
		return nil, errors.New("a timer is already active")
	}
	entry, err := s.repo.StartTimeEntry(projectID, taskName, isBillable)
	if err != nil {
		return nil, err
	}
	s.activeEntry, s.parentEntry, s.state = entry, nil, StateRunning
	return cloneEntry(entry), nil
}

func (s *TimerService) StartQuickShift(taskName string) (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return nil, err
	}
	if s.state != StateRunning || s.activeEntry == nil {
		return nil, errors.New("cannot start QuickShift when main timer is not running")
	}
	entry, err := s.repo.StartQuickShift(s.activeEntry.ID, taskName)
	if err != nil {
		return nil, err
	}
	// The repository pauses the parent atomically with creating the child.
	parent := cloneEntry(s.activeEntry)
	pausedAt := entry.StartedAt
	parent.PausedAt = &pausedAt
	s.parentEntry, s.activeEntry, s.state = parent, entry, StateQuickShift
	return cloneEntry(entry), nil
}

func (s *TimerService) FinishQuickShift() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if s.state != StateQuickShift || s.activeEntry == nil || s.parentEntry == nil {
		return errors.New("no active QuickShift session")
	}
	stopped, err := s.stopEntryLocked(s.activeEntry, s.now().UTC())
	if err != nil {
		return err
	}
	s.activeEntry = stopped // Retain the result if reloading the parent fails.
	// Stopping the child also resumes its parent in the same DB transaction.
	parent, err := s.repo.GetEntry(s.parentEntry.ID)
	if err != nil {
		return err
	}
	s.activeEntry, s.parentEntry, s.state = parent, nil, StateRunning
	return nil
}

// stopEntryLocked is retryable even if a previous read after the save failed.
func (s *TimerService) stopEntryLocked(entry *db.TimeEntry, endedAt time.Time) (*db.TimeEntry, error) {
	if entry.EndedAt != nil {
		return entry, nil
	}
	if err := s.repo.StopTimeEntry(entry.ID, endedAt); err != nil {
		return nil, err
	}
	return s.repo.GetEntry(entry.ID)
}

func (s *TimerService) Stop() (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return nil, err
	}
	if s.activeEntry == nil {
		return nil, errors.New("no active timer to stop")
	}
	stopped, err := s.stopEntryLocked(s.activeEntry, s.now().UTC())
	if err != nil {
		return nil, err
	}
	s.activeEntry = stopped
	if s.state == StateQuickShift && s.parentEntry != nil {
		// Use the original child stop time on retry, not a later wall clock.
		if _, err := s.stopEntryLocked(s.parentEntry, *stopped.EndedAt); err != nil {
			return nil, err
		}
	}
	s.activeEntry, s.parentEntry, s.state = nil, nil, StateIdle
	return cloneEntry(stopped), nil
}

func (s *TimerService) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if s.state != StateRunning || s.activeEntry == nil {
		return errors.New("can only pause a running timer")
	}
	entry := cloneEntry(s.activeEntry)
	now := s.now().UTC()
	entry.PausedAt = &now
	if err := s.repo.UpdateEntry(entry); err != nil {
		return err
	}
	s.activeEntry, s.state = entry, StatePaused
	return nil
}

func (s *TimerService) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if s.state != StatePaused || s.activeEntry == nil || s.activeEntry.PausedAt == nil {
		return errors.New("can only resume a paused timer")
	}
	entry := cloneEntry(s.activeEntry)
	entry.PausedNS += int64(max(0, s.now().Sub(*entry.PausedAt)))
	entry.PausedAt = nil
	if err := s.repo.UpdateEntry(entry); err != nil {
		return err
	}
	s.activeEntry, s.state = entry, StateRunning
	return nil
}

func (s *TimerService) checkActiveLocked() error {
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if s.activeEntry == nil || s.activeEntry.EndedAt != nil {
		return errors.New("no active entry")
	}
	return nil
}

// Save synchronously so callers can observe errors and shutdown cannot lose edits.
func (s *TimerService) UpdateBookingText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkActiveLocked(); err != nil {
		return err
	}
	if err := s.repo.UpdateActiveBookingText(s.activeEntry.ID, text); err != nil {
		return err
	}
	s.activeEntry.BookingText = text
	return nil
}

func (s *TimerService) UpdateTaskName(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkActiveLocked(); err != nil {
		return err
	}
	if err := s.repo.UpdateActiveTaskName(s.activeEntry.ID, name); err != nil {
		return err
	}
	s.activeEntry.TaskName = name
	return nil
}

// SetProject assigns or clears the current project without changing timer state.
func (s *TimerService) SetProject(projectID *int64) error {
	if projectID != nil { return s.AssignProject(*projectID) }
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkActiveLocked(); err != nil { return err }
	updated := cloneEntry(s.activeEntry)
	updated.ProjectID = nil
	if err := s.repo.UpdateEntry(updated); err != nil { return err }
	s.activeEntry.ProjectID = nil
	s.activeEntry.ProjectName = ""
	s.activeEntry.CustomerName = ""
	s.activeEntry.ProjectColor = ""
	return nil
}

func (s *TimerService) AssignProject(projectID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkActiveLocked(); err != nil {
		return err
	}
	project, err := s.repo.GetProject(projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("project not found")
	}
	if err := s.repo.AssignProject(s.activeEntry.ID, projectID); err != nil {
		return err
	}
	s.activeEntry.ProjectID = &projectID
	s.activeEntry.ProjectName = project.Name
	s.activeEntry.CustomerName = project.CustomerName
	s.activeEntry.ProjectColor = project.Color
	return nil
}

func elapsedAt(entry *db.TimeEntry, now time.Time) time.Duration {
	if entry == nil {
		return 0
	}
	if entry.EndedAt != nil {
		return time.Duration(entry.DurationSec) * time.Second
	}
	if entry.PausedAt != nil && entry.PausedAt.Before(now) {
		now = *entry.PausedAt
	}
	return max(0, now.Sub(entry.StartedAt)-time.Duration(entry.PausedNS))
}

func cloneEntry(entry *db.TimeEntry) *db.TimeEntry {
	if entry == nil {
		return nil
	}
	copy := *entry
	if entry.ProjectID != nil {
		value := *entry.ProjectID
		copy.ProjectID = &value
	}
	if entry.ParentID != nil {
		value := *entry.ParentID
		copy.ParentID = &value
	}
	if entry.EndedAt != nil {
		value := *entry.EndedAt
		copy.EndedAt = &value
	}
	if entry.PausedAt != nil {
		value := *entry.PausedAt
		copy.PausedAt = &value
	}
	return &copy
}

func (s *TimerService) GetCurrentState() (TimerState, *db.TimeEntry, time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, cloneEntry(s.activeEntry), elapsedAt(s.activeEntry, s.now())
}

func (s *TimerService) runTicker() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopTicker:
			return
		case <-ticker.C:
			s.mu.RLock()
			if s.closed {
				s.mu.RUnlock()
				return
			}
			state, entry := s.state, cloneEntry(s.activeEntry)
			elapsed := elapsedAt(entry, s.now())
			listeners := append([]TickListener(nil), s.listeners...)
			s.mu.RUnlock()
			for _, fn := range listeners {
				fn(elapsed, state, cloneEntry(entry))
			}
		}
	}
}
