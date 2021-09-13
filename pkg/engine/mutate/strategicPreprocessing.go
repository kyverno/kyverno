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
	return fmt.Sprintf("condition failed: %s", ce.errorChain.Error())
}

func NewConditionError(err error) error {
	return ConditionError{err}
}

type GlobalConditionError struct {
	errorChain error
}

func (ce GlobalConditionError) Error() string {
	return fmt.Sprintf("global condition failed: %s", ce.errorChain.Error())
}

func NewGlobalConditionError(err error) error {
	return GlobalConditionError{err}
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
		return walkList(logger, pattern, resource)
	}

	return nil
}

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

	nonAnchors, err := filterKeys(pattern, func(key string) bool {
		return !hasAnchor(key)
	})
	if err != nil {
		return err
	}

	var resourceValue *yaml.RNode

	for _, field := range nonAnchors {
		if resource == nil || resource.Field(field) == nil {
			resourceValue = nil
		} else {
			resourceValue = resource.Field(field).Value
		}

		err := preProcessRecursive(logger, pattern.Field(field).Value, resourceValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func walkList(logger logr.Logger, pattern, resource *yaml.RNode) error {
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
		hasAnyAnchor := hasAnchors(patternElement, hasAnchor)
		hasGlobalConditions := hasAnchors(patternElement, anchor.IsGlobalAnchor)
		if hasAnyAnchor {

			anyGlobalConditionPassed := false
			var lastGlobalAnchorError error = nil

			for _, resourceElement := range resourceElements {
				err := preProcessRecursive(logger, patternElement, resourceElement)
				if err != nil {
					switch err.(type) {
					case ConditionError:
						// Skip element, if condition has failed
						continue
					case GlobalConditionError:
						lastGlobalAnchorError = err
						continue
					}

					return err
				} else {
					if hasGlobalConditions {
						// global anchor has passed, there is no need to return an error
						anyGlobalConditionPassed = true
					}

					// If condition is satisfied, create new pattern list element based on patternElement
					// but related with current resource element by name.
					// Resource element must have name. Without name kustomize won't be able to update this element.
					// In case if element does not have name, skip it.
					resourceElementName := resourceElement.Field("name")
					if resourceElementName.IsNilOrEmpty() {
						continue
					}

					newNode := patternElement.Copy()
					empty, err := deleteConditionsFromNestedMaps(newNode)
					if err != nil {
						return err
					}

					// Do not add an empty element to the patch
					if empty {
						continue
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

			if !anyGlobalConditionPassed && lastGlobalAnchorError != nil {
				return lastGlobalAnchorError
			}
		}
	}

	return nil
}

// validateConditions checks all conditions from current map.
// If at least one condition fails, return error.
// If caller handles list of maps and gets an error, it must skip element.
// If caller handles list of maps and gets GlobalConditionError, it must skip entire rule.
// If caller handles map, it must stop processing and skip entire rule.
func validateConditions(logger logr.Logger, pattern, resource *yaml.RNode) error {
	var err error
	err = validateConditionsInternal(logger, pattern, resource, anchor.IsGlobalAnchor)
	if err != nil {
		return NewGlobalConditionError(err)
	}

	err = validateConditionsInternal(logger, pattern, resource, anchor.IsConditionAnchor)
	if err != nil {
		return NewConditionError(err)
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

func hasAnchor(key string) bool {
	return anchor.ContainsCondition(key) || anchor.IsAddingAnchor(key)
}

func hasAnchors(pattern *yaml.RNode, isAnchor func(key string) bool) bool {
	if yaml.MappingNode == pattern.YNode().Kind {
		fields, err := pattern.Fields()
		if err != nil {
			return false
		}

		for _, key := range fields {
			if isAnchor(key) {
				return true
			}

			patternNode := pattern.Field(key)
			if !patternNode.IsNilOrEmpty() {
				if hasAnchors(patternNode.Value, isAnchor) {
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

	return err
}

func deleteConditionsFromNestedMaps(pattern *yaml.RNode) (bool, error) {
	if pattern.YNode().Kind != yaml.MappingNode {
		return false, nil
	}

	fields, err := pattern.Fields()
	if err != nil {
		return false, err
	}

	for _, field := range fields {
		if anchor.ContainsCondition(field) {
			err = pattern.PipeE(yaml.Clear(field))
			if err != nil {
				return false, err
			}
		} else {
			child := pattern.Field(field).Value
			if child != nil {
				empty, err := deleteConditionsFromNestedMaps(child)
				if err != nil {
					return false, err
				}

				if empty {
					err = pattern.PipeE(yaml.Clear(field))
					if err != nil {
						return false, err
					}
				}
			}
		}
	}

	fields, err = pattern.Fields()
	if err != nil {
		return false, err
	}

	if len(fields) == 0 {
		return true, nil
	}

	return false, nil
}

func deleteConditionElements(pattern *yaml.RNode) error {
	fields, err := pattern.Fields()
	if err != nil {
		return err
	}

	for _, field := range fields {
		ok, err := deleteAnchors(pattern.Field(field).Value)
		if err != nil {
			return err
		}
		if ok {
			err = pattern.PipeE(yaml.Clear(field))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteAnchors deletes all the anchors and returns true,
// if this node must be deleted from patch.
// Node is considered to be deleted, if there were only
// anchors elemets. After anchors elements are removed,
// we have patch with nil values which could cause
// unnecessary resource elements deletion.
func deleteAnchors(node *yaml.RNode) (bool, error) {
	switch node.YNode().Kind {
	case yaml.MappingNode:
		return deleteAnchorsInMap(node)
	case yaml.SequenceNode:
		return deleteAnchorsInList(node)
	}

	return false, nil
}

func deleteAnchorsInMap(node *yaml.RNode) (bool, error) {
	conditions, err := filterKeys(node, anchor.ContainsCondition)
	if err != nil {
		return false, err
	}

	// Remove all conditions first.
	for _, condition := range conditions {
		err = node.PipeE(yaml.Clear(condition))
		if err != nil {
			return false, err
		}
	}

	fields, err := node.Fields()
	if err != nil {
		return false, err
	}

	needToDelete := true

	// Go further through the map elements.
	for _, field := range fields {
		ok, err := deleteAnchors(node.Field(field).Value)
		if err != nil {
			return false, err
		}

		if ok {
			err = node.PipeE(yaml.Clear(field))
			if err != nil {
				return false, err
			}
		} else {
			// If we have at least one element without anchor,
			// then we don't need to delete this element.
			needToDelete = false
		}
	}

	return needToDelete, nil
}

func deleteAnchorsInList(node *yaml.RNode) (bool, error) {
	elements, err := node.Elements()
	if err != nil {
		return false, err
	}

	wasEmpty := len(elements) == 0

	for i, element := range elements {
		if hasAnchors(element, hasAnchor) {
			deleteListElement(node, i)
		} else {
			// This element also could have some conditions
			// inside sub-arrays. Delete them too.

			ok, err := deleteAnchors(element)
			if err != nil {
				return false, err
			}
			if ok {
				deleteListElement(node, i)
			}
		}
	}

	elements, err = node.Elements()
	if err != nil {
		return false, err
	}
	if len(elements) == 0 && !wasEmpty {
		return true, nil
	}

	return false, nil
}

func deleteListElement(list *yaml.RNode, i int) {
	content := list.YNode().Content
	list.YNode().Content = append(content[:i], content[i+1:]...)
}

func validateConditionsInternal(logger logr.Logger, pattern, resource *yaml.RNode, filter func(string) bool) error {
	conditions, err := filterKeys(pattern, filter)
	if err != nil {
		return err
	}

	for _, condition := range conditions {
		conditionKey := removeAnchor(condition)
		if resource == nil || resource.Field(conditionKey) == nil {
			return fmt.Errorf("could not found \"%s\" key in the resource", conditionKey)
		}

		err = checkCondition(logger, pattern.Field(condition).Value, resource.Field(conditionKey).Value)
		if err != nil {
			return err
		}
	}

	return nil
}
