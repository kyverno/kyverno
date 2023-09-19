package match

import (
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func CheckSelector(expected *metav1.LabelSelector, actual map[string]string) (bool, error) {
	if expected == nil {
		return false, nil
	}
	expected = wildcards.ReplaceInSelector(expected, actual)
	selector, err := metav1.LabelSelectorAsSelector(expected)
	if err != nil {
		logging.Error(err, "failed to build label selector")
		return false, err
	}
	if selector.Matches(labels.Set(actual)) {
		return true, nil
	}
	return false, nil
}
