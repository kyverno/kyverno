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

// GlobalContextEntryLister helps list GlobalContextEntries.
// All objects returned here must be treated as read-only.
type GlobalContextEntryLister interface {
	// List lists all GlobalContextEntries in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2alpha1.GlobalContextEntry, err error)
	// Get retrieves the GlobalContextEntry from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v2alpha1.GlobalContextEntry, error)
	GlobalContextEntryListerExpansion
}

// globalContextEntryLister implements the GlobalContextEntryLister interface.
type globalContextEntryLister struct {
	indexer cache.Indexer
}

// NewGlobalContextEntryLister returns a new GlobalContextEntryLister.
func NewGlobalContextEntryLister(indexer cache.Indexer) GlobalContextEntryLister {
	return &globalContextEntryLister{indexer: indexer}
}

// List lists all GlobalContextEntries in the indexer.
func (s *globalContextEntryLister) List(selector labels.Selector) (ret []*v2alpha1.GlobalContextEntry, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v2alpha1.GlobalContextEntry))
	})
	return ret, err
}

// Get retrieves the GlobalContextEntry from the index for a given name.
func (s *globalContextEntryLister) Get(name string) (*v2alpha1.GlobalContextEntry, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v2alpha1.Resource("globalcontextentry"), name)
	}
	return obj.(*v2alpha1.GlobalContextEntry), nil
}
