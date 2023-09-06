package values

type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}
