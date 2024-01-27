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

package v1

import (
	"context"
	time "time"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/kyverno/kyverno/pkg/client/listers/reports/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterBackgroundScanReportInformer provides access to a shared informer and lister for
// ClusterBackgroundScanReports.
type ClusterBackgroundScanReportInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.ClusterBackgroundScanReportLister
}

type clusterBackgroundScanReportInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterBackgroundScanReportInformer constructs a new informer for ClusterBackgroundScanReport type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterBackgroundScanReportInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterBackgroundScanReportInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterBackgroundScanReportInformer constructs a new informer for ClusterBackgroundScanReport type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterBackgroundScanReportInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ReportsV1().ClusterBackgroundScanReports().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ReportsV1().ClusterBackgroundScanReports().Watch(context.TODO(), options)
			},
		},
		&reportsv1.ClusterBackgroundScanReport{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterBackgroundScanReportInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterBackgroundScanReportInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterBackgroundScanReportInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&reportsv1.ClusterBackgroundScanReport{}, f.defaultInformer)
}

func (f *clusterBackgroundScanReportInformer) Lister() v1.ClusterBackgroundScanReportLister {
	return v1.NewClusterBackgroundScanReportLister(f.Informer().GetIndexer())
}
