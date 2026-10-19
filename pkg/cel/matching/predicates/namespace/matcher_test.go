/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespace_test

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/matching/predicates/namespace"
	registrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/webhook"
)

type structNamespace struct {
	name   string
	labels map[string]string
}

func (s structNamespace) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (s structNamespace) DeepCopyObject() runtime.Object   { return s }

func (s structNamespace) GetNamespace() string                           { return "" }
func (s structNamespace) SetNamespace(namespace string)                  {}
func (s structNamespace) GetName() string                                { return s.name }
func (s structNamespace) SetName(name string)                            {}
func (s structNamespace) GetGenerateName() string                        { return "" }
func (s structNamespace) SetGenerateName(name string)                    {}
func (s structNamespace) GetUID() types.UID                              { return "" }
func (s structNamespace) SetUID(uid types.UID)                           {}
func (s structNamespace) GetResourceVersion() string                     { return "" }
func (s structNamespace) SetResourceVersion(version string)              {}
func (s structNamespace) GetGeneration() int64                           { return 0 }
func (s structNamespace) SetGeneration(generation int64)                 {}
func (s structNamespace) GetSelfLink() string                            { return "" }
func (s structNamespace) SetSelfLink(selfLink string)                    {}
func (s structNamespace) GetCreationTimestamp() metav1.Time              { return metav1.Time{} }
func (s structNamespace) SetCreationTimestamp(t metav1.Time)             {}
func (s structNamespace) GetDeletionTimestamp() *metav1.Time             { return nil }
func (s structNamespace) SetDeletionTimestamp(t *metav1.Time)            {}
func (s structNamespace) GetDeletionGracePeriodSeconds() *int64          { return nil }
func (s structNamespace) SetDeletionGracePeriodSeconds(sec *int64)       {}
func (s structNamespace) GetLabels() map[string]string                   { return s.labels }
func (s structNamespace) SetLabels(labels map[string]string)             {}
func (s structNamespace) GetAnnotations() map[string]string              { return nil }
func (s structNamespace) SetAnnotations(a map[string]string)             {}
func (s structNamespace) GetFinalizers() []string                        { return nil }
func (s structNamespace) SetFinalizers(finalizers []string)              {}
func (s structNamespace) GetOwnerReferences() []metav1.OwnerReference    { return nil }
func (s structNamespace) SetOwnerReferences(r []metav1.OwnerReference)   {}
func (s structNamespace) GetManagedFields() []metav1.ManagedFieldsEntry  { return nil }
func (s structNamespace) SetManagedFields(f []metav1.ManagedFieldsEntry) {}

type nonAccessorStruct struct{}

func (nonAccessorStruct) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (nonAccessorStruct) DeepCopyObject() runtime.Object   { return nonAccessorStruct{} }

