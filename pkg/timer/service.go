package timer

import (
	"errors"
	"sync"
	"time"

	"yokto-time/pkg/db"
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

type TickListener func(elapsed time.Duration, state TimerState, entry *db.TimeEntry)

type TimerService struct {
	repo db.Repository
	mu   sync.RWMutex

	state        TimerState
	activeEntry  *db.TimeEntry
	parentEntry  *db.TimeEntry // Held in background when in QuickShift
	pausedAt     time.Time
	totalPaused  time.Duration
	listeners    []TickListener
	stopTicker   chan struct{}
	debounceText string
	saveDebounce *time.Timer
}

func NewTimerService(repo db.Repository) *TimerService {
	s := &TimerService{
		repo:       repo,
		state:      StateIdle,
		stopTicker: make(chan struct{}),
	}

	// Restore active entry from DB on startup if any
	active, err := repo.GetActiveEntry()
	if err == nil && active != nil {
		s.activeEntry = active
		if active.IsQuickShift && active.ParentID != nil {
			s.state = StateQuickShift
			parent, _ := repo.GetEntry(*active.ParentID)
			s.parentEntry = parent
		} else {
			s.state = StateRunning
		}
	}

	go s.runTicker()
	return s
}

func (s *TimerService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stopTicker:
	default:
		close(s.stopTicker)
	}
}

func (s *TimerService) OnTick(fn TickListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *TimerService) Start(projectID *int64, taskName string, isBillable bool) (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateRunning || s.state == StateQuickShift {
		return nil, errors.New("a timer is already running")
	}

	entry, err := s.repo.StartTimeEntry(projectID, taskName, isBillable)
	if err != nil {
		return nil, err
	}

	s.activeEntry = entry
	s.parentEntry = nil
	s.state = StateRunning
	s.totalPaused = 0
	return entry, nil
}

func (s *TimerService) StartQuickShift(taskName string) (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning || s.activeEntry == nil {
		return nil, errors.New("cannot start QuickShift when main timer is not running")
	}

	parent := s.activeEntry
	qsEntry, err := s.repo.StartQuickShift(parent.ID, taskName)
	if err != nil {
		return nil, err
	}

	s.parentEntry = parent
	s.activeEntry = qsEntry
	s.state = StateQuickShift
	return qsEntry, nil
}

func (s *TimerService) FinishQuickShift() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateQuickShift || s.activeEntry == nil || s.parentEntry == nil {
		return errors.New("no active QuickShift session")
	}

	now := time.Now().UTC()
	if err := s.repo.StopTimeEntry(s.activeEntry.ID, now); err != nil {
		return err
	}

	// Resume parent
	s.activeEntry = s.parentEntry
	s.parentEntry = nil
	s.state = StateRunning
	return nil
}

func (s *TimerService) Stop() (*db.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeEntry == nil {
		return nil, errors.New("no active timer to stop")
	}

	now := time.Now().UTC()
	// If in QuickShift, stop active QuickShift first
	if s.state == StateQuickShift {
		_ = s.repo.StopTimeEntry(s.activeEntry.ID, now)
		if s.parentEntry != nil {
			_ = s.repo.StopTimeEntry(s.parentEntry.ID, now)
		}
	} else {
		if err := s.repo.StopTimeEntry(s.activeEntry.ID, now); err != nil {
			return nil, err
		}
	}

	stoppedEntry := s.activeEntry
	s.activeEntry = nil
	s.parentEntry = nil
	s.state = StateIdle
	s.totalPaused = 0
	return stoppedEntry, nil
}

func (s *TimerService) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning {
		return errors.New("can only pause a running timer")
	}

	s.pausedAt = time.Now()
	s.state = StatePaused
	return nil
}

func (s *TimerService) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StatePaused {
		return errors.New("can only resume a paused timer")
	}

	s.totalPaused += time.Since(s.pausedAt)
	s.state = StateRunning
	return nil
}

func (s *TimerService) UpdateBookingText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeEntry == nil {
		return errors.New("no active entry")
	}

	s.activeEntry.BookingText = text
	s.debounceText = text

	if s.saveDebounce != nil {
		s.saveDebounce.Stop()
	}
	activeID := s.activeEntry.ID
	s.saveDebounce = time.AfterFunc(200*time.Millisecond, func() {
		_ = s.repo.UpdateActiveBookingText(activeID, text)
	})
	return nil
}

func (s *TimerService) UpdateTaskName(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeEntry == nil {
		return errors.New("no active entry")
	}
	s.activeEntry.TaskName = name
	return s.repo.UpdateActiveTaskName(s.activeEntry.ID, name)
}

func (s *TimerService) AssignProject(projectID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeEntry == nil {
		return errors.New("no active entry")
	}
	s.activeEntry.ProjectID = &projectID
	proj, err := s.repo.GetProject(projectID)
	if err == nil && proj != nil {
		s.activeEntry.ProjectName = proj.Name
		s.activeEntry.CustomerName = proj.CustomerName
		s.activeEntry.ProjectColor = proj.Color
	}
	return s.repo.AssignProject(s.activeEntry.ID, projectID)
}

func (s *TimerService) GetCurrentState() (TimerState, *db.TimeEntry, time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.activeEntry == nil || s.state == StateIdle {
		return StateIdle, nil, 0
	}

	var elapsed time.Duration
	if s.state == StatePaused {
		elapsed = s.pausedAt.Sub(s.activeEntry.StartedAt) - s.totalPaused
	} else {
		elapsed = time.Since(s.activeEntry.StartedAt) - s.totalPaused
	}
	if elapsed < 0 {
		elapsed = 0
	}
	return s.state, s.activeEntry, elapsed
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
			st := s.state
			entry := s.activeEntry
			var elapsed time.Duration
			if entry != nil && st != StateIdle {
				if st == StatePaused {
					elapsed = s.pausedAt.Sub(entry.StartedAt) - s.totalPaused
				} else {
					elapsed = time.Since(entry.StartedAt) - s.totalPaused
				}
				if elapsed < 0 {
					elapsed = 0
				}
			}
			listeners := make([]TickListener, len(s.listeners))
			copy(listeners, s.listeners)
			s.mu.RUnlock()

			for _, fn := range listeners {
				fn(elapsed, st, entry)
			}
		}
	}
}
