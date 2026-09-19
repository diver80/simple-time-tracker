package db

import (
	"path/filepath"
	"testing"
	"time"
)

// TestCreateManualEntryBasic tests basic manual entry creation with proper time bounds
func TestCreateManualEntryBasic(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	entry := &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Manual Task",
		BookingText: "Manual booking",
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	}

	result, err := repo.CreateManualEntry(entry)
	if err != nil {
		t.Fatalf("failed to create manual entry: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if result.DurationSec != 7200 { // 2 hours
		t.Errorf("expected duration 7200 sec, got %d", result.DurationSec)
	}
	if result.TaskName != "Manual Task" {
		t.Errorf("expected task name 'Manual Task', got %q", result.TaskName)
	}
	if *result.EndedAt != end {
		t.Errorf("expected end time %v, got %v", end, result.EndedAt)
	}
}

// TestCreateManualEntryValidation tests validation of manual entry creation
func TestCreateManualEntryValidation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_validation_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)

	// Test 1: nil entry
	_, err = repo.CreateManualEntry(nil)
	if err == nil {
		t.Errorf("expected error for nil entry")
	}

	// Test 2: empty task name
	entry := &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "   ", // only whitespace
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Errorf("expected error for empty/whitespace task name")
	}

	// Test 3: missing end time
	entry = &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task",
		StartedAt:  start,
		EndedAt:    nil,
		IsBillable: true,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Errorf("expected error for missing end time")
	}

	// Test 4: end before start
	endBeforeStart := start.Add(-1 * time.Hour)
	entry = &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task",
		StartedAt:  start,
		EndedAt:    &endBeforeStart,
		IsBillable: true,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Errorf("expected error for end before start")
	}

	// Test 5: end equal to start
	entry = &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task",
		StartedAt:  start,
		EndedAt:    &start,
		IsBillable: true,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Errorf("expected error for end equal to start")
	}
}

// TestCreateManualEntryWithPauses tests manual entry creation with pause deductions
func TestCreateManualEntryWithPauses(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_pauses_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour) // 60 minutes total

	// Create entry with 15 minutes accumulated pause
	entry := &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task with Pauses",
		StartedAt:  start,
		EndedAt:    &end,
		PausedNS:   int64(15 * time.Minute), // 15 min accumulated
		IsBillable: true,
	}

	result, err := repo.CreateManualEntry(entry)
	if err != nil {
		t.Fatalf("failed to create manual entry with pauses: %v", err)
	}

	// Expected duration: 60 min - 15 min = 45 min = 2700 sec
	expectedDurationSec := int64(45 * 60)
	if result.DurationSec != expectedDurationSec {
		t.Errorf("expected duration %d sec, got %d sec", expectedDurationSec, result.DurationSec)
	}
}

// TestCreateManualEntryUTC tests that timestamps are normalized to UTC
func TestCreateManualEntryUTC(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_utc_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	// Create timestamps in non-UTC timezone
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, loc)
	end := start.Add(1 * time.Hour)

	entry := &TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "UTC Test",
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	}

	result, err := repo.CreateManualEntry(entry)
	if err != nil {
		t.Fatalf("failed to create manual entry: %v", err)
	}

	// Verify stored timestamps are in UTC
	if result.StartedAt.Location() != time.UTC {
		t.Errorf("expected UTC location for StartedAt, got %v", result.StartedAt.Location())
	}
	if result.EndedAt.Location() != time.UTC {
		t.Errorf("expected UTC location for EndedAt, got %v", result.EndedAt.Location())
	}
}

// TestUpdateCompletedEntryBasic tests basic update of a completed entry
func TestUpdateCompletedEntryBasic(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_completed_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	// Create a completed entry
	created, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Original Task",
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	})

	// Update the task name only
	created.TaskName = "Updated Task"
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatalf("failed to update completed entry: %v", err)
	}

	// Verify update
	retrieved, _ := repo.GetEntry(created.ID)
	if retrieved.TaskName != "Updated Task" {
		t.Errorf("expected task name 'Updated Task', got %q", retrieved.TaskName)
	}
}

// TestUpdateCompletedEntryRejectLive tests that live entries cannot be updated via UpdateCompletedEntry
func TestUpdateCompletedEntryRejectLive(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_reject_live_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	// Create an active (live) entry
	active, _ := repo.StartTimeEntry(&proj.ID, "Active Task", true)

	// Try to update with UpdateCompletedEntry - should fail
	err = repo.UpdateCompletedEntry(active)
	if err == nil {
		t.Errorf("expected error when updating live entry with UpdateCompletedEntry")
	}
}

// TestUpdateCompletedEntryWithTimeAdjustment tests duration recalculation when time bounds change
func TestUpdateCompletedEntryWithTimeAdjustment(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_time_adj_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour) // 60 minutes

	// Create entry with accumulated pause
	created, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task with Pause",
		StartedAt:  start,
		EndedAt:    &end,
		PausedNS:   int64(15 * time.Minute), // 15 min pause
		IsBillable: true,
	})
	// Created with duration: 60 - 15 = 45 minutes

	// Extend end time by 30 minutes
	newEnd := start.Add(90 * time.Minute)
	created.EndedAt = &newEnd
	created.DurationSec = 0 // Let it recalculate

	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatalf("failed to update with time adjustment: %v", err)
	}

	// Verify duration recalculation: 90 min - 15 min pause = 75 min = 4500 sec
	retrieved, _ := repo.GetEntry(created.ID)
	expectedDurationSec := int64(75 * 60)
	if retrieved.DurationSec != expectedDurationSec {
		t.Errorf("expected duration %d sec, got %d sec", expectedDurationSec, retrieved.DurationSec)
	}
}

