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

// FakeClusterBackgroundScanReports implements ClusterBackgroundScanReportInterface
type FakeClusterBackgroundScanReports struct {
	Fake *FakeReportsV1
}

var clusterbackgroundscanreportsResource = v1.SchemeGroupVersion.WithResource("clusterbackgroundscanreports")

var clusterbackgroundscanreportsKind = v1.SchemeGroupVersion.WithKind("ClusterBackgroundScanReport")

// Get takes name of the clusterBackgroundScanReport, and returns the corresponding clusterBackgroundScanReport object, and an error if there is any.
func (c *FakeClusterBackgroundScanReports) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ClusterBackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterbackgroundscanreportsResource, name), &v1.ClusterBackgroundScanReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterBackgroundScanReport), err
}

// List takes label and field selectors, and returns the list of ClusterBackgroundScanReports that match those selectors.
func (c *FakeClusterBackgroundScanReports) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ClusterBackgroundScanReportList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterbackgroundscanreportsResource, clusterbackgroundscanreportsKind, opts), &v1.ClusterBackgroundScanReportList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterBackgroundScanReportList{ListMeta: obj.(*v1.ClusterBackgroundScanReportList).ListMeta}
	for _, item := range obj.(*v1.ClusterBackgroundScanReportList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterBackgroundScanReports.
func (c *FakeClusterBackgroundScanReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterbackgroundscanreportsResource, opts))
}

// Create takes the representation of a clusterBackgroundScanReport and creates it.  Returns the server's representation of the clusterBackgroundScanReport, and an error, if there is any.
func (c *FakeClusterBackgroundScanReports) Create(ctx context.Context, clusterBackgroundScanReport *v1.ClusterBackgroundScanReport, opts metav1.CreateOptions) (result *v1.ClusterBackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterbackgroundscanreportsResource, clusterBackgroundScanReport), &v1.ClusterBackgroundScanReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterBackgroundScanReport), err
}

// Update takes the representation of a clusterBackgroundScanReport and updates it. Returns the server's representation of the clusterBackgroundScanReport, and an error, if there is any.
func (c *FakeClusterBackgroundScanReports) Update(ctx context.Context, clusterBackgroundScanReport *v1.ClusterBackgroundScanReport, opts metav1.UpdateOptions) (result *v1.ClusterBackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterbackgroundscanreportsResource, clusterBackgroundScanReport), &v1.ClusterBackgroundScanReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterBackgroundScanReport), err
}

// Delete takes name of the clusterBackgroundScanReport and deletes it. Returns an error if one occurs.
func (c *FakeClusterBackgroundScanReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusterbackgroundscanreportsResource, name, opts), &v1.ClusterBackgroundScanReport{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterBackgroundScanReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterbackgroundscanreportsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1.ClusterBackgroundScanReportList{})
	return err
}

// Patch applies the patch and returns the patched clusterBackgroundScanReport.
func (c *FakeClusterBackgroundScanReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterBackgroundScanReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterbackgroundscanreportsResource, name, pt, data, subresources...), &v1.ClusterBackgroundScanReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterBackgroundScanReport), err
}
