package patch

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/pkg/errors"
	"sigs.k8s.io/kustomize/kyaml/yaml"
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
	if _, err := handleAddIfNotPresentAnchor(pattern, resource); err != nil {
		return errors.Wrap(err, "failed to process addIfNotPresent anchor")
	}

	if err := validateConditions(logger, pattern, resource); err != nil {
		return err // do not wrap condition errors
	}

	isNotAnchor := func(key string) bool {
		return !hasAnchor(key)
	}

	nonAnchors, err := filterKeys(pattern, isNotAnchor)
	if err != nil {
		return err
	}

	for _, field := range nonAnchors {
		var resourceValue *yaml.RNode
		if resource != nil && resource.Field(field) != nil {
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
			patternElementCopy := patternElement.Copy()

			for _, resourceElement := range resourceElements {
				if err := preProcessRecursive(logger, patternElementCopy, resourceElement); err != nil {
					logger.V(3).Info("anchor mismatch", "reason", err.Error())
					if isConditionError(err) {
						continue
					}

					if isGlobalConditionError(err) {
						lastGlobalAnchorError = err
						continue
					}

					return err
				}

				if hasGlobalConditions {
					// global anchor has passed, there is no need to return an error
					anyGlobalConditionPassed = true
				} else {
					if err := handlePatternName(pattern, patternElementCopy, resourceElement); err != nil {
						return errors.Wrap(err, "failed to update name in pattern")
					}
				}
			}
			if resource == nil {
				if err := preProcessRecursive(logger, patternElementCopy, resource); err != nil {
					logger.V(3).Info("anchor mismatch", "reason", err.Error())
					if isConditionError(err) {
						continue
					}

					return err
				}

				if hasGlobalConditions {
					// global anchor has passed, there is no need to return an error
					anyGlobalConditionPassed = true
				}
			}
			if !anyGlobalConditionPassed && lastGlobalAnchorError != nil {
				return lastGlobalAnchorError
			}
		}
	}

	return nil
}

func handlePatternName(pattern, patternElement, resourceElement *yaml.RNode) error {
	// If condition is satisfied, create new pattern list element based on patternElement
	// but related with current resource element by name.
	// Resource element must have name. Without name kustomize won't be able to update this element.
	// In case if element does not have name, skip it.
	resourceElementName := resourceElement.Field("name")
	if resourceElementName.IsNilOrEmpty() {
		return nil
	}

	newNode := patternElement.Copy()
	empty, err := deleteAnchors(newNode, true, false)
	if err != nil {
		return err
	}

	// Do not add an empty element to the patch
	if empty {
		return nil
	}

	err = newNode.PipeE(yaml.SetField("name", resourceElementName.Value))
	if err != nil {
		return err
	}

	err = pattern.PipeE(yaml.Append(newNode.YNode()))
	if err != nil {
		return err
	}

	return nil
}

func isConditionError(err error) bool {
	_, ok := err.(ConditionError)
	return ok
}

