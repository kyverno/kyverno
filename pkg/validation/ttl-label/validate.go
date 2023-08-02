package ttllabel

import (
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
)

func Validate(ttlValue string) error {
	_, err := time.ParseDuration(ttlValue)
	if err != nil {
		// Try parsing ttlValue as a time in ISO 8601 format
		_, err := time.Parse(kyverno.ValueTtlDateTimeLayout, ttlValue)
		if err != nil {
			_, err = time.Parse(kyverno.ValueTtlDateLayout, ttlValue)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
