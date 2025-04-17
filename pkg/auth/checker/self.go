package checker

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type self struct {
	client authorizationv1client.SelfSubjectAccessReviewInterface
}

func (c self) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error) {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:       group,
				Version:     version,
				Resource:    resource,
				Subresource: subresource,
				Namespace:   namespace,
				Verb:        verb,
				Name:        name,
			},
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
