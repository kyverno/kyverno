package jsonutils

import (
	"fmt"
	"strconv"

	"github.com/kyverno/kyverno/pkg/engine/common"
)

// ActionData represents data available for action on current element
type ActionData struct {
	Document interface{}
	Element  interface{}
	Path     string
}

// Action encapsulates the logic that must be performed for each
// JSON element
type Action func(data *ActionData) (interface{}, error)

// OnlyForLeafsAndKeys is an action modifier - apply action only for leafs and map keys
func OnlyForLeafsAndKeys(action Action) Action {
	return func(data *ActionData) (interface{}, error) {
		switch typed := data.Element.(type) {
		case map[string]interface{}, []interface{}: // skip arrays and maps
			return data.Element, nil
		case Key:
			data.Element = typed.Key
			return action(data)
		default: // leaf detected
			return action(data)
		}
	}
}

// Traversal is a type that encapsulates JSON traversal algorithm
// It traverses entire JSON structure applying some logic to its elements
type Traversal struct {
	document interface{}
	action   Action
}

// NewTraversal creates JSON Traversal object
func NewTraversal(document interface{}, action Action) *Traversal {
	return &Traversal{
		document,
		action,
	}
}

// Key type is needed for traversal to specify the key
type Key struct {
	Key string
}

// TraverseJSON performs a traverse of JSON document and applying
// action for each JSON element
func (t *Traversal) TraverseJSON() (interface{}, error) {
	return t.traverseJSON(t.document, "")
}

func (t *Traversal) traverseJSON(element interface{}, path string) (interface{}, error) {
	// perform an action
	element, err := t.action(&ActionData{t.document, element, path})
	if err != nil {
		return element, err
	}

	// traverse further
	switch typed := element.(type) {
	case map[string]interface{}:
		return t.traverseObject(common.CopyMap(typed), path)

	case []interface{}:
		return t.traverseList(common.CopySlice(typed), path)

	case []map[string]interface{}:
		return t.traverseList(common.CopySliceOfMaps(typed), path)

	case Key:
		return typed.Key, nil

	default:
		return element, nil
	}
}

func (t *Traversal) traverseObject(object map[string]interface{}, path string) (map[string]interface{}, error) {
	for key, element := range object {
		newKey, err := t.traverseJSON(Key{key}, path)
		if err != nil {
			return nil, err
		}
		var newKeyStr string
		if newKey == nil {
			newKeyStr = key
		} else {
			var ok bool
			if newKeyStr, ok = newKey.(string); !ok {
				return nil, fmt.Errorf("expected string after substituting variables in key \"%s\"", key)
			}
		}

		value, err := t.traverseJSON(element, path+"/"+key)
		if err != nil {
			return nil, err
		}

		if newKeyStr != key {
			// key was renamed after var substitution
			// delete old key
			object[newKeyStr] = value
			delete(object, key)
		} else {
			object[key] = value
		}
	}
	return object, nil
}

func (t *Traversal) traverseList(list []interface{}, path string) ([]interface{}, error) {
	for idx, element := range list {
		value, err := t.traverseJSON(element, path+"/"+strconv.Itoa(idx))
		if err != nil {
			return nil, err
		}
		list[idx] = value
	}
	return list, nil
}
