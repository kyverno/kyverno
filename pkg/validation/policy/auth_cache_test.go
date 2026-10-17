package policy

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestValidate_AuthorizationCache exercises cache creation and forwarding through every action factory.
func TestValidate_AuthorizationCache(t *testing.T) {
	const backgroundSA = "system:serviceaccount:kyverno:background-controller"
	const reportsSA = "system:serviceaccount:kyverno:reports-controller"
	type accessReview struct {
		user, verb, resource, namespace string
	}
	for _, tt := range []struct {
		name            string
		action          string
		evaluationError string
	}{
		{name: "validate", action: `"validate":{"pattern":{"metadata":{"labels":{"team":"*"}}}}`},
		{name: "mutate", action: `"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"team":"example"}}}}`},
		{name: "generate", action: `"generate":{"apiVersion":"v1","kind":"ConfigMap","name":"generated-config","namespace":"default","data":{"data":{"team":"example"}}}`},
		{
			name:            "evaluation errors remain permission warnings",
			action:          `"validate":{"pattern":{"metadata":{"labels":{"team":"*"}}}}`,
			evaluationError: "authorization webhook timeout",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{
				"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy",
				"metadata":{"name":"shared-authorization-cache","annotations":{"pod-policies.kyverno.io/autogen-controllers":"none"}},
				"spec":{"background":true,"rules":[
					{"name":"rule-1","match":{"any":[{"resources":{"kinds":["Pod"]}}]},%s},
					{"name":"rule-2","match":{"any":[{"resources":{"kinds":["Pod"]}}]},%s}
				]}
			}`, tt.action, tt.action)), &policy))

			kubeClient := kubefake.NewSimpleClientset()
			kubeClient.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "pods", Kind: "Pod", Namespaced: true},
					{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
				},
			}}
			calls := map[accessReview]int{}
			kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				attrs := review.Spec.ResourceAttributes
				require.NotNil(t, attrs)
				assert.Equal(t, "v1", attrs.Version)
				assert.Empty(t, attrs.Name)
				assert.Empty(t, attrs.Subresource)
				calls[accessReview{review.Spec.User, attrs.Verb, attrs.Resource, attrs.Namespace}]++
				return true, &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed:         tt.evaluationError == "",
					EvaluationError: tt.evaluationError,
				}}, nil
			})
			client := dclient.NewFakeClientWithDisco(
				dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), kubeClient,
				dclient.NewServerResourcesDiscovery(kubeClient.Discovery()),
			)
			expected := map[accessReview]int{
				{reportsSA, "get", "pods", ""}:   0,
				{reportsSA, "list", "pods", ""}:  0,
				{reportsSA, "watch", "pods", ""}: 0,
			}
			if tt.name == "generate" {
				expected[accessReview{backgroundSA, "get", "configmaps", "default"}] = 0
				expected[accessReview{backgroundSA, "create", "configmaps", "default"}] = 0
			}
			// Each validation must share decisions across its rules, then discard them.
			for pass := 1; pass <= 2; pass++ {
				warnings, err := Validate(&policy, nil, client, false, backgroundSA, reportsSA)
				require.NoError(t, err)
				if tt.evaluationError != "" {
					assert.Contains(t, strings.Join(warnings, "\n"), reportsSA+" requires permissions get,list,watch for resource Pod")
				}
				for key := range expected {
					expected[key] = pass
				}
				assert.Equal(t, expected, calls, "each authorization tuple should be reviewed once per validation")
			}
		})
	}
}
