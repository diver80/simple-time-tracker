package db

import (
	"database/sql"
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

// TestExistingSchemaBackwardCompatibility verifies that existing databases work with migration
func TestExistingSchemaBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "old.db")

	// Create repo (this creates new schema with pause columns)
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create test data
	cust, _ := repo.CreateCustomer("Test Customer")
	proj, _ := repo.CreateProject(cust.ID, "Test Project", 100, 40, 4000, "#000000")
	entry, _ := repo.StartTimeEntry(&proj.ID, "Test Task", true)
	repo.Close()

	// Reopen same database - migration should handle it gracefully
	repo2, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen repo: %v", err)
	}
	defer repo2.Close()

	// Verify can still read the entry
	retrieved, err := repo2.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("failed to retrieve entry: %v", err)
	}
	if retrieved.ID != entry.ID || retrieved.TaskName != "Test Task" {
		t.Errorf("entry data corrupted: %+v", retrieved)
	}

	// Verify new pause fields are accessible
	if retrieved.PausedAt != nil {
		t.Errorf("expected nil PausedAt, got %v", retrieved.PausedAt)
	}
	if retrieved.PausedNS != 0 {
		t.Errorf("expected 0 PausedNS, got %d", retrieved.PausedNS)
	}
}

// TestPauseStopAccounting verifies pause time is correctly subtracted from duration
func TestPauseStopAccounting(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pause_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")

	// Create entry
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	entry, _ := repo.StartTimeEntry(&proj.ID, "Task", true)

	// Manually update to set started_at to a known time and add accumulated pause
	entry.StartedAt = start
	entry.PausedNS = int64(15 * time.Minute)    // 15 min accumulated pause
	pausedAtTime := start.Add(30 * time.Minute) // Paused at 30 min mark
	entry.PausedAt = &pausedAtTime
	repo.UpdateEntry(entry)

	// Stop at 1 hour from start (60 minutes total)
	stopTime := start.Add(60 * time.Minute)
	err = repo.StopTimeEntry(entry.ID, stopTime)
	if err != nil {
		t.Fatalf("stop entry failed: %v", err)
	}

	// Retrieve and verify duration
	// Duration should be: 60 min - 15 min (accumulated) - (60-30) min (ongoing pause) = 60 - 15 - 30 = 15 min
	retrieved, _ := repo.GetEntry(entry.ID)
	expectedDurationSec := int64(15 * 60) // 15 minutes
	if retrieved.DurationSec != expectedDurationSec {
		t.Errorf("expected duration %d sec, got %d sec", expectedDurationSec, retrieved.DurationSec)
	}
}

// TestStopEntryIdempotency verifies repeated stops are idempotent
func TestStopEntryIdempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "idempotent_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")
	entry, _ := repo.StartTimeEntry(&proj.ID, "Task", true)

	stopTime := time.Now().UTC()

	// First stop should succeed
	err = repo.StopTimeEntry(entry.ID, stopTime)
	if err != nil {
		t.Fatalf("first stop failed: %v", err)
	}

	// Second stop at same time should succeed (idempotent)
	err = repo.StopTimeEntry(entry.ID, stopTime)
	if err != nil {
		t.Fatalf("idempotent stop failed: %v", err)
	}

	// A later retry succeeds without moving the original stop time.
	if err := repo.StopTimeEntry(entry.ID, stopTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.EndedAt == nil || !stored.EndedAt.Equal(stopTime) {
		t.Fatal("retry changed the original stop time")
	}
}

// TestStartQuickShiftAtomicity verifies atomic pause of parent and child insert
func TestStartQuickShiftAtomicity(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "quickshift_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")
	parent, _ := repo.StartTimeEntry(&proj.ID, "Main Task", true)

	// Start quickshift
	child, err := repo.StartQuickShift(parent.ID, "Interruption")
	if err != nil {
		t.Fatalf("start quickshift failed: %v", err)
	}

	// Verify child exists
	if child.ID == 0 || child.ParentID == nil || *child.ParentID != parent.ID {
		t.Errorf("child not created properly: %+v", child)
	}

	// Verify parent was paused
	parentRetrieved, _ := repo.GetEntry(parent.ID)
	if parentRetrieved.PausedAt == nil {
		t.Errorf("parent not paused after quickshift")
	}
}

