# Yokto-Style Time Tracking App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a high-performance, minimalist desktop time tracking application for macOS inspired by Yokto, built in Go using [`github.com/gogpu/gogpu`](https://github.com/gogpu/gogpu) and [`github.com/gogpu/ui`](https://github.com/gogpu/ui), featuring Customer → Project → Task hierarchy, instant start with live editable booking text, QuickShift interruption tracking, a calendar-like Day View visualizing time blocks, monthly CSV export, and macOS Menu Bar status item integration.

**Architecture:** The application uses a modular Go architecture:
- `pkg/db`: Embedded pure-Go SQLite database (`modernc.org/sqlite`) for zero-CGO data storage, schema migrations, and relational queries across Customers, Projects, Tasks, and TimeEntries.
- `pkg/timer`: Decoupled timer service with wall-clock drift compensation, state machine (Idle, Running, QuickShift, Paused), live booking text persistence, and rounding math.
- `pkg/export`: Monthly and custom date range CSV export engine with customizable rounding (none, 5m, 15m) and billable amount calculations.
- `pkg/window`: Native macOS Cocoa `NSStatusItem` in the menu bar with live timer title, native drop-down menu shortcuts (Day View, Monthly Export, QuickShift, Toggle Window), and HUD popover positioning.
- `pkg/ui`: Gogpu GPU-accelerated UI containing four primary views: Tracker HUD, Calendar Day View, Monthly Export View, and Projects & Customers View.

**Tech Stack:**
- Language: Go 1.27 (`go1.27.1 darwin/arm64`)
- GUI Framework: `github.com/gogpu/gogpu v0.53.0`, `github.com/gogpu/ui v0.1.54`, `github.com/gogpu/gg v0.52.5`
- Database: `modernc.org/sqlite v1.35.0` (Pure Go, Zero CGO for storage)
- Native Windowing: macOS Cocoa / Objective-C runtime via CGO in `pkg/window/manager_darwin.go` (with cross-platform fallback)

## Global Constraints
- Pure Go for all core logic, data storage, and UI widgets.
- Platform: macOS primary target (menu bar status item + popover HUD), with cross-platform fallback for testing.
- Low-friction: A timer can be started with existing Customer/Project/Task OR started immediately with quick ad-hoc names and categorized later.
- Live booking text auto-saves directly to SQLite on change so no notes are lost.
- Calendar Day View displays 24h / workday hour scale (07:00 to 20:00) with task block heights proportional to duration.

---

## File Structure

```text
time/
├── cmd/
│   └── yokto-time/
│       └── main.go                  # Main entry point, flags, wiring
├── pkg/
│   ├── db/
│   │   ├── models.go                # Customer, Project, Task, TimeEntry structs
│   │   ├── schema.go                # SQLite DDL migrations & indexes
│   │   ├── repository.go            # CRUD operations & queries
│   │   └── repository_test.go       # SQLite unit tests
│   ├── timer/
│   │   ├── service.go               # Timer state machine & ticker
│   │   ├── rounding.go              # Rounding algorithms (ceil, floor, nearest)
│   │   ├── rounding_test.go         # Rounding test suite
│   │   └── service_test.go          # State transition & QuickShift unit tests
│   ├── export/
│   │   ├── csv.go                   # Monthly CSV generator
│   │   └── csv_test.go              # CSV formatting & billing unit tests
│   ├── window/
│   │   ├── manager.go               # WindowManager interface & factory
│   │   ├── manager_darwin.go        # macOS NSStatusItem & popover positioning
│   │   └── manager_fallback.go      # Non-darwin stub
│   └── ui/
│       ├── theme.go                 # Liquid glass & minimalist dark/light palette
│       ├── components.go            # Reusable UI widgets (Pill, Button, TimeInput)
│       ├── app_view.go              # Navigation root & tab container
│       ├── hud_view.go              # Tracker view: timer, task, live booking text, QuickShift
│       ├── calendar_view.go         # Calendar-like day view with task blocks & now-line
│       ├── export_view.go           # Monthly export view with CSV download & preview
│       └── project_view.go          # Customer & Project budget management
├── build.sh                         # Build script with CGO & Cocoa flags
├── go.mod
└── go.sum
```

---

## Proposed Changes

