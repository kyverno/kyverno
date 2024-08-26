package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

// AuthChecks provides methods to performing operations on resource
type AuthChecks interface {
	// User returns the subject
	User() string
	// CanI returns 'true' if user has permissions for all specified verbs.
	// When the result is 'false' a message with details on missing verbs is returned.
	CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error)
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

func (a *Auth) User() string {
	return a.user
}

func (a *Auth) CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error) {
	var failedVerbs []string
	for _, v := range verbs {
		if ok, err := a.check(ctx, v, gvk, namespace, name, subresource); err != nil {
			return false, "", err
		} else if !ok {
			failedVerbs = append(failedVerbs, v)
		}
	}

	if len(failedVerbs) > 0 {
		msg := buildMessage(gvk, subresource, failedVerbs, a.user, namespace)
		return false, msg, nil
	}

	return true, "", nil
}

// CanICreate returns 'true' if self can 'create' resource
func (a *Auth) CanICreate(ctx context.Context, gvk, namespace, name, subresource string) (bool, error) {
	return a.check(ctx, "create", gvk, namespace, name, subresource)
}

func (a *Auth) check(ctx context.Context, verb, gvk, namespace, name, subresource string) (bool, error) {
	subjectReview := a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews()
	canI := auth.NewCanI(a.client.Discovery(), subjectReview, gvk, namespace, name, verb, subresource, a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func buildMessage(gvk string, subresource string, failedVerbs []string, user string, namespace string) string {
	resource := gvk
	if subresource != "" {
		resource = gvk + "/" + subresource
	}

	permissions := strings.Join(failedVerbs, ",")
	msg := fmt.Sprintf("%s requires permissions %s for resource %s", user, permissions, resource)
	if namespace != "" {
		msg = fmt.Sprintf("%s in namespace %s", msg, namespace)
	}
	return msg
}
