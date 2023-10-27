package color

import (
	"github.com/fatih/color"
)

var (
	BoldGreen  *color.Color
	BoldRed    *color.Color
	BoldYellow *color.Color
	BoldFgCyan *color.Color
)

func Init(noColor bool) {
	toggleColor := func(c *color.Color) *color.Color {
		if noColor {
			c.DisableColor()
		}
		return c
	}
	BoldGreen = toggleColor(color.New(color.FgGreen).Add(color.Bold))
	BoldRed = toggleColor(color.New(color.FgRed).Add(color.Bold))
	BoldYellow = toggleColor(color.New(color.FgYellow).Add(color.Bold))
	BoldFgCyan = toggleColor(color.New(color.FgCyan).Add(color.Bold))
}
