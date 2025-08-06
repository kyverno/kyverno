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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/webhook"
)

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
			name:           "request is for namespace/finalizer",
			attr:           admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, namespace1.Name, "mock-name", schema.GroupVersionResource{Resource: "namespaces"}, "finalizers", admission.Create, &metav1.CreateOptions{}, false, nil),
			expectedLabels: namespace1Labels,
		},
		{
			name:           "request is for pod",
			attr:           admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, namespace1.Name, "mock-name", schema.GroupVersionResource{Resource: "pods"}, "", admission.Create, &metav1.CreateOptions{}, false, nil),
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
