package ui

import (
	"fmt"
	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

func formatProjectLabel(p *db.Project) string {
	if p == nil {
		return "(Kein Projekt)"
	}
	if p.CustomerName != "" {
		return fmt.Sprintf("%s - %s", p.CustomerName, p.Name)
	}
	return p.Name
}

// ProjectPicker is a dropdown-like widget for selecting a project.
type ProjectPicker struct {
	widget.WidgetBase

	repo            db.Repository
	onChange        func(*db.Project)
	selectedProject *db.Project
	projects        []db.Project

	isExpanded bool
}

// NewProjectPicker creates a new ProjectPicker widget.
func NewProjectPicker(repo db.Repository, onChange func(*db.Project)) *ProjectPicker {
	pp := &ProjectPicker{
		repo:     repo,
		onChange: onChange,
	}
	pp.SetVisible(true)
	pp.SetEnabled(true)
	pp.Refresh()
	return pp
}

// SelectedProject returns the currently selected project.
func (pp *ProjectPicker) SelectedProject() *db.Project {
	return pp.selectedProject
}

// SetProjectID sets the selected project by ID.
func (pp *ProjectPicker) SetProjectID(id *int64) {
	if id == nil {
		pp.selectedProject = nil
		return
	}
	for i := range pp.projects {
		if pp.projects[i].ID == *id {
			pp.selectedProject = &pp.projects[i]
			return
		}
	}
	pp.selectedProject = nil
}

// Refresh reloads the project list from the repository.
func (pp *ProjectPicker) Refresh() error {
	projects, err := pp.repo.ListProjects(nil)
	if err != nil {
		return err
	}
	pp.projects = projects
	return nil
}

func (pp *ProjectPicker) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.ConstrainWidth(200)
	h := float32(28)
	if pp.isExpanded {
		h = c.ConstrainHeight(float32(28 + len(pp.projects)*24))
	}
	return geometry.Sz(w, h)
}

func (pp *ProjectPicker) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := pp.Bounds()
	theme := DefaultDarkTheme

	// Main button background
	btnRect := geometry.NewRect(b.Min.X, b.Min.Y, b.Width(), 28)
	canvas.DrawRoundRect(btnRect, theme.InputBg, 4)
	canvas.StrokeRoundRect(btnRect, theme.InputBorder, 4, 1.0)

	// Display selected project or placeholder
	displayText := "(Kein Projekt)"
	if pp.selectedProject != nil {
		displayText = pp.selectedProject.Name
	}
	canvas.DrawText(displayText, geometry.NewRect(b.Min.X+6, b.Min.Y+4, b.Width()-20, 20), 11, theme.TextPrimary, false, widget.TextAlignLeft)

	// Dropdown arrow
	arrowText := "v"
	if pp.isExpanded {
		arrowText = "^"
	}
	canvas.DrawText(arrowText, geometry.NewRect(b.Max.X-18, b.Min.Y+4, 12, 20), 11, theme.TextPrimary, false, widget.TextAlignCenter)

	// Dropdown menu if expanded
	if pp.isExpanded {
		menuY := b.Min.Y + 30
		for i, proj := range pp.projects {
			itemRect := geometry.NewRect(b.Min.X, menuY+float32(i*24), b.Width(), 24)
			itemBg := theme.InputBg
			if pp.selectedProject != nil && pp.selectedProject.ID == proj.ID {
				itemBg = widget.RGBA8(60, 72, 98, 255)
			}
			canvas.DrawRect(itemRect, itemBg)
			canvas.DrawText(proj.Name, geometry.NewRect(itemRect.Min.X+8, itemRect.Min.Y+4, itemRect.Width()-16, 16), 10, theme.TextPrimary, false, widget.TextAlignLeft)
		}
	}
}

func (pp *ProjectPicker) Event(ctx widget.Context, e event.Event) bool {
	switch ev := e.(type) {
	case *event.MouseEvent:
		b := pp.Bounds()
		btnRect := geometry.NewRect(b.Min.X, b.Min.Y, b.Width(), 28)

		if ev.MouseType == event.MousePress && btnRect.Contains(ev.Position) {
			pp.isExpanded = !pp.isExpanded
			return true
		}

		if pp.isExpanded && ev.MouseType == event.MousePress {
			menuY := b.Min.Y + 30
			for i := range pp.projects {
				itemRect := geometry.NewRect(b.Min.X, menuY+float32(i*24), b.Width(), 24)
				if itemRect.Contains(ev.Position) {
					pp.selectedProject = &pp.projects[i]
					pp.isExpanded = false
					if pp.onChange != nil {
						pp.onChange(pp.selectedProject)
					}
					return true
				}
			}
		}
	}
	return false
}

func (pp *ProjectPicker) Children() []widget.Widget {
	return nil
}
