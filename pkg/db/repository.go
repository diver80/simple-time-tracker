package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Repository defines all persistence operations.
type Repository interface {
	Close() error
	CreateCustomer(name string) (*Customer, error)
	ListCustomers() ([]Customer, error)
	GetCustomer(id int64) (*Customer, error)

	CreateProject(custID int64, name string, rate, budgetHours, budgetCost float64, color string) (*Project, error)
	ListProjects(custID *int64) ([]Project, error)
	GetProject(id int64) (*Project, error)

	StartTimeEntry(projectID *int64, taskName string, isBillable bool) (*TimeEntry, error)
	StartQuickShift(parentID int64, taskName string) (*TimeEntry, error)
	UpdateActiveBookingText(id int64, text string) error
	UpdateActiveTaskName(id int64, name string) error
	AssignProject(id int64, projectID int64) error
	StopTimeEntry(id int64, endedAt time.Time) error
	GetActiveEntry() (*TimeEntry, error)
	GetEntry(id int64) (*TimeEntry, error)
	UpdateEntry(entry *TimeEntry) error
	DeleteEntry(id int64) error
	CreateManualEntry(entry *TimeEntry) (*TimeEntry, error)
	UpdateCompletedEntry(entry *TimeEntry) error
	DeleteCompletedEntry(id int64) error

	ListEntriesForDay(day time.Time) ([]TimeEntry, error)
	ListEntriesForMonth(year int, month time.Month, customerID *int64, projectID *int64) ([]TimeEntry, error)
	GetProjectBudgetSummary(projectID int64) (totalHours float64, totalCost float64, err error)
}

