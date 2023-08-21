package autogen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getAutogenRuleName(t *testing.T) {
	testCases := []struct {
		name     string
		ruleName string
		prefix   string
		expected string
	}{
		{"valid", "valid-rule-name", "autogen", "autogen-valid-rule-name"},
		{"truncated", "too-long-this-rule-name-will-be-truncated-to-63-characters", "autogen", "autogen-too-long-this-rule-name-will-be-truncated-to-63-charact"},
		{"valid-cronjob", "valid-rule-name", "autogen-cronjob", "autogen-cronjob-valid-rule-name"},
		{"truncated-cronjob", "too-long-this-rule-name-will-be-truncated-to-63-characters", "autogen-cronjob", "autogen-cronjob-too-long-this-rule-name-will-be-truncated-to-63"},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			res := getAutogenRuleName(test.prefix, test.ruleName)
			assert.Equal(t, test.expected, res)
		})
	}
}

func Test_isAutogenRule(t *testing.T) {
	testCases := []struct {
		name     string
		ruleName string
		expected bool
	}{
		{"normal", "valid-rule-name", false},
		{"simple", "autogen-simple", true},
		{"simple-cronjob", "autogen-cronjob-simple", true},
		{"truncated", "autogen-too-long-this-rule-name-will-be-truncated-to-63-charact", true},
		{"truncated-cronjob", "autogen-cronjob-too-long-this-rule-name-will-be-truncated-to-63", true},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			res := isAutogenRuleName(test.ruleName)
			assert.Equal(t, test.expected, res)
		})
	}
}

func Test_CanAutoGen(t *testing.T) {
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
			name:                "rule-with-exclude-names",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names":["test"]}}}]}}`),
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
			expectedControllers: PodControllers,
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
				controllers = "none"
			}
			assert.Equal(t, test.expectedControllers, controllers, fmt.Sprintf("test %s failed", test.name))
		})
	}
}

func Test_GetSupportedControllers(t *testing.T) {
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
			expectedControllers: PodControllers,
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
		{
			name:                "rule-with-match-kinds-pod-only-validate-exclude",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","podSecurity": {"level": "baseline","version":"v1.24","exclude":[{"controlName":"SELinux","restrictedField":"spec.containers[*].securityContext.seLinuxOptions.role","images":["nginx"],"values":["baz"]}, {"controlName":"SELinux","restrictedField":"spec.initContainers[*].securityContext.seLinuxOptions.role","images":["nodejs"],"values":["init-baz"]}]}}}]}}`),
			expectedControllers: PodControllers,
		},
		{
			name:                "rule-with-validate-podsecurity",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"pod-security"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"restricted","match":{"all":[{"resources":{"kinds":["Pod"]}}]},"validate":{"podSecurity":{"level":"restricted","version":"v1.24"}}}]}}`),
			expectedControllers: PodControllers,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal(test.policy, &policy)
			assert.NilError(t, err)

			controllers := GetSupportedControllers(&policy.Spec)

			var expectedControllers []string
			if test.expectedControllers != "none" {
				expectedControllers = strings.Split(test.expectedControllers, ",")
			}

			assert.DeepEqual(t, expectedControllers, controllers)
		})
	}
}

func Test_GetRequestedControllers(t *testing.T) {
	testCases := []struct {
		name                string
		meta                metav1.ObjectMeta
		expectedControllers []string
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
			expectedControllers: []string{},
		},
		{
			name:                "annotation-job",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: "Job"}},
			expectedControllers: []string{"Job"},
		},
		{
			name:                "annotation-job-deployment",
			meta:                metav1.ObjectMeta{Annotations: map[string]string{kyverno.AnnotationAutogenControllers: "Job,Deployment"}},
			expectedControllers: []string{"Job", "Deployment"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			controllers := GetRequestedControllers(&test.meta)
			assert.DeepEqual(t, test.expectedControllers, controllers)
		})
	}
}

func TestUpdateGenRuleByte(t *testing.T) {
	tests := []struct {
		pbyte   []byte
		kind    string
		want    []byte
		wantErr bool
	}{
		{
			pbyte: []byte("request.object.spec"),
			kind:  "Pod",
			want:  []byte("request.object.spec.template.spec"),
		},
		{
			pbyte: []byte("request.oldObject.spec"),
			kind:  "Pod",
			want:  []byte("request.oldObject.spec.template.spec"),
		},
		{
			pbyte: []byte("request.object.spec"),
			kind:  "Cronjob",
			want:  []byte("request.object.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte: []byte("request.oldObject.spec"),
			kind:  "Cronjob",
			want:  []byte("request.oldObject.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte: []byte("request.object.metadata"),
			kind:  "Pod",
			want:  []byte("request.object.spec.template.metadata"),
		},
	}
	for _, tt := range tests {
		got := updateGenRuleByte(tt.pbyte, tt.kind)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("updateGenRuleByte() = %v, want %v", string(got), string(tt.want))
		}
	}
}

func TestUpdateCELFields(t *testing.T) {
	tests := []struct {
		pbyte   []byte
		kind    string
		want    []byte
		wantErr bool
	}{
		{
			pbyte: []byte("object.spec"),
			kind:  "Pod",
			want:  []byte("object.spec.template.spec"),
		},
		{
			pbyte: []byte("oldObject.spec"),
			kind:  "Pod",
			want:  []byte("oldObject.spec.template.spec"),
		},
		{
			pbyte: []byte("object.spec"),
			kind:  "Cronjob",
			want:  []byte("object.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte: []byte("oldObject.spec"),
			kind:  "Cronjob",
			want:  []byte("oldObject.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte: []byte("object.metadata"),
			kind:  "Pod",
			want:  []byte("object.spec.template.metadata"),
		},
	}
	for _, tt := range tests {
		got := updateCELFields(tt.pbyte, tt.kind)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("updateCELFields() = %v, want %v", string(got), string(tt.want))
		}
	}
}

func Test_ComputeRules(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	testCases := []struct {
		name          string
		policy        string
		expectedRules []kyvernov1.Rule
	}{
		{
			name: "rule-with-match-name",
			policy: `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image
