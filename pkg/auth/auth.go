package auth

import (
	"context"
	"fmt"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// Discovery provides interface to mange Kind and GVR mapping
type Discovery interface {
	GetGVRFromGVK(schema.GroupVersionKind) (schema.GroupVersionResource, error)
}

// CanIOptions provides utility to check if user has authorization for the given operation
type CanIOptions interface {
	// RunAccessCheck checks if the caller can perform the operation
	// - operation is a combination of namespace, kind, verb
	// - can only evaluate a single verb
	// - group version resource is determined from the kind using the discovery client REST mapper
	// - If disallowed, the reason and evaluationError is available in the logs
	// - each can generates a SubjectAccessReview resource and response is evaluated for permissions
	RunAccessCheck(context.Context) (bool, error)
}

type canIOptions struct {
	namespace   string
	verb        string
	gvk         string
	subresource string
	user        string
	discovery   Discovery
	sarClient   authorizationv1client.SubjectAccessReviewInterface
}

// NewCanI returns a new instance of operation access controller evaluator
func NewCanI(discovery Discovery, sarClient authorizationv1client.SubjectAccessReviewInterface, gvk, namespace, verb, subresource string, user string) CanIOptions {
	return &canIOptions{
		namespace:   namespace,
		verb:        verb,
		gvk:         gvk,
		subresource: subresource,
		user:        user,
		discovery:   discovery,
		sarClient:   sarClient,
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
	apiVersion, kind := kubeutils.GetKindFromGVK(o.gvk)
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse group/version %s", apiVersion)
	}
	gvr, err := o.discovery.GetGVRFromGVK(gv.WithKind(kind))
	if err != nil {
		return false, fmt.Errorf("failed to get GVR for kind %s", o.gvk)
	}

	if gvr.Empty() {
		// cannot find GVR
		return false, fmt.Errorf("failed to get the Group Version Resource for kind %s", o.gvk)
	}

	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   o.namespace,
				Verb:        o.verb,
				Group:       gvr.Group,
				Resource:    gvr.Resource,
				Subresource: o.subresource,
			},
			User: o.user,
		},
	}

	logger := logger.WithValues("kind", sar.Kind, "namespace", sar.Namespace, "name", sar.Name, "gvr", gvr.String())
	resp, err := o.sarClient.Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "failed to create resource")
		return false, err
	}

	if !resp.Status.Allowed {
		reason := resp.Status.Reason
		evaluationError := resp.Status.EvaluationError
		logger.Info("disallowed operation", "reason", reason, "evaluationError", evaluationError)
	}

	return resp.Status.Allowed, nil
}
