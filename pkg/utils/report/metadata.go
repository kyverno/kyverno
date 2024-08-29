package report

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	LabelDomain = "kyverno.io"
	//	resource labels
	LabelResourceHash           = "audit.kyverno.io/resource.hash"
	LabelResourceUid            = "audit.kyverno.io/resource.uid"
	LabelResourceGVR            = "audit.kyverno.io/resource.gvr"
	LabelResourceGroup          = "audit.kyverno.io/resource.group"
	LabelResourceVersion        = "audit.kyverno.io/resource.version"
	LabelResourceKind           = "audit.kyverno.io/resource.kind"
	LabelSource                 = "audit.kyverno.io/source"
	AnnotationResourceNamespace = "audit.kyverno.io/resource.namespace"
	AnnotationResourceName      = "audit.kyverno.io/resource.name"
	//	policy labels
	LabelDomainClusterPolicy                    = "cpol.kyverno.io"
	LabelDomainPolicy                           = "pol.kyverno.io"
	LabelPrefixClusterPolicy                    = LabelDomainClusterPolicy + "/"
	LabelPrefixPolicy                           = LabelDomainPolicy + "/"
	LabelPrefixPolicyException                  = "polex.kyverno.io/"
	LabelPrefixValidatingAdmissionPolicy        = "validatingadmissionpolicy.apiserver.io/"
	LabelPrefixValidatingAdmissionPolicyBinding = "validatingadmissionpolicybinding.apiserver.io/"
	//	aggregated admission report label
	LabelAggregatedReport = "audit.kyverno.io/report.aggregate"
)

func IsPolicyLabel(label string) bool {
	return strings.HasPrefix(label, LabelPrefixPolicy) ||
		strings.HasPrefix(label, LabelPrefixClusterPolicy) ||
		strings.HasPrefix(label, LabelPrefixPolicyException) ||
		strings.HasPrefix(label, LabelPrefixValidatingAdmissionPolicy) ||
		strings.HasPrefix(label, LabelPrefixValidatingAdmissionPolicyBinding)
}

func PolicyNameFromLabel(namespace, label string) (string, error) {
	names := strings.Split(label, "/")
	if len(names) == 2 {
		if names[0] == LabelDomainClusterPolicy {
			return names[1], nil
		} else if names[0] == LabelDomainPolicy {
			return namespace + "/" + names[1], nil
		}
	}
	return "", fmt.Errorf("cannot get policy name from label, incorrect format: %s", label)
}

func PolicyLabelPrefix(policy engineapi.GenericPolicy) string {
	if policy.IsNamespaced() {
		return LabelPrefixPolicy
	}
	if policy.GetType() == engineapi.KyvernoPolicyType {
		return LabelPrefixClusterPolicy
	}
	return LabelPrefixValidatingAdmissionPolicy
}

func PolicyLabelDomain(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return LabelDomainPolicy
	}
	return LabelDomainClusterPolicy
}

func PolicyLabel(policy engineapi.GenericPolicy) string {
	return PolicyLabelPrefix(policy) + policy.GetName()
}

func PolicyExceptionLabel(exception kyvernov2.PolicyException) string {
	return LabelPrefixPolicyException + exception.GetName()
}

func ValidatingAdmissionPolicyBindingLabel(binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) string {
	return LabelPrefixValidatingAdmissionPolicyBinding + binding.GetName()
}

func CleanupKyvernoLabels(obj metav1.Object) {
	labels := obj.GetLabels()
	for key := range labels {
		if strings.Contains(key, LabelDomain) {
			delete(labels, key)
		}
	}
}

func SetManagedByKyvernoLabel(obj metav1.Object) {
	controllerutils.SetLabel(obj, kyverno.LabelAppManagedBy, kyverno.ValueKyvernoApp)
}

func SetSource(obj metav1.Object, source string) {
	controllerutils.SetLabel(obj, LabelSource, source)
}

func SetResourceUid(report reportsv1.ReportInterface, uid types.UID) {
	controllerutils.SetLabel(report, LabelResourceUid, string(uid))
}

