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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const (
	//	resource labels
	LabelResourceHash = "audit.kyverno.io/resource.hash"
	LabelResourceUid  = "audit.kyverno.io/resource.uid"
	//	policy labels
	LabelDomainClusterPolicy = "cpol.kyverno.io"
	LabelDomainPolicy        = "pol.kyverno.io"
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

func SetResourceLabels(report kyvernov1alpha2.ReportInterface, uid types.UID) {
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
