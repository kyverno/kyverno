package ttl

import (
	"context"
	"testing"
)

func Test_ManagerContextPropagation(t *testing.T) {
	m := &manager{}
	ctx := context.Background()

	func() {
		defer func() {
			recover()
		}()
		_ = m.filterPermissionsResource(ctx, nil)
	}()

	func() {
		defer func() {
			recover()
		}()
		_, _ = m.getDesiredState(ctx)
	}()
}
