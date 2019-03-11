package webhooks_test

import (
	"testing"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/webhooks"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAdmissionIsRequired(t *testing.T) {
	var request v1beta1.AdmissionRequest
	request.Kind.Kind = "ConfigMap"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "CronJob"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "DaemonSet"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Deployment"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Endpoints"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "HorizontalPodAutoscaler"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Ingress"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Job"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "LimitRange"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Namespace"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "NetworkPolicy"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PersistentVolumeClaim"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PodDisruptionBudget"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "PodTemplate"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "ResourceQuota"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Secret"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "Service"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
	request.Kind.Kind = "StatefulSet"
	assertEq(t, true, webhooks.AdmissionIsRequired(&request))
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

	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))
	resource.Kind = "Deployment"
	assertEq(t, false, webhooks.IsRuleApplicableToRequest(resource, &request))
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
	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))
	resourceName = "test-config-map-new"
	assertEq(t, false, webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"test-config-map-new","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"","namespace":"default","creationTimestamp":null,"labels":{"label1":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assertEq(t, false, webhooks.IsRuleApplicableToRequest(resource, &request))
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

	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))
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
	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))

	objectByteArray = []byte(`{"metadata":{"name":"test-config-map","namespace":"default","creationTimestamp":null,"labels":{"label3":"test1","label2":"test2"}}}`)
	request.Object.Raw = objectByteArray
	assertEq(t, false, webhooks.IsRuleApplicableToRequest(resource, &request))

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

	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))
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

	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))

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

	assertEq(t, true, webhooks.IsRuleApplicableToRequest(resource, &request))
}
