package table

import (
	pt "github.com/jedib0t/go-pretty/v6/table"
)

type Row struct {
	IsFailure bool
	ID        int
	Policy    string
	Rule      string
	Resource  string
	Result    string
	Reason    string
	Message   string
}

func (r Row) forTable(detail bool) pt.Row {
	tr := pt.Row{r.ID, r.Policy, r.Rule, r.Resource, r.Result, r.Reason}
	if detail {
		tr = append(tr, r.Message)
	}
	return tr
}
