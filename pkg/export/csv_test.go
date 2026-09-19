package export

import (
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
)

// MockRepository is a stub for testing error handling
type MockRepository struct {
	db.Repository
	listProjectsErr error
}

func (m *MockRepository) ListProjects(custID *int64) ([]db.Project, error) {
	if m.listProjectsErr != nil {
		return nil, m.listProjectsErr
	}
	return []db.Project{}, nil
}

func (m *MockRepository) ListEntriesForMonth(year int, month time.Month, customerID *int64, projectID *int64) ([]db.TimeEntry, error) {
	return []db.TimeEntry{}, nil
}

// TestListProjectsError verifies that GenerateMonthlyCSV returns wrapped error
// when ListProjects fails, with no CSV or summary returned.
func TestListProjectsError(t *testing.T) {
	mockRepo := &MockRepository{
		listProjectsErr: fmt.Errorf("database connection failed"),
	}

	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 0,
		RoundMode:   timer.RoundNone,
	}

	csvContent, summary, err := GenerateMonthlyCSV(mockRepo, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list projects") {
		t.Errorf("expected wrapped error message, got: %v", err)
	}
	if csvContent != "" {
		t.Errorf("expected empty CSV on error, got: %s", csvContent)
	}
	if summary != nil {
		t.Errorf("expected nil summary on error, got: %+v", summary)
	}
}

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

// TestTotalDurationDriftCorrection verifies that total HH:MM is computed from
// accumulated rounded durations, not from accumulated decimal hours.
// Ten one-minute entries total ten minutes. Summing their individually rounded
// decimal hours instead gives 10 * 0.02h = 0.20h (twelve minutes).
func TestTotalDurationDriftCorrection(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100.0, 0, 0, "#FFF")

	// Create 10 entries of 1 minute each (60 seconds)
	// With RoundNearest(1 min), each stays 1 min: total = 10 min
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		entry, _ := repo.StartTimeEntry(&proj.ID, "Test", false)
		entry.StartedAt = now.Add(time.Duration(i*10) * time.Minute)
		end := entry.StartedAt.Add(60 * time.Second)
		entry.EndedAt = &end
		entry.DurationSec = 60
		repo.UpdateEntry(entry)
	}

	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 1,
		RoundMode:   timer.RoundNearest,
	}

	_, summary, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		t.Fatalf("failed to generate CSV: %v", err)
	}

	// 10 entries * 60 seconds rounded = 10 minutes total
	// DecimalHours(10 min) = 10/60 ≈ 0.167
	expectedHours := timer.DecimalHours(10 * time.Minute)
	if summary.TotalHours != expectedHours {
		t.Errorf("expected %f total hours, got %f", expectedHours, summary.TotalHours)
	}
	if summary.FormattedTotal != "00:10" {
		t.Errorf("expected FormattedTotal '00:10', got '%s'", summary.FormattedTotal)
	}
}

// TestRevenueBillingPrecision verifies that revenue is computed per line as
// roundedDur.Hours() * rate, rounded to cents, then summed.
// Example: 1 minute at 60/h should bill exactly 1.00, not 1.20.
func TestRevenueBillingPrecision(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 60.0, 0, 0, "#FFF")

	// Create entry: 1 minute (60 seconds) at 60/h rate
	// 1 min = 1/60 hour ≈ 0.01667 hours
	// 0.01667 * 60 = 1.00 (exactly)
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	entry, _ := repo.StartTimeEntry(&proj.ID, "1-min task", true)
	entry.StartedAt = now
	end := now.Add(60 * time.Second)
	entry.EndedAt = &end
	entry.DurationSec = 60
	repo.UpdateEntry(entry)

	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 0,
		RoundMode:   timer.RoundNone,
	}

	csvContent, summary, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		t.Fatalf("failed to generate CSV: %v", err)
	}

	// Revenue should be exactly 1.00, not 1.20
	if summary.TotalRevenue != 1.00 {
		t.Errorf("expected revenue 1.00, got %f", summary.TotalRevenue)
	}

	// Verify CSV contains the correct revenue value
	if !strings.Contains(csvContent, "1.00") && !strings.Contains(csvContent, "1,00") {
		t.Errorf("CSV should contain revenue 1.00 or 1,00, got:\n%s", csvContent)
	}
}

