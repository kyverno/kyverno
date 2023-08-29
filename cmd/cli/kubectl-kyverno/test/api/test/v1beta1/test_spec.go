package v1beta1

type TestSpec struct {
	Policies  []string     `json:"policies"`
	Resources []string     `json:"resources"`
	Variables string       `json:"variables"`
	UserInfo  string       `json:"userinfo"`
	Results   []TestResult `json:"results"`
}
