package generation

import (
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/engine"
	utils "github.com/kyverno/kyverno/pkg/engine/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

// stripNonPolicyFields - remove feilds which get updated with each request by kyverno and are non policy fields
func stripNonPolicyFields(obj, newRes map[string]interface{}, logger logr.Logger) (map[string]interface{}, map[string]interface{}) {
	if metadata, found := obj["metadata"]; found {
		requiredMetadataInObj := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			delete(annotations.(map[string]interface{}), "kubectl.kubernetes.io/last-applied-configuration")
			requiredMetadataInObj["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			delete(labels.(map[string]interface{}), generate.LabelClonePolicyName)
			requiredMetadataInObj["labels"] = labels
		}
		obj["metadata"] = requiredMetadataInObj
	}

	if metadata, found := newRes["metadata"]; found {
		requiredMetadataInNewRes := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			requiredMetadataInNewRes["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			requiredMetadataInNewRes["labels"] = labels
		}
		newRes["metadata"] = requiredMetadataInNewRes
	}

	delete(obj, "status")

	if _, found := obj["spec"]; found {
		delete(obj["spec"].(map[string]interface{}), "tolerations")
	}

	if dataMap, found := obj["data"]; found {
		keyInData := make([]string, 0)
		switch dataMap := dataMap.(type) {
		case map[string]interface{}:
			for k := range dataMap {
				keyInData = append(keyInData, k)
			}
		}

		if len(keyInData) > 0 {
			for _, dataKey := range keyInData {
				originalResourceData := dataMap.(map[string]interface{})[dataKey]
				replaceData := strings.Replace(originalResourceData.(string), "\n", "", -1)
				dataMap.(map[string]interface{})[dataKey] = replaceData

				newResourceData := newRes["data"].(map[string]interface{})[dataKey]
				replacenewResourceData := strings.Replace(newResourceData.(string), "\n", "", -1)
				newRes["data"].(map[string]interface{})[dataKey] = replacenewResourceData
			}
		} else {
			logger.V(4).Info("data is not of type map[string]interface{}")
		}
	}

	return obj, newRes
}

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

	switch typedConditions := conditions.(type) {
	case kyvernov1.AnyAllConditions:
		conditionsAll := append(typedConditions.AllConditions, typedConditions.AnyConditions...)
		for _, condition := range conditionsAll {
			if reflect.DeepEqual(condition, expected) {
				return true
			}
		}
	case []kyvernov1.Condition:
		return reflect.DeepEqual(typedConditions, expected)
	}
	return false
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
