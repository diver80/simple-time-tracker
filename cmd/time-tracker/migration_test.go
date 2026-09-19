package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyDB_FromTimeTracker(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "simple-time-tracker", "data.db")
	timeTrackerLegacy := filepath.Join(tempDir, "time-tracker", "data.db")
	yoktoLegacy := filepath.Join(tempDir, "yokto-time", "data.db")

	_ = os.MkdirAll(filepath.Dir(timeTrackerLegacy), 0755)
	_ = os.WriteFile(timeTrackerLegacy, []byte("time-tracker-database-content"), 0644)

	migrated := migrateLegacyDB(destPath, []string{timeTrackerLegacy, yoktoLegacy})
	if !migrated {
		t.Fatal("expected migrateLegacyDB to return true when migrating from time-tracker")
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read migrated database: %v", err)
	}
	if string(content) != "time-tracker-database-content" {
		t.Fatalf("unexpected content in migrated database: %s", string(content))
	}
}

func TestMigrateLegacyDB_FromYoktoTimeFallback(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "simple-time-tracker", "data.db")
	timeTrackerLegacy := filepath.Join(tempDir, "time-tracker", "data.db")
	yoktoLegacy := filepath.Join(tempDir, "yokto-time", "data.db")

	_ = os.MkdirAll(filepath.Dir(yoktoLegacy), 0755)
	_ = os.WriteFile(yoktoLegacy, []byte("yokto-database-content"), 0644)

	migrated := migrateLegacyDB(destPath, []string{timeTrackerLegacy, yoktoLegacy})
	if !migrated {
		t.Fatal("expected migrateLegacyDB to return true when falling back to yokto-time")
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read migrated database: %v", err)
	}
	if string(content) != "yokto-database-content" {
		t.Fatalf("unexpected content in migrated database: %s", string(content))
	}
}

func TestMigrateLegacyDB_NoOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "simple-time-tracker", "data.db")
	timeTrackerLegacy := filepath.Join(tempDir, "time-tracker", "data.db")

	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	_ = os.WriteFile(destPath, []byte("existing-simple-time-tracker-data"), 0644)

	_ = os.MkdirAll(filepath.Dir(timeTrackerLegacy), 0755)
	_ = os.WriteFile(timeTrackerLegacy, []byte("time-tracker-database-content"), 0644)

	migrated := migrateLegacyDB(destPath, []string{timeTrackerLegacy})
	if migrated {
		t.Fatal("expected migrateLegacyDB to return false when destination already exists")
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read database: %v", err)
	}
	if string(content) != "existing-simple-time-tracker-data" {
		t.Fatalf("destination data was overwritten: %s", string(content))
	}
}

func TestMigrateLegacyDB_NoLegacyFiles(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "simple-time-tracker", "data.db")
	timeTrackerLegacy := filepath.Join(tempDir, "time-tracker", "data.db")

	migrated := migrateLegacyDB(destPath, []string{timeTrackerLegacy})
	if migrated {
		t.Fatal("expected migrateLegacyDB to return false when legacy files do not exist")
	}
}

func TestDefaultDBPath_UsesSimpleTimeTracker(t *testing.T) {
	tempDir := t.TempDir()
	path := resolveDefaultDBPath(tempDir)
	if !strings.Contains(path, filepath.Join("simple-time-tracker", "data.db")) {
		t.Fatalf("expected resolveDefaultDBPath to point to simple-time-tracker/data.db, got: %s", path)
	}
}

func TestMigrateLegacyDB_WithWALAndSHM(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "simple-time-tracker", "data.db")
	timeTrackerLegacy := filepath.Join(tempDir, "time-tracker", "data.db")

	_ = os.MkdirAll(filepath.Dir(timeTrackerLegacy), 0755)
	_ = os.WriteFile(timeTrackerLegacy, []byte("main-db-content"), 0644)
	_ = os.WriteFile(timeTrackerLegacy+"-wal", []byte("wal-journal-content"), 0644)
	_ = os.WriteFile(timeTrackerLegacy+"-shm", []byte("shm-index-content"), 0644)

	migrated := migrateLegacyDB(destPath, []string{timeTrackerLegacy})
	if !migrated {
		t.Fatal("expected migrateLegacyDB to return true when migrating with WAL and SHM")
	}

	content, err := os.ReadFile(destPath)
	if err != nil || string(content) != "main-db-content" {
		t.Fatalf("unexpected main db content: %v, string: %s", err, string(content))
	}

	walContent, err := os.ReadFile(destPath + "-wal")
	if err != nil || string(walContent) != "wal-journal-content" {
		t.Fatalf("unexpected wal content: %v, string: %s", err, string(walContent))
	}

	shmContent, err := os.ReadFile(destPath + "-shm")
	if err != nil || string(shmContent) != "shm-index-content" {
		t.Fatalf("unexpected shm content: %v, string: %s", err, string(shmContent))
	}
}

