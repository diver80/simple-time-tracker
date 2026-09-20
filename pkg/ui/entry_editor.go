package ui

import (
	"fmt"
	"strings"
	"time"

	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// EntryEditor is a CRUD editor for TimeEntry with validation and error handling.
type EntryEditor struct {
	widget.WidgetBase

	repo                db.Repository
	entry               *db.TimeEntry      // Original entry being edited; nil for new entry
	currentDay          time.Time          // The day this entry is associated with
	onEntriesChanged    func()              // Callback after successful mutations
	onCancel            func()              // Callback when editor is cancelled
	onRequestRedraw     func()              // Request redraw
	isDraft             bool                // True if unsaved changes exist
	errorMessage        string              // Validation or mutation error
	deleteConfirmation  bool                // True if delete was clicked once, waiting for confirmation
	showDeleteWarning   bool                // Display delete warning

	// Form fields
	taskInput           *TextInput
	notesInput          *TextInput
	startDTInput        *TextInput
	endDTInput          *TextInput
	projectPicker       *ProjectPicker
	billableToggle      bool

	// Buttons
	saveBtn             *GlassButton
	cancelBtn           *GlassButton
	deleteBtn           *GlassButton
	confirmDeleteBtn    *GlassButton
	cancelDeleteBtn     *GlassButton
	plus15Btn           *GlassButton
	plus30Btn           *GlassButton
	plus60Btn           *GlassButton

	// Bounds tracking
	billableBounds      geometry.Rect
}

// NewEntryEditor creates a new entry editor for creating or editing an entry.
func NewEntryEditor(repo db.Repository, entry *db.TimeEntry, currentDay time.Time, onEntriesChanged func(), onCancel func(), onRequestRedraw func()) *EntryEditor {
	ed := &EntryEditor{
		repo:             repo,
		entry:            entry,
		currentDay:       currentDay,
		onEntriesChanged: onEntriesChanged,
		onCancel:         onCancel,
		onRequestRedraw:  onRequestRedraw,
		billableToggle:   false,
	}

	// Initialize form fields
	ed.taskInput = NewTextInput("Task", false, func(s string) {
		ed.isDraft = true
	})
	ed.notesInput = NewTextInput("Notizen", true, func(s string) {
		ed.isDraft = true
	})
	ed.startDTInput = NewTextInput("Startzeit (02.01.2006 15:04:05)", false, func(s string) {
		ed.isDraft = true
	})
	ed.endDTInput = NewTextInput("Endzeit (02.01.2006 15:04:05)", false, func(s string) {
		ed.isDraft = true
	})
	ed.projectPicker = NewProjectPicker(repo, func(p *db.Project) {
		ed.isDraft = true
	})

	ed.saveBtn = NewGlassButton("Speichern", ed.handleSave).SetCompact(true)
	ed.saveBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), DefaultDarkTheme.AccentPrimary, DefaultDarkTheme.CardBorder)
	ed.cancelBtn = NewGlassButton("Abbrechen", ed.handleCancel).SetCompact(true)
	ed.deleteBtn = NewGlassButton("Löschen", ed.handleDeleteClick).SetCompact(true)
	ed.confirmDeleteBtn = NewGlassButton("Löschen", ed.handleDeleteConfirm).SetCompact(true)
	ed.confirmDeleteBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), DefaultDarkTheme.StopColor, widget.RGBA8(220, 38, 38, 255))
	ed.cancelDeleteBtn = NewGlassButton("Abbrechen", ed.handleDeleteCancel).SetCompact(true)
	ed.plus15Btn = NewGlassButton("+15m", func() {
		ed.addDuration(15 * time.Minute)
	}).SetCompact(true)
	ed.plus30Btn = NewGlassButton("+30m", func() {
		ed.addDuration(30 * time.Minute)
	}).SetCompact(true)
	ed.plus60Btn = NewGlassButton("+1h", func() {
		ed.addDuration(60 * time.Minute)
	}).SetCompact(true)

	// Set parent for dirty redraw propagation
	ed.taskInput.SetParent(ed)
	ed.notesInput.SetParent(ed)
	ed.startDTInput.SetParent(ed)
	ed.endDTInput.SetParent(ed)
	ed.projectPicker.SetParent(ed)
	ed.saveBtn.SetParent(ed)
	ed.cancelBtn.SetParent(ed)
	ed.deleteBtn.SetParent(ed)
	ed.confirmDeleteBtn.SetParent(ed)
	ed.cancelDeleteBtn.SetParent(ed)
	ed.plus15Btn.SetParent(ed)
	ed.plus30Btn.SetParent(ed)
	ed.plus60Btn.SetParent(ed)

	// Populate form if editing existing entry
	if entry != nil {
		ed.taskInput.SetText(entry.TaskName)
		ed.notesInput.SetText(entry.BookingText)
		ed.startDTInput.SetText(entry.StartedAt.Format("02.01.2006 15:04:05"))
		if entry.EndedAt != nil {
			ed.endDTInput.SetText(entry.EndedAt.Format("02.01.2006 15:04:05"))
		}
		ed.billableToggle = entry.IsBillable
		if entry.ProjectID != nil {
			ed.projectPicker.SetProjectID(entry.ProjectID)
		}
	} else {
		if currentDay.IsZero() {
			currentDay = time.Now()
		}
		// For new entries, set default times on selected day (9:00-10:00)
		start := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), 9, 0, 0, 0, time.Local)
		end := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), 10, 0, 0, 0, time.Local)
		ed.startDTInput.SetText(start.Format("02.01.2006 15:04:05"))
		ed.endDTInput.SetText(end.Format("02.01.2006 15:04:05"))
	}

	ed.SetVisible(true)
	ed.SetEnabled(true)
	return ed
}