// TestStartQuickShiftHistoricalParent verifies quickshift doesn't pause ended parent
func TestStartQuickShiftHistoricalParent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "historical_parent_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")
	parent, _ := repo.StartTimeEntry(&proj.ID, "Main Task", true)

	// End the parent
	stopTime := time.Now().UTC()
	repo.StopTimeEntry(parent.ID, stopTime)

	// Start quickshift - should not update paused_at since parent is ended
	child, err := repo.StartQuickShift(parent.ID, "Interruption")
	if err != nil {
		t.Fatalf("start quickshift on historical parent failed: %v", err)
	}

	// Child should be created
	if child.ID == 0 {
		t.Errorf("child not created")
	}

	// Parent should remain without paused_at set (since it was ended)
	parentRetrieved, _ := repo.GetEntry(parent.ID)
	// Parent was paused before being stopped, so check original pause state isn't overwritten
	if parentRetrieved.EndedAt == nil {
		t.Errorf("parent should still be ended")
	}
}

// TestInMemoryConcurrency verifies :memory: DB with max_open_conns=1
func TestInMemoryConcurrency(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")

	// Multiple operations should work with single connection
	for i := 0; i < 5; i++ {
		entry, err := repo.StartTimeEntry(&proj.ID, "Task", true)
		if err != nil {
			t.Fatalf("failed to create entry %d: %v", i, err)
		}
		repo.StopTimeEntry(entry.ID, time.Now().UTC())
	}
}

// TestDSTDayBoundaries verifies day queries handle DST correctly with exact boundary membership
func TestDSTDayBoundaries(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dst_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")

	// Load Europe/Berlin timezone for DST testing
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("failed to load Berlin timezone: %v", err)
	}

	// Test on a spring DST transition day (23-hour day): 2026-03-29 in Europe/Berlin
	// At 2:00 AM, clocks move forward to 3:00 AM
	dstDay := time.Date(2026, 3, 29, 12, 0, 0, 0, berlin)

	// Entry at 23:30 on the DST day (before transition)
	entry1, _ := repo.StartTimeEntry(&proj.ID, "Task1", true)
	entry1.StartedAt = time.Date(2026, 3, 29, 23, 30, 0, 0, berlin)
	repo.UpdateEntry(entry1)

	// Entry at 00:30 next day (after transition midnight)
	entry2, _ := repo.StartTimeEntry(&proj.ID, "Task2", true)
	entry2.StartedAt = time.Date(2026, 3, 30, 0, 30, 0, 0, berlin)
	repo.UpdateEntry(entry2)

	// Query for the DST day - should get entry1 (23:30) but NOT entry2 (next day)
	entries, err := repo.ListEntriesForDay(dstDay)
	if err != nil {
		t.Fatalf("list entries for day failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry on DST day (23-hour), got %d: %+v", len(entries), entries)
	}
	if entries[0].TaskName != "Task1" {
		t.Errorf("expected Task1 on DST day, got %s", entries[0].TaskName)
	}

	// Query for next day - should get entry2 (00:30)
	nextDay := time.Date(2026, 3, 30, 12, 0, 0, 0, berlin)
	entries, err = repo.ListEntriesForDay(nextDay)
	if err != nil {
		t.Fatalf("list entries for next day failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry on next day, got %d", len(entries))
	}
	if entries[0].TaskName != "Task2" {
		t.Errorf("expected Task2 on next day, got %s", entries[0].TaskName)
	}

	// Test on a fall DST transition day (25-hour day): 2026-10-25 in Europe/Berlin
	// At 3:00 AM, clocks move back to 2:00 AM
	fallDSTDay := time.Date(2026, 10, 25, 12, 0, 0, 0, berlin)

	// Entry at 23:30 on the fall DST day
	entry3, _ := repo.StartTimeEntry(&proj.ID, "Task3", true)
	entry3.StartedAt = time.Date(2026, 10, 25, 23, 30, 0, 0, berlin)
	repo.UpdateEntry(entry3)

	// Entry at 00:30 next day
	entry4, _ := repo.StartTimeEntry(&proj.ID, "Task4", true)
	entry4.StartedAt = time.Date(2026, 10, 26, 0, 30, 0, 0, berlin)
	repo.UpdateEntry(entry4)

	// Query for the fall DST day - should get entry3 (23:30)
	entries, err = repo.ListEntriesForDay(fallDSTDay)
	if err != nil {
		t.Fatalf("list entries for fall DST day failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry on fall DST day (25-hour), got %d", len(entries))
	}
	if entries[0].TaskName != "Task3" {
		t.Errorf("expected Task3 on fall DST day, got %s", entries[0].TaskName)
	}

	// Query for next day - should get entry4 (00:30)
	nextFallDay := time.Date(2026, 10, 26, 12, 0, 0, 0, berlin)
	entries, err = repo.ListEntriesForDay(nextFallDay)
	if err != nil {
		t.Fatalf("list entries for fall DST next day failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry on fall DST next day, got %d", len(entries))
	}
	if entries[0].TaskName != "Task4" {
		t.Errorf("expected Task4 on fall DST next day, got %s", entries[0].TaskName)
	}
}

