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
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ValidatingWebhookConfigurationLister helps list ValidatingWebhookConfigurations.
// All objects returned here must be treated as read-only.
type ValidatingWebhookConfigurationLister interface {
	// List lists all ValidatingWebhookConfigurations in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.ValidatingWebhookConfiguration, err error)
	// Get retrieves the ValidatingWebhookConfiguration from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.ValidatingWebhookConfiguration, error)
	ValidatingWebhookConfigurationListerExpansion
}

// validatingWebhookConfigurationLister implements the ValidatingWebhookConfigurationLister interface.
type validatingWebhookConfigurationLister struct {
	indexer cache.Indexer
}

// NewValidatingWebhookConfigurationLister returns a new ValidatingWebhookConfigurationLister.
func NewValidatingWebhookConfigurationLister(indexer cache.Indexer) ValidatingWebhookConfigurationLister {
	return &validatingWebhookConfigurationLister{indexer: indexer}
}

// List lists all ValidatingWebhookConfigurations in the indexer.
func (s *validatingWebhookConfigurationLister) List(selector labels.Selector) (ret []*v1.ValidatingWebhookConfiguration, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.ValidatingWebhookConfiguration))
	})
	return ret, err
}

// Get retrieves the ValidatingWebhookConfiguration from the index for a given name.
func (s *validatingWebhookConfigurationLister) Get(name string) (*v1.ValidatingWebhookConfiguration, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("validatingwebhookconfiguration"), name)
	}
	return obj.(*v1.ValidatingWebhookConfiguration), nil
}
