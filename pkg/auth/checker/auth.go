package checker

import (
	"context"

	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// AuthResult contains authorization check result
type AuthResult struct {
	Allowed         bool
	Reason          string
	EvaluationError string
}

// AuthChecker provides utility to check authorization
type AuthChecker interface {
	// Check checks if the caller can perform an operation
	Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error)
}

func NewSelfChecker(client authorizationv1client.SelfSubjectAccessReviewInterface) AuthChecker {
	return self{
		client: client,
	}
}

func NewSubjectChecker(client authorizationv1client.SubjectAccessReviewInterface, user string, groups []string) AuthChecker {
	return subject{
		client: client,
		user:   user,
		groups: groups,
	}
}