func TestGetNamespaceLabels(t *testing.T) {
	namespace1Labels := map[string]string{
		"runlevel": "1",
	}
	namespace1 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "1",
			Labels: namespace1Labels,
		},
	}
	namespace2Labels := map[string]string{
		"runlevel": "2",
	}
	namespace2 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "2",
			Labels: namespace2Labels,
		},
	}

	tests := []struct {
		name           string
		attr           admission.Attributes
		expectedLabels map[string]string
	}{
		{
			name:           "request is for creating namespace, the labels should be from the object itself",
			attr:           admission.NewAttributesRecord(&namespace2, nil, schema.GroupVersionKind{}, "", namespace2.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectedLabels: namespace2Labels,
		},
		{
			name: "request is for creating namespace with generateName (empty Name), the labels should be from the object itself",
			attr: admission.NewAttributesRecord(
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "generated-ns-",
						Labels:       namespace2Labels,
					},
				},
				nil,
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
				"",
				"",
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
				"",
				admission.Create,
				&metav1.CreateOptions{},
				false,
				nil,
			),
			expectedLabels: namespace2Labels,
		},
		{
			name:           "request is for updating namespace, the labels should be from the new object",
			attr:           admission.NewAttributesRecord(&namespace2, nil, schema.GroupVersionKind{}, namespace2.Name, namespace2.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Update, &metav1.UpdateOptions{}, false, nil),
			expectedLabels: namespace2Labels,
		},
		{
			name:           "request is for deleting namespace, the labels should be from the cache",
			attr:           admission.NewAttributesRecord(&namespace2, nil, schema.GroupVersionKind{}, namespace1.Name, namespace1.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name:           "request is for deleting namespace with valid OldObject, the labels should be from OldObject",
			attr:           admission.NewAttributesRecord(nil, &namespace2, schema.GroupVersionKind{}, "", namespace2.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expectedLabels: namespace2Labels,
		},
		{
			name:           "request is for deleting namespace with empty unstructured OldObject, the labels should be from the cache",
			attr:           admission.NewAttributesRecord(nil, &unstructured.Unstructured{}, schema.GroupVersionKind{}, namespace1.Name, namespace1.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name:           "request is for deleting namespace with typed-nil OldObject, the labels should be from the cache",
			attr:           admission.NewAttributesRecord(nil, (*corev1.Namespace)(nil), schema.GroupVersionKind{}, namespace1.Name, namespace1.Name, schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name:           "request is for deleting namespace with concrete struct OldObject, the labels should be from OldObject",
			attr:           admission.NewAttributesRecord(nil, structNamespace{name: "2", labels: namespace2Labels}, schema.GroupVersionKind{}, "", "2", schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expectedLabels: namespace2Labels,
		},
		{
			name:           "request is for namespace/finalizer",
			attr:           admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, namespace1.Name, "mock-name", schema.GroupVersionResource{Resource: "namespaces"}, "finalizers", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name:           "request is for pod",
			attr:           admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, namespace1.Name, "mock-name", schema.GroupVersionResource{Resource: "pods"}, "", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name: "request is for namespaced custom resource with Resource namespaces, labels should be from cache",
			attr: admission.NewAttributesRecord(
				nil,
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "custom.example.com/v1",
						"kind":       "CustomNamespace",
						"metadata": map[string]interface{}{
							"name":      "cr-1",
							"namespace": namespace1.Name,
							"labels": map[string]interface{}{
								"runlevel": "99",
							},
						},
					},
				},
				schema.GroupVersionKind{Group: "custom.example.com", Version: "v1", Kind: "CustomNamespace"},
				namespace1.Name,
				"cr-1",
				schema.GroupVersionResource{Group: "custom.example.com", Version: "v1", Resource: "namespaces"},
				"",
				admission.Delete,
				&metav1.DeleteOptions{},
				false,
				nil,
			),
			expectedLabels: namespace1Labels,
		},
	}
	matcher := namespace.Matcher{
		Namespace: &namespace1,
	}
	for _, tt := range tests {
		actualLabels, err := matcher.GetNamespaceLabels(tt.attr)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(actualLabels, tt.expectedLabels) {
			t.Errorf("expected labels to be %#v, got %#v", tt.expectedLabels, actualLabels)
		}
	}
}

func TestGetNamespaceLabels_NotFound(t *testing.T) {
	namespace1 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "1",
			Labels: map[string]string{"runlevel": "1"},
		},
	}
	matcher := namespace.Matcher{
		Namespace: &namespace1,
	}

	tests := []struct {
		name string
		attr admission.Attributes
	}{
		{
			name: "request is for deleting namespace with nil OldObject and mismatched name",
			attr: admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, "", "2", schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
		},
		{
			name: "request is for deleting namespace with empty unstructured OldObject and mismatched name",
			attr: admission.NewAttributesRecord(nil, &unstructured.Unstructured{}, schema.GroupVersionKind{}, "", "2", schema.GroupVersionResource{Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := matcher.GetNamespaceLabels(tt.attr)
			if labels != nil {
				t.Errorf("expected nil labels, got %#v", labels)
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !apierrors.IsNotFound(err) {
				t.Errorf("expected not-found error, got %v", err)
			}
		})
	}
}

func TestNotExemptClusterScopedResource(t *testing.T) {
	hook := &registrationv1.ValidatingWebhook{
		NamespaceSelector: &metav1.LabelSelector{},
	}
	attr := admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{},
		"",
		"mock-name",
		schema.GroupVersionResource{Version: "v1", Resource: "nodes"},
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		nil,
	)
	matcher := namespace.Matcher{}
	matches, err := matcher.MatchNamespaceSelector(webhook.NewValidatingWebhookAccessor("mock-hook", "mock-cfg", hook), attr)
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Errorf("cluster scoped resources (but not a namespace) should not be exempted from webhooks")
	}
}
func TestGetNamespaceWithNilNamespace(t *testing.T) {
	// Test CLI mode where namespace object is nil
	matcher := namespace.Matcher{
		Namespace: nil,
	}

	// Test getting namespace when matcher has nil namespace (CLI mode)
	ns, err := matcher.GetNamespace("default")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ns == nil {
		t.Fatal("expected namespace object, got nil")
	}
	if ns.GetName() != "default" {
		t.Errorf("expected namespace name 'default', got %s", ns.GetName())
	}
	labels := ns.GetLabels()
	if labels == nil {
		t.Fatal("expected labels, got nil")
	}
	if labels["kubernetes.io/metadata.name"] != "default" {
		t.Errorf("expected label kubernetes.io/metadata.name='default', got %s", labels["kubernetes.io/metadata.name"])
	}
}

