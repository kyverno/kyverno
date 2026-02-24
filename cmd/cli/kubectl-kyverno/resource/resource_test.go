package resource

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestYamlToUnstructured_EmptyAnnotations(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		checkFn func(*testing.T, *unstructured.Unstructured)
	}{
		{
			name: "pod with nil annotations",
			yaml: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations:
spec:
  containers:
  - name: nginx
    image: nginx`,
			wantErr: false,
			checkFn: func(t *testing.T, resource *unstructured.Unstructured) {
				annotations := resource.GetAnnotations()
				if annotations == nil {
					t.Error("Expected annotations to be empty map, got nil")
				}
				if len(annotations) != 0 {
					t.Errorf("Expected empty annotations, got %v", annotations)
				}
			},
		},
		{
			name: "pod with empty annotations map",
			yaml: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations: {}
spec:
  containers:
  - name: nginx
    image: nginx`,
			wantErr: false,
			checkFn: func(t *testing.T, resource *unstructured.Unstructured) {
				annotations := resource.GetAnnotations()
				if annotations == nil {
					t.Error("Expected annotations to be empty map, got nil")
				}
				if len(annotations) != 0 {
					t.Errorf("Expected empty annotations, got %v", annotations)
				}
			},
		},
		{
			name: "deployment with nil template annotations",
			yaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
      annotations:
    spec:
      containers:
      - name: nginx
        image: nginx`,
			wantErr: false,
			checkFn: func(t *testing.T, resource *unstructured.Unstructured) {
				// Check that template.metadata.annotations was normalized to empty map
				templateMeta, found, err := unstructured.NestedMap(resource.Object, "spec", "template", "metadata")
				if err != nil {
					t.Fatalf("Error getting template metadata: %v", err)
				}
				if !found {
					t.Fatal("Template metadata not found")
				}
				annotations, ok := templateMeta["annotations"]
				if !ok {
					t.Error("Expected annotations field to exist")
				}
				// Should be empty map, not nil
				annotationsMap, ok := annotations.(map[string]interface{})
				if !ok {
					t.Errorf("Expected annotations to be map[string]interface{}, got %T", annotations)
				}
				if len(annotationsMap) != 0 {
					t.Errorf("Expected empty annotations map, got %v", annotationsMap)
				}
			},
		},
		{
			name: "deployment with nil labels",
			yaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  labels:
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx`,
			wantErr: false,
			checkFn: func(t *testing.T, resource *unstructured.Unstructured) {
				labels := resource.GetLabels()
				if labels == nil {
					t.Error("Expected labels to be empty map, got nil")
				}
				if len(labels) != 0 {
					t.Errorf("Expected empty labels, got %v", labels)
				}
			},
		},
		{
			name: "configmap with nil data",
			yaml: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:`,
			wantErr: false,
			checkFn: func(t *testing.T, resource *unstructured.Unstructured) {
				data, found, err := unstructured.NestedMap(resource.Object, "data")
				if err != nil {
					t.Fatalf("Error getting data: %v", err)
				}
				if !found {
					t.Error("Expected data field to exist")
				}
				if len(data) != 0 {
					t.Errorf("Expected empty data map, got %v", data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := YamlToUnstructured([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("YamlToUnstructured() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.checkFn != nil {
				tt.checkFn(t, resource)
			}
		})
	}
}

func TestNormalizeNilMaps(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "normalize nil annotations",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": nil,
				},
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
				},
			},
		},
		{
			name: "normalize nil labels",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": nil,
				},
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{},
				},
			},
		},
		{
			name: "normalize nested nil annotations in template",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": nil,
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{},
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
					},
				},
			},
		},
		{
			name: "don't modify non-nil annotations",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"key": "value",
					},
				},
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"key": "value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeNilMaps(tt.input)
			// Check if the result matches expected
			checkMapsEqual(t, tt.expected, tt.input)
		})
	}
}

func checkMapsEqual(t *testing.T, expected, actual map[string]interface{}) {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			t.Errorf("Expected key %s not found in actual map", key)
			continue
		}

		switch expectedVal := expectedValue.(type) {
		case map[string]interface{}:
			actualVal, ok := actualValue.(map[string]interface{})
			if !ok {
				t.Errorf("Expected map for key %s, got %T", key, actualValue)
				continue
			}
			checkMapsEqual(t, expectedVal, actualVal)
		default:
			if expectedValue != actualValue {
				t.Errorf("For key %s, expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
