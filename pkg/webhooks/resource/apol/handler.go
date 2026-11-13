// Package apol serves the Kubernetes authorization webhook protocol for
// AuthorizingPolicy: typed authorization.k8s.io/v1 SubjectAccessReview
// requests (with conditional decisions negotiated through
// spec.authorizationOptions.handledDecisionTypes) and
// authorization.k8s.io/v1alpha1 AuthorizationConditionsReview callbacks.
package apol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	authorizationv1 "k8s.io/api/authorization/v1"
	authorizationv1alpha1 "k8s.io/api/authorization/v1alpha1"
	"k8s.io/apimachinery/pkg/api/operation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	authorizationvalidation "k8s.io/apiserver/pkg/apis/authorization/validation"
	genericfeatures "k8s.io/apiserver/pkg/features"
)

var logger = logging.WithName("apol-webhook")

const (
	// MaxRequestBytes bounds request bodies. An AuthorizationConditionsReview
	// carries up to two objects of at most a few MiB each.
	MaxRequestBytes = 16 << 20

	// CombinedPath accepts both kinds and dispatches on TypeMeta. Use it when
	// conditionsReview.kubeConfigContextName is not set, because kube-apiserver
	// then sends both payloads to the same endpoint.
	CombinedPath = config.AuthorizingPolicyCombinedPath
)

var (
	sarGVK = authorizationv1.SchemeGroupVersion.WithKind("SubjectAccessReview")
	acrGVK = authorizationv1alpha1.SchemeGroupVersion.WithKind("AuthorizationConditionsReview")
	// validationOp evaluates upstream declarative validation as a
	// kube-apiserver with the ConditionalAuthorization gate enabled would.
	validationOp = operation.Operation{
		Type:    operation.Create,
		Options: map[string]bool{string(genericfeatures.ConditionalAuthorization): true},
	}
)

// Handler serves the authorization webhook endpoints.
type Handler struct {
	engine apolengine.Engine
}

// New returns a Handler using the given engine.
func New(engine apolengine.Engine) *Handler {
	return &Handler{engine: engine}
}

// HandleSubjectAccessReview serves SubjectAccessReview requests only.
func (h *Handler) HandleSubjectAccessReview(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, sarGVK.Kind)
}

// HandleConditionsReview serves AuthorizationConditionsReview requests only.
func (h *Handler) HandleConditionsReview(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, acrGVK.Kind)
}

// HandleCombined dispatches on apiVersion and kind.
func (h *Handler) HandleCombined(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "")
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, onlyKind string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxRequestBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("reading request body: %w", err))
		return
	}
	resp, status, err := h.Review(r.Context(), body, onlyKind)
	if err != nil {
		logger.V(2).Info("rejecting authorization webhook request", "error", err.Error())
		writeError(w, status, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(resp); err != nil {
		logger.Error(err, "failed to write authorization webhook response")
	}
}

// Review decodes one request body, dispatches on TypeMeta, and returns the
// encoded response. A non-nil error carries the HTTP status to return;
// kube-apiserver treats such failures according to the webhook failurePolicy
// (and fails closed for conditions reviews).
func (h *Handler) Review(ctx context.Context, body []byte, onlyKind string) ([]byte, int, error) {
	var tm metav1.TypeMeta
	if err := json.Unmarshal(body, &tm); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("decoding TypeMeta: %w", err)
	}
	gvk := tm.GroupVersionKind()
	if onlyKind != "" && gvk.Kind != onlyKind {
		return nil, http.StatusBadRequest, fmt.Errorf("this endpoint serves %s, got %q", onlyKind, gvk.Kind)
	}
	switch gvk {
	case sarGVK:
		var sar authorizationv1.SubjectAccessReview
		if err := json.Unmarshal(body, &sar); err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("decoding SubjectAccessReview: %w", err)
		}
		return encode(h.reviewSubjectAccess(ctx, &sar))
	case acrGVK:
		var acr authorizationv1alpha1.AuthorizationConditionsReview
		if err := json.Unmarshal(body, &acr); err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("decoding AuthorizationConditionsReview: %w", err)
		}
		out, err := h.reviewConditions(ctx, &acr)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		return encode(out)
	default:
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported apiVersion/kind %q/%q; supported: %s, %s", tm.APIVersion, tm.Kind, sarGVK, acrGVK)
	}
}