Grouped logically from foundational data and engine layers up to UI views and main integration.

---

### Task 1: Project Scaffolding & Go Modules

**Files:**
- Create: `go.mod`
- Create: `build.sh`
- Create: `.gitignore`

**Interfaces:**
- Produces: Initialized Go project with `github.com/gogpu/gogpu`, `github.com/gogpu/ui`, `modernc.org/sqlite`.

- [ ] **Step 1: Create `go.mod` and add dependencies**

```go
module yokto-time

go 1.27.0

require (
	github.com/gogpu/gogpu v0.53.0
	github.com/gogpu/ui v0.1.54
	modernc.org/sqlite v1.35.0
)
```

- [ ] **Step 2: Run `go mod tidy`**

Run: `go mod tidy`
Expected: Resolves dependencies and generates `go.sum`.

- [ ] **Step 3: Create `.gitignore` and `build.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
APP_NAME="yokto-time"
echo "Building ${APP_NAME}..."
go build -o "bin/${APP_NAME}" ./cmd/yokto-time
echo "Build complete: bin/${APP_NAME}"
```

- [ ] **Step 4: Commit scaffolding**

```bash
git add go.mod go.sum .gitignore build.sh
git commit -m "chore: initialize project scaffolding and dependencies"
```

---

### Task 2: Database Layer (Customer → Project → Task → TimeEntry)

**Files:**
- Create: `pkg/db/models.go`
- Create: `pkg/db/schema.go`
- Create: `pkg/db/repository.go`
- Test: `pkg/db/repository_test.go`

**Interfaces:**
- Produces:
  ```go
  type Customer struct { ID int64; Name string; CreatedAt time.Time }
  type Project struct { ID int64; CustomerID int64; Name string; HourlyRate float64; BudgetHours float64; Color string; IsActive bool }
  type TimeEntry struct {
      ID int64
      ProjectID *int64
      CustomerName string
      ProjectName  string
      TaskName     string
      BookingText  string
      StartedAt    time.Time
      EndedAt      *time.Time
      DurationSec  int64
      IsBillable   bool
      IsQuickShift bool
      ParentID     *int64
      CreatedAt    time.Time
      UpdatedAt    time.Time
  }
  type Repository interface {
      Close() error
      CreateCustomer(name string) (*Customer, error)
      ListCustomers() ([]Customer, error)
      CreateProject(custID int64, name string, rate, budget float64, color string) (*Project, error)
      ListProjects(custID *int64) ([]Project, error)
      StartTimeEntry(projectID *int64, taskName string, isBillable bool) (*TimeEntry, error)
      StartQuickShift(parentID int64, taskName string) (*TimeEntry, error)
      UpdateActiveBookingText(id int64, text string) error
      UpdateActiveTaskName(id int64, name string) error
      AssignProject(id int64, projectID int64) error
      StopTimeEntry(id int64, endedAt time.Time) error
      GetActiveEntry() (*TimeEntry, error)
      ListEntriesForDay(day time.Time) ([]TimeEntry, error)
      ListEntriesForMonth(year int, month time.Month, customerID *int64, projectID *int64) ([]TimeEntry, error)
      DeleteEntry(id int64) error
  }
  ```

- [ ] **Step 1: Write the failing unit tests for SQLite repository**
- [ ] **Step 2: Run test to verify failure** (`go test ./pkg/db/... -v`)
- [ ] **Step 3: Implement models, schema, and repository**
- [ ] **Step 4: Run tests to verify passing**
- [ ] **Step 5: Commit**

---

### Task 3: Timer Engine, Live Booking Text & QuickShift

**Files:**
- Create: `pkg/timer/service.go`
- Create: `pkg/timer/rounding.go`
- Test: `pkg/timer/rounding_test.go`
- Test: `pkg/timer/service_test.go`

**Interfaces:**
- Consumes: `pkg/db.Repository`, `pkg/db.TimeEntry`
- Produces: `TimerService` with `StateIdle`, `StateRunning`, `StateQuickShift`, `StatePaused`

- [ ] **Step 1: Write rounding and timer service tests**
- [ ] **Step 2: Run tests to verify failure** (`go test ./pkg/timer/... -v`)
- [ ] **Step 3: Implement rounding math and TimerService with debounced booking text autosave**
- [ ] **Step 4: Run tests to verify passing**
- [ ] **Step 5: Commit**

