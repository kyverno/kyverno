package table

type Table struct {
	RawRows []Row
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

func (t *Table) Rows(detailed bool) interface{} {
	if detailed {
		return t.RawRows
	}
	var rows []RowCompact
	for _, row := range t.RawRows {
		rows = append(rows, row.RowCompact)
	}
	return rows
}
