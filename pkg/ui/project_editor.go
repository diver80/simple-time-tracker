package ui

import (
	"fmt"
	"strconv"
	"strings"

	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// ProjectEditor is a modal dialog for creating a new project and customer.
type ProjectEditor struct {
	widget.WidgetBase

	repo            db.Repository
	onSaved         func()
	onCancel        func()
	onRequestRedraw func()

	customerInput *TextInput
	projectInput  *TextInput
	rateInput     *TextInput
	hoursInput    *TextInput

	saveBtn   *GlassButton
	cancelBtn *GlassButton

	errorMessage string
}

// NewProjectEditor creates a new project editor modal dialog.
func NewProjectEditor(repo db.Repository, onSaved func(), onCancel func()) *ProjectEditor {
	ed := &ProjectEditor{
		repo:     repo,
		onSaved:  onSaved,
		onCancel: onCancel,
	}

	ed.customerInput = NewTextInput("Kunde / Firma (z.B. Acme Corp)", false, nil)
	ed.projectInput = NewTextInput("Projektname (erforderlich)", false, nil)
	ed.rateInput = NewTextInput("Stundensatz in € (z.B. 120)", false, nil)
	ed.hoursInput = NewTextInput("Budget in Stunden (z.B. 40)", false, nil)

	ed.saveBtn = NewGlassButton("💾 Speichern", ed.handleSave).SetCompact(true)
	ed.cancelBtn = NewGlassButton("✕ Abbrechen", ed.handleCancel).SetCompact(true)

	ed.customerInput.SetParent(ed)
	ed.projectInput.SetParent(ed)
	ed.rateInput.SetParent(ed)
	ed.hoursInput.SetParent(ed)
	ed.saveBtn.SetParent(ed)
	ed.cancelBtn.SetParent(ed)

	ed.SetVisible(true)
	ed.SetEnabled(true)
	return ed
}

// SetOnRequestRedraw registers a callback to request redraws on state changes.
func (ed *ProjectEditor) SetOnRequestRedraw(fn func()) {
	ed.onRequestRedraw = fn
}

func (ed *ProjectEditor) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(380)
	h := c.ConstrainHeight(340)
	return geometry.Sz(w, h)
}

