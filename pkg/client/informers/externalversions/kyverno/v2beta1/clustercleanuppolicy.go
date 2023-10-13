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

// ClusterCleanupPolicyInformer provides access to a shared informer and lister for
// ClusterCleanupPolicies.
type ClusterCleanupPolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v2beta1.ClusterCleanupPolicyLister
}

type clusterCleanupPolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterCleanupPolicyInformer constructs a new informer for ClusterCleanupPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterCleanupPolicyInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterCleanupPolicyInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterCleanupPolicyInformer constructs a new informer for ClusterCleanupPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterCleanupPolicyInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV2beta1().ClusterCleanupPolicies().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV2beta1().ClusterCleanupPolicies().Watch(context.TODO(), options)
			},
		},
		&kyvernov2beta1.ClusterCleanupPolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterCleanupPolicyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterCleanupPolicyInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterCleanupPolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kyvernov2beta1.ClusterCleanupPolicy{}, f.defaultInformer)
}

func (f *clusterCleanupPolicyInformer) Lister() v2beta1.ClusterCleanupPolicyLister {
	return v2beta1.NewClusterCleanupPolicyLister(f.Informer().GetIndexer())
}
