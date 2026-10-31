package apply

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A namespaced policy only applies to resources in its own namespace, which is how the
// deleting controller and the image validating webhooks scope it in a cluster.
func Test_Apply_NamespacedPolicyScope(t *testing.T) {
	testcases := []struct {
		name            string
		config          ApplyCommandConfig
		policyNamespace string
	}{{
		name: "deleting policy",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{"../../../../../test/cli/test-deleting-policy/namespaced-scope/policy.yaml"},
			ResourcePaths: []string{"../../../../../test/cli/test-deleting-policy/namespaced-scope/resource.yaml"},
		},
		policyNamespace: "team-a",
	}, {
		name: "image validating policy",
		config: ApplyCommandConfig{
			PolicyPaths: []string{"../../../../../test/cli/test-image-validating-policy/namespaced-scope/policy.yaml"},
			ResourcePaths: []string{
				"../../../../../test/cli/test-image-validating-policy/namespaced-scope/good-pod.yaml",
				"../../../../../test/cli/test-image-validating-policy/namespaced-scope/bad-pod.yaml",
				"../../../../../test/cli/test-image-validating-policy/namespaced-scope/bad-pod-other-namespace.yaml",
			},
		},
		policyNamespace: "default",
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, responses, err := tc.config.applyCommandHelper(context.TODO(), io.Discard)
			assert.NoError(t, err)
			evaluated := map[string]int{}
			for _, response := range responses {
				if len(response.PolicyResponse.Rules) > 0 {
					evaluated[response.Resource.GetNamespace()]++
				}
			}
			assert.Greater(t, evaluated[tc.policyNamespace], 0, "the policy should be evaluated in its own namespace")
			for namespace := range evaluated {
				assert.Equal(t, tc.policyNamespace, namespace, "the policy should not be evaluated in another namespace")
			}
		})
	}
}
