package deprecations

import (
	"fmt"
	"io"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func CheckUserInfo(out io.Writer, path string, resource *v1alpha1.UserInfo) bool {
	if resource != nil {
		if resource.APIVersion == "" || resource.Kind == "" {
			if out != nil {
				fmt.Fprintf(out, "\nWARNING: user infos file (%s) uses a deprecated schema that will be removed in 1.15\n", path)
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
				fmt.Fprintf(out, "\nWARNING: values file (%s) uses a deprecated schema that will be removed in 1.15\n", path)
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
				fmt.Fprintf(out, "\nWARNING: test file (%s) uses a deprecated schema that will be removed in 1.15\n", path)
			}
			return true
		}
	}
	return false
}
