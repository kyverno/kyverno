package webhooks

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

// DebugModeOptions holds the options to configure debug mode
type DebugModeOptions struct {
	// DumpPayload is used to activate/deactivate debug mode.
	DumpPayload bool
}

type Handler interface {
	Execute(context.Context, logr.Logger, handlers.AdmissionRequest, string, time.Time) admissionv1.AdmissionResponse
}

type HandlerFunc func(context.Context, logr.Logger, handlers.AdmissionRequest, string, time.Time) admissionv1.AdmissionResponse

func (f HandlerFunc) Execute(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) admissionv1.AdmissionResponse {
	return f(ctx, logger, request, failurePolicy, startTime)
}

type CELExceptionHandlers struct {
	// Validation performs the validation check on cel exception resources
	Validation Handler
}

type ExceptionHandlers struct {
	// Validation performs the validation check on exception resources
	Validation Handler
}

type GlobalContextHandlers struct {
	// Validation performs the validation check on global context entries
	Validation Handler
}

type PolicyHandlers struct {
	// Mutation performs the mutation of policy resources
	Mutation Handler
	// Validation performs the validation check on policy resources
	Validation Handler
}

type ResourceHandlers struct {
	// ValidatingPolicies evaluates validating policies against kube resources
	ValidatingPolicies Handler
	// NamespacedValidatingPolicies evaluates namespaced validating policies against kube resources
	NamespacedValidatingPolicies Handler
	// ImageVerificationPolicies evaluates imageverificationpolicies mutation phase against kube resources
	ImageVerificationPoliciesMutation Handler
	// NamespacedImageVerificationPoliciesMutation evaluates namespaced imageverificationpolicies mutation phase against kube resources
	NamespacedImageVerificationPoliciesMutation Handler
	// ImageVerificationPolicies evaluates imageverificationpolicies validation phase against kube resources
	ImageVerificationPolicies Handler
	// NamespacedImageVerificationPolicies evaluates namespaced imageverificationpolicies validation phase against kube resources
	NamespacedImageVerificationPolicies Handler
	// GeneratingPolicies evaluates generating policies against kube resources
	GeneratingPolicies Handler
	// NamespacedGeneratingPolicies evaluates namespaced generating policies against kube resources
	NamespacedGeneratingPolicies Handler
	// MutatingPolicies evaluates mutating policies against kube resources
	MutatingPolicies Handler
	// NamespacedMutatingPolicies evaluates namespaced mutating policies against kube resources
	NamespacedMutatingPolicies Handler
	// AuthorizingPolicies serves the authorization webhook endpoints (SubjectAccessReview
	// and AuthorizationConditionsReview). Nil disables the endpoints.
	AuthorizingPolicies AuthorizingPolicyHandler
}

// AuthorizingPolicyHandler serves the Kubernetes authorization webhook protocol for AuthorizingPolicy.
type AuthorizingPolicyHandler interface {
	// HandleSubjectAccessReview serves authorization.k8s.io/v1 SubjectAccessReview requests.
	HandleSubjectAccessReview(w http.ResponseWriter, r *http.Request)
	// HandleConditionsReview serves authorization.k8s.io/v1alpha1 AuthorizationConditionsReview requests.
	HandleConditionsReview(w http.ResponseWriter, r *http.Request)
	// HandleCombined serves both kinds on one endpoint and dispatches on TypeMeta.
	HandleCombined(w http.ResponseWriter, r *http.Request)
}
