package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/registryclient"
	corev1 "k8s.io/api/core/v1"
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
	NewResourcePtr() *unstructured.Unstructured
	OldResourcePtr() *unstructured.Unstructured
	AdmissionInfo() kyvernov1beta1.RequestInfo
	JSONContext() enginecontext.Interface
	FindExceptions(rule string) ([]*kyvernov2alpha1.PolicyException, error)
	Client() dclient.Interface
	NamespaceLabels() map[string]string
	SubResource() string
	ExcludeResourceFunc() ExcludeFunc
	ExcludeGroupRole() []string
	Copy() PolicyContext
	SubresourcesInPolicy() []SubResource
	ResolveConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error)
	AdmissionOperation() bool
	RequestResource() metav1.GroupVersionResource
	Element() unstructured.Unstructured
	SetElement(element unstructured.Unstructured)
}

type Engine interface {
	Validate(
		ctx context.Context,
		rclient registryclient.Client,
		policyContext *PolicyContext,
		cfg config.Configuration,
	) *EngineResponse
}