func TestGetNamespaceLabelsWithNilNamespace(t *testing.T) {
	// Test getting labels in CLI mode
	matcher := namespace.Matcher{
		Namespace: nil,
	}

	// Test for a pod in default namespace
	attr := admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{},
		"default",
		"test-pod",
		schema.GroupVersionResource{Resource: "pods"},
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		nil,
	)

	labels, err := matcher.GetNamespaceLabels(attr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if labels == nil {
		t.Fatal("expected labels, got nil")
	}
	if labels["kubernetes.io/metadata.name"] != "default" {
		t.Errorf("expected label kubernetes.io/metadata.name='default', got %s", labels["kubernetes.io/metadata.name"])
	}
}

func TestMatchNamespaceSelectorWithNilNamespace(t *testing.T) {
	// Test namespace selector matching in CLI mode
	matcher := namespace.Matcher{
		Namespace: nil,
	}

	// Create a webhook that selects namespaces with kubernetes.io/metadata.name=default
	hook := &registrationv1.ValidatingWebhook{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"kubernetes.io/metadata.name": "default",
			},
		},
	}

	// Test with a pod in default namespace - should match
	attr := admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{},
		"default",
		"test-pod",
		schema.GroupVersionResource{Resource: "pods"},
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		nil,
	)

	matches, err := matcher.MatchNamespaceSelector(
		webhook.NewValidatingWebhookAccessor("test-hook", "test-cfg", hook),
		attr,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !matches {
		t.Error("expected namespace selector to match for default namespace, but it didn't")
	}

	// Test with a pod in kube-system namespace - should NOT match
	attr2 := admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{},
		"kube-system",
		"test-pod",
		schema.GroupVersionResource{Resource: "pods"},
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		nil,
	)

	matches2, err2 := matcher.MatchNamespaceSelector(
		webhook.NewValidatingWebhookAccessor("test-hook", "test-cfg", hook),
		attr2,
	)
	if err2 != nil {
		t.Fatalf("expected no error, got %v", err2)
	}
	if matches2 {
		t.Error("expected namespace selector to NOT match for kube-system, but it did")
	}

	// Test with empty namespace selector - should match everything
	hookEmpty := &registrationv1.ValidatingWebhook{
		NamespaceSelector: &metav1.LabelSelector{},
	}

	matches3, err3 := matcher.MatchNamespaceSelector(
		webhook.NewValidatingWebhookAccessor("test-hook-empty", "test-cfg", hookEmpty),
		attr,
	)
	if err3 != nil {
		t.Fatalf("expected no error with empty selector, got %v", err3)
	}
	if !matches3 {
		t.Error("expected empty namespace selector to match, but it didn't")
	}
}

func TestGetNamespaceDifferentNames(t *testing.T) {
	// Test getting different namespace names in CLI mode
	matcher := namespace.Matcher{
		Namespace: nil,
	}

	testCases := []string{
		"default",
		"kube-system",
		"kube-public",
		"my-custom-namespace",
	}

	for _, nsName := range testCases {
		ns, err := matcher.GetNamespace(nsName)
		if err != nil {
			t.Errorf("unexpected error for namespace %s: %v", nsName, err)
			continue
		}
		if ns.GetName() != nsName {
			t.Errorf("expected namespace name %s, got %s", nsName, ns.GetName())
		}
		if ns.GetLabels()["kubernetes.io/metadata.name"] != nsName {
			t.Errorf("expected label kubernetes.io/metadata.name=%s, got %s",
				nsName, ns.GetLabels()["kubernetes.io/metadata.name"])
		}
	}
}

