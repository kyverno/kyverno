package autogenv2

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Test_CanAutoGen(t *testing.T) {
	testCases := []struct {
		name                string
		policy              []byte
		expectedControllers sets.Set[string]
	}{
		{
			name:                "rule-with-match-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Namespace"],"name":"*"}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-match-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"],"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-exclude-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"test"}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-exclude-names",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names":["test"]}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-exclude-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-deny",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","deny":{"conditions":[{"key":"{{request.object.metadata.labels.foo}}","operator":"Equals","value":"bar"}]}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-match-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"preconditions":{"any":[{"key":"{{request.operation}}","operator":"Equals","value":"CREATE"}]},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-exclude-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-match-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-exclude-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod"],"namespaces":["test"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-mutate-patches",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchesJson6902":"-op:add\npath:/spec/containers/0/env/-1\nvalue:{\"name\":\"SERVICE\",\"value\":{{request.object.spec.template.metadata.labels.app}}}"}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-generate",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-networkpolicy"},"spec":{"rules":[{"name":"default-deny-ingress","match":{"resources":{"kinds":["Namespace"],"name":"*"}},"exclude":{"resources":{"namespaces":["kube-system","default","kube-public","kyverno"]}},"generate":{"kind":"NetworkPolicy","name":"default-deny-ingress","namespace":"{{request.object.metadata.name}}","synchronize":true,"data":{"spec":{"podSelector":{},"policyTypes":["Ingress"]}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-predefined-invalid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"DaemonSet,Deployment,StatefulSet","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-only-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Namespace"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New("none"),
		},
		{
			name:                "rule-with-match-kinds-pod-only-validate-exclude",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","podSecurity": {"level": "baseline","version":"v1.24","exclude":[{"controlName":"SELinux","restrictedField":"spec.containers[*].securityContext.seLinuxOptions.role","images":["nginx"],"values":["baz"]}, {"controlName":"SELinux","restrictedField":"spec.initContainers[*].securityContext.seLinuxOptions.role","images":["nodejs"],"values":["init-baz"]}]}}}]}}`),
			expectedControllers: PodControllers,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal(test.policy, &policy)
			assert.NilError(t, err)

			applyAutoGen, controllers := CanAutoGen(&policy.Spec)
			if !applyAutoGen {
				controllers = sets.New("none")
			}

			equalityTest := test.expectedControllers.Equal(controllers)
			assert.Assert(t, equalityTest, fmt.Sprintf("expected: %v, got: %v", test.expectedControllers, controllers))
		})
	}
}

func Test_GetSupportedControllers(t *testing.T) {
	testCases := []struct {
		name                string
		policy              []byte
		expectedControllers sets.Set[string]
	}{
		{
			name:                "rule-with-match-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Namespace"],"name":"*"}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-match-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"],"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-exclude-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"test"}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-exclude-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-deny",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","deny":{"conditions":[{"key":"{{request.object.metadata.labels.foo}}","operator":"Equals","value":"bar"}]}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-match-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"preconditions":{"any":[{"key":"{{request.operation}}","operator":"Equals","value":"CREATE"}]},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-exclude-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-match-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-exclude-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod"],"namespaces":["test"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-mutate-patches",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchesJson6902":"-op:add\npath:/spec/containers/0/env/-1\nvalue:{\"name\":\"SERVICE\",\"value\":{{request.object.spec.template.metadata.labels.app}}}"}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-generate",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-networkpolicy"},"spec":{"rules":[{"name":"default-deny-ingress","match":{"resources":{"kinds":["Namespace"],"name":"*"}},"exclude":{"resources":{"namespaces":["kube-system","default","kube-public","kyverno"]}},"generate":{"kind":"NetworkPolicy","name":"default-deny-ingress","namespace":"{{request.object.metadata.name}}","synchronize":true,"data":{"spec":{"podSelector":{},"policyTypes":["Ingress"]}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-predefined-invalid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"DaemonSet,Deployment,StatefulSet","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-only-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Namespace"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: sets.New[string](),
		},
		{
			name:                "rule-with-match-kinds-pod-only-validate-exclude",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","podSecurity": {"level": "baseline","version":"v1.24","exclude":[{"controlName":"SELinux","restrictedField":"spec.containers[*].securityContext.seLinuxOptions.role","images":["nginx"],"values":["baz"]}, {"controlName":"SELinux","restrictedField":"spec.initContainers[*].securityContext.seLinuxOptions.role","images":["nodejs"],"values":["init-baz"]}]}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-validate-podsecurity",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"pod-security"},"spec":{"rules":[{"name":"restricted","match":{"all":[{"resources":{"kinds":["Pod"]}}]},"validate":{"failureAction":"enforce","podSecurity":{"level":"restricted","version":"v1.24"}}}]}}`),
			expectedControllers: PodControllers,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal(test.policy, &policy)
			assert.NilError(t, err)

			controllers := GetSupportedControllers(&policy.Spec)

			equalityTest := test.expectedControllers.Equal(controllers)
			assert.Assert(t, equalityTest, fmt.Sprintf("expected: %v, got: %v", test.expectedControllers, controllers))
		})
	}
}

func Test_GetRequestedControllers(t *testing.T) {
	testCases := []struct {
		name                string
		meta                metav1.ObjectMeta
		expectedControllers sets.Set[string]
	}{
		{
			name:                "annotations-nil",
			meta:                metav1.ObjectMeta{},
			expectedControllers: nil,
		},
		{
			name:                "annotation-not-set",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{}},
			expectedControllers: nil,
		},
		{
			name:                "annotation-empty",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: ""}},
			expectedControllers: nil,
		},
		{
			name:                "annotation-none",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: "none"}},
			expectedControllers: sets.New[string](),
		},
		{
			name:                "annotation-job",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: "Job"}},
			expectedControllers: sets.New[string]("Job"),
		},
		{
			name:                "annotation-job-deployment",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: "Job,Deployment"}},
			expectedControllers: sets.New[string]("Job", "Deployment"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			controllers := GetRequestedControllers(&test.meta)

			equalityTest := test.expectedControllers.Equal(controllers)
			assert.Assert(t, equalityTest, fmt.Sprintf("expected: %v, got: %v", test.expectedControllers, controllers))
		})
	}
}

func TestExtractPodSpec(t *testing.T) {
	testCases := []struct {
		name        string
		resource    unstructured.Unstructured  // The input resource
		expectedPod *unstructured.Unstructured // Expected pod spec
		expectError bool                       // Whether an error is expected
	}{
		{
			name: "extract pod spec from deployment",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "nginx",
										"image": "nginx",
									},
								},
							},
						},
					},
				},
			},
			expectedPod: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "nginx",
							"image": "nginx",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "extract pod spec from cronjob",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "test-cronjob",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"schedule": "* * * * *",
						"jobTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "nginx",
												"image": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedPod: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "nginx",
							"image": "nginx",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "no pod spec in configmap",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-configmap",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"key": "value",
					},
				},
			},
			expectedPod: nil,
			expectError: false,
		},
		{
			name: "invalid resource structure",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   nil, // missing metadata
					"spec":       nil, // missing spec
				},
			},
			expectedPod: nil,
			expectError: true,
		},
	}

	autogen := NewAutogenV2()
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// Call the function under test
			podSpec, err := autogen.ExtractPodSpec(test.resource)

			// Check for errors
			if test.expectError {
				assert.ErrorContains(t, err, "error extracting pod spec")
			} else {
				assert.NilError(t, err)
			}

			// Check for pod spec correctness
			if test.expectedPod != nil {
				assert.Assert(t, podSpec != nil, "expected pod spec but got nil")
				assert.DeepEqual(t, test.expectedPod.Object, podSpec.Object)
			} else {
				assert.Assert(t, podSpec == nil, "expected nil pod spec but got a non-nil value")
			}
		})
	}
}
