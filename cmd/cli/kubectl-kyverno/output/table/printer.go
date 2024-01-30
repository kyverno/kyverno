package table

import (
	"io"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/lensesio/tableprinter"
)

func rowsLength(length int) bool {
	return length > 10
}

func NewTablePrinter(out io.Writer) *tableprinter.Printer {
	printer := tableprinter.New(out)
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.HeaderBgColor = color.HeaderBgColor
	printer.HeaderFgColor = color.HeaderFgColor
	printer.RowLengthTitle = rowsLength
	return printer
}
