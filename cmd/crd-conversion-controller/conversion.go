package main

import (
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Convert perform the actual conversion of the CRDs to and from the stored version
// failures will be reported as Reason in the conversion response.
func Convert(logger logr.Logger, request *v1.ConversionRequest) *v1.ConversionResponse {
	var convertedObjects []runtime.RawExtension
	var conversionStatus metav1.Status

	for _, obj := range request.Objects {
		convertedObject, status := convertObject(logger, &obj, request.DesiredAPIVersion)
		if status.Status != metav1.StatusSuccess {
			return &v1.ConversionResponse{
				Result: status,
			}
		}
		convertedObjects = append(convertedObjects, runtime.RawExtension{Object: convertedObject})
	}

	conversionStatus.Status = metav1.StatusSuccess
	return &v1.ConversionResponse{
		ConvertedObjects: convertedObjects,
		Result:           conversionStatus,
	}
}

func convertObject(logger logr.Logger, obj *runtime.RawExtension, toVersion string) (*unstructured.Unstructured, metav1.Status) {
	var conversionStatus metav1.Status
	cr := unstructured.Unstructured{}

	if err := cr.UnmarshalJSON(obj.Raw); err != nil {
		conversionStatus = metav1.Status{
			Message: fmt.Sprintf("failed to unmarshal object (%v) with error: %v", string(obj.Raw), err),
			Status:  metav1.StatusFailure,
		}
		return nil, conversionStatus
	}

	fromVersion := cr.GetAPIVersion()
	if toVersion == fromVersion {
		conversionStatus = metav1.Status{
			Message: fmt.Sprintf("conversion from a version to itself should not call the webhook: %s", toVersion),
			Status:  metav1.StatusFailure,
		}
		return nil, conversionStatus
	}

	logger.Info("converting policy", "name", cr.GetName(), "kind", cr.GetKind(), "from", fromVersion, "to", toVersion)
	convertedObject := cr.DeepCopy()
	if err := convertPolicy(logger, convertedObject, toVersion); err != nil {
		conversionStatus = metav1.Status{
			Message: fmt.Sprintf("failed to convert policy (%v) with error: %v", cr.GetName(), err),
			Status:  metav1.StatusFailure,
		}
		return nil, conversionStatus
	}

	conversionStatus = metav1.Status{
		Status: metav1.StatusSuccess,
	}
	convertedObject.SetAPIVersion(toVersion)
	return convertedObject, conversionStatus
}

func convertPolicy(logger logr.Logger, convertedObject *unstructured.Unstructured, toVersion string) error {
	// Extract the rules field
	rules, ok, err := unstructured.NestedSlice(convertedObject.Object, "spec", "rules")
	if err != nil {
		return fmt.Errorf("failed to extract rules: %v", err)
	}
	if !ok {
		return nil // No rules found
	}

	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}

		if err := convertRule(ruleMap, "validate", func(validateMap map[string]interface{}) {
			convertValidateRule(logger, convertedObject, ruleMap, validateMap, toVersion)
		}); err != nil {
			return err
		}

		if err := convertRule(ruleMap, "mutate", func(mutateMap map[string]interface{}) {
			convertMutateRule(logger, convertedObject, ruleMap, mutateMap, toVersion)
		}); err != nil {
			return err
		}

		if err := convertRule(ruleMap, "generate", func(generateMap map[string]interface{}) {
			convertGenerateRule(logger, convertedObject, ruleMap, generateMap, toVersion)
		}); err != nil {
			return err
		}
	}

	if err := unstructured.SetNestedSlice(convertedObject.Object, rules, "spec", "rules"); err != nil {
		return fmt.Errorf("failed to set rules: %v", err)
	}

	return nil
}

func convertRule(ruleMap map[string]interface{}, ruleType string, converter func(map[string]interface{})) error {
	field, ok := ruleMap[ruleType]
	if !ok {
		return nil // Rule type not found
	}

	fieldMap, ok := field.(map[string]interface{})
	if !ok {
		return nil
	}

	converter(fieldMap)

	return nil
}

