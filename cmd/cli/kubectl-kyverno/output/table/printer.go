package table

import (
	"io"

	pt "github.com/jedib0t/go-pretty/v6/table"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
)

var (
	headerCompact  = pt.Row{"ID", "POLICY", "RULE", "RESOURCE", "RESULT", "REASON"}
	headerDetailed = append(append(pt.Row{}, headerCompact...), "MESSAGE")
)

func newTableWriter() pt.Writer {
	t := pt.NewWriter()
	t.SetStyle(pt.StyleRounded)
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateHeader = true
	t.Style().Size.WidthMax = 300
	return t
}

func Print(out io.Writer, t Table, detail bool) error {
	tw := newTableWriter()

	tw.AppendHeader(getHeader(detail))
	for _, row := range t.RawRows {
		tw.AppendRow(row.forTable(detail))
	}
	_, err := io.WriteString(out, tw.Render())
	return err
}

func getHeader(detail bool) pt.Row {
	header := headerCompact
	if detail {
		header = headerDetailed
	}
	result := make(pt.Row, len(header))
	for i, val := range header {
		result[i] = color.Header(val)
	}
	return result
}
