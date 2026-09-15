package resource

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

func Test_ValidateContextPropagation(t *testing.T) {
	h := &validationHandlers{}
	ctx := context.Background()

	func() {
		defer func() {
			recover()
		}()
		h.Validate(ctx, logr.Discard(), handlers.AdmissionRequest{}, time.Now())
	}()
}
