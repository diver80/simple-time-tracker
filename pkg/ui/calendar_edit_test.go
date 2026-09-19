package ui

import (
	"strings"
	"testing"
	"time"

	"time-tracker/pkg/db"

	"github.com/gogpu/gg"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// TestCreateNewEntry tests creating a new entry through the editor with defaults on selected day
func TestCreateNewEntry(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	entriesChangedCalled := false
	var onEntriesChanged func()
	onEntriesChanged = func() {
		entriesChangedCalled = true
	}

	editor := NewEntryEditor(repo, nil, day, onEntriesChanged, func() {}, func() {})

	// Verify default times are set on the selected day (9:00-10:00)
	startText := editor.startDTInput.Text()
	if !strings.Contains(startText, "19.09.2026 09:00") {
		t.Errorf("Expected default start time on selected day, got %s", startText)
	}

	// Simulate user input
	editor.taskInput.SetText("Test Task")
	editor.notesInput.SetText("Test Notes")
	editor.billableToggle = true

	// Trigger save
	editor.handleSave()

	// Verify entry was created
	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if !entriesChangedCalled {
		t.Error("onEntriesChanged callback was not called")
	}

	entry := entries[0]
	if entry.TaskName != "Test Task" {
		t.Errorf("Expected task name 'Test Task', got '%s'", entry.TaskName)
	}
	if entry.BookingText != "Test Notes" {
		t.Errorf("Expected booking text 'Test Notes', got '%s'", entry.BookingText)
	}
	if !entry.IsBillable {
		t.Error("Expected entry to be billable")
	}
	if entry.DurationSec != 3600 { // 1 hour = 3600 seconds
		t.Errorf("Expected duration 3600 sec, got %d", entry.DurationSec)
	}
}

// TestValidateEmptyTask tests validation for empty and whitespace-only task
func TestValidateEmptyTask(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	// Test empty task
	editor.taskInput.SetText("")
	editor.startDTInput.SetText("19.09.2026 09:00:00")
	editor.endDTInput.SetText("19.09.2026 10:00:00")
	editor.handleSave()

	if editor.errorMessage == "" {
		t.Error("Expected validation error for empty task")
	}

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 0 {
		t.Error("Entry should not be created with empty task")
	}

	// Test whitespace-only task
	editor.errorMessage = ""
	editor.taskInput.SetText("   \t  ")
	editor.handleSave()

	if editor.errorMessage == "" {
		t.Error("Expected validation error for whitespace-only task")
	}

	entries, _ = repo.ListEntriesForDay(day)
	if len(entries) != 0 {
		t.Error("Entry should not be created with whitespace-only task")
	}
}

// TestValidateInvalidDateTime tests validation for invalid datetime
func TestValidateInvalidDateTime(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	editor.taskInput.SetText("Task")
	editor.startDTInput.SetText("invalid-date")
	editor.endDTInput.SetText("19.09.2026 10:00:00")

	editor.handleSave()

	if editor.errorMessage == "" {
		t.Error("Expected validation error for invalid datetime")
	}

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 0 {
		t.Error("Entry should not be created with invalid datetime")
	}
}

// TestValidateEndBeforeStart tests validation for end before start
func TestValidateEndBeforeStart(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	editor.taskInput.SetText("Task")
	editor.startDTInput.SetText("19.09.2026 10:00:00")
	editor.endDTInput.SetText("19.09.2026 09:00:00")

	editor.handleSave()

	if editor.errorMessage == "" {
		t.Error("Expected validation error for end before start")
	}

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 0 {
		t.Error("Entry should not be created with end before start")
	}
}

// TestEditEntry tests editing an existing entry
func TestEditEntry(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create initial entry
	origEntry := &db.TimeEntry{
		TaskName:    "Original Task",
		BookingText: "Original Notes",
		StartedAt:   time.Date(2026, 9, 19, 9, 0, 0, 0, time.Local),
		EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}[0],
		DurationSec: 3600,
		IsBillable:  false,
	}
	created, _ := repo.CreateManualEntry(origEntry)

	entriesChangedCalled := false
	editor := NewEntryEditor(repo, created, day, func() { entriesChangedCalled = true }, func() {}, func() {})

	// Verify initial values loaded
	if editor.taskInput.Text() != "Original Task" {
		t.Errorf("Expected task 'Original Task', got '%s'", editor.taskInput.Text())
	}

	// Edit the entry
	editor.taskInput.SetText("Updated Task")
	editor.billableToggle = true

	editor.handleSave()

	if !entriesChangedCalled {
		t.Error("onEntriesChanged callback was not called")
	}

	// Verify entry was updated
	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) == 0 {
		t.Fatal("Entry should exist after update")
	}
	if entries[0].TaskName != "Updated Task" {
		t.Errorf("Expected updated task name, got '%s'", entries[0].TaskName)
	}
	if !entries[0].IsBillable {
		t.Error("Expected updated entry to be billable")
	}
}

