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

// FakeClusterCleanupPolicies implements ClusterCleanupPolicyInterface
type FakeClusterCleanupPolicies struct {
	Fake *FakeKyvernoV2alpha1
}

var clustercleanuppoliciesResource = v2alpha1.SchemeGroupVersion.WithResource("clustercleanuppolicies")

var clustercleanuppoliciesKind = v2alpha1.SchemeGroupVersion.WithKind("ClusterCleanupPolicy")

// Get takes name of the clusterCleanupPolicy, and returns the corresponding clusterCleanupPolicy object, and an error if there is any.
func (c *FakeClusterCleanupPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.ClusterCleanupPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clustercleanuppoliciesResource, name), &v2alpha1.ClusterCleanupPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ClusterCleanupPolicy), err
}

// List takes label and field selectors, and returns the list of ClusterCleanupPolicies that match those selectors.
func (c *FakeClusterCleanupPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.ClusterCleanupPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clustercleanuppoliciesResource, clustercleanuppoliciesKind, opts), &v2alpha1.ClusterCleanupPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.ClusterCleanupPolicyList{ListMeta: obj.(*v2alpha1.ClusterCleanupPolicyList).ListMeta}
	for _, item := range obj.(*v2alpha1.ClusterCleanupPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterCleanupPolicies.
func (c *FakeClusterCleanupPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clustercleanuppoliciesResource, opts))
}

// Create takes the representation of a clusterCleanupPolicy and creates it.  Returns the server's representation of the clusterCleanupPolicy, and an error, if there is any.
func (c *FakeClusterCleanupPolicies) Create(ctx context.Context, clusterCleanupPolicy *v2alpha1.ClusterCleanupPolicy, opts v1.CreateOptions) (result *v2alpha1.ClusterCleanupPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clustercleanuppoliciesResource, clusterCleanupPolicy), &v2alpha1.ClusterCleanupPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ClusterCleanupPolicy), err
}

// Update takes the representation of a clusterCleanupPolicy and updates it. Returns the server's representation of the clusterCleanupPolicy, and an error, if there is any.
func (c *FakeClusterCleanupPolicies) Update(ctx context.Context, clusterCleanupPolicy *v2alpha1.ClusterCleanupPolicy, opts v1.UpdateOptions) (result *v2alpha1.ClusterCleanupPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clustercleanuppoliciesResource, clusterCleanupPolicy), &v2alpha1.ClusterCleanupPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ClusterCleanupPolicy), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterCleanupPolicies) UpdateStatus(ctx context.Context, clusterCleanupPolicy *v2alpha1.ClusterCleanupPolicy, opts v1.UpdateOptions) (*v2alpha1.ClusterCleanupPolicy, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clustercleanuppoliciesResource, "status", clusterCleanupPolicy), &v2alpha1.ClusterCleanupPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ClusterCleanupPolicy), err
}

// Delete takes name of the clusterCleanupPolicy and deletes it. Returns an error if one occurs.
func (c *FakeClusterCleanupPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clustercleanuppoliciesResource, name, opts), &v2alpha1.ClusterCleanupPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterCleanupPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clustercleanuppoliciesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.ClusterCleanupPolicyList{})
	return err
}

// Patch applies the patch and returns the patched clusterCleanupPolicy.
func (c *FakeClusterCleanupPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.ClusterCleanupPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clustercleanuppoliciesResource, name, pt, data, subresources...), &v2alpha1.ClusterCleanupPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.ClusterCleanupPolicy), err
}
