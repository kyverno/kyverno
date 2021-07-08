package mutate

import (
	anchor "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/minio/pkg/wildcard"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// preProcessPattern - Dynamically preProcess the yaml
// 1> For conditional anchor remove anchors from the pattern.
// 2> For Adding anchors remove anchor tags.

// The whole yaml is structured as a pointer tree.
// https://godoc.org/gopkg.in/yaml.v3#Node
// A single Node contains Tag to identify it as MappingNode (map[string]interface{}), Sequence ([]interface{}), ScalarNode (string, int, float bool etc.)
// A parent node having MappingNode keeps the data as <keyNode>, <ValueNode> inside it's Content field and Tag field as "!!map".
// A parent node having Sequence keeps the data as array of Node inside Content field and a Tag field as "!!seq".
// https://github.com/kubernetes-sigs/kustomize/blob/master/kyaml/yaml/rnode.go
func preProcessPattern(pattern, resource *yaml.RNode) error {
	switch pattern.YNode().Kind {
	case yaml.MappingNode:
		err := walkMap(pattern, resource)
		if err != nil {
			return err
		}
	case yaml.SequenceNode:
		err := walkArray(pattern, resource)
		if err != nil {
			return err
		}
	case yaml.ScalarNode:
		if pattern.YNode().Value != resource.YNode().Value {
			if wildcard.Match(pattern.YNode().Value, resource.YNode().Value) {
			}
		}
	}
	return nil
}

// getIndex - get the index of the key from the fields.
var getIndex = func(k string, list []string) int {
	for i, v := range list {
		if v == k {
			return 2 * i
		}
	}
	return -1
}

// removeAnchorNode - removes anchor nodes from yaml
func removeAnchorNode(targetNode *yaml.RNode, index int) {
	targetNode.YNode().Content = append(targetNode.YNode().Content[:index], targetNode.YNode().Content[index+2:]...)
}

func removeKeyFromFields(key string, fields []string) []string {
	for i, v := range fields {
		if v == key {
			return append(fields[:i], fields[i+1:]...)
		}
	}
	return fields
}

// walkMap - walk through the MappingNode
/* 1> For conditional anchor remove anchors from the pattern, patchStrategicMerge will add the anchors as a new patch,
so it is necessary to remove the anchor mapsfrom the pattern before calling patchStrategicMerge.
| (volumes):
| - (hostPath):
|   path: "/var/run/docker.sock"
walkMap will remove the node containing (volumes) from the yaml
*/

/* 2> For Adding anchors remove anchor tags.
annotations:
 - "+(annotation1)": "atest1"
will remove "+(" and ")" chars from pattern.
*/
func walkMap(pattern, resource *yaml.RNode) error {
	var err error

	// 1. Get all conditions from current map.
	// If at least one condition fails - skip application.
	// If condition fails in array element - skip this element.
	conditions, err := getConditions(pattern)
	if err != nil {
		return err
	}

	for _, condition := range conditions {
		conditionKey := removeAnchor(condition)
		resourceField := resource.Field(conditionKey)
		if resourceField == nil {
			continue
		}

		// err = validate(pattern.Field(condition).Value, resourceField.Value)
		// if err != nil {
		// 	return err
		// }
	}

	// 2. If all conditions passed, remove them from patch
	for _, condition := range conditions {
		_, err = pattern.Pipe(yaml.Clear(condition))
		if err != nil {
			return err
		}
	}

	// 3. Handle all non-conditional keys
	fields, err := pattern.Fields()
	for i, field := range fields {

		// 4. Handle adding anchors
		if anchor.IsAddingAnchor(field) {
			key, _ := anchor.RemoveAnchor(field)

			resourceNode := resource.Field(key)
			if resourceNode != nil {
				// Resource already has this field.
				// Delete the field with adding andhor from patch.
				_, err = pattern.Pipe(yaml.Clear(field))
				if err != nil {
					return err
				}
				continue
			}

			// Remove anchor wrap from patch field.
			renameField(field, key, pattern)
		}

		// 5. Handle non-anchor key
		// Just go down the pattern and handle other anchors.
		// If resource element exists, then go one level down.
		var resourceNode *yaml.RNode

		if r := resource.Field(field); r != nil {
			resourceNode = r.Value
		} else {
			resourceNode = nil
		}

		err := preProcessPattern(pattern.Field(field).Value, resourceNode)
		if err != nil {
			return err
		}
	}
	return nil
}

// walkArray - walk through array elements
// 1> processNonAssocSequence - process array of basic types. Ex:- {command: ["ls", "ls -l"]}
// 2> processAssocSequence - process array having MappingNode. like containers, volumes etc.
func walkArray(pattern, resource *yaml.RNode) error {
	pafs, err := pattern.Elements()
	if err != nil {
		return err
	}
	if len(pafs) == 0 {
		return nil
	}
	switch pafs[0].YNode().Kind {
	case yaml.MappingNode:
		return processAssocSequence(pattern, resource)
	case yaml.ScalarNode:
		return processNonAssocSequence(pattern, resource)
	}
	return nil
}

// processAssocSequence - process arrays
// in many cases like containers, volumes kustomize uses name field to match resource for processing
// 1> If any conditional anchor match resource field and if the pattern doesn't contains "name" field and
// 		resource contains "name" field then copy the name field from resource to pattern.
// 2> If the resource doesn't contains "name" field then just remove anchor field from yaml.
/*
  Policy:
		"spec": {
			"containers": [{
			"(image)": "*:latest",
			"imagePullPolicy": "Always"
		}]}

  Resource:
	    "spec": {
			"containers": [
				{
				"name": "nginx",
				"image": "nginx:latest",
				"imagePullPolicy": "Never"
				}]
		}
	After Preprocessing:
		"spec": {
			"containers": [{
			"name": "nginx",
			"imagePullPolicy": "Always"
		}]}

	kustomize uses name field to match resource for processing. So if containers doesn't contains name field then it will be skipped.
	So if a conditional anchor image matches resource then remove "(image)" field from yaml and add the matching names from the resource.
*/
func processAssocSequence(pattern, resource *yaml.RNode) error {
	patternElements, err := pattern.Elements()
	if err != nil {
		return err
	}
	for _, patternElement := range patternElements {
		if hasAnchors(patternElement) {
			err := processAnchorSequence(patternElement, resource, pattern)
			if err != nil {
				return err
			}
		}
	}
	// remove the elements with anchors
	err = removeAnchorElements(pattern)
	if err != nil {
		return err
	}
	return preProcessArrayPattern(pattern, resource)
}

func preProcessArrayPattern(pattern, resource *yaml.RNode) error {
	patternElements, err := pattern.Elements()
	if err != nil {
		return err
	}
	resourceElements, err := resource.Elements()
	if err != nil {
		return err
	}
	for _, patternElement := range patternElements {
		patternNameField := patternElement.Field("name")
		if patternNameField != nil {
			patternNameValue, err := patternNameField.Value.String()
			if err != nil {
				return err
			}
			for _, resourceElement := range resourceElements {
				resourceNameField := resourceElement.Field("name")
				if resourceNameField != nil {
					resourceNameValue, err := resourceNameField.Value.String()
					if err != nil {
						return err
					}
					if patternNameValue == resourceNameValue {
						err := preProcessPattern(patternElement, resourceElement)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

/*
	removeAnchorSequence :- removes element containing conditional anchor

	Pattern:
		"spec": {
			"containers": [{
			"(image)": "*:latest",
			"imagePullPolicy": "Always"
		},
		{
			"name": "nginx",
			"imagePullPolicy": "Always"
		}]}
	After Removing Conditional Sequence:
		"spec": {
			"containers": [{
			"name": "nginx",
			"imagePullPolicy": "Always"
		}]}
*/
func removeAnchorElements(pattern *yaml.RNode) error {
	patternElements, err := pattern.Elements()
	if err != nil {
		return err
	}

	removedIndex, err := getIndexToBeRemoved(patternElements)
	if err != nil {
		return err
	}

	if len(removedIndex) == 0 {
		return nil
	}

	preservedPatterns := removeByIndex(pattern, removedIndex)
	pattern.YNode().Content = preservedPatterns
	return nil
}

func processAnchorSequence(pattern, resource, arrayPattern *yaml.RNode) error {
	resourceElements, err := resource.Elements()
	if err != nil {
		return err
	}
	switch pattern.YNode().Kind {
	case yaml.MappingNode:
		for _, resourceElement := range resourceElements {
			err := processAnchorMap(pattern, resourceElement, arrayPattern)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// processAnchorMap - process arrays
// in many cases like containers, volumes kustomize uses name field to match resource for processing
// 1> If any conditional anchor match resource field and if the pattern doesn't contains "name" field and
// 		resource contains "name" field then copy the name field from resource to pattern.
// 2> If the resource doesn't contains "name" field then just remove anchor field from yaml.
/*
  Policy:
		"spec": {
			"containers": [{
			"(image)": "*:latest",
			"imagePullPolicy": "Always"
		}]}

  Resource:
	    "spec": {
			"containers": [
				{
				"name": "nginx",
				"image": "nginx:latest",
				"imagePullPolicy": "Never"
				}]
		}
	After Preprocessing:
		"spec": {
			"containers": [{
			"(image)": "*:latest",
			"imagePullPolicy": "Always"
		},
		{
			"name": "nginx",
			"imagePullPolicy": "Always"
		}]}

	kustomize uses name field to match resource for processing. So if containers doesn't contains name field then it will be skipped.
	So if a conditional anchor image matches resouce then remove "(image)" field from yaml and add the matching names from the resource.
*/
func processAnchorMap(pattern, resource, arrayPattern *yaml.RNode) error {
	sfields, fields, err := getAnchorSortedFields(pattern)
	if err != nil {
		return err
	}
	for _, key := range sfields {
		if anchor.IsConditionAnchor(key) {
			_, efields, err := getAnchorSortedFields(resource)
			if err != nil {
				return err
			}
			noAnchorKey := removeAnchor(key)
			eind := getIndex("name", efields)
			if eind != -1 && getIndex("name", fields) == -1 {
				patternMapNode := pattern.Field(key)
				resourceMapNode := resource.Field(noAnchorKey)
				if resourceMapNode != nil {
					pval, err := patternMapNode.Value.String()
					if err != nil {
						return err
					}
					eval, err := resourceMapNode.Value.String()
					if err != nil {
						return err
					}
					if wildcard.Match(pval, eval) {
						newNodeString, err := pattern.String()
						if err != nil {
							return err
						}
						newNode, err := yaml.Parse(newNodeString)
						if err != nil {
							return err
						}
						for i, ekey := range efields {
							if ekey == noAnchorKey {
								pind := getIndex(key, fields)
								if pind == -1 {
									continue
								}
								removeAnchorNode(newNode, pind)
								sfields = removeKeyFromFields(key, sfields)
								fields = removeKeyFromFields(key, fields)

								if ekey == "name" {
									newNode.YNode().Content = append(newNode.YNode().Content, resource.YNode().Content[2*i])
									newNode.YNode().Content = append(newNode.YNode().Content, resource.YNode().Content[2*i+1])
								}

							} else if ekey == "name" {
								newNode.YNode().Content = append(newNode.YNode().Content, resource.YNode().Content[2*i])
								newNode.YNode().Content = append(newNode.YNode().Content, resource.YNode().Content[2*i+1])
							}
						}
						arrayPattern.YNode().Content = append(arrayPattern.YNode().Content, newNode.YNode())
					}
				}

			} else {
				ind := getIndex(key, fields)
				if ind == -1 {
					continue
				}
				removeAnchorNode(pattern, ind)
				sfields = removeKeyFromFields(key, sfields)
				fields = removeKeyFromFields(key, fields)

			}
			continue
		}
	}
	return nil
}

func processNonAssocSequence(pattern, resource *yaml.RNode) error {
	pafs, err := pattern.Elements()
	if err != nil {
		return err
	}
	rafs, err := resource.Elements()
	if err != nil {
		return err
	}
	for _, sa := range rafs {
		des, err := sa.String()
		if err != nil {
			return err
		}
		ok := false
		for _, ra := range pafs {
			src, err := ra.String()
			if err != nil {
				return err
			}
			if des == src {
				ok = true
				break
			}
		}
		if !ok {
			pattern.YNode().Content = append(pattern.YNode().Content, sa.YNode())
		}

	}
	return nil
}

func getConditions(pattern *yaml.RNode) ([]string, error) {
	conditions := make([]string, 0)
	fields, err := pattern.Fields()
	if err != nil {
		return conditions, err
	}

	for _, key := range fields {
		if anchor.IsConditionAnchor(key) {
			conditions = append(conditions, key)
			continue
		}
	}
	return conditions, nil
}

func hasAnchors(pattern *yaml.RNode) bool {
	switch pattern.YNode().Kind {
	case yaml.MappingNode:
		fields, err := pattern.Fields()
		if err != nil {
			return false
		}
		for _, key := range fields {

			if anchor.IsConditionAnchor(key) || anchor.IsAddingAnchor(key) {
				return true
			}
			patternMapNode := pattern.Field(key)
			if !patternMapNode.IsNilOrEmpty() {
				if hasAnchors(patternMapNode.Value) {
					return true
				}
			}
		}
	case yaml.SequenceNode:
		pafs, err := pattern.Elements()
		if err != nil {
			return false
		}
		for _, pa := range pafs {
			if hasAnchors(pa) {
				return true
			}
		}
	}
	return false
}

func removeByIndex(pattern *yaml.RNode, removedIndex []int) []*yaml.Node {
	preservedPatterns := make([]*yaml.Node, 0)
	i := 0
	for index := 0; index < (len(pattern.YNode().Content)); index++ {
		if i < len(removedIndex) && index == removedIndex[i] {
			i++
			continue
		}

		preservedPatterns = append(preservedPatterns, pattern.YNode().Content[index])
	}
	return preservedPatterns
}

func getIndexToBeRemoved(patternElements []*yaml.RNode) (removedIndex []int, err error) {
	for index, patternElement := range patternElements {
		if hasAnchors(patternElement) {
			sfields, _, err := getAnchorSortedFields(patternElement)
			if err != nil {
				return nil, err
			}
			for _, key := range sfields {
				if anchor.IsConditionAnchor(key) {
					removedIndex = append(removedIndex, index)
					break
				}
			}
		}
	}
	return
}

func getAnchorSortedFields(pattern *yaml.RNode) ([]string, []string, error) {
	anchors := make([]string, 0)
	nonAnchors := make([]string, 0)
	nestedAnchors := make([]string, 0)
	fields, err := pattern.Fields()
	if err != nil {
		return fields, fields, err
	}
	for _, key := range fields {
		if anchor.IsConditionAnchor(key) {
			anchors = append(anchors, key)
			continue
		}
		patternMapNode := pattern.Field(key)

		if !patternMapNode.IsNilOrEmpty() {
			if hasAnchors(patternMapNode.Value) {
				nestedAnchors = append(nestedAnchors, key)
				continue
			}
		}
		nonAnchors = append(nonAnchors, key)
	}
	anchors = append(anchors, nestedAnchors...)
	return append(anchors, nonAnchors...), fields, nil
}

func renameField(name, newName string, pattern *yaml.RNode) {
	pattern.Field(name).Key.YNode().Value = newName
}
