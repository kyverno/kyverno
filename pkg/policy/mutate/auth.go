package mutate

import (
	"context"

	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

type authChecker struct {
	client dclient.Interface
}

type AuthChecker interface {
	CanICreate(ctx context.Context, kind, namespace, subresource string) (bool, error)
	CanIUpdate(ctx context.Context, kind, namespace, subresource string) (bool, error)
	CanIGet(ctx context.Context, kind, namespace, subresource string) (bool, error)
}

func newAuthChecker(client dclient.Interface) AuthChecker {
	return &authChecker{client: client}
}

func (a *authChecker) CanICreate(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SelfSubjectAccessReviews(), kind, namespace, "create", subresource)
	return checker.RunAccessCheck(ctx)
}

func (a *authChecker) CanIUpdate(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SelfSubjectAccessReviews(), kind, namespace, "update", subresource)
	return checker.RunAccessCheck(ctx)
}

func (a *authChecker) CanIGet(ctx context.Context, kind, namespace, subresource string) (bool, error) {
	checker := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SelfSubjectAccessReviews(), kind, namespace, "get", subresource)
	return checker.RunAccessCheck(ctx)
}
