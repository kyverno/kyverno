package values

type Rule struct {
	Name          string                   `json:"name"`
	Values        map[string]interface{}   `json:"values"`
	ForeachValues map[string][]interface{} `json:"foreachValues"`
}
