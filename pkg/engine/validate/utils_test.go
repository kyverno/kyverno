package validate

import (
	"container/list"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasNestedAnchors_EmptyMap(t *testing.T) {
	pattern := map[string]interface{}{}
	result := hasNestedAnchors(pattern)
	assert.False(t, result, "empty map should not have anchors")
}

func TestHasNestedAnchors_NoAnchors(t *testing.T) {
	pattern := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test",
		},
	}
	result := hasNestedAnchors(pattern)
	assert.False(t, result, "map without anchors should return false")
}

func TestHasNestedAnchors_WithConditionAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"(metadata)": map[string]interface{}{
			"name": "test",
		},
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "map with condition anchor should return true")
}

func TestHasNestedAnchors_WithExistenceAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"^(containers)": []interface{}{
			map[string]interface{}{"name": "test"},
		},
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "map with existence anchor should return true")
}

func TestHasNestedAnchors_WithEqualityAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"=(metadata)": map[string]interface{}{
			"name": "test",
		},
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "map with equality anchor should return true")
}

func TestHasNestedAnchors_WithNegationAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"X(badField)": "value",
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "map with negation anchor should return true")
}

func TestHasNestedAnchors_WithGlobalAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"<(globalRef)": "value",
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "map with global anchor should return true")
}

func TestHasNestedAnchors_NestedAnchor(t *testing.T) {
	pattern := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"(containers)": []interface{}{
					map[string]interface{}{"name": "test"},
				},
			},
		},
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "deeply nested anchor should be detected")
}

func TestHasNestedAnchors_ArrayWithNoAnchors(t *testing.T) {
	pattern := []interface{}{
		map[string]interface{}{"name": "test"},
		map[string]interface{}{"name": "test2"},
	}
	result := hasNestedAnchors(pattern)
	assert.False(t, result, "array without anchors should return false")
}

func TestHasNestedAnchors_ArrayWithAnchors(t *testing.T) {
	pattern := []interface{}{
		map[string]interface{}{"(name)": "test"},
	}
	result := hasNestedAnchors(pattern)
	assert.True(t, result, "array with anchor in element should return true")
}

func TestHasNestedAnchors_PrimitiveTypes(t *testing.T) {
	testCases := []struct {
		name    string
		pattern interface{}
	}{
		{"string", "test"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasNestedAnchors(tc.pattern)
			assert.False(t, result, "primitive type %s should return false", tc.name)
		})
	}
}

func TestGetAnchorsFromMap_Empty(t *testing.T) {
	anchorsMap := map[string]interface{}{}
	result := getAnchorsFromMap(anchorsMap)
	assert.Empty(t, result, "empty map should return empty result")
}

func TestGetAnchorsFromMap_NoAnchors(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"metadata": "value",
		"spec":     "value",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Empty(t, result, "map without anchors should return empty result")
}

func TestGetAnchorsFromMap_ConditionAnchor(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"(metadata)": "value",
		"spec":       "other",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "(metadata)")
}

func TestGetAnchorsFromMap_ExistenceAnchor(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"^(containers)": "value",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "^(containers)")
}

func TestGetAnchorsFromMap_EqualityAnchor(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"=(name)": "value",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "=(name)")
}

func TestGetAnchorsFromMap_NegationAnchor(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"X(badField)": "value",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "X(badField)")
}

func TestGetAnchorsFromMap_GlobalAnchor(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"<(globalRef)": "value",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "<(globalRef)")
}

func TestGetAnchorsFromMap_MultipleAnchors(t *testing.T) {
	anchorsMap := map[string]interface{}{
		"(condition)":  "value1",
		"^(existence)": "value2",
		"=(equality)":  "value3",
		"normalKey":    "value4",
	}
	result := getAnchorsFromMap(anchorsMap)
	assert.Len(t, result, 3)
	assert.NotContains(t, result, "normalKey")
}

func TestGetSortedNestedAnchorResource_Empty(t *testing.T) {
	resources := map[string]interface{}{}
	result := getSortedNestedAnchorResource(resources)
	assert.Equal(t, 0, result.Len())
}

func TestGetSortedNestedAnchorResource_NoAnchors(t *testing.T) {
	resources := map[string]interface{}{
		"alpha": "value1",
		"beta":  "value2",
		"gamma": "value3",
	}
	result := getSortedNestedAnchorResource(resources)
	assert.Equal(t, 3, result.Len())
	// Should be sorted alphabetically
	keys := listToSlice(result)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, keys)
}

func TestGetSortedNestedAnchorResource_WithNestedAnchors(t *testing.T) {
	resources := map[string]interface{}{
		"zeta": "value1",
		"alpha": map[string]interface{}{
			"(anchor)": "nestedValue",
		},
		"beta": "value2",
	}
	result := getSortedNestedAnchorResource(resources)
	keys := listToSlice(result)
	// alpha should be first because it has nested anchors
	assert.Equal(t, "alpha", keys[0])
}

func TestGetSortedNestedAnchorResource_GlobalAnchorFirst(t *testing.T) {
	resources := map[string]interface{}{
		"zeta":         "value1",
		"<(globalRef)": "globalValue",
		"alpha":        "value2",
	}
	result := getSortedNestedAnchorResource(resources)
	keys := listToSlice(result)
	// Global anchor should be first
	assert.Equal(t, "<(globalRef)", keys[0])
}

// Helper function to convert list to slice
func listToSlice(l *list.List) []string {
	result := make([]string, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(string))
	}
	return result
}
