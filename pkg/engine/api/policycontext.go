package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
	SetResources(oldResource, newResource unstructured.Unstructured) error
	SetOperation(kyvernov1.AdmissionOperation) error
	AdmissionInfo() kyvernov2.RequestInfo
	Operation() kyvernov1.AdmissionOperation
	NamespaceLabels() map[string]string
	RequestResource() metav1.GroupVersionResource
	ResourceKind() (schema.GroupVersionKind, string)
	AdmissionOperation() bool
	Element() unstructured.Unstructured
	SetElement(element unstructured.Unstructured)

	JSONContext() enginecontext.Interface
	Copy() PolicyContext
}
