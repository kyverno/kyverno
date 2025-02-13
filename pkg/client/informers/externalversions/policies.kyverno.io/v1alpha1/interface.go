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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// CELPolicyExceptions returns a CELPolicyExceptionInformer.
	CELPolicyExceptions() CELPolicyExceptionInformer
	// ImageVerificationPolicies returns a ImageVerificationPolicyInformer.
	ImageVerificationPolicies() ImageVerificationPolicyInformer
	// ValidatingPolicies returns a ValidatingPolicyInformer.
	ValidatingPolicies() ValidatingPolicyInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// CELPolicyExceptions returns a CELPolicyExceptionInformer.
func (v *version) CELPolicyExceptions() CELPolicyExceptionInformer {
	return &cELPolicyExceptionInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// ImageVerificationPolicies returns a ImageVerificationPolicyInformer.
func (v *version) ImageVerificationPolicies() ImageVerificationPolicyInformer {
	return &imageVerificationPolicyInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// ValidatingPolicies returns a ValidatingPolicyInformer.
func (v *version) ValidatingPolicies() ValidatingPolicyInformer {
	return &validatingPolicyInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
