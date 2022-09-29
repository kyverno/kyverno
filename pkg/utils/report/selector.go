package report

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

func SelectorPolicyDoesNotExist(policy kyvernov1.PolicyInterface) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.DoesNotExist, nil)
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}

func SelectorPolicyExists(policy kyvernov1.PolicyInterface) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.Exists, nil)
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}

func SelectorPolicyNotEquals(policy kyvernov1.PolicyInterface) (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(PolicyLabel(policy), selection.NotEquals, []string{policy.GetResourceVersion()})
	if err == nil {
		selector = selector.Add(*requirement)
	}
	return selector, err
}
