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

package v1beta1

import (
	v1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

func (s updateRequestNamespaceLister) GetUpdateRequestsForClusterPolicy(policy string) ([]*v1beta1.UpdateRequest, error) {
	var list []*v1beta1.UpdateRequest
	urs, err := s.List(labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for idx, ur := range urs {
		if ur.Spec.Policy == policy {
			list = append(list, urs[idx])
		}
	}
	return list, err
}
