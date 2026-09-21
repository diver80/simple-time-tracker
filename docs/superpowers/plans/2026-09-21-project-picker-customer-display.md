# ProjectPicker "Kunde - Projekt" Display & Deselection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update `ProjectPicker` to display `"<Kunde> - <Projekt>"` in both the combobox button and the dropdown list, and include `"(Kein Projekt)"` as the first selectable item to allow clearing project assignments.

**Architecture:** Add `formatProjectLabel(*db.Project) string` in `pkg/ui/project_picker.go`. Update `Draw()`, `Layout()`, and `Event()` in `ProjectPicker` to render and handle `1 + len(projects)` rows with Index 0 being `(Kein Projekt)`. Verify with unit tests and release v1.0.6.

**Tech Stack:** Go 1.27, Gogpu UI, SQLite (`modernc.org/sqlite`).

## Global Constraints
- Display name: `Simple Time Tracker`
- Binary: `stt`
- Bundle ID: `com.avono.simple-time-tracker`
- Version: `1.0.6`
- Homebrew Tap: `diver80/homebrew-tap`
- Target Files: `pkg/ui/project_picker.go`, `pkg/ui/project_picker_test.go`, `cmd/time-tracker/main.go`, `build.sh`

---

### Task 1: Label Formatting Helper & Unit Tests

**Files:**
- Create: `pkg/ui/project_picker_test.go`
- Modify: `pkg/ui/project_picker.go:1-35`

**Interfaces:**
- Produces: `formatProjectLabel(p *db.Project) string`

- [ ] **Step 1: Write failing test for `formatProjectLabel`**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test time-tracker/pkg/ui -run TestFormatProjectLabel`
Expected: FAIL with undefined `formatProjectLabel`.

- [ ] **Step 3: Implement `formatProjectLabel` in `pkg/ui/project_picker.go`**

```go
func formatProjectLabel(p *db.Project) string {
	if p == nil {
		return "(Kein Projekt)"
	}
	if p.CustomerName != "" {
		return fmt.Sprintf("%s - %s", p.CustomerName, p.Name)
	}
	return p.Name
}
```
*(Ensure `"fmt"` is included in imports in `pkg/ui/project_picker.go`)*

- [ ] **Step 4: Run test to verify it passes**

Run: `go test time-tracker/pkg/ui -run TestFormatProjectLabel`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/project_picker.go pkg/ui/project_picker_test.go
git commit -m "feat(ui): add formatProjectLabel supporting Customer - Project display"
```

---

### Task 2: ProjectPicker Dropdown & Deselection Support

**Files:**
- Modify: `pkg/ui/project_picker.go:65-140`
- Test: `pkg/ui/project_picker_test.go`

**Interfaces:**
- Consumes: `formatProjectLabel(p *db.Project) string`
- Produces: Updated `Draw()`, `Layout()`, and `Event()` in `ProjectPicker` supporting `(Kein Projekt)` at index 0 and formatted project rows.

- [ ] **Step 1: Write failing tests for dropdown rendering, layout height, and selection/deselection**

In `pkg/ui/project_picker_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test time-tracker/pkg/ui -run TestProjectPickerLayoutAndSelection`
Expected: FAIL.

- [ ] **Step 3: Update `Layout()`, `Draw()`, and `Event()` in `pkg/ui/project_picker.go`**

