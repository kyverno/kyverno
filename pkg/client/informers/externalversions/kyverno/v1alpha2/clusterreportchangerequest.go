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

package v1alpha2

import (
	"context"
	time "time"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterReportChangeRequestInformer provides access to a shared informer and lister for
// ClusterReportChangeRequests.
type ClusterReportChangeRequestInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.ClusterReportChangeRequestLister
}

type clusterReportChangeRequestInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterReportChangeRequestInformer constructs a new informer for ClusterReportChangeRequest type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterReportChangeRequestInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterReportChangeRequestInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterReportChangeRequestInformer constructs a new informer for ClusterReportChangeRequest type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterReportChangeRequestInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV1alpha2().ClusterReportChangeRequests().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV1alpha2().ClusterReportChangeRequests().Watch(context.TODO(), options)
			},
		},
		&kyvernov1alpha2.ClusterReportChangeRequest{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterReportChangeRequestInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterReportChangeRequestInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterReportChangeRequestInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kyvernov1alpha2.ClusterReportChangeRequest{}, f.defaultInformer)
}

func (f *clusterReportChangeRequestInformer) Lister() v1alpha2.ClusterReportChangeRequestLister {
	return v1alpha2.NewClusterReportChangeRequestLister(f.Informer().GetIndexer())
}
