package test

import (
	"os"

	"github.com/fatih/color"
	"github.com/kataras/tablewriter"
	"github.com/lensesio/tableprinter"
)

var (
	BoldGreen     *color.Color
	BoldRed       *color.Color
	BoldYellow    *color.Color
	BoldFgCyan    *color.Color
	headerBgColor int
	headerFgColor int
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
		headerBgColor = tablewriter.BgBlackColor
		headerFgColor = tablewriter.FgGreenColor
	}
}

func NewTablePrinter() *tableprinter.Printer {
	printer := tableprinter.New(os.Stdout)
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.HeaderBgColor = headerBgColor
	printer.HeaderFgColor = headerFgColor
	printer.RowLengthTitle = func(rowsLength int) bool {
		return rowsLength > 10
	}
	return printer
}
