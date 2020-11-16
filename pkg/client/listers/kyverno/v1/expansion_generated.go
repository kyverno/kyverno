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
	kyvernov1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClusterPolicyListerExpansion allows custom methods to be added to
// ClusterPolicyLister.
type ClusterPolicyListerExpansion interface {
	ListResources(selector labels.Selector) (ret []*kyvernov1.ClusterPolicy, err error)
}

// GenerateRequestListerExpansion allows custom methods to be added to
// GenerateRequestLister.
type GenerateRequestListerExpansion interface{}

// GenerateRequestNamespaceListerExpansion allows custom methods to be added to
// GenerateRequestNamespaceLister.
type GenerateRequestNamespaceListerExpansion interface {
	GetGenerateRequestsForClusterPolicy(policy string) ([]*kyvernov1.GenerateRequest, error)
	GetGenerateRequestsForResource(kind, namespace, name string) ([]*kyvernov1.GenerateRequest, error)
}

// PolicyListerExpansion allows custom methods to be added to
// PolicyLister.
type PolicyListerExpansion interface{}

// PolicyNamespaceListerExpansion allows custom methods to be added to
// PolicyNamespaceLister.
type PolicyNamespaceListerExpansion interface{}