func (ed *ProjectEditor) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := ed.Bounds()
	theme := DefaultDarkTheme

	// Modal background card
	canvas.DrawRoundRect(b, theme.CardBg, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	// Title
	canvas.DrawText("Neues Projekt anlegen", geometry.NewRect(b.Min.X+16, b.Min.Y+14, b.Width()-32, 20), 14, theme.TextPrimary, true, widget.TextAlignLeft)

	y := b.Min.Y + 42

	// Customer Name
	canvas.DrawText("Kunde / Auftraggeber:", geometry.NewRect(b.Min.X+16, y, b.Width()-32, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.customerInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.customerInput.Draw(ctx, canvas)
	y += 36

	// Project Name
	canvas.DrawText("Projektname (Pflichtfeld):", geometry.NewRect(b.Min.X+16, y, b.Width()-32, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16
	ed.projectInput.SetBounds(geometry.NewRect(b.Min.X+16, y, b.Width()-32, 28))
	ed.projectInput.Draw(ctx, canvas)
	y += 36

	// Rate & Hours side-by-side
	gap := float32(12)
	halfW := (b.Width() - 32 - gap) / 2
	canvas.DrawText("Stundensatz (€/h):", geometry.NewRect(b.Min.X+16, y, halfW, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	canvas.DrawText("Budget (Stunden):", geometry.NewRect(b.Min.X+16+halfW+gap, y, halfW, 14), 10, theme.TextMuted, false, widget.TextAlignLeft)
	y += 16

	ed.rateInput.SetBounds(geometry.NewRect(b.Min.X+16, y, halfW, 28))
	ed.rateInput.Draw(ctx, canvas)

	ed.hoursInput.SetBounds(geometry.NewRect(b.Min.X+16+halfW+gap, y, halfW, 28))
	ed.hoursInput.Draw(ctx, canvas)
	y += 36

	// Error message
	if ed.errorMessage != "" {
		canvas.DrawText(ed.errorMessage, geometry.NewRect(b.Min.X+16, y, b.Width()-32, 20), 10, widget.RGBA8(255, 100, 100, 255), true, widget.TextAlignLeft)
	}

	// Action buttons at bottom
	actionY := b.Max.Y - 36
	ed.saveBtn.SetBounds(geometry.NewRect(b.Min.X+16, actionY, 100, 24))
	ed.saveBtn.Draw(ctx, canvas)

	ed.cancelBtn.SetBounds(geometry.NewRect(b.Min.X+126, actionY, 100, 24))
	ed.cancelBtn.Draw(ctx, canvas)
}

func (ed *ProjectEditor) Event(ctx widget.Context, e event.Event) bool {
	// Keyboard tab navigation cycling between inputs
	switch ev := e.(type) {
	case *event.KeyEvent:
		if ev.KeyType == event.KeyPress && ev.Key == event.KeyTab {
			inputs := []*TextInput{ed.customerInput, ed.projectInput, ed.rateInput, ed.hoursInput}
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

	// Forward events to inputs
	if ed.customerInput.Event(ctx, e) {
		return true
	}
	if ed.projectInput.Event(ctx, e) {
		return true
	}
	if ed.rateInput.Event(ctx, e) {
		return true
	}
	if ed.hoursInput.Event(ctx, e) {
		return true
	}

	// Forward events to action buttons
	if ed.saveBtn.Event(ctx, e) {
		return true
	}
	if ed.cancelBtn.Event(ctx, e) {
		return true
	}

	// Intercept other mouse clicks within modal to prevent click-through
	switch ev := e.(type) {
	case *event.MouseEvent:
		if ed.Bounds().Contains(ev.Position) {
			return true
		}
	}

	return false
}

func (ed *ProjectEditor) Children() []widget.Widget {
	return []widget.Widget{
		ed.customerInput,
		ed.projectInput,
		ed.rateInput,
		ed.hoursInput,
		ed.saveBtn,
		ed.cancelBtn,
	}
}

func (ed *ProjectEditor) handleSave() {
	ed.errorMessage = ""

	projName := strings.TrimSpace(ed.projectInput.Text())
	if projName == "" {
		ed.errorMessage = "Projektname ist erforderlich"
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	custName := strings.TrimSpace(ed.customerInput.Text())
	if custName == "" {
		custName = "Standard"
	}

	cust, err := ed.repo.CreateCustomerIfNotExists(custName)
	if err != nil {
		ed.errorMessage = fmt.Sprintf("Fehler beim Kunden: %v", err)
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	var rate float64
	rateStr := strings.TrimSpace(ed.rateInput.Text())
	if rateStr != "" {
		r, err := strconv.ParseFloat(rateStr, 64)
		if err != nil || r < 0 {
			ed.errorMessage = "Ungültiger Stundensatz"
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
		rate = r
	}

	var hours float64
	hoursStr := strings.TrimSpace(ed.hoursInput.Text())
	if hoursStr != "" {
		h, err := strconv.ParseFloat(hoursStr, 64)
		if err != nil || h < 0 {
			ed.errorMessage = "Ungültige Budgetstunden"
			if ed.onRequestRedraw != nil {
				ed.onRequestRedraw()
			}
			return
		}
		hours = h
	}

	budgetCost := rate * hours
	color := "#3B82F6"

	_, err = ed.repo.CreateProject(cust.ID, projName, rate, hours, budgetCost, color)
	if err != nil {
		ed.errorMessage = fmt.Sprintf("Fehler beim Erstellen: %v", err)
		if ed.onRequestRedraw != nil {
			ed.onRequestRedraw()
		}
		return
	}

	if ed.onSaved != nil {
		ed.onSaved()
	}
}

func (ed *ProjectEditor) handleCancel() {
	if ed.onCancel != nil {
		ed.onCancel()
	}
}
