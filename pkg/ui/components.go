package ui

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// GlassButton is an interactive styled button with hover and press effects.
type GlassButton struct {
	widget.WidgetBase
	text         string
	onClick      func()
	isHovered    bool
	isPressed    bool
	customBg     *widget.Color
	customFg     *widget.Color
	customBorder *widget.Color
	compact      bool
}

func NewGlassButton(text string, onClick func()) *GlassButton {
	b := &GlassButton{
		text:    text,
		onClick: onClick,
	}
	b.SetVisible(true)
	b.SetEnabled(true)
	return b
}

func (b *GlassButton) SetCompact(compact bool) *GlassButton {
	b.compact = compact
	return b
}

func (b *GlassButton) SetText(text string) {
	b.text = text
}

func (b *GlassButton) SetCustomColors(fg, bg, border widget.Color) *GlassButton {
	b.customFg = &fg
	b.customBg = &bg
	b.customBorder = &border
	return b
}

func (b *GlassButton) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	width := float32(len(b.text)*8 + 24)
	if width < 48 {
		width = 48
	}
	height := float32(28)
	if b.compact {
		height = 22
		width = float32(len(b.text)*7 + 16)
	}
	return c.Constrain(geometry.Sz(width, height))
}

func (b *GlassButton) Draw(ctx widget.Context, canvas widget.Canvas) {
	bounds := b.Bounds()
	radius := float32(6)

	bg := widget.RGBA8(40, 46, 60, 255)
	border := widget.RGBA8(70, 78, 98, 255)
	fg := widget.RGBA8(240, 244, 250, 255)

	if b.customBg != nil {
		bg = *b.customBg
	}
	if b.customBorder != nil {
		border = *b.customBorder
	}
	if b.customFg != nil {
		fg = *b.customFg
	}

	if b.isHovered {
		bg = widget.RGBA(bg.R*1.15, bg.G*1.15, bg.B*1.15, bg.A)
		border = widget.RGBA(border.R*1.2, border.G*1.2, border.B*1.2, border.A)
	}
	if b.isPressed {
		bg = widget.RGBA(bg.R*0.85, bg.G*0.85, bg.B*0.85, bg.A)
	}

	canvas.DrawRoundRect(bounds, bg, radius)
	canvas.StrokeRoundRect(bounds, border, radius, 1.0)

	fontSize := float32(11)
	if b.compact {
		fontSize = 10
	}
	textBounds := geometry.NewRect(bounds.Min.X+4, bounds.Min.Y+4, bounds.Width()-8, bounds.Height()-8)
	canvas.DrawText(b.text, textBounds, fontSize, fg, false, widget.TextAlignCenter)
}

func (b *GlassButton) Event(ctx widget.Context, e event.Event) bool {
	switch ev := e.(type) {
	case *event.MouseEvent:
		contains := b.Bounds().Contains(ev.Position)
		if ev.MouseType == event.MouseMove {
			if contains != b.isHovered {
				b.isHovered = contains
				return true
			}
		} else if ev.MouseType == event.MousePress && contains {
			b.isPressed = true
			if b.onClick != nil {
				b.onClick()
			}
			return true
		} else if ev.MouseType == event.MouseRelease {
			if b.isPressed {
				b.isPressed = false
				return true
			}
		}
	}
	return false
}

func (b *GlassButton) Children() []widget.Widget {
	return nil
}

// PillBadge renders a rounded pill badge with background and text.
type PillBadge struct {
	widget.WidgetBase
	text  string
	color widget.Color
}

func NewPillBadge(text string, color widget.Color) *PillBadge {
	p := &PillBadge{text: text, color: color}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

func (p *PillBadge) SetText(text string) {
	p.text = text
}

func (p *PillBadge) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	width := float32(len(p.text)*7 + 16)
	if width < 32 {
		width = 32
	}
	return c.Constrain(geometry.Sz(width, 18))
}

func (p *PillBadge) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := p.Bounds()
	radius := float32(9)

	bg := widget.RGBA(p.color.R*0.2, p.color.G*0.2, p.color.B*0.2, 0.85)
	border := widget.RGBA(p.color.R, p.color.G, p.color.B, 0.7)

	canvas.DrawRoundRect(b, bg, radius)
	canvas.StrokeRoundRect(b, border, radius, 1.0)

	textBounds := geometry.NewRect(b.Min.X+4, b.Min.Y+2, b.Width()-8, b.Height()-4)
	canvas.DrawText(p.text, textBounds, 10, p.color, false, widget.TextAlignCenter)
}

func (p *PillBadge) Event(ctx widget.Context, e event.Event) bool {
	return false
}

func (p *PillBadge) Children() []widget.Widget {
	return nil
}