func isGlobalConditionError(err error) bool {
	_, ok := err.(GlobalConditionError)
	return ok
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

// handleAddIfNotPresentAnchor handles adding anchors.
// Remove anchor from pattern, if field already exists.
// Remove anchor wrapping from key, if field does not exist in the resource.
func handleAddIfNotPresentAnchor(pattern, resource *yaml.RNode) (int, error) {
	anchors, err := filterKeys(pattern, anchor.IsAddIfNotPresentAnchor)
	if err != nil {
		return 0, err
	}

	for _, a := range anchors {
		key, _ := anchor.RemoveAnchor(a)
		if resource != nil && resource.Field(key) != nil {
			// Resource already has this field.
			// Delete the field with addIfNotPresent anchor from patch.
			err = pattern.PipeE(yaml.Clear(a))
			if err != nil {
				return 0, err
			}
		} else {
			// Remove anchor tags from patch field key.
			renameField(a, key, pattern)
		}
	}

	return len(anchors), nil
}

func filterKeys(pattern *yaml.RNode, condition func(string) bool) ([]string, error) {
	if !isMappingNode(pattern) {
		return nil, nil
	}

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

func isMappingNode(node *yaml.RNode) bool {
	if err := yaml.ErrorIfInvalid(node, yaml.MappingNode); err != nil {
		return false
	}

	return true
}

func hasAnchor(key string) bool {
	return anchor.ContainsCondition(key) || anchor.IsAddIfNotPresentAnchor(key)
}

func hasAnchors(pattern *yaml.RNode, isAnchor func(key string) bool) bool {
	ynode := pattern.YNode() //nolint:ifshort
	if ynode.Kind == yaml.MappingNode {
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
	} else if ynode.Kind == yaml.ScalarNode {
		v := ynode.Value
		return anchor.ContainsCondition(v)
	} else if ynode.Kind == yaml.SequenceNode {
		elements, _ := pattern.Elements()
		for _, e := range elements {
			if hasAnchors(e, isAnchor) {
				return true
			}
		}

		return false
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

	err = validate.MatchPattern(logger, resourceInterface, patternInterface)
	if err != nil {
		return err
	}

	return nil
}

func deleteConditionElements(pattern *yaml.RNode) error {
	fields, err := pattern.Fields()
	if err != nil {
		return err
	}

	for _, field := range fields {
		deleteScalar := anchor.ContainsCondition(field)
		canDelete, err := deleteAnchors(pattern.Field(field).Value, deleteScalar, false)
		if err != nil {
			return err
		}
		if canDelete {
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
// anchors elements. After anchors elements are removed,
// A patch with nil values which could cause
// unnecessary resource elements deletion.
func deleteAnchors(node *yaml.RNode, deleteScalar, traverseMappingNodes bool) (bool, error) {
	switch node.YNode().Kind {
	case yaml.MappingNode:
		return deleteAnchorsInMap(node, traverseMappingNodes)
	case yaml.SequenceNode:
		return deleteAnchorsInList(node, traverseMappingNodes)
	case yaml.ScalarNode:
		return deleteScalar, nil
	}

	return false, nil
}

func deleteAnchorsInMap(node *yaml.RNode, traverseMappingNodes bool) (bool, error) {
	conditions, err := filterKeys(node, anchor.ContainsCondition)
	if err != nil {
		return false, err
	}

	// remove all conditional anchors with no child nodes first
	anchorsExist := false
	for _, condition := range conditions {
		field := node.Field(condition)
		shouldDelete, err := deleteAnchors(field.Value, true, traverseMappingNodes)
		if err != nil {
			return false, err
		}

		if shouldDelete {
			if err := node.PipeE(yaml.Clear(condition)); err != nil {
				return false, err
			}
		} else {
			anchorsExist = true
		}
	}

	if anchorsExist {
		if err := stripAnchorsFromNode(node, ""); err != nil {
			return false, errors.Wrap(err, "failed to remove anchor tags")
		}
	}

	fields, err := node.Fields()
	if err != nil {
		return false, err
	}

	needToDelete := true
	for _, field := range fields {
		canDelete, err := deleteAnchors(node.Field(field).Value, false, traverseMappingNodes)
		if err != nil {
			return false, err
		}

		if canDelete {
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

// stripAnchorFromNode strips one or more anchor fields from the node.
// If key is "" all anchor fields are stripped. Otherwise, only the matching
// field is stripped.
func stripAnchorsFromNode(node *yaml.RNode, key string) error {
	anchors, err := filterKeys(node, anchor.ContainsCondition)
	if err != nil {
		return err
	}

	for _, a := range anchors {
		k, _ := anchor.RemoveAnchor(a)
		if key == "" || k == key {
			renameField(a, k, node)
		}
	}

	return nil
}

func deleteAnchorsInList(node *yaml.RNode, traverseMappingNodes bool) (bool, error) {
	elements, err := node.Elements()
	if err != nil {
		return false, err
	}

	wasEmpty := len(elements) == 0
	for i, element := range elements {
		if hasAnchors(element, hasAnchor) {
			shouldDelete := true
			if traverseMappingNodes && isMappingNode(element) {
				shouldDelete, err = deleteAnchors(element, true, traverseMappingNodes)
				if err != nil {
					return false, errors.Wrap(err, "failed to delete anchors")
				}
			}

			if shouldDelete {
				deleteListElement(node, i)
			}
		} else {
			// This element also could have some conditions
			// inside sub-arrays. Delete them too.
			canDelete, err := deleteAnchors(element, false, traverseMappingNodes)
			if err != nil {
				return false, errors.Wrap(err, "failed to delete anchors")
			}
			if canDelete {
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
		conditionKey, _ := anchor.RemoveAnchor(condition)
		if resource == nil || resource.Field(conditionKey) == nil {
			return fmt.Errorf("could not found \"%s\" key in the resource", conditionKey)
		}

		patternValue := pattern.Field(condition).Value
		resourceValue := resource.Field(conditionKey).Value
		if count, err := handleAddIfNotPresentAnchor(patternValue, resourceValue); err != nil {
			return err
		} else if count > 0 {
			continue
		}

		if err := checkCondition(logger, patternValue, resourceValue); err != nil {
			return err
		}
	}

	return nil
}