// TestDeleteEntry tests deleting an entry with confirmation
func TestDeleteEntry(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create entry to delete
	entry := &db.TimeEntry{
		TaskName:    "Task to Delete",
		StartedAt:   time.Date(2026, 9, 19, 9, 0, 0, 0, time.Local),
		EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}[0],
		DurationSec: 3600,
	}
	created, _ := repo.CreateManualEntry(entry)

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 1 {
		t.Fatal("Entry should be created first")
	}

	entriesChangedCalled := false
	editor := NewEntryEditor(repo, created, day, func() { entriesChangedCalled = true }, func() {}, func() {})

	// Click delete
	editor.handleDeleteClick()
	if !editor.showDeleteWarning {
		t.Error("Delete warning should be shown")
	}

	// Confirm delete
	editor.handleDeleteConfirm()

	if !entriesChangedCalled {
		t.Error("onEntriesChanged callback was not called after delete")
	}

	entries, _ = repo.ListEntriesForDay(day)
	if len(entries) != 0 {
		t.Error("Entry should be deleted")
	}
}

// TestCancelDelete tests canceling delete operation
func TestCancelDelete(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	entry := &db.TimeEntry{
		TaskName:    "Task",
		StartedAt:   time.Date(2026, 9, 19, 9, 0, 0, 0, time.Local),
		EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}[0],
		DurationSec: 3600,
	}
	created, _ := repo.CreateManualEntry(entry)

	editor := NewEntryEditor(repo, created, day, nil, func() {}, func() {})

	// Click delete then cancel
	editor.handleDeleteClick()
	editor.handleDeleteCancel()

	if editor.showDeleteWarning {
		t.Error("Delete warning should be hidden after cancel")
	}

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 1 {
		t.Error("Entry should not be deleted after cancel")
	}
}

// TestCancelEdit tests canceling edit
func TestCancelEdit(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	// Make changes
	editor.taskInput.SetText("New Task")
	editor.isDraft = true

	// Cancel
	cancelCalled := false
	editor.onCancel = func() {
		cancelCalled = true
	}
	editor.handleCancel()

	if !cancelCalled {
		t.Error("Cancel callback should be called")
	}
}

// TestDateTimeParsingFallback tests fallback parsing without seconds
func TestDateTimeParsingFallback(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	// Use format without seconds
	editor.taskInput.SetText("Task")
	editor.startDTInput.SetText("19.09.2026 09:00")
	editor.endDTInput.SetText("19.09.2026 10:00")

	editor.handleSave()

	if editor.errorMessage != "" {
		t.Errorf("Should parse datetime without seconds, got error: %s", editor.errorMessage)
	}

	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) != 1 {
		t.Error("Entry should be created with fallback parsing")
	}
}

// TestDateTimeParsingPreservesNanoseconds tests that unchanged formatted inputs preserve original nanoseconds
func TestDateTimeParsingPreservesNanoseconds(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create entry with nanoseconds
	startWithNanos := time.Date(2026, 9, 19, 9, 0, 0, 123456789, time.Local)
	endWithNanos := time.Date(2026, 9, 19, 10, 0, 0, 987654321, time.Local)
	origEntry := &db.TimeEntry{
		TaskName:    "Task",
		StartedAt:   startWithNanos,
		EndedAt:     &endWithNanos,
		DurationSec: 3600,
	}
	created, _ := repo.CreateManualEntry(origEntry)

	editor := NewEntryEditor(repo, created, day, nil, func() {}, func() {})

	// Edit without changing the time strings
	editor.taskInput.SetText("Updated Task")
	editor.handleSave()

	// Verify nanoseconds were preserved
	entries, _ := repo.ListEntriesForDay(day)
	if len(entries) == 0 {
		t.Fatal("Entry should exist")
	}
	if entries[0].StartedAt.Nanosecond() != 123456789 {
		t.Errorf("Expected start nanos 123456789, got %d", entries[0].StartedAt.Nanosecond())
	}
	if entries[0].EndedAt.Nanosecond() != 987654321 {
		t.Errorf("Expected end nanos 987654321, got %d", entries[0].EndedAt.Nanosecond())
	}
}