```go
func (pp *ProjectPicker) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(200)
	h := float32(28)
	if pp.isExpanded {
		// 1 row for "(Kein Projekt)" + len(pp.projects) project rows
		h = c.ConstrainHeight(float32(28 + (1+len(pp.projects))*24))
	}
	return geometry.Sz(w, h)
}

func (pp *ProjectPicker) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := pp.Bounds()
	theme := DefaultDarkTheme

	// Main button background
	btnRect := geometry.NewRect(b.Min.X, b.Min.Y, b.Width(), 28)
	canvas.DrawRoundRect(btnRect, theme.InputBg, 4)
	canvas.StrokeRoundRect(btnRect, theme.InputBorder, 4, 1.0)

	// Display selected project or placeholder
	displayText := formatProjectLabel(pp.selectedProject)
	canvas.DrawText(displayText, geometry.NewRect(b.Min.X+6, b.Min.Y+4, b.Width()-24, 20), 11, theme.TextPrimary, false, widget.TextAlignLeft)

	// Dropdown arrow
	arrowText := "v"
	if pp.isExpanded {
		arrowText = "^"
	}
	canvas.DrawText(arrowText, geometry.NewRect(b.Max.X-18, b.Min.Y+4, 12, 20), 11, theme.TextPrimary, false, widget.TextAlignCenter)

	// Dropdown menu if expanded
	if pp.isExpanded {
		menuY := b.Min.Y + 30

		// Row 0: "(Kein Projekt)"
		noneRect := geometry.NewRect(b.Min.X, menuY, b.Width(), 24)
		noneBg := theme.InputBg
		if pp.selectedProject == nil {
			noneBg = widget.RGBA8(60, 72, 98, 255)
		}
		canvas.DrawRect(noneRect, noneBg)
		canvas.DrawText("(Kein Projekt)", geometry.NewRect(noneRect.Min.X+8, noneRect.Min.Y+4, noneRect.Width()-16, 16), 10, theme.TextSecondary, false, widget.TextAlignLeft)

		// Rows 1..N: Projects
		for i, proj := range pp.projects {
			itemRect := geometry.NewRect(b.Min.X, menuY+float32((i+1)*24), b.Width(), 24)
			itemBg := theme.InputBg
			if pp.selectedProject != nil && pp.selectedProject.ID == proj.ID {
				itemBg = widget.RGBA8(60, 72, 98, 255)
			}
			canvas.DrawRect(itemRect, itemBg)
			itemText := formatProjectLabel(&proj)
			canvas.DrawText(itemText, geometry.NewRect(itemRect.Min.X+8, itemRect.Min.Y+4, itemRect.Width()-16, 16), 10, theme.TextPrimary, false, widget.TextAlignLeft)
		}
	}
}

func (pp *ProjectPicker) Event(ctx widget.Context, e event.Event) bool {
	switch ev := e.(type) {
	case *event.MouseEvent:
		b := pp.Bounds()
		btnRect := geometry.NewRect(b.Min.X, b.Min.Y, b.Width(), 28)

		if ev.MouseType == event.MousePress && btnRect.Contains(ev.Position) {
			pp.isExpanded = !pp.isExpanded
			return true
		}

		if pp.isExpanded && ev.MouseType == event.MousePress {
			menuY := b.Min.Y + 30

			// Row 0: "(Kein Projekt)"
			noneRect := geometry.NewRect(b.Min.X, menuY, b.Width(), 24)
			if noneRect.Contains(ev.Position) {
				pp.selectedProject = nil
				pp.isExpanded = false
				if pp.onChange != nil {
					pp.onChange(nil)
				}
				return true
			}

			// Rows 1..N: Projects
			for i := range pp.projects {
				itemRect := geometry.NewRect(b.Min.X, menuY+float32((i+1)*24), b.Width(), 24)
				if itemRect.Contains(ev.Position) {
					pp.selectedProject = &pp.projects[i]
					pp.isExpanded = false
					if pp.onChange != nil {
						pp.onChange(pp.selectedProject)
					}
					return true
				}
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test time-tracker/pkg/ui -run TestProjectPicker -v`
Expected: PASS.

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: All packages pass.

- [ ] **Step 6: Commit**

```bash
git add pkg/ui/project_picker.go pkg/ui/project_picker_test.go
git commit -m "feat(ui): display Customer - Project in ProjectPicker with unselect row"
```

---

### Task 3: Regression Testing, Version Bump & Release v1.0.6

**Files:**
- Modify: `cmd/time-tracker/main.go:25-28`
- Modify: `build.sh:11`

- [ ] **Step 1: Bump version to `1.0.6`**
  - In `cmd/time-tracker/main.go`: `version = "1.0.6"`
  - In `build.sh`: `VERSION="${VERSION:-1.0.6}"`

- [ ] **Step 2: Run all tests across repo**
  Run: `go test ./... -v`
  Expected: All tests pass.

- [ ] **Step 3: Test local install**
  Run: `./build.sh install`
  Expected: Successful compilation of Universal 2 binary and installation to `/Applications/Simple Time Tracker.app`.

- [ ] **Step 4: Commit and push**
  ```bash
  git add cmd/time-tracker/main.go build.sh
  git commit -m "chore(release): bump version to 1.0.6"
  git push origin main
  ```

- [ ] **Step 5: Release v1.0.6 to GitHub and Homebrew Tap**
  Run: `./build.sh release 1.0.6`
  Expected: GitHub release created, cask updated in `diver80/homebrew-tap`.
