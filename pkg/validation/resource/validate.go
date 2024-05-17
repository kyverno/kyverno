package resource

import (
	"context"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
)

func ValidateTtlLabel(_ context.Context, object metav1.Object) error {
	labels := object.GetLabels()
	if labels == nil {
		return nil
	}
	if ttl, ok := labels[kyverno.LabelCleanupTtl]; !ok {
		return nil
	} else {
		_, err := strfmt.ParseDuration(ttl)
		if err != nil {
			// Try parsing ttlValue as a time in ISO 8601 format
			_, err := time.Parse(kyverno.ValueTtlDateTimeLayout, ttl)
			if err != nil {
				_, err = time.Parse(kyverno.ValueTtlDateLayout, ttl)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
