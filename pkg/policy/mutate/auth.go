package mutate

import (
	"context"

	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

type authChecker struct {
	client dclient.Interface
	user   string
}

type AuthChecker interface {
	CanIUpdate(ctx context.Context, gvks, namespace, subresource string) (bool, error)
	CanIGet(ctx context.Context, gvks, namespace, subresource string) (bool, error)
}

func newAuthChecker(client dclient.Interface, user string) AuthChecker {
	return &authChecker{client: client, user: user}
}

func (a *authChecker) CanIUpdate(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "update", subresource, a.user)
	ok, _, err := checker.RunAccessCheck(ctx)
	return ok, err
}

func (a *authChecker) CanIGet(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "get", subresource, a.user)
	ok, _, err := checker.RunAccessCheck(ctx)
	return ok, err
}
