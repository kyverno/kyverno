/*
Copyright The Kubernetes Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/client/applyconfigurations/kyverno/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceFilterApplyConfiguration represents an declarative configuration of the ResourceFilter type for use
// with apply.
type ResourceFilterApplyConfiguration struct {
	*v1.UserInfoApplyConfiguration         `json:"UserInfo,omitempty"`
	*ResourceDescriptionApplyConfiguration `json:"resources,omitempty"`
}

// ResourceFilterApplyConfiguration constructs an declarative configuration of the ResourceFilter type for use with
// apply.
func ResourceFilter() *ResourceFilterApplyConfiguration {
	return &ResourceFilterApplyConfiguration{}
}

// WithRoles adds the given value to the Roles field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Roles field.
func (b *ResourceFilterApplyConfiguration) WithRoles(values ...string) *ResourceFilterApplyConfiguration {
	b.ensureUserInfoApplyConfigurationExists()
	for i := range values {
		b.Roles = append(b.Roles, values[i])
	}
	return b
}

// WithClusterRoles adds the given value to the ClusterRoles field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ClusterRoles field.
func (b *ResourceFilterApplyConfiguration) WithClusterRoles(values ...string) *ResourceFilterApplyConfiguration {
	b.ensureUserInfoApplyConfigurationExists()
	for i := range values {
		b.ClusterRoles = append(b.ClusterRoles, values[i])
	}
	return b
}

// WithSubjects adds the given value to the Subjects field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Subjects field.
func (b *ResourceFilterApplyConfiguration) WithSubjects(values ...rbacv1.Subject) *ResourceFilterApplyConfiguration {
	b.ensureUserInfoApplyConfigurationExists()
	for i := range values {
		b.Subjects = append(b.Subjects, values[i])
	}
	return b
}

func (b *ResourceFilterApplyConfiguration) ensureUserInfoApplyConfigurationExists() {
	if b.UserInfoApplyConfiguration == nil {
		b.UserInfoApplyConfiguration = &v1.UserInfoApplyConfiguration{}
	}
}

// WithKinds adds the given value to the Kinds field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Kinds field.
func (b *ResourceFilterApplyConfiguration) WithKinds(values ...string) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	for i := range values {
		b.Kinds = append(b.Kinds, values[i])
	}
	return b
}

// WithNames adds the given value to the Names field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Names field.
func (b *ResourceFilterApplyConfiguration) WithNames(values ...string) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	for i := range values {
		b.Names = append(b.Names, values[i])
	}
	return b
}

// WithNamespaces adds the given value to the Namespaces field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Namespaces field.
func (b *ResourceFilterApplyConfiguration) WithNamespaces(values ...string) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	for i := range values {
		b.Namespaces = append(b.Namespaces, values[i])
	}
	return b
}

// WithAnnotations puts the entries into the Annotations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Annotations field,
// overwriting an existing map entries in Annotations field with the same key.
func (b *ResourceFilterApplyConfiguration) WithAnnotations(entries map[string]string) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	if b.Annotations == nil && len(entries) > 0 {
		b.Annotations = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.Annotations[k] = v
	}
	return b
}

// WithSelector sets the Selector field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Selector field is set to the value of the last call.
func (b *ResourceFilterApplyConfiguration) WithSelector(value metav1.LabelSelector) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	b.Selector = &value
	return b
}

// WithNamespaceSelector sets the NamespaceSelector field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the NamespaceSelector field is set to the value of the last call.
func (b *ResourceFilterApplyConfiguration) WithNamespaceSelector(value metav1.LabelSelector) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	b.NamespaceSelector = &value
	return b
}

// WithOperations adds the given value to the Operations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Operations field.
func (b *ResourceFilterApplyConfiguration) WithOperations(values ...kyvernov1.AdmissionOperation) *ResourceFilterApplyConfiguration {
	b.ensureResourceDescriptionApplyConfigurationExists()
	for i := range values {
		b.Operations = append(b.Operations, values[i])
	}
	return b
}

func (b *ResourceFilterApplyConfiguration) ensureResourceDescriptionApplyConfigurationExists() {
	if b.ResourceDescriptionApplyConfiguration == nil {
		b.ResourceDescriptionApplyConfiguration = &ResourceDescriptionApplyConfiguration{}
	}
}
