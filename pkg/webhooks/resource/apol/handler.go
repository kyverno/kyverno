package apol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/kyverno/kyverno/pkg/logging"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
)

const (
	SARPath              = "/authz/subjectaccessreview"
	ConditionsReviewPath = "/authz/conditions"

	// ConditionsModeHumanReadable is the only currently recognised value for the
	// spec.conditionalAuthorization.mode field on a SubjectAccessReview request.
	ConditionsModeHumanReadable = "HumanReadable"
)

var logger = logging.WithName("apol-handler")

// Provider supplies compiled AuthorizingPolicy instances.
type Provider interface {
	Fetch(ctx context.Context) ([]*apolcompiler.Policy, error)
}

// Handler manages authorization decisions for AuthorizingPolicy resources.
type Handler interface {
	HandleSubjectAccessReview(w http.ResponseWriter, r *http.Request)
	HandleConditionsReview(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	provider Provider
}

// authorizationConditionsReview models the KEP-5681 callback shape.
type authorizationConditionsReview struct {
	APIVersion string                           `json:"apiVersion,omitempty"`
	Kind       string                           `json:"kind,omitempty"`
	Request    *authorizationConditionsRequest  `json:"request,omitempty"`
	Response   *authorizationConditionsResponse `json:"response,omitempty"`
}

type authorizationConditionsRequest struct {
	ConditionSetChain []sarConditionSet                       `json:"conditionSetChain,omitempty"`
	Spec              authorizationv1.SubjectAccessReviewSpec `json:"spec,omitempty"`
	Object            map[string]any                          `json:"object,omitempty"`
	OldObject         map[string]any                          `json:"oldObject,omitempty"`
}

type authorizationConditionsResponse struct {
	Allowed           bool              `json:"allowed"`
	Denied            bool              `json:"denied,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	EvaluationError   string            `json:"evaluationError,omitempty"`
	ConditionSetChain []sarConditionSet `json:"conditionSetChain,omitempty"`
}

type sarCondition struct {
	ID          string `json:"id"`
	Effect      string `json:"effect"`
	Condition   string `json:"condition"`
	Description string `json:"description,omitempty"`
}

type sarConditionSet struct {
	Allowed           bool              `json:"allowed,omitempty"`
	Denied            bool              `json:"denied,omitempty"`
	FailureMode       string            `json:"failureMode,omitempty"`
	AuthorizerName    string            `json:"authorizerName"`
	ConditionsType    string            `json:"conditionsType"`
	Conditions        []sarCondition    `json:"conditions,omitempty"`
	ConditionSetChain []sarConditionSet `json:"conditionSetChain,omitempty"`
}

type sarStatusResponse struct {
	Allowed           bool              `json:"allowed"`
	Denied            bool              `json:"denied"`
	Reason            string            `json:"reason,omitempty"`
	EvaluationError   string            `json:"evaluationError,omitempty"`
	ConditionSetChain []sarConditionSet `json:"conditionSetChain,omitempty"`
}

type sarResponse struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Status     sarStatusResponse `json:"status"`
}

type conditionDecision struct {
	allowed         bool
	denied          bool
	reason          string
	evaluationError string
}

// New creates a new authorization policy handler backed by the given provider.
func New(provider Provider) Handler {
	return &handler{provider: provider}
}

// HandleSubjectAccessReview processes SubjectAccessReview requests.
func (h *handler) HandleSubjectAccessReview(w http.ResponseWriter, r *http.Request) {
	log := logger.WithValues("endpoint", "SAR")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var sar authorizationv1.SubjectAccessReview
	if err := json.Unmarshal(body, &sar); err != nil {
		log.Error(err, "failed to decode SubjectAccessReview")
		http.Error(w, fmt.Sprintf("failed to decode SubjectAccessReview: %v", err), http.StatusBadRequest)
		return
	}

	log = log.WithValues("user", sar.Spec.User)
	log.V(4).Info("processing SubjectAccessReview")

	decision := h.evaluate(r.Context(), &sar)
	status := sarStatusResponse{
		Allowed:         decision.Effect == policiesv1alpha1.AuthorizingRuleEffectAllow,
		Denied:          decision.Effect == policiesv1alpha1.AuthorizingRuleEffectDeny,
		Reason:          decision.Reason,
		EvaluationError: decision.EvaluationError,
	}

	mode := extractConditionsMode(body)
	if decision.Effect == policiesv1alpha1.AuthorizingRuleEffectConditional {
		if mode == "" {
			status.Allowed = false
			status.Denied = false
			if conditionalHasDeny(decision.ConditionSet) {
				status.Denied = true
				status.Reason = "conditional decision folded to deny when conditions mode is unset"
			} else {
				status.Reason = "conditional decision folded to no-opinion when conditions mode is unset"
			}
		} else {
			status.Allowed = false
			status.Denied = false
			conditions := make([]sarCondition, 0, len(decision.ConditionSet))
			for _, cond := range decision.ConditionSet {
				conditions = append(conditions, sarCondition{
					ID:          cond.ID,
					Effect:      string(cond.Effect),
					Condition:   cond.Condition,
					Description: cond.Description,
				})
			}
			status.ConditionSetChain = []sarConditionSet{{
				AuthorizerName: "kyverno",
				ConditionsType: "k8s.io/cel",
				FailureMode:    "Deny",
				Conditions:     conditions,
			}}
		}
	}

	response := sarResponse{
		APIVersion: sar.APIVersion,
		Kind:       sar.Kind,
		Status:     status,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error(err, "failed to encode response")
	}
}

// evaluate runs all compiled policies against the SAR and returns the first decisive decision.
func (h *handler) evaluate(ctx context.Context, sar *authorizationv1.SubjectAccessReview) apolengine.AuthorizationDecision {
	epolog := logger.WithValues(
		"user", sar.Spec.User,
		"uid", sar.Spec.UID,
		"groups", sar.Spec.Groups,
	)

	epolog.V(3).Info("starting APOL evaluation")

	policies, err := h.provider.Fetch(ctx)
	if err != nil {
		epolog.Error(err, "failed to fetch policies")
		return apolengine.AuthorizationDecision{
			Effect: policiesv1alpha1.AuthorizingRuleEffectNoOpinion,
			Reason: fmt.Sprintf("failed to fetch policies: %v", err),
		}
	}

	epolog.V(4).Info("fetched policies", "count", len(policies))

	req := requestToMap(sar)
	policyErrors := make([]string, 0)
	for _, compiled := range policies {
		policyLog := epolog.WithValues("policy", compiled.Name)
		policyLog.V(4).Info("evaluating policy", "rules", len(compiled.Rules))

		eng := apolengine.NewEngine(compiled)
		decision, err := eng.HandleSAR(ctx, req)
		if err != nil {
			policyLog.Error(err, "policy evaluation error")
			policyErrors = append(policyErrors, fmt.Sprintf("policy %q: %v", compiled.Name, err))
			continue
		}
		if decision.EvaluationError != "" {
			policyError := fmt.Sprintf("policy %q: %s", compiled.Name, decision.EvaluationError)
			if !slices.Contains(policyErrors, policyError) {
				policyErrors = append(policyErrors, policyError)
			}
			policyLog.V(2).Info("recorded policy evaluation error", "error", decision.EvaluationError)
		}

		policyLog.V(3).Info("policy evaluation result", "effect", decision.Effect, "reason", decision.Reason)

		if decision.Effect == policiesv1alpha1.AuthorizingRuleEffectAllow ||
			decision.Effect == policiesv1alpha1.AuthorizingRuleEffectDeny ||
			decision.Effect == policiesv1alpha1.AuthorizingRuleEffectConditional {
			policyLog.V(2).Info("policy matched with decisive decision", "effect", decision.Effect)
			return decision
		}

		policyLog.V(4).Info("policy did not return decisive decision")
	}

	if len(policyErrors) > 0 {
		evalError := strings.Join(policyErrors, "; ")
		epolog.V(2).Info("no policy matched and policy errors were recorded", "policyErrorCount", len(policyErrors))
		return apolengine.AuthorizationDecision{
			Effect:          policiesv1alpha1.AuthorizingRuleEffectNoOpinion,
			Reason:          "no policy matched",
			EvaluationError: evalError,
		}
	}

	epolog.V(2).Info("no policy matched")
	return apolengine.AuthorizationDecision{
		Effect: policiesv1alpha1.AuthorizingRuleEffectNoOpinion,
		Reason: "no policy matched",
	}
}

// HandleConditionsReview processes AuthorizationConditionsReview callback requests.
func (h *handler) HandleConditionsReview(w http.ResponseWriter, r *http.Request) {
	log := logger.WithValues("endpoint", "ConditionsReview")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	acr, requestActivation, err := parseConditionsReviewPayload(body)
	if err != nil {
		log.Error(err, "failed to decode conditions review payload")
		http.Error(w, fmt.Sprintf("failed to decode conditions review payload: %v", err), http.StatusBadRequest)
		return
	}

	log.V(4).Info("processing conditions review")

	if acr.Request == nil || len(acr.Request.ConditionSetChain) == 0 {
		http.Error(w, "request.conditionSetChain is required", http.StatusBadRequest)
		return
	}

	decision := evaluateConditionSetChain(r.Context(), acr.Request.ConditionSetChain, requestActivation)
	acr.Response = &authorizationConditionsResponse{
		Allowed:         decision.allowed,
		Denied:          decision.denied,
		Reason:          decision.reason,
		EvaluationError: decision.evaluationError,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(acr); err != nil {
		log.Error(err, "failed to encode response")
	}
}

// parseConditionsReviewPayload accepts AuthorizationConditionsReview payload.
func parseConditionsReviewPayload(body []byte) (*authorizationConditionsReview, map[string]any, error) {
	var acr authorizationConditionsReview
	if err := json.Unmarshal(body, &acr); err != nil {
		return nil, nil, err
	}
	if acr.Request == nil {
		return nil, nil, fmt.Errorf("missing request")
	}
	activation := requestToMapFromSpec(acr.Request.Spec, acr.Request.Object, acr.Request.OldObject)
	return &acr, activation, nil
}

// requestToMap converts a SubjectAccessReview into a map suitable for CEL evaluation.
func requestToMap(sar *authorizationv1.SubjectAccessReview) map[string]interface{} {
	return requestToMapFromSpec(sar.Spec, nil, nil)
}

func requestToMapFromSpec(spec authorizationv1.SubjectAccessReviewSpec, object, oldObject map[string]any) map[string]interface{} {
	result := map[string]interface{}{
		"user":   spec.User,
		"uid":    spec.UID,
		"groups": spec.Groups,
		"extra":  spec.Extra,
	}

	if spec.ResourceAttributes != nil {
		attrs := spec.ResourceAttributes
		result["verb"] = attrs.Verb
		result["namespace"] = attrs.Namespace
		result["name"] = attrs.Name
		result["resource"] = attrs.Resource
		result["subresource"] = attrs.Subresource
		result["apiGroup"] = attrs.Group
		result["apiVersion"] = attrs.Version
	}

	if spec.NonResourceAttributes != nil {
		nattrs := spec.NonResourceAttributes
		result["path"] = nattrs.Path
		result["verb"] = nattrs.Verb
	}

	if object != nil {
		result["object"] = object
		if labels := extractObjectLabels(object); labels != nil {
			result["resourceLabels"] = labels
		}
	}
	if oldObject != nil {
		result["oldObject"] = oldObject
		if labels := extractObjectLabels(oldObject); labels != nil {
			result["oldResourceLabels"] = labels
		}
	}

	return result
}

func extractObjectLabels(object map[string]any) map[string]any {
	metadataAny, ok := object["metadata"]
	if !ok {
		return nil
	}
	metadata, ok := metadataAny.(map[string]any)
	if !ok {
		return nil
	}
	labelsAny, ok := metadata["labels"]
	if !ok {
		return nil
	}
	labels, ok := labelsAny.(map[string]any)
	if !ok {
		return nil
	}
	return labels
}

func extractConditionsMode(body []byte) string {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	specAny, ok := raw["spec"]
	if !ok {
		return ""
	}
	spec, ok := specAny.(map[string]any)
	if !ok {
		return ""
	}

	var mode string
	if caAny, ok := spec["conditionalAuthorization"]; ok {
		if ca, ok := caAny.(map[string]any); ok {
			if modeAny, ok := ca["mode"]; ok {
				if m, ok := modeAny.(string); ok {
					mode = strings.TrimSpace(m)
				}
			}
		}
	}
	if mode == "" {
		if modeAny, ok := spec["conditionsMode"]; ok {
			if m, ok := modeAny.(string); ok {
				mode = strings.TrimSpace(m)
			}
		}
	}
	if mode == "" {
		return ""
	}

	switch mode {
	case ConditionsModeHumanReadable:
		return mode
	default:
		logger.Info("unrecognised conditionalAuthorization mode; folding to concrete decision", "mode", mode)
		return ""
	}
}

func conditionalHasDeny(conds []apolengine.Condition) bool {
	for _, cond := range conds {
		if cond.Effect == policiesv1alpha1.AuthorizingConditionEffectDeny {
			return true
		}
	}
	return false
}

func evaluateConditionSetChain(ctx context.Context, chain []sarConditionSet, requestActivation map[string]any) conditionDecision {
	allowed := false
	for _, set := range chain {
		decision := evaluateConditionSet(ctx, set, requestActivation)
		if decision.denied {
			return decision
		}
		if decision.evaluationError != "" {
			return decision
		}
		if decision.allowed {
			allowed = true
		}
	}
	if allowed {
		return conditionDecision{allowed: true, reason: "allow condition matched"}
	}
	return conditionDecision{reason: "no condition in set allowed the request"}
}

func evaluateConditionSet(ctx context.Context, set sarConditionSet, requestActivation map[string]any) conditionDecision {
	condLog := logger.WithValues("authorizerName", set.AuthorizerName)

	if set.Allowed {
		condLog.V(3).Info("condition set unconditionally allowed")
		return conditionDecision{allowed: true, reason: "unconditional allow condition set"}
	}
	if set.Denied {
		condLog.V(3).Info("condition set unconditionally denied")
		return conditionDecision{denied: true, reason: "unconditional deny condition set"}
	}
	if len(set.ConditionSetChain) > 0 {
		condLog.V(4).Info("evaluating nested condition set chain", "chainLength", len(set.ConditionSetChain))
		return evaluateConditionSetChain(ctx, set.ConditionSetChain, requestActivation)
	}

	env, err := buildConditionEvaluationEnv()
	if err != nil {
		condLog.Error(err, "failed to build condition evaluation environment")
		return conditionDecision{denied: true, reason: "condition environment init failed", evaluationError: err.Error()}
	}
	activation := map[string]any{
		celcompiler.RequestKey: requestActivation,
	}

	condLog.V(4).Info("evaluating conditions", "count", len(set.Conditions))

	allowTrue := false
	for _, cond := range set.Conditions {
		conditionLog := condLog.WithValues("conditionID", cond.ID, "effect", cond.Effect)
		conditionLog.V(4).Info("evaluating condition expression", "expression", cond.Condition)

		truthy, evalErr := evalCondition(ctx, env, cond.Condition, activation)
		if evalErr != nil {
			conditionLog.Error(evalErr, "condition evaluation error")
			switch cond.Effect {
			case string(policiesv1alpha1.AuthorizingConditionEffectDeny):
				if strings.EqualFold(set.FailureMode, string(policiesv1alpha1.AuthorizingRuleEffectNoOpinion)) {
					conditionLog.V(3).Info("deny condition failed with no-opinion failure mode")
					return conditionDecision{reason: "deny condition evaluation failed", evaluationError: evalErr.Error()}
				}
				conditionLog.V(3).Info("deny condition failed, applying deny decision")
				return conditionDecision{denied: true, reason: "deny condition evaluation failed", evaluationError: evalErr.Error()}
			case string(policiesv1alpha1.AuthorizingConditionEffectNoOpinion):
				conditionLog.V(3).Info("no-opinion condition failed")
				return conditionDecision{reason: "no-opinion condition evaluation failed", evaluationError: evalErr.Error()}
			default:
				conditionLog.V(4).Info("allow condition evaluation failed, ignoring error per KEP semantics")
				continue
			}
		}

		conditionLog.V(4).Info("condition evaluation result", "result", truthy)
		if !truthy {
			conditionLog.V(4).Info("condition expression returned false")
			continue
		}

		switch cond.Effect {
		case string(policiesv1alpha1.AuthorizingConditionEffectDeny):
			conditionLog.V(2).Info("condition denied the request")
			return conditionDecision{denied: true, reason: fmt.Sprintf("condition %q denied", cond.ID)}
		case string(policiesv1alpha1.AuthorizingConditionEffectNoOpinion):
			conditionLog.V(3).Info("condition returned no-opinion")
			return conditionDecision{reason: fmt.Sprintf("condition %q returned no-opinion", cond.ID)}
		case string(policiesv1alpha1.AuthorizingConditionEffectAllow):
			conditionLog.V(4).Info("allow condition matched")
			allowTrue = true
		}
	}

	if allowTrue {
		condLog.V(2).Info("at least one allow condition matched")
		return conditionDecision{allowed: true, reason: "allow condition matched"}
	}
	condLog.V(3).Info("no condition in set allowed the request")
	return conditionDecision{reason: "no condition in set allowed the request"}
}

func buildConditionEvaluationEnv() (*cel.Env, error) {
	base := environment.MustBaseEnvSet(version.MajorMinor(1, 0))
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, err
	}
	return env.Extend(cel.Variable(celcompiler.RequestKey, cel.DynType))
}

func evalCondition(ctx context.Context, env *cel.Env, expression string, activation map[string]any) (bool, error) {
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return false, issues.Err()
	}
	if !ast.OutputType().IsExactType(types.BoolType) {
		return false, fmt.Errorf("expression must evaluate to bool")
	}
	prog, err := env.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := prog.ContextEval(ctx, activation)
	if err != nil {
		return false, err
	}
	return out == ref.Val(types.True), nil
}
