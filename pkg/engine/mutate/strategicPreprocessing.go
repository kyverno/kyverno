package mutate

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	anchor "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type ConditionError struct {
	errorChain error
}

func (ce ConditionError) Error() string {
	return fmt.Sprintf("Condition failed: %s", ce.errorChain.Error())
}

func NewConditionError(err error) error {
	return ConditionError{err}
}

// preProcessPattern - Dynamically preProcess the yaml
// 1> For conditional anchor remove anchors from the pattern.
// 2> For Adding anchors remove anchor tags.

// The whole yaml is structured as a pointer tree.
// https://godoc.org/gopkg.in/yaml.v3#Node
// A single Node contains Tag to identify it as MappingNode (map[string]interface{}), Sequence ([]interface{}), ScalarNode (string, int, float bool etc.)
// A parent node having MappingNode keeps the data as <keyNode>, <ValueNode> inside it's Content field and Tag field as "!!map".
// A parent node having Sequence keeps the data as array of Node inside Content field and a Tag field as "!!seq".
// https://github.com/kubernetes-sigs/kustomize/blob/master/kyaml/yaml/rnode.go
func preProcessPattern(logger logr.Logger, pattern, resource *yaml.RNode) error {
	err := preProcessRecursive(logger, pattern, resource)
	if err != nil {
		return err
	}
	return deleteConditionElements(pattern)
}

func preProcessRecursive(logger logr.Logger, pattern, resource *yaml.RNode) error {
	switch pattern.YNode().Kind {
	case yaml.MappingNode:
		return walkMap(logger, pattern, resource)
	case yaml.SequenceNode:
		return walkArray(logger, pattern, resource)
	}

	return nil
}

// walkMap - walk through the MappingNode
func walkMap(logger logr.Logger, pattern, resource *yaml.RNode) error {
	var err error

	err = validateConditions(logger, pattern, resource)
	if err != nil {
		return err
	}

	err = handleAddings(logger, pattern, resource)
	if err != nil {
		return err
	}

	fields, err := pattern.Fields()
	if err != nil {
		return err
	}

	for _, field := range fields {
		var resourceNode *yaml.RNode

		if resource == nil || resource.Field(field) == nil {
			// In case if we have pattern, but not corresponding resource part,
			// just walk down and remove all anchors. nil here indicates that
			// resourceNode is empty
			resourceNode = nil
		} else {
			resourceNode = resource.Field(field).Value
		}

		err := preProcessRecursive(logger, pattern.Field(field).Value, resourceNode)
		if err != nil {
			return err
		}
	}

	return nil
}

// walkArray - walk through array elements
func walkArray(logger logr.Logger, pattern, resource *yaml.RNode) error {
	elements, err := pattern.Elements()
	if err != nil {
		return err
	}

	if len(elements) == 0 {
		return nil
	}

	if elements[0].YNode().Kind == yaml.MappingNode {
		return processListOfMaps(logger, pattern, resource)
	}

	return nil
}

