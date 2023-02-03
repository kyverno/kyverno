package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExcludeFunc is a function used to determine if a resource is excluded
type ExcludeFunc = func(kind, namespace, name string) bool

type SubResource struct {
	APIResource    metav1.APIResource
	ParentResource metav1.APIResource
}

type PolicyContext interface {
	Policy() kyvernov1.PolicyInterface
	NewResource() unstructured.Unstructured
	OldResource() unstructured.Unstructured
	AdmissionInfo() kyvernov1beta1.RequestInfo
	NamespaceLabels() map[string]string
	SubResource() string
	SubresourcesInPolicy() []SubResource
	AdmissionOperation() bool
	RequestResource() metav1.GroupVersionResource
	Element() unstructured.Unstructured
	SetElement(element unstructured.Unstructured)

	JSONContext() enginecontext.Interface
	Client() dclient.Interface
	Copy() PolicyContext

	FindExceptions(rule string) ([]*kyvernov2alpha1.PolicyException, error)
}