type SQLiteRepository struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewRepository(dbPath string) (*SQLiteRepository, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Set max open connections to 1 universally for safety (URI and in-memory)
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Backward-compatible migration: add paused_at and paused_ns columns if they don't exist
	if err := migrateAddPauseColumns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate pause columns: %w", err)
	}

	// Recover an interruption created by a version that did not persist pauses.
	// Completed historical bookings are intentionally left unchanged.
	if _, err := db.Exec(`
		UPDATE time_entries SET paused_at = (
			SELECT MIN(child.started_at) FROM time_entries child
			WHERE child.parent_id = time_entries.id AND child.is_quickshift = 1 AND child.ended_at IS NULL
		)
		WHERE ended_at IS NULL AND paused_at IS NULL AND EXISTS (
			SELECT 1 FROM time_entries child
			WHERE child.parent_id = time_entries.id AND child.is_quickshift = 1 AND child.ended_at IS NULL
		)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to restore interrupted entry: %w", err)
	}
	return &SQLiteRepository{db: db}, nil
}

func migrateAddPauseColumns(db *sql.DB) error {
	// Check if paused_at column exists using PRAGMA table_info
	rows, err := db.Query("PRAGMA table_info(time_entries)")
	if err != nil {
		return err
	}

	var columnExists [2]bool // [0] = paused_at, [1] = paused_ns
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "paused_at" {
			columnExists[0] = true
		}
		if name == "paused_ns" {
			columnExists[1] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close() // Close before ALTER TABLE

	// Add paused_at if missing
	if !columnExists[0] {
		if _, err := db.Exec("ALTER TABLE time_entries ADD COLUMN paused_at DATETIME"); err != nil {
			return err
		}
	}

	// Add paused_ns if missing
	if !columnExists[1] {
		if _, err := db.Exec("ALTER TABLE time_entries ADD COLUMN paused_ns INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}

	return nil
}

func (r *SQLiteRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.db.Close()
}

func (r *SQLiteRepository) CreateCustomer(name string) (*Customer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	res, err := r.db.Exec("INSERT INTO customers (name, created_at) VALUES (?, ?)", name, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Customer{ID: id, Name: name, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListCustomers() ([]Customer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query("SELECT id, name, created_at FROM customers ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *SQLiteRepository) GetCustomer(id int64) (*Customer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var c Customer
	err := r.db.QueryRow("SELECT id, name, created_at FROM customers WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SQLiteRepository) CreateProject(custID int64, name string, rate, budgetHours, budgetCost float64, color string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if color == "" {
		color = "#3B82F6"
	}
	now := time.Now().UTC()
	res, err := r.db.Exec(
		"INSERT INTO projects (customer_id, name, hourly_rate, budget_hours, budget_cost, color, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?)",
		custID, name, rate, budgetHours, budgetCost, color, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var custName string
	_ = r.db.QueryRow("SELECT name FROM customers WHERE id = ?", custID).Scan(&custName)

	return &Project{
		ID:           id,
		CustomerID:   custID,
		CustomerName: custName,
		Name:         name,
		HourlyRate:   rate,
		BudgetHours:  budgetHours,
		BudgetCost:   budgetCost,
		Color:        color,
		IsActive:     true,
		CreatedAt:    now,
	}, nil
}

func (r *SQLiteRepository) ListProjects(custID *int64) ([]Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT p.id, p.customer_id, c.name, p.name, p.hourly_rate, p.budget_hours, p.budget_cost, p.color, p.is_active, p.created_at
		FROM projects p
		JOIN customers c ON p.customer_id = c.id
	`
	var args []interface{}
	if custID != nil {
		query += " WHERE p.customer_id = ?"
		args = append(args, *custID)
	}
	query += " ORDER BY c.name ASC, p.name ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.CustomerID, &p.CustomerName, &p.Name, &p.HourlyRate, &p.BudgetHours, &p.BudgetCost, &p.Color, &p.IsActive, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *SQLiteRepository) GetProject(id int64) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT p.id, p.customer_id, c.name, p.name, p.hourly_rate, p.budget_hours, p.budget_cost, p.color, p.is_active, p.created_at
		FROM projects p
		JOIN customers c ON p.customer_id = c.id
		WHERE p.id = ?
	`
	var p Project
	err := r.db.QueryRow(query, id).
		Scan(&p.ID, &p.CustomerID, &p.CustomerName, &p.Name, &p.HourlyRate, &p.BudgetHours, &p.BudgetCost, &p.Color, &p.IsActive, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SQLiteRepository) StartTimeEntry(projectID *int64, taskName string, isBillable bool) (*TimeEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if taskName == "" {
		taskName = "Untitled Task"
	}
	now := time.Now().UTC()
	billableInt := 0
	if isBillable {
		billableInt = 1
	}

	res, err := r.db.Exec(
		`INSERT INTO time_entries (project_id, task_name, booking_text, started_at, is_billable, is_quickshift, created_at, updated_at)
		 VALUES (?, ?, '', ?, ?, 0, ?, ?)`,
		projectID, taskName, now, billableInt, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.getEntryLocked(id)
}

func (r *SQLiteRepository) StartQuickShift(parentID int64, taskName string) (*TimeEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if taskName == "" {
		taskName = "QuickShift Interruption"
	}
	now := time.Now().UTC()

	// Begin transaction to atomically insert child and pause parent
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if parent exists and is not yet ended
	var parentEnded *time.Time
	err = tx.QueryRow("SELECT ended_at FROM time_entries WHERE id = ?", parentID).Scan(&parentEnded)
	if err != nil {
		return nil, err
	}

	// Only update parent pause if ended is null (still running)
	if parentEnded == nil {
		_, err = tx.Exec(
			"UPDATE time_entries SET paused_at = ?, updated_at = ? WHERE id = ?",
			now, now, parentID,
		)
		if err != nil {
			return nil, err
		}
	}

	// Insert child entry
	res, err := tx.Exec(
		`INSERT INTO time_entries (project_id, task_name, booking_text, started_at, is_billable, is_quickshift, parent_id, created_at, updated_at)
		 VALUES (NULL, ?, '', ?, 1, 1, ?, ?, ?)`,
		taskName, now, parentID, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.getEntryLocked(id)
}

func (r *SQLiteRepository) UpdateActiveBookingText(id int64, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	_, err := r.db.Exec("UPDATE time_entries SET booking_text = ?, updated_at = ? WHERE id = ?", text, now, id)
	return err
}

func (r *SQLiteRepository) UpdateActiveTaskName(id int64, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	_, err := r.db.Exec("UPDATE time_entries SET task_name = ?, updated_at = ? WHERE id = ?", name, now, id)
	return err
}

func (r *SQLiteRepository) AssignProject(id int64, projectID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	_, err := r.db.Exec("UPDATE time_entries SET project_id = ?, updated_at = ? WHERE id = ?", projectID, now, id)
	return err
}

func (r *SQLiteRepository) StopTimeEntry(id int64, endedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	endedAt = endedAt.UTC()

	// Begin transaction for atomic stop and parent resume
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var startedAt time.Time
	var currentEndedAt *time.Time
	var pausedNS int64
	var pausedAt *time.Time
	var isQuickShift int
	var parentID *int64
	err = tx.QueryRow("SELECT started_at, ended_at, paused_ns, paused_at, is_quickshift, parent_id FROM time_entries WHERE id = ?", id).
		Scan(&startedAt, &currentEndedAt, &pausedNS, &pausedAt, &isQuickShift, &parentID)
	if err != nil {
		return err
	}

	// A retry must preserve the first successful stop, even if its caller's
	// subsequent read failed or the retry uses a later wall-clock timestamp.
	if currentEndedAt != nil {
		return nil
	}
	if endedAt.Before(startedAt) {
		return fmt.Errorf("end time cannot be before start time")
	}

	// Calculate duration: subtract ongoing pause if PausedAt is set
	durationNS := endedAt.Sub(startedAt).Nanoseconds()

	// Subtract accumulated paused time
	durationNS -= pausedNS

	// Subtract ongoing pause period (EndedAt - PausedAt) if paused
	if pausedAt != nil && pausedAt.Before(endedAt) {
		durationNS -= endedAt.Sub(*pausedAt).Nanoseconds()
	}

	// Cap non-negative
	if durationNS < 0 {
		durationNS = 0
	}

	durationSec := durationNS / 1_000_000_000

	now := time.Now().UTC()
	_, err = tx.Exec(
		"UPDATE time_entries SET ended_at = ?, duration_sec = ?, updated_at = ? WHERE id = ?",
		endedAt, durationSec, now, id,
	)
	if err != nil {
		return err
	}

	// If this is a QuickShift with a parent, resume the parent if it's still open
	if isQuickShift == 1 && parentID != nil {
		var parentEndedAt *time.Time
		var parentPausedAt *time.Time
		var parentPausedNS int64
		err = tx.QueryRow("SELECT ended_at, paused_at, paused_ns FROM time_entries WHERE id = ?", *parentID).
			Scan(&parentEndedAt, &parentPausedAt, &parentPausedNS)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Only resume if parent is still open (ended_at is null) and was paused
		if parentEndedAt == nil && parentPausedAt != nil {
			// Calculate resumed time: max(0, endedAt - parentPausedAt)
			resumedNS := endedAt.Sub(*parentPausedAt).Nanoseconds()
			if resumedNS < 0 {
				resumedNS = 0
			}

			// Add resumed time to accumulated paused_ns and clear paused_at
			newPausedNS := parentPausedNS + resumedNS
			_, err = tx.Exec(
				"UPDATE time_entries SET paused_ns = ?, paused_at = NULL, updated_at = ? WHERE id = ?",
				newPausedNS, now, *parentID,
			)
			if err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *SQLiteRepository) GetActiveEntry() (*TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var id int64
	err := r.db.QueryRow("SELECT id FROM time_entries WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1").Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.getEntryLocked(id)
}

func (r *SQLiteRepository) GetEntry(id int64) (*TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getEntryLocked(id)
}

func (r *SQLiteRepository) getEntryLocked(id int64) (*TimeEntry, error) {
	query := `
		SELECT t.id, t.project_id, COALESCE(c.name, ''), COALESCE(p.name, ''), COALESCE(p.color, '#64748B'),
		       t.task_name, t.booking_text, t.started_at, t.ended_at, t.duration_sec,
		       t.is_billable, t.is_quickshift, t.parent_id, t.paused_at, t.paused_ns, t.created_at, t.updated_at
		FROM time_entries t
		LEFT JOIN projects p ON t.project_id = p.id
		LEFT JOIN customers c ON p.customer_id = c.id
		WHERE t.id = ?
	`
	var e TimeEntry
	var isBillable, isQuickShift int
	err := r.db.QueryRow(query, id).Scan(
		&e.ID, &e.ProjectID, &e.CustomerName, &e.ProjectName, &e.ProjectColor,
		&e.TaskName, &e.BookingText, &e.StartedAt, &e.EndedAt, &e.DurationSec,
		&isBillable, &isQuickShift, &e.ParentID, &e.PausedAt, &e.PausedNS, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.IsBillable = isBillable == 1
	e.IsQuickShift = isQuickShift == 1
	return &e, nil
}

func (r *SQLiteRepository) UpdateEntry(entry *TimeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Explicit nil check before deref
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// Normalize timestamps to UTC
	startedAt := entry.StartedAt.UTC()
	var endedAt *time.Time
	if entry.EndedAt != nil {
		val := entry.EndedAt.UTC()
		endedAt = &val
	}
	var pausedAt *time.Time
	if entry.PausedAt != nil {
		val := entry.PausedAt.UTC()
		pausedAt = &val
	}

	// Validate: reject negative duration
	if entry.DurationSec < 0 {
		return fmt.Errorf("duration cannot be negative")
	}

	// Validate: reject negative paused_ns
	if entry.PausedNS < 0 {
		return fmt.Errorf("paused_ns cannot be negative")
	}

	// Validate: reject end before start
	if endedAt != nil && endedAt.Before(startedAt) {
		return fmt.Errorf("end time cannot be before start time")
	}

	now := time.Now().UTC()
	billable := 0
	if entry.IsBillable {
		billable = 1
	}
	result, err := r.db.Exec(`
		UPDATE time_entries
		SET project_id = ?, task_name = ?, booking_text = ?, started_at = ?, ended_at = ?, duration_sec = ?, is_billable = ?, paused_at = ?, paused_ns = ?, updated_at = ?
		WHERE id = ?
	`, entry.ProjectID, entry.TaskName, entry.BookingText, startedAt, endedAt, entry.DurationSec, billable, pausedAt, entry.PausedNS, now, entry.ID)
	if err != nil {
		return err
	}

	// Check if entry was actually updated (detect lost/deleted active entry)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *SQLiteRepository) DeleteEntry(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec("DELETE FROM time_entries WHERE id = ?", id)
	return err
}

// CreateManualEntry atomically inserts an already-ended booking.
// It validates nonempty trimmed task, valid nonzero started/end timestamps with end strictly after start,
// and derives DurationSec from timestamps (subtracting any pauses if set).
// Never calls StartTimeEntry then Stop; this creates the complete entry in one operation.
func (r *SQLiteRepository) CreateManualEntry(entry *TimeEntry) (*TimeEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry == nil {
		return nil, fmt.Errorf("entry cannot be nil")
	}

	// Validate task name is nonempty when trimmed
	if strings.TrimSpace(entry.TaskName) == "" {
		return nil, fmt.Errorf("task name cannot be empty")
	}

	// Normalize timestamps to UTC
	startedAt := entry.StartedAt.UTC()
	var endedAt *time.Time
	if entry.EndedAt != nil {
		val := entry.EndedAt.UTC()
		endedAt = &val
	}

	// Validate: nonzero start time
	if startedAt.IsZero() {
		return nil, fmt.Errorf("start time cannot be zero")
	}

	// Validate: endedAt must be set and strictly after startedAt
	if endedAt == nil {
		return nil, fmt.Errorf("manual entry must have an end time")
	}
	if endedAt.IsZero() {
		return nil, fmt.Errorf("end time cannot be zero")
	}
	if !endedAt.After(startedAt) {
		return nil, fmt.Errorf("end time must be strictly after start time")
	}

	// Reject negative paused_ns at create
	if entry.PausedNS < 0 {
		return nil, fmt.Errorf("paused_ns cannot be negative")
	}

	// Derive DurationSec from timestamps
	durationNS := endedAt.Sub(startedAt).Nanoseconds()

	// Subtract accumulated paused time if set
	if entry.PausedNS > 0 {
		durationNS -= entry.PausedNS
	}

	// Subtract ongoing pause period if PausedAt is set
	var pausedAt *time.Time
	if entry.PausedAt != nil {
		val := entry.PausedAt.UTC()
		pausedAt = &val
		// Validate PausedAt is within [startedAt, endedAt]
		if pausedAt.Before(startedAt) || pausedAt.After(*endedAt) {
			return nil, fmt.Errorf("paused_at must be within time entry bounds")
		}
		if pausedAt.Before(*endedAt) && pausedAt.After(startedAt) {
			durationNS -= endedAt.Sub(*pausedAt).Nanoseconds()
		}
	}

	// Reject if span is shorter than total pauses
	if durationNS < 0 {
		return nil, fmt.Errorf("time span is shorter than accumulated pauses")
	}

	durationSec := durationNS / 1_000_000_000

	now := time.Now().UTC()
	billable := 0
	if entry.IsBillable {
		billable = 1
	}
	quickshift := 0
	if entry.IsQuickShift {
		quickshift = 1
	}

	result, err := r.db.Exec(`
		INSERT INTO time_entries (project_id, task_name, booking_text, started_at, ended_at, duration_sec, is_billable, is_quickshift, parent_id, paused_at, paused_ns, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ProjectID, entry.TaskName, entry.BookingText, startedAt, endedAt, durationSec, billable, quickshift, entry.ParentID, pausedAt, entry.PausedNS, now, now)

	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Retrieve and return the inserted entry
	return r.getEntryLocked(id)
}

