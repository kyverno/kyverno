package filter

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

type Filter interface {
	Apply(v1alpha1.TestResult) bool
}

type policy struct {
	value string
}

func (f policy) Apply(result v1alpha1.TestResult) bool {
	if result.Policy == "" {
		return true
	}
	if wildcard.Match(f.value, result.Policy) {
		return true
	}
	return false
}

type rule struct {
	value string
}

func (f rule) Apply(result v1alpha1.TestResult) bool {
	if result.Rule == "" {
		return true
	}
	if wildcard.Match(f.value, result.Rule) {
		return true
	}
	return false
}

type resource struct {
	value string
}

func (f resource) Apply(result v1alpha1.TestResult) bool {
	if result.Resource == "" {
		return true
	}
	if wildcard.Match(f.value, result.Resource) {
		return true
	}
	return false
}

type composite struct {
	filters []Filter
}

func (f composite) Apply(result v1alpha1.TestResult) bool {
	for _, f := range f.filters {
		if !f.Apply(result) {
			return false
		}
	}
	return true
}

func ParseFilter(in string) (Filter, []error) {
	var filters []Filter
	var errors []error
	if in != "" {
		for _, t := range strings.Split(in, ",") {
			parts := strings.Split(t, "=")
			if len(parts) != 2 {
				errors = append(errors, fmt.Errorf("Invalid test-case-selector argument (%s). Parameter must be in the form `<key>=<value>`.", t))
			} else {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				switch key {
				case "policy":
					filters = append(filters, policy{value})
				case "rule":
					filters = append(filters, rule{value})
				case "resource":
					filters = append(filters, resource{value})
				default:
					errors = append(errors, fmt.Errorf("Invalid test-case-selector (%s). Parameter can only be policy, rule or resource.", t))
				}
			}
		}
	}
	return composite{filters}, errors
}