// TestActiveEntryGuard tests that active entries cannot be edited
// Note: Active entries can only be created via StartTimeEntry, not CreateManualEntry
// This test verifies that the guard prevents editing if somehow an active entry appears
func TestActiveEntryGuard(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Create a completed entry first
	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)
	completedEntry := &db.TimeEntry{
		TaskName:    "Completed Task",
		StartedAt:   time.Date(2026, 9, 19, 9, 0, 0, 0, time.Local),
		EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}[0],
		DurationSec: 3600,
	}
	created, _ := repo.CreateManualEntry(completedEntry)

	// Manually set EndedAt to nil to simulate active entry
	// (This is just for testing the guard; in production, active entries come from StartTimeEntry)
	activeEntry := *created
	activeEntry.EndedAt = nil

	editor := NewEntryEditor(repo, &activeEntry, day, nil, func() {}, func() {})

	// Verify warning is shown in title
	ctx := widget.NewContext()
	canvas := render.NewCanvas(gg.NewContext(400, 600), 400, 600)
	editor.SetBounds(geometry.NewRect(0, 0, 400, 600))

	// Try to save - should be blocked
	editor.taskInput.SetText("Should Not Save")
	editor.handleSave()

	// Verify error message was set (save was blocked)
	if editor.errorMessage == "" {
		t.Error("Expected error message when trying to save active entry")
	}
	if !strings.Contains(editor.errorMessage, "aktiv") {
		t.Errorf("Expected active entry error message, got: %s", editor.errorMessage)
	}

	// Draw should show warning in title area
	editor.Draw(ctx, canvas)
}

// TestCalendarViewTimelineClick tests clicking on timeline blocks opens editor
func TestCalendarViewTimelineClick(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create an entry
	entry := &db.TimeEntry{
		TaskName:    "Timeline Task",
		StartedAt:   time.Date(2026, 9, 19, 9, 0, 0, 0, time.Local),
		EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}[0],
		DurationSec: 3600,
	}
	repo.CreateManualEntry(entry)

	view := NewCalendarView(repo, func() {})
	view.currentDay = day
	view.Refresh()

	// Set bounds and draw to calculate block positions
	ctx := widget.NewContext()
	canvas := render.NewCanvas(gg.NewContext(420, 560), 420, 560)
	view.SetBounds(geometry.NewRect(0, 0, 420, 560))
	view.Draw(ctx, canvas)

	// Verify bounds were recorded
	if len(view.timelineBlockBounds) == 0 {
		t.Error("Timeline blocks should have recorded bounds")
		return
	}

	// Click on the recorded block
	blockBounds := view.timelineBlockBounds[0].bounds
	clickPos := geometry.Pt(blockBounds.Min.X+5, blockBounds.Min.Y+5)
	click := &event.MouseEvent{
		MouseType: event.MousePress,
		Position:  clickPos,
	}

	view.Event(ctx, click)

	// Verify editor was opened
	if view.editor == nil {
		t.Error("Editor should be opened after clicking timeline block")
	}
}

// TestCalendarViewListModeClick tests clicking on list items opens editor
func TestCalendarViewListModeClick(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create entries
	for i := 0; i < 3; i++ {
		entry := &db.TimeEntry{
			TaskName:    "Task " + string(rune('A'+i)),
			StartedAt:   time.Date(2026, 9, 19, 9+i, 0, 0, 0, time.Local),
			EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 10+i, 0, 0, 0, time.Local)}[0],
			DurationSec: 3600,
		}
		repo.CreateManualEntry(entry)
	}

	view := NewCalendarView(repo, func() {})
	view.currentDay = day
	view.isListMode = true
	view.Refresh()

	// Set bounds and draw
	ctx := widget.NewContext()
	canvas := render.NewCanvas(gg.NewContext(420, 560), 420, 560)
	view.SetBounds(geometry.NewRect(0, 0, 420, 560))
	view.Draw(ctx, canvas)

	// Verify bounds were recorded
	if len(view.listItemBounds) == 0 {
		t.Error("List items should have recorded bounds")
		return
	}

	// Click on first item
	firstItemBounds := view.listItemBounds[0].bounds
	clickPos := geometry.Pt(firstItemBounds.Min.X+5, firstItemBounds.Min.Y+5)
	click := &event.MouseEvent{
		MouseType: event.MousePress,
		Position:  clickPos,
	}

	view.Event(ctx, click)

	// Verify editor was opened
	if view.editor == nil {
		t.Error("Editor should be opened after clicking list item")
	}
}

