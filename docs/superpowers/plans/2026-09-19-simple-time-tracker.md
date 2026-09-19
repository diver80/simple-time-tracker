# Simple Time Tracker (stt) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix input and focus handling in Simple Time Tracker, add calendar timeline click-to-create and manual booking editing, generate a modern macOS app icon, and build a full multi-platform build and Homebrew tap release pipeline.

**Architecture:** Enhances `TextInput` with explicit focus lifecycle methods (`SetFocused`/`IsFocused`) and outside-click blur. Wires event forwarding and widget tree traversal in `HUDView` and `AppView`. Implements click-to-hour math on the calendar timeline and quick duration buttons in `EntryEditor`. Adds a Python-based macOS squircle icon generator and a comprehensive `build.sh` script supporting Universal 2 builds, DMG creation, local installation, and Homebrew tap Cask publishing.

**Tech Stack:** Go 1.27, `gogpu/ui` v0.1.54, SQLite (`modernc.org/sqlite`), Cocoa/macOS Obj-C runtime (`manager_darwin.m`), Python 3 Pillow (icon generation), Homebrew Cask.

## Global Constraints

- Application Display Name: `Simple Time Tracker`
- Binary Name: `stt`
- Bundle ID: `com.avono.simple-time-tracker`
- Homebrew Tap: `diver80/homebrew-tap`
- Homebrew Cask: `Casks/simple-time-tracker.rb`
- GitHub Repository: `diver80/simple-time-tracker`
- Preserves SQLite backward compatibility with existing databases at `~/Library/Application Support/time-tracker/data.db` and legacy `yokto-time`.

---

### Task 1: Fix `TextInput` Focus Lifecycle, Blur, and Navigation

**Files:**
- Modify: `pkg/ui/text_input.go`
- Test: `pkg/ui/text_input_test.go`

**Interfaces:**
- Produces:
  - `(t *TextInput) SetFocused(focused bool)`
  - `(t *TextInput) IsFocused() bool`
  - Outside-click blur in `handleMouseEvent(ctx, ev)`

- [ ] **Step 1: Write failing tests for `SetFocused`, `IsFocused`, and outside-click blur**

Add to `pkg/ui/text_input_test.go`:
```go
func TestTextInputSetFocusedLifecycle(t *testing.T) {
	ti := NewTextInput("placeholder", false, nil)
	ti.SetBounds(geometry.NewRect(10, 10, 200, 30))

	if ti.IsFocused() {
		t.Fatal("expected newly created TextInput to not be focused")
	}

	ti.SetFocused(true)
	if !ti.IsFocused() {
		t.Fatal("expected TextInput to be focused after SetFocused(true)")
	}

	ti.SetFocused(false)
	if ti.IsFocused() {
		t.Fatal("expected TextInput to blur after SetFocused(false)")
	}
}

func TestTextInputOutsideClickBlur(t *testing.T) {
	ti := NewTextInput("placeholder", false, nil)
	ti.SetBounds(geometry.NewRect(10, 10, 200, 30))
	ti.SetFocused(true)

	// Simulate mouse press outside bounds
	outsideEv := event.NewMouseEvent(event.MousePress, geometry.Pt(300, 300), event.ButtonLeft, event.ModNone)
	mockCtx := &testWidgetContext{}
	ti.Event(mockCtx, outsideEv)

	if ti.IsFocused() {
		t.Fatal("expected TextInput to lose focus when clicking outside bounds")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/ui -run "TestTextInputSetFocusedLifecycle|TestTextInputOutsideClickBlur" -v`  
Expected: FAIL (`SetFocused` or `IsFocused` not defined).

- [ ] **Step 3: Implement `SetFocused`, `IsFocused`, and outside click blur in `TextInput`**

