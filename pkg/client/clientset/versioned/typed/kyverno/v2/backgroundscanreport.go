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

package v2

import (
	"context"
	"time"

	v2 "github.com/kyverno/kyverno/api/kyverno/v2"
	scheme "github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BackgroundScanReportsGetter has a method to return a BackgroundScanReportInterface.
// A group's client should implement this interface.
type BackgroundScanReportsGetter interface {
	BackgroundScanReports(namespace string) BackgroundScanReportInterface
}

// BackgroundScanReportInterface has methods to work with BackgroundScanReport resources.
type BackgroundScanReportInterface interface {
	Create(ctx context.Context, backgroundScanReport *v2.BackgroundScanReport, opts v1.CreateOptions) (*v2.BackgroundScanReport, error)
	Update(ctx context.Context, backgroundScanReport *v2.BackgroundScanReport, opts v1.UpdateOptions) (*v2.BackgroundScanReport, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v2.BackgroundScanReport, error)
	List(ctx context.Context, opts v1.ListOptions) (*v2.BackgroundScanReportList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2.BackgroundScanReport, err error)
	BackgroundScanReportExpansion
}

// backgroundScanReports implements BackgroundScanReportInterface
type backgroundScanReports struct {
	client rest.Interface
	ns     string
}

// newBackgroundScanReports returns a BackgroundScanReports
func newBackgroundScanReports(c *KyvernoV2Client, namespace string) *backgroundScanReports {
	return &backgroundScanReports{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the backgroundScanReport, and returns the corresponding backgroundScanReport object, and an error if there is any.
func (c *backgroundScanReports) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2.BackgroundScanReport, err error) {
	result = &v2.BackgroundScanReport{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of BackgroundScanReports that match those selectors.
func (c *backgroundScanReports) List(ctx context.Context, opts v1.ListOptions) (result *v2.BackgroundScanReportList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v2.BackgroundScanReportList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested backgroundScanReports.
func (c *backgroundScanReports) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a backgroundScanReport and creates it.  Returns the server's representation of the backgroundScanReport, and an error, if there is any.
func (c *backgroundScanReports) Create(ctx context.Context, backgroundScanReport *v2.BackgroundScanReport, opts v1.CreateOptions) (result *v2.BackgroundScanReport, err error) {
	result = &v2.BackgroundScanReport{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(backgroundScanReport).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a backgroundScanReport and updates it. Returns the server's representation of the backgroundScanReport, and an error, if there is any.
func (c *backgroundScanReports) Update(ctx context.Context, backgroundScanReport *v2.BackgroundScanReport, opts v1.UpdateOptions) (result *v2.BackgroundScanReport, err error) {
	result = &v2.BackgroundScanReport{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		Name(backgroundScanReport.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(backgroundScanReport).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the backgroundScanReport and deletes it. Returns an error if one occurs.
func (c *backgroundScanReports) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *backgroundScanReports) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("backgroundscanreports").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched backgroundScanReport.
func (c *backgroundScanReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2.BackgroundScanReport, err error) {
	result = &v2.BackgroundScanReport{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("backgroundscanreports").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
