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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExcludeFunc is a function used to determine if a resource is excluded
type ExcludeFunc = func(kind, namespace, name string) bool

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

	Checkpoint()
	Restore()
	Reset()
}

type Engine interface {
	Validate(
		ctx context.Context,
		rclient registryclient.Client,
		policyContext *PolicyContext,
		cfg config.Configuration,
	) *EngineResponse
}
