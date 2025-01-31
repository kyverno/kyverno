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

package v2alpha1

import (
	v2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// CELPolicyExceptionLister helps list CELPolicyExceptions.
// All objects returned here must be treated as read-only.
type CELPolicyExceptionLister interface {
	// List lists all CELPolicyExceptions in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2alpha1.CELPolicyException, err error)
	// CELPolicyExceptions returns an object that can list and get CELPolicyExceptions.
	CELPolicyExceptions(namespace string) CELPolicyExceptionNamespaceLister
	CELPolicyExceptionListerExpansion
}

// cELPolicyExceptionLister implements the CELPolicyExceptionLister interface.
type cELPolicyExceptionLister struct {
	indexer cache.Indexer
}

// NewCELPolicyExceptionLister returns a new CELPolicyExceptionLister.
func NewCELPolicyExceptionLister(indexer cache.Indexer) CELPolicyExceptionLister {
	return &cELPolicyExceptionLister{indexer: indexer}
}

// List lists all CELPolicyExceptions in the indexer.
func (s *cELPolicyExceptionLister) List(selector labels.Selector) (ret []*v2alpha1.CELPolicyException, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v2alpha1.CELPolicyException))
	})
	return ret, err
}

// CELPolicyExceptions returns an object that can list and get CELPolicyExceptions.
func (s *cELPolicyExceptionLister) CELPolicyExceptions(namespace string) CELPolicyExceptionNamespaceLister {
	return cELPolicyExceptionNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CELPolicyExceptionNamespaceLister helps list and get CELPolicyExceptions.
// All objects returned here must be treated as read-only.
type CELPolicyExceptionNamespaceLister interface {
	// List lists all CELPolicyExceptions in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2alpha1.CELPolicyException, err error)
	// Get retrieves the CELPolicyException from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v2alpha1.CELPolicyException, error)
	CELPolicyExceptionNamespaceListerExpansion
}

// cELPolicyExceptionNamespaceLister implements the CELPolicyExceptionNamespaceLister
// interface.
type cELPolicyExceptionNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CELPolicyExceptions in the indexer for a given namespace.
func (s cELPolicyExceptionNamespaceLister) List(selector labels.Selector) (ret []*v2alpha1.CELPolicyException, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v2alpha1.CELPolicyException))
	})
	return ret, err
}

// Get retrieves the CELPolicyException from the indexer for a given namespace and name.
func (s cELPolicyExceptionNamespaceLister) Get(name string) (*v2alpha1.CELPolicyException, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v2alpha1.Resource("celpolicyexception"), name)
	}
	return obj.(*v2alpha1.CELPolicyException), nil
}
