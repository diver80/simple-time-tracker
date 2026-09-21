package ui

import (
	"testing"
	"time-tracker/pkg/db"
)

func TestFormatProjectLabel(t *testing.T) {
	// 1. Nil project -> "(Kein Projekt)"
	if got := formatProjectLabel(nil); got != "(Kein Projekt)" {
		t.Errorf("expected '(Kein Projekt)', got %q", got)
	}

	// 2. Project with CustomerName -> "Customer - Project"
	p1 := &db.Project{
		ID:           1,
		CustomerName: "Acme Corporation",
		Name:         "Web Platform Redesign",
	}
	if got := formatProjectLabel(p1); got != "Acme Corporation - Web Platform Redesign" {
		t.Errorf("expected 'Acme Corporation - Web Platform Redesign', got %q", got)
	}

	// 3. Project without CustomerName -> fallback to Project Name
	p2 := &db.Project{
		ID:   2,
		Name: "Internal Work",
	}
	if got := formatProjectLabel(p2); got != "Internal Work" {
		t.Errorf("expected 'Internal Work', got %q", got)
	}
}
