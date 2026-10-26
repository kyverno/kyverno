package deprecations

import (
	"fmt"
	"io"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
)

func CheckUserInfo(out io.Writer, path string, resource *v1alpha1.UserInfo) bool {
	if resource != nil {
		if resource.APIVersion == "" || resource.Kind == "" {
			if out != nil {
				fmt.Fprintf(out, "\n%s user infos file (%s) uses a deprecated schema that will be removed in 1.15\n", color.BoldYellow("WARNING:"), path)
			}
			return true
		}
	}
	return false
}

func CheckValues(out io.Writer, path string, resource *v1alpha1.Values) bool {
	if resource != nil {
		if resource.APIVersion == "" || resource.Kind == "" {
			if out != nil {
				fmt.Fprintf(out, "\n%s values file (%s) uses a deprecated schema that will be removed in 1.15\n", color.BoldYellow("WARNING:"), path)
			}
			return true
		}
	}
	return false
}

func CheckTest(out io.Writer, path string, resource *v1alpha1.Test) bool {
	if resource != nil {
		if resource.APIVersion == "" || resource.Kind == "" || resource.Name != "" {
			if out != nil {
				fmt.Fprintf(out, "\n%s test file (%s) uses a deprecated schema that will be removed in 1.15\n", color.BoldYellow("WARNING:"), path)
			}
			return true
		}
	}
	return false
}
