package ui

import (
	"testing"
	"time"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// TestTextInputCreation tests basic widget creation and initialization
func TestTextInputCreation(t *testing.T) {
	ti := NewTextInput("Enter text", false, nil)
	if ti == nil {
		t.Fatal("NewTextInput returned nil")
	}
	if ti.Text() != "" {
		t.Errorf("Expected empty text, got %q", ti.Text())
	}
	if !ti.IsFocusable() {
		t.Error("TextInput should be focusable")
	}
}

// TestTextInputText tests Text() and SetText() methods
func TestTextInputText(t *testing.T) {
	ti := NewTextInput("", false, nil)

	ti.SetText("hello")
	if ti.Text() != "hello" {
		t.Errorf("Expected 'hello', got %q", ti.Text())
	}

	ti.SetText("world")
	if ti.Text() != "world" {
		t.Errorf("Expected 'world', got %q", ti.Text())
	}
}

// TestTextInputSetTextNoCallback ensures SetText doesn't trigger onChange
func TestTextInputSetTextNoCallback(t *testing.T) {
	called := false
	ti := NewTextInput("", false, func(s string) {
		called = true
	})

	ti.SetText("test")
	if called {
		t.Error("SetText should not trigger onChange callback")
	}

	if ti.Text() != "test" {
		t.Errorf("Expected 'test', got %q", ti.Text())
	}
}

// TestTextInputUnicodeHandling tests proper Unicode rune handling
func TestTextInputUnicodeHandling(t *testing.T) {
	ti := NewTextInput("", false, nil)

	// Test with mixed ASCII and Unicode
	ti.SetText("hello✓world")
	if ti.Text() != "hello✓world" {
		t.Errorf("Expected 'hello✓world', got %q", ti.Text())
	}

	// Verify rune count is correct (✓ is one rune, "helloworld" is 10)
	runes := []rune(ti.Text())
	if len(runes) != 11 {
		t.Errorf("Expected 11 runes, got %d", len(runes))
	}

	// Test emoji-like characters
	ti.SetText("🎯🎨🎭")
	if ti.Text() != "🎯🎨🎭" {
		t.Errorf("Expected emoji string, got %q", ti.Text())
	}
}

// TestTextInputMasking tests password/masking functionality
func TestTextInputMasking(t *testing.T) {
	ti := NewTextInput("", false, nil)
	ti.SetText("secret")

	if ti.Text() != "secret" {
		t.Errorf("Expected 'secret', got %q", ti.Text())
	}

	// Set masked (should not change the actual text)
	ti.SetMasked(true)

	if ti.Text() != "secret" {
		t.Errorf("Masking should not change Text(), expected 'secret', got %q", ti.Text())
	}

	if ti.maskChar != '•' {
		t.Errorf("Expected maskChar '•', got %q", ti.maskChar)
	}

	// Unmask
	ti.SetMasked(false)
	if ti.maskChar != 0 {
		t.Error("Unmask should clear maskChar")
	}
}

// TestTextInputHelperMethods tests internal helper functions
func TestTextInputHelperMethods(t *testing.T) {
	ti := NewTextInput("", false, nil)

	// Test insertRune
	ti.insertRune('a')
	if ti.Text() != "a" || ti.cursorPos != 1 {
		t.Errorf("insertRune failed: text=%q, cursorPos=%d", ti.Text(), ti.cursorPos)
	}

	ti.insertRune('b')
	if ti.Text() != "ab" || ti.cursorPos != 2 {
		t.Errorf("insertRune failed: text=%q, cursorPos=%d", ti.Text(), ti.cursorPos)
	}

	// Test insertRune with cursor in middle
	ti.cursorPos = 1
	ti.insertRune('x')
	if ti.Text() != "axb" || ti.cursorPos != 2 {
		t.Errorf("insertRune at middle failed: text=%q, cursorPos=%d", ti.Text(), ti.cursorPos)
	}
}

// TestTextInputSelection tests selection state management
func TestTextInputSelection(t *testing.T) {
	ti := NewTextInput("", false, nil)
	ti.SetText("hello")

	// Initially no selection
	if ti.selStart != -1 || ti.selEnd != -1 {
		t.Error("Should have no selection initially")
	}

	// Set selection
	ti.selStart = 0
	ti.selEnd = 5
	if ti.selStart != 0 || ti.selEnd != 5 {
		t.Error("Selection not set correctly")
	}

	// Test getSelection helper (accounts for reversed selection)
	ti.selStart = 5
	ti.selEnd = 0
	start, end := ti.getSelection()
	if start != 0 || end != 5 {
		t.Errorf("getSelection should normalize: got %d-%d", start, end)
	}
}

// TestTextInputDeleteSelection tests deleteSelection helper
func TestTextInputDeleteSelection(t *testing.T) {
	ti := NewTextInput("", false, nil)

	ti.SetText("hello world")

	// Select "hello"
	ti.selStart = 0
	ti.selEnd = 5

	// Delete selection
	ti.deleteSelection()

	if ti.Text() != " world" {
		t.Errorf("Expected ' world', got %q", ti.Text())
	}

	if ti.cursorPos != 0 {
		t.Errorf("Expected cursor at 0, got %d", ti.cursorPos)
	}

	if ti.selStart != -1 || ti.selEnd != -1 {
		t.Error("Selection should be cleared after delete")
	}
}

// TestTextInputMultilineFlag tests multiline mode initialization
func TestTextInputMultilineFlag(t *testing.T) {
	singleLine := NewTextInput("single", false, nil)
	multiLine := NewTextInput("multi", true, nil)

	if singleLine.multiline {
		t.Error("Single-line input should have multiline=false")
	}

	if !multiLine.multiline {
		t.Error("Multi-line input should have multiline=true")
	}
}

// TestTextInputClipboard tests clipboard function setter
func TestTextInputClipboard(t *testing.T) {
	ti := NewTextInput("", false, nil)

	readCalled := false
	writeCalled := false

	readFunc := func() (string, error) {
		readCalled = true
		return "clipboard text", nil
	}

	writeFunc := func(s string) error {
		writeCalled = true
		return nil
	}

	ti.SetClipboard(readFunc, writeFunc)

	if ti.clipboardRead == nil || ti.clipboardWrite == nil {
		t.Error("Clipboard functions not set")
	}

	// Test they can be called
	text, _ := ti.clipboardRead()
	if !readCalled || text != "clipboard text" {
		t.Error("Clipboard read function not working")
	}

	_ = ti.clipboardWrite("test")
	if !writeCalled {
		t.Error("Clipboard write function not working")
	}
}

// TestTextInputChildren tests Children() method returns nil
func TestTextInputChildren(t *testing.T) {
	ti := NewTextInput("", false, nil)
	children := ti.Children()

	if children != nil {
		t.Error("TextInput should have no children")
	}
}

// TestTextInputCursorBounds tests cursor position boundary conditions
func TestTextInputCursorBounds(t *testing.T) {
	ti := NewTextInput("", false, nil)
	ti.SetText("hello")

	// Test insertRune with cursor at start
	ti.cursorPos = 0
	ti.insertRune('X')
	if ti.Text() != "Xhello" {
		t.Errorf("Expected 'Xhello', got %q", ti.Text())
	}

	// Test insertRune with cursor out of bounds (should clamp)
	ti.cursorPos = 1000
	initialLen := len(ti.runes)
	ti.insertRune('Y')
	if ti.cursorPos != initialLen+1 {
		t.Errorf("insertRune should handle out-of-bounds cursor")
	}
}

// TestTextInputTriggerChange tests onChange callback behavior
func TestTextInputTriggerChange(t *testing.T) {
	callCount := 0
	lastText := ""

	ti := NewTextInput("", false, func(s string) {
		callCount++
		lastText = s
	})

	// Direct call to triggerChange
	ti.runes = []rune("test")
	ti.triggerChange()

	if callCount != 1 {
		t.Errorf("Expected onChange called once, got %d", callCount)
	}

	if lastText != "test" {
		t.Errorf("Expected 'test', got %q", lastText)
	}

	// Test nil callback doesn't panic
	ti2 := NewTextInput("", false, nil)
	ti2.runes = []rune("safe")
	ti2.triggerChange() // Should not panic
}

// TestTextInputCaretBlinking tests caret animation state
func TestTextInputCaretBlinking(t *testing.T) {
	ti := NewTextInput("", false, nil)

	if ti.caretBlink != 0 {
		t.Errorf("Initial caretBlink should be 0, got %f", ti.caretBlink)
	}

	// Simulate advancing caret blink
	for i := 0; i < 70; i++ {
		ti.caretBlink += 0.016
		if ti.caretBlink > 1.0 {
			ti.caretBlink -= 1.0
		}
	}

	// After cycling, should be close to 0 again
	if ti.caretBlink < 0 || ti.caretBlink > 1.0 {
		t.Errorf("Caret blink should be in [0,1], got %f", ti.caretBlink)
	}
}

// TestTextInputPlaceholder tests placeholder property
func TestTextInputPlaceholder(t *testing.T) {
	placeholder := "Enter your password"
	ti := NewTextInput(placeholder, false, nil)

	if ti.placeholder != placeholder {
		t.Errorf("Expected placeholder %q, got %q", placeholder, ti.placeholder)
	}
}

// TestTextInputFocusState tests focus flag
func TestTextInputFocusState(t *testing.T) {
	ti := NewTextInput("", false, nil)

	if ti.isFocused {
		t.Error("Should not be focused initially")
	}

	ti.isFocused = true
	if !ti.isFocused {
		t.Error("Should be focused after setting flag")
	}
}

// TestTextInputRuneSliceHandling tests rune slice operations
func TestTextInputRuneSliceHandling(t *testing.T) {
	ti := NewTextInput("", false, nil)

	// Complex Unicode string with combining characters
	complex := "café"
	ti.SetText(complex)

	// Verify runes are stored correctly
	if string(ti.runes) != complex {
		t.Errorf("Expected %q, got %q", complex, string(ti.runes))
	}

	// Verify rune count
	expectedRunes := len([]rune(complex))
	if len(ti.runes) != expectedRunes {
		t.Errorf("Expected %d runes, got %d", expectedRunes, len(ti.runes))
	}
}

// TestTextInputEmptyText tests empty string behavior
func TestTextInputEmptyText(t *testing.T) {
	ti := NewTextInput("placeholder", false, nil)

	// Initially empty
	if ti.Text() != "" {
		t.Error("Should start with empty text")
	}

	// Set and clear
	ti.SetText("something")
	ti.SetText("")

	if ti.Text() != "" {
		t.Error("Should be empty after SetText('')")
	}
}

// TestTextInputCursorPositionBounds tests cursor position boundary conditions
func TestTextInputCursorPositionBounds(t *testing.T) {
	ti := NewTextInput("", false, nil)
	ti.SetText("test")

	// Cursor should be valid position
	if ti.cursorPos > len(ti.runes) {
		t.Errorf("Cursor position %d exceeds text length %d", ti.cursorPos, len(ti.runes))
	}

	// After SetText, cursor position should be updated if necessary
	ti.cursorPos = 100
	ti.SetText("hi")
	if ti.cursorPos > len(ti.runes) {
		t.Errorf("Cursor position should be clamped, got %d", ti.cursorPos)
	}
}

// TestTextInputConsecutiveInsertions tests multiple insertions in sequence
func TestTextInputConsecutiveInsertions(t *testing.T) {
	changeCount := 0
	ti := NewTextInput("", false, func(s string) {
		changeCount++
	})

	// Simulate typing "hello"
	for _, ch := range "hello" {
		ti.insertRune(ch)
		ti.triggerChange()
	}

	if ti.Text() != "hello" {
		t.Errorf("Expected 'hello', got %q", ti.Text())
	}

	if changeCount != 5 {
		t.Errorf("Expected 5 onChange calls, got %d", changeCount)
	}

	if ti.cursorPos != 5 {
		t.Errorf("Expected cursor at 5, got %d", ti.cursorPos)
	}
}

// TestTextInputStateConsistency tests internal state consistency
func TestTextInputStateConsistency(t *testing.T) {
	ti := NewTextInput("", false, nil)
	ti.SetText("example")

	// Verify state is consistent
	if len(ti.runes) != len([]rune("example")) {
		t.Error("Rune slice length inconsistent")
	}

	if ti.Text() != string(ti.runes) {
		t.Error("Text() doesn't match runes")
	}

	// After insertions, should remain consistent
	ti.cursorPos = 3
	ti.insertRune('!')
	if ti.Text() != string(ti.runes) {
		t.Error("Text() doesn't match runes after insertion")
	}
}

type testWidgetContext struct {
	focused widget.Widget
}

var _ widget.Context = (*testWidgetContext)(nil)

func (c *testWidgetContext) RequestFocus(w widget.Widget)       { c.focused = w }
func (c *testWidgetContext) ReleaseFocus(w widget.Widget)       { if c.focused == w { c.focused = nil } }
func (c *testWidgetContext) IsFocused(w widget.Widget) bool     { return c.focused == w }
func (c *testWidgetContext) FocusedWidget() widget.Widget         { return c.focused }
func (c *testWidgetContext) Now() time.Time                       { return time.Now() }
func (c *testWidgetContext) DeltaTime() time.Duration             { return 0 }
func (c *testWidgetContext) Invalidate()                          {}
func (c *testWidgetContext) InvalidateRect(r geometry.Rect)       {}
func (c *testWidgetContext) Cursor() widget.CursorType            { return widget.CursorDefault }
func (c *testWidgetContext) SetCursor(cursor widget.CursorType)   {}
func (c *testWidgetContext) Scale() float32                       { return 1.0 }
func (c *testWidgetContext) ThemeProvider() widget.ThemeProvider   { return nil }
func (c *testWidgetContext) OverlayManager() widget.OverlayManager { return nil }
func (c *testWidgetContext) WindowSize() geometry.Size            { return geometry.Sz(800, 600) }
func (c *testWidgetContext) Scheduler() widget.SchedulerRef       { return nil }

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

	ti.selStart = 2
	ti.selEnd = 5

	ti.SetFocused(false)
	if ti.IsFocused() {
		t.Fatal("expected TextInput to blur after SetFocused(false)")
	}
	if ti.selStart != -1 || ti.selEnd != -1 {
		t.Fatal("expected selection to be reset after blur")
	}
}

func TestTextInputOutsideClickBlur(t *testing.T) {
	ti := NewTextInput("placeholder", false, nil)
	ti.SetBounds(geometry.NewRect(10, 10, 200, 30))
	ti.SetFocused(true)

	// Simulate mouse press outside bounds
	outsideEv := &event.MouseEvent{
		MouseType: event.MousePress,
		Position:  geometry.Pt(300, 300),
		Button:    event.ButtonLeft,
	}
	mockCtx := &testWidgetContext{}
	mockCtx.RequestFocus(ti)
	ti.Event(mockCtx, outsideEv)

	if ti.IsFocused() {
		t.Fatal("expected TextInput to lose focus when clicking outside bounds")
	}
	if mockCtx.IsFocused(ti) {
		t.Fatal("expected context to release focus when clicking outside bounds")
	}
}

