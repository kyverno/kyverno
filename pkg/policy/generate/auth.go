package generate

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

// Operations provides methods to performing operations on resource
type Operations interface {
	// CanICreate returns 'true' if self can 'create' resource
	CanICreate(ctx context.Context, gvk, namespace, subresource string) (bool, error)
	// CanIUpdate returns 'true' if self can 'update' resource
	CanIUpdate(ctx context.Context, gvk, namespace, subresource string) (bool, error)
	// CanIDelete returns 'true' if self can 'delete' resource
	CanIDelete(ctx context.Context, gvk, namespace, subresource string) (bool, error)
	// CanIGet returns 'true' if self can 'get' resource
	CanIGet(ctx context.Context, gvk, namespace, subresource string) (bool, error)
}

// Auth provides implementation to check if caller/self/kyverno has access to perofrm operations
type Auth struct {
	client dclient.Interface
	user   string
	log    logr.Logger
}

// NewAuth returns a new instance of Auth for operations
func NewAuth(client dclient.Interface, user string, log logr.Logger) *Auth {
	a := Auth{
		client: client,
		user:   user,
		log:    log,
	}
	return &a
}

// CanICreate returns 'true' if self can 'create' resource
func (a *Auth) CanICreate(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "create", "", a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// CanIUpdate returns 'true' if self can 'update' resource
func (a *Auth) CanIUpdate(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "update", "", a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// CanIDelete returns 'true' if self can 'delete' resource
func (a *Auth) CanIDelete(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "delete", "", a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// CanIGet returns 'true' if self can 'get' resource
func (a *Auth) CanIGet(ctx context.Context, gvk, namespace, subresource string) (bool, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), gvk, namespace, "get", "", a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}
