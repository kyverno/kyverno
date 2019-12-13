package backoff

import (
	"context"
	"testing"
	"time"
)

func TestContext(t *testing.T) {
	b := NewConstantBackOff(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cb := WithContext(b, ctx)

	if cb.Context() != ctx {
		t.Error("invalid context")
	}

	cancel()

	if cb.NextBackOff() != Stop {
		t.Error("invalid next back off")
	}
}
