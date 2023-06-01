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
	CanICreate(ctx context.Context, kind, namespace, subresource string) (bool, error)
	CanIUpdate(ctx context.Context, kind, namespace, subresource string) (bool, error)
	CanIGet(ctx context.Context, kind, namespace, subresource string) (bool, error)
}

func newAuthChecker(client dclient.Interface, user string) AuthChecker {
	return &authChecker{client: client, user: user}
}

func (a *authChecker) CanICreate(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, "create", subresource, a.user)
	return checker.RunAccessCheck(ctx)
}

func (a *authChecker) CanIUpdate(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, "update", subresource, a.user)
	return checker.RunAccessCheck(ctx)
}

func (a *authChecker) CanIGet(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, "get", subresource, a.user)
	return checker.RunAccessCheck(ctx)
}
