package ui

import (
	"fmt"

	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// ProjectView allows viewing and managing Customers and Projects with budget bars.
type ProjectView struct {
	widget.WidgetBase

	repo            db.Repository
	customers       []db.Customer
	projects        []db.Project
	projectHours    map[int64]float64
	projectCost     map[int64]float64
	onRequestRedraw func()

	addProjectBtn   *GlassButton
	clearDemoBtn    *GlassButton
	addSampleBtn    *GlassButton

	confirmDeleteID int64
	confirmYesBtn   *GlassButton
	confirmNoBtn    *GlassButton

	deleteBtnBounds map[int64]geometry.Rect
	projectEditor   *ProjectEditor
}

func NewProjectView(repo db.Repository, onRequestRedraw func()) *ProjectView {
	pv := &ProjectView{
		repo:            repo,
		projectHours:    make(map[int64]float64),
		projectCost:     make(map[int64]float64),
		onRequestRedraw: onRequestRedraw,
		deleteBtnBounds: make(map[int64]geometry.Rect),
	}
	pv.SetVisible(true)
	pv.SetEnabled(true)

	pv.addProjectBtn = NewGlassButton("+ Projekt", func() {
		pv.openProjectEditor()
	}).SetCompact(true)

	pv.clearDemoBtn = NewGlassButton("Demodaten leeren", func() {
		pv.handleClearDemo()
	}).SetCompact(true)

	pv.addSampleBtn = NewGlassButton("+ Beispiel-Kunden & Projekte anlegen", func() {
		pv.seedSampleData()
		pv.Refresh()
	}).SetCompact(true)

	pv.confirmYesBtn = NewGlassButton("Ja", func() {
		pv.confirmDelete()
	}).SetCompact(true)

	pv.confirmNoBtn = NewGlassButton("Nein", func() {
		pv.cancelDelete()
	}).SetCompact(true)

	// Set parents for dirty redraw propagation
	pv.addProjectBtn.SetParent(pv)
	pv.clearDemoBtn.SetParent(pv)
	pv.addSampleBtn.SetParent(pv)
	pv.confirmYesBtn.SetParent(pv)
	pv.confirmNoBtn.SetParent(pv)

	pv.Refresh()
	return pv
}

func (pv *ProjectView) openProjectEditor() {
	pv.projectEditor = NewProjectEditor(pv.repo, func() {
		pv.projectEditor = nil
		pv.Refresh()
	}, func() {
		pv.projectEditor = nil
		if pv.onRequestRedraw != nil {
			pv.onRequestRedraw()
		}
	})
	pv.projectEditor.SetParent(pv)
	if pv.onRequestRedraw != nil {
		pv.onRequestRedraw()
	}
}

func (pv *ProjectView) hasDemoData() bool {
	demoNames := map[string]bool{
		"Acme Corporation": true,
		"Starlight Media":  true,
		"Acme Corp":        true,
	}
	for _, c := range pv.customers {
		if demoNames[c.Name] {
			return true
		}
	}
	return false
}

func (pv *ProjectView) handleClearDemo() {
	_ = pv.repo.ClearAllDemoData()
	pv.confirmDeleteID = 0
	pv.Refresh()
}

func (pv *ProjectView) requestDeleteProject(id int64) {
	pv.confirmDeleteID = id
	if pv.onRequestRedraw != nil {
		pv.onRequestRedraw()
	}
}

func (pv *ProjectView) confirmDelete() {
	if pv.confirmDeleteID != 0 {
		_ = pv.repo.DeleteProject(pv.confirmDeleteID)
		pv.confirmDeleteID = 0
		pv.Refresh()
	}
}

func (pv *ProjectView) cancelDelete() {
	pv.confirmDeleteID = 0
	if pv.onRequestRedraw != nil {
		pv.onRequestRedraw()
	}
}

func (pv *ProjectView) seedSampleData() {
	cust1, _ := pv.repo.CreateCustomer("Acme Corp")
	if cust1 != nil {
		_, _ = pv.repo.CreateProject(cust1.ID, "Website Redesign", 120.0, 40.0, 4800.0, "#3B82F6")
		_, _ = pv.repo.CreateProject(cust1.ID, "SEO & Performance", 110.0, 20.0, 2200.0, "#10B981")
	}

	cust2, _ := pv.repo.CreateCustomer("Starlight Media")
	if cust2 != nil {
		_, _ = pv.repo.CreateProject(cust2.ID, "iOS App Entwicklung", 140.0, 80.0, 11200.0, "#8B5CF6")
	}
}

func (pv *ProjectView) Refresh() {
	custs, _ := pv.repo.ListCustomers()
	pv.customers = custs

	projs, _ := pv.repo.ListProjects(nil)
	pv.projects = projs

	for _, p := range projs {
		h, c, _ := pv.repo.GetProjectBudgetSummary(p.ID)
		pv.projectHours[p.ID] = h
		pv.projectCost[p.ID] = c
	}

	if pv.onRequestRedraw != nil {
		pv.onRequestRedraw()
	}
}

func (pv *ProjectView) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(420)
	h := c.ConstrainHeight(560)
	return geometry.Sz(w, h)
}