---

### Task 4: Monthly Bookings CSV Exporter

**Files:**
- Create: `pkg/export/csv.go`
- Test: `pkg/export/csv_test.go`

**Interfaces:**
- Consumes: `pkg/db.Repository`, `pkg/db.TimeEntry`, `pkg/timer.RoundDuration`
- Produces: `GenerateMonthlyCSV(repo db.Repository, opts ExportOptions) (string, error)`

- [ ] **Step 1: Write CSV export test**
- [ ] **Step 2: Run tests to verify failure** (`go test ./pkg/export/... -v`)
- [ ] **Step 3: Implement CSV generator with UTF-8 BOM and decimal hours**
- [ ] **Step 4: Run tests to verify passing**
- [ ] **Step 5: Commit**

---

### Task 5: macOS Menu Bar & Window Manager

**Files:**
- Create: `pkg/window/manager.go`
- Create: `pkg/window/manager_darwin.go`
- Create: `pkg/window/manager_fallback.go`

**Interfaces:**
- Produces: `WindowManager` interface with `NSStatusItem`, timer updates, and popover positioning.

- [ ] **Step 1: Implement Cocoa NSStatusItem and menu actions (Day View, Monthly Export, QuickShift)**
- [ ] **Step 2: Implement manager_fallback.go for non-darwin environments**
- [ ] **Step 3: Verify darwin build compiles with CGO**
- [ ] **Step 4: Commit**

---

### Task 6: Gogpu UI Components & Theme

**Files:**
- Create: `pkg/ui/theme.go`
- Create: `pkg/ui/components.go`
- Test: `pkg/ui/components_test.go`

- [ ] **Step 1: Write component layout tests**
- [ ] **Step 2: Implement minimalist dark theme and widgets**
- [ ] **Step 3: Run component tests** (`go test ./pkg/ui/... -v`)
- [ ] **Step 4: Commit**

---

### Task 7: Tracker View (Customer/Project/Task, Live Booking Text & QuickShift HUD)

**Files:**
- Create: `pkg/ui/hud_view.go`

- [ ] **Step 1: Implement HUDView layout and event binding**
- [ ] **Step 2: Wire debounced booking text edits to TimerService.UpdateBookingText**
- [ ] **Step 3: Verify HUDView compiles**
- [ ] **Step 4: Commit**

---

### Task 8: Calendar-like Day View (Visual Time Block Timeline)

**Files:**
- Create: `pkg/ui/calendar_view.go`

- [ ] **Step 1: Implement CalendarView layout and vertical time-to-pixel coordinate projection**
- [ ] **Step 2: Implement task block renderer, duration badges, and click inspector**
- [ ] **Step 3: Verify compiling and layout bounds**
- [ ] **Step 4: Commit**

---

### Task 9: Monthly Export View & Project Management

**Files:**
- Create: `pkg/ui/export_view.go`
- Create: `pkg/ui/project_view.go`

- [ ] **Step 1: Implement ExportView (month/customer/project filters, preview, CSV export, clipboard copy)**
- [ ] **Step 2: Implement ProjectView (customer creation, projects, hourly rates, budget tracking)**
- [ ] **Step 3: Commit**

---

### Task 10: App Container, Menu Routing & End-to-End Verification

**Files:**
- Create: `pkg/ui/app_view.go`
- Create: `cmd/yokto-time/main.go`

- [ ] **Step 1: Implement AppView with tab strip and menu bar routing**
- [ ] **Step 2: Implement cmd/yokto-time/main.go (CLI flags, database init, service wiring)**
- [ ] **Step 3: Run comprehensive automated test suite** (`go test ./pkg/... -v`)
- [ ] **Step 4: Build binary and verify execution** (`./build.sh`)
- [ ] **Step 5: Commit**

---

## Verification Plan

### Automated Tests
```bash
go test ./pkg/... -v
```

### Manual Verification
1. Menu Bar: Status item shows live ticking time and opens popover.
2. QuickShift: Interruption pauses main task, logs subtask, resumes main task.
3. Calendar Day View: Visualizes blocks from 07:00–21:00 with colors and customer/project/task info.
4. Monthly CSV Export: Correct columns, rates, decimal hours, and totals.
