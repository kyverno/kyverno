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

package v2beta1

import (
	"context"
	time "time"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	v2beta1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// PolicyExceptionInformer provides access to a shared informer and lister for
// PolicyExceptions.
type PolicyExceptionInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v2beta1.PolicyExceptionLister
}

type policyExceptionInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewPolicyExceptionInformer constructs a new informer for PolicyException type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewPolicyExceptionInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredPolicyExceptionInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredPolicyExceptionInformer constructs a new informer for PolicyException type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPolicyExceptionInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV2beta1().PolicyExceptions(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV2beta1().PolicyExceptions(namespace).Watch(context.TODO(), options)
			},
		},
		&kyvernov2beta1.PolicyException{},
		resyncPeriod,
		indexers,
	)
}

func (f *policyExceptionInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPolicyExceptionInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *policyExceptionInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kyvernov2beta1.PolicyException{}, f.defaultInformer)
}

func (f *policyExceptionInformer) Lister() v2beta1.PolicyExceptionLister {
	return v2beta1.NewPolicyExceptionLister(f.Informer().GetIndexer())
}