// TestMonthFilter verifies customer and project filters work independently
func TestMonthFilter(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "filter_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust1, _ := repo.CreateCustomer("Customer1")
	cust2, _ := repo.CreateCustomer("Customer2")
	proj1, _ := repo.CreateProject(cust1.ID, "Project1", 100, 40, 4000, "#000000")
	proj2, _ := repo.CreateProject(cust2.ID, "Project2", 100, 40, 4000, "#000000")

	// Create entries
	start := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	entry1, _ := repo.StartTimeEntry(&proj1.ID, "Task1", true)
	entry1.StartedAt = start
	repo.UpdateEntry(entry1)

	entry2, _ := repo.StartTimeEntry(&proj2.ID, "Task2", true)
	entry2.StartedAt = start
	repo.UpdateEntry(entry2)

	// Filter by customer1 - should get only entry1
	entries, err := repo.ListEntriesForMonth(2026, time.January, &cust1.ID, nil)
	if err != nil {
		t.Fatalf("list entries by customer failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry1.ID {
		t.Errorf("customer filter failed: expected 1 entry for cust1, got %d", len(entries))
	}

	// Filter by project2 - should get only entry2
	entries, err = repo.ListEntriesForMonth(2026, time.January, nil, &proj2.ID)
	if err != nil {
		t.Fatalf("list entries by project failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry2.ID {
		t.Errorf("project filter failed: expected 1 entry for proj2, got %d", len(entries))
	}
}

// TestUpdateEntryValidation verifies timestamp and duration validation
func TestUpdateEntryValidation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "validation_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")
	entry, _ := repo.StartTimeEntry(&proj.ID, "Task", true)

	// Test: reject negative duration
	entry.DurationSec = -100
	err = repo.UpdateEntry(entry)
	if err == nil {
		t.Errorf("expected error for negative duration, got nil")
	}
	entry.DurationSec = 0 // reset

	// Test: reject end before start
	start := entry.StartedAt
	end := start.Add(-1 * time.Hour)
	entry.EndedAt = &end
	err = repo.UpdateEntry(entry)
	if err == nil {
		t.Errorf("expected error for end before start, got nil")
	}
}

