/*
Copyright 2026 The Kyverno Authors.

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

package namespace

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func Test_isNil(t *testing.T) {
	var nilNamespacePtr *corev1.Namespace
	var nonNilNamespacePtr = &corev1.Namespace{}
	var concreteStruct = corev1.Namespace{}
	var nilChan chan int
	var nonNilChan = make(chan int)
	var nilFunc func()
	var nonNilFunc = func() {}
	var nilMap map[string]string
	var nonNilMap = map[string]string{}
	var nilSlice []string
	var nonNilSlice = []string{}
	var nilInterface runtime.Object

	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{name: "nil interface", input: nil, expected: true},
		{name: "typed nil interface variable", input: nilInterface, expected: true},
		{name: "typed nil pointer", input: nilNamespacePtr, expected: true},
		{name: "non-nil pointer", input: nonNilNamespacePtr, expected: false},
		{name: "concrete struct (non-pointer)", input: concreteStruct, expected: false},
		{name: "primitive int", input: 42, expected: false},
		{name: "primitive string", input: "test", expected: false},
		{name: "primitive bool", input: true, expected: false},
		{name: "nil chan", input: nilChan, expected: true},
		{name: "non-nil chan", input: nonNilChan, expected: false},
		{name: "nil func", input: nilFunc, expected: true},
		{name: "non-nil func", input: nonNilFunc, expected: false},
		{name: "nil map", input: nilMap, expected: true},
		{name: "non-nil map", input: nonNilMap, expected: false},
		{name: "nil slice", input: nilSlice, expected: true},
		{name: "non-nil slice", input: nonNilSlice, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNil(tt.input)
			if got != tt.expected {
				t.Errorf("isNil(%s) = %v, expected %v", tt.name, got, tt.expected)
			}
		})
	}
}

func Test_isCoreNamespace(t *testing.T) {
	tests := []struct {
		name     string
		attr     admission.Attributes
		expected bool
	}{
		{
			name:     "nil attributes",
			attr:     nil,
			expected: false,
		},
		{
			name:     "core v1 Namespace with full GVK and GVR",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, "", "test-ns", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: true,
		},
		{
			name:     "core Namespace with empty version GVK and GVR",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "", Version: "", Kind: "Namespace"}, "", "test-ns", schema.GroupVersionResource{Group: "", Version: "", Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: true,
		},
		{
			name:     "core namespaces resource with empty GVK",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, "", "test-ns", schema.GroupVersionResource{Group: "", Version: "", Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: true,
		},
		{
			name:     "custom resource with Resource namespaces in custom API group",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "custom.example.com", Version: "v1", Kind: "CustomNamespace"}, "", "test-cr", schema.GroupVersionResource{Group: "custom.example.com", Version: "v1", Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: false,
		},
		{
			name:     "custom resource with Kind Namespace in custom API group",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Namespace"}, "", "test-cr", schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "namespaces"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: false,
		},
		{
			name:     "ordinary namespaced pod",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, "default", "test-pod", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: false,
		},
		{
			name:     "cluster-scoped clusterrole",
			attr:     admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, "", "admin", schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}, "", admission.Delete, &metav1.DeleteOptions{}, false, nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCoreNamespace(tt.attr)
			if got != tt.expected {
				t.Errorf("isCoreNamespace(%s) = %v, expected %v", tt.name, got, tt.expected)
			}
		})
	}
}