// UpdateCompletedEntry updates a completed (ended) entry.
// It rejects live rows (where ended_at is null) at the SQL level.
// It preserves QuickShift parent metadata and project_id.
// If time bounds (started_at/ended_at) are unchanged, pauses and pause deductions are preserved.
// On time adjustment, it derives new duration by subtracting existing effective pauses using OLD endedAt.
// It rejects shorter spans than total pauses, and clears terminal PausedAt while consolidating pause to PausedNS.
func (r *SQLiteRepository) UpdateCompletedEntry(entry *TimeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// Validate task name is nonempty when trimmed
	if strings.TrimSpace(entry.TaskName) == "" {
		return fmt.Errorf("task name cannot be empty")
	}

	// Normalize timestamps to UTC
	startedAt := entry.StartedAt.UTC()
	var endedAt *time.Time
	if entry.EndedAt != nil {
		val := entry.EndedAt.UTC()
		endedAt = &val
	}

	// Validate: nonzero start time
	if startedAt.IsZero() {
		return fmt.Errorf("start time cannot be zero")
	}

	// Validate: must have an end time (completed entry)
	if endedAt == nil {
		return fmt.Errorf("cannot update entry without end time using UpdateCompletedEntry")
	}

	// Validate: nonzero end time
	if endedAt.IsZero() {
		return fmt.Errorf("end time cannot be zero")
	}

	// Validate: end must be after start
	if !endedAt.After(startedAt) {
		return fmt.Errorf("end time must be strictly after start time")
	}

	// Validate: reject negative paused_ns
	if entry.PausedNS < 0 {
		return fmt.Errorf("paused_ns cannot be negative")
	}

	// Get current entry to check if live and compare time bounds
	var currentEndedAt *time.Time
	var currentStartedAt time.Time
	var currentPausedNS int64
	var currentPausedAt *time.Time
	err := r.db.QueryRow("SELECT started_at, ended_at, paused_ns, paused_at FROM time_entries WHERE id = ?", entry.ID).
		Scan(&currentStartedAt, &currentEndedAt, &currentPausedNS, &currentPausedAt)
	if err != nil {
		return err
	}

	// Reject if entry is live (ended_at is null)
	if currentEndedAt == nil {
		return fmt.Errorf("cannot update live entry; use UpdateEntry for active/paused entries")
	}

	// Determine if time bounds changed
	timeBoundsChanged := !currentStartedAt.Equal(startedAt) || (currentEndedAt == nil && endedAt != nil) || (currentEndedAt != nil && endedAt == nil) || (currentEndedAt != nil && endedAt != nil && !currentEndedAt.Equal(*endedAt))

	var newDurationSec int64 = entry.DurationSec
	var newPausedNS int64 = entry.PausedNS
	var newPausedAt *time.Time = entry.PausedAt

	if timeBoundsChanged {
		// Recalculate duration based on new time bounds and existing pauses using OLD currentEndedAt
		durationNS := endedAt.Sub(startedAt).Nanoseconds()

		// Subtract accumulated paused time
		durationNS -= currentPausedNS

		// Subtract ongoing pause period if it exists in current data, using OLD currentEndedAt
		if currentPausedAt != nil && currentPausedAt.Before(*currentEndedAt) {
			durationNS -= currentEndedAt.Sub(*currentPausedAt).Nanoseconds()
		}

		// Reject if span is shorter than total pauses
		if durationNS < 0 {
			return fmt.Errorf("time span is shorter than accumulated pauses")
		}

		newDurationSec = durationNS / 1_000_000_000

		// Consolidate pause: clear terminal PausedAt and add ongoing pause to PausedNS
		if currentPausedAt != nil {
			// Use OLD currentEndedAt to compute the original ongoing pause
			ongoingPauseNS := currentEndedAt.Sub(*currentPausedAt).Nanoseconds()
			if ongoingPauseNS > 0 {
				newPausedNS = currentPausedNS + ongoingPauseNS
			}
			newPausedAt = nil
		} else {
			// No existing pause, initialize newPausedNS from current when time changed
			newPausedNS = currentPausedNS
		}
	} else {
		// Time bounds unchanged: preserve pauses if not explicitly provided in entry
		// If entry has explicit pause data, use it; otherwise preserve current
		if entry.PausedAt == nil && entry.PausedNS == 0 {
			newPausedNS = currentPausedNS
			newPausedAt = currentPausedAt
		}
	}

	now := time.Now().UTC()
	billable := 0
	if entry.IsBillable {
		billable = 1
	}
	result, err := r.db.Exec(`
		UPDATE time_entries
		SET task_name = ?, booking_text = ?, started_at = ?, ended_at = ?, duration_sec = ?, is_billable = ?, project_id = ?, paused_at = ?, paused_ns = ?, updated_at = ?
		WHERE id = ? AND ended_at IS NOT NULL
	`, entry.TaskName, entry.BookingText, startedAt, endedAt, newDurationSec, billable, entry.ProjectID, newPausedAt, newPausedNS, now, entry.ID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("entry not found or is still active")
	}

	return nil
}

