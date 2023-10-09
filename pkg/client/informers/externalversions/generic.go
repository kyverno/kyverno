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

package externalversions

import (
	"fmt"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	v1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	v2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	v2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=kyverno.io, Version=v1
	case v1.SchemeGroupVersion.WithResource("clusterpolicies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1().ClusterPolicies().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("policies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1().Policies().Informer()}, nil

		// Group=kyverno.io, Version=v1alpha2
	case v1alpha2.SchemeGroupVersion.WithResource("admissionreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1alpha2().AdmissionReports().Informer()}, nil
	case v1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1alpha2().BackgroundScanReports().Informer()}, nil
	case v1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1alpha2().ClusterAdmissionReports().Informer()}, nil
	case v1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1alpha2().ClusterBackgroundScanReports().Informer()}, nil

		// Group=kyverno.io, Version=v1beta1
	case v1beta1.SchemeGroupVersion.WithResource("updaterequests"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V1beta1().UpdateRequests().Informer()}, nil

		// Group=kyverno.io, Version=v2alpha1
	case v2alpha1.SchemeGroupVersion.WithResource("cleanuppolicies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2alpha1().CleanupPolicies().Informer()}, nil
	case v2alpha1.SchemeGroupVersion.WithResource("clustercleanuppolicies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2alpha1().ClusterCleanupPolicies().Informer()}, nil
	case v2alpha1.SchemeGroupVersion.WithResource("policyexceptions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2alpha1().PolicyExceptions().Informer()}, nil

		// Group=kyverno.io, Version=v2beta1
	case v2beta1.SchemeGroupVersion.WithResource("clusterpolicies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2beta1().ClusterPolicies().Informer()}, nil
	case v2beta1.SchemeGroupVersion.WithResource("policies"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2beta1().Policies().Informer()}, nil
	case v2beta1.SchemeGroupVersion.WithResource("policyexceptions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Kyverno().V2beta1().PolicyExceptions().Informer()}, nil

		// Group=wgpolicyk8s.io, Version=v1alpha2
	case policyreportv1alpha2.SchemeGroupVersion.WithResource("clusterpolicyreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Wgpolicyk8s().V1alpha2().ClusterPolicyReports().Informer()}, nil
	case policyreportv1alpha2.SchemeGroupVersion.WithResource("policyreports"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Wgpolicyk8s().V1alpha2().PolicyReports().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
