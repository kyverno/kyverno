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

	v1 "github.com/kyverno/kyverno/api/reports/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeEphemeralReports implements EphemeralReportInterface
type FakeEphemeralReports struct {
	Fake *FakeReportsV1
	ns   string
}

var ephemeralreportsResource = v1.SchemeGroupVersion.WithResource("ephemeralreports")

var ephemeralreportsKind = v1.SchemeGroupVersion.WithKind("EphemeralReport")

// Get takes name of the ephemeralReport, and returns the corresponding ephemeralReport object, and an error if there is any.
func (c *FakeEphemeralReports) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.EphemeralReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(ephemeralreportsResource, c.ns, name), &v1.EphemeralReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.EphemeralReport), err
}

// List takes label and field selectors, and returns the list of EphemeralReports that match those selectors.
func (c *FakeEphemeralReports) List(ctx context.Context, opts metav1.ListOptions) (result *v1.EphemeralReportList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(ephemeralreportsResource, ephemeralreportsKind, c.ns, opts), &v1.EphemeralReportList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.EphemeralReportList{ListMeta: obj.(*v1.EphemeralReportList).ListMeta}
	for _, item := range obj.(*v1.EphemeralReportList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested ephemeralReports.
func (c *FakeEphemeralReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(ephemeralreportsResource, c.ns, opts))

}

// Create takes the representation of a ephemeralReport and creates it.  Returns the server's representation of the ephemeralReport, and an error, if there is any.
func (c *FakeEphemeralReports) Create(ctx context.Context, ephemeralReport *v1.EphemeralReport, opts metav1.CreateOptions) (result *v1.EphemeralReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(ephemeralreportsResource, c.ns, ephemeralReport), &v1.EphemeralReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.EphemeralReport), err
}

// Update takes the representation of a ephemeralReport and updates it. Returns the server's representation of the ephemeralReport, and an error, if there is any.
func (c *FakeEphemeralReports) Update(ctx context.Context, ephemeralReport *v1.EphemeralReport, opts metav1.UpdateOptions) (result *v1.EphemeralReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(ephemeralreportsResource, c.ns, ephemeralReport), &v1.EphemeralReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.EphemeralReport), err
}

// Delete takes name of the ephemeralReport and deletes it. Returns an error if one occurs.
func (c *FakeEphemeralReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(ephemeralreportsResource, c.ns, name, opts), &v1.EphemeralReport{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEphemeralReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(ephemeralreportsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.EphemeralReportList{})
	return err
}

// Patch applies the patch and returns the patched ephemeralReport.
func (c *FakeEphemeralReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.EphemeralReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(ephemeralreportsResource, c.ns, name, pt, data, subresources...), &v1.EphemeralReport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.EphemeralReport), err
}
