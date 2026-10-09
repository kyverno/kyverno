package updaterequest

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type policyUIDContextKey struct{}

// WithPolicyUID carries the controller-resolved policy identity into asynchronous
// UpdateRequest creation. It is internal context, never resource-supplied data.
func WithPolicyUID(ctx context.Context, uid types.UID) context.Context {
	return context.WithValue(ctx, policyUIDContextKey{}, uid)
}

func policyUIDFromContext(ctx context.Context) types.UID {
	uid, _ := ctx.Value(policyUIDContextKey{}).(types.UID)
	return uid
}
