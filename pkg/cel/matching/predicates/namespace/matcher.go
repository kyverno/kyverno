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

package namespace

import (
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/admission"
)

type NamespaceSelectorProvider interface {
	// GetNamespaceSelector gets the webhook NamespaceSelector field.
	GetParsedNamespaceSelector() (labels.Selector, error)
}

// Matcher decides if a request is exempted by the NamespaceSelector of a
// webhook configuration.
type Matcher struct {
	Namespace runtime.Object
}

func (m *Matcher) GetNamespace(name string) (metav1.Object, error) {
	// Check for both nil interface and nil pointer
	if m.Namespace == nil || reflect.ValueOf(m.Namespace).IsNil() {
		return &minimalNamespace{
			name: name,
			labels: map[string]string{
				"kubernetes.io/metadata.name": name,
			},
		}, nil
	}

	accessor, err := meta.Accessor(m.Namespace)
	if err != nil {
		return nil, err
	}

	if accessor.GetName() != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "v1", Resource: "namespaces"}, name)
	}
	return accessor, nil
}

// Validate checks if the Matcher has a NamespaceLister and Client.
func (m *Matcher) Validate() error {
	var errs []error
	if m.Namespace == nil {
		errs = append(errs, fmt.Errorf("the namespace matcher requires a namespace"))
	}
	return utilerrors.NewAggregate(errs)
}

// GetNamespaceLabels gets the labels of the namespace related to the attr.
func (m *Matcher) GetNamespaceLabels(attr admission.Attributes) (map[string]string, error) {
	// If the request itself is creating or updating a namespace, then get the
	// labels from attr.Object, because namespaceLister doesn't have the latest
	// namespace yet.
	//
	// However, if the request is deleting a namespace, then get the label from
	// the namespace in the namespaceLister, because a delete request is not
	// going to change the object, and attr.Object will be a DeleteOptions
	// rather than a namespace object.
	if attr.GetResource().Resource == "namespaces" &&
		len(attr.GetSubresource()) == 0 &&
		(attr.GetOperation() == admission.Create || attr.GetOperation() == admission.Update) {
		accessor, err := meta.Accessor(attr.GetObject())
		if err != nil {
			return nil, err
		}
		return accessor.GetLabels(), nil
	}

	namespace, err := m.GetNamespace(attr.GetNamespace())
	if err != nil {
		return nil, err
	}
	return namespace.GetLabels(), nil
}

// MatchNamespaceSelector decideds whether the request matches the
// namespaceSelctor of the webhook. Only when they match, the webhook is called.
func (m *Matcher) MatchNamespaceSelector(p NamespaceSelectorProvider, attr admission.Attributes) (bool, *apierrors.StatusError) {
	namespaceName := attr.GetNamespace()
	if len(namespaceName) == 0 && attr.GetResource().Resource != "namespaces" {
		// If the request is about a cluster scoped resource, and it is not a
		// namespace, it is never exempted.
		// TODO: figure out a way selective exempt cluster scoped resources.
		// Also update the comment in types.go
		return true, nil
	}
	selector, err := p.GetParsedNamespaceSelector()
	if err != nil {
		return false, apierrors.NewInternalError(err)
	}
	if selector.Empty() {
		return true, nil
	}

	namespaceLabels, err := m.GetNamespaceLabels(attr)
	// this means the namespace is not found, for backwards compatibility,
	// return a 404
	if apierrors.IsNotFound(err) {
		status, ok := err.(apierrors.APIStatus)
		if !ok {
			return false, apierrors.NewInternalError(err)
		}
		return false, &apierrors.StatusError{ErrStatus: status.Status()}
	}
	if err != nil {
		return false, apierrors.NewInternalError(err)
	}
	return selector.Matches(labels.Set(namespaceLabels)), nil
}

// minimalNamespace implements metav1.Object for CLI mode
type minimalNamespace struct {
	name   string
	labels map[string]string
}

func (n *minimalNamespace) GetNamespace() string                           { return "" }
func (n *minimalNamespace) SetNamespace(namespace string)                  {}
func (n *minimalNamespace) GetName() string                                { return n.name }
func (n *minimalNamespace) SetName(name string)                            {}
func (n *minimalNamespace) GetGenerateName() string                        { return "" }
func (n *minimalNamespace) SetGenerateName(name string)                    {}
func (n *minimalNamespace) GetUID() types.UID                              { return "" }
func (n *minimalNamespace) SetUID(uid types.UID)                           {}
func (n *minimalNamespace) GetResourceVersion() string                     { return "" }
func (n *minimalNamespace) SetResourceVersion(version string)              {}
func (n *minimalNamespace) GetGeneration() int64                           { return 0 }
func (n *minimalNamespace) SetGeneration(generation int64)                 {}
func (n *minimalNamespace) GetSelfLink() string                            { return "" }
func (n *minimalNamespace) SetSelfLink(selfLink string)                    {}
func (n *minimalNamespace) GetCreationTimestamp() metav1.Time              { return metav1.Time{} }
func (n *minimalNamespace) SetCreationTimestamp(t metav1.Time)             {}
func (n *minimalNamespace) GetDeletionTimestamp() *metav1.Time             { return nil }
func (n *minimalNamespace) SetDeletionTimestamp(t *metav1.Time)            {}
func (n *minimalNamespace) GetDeletionGracePeriodSeconds() *int64          { return nil }
func (n *minimalNamespace) SetDeletionGracePeriodSeconds(s *int64)         {}
func (n *minimalNamespace) GetLabels() map[string]string                   { return n.labels }
func (n *minimalNamespace) SetLabels(labels map[string]string)             {}
func (n *minimalNamespace) GetAnnotations() map[string]string              { return nil }
func (n *minimalNamespace) SetAnnotations(a map[string]string)             {}
func (n *minimalNamespace) GetFinalizers() []string                        { return nil }
func (n *minimalNamespace) SetFinalizers(finalizers []string)              {}
func (n *minimalNamespace) GetOwnerReferences() []metav1.OwnerReference    { return nil }
func (n *minimalNamespace) SetOwnerReferences(r []metav1.OwnerReference)   {}
func (n *minimalNamespace) GetManagedFields() []metav1.ManagedFieldsEntry  { return nil }
func (n *minimalNamespace) SetManagedFields(f []metav1.ManagedFieldsEntry) {}
