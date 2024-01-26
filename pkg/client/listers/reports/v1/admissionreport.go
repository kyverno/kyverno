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

package v1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// AdmissionReportLister helps list AdmissionReports.
// All objects returned here must be treated as read-only.
type AdmissionReportLister interface {
	// List lists all AdmissionReports in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.AdmissionReport, err error)
	// AdmissionReports returns an object that can list and get AdmissionReports.
	AdmissionReports(namespace string) AdmissionReportNamespaceLister
	AdmissionReportListerExpansion
}

// admissionReportLister implements the AdmissionReportLister interface.
type admissionReportLister struct {
	indexer cache.Indexer
}

// NewAdmissionReportLister returns a new AdmissionReportLister.
func NewAdmissionReportLister(indexer cache.Indexer) AdmissionReportLister {
	return &admissionReportLister{indexer: indexer}
}

// List lists all AdmissionReports in the indexer.
func (s *admissionReportLister) List(selector labels.Selector) (ret []*v1.AdmissionReport, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.AdmissionReport))
	})
	return ret, err
}

// AdmissionReports returns an object that can list and get AdmissionReports.
func (s *admissionReportLister) AdmissionReports(namespace string) AdmissionReportNamespaceLister {
	return admissionReportNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// AdmissionReportNamespaceLister helps list and get AdmissionReports.
// All objects returned here must be treated as read-only.
type AdmissionReportNamespaceLister interface {
	// List lists all AdmissionReports in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.AdmissionReport, err error)
	// Get retrieves the AdmissionReport from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.AdmissionReport, error)
	AdmissionReportNamespaceListerExpansion
}

// admissionReportNamespaceLister implements the AdmissionReportNamespaceLister
// interface.
type admissionReportNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all AdmissionReports in the indexer for a given namespace.
func (s admissionReportNamespaceLister) List(selector labels.Selector) (ret []*v1.AdmissionReport, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.AdmissionReport))
	})
	return ret, err
}

// Get retrieves the AdmissionReport from the indexer for a given namespace and name.
func (s admissionReportNamespaceLister) Get(name string) (*v1.AdmissionReport, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("admissionreport"), name)
	}
	return obj.(*v1.AdmissionReport), nil
}
