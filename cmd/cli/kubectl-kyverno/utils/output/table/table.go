package table

type Table struct {
	RawRows []Row
}

func (t *Table) Rows(detailed bool) interface{} {
	if detailed {
		return t.RawRows
	}
	var rows []CompactRow
	for _, row := range t.RawRows {
		rows = append(rows, row.CompactRow)
	}
	return rows
}

func (t *Table) AddFailed(rows ...Row) {
	for _, row := range rows {
		if row.IsFailure {
			t.RawRows = append(t.RawRows, row)
		}
	}
}

func (t *Table) Add(rows ...Row) {
	t.RawRows = append(t.RawRows, rows...)
}

type CompactRow struct {
	IsFailure bool
	ID        int    `header:"id"`
	Policy    string `header:"policy"`
	Rule      string `header:"rule"`
	Resource  string `header:"resource"`
	Result    string `header:"result"`
}

type Row struct {
	CompactRow `header:"inline"`
	Message    string `header:"message"`
}
