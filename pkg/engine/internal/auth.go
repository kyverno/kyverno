package internal

import (
	"context"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Authorizer implements authorizer.Authorizer interface. It is intended to be used in validate.cel subrules.
type Authorizer struct {
	client       engineapi.Client
	resourceKind schema.GroupVersionKind
}

func (a *Authorizer) Authorize(ctx context.Context, attributes authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	ok, reason, err := a.client.CanI(ctx,
		a.resourceKind.Kind,
		attributes.GetNamespace(),
		attributes.GetVerb(),
		attributes.GetSubresource(),
		attributes.GetUser().GetName(),
	)
	if err != nil {
		return authorizer.DecisionDeny, reason, err
	}

	if ok {
		return authorizer.DecisionAllow, reason, nil
	} else {
		return authorizer.DecisionDeny, reason, nil
	}
}

func NewAuthorizer(client engineapi.Client, resourceKind schema.GroupVersionKind) Authorizer {
	return Authorizer{
		client:       client,
		resourceKind: resourceKind,
	}
}
