package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpgradeRestoresLegacyQuickShiftPause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	oldSchema := strings.ReplaceAll(SchemaSQL, "    paused_at DATETIME,\n", "")
	oldSchema = strings.ReplaceAll(oldSchema, "    paused_ns INTEGER NOT NULL DEFAULT 0,\n", "")
	if strings.Contains(oldSchema, "paused_") {
		t.Fatal("legacy fixture contains pause fields")
	}
	if _, err := conn.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	insert := `INSERT INTO time_entries (id, task_name, started_at, is_quickshift, parent_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := conn.Exec(insert, 1, "Main", start, 0, nil, start, start); err != nil {
		t.Fatal(err)
	}
	childStart := start.Add(10 * time.Minute)
	if _, err := conn.Exec(insert, 2, "Call", childStart, 1, 1, childStart, childStart); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	parent, err := repo.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if parent.PausedAt == nil || !parent.PausedAt.Equal(childStart) {
		t.Fatalf("pause not restored: %+v", parent)
	}
	if err := repo.StopTimeEntry(2, start.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.StopTimeEntry(1, start.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	parent, err = repo.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if parent.DurationSec != 900 {
		t.Fatalf("duration=%d, want 900", parent.DurationSec)
	}
}

func TestMonthUsesLocalCalendarBoundaries(t *testing.T) {
	// These tests are deliberately not parallel: time.Local is process-global.
	original := time.Local
	t.Cleanup(func() { time.Local = original })
	for _, offset := range []int{2 * 3600, -7 * 3600} {
		time.Local = time.FixedZone("test-local", offset)
		repo, err := NewRepository(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)
		var wanted []int64
		for i, instant := range []time.Time{start.Add(-time.Nanosecond), start, end.Add(-time.Nanosecond), end} {
			entry, err := repo.StartTimeEntry(nil, "Boundary", false)
			if err != nil {
				t.Fatal(err)
			}
			entry.StartedAt = instant
			if err := repo.UpdateEntry(entry); err != nil {
				t.Fatal(err)
			}
			if i == 1 || i == 2 {
				wanted = append(wanted, entry.ID)
			}
		}
		entries, err := repo.ListEntriesForMonth(2026, time.September, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 || entries[0].ID != wanted[0] || entries[1].ID != wanted[1] {
			t.Fatalf("offset=%d: incorrect month boundaries: %+v", offset, entries)
		}
		if err := repo.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStopBeforePauseDoesNotInflateDuration(t *testing.T) {
	repo, err := NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	entry, err := repo.StartTimeEntry(nil, "Task", true)
	if err != nil {
		t.Fatal(err)
	}
	pause := entry.StartedAt.Add(time.Hour)
	entry.PausedAt = &pause
	if err := repo.UpdateEntry(entry); err != nil {
		t.Fatal(err)
	}
	if err := repo.StopTimeEntry(entry.ID, entry.StartedAt.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetEntry(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DurationSec != 600 {
		t.Fatalf("duration=%d, want 600", stored.DurationSec)
	}
}
