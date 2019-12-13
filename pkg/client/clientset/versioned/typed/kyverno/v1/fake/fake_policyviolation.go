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
	kyvernov1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePolicyViolations implements PolicyViolationInterface
type FakePolicyViolations struct {
	Fake *FakeKyvernoV1
	ns   string
}

var policyviolationsResource = schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policyviolations"}

var policyviolationsKind = schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "PolicyViolation"}

// Get takes name of the policyViolation, and returns the corresponding policyViolation object, and an error if there is any.
func (c *FakePolicyViolations) Get(name string, options v1.GetOptions) (result *kyvernov1.PolicyViolation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(policyviolationsResource, c.ns, name), &kyvernov1.PolicyViolation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*kyvernov1.PolicyViolation), err
}

// List takes label and field selectors, and returns the list of PolicyViolations that match those selectors.
func (c *FakePolicyViolations) List(opts v1.ListOptions) (result *kyvernov1.PolicyViolationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(policyviolationsResource, policyviolationsKind, c.ns, opts), &kyvernov1.PolicyViolationList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &kyvernov1.PolicyViolationList{ListMeta: obj.(*kyvernov1.PolicyViolationList).ListMeta}
	for _, item := range obj.(*kyvernov1.PolicyViolationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested policyViolations.
func (c *FakePolicyViolations) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(policyviolationsResource, c.ns, opts))

}

// Create takes the representation of a policyViolation and creates it.  Returns the server's representation of the policyViolation, and an error, if there is any.
func (c *FakePolicyViolations) Create(policyViolation *kyvernov1.PolicyViolation) (result *kyvernov1.PolicyViolation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(policyviolationsResource, c.ns, policyViolation), &kyvernov1.PolicyViolation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*kyvernov1.PolicyViolation), err
}

// Update takes the representation of a policyViolation and updates it. Returns the server's representation of the policyViolation, and an error, if there is any.
func (c *FakePolicyViolations) Update(policyViolation *kyvernov1.PolicyViolation) (result *kyvernov1.PolicyViolation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(policyviolationsResource, c.ns, policyViolation), &kyvernov1.PolicyViolation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*kyvernov1.PolicyViolation), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePolicyViolations) UpdateStatus(policyViolation *kyvernov1.PolicyViolation) (*kyvernov1.PolicyViolation, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(policyviolationsResource, "status", c.ns, policyViolation), &kyvernov1.PolicyViolation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*kyvernov1.PolicyViolation), err
}

// Delete takes name of the policyViolation and deletes it. Returns an error if one occurs.
func (c *FakePolicyViolations) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(policyviolationsResource, c.ns, name), &kyvernov1.PolicyViolation{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePolicyViolations) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(policyviolationsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &kyvernov1.PolicyViolationList{})
	return err
}

// Patch applies the patch and returns the patched policyViolation.
func (c *FakePolicyViolations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *kyvernov1.PolicyViolation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(policyviolationsResource, c.ns, name, pt, data, subresources...), &kyvernov1.PolicyViolation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*kyvernov1.PolicyViolation), err
}