// DeleteCompletedEntry deletes a completed entry.
// It protects unfinished rows (live entries with no end time).
// It protects entries with live children (completed children are preserved via SET NULL).
// Returns meaningful errors and checks rows affected.
func (r *SQLiteRepository) DeleteCompletedEntry(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Begin transaction for atomic checks and delete
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if entry exists and is completed (has ended_at)
	var endedAt *time.Time
	err = tx.QueryRow("SELECT ended_at FROM time_entries WHERE id = ?", id).
		Scan(&endedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("entry not found")
		}
		return err
	}

	// Reject if entry is live (unfinished)
	if endedAt == nil {
		return fmt.Errorf("cannot delete live entry; use DeleteEntry for active/paused entries")
	}

	// Check if this entry has any live children
	var liveChildCount int64
	err = tx.QueryRow(
		"SELECT COUNT(*) FROM time_entries WHERE parent_id = ? AND ended_at IS NULL",
		id,
	).Scan(&liveChildCount)
	if err != nil {
		return err
	}

	if liveChildCount > 0 {
		return fmt.Errorf("cannot delete entry with live children; complete or delete them first")
	}

	// Delete the entry (includes protection: only if ended_at IS NOT NULL AND no live children)
	result, err := tx.Exec(
		"DELETE FROM time_entries WHERE id = ? AND ended_at IS NOT NULL AND NOT EXISTS (SELECT 1 FROM time_entries child WHERE child.parent_id = time_entries.id AND child.ended_at IS NULL)",
		id,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("entry not found or still has live children")
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *SQLiteRepository) ListEntriesForDay(day time.Time) ([]TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	y, m, d := day.Date()
	localMidnight := time.Date(y, m, d, 0, 0, 0, 0, day.Location())
	endOfDay := localMidnight.AddDate(0, 0, 1)
	startOfDay := localMidnight.UTC()
	endOfDay = endOfDay.UTC()

	query := `
		SELECT t.id, t.project_id, COALESCE(c.name, ''), COALESCE(p.name, ''), COALESCE(p.color, '#64748B'),
		       t.task_name, t.booking_text, t.started_at, t.ended_at, t.duration_sec,
		       t.is_billable, t.is_quickshift, t.parent_id, t.paused_at, t.paused_ns, t.created_at, t.updated_at
		FROM time_entries t
		LEFT JOIN projects p ON t.project_id = p.id
		LEFT JOIN customers c ON p.customer_id = c.id
		WHERE t.started_at >= ? AND t.started_at < ?
		ORDER BY t.started_at ASC
	`
	rows, err := r.db.Query(query, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TimeEntry
	for rows.Next() {
		var e TimeEntry
		var isBillable, isQuickShift int
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.CustomerName, &e.ProjectName, &e.ProjectColor,
			&e.TaskName, &e.BookingText, &e.StartedAt, &e.EndedAt, &e.DurationSec,
			&isBillable, &isQuickShift, &e.ParentID, &e.PausedAt, &e.PausedNS, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.IsBillable = isBillable == 1
		e.IsQuickShift = isQuickShift == 1
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *SQLiteRepository) ListEntriesForMonth(year int, month time.Month, customerID *int64, projectID *int64) ([]TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use local time boundaries and convert to UTC to match UI months
	loc := time.Local
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc).UTC()
	endOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0).UTC()

	query := `
		SELECT t.id, t.project_id, COALESCE(c.name, ''), COALESCE(p.name, ''), COALESCE(p.color, '#64748B'),
		       t.task_name, t.booking_text, t.started_at, t.ended_at, t.duration_sec,
		       t.is_billable, t.is_quickshift, t.parent_id, t.paused_at, t.paused_ns, t.created_at, t.updated_at
		FROM time_entries t
		LEFT JOIN projects p ON t.project_id = p.id
		LEFT JOIN customers c ON p.customer_id = c.id
		WHERE t.started_at >= ? AND t.started_at < ?
	`
	args := []interface{}{startOfMonth, endOfMonth}
	if projectID != nil {
		query += " AND t.project_id = ?"
		args = append(args, *projectID)
	}
	if customerID != nil {
		query += " AND p.customer_id = ?"
		args = append(args, *customerID)
	}
	query += " ORDER BY t.started_at ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TimeEntry
	for rows.Next() {
		var e TimeEntry
		var isBillable, isQuickShift int
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.CustomerName, &e.ProjectName, &e.ProjectColor,
			&e.TaskName, &e.BookingText, &e.StartedAt, &e.EndedAt, &e.DurationSec,
			&isBillable, &isQuickShift, &e.ParentID, &e.PausedAt, &e.PausedNS, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.IsBillable = isBillable == 1
		e.IsQuickShift = isQuickShift == 1
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *SQLiteRepository) GetProjectBudgetSummary(projectID int64) (float64, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT COALESCE(SUM(t.duration_sec), 0), COALESCE(p.hourly_rate, 0)
		FROM time_entries t
		JOIN projects p ON t.project_id = p.id
		WHERE t.project_id = ? AND t.is_billable = 1
	`
	var totalSec int64
	var rate float64
	err := r.db.QueryRow(query, projectID).Scan(&totalSec, &rate)
	if err != nil {
		return 0, 0, err
	}

	totalHours := float64(totalSec) / 3600.0
	totalCost := totalHours * rate
	return totalHours, totalCost, nil
}
