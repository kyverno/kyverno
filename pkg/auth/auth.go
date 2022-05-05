package auth

import (
	"fmt"
	"reflect"

	client "github.com/kyverno/kyverno/pkg/dclient"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//CanIOptions provides utility to check if user has authorization for the given operation
type CanIOptions struct {
	namespace string
	verb      string
	kind      string
	client    client.Interface
}

//NewCanI returns a new instance of operation access controller evaluator
func NewCanI(client client.Interface, kind, namespace, verb string) *CanIOptions {
	return &CanIOptions{
		namespace: namespace,
		kind:      kind,
		verb:      verb,
		client:    client,
	}
}

//RunAccessCheck checks if the caller can perform the operation
// - operation is a combination of namespace, kind, verb
// - can only evaluate a single verb
// - group version resource is determined from the kind using the discovery client REST mapper
// - If disallowed, the reason and evaluationError is available in the logs
// - each can generates a SelfSubjectAccessReview resource and response is evaluated for permissions
func (o *CanIOptions) RunAccessCheck() (bool, error) {
	// get GroupVersionResource from RESTMapper
	// get GVR from kind
	gvr, err := o.client.Discovery().GetGVRFromKind(o.kind)
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
				Namespace: o.namespace,
				Verb:      o.verb,
				Group:     gvr.Group,
				Resource:  gvr.Resource,
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
	resp, err := o.client.CreateResource("", "SelfSubjectAccessReview", "", sar, false)
	if err != nil {
		logger.Error(err, "failed to create resource")
		return false, err
	}

	// status.allowed
	allowed, ok, err := unstructured.NestedBool(resp.Object, "status", "allowed")
	if !ok {
		if err != nil {
			logger.Error(err, "failed to get the field", "field", "status.allowed")
		}
		logger.Info("field not found", "field", "status.allowed")
	}

	if !allowed {
		// status.reason
		reason, ok, err := unstructured.NestedString(resp.Object, "status", "reason")
		if !ok {
			if err != nil {
				logger.Error(err, "failed to get the field", "field", "status.reason")
			}
			logger.Info("field not found", "field", "status.reason")
		}
		// status.evaluationError
		evaluationError, ok, err := unstructured.NestedString(resp.Object, "status", "evaluationError")
		if !ok {
			if err != nil {
				logger.Error(err, "failed to get the field", "field", "status.evaluationError")
			}
			logger.Info("field not found", "field", "status.evaluationError")
		}

		// Reporting ? (just logs)
		logger.Info("disallowed operation", "reason", reason, "evaluationError", evaluationError)
	}

	return allowed, nil
}
