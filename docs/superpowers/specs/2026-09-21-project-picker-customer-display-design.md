# ProjectPicker "Kunde - Projekt" Display & Deselection Design Specification

**Date:** 2026-09-21  
**Status:** Approved  
**Target Version:** 1.0.6  
**Scope:** `pkg/ui/project_picker.go`, `pkg/ui/project_picker_test.go`

---

## 1. Context & Motivation
When selecting projects in Simple Time Tracker (`stt`), the dropdown combobox (`ProjectPicker`) currently renders only the project name (e.g. `"Web Platform Redesign"`). Users managing multiple customers with similar or generic project names need immediate visibility of the customer association in both the closed combobox header and the expanded dropdown list.

Additionally, once a project is selected, there is currently no option in the expanded list to revert to `(Kein Projekt)`.

---

## 2. Requirements & Behavior

### 2.1 Label Formatting
- Introduce a helper function `formatProjectLabel(p *db.Project) string`:
  - If `p == nil`: returns `"(Kein Projekt)"`
  - If `p.CustomerName != ""`: returns `"<CustomerName> - <ProjectName>"` (e.g., `"Acme Corporation - Web Platform Redesign"`)
  - Fallback (when `CustomerName == ""`): returns `<ProjectName>`

### 2.2 Closed State Display
- The closed button displays `formatProjectLabel(pp.selectedProject)`.
- If no project is selected (`pp.selectedProject == nil`), it displays `"(Kein Projekt)"`.
- The display is truncated cleanly if exceeding bounds via standard text clipping.

### 2.3 Expanded Dropdown List
- The dropdown list contains `1 + len(pp.projects)` rows:
  - **Index 0:** `(Kein Projekt)`
    - Active highlight if `pp.selectedProject == nil`.
    - Clicking row 0 sets `pp.selectedProject = nil`, closes the dropdown, and invokes `pp.onChange(nil)`.
  - **Index `i + 1`:** Project `pp.projects[i]`
    - Display text: `formatProjectLabel(&pp.projects[i])`.
    - Active highlight if `pp.selectedProject != nil && pp.selectedProject.ID == proj.ID`.
    - Clicking row `i + 1` sets `pp.selectedProject = &pp.projects[i]`, closes the dropdown, and invokes `pp.onChange(pp.selectedProject)`.

### 2.4 Layout Height Calculation
- In `ProjectPicker.Layout`:
  - Closed height: 28px.
  - Expanded height: `28 + (1 + len(pp.projects)) * 24` px.

### 2.5 Integration with Consumers
- **`HUDView` (Tracker):**
  - Already handles `project == nil` in `projectInput`'s `onChange` callback:
    `if project != nil { id = &project.ID }` -> passes `nil` to `timerSvc.SetProject(nil)`.
- **`EntryEditor` (Calendar / Tag View):**
  - Already checks `if ed.projectPicker.SelectedProject() != nil { projectID = &ed.projectPicker.SelectedProject().ID }`.
  - When `nil`, `projectID` is set to `nil`.

---

## 3. Architecture & Data Flow

```
+-------------------------------------------------------------+
|                        ProjectPicker                        |
|                                                             |
|   Closed:  [ Acme Corporation - Web Platform Redesign    v ]|
+-------------------------------------------------------------+
|   Expanded Menu:                                            |
|   +-------------------------------------------------------+ |
|   | (Kein Projekt)                         [clears proj]  | |
|   | Acme Corporation - Web Platform Redesign   [active]   | |
|   | Acme Corporation - Security Audit                     | |
|   | Initech - Office Relocation                           | |
|   +-------------------------------------------------------+ |
+-------------------------------------------------------------+
```

Data retrieval:
`pp.repo.ListProjects(nil)` already performs an inner join on `customers`:
```sql
SELECT p.id, p.customer_id, c.name, p.name, p.hourly_rate, p.budget_hours, p.budget_cost, p.color, p.is_active, p.created_at
FROM projects p
JOIN customers c ON p.customer_id = c.id
ORDER BY c.name ASC, p.name ASC
```
The scanned `p.CustomerName` is already populated. No database schema or query changes are required.

---

## 4. Verification & Testing

1. **Unit Tests (`pkg/ui/project_picker_test.go`):**
   - `TestProjectPickerFormatting`: verifies that projects with a customer display as `"Customer - Project"`, fallback to project name when customer name is empty, and `"(Kein Projekt)"` when nil.
   - `TestProjectPickerDropdownItems`: verifies that index 0 is `(Kein Projekt)` and subsequent items are projects.
   - `TestProjectPickerSelectAndDeselect`:
     - Select project -> `onChange` receives project with ID, closed label shows `"Customer - Project"`.
     - Click `(Kein Projekt)` -> `onChange` receives `nil`, closed label shows `"(Kein Projekt)"`.
   - `TestProjectPickerLayoutHeight`: verifies closed height (28px) vs expanded height (`28 + (1+N)*24`).
2. **Regression Testing:**
   - Run `go test ./...` across all packages (`cmd/time-tracker`, `pkg/db`, `pkg/export`, `pkg/timer`, `pkg/ui`).
   - Run `./build.sh install` and verify in macOS application.