// TestCalendarViewListModeScroll tests mouse wheel scrolling in list mode
func TestCalendarViewListModeScroll(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)

	// Create many entries (outside normal hours)
	for i := 0; i < 10; i++ {
		entry := &db.TimeEntry{
			TaskName:    "Task " + string(rune('A'+i)),
			StartedAt:   time.Date(2026, 9, 19, 1+i, 0, 0, 0, time.Local),
			EndedAt:     &[]time.Time{time.Date(2026, 9, 19, 2+i, 0, 0, 0, time.Local)}[0],
			DurationSec: 3600,
		}
		repo.CreateManualEntry(entry)
	}

	view := NewCalendarView(repo, func() {})
	view.currentDay = day
	view.isListMode = true
	view.Refresh()

	ctx := widget.NewContext()

	// Test scroll down
	if view.listScrollOffset != 0 {
		t.Error("Should start at offset 0")
	}

	wheel := &event.WheelEvent{Delta: geometry.Pt(0, -1)}
	view.Event(ctx, wheel)

	if view.listScrollOffset != 1 {
		t.Errorf("Expected scroll down to offset 1, got %d", view.listScrollOffset)
	}

	// Test scroll up
	wheel = &event.WheelEvent{Delta: geometry.Pt(0, 1)}
	view.Event(ctx, wheel)

	if view.listScrollOffset != 0 {
		t.Errorf("Expected scroll up to offset 0, got %d", view.listScrollOffset)
	}
}

// TestCalendarViewAddButtonDefaultsToSelectedDay tests that add button creates entry on selected day
func TestCalendarViewAddButtonDefaultsToSelectedDay(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	view := NewCalendarView(repo, func() {})
	view.currentDay = day

	if view.addBtn == nil {
		t.Fatal("Add button should exist")
	}

	// Editor should not be open initially
	if view.editor != nil {
		t.Error("Editor should not be open initially")
	}

	// Simulate clicking add button
	view.addBtn.onClick()

	if view.editor == nil {
		t.Error("Editor should be open after clicking add button")
	}

	// Verify default start time matches selected day
	startText := view.editor.startDTInput.Text()
	expectedDay := "20.09.2026"
	if !strings.Contains(startText, expectedDay) {
		t.Errorf("Expected start time to contain %s, got %s", expectedDay, startText)
	}
}

// TestBillableToggleHitRect tests that billable toggle can be clicked
func TestBillableToggleHitRect(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)
	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	ctx := widget.NewContext()
	canvas := render.NewCanvas(gg.NewContext(400, 600), 400, 600)
	editor.SetBounds(geometry.NewRect(0, 0, 400, 600))

	// Draw to set billableBounds
	editor.Draw(ctx, canvas)

	if editor.billableBounds.IsEmpty() {
		t.Error("Billable bounds should be set after draw")
		return
	}

	// Click on toggle
	clickPos := geometry.Pt(editor.billableBounds.Min.X+5, editor.billableBounds.Min.Y+5)
	click := &event.MouseEvent{
		MouseType: event.MousePress,
		Position:  clickPos,
	}

	initialState := editor.billableToggle
	editor.Event(ctx, click)

	if editor.billableToggle == initialState {
		t.Error("Billable toggle should change on click")
	}
}

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
	pressEv := event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, clickPt, clickPt, event.ModNone)

	handled := cv.Event(mockCtx, pressEv)
	if !handled || cv.editor == nil {
		t.Fatalf("expected clicking timeline slot to open editor, handled=%v, editor=%v", handled, cv.editor)
	}

	startStr := cv.editor.startDTInput.Text()
	if !strings.Contains(startStr, "14:00") {
		t.Errorf("expected editor start time to be 14:00, got: %s", startStr)
	}
}

