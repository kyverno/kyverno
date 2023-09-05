package values

type Resource struct {
	Name   string                 `json:"name"`
	Values map[string]interface{} `json:"values"`
}
