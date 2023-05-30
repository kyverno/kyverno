package checker

import (
	"context"
)

func Check(ctx context.Context, checker AuthChecker, group, version, resource, subresource, namespace string, verbs ...string) (bool, error) {
	for _, verb := range verbs {
		result, err := checker.Check(ctx, group, version, resource, subresource, namespace, verb)
		if err != nil {
			return false, err
		}
		if !result.Allowed {
			return false, nil
		}
	}
	return true, nil
}