In `pkg/ui/text_input.go`:
```go
// SetFocused updates focus state and resets selection on blur.
func (t *TextInput) SetFocused(focused bool) {
	t.isFocused = focused
	if !focused {
		t.selStart = -1
		t.selEnd = -1
	}
}

// IsFocused returns true if the input currently has focus.
func (t *TextInput) IsFocused() bool {
	return t.isFocused
}
```
Update `handleMouseEvent` in `pkg/ui/text_input.go`:
```go
func (t *TextInput) handleMouseEvent(ctx widget.Context, ev *event.MouseEvent) bool {
	bounds := t.Bounds()

	switch ev.MouseType {
	case event.MousePress:
		if bounds.Contains(ev.Position) {
			ctx.RequestFocus(t)
			t.isFocused = true

			padding := float32(6)
			textBounds := geometry.NewRect(
				bounds.Min.X+padding,
				bounds.Min.Y+padding,
				bounds.Width()-2*padding,
				bounds.Height()-2*padding,
			)

			clickX := ev.Position.X - textBounds.Min.X
			if clickX < 0 {
				clickX = 0
			}

			estimatedPos := int(clickX / 6)
			if estimatedPos > len(t.runes) {
				estimatedPos = len(t.runes)
			}
			t.cursorPos = estimatedPos

			t.selStart = -1
			t.selEnd = -1
			ctx.Invalidate()
			return true
		} else {
			if t.isFocused {
				t.isFocused = false
				t.selStart = -1
				t.selEnd = -1
				ctx.Invalidate()
			}
		}
	case event.MouseMove:
		if bounds.Contains(ev.Position) {
			ctx.SetCursor(widget.CursorText)
			return true
		}
	}

	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/ui -run "TestTextInput.*" -v`  
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/ui/text_input.go pkg/ui/text_input_test.go
git commit -m "fix(ui): implement SetFocused, IsFocused, and outside-click blur on TextInput"
```

---

### Task 2: Fix `HUDView` & `AppView` Event Routing and Widget Tree

**Files:**
- Modify: `pkg/ui/hud_view.go`
- Modify: `pkg/ui/app_view.go`
- Test: `pkg/ui/hud_view_test.go`
- Test: `pkg/ui/app_view_test.go`

**Interfaces:**
- Consumes: `TextInput.SetFocused`, `TextInput.IsFocused`, `TextInput.Event`
- Produces: Correct event routing for taskInput, notesInput, and projectInput in `HUDView.Event(ctx, e)`

- [ ] **Step 1: Write test for `HUDView` event delegation to text inputs and picker**

In `pkg/ui/hud_view_test.go`:
```go
func TestHUDViewInputEventDelegation(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()
	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	hud := NewHUDView(timerSvc, repo, nil)
	hud.SetBounds(geometry.NewRect(0, 0, 400, 520))

	// Layout and draw to establish bounds
	mockCtx := &testWidgetContext{}
	hud.Draw(mockCtx, &testCanvas{})

	// Click on task input
	taskBounds := hud.taskInput.Bounds()
	clickPt := geometry.Pt(taskBounds.Min.X+10, taskBounds.Min.Y+10)
	pressEv := event.NewMouseEvent(event.MousePress, clickPt, event.ButtonLeft, event.ModNone)

	handled := hud.Event(mockCtx, pressEv)
	if !handled || !hud.taskInput.IsFocused() {
		t.Fatalf("expected clicking task input to focus it, handled=%v, focused=%v", handled, hud.taskInput.IsFocused())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/ui -run "TestHUDViewInputEventDelegation" -v`  
Expected: FAIL (`handled=false` because `HUDView.Event` did not forward to `taskInput`).

- [ ] **Step 3: Fix `HUDView.Event`, `refreshState`, and `Children()`**

In `pkg/ui/hud_view.go`:
1. In `refreshState()`, do not overwrite `inputTaskName` and `inputBookingText` if the inputs are currently focused:
```go
func (h *HUDView) refreshState() {
	h.state, h.activeEntry, h.elapsed = h.timerSvc.GetCurrentState()
	if h.activeEntry != nil {
		if !h.taskInput.IsFocused() {
			h.inputTaskName = h.activeEntry.TaskName
			h.taskInput.SetText(h.inputTaskName)
		}
		if !h.notesInput.IsFocused() {
			h.inputBookingText = h.activeEntry.BookingText
			h.notesInput.SetText(h.inputBookingText)
		}
		h.projectInput.SetProjectID(h.activeEntry.ProjectID)
	}
	// ... rest of refreshState
}
```
2. In `HUDView.Event(ctx, e)`:
```go
func (h *HUDView) Event(ctx widget.Context, e event.Event) bool {
	// 1. Give project picker top priority if expanded or clicked
	if h.projectInput.Event(ctx, e) {
		return true
	}

	// 2. Delegate to action buttons
	if h.state == timer.StateIdle {
		if h.startBtn.Event(ctx, e) {
			return true
		}
	} else if h.state == timer.StateQuickShift {
		if h.finishQSBtn.Event(ctx, e) {
			return true
		}
		if h.stopBtn.Event(ctx, e) {
			return true
		}
	} else {
		if h.stopBtn.Event(ctx, e) {
			return true
		}
		if h.quickShiftBtn.Event(ctx, e) {
			return true
		}
	}

	// 3. Delegate to Text Inputs
	if h.taskInput.Event(ctx, e) {
		return true
	}
	if h.notesInput.Event(ctx, e) {
		return true
	}

	return false
}
```
3. Update `HUDView.Children()`:
```go
func (h *HUDView) Children() []widget.Widget {
	return []widget.Widget{
		h.taskInput,
		h.notesInput,
		h.projectInput,
		h.startBtn,
		h.stopBtn,
		h.quickShiftBtn,
		h.finishQSBtn,
		h.pauseBtn,
		h.resumeBtn,
	}
}
```
4. Update `AppView.Children()` in `pkg/ui/app_view.go`:
```go
func (a *AppView) Children() []widget.Widget {
	children := []widget.Widget{
		a.tabTrackerBtn,
		a.tabCalendarBtn,
		a.tabExportBtn,
		a.tabProjectsBtn,
		a.tabShieldBtn,
	}
	switch a.activeTab {
	case TabTracker:
		children = append(children, a.hudView)
	case TabCalendar:
		children = append(children, a.calendarView)
	case TabExport:
		children = append(children, a.exportView)
	case TabProjects:
		children = append(children, a.projectView)
	}
	return children
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/ui -run "TestHUDViewInputEventDelegation|TestAppView.*" -v`  
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/ui/hud_view.go pkg/ui/app_view.go pkg/ui/hud_view_test.go
git commit -m "fix(ui): route events properly to HUD inputs and expose complete widget tree hierarchy"
```

---

### Task 3: Calendar Timeline Click-to-Create & `EntryEditor` Quick Actions

**Files:**
- Modify: `pkg/ui/calendar_view.go`
- Modify: `pkg/ui/entry_editor.go`
- Test: `pkg/ui/calendar_edit_test.go`

**Interfaces:**
- Produces:
  - Timeline empty slot detection in `CalendarView.Event` opening `EntryEditor` pre-filled with the clicked hour
  - Quick duration buttons (`+15m`, `+30m`, `+1h`) in `EntryEditor`

- [ ] **Step 1: Write test for timeline slot click-to-create**

In `pkg/ui/calendar_edit_test.go`:
```go
func TestCalendarTimelineClickToCreate(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()

	cv := NewCalendarView(repo, nil)
	cv.SetBounds(geometry.NewRect(0, 0, 420, 640))

	mockCtx := &testWidgetContext{}
	cv.Draw(mockCtx, &testCanvas{})

	// Simulate clicking on the timeline at 14:00 (Y coordinate inside content area)
	// Day starts at 7, ends at 21. Content top is ~84, bottom is ~590.
	contentTop := float32(84)
	contentBottom := float32(590)
	hourHeight := (contentBottom - contentTop) / float32(dayEndHour-dayStartHour)
	targetY := contentTop + float32(14-dayStartHour)*hourHeight + 10

	clickPt := geometry.Pt(100, targetY)
	pressEv := event.NewMouseEvent(event.MousePress, clickPt, event.ButtonLeft, event.ModNone)

	handled := cv.Event(mockCtx, pressEv)
	if !handled || cv.editor == nil {
		t.Fatalf("expected clicking timeline slot to open editor, handled=%v, editor=%v", handled, cv.editor)
	}

	startStr := cv.editor.startDTInput.Text()
	if !strings.Contains(startStr, "14:00") {
		t.Errorf("expected editor start time to be 14:00, got: %s", startStr)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/ui -run "TestCalendarTimelineClickToCreate" -v`  
Expected: FAIL (`handled=false` or `cv.editor == nil`).

- [ ] **Step 3: Implement timeline click-to-create and `EntryEditor` duration buttons**

In `pkg/ui/calendar_view.go`:
Add timeline area click detection in `CalendarView.Event`:
```go
// Check if click was in timeline content area
if ev.Position.X >= timelineGutterX && ev.Position.X <= b.Max.X-16 &&
   ev.Position.Y >= contentTop && ev.Position.Y <= contentBottom {
    hourFraction := (ev.Position.Y - contentTop) / hourHeight
    clickedHour := dayStartHour + int(hourFraction)
    if clickedHour >= dayStartHour && clickedHour < dayEndHour {
        start := time.Date(cv.currentDay.Year(), cv.currentDay.Month(), cv.currentDay.Day(), clickedHour, 0, 0, 0, time.Local)
        end := start.Add(1 * time.Hour)
        cv.openEditorWithTimes(start, end)
        return true
    }
}
```
In `pkg/ui/entry_editor.go`:
1. Add `plus15Btn`, `plus30Btn`, `plus60Btn` buttons.
2. Clicking these buttons parses `endDTInput`, adds the duration, and updates `endDTInput.SetText()`.
3. Support keyboard Tab key to cycle between `taskInput` $\rightarrow$ `notesInput` $\rightarrow$ `startDTInput` $\rightarrow$ `endDTInput`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/ui -run "TestCalendar.*" -v`  
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/ui/calendar_view.go pkg/ui/entry_editor.go pkg/ui/calendar_edit_test.go
git commit -m "feat(ui): enable calendar timeline click-to-create and quick duration buttons in editor"
```

---

### Task 4: App Icon Generator & Asset Creation

**Files:**
- Create: `assets/generate_icon.py`
- Create: `assets/AppIcon.icns`
- Create: `assets/icon.png`
- Create: `assets/icon.ico`

**Interfaces:**
- Produces: `assets/AppIcon.icns` for macOS app bundle, `icon.png` (1024x1024 master)

- [ ] **Step 1: Write `assets/generate_icon.py` using Pillow**

Create `assets/generate_icon.py` supporting:
- Supersampled 2048×2048 squircle rendering with Apple HIG dimensions and smooth ambient drop shadow.
- Deep midnight obsidian/slate base `#0b0f19`.
- Luminous emerald `#10B981` and cyan `#06B6D4` stopwatch ring with precision ticks and central indicator.
- Export to 1024×1024 `icon.png`, multi-size `icon.ico`, and `iconset` containing:
  - `icon_16x16.png`, `icon_16x16@2x.png`
  - `icon_32x32.png`, `icon_32x32@2x.png`
  - `icon_128x128.png`, `icon_128x128@2x.png`
  - `icon_256x256.png`, `icon_256x256@2x.png`
  - `icon_512x512.png`, `icon_512x512@2x.png`
- Compiles with macOS `iconutil -c icns` into `assets/AppIcon.icns`.

- [ ] **Step 2: Run `python3 assets/generate_icon.py` to generate icons**

Run: `python3 assets/generate_icon.py`  
Verify: `test -f assets/AppIcon.icns && test -f assets/icon.png`  
Expected: Created `assets/AppIcon.icns` (~2MB) and `assets/icon.png`.

- [ ] **Step 3: Commit icon generator and assets**

```bash
git add assets/
git commit -m "feat(assets): add macOS squircle icon generator and compiled AppIcon.icns"
```

---

### Task 5: Upgrade Build Pipeline & Homebrew Tap Release

**Files:**
- Modify: `build.sh`

**Interfaces:**
- Produces:
  - `./build.sh osx`: macOS Universal 2 .app & .dmg
  - `./build.sh install`: Installs to `/Applications/Simple Time Tracker.app`
  - `./build.sh release [version]`: Publishes to GitHub & updates `diver80/homebrew-tap`

- [ ] **Step 1: Write full `build.sh` script**

Update `build.sh` mirroring `jira-quick-access/build.sh`:
- App configuration:
  - `APP_NAME="stt"`
  - `APP_DISPLAY_NAME="Simple Time Tracker"`
  - `BUNDLE_ID="com.avono.simple-time-tracker"`
  - `VERSION="${VERSION:-1.0.0}"`
- `build_osx`:
  - Builds darwin/arm64 and darwin/amd64
  - Creates Universal 2 binary with `lipo`
  - Assembles `Simple Time Tracker.app` with `Contents/Info.plist` and `Contents/Resources/AppIcon.icns`
  - Builds `.dmg` with `/Applications` drag-and-drop link
  - Creates `.tar.gz` archive
- `install_osx`:
  - Builds `.app`, copies to `/Applications/Simple Time Tracker.app`, removes quarantine
- `release_homebrew`:
  - Calculates SHA256 of DMG
  - Tags release, creates GitHub release via `gh release create`
  - Clones `diver80/homebrew-tap`, generates `Casks/simple-time-tracker.rb`, commits and pushes

- [ ] **Step 2: Run `./build.sh osx` to verify build and packaging**

Run: `./build.sh osx`  
Verify:
- `dist/osx.noindex/Simple Time Tracker.app` contains `Contents/MacOS/stt` and `Contents/Resources/AppIcon.icns`
- `file dist/osx.noindex/Simple\ Time\ Tracker.app/Contents/MacOS/stt` outputs: `Mach-O universal binary with 2 architectures: [x86_64] [arm64]`
- `dist/osx/Simple Time Tracker-v1.0.0-macOS-Universal.dmg` exists.

- [ ] **Step 3: Commit build script**

```bash
git add build.sh
git commit -m "feat(build): add multi-platform universal build, dmg packaging, and homebrew release script"
```

---

### Task 6: Align Application Branding and Full Regression Verification

**Files:**
- Modify: `cmd/time-tracker/main.go`
- Modify: `pkg/window/manager_darwin.m`
- Run: `go test ./...`

**Interfaces:**
- Updates window title and menu labels to `Simple Time Tracker`
- Preserves migration from legacy `yokto-time` and `time-tracker` database locations

- [ ] **Step 1: Update window title and menu actions in `cmd/time-tracker/main.go` and `manager_darwin.m`**

- In `cmd/time-tracker/main.go`:
  - Update `gogpuApp` title to `"Simple Time Tracker"`.
  - Ensure database path looks in `~/Library/Application Support/simple-time-tracker/data.db`, auto-migrating from `time-tracker/data.db` or `yokto-time/data.db` if found.
- In `pkg/window/manager_darwin.m`:
  - Update window search title to `"Simple Time Tracker"`.
  - Update menu title to `"Simple Time Tracker"`.

- [ ] **Step 2: Run all unit and integration tests**

Run: `go test ./... -v`  
Expected: All tests PASS.

- [ ] **Step 3: Verify local build and installation**

Run: `./build.sh install`  
Verify: `/Applications/Simple Time Tracker.app` installed successfully.

- [ ] **Step 4: Commit changes**

```bash
git add cmd/ pkg/window/
git commit -m "chore(branding): align window, menus, and database paths to Simple Time Tracker"
```
