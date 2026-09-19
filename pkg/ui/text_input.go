package ui

import (
	"unicode"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// TextInput is a reusable text input widget supporting single-line and multi-line editing.
// It uses rune slices for proper Unicode handling, supports text selection, masking,
// clipboard operations, and mouse-based cursor placement.
type TextInput struct {
	widget.WidgetBase

	placeholder string
	multiline   bool
	onChange    func(string)

	// Text state as rune slice for proper unicode handling
	runes []rune

	// Cursor and selection
	cursorPos int // position in runes
	selStart  int // selection start, -1 if no selection
	selEnd    int // selection end

	// Visual state
	isFocused    bool
	maskChar     rune // if != 0, mask input
	caretBlink   float32
	lastKeyTime  int64

	// Clipboard
	clipboardRead  func() (string, error)
	clipboardWrite func(string) error
}

// NewTextInput creates a new text input widget.
func NewTextInput(placeholder string, multiline bool, onChange func(string)) *TextInput {
	t := &TextInput{
		placeholder: placeholder,
		multiline:   multiline,
		onChange:    onChange,
		runes:       []rune{},
		cursorPos:   0,
		selStart:    -1,
		selEnd:      -1,
		isFocused:   false,
		maskChar:    0,
	}
	t.SetVisible(true)
	t.SetEnabled(true)
	return t
}

// Text returns the current text as a string.
func (t *TextInput) Text() string {
	return string(t.runes)
}

// SetText sets the text without calling onChange callback.
func (t *TextInput) SetText(text string) {
	t.runes = []rune(text)
	if t.cursorPos > len(t.runes) {
		t.cursorPos = len(t.runes)
	}
	t.selStart = -1
	t.selEnd = -1
}

// SetMasked sets whether input should be masked (like a password field).
func (t *TextInput) SetMasked(masked bool) {
	if masked {
		t.maskChar = '•'
	} else {
		t.maskChar = 0
	}
}

// SetClipboard sets custom clipboard read/write functions.
func (t *TextInput) SetClipboard(read func() (string, error), write func(string) error) {
	t.clipboardRead = read
	t.clipboardWrite = write
}

// IsFocusable returns true, allowing this widget to receive focus.
func (t *TextInput) IsFocusable() bool {
	return true
}

// Layout determines the size of the text input.
func (t *TextInput) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	width := c.ConstrainWidth(200)
	height := float32(28)
	if t.multiline {
		height = c.ConstrainHeight(120)
	}
	return geometry.Sz(width, height)
}

// Draw renders the text input widget.
func (t *TextInput) Draw(ctx widget.Context, canvas widget.Canvas) {
	bounds := t.Bounds()
	theme := DefaultDarkTheme

	// Background and border
	bg := theme.InputBg
	border := theme.InputBorder
	if t.isFocused {
		border = theme.AccentPrimary
	}

	canvas.DrawRoundRect(bounds, bg, 4)
	canvas.StrokeRoundRect(bounds, border, 4, 1.5)

	// Text rendering area with padding
	padding := float32(6)
	textBounds := geometry.NewRect(
		bounds.Min.X+padding,
		bounds.Min.Y+padding,
		bounds.Width()-2*padding,
		bounds.Height()-2*padding,
	)

	canvas.PushClip(textBounds)

	// Display text or placeholder
	displayText := t.Text()
	if displayText == "" && !t.isFocused {
		// Show placeholder
		canvas.DrawText(t.placeholder, textBounds, 11, theme.TextMuted, false, widget.TextAlignLeft)
	} else {
		// Show actual text (masked if needed)
		visibleText := displayText
		if t.maskChar != 0 {
			runes := make([]rune, len(t.runes))
			for i := range t.runes {
				runes[i] = t.maskChar
			}
			visibleText = string(runes)
		}

		canvas.DrawText(visibleText, textBounds, 11, theme.TextPrimary, false, widget.TextAlignLeft)

		// Draw selection background if focused
		if t.isFocused && t.selStart >= 0 && t.selStart != t.selEnd {
			selStart := t.selStart
			selEnd := t.selEnd
			if selStart > selEnd {
				selStart, selEnd = selEnd, selStart
			}

			// Measure text for selection highlighting
			textBefore := string(t.runes[:selStart])
			textSelected := string(t.runes[selStart:selEnd])

			widthBefore := canvas.MeasureText(textBefore, 11, false)
			widthSelected := canvas.MeasureText(textSelected, 11, false)

			selRect := geometry.NewRect(
				textBounds.Min.X+widthBefore,
				textBounds.Min.Y,
				widthSelected,
				textBounds.Height(),
			)
			selColor := widget.RGBA(theme.AccentPrimary.R, theme.AccentPrimary.G, theme.AccentPrimary.B, 0.4)
			canvas.DrawRect(selRect, selColor)
		}

		// Draw caret if focused
		if t.isFocused && t.selStart < 0 {
			textBefore := string(t.runes[:t.cursorPos])
			caretX := textBounds.Min.X + canvas.MeasureText(textBefore, 11, false)

			// Blinking caret
			t.caretBlink += 0.016 // ~60fps
			if t.caretBlink > 1.0 {
				t.caretBlink = 0
			}

			if t.caretBlink < 0.5 {
				canvas.DrawLine(
					geometry.Pt(caretX, textBounds.Min.Y+2),
					geometry.Pt(caretX, textBounds.Max.Y-2),
					theme.AccentPrimary,
					1.5,
				)
			}
		}
	}

	canvas.PopClip()
}

