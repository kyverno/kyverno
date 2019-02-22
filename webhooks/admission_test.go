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
	request.Kind.Kind = "Endpoint"
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
	resource := types.PolicyResource{
		Kind: "ConfigMap",
	}
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
	}

	assertEq(t, true, webhooks.IsRuleResourceFitsRequest(resource, &request))
	resource.Kind = "Deployment"
	assertEq(t, false, webhooks.IsRuleResourceFitsRequest(resource, &request))
}

func TestIsRuleResourceFitsRequest_Name(t *testing.T) {
	resourceName := "test-config-map"
	resource := types.PolicyResource{
		Kind: "ConfigMap",
		Name: &resourceName,
	}
	request := v1beta1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{Kind: "ConfigMap"},
		Name: "test-config-map",
	}

	assertEq(t, true, webhooks.IsRuleResourceFitsRequest(resource, &request))
	resourceName = "test-config-map-new"
	assertEq(t, false, webhooks.IsRuleResourceFitsRequest(resource, &request))
	request.Name = "test-config-map-new"
	assertEq(t, true, webhooks.IsRuleResourceFitsRequest(resource, &request))
	request.Name = ""
	assertEq(t, false, webhooks.IsRuleResourceFitsRequest(resource, &request))
}

func TestIsRuleApplicableToRequest(t *testing.T) {
	// TODO
}
