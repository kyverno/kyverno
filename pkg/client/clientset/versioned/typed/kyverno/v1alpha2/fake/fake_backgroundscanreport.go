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

	v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBackgroundScanReports implements BackgroundScanReportInterface
type FakeBackgroundScanReports struct {
	Fake *FakeKyvernoV1alpha2
	ns   string
}

var backgroundscanreportsResource = schema.GroupVersionResource{Group: "kyverno.io", Version: "v1alpha2", Resource: "backgroundscanreports"}

var backgroundscanreportsKind = schema.GroupVersionKind{Group: "kyverno.io", Version: "v1alpha2", Kind: "BackgroundScanReport"}

// Get takes name of the backgroundScanReport, and returns the corresponding backgroundScanReport object, and an error if there is any.
func (c *FakeBackgroundScanReports) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.BackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(backgroundscanreportsResource, c.ns, name), &v1alpha2.BackgroundScanReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.BackgroundScanReport), err
}

// List takes label and field selectors, and returns the list of BackgroundScanReports that match those selectors.
func (c *FakeBackgroundScanReports) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.BackgroundScanReportList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(backgroundscanreportsResource, backgroundscanreportsKind, c.ns, opts), &v1alpha2.BackgroundScanReportList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.BackgroundScanReportList{ListMeta: obj.(*v1alpha2.BackgroundScanReportList).ListMeta}
	for _, item := range obj.(*v1alpha2.BackgroundScanReportList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested backgroundScanReports.
func (c *FakeBackgroundScanReports) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(backgroundscanreportsResource, c.ns, opts))

}

// Create takes the representation of a backgroundScanReport and creates it.  Returns the server's representation of the backgroundScanReport, and an error, if there is any.
func (c *FakeBackgroundScanReports) Create(ctx context.Context, backgroundScanReport *v1alpha2.BackgroundScanReport, opts v1.CreateOptions) (result *v1alpha2.BackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(backgroundscanreportsResource, c.ns, backgroundScanReport), &v1alpha2.BackgroundScanReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.BackgroundScanReport), err
}

// Update takes the representation of a backgroundScanReport and updates it. Returns the server's representation of the backgroundScanReport, and an error, if there is any.
func (c *FakeBackgroundScanReports) Update(ctx context.Context, backgroundScanReport *v1alpha2.BackgroundScanReport, opts v1.UpdateOptions) (result *v1alpha2.BackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(backgroundscanreportsResource, c.ns, backgroundScanReport), &v1alpha2.BackgroundScanReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.BackgroundScanReport), err
}

// Delete takes name of the backgroundScanReport and deletes it. Returns an error if one occurs.
func (c *FakeBackgroundScanReports) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(backgroundscanreportsResource, c.ns, name, opts), &v1alpha2.BackgroundScanReport{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBackgroundScanReports) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(backgroundscanreportsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha2.BackgroundScanReportList{})
	return err
}

// Patch applies the patch and returns the patched backgroundScanReport.
func (c *FakeBackgroundScanReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.BackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(backgroundscanreportsResource, c.ns, name, pt, data, subresources...), &v1alpha2.BackgroundScanReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.BackgroundScanReport), err
}
