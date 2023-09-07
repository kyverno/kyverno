package table

type RowCompact struct {
	IsFailure bool
	ID        int    `header:"id"`
	Policy    string `header:"policy"`
	Rule      string `header:"rule"`
	Resource  string `header:"resource"`
	Result    string `header:"result"`
	Reason    string `header:"reason"`
}