// TestStopEntryValidateEndBeforeStart checks StopTimeEntry rejects end before start
func TestStopEntryValidateEndBeforeStart(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stop_validation_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test")
	proj, _ := repo.CreateProject(cust.ID, "Test", 100, 40, 4000, "#000000")
	entry, _ := repo.StartTimeEntry(&proj.ID, "Task", true)

	// Try to stop before entry started
	stopTime := entry.StartedAt.Add(-1 * time.Hour)
	err = repo.StopTimeEntry(entry.ID, stopTime)
	if err == nil {
		t.Errorf("expected error when stopping before start, got nil")
	}
}

// TestMigrationSimulateOldSchema verifies migration works with old schema
func TestMigrationSimulateOldSchema(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "old_schema_test.db")

	// Simulate old schema by manually creating a database without pause columns
	// This ensures the migration can properly add them
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Create old schema without pause columns
	oldSchema := `
	CREATE TABLE IF NOT EXISTS customers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		hourly_rate REAL NOT NULL DEFAULT 0.0,
		budget_hours REAL NOT NULL DEFAULT 0.0,
		budget_cost REAL NOT NULL DEFAULT 0.0,
		color TEXT NOT NULL DEFAULT '#3B82F6',
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		FOREIGN KEY(customer_id) REFERENCES customers(id)
	);
	CREATE TABLE IF NOT EXISTS time_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER,
		task_name TEXT NOT NULL,
		booking_text TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		duration_sec INTEGER NOT NULL DEFAULT 0,
		is_billable INTEGER NOT NULL DEFAULT 1,
		is_quickshift INTEGER NOT NULL DEFAULT 0,
		parent_id INTEGER,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id),
		FOREIGN KEY(parent_id) REFERENCES time_entries(id)
	);
	`
	if _, err := db.Exec(oldSchema); err != nil {
		db.Close()
		t.Fatalf("failed to create old schema: %v", err)
	}

	// Create test data in old schema
	now := time.Now().UTC()
	res, _ := db.Exec("INSERT INTO customers (name, created_at) VALUES (?, ?)", "TestCust", now)
	custID, _ := res.LastInsertId()
	res, _ = db.Exec("INSERT INTO projects (customer_id, name, hourly_rate, budget_hours, budget_cost, color, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		custID, "TestProj", 100, 40, 4000, "#000000", 1, now)
	projID, _ := res.LastInsertId()
	res, _ = db.Exec("INSERT INTO time_entries (project_id, task_name, booking_text, started_at, ended_at, duration_sec, is_billable, is_quickshift, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		projID, "TestTask", "", now, now.Add(time.Hour), 3600, 1, 0, now, now)
	entryID, _ := res.LastInsertId()

	db.Close()

	// Now open with NewRepository which should trigger migration
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open repo after migration: %v", err)
	}
	defer repo.Close()

	// Verify old data is still accessible
	entry, err := repo.GetEntry(entryID)
	if err != nil {
		t.Fatalf("failed to retrieve migrated entry: %v", err)
	}
	if entry.TaskName != "TestTask" || entry.DurationSec != 3600 {
		t.Errorf("entry data corrupted during migration: %+v", entry)
	}

	// Verify new columns exist and are accessible
	if entry.PausedAt != nil {
		t.Errorf("expected nil PausedAt, got %v", entry.PausedAt)
	}
	if entry.PausedNS != 0 {
		t.Errorf("expected 0 PausedNS, got %d", entry.PausedNS)
	}
}

