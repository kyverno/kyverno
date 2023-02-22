package generate

import (
	"fmt"
	"strconv"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
)

func increaseRetryAnnotation(ur *kyvernov1beta1.UpdateRequest) (int, map[string]string, error) {
	urAnnotations := ur.Annotations
	if len(urAnnotations) == 0 {
		urAnnotations = map[string]string{
			urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]: "1",
		}
	}

	retry := 1
	val, ok := urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]
	if !ok {
		urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = "1"
	} else {
		retryUint, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return retry, urAnnotations, fmt.Errorf("unable to convert retry-count %v: %w", val, err)
		}
		retry = int(retryUint)
		retry += 1
		incrementedRetryString := strconv.Itoa(retry)
		urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = incrementedRetryString
	}

	return retry, urAnnotations, nil
}

func TriggerFromLabels(labels map[string]string) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		Kind:       labels[common.GenerateTriggerKindLabel],
		Namespace:  labels[common.GenerateTriggerNSLabel],
		Name:       labels[common.GenerateTriggerNameLabel],
		APIVersion: labels[common.GenerateTriggerAPIVersionLabel],
	}
}
