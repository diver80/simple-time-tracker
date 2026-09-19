package ui

import (
	"fmt"

	"yokto-time/pkg/db"

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

	addSampleBtn *GlassButton
}

func NewProjectView(repo db.Repository, onRequestRedraw func()) *ProjectView {
	pv := &ProjectView{
		repo:            repo,
		projectHours:    make(map[int64]float64),
		projectCost:     make(map[int64]float64),
		onRequestRedraw: onRequestRedraw,
	}
	pv.SetVisible(true)
	pv.SetEnabled(true)

	pv.addSampleBtn = NewGlassButton("+ Beispiel-Kunden & Projekte anlegen", func() {
		pv.seedSampleData()
		pv.Refresh()
	}).SetCompact(true)

	pv.Refresh()
	return pv
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
	canvas.DrawText("Kunden & Projektbudgets", geometry.NewRect(b.Min.X+20, topY, 200, 20), 14, theme.TextPrimary, true, widget.TextAlignLeft)

	if len(pv.projects) == 0 {
		pv.addSampleBtn.SetBounds(geometry.NewRect(b.Max.X-240, topY, 220, 24))
		pv.addSampleBtn.Draw(ctx, canvas)

		emptyY := topY + 80
		canvas.DrawText("Noch keine Projekte vorhanden.", geometry.NewRect(b.Min.X+20, emptyY, b.Width()-40, 20), 12, theme.TextMuted, false, widget.TextAlignCenter)
		canvas.DrawText("Klicke oben auf '+ Beispiel-Kunden anlegen', um Testprojekte zu laden.", geometry.NewRect(b.Min.X+20, emptyY+24, b.Width()-40, 20), 11, theme.TextSecondary, false, widget.TextAlignCenter)
		return
	}

	listY := topY + 36
	for i, p := range pv.projects {
		if i >= 5 {
			break // Display up to 5 cards in compact window
		}
		cardH := float32(82)
		cardRect := geometry.NewRect(b.Min.X+16, listY, b.Width()-32, cardH)

		canvas.DrawRoundRect(cardRect, theme.InputBg, 8)
		canvas.StrokeRoundRect(cardRect, theme.InputBorder, 8, 1.0)

		// Color indicator bar
		stripRect := geometry.NewRect(cardRect.Min.X, cardRect.Min.Y, 4, cardH)
		canvas.DrawRoundRect(stripRect, ParseHexColor(p.Color), 2)

		// Project & Customer title
		title := fmt.Sprintf("[%s] %s", p.CustomerName, p.Name)
		canvas.DrawText(title, geometry.NewRect(cardRect.Min.X+12, cardRect.Min.Y+8, cardRect.Width()-24, 16), 11, theme.TextPrimary, true, widget.TextAlignLeft)

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

		listY += cardH + 10
	}
}

func (pv *ProjectView) Event(ctx widget.Context, e event.Event) bool {
	if len(pv.projects) == 0 {
		if pv.addSampleBtn.Event(ctx, e) {
			return true
		}
	}
	return false
}

func (pv *ProjectView) Children() []widget.Widget {
	return nil
}
