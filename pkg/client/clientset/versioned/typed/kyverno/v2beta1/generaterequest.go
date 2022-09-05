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

package v2beta1

import (
	"context"
	"time"

	v2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	scheme "github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// GenerateRequestsGetter has a method to return a GenerateRequestInterface.
// A group's client should implement this interface.
type GenerateRequestsGetter interface {
	GenerateRequests(namespace string) GenerateRequestInterface
}

// GenerateRequestInterface has methods to work with GenerateRequest resources.
type GenerateRequestInterface interface {
	Create(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.CreateOptions) (*v2beta1.GenerateRequest, error)
	Update(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.UpdateOptions) (*v2beta1.GenerateRequest, error)
	UpdateStatus(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.UpdateOptions) (*v2beta1.GenerateRequest, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v2beta1.GenerateRequest, error)
	List(ctx context.Context, opts v1.ListOptions) (*v2beta1.GenerateRequestList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.GenerateRequest, err error)
	GenerateRequestExpansion
}

// generateRequests implements GenerateRequestInterface
type generateRequests struct {
	client rest.Interface
	ns     string
}

// newGenerateRequests returns a GenerateRequests
func newGenerateRequests(c *KyvernoV2beta1Client, namespace string) *generateRequests {
	return &generateRequests{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the generateRequest, and returns the corresponding generateRequest object, and an error if there is any.
func (c *generateRequests) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2beta1.GenerateRequest, err error) {
	result = &v2beta1.GenerateRequest{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("generaterequests").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of GenerateRequests that match those selectors.
func (c *generateRequests) List(ctx context.Context, opts v1.ListOptions) (result *v2beta1.GenerateRequestList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v2beta1.GenerateRequestList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("generaterequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested generateRequests.
func (c *generateRequests) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("generaterequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a generateRequest and creates it.  Returns the server's representation of the generateRequest, and an error, if there is any.
func (c *generateRequests) Create(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.CreateOptions) (result *v2beta1.GenerateRequest, err error) {
	result = &v2beta1.GenerateRequest{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("generaterequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(generateRequest).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a generateRequest and updates it. Returns the server's representation of the generateRequest, and an error, if there is any.
func (c *generateRequests) Update(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.UpdateOptions) (result *v2beta1.GenerateRequest, err error) {
	result = &v2beta1.GenerateRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("generaterequests").
		Name(generateRequest.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(generateRequest).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *generateRequests) UpdateStatus(ctx context.Context, generateRequest *v2beta1.GenerateRequest, opts v1.UpdateOptions) (result *v2beta1.GenerateRequest, err error) {
	result = &v2beta1.GenerateRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("generaterequests").
		Name(generateRequest.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(generateRequest).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the generateRequest and deletes it. Returns an error if one occurs.
func (c *generateRequests) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("generaterequests").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *generateRequests) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("generaterequests").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched generateRequest.
func (c *generateRequests) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.GenerateRequest, err error) {
	result = &v2beta1.GenerateRequest{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("generaterequests").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
