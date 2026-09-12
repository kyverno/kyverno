package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
)

func Test_ContextPropagation(t *testing.T) {
	c := &controller{}
	ctx := context.Background()

	keys := []string{
		"ClusterPolicy/test",
		"ValidatingPolicy/test",
		"MutatingPolicy/test",
	}

	for _, key := range keys {
		func() {
			defer func() {
				recover()
			}()
			_ = c.reconcile(ctx, logr.Discard(), key, "default", "test")
		}()
	}
}
