package export

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yokto-time/pkg/db"
	"yokto-time/pkg/timer"
)

func TestGenerateMonthlyCSV(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	// 1. Create test customer & project
	cust, err := repo.CreateCustomer("MegaCorp")
	if err != nil {
		t.Fatalf("failed to create customer: %v", err)
	}
	proj, err := repo.CreateProject(cust.ID, "Cloud Migration", 150.0, 50.0, 7500.0, "#3B82F6")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// 2. Create sample entries in September 2026
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	entry1, err := repo.StartTimeEntry(&proj.ID, "Terraform IAM setup", true)
	if err != nil {
		t.Fatalf("failed to start entry: %v", err)
	}
	entry1.BookingText = "Configured AWS roles and policies"
	entry1.StartedAt = now
	end1 := now.Add(90 * time.Minute) // 1.5 hours
	entry1.EndedAt = &end1
	entry1.DurationSec = 5400
	_ = repo.UpdateEntry(entry1)

	// Second entry: QuickShift call (15 mins)
	qs, err := repo.StartQuickShift(entry1.ID, "Ad-hoc Client Q&A")
	if err != nil {
		t.Fatalf("failed to start quickshift: %v", err)
	}
	qs.BookingText = "Answered security audit questions"
	qs.StartedAt = end1
	end2 := end1.Add(15 * time.Minute)
	qs.EndedAt = &end2
	qs.DurationSec = 900
	_ = repo.UpdateEntry(qs)

	// 3. Export CSV with 15m Ceil rounding
	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 15,
		RoundMode:   timer.RoundCeil,
	}

	csvContent, summary, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		t.Fatalf("failed to generate monthly CSV: %v", err)
	}

	if summary.TotalEntries != 2 {
		t.Errorf("expected 2 total entries, got %d", summary.TotalEntries)
	}
	// 90m (1.5h) + 15m (0.25h) = 1.75 hours
	if summary.TotalHours != 1.75 {
		t.Errorf("expected 1.75 total hours, got %f", summary.TotalHours)
	}

	// Revenue: 1.5h * 150 = 225 (Quickshift had no project assigned, rate 0)
	if summary.TotalRevenue != 225.0 {
		t.Errorf("expected 225.0 revenue, got %f", summary.TotalRevenue)
	}

	// Verify headers and content
	if !strings.Contains(csvContent, "Terraform IAM setup") {
		t.Errorf("CSV missing task name 'Terraform IAM setup'")
	}
	if !strings.Contains(csvContent, "Configured AWS roles and policies") {
		t.Errorf("CSV missing booking text")
	}
	if !strings.Contains(csvContent, "SUMME") {
		t.Errorf("CSV missing SUMME summary row")
	}
}