func convertValidateRule(logger logr.Logger, convertedObject *unstructured.Unstructured, ruleMap map[string]interface{}, validateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the validationFailureAction field from the spec field
		validationFailureAction, ok, _ := unstructured.NestedFieldCopy(convertedObject.Object, "spec", "validationFailureAction")
		if ok {
			validateMap["validationFailureAction"] = validationFailureAction
			err := unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureAction", "version", toVersion)
			}
		}

		// Extract the validationFailureActionOverrides field from the spec field if exists
		validationFailureActionOverrides, ok, _ := unstructured.NestedFieldCopy(convertedObject.Object, "spec", "validationFailureActionOverrides")
		if ok {
			validateMap["validationFailureActionOverrides"] = validationFailureActionOverrides
			err := unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
		}
	} else {
		err := unstructured.SetNestedField(convertedObject.Object, validateMap["validationFailureAction"], "spec", "validationFailureAction")
		if err != nil {
			logger.Error(err, "failed to set validationFailureAction", "version", toVersion)
		}
		delete(validateMap, "validationFailureAction")
		err = unstructured.SetNestedMap(ruleMap, validateMap, "validate")
		if err != nil {
			logger.Error(err, "failed to set validationFailureAction", "version", toVersion)
		}

		// Extract the validationFailureActionOverrides field
		value, ok := validateMap["validationFailureActionOverrides"]
		if ok {
			err := unstructured.SetNestedField(convertedObject.Object, value, "spec", "validationFailureActionOverrides")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
			delete(validateMap, "validationFailureAction")
			err = unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
		}
	}
}

func convertMutateRule(logger logr.Logger, convertedObject *unstructured.Unstructured, ruleMap map[string]interface{}, mutateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the mutateExistingOnPolicyUpdate field from the spec field
		mutateExistingOnPolicyUpdate, ok, _ := unstructured.NestedFieldCopy(convertedObject.Object, "spec", "mutateExistingOnPolicyUpdate")
		if ok {
			mutateMap["mutateExistingOnPolicyUpdate"] = mutateExistingOnPolicyUpdate
			err := unstructured.SetNestedMap(ruleMap, mutateMap, "mutate")
			if err != nil {
				logger.Error(err, "failed to set mutateExistingOnPolicyUpdate", "version", toVersion)
			}
		}
	} else {
		err := unstructured.SetNestedField(convertedObject.Object, mutateMap["mutateExistingOnPolicyUpdate"], "spec", "mutateExistingOnPolicyUpdate")
		if err != nil {
			logger.Error(err, "failed to set mutateExistingOnPolicyUpdate", "version", toVersion)
		}
		delete(mutateMap, "mutateExistingOnPolicyUpdate")
		err = unstructured.SetNestedMap(ruleMap, mutateMap, "mutate")
		if err != nil {
			logger.Error(err, "failed to set mutateExistingOnPolicyUpdate", "version", toVersion)
		}
	}
}

func convertGenerateRule(logger logr.Logger, convertedObject *unstructured.Unstructured, ruleMap map[string]interface{}, generateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the generateExisting field from the spec field
		generateExisting, ok, _ := unstructured.NestedFieldCopy(convertedObject.Object, "spec", "generateExisting")
		if ok {
			generateMap["generateExisting"] = generateExisting
			err := unstructured.SetNestedMap(ruleMap, generateMap, "generate")
			if err != nil {
				logger.Error(err, "failed to set generateExisting", "version", toVersion)
			}
		}
	} else {
		err := unstructured.SetNestedField(convertedObject.Object, generateMap["generateExisting"], "spec", "generateExisting")
		if err != nil {
			logger.Error(err, "failed to set generateExisting", "version", toVersion)
		}
		delete(generateMap, "generateExisting")
		err = unstructured.SetNestedMap(ruleMap, generateMap, "generate")
		if err != nil {
			logger.Error(err, "failed to set generateExisting", "version", toVersion)
		}
	}
}
