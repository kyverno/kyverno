package v2

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
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

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// Call the function under test
			podSpec, err := extractPodSpec(&test.resource)

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

func Test_GetAutogenRuleNames(t *testing.T) {
	testCases := []struct {
		name          string
		policy        string
		expectedRules []string
	}{
		{
			name: "rule-with-match-name",
			policy: `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image
spec:
  background: false
  webhookTimeoutSeconds: 30
  failurePolicy: Fail
  rules:
    - name: check-image
      match:
        resources:
          kinds:
            - Pod
      verifyImages:
      - imageReferences: 
        - "*"
        attestors:
        - count: 1
          entries:
          - keyless:
              roots: |-
                -----BEGIN CERTIFICATE-----
                MIIDjTCCAnWgAwIBAgIQb8yUrbw3aYZAubIjOJkFBjANBgkqhkiG9w0BAQsFADBZ
                MRMwEQYKCZImiZPyLGQBGRYDY29tMRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVt
                bzEmMCQGA1UEAxMddmVuYWZpZGVtby1FQzJBTUFaLVFOSVI4OUktQ0EwHhcNMjAx
                MjE0MjEzNzAzWhcNMjUxMjE0MjE0NzAzWjBZMRMwEQYKCZImiZPyLGQBGRYDY29t
                MRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVtbzEmMCQGA1UEAxMddmVuYWZpZGVt
                by1FQzJBTUFaLVFOSVI4OUktQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
                AoIBAQC5CTVQczGnh77yNxq+BGh5ff0qNcRTkFll+y8lJbMPHevebF7JLWBQTGS7
                9aHIqUQLjy9sPOkdMrDh/vOZNVhVrHon9uwepF81dUMJ9lMbfQSI/tytp78f0z6b
                DVRHYZr/taYSkqNPT2FuHOijc7Y+oB3Q1DzPSoBc3a6I5DM6ET6O2GZWo3mqpImG
                J8+dNllYgjVKEuxuPqQjT7VD4fB2GqJbwwL0E8bSyfsgMV9Y+qHdznkm8v+TbYoc
                9uS83f1fjjp98D7VtWpSC4O/27JWgEED/BB58sOipUQHiECr6dD5VWGJ9fnVOV2i
                vHqj9cKS6BGMkAh99ss0Bu/3DEBxAgMBAAGjUTBPMAsGA1UdDwQEAwIBhjAPBgNV
                HRMBAf8EBTADAQH/MB0GA1UdDgQWBBTuZecNgrj3Gdv9XpekFZuIkYtu9jAQBgkr
                BgEEAYI3FQEEAwIBADANBgkqhkiG9w0BAQsFAAOCAQEADPNrGypaKliXJ+H7gt6b
                NJSBdWB9EV63CdvxjLOuqvp3IUu8KIV2mMsulEjxjAb5kya0SURJVFvr9rrLVxvR
                e6B2SJUGUKJkX1Cq4nIthwGfJTEnypYhqMKkfUYjqfszU+1CerRD2ZTJHeKZsc7M
                GdxLXeocztZ220idf6uDYeNLnGLBfkodEgFV0RmrlnHQYQdRqj3hjClLAkNqKVrz
                rxNyyQvgaswK+4kHAPQhv+ipx4Q0eeROpp3prJ+dD0hhk8niQSKWQWZHyElhzIKv
                FlDw3fzPhtberBblY4Y9u525ev999SogMBTXoSkfajRR2ol10xUxY60kVbqoEUln
                kA==
                -----END CERTIFICATE-----`,
			expectedRules: []string{"check-image", "autogen-check-image", "autogen-cronjob-check-image"},
		},
		{
			name: "rule-with-match-name",
			policy: `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image
  annotations:
    pod-policies.kyverno.io/autogen-controllers: Deployment,Job,StatefulSet
spec:
  background: false
  webhookTimeoutSeconds: 30
  failurePolicy: Fail
  rules:
    - name: check-image
      match:
        resources:
          kinds:
            - Pod
      verifyImages:
      - imageReferences: 
        - "*"
        attestors:
        - count: 1
          entries:
          - keyless:
              roots: |-
                -----BEGIN CERTIFICATE-----
                MIIDjTCCAnWgAwIBAgIQb8yUrbw3aYZAubIjOJkFBjANBgkqhkiG9w0BAQsFADBZ
                MRMwEQYKCZImiZPyLGQBGRYDY29tMRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVt
                bzEmMCQGA1UEAxMddmVuYWZpZGVtby1FQzJBTUFaLVFOSVI4OUktQ0EwHhcNMjAx
                MjE0MjEzNzAzWhcNMjUxMjE0MjE0NzAzWjBZMRMwEQYKCZImiZPyLGQBGRYDY29t
                MRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVtbzEmMCQGA1UEAxMddmVuYWZpZGVt
                by1FQzJBTUFaLVFOSVI4OUktQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
                AoIBAQC5CTVQczGnh77yNxq+BGh5ff0qNcRTkFll+y8lJbMPHevebF7JLWBQTGS7
                9aHIqUQLjy9sPOkdMrDh/vOZNVhVrHon9uwepF81dUMJ9lMbfQSI/tytp78f0z6b
                DVRHYZr/taYSkqNPT2FuHOijc7Y+oB3Q1DzPSoBc3a6I5DM6ET6O2GZWo3mqpImG
                J8+dNllYgjVKEuxuPqQjT7VD4fB2GqJbwwL0E8bSyfsgMV9Y+qHdznkm8v+TbYoc
                9uS83f1fjjp98D7VtWpSC4O/27JWgEED/BB58sOipUQHiECr6dD5VWGJ9fnVOV2i
                vHqj9cKS6BGMkAh99ss0Bu/3DEBxAgMBAAGjUTBPMAsGA1UdDwQEAwIBhjAPBgNV
                HRMBAf8EBTADAQH/MB0GA1UdDgQWBBTuZecNgrj3Gdv9XpekFZuIkYtu9jAQBgkr
                BgEEAYI3FQEEAwIBADANBgkqhkiG9w0BAQsFAAOCAQEADPNrGypaKliXJ+H7gt6b
                NJSBdWB9EV63CdvxjLOuqvp3IUu8KIV2mMsulEjxjAb5kya0SURJVFvr9rrLVxvR
                e6B2SJUGUKJkX1Cq4nIthwGfJTEnypYhqMKkfUYjqfszU+1CerRD2ZTJHeKZsc7M
                GdxLXeocztZ220idf6uDYeNLnGLBfkodEgFV0RmrlnHQYQdRqj3hjClLAkNqKVrz
                rxNyyQvgaswK+4kHAPQhv+ipx4Q0eeROpp3prJ+dD0hhk8niQSKWQWZHyElhzIKv
                FlDw3fzPhtberBblY4Y9u525ev999SogMBTXoSkfajRR2ol10xUxY60kVbqoEUln
                kA==
                -----END CERTIFICATE-----`,
			expectedRules: []string{"check-image", "autogen-check-image"},
		},
		{
			name: "rule-with-match-name",
			policy: `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image
  annotations:
    pod-policies.kyverno.io/autogen-controllers: Deployment,CronJob,Job
spec:
  background: false
  webhookTimeoutSeconds: 30
  failurePolicy: Fail
  rules:
    - name: check-image
      match:
        resources:
          kinds:
            - Pod
      verifyImages:
      - imageReferences: 
        - "*"
        attestors:
        - count: 1
          entries:
          - keyless:
              roots: |-
                -----BEGIN CERTIFICATE-----
                MIIDjTCCAnWgAwIBAgIQb8yUrbw3aYZAubIjOJkFBjANBgkqhkiG9w0BAQsFADBZ
                MRMwEQYKCZImiZPyLGQBGRYDY29tMRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVt
                bzEmMCQGA1UEAxMddmVuYWZpZGVtby1FQzJBTUFaLVFOSVI4OUktQ0EwHhcNMjAx
                MjE0MjEzNzAzWhcNMjUxMjE0MjE0NzAzWjBZMRMwEQYKCZImiZPyLGQBGRYDY29t
                MRowGAYKCZImiZPyLGQBGRYKdmVuYWZpZGVtbzEmMCQGA1UEAxMddmVuYWZpZGVt
                by1FQzJBTUFaLVFOSVI4OUktQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
                AoIBAQC5CTVQczGnh77yNxq+BGh5ff0qNcRTkFll+y8lJbMPHevebF7JLWBQTGS7
                9aHIqUQLjy9sPOkdMrDh/vOZNVhVrHon9uwepF81dUMJ9lMbfQSI/tytp78f0z6b
                DVRHYZr/taYSkqNPT2FuHOijc7Y+oB3Q1DzPSoBc3a6I5DM6ET6O2GZWo3mqpImG
                J8+dNllYgjVKEuxuPqQjT7VD4fB2GqJbwwL0E8bSyfsgMV9Y+qHdznkm8v+TbYoc
                9uS83f1fjjp98D7VtWpSC4O/27JWgEED/BB58sOipUQHiECr6dD5VWGJ9fnVOV2i
                vHqj9cKS6BGMkAh99ss0Bu/3DEBxAgMBAAGjUTBPMAsGA1UdDwQEAwIBhjAPBgNV
                HRMBAf8EBTADAQH/MB0GA1UdDgQWBBTuZecNgrj3Gdv9XpekFZuIkYtu9jAQBgkr
                BgEEAYI3FQEEAwIBADANBgkqhkiG9w0BAQsFAAOCAQEADPNrGypaKliXJ+H7gt6b
                NJSBdWB9EV63CdvxjLOuqvp3IUu8KIV2mMsulEjxjAb5kya0SURJVFvr9rrLVxvR
                e6B2SJUGUKJkX1Cq4nIthwGfJTEnypYhqMKkfUYjqfszU+1CerRD2ZTJHeKZsc7M
                GdxLXeocztZ220idf6uDYeNLnGLBfkodEgFV0RmrlnHQYQdRqj3hjClLAkNqKVrz
                rxNyyQvgaswK+4kHAPQhv+ipx4Q0eeROpp3prJ+dD0hhk8niQSKWQWZHyElhzIKv
                FlDw3fzPhtberBblY4Y9u525ev999SogMBTXoSkfajRR2ol10xUxY60kVbqoEUln
                kA==
                -----END CERTIFICATE-----`,
			expectedRules: []string{"check-image", "autogen-check-image", "autogen-cronjob-check-image"},
		},
		{
			name: "rule-with-match-name",
			policy: `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-latest-tag
  annotations:
    pod-policies.kyverno.io/autogen-controllers: Deployment,CronJob
spec:
  rules:
  - match:
      any:
        - resources:
            kinds:
            - Pod
    name: require-image-tag
    validate:
      failureAction: Audit
      message: An image tag is required.
      pattern:
        spec:
          containers:
          - image: '*:*'
  - match:
      any:
        - resources:
            kinds:
            - Pod
    name: validate-image-tag
    validate:
      failureAction: Audit
      message: Using a mutable image tag e.g. 'latest' is not allowed.
      pattern:
        spec:
          containers:
          - image: '!*:latest' `,
			expectedRules: []string{"require-image-tag", "autogen-require-image-tag", "autogen-cronjob-require-image-tag", "validate-image-tag", "autogen-validate-image-tag", "autogen-cronjob-validate-image-tag"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			policies, _, _, _, _, _, _, err := yamlutils.GetPolicy([]byte(test.policy))
			assert.NilError(t, err)
			assert.Equal(t, 1, len(policies))
			rules := GetAutogenRuleNames(policies[0])
			assert.DeepEqual(t, test.expectedRules, rules)
		})
	}
}
