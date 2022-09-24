package report

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const (
	//	admission request labels
	LabelRequestGroup     = "audit.kyverno.io/request.group"
	LabelRequestKind      = "audit.kyverno.io/request.kind"
	LabelRequestName      = "audit.kyverno.io/request.name"
	LabelRequestNamespace = "audit.kyverno.io/request.namespace"
	LabelRequestUid       = "audit.kyverno.io/request.uid"
	LabelRequestVersion   = "audit.kyverno.io/request.version"
	//	resource labels
	LabelResourceHash      = "audit.kyverno.io/resource.hash"
	LabelResourceName      = "audit.kyverno.io/resource.name"
	LabelResourceNamespace = "audit.kyverno.io/resource.namespace"
	LabelResourceUid       = "audit.kyverno.io/resource.uid"
	//	resource gvk labels
	LabelResourceGvkGroup   = "audit.kyverno.io/resource.gvk.group"
	LabelResourceGvkKind    = "audit.kyverno.io/resource.gvk.kind"
	LabelResourceGvkVersion = "audit.kyverno.io/resource.gvk.version"
	//	policy labels
	LabelDomainClusterPolicy = "pol.kyverno.io"
	LabelDomainPolicy        = "cpol.kyverno.io"
	LabelPrefixClusterPolicy = LabelDomainClusterPolicy + "/"
	LabelPrefixPolicy        = LabelDomainPolicy + "/"
)

func IsPolicyLabel(label string) bool {
	return strings.HasPrefix(label, LabelPrefixPolicy) || strings.HasPrefix(label, LabelPrefixClusterPolicy)
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

func PolicyLabelPrefix(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return LabelPrefixPolicy
	}
	return LabelPrefixClusterPolicy
}

func PolicyLabelDomain(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return LabelDomainPolicy
	}
	return LabelDomainClusterPolicy
}

func PolicyLabel(policy kyvernov1.PolicyInterface) string {
	return PolicyLabelPrefix(policy) + policy.GetName()
}

func SetManagedByKyvernoLabel(obj metav1.Object) {
	controllerutils.SetLabel(obj, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
}

func SetAdmissionLabels(report kyvernov1alpha2.ReportInterface, request *admissionv1.AdmissionRequest) {
	controllerutils.SetLabel(report, LabelRequestGroup, request.Kind.Group)
	controllerutils.SetLabel(report, LabelRequestKind, request.Kind.Kind)
	controllerutils.SetLabel(report, LabelRequestName, request.Name)
	controllerutils.SetLabel(report, LabelRequestNamespace, request.Namespace)
	controllerutils.SetLabel(report, LabelRequestUid, string(request.UID))
	controllerutils.SetLabel(report, LabelRequestVersion, request.Kind.Version)
}

func SetResourceLabels(report kyvernov1alpha2.ReportInterface, namespace, name string, uid types.UID) {
	controllerutils.SetLabel(report, LabelResourceName, name)
	controllerutils.SetLabel(report, LabelResourceNamespace, namespace)
	controllerutils.SetLabel(report, LabelResourceUid, string(uid))
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

func SetResourceVersionLabels(report kyvernov1alpha2.ReportInterface, resource *unstructured.Unstructured) {
	if resource != nil {
		controllerutils.SetLabel(report, LabelResourceHash, CalculateResourceHash(*resource))
	} else {
		controllerutils.SetLabel(report, LabelResourceHash, "")
	}
}

func SetResourceGvkLabels(report kyvernov1alpha2.ReportInterface, group, version, kind string) {
	controllerutils.SetLabel(report, LabelResourceGvkGroup, group)
	controllerutils.SetLabel(report, LabelResourceGvkKind, kind)
	controllerutils.SetLabel(report, LabelResourceGvkVersion, version)
}

func SetPolicyLabel(report kyvernov1alpha2.ReportInterface, policy kyvernov1.PolicyInterface) {
	controllerutils.SetLabel(report, PolicyLabel(policy), policy.GetResourceVersion())
}

func GetResourceUid(report metav1.Object) types.UID {
	return types.UID(report.GetLabels()[LabelResourceUid])
}

func GetResourceHash(report metav1.Object) string {
	return report.GetLabels()[LabelResourceHash]
}

func CompareHash(report metav1.Object, hash string) bool {
	return GetResourceHash(report) == hash
}
