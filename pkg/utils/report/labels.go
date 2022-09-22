package report

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	LabelResourceGeneration = "audit.kyverno.io/resource.generation"
	LabelResourceHash       = "audit.kyverno.io/resource.hash"
	LabelResourceName       = "audit.kyverno.io/resource.name"
	LabelResourceNamespace  = "audit.kyverno.io/resource.namespace"
	LabelResourceUid        = "audit.kyverno.io/resource.uid"
	LabelResourceVersion    = "audit.kyverno.io/resource.version"
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

func SetAdmissionLabels(report kyvernov1alpha2.ReportChangeRequestInterface, request *admissionv1.AdmissionRequest) {
	controllerutils.SetLabel(report, LabelRequestGroup, request.Kind.Group)
	controllerutils.SetLabel(report, LabelRequestKind, request.Kind.Kind)
	controllerutils.SetLabel(report, LabelRequestName, request.Name)
	controllerutils.SetLabel(report, LabelRequestNamespace, request.Namespace)
	controllerutils.SetLabel(report, LabelRequestUid, string(request.UID))
	controllerutils.SetLabel(report, LabelRequestVersion, request.Kind.Version)
}

func SetResourceLabels(report kyvernov1alpha2.ReportChangeRequestInterface, resource metav1.Object) {
	controllerutils.SetLabel(report, LabelResourceName, resource.GetName())
	controllerutils.SetLabel(report, LabelResourceNamespace, resource.GetNamespace())
	controllerutils.SetLabel(report, LabelResourceUid, string(resource.GetUID()))
	controllerutils.SetLabel(report, LabelResourceVersion, resource.GetResourceVersion())
	SetResourceVersionLabels(report, resource)
}

func CalculateResourceHash(resource metav1.Object) string {
	var input []interface{}
	for _, entry := range resource.GetManagedFields() {
		if entry.Subresource == "" {
			input = append(input, entry)
		}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func SetResourceVersionLabels(report kyvernov1alpha2.ReportChangeRequestInterface, resource metav1.Object) {
	if resource != nil {
		controllerutils.SetLabel(report, LabelResourceHash, CalculateResourceHash(resource))
		controllerutils.SetLabel(report, LabelResourceGeneration, strconv.FormatInt(resource.GetGeneration(), 10))
		controllerutils.SetLabel(report, LabelResourceVersion, resource.GetResourceVersion())
	} else {
		controllerutils.SetLabel(report, LabelResourceHash, "")
		controllerutils.SetLabel(report, LabelResourceGeneration, "")
		controllerutils.SetLabel(report, LabelResourceVersion, "")
	}
}

func SetResourceGvkLabels(report kyvernov1alpha2.ReportChangeRequestInterface, group, version, kind string) {
	controllerutils.SetLabel(report, LabelResourceGvkGroup, group)
	controllerutils.SetLabel(report, LabelResourceGvkKind, kind)
	controllerutils.SetLabel(report, LabelResourceGvkVersion, version)
}

func SetPolicyLabel(report kyvernov1alpha2.ReportChangeRequestInterface, policy kyvernov1.PolicyInterface) {
	controllerutils.SetLabel(report, PolicyLabel(policy), policy.GetResourceVersion())
}

func GetResourceUid(report kyvernov1alpha2.ReportChangeRequestInterface) types.UID {
	return types.UID(report.GetLabels()[LabelResourceUid])
}

func GetResourceVersion(report kyvernov1alpha2.ReportChangeRequestInterface) string {
	return report.GetLabels()[LabelResourceVersion]
}

func GetResourceHash(report kyvernov1alpha2.ReportChangeRequestInterface) string {
	return report.GetLabels()[LabelResourceHash]
}

func CompareHash(report kyvernov1alpha2.ReportChangeRequestInterface, resource metav1.Object) bool {
	return GetResourceHash(report) == CalculateResourceHash(resource)
}
