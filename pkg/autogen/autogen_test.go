package autogen

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_getControllers(t *testing.T) {
	testCases := []struct {
		name                string
		policy              []byte
		expectedControllers string
	}{
		{
			name:                "rule-with-match-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Namespace"],"name":"*"}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-match-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"],"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"test"}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-deny",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","deny":{"conditions":[{"key":"{{request.object.metadata.labels.foo}}","operator":"Equals","value":"bar"}]}}}]}}`),
			expectedControllers: engine.PodControllers,
		},

		{
			name:                "rule-with-match-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"preconditions":{"any":[{"key":"{{request.operation}}","operator":"Equals","value":"CREATE"}]},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-match-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: engine.PodControllers,
		},
		{
			name:                "rule-with-exclude-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod"],"namespaces":["test"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: engine.PodControllers,
		},
		{
			name:                "rule-with-mutate-patches",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchesJson6902":"-op:add\npath:/spec/containers/0/env/-1\nvalue:{\"name\":\"SERVICE\",\"value\":{{request.object.spec.template.metadata.labels.app}}}"}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-generate",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-networkpolicy"},"spec":{"rules":[{"name":"default-deny-ingress","match":{"resources":{"kinds":["Namespace"],"name":"*"}},"exclude":{"resources":{"namespaces":["kube-system","default","kube-public","kyverno"]}},"generate":{"kind":"NetworkPolicy","name":"default-deny-ingress","namespace":"{{request.object.metadata.name}}","synchronize":true,"data":{"spec":{"podSelector":{},"policyTypes":["Ingress"]}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-predefined-invalid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"DaemonSet,Deployment,StatefulSet","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-only-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Namespace"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
	}

	for _, test := range testCases {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(test.policy, &policy)
		assert.NilError(t, err)

		applyAutoGen, controllers := CanAutoGen(&policy.Spec, log.Log)
		if !applyAutoGen {
			controllers = "none"
		}
		assert.Equal(t, test.expectedControllers, controllers, fmt.Sprintf("test %s failed", test.name))
	}
}
