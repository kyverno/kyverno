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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha2

import (
	v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ClusterAdmissionReportLister helps list ClusterAdmissionReports.
// All objects returned here must be treated as read-only.
type ClusterAdmissionReportLister interface {
	// List lists all ClusterAdmissionReports in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha2.ClusterAdmissionReport, err error)
	// Get retrieves the ClusterAdmissionReport from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha2.ClusterAdmissionReport, error)
	ClusterAdmissionReportListerExpansion
}

// clusterAdmissionReportLister implements the ClusterAdmissionReportLister interface.
type clusterAdmissionReportLister struct {
	indexer cache.Indexer
}

// NewClusterAdmissionReportLister returns a new ClusterAdmissionReportLister.
func NewClusterAdmissionReportLister(indexer cache.Indexer) ClusterAdmissionReportLister {
	return &clusterAdmissionReportLister{indexer: indexer}
}

// List lists all ClusterAdmissionReports in the indexer.
func (s *clusterAdmissionReportLister) List(selector labels.Selector) (ret []*v1alpha2.ClusterAdmissionReport, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.ClusterAdmissionReport))
	})
	return ret, err
}

// Get retrieves the ClusterAdmissionReport from the index for a given name.
func (s *clusterAdmissionReportLister) Get(name string) (*v1alpha2.ClusterAdmissionReport, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha2.Resource("clusteradmissionreport"), name)
	}
	return obj.(*v1alpha2.ClusterAdmissionReport), nil
}
