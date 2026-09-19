package db

import (
	"time"
)

// Customer represents a client for whom work is performed.
type Customer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Project represents a project under a Customer.
type Project struct {
	ID          int64     `json:"id"`
	CustomerID  int64     `json:"customer_id"`
	CustomerName string   `json:"customer_name"`
	Name        string    `json:"name"`
	HourlyRate  float64   `json:"hourly_rate"`
	BudgetHours float64   `json:"budget_hours"`
	BudgetCost  float64   `json:"budget_cost"`
	Color       string    `json:"color"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// TimeEntry represents a single tracked time block.
type TimeEntry struct {
	ID           int64      `json:"id"`
	ProjectID    *int64     `json:"project_id,omitempty"`
	CustomerName string     `json:"customer_name"`
	ProjectName  string     `json:"project_name"`
	ProjectColor string     `json:"project_color"`
	TaskName     string     `json:"task_name"`
	BookingText  string     `json:"booking_text"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	DurationSec  int64      `json:"duration_sec"`
	IsBillable   bool       `json:"is_billable"`
	IsQuickShift bool       `json:"is_quickshift"`
	ParentID     *int64     `json:"parent_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