func SetResourceGVR(report reportsv1.ReportInterface, gvr schema.GroupVersionResource) {
	if gvr.Group != "" {
		controllerutils.SetLabel(report, LabelResourceGVR, gvr.Resource+"."+gvr.Version+"."+gvr.Group)
	} else {
		controllerutils.SetLabel(report, LabelResourceGVR, gvr.Resource+"."+gvr.Version)
	}
}

func SetResourceGVK(report reportsv1.ReportInterface, gvk schema.GroupVersionKind) {
	controllerutils.SetLabel(report, LabelResourceGroup, gvk.Group)
	controllerutils.SetLabel(report, LabelResourceVersion, gvk.Version)
	controllerutils.SetLabel(report, LabelResourceKind, gvk.Kind)
}

func SetResourceNamespaceAndName(report reportsv1.ReportInterface, namespace, name string) {
	controllerutils.SetAnnotation(report, AnnotationResourceNamespace, namespace)
	controllerutils.SetAnnotation(report, AnnotationResourceName, name)
}

func CalculateResourceHash(resource unstructured.Unstructured) string {
	copy := resource.DeepCopy()
	obj := copy.Object
	labels := copy.GetLabels()
	annotations := copy.GetAnnotations()
	unstructured.RemoveNestedField(obj, "metadata")
	unstructured.RemoveNestedField(obj, "status")
	unstructured.RemoveNestedField(obj, "scale")
	// fix for pods
	unstructured.RemoveNestedField(obj, "spec", "nodeName")
	input := []interface{}{labels, annotations, obj}
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	hash := md5.Sum(data) //nolint:gosec
	return hex.EncodeToString(hash[:])
}

func SetResourceVersionLabels(report reportsv1.ReportInterface, resource *unstructured.Unstructured) {
	if resource != nil {
		controllerutils.SetLabel(report, LabelResourceHash, CalculateResourceHash(*resource))
	} else {
		controllerutils.SetLabel(report, LabelResourceHash, "")
	}
}

func SetPolicyLabel(report reportsv1.ReportInterface, policy engineapi.GenericPolicy) {
	controllerutils.SetLabel(report, PolicyLabel(policy), policy.GetResourceVersion())
}

func SetPolicyExceptionLabel(report reportsv1.ReportInterface, exception kyvernov2.PolicyException) {
	controllerutils.SetLabel(report, PolicyExceptionLabel(exception), exception.GetResourceVersion())
}

func SetValidatingAdmissionPolicyBindingLabel(report reportsv1.ReportInterface, binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) {
	controllerutils.SetLabel(report, ValidatingAdmissionPolicyBindingLabel(binding), binding.GetResourceVersion())
}

func GetSource(report metav1.Object) string {
	return controllerutils.GetLabel(report, LabelSource)
}

func GetResourceUid(report metav1.Object) types.UID {
	return types.UID(controllerutils.GetLabel(report, LabelResourceUid))
}

func GetResourceGVR(report metav1.Object) schema.GroupVersionResource {
	arg := controllerutils.GetLabel(report, LabelResourceGVR)
	dots := strings.Count(arg, ".")
	if dots >= 2 {
		s := strings.SplitN(arg, ".", 3)
		return schema.GroupVersionResource{Group: s[2], Version: s[1], Resource: s[0]}
	} else if dots == 1 {
		s := strings.SplitN(arg, ".", 2)
		return schema.GroupVersionResource{Version: s[1], Resource: s[0]}
	}
	return schema.GroupVersionResource{Resource: arg}
}

func GetResourceNamespaceAndName(report metav1.Object) (string, string) {
	return controllerutils.GetAnnotation(report, AnnotationResourceNamespace), controllerutils.GetAnnotation(report, AnnotationResourceName)
}

func GetResourceHash(report metav1.Object) string {
	return report.GetLabels()[LabelResourceHash]
}

func CompareHash(report metav1.Object, hash string) bool {
	return GetResourceHash(report) == hash
}
