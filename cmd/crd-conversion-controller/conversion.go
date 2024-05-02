package main

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// DataAnnotation is the annotation that conversion webhooks
	// use to retain the data in case of down-conversion from the preferred version.
	DataAnnotation = "kyverno.io/conversion-data"
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
	if err := convertPolicy(logger, convertedObject, fromVersion, toVersion); err != nil {
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

func convertPolicy(logger logr.Logger, convertedObject *unstructured.Unstructured, fromVersion, toVersion string) error {
	if (fromVersion == "kyverno.io/v1" && toVersion == "kyverno.io/v2beta1") ||
		(fromVersion == "kyverno.io/v2beta1" && toVersion == "kyverno.io/v1") {
		return nil
	}

	// extract the spec field
	spec, _, err := unstructured.NestedMap(convertedObject.Object, "spec")
	if err != nil {
		return fmt.Errorf("failed to extract spec: %v", err)
	}

	if toVersion == "kyverno.io/v2" {
		// store the spec field in the annotations of the converted policy
		annotations := convertedObject.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		data, err := json.Marshal(spec)
		if err != nil {
			return fmt.Errorf("failed to marshal the spec field: %v", err)
		}
		annotations[DataAnnotation] = string(data)
		convertedObject.SetAnnotations(annotations)
	} else {
		// set the spec field from the annotation data if exist
		data, found := convertedObject.GetAnnotations()[DataAnnotation]
		if found {
			var spec map[string]interface{}
			if err := json.Unmarshal([]byte(data), &spec); err != nil {
				return fmt.Errorf("failed to unmarshal the annotation %v: %v", DataAnnotation, err)
			}

			if err := unstructured.SetNestedMap(convertedObject.Object, spec, "spec"); err != nil {
				return fmt.Errorf("failed to set spec: %v", err)
			}
			// remove the annotation data from the converted object
			delete(convertedObject.GetAnnotations(), DataAnnotation)
			return nil
		}
	}

	rules, _ := spec["rules"].([]interface{})
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}

		convertRule(ruleMap, "validate", func(validateMap map[string]interface{}) {
			convertValidateRule(logger, spec, ruleMap, validateMap, toVersion)
		})

		convertRule(ruleMap, "mutate", func(mutateMap map[string]interface{}) {
			convertMutateRule(logger, spec, ruleMap, mutateMap, toVersion)
		})

		convertRule(ruleMap, "generate", func(generateMap map[string]interface{}) {
			convertGenerateRule(logger, spec, ruleMap, generateMap, toVersion)
		})

		// extract the match block if exists
		match, ok := ruleMap["match"].(map[string]interface{})
		if ok {
			resources, found := match["resources"].(map[string]interface{})
			if found {
				unstructured.SetNestedField(match, []interface{}{map[string]interface{}{"resources": resources}}, "any")
				unstructured.RemoveNestedField(match, "resources")
			}

			roles, found := match["roles"].([]interface{})
			if found {
				unstructured.SetNestedField(match, []interface{}{map[string]interface{}{"roles": roles}}, "any")
				unstructured.RemoveNestedField(match, "roles")
			}

			clusterRoles, found := match["clusterRoles"].([]interface{})
			if found {
				unstructured.SetNestedField(match, []interface{}{map[string]interface{}{"clusterRoles": clusterRoles}}, "any")
				unstructured.RemoveNestedField(match, "clusterRoles")
			}

			subjects, found := match["subjects"].([]interface{})
			if found {
				unstructured.SetNestedField(match, []interface{}{map[string]interface{}{"subjects": subjects}}, "any")
				unstructured.RemoveNestedField(match, "subjects")
			}

			unstructured.SetNestedMap(ruleMap, match, "match")
		}

		// extract the exclude block if exists
		exclude, ok := ruleMap["exclude"].(map[string]interface{})
		if ok {
			resources, found := exclude["resources"].(map[string]interface{})
			if found {
				unstructured.SetNestedField(exclude, []interface{}{map[string]interface{}{"resources": resources}}, "any")
				unstructured.RemoveNestedField(exclude, "resources")
			}

			roles, found := exclude["roles"].([]interface{})
			if found {
				unstructured.SetNestedField(exclude, []interface{}{map[string]interface{}{"roles": roles}}, "any")
				unstructured.RemoveNestedField(exclude, "roles")
			}

			clusterRoles, found := exclude["clusterRoles"].([]interface{})
			if found {
				unstructured.SetNestedField(exclude, []interface{}{map[string]interface{}{"clusterRoles": clusterRoles}}, "any")
				unstructured.RemoveNestedField(exclude, "clusterRoles")
			}

			subjects, found := exclude["subjects"].([]interface{})
			if found {
				unstructured.SetNestedField(exclude, []interface{}{map[string]interface{}{"subjects": subjects}}, "any")
				unstructured.RemoveNestedField(exclude, "subjects")
			}

			unstructured.SetNestedMap(ruleMap, exclude, "exclude")
		}
	}

	spec["rules"] = rules
	// TODO: remove the deprecated fields
	delete(spec, "schemaValidation")
	if err := unstructured.SetNestedMap(convertedObject.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %v", err)
	}

	return nil
}