// Event handles keyboard and mouse events.
func (t *TextInput) Event(ctx widget.Context, e event.Event) bool {
	switch ev := e.(type) {
	case *event.MouseEvent:
		return t.handleMouseEvent(ctx, ev)
	case *event.KeyEvent:
		return t.handleKeyEvent(ctx, ev)
	case *event.FocusEvent:
		return t.handleFocusEvent(ev)
	}
	return false
}

func (t *TextInput) handleMouseEvent(ctx widget.Context, ev *event.MouseEvent) bool {
	bounds := t.Bounds()

	switch ev.MouseType {
	case event.MousePress:
		if bounds.Contains(ev.Position) {
			ctx.RequestFocus(t)
			t.isFocused = true

			// Calculate cursor position from click using approximate character widths
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

			// Find rune position using simple estimation (6px per char at 11pt)
			// This will be refined by the Draw() method which has access to canvas.MeasureText()
			estimatedPos := int(clickX / 6)
			if estimatedPos > len(t.runes) {
				estimatedPos = len(t.runes)
			}
			t.cursorPos = estimatedPos

			t.selStart = -1
			t.selEnd = -1
			ctx.Invalidate()
			return true
		}

	case event.MouseMove:
		if bounds.Contains(ev.Position) {
			ctx.SetCursor(widget.CursorText)
			return true
		}
	}

	return false
}

