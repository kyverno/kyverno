package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ExcludeFunc is a function used to determine if a resource is excluded
type ExcludeFunc = func(kind, namespace, name string) bool

type PolicyContext interface {
	Policy() kyvernov1.PolicyInterface
	NewResource() unstructured.Unstructured
	OldResource() unstructured.Unstructured
	AdmissionInfo() kyvernov1beta1.RequestInfo
	Operation() kyvernov1.AdmissionOperation
	NamespaceLabels() map[string]string
	RequestResource() metav1.GroupVersionResource
	ResourceKind() (schema.GroupVersionKind, string)
	AdmissionOperation() bool
	Element() unstructured.Unstructured
	SetElement(element unstructured.Unstructured)

	OldPolicyContext() (PolicyContext, error)
	JSONContext() enginecontext.Interface
	Copy() PolicyContext
}
