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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	scheme "github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// MutatingPoliciesGetter has a method to return a MutatingPolicyInterface.
// A group's client should implement this interface.
type MutatingPoliciesGetter interface {
	MutatingPolicies() MutatingPolicyInterface
}

// MutatingPolicyInterface has methods to work with MutatingPolicy resources.
type MutatingPolicyInterface interface {
	Create(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.CreateOptions) (*v1alpha1.MutatingPolicy, error)
	Update(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.UpdateOptions) (*v1alpha1.MutatingPolicy, error)
	UpdateStatus(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.UpdateOptions) (*v1alpha1.MutatingPolicy, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.MutatingPolicy, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.MutatingPolicyList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MutatingPolicy, err error)
	MutatingPolicyExpansion
}

// mutatingPolicies implements MutatingPolicyInterface
type mutatingPolicies struct {
	client rest.Interface
}

// newMutatingPolicies returns a MutatingPolicies
func newMutatingPolicies(c *PoliciesV1alpha1Client) *mutatingPolicies {
	return &mutatingPolicies{
		client: c.RESTClient(),
	}
}

// Get takes name of the mutatingPolicy, and returns the corresponding mutatingPolicy object, and an error if there is any.
func (c *mutatingPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.MutatingPolicy, err error) {
	result = &v1alpha1.MutatingPolicy{}
	err = c.client.Get().
		Resource("mutatingpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of MutatingPolicies that match those selectors.
func (c *mutatingPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MutatingPolicyList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.MutatingPolicyList{}
	err = c.client.Get().
		Resource("mutatingpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested mutatingPolicies.
func (c *mutatingPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("mutatingpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a mutatingPolicy and creates it.  Returns the server's representation of the mutatingPolicy, and an error, if there is any.
func (c *mutatingPolicies) Create(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.CreateOptions) (result *v1alpha1.MutatingPolicy, err error) {
	result = &v1alpha1.MutatingPolicy{}
	err = c.client.Post().
		Resource("mutatingpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(mutatingPolicy).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a mutatingPolicy and updates it. Returns the server's representation of the mutatingPolicy, and an error, if there is any.
func (c *mutatingPolicies) Update(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.UpdateOptions) (result *v1alpha1.MutatingPolicy, err error) {
	result = &v1alpha1.MutatingPolicy{}
	err = c.client.Put().
		Resource("mutatingpolicies").
		Name(mutatingPolicy.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(mutatingPolicy).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *mutatingPolicies) UpdateStatus(ctx context.Context, mutatingPolicy *v1alpha1.MutatingPolicy, opts v1.UpdateOptions) (result *v1alpha1.MutatingPolicy, err error) {
	result = &v1alpha1.MutatingPolicy{}
	err = c.client.Put().
		Resource("mutatingpolicies").
		Name(mutatingPolicy.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(mutatingPolicy).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the mutatingPolicy and deletes it. Returns an error if one occurs.
func (c *mutatingPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("mutatingpolicies").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *mutatingPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("mutatingpolicies").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched mutatingPolicy.
func (c *mutatingPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MutatingPolicy, err error) {
	result = &v1alpha1.MutatingPolicy{}
	err = c.client.Patch(pt).
		Resource("mutatingpolicies").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
