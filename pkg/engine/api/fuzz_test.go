package api

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

func FuzzEngineResponse(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)

		resource, err := ff.GetBytes()
		if err != nil {
			return
		}

		resourceUnstructured, err := kubeutils.BytesToUnstructured(resource)
		if err != nil {
			return
		}
		namespaceLabels := make(map[string]string)
		ff.FuzzMap(&namespaceLabels)
		resp := NewEngineResponse(*resourceUnstructured, nil, namespaceLabels)
		_ = resp.GetPatches()
		_ = resp.GetFailedRules()
		_ = resp.GetFailedRulesWithErrors()
		_ = resp.GetValidationFailureAction()
		_ = resp.GetSuccessRules()
	})
}
