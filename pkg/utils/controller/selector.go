package controller

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func SelectorNotManagedByKyverno() (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(kyvernov1.LabelAppManagedBy, selection.NotEquals, []string{kyvernov1.ValueKyvernoApp})
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}
