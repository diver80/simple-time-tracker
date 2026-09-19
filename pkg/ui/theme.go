package ui

import (
	"github.com/gogpu/ui/widget"
)

// AppTheme defines styling colors for the Yokto-inspired minimalist UI.
type AppTheme struct {
	Background      widget.Color
	CardBg          widget.Color
	CardBorder      widget.Color
	TextPrimary     widget.Color
	TextSecondary   widget.Color
	TextMuted       widget.Color
	AccentPrimary   widget.Color // Emerald/Indigo
	AccentHover     widget.Color
	StopColor       widget.Color // Coral Red
	QuickShiftColor widget.Color // Amber
	QuickShiftBg    widget.Color
	InputBorder     widget.Color
	InputBg         widget.Color
	TabActive       widget.Color
	TabInactive     widget.Color
	LineSeparator   widget.Color
}

var DefaultDarkTheme = AppTheme{
	Background:      widget.RGBA8(18, 20, 26, 248),
	CardBg:          widget.RGBA8(28, 32, 42, 220),
	CardBorder:      widget.RGBA8(50, 56, 72, 200),
	TextPrimary:     widget.RGBA8(240, 244, 250, 255),
	TextSecondary:   widget.RGBA8(160, 170, 190, 255),
	TextMuted:       widget.RGBA8(105, 115, 135, 255),
	AccentPrimary:   widget.RGBA8(16, 185, 129, 255), // Emerald green
	AccentHover:     widget.RGBA8(5, 150, 105, 255),
	StopColor:       widget.RGBA8(239, 68, 68, 255),  // Coral red
	QuickShiftColor: widget.RGBA8(245, 158, 11, 255), // Amber
	QuickShiftBg:    widget.RGBA8(69, 39, 0, 180),
	InputBorder:     widget.RGBA8(60, 68, 86, 255),
	InputBg:         widget.RGBA8(22, 26, 35, 240),
	TabActive:       widget.RGBA8(45, 52, 68, 255),
	TabInactive:     widget.RGBA8(0, 0, 0, 0),
	LineSeparator:   widget.RGBA8(40, 46, 60, 255),
}

// ProjectColors provides nice palette choices for project badges
var ProjectColors = []string{
	"#3B82F6", // Blue
	"#10B981", // Emerald
	"#8B5CF6", // Purple
	"#F59E0B", // Amber
	"#EC4899", // Pink
	"#06B6D4", // Cyan
	"#F97316", // Orange
}

// ParseHexColor parses a hex color string like "#3B82F6" into a widget.Color.
func ParseHexColor(hex string) widget.Color {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return widget.RGBA8(99, 102, 241, 255)
	}

	var r, g, b uint8
	for i := 0; i < 6; i++ {
		c := hex[i]
		var val uint8
		if c >= '0' && c <= '9' {
			val = c - '0'
		} else if c >= 'a' && c <= 'f' {
			val = c - 'a' + 10
		} else if c >= 'A' && c <= 'F' {
			val = c - 'A' + 10
		}
		if i < 2 {
			r = (r << 4) | val
		} else if i < 4 {
			g = (g << 4) | val
		} else {
			b = (b << 4) | val
		}
	}
	return widget.RGBA8(r, g, b, 255)
}
