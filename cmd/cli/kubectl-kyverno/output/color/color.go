package color

import (
	"strings"

	"github.com/kataras/tablewriter"
	"github.com/kyverno/kyverno/ext/output/color"
)

var (
	HeaderBgColor int
	HeaderFgColor int
)

func Init(noColor bool) {
	color.Init(noColor, false)
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
		return color.BoldFgCyan.Sprint(name)
	}
	return color.BoldFgCyan.Sprint(namespace) + "/" + color.BoldFgCyan.Sprint(name)
}

func Rule(name string) string {
	return color.BoldFgCyan.Sprint(name)
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
		return color.BoldFgCyan.Sprint(kind) + "/" + color.BoldFgCyan.Sprint(name)
	}
	return color.BoldFgCyan.Sprint(namespace) + "/" + color.BoldFgCyan.Sprint(kind) + "/" + color.BoldFgCyan.Sprint(name)
}

func Excluded() string {
	return color.BoldYellow.Sprint("Excluded")
}

func NotFound() string {
	return color.BoldYellow.Sprint("Not found")
}

func ResultPass() string {
	return color.BoldGreen.Sprint("Pass")
}

func ResultFail() string {
	return color.BoldRed.Sprint("Fail")
}

func ResultWarn() string {
	return color.BoldYellow.Sprint("Warn")
}

func ResultError() string {
	return color.BoldRed.Sprint("Error")
}

func ResultSkip() string {
	return color.BoldFgCyan.Sprint("Skip")
}
