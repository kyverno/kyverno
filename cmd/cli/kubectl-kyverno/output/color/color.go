package color

import (
	"strings"

	"github.com/fatih/color"
	"github.com/kataras/tablewriter"
)

var (
	BoldGreen     *color.Color
	BoldRed       *color.Color
	BoldYellow    *color.Color
	BoldFgCyan    *color.Color
	HeaderBgColor int
	HeaderFgColor int
)

func InitColors(noColor bool) {
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
	if !noColor {
		HeaderBgColor = tablewriter.BgBlackColor
		HeaderFgColor = tablewriter.FgGreenColor
	}
}

func Policy(namespace, name string) string {
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			namespace = parts[0]
			name = parts[1]
		}
	}
	if namespace == "" {
		return BoldFgCyan.Sprint(name)
	}
	return BoldFgCyan.Sprint(namespace) + "/" + BoldFgCyan.Sprint(name)
}

func Rule(name string) string {
	return BoldFgCyan.Sprint(name)
}

func Resource(kind, namespace, name string) string {
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			namespace = parts[0]
			name = parts[1]
		}
	}
	if namespace == "" {
		return BoldFgCyan.Sprint(kind) + "/" + BoldFgCyan.Sprint(name)
	}
	return BoldFgCyan.Sprint(namespace) + "/" + BoldFgCyan.Sprint(kind) + "/" + BoldFgCyan.Sprint(name)
}

func Excluded() string {
	return BoldYellow.Sprint("Excluded")
}

func NotFound() string {
	return BoldYellow.Sprint("Not found")
}

func ResultPass() string {
	return BoldGreen.Sprint("Pass")
}

func ResultFail() string {
	return BoldRed.Sprint("Fail")
}

func ResultWarn() string {
	return BoldYellow.Sprint("Warn")
}

func ResultError() string {
	return BoldRed.Sprint("Error")
}

func ResultSkip() string {
	return BoldFgCyan.Sprint("Skip")
}
