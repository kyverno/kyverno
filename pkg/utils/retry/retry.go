package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
)

// RetryFunc allows retrying a function on error within a given timeout
func RetryFunc(ctx context.Context, retryInterval, timeout time.Duration, logger logr.Logger, msg string, run func(context.Context) error) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		registerTicker := time.NewTicker(retryInterval)
		defer registerTicker.Stop()
		var err error
		for {
			select {
			case <-registerTicker.C:
				if err = run(ctx); err != nil {
					logger.V(3).Info(msg, "reason", err.Error())
				} else {
					return nil
				}
			case <-ctx.Done():
				return fmt.Errorf("retry times out: %w", err)
			}
		}
	}
}
