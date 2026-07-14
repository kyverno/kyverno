package evaluator

import (
	"encoding/json"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"gomodules.xyz/jsonpatch/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ImageVerificationOutcome struct {
	// Name is the rule name specified in policy
	Name string `json:"name,omitempty"`
	// RuleType is the rule type (Mutation,Generation,Validation) for Kyverno Policy
	RuleType engineapi.RuleType `json:"ruleType,omitempty"`
	// Message is the message response from the rule application
	Message string `json:"message,omitempty"`
	// Status rule status
	Status engineapi.RuleStatus `json:"status,omitempty"`
	// EmitWarning enable passing rule message as warning to api server warning header
	EmitWarning bool `json:"emitWarning,omitempty"`
	// Properties are the additional properties from the rule that will be added to the policy report result
	Properties map[string]string `json:"properties,omitempty"`
}

type ImageVerifyEngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []ImageVerifyPolicyResponse
}

type ImageVerifyPolicyResponse struct {
	Policy     policiesv1beta1.ImageValidatingPolicyLike
	Exceptions []*policiesv1beta1.PolicyException
	Actions    sets.Set[admissionregistrationv1.ValidationAction]
	Result     engineapi.RuleResponse
}

func outcomeFromPolicyResponse(responses map[string]ImageVerifyPolicyResponse) map[string]ImageVerificationOutcome {
	outcomes := make(map[string]ImageVerificationOutcome)
	for pol, resp := range responses {
		outcomes[pol] = ImageVerificationOutcome{
			Name:        resp.Result.Name(),
			RuleType:    resp.Result.RuleType(),
			Message:     resp.Result.Message(),
			Status:      resp.Result.Status(),
			EmitWarning: resp.Result.EmitWarning(),
			Properties:  resp.Result.Properties(),
		}
	}
	return outcomes
}

func MakeImageVerifyOutcomePatch(hasAnnotations bool, responses map[string]ImageVerifyPolicyResponse) ([]jsonpatch.JsonPatchOperation, error) {
	patches := make([]jsonpatch.JsonPatchOperation, 0)
	annotationKey := "/metadata/annotations/" + strings.ReplaceAll(kyverno.AnnotationImageVerifyOutcomes, "/", "~1")
	if !hasAnnotations {
		patch := jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/metadata/annotations",
			Value:     map[string]string{},
		}
		logger.V(4).Info("adding annotation patch", "patch", patch)
		patches = append(patches, patch)
	}

	outcomes := outcomeFromPolicyResponse(responses)
	data, err := json.Marshal(outcomes)
	if err != nil {
		return nil, err
	}

	patch := jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      annotationKey,
		Value:     string(data),
	}

	logger.V(4).Info("adding image verification patch", "patch", patch)
	patches = append(patches, patch)
	return patches, nil
}

func Validate(ivpol policiesv1beta1.ImageValidatingPolicyLike, lister k8scorev1.SecretInterface) ([]string, error) {
	ictx, er := imagedataloader.NewImageContext(lister)
	if er != nil {
		return nil, nil
	}

	compiler := NewCompiler(ictx, lister, nil)
	_, errList := compiler.Compile(ivpol, nil)

	errs := make(field.ErrorList, 0, len(errList))
	if len(errList) > 0 {
		errs = errList
	}

	if ivpol.GetNamespace() != "" && !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		if engine.ExpressionsUseHTTP(ivpolExpressions(ivpol)...) {
			errs = append(errs, field.Forbidden(field.NewPath("spec"), "http.* is not allowed in namespaced policies; set --allowHTTPInNamespacedPolicies to enable"))
		}
	}

	if len(errs) == 0 {
		return nil, nil
	}

	warnings := make([]string, 0, len(errs.ToAggregate().Errors()))
	for _, e := range errs.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}

	return warnings, errs.ToAggregate()
}

func ivpolExpressions(ivpol policiesv1beta1.ImageValidatingPolicyLike) []string {
	spec := ivpol.GetSpec()
	if spec == nil {
		return nil
	}
	exprs := make([]string, 0, len(spec.Variables)+len(spec.MatchConditions)+len(spec.Validations)*2+len(spec.AuditAnnotations)+len(spec.ImageExtractors)+len(spec.MatchImageReferences))
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	for _, mc := range spec.MatchConditions {
		exprs = append(exprs, mc.Expression)
	}
	for _, val := range spec.Validations {
		exprs = append(exprs, val.Expression, val.MessageExpression)
	}
	for _, aa := range spec.AuditAnnotations {
		exprs = append(exprs, aa.ValueExpression)
	}
	for _, ie := range spec.ImageExtractors {
		exprs = append(exprs, ie.Expression)
	}
	for _, mir := range spec.MatchImageReferences {
		if mir.Expression != "" {
			exprs = append(exprs, mir.Expression)
		}
	}
	return exprs
}
