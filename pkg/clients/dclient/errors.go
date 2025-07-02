package dclient

import (
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	permanentErrorPatterns = []string{
		"is forbidden",
		"Operation on Calico tiered policy is forbidden",
		"the server could not find the requested resource",
		"no Kind is registered for the type",
	}

	temporaryErrorPatterns = []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadline exceeded",
		"service unavailable",
		"internal server error",
		"server is currently unable to handle the request",
		"too many requests",
	}

	discoveryErrorPatterns = []string{
		"no matches for kind",
		"unable to recognize",
	}
)

func IsRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Kubernetes API errors that are permanent access issues (should skip, not retry)
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		return true
	}

	// Check for resource not found/not available errors that are permanent (should skip, not retry)
	if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
		return true
	}

	// Check for specific error messages that indicate permanent permission or resource type issues
	errStr := err.Error()

	for _, pattern := range permanentErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check for temporary errors that might resolve on retry
	// These patterns indicate potentially temporary issues that should NOT be skipped
	for _, pattern := range temporaryErrorPatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			// These are temporary errors - should NOT be recovered (should trigger retry)
			return false
		}
	}

	// For resource discovery errors, check if they contain specific patterns
	// "no matches for kind" and "unable to recognize" could be temporary during API discovery
	for _, pattern := range discoveryErrorPatterns {
		if strings.Contains(errStr, pattern) {
			// These could be temporary during API server startup or CRD installation
			// However, if they persist, they indicate the resource type doesn't exist
			// For cleanup controller, it's safer to skip these since they likely indicate
			// resource types that don't exist in the cluster
			return true
		}
	}

	// Default: treat unknown errors as non-recoverable (should trigger retry)
	// This ensures that unexpected errors get retried rather than silently ignored
	return false
}
