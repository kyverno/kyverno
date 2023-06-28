package test

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

type filter = func(api.TestResults) bool

func noFilter(api.TestResults) bool {
	return true
}

func parseFilter(in string) filter {
	var filters []filter
	if in != "" {
		for _, t := range strings.Split(in, ",") {
			parts := strings.Split(t, "=")
			if len(parts) != 2 {
				fmt.Printf("\n Invalid test-case-selector argument (%s). Selecting all test cases. \n", t)
				return noFilter
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "policy":
				filters = append(filters, func(r api.TestResults) bool {
					return r.Policy == "" || r.Policy == value
				})
			case "rule":
				filters = append(filters, func(r api.TestResults) bool {
					return r.Rule == "" || r.Rule == value
				})
			case "resource":
				filters = append(filters, func(r api.TestResults) bool {
					return r.Resource == "" || r.Resource == value
				})
			default:
				fmt.Printf("\n Invalid parameter. Parameter can only be policy, rule or resource. Selecting all test cases \n")
				return noFilter
			}
		}
	}
	return func(r api.TestResults) bool {
		for _, filter := range filters {
			if !filter(r) {
				return false
			}
		}
		return true
	}
}
