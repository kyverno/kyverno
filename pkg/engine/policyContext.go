package engine

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext struct {
	// policy to be processed
	Policy kyverno.ClusterPolicy
	// resource to be processed
	Resource      unstructured.Unstructured
	AdmissionInfo RequestInfo
}

// RequestInfo contains permission info carried in an admission request
type RequestInfo struct {
	// Roles is a list of possible role send the request
	Roles []string
	// ClusterRoles is a list of possible clusterRoles send the request
	ClusterRoles []string
	// UserInfo is the userInfo carried in the admission request
	AdmissionUserInfo authenticationv1.UserInfo
}
