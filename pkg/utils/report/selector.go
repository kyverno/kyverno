package report

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

func SelectorResourceUidEquals(uid types.UID) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(LabelResourceUid, selection.Equals, []string{string(uid)})
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}

func SelectorPolicyDoesNotExist(policy engineapi.GenericPolicy) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.DoesNotExist, nil)
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}

func SelectorPolicyExists(policy engineapi.GenericPolicy) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.Exists, nil)
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}

func SelectorPolicyNotEquals(policy engineapi.GenericPolicy) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.NotEquals, []string{policy.GetResourceVersion()})
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}
