package generation

import (
	"reflect"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/engine"
	utils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	admissionv1 "k8s.io/api/admission/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func buildURSpec(requestType kyvernov1beta1.RequestType, policyKey, ruleName string, resource kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov1beta1.UpdateRequestSpec {
	return kyvernov1beta1.UpdateRequestSpec{
		Type:             requestType,
		Policy:           policyKey,
		Rule:             ruleName,
		Resource:         resource,
		DeleteDownstream: deleteDownstream,
	}
}

func buildURContext(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext) kyvernov1beta1.UpdateRequestSpecContext {
	return kyvernov1beta1.UpdateRequestSpecContext{
		UserRequestInfo: policyContext.AdmissionInfo(),
		AdmissionRequestInfo: kyvernov1beta1.AdmissionRequestInfoObject{
			AdmissionRequest: request,
			Operation:        request.Operation,
		},
	}
}

func precondition(rule kyvernov1.Rule, expected kyvernov1.Condition) bool {
	conditions, err := utils.TransformConditions(rule.GetAnyAllConditions())
	if err != nil {
		return false
	}

	var conditionsAll []kyvernov1.Condition
	switch typedConditions := conditions.(type) {
	case kyvernov1.AnyAllConditions:
		conditionsAll = append(typedConditions.AllConditions, typedConditions.AnyConditions...)
	case []kyvernov1.Condition:
		conditionsAll = typedConditions
	}
	for _, condition := range conditionsAll {
		copy := condition.DeepCopy()
		copy.RawKey = trimKeySpaces(condition.RawKey)
		if reflect.DeepEqual(*copy, expected) {
			return true
		}
	}
	return false
}

func trimKeySpaces(rawKey *apiextv1.JSON) *apiextv1.JSON {
	keys := regex.RegexVariableKey.FindAllStringSubmatch(string(rawKey.Raw), -1)
	if len(keys) != 0 {
		return kyvernov1.ToJSON(strings.TrimSpace(keys[0][1]))
	}
	return kyvernov1.ToJSON("")
}

func compareLabels(new, old map[string]string) bool {
	if new == nil {
		return true
	}
	if new[common.GeneratePolicyLabel] != old[common.GeneratePolicyLabel] ||
		new[common.GeneratePolicyNamespaceLabel] != old[common.GeneratePolicyNamespaceLabel] ||
		new[common.GenerateRuleLabel] != old[common.GenerateRuleLabel] ||
		new[common.GenerateTriggerNameLabel] != old[common.GenerateTriggerNameLabel] ||
		new[common.GenerateTriggerNSLabel] != old[common.GenerateTriggerNSLabel] ||
		new[common.GenerateTriggerKindLabel] != old[common.GenerateTriggerKindLabel] {
		return false
	}
	return true
}
