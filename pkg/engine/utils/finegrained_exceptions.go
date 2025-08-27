package utils

import (
	"strings"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/ext/wildcard"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MatchesFinegrainedException checks if the resource matches any fine-grained exception criteria
// for the given policy rule. It returns true if the resource should be exempted based on
// image-based or value-based exception criteria.
func MatchesFinegrainedException(
	polexs []*kyvernov2.PolicyException,
	policyContext engineapi.PolicyContext,
	policyName, ruleName string,
	logger logr.Logger,
) ([]kyvernov2.PolicyException, kyvernov2.ExceptionReportMode) {
	var matchedExceptions []kyvernov2.PolicyException
	defaultReportMode := kyvernov2.ExceptionReportSkip

	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}

	for _, polex := range polexs {
		// First check if this exception applies to the policy/rule
		if !polex.Spec.Contains(policyName, ruleName) {
			continue
		}

		// Check each exception in the PolicyException
		for _, exception := range polex.Spec.Exceptions {
			if !exception.Contains(policyName, ruleName) {
				continue
			}

			// Check if this exception has fine-grained criteria
			if !exception.IsFinegrained() {
				continue
			}

			// Check image-based exceptions
			if exception.HasImageExceptions() && matchesImageException(&exception, policyContext, logger) {
				matchedExceptions = append(matchedExceptions, *polex)
				if exception.GetReportMode() != kyvernov2.ExceptionReportSkip {
					defaultReportMode = exception.GetReportMode()
				}
				continue
			}

			// Check value-based exceptions
			if exception.HasValueExceptions() && matchesValueException(&exception, resource, logger) {
				matchedExceptions = append(matchedExceptions, *polex)
				if exception.GetReportMode() != kyvernov2.ExceptionReportSkip {
					defaultReportMode = exception.GetReportMode()
				}
				continue
			}
		}
	}

	return matchedExceptions, defaultReportMode
}

// matchesImageException checks if any images in the resource match the image exception criteria
func matchesImageException(
	exception *kyvernov2.Exception,
	policyContext engineapi.PolicyContext,
	logger logr.Logger,
) bool {
	images := policyContext.JSONContext().ImageInfo()

	for _, imageException := range exception.Images {
		for _, infoMap := range images {
			for _, imageInfo := range infoMap {
				image := imageInfo.String()
				for _, imageRef := range imageException.ImageReferences {
					if wildcard.Match(imageRef, image) {
						logger.V(4).Info("image matches exception", "image", image, "pattern", imageRef)
						return true
					}
				}
			}
		}
	}

	return false
}

// matchesValueException checks if any values in the resource match the value exception criteria
func matchesValueException(
	exception *kyvernov2.Exception,
	resource unstructured.Unstructured,
	logger logr.Logger,
) bool {
	for _, valueException := range exception.Values {
		if matchesValueAtPath(&valueException, resource, logger) {
			return true
		}
	}
	return false
}

// matchesValueAtPath checks if the value at the specified path matches the exception criteria
func matchesValueAtPath(
	valueException *kyvernov2.ValueException,
	resource unstructured.Unstructured,
	logger logr.Logger,
) bool {
	// Extract values using JSONPath-like expression
	values, err := extractValuesFromPath(valueException.Path, resource)
	if err != nil {
		logger.V(4).Info("failed to extract values from path", "path", valueException.Path, "error", err)
		return false
	}

	// Get the operator (default to equals)
	operator := kyvernov2.ValueOperatorEquals
	if valueException.Operator != nil {
		operator = *valueException.Operator
	}

	// Check if any extracted value matches the exception criteria
	for _, extractedValue := range values {
		if matchesValue(extractedValue, valueException.Values, operator) {
			logger.V(4).Info("value matches exception", "value", extractedValue, "path", valueException.Path, "operator", operator)
			return true
		}
	}

	return false
}

// extractValuesFromPath extracts values from the resource using a simplified JSONPath-like syntax
func extractValuesFromPath(path string, resource unstructured.Unstructured) ([]string, error) {
	var values []string

	// Handle simple paths like "metadata.labels.environment"
	if !strings.Contains(path, "[") {
		value, found, err := unstructured.NestedString(resource.Object, strings.Split(path, ".")...)
		if err != nil {
			return nil, err
		}
		if found {
			values = append(values, value)
		}
		return values, nil
	}

	// For more complex paths with arrays, we'll implement a basic parser
	// This is a simplified implementation - a full JSONPath implementation would be more robust
	return extractComplexPath(path, resource.Object)
}

// extractComplexPath handles complex paths with array notation
func extractComplexPath(path string, obj interface{}) ([]string, error) {
	var values []string

	// This is a simplified implementation for paths like "spec.containers[*].image"
	// In a production implementation, you'd want a more robust JSONPath parser

	if strings.Contains(path, "spec.containers[*].image") {
		if objMap, ok := obj.(map[string]interface{}); ok {
			if spec, found := objMap["spec"]; found {
				if specMap, ok := spec.(map[string]interface{}); ok {
					if containers, found := specMap["containers"]; found {
						if containersList, ok := containers.([]interface{}); ok {
							for _, container := range containersList {
								if containerMap, ok := container.(map[string]interface{}); ok {
									if image, found := containerMap["image"]; found {
										if imageStr, ok := image.(string); ok {
											values = append(values, imageStr)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Add more complex path handling as needed
	return values, nil
}

// matchesValue checks if a value matches the exception criteria using the specified operator
func matchesValue(value string, exemptedValues []string, operator kyvernov2.ValueOperator) bool {
	switch operator {
	case kyvernov2.ValueOperatorEquals, "": // Default to equals
		for _, exemptedValue := range exemptedValues {
			if value == exemptedValue {
				return true
			}
		}
	case kyvernov2.ValueOperatorIn:
		for _, exemptedValue := range exemptedValues {
			if value == exemptedValue {
				return true
			}
		}
	case kyvernov2.ValueOperatorStartsWith:
		for _, exemptedValue := range exemptedValues {
			if strings.HasPrefix(value, exemptedValue) {
				return true
			}
		}
	case kyvernov2.ValueOperatorEndsWith:
		for _, exemptedValue := range exemptedValues {
			if strings.HasSuffix(value, exemptedValue) {
				return true
			}
		}
	case kyvernov2.ValueOperatorContains:
		for _, exemptedValue := range exemptedValues {
			if strings.Contains(value, exemptedValue) {
				return true
			}
		}
	}
	return false
}
