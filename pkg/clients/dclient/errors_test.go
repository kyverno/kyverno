package dclient

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		shouldRecover bool
		description   string
	}{
		{
			name:          "nil error",
			err:           nil,
			shouldRecover: false,
			description:   "nil errors should not be recovered",
		},
		{
			name:          "forbidden error",
			err:           apierrors.NewForbidden(schema.GroupResource{Group: "projectcalico.org", Resource: "networkpolicies"}, "test", fmt.Errorf("Operation on Calico tiered policy is forbidden")),
			shouldRecover: true,
			description:   "forbidden errors indicate permanent access restrictions and should be skipped",
		},
		{
			name:          "unauthorized error",
			err:           apierrors.NewUnauthorized("access denied"),
			shouldRecover: true,
			description:   "unauthorized errors indicate permanent access restrictions and should be skipped",
		},
		{
			name:          "not found error",
			err:           apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test"),
			shouldRecover: true,
			description:   "resource not found errors are permanent and should be skipped",
		},
		{
			name:          "method not supported error",
			err:           apierrors.NewMethodNotSupported(schema.GroupResource{Group: "", Resource: "test"}, "list"),
			shouldRecover: true,
			description:   "method not supported errors are permanent and should be skipped",
		},
		{
			name:          "calico tiered policy error",
			err:           fmt.Errorf("Operation on Calico tiered policy is forbidden"),
			shouldRecover: true,
			description:   "specific Calico error messages indicate permanent restrictions and should be skipped",
		},
		{
			name:          "server could not find resource error",
			err:           fmt.Errorf("the server could not find the requested resource"),
			shouldRecover: true,
			description:   "server cannot find resource errors are permanent and should be skipped",
		},
		{
			name:          "no Kind registered error",
			err:           fmt.Errorf("no Kind is registered for the type"),
			shouldRecover: true,
			description:   "unregistered Kind errors are permanent and should be skipped",
		},
		{
			name:          "no matches for kind error",
			err:           fmt.Errorf("no matches for kind NetworkPolicy"),
			shouldRecover: true,
			description:   "no matches for kind likely indicates missing CRDs and should be skipped for cleanup",
		},
		{
			name:          "unable to recognize error",
			err:           fmt.Errorf("unable to recognize resource"),
			shouldRecover: true,
			description:   "unable to recognize errors likely indicate missing resource definitions and should be skipped",
		},
		{
			name:          "connection refused error",
			err:           fmt.Errorf("connection refused"),
			shouldRecover: false,
			description:   "connection refused is a temporary network issue and should trigger retry",
		},
		{
			name:          "timeout error",
			err:           fmt.Errorf("request timeout"),
			shouldRecover: false,
			description:   "timeout errors are temporary and should trigger retry",
		},
		{
			name:          "deadline exceeded error",
			err:           fmt.Errorf("context deadline exceeded"),
			shouldRecover: false,
			description:   "deadline exceeded errors are temporary and should trigger retry",
		},
		{
			name:          "service unavailable error",
			err:           fmt.Errorf("service unavailable"),
			shouldRecover: false,
			description:   "service unavailable errors are temporary and should trigger retry",
		},
		{
			name:          "internal server error",
			err:           fmt.Errorf("internal server error"),
			shouldRecover: false,
			description:   "internal server errors are temporary and should trigger retry",
		},
		{
			name:          "too many requests error",
			err:           fmt.Errorf("too many requests"),
			shouldRecover: false,
			description:   "rate limiting errors are temporary and should trigger retry",
		},
		{
			name:          "unknown error",
			err:           fmt.Errorf("some unknown error occurred"),
			shouldRecover: false,
			description:   "unknown errors should trigger retry to ensure they're not silently ignored",
		},
		{
			name:          "network connection error",
			err:           fmt.Errorf("connection reset by peer"),
			shouldRecover: false,
			description:   "network connection errors are temporary and should trigger retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.err)
			if result != tt.shouldRecover {
				t.Errorf("isRecoverableError() = %v, want %v. %s", result, tt.shouldRecover, tt.description)
			}
		})
	}
}