func TestMatchNamespaceSelector_NamespaceDelete(t *testing.T) {
	criticalHook := &registrationv1.ValidatingWebhook{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"critical": "true",
			},
		},
	}
	accessor := webhook.NewValidatingWebhookAccessor("test-hook", "test-cfg", criticalHook)

	prodNamespace := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "production",
			Labels: map[string]string{
				"critical": "true",
			},
		},
	}

	nonCriticalNamespace := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "staging",
			Labels: map[string]string{
				"critical": "false",
			},
		},
	}

	// Matcher with nil cache (typical in CLI or cache-miss admission paths)
	matcherWithoutCache := namespace.Matcher{Namespace: nil}

	// Matcher with cache
	cachedNs := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cached-ns",
			Labels: map[string]string{
				"critical": "true",
			},
		},
	}
	matcherWithCache := namespace.Matcher{Namespace: &cachedNs}

	t.Run("DELETE operation with OldObject matching labels returns true", func(t *testing.T) {
		attr := admission.NewAttributesRecord(
			nil,
			&prodNamespace,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			prodNamespace.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected namespace selector to match for deleted namespace with matching labels")
		}
	})

	t.Run("DELETE operation with OldObject nonmatching labels returns false", func(t *testing.T) {
		attr := admission.NewAttributesRecord(
			nil,
			&nonCriticalNamespace,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			nonCriticalNamespace.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if matches {
			t.Errorf("expected namespace selector NOT to match for deleted namespace with nonmatching labels")
		}
	})

	t.Run("DELETE operation with nil OldObject falls back to cache/resolver", func(t *testing.T) {
		attr := admission.NewAttributesRecord(
			nil,
			nil,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			cachedNs.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected nil OldObject to fall back to cache and match")
		}
	})

	t.Run("DELETE operation with typed-nil OldObject falls back to cache/resolver", func(t *testing.T) {
		var nilNs *corev1.Namespace
		attr := admission.NewAttributesRecord(
			nil,
			nilNs,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			cachedNs.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected typed-nil OldObject to fall back to cache and match")
		}
	})

	t.Run("DELETE operation with concrete struct OldObject matching labels returns true without panicking", func(t *testing.T) {
		concreteNs := structNamespace{
			name: "production",
			labels: map[string]string{
				"critical": "true",
			},
		}
		attr := admission.NewAttributesRecord(
			nil,
			concreteNs,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			concreteNs.name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected concrete struct OldObject with matching labels to match")
		}
	})

	t.Run("DELETE operation with concrete struct OldObject nonmatching labels returns false without panicking", func(t *testing.T) {
		concreteNs := structNamespace{
			name: "staging",
			labels: map[string]string{
				"critical": "false",
			},
		}
		attr := admission.NewAttributesRecord(
			nil,
			concreteNs,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			concreteNs.name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if matches {
			t.Errorf("expected concrete struct OldObject with nonmatching labels NOT to match")
		}
	})

	t.Run("DELETE operation with nonAccessorStruct OldObject returns false with internal error", func(t *testing.T) {
		attr := admission.NewAttributesRecord(
			nil,
			nonAccessorStruct{},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			"test-ns",
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if matches {
			t.Errorf("expected namespace selector NOT to match when OldObject accessor fails")
		}
		if err == nil {
			t.Fatalf("expected internal error, got nil")
		}
		if apierrors.IsNotFound(err) {
			t.Errorf("expected internal error, not NotFound error")
		}
		if !apierrors.IsInternalError(err) {
			t.Errorf("expected internal error, got %v", err)
		}
	})

	t.Run("DELETE operation with non-nil empty unstructured OldObject falls back to cache/resolver", func(t *testing.T) {
		emptyUnstructured := &unstructured.Unstructured{}
		attr := admission.NewAttributesRecord(
			nil,
			emptyUnstructured,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			cachedNs.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected non-nil empty OldObject to fall back to cache and match")
		}
	})

	t.Run("DELETE operation with nil OldObject and name mismatch returns not found error", func(t *testing.T) {
		attr := admission.NewAttributesRecord(
			nil,
			nil,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			"different-ns",
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if matches {
			t.Errorf("expected namespace selector NOT to match when requested namespace is not found in cache")
		}
		if err == nil {
			t.Fatalf("expected not-found error, got nil")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected not-found error, got %v", err)
		}
	})

	t.Run("DELETE operation with non-nil empty unstructured OldObject and name mismatch returns not found error", func(t *testing.T) {
		emptyUnstructured := &unstructured.Unstructured{}
		attr := admission.NewAttributesRecord(
			nil,
			emptyUnstructured,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			"different-ns",
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if matches {
			t.Errorf("expected namespace selector NOT to match when requested namespace is not found in cache")
		}
		if err == nil {
			t.Fatalf("expected not-found error, got nil")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected not-found error, got %v", err)
		}
	})

	t.Run("CREATE and UPDATE operations match via Object", func(t *testing.T) {
		attrCreate := admission.NewAttributesRecord(
			&prodNamespace,
			nil,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			prodNamespace.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Create,
			&metav1.CreateOptions{},
			false,
			nil,
		)

		matchesCreate, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attrCreate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matchesCreate {
			t.Errorf("expected CREATE to match via Object")
		}

		attrUpdate := admission.NewAttributesRecord(
			&prodNamespace,
			&prodNamespace,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			prodNamespace.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Update,
			&metav1.UpdateOptions{},
			false,
			nil,
		)

		matchesUpdate, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attrUpdate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matchesUpdate {
			t.Errorf("expected UPDATE to match via Object")
		}
	})

	t.Run("CREATE operation with generateName matches via Object", func(t *testing.T) {
		genNamespace := corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "production-",
				Labels: map[string]string{
					"critical": "true",
				},
			},
		}
		attrCreate := admission.NewAttributesRecord(
			&genNamespace,
			nil,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			"",
			"",
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"",
			admission.Create,
			&metav1.CreateOptions{},
			false,
			nil,
		)

		matchesCreate, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attrCreate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matchesCreate {
			t.Errorf("expected CREATE with generateName to match via Object")
		}
	})

	t.Run("DELETE requests for ordinary namespaced resources check parent namespace", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: cachedNs.Name,
			},
		}
		attrPodDelete := admission.NewAttributesRecord(
			nil,
			&pod,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			cachedNs.Name,
			pod.Name,
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attrPodDelete)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected namespaced pod DELETE to match based on parent namespace labels")
		}
	})

	t.Run("Cluster-scoped resource other than namespaces is not exempted", func(t *testing.T) {
		attrClusterRole := admission.NewAttributesRecord(
			nil,
			nil,
			schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
			"",
			"admin",
			schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attrClusterRole)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected cluster-scoped non-namespace resource not to be exempted by namespaceSelector")
		}
	})

	t.Run("Namespace subresource does not treat subresource as namespace object", func(t *testing.T) {
		attrSubresource := admission.NewAttributesRecord(
			&prodNamespace,
			nil,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			cachedNs.Name,
			"production",
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			"finalize",
			admission.Update,
			&metav1.UpdateOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attrSubresource)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected namespace subresource to look up parent namespace from cache")
		}
	})

	t.Run("Cluster-scoped custom resource with Resource namespaces is not treated as core namespace", func(t *testing.T) {
		customNsCR := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "custom.example.com/v1",
				"kind":       "CustomNamespace",
				"metadata": map[string]interface{}{
					"name": "custom-cr",
					"labels": map[string]interface{}{
						"critical": "false",
					},
				},
			},
		}
		attr := admission.NewAttributesRecord(
			nil,
			customNsCR,
			schema.GroupVersionKind{Group: "custom.example.com", Version: "v1", Kind: "CustomNamespace"},
			"",
			"custom-cr",
			schema.GroupVersionResource{Group: "custom.example.com", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithoutCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected cluster-scoped custom resource not to be exempted by namespaceSelector")
		}
	})

	t.Run("Namespaced custom resource with Resource namespaces resolves parent namespace labels", func(t *testing.T) {
		customNsCR := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "custom.example.com/v1",
				"kind":       "CustomNamespace",
				"metadata": map[string]interface{}{
					"name":      "custom-cr",
					"namespace": cachedNs.Name,
					"labels": map[string]interface{}{
						"critical": "false",
					},
				},
			},
		}
		attr := admission.NewAttributesRecord(
			nil,
			customNsCR,
			schema.GroupVersionKind{Group: "custom.example.com", Version: "v1", Kind: "CustomNamespace"},
			cachedNs.Name,
			"custom-cr",
			schema.GroupVersionResource{Group: "custom.example.com", Version: "v1", Resource: "namespaces"},
			"",
			admission.Delete,
			&metav1.DeleteOptions{},
			false,
			nil,
		)

		matches, err := matcherWithCache.MatchNamespaceSelector(accessor, attr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Errorf("expected namespaced custom resource to match via parent namespace labels")
		}

		labels, getErr := matcherWithCache.GetNamespaceLabels(attr)
		if getErr != nil {
			t.Fatalf("unexpected error: %v", getErr)
		}
		if !reflect.DeepEqual(labels, cachedNs.Labels) {
			t.Errorf("expected parent namespace labels %#v, got %#v", cachedNs.Labels, labels)
		}
	})
}

