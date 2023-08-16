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
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterAdmissionReports implements ClusterAdmissionReportInterface
type FakeClusterAdmissionReports struct {
	Fake *FakeKyvernoV1alpha2
}

var clusteradmissionreportsResource = v1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports")

var clusteradmissionreportsKind = v1alpha2.SchemeGroupVersion.WithKind("ClusterAdmissionReport")

// Get takes name of the clusterAdmissionReport, and returns the corresponding clusterAdmissionReport object, and an error if there is any.
func (c *FakeClusterAdmissionReports) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.ClusterAdmissionReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusteradmissionreportsResource, name), &v1alpha2.ClusterAdmissionReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ClusterAdmissionReport), err
}

// List takes label and field selectors, and returns the list of ClusterAdmissionReports that match those selectors.
func (c *FakeClusterAdmissionReports) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.ClusterAdmissionReportList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusteradmissionreportsResource, clusteradmissionreportsKind, opts), &v1alpha2.ClusterAdmissionReportList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.ClusterAdmissionReportList{ListMeta: obj.(*v1alpha2.ClusterAdmissionReportList).ListMeta}
	for _, item := range obj.(*v1alpha2.ClusterAdmissionReportList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterAdmissionReports.
func (c *FakeClusterAdmissionReports) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusteradmissionreportsResource, opts))
}

// Create takes the representation of a clusterAdmissionReport and creates it.  Returns the server's representation of the clusterAdmissionReport, and an error, if there is any.
func (c *FakeClusterAdmissionReports) Create(ctx context.Context, clusterAdmissionReport *v1alpha2.ClusterAdmissionReport, opts v1.CreateOptions) (result *v1alpha2.ClusterAdmissionReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusteradmissionreportsResource, clusterAdmissionReport), &v1alpha2.ClusterAdmissionReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ClusterAdmissionReport), err
}

// Update takes the representation of a clusterAdmissionReport and updates it. Returns the server's representation of the clusterAdmissionReport, and an error, if there is any.
func (c *FakeClusterAdmissionReports) Update(ctx context.Context, clusterAdmissionReport *v1alpha2.ClusterAdmissionReport, opts v1.UpdateOptions) (result *v1alpha2.ClusterAdmissionReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusteradmissionreportsResource, clusterAdmissionReport), &v1alpha2.ClusterAdmissionReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ClusterAdmissionReport), err
}

// Delete takes name of the clusterAdmissionReport and deletes it. Returns an error if one occurs.
func (c *FakeClusterAdmissionReports) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusteradmissionreportsResource, name, opts), &v1alpha2.ClusterAdmissionReport{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterAdmissionReports) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusteradmissionreportsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha2.ClusterAdmissionReportList{})
	return err
}

// Patch applies the patch and returns the patched clusterAdmissionReport.
func (c *FakeClusterAdmissionReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterAdmissionReport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusteradmissionreportsResource, name, pt, data, subresources...), &v1alpha2.ClusterAdmissionReport{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ClusterAdmissionReport), err
}
