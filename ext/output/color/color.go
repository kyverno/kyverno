package color

import (
	"github.com/fatih/color"
)

// Color is an alias to color.Color
type Color = color.Color

var (
	BoldGreen  *Color
	BoldRed    *Color
	BoldYellow *Color
	BoldFgCyan *Color
)

func Init(noColor bool, force bool) {
	toggleColor := func(c *Color) *Color {
		if noColor {
			c.DisableColor()
		} else if force {
			c.EnableColor()
		}
		return c
	}
	BoldGreen = toggleColor(color.New(color.FgGreen).Add(color.Bold))
	BoldRed = toggleColor(color.New(color.FgRed).Add(color.Bold))
	BoldYellow = toggleColor(color.New(color.FgYellow).Add(color.Bold))
	BoldFgCyan = toggleColor(color.New(color.FgCyan).Add(color.Bold))
}
