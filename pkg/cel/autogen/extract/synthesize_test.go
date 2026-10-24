package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func parentAttributes(namespace string) admission.Attributes {
	return admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		namespace,
		"my-jobset",
		schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)
}

func TestSynthesizePodAttributesNamespace(t *testing.T) {
	tests := []struct {
		name            string
		parentNamespace string
		metadata        map[string]any
		want            string
	}{
		{
			name:            "template without metadata uses the parent namespace",
			parentNamespace: "default",
			want:            "default",
		},
		{
			name:            "template without a namespace uses the parent namespace",
			parentNamespace: "default",
			metadata:        map[string]any{"labels": map[string]any{"app": "worker"}},
			want:            "default",
		},
		{
			name:            "template namespace never overrides a namespaced parent",
			parentNamespace: "default",
			metadata:        map[string]any{"namespace": "kube-system"},
			want:            "default",
		},
		{
			name:            "cluster-scoped parent falls back to the template namespace",
			parentNamespace: "",
			metadata:        map[string]any{"namespace": "workloads"},
			want:            "workloads",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := map[string]any{
				"spec": map[string]any{"containers": []any{container("worker", "nginx")}},
			}
			if tt.metadata != nil {
				// a copy, so an in-place change to the template shows up as a
				// difference from tt.metadata below
				template["metadata"] = runtime.DeepCopyJSON(tt.metadata)
			}
			tpl := &Extracted{Template: template, Path: "spec.template"}
			attr := SynthesizePodAttributes(tpl, tpl, parentAttributes(tt.parentNamespace))

			assert.Equal(t, tt.want, attr.GetNamespace())
			assert.Equal(t, tt.want, attr.GetObject().(*unstructured.Unstructured).GetNamespace())
			assert.Equal(t, tt.want, attr.GetOldObject().(*unstructured.Unstructured).GetNamespace())
			if tt.metadata != nil {
				// the parent's template itself must be left untouched
				assert.Equal(t, tt.metadata, template["metadata"])
			}
		})
	}
}
