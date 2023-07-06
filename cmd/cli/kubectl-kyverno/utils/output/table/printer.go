package table

import (
	"os"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/color"
	"github.com/lensesio/tableprinter"
)

func NewTablePrinter() *tableprinter.Printer {
	printer := tableprinter.New(os.Stdout)
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.HeaderBgColor = color.HeaderBgColor
	printer.HeaderFgColor = color.HeaderFgColor
	printer.RowLengthTitle = func(rowsLength int) bool {
		return rowsLength > 10
	}
	return printer
}