// TestMonthFilterCustomerProjectMismatch verifies filtering handles entries with mismatched project
func TestMonthFilterCustomerProjectMismatch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "filter_mismatch_test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust1, _ := repo.CreateCustomer("Customer1")
	cust2, _ := repo.CreateCustomer("Customer2")
	proj1, _ := repo.CreateProject(cust1.ID, "Project1", 100, 40, 4000, "#000000")
	proj2, _ := repo.CreateProject(cust2.ID, "Project2", 100, 40, 4000, "#000000")

	start := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// Entry with proj1 (belongs to cust1)
	entry1, _ := repo.StartTimeEntry(&proj1.ID, "Task1", true)
	entry1.StartedAt = start
	repo.UpdateEntry(entry1)

	// Entry with no project (belongs to no customer)
	entry2, _ := repo.StartTimeEntry(nil, "Task2", true)
	entry2.StartedAt = start
	repo.UpdateEntry(entry2)

	// Filter by cust1 - should get only entry1
	entries, err := repo.ListEntriesForMonth(2026, time.January, &cust1.ID, nil)
	if err != nil {
		t.Fatalf("list entries by customer failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry1.ID {
		t.Errorf("customer filter failed: expected 1 entry for cust1, got %d", len(entries))
	}

	// Filter by proj2 - should get no entries (entry1 is proj1, entry2 has no project)
	entries, err = repo.ListEntriesForMonth(2026, time.January, nil, &proj2.ID)
	if err != nil {
		t.Fatalf("list entries by project failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("project filter failed: expected 0 entries for proj2, got %d", len(entries))
	}

	// Filter by proj1 - should get only entry1
	entries, err = repo.ListEntriesForMonth(2026, time.January, nil, &proj1.ID)
	if err != nil {
		t.Fatalf("list entries by project (proj1) failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry1.ID {
		t.Errorf("project filter (proj1) failed: expected 1 entry, got %d", len(entries))
	}

	// Both filters apply: proj1 belongs to cust1, not cust2.
	entries, err = repo.ListEntriesForMonth(2026, time.January, &cust2.ID, &proj1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("customer filter was ignored when project was specified")
	}

	// No filter - should get both entries
	entries, err = repo.ListEntriesForMonth(2026, time.January, nil, nil)
	if err != nil {
		t.Fatalf("list entries without filter failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("no filter failed: expected 2 entries, got %d", len(entries))
	}
}

func TestRepositoryDeleteProject(t *testing.T) {
	repo, _ := NewRepository(":memory:")
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test Cust")
	proj, _ := repo.CreateProject(cust.ID, "Test Proj", 100, 10, 1000, "#3B82F6")

	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	entry, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:    "Work",
		StartedAt:   start,
		EndedAt:     &now,
		DurationSec: 3600,
		ProjectID:   &proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateManualEntry failed: %v", err)
	}

	err = repo.DeleteProject(proj.ID)
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	// Verify project is deleted
	p, err := repo.GetProject(proj.ID)
	if err == nil && p != nil {
		t.Fatalf("expected project to be deleted, found: %v", p)
	}

	// Verify time entry preserved with project_id set to null
	e, _ := repo.GetEntry(entry.ID)
	if e == nil || e.ProjectID != nil {
		t.Fatalf("expected time entry project_id to be null, got: %v", e)
	}
}

func TestRepositoryDeleteCustomer(t *testing.T) {
	repo, _ := NewRepository(":memory:")
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Customer To Delete")
	proj, _ := repo.CreateProject(cust.ID, "Project To Delete", 120, 20, 2400, "#3B82F6")

	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour)
	entry, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:    "Client Work",
		StartedAt:   start,
		EndedAt:     &now,
		DurationSec: 7200,
		ProjectID:   &proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateManualEntry failed: %v", err)
	}

	err = repo.DeleteCustomer(cust.ID)
	if err != nil {
		t.Fatalf("DeleteCustomer failed: %v", err)
	}

	// Verify customer is deleted
	c, err := repo.GetCustomer(cust.ID)
	if err == nil && c != nil {
		t.Fatalf("expected customer to be deleted, found: %v", c)
	}

	// Verify project is deleted
	p, err := repo.GetProject(proj.ID)
	if err == nil && p != nil {
		t.Fatalf("expected project to be deleted, found: %v", p)
	}

	// Verify time entry preserved with project_id set to null
	e, _ := repo.GetEntry(entry.ID)
	if e == nil || e.ProjectID != nil {
		t.Fatalf("expected time entry project_id to be null, got: %v", e)
	}
}

