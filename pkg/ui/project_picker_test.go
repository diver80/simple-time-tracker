package ui

import (
	"testing"
	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
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

func TestProjectPickerLayoutAndSelection(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	cust, err := repo.CreateCustomer("Acme Corp")
	if err != nil {
		t.Fatalf("failed to create customer: %v", err)
	}
	p1, err := repo.CreateProject(cust.ID, "Alpha", 100.0, 10.0, 1000.0, "#3B82F6")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	var changedProj *db.Project
	picker := NewProjectPicker(repo, func(p *db.Project) {
		changedProj = p
	})
	picker.SetBounds(geometry.NewRect(0, 0, 300, 28))

	// Initial closed state: 28px height
	szClosed := picker.Layout(&testWidgetContext{}, geometry.Tight(geometry.Sz(300, 28)))
	if szClosed.Height != 28 {
		t.Errorf("expected closed height 28, got %v", szClosed.Height)
	}

	// Expand picker: 1 project + 1 "(Kein Projekt)" row = 2 rows -> 28 + 2*24 = 76px
	picker.isExpanded = true
	szExpanded := picker.Layout(&testWidgetContext{}, geometry.Loose(geometry.Sz(300, 500)))
	if szExpanded.Height != float32(28+2*24) {
		t.Errorf("expected expanded height %v, got %v", 28+2*24, szExpanded.Height)
	}

	// Click row 1 (Alpha): should select project
	evClickProject := &event.MouseEvent{
		Base:      event.NewBase(event.TypeMouse, event.ModNone),
		MouseType: event.MousePress,
		Position:  geometry.Pt(50, 30+24+5), // row 1
	}
	if !picker.Event(&testWidgetContext{}, evClickProject) {
		t.Error("expected click on project to be handled")
	}
	if picker.SelectedProject() == nil || picker.SelectedProject().ID != p1.ID {
		t.Errorf("expected project %d selected, got %v", p1.ID, picker.SelectedProject())
	}
	if changedProj == nil || changedProj.ID != p1.ID {
		t.Errorf("expected onChange to receive project %d, got %v", p1.ID, changedProj)
	}

	// Re-expand and click row 0: "(Kein Projekt)"
	picker.isExpanded = true
	evClickNone := &event.MouseEvent{
		Base:      event.NewBase(event.TypeMouse, event.ModNone),
		MouseType: event.MousePress,
		Position:  geometry.Pt(50, 30+5), // row 0: (Kein Projekt)
	}
	if !picker.Event(&testWidgetContext{}, evClickNone) {
		t.Error("expected click on Kein Projekt to be handled")
	}
	if picker.SelectedProject() != nil {
		t.Errorf("expected project to be cleared (nil), got %v", picker.SelectedProject())
	}
	if changedProj != nil {
		t.Errorf("expected onChange to receive nil, got %v", changedProj)
	}
}
