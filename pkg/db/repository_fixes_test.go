package db

import (
	"testing"
	"time"
)

// TestCreateManualEntryZeroTimes validates rejection of zero timestamps.
func TestCreateManualEntryZeroTimes(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Now().UTC()
	// Zero start time
	entry := &TimeEntry{
		TaskName:  "Task",
		StartedAt: time.Time{},
		EndedAt:   &now,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error for zero start time")
	}

	// Zero end time
	entry = &TimeEntry{
		TaskName:  "Task",
		StartedAt: now,
		EndedAt:   &time.Time{},
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error for zero end time")
	}
}

// TestCreateManualEntryNegativePausedNS validates rejection of negative paused_ns.
func TestCreateManualEntryNegativePausedNS(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	entry := &TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedNS:  -100,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error for negative paused_ns")
	}
}

// TestCreateManualEntrySpanShorterThanPauses rejects when span < pauses.
func TestCreateManualEntrySpanShorterThanPauses(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Second) // 10 seconds span
	// 20 seconds of paused time
	entry := &TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedNS:  20_000_000_000, // 20 seconds
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error when span is shorter than pauses")
	}
}

// TestCreateManualEntryPausedAtValidation ensures paused_at is within bounds.
func TestCreateManualEntryPausedAtValidation(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	// PausedAt before start
	pausedBefore := start.Add(-time.Minute)
	entry := &TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedAt:  &pausedBefore,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error for paused_at before start")
	}

	// PausedAt after end
	pausedAfter := end.Add(time.Minute)
	entry = &TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedAt:  &pausedAfter,
	}
	_, err = repo.CreateManualEntry(entry)
	if err == nil {
		t.Fatal("expected error for paused_at after end")
	}
}

// TestUpdateCompletedEntryZeroTimes validates rejection of zero timestamps.
func TestUpdateCompletedEntryZeroTimes(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update with zero start time
	created.StartedAt = time.Time{}
	err = repo.UpdateCompletedEntry(created)
	if err == nil {
		t.Fatal("expected error for zero start time")
	}

	// Restore and try zero end time
	created.StartedAt = start
	created.EndedAt = &time.Time{}
	err = repo.UpdateCompletedEntry(created)
	if err == nil {
		t.Fatal("expected error for zero end time")
	}
}

// TestUpdateCompletedEntryProjectIDPersists verifies project_id is updated.
func TestUpdateCompletedEntryProjectIDPersists(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, err := repo.CreateCustomer("Customer")
	if err != nil {
		t.Fatal(err)
	}
	proj1, err := repo.CreateProject(cust.ID, "Project1", 100, 100, 10000, "#FF0000")
	if err != nil {
		t.Fatal(err)
	}
	proj2, err := repo.CreateProject(cust.ID, "Project2", 100, 100, 10000, "#00FF00")
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		ProjectID: &proj1.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update to different project
	created.ProjectID = &proj2.ID
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := repo.GetEntry(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.ProjectID == nil || *retrieved.ProjectID != proj2.ID {
		t.Fatalf("project_id not updated: got %v, want %v", retrieved.ProjectID, proj2.ID)
	}
}

// TestUpdateCompletedEntryProjectIDRemoved tests setting project_id to NULL.
func TestUpdateCompletedEntryProjectIDRemoved(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, err := repo.CreateCustomer("Customer")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := repo.CreateProject(cust.ID, "Project", 100, 100, 10000, "#FF0000")
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		ProjectID: &proj.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Remove project assignment
	created.ProjectID = nil
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := repo.GetEntry(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.ProjectID != nil {
		t.Fatalf("project_id not removed: got %v, want nil", retrieved.ProjectID)
	}
}

// TestUpdateCompletedEntryOldTerminalPauseAfterEndChange verifies old pause consolidation uses OLD endedAt.
func TestUpdateCompletedEntryOldTerminalPauseAfterEndChange(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Create entry: 9:00-10:00 with pause at 9:30
	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	pauseAt := start.Add(30 * time.Minute) // 9:30
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedAt:  &pauseAt, // Ongoing pause from 9:30 to 10:00 = 30 min
	})
	if err != nil {
		t.Fatal(err)
	}

	// Duration should be 30 min (60 min - 30 min pause)
	if created.DurationSec != 1800 {
		t.Fatalf("initial duration=%d, want 1800 (30 min)", created.DurationSec)
	}

	// Now extend end time: 9:00-11:00 (1 hour longer)
	// The old pause was from 9:30-10:00 = 30 min
	// Should NOT use the new span 10:00-11:00 as pause; should consolidate old 30 min
	newEnd := start.Add(2 * time.Hour)
	created.EndedAt = &newEnd
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := repo.GetEntry(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Duration should be 90 min (120 min total - 30 min consolidated pause)
	// NOT 60 min (120 min - 60 min new span)
	if retrieved.DurationSec != 5400 {
		t.Fatalf("after time extend, duration=%d, want 5400 (90 min), bug used NEW endedAt", retrieved.DurationSec)
	}
	if retrieved.PausedAt != nil {
		t.Fatalf("pausedAt should be nil after consolidation, got %v", retrieved.PausedAt)
	}
	if retrieved.PausedNS != 30*60*1_000_000_000 {
		t.Fatalf("pausedNS=%d, want %d (30 min consolidated)", retrieved.PausedNS, 30*60*1_000_000_000)
	}
}

// TestUpdateCompletedEntryUnchangedTimeBoundsPreservesDuration verifies same time bounds preserve DB duration.
func TestUpdateCompletedEntryUnchangedTimeBoundsPreservesDuration(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
		PausedNS:  15*60*1_000_000_000, // 15 min paused
	})
	if err != nil {
		t.Fatal(err)
	}

	originalDuration := created.DurationSec
	originalPausedNS := created.PausedNS

	// Edit task name only, keep time bounds same
	created.TaskName = "Updated Task"
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := repo.GetEntry(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Duration and pauses should be preserved even if malicious caller omits them
	if retrieved.DurationSec != originalDuration {
		t.Fatalf("duration changed on time-unchanged update: got %d, want %d", retrieved.DurationSec, originalDuration)
	}
	if retrieved.PausedNS != originalPausedNS {
		t.Fatalf("pausedNS changed on time-unchanged update: got %d, want %d", retrieved.PausedNS, originalPausedNS)
	}
}

