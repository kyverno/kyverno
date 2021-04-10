package store

var Mock bool
var ContextVar Context

func SetMock(mock bool) {
	Mock = mock
}

func GetMock() bool {
	return Mock
}

func SetContext(context Context) {
	ContextVar = context
}

func GetContext() Context {
	return ContextVar
}

func GetPolicyFromContext(policyName string) *Policy {
	for _, policy := range ContextVar.Policies {
		if policy.Name == policyName {
			return &policy
		}
	}
	return nil
}

func GetPolicyRuleFromContext(policyName string, ruleName string) *Rule {
	for _, policy := range ContextVar.Policies {
		if policy.Name == policyName {
			for _, rule := range policy.Rules {
				if rule.Name == ruleName {
					return &rule
				}
			}
		}
	}
	return nil
}

type Context struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Name  string `json:"name"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}