func convertRule(ruleMap map[string]interface{}, ruleType string, converter func(map[string]interface{})) {
	field, ok := ruleMap[ruleType]
	if !ok {
		return
	}

	fieldMap, ok := field.(map[string]interface{})
	if !ok {
		return
	}

	converter(fieldMap)
}

func convertValidateRule(logger logr.Logger, specMap, ruleMap, validateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the validationFailureAction field from the spec field
		validationFailureAction, ok, _ := unstructured.NestedFieldCopy(specMap, "validationFailureAction")
		if ok {
			validateMap["validationFailureAction"] = validationFailureAction
			err := unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureAction", "version", toVersion)
			}
			unstructured.RemoveNestedField(specMap, "validationFailureAction")
		}

		// Extract the validationFailureActionOverrides field from the spec field if exists
		validationFailureActionOverrides, ok, _ := unstructured.NestedFieldCopy(specMap, "validationFailureActionOverrides")
		if ok {
			validateMap["validationFailureActionOverrides"] = validationFailureActionOverrides
			err := unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
			unstructured.RemoveNestedField(specMap, "validationFailureActionOverrides")
		}
	} else {
		err := unstructured.SetNestedField(specMap, validateMap["validationFailureAction"], "validationFailureAction")
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
			err := unstructured.SetNestedField(specMap, value, "validationFailureActionOverrides")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
			delete(validateMap, "validationFailureActionOverrides")
			err = unstructured.SetNestedMap(ruleMap, validateMap, "validate")
			if err != nil {
				logger.Error(err, "failed to set validationFailureActionOverrides", "version", toVersion)
			}
		}
	}
}

func convertMutateRule(logger logr.Logger, specMap, ruleMap, mutateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the mutateExistingOnPolicyUpdate field from the spec field
		mutateExistingOnPolicyUpdate, ok, _ := unstructured.NestedFieldCopy(specMap, "mutateExistingOnPolicyUpdate")
		if ok {
			mutateMap["mutateExistingOnPolicyUpdate"] = mutateExistingOnPolicyUpdate
			err := unstructured.SetNestedMap(ruleMap, mutateMap, "mutate")
			if err != nil {
				logger.Error(err, "failed to set mutateExistingOnPolicyUpdate", "version", toVersion)
			}
			unstructured.RemoveNestedField(specMap, "mutateExistingOnPolicyUpdate")
		}
	} else {
		err := unstructured.SetNestedField(specMap, mutateMap["mutateExistingOnPolicyUpdate"], "mutateExistingOnPolicyUpdate")
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

func convertGenerateRule(logger logr.Logger, specMap, ruleMap, generateMap map[string]interface{}, toVersion string) {
	if toVersion == "kyverno.io/v2" {
		// Extract the generateExisting field from the spec field
		generateExisting, ok, _ := unstructured.NestedFieldCopy(specMap, "generateExisting")
		if ok {
			generateMap["generateExisting"] = generateExisting
			err := unstructured.SetNestedMap(ruleMap, generateMap, "generate")
			if err != nil {
				logger.Error(err, "failed to set generateExisting", "version", toVersion)
			}
			unstructured.RemoveNestedField(specMap, "generateExisting")
		} else {
			// Extract the generateExistingOnPolicyUpdate field from the spec field
			generateExistingOnPolicyUpdate, ok, _ := unstructured.NestedFieldCopy(specMap, "generateExistingOnPolicyUpdate")
			if ok {
				generateMap["generateExisting"] = generateExistingOnPolicyUpdate
				err := unstructured.SetNestedMap(ruleMap, generateMap, "generate")
				if err != nil {
					logger.Error(err, "failed to set generateExisting", "version", toVersion)
				}
				unstructured.RemoveNestedField(specMap, "generateExistingOnPolicyUpdate")
			}
		}
	} else {
		err := unstructured.SetNestedField(specMap, generateMap["generateExisting"], "generateExisting")
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
