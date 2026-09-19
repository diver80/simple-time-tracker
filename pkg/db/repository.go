package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

	if _, err := db.Exec(SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteRepository{db: db}, nil
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

	res, err := r.db.Exec(
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

	var startedAt time.Time
	err := r.db.QueryRow("SELECT started_at FROM time_entries WHERE id = ?", id).Scan(&startedAt)
	if err != nil {
		return err
	}

	durationSec := int64(endedAt.Sub(startedAt).Seconds())
	if durationSec < 0 {
		durationSec = 0
	}

	now := time.Now().UTC()
	_, err = r.db.Exec(
		"UPDATE time_entries SET ended_at = ?, duration_sec = ?, updated_at = ? WHERE id = ?",
		endedAt.UTC(), durationSec, now, id,
	)
	return err
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
		       t.is_billable, t.is_quickshift, t.parent_id, t.created_at, t.updated_at
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
		&isBillable, &isQuickShift, &e.ParentID, &e.CreatedAt, &e.UpdatedAt,
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

	now := time.Now().UTC()
	billable := 0
	if entry.IsBillable {
		billable = 1
	}
	_, err := r.db.Exec(`
		UPDATE time_entries
		SET project_id = ?, task_name = ?, booking_text = ?, started_at = ?, ended_at = ?, duration_sec = ?, is_billable = ?, updated_at = ?
		WHERE id = ?
	`, entry.ProjectID, entry.TaskName, entry.BookingText, entry.StartedAt, entry.EndedAt, entry.DurationSec, billable, now, entry.ID)
	return err
}

func (r *SQLiteRepository) DeleteEntry(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec("DELETE FROM time_entries WHERE id = ?", id)
	return err
}

func (r *SQLiteRepository) ListEntriesForDay(day time.Time) ([]TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	y, m, d := day.Date()
	startOfDay := time.Date(y, m, d, 0, 0, 0, 0, day.Location()).UTC()
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT t.id, t.project_id, COALESCE(c.name, ''), COALESCE(p.name, ''), COALESCE(p.color, '#64748B'),
		       t.task_name, t.booking_text, t.started_at, t.ended_at, t.duration_sec,
		       t.is_billable, t.is_quickshift, t.parent_id, t.created_at, t.updated_at
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
			&isBillable, &isQuickShift, &e.ParentID, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.IsBillable = isBillable == 1
		e.IsQuickShift = isQuickShift == 1
		list = append(list, e)
	}
	return list, nil
}

func (r *SQLiteRepository) ListEntriesForMonth(year int, month time.Month, customerID *int64, projectID *int64) ([]TimeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	query := `
		SELECT t.id, t.project_id, COALESCE(c.name, ''), COALESCE(p.name, ''), COALESCE(p.color, '#64748B'),
		       t.task_name, t.booking_text, t.started_at, t.ended_at, t.duration_sec,
		       t.is_billable, t.is_quickshift, t.parent_id, t.created_at, t.updated_at
		FROM time_entries t
		LEFT JOIN projects p ON t.project_id = p.id
		LEFT JOIN customers c ON p.customer_id = c.id
		WHERE t.started_at >= ? AND t.started_at < ?
	`
	args := []interface{}{startOfMonth, endOfMonth}
	if projectID != nil {
		query += " AND t.project_id = ?"
		args = append(args, *projectID)
	} else if customerID != nil {
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
			&isBillable, &isQuickShift, &e.ParentID, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.IsBillable = isBillable == 1
		e.IsQuickShift = isQuickShift == 1
		list = append(list, e)
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