// processListOfMaps - process arrays
// in many cases like containers, volumes kustomize uses name field to match resource for processing
// If any conditional anchor match resource field and if the pattern doesn't contain "name" field and
// resource contains "name" field, then copy the name field from resource to pattern.
func processListOfMaps(logger logr.Logger, pattern, resource *yaml.RNode) error {
	patternElements, err := pattern.Elements()
	if err != nil {
		return err
	}

	resourceElements, err := resource.Elements()
	if err != nil {
		return err
	}

	for _, patternElement := range patternElements {
		// If pattern has conditions, look for matching elements and process them
		if hasAnchors(patternElement) {
			for _, resourceElement := range resourceElements {
				err := preProcessRecursive(logger, patternElement, resourceElement)
				if err != nil {
					if _, ok := err.(ConditionError); ok {
						// Skip element, if condition has failed
						continue
					}

					return err
				} else {
					// If condition is satisfied, create new pattern list element based on patternElement
					// but related with current resource element by name.
					// Resource element must have name. Without name kustomize won't be able to update this element.
					// In case if element does not have name, skip it.
					resourceElementName := resourceElement.Field("name")
					if resourceElementName.IsNilOrEmpty() {
						continue
					}

					newNode := patternElement.Copy()
					err := deleteConditionsFromNestedMaps(newNode)
					if err != nil {
						return err
					}

					err = newNode.PipeE(yaml.SetField("name", resourceElementName.Value))
					if err != nil {
						return err
					}

					err = pattern.PipeE(yaml.Append(newNode.YNode()))
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateConditions checks all conditions from current map.
// If at least one condition fails, return error.
// If caller handles list of maps and gets an error, it must skip element.
// If caller handles map, it must stop processing and skip entire rule.
func validateConditions(logger logr.Logger, pattern, resource *yaml.RNode) error {
	conditions, err := filterKeys(pattern, anchor.IsConditionAnchor)
	if err != nil {
		return err
	}

	for _, condition := range conditions {
		conditionKey := removeAnchor(condition)
		if resource == nil || resource.Field(conditionKey) == nil {
			continue
		}

		err = checkCondition(logger, pattern.Field(condition).Value, resource.Field(conditionKey).Value)
		if err != nil {
			return err
		}
	}

	return nil
}

// handleAddings handles adding anchors.
// Remove anchor from pattern, if field already exists.
// Remove anchor wrapping from key, if field does not exist in the resource.
func handleAddings(logger logr.Logger, pattern, resource *yaml.RNode) error {
	addings, err := filterKeys(pattern, anchor.IsAddingAnchor)
	if err != nil {
		return err
	}

	for _, adding := range addings {
		key, _ := anchor.RemoveAnchor(adding)
		if resource != nil && resource.Field(key) != nil {
			// Resource already has this field.
			// Delete the field with adding anchor from patch.
			err = pattern.PipeE(yaml.Clear(adding))
			if err != nil {
				return err
			}
			continue
		}

		// Remove anchor wrap from patch field.
		renameField(adding, key, pattern)
	}

	return nil
}

func filterKeys(pattern *yaml.RNode, condition func(string) bool) ([]string, error) {
	keys := make([]string, 0)
	fields, err := pattern.Fields()
	if err != nil {
		return keys, err
	}

	for _, key := range fields {
		if condition(key) {
			keys = append(keys, key)
			continue
		}
	}
	return keys, nil
}

func hasAnchors(pattern *yaml.RNode) bool {
	if yaml.MappingNode == pattern.YNode().Kind {
		fields, err := pattern.Fields()
		if err != nil {
			return false
		}

		for _, key := range fields {
			if anchor.IsConditionAnchor(key) || anchor.IsAddingAnchor(key) {
				return true
			}

			patternNode := pattern.Field(key)
			if !patternNode.IsNilOrEmpty() {
				if hasAnchors(patternNode.Value) {
					return true
				}
			}
		}
	}

	return false
}

func renameField(name, newName string, pattern *yaml.RNode) {
	field := pattern.Field(name)
	if field == nil {
		return
	}

	field.Key.YNode().Value = newName
}

func convertRNodeToInterface(document *yaml.RNode) (interface{}, error) {
	if document.YNode().Kind == yaml.ScalarNode {
		return document.YNode().Value, nil
	}

	rawDocument, err := document.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var documentInterface interface{}

	err = json.Unmarshal(rawDocument, &documentInterface)
	if err != nil {
		return nil, err
	}

	return documentInterface, nil
}

func checkCondition(logger logr.Logger, pattern *yaml.RNode, resource *yaml.RNode) error {
	patternInterface, err := convertRNodeToInterface(pattern)
	if err != nil {
		return err
	}

	resourceInterface, err := convertRNodeToInterface(resource)
	if err != nil {
		return err
	}

	_, err = validate.ValidateResourceWithPattern(logger, resourceInterface, patternInterface)
	if err != nil {
		return NewConditionError(err)
	}

	return nil
}

func deleteConditionsFromNestedMaps(pattern *yaml.RNode) error {
	if pattern.YNode().Kind != yaml.MappingNode {
		return nil
	}

	fields, err := pattern.Fields()
	if err != nil {
		return err
	}

	for _, field := range fields {
		if anchor.IsConditionAnchor(field) {
			err = pattern.PipeE(yaml.Clear(field))
			if err != nil {
				return err
			}
		} else {
			child := pattern.Field(field).Value
			if child != nil {
				err = deleteConditionsFromNestedMaps(child)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func deleteConditionElements(pattern *yaml.RNode) error {
	switch pattern.YNode().Kind {
	case yaml.MappingNode:
		fields, err := pattern.Fields()
		if err != nil {
			return err
		}

		for _, field := range fields {
			if anchor.IsConditionAnchor(field) {
				err = pattern.PipeE(yaml.Clear(field))
				if err != nil {
					return err
				}
			} else {
				child := pattern.Field(field).Value
				if child != nil {
					err = deleteConditionElements(child)
					if err != nil {
						return err
					}
				}
			}
		}
	case yaml.SequenceNode:
		elements, err := pattern.Elements()
		if err != nil {
			return err
		}

		// In this case we have no resource elements that matched the condition.
		// Just create dummy element with empty name so list must not be deleted.
		if len(elements) == 1 {
			element := elements[0]
			if hasAnchors(element) {
				deleteListElement(pattern, 0)
				dummy, err := yaml.Parse(`{ "name": "" }`)
				if err != nil {
					return err
				}

				err = pattern.PipeE(yaml.Append(dummy.YNode()))
				if err != nil {
					return err
				}
			}

			return nil
		}

		for i, element := range elements {
			if hasAnchors(element) {
				deleteListElement(pattern, i)
			} else {
				deleteConditionElements(element)
			}
		}
	}

	return nil
}

func deleteListElement(list *yaml.RNode, i int) {
	content := list.YNode().Content
	list.YNode().Content = append(content[:i], content[i+1:]...)
}
