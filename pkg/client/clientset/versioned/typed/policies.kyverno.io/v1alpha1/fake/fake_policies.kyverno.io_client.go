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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policies.kyverno.io/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakePoliciesV1alpha1 struct {
	*testing.Fake
}

func (c *FakePoliciesV1alpha1) CELPolicyExceptions(namespace string) v1alpha1.CELPolicyExceptionInterface {
	return &FakeCELPolicyExceptions{c, namespace}
}

func (c *FakePoliciesV1alpha1) ImageVerificationPolicies() v1alpha1.ImageVerificationPolicyInterface {
	return &FakeImageVerificationPolicies{c}
}

func (c *FakePoliciesV1alpha1) MutatingPolicies() v1alpha1.MutatingPolicyInterface {
	return &FakeMutatingPolicies{c}
}

func (c *FakePoliciesV1alpha1) ValidatingPolicies() v1alpha1.ValidatingPolicyInterface {
	return &FakeValidatingPolicies{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakePoliciesV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
