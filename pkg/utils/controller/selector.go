package controller

import (
	"github.com/kyverno/kyverno/api/kyverno"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func SelectorNotManagedByKyverno() (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(kyverno.LabelAppManagedBy, selection.NotEquals, []string{kyverno.ValueKyvernoApp})
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}
