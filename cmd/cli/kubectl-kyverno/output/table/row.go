package table

type Row struct {
	RowCompact `header:"inline"`
	Message    string `header:"message"`
}
