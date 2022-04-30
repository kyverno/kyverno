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
	v1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PolicyReportLister helps list PolicyReports.
// All objects returned here must be treated as read-only.
type PolicyReportLister interface {
	// List lists all PolicyReports in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha2.PolicyReport, err error)
	// PolicyReports returns an object that can list and get PolicyReports.
	PolicyReports(namespace string) PolicyReportNamespaceLister
	PolicyReportListerExpansion
}

// policyReportLister implements the PolicyReportLister interface.
type policyReportLister struct {
	indexer cache.Indexer
}

// NewPolicyReportLister returns a new PolicyReportLister.
func NewPolicyReportLister(indexer cache.Indexer) PolicyReportLister {
	return &policyReportLister{indexer: indexer}
}

// List lists all PolicyReports in the indexer.
func (s *policyReportLister) List(selector labels.Selector) (ret []*v1alpha2.PolicyReport, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.PolicyReport))
	})
	return ret, err
}

// PolicyReports returns an object that can list and get PolicyReports.
func (s *policyReportLister) PolicyReports(namespace string) PolicyReportNamespaceLister {
	return policyReportNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PolicyReportNamespaceLister helps list and get PolicyReports.
// All objects returned here must be treated as read-only.
type PolicyReportNamespaceLister interface {
	// List lists all PolicyReports in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha2.PolicyReport, err error)
	// Get retrieves the PolicyReport from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha2.PolicyReport, error)
	PolicyReportNamespaceListerExpansion
}

// policyReportNamespaceLister implements the PolicyReportNamespaceLister
// interface.
type policyReportNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PolicyReports in the indexer for a given namespace.
func (s policyReportNamespaceLister) List(selector labels.Selector) (ret []*v1alpha2.PolicyReport, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.PolicyReport))
	})
	return ret, err
}

// Get retrieves the PolicyReport from the indexer for a given namespace and name.
func (s policyReportNamespaceLister) Get(name string) (*v1alpha2.PolicyReport, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha2.Resource("policyreport"), name)
	}
	return obj.(*v1alpha2.PolicyReport), nil
}
