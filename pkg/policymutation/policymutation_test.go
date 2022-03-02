package policymutation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func currentDir() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}

	return filepath.Join(homedir, "github.com/kyverno/kyverno"), nil
}

func Test_checkForGVKFormatPatch(t *testing.T) {
	testCases := []struct {
		name            string
		policy          []byte
		expectedPatches []byte
	}{
		{
			name:            "match-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-kinds-empty"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["ConfigMap","batch.volcano.sh/v1alpha1/Job"]}},"validate":{"message":"Metadatalabel'name'isrequired.","pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "match-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-kinds"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["configmap","batch.volcano.sh/v1alpha1/job"]}},"validate":{"message":"Metadatalabel'name'isrequired.","pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/match/resources/kinds","op":"replace","value":["Configmap","batch.volcano.sh/v1alpha1/Job"]}`),
		},
		{
			name:            "exclude-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-kinds-empty"},"spec":{"rules":[{"name":"test","exclude":{"resources":{"kinds":["ConfigMap","batch.volcano.sh/v1alpha1/Job"]}},"validate":{"message":"Metadatalabel'name'isrequired.","pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "exclude-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-kinds"},"spec":{"rules":[{"name":"test","exclude":{"resources":{"kinds":["configmap","batch.volcano.sh/v1alpha1/job"]}},"validate":{"message":"Metadatalabel'name'isrequired.","pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/exclude/resources/kinds","op":"replace","value":["Configmap","batch.volcano.sh/v1alpha1/Job"]}`),
		},
		{
			name:            "match-any-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-any-kinds-empty"},"spec":{"rules":[{"name":"test","match":{"any":[{"resources":{"kinds":["Deployment","Pod","DaemonSet"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "match-any-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-any-kinds"},"spec":{"rules":[{"name":"test","match":{"any":[{"resources":{"kinds":["deployment","pod","DaemonSet"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/match/any/0/resources/kinds","op":"replace","value":["Deployment","Pod"]}`),
		},
		{
			name:            "match-all-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-all-kinds-empty"},"spec":{"rules":[{"name":"test","match":{"all":[{"resources":{"kinds":["Deployment","Pod","DaemonSet"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "match-all-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"match-all-kinds"},"spec":{"rules":[{"name":"test","match":{"all":[{"resources":{"kinds":["deployment","pod","DaemonSet"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/match/all/0/resources/kinds","op":"replace","value":["Deployment","Pod"]}`),
		},
		{
			name:            "exclude-any-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-any-kinds-empty"},"spec":{"rules":[{"name":"test","exclude":{"any":[{"resources":{"kinds":["Deployment","Pod","DaemonSet"]}},{"resources":{"kinds":["ConfigMap"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "exclude-any-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-any-kinds-empty"},"spec":{"rules":[{"name":"test","exclude":{"any":[{"resources":{"kinds":["Deployment","Pod","DaemonSet"]}},{"resources":{"kinds":["configMap"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/exclude/any/1/resources/kinds","op":"replace","value":["ConfigMap"]}`),
		},
		{
			name:            "exclude-all-kinds-empty",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-all-kinds-empty"},"spec":{"rules":[{"name":"test","exclude":{"all":[{"resources":{"kinds":["Deployment","Pod","DaemonSet"]}},{"resources":{"kinds":["ConfigMap"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: nil,
		},
		{
			name:            "exclude-all-kinds",
			policy:          []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"exclude-all-kinds"},"spec":{"rules":[{"name":"test","exclude":{"all":[{"resources":{"kinds":["Deployment","pod","DaemonSet"]}},{"resources":{"kinds":["ConfigMap"]}}]},"validate":{"pattern":{"metadata":{"labels":{"name":"?*"}}}}}]}}`),
			expectedPatches: []byte(`{"path":"/spec/rules/0/exclude/all/0/resources/kinds","op":"replace","value":["Pod"]}`),
		},
	}

	for _, test := range testCases {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(test.policy, &policy)
		assert.NilError(t, err, fmt.Sprintf("failed to convert policy test %s: %v", test.name, err))

		patches, errs := checkForGVKFormatPatch(&policy, log.Log)
		assert.Assert(t, len(errs) == 0)
		for _, p := range patches {
			assert.Equal(t, string(p), string(test.expectedPatches), fmt.Sprintf("test %s failed", test.name))
		}
	}
}
