package resource

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFixupGenerateLabels(t *testing.T) {
	tests := []struct {
		name string
		obj  unstructured.Unstructured
		want unstructured.Unstructured
	}{{
		name: "not set",
	}, {
		name: "empty",
		obj:  unstructured.Unstructured{Object: map[string]interface{}{}},
		want: unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "kyverno",
				},
			},
		}},
	}, {
		name: "with label",
		obj: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
					},
				},
			},
		},
		want: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
					},
				},
			},
		},
	}, {
		name: "with generate labels",
		obj: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"foo":                                   "bar",
						"generate.kyverno.io/policy-name":       "add-networkpolicy",
						"generate.kyverno.io/policy-namespace":  "",
						"generate.kyverno.io/rule-name":         "default-deny",
						"generate.kyverno.io/trigger-group":     "",
						"generate.kyverno.io/trigger-kind":      "Namespace",
						"generate.kyverno.io/trigger-name":      "hello-world-namespace",
						"generate.kyverno.io/trigger-namespace": "default",
						"generate.kyverno.io/trigger-version":   "v1",
					},
				},
			},
		},
		want: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
						"foo":                          "bar",
					},
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FixupGenerateLabels(tt.obj)
			if !reflect.DeepEqual(tt.obj, tt.want) {
				t.Errorf("FixupGenerateLabels() = %v, want %v", tt.obj, tt.want)
			}
		})
	}
}
