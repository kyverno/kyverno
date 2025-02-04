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
	"context"

	v2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeImageVerificationPolicies implements ImageVerificationPolicyInterface
type FakeImageVerificationPolicies struct {
	Fake *FakeKyvernoV2alpha1
}

var imageverificationpoliciesResource = v2alpha1.SchemeGroupVersion.WithResource("imageverificationpolicies")

var imageverificationpoliciesKind = v2alpha1.SchemeGroupVersion.WithKind("ImageVerificationPolicy")

// Get takes name of the imageVerificationPolicy, and returns the corresponding imageVerificationPolicy object, and an error if there is any.
func (c *FakeImageVerificationPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.ImageVerificationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(imageverificationpoliciesResource, name), &v2alpha1.ImageVerificationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ImageVerificationPolicy), err
}

// List takes label and field selectors, and returns the list of ImageVerificationPolicies that match those selectors.
func (c *FakeImageVerificationPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.ImageVerificationPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(imageverificationpoliciesResource, imageverificationpoliciesKind, opts), &v2alpha1.ImageVerificationPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.ImageVerificationPolicyList{ListMeta: obj.(*v2alpha1.ImageVerificationPolicyList).ListMeta}
	for _, item := range obj.(*v2alpha1.ImageVerificationPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageVerificationPolicies.
func (c *FakeImageVerificationPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(imageverificationpoliciesResource, opts))
}

// Create takes the representation of a imageVerificationPolicy and creates it.  Returns the server's representation of the imageVerificationPolicy, and an error, if there is any.
func (c *FakeImageVerificationPolicies) Create(ctx context.Context, imageVerificationPolicy *v2alpha1.ImageVerificationPolicy, opts v1.CreateOptions) (result *v2alpha1.ImageVerificationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imageverificationpoliciesResource, imageVerificationPolicy), &v2alpha1.ImageVerificationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ImageVerificationPolicy), err
}

// Update takes the representation of a imageVerificationPolicy and updates it. Returns the server's representation of the imageVerificationPolicy, and an error, if there is any.
func (c *FakeImageVerificationPolicies) Update(ctx context.Context, imageVerificationPolicy *v2alpha1.ImageVerificationPolicy, opts v1.UpdateOptions) (result *v2alpha1.ImageVerificationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(imageverificationpoliciesResource, imageVerificationPolicy), &v2alpha1.ImageVerificationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ImageVerificationPolicy), err
}

// Delete takes name of the imageVerificationPolicy and deletes it. Returns an error if one occurs.
func (c *FakeImageVerificationPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(imageverificationpoliciesResource, name, opts), &v2alpha1.ImageVerificationPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeImageVerificationPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(imageverificationpoliciesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.ImageVerificationPolicyList{})
	return err
}

// Patch applies the patch and returns the patched imageVerificationPolicy.
func (c *FakeImageVerificationPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.ImageVerificationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(imageverificationpoliciesResource, name, pt, data, subresources...), &v2alpha1.ImageVerificationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ImageVerificationPolicy), err
}