// TestSpreadsheetFormulaInjection verifies that user-controlled text fields
// are sanitized against formula injection attacks.
func TestSpreadsheetFormulaInjection(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	// Create customer with formula injection in name
	cust, _ := repo.CreateCustomer("-2+5*3")
	// Create project with formula injection in name
	proj, _ := repo.CreateProject(cust.ID, "=INDIRECT(\"url\")", 100.0, 0, 0, "#FFF")

	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	// TaskName with formula injection
	entry, _ := repo.StartTimeEntry(&proj.ID, "+1234567890", true)
	// BookingText with formula injection
	entry.BookingText = "@SUM(A1:A100)"
	entry.StartedAt = now
	end := now.Add(1 * time.Hour)
	entry.EndedAt = &end
	entry.DurationSec = 3600
	repo.UpdateEntry(entry)

	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 0,
		RoundMode:   timer.RoundNone,
	}

	csvContent, _, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		t.Fatalf("failed to generate CSV: %v", err)
	}

	// All formula-injection-prone fields should be prefixed with single quote
	if !strings.Contains(csvContent, "'-2+5*3") {
		t.Errorf("CustomerName not sanitized: expected \"'-2+5*3\" in CSV, got:\n%s", csvContent)
	}
	if !strings.Contains(csvContent, "'=INDIRECT") {
		t.Errorf("ProjectName not sanitized: expected \"'=INDIRECT\" in CSV, got:\n%s", csvContent)
	}
	if !strings.Contains(csvContent, "'+1234567890") {
		t.Errorf("TaskName not sanitized: expected \"'+1234567890\" in CSV, got:\n%s", csvContent)
	}
	if !strings.Contains(csvContent, "'@SUM") {
		t.Errorf("BookingText not sanitized: expected \"'@SUM\" in CSV, got:\n%s", csvContent)
	}
}

// TestParsedCSVUserFields verifies that all four user-controlled fields
// are correctly written and can be parsed via encoding/csv.
func TestParsedCSVUserFields(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := db.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Acme Corp")
	proj, _ := repo.CreateProject(cust.ID, "Widget Project", 100.0, 0, 0, "#FFF")

	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	entry, _ := repo.StartTimeEntry(&proj.ID, "Implementation", true)
	entry.BookingText = "Coded feature X"
	entry.StartedAt = now
	end := now.Add(2 * time.Hour)
	entry.EndedAt = &end
	entry.DurationSec = 7200
	repo.UpdateEntry(entry)

	opts := ExportOptions{
		Year:        2026,
		Month:       time.September,
		RoundingMin: 0,
		RoundMode:   timer.RoundNone,
	}

	csvContent, _, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		t.Fatalf("failed to generate CSV: %v", err)
	}

	// Skip BOM and parse CSV
	csvLines := strings.TrimPrefix(csvContent, "\xEF\xBB\xBF")
	reader := csv.NewReader(strings.NewReader(csvLines))
	reader.Comma = ';'

	// Read header
	header, err := reader.Read()
	if err != nil {
		t.Fatalf("failed to read header: %v", err)
	}

	// Find column indices
	custIdx, projIdx, taskIdx, bookingIdx := -1, -1, -1, -1
	for i, col := range header {
		switch col {
		case "Kunde":
			custIdx = i
		case "Projekt":
			projIdx = i
		case "Aufgabe":
			taskIdx = i
		case "Buchungstext":
			bookingIdx = i
		}
	}

	if custIdx < 0 || projIdx < 0 || taskIdx < 0 || bookingIdx < 0 {
		t.Fatalf("failed to find all required columns in header: %v", header)
	}

	// Read data row (skip SUMME row)
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("failed to read data row: %v", err)
	}

	// Verify four user fields
	if row[custIdx] != "Acme Corp" {
		t.Errorf("expected CustomerName 'Acme Corp', got %q", row[custIdx])
	}
	if row[projIdx] != "Widget Project" {
		t.Errorf("expected ProjectName 'Widget Project', got %q", row[projIdx])
	}
	if row[taskIdx] != "Implementation" {
		t.Errorf("expected TaskName 'Implementation', got %q", row[taskIdx])
	}
	if row[bookingIdx] != "Coded feature X" {
		t.Errorf("expected BookingText 'Coded feature X', got %q", row[bookingIdx])
	}
}

// TestSanitizeForSpreadsheet is a unit test for the sanitization function.
func TestSanitizeForSpreadsheet(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Formula triggers
		{"=SUM(A:A)", "'=SUM(A:A)"},
		{"+1234", "'+1234"},
		{"-5", "'-5"},
		{"@IMPORT", "'@IMPORT"},
		{"\tDATA", "'\tDATA"},
		{"\rCR", "'\rCR"},
		{"\nLF", "'\nLF"},
		// Normal text - should not be modified
		{"Normal text", "Normal text"},
		{"123", "123"},
		{"Text=with=equals", "Text=with=equals"},
		{" Leading space is OK", " Leading space is OK"},
		{" =formula after space", "' =formula after space"},
		{"\u2003=1+1", "'\u2003=1+1"},
		{" \tDATA", "' \tDATA"},
		{"\u00a0@SUM(A1)", "'\u00a0@SUM(A1)"},
		{"\u2003ordinary text", "\u2003ordinary text"},
		{"", ""},
	}

	for _, tt := range tests {
		result := sanitizeForSpreadsheet(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeForSpreadsheet(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