func (pv *ProjectView) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := pv.Bounds()
	theme := DefaultDarkTheme

	canvas.DrawRoundRect(b, theme.CardBg, 12)
	canvas.StrokeRoundRect(b, theme.CardBorder, 12, 1.0)

	topY := b.Min.Y + 16
	canvas.DrawText("Kunden & Projektbudgets", geometry.NewRect(b.Min.X+16, topY, 170, 24), 13, theme.TextPrimary, true, widget.TextAlignLeft)

	// Header action buttons on top right
	btnRight := b.Max.X - 16
	addBtnW := float32(76)
	pv.addProjectBtn.SetBounds(geometry.NewRect(btnRight-addBtnW, topY, addBtnW, 24))
	pv.addProjectBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.AccentPrimary, theme.CardBorder)
	pv.addProjectBtn.Draw(ctx, canvas)
	btnRight -= addBtnW + 8

	if pv.hasDemoData() {
		clearBtnW := float32(116)
		pv.clearDemoBtn.SetBounds(geometry.NewRect(btnRight-clearBtnW, topY, clearBtnW, 24))
		pv.clearDemoBtn.SetCustomColors(theme.QuickShiftColor, widget.RGBA8(44, 34, 18, 255), widget.RGBA8(217, 119, 6, 255))
		pv.clearDemoBtn.Draw(ctx, canvas)
	}

	pv.deleteBtnBounds = make(map[int64]geometry.Rect)

	if len(pv.projects) == 0 {
		emptyY := topY + 70
		canvas.DrawText("Noch keine Projekte vorhanden.", geometry.NewRect(b.Min.X+20, emptyY, b.Width()-40, 20), 12, theme.TextMuted, false, widget.TextAlignCenter)
		canvas.DrawText("Klicke oben auf '+ Projekt', um ein Projekt anzulegen,\noder lade Testdaten:", geometry.NewRect(b.Min.X+20, emptyY+24, b.Width()-40, 32), 11, theme.TextSecondary, false, widget.TextAlignCenter)

		sampleBtnW := float32(230)
		pv.addSampleBtn.SetBounds(geometry.NewRect(b.Min.X+(b.Width()-sampleBtnW)/2, emptyY+68, sampleBtnW, 26))
		pv.addSampleBtn.Draw(ctx, canvas)
	} else {
		listY := topY + 36
		for _, p := range pv.projects {
			cardH := float32(82)
			if listY+cardH > b.Max.Y-10 {
				break // Display cards fitting in view bounds
			}
			cardRect := geometry.NewRect(b.Min.X+16, listY, b.Width()-32, cardH)

			if pv.confirmDeleteID == p.ID {
				// Confirmation state
				canvas.DrawRoundRect(cardRect, widget.RGBA8(42, 24, 28, 255), 8)
				canvas.StrokeRoundRect(cardRect, theme.StopColor, 8, 1.0)

				stripRect := geometry.NewRect(cardRect.Min.X, cardRect.Min.Y, 4, cardH)
				canvas.DrawRoundRect(stripRect, theme.StopColor, 2)

				prompt := fmt.Sprintf("Projekt '%s' wirklich löschen?", p.Name)
				canvas.DrawText(prompt, geometry.NewRect(cardRect.Min.X+14, cardRect.Min.Y+12, cardRect.Width()-28, 16), 11, theme.TextPrimary, true, widget.TextAlignLeft)
				canvas.DrawText("Bestehende Zeiteinträge bleiben sicher erhalten.", geometry.NewRect(cardRect.Min.X+14, cardRect.Min.Y+30, cardRect.Width()-28, 14), 10, theme.TextSecondary, false, widget.TextAlignLeft)

				pv.confirmYesBtn.SetBounds(geometry.NewRect(cardRect.Min.X+14, cardRect.Min.Y+50, 60, 22))
				pv.confirmYesBtn.SetCustomColors(widget.RGBA8(255, 255, 255, 255), theme.StopColor, widget.RGBA8(239, 68, 68, 255))
				pv.confirmYesBtn.Draw(ctx, canvas)

				pv.confirmNoBtn.SetBounds(geometry.NewRect(cardRect.Min.X+82, cardRect.Min.Y+50, 60, 22))
				pv.confirmNoBtn.Draw(ctx, canvas)
			} else {
				// Normal project card
				canvas.DrawRoundRect(cardRect, theme.InputBg, 8)
				canvas.StrokeRoundRect(cardRect, theme.InputBorder, 8, 1.0)

				// Color indicator bar
				stripRect := geometry.NewRect(cardRect.Min.X, cardRect.Min.Y, 4, cardH)
				canvas.DrawRoundRect(stripRect, ParseHexColor(p.Color), 2)

				// Delete button (✕)
				delRect := geometry.NewRect(cardRect.Max.X-24, cardRect.Min.Y+6, 18, 18)
				pv.deleteBtnBounds[p.ID] = delRect
				canvas.DrawRoundRect(delRect, widget.RGBA8(46, 52, 68, 200), 4)
				canvas.DrawText("✕", delRect, 10, widget.RGBA8(239, 68, 68, 255), true, widget.TextAlignCenter)

				// Project & Customer title
				title := fmt.Sprintf("[%s] %s", p.CustomerName, p.Name)
				canvas.DrawText(title, geometry.NewRect(cardRect.Min.X+12, cardRect.Min.Y+8, cardRect.Width()-42, 16), 11, theme.TextPrimary, true, widget.TextAlignLeft)

				rateStr := fmt.Sprintf("Stundensatz: %.0f €/h", p.HourlyRate)
				canvas.DrawText(rateStr, geometry.NewRect(cardRect.Min.X+12, cardRect.Min.Y+26, 140, 14), 10, theme.TextSecondary, false, widget.TextAlignLeft)

				// Hours & Budget
				usedHours := pv.projectHours[p.ID]
				budgetStr := fmt.Sprintf("%.1f / %.0f Std.", usedHours, p.BudgetHours)
				if p.BudgetHours <= 0 {
					budgetStr = fmt.Sprintf("%.1f Std. (ohne Limit)", usedHours)
				}
				canvas.DrawText(budgetStr, geometry.NewRect(cardRect.Max.X-160, cardRect.Min.Y+26, 148, 14), 10, theme.TextPrimary, true, widget.TextAlignRight)

				// Progress Bar
				barY := cardRect.Min.Y + 48
				barW := cardRect.Width() - 24
				barBg := geometry.NewRect(cardRect.Min.X+12, barY, barW, 8)
				canvas.DrawRoundRect(barBg, widget.RGBA8(45, 52, 68, 255), 4)

				pct := float32(0.0)
				if p.BudgetHours > 0 {
					pct = float32(usedHours / p.BudgetHours)
					if pct > 1.0 {
						pct = 1.0
					}
				}
				if pct > 0 {
					fillColor := theme.AccentPrimary
					if pct >= 0.85 {
						fillColor = theme.QuickShiftColor
					}
					if pct >= 1.0 {
						fillColor = theme.StopColor
					}
					fillW := barW * pct
					if fillW < 8 {
						fillW = 8
					}
					fillBar := geometry.NewRect(cardRect.Min.X+12, barY, fillW, 8)
					canvas.DrawRoundRect(fillBar, fillColor, 4)
				}
			}

			listY += cardH + 10
		}
	}

	// Modal editor overlay
	if pv.projectEditor != nil {
		canvas.DrawRoundRect(b, widget.RGBA8(10, 14, 22, 215), 12)

		modalW := float32(380)
		modalH := float32(340)
		modalX := b.Min.X + (b.Width()-modalW)/2
		modalY := b.Min.Y + (b.Height()-modalH)/2
		pv.projectEditor.SetBounds(geometry.NewRect(modalX, modalY, modalW, modalH))
		pv.projectEditor.Draw(ctx, canvas)
	}
}

