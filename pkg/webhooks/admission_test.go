package webhooks_test

import (
	"testing"

	"gotest.tools/assert"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/webhooks"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAdmissionIsRequired(t *testing.T) {
	var request v1beta1.AdmissionRequest
	request.Kind.Kind = "ConfigMap"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "CronJob"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "DaemonSet"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Deployment"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Endpoints"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "HorizontalPodAutoscaler"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Ingress"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Job"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "LimitRange"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Namespace"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "NetworkPolicy"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PersistentVolumeClaim"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PodDisruptionBudget"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PodTemplate"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "ResourceQuota"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Secret"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Service"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "StatefulSet"
	assert.Assert(t, webhooks.AdmissionIsRequired(&request))
}

func TestIsRuleResourceFitsRequest_Kind(t *testing.T) {
	resourceName := "test-config-map"
	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Name: &resourceName,
	}
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	objectByteArray := []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray

	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))
	resource.Kind = "Deployment"
	assert.Assert(t, false == webhooks.IsRuleApplicableToRequest(resource, &request))
}

func TestIsRuleResourceFitsRequest_Name(t *testing.T) {
	resourceName := "test-config-map"
	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Name: &resourceName,
	}
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	objectByteArray := []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))
	resourceName = "test-config-map-new"
	assert.Assert(t, false == webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"test-config-map-new","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assert.Assert(t, false == webhooks.IsRuleApplicableToRequest(resource, &request))
}

func TestIsRuleResourceFitsRequest_MatchExpressions(t *testing.T) {
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
				metav1.LabelSelectorRequirement{
					Key:      "label1",
					Operator: "In",
					Values: []string{
						"test1",
						"test8",
						"test201",
					},
				},
				metav1.LabelSelectorRequirement{
					Key:      "label3",
					Operator: "DoesNotExist",
					Values:   nil,
				},
			},
		},
	}

	objectByteArray := []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray

	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))
}

func TestIsRuleResourceFitsRequest_MatchLabels(t *testing.T) {
	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
				"label2": "test2",
			},
			MatchExpressions: nil,
		},
	}

	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	objectByteArray := []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label3":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assert.Assert(t, false == webhooks.IsRuleApplicableToRequest(resource, &request))

	resource = types.PolicyResource{
		Kind: "ConfigMap",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label3": "test1",
				"label2": "test2",
			},
			MatchExpressions: nil,
		},
	}

	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))
}

func TestIsRuleResourceFitsRequest_MatchLabelsAndMatchExpressions(t *testing.T) {
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "In",
					Values: []string{
						"test2",
					},
				},
			},
		},
	}

	objectByteArray := []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray

	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))

	resource = types.PolicyResource{
		Kind: "ConfigMap",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
			},
		},
	}

	objectByteArray = []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray

	assert.Assert(t, webhooks.IsRuleApplicableToRequest(resource, &request))
}
