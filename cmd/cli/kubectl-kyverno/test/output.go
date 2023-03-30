package test

import (
	"os"

	"github.com/fatih/color"
	"github.com/kataras/tablewriter"
	"github.com/lensesio/tableprinter"
)

var (
	boldGreen  = color.New(color.FgGreen).Add(color.Bold)
	boldRed    = color.New(color.FgRed).Add(color.Bold)
	boldYellow = color.New(color.FgYellow).Add(color.Bold)
	boldFgCyan = color.New(color.FgCyan).Add(color.Bold)
)

func colorize(noColor bool, color *color.Color, format string, a ...interface{}) string {
	if noColor {
		return format
	}
	return color.Sprintf(format, a...)
}

func newTablePrinter(noColor bool) *tableprinter.Printer {
	printer := tableprinter.New(os.Stdout)
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.RowLengthTitle = func(rowsLength int) bool {
		return rowsLength > 10
	}
	if !noColor {
		printer.HeaderBgColor = tablewriter.BgBlackColor
		printer.HeaderFgColor = tablewriter.FgGreenColor
	}
	return printer
}
