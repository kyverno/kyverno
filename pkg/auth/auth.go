package auth

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
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
	RunAccessCheck(context.Context) (bool, string, error)
}

type canIOptions struct {
	namespace   string
	verb        string
	gvk         string
	subresource string
	user        string
	name        string
	discovery   Discovery
	checker     checker.AuthChecker
}

// NewCanI returns a new instance of operation access controller evaluator
func NewCanI(discovery Discovery, sarClient authorizationv1client.SubjectAccessReviewInterface, gvk, namespace, name, verb, subresource string, user string) CanIOptions {
	return &canIOptions{
		name:        name,
		namespace:   namespace,
		verb:        verb,
		gvk:         gvk,
		subresource: subresource,
		user:        user,
		discovery:   discovery,
		checker:     checker.NewSubjectChecker(sarClient, user, nil),
	}
}

// RunAccessCheck checks if the caller can perform the operation
// - operation is a combination of namespace, kind, verb
// - can only evaluate a single verb
// - group version resource is determined from the kind using the discovery client REST mapper
// - If disallowed, the reason and evaluationError is available in the logs
// - each can generates a SelfSubjectAccessReview resource and response is evaluated for permissions
func (o *canIOptions) RunAccessCheck(ctx context.Context) (bool, string, error) {
	// get GroupVersionResource from RESTMapper
	// get GVR from kind
	apiVersion, kind := kubeutils.GetKindFromGVK(o.gvk)
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse group/version %s", apiVersion)
	}
	gvr, err := o.discovery.GetGVRFromGVK(gv.WithKind(kind))
	if err != nil {
		return false, "", fmt.Errorf("failed to get GVR for kind %s", o.gvk)
	}
	if gvr.Empty() {
		// cannot find GVR
		return false, "", fmt.Errorf("failed to get the Group Version Resource for kind %s", o.gvk)
	}
	logger := logger.WithValues("kind", kind, "namespace", o.namespace, "gvr", gvr.String(), "verb", o.verb)
	result, err := o.checker.Check(ctx, gvr.Group, gvr.Version, gvr.Resource, o.subresource, o.namespace, o.name, o.verb)
	if err != nil {
		logger.Error(err, "failed to check permissions")
		return false, "", err
	}
	if !result.Allowed {
		logger.V(3).Info("disallowed operation", "reason", result.Reason, "evaluationError", result.EvaluationError)
	}
	return result.Allowed, result.Reason, nil
}
