package checker

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type subject struct {
	client authorizationv1client.SubjectAccessReviewInterface
	user   string
	groups []string
}

func (c subject) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error) {
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:       group,
				Version:     version,
				Resource:    resource,
				Subresource: subresource,
				Namespace:   namespace,
				Verb:        verb,
				Name:        name,
			},
			User:   c.user,
			Groups: c.groups,
		},
	}
	resp, err := c.client.Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return &AuthResult{
		Allowed:         resp.Status.Allowed,
		Reason:          resp.Status.Reason,
		EvaluationError: resp.Status.EvaluationError,
	}, nil
}
