package fix

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func FixValues(values v1alpha1.Values) (v1alpha1.Values, []string, error) {
	var messages []string
	if values.APIVersion == "" {
		messages = append(messages, "api version is not set, setting `cli.kyverno.io/v1alpha1`")
		values.APIVersion = "cli.kyverno.io/v1alpha1"
	}
	if values.Kind == "" {
		messages = append(messages, "kind is not set, setting `Values`")
		values.Kind = "Values"
	}
	return values, messages, nil
}
