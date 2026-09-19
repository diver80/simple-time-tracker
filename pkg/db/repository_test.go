package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	defer repo.Close()

	// 1. Create Customer & Project
	cust, err := repo.CreateCustomer("Acme Corp")
	if err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	if cust.ID == 0 || cust.Name != "Acme Corp" {
		t.Fatalf("unexpected customer: %+v", cust)
	}

	proj, err := repo.CreateProject(cust.ID, "Brand Identity", 120.0, 40.0, 4800.0, "#4F46E5")
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	if proj.ID == 0 || proj.Name != "Brand Identity" {
		t.Fatalf("unexpected project: %+v", proj)
	}

	// 2. Start Entry
	entry, err := repo.StartTimeEntry(&proj.ID, "Logo Redesign", true)
	if err != nil {
		t.Fatalf("start entry failed: %v", err)
	}
	if entry.TaskName != "Logo Redesign" {
		t.Errorf("expected Logo Redesign, got %s", entry.TaskName)
	}

	// Verify Active Entry
	active, err := repo.GetActiveEntry()
	if err != nil || active == nil || active.ID != entry.ID {
		t.Fatalf("expected active entry %d, got %+v (err: %v)", entry.ID, active, err)
	}

	// 3. Update live booking text
	err = repo.UpdateActiveBookingText(entry.ID, "- Initial moodboard sketches\n- Color palette selection")
	if err != nil {
		t.Fatalf("update booking text failed: %v", err)
	}

	// 4. QuickShift Interruption
	qs, err := repo.StartQuickShift(entry.ID, "Urgent Call with Client")
	if err != nil {
		t.Fatalf("start quickshift failed: %v", err)
	}
	if !qs.IsQuickShift || qs.ParentID == nil || *qs.ParentID != entry.ID {
		t.Fatalf("expected quickshift child linked to parent, got %+v", qs)
	}

	// 5. Stop Quickshift
	now := time.Now().UTC()
	err = repo.StopTimeEntry(qs.ID, now)
	if err != nil {
		t.Fatalf("stop quickshift failed: %v", err)
	}

	// 6. Stop Main Entry
	err = repo.StopTimeEntry(entry.ID, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("stop main entry failed: %v", err)
	}

	// 7. Verify Day Listing
	entries, err := repo.ListEntriesForDay(now)
	if err != nil {
		t.Fatalf("list entries for day failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for day, got %d", len(entries))
	}

	// 8. Test Budget Summary
	hours, cost, err := repo.GetProjectBudgetSummary(proj.ID)
	if err != nil {
		t.Fatalf("budget summary failed: %v", err)
	}
	if hours <= 0 || cost <= 0 {
		t.Errorf("expected positive hours and cost, got hours=%f, cost=%f", hours, cost)
	}
}
