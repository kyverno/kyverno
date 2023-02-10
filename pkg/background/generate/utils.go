package generate

import (
	"fmt"
	"strconv"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
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
