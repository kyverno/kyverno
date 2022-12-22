package auth

import (
	"context"
	"fmt"
	"reflect"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// Discovery provides interface to mange Kind and GVR mapping
type Discovery interface {
	GetGVRFromKind(kind string) (schema.GroupVersionResource, error)
}

// CanIOptions provides utility to check if user has authorization for the given operation
type CanIOptions interface {
	// RunAccessCheck checks if the caller can perform the operation
	// - operation is a combination of namespace, kind, verb
	// - can only evaluate a single verb
	// - group version resource is determined from the kind using the discovery client REST mapper
	// - If disallowed, the reason and evaluationError is available in the logs
	// - each can generates a SelfSubjectAccessReview resource and response is evaluated for permissions
	RunAccessCheck(context.Context) (bool, error)
}

type canIOptions struct {
	namespace   string
	verb        string
	kind        string
	subresource string
	discovery   Discovery
	ssarClient  authorizationv1client.SelfSubjectAccessReviewInterface
}

// NewCanI returns a new instance of operation access controller evaluator
func NewCanI(discovery Discovery, ssarClient authorizationv1client.SelfSubjectAccessReviewInterface, kind, namespace, verb, subresource string) CanIOptions {
	return &canIOptions{
		namespace:   namespace,
		verb:        verb,
		kind:        kind,
		subresource: subresource,
		discovery:   discovery,
		ssarClient:  ssarClient,
	}
}

// RunAccessCheck checks if the caller can perform the operation
// - operation is a combination of namespace, kind, verb
// - can only evaluate a single verb
// - group version resource is determined from the kind using the discovery client REST mapper
// - If disallowed, the reason and evaluationError is available in the logs
// - each can generates a SelfSubjectAccessReview resource and response is evaluated for permissions
func (o *canIOptions) RunAccessCheck(ctx context.Context) (bool, error) {
	// get GroupVersionResource from RESTMapper
	// get GVR from kind
	gvr, err := o.discovery.GetGVRFromKind(o.kind)
	if err != nil {
		return false, fmt.Errorf("failed to get GVR for kind %s", o.kind)
	}

	if reflect.DeepEqual(gvr, schema.GroupVersionResource{}) {
		// cannot find GVR
		return false, fmt.Errorf("failed to get the Group Version Resource for kind %s", o.kind)
	}

	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   o.namespace,
				Verb:        o.verb,
				Group:       gvr.Group,
				Resource:    gvr.Resource,
				Subresource: o.subresource,
			},
		},
	}
	// Set self subject access review
	// - namespace
	// - verb
	// - resource
	// - subresource
	logger := logger.WithValues("kind", sar.Kind, "namespace", sar.Namespace, "name", sar.Name)

	// Create the Resource
	resp, err := o.ssarClient.Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "failed to create resource")
		return false, err
	}

	if !resp.Status.Allowed {
		reason := resp.Status.Reason
		evaluationError := resp.Status.EvaluationError
		// Reporting ? (just logs)
		logger.Info("disallowed operation", "reason", reason, "evaluationError", evaluationError)
	}

	return resp.Status.Allowed, nil
}