func TestGetNamespace_NilAndConcrete(t *testing.T) {
	t.Run("nil interface returns minimalNamespace", func(t *testing.T) {
		m := namespace.Matcher{Namespace: nil}
		ns, err := m.GetNamespace("test-nil-interface")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ns.GetName() != "test-nil-interface" {
			t.Errorf("expected name 'test-nil-interface', got '%s'", ns.GetName())
		}
		if ns.GetLabels()["kubernetes.io/metadata.name"] != "test-nil-interface" {
			t.Errorf("expected label kubernetes.io/metadata.name='test-nil-interface', got '%s'", ns.GetLabels()["kubernetes.io/metadata.name"])
		}
	})

	t.Run("typed-nil pointer returns minimalNamespace", func(t *testing.T) {
		var nilNs *corev1.Namespace
		m := namespace.Matcher{Namespace: nilNs}
		ns, err := m.GetNamespace("test-typed-nil")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ns.GetName() != "test-typed-nil" {
			t.Errorf("expected name 'test-typed-nil', got '%s'", ns.GetName())
		}
		if ns.GetLabels()["kubernetes.io/metadata.name"] != "test-typed-nil" {
			t.Errorf("expected label kubernetes.io/metadata.name='test-typed-nil', got '%s'", ns.GetLabels()["kubernetes.io/metadata.name"])
		}
	})

	t.Run("non-nil pointer returns accessor or not found", func(t *testing.T) {
		realNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cached-ns",
				Labels: map[string]string{
					"env": "production",
				},
			},
		}
		m := namespace.Matcher{Namespace: realNs}

		ns, err := m.GetNamespace("cached-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ns.GetName() != "cached-ns" {
			t.Errorf("expected name 'cached-ns', got '%s'", ns.GetName())
		}
		if ns.GetLabels()["env"] != "production" {
			t.Errorf("expected label env='production', got '%s'", ns.GetLabels()["env"])
		}

		_, errMismatch := m.GetNamespace("different-ns")
		if errMismatch == nil {
			t.Fatalf("expected error for name mismatch, got nil")
		}
		if !apierrors.IsNotFound(errMismatch) {
			t.Errorf("expected not-found error, got %v", errMismatch)
		}
	})

	t.Run("concrete non-pointer struct returns accessor without panicking", func(t *testing.T) {
		concreteNs := structNamespace{
			name: "concrete-ns",
			labels: map[string]string{
				"tier": "frontend",
			},
		}
		m := namespace.Matcher{Namespace: concreteNs}

		ns, err := m.GetNamespace("concrete-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ns.GetName() != "concrete-ns" {
			t.Errorf("expected name 'concrete-ns', got '%s'", ns.GetName())
		}
		if ns.GetLabels()["tier"] != "frontend" {
			t.Errorf("expected label tier='frontend', got '%s'", ns.GetLabels()["tier"])
		}

		_, errMismatch := m.GetNamespace("different-ns")
		if errMismatch == nil {
			t.Fatalf("expected error for name mismatch, got nil")
		}
		if !apierrors.IsNotFound(errMismatch) {
			t.Errorf("expected not-found error, got %v", errMismatch)
		}
	})

	t.Run("concrete non-pointer struct not implementing metav1.Object does not panic", func(t *testing.T) {
		m := namespace.Matcher{Namespace: nonAccessorStruct{}}
		_, err := m.GetNamespace("any-ns")
		if err == nil {
			t.Fatalf("expected error from meta.Accessor, got nil")
		}
	})
}
