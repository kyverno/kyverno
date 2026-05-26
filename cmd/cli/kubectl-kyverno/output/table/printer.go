package table

import (
	"io"

	pt "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/kyverno/kyverno/ext/output/color"
)

var (
	headerCompact  = pt.Row{"ID", "POLICY", "RULE", "RESOURCE", "RESULT", "REASON"}
	headerDetailed = append(append(pt.Row{}, headerCompact...), "MESSAGE")
)

func newTableWriter() pt.Writer {
	t := pt.NewWriter()
	t.SetStyle(pt.StyleLight)
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateRows = false
	t.Style().Options.SeparateHeader = true
	t.Style().Size.WidthMax = 300
	return t
}

func Print(out io.Writer, t Table, detailed bool) error {
	tw := newTableWriter()

	header := getHeader(detailed)
	tw.AppendHeader(header)
	configs := make([]pt.ColumnConfig, 0, len(header))
	for _, col := range header {
		if colStr, ok := col.(string); ok {
			configs = append(configs, pt.ColumnConfig{
				Name:             colStr,
				ColorsHeader:     color.BoldGreen,
				WidthMax:         100,
				WidthMaxEnforcer: text.WrapSoft,
			})
		}
	}
	tw.SetColumnConfigs(configs)

	for _, row := range t.RawRows {
		tw.AppendRow(row.forTable(detailed))
	}
	_, err := io.WriteString(out, tw.Render())
	return err
}

func getHeader(detailed bool) pt.Row {
	if detailed {
		result := make(pt.Row, len(headerDetailed))
		copy(result, headerDetailed)
		return result
	}
	result := make(pt.Row, len(headerCompact))
	copy(result, headerCompact)
	return result
}