func TestEntryEditorQuickDurationButtons(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)
	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})

	start := time.Date(2026, 9, 19, 14, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 19, 15, 0, 0, 0, time.Local)
	editor.startDTInput.SetText(start.Format("02.01.2006 15:04:05"))
	editor.endDTInput.SetText(end.Format("02.01.2006 15:04:05"))

	// Test +15m button
	if editor.plus15Btn == nil {
		t.Fatal("plus15Btn should not be nil")
	}
	editor.plus15Btn.onClick()
	if !strings.Contains(editor.endDTInput.Text(), "15:15:00") {
		t.Errorf("expected end time 15:15:00 after +15m, got: %s", editor.endDTInput.Text())
	}

	// Test +30m button
	if editor.plus30Btn == nil {
		t.Fatal("plus30Btn should not be nil")
	}
	editor.plus30Btn.onClick()
	if !strings.Contains(editor.endDTInput.Text(), "15:45:00") {
		t.Errorf("expected end time 15:45:00 after +30m, got: %s", editor.endDTInput.Text())
	}

	// Test +1h button
	if editor.plus60Btn == nil {
		t.Fatal("plus60Btn should not be nil")
	}
	editor.plus60Btn.onClick()
	if !strings.Contains(editor.endDTInput.Text(), "16:45:00") {
		t.Errorf("expected end time 16:45:00 after +1h, got: %s", editor.endDTInput.Text())
	}
}

func TestEntryEditorQuickDurationButtonsViaMouseEvent(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)
	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})
	editor.SetBounds(geometry.NewRect(0, 0, 400, 600))

	mockCtx := &testWidgetContext{}
	editor.Draw(mockCtx, &testCanvas{})

	start := time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 19, 11, 0, 0, 0, time.Local)
	editor.startDTInput.SetText(start.Format("02.01.2006 15:04:05"))
	editor.endDTInput.SetText(end.Format("02.01.2006 15:04:05"))

	// Click +15m button via mouse event
	b15 := editor.plus15Btn.Bounds()
	clickPt := geometry.Pt(b15.Min.X+5, b15.Min.Y+5)
	pressEv := event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, clickPt, clickPt, event.ModNone)
	handled := editor.Event(mockCtx, pressEv)
	if !handled {
		t.Fatal("expected clicking +15m button to be handled")
	}
	if !strings.Contains(editor.endDTInput.Text(), "11:15:00") {
		t.Errorf("expected end time 11:15:00, got: %s", editor.endDTInput.Text())
	}
}

func TestEntryEditorTabFocusCycle(t *testing.T) {
	repo, _ := db.NewRepository(":memory:")
	defer repo.Close()

	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local)
	editor := NewEntryEditor(repo, nil, day, nil, func() {}, func() {})
	mockCtx := &testWidgetContext{}

	// Focus taskInput initially
	editor.taskInput.SetFocused(true)
	mockCtx.RequestFocus(editor.taskInput)

	tabEv := event.NewKeyEvent(event.KeyPress, event.KeyTab, 0, event.ModNone)

	// Tab 1: taskInput -> notesInput
	handled := editor.Event(mockCtx, tabEv)
	if !handled {
		t.Fatal("expected Tab key event to be handled")
	}
	if !editor.notesInput.IsFocused() || editor.taskInput.IsFocused() {
		t.Errorf("expected notesInput to be focused, got task=%v, notes=%v", editor.taskInput.IsFocused(), editor.notesInput.IsFocused())
	}

	// Tab 2: notesInput -> startDTInput
	editor.Event(mockCtx, tabEv)
	if !editor.startDTInput.IsFocused() || editor.notesInput.IsFocused() {
		t.Errorf("expected startDTInput to be focused, got notes=%v, start=%v", editor.notesInput.IsFocused(), editor.startDTInput.IsFocused())
	}

	// Tab 3: startDTInput -> endDTInput
	editor.Event(mockCtx, tabEv)
	if !editor.endDTInput.IsFocused() || editor.startDTInput.IsFocused() {
		t.Errorf("expected endDTInput to be focused, got start=%v, end=%v", editor.startDTInput.IsFocused(), editor.endDTInput.IsFocused())
	}

	// Tab 4: endDTInput -> taskInput (cycle back)
	editor.Event(mockCtx, tabEv)
	if !editor.taskInput.IsFocused() || editor.endDTInput.IsFocused() {
		t.Errorf("expected taskInput to be focused after cycle, got end=%v, task=%v", editor.endDTInput.IsFocused(), editor.taskInput.IsFocused())
	}

	// Shift+Tab: taskInput -> endDTInput (reverse cycle)
	shiftTabEv := event.NewKeyEvent(event.KeyPress, event.KeyTab, 0, event.ModShift)
	editor.Event(mockCtx, shiftTabEv)
	if !editor.endDTInput.IsFocused() || editor.taskInput.IsFocused() {
		t.Errorf("expected endDTInput to be focused on Shift+Tab, got task=%v, end=%v", editor.taskInput.IsFocused(), editor.endDTInput.IsFocused())
	}
}