// TestUpdateCompletedEntryMaliciousDurationSecIgnored verifies caller DurationSec is ignored on time change.
func TestUpdateCompletedEntryMaliciousDurationSecIgnored(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Extend time and inject malicious DurationSec
	newEnd := start.Add(2 * time.Hour)
	created.EndedAt = &newEnd
	created.DurationSec = 999999 // Malicious value
	err = repo.UpdateCompletedEntry(created)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := repo.GetEntry(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Duration should be derived from timestamps, not the malicious value
	// 2 hours = 7200 seconds
	if retrieved.DurationSec != 7200 {
		t.Fatalf("malicious DurationSec was used: got %d, want 7200", retrieved.DurationSec)
	}
}

// TestDeleteCompletedEntryProtectsLiveChildren prevents deletion of parents with live children.
func TestDeleteCompletedEntryProtectsLiveChildren(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Create parent entry
	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	parent, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Parent",
		StartedAt: start,
		EndedAt:   &end,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create live child via StartQuickShift
	_, err = repo.StartQuickShift(parent.ID, "ChildTask")
	if err != nil {
		t.Fatal(err)
	}

	// Try to delete parent with live child
	err = repo.DeleteCompletedEntry(parent.ID)
	if err == nil {
		t.Fatal("expected error deleting parent with live child")
	}

	// Verify parent still exists
	retrieved, err := repo.GetEntry(parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved == nil {
		t.Fatal("parent was deleted despite having live child")
	}
}

// TestDeleteCompletedEntryAllowsWhenNoLiveChildren allows deletion when no live children exist.
func TestDeleteCompletedEntryAllowsWhenNoLiveChildren(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Create parent without children
	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	parent, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Parent",
		StartedAt: start,
		EndedAt:   &end,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete parent should succeed
	err = repo.DeleteCompletedEntry(parent.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify parent deleted
	_, err = repo.GetEntry(parent.ID)
	if err == nil {
		t.Fatal("parent should be deleted")
	}
}

// TestUpdateCompletedEntryEmptyTaskRejected validates task name validation.
func TestUpdateCompletedEntryEmptyTaskRejected(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	created, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:  "Task",
		StartedAt: start,
		EndedAt:   &end,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Clear task name
	created.TaskName = ""
	err = repo.UpdateCompletedEntry(created)
	if err == nil {
		t.Fatal("expected error for empty task name")
	}

	// Whitespace only
	created.TaskName = "   "
	err = repo.UpdateCompletedEntry(created)
	if err == nil {
		t.Fatal("expected error for whitespace-only task name")
	}
}