func (pv *ProjectView) Event(ctx widget.Context, e event.Event) bool {
	// If modal editor is open, route events exclusively to it
	if pv.projectEditor != nil {
		if pv.projectEditor.Event(ctx, e) {
			return true
		}
		// Absorb mouse clicks while modal is open
		if _, ok := e.(*event.MouseEvent); ok {
			return true
		}
		return true
	}

	// Confirmation buttons for delete
	if pv.confirmDeleteID != 0 {
		if pv.confirmYesBtn.Event(ctx, e) {
			return true
		}
		if pv.confirmNoBtn.Event(ctx, e) {
			return true
		}
	}

	// Header actions
	if pv.addProjectBtn.Event(ctx, e) {
		return true
	}
	if pv.hasDemoData() && pv.clearDemoBtn.Event(ctx, e) {
		return true
	}

	// Empty state button
	if len(pv.projects) == 0 {
		if pv.addSampleBtn.Event(ctx, e) {
			return true
		}
	}

	// Check card delete buttons
	switch ev := e.(type) {
	case *event.MouseEvent:
		if ev.MouseType == event.MousePress {
			for id, rect := range pv.deleteBtnBounds {
				if rect.Contains(ev.Position) {
					pv.requestDeleteProject(id)
					return true
				}
			}
		}
	}

	return false
}

func (pv *ProjectView) Children() []widget.Widget {
	if pv.projectEditor != nil {
		return []widget.Widget{pv.projectEditor}
	}

	children := []widget.Widget{
		pv.addProjectBtn,
	}
	if pv.hasDemoData() {
		children = append(children, pv.clearDemoBtn)
	}
	if len(pv.projects) == 0 {
		children = append(children, pv.addSampleBtn)
	}
	if pv.confirmDeleteID != 0 {
		children = append(children, pv.confirmYesBtn, pv.confirmNoBtn)
	}
	return children
}
