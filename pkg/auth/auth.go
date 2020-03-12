package auth

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//CanIOptions provides utility ti check if user has authorization for the given operation
type CanIOptions struct {
	namespace string
	verb      string
	kind      string
	client    *client.Client
}

//NewCanI returns a new instance of operation access controler evaluator
func NewCanI(client *client.Client, kind, namespace, verb string) *CanIOptions {
	o := CanIOptions{
		client: client,
	}

	o.namespace = namespace
	o.kind = kind
	o.verb = verb

	return &o
}

//RunAccessCheck checks if the caller can perform the operation
// - operation is a combination of namespace, kind, verb
// - can only evaluate a single verb
// - group version resource is determined from the kind using the discovery client REST mapper
// - If disallowed, the reason and evaluationError is avialable in the logs
// - each can generates a SelfSubjectAccessReview resource and response is evaluated for permissions
func (o *CanIOptions) RunAccessCheck() (bool, error) {
	// get GroupVersionResource from RESTMapper
	// get GVR from kind
	gvr := o.client.DiscoveryClient.GetGVRFromKind(o.kind)
	if reflect.DeepEqual(gvr, schema.GroupVersionResource{}) {
		// cannot find GVR
		return false, fmt.Errorf("failed to get the Group Version Resource for kind %s", o.kind)
	}

	var sar *authorizationv1.SelfSubjectAccessReview

	sar = &authorizationv1.SelfSubjectAccessReview{
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

	// Create the Resource
	resp, err := o.client.CreateResource("SelfSubjectAccessReview", "", sar, false)
	if err != nil {
		glog.Errorf("failed to create resource %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
		return false, err
	}

	// status.allowed
	allowed, ok, err := unstructured.NestedBool(resp.Object, "status", "allowed")
	if !ok {
		if err != nil {
			glog.Errorf("unexpected error when getting status.allowed for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
		}
		glog.Errorf("status.allowed not found for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
	}

	if !allowed {
		// status.reason
		reason, ok, err := unstructured.NestedString(resp.Object, "status", "reason")
		if !ok {
			if err != nil {
				glog.Errorf("unexpected error when getting status.reason for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
			}
			glog.Errorf("status.reason not found for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
		}
		// status.evaluationError
		evaluationError, ok, err := unstructured.NestedString(resp.Object, "status", "evaludationError")
		if !ok {
			if err != nil {
				glog.Errorf("unexpected error when getting status.evaluationError for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
			}
			glog.Errorf("status.evaluationError not found for %s/%s/%s", sar.Kind, sar.Namespace, sar.Name)
		}

		// Reporting ? (just logs)
		glog.Errorf("reason to disallow operation: %s", reason)
		glog.Errorf("evaluationError to disallow operation: %s", evaluationError)
	}

	return allowed, nil
}