func (ed *EntryEditor) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(400)
	h := c.ConstrainHeight(600)
	return geometry.Sz(w, h)
}

func (ed *EntryEditor) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := ed.Bounds()
	theme := &DefaultDarkTheme

	// Background: 100% opaque solid card to ensure no underlying elements bleed through
	canvas.DrawRoundRect(b, widget.RGBA8(24, 28, 38, 255), 12)
	canvas.StrokeRoundRect(b, widget.RGBA8(60, 70, 92, 255), 12, 1.0)

	// Title with active entry warning
	titleText := "Neue Buchung"
	if ed.entry != nil {
		titleText = "Buchung bearbeiten"
		if ed.entry.EndedAt == nil {
			titleText += " (AKTIV)"
		}
	}
	canvas.DrawText(titleText, geometry.NewRect(b.Min.X+16, b.Min.Y+12, b.Width()-32, 20), 14, theme.TextPrimary, true, widget.TextAlignLeft)

	// Active entry warning
	if ed.entry != nil && ed.entry.EndedAt == nil {
		warningY := b.Min.Y + 36
		canvas.DrawText("Hinweis: Kann nicht bearbeitet werden, während aktiv", geometry.NewRect(b.Min.X+16, warningY, b.Width()-32, 24), 10, widget.RGBA8(255, 152, 0, 255), true, widget.TextAlignLeft)
	}

	// Form layout (simple vertical stacking)
	y := b.Min.Y + 40

	// Task Input
	canvas.DrawText("Task:", geometry.NewRect(b.Min.X+16, y, 80, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.taskInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.taskInput.Draw(ctx, canvas)
	y += 36

	// Notes Input
	canvas.DrawText("Notizen:", geometry.NewRect(b.Min.X+16, y, 80, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.notesInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 60))
	ed.notesInput.Draw(ctx, canvas)
	y += 68

	// Start DateTime
	canvas.DrawText("Startzeit:", geometry.NewRect(b.Min.X+16, y, 80, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.startDTInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.startDTInput.Draw(ctx, canvas)
	y += 36

	// End DateTime
	canvas.DrawText("Endzeit:", geometry.NewRect(b.Min.X+16, y+4, 60, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)

	btnW := float32(46)
	btnH := float32(20)
	btnGap := float32(6)
	totalBtnW := 3*btnW + 2*btnGap
	btnStartX := b.Max.X - 16 - totalBtnW
	minBtnStartX := b.Min.X + 16 + 60 + 8
	if btnStartX < minBtnStartX {
		btnStartX = minBtnStartX
	}

	ed.plus15Btn.SetBounds(geometry.NewRect(btnStartX, y, btnW, btnH))
	ed.plus15Btn.Draw(ctx, canvas)

	ed.plus30Btn.SetBounds(geometry.NewRect(btnStartX+btnW+btnGap, y, btnW, btnH))
	ed.plus30Btn.Draw(ctx, canvas)

	ed.plus60Btn.SetBounds(geometry.NewRect(btnStartX+2*(btnW+btnGap), y, btnW, btnH))
	ed.plus60Btn.Draw(ctx, canvas)

	y += 24
	ed.endDTInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.endDTInput.Draw(ctx, canvas)
	y += 36

	// Project Picker
	canvas.DrawText("Projekt:", geometry.NewRect(b.Min.X+16, y, 80, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.projectPicker.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.projectPicker.Draw(ctx, canvas)
	y += 36

	// Billable Toggle
	canvas.DrawText("Abrechenbar:", geometry.NewRect(b.Min.X+16, y, 100, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	toggleRect := geometry.NewRect(b.Min.X+120, y, 24, 18)
	// Store for hit testing in Event handler
	ed.billableBounds = toggleRect
	if ed.billableToggle {
		canvas.DrawRoundRect(toggleRect, widget.RGBA8(76, 175, 80, 255), 9)
	} else {
		canvas.DrawRoundRect(toggleRect, widget.RGBA8(100, 100, 100, 255), 9)
	}
	canvas.StrokeRoundRect(toggleRect, theme.InputBorder, 9, 1.0)
	y += 28

	// Error Message
	if ed.errorMessage != "" {
		canvas.DrawText(ed.errorMessage, geometry.NewRect(b.Min.X+16, y, b.Width()-32, 24), 9, widget.RGBA8(255, 100, 100, 255), true, widget.TextAlignLeft)
		y += 32
	}

	// Fixed bottom action area (visible at height 524)
	// Place buttons at fixed bottom position with padding
	actionY := b.Max.Y - 36

	// Delete Confirmation
	if ed.showDeleteWarning && ed.entry != nil {
		canvas.DrawText("Wirklich löschen? Klicke zum Bestätigen.", geometry.NewRect(b.Min.X+16, actionY-28, b.Width()-32, 20), 10, widget.RGBA8(255, 152, 0, 255), true, widget.TextAlignLeft)
		ed.confirmDeleteBtn.SetBounds(geometry.NewRect(b.Min.X+16, actionY, 60, 24))
		ed.confirmDeleteBtn.Draw(ctx, canvas)
		ed.cancelDeleteBtn.SetBounds(geometry.NewRect(b.Min.X+90, actionY, 80, 24))
		ed.cancelDeleteBtn.Draw(ctx, canvas)
	} else {
		// Button Layout (Save/Cancel/Delete)
		ed.saveBtn.SetBounds(geometry.NewRect(b.Min.X+16, actionY, 90, 24))
		ed.saveBtn.Draw(ctx, canvas)

		ed.cancelBtn.SetBounds(geometry.NewRect(b.Min.X+110, actionY, 90, 24))
		ed.cancelBtn.Draw(ctx, canvas)

		if ed.entry != nil {
			ed.deleteBtn.SetBounds(geometry.NewRect(b.Min.X+204, actionY, 90, 24))
			ed.deleteBtn.Draw(ctx, canvas)
		}
	}
}

func (ed *EntryEditor) Event(ctx widget.Context, e event.Event) bool {
	// Check if entry is active (being edited/running) - block interactions
	if ed.entry != nil && ed.entry.EndedAt == nil {
		// Active entry - only allow Cancel button
		switch ev := e.(type) {
		case *event.MouseEvent:
			if ev.MouseType == event.MousePress {
				if ed.cancelBtn.Bounds().Contains(ev.Position) {
					if ed.cancelBtn.Event(ctx, e) {
						return true
					}
				}
			}
		}
		return false
	}

	// Keyboard navigation: Tab / Shift+Tab cycling between inputs
	switch ev := e.(type) {
	case *event.KeyEvent:
		if ev.KeyType == event.KeyPress && ev.Key == event.KeyTab {
			inputs := []*TextInput{ed.taskInput, ed.notesInput, ed.startDTInput, ed.endDTInput}
			curIdx := -1
			for i, inp := range inputs {
				if inp.IsFocused() || (ctx != nil && ctx.IsFocused(inp)) {
					curIdx = i
					break
				}
			}

			var nextIdx int
			if ev.Modifiers().Has(event.ModShift) {
				if curIdx <= 0 {
					nextIdx = len(inputs) - 1
				} else {
					nextIdx = curIdx - 1
				}
			} else {
				if curIdx < 0 || curIdx >= len(inputs)-1 {
					nextIdx = 0
				} else {
					nextIdx = curIdx + 1
				}
			}

			for i, inp := range inputs {
				if i == nextIdx {
					inp.SetFocused(true)
					if ctx != nil {
						ctx.RequestFocus(inp)
					}
				} else {
					inp.SetFocused(false)
					if ctx != nil {
						ctx.ReleaseFocus(inp)
					}
				}
			}
			if ctx != nil {
				ctx.Invalidate()
			}
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return true
		}
	}

	// Quick duration buttons
	if ed.plus15Btn.Event(ctx, e) {
		return true
	}
	if ed.plus30Btn.Event(ctx, e) {
		return true
	}
	if ed.plus60Btn.Event(ctx, e) {
		return true
	}

	// Forward events to form widgets
	if ed.taskInput.Event(ctx, e) {
		return true
	}
	if ed.notesInput.Event(ctx, e) {
		return true
	}
	if ed.startDTInput.Event(ctx, e) {
		return true
	}
	if ed.endDTInput.Event(ctx, e) {
		return true
	}
	if ed.projectPicker.Event(ctx, e) {
		return true
	}

	// Billable toggle
	switch ev := e.(type) {
	case *event.MouseEvent:
		if ev.MouseType == event.MousePress {
			if ed.billableBounds.Contains(ev.Position) {
				ed.billableToggle = !ed.billableToggle
				ed.isDraft = true
				return true
			}
		}
	}

	// Button events
	if ed.showDeleteWarning && ed.entry != nil {
		if ed.confirmDeleteBtn.Event(ctx, e) {
			return true
		}
		if ed.cancelDeleteBtn.Event(ctx, e) {
			return true
		}
	} else {
		if ed.saveBtn.Event(ctx, e) {
			return true
		}
		if ed.cancelBtn.Event(ctx, e) {
			return true
		}
		if ed.entry != nil && ed.deleteBtn.Event(ctx, e) {
			return true
		}
	}

	return false
}

func (ed *EntryEditor) Children() []widget.Widget {
	children := []widget.Widget{
		ed.taskInput,
		ed.notesInput,
		ed.startDTInput,
		ed.endDTInput,
		ed.plus15Btn,
		ed.plus30Btn,
		ed.plus60Btn,
		ed.projectPicker,
		ed.saveBtn,
		ed.cancelBtn,
	}
	if ed.entry != nil {
		children = append(children, ed.deleteBtn)
	}
	if ed.showDeleteWarning && ed.entry != nil {
		children = append(children, ed.confirmDeleteBtn, ed.cancelDeleteBtn)
	}
	return children
}

func (ed *EntryEditor) handleSave() {
	ed.errorMessage = ""

	// Block save if entry is active (being edited/running)
	if ed.entry != nil && ed.entry.EndedAt == nil {
		ed.errorMessage = "Hinweis: Kann nicht bearbeitet werden, während aktiv"
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	// Validate task - check for whitespace-only input
	task := strings.TrimSpace(ed.taskInput.Text())
	if task == "" {
		ed.errorMessage = "Task ist erforderlich (keine Leerzeichen)"
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	// Parse start and end times using local time
	// Detect if input was formatted with or without seconds/nanoseconds
	startStr := ed.startDTInput.Text()
	endStr := ed.endDTInput.Text()

	// For new entries or when editing, preserve original nanoseconds if input unchanged
	var startTime, endTime time.Time
	var err error

	// Parse start time - try with seconds first, then without
	startTime, err = time.ParseInLocation("02.01.2006 15:04:05", startStr, time.Local)
	if err != nil {
		startTime, err = time.ParseInLocation("02.01.2006 15:04", startStr, time.Local)
		if err != nil {
			ed.errorMessage = "Ungültige Startzeit"
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
	}

	// Parse end time - try with seconds first, then without
	endTime, err = time.ParseInLocation("02.01.2006 15:04:05", endStr, time.Local)
	if err != nil {
		endTime, err = time.ParseInLocation("02.01.2006 15:04", endStr, time.Local)
		if err != nil {
			ed.errorMessage = "Ungültige Endzeit"
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
	}

	// If editing existing entry and times are unchanged (formatted same), preserve original nanoseconds/metadata
	if ed.entry != nil {
		origStartStr := ed.entry.StartedAt.Format("02.01.2006 15:04:05")
		origEndStr := ed.entry.EndedAt.Format("02.01.2006 15:04:05")
		if startStr == origStartStr {
			startTime = ed.entry.StartedAt
		}
		if endStr == origEndStr {
			endTime = *ed.entry.EndedAt
		}
	}

	// Validate end >= start
	if endTime.Before(startTime) {
		ed.errorMessage = "Endzeit muss nach Startzeit liegen"
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	// Calculate duration
	durationSec := int64(endTime.Sub(startTime).Seconds())

	// Get project
	var projectID *int64
	if ed.projectPicker.SelectedProject() != nil {
		projectID = &ed.projectPicker.SelectedProject().ID
	}

	// Create or update
	if ed.entry == nil {
		// Create new entry
		newEntry := &db.TimeEntry{
			TaskName:    task,
			BookingText: ed.notesInput.Text(),
			StartedAt:   startTime,
			EndedAt:     &endTime,
			DurationSec: durationSec,
			ProjectID:   projectID,
			IsBillable:  ed.billableToggle,
		}
		created, err := ed.repo.CreateManualEntry(newEntry)
		if err != nil {
			ed.errorMessage = fmt.Sprintf("Fehler beim Erstellen: %v", err)
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
		ed.entry = created
		// Update currentDay to match saved entry's day
		ed.currentDay = created.StartedAt
	} else {
		// Update existing entry - preserve original fields, only update editable ones
		updated := *ed.entry // Copy original
		updated.TaskName = task
		updated.BookingText = ed.notesInput.Text()
		updated.StartedAt = startTime
		updated.EndedAt = &endTime
		updated.DurationSec = durationSec
		updated.ProjectID = projectID
		updated.IsBillable = ed.billableToggle
		// Preserve: ID, CreatedAt, UpdatedAt, ParentID, PausedAt, PausedNS

		err := ed.repo.UpdateCompletedEntry(&updated)
		if err != nil {
			ed.errorMessage = fmt.Sprintf("Fehler beim Aktualisieren: %v", err)
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
		ed.entry = &updated
		// Update currentDay to match updated entry's day
		ed.currentDay = updated.StartedAt
	}

	ed.isDraft = false
	if ed.onEntriesChanged != nil {
		ed.onEntriesChanged()
	}
	if ed.onCancel != nil {
		ed.onCancel()
	}
}

func (ed *EntryEditor) handleCancel() {
	if ed.onCancel != nil {
		ed.onCancel()
	}
}

func (ed *EntryEditor) handleDeleteClick() {
	// Block delete if entry is active
	if ed.entry != nil && ed.entry.EndedAt == nil {
		ed.errorMessage = "Hinweis: Kann nicht gelöscht werden, während aktiv"
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}
	ed.showDeleteWarning = true
	if ed.onRequestRedraw != nil {
		ed.onRequestRedraw()
	}
}

func (ed *EntryEditor) handleDeleteConfirm() {
	if ed.entry == nil {
		return
	}

	ed.errorMessage = ""
	err := ed.repo.DeleteCompletedEntry(ed.entry.ID)
	if err != nil {
		ed.errorMessage = fmt.Sprintf("Fehler beim Löschen: %v", err)
		ed.showDeleteWarning = false
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	ed.entry = nil
	ed.showDeleteWarning = false
	if ed.onEntriesChanged != nil {
		ed.onEntriesChanged()
	}
	if ed.onCancel != nil {
		ed.onCancel()
	}
}

func (ed *EntryEditor) handleDeleteCancel() {
	ed.showDeleteWarning = false
	if ed.onRequestRedraw != nil {
		ed.onRequestRedraw()
	}
}

// SetTimes sets the start and end times and updates the current day.
func (ed *EntryEditor) SetTimes(start, end time.Time) {
	ed.startDTInput.SetText(start.Format("02.01.2006 15:04:05"))
	ed.endDTInput.SetText(end.Format("02.01.2006 15:04:05"))
	ed.currentDay = start
}

func (ed *EntryEditor) addDuration(d time.Duration) {
	startStr := strings.TrimSpace(ed.startDTInput.Text())
	endStr := strings.TrimSpace(ed.endDTInput.Text())

	var endTime time.Time
	var err error

	if startStr == "" && endStr == "" {
		now := time.Now()
		ed.startDTInput.SetText(now.Format("02.01.2006 15:04:05"))
		endTime = now
	} else if endStr != "" {
		endTime, err = time.ParseInLocation("02.01.2006 15:04:05", endStr, time.Local)
		if err != nil {
			endTime, err = time.ParseInLocation("02.01.2006 15:04", endStr, time.Local)
		}
	}

	if (startStr != "" || endStr != "") && (err != nil || endStr == "") {
		startTime, err2 := time.ParseInLocation("02.01.2006 15:04:05", startStr, time.Local)
		if err2 != nil {
			startTime, err2 = time.ParseInLocation("02.01.2006 15:04", startStr, time.Local)
		}
		if err2 == nil {
			endTime = startTime
		} else {
			endTime = time.Now()
		}
	}

	newEndTime := endTime.Add(d)
	ed.endDTInput.SetText(newEndTime.Format("02.01.2006 15:04:05"))
	ed.isDraft = true
	if ed.onRequestRedraw != nil {
		ed.onRequestRedraw()
	}
}
