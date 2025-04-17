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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	policieskyvernoiov1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ImageValidatingPolicyInformer provides access to a shared informer and lister for
// ImageValidatingPolicies.
type ImageValidatingPolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ImageValidatingPolicyLister
}

type imageValidatingPolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewImageValidatingPolicyInformer constructs a new informer for ImageValidatingPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewImageValidatingPolicyInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredImageValidatingPolicyInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredImageValidatingPolicyInformer constructs a new informer for ImageValidatingPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredImageValidatingPolicyInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PoliciesV1alpha1().ImageValidatingPolicies().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PoliciesV1alpha1().ImageValidatingPolicies().Watch(context.TODO(), options)
			},
		},
		&policieskyvernoiov1alpha1.ImageValidatingPolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *imageValidatingPolicyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredImageValidatingPolicyInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *imageValidatingPolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&policieskyvernoiov1alpha1.ImageValidatingPolicy{}, f.defaultInformer)
}

func (f *imageValidatingPolicyInformer) Lister() v1alpha1.ImageValidatingPolicyLister {
	return v1alpha1.NewImageValidatingPolicyLister(f.Informer().GetIndexer())
}
