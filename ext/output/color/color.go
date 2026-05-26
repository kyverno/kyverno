package color

import (
	"strings"

	"github.com/jedib0t/go-pretty/v6/text"
)

var (
	BoldGreen  = text.Colors{text.FgGreen, text.Bold}
	BoldRed    = text.Colors{text.FgRed, text.Bold}
	BoldYellow = text.Colors{text.FgYellow, text.Bold}
	BoldFgCyan = text.Colors{text.FgCyan, text.Bold}
)

func Init(noColor bool) {
	if noColor {
		text.DisableColors()
	}
}

func Force(enable bool) {
	if enable {
		text.EnableColors()
	} else {
		text.DisableColors()
	}
}

func Enabled() bool {
	// return true if color is enabled - work around since go-pretty doesn't allow to query
	return strings.Contains(text.FgGreen.Sprint("x"), text.EscapeStart)
}