func (h *Handler) reviewSubjectAccess(ctx context.Context, in *authorizationv1.SubjectAccessReview) *authorizationv1.SubjectAccessReview {
	out := &authorizationv1.SubjectAccessReview{
		TypeMeta:   metav1.TypeMeta{APIVersion: sarGVK.GroupVersion().String(), Kind: sarGVK.Kind},
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
	}
	conditionsAware := authorizationv1.SupportsConditionalAuthorization(in.Spec.AuthorizationOptions)
	request := &authorizationv1.SubjectAccessReview{Spec: in.Spec}
	errs := authorizationvalidation.ValidateSubjectAccessReviewSpec(in.Spec, field.NewPath("spec"))
	errs = append(errs, authorizationv1.Validate_SubjectAccessReview(ctx, validationOp, nil, request, nil)...)
	if len(errs) > 0 {
		// Malformed requests are denied: Kyverno cannot prove no Deny rule applies.
		out.Status = authorizationv1.SubjectAccessReviewStatus{Denied: true, Reason: "failed closed: invalid SubjectAccessReview", EvaluationError: errs.ToAggregate().Error()}
		return out
	}
	decision := h.engine.Authorize(ctx, &in.Spec, conditionsAware)
	out.Status = ToStatus(decision, conditionsAware)
	// Self-check against the exact validation kube-apiserver applies to webhook responses.
	errs = authorizationvalidation.ValidateSubjectAccessReviewStatus(out.Status, conditionsAware, field.NewPath("status"))
	errs = append(errs, authorizationv1.Validate_SubjectAccessReview(ctx, validationOp, nil, out, nil)...)
	if len(errs) > 0 {
		logger.Error(errs.ToAggregate(), "produced an invalid SubjectAccessReview status, failing closed")
		out.Status = authorizationv1.SubjectAccessReviewStatus{Denied: true, Reason: "failed closed: invalid response", EvaluationError: errs.ToAggregate().Error()}
	}
	logger.V(4).Info("authorized", "user", in.Spec.User, "conditionsAware", conditionsAware, "decision", decision.Type)
	return out
}

// ToStatus maps an engine decision to SubjectAccessReviewStatus. A conditional
// decision is only ever returned to callers that opted in; for any other caller
// it fails closed (Deny if Deny is a possible outcome, otherwise NoOpinion).
func ToStatus(d authorizationv1.ConditionsAwareDecision, conditionsAware bool) authorizationv1.SubjectAccessReviewStatus {
	switch d.Type {
	case authorizationv1.ConditionsAwareDecisionTypeAllow:
		return authorizationv1.SubjectAccessReviewStatus{Allowed: true, Reason: reason(d.Allow), EvaluationError: evalError(d.Allow)}
	case authorizationv1.ConditionsAwareDecisionTypeDeny:
		return authorizationv1.SubjectAccessReviewStatus{Denied: true, Reason: reason(d.Deny), EvaluationError: evalError(d.Deny)}
	case authorizationv1.ConditionsAwareDecisionTypeNoOpinion:
		return authorizationv1.SubjectAccessReviewStatus{Reason: reason(d.NoOpinion), EvaluationError: evalError(d.NoOpinion)}
	case authorizationv1.ConditionsAwareDecisionTypeConditionsMap:
		if conditionsAware && d.ConditionsMap != nil {
			// reason and evaluationError must be zero for conditional responses.
			return authorizationv1.SubjectAccessReviewStatus{ConditionalDecision: &d}
		}
		if d.ConditionsMap != nil && len(d.ConditionsMap.DenyConditions) > 0 {
			return authorizationv1.SubjectAccessReviewStatus{Denied: true, Reason: "failed closed: caller does not handle conditional decisions"}
		}
		return authorizationv1.SubjectAccessReviewStatus{Reason: "failed closed: caller does not handle conditional decisions"}
	default:
		return authorizationv1.SubjectAccessReviewStatus{Denied: true, Reason: "failed closed", EvaluationError: fmt.Sprintf("unsupported decision type %q", d.Type)}
	}
}

func (h *Handler) reviewConditions(ctx context.Context, in *authorizationv1alpha1.AuthorizationConditionsReview) (*authorizationv1alpha1.AuthorizationConditionsReview, error) {
	if in.Request == nil {
		return nil, errors.New("request is required")
	}
	if in.Request.AdmissionRequest == nil {
		return nil, errors.New("request.admissionRequest is required to echo response.uid")
	}
	if in.Request.AdmissionRequest.UID == "" {
		return nil, errors.New("request.admissionRequest.uid is required")
	}
	decision := h.engine.EvaluateConditions(ctx, &in.Request.Decision, in.Request.AdmissionRequest)
	out := &authorizationv1alpha1.AuthorizationConditionsReview{
		TypeMeta: metav1.TypeMeta{APIVersion: acrGVK.GroupVersion().String(), Kind: acrGVK.Kind},
		Response: &authorizationv1alpha1.AuthorizationConditionsResponse{
			UID:      in.Request.AdmissionRequest.UID,
			Decision: decision,
		},
	}
	errs := authorizationvalidation.ValidateAuthorizationConditionsReview(out)
	errs = append(errs, authorizationv1alpha1.Validate_AuthorizationConditionsReview(ctx, validationOp, nil, out, nil)...)
	if len(errs) > 0 {
		logger.Error(errs.ToAggregate(), "produced an invalid AuthorizationConditionsReview response, failing closed")
		out.Response.Decision = authorizationv1.ConditionsAwareDecision{
			Type: authorizationv1.ConditionsAwareDecisionTypeDeny,
			Deny: &authorizationv1.UnconditionalDecision{Reason: "failed closed: invalid response", EvaluationError: errs.ToAggregate().Error()},
		}
	}
	return out, nil
}

func encode(obj any) ([]byte, int, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return raw, http.StatusOK, nil
}

func writeError(w http.ResponseWriter, status int, err error) {
	http.Error(w, err.Error(), status)
}

func reason(u *authorizationv1.UnconditionalDecision) string {
	if u == nil {
		return ""
	}
	return u.Reason
}

func evalError(u *authorizationv1.UnconditionalDecision) string {
	if u == nil {
		return ""
	}
	return u.EvaluationError
}