// TestUpdateCompletedEntryPreservePausesOnTitleEdit tests that pauses are preserved on title-only edits
func TestUpdateCompletedEntryPreservePausesOnTitleEdit(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_preserve_pauses_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)

	// Create entry with pauses
	created, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task",
		StartedAt:  start,
		EndedAt:    &end,
		PausedNS:   int64(15 * time.Minute),
		IsBillable: true,
	})

	// Store original duration
	originalDuration := created.DurationSec

	// Update only task name (no time bounds change)
	created.TaskName = "Updated Task"
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Verify duration remains unchanged
	retrieved, _ := repo.GetEntry(created.ID)
	if retrieved.DurationSec != originalDuration {
		t.Errorf("expected duration to remain %d sec, but got %d sec", originalDuration, retrieved.DurationSec)
	}
}

// TestUpdateCompletedEntryRejectInvalidSpan tests rejection of span shorter than pauses
func TestUpdateCompletedEntryRejectInvalidSpan(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_invalid_span_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour) // 60 minutes

	// Create entry with 45 minutes of pause
	created, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task",
		StartedAt:  start,
		EndedAt:    &end,
		PausedNS:   int64(45 * time.Minute), // 45 min pause
		IsBillable: true,
	})

	// Try to shrink end time to 30 minutes (shorter than 45 minute pause)
	shortEnd := start.Add(30 * time.Minute)
	created.EndedAt = &shortEnd

	err = repo.UpdateCompletedEntry(created)
	if err == nil {
		t.Errorf("expected error for span shorter than pauses")
	}
}

// TestDeleteCompletedEntryBasic tests basic deletion of a completed entry
func TestDeleteCompletedEntryBasic(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delete_completed_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)

	// Create a completed entry
	created, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Task to Delete",
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	})

	// Delete it
	err = repo.DeleteCompletedEntry(created.ID)
	if err != nil {
		t.Fatalf("failed to delete completed entry: %v", err)
	}

	// Verify deletion
	_, err = repo.GetEntry(created.ID)
	if err == nil {
		t.Errorf("expected error when retrieving deleted entry")
	}
}

// TestDeleteCompletedEntryRejectLive tests that live entries cannot be deleted via DeleteCompletedEntry
func TestDeleteCompletedEntryRejectLive(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delete_reject_live_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	// Create an active (live) entry
	active, _ := repo.StartTimeEntry(&proj.ID, "Active Task", true)

	// Try to delete with DeleteCompletedEntry - should fail
	err = repo.DeleteCompletedEntry(active.ID)
	if err == nil {
		t.Errorf("expected error when deleting live entry with DeleteCompletedEntry")
	}

	// Verify entry still exists
	retrieved, err := repo.GetEntry(active.ID)
	if err != nil || retrieved.ID == 0 {
		t.Errorf("expected entry to still exist after failed delete")
	}
}

// TestDeleteCompletedEntryNotFound tests error handling for non-existent entries
func TestDeleteCompletedEntryNotFound(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delete_not_found_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	// Try to delete non-existent entry
	err = repo.DeleteCompletedEntry(99999)
	if err == nil {
		t.Errorf("expected error for non-existent entry")
	}
}

// TestCreateManualEntryWhileTimerRunning tests creating manual entry while timer is active
func TestCreateManualEntryWhileTimerRunning(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_while_timer_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	// Start a timer
	active, _ := repo.StartTimeEntry(&proj.ID, "Active", true)

	// Create manual entry in parallel (different time range)
	start := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)

	created, err := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Manual Task",
		StartedAt:  start,
		EndedAt:    &end,
		IsBillable: true,
	})

	if err != nil {
		t.Fatalf("failed to create manual entry while timer running: %v", err)
	}
	if created.ID == 0 {
		t.Errorf("expected non-zero ID for created entry")
	}

	// Verify active entry is still active
	activeCheck, _ := repo.GetActiveEntry()
	if activeCheck == nil || activeCheck.ID != active.ID {
		t.Errorf("expected active entry to remain unchanged")
	}
}

// TestManualEntryInteractionWithQuickShift tests manual entries with QuickShift metadata
func TestManualEntryInteractionWithQuickShift(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "manual_quickshift_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("TestCust")
	proj, _ := repo.CreateProject(cust.ID, "TestProj", 100, 40, 4000, "#000000")

	// Create a parent entry
	parentStart := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	parentEnd := parentStart.Add(2 * time.Hour)
	parent, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Parent",
		StartedAt:  parentStart,
		EndedAt:    &parentEnd,
		IsBillable: true,
	})

	// Create a child QuickShift entry
	childStart := parentStart.Add(30 * time.Minute)
	childEnd := childStart.Add(30 * time.Minute)

	child, _ := repo.CreateManualEntry(&TimeEntry{
		ProjectID:  &proj.ID,
		TaskName:   "Child",
		StartedAt:  childStart,
		EndedAt:    &childEnd,
		IsQuickShift: true,
		ParentID:   &parent.ID,
		IsBillable: true,
	})

	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Errorf("expected child to have parent relationship")
	}
}
