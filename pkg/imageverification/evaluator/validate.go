package eval

import (
	"encoding/json"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"gomodules.xyz/jsonpatch/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
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
	Policy     *policiesv1alpha1.ImageValidatingPolicy
	Exceptions []*policiesv1alpha1.PolicyException
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

func Validate(ivpol *policiesv1alpha1.ImageValidatingPolicy, lister k8scorev1.SecretInterface) ([]string, error) {
	ictx, er := imagedataloader.NewImageContext(lister)
	if er != nil {
		return nil, nil
	}

	compiler := NewCompiler(ictx, lister, nil)
	_, err := compiler.Compile(ivpol, nil)
	if err == nil {
		return nil, nil
	}

	warnings := make([]string, 0, len(err.ToAggregate().Errors()))
	for _, e := range err.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}

	return warnings, err.ToAggregate()
}