func (t *TextInput) handleKeyEvent(ctx widget.Context, ev *event.KeyEvent) bool {
	if !t.isFocused || ev.KeyType != event.KeyPress {
		return false
	}

	modifiers := ev.Modifiers()

	// Handle special keys
	switch ev.Key {
	case event.KeyEnter:
		if t.multiline {
			t.insertRune('\n')
			t.triggerChange()
			ctx.Invalidate()
			return true
		}
		return false

	case event.KeyBackspace:
		t.deleteSelection()
		if t.cursorPos > 0 {
			t.cursorPos--
			t.runes = append(t.runes[:t.cursorPos], t.runes[t.cursorPos+1:]...)
			t.triggerChange()
			ctx.Invalidate()
		}
		t.selStart = -1
		t.selEnd = -1
		return true

	case event.KeyDelete:
		t.deleteSelection()
		if t.cursorPos < len(t.runes) {
			t.runes = append(t.runes[:t.cursorPos], t.runes[t.cursorPos+1:]...)
			t.triggerChange()
			ctx.Invalidate()
		}
		t.selStart = -1
		t.selEnd = -1
		return true

	case event.KeyLeft:
		if modifiers.Has(event.ModShift) {
			if t.selStart < 0 {
				t.selStart = t.cursorPos
				t.selEnd = t.cursorPos
			}
			if t.cursorPos > 0 {
				t.cursorPos--
				t.selEnd = t.cursorPos
			}
		} else {
			t.selStart = -1
			t.selEnd = -1
			if t.cursorPos > 0 {
				t.cursorPos--
			}
		}
		ctx.Invalidate()
		return true

	case event.KeyRight:
		if modifiers.Has(event.ModShift) {
			if t.selStart < 0 {
				t.selStart = t.cursorPos
				t.selEnd = t.cursorPos
			}
			if t.cursorPos < len(t.runes) {
				t.cursorPos++
				t.selEnd = t.cursorPos
			}
		} else {
			t.selStart = -1
			t.selEnd = -1
			if t.cursorPos < len(t.runes) {
				t.cursorPos++
			}
		}
		ctx.Invalidate()
		return true

	case event.KeyUp:
		if t.multiline {
			if t.cursorPos > 0 {
				t.cursorPos = 0
			}
			t.selStart = -1
			t.selEnd = -1
			ctx.Invalidate()
			return true
		}

	case event.KeyDown:
		if t.multiline {
			if t.cursorPos < len(t.runes) {
				t.cursorPos = len(t.runes)
			}
			t.selStart = -1
			t.selEnd = -1
			ctx.Invalidate()
			return true
		}

	case event.KeyHome:
		t.cursorPos = 0
		t.selStart = -1
		t.selEnd = -1
		ctx.Invalidate()
		return true

	case event.KeyEnd:
		t.cursorPos = len(t.runes)
		t.selStart = -1
		t.selEnd = -1
		ctx.Invalidate()
		return true

	case event.KeyTab:
		// Allow tab for focus navigation
		return false
	}

	// Handle Ctrl+A / Cmd+A (select all)
	if modifiers.Has(event.ModCtrl) || modifiers.Has(event.ModSuper) {
		switch ev.Key {
		case event.KeyA:
			t.selStart = 0
			t.selEnd = len(t.runes)
			ctx.Invalidate()
			return true
		case event.KeyC:
			// Copy
			if t.selStart >= 0 && t.selStart != t.selEnd {
				start, end := t.getSelection()
				text := string(t.runes[start:end])
				if t.clipboardWrite != nil {
					_ = t.clipboardWrite(text)
				}
				return true
			}
			return false
		case event.KeyX:
			// Cut
			if t.selStart >= 0 && t.selStart != t.selEnd {
				start, end := t.getSelection()
				text := string(t.runes[start:end])
				if t.clipboardWrite != nil {
					_ = t.clipboardWrite(text)
				}
				t.deleteSelection()
				t.triggerChange()
				ctx.Invalidate()
				return true
			}
			return false
		case event.KeyV:
			// Paste
			if t.clipboardRead != nil {
				if text, err := t.clipboardRead(); err == nil {
					t.deleteSelection()
					for _, r := range text {
						if t.multiline || r != '\n' {
							t.insertRune(r)
						}
					}
					t.triggerChange()
					ctx.Invalidate()
					return true
				}
			}
			return false
		}
	}

	// Handle regular rune input
	if ev.Rune != 0 && unicode.IsGraphic(ev.Rune) {
		t.deleteSelection()
		t.insertRune(ev.Rune)
		t.triggerChange()
		ctx.Invalidate()
		return true
	}

	return false
}

func (t *TextInput) handleFocusEvent(ev *event.FocusEvent) bool {
	if ev.IsGained() {
		t.isFocused = true
		t.selStart = -1
		t.selEnd = -1
		return true
	} else if ev.IsLost() {
		t.isFocused = false
		t.selStart = -1
		t.selEnd = -1
		return true
	}
	return false
}

// Children returns nil as TextInput has no child widgets.
func (t *TextInput) Children() []widget.Widget {
	return nil
}

// Helper methods

func (t *TextInput) insertRune(r rune) {
	if t.cursorPos < 0 {
		t.cursorPos = 0
	}
	if t.cursorPos > len(t.runes) {
		t.cursorPos = len(t.runes)
	}

	t.runes = append(t.runes[:t.cursorPos], append([]rune{r}, t.runes[t.cursorPos:]...)...)
	t.cursorPos++
}

func (t *TextInput) deleteSelection() {
	if t.selStart >= 0 && t.selStart != t.selEnd {
		start, end := t.getSelection()
		t.runes = append(t.runes[:start], t.runes[end:]...)
		t.cursorPos = start
	}
	t.selStart = -1
	t.selEnd = -1
}

func (t *TextInput) getSelection() (int, int) {
	start := t.selStart
	end := t.selEnd
	if start > end {
		start, end = end, start
	}
	return start, end
}

func (t *TextInput) triggerChange() {
	if t.onChange != nil {
		t.onChange(t.Text())
	}
}
