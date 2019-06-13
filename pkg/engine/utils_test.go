package engine

import (
	"testing"

	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceMeetsDescription_Kind(t *testing.T) {
	resourceName := "test-config-map"
	resourceDescription := types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	groupVersionKind := metav1.GroupVersionKind{Kind: "ConfigMap"}

	rawResource := []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)

	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
	resourceDescription.Kinds[0] = "Deployment"
	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
	resourceDescription.Kinds[0] = "ConfigMap"
	groupVersionKind.Kind = "Deployment"
	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
}

func TestResourceMeetsDescription_Name(t *testing.T) {
	resourceName := "test-config-map"
	resourceDescription := types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	groupVersionKind := metav1.GroupVersionKind{Kind: "ConfigMap"}

	rawResource := []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)

	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
	resourceName = "test-config-map-new"
	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	rawResource = []byte(`{
		"metadata":{
			"name":"test-config-map-new",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)
	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	rawResource = []byte(`{
		"metadata":{
			"name":"",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)
	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
}

func TestResourceMeetsDescription_MatchExpressions(t *testing.T) {
	resourceName := "test-config-map"
	resourceDescription := types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
				metav1.LabelSelectorRequirement{
					Key:      "label1",
					Operator: "In",
					Values: []string{
						"test1",
						"test8",
						"test201",
					},
				},
				metav1.LabelSelectorRequirement{
					Key:      "label3",
					Operator: "DoesNotExist",
					Values:   nil,
				},
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "In",
					Values: []string{
						"test2",
					},
				},
			},
		},
	}
	groupVersionKind := metav1.GroupVersionKind{Kind: "ConfigMap"}
	rawResource := []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)

	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	rawResource = []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1234567890",
				"label2":"test2"
			}
		}
	}`)

	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
}

func TestResourceMeetsDescription_MatchLabels(t *testing.T) {
	resourceName := "test-config-map"
	resourceDescription := types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
				"label2": "test2",
			},
			MatchExpressions: nil,
		},
	}
	groupVersionKind := metav1.GroupVersionKind{Kind: "ConfigMap"}

	rawResource := []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)
	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	rawResource = []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label3":"test1",
				"label2":"test2"
			}
		}
	}`)
	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	resourceDescription = types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label3": "test1",
				"label2": "test2",
			},
			MatchExpressions: nil,
		},
	}

	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
}

func TestResourceMeetsDescription_MatchLabelsAndMatchExpressions(t *testing.T) {
	resourceName := "test-config-map"
	resourceDescription := types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "In",
					Values: []string{
						"test2",
					},
				},
			},
		},
	}
	groupVersionKind := metav1.GroupVersionKind{Kind: "ConfigMap"}

	rawResource := []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)

	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	resourceDescription = types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
			},
		},
	}

	rawResource = []byte(`{
		"metadata":{
			"name":"test-config-map",
			"namespace":"default",
			"creationTimestamp":null,
			"labels":{
				"label1":"test1",
				"label2":"test2"
			}
		}
	}`)
	assert.Assert(t, ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	resourceDescription = types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "In",
					Values: []string{
						"sometest1",
					},
				},
			},
		},
	}

	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))

	resourceDescription = types.ResourceDescription{
		Kinds: []string{"ConfigMap"},
		Name:  &resourceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"label1": "test1",
				"label3": "test3",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "label2",
					Operator: "In",
					Values: []string{
						"test2",
					},
				},
			},
		},
	}

	assert.Assert(t, false == ResourceMeetsDescription(rawResource, resourceDescription, groupVersionKind))
}

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	assert.Assert(t, isConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	assert.Assert(t, isConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	assert.Assert(t, !isConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	assert.Assert(t, !isConditionAnchor(str))
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	assert.Assert(t, !isConditionAnchor(str))
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	assert.Assert(t, !isConditionAnchor(str))
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	assert.Assert(t, !isConditionAnchor(str))
}

func TestIsExistanceAnchor_Yes(t *testing.T) {
	assert.Assert(t, isExistanceAnchor("^(abc)"))
}

func TestIsExistanceAnchor_NoRightBracket(t *testing.T) {
	assert.Assert(t, !isExistanceAnchor("^(abc"))
}

func TestIsExistanceAnchor_OnlyHat(t *testing.T) {
	assert.Assert(t, !isExistanceAnchor("^abc"))
}

func TestIsExistanceAnchor_ConditionAnchor(t *testing.T) {
	assert.Assert(t, !isExistanceAnchor("(abc)"))
}

func TestRemoveAnchor_ConditionAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("(abc)"), "abc")
}

func TestRemoveAnchor_ExistanceAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("^(abc)"), "abc")
}

func TestRemoveAnchor_EmptyExistanceAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("^()"), "")
}