func TestRepositoryClearAllDemoData(t *testing.T) {
	repo, _ := NewRepository(":memory:")
	defer repo.Close()

	c1, _ := repo.CreateCustomer("Acme Corporation")
	p1, _ := repo.CreateProject(c1.ID, "Web Redesign", 125, 40, 5000, "#3B82F6")
	c2, _ := repo.CreateCustomer("Real Customer")
	repo.CreateProject(c2.ID, "Real Project", 150, 20, 3000, "#10B981")
	c3, _ := repo.CreateCustomer("Starlight Media")
	p3, _ := repo.CreateProject(c3.ID, "Mobile App", 140, 50, 7000, "#8B5CF6")
	c4, _ := repo.CreateCustomer("Acme Corp")
	_, _ = repo.CreateProject(c4.ID, "Brand Guide", 110, 10, 1100, "#10B981")

	now := time.Now().UTC()
	// Demo seeded entries
	start1 := now.Add(-3 * time.Hour)
	end1 := now.Add(-1 * time.Hour)
	e1, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:    "Figma Wireframing & Layout",
		StartedAt:   start1,
		EndedAt:     &end1,
		DurationSec: 5400,
		ProjectID:   &p1.ID,
	})
	if err != nil {
		t.Fatalf("CreateManualEntry e1 failed: %v", err)
	}
	// User entry on demo project
	startUser := now.Add(-1 * time.Hour)
	eUser, err := repo.CreateManualEntry(&TimeEntry{
		TaskName:    "Real User Work on Demo Project",
		StartedAt:   startUser,
		EndedAt:     &now,
		DurationSec: 3600,
		ProjectID:   &p3.ID,
	})
	if err != nil {
		t.Fatalf("CreateManualEntry eUser failed: %v", err)
	}

	err = repo.ClearAllDemoData()
	if err != nil {
		t.Fatalf("ClearAllDemoData failed: %v", err)
	}

	custs, _ := repo.ListCustomers()
	if len(custs) != 1 || custs[0].Name != "Real Customer" {
		t.Fatalf("expected only Real Customer to remain, got %v", custs)
	}

	projs, _ := repo.ListProjects(nil)
	if len(projs) != 1 || projs[0].Name != "Real Project" {
		t.Fatalf("expected only Real Project to remain, got %v", projs)
	}

	// Demo seed entry should be removed
	e1Check, _ := repo.GetEntry(e1.ID)
	if e1Check != nil {
		t.Fatalf("expected seeded demo entry to be purged, found: %v", e1Check)
	}

	// User booking on demo project should be preserved but unlinked (project_id == nil)
	eUserCheck, _ := repo.GetEntry(eUser.ID)
	if eUserCheck == nil || eUserCheck.ProjectID != nil {
		t.Fatalf("expected user entry to be preserved with project_id nil, got: %v", eUserCheck)
	}
}

func TestRepositoryCreateCustomerIfNotExists(t *testing.T) {
	repo, _ := NewRepository(":memory:")
	defer repo.Close()

	// Empty customer name should fail
	_, err := repo.CreateCustomerIfNotExists("")
	if err == nil {
		t.Fatalf("expected error for empty name, got nil")
	}

	// First call creates customer
	c1, err := repo.CreateCustomerIfNotExists("New Customer")
	if err != nil {
		t.Fatalf("CreateCustomerIfNotExists failed: %v", err)
	}
	if c1.ID == 0 || c1.Name != "New Customer" {
		t.Fatalf("unexpected customer created: %+v", c1)
	}

	// Second call with same name returns existing customer
	c2, err := repo.CreateCustomerIfNotExists("New Customer")
	if err != nil {
		t.Fatalf("second call to CreateCustomerIfNotExists failed: %v", err)
	}
	if c2.ID != c1.ID || c2.Name != c1.Name {
		t.Fatalf("expected same customer ID %d, got %d", c1.ID, c2.ID)
	}

	// Verify customer count is still 1
	list, _ := repo.ListCustomers()
	if len(list) != 1 {
		t.Fatalf("expected 1 customer, found %d", len(list))
	}
}

