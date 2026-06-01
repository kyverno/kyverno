package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsGvkSupported(t *testing.T) {
	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
		want bool
	}{{
		name: "core event is not supported",
		gvk:  corev1.SchemeGroupVersion.WithKind("Event"),
		want: false,
	}, {
		name: "events.k8s.io/v1 Event is not supported",
		gvk:  eventsv1.SchemeGroupVersion.WithKind("Event"),
		want: false,
	}, {
		name: "events.k8s.io/v1beta1 Event is not supported",
		gvk:  eventsv1beta1.SchemeGroupVersion.WithKind("Event"),
		want: false,
	}, {
		name: "Pod is supported",
		gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		want: true,
	}, {
		name: "Deployment is supported",
		gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		want: true,
	}, {
		name: "ConfigMap is supported",
		gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		want: true,
	}, {
		name: "Service is supported",
		gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
		want: true,
	}, {
		name: "Namespace is supported",
		gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
		want: true,
	}, {
		name: "CRD is supported",
		gvk:  schema.GroupVersionKind{Group: "custom.example.com", Version: "v1", Kind: "MyResource"},
		want: true,
	}, {
		name: "empty GVK is supported",
		gvk:  schema.GroupVersionKind{},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGvkSupported(tt.gvk)
			assert.Equal(t, tt.want, got)
		})
	}
}
