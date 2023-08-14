package internal

import (
	"context"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Authorizer implements authorizer.Authorizer interface
type Authorizer struct {
	Client       engineapi.Client
	ResourceKind schema.GroupVersionKind
}

func (a Authorizer) Authorize(ctx context.Context, attributes authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	ok, err := a.Client.CanI(ctx,
		a.ResourceKind.Kind,
		attributes.GetNamespace(),
		attributes.GetVerb(),
		attributes.GetSubresource(),
		attributes.GetUser().GetName(),
	)
	if err != nil {
		return authorizer.DecisionDeny, "", err
	}

	if ok {
		return authorizer.DecisionAllow, "", nil
	} else {
		return authorizer.DecisionDeny, "", nil
	}
}

// User implements user.Info interface
type User struct {
	Name   string
	UID    string
	Groups []string
	Extra  map[string][]string
}

func (u User) GetName() string {
	return u.Name
}

func (u User) GetUID() string {
	return u.UID
}

func (u User) GetGroups() []string {
	return u.Groups
}

func (u User) GetExtra() map[string][]string {
	return u.Extra
}