spec:
  validationFailureAction: enforce
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
			expectedRules: []kyvernov1.Rule{{
				Name: "check-image",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"Pod"},
					},
				},
				VerifyImages: []kyvernov1.ImageVerification{{
					ImageReferences: []string{"*"},
					Attestors: []kyvernov1.AttestorSet{{
						Count: intPtr(1),
						Entries: []kyvernov1.Attestor{{
							Keyless: &kyvernov1.KeylessAttestor{
								Roots: `-----BEGIN CERTIFICATE-----
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
							},
						}},
					}},
				}},
			}, {
				Name: "autogen-check-image",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"DaemonSet", "Deployment", "Job", "StatefulSet", "ReplicaSet", "ReplicationController"},
					},
				},
				VerifyImages: []kyvernov1.ImageVerification{{
					ImageReferences: []string{"*"},
					Attestors: []kyvernov1.AttestorSet{{
						Count: intPtr(1),
						Entries: []kyvernov1.Attestor{{
							Keyless: &kyvernov1.KeylessAttestor{
								Roots: `-----BEGIN CERTIFICATE-----
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
							},
						}},
					}},
				}},
			}, {
				Name: "autogen-cronjob-check-image",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"CronJob"},
					},
				},
				VerifyImages: []kyvernov1.ImageVerification{{
					ImageReferences: []string{"*"},
					Attestors: []kyvernov1.AttestorSet{{
						Count: intPtr(1),
						Entries: []kyvernov1.Attestor{{
							Keyless: &kyvernov1.KeylessAttestor{
								Roots: `-----BEGIN CERTIFICATE-----
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
							},
						}},
					}},
				}},
			}},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			policies, _, err := yamlutils.GetPolicy([]byte(test.policy))
			assert.NilError(t, err)
			assert.Equal(t, 1, len(policies))
			rules := computeRules(policies[0])
			assert.DeepEqual(t, test.expectedRules, rules)
		})
	}
}

func Test_PodSecurityWithNoExceptions(t *testing.T) {
	policy := []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"pod-security"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"restricted","match":{"all":[{"resources":{"kinds":["Pod"]}}]},"validate":{"podSecurity":{"level":"restricted","version":"v1.24"}}}]}}`)
	policies, _, err := yamlutils.GetPolicy([]byte(policy))
	assert.NilError(t, err)
	assert.Equal(t, 1, len(policies))

	rules := computeRules(policies[0])
	assert.Equal(t, 3, len(rules))
}

func Test_ValidateWithCELExpressions(t *testing.T) {
	policy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "disallow-host-path"
		},
		"spec": {
		  "validationFailureAction": "Enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "host-path",
			  "match": {
				"any": [
				  {
					"resources": {
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "validate": {
				"cel": {
				  "expressions": [
					{
					  "expression": "!has(object.spec.volumes) || object.spec.volumes.all(volume, !has(volume.hostPath))",
					  "message": "HostPath volumes are forbidden. The field spec.template.spec.volumes[*].hostPath must be unset."
					}
				  ]
				}
			  }
			}
		  ]
		}
	  }
`)
	policies, _, err := yamlutils.GetPolicy([]byte(policy))
	assert.NilError(t, err)
	assert.Equal(t, 1, len(policies))

	rules := computeRules(policies[0])
	assert.Equal(t, 3, len(rules))
}
