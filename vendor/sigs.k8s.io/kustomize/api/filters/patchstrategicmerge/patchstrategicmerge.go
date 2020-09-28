// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package patchstrategicmerge

import (
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge2"
)

type Filter struct {
	Patch *yaml.RNode
}

var _ kio.Filter = Filter{}

func (pf Filter) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	var result []*yaml.RNode
	for i := range nodes {
		r, err := merge2.Merge(pf.Patch, nodes[i])
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}
