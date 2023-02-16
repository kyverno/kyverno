package api

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// CleanupPolicyResponse stores statistics for the single policy application
type CleanupPolicyResponse struct {
	// Resource is the deleted resource
	Resource unstructured.Unstructured
	// ExecutionStats policy execution stats
	ExecutionStats
	// Number of deleted objects
	DeletedObjects int
	// Message
	Message string
}

type CleanupResponse struct {
	PolicyResponse CleanupPolicyResponse
}
